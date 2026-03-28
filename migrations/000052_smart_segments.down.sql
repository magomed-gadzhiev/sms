-- migrations/000052_smart_segments.down.sql
ALTER TABLE campaigns DROP COLUMN IF EXISTS segment_id;
DROP TABLE IF EXISTS saved_segments;
