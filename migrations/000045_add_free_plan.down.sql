UPDATE clients SET plan_id = NULL WHERE plan_id = (SELECT id FROM subscription_plans WHERE name = 'free' LIMIT 1);
DELETE FROM subscription_plans WHERE name = 'free';
