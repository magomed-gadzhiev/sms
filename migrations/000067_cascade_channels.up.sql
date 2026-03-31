CREATE TABLE delivery_channels (
    id           UUID        NOT NULL DEFAULT gen_random_uuid(),
    channel_type TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    description  TEXT,
    config       JSONB       NOT NULL DEFAULT '{}',
    active       BOOLEAN     NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id),
    UNIQUE (channel_type)
);

-- Seed SMS channel
INSERT INTO delivery_channels (channel_type, name, description)
VALUES ('sms', 'SMS', 'Standard SMS delivery channel');
