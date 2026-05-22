CREATE TABLE notification_settings (
    user_id UUID NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    in_app BOOLEAN DEFAULT true,
    email BOOLEAN DEFAULT false,
    PRIMARY KEY (user_id, event_type)
);

CREATE TABLE notification_extra_emails (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (user_id, email)
);

-- Seed default settings for existing users
INSERT INTO notification_settings (user_id, event_type, in_app, email)
SELECT u.id, evt.type, true, false
FROM (SELECT DISTINCT user_id AS id FROM notifications) u
CROSS JOIN (VALUES
    ('campaign_completed'),
    ('campaign_failed'),
    ('low_balance'),
    ('sender_name_approved'),
    ('sender_name_rejected'),
    ('balance_topped_up')
) AS evt(type)
ON CONFLICT DO NOTHING;
