-- Откат: возврат дефолтов к USD
ALTER TABLE accounts ALTER COLUMN currency SET DEFAULT 'USD';
ALTER TABLE transactions ALTER COLUMN currency SET DEFAULT 'USD';
ALTER TABLE pricing_rules ALTER COLUMN currency SET DEFAULT 'USD';

CREATE OR REPLACE FUNCTION create_account_for_new_client()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO accounts (id, client_id, balance, currency, created_at, updated_at)
    VALUES (uuid_generate_v4(), NEW.id, 0, 'USD', NOW(), NOW())
    ON CONFLICT (client_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
