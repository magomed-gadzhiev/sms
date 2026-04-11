-- Fix: change template_campaign_id FK from CASCADE to RESTRICT
-- so that deleting a campaign used as a schedule template is blocked
-- instead of silently removing the schedule.
ALTER TABLE campaign_schedules
    DROP CONSTRAINT campaign_schedules_template_campaign_id_fkey;

ALTER TABLE campaign_schedules
    ADD CONSTRAINT campaign_schedules_template_campaign_id_fkey
        FOREIGN KEY (template_campaign_id)
        REFERENCES campaigns(id)
        ON DELETE RESTRICT;
