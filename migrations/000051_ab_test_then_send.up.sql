-- migrations/000051_ab_test_then_send.up.sql

-- Extend campaign_ab_config for test-then-send
ALTER TABLE campaign_ab_config ADD COLUMN strategy VARCHAR(20) NOT NULL DEFAULT 'full_split';
ALTER TABLE campaign_ab_config ADD COLUMN test_percentage INT NOT NULL DEFAULT 100;
ALTER TABLE campaign_ab_config ADD COLUMN test_phase VARCHAR(20) NOT NULL DEFAULT 'none';
ALTER TABLE campaign_ab_config ADD COLUMN winning_metric VARCHAR(20) NOT NULL DEFAULT 'delivery_rate';
ALTER TABLE campaign_ab_config ADD COLUMN test_started_at TIMESTAMPTZ;
ALTER TABLE campaign_ab_config ADD COLUMN rollout_started_at TIMESTAMPTZ;

-- Drop existing metric CHECK constraint and replace
ALTER TABLE campaign_ab_config DROP CONSTRAINT IF EXISTS campaign_ab_config_metric_check;
ALTER TABLE campaign_ab_config ADD CONSTRAINT campaign_ab_config_metric_check
    CHECK (metric IN ('delivery_rate', 'click_rate', 'unique_click_rate'));

-- Add click counts to variants
ALTER TABLE campaign_variants ADD COLUMN click_count INT NOT NULL DEFAULT 0;
ALTER TABLE campaign_variants ADD COLUMN unique_click_count INT NOT NULL DEFAULT 0;

-- Add click columns to stats snapshots
ALTER TABLE campaign_stats_snapshots ADD COLUMN clicks INT NOT NULL DEFAULT 0;
ALTER TABLE campaign_stats_snapshots ADD COLUMN unique_clicks INT NOT NULL DEFAULT 0;
