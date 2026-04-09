-- Rollback hierarchical tariff periods system
DROP TABLE IF EXISTS provider_cost_tiers;
DROP TABLE IF EXISTS provider_cost_periods;
DROP TABLE IF EXISTS tariff_tiers_new;
DROP TABLE IF EXISTS tariff_periods_new;
