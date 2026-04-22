BEGIN;

-- commit_idempotency_guard — non-partitioned таблица, используется как
-- идемпотентность-guard для ChargeMessageDual.
--
-- Причина отдельной таблицы: aggregator_margin_log partitioned by created_at,
-- а Postgres запрещает solo-unique constraint на partitioned table без
-- включения partition key. Composite UNIQUE (idempotency_key, created_at)
-- не работает для ON CONFLICT (idempotency_key), что ломало всю
-- идемпотентность dual-charge.
--
-- Решение: маленькая non-partitioned таблица только для guard'а. PK =
-- message_id → встроенный unique. margin_log остаётся partitioned для
-- analytics, но без роли guard'а — в него пишут только после успешной
-- вставки в guard.
--
-- Retention: таблица растёт со скоростью ~1 row на subaccount SMS. Через
-- несколько месяцев понадобится архивация (отдельная задача). TTL через
-- DELETE WHERE created_at < NOW() - interval '30 days' — когда понадобится.
CREATE TABLE commit_idempotency_guard (
    message_id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индекс для будущего retention-job'а.
CREATE INDEX idx_commit_idempotency_guard_created_at ON commit_idempotency_guard(created_at);

COMMIT;
