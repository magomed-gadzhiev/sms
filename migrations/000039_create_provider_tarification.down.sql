DROP TRIGGER IF EXISTS update_provider_usage_counters_updated_at ON provider_usage_counters;
DROP TRIGGER IF EXISTS update_provider_tariff_plans_updated_at ON provider_tariff_plans;
DROP TABLE IF EXISTS provider_tarification_log;
DROP TABLE IF EXISTS provider_usage_counters;
DROP TABLE IF EXISTS provider_tariff_tiers;
DROP TABLE IF EXISTS provider_tariff_periods;
DROP TABLE IF EXISTS provider_tariff_plans;
