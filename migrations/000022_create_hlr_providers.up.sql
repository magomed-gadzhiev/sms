-- HLR/MNP providers table

CREATE TABLE IF NOT EXISTS hlr_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    adapter_type VARCHAR(50) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    priority INTEGER NOT NULL DEFAULT 1,
    supported_regions TEXT[] NOT NULL DEFAULT '{}',
    cost_per_lookup NUMERIC(20,6) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'healthy',
    success_rate NUMERIC(5,2) DEFAULT 100.00,
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_hlr_providers_active ON hlr_providers (active, priority);
CREATE INDEX idx_hlr_providers_status ON hlr_providers (status);

CREATE TRIGGER update_hlr_providers_updated_at
    BEFORE UPDATE ON hlr_providers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
