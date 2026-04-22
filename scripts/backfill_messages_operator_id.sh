#!/usr/bin/env bash
# Backfill messages.operator_id for historical rows.
#
# Why a shell script instead of a migration: a DO block runs inside one
# transaction. With 10M rows that balloons WAL + dead tuples past available
# disk on the sandbox. Here each batch is its own psql invocation = its own
# autocommit transaction = bounded WAL footprint per batch.
#
# Matching rules mirror FindByNumber in operator_prefix_repository.go:
#   destination LIKE prefix || '%' with longest-prefix, priority-tie-break,
#   active-only.
#
# Usage (on sms-server):
#   docker exec -i postgres bash -lc "$(cat scripts/backfill_messages_operator_id.sh)"
#
# Safe to interrupt + re-run: WHERE operator_id IS NULL keeps it idempotent.

set -euo pipefail

BATCH_SIZE=${BATCH_SIZE:-5000}
SLEEP_MS=${SLEEP_MS:-100}
TOTAL=0

while :; do
    UPDATED=$(psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postgres}" -t -A -c "
        WITH batch AS (
            SELECT id, created_at
            FROM messages
            WHERE operator_id IS NULL
            LIMIT ${BATCH_SIZE}
            FOR UPDATE SKIP LOCKED
        )
        UPDATE messages m
        SET operator_id = (
            SELECT op.operator_id
            FROM operator_prefixes op
            WHERE op.active
              AND m.destination LIKE op.prefix || '%'
            ORDER BY length(op.prefix) DESC, op.priority DESC
            LIMIT 1
        )
        FROM batch b
        WHERE m.id = b.id
          AND m.created_at = b.created_at
        RETURNING 1;
    " | wc -l)

    UPDATED=$(echo "$UPDATED" | tr -d '[:space:]')
    [ "$UPDATED" -eq 0 ] && break
    TOTAL=$((TOTAL + UPDATED))
    echo "batch: +${UPDATED}  total: ${TOTAL}"
    # Yield a bit to let autovacuum catch up
    sleep "$(awk "BEGIN { print ${SLEEP_MS}/1000 }")"
done

echo "done: backfilled ${TOTAL} rows"
