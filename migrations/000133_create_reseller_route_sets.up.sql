CREATE TABLE IF NOT EXISTS reseller_route_sets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reseller_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_reseller_route_sets_name UNIQUE (reseller_id, name)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_reseller_route_sets_default
    ON reseller_route_sets (reseller_id) WHERE is_default = true;

CREATE INDEX IF NOT EXISTS idx_reseller_route_sets_reseller
    ON reseller_route_sets (reseller_id);

CREATE TRIGGER update_reseller_route_sets_updated_at
    BEFORE UPDATE ON reseller_route_sets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
