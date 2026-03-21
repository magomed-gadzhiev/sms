-- Drop the old audit_log table (from 000001) and recreate with tenant-aware schema
DROP TABLE IF EXISTS audit_log CASCADE;

-- New audit_log with tenant support, partitioned by month
CREATE TABLE audit_log (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    user_id UUID,
    action VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255),
    details JSONB,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Partitions for 2026 (all 12 months)
CREATE TABLE audit_log_y2026m01 PARTITION OF audit_log
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE audit_log_y2026m02 PARTITION OF audit_log
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE audit_log_y2026m03 PARTITION OF audit_log
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE audit_log_y2026m04 PARTITION OF audit_log
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE audit_log_y2026m05 PARTITION OF audit_log
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE audit_log_y2026m06 PARTITION OF audit_log
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE audit_log_y2026m07 PARTITION OF audit_log
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE audit_log_y2026m08 PARTITION OF audit_log
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE audit_log_y2026m09 PARTITION OF audit_log
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE audit_log_y2026m10 PARTITION OF audit_log
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE audit_log_y2026m11 PARTITION OF audit_log
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE audit_log_y2026m12 PARTITION OF audit_log
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

-- Partitions for 2027 Q1
CREATE TABLE audit_log_y2027m01 PARTITION OF audit_log
    FOR VALUES FROM ('2027-01-01') TO ('2027-02-01');
CREATE TABLE audit_log_y2027m02 PARTITION OF audit_log
    FOR VALUES FROM ('2027-02-01') TO ('2027-03-01');
CREATE TABLE audit_log_y2027m03 PARTITION OF audit_log
    FOR VALUES FROM ('2027-03-01') TO ('2027-04-01');

-- Indexes
CREATE INDEX idx_audit_log_tenant_id ON audit_log(tenant_id, created_at DESC);
CREATE INDEX idx_audit_log_action ON audit_log(action, created_at DESC);
CREATE INDEX idx_audit_log_user_id ON audit_log(user_id, created_at DESC) WHERE user_id IS NOT NULL;
