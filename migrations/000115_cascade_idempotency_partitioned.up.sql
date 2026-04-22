-- 000115_cascade_idempotency_partitioned.up.sql
--
-- Фикс миграции 000107: delivery_attempts — partitioned-by-created_at,
-- поэтому UNIQUE должен включать все partitioning columns. Сама 000107
-- на prod-схеме не применилась (см. commit a6eec05 predecessor).
--
-- Ограничение: seman­тика идемпотентности ослаблена — дубликат
-- (delivery_id, step_order) с другим created_at пройдёт. Компенсируется
-- на уровне приложения (см. attempt_repo.go: ON CONFLICT расширен).

ALTER TABLE delivery_attempts
    ADD CONSTRAINT uq_delivery_step UNIQUE (delivery_id, step_order, created_at);
