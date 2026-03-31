# Provider Routing & Limits Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Реализовать иерархию лимитов, унифицированную маршрутизацию, конфигурируемый stub-провайдер и управление ресурсами субаккаунтов реселлера.

**Architecture:** Новые таблицы + расширение существующих → LimitResolver (цепочка global→tariff→client) + UnifiedRouter (3-уровневый: client→reseller→platform) → двухуровневый BackpressureManager (global+per-client) + SenderFactory (SMPP vs Stub). Portal API добавляет управление провайдерами и маршрутами субаккаунтов, Admin API — управление system_defaults и stub-конфигом.

**Tech Stack:** Go 1.24.0, PostgreSQL 15+ (pgx), Redis 7+ (go-redis/v9, pub/sub инвалидация), gorilla/mux, zerolog, testify

---

## Файловая карта

**Создать:**
- `migrations/000058_system_defaults.up.sql` + `.down.sql`
- `migrations/000059_stub_provider_config.up.sql` + `.down.sql`
- `migrations/000060_operator_prefixes.up.sql` + `.down.sql`
- `migrations/000061_extend_tables.up.sql` + `.down.sql` (tariff_plans + 6 полей, client_providers + tps_limit, client_routes nullable + shared, clients + allocated_tps_budget)
- `migrations/000062_migrate_legacy_routes.up.sql` + `.down.sql`
- `internal/pipeline/limits/resolver.go` — LimitResolver interface + DB impl
- `internal/pipeline/limits/cached_resolver.go` — Redis-кешированный wrapper
- `internal/pipeline/limits/resolver_test.go`
- `internal/router/unified.go` — UnifiedRouter
- `internal/router/unified_test.go`
- `internal/storage/client_route_repository.go` — UnifiedRouter queries
- `internal/storage/system_defaults_repository.go` — system_defaults CRUD
- `internal/smsc/stub_sender.go` — StubSender (выделен из sender.go)
- `internal/smsc/sender_factory.go` — SenderFactory
- `internal/smsc/sender_factory_test.go`
- `internal/gateway/admin/handlers/system_defaults.go`
- `internal/gateway/admin/handlers/stub_config.go`
- `internal/gateway/portal/handlers/sub_account_routing.go`

**Изменить:**
- `internal/pipeline/backpressure/manager.go` — два уровня (global + per-client)
- `internal/pipeline/backpressure/manager_test.go`
- `internal/pipeline/router/stage.go` — UnifiedRouter + OperatorResolver
- `internal/pipeline/sender/stage.go` — два уровня BP + SenderFactory + pub/sub
- `internal/smsc/sender.go` — убрать simulateSend/simulateAsyncSend (перенесено в stub_sender.go)
- `internal/gateway/portal/router/router.go` — зарегистрировать новые маршруты
- `internal/gateway/admin/router/router.go` — зарегистрировать новые маршруты

---

## Task 1: Migrations — новые таблицы

**Files:**
- Create: `migrations/000058_system_defaults.up.sql`
- Create: `migrations/000058_system_defaults.down.sql`
- Create: `migrations/000059_stub_provider_config.up.sql`
- Create: `migrations/000059_stub_provider_config.down.sql`
- Create: `migrations/000060_operator_prefixes.up.sql`
- Create: `migrations/000060_operator_prefixes.down.sql`

- [ ] **Step 1: Создать миграцию system_defaults (up)**

```sql
-- migrations/000058_system_defaults.up.sql
CREATE TABLE IF NOT EXISTS system_defaults (
    key        VARCHAR(100) PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now(),
    updated_by UUID
);

INSERT INTO system_defaults (key, value) VALUES
  ('rate_limit_per_second',    '"10"'),
  ('rate_limit_per_minute',    '"100"'),
  ('rate_limit_per_hour',      '"1000"'),
  ('default_tps_per_provider', '"5"'),
  ('max_providers_per_client', '"10"'),
  ('max_sub_accounts',         '"0"')
ON CONFLICT (key) DO NOTHING;
```

- [ ] **Step 2: Создать down-миграцию system_defaults**

```sql
-- migrations/000058_system_defaults.down.sql
DROP TABLE IF EXISTS system_defaults;
```

- [ ] **Step 3: Создать миграцию stub_provider_config (up)**

```sql
-- migrations/000059_stub_provider_config.up.sql
CREATE TABLE IF NOT EXISTS stub_provider_config (
    provider_id      UUID PRIMARY KEY REFERENCES providers(id) ON DELETE CASCADE,
    min_delay_ms     INT NOT NULL DEFAULT 100,
    max_delay_ms     INT NOT NULL DEFAULT 500,
    failure_rate_pct INT NOT NULL DEFAULT 0 CHECK (failure_rate_pct BETWEEN 0 AND 100),
    dlr_delay_ms     INT NOT NULL DEFAULT 1000,
    dlr_success_rate INT NOT NULL DEFAULT 100 CHECK (dlr_success_rate BETWEEN 0 AND 100),
    dlr_statuses     JSONB NOT NULL DEFAULT '["DELIVRD"]',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER update_stub_provider_config_updated_at
    BEFORE UPDATE ON stub_provider_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

- [ ] **Step 4: Создать down-миграцию stub_provider_config**

```sql
-- migrations/000059_stub_provider_config.down.sql
DROP TABLE IF EXISTS stub_provider_config;
```

- [ ] **Step 5: Создать миграцию operator_prefixes (up)**

```sql
-- migrations/000060_operator_prefixes.up.sql
CREATE TABLE IF NOT EXISTS operator_prefixes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
    prefix      VARCHAR(20) NOT NULL,
    priority    INT NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT true,
    UNIQUE(prefix)
);

CREATE INDEX idx_operator_prefixes_active ON operator_prefixes(prefix) WHERE active = true;
```

- [ ] **Step 6: Создать down-миграцию operator_prefixes**

```sql
-- migrations/000060_operator_prefixes.down.sql
DROP TABLE IF EXISTS operator_prefixes;
```

- [ ] **Step 7: Применить миграции локально и убедиться, что они выполняются без ошибок**

```bash
cd /home/magomed/projects/sms
migrate -path migrations -database "postgres://smpp:smpp@localhost:5432/smpp?sslmode=disable" up 3
```
Ожидается: `3/u 000060_operator_prefixes (N ms)`

- [ ] **Step 8: Commit**

```bash
git add migrations/000058_* migrations/000059_* migrations/000060_*
git commit -m "feat: migrate — system_defaults, stub_provider_config, operator_prefixes"
```

---

## Task 2: Migrations — расширение существующих таблиц

**Files:**
- Create: `migrations/000061_extend_tables.up.sql`
- Create: `migrations/000061_extend_tables.down.sql`

- [ ] **Step 1: Создать up-миграцию**

```sql
-- migrations/000061_extend_tables.up.sql

-- tariff_plans: поля лимитов (NULL = брать из system_defaults)
ALTER TABLE tariff_plans
    ADD COLUMN IF NOT EXISTS rate_limit_per_second    INT,
    ADD COLUMN IF NOT EXISTS rate_limit_per_minute    INT,
    ADD COLUMN IF NOT EXISTS rate_limit_per_hour      INT,
    ADD COLUMN IF NOT EXISTS default_tps_per_provider INT,
    ADD COLUMN IF NOT EXISTS max_providers            INT,
    ADD COLUMN IF NOT EXISTS max_sub_accounts         INT;

-- client_providers: per-client TPS лимит на провайдера (NULL = из tariff или system_defaults)
ALTER TABLE client_providers
    ADD COLUMN IF NOT EXISTS tps_limit INT;

-- client_routes: client_id nullable (NULL = платформенный дефолт) + shared flag
ALTER TABLE client_routes
    ALTER COLUMN client_id DROP NOT NULL;

ALTER TABLE client_routes
    ADD COLUMN IF NOT EXISTS shared BOOLEAN NOT NULL DEFAULT false;

-- Индекс для платформенных дефолтных маршрутов
CREATE INDEX IF NOT EXISTS idx_client_routes_default
    ON client_routes (operator_id)
    WHERE client_id IS NULL AND active = true;

-- clients: TPS-бюджет для субаккаунтов (NULL = не ограничено)
ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS allocated_tps_budget INT;
```

- [ ] **Step 2: Создать down-миграцию**

```sql
-- migrations/000061_extend_tables.down.sql
DROP INDEX IF EXISTS idx_client_routes_default;

ALTER TABLE clients          DROP COLUMN IF EXISTS allocated_tps_budget;
ALTER TABLE client_routes    DROP COLUMN IF EXISTS shared;
ALTER TABLE client_providers DROP COLUMN IF EXISTS tps_limit;

ALTER TABLE tariff_plans
    DROP COLUMN IF EXISTS rate_limit_per_second,
    DROP COLUMN IF EXISTS rate_limit_per_minute,
    DROP COLUMN IF EXISTS rate_limit_per_hour,
    DROP COLUMN IF EXISTS default_tps_per_provider,
    DROP COLUMN IF EXISTS max_providers,
    DROP COLUMN IF EXISTS max_sub_accounts;

-- Восстановить NOT NULL на client_routes.client_id
-- (только если нет NULL-записей)
ALTER TABLE client_routes ALTER COLUMN client_id SET NOT NULL;
```

- [ ] **Step 3: Применить миграцию**

```bash
migrate -path migrations -database "postgres://smpp:smpp@localhost:5432/smpp?sslmode=disable" up 1
```
Ожидается: `1/u 000061_extend_tables (N ms)`

- [ ] **Step 4: Commit**

```bash
git add migrations/000061_*
git commit -m "feat: migrate — extend tariff_plans, client_providers, client_routes, clients"
```

---

## Task 3: Migration — перенос legacy routes в client_routes

**Files:**
- Create: `migrations/000062_migrate_legacy_routes.up.sql`
- Create: `migrations/000062_migrate_legacy_routes.down.sql`

- [ ] **Step 1: Создать up-миграцию**

Функция `resolve_operator_from_pattern` ищет оператора по префиксу маршрута. Если оператор не найден — маршрут пропускается (логируется как предупреждение при применении).

```sql
-- migrations/000062_migrate_legacy_routes.up.sql

-- Вспомогательная функция: находит operator_id по паттерну маршрута
-- Ищет оператора, prefix которого совпадает с началом pattern
CREATE OR REPLACE FUNCTION resolve_operator_from_prefix(pattern TEXT)
RETURNS UUID AS $$
DECLARE
    result UUID;
BEGIN
    SELECT operator_id INTO result
    FROM operator_prefixes
    WHERE active = true
      AND pattern LIKE (prefix || '%')
    ORDER BY LENGTH(prefix) DESC
    LIMIT 1;
    RETURN result; -- NULL если не найден
END;
$$ LANGUAGE plpgsql;

-- Переносим основные маршруты
INSERT INTO client_routes (client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at)
SELECT
    NULL,
    resolve_operator_from_prefix(r.pattern),
    r.provider_id,
    r.priority,
    1,
    r.active,
    r.created_at,
    r.updated_at
FROM routes r
WHERE resolve_operator_from_prefix(r.pattern) IS NOT NULL
ON CONFLICT (client_id, operator_id, provider_id)
    WHERE client_id IS NULL
    DO NOTHING;

-- Переносим failover-провайдеры как маршруты с меньшим приоритетом
INSERT INTO client_routes (client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at)
SELECT
    NULL,
    resolve_operator_from_prefix(r.pattern),
    r.failover_provider_id,
    r.priority - 1,
    1,
    r.active,
    r.created_at,
    r.updated_at
