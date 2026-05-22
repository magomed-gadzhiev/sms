-- migrations/000048_frequency_caps.up.sql

CREATE TABLE frequency_caps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    cap_type TEXT NOT NULL DEFAULT 'marketing' CHECK (cap_type IN ('marketing', 'transactional', 'all')),
    max_messages INT NOT NULL,
    period_hours INT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (client_id, cap_type)
);

CREATE INDEX idx_freq_caps_client ON frequency_caps(client_id);

CREATE TABLE campaign_frequency_overrides (
    campaign_id UUID PRIMARY KEY REFERENCES campaigns(id) ON DELETE CASCADE,
    max_messages INT,
    period_hours INT,
    bypass BOOLEAN NOT NULL DEFAULT false
);
