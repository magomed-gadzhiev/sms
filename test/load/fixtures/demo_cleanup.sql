-- =============================================================================
-- Удаление ВСЕХ демо-данных: созданных demo_data.sql (маркер '-00da-0000-0000-')
-- и базовых из demo_seed.sql.
--
-- Пользователь `admin` НЕ удаляется (используется seed-admin'ом), но его пароль
-- мог быть перезаписан demo_seed.sql — см. README.
-- Партиции не трогаем.
-- =============================================================================

BEGIN;

-- ---------- Данные demo_data.sql ----------
DELETE FROM campaign_stats_snapshots WHERE campaign_id::text LIKE 'ff%';
DELETE FROM campaign_recipients       WHERE id::text LIKE 'f0______-00da-0000-0000-%';
DELETE FROM campaigns                 WHERE id::text LIKE 'ff%';

DELETE FROM contact_imports        WHERE id::text LIKE 'f1______-00da-0000-0000-%';
DELETE FROM contacts               WHERE id::text LIKE 'fc______-00da-0000-0000-%';
DELETE FROM contact_list_attributes WHERE id::text LIKE 'fe______-00da-0000-0000-%';
DELETE FROM contact_lists          WHERE id::text LIKE 'fd______-00da-0000-0000-%';

DELETE FROM dlr_receipts      WHERE id::text LIKE 'fb______-00da-0000-0000-%';
DELETE FROM messages          WHERE id::text LIKE 'fa______-00da-0000-0000-%';

DELETE FROM tarification_log  WHERE id::text LIKE 'aa______-00da-0000-0000-%';
DELETE FROM transactions      WHERE id::text LIKE 'f4______-00da-0000-0000-%';

DELETE FROM price_rules       WHERE id::text LIKE 'f8______-00da-0000-0000-%';
DELETE FROM resolved_rules_invalidation_outbox WHERE rule_id::text LIKE 'f8______-00da-0000-0000-%';

DELETE FROM tariff_tiers      WHERE id::text LIKE 'f7______-00da-0000-0000-%';
DELETE FROM tariff_periods    WHERE id::text LIKE 'f6______-00da-0000-0000-%';
DELETE FROM tariff_plans      WHERE id::text LIKE 'f5______-00da-0000-0000-%';

DELETE FROM operator_registrations WHERE id::text LIKE 'f3______-00da-0000-0000-%';
DELETE FROM default_sender_names
WHERE client_id IN ('c0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000003');
DELETE FROM sender_names           WHERE id::text LIKE 'f2______-00da-0000-0000-%';

-- ---------- Базовые строки demo_seed.sql ----------
DELETE FROM accounts          WHERE client_id::text LIKE 'c0000000-%';
DELETE FROM api_key_scopes    WHERE api_key_id::text LIKE 'e0%';
DELETE FROM api_keys          WHERE id::text LIKE 'e0%';
DELETE FROM users             WHERE username IN ('demo-client', 'demo-reseller');
DELETE FROM templates
WHERE client_id = 'c0000000-0000-0000-0000-000000000001'
  AND name IN ('otp-code', 'delivery-notify', 'promo-offer');
DELETE FROM client_companies  WHERE client_id::text LIKE 'c0000000-%';
DELETE FROM clients           WHERE id::text LIKE 'c0000000-%';
DELETE FROM routes            WHERE id::text LIKE 'b0%';
DELETE FROM providers         WHERE id::text LIKE 'a0%';
DELETE FROM hlr_providers     WHERE id::text LIKE '11110000-%';
DELETE FROM companies         WHERE id::text LIKE 'cc0%';

COMMIT;

-- Контроль: всё удалено (ожидаем 0)
SELECT 'messages'   AS leftover, count(*) FROM messages        WHERE id::text LIKE 'fa%'
UNION ALL SELECT 'contacts',   count(*) FROM contacts          WHERE id::text LIKE 'fc%'
UNION ALL SELECT 'campaigns',  count(*) FROM campaigns         WHERE id::text LIKE 'ff%'
UNION ALL SELECT 'tx',         count(*) FROM transactions      WHERE id::text LIKE 'f4%'
UNION ALL SELECT 'demo_clients', count(*) FROM clients         WHERE id::text LIKE 'c0000000-%';
