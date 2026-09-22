-- =============================================================================
-- Демо-данные стенда (применять ПОВЕРХ test/load/fixtures/demo_seed.sql).
--
-- Добавляет: имена отправителя, контакты и списки, кампании, историю
-- сообщений за 30 дней (~75k) с DLR-квитками, тарифы и tarification_log,
-- price rules и транзакции по счетам.
--
-- Все создаваемые ID детерминированы и имеют маркер: <XX><6 цифр>-00da-0000-0000-<12 цифр>.
-- Удаление — test/load/fixtures/demo_cleanup.sql (по маркеру '-00da-0000-0000-').
-- Повторный запуск: сначала demo_cleanup.sql (таймстемпы относительны now(),
-- поэтому идемпотентность через ON CONFLICT здесь невозможна).
--
-- Требует выполненных миграций (включая 000091, 000104, 000123) и demo_seed.sql.
-- Объём: см. константы в секции 5 (DEMO_MSG_COUNT) — 75000 сообщений за 30 дней.
-- =============================================================================

BEGIN;

-- ---------- 0. Партиции-страховка (текущий месяц ±1) ----------
DO $$
DECLARE
    m timestamptz;
BEGIN
    FOR m IN SELECT generate_series(date_trunc('month', now()) - interval '1 month',
                                    date_trunc('month', now()), interval '1 month')
    LOOP
        EXECUTE format('CREATE TABLE IF NOT EXISTS messages_y%sm%s PARTITION OF messages
                        FOR VALUES FROM (%L) TO (%L)',
                       to_char(m, 'YYYY'), to_char(m, 'MM'), m, m + interval '1 month');
        EXECUTE format('CREATE TABLE IF NOT EXISTS tarification_log_%s_%s PARTITION OF tarification_log
                        FOR VALUES FROM (%L) TO (%L)',
                       to_char(m, 'YYYY'), to_char(m, 'MM'), m, m + interval '1 month');
    END LOOP;
END $$;

-- ---------- 1. Имена отправителя ----------
-- company_id обязателен (миграция 000091): Main -> cc..01, Light -> cc..03.
INSERT INTO sender_names (id, client_id, company_id, name, status, rejection_reason, reviewer_id, reviewed_at)
VALUES
    ('f2000001-00da-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001', 'cc000000-0000-0000-0000-000000000001', 'Demo',  'approved', NULL, (SELECT id FROM users WHERE username = 'admin'), now() - interval '35 days'),
    ('f2000002-00da-0000-0000-000000000002', 'c0000000-0000-0000-0000-000000000001', 'cc000000-0000-0000-0000-000000000001', 'OTP',   'approved', NULL, (SELECT id FROM users WHERE username = 'admin'), now() - interval '35 days'),
    ('f2000003-00da-0000-0000-000000000003', 'c0000000-0000-0000-0000-000000000001', 'cc000000-0000-0000-0000-000000000001', 'PROMO', 'pending',   NULL, NULL, NULL),
    ('f2000004-00da-0000-0000-000000000004', 'c0000000-0000-0000-0000-000000000001', 'cc000000-0000-0000-0000-000000000001', 'NEWS',  'rejected', 'Имя не согласовано с операторами (демо)', (SELECT id FROM users WHERE username = 'admin'), now() - interval '10 days'),
    ('f2000005-00da-0000-0000-000000000005', 'c0000000-0000-0000-0000-000000000003', 'cc000000-0000-0000-0000-000000000003', 'Light', 'approved', NULL, (SELECT id FROM users WHERE username = 'admin'), now() - interval '20 days')
ON CONFLICT (client_id, name) DO NOTHING;

-- Регистрации имён у операторов (для approved имён Demo и OTP)
INSERT INTO operator_registrations (id, sender_name_id, operator_id, registration_type, status, approved_type, approved_at, moderator_note, submitted_at, resolved_at)
SELECT ('f3' || lpad(row_number() OVER (ORDER BY s.id, o.code)::text, 6, '0')
        || '-00da-0000-0000-' || lpad(row_number() OVER (ORDER BY s.id, o.code)::text, 12, '0'))::uuid,
       s.id, o.id,
       CASE WHEN s.name = 'OTP' THEN 'paid' ELSE 'free' END,
       'approved',
       CASE WHEN s.name = 'OTP' THEN 'paid' ELSE 'free' END,
       now() - interval '33 days',
       CASE WHEN s.name = 'OTP' THEN 'Платная регистрация подтверждена' ELSE 'Бесплатная регистрация' END,
       now() - interval '40 days',
       now() - interval '33 days'
FROM sender_names s
JOIN operators o ON o.code IN ('MTS_RU', 'BEELINE_RU', 'MEGAFON_RU', 'TELE2_RU')
WHERE s.id IN ('f2000001-00da-0000-0000-000000000001', 'f2000002-00da-0000-0000-000000000002')
ON CONFLICT (sender_name_id, operator_id) DO NOTHING;

