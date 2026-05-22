CREATE TABLE default_sender_names (
    client_id UUID NOT NULL,
    channel VARCHAR(20) NOT NULL CHECK (channel IN ('sms', 'viber')),
    sender_name_id UUID NOT NULL,
    PRIMARY KEY (client_id, channel)
);
