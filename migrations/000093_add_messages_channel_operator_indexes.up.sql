-- Follow-up indexes for columns added in 000092
CREATE INDEX IF NOT EXISTS idx_messages_country_id ON messages(country_id) WHERE country_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_send_method ON messages(send_method) WHERE send_method IS NOT NULL;
