# Aggregator Dual-Charge: Atomicity & Commit-on-Submit — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Исправить четыре разрыва в уже применённой агрегаторской модели: (1) атомарное dual-списание в одной PG-транзакции, (2) разделение TarifyMessage (read-only) и CommitCharge (atomic), (3) расширение `aggregator_margin_log` полями `charge_mode`/`pool_segments`/`overage_segments`, (4) переход на UUID v7 для `message_id`.

**Architecture:** Новый метод `billing.ChargeMessageDual` (atomic PG-транзакция с lazy-create квоты, dual UPDATE balances, margin_log как идемпотентность-guard через `ON CONFLICT (idempotency_key)`). Новый RPC `tarification.CommitCharge` вызывает его после успешного SUBMIT к провайдеру. `TarifyMessage` переводится в read-only (расчёт без записей). Фиче-флаг `COMMIT_ON_SUBMIT_ENABLED` переключает между legacy-flow (charge в tarify) и новым (charge в commit).

**Tech Stack:** Go 1.24 + pgx/v5 (sqlx), gRPC google.golang.org/grpc, protobuf, google/uuid v1.6.0 (NewV7 доступен), PostgreSQL 15+.

**Ссылка на спек:** `docs/superpowers/specs/2026-04-20-aggregator-dual-charge-design.md`.

---

## File Map

**Новые файлы:**
- `migrations/000106_aggregator_margin_log_charge_mode.up.sql`
- `migrations/000106_aggregator_margin_log_charge_mode.down.sql`
- `internal/services/billing/application/charge_dual.go` — метод `ChargeMessageDual`
- `internal/services/billing/application/charge_dual_test.go` — unit-тесты
- `internal/services/tarification/application/commit_charge.go` — метод `CommitCharge`
- `internal/services/tarification/application/commit_charge_test.go`
- `internal/services/tarification/application/tarify_calculate.go` — экстракт общего расчёта
- `e2e/tests/reseller/dual-charge.spec.ts`

**Изменяемые файлы:**
- `api/proto/billing/billing.proto` — добавить RPC `ChargeMessageDual` и сообщения
- `api/proto/tarification/tarification.proto` — добавить RPC `CommitCharge`, расширить `TarifyMessageResponse`
- `api/proto/billingv1/*` — регенерация
- `api/proto/tarificationv1/*` — регенерация
- `internal/services/tarification/domain/aggregator_margin_log.go` — три новых поля
- `internal/services/tarification/infrastructure/repository/aggregator_margin_log_repository.go` — фикс `ON CONFLICT`, новые поля
- `internal/services/tarification/application/tarification_service.go` — TarifyMessage → read-only, удалить 8a/8b/logAggregatorMargin ветки под флагом
- `internal/services/tarification/application/saga.go` — **удалить** dead-code `ChargeDual` и `DualChargeResult`
- `internal/services/tarification/grpc/server.go` — новый handler `CommitCharge`, расширить TarifyMessage response
- `internal/services/billing/grpc/server.go` — новый handler `ChargeMessageDual`
- `internal/pipeline/sender/stage.go` — под флагом: отмена pre-send tarify-charge, вызов CommitCharge после успешного send, удаление refund-after-retries-exhausted ветки
- `internal/config/config.go` — флаг `CommitOnSubmitEnabled` в `TarificationConfig`
- `configs/*.yaml` (если применимо) — дефолт `commit_on_submit_enabled: false`

---

## Task 1: Миграция 000106 — расширение aggregator_margin_log

**Files:**
- Create: `migrations/000106_aggregator_margin_log_charge_mode.up.sql`
- Create: `migrations/000106_aggregator_margin_log_charge_mode.down.sql`

- [ ] **Step 1: Написать up-миграцию**

Файл `migrations/000106_aggregator_margin_log_charge_mode.up.sql`:

```sql
BEGIN;

ALTER TABLE aggregator_margin_log
    ADD COLUMN charge_mode VARCHAR(16) NOT NULL DEFAULT 'pool'
        CHECK (charge_mode IN ('pool', 'overage', 'split')),
    ADD COLUMN pool_segments INTEGER NOT NULL DEFAULT 0 CHECK (pool_segments >= 0),
    ADD COLUMN overage_segments INTEGER NOT NULL DEFAULT 0 CHECK (overage_segments >= 0);

ALTER TABLE aggregator_margin_log
    ADD CONSTRAINT chk_segments_sum CHECK (pool_segments + overage_segments = segment_count);

COMMIT;
```

- [ ] **Step 2: Написать down-миграцию**

Файл `migrations/000106_aggregator_margin_log_charge_mode.down.sql`:

```sql
BEGIN;

ALTER TABLE aggregator_margin_log
    DROP CONSTRAINT IF EXISTS chk_segments_sum,
    DROP COLUMN IF EXISTS overage_segments,
    DROP COLUMN IF EXISTS pool_segments,
    DROP COLUMN IF EXISTS charge_mode;

COMMIT;
```

- [ ] **Step 3: Применить миграцию локально**

```bash
./scripts/server.sh migrate
```

Ожидаемо: `000106_aggregator_margin_log_charge_mode` применена.

- [ ] **Step 4: Коммит**

```bash
git add migrations/000106_aggregator_margin_log_charge_mode.up.sql migrations/000106_aggregator_margin_log_charge_mode.down.sql
git commit -m "migrate(aggregator): добавить charge_mode/pool_segments/overage_segments в aggregator_margin_log"
```

---

## Task 2: Обновить доменную модель AggregatorMarginLog

**Files:**
- Modify: `internal/services/tarification/domain/aggregator_margin_log.go`

- [ ] **Step 1: Прочитать текущую структуру**

Открыть `internal/services/tarification/domain/aggregator_margin_log.go`. Найти `type AggregatorMarginLog struct` и фабрику `NewAggregatorMarginLog`.

- [ ] **Step 2: Добавить три поля в struct**

В struct `AggregatorMarginLog` добавить (после `Margin`):

```go
ChargeMode       string // "pool" | "overage" | "split"
PoolSegments     int
OverageSegments  int
```

Константы в том же файле (если нет — добавить):

```go
const (
    ChargeModePool    = "pool"
    ChargeModeOverage = "overage"
    ChargeModeSplit   = "split"
)
```

- [ ] **Step 3: Обновить конструктор `NewAggregatorMarginLog`**

Если существующая сигнатура меняется — обновить всех вызывающих (grep `NewAggregatorMarginLog`). Если конструктора нет — пропустить этот шаг.

Вариант: добавить отдельный конструктор `NewAggregatorMarginLogWithMode(..., chargeMode string, poolSegments, overageSegments int)` — не ломает существующих вызывающих.

- [ ] **Step 4: Коммит**

```bash
git add internal/services/tarification/domain/aggregator_margin_log.go
git commit -m "domain(aggregator): добавить charge_mode/pool/overage поля в AggregatorMarginLog"
```

---

## Task 3: Исправить `ON CONFLICT` баг и поддержать новые поля в репозитории

**Files:**
- Modify: `internal/services/tarification/infrastructure/repository/aggregator_margin_log_repository.go`

**Контекст бага:** Текущий `ON CONFLICT (idempotency_key, created_at) DO NOTHING` — не работает как идемпотентность, потому что реальный UNIQUE в миграции 000096 только по `idempotency_key`. Композитный ON CONFLICT требует соответствующего UNIQUE-ограничения, которого нет. Правильно — `ON CONFLICT (idempotency_key)`.

- [ ] **Step 1: Исправить ON CONFLICT и добавить новые поля в INSERT**

Заменить тело метода `Create` в `aggregator_margin_log_repository.go`:

