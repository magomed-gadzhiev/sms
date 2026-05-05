DROP INDEX IF EXISTS idx_sra_pending_retry;
ALTER TABLE subaccount_routing_assignment
    DROP COLUMN IF EXISTS materialize_retry_count,
    DROP COLUMN IF EXISTS last_materialize_error_text,
    DROP COLUMN IF EXISTS last_materialize_error_at;
