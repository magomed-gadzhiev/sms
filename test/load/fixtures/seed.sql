-- =============================================================================
-- Seed data для нагрузочного тестирования SMS Gateway Platform
-- Выполнять перед запуском load-тестов.
-- Требует выполненных миграций (000001–000025).
-- =============================================================================

BEGIN;

-- ============================================================
-- 0. Партиции messages и audit_log для 2026 года
-- ============================================================
DO $$
BEGIN
    FOR m IN 1..12 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS messages_y2026m%s PARTITION OF messages
             FOR VALUES FROM (%L) TO (%L)',
            lpad(m::text, 2, '0'),
            format('2026-%s-01', lpad(m::text, 2, '0')),
            CASE WHEN m = 12
                THEN '2027-01-01'
                ELSE format('2026-%s-01', lpad((m+1)::text, 2, '0'))
            END
        );
    END LOOP;
END $$;

DO $$
BEGIN
    FOR m IN 1..12 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS audit_log_y2026m%s PARTITION OF audit_log
             FOR VALUES FROM (%L) TO (%L)',
            lpad(m::text, 2, '0'),
            format('2026-%s-01', lpad(m::text, 2, '0')),
            CASE WHEN m = 12
                THEN '2027-01-01'
                ELSE format('2026-%s-01', lpad((m+1)::text, 2, '0'))
            END
        );
    END LOOP;
END $$;

-- Партиции messages на 2025
DO $$
BEGIN
    FOR m IN 1..12 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS messages_y2025m%s PARTITION OF messages
             FOR VALUES FROM (%L) TO (%L)',
            lpad(m::text, 2, '0'),
            format('2025-%s-01', lpad(m::text, 2, '0')),
            CASE WHEN m = 12
                THEN '2026-01-01'
                ELSE format('2025-%s-01', lpad((m+1)::text, 2, '0'))
            END
        );
    END LOOP;
END $$;

-- ============================================================
-- 1. Провайдеры (SMSC)
-- ============================================================
INSERT INTO providers (id, name, host, port, system_id, password, system_type, bind_type,
                       max_connections, active, priority, throughput_per_second)
VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Provider-MTS-RU',      'smsc-mts.local',    2775, 'mts_sys',    'mts_pass',    'CMT', 'transceiver', 20, true, 1, 500),
    ('a0000000-0000-0000-0000-000000000002', 'Provider-Beeline-RU',  'smsc-bee.local',    2775, 'bee_sys',    'bee_pass',    'CMT', 'transceiver', 15, true, 2, 400),
    ('a0000000-0000-0000-0000-000000000003', 'Provider-Megafon-RU',  'smsc-mega.local',   2775, 'mega_sys',   'mega_pass',   'CMT', 'transceiver', 15, true, 3, 400),
    ('a0000000-0000-0000-0000-000000000004', 'Provider-Tele2-RU',    'smsc-tele2.local',  2775, 'tele2_sys',  'tele2_pass',  'CMT', 'transceiver', 10, true, 4, 300),
    ('a0000000-0000-0000-0000-000000000005', 'Provider-Tinkoff-RU',  'smsc-tink.local',   2775, 'tink_sys',   'tink_pass',   'CMT', 'transceiver', 10, true, 5, 200),
    ('a0000000-0000-0000-0000-000000000006', 'Provider-International', 'smsc-intl.local', 2775, 'intl_sys',   'intl_pass',   'CMT', 'transceiver',  5, true, 10, 100),
    ('a0000000-0000-0000-0000-000000000007', 'Provider-Backup',      'smsc-backup.local', 2775, 'backup_sys', 'backup_pass', 'CMT', 'transceiver',  5, true, 99, 50)
ON CONFLICT (name) DO NOTHING;

