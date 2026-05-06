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
    echo "WARNING: this will DROP current postgres state. Continue? (y/N)"
    read -r answer
    if [ "$answer" != "y" ] && [ "$answer" != "Y" ]; then
        echo "Aborted DB restore. Code rollback only."
    else
        echo "Stopping postgres..."
        $COMPOSE stop postgres
        echo "Detecting postgres data volume..."
        docker volume ls | grep postgres | head -3
        VOLUME_NAME=$(docker volume ls --format '{{.Name}}' | grep postgres-data | head -1)
        if [ -z "$VOLUME_NAME" ]; then
            echo "ERROR: postgres volume not found. Manual cleanup required."
            exit 1
        fi
        echo "Removing volume: $VOLUME_NAME"
        docker volume rm "$VOLUME_NAME" || true
        echo "Starting postgres for restore..."
        $COMPOSE up -d postgres
        sleep 15
        echo "Restoring from snapshot..."
        zcat "$SNAPSHOT_DIR/base.tar.gz" | $COMPOSE exec -T postgres tar -xf - -C /var/lib/postgresql/data
        $COMPOSE restart postgres
        sleep 10
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
