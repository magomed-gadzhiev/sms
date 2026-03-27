CREATE TABLE IF NOT EXISTS client_routing_strategies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id),
    operator_id UUID REFERENCES operators(id),
    strategy VARCHAR(20) NOT NULL CHECK (strategy IN ('priority', 'weighted', 'smart')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(client_id, operator_id)
);

-- Partial unique for default account strategy (operator_id IS NULL)
CREATE UNIQUE INDEX uq_client_routing_strategy_default
    ON client_routing_strategies (client_id)
    WHERE operator_id IS NULL;

CREATE TABLE IF NOT EXISTS client_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id),
    operator_id UUID NOT NULL REFERENCES operators(id),
    provider_id UUID NOT NULL REFERENCES providers(id),
    priority INT NOT NULL DEFAULT 0,
    weight INT NOT NULL DEFAULT 1,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(client_id, operator_id, provider_id)
);

CREATE INDEX idx_client_routes_lookup ON client_routes(client_id, operator_id) WHERE active = true;

CREATE TRIGGER update_client_routing_strategies_updated_at
    BEFORE UPDATE ON client_routing_strategies
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_client_routes_updated_at
    BEFORE UPDATE ON client_routes
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
