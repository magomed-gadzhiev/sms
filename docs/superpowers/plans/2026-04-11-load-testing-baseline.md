# Load Testing Baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace scattered k6 scripts with a unified baseline test suite that pushes metrics to Prometheus and enables run-over-run comparison in Grafana.

**Architecture:** k6 runs on the server (`72.56.232.202`) against Client Gateway on `:8080`. Results stream via `--out experimental-prometheus-rw` to Prometheus (`:9091`). A new Grafana dashboard exposes two `run_id` variables so any two runs can be overlaid. The shell runner orchestrates smoke → pipeline → optional soak in sequence, aborting if smoke fails.

**Tech Stack:** k6 >= 0.42, Prometheus remote write receiver, Grafana (provisioned JSON dashboard), bash

---

## File Map

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `scripts/k6_baseline_test.js` | All test scenarios, metrics, summary |
| Create | `scripts/run-baseline.sh` | Runner: preflight, tagging, k6 invocation |
| Create | `deployments/configs/grafana/dashboards/load-test-baseline.json` | Grafana dashboard |
| Modify | `deployments/docker-compose.yml` line 1210–1214 | Add `--web.enable-remote-write-receiver` |
| Delete | `scripts/k6_load_test.js` | Replaced |
| Delete | `scripts/k6_10k_load_test.js` | Replaced |
| Delete | `scripts/k6_quick_test.js` | Replaced |
| Delete | `scripts/load-test.sh` | Replaced |

---

## Task 1: Enable Prometheus Remote Write Receiver

**Files:**
- Modify: `deployments/docker-compose.yml:1210-1214`

- [ ] **Step 1: Add the flag**

Open `deployments/docker-compose.yml`. Find the `prometheus` service `command` block (around line 1210) and add the remote-write flag:

```yaml
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9091:9090"
    volumes:
      - ./configs/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/usr/share/prometheus/console_libraries'
      - '--web.console.templates=/usr/share/prometheus/consoles'
      - '--web.enable-remote-write-receiver'
    networks:
      - smpp-network
    restart: unless-stopped
```

- [ ] **Step 2: Commit**

```bash
git add deployments/docker-compose.yml
git commit -m "feat(load-test): enable prometheus remote write receiver"
```

---

## Task 2: Create k6 Baseline Test Script

**Files:**
- Create: `scripts/k6_baseline_test.js`

- [ ] **Step 1: Create the file**

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Counter, Trend } from 'k6/metrics';
import { SharedArray } from 'k6/data';

// ─── Fixtures ──────────────────────────────────────────────────────────────

const destinations = new SharedArray('destinations', function () {
    const data = JSON.parse(open('../test/load/fixtures/destinations.json'));
    const all = [];
    for (const group of data.domestic) all.push(...group.numbers);
    for (const group of data.international) all.push(...group.numbers);
    return all;
});

const sources = new SharedArray('sources', function () {
    const data = JSON.parse(open('../test/load/fixtures/sources.json'));
    return data.sources.map(s => s.name);
});

const messageTemplates = new SharedArray('messages', function () {
    const data = JSON.parse(open('../test/load/fixtures/messages.json'));
    const all = [];
    for (const cat of data.templates) all.push(...cat.texts);
    return all;
});

// ─── Custom Metrics ────────────────────────────────────────────────────────

const sendLatency    = new Trend('sms_send_latency',    true); // milliseconds
const e2eLatency     = new Trend('sms_e2e_latency',     true);
const pipelineErrors = new Rate('sms_pipeline_errors');
const sentTotal      = new Counter('sms_sent_total');
const batchTotal     = new Counter('sms_batch_total');

// ─── Config from env ───────────────────────────────────────────────────────

const SCENARIO = __ENV.SCENARIO  || 'pipeline';
const BASE_URL = __ENV.BASE_URL  || 'http://localhost:8080';
const API_KEY  = __ENV.API_KEY   || 'test-api-key';
const RUN_ID   = __ENV.RUN_ID   || 'unknown';
const RUN_TYPE = __ENV.RUN_TYPE  || 'regression';
const GIT_SHA  = __ENV.GIT_SHA   || 'unknown';

