-- migrations/000043_campaigns.up.sql

-- Campaigns
CREATE TABLE campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'scheduled', 'materializing', 'running', 'paused', 'completed', 'cancelled')),
    contact_list_id UUID NOT NULL REFERENCES contact_lists(id),
    template_id UUID REFERENCES templates(id),
    source TEXT NOT NULL DEFAULT '',
    segment_rules JSONB,
    segment_tags TEXT[],
    send_rate INT NOT NULL DEFAULT 0,
    scheduled_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    retry_config JSONB,
    total_recipients INT NOT NULL DEFAULT 0,
    sent_count INT NOT NULL DEFAULT 0,
    delivered_count INT NOT NULL DEFAULT 0,
    failed_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_campaigns_client ON campaigns(client_id);
CREATE INDEX idx_campaigns_status ON campaigns(client_id, status);

-- Campaign Variants (A/B testing)
CREATE TABLE campaign_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    template_id UUID REFERENCES templates(id),
    percentage INT NOT NULL DEFAULT 0,
    is_winner BOOLEAN NOT NULL DEFAULT false,
    is_control BOOLEAN NOT NULL DEFAULT false,
    sent_count INT NOT NULL DEFAULT 0,
    delivered_count INT NOT NULL DEFAULT 0,
    failed_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_cv_campaign ON campaign_variants(campaign_id);

-- Campaign A/B Config
CREATE TABLE campaign_ab_config (
    campaign_id UUID PRIMARY KEY REFERENCES campaigns(id) ON DELETE CASCADE,
    metric TEXT NOT NULL DEFAULT 'delivery_rate' CHECK (metric IN ('delivery_rate')),
    test_duration_hours INT NOT NULL DEFAULT 1,
    auto_select_winner BOOLEAN NOT NULL DEFAULT false,
    winner_variant_id UUID REFERENCES campaign_variants(id),
    winner_selected_at TIMESTAMPTZ
);

-- Campaign Recipients (hash-partitioned by campaign_id, 16 partitions)
CREATE TABLE campaign_recipients (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL,
    contact_id UUID NOT NULL,
    phone TEXT NOT NULL,
    variant_id UUID,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'delivered', 'failed', 'retry', 'cancelled')),
    message_id UUID,
    retry_count INT NOT NULL DEFAULT 0,
    last_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
) PARTITION BY HASH (campaign_id);

-- Create 16 hash partitions
CREATE TABLE campaign_recipients_p0  PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 0);
CREATE TABLE campaign_recipients_p1  PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 1);
CREATE TABLE campaign_recipients_p2  PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 2);
CREATE TABLE campaign_recipients_p3  PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 3);
CREATE TABLE campaign_recipients_p4  PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 4);
CREATE TABLE campaign_recipients_p5  PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 5);
CREATE TABLE campaign_recipients_p6  PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 6);
CREATE TABLE campaign_recipients_p7  PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 7);
CREATE TABLE campaign_recipients_p8  PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 8);
CREATE TABLE campaign_recipients_p9  PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 9);
CREATE TABLE campaign_recipients_p10 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 10);
CREATE TABLE campaign_recipients_p11 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 11);
CREATE TABLE campaign_recipients_p12 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 12);
CREATE TABLE campaign_recipients_p13 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 13);
CREATE TABLE campaign_recipients_p14 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 14);
CREATE TABLE campaign_recipients_p15 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 15);

CREATE INDEX idx_cr_campaign_status  ON campaign_recipients(campaign_id, status);
CREATE INDEX idx_cr_campaign_variant ON campaign_recipients(campaign_id, variant_id);
CREATE INDEX idx_cr_message_id       ON campaign_recipients(message_id);

-- Campaign Retry Log
CREATE TABLE campaign_retry_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    recipient_id UUID NOT NULL,
    retry_number INT NOT NULL,
    provider_id UUID,
    status TEXT NOT NULL,
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_crl_campaign   ON campaign_retry_log(campaign_id);
CREATE INDEX idx_crl_recipient  ON campaign_retry_log(recipient_id);

-- Campaign Stats Snapshots
-- variant_id uses sentinel UUID '00000000-0000-0000-0000-000000000000' for campaign-level
-- (non-variant) snapshots so the composite PK works without NULLs.
CREATE TABLE campaign_stats_snapshots (
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    variant_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
    snapshot_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent INT NOT NULL DEFAULT 0,
    delivered INT NOT NULL DEFAULT 0,
    failed INT NOT NULL DEFAULT 0,
    pending INT NOT NULL DEFAULT 0,
    avg_delivery_time_ms INT NOT NULL DEFAULT 0,
    cost DECIMAL(12,4) NOT NULL DEFAULT 0,
    PRIMARY KEY (campaign_id, variant_id, snapshot_at)
);

CREATE INDEX idx_css_campaign ON campaign_stats_snapshots(campaign_id, snapshot_at DESC);
