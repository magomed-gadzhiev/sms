DROP INDEX IF EXISTS idx_providers_source_client_id;

ALTER TABLE providers DROP CONSTRAINT IF EXISTS providers_ownership_source_consistency;
ALTER TABLE providers DROP CONSTRAINT IF EXISTS providers_ownership_check;

ALTER TABLE providers DROP COLUMN IF EXISTS source_client_id;
ALTER TABLE providers DROP COLUMN IF EXISTS ownership;
