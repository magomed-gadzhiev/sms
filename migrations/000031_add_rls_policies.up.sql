-- IMPORTANT: When app.current_client_id is not set, current_setting returns NULL,
-- which causes the policy to block all rows for sms_app role.
-- Use superuser or BYPASSRLS role for service/background operations.

-- Enable RLS on tables with client data
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE client_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE pricing_rules ENABLE ROW LEVEL SECURITY;

-- Policy: messages — клиент видит только свои сообщения
CREATE POLICY messages_tenant_isolation ON messages
    USING (client_id = current_setting('app.current_client_id', true)::uuid);

-- Policy: accounts — клиент видит только свой аккаунт
CREATE POLICY accounts_tenant_isolation ON accounts
    USING (client_id = current_setting('app.current_client_id', true)::uuid);

-- Policy: transactions — клиент видит только свои транзакции
CREATE POLICY transactions_tenant_isolation ON transactions
    USING (client_id = current_setting('app.current_client_id', true)::uuid);

-- Policy: client_configs — клиент видит только свои конфиги
CREATE POLICY client_configs_tenant_isolation ON client_configs
    USING (client_id = current_setting('app.current_client_id', true)::uuid);

-- Policy: pricing_rules — клиент видит свои правила + глобальные (client_id IS NULL)
CREATE POLICY pricing_rules_tenant_isolation ON pricing_rules
    USING (client_id = current_setting('app.current_client_id', true)::uuid OR client_id IS NULL);

-- Policy: templates
ALTER TABLE templates ENABLE ROW LEVEL SECURITY;
CREATE POLICY templates_tenant_isolation ON templates
    USING (client_id = current_setting('app.current_client_id', true)::uuid);

-- Policy: webhook_subscriptions
ALTER TABLE webhook_subscriptions ENABLE ROW LEVEL SECURITY;
CREATE POLICY webhook_subscriptions_tenant_isolation ON webhook_subscriptions
    USING (client_id = current_setting('app.current_client_id', true)::uuid);

-- Создаём роль для приложения если не существует
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'sms_app') THEN
        CREATE ROLE sms_app LOGIN;
        -- Note: password must be set via deployment secrets (e.g., Ansible vault, env-based ALTER ROLE)
    END IF;
END
$$;

-- Даём права на таблицы
GRANT SELECT, INSERT, UPDATE, DELETE ON messages TO sms_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON accounts TO sms_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON transactions TO sms_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON client_configs TO sms_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON pricing_rules TO sms_app;
GRANT SELECT ON subscription_plans TO sms_app;
GRANT SELECT, UPDATE ON clients TO sms_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON templates TO sms_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON webhook_subscriptions TO sms_app;
