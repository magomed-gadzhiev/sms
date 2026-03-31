-- Seed Max Messenger channel (inactive by default)
INSERT INTO delivery_channels (channel_type, name, description, config, active)
VALUES (
    'max_messenger',
    'Max Messenger',
    'Мессенджер Max — доставка сообщений через Bot API',
    '{}',
    false
) ON CONFLICT (channel_type) DO NOTHING;

-- Enable Max Messenger support for all operators (internet-based, operator-independent)
INSERT INTO operator_channel_support (operator_id, channel_type, supported)
SELECT id, 'max_messenger', true FROM operators
ON CONFLICT (operator_id, channel_type) DO NOTHING;
