-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Providers table: SMSC провайдеры
CREATE TABLE providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    host VARCHAR(255) NOT NULL,
    port INTEGER NOT NULL DEFAULT 2775,
    system_id VARCHAR(255) NOT NULL,
    password VARCHAR(255) NOT NULL,
    system_type VARCHAR(50) DEFAULT 'CMT',
    bind_type VARCHAR(20) NOT NULL CHECK (bind_type IN ('transceiver', 'transmitter', 'receiver')),
    bind_ton INTEGER DEFAULT 0,
    bind_npi INTEGER DEFAULT 0,
    addr_ton INTEGER DEFAULT 0,
    addr_npi INTEGER DEFAULT 0,
    address_range VARCHAR(255) DEFAULT '',
    max_connections INTEGER DEFAULT 1,
    active BOOLEAN DEFAULT true,
    priority INTEGER DEFAULT 0,
    throughput_per_second INTEGER DEFAULT 10,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_providers_active ON providers(active);
CREATE INDEX idx_providers_priority ON providers(priority);

-- Routes table: правила маршрутизации
CREATE TABLE routes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    pattern VARCHAR(255) NOT NULL, -- префикс номера или паттерн
    pattern_type VARCHAR(20) NOT NULL CHECK (pattern_type IN ('prefix', 'regex', 'exact')),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
    priority INTEGER DEFAULT 0,
    active BOOLEAN DEFAULT true,
    failover_provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_routes_pattern ON routes(pattern);
CREATE INDEX idx_routes_active ON routes(active, priority);
CREATE INDEX idx_routes_provider ON routes(provider_id);

-- Clients table: клиенты API
CREATE TABLE clients (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    api_key VARCHAR(255) NOT NULL UNIQUE,
    secret VARCHAR(255) NOT NULL,
    active BOOLEAN DEFAULT true,
    rate_limit_per_second INTEGER DEFAULT 10,
    rate_limit_per_minute INTEGER DEFAULT 100,
    rate_limit_per_hour INTEGER DEFAULT 1000,
    allowed_source_addresses TEXT[], -- массив разрешенных source адресов
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_clients_api_key ON clients(api_key);
CREATE INDEX idx_clients_active ON clients(active);

-- Messages table: основная таблица сообщений (партиционированная по месяцам)
CREATE TABLE messages (
    id UUID DEFAULT uuid_generate_v4(),
    message_id VARCHAR(255), -- внутренний ID сообщения
    external_id VARCHAR(255), -- ID от клиента/SMPP
    source VARCHAR(20) NOT NULL,
    destination VARCHAR(20) NOT NULL,
    text TEXT NOT NULL,
    encoding VARCHAR(20) DEFAULT 'GSM7' CHECK (encoding IN ('GSM7', 'UCS2', 'ASCII')),
    data_coding INTEGER DEFAULT 0,
    esm_class INTEGER DEFAULT 0,
    protocol_id INTEGER DEFAULT 0,
    priority_flag INTEGER DEFAULT 0,
    replace_if_present INTEGER DEFAULT 0,
    registered_delivery INTEGER DEFAULT 1,
    validity_period TIMESTAMP WITH TIME ZONE,
    service_type VARCHAR(50) DEFAULT '',
    source_addr_ton INTEGER DEFAULT 0,
    source_addr_npi INTEGER DEFAULT 0,
    dest_addr_ton INTEGER DEFAULT 0,
    dest_addr_npi INTEGER DEFAULT 0,
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN ('pending', 'queued', 'sent', 'delivered', 'failed', 'expired', 'rejected')),
    status_message TEXT,
    provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    route_id UUID REFERENCES routes(id) ON DELETE SET NULL,
    client_id UUID REFERENCES clients(id) ON DELETE SET NULL,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 5,
    next_retry_at TIMESTAMP WITH TIME ZONE,
    smpp_message_id VARCHAR(255), -- ID от SMSC провайдера
    submitted_at TIMESTAMP WITH TIME ZONE,
    delivered_at TIMESTAMP WITH TIME ZONE,
    failed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Создаем партиции для messages на год вперед (по месяцам)
CREATE TABLE messages_y2024m01 PARTITION OF messages
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
CREATE TABLE messages_y2024m02 PARTITION OF messages
    FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');
CREATE TABLE messages_y2024m03 PARTITION OF messages
    FOR VALUES FROM ('2024-03-01') TO ('2024-04-01');
CREATE TABLE messages_y2024m04 PARTITION OF messages
    FOR VALUES FROM ('2024-04-01') TO ('2024-05-01');
CREATE TABLE messages_y2024m05 PARTITION OF messages
    FOR VALUES FROM ('2024-05-01') TO ('2024-06-01');
CREATE TABLE messages_y2024m06 PARTITION OF messages
    FOR VALUES FROM ('2024-06-01') TO ('2024-07-01');
CREATE TABLE messages_y2024m07 PARTITION OF messages
    FOR VALUES FROM ('2024-07-01') TO ('2024-08-01');
CREATE TABLE messages_y2024m08 PARTITION OF messages
    FOR VALUES FROM ('2024-08-01') TO ('2024-09-01');
CREATE TABLE messages_y2024m09 PARTITION OF messages
    FOR VALUES FROM ('2024-09-01') TO ('2024-10-01');
CREATE TABLE messages_y2024m10 PARTITION OF messages
    FOR VALUES FROM ('2024-10-01') TO ('2024-11-01');
CREATE TABLE messages_y2024m11 PARTITION OF messages
    FOR VALUES FROM ('2024-11-01') TO ('2024-12-01');
CREATE TABLE messages_y2024m12 PARTITION OF messages
    FOR VALUES FROM ('2024-12-01') TO ('2025-01-01');

-- Индексы для messages (будут применены к каждой партиции автоматически)
CREATE INDEX idx_messages_message_id ON messages(message_id);
CREATE INDEX idx_messages_external_id ON messages(external_id);
CREATE INDEX idx_messages_destination ON messages(destination);
CREATE INDEX idx_messages_status ON messages(status);
CREATE INDEX idx_messages_client_id ON messages(client_id);
CREATE INDEX idx_messages_provider_id ON messages(provider_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);
CREATE INDEX idx_messages_next_retry_at ON messages(next_retry_at) WHERE status = 'failed' AND next_retry_at IS NOT NULL;

-- DLR Receipts table: delivery receipts от SMSC
CREATE TABLE dlr_receipts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id UUID NOT NULL,
    message_created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    smpp_message_id VARCHAR(255) NOT NULL,
    provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    receipted_message_id VARCHAR(255),
    submit_date TIMESTAMP WITH TIME ZONE,
    done_date TIMESTAMP WITH TIME ZONE,
    stat VARCHAR(50) NOT NULL,
    err INTEGER,
    text TEXT,
    source VARCHAR(20),
    destination VARCHAR(20),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    FOREIGN KEY (message_id, message_created_at) REFERENCES messages(id, created_at) ON DELETE CASCADE
);

