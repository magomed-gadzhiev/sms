-- Add index on messages.smpp_message_id for DLR status lookup queries.
-- These queries run constantly and without index cause full table scans on 10M+ rows.
CREATE INDEX IF NOT EXISTS idx_messages_smpp_message_id
    ON messages (smpp_message_id);