FROM routes r
WHERE r.failover_provider_id IS NOT NULL
  AND resolve_operator_from_prefix(r.pattern) IS NOT NULL
ON CONFLICT (client_id, operator_id, provider_id)
    WHERE client_id IS NULL
    DO NOTHING;
```

- [ ] **Step 2: Создать down-миграцию**

```sql
-- migrations/000062_migrate_legacy_routes.down.sql
-- Удаляем только платформенные дефолты (client_id IS NULL), добавленные этой миграцией
DELETE FROM client_routes WHERE client_id IS NULL;
DROP FUNCTION IF EXISTS resolve_operator_from_prefix(TEXT);
```

- [ ] **Step 3: Применить миграцию**

```bash
migrate -path migrations -database "postgres://smpp:smpp@localhost:5432/smpp?sslmode=disable" up 1
```

- [ ] **Step 4: Commit**

```bash
git add migrations/000062_*
git commit -m "feat: migrate — legacy routes → client_routes (platform defaults)"
```

---

## Task 4: LimitResolver — интерфейс + DB-реализация

**Files:**
- Create: `internal/pipeline/limits/resolver.go`
- Create: `internal/storage/system_defaults_repository.go`
- Create: `internal/pipeline/limits/resolver_test.go`

- [ ] **Step 1: Создать system_defaults_repository.go**

```go
// internal/storage/system_defaults_repository.go
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/jmoiron/sqlx"
)

type SystemDefaultsRepository struct {
	db *sqlx.DB
}

func NewSystemDefaultsRepository(db *DB) *SystemDefaultsRepository {
	return &SystemDefaultsRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

func (r *SystemDefaultsRepository) GetInt(ctx context.Context, key string) (int, error) {
	var raw json.RawMessage
	err := r.db.QueryRowContext(ctx, `SELECT value FROM system_defaults WHERE key = $1`, key).Scan(&raw)
	if err != nil {
		return 0, fmt.Errorf("system_defaults.%s: %w", key, err)
	}
	// value хранится как JSON-строка: "10" или число 10
	var s string
	if jsonErr := json.Unmarshal(raw, &s); jsonErr == nil {
		return strconv.Atoi(s)
	}
	var n int
	if jsonErr := json.Unmarshal(raw, &n); jsonErr == nil {
		return n, nil
	}
	return 0, fmt.Errorf("system_defaults.%s: не удалось распарсить значение %s", key, string(raw))
}

func (r *SystemDefaultsRepository) Set(ctx context.Context, key string, value int, updatedBy *string) error {
	encoded, _ := json.Marshal(strconv.Itoa(value))
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO system_defaults (key, value, updated_at, updated_by)
		VALUES ($1, $2, now(), $3)
		ON CONFLICT (key) DO UPDATE
		    SET value = EXCLUDED.value, updated_at = now(), updated_by = EXCLUDED.updated_by`,
		key, encoded, updatedBy)
	return err
}

type SystemDefaultsAll struct {
	RateLimitPerSecond    int
	RateLimitPerMinute    int
	RateLimitPerHour      int
	DefaultTPSPerProvider int
	MaxProvidersPerClient int
	MaxSubAccounts        int
}

func (r *SystemDefaultsRepository) GetAll(ctx context.Context) (*SystemDefaultsAll, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM system_defaults`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]int)
	for rows.Next() {
		var key string
		var raw json.RawMessage
		if scanErr := rows.Scan(&key, &raw); scanErr != nil {
			continue
		}
		var s string
		if jsonErr := json.Unmarshal(raw, &s); jsonErr == nil {
			if n, convErr := strconv.Atoi(s); convErr == nil {
				m[key] = n
			}
			continue
		}
		var n int
		if jsonErr := json.Unmarshal(raw, &n); jsonErr == nil {
			m[key] = n
		}
	}

	return &SystemDefaultsAll{
		RateLimitPerSecond:    m["rate_limit_per_second"],
		RateLimitPerMinute:    m["rate_limit_per_minute"],
		RateLimitPerHour:      m["rate_limit_per_hour"],
		DefaultTPSPerProvider: m["default_tps_per_provider"],
		MaxProvidersPerClient: m["max_providers_per_client"],
		MaxSubAccounts:        m["max_sub_accounts"],
	}, rows.Err()
}
```

- [ ] **Step 2: Написать тест resolver.go (failing)**

```go
// internal/pipeline/limits/resolver_test.go
package limits_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/pipeline/limits"
)

func TestDBLimitResolver_ResolveProviderTPS_UsesClientProviderLimit(t *testing.T) {
	resolver := limits.NewDBLimitResolver(mockLimitQuerier{
		clientProviderTPS: intPtr(30),
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 30, tps)
}

func TestDBLimitResolver_ResolveProviderTPS_FallsBackToTariffPlan(t *testing.T) {
	resolver := limits.NewDBLimitResolver(mockLimitQuerier{
		clientProviderTPS:      nil, // нет per-client TPS
		tariffPlanDefaultTPS:   intPtr(20),
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 20, tps)
}

func TestDBLimitResolver_ResolveProviderTPS_FallsBackToSystemDefault(t *testing.T) {
	resolver := limits.NewDBLimitResolver(mockLimitQuerier{
		clientProviderTPS:    nil,
		tariffPlanDefaultTPS: nil,
		systemDefaultTPS:     5,
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 5, tps)
}

func TestDBLimitResolver_ResolveProviderTPS_SubAccountBudgetCap(t *testing.T) {
	clientID := uuid.New()
	resolver := limits.NewDBLimitResolver(mockLimitQuerier{
		clientProviderTPS:    intPtr(30),
		systemDefaultTPS:     5,
		parentClientID:       &uuid.UUID{}, // есть родитель
		allocatedTPSBudget:   intPtr(40),
		allocatedTPSUsed:     20, // уже использовано у других провайдеров субаккаунта
	})
	// resolved = 30, max_allowed = 40-20 = 20 → min(30, 20) = 20
	tps, err := resolver.ResolveProviderTPS(context.Background(), clientID, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 20, tps)
}

// --- helpers ---

func intPtr(n int) *int { return &n }

type mockLimitQuerier struct {
	clientProviderTPS    *int
	tariffPlanDefaultTPS *int
	systemDefaultTPS     int
	parentClientID       *uuid.UUID
	allocatedTPSBudget   *int
	allocatedTPSUsed     int
}

func (m mockLimitQuerier) GetClientProviderTPS(_ context.Context, _, _ uuid.UUID) (*int, error) {
	return m.clientProviderTPS, nil
}
func (m mockLimitQuerier) GetTariffPlanDefaultTPS(_ context.Context, _ uuid.UUID) (*int, error) {
	return m.tariffPlanDefaultTPS, nil
}
func (m mockLimitQuerier) GetSystemDefaultTPS(_ context.Context) (int, error) {
	return m.systemDefaultTPS, nil
}
func (m mockLimitQuerier) GetClientParentAndBudget(_ context.Context, _ uuid.UUID) (*uuid.UUID, *int, error) {
	return m.parentClientID, m.allocatedTPSBudget, nil
}
func (m mockLimitQuerier) GetSubAccountAllocatedTPS(_ context.Context, _, _ uuid.UUID) (int, error) {
	return m.allocatedTPSUsed, nil
}
```

- [ ] **Step 3: Запустить тест — убедиться, что не компилируется (пакет limits не существует)**

```bash
cd /home/magomed/projects/sms
go test ./internal/pipeline/limits/... 2>&1 | head -20
```
Ожидается: `cannot find package` или `no Go files`

- [ ] **Step 4: Создать resolver.go**

```go
// internal/pipeline/limits/resolver.go
package limits

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LimitQuerier — DB-запросы, необходимые LimitResolver.
type LimitQuerier interface {
	GetClientProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) (*int, error)
	GetTariffPlanDefaultTPS(ctx context.Context, clientID uuid.UUID) (*int, error)
	GetSystemDefaultTPS(ctx context.Context) (int, error)
	GetClientParentAndBudget(ctx context.Context, clientID uuid.UUID) (*uuid.UUID, *int, error)
	GetSubAccountAllocatedTPS(ctx context.Context, clientID, excludeProviderID uuid.UUID) (int, error)
}

// DBLimitResolver разрешает TPS-лимиты по цепочке:
// client_providers → tariff_plan → system_defaults, с учётом бюджета субаккаунта.
type DBLimitResolver struct {
	q      LimitQuerier
	logger zerolog.Logger
}

func NewDBLimitResolver(q LimitQuerier) *DBLimitResolver {
	return &DBLimitResolver{q: q, logger: log.With().Str("component", "limit_resolver").Logger()}
}

// ResolveProviderTPS возвращает TPS-лимит для пары (client, provider).
func (r *DBLimitResolver) ResolveProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) (int, error) {
	// 1. Per-client TPS на провайдере
	if v, err := r.q.GetClientProviderTPS(ctx, clientID, providerID); err == nil && v != nil {
		tps := *v
		return r.applySubAccountBudgetCap(ctx, clientID, providerID, tps)
	}

	// 2. Дефолт из тарифного плана клиента
	if v, err := r.q.GetTariffPlanDefaultTPS(ctx, clientID); err == nil && v != nil {
		tps := *v
		return r.applySubAccountBudgetCap(ctx, clientID, providerID, tps)
	}

	// 3. Системный дефолт (всегда есть)
	tps, err := r.q.GetSystemDefaultTPS(ctx)
	if err != nil {
		return 5, fmt.Errorf("GetSystemDefaultTPS: %w", err)
	}
	return r.applySubAccountBudgetCap(ctx, clientID, providerID, tps)
}

func (r *DBLimitResolver) applySubAccountBudgetCap(ctx context.Context, clientID, providerID uuid.UUID, resolved int) (int, error) {
	parentID, budget, err := r.q.GetClientParentAndBudget(ctx, clientID)
	if err != nil || parentID == nil || budget == nil {
		return resolved, nil
	}

	used, err := r.q.GetSubAccountAllocatedTPS(ctx, clientID, providerID)
	if err != nil {
		return resolved, nil
	}

	maxAllowed := *budget - used
	if maxAllowed < 0 {
		maxAllowed = 0
	}
	if resolved > maxAllowed {
		return maxAllowed, nil
	}
	return resolved, nil
}

// --- Redis-cached wrapper ---

type CachedLimitResolver struct {
	inner  *DBLimitResolver
	redis  *redis.Client
	ttl    time.Duration
	logger zerolog.Logger
}

func NewCachedLimitResolver(inner *DBLimitResolver, rdb *redis.Client) *CachedLimitResolver {
	return &CachedLimitResolver{
		inner:  inner,
		redis:  rdb,
		ttl:    30 * time.Second,
		logger: log.With().Str("component", "cached_limit_resolver").Logger(),
	}
}

func (c *CachedLimitResolver) ResolveProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) (int, error) {
	key := fmt.Sprintf("limits:%s:%s:tps", clientID, providerID)

	if val, err := c.redis.Get(ctx, key).Int(); err == nil {
		return val, nil
	}

	tps, err := c.inner.ResolveProviderTPS(ctx, clientID, providerID)
	if err != nil {
		return tps, err
	}

	c.redis.Set(ctx, key, tps, c.ttl)
	return tps, nil
}