CREATE INDEX idx_dlr_receipts_message_id ON dlr_receipts(message_id);
CREATE INDEX idx_dlr_receipts_smpp_message_id ON dlr_receipts(smpp_message_id);
CREATE INDEX idx_dlr_receipts_created_at ON dlr_receipts(created_at);

-- Audit Log table: аудит операций
CREATE TABLE audit_log (
    id UUID DEFAULT uuid_generate_v4(),
    entity_type VARCHAR(50) NOT NULL, -- 'message', 'provider', 'client', 'route'
    entity_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL, -- 'create', 'update', 'delete', 'send', 'deliver', 'fail'
    actor_type VARCHAR(50), -- 'client', 'system', 'provider'
    actor_id UUID,
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Партиции для audit_log (по месяцам, на год)
CREATE TABLE audit_log_y2024m01 PARTITION OF audit_log
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
CREATE TABLE audit_log_y2024m02 PARTITION OF audit_log
    FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');
CREATE TABLE audit_log_y2024m03 PARTITION OF audit_log
    FOR VALUES FROM ('2024-03-01') TO ('2024-04-01');
CREATE TABLE audit_log_y2024m04 PARTITION OF audit_log
    FOR VALUES FROM ('2024-04-01') TO ('2024-05-01');
CREATE TABLE audit_log_y2024m05 PARTITION OF audit_log
    FOR VALUES FROM ('2024-05-01') TO ('2024-06-01');
CREATE TABLE audit_log_y2024m06 PARTITION OF audit_log
    FOR VALUES FROM ('2024-06-01') TO ('2024-07-01');
CREATE TABLE audit_log_y2024m07 PARTITION OF audit_log
    FOR VALUES FROM ('2024-07-01') TO ('2024-08-01');
CREATE TABLE audit_log_y2024m08 PARTITION OF audit_log
    FOR VALUES FROM ('2024-08-01') TO ('2024-09-01');
CREATE TABLE audit_log_y2024m09 PARTITION OF audit_log
    FOR VALUES FROM ('2024-09-01') TO ('2024-10-01');
CREATE TABLE audit_log_y2024m10 PARTITION OF audit_log
    FOR VALUES FROM ('2024-10-01') TO ('2024-11-01');
CREATE TABLE audit_log_y2024m11 PARTITION OF audit_log
    FOR VALUES FROM ('2024-11-01') TO ('2024-12-01');
CREATE TABLE audit_log_y2024m12 PARTITION OF audit_log
    FOR VALUES FROM ('2024-12-01') TO ('2025-01-01');

-- Индексы для audit_log
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_log_actor ON audit_log(actor_type, actor_id);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);
CREATE INDEX idx_audit_log_action ON audit_log(action);

-- Функция для автоматического обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Триггеры для обновления updated_at
CREATE TRIGGER update_providers_updated_at BEFORE UPDATE ON providers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_routes_updated_at BEFORE UPDATE ON routes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_clients_updated_at BEFORE UPDATE ON clients
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_messages_updated_at BEFORE UPDATE ON messages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
