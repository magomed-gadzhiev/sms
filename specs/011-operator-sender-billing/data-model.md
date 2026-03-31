# Data Model: Operator Sender Name Billing

**Feature**: 011-operator-sender-billing  
**Date**: 2026-03-31

---

## 1. Изменение: таблица `operators`

**Migration**: `000065_operator_tariff.up.sql`

```sql
ALTER TABLE operators
  ADD COLUMN monthly_tariff_amount NUMERIC(20,6) DEFAULT NULL;
```

**Down**:
```sql
ALTER TABLE operators DROP COLUMN IF EXISTS monthly_tariff_amount;
```

**Правила**:
- `monthly_tariff_amount` = NULL для операторов с `supports_paid_sender = false` (стратегия "free-only")
- `monthly_tariff_amount` > 0 для операторов с `supports_paid_sender = true` (стратегия "free-and-paid")
- Валюта: RUB (единственная валюта платформы)

**Go domain model** (изменение `internal/services/routing/domain/operator.go`):
```go
type Operator struct {
    // ... существующие поля ...
    MonthlyTariffAmount *decimal.Decimal // nil для free-only операторов
}
```

---

## 2. Новая таблица: `sender_name_billing_records`

**Migration**: `000066_sender_name_billing.up.sql`

```sql
CREATE TABLE sender_name_billing_records (
    id                    UUID         NOT NULL DEFAULT uuid_generate_v4(),
    sender_registration_id UUID        NOT NULL REFERENCES sender_registrations(id),
    client_id             UUID         NOT NULL REFERENCES clients(id),
    operator_id           UUID         NOT NULL REFERENCES operators(id),
    billing_month         DATE         NOT NULL, -- первое число месяца (e.g. 2026-04-01)
    amount                NUMERIC(20,6) NOT NULL CHECK (amount > 0),
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    PRIMARY KEY (id),
    UNIQUE (sender_registration_id, billing_month) -- идемпотентность cron-планировщика
);

CREATE INDEX idx_snbr_client_id   ON sender_name_billing_records(client_id);
CREATE INDEX idx_snbr_operator_id ON sender_name_billing_records(operator_id);
CREATE INDEX idx_snbr_month       ON sender_name_billing_records(billing_month);
CREATE INDEX idx_snbr_reg_month   ON sender_name_billing_records(sender_registration_id, billing_month);
```

**Down**:
```sql
DROP TABLE IF EXISTS sender_name_billing_records;
```

**Поля**:

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | UUID | PK |
| `sender_registration_id` | UUID | FK → sender_registrations(id) |
| `client_id` | UUID | Денормализовано для быстрого запроса по клиенту |
| `operator_id` | UUID | Денормализовано; тариф зафиксирован в `amount` |
| `billing_month` | DATE | Первое число расчётного месяца (e.g. 2026-04-01) |
| `amount` | NUMERIC(20,6) | Сумма начисления в RUB на момент выставления счёта |
| `created_at` | TIMESTAMPTZ | Время создания записи |

**Примечания**:
- Таблица не партиционируется — ожидаемый объём <10 000 записей/мес, партиционирование избыточно
- `amount` фиксирует тариф оператора на момент начисления (FR-006); изменение тарифа не влияет на ранее выставленные записи (SC-005)
- UNIQUE(sender_registration_id, billing_month) обеспечивает идемпотентность: повторный запуск планировщика → INSERT ... ON CONFLICT DO NOTHING

---

## 3. Go domain model: `SenderNameBillingRecord`

**Файл**: `internal/services/tarification/domain/sender_billing.go`

```go
package domain

import (
    "time"
    "github.com/google/uuid"
    "github.com/shopspring/decimal"
)

type SenderNameBillingRecord struct {
    ID                   uuid.UUID
    SenderRegistrationID uuid.UUID
    ClientID             uuid.UUID
    OperatorID           uuid.UUID
    BillingMonth         time.Time       // первое число месяца, UTC
    Amount               decimal.Decimal // в RUB
    CreatedAt            time.Time
}

// BillingMonthKey возвращает первое число месяца для t в UTC.
func BillingMonthKey(t time.Time) time.Time {
    return time.Date(t.UTC().Year(), t.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
}
```

**Интерфейс репозитория** (добавить в `internal/services/tarification/domain/interfaces.go`):

```go
type SenderBillingRepository interface {
    // Create создаёт billing record; при дубликате (reg_id + month) возвращает ErrBillingRecordExists
    Create(ctx context.Context, record *SenderNameBillingRecord) error
    // CreateIfNotExists — INSERT ON CONFLICT DO NOTHING; возвращает (created bool, err error)
    CreateIfNotExists(ctx context.Context, record *SenderNameBillingRecord) (bool, error)
    // ListByRegistration возвращает все записи по регистрации, DESC по billing_month
    ListByRegistration(ctx context.Context, regID uuid.UUID, limit, offset int) ([]*SenderNameBillingRecord, int, error)
    // ListByClient возвращает все записи по клиенту за период
    ListByClient(ctx context.Context, clientID uuid.UUID, from, to time.Time, limit, offset int) ([]*SenderNameBillingRecord, int, error)
}
```

---

## 4. Связи между сущностями

```
operators (routing-service)
  ├── monthly_tariff_amount   ← НОВОЕ ПОЛЕ
  └─── 1:N sender_registrations (tarification-service)
              └─── 1:N sender_name_billing_records   ← НОВАЯ ТАБЛИЦА
```

**Поток создания billing record**:

```
Client (portal) → portal-gateway
  → tarification-service gRPC: CreateSenderBillingRecord(reg_id, amount)
    → sender_name_billing_records INSERT

Monthly cron (tarification-service)
  → SELECT sender_registrations WHERE type='paid' AND status='active'
  → FOR EACH reg:
      GET operator.monthly_tariff_amount (cached или gRPC GetOperator)
      → sender_name_billing_records INSERT ON CONFLICT DO NOTHING
```

---

## 5. Обновлённый Operator domain (routing-service)

```go
// internal/services/routing/domain/operator.go
type Operator struct {
    ID                  uuid.UUID
    CountryID           uuid.UUID
    Name                string
    Code                string
    SupportsPaidSender  bool
    SupportsFreeSender  bool
    MonthlyTariffAmount *decimal.Decimal // nil = free-only
    Active              bool
    CreatedAt           time.Time
    UpdatedAt           time.Time
}

// HasPaidRegistration возвращает true если оператор поддерживает платную регистрацию
func (o *Operator) HasPaidRegistration() bool {
    return o.SupportsPaidSender && o.MonthlyTariffAmount != nil
}
```

---

## Примечания к библиотеке decimal

Если `shopspring/decimal` не используется в codebase — использовать `float64` не следует (финансовые расчёты).  
Проверить: `grep -r "shopspring" go.mod`. Если отсутствует — хранить как `int64` (копейки/центы RUB × 10000) или использовать `pgx` тип `pgtype.Numeric` и маппить в `decimal.Decimal`.  
Альтернатива: хранить и передавать как строку `"1500.000000"`, парсить на клиенте — простейший вариант без новой зависимости.
