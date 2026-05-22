-- Rollback of 000103_campaigns_use_subscriber_timezone.up.sql

ALTER TABLE campaigns
    DROP COLUMN use_subscriber_timezone;
