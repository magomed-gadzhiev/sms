-- migrations/000098_reseller_tariff_tables.down.sql
BEGIN;
DROP TABLE IF EXISTS sub_account_template_assignments CASCADE;
DROP TABLE IF EXISTS reseller_tariff_tiers CASCADE;
DROP TABLE IF EXISTS reseller_tariff_periods CASCADE;
DROP TABLE IF EXISTS reseller_tariff_plans CASCADE;
DROP TABLE IF EXISTS reseller_tariff_templates CASCADE;
COMMIT;