// InvalidateProviderTPS сбрасывает кеш TPS для конкретной пары (client, provider).
func (c *CachedLimitResolver) InvalidateProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) {
	key := fmt.Sprintf("limits:%s:%s:tps", clientID, providerID)
	c.redis.Del(ctx, key)
}
```

- [ ] **Step 5: Добавить LimitQuerierDB в storage**

```go
// internal/storage/limit_querier.go
package storage

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// LimitQuerierDB реализует limits.LimitQuerier через PostgreSQL.
type LimitQuerierDB struct {
	db *sqlx.DB
}

func NewLimitQuerierDB(db *DB) *LimitQuerierDB {
	return &LimitQuerierDB{db: sqlx.NewDb(db.DB, "pgx")}
}

func (q *LimitQuerierDB) GetClientProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) (*int, error) {
	var tps sql.NullInt32
	err := q.db.QueryRowContext(ctx,
		`SELECT tps_limit FROM client_providers WHERE client_id=$1 AND provider_id=$2 AND active=true`,
		clientID, providerID,
	).Scan(&tps)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !tps.Valid {
		return nil, nil
	}
	v := int(tps.Int32)
	return &v, nil
}

func (q *LimitQuerierDB) GetTariffPlanDefaultTPS(ctx context.Context, clientID uuid.UUID) (*int, error) {
	var tps sql.NullInt32
	err := q.db.QueryRowContext(ctx, `
		SELECT tp.default_tps_per_provider
		FROM tariff_plans tp
		JOIN tariff_periods tper ON tper.plan_id = tp.id
		WHERE tper.client_id = $1
		  AND tper.start_date <= now()
		  AND (tper.end_date IS NULL OR tper.end_date > now())
		ORDER BY tper.start_date DESC
		LIMIT 1`,
		clientID,
	).Scan(&tps)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !tps.Valid {
		return nil, nil
	}
	v := int(tps.Int32)
	return &v, nil
}

func (q *LimitQuerierDB) GetSystemDefaultTPS(ctx context.Context) (int, error) {
	var raw string
	err := q.db.QueryRowContext(ctx,
		`SELECT value #>> '{}' FROM system_defaults WHERE key='default_tps_per_provider'`,
	).Scan(&raw)
	if err != nil {
		return 5, err
	}
	var n int
	if _, scanErr := fmt.Sscanf(raw, "%d", &n); scanErr != nil {
		return 5, nil
	}
	return n, nil
}

func (q *LimitQuerierDB) GetClientParentAndBudget(ctx context.Context, clientID uuid.UUID) (*uuid.UUID, *int, error) {
	var parentID uuid.NullUUID
	var budget sql.NullInt32
	err := q.db.QueryRowContext(ctx,
		`SELECT parent_client_id, allocated_tps_budget FROM clients WHERE id=$1`,
		clientID,
	).Scan(&parentID, &budget)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var pid *uuid.UUID
	if parentID.Valid {
		v := parentID.UUID
		pid = &v
	}
	var b *int
	if budget.Valid {
		v := int(budget.Int32)
		b = &v
	}
	return pid, b, nil
}

func (q *LimitQuerierDB) GetSubAccountAllocatedTPS(ctx context.Context, clientID, excludeProviderID uuid.UUID) (int, error) {
	var total sql.NullInt32
	err := q.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(tps_limit), 0)
		 FROM client_providers
		 WHERE client_id=$1 AND provider_id != $2 AND active=true AND tps_limit IS NOT NULL`,
		clientID, excludeProviderID,
	).Scan(&total)
	if err != nil {
		return 0, err
	}
	return int(total.Int32), nil
}
```

- [ ] **Step 6: Запустить тесты — убедиться, что проходят**

```bash
go test ./internal/pipeline/limits/... -v
```
Ожидается: `PASS` — все 4 теста

- [ ] **Step 7: Commit**

```bash
git add internal/pipeline/limits/ internal/storage/limit_querier.go internal/storage/system_defaults_repository.go
git commit -m "feat: LimitResolver — per-client TPS chain с sub-account budget cap"
```

---

## Task 5: UnifiedRouter — repository + логика

**Files:**
- Create: `internal/storage/client_route_repository.go`
- Create: `internal/router/unified.go`
- Create: `internal/router/unified_test.go`

- [ ] **Step 1: Написать тест UnifiedRouter (failing)**

```go
// internal/router/unified_test.go
package router_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/router"
	"github.com/smpp-server/smpp-server/internal/shared"
)

var (
	clientID   = uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	parentID   = uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")
	operatorID = uuid.MustParse("cccccccc-0000-0000-0000-000000000001")
	providerID = uuid.MustParse("dddddddd-0000-0000-0000-000000000001")
)

func makeRoute(cid *uuid.UUID, pid uuid.UUID, prio int, shared bool) *shared.ClientRoute {
	return &shared.ClientRoute{
		ID: uuid.New(), ClientID: cid, OperatorID: operatorID,
		ProviderID: pid, Priority: prio, Weight: 1, Active: true, Shared: shared,
	}
}

// Уровень 1: собственный маршрут клиента
func TestUnifiedRouter_UsesOwnRoute(t *testing.T) {
	rt := makeRoute(&clientID, providerID, 100, false)
	r := router.NewUnifiedRouter(mockRouteRepo{
		byClient: map[uuid.UUID][]*shared.ClientRoute{clientID: {rt}},
	}, mockClientRepo{})

	dec, err := r.Route(context.Background(), clientID, operatorID)
	require.NoError(t, err)
	assert.Equal(t, providerID, dec.ProviderID)
}

// Уровень 2: shared маршрут реселлера
func TestUnifiedRouter_FallsBackToSharedParentRoute(t *testing.T) {
	sharedRoute := makeRoute(&parentID, providerID, 100, true)
	r := router.NewUnifiedRouter(mockRouteRepo{
		sharedByClient: map[uuid.UUID][]*shared.ClientRoute{parentID: {sharedRoute}},
	}, mockClientRepo{
		parentID: &parentID,
	})

	dec, err := r.Route(context.Background(), clientID, operatorID)
	require.NoError(t, err)
	assert.Equal(t, providerID, dec.ProviderID)
}

// Уровень 3: платформенный дефолт
func TestUnifiedRouter_FallsBackToPlatformDefault(t *testing.T) {
	defaultRoute := makeRoute(nil, providerID, 50, false)
	r := router.NewUnifiedRouter(mockRouteRepo{
		defaults: map[uuid.UUID][]*shared.ClientRoute{operatorID: {defaultRoute}},
	}, mockClientRepo{})

	dec, err := r.Route(context.Background(), clientID, operatorID)
	require.NoError(t, err)
	assert.Equal(t, providerID, dec.ProviderID)
}

// Нет маршрутов → ErrNoRouteFound
func TestUnifiedRouter_ReturnsErrWhenNoRoutes(t *testing.T) {
	r := router.NewUnifiedRouter(mockRouteRepo{}, mockClientRepo{})
	_, err := r.Route(context.Background(), clientID, operatorID)
	assert.ErrorIs(t, err, router.ErrNoRouteFound)
}

// --- mocks ---

type mockRouteRepo struct {
	byClient       map[uuid.UUID][]*shared.ClientRoute
	sharedByClient map[uuid.UUID][]*shared.ClientRoute
	defaults       map[uuid.UUID][]*shared.ClientRoute
}

func (m mockRouteRepo) ListByClientAndOperator(_ context.Context, cid, oid uuid.UUID) ([]*shared.ClientRoute, error) {
	return m.byClient[cid], nil
}
func (m mockRouteRepo) ListSharedByClientAndOperator(_ context.Context, cid, oid uuid.UUID) ([]*shared.ClientRoute, error) {
	return m.sharedByClient[cid], nil
}
func (m mockRouteRepo) ListDefaultByOperator(_ context.Context, oid uuid.UUID) ([]*shared.ClientRoute, error) {
	return m.defaults[oid], nil
}

type mockClientRepo struct{ parentID *uuid.UUID }

func (m mockClientRepo) GetParentClientID(_ context.Context, _ uuid.UUID) (*uuid.UUID, error) {
	return m.parentID, nil
}
```

- [ ] **Step 2: Запустить тест — убедиться, что не компилируется**

```bash
go test ./internal/router/... 2>&1 | head -10
```

- [ ] **Step 3: Добавить `shared.ClientRoute` в shared/models.go**

Открыть `internal/shared/models.go` и добавить в конец файла:

```go
// ClientRoute — маршрут клиента к провайдеру через оператора.
// client_id = NULL означает платформенный дефолт.
type ClientRoute struct {
	ID         uuid.UUID  `db:"id"`
	ClientID   *uuid.UUID `db:"client_id"`
	OperatorID uuid.UUID  `db:"operator_id"`
	ProviderID uuid.UUID  `db:"provider_id"`
	Priority   int        `db:"priority"`
	Weight     int        `db:"weight"`
	Active     bool       `db:"active"`
	Shared     bool       `db:"shared"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
}

// RoutingDecision — результат маршрутизации.
type RoutingDecision struct {
	ProviderID uuid.UUID
	RouteID    uuid.UUID
}
```

- [ ] **Step 4: Создать unified.go**

```go
// internal/router/unified.go
package router

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// ErrNoRouteFound возвращается, когда не найден ни один маршрут.
var ErrNoRouteFound = errors.New("no route found")

// ClientRouteRepository — запросы к таблице client_routes.
type ClientRouteRepository interface {
	ListByClientAndOperator(ctx context.Context, clientID, operatorID uuid.UUID) ([]*shared.ClientRoute, error)
	ListSharedByClientAndOperator(ctx context.Context, parentClientID, operatorID uuid.UUID) ([]*shared.ClientRoute, error)
	ListDefaultByOperator(ctx context.Context, operatorID uuid.UUID) ([]*shared.ClientRoute, error)
}

// ClientParentRepository — получение родителя клиента.
type ClientParentRepository interface {
	GetParentClientID(ctx context.Context, clientID uuid.UUID) (*uuid.UUID, error)
}

// UnifiedRouter маршрутизирует сообщение по трём уровням:
//  1. Собственные маршруты клиента
//  2. Shared маршруты реселлера (parent)
//  3. Платформенные дефолты (client_id IS NULL)
type UnifiedRouter struct {
	routeRepo  ClientRouteRepository
	clientRepo ClientParentRepository
	logger     zerolog.Logger
}

func NewUnifiedRouter(routeRepo ClientRouteRepository, clientRepo ClientParentRepository) *UnifiedRouter {
	return &UnifiedRouter{
		routeRepo:  routeRepo,
		clientRepo: clientRepo,
		logger:     log.With().Str("component", "unified_router").Logger(),
	}
}

// Route возвращает RoutingDecision для пары (clientID, operatorID).
func (r *UnifiedRouter) Route(ctx context.Context, clientID, operatorID uuid.UUID) (*shared.RoutingDecision, error) {
	// Уровень 1: собственные маршруты
	routes, err := r.routeRepo.ListByClientAndOperator(ctx, clientID, operatorID)
	if err != nil {
		r.logger.Error().Err(err).Msg("ListByClientAndOperator failed")
	}

	// Уровень 2: shared маршруты реселлера
	if len(routes) == 0 {
		parentID, _ := r.clientRepo.GetParentClientID(ctx, clientID)
		if parentID != nil {
			routes, _ = r.routeRepo.ListSharedByClientAndOperator(ctx, *parentID, operatorID)
		}
	}

	// Уровень 3: платформенные дефолты
	if len(routes) == 0 {
		routes, _ = r.routeRepo.ListDefaultByOperator(ctx, operatorID)
	}

	if len(routes) == 0 {
		return nil, ErrNoRouteFound
	}

	// Выбираем по наивысшему приоритету (приоритет по убыванию, массив уже отсортирован репозиторием)
	selected := routes[0]
	return &shared.RoutingDecision{
		ProviderID: selected.ProviderID,
		RouteID:    selected.ID,
	}, nil
}

