-- Add stub entries for delivery channels that cascade supports but weren't seeded:
-- viber, whatsapp, hlr. sms and max_messenger already exist (see migrations
-- 000072/000073). Uses postman-echo as a harmless dummy endpoint so cascade
-- workers can register the channel without hitting real provider APIs.

INSERT INTO delivery_channels (channel_type, name, description, config, active)
VALUES
    ('viber',    'Viber',    'Stub Viber channel for dev sandbox',    '{"api_key": "dummy-viber-api-key", "provider_url": "https://postman-echo.com/post", "webhook_secret": "viber_webhook_secret_stub_000000000000"}'::jsonb, true),
    ('whatsapp', 'WhatsApp', 'Stub WhatsApp channel for dev sandbox', '{"api_key": "dummy-whatsapp-api-key", "provider_url": "https://postman-echo.com/post", "webhook_secret": "whatsapp_webhook_secret_stub_000000000000"}'::jsonb, true),
    ('hlr',      'HLR',      'Stub HLR channel for dev sandbox',      '{"api_key": "dummy-hlr-api-key", "provider_url": "https://postman-echo.com/post"}'::jsonb, true)
ON CONFLICT (channel_type) DO NOTHING;
