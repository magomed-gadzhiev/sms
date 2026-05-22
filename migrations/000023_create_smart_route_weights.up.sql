-- Smart route weights for cost/quality optimization

CREATE TABLE IF NOT EXISTS smart_route_weights (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_code VARCHAR(20),
    country_code VARCHAR(3) NOT NULL,
    cost_weight NUMERIC(3,2) NOT NULL DEFAULT 0.60,
    quality_weight NUMERIC(3,2) NOT NULL DEFAULT 0.40,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_weights_sum CHECK (cost_weight + quality_weight = 1.00),
    CONSTRAINT uq_smart_route_weights UNIQUE (operator_code, country_code)
);

CREATE INDEX idx_smart_route_weights_lookup ON smart_route_weights (country_code, operator_code, active);

CREATE TRIGGER update_smart_route_weights_updated_at
    BEFORE UPDATE ON smart_route_weights
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
