-- migrations/000082_migrate_tariff_plans_to_periods.down.sql
-- Remove rows migrated from old tariff_plans (scope_priority=30, operator_id IS NOT NULL, client_id IS NULL)

DELETE FROM tariff_tiers_new
WHERE tariff_period_id IN (
  SELECT id FROM tariff_periods_new
  WHERE scope_priority = 30
    AND operator_id IS NOT NULL
    AND client_id IS NULL
    AND traffic_type IS NULL
);

DELETE FROM tariff_periods_new
WHERE scope_priority = 30
  AND operator_id IS NOT NULL
  AND client_id IS NULL
  AND traffic_type IS NULL;
