CREATE TABLE IF NOT EXISTS reseller_provider_set_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    set_id UUID NOT NULL REFERENCES reseller_provider_sets(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
    priority INT NOT NULL DEFAULT 0,
    expose_cost BOOLEAN NOT NULL DEFAULT false,
    expose_provider_name BOOLEAN NOT NULL DEFAULT true,
    CONSTRAINT uq_reseller_provider_set_items UNIQUE (set_id, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_reseller_provider_set_items_set
    ON reseller_provider_set_items (set_id);
CREATE INDEX IF NOT EXISTS idx_reseller_provider_set_items_provider
    ON reseller_provider_set_items (provider_id);
