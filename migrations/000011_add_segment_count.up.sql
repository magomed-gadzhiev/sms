-- T001: Add segment_count to messages table
ALTER TABLE messages ADD COLUMN IF NOT EXISTS segment_count INTEGER NOT NULL DEFAULT 1;

-- T002: Add segment_count to transactions table
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS segment_count INTEGER NOT NULL DEFAULT 1;

-- Add expired_at to messages table for DLR expiry tracking
ALTER TABLE messages ADD COLUMN IF NOT EXISTS expired_at TIMESTAMP WITH TIME ZONE;