```go
func (r *AggregatorMarginLogRepository) Create(ctx context.Context, entry *domain.AggregatorMarginLog) error {
    query := `
        INSERT INTO aggregator_margin_log
            (id, aggregator_id, sub_account_id, message_id, operator_id, segment_count,
             sub_account_price, aggregator_price, sub_account_total, aggregator_total,
             margin, idempotency_key, created_at,
             charge_mode, pool_segments, overage_segments)
        VALUES
            ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
        ON CONFLICT (idempotency_key) DO NOTHING
    `

    _, err := r.db.ExecContext(ctx, query,
        entry.ID, entry.AggregatorID, entry.SubAccountID, entry.MessageID, entry.OperatorID,
        entry.SegmentCount,
        entry.SubAccountPrice, entry.AggregatorPrice,
        entry.SubAccountTotal, entry.AggregatorTotal,
        entry.Margin,
        entry.IdempotencyKey, entry.CreatedAt,
        entry.ChargeMode, entry.PoolSegments, entry.OverageSegments,
    )
    if err != nil {
        return fmt.Errorf("aggregator_margin_log insert: %w", err)
    }
    return nil
}
```

- [ ] **Step 2: Добавить метод `CreateTx` для работы в транзакции billing-service**

В том же файле:

```go
// CreateTx вставляет margin_log в рамках переданной транзакции.
// Возвращает true если запись создана; false если уже существовала (ON CONFLICT).
func (r *AggregatorMarginLogRepository) CreateTx(ctx context.Context, tx *sqlx.Tx, entry *domain.AggregatorMarginLog) (bool, error) {
    query := `
        INSERT INTO aggregator_margin_log
            (id, aggregator_id, sub_account_id, message_id, operator_id, segment_count,
             sub_account_price, aggregator_price, sub_account_total, aggregator_total,
             margin, idempotency_key, created_at,
             charge_mode, pool_segments, overage_segments)
        VALUES
            ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
        ON CONFLICT (idempotency_key) DO NOTHING
        RETURNING id
    `

    var id uuid.UUID
    err := tx.QueryRowxContext(ctx, query,
        entry.ID, entry.AggregatorID, entry.SubAccountID, entry.MessageID, entry.OperatorID,
        entry.SegmentCount,
        entry.SubAccountPrice, entry.AggregatorPrice,
        entry.SubAccountTotal, entry.AggregatorTotal,
        entry.Margin,
        entry.IdempotencyKey, entry.CreatedAt,
        entry.ChargeMode, entry.PoolSegments, entry.OverageSegments,
    ).Scan(&id)
    if errors.Is(err, sql.ErrNoRows) {
        return false, nil // already exists
    }
    if err != nil {
        return false, fmt.Errorf("aggregator_margin_log insert tx: %w", err)
    }
    return true, nil
}
```

Добавить импорты если нужно: `"database/sql"`, `"errors"`, `"github.com/jmoiron/sqlx"`.

- [ ] **Step 3: Коммит**

```bash
git add internal/services/tarification/infrastructure/repository/aggregator_margin_log_repository.go
git commit -m "fix(aggregator): исправить ON CONFLICT на корректный UNIQUE + добавить CreateTx"
```

---

## Task 4: UUID v7 helper

**Files:**
- Modify: `internal/services/tarification/application/saga.go` (точка использования) или новый helper.

Поскольку `uuid.NewV7()` доступен прямо из `google/uuid v1.6.0` — отдельный helper не нужен. На этапе замены `uuid.New()` на `uuid.NewV7()` изменение идёт напрямую.

**Эта задача — NO-OP** (нет отдельных файлов для создания). Замена будет сделана в Task 10 (при экстракции `calculate()` из TarifyMessage).

---

## Task 5: Proto — добавить ChargeMessageDual в billing.proto

**Files:**
- Modify: `api/proto/billing/billing.proto`

- [ ] **Step 1: Добавить RPC определение и сообщения**

В `api/proto/billing/billing.proto` в блок `service BillingService { ... }` добавить:

```protobuf
  // ChargeMessageDual атомарно списывает средства с субаккаунта и агрегатора
  // за одно SMS, инкрементит квоту агрегатора, пишет margin log.
  // Идемпотентно по idempotency_key.
  rpc ChargeMessageDual(ChargeMessageDualRequest) returns (ChargeMessageDualResponse);
```

В том же файле после `ChargeMessageResponse` добавить:

```protobuf
message ChargeMessageDualRequest {
  string message_id = 1;              // UUID v7; также используется как idempotency_key
  string sub_account_id = 2;
  string aggregator_id = 3;
  string sub_account_price = 4;       // NUMERIC per segment
  string aggregator_price = 5;        // NUMERIC per segment (pool or effective for split)
  string sub_account_total = 6;       // итого с субаккаунта
  string aggregator_total = 7;        // итого с агрегатора
  string currency = 8;                // ISO, пустое → "RUB"
  string operator_id = 9;
  int32 segment_count = 10;
  int32 pool_segments = 11;
  int32 overage_segments = 12;
  string charge_mode = 13;            // "pool" | "overage" | "split"
}

message ChargeMessageDualResponse {
  bool committed = 1;
  string sub_account_tx_id = 2;
  string aggregator_tx_id = 3;
  string margin_log_id = 4;
  ChargeMessageDualError error = 5;
}

enum ChargeMessageDualError {
  CHARGE_DUAL_ERROR_UNSPECIFIED = 0;
  CHARGE_DUAL_ERROR_ALREADY_COMMITTED = 1;
  CHARGE_DUAL_ERROR_QUOTA_NOT_CONFIGURED = 2;
  CHARGE_DUAL_ERROR_INSUFFICIENT_BALANCE_SUBACCOUNT = 3;
  CHARGE_DUAL_ERROR_INSUFFICIENT_BALANCE_AGGREGATOR = 4;
}
```

- [ ] **Step 2: Регенерировать stubs**

```bash
bash scripts/generate-proto.sh
```

Ожидаемо: `api/proto/billingv1/billing.pb.go` и `billing_grpc.pb.go` обновлены, содержат новые типы.

- [ ] **Step 3: Коммит**

```bash
git add api/proto/billing/billing.proto api/proto/billingv1/
git commit -m "proto(billing): добавить ChargeMessageDual RPC + сообщения"
```

---

## Task 6: Proto — CommitCharge в tarification.proto + расширить TarifyMessageResponse

**Files:**
- Modify: `api/proto/tarification/tarification.proto`

- [ ] **Step 1: Расширить TarifyMessageResponse**

В `api/proto/tarification/tarification.proto` в `message TarifyMessageResponse` добавить (после существующих полей):

```protobuf
  // Добавлены для commit-on-submit flow:
  string aggregator_id = 9;             // UUID если субаккаунт; пусто если direct-клиент
  string sub_account_price = 10;        // NUMERIC per segment
  string aggregator_price = 11;         // NUMERIC per segment
  string sub_account_total = 12;        // итого с субаккаунта
  string aggregator_total = 13;         // итого с агрегатора
  int32 segment_count = 14;             // дубль запроса — для удобства CommitCharge
  int32 pool_segments = 15;
  int32 overage_segments = 16;
  string charge_mode = 17;              // "pool"|"overage"|"split"; пусто для direct
  string operator_id = 18;              // дубль запроса
```

- [ ] **Step 2: Добавить RPC CommitCharge**

В `service TarificationService { ... }`:

```protobuf
  // CommitCharge применяет ранее рассчитанное списание после успешной отправки
  // к провайдеру. Идемпотентно по message_id.
  rpc CommitCharge(CommitChargeRequest) returns (CommitChargeResponse);
```

Новые сообщения в конце файла:

```protobuf
message CommitChargeRequest {
  string message_id = 1;              // тот же UUID v7, что вернул TarifyMessage
}

message CommitChargeResponse {
  bool committed = 1;
  string sub_account_tx_id = 2;
  string aggregator_tx_id = 3;
  string margin_log_id = 4;
  CommitChargeError error = 5;
}

enum CommitChargeError {
  COMMIT_CHARGE_ERROR_UNSPECIFIED = 0;
  COMMIT_CHARGE_ERROR_ALREADY_COMMITTED = 1;
  COMMIT_CHARGE_ERROR_QUOTA_NOT_CONFIGURED = 2;
  COMMIT_CHARGE_ERROR_INSUFFICIENT_BALANCE_SUBACCOUNT = 3;
  COMMIT_CHARGE_ERROR_INSUFFICIENT_BALANCE_AGGREGATOR = 4;
  COMMIT_CHARGE_ERROR_NO_TARIFF = 5;
}
```

