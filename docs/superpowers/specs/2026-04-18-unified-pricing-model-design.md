# Unified Pricing Model (Pillar A)

**Дата:** 2026-04-18
**Автор:** brainstorm session
**Статус:** черновик спеки, требует review перед планом реализации
**Скоуп:** единая модель ценообразования для трёхуровневой иерархии (платформа → агрегатор → субаккаунт). НЕ включает: cost провайдеров/ownership (Pillar B), атомарность dual charge (Pillar C), performance-оптимизации сверх заложенных (Pillar D).

## 1. Мотивация

В текущей системе параллельно существуют три-четыре модели ценообразования:

- `tariff_plans` + `tariff_periods` + `tariff_tiers` (legacy, используется в горячем пути)
- `tariff_periods_new` (иерархическая, миграции применены, но горячий путь её не вызывает)
- `aggregator_tariffs` (плоская, по субаккаунтам, используется в горячем пути)
- `reseller_tariff_plans` (копия платформенной иерархии, не интегрирована в горячий путь)

Следствия: дублирование измерений (country/operator/sender_category/traffic_type) в разных таблицах, недетерминированный выбор между кодовыми путями, риск расхождения цен, непрозрачная маржа.

Цель спеки — одна модель правил ценообразования, работающая на всех трёх уровнях владения, с детерминированным fallback-алгоритмом и гарантированной производительностью на горячем пути.

## 2. Решения, принятые в штурме

| # | Вопрос | Решение |
|---|---|---|
| 1 | Топология | Фиксированные 3 уровня: платформа → агрегатор → субаккаунт. Нет рекурсии. |
| 2 | Как цена перетекает между уровнями | Наследование + точечные override (модель C). |
| 3 | Конфликт уровня vs специфичности | Owner-first. Catch-all субаккаунта бьёт специфичный тариф платформы. Менеджер управляет рисками через гранулярность override. |
| 4 | Приоритет измерений внутри уровня | `traffic_type > sender_category > operator > country` (битмап 8/4/2/1). |
| 5 | Архитектура данных | Единая таблица `price_rules` (source of truth) + денормализованная `resolved_rules` (горячий кеш, ленивая материализация по запросу). |
| 6 | Инвариант «цена всегда найдётся» | DB-constraint: обязательная catch-all строка на уровне платформы. Service on startup — sanity check. |
| 7 | Перекрытие периодов | Запрещено на уровне БД (`EXCLUDE` constraint). Редактирование = close+open атомарно. |
| 8 | Мультивалютность | В v1 единая валюта. Задел: `countries.currency` уже есть, включается позже без breaking change. |
| 9 | Тиры | Хранятся в `JSONB`. Парсинг снимается in-process LRU-кешем, ключ = `source_rule_id`, инвалидация по `rules_version`. |
| 10 | Модели ценообразования в v1 | `fixed`, `tiered` (прогрессивный), `prepaid_threshold`. **Исключены из v1:** retro-tier, `rolling_30d` period. |
| 11 | Usage counter | Един per-subaccount-per-period. `source_rule_id` **не** в PK: счётчик продолжается при смене правила. |
| 12 | TZ для `effective_date` | UTC. Цены меняются в 00:00 UTC. |
| 13 | Шаблоны (reseller) | Остаются как UX в админке. На уровне данных — сгенерированные независимые правила. |

## 3. Модель данных

### 3.1 `price_rules` — источник истины

```sql
CREATE TYPE price_owner_type AS ENUM ('platform', 'aggregator', 'subaccount');
CREATE TYPE price_model_type AS ENUM ('fixed', 'tiered', 'prepaid_threshold');

CREATE TABLE price_rules (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type       price_owner_type NOT NULL,
    owner_id         UUID NULL,                        -- NULL iff owner_type='platform'
    country          VARCHAR(2)  NULL REFERENCES countries(iso_code),
    operator         VARCHAR(50) NULL REFERENCES operators(code),
    sender_category  VARCHAR(16) NULL,                 -- CHECK IN ('paid','free','none')
    traffic_type     VARCHAR(16) NULL,                 -- CHECK IN ('transactional','marketing','service')
    valid_from       TIMESTAMPTZ NOT NULL,
    valid_to         TIMESTAMPTZ NULL,
    price_model      price_model_type NOT NULL,
    price_value      NUMERIC(12,6) NULL,
    tiers_json       JSONB NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by       UUID NULL,

    CONSTRAINT owner_id_platform_rule CHECK (
        (owner_type = 'platform' AND owner_id IS NULL)
        OR (owner_type <> 'platform' AND owner_id IS NOT NULL)
    ),
    CONSTRAINT value_xor_tiers CHECK (
        (price_model = 'fixed' AND price_value IS NOT NULL AND tiers_json IS NULL)
        OR (price_model IN ('tiered','prepaid_threshold') AND tiers_json IS NOT NULL)
    ),
    CONSTRAINT valid_to_after_from CHECK (valid_to IS NULL OR valid_to > valid_from)
);
```

