# Unified Currency via Operator Meta Lookup — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Расширить `OperatorCodeLookup → OperatorMetaLookup` (возвращает `{Code, Currency}`), резолвить currency в unified hot path через `operators → countries` JOIN вместо hardcoded `"RUB"`. Закрывает Phase 3 should-fix #1.

**Architecture:** Один `LEFT JOIN` в repo, кеш того же устройства с новым value-типом (`OperatorMeta`), тип переименовывается. Hot path получает `meta.Currency`, передаёт в `saga.Charge`. Пустой currency (orphan оператор без страны) → fallback на legacy с новой меткой `currency_resolve_error`. Replay idempotency делает дополнительный cached lookup вместо schema-migration. Schema не меняется.

**Tech Stack:** Go 1.24, sqlx, `github.com/google/uuid`, prometheus. Базовая ветка — `feat/tarification-log-nullable-plan` (не master); работа предполагает что nullable plan/period + SourceRuleID pointers уже есть.

---

## File Structure

**Модифицировать:**
- `internal/services/tarification/application/operator_lookup.go` — rename interface, add `OperatorMeta` type, rename `Code()` → `Meta()`, cache stores `OperatorMeta`.
- `internal/services/tarification/application/operator_lookup_test.go` — обновить тесты под новый контракт.
- `internal/services/tarification/infrastructure/repository/operator_repository.go` — SQL `LEFT JOIN countries`, `GetMetaByID` replaces `GetCodeByID`.
- `internal/services/tarification/application/unified_path.go` — `Meta()` вместо `Code()`, `meta.Currency` вместо `"RUB"`, новая fallback-ветка, replay currency lookup.
- `internal/services/tarification/application/unified_path_test.go` — `stubOperatorLookup` возвращает `OperatorMeta`, новый тест `TestTarifyUnified_EmptyCurrency_Fallback`.
- `internal/services/tarification/application/tarification_service.go` — `SetUnifiedDependencies` parameter type rename; outer idempotency short-circuit резолвит currency через unified lookup.

**Без изменений:**
- `cmd/services/tarification-service/main.go` — `NewCachedOperatorLookup(operatorRepo)` signature остаётся inferred; compile-check в задаче.

**Не создавать:** миграций нет.

---

## Task 1: Extend operator_lookup.go — OperatorMeta + Meta() method

**Files:**
- Modify: `internal/services/tarification/application/operator_lookup.go`
- Modify: `internal/services/tarification/application/operator_lookup_test.go`

### Step 1: Replace the file contents

Replace `internal/services/tarification/application/operator_lookup.go` entirely with:

```go
// operator_lookup.go
package application

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// OperatorMeta — метаданные оператора, необходимые tarification hot path.
// Currency источник — countries.currency по operator.country_id.
type OperatorMeta struct {
	Code     string
	Currency string
}

// OperatorMetaLookup — абстракция резолва UUID → OperatorMeta.
type OperatorMetaLookup interface {
	Meta(ctx context.Context, operatorID uuid.UUID) (OperatorMeta, error)
}

// operatorMetaSource — минимальный интерфейс, нужный кешу.
type operatorMetaSource interface {
	GetMetaByID(ctx context.Context, id uuid.UUID) (OperatorMeta, error)
}

// CachedOperatorLookup — in-memory map без TTL. Операторы — справочник
// (десятки записей), код/страна иммутабельны в пределах инстанса, инвалидация
// через рестарт сервиса.
type CachedOperatorLookup struct {
	inner operatorMetaSource
	mu    sync.RWMutex
	cache map[uuid.UUID]OperatorMeta
}

// NewCachedOperatorLookup создаёт lookup с пустым кешем.
func NewCachedOperatorLookup(inner operatorMetaSource) *CachedOperatorLookup {
	return &CachedOperatorLookup{
		inner: inner,
		cache: make(map[uuid.UUID]OperatorMeta),
	}
}

// Meta возвращает OperatorMeta для заданного UUID.
// При кеш-хите — возвращает без обращения к БД.
// При ошибке inner — НЕ кеширует результат, следующий вызов повторит запрос.
func (c *CachedOperatorLookup) Meta(ctx context.Context, id uuid.UUID) (OperatorMeta, error) {
	c.mu.RLock()
	if meta, ok := c.cache[id]; ok {
		c.mu.RUnlock()
		return meta, nil
	}
	c.mu.RUnlock()

	meta, err := c.inner.GetMetaByID(ctx, id)
	if err != nil {
		return OperatorMeta{}, err
	}
	c.mu.Lock()
	c.cache[id] = meta
	c.mu.Unlock()
	return meta, nil
}
```

