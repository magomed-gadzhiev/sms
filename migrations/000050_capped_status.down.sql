-- migrations/000050_capped_status.down.sql
ALTER TABLE campaigns DROP COLUMN IF EXISTS capped_count;

DO $$
DECLARE
    part_name TEXT;
BEGIN
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
    CHECK (status IN ('pending', 'sent', 'delivered', 'failed', 'retry', 'cancelled'));
