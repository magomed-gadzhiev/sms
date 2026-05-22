-- Удаление триггера
DROP TRIGGER IF EXISTS update_client_configs_updated_at ON client_configs;

-- Удаление индекса
DROP INDEX IF EXISTS idx_client_configs_client_id;
DROP INDEX IF EXISTS idx_clients_email;

-- Удаление таблицы client_configs
DROP TABLE IF EXISTS client_configs;

-- Удаление дополнительных полей из clients
ALTER TABLE clients 
    DROP COLUMN IF EXISTS email,
    DROP COLUMN IF EXISTS contact_person,
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS metadata,
    DROP COLUMN IF EXISTS rate_limit_per_day;