### Step 2: Replace the test file contents

Replace `internal/services/tarification/application/operator_lookup_test.go` entirely with:

```go
// operator_lookup_test.go
package application

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubOpRepo struct {
	calls int64
	metas map[uuid.UUID]OperatorMeta
	err   error
}

func (s *stubOpRepo) GetMetaByID(_ context.Context, id uuid.UUID) (OperatorMeta, error) {
	atomic.AddInt64(&s.calls, 1)
	if s.err != nil {
		return OperatorMeta{}, s.err
	}
	m, ok := s.metas[id]
	if !ok {
		return OperatorMeta{}, errors.New("not found")
	}
	return m, nil
}

func TestCachedOperatorLookup_HitAfterFirstCall(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{metas: map[uuid.UUID]OperatorMeta{id: {Code: "mts-ru", Currency: "RUB"}}}
	l := NewCachedOperatorLookup(repo)

	for i := 0; i < 10; i++ {
		meta, err := l.Meta(context.Background(), id)
		require.NoError(t, err)
		require.Equal(t, "mts-ru", meta.Code)
		require.Equal(t, "RUB", meta.Currency)
	}
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_MissForNewID(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	repo := &stubOpRepo{metas: map[uuid.UUID]OperatorMeta{
		id1: {Code: "a", Currency: "RUB"},
		id2: {Code: "b", Currency: "KZT"},
	}}
	l := NewCachedOperatorLookup(repo)

	_, _ = l.Meta(context.Background(), id1)
	_, _ = l.Meta(context.Background(), id2)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_ErrorNotCached(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{err: errors.New("db down")}
	l := NewCachedOperatorLookup(repo)

	_, err := l.Meta(context.Background(), id)
	require.Error(t, err)

	repo.err = nil
	repo.metas = map[uuid.UUID]OperatorMeta{id: {Code: "x", Currency: "RUB"}}
	meta, err := l.Meta(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "x", meta.Code)
	require.Equal(t, "RUB", meta.Currency)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_EmptyCurrencyCached(t *testing.T) {
	// Meta с пустой currency — валидный результат (orphan operator), должен кешироваться.
	// Проверка отказа от cache'а для Currency=="" привела бы к storm DB calls.
	id := uuid.New()
	repo := &stubOpRepo{metas: map[uuid.UUID]OperatorMeta{id: {Code: "orphan", Currency: ""}}}
	l := NewCachedOperatorLookup(repo)

	for i := 0; i < 5; i++ {
		meta, err := l.Meta(context.Background(), id)
		require.NoError(t, err)
		require.Equal(t, "orphan", meta.Code)
		require.Equal(t, "", meta.Currency)
	}
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_ConcurrentNoRace(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{metas: map[uuid.UUID]OperatorMeta{id: {Code: "mts-ru", Currency: "RUB"}}}
	l := NewCachedOperatorLookup(repo)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = l.Meta(context.Background(), id)
		}()
	}
	wg.Wait()
	// gate: race detector должен пройти (go test -race).
}
```

### Step 3: Run tests — expect FAIL for unified_path tests (uses old API)

Run: `go test -race ./internal/services/tarification/application/ -run TestCachedOperatorLookup -v`
Expected: 5 PASS (all new tests).

Run: `go build ./internal/services/tarification/...`
Expected: FAIL in `unified_path.go` (uses `.Code()`), `unified_path_test.go`, `tarification_service.go`, `main.go`. Fixed in subsequent tasks.

Device Guard may SKIP Go locally — rely on CI.

### Step 4: Commit