INSERT INTO default_sender_names (client_id, channel, sender_name_id) VALUES
    ('c0000000-0000-0000-0000-000000000001', 'sms', 'f2000001-00da-0000-0000-000000000001'),
    ('c0000000-0000-0000-0000-000000000003', 'sms', 'f2000005-00da-0000-0000-000000000005')
ON CONFLICT (client_id, channel) DO NOTHING;

-- ---------- 2. Списки контактов ----------
INSERT INTO contact_lists (id, client_id, name, description) VALUES
    ('fd000001-00da-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001', 'OTP-база',     'База для транзакционных OTP-рассылок (демо)'),
    ('fd000002-00da-0000-0000-000000000002', 'c0000000-0000-0000-0000-000000000001', 'Промо-осень',  'Подписчики промо-рассылок, осень 2026 (демо)');

INSERT INTO contact_list_attributes (id, contact_list_id, name, display_name, type, required, position) VALUES
    ('fe000001-00da-0000-0000-000000000001', 'fd000001-00da-0000-0000-000000000001', 'first_name', 'Имя',         'string', true,  0),
    ('fe000002-00da-0000-0000-000000000002', 'fd000001-00da-0000-0000-000000000001', 'last_name',  'Фамилия',     'string', true,  1),
    ('fe000003-00da-0000-0000-000000000003', 'fd000001-00da-0000-0000-000000000001', 'city',       'Город',       'string', false, 2),
    ('fe000004-00da-0000-0000-000000000004', 'fd000002-00da-0000-0000-000000000002', 'first_name', 'Имя',         'string', true,  0),
    ('fe000005-00da-0000-0000-000000000005', 'fd000002-00da-0000-0000-000000000002', 'last_name',  'Фамилия',     'string', true,  1),
    ('fe000006-00da-0000-0000-000000000006', 'fd000002-00da-0000-0000-000000000002', 'birthday',   'День рождения', 'date', false, 2)
ON CONFLICT (contact_list_id, name) DO NOTHING;

-- Контакты: 1000 в OTP-базе (n = 1..1000), 500 в Промо-осени (n = 1001..1500).
-- Телефоны детерминированы (n * 7919 mod 10^7 — уникальны в пределах списка).
INSERT INTO contacts (id, contact_list_id, phone, attributes, tags)
SELECT
    ('fc' || lpad(n::text, 6, '0') || '-00da-0000-0000-' || lpad(n::text, 12, '0'))::uuid,
    CASE WHEN n <= 1000 THEN 'fd000001-00da-0000-0000-000000000001'::uuid ELSE 'fd000002-00da-0000-0000-000000000002'::uuid END,
    '7' || (ARRAY['910','916','985','903','906','920','925','900','901'])[1 + n % 9]
        || lpad(((n * 7919) % 10000000)::text, 7, '0'),
    jsonb_build_object(
        'first_name', (ARRAY['Александр','Мария','Иван','Анна','Дмитрий','Елена','Сергей','Ольга',
                             'Алексей','Татьяна','Андрей','Наталья','Михаил','Екатерина','Николай',
                             'Ирина','Владимир','Светлана','Павел','Юлия'])[1 + n % 20],
        'last_name',  (ARRAY['Иванов','Петров','Смирнов','Кузнецов','Попов','Васильев','Соколов',
                             'Михайлов','Новиков','Фёдоров','Морозов','Волков','Алексеев','Лебедев',
                             'Семёнов','Егоров','Павлов','Козлов','Степанов','Николаев'])[1 + n % 20],
        'city',       (ARRAY['Москва','Санкт-Петербург','Новосибирск','Екатеринбург','Казань',
                             'Нижний Новгород','Челябинск','Самара','Омск','Ростов-на-Дону'])[1 + n % 10])
    || CASE WHEN n > 1000 AND n % 3 = 0
            THEN jsonb_build_object('birthday', (1965 + n % 35) || '-' || lpad((1 + n % 12)::text, 2, '0') || '-' || lpad((1 + n % 28)::text, 2, '0'))
            ELSE '{}'::jsonb END,
    CASE WHEN n <= 1000
         THEN CASE WHEN n % 5 < 3 THEN ARRAY['otp','verified'] ELSE ARRAY['otp'] END
         ELSE CASE WHEN n % 5 < 2 THEN ARRAY['promo','newsletter'] ELSE ARRAY['promo'] END END
FROM generate_series(1, 1500) AS n
ON CONFLICT (contact_list_id, phone) DO NOTHING;

INSERT INTO contact_imports (id, contact_list_id, client_id, file_name, file_size, status,
                             total_rows, imported_count, updated_count, error_count, errors, column_mapping,
                             created_at, completed_at)
