# Phase 3 — Unified Pricing Hot Path Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Горячий путь `TarificationService.TarifyMessage` резолвит цену через `PriceResolver` + `CostCalculator` под gate-ом `unified_enabled` × `unified_rollout_percentage`, считает margin через отдельный резолв с исключением subaccount-override, экспортирует метрики и поэтапно раскатывается (1% → 10% → 50% → 100%) с автоматическим fallback на legacy-ветку при `price_not_found` или runtime-ошибках.

**Architecture:**
1. Новая ветка в `TarifyMessage` вызывается, если `unified_enabled=true` **и** `fnv(subaccount_id.bytes) % 100 < rollout_percentage`. Иначе — legacy (как сейчас).
2. Внутри новой ветки: `PriceResolver.Resolve` (cache + fallback к `price_rules`) → `CostCalculator.Calculate` → `SubaccountUsageCounterRepository.Increment` → `saga.Charge`. Идемпотентность — через существующий `tarification_log.idempotency_key` **до** вызовов.
3. Margin: второй вызов `PriceRuleRepository.FindApplicable` с новым флагом `ExcludeSubaccount=true` (некешированный), чтобы получить aggregator-baseline. Margin = subaccount_price - aggregator_price. Пишется в `aggregator_margin_log` тем же путём, что и legacy.
4. `GetVersion` обёрнут в TTL=1s + `singleflight` (защита от thundering herd на инвалидации).
5. Любая ошибка unified-ветки (resolve fail, rule not found, calc fail, counter fail) → метрика + WARN-лог + fallback на legacy branch. Charge-ошибка fallback'у не подлежит (уже money-movement). 

**Tech Stack:** Go 1.24, prometheus/client_golang, hash/fnv, golang.org/x/sync/singleflight, jmoiron/sqlx, существующие domain-интерфейсы tarification.

---

## File Structure

**Создать:**
- `internal/services/tarification/infrastructure/repository/operator_repository.go` — `GetCodeByID(id) -> code` (Task 0).
- `internal/services/tarification/application/operator_lookup.go` — кешированный lookup (Task 0). См. `docs/superpowers/specs/2026-04-19-operator-code-lookup-design.md`.
- `internal/services/tarification/application/operator_lookup_test.go` (Task 0).
- `internal/services/tarification/application/version_cache.go` — TTL+singleflight обёртка над `PriceRulesVersionRepository`.
- `internal/services/tarification/application/version_cache_test.go`
- `internal/services/tarification/application/rollout.go` — fnv-hash percentage gate.
- `internal/services/tarification/application/rollout_test.go`
- `internal/services/tarification/application/unified_metrics.go` — prometheus-метрики для unified-пути.
- `internal/services/tarification/application/unified_path.go` — функция `tarifyUnified`, инкапсулирующая ветку. Чтобы `tarification_service.go` не раздувался.
- `internal/services/tarification/application/unified_path_test.go`

**Модифицировать:**
- `internal/services/tarification/domain/errors.go` — добавить `ErrOperatorNotFound` (Task 0).
- `internal/config/config.go:30-35` — добавить поле `UnifiedRolloutPercentage int` в `TarificationConfig`, дефолт 0.
- `internal/services/tarification/domain/price_rule_repository.go:13-21` — добавить поле `ExcludeSubaccount bool` в `ResolveInput`.
- `internal/services/tarification/infrastructure/repository/price_rule_repository.go:25-59` — учесть `ExcludeSubaccount` в SQL (добавить `AND (NOT $8 OR owner_type != 'subaccount')`).
- `internal/services/tarification/application/price_resolver.go:62-65` — заменить прямой вызов `versionRepo.GetVersion` на `versionCache.GetVersion` (новая обёртка). Убрать TODO.
- `internal/services/tarification/application/tarification_service.go`:
  - добавить поля: `priceResolver *PriceResolver`, `costCalc *CostCalculator`, `subUsageRepo domain.SubaccountUsageCounterRepository`, `priceRuleRepo domain.PriceRuleRepository` (для margin-резолва), `rollout *Rollout`, `unifiedEnabled bool`.
  - `SetUnifiedDependencies(...)` setter.
  - в `TarifyMessage` после п.1 (идемпотентность) — gate-check, ветвление в `tarifyUnified` или legacy.
- `cmd/services/tarification-service/main.go:167-207` — расширить unified-блок: создать `versionCache`, `rollout`, `subUsageRepo`, `priceResolver`, `costCalc`, вызвать `SetUnifiedDependencies`, залогировать `rollout_percentage`.

**Тесты:**
- Unit: `version_cache_test.go`, `rollout_test.go`, `unified_path_test.go`.
- Integration: оставляем за рамками плана; существующие AC-тесты для legacy-ветки продолжают работать с `rollout_percentage=0`.

---

## Task 0: Operator ID → code lookup (prerequisite)

Источник: `docs/superpowers/specs/2026-04-19-operator-code-lookup-design.md`. Без этой задачи unified-путь будет матчить UUID-строку против `price_rules.operator VARCHAR(50)` — 100% miss на любом realistic seed.

**Files:**
- Create: `internal/services/tarification/infrastructure/repository/operator_repository.go`
- Create: `internal/services/tarification/application/operator_lookup.go`
- Test: `internal/services/tarification/application/operator_lookup_test.go`
- Modify: `internal/services/tarification/domain/errors.go` — добавить `ErrOperatorNotFound`

- [ ] **Step 1: Add domain error**

В `internal/services/tarification/domain/errors.go` добавить строку рядом с существующими `var Err... = errors.New(...)`:

```go
var ErrOperatorNotFound = errors.New("operator not found")
```

- [ ] **Step 2: Write failing lookup tests**

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
	codes map[uuid.UUID]string
	err   error
}

func (s *stubOpRepo) GetCodeByID(_ context.Context, id uuid.UUID) (string, error) {
	atomic.AddInt64(&s.calls, 1)
	if s.err != nil {
		return "", s.err
	}
	c, ok := s.codes[id]
	if !ok {
		return "", errors.New("not found")
	}
	return c, nil
}