```bash
git add internal/services/tarification/application/operator_lookup.go internal/services/tarification/application/operator_lookup_test.go
git commit -m "refactor(tarification): OperatorCodeLookup → OperatorMetaLookup

Returns OperatorMeta{Code, Currency} instead of just code string.
Cache stores the struct. Wider package compile will break until
downstream tasks rewire callers — intentional.

See docs/superpowers/specs/2026-04-19-unified-currency-via-operator-meta-design.md.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 2: Repository GetMetaByID with operators→countries JOIN

**Files:**
- Modify: `internal/services/tarification/infrastructure/repository/operator_repository.go`

### Step 1: Replace file contents

Replace `internal/services/tarification/infrastructure/repository/operator_repository.go` entirely with:

```go
// operator_repository.go
package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// OperatorRepository — тонкий reader для operators + countries JOIN.
// operators и countries — shared-reference-data; tarification читает их
// напрямую, чтобы не вводить gRPC-зависимость для read-only lookup'а.
type OperatorRepository struct {
	db *sqlx.DB
}

// NewOperatorRepository создаёт репозиторий с готовым sqlx-пулом.
func NewOperatorRepository(db *sqlx.DB) *OperatorRepository {
	return &OperatorRepository{db: db}
}

const getOperatorMetaSQL = `
  SELECT o.code, COALESCE(c.currency, '')
  FROM operators o
  LEFT JOIN countries c ON c.id = o.country_id
  WHERE o.id = $1
`

// GetMetaByID возвращает Code + Currency для заданного UUID.
// LEFT JOIN + COALESCE гарантирует, что оператор без country или с
// country.currency=NULL вернётся с Currency="" — невыход в ошибку, чтобы
// вызывающий мог отдельно решить fallback-стратегию.
// Возвращает domain.ErrOperatorNotFound, если оператор вовсе не найден.
func (r *OperatorRepository) GetMetaByID(ctx context.Context, id uuid.UUID) (application.OperatorMeta, error) {
	var m application.OperatorMeta
	err := r.db.QueryRowxContext(ctx, getOperatorMetaSQL, id).Scan(&m.Code, &m.Currency)
	if errors.Is(err, sql.ErrNoRows) {
		return application.OperatorMeta{}, domain.ErrOperatorNotFound
	}
	return m, err
}
```

**Кросс-package note:** infrastructure/repository импортирует application — этот паттерн уже используется в том же пакете (см. `tariff_plan_repository.go` для `domain.TariffPlan` — там ок, но здесь мы импортируем ИЗ application, а не только domain). Это приемлемо: `OperatorMeta` — value-type без поведения, alternative (дублировать тип в domain) добавляет код без value.

### Step 2: Run build

Run: `go build ./internal/services/tarification/infrastructure/repository/...`
Expected: PASS (может SKIP локально). Если компилятор ругается на circular import `application ↔ repository` — это регрессия плана, остановиться и доложить.

### Step 3: Commit

```bash
git add internal/services/tarification/infrastructure/repository/operator_repository.go
git commit -m "feat(tarification): OperatorRepository.GetMetaByID joins countries for currency

LEFT JOIN + COALESCE: operator without country (or country.currency=NULL)
returns Currency='' rather than error — callers decide fallback policy.
sql.ErrNoRows still maps to domain.ErrOperatorNotFound.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 3: unified_path.go — use Meta, drop hardcoded RUB, fix replay

**Files:**
- Modify: `internal/services/tarification/application/unified_path.go`

### Step 1: Update `unifiedDeps` struct — field type `operatorLookup`

Find:
```go
	operatorLookup OperatorCodeLookup
```

Replace with:
```go
	operatorLookup OperatorMetaLookup
```

### Step 2: Replace the operator-lookup step (around line 65-72)

Current:
```go
	// 3. operator_id → operators.code (Task 0).
	operatorCode, err := d.operatorLookup.Code(ctx, req.OperatorID)
	if err != nil {
		unifiedFallbackTotal.WithLabelValues("operator_lookup_error").Inc()
		log.Warn().Err(err).Str("operator_id", req.OperatorID.String()).
			Msg("unified: operator code lookup failed — fallback")
		return nil, "operator_lookup_error", nil
	}
```

