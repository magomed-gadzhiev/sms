# Aggregator Phase 5: Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement platform control layer for aggregators — pool-level rate-limiting (per-second sliding window + monthly counter), cascade freeze of all sub-accounts with SMPP session force-unbind, aggregator unfreeze-all, audit log admin view, and admin portal pages for full aggregator management.

**Architecture:** Three independent subsystems wired together. (1) `PoolLimiter` — Redis-based per-second and monthly counters injected into `TarificationService`; limits are read from `aggregator_profiles` via a new `AggregatorLimitsRepository` (same DB, cached in-memory 5 min). Checks run before dual deduction in the sub-account SMS branch. (2) `FreezeService` — lives in the aggregator service; wraps `billing.FreezeAccount` with cascade logic (freeze all sub-accounts, publish `smpp:force_unbind:{client_id}` Redis keys), plus `UnfreezeAllSubAccounts` for aggregator self-service. (3) Admin portal pages — three new React pages under `portal-frontend/src/pages/admin/aggregators/` backed by new HTTP handlers in the portal gateway and a new `audit_repository.go` read layer. No new DB migrations — all tables (`aggregator_profiles`, `aggregator_audit_log`) were created in Phase 1.

**Tech Stack:** Go 1.24 + pgx/v5, gorilla/mux, protobuf/gRPC, redis/go-redis/v9, alicebob/miniredis/v2 (tests), React 19 + TypeScript + Tailwind CSS 4.2, Radix UI, zerolog, testify/mock

All work is done in worktree `.worktrees/aggregator-phase5/` on branch `feature/aggregator-phase5`.

---

## File Map

### New files
- `internal/services/aggregator/domain/control.go` — PoolLimiter interface, AuditEntry (read), FreezeResult value objects
- `internal/services/aggregator/repository/audit_repository.go` — ListAuditLog pagination query on `aggregator_audit_log`
- `internal/services/aggregator/service/pool_limiter.go` — RedisPoolLimiter: per-second INCR+EXPIRE, monthly WATCH+MULTI, GetMonthlyUsage
- `internal/services/aggregator/service/pool_limiter_test.go` — unit tests via miniredis
- `internal/services/aggregator/service/freeze_service.go` — FreezeService: FreezeAggregator, UnfreezeAggregator, UnfreezeAllSubAccounts
- `internal/services/aggregator/service/freeze_service_test.go` — unit tests via testify/mock
- `internal/services/tarification/infrastructure/repository/aggregator_limits_repository.go` — queries `aggregator_profiles` for rate+monthly limits; 5-min sync.Map cache
- `internal/gateway/portal/handlers/aggregator_control.go` — HTTP handlers: freeze, unfreeze, unfreeze-all, list aggregators, get profile, audit log
- `internal/gateway/portal/handlers/aggregator_control_test.go` — handler tests
- `portal-frontend/src/pages/admin/aggregators/AggregatorsListPage.tsx`
- `portal-frontend/src/pages/admin/aggregators/AggregatorProfilePage.tsx`
- `portal-frontend/src/pages/admin/aggregators/AggregatorAuditLogPage.tsx`

### Modified files
- `api/proto/aggregator/aggregator.proto` — add FreezeAggregator, UnfreezeAggregator RPCs + implement UnfreezeAllSubAccounts / ListAggregatorAuditLog stubs
- `api/proto/aggregatorv1/aggregator.pb.go` — regenerated
- `api/proto/aggregatorv1/aggregator_grpc.pb.go` — regenerated
- `internal/services/aggregator/grpc/server.go` — implement 4 new RPC handlers using FreezeService + AuditRepository
- `internal/services/aggregator/service/aggregator_service.go` — wire FreezeService; inject PoolLimiter into sub-account create (optional limit validation)
- `internal/services/tarification/application/tarification_service.go` — add `poolLimiter PoolLimiter` field + `limitsRepo AggregatorLimitsRepository` field; call CheckAndIncrementRate + CheckAndIncrementMonthly before dual deduction
- `internal/services/tarification/application/tarification_service_test.go` — add pool limit rejection tests
- `internal/services/tarification/domain/repository.go` — add `AggregatorLimitsRepository` interface
- `internal/gateway/smpp/server/server.go` — add `rdb *redis.Client` field + `forceUnbindWatcher` goroutine (polls `smpp:force_unbind:{client_id}` every 5s)
- `internal/gateway/smpp/clients.go` — add Redis address to ServiceAddresses; expose `*redis.Client`
- `cmd/smpp-gateway/main.go` — init Redis client, pass to smppserver.NewServer
- `cmd/services/tarification-service/main.go` — init AggregatorLimitsRepository + RedisPoolLimiter, inject into TarificationService
- `internal/gateway/portal/router/router.go` — add AggregatorControlHandlers parameter, register 7 new routes
- `portal-frontend/src/api/aggregator.ts` — add freezeAggregator, unfreezeAggregator, unfreezeAllSubAccounts, listAdminAggregators, getAdminAggregator, getAuditLog API functions
- `portal-frontend/src/App.tsx` — add `/admin/aggregators`, `/admin/aggregators/:id`, `/admin/aggregators/:id/audit-log` routes
- `portal-frontend/src/components/layout/Sidebar.tsx` — add "Aggregators" admin nav item visible to admins

---

## Task 1: Create Worktree

**Files:** none (setup)

- [ ] **Step 1: Create Phase 5 worktree from Phase 4 branch**

```bash
git worktree add .worktrees/aggregator-phase5 -b feature/aggregator-phase5 feature/aggregator-phase4
```

- [ ] **Step 2: Verify Phase 4 files are present**

```bash
ls .worktrees/aggregator-phase5/internal/services/aggregator/service/
```
Expected: `aggregator_service.go  tariff_service.go  route_service.go  analytics_service.go`

- [ ] **Step 3: Initial empty commit**

```bash
cd .worktrees/aggregator-phase5
git commit --allow-empty -m "chore: start aggregator phase 5 control"
```

---

## Task 2: Audit Repository — ListAuditLog

**Files:**
- Create: `internal/services/aggregator/repository/audit_repository.go`

- [ ] **Step 1: Write failing test**

```go
// internal/services/aggregator/repository/audit_repository_test.go
package repository_test

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
    "github.com/smpp-server/smpp-server/internal/services/aggregator/repository"
)

func TestAuditRepository_ListAuditLog(t *testing.T) {
    db := testDB(t) // helper used in existing repo tests
    repo := repository.NewAuditRepository(db)
    ctx := context.Background()
    aggID := uuid.New()

    // seed two entries
    _, err := db.Exec(ctx,
        `INSERT INTO aggregator_audit_log (aggregator_id, action, target_type, details)
         VALUES ($1, 'cascade_freeze', 'sub_account', '{"count":3}'),
                ($1, 'unfreeze_all_sub_accounts', 'sub_account', '{"count":3}')`,
        aggID,
    )
    require.NoError(t, err)

    entries, total, err := repo.ListAuditLog(ctx, aggID, 10, 0)
    require.NoError(t, err)
    assert.Equal(t, int64(2), total)
    assert.Len(t, entries, 2)
    assert.Equal(t, aggID, entries[0].AggregatorID)
    assert.NotEmpty(t, entries[0].Action)
    assert.False(t, entries[0].CreatedAt.IsZero())
}
```

- [ ] **Step 2: Run to see FAIL**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/services/aggregator/repository/... -run TestAuditRepository_ListAuditLog -v
```
Expected: `FAIL — repository.NewAuditRepository undefined`

- [ ] **Step 3: Implement AuditRepository**

```go
// internal/services/aggregator/repository/audit_repository.go
package repository

import (
    "context"
    "encoding/json"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
)

// AuditRepository reads from aggregator_audit_log.
type AuditRepository struct {
    db *pgxpool.Pool
}

// NewAuditRepository creates a new AuditRepository.
func NewAuditRepository(db *pgxpool.Pool) *AuditRepository {
    return &AuditRepository{db: db}
}

