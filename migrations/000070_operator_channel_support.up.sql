CREATE TABLE operator_channel_support (
    id           UUID        NOT NULL DEFAULT gen_random_uuid(),
    operator_id  UUID        NOT NULL REFERENCES operators(id),
    channel_type TEXT        NOT NULL,
    supported    BOOLEAN     NOT NULL DEFAULT true,
    notes        TEXT,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   UUID,

    PRIMARY KEY (id),
    UNIQUE (operator_id, channel_type)
);

-- Seed: all existing operators support SMS
INSERT INTO operator_channel_support (operator_id, channel_type, supported)
SELECT id, 'sms', true FROM operators;
