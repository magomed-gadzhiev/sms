-- Seed initial admin user
-- Username: admin, Password: Admin123!
-- bcrypt hash generated with cost 10
INSERT INTO users (id, username, email, password_hash, role_id, active, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000100',
    'admin',
    'admin@example.com',
    '$2a$10$Od96EF1oN2Cp7JPpMNyLCuHs.mN3dRB82Ukq8TRhEDgKokaDj2jYO',
    '00000000-0000-0000-0000-000000000001', -- admin role
    true,
    NOW(),
    NOW()
) ON CONFLICT (username) DO NOTHING;
