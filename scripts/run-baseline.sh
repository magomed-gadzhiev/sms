#!/usr/bin/env bash
# run-baseline.sh — SMS Gateway load test runner
# Usage:
#   RUN_TYPE=baseline ./scripts/run-baseline.sh
#   RUN_TYPE=regression RUN_SOAK=1 ./scripts/run-baseline.sh
#   BASE_URL=http://72.56.232.202:8080 API_KEY=<key> ./scripts/run-baseline.sh

set -euo pipefail

# ─── Configuration ─────────────────────────────────────────────────────────
BASE_URL="${BASE_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-test-api-key}"
RUN_TYPE="${RUN_TYPE:-regression}"
RUN_SOAK="${RUN_SOAK:-0}"
PROM_URL="${PROM_URL:-http://localhost:9091/api/v1/write}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
K6_SCRIPT="${SCRIPT_DIR}/k6_baseline_test.js"

# ─── Run metadata ──────────────────────────────────────────────────────────
RUN_ID="$(date +%Y-%m-%d-%H%M)"
GIT_SHA="$(git -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

# ─── Preflight ─────────────────────────────────────────────────────────────
echo "================================================================"
echo "  SMS Gateway Baseline Test Runner"
echo "================================================================"
echo "  RUN_ID:   ${RUN_ID}"
echo "  RUN_TYPE: ${RUN_TYPE}"
echo "  GIT_SHA:  ${GIT_SHA}"
echo "  BASE_URL: ${BASE_URL}"
echo "  SOAK:     ${RUN_SOAK}"
echo "================================================================"
echo ""

if ! command -v k6 &>/dev/null; then
    echo "ERROR: k6 not found."
    echo "Install: https://k6.io/docs/get-started/installation/"
    echo "  Linux:  sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69"
    echo "          echo 'deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main' | sudo tee /etc/apt/sources.list.d/k6.list"
    echo "          sudo apt-get update && sudo apt-get install k6"
    exit 1
fi

echo "k6 $(k6 version | head -1)"
echo ""

# ─── LOAD_TEST_MODE check ──────────────────────────────────────────────────
# Client-gateway requires LOAD_TEST_MODE=true to bypass JWT auth for load tests.
# Ensure this is set in deployments/.env (persists across docker compose restarts).
COMPOSE_ENV="${SCRIPT_DIR}/../deployments/.env"
if ! grep -q "LOAD_TEST_MODE=true" "${COMPOSE_ENV}" 2>/dev/null; then
    echo "WARNING: LOAD_TEST_MODE=true not found in deployments/.env"
    echo "         Client-gateway may reject requests with 401."
    echo "         Fix: echo 'LOAD_TEST_MODE=true' >> ${COMPOSE_ENV}"
    echo "         Then: docker compose -f deployments/docker-compose.yml up -d client-gateway-1 client-gateway-2"
    echo ""
fi

# ─── Gateway health check ──────────────────────────────────────────────────
echo "Checking gateway at ${BASE_URL}/health ..."
if ! curl -sf --max-time 5 "${BASE_URL}/health" >/dev/null 2>&1; then
    echo "ERROR: Gateway not reachable at ${BASE_URL}/health"
    echo "       Check that the SMS gateway is running."
    exit 1
fi

# Quick auth check — catch LOAD_TEST_MODE=false before running full test
AUTH_STATUS=$(curl -sf --max-time 5 -o /dev/null -w "%{http_code}" \
    -X POST "${BASE_URL}/api/v1/sms/send" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: load-test-probe" \
    -d '{"source":"Probe","destination":"79000000000","text":"probe"}' 2>/dev/null || echo "000")
if [[ "${AUTH_STATUS}" == "401" ]]; then
    echo "ERROR: Gateway returned 401 — LOAD_TEST_MODE is likely false."
    echo "       Fix: echo 'LOAD_TEST_MODE=true' >> deployments/.env"
    echo "       Then: docker compose -f deployments/docker-compose.yml up -d client-gateway-1 client-gateway-2"
    exit 1
fi
echo "Gateway healthy (auth probe: HTTP ${AUTH_STATUS})."
echo ""

# ─── Prometheus remote write setup ─────────────────────────────────────────
PROM_FLAGS=()
PROM_HOST="${PROM_URL%/api/v1/write}"

if curl -sf --max-time 3 "${PROM_HOST}/-/ready" >/dev/null 2>&1; then
    echo "Prometheus reachable — metrics will stream to Grafana"
    PROM_FLAGS=("--out" "experimental-prometheus-rw")
    export K6_PROMETHEUS_RW_SERVER_URL="${PROM_URL}"
    export K6_PROMETHEUS_RW_TREND_STATS="p(50),p(95),p(99)"
    export K6_PROMETHEUS_RW_PUSH_INTERVAL="5s"
else
    echo "WARNING: Prometheus not reachable at ${PROM_HOST}"
    echo "         Test will run but metrics will NOT appear in Grafana."
    echo "         Check: docker ps | grep prometheus"
fi
echo ""

# ─── k6 runner helper ──────────────────────────────────────────────────────
run_k6() {
    local scenario="$1"
    echo "─── Scenario: ${scenario} ──────────────────────────────────────"
    k6 run \
        "${PROM_FLAGS[@]}" \
        --env SCENARIO="${scenario}" \
        --env BASE_URL="${BASE_URL}" \
        --env API_KEY="${API_KEY}" \
        --env RUN_ID="${RUN_ID}" \
        --env RUN_TYPE="${RUN_TYPE}" \
        --env GIT_SHA="${GIT_SHA}" \
        "${K6_SCRIPT}"
    echo ""
}

# ─── Step 1: Smoke ─────────────────────────────────────────────────────────
echo "Step 1/3: Smoke test (2 min, 10 VUs)"
if ! run_k6 smoke; then
    echo "================================================================"
    echo "  SMOKE TEST FAILED — system is unhealthy, aborting."
    echo "  Check: curl ${BASE_URL}/health"
    echo "================================================================"
    exit 1
fi

echo "Smoke passed. Proceeding to pipeline baseline."
echo ""

# ─── Step 2: Pipeline baseline ─────────────────────────────────────────────
echo "Step 2/3: Pipeline baseline (12 min, ramp 0→200→500→0 VUs)"
run_k6 pipeline

# ─── Step 3: Soak (optional) ───────────────────────────────────────────────
if [[ "${RUN_SOAK}" == "1" ]]; then
    echo "Step 3/3: Soak test (30 min, 200 VUs)"
    run_k6 soak
else
    echo "Step 3/3: Soak skipped (enable with: RUN_SOAK=1)"
fi

# ─── Done ──────────────────────────────────────────────────────────────────
echo "================================================================"
echo "  Done. run_id=${RUN_ID}"
if [[ ${#PROM_FLAGS[@]} -gt 0 ]]; then
    echo ""
    echo "  Grafana: http://72.56.232.202:3001"
    echo "  Dashboard: Load Test Baseline"
    echo "  Select run_id: ${RUN_ID}"
fi
echo "================================================================"
