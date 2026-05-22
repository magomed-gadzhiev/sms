CREATE TABLE IF NOT EXISTS provider_tariff_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id),
    operator_id UUID NOT NULL REFERENCES operators(id),
    strategy VARCHAR(50) NOT NULL CHECK (strategy IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold')),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_provider_tariff_plans_unique_active
    ON provider_tariff_plans(provider_id, operator_id) WHERE active = true;

CREATE TABLE IF NOT EXISTS provider_tariff_periods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_tariff_plan_id UUID NOT NULL REFERENCES provider_tariff_plans(id) ON DELETE CASCADE,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_date > start_date)
);

ALTER TABLE provider_tariff_periods ADD CONSTRAINT provider_tariff_periods_no_overlap
    EXCLUDE USING gist (provider_tariff_plan_id WITH =, daterange(start_date, end_date, '[]') WITH &&);

CREATE INDEX idx_provider_tariff_periods_plan ON provider_tariff_periods(provider_tariff_plan_id);

CREATE TABLE IF NOT EXISTS provider_tariff_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_tariff_period_id UUID NOT NULL REFERENCES provider_tariff_periods(id) ON DELETE CASCADE,
    from_count INT NOT NULL DEFAULT 0 CHECK (from_count >= 0),
    price_per_segment NUMERIC(20,6) NOT NULL CHECK (price_per_segment >= 0),
    UNIQUE(provider_tariff_period_id, from_count)
);

CREATE TABLE IF NOT EXISTS provider_usage_counters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_tariff_plan_id UUID NOT NULL REFERENCES provider_tariff_plans(id),
    provider_tariff_period_id UUID NOT NULL REFERENCES provider_tariff_periods(id),
    segment_count INT NOT NULL DEFAULT 0 CHECK (segment_count >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider_tariff_plan_id, provider_tariff_period_id)
);

-- Provider tarification log with monthly partitioning
CREATE TABLE IF NOT EXISTS provider_tarification_log (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    provider_id UUID NOT NULL,
    operator_id UUID NOT NULL,
    client_id UUID NOT NULL,
    message_id UUID NOT NULL,
    segment_count INT NOT NULL,
    price_per_segment NUMERIC(20,6) NOT NULL,
    total_cost NUMERIC(20,6) NOT NULL,
    strategy VARCHAR(50) NOT NULL,
    provider_tariff_plan_id UUID NOT NULL,
    provider_tariff_period_id UUID NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- 12 monthly partitions for 2026
CREATE TABLE provider_tarification_log_2026_01 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE provider_tarification_log_2026_02 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE provider_tarification_log_2026_03 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE provider_tarification_log_2026_04 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE provider_tarification_log_2026_05 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE provider_tarification_log_2026_06 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE provider_tarification_log_2026_07 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE provider_tarification_log_2026_08 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE provider_tarification_log_2026_09 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE provider_tarification_log_2026_10 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE provider_tarification_log_2026_11 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE provider_tarification_log_2026_12 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

CREATE INDEX idx_provider_tarification_log_provider ON provider_tarification_log(provider_id, created_at);
CREATE INDEX idx_provider_tarification_log_client ON provider_tarification_log(client_id, created_at);
CREATE INDEX idx_provider_tarification_log_message ON provider_tarification_log(message_id);
CREATE UNIQUE INDEX idx_provider_tarification_log_idempotency ON provider_tarification_log(idempotency_key, created_at);

CREATE TRIGGER update_provider_tariff_plans_updated_at
    BEFORE UPDATE ON provider_tariff_plans
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_provider_usage_counters_updated_at
    BEFORE UPDATE ON provider_usage_counters
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