-- ============================================================
-- 2. Маршруты
-- ============================================================
INSERT INTO routes (id, name, pattern, pattern_type, provider_id, priority, active, failover_provider_id)
VALUES
    -- Российские операторы (по префиксам)
    ('b0000000-0000-0000-0000-000000000001', 'MTS-Russia',       '7910', 'prefix', 'a0000000-0000-0000-0000-000000000001', 1, true, 'a0000000-0000-0000-0000-000000000007'),
    ('b0000000-0000-0000-0000-000000000002', 'MTS-Russia-2',     '7916', 'prefix', 'a0000000-0000-0000-0000-000000000001', 1, true, 'a0000000-0000-0000-0000-000000000007'),
    ('b0000000-0000-0000-0000-000000000003', 'MTS-Russia-3',     '7985', 'prefix', 'a0000000-0000-0000-0000-000000000001', 1, true, 'a0000000-0000-0000-0000-000000000007'),
    ('b0000000-0000-0000-0000-000000000004', 'Beeline-Russia',   '7903', 'prefix', 'a0000000-0000-0000-0000-000000000002', 2, true, 'a0000000-0000-0000-0000-000000000007'),
    ('b0000000-0000-0000-0000-000000000005', 'Beeline-Russia-2', '7906', 'prefix', 'a0000000-0000-0000-0000-000000000002', 2, true, 'a0000000-0000-0000-0000-000000000007'),
    ('b0000000-0000-0000-0000-000000000006', 'Megafon-Russia',   '7920', 'prefix', 'a0000000-0000-0000-0000-000000000003', 3, true, 'a0000000-0000-0000-0000-000000000007'),
    ('b0000000-0000-0000-0000-000000000007', 'Megafon-Russia-2', '7925', 'prefix', 'a0000000-0000-0000-0000-000000000003', 3, true, 'a0000000-0000-0000-0000-000000000007'),
    ('b0000000-0000-0000-0000-000000000008', 'Tele2-Russia',     '7900', 'prefix', 'a0000000-0000-0000-0000-000000000004', 4, true, 'a0000000-0000-0000-0000-000000000007'),
    ('b0000000-0000-0000-0000-000000000009', 'Tele2-Russia-2',   '7901', 'prefix', 'a0000000-0000-0000-0000-000000000004', 4, true, 'a0000000-0000-0000-0000-000000000007'),
    ('b0000000-0000-0000-0000-000000000010', 'Tinkoff-Russia',   '7999', 'prefix', 'a0000000-0000-0000-0000-000000000005', 5, true, 'a0000000-0000-0000-0000-000000000007'),
    -- Международные
    ('b0000000-0000-0000-0000-000000000011', 'International-CIS', '375',  'prefix', 'a0000000-0000-0000-0000-000000000006', 10, true, NULL),
    ('b0000000-0000-0000-0000-000000000012', 'International-KZ',  '77',   'prefix', 'a0000000-0000-0000-0000-000000000006', 10, true, NULL),
    -- Catch-all (низкий приоритет)
    ('b0000000-0000-0000-0000-000000000013', 'Default-Fallback',  '7',    'prefix', 'a0000000-0000-0000-0000-000000000007', 100, true, NULL)
ON CONFLICT DO NOTHING;

-- ============================================================
-- 3. Клиенты для нагрузочного тестирования
-- ============================================================
INSERT INTO clients (id, name, api_key, secret, active,
                     rate_limit_per_second, rate_limit_per_minute, rate_limit_per_hour,
                     allowed_source_addresses)
VALUES
    -- Высоконагруженный клиент (основной для load-тестов)
    ('c0000000-0000-0000-0000-000000000001', 'LoadTest-HighVolume', 'lt-high-volume-key-001', 'lt-secret-001', true,
     10000, 600000, 36000000,
     ARRAY['LoadTest', 'LT-INFO', 'LT-PROMO', 'LT-OTP', 'LT-ALERT']),
    -- Средненагруженный клиент
    ('c0000000-0000-0000-0000-000000000002', 'LoadTest-MedVolume',  'lt-med-volume-key-002',  'lt-secret-002', true,
     1000, 60000, 3600000,
     ARRAY['MedTest', 'MT-INFO', 'MT-OTP']),
    -- Лёгкий клиент
    ('c0000000-0000-0000-0000-000000000003', 'LoadTest-LowVolume',  'lt-low-volume-key-003',  'lt-secret-003', true,
     100, 6000, 360000,
     ARRAY['LowTest', 'LT-API']),
    -- Клиент с дефолтным test-api-key (совместимость с существующими тестами)
    ('c0000000-0000-0000-0000-000000000004', 'LoadTest-Default',    'test-api-key',            'test-secret',   true,
     10000, 600000, 36000000,
     ARRAY['12345', '67890', 'SMS', 'API', 'TEST'])
