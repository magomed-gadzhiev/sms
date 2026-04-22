DROP INDEX IF EXISTS idx_sender_names_client_channel;

BEGIN;
ALTER TABLE sender_names
  DROP CONSTRAINT IF EXISTS sender_names_client_id_name_channel_key;

ALTER TABLE sender_names
  ADD CONSTRAINT sender_names_client_id_name_key
  UNIQUE (client_id, name);
COMMIT;

ALTER TABLE sender_names DROP COLUMN IF EXISTS channel;
