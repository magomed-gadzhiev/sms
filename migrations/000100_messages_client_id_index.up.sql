-- Add index on messages.client_id for fast detalization queries.
-- Cannot use CONCURRENTLY on partitioned tables, but the index
-- propagates to all existing and future partitions automatically.
CREATE INDEX IF NOT EXISTS idx_messages_client_id
    ON messages (client_id);
