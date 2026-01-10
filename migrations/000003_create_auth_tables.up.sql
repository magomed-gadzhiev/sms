-- Enable UUID extension (if not already enabled)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Roles table: роли пользователей
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_roles_name ON roles(name);

-- Permissions table: права доступа
CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(resource, action)
);

CREATE INDEX idx_permissions_resource ON permissions(resource);
CREATE INDEX idx_permissions_action ON permissions(action);

-- Role permissions: связь между ролями и правами
CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX idx_role_permissions_role_id ON role_permissions(role_id);
CREATE INDEX idx_role_permissions_permission_id ON role_permissions(permission_id);

-- Users table: пользователи системы
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role_id ON users(role_id);
CREATE INDEX idx_users_active ON users(active);

-- API Keys table: API ключи пользователей
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    key_prefix VARCHAR(10) NOT NULL, -- первые символы ключа для отображения
    active BOOLEAN DEFAULT true,
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_active ON api_keys(active);

-- API Key scopes: права доступа для API ключей
CREATE TABLE api_key_scopes (
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    scope VARCHAR(100) NOT NULL,
    PRIMARY KEY (api_key_id, scope)
);

CREATE INDEX idx_api_key_scopes_api_key_id ON api_key_scopes(api_key_id);

-- Refresh tokens table: токены для обновления access tokens
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked BOOLEAN DEFAULT false,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);

-- Триггер для обновления updated_at
CREATE TRIGGER update_roles_updated_at BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Вставляем базовые роли
INSERT INTO roles (id, name, description) VALUES
    ('00000000-0000-0000-0000-000000000001', 'admin', 'Администратор системы с полными правами'),
    ('00000000-0000-0000-0000-000000000002', 'client', 'Клиент с правами на отправку сообщений'),
    ('00000000-0000-0000-0000-000000000003', 'operator', 'Оператор с правами на просмотр и управление');

-- Вставляем базовые права доступа
INSERT INTO permissions (id, resource, action, description) VALUES
    ('10000000-0000-0000-0000-000000000001', 'messages', 'read', 'Чтение сообщений'),
    ('10000000-0000-0000-0000-000000000002', 'messages', 'write', 'Отправка сообщений'),
    ('10000000-0000-0000-0000-000000000003', 'messages', 'delete', 'Удаление сообщений'),
    ('10000000-0000-0000-0000-000000000004', 'clients', 'read', 'Просмотр клиентов'),
    ('10000000-0000-0000-0000-000000000005', 'clients', 'write', 'Создание и изменение клиентов'),
    ('10000000-0000-0000-0000-000000000006', 'clients', 'delete', 'Удаление клиентов'),
    ('10000000-0000-0000-0000-000000000007', 'providers', 'read', 'Просмотр провайдеров'),
    ('10000000-0000-0000-0000-000000000008', 'providers', 'write', 'Создание и изменение провайдеров'),
    ('10000000-0000-0000-0000-000000000009', 'providers', 'delete', 'Удаление провайдеров'),
    ('10000000-0000-0000-0000-000000000010', 'routes', 'read', 'Просмотр маршрутов'),
    ('10000000-0000-0000-0000-000000000011', 'routes', 'write', 'Создание и изменение маршрутов'),
    ('10000000-0000-0000-0000-000000000012', 'routes', 'delete', 'Удаление маршрутов'),
    ('10000000-0000-0000-0000-000000000013', 'analytics', 'read', 'Просмотр аналитики'),
    ('10000000-0000-0000-0000-000000000014', 'billing', 'read', 'Просмотр биллинга'),
    ('10000000-0000-0000-0000-000000000015', 'billing', 'write', 'Управление биллингом'),
    ('10000000-0000-0000-0000-000000000016', 'users', 'read', 'Просмотр пользователей'),
    ('10000000-0000-0000-0000-000000000017', 'users', 'write', 'Создание и изменение пользователей'),
    ('10000000-0000-0000-0000-000000000018', 'users', 'delete', 'Удаление пользователей');

-- Назначаем права роли admin (все права)
INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000001', id FROM permissions;

-- Назначаем права роли client (messages: read, write)
INSERT INTO role_permissions (role_id, permission_id) VALUES
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001'), -- messages:read
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000002'), -- messages:write
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000013'); -- analytics:read

-- Назначаем права роли operator (read права на все ресурсы)
INSERT INTO role_permissions (role_id, permission_id) VALUES
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001'), -- messages:read
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000004'), -- clients:read
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000007'), -- providers:read
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000010'), -- routes:read
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000013'), -- analytics:read
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000014'), -- billing:read
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000016'); -- users:read