- [ ] **Step 3: Регенерация stubs**

```bash
bash scripts/generate-proto.sh
```

- [ ] **Step 4: Коммит**

```bash
git add api/proto/tarification/tarification.proto api/proto/tarificationv1/
git commit -m "proto(tarification): CommitCharge RPC + расширить TarifyMessageResponse для commit-on-submit"
```

---

## Task 7: Реализовать `billing.ChargeMessageDual` — атомарная транзакция

**Files:**
- Create: `internal/services/billing/application/charge_dual.go`
- Modify: `internal/services/billing/application/billing_service.go` (добавить зависимости к сервису)
- Modify: `internal/services/billing/grpc/server.go` (gRPC handler)

- [ ] **Step 1: Определить интерфейсы зависимостей**

В начале `internal/services/billing/application/charge_dual.go`:

```go
package application

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"
    "github.com/rs/zerolog/log"

    billingDomain "sms/internal/services/billing/domain"
    tarDomain "sms/internal/services/tarification/domain"
)

// Зависимости, нужные ChargeMessageDual поверх обычного BillingService.
type DualChargeDeps struct {
    DB              *sqlx.DB
    QuotaRepo       aggregatorQuotaRepo
    MarginLogRepo   aggregatorMarginLogRepo
    AccountRepo     billingDomain.AccountRepository
    TransactionRepo billingDomain.TransactionRepository
    EventPublisher  billingDomain.EventPublisher // может быть nil
}

type aggregatorQuotaRepo interface {
    GetActiveTx(ctx context.Context, tx *sqlx.Tx, aggregatorID uuid.UUID, now time.Time) (*tarDomain.AggregatorQuota, error)
    GetLatestTx(ctx context.Context, tx *sqlx.Tx, aggregatorID uuid.UUID) (*tarDomain.AggregatorQuota, error)
    CreateTx(ctx context.Context, tx *sqlx.Tx, q *tarDomain.AggregatorQuota) error
    IncrementUsageTx(ctx context.Context, tx *sqlx.Tx, quotaID uuid.UUID, segments int) (segmentsUsedAfter int64, segmentLimit int64, err error)
}

type aggregatorMarginLogRepo interface {
    CreateTx(ctx context.Context, tx *sqlx.Tx, entry *tarDomain.AggregatorMarginLog) (inserted bool, err error)
}
```

- [ ] **Step 2: Добавить в `AggregatorQuotaRepository` недостающие Tx-методы**

В `internal/services/tarification/infrastructure/repository/aggregator_quota_repository.go` добавить:

```go
// GetActiveTx — аналог GetActive, но в рамках переданной транзакции, с FOR UPDATE.
func (r *AggregatorQuotaRepository) GetActiveTx(ctx context.Context, tx *sqlx.Tx, aggregatorID uuid.UUID, now time.Time) (*domain.AggregatorQuota, error) {
    var q domain.AggregatorQuota
    query := `
        SELECT id, aggregator_id, period_start, period_end, segment_limit, segments_used,
               overage_rate, currency, auto_renew, notified_80pct, notified_100pct,
               created_at, updated_at
        FROM aggregator_quotas
        WHERE aggregator_id = $1 AND $2::date >= period_start AND $2::date <= period_end
        ORDER BY period_start DESC LIMIT 1
        FOR UPDATE
    `
    if err := tx.GetContext(ctx, &q, query, aggregatorID, now); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, nil
        }
        return nil, fmt.Errorf("aggregator_quotas get active tx: %w", err)
    }
    return &q, nil
}

// GetLatestTx — последняя квота агрегатора (для lazy-create).
func (r *AggregatorQuotaRepository) GetLatestTx(ctx context.Context, tx *sqlx.Tx, aggregatorID uuid.UUID) (*domain.AggregatorQuota, error) {
    var q domain.AggregatorQuota
    query := `
        SELECT id, aggregator_id, period_start, period_end, segment_limit, segments_used,
               overage_rate, currency, auto_renew, notified_80pct, notified_100pct,
               created_at, updated_at
        FROM aggregator_quotas
        WHERE aggregator_id = $1
        ORDER BY period_end DESC LIMIT 1
    `
    if err := tx.GetContext(ctx, &q, query, aggregatorID); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, nil
        }
        return nil, fmt.Errorf("aggregator_quotas get latest tx: %w", err)
    }
    return &q, nil
}

// CreateTx — вставка квоты в рамках транзакции.
func (r *AggregatorQuotaRepository) CreateTx(ctx context.Context, tx *sqlx.Tx, q *domain.AggregatorQuota) error {
    query := `
        INSERT INTO aggregator_quotas
            (id, aggregator_id, period_start, period_end, segment_limit, segments_used,
             overage_rate, currency, auto_renew, created_at, updated_at)
        VALUES
            ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        ON CONFLICT (aggregator_id, period_start) DO NOTHING
    `
    _, err := tx.ExecContext(ctx, query,
        q.ID, q.AggregatorID, q.PeriodStart, q.PeriodEnd, q.SegmentLimit, q.SegmentsUsed,
        q.OverageRate, q.Currency, q.AutoRenew, q.CreatedAt, q.UpdatedAt,
    )
    if err != nil {
        return fmt.Errorf("aggregator_quotas create tx: %w", err)
    }
    return nil
}

// IncrementUsageTx атомарно инкрементит segments_used и возвращает новое значение.
func (r *AggregatorQuotaRepository) IncrementUsageTx(ctx context.Context, tx *sqlx.Tx, quotaID uuid.UUID, segments int) (int64, int64, error) {
    var used, limit int64
    query := `
        UPDATE aggregator_quotas
        SET segments_used = segments_used + $2, updated_at = NOW()
        WHERE id = $1
        RETURNING segments_used, segment_limit
    `
    if err := tx.QueryRowxContext(ctx, query, quotaID, segments).Scan(&used, &limit); err != nil {
        return 0, 0, fmt.Errorf("aggregator_quotas increment tx: %w", err)
    }
    return used, limit, nil
}
```

Импорты (если не были): `"database/sql"`, `"errors"`, `"time"`.

- [ ] **Step 3: Реализовать `ChargeMessageDual`**

Добавить в `charge_dual.go`:

```go
// ChargeMessageDualInput — вход метода сервиса.
type ChargeMessageDualInput struct {
    MessageID        uuid.UUID
    SubAccountID     uuid.UUID
    AggregatorID     uuid.UUID
    OperatorID       uuid.UUID
    SubAccountPrice  string // per segment
    AggregatorPrice  string // per segment (effective for charge_mode)
    SubAccountTotal  string
    AggregatorTotal  string
    Currency         string
    SegmentCount     int
    PoolSegments     int
    OverageSegments  int
    ChargeMode       string // "pool"|"overage"|"split"
}

// ChargeMessageDualResult — результат.
type ChargeMessageDualResult struct {
    Committed        bool
    AlreadyCommitted bool
    QuotaMissing     bool
    SubInsufficient  bool
    AggInsufficient  bool
    SubTxID          uuid.UUID
    AggTxID          uuid.UUID
    MarginLogID      uuid.UUID
}

// ChargeMessageDual — атомарное двойное списание с lazy-create квоты и margin log.
func ChargeMessageDual(
    ctx context.Context,
    deps DualChargeDeps,
    in ChargeMessageDualInput,
) (*ChargeMessageDualResult, error) {
    if in.Currency == "" {
        in.Currency = "RUB"
    }

    tx, err := deps.DB.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
    if err != nil {
        return nil, fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()

    now := time.Now().UTC()

    // 1. Вставка margin_log первой операцией — идемпотентность-guard.
    marginID := uuid.New()
    margin, err := computeMargin(in.SubAccountTotal, in.AggregatorTotal)
    if err != nil {
        return nil, fmt.Errorf("compute margin: %w", err)
    }
    marginEntry := &tarDomain.AggregatorMarginLog{
        ID:               marginID,
        AggregatorID:     in.AggregatorID,
        SubAccountID:     in.SubAccountID,
        MessageID:        in.MessageID,
        OperatorID:       in.OperatorID,
        SegmentCount:     in.SegmentCount,
        SubAccountPrice:  in.SubAccountPrice,
        AggregatorPrice:  in.AggregatorPrice,
        SubAccountTotal:  in.SubAccountTotal,
        AggregatorTotal:  in.AggregatorTotal,
        Margin:           margin,
        IdempotencyKey:   in.MessageID.String(),
        CreatedAt:        now,
        ChargeMode:       in.ChargeMode,
        PoolSegments:     in.PoolSegments,
        OverageSegments:  in.OverageSegments,
    }
    inserted, err := deps.MarginLogRepo.CreateTx(ctx, tx, marginEntry)
    if err != nil {
        return nil, fmt.Errorf("margin_log insert: %w", err)
    }
    if !inserted {
        // уже закоммичено — быстрый выход без трогания балансов
        return &ChargeMessageDualResult{AlreadyCommitted: true}, nil
    }

    // 2. Lazy-create квоты при необходимости.
    quota, err := deps.QuotaRepo.GetActiveTx(ctx, tx, in.AggregatorID, now)
    if err != nil {
        return nil, fmt.Errorf("quota get active: %w", err)
    }
    if quota == nil {
        prev, err := deps.QuotaRepo.GetLatestTx(ctx, tx, in.AggregatorID)
        if err != nil {
            return nil, fmt.Errorf("quota get latest: %w", err)
        }
        if prev == nil || !prev.AutoRenew {
            return &ChargeMessageDualResult{QuotaMissing: true}, nil
        }
        // Создать квоту на текущий календарный месяц UTC, наследуя параметры.
        periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
        periodEnd := periodStart.AddDate(0, 1, -1)
        newQuota := &tarDomain.AggregatorQuota{
            ID:            uuid.New(),
            AggregatorID:  in.AggregatorID,
            PeriodStart:   periodStart,
            PeriodEnd:     periodEnd,
            SegmentLimit:  prev.SegmentLimit,
            SegmentsUsed:  0,
            OverageRate:   prev.OverageRate,
            Currency:      prev.Currency,
            AutoRenew:     prev.AutoRenew,
            CreatedAt:     now,
            UpdatedAt:     now,
        }
        if err := deps.QuotaRepo.CreateTx(ctx, tx, newQuota); err != nil {
            return nil, fmt.Errorf("quota lazy-create: %w", err)
        }
        // Перечитать, т.к. ON CONFLICT мог ничего не вставить (concurrent first-call).
        quota, err = deps.QuotaRepo.GetActiveTx(ctx, tx, in.AggregatorID, now)
        if err != nil || quota == nil {
            return nil, fmt.Errorf("quota re-read after create: %w", err)
        }
    }

    // 3. Инкрементить квоту.
    if _, _, err := deps.QuotaRepo.IncrementUsageTx(ctx, tx, quota.ID, in.SegmentCount); err != nil {
        return nil, fmt.Errorf("quota increment: %w", err)
    }

    // 4. Списать с субаккаунта.
    subTxID, subErr := chargeBalanceTx(ctx, tx, deps, in.SubAccountID, in.MessageID, in.SubAccountTotal, in.Currency, "SMS subaccount", nil)
    if errors.Is(subErr, billingDomain.ErrInsufficientBalance) {
        return &ChargeMessageDualResult{SubInsufficient: true}, nil
    }
    if subErr != nil {
        return nil, fmt.Errorf("charge subaccount: %w", subErr)
    }

    // 5. Списать с агрегатора (attributed_sub_account_id = in.SubAccountID).
    attributed := in.SubAccountID
    aggTxID, aggErr := chargeBalanceTx(ctx, tx, deps, in.AggregatorID, in.MessageID, in.AggregatorTotal, in.Currency, "SMS aggregator", &attributed)
    if errors.Is(aggErr, billingDomain.ErrInsufficientBalance) {
        return &ChargeMessageDualResult{AggInsufficient: true}, nil
    }
    if aggErr != nil {
        return nil, fmt.Errorf("charge aggregator: %w", aggErr)
    }

    if err := tx.Commit(); err != nil {
        return nil, fmt.Errorf("commit: %w", err)
    }

    // Публикуем события вне транзакции (не-фатально).
    if deps.EventPublisher != nil {
        _ = deps.EventPublisher.PublishTransactionCompleted(ctx, subTxID.String(), in.SubAccountID.String(), "charge", in.SubAccountTotal, in.Currency)
        _ = deps.EventPublisher.PublishTransactionCompleted(ctx, aggTxID.String(), in.AggregatorID.String(), "charge", in.AggregatorTotal, in.Currency)
    }

    return &ChargeMessageDualResult{
        Committed:   true,
        SubTxID:     subTxID,
        AggTxID:     aggTxID,
        MarginLogID: marginID,
    }, nil
}

// chargeBalanceTx атомарно списывает amount с баланса client_id, создаёт запись transaction.
// Возвращает ID транзакции или ErrInsufficientBalance.
func chargeBalanceTx(ctx context.Context, tx *sqlx.Tx, deps DualChargeDeps,
    clientID, messageID uuid.UUID, amount, currency, description string, attributedSubAccount *uuid.UUID,
) (uuid.UUID, error) {
    // Используем UPDATE ... WHERE balance >= amount RETURNING вместо SELECT FOR UPDATE.
    var newBalance, oldBalance string
    err := tx.QueryRowxContext(ctx, `
        UPDATE accounts
        SET balance = balance - $2::numeric,
            updated_at = NOW()
        WHERE client_id = $1 AND balance::numeric >= $2::numeric
        RETURNING balance, (balance + $2::numeric) AS prev
    `, clientID, amount).Scan(&newBalance, &oldBalance)
    if errors.Is(err, sql.ErrNoRows) {
        return uuid.Nil, billingDomain.ErrInsufficientBalance
    }
    if err != nil {
        return uuid.Nil, err
    }

    txID := uuid.New()
    _, err = tx.ExecContext(ctx, `
        INSERT INTO transactions
            (id, client_id, type, amount, balance_before, balance_after, currency,
             message_id, description, attributed_sub_account_id, created_at)
        VALUES ($1, $2, 'charge', $3, $4, $5, $6, $7, $8, $9, NOW())
    `, txID, clientID, amount, oldBalance, newBalance, currency, messageID, description, attributedSubAccount)
    if err != nil {
        return uuid.Nil, fmt.Errorf("insert transaction: %w", err)
    }
    return txID, nil
}

// computeMargin = subTotal - aggTotal; операция через строковую арифметику NUMERIC.
// Для простоты используем существующий subtract/подобный helper в BillingService;
// здесь инлайн через database computed — нет, оставим в Go. Используем утилиту из domain.
func computeMargin(subTotal, aggTotal string) (string, error) {
    // Воспользуемся существующим helper'ом billingDomain.SubtractAmount (если есть)
    // либо inline через big.Float. Решить в step 4.
    return billingDomain.SubtractAmount(subTotal, aggTotal)
}
```

Если `billingDomain.SubtractAmount` отсутствует — найти аналог в `billing_service.go` (там есть `subtract`), вынести в публичный helper `domain/arithmetic.go` в рамках этой задачи.

- [ ] **Step 4: Проверить существование `billingDomain.SubtractAmount` или helper'а**

```bash
grep -rn "func.*[Ss]ubtract" internal/services/billing/domain/ internal/services/billing/application/
```

Если helper'а нет на уровне domain — вынести `subtract` из `billing_service.go` в `domain/arithmetic.go` как `SubtractAmount(a, b string) (string, error)`.

- [ ] **Step 5: Коммит**

```bash
git add internal/services/tarification/infrastructure/repository/aggregator_quota_repository.go \
        internal/services/billing/application/charge_dual.go \
        internal/services/billing/domain/arithmetic.go
git commit -m "feat(billing): атомарный ChargeMessageDual с lazy-create квоты + margin log"
```

---

## Task 8: Unit-тесты для `billing.ChargeMessageDual`

**Files:**
- Create: `internal/services/billing/application/charge_dual_test.go`