VALUES
    ('f1000001-00da-0000-0000-000000000001', 'fd000001-00da-0000-0000-000000000001',
     'c0000000-0000-0000-0000-000000000001', 'otp_base_2026-09.csv', 48211, 'completed',
     1000, 1000, 0, 0, NULL,
     '{"phone":0,"first_name":1,"last_name":2,"city":3}'::jsonb,
     now() - interval '25 days', now() - interval '25 days' + interval '2 minutes'),
    ('f1000002-00da-0000-0000-000000000002', 'fd000002-00da-0000-0000-000000000002',
     'c0000000-0000-0000-0000-000000000001', 'promo_autumn.csv', 28405, 'completed',
     500, 498, 0, 2,
     '[{"row":13,"error":"invalid phone"},{"row":207,"error":"duplicate"}]'::jsonb,
     '{"phone":0,"first_name":1,"last_name":2,"birthday":3}'::jsonb,
     now() - interval '12 days', now() - interval '12 days' + interval '90 seconds');

UPDATE contact_lists cl
SET contacts_count = (SELECT count(*) FROM contacts c WHERE c.contact_list_id = cl.id)
WHERE cl.id IN ('fd000001-00da-0000-0000-000000000001', 'fd000002-00da-0000-0000-000000000002');

-- ---------- 3. Кампании ----------
INSERT INTO campaigns (id, client_id, name, status, contact_list_id, template_id, source, send_rate,
                       scheduled_at, started_at, completed_at, created_at)
VALUES
    ('ff000001-00da-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001',
     'Осенняя распродажа −20%', 'completed',
     'fd000002-00da-0000-0000-000000000002',
     (SELECT id FROM templates WHERE client_id = 'c0000000-0000-0000-0000-000000000001' AND name = 'promo-offer'),
     'Demo', 200,
     now() - interval '7 days', now() - interval '7 days' + interval '2 minutes', now() - interval '6 days',
     now() - interval '9 days'),
    ('ff000002-00da-0000-0000-000000000002', 'c0000000-0000-0000-0000-000000000001',
     'Верификация OTP — дайджест', 'paused',
     'fd000001-00da-0000-0000-000000000001',
     (SELECT id FROM templates WHERE client_id = 'c0000000-0000-0000-0000-000000000001' AND name = 'otp-code'),
     'OTP', 100,
     now() - interval '4 hours', now() - interval '3 hours', NULL,
     now() - interval '5 hours'),
    ('ff000003-00da-0000-0000-000000000003', 'c0000000-0000-0000-0000-000000000001',
     'Чёрная пятница — анонс', 'scheduled',
     'fd000002-00da-0000-0000-000000000002', NULL, 'Demo', 500,
     now() + interval '2 days', NULL, NULL, now() - interval '2 days'),
    ('ff000004-00da-0000-0000-000000000004', 'c0000000-0000-0000-0000-000000000001',
     'Тестовый черновик', 'draft',
     'fd000001-00da-0000-0000-000000000001', NULL, '', 0,
     NULL, NULL, NULL, now() - interval '1 day'),
    ('ff000005-00da-0000-0000-000000000005', 'c0000000-0000-0000-0000-000000000001',
     'Прошлая акция (отменена)', 'cancelled',
     'fd000002-00da-0000-0000-000000000002', NULL, 'Demo', 300,
     now() - interval '15 days', NULL, NULL, now() - interval '18 days');

-- Получатели завершённой кампании 1 (все 500 контактов «Промо-осени»)
INSERT INTO campaign_recipients (id, campaign_id, contact_id, phone, status, created_at, updated_at)
SELECT ('f0' || lpad(k::text, 6, '0') || '-00da-0000-0000-' || lpad(k::text, 12, '0'))::uuid,
       'ff000001-00da-0000-0000-000000000001', c.id, c.phone,
       CASE WHEN k % 50 < 46 THEN 'delivered' WHEN k % 50 < 49 THEN 'failed' ELSE 'cancelled' END,
       now() - interval '7 days' + make_interval(secs => k * 11),
       now() - interval '6 days' + make_interval(secs => k * 7)
FROM (SELECT c.*, row_number() OVER (ORDER BY c.id) AS k
      FROM contacts c
      WHERE c.contact_list_id = 'fd000002-00da-0000-0000-000000000002') c;

-- Получатели приостановленной кампании 2 (первые 200 контактов «OTP-базы»).
-- Статус именно paused, не running: воркер кампаний подхватывает running-кампании
-- и начинает реальную рассылку, а переменные шаблона заполнить нечем.
INSERT INTO campaign_recipients (id, campaign_id, contact_id, phone, status, created_at, updated_at)
SELECT ('f0' || lpad((500 + k)::text, 6, '0') || '-00da-0000-0000-' || lpad((500 + k)::text, 12, '0'))::uuid,
       'ff000002-00da-0000-0000-000000000002', c.id, c.phone,
       CASE WHEN k % 10 < 7 THEN 'delivered' WHEN k % 10 = 7 THEN 'sent'
            WHEN k % 10 = 8 THEN 'failed' ELSE 'pending' END,
       now() - interval '3 hours' + make_interval(secs => k * 9),
       now() - make_interval(mins => (k % 30)::int)
