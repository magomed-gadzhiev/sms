-- Maintenance: маркирует сообщения, застрявшие в status='pending' до фикса
-- pipeline (commits 7bad29e + 7b348d1), как rejected с пометкой в
-- status_message. После фикса такие сообщения не появляются.
--
-- Запуск: ./scripts/server.sh exec 'docker cp scripts/maintenance/fix_historical_pending.sql
--           postgres:/tmp/fix.sql && docker exec postgres psql -U smpp -d smpp_db -f /tmp/fix.sql'

-- NB: в prod-подобной БД может быть много pending от load-test. Ограничиваем
-- скрипт конкретными tenant'ами чтобы не тронуть load-test historical data.

-- Dry-run count (tenants должны передаваться через :clients):
SELECT COUNT(*) AS stuck_pending
FROM messages
WHERE status = 'pending'
  AND created_at < '2026-04-22 19:30:00+00'
  AND client_id IN (
    'a0000000-0000-0000-0000-000000000001', -- TestAggregator
    'a0000000-0000-0000-0000-000000000002'  -- TestSubAccount
  );

-- Apply:
UPDATE messages
SET status = 'rejected',
    status_message = 'legacy pre-fix pending — tarification rejected без публикации в sms.sent',
    updated_at = NOW()
WHERE status = 'pending'
  AND created_at < '2026-04-22 19:30:00+00'
  AND client_id IN (
    'a0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000002'
  )
RETURNING id, client_id, source, destination, created_at;
