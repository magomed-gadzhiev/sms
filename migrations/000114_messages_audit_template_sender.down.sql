-- Rollback Migration 000114: drop audit linkage columns from messages.

BEGIN;

DROP INDEX IF EXISTS idx_messages_sender_name_id;
DROP INDEX IF EXISTS idx_messages_template_id;

ALTER TABLE messages
    DROP COLUMN IF EXISTS sender_name_id,
    DROP COLUMN IF EXISTS template_id;

COMMIT;
