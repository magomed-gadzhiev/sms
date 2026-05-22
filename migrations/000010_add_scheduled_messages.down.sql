-- migrations/000010_add_scheduled_messages.down.sql
DROP INDEX IF EXISTS idx_messages_scheduled;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_status_check;
ALTER TABLE messages ADD CONSTRAINT messages_status_check
    CHECK (status IN ('pending', 'queued', 'sent', 'delivered', 'failed', 'expired', 'rejected'));
ALTER TABLE messages DROP COLUMN IF EXISTS scheduled_at;