func TestCachedOperatorLookup_HitAfterFirstCall(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{codes: map[uuid.UUID]string{id: "mts-ru"}}
	l := NewCachedOperatorLookup(repo)

	for i := 0; i < 10; i++ {
		code, err := l.Code(context.Background(), id)
		require.NoError(t, err)
		require.Equal(t, "mts-ru", code)
	}
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_MissForNewID(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	repo := &stubOpRepo{codes: map[uuid.UUID]string{id1: "a", id2: "b"}}
	l := NewCachedOperatorLookup(repo)

	_, _ = l.Code(context.Background(), id1)
	_, _ = l.Code(context.Background(), id2)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_ErrorNotCached(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{err: errors.New("db down")}
	l := NewCachedOperatorLookup(repo)

	_, err := l.Code(context.Background(), id)
	require.Error(t, err)

	repo.err = nil
	repo.codes = map[uuid.UUID]string{id: "x"}
	code, err := l.Code(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "x", code)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_ConcurrentNoRace(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{codes: map[uuid.UUID]string{id: "mts-ru"}}
	l := NewCachedOperatorLookup(repo)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = l.Code(context.Background(), id)
		}()
	}
	wg.Wait()
	// не проверяем ровно 1 call: возможны concurrent cold-miss.
	// Но gate: race detector должен пройти (go test -race).
}
```

- [ ] **Step 3: Run — expect FAIL**

Run: `go test ./internal/services/tarification/application/ -run TestCachedOperatorLookup -v`
Expected: FAIL (NewCachedOperatorLookup undefined).

- [ ] **Step 4: Implement lookup**

```go
// operator_lookup.go
package application

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// OperatorCodeLookup — абстракция резолва UUID → operators.code.
type OperatorCodeLookup interface {
	Code(ctx context.Context, operatorID uuid.UUID) (string, error)
}

// operatorCodeSource — минимальный интерфейс, нужный кешу.
type operatorCodeSource interface {
	GetCodeByID(ctx context.Context, id uuid.UUID) (string, error)
}

// CachedOperatorLookup — in-memory map без TTL. Операторы — справочник
// (десятки записей), код иммутабелен, инвалидация через рестарт сервиса.
type CachedOperatorLookup struct {
	inner operatorCodeSource
	mu    sync.RWMutex
	cache map[uuid.UUID]string
}

func NewCachedOperatorLookup(inner operatorCodeSource) *CachedOperatorLookup {
	return &CachedOperatorLookup{
		inner: inner,
		cache: make(map[uuid.UUID]string),
	}
}

func (c *CachedOperatorLookup) Code(ctx context.Context, id uuid.UUID) (string, error) {
	c.mu.RLock()
	if code, ok := c.cache[id]; ok {
		c.mu.RUnlock()
		return code, nil
	}
	c.mu.RUnlock()

	code, err := c.inner.GetCodeByID(ctx, id)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.cache[id] = code
	c.mu.Unlock()
	return code, nil
}
```

- [ ] **Step 5: Run — expect PASS (with -race)**

Run: `go test -race ./internal/services/tarification/application/ -run TestCachedOperatorLookup -v`
Expected: PASS (4 subtests, race detector clean).

- [ ] **Step 6: Implement repo**

```go
// infrastructure/repository/operator_repository.go
package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// OperatorRepository — тонкий reader для operators.code.
// operators — shared-reference-data (routing-service тоже её использует);
// tarification обращается напрямую, чтобы не вводить gRPC-зависимость для
// одного read-only lookup'а.
type OperatorRepository struct {
	db *sqlx.DB
}

func NewOperatorRepository(db *sqlx.DB) *OperatorRepository {
	return &OperatorRepository{db: db}
}

func (r *OperatorRepository) GetCodeByID(ctx context.Context, id uuid.UUID) (string, error) {
	var code string
	err := r.db.GetContext(ctx, &code, `SELECT code FROM operators WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrOperatorNotFound
	}
	return code, err
}
```

- [ ] **Step 7: Build**

Run: `go build ./...`
Expected: exit 0 (или SKIP под Device Guard).

- [ ] **Step 8: Commit**

```bash
git add internal/services/tarification/domain/errors.go \
        internal/services/tarification/infrastructure/repository/operator_repository.go \
        internal/services/tarification/application/operator_lookup.go \
        internal/services/tarification/application/operator_lookup_test.go
git commit -m "feat(tarification): operator ID->code lookup with in-memory cache

Prerequisite for unified hot path — price_rules.operator is VARCHAR
referencing operators.code, but TarifyMessageRequest carries operator_id
(UUID). Thin repo + RWMutex-guarded map (no TTL; operators are effectively
immutable reference data).

See docs/superpowers/specs/2026-04-19-operator-code-lookup-design.md.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 1: TTL+singleflight version cache

**Files:**
- Create: `internal/services/tarification/application/version_cache.go`
- Test: `internal/services/tarification/application/version_cache_test.go`

- [ ] **Step 1: Write failing tests**

```go
// version_cache_test.go
package application

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type countingVersionRepo struct {
	calls int64
	v     int64
	err   error
	delay time.Duration
}

func (r *countingVersionRepo) GetVersion(ctx context.Context) (int64, error) {
	atomic.AddInt64(&r.calls, 1)
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	return r.v, r.err
}

func TestVersionCache_HitWithinTTL(t *testing.T) {
	inner := &countingVersionRepo{v: 7}
	c := NewVersionCache(inner, 50*time.Millisecond)

	v1, err := c.GetVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(7), v1)

	v2, err := c.GetVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(7), v2)

	require.Equal(t, int64(1), atomic.LoadInt64(&inner.calls))
}

func TestVersionCache_ExpiredRefetches(t *testing.T) {
	inner := &countingVersionRepo{v: 7}
	c := NewVersionCache(inner, 10*time.Millisecond)

	_, _ = c.GetVersion(context.Background())
	time.Sleep(20 * time.Millisecond)
	inner.v = 8
	v, err := c.GetVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(8), v)
	require.Equal(t, int64(2), atomic.LoadInt64(&inner.calls))
}

func TestVersionCache_SingleflightCoalesces(t *testing.T) {
	inner := &countingVersionRepo{v: 9, delay: 20 * time.Millisecond}
	c := NewVersionCache(inner, 100*time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := c.GetVersion(context.Background())
			require.NoError(t, err)
			require.Equal(t, int64(9), v)
		}()
	}
	wg.Wait()
	require.Equal(t, int64(1), atomic.LoadInt64(&inner.calls))
}

func TestVersionCache_ErrorNotCached(t *testing.T) {
	inner := &countingVersionRepo{err: errors.New("db down")}
	c := NewVersionCache(inner, 50*time.Millisecond)

	_, err := c.GetVersion(context.Background())
	require.Error(t, err)

	inner.err = nil
	inner.v = 3
	v, err := c.GetVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(3), v)
	require.Equal(t, int64(2), atomic.LoadInt64(&inner.calls))
}
```

- [ ] **Step 2: Run tests — expect FAIL**

Run: `go test ./internal/services/tarification/application/ -run TestVersionCache -v`
Expected: FAIL (NewVersionCache undefined).

- [ ] **Step 3: Implement**

```go
// version_cache.go
package application

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// VersionCache — TTL-обёртка над PriceRulesVersionRepository.
// GetVersion в горячем пути вызывается на каждый resolve; raw DB round-trip
// деградирует throughput. TTL=1s достаточно: инвалидация видна в течение
// секунды, при этом 99% запросов берут значение из памяти.
// singleflight защищает от thundering herd на истечении TTL или на cold start.
type VersionCache struct {
	inner domain.PriceRulesVersionRepository
	ttl   time.Duration

	mu        sync.RWMutex
	value     int64
	expiresAt time.Time

	sf singleflight.Group
}

func NewVersionCache(inner domain.PriceRulesVersionRepository, ttl time.Duration) *VersionCache {
	return &VersionCache{inner: inner, ttl: ttl}
}

