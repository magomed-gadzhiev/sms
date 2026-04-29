-- Seed sandbox test users (aggregator + subaccount) used for UX-audit and manual QA.
-- Password: Admin123!  (bcrypt cost=10, same hash as admin seed in 000026)
--
-- PRODUCTION GUARD: миграция skip'ается, если app.environment='production'.
-- Чтобы заблокировать на проде, ops one-time:
--   ALTER DATABASE smpp_db SET app.environment = 'production';
-- Sandbox/dev/CI без выставленной GUC — выполняется.

DO $$
BEGIN
    IF current_setting('app.environment', true) = 'production' THEN
        RAISE NOTICE 'Skipping sandbox seed (000116) on production environment';
        RETURN;
    END IF;

    -- Aggregator reseller client
    INSERT INTO clients (id, name, is_reseller, active, created_at, updated_at)
    VALUES (
        'a0000000-0000-0000-0000-000000000001',
        'Aggregator Test',
        true,
        true,
        NOW(),
        NOW()
    ) ON CONFLICT (id) DO NOTHING;

    -- Aggregator portal user
    INSERT INTO users (id, username, email, password_hash, role_id, active, client_id, created_at, updated_at)
    VALUES (
        '0a233a58-ffde-44f9-9025-63559da7234c',
        'aggregator',
        'aggregator@test.local',
        '$2a$10$Od96EF1oN2Cp7JPpMNyLCuHs.mN3dRB82Ukq8TRhEDgKokaDj2jYO',
        '00000000-0000-0000-0000-000000000002', -- client role
        true,
        'a0000000-0000-0000-0000-000000000001',
        NOW(),
        NOW()
    ) ON CONFLICT (username) DO UPDATE
        SET password_hash = EXCLUDED.password_hash,
            email = EXCLUDED.email,
            active = true,
            client_id = EXCLUDED.client_id,
            updated_at = NOW();
END $$;
