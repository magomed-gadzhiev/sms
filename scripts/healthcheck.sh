#!/bin/bash
# Health check для всех критических prod-сервисов.
# Возвращает 0 если все ок, 1 если хотя бы один сервис не отвечает за timeout.
# Запускается из /opt/sms (working directory).
set -uo pipefail  # без -e — мы хотим check'нуть все, не fail-fast

TIMEOUT=${HEALTHCHECK_TIMEOUT:-30}  # секунды на каждый сервис
COMPOSE="docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml"

# Resolve env (для REDIS_PASSWORD).
if [ -f deployments/.env.prod ]; then
    set -a
    . deployments/.env.prod
    set +a
fi

# Сервис → команда проверки. Через gateway HAProxy на host: 8080/8081/8082 — HTTP.
# DB / Redis — через docker compose exec.
declare -A CHECKS=(
    ["postgres"]="$COMPOSE exec -T postgres pg_isready -U smpp -d smpp_db"
    ["redis"]="$COMPOSE exec -T redis sh -c 'redis-cli -a \"\$REDIS_PASSWORD\" ping' | grep -q PONG"
    ["worker-up"]="$COMPOSE ps worker | grep -q 'Up'"
    ["client-gateway"]="curl -fsS --max-time 5 http://localhost:8080/health"
    ["admin-gateway"]="curl -fsS --max-time 5 http://localhost:8081/health"
    ["portal-gateway"]="curl -fsS --max-time 5 http://localhost:8082/health"
    ["prometheus-up"]="$COMPOSE ps prometheus | grep -q 'Up'"
    ["alertmanager-up"]="$COMPOSE ps alertmanager | grep -q 'Up'"
    ["caddy-up"]="$COMPOSE ps caddy | grep -q 'Up'"
)

failed=()
for svc in "${!CHECKS[@]}"; do
    cmd="${CHECKS[$svc]}"
    printf "Checking %-20s ... " "$svc"
    if timeout "$TIMEOUT" bash -c "$cmd" >/dev/null 2>&1; then
        echo "OK"
    else
        echo "FAIL"
        failed+=("$svc")
    fi
done

if [ ${#failed[@]} -gt 0 ]; then
    echo ""
    echo "FAILED services: ${failed[*]}"
    exit 1
fi
echo ""
echo "All services healthy."
