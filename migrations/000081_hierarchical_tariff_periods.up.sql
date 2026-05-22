-- Hierarchical tariff periods system with multi-dimensional scope
-- btree_gist already enabled in migration 000013

CREATE TABLE IF NOT EXISTS tariff_periods_new (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  -- Dimensions (all nullable, NULL = "any")
  country_id      UUID REFERENCES countries(id),
  operator_id     UUID REFERENCES operators(id),
  sender_category TEXT CHECK (sender_category IN ('shared', 'paid_registered', 'free_registered')),
  traffic_type    TEXT CHECK (traffic_type IN ('authorization', 'transactional', 'service', 'extensible')),
  client_id       UUID REFERENCES accounts(id),

  -- Scope (computed on write, never edited)
  scope_key       TEXT NOT NULL,
  scope_priority  INT  NOT NULL,

  -- Strategy
  strategy        TEXT NOT NULL CHECK (strategy IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold')),

  -- Dates
  start_date      DATE NOT NULL,
  end_date        DATE,  -- NULL = open-ended

  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT valid_dates CHECK (end_date IS NULL OR end_date > start_date)
);

-- Overlap prevention: treat open-ended as far-future date for exclusion
ALTER TABLE tariff_periods_new ADD CONSTRAINT no_overlap_in_scope
  EXCLUDE USING gist (
    scope_key WITH =,
    daterange(start_date, COALESCE(end_date, '9999-12-31'::date), '[]') WITH &&
  );

CREATE INDEX idx_tariff_periods_new_lookup ON tariff_periods_new (
  country_id, operator_id, sender_category, traffic_type, client_id,
  start_date, end_date
);

CREATE INDEX idx_tariff_periods_new_scope ON tariff_periods_new (scope_priority, scope_key);

-- Tiers for new periods (same structure, new FK)
CREATE TABLE IF NOT EXISTS tariff_tiers_new (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tariff_period_id  UUID NOT NULL REFERENCES tariff_periods_new(id) ON DELETE CASCADE,
  from_count        INT NOT NULL CHECK (from_count >= 0),
  price_per_segment NUMERIC(20,6) NOT NULL CHECK (price_per_segment >= 0),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tariff_period_id, from_count)
);

-- Provider cost periods
CREATE TABLE IF NOT EXISTS provider_cost_periods (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider_id     UUID NOT NULL REFERENCES providers(id),
  country_id      UUID REFERENCES countries(id),
  operator_id     UUID REFERENCES operators(id),
  traffic_type    TEXT CHECK (traffic_type IN ('authorization', 'transactional', 'service', 'extensible')),

  scope_key       TEXT NOT NULL,
  scope_priority  INT  NOT NULL,
  strategy        TEXT NOT NULL CHECK (strategy IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold')),

  start_date      DATE NOT NULL,
  end_date        DATE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT valid_cost_dates CHECK (end_date IS NULL OR end_date > start_date)
);

ALTER TABLE provider_cost_periods ADD CONSTRAINT no_cost_overlap_in_scope
  EXCLUDE USING gist (
    scope_key WITH =,
    daterange(start_date, COALESCE(end_date, '9999-12-31'::date), '[]') WITH &&
  );

CREATE INDEX idx_provider_cost_periods_lookup ON provider_cost_periods (
  provider_id, country_id, operator_id, traffic_type, start_date, end_date
);

CREATE INDEX idx_provider_cost_periods_scope ON provider_cost_periods (scope_priority, scope_key);

CREATE TABLE IF NOT EXISTS provider_cost_tiers (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cost_period_id   UUID NOT NULL REFERENCES provider_cost_periods(id) ON DELETE CASCADE,
  from_count       INT NOT NULL CHECK (from_count >= 0),
  cost_per_segment NUMERIC(20,6) NOT NULL CHECK (cost_per_segment >= 0),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (cost_period_id, from_count)
);