FROM (SELECT c.*, row_number() OVER (ORDER BY c.id) AS k
      FROM contacts c
      WHERE c.contact_list_id = 'fd000001-00da-0000-0000-000000000001') c
WHERE k <= 200;

-- Часовые снапшоты статистики кампании 1 за время её выполнения (~24 часа)
INSERT INTO campaign_stats_snapshots (campaign_id, variant_id, snapshot_at, sent, delivered, failed, pending, avg_delivery_time_ms, cost)
SELECT 'ff000001-00da-0000-0000-000000000001', '00000000-0000-0000-0000-000000000000', ts,
       sent, (sent * 92 / 100), (sent * 6 / 100), sent - (sent * 92 / 100) - (sent * 6 / 100),
       2400 + (h % 5) * 150, round((sent * 92 / 100) * 0.6, 4)
FROM generate_series(now() - interval '7 days', now() - interval '6 days', interval '1 hour')
     WITH ORDINALITY AS g(ts, h)
CROSS JOIN LATERAL (SELECT LEAST(500, h * 40)::int AS sent) s;

-- Синхронизация счётчиков кампаний с фактическими получателями
UPDATE campaigns cp
SET total_recipients = r.total,
    sent_count       = r.sent,
    delivered_count  = r.delivered,
    failed_count     = r.failed
FROM (SELECT campaign_id, count(*) AS total,
             count(*) FILTER (WHERE status IN ('sent', 'delivered', 'failed')) AS sent,
             count(*) FILTER (WHERE status = 'delivered') AS delivered,
             count(*) FILTER (WHERE status = 'failed') AS failed
      FROM campaign_recipients
      WHERE campaign_id::text LIKE 'ff0000%'
      GROUP BY campaign_id) r
WHERE cp.id = r.campaign_id;

-- ---------- 4. История сообщений: 75000 строк за последние 30 дней ----------
-- Распределение статусов (n % 100): delivered 85%, failed 7%, sent 2%,
-- queued/pending/expired/rejected/scheduled/cancelled по 1%.
-- Клиенты (n % 20): Demo-Main 60%, Demo-Light 25%, Demo-Reseller 10%, Demo-Default 5%.
-- Операторы (n % 10): MTS 40%, Megafon 20%, Beeline 20%, Tele2 20%.
WITH ru AS (
    SELECT id AS country_id FROM countries WHERE iso_code = 'RU'
),
ops AS (
    SELECT id, code FROM operators WHERE code IN ('MTS_RU', 'MEGAFON_RU', 'BEELINE_RU', 'TELE2_RU')
),
tpl AS (
    SELECT id, name FROM templates
    WHERE client_id = 'c0000000-0000-0000-0000-000000000001'
      AND name IN ('otp-code', 'delivery-notify', 'promo-offer')
),
base AS (
    SELECT n,
           CASE WHEN n % 20 < 12 THEN 'c0000000-0000-0000-0000-000000000001'
                WHEN n % 20 < 17 THEN 'c0000000-0000-0000-0000-000000000003'
                WHEN n % 20 < 19 THEN 'c0000000-0000-0000-0000-000000000002'
                ELSE 'c0000000-0000-0000-0000-000000000004' END::uuid AS client_id,
           CASE WHEN n % 100 <= 84 THEN 'delivered'
                WHEN n % 100 <= 91 THEN 'failed'
                WHEN n % 100 <= 93 THEN 'sent'
                WHEN n % 100 = 94  THEN 'queued'
                WHEN n % 100 = 95  THEN 'pending'
                WHEN n % 100 = 96  THEN 'expired'
                WHEN n % 100 = 97  THEN 'rejected'
                WHEN n % 100 = 98  THEN 'scheduled'
                ELSE 'cancelled' END AS status,
           LEAST(date_trunc('day', now()) - make_interval(days => n % 30)
                     + make_interval(hours => (n * 7) % 24, mins => n % 60, secs => (n * 13) % 60),
                 now() - interval '5 minutes') AS created_at
    FROM generate_series(1, 75000) AS n
)
INSERT INTO messages (id, message_id, source, destination, text, encoding, data_coding,
                      status, status_message, provider_id, route_id, client_id,
                      retry_count, smpp_message_id, submitted_at, delivered_at, failed_at,
                      created_at, updated_at, scheduled_at, segment_count, expired_at,
                      channel, operator_id, country_id, send_method,
                      template_id, sender_name_id,
                      source_addr_ton, source_addr_npi, dest_addr_ton, dest_addr_npi,
                      registered_delivery, priority_flag)