// ─── Options per scenario ──────────────────────────────────────────────────

const commonTags = { run_id: RUN_ID, run_type: RUN_TYPE, git_sha: GIT_SHA };

const smokeOptions = {
    scenarios: {
        smoke: {
            executor: 'constant-vus',
            vus: 10,
            duration: '2m',
            exec: 'smokeDefault',
            tags: { scenario: 'smoke' },
        },
    },
    thresholds: {
        'http_req_failed':   ['rate<0.01'],
        'http_req_duration': ['p(95)<1000'],
    },
    tags: commonTags,
};

const pipelineOptions = {
    scenarios: {
        pipeline_baseline: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '2m', target: 200 },
                { duration: '3m', target: 500 },
                { duration: '5m', target: 500 },
                { duration: '2m', target: 0 },
            ],
            exec: 'pipelineDefault',
            gracefulRampDown: '30s',
            tags: { scenario: 'pipeline' },
        },
    },
    thresholds: {
        'http_req_duration{name:SendSMS}': ['p(95)<500', 'p(99)<1000'],
        'http_req_failed':                 ['rate<0.01'],
        'sms_pipeline_errors':             ['rate<0.02'],
    },
    tags: commonTags,
};

const soakOptions = {
    scenarios: {
        soak: {
            executor: 'constant-vus',
            vus: 200,
            duration: '30m',
            exec: 'pipelineDefault',
            tags: { scenario: 'soak' },
        },
    },
    thresholds: {
        'http_req_duration{name:SendSMS}': ['p(95)<500'],
        'http_req_failed':                 ['rate<0.01'],
        'sms_pipeline_errors':             ['rate<0.02'],
    },
    tags: commonTags,
};

export const options =
    SCENARIO === 'smoke' ? smokeOptions :
    SCENARIO === 'soak'  ? soakOptions  :
                           pipelineOptions;

// ─── Helpers ───────────────────────────────────────────────────────────────

const headers = { 'Content-Type': 'application/json', 'X-API-Key': API_KEY };

function pick(arr) { return arr[Math.floor(Math.random() * arr.length)]; }

function destination() {
    if (Math.random() < 0.7) return pick(destinations);
    const prefixes = ['7910', '7903', '7920', '7900', '7999', '7985', '7925'];
    return pick(prefixes) + Math.floor(Math.random() * 10000000).toString().padStart(7, '0');
}

function messageText() {
    return pick(messageTemplates)
        .replace('{code}',       Math.floor(100000 + Math.random() * 900000))
        .replace('{order_id}',   Math.floor(10000  + Math.random() * 90000))
        .replace('{amount}',     (Math.random() * 10000).toFixed(2))
        .replace('{balance}',    (Math.random() * 100000).toFixed(2))
        .replace('{receipt_id}', Math.floor(1000000 + Math.random() * 9000000))
        .replace('{tracking}',   'LT' + Math.floor(1000000000 + Math.random() * 9000000000))
        .replace('{discount}',   Math.floor(5 + Math.random() * 75))
        .replace('{promo}',      Math.floor(1000 + Math.random() * 9000))
        .replace('{time}',       `${Math.floor(8 + Math.random() * 12)}:${Math.floor(Math.random() * 60).toString().padStart(2, '0')}`)
        .replace('{ip}',         `${Math.floor(1 + Math.random() * 254)}.${Math.floor(Math.random() * 256)}.${Math.floor(Math.random() * 256)}.1`)
        .replace('{rate}',       Math.floor(100 + Math.random() * 9900))
        .replace('{tx_id}',      Math.floor(10000000 + Math.random() * 90000000));
}

// VU-local queue of sent messages for e2e tracking
// Each VU has its own copy; no shared mutable state between VUs.
const pendingMessages = []; // { id: string, sentAt: number }
const E2E_TIMEOUT_MS = 30000;