Replace with:
```go
	// 3. operator_id → OperatorMeta (code + currency from operators→countries JOIN).
	meta, err := d.operatorLookup.Meta(ctx, req.OperatorID)
	if err != nil {
		unifiedFallbackTotal.WithLabelValues("operator_lookup_error").Inc()
		log.Warn().Err(err).Str("operator_id", req.OperatorID.String()).
			Msg("unified: operator meta lookup failed — fallback")
		return nil, "operator_lookup_error", nil
	}
	if meta.Currency == "" {
		unifiedFallbackTotal.WithLabelValues("currency_resolve_error").Inc()
		log.Warn().Str("operator_id", req.OperatorID.String()).
			Msg("unified: operator has no currency (orphan country) — fallback")
		return nil, "currency_resolve_error", nil
	}
	operatorCode := meta.Code
```

### Step 3: Replace step 7 (charge) currency assignment

Current (around line 122-129):
```go
	// 7. Charge — after this point fallback is unsafe.
	// NOTE: double-charge on retry is prevented by billing-service idempotency
	// on message_id (see billing_service.go:425, GetByMessageID short-circuit).
	// If step 10 (tarification_log Create) later fails, a client retry with
	// the same idempotency key will miss step 1's short-circuit, re-enter
	// tarifyUnified, and billing will return the existing transaction
	// instead of charging twice.
	currency := "RUB" // unified model does not denormalize currency per rule; platform default. Non-RUB accounts will receive billing currency-mismatch errors → fallback to legacy. Resolve via resolved_rule in a follow-up.
	chargeResult, err := d.saga.Charge(ctx,
```

Replace with:
```go
	// 7. Charge — after this point fallback is unsafe.
	// NOTE: double-charge on retry is prevented by billing-service idempotency
	// on message_id (see billing_service.go:425, GetByMessageID short-circuit).
	// If step 10 (tarification_log Create) later fails, a client retry with
	// the same idempotency key will miss step 1's short-circuit, re-enter
	// tarifyUnified, and billing will return the existing transaction
	// instead of charging twice.
	currency := meta.Currency
	chargeResult, err := d.saga.Charge(ctx,
```

### Step 4: Replace idempotency short-circuit (top of tarifyUnified)

The nullable-plan branch already has a switch handling `TariffPlanID` vs `SourceRuleID`. Extend it with currency lookup.

Current:
```go
	} else if existing != nil {
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
	}
```

Replace with:
```go
	} else if existing != nil {
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
		// Currency для replay: cached lookup. Fallback на RUB если падает —
		// replay-ответ не money-критичен, billing валидирует независимо.
		replayCurrency := "RUB"
		if replayMeta, metaErr := d.operatorLookup.Meta(ctx, existing.OperatorID); metaErr == nil && replayMeta.Currency != "" {
			replayCurrency = replayMeta.Currency
		}
		return &TarifyMessageResponse{
			Approved:     true,
			TotalAmount:  existing.TotalAmount,
			Currency:     replayCurrency,
			Strategy:     string(existing.Strategy),
			TariffPlanID: tariffPlanID,
		}, "", nil
	}
```

### Step 5: Verify no remaining `.Code(` calls in the file

Run: `grep -n 'operatorLookup.Code(' internal/services/tarification/application/unified_path.go`
Expected: no output.

### Step 6: Run build

Run: `go build ./internal/services/tarification/application/...`
Expected: compile fails in `unified_path_test.go` and `tarification_service.go` (tests use old `stubOperatorLookup.Code()`, service uses `OperatorCodeLookup` as a type). Fixed in Tasks 4-5.

### Step 7: Commit

```bash
git add internal/services/tarification/application/unified_path.go
git commit -m "feat(tarification): unified hot path resolves currency via OperatorMetaLookup

- Drops hardcoded currency := \"RUB\". meta.Currency flows into saga.Charge.
- New fallback reason currency_resolve_error for orphan operators
  (country_id NULL or country.currency NULL).
- Idempotency replay looks up currency via cached Meta() instead of
  hardcoding RUB; falls back to RUB on lookup failure (response-only,
  billing revalidates on retry).

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 4: unified_path_test.go — stubs return OperatorMeta, add empty-currency test

**Files:**
- Modify: `internal/services/tarification/application/unified_path_test.go`

### Step 1: Replace the `stubOperatorLookup` type definition

Find (around line 65-74):
```go
type stubOperatorLookup struct {
	code  string
	err   error
	calls int
}

