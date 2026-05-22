-- =============================================================================
-- Локальный demo-seed для разработки.
-- Учитывает обязательную привязку accounts.company_id (миграция 000091).
-- Идемпотентен.
-- =============================================================================

BEGIN;

-- На время сидинга отключаем триггер автосоздания accounts (он устарел —
-- не передаёт company_id, см. миграцию 000091).
ALTER TABLE clients DISABLE TRIGGER trg_create_account_for_new_client;

-- ---------- 1. Провайдеры ----------
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

-- ---------- 2. Маршруты ----------
INSERT INTO routes (id, name, pattern, pattern_type, provider_id, priority, active, failover_provider_id)
VALUES
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
    ('b0000000-0000-0000-0000-000000000011', 'International-CIS', '375',  'prefix', 'a0000000-0000-0000-0000-000000000006', 10, true, NULL),
    ('b0000000-0000-0000-0000-000000000012', 'International-KZ',  '77',   'prefix', 'a0000000-0000-0000-0000-000000000006', 10, true, NULL),
    ('b0000000-0000-0000-0000-000000000013', 'Default-Fallback',  '7',    'prefix', 'a0000000-0000-0000-0000-000000000007', 100, true, NULL)
ON CONFLICT DO NOTHING;

-- ---------- 3. Клиенты (без триггера accounts) ----------
INSERT INTO clients (id, name, api_key, secret, active,
                     rate_limit_per_second, rate_limit_per_minute, rate_limit_per_hour,
                     allowed_source_addresses)
VALUES
    ('c0000000-0000-0000-0000-000000000001', 'Demo-Main',     'demo-main-key-001',   'demo-secret-001', true,
     1000, 60000, 3600000, ARRAY['Demo','OTP','INFO']),
    ('c0000000-0000-0000-0000-000000000002', 'Demo-Reseller', 'demo-reseller-key-002','demo-secret-002', true,
     500,  30000, 1800000, ARRAY['Reseller']),  -- is_reseller=true ставим ниже UPDATE'ом
    ('c0000000-0000-0000-0000-000000000003', 'Demo-Light',    'demo-light-key-003',  'demo-secret-003', true,
     100,  6000,  360000,  ARRAY['Light']),
    ('c0000000-0000-0000-0000-000000000004', 'Demo-Default',  'test-api-key',         'test-secret',    true,
     10000,600000,36000000,ARRAY['12345','67890','SMS','API','TEST'])
ON CONFLICT (api_key) DO UPDATE SET
    rate_limit_per_second = EXCLUDED.rate_limit_per_second,
    active                = true;

-- Demo-Reseller: включаем reseller-режим (panel /network)
UPDATE clients
SET is_reseller = true, max_sub_accounts = 50
WHERE id = 'c0000000-0000-0000-0000-000000000002';

