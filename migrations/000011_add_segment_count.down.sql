ALTER TABLE messages DROP COLUMN IF EXISTS expired_at;
ALTER TABLE transactions DROP COLUMN IF EXISTS segment_count;
ALTER TABLE messages DROP COLUMN IF EXISTS segment_count;