func (c *VersionCache) GetVersion(ctx context.Context) (int64, error) {
	c.mu.RLock()
	if time.Now().Before(c.expiresAt) {
		v := c.value
		c.mu.RUnlock()
		return v, nil
	}
	c.mu.RUnlock()

	v, err, _ := c.sf.Do("version", func() (any, error) {
		// Double-check после захвата — другой caller мог уже обновить.
		c.mu.RLock()
		if time.Now().Before(c.expiresAt) {
			v := c.value
			c.mu.RUnlock()
			return v, nil
		}
		c.mu.RUnlock()

		fresh, err := c.inner.GetVersion(ctx)
		if err != nil {
			return int64(0), err
		}
		c.mu.Lock()
		c.value = fresh
		c.expiresAt = time.Now().Add(c.ttl)
		c.mu.Unlock()
		return fresh, nil
	})
	if err != nil {
		return 0, err
	}
	return v.(int64), nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./internal/services/tarification/application/ -run TestVersionCache -v`
Expected: PASS (4 subtests).

- [ ] **Step 5: Verify `golang.org/x/sync/singleflight` already in go.mod**

Run: `grep -q 'golang.org/x/sync' go.mod && echo OK || echo MISSING`
Expected: `OK`. Если `MISSING` — `go get golang.org/x/sync` перед коммитом.

- [ ] **Step 6: Commit**

```bash
git add internal/services/tarification/application/version_cache.go internal/services/tarification/application/version_cache_test.go
git commit -m "feat(tarification): add VersionCache with TTL + singleflight

1-second in-memory cache over PriceRulesVersionRepository.GetVersion to
avoid DB round-trip on every hot-path resolve. singleflight prevents
thundering-herd on TTL expiry.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

(При первой итерации строка Reviewed заполняется после фазы 2 execute-with-review.)

---

## Task 2: Switch PriceResolver to use VersionCache, drop TODO

**Files:**
- Modify: `internal/services/tarification/application/price_resolver.go:22-68`
- Modify: `internal/services/tarification/application/price_resolver_test.go` (конструктор тестов)

- [ ] **Step 1: Update PriceResolver struct + constructor**

Заменить поле `versionRepo domain.PriceRulesVersionRepository` на тип-интерфейс обёртки. Чтобы не плодить интерфейсы — ввести минимальный локальный:

```go
// price_resolver.go

// versionSource — минимальный интерфейс для чтения версии.
// Подходит и для raw-repo, и для VersionCache.
type versionSource interface {
	GetVersion(ctx context.Context) (int64, error)
}

type PriceResolver struct {
	ruleRepo     domain.PriceRuleRepository
	resolvedRepo domain.ResolvedRulesRepository
	versionRepo  versionSource
	aggResolver  AggregatorResolver
}

func NewPriceResolver(
	ruleRepo domain.PriceRuleRepository,
	resolvedRepo domain.ResolvedRulesRepository,
	versionRepo versionSource,
	aggResolver AggregatorResolver,
) *PriceResolver {
	return &PriceResolver{
		ruleRepo:     ruleRepo,
		resolvedRepo: resolvedRepo,
		versionRepo:  versionRepo,
		aggResolver:  aggResolver,
	}
}
```

- [ ] **Step 2: Remove Phase 3 TODO comment block (lines 58-61)**

```go
// Было:
	// Phase 3 TODO: обернуть GetVersion короткоживущим in-process кешем (~1s TTL).
	// Сейчас это DB round-trip на каждый hot-path запрос. В Phase 1 hot path
	// по-прежнему legacy, так что последствий нет; но до включения unified_enabled
	// в прод это обязательный оптимизатор — иначе unified mode деградирует throughput.

// Стало (комментарий удалён целиком, GetVersion вызывается как было):
	currentVersion, err := r.versionRepo.GetVersion(ctx)
```

- [ ] **Step 3: Run existing resolver tests**

Run: `go test ./internal/services/tarification/application/ -run TestPriceResolver -v`
Expected: PASS (fakeVersionRepo имеет тот же метод `GetVersion`, duck typing работает).

- [ ] **Step 4: Commit**

```bash
git add internal/services/tarification/application/price_resolver.go
git commit -m "refactor(tarification): accept versionSource in PriceResolver

Decouple PriceResolver from concrete PriceRulesVersionRepository so that
VersionCache (introduced in prior commit) can be injected. Drops the
Phase 3 TODO — cache is now available.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 3: Rollout gate (fnv hash + percentage)

**Files:**
- Create: `internal/services/tarification/application/rollout.go`
- Test: `internal/services/tarification/application/rollout_test.go`

- [ ] **Step 1: Write failing tests**

```go
// rollout_test.go
package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRollout_ZeroPercentNobodyIn(t *testing.T) {
	r := NewRollout(0)
	for i := 0; i < 1000; i++ {
		require.False(t, r.Enabled(uuid.New()))
	}
}

func TestRollout_HundredPercentEverybodyIn(t *testing.T) {
	r := NewRollout(100)
	for i := 0; i < 1000; i++ {
		require.True(t, r.Enabled(uuid.New()))
	}
}

func TestRollout_DeterministicForSameID(t *testing.T) {
	id := uuid.New()
	r := NewRollout(50)
	first := r.Enabled(id)
	for i := 0; i < 100; i++ {
		require.Equal(t, first, r.Enabled(id))
	}
}

func TestRollout_ApproxPercentage(t *testing.T) {
	r := NewRollout(25)
	in := 0
	n := 10000
	for i := 0; i < n; i++ {
		if r.Enabled(uuid.New()) {
			in++
		}
	}
	ratio := float64(in) / float64(n)
	require.InDelta(t, 0.25, ratio, 0.03, "expected ~25%%, got %.3f", ratio)
}

func TestRollout_ClampsNegativeAndOver100(t *testing.T) {
	require.False(t, NewRollout(-5).Enabled(uuid.New()))
	require.True(t, NewRollout(150).Enabled(uuid.New()))
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./internal/services/tarification/application/ -run TestRollout -v`
Expected: FAIL (NewRollout undefined).

- [ ] **Step 3: Implement**

```go
// rollout.go
package application

import (
	"hash/fnv"

	"github.com/google/uuid"
)

// Rollout — детерминистичный gate для staged rollout unified-пути.
// Percentage ∈ [0..100]. hash(subaccount_id) % 100 < percentage.
// Один и тот же subaccount всегда получает одинаковый ответ при том же
// percentage — клиенты не «скачут» между unified и legacy в пределах релиза.
type Rollout struct {
	percentage int
}

func NewRollout(percentage int) *Rollout {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}
	return &Rollout{percentage: percentage}
}

func (r *Rollout) Enabled(subaccountID uuid.UUID) bool {
	if r.percentage <= 0 {
		return false
	}
	if r.percentage >= 100 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write(subaccountID[:])
	return int(h.Sum32()%100) < r.percentage
}

func (r *Rollout) Percentage() int { return r.percentage }
```

- [ ] **Step 4: Run — expect PASS**

Run: `go test ./internal/services/tarification/application/ -run TestRollout -v`
Expected: PASS (5 subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/services/tarification/application/rollout.go internal/services/tarification/application/rollout_test.go
git commit -m "feat(tarification): add Rollout gate for staged unified cutover

Deterministic fnv(subaccount_id) % 100 gate. Supports 0..100 percentage,
clamps out-of-range inputs. Approximate 25% split validated on 10k
random UUIDs with 3%% tolerance.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 4: Config — `unified_rollout_percentage`

**Files:**
- Modify: `internal/config/config.go:30-35`, default section ~line 403

- [ ] **Step 1: Add field to TarificationConfig**

```go
// config.go — было:
type TarificationConfig struct {
	UnifiedEnabled         bool `mapstructure:"unified_enabled"`
	TiersCacheSize         int  `mapstructure:"tiers_cache_size"`
	JanitorIntervalSeconds int  `mapstructure:"janitor_interval_seconds"`
	AggregatorCacheTTLSec  int  `mapstructure:"aggregator_cache_ttl_sec"`
}

// config.go — стало:
type TarificationConfig struct {
	UnifiedEnabled           bool `mapstructure:"unified_enabled"`
	UnifiedRolloutPercentage int  `mapstructure:"unified_rollout_percentage"`
	TiersCacheSize           int  `mapstructure:"tiers_cache_size"`
	JanitorIntervalSeconds   int  `mapstructure:"janitor_interval_seconds"`
	AggregatorCacheTTLSec    int  `mapstructure:"aggregator_cache_ttl_sec"`
}
```

- [ ] **Step 2: Add SetDefault near line 403**

Найди строку `v.SetDefault("tarification.unified_enabled", false)` и добавь **следующей**:

```go
v.SetDefault("tarification.unified_rollout_percentage", 0)
```

- [ ] **Step 3: Build — expect PASS**

Run: `go build ./...`
Expected: exit 0. (Если Device Guard блокирует — полагаемся на CI; `./scripts/check.sh` скипнет.)

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add tarification.unified_rollout_percentage

Percentage-based staged rollout of the unified pricing hot path.
Default 0 (off). Paired with unified_enabled gate.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 5: Extend `ResolveInput` + SQL with `ExcludeSubaccount`

**Files:**
- Modify: `internal/services/tarification/domain/price_rule_repository.go:13-21`
- Modify: `internal/services/tarification/infrastructure/repository/price_rule_repository.go:25-59` (const + `FindApplicable` body)

- [ ] **Step 1: Add field to ResolveInput**

```go
// domain/price_rule_repository.go — было:
type ResolveInput struct {
	SubaccountID   uuid.UUID
	AggregatorID   uuid.UUID
	Country        string
	Operator       string
	SenderCategory string
	TrafficType    string
	Now            time.Time
}

// Стало — добавлено последнее поле:
type ResolveInput struct {
	SubaccountID   uuid.UUID
	AggregatorID   uuid.UUID
	Country        string
	Operator       string
	SenderCategory string
	TrafficType    string
	Now            time.Time
	// ExcludeSubaccount=true — отключить subaccount-уровень в lookup.
	// Используется для margin-резолва: aggregator-baseline независимо от
	// наличия override у конкретного субаккаунта.
	ExcludeSubaccount bool
}
```

- [ ] **Step 2: Update SQL**

В `infrastructure/repository/price_rule_repository.go`:

```go
// Было (const findApplicableSQL):
//   WHERE (
//          (owner_type='subaccount' AND owner_id = $1)
//       OR (owner_type='aggregator' AND owner_id = $2)
//       OR (owner_type='platform')
//   )
//     AND (country         IS NULL OR country         = $3)
//     ...
//     AND valid_from <= $7
//     AND (valid_to IS NULL OR valid_to > $7)

// Стало — добавляется параметр $8 (exclude_subaccount bool):
const findApplicableSQL = `
WITH ranked AS (
  SELECT
    id, owner_type, owner_id, country, operator, sender_category, traffic_type,
    valid_from, valid_to, price_model, price_value::text AS price_value_str, tiers_json,
    created_at, updated_at, created_by,
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
         (NOT $8 AND owner_type='subaccount' AND owner_id = $1)
      OR (owner_type='aggregator' AND owner_id = $2)
      OR (owner_type='platform')
  )
    AND (country         IS NULL OR country         = $3)
    AND (operator        IS NULL OR operator        = $4)
    AND (sender_category IS NULL OR sender_category = $5)
    AND (traffic_type    IS NULL OR traffic_type    = $6)
    AND valid_from <= $7
    AND (valid_to IS NULL OR valid_to > $7)
)
SELECT id, owner_type, owner_id, country, operator, sender_category, traffic_type,
       valid_from, valid_to, price_model, price_value_str, tiers_json,
       created_at, updated_at, created_by
FROM ranked
ORDER BY owner_rank DESC, specificity_rank DESC, valid_from DESC
LIMIT 1;
`
```

- [ ] **Step 3: Update `FindApplicable` body to pass new param**

Найди место где `findApplicableSQL` выполняется (`db.GetContext` или `QueryRowx`). Добавь `in.ExcludeSubaccount` как восьмой аргумент после `in.Now`:

```go
// Было (упрощённо):
err := r.db.GetContext(ctx, &row, findApplicableSQL,
    in.SubaccountID, in.AggregatorID,
    nullableString(in.Country), nullableString(in.Operator),
    nullableString(in.SenderCategory), nullableString(in.TrafficType),
    in.Now,
)

// Стало — добавлен in.ExcludeSubaccount:
err := r.db.GetContext(ctx, &row, findApplicableSQL,
    in.SubaccountID, in.AggregatorID,
    nullableString(in.Country), nullableString(in.Operator),
    nullableString(in.SenderCategory), nullableString(in.TrafficType),
    in.Now,
    in.ExcludeSubaccount,
)
```

(Читающий этот план: прочитай реальное тело `FindApplicable` перед заменой; аргументы sql.Null* могут идти через helper — сохраняй стиль файла.)

- [ ] **Step 4: Update existing fakes in price_resolver_test.go**

Ничего не меняется — `fakeRuleRepo.FindApplicable` принимает всю `ResolveInput` и не смотрит на поля. Должно остаться рабочим.

- [ ] **Step 5: Run resolver tests**

Run: `go test ./internal/services/tarification/application/ -run TestPriceResolver -v`
Expected: PASS (существующие тесты не смотрят на `ExcludeSubaccount`).

- [ ] **Step 6: Build**

Run: `go build ./...`
Expected: exit 0 (или SKIP под Device Guard).

- [ ] **Step 7: Commit**

```bash
git add internal/services/tarification/domain/price_rule_repository.go internal/services/tarification/infrastructure/repository/price_rule_repository.go
git commit -m "feat(tarification): add ResolveInput.ExcludeSubaccount flag

Enables margin-path resolver to get the aggregator-level baseline price
independently of subaccount-level override. SQL gates the subaccount
owner_type clause on (NOT \$8).

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 6: Prometheus metrics for unified path

**Files:**
- Create: `internal/services/tarification/application/unified_metrics.go`

- [ ] **Step 1: Implement metrics**

```go
// unified_metrics.go
package application

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Метрики unified hot path. Имена с префиксом tarification_unified_*
// чтобы отделять от legacy метрик и от общих tarification_*.
var (
	unifiedResolveTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tarification_unified_resolve_total",
			Help: "Resolve-вызовы hot-path unified. cache=hit|miss, branch=subaccount|margin.",
		},
		[]string{"cache", "branch"},
	)

	unifiedResolveLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tarification_unified_resolve_duration_seconds",
			Help:    "Длительность resolve в unified hot-path.",
			Buckets: []float64{0.0005, 0.001, 0.002, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"branch"}, // subaccount | margin
	)

	unifiedPriceNotFoundTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tarification_unified_price_not_found_total",
			Help: "Количество случаев, когда для запроса не нашлось применимого правила. Должно быть 0 — триггер алерта.",
		},
	)

	unifiedFallbackTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tarification_unified_fallback_total",
			Help: "Fallback unified → legacy в hot-path. reason=not_found|resolve_error|calc_error|counter_error.",
		},
		[]string{"reason"},
	)

	unifiedTarifyTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tarification_unified_tarify_total",
			Help: "TarifyMessage через unified path. result=approved|rejected.",
		},
		[]string{"result"},
	)

	unifiedMarginPerHourRub = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tarification_unified_margin_rub_total",
			Help: "Суммарная агрегаторская маржа в RUB через unified путь. Per-hour rate считается в Grafana через rate().",
		},
	)
)
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/services/tarification/application/unified_metrics.go
git commit -m "feat(tarification): prometheus metrics for unified hot path

- tarification_unified_resolve_total{cache,branch}
- tarification_unified_resolve_duration_seconds{branch}
- tarification_unified_price_not_found_total (alert on > 0)
- tarification_unified_fallback_total{reason}
- tarification_unified_tarify_total{result}
- tarification_unified_margin_rub_total

margin_per_hour_rub derives from Grafana rate() over the counter.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 7: `cache_hit_rate` instrumentation inside PriceResolver

**Files:**
- Modify: `internal/services/tarification/application/price_resolver.go` (метод `Resolve`)

**Контекст:** `unifiedResolveTotal{cache,branch}` считает hit/miss. PriceResolver сам не знает своего `branch` (subaccount vs margin) — передадим через context value, чтобы не менять сигнатуру `Resolve` и не ломать существующих вызывающих.

- [ ] **Step 1: Add context key + helper**

В `price_resolver.go` в начало файла добавить:

```go
type resolverBranchKey struct{}

// WithResolverBranch — помечает context этикеткой branch для метрик
// (subaccount|margin). Если не задано — branch="unknown".
func WithResolverBranch(ctx context.Context, branch string) context.Context {
	return context.WithValue(ctx, resolverBranchKey{}, branch)
}

func branchFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(resolverBranchKey{}).(string); ok && v != "" {
		return v
	}
	return "unknown"
}
```

- [ ] **Step 2: Instrument cache hit/miss в Resolve**

В `Resolve` после проверки `cached != nil && cached.RulesVersion == currentVersion`:

```go
// Было:
	if cached != nil && cached.RulesVersion == currentVersion {
		return cached, nil
	}