-- ---------- 4. Companies + client_companies ----------
INSERT INTO companies (id, name, full_name, inn, is_offer, active)
VALUES
    ('cc000000-0000-0000-0000-000000000001', 'ООО Демо-Главный',     'Общество с ограниченной ответственностью «Демо-Главный»',     '7700000001', false, true),
    ('cc000000-0000-0000-0000-000000000002', 'ООО Демо-Реселлер',    'Общество с ограниченной ответственностью «Демо-Реселлер»',    '7700000002', false, true),
    ('cc000000-0000-0000-0000-000000000003', 'ИП Демо-Лайт',          'Индивидуальный предприниматель Демо-Лайт',                    '770000000003', false, true),
    ('cc000000-0000-0000-0000-000000000004', 'Demo-Default-Offer',    'Договор-оферта (Demo-Default)',                               NULL,         true,  true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO client_companies (client_id, company_id, is_default)
VALUES
    ('c0000000-0000-0000-0000-000000000001','cc000000-0000-0000-0000-000000000001', true),
    ('c0000000-0000-0000-0000-000000000002','cc000000-0000-0000-0000-000000000002', true),
    ('c0000000-0000-0000-0000-000000000003','cc000000-0000-0000-0000-000000000003', true),
    ('c0000000-0000-0000-0000-000000000004','cc000000-0000-0000-0000-000000000004', true)
ON CONFLICT (client_id, company_id) DO NOTHING;

-- ---------- 5. Accounts с балансами ----------
INSERT INTO accounts (id, client_id, company_id, balance, currency, credit_limit, low_balance_threshold)
VALUES
    (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000001','cc000000-0000-0000-0000-000000000001', 100000.00, 'RUB', 0, 100),
    (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000002','cc000000-0000-0000-0000-000000000002',  50000.00, 'RUB', 0, 100),
    (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000003','cc000000-0000-0000-0000-000000000003',  10000.00, 'RUB', 0, 100),
    (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000004','cc000000-0000-0000-0000-000000000004', 999999.00, 'RUB', 0, 100)
ON CONFLICT (client_id) DO UPDATE SET
    balance    = EXCLUDED.balance,
    company_id = EXCLUDED.company_id;

-- ---------- 6. Пользователи ----------
-- admin/example local password и demo-client/example local password
INSERT INTO users (id, username, email, password_hash, role_id, active, client_id)
VALUES
    ('d0000000-0000-0000-0000-000000000001', 'admin',       'admin@demo.local',
     '$2a$10$Od96EF1oN2Cp7JPpMNyLCuHs.mN3dRB82Ukq8TRhEDgKokaDj2jYO',
     '00000000-0000-0000-0000-000000000001', true, NULL),
    ('d0000000-0000-0000-0000-000000000002', 'demo-client', 'client@demo.local',
     '$2a$10$Od96EF1oN2Cp7JPpMNyLCuHs.mN3dRB82Ukq8TRhEDgKokaDj2jYO',
     '00000000-0000-0000-0000-000000000002', true, 'c0000000-0000-0000-0000-000000000001'),
    ('d0000000-0000-0000-0000-000000000003', 'demo-reseller','reseller@demo.local',
     '$2a$10$Od96EF1oN2Cp7JPpMNyLCuHs.mN3dRB82Ukq8TRhEDgKokaDj2jYO',
     '00000000-0000-0000-0000-000000000002', true, 'c0000000-0000-0000-0000-000000000002')
ON CONFLICT (username) DO UPDATE SET
    client_id     = EXCLUDED.client_id,
    password_hash = EXCLUDED.password_hash,
    active        = true;

-- ---------- 7. API-ключи ----------
INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, active)
VALUES
    ('e0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000002',
     'demo-main',    'hash_demo_main_key_001', 'demo-ma', true),
    ('e0000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000002',
     'demo-default', 'hash_test_api_key',      'test-ap', true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO api_key_scopes (api_key_id, scope) VALUES
    ('e0000000-0000-0000-0000-000000000001','messages:send'),
    ('e0000000-0000-0000-0000-000000000001','messages:read'),
    ('e0000000-0000-0000-0000-000000000001','messages:delete'),
    ('e0000000-0000-0000-0000-000000000002','messages:send'),
    ('e0000000-0000-0000-0000-000000000002','messages:read')
ON CONFLICT DO NOTHING;

-- ---------- 8. HLR-провайдеры ----------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'hlr_providers') THEN
        INSERT INTO hlr_providers (id, name, adapter_type, config, priority, supported_regions,
                                   cost_per_lookup, status, success_rate, active)
        VALUES
            ('11110000-0000-0000-0000-000000000001', 'HLR-Primary',   'http_rest',
             '{"api_key":"hlr-test-key","base_url":"https://hlr-test.local"}',
             1, ARRAY['RU','KZ','BY','UA'], 0.001, 'healthy', 99.9, true),
            ('11110000-0000-0000-0000-000000000002', 'HLR-Secondary', 'http_rest',
             '{"api_key":"hlr-test-key-2","base_url":"https://hlr-test-2.local"}',
             2, ARRAY['RU','KZ'], 0.002, 'healthy', 98.5, true)
        ON CONFLICT (id) DO NOTHING;
    END IF;
END $$;

-- ---------- 9. Шаблоны ----------
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
             ARRAY['order_id','courier'], 'approved'),
            (uuid_generate_v4(), 'c0000000-0000-0000-0000-000000000001',
             'promo-offer', 'Скидка {{discount}}% на {{product}}! Код: {{promo_code}}.',
             ARRAY['discount','product','promo_code'], 'approved')
        ON CONFLICT DO NOTHING;
    END IF;
END $$;

-- Включаем триггер обратно
ALTER TABLE clients ENABLE TRIGGER trg_create_account_for_new_client;

COMMIT;

-- Статистика
SELECT 'providers'  AS entity, count(*) FROM providers WHERE name LIKE 'Provider-%'
UNION ALL SELECT 'routes',     count(*) FROM routes WHERE name LIKE '%Russia%' OR name LIKE '%International%' OR name LIKE '%Fallback%'
UNION ALL SELECT 'clients',    count(*) FROM clients WHERE name LIKE 'Demo-%'
UNION ALL SELECT 'companies',  count(*) FROM companies WHERE id::text LIKE 'cc000000%'
UNION ALL SELECT 'accounts',   count(*) FROM accounts WHERE client_id::text LIKE 'c0000000%'
UNION ALL SELECT 'users',      count(*) FROM users WHERE username IN ('admin','demo-client','demo-reseller')
UNION ALL SELECT 'api_keys',   count(*) FROM api_keys WHERE id::text LIKE 'e0000000%'
UNION ALL SELECT 'countries',  count(*) FROM countries
UNION ALL SELECT 'operators',  count(*) FROM operators;
