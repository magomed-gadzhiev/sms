-- Бизнес-классификация транзакций (MVP-feedback №5).
-- Финансовый `type` (charge/credit/refund/adjustment/transfer_*) остаётся
-- как индикатор направления (приход/расход). `operation_kind` отвечает на
-- вопрос «за что списано/начислено» (Сообщения / Имена отправителей /
-- Шаблон оператора / Тариф оператора / Прочее).
--
-- Стиль колонки выбран VARCHAR + CHECK (как у существующего `type`),
-- не CREATE TYPE — однотипно с миграциями 000006/000119.

ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS operation_kind VARCHAR(32) NOT NULL DEFAULT 'other'
        CHECK (operation_kind IN (
            'message',
            'sender_name',
            'operator_template',
            'operator_tariff',
            'other'
        ));

-- Backfill существующих записей по эвристикам.
-- Если ничего не подошло — остаётся 'other' (по умолчанию).

UPDATE transactions SET operation_kind = 'message'
WHERE operation_kind = 'other'
  AND (message_id IS NOT NULL OR (metadata->>'source') = 'message');

UPDATE transactions SET operation_kind = 'sender_name'
WHERE operation_kind = 'other'
  AND ((metadata->>'source') = 'sender_name'
       OR description ILIKE '%имя отправителя%'
       OR description ILIKE '%sender name%'
       OR description ILIKE '%sender_name%');

UPDATE transactions SET operation_kind = 'operator_template'
WHERE operation_kind = 'other'
  AND ((metadata->>'source') = 'operator_template'
       OR description ILIKE '%шаблон оператор%'
       OR description ILIKE '%operator_template%');

UPDATE transactions SET operation_kind = 'operator_tariff'
WHERE operation_kind = 'other'
  AND ((metadata->>'source') = 'operator_tariff'
       OR description ILIKE '%тариф оператор%'
       OR description ILIKE '%operator_tariff%');

-- Индекс для отчёта по бизнес-операциям.
-- idx_transactions_client_created уже существует (миграция 000006).
CREATE INDEX IF NOT EXISTS idx_transactions_client_kind_created
    ON transactions(client_id, operation_kind, created_at DESC);
