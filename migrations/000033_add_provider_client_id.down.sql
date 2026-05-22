DROP INDEX IF EXISTS idx_providers_client_id;
ALTER TABLE providers
  DROP COLUMN IF EXISTS routing_rules,
  DROP COLUMN IF EXISTS tps_limit,
  DROP COLUMN IF EXISTS tags,
  DROP COLUMN IF EXISTS description,
  DROP COLUMN IF EXISTS client_id;
