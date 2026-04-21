-- Down migration for 000111_backfill_messages_operator_id.
--
-- Intentionally a no-op. Reverting a backfill by nulling operator_id would
-- destroy data that new code paths (post-migration 000111) may already rely
-- on, and we cannot distinguish backfilled values from values written by
-- live traffic after the up migration ran.
--
-- If a rollback of the business decision is ever required, do it as an
-- explicit forward migration with a documented predicate, not here.

SELECT 1;
