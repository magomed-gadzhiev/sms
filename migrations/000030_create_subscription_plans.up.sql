CREATE TABLE subscription_plans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    monthly_price_rub NUMERIC(10, 2) NOT NULL DEFAULT 0,
    max_sms_per_month INTEGER NOT NULL DEFAULT 50000,
    max_smpp_connections INTEGER NOT NULL DEFAULT 1,
    max_users INTEGER NOT NULL DEFAULT 1,
    rate_limit_per_second INTEGER NOT NULL DEFAULT 10,
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 100,
    rate_limit_per_hour INTEGER NOT NULL DEFAULT 1000,
    rate_limit_per_day INTEGER NOT NULL DEFAULT 10000,
    features JSONB NOT NULL DEFAULT '{}',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TRIGGER update_subscription_plans_updated_at BEFORE UPDATE ON subscription_plans
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Seed default plans
INSERT INTO subscription_plans (name, display_name, monthly_price_rub, max_sms_per_month, max_smpp_connections, max_users, rate_limit_per_second, rate_limit_per_minute, rate_limit_per_hour, rate_limit_per_day, features) VALUES
('free', 'Free / Trial', 0, 1000, 1, 1, 5, 50, 500, 1000, '{"analytics": false, "webhooks": false, "hlr": false, "smart_routing": false, "sub_accounts": false, "white_label": false}'),
('starter', 'Starter', 5000, 50000, 1, 1, 10, 100, 1000, 10000, '{"analytics": true, "webhooks": false, "hlr": false, "smart_routing": false, "sub_accounts": false, "white_label": false}'),
('business', 'Business', 15000, 300000, 5, 5, 50, 500, 5000, 50000, '{"analytics": true, "webhooks": true, "hlr": true, "smart_routing": true, "sub_accounts": false, "white_label": false}'),
('pro', 'Pro', 40000, 1500000, 100, 50, 200, 2000, 20000, 200000, '{"analytics": true, "webhooks": true, "hlr": true, "smart_routing": true, "sub_accounts": true, "white_label": false}')
ON CONFLICT (name) DO NOTHING;

-- Add plan_id to clients
ALTER TABLE clients ADD COLUMN plan_id UUID REFERENCES subscription_plans(id) ON DELETE RESTRICT;

-- Set default plan (starter) for existing clients
UPDATE clients SET plan_id = (SELECT id FROM subscription_plans WHERE name = 'starter');

-- Make plan_id NOT NULL after backfill
ALTER TABLE clients ALTER COLUMN plan_id SET NOT NULL;

-- Add monthly usage tracking
ALTER TABLE clients ADD COLUMN monthly_sms_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE clients ADD COLUMN monthly_sms_reset_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT date_trunc('month', NOW()) + INTERVAL '1 month';

CREATE INDEX idx_clients_plan_id ON clients(plan_id);
CREATE INDEX idx_subscription_plans_active ON subscription_plans(active);