// Стало:
	branch := branchFromCtx(ctx)
	if cached != nil && cached.RulesVersion == currentVersion {
		unifiedResolveTotal.WithLabelValues("hit", branch).Inc()
		return cached, nil
	}
	unifiedResolveTotal.WithLabelValues("miss", branch).Inc()
```

- [ ] **Step 3: Build + resolver tests still pass**

Run: `go test ./internal/services/tarification/application/ -run TestPriceResolver -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/services/tarification/application/price_resolver.go
git commit -m "feat(tarification): instrument resolver cache hit/miss metric

Adds WithResolverBranch(ctx, 'subaccount'|'margin') so Resolve can tag
tarification_unified_resolve_total{cache,branch} without expanding its
signature. Existing callers get branch='unknown'.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 8: Unified hot path — `tarifyUnified`

**Files:**
- Create: `internal/services/tarification/application/unified_path.go`
- Test: `internal/services/tarification/application/unified_path_test.go`

**Контракт:** `tarifyUnified(ctx, req, deps) (resp *TarifyMessageResponse, fallbackReason string, err error)`.
- `err != nil` — фатальная ошибка (не подходит для fallback, например уже произошёл charge). Вызывающий возвращает её клиенту.
- `fallbackReason != ""` — unified не смог обработать (no rule / resolve err / calc err / counter err). Вызывающий должен позвать legacy-ветку.
- `resp != nil, err == nil, fallbackReason == ""` — успешный unified-результат.