ON CONFLICT (api_key) DO UPDATE SET
    rate_limit_per_second = EXCLUDED.rate_limit_per_second,
    rate_limit_per_minute = EXCLUDED.rate_limit_per_minute,
    rate_limit_per_hour   = EXCLUDED.rate_limit_per_hour,
    active                = true;

-- ============================================================
-- 4. Пользователи и API-ключи для auth (портальные тесты)
-- ============================================================
INSERT INTO users (id, username, email, password_hash, role_id, active, client_id)
VALUES
    ('d0000000-0000-0000-0000-000000000001', 'loadtest-admin', 'loadtest-admin@test.local',
     '$2a$10$Od96EF1oN2Cp7JPpMNyLCuHs.mN3dRB82Ukq8TRhEDgKokaDj2jYO',  -- Admin123!
     '00000000-0000-0000-0000-000000000001', true, NULL),
    ('d0000000-0000-0000-0000-000000000002', 'loadtest-client', 'loadtest-client@test.local',
     '$2a$10$Od96EF1oN2Cp7JPpMNyLCuHs.mN3dRB82Ukq8TRhEDgKokaDj2jYO',  -- Admin123!
     '00000000-0000-0000-0000-000000000002', true, 'c0000000-0000-0000-0000-000000000001')
ON CONFLICT (username) DO UPDATE SET
    client_id     = EXCLUDED.client_id,
    password_hash = EXCLUDED.password_hash;

INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, active)
VALUES
    ('e0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000002',
     'loadtest-primary', 'hash_lt_high_volume_key_001', 'lt-high-', true),
    ('e0000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000002',
     'loadtest-default', 'hash_test_api_key', 'test-api', true)
ON CONFLICT DO NOTHING;

INSERT INTO api_key_scopes (api_key_id, scope)
VALUES
    ('e0000000-0000-0000-0000-000000000001', 'messages:send'),
    ('e0000000-0000-0000-0000-000000000001', 'messages:read'),
    ('e0000000-0000-0000-0000-000000000001', 'messages:delete'),
    ('e0000000-0000-0000-0000-000000000002', 'messages:send'),
    ('e0000000-0000-0000-0000-000000000002', 'messages:read')
ON CONFLICT DO NOTHING;