// ─── Request functions ─────────────────────────────────────────────────────

function sendSMS() {
    const payload = JSON.stringify({
        source:              pick(sources),
        destination:         destination(),
        text:                messageText(),
        external_id:         `k6-${__VU}-${__ITER}-${Date.now()}`,
        registered_delivery: true,
    });

    const res = http.post(`${BASE_URL}/api/v1/sms/send`, payload, {
        headers,
        tags: { name: 'SendSMS' },
    });

    sendLatency.add(res.timings.duration);
    sentTotal.add(1);

    const ok = check(res, {
        'SendSMS status 200': r => r.status === 200,
        'SendSMS has message_id': r => {
            try { return !!JSON.parse(r.body).message_id; } catch { return false; }
        },
    });

    if (ok) {
        try {
            const body = JSON.parse(res.body);
            if (body.message_id) {
                pendingMessages.push({ id: body.message_id, sentAt: Date.now() });
                // Cap queue at 20 to prevent unbounded growth per VU
                if (pendingMessages.length > 20) pendingMessages.shift();
            }
        } catch { /* ignore parse errors */ }
    }

    return ok;
}

function sendBatch() {
    const batchSize = 5 + Math.floor(Math.random() * 16); // 5-20 messages
    const messages = Array.from({ length: batchSize }, (_, i) => ({
        source:      pick(sources),
        destination: destination(),
        text:        messageText(),
        external_id: `k6-batch-${__VU}-${__ITER}-${i}-${Date.now()}`,
    }));

    const res = http.post(`${BASE_URL}/api/v1/sms/batch`, JSON.stringify({ messages }), {
        headers,
        tags: { name: 'SendBatch' },
    });

    batchTotal.add(batchSize);

    return check(res, {
        'SendBatch status 200': r => r.status === 200,
        'SendBatch has results': r => {
            try { return Array.isArray(JSON.parse(r.body).results); } catch { return false; }
        },
    });
}

function pollStatus() {
    // If no pending messages, fall back to sendSMS
    if (pendingMessages.length === 0) return sendSMS();

    const msg = pendingMessages[0];
    const elapsed = Date.now() - msg.sentAt;

    // Timeout — message never reached delivered
    if (elapsed > E2E_TIMEOUT_MS) {
        pendingMessages.shift();
        pipelineErrors.add(1);
        return false;
    }

    const res = http.get(`${BASE_URL}/api/v1/sms/status/${msg.id}`, {
        headers: { 'X-API-Key': API_KEY },
        tags: { name: 'PollStatus' },
    });

    if (!check(res, { 'PollStatus status 200': r => r.status === 200 })) {
        return false;
    }

    try {
        const status = JSON.parse(res.body).status;
        if (status === 'delivered' || status === 'sent') {
            e2eLatency.add(Date.now() - msg.sentAt);
            pendingMessages.shift();
        } else if (status === 'failed' || status === 'rejected') {
            pipelineErrors.add(1);
            pendingMessages.shift();
        }
        // 'pending', 'queued' etc → stay in queue, poll again next iteration
    } catch { /* ignore */ }

    return true;
}

function healthCheck() {
    const res = http.get(`${BASE_URL}/health`, { tags: { name: 'Health' } });
    return check(res, { 'Health status 200': r => r.status === 200 });
}

// ─── Scenario executors ────────────────────────────────────────────────────

export function smokeDefault() {
    const ok = Math.random() < 0.8 ? sendSMS() : healthCheck();
    sleep(0.1);
    return ok;
}

export function pipelineDefault() {
    const roll = Math.random() * 100;
    let ok;
    if      (roll < 70) ok = sendSMS();
    else if (roll < 85) ok = sendBatch();
    else                ok = pollStatus();
    sleep(0.05);
    return ok;
}

// Required by k6 when no scenario `exec` is set; also used for local dev runs
export default function () {
    if (SCENARIO === 'smoke') return smokeDefault();
    return pipelineDefault();
}

// ─── Summary ───────────────────────────────────────────────────────────────

