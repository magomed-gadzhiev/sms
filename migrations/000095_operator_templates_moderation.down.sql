-- migrations/000095_operator_templates_moderation.down.sql
BEGIN;
DROP INDEX IF EXISTS idx_optpl_moderation_status;
ALTER TABLE operator_templates
    DROP COLUMN IF EXISTS moderation_status,
    DROP COLUMN IF EXISTS moderator_note,
    DROP COLUMN IF EXISTS submitted_at,
    DROP COLUMN IF EXISTS resolved_at;
COMMIT;