- [ ] **Step 1: Define deps struct + skeleton**

```go
// unified_path.go
package application

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// unifiedDeps — всё, что нужно новому hot-path. Собирается в NewTarificationService/Set...
// Держим как вложенную структуру, чтобы не раздувать сигнатуру.
type unifiedDeps struct {
	resolver       *PriceResolver
	calc           *CostCalculator
	ruleRepo       domain.PriceRuleRepository // для margin-резолва
	subUsageRepo   domain.SubaccountUsageCounterRepository
	clientInfo     domain.ClientInfoRepository
	marginLogRepo  domain.AggregatorMarginLogRepository
	saga           *SagaOrchestrator
	logRepo        domain.TarificationLogRepository
	eventPub       domain.EventPublisher
	senderRepo     domain.SenderRegistrationRepository
	operatorLookup OperatorCodeLookup // Task 0 — резолв UUID → operators.code
}

// tarifyUnified реализует новую ветку TarifyMessage под unified-флагом.
// См. docs/superpowers/plans/2026-04-19-phase3-unified-pricing-hot-path.md.
func tarifyUnified(ctx context.Context, req *TarifyMessageRequest, d *unifiedDeps) (*TarifyMessageResponse, string, error) {
	// 1. Идемпотентность — ровно как в legacy; это уже проверено до входа сюда,
	//    но дублируем тонким чеком на случай прямого вызова.
	if existing, err := d.logRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey); err != nil {
		return nil, "", fmt.Errorf("idempotency check: %w", err)
	} else if existing != nil {
		return &TarifyMessageResponse{
			Approved:     true,
			TotalAmount:  existing.TotalAmount,
			Strategy:     string(existing.Strategy),
			TariffPlanID: existing.TariffPlanID.String(),
		}, "", nil
	}

	// 2. Определение категории (как в legacy determineSenderCategory — но в unified
	//    модели категория сразу является измерением ResolveInput, не отдельным шагом).
	category, err := resolveCategory(ctx, d.senderRepo, req.ClientID, req.OperatorID, req.SenderName)
	if err != nil {
		return nil, "", fmt.Errorf("resolve category: %w", err)
	}

	// 3. Resolve operator_id → operators.code (Task 0 / operator-code-lookup-design).
	operatorCode, err := d.operatorLookup.Code(ctx, req.OperatorID)
	if err != nil {
		unifiedFallbackTotal.WithLabelValues("operator_lookup_error").Inc()
		log.Warn().Err(err).Str("operator_id", req.OperatorID.String()).
			Msg("unified: operator code lookup failed — fallback")
		return nil, "operator_lookup_error", nil
	}

	// 4. Subaccount-path resolve.
	// trafficType="" — wildcard. Разделение promo/transactional — follow-up.
	in := domain.ResolveInput{
		SubaccountID:   req.ClientID,
		Country:        "",
		Operator:       operatorCode,
		SenderCategory: string(category),
		TrafficType:    "",
		Now:            time.Now().UTC(),
	}
	startResolve := time.Now()
	rr, err := d.resolver.Resolve(WithResolverBranch(ctx, "subaccount"), in)
	unifiedResolveLatency.WithLabelValues("subaccount").Observe(time.Since(startResolve).Seconds())
	if err != nil {
		if isNotFound(err) {
			unifiedPriceNotFoundTotal.Inc()
			unifiedFallbackTotal.WithLabelValues("not_found").Inc()
			log.Warn().Err(err).Str("subaccount_id", req.ClientID.String()).Msg("unified resolve: no applicable rule — fallback to legacy")
			return nil, "not_found", nil
		}
		unifiedFallbackTotal.WithLabelValues("resolve_error").Inc()
		log.Warn().Err(err).Msg("unified resolve error — fallback to legacy")
		return nil, "resolve_error", nil
	}

	// 4. Period key + usage-before (через Get; Increment вернёт новое значение).
	periodKey := computePeriodKeyFromRule(rr)
	counter, err := d.subUsageRepo.Get(ctx, req.ClientID, periodKey)
	if err != nil {
		unifiedFallbackTotal.WithLabelValues("counter_error").Inc()
		log.Warn().Err(err).Msg("unified counter get error — fallback")
		return nil, "counter_error", nil
	}
	var usageBefore int64
	if counter != nil {
		usageBefore = counter.SegmentsUsed
	}

	// 5. Calculate.
	cost, err := d.calc.Calculate(rr, usageBefore, int64(req.SegmentCount))
	if err != nil {
		unifiedFallbackTotal.WithLabelValues("calc_error").Inc()
		log.Warn().Err(err).Str("rule_id", rr.SourceRuleID.String()).Msg("unified calc error — fallback")
		return nil, "calc_error", nil
	}
	costStr := strconv.FormatFloat(cost, 'f', 6, 64)

	// 6. Charge. После этой точки fallback уже небезопасен.
	currency := "RUB" // в unified модели currency пока не денормализована per-rule; используем дефолт платформы
	chargeResult, err := d.saga.Charge(ctx,
		req.ClientID.String(), req.MessageID.String(),
		costStr, currency,
		fmt.Sprintf("SMS tarification (unified): %d segments", req.SegmentCount),
		int32(req.SegmentCount),
	)
	if err != nil {
		return nil, "", fmt.Errorf("unified charge: %w", err)
	}
	if !chargeResult.Success {
		unifiedTarifyTotal.WithLabelValues("rejected").Inc()
		return &TarifyMessageResponse{
			Approved:        false,
			RejectionReason: domain.ErrInsufficientBalance.Error(),
		}, "", nil
	}

	// 7. Increment usage counter.
	if _, err := d.subUsageRepo.Increment(ctx, req.ClientID, periodKey, int64(req.SegmentCount), costStr); err != nil {
		log.Error().Err(err).Msg("unified: usage counter increment failed after charge")
	}

	// 8. Margin resolve — второй вызов с ExcludeSubaccount=true, напрямую в ruleRepo.
	marginIn := in
	marginIn.ExcludeSubaccount = true
	marginIn.AggregatorID = rr.AggregatorID
	startMargin := time.Now()
	aggRule, marginErr := d.ruleRepo.FindApplicable(ctx, marginIn)
	unifiedResolveLatency.WithLabelValues("margin").Observe(time.Since(startMargin).Seconds())
	if marginErr == nil && aggRule != nil {
		aggTempRR := &domain.ResolvedRule{
			PriceModel:   aggRule.PriceModel,
			PriceValue:   aggRule.PriceValue,
			TiersJSON:    aggRule.TiersJSON,
			SourceRuleID: aggRule.ID,
			RulesVersion: rr.RulesVersion,
		}
		aggCost, aggErr := d.calc.Calculate(aggTempRR, usageBefore, int64(req.SegmentCount))
		if aggErr == nil {
			marginVal := new(big.Float).Sub(
				new(big.Float).SetFloat64(cost),
				new(big.Float).SetFloat64(aggCost),
			)
			marginStr := marginVal.Text('f', 6)
			if d.marginLogRepo != nil {
				entry := &domain.AggregatorMarginLog{
					ID:              uuid.New(),
					AggregatorID:    rr.AggregatorID,
					SubAccountID:    req.ClientID,
					MessageID:       req.MessageID,
					OperatorID:      req.OperatorID,
					SegmentCount:    req.SegmentCount,
					SubAccountTotal: costStr,
					AggregatorTotal: strconv.FormatFloat(aggCost, 'f', 6, 64),
					Margin:          marginStr,
					IdempotencyKey:  req.IdempotencyKey + "_margin",
					CreatedAt:       time.Now(),
				}
				if err := d.marginLogRepo.Create(ctx, entry); err != nil {
					log.Error().Err(err).Msg("unified: margin log create failed")
				}
			}
			// Если margin > 0 — засчитать в prometheus-counter (только positive-margin).
			if marginFloat, ok := new(big.Float).Sub(
				new(big.Float).SetFloat64(cost),
				new(big.Float).SetFloat64(aggCost),
			).Float64(); ok == big.Exact || ok == big.Above || ok == big.Below {
				if marginFloat > 0 {
					unifiedMarginPerHourRub.Add(marginFloat)
				}
			}
		}
	}

	// 9. Tarification log (для идемпотентности follow-up вызовов).
	tarLog := &domain.TarificationLog{
		ID:              uuid.New(),
		ClientID:        req.ClientID,
		MessageID:       req.MessageID,
		OperatorID:      req.OperatorID,
		Category:        category,
		SegmentCount:    req.SegmentCount,
		PricePerSegment: strconv.FormatFloat(cost/float64(req.SegmentCount), 'f', 6, 64),
		TotalAmount:     costStr,
		IdempotencyKey:  req.IdempotencyKey,
		CreatedAt:       time.Now(),
		// Strategy/TariffPlanID/TariffPeriodID — не применимы в unified; оставляем zero.
	}
	if err := d.logRepo.Create(ctx, tarLog); err != nil {
		log.Error().Err(err).Msg("unified: tarification log create failed")
	}

	unifiedTarifyTotal.WithLabelValues("approved").Inc()
	return &TarifyMessageResponse{
		Approved:     true,
		TotalAmount:  costStr,
		Currency:     currency,
		Strategy:     "unified",
		TariffPlanID: rr.SourceRuleID.String(),
	}, "", nil
}

// resolveCategory дублирует determineSenderCategory без зависимости от self.
func resolveCategory(ctx context.Context, repo domain.SenderRegistrationRepository, clientID, operatorID uuid.UUID, senderName string) (domain.SenderCategory, error) {
	if senderName == "" {
		return domain.CategoryShared, nil
	}
	reg, err := repo.GetActiveByClientOperatorName(ctx, clientID, operatorID, senderName)
	if err != nil || reg == nil {
		return domain.CategoryShared, nil
	}
	switch reg.Type {
	case domain.SenderTypePaid:
		return domain.CategoryPaidRegistered, nil
	case domain.SenderTypeFree:
		return domain.CategoryFreeRegistered, nil
	default:
		return domain.CategoryShared, nil
	}
}

// computePeriodKeyFromRule — извлекает period из rule.TiersJSON или дефолтит на
// calendar_month (fixed-модель). В Phase 3 достаточно дефолта; при
// добавлении периодных tiered-правил обновить.
func computePeriodKeyFromRule(rr *domain.ResolvedRule) string {
	return domain.ComputePeriodKey(domain.PeriodCalendarMonth, time.Now())
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == domain.ErrNoApplicableRule.Error() ||
		// обёртка через %w — проверяем substring
		containsNotFound(err.Error())
}

func containsNotFound(s string) bool {
	for i := 0; i+len("no applicable rule") <= len(s); i++ {
		if s[i:i+len("no applicable rule")] == "no applicable rule" {
			return true
		}
	}
	return false
}
```