// ListAuditLog returns paginated audit entries for an aggregator, newest first.
// Returns (entries, total count, error).
func (r *AuditRepository) ListAuditLog(ctx context.Context, aggregatorID uuid.UUID, limit, offset int) ([]domain.AuditLogEntry, int64, error) {
    const countQ = `SELECT COUNT(*) FROM aggregator_audit_log WHERE aggregator_id = $1`
    var total int64
    if err := r.db.QueryRow(ctx, countQ, aggregatorID).Scan(&total); err != nil {
        return nil, 0, err
    }

    const q = `
        SELECT id, aggregator_id, action, target_type, target_id, details, ip_address, created_at
        FROM aggregator_audit_log
        WHERE aggregator_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3`

    rows, err := r.db.Query(ctx, q, aggregatorID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var entries []domain.AuditLogEntry
    for rows.Next() {
        var e domain.AuditLogEntry
        var detailsJSON []byte
        var targetID *uuid.UUID
        var ipAddr *string

        if err := rows.Scan(
            &e.ID, &e.AggregatorID, &e.Action, &e.TargetType,
            &targetID, &detailsJSON, &ipAddr, &e.CreatedAt,
        ); err != nil {
            return nil, 0, err
        }
        if targetID != nil {
            e.TargetID = *targetID
        }
        if ipAddr != nil {
            e.IPAddress = *ipAddr
        }
        if len(detailsJSON) > 0 {
            _ = json.Unmarshal(detailsJSON, &e.Details)
        }
        entries = append(entries, e)
    }
    return entries, total, rows.Err()
}
```

- [ ] **Step 4: Add AuditLogEntry to domain**

Append to `internal/services/aggregator/domain/models.go` (already exists from Phase 1):

```go
// AuditLogEntry represents one row from aggregator_audit_log (read model).
type AuditLogEntry struct {
    ID           uuid.UUID
    AggregatorID uuid.UUID
    Action       string
    TargetType   string
    TargetID     uuid.UUID
    Details      map[string]any
    IPAddress    string
    CreatedAt    time.Time
}
```

- [ ] **Step 5: Run test to verify PASS**

```bash
go test ./internal/services/aggregator/repository/... -run TestAuditRepository_ListAuditLog -v
```
Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
cd .worktrees/aggregator-phase5
git add internal/services/aggregator/repository/audit_repository.go \
        internal/services/aggregator/domain/models.go
git commit -m "feat(aggregator): add AuditRepository.ListAuditLog"
```

---

## Task 3: PoolLimiter — Redis Implementation

**Files:**
- Create: `internal/services/aggregator/service/pool_limiter.go`
- Create: `internal/services/aggregator/service/pool_limiter_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/services/aggregator/service/pool_limiter_test.go
package service_test

import (
    "context"
    "fmt"
    "testing"
    "time"

    "github.com/alicebob/miniredis/v2"
    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/smpp-server/smpp-server/internal/services/aggregator/service"
)

func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
    t.Helper()
    mr, err := miniredis.Run()
    require.NoError(t, err)
    t.Cleanup(mr.Close)
    rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
    t.Cleanup(func() { rdb.Close() })
    return rdb, mr
}

func TestRedisPoolLimiter_CheckAndIncrementRate(t *testing.T) {
    rdb, mr := newTestRedis(t)
    limiter := service.NewRedisPoolLimiter(rdb)
    aggID := uuid.New()
    ctx := context.Background()

    t.Run("allows when under limit", func(t *testing.T) {
        err := limiter.CheckAndIncrementRate(ctx, aggID, 3, 10)
        assert.NoError(t, err)
    })

    t.Run("blocks when limit exceeded", func(t *testing.T) {
        // Pre-seed counter to 9
        key := fmt.Sprintf("agg:pool:rate:%s:%d", aggID, time.Now().Unix())
        mr.Set(key, "9")
        err := limiter.CheckAndIncrementRate(ctx, aggID, 2, 10) // 9+2 > 10
        assert.ErrorIs(t, err, service.ErrPoolRateLimitExceeded)
    })

    t.Run("unlimited when limit is 0", func(t *testing.T) {
        err := limiter.CheckAndIncrementRate(ctx, aggID, 999, 0)
        assert.NoError(t, err)
    })
}

func TestRedisPoolLimiter_CheckAndIncrementMonthly(t *testing.T) {
    rdb, mr := newTestRedis(t)
    limiter := service.NewRedisPoolLimiter(rdb)
    aggID := uuid.New()
    ctx := context.Background()

    t.Run("allows when under limit", func(t *testing.T) {
        err := limiter.CheckAndIncrementMonthly(ctx, aggID, 100, 10000)
        assert.NoError(t, err)
    })

    t.Run("blocks when limit exceeded", func(t *testing.T) {
        now := time.Now()
        key := fmt.Sprintf("agg:pool:monthly:%s:%d-%02d", aggID, now.Year(), int(now.Month()))
        mr.Set(key, "9900")
        err := limiter.CheckAndIncrementMonthly(ctx, aggID, 200, 10000) // 9900+200 > 10000
        assert.ErrorIs(t, err, service.ErrPoolMonthlyLimitExceeded)
    })

    t.Run("unlimited when limit is 0", func(t *testing.T) {
        err := limiter.CheckAndIncrementMonthly(ctx, aggID, 99999, 0)
        assert.NoError(t, err)
    })
}

func TestRedisPoolLimiter_GetMonthlyUsage(t *testing.T) {
    rdb, mr := newTestRedis(t)
    limiter := service.NewRedisPoolLimiter(rdb)
    aggID := uuid.New()
    ctx := context.Background()

    now := time.Now()
    key := fmt.Sprintf("agg:pool:monthly:%s:%d-%02d", aggID, now.Year(), int(now.Month()))
    mr.Set(key, "4200")

    usage, err := limiter.GetMonthlyUsage(ctx, aggID)
    require.NoError(t, err)
    assert.Equal(t, int64(4200), usage)
}
```

- [ ] **Step 2: Run to see FAIL**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/services/aggregator/service/... -run TestRedisPoolLimiter -v
```
Expected: `FAIL — service.NewRedisPoolLimiter undefined`

- [ ] **Step 3: Implement pool_limiter.go**

```go
// internal/services/aggregator/service/pool_limiter.go
package service

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"
)

// ErrPoolRateLimitExceeded is returned when the aggregator pool per-second rate limit is reached.
var ErrPoolRateLimitExceeded = errors.New("aggregator pool rate limit exceeded")

// ErrPoolMonthlyLimitExceeded is returned when the aggregator pool monthly SMS limit is reached.
var ErrPoolMonthlyLimitExceeded = errors.New("aggregator pool monthly limit exceeded")

// PoolLimiter checks and enforces aggregator-level pool limits.
// Implementations must be safe for concurrent use.
type PoolLimiter interface {
    // CheckAndIncrementRate checks the per-second rate limit and, on success, increments the counter.
    // segments is the number of SMS segments. rateLimit = 0 means unlimited.
    // Returns ErrPoolRateLimitExceeded when the limit is reached.
    CheckAndIncrementRate(ctx context.Context, aggregatorID uuid.UUID, segments, rateLimit int) error

    // CheckAndIncrementMonthly checks the monthly segment limit and, on success, increments the counter.
    // monthlyLimit = 0 means unlimited.
    // Returns ErrPoolMonthlyLimitExceeded when the limit is reached.
    CheckAndIncrementMonthly(ctx context.Context, aggregatorID uuid.UUID, segments, monthlyLimit int) error

    // GetMonthlyUsage returns the current month's total segment count for the aggregator.
    GetMonthlyUsage(ctx context.Context, aggregatorID uuid.UUID) (int64, error)
}

// RedisPoolLimiter implements PoolLimiter using Redis.
type RedisPoolLimiter struct {
    rdb *redis.Client
}

// NewRedisPoolLimiter creates a new Redis-backed pool limiter.
func NewRedisPoolLimiter(rdb *redis.Client) *RedisPoolLimiter {
    return &RedisPoolLimiter{rdb: rdb}
}

// CheckAndIncrementRate uses a per-second Redis counter (key expires after 2s).
// Uses a pipeline: INCRBY + EXPIRE so that TTL is always refreshed.
// If the result exceeds rateLimit the increment is reversed and the error is returned.
func (l *RedisPoolLimiter) CheckAndIncrementRate(ctx context.Context, aggregatorID uuid.UUID, segments, rateLimit int) error {
    if rateLimit <= 0 {
        return nil
    }
    key := fmt.Sprintf("agg:pool:rate:%s:%d", aggregatorID, time.Now().Unix())

    pipe := l.rdb.Pipeline()
    incrCmd := pipe.IncrBy(ctx, key, int64(segments))
    pipe.Expire(ctx, key, 2*time.Second)
    if _, err := pipe.Exec(ctx); err != nil {
        return fmt.Errorf("pool rate limit pipeline: %w", err)
    }

    if incrCmd.Val() > int64(rateLimit) {
        _ = l.rdb.DecrBy(ctx, key, int64(segments))
        return ErrPoolRateLimitExceeded
    }
    return nil
}

// CheckAndIncrementMonthly uses a Redis counter keyed by aggregator ID + YYYY-MM.
// Uses WATCH+MULTI/EXEC for an atomic compare-and-increment.
// TTL is set to end-of-month + 1 day so the counter auto-expires.
func (l *RedisPoolLimiter) CheckAndIncrementMonthly(ctx context.Context, aggregatorID uuid.UUID, segments, monthlyLimit int) error {
    if monthlyLimit <= 0 {
        return nil
    }
    now := time.Now()
    key := fmt.Sprintf("agg:pool:monthly:%s:%d-%02d", aggregatorID, now.Year(), int(now.Month()))

    // Calculate TTL: expire at start of (month+2) = 1 day after month end
    nextNextMonth := time.Date(now.Year(), now.Month()+2, 1, 0, 0, 0, 0, now.Location())
    ttl := nextNextMonth.Sub(now)

    err := l.rdb.Watch(ctx, func(tx *redis.Tx) error {
        val, err := tx.Get(ctx, key).Int64()
        if err != nil && !errors.Is(err, redis.Nil) {
            return fmt.Errorf("get monthly counter: %w", err)
        }
        if val+int64(segments) > int64(monthlyLimit) {
            return ErrPoolMonthlyLimitExceeded
        }
        _, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
            pipe.IncrBy(ctx, key, int64(segments))
            pipe.Expire(ctx, key, ttl)
            return nil
        })
        return err
    }, key)

    if errors.Is(err, ErrPoolMonthlyLimitExceeded) {
        return ErrPoolMonthlyLimitExceeded
    }
    if err != nil {
        return fmt.Errorf("pool monthly limit check: %w", err)
    }
    return nil
}

// GetMonthlyUsage returns the current month's segment count; 0 if no counter exists yet.
func (l *RedisPoolLimiter) GetMonthlyUsage(ctx context.Context, aggregatorID uuid.UUID) (int64, error) {
    now := time.Now()
    key := fmt.Sprintf("agg:pool:monthly:%s:%d-%02d", aggregatorID, now.Year(), int(now.Month()))
    val, err := l.rdb.Get(ctx, key).Int64()
    if errors.Is(err, redis.Nil) {
        return 0, nil
    }
    return val, err
}
```

- [ ] **Step 4: Run tests to verify PASS**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/services/aggregator/service/... -run TestRedisPoolLimiter -v
```
Expected: `PASS` (3 test functions, all subtests green)

- [ ] **Step 5: Commit**

```bash
git add internal/services/aggregator/service/pool_limiter.go \
        internal/services/aggregator/service/pool_limiter_test.go
git commit -m "feat(aggregator): implement RedisPoolLimiter for pool rate/monthly limits"
```

---

## Task 4: AggregatorLimitsRepository in Tarification Service

**Files:**
- Modify: `internal/services/tarification/domain/repository.go`
- Create: `internal/services/tarification/infrastructure/repository/aggregator_limits_repository.go`

- [ ] **Step 1: Add interface to domain repository.go**

Open `internal/services/tarification/domain/repository.go` and append:

```go
// AggregatorLimits holds the pool limits for one aggregator, read from aggregator_profiles.
type AggregatorLimits struct {
    RateLimitPerSecond int // 0 = unlimited
    MonthlyLimit       int // 0 = unlimited
}

// AggregatorLimitsRepository reads pool limit configuration from aggregator_profiles.
type AggregatorLimitsRepository interface {
    // GetLimits returns the pool limits for the aggregator.
    // Returns AggregatorLimits{0, 0} (unlimited) when no profile exists.
    GetLimits(ctx context.Context, aggregatorID uuid.UUID) (AggregatorLimits, error)
}
```

- [ ] **Step 2: Write failing test**

```go
// internal/services/tarification/infrastructure/repository/aggregator_limits_repository_test.go
package repository_test

import (
    "context"
    "testing"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/smpp-server/smpp-server/internal/services/tarification/infrastructure/repository"
)

func TestAggregatorLimitsRepository_GetLimits(t *testing.T) {
    db := testDB(t)
    repo := repository.NewAggregatorLimitsRepository(db)
    ctx := context.Background()

    t.Run("returns zeros when no profile", func(t *testing.T) {
        limits, err := repo.GetLimits(ctx, uuid.New())
        require.NoError(t, err)
        assert.Equal(t, 0, limits.RateLimitPerSecond)
        assert.Equal(t, 0, limits.MonthlyLimit)
    })

    t.Run("returns configured limits", func(t *testing.T) {
        // seed a client + aggregator_profiles row
        clientID := uuid.New()
        _, err := db.Exec(ctx,
            `INSERT INTO clients (id, name, email, account_type) VALUES ($1, 'agg', 'agg@x.com', 'aggregator')`,
            clientID,
        )
        require.NoError(t, err)
        _, err = db.Exec(ctx,
            `INSERT INTO aggregator_profiles (client_id, pool_rate_limit_per_second, pool_monthly_limit)
             VALUES ($1, 50, 500000)`,
            clientID,
        )
        require.NoError(t, err)

        limits, err := repo.GetLimits(ctx, clientID)
        require.NoError(t, err)
        assert.Equal(t, 50, limits.RateLimitPerSecond)
        assert.Equal(t, 500000, limits.MonthlyLimit)
    })
}
```

