-- Add role column to sessions table
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS role VARCHAR(50);
