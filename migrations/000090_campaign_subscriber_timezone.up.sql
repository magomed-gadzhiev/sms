-- migrations/000090_campaign_subscriber_timezone.up.sql
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS use_subscriber_timezone BOOLEAN NOT NULL DEFAULT FALSE;