SELECT
    ('fa' || lpad(b.n::text, 6, '0') || '-00da-0000-0000-' || lpad(b.n::text, 12, '0'))::uuid,
    'demo-' || b.n,
    CASE b.client_id
        WHEN 'c0000000-0000-0000-0000-000000000001'::uuid THEN (ARRAY['Demo','OTP','INFO'])[1 + b.n % 3]
        WHEN 'c0000000-0000-0000-0000-000000000003'::uuid THEN 'Light'
        WHEN 'c0000000-0000-0000-0000-000000000002'::uuid THEN 'Reseller'
        ELSE (ARRAY['12345','SMS','TEST'])[1 + b.n % 3] END,
    '7' || CASE ops.code
        WHEN 'MTS_RU'      THEN (ARRAY['910','916','985'])[1 + b.n % 3]
        WHEN 'MEGAFON_RU'  THEN (ARRAY['920','925'])[1 + b.n % 2]
        WHEN 'BEELINE_RU'  THEN (ARRAY['903','906'])[1 + b.n % 2]
        ELSE (ARRAY['900','901'])[1 + b.n % 2] END
        || lpad(((b.n * 7919) % 10000000)::text, 7, '0'),
    CASE b.n % 3
        WHEN 0 THEN 'Ваш код подтверждения: ' || lpad(((b.n * 7591) % 1000000)::text, 6, '0') || '. Действует 5 минут. Никому не сообщайте его.'
        WHEN 1 THEN 'Заказ №' || (10000 + b.n % 89999) || ' собран и передан курьеру. Отследить доставку: demo-shop.ru/track/' || b.n
        ELSE 'Только до 30 сентября! Скидка 20% на весь каталог по промокоду SALE' || lpad((b.n % 900 + 100)::text, 3, '0') || '. Доставка бесплатная. Успейте оформить заказ!' END,
    'UCS2', 8,
    b.status,
    CASE b.status
        WHEN 'failed'   THEN (ARRAY['Абонент недоступен','Недостаточно средств у абонента','Абонент занят'])[1 + b.n % 3]
        WHEN 'rejected' THEN 'Отклонено оператором: некорректный номер'
        WHEN 'expired'  THEN 'Истёк срок доставки'
        ELSE NULL END,
    CASE ops.code
        WHEN 'MTS_RU'     THEN 'a0000000-0000-0000-0000-000000000001'::uuid
        WHEN 'MEGAFON_RU' THEN 'a0000000-0000-0000-0000-000000000003'::uuid
        WHEN 'BEELINE_RU' THEN 'a0000000-0000-0000-0000-000000000002'::uuid
        ELSE 'a0000000-0000-0000-0000-000000000004'::uuid END,
    CASE ops.code
        WHEN 'MTS_RU'     THEN ('b0000000-0000-0000-0000-0000000000' || lpad(((b.n % 3) + 1)::text, 2, '0'))::uuid
        WHEN 'MEGAFON_RU' THEN ('b0000000-0000-0000-0000-0000000000' || lpad((6 + b.n % 2)::text, 2, '0'))::uuid
        WHEN 'BEELINE_RU' THEN ('b0000000-0000-0000-0000-0000000000' || lpad((4 + b.n % 2)::text, 2, '0'))::uuid
        ELSE ('b0000000-0000-0000-0000-0000000000' || lpad((8 + b.n % 2)::text, 2, '0'))::uuid END,
    b.client_id,
    CASE WHEN b.status = 'failed' THEN 1 + b.n % 2 ELSE 0 END,
    CASE WHEN b.status IN ('sent', 'delivered', 'failed', 'expired') THEN 'sm-demo-' || b.n END,
    CASE WHEN b.status IN ('sent', 'delivered', 'failed', 'expired')
         THEN b.created_at + make_interval(secs => 1 + b.n % 3) END,
    CASE WHEN b.status = 'delivered'
         THEN b.created_at + make_interval(secs => 3 + (b.n * 17) % 40) END,
    CASE WHEN b.status = 'failed'
         THEN b.created_at + make_interval(secs => 5 + (b.n * 11) % 60) END,
    b.created_at,
    CASE WHEN b.status = 'delivered' THEN b.created_at + make_interval(secs => 3 + (b.n * 17) % 40)
         WHEN b.status = 'failed'    THEN b.created_at + make_interval(secs => 5 + (b.n * 11) % 60)
         ELSE b.created_at END,
    CASE WHEN b.status = 'scheduled' THEN now() + make_interval(hours => 1 + b.n % 24) END,
    CASE b.n % 3 WHEN 0 THEN 1 WHEN 1 THEN 1 + b.n % 2 ELSE 2 + b.n % 2 END,
    CASE WHEN b.status = 'expired' THEN b.created_at + interval '3 days' END,
    'sms',
    ops.id,
    ru.country_id,
    (ARRAY['PORTAL','API','SMPP'])[1 + b.n % 3],
    CASE WHEN b.n % 20 < 12 THEN tpl.id END,
    CASE WHEN b.client_id = 'c0000000-0000-0000-0000-000000000001'::uuid AND b.n % 4 = 0 THEN 'f2000002-00da-0000-0000-000000000002'::uuid
         WHEN b.client_id = 'c0000000-0000-0000-0000-000000000001'::uuid AND b.n % 4 = 1 THEN 'f2000001-00da-0000-0000-000000000001'::uuid
         WHEN b.client_id = 'c0000000-0000-0000-0000-000000000003'::uuid AND b.n % 4 = 0 THEN 'f2000005-00da-0000-0000-000000000005'::uuid END,
    5, 0, 1, 1, 1, 0
