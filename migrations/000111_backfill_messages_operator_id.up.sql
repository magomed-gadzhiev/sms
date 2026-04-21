-- Backfill messages.operator_id for historical rows using operator_prefixes.
--
-- Matching rules mirror repository/operator_prefix_repository.go (FindByNumber):
--   destination LIKE prefix || '%'
--   tie-break: ORDER BY length(prefix) DESC, priority DESC
--   only active prefixes participate (partial index idx_operator_prefixes_active)
--
-- messages is partitioned by created_at and large (~10M rows). We do a batched
-- UPDATE inside a single DO block with FOR UPDATE SKIP LOCKED so long-running
-- backfill does not block live INSERTs on current partition.
--
-- Safe to re-run: filter `operator_id IS NULL` makes it idempotent.

DO $$
DECLARE
    batch_size INT := 10000;
    updated    INT;
    total      BIGINT := 0;
BEGIN
    LOOP
        WITH batch AS (
            SELECT id, created_at
            FROM messages
            WHERE operator_id IS NULL
            LIMIT batch_size
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
          AND m.created_at = b.created_at;

        GET DIAGNOSTICS updated = ROW_COUNT;
        total := total + updated;

        EXIT WHEN updated = 0;

        -- Commit visibility of the batch. In a DO block we can't COMMIT on
        -- older PG, but on 15+ we could. We instead rely on the outer
        -- migration transaction; if running outside a transaction (psql \i
        -- with AUTOCOMMIT on), each UPDATE is its own txn.
        PERFORM pg_sleep(0);
    END LOOP;

    RAISE NOTICE 'backfill_messages_operator_id: updated % rows', total;
END $$;
