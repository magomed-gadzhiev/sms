CREATE TABLE IF NOT EXISTS client_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id),
    provider_id UUID NOT NULL REFERENCES providers(id),
    ownership VARCHAR(20) NOT NULL CHECK (ownership IN ('platform', 'private', 'inherited')),
    source_client_id UUID REFERENCES clients(id),
    shared_priority INT NOT NULL DEFAULT 0,
    expose_cost BOOLEAN NOT NULL DEFAULT false,
    expose_provider_name BOOLEAN NOT NULL DEFAULT true,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(client_id, provider_id)
);

CREATE INDEX idx_client_providers_client ON client_providers(client_id) WHERE active = true;
CREATE INDEX idx_client_providers_provider ON client_providers(provider_id) WHERE active = true;
CREATE INDEX idx_client_providers_source ON client_providers(source_client_id) WHERE source_client_id IS NOT NULL;

CREATE TRIGGER update_client_providers_updated_at
    BEFORE UPDATE ON client_providers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
