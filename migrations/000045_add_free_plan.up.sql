INSERT INTO subscription_plans (id, name, display_name, monthly_price_rub, max_sms_per_month, max_smpp_connections, max_users, active)
VALUES (gen_random_uuid(), 'free', 'Free', 0, 100, 1, 1, true)
ON CONFLICT (name) DO NOTHING;

UPDATE clients SET plan_id = (SELECT id FROM subscription_plans WHERE name = 'free' LIMIT 1)
WHERE plan_id IS NULL;
