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
    echo "DESTRUCTIVE — postgres data будет полностью заменён snapshot'ом."

    if [ -t 0 ]; then
        echo "Continue? (yes/N)"
        read -r answer
        if [ "$answer" != "yes" ]; then
            echo "Aborted by operator."
            exit 1
        fi
    else
        echo "(non-interactive mode: proceeding with restore — caller responsibility)"
    fi

    # Resolve postgres data volume name. Compose project name = directory
    # parent имя (deployments), volume = "<project>_postgres-data".
    VOL=$(docker volume ls --format '{{.Name}}' | grep -E '_postgres-data$' | head -1)
    if [ -z "$VOL" ]; then
        echo "ERROR: postgres-data volume not found via 'docker volume ls'"
        exit 1
    fi
    echo "Using volume: $VOL"

    echo "Stopping postgres ..."
    $COMPOSE stop postgres

    echo "Restoring snapshot offline (alpine helper) ..."
    echo "  snapshot: $SNAPSHOT_DIR/base.tar.gz → volume: $VOL"
    # alpine sh -c использует свой `set -eu` потому что docker subprocess
    # не наследует pipefail/errexit от parent shell'а. UID/GID 999:999 —
    # дефолт postgres:15 image (см. также restore-test.sh).
    # Glob `.[!.]*` матчит dotfiles за исключением `.` и `..` (safe для PGDATA).
    docker run --rm \
        -v "$VOL:/data" \
        -v "$SNAPSHOT_DIR:/backup:ro" \
        alpine sh -c '
            set -eu
            rm -rf /data/* /data/.[!.]* 2>/dev/null || true
            tar -xzf /backup/base.tar.gz -C /data
            chown -R 999:999 /data
        '

    echo "Starting postgres ..."
    $COMPOSE up -d postgres

    # Wait-loop вместо hardcoded sleep: SELECT 1 вернёт OK как только pg
    # принимает connections, что соответствует «recovery complete + ready».
    # 60 итераций × 2s = 120s max. Под incident-stress'ом explicit timeout
    # лучше чем silent advance с unfinished WAL replay (см. code-review).
    echo "Waiting for postgres to accept connections ..."
    POSTGRES_READY=0
    for i in $(seq 1 60); do
        if $COMPOSE exec -T postgres psql -U smpp -d smpp_db -c "SELECT 1" >/dev/null 2>&1; then
            POSTGRES_READY=1
            break
        fi
        echo "  waiting ($i/60) ..."
        sleep 2
    done
    if [ "$POSTGRES_READY" != 1 ]; then
        echo "ERROR: postgres failed to accept connections within 120s after restore"
        echo "Manual recovery: verify tar (tar -tzf $SNAPSHOT_DIR/base.tar.gz | head),"
        echo "проверить docker volume на stale mounts ($COMPOSE logs postgres | tail -50)."
        exit 1
    fi
    echo "DB restore OK"
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
