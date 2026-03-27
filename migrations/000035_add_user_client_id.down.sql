DROP INDEX IF EXISTS idx_users_client_id;
ALTER TABLE users DROP COLUMN IF EXISTS client_id;
