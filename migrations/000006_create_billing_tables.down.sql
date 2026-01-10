-- Удаление триггеров
DROP TRIGGER IF EXISTS update_pricing_rules_updated_at ON pricing_rules;
DROP TRIGGER IF EXISTS update_accounts_updated_at ON accounts;

-- Удаление таблиц (в обратном порядке из-за внешних ключей)
DROP TABLE IF EXISTS pricing_rules;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS accounts;