func (s *stubOperatorLookup) Code(_ context.Context, _ uuid.UUID) (string, error) {
	s.calls++
	return s.code, s.err
}
```

Replace with:
```go
type stubOperatorLookup struct {
	meta  OperatorMeta
	err   error
	calls int
}

func (s *stubOperatorLookup) Meta(_ context.Context, _ uuid.UUID) (OperatorMeta, error) {
	s.calls++
	return s.meta, s.err
}
```

### Step 2: Update all call-sites that construct `stubOperatorLookup{code: "..."}`

Every `stubOperatorLookup{code: "mts-ru"}` literal becomes `stubOperatorLookup{meta: OperatorMeta{Code: "mts-ru", Currency: "RUB"}}`.

Every `stubOperatorLookup{err: ...}` literal stays as-is (err field unchanged).

Use multi-line replace — find-and-replace of the pattern `stubOperatorLookup{code: "mts-ru"}` in all contexts (happy path, idempotency, charge-fail, calc-error tests). Typical literals in this file:
- Line 158: `opLookup := &stubOperatorLookup{code: "mts-ru"}` → `opLookup := &stubOperatorLookup{meta: OperatorMeta{Code: "mts-ru", Currency: "RUB"}}`
- Line 234: `operatorLookup: &stubOperatorLookup{code: "mts-ru"}` → `operatorLookup: &stubOperatorLookup{meta: OperatorMeta{Code: "mts-ru", Currency: "RUB"}}`
- Line 286: same shape as 234.
- Line 351: same as line 158.
- Line 400: `opLookup := &stubOperatorLookup{err: domain.ErrOperatorNotFound}` — **leave unchanged**, error case still works.
- Line 411: same as 400.

After: all happy-path stubs carry `OperatorMeta{Code: "mts-ru", Currency: "RUB"}`.

### Step 3: In idempotency-short-circuit test, assert `resp.Currency == "RUB"`

Locate `TestTarifyUnified_IdempotencyShortCircuits`. Its setup should already pass a stub lookup. Current assertion on response (likely):
```go
require.Equal(t, "RUB", resp.Currency)
```

Verify it exists and matches. If the test assertion is `require.Equal(t, "RUB", resp.Currency)` already — no change needed; new code now reaches that answer legitimately via lookup. Add assertion `require.Equal(t, 1, opLookup.calls)` or similar **only** if lookup stub is bound to the deps of this test (check the test's local structure and add minimally).

### Step 4: Add new test `TestTarifyUnified_EmptyCurrency_Fallback`

Insert after `TestTarifyUnified_OperatorLookupError_Fallback` (or near end of file):

```go
func TestTarifyUnified_EmptyCurrency_Fallback(t *testing.T) {
	ctx := context.Background()
	before := testutil.ToFloat64(unifiedFallbackTotal.WithLabelValues("currency_resolve_error"))

	// Stub returns meta with empty Currency — orphan operator scenario.
	opLookup := &stubOperatorLookup{meta: OperatorMeta{Code: "orphan", Currency: ""}}
	logRepo := &stubLogRepo{}
	ruleRepo := &callCountingRuleRepo{}
	deps := &unifiedDeps{
		resolver:       NewPriceResolver(ruleRepo, &stubResolvedRepo{}, &stubVersionRepo{}, &stubAggResolver{}),
		calc:           NewCostCalculator(NewTiersCache(16)),
		ruleRepo:       ruleRepo,
		subUsageRepo:   &stubSubUsage{},
		marginLogRepo:  &stubMarginLog{},
		saga:           &stubSaga{},
		logRepo:        logRepo,
		senderRepo:     &stubSenderRepo{},
		operatorLookup: opLookup,
	}
	req := &TarifyMessageRequest{
		ClientID:       uuid.New(),
		MessageID:      uuid.New(),
		OperatorID:     uuid.New(),
		SegmentCount:   1,
		IdempotencyKey: "idem-empty-currency",
	}

	resp, fallback, err := tarifyUnified(ctx, req, deps)

	require.NoError(t, err)
	require.Nil(t, resp)
	require.Equal(t, "currency_resolve_error", fallback)
	after := testutil.ToFloat64(unifiedFallbackTotal.WithLabelValues("currency_resolve_error"))
	require.InDelta(t, 1.0, after-before, 0.001)
	// rule repo NOT called — fallback fires before resolve
	require.Equal(t, 0, ruleRepo.calls)
}
```

Imports used:
- `testutil "github.com/prometheus/client_golang/prometheus/testutil"` — already in file.
- Other stubs (`stubResolvedRepo`, `stubVersionRepo`, `stubAggResolver`, `stubSubUsage`, `stubMarginLog`, `stubSaga`, `stubSenderRepo`, `callCountingRuleRepo`, `stubLogRepo`) — already defined in the file.

**If any stub constructor has different exported form in current test file, adapt inline. Do NOT invent new stub types.** Before writing, grep the file for existing stub literals to copy the shape:

Run: `grep -n 'stub.*{$\|= &stub' internal/services/tarification/application/unified_path_test.go`
Use the pattern observed in existing tests (e.g. `TestTarifyUnified_FallbackOnNotFound`).

### Step 5: Run tests

Run: `go test -race ./internal/services/tarification/application/ -run TestTarifyUnified -v`
Expected: all existing TestTarifyUnified tests PASS + new TestTarifyUnified_EmptyCurrency_Fallback PASS. 7 tests total.

Device Guard may SKIP locally — rely on CI.

### Step 6: Commit

```bash
git add internal/services/tarification/application/unified_path_test.go
git commit -m "test(tarification): stub OperatorMeta + empty-currency fallback test

