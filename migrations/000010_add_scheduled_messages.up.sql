-- migrations/000010_add_scheduled_messages.up.sql

-- Add scheduled_at column to messages table
ALTER TABLE messages ADD COLUMN scheduled_at TIMESTAMPTZ;

-- Drop existing CHECK constraint (name may vary: messages_status_check or messages_check)
-- Use DO block to find and drop it dynamically
DO $$
DECLARE
    constraint_name TEXT;
BEGIN
    SELECT conname INTO constraint_name
    FROM pg_constraint
    WHERE conrelid = 'messages'::regclass AND contype = 'c' AND conname LIKE '%status%'
    LIMIT 1;
    IF constraint_name IS NOT NULL THEN
        EXECUTE 'ALTER TABLE messages DROP CONSTRAINT ' || constraint_name;
    END IF;
END $$;

-- Re-create with new statuses
ALTER TABLE messages ADD CONSTRAINT messages_status_check
    CHECK (status IN ('pending', 'queued', 'sent', 'delivered', 'failed', 'expired', 'rejected', 'scheduled', 'cancelled'));

-- Partial index for scheduler queries (only covers scheduled messages)
CREATE INDEX idx_messages_scheduled ON messages(status, scheduled_at)
    WHERE status = 'scheduled';
