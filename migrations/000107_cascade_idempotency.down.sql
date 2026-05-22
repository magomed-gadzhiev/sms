-- migrations/000107_cascade_idempotency.down.sql
ALTER TABLE delivery_attempts
    DROP CONSTRAINT IF EXISTS uq_delivery_step;
