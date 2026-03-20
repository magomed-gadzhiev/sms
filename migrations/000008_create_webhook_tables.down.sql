-- migrations/000008_create_webhook_tables.down.sql
DROP TRIGGER IF EXISTS update_webhook_subscriptions_updated_at ON webhook_subscriptions;
DROP TABLE IF EXISTS webhook_subscriptions;