FROM base b
CROSS JOIN ru
JOIN ops ON ops.code = CASE WHEN b.n % 10 <= 3 THEN 'MTS_RU'
                            WHEN b.n % 10 <= 5 THEN 'MEGAFON_RU'
                            WHEN b.n % 10 <= 7 THEN 'BEELINE_RU'
                            ELSE 'TELE2_RU' END
LEFT JOIN tpl ON b.n % 20 < 12
              AND tpl.name = CASE b.n % 3 WHEN 0 THEN 'otp-code' WHEN 1 THEN 'delivery-notify' ELSE 'promo-offer' END;

-- ---------- 5. DLR-квитки (для delivered и failed) ----------
INSERT INTO dlr_receipts (id, message_id, message_created_at, smpp_message_id, provider_id,
                          receipted_message_id, submit_date, done_date, stat, err, source, destination, created_at)
SELECT ('fb' || lpad(substring(m.message_id from 6)::int::text, 6, '0')
        || '-00da-0000-0000-' || lpad(substring(m.message_id from 6)::int::text, 12, '0'))::uuid,
       m.id, m.created_at, m.smpp_message_id, m.provider_id,
       m.message_id,
       m.created_at,
       COALESCE(m.delivered_at, m.failed_at),
       CASE m.status WHEN 'delivered' THEN 'DELIVRD' ELSE 'UNDELIV' END,
       CASE m.status WHEN 'delivered' THEN 0 ELSE (ARRAY[1, 41, 47])[1 + substring(m.message_id from 6)::int % 3] END,
       m.source, m.destination,
       COALESCE(m.delivered_at, m.failed_at)
FROM messages m
WHERE m.id::text LIKE 'fa______-00da-0000-0000-%'
  AND m.status IN ('delivered', 'failed');

-- ---------- 6. Тарифы операторов (легаси-схема, нужна tarification_log) ----------
INSERT INTO tariff_plans (id, operator_id, sender_category, strategy, active)
SELECT ('f5' || lpad(row_number() OVER (ORDER BY o.code)::text, 6, '0')
        || '-00da-0000-0000-' || lpad(row_number() OVER (ORDER BY o.code)::text, 12, '0'))::uuid,
       o.id, 'shared', 'fixed', true
FROM operators o
WHERE o.code IN ('MTS_RU', 'MEGAFON_RU', 'BEELINE_RU', 'TELE2_RU')
ON CONFLICT DO NOTHING;

INSERT INTO tariff_periods (id, tariff_plan_id, start_date, end_date)
SELECT ('f6' || lpad(row_number() OVER (ORDER BY tp.id)::text, 6, '0')
        || '-00da-0000-0000-' || lpad(row_number() OVER (ORDER BY tp.id)::text, 12, '0'))::uuid,
       tp.id, '2026-01-01', '2026-12-31'
FROM tariff_plans tp
WHERE tp.id::text LIKE 'f5______-00da-0000-0000-%'
ON CONFLICT DO NOTHING;

-- Цена за сегмент: MTS 0.40, Megafon 0.38, Beeline 0.35, Tele2 0.30
INSERT INTO tariff_tiers (id, tariff_period_id, from_count, price_per_segment)
SELECT ('f7' || lpad(row_number() OVER (ORDER BY tp.id)::text, 6, '0')
        || '-00da-0000-0000-' || lpad(row_number() OVER (ORDER BY tp.id)::text, 12, '0'))::uuid,
       tper.id, 0,
       CASE o.code WHEN 'MTS_RU' THEN 0.40 WHEN 'MEGAFON_RU' THEN 0.38 WHEN 'BEELINE_RU' THEN 0.35 ELSE 0.30 END
FROM tariff_plans tp
JOIN operators o ON o.id = tp.operator_id
JOIN tariff_periods tper ON tper.tariff_plan_id = tp.id
WHERE tp.id::text LIKE 'f5______-00da-0000-0000-%'
ON CONFLICT DO NOTHING;

-- Тарификация доставленных сообщений (портал читает отсюда стоимость в детализации)
INSERT INTO tarification_log (id, client_id, message_id, operator_id, sender_category, strategy,
                              tariff_plan_id, tariff_period_id, segment_count, price_per_segment,
                              total_amount, idempotency_key, created_at)
