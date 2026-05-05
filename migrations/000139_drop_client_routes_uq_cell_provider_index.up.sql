-- Follow-up к 000138: uq_cell_provider в sandbox оказался не CONSTRAINT,
-- а отдельным UNIQUE INDEX (`pg_indexes` подтвердил, `pg_constraint` пуст).
-- DROP CONSTRAINT IF EXISTS из 000138 был no-op. Дропаем индекс напрямую.
--
-- Семантика та же: убрать sandbox-only защиту, которая блокировала override
-- на тот же (provider_id, route_type), что в template (см. комментарий 000138).
DROP INDEX IF EXISTS uq_cell_provider;
