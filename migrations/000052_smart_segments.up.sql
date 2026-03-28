-- migrations/000052_smart_segments.up.sql

CREATE TABLE saved_segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    contact_list_ids UUID[] NOT NULL,
    rules JSONB NOT NULL DEFAULT '{}',
    tag_rules JSONB,
    estimated_count INT NOT NULL DEFAULT 0,
    estimated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_segments_client ON saved_segments(client_id);

-- Add segment_id to campaigns
ALTER TABLE campaigns ADD COLUMN segment_id UUID REFERENCES saved_segments(id);
