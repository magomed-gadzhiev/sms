-- migrations/000050_capped_status.up.sql

-- Drop the CHECK constraint on campaign_recipients.status and recreate with new values
-- Hash-partitioned tables: constraint is on each partition
DO $$
DECLARE
    part_name TEXT;
BEGIN
    -- Drop constraint from parent and all partitions
    FOR part_name IN
        SELECT tablename FROM pg_tables WHERE tablename LIKE 'campaign_recipients%'
    LOOP
        EXECUTE format(
            'ALTER TABLE %I DROP CONSTRAINT IF EXISTS campaign_recipients_status_check',
            part_name
        );
    END LOOP;
END $$;

ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_status_check
    CHECK (status IN ('pending', 'sent', 'delivered', 'failed', 'retry', 'cancelled', 'capped', 'pending_rollout', 'skipped_quiet_hours'));

-- Add capped_count to campaigns
ALTER TABLE campaigns ADD COLUMN capped_count INT NOT NULL DEFAULT 0;
