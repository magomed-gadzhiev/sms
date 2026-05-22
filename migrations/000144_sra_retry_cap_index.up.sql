CREATE INDEX idx_sra_pending_retry_active ON subaccount_routing_assignment (last_materialize_error_at) WHERE last_materialize_error_at IS NOT NULL AND materialize_retry_count < 100;
