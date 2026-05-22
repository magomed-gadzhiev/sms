-- 000115_cascade_idempotency_partitioned.down.sql

ALTER TABLE delivery_attempts
    DROP CONSTRAINT IF EXISTS uq_delivery_step;
