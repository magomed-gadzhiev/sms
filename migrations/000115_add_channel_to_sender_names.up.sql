ALTER TABLE sender_names
  ADD COLUMN channel VARCHAR(16) NOT NULL DEFAULT 'sms'
  CHECK (channel IN ('sms', 'voice', 'viber'));

BEGIN;
ALTER TABLE sender_names
  DROP CONSTRAINT IF EXISTS sender_names_client_id_name_key;

ALTER TABLE sender_names
  ADD CONSTRAINT sender_names_client_id_name_channel_key
  UNIQUE (client_id, name, channel);
COMMIT;

CREATE INDEX IF NOT EXISTS idx_sender_names_client_channel
  ON sender_names (client_id, channel);
