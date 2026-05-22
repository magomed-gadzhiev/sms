-- Создаём аккаунты для существующих клиентов, у которых их нет
INSERT INTO accounts (id, client_id, balance, currency, created_at, updated_at)
SELECT
    uuid_generate_v4(),
    c.id,
    0,
    'RUB',
    NOW(),
    NOW()
FROM clients c
LEFT JOIN accounts a ON a.client_id = c.id
WHERE a.client_id IS NULL;

-- Триггерная функция: автоматически создаёт аккаунт при добавлении клиента
CREATE OR REPLACE FUNCTION create_account_for_new_client()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO accounts (id, client_id, balance, currency, created_at, updated_at)
    VALUES (uuid_generate_v4(), NEW.id, 0, 'RUB', NOW(), NOW())
    ON CONFLICT (client_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_create_account_for_new_client
AFTER INSERT ON clients
FOR EACH ROW EXECUTE FUNCTION create_account_for_new_client();
