ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS channel     VARCHAR(20) DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS operator_id UUID        DEFAULT NULL REFERENCES operators(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS country_id  UUID        DEFAULT NULL REFERENCES countries(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS send_method VARCHAR(20) DEFAULT NULL;

COMMENT ON COLUMN messages.channel     IS 'Канал доставки: SMS, MAX, Viber';
COMMENT ON COLUMN messages.operator_id IS 'Оператор абонента (определяется при маршрутизации)';
COMMENT ON COLUMN messages.country_id  IS 'Страна абонента';
COMMENT ON COLUMN messages.send_method IS 'Способ отправки: PORTAL, API, SMPP';

CREATE INDEX IF NOT EXISTS idx_messages_operator_id ON messages(operator_id) WHERE operator_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_channel     ON messages(channel)     WHERE channel IS NOT NULL;
