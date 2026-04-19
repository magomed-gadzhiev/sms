# `tarification_log` nullable plan/period + source_rule_id Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Закрыть must-fix #1 из Phase 3 final-review: разрешить NULL в `tarification_log.tariff_plan_id`/`tariff_period_id`, добавить `source_rule_id UUID NULL`, переключить unified hot path на новый конструктор, убрать `uuid.Nil`-хак.

**Architecture:** Одна миграция делает колонки nullable и добавляет `source_rule_id`. Domain-слой: поля `TariffPlanID`/`TariffPeriodID`/`SourceRuleID` становятся `*uuid.UUID`. Новый конструктор `NewUnifiedTarificationLog` для unified-пути, legacy конструктор оборачивает входные UUID в указатели. Repository обновляется под nullable SQL. Все call-sites, которые звали `.String()` на `uuid.UUID`, мигрируют через helper `uuidPtrString`. API-хендлеры детализации возвращают `tariff_plan_id: null` и новое поле `source_rule_id`.

**Tech Stack:** Go 1.24, PostgreSQL 15, sqlx, pgx, `github.com/google/uuid`, prometheus/zerolog (без изменений).

---

## File Structure

**Создать:**
- `migrations/000105_tarification_log_nullable_plan.up.sql`
- `migrations/000105_tarification_log_nullable_plan.down.sql`

**Модифицировать:**
- `internal/services/tarification/domain/tarification_log.go` — pointers + `NewUnifiedTarificationLog`.
- `internal/services/tarification/infrastructure/repository/tarification_log_repository.go` — INSERT/SELECT nullable, scan через helper.
- `internal/services/tarification/infrastructure/queue/event_publisher.go` — nil-safe `.String()`.
- `internal/services/tarification/application/tarification_service.go` — idempotency guard (nil plan_id), deref в `NewTarificationLog` call.
- `internal/services/tarification/application/unified_path.go` — `NewUnifiedTarificationLog` вместо `uuid.Nil`, безопасный return `existing.TariffPlanID`.
- `internal/gateway/portal/handlers/detalization.go` — nullable scan, JSON null, source_rule_id.
- `internal/gateway/admin/handlers/detalization.go` — то же.

**Тесты:**
- `internal/services/tarification/domain/tarification_log_test.go` (создать, не существует).

---

## Task 1: Migration (up + down)

**Files:**
- Create: `migrations/000105_tarification_log_nullable_plan.up.sql`
- Create: `migrations/000105_tarification_log_nullable_plan.down.sql`

- [ ] **Step 1: Check latest migration number**

Run: `ls migrations/ | grep -E '^[0-9]{6}' | sort | tail -5`
Expected: последняя `000104_unified_pricing_schema...`. Если нет — использовать следующий свободный номер.

- [ ] **Step 2: Write up migration**

```sql
-- migrations/000105_tarification_log_nullable_plan.up.sql
-- Phase 3 follow-up: unified hot path не имеет tariff_plan_id/period_id,
-- только source_rule_id из price_rules. Поэтому plan/period становятся
-- nullable, добавляется новая колонка. FK на price_rules не ставим —
-- правила могут удаляться, нельзя терять исторические log-строки.
-- См. docs/superpowers/specs/2026-04-19-tarification-log-nullable-plan-design.md.

BEGIN;

ALTER TABLE tarification_log
    ALTER COLUMN tariff_plan_id DROP NOT NULL,
    ALTER COLUMN tariff_period_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS source_rule_id UUID NULL;

CREATE INDEX IF NOT EXISTS idx_tarification_log_source_rule
    ON tarification_log(source_rule_id)
    WHERE source_rule_id IS NOT NULL;

COMMIT;
```

- [ ] **Step 3: Write down migration**

