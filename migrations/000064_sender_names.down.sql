-- Rollback Migration 000064: Sender Names & Message Templates Registration

BEGIN;

-- Drop indexes first
DROP INDEX IF EXISTS idx_templates_sender_name_id;
DROP INDEX IF EXISTS idx_snsh_actor_id;
DROP INDEX IF EXISTS idx_snsh_sender_name_id;
DROP INDEX IF EXISTS idx_sender_names_client_status;
DROP INDEX IF EXISTS idx_sender_names_status;
DROP INDEX IF EXISTS idx_sender_names_client_id;

-- Remove FK from templates
ALTER TABLE templates DROP COLUMN IF EXISTS sender_name_id;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS sender_name_status_history;
DROP TABLE IF EXISTS sender_names;

COMMIT;
