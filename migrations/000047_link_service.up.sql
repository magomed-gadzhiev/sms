-- migrations/000047_link_service.up.sql

-- Custom domains for client-branded short links
CREATE TABLE client_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    domain TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending_dns' CHECK (status IN ('pending_dns', 'pending_ssl', 'active', 'failed')),
    dns_txt_record TEXT NOT NULL,
    dns_verified_at TIMESTAMPTZ,
    ssl_cert_path TEXT,
    ssl_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_client_domains_client ON client_domains(client_id);
CREATE INDEX idx_client_domains_status ON client_domains(status);

-- Short links
CREATE TABLE short_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    domain_id UUID REFERENCES client_domains(id),
    code VARCHAR(10) NOT NULL UNIQUE,
    original_url TEXT NOT NULL,
    message_id UUID,
    campaign_id UUID,
    recipient_id UUID,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_short_links_code ON short_links(code);
CREATE INDEX idx_short_links_client ON short_links(client_id);
CREATE INDEX idx_short_links_campaign ON short_links(campaign_id);

-- Click events (monthly partitioned)
CREATE TABLE click_events (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    short_link_id UUID NOT NULL,
    client_id UUID NOT NULL,
    campaign_id UUID,
    phone TEXT,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip_address INET,
    user_agent TEXT,
    referer TEXT,
    country_code VARCHAR(2),
    is_unique BOOLEAN NOT NULL DEFAULT false
) PARTITION BY RANGE (clicked_at);

-- Create partitions for current and next months
CREATE TABLE click_events_2026_03 PARTITION OF click_events
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE click_events_2026_04 PARTITION OF click_events
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE click_events_2026_05 PARTITION OF click_events
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE INDEX idx_click_events_link ON click_events(short_link_id, clicked_at);
CREATE INDEX idx_click_events_campaign ON click_events(campaign_id, clicked_at);
