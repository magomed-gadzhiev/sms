-- migrations/000082_migrate_tariff_plans_to_periods.up.sql
-- Copy existing tariff_plans + tariff_periods into tariff_periods_new.
-- Preserves period IDs so tariff_tiers can be migrated with same FK.

INSERT INTO tariff_periods_new (
  id, country_id, operator_id, sender_category, traffic_type, client_id,
  scope_key, scope_priority, strategy,
  start_date, end_date, created_at
)
SELECT
  tp.id,
  o.country_id,
  plan.operator_id,
  plan.sender_category,
  NULL::text    AS traffic_type,
  NULL::uuid    AS client_id,
  'country:' || o.country_id::text
    || '|operator:' || plan.operator_id::text
    || '|sender_category:' || plan.sender_category AS scope_key,
  30            AS scope_priority,
  plan.strategy,
  tp.start_date,
  tp.end_date,
  tp.created_at
FROM tariff_periods tp
JOIN tariff_plans plan ON plan.id = tp.tariff_plan_id
JOIN operators o ON o.id = plan.operator_id
ON CONFLICT DO NOTHING;

-- Migrate tiers: copy from tariff_tiers where the period was migrated
INSERT INTO tariff_tiers_new (id, tariff_period_id, from_count, price_per_segment, created_at)
SELECT t.id, t.tariff_period_id, t.from_count, t.price_per_segment, t.created_at
FROM tariff_tiers t
WHERE EXISTS (SELECT 1 FROM tariff_periods_new WHERE id = t.tariff_period_id)
ON CONFLICT DO NOTHING;
