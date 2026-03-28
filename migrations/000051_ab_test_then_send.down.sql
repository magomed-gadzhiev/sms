-- migrations/000051_ab_test_then_send.down.sql
ALTER TABLE campaign_stats_snapshots DROP COLUMN IF EXISTS unique_clicks;
ALTER TABLE campaign_stats_snapshots DROP COLUMN IF EXISTS clicks;
ALTER TABLE campaign_variants DROP COLUMN IF EXISTS unique_click_count;
ALTER TABLE campaign_variants DROP COLUMN IF EXISTS click_count;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS rollout_started_at;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS test_started_at;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS winning_metric;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS test_phase;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS test_percentage;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS strategy;
