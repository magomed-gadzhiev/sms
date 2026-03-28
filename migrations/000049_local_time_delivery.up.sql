-- migrations/000049_local_time_delivery.up.sql

ALTER TABLE campaign_recipients ADD COLUMN deliver_at TIMESTAMPTZ;
CREATE INDEX idx_cr_deliver_at ON campaign_recipients(status, deliver_at) WHERE deliver_at IS NOT NULL;

CREATE TABLE client_quiet_hours (
    client_id UUID PRIMARY KEY REFERENCES clients(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    start_hour INT NOT NULL DEFAULT 22 CHECK (start_hour >= 0 AND start_hour <= 23),
    end_hour INT NOT NULL DEFAULT 8 CHECK (end_hour >= 0 AND end_hour <= 23),
    action TEXT NOT NULL DEFAULT 'postpone' CHECK (action IN ('postpone', 'skip')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
