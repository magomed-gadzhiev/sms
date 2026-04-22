-- migrations/000107_cascade_idempotency.up.sql

-- Pre-flight: убедиться, что дубликатов нет (выполнить вручную перед миграцией):
-- SELECT delivery_id, step_order, COUNT(*)
-- FROM delivery_attempts
-- GROUP BY delivery_id, step_order
-- HAVING COUNT(*) > 1;

ALTER TABLE delivery_attempts
    ADD CONSTRAINT uq_delivery_step UNIQUE (delivery_id, step_order);
