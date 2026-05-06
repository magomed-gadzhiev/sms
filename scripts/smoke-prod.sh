#!/bin/bash
# Production smoke test — black-box после deploy/cutover (Plan 7 Task 12).
# Требует env: PORTAL_DOMAIN, ADMIN_EMAIL, ADMIN_PASSWORD.
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

echo ""
echo "=== AUTOMATED smoke OK ==="
echo ""
echo "Manual checks (run via SSH on prod-host):"
echo "  - ./scripts/server.sh logs worker | grep 'partition maintenance'"
echo "  - docker compose ... exec prometheus curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.health!=\"up\")'"
echo "  - curl http://localhost:9093/api/v2/alerts | jq '[.[] | .labels.alertname]'"
