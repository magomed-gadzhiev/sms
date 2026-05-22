CREATE TABLE IF NOT EXISTS stub_provider_config (
    provider_id      UUID PRIMARY KEY REFERENCES providers(id) ON DELETE CASCADE,
    min_delay_ms     INT NOT NULL DEFAULT 100,
    max_delay_ms     INT NOT NULL DEFAULT 500,
    failure_rate_pct INT NOT NULL DEFAULT 0 CHECK (failure_rate_pct BETWEEN 0 AND 100),
    dlr_delay_ms     INT NOT NULL DEFAULT 1000,
    dlr_success_rate INT NOT NULL DEFAULT 100 CHECK (dlr_success_rate BETWEEN 0 AND 100),
    dlr_statuses     JSONB NOT NULL DEFAULT '["DELIVRD"]',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER update_stub_provider_config_updated_at
    BEFORE UPDATE ON stub_provider_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
