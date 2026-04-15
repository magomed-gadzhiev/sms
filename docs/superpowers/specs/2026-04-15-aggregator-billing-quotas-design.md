# Aggregator Billing: Infrastructure Quotas & Billing Modes

**Date:** 2026-04-15  
**Status:** Draft  
**Approach:** Extension of existing usage_counters / prepaid_threshold infrastructure

## Problem

Агрегаторы приходят на платформу со своими провайдерами и субаккаунтами. Текущая модель тарифицирует каждое сообщение, но для агрегаторов нужна другая модель: пакетная квота на использование инфраструктуры с overage после исчерпания. При этом агрегатор может использовать как своих, так и общих провайдеров платформы — стоимость для платформы принципиально разная.

Также нужна гибкость в биллинге субаккаунтов: агрегатор может вести биллинг через платформу, а может управлять финансами субаккаунтов самостоятельно.

## Design Decisions

- **Квота как pool:** Все субаккаунты агрегатора расходуют единую квоту. Агрегатор контролирует расход через daily/monthly лимиты на субаккаунтах, без выделения подквот.
- **Overage прозрачен:** При исчерпании квоты субаккаунты продолжают работать, overage списывается с баланса агрегатора автоматически.
- **Три billing_mode:** own / aggregator / hybrid на уровне субаккаунта.
- **Квота опциональна:** Без записи в aggregator_quotas агрегатор работает как обычный клиент.
- **Субаккаунты работают с платформой напрямую** (API, портал), но финансовое управление — через агрегатора.

## Data Model

### Новая таблица: aggregator_quotas

```sql
CREATE TABLE aggregator_quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    segment_limit BIGINT NOT NULL,
    segments_used BIGINT NOT NULL DEFAULT 0,
    overage_rate NUMERIC(10,4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'RUB',
    auto_renew BOOLEAN NOT NULL DEFAULT true,
    notified_80pct BOOLEAN NOT NULL DEFAULT false,
    notified_100pct BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(aggregator_id, period_start)
);
```

### ALTER: clients

```sql
ALTER TABLE clients
    ADD COLUMN billing_mode VARCHAR(20) NOT NULL DEFAULT 'own'
        CHECK (billing_mode IN ('own', 'aggregator', 'hybrid')),
    ADD COLUMN spending_limit_monthly NUMERIC(12,2),
    ADD COLUMN spending_limit_daily NUMERIC(12,2);
```

- `billing_mode` — определяет, с чьего баланса списывать за трафик через общего провайдера.
- `spending_limit_monthly` / `spending_limit_daily` — максимальная сумма, которую субаккаунт может потратить с баланса агрегатора (для режимов aggregator и hybrid).

### ALTER: transactions

```sql
ALTER TABLE transactions
    ADD COLUMN attributed_sub_account_id UUID REFERENCES clients(id);
```

Когда списание идёт с баланса агрегатора за субаккаунт — фиксирует инициатора расхода. Позволяет агрегатору видеть breakdown по субаккаунтам.

### Определение типа провайдера

Не требует новых таблиц. При роутинге:
- `provider.id IN aggregator_profiles.allowed_provider_ids` → свой провайдер (трафик = $0)
- Иначе → общий провайдер (списание с баланса по тарифу)

## Tarification Pipeline

Когда субаккаунт агрегатора отправляет сообщение:

### Шаг 1: Определение контекста

```
sub_account = отправитель
aggregator = sub_account.parent_client
quota = aggregator_quotas WHERE aggregator_id = aggregator.id
        AND period_start <= now() AND period_end > now()
```

### Шаг 2: Инфраструктурная квота (всегда)

```
UPDATE aggregator_quotas
SET segments_used = segments_used + $segments
WHERE id = $quota_id
RETURNING segments_used, segment_limit
```

- `segments_used <= segment_limit` → квота покрывает, $0
- `segments_used > segment_limit` → overage: списание `overage_rate × segments` с баланса агрегатора (transaction.attributed_sub_account_id = sub_account.id)
- Квота отсутствует → шаг пропускается

### Шаг 3: Роутинг → выбор провайдера

Стандартный роутинг. Определяем: свой или общий провайдер.

### Шаг 4: Списание за трафик (только общий провайдер)

```
billing_mode = "own":
    TarifyMessage(client = sub_account)
    Нет денег → отклонение

billing_mode = "aggregator":
    TarifyMessage(client = aggregator)
    transaction.attributed_sub_account_id = sub_account.id

billing_mode = "hybrid":
    TarifyMessage(client = sub_account)
    Успех → готово
    Нет денег → TarifyMessage(client = aggregator)
                transaction.attributed_sub_account_id = sub_account.id
```

Своий провайдер → шаг 4 пропускается (трафик = $0).

### Шаг 5: Отправка

Hybrid-режим: частичное списание не производится. Если у субаккаунта недостаточно средств — всё сообщение целиком оплачивает агрегатор.

## Quota Lifecycle

**Создание:** Админ платформы создаёт квоту при подключении агрегатора.

**Автопродление:** Cron-задача в начале нового периода создаёт новую запись с теми же параметрами и обнулённым segments_used. Старая запись сохраняется для истории.

