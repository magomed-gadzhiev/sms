-- migrations/000098_reseller_tariff_tables.up.sql
BEGIN;

-- 1. Tariff templates (named groups of plans)
CREATE TABLE reseller_tariff_templates (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reseller_id UUID        NOT NULL REFERENCES clients(id),
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    active      BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_reseller_template_unique_name
    ON reseller_tariff_templates (reseller_id, name) WHERE active = true;
CREATE INDEX idx_reseller_template_reseller ON reseller_tariff_templates (reseller_id) WHERE active = true;

CREATE TRIGGER update_reseller_tariff_templates_updated_at
    BEFORE UPDATE ON reseller_tariff_templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 2. Tariff plans (belong to template OR sub-account override)
CREATE TABLE reseller_tariff_plans (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reseller_id     UUID        NOT NULL REFERENCES clients(id),
    template_id     UUID        REFERENCES reseller_tariff_templates(id) ON DELETE CASCADE,
    sub_account_id  UUID        REFERENCES clients(id),
    country_id      UUID,
    operator_id     UUID,
    sender_category VARCHAR(32) NOT NULL DEFAULT 'standard',
    traffic_type    VARCHAR(32) NOT NULL DEFAULT 'any',
    strategy        VARCHAR(30) NOT NULL CHECK (strategy IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold')),
    active          BOOLEAN     NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (template_id IS NOT NULL AND sub_account_id IS NULL) OR
        (template_id IS NULL AND sub_account_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_reseller_plan_template_dims
    ON reseller_tariff_plans (
        template_id,
        COALESCE(country_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(operator_id, '00000000-0000-0000-0000-000000000000'::uuid),
        sender_category, traffic_type
    ) WHERE active = true AND template_id IS NOT NULL;

CREATE UNIQUE INDEX idx_reseller_plan_sub_account_dims
    ON reseller_tariff_plans (
        sub_account_id,
        COALESCE(country_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(operator_id, '00000000-0000-0000-0000-000000000000'::uuid),
        sender_category, traffic_type
    ) WHERE active = true AND sub_account_id IS NOT NULL;

CREATE INDEX idx_reseller_plan_template ON reseller_tariff_plans (template_id) WHERE active = true;
CREATE INDEX idx_reseller_plan_sub_account ON reseller_tariff_plans (sub_account_id) WHERE active = true;
CREATE INDEX idx_reseller_plan_reseller ON reseller_tariff_plans (reseller_id);

CREATE TRIGGER update_reseller_tariff_plans_updated_at
    BEFORE UPDATE ON reseller_tariff_plans
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 3. Tariff periods (date ranges within a plan, no overlaps)
CREATE TABLE reseller_tariff_periods (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tariff_plan_id UUID NOT NULL REFERENCES reseller_tariff_plans(id) ON DELETE CASCADE,
    start_date     DATE NOT NULL,
    end_date       DATE NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (end_date > start_date)
);

ALTER TABLE reseller_tariff_periods ADD CONSTRAINT reseller_periods_no_overlap
    EXCLUDE USING gist (tariff_plan_id WITH =, daterange(start_date, end_date, '[]') WITH &&);

CREATE INDEX idx_reseller_period_plan ON reseller_tariff_periods (tariff_plan_id);
CREATE INDEX idx_reseller_period_dates ON reseller_tariff_periods (tariff_plan_id, start_date, end_date);

-- 4. Tariff tiers (volume-based pricing within a period)
CREATE TABLE reseller_tariff_tiers (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tariff_period_id UUID        NOT NULL REFERENCES reseller_tariff_periods(id) ON DELETE CASCADE,
    from_count       INTEGER     NOT NULL DEFAULT 0 CHECK (from_count >= 0),
    price_per_segment NUMERIC(10,6) NOT NULL CHECK (price_per_segment >= 0)
);

CREATE UNIQUE INDEX idx_reseller_tier_unique ON reseller_tariff_tiers (tariff_period_id, from_count);

-- 5. Template assignments (one template per sub-account)
CREATE TABLE sub_account_template_assignments (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    sub_account_id UUID        NOT NULL REFERENCES clients(id),
    template_id    UUID        NOT NULL REFERENCES reseller_tariff_templates(id),
    assigned_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_sub_account_template_unique ON sub_account_template_assignments (sub_account_id);

COMMIT;
