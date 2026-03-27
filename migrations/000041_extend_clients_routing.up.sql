ALTER TABLE clients ADD COLUMN IF NOT EXISTS routing_mode VARCHAR(20) NOT NULL DEFAULT 'legacy'
    CHECK (routing_mode IN ('legacy', 'new', 'hybrid'));
ALTER TABLE clients ADD COLUMN IF NOT EXISTS cost_visibility_enabled BOOLEAN NOT NULL DEFAULT false;
