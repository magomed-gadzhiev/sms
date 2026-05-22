ALTER TABLE default_sender_names DROP CONSTRAINT IF EXISTS default_sender_names_channel_check;
DELETE FROM default_sender_names WHERE channel = 'max';
ALTER TABLE default_sender_names
    ADD CONSTRAINT default_sender_names_channel_check
    CHECK (channel::text = ANY (ARRAY['sms','viber']::text[]));