SELECT ('aa' || lpad(substring(m.message_id from 6)::int::text, 6, '0')
        || '-00da-0000-0000-' || lpad(substring(m.message_id from 6)::int::text, 12, '0'))::uuid,
       m.client_id, m.id, m.operator_id, 'shared', 'fixed',
       tp.id, tper.id, m.segment_count, tt.price_per_segment,
       round(m.segment_count * tt.price_per_segment, 6),
       'demo-tl-' || substring(m.message_id from 6),
       m.delivered_at + interval '2 seconds'
FROM messages m
JOIN tariff_plans tp ON tp.operator_id = m.operator_id AND tp.sender_category = 'shared'
JOIN tariff_periods tper ON tper.tariff_plan_id = tp.id
JOIN tariff_tiers tt ON tt.tariff_period_id = tper.id AND tt.from_count = 0
WHERE m.id::text LIKE 'fa______-00da-0000-0000-%'
  AND m.status = 'delivered';

-- ---------- 7. Price rules (единая схема 000104) ----------
INSERT INTO price_rules (id, owner_type, owner_id, country, operator, sender_category, traffic_type,
                         valid_from, valid_to, price_model, price_value, tiers_json, created_by)
VALUES
    ('f8000001-00da-0000-0000-000000000001', 'platform', NULL, NULL, NULL, NULL, NULL,
     now() - interval '2 years', NULL, 'fixed', 0.35, NULL,
     (SELECT id FROM users WHERE username = 'admin')),
    ('f8000002-00da-0000-0000-000000000002', 'platform', NULL, 'RU', 'MTS_RU', NULL, NULL,
     now() - interval '1 year', NULL, 'fixed', 0.40, NULL,
     (SELECT id FROM users WHERE username = 'admin')),
    ('f8000003-00da-0000-0000-000000000003', 'platform', NULL, 'RU', 'BEELINE_RU', NULL, NULL,
     now() - interval '1 year', NULL, 'fixed', 0.35, NULL,
     (SELECT id FROM users WHERE username = 'admin')),
    ('f8000004-00da-0000-0000-000000000004', 'aggregator', 'c0000000-0000-0000-0000-000000000002', NULL, NULL, 'paid', 'transactional',
     now() - interval '6 months', NULL, 'tiered', NULL,
     '[{"from_segments":0,"price_per_segment":0.50},{"from_segments":10000,"price_per_segment":0.45},{"from_segments":100000,"price_per_segment":0.40}]'::jsonb,
     (SELECT id FROM users WHERE username = 'admin'))
ON CONFLICT DO NOTHING;

-- ---------- 8. Транзакции по счетам за 30 дней ----------
-- Ежедневные charge из фактической тарификации + несколько ручных операций.
-- Цепочка balance_before/after сходится к текущему балансу счёта из demo_seed.sql.
WITH daily AS (
    SELECT tl.client_id,
           date_trunc('day', tl.created_at) + interval '5 minutes' AS ts,
           round(sum(tl.total_amount), 2) AS amount,
           sum(tl.segment_count) AS segs
    FROM tarification_log tl
    WHERE tl.id::text LIKE 'aa______-00da-0000-0000-%'
    GROUP BY tl.client_id, date_trunc('day', tl.created_at)
),
extra(client_id, ts, type, amount, description, operation_kind, payment_method) AS (
    VALUES
        ('c0000000-0000-0000-0000-000000000001'::uuid, now() - interval '28 days', 'credit',   50000::numeric, 'Пополнение баланса (демо)',                'other',      'bank_card'),
        ('c0000000-0000-0000-0000-000000000001'::uuid, now() - interval '20 days', 'refund',     320.50::numeric, 'Возврат за недоставленные SMS (демо)',     'message',    NULL),
        ('c0000000-0000-0000-0000-000000000001'::uuid, now() - interval '12 days', 'charge',     500::numeric, 'Регистрация имени отправителя PROMO (демо)','sender_name', NULL),
        ('c0000000-0000-0000-0000-000000000001'::uuid, now() - interval '5 days',  'adjustment', -150::numeric, 'Корректировка тарифа (демо)',              'other',      NULL),
        ('c0000000-0000-0000-0000-000000000003'::uuid, now() - interval '26 days', 'credit',    15000::numeric, 'Пополнение баланса (демо)',                'other',      'invoice'),
        ('c0000000-0000-0000-0000-000000000002'::uuid, now() - interval '27 days', 'credit',    30000::numeric, 'Пополнение баланса (демо)',                'other',      'bank_transfer'),
        ('c0000000-0000-0000-0000-000000000002'::uuid, now() - interval '14 days', 'transfer_in', 10000::numeric, 'Перевод от платформы (демо)',              'other',      NULL),
        ('c0000000-0000-0000-0000-000000000004'::uuid, now() - interval '29 days', 'credit',   200000::numeric, 'Пополнение баланса (демо)',                'other',      'invoice')
),
all_tx AS (
    SELECT client_id, ts, 'charge'::varchar AS type, amount,
           'Списания за SMS за ' || date_trunc('day', ts)::date || ' (сегментов: ' || segs || ')' AS description,
           'message'::varchar AS operation_kind, NULL::varchar AS payment_method, segs
    FROM daily
    UNION ALL
    SELECT client_id, ts, type, amount, description, operation_kind, payment_method, NULL FROM extra
),
numbered AS (
    SELECT a.*,
           dense_rank() OVER (ORDER BY a.client_id) AS cseq,
           row_number() OVER (PARTITION BY a.client_id ORDER BY a.ts) AS rn
    FROM all_tx a
),
signed AS (
    SELECT n.*,
           CASE n.type
               WHEN 'charge' THEN -n.amount
               WHEN 'transfer_out' THEN -n.amount
               ELSE n.amount END AS signed_amount,
           a.balance - xx.net AS start_balance
    FROM numbered n
    JOIN accounts a ON a.client_id = n.client_id
    CROSS JOIN LATERAL (
        SELECT sum(CASE x.type WHEN 'charge' THEN -x.amount
                                WHEN 'transfer_out' THEN -x.amount
                                ELSE x.amount END) AS net
        FROM all_tx x WHERE x.client_id = n.client_id
    ) xx
)
INSERT INTO transactions (id, client_id, type, amount, currency, balance_before, balance_after,
                          description, message_id, payment_method, metadata, operation_kind, segment_count, created_at)
