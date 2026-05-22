-- No-op placeholder.
--
-- Original intent: bulk-backfill messages.operator_id for historical rows via
-- operator_prefixes longest-prefix match. A DO block wrapping UPDATE ... FROM
-- batch with FOR UPDATE SKIP LOCKED hit two problems on the current sandbox:
--   1. Single migration transaction = all 10M updated tuples held until commit,
--      bloating the partitioned table + indexes past available disk.
--   2. Even with SKIP LOCKED, long-running txn blocks VACUUM of new partitions.
--
-- Backfill moved to scripts/backfill_messages_operator_id.sh which runs batches
-- in separate transactions with autocommit. New messages written after the
-- pipeline fix (SentMessage.OperatorID writeback in status stage) already get
-- operator_id set correctly — backfill only matters for historical analytics.

SELECT 1;