export function handleSummary(data) {
    const m = data.metrics;
    const fmt = (v, unit) => (v !== undefined ? `${v.toFixed(1)}${unit}` : 'N/A');

    let s = '\n' + '='.repeat(65) + '\n';
    s += `  SMS Gateway ${SCENARIO.toUpperCase()} — run_id: ${RUN_ID}  git: ${GIT_SHA}\n`;
    s += '='.repeat(65) + '\n\n';

    if (m.http_reqs) {
        const r = m.http_reqs.values;
        s += `Throughput:      ${fmt(r.rate, ' req/s')}  (total: ${r.count})\n`;
    }
    if (m.http_req_duration) {
        const d = m.http_req_duration.values;
        s += `HTTP Latency:    avg=${fmt(d.avg, 'ms')}  p95=${fmt(d['p(95)'], 'ms')}  p99=${fmt(d['p(99)'], 'ms')}\n`;
    }
    if (m.sms_e2e_latency) {
        const d = m.sms_e2e_latency.values;
        s += `E2E Latency:     avg=${fmt(d.avg, 'ms')}  p95=${fmt(d['p(95)'], 'ms')}  p99=${fmt(d['p(99)'], 'ms')}\n`;
    }
    if (m.http_req_failed) {
        s += `Error rate:      ${fmt(m.http_req_failed.values.rate * 100, '%')}\n`;
    }
    if (m.sms_pipeline_errors) {
        s += `Pipeline errors: ${fmt(m.sms_pipeline_errors.values.rate * 100, '%')}\n`;
    }
    if (m.sms_sent_total) {
        s += `SMS sent:        ${m.sms_sent_total.values.count}\n`;
    }

    s += '\n' + '='.repeat(65) + '\n';

    return {
        stdout: s,
        [`baseline-${SCENARIO}-${RUN_ID}.json`]: JSON.stringify(data, null, 2),
    };
}
```

- [ ] **Step 2: Commit**

```bash
git add scripts/k6_baseline_test.js
git commit -m "feat(load-test): add k6 baseline test script with e2e latency tracking"
```

---

## Task 3: Create run-baseline.sh Runner

**Files:**
- Create: `scripts/run-baseline.sh`

- [ ] **Step 1: Create the file**

```bash
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

# ─── Prometheus remote write setup ─────────────────────────────────────────
PROM_FLAGS=""
PROM_HOST="${PROM_URL%/api/v1/write}"

if curl -sf --max-time 3 "${PROM_HOST}/-/ready" >/dev/null 2>&1; then
    echo "Prometheus reachable — metrics will stream to Grafana"
    PROM_FLAGS="--out experimental-prometheus-rw"
    export K6_PROMETHEUS_RW_SERVER_URL="${PROM_URL}"
    export K6_PROMETHEUS_RW_TREND_STATS="p50,p95,p99"
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
        ${PROM_FLAGS} \
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
if [[ -n "${PROM_FLAGS}" ]]; then
    echo ""
    echo "  Grafana: http://72.56.232.202:3001"
    echo "  Dashboard: Load Test Baseline"
    echo "  Select run_id: ${RUN_ID}"