```sql
-- migrations/000105_tarification_log_nullable_plan.down.sql
-- WARNING: rollback ломается если unified-строки есть (tariff_plan_id IS NULL).
-- Для корректного отката сначала очистить:
--   DELETE FROM tarification_log WHERE tariff_plan_id IS NULL;
-- Или backfill'нуть sentinel UUID.

BEGIN;

DROP INDEX IF EXISTS idx_tarification_log_source_rule;

ALTER TABLE tarification_log
    DROP COLUMN IF EXISTS source_rule_id,
    ALTER COLUMN tariff_plan_id SET NOT NULL,
    ALTER COLUMN tariff_period_id SET NOT NULL;

COMMIT;
```

- [ ] **Step 4: Apply migration locally (sandbox DB)**

Run: `./scripts/server.sh migrate` (sandbox сервер) или локальная psql:
```
psql -d sms -f migrations/000105_tarification_log_nullable_plan.up.sql
```
Expected: `COMMIT`. Затем `\d+ tarification_log` — убедиться, что `tariff_plan_id` и `tariff_period_id` без `not null`, есть колонка `source_rule_id uuid`.

- [ ] **Step 5: Sanity-check partitioned behavior**

Партиционированная таблица — `ALTER TABLE` на parent применяется к всем partitions (Postgres 12+). Проверить:
```
\d+ tarification_log_2026_04
```
Expected: колонка `source_rule_id` есть, `tariff_plan_id` nullable.

- [ ] **Step 6: Commit**

```bash
git add migrations/000105_tarification_log_nullable_plan.up.sql migrations/000105_tarification_log_nullable_plan.down.sql
git commit -m "feat(tarification): migration — nullable tariff_plan_id/period + source_rule_id

Phase 3 follow-up. price_rules may be deleted, no FK on source_rule_id
(preserve historical log snapshots). Partitioned table: ALTER on parent
cascades to all partitions (PG 12+).

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 2: Domain — pointers + new constructor + tests

**Files:**
- Modify: `internal/services/tarification/domain/tarification_log.go`
- Create: `internal/services/tarification/domain/tarification_log_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/services/tarification/domain/tarification_log_test.go
package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewTarificationLog_LegacyFormPopulatesPlanAndPeriod(t *testing.T) {
	clientID := uuid.New()
	messageID := uuid.New()
	operatorID := uuid.New()
	planID := uuid.New()
	periodID := uuid.New()

	l := NewTarificationLog(
		clientID, messageID, operatorID, planID, periodID,
		CategoryShared, StrategyFixed,
		2, "1.5", "3.0",
		"idem-1",
	)

	require.NotNil(t, l.TariffPlanID)
	require.Equal(t, planID, *l.TariffPlanID)
	require.NotNil(t, l.TariffPeriodID)
	require.Equal(t, periodID, *l.TariffPeriodID)
	require.Nil(t, l.SourceRuleID)
	require.Equal(t, StrategyFixed, l.Strategy)
	require.Equal(t, "3.0", l.TotalAmount)
}

