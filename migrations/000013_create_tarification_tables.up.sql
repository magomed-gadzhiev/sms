-- Tarification service tables

CREATE TABLE IF NOT EXISTS sender_registrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL,
    operator_id UUID NOT NULL,
    sender_name VARCHAR(11) NOT NULL,
    type VARCHAR(20) NOT NULL CHECK (type IN ('paid', 'free')),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('active', 'pending', 'expired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_sender_registrations_unique ON sender_registrations(client_id, operator_id, sender_name);
CREATE INDEX idx_sender_registrations_client_operator ON sender_registrations(client_id, operator_id);
CREATE INDEX idx_sender_registrations_operator_status ON sender_registrations(operator_id, status);

CREATE TABLE IF NOT EXISTS tariff_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_id UUID NOT NULL,
    sender_category VARCHAR(30) NOT NULL CHECK (sender_category IN ('shared', 'paid_registered', 'free_registered')),
    strategy VARCHAR(30) NOT NULL CHECK (strategy IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold')),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_tariff_plans_unique_active ON tariff_plans(operator_id, sender_category) WHERE active = true;
CREATE INDEX idx_tariff_plans_operator ON tariff_plans(operator_id);

CREATE TABLE IF NOT EXISTS tariff_periods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tariff_plan_id UUID NOT NULL REFERENCES tariff_plans(id) ON DELETE CASCADE,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_date > start_date)
);

-- Exclusion constraint to prevent overlapping periods within the same tariff plan
CREATE EXTENSION IF NOT EXISTS btree_gist;
ALTER TABLE tariff_periods ADD CONSTRAINT tariff_periods_no_overlap
    EXCLUDE USING gist (tariff_plan_id WITH =, daterange(start_date, end_date, '[]') WITH &&);

CREATE INDEX idx_tariff_periods_plan ON tariff_periods(tariff_plan_id);
CREATE INDEX idx_tariff_periods_dates ON tariff_periods(tariff_plan_id, start_date, end_date);

CREATE TABLE IF NOT EXISTS tariff_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tariff_period_id UUID NOT NULL REFERENCES tariff_periods(id) ON DELETE CASCADE,
    from_count INTEGER NOT NULL DEFAULT 0 CHECK (from_count >= 0),
    price_per_segment NUMERIC(20,6) NOT NULL CHECK (price_per_segment >= 0)
);

CREATE UNIQUE INDEX idx_tariff_tiers_unique ON tariff_tiers(tariff_period_id, from_count);

CREATE TABLE IF NOT EXISTS pricing_periods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tariff_period_id UUID NOT NULL REFERENCES tariff_periods(id) ON DELETE CASCADE,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_date > start_date)
);

ALTER TABLE pricing_periods ADD CONSTRAINT pricing_periods_no_overlap
    EXCLUDE USING gist (tariff_period_id WITH =, daterange(start_date, end_date, '[]') WITH &&);

CREATE INDEX idx_pricing_periods_tariff_period ON pricing_periods(tariff_period_id, start_date, end_date);

CREATE TABLE IF NOT EXISTS prepaid_fees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tariff_plan_id UUID NOT NULL REFERENCES tariff_plans(id),
    tariff_period_id UUID NOT NULL REFERENCES tariff_periods(id),
    amount NUMERIC(20,6) NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,
    charged BOOLEAN NOT NULL DEFAULT false,
    charged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS usage_counters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL,
    tariff_plan_id UUID NOT NULL REFERENCES tariff_plans(id),
    tariff_period_id UUID NOT NULL REFERENCES tariff_periods(id),
    segment_count INTEGER NOT NULL DEFAULT 0 CHECK (segment_count >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_usage_counters_unique ON usage_counters(client_id, tariff_plan_id, tariff_period_id);

-- Tarification log with monthly partitioning (same pattern as messages table)
CREATE TABLE IF NOT EXISTS tarification_log (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL,
    message_id UUID NOT NULL,
    operator_id UUID NOT NULL,
    sender_category VARCHAR(30) NOT NULL,
    strategy VARCHAR(30) NOT NULL,
    tariff_plan_id UUID NOT NULL,
    tariff_period_id UUID NOT NULL,
    segment_count INTEGER NOT NULL,
    price_per_segment NUMERIC(20,6) NOT NULL,
    total_amount NUMERIC(20,6) NOT NULL,
    recalc_amount NUMERIC(20,6),
    idempotency_key VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create partitions for 2026
CREATE TABLE tarification_log_2026_01 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE tarification_log_2026_02 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE tarification_log_2026_03 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE tarification_log_2026_04 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE tarification_log_2026_05 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE tarification_log_2026_06 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE tarification_log_2026_07 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE tarification_log_2026_08 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE tarification_log_2026_09 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE tarification_log_2026_10 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE tarification_log_2026_11 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE tarification_log_2026_12 PARTITION OF tarification_log
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

CREATE INDEX idx_tarification_log_client ON tarification_log(client_id, created_at);
CREATE INDEX idx_tarification_log_message ON tarification_log(message_id);
CREATE UNIQUE INDEX idx_tarification_log_idempotency ON tarification_log(idempotency_key, created_at);

-- Triggers
CREATE TRIGGER update_sender_registrations_updated_at
    BEFORE UPDATE ON sender_registrations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tariff_plans_updated_at
    BEFORE UPDATE ON tariff_plans
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_usage_counters_updated_at
    BEFORE UPDATE ON usage_counters
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
