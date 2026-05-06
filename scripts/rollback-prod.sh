#!/bin/bash
# Production rollback.
# Usage:
#   ./scripts/rollback-prod.sh <previous_git_ref> [<snapshot_dir>]
#
# Без <snapshot_dir> — git revert + redeploy (миграции НЕ откатываются).
# С <snapshot_dir> — full DB restore from snapshot (DESTRUCTIVE).
set -euo pipefail

cd /opt/sms

PREV_REF=${1:?previous git ref required}
SNAPSHOT_DIR=${2:-}

COMPOSE="docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod"

echo "=== Rollback to $PREV_REF ==="
git checkout "$PREV_REF"

if [ -n "$SNAPSHOT_DIR" ] && [ -f "$SNAPSHOT_DIR/base.tar.gz" ]; then
    echo ""
    echo "=== DB Restore from $SNAPSHOT_DIR ==="
    echo ""
    echo "!!! WARNING — UNTESTED MECHANICS !!!"
    echo "Implementer flagged: tar extraction в running postgres некорректна."
    echo "Корректный workflow требует stopped container + offline tar в volume:"
    echo ""
    echo "  $COMPOSE stop postgres"
    echo "  VOL=\$(docker volume ls --format '{{.Name}}' | grep -E 'postgres-data\$' | head -1)"
    echo "  docker run --rm -v \"\$VOL:/data\" -v \"$SNAPSHOT_DIR:/backup:ro\" alpine sh -c \\"
    echo "    'rm -rf /data/* /data/.* 2>/dev/null; tar -xzf /backup/base.tar.gz -C /data && chown -R 999:999 /data'"
    echo "  $COMPOSE up -d postgres"
    echo ""
    echo "TODO Plan 8: implement and end-to-end test offline restore."
    echo "В Plan 7 restore делать ВРУЧНУЮ по командам выше."
    echo ""
    echo "Skip auto-restore (CODE-ONLY rollback). Continue with redeploy? (y/N)"
    if [ -t 0 ]; then
        read -r answer
        if [ "$answer" != "y" ] && [ "$answer" != "Y" ]; then
            echo "Aborted."
            exit 1
        fi
    else
        echo "(non-interactive: auto-continue with code-only rollback)"
    fi
fi

echo ""
echo "=== Redeploy on $PREV_REF ==="
$COMPOSE up -d --build

sleep 30
if ./scripts/healthcheck.sh; then
    echo "=== Rollback SUCCEEDED ==="
else
    echo "=== Rollback FAILED — manual intervention required ==="
    exit 1
fi
