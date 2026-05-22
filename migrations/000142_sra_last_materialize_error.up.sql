ALTER TABLE subaccount_routing_assignment
    ADD COLUMN last_materialize_error_at TIMESTAMPTZ,
    ADD COLUMN last_materialize_error_text TEXT,
    ADD COLUMN materialize_retry_count INT NOT NULL DEFAULT 0;

CREATE INDEX idx_sra_pending_retry
    ON subaccount_routing_assignment (last_materialize_error_at)
    WHERE last_materialize_error_at IS NOT NULL;
