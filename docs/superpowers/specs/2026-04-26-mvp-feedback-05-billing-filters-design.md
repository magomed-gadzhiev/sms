# Spec №5: Биллинг — расширенные фильтры, группировка, классификация операций

**Дата:** 2026-04-26
**Источник:** MVP-таблица, строка «Финансы → Биллинг»
**Статус:** Не сделано (есть только базовые фильтры).

## Контекст

Спека:
> Добавить Фильтры:
>   - Период
>   - Операция (Сообщения / Имена отправителей / Шаблон оператора / Тариф оператора / Другие услуги)
>   - Группировка (По дням / По месяцам / По годам)
>
> Таблица: Дата / Операция / Приход (пополнения) / Расход / Итог.

Аудит:
- UI: [BillingPage.tsx](portal-frontend/src/pages/billing/BillingPage.tsx) — фильтры `date_from`, `date_to`, `type` (charge/credit/refund/transfer_in/transfer_out). Колонки таблицы: Дата / Тип / Сумма / Баланс после / Описание / ID сообщения.
- Backend: [billing/handlers/billing.go:57](internal/gateway/portal/handlers/billing.go#L57) `GetTransactions`. В [transaction_repository.go](internal/services/billing/infrastructure/repository/transaction_repository.go) — методы по `client_id+type` и `client_id+period`, **агрегаций нет**.
- БД: `transactions` (`migrations/000006`, `000119`). Enum типов: `charge | credit | refund | adjustment | transfer_out | transfer_in`. Это финансовая категоризация, **не бизнес-операция**.
- Биллинг шаблонов/имён отправителей живёт в отдельной таблице `sender_name_billing_records` (`migrations/000066`) и в `transactions` не вливается.

## Решение

### Подход

1. **Добавить колонку `operation_kind` в `transactions`** (бизнес-классификация). Финансовый `type` оставить — у него своя роль (приход/расход).
2. **Backfill** существующих записей по `metadata.source` / `description` / связке с `messages`/`sender_name_billing_records`.
3. **Агрегация на лету** через `date_trunc + SUM` с индексом `(client_id, created_at)`. Materialized view не делаем — объёмы пока < 10M tx/мес/клиент.
4. UI расширить фильтрами и group-by.

### Что меняем

**1. Миграция**

Новая миграция `000120_add_operation_kind_to_transactions`:

```sql
CREATE TYPE operation_kind AS ENUM (
  'message',          -- списание за SMS (= Сообщения)
  'sender_name',      -- регистрация/продление имени отправителя
  'operator_template',-- регистрация шаблона у оператора
  'operator_tariff',  -- начисление по тарифу оператора
  'other'             -- прочее (топап, корректировка, перевод)
);

ALTER TABLE transactions
  ADD COLUMN operation_kind operation_kind NOT NULL DEFAULT 'other';

-- Backfill (heuristics)
UPDATE transactions SET operation_kind = 'message'
  WHERE message_id IS NOT NULL OR (metadata->>'source') = 'message';
UPDATE transactions SET operation_kind = 'sender_name'
  WHERE (metadata->>'source') = 'sender_name'
     OR description ILIKE '%имя отправителя%'
     OR description ILIKE '%sender name%';
UPDATE transactions SET operation_kind = 'operator_template'
  WHERE (metadata->>'source') = 'operator_template'
     OR description ILIKE '%шаблон оператор%';
UPDATE transactions SET operation_kind = 'operator_tariff'
  WHERE (metadata->>'source') = 'operator_tariff'
     OR description ILIKE '%тариф оператор%';

CREATE INDEX idx_transactions_client_created
  ON transactions(client_id, created_at DESC);
CREATE INDEX idx_transactions_client_kind_created
  ON transactions(client_id, operation_kind, created_at DESC);
```

`down`: `DROP COLUMN`, `DROP INDEX`, `DROP TYPE`.

**2. Все места записи `transactions` явно проставляют `operation_kind`**

Список мест:
- `internal/services/billing/application/billing_service.go` — `ChargeMessage` → `kind=message`.
- `internal/services/sender/...` (или где списывается за регистрацию имён) → `kind=sender_name`.
- `internal/services/template/...` (если есть billing операторских шаблонов) → `kind=operator_template`.
- `internal/services/tarification/...` → `kind=operator_tariff`.
- Топапы / корректировки / переводы → `kind=other`.

В коде billing-service сделать обязательный аргумент `kind operation_kind` на всех методах, создающих транзакции — компилятор поймает пропущенные места.

**3. API: `GET /portal/v1/billing/transactions`**

Параметры:
- `date_from`, `date_to` — период (обязателен; дефолт = текущий месяц если не передан).
- `kind` — `message | sender_name | operator_template | operator_tariff | other` (опционально, мульти-выбор через повторение `?kind=message&kind=other`).
- `group_by` — `none | day | month | year` (дефолт `none`; при `none` возвращается плоский список как сейчас).
- `page`, `page_size` — пагинация (дефолт 50, как сейчас).

При `group_by != none` ответ возвращает агрегированные строки:
```json
{
  "items": [
    {
      "bucket_start": "2026-04-01T00:00:00Z",
      "bucket_label": "Апрель 2026",
      "kind": "message",
      "income": "0.00",
      "expense": "1234.56",
      "total": "-1234.56",
      "count": 567
    },
    ...
  ],
  "summary": {
    "income": "5000.00",
    "expense": "4500.00",
    "total": "500.00"
  }
}
```

**Логика агрегации (на лету):**
```sql
SELECT
  date_trunc($group_by, created_at) AS bucket_start,
  operation_kind AS kind,
  SUM(CASE WHEN type IN ('credit','refund','transfer_in') THEN amount ELSE 0 END) AS income,
  SUM(CASE WHEN type IN ('charge','adjustment','transfer_out') THEN amount ELSE 0 END) AS expense,
  COUNT(*) AS count
FROM transactions
WHERE client_id = $1
  AND created_at >= $2 AND created_at < $3
  AND ($kind IS NULL OR operation_kind = ANY($kind))
GROUP BY bucket_start, kind
ORDER BY bucket_start DESC, kind;
```

`income - expense = total` считается на бэке. `summary` — сумма по всем группам.

**4. Frontend: BillingPage.tsx**

a. Добавить фильтры:
   - Период: уже есть, оставить.
   - Операция: мульти-селект с лейблами «Сообщения / Имена отправителей / Шаблон оператора / Тариф оператора / Другие услуги». Маппинг лейблов на `kind` в константе.
   - Группировка: радио «Без группировки / По дням / По месяцам / По годам» (дефолт «Без группировки»).

b. Таблица:
   - При `group_by=none` (как сейчас): колонки **Дата / Операция / Приход / Расход / Описание / ID сообщения**. Поле «Сумма» заменить на пару «Приход / Расход» (одна из двух пуста по строке). «Баланс после» можно оставить как опциональную колонку.
   - При `group_by≠none`: колонки **Период / Операция / Приход / Расход / Итог / Кол-во**. Период — отформатированный `bucket_label`.
   - Под таблицей — totals-строка из `summary`: «Итого: Приход 5000 ₽ / Расход 4500 ₽ / Сальдо +500 ₽».

### Открытые вопросы (закрываю сам)

- **Дефолтная группировка:** «Без группировки» (плоский список как сейчас) — пользователь явно выбирает агрегацию. Не делаем дефолт «по дням», чтобы не менять текущий UX.
- **Период по умолчанию:** текущий месяц (от 1-го числа до сегодня). Если квартал/год велик — пользователь расширяет вручную.
- **«Итог»:** разница `Приход − Расход` для строки. Running balance не считаем — это отдельная фича «Выписка», в спеке не просят.
- **Heuristic backfill:** если совпадений нет, остаётся `other`. Это приемлемо — старые записи в любом случае не критичны для отчётности.

### Что НЕ делаем

- Materialized view. Слишком рано для нашего объёма.
- Экспорт в Excel. Если нужен — отдельный спек (часть фичи «Экспорт детализации» №10).
- Графики/диаграммы. Спека этого не просит.

## Acceptance criteria

1. Миграция `000120` накатывается, `down` чистый.
2. Все места создания транзакций используют `operation_kind`. Тест: создать сообщение, проверить что в `transactions` соответствующая строка имеет `kind='message'`.
3. API `GET /portal/v1/billing/transactions` с `group_by=month` возвращает агрегаты по месяцам.
4. UI фильтр «Операция» — мульти-селект, при выборе нескольких операций отправляется массив `kind`.
5. UI фильтр «Группировка» меняет колонки таблицы.
6. Под таблицей — totals.
7. Производительность: запрос `group_by=day` за 12 месяцев на клиенте с 1M tx — отвечает за <500мс (с индексом `idx_transactions_client_created`).

## Размер задачи

M (4-5 рабочих дней: миграция с backfill + рефакторинг billing-service для обязательного `kind` + API агрегации + UI расширение).