SELECT ('f4' || lpad((s.cseq * 100000 + s.rn)::text, 6, '0')
        || '-00da-0000-0000-' || lpad((s.cseq * 100000 + s.rn)::text, 12, '0'))::uuid,
       s.client_id, s.type, abs(s.amount), 'RUB',
       COALESCE(round(s.start_balance + sum(s.signed_amount) OVER (PARTITION BY s.client_id ORDER BY s.ts, s.rn
               ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING), 2), round(s.start_balance, 2)),
       round(s.start_balance + sum(s.signed_amount) OVER (PARTITION BY s.client_id ORDER BY s.ts, s.rn), 2),
       s.description, NULL, s.payment_method,
       CASE WHEN s.type = 'charge' THEN jsonb_build_object('source', 'message', 'segments', s.segs) ELSE NULL END,
       s.operation_kind,
       COALESCE(s.segs, 1),
       s.ts
FROM signed s;

COMMIT;

-- Статистика
SELECT 'sender_names'         AS entity, count(*) FROM sender_names         WHERE id::text LIKE 'f2%'
UNION ALL SELECT 'operator_registrations', count(*) FROM operator_registrations WHERE id::text LIKE 'f3%'
UNION ALL SELECT 'contact_lists',        count(*) FROM contact_lists        WHERE id::text LIKE 'fd%'
UNION ALL SELECT 'contacts',            count(*) FROM contacts             WHERE id::text LIKE 'fc%'
UNION ALL SELECT 'campaigns',           count(*) FROM campaigns            WHERE id::text LIKE 'ff%'
UNION ALL SELECT 'campaign_recipients', count(*) FROM campaign_recipients  WHERE id::text LIKE 'f0%'
UNION ALL SELECT 'campaign_snapshots',  count(*) FROM campaign_stats_snapshots WHERE campaign_id::text LIKE 'ff%'
UNION ALL SELECT 'messages',            count(*) FROM messages             WHERE id::text LIKE 'fa%'
UNION ALL SELECT 'messages_delivered',  count(*) FROM messages             WHERE id::text LIKE 'fa%' AND status = 'delivered'
UNION ALL SELECT 'dlr_receipts',        count(*) FROM dlr_receipts         WHERE id::text LIKE 'fb%'
UNION ALL SELECT 'tariff_plans',        count(*) FROM tariff_plans         WHERE id::text LIKE 'f5%'
UNION ALL SELECT 'tarification_log',    count(*) FROM tarification_log     WHERE id::text LIKE 'aa%'
UNION ALL SELECT 'price_rules',         count(*) FROM price_rules          WHERE id::text LIKE 'f8%'
UNION ALL SELECT 'transactions',        count(*) FROM transactions         WHERE id::text LIKE 'f4%'
UNION ALL SELECT 'tx_balance_check',    count(*) FROM transactions t
       JOIN accounts a ON a.client_id = t.client_id
       WHERE t.id::text LIKE 'f4%'
         AND t.created_at = (SELECT max(t2.created_at) FROM transactions t2
                             WHERE t2.client_id = t.client_id AND t2.id::text LIKE 'f4%')
         AND abs(a.balance - t.balance_after) > 0.01;
