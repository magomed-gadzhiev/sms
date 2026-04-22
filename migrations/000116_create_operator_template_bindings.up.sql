BEGIN;

CREATE TABLE operator_template_bindings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    sender_name_id UUID NOT NULL REFERENCES sender_names(id) ON DELETE CASCADE,
    operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected')),
    rejection_reason TEXT,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (template_id, operator_id)
);

CREATE INDEX idx_otb_operator_status ON operator_template_bindings (operator_id, status);
CREATE INDEX idx_otb_sender_name ON operator_template_bindings (sender_name_id);
CREATE INDEX idx_otb_template ON operator_template_bindings (template_id);

COMMIT;