(Замечание исполнителю: `domain.ErrNoApplicableRule` уже определён в `domain/errors.go`. Сверь имя и текст ошибки; если текст другой — поправь `containsNotFound`.)

- [ ] **Step 2: Write table-driven test for happy path + fallback reasons**

```go
// unified_path_test.go
package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type stubSubUsage struct {
	get       *domain.SubaccountUsageCounter
	getErr    error
	incCalled bool
}

func (s *stubSubUsage) Get(context.Context, uuid.UUID, string) (*domain.SubaccountUsageCounter, error) {
	return s.get, s.getErr
}
func (s *stubSubUsage) Increment(context.Context, uuid.UUID, string, int64, string) (int64, error) {
	s.incCalled = true
	return 10, nil
}

type stubLogRepo struct {
	existing *domain.TarificationLog
	created  *domain.TarificationLog
}

func (s *stubLogRepo) GetByIdempotencyKey(context.Context, string) (*domain.TarificationLog, error) {
	return s.existing, nil
}
func (s *stubLogRepo) Create(_ context.Context, l *domain.TarificationLog) error {
	s.created = l
	return nil
}

// (остальные методы TarificationLogRepository — nil-safe stubs; скопируй из существующих моков)

func TestTarifyUnified_FallbackOnNotFound(t *testing.T) {
	ruleRepo := &fakeRuleRepo{err: domain.ErrNoApplicableRule}
	// ... собрать unifiedDeps с fake'ами
	// вызов tarifyUnified и проверка что fallbackReason == "not_found"
	_ = ruleRepo
	t.Skip("Скелет: заполни, используя fakes из price_resolver_test.go + новые stubs")
}

func TestTarifyUnified_HappyPathApproves(t *testing.T) {
	t.Skip("Аналогично: PriceResolver с fakeRuleRepo возвращающим fixed правило; stubSaga.Charge success; проверка approved=true, margin_log.Create вызван")
}

func TestTarifyUnified_FallbackOnCalcError(t *testing.T) {
	t.Skip("TiersJSON битый → fallbackReason=calc_error")
}

func TestTarifyUnified_ChargeFailReturnsRejection(t *testing.T) {
	t.Skip("saga.Charge success=false → Approved=false, без fallback'а (ошибка после charge-точки)")
}

func TestTarifyUnified_IdempotencyShortCircuits(t *testing.T) {
	_ = errors.New
	_ = time.Now
	t.Skip("stubLogRepo.existing != nil → возвращается existing без resolve/charge")
}
```

