# Data Model: Multichannel Cascade Delivery

**Feature**: `012-multichannel-cascade`  
**Branch**: `012-multichannel-cascade`  
**Date**: 2026-03-31

---

## Entities

### 1. DeliveryChannel (Канал доставки)

```
delivery_channels
─────────────────
id              UUID PK
channel_type    TEXT NOT NULL  -- 'sms' | 'flash_call' | 'reverse_call' | 'messenger'
name            TEXT NOT NULL
description     TEXT
config          JSONB NOT NULL DEFAULT '{}'  -- provider URL, API key (encrypted), etc.
active          BOOLEAN NOT NULL DEFAULT true
created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()

UNIQUE (channel_type)
```

**Примечания**:
- `config` хранит параметры подключения провайдера (зашифрованные sensitive поля — Constitution V).
- SMS-канал создаётся при первой миграции как системный.
- `channel_type` — дискриминатор для выбора Go-адаптера при компиляции.

---

### 2. DeliveryStrategy (Стратегия доставки)

```
delivery_strategies
───────────────────
id              UUID PK
name            TEXT NOT NULL
description     TEXT
mode            TEXT NOT NULL  -- 'sequential' | 'parallel'
active          BOOLEAN NOT NULL DEFAULT true
created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()

UNIQUE (name)
```

---

### 3. DeliveryStrategyStep (Шаг стратегии)

```
delivery_strategy_steps
───────────────────────
id              UUID PK
strategy_id     UUID NOT NULL REFERENCES delivery_strategies(id) ON DELETE CASCADE
channel_id      UUID NOT NULL REFERENCES delivery_channels(id)
step_order      INT NOT NULL  -- 1-based ordering
timeout_s       INT NOT NULL DEFAULT 30  -- seconds to wait before fallback
billable        BOOLEAN NOT NULL DEFAULT true  -- is attempt billable even if fails?
created_at      TIMESTAMPTZ NOT NULL DEFAULT now()

UNIQUE (strategy_id, step_order)
UNIQUE (strategy_id, channel_id)  -- одинаковый канал дважды в стратегии — ошибка
```

---

### 4. Delivery (Доставка)

Партиционируется по месяцам (Constitution V — high write volume).

```
deliveries
──────────
id              UUID PK
client_id       UUID NOT NULL
message_id      UUID  -- nullable (ссылка на messages если применимо)
strategy_id     UUID NOT NULL REFERENCES delivery_strategies(id)
recipient       TEXT NOT NULL  -- MSISDN
text            TEXT NOT NULL
sender_name     TEXT
status          TEXT NOT NULL  -- 'pending' | 'in_progress' | 'delivered' | 'failed' | 'cancelled'
current_step    INT NOT NULL DEFAULT 1
delivered_via   TEXT  -- channel_type канала, доставившего сообщение
total_cost      NUMERIC(12,4) DEFAULT 0
currency        TEXT NOT NULL DEFAULT 'RUB'
request_id      TEXT  -- X-Request-ID для tracing
created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()

-- Партиционирование по created_at (monthly)
```

**Состояния delivery**:
```
pending → in_progress → delivered
                      → failed
         in_progress → cancelled  (if channel disabled mid-cascade)
```

---

### 5. DeliveryAttempt (Попытка доставки)

Партиционируется по месяцам (Constitution V — high write volume, ~N attempts per delivery).

```
delivery_attempts
─────────────────
id              UUID PK
delivery_id     UUID NOT NULL  -- → deliveries.id
channel_id      UUID NOT NULL  -- → delivery_channels.id
channel_type    TEXT NOT NULL  -- денормализовано для быстрого доступа
step_order      INT NOT NULL
status          TEXT NOT NULL  -- 'pending' | 'sent' | 'delivered' | 'failed' | 'timeout' | 'skipped' | 'late_duplicate'
provider_ref    TEXT  -- external ID от провайдера
cost            NUMERIC(12,4) DEFAULT 0
currency        TEXT NOT NULL DEFAULT 'RUB'
error_message   TEXT
sent_at         TIMESTAMPTZ
result_at       TIMESTAMPTZ  -- когда получен результат (webhook / timeout)
created_at      TIMESTAMPTZ NOT NULL DEFAULT now()

-- Партиционирование по created_at (monthly)
```

