-- Lookup log with monthly partitioning (same pattern as tarification_log)

CREATE TABLE IF NOT EXISTS lookup_log (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    msisdn VARCHAR(20) NOT NULL,
    operator_mccmnc VARCHAR(10),
    operator_name VARCHAR(100),
    number_status VARCHAR(20) NOT NULL,
    country_code VARCHAR(3),
    number_type VARCHAR(10),
    is_ported BOOLEAN,
    hlr_provider_id UUID REFERENCES hlr_providers(id),
    source VARCHAR(20) NOT NULL,
    client_id UUID NOT NULL,
    cached BOOLEAN NOT NULL DEFAULT false,
    latency_ms INTEGER,
    request_id VARCHAR(100) NOT NULL,
    message_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create partitions for 2026
CREATE TABLE lookup_log_2026_01 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE lookup_log_2026_02 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE lookup_log_2026_03 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE lookup_log_2026_04 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE lookup_log_2026_05 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE lookup_log_2026_06 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE lookup_log_2026_07 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE lookup_log_2026_08 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE lookup_log_2026_09 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE lookup_log_2026_10 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE lookup_log_2026_11 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE lookup_log_2026_12 PARTITION OF lookup_log
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

CREATE INDEX idx_lookup_log_client ON lookup_log (client_id, created_at);
CREATE INDEX idx_lookup_log_msisdn ON lookup_log (msisdn, created_at);
CREATE INDEX idx_lookup_log_request ON lookup_log (request_id);