-- ============================================================
-- 5. Биллинг-аккаунты с высокими балансами
-- ============================================================
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'billing_accounts') THEN
        INSERT INTO billing_accounts (id, client_id, balance, currency)
        VALUES
            (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000001', 999999999.00, 'RUB'),
            (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000002', 999999999.00, 'RUB'),
            (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000003', 999999999.00, 'RUB'),
            (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000004', 999999999.00, 'RUB')
        ON CONFLICT DO NOTHING;
    END IF;
END $$;

-- ============================================================
-- 6. HLR-провайдеры
-- ============================================================
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'hlr_providers') THEN
        INSERT INTO hlr_providers (id, name, adapter_type, config, priority, supported_regions,
                                   cost_per_lookup, status, success_rate, active)
        VALUES
            (uuid_generate_v4(), 'HLR-Primary',   'http_rest',
             '{"api_key": "hlr-test-key", "base_url": "https://hlr-test.local"}',
             1, ARRAY['RU', 'KZ', 'BY', 'UA'], 0.001, 'healthy', 99.9, true),
            (uuid_generate_v4(), 'HLR-Secondary',  'http_rest',
             '{"api_key": "hlr-test-key-2", "base_url": "https://hlr-test-2.local"}',
             2, ARRAY['RU', 'KZ'], 0.002, 'healthy', 98.5, true)
        ON CONFLICT DO NOTHING;
    END IF;
END $$;

-- ============================================================
-- 7. Шаблоны для тестирования batch с template_id
-- ============================================================
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'templates') THEN
        INSERT INTO templates (id, client_id, name, body, variables, status)
        VALUES
            (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000001',
             'otp-code', 'Ваш код подтверждения: {{code}}. Действует 5 минут.',
             ARRAY['code'], 'approved'),
            (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000001',
             'delivery-notify', 'Ваш заказ №{{order_id}} доставлен. Курьер: {{courier}}.',
             ARRAY['order_id', 'courier'], 'approved'),
            (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000001',
             'promo-offer', 'Скидка {{discount}}% на {{product}}! Код: {{promo_code}}.',
             ARRAY['discount', 'product', 'promo_code'], 'approved')
        ON CONFLICT DO NOTHING;
    END IF;
END $$;

-- ============================================================
-- 10. Sub-account QA test users (blocking portal sub-account QA)
-- ============================================================
-- Пароль: Test1234! (bcrypt cost=10, prefix $2a$).
-- См. docs/reports/test-credentials.md для полного списка.
-- ON CONFLICT (email) DO UPDATE гарантирует идемпотентность при редеплое:
-- если кто-то вручную сбросит хэш, seed восстановит известный пароль.
INSERT INTO users (id, username, email, password_hash, role_id, active, client_id)
VALUES
    ('39ed66cb-cc45-4b3a-b99e-3ce5f9e3c6e5', 'subacc1',   'subacc@test.local',
     '$2a$10$5Eb9tCKwrrqXl5nWs.5l3O.Fl4ej19WeJAZ9.0sJ2b6frKQd2YQay',  -- Test1234!
     '00000000-0000-0000-0000-000000000002', true, 'a0000000-0000-0000-0000-000000000002'),
    ('6259d929-0a89-413d-ad3a-c9477be37fdf', 'clean-a',   'clean-a@test.local',
     '$2a$10$5Eb9tCKwrrqXl5nWs.5l3O.Fl4ej19WeJAZ9.0sJ2b6frKQd2YQay',  -- Test1234!
     '00000000-0000-0000-0000-000000000002', true, 'f6c4b3fd-f0d3-4d73-a59a-b6c43a312f8d'),
    ('be8e634a-dae6-47cc-b307-a888b8de7f41', 'problem-b', 'problem-b@test.local',
     '$2a$10$5Eb9tCKwrrqXl5nWs.5l3O.Fl4ej19WeJAZ9.0sJ2b6frKQd2YQay',  -- Test1234!
     '00000000-0000-0000-0000-000000000002', true, '6063f9b6-5aab-4474-b13c-24e653cb28f0'),
    ('2b6dbb32-2ce9-432a-9e0d-8a7e931716cc', 'heavy-c',   'heavy-c@test.local',
     '$2a$10$5Eb9tCKwrrqXl5nWs.5l3O.Fl4ej19WeJAZ9.0sJ2b6frKQd2YQay',  -- Test1234!
     '00000000-0000-0000-0000-000000000002', true, 'de3712f8-1bfe-4fe5-9be4-81657a3aa746')
ON CONFLICT (email) DO UPDATE SET
    password_hash = EXCLUDED.password_hash,
    active        = EXCLUDED.active,
    client_id     = EXCLUDED.client_id;

COMMIT;

-- Вывод статистики сидинга
SELECT 'providers'  AS entity, count(*) FROM providers WHERE name LIKE 'Provider-%'
UNION ALL
SELECT 'routes',     count(*) FROM routes WHERE name LIKE '%Russia%' OR name LIKE '%International%' OR name LIKE '%Fallback%'
UNION ALL
SELECT 'clients',    count(*) FROM clients WHERE name LIKE 'LoadTest-%'
UNION ALL
SELECT 'users',      count(*) FROM users WHERE username LIKE 'loadtest-%';
