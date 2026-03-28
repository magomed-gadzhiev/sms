-- migrations/000043_campaigns.down.sql
DROP TABLE IF EXISTS campaign_stats_snapshots;
DROP TABLE IF EXISTS campaign_retry_log;
DROP TABLE IF EXISTS campaign_recipients_p0, campaign_recipients_p1, campaign_recipients_p2, campaign_recipients_p3,
    campaign_recipients_p4, campaign_recipients_p5, campaign_recipients_p6, campaign_recipients_p7,
    campaign_recipients_p8, campaign_recipients_p9, campaign_recipients_p10, campaign_recipients_p11,
    campaign_recipients_p12, campaign_recipients_p13, campaign_recipients_p14, campaign_recipients_p15;
DROP TABLE IF EXISTS campaign_recipients;
DROP TABLE IF EXISTS campaign_ab_config;
DROP TABLE IF EXISTS campaign_variants;
DROP TABLE IF EXISTS campaigns;