- stubOperatorLookup.Code → Meta, carries OperatorMeta{Code, Currency}
- happy-path stubs set Currency=\"RUB\"
- new TestTarifyUnified_EmptyCurrency_Fallback asserts empty-currency
  triggers fallback reason currency_resolve_error and the rule repo
  is never called

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 5: tarification_service.go — setter rename, outer short-circuit currency

**Files:**
- Modify: `internal/services/tarification/application/tarification_service.go`

### Step 1: Update `SetUnifiedDependencies` parameter type

Find:
```go
func (s *TarificationService) SetUnifiedDependencies(
	enabled bool,
	rollout *Rollout,
	resolver *PriceResolver,
	calc *CostCalculator,
	ruleRepo domain.PriceRuleRepository,
	subUsageRepo domain.SubaccountUsageCounterRepository,
	operatorLookup OperatorCodeLookup,
) {
```

Replace parameter type `OperatorCodeLookup` → `OperatorMetaLookup`:
```go
func (s *TarificationService) SetUnifiedDependencies(
	enabled bool,
	rollout *Rollout,
	resolver *PriceResolver,
	calc *CostCalculator,
	ruleRepo domain.PriceRuleRepository,
	subUsageRepo domain.SubaccountUsageCounterRepository,
	operatorLookup OperatorMetaLookup,
) {
```

Body unchanged — just passes the lookup into `unifiedDeps`.

### Step 2: Update outer idempotency short-circuit (unified replay currency)

Find the nullable-plan branch's switch block inside `TarifyMessage`:

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

Replace the unified branch to resolve currency via lookup (falls back to "RUB" only if lookup fails):

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
			// Unified replay: резолвим currency через тот же cached lookup,
			// что использовался при первом вызове; fallback на RUB если lookup
			// недоступен (response-only, billing перевалидирует).
			existingCurrency = "RUB"
			if s.unifiedDeps != nil && s.unifiedDeps.operatorLookup != nil {
				if meta, metaErr := s.unifiedDeps.operatorLookup.Meta(ctx, existing.OperatorID); metaErr == nil && meta.Currency != "" {
					existingCurrency = meta.Currency
				}
			}
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

**Sanity:** `s.unifiedDeps` is `*unifiedDeps` — non-nil check guards from nil-pointer deref when SetUnifiedDependencies wasn't called (service still usable for legacy-only deploys). `operatorLookup` field inside is `OperatorMetaLookup` — non-nil check via interface compare: if the interface header is zero-value (never assigned), comparison `!= nil` is `false`. Wait — interface comparison to nil is subtle; the setter always writes a concrete value, so if `unifiedDeps` non-nil, `operatorLookup` non-nil too. The double check is defensive belt-and-suspenders — acceptable.