fi
echo "================================================================"
```

- [ ] **Step 2: Make executable and commit**

```bash
chmod +x scripts/run-baseline.sh
git add scripts/run-baseline.sh
git commit -m "feat(load-test): add run-baseline.sh orchestration script"
```

---

## Task 4: Create Grafana Baseline Dashboard

**Files:**
- Create: `deployments/configs/grafana/dashboards/load-test-baseline.json`

- [ ] **Step 1: Create the dashboard JSON**

```json
{
  "annotations": { "list": [] },
  "description": "k6 load test baseline — compare any two runs side by side",
  "editable": true,
  "graphTooltip": 1,
  "id": null,
  "links": [],
  "panels": [
    {
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "fieldConfig": {
        "defaults": { "color": { "mode": "palette-classic" }, "unit": "short" },
        "overrides": []
      },
      "gridPos": { "h": 6, "w": 24, "x": 0, "y": 0 },
      "id": 1,
      "options": {
        "legend": { "calcs": ["max"], "displayMode": "list", "placement": "bottom" },
        "tooltip": { "mode": "multi" }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_vus{run_id=\"$run_id\"}",
          "legendFormat": "VUs — current ({{run_id}})"
        },
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_vus{run_id=\"$baseline_run_id\"}",
          "legendFormat": "VUs — baseline ({{run_id}})"
        }
      ],
      "title": "Virtual Users",
      "type": "timeseries"
    },
    {
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "fieldConfig": {
        "defaults": { "color": { "mode": "palette-classic" }, "unit": "reqps" },
        "overrides": []
      },
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 6 },
      "id": 2,
      "options": {
        "legend": { "calcs": ["mean", "max"], "displayMode": "table", "placement": "bottom" },
        "tooltip": { "mode": "multi" }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "rate(k6_http_reqs_total{run_id=\"$run_id\"}[30s])",
          "legendFormat": "Throughput — current"
        },
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "rate(k6_http_reqs_total{run_id=\"$baseline_run_id\"}[30s])",
          "legendFormat": "Throughput — baseline"
        }
      ],
      "title": "Throughput (req/s)",
      "type": "timeseries"
    },
    {
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "fieldConfig": {
        "defaults": { "color": { "mode": "palette-classic" }, "unit": "ms" },
        "overrides": []
      },
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 6 },
      "id": 3,
      "options": {
        "legend": { "calcs": ["mean", "max"], "displayMode": "table", "placement": "bottom" },
        "tooltip": { "mode": "multi" }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_http_req_duration_p95{name=\"SendSMS\",run_id=\"$run_id\"}",
          "legendFormat": "p95 — current"
        },
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_http_req_duration_p99{name=\"SendSMS\",run_id=\"$run_id\"}",
          "legendFormat": "p99 — current"
        },
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_http_req_duration_p95{name=\"SendSMS\",run_id=\"$baseline_run_id\"}",
          "legendFormat": "p95 — baseline"
        },
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_http_req_duration_p99{name=\"SendSMS\",run_id=\"$baseline_run_id\"}",
          "legendFormat": "p99 — baseline"
        }
      ],
      "title": "HTTP Send Latency — SendSMS (ms)",
      "type": "timeseries"
    },
    {
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "fieldConfig": {
        "defaults": { "color": { "mode": "palette-classic" }, "unit": "ms" },
        "overrides": []
      },
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 14 },
      "id": 4,
      "options": {
        "legend": { "calcs": ["mean", "max"], "displayMode": "table", "placement": "bottom" },
        "tooltip": { "mode": "multi" }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_sms_e2e_latency_p50{run_id=\"$run_id\"}",
          "legendFormat": "p50 — current"
        },
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_sms_e2e_latency_p95{run_id=\"$run_id\"}",
          "legendFormat": "p95 — current"
        },
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_sms_e2e_latency_p95{run_id=\"$baseline_run_id\"}",
          "legendFormat": "p95 — baseline"
        }
      ],
      "title": "E2E Pipeline Latency — Send to DLR (ms)",
      "type": "timeseries"
    },
    {
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "fieldConfig": {
        "defaults": { "color": { "mode": "palette-classic" }, "unit": "percentunit" },
        "overrides": []
      },
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 14 },
      "id": 5,
      "options": {
        "legend": { "calcs": ["mean", "max"], "displayMode": "table", "placement": "bottom" },
        "tooltip": { "mode": "multi" }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_http_req_failed_ratio{run_id=\"$run_id\"}",
          "legendFormat": "HTTP errors — current"
        },
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_sms_pipeline_errors_ratio{run_id=\"$run_id\"}",
          "legendFormat": "Pipeline errors — current"
        },
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_http_req_failed_ratio{run_id=\"$baseline_run_id\"}",
          "legendFormat": "HTTP errors — baseline"
        }
      ],
      "title": "Error Rate",
      "type": "timeseries"
    },
    {
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "fieldConfig": {
        "defaults": { "unit": "short" },
        "overrides": []
      },
      "gridPos": { "h": 4, "w": 6, "x": 0, "y": 22 },
      "id": 6,
      "options": { "colorMode": "background", "graphMode": "none", "justifyMode": "center", "orientation": "auto", "reduceOptions": { "calcs": ["lastNotNull"] } },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_sms_sent_total_total{run_id=\"$run_id\"}",
          "legendFormat": "SMS sent"
        }
      ],
      "title": "Total SMS Sent",
      "type": "stat"
    },
    {
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "fieldConfig": {
        "defaults": { "unit": "ms" },
        "overrides": []
      },
      "gridPos": { "h": 4, "w": 6, "x": 6, "y": 22 },
      "id": 7,
      "options": { "colorMode": "background", "graphMode": "none", "justifyMode": "center", "orientation": "auto", "reduceOptions": { "calcs": ["max"] } },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_http_req_duration_p95{name=\"SendSMS\",run_id=\"$run_id\"}",
          "legendFormat": "p95 send latency"
        }
      ],
      "title": "Peak p95 Send Latency",
      "type": "stat"
    },
    {
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "fieldConfig": {
        "defaults": { "unit": "ms" },
        "overrides": []
      },
      "gridPos": { "h": 4, "w": 6, "x": 12, "y": 22 },
      "id": 8,
      "options": { "colorMode": "background", "graphMode": "none", "justifyMode": "center", "orientation": "auto", "reduceOptions": { "calcs": ["max"] } },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_sms_e2e_latency_p95{run_id=\"$run_id\"}",
          "legendFormat": "p95 e2e latency"
        }
      ],
      "title": "Peak p95 E2E Latency",
      "type": "stat"
    },
    {
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "fieldConfig": {
        "defaults": { "unit": "percentunit", "thresholds": { "mode": "absolute", "steps": [{"color": "green", "value": null}, {"color": "yellow", "value": 0.01}, {"color": "red", "value": 0.02}] } },
        "overrides": []
      },
      "gridPos": { "h": 4, "w": 6, "x": 18, "y": 22 },
      "id": 9,
      "options": { "colorMode": "background", "graphMode": "none", "justifyMode": "center", "orientation": "auto", "reduceOptions": { "calcs": ["max"] } },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "k6_http_req_failed_ratio{run_id=\"$run_id\"}",
          "legendFormat": "error rate"
        }
      ],
      "title": "Peak Error Rate",
      "type": "stat"
    }
  ],
  "refresh": "10s",
  "schemaVersion": 38,
  "tags": ["k6", "load-test", "baseline"],
  "templating": {
    "list": [
      {
        "current": {},
        "datasource": { "type": "prometheus", "uid": "prometheus" },
        "definition": "label_values(k6_http_reqs_total, run_id)",
        "hide": 0,
        "includeAll": false,
        "label": "Current Run",
        "multi": false,
        "name": "run_id",
        "options": [],
        "query": {
          "query": "label_values(k6_http_reqs_total, run_id)",
          "refId": "StandardVariableQuery"
        },
        "refresh": 2,
        "regex": "",
        "sort": 2,
        "type": "query"
      },
      {
        "current": {},
        "datasource": { "type": "prometheus", "uid": "prometheus" },
        "definition": "label_values(k6_http_reqs_total, run_id)",
        "hide": 0,
        "includeAll": true,
        "label": "Baseline Run",
        "multi": false,
        "name": "baseline_run_id",
        "options": [],
        "query": {
          "query": "label_values(k6_http_reqs_total, run_id)",
          "refId": "StandardVariableQuery"
        },
        "refresh": 2,
        "regex": "",
        "sort": 2,
        "type": "query"
      }
    ]
  },
  "time": { "from": "now-30m", "to": "now" },
  "timepicker": {},
  "timezone": "browser",
  "title": "Load Test Baseline",
  "uid": "load-test-baseline",
  "version": 1
}
```

- [ ] **Step 2: Commit**

```bash
git add deployments/configs/grafana/dashboards/load-test-baseline.json
git commit -m "feat(load-test): add grafana baseline comparison dashboard"
```

---

## Task 5: Remove Old Scripts

**Files:**
- Delete: `scripts/k6_load_test.js`, `scripts/k6_10k_load_test.js`, `scripts/k6_quick_test.js`, `scripts/load-test.sh`

- [ ] **Step 1: Delete and commit**

```bash
git rm scripts/k6_load_test.js scripts/k6_10k_load_test.js scripts/k6_quick_test.js scripts/load-test.sh
git commit -m "chore(load-test): remove old k6 scripts replaced by k6_baseline_test.js"
```

---

## Task 6: Deploy and Smoke-Test on Server

- [ ] **Step 1: Push to GitHub**

```bash
git push origin master
```

- [ ] **Step 2: Deploy on server**

```bash
./scripts/server.sh deploy
```

Expected output: containers restarting, Prometheus restarts with remote write receiver enabled.

- [ ] **Step 3: Verify Prometheus remote write is live**

```bash
./scripts/server.sh exec "curl -sf http://localhost:9091/-/ready && echo OK"
```

Expected: `OK`

```bash
./scripts/server.sh exec "curl -sf -X POST http://localhost:9091/api/v1/write -H 'Content-Type: application/x-protobuf' 2>&1 | head -5"
```

Expected: response (even an error body) — not `connection refused`. If you get `connection refused`, the flag wasn't applied; check docker logs prometheus.

- [ ] **Step 4: Install k6 on server (if not present)**

```bash
./scripts/server.sh exec "k6 version 2>/dev/null || (sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69 && echo 'deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main' | sudo tee /etc/apt/sources.list.d/k6.list && sudo apt-get update -q && sudo apt-get install -y k6)"
```

Expected: `k6 v0.xx.x ...`

- [ ] **Step 5: Run smoke test on server**

```bash
./scripts/server.sh exec "cd /opt/sms && BASE_URL=http://localhost:8080 API_KEY=<actual-api-key> RUN_TYPE=baseline bash scripts/run-baseline.sh 2>&1 | tail -40"
```

Replace `<actual-api-key>` with a valid key from the running system.

Expected: smoke passes, pipeline baseline runs, summary printed with `run_id=YYYY-MM-DD-HHmm`.

- [ ] **Step 6: Verify Grafana dashboard**

Open `http://72.56.232.202:3001`, find dashboard **"Load Test Baseline"**, select the `run_id` from the dropdown. Panels should show data.

