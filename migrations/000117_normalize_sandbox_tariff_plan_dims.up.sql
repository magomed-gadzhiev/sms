-- Normalize legacy dimension values in reseller_tariff_plans so the
-- network tariff editor (which matches filters exactly: country_id=$,
-- sender_category=$, traffic_type=$) can find the sandbox-seeded plans.
--
-- Pre-state in sandbox DB (2026-04-23):
--   * Some rows had country_id IS NULL (wildcard country from early seeds).
--     The editor does NOT treat NULL as wildcard — it requires an exact
--     country_id match — so those rows were effectively invisible.
--   * sender_category carried legacy values 'standard' / 'free_registered' /
--     'shared' from the pre-UI schema, while the portal UI only offers
--     'paid_registered' / 'paid_unregistered' / 'free'. No CHECK constraint
--     existed on the column, so the drift went unnoticed.
--
-- This migration rewrites existing rows to the UI-visible values. No new
-- CHECK constraint is added here — that is a separate design decision.

BEGIN;

UPDATE reseller_tariff_plans
   SET country_id = (SELECT id FROM countries WHERE iso_code = 'RU')
 WHERE country_id IS NULL;

UPDATE reseller_tariff_plans
   SET sender_category = 'paid_registered'
 WHERE sender_category = 'standard';

UPDATE reseller_tariff_plans
   SET sender_category = 'free'
 WHERE sender_category = 'free_registered';

UPDATE reseller_tariff_plans
   SET sender_category = 'paid_unregistered'
 WHERE sender_category = 'shared';

COMMIT;