**Invariant: catch-all на платформе обязателен и уникален.**

```sql
CREATE UNIQUE INDEX platform_catchall_singleton ON price_rules (owner_type)
WHERE owner_type = 'platform'
  AND country IS NULL AND operator IS NULL
  AND sender_category IS NULL AND traffic_type IS NULL
  AND valid_to IS NULL;
```

Дополнительно: на старте `tarification-service` читаем `price_rules` и валидируем наличие активной catch-all строки. Нет → сервис не поднимается.

**Invariant: перекрытия периодов запрещены.**

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE price_rules ADD CONSTRAINT no_overlapping_periods
EXCLUDE USING gist (
    owner_type WITH =,
    COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid) WITH =,
    COALESCE(country, '') WITH =,
    COALESCE(operator, '') WITH =,
    COALESCE(sender_category, '') WITH =,
    COALESCE(traffic_type, '') WITH =,
    tstzrange(valid_from, valid_to, '[)') WITH &&
);
```

**Основной композитный индекс для lookup:**

```sql
CREATE INDEX idx_price_rules_lookup ON price_rules
    (owner_type, owner_id, country, operator, sender_category, traffic_type, valid_from DESC);
```

### 3.2 `resolved_rules` — горячий кеш

```sql
CREATE TABLE resolved_rules (
    subaccount_id    UUID NOT NULL,
    country          VARCHAR(2) NOT NULL,
    operator         VARCHAR(50) NOT NULL,
    sender_category  VARCHAR(16) NOT NULL,
    traffic_type     VARCHAR(16) NOT NULL,
    effective_date   DATE NOT NULL,                  -- по UTC
    price_model      price_model_type NOT NULL,
    price_value      NUMERIC(12,6) NULL,
    tiers_json       JSONB NULL,
    source_rule_id   UUID NOT NULL REFERENCES price_rules(id),
    source_level     price_owner_type NOT NULL,
    aggregator_id    UUID NOT NULL,                  -- денорм для маржи и инвалидации
    resolved_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    rules_version    BIGINT NOT NULL,

    PRIMARY KEY (subaccount_id, country, operator, sender_category, traffic_type, effective_date)
);

CREATE INDEX idx_resolved_rules_by_aggregator ON resolved_rules (aggregator_id);
CREATE INDEX idx_resolved_rules_by_source ON resolved_rules (source_rule_id);
```

### 3.3 `price_rules_version` — версия источника истины

```sql
CREATE TABLE price_rules_version (
    id         INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),  -- singleton
    version    BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO price_rules_version (id) VALUES (1);
```

Триггер `AFTER INSERT/UPDATE/DELETE` на `price_rules` инкрементит версию + пишет событие в outbox (см. 3.5).

### 3.4 `subaccount_usage_counters` — единый счётчик

```sql
CREATE TABLE subaccount_usage_counters (
    subaccount_id    UUID NOT NULL,
    period_key       TEXT NOT NULL,                  -- '2026-04', '2026-04-18'
    segments_used    BIGINT NOT NULL DEFAULT 0,
    amount_charged   NUMERIC(14,6) NOT NULL DEFAULT 0,
    last_updated     TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (subaccount_id, period_key)
);
```

`period_key` вычисляется из `tiers_json.period` поля (`calendar_month` → `'YYYY-MM'`, `calendar_day` → `'YYYY-MM-DD'`). В v1 rolling окон нет.

### 3.5 Outbox инвалидации

```sql
CREATE TABLE resolved_rules_invalidation_outbox (
    id             BIGSERIAL PRIMARY KEY,
    rule_id        UUID NOT NULL,
    owner_type     price_owner_type NOT NULL,
    owner_id       UUID NULL,
    affected_dims  JSONB NOT NULL,                   -- {country, operator, ...} of the rule
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at   TIMESTAMPTZ NULL
);
```

Отдельный воркер `resolved_rules_janitor` читает unprocessed events и **удаляет** затронутые строки `resolved_rules` (ленивая инвалидация):

- `owner_type='subaccount'` → `DELETE WHERE subaccount_id = owner_id AND <dims match>`
- `owner_type='aggregator'` → `DELETE WHERE aggregator_id = owner_id AND <dims match>`
- `owner_type='platform'` → `DELETE WHERE <dims match>` (потенциально полная чистка)

После DELETE → `UPDATE outbox SET processed_at = now()`.

## 4. Алгоритм разрешения цены

### 4.1 Горячий путь

**Вход:** `(subaccount_id, country, operator, sender_category, traffic_type, now)`

**Стадия 1 — cache hit:**

```sql
SELECT price_model, price_value, tiers_json, source_rule_id, source_level, rules_version
FROM resolved_rules
WHERE subaccount_id = $1 AND country = $2 AND operator = $3
  AND sender_category = $4 AND traffic_type = $5
  AND effective_date = (now() AT TIME ZONE 'UTC')::date;
