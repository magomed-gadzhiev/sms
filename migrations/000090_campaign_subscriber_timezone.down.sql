-- migrations/000090_campaign_subscriber_timezone.down.sql
ALTER TABLE campaigns DROP COLUMN IF EXISTS use_subscriber_timezone;
