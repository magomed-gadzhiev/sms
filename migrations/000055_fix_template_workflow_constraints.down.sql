ALTER TABLE templates DROP CONSTRAINT IF EXISTS templates_status_check;
ALTER TABLE templates ADD CONSTRAINT templates_status_check
    CHECK (status IN ('draft', 'approved', 'rejected'));

ALTER TABLE template_audit_log DROP CONSTRAINT IF EXISTS template_audit_log_action_check;
ALTER TABLE template_audit_log ADD CONSTRAINT template_audit_log_action_check
    CHECK (action IN ('created', 'updated', 'approved', 'rejected', 'deleted'));
