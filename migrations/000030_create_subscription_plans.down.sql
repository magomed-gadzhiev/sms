ALTER TABLE clients DROP COLUMN IF EXISTS monthly_sms_reset_at;
ALTER TABLE clients DROP COLUMN IF EXISTS monthly_sms_count;
ALTER TABLE clients DROP COLUMN IF EXISTS plan_id;
DROP TABLE IF EXISTS subscription_plans;