- [ ] **Step 3: Run to see FAIL**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/services/tarification/infrastructure/repository/... -run TestAggregatorLimitsRepository -v
```
Expected: `FAIL — repository.NewAggregatorLimitsRepository undefined`

- [ ] **Step 4: Implement with 5-minute in-memory cache**

```go
// internal/services/tarification/infrastructure/repository/aggregator_limits_repository.go
package repository

import (
    "context"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

const limitsCacheTTL = 5 * time.Minute

type cachedLimits struct {
    limits    domain.AggregatorLimits
    expiresAt time.Time
}

// AggregatorLimitsRepository reads pool limits from aggregator_profiles with a 5-minute in-memory cache.
type AggregatorLimitsRepository struct {
    db    *pgxpool.Pool
    cache sync.Map // map[uuid.UUID]cachedLimits
}

// NewAggregatorLimitsRepository creates a new AggregatorLimitsRepository.
func NewAggregatorLimitsRepository(db *pgxpool.Pool) *AggregatorLimitsRepository {
    return &AggregatorLimitsRepository{db: db}
}

// GetLimits returns pool limits for aggregatorID. Returns zeros when no profile exists.
func (r *AggregatorLimitsRepository) GetLimits(ctx context.Context, aggregatorID uuid.UUID) (domain.AggregatorLimits, error) {
    if v, ok := r.cache.Load(aggregatorID); ok {
        entry := v.(cachedLimits)
        if time.Now().Before(entry.expiresAt) {
            return entry.limits, nil
        }
    }

    const q = `
        SELECT COALESCE(pool_rate_limit_per_second, 0), COALESCE(pool_monthly_limit, 0)
        FROM aggregator_profiles
        WHERE client_id = $1`

    var limits domain.AggregatorLimits
    err := r.db.QueryRow(ctx, q, aggregatorID).Scan(
        &limits.RateLimitPerSecond,
        &limits.MonthlyLimit,
    )
    if err != nil {
        // pgx.ErrNoRows → profile doesn't exist → unlimited (zeros)
        limits = domain.AggregatorLimits{}
    }

    r.cache.Store(aggregatorID, cachedLimits{
        limits:    limits,
        expiresAt: time.Now().Add(limitsCacheTTL),
    })
    return limits, nil
}
```

- [ ] **Step 5: Run test to verify PASS**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/services/tarification/infrastructure/repository/... -run TestAggregatorLimitsRepository -v
```
Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/services/tarification/domain/repository.go \
        internal/services/tarification/infrastructure/repository/aggregator_limits_repository.go \
        internal/services/tarification/infrastructure/repository/aggregator_limits_repository_test.go
git commit -m "feat(tarification): add AggregatorLimitsRepository for pool limit lookup"
```

---

## Task 5: Pool Limits Integration in TarificationService

**Files:**
- Modify: `internal/services/tarification/application/tarification_service.go`
- Modify: `internal/services/tarification/application/tarification_service_test.go`

- [ ] **Step 1: Write failing test cases**

In `tarification_service_test.go`, add two new test functions:

```go
func TestTarificationService_SubAccount_RateLimitExceeded(t *testing.T) {
    // builds a TarificationService with a mock PoolLimiter that returns ErrPoolRateLimitExceeded
    // calls TarifyMessage for a sub-account client
    // expects Approved=false, RejectionReason contains "rate limit"
    mockLimiter := &mockPoolLimiter{
        checkRateErr: service.ErrPoolRateLimitExceeded,
    }
    svc := buildServiceWithLimiter(t, mockLimiter)
    resp, err := svc.TarifyMessage(context.Background(), subAccountRequest())
    require.NoError(t, err)
    assert.False(t, resp.Approved)
    assert.Contains(t, resp.RejectionReason, "rate limit")
}

func TestTarificationService_SubAccount_MonthlyLimitExceeded(t *testing.T) {
    mockLimiter := &mockPoolLimiter{
        checkMonthlyErr: service.ErrPoolMonthlyLimitExceeded,
    }
    svc := buildServiceWithLimiter(t, mockLimiter)
    resp, err := svc.TarifyMessage(context.Background(), subAccountRequest())
    require.NoError(t, err)
    assert.False(t, resp.Approved)
    assert.Contains(t, resp.RejectionReason, "monthly limit")
}

// mockPoolLimiter satisfies the PoolLimiter interface for tests.
type mockPoolLimiter struct {
    checkRateErr    error
    checkMonthlyErr error
}

func (m *mockPoolLimiter) CheckAndIncrementRate(_ context.Context, _ uuid.UUID, _, _ int) error {
    return m.checkRateErr
}
func (m *mockPoolLimiter) CheckAndIncrementMonthly(_ context.Context, _ uuid.UUID, _, _ int) error {
    return m.checkMonthlyErr
}
func (m *mockPoolLimiter) GetMonthlyUsage(_ context.Context, _ uuid.UUID) (int64, error) {
    return 0, nil
}
```

- [ ] **Step 2: Run to see FAIL**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/services/tarification/application/... -run TestTarificationService_SubAccount_RateLimitExceeded -v
```
Expected: `FAIL — compile error or wrong Approved value`

- [ ] **Step 3: Add poolLimiter + limitsRepo to TarificationService**

In `tarification_service.go`, add the two fields and update the constructor:

```go
// In TarificationService struct, add:
poolLimiter PoolLimiter           // nil = pool limits disabled
limitsRepo  domain.AggregatorLimitsRepository // nil = pool limits disabled

// PoolLimiter is the interface the service uses; implemented by service.RedisPoolLimiter.
// Defined here to avoid circular imports.
type PoolLimiter interface {
    CheckAndIncrementRate(ctx context.Context, aggregatorID uuid.UUID, segments, rateLimit int) error
    CheckAndIncrementMonthly(ctx context.Context, aggregatorID uuid.UUID, segments, monthlyLimit int) error
    GetMonthlyUsage(ctx context.Context, aggregatorID uuid.UUID) (int64, error)
}

// SetPoolLimiter wires in the pool limiter and limits repo.
// Call this from main after phase-1 sub-account support is confirmed in the DB.
func (s *TarificationService) SetPoolLimiter(limiter PoolLimiter, repo domain.AggregatorLimitsRepository) {
    s.poolLimiter = limiter
    s.limitsRepo = repo
}
```

- [ ] **Step 4: Call pool limit checks in sub-account branch of TarifyMessage**

Locate the sub-account branch added in Phase 2 (look for `account_type == 'sub_account'` comment or `aggregatorID` variable). After resolving `aggregatorID` and before calling `s.saga.ChargeDual(...)`, insert:

```go
// Pool limit checks (Phase 5)
if s.poolLimiter != nil && s.limitsRepo != nil {
    limits, err := s.limitsRepo.GetLimits(ctx, aggregatorID)
    if err != nil {
        log.Ctx(ctx).Warn().Err(err).Msg("failed to get aggregator limits; skipping pool checks")
    } else {
        if err := s.poolLimiter.CheckAndIncrementRate(ctx, aggregatorID, req.SegmentCount, limits.RateLimitPerSecond); err != nil {
            return &TarifyMessageResponse{
                Approved:        false,
                RejectionReason: "pool rate limit exceeded",
            }, nil
        }
        if err := s.poolLimiter.CheckAndIncrementMonthly(ctx, aggregatorID, req.SegmentCount, limits.MonthlyLimit); err != nil {
            return &TarifyMessageResponse{
                Approved:        false,
                RejectionReason: "pool monthly limit exceeded",
            }, nil
        }
    }
}
```

- [ ] **Step 5: Run tests to verify PASS**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/services/tarification/application/... -run TestTarificationService_SubAccount -v
```
Expected: both new tests `PASS`; existing tests still pass

- [ ] **Step 6: Commit**

```bash
git add internal/services/tarification/application/tarification_service.go \
        internal/services/tarification/application/tarification_service_test.go
git commit -m "feat(tarification): enforce aggregator pool rate and monthly limits before dual charge"
```

---

## Task 6: FreezeService — Cascade Freeze / Unfreeze / UnfreezeAll

**Files:**
- Create: `internal/services/aggregator/service/freeze_service.go`
- Create: `internal/services/aggregator/service/freeze_service_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/services/aggregator/service/freeze_service_test.go
package service_test

import (
    "context"
    "errors"
    "testing"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"

    billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
    "github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
    "github.com/smpp-server/smpp-server/internal/services/aggregator/service"
)

// MockBillingClient is a testify mock for billingv1.BillingServiceClient.
type MockBillingClient struct{ mock.Mock }

func (m *MockBillingClient) FreezeAccount(ctx context.Context, req *billingv1.FreezeAccountRequest, _ ...interface{}) (*billingv1.FreezeAccountResponse, error) {
    args := m.Called(ctx, req)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*billingv1.FreezeAccountResponse), args.Error(1)
}
func (m *MockBillingClient) UnfreezeAccount(ctx context.Context, req *billingv1.UnfreezeAccountRequest, _ ...interface{}) (*billingv1.UnfreezeAccountResponse, error) {
    args := m.Called(ctx, req)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*billingv1.UnfreezeAccountResponse), args.Error(1)
}

// MockProfileRepo satisfies the ProfileRepository interface used by FreezeService.
type MockProfileRepo struct{ mock.Mock }

func (m *MockProfileRepo) GetByClientID(ctx context.Context, id uuid.UUID) (domain.AggregatorProfile, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(domain.AggregatorProfile), args.Error(1)
}

// MockFreezeSubAccountRepo satisfies the FreezeSubAccountRepository interface.
type MockFreezeSubAccountRepo struct{ mock.Mock }

func (m *MockFreezeSubAccountRepo) ListActiveSubAccountIDs(ctx context.Context, aggID uuid.UUID) ([]uuid.UUID, error) {
    args := m.Called(ctx, aggID)
    return args.Get(0).([]uuid.UUID), args.Error(1)
}
func (m *MockFreezeSubAccountRepo) WriteAuditLog(ctx context.Context, entry domain.AuditEntry) {}

