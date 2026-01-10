-- Расширение таблицы clients для поддержки дополнительных полей
ALTER TABLE clients 
    ADD COLUMN IF NOT EXISTS email VARCHAR(255),
    ADD COLUMN IF NOT EXISTS contact_person VARCHAR(255),
    ADD COLUMN IF NOT EXISTS phone VARCHAR(50),
    ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS rate_limit_per_day INTEGER DEFAULT 10000;

-- Создание индекса для email
CREATE INDEX IF NOT EXISTS idx_clients_email ON clients(email);

-- Создание таблицы client_configs для конфигурации клиентов
CREATE TABLE IF NOT EXISTS client_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id UUID NOT NULL UNIQUE REFERENCES clients(id) ON DELETE CASCADE,
    rate_limit_per_second INTEGER DEFAULT 10,
    rate_limit_per_minute INTEGER DEFAULT 100,
    rate_limit_per_hour INTEGER DEFAULT 1000,
    rate_limit_per_day INTEGER DEFAULT 10000,
    allowed_sources TEXT[] DEFAULT '{}',
    blocked_destinations TEXT[] DEFAULT '{}',
    settings JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Создание индекса для client_id
CREATE INDEX IF NOT EXISTS idx_client_configs_client_id ON client_configs(client_id);

-- Триггер для обновления updated_at в client_configs
CREATE TRIGGER update_client_configs_updated_at BEFORE UPDATE ON client_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