```

Сверяем `rules_version` со значением в `price_rules_version`:
- совпадает → используем
- отстала → удаляем строку, переходим в стадию 2

**Стадия 2 — resolve + materialize:**

```sql
WITH ranked AS (
  SELECT *,
    CASE owner_type
      WHEN 'subaccount' THEN 3
      WHEN 'aggregator' THEN 2
      WHEN 'platform'   THEN 1
    END AS owner_rank,
    (CASE WHEN traffic_type    IS NOT NULL THEN 8 ELSE 0 END) +
    (CASE WHEN sender_category IS NOT NULL THEN 4 ELSE 0 END) +
    (CASE WHEN operator        IS NOT NULL THEN 2 ELSE 0 END) +
    (CASE WHEN country         IS NOT NULL THEN 1 ELSE 0 END) AS specificity_rank
  FROM price_rules
  WHERE (
          (owner_type = 'subaccount' AND owner_id = $subaccount_id)
       OR (owner_type = 'aggregator' AND owner_id = $aggregator_id)
       OR (owner_type = 'platform')
        )
    AND (country         IS NULL OR country         = $country)
    AND (operator        IS NULL OR operator        = $operator)
    AND (sender_category IS NULL OR sender_category = $sender_category)
    AND (traffic_type    IS NULL OR traffic_type    = $traffic_type)
    AND valid_from <= $now
    AND (valid_to IS NULL OR valid_to > $now)
)
SELECT * FROM ranked
ORDER BY owner_rank DESC, specificity_rank DESC, valid_from DESC
LIMIT 1;
```

`aggregator_id` — из кеша `client:{subaccount_id}:aggregator_id` (Redis, TTL 5min).

Записываем результат в `resolved_rules` с `rules_version = version_at_select_time` (не `current_version` на момент записи):

```sql
INSERT INTO resolved_rules (..., rules_version) VALUES (..., $version_at_select)
ON CONFLICT (...) DO UPDATE SET ...;
```

Это защищает от race: если `price_rules` обновились между SELECT и INSERT, мы запишем старую версию, janitor её позже удалит.

### 4.2 Расчёт стоимости сообщения

```
resolved = resolve(...)
usage_before = SELECT segments_used FROM subaccount_usage_counters
               WHERE subaccount_id = X AND period_key = compute_period_key(resolved)

switch resolved.price_model:
  case 'fixed':
    cost = resolved.price_value * segments
  case 'tiered':
    cost = walk_tiers_progressive(resolved.tiers_json.tiers, usage_before, segments)
  case 'prepaid_threshold':
    included = resolved.tiers_json.included_segments
    overage  = resolved.tiers_json.overage_price
    cost = max(0, (usage_before + segments - included)) * overage

UPSERT subaccount_usage_counters
  SET segments_used = segments_used + segments,
      amount_charged = amount_charged + cost

charge(cost) via billing-service
```

Абонплата (`prepaid_amount` в `tiered_json` для `prepaid_threshold`) списывается отдельным scheduler-ом в начале периода, не на горячем пути.

### 4.3 `tiers_json` — схема

```json
// price_model='tiered'
{
  "period": "calendar_month",  // calendar_month | calendar_day
  "tiers": [
    {"up_to": 10000,   "price": 1.00},
    {"up_to": 100000,  "price": 0.85},
    {"up_to": null,    "price": 0.55}   // null = бесконечность
  ]
}

