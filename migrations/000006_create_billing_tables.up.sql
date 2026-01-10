-- Таблица для счетов клиентов
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    balance NUMERIC(20, 6) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    UNIQUE(client_id)
);

CREATE INDEX idx_accounts_client_id ON accounts(client_id);

-- Таблица для транзакций
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('charge', 'credit', 'refund', 'adjustment')),
    amount NUMERIC(20, 6) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    balance_before NUMERIC(20, 6) NOT NULL,
    balance_after NUMERIC(20, 6) NOT NULL,
    description TEXT,
    message_id UUID REFERENCES messages(id) ON DELETE SET NULL,
    payment_method VARCHAR(50),
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_transactions_client_id ON transactions(client_id);
CREATE INDEX idx_transactions_type ON transactions(type);
CREATE INDEX idx_transactions_created_at ON transactions(created_at);
CREATE INDEX idx_transactions_message_id ON transactions(message_id);
CREATE INDEX idx_transactions_client_created ON transactions(client_id, created_at DESC);

-- Таблица для правил тарификации
CREATE TABLE pricing_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id UUID REFERENCES clients(id) ON DELETE CASCADE, -- NULL для глобальных правил
    destination_pattern VARCHAR(255) NOT NULL, -- regex паттерн номера получателя
    price_per_message NUMERIC(20, 6) NOT NULL CHECK (price_per_message >= 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    priority INTEGER NOT NULL DEFAULT 0, -- Чем выше, тем выше приоритет
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_pricing_rules_client_id ON pricing_rules(client_id);
CREATE INDEX idx_pricing_rules_active ON pricing_rules(active, priority DESC);
CREATE INDEX idx_pricing_rules_client_active ON pricing_rules(client_id, active, priority DESC) WHERE client_id IS NOT NULL;
CREATE INDEX idx_pricing_rules_global_active ON pricing_rules(active, priority DESC) WHERE client_id IS NULL;

-- Функция для автоматического обновления updated_at
CREATE TRIGGER update_accounts_updated_at BEFORE UPDATE ON accounts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_pricing_rules_updated_at BEFORE UPDATE ON pricing_rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