If dashboard doesn't appear: Grafana auto-provisions from the mounted volume. Check provisioning:

```bash
./scripts/server.sh exec "docker exec grafana ls /var/lib/grafana/dashboards/ | grep baseline"
```

Expected: `load-test-baseline.json`

---

## Self-Review

**Spec coverage check:**
- ✅ Prometheus remote write receiver enabled (Task 1)
- ✅ k6 script with smoke, pipeline, soak scenarios (Task 2)
- ✅ e2e latency tracking via pollStatus (Task 2)
- ✅ run_id / run_type / git_sha tags (Task 2, Task 3)
- ✅ Thresholds (pipeline + soak scenarios in Task 2)
- ✅ Runner with abort-on-smoke-failure (Task 3)
- ✅ Grafana dashboard with run comparison (Task 4)
- ✅ Old scripts removed (Task 5)
- ✅ RUN_SOAK=1 optional soak (Task 3)

**Type consistency check:**
- `pendingMessages` used in `sendSMS`, `pollStatus` — consistent `{ id, sentAt }` shape ✅
- `smokeDefault` / `pipelineDefault` referenced in scenario `exec` fields — names match ✅
- Prometheus metric names (`k6_http_req_duration_p95`, `k6_sms_e2e_latency_p95`, `k6_sms_pipeline_errors_ratio`) — consistent between script and dashboard ✅
- Dashboard `uid: "prometheus"` — matches datasource uid in `datasources.yml` ✅
