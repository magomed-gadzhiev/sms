-- Приведение всех валют к единому стандарту RUB

-- Обновить дефолты таблиц
ALTER TABLE accounts ALTER COLUMN currency SET DEFAULT 'RUB';
ALTER TABLE transactions ALTER COLUMN currency SET DEFAULT 'RUB';
ALTER TABLE pricing_rules ALTER COLUMN currency SET DEFAULT 'RUB';

-- Обновить существующие данные с USD/EUR на RUB
UPDATE accounts SET currency = 'RUB' WHERE currency != 'RUB';
UPDATE transactions SET currency = 'RUB' WHERE currency != 'RUB';
UPDATE pricing_rules SET currency = 'RUB' WHERE currency != 'RUB';
UPDATE balance_transfers SET currency = 'RUB' WHERE currency != 'RUB';

-- Обновить триггерную функцию для новых клиентов
CREATE OR REPLACE FUNCTION create_account_for_new_client()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO accounts (id, client_id, balance, currency, created_at, updated_at)
    VALUES (uuid_generate_v4(), NEW.id, 0, 'RUB', NOW(), NOW())
    ON CONFLICT (client_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
