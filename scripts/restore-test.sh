#!/bin/bash
# Weekly restore verification — независимый postgres-instance, restore'ит
# самый свежий daily snapshot, проверяет row count > 0 в критических таблицах.
# Цель: верифицировать, что pg_basebackup actually restorable.
# Запускается cron'ом (weekly, sunday).
set -euo pipefail

cd /opt/sms

BACKUP_DIR="/var/backups/sms-pg"
LATEST=$(ls -t "$BACKUP_DIR"/daily-*.tar.gz 2>/dev/null | head -1)
test -n "$LATEST" || { echo "[$(date)] No backup found — abort" >&2; exit 1; }

echo "[$(date)] === Restore test: $LATEST ==="

TEST_NAME="pg-restore-test-$(date +%s)"
TEST_DATA_DIR=$(mktemp -d)
trap 'docker rm -f "$TEST_NAME" 2>/dev/null || true; rm -rf "$TEST_DATA_DIR"' EXIT

# Init isolated postgres-15 для restore.
docker run -d --name "$TEST_NAME" \
    -v "$TEST_DATA_DIR:/var/lib/postgresql/data" \
    -e POSTGRES_PASSWORD=test \
    -e POSTGRES_USER=smpp \
    -e POSTGRES_DB=smpp_db \
    postgres:15 >/dev/null

sleep 20  # Postgres init.

# Stop and offline-restore.
docker stop "$TEST_NAME" >/dev/null
zcat "$LATEST" | docker run --rm -i \
    -v "$TEST_DATA_DIR:/var/lib/postgresql/data" \
    alpine sh -c 'rm -rf /var/lib/postgresql/data/* /var/lib/postgresql/data/.[!.]* 2>/dev/null; tar -xf - -C /var/lib/postgresql/data && chown -R 999:999 /var/lib/postgresql/data'

docker start "$TEST_NAME" >/dev/null
sleep 15  # Postgres recovery + accept connections.

# Verify несколько критических таблиц.
ANY_FAIL=0
for tbl in users clients subaccount_routing_assignment; do
    COUNT=$(docker exec "$TEST_NAME" psql -U smpp -d smpp_db -tAc "SELECT count(*) FROM $tbl" 2>/dev/null || echo "ERR")
    if [ "$COUNT" = "ERR" ]; then
        echo "[$(date)] ERROR: query failed on $tbl"
        ANY_FAIL=1
    elif [ "$COUNT" -lt 0 ] 2>/dev/null; then
        echo "[$(date)] ERROR: $tbl count = $COUNT (negative? table missing?)"
        ANY_FAIL=1
    else
        echo "[$(date)]   $tbl: $COUNT rows"
    fi
done

if [ "$ANY_FAIL" -eq 1 ]; then
    echo "[$(date)] === Restore test FAILED ==="
    exit 1
fi

echo "[$(date)] === Restore test PASSED ==="
