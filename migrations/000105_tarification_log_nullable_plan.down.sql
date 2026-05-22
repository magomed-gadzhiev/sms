-- migrations/000105_tarification_log_nullable_plan.down.sql
-- WARNING: rollback ломается если unified-строки есть (tariff_plan_id IS NULL).
-- Для корректного отката сначала очистить:
--   DELETE FROM tarification_log WHERE tariff_plan_id IS NULL;
-- Или backfill'нуть sentinel UUID.

BEGIN;

DROP INDEX IF EXISTS idx_tarification_log_source_rule;

ALTER TABLE tarification_log
    DROP COLUMN IF EXISTS source_rule_id,
    ALTER COLUMN tariff_plan_id SET NOT NULL,
    ALTER COLUMN tariff_period_id SET NOT NULL;

COMMIT;
