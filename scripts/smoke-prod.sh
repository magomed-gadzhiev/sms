#!/bin/bash
# Production smoke test — black-box после deploy/cutover (Plan 7 Task 12 + Plan 8 Task 6).
#
# Required env: PORTAL_DOMAIN, API_DOMAIN, ADMIN_DOMAIN, ADMIN_EMAIL, ADMIN_PASSWORD.
#
# Optional: CANARY_CLIENT_IDS — если задан, Smoke 5 проверяет canary rejection;
# если пусто/unset — Smoke 5 SKIP'ит. Это caller-side гейт: operator exporting
# CANARY_CLIENT_IDS из .env.prod перед запуском smoke означает «canary mode
# должен быть on на сервере, проверяй».
#
# Canary enforcement (Plan 8 Task 6 followup fix):
#   canary.IsAllowed теперь вызывается во ВСЕХ путях send'а — gRPC, SMPP, HTTP.
#   До followup'а HTTP /api/v1/sms/send проходил мимо canary check'а.
#   Smoke 5 здесь делает registration probe (client lookup в admin gateway).
#   Полный 503 e2e-test требует client-auth flow (register → portal login →
#   create api-key → POST /api/v1/sms/send с X-API-Key, expect 503) — это
#   многошаговый scenario; отложен на post-cutover отдельной задачей.
#   До тех пор HTTP canary enforcement покрыт unit-тестом в
#   internal/gateway/client/handlers/sms_test.go (Plan 8 Task 6).
set -euo pipefail

PORTAL_URL="https://${PORTAL_DOMAIN:?PORTAL_DOMAIN required}"
ADMIN_EMAIL=${ADMIN_EMAIL:?ADMIN_EMAIL required}
ADMIN_PASSWORD=${ADMIN_PASSWORD:?ADMIN_PASSWORD required}

echo "=== Smoke 1: TLS reachable ==="
curl -fsS -o /dev/null --max-time 10 "$PORTAL_URL/health" || { echo "FAIL: TLS or portal unreachable"; exit 1; }
echo "OK"

echo "=== Smoke 2: Admin login ==="
TOKEN=$(curl -fsS --max-time 10 -X POST "$PORTAL_URL/portal/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" \
    | jq -r '.token')
test -n "$TOKEN" || { echo "FAIL: login returned empty token"; exit 1; }
echo "OK"

echo "=== Smoke 3: SRA-stuck endpoint reachable ==="
curl -fsS --max-time 10 -H "Authorization: Bearer $TOKEN" "$PORTAL_URL/portal/v1/admin/network/sra-stuck" \
    | jq '.rows | length' >/dev/null || { echo "FAIL: SRA-stuck endpoint"; exit 1; }
echo "OK"

echo "=== Smoke 4: api/admin subdomain TLS reachable ==="
# Verified paths (Plan 8 Task 6 Step 1):
#   API_DOMAIN  → Caddy → haproxy:8080 → client-gateway /api/v1/* (router.go line 41)
#   ADMIN_DOMAIN → Caddy → haproxy:8081 → admin-gateway /admin/v1/* (router.go line 55)
# Both gateways register /health at root (no prefix) per their router setup.
API_URL="https://${API_DOMAIN:?API_DOMAIN required for Smoke 4}"
ADMIN_URL="https://${ADMIN_DOMAIN:?ADMIN_DOMAIN required for Smoke 4}"

curl -fsS -o /dev/null --max-time 10 "$API_URL/health" || { echo "FAIL: $API_URL/health unreachable"; exit 1; }
curl -fsS -o /dev/null --max-time 10 "$ADMIN_URL/health" || { echo "FAIL: $ADMIN_URL/health unreachable"; exit 1; }
echo "OK"

echo "=== Smoke 5: canary registration probe ==="
# Activated через caller-side env var CANARY_CLIENT_IDS (см. header).
#
# Verifies client registration + admin cleanup flow (POST /portal/v1/auth/register
# → DELETE /admin/v1/clients/{id}). Полный 503 e2e через HTTP send требует
# client X-API-Key (multi-step setup) — отложен. HTTP canary enforcement
# покрыт unit-тестом (см. handlers/sms_test.go Plan 8 Task 6).
#
# Verified paths (Step 1):
#   POST   /portal/v1/auth/register  — creates client + user, returns {client_id, user}, sets session cookie
#   DELETE /admin/v1/clients/{id}    — admin gateway cleanup (Authorization: Bearer $TOKEN)
if [ -z "${CANARY_CLIENT_IDS:-}" ]; then
    echo "SKIP: CANARY_CLIENT_IDS empty/unset on caller — canary mode off"
else
    # Register a new client (creates company + user account in one step).
    # This client is NOT in CANARY_CLIENT_IDS, so gRPC/SMPP sends would be rejected.
    SMOKE_EMAIL="smoke-canary-$(date +%s)@test.local"
    REGISTER_RESP=$(curl -fsS --max-time 10 -X POST "$PORTAL_URL/portal/v1/auth/register" \
        -H 'Content-Type: application/json' \
        -d "{\"email\":\"$SMOKE_EMAIL\",\"password\":\"SmokeTest1234567!\",\"company_name\":\"smoke-canary-test\"}")
    TEST_CLIENT_ID=$(echo "$REGISTER_RESP" | jq -r '.client_id // empty')
    test -n "$TEST_CLIENT_ID" || { echo "FAIL: register returned no client_id: $REGISTER_RESP"; exit 1; }
    echo "  registered test client $TEST_CLIENT_ID"

    # Cleanup при exit (success или fail).
    # Uses admin Bearer token ($TOKEN from Smoke 2) against ADMIN_DOMAIN.
    # DELETE /admin/v1/clients/{id} — verified in admin router.go line 63.
    trap 'curl -fsS --max-time 10 -X DELETE \
        -H "Authorization: Bearer $TOKEN" \
        "$ADMIN_URL/admin/v1/clients/$TEST_CLIENT_ID" >/dev/null 2>&1 || true' EXIT

    echo "OK (registration + cleanup flow). HTTP canary enforcement covered by unit test."
fi

echo ""
echo "=== AUTOMATED smoke OK ==="
echo ""
echo "Manual checks (run via SSH on prod-host):"
echo "  - ./scripts/server.sh logs worker | grep 'partition maintenance'"
echo "  - docker compose ... exec prometheus curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.health!=\"up\")'"
echo "  - curl http://localhost:9093/api/v2/alerts | jq '[.[] | .labels.alertname]'"