// price_model='prepaid_threshold'
{
  "period": "calendar_month",
  "prepaid_amount": 10000.00,
  "included_segments": 10000,
  "overage_price": 0.60
}
```

Валидация: при INSERT/UPDATE `price_rules` — application-level проверка (не DB CHECK), т.к. structure-validation JSONB в CHECK громоздкая.

## 5. Стратегия миграции (expand/contract)

### Фаза 1 — Expand schema

- Миграции: `price_rules`, `resolved_rules`, `price_rules_version`, `subaccount_usage_counters`, `resolved_rules_invalidation_outbox`, все constraints и индексы.
- Код: `PriceResolver` сервис, `PriceRuleRepository`, `ResolvedRulesRepository`, `ResolvedRulesJanitor` воркер. Фича-флаг `UNIFIED_TARIFICATION_ENABLED=false`.
- Юнит- и интеграционные тесты на фикстурах.
- Прод не затронут. Rollback = revert миграций.

### Фаза 2 — Backfill + dual-write

- `backfill_price_rules` скрипт:
  - `tariff_plans`/`tariff_periods`/`tariff_tiers` → `price_rules` (owner_type=platform)
  - `tariff_periods_new` → `price_rules` (owner_type=platform, измерения из scope)
  - `aggregator_tariffs` с `sub_account_id IS NULL` → owner_type=aggregator
  - `aggregator_tariffs` с `sub_account_id != NULL` → owner_type=subaccount
  - `reseller_tariff_plans` с назначениями → owner_type=subaccount или aggregator (в зависимости от scope шаблона)
- Лечение overlapping: сортировка по `created_at`, auto-close предыдущего при конфликте, с логом.
- Pre-flight аудит legacy-данных: NULL там, где не должно, сломанные FK, дубли.
- Включаем synchronous dual-write в одной DB-транзакции: API-изменения пишут в обе системы, фейл любой → rollback.
- Reconciliation-job 1 раз в сутки: для каждого (subaccount × measurement) сверяем цену legacy vs новый `PriceResolver`. Расхождения → алерт.
- Длительность: 1–2 недели.

### Фаза 3 — Cutover чтения

- `UNIFIED_TARIFICATION_ENABLED=true`. Горячий путь читает из `resolved_rules`/`price_rules`.
- Legacy dual-write продолжает работать (страховка).
- Мониторинг в Grafana: средняя цена, маржа, % cache hit/miss, % «цена не найдена» (должно быть 0%).
- Rollback = флаг → false. DB-состояние не трогаем.
- Длительность: 1–2 недели без инцидентов.

### Фаза 4 — Contract

- Отключаем dual-write. Legacy-таблицы переименовываются в `_deprecated_*`.
- Удаляем legacy-код: `tariff_plan_service`, `aggregator_tariffs` repo, `reseller_tariff_plans` repo как отдельные сущности.
- Через 1–2 недели — финальный DROP `_deprecated_*`.

Общий срок: 6–10 недель.

## 6. Границы спеки

**Не входит в Pillar A:**

- Cost провайдеров, `client_providers.ownership`, маржа платформы от cost (Pillar B).
- Атомарность dual charge (агрегатор + субаккаунт) и margin log consistency (Pillar C). Margin в A считается по факту через дополнительный resolve на уровне агрегатора: `margin = subaccount_resolved.price - aggregator_resolved.price_for_same_cell`.
- Батчинг списаний, снятие row-lock contention на `accounts`, pre-warming кеша при массовой инвалидации (Pillar D).

## 7. Известные риски и open questions

1. **Cache stampede при глобальной инвалидации.** DELETE всех строк `resolved_rules` при изменении platform catch-all → волна стадий 2 для всех активных субаккаунтов. Пока принимаем; pre-warmer — Pillar D.
2. **Row-level contention на `subaccount_usage_counters`.** Все сообщения одного субаккаунта сериализуются на одной строке. Пока принимаем; батчинг — Pillar D.
3. **Dual-write failure mode.** Сбой в новом коде → rollback всей транзакции → API-запись не проходит. Это делает фазу 2 рискованной для стабильности API. Митигация: тщательное тестирование новых путей до включения dual-write.
4. **JSONB schema validation.** В v1 — application-level. Если сформируется много ошибок — можно перевести на CHECK с `jsonb_typeof` позже.
5. **Шаблоны агрегаторов.** Сейчас проектируется как «сгенерировать N независимых правил». Если продукту нужно «изменение шаблона автоматически распространяется на всех его применивших» — это требует дополнительного слоя в БД (template_id в `price_rules` + resolve template at read). Решение отложено до следующего спека, если понадобится.

## 8. Deliverables

- Миграции SQL (фазы 1, 2, 4).
- Доменная модель Go: `PriceRule`, `ResolvedRule`, `SubaccountUsageCounter`, `PriceModel` enum.
- Repositories: `PriceRuleRepository`, `ResolvedRulesRepository`, `UsageCounterRepository`.
- Application-слой: `PriceResolver`, `CostCalculator`, `ResolvedRulesJanitor` воркер.
- Backfill-скрипт `backfill_price_rules`.
- Reconciliation-job.
- Интеграция в `tarification_service.TarifyMessage` под фича-флагом.
- Grafana-панель с метриками cache hit rate, resolve latency, % «цена не найдена».
- Документация для админ-API: как CRUD-ить `price_rules`, что такое catch-all invariant.
