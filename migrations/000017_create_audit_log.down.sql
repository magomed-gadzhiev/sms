DROP TABLE IF EXISTS audit_log CASCADE;

-- Restore the original audit_log table from 000001_init_schema
CREATE TABLE audit_log (
    id UUID DEFAULT uuid_generate_v4(),
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    actor_type VARCHAR(50),
    actor_id UUID,
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Restore original partitions (2024)
CREATE TABLE audit_log_y2024m01 PARTITION OF audit_log
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
CREATE TABLE audit_log_y2024m02 PARTITION OF audit_log
    FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');
CREATE TABLE audit_log_y2024m03 PARTITION OF audit_log
    FOR VALUES FROM ('2024-03-01') TO ('2024-04-01');
CREATE TABLE audit_log_y2024m04 PARTITION OF audit_log
    FOR VALUES FROM ('2024-04-01') TO ('2024-05-01');
CREATE TABLE audit_log_y2024m05 PARTITION OF audit_log
    FOR VALUES FROM ('2024-05-01') TO ('2024-06-01');
CREATE TABLE audit_log_y2024m06 PARTITION OF audit_log
    FOR VALUES FROM ('2024-06-01') TO ('2024-07-01');
CREATE TABLE audit_log_y2024m07 PARTITION OF audit_log
    FOR VALUES FROM ('2024-07-01') TO ('2024-08-01');
CREATE TABLE audit_log_y2024m08 PARTITION OF audit_log
    FOR VALUES FROM ('2024-08-01') TO ('2024-09-01');
CREATE TABLE audit_log_y2024m09 PARTITION OF audit_log
    FOR VALUES FROM ('2024-09-01') TO ('2024-10-01');
CREATE TABLE audit_log_y2024m10 PARTITION OF audit_log
    FOR VALUES FROM ('2024-10-01') TO ('2024-11-01');
CREATE TABLE audit_log_y2024m11 PARTITION OF audit_log
    FOR VALUES FROM ('2024-11-01') TO ('2024-12-01');
CREATE TABLE audit_log_y2024m12 PARTITION OF audit_log
    FOR VALUES FROM ('2024-12-01') TO ('2025-01-01');

-- Restore original indexes
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_log_actor ON audit_log(actor_type, actor_id);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);
CREATE INDEX idx_audit_log_action ON audit_log(action);
