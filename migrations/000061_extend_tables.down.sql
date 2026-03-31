DROP INDEX IF EXISTS idx_client_routes_default;

ALTER TABLE clients          DROP COLUMN IF EXISTS allocated_tps_budget;
ALTER TABLE client_routes    DROP COLUMN IF EXISTS shared;
ALTER TABLE client_providers DROP COLUMN IF EXISTS tps_limit;

ALTER TABLE tariff_plans
    DROP COLUMN IF EXISTS rate_limit_per_second,
    DROP COLUMN IF EXISTS rate_limit_per_minute,
    DROP COLUMN IF EXISTS rate_limit_per_hour,
    DROP COLUMN IF EXISTS default_tps_per_provider,
    DROP COLUMN IF EXISTS max_providers,
    DROP COLUMN IF EXISTS max_sub_accounts;

-- Восстановить NOT NULL на client_routes.client_id
-- (только если нет NULL-записей)
ALTER TABLE client_routes ALTER COLUMN client_id SET NOT NULL;