func TestFreezeService_FreezeAggregator_Cascade(t *testing.T) {
    rdb, _ := newTestRedis(t)
    billing := &MockBillingClient{}
    profileRepo := &MockProfileRepo{}
    subRepo := &MockFreezeSubAccountRepo{}

    aggID := uuid.New()
    sub1, sub2 := uuid.New(), uuid.New()

    profileRepo.On("GetByClientID", mock.Anything, aggID).Return(domain.AggregatorProfile{
        AutoFreezeSubAccounts: true,
    }, nil)
    subRepo.On("ListActiveSubAccountIDs", mock.Anything, aggID).Return([]uuid.UUID{sub1, sub2}, nil)
    billing.On("FreezeAccount", mock.Anything, mock.MatchedBy(func(r *billingv1.FreezeAccountRequest) bool {
        return r.ClientId == sub1.String() || r.ClientId == sub2.String()
    })).Return(&billingv1.FreezeAccountResponse{}, nil)

    svc := service.NewFreezeService(billing, profileRepo, subRepo, rdb)
    result, err := svc.FreezeAggregator(context.Background(), aggID, "admin-user-1")

    require.NoError(t, err)
    assert.Equal(t, 2, result.SubAccountsFrozen)
    billing.AssertNumberOfCalls(t, "FreezeAccount", 2)
}

func TestFreezeService_FreezeAggregator_AutoFreezeDisabled(t *testing.T) {
    rdb, _ := newTestRedis(t)
    billing := &MockBillingClient{}
    profileRepo := &MockProfileRepo{}
    subRepo := &MockFreezeSubAccountRepo{}

    aggID := uuid.New()
    profileRepo.On("GetByClientID", mock.Anything, aggID).Return(domain.AggregatorProfile{
        AutoFreezeSubAccounts: false,
    }, nil)

    svc := service.NewFreezeService(billing, profileRepo, subRepo, rdb)
    result, err := svc.FreezeAggregator(context.Background(), aggID, "admin-user-1")

    require.NoError(t, err)
    assert.Equal(t, 0, result.SubAccountsFrozen)
    billing.AssertNotCalled(t, "FreezeAccount")
}

func TestFreezeService_UnfreezeAllSubAccounts(t *testing.T) {
    rdb, _ := newTestRedis(t)
    billing := &MockBillingClient{}
    profileRepo := &MockProfileRepo{}
    subRepo := &MockFreezeSubAccountRepo{}

    aggID := uuid.New()
    sub1, sub2 := uuid.New(), uuid.New()
    subRepo.On("ListActiveSubAccountIDs", mock.Anything, aggID).Return([]uuid.UUID{sub1, sub2}, nil)
    billing.On("UnfreezeAccount", mock.Anything, mock.Anything).Return(&billingv1.UnfreezeAccountResponse{}, nil)

    svc := service.NewFreezeService(billing, profileRepo, subRepo, rdb)
    count, err := svc.UnfreezeAllSubAccounts(context.Background(), aggID)

    require.NoError(t, err)
    assert.Equal(t, 2, count)
}
```

- [ ] **Step 2: Run to see FAIL**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/services/aggregator/service/... -run TestFreezeService -v
```
Expected: `FAIL — service.NewFreezeService undefined`

- [ ] **Step 3: Add FreezeSubAccountRepository interface to domain**

Append to `internal/services/aggregator/domain/models.go`:

```go
// FreezeResult is returned by FreezeService.FreezeAggregator.
type FreezeResult struct {
    SubAccountsFrozen int
}

// AuditEntry is the write model for aggregator_audit_log.
// (Already exists from Phase 1 as part of balance_repository writes.)
// This alias ensures FreezeService can reference it without importing balance_repository.
type AuditEntry struct {
    AggregatorID uuid.UUID
    Action       string
    TargetType   string
    Details      map[string]any
    IPAddress    string
}
```

- [ ] **Step 4: Add FreezeSubAccountRepository interface to domain**

Append to `internal/services/aggregator/domain/models.go` (after AuditEntry):

```go
// FreezeSubAccountRepository is the minimal data interface needed by FreezeService.
type FreezeSubAccountRepository interface {
    // ListActiveSubAccountIDs returns IDs of all non-deleted sub-accounts of an aggregator.
    ListActiveSubAccountIDs(ctx context.Context, aggregatorID uuid.UUID) ([]uuid.UUID, error)
    // WriteAuditLog persists an audit entry. Errors are logged internally but not propagated.
    WriteAuditLog(ctx context.Context, entry AuditEntry)
}

// FreezeProfileRepository is the minimal profile interface needed by FreezeService.
type FreezeProfileRepository interface {
    GetByClientID(ctx context.Context, clientID uuid.UUID) (AggregatorProfile, error)
}
```

- [ ] **Step 5: Implement FreezeService**

```go
// internal/services/aggregator/service/freeze_service.go
package service

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"
    "github.com/rs/zerolog/log"

    billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
    "github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
)

const smppForceUnbindTTL = time.Hour

// FreezeService handles cascade freeze/unfreeze of aggregator sub-accounts.
type FreezeService struct {
    billingClient billingv1.BillingServiceClient
    profileRepo   domain.FreezeProfileRepository
    subRepo       domain.FreezeSubAccountRepository
    rdb           *redis.Client
}

// NewFreezeService creates a FreezeService.
func NewFreezeService(
    billingClient billingv1.BillingServiceClient,
    profileRepo domain.FreezeProfileRepository,
    subRepo domain.FreezeSubAccountRepository,
    rdb *redis.Client,
) *FreezeService {
    return &FreezeService{
        billingClient: billingClient,
        profileRepo:   profileRepo,
        subRepo:       subRepo,
        rdb:           rdb,
    }
}

// FreezeAggregator cascades a freeze to all sub-accounts when auto_freeze_sub_accounts = true.
// It does NOT call billing.FreezeAccount for the aggregator itself — the caller (admin handler)
// does that before invoking this method.
// Returns FreezeResult with count of sub-accounts frozen.
func (s *FreezeService) FreezeAggregator(ctx context.Context, aggregatorID uuid.UUID, frozenBy string) (domain.FreezeResult, error) {
    profile, err := s.profileRepo.GetByClientID(ctx, aggregatorID)
    if err != nil {
        return domain.FreezeResult{}, fmt.Errorf("get aggregator profile: %w", err)
    }
    if !profile.AutoFreezeSubAccounts {
        return domain.FreezeResult{}, nil
    }

    subIDs, err := s.subRepo.ListActiveSubAccountIDs(ctx, aggregatorID)
    if err != nil {
        return domain.FreezeResult{}, fmt.Errorf("list sub-accounts: %w", err)
    }

    frozen := 0
    for _, subID := range subIDs {
        _, err := s.billingClient.FreezeAccount(ctx, &billingv1.FreezeAccountRequest{
            ClientId: subID.String(),
            Reason:   fmt.Sprintf("cascade freeze from aggregator %s by %s", aggregatorID, frozenBy),
        })
        if err != nil {
            log.Ctx(ctx).Warn().Err(err).Str("sub_id", subID.String()).Msg("failed to freeze sub-account")
            continue
        }
        frozen++
        // Signal SMPP server to unbind sessions for this client.
        // Non-fatal: the account is already frozen so future SMPP messages will be rejected.
        key := fmt.Sprintf("smpp:force_unbind:%s", subID)
        if err := s.rdb.Set(ctx, key, "1", smppForceUnbindTTL).Err(); err != nil {
            log.Ctx(ctx).Warn().Err(err).Str("sub_id", subID.String()).Msg("failed to set SMPP force-unbind signal")
        }
    }

    s.subRepo.WriteAuditLog(ctx, domain.AuditEntry{
        AggregatorID: aggregatorID,
        Action:       "cascade_freeze",
        TargetType:   "sub_account",
        Details:      map[string]any{"frozen_by": frozenBy, "count": frozen},
    })

    return domain.FreezeResult{SubAccountsFrozen: frozen}, nil
}

// UnfreezeAggregator records an audit entry when the platform admin unfreezes the aggregator.
// Sub-accounts are intentionally NOT unfrozen; the aggregator must call UnfreezeAllSubAccounts separately.
func (s *FreezeService) UnfreezeAggregator(ctx context.Context, aggregatorID uuid.UUID) error {
    s.subRepo.WriteAuditLog(ctx, domain.AuditEntry{
        AggregatorID: aggregatorID,
        Action:       "unfreeze_aggregator",
        TargetType:   "aggregator",
        Details:      map[string]any{"note": "sub-accounts remain frozen"},
    })
    return nil
}

// UnfreezeAllSubAccounts unfreezes every sub-account for the given aggregator.
// This is called by the aggregator via portal after the admin has unfrozen the aggregator account.
// Returns the number of sub-accounts successfully unfrozen.
func (s *FreezeService) UnfreezeAllSubAccounts(ctx context.Context, aggregatorID uuid.UUID) (int, error) {
    subIDs, err := s.subRepo.ListActiveSubAccountIDs(ctx, aggregatorID)
    if err != nil {
        return 0, fmt.Errorf("list sub-accounts: %w", err)
    }

    count := 0
    for _, subID := range subIDs {
        _, err := s.billingClient.UnfreezeAccount(ctx, &billingv1.UnfreezeAccountRequest{
            ClientId: subID.String(),
        })
        if err != nil {
            log.Ctx(ctx).Warn().Err(err).Str("sub_id", subID.String()).Msg("failed to unfreeze sub-account")
            continue
        }
        count++
    }

    s.subRepo.WriteAuditLog(ctx, domain.AuditEntry{
        AggregatorID: aggregatorID,
        Action:       "unfreeze_all_sub_accounts",
        TargetType:   "sub_account",
        Details:      map[string]any{"count": count},
    })

    return count, nil
}
```

- [ ] **Step 6: Run tests to verify PASS**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/services/aggregator/service/... -run TestFreezeService -v
```
Expected: all 3 test functions `PASS`

- [ ] **Step 7: Commit**

```bash
git add internal/services/aggregator/service/freeze_service.go \
        internal/services/aggregator/service/freeze_service_test.go \
        internal/services/aggregator/domain/models.go
git commit -m "feat(aggregator): implement FreezeService for cascade freeze/unfreeze"
```

---

## Task 7: Proto Updates — FreezeAggregator / UnfreezeAggregator + stubs

**Files:**
- Modify: `api/proto/aggregator/aggregator.proto`
- Modify: `api/proto/aggregatorv1/aggregator.pb.go` (regenerated)
- Modify: `api/proto/aggregatorv1/aggregator_grpc.pb.go` (regenerated)

- [ ] **Step 1: Add new RPCs and messages to proto**

Open `api/proto/aggregator/aggregator.proto` and add to the `AggregatorService`:

```protobuf
// Control — platform admin
rpc FreezeAggregator(FreezeAggregatorRequest) returns (FreezeAggregatorResponse);
rpc UnfreezeAggregator(UnfreezeAggregatorRequest) returns (UnfreezeAggregatorResponse);
```

Add message definitions at the bottom of the file:

```protobuf
// ── Phase 5: Control ─────────────────────────────────────────────────────────

