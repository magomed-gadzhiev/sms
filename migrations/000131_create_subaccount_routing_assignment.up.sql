CREATE TABLE IF NOT EXISTS subaccount_routing_assignment (
    client_id UUID PRIMARY KEY REFERENCES clients(id) ON DELETE CASCADE,
    provider_set_id UUID REFERENCES reseller_provider_sets(id) ON DELETE SET NULL,
    route_set_id UUID, -- FK добавится в Plan 2 (после создания reseller_route_sets)
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sra_provider_set
    ON subaccount_routing_assignment (provider_set_id);
