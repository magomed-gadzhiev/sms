-- Aggregator infrastructure quotas (pool for all sub-accounts)
CREATE TABLE aggregator_quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    segment_limit BIGINT NOT NULL CHECK (segment_limit > 0),
    segments_used BIGINT NOT NULL DEFAULT 0 CHECK (segments_used >= 0),
    overage_rate NUMERIC(10,4) NOT NULL CHECK (overage_rate >= 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'RUB',
    auto_renew BOOLEAN NOT NULL DEFAULT true,
    notified_80pct BOOLEAN NOT NULL DEFAULT false,
    notified_100pct BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_aggregator_quota_period UNIQUE(aggregator_id, period_start),
    CONSTRAINT chk_period_range CHECK (period_end > period_start)
);

CREATE INDEX idx_aggregator_quotas_active ON aggregator_quotas(aggregator_id, period_start, period_end);

-- Billing mode and spending limits on clients (for sub-accounts)
ALTER TABLE clients
    ADD COLUMN billing_mode VARCHAR(20) NOT NULL DEFAULT 'own'
        CHECK (billing_mode IN ('own', 'aggregator', 'hybrid')),
    ADD COLUMN spending_limit_monthly NUMERIC(12,2),
    ADD COLUMN spending_limit_daily NUMERIC(12,2);

-- Attribution on transactions (which sub-account triggered the charge on aggregator's balance)
ALTER TABLE transactions
    ADD COLUMN attributed_sub_account_id UUID REFERENCES clients(id);

CREATE INDEX idx_transactions_attributed ON transactions(attributed_sub_account_id, created_at)
    WHERE attributed_sub_account_id IS NOT NULL;
