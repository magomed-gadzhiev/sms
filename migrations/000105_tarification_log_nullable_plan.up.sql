-- migrations/000105_tarification_log_nullable_plan.up.sql
-- Phase 3 follow-up: unified hot path не имеет tariff_plan_id/period_id,
-- только source_rule_id из price_rules. Поэтому plan/period становятся
-- nullable, добавляется новая колонка. FK на price_rules не ставим —
-- правила могут удаляться, нельзя терять исторические log-строки.
-- См. docs/superpowers/specs/2026-04-19-tarification-log-nullable-plan-design.md.
--
-- LOCK NOTE: ALTER TABLE на partitioned parent в PG 12+ каскадно применяется
-- к дочерним partitions. DROP NOT NULL/ADD COLUMN без default — только
-- metadata change, без rewrite строк. Ожидаемый AccessExclusive lock — sub-second
-- на каждую партицию, в пике ~2k writes/sec приостанавливается суммарно <1s.
-- Планировать применение миграции вне пиков записи (не обязательно, но аккуратнее).

BEGIN;

ALTER TABLE tarification_log
    ALTER COLUMN tariff_plan_id DROP NOT NULL,
    ALTER COLUMN tariff_period_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS source_rule_id UUID NULL;

CREATE INDEX IF NOT EXISTS idx_tarification_log_source_rule
    ON tarification_log(source_rule_id)
    WHERE source_rule_id IS NOT NULL;

COMMIT;
