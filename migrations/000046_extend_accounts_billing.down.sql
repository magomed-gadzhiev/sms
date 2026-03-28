DROP INDEX IF EXISTS idx_accounts_low_balance;
DROP INDEX IF EXISTS idx_accounts_frozen;

ALTER TABLE accounts
    DROP COLUMN IF EXISTS frozen,
    DROP COLUMN IF EXISTS frozen_at,
    DROP COLUMN IF EXISTS frozen_by,
    DROP COLUMN IF EXISTS credit_limit,
    DROP COLUMN IF EXISTS low_balance_threshold;
