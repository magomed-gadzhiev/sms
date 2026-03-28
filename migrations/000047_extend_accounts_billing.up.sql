ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS frozen BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS frozen_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS frozen_by TEXT,
    ADD COLUMN IF NOT EXISTS credit_limit NUMERIC(20,6) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS low_balance_threshold NUMERIC(20,6) NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_accounts_frozen ON accounts(frozen) WHERE frozen = true;
CREATE INDEX IF NOT EXISTS idx_accounts_low_balance ON accounts(client_id)
    WHERE low_balance_threshold > 0;