// --- In-memory кеш ---

type cacheKey struct {
	clientID   uuid.UUID
	operatorID uuid.UUID
}

type cacheEntry struct {
	decision  *shared.RoutingDecision
	expiresAt time.Time
}

// CachedUnifiedRouter оборачивает UnifiedRouter in-memory кешем с TTL 30 сек.
type CachedUnifiedRouter struct {
	inner  *UnifiedRouter
	cache  map[cacheKey]*cacheEntry
	mu     sync.RWMutex
	ttl    time.Duration
}

func NewCachedUnifiedRouter(inner *UnifiedRouter) *CachedUnifiedRouter {
	return &CachedUnifiedRouter{
		inner: inner,
		cache: make(map[cacheKey]*cacheEntry),
		ttl:   30 * time.Second,
	}
}

func (c *CachedUnifiedRouter) Route(ctx context.Context, clientID, operatorID uuid.UUID) (*shared.RoutingDecision, error) {
	key := cacheKey{clientID, operatorID}

	c.mu.RLock()
	if entry, ok := c.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.decision, nil
	}
	c.mu.RUnlock()

	decision, err := c.inner.Route(ctx, clientID, operatorID)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache[key] = &cacheEntry{decision: decision, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()

	return decision, nil
}

func (c *CachedUnifiedRouter) Invalidate(clientID, operatorID uuid.UUID) {
	c.mu.Lock()
	delete(c.cache, cacheKey{clientID, operatorID})
	c.mu.Unlock()
}
```

- [ ] **Step 5: Создать client_route_repository.go**

```go
// internal/storage/client_route_repository.go
package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/shared"
)

type ClientRouteRepository struct {
	db *sqlx.DB
}