### Step 3: Run build

Run: `go build ./internal/services/tarification/application/...`
Expected: PASS.

Run: `go build ./cmd/services/tarification-service/...`
Expected: PASS (main.go's `NewCachedOperatorLookup` returns `*CachedOperatorLookup` which now implements `OperatorMetaLookup` — no code change needed there).

### Step 4: Run all tarification tests

Run: `go test ./internal/services/tarification/... -count=1`
Expected: all PASS. If `TestTarificationService_TarifyMessage_IdempotencyReplay_Unified` or similar service-level test exists and asserts `resp.Currency == "RUB"` — it stays valid (stub environment, no unifiedDeps set, fallback path hits "RUB").

Device Guard may SKIP — rely on CI.

### Step 5: Commit

```bash
git add internal/services/tarification/application/tarification_service.go
git commit -m "feat(tarification): outer idempotency short-circuit resolves unified currency

SetUnifiedDependencies now takes OperatorMetaLookup (renamed from
OperatorCodeLookup). Unified replay path in TarifyMessage step 1
calls the same cached lookup instead of hardcoding RUB; falls back
to RUB on lookup failure (response-only).

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 6: Sandbox smoke verify

No code change. Deploy the branch to sandbox, apply nothing (no migration), restart tarification-service, verify startup logs show no fatal/panic/error and `unified_enabled` remains false (sandbox default). Confirms the rename didn't break wiring.

- [ ] **Step 1: Push branch to origin**

```bash
git push -u origin feat/unified-currency-via-operator-meta
```

- [ ] **Step 2: Deploy**

```bash
DEPLOY_BRANCH=feat/unified-currency-via-operator-meta ./scripts/server.sh sync
DEPLOY_BRANCH=feat/unified-currency-via-operator-meta ./scripts/server.sh deploy tarification-service
```

- [ ] **Step 3: Verify logs**

Wait 15s. Then:
```bash
./scripts/server.sh exec "docker compose -f /opt/sms/deployments/docker-compose.yml logs tarification-service --tail=50"
```

Expected: `запуск Tarification Service` ... `gRPC сервер запущен`. No `FATAL`, no `panic`, no `ERR`.

- [ ] **Step 4: If green, no commit — just note result**

No commit — this is a verification-only task. Report OK / FAIL.

---

## Self-review

**1. Spec coverage:**
- D1 (extended lookup): Tasks 1 + 2 ✅
- D2 (empty currency fallback): Tasks 3 (unified path branch) + 4 (test) ✅
- D3 (replay lookup, no schema change): Task 3 step 4 (inner) + Task 5 step 2 (outer) ✅
- SetUnifiedDependencies type rename: Task 5 step 1 ✅
- Sandbox verify: Task 6 ✅
- Frontend update: not in scope per spec.
- Tests: Tasks 1 (lookup unit tests) + 4 (hot-path stubs) ✅

**2. Placeholder scan:** нет TBD/TODO/fill-in. Каждый шаг содержит конкретный код.

**3. Type consistency:**
- `OperatorMeta{Code, Currency}` — идентично в Task 1, 2, 3, 4.
- `OperatorMetaLookup.Meta(ctx, uuid) (OperatorMeta, error)` — один signature в Task 1, используется в Task 3, 4, 5.
- `operatorMetaSource.GetMetaByID(ctx, uuid) (OperatorMeta, error)` — Task 1 ↔ Task 2 (repo имплементит).

**4. Existing-code adaptation reminder:** Task 4 Step 2 предполагает что test-file имеет текущие literals в ожидаемой форме. Подагент должен grep'ать файл перед заменой, не полагаться слепо на номера строк.

---

## Execution handoff

**Plan complete and saved to `docs/superpowers/plans/2026-04-19-unified-currency-via-operator-meta.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task + two-stage review. 6 tasks, each with clear scope.

**2. Inline Execution** — batch через executing-plans.

Рекомендую **Subagent-Driven**. Каждый коммит — через `/execute-with-review` wrapper.
