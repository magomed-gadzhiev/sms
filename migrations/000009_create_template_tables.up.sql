CREATE TABLE templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    variables TEXT[] NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'approved', 'rejected')),
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_templates_client_id ON templates(client_id);
CREATE INDEX idx_templates_client_status ON templates(client_id, status);
CREATE UNIQUE INDEX idx_templates_client_name ON templates(client_id, name);

CREATE TRIGGER update_templates_updated_at BEFORE UPDATE ON templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE template_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id UUID REFERENCES templates(id) ON DELETE SET NULL,
    action VARCHAR(20) NOT NULL
        CHECK (action IN ('created', 'updated', 'approved', 'rejected', 'deleted')),
    old_body TEXT,
    new_body TEXT,
    actor_id UUID,
    actor_type VARCHAR(20),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_template_audit_template_id ON template_audit_log(template_id);