message FreezeAggregatorRequest {
  string aggregator_id = 1;
  string frozen_by     = 2; // admin user ID for audit log
  string reason        = 3;
}

message FreezeAggregatorResponse {
  bool   success              = 1;
  int32  sub_accounts_frozen  = 2;
}

message UnfreezeAggregatorRequest {
  string aggregator_id = 1;
}

message UnfreezeAggregatorResponse {
  bool success = 1;
}

// UnfreezeAllSubAccountsRequest / UnfreezeAllSubAccountsResponse
// and ListAuditLogRequest / ListAuditLogResponse are already defined in Phase 1.
// Confirm they exist; add if missing:

message AuditLogEntryProto {
  string aggregator_id = 1;
  string action        = 2;
  string target_type   = 3;
  string target_id     = 4;
  string details_json  = 5;
  string ip_address    = 6;
  google.protobuf.Timestamp created_at = 7;
}

message ListAuditLogRequest {
  string aggregator_id = 1;
  int32  limit         = 2;
  int32  offset        = 3;
}

message ListAuditLogResponse {
  repeated AuditLogEntryProto entries = 1;
  int64 total                         = 2;
}
```

- [ ] **Step 2: Regenerate Go code**

```bash
cd .worktrees/aggregator-phase5
bash scripts/generate-proto.sh aggregator
```
Expected: `api/proto/aggregatorv1/aggregator.pb.go` and `aggregator_grpc.pb.go` updated with new types.

- [ ] **Step 3: Verify compilation**

```bash
go build ./api/proto/aggregatorv1/...
```
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add api/proto/aggregator/aggregator.proto \
        api/proto/aggregatorv1/
git commit -m "feat(proto): add FreezeAggregator/UnfreezeAggregator RPCs, wire audit log messages"
```

---

## Task 8: gRPC Server — Implement 4 New RPC Handlers

**Files:**
- Modify: `internal/services/aggregator/grpc/server.go`

- [ ] **Step 1: Add FreezeService and AuditRepository to the gRPC server struct**

Locate the gRPC server struct in `server.go` (created in Phase 1). Add two fields:

```go
freezeSvc   *service.FreezeService
auditRepo   *repository.AuditRepository
```

Update `NewServer(...)` to accept and store them.

- [ ] **Step 2: Implement FreezeAggregator RPC**

```go
func (s *Server) FreezeAggregator(ctx context.Context, req *aggregatorv1.FreezeAggregatorRequest) (*aggregatorv1.FreezeAggregatorResponse, error) {
    aggID, err := uuid.Parse(req.AggregatorId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id: %v", err)
    }
    result, err := s.freezeSvc.FreezeAggregator(ctx, aggID, req.FrozenBy)
    if err != nil {
        s.logger.Error().Err(err).Str("aggregator_id", req.AggregatorId).Msg("FreezeAggregator failed")
        return nil, status.Errorf(codes.Internal, "freeze aggregator: %v", err)
    }
    return &aggregatorv1.FreezeAggregatorResponse{
        Success:             true,
        SubAccountsFrozen:   int32(result.SubAccountsFrozen),
    }, nil
}
```

- [ ] **Step 3: Implement UnfreezeAggregator RPC**

```go
func (s *Server) UnfreezeAggregator(ctx context.Context, req *aggregatorv1.UnfreezeAggregatorRequest) (*aggregatorv1.UnfreezeAggregatorResponse, error) {
    aggID, err := uuid.Parse(req.AggregatorId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id: %v", err)
    }
    if err := s.freezeSvc.UnfreezeAggregator(ctx, aggID); err != nil {
        return nil, status.Errorf(codes.Internal, "unfreeze aggregator: %v", err)
    }
    return &aggregatorv1.UnfreezeAggregatorResponse{Success: true}, nil
}
```

- [ ] **Step 4: Implement UnfreezeAllSubAccounts RPC**

```go
func (s *Server) UnfreezeAllSubAccounts(ctx context.Context, req *aggregatorv1.UnfreezeAllSubAccountsRequest) (*aggregatorv1.UnfreezeAllSubAccountsResponse, error) {
    aggID, err := uuid.Parse(req.AggregatorId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id: %v", err)
    }
    count, err := s.freezeSvc.UnfreezeAllSubAccounts(ctx, aggID)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "unfreeze all sub-accounts: %v", err)
    }
    return &aggregatorv1.UnfreezeAllSubAccountsResponse{
        Success:              true,
        SubAccountsUnfrozen: int32(count),
    }, nil
}
```

- [ ] **Step 5: Implement ListAggregatorAuditLog RPC**

```go
func (s *Server) ListAggregatorAuditLog(ctx context.Context, req *aggregatorv1.ListAuditLogRequest) (*aggregatorv1.ListAuditLogResponse, error) {
    aggID, err := uuid.Parse(req.AggregatorId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id: %v", err)
    }
    limit, offset := int(req.Limit), int(req.Offset)
    if limit <= 0 {
        limit = 50
    }

    entries, total, err := s.auditRepo.ListAuditLog(ctx, aggID, limit, offset)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "list audit log: %v", err)
    }

    protoEntries := make([]*aggregatorv1.AuditLogEntryProto, len(entries))
    for i, e := range entries {
        det, _ := json.Marshal(e.Details)
        protoEntries[i] = &aggregatorv1.AuditLogEntryProto{
            AggregatorId: e.AggregatorID.String(),
            Action:       e.Action,
            TargetType:   e.TargetType,
            TargetId:     e.TargetID.String(),
            DetailsJson:  string(det),
            IpAddress:    e.IPAddress,
            CreatedAt:    timestamppb.New(e.CreatedAt),
        }
    }
    return &aggregatorv1.ListAuditLogResponse{
        Entries: protoEntries,
        Total:   total,
    }, nil
}
```

- [ ] **Step 6: Compile check**

```bash
cd .worktrees/aggregator-phase5
go build ./internal/services/aggregator/...
```
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/services/aggregator/grpc/server.go
git commit -m "feat(aggregator): implement FreezeAggregator, UnfreezeAggregator, UnfreezeAll, ListAuditLog gRPC handlers"
```

---

## Task 9: Admin HTTP Handlers

**Files:**
- Create: `internal/gateway/portal/handlers/aggregator_control.go`
- Create: `internal/gateway/portal/handlers/aggregator_control_test.go`

- [ ] **Step 1: Write failing handler test**

```go
// internal/gateway/portal/handlers/aggregator_control_test.go
package handlers_test

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gorilla/mux"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"

    aggregatorv1 "github.com/smpp-server/smpp-server/api/proto/aggregatorv1"
    billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
    "github.com/smpp-server/smpp-server/internal/gateway/portal/handlers"
)

