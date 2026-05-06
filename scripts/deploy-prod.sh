#!/bin/bash
# Production deploy procedure.
# Usage:
#   ./scripts/deploy-prod.sh                  # full stack
#   ./scripts/deploy-prod.sh --service worker # один service
#
# Запускается из /opt/sms на prod-host через ssh.
set -euo pipefail

cd /opt/sms

# Pre-flight checks.
echo "=== Pre-flight ==="
test -f deployments/.env.prod || { echo "ERROR: .env.prod missing — abort"; exit 1; }
test -d /var/backups/sms-pg || { echo "Creating /var/backups/sms-pg ..."; sudo mkdir -p /var/backups/sms-pg && sudo chown "$USER" /var/backups/sms-pg; }

COMPOSE="docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod"

# Resolve POSTGRES_PASSWORD для pg_basebackup invocation.
set -a
. deployments/.env.prod
set +a

# Snapshot DB перед deploy.
echo ""
echo "=== Backup DB (pg_basebackup) ==="
SNAPSHOT_DIR="/var/backups/sms-pg/pre-deploy-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$SNAPSHOT_DIR"
$COMPOSE exec -T postgres \
    pg_basebackup -U smpp -D - -Ft -P -X fetch | gzip > "$SNAPSHOT_DIR/base.tar.gz"

# Verify size — пустой/маленький snapshot = тревога.
SIZE=$(stat -c%s "$SNAPSHOT_DIR/base.tar.gz")
if [ "$SIZE" -lt 1048576 ]; then
    echo "ERROR: backup size $SIZE bytes < 1MB — likely corrupted"
    exit 1
fi
echo "Snapshot OK: $SNAPSHOT_DIR/base.tar.gz ($((SIZE / 1024 / 1024)) MB)"

# Capture текущий git ref для rollback.
PREV_REF=$(git rev-parse HEAD)
echo "$PREV_REF" > "$SNAPSHOT_DIR/prev_ref.txt"
echo "Previous ref: $PREV_REF (saved for rollback)"

# Pull latest.
echo ""
echo "=== Git pull ==="
git fetch origin
TARGET_REF=${DEPLOY_REF:-$(git rev-parse origin/master)}
git checkout "$TARGET_REF"
echo "Deploying ref: $TARGET_REF"

# Apply migrations.
echo ""
echo "=== Migrations ==="
docker run --rm \
    -v /opt/sms/migrations:/migrations \
    --network deployments_smpp-network \
    migrate/migrate \
    -path /migrations \
    -database "postgres://smpp:${POSTGRES_PASSWORD}@postgres:5432/smpp_db?sslmode=disable" \
    up

# Deploy.
echo ""
echo "=== Deploy ==="
if [ "${1:-}" = "--service" ] && [ -n "${2:-}" ]; then
    echo "Single-service: $2"
    $COMPOSE up -d --build "$2"
else
    echo "Full stack"
    $COMPOSE up -d --build
fi

# Wait services + health check.
echo ""
echo "=== Health check (60s grace) ==="
sleep 60
if ./scripts/healthcheck.sh; then
    echo ""
    echo "=== Deploy SUCCEEDED ==="
    echo "Snapshot retained: $SNAPSHOT_DIR (manual cleanup after 7 days)"
    echo "Rollback command if needed: ./scripts/rollback-prod.sh $PREV_REF $SNAPSHOT_DIR"
else
    echo ""
    echo "=== Health check FAILED — initiating CODE-ONLY auto-rollback ==="
    echo "(DB restore is destructive + unattended — only manual: ./scripts/rollback-prod.sh $PREV_REF $SNAPSHOT_DIR)"
    # SNAPSHOT_DIR DELIBERATELY NOT passed: rollback с DB restore требует
    # interactive confirmation → hang в auto-deploy. Snapshot retained
    # для manual restore оператором, если требуется откат миграций.
    ./scripts/rollback-prod.sh "$PREV_REF"
    exit 1
fi
