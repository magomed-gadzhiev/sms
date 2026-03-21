-- Add sub-account (reseller) support to clients table
ALTER TABLE clients
    ADD COLUMN parent_client_id UUID DEFAULT NULL REFERENCES clients(id) ON DELETE RESTRICT,
    ADD COLUMN is_reseller BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN max_sub_accounts INTEGER NOT NULL DEFAULT 0;

-- Partial index for sub-accounts lookup
CREATE INDEX idx_clients_parent_client_id ON clients(parent_client_id) WHERE parent_client_id IS NOT NULL;

-- CHECK: is_reseller can only be true if parent_client_id IS NULL (top-level clients only)
ALTER TABLE clients ADD CONSTRAINT chk_reseller_is_top_level
    CHECK (is_reseller = false OR parent_client_id IS NULL);