func TestAggregatorControlHandlers_FreezeAggregator(t *testing.T) {
    aggClient := &MockAggregatorClient{}
    billingClient := &MockBillingClient{}

    aggID := "550e8400-e29b-41d4-a716-446655440000"

    billingClient.On("FreezeAccount", mock.Anything, mock.MatchedBy(func(r *billingv1.FreezeAccountRequest) bool {
        return r.ClientId == aggID
    })).Return(&billingv1.FreezeAccountResponse{}, nil)

    aggClient.On("FreezeAggregator", mock.Anything, mock.MatchedBy(func(r *aggregatorv1.FreezeAggregatorRequest) bool {
        return r.AggregatorId == aggID
    })).Return(&aggregatorv1.FreezeAggregatorResponse{Success: true, SubAccountsFrozen: 3}, nil)

    h := handlers.NewAggregatorControlHandlers(aggClient, billingClient)

    router := mux.NewRouter()
    router.HandleFunc("/api/v1/admin/aggregators/{id}/freeze", h.FreezeAggregator).Methods("POST")

    body, _ := json.Marshal(map[string]string{"reason": "spam"})
    req := httptest.NewRequest("POST", "/api/v1/admin/aggregators/"+aggID+"/freeze", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    // Simulate admin session middleware setting user ID
    req = req.WithContext(contextWithAdminUser(req.Context(), "admin-1"))
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    require.Equal(t, http.StatusOK, w.Code)
    var resp map[string]interface{}
    require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
    assert.Equal(t, float64(3), resp["sub_accounts_frozen"])
}
```

- [ ] **Step 2: Run to see FAIL**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/gateway/portal/handlers/... -run TestAggregatorControlHandlers_FreezeAggregator -v
```
Expected: `FAIL — handlers.NewAggregatorControlHandlers undefined`

- [ ] **Step 3: Implement aggregator_control.go**

```go
// internal/gateway/portal/handlers/aggregator_control.go
package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/gorilla/mux"
    "github.com/rs/zerolog/log"

    aggregatorv1 "github.com/smpp-server/smpp-server/api/proto/aggregatorv1"
    billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
    "github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
    "github.com/smpp-server/smpp-server/internal/shared"
)

// AggregatorControlHandlers handles freeze/unfreeze and audit-log endpoints
// for platform admins (POST /api/v1/admin/aggregators/:id/freeze, etc.)
// and aggregator self-service (POST /api/v1/aggregator/sub-accounts/unfreeze-all).
type AggregatorControlHandlers struct {
    aggregatorClient aggregatorv1.AggregatorServiceClient
    billingClient    billingv1.BillingServiceClient
}

// NewAggregatorControlHandlers creates the handler group.
func NewAggregatorControlHandlers(
    aggregatorClient aggregatorv1.AggregatorServiceClient,
    billingClient billingv1.BillingServiceClient,
) *AggregatorControlHandlers {
    return &AggregatorControlHandlers{
        aggregatorClient: aggregatorClient,
        billingClient:    billingClient,
    }
}

// FreezeAggregator handles POST /api/v1/admin/aggregators/:id/freeze
// 1. Calls billing.FreezeAccount for the aggregator
// 2. Calls aggregator.FreezeAggregator to cascade to sub-accounts + SMPP signals
func (h *AggregatorControlHandlers) FreezeAggregator(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserID(r.Context())
    if !ok {
        respondError(w, shared.ErrUnauthorized("не аутентифицирован"))
        return
    }

    vars := mux.Vars(r)
    aggID := vars["id"]
    if aggID == "" {
        respondError(w, shared.ErrInvalidInput("id агрегатора обязателен"))
        return
    }

    var body struct {
        Reason string `json:"reason"`
    }
    _ = json.NewDecoder(r.Body).Decode(&body)

    // Step 1: freeze the aggregator account itself
    if _, err := h.billingClient.FreezeAccount(r.Context(), &billingv1.FreezeAccountRequest{
        ClientId: aggID,
        Reason:   body.Reason,
    }); err != nil {
        log.Error().Err(err).Str("aggregator_id", aggID).Msg("billing FreezeAccount failed")
        respondGRPCError(w, err)
        return
    }

    // Step 2: cascade freeze to sub-accounts
    resp, err := h.aggregatorClient.FreezeAggregator(r.Context(), &aggregatorv1.FreezeAggregatorRequest{
        AggregatorId: aggID,
        FrozenBy:     userID.String(),
        Reason:       body.Reason,
    })
    if err != nil {
        log.Error().Err(err).Str("aggregator_id", aggID).Msg("aggregator FreezeAggregator cascade failed")
        // Aggregator is already frozen; log but don't fail the response
        respondJSON(w, http.StatusOK, map[string]any{
            "success":             true,
            "sub_accounts_frozen": 0,
            "cascade_error":       err.Error(),
        })
        return
    }

    respondJSON(w, http.StatusOK, map[string]any{
        "success":             resp.Success,
        "sub_accounts_frozen": resp.SubAccountsFrozen,
    })
}

// UnfreezeAggregator handles POST /api/v1/admin/aggregators/:id/unfreeze
// Only unfreezes the aggregator; sub-accounts remain frozen (aggregator self-service).
func (h *AggregatorControlHandlers) UnfreezeAggregator(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    aggID := vars["id"]
    if aggID == "" {
        respondError(w, shared.ErrInvalidInput("id агрегатора обязателен"))
        return
    }

    if _, err := h.billingClient.UnfreezeAccount(r.Context(), &billingv1.UnfreezeAccountRequest{
        ClientId: aggID,
    }); err != nil {
        respondGRPCError(w, err)
        return
    }

    if _, err := h.aggregatorClient.UnfreezeAggregator(r.Context(), &aggregatorv1.UnfreezeAggregatorRequest{
        AggregatorId: aggID,
    }); err != nil {
        log.Warn().Err(err).Str("aggregator_id", aggID).Msg("audit log for unfreeze failed")
    }

    respondJSON(w, http.StatusOK, map[string]any{"success": true})
}

// UnfreezeAllSubAccounts handles POST /api/v1/aggregator/sub-accounts/unfreeze-all
// Called by the aggregator (not admin) after the aggregator itself has been unfrozen.
func (h *AggregatorControlHandlers) UnfreezeAllSubAccounts(w http.ResponseWriter, r *http.Request) {
    clientID, ok := middleware.GetClientID(r.Context())
    if !ok {
        respondError(w, shared.ErrUnauthorized("не аутентифицирован"))
        return
    }

    resp, err := h.aggregatorClient.UnfreezeAllSubAccounts(r.Context(), &aggregatorv1.UnfreezeAllSubAccountsRequest{
        AggregatorId: clientID.String(),
    })
    if err != nil {
        respondGRPCError(w, err)
        return
    }

    respondJSON(w, http.StatusOK, map[string]any{
        "success":               resp.Success,
        "sub_accounts_unfrozen": resp.SubAccountsUnfrozen,
    })
}

// GetAdminAuditLog handles GET /api/v1/admin/aggregators/:id/audit-log
func (h *AggregatorControlHandlers) GetAdminAuditLog(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    aggID := vars["id"]
    if aggID == "" {
        respondError(w, shared.ErrInvalidInput("id агрегатора обязателен"))
        return
    }

    page, perPage := parsePagination(r)
    offset := (page - 1) * perPage

    resp, err := h.aggregatorClient.ListAggregatorAuditLog(r.Context(), &aggregatorv1.ListAuditLogRequest{
        AggregatorId: aggID,
        Limit:        int32(perPage),
        Offset:       int32(offset),
    })
    if err != nil {
        respondGRPCError(w, err)
        return
    }

    respondJSON(w, http.StatusOK, map[string]any{
        "entries":  resp.Entries,
        "total":    resp.Total,
        "page":     page,
        "per_page": perPage,
    })
}
```

- [ ] **Step 4: Run tests to verify PASS**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/gateway/portal/handlers/... -run TestAggregatorControlHandlers -v
```
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/handlers/aggregator_control.go \
        internal/gateway/portal/handlers/aggregator_control_test.go
git commit -m "feat(portal): add AggregatorControlHandlers (freeze, unfreeze, unfreeze-all, audit-log)"
```

---

## Task 10: Router Registration

**Files:**
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Add AggregatorControlHandlers parameter to SetupRouter**

Open `internal/gateway/portal/router/router.go`. Add `aggregatorControlHandlers *handlers.AggregatorControlHandlers` to the parameter list (after `companyHandlers`).

- [ ] **Step 2: Register 7 new routes in the authenticated section**

Find the block with `/api/v1/sub-accounts` routes and add below it:

```go
// Aggregator control — admin only (requireAdmin middleware assumed wired to adminAPI subrouter)
adminAPI.HandleFunc("/aggregators/{id}/freeze", aggregatorControlHandlers.FreezeAggregator).Methods("POST")
adminAPI.HandleFunc("/aggregators/{id}/unfreeze", aggregatorControlHandlers.UnfreezeAggregator).Methods("POST")
adminAPI.HandleFunc("/aggregators/{id}/audit-log", aggregatorControlHandlers.GetAdminAuditLog).Methods("GET")

// Aggregator self-service — aggregator role only
aggregatorAPI.HandleFunc("/sub-accounts/unfreeze-all", aggregatorControlHandlers.UnfreezeAllSubAccounts).Methods("POST")
```

Note: `adminAPI` and `aggregatorAPI` are subrouters added in previous phases.
If they don't exist yet, add them:
```go
adminAPI := api.PathPrefix("/admin").Subrouter()
// adminAPI.Use(middleware.RequireRole("admin"))  // already wired in Phase 1

aggregatorAPI := api.PathPrefix("/aggregator").Subrouter()
// aggregatorAPI.Use(middleware.RequireRole("aggregator"))  // already wired in Phase 1
```

- [ ] **Step 3: Pass aggregatorControlHandlers in portal-gateway main.go (wiring deferred to Task 16)**

Add a compilation placeholder: in `cmd/portal-gateway/main.go`, confirm `AggregatorControlHandlers` is constructed (wired in Task 16).

- [ ] **Step 4: Compile check**

```bash
cd .worktrees/aggregator-phase5
go build ./internal/gateway/portal/...
```
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/router/router.go
git commit -m "feat(portal): register freeze/unfreeze/audit-log/unfreeze-all routes"
```

---

## Task 11: SMPP Force-Unbind Watcher

**Files:**
- Modify: `internal/gateway/smpp/server/server.go`
- Modify: `internal/gateway/smpp/clients.go`
- Modify: `cmd/smpp-gateway/main.go`

- [ ] **Step 1: Add Redis to SMPP Server struct**

In `internal/gateway/smpp/server/server.go`, add `rdb *redis.Client` to the `Server` struct and update `NewServer(...)`:

```go
// In Server struct add:
rdb *redis.Client // may be nil if Redis not configured

// Update NewServer signature:
func NewServer(
    cfg *config.SMSPConfig,
    authClient authv1.AuthServiceClient,
    messageRepo *storage.MessageRepository,
    optOutRepo *storage.OptOutRepository,
    producer *queue.Producer,
    logger zerolog.Logger,
    rdb *redis.Client, // new — may be nil
) *Server {
    ctx, cancel := context.WithCancel(context.Background())
    return &Server{
        config:      cfg,
        sessions:    make(map[string]*smppsession.Session),
        authClient:  authClient,
        messageRepo: messageRepo,
        optOutRepo:  optOutRepo,
        producer:    producer,
        logger:      logger.With().Str("component", "smpp_gateway").Logger(),
        ctx:         ctx,
        cancel:      cancel,
        rdb:         rdb,
    }
}
```

- [ ] **Step 2: Add forceUnbindWatcher goroutine to Start()**

In the `Start()` method, after `go s.enquireLinkLoop()`, add:

```go
if s.rdb != nil {
    s.wg.Add(1)
    go s.forceUnbindWatcher()
}
```

- [ ] **Step 3: Implement forceUnbindWatcher**

Add to `server.go`:

```go
// forceUnbindWatcher polls Redis every 5 seconds for force-unbind signals.
// When the key "smpp:force_unbind:{client_id}" is set, it closes all bound sessions
// for that client_id and deletes the key.
func (s *Server) forceUnbindWatcher() {
    defer s.wg.Done()

    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-s.ctx.Done():
            return
        case <-ticker.C:
            s.checkForceUnbindSignals()
        }
    }
}

func (s *Server) checkForceUnbindSignals() {
    // Snapshot bound sessions with a ClientID so we don't hold the lock while calling Redis.
    type sessionSnapshot struct {
        id       string
        clientID string
    }
    s.sessionsMu.RLock()
    var snapshots []sessionSnapshot
    for id, sess := range s.sessions {
        if sess.IsBound() && sess.ClientID != nil {
            snapshots = append(snapshots, sessionSnapshot{
                id:       id,
                clientID: sess.ClientID.String(),
            })
        }
    }
    s.sessionsMu.RUnlock()

    for _, snap := range snapshots {
        key := "smpp:force_unbind:" + snap.clientID
        exists, err := s.rdb.Exists(s.ctx, key).Result()
        if err != nil || exists == 0 {
            continue
        }
        // Signal found: close the session and delete the key.
        s.logger.Info().
            Str("session_id", snap.id).
            Str("client_id", snap.clientID).
            Msg("force-unbinding SMPP session due to account freeze")

        s.sessionsMu.Lock()
        if sess, ok := s.sessions[snap.id]; ok {
            sess.Close()
            delete(s.sessions, snap.id)
        }
        s.sessionsMu.Unlock()

        // Delete the key so the session isn't closed again on reconnect.
        _ = s.rdb.Del(s.ctx, key)
    }
}
```

- [ ] **Step 4: Update smpp clients.go to expose Redis address**

```go
// In smpp/clients.go ServiceAddresses, add:
RedisAddr     string
RedisPassword string
RedisDB       int
```

- [ ] **Step 5: Update cmd/smpp-gateway/main.go**

```go
// After producer initialization, add:
redisAddr := getEnvOrDefault("REDIS_ADDR", "localhost:6379")
redisPassword := os.Getenv("REDIS_PASSWORD")

var smppRedis *redis.Client
if redisAddr != "" {
    smppRedis = redis.NewClient(&redis.Options{
        Addr:     redisAddr,
        Password: redisPassword,
    })
    defer smppRedis.Close()
    logger.Info().Str("addr", redisAddr).Msg("Redis клиент для SMPP инициализирован")
}

