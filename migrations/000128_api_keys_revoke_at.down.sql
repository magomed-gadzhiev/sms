DROP INDEX IF EXISTS api_keys_revoke_at_idx;
ALTER TABLE api_keys DROP COLUMN IF EXISTS revoke_at;