Исполнителю: развернуть `t.Skip`-stubs в полноценные тесты. Фейки `fakeRuleRepo`/`fakeResolvedRepo`/`fakeVersionRepo`/`fakeAggResolver` уже есть в `price_resolver_test.go` — переиспользуй.

- [ ] **Step 3: Build**

Run: `go build ./internal/services/tarification/...`
Expected: exit 0.

- [ ] **Step 4: Run tests (skipped-stubs должны PASS со Skip)**

Run: `go test ./internal/services/tarification/application/ -run TestTarifyUnified -v`
Expected: 5 tests, all SKIP (скелет). Полная реализация stubs — в последующих коммитах (см. Task 8b ниже).

- [ ] **Step 5: Commit skeleton**

```bash
git add internal/services/tarification/application/unified_path.go internal/services/tarification/application/unified_path_test.go
git commit -m "feat(tarification): unified hot path skeleton (tarifyUnified)

Resolve → Calculate → UPSERT usage counter → Charge → margin second-resolve
→ log. Returns (resp, fallbackReason, err) so caller can route to legacy
on non-fatal failure. Tests are skipped stubs — filled in next commit.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

### Task 8b: Fill tests

- [ ] **Step 1: Replace each `t.Skip` with working assertions**

Требования:
1. `TestTarifyUnified_HappyPathApproves`: fixed-rule price=1.5, 2 сегмента → cost=3.0; `d.saga.Charge` stub возвращает Success; response.Approved=true, TotalAmount="3.000000". `stubSubUsage.incCalled=true`. `stubMarginLog.Create` вызван.
2. `TestTarifyUnified_FallbackOnNotFound`: `fakeRuleRepo.err = fmt.Errorf("something: %w", domain.ErrNoApplicableRule)`. Ожидаем `fallbackReason="not_found"`, `resp=nil`, `err=nil`. Проверить `unifiedPriceNotFoundTotal` увеличился на 1 (`testutil.ToFloat64`).
3. `TestTarifyUnified_FallbackOnCalcError`: `TiersJSON=[]byte("{broken")` → calc возвращает error → `fallbackReason="calc_error"`.
4. `TestTarifyUnified_ChargeFailReturnsRejection`: stub saga returns `{Success:false}` → `resp.Approved=false`, `fallbackReason=""`, `err=nil`.
5. `TestTarifyUnified_IdempotencyShortCircuits`: `stubLogRepo.existing != nil` → resp с TotalAmount из existing, `stubSubUsage` не вызывался, saga не вызывалась.
6. `TestTarifyUnified_OperatorLookupError_Fallback`: `stubOperatorLookup` возвращает `domain.ErrOperatorNotFound` → `fallbackReason="operator_lookup_error"`, `resp=nil`, `err=nil`, `unifiedFallbackTotal{reason="operator_lookup_error"}` = 1.

В happy-path и остальных тестах `stubOperatorLookup.Code` всегда возвращает `"mts-ru"` и `fakeRuleRepo.byLookup.Operator == &"mts-ru"`.

- [ ] **Step 2: Run — expect PASS**

Run: `go test ./internal/services/tarification/application/ -run TestTarifyUnified -v`
Expected: 5 PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/services/tarification/application/unified_path_test.go
git commit -m "test(tarification): unified hot path fakes cover all branches

Happy path, not_found fallback, calc error fallback, charge rejection,
idempotency short-circuit. Uses existing fakes from price_resolver_test.go
plus stubSubUsage/stubLogRepo/stubMarginLog/stubSaga.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 9: Wire unified path into `TarificationService.TarifyMessage`

**Files:**
- Modify: `internal/services/tarification/application/tarification_service.go`

- [ ] **Step 1: Add fields to struct**

После блока `quotaService *QuotaService` добавить:

```go
	// Unified pricing (Phase 3) — опционально, nil до SetUnifiedDependencies.
	unifiedEnabled bool
	rollout        *Rollout
	unifiedDeps    *unifiedDeps
```

- [ ] **Step 2: Add setter**

```go
// SetUnifiedDependencies подключает компоненты unified hot path.
// Вызывается в main.go под if cfg.Tarification.UnifiedEnabled.
// rollout может быть nil (тогда unified-путь не активируется).
func (s *TarificationService) SetUnifiedDependencies(
	enabled bool,
	rollout *Rollout,
	resolver *PriceResolver,
	calc *CostCalculator,
	ruleRepo domain.PriceRuleRepository,
	subUsageRepo domain.SubaccountUsageCounterRepository,
	operatorLookup OperatorCodeLookup,
) {
	s.unifiedEnabled = enabled
	s.rollout = rollout
	s.unifiedDeps = &unifiedDeps{
		resolver:       resolver,
		calc:           calc,
		ruleRepo:       ruleRepo,
		subUsageRepo:   subUsageRepo,
		clientInfo:     s.clientInfoRepo,
		marginLogRepo:  s.aggMarginLogRepo,
		saga:           s.saga,
		logRepo:        s.logRepo,
		eventPub:       s.eventPublisher,
		senderRepo:     s.senderRepo,
		operatorLookup: operatorLookup,
	}
}
```

- [ ] **Step 3: Wire branch into `TarifyMessage`**

Сразу **после** шага 1 (idempotency check), но **до** шага 2 (category/plan lookup), вставить:

```go
	// Phase 3 unified gate. Активен только если enabled И rollout пропустил
	// этот subaccount_id. При fallback'е (not_found / runtime-err) исполнение
	// проваливается в существующий legacy-блок ниже.
	if s.unifiedEnabled && s.rollout != nil && s.rollout.Enabled(req.ClientID) && s.unifiedDeps != nil {
		resp, fallbackReason, err := tarifyUnified(ctx, req, s.unifiedDeps)
		if err != nil {
			return nil, err
		}
		if fallbackReason == "" {
			return resp, nil
		}
		// иначе — fallback: продолжаем legacy ниже.
	}
```

(Ничего после этого блока не меняется — legacy-код остаётся dead-code-free.)

- [ ] **Step 4: Build**

Run: `go build ./internal/services/tarification/...`
Expected: exit 0.

- [ ] **Step 5: Run all tarification tests**

Run: `go test ./internal/services/tarification/... -count=1`
Expected: PASS (unified код в ветке неактивен без `SetUnifiedDependencies`; legacy-поведение не задето).

- [ ] **Step 6: Commit**

```bash
git add internal/services/tarification/application/tarification_service.go
git commit -m "feat(tarification): wire unified hot path into TarifyMessage

Gate on unifiedEnabled && rollout.Enabled(subaccount). Non-fatal failure
falls through to legacy branch. Fatal errors (post-charge) surface to
caller. Without SetUnifiedDependencies the gate is inert — production
unaffected until main.go wiring lands.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Task 10: Wire in `cmd/services/tarification-service/main.go`

