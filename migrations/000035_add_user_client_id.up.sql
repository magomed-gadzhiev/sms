ALTER TABLE users ADD COLUMN client_id UUID REFERENCES clients(id) ON DELETE SET NULL;
CREATE INDEX idx_users_client_id ON users(client_id);

-- Привязываем существующих пользователей к клиентам по email
UPDATE users u SET client_id = c.id
FROM clients c WHERE u.email = c.email AND u.client_id IS NULL;
