ALTER TABLE accounts DROP COLUMN IF EXISTS frozen;
ALTER TABLE accounts DROP COLUMN IF EXISTS credit_limit;
ALTER TABLE accounts DROP COLUMN IF EXISTS low_balance_threshold;
ALTER TABLE accounts DROP COLUMN IF EXISTS frozen_at;
ALTER TABLE accounts DROP COLUMN IF EXISTS frozen_by;