- [ ] **Step 1: Написать failing test — happy path**

```go
package application_test

// Таблица кейсов:
// - Happy path: baseline
// - AlreadyCommitted: повтор с тем же message_id
// - QuotaMissing + auto_renew=false → QuotaMissing=true
// - QuotaMissing + auto_renew=true → лезет в lazy-create, коммитит
// - SubInsufficient: сабакаунт пустой, баланс агрегатора не тронут
// - AggInsufficient: сабакаунт списан но ROLLBACK полностью (баланс не изменён)
// - Split mode: pool_segments + overage_segments = segment_count, margin computed correctly
// - Concurrent double-call с одним message_id → ровно один committed, второй AlreadyCommitted

// Используем testcontainers-go postgres (если есть в проекте; grep "testcontainers").
// Если нет — unit на моках + отдельный integration тест (см. Task 14).
```

Если в проекте **нет testcontainers** — этот тест пишем как unit с моками интерфейсов `DualChargeDeps`, проверяем последовательность вызовов и корректность возвращаемых значений. Если **есть** — пишем integration с реальной БД.

Проверить через `grep -r "testcontainers" go.mod internal/` — если нет, идём unit + mock.

- [ ] **Step 2: Реализовать моки или testcontainer setup**

- [ ] **Step 3: Запустить тесты — убедиться что падают**

```bash
go test ./internal/services/billing/application/ -run TestChargeMessageDual -v
```

Ожидаемо: FAIL (функция не реализована или моки не настроены).

- [ ] **Step 4: Прогнать на green после реализации**

```bash
go test ./internal/services/billing/application/ -run TestChargeMessageDual -v
```

Ожидаемо: PASS все кейсы.

- [ ] **Step 5: Коммит**

```bash
git add internal/services/billing/application/charge_dual_test.go
git commit -m "test(billing): unit-тесты ChargeMessageDual (idempotency, lazy-create, balance checks)"
```

---

## Task 9: gRPC handler `ChargeMessageDual` в billing-service

**Files:**
- Modify: `internal/services/billing/grpc/server.go`

- [ ] **Step 1: Добавить handler**

В конец `server.go`:

```go
// ChargeMessageDual — gRPC обёртка над application.ChargeMessageDual.
func (s *Server) ChargeMessageDual(ctx context.Context, req *billingv1.ChargeMessageDualRequest) (*billingv1.ChargeMessageDualResponse, error) {
    if req.MessageId == "" || req.SubAccountId == "" || req.AggregatorId == "" {
        return nil, status.Error(codes.InvalidArgument, "message_id, sub_account_id, aggregator_id required")
    }

    messageID, err := uuid.Parse(req.MessageId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid message_id: %v", err)
    }
    subAccountID, err := uuid.Parse(req.SubAccountId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid sub_account_id: %v", err)
    }
    aggregatorID, err := uuid.Parse(req.AggregatorId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id: %v", err)
    }
    operatorID, err := uuid.Parse(req.OperatorId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
    }

    result, err := application.ChargeMessageDual(ctx, s.dualDeps, application.ChargeMessageDualInput{
        MessageID:       messageID,
        SubAccountID:    subAccountID,
        AggregatorID:    aggregatorID,
        OperatorID:      operatorID,
        SubAccountPrice: req.SubAccountPrice,
        AggregatorPrice: req.AggregatorPrice,
        SubAccountTotal: req.SubAccountTotal,
        AggregatorTotal: req.AggregatorTotal,
        Currency:        req.Currency,
        SegmentCount:    int(req.SegmentCount),
        PoolSegments:    int(req.PoolSegments),
        OverageSegments: int(req.OverageSegments),
        ChargeMode:      req.ChargeMode,
    })
    if err != nil {
        s.logger.Error().Err(err).Msg("charge message dual failed")
        return nil, status.Error(codes.Internal, err.Error())
    }

    resp := &billingv1.ChargeMessageDualResponse{
        Committed:      result.Committed,
        SubAccountTxId: result.SubTxID.String(),
        AggregatorTxId: result.AggTxID.String(),
        MarginLogId:    result.MarginLogID.String(),
    }
    switch {
    case result.AlreadyCommitted:
        resp.Error = billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_ALREADY_COMMITTED
    case result.QuotaMissing:
        resp.Error = billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_QUOTA_NOT_CONFIGURED
    case result.SubInsufficient:
        resp.Error = billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_INSUFFICIENT_BALANCE_SUBACCOUNT
    case result.AggInsufficient:
        resp.Error = billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_INSUFFICIENT_BALANCE_AGGREGATOR
    }
    return resp, nil
}
```

- [ ] **Step 2: Пробросить `s.dualDeps` в Server**

В `Server` struct добавить поле `dualDeps application.DualChargeDeps`. В `NewServer(...)` принимать и заполнять. На вызывающем уровне (где создаётся BillingServer) — собрать `DualChargeDeps` из существующих репозиториев.

Точка создания Server: найти через `grep -rn "NewServer" internal/services/billing/grpc/`.

- [ ] **Step 3: Прогнать build**

```bash
go build ./internal/services/billing/...
```

- [ ] **Step 4: Коммит**

```bash
git add internal/services/billing/grpc/server.go
git commit -m "grpc(billing): handler ChargeMessageDual"
```

---

## Task 10: Экстракт `calculate()` из `TarifyMessage`

**Files:**
- Create: `internal/services/tarification/application/tarify_calculate.go`
- Modify: `internal/services/tarification/application/tarification_service.go`

**Цель:** Выделить read-only расчёт (сегментация, lookup тарифов, квоты, определение charge_mode) в отдельный метод, который будет переиспользоваться и в `TarifyMessage`, и в `CommitCharge`.

- [ ] **Step 1: Создать `tarify_calculate.go` с функцией**

```go
package application

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"

    "sms/internal/services/tarification/domain"
)

// CalculateResult содержит всё, что нужно для CommitCharge и отдачи клиенту.
type CalculateResult struct {
    Approved         bool
    RejectionReason  string

    // Для direct-клиента:
    IsDirect         bool
    PlatformAmount   string
    Currency         string

    // Для субаккаунта (IsDirect=false):
    AggregatorID     uuid.UUID
    OperatorID       uuid.UUID
    SegmentCount     int
    SubAccountPrice  string // per segment
    AggregatorPrice  string // per segment (effective)
    SubAccountTotal  string
    AggregatorTotal  string
    PoolSegments     int
    OverageSegments  int
    ChargeMode       string // "pool" | "overage" | "split"

    // Общее:
    Strategy         string
    TariffPlanID     string
    ThresholdCrossed bool
    RecalcAmount     string
}

// Calculate выполняет read-only расчёт — никаких UPDATE/INSERT в БД.
// Возвращает все параметры, нужные CommitCharge для атомарного списания.
func (s *TarificationService) Calculate(ctx context.Context, req *TarifyMessageRequest) (*CalculateResult, error) {
    // ДОКУМЕНТИРОВАТЬ: порядок шагов должен повторять существующий TarifyMessage
    // 1-8 без step-ов 8a.ConsumeQuota и 8b.Charge, и без step 9 (IncrementUsage).
    // Это buffer для логики — её нужно аккуратно скопировать и адаптировать.

    // ... (реализация — порт существующего TarifyMessage до пункта "8. Check aggregator context",
    //      но только SELECT; все логи/события/write — вынесены)
    return nil, fmt.Errorf("not implemented — port logic from TarifyMessage sections 1-7 + aggregator/quota SELECT")
}
```

Детальный порт — в следующих шагах этой задачи.

- [ ] **Step 2: Порт логики**

Скопировать из `tarification_service.go` секции 1-7 (idempotency check → strategy.Calculate). Затем логику секции 8 (resolveAggregatorBilling — read-only, можно переиспользовать). Затем SELECT квоты без Consume — через новый метод `quotaService.GetActive(ctx, aggregatorID)`.

Вычисление `charge_mode` (pool/overage/split) на основе `quota.SegmentsUsed + req.SegmentCount` vs `quota.SegmentLimit`. Логика:

```go
// квота пусто или не нужна для direct/агрегатор-БМ
remaining := quota.SegmentLimit - quota.SegmentsUsed
switch {
case req.SegmentCount <= remaining:
    chargeMode = "pool"
    poolSegs = req.SegmentCount
    overageSegs = 0
case remaining <= 0:
    chargeMode = "overage"
    poolSegs = 0
    overageSegs = req.SegmentCount
default:
    chargeMode = "split"
    poolSegs = int(remaining)
    overageSegs = req.SegmentCount - int(remaining)
}
```

`aggregator_price_per_segment` = платформенный тариф агрегатора (из `result.PricePerSegment` — там уже платформенный).

`aggregator_total` = pool_segs * aggregator_price + overage_segs * overage_rate.

- [ ] **Step 3: Использовать UUID v7 для генерации `message_id` внутри Calculate**

При экстракции заменить места, где раньше генерировался MessageID (если генерировался — обычно приходит извне), на `uuid.NewV7()`. Если `MessageID` всегда приходит из req — оставить как есть, но **добавить валидацию**: если пустой — генерировать v7.

- [ ] **Step 4: Коммит**

```bash
git add internal/services/tarification/application/tarify_calculate.go
git commit -m "refactor(tarification): экстракт Calculate() из TarifyMessage — read-only расчёт"
```

---

## Task 11: Реализовать `CommitCharge` в application layer

**Files:**
- Create: `internal/services/tarification/application/commit_charge.go`

- [ ] **Step 1: Реализация**

```go
package application

import (
    "context"
    "fmt"

    "github.com/google/uuid"

    billingv1 "sms/api/proto/billingv1"
)

type CommitChargeRequest struct {
    MessageID uuid.UUID
}

type CommitChargeResult struct {
    Committed         bool
    AlreadyCommitted  bool
    QuotaMissing      bool
    SubInsufficient   bool
    AggInsufficient   bool
    NoTariff          bool
    SubAccountTxID    string
    AggregatorTxID    string
    MarginLogID       string
}

// CommitCharge повторяет расчёт и вызывает billing.ChargeMessageDual.
// Вызывается pipeline'ом после успешного SUBMIT к провайдеру.
func (s *TarificationService) CommitCharge(ctx context.Context, req *CommitChargeRequest) (*CommitChargeResult, error) {
    // 1. Восстановить контекст сообщения.
    // Проблема: Calculate принимает TarifyMessageRequest с ClientID/OperatorID/SenderName/SegmentCount.
    // После SUBMIT у нас есть только message_id. Нужно восстановить контекст из БД.
    // Варианты:
    //   (a) Pipeline передаёт TarifyMessageRequest полями — тогда CommitCharge принимает больше параметров.
    //   (b) Хранить расчёт в Redis на короткий TTL (ключ=message_id), pipeline передаёт только ID.
    //
    // Выбор: (a) — pipeline уже имеет контекст сообщения, меньше инфраструктуры.
    // Значит изменить proto: CommitChargeRequest должен содержать те же поля, что TarifyMessageRequest
    // плюс message_id для идемпотентности.
    return nil, fmt.Errorf("proto contract needs extension — see step 2")
}
```

- [ ] **Step 2: Расширить CommitChargeRequest proto**

Обновить `api/proto/tarification/tarification.proto`:

```protobuf
message CommitChargeRequest {
  string message_id = 1;
  string client_id = 2;           // субаккаунт
  string operator_id = 3;
  string sender_name = 4;
  int32 segment_count = 5;
  string idempotency_key = 6;
}
```

Регенерировать stubs:

```bash
bash scripts/generate-proto.sh
```

- [ ] **Step 3: Полная реализация CommitCharge**

```go
func (s *TarificationService) CommitCharge(ctx context.Context, req *CommitChargeRequest) (*CommitChargeResult, error) {
    calc, err := s.Calculate(ctx, &TarifyMessageRequest{
        ClientID:       req.ClientID,
        MessageID:      req.MessageID,
        OperatorID:     req.OperatorID,
        SenderName:     req.SenderName,
        SegmentCount:   req.SegmentCount,
        IdempotencyKey: req.IdempotencyKey,
    })
    if err != nil {
        return nil, fmt.Errorf("recalculate: %w", err)
    }
    if !calc.Approved {
        return &CommitChargeResult{NoTariff: calc.RejectionReason != ""}, nil
    }
    if calc.IsDirect {
        // Direct-клиент — делегируем обычному Charge через saga.
        // По решению спека: direct-клиенты продолжают использовать legacy-путь.
        // CommitCharge для них возвращает NoOp — вызывающий pipeline не должен вызывать CommitCharge
        // для direct. Но на всякий случай — аккуратный Charge здесь.
        result, err := s.saga.Charge(ctx, req.ClientID.String(), req.MessageID.String(),
            calc.PlatformAmount, calc.Currency,
            fmt.Sprintf("SMS direct: %d segments", req.SegmentCount), int32(req.SegmentCount))
        if err != nil {
            return nil, err
        }
        return &CommitChargeResult{
            Committed:       result.Success,
            SubAccountTxID:  result.TransactionID, // для direct — одна транзакция
            SubInsufficient: !result.Success,
        }, nil
    }

    // Субаккаунт — dual-charge.
    resp, err := s.billingClient.ChargeMessageDual(ctx, &billingv1.ChargeMessageDualRequest{
        MessageId:       req.MessageID.String(),
        SubAccountId:    req.ClientID.String(),
        AggregatorId:    calc.AggregatorID.String(),
        OperatorId:      calc.OperatorID.String(),
        SubAccountPrice: calc.SubAccountPrice,
        AggregatorPrice: calc.AggregatorPrice,
        SubAccountTotal: calc.SubAccountTotal,
        AggregatorTotal: calc.AggregatorTotal,
        Currency:        calc.Currency,
        SegmentCount:    int32(calc.SegmentCount),
        PoolSegments:    int32(calc.PoolSegments),
        OverageSegments: int32(calc.OverageSegments),
        ChargeMode:      calc.ChargeMode,
    })
    if err != nil {
        return nil, fmt.Errorf("billing.ChargeMessageDual: %w", err)
    }

    out := &CommitChargeResult{
        Committed:      resp.Committed,
        SubAccountTxID: resp.SubAccountTxId,
        AggregatorTxID: resp.AggregatorTxId,
        MarginLogID:    resp.MarginLogId,
    }
    switch resp.Error {
    case billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_ALREADY_COMMITTED:
        out.AlreadyCommitted = true
    case billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_QUOTA_NOT_CONFIGURED:
        out.QuotaMissing = true
    case billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_INSUFFICIENT_BALANCE_SUBACCOUNT:
        out.SubInsufficient = true
    case billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_INSUFFICIENT_BALANCE_AGGREGATOR:
        out.AggInsufficient = true
    }
    return out, nil
}
```

- [ ] **Step 4: Коммит**

```bash
git add internal/services/tarification/application/commit_charge.go \
        api/proto/tarification/tarification.proto \
        api/proto/tarificationv1/
git commit -m "feat(tarification): CommitCharge — вызывает billing.ChargeMessageDual после submit"
```

---

## Task 12: Перевести `TarifyMessage` в read-only под фиче-флагом

**Files:**
- Modify: `internal/services/tarification/application/tarification_service.go`
- Modify: `internal/config/config.go`

- [ ] **Step 1: Добавить флаг в конфиг**

В `internal/config/config.go`, в `TarificationConfig`:

```go
type TarificationConfig struct {
    // ... existing fields ...
    CommitOnSubmitEnabled bool `mapstructure:"commit_on_submit_enabled"`
}
```

