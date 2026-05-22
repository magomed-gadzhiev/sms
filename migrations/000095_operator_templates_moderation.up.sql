-- migrations/000095_operator_templates_moderation.up.sql
BEGIN;

ALTER TABLE operator_templates
    ADD COLUMN moderation_status VARCHAR(32) NOT NULL DEFAULT 'draft'
        CHECK (moderation_status IN ('draft', 'submitted', 'approved', 'rejected', 'revision_requested')),
    ADD COLUMN moderator_note    TEXT,
    ADD COLUMN submitted_at      TIMESTAMPTZ,
    ADD COLUMN resolved_at       TIMESTAMPTZ;

CREATE INDEX idx_optpl_moderation_status ON operator_templates (moderation_status);

COMMIT;