**Статусы attempt**:
```
pending → sent → delivered     (канал успешно доставил)
               → failed        (явный отказ провайдера)
               → timeout       (scheduler сработал по timeout_s)
               → late_duplicate (доставлен после завершения каскада другим каналом)
        → skipped              (reachability check — канал недоступен для оператора)
```

---

### 6. OperatorChannelSupport (Поддержка каналов оператором)

```
operator_channel_support
────────────────────────
id              UUID PK
operator_id     UUID NOT NULL REFERENCES operators(id)
channel_type    TEXT NOT NULL
supported       BOOLEAN NOT NULL DEFAULT true
notes           TEXT
updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
updated_by      UUID  -- admin user id

UNIQUE (operator_id, channel_type)
```

---

## Relationships

```
delivery_strategies ──< delivery_strategy_steps >── delivery_channels
       │
       │ (strategy_id)
       ▼
    deliveries ──< delivery_attempts >── delivery_channels
       │
       │ (implicit via recipient MSISDN)
       ▼
    operators ──< operator_channel_support >── channel_type
```

---

## Migrations Plan

| Номер | Файл | Что создаёт |
|-------|------|-------------|
| 000067 | `cascade_channels` | `delivery_channels` |
| 000068 | `delivery_strategies` | `delivery_strategies`, `delivery_strategy_steps` |
| 000069 | `deliveries` | `deliveries` (monthly partitioned), `delivery_attempts` (monthly partitioned), индексы |
| 000070 | `operator_channel_support` | `operator_channel_support`, seed данные для SMS |

---

## Kafka Events (Schema)

Все события включают `schema_version` (Constitution II).

### CascadeStartEvent (топик: `cascade.start`)
```json
{
  "schema_version": "1",
  "delivery_id": "uuid",
  "client_id": "uuid",
  "strategy_id": "uuid",
  "recipient": "+79001234567",
  "text": "Your code: 1234",
  "sender_name": "MyApp",
  "request_id": "uuid"
}
```

### CascadeAttemptSendCommand (топик: `cascade.attempt.send`)
```json
{
  "schema_version": "1",
  "attempt_id": "uuid",
  "delivery_id": "uuid",
  "channel_type": "flash_call",
  "channel_config": {},
  "recipient": "+79001234567",
  "text": "Your code: 1234",
  "timeout_s": 15,
  "request_id": "uuid"
}
```

### CascadeAttemptResultEvent (топик: `cascade.attempt.result`)
```json
{
  "schema_version": "1",
  "attempt_id": "uuid",
  "delivery_id": "uuid",
  "channel_type": "flash_call",
  "status": "delivered",  // "delivered" | "failed" | "timeout"
  "provider_ref": "ext-123",
  "error_message": "",
  "result_at": "2026-03-31T10:00:00Z"
}
```

### CascadeBillingCommand (топик: `cascade.billing`)
```json
{
  "schema_version": "1",
  "delivery_id": "uuid",
  "attempt_id": "uuid",
  "client_id": "uuid",
  "channel_type": "flash_call",
  "billable": true,
  "idempotency_key": "attempt_id",
  "request_id": "uuid"
}
```

---

## Domain State Machine (Go)

```go
// domain/delivery.go
type DeliveryStatus string
const (
    DeliveryPending    DeliveryStatus = "pending"
    DeliveryInProgress DeliveryStatus = "in_progress"
    DeliveryDelivered  DeliveryStatus = "delivered"
    DeliveryFailed     DeliveryStatus = "failed"
    DeliveryCancelled  DeliveryStatus = "cancelled"
)

// Valid transitions:
// Pending     → InProgress (first attempt created)
// InProgress  → Delivered  (any attempt delivered)
// InProgress  → InProgress (next step triggered)
// InProgress  → Failed     (all steps exhausted)
// InProgress  → Cancelled  (admin disabled all channels)

type AttemptStatus string
const (
    AttemptPending       AttemptStatus = "pending"
    AttemptSent          AttemptStatus = "sent"
    AttemptDelivered     AttemptStatus = "delivered"
    AttemptFailed        AttemptStatus = "failed"
    AttemptTimeout       AttemptStatus = "timeout"
    AttemptSkipped       AttemptStatus = "skipped"
    AttemptLateDuplicate AttemptStatus = "late_duplicate"
)
```