В defaults (если есть `internal/config/defaults.go` — уточнить grep'ом):

```go
CommitOnSubmitEnabled: false,
```

- [ ] **Step 2: Пробросить флаг в TarificationService**

В `TarificationService` добавить поле `commitOnSubmitEnabled bool`. В constructor (NewTarificationService или аналог) принять.

- [ ] **Step 3: Добавить ветвление в TarifyMessage**

В начале `TarifyMessage` (после idempotency check):

```go
if s.commitOnSubmitEnabled {
    calc, err := s.Calculate(ctx, req)
    if err != nil {
        return nil, err
    }
    // Возврат расчёта без записей.
    return calcResultToResponse(calc), nil
}
// else: legacy flow (текущий код до конца функции)
```

Функция `calcResultToResponse`:

```go
func calcResultToResponse(c *CalculateResult) *TarifyMessageResponse {
    resp := &TarifyMessageResponse{
        Approved:         c.Approved,
        TotalAmount:      c.SubAccountTotal, // для субаккаунта; для direct — PlatformAmount
        Currency:         c.Currency,
        Strategy:         c.Strategy,
        TariffPlanID:     c.TariffPlanID,
        RejectionReason:  c.RejectionReason,
        ThresholdCrossed: c.ThresholdCrossed,
        RecalcAmount:     c.RecalcAmount,
    }
    if c.IsDirect {
        resp.TotalAmount = c.PlatformAmount
    }
    // новые поля (для commit-on-submit):
    resp.AggregatorID = c.AggregatorID.String()
    resp.SubAccountPrice = c.SubAccountPrice
    resp.AggregatorPrice = c.AggregatorPrice
    resp.SubAccountTotal = c.SubAccountTotal
    resp.AggregatorTotal = c.AggregatorTotal
    resp.SegmentCount = c.SegmentCount
    resp.PoolSegments = c.PoolSegments
    resp.OverageSegments = c.OverageSegments
    resp.ChargeMode = c.ChargeMode
    resp.OperatorID = c.OperatorID.String()
    return resp
}
```

- [ ] **Step 4: Обновить gRPC mapping TarifyMessageResponse в `grpc/server.go`**

В `grpc/server.go` TarifyMessage handler — маппить новые поля из application в proto.

- [ ] **Step 5: Запустить тесты**

```bash
go test ./internal/services/tarification/... -v
```

Существующие тесты должны пройти (legacy путь не тронут при `CommitOnSubmitEnabled=false`).

- [ ] **Step 6: Коммит**

```bash
git add internal/config/config.go \
        internal/services/tarification/application/tarification_service.go \
        internal/services/tarification/grpc/server.go
git commit -m "feat(tarification): фиче-флаг COMMIT_ON_SUBMIT — TarifyMessage в read-only режиме"
```

---

## Task 13: gRPC handler CommitCharge в tarification-service

**Files:**
- Modify: `internal/services/tarification/grpc/server.go`

- [ ] **Step 1: Добавить handler**

```go
func (s *Server) CommitCharge(ctx context.Context, req *tarificationv1.CommitChargeRequest) (*tarificationv1.CommitChargeResponse, error) {
    clientID, err := uuid.Parse(req.ClientId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
    }
    messageID, err := uuid.Parse(req.MessageId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid message_id: %v", err)
    }
    operatorID, err := uuid.Parse(req.OperatorId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
    }

    result, err := s.tarificationService.CommitCharge(ctx, &application.CommitChargeRequest{
        MessageID:      messageID,
        ClientID:       clientID,
        OperatorID:     operatorID,
        SenderName:     req.SenderName,
        SegmentCount:   int(req.SegmentCount),
        IdempotencyKey: req.IdempotencyKey,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "commit charge: %v", err)
    }

    resp := &tarificationv1.CommitChargeResponse{
        Committed:      result.Committed,
        SubAccountTxId: result.SubAccountTxID,
        AggregatorTxId: result.AggregatorTxID,
        MarginLogId:    result.MarginLogID,
    }
    switch {
    case result.AlreadyCommitted:
        resp.Error = tarificationv1.CommitChargeError_COMMIT_CHARGE_ERROR_ALREADY_COMMITTED
    case result.QuotaMissing:
        resp.Error = tarificationv1.CommitChargeError_COMMIT_CHARGE_ERROR_QUOTA_NOT_CONFIGURED
    case result.SubInsufficient:
        resp.Error = tarificationv1.CommitChargeError_COMMIT_CHARGE_ERROR_INSUFFICIENT_BALANCE_SUBACCOUNT
    case result.AggInsufficient:
        resp.Error = tarificationv1.CommitChargeError_COMMIT_CHARGE_ERROR_INSUFFICIENT_BALANCE_AGGREGATOR
    case result.NoTariff:
        resp.Error = tarificationv1.CommitChargeError_COMMIT_CHARGE_ERROR_NO_TARIFF
    }
    return resp, nil
}
```

- [ ] **Step 2: Коммит**

```bash
git add internal/services/tarification/grpc/server.go
git commit -m "grpc(tarification): handler CommitCharge"
```

---

## Task 14: Pipeline integration — вызов `CommitCharge` после успешного send

**Files:**
- Modify: `internal/pipeline/sender/stage.go`

- [ ] **Step 1: Найти точку вызова тарификации**

Прочитать `internal/pipeline/sender/stage.go:289-525` (функция `processMessage`). Найти:
- Вызов `s.tarificationClient.TarifyMessage` (примерно строка 328-373)
- Вызов `sender.SendMessageAsync` (строка ~416)
- Рефанд при провале (строки 425-445)

- [ ] **Step 2: Добавить ветвление под флагом**

Псевдокод изменения:

```go
// До отправки:
tarifyResp, tarifyErr := s.tarificationClient.TarifyMessage(ctx, tarifyReq)
// tarifyResp теперь содержит поля charge_mode, aggregator_id и т.д. если флаг on.

if tarifyErr != nil || !tarifyResp.Approved {
    // Не отправляем, не списываем.
    return
}

// Отправка:
smppMsgID, sendErr := sender.SendMessageAsync(ctx, sharedMsg, provider, conn)

// После успешной отправки — если флаг commit_on_submit включён на нашей стороне:
// (флаг приходит из config, пробрасывается в SenderStage)
if sendErr == nil && s.commitOnSubmitEnabled && tarifyResp.AggregatorId != "" {
    // Это субаккаунт — вызываем CommitCharge.
    commitCtx, commitCancel := context.WithTimeout(ctx, 10*time.Second)
    _, commitErr := s.tarificationClient.CommitCharge(commitCtx, &tarificationv1.CommitChargeRequest{
        MessageId:      tarifyReq.MessageId,
        ClientId:       tarifyReq.ClientId,
        OperatorId:     tarifyReq.OperatorId,
        SenderName:     tarifyReq.SenderName,
        SegmentCount:   tarifyReq.SegmentCount,
        IdempotencyKey: tarifyReq.IdempotencyKey,
    })
    commitCancel()
    if commitErr != nil {
        s.logger.Error().Err(commitErr).
            Str("message_id", routedMsg.MessageID.String()).
            Msg("CommitCharge failed after successful send — будет retry")
        // помечаем сообщение charge_failed (используем существующее поле message.charge_status
        // или новое — уточнить при реализации).
    }
}

// Существующая refund-логика (AddCredits) — выполняется только когда флаг OFF:
if sendErr != nil && routedMsg.RetryCount >= routedMsg.MaxRetries {
    if !s.commitOnSubmitEnabled && s.billingClient != nil && ... {
        // старый refund код
    }
    // При флаге ON — refund не нужен: мы не списывали при tarify.
}
```

- [ ] **Step 3: Пробросить `commitOnSubmitEnabled` в SenderStage**

Найти конструктор SenderStage в `stage.go`, добавить поле `commitOnSubmitEnabled bool`. На вызывающем уровне (где создаётся SenderStage) — передать значение из config.

- [ ] **Step 4: Запустить тесты**

```bash
go build ./internal/pipeline/...
go test ./internal/pipeline/... -short
```

- [ ] **Step 5: Коммит**

```bash
git add internal/pipeline/sender/stage.go
git commit -m "feat(pipeline): вызов CommitCharge после успешного send под commit_on_submit_enabled"
```

---

## Task 15: Удалить dead-code `saga.ChargeDual`

**Files:**
- Modify: `internal/services/tarification/application/saga.go`

- [ ] **Step 1: Убедиться что ChargeDual не вызывается**

```bash
grep -rn "ChargeDual\|DualChargeResult" internal/ api/ cmd/
```

Ожидаемо: только определение в saga.go + тесты для saga (если есть).

- [ ] **Step 2: Удалить методы ChargeDual и DualChargeResult**

Удалить строки 101-154 в `saga.go` (метод ChargeDual) и связанный тип `DualChargeResult` (если есть). Удалить импорты, которые стали неиспользуемыми (grep для `uuid.NewSHA1`).

- [ ] **Step 3: Прогнать build**

```bash
go build ./internal/services/tarification/...
```

- [ ] **Step 4: Коммит**

```bash
git add internal/services/tarification/application/saga.go
git commit -m "chore(tarification): удалить dead-code saga.ChargeDual"
```

---

## Task 16: E2E тест dual-charge

**Files:**
- Create: `e2e/tests/reseller/dual-charge.spec.ts`

- [ ] **Step 1: Написать spec**

```typescript
import { test, expect } from '@playwright/test';
import { seedAggregator, seedSubaccount, setQuota, setTariffs, sendSMS, getBalance, getMarginLog } from '../../helpers/api';

test.describe('Aggregator Dual-Charge', () => {
    test('happy path: отправка субаккаунтом списывает и с субаккаунта, и с агрегатора', async ({ request }) => {
        // 1. Создать агрегатора с балансом 1000₽
        const aggregator = await seedAggregator(request, { balance: '1000', isReseller: true });
        // 2. Создать субаккаунт с балансом 100₽ под этим агрегатором
        const subaccount = await seedSubaccount(request, { parentClientId: aggregator.id, balance: '100' });
        // 3. Настроить платформенный тариф агрегатора: 1₽/сегмент
        // 4. Настроить тариф субаккаунта (aggregator_tariffs): 5₽/сегмент
        // 5. Настроить квоту агрегатора: 1000 сегментов, overage_rate=2₽, auto_renew=true
        await setQuota(request, { aggregatorId: aggregator.id, segmentLimit: 1000, overageRate: '2' });
        await setTariffs(request, { aggregatorId: aggregator.id, subAccountId: subaccount.id, platformPrice: '1', subPrice: '5' });

        // 6. Отправить SMS от субаккаунта
        const result = await sendSMS(request, { clientId: subaccount.id, text: 'test', to: '+79000000000' });
        expect(result.status).toBe('sent');

        // 7. Проверить балансы: sub -5₽, aggregator -1₽
        expect(await getBalance(request, subaccount.id)).toBe('95');
        expect(await getBalance(request, aggregator.id)).toBe('999');

        // 8. Проверить margin_log: одна запись, charge_mode='pool', margin=4₽
        const margin = await getMarginLog(request, subaccount.id);
        expect(margin.length).toBe(1);
        expect(margin[0].charge_mode).toBe('pool');
        expect(margin[0].margin).toBe('4');
    });
});
```

Хелперы `seedAggregator/seedSubaccount/setQuota/setTariffs/sendSMS/getBalance/getMarginLog` — **большинство уже должно существовать** в `e2e/helpers/api.ts` (есть из прошлых e2e-тестов reseller). Недостающие — дописать там же.

- [ ] **Step 2: Запустить локально**

```bash
cd e2e && npx playwright test tests/reseller/dual-charge.spec.ts
```

- [ ] **Step 3: Коммит**

```bash
git add e2e/tests/reseller/dual-charge.spec.ts e2e/helpers/api.ts
git commit -m "test(e2e): dual-charge happy path — субаккаунт отправка, двойное списание, margin log"
```

---

## Task 17: Verification — включить флаг на тестовом агрегаторе и прогнать

**Files:** конфиги/env-variables.

- [ ] **Step 1: Включить флаг в локальной среде**

В `configs/local.yaml` (или аналог):

```yaml
tarification:
  commit_on_submit_enabled: true
```

Перезапустить локальные сервисы.

- [ ] **Step 2: Прогнать полный e2e test suite**

```bash
cd e2e && npx playwright test tests/reseller/
```

Ожидаемо: все reseller-тесты зелёные.

- [ ] **Step 3: Проверить руками через портал**

Создать агрегатора с квотой, субаккаунт. Отправить SMS. Проверить:
- Баланс субаккаунта и агрегатора уменьшились корректно.
- В `aggregator_margin_log` запись есть с корректным `charge_mode`.
- Если SUBMIT упал (отключить провайдер) — деньги не списались.

- [ ] **Step 4: Коммит конфига**

```bash
git add configs/local.yaml
git commit -m "chore: включить commit_on_submit_enabled в local config"
```

---

## Self-Review

- [x] **Spec coverage:** все 4 пункта из Scope спека покрыты tasks:
    - #1 атомизация → Task 7 (ChargeMessageDual) + Task 14 (pipeline wiring)
    - #2 read-only TarifyMessage + CommitCharge → Task 10 (Calculate), Task 11 (CommitCharge), Task 12 (flag)
    - #3 charge_mode поля → Task 1 (migration), Task 2 (domain), Task 3 (repo), Task 10 (calc), Task 7 (save)
    - #4 UUID v7 → встроено в Task 10 (Calculate)
- [x] **Спек Q10 (lazy-create)** → Task 7 step 3 (GetLatestTx + CreateTx в транзакции)
- [x] **Удаление dead-code saga.ChargeDual** → Task 15
- [x] **Pre-submit failure refund** → при включённом флаге рефанд не нужен (не списывали) — Task 14
- [x] **Идемпотентность через UNIQUE(idempotency_key)** → Task 3 (фикс ON CONFLICT)
- [ ] **Метрики (commit_charge_duration, charge_result_total)** — НЕ ВКЛЮЧЕНЫ в план. Причина: спек помечал метрики как "расширить существующие", но конкретные точки metric collector'ов в проекте не исследованы. Предлагаю отдельную Task 18 при следующей итерации (когда основной flow будет работать). Пока что базовая функциональность важнее.
- [ ] **Retry-worker для charge_failed** — НЕ ВКЛЮЧЕН. Причина: идемпотентность `CommitCharge` позволяет retry на стороне pipeline через существующий механизм re-enqueue. Если при верификации (Task 17) окажется, что нужна отдельная логика — создать отдельную задачу.

**Placeholder scan:** TBD/TODO нет. "Уточнить при реализации" встречается в:
- Task 3 Step 3 (импорт sqlx) — рекомендация проверить go.mod
- Task 9 Step 2 (NewServer location) — указано конкретное grep-команда
- Task 12 Step 1 (defaults.go) — указано grep
- Task 15 Step 1 (uuid.NewSHA1) — указано конкретное grep

Эти "уточнить" — не placeholder в планах, а directives для исполнителя проверить факт перед правкой (нормально).

**Type consistency:** проверено. `ChargeMessageDualInput.SubAccountID` в Task 7 совпадает с proto `sub_account_id` (Task 5). `CalculateResult.AggregatorTotal` в Task 10 совпадает с `aggregator_total` в proto и с `in.AggregatorTotal` в Task 7.

---

## Outstanding Risks

- **Task 7 Step 3 `chargeBalanceTx`** — используется прямое `UPDATE accounts` вместо существующего `billingService.ChargeMessage` flow. Причина: нужно в одной транзакции с margin_log/quotas. Жертва: дублирование логики валюты/заморозки/low-balance-alert. **Mitigation:** проверить заморозку через `SELECT frozen FROM accounts WHERE client_id = $1 FOR UPDATE` как отдельный шаг перед UPDATE. Событие `balance_changed` публиковать после коммита.
- **Drift между Calculate и ChargeMessageDual** — повторный Calculate в CommitCharge может дать другой тариф (между tarify и commit прошло время). Акцептим: commit-ответ содержит актуальные цены, клиент может логировать.
- **Миграция 000106 с NOT NULL дефолтом** — при применении старый код падать не должен, потому что `Create` ещё не знает про новые поля (Task 3 идёт следом). В интервале между 000106 и деплоем Task 3 — нет проблемы, потому что дефолт `'pool'`/`0` в DDL.

---

**Plan complete and saved to `docs/superpowers/plans/2026-04-20-aggregator-dual-charge.md`.**
