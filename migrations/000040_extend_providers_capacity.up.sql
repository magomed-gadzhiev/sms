-- Note: tps_limit already exists from migration 000033. We only add quotas.
ALTER TABLE providers ADD COLUMN IF NOT EXISTS daily_quota INT;
ALTER TABLE providers ADD COLUMN IF NOT EXISTS monthly_quota INT;
