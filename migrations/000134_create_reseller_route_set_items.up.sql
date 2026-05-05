CREATE TABLE IF NOT EXISTS reseller_route_set_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    set_id UUID NOT NULL REFERENCES reseller_route_sets(id) ON DELETE CASCADE,
    name VARCHAR(255),
    comment TEXT,
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
    priority INT NOT NULL DEFAULT 0,
    share INT NOT NULL DEFAULT 100,
    route_type VARCHAR(10) NOT NULL DEFAULT 'sms',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reseller_route_set_items_set
    ON reseller_route_set_items (set_id);
CREATE INDEX IF NOT EXISTS idx_reseller_route_set_items_provider
    ON reseller_route_set_items (provider_id);

CREATE TRIGGER update_reseller_route_set_items_updated_at
    BEFORE UPDATE ON reseller_route_set_items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