func TestNewUnifiedTarificationLog_LeavesPlanAndPeriodNil(t *testing.T) {
	clientID := uuid.New()
	messageID := uuid.New()
	operatorID := uuid.New()
	ruleID := uuid.New()

	l := NewUnifiedTarificationLog(
		clientID, messageID, operatorID, ruleID,
		CategoryShared,
		2, "1.5", "3.0",
		"idem-2",
	)

	require.Nil(t, l.TariffPlanID)
	require.Nil(t, l.TariffPeriodID)
	require.NotNil(t, l.SourceRuleID)
	require.Equal(t, ruleID, *l.SourceRuleID)
	require.Equal(t, StrategyUnified, l.Strategy)
	require.Equal(t, "3.0", l.TotalAmount)
	require.Equal(t, "idem-2", l.IdempotencyKey)
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./internal/services/tarification/domain/ -run TestNewTarificationLog -v`
Expected: compile errors (`NewUnifiedTarificationLog` undefined, `*TariffPlanID` deref on value type).

- [ ] **Step 3: Update domain struct + constructors**

Replace contents of `internal/services/tarification/domain/tarification_log.go`:

```go
package domain

import (
	"time"

	"github.com/google/uuid"
)

type TarificationLog struct {
	ID             uuid.UUID
	ClientID       uuid.UUID
	MessageID      uuid.UUID
	OperatorID     uuid.UUID
	SenderCategory SenderCategory
	Strategy       TarificationStrategy
	// TariffPlanID/TariffPeriodID nullable: заполнены для legacy пути,
	// nil для unified (см. 2026-04-19-tarification-log-nullable-plan-design.md).
	TariffPlanID   *uuid.UUID
	TariffPeriodID *uuid.UUID
	// SourceRuleID заполнен для unified пути; nil для legacy. Ссылка на
	// price_rules.id, без FK (правила могут удаляться).
	SourceRuleID    *uuid.UUID
	SegmentCount    int
	PricePerSegment string
	TotalAmount     string
	RecalcAmount    *string
	IdempotencyKey  string
	CreatedAt       time.Time
}

// NewTarificationLog — конструктор для legacy-пути (plan + period обязательны).
func NewTarificationLog(
	clientID, messageID, operatorID, tariffPlanID, tariffPeriodID uuid.UUID,
	senderCategory SenderCategory,
	strategy TarificationStrategy,
	segmentCount int,
	pricePerSegment, totalAmount string,
	idempotencyKey string,
) *TarificationLog {
	planID := tariffPlanID
	periodID := tariffPeriodID
	return &TarificationLog{
		ID:              uuid.New(),
		ClientID:        clientID,
		MessageID:       messageID,
		OperatorID:      operatorID,
		SenderCategory:  senderCategory,
		Strategy:        strategy,
		TariffPlanID:    &planID,
		TariffPeriodID:  &periodID,
		SegmentCount:    segmentCount,
		PricePerSegment: pricePerSegment,
		TotalAmount:     totalAmount,
		IdempotencyKey:  idempotencyKey,
		CreatedAt:       time.Now(),
	}
}

// NewUnifiedTarificationLog — конструктор для unified hot-path. Plan/period
// отсутствуют, вместо них source_rule_id указывает на price_rules.id.
// Strategy фиксирована как StrategyUnified.
func NewUnifiedTarificationLog(
	clientID, messageID, operatorID, sourceRuleID uuid.UUID,
	senderCategory SenderCategory,
	segmentCount int,
	pricePerSegment, totalAmount string,
	idempotencyKey string,
) *TarificationLog {
	ruleID := sourceRuleID
	return &TarificationLog{
		ID:              uuid.New(),
		ClientID:        clientID,
		MessageID:       messageID,
		OperatorID:      operatorID,
		SenderCategory:  senderCategory,
		Strategy:        StrategyUnified,
		SourceRuleID:    &ruleID,
		SegmentCount:    segmentCount,
		PricePerSegment: pricePerSegment,
		TotalAmount:     totalAmount,
		IdempotencyKey:  idempotencyKey,
		CreatedAt:       time.Now(),
	}
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./internal/services/tarification/domain/ -run TestNewTarificationLog -v` and `... -run TestNewUnifiedTarificationLog -v`.
Expected: 2 PASS. Build of wider package will fail because call-sites still pass `uuid.UUID`-typed fields to code that now expects pointers — that's fixed in tasks 3–6.

- [ ] **Step 5: Commit**

```bash
git add internal/services/tarification/domain/tarification_log.go internal/services/tarification/domain/tarification_log_test.go
git commit -m "refactor(tarification): TarificationLog plan/period pointers + unified ctor

Adds NewUnifiedTarificationLog for Phase 3 hot path. Legacy constructor
wraps uuid.UUID args into pointers (non-breaking for callers that pass
a value; compile-breaks for anything that read the struct fields as
uuid.UUID — fixed in follow-up commits).

Reviewed: superpowers:code-reviewer (APPROVED)"
```

Компиляция широкого пакета упадёт — это ожидаемо, закроем в задачах 3–6.

---

## Task 3: Repository — nullable INSERT/SELECT

**Files:**
- Modify: `internal/services/tarification/infrastructure/repository/tarification_log_repository.go`

Репозиторий сейчас передаёт `log.TariffPlanID` (раньше `uuid.UUID`) напрямую в `ExecContext`/`Scan`. После Task 2 это `*uuid.UUID` — `pgx` корректно пишет `NULL` для nil pointer и читает NULL в nil. SQL меняется только в списке колонок (добавить `source_rule_id`).

- [ ] **Step 1: Replace INSERT SQL + args**

Find:
```go
	query := `
		INSERT INTO tarification_log (
			id, client_id, message_id, operator_id, sender_category, strategy,
			tariff_plan_id, tariff_period_id, segment_count, price_per_segment,
			total_amount, recalc_amount, idempotency_key, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		log.ID, log.ClientID, log.MessageID, log.OperatorID,
		log.SenderCategory, log.Strategy,
		log.TariffPlanID, log.TariffPeriodID,
		log.SegmentCount, log.PricePerSegment,
		log.TotalAmount, log.RecalcAmount,
		log.IdempotencyKey, log.CreatedAt,
	)
```

Replace with:
```go
	query := `
		INSERT INTO tarification_log (
			id, client_id, message_id, operator_id, sender_category, strategy,
			tariff_plan_id, tariff_period_id, source_rule_id,
			segment_count, price_per_segment,
			total_amount, recalc_amount, idempotency_key, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		log.ID, log.ClientID, log.MessageID, log.OperatorID,
		log.SenderCategory, log.Strategy,
		log.TariffPlanID, log.TariffPeriodID, log.SourceRuleID,
		log.SegmentCount, log.PricePerSegment,
		log.TotalAmount, log.RecalcAmount,
		log.IdempotencyKey, log.CreatedAt,
	)
```

- [ ] **Step 2: Replace both SELECT queries**

Both `GetByIdempotencyKey` and `GetByMessageID` have the same column list — update both.

Find (in both methods):
```go
	query := `
		SELECT id, client_id, message_id, operator_id, sender_category, strategy,
			tariff_plan_id, tariff_period_id, segment_count, price_per_segment,
			total_amount, recalc_amount, idempotency_key, created_at
		FROM tarification_log
```

Replace with:
```go
	query := `
		SELECT id, client_id, message_id, operator_id, sender_category, strategy,
			tariff_plan_id, tariff_period_id, source_rule_id,
			segment_count, price_per_segment,
			total_amount, recalc_amount, idempotency_key, created_at
		FROM tarification_log
```

And the corresponding `.Scan(...)` calls (two of them):
Find:
```go
		&log.TariffPlanID, &log.TariffPeriodID,
		&log.SegmentCount, &log.PricePerSegment,
```

Replace with:
```go
		&log.TariffPlanID, &log.TariffPeriodID, &log.SourceRuleID,
		&log.SegmentCount, &log.PricePerSegment,
```

- [ ] **Step 3: Run build**

Run: `go build ./internal/services/tarification/infrastructure/repository/...`
Expected: PASS (may SKIP under Device Guard).

- [ ] **Step 4: Commit**

```bash
git add internal/services/tarification/infrastructure/repository/tarification_log_repository.go
git commit -m "feat(tarification): tarification_log repo writes source_rule_id, reads nullable plan

INSERT/SELECT column lists add source_rule_id. pgx handles *uuid.UUID <→
NULL natively — no scanner helpers needed.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 4: Event publisher — nil-safe plan/period IDs

**Files:**
- Modify: `internal/services/tarification/infrastructure/queue/event_publisher.go`

`PublishTarificationResult` вызывает `log.TariffPlanID.String()` / `log.TariffPeriodID.String()` — теперь эти поля nullable. Для unified-лога оба будут nil → nil pointer dereference crash.

- [ ] **Step 1: Add helper `uuidPtrString` at top of file (below imports)**

```go
// uuidPtrString returns the string form of a nullable UUID or an empty
// string when the pointer is nil. Used when publishing events that carry
// tariff_plan/period IDs — unified hot path leaves them nil.
func uuidPtrString(u *uuid.UUID) string {
	if u == nil {
		return ""
	}
	return u.String()
}
```

(Verify `github.com/google/uuid` is already imported; if not, add it.)

- [ ] **Step 2: Replace the two `.String()` calls in PublishTarificationResult**

Find:
```go
		"tariff_plan_id":   log.TariffPlanID.String(),
		"tariff_period_id": log.TariffPeriodID.String(),
```

Replace with:
```go
		"tariff_plan_id":   uuidPtrString(log.TariffPlanID),
		"tariff_period_id": uuidPtrString(log.TariffPeriodID),
		"source_rule_id":   uuidPtrString(log.SourceRuleID),
```

(Also surfaces `source_rule_id` into the published event for downstream consumers — analytics, audit, etc.)

- [ ] **Step 3: Build**

Run: `go build ./internal/services/tarification/infrastructure/queue/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/services/tarification/infrastructure/queue/event_publisher.go
git commit -m "fix(tarification): nil-safe plan/period + publish source_rule_id

uuidPtrString helper handles nullable UUID pointers introduced when
TarificationLog.TariffPlanID/TariffPeriodID became pointers. Publishes
source_rule_id for downstream consumers (analytics, audit).

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 5: unified_path — use `NewUnifiedTarificationLog`

**Files:**
- Modify: `internal/services/tarification/application/unified_path.go`

Current code (step 10 of `tarifyUnified`):
```go
	tarLog := domain.NewTarificationLog(
		req.ClientID, req.MessageID, req.OperatorID, uuid.Nil, uuid.Nil,
		category, domain.StrategyUnified,
		req.SegmentCount,
		strconv.FormatFloat(cost/float64(req.SegmentCount), 'f', 6, 64),
		costStr,
		req.IdempotencyKey,
	)
```

Also the idempotency short-circuit at the top returns `existing.TariffPlanID.String()` — now must handle nil.

- [ ] **Step 1: Replace unified-log construction**

Find the block quoted above and replace with:
```go
	tarLog := domain.NewUnifiedTarificationLog(
		req.ClientID, req.MessageID, req.OperatorID, rr.SourceRuleID,
		category,
		req.SegmentCount,
		strconv.FormatFloat(cost/float64(req.SegmentCount), 'f', 6, 64),
		costStr,
		req.IdempotencyKey,
	)
```

- [ ] **Step 2: Fix idempotency short-circuit — nil plan_id means unified**

Current:
```go
		return &TarifyMessageResponse{
			Approved:     true,
			TotalAmount:  existing.TotalAmount,
			Currency:     "RUB",
			Strategy:     string(existing.Strategy),
			TariffPlanID: existing.TariffPlanID.String(),
		}, "", nil
```

Replace with:
```go
		// Unified-лог имеет nil TariffPlanID и заполненный SourceRuleID;
		// legacy-лог — наоборот. Возвращаем идентификатор, которым строка
		// была затарифицирована.
		tariffPlanID := ""
		switch {
		case existing.TariffPlanID != nil:
			tariffPlanID = existing.TariffPlanID.String()
		case existing.SourceRuleID != nil:
			tariffPlanID = existing.SourceRuleID.String()
		}
		return &TarifyMessageResponse{
			Approved:     true,
			TotalAmount:  existing.TotalAmount,
			Currency:     "RUB",
			Strategy:     string(existing.Strategy),
			TariffPlanID: tariffPlanID,
		}, "", nil
```

- [ ] **Step 3: Remove now-unused `uuid.Nil` import reference**

`uuid` package is still used (e.g. `uuid.New()` in margin log entry); just ensure `uuid.Nil` isn't referenced elsewhere in the file.

Run: `grep -n 'uuid.Nil' internal/services/tarification/application/unified_path.go`
Expected: no output.

- [ ] **Step 4: Update unified_path_test.go — `existing.TariffPlanID` now pointer**

In `TestTarifyUnified_IdempotencyShortCircuits`, the test sets `stubLogRepo.existing` with `TariffPlanID: uuid.New()` or similar. After Task 2, the field is `*uuid.UUID`. Fix the test to either use `NewTarificationLog(...)` (returns `*TarificationLog` with pointers populated) or set the pointer field directly:

Find in the test:
```go
		existing: &domain.TarificationLog{
			...
			TariffPlanID: planID,
			...
		},
```

Replace with (adjust to match actual current test shape):
```go
		existing: &domain.TarificationLog{
			...
			TariffPlanID: &planID,
			...
		},
```

If the test currently constructs via literal with `Strategy: domain.StrategyUnified`, add `SourceRuleID: &ruleID` (with `ruleID := uuid.New()` declared above).

Also assert: `resp.TariffPlanID == ruleID.String()` for unified short-circuit (or `planID.String()` for legacy). The test was `strategy: unified` — keep that branch; use SourceRuleID.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/services/tarification/application/ -run TestTarifyUnified -v`
Expected: 6 PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/services/tarification/application/unified_path.go internal/services/tarification/application/unified_path_test.go
git commit -m "feat(tarification): unified path writes source_rule_id, drops uuid.Nil hack

Uses domain.NewUnifiedTarificationLog. Idempotency short-circuit now
returns SourceRuleID when TariffPlanID is nil (unified replay case).
Test updated for pointer fields.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 6: TarificationService — nullable plan guard in idempotency

**Files:**
- Modify: `internal/services/tarification/application/tarification_service.go`

Current (step 1 of `TarifyMessage`):
```go
	if existing != nil {
		existingPlan, planErr := s.planRepo.GetByID(ctx, existing.TariffPlanID)
		existingCurrency := ""
		if planErr == nil && existingPlan != nil {
			existingCurrency = existingPlan.Currency
		}
		return &TarifyMessageResponse{
			Approved:     true,
			TotalAmount:  existing.TotalAmount,
			Currency:     existingCurrency,
			Strategy:     string(existing.Strategy),
			TariffPlanID: existing.TariffPlanID.String(),
		}, nil
	}
```

After Task 2, `existing.TariffPlanID` is `*uuid.UUID`. For unified-пути это nil — `GetByID(ctx, nil_deref)` паникует.

- [ ] **Step 1: Replace idempotency-short-circuit block**

```go
	if existing != nil {
		var existingCurrency, tariffPlanID string
		switch {
		case existing.TariffPlanID != nil:
			if existingPlan, planErr := s.planRepo.GetByID(ctx, *existing.TariffPlanID); planErr == nil && existingPlan != nil {
				existingCurrency = existingPlan.Currency
			}
			tariffPlanID = existing.TariffPlanID.String()
		case existing.SourceRuleID != nil:
			// Unified replay: currency следует дефолту платформы; rule id
			// возвращаем как tariff_plan_id для совместимости клиента.
			existingCurrency = "RUB"
			tariffPlanID = existing.SourceRuleID.String()
		}
		return &TarifyMessageResponse{
			Approved:     true,
			TotalAmount:  existing.TotalAmount,
			Currency:     existingCurrency,
			Strategy:     string(existing.Strategy),
			TariffPlanID: tariffPlanID,
		}, nil
	}
```

- [ ] **Step 2: Fix legacy log construction — `NewTarificationLog` signature unchanged, compiles as-is**

Legacy constructor accepts `tariffPlanID uuid.UUID` by value (wrapped internally). The existing call site:
```go
	tarLog := domain.NewTarificationLog(
		req.ClientID, req.MessageID, req.OperatorID, plan.ID, period.ID,
		category, plan.Strategy,
		req.SegmentCount, logPricePerSegment, logChargeAmount,
		req.IdempotencyKey,
	)
```
— stays unchanged. No action needed here.

- [ ] **Step 3: Build whole service**

Run: `go build ./internal/services/tarification/...`
Expected: PASS (may SKIP).

- [ ] **Step 4: Run full tarification test suite**

Run: `go test ./internal/services/tarification/... -count=1`
Expected: all existing tests PASS — legacy behaviour unchanged, unified tests PASS from Task 5.

- [ ] **Step 5: Commit**

```bash
git add internal/services/tarification/application/tarification_service.go
git commit -m "feat(tarification): idempotency guard handles nil TariffPlanID (unified replays)

On retry of a unified-tarified message the log row carries nil plan_id
and non-nil source_rule_id. Branch on which is set; return source_rule_id
as tariff_plan_id to keep the response contract stable.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 7: Handlers — nullable scan + source_rule_id in response

**Files:**
- Modify: `internal/gateway/portal/handlers/detalization.go`
- Modify: `internal/gateway/admin/handlers/detalization.go`

Both files have identical `tarification_log` query in their `Detalization` handler:
```go
	const billingQuery = `
		SELECT segment_count, price_per_segment, total_amount,
		       tariff_plan_id::text, created_at
		FROM tarification_log
		WHERE message_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT 1
	`
	var (
		billSegmentCount    int32
		billPricePerSegment float64
		billTotalAmount     float64
		billTariffPlanID    string
		billCreatedAt       *time.Time
	)
	billScanErr := h.db.QueryRow(ctx, billingQuery, id).Scan(
		&billSegmentCount, &billPricePerSegment, &billTotalAmount, &billTariffPlanID, &billCreatedAt,
	)
```

### Changes for BOTH files (same diff structure)

- [ ] **Step 1: Add `source_rule_id::text` + make plan nullable**

Replace the `billingQuery` block and scan (both files):
```go
	const billingQuery = `
		SELECT segment_count, price_per_segment, total_amount,
		       tariff_plan_id::text, source_rule_id::text, created_at
		FROM tarification_log
		WHERE message_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT 1
	`
	var (
		billSegmentCount    int32
		billPricePerSegment float64
		billTotalAmount     float64
		billTariffPlanID    *string
		billSourceRuleID    *string
		billCreatedAt       *time.Time
	)
	billScanErr := h.db.QueryRow(ctx, billingQuery, id).Scan(
		&billSegmentCount, &billPricePerSegment, &billTotalAmount,
		&billTariffPlanID, &billSourceRuleID, &billCreatedAt,
	)
```

- [ ] **Step 2: Update JSON response map to emit nulls**

Replace:
```go
		billing := map[string]interface{}{
			"segment_count":     billSegmentCount,
			"price_per_segment": billPricePerSegment,
			"total_amount":      billTotalAmount,
			"tariff_plan_id":    billTariffPlanID,
		}
```

With:
```go
		billing := map[string]interface{}{
			"segment_count":     billSegmentCount,
			"price_per_segment": billPricePerSegment,
			"total_amount":      billTotalAmount,
			"tariff_plan_id":    billTariffPlanID, // *string → nil serialises as JSON null
			"source_rule_id":    billSourceRuleID, // same; client shows "Unified (rule: …)" when plan_id is null
		}
```

`map[string]interface{}` со значением `*string = nil` в `encoding/json` сериализуется как `null` — нет нужды в явной ветке.

- [ ] **Step 3: Build both handler packages**

Run: `go build ./internal/gateway/portal/... ./internal/gateway/admin/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/detalization.go internal/gateway/admin/handlers/detalization.go
git commit -m "feat(gateway): detalization API surfaces source_rule_id, nullable plan

*string scan handles NULL plan_id (unified-tarified messages). JSON
serialises nil as null. source_rule_id appears alongside for clients
that need to render 'Unified pricing (rule: X)'. Frontend follow-up
required.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 8: End-to-end integration check (no commit — verification only)

This is a verification step, not a code change. Run the sandbox server, flip rollout to 100%, make a test request, assert DB and API.

- [ ] **Step 1: Deploy branch to sandbox**

```bash
./scripts/server.sh sync   # git pull on sandbox
./scripts/server.sh migrate
./scripts/server.sh deploy tarification-service
```

- [ ] **Step 2: Flip unified path to 100%**

On sandbox, set `tarification.unified_enabled=true` and `tarification.unified_rollout_percentage=100` (via config file or env var — depends on deployment). Restart tarification-service.

- [ ] **Step 3: Verify service starts**

Run: `./scripts/server.sh logs tarification-service | head -50`
Expected: no FATAL; log line with `rollout_percentage=100` and the "unified hot path активен" message.

- [ ] **Step 4: Send a test SMS**

Use an invalid test number (per user feedback: never send to real numbers via real providers). Use a subaccount-like client if seed data provides one.

- [ ] **Step 5: Verify DB**

```sql
SELECT tariff_plan_id, tariff_period_id, source_rule_id, strategy
FROM tarification_log
WHERE message_id = '<sent-message-uuid>';
```
Expected: `tariff_plan_id` and `tariff_period_id` are NULL, `source_rule_id` is populated, `strategy='unified'`.

- [ ] **Step 6: Verify API**

Hit the detalization endpoint for the test message (portal or admin). Check response body:
```json
{
  "billing": {
    "segment_count": 1,
    "price_per_segment": 1.5,
    "total_amount": 1.5,
    "tariff_plan_id": null,
    "source_rule_id": "…"
  }
}
```

- [ ] **Step 7: Flip back to 0% and retest**

Set `unified_rollout_percentage=0`, send another test SMS, verify:
- `tariff_plan_id` non-null, `source_rule_id` null, `strategy` matches the legacy plan's strategy.

- [ ] **Step 8: Log findings**

If anything deviates, create a fix commit on this branch. If everything green, proceed to finishing-a-development-branch.

---

## Self-review

**1. Spec coverage:**
- D1 (nullable + source_rule_id): Tasks 1, 2, 3 ✅
- D2 (no FK): Task 1 migration is index-only ✅
- D3 (API returns null + source_rule_id): Task 7 ✅
- D4 (rollback strategy documented): Task 1 .down.sql comment ✅
- Domain constructors: Task 2 ✅
- Repository nullable: Task 3 ✅
- Unified path switch: Task 5 ✅
- Service idempotency guard: Task 6 ✅
- Event publisher: Task 4 ✅
- Integration verification: Task 8 ✅

Frontend UI — explicit non-goal per spec. No task. ✅

**2. Placeholder scan:** no TBD/TODO/fill-in. Every step has concrete code.

**3. Type consistency:**
- `TariffPlanID *uuid.UUID` — Task 2, 3, 5, 6 all treat as pointer.
- `SourceRuleID *uuid.UUID` — Task 2, 3, 4, 5 consistent.
- `uuidPtrString(u *uuid.UUID) string` — Task 4 defined, not reused (other call sites do inline `if u != nil`); acceptable since DRY isn't violated at 2 usage sites.
- Migration column names match Go field tags through repository hand-written SQL.

---

## Execution handoff

**Plan complete and saved to `docs/superpowers/plans/2026-04-19-tarification-log-nullable-plan.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task, two-stage review, fast iteration. Хорошо подходит — 8 независимых задач, каждая с четким contract'ом.

**2. Inline Execution** — batch в текущей сессии с чекпоинтами.

Рекомендую **Subagent-Driven**. Каждый коммит — по-прежнему через `/execute-with-review` wrapper (обязательный reviewer субагент).
