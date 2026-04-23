-- Reverse of 000117. Legacy values are reinstated only for rows still
-- matching the post-migration state on sandbox; production rows that
-- predated 000117 with 'paid_registered' etc. are indistinguishable from
-- remapped rows, so a clean rollback is not possible. This down script is
-- therefore best-effort for sandbox only.

BEGIN;

-- We can't recover the original NULL country, so skip country_id.

UPDATE reseller_tariff_plans
   SET sender_category = 'standard'
 WHERE sender_category = 'paid_registered';

UPDATE reseller_tariff_plans
   SET sender_category = 'free_registered'
 WHERE sender_category = 'free';

UPDATE reseller_tariff_plans
   SET sender_category = 'shared'
 WHERE sender_category = 'paid_unregistered';

COMMIT;
