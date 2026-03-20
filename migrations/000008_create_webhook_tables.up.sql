-- migrations/000008_create_webhook_tables.up.sql
CREATE TABLE webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    event_types TEXT[] NOT NULL,
    secret TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_webhook_subscriptions_client_id ON webhook_subscriptions(client_id);
CREATE INDEX idx_webhook_subscriptions_active ON webhook_subscriptions(client_id, active) WHERE active = true;

CREATE TRIGGER update_webhook_subscriptions_updated_at BEFORE UPDATE ON webhook_subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
