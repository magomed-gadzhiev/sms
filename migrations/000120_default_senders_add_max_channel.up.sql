-- Расширяем CHECK constraint default_sender_names: добавляем канал MAX
-- (мессенджер MAX уже поддержан как канал в каскаде, но раньше не мог быть default sender).
ALTER TABLE default_sender_names DROP CONSTRAINT IF EXISTS default_sender_names_channel_check;
ALTER TABLE default_sender_names
    ADD CONSTRAINT default_sender_names_channel_check
    CHECK (channel::text = ANY (ARRAY['sms','viber','max']::text[]));
