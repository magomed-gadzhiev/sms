DROP POLICY IF EXISTS messages_tenant_isolation ON messages;
DROP POLICY IF EXISTS accounts_tenant_isolation ON accounts;
DROP POLICY IF EXISTS transactions_tenant_isolation ON transactions;
DROP POLICY IF EXISTS client_configs_tenant_isolation ON client_configs;
DROP POLICY IF EXISTS pricing_rules_tenant_isolation ON pricing_rules;
DROP POLICY IF EXISTS templates_tenant_isolation ON templates;
DROP POLICY IF EXISTS webhook_subscriptions_tenant_isolation ON webhook_subscriptions;

ALTER TABLE messages DISABLE ROW LEVEL SECURITY;
ALTER TABLE accounts DISABLE ROW LEVEL SECURITY;
ALTER TABLE transactions DISABLE ROW LEVEL SECURITY;
ALTER TABLE client_configs DISABLE ROW LEVEL SECURITY;
ALTER TABLE pricing_rules DISABLE ROW LEVEL SECURITY;
ALTER TABLE templates DISABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_subscriptions DISABLE ROW LEVEL SECURITY;

-- Note: REVOKE and DROP ROLE sms_app omitted intentionally
-- The role may be shared by other components
