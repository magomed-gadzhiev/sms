-- Add index on messages.client_id for fast detalization queries.
-- This index propagates to all partitions automatically.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_client_id
    ON messages (client_id);
