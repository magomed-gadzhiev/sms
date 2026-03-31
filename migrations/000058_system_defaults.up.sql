CREATE TABLE IF NOT EXISTS system_defaults (
    key        VARCHAR(100) PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now(),
    updated_by UUID
);

INSERT INTO system_defaults (key, value) VALUES
  ('rate_limit_per_second',    '"10"'),
  ('rate_limit_per_minute',    '"100"'),
  ('rate_limit_per_hour',      '"1000"'),
  ('default_tps_per_provider', '"5"'),
  ('max_providers_per_client', '"10"'),
  ('max_sub_accounts',         '"0"')
ON CONFLICT (key) DO NOTHING;