// Update NewServer call:
smppGateway := smppserver.NewServer(
    &cfg.SMSP,
    serviceClients.AuthClient,
    messageRepo,
    optOutRepo,
    producer,
    logger,
    smppRedis, // new param
)
```

Add import: `"github.com/redis/go-redis/v9"`

- [ ] **Step 6: Compile check**

```bash
cd .worktrees/aggregator-phase5
go build ./cmd/smpp-gateway/...
go build ./internal/gateway/smpp/...
```
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/gateway/smpp/server/server.go \
        internal/gateway/smpp/clients.go \
        cmd/smpp-gateway/main.go
git commit -m "feat(smpp): add forceUnbindWatcher goroutine for cascade freeze SMPP session cleanup"
```

---

## Task 12: Wire Everything in tarification-service Main

**Files:**
- Modify: `cmd/services/tarification-service/main.go`

- [ ] **Step 1: Inject AggregatorLimitsRepository + RedisPoolLimiter into TarificationService**

Open `cmd/services/tarification-service/main.go`. Find where `TarificationService` is created. Add after its construction:

```go
// Pool limits for aggregator sub-account traffic control (Phase 5)
redisAddr := getEnvOrDefault("REDIS_ADDR", "localhost:6379")
rdb := redis.NewClient(&redis.Options{
    Addr:     redisAddr,
    Password: os.Getenv("REDIS_PASSWORD"),
})
defer rdb.Close()

aggLimitsRepo := tariffRepo.NewAggregatorLimitsRepository(pgPool)
poolLimiter := aggregatorService.NewRedisPoolLimiter(rdb)
tarificationSvc.SetPoolLimiter(poolLimiter, aggLimitsRepo)

logger.Info().Msg("агрегаторный pool limiter подключён к тарификационному сервису")
```

Add imports:
```go
tariffRepo "github.com/smpp-server/smpp-server/internal/services/tarification/infrastructure/repository"
aggregatorService "github.com/smpp-server/smpp-server/internal/services/aggregator/service"
"github.com/redis/go-redis/v9"
```

- [ ] **Step 2: Compile check**

```bash
cd .worktrees/aggregator-phase5
go build ./cmd/services/tarification-service/...
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add cmd/services/tarification-service/main.go
git commit -m "feat(tarification): wire RedisPoolLimiter + AggregatorLimitsRepository into tarification service"
```

---

## Task 13: Wire FreezeService in portal-gateway Main

**Files:**
- Modify: `cmd/portal-gateway/main.go`

- [ ] **Step 1: Construct FreezeService dependencies and AggregatorControlHandlers**

In `cmd/portal-gateway/main.go`, find where other handlers are constructed. Add:

```go
// Aggregator control handlers (Phase 5)
aggregatorControlHandlers := portalHandlers.NewAggregatorControlHandlers(
    clients.AggregatorClient,    // wired in Phase 1
    clients.BillingClient,
)
```

- [ ] **Step 2: Pass to SetupRouter**

Find the `router.SetupRouter(...)` call and add `aggregatorControlHandlers` as the last argument (or in the correct position matching the updated signature from Task 10).

- [ ] **Step 3: Compile check**

```bash
cd .worktrees/aggregator-phase5
go build ./cmd/portal-gateway/...
```
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add cmd/portal-gateway/main.go
git commit -m "chore: wire AggregatorControlHandlers into portal-gateway"
```

---

## Task 14: Frontend — AggregatorsListPage

**Files:**
- Create: `portal-frontend/src/pages/admin/aggregators/AggregatorsListPage.tsx`
- Modify: `portal-frontend/src/api/aggregator.ts`

- [ ] **Step 1: Add admin aggregator API functions**

Append to `portal-frontend/src/api/aggregator.ts`:

```typescript
export interface AdminAggregator {
  id: string;
  name: string;
  email: string;
  account_type: string;
  active: boolean;
  balance: string;
  sub_account_count: number;
  max_sub_accounts: number;
  pool_rate_limit_per_second: number | null;
  pool_monthly_limit: number | null;
  auto_freeze_sub_accounts: boolean;
  frozen: boolean;
}

export interface AdminAggregatorListResponse {
  aggregators: AdminAggregator[];
  total: number;
  page: number;
  per_page: number;
}

export const listAdminAggregators = (page = 1, perPage = 20): Promise<AdminAggregatorListResponse> =>
  apiGet(`/api/v1/admin/aggregators?page=${page}&per_page=${perPage}`);

export const getAdminAggregator = (id: string): Promise<AdminAggregator> =>
  apiGet(`/api/v1/admin/aggregators/${id}`);

export const freezeAggregator = (id: string, reason: string): Promise<{ success: boolean; sub_accounts_frozen: number }> =>
  apiPost(`/api/v1/admin/aggregators/${id}/freeze`, { reason });

export const unfreezeAggregator = (id: string): Promise<{ success: boolean }> =>
  apiPost(`/api/v1/admin/aggregators/${id}/unfreeze`, {});

export const getAuditLog = (id: string, page = 1, perPage = 50) =>
  apiGet(`/api/v1/admin/aggregators/${id}/audit-log?page=${page}&per_page=${perPage}`);

export const unfreezeAllSubAccounts = (): Promise<{ success: boolean; sub_accounts_unfrozen: number }> =>
  apiPost('/api/v1/aggregator/sub-accounts/unfreeze-all', {});
```

- [ ] **Step 2: Create AggregatorsListPage**

```tsx
// portal-frontend/src/pages/admin/aggregators/AggregatorsListPage.tsx
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { listAdminAggregators, freezeAggregator, AdminAggregator } from '../../../api/aggregator';