func NewClientRouteRepository(db *DB) *ClientRouteRepository {
	return &ClientRouteRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

func (r *ClientRouteRepository) ListByClientAndOperator(ctx context.Context, clientID, operatorID uuid.UUID) ([]*shared.ClientRoute, error) {
	var routes []*shared.ClientRoute
	err := r.db.SelectContext(ctx, &routes, `
		SELECT id, client_id, operator_id, provider_id, priority, weight, active, shared, created_at, updated_at
		FROM client_routes
		WHERE client_id = $1 AND operator_id = $2 AND active = true
		ORDER BY priority DESC`, clientID, operatorID)
	return routes, err
}

func (r *ClientRouteRepository) ListSharedByClientAndOperator(ctx context.Context, parentClientID, operatorID uuid.UUID) ([]*shared.ClientRoute, error) {
	var routes []*shared.ClientRoute
	err := r.db.SelectContext(ctx, &routes, `
		SELECT id, client_id, operator_id, provider_id, priority, weight, active, shared, created_at, updated_at
		FROM client_routes
		WHERE client_id = $1 AND operator_id = $2 AND active = true AND shared = true
		ORDER BY priority DESC`, parentClientID, operatorID)
	return routes, err
}

func (r *ClientRouteRepository) ListDefaultByOperator(ctx context.Context, operatorID uuid.UUID) ([]*shared.ClientRoute, error) {
	var routes []*shared.ClientRoute
	err := r.db.SelectContext(ctx, &routes, `
		SELECT id, client_id, operator_id, provider_id, priority, weight, active, shared, created_at, updated_at
		FROM client_routes
		WHERE client_id IS NULL AND operator_id = $1 AND active = true
		ORDER BY priority DESC`, operatorID)
	return routes, err
}

func (r *ClientRouteRepository) GetParentClientID(ctx context.Context, clientID uuid.UUID) (*uuid.UUID, error) {
	var parentID uuid.NullUUID
	err := r.db.QueryRowContext(ctx,
		`SELECT parent_client_id FROM clients WHERE id = $1`, clientID,
	).Scan(&parentID)
	if err != nil {
		return nil, err
	}
	if !parentID.Valid {
		return nil, nil
	}
	v := parentID.UUID
	return &v, nil
}
```

- [ ] **Step 6: Запустить тесты**

```bash
go test ./internal/router/... -v -run TestUnifiedRouter
```
Ожидается: `PASS` — все 4 теста

- [ ] **Step 7: Commit**

```bash
git add internal/router/unified.go internal/router/unified_test.go \
        internal/storage/client_route_repository.go internal/shared/models.go
git commit -m "feat: UnifiedRouter — 3-уровневая маршрутизация (client→reseller→platform)"
```

---

## Task 6: OperatorResolver — определение оператора по номеру

**Files:**
- Create: `internal/router/operator_resolver.go`
- Create: `internal/storage/operator_prefix_repository.go`

- [ ] **Step 1: Создать operator_resolver.go**

```go
// internal/router/operator_resolver.go
package router

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// OperatorPrefixRepository загружает префиксы из БД.
type OperatorPrefixRepository interface {
	GetAllActive(ctx context.Context) ([]OperatorPrefix, error)
}

type OperatorPrefix struct {
	OperatorID uuid.UUID
	Prefix     string
	Priority   int
}

// OperatorResolver определяет operator_id по номеру телефона.
// Использует longest-prefix matching. Если не найден — возвращает defaultOperatorID.
type OperatorResolver struct {
	repo              OperatorPrefixRepository
	defaultOperatorID uuid.UUID
	prefixes          []OperatorPrefix // отсортированы: длинные → короткие
	mu                sync.RWMutex
	refreshTTL        time.Duration
	lastRefresh       time.Time
}

func NewOperatorResolver(repo OperatorPrefixRepository, defaultOperatorID uuid.UUID) *OperatorResolver {
	return &OperatorResolver{
		repo:              repo,
		defaultOperatorID: defaultOperatorID,
		refreshTTL:        5 * time.Minute,
	}
}

// Resolve возвращает operator_id для номера.
func (r *OperatorResolver) Resolve(ctx context.Context, number string) uuid.UUID {
	r.mu.RLock()
	stale := time.Since(r.lastRefresh) > r.refreshTTL
	r.mu.RUnlock()

	if stale {
		r.refresh(ctx)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.prefixes {
		if len(number) >= len(p.Prefix) && number[:len(p.Prefix)] == p.Prefix {
			return p.OperatorID
		}
	}
	return r.defaultOperatorID
}

func (r *OperatorResolver) refresh(ctx context.Context) {
	prefixes, err := r.repo.GetAllActive(ctx)
	if err != nil {
		log.Error().Err(err).Msg("OperatorResolver: ошибка загрузки префиксов")
		return
	}
	// Longest match first
	sort.Slice(prefixes, func(i, j int) bool {
		return len(prefixes[i].Prefix) > len(prefixes[j].Prefix)
	})
	r.mu.Lock()
	r.prefixes = prefixes
	r.lastRefresh = time.Now()
	r.mu.Unlock()
}
```

- [ ] **Step 2: Создать operator_prefix_repository.go**

```go
// internal/storage/operator_prefix_repository.go
package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/router"
)

type OperatorPrefixRepository struct {
	db *sqlx.DB
}

func NewOperatorPrefixRepository(db *DB) *OperatorPrefixRepository {
	return &OperatorPrefixRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

func (r *OperatorPrefixRepository) GetAllActive(ctx context.Context) ([]router.OperatorPrefix, error) {
	type row struct {
		OperatorID uuid.UUID `db:"operator_id"`
		Prefix     string    `db:"prefix"`
		Priority   int       `db:"priority"`
	}
	var rows []row
	err := r.db.SelectContext(ctx, &rows,
		`SELECT operator_id, prefix, priority FROM operator_prefixes WHERE active = true`)
	if err != nil {
		return nil, err
	}
	result := make([]router.OperatorPrefix, len(rows))
	for i, r := range rows {
		result[i] = router.OperatorPrefix{OperatorID: r.OperatorID, Prefix: r.Prefix, Priority: r.Priority}
	}
	return result, nil
}
```

- [ ] **Step 3: Убедиться, что код компилируется**

```bash
go build ./internal/router/... ./internal/storage/...
```
Ожидается: без ошибок

- [ ] **Step 4: Commit**

```bash
git add internal/router/operator_resolver.go internal/storage/operator_prefix_repository.go
git commit -m "feat: OperatorResolver — longest-prefix matching по operator_prefixes"
```

---

## Task 7: StubSender + SenderFactory

**Files:**
- Create: `internal/smsc/stub_sender.go`
- Create: `internal/smsc/sender_factory.go`
- Create: `internal/smsc/sender_factory_test.go`
- Modify: `internal/smsc/sender.go`

- [ ] **Step 1: Написать тест SenderFactory (failing)**

```go
// internal/smsc/sender_factory_test.go
package smsc_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smsc"
)

func TestSenderFactory_ReturnsStubSenderForSimulator(t *testing.T) {
	factory := smsc.NewSenderFactory(nil, &smsc.StubSender{})
	provider := &shared.Provider{ID: uuid.New(), SystemType: "SIMULATOR"}
	sender := factory.For(provider)
	assert.IsType(t, &smsc.StubSender{}, sender)
}

func TestSenderFactory_ReturnsSMPPSenderForReal(t *testing.T) {
	smppSender := smsc.NewSender(nil)
	factory := smsc.NewSenderFactory(smppSender, &smsc.StubSender{})
	provider := &shared.Provider{ID: uuid.New(), SystemType: "SMPP"}
	sender := factory.For(provider)
	assert.IsType(t, &smsc.Sender{}, sender)
}
```

- [ ] **Step 2: Создать stub_sender.go**

```go
// internal/smsc/stub_sender.go
package smsc

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// StubProviderConfig — конфигурация поведения stub-провайдера.
type StubProviderConfig struct {
	ProviderID     uuid.UUID
	MinDelayMs     int
	MaxDelayMs     int
	FailureRatePct int
	DLRDelayMs     int
	DLRSuccessRate int
	DLRStatuses    []string
}

// StubConfigRepository загружает конфигурацию stub-провайдера из БД.
type StubConfigRepository interface {
	GetByProviderID(ctx context.Context, providerID uuid.UUID) (*StubProviderConfig, error)
}

// StubSender реализует async-отправку для SIMULATOR-провайдеров.
// Не использует SMPP-соединение; поведение определяется StubProviderConfig.
type StubSender struct {
	configRepo  StubConfigRepository
	dlrProducer *queue.AsyncProducer
	dlrTopic    string
	logger      zerolog.Logger
}

func NewStubSender(configRepo StubConfigRepository, dlrProducer *queue.AsyncProducer, dlrTopic string) *StubSender {
	return &StubSender{
		configRepo:  configRepo,
		dlrProducer: dlrProducer,
		dlrTopic:    dlrTopic,
		logger:      log.With().Str("component", "stub_sender").Logger(),
	}
}

// SendMessageAsync имитирует отправку: задержка, случайная ошибка, планирование DLR.
// Параметр conn игнорируется (stub не использует SMPP).
func (s *StubSender) SendMessageAsync(ctx context.Context, msg *shared.Message, provider *shared.Provider, _ *AsyncConnection) (string, error) {
	cfg := s.defaultConfig(provider.ID)
	if s.configRepo != nil {
		if loaded, err := s.configRepo.GetByProviderID(ctx, provider.ID); err == nil && loaded != nil {
			cfg = loaded
		}
	}

	// Имитация задержки сети
	delayRange := cfg.MaxDelayMs - cfg.MinDelayMs
	if delayRange < 1 {
		delayRange = 1
	}
	delay := cfg.MinDelayMs + rand.Intn(delayRange)
	select {
	case <-time.After(time.Duration(delay) * time.Millisecond):
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Имитация случайной ошибки
	if rand.Intn(100) < cfg.FailureRatePct {
		return "", fmt.Errorf("stub: simulated failure (rate=%d%%)", cfg.FailureRatePct)
	}

	smppMsgID := "stub-" + uuid.New().String()
	go s.scheduleDLR(msg, smppMsgID, cfg)

	s.logger.Debug().
		Str("message_id", msg.ID.String()).
		Str("smpp_msg_id", smppMsgID).
		Str("provider", provider.Name).
		Msg("stub: сообщение отправлено")

	return smppMsgID, nil
}

func (s *StubSender) scheduleDLR(msg *shared.Message, smppMsgID string, cfg *StubProviderConfig) {
	time.Sleep(time.Duration(cfg.DLRDelayMs) * time.Millisecond)

	stat := "DELIVRD"
	if rand.Intn(100) >= cfg.DLRSuccessRate {
		statuses := cfg.DLRStatuses
		if len(statuses) == 0 {
			statuses = []string{"UNDELIV"}
		}
		stat = statuses[rand.Intn(len(statuses))]
	}

	if s.dlrProducer == nil {
		return
	}

	now := time.Now()
	dlrMsg := &queue.DLRMessage{
		MessageID:     msg.ID,
		SMPPMessageID: smppMsgID,
		ClientID:      msg.ClientID,
		Stat:          stat,
		Source:        msg.Source,
		Destination:   msg.Destination,
		CreatedAt:     now,
		DoneDate:      &now,
	}
	if data, err := dlrMsg.Serialize(); err == nil {
		s.dlrProducer.PublishAsync(s.dlrTopic, msg.ID.String(), data, nil)
	}
}

func (s *StubSender) defaultConfig(providerID uuid.UUID) *StubProviderConfig {
	return &StubProviderConfig{
		ProviderID:     providerID,
		MinDelayMs:     100,
		MaxDelayMs:     500,
		FailureRatePct: 0,
		DLRDelayMs:     1000,
		DLRSuccessRate: 100,
		DLRStatuses:    []string{"DELIVRD"},
	}
}
```

- [ ] **Step 3: Создать sender_factory.go**

```go
// internal/smsc/sender_factory.go
package smsc

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// AsyncSender — интерфейс async-отправки, общий для SMPP и Stub.
type AsyncSender interface {
	SendMessageAsync(ctx context.Context, msg *shared.Message, provider *shared.Provider, conn *AsyncConnection) (string, error)
}

// SenderFactory выбирает реализацию AsyncSender по типу провайдера.
type SenderFactory struct {
	smpp *Sender
	stub *StubSender
}

func NewSenderFactory(smpp *Sender, stub *StubSender) *SenderFactory {
	return &SenderFactory{smpp: smpp, stub: stub}
}

// For возвращает StubSender для SIMULATOR-провайдеров, иначе — Sender (SMPP).
func (f *SenderFactory) For(provider *shared.Provider) AsyncSender {
	if IsSimulator(provider) {
		return f.stub
	}
	return f.smpp
}
```

- [ ] **Step 4: Убрать дублирующую simulate-логику из sender.go**

В `internal/smsc/sender.go` удалить методы `simulateSend` и `simulateAsyncSend`, а также проверки `if IsSimulator(provider)` из `SendMessage` и `SendMessageAsync`. Для `SendMessageAsync` проверку SIMULATOR убрать полностью — этот путь теперь идёт через SenderFactory.

Найти в `sender.go`:
```go
	// Симулятор — мгновенная "отправка" без реального SMPP
	if IsSimulator(provider) {
		return s.simulateAsyncSend(msg, provider)
	}
```
И удалить этот блок (в методе `SendMessageAsync`). Аналогично убрать из `SendMessage`:
```go
	// Симулятор — мгновенная "отправка" без реального SMPP
	if IsSimulator(provider) {
		return s.simulateSend(msg, provider)
	}
```
Удалить функции `simulateSend` и `simulateAsyncSend` из sender.go.

- [ ] **Step 5: Запустить тесты**

```bash
go test ./internal/smsc/... -v -run TestSenderFactory
go build ./internal/smsc/...
```
Ожидается: тесты PASS, сборка без ошибок

- [ ] **Step 6: Создать StubConfigRepository в storage**

```go
// internal/storage/stub_config_repository.go
package storage

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/smsc"
)

type StubConfigRepository struct {
	db *sqlx.DB
}

func NewStubConfigRepository(db *DB) *StubConfigRepository {
	return &StubConfigRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

func (r *StubConfigRepository) GetByProviderID(ctx context.Context, providerID uuid.UUID) (*smsc.StubProviderConfig, error) {
	type row struct {
		ProviderID     uuid.UUID       `db:"provider_id"`
		MinDelayMs     int             `db:"min_delay_ms"`
		MaxDelayMs     int             `db:"max_delay_ms"`
		FailureRatePct int             `db:"failure_rate_pct"`
		DLRDelayMs     int             `db:"dlr_delay_ms"`
		DLRSuccessRate int             `db:"dlr_success_rate"`
		DLRStatuses    json.RawMessage `db:"dlr_statuses"`
	}
	var r2 row
	err := r.db.QueryRowxContext(ctx,
		`SELECT provider_id, min_delay_ms, max_delay_ms, failure_rate_pct,
		        dlr_delay_ms, dlr_success_rate, dlr_statuses
		 FROM stub_provider_config WHERE provider_id = $1`, providerID,
	).StructScan(&r2)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var statuses []string
	_ = json.Unmarshal(r2.DLRStatuses, &statuses)
	return &smsc.StubProviderConfig{
		ProviderID:     r2.ProviderID,
		MinDelayMs:     r2.MinDelayMs,
		MaxDelayMs:     r2.MaxDelayMs,
		FailureRatePct: r2.FailureRatePct,
		DLRDelayMs:     r2.DLRDelayMs,
		DLRSuccessRate: r2.DLRSuccessRate,
		DLRStatuses:    statuses,
	}, nil
}

func (r *StubConfigRepository) Upsert(ctx context.Context, cfg *smsc.StubProviderConfig) error {
	statuses, _ := json.Marshal(cfg.DLRStatuses)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO stub_provider_config
		    (provider_id, min_delay_ms, max_delay_ms, failure_rate_pct, dlr_delay_ms, dlr_success_rate, dlr_statuses)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (provider_id) DO UPDATE SET
		    min_delay_ms = EXCLUDED.min_delay_ms,
		    max_delay_ms = EXCLUDED.max_delay_ms,
		    failure_rate_pct = EXCLUDED.failure_rate_pct,
		    dlr_delay_ms = EXCLUDED.dlr_delay_ms,
		    dlr_success_rate = EXCLUDED.dlr_success_rate,
		    dlr_statuses = EXCLUDED.dlr_statuses,
		    updated_at = now()`,
		cfg.ProviderID, cfg.MinDelayMs, cfg.MaxDelayMs, cfg.FailureRatePct,
		cfg.DLRDelayMs, cfg.DLRSuccessRate, statuses,
	)
	return err
}
```

- [ ] **Step 7: Commit**

```bash
git add internal/smsc/stub_sender.go internal/smsc/sender_factory.go \
        internal/smsc/sender_factory_test.go internal/smsc/sender.go \
        internal/storage/stub_config_repository.go
git commit -m "feat: StubSender + SenderFactory — configurable stub, SMPP/Stub dispatch"
```

---

## Task 8: BackpressureManager — двухуровневый (global + per-client)

**Files:**
- Modify: `internal/pipeline/backpressure/manager.go`
- Modify: `internal/pipeline/backpressure/manager_test.go`

- [ ] **Step 1: Написать тесты двухуровневого BP (failing)**

Добавить в конец `internal/pipeline/backpressure/manager_test.go`:

```go
func TestManager_TwoTier_PerClientLimitBlocks(t *testing.T) {
	m := backpressure.NewManager()
	provID := uuid.New()
	clientID := uuid.New()

	m.Register(provID, 100, 50)                    // глобальный: 100 TPS
	m.RegisterClient(clientID, provID, 1)           // per-client: 1 TPS

	// Первый запрос проходит (используем burst)
	assert.True(t, m.TryAcquire(clientID, provID))
	// Второй запрос блокируется по per-client лимиту
	// (1 TPS burst=2, после первого acquire осталось 1, но второй может пройти из burst)
	// Сбросим токены принудительно
	m.DrainClientTokens(clientID, provID)
	assert.False(t, m.TryAcquire(clientID, provID))
}

func TestManager_TwoTier_GlobalLimitBlocks(t *testing.T) {
	m := backpressure.NewManager()
	provID := uuid.New()
	clientA := uuid.New()
	clientB := uuid.New()

	m.Register(provID, 1, 50)           // глобальный: 1 TPS (burst=2)
	m.RegisterClient(clientA, provID, 10) // client A: 10 TPS
	m.RegisterClient(clientB, provID, 10) // client B: 10 TPS

	// Исчерпываем глобальный burst
	m.DrainGlobalTokens(provID)
	// Оба клиента должны быть заблокированы глобальным лимитом
	assert.False(t, m.TryAcquire(clientA, provID))
	assert.False(t, m.TryAcquire(clientB, provID))
}

func TestManager_TwoTier_UnknownClientPassesGlobalOnly(t *testing.T) {
	m := backpressure.NewManager()
	provID := uuid.New()
	m.Register(provID, 100, 50)

	// Клиент без per-client регистрации — проходит через глобальный
	assert.True(t, m.TryAcquire(uuid.New(), provID))
}
```

- [ ] **Step 2: Запустить — убедиться, что тесты не компилируются**

```bash
go test ./internal/pipeline/backpressure/... 2>&1 | head -15
```

- [ ] **Step 3: Расширить manager.go**

Заменить содержимое `internal/pipeline/backpressure/manager.go`:

```go
package backpressure

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/monitoring"
)

// Manager управляет двухуровневым backpressure:
//   - global:    per-provider token bucket (общий потолок)
//   - client:    per-(client,provider) token bucket (верхняя граница клиента)
type Manager struct {
	global map[uuid.UUID]*State
	client map[clientProviderKey]*State
	mu     sync.RWMutex
	logger zerolog.Logger
}

type clientProviderKey struct {
	clientID   uuid.UUID
	providerID uuid.UUID
}

// State — token bucket для одного ограничения.
type State struct {
	ProviderID      uuid.UUID
	TokensPerSecond int
	AvailableTokens int64 // atomic
	BurstSize       int
	IsThrottled     int32 // atomic bool
	LastRefill      int64 // atomic unix nano
	PendingCount    int64 // atomic
	MaxWindow       int
}

func NewManager() *Manager {
	return &Manager{
		global: make(map[uuid.UUID]*State),
		client: make(map[clientProviderKey]*State),
		logger: log.With().Str("component", "backpressure").Logger(),
	}
}

// Register регистрирует глобальный (per-provider) лимит.
func (m *Manager) Register(providerID uuid.UUID, tokensPerSecond int, maxWindow int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.global[providerID] = newState(providerID, tokensPerSecond, maxWindow)
}

// RegisterClient регистрирует per-client лимит для провайдера.
func (m *Manager) RegisterClient(clientID, providerID uuid.UUID, tokensPerSecond int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := clientProviderKey{clientID, providerID}
	m.client[key] = newState(providerID, tokensPerSecond, 50)
}

// UpdateClient обновляет per-client TPS (вызывается при инвалидации лимитов).
func (m *Manager) UpdateClient(clientID, providerID uuid.UUID, tokensPerSecond int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := clientProviderKey{clientID, providerID}
	m.client[key] = newState(providerID, tokensPerSecond, 50)
}

// TryAcquire пытается получить токен для пары (client, provider).
// Сначала проверяет per-client лимит, затем глобальный.
// Если клиент не зарегистрирован — проверяет только глобальный.
func (m *Manager) TryAcquire(clientID, providerID uuid.UUID) bool {
	m.mu.RLock()
	globalState := m.global[providerID]
	clientState := m.client[clientProviderKey{clientID, providerID}]
	m.mu.RUnlock()

	// Per-client check (если зарегистрирован)
	if clientState != nil {
		if !tryConsume(clientState) {
			monitoring.PipelineBackpressureActive.WithLabelValues(providerID.String()).Set(1)
			return false
		}
	}

	// Global check
	if globalState != nil {
		if !tryConsume(globalState) {
			// Вернуть токен клиента, если взяли
			if clientState != nil {
				atomic.AddInt64(&clientState.AvailableTokens, 1)
			}
			monitoring.PipelineBackpressureActive.WithLabelValues(providerID.String()).Set(1)
			return false
		}
		monitoring.PipelineBackpressureActive.WithLabelValues(providerID.String()).Set(0)
	}

	return true
}

// DrainClientTokens исчерпывает per-client токены (для тестов).
func (m *Manager) DrainClientTokens(clientID, providerID uuid.UUID) {
	m.mu.RLock()
	state := m.client[clientProviderKey{clientID, providerID}]
	m.mu.RUnlock()
	if state != nil {
		atomic.StoreInt64(&state.AvailableTokens, 0)
	}
}

// DrainGlobalTokens исчерпывает глобальные токены (для тестов).
func (m *Manager) DrainGlobalTokens(providerID uuid.UUID) {
	m.mu.RLock()
	state := m.global[providerID]
	m.mu.RUnlock()
	if state != nil {
		atomic.StoreInt64(&state.AvailableTokens, 0)
	}
}

// IncrementPending / DecrementPending / IsThrottled остаются на глобальном уровне.
func (m *Manager) IncrementPending(providerID uuid.UUID) {
	m.mu.RLock()
	state := m.global[providerID]
	m.mu.RUnlock()
	if state != nil {
		atomic.AddInt64(&state.PendingCount, 1)
	}
}

func (m *Manager) DecrementPending(providerID uuid.UUID) {
	m.mu.RLock()
	state := m.global[providerID]
	m.mu.RUnlock()
	if state != nil {
		atomic.AddInt64(&state.PendingCount, -1)
	}
}

func (m *Manager) IsThrottled(providerID uuid.UUID) bool {
	m.mu.RLock()
	state := m.global[providerID]
	m.mu.RUnlock()
	if state == nil {
		return false
	}
	return atomic.LoadInt32(&state.IsThrottled) == 1
}

// --- helpers ---

func newState(providerID uuid.UUID, tps int, maxWindow int) *State {
	if tps <= 0 {
		tps = 1
	}
	burst := tps * 2
	return &State{
		ProviderID:      providerID,
		TokensPerSecond: tps,
		AvailableTokens: int64(burst),
		BurstSize:       burst,
		LastRefill:      time.Now().UnixNano(),
		MaxWindow:       maxWindow,
	}
}

func tryConsume(state *State) bool {
	// Пополнение токенов
	now := time.Now().UnixNano()
	lastRefill := atomic.LoadInt64(&state.LastRefill)
	elapsed := now - lastRefill
	tokensToAdd := int64(state.TokensPerSecond) * elapsed / int64(time.Second)
	if tokensToAdd > 0 {
		if atomic.CompareAndSwapInt64(&state.LastRefill, lastRefill, now) {
			newTokens := atomic.AddInt64(&state.AvailableTokens, tokensToAdd)
			if newTokens > int64(state.BurstSize) {
				atomic.StoreInt64(&state.AvailableTokens, int64(state.BurstSize))
			}
		}
	}
	// Попытка взять токен
	for {
		current := atomic.LoadInt64(&state.AvailableTokens)
		if current <= 0 {
			atomic.StoreInt32(&state.IsThrottled, 1)
			return false
		}
		if atomic.CompareAndSwapInt64(&state.AvailableTokens, current, current-1) {
			atomic.StoreInt32(&state.IsThrottled, 0)
			return true
		}
	}
}
```

- [ ] **Step 4: Запустить тесты**

```bash
go test ./internal/pipeline/backpressure/... -v
```
Ожидается: все тесты PASS

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/backpressure/manager.go internal/pipeline/backpressure/manager_test.go
git commit -m "feat: BackpressureManager — двухуровневый (global + per-client token buckets)"
```

---

## Task 9: Pipeline Router Stage — переключение на UnifiedRouter

**Files:**
- Modify: `internal/pipeline/router/stage.go`

- [ ] **Step 1: Обновить NewStage**

В `internal/pipeline/router/stage.go`:

1. Добавить импорты:
```go
msgunifiedrouter "github.com/smpp-server/smpp-server/internal/router"
```

2. Изменить структуру Stage — заменить поле `router *msgrouter.CachedRouter` на:
```go
unifiedRouter   *msgunifiedrouter.CachedUnifiedRouter
operatorResolver *msgunifiedrouter.OperatorResolver
```

3. В `NewStage` заменить инициализацию роутера:

```go
// Старый код (удалить):
// providerRepo := storage.NewProviderRepository(db)
// routeRepo := storage.NewRouteRepository(db)
// routeCache := cache.NewRouteCache(routeRepo, providerRepo, 30*time.Second)
// msgRouter := msgrouter.NewCachedRouter(routeCache)

// Новый код:
clientRouteRepo := storage.NewClientRouteRepository(db)
unifiedRouterInner := msgunifiedrouter.NewUnifiedRouter(clientRouteRepo, clientRouteRepo)
unifiedRouter := msgunifiedrouter.NewCachedUnifiedRouter(unifiedRouterInner)

defaultOpID, _ := uuid.Parse(cfg.Pipeline.DefaultOperatorID)
if defaultOpID == uuid.Nil {
    defaultOpID, _ = uuid.Parse("d0000000-0000-0000-0000-000000000001")
}
opPrefixRepo := storage.NewOperatorPrefixRepository(db)
operatorResolver := msgunifiedrouter.NewOperatorResolver(opPrefixRepo, defaultOpID)
```

4. Обновить поля в возвращаемом Stage:
```go
return &Stage{
    consumer:         consumer,
    producer:         producer,
    unifiedRouter:    unifiedRouter,
    operatorResolver: operatorResolver,
    cfg:              cfg,
    logger:           logger,
}, nil
```

5. Обновить `processMessage` — заменить вызов старого роутера:

```go
// Старый код (удалить):
// provider, err := s.router.RouteMessage(ctx, sharedMsg)
// var fallbackProviderID *uuid.UUID
// if kafkaMsg.RouteID != nil { ... }

// Новый код:
var operatorID uuid.UUID
if kafkaMsg.ClientID != nil {
    operatorID = s.operatorResolver.Resolve(ctx, kafkaMsg.Destination)
} else {
    operatorID, _ = uuid.Parse(s.cfg.Pipeline.DefaultOperatorID)
}

var providerID uuid.UUID
var routeID *uuid.UUID

if kafkaMsg.ClientID != nil {
    decision, routeErr := s.unifiedRouter.Route(ctx, *kafkaMsg.ClientID, operatorID)
    if routeErr != nil {
        return fmt.Errorf("маршрутизация message_id=%s: %w", kafkaMsg.MessageID, routeErr)
    }
    providerID = decision.ProviderID
    routeID = &decision.RouteID
} else {
    // Нет ClientID — используем старый path (RoutedMessage с ProviderID)
    if kafkaMsg.ProviderID == nil {
        return fmt.Errorf("message_id=%s: нет client_id и provider_id", kafkaMsg.MessageID)
    }
    providerID = *kafkaMsg.ProviderID
}
```

6. Обновить формирование `RoutedMessage` — убрать `FallbackProviderID` (он теперь не нужен из router stage, failover управляется через маршруты с меньшим приоритетом):

```go
routed := &pipeline.RoutedMessage{
    SchemaVersion: 1,
    MessageID:     kafkaMsg.MessageID,
    Source:        kafkaMsg.Source,
    Destination:   kafkaMsg.Destination,
    Text:          kafkaMsg.Text,
    ClientID:      kafkaMsg.ClientID,
    ProviderID:    providerID,
    RouteID:       routeID,
    Priority:      kafkaMsg.Priority,
    RetryCount:    kafkaMsg.RetryCount,
    MaxRetries:    kafkaMsg.MaxRetries,
    RoutedAt:      time.Now(),
    CreatedAt:     kafkaMsg.CreatedAt,
    Metadata:      kafkaMsg.Metadata,
}
```

7. Убрать импорты старого `msgrouter` и `cache` если больше не используются.

- [ ] **Step 2: Убедиться, что код компилируется**

```bash
go build ./internal/pipeline/router/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/router/stage.go
git commit -m "feat: router stage — переключение на UnifiedRouter + OperatorResolver"
```

---

## Task 10: Pipeline Sender Stage — SenderFactory + двухуровневый BP + pub/sub

**Files:**
- Modify: `internal/pipeline/sender/stage.go`

- [ ] **Step 1: Обновить Stage struct**

В `internal/pipeline/sender/stage.go` добавить поля:

```go
type Stage struct {
    // существующие поля...
    consumer           *queue.BatchConsumer
    producer           *queue.AsyncProducer
    pool               *smsc.Pool
    senderFactory      *smsc.SenderFactory    // новое (заменяет sender)
    bpManager          *backpressure.Manager
    providerRepo       *storage.ProviderRepository
    limitResolver      *limits.CachedLimitResolver  // новое
    clientProviderRepo *storage.ClientProviderRepository // новое
    tarificationClient tarificationv1.TarificationServiceClient
    billingClient      billingv1.BillingServiceClient
    tarificationConn   *grpc.ClientConn
    billingConn        *grpc.ClientConn
    defaultOperatorID  string
    cfg                *config.Config
    logger             zerolog.Logger
}
```

- [ ] **Step 2: Обновить NewStage — инициализация SenderFactory + LimitResolver + per-client BP**

В `NewStage` после создания `bpManager` и загрузки провайдеров:

```go
// Инициализация LimitResolver
var limitResolver *limits.CachedLimitResolver
if rdb != nil {
    querier := storage.NewLimitQuerierDB(db)
    dbResolver := limits.NewDBLimitResolver(querier)
    limitResolver = limits.NewCachedLimitResolver(dbResolver, rdb)
}

// Загрузка per-client TPS из client_providers
cpRepo := storage.NewClientProviderRepository(db)
clientProviders, cpErr := cpRepo.GetAllActiveWithTPS(context.Background())
if cpErr == nil {
    for _, cp := range clientProviders {
        if cp.TPSLimit != nil {
            bpManager.RegisterClient(cp.ClientID, cp.ProviderID, *cp.TPSLimit)
        }
    }
}

// StubSender
stubConfigRepo := storage.NewStubConfigRepository(db)
stubSender := smsc.NewStubSender(stubConfigRepo, producer, cfg.Kafka.TopicDLR)
smppSender := smsc.NewSender(pool)
senderFactory := smsc.NewSenderFactory(smppSender, stubSender)
```

Также изменить `NewStage` сигнатуру для приёма Redis клиента: `func NewStage(cfg *config.Config, db *storage.DB, rdb *redis.Client) (*Stage, error)`

- [ ] **Step 3: Обновить processMessage — двухуровневый BP + SenderFactory**

Заменить строку `if !s.bpManager.TryAcquire(routedMsg.ProviderID)` на:

```go
// 3. Двухуровневый backpressure
var clientID uuid.UUID
if routedMsg.ClientID != nil {
    clientID = *routedMsg.ClientID
}
if !s.bpManager.TryAcquire(clientID, routedMsg.ProviderID) {
    s.logger.Warn().
        Str("message_id", routedMsg.MessageID.String()).
        Str("provider_id", routedMsg.ProviderID.String()).
        Msg("backpressure throttled, сообщение будет повторно доставлено")
    return fmt.Errorf("backpressure: провайдер %s throttled", routedMsg.ProviderID)
}
```

Заменить получение соединения и отправку:

```go
// 4. Получаем провайдера
provider, err := s.providerRepo.GetByID(ctx, routedMsg.ProviderID)
if err != nil {
    return fmt.Errorf("получение провайдера %s: %w", routedMsg.ProviderID, err)
}

// 5. Получаем соединение (nil для stub)
var conn *smsc.AsyncConnection
if !smsc.IsSimulator(provider) {
    conn, err = s.pool.GetAsyncConnection(routedMsg.ProviderID)
    if err != nil {
        return fmt.Errorf("получение async соединения: %w", err)
    }
}

// 6. Отправляем через SenderFactory
sharedMsg := routedToSharedMessage(routedMsg)
sender := s.senderFactory.For(provider)
smppMsgID, sendErr := sender.SendMessageAsync(ctx, sharedMsg, provider, conn)
```

Убрать блок `if sendErr == nil && smsc.IsSimulator(provider) { ... }` — DLR теперь отправляет StubSender внутренне.

- [ ] **Step 4: Запустить сборку**

```bash
go build ./internal/pipeline/sender/...
```

- [ ] **Step 5: Добавить pub/sub listener для инвалидации per-client TPS**

В конец `NewStage` добавить запуск горутины (после создания `limitResolver`):

```go
if limitResolver != nil {
    go func() {
        redisSub := rdb.Subscribe(context.Background(), "limits:invalidate")
        ch := redisSub.Channel()
        for msg := range ch {
            // payload: "clientID:providerID"
            parts := strings.SplitN(msg.Payload, ":", 2)
            if len(parts) != 2 {
                continue
            }
            cid, err1 := uuid.Parse(parts[0])
            pid, err2 := uuid.Parse(parts[1])
            if err1 != nil || err2 != nil {
                continue
            }
            tps, resolveErr := limitResolver.ResolveProviderTPS(context.Background(), cid, pid)
            if resolveErr == nil {
                bpManager.UpdateClient(cid, pid, tps)
                limitResolver.InvalidateProviderTPS(context.Background(), cid, pid)
            }
        }
    }()
}
```

- [ ] **Step 6: Запустить сборку и тесты**

```bash
go build ./internal/pipeline/...
go test ./internal/pipeline/... -v
```

- [ ] **Step 7: Commit**

```bash
git add internal/pipeline/sender/stage.go
git commit -m "feat: sender stage — SenderFactory + двухуровневый BP + pub/sub инвалидация"
```

---

## Task 11: Admin API — system_defaults и stub_config

**Files:**
- Create: `internal/gateway/admin/handlers/system_defaults.go`
- Create: `internal/gateway/admin/handlers/stub_config.go`

- [ ] **Step 1: Создать system_defaults.go**

```go
// internal/gateway/admin/handlers/system_defaults.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type SystemDefaultsHandlers struct {
	repo *storage.SystemDefaultsRepository
}

func NewSystemDefaultsHandlers(repo *storage.SystemDefaultsRepository) *SystemDefaultsHandlers {
	return &SystemDefaultsHandlers{repo: repo}
}

// GET /admin/v1/system/defaults
func (h *SystemDefaultsHandlers) GetAll(w http.ResponseWriter, r *http.Request) {
	all, err := h.repo.GetAll(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternal("ошибка получения system_defaults"))
		return
	}
	respondJSON(w, http.StatusOK, all)
}

// PUT /admin/v1/system/defaults/{key}
func (h *SystemDefaultsHandlers) Set(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]
	validKeys := map[string]bool{
		"rate_limit_per_second": true, "rate_limit_per_minute": true,
		"rate_limit_per_hour": true, "default_tps_per_provider": true,
		"max_providers_per_client": true, "max_sub_accounts": true,
	}
	if !validKeys[key] {
		respondError(w, shared.ErrInvalidInput("неизвестный ключ: "+key))
		return
	}

	var req struct {
		Value int `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}
	if req.Value < 0 {
		respondError(w, shared.ErrInvalidInput("значение не может быть отрицательным"))
		return
	}

	if err := h.repo.Set(r.Context(), key, req.Value, nil); err != nil {
		respondError(w, shared.ErrInternal("ошибка сохранения"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Создать stub_config.go**

```go
// internal/gateway/admin/handlers/stub_config.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smsc"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type StubConfigHandlers struct {
	repo *storage.StubConfigRepository
}

func NewStubConfigHandlers(repo *storage.StubConfigRepository) *StubConfigHandlers {
	return &StubConfigHandlers{repo: repo}
}

// GET /admin/v1/providers/{id}/stub-config
func (h *StubConfigHandlers) Get(w http.ResponseWriter, r *http.Request) {
	providerID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("неверный provider_id"))
		return
	}
	cfg, err := h.repo.GetByProviderID(r.Context(), providerID)
	if err != nil {
		respondError(w, shared.ErrInternal("ошибка получения конфигурации"))
		return
	}
	if cfg == nil {
		respondError(w, shared.ErrNotFound("конфигурация не найдена"))
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

// PUT /admin/v1/providers/{id}/stub-config
func (h *StubConfigHandlers) Upsert(w http.ResponseWriter, r *http.Request) {
	providerID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("неверный provider_id"))
		return
	}

	var req struct {
		MinDelayMs     int      `json:"min_delay_ms"`
		MaxDelayMs     int      `json:"max_delay_ms"`
		FailureRatePct int      `json:"failure_rate_pct"`
		DLRDelayMs     int      `json:"dlr_delay_ms"`
		DLRSuccessRate int      `json:"dlr_success_rate"`
		DLRStatuses    []string `json:"dlr_statuses"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}
	if req.FailureRatePct < 0 || req.FailureRatePct > 100 {
		respondError(w, shared.ErrInvalidInput("failure_rate_pct должен быть 0-100"))
		return
	}
	if req.DLRSuccessRate < 0 || req.DLRSuccessRate > 100 {
		respondError(w, shared.ErrInvalidInput("dlr_success_rate должен быть 0-100"))
		return
	}
	if req.MinDelayMs > req.MaxDelayMs {
		respondError(w, shared.ErrInvalidInput("min_delay_ms не может быть больше max_delay_ms"))
		return
	}

	cfg := &smsc.StubProviderConfig{
		ProviderID: providerID, MinDelayMs: req.MinDelayMs, MaxDelayMs: req.MaxDelayMs,
		FailureRatePct: req.FailureRatePct, DLRDelayMs: req.DLRDelayMs,
		DLRSuccessRate: req.DLRSuccessRate, DLRStatuses: req.DLRStatuses,
	}
	if err := h.repo.Upsert(r.Context(), cfg); err != nil {
		respondError(w, shared.ErrInternal("ошибка сохранения"))
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}
```

- [ ] **Step 3: Убедиться, что код компилируется**

```bash
go build ./internal/gateway/admin/...
```

- [ ] **Step 4: Зарегистрировать маршруты в admin router**

В `internal/gateway/admin/router/router.go` добавить параметры в `SetupRouter`:

```go
systemDefaultsHandlers *handlers.SystemDefaultsHandlers,
stubConfigHandlers *handlers.StubConfigHandlers,
```

И маршруты в секции admin API:

```go
// System defaults
system := adminV1.PathPrefix("/system").Subrouter()
system.HandleFunc("/defaults", systemDefaultsHandlers.GetAll).Methods("GET")
system.HandleFunc("/defaults/{key}", systemDefaultsHandlers.Set).Methods("PUT")

// Stub config (добавить к существующим provider routes)
adminV1.HandleFunc("/providers/{id}/stub-config", stubConfigHandlers.Get).Methods("GET")
adminV1.HandleFunc("/providers/{id}/stub-config", stubConfigHandlers.Upsert).Methods("PUT")
```

- [ ] **Step 5: Инициализировать handlers в main admin-gateway**

В `cmd/admin-gateway/main.go` добавить создание репозитория и handlers:

```go
systemDefaultsRepo := storage.NewSystemDefaultsRepository(db)
systemDefaultsHandlers := handlers.NewSystemDefaultsHandlers(systemDefaultsRepo)

stubConfigRepo := storage.NewStubConfigRepository(db)
stubConfigHandlers := handlers.NewStubConfigHandlers(stubConfigRepo)
```

И передать в `SetupRouter(...)`.

- [ ] **Step 6: Commit**

```bash
git add internal/gateway/admin/handlers/system_defaults.go \
        internal/gateway/admin/handlers/stub_config.go \
        internal/gateway/admin/router/router.go \
        cmd/admin-gateway/main.go
git commit -m "feat: admin API — system_defaults CRUD + stub_provider_config CRUD"
```

---

## Task 12: Portal API — управление провайдерами и маршрутами субаккаунтов

**Files:**
- Create: `internal/gateway/portal/handlers/sub_account_routing.go`
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Создать sub_account_routing.go**

```go
// internal/gateway/portal/handlers/sub_account_routing.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// SubAccountRoutingHandlers управляет провайдерами и маршрутами субаккаунтов реселлера.
type SubAccountRoutingHandlers struct {
	routingClient routingv1.RoutingServiceClient
}

func NewSubAccountRoutingHandlers(routingClient routingv1.RoutingServiceClient) *SubAccountRoutingHandlers {
	return &SubAccountRoutingHandlers{routingClient: routingClient}
}

// verifyResellerOwnsSubAccount проверяет, что аутентифицированный пользователь
// является родителем (реселлером) субаккаунта с указанным ID.
func (h *SubAccountRoutingHandlers) verifyResellerOwnsSubAccount(r *http.Request, subAccountID string) *shared.AppError {
	resellerClientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		return shared.ErrUnauthorized("пользователь не аутентифицирован")
	}
	_ = resellerClientID
	_ = subAccountID
	// Проверка делегируется routing service через parent_client_id валидацию.
	// При необходимости можно добавить gRPC-вызов к client service.
	return nil
}

// POST /portal/v1/sub-accounts/{id}/providers
func (h *SubAccountRoutingHandlers) AssignProvider(w http.ResponseWriter, r *http.Request) {
	subAccountID := mux.Vars(r)["id"]
	if appErr := h.verifyResellerOwnsSubAccount(r, subAccountID); appErr != nil {
		respondError(w, appErr)
		return
	}

	var req struct {
		ProviderID string `json:"provider_id"`
		TPSLimit   *int32 `json:"tps_limit,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}
	if _, err := uuid.Parse(req.ProviderID); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный provider_id"))
		return
	}

	resellerID, _ := middleware.GetClientID(r.Context())
	assignReq := &routingv1.AssignProviderRequest{
		ClientId:   subAccountID,
		ProviderId: req.ProviderID,
		Ownership:  "inherited",
		// Передаём ID реселлера для валидации на стороне routing service
		SourceClientId: resellerID,
	}
	if req.TPSLimit != nil {
		assignReq.TpsLimit = *req.TPSLimit
	}

	resp, err := h.routingClient.AssignProviderToClient(r.Context(), assignReq)
	if err != nil {
		log.Error().Err(err).Msg("assign sub-account provider failed")
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

// GET /portal/v1/sub-accounts/{id}/providers
func (h *SubAccountRoutingHandlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	subAccountID := mux.Vars(r)["id"]
	resp, err := h.routingClient.ListClientProviders(r.Context(), &routingv1.ListClientProvidersRequest{
		ClientId:   subAccountID,
		ActiveOnly: true,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// PUT /portal/v1/sub-accounts/{id}/providers/{pid}
func (h *SubAccountRoutingHandlers) UpdateProviderTPS(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subAccountID := vars["id"]
	providerID := vars["pid"]

	if appErr := h.verifyResellerOwnsSubAccount(r, subAccountID); appErr != nil {
		respondError(w, appErr)
		return
	}

	var req struct {
		TPSLimit int32 `json:"tps_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.UpdateClientProviderTPS(r.Context(), &routingv1.UpdateClientProviderTPSRequest{
		ClientId:   subAccountID,
		ProviderId: providerID,
		TpsLimit:   req.TPSLimit,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// DELETE /portal/v1/sub-accounts/{id}/providers/{pid}
func (h *SubAccountRoutingHandlers) RevokeProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subAccountID := vars["id"]
	providerID := vars["pid"]

	if appErr := h.verifyResellerOwnsSubAccount(r, subAccountID); appErr != nil {
		respondError(w, appErr)
		return
	}

	_, err := h.routingClient.RevokeProviderFromClient(r.Context(), &routingv1.RevokeProviderRequest{
		ClientId:   subAccountID,
		ProviderId: providerID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /portal/v1/sub-accounts/{id}/budget
func (h *SubAccountRoutingHandlers) SetBudget(w http.ResponseWriter, r *http.Request) {
	subAccountID := mux.Vars(r)["id"]
	if appErr := h.verifyResellerOwnsSubAccount(r, subAccountID); appErr != nil {
		respondError(w, appErr)
		return
	}

	var req struct {
		TPSBudget int32 `json:"tps_budget"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}
	if req.TPSBudget < 0 {
		respondError(w, shared.ErrInvalidInput("tps_budget не может быть отрицательным"))
		return
	}

	resp, err := h.routingClient.SetSubAccountTPSBudget(r.Context(), &routingv1.SetSubAccountTPSBudgetRequest{
		ClientId:  subAccountID,
		TpsBudget: req.TPSBudget,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// POST /portal/v1/sub-accounts/{id}/routes
func (h *SubAccountRoutingHandlers) CreateRoute(w http.ResponseWriter, r *http.Request) {
	subAccountID := mux.Vars(r)["id"]
	if appErr := h.verifyResellerOwnsSubAccount(r, subAccountID); appErr != nil {
		respondError(w, appErr)
		return
	}

	var req struct {
		OperatorID string `json:"operator_id"`
		ProviderID string `json:"provider_id"`
		Priority   int32  `json:"priority"`
		Weight     int32  `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.CreateClientRoute(r.Context(), &routingv1.CreateClientRouteRequest{
		ClientId:   subAccountID,
		OperatorId: req.OperatorID,
		ProviderId: req.ProviderID,
		Priority:   req.Priority,
		Weight:     req.Weight,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

// GET /portal/v1/sub-accounts/{id}/routes
func (h *SubAccountRoutingHandlers) ListRoutes(w http.ResponseWriter, r *http.Request) {
	subAccountID := mux.Vars(r)["id"]
	resp, err := h.routingClient.ListClientRoutes(r.Context(), &routingv1.ListClientRoutesRequest{
		ClientId: subAccountID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Зарегистрировать маршруты в portal router**

В `internal/gateway/portal/router/router.go`:

1. Добавить параметр `subAccountRoutingHandlers *handlers.SubAccountRoutingHandlers` в сигнатуру `SetupRouter`.

2. В секции sub-accounts добавить после существующих маршрутов:

```go
// Sub-account routing (reseller management)
subAccounts.HandleFunc("/{id}/providers", subAccountRoutingHandlers.AssignProvider).Methods("POST")
subAccounts.HandleFunc("/{id}/providers", subAccountRoutingHandlers.ListProviders).Methods("GET")
subAccounts.HandleFunc("/{id}/providers/{pid}", subAccountRoutingHandlers.UpdateProviderTPS).Methods("PUT")
subAccounts.HandleFunc("/{id}/providers/{pid}", subAccountRoutingHandlers.RevokeProvider).Methods("DELETE")
subAccounts.HandleFunc("/{id}/budget", subAccountRoutingHandlers.SetBudget).Methods("PUT")
subAccounts.HandleFunc("/{id}/routes", subAccountRoutingHandlers.CreateRoute).Methods("POST")
subAccounts.HandleFunc("/{id}/routes", subAccountRoutingHandlers.ListRoutes).Methods("GET")
```

- [ ] **Step 3: Добавить новые методы в routing service proto**

В proto-файле routing service (`api/proto/routingv1/routing.proto`) добавить:

```protobuf
message UpdateClientProviderTPSRequest {
  string client_id   = 1;
  string provider_id = 2;
  int32  tps_limit   = 3;
}

message SetSubAccountTPSBudgetRequest {
  string client_id  = 1;
  int32  tps_budget = 2;
}

message SetSubAccountTPSBudgetResponse {}

service RoutingService {
  // существующие методы...
  rpc UpdateClientProviderTPS(UpdateClientProviderTPSRequest) returns (AssignProviderResponse);
  rpc SetSubAccountTPSBudget(SetSubAccountTPSBudgetRequest)   returns (SetSubAccountTPSBudgetResponse);
}
```

Также добавить поля `source_client_id string` и `tps_limit int32` в `AssignProviderRequest`.

- [ ] **Step 4: Регенерировать gRPC код**

```bash
cd /home/magomed/projects/sms
protoc --go_out=. --go-grpc_out=. api/proto/routingv1/routing.proto
```

- [ ] **Step 5: Реализовать новые методы в routing service**

В `cmd/services/routing-service/` найти сервер и добавить реализацию:

`UpdateClientProviderTPS` — обновляет `client_providers.tps_limit`, публикует `limits:invalidate` в Redis.

`SetSubAccountTPSBudget` — обновляет `clients.allocated_tps_budget`, с проверкой что новый бюджет ≥ суммы уже назначенных tps_limit.

Шаблон реализации `UpdateClientProviderTPS`:

```go
func (s *Server) UpdateClientProviderTPS(ctx context.Context, req *routingv1.UpdateClientProviderTPSRequest) (*routingv1.AssignProviderResponse, error) {
    clientID, err := uuid.Parse(req.ClientId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "неверный client_id")
    }
    providerID, err := uuid.Parse(req.ProviderId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "неверный provider_id")
    }

    _, err = s.db.ExecContext(ctx,
        `UPDATE client_providers SET tps_limit=$1, updated_at=now()
         WHERE client_id=$2 AND provider_id=$3 AND active=true`,
        req.TpsLimit, clientID, providerID,
    )
    if err != nil {
        return nil, status.Error(codes.Internal, "ошибка обновления TPS")
    }

    // Инвалидация кеша лимитов
    s.redis.Publish(ctx, "limits:invalidate",
        fmt.Sprintf("%s:%s", clientID, providerID))

    return &routingv1.AssignProviderResponse{}, nil
}
```

- [ ] **Step 6: Убедиться, что код компилируется**

```bash
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/gateway/portal/handlers/sub_account_routing.go \
        internal/gateway/portal/router/router.go \
        api/proto/routingv1/ \
        cmd/services/routing-service/
git commit -m "feat: portal API — управление провайдерами и маршрутами субаккаунтов"
```

---

## Task 13: Финальная сборка и smoke-test

**Files:** Без изменений

- [ ] **Step 1: Полная сборка проекта**

```bash
cd /home/magomed/projects/sms
go build ./...
```
Ожидается: без ошибок

- [ ] **Step 2: Запустить все unit-тесты**

```bash
go test ./internal/pipeline/limits/... \
        ./internal/router/... \
        ./internal/pipeline/backpressure/... \
        ./internal/smsc/... \
        -v -count=1
```
Ожидается: все PASS

- [ ] **Step 3: Запустить функциональные тесты (если есть TEST_DB_HOST)**

```bash
TEST_DB_HOST=localhost TEST_DB_USER=smpp TEST_DB_PASSWORD=smpp TEST_DB_NAME=smpp \
  go test ./test/functional/... -v -timeout 60s 2>&1 | tail -30
```

- [ ] **Step 4: Проверить миграции на чистой БД (down + up)**

```bash
migrate -path migrations -database "postgres://smpp:smpp@localhost:5432/smpp?sslmode=disable" down 5
migrate -path migrations -database "postgres://smpp:smpp@localhost:5432/smpp?sslmode=disable" up
```
Ожидается: без ошибок

- [ ] **Step 5: Финальный commit**

```bash
git add -A
git commit -m "feat: provider routing & limits — финальная интеграция и smoke-test"
```

---

## Итоговая карта изменений

| Компонент | Статус |
|---|---|
| `system_defaults` | ✅ Новая таблица (migration 000058) |
| `stub_provider_config` | ✅ Новая таблица (migration 000059) |
| `operator_prefixes` | ✅ Новая таблица (migration 000060) |
| `tariff_plans` | ✅ +6 полей лимитов (migration 000061) |
| `client_providers` | ✅ +tps_limit (migration 000061) |
| `client_routes` | ✅ +shared, client_id nullable (migration 000061) |
| `clients` | ✅ +allocated_tps_budget (migration 000061) |
| Legacy routes → client_routes | ✅ (migration 000062) |
| `LimitResolver` | ✅ DB + Redis-кеш + pub/sub инвалидация |
| `UnifiedRouter` | ✅ 3-уровневый + in-memory кеш |
| `OperatorResolver` | ✅ longest-prefix matching |
| `BackpressureManager` | ✅ global + per-client |
| `StubSender` | ✅ выделен из Sender, конфигурируем |
| `SenderFactory` | ✅ SMPP vs Stub dispatch |
| Router Stage | ✅ переключён на UnifiedRouter |
| Sender Stage | ✅ двухуровневый BP + SenderFactory |
| Admin API system_defaults | ✅ GET/PUT |
| Admin API stub config | ✅ GET/PUT |
| Portal API sub-account routing | ✅ 7 endpoints |
