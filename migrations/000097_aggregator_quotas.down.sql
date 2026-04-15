DROP INDEX IF EXISTS idx_transactions_attributed;
ALTER TABLE transactions DROP COLUMN IF EXISTS attributed_sub_account_id;

ALTER TABLE clients DROP COLUMN IF EXISTS spending_limit_daily;
ALTER TABLE clients DROP COLUMN IF EXISTS spending_limit_monthly;
ALTER TABLE clients DROP COLUMN IF EXISTS billing_mode;

DROP TABLE IF EXISTS aggregator_quotas;