**Изменение пакета:** Админ может изменить segment_limit или overage_rate на текущий период — применяется немедленно. Если новый лимит меньше текущего расхода — агрегатор сразу в overage.

**Без квоты:** Агрегатор без записи в aggregator_quotas работает как обычный клиент — pipeline пропускает шаг 2.

## Aggregator Controls for Sub-Accounts

### Существующие механизмы
- `daily_limit` / `monthly_limit` на субаккаунте (в сегментах)
- `rate_limit` per second/minute на субаккаунте

### Новые механизмы
- `billing_mode` — режим биллинга
- `spending_limit_monthly` / `spending_limit_daily` — максимальная сумма с баланса агрегатора за субаккаунт

spending_limit проверяется при billing_mode = aggregator/hybrid перед списанием с баланса агрегатора:
```sql
SELECT COALESCE(SUM(amount), 0)
FROM transactions
WHERE attributed_sub_account_id = $sub_account_id
  AND created_at >= $period_start
```

## Resource Exhaustion Scenarios

| Квота | Баланс агрегатора | billing_mode | Свой провайдер | Результат |
|-------|-------------------|--------------|----------------|-----------|
| В рамках | Есть | any | Да | Отправка, квота -1 |
| В рамках | Есть | any | Нет | Отправка, квота -1, списание за трафик по billing_mode |
| В рамках | Нет | own | Нет | Отправка если у субаккаунта есть баланс, иначе отклонение |
| В рамках | Нет | aggregator | Нет | Отклонение (трафик через общего провайдера оплатить нечем) |
| В рамках | Нет | hybrid | Нет | Отправка если у субаккаунта есть баланс, иначе отклонение (aggregator fallback невозможен) |
| В рамках | Нет | own | Да | Отправка, квота -1, трафик $0 |
| В рамках | Нет | aggregator | Да | Отправка, квота -1, трафик $0 (свой провайдер, баланс не нужен) |
| Исчерпана | Есть | any | Да | Отправка, overage с агрегатора |
| Исчерпана | Есть | any | Нет | Отправка, overage + трафик с агрегатора/субаккаунта по billing_mode |
| Исчерпана | Нет | own | Нет | Отклонение (overage оплатить нечем) |
| Исчерпана | Нет | aggregator | Нет | Отклонение |
| Нет квоты | — | — | — | Работает как обычный клиент через TarifyMessage |

## Notifications

| Событие | Порог | Канал |
|---------|-------|-------|
| Квота приближается | 80% segment_limit | in-app + email |
| Квота исчерпана | 100% segment_limit | in-app + email |
| Баланс низкий | low_balance_threshold (существующий) | in-app + email |
| Субаккаунт достиг spending_limit | 100% spending_limit_monthly | in-app |

Проверка порогов — при инкременте квоты, с однократной отправкой за период (флаги notified_80pct, notified_100pct).

## Monitoring

### Prometheus-метрики

```
sms_aggregator_quota_used{aggregator_id}        — gauge, текущий segments_used
sms_aggregator_quota_limit{aggregator_id}       — gauge, segment_limit
sms_aggregator_quota_utilization{aggregator_id} — gauge, % использования
sms_aggregator_overage_total{aggregator_id}     — counter, сегменты в overage
sms_aggregator_billing_mode_charges{aggregator_id, sub_account_id, mode} — counter
```

## API Surface

### Admin API (управление квотами)

- `POST /api/admin/aggregators/{id}/quotas` — создать квоту
- `PUT /api/admin/aggregators/{id}/quotas/{quota_id}` — изменить квоту
- `GET /api/admin/aggregators/{id}/quotas` — список квот (текущая + история)
- `GET /api/admin/aggregators/quotas/overview` — все агрегаторы с расходом квот

### Portal API (для агрегатора)

- `GET /api/portal/quota` — текущая квота (использовано / лимит / % / прогноз)
- `GET /api/portal/quota/spending` — breakdown расходов по субаккаунтам и провайдерам
- `PUT /api/portal/sub-accounts/{id}/billing` — установить billing_mode, spending_limits
- `GET /api/portal/sub-accounts/{id}/billing` — текущие настройки биллинга субаккаунта

## Migration & Backward Compatibility

**Обычные клиенты:** Никаких изменений. billing_mode = 'own' по умолчанию, квота отсутствует → pipeline работает как раньше.

**Существующие агрегаторы:** Продолжают работать без квоты. Маркап и margin tracking через aggregator_tariffs / aggregator_margin_log остаются без изменений.

**Включение нового режима** — поэтапно для каждого агрегатора:
1. Админ создаёт aggregator_quota
2. Агрегатор устанавливает billing_mode на субаккаунтах
3. Агрегатор настраивает spending_limit при необходимости

**Что НЕ меняется:**
- Стандартный TarifyMessage
- usage_counters для per-client тарификации
- aggregator_tariffs и margin log
- Роутинг (добавляется только проверка свой/общий провайдер)
- Биллинг обычных клиентов

## Implementation Order

1. Миграция: aggregator_quotas, ALTER clients, ALTER transactions
2. Backend: логика квоты и billing_mode в тарификационном pipeline
3. Admin API: CRUD квот
4. Portal API: управление billing_mode, spending_limits, просмотр квоты
5. Frontend: UI для агрегатора и админа
6. Мониторинг: метрики + уведомления
