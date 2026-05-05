#!/usr/bin/env bash
# Plan 6 Task 6 — host iptables firewall для Redis (port 6379).
# Defense in depth поверх:
#   1. compose port mapping 127.0.0.1:6379:6379 (Plan 5 Task 3)
#   2. requirepass (Plan 4 Task 3)
#   3. redis.conf protected-mode + bind 0.0.0.0 (Plan 6 Task 5)
#
# Идемпотентен: safe to re-run.
# Требует sudo.
set -euo pipefail

REDIS_PORT=6379

echo "Setting up iptables rules for Redis port ${REDIS_PORT}..."

# 1. ACCEPT loopback (host services могут подключаться через 127.0.0.1).
sudo iptables -C INPUT -i lo -p tcp --dport ${REDIS_PORT} -j ACCEPT 2>/dev/null \
    || sudo iptables -I INPUT 1 -i lo -p tcp --dport ${REDIS_PORT} -j ACCEPT

# 2. ACCEPT уже-установленные/related (existing connections).
sudo iptables -C INPUT -p tcp --dport ${REDIS_PORT} -m state --state ESTABLISHED,RELATED -j ACCEPT 2>/dev/null \
    || sudo iptables -I INPUT 2 -p tcp --dport ${REDIS_PORT} -m state --state ESTABLISHED,RELATED -j ACCEPT

# 3. ACCEPT docker bridge (контейнеры в smpp-network подключаются через docker0).
DOCKER_BRIDGE=$(docker network inspect $(docker network ls -q -f name=smpp-network) --format '{{range .Options}}{{.}}{{end}}' 2>/dev/null | grep -oP 'br-[a-f0-9]+' | head -1 || echo "")
if [ -n "$DOCKER_BRIDGE" ]; then
    sudo iptables -C INPUT -i "$DOCKER_BRIDGE" -p tcp --dport ${REDIS_PORT} -j ACCEPT 2>/dev/null \
        || sudo iptables -I INPUT 3 -i "$DOCKER_BRIDGE" -p tcp --dport ${REDIS_PORT} -j ACCEPT
    echo "  ACCEPT for docker bridge: ${DOCKER_BRIDGE}"
else
    echo "  WARN: docker bridge not found — skipping bridge ACCEPT rule"
fi

# 4. DROP всё остальное на 6379.
sudo iptables -C INPUT -p tcp --dport ${REDIS_PORT} -j DROP 2>/dev/null \
    || sudo iptables -A INPUT -p tcp --dport ${REDIS_PORT} -j DROP

echo "Rules applied. Current rules for port ${REDIS_PORT}:"
sudo iptables -L INPUT -n --line-numbers | grep ":${REDIS_PORT}\b" || echo "  (none)"

# Persistence: try netfilter-persistent, fallback iptables-save to file.
if command -v netfilter-persistent >/dev/null 2>&1; then
    sudo netfilter-persistent save
    echo "Persisted via netfilter-persistent."
elif [ -d /etc/iptables ]; then
    sudo iptables-save | sudo tee /etc/iptables/rules.v4 >/dev/null
    echo "Persisted to /etc/iptables/rules.v4."
else
    echo "WARN: no persistence mechanism found — rules will be lost on reboot."
    echo "      Install netfilter-persistent: apt-get install -y iptables-persistent"
fi

echo "Done."
