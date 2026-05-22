#!/bin/bash
# Daily pg_basebackup для prod. Запускается cron'ом из host crontab
# (см. docs/ops/prod-cutover-runbook.md Pre-cutover для setup).
set -euo pipefail

cd /opt/sms

BACKUP_DIR="/var/backups/sms-pg"
RETENTION_DAYS=${PG_BACKUP_RETENTION_DAYS:-7}
DC="docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod"

mkdir -p "$BACKUP_DIR"

DATE=$(date +%Y%m%d-%H%M%S)
TARGET="$BACKUP_DIR/daily-$DATE.tar.gz"

echo "[$(date)] === pg_basebackup → $TARGET ==="
$DC exec -T postgres pg_basebackup -U smpp -D - -Ft -P -X fetch 2>>"$BACKUP_DIR/backup.log" | gzip > "$TARGET"

# Verify size — пустой/маленький snapshot = тревога.
SIZE=$(stat -c%s "$TARGET" 2>/dev/null || echo 0)
if [ "$SIZE" -lt 1048576 ]; then
    echo "[$(date)] ERROR: backup size $SIZE bytes < 1MB — likely corrupted" >&2
    rm -f "$TARGET"
    exit 1
fi

echo "[$(date)] Snapshot OK: $TARGET ($((SIZE / 1024 / 1024)) MB)"

# Retention cleanup — старше RETENTION_DAYS.
DELETED=$(find "$BACKUP_DIR" -name 'daily-*.tar.gz' -mtime +"$RETENTION_DAYS" -delete -print | wc -l)
echo "[$(date)] Retention cleanup: removed $DELETED snapshots older than $RETENTION_DAYS days"
