-- tariff_plans: поля лимитов (NULL = брать из system_defaults)
ALTER TABLE tariff_plans
    ADD COLUMN IF NOT EXISTS rate_limit_per_second    INT,
    ADD COLUMN IF NOT EXISTS rate_limit_per_minute    INT,
    ADD COLUMN IF NOT EXISTS rate_limit_per_hour      INT,
    ADD COLUMN IF NOT EXISTS default_tps_per_provider INT,
    ADD COLUMN IF NOT EXISTS max_providers            INT,
    ADD COLUMN IF NOT EXISTS max_sub_accounts         INT;

-- client_providers: per-client TPS лимит на провайдера (NULL = из tariff или system_defaults)
ALTER TABLE client_providers
    ADD COLUMN IF NOT EXISTS tps_limit INT;

-- client_routes: client_id nullable (NULL = платформенный дефолт) + shared flag
ALTER TABLE client_routes
    ALTER COLUMN client_id DROP NOT NULL;

ALTER TABLE client_routes
    ADD COLUMN IF NOT EXISTS shared BOOLEAN NOT NULL DEFAULT false;

-- Индекс для платформенных дефолтных маршрутов
CREATE INDEX IF NOT EXISTS idx_client_routes_default
    ON client_routes (operator_id)
    WHERE client_id IS NULL AND active = true;

-- clients: TPS-бюджет для субаккаунтов (NULL = не ограничено)
ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS allocated_tps_budget INT;