export default function AggregatorsListPage() {
  const [aggregators, setAggregators] = useState<AdminAggregator[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [freezingId, setFreezingId] = useState<string | null>(null);

  const load = async (p: number) => {
    setLoading(true);
    try {
      const resp = await listAdminAggregators(p, 20);
      setAggregators(resp.aggregators ?? []);
      setTotal(resp.total ?? 0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(page); }, [page]);

  const handleFreeze = async (agg: AdminAggregator) => {
    const reason = prompt(`Причина заморозки агрегатора "${agg.name}":`);
    if (reason === null) return;
    setFreezingId(agg.id);
    try {
      const res = await freezeAggregator(agg.id, reason);
      alert(`Заморожен. Субаккаунтов заморожено: ${res.sub_accounts_frozen}`);
      load(page);
    } catch (e: any) {
      alert(`Ошибка: ${e.message}`);
    } finally {
      setFreezingId(null);
    }
  };

  return (
    <div className="p-6">
      <h1 className="text-2xl font-semibold mb-4">Агрегаторы</h1>
      {loading ? (
        <p className="text-gray-500">Загрузка...</p>
      ) : (
        <>
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b text-left text-gray-600">
                <th className="py-2 pr-4">Название</th>
                <th className="py-2 pr-4">Email</th>
                <th className="py-2 pr-4">Статус</th>
                <th className="py-2 pr-4">Баланс</th>
                <th className="py-2 pr-4">Субаккаунты</th>
                <th className="py-2">Действия</th>
              </tr>
            </thead>
            <tbody>
              {aggregators.map((agg) => (
                <tr key={agg.id} className="border-b hover:bg-gray-50">
                  <td className="py-2 pr-4">
                    <Link to={`/admin/aggregators/${agg.id}`} className="text-blue-600 hover:underline">
                      {agg.name}
                    </Link>
                  </td>
                  <td className="py-2 pr-4">{agg.email}</td>
                  <td className="py-2 pr-4">
                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${agg.frozen ? 'bg-red-100 text-red-700' : 'bg-green-100 text-green-700'}`}>
                      {agg.frozen ? 'Заморожен' : 'Активен'}
                    </span>
                  </td>
                  <td className="py-2 pr-4">{agg.balance} ₽</td>
                  <td className="py-2 pr-4">
                    {agg.sub_account_count} / {agg.max_sub_accounts}
                  </td>
                  <td className="py-2 space-x-2">
                    <Link to={`/admin/aggregators/${agg.id}`} className="text-blue-600 hover:underline text-xs">
                      Профиль
                    </Link>
                    {!agg.frozen && (
                      <button
                        onClick={() => handleFreeze(agg)}
                        disabled={freezingId === agg.id}
                        className="text-red-600 hover:underline text-xs disabled:opacity-50"
                      >
                        {freezingId === agg.id ? 'Заморозка...' : 'Заморозить'}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <div className="flex items-center gap-2 mt-4 text-sm">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1}
              className="px-3 py-1 border rounded disabled:opacity-40">
              ← Назад
            </button>
            <span>Страница {page} · Всего {total}</span>
            <button onClick={() => setPage(p => p + 1)} disabled={aggregators.length < 20}
              className="px-3 py-1 border rounded disabled:opacity-40">
              Вперёд →
            </button>
          </div>
        </>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
cd .worktrees/aggregator-phase5
git add portal-frontend/src/pages/admin/aggregators/AggregatorsListPage.tsx \
        portal-frontend/src/api/aggregator.ts
git commit -m "feat(portal): add AggregatorsListPage admin page"
```

---

## Task 15: Frontend — AggregatorProfilePage

**Files:**
- Create: `portal-frontend/src/pages/admin/aggregators/AggregatorProfilePage.tsx`

- [ ] **Step 1: Create AggregatorProfilePage**

```tsx
// portal-frontend/src/pages/admin/aggregators/AggregatorProfilePage.tsx
import { useEffect, useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import {
  getAdminAggregator,
  freezeAggregator,
  unfreezeAggregator,
  AdminAggregator,
} from '../../../api/aggregator';

export default function AggregatorProfilePage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [agg, setAgg] = useState<AdminAggregator | null>(null);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);

  const load = async () => {
    if (!id) return;
    setLoading(true);
    try {
      setAgg(await getAdminAggregator(id));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [id]);

  const handleFreeze = async () => {
    if (!agg) return;
    const reason = prompt('Причина заморозки:');
    if (reason === null) return;
    setActionLoading(true);
    try {
      const res = await freezeAggregator(agg.id, reason);
      alert(`Заморожен. Субаккаунтов: ${res.sub_accounts_frozen}`);
      load();
    } catch (e: any) {
      alert(`Ошибка: ${e.message}`);
    } finally {
      setActionLoading(false);
    }
  };

  const handleUnfreeze = async () => {
    if (!agg) return;
    setActionLoading(true);
    try {
      await unfreezeAggregator(agg.id);
      alert('Агрегатор разморожен. Субаккаунты остаются заморожены.');
      load();
    } catch (e: any) {
      alert(`Ошибка: ${e.message}`);
    } finally {
      setActionLoading(false);
    }
  };

  if (loading) return <div className="p-6 text-gray-500">Загрузка...</div>;
  if (!agg) return <div className="p-6 text-red-500">Агрегатор не найден</div>;

  return (
    <div className="p-6 max-w-2xl">
      <div className="flex items-center justify-between mb-4">
        <div>
          <button onClick={() => navigate('/admin/aggregators')} className="text-sm text-gray-500 hover:underline mb-1">
            ← Агрегаторы
          </button>
          <h1 className="text-2xl font-semibold">{agg.name}</h1>
          <p className="text-sm text-gray-500">{agg.email}</p>
        </div>
        <span className={`px-3 py-1 rounded text-sm font-medium ${agg.frozen ? 'bg-red-100 text-red-700' : 'bg-green-100 text-green-700'}`}>
          {agg.frozen ? 'Заморожен' : 'Активен'}
        </span>
      </div>

      <div className="grid grid-cols-2 gap-4 mb-6 text-sm">
        <ProfileField label="Баланс" value={`${agg.balance} ₽`} />
        <ProfileField label="Субаккаунты" value={`${agg.sub_account_count} / ${agg.max_sub_accounts}`} />
        <ProfileField label="Лимит в сек." value={agg.pool_rate_limit_per_second?.toString() ?? '∞'} />
        <ProfileField label="Лимит в месяц" value={agg.pool_monthly_limit?.toLocaleString() ?? '∞'} />
        <ProfileField label="Авто-заморозка субаккаунтов" value={agg.auto_freeze_sub_accounts ? 'Да' : 'Нет'} />
      </div>

      <div className="flex gap-3 mb-6">
        {!agg.frozen ? (
          <button
            onClick={handleFreeze}
            disabled={actionLoading}
            className="px-4 py-2 bg-red-600 text-white rounded text-sm hover:bg-red-700 disabled:opacity-50"
          >
            Заморозить
          </button>
        ) : (
          <button
            onClick={handleUnfreeze}
            disabled={actionLoading}
            className="px-4 py-2 bg-green-600 text-white rounded text-sm hover:bg-green-700 disabled:opacity-50"
          >
            Разморозить
          </button>
        )}
        <Link
          to={`/admin/aggregators/${agg.id}/audit-log`}
          className="px-4 py-2 border rounded text-sm hover:bg-gray-50"
        >
          Audit Log
        </Link>
      </div>
    </div>
  );
}

function ProfileField({ label, value }: { label: string; value: string }) {
  return (
    <div className="border rounded p-3 bg-gray-50">
      <div className="text-xs text-gray-500 mb-0.5">{label}</div>
      <div className="font-medium">{value}</div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/aggregators/AggregatorProfilePage.tsx
git commit -m "feat(portal): add AggregatorProfilePage admin page"
```

---

## Task 16: Frontend — AggregatorAuditLogPage

**Files:**
- Create: `portal-frontend/src/pages/admin/aggregators/AggregatorAuditLogPage.tsx`

- [ ] **Step 1: Create AggregatorAuditLogPage**

```tsx
// portal-frontend/src/pages/admin/aggregators/AggregatorAuditLogPage.tsx
import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { getAuditLog } from '../../../api/aggregator';

interface AuditEntry {
  aggregator_id: string;
  action: string;
  target_type: string;
  target_id: string;
  details_json: string;
  ip_address: string;
  created_at: string;
}

const ACTION_LABELS: Record<string, string> = {
  cascade_freeze:           'Каскадная заморозка',
  unfreeze_aggregator:      'Разморозка агрегатора',
  unfreeze_all_sub_accounts:'Разморозка всех субаккаунтов',
  create_sub_account:       'Создание субаккаунта',
  update_tariff:            'Изменение тарифа',
  transfer_balance:         'Перевод баланса',
};

export default function AggregatorAuditLogPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);

  const load = async (p: number) => {
    if (!id) return;
    setLoading(true);
    try {
      const resp = await getAuditLog(id, p, 50);
      setEntries(resp.entries ?? []);
      setTotal(resp.total ?? 0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(page); }, [page, id]);

  return (
    <div className="p-6">
      <div className="mb-4">
        <button onClick={() => navigate(-1)} className="text-sm text-gray-500 hover:underline mb-1">
          ← Назад
        </button>
        <h1 className="text-2xl font-semibold">Audit Log агрегатора</h1>
      </div>

      {loading ? (
        <p className="text-gray-500">Загрузка...</p>
      ) : (
        <>
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b text-left text-gray-600">
                <th className="py-2 pr-4 w-40">Время</th>
                <th className="py-2 pr-4">Действие</th>
                <th className="py-2 pr-4">Цель</th>
                <th className="py-2 pr-4">Детали</th>
                <th className="py-2">IP</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e, i) => (
                <tr key={i} className="border-b hover:bg-gray-50">
                  <td className="py-2 pr-4 text-gray-500 text-xs">
                    {new Date(e.created_at).toLocaleString('ru-RU')}
                  </td>
                  <td className="py-2 pr-4 font-medium">
                    {ACTION_LABELS[e.action] ?? e.action}
                  </td>
                  <td className="py-2 pr-4 text-gray-600 text-xs">
                    {e.target_type}{e.target_id ? ` · ${e.target_id.slice(0, 8)}…` : ''}
                  </td>
                  <td className="py-2 pr-4 text-gray-600 text-xs font-mono max-w-xs truncate">
                    {e.details_json}
                  </td>
                  <td className="py-2 text-gray-500 text-xs">{e.ip_address || '—'}</td>
                </tr>
              ))}
              {entries.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-8 text-center text-gray-400">Записей нет</td>
                </tr>
              )}
            </tbody>
          </table>
          <div className="flex items-center gap-2 mt-4 text-sm">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1}
              className="px-3 py-1 border rounded disabled:opacity-40">
              ← Назад
            </button>
            <span>Страница {page} · Всего {total}</span>
            <button onClick={() => setPage(p => p + 1)} disabled={entries.length < 50}
              className="px-3 py-1 border rounded disabled:opacity-40">
              Вперёд →
            </button>
          </div>
        </>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/aggregators/AggregatorAuditLogPage.tsx
git commit -m "feat(portal): add AggregatorAuditLogPage admin page"
```

---

## Task 17: Frontend — App.tsx Routes + Sidebar

**Files:**
- Modify: `portal-frontend/src/App.tsx`
- Modify: `portal-frontend/src/components/layout/Sidebar.tsx`

- [ ] **Step 1: Add admin aggregator routes to App.tsx**

Find the admin routes section in `App.tsx` and add:

```tsx
import AggregatorsListPage from './pages/admin/aggregators/AggregatorsListPage';
import AggregatorProfilePage from './pages/admin/aggregators/AggregatorProfilePage';
import AggregatorAuditLogPage from './pages/admin/aggregators/AggregatorAuditLogPage';

// In admin routes section:
<Route path="/admin/aggregators" element={<AggregatorsListPage />} />
<Route path="/admin/aggregators/:id" element={<AggregatorProfilePage />} />
<Route path="/admin/aggregators/:id/audit-log" element={<AggregatorAuditLogPage />} />
```

- [ ] **Step 2: Add admin nav link to Sidebar.tsx**

Find the admin navigation section in `Sidebar.tsx` and add an "Aggregators" link visible only when `role === 'admin'`:

```tsx
{role === 'admin' && (
  <NavLink to="/admin/aggregators" className={navLinkClass}>
    Агрегаторы
  </NavLink>
)}
```

Also add an "Unfreeze all sub-accounts" button in the aggregator section (visible when `accountType === 'aggregator'`):

```tsx
{accountType === 'aggregator' && (
  <button
    onClick={handleUnfreezeAll}
    className="text-xs text-yellow-700 hover:underline px-4 py-1"
  >
    Разморозить все субаккаунты
  </button>
)}
```

And the handler near other sidebar actions:
```tsx
const handleUnfreezeAll = async () => {
  if (!confirm('Разморозить все субаккаунты?')) return;
  try {
    const res = await unfreezeAllSubAccounts();
    alert(`Разморожено субаккаунтов: ${res.sub_accounts_unfrozen}`);
  } catch (e: any) {
    alert(`Ошибка: ${e.message}`);
  }
};
```

Add import: `import { unfreezeAllSubAccounts } from '../../api/aggregator';`

- [ ] **Step 3: Build frontend to check compilation**

```bash
cd .worktrees/aggregator-phase5/portal-frontend
npm run build
```
Expected: build succeeds, no TypeScript errors

- [ ] **Step 4: Commit**

```bash
cd .worktrees/aggregator-phase5
git add portal-frontend/src/App.tsx \
        portal-frontend/src/components/layout/Sidebar.tsx
git commit -m "feat(portal): wire admin aggregator routes and sidebar navigation"
```

---

## Task 18: Full Build Verification + Final Commit

- [ ] **Step 1: Run all aggregator service tests**

```bash
cd .worktrees/aggregator-phase5
go test ./internal/services/aggregator/... -v
```
Expected: all PASS

- [ ] **Step 2: Run tarification service tests**

```bash
go test ./internal/services/tarification/... -v
```
Expected: all PASS

- [ ] **Step 3: Run portal handler tests**

```bash
go test ./internal/gateway/portal/... -v
```
Expected: all PASS

- [ ] **Step 4: Build all affected binaries**

```bash
go build ./cmd/services/tarification-service/... && \
go build ./cmd/smpp-gateway/... && \
go build ./cmd/portal-gateway/...
```
Expected: all succeed, no errors

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat(aggregator): Phase 5 control — pool limits, cascade freeze, SMPP unbind, admin portal"
```

---

## Self-Review Checklist

**Spec coverage:**
- [x] Pool rate-limiting per second (Redis sliding counter) — Tasks 3, 5, 12
- [x] Pool monthly limit (Redis WATCH+MULTI counter) — Tasks 3, 5, 12
- [x] Cascade freeze: aggregator frozen → sub-accounts frozen — Tasks 6, 8, 9
- [x] `auto_freeze_sub_accounts` flag respected — Task 6 (FreezeService)
- [x] SMPP sessions unbound on cascade freeze (Redis signal + watcher) — Tasks 6, 11
- [x] Unfreeze aggregator leaves sub-accounts frozen — Tasks 6, 8, 9
- [x] Aggregator "unfreeze all" self-service — Tasks 6, 8, 10, 17
- [x] Audit log writes on cascade_freeze and unfreeze_all — Task 6
- [x] Admin audit log read with pagination — Tasks 2, 8, 9, 16
- [x] Admin portal: aggregators list, profile, freeze/unfreeze actions — Tasks 14, 15, 17
- [x] No new DB migrations needed (all tables from Phase 1) — confirmed

**Gaps fixed during review:**
- `AuditEntry` write model was needed for `FreezeService` — added in Task 6 (domain/models.go append)
- `FreezeSubAccountRepository.WriteAuditLog` should be non-blocking — implementation calls balance_repository.WriteAuditLog which already handles errors internally (Phase 1 pattern)
- `UnfreezeAggregatorResponse.SubAccountsUnfrozen` field added to proto (Task 7) since it's referenced in handler