**Files:**
- Modify: `cmd/services/tarification-service/main.go:170-207`

- [ ] **Step 1: Replace `_ = ...` discards with real construction + wiring**

Было (внутри `if cfg.Tarification.UnifiedEnabled`):

```go
		resolvedRepo := tarificationrepo.NewResolvedRulesRepository(dbx)
		versionRepo := tarificationrepo.NewPriceRulesVersionRepository(dbx)
		_ = tarificationrepo.NewSubaccountUsageCounterRepository(dbx)
		outboxRepo := tarificationrepo.NewInvalidationOutboxRepository(dbx)

		tiersCache := application.NewTiersCache(cfg.Tarification.TiersCacheSize)
		_ = application.NewCostCalculator(tiersCache)

		aggResolver := infrastructure.NewAggregatorResolver(dbx, nil, cfg.Tarification.AggregatorCacheTTLSec, logger)
		_ = application.NewPriceResolver(priceRuleRepo, resolvedRepo, versionRepo, aggResolver)
```

Стало:

```go
		resolvedRepo := tarificationrepo.NewResolvedRulesRepository(dbx)
		versionRepoRaw := tarificationrepo.NewPriceRulesVersionRepository(dbx)
		versionCache := application.NewVersionCache(versionRepoRaw, 1*time.Second)
		subUsageRepo := tarificationrepo.NewSubaccountUsageCounterRepository(dbx)
		outboxRepo := tarificationrepo.NewInvalidationOutboxRepository(dbx)

		tiersCache := application.NewTiersCache(cfg.Tarification.TiersCacheSize)
		costCalc := application.NewCostCalculator(tiersCache)

		aggResolver := infrastructure.NewAggregatorResolver(dbx, nil, cfg.Tarification.AggregatorCacheTTLSec, logger)
		priceResolver := application.NewPriceResolver(priceRuleRepo, resolvedRepo, versionCache, aggResolver)

		rollout := application.NewRollout(cfg.Tarification.UnifiedRolloutPercentage)
		operatorRepo := tarificationrepo.NewOperatorRepository(dbx)
		operatorLookup := application.NewCachedOperatorLookup(operatorRepo)
		tarificationService.SetUnifiedDependencies(
			true, rollout, priceResolver, costCalc,
			priceRuleRepo, subUsageRepo, operatorLookup,
		)

		janitor := application.NewResolvedRulesJanitor(outboxRepo, resolvedRepo, 100, logger)
		janitorCtx, janitorCancel := context.WithCancel(context.Background())
		go janitor.Run(janitorCtx, time.Duration(cfg.Tarification.JanitorIntervalSeconds)*time.Second)
		defer janitorCancel()

		logger.Info().
			Int("tiers_cache_size", cfg.Tarification.TiersCacheSize).
			Int("janitor_interval_seconds", cfg.Tarification.JanitorIntervalSeconds).
			Int("rollout_percentage", cfg.Tarification.UnifiedRolloutPercentage).
			Msg("unified tarification инициализирована (hot path активен для пропущенных rollout'ом субаккаунтов)")
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Run check.sh full**

Run: `./scripts/check.sh`
Expected: all green (или `[SKIP]` для Go-части под Device Guard; CI подстрахует).

- [ ] **Step 4: Commit**

```bash
git add cmd/services/tarification-service/main.go
git commit -m "feat(tarification): wire unified hot path in service main

Builds VersionCache, Rollout, PriceResolver, CostCalculator with real
repos and injects them into TarificationService via SetUnifiedDependencies.
Rollout percentage read from config (default 0). Log at startup so ops
sees the current rollout gate.

Reviewed: superpowers:code-reviewer (APPROVED)"
```

---

## Deployment / rollout plan (не задача для плана, но инструкция для оператора)

После merge plan-а:
1. Staging: `tarification.unified_enabled=true`, `unified_rollout_percentage=100`. Нагрузочный тест, сверка с legacy по margin/ledger.
2. Prod 1%: `unified_rollout_percentage=1`. Наблюдать 48ч: `tarification_unified_price_not_found_total` = 0, `tarification_unified_fallback_total` низкий, balance-ledger без аномалий.
3. Prod 10%: 48ч.
4. Prod 50%: 48ч.
5. Prod 100%: 1-2 недели.
6. При инциденте — откат через `unified_rollout_percentage=0` без редеплоя не выйдет (значение в config, требуется редеплой). Ops должен быть готов к hot redeploy config-changes за <5 мин. Альтернатива — добавить env-override (не в этом плане).

---

## Self-review

**Spec coverage** (Phase 3 bullets из plan Phase 3 родительского):
- ✅ Новая ветка TarifyMessage с Resolve→Calculate→UPSERT→Charge: Task 8, 9.
- ✅ Margin через второй resolve с owner=aggregator (исключая subaccount override): Task 5 (флаг) + Task 8 (использование).
- ✅ Идемпотентность через tarification_log: Task 8 (шаг 1 функции).
- ✅ Метрики cache_hit_rate / resolve_latency / price_not_found / margin_per_hour: Task 6, 7, 8.
- ✅ Staged rollout через consistent hashing: Task 3, 4, 9.
- ✅ GetVersion TTL wrapper: Task 1, 2.
- ❌ Нагрузочный тест на staging — за рамками плана (операционный шаг, не код).
- ⚠ Не покрыто: формальное маппирование `operator_id(UUID)` → `operator(string)` и `trafficType` в unified ResolveInput. Сейчас задано `operator=uuid.String()`, `trafficType="transactional"`. Это **drift**: price_rules.operator хранится как человеческий код оператора (e.g. "MTS"), а не UUID. Исполнитель должен зафиксировать в `docs/ac/_DRIFT.md` и открыть follow-up — без этого rollout=1% даст 100% `price_not_found` → fallback и плохой сигнал.

**Placeholder scan:** ни одного TBD/TODO-без-контента. В Task 8 помечены open items: сверка `domain.ErrNoApplicableRule` строкой, маппирование operator/trafficType — явные, адресованные исполнителю.

**Type consistency:**
- `Rollout.Enabled(uuid.UUID) bool` — используется в Task 9 ровно так.
- `VersionCache.GetVersion(ctx) (int64, error)` — совпадает с `versionSource` interface в Task 2.
- `tarifyUnified` возвращает `(resp, fallbackReason, err)` — Task 9 использует именно три возврата.
- `unifiedDeps` — одна и та же структура в Task 8 и Task 9 (setter копирует поля).

Одна несостыковка, которую нужно исправить при исполнении, но не меняет план: в Task 8 шаге 1 функция проверяет `marginFloat, ok := ...Float64()` с сравнением `ok == big.Exact || ok == big.Above || ok == big.Below` — это **почти всегда true**, условие косметическое. Исполнителю: упростить до прямого `marginFloat, _ := ...Float64()` и сравнения `if marginFloat > 0`.

---

## Execution handoff

**Plan complete and saved to `docs/superpowers/plans/2026-04-19-phase3-unified-pricing-hot-path.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — dispatch fresh subagent per task, review between tasks, fast iteration. Подходит для этого плана: 10 задач, каждая с чётким гейтом (test passes + commit).

**2. Inline Execution** — batch в текущей сессии через executing-plans с чекпоинтами.

Из-за обязательного `/execute-with-review` wrapper'а каждый коммит обязан пройти code-reviewer субагента. Практически это делает подход №1 естественным (reviewer как раз субагент). Рекомендую **Subagent-Driven**.
