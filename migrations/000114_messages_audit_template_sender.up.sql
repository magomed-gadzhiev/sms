-- Migration 000114: Audit trail linkage from messages to templates and sender_names.
--
-- Motivation (2026-04-22 E2E sender-names report, Bugs #6 + #7):
--   * messages table has no FK to templates/sender_names — impossible to prove
--     an approved template/sender was used for a sent SMS.
--   * Adds nullable UUID columns (legacy rows lack the linkage; future send
--     handlers populate them after validation).
--
-- Partitioning note: `messages` is monthly-partitioned (PG12+). ALTER TABLE on
-- the parent partitioned table propagates the new columns and FK constraints to
-- all existing and future partitions automatically. No per-partition script
-- needed (verified: PG12+ supports ADD COLUMN / ADD CONSTRAINT on partitioned
-- parents).

BEGIN;

ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS template_id    UUID NULL
        REFERENCES templates(id)    ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS sender_name_id UUID NULL
        REFERENCES sender_names(id) ON DELETE SET NULL;

-- Indexes: aggregator/analytics filter by template_id and sender_name_id; keep
-- them partial so we don't index the (majority legacy) NULL rows.
CREATE INDEX IF NOT EXISTS idx_messages_template_id
    ON messages (template_id)
    WHERE template_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_messages_sender_name_id
    ON messages (sender_name_id)
    WHERE sender_name_id IS NOT NULL;

COMMIT;
