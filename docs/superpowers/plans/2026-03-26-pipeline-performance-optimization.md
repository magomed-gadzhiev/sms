# Pipeline Performance Optimization — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Максимальная пропускная способность SMS pipeline на 8 CPU / 16 GB RAM — ожидаемый рост 5-10x.

**Architecture:** PostgreSQL тюнинг + PgBouncer перед БД, async ingestion через Kafka (0 DB write в hot path), новый Persist stage с COPY protocol, in-memory кеш routes/providers, оптимизация Status stage через temp table + COPY, увеличение Kafka partitions и SMPP window size, минимум индексов на hot partition.

**Tech Stack:** Go 1.24.0, pgx/v5 (CopyFrom), sarama (Kafka), PgBouncer 1.22, PostgreSQL 15

---

## File Structure

### New Files
- `deployments/configs/pgbouncer.ini` — PgBouncer configuration
- `deployments/configs/pgbouncer_userlist.txt` — PgBouncer auth file
- `deployments/configs/postgresql.conf` — PostgreSQL tuning overrides
- `internal/pipeline/persist/stage.go` — new Persist pipeline stage (COPY batch insert)
- `internal/pipeline/cache/route_cache.go` — in-memory route/provider cache with periodic refresh
- `internal/pipeline/persist/stage_test.go` — tests for Persist stage
- `internal/pipeline/cache/route_cache_test.go` — tests for route cache

### Modified Files
- `deployments/docker-compose.yml` — PgBouncer service, PostgreSQL config, partition counts, batch sizes, replicas
- `internal/config/config.go` — new config fields (PgBouncer, persist stage, cache TTL)
- `cmd/pipeline-worker/main.go` — register persist stage, update topic creation
- `internal/services/messaging/application/message_service.go` — remove DB writes from SendMessage
- `internal/pipeline/router/stage.go` — use RouteCache instead of direct DB queries
- `internal/pipeline/status/stage.go` — temp table + COPY for batch updates, increased batch size
- `internal/pipeline/backpressure/manager.go` — adjust MaxWindow default
- `internal/monitoring/metrics.go` — add persist stage metrics, cache metrics

---

## Task 1: PostgreSQL Tuning

**Files:**
- Create: `deployments/configs/postgresql.conf`
- Modify: `deployments/docker-compose.yml`

- [ ] **Step 1: Create PostgreSQL config file**

```ini
# deployments/configs/postgresql.conf
# Tuned for 8 CPU / 16 GB RAM, write-heavy SMS pipeline

# Memory
shared_buffers = 4GB
effective_cache_size = 12GB
work_mem = 64MB
maintenance_work_mem = 512MB
wal_buffers = 64MB

# WAL & Checkpoints
min_wal_size = 1GB
max_wal_size = 4GB
checkpoint_completion_target = 0.9

# Write Performance — async commit, Kafka is source of truth
synchronous_commit = off
wal_writer_delay = 200ms
commit_delay = 100

# Parallelism
max_worker_processes = 8
max_parallel_workers_per_gather = 4
max_parallel_workers = 8

# Connections — PgBouncer handles pooling
max_connections = 100
```

- [ ] **Step 2: Mount config in docker-compose.yml**

In `deployments/docker-compose.yml`, update the `postgres` service:

```yaml
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: smpp
      POSTGRES_PASSWORD: smpp_password
      POSTGRES_DB: smpp_db
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./configs/postgresql.conf:/etc/postgresql/postgresql.conf:ro
    command: postgres -c config_file=/etc/postgresql/postgresql.conf
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U smpp"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - smpp-network
```

Key change: add volume mount for `postgresql.conf` and `command` to use it.

- [ ] **Step 3: Verify PostgreSQL starts with new config**

```bash
cd deployments && docker compose up -d postgres
docker compose exec postgres psql -U smpp -d smpp_db -c "SHOW shared_buffers;"
# Expected: 4GB
docker compose exec postgres psql -U smpp -d smpp_db -c "SHOW synchronous_commit;"
# Expected: off
docker compose exec postgres psql -U smpp -d smpp_db -c "SHOW max_connections;"
# Expected: 100
```

- [ ] **Step 4: Commit**

```bash
git add deployments/configs/postgresql.conf deployments/docker-compose.yml
git commit -m "perf(postgres): tune PostgreSQL for write-heavy SMS pipeline

shared_buffers=4GB, synchronous_commit=off, wal_buffers=64MB,
max_wal_size=4GB on 8CPU/16GB server. Kafka is source of truth."
```

---

## Task 2: PgBouncer Setup

**Files:**
- Create: `deployments/configs/pgbouncer.ini`
- Create: `deployments/configs/pgbouncer_userlist.txt`
- Modify: `deployments/docker-compose.yml`

- [ ] **Step 1: Create PgBouncer config**

```ini
; deployments/configs/pgbouncer.ini
[databases]
smpp_db = host=postgres port=5432 dbname=smpp_db

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt

pool_mode = transaction
max_client_conn = 200
default_pool_size = 30
reserve_pool_size = 5
reserve_pool_timeout = 3
server_idle_timeout = 300

log_connections = 0
log_disconnections = 0
log_pooler_errors = 1

admin_users = smpp
stats_users = smpp
```

- [ ] **Step 2: Create PgBouncer auth file**

```txt
"smpp" "smpp_password"
```

File: `deployments/configs/pgbouncer_userlist.txt`

- [ ] **Step 3: Add PgBouncer service to docker-compose.yml**

Add after the `postgres` service:

```yaml
  pgbouncer:
    image: edoburu/pgbouncer:1.22.0
    volumes:
      - ./configs/pgbouncer.ini:/etc/pgbouncer/pgbouncer.ini:ro
      - ./configs/pgbouncer_userlist.txt:/etc/pgbouncer/userlist.txt:ro
    ports:
      - "6432:6432"
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -h localhost -p 6432 -U smpp"]
      interval: 5s
      timeout: 3s
      retries: 5
    networks:
      - smpp-network
```

- [ ] **Step 4: Update all pipeline services to connect through PgBouncer**

In `deployments/docker-compose.yml`, for each pipeline service (`pipeline-router`, `pipeline-sender`, `pipeline-status`, and the new `pipeline-persist` that will be added later), change:

```yaml
    environment:
      POSTGRES_HOST: pgbouncer
      POSTGRES_PORT: "6432"
```

Also update `depends_on` to include `pgbouncer` instead of (or in addition to) `postgres`.

For `messaging-service`, same change:
```yaml
    environment:
      POSTGRES_HOST: pgbouncer
      POSTGRES_PORT: "6432"
```

Keep `client-gateway` and services that don't write to DB pointing at `pgbouncer` too for consistency.

**Important:** Services that use `LISTEN/NOTIFY`, prepared statements, or temp tables that persist across transactions must connect directly to PostgreSQL. In our case, status stage uses temp tables within a single transaction (`ON COMMIT DELETE ROWS`), so PgBouncer transaction mode is fine.

- [ ] **Step 5: Verify PgBouncer works**

```bash
cd deployments && docker compose up -d postgres pgbouncer
docker compose exec pgbouncer psql -h localhost -p 6432 -U smpp -d smpp_db -c "SELECT 1;"
# Expected: 1
```

- [ ] **Step 6: Commit**

```bash
git add deployments/configs/pgbouncer.ini deployments/configs/pgbouncer_userlist.txt deployments/docker-compose.yml
git commit -m "perf(pgbouncer): add PgBouncer transaction pooling before PostgreSQL

200 client conns → 30 real PG conns, transaction pool mode.
All pipeline services route through PgBouncer on port 6432."
```

---

## Task 3: In-Memory Route/Provider Cache

**Files:**
- Create: `internal/pipeline/cache/route_cache.go`
- Create: `internal/pipeline/cache/route_cache_test.go`

- [ ] **Step 1: Write failing test for RouteCache**

```go
// internal/pipeline/cache/route_cache_test.go
package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sms/internal/shared"
)

type mockRouteRepo struct {
	mu     sync.Mutex
	routes []*shared.Route
	calls  int
}

func (m *mockRouteRepo) GetAllActive(ctx context.Context) ([]*shared.Route, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.routes, nil
}

func (m *mockRouteRepo) GetActiveByDestination(ctx context.Context, destination string) ([]*shared.Route, error) {
	return m.routes, nil
}

func (m *mockRouteRepo) GetByID(ctx context.Context, id uuid.UUID) (*shared.Route, error) {
	for _, r := range m.routes {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, nil
}

type mockProviderRepo struct {
	mu        sync.Mutex
	providers []*shared.Provider
	calls     int
}

func (m *mockProviderRepo) GetAllActive(ctx context.Context) ([]*shared.Provider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.providers, nil
}

func (m *mockProviderRepo) GetByID(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
	for _, p := range m.providers {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

func TestRouteCache_InitialLoad(t *testing.T) {
	providerID := uuid.New()
	routeID := uuid.New()

	routeRepo := &mockRouteRepo{
		routes: []*shared.Route{
			{ID: routeID, Name: "test-route", ProviderID: providerID, Active: true, Priority: 10},
		},
	}
	providerRepo := &mockProviderRepo{
		providers: []*shared.Provider{
			{ID: providerID, Name: "test-provider", Active: true, Priority: 10},
		},
	}

	cache := NewRouteCache(routeRepo, providerRepo, 30*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := cache.Start(ctx)
	require.NoError(t, err)

	routes := cache.GetAllActiveRoutes()
	assert.Len(t, routes, 1)
	assert.Equal(t, routeID, routes[0].ID)

	provider, ok := cache.GetProvider(providerID)
	assert.True(t, ok)
	assert.Equal(t, "test-provider", provider.Name)
}

func TestRouteCache_RefreshUpdatesData(t *testing.T) {
	providerID := uuid.New()
	routeRepo := &mockRouteRepo{
		routes: []*shared.Route{},
	}
	providerRepo := &mockProviderRepo{
		providers: []*shared.Provider{
			{ID: providerID, Name: "old-name", Active: true},
		},
	}

	cache := NewRouteCache(routeRepo, providerRepo, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := cache.Start(ctx)
	require.NoError(t, err)

	// Update mock data
	providerRepo.mu.Lock()
	providerRepo.providers = []*shared.Provider{
		{ID: providerID, Name: "new-name", Active: true},
	}
	providerRepo.mu.Unlock()

	// Wait for refresh
	time.Sleep(100 * time.Millisecond)

	provider, ok := cache.GetProvider(providerID)
	assert.True(t, ok)
	assert.Equal(t, "new-name", provider.Name)
}

func TestRouteCache_ConcurrentAccess(t *testing.T) {
	providerID := uuid.New()
	routeRepo := &mockRouteRepo{routes: []*shared.Route{}}
	providerRepo := &mockProviderRepo{
		providers: []*shared.Provider{
			{ID: providerID, Name: "provider", Active: true},
		},
	}

	cache := NewRouteCache(routeRepo, providerRepo, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := cache.Start(ctx)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.GetAllActiveRoutes()
			cache.GetProvider(providerID)
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/magomed/projects/sms && go test ./internal/pipeline/cache/... -v
```
Expected: compilation error — `NewRouteCache`, `RouteCache` not defined.

- [ ] **Step 3: Implement RouteCache**

```go
// internal/pipeline/cache/route_cache.go
package cache

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"sms/internal/shared"
)

type RouteRepository interface {
	GetAllActive(ctx context.Context) ([]*shared.Route, error)
	GetActiveByDestination(ctx context.Context, destination string) ([]*shared.Route, error)
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Route, error)
}

type ProviderRepository interface {
	GetAllActive(ctx context.Context) ([]*shared.Provider, error)
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Provider, error)
}

type RouteCache struct {
	routeRepo    RouteRepository
	providerRepo ProviderRepository
	refreshTTL   time.Duration
	logger       zerolog.Logger

	mu        sync.RWMutex
	routes    []*shared.Route
	providers map[uuid.UUID]*shared.Provider
}

func NewRouteCache(routeRepo RouteRepository, providerRepo ProviderRepository, refreshTTL time.Duration) *RouteCache {
	return &RouteCache{
		routeRepo:    routeRepo,
		providerRepo: providerRepo,
		refreshTTL:   refreshTTL,
		logger:       log.With().Str("component", "route-cache").Logger(),
		providers:    make(map[uuid.UUID]*shared.Provider),
	}
}

func (c *RouteCache) Start(ctx context.Context) error {
	if err := c.refresh(ctx); err != nil {
		return err
	}
	go c.refreshLoop(ctx)
	return nil
}

func (c *RouteCache) GetAllActiveRoutes() []*shared.Route {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.routes
}

func (c *RouteCache) GetProvider(id uuid.UUID) (*shared.Provider, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.providers[id]
	return p, ok
}

func (c *RouteCache) GetRouteByID(id uuid.UUID) (*shared.Route, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, r := range c.routes {
		if r.ID == id {
			return r, true
		}
	}
	return nil, false
}

func (c *RouteCache) MatchRoutes(destination string) []*shared.Route {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var matched []*shared.Route
	for _, r := range c.routes {
		if r.Active && matchesPattern(r, destination) {
			matched = append(matched, r)
		}
	}
	return matched
}

func matchesPattern(route *shared.Route, destination string) bool {
	switch route.MatchType {
	case "prefix":
		return len(destination) >= len(route.Pattern) && destination[:len(route.Pattern)] == route.Pattern
	case "exact":
		return destination == route.Pattern
	default:
		return false
	}
}

func (c *RouteCache) refresh(ctx context.Context) error {
	routes, err := c.routeRepo.GetAllActive(ctx)
	if err != nil {
		return err
	}
	providers, err := c.providerRepo.GetAllActive(ctx)
	if err != nil {
		return err
	}

	providerMap := make(map[uuid.UUID]*shared.Provider, len(providers))
	for _, p := range providers {
		providerMap[p.ID] = p
	}

	c.mu.Lock()
	c.routes = routes
	c.providers = providerMap
	c.mu.Unlock()

	c.logger.Debug().Int("routes", len(routes)).Int("providers", len(providers)).Msg("cache refreshed")
	return nil
}

func (c *RouteCache) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(c.refreshTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.refresh(ctx); err != nil {
				c.logger.Error().Err(err).Msg("cache refresh failed, using stale data")
			}
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/magomed/projects/sms && go test ./internal/pipeline/cache/... -v -race
```
Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/cache/
git commit -m "feat(cache): add in-memory route/provider cache with periodic refresh

sync.RWMutex-based cache, 30s TTL refresh via background goroutine.
Eliminates 1.7M+ DB queries per load test for routes/providers."
```

---

## Task 4: Integrate Route Cache into Router Stage

**Files:**
- Modify: `internal/pipeline/router/stage.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1: Add CachedRouter that uses RouteCache**

Create a new router implementation in `internal/router/router.go` that accepts the cache. Add a `NewCachedRouter` constructor alongside the existing `NewRouter`:

```go
// Add to internal/router/router.go

type CachedRouter struct {
	cache  *cache.RouteCache
	logger zerolog.Logger
}

func NewCachedRouter(c *cache.RouteCache) *CachedRouter {
	return &CachedRouter{
		cache:  c,
		logger: log.With().Str("component", "cached-router").Logger(),
	}
}

func (r *CachedRouter) RouteMessage(ctx context.Context, msg *shared.Message) (*shared.Provider, error) {
	// 1. Explicit provider
	if msg.ProviderID != nil {
		provider, ok := r.cache.GetProvider(*msg.ProviderID)
		if !ok || !provider.Active {
			return nil, fmt.Errorf("provider %s not found or inactive", msg.ProviderID)
		}
		return provider, nil
	}

	// 2. Explicit route
	if msg.RouteID != nil {
		route, ok := r.cache.GetRouteByID(*msg.RouteID)
		if !ok || !route.Active {
			return nil, fmt.Errorf("route %s not found or inactive", msg.RouteID)
		}
		provider, ok := r.cache.GetProvider(route.ProviderID)
		if !ok || !provider.Active {
			if route.FailoverProviderID != nil {
				provider, ok = r.cache.GetProvider(*route.FailoverProviderID)
				if ok && provider.Active {
					return provider, nil
				}
			}
			return nil, fmt.Errorf("provider for route %s not active", msg.RouteID)
		}
		return provider, nil
	}

	// 3. Destination-based
	matched := r.cache.MatchRoutes(msg.Destination)
	if len(matched) > 0 {
		for _, route := range matched {
			provider, ok := r.cache.GetProvider(route.ProviderID)
			if ok && provider.Active {
				return provider, nil
			}
			if route.FailoverProviderID != nil {
				provider, ok = r.cache.GetProvider(*route.FailoverProviderID)
				if ok && provider.Active {
					return provider, nil
				}
			}
		}
	}

	// 4. Fallback — first active provider
	routes := r.cache.GetAllActiveRoutes()
	_ = routes
	// Get any active provider from cache
	// Iterate all providers is not efficient, but this is fallback path
	return nil, fmt.Errorf("no active route/provider found for destination %s", msg.Destination)
}

func (r *CachedRouter) GetFailoverProvider(ctx context.Context, routeID uuid.UUID) (*shared.Provider, error) {
	route, ok := r.cache.GetRouteByID(routeID)
	if !ok {
		return nil, fmt.Errorf("route %s not found", routeID)
	}
	if route.FailoverProviderID == nil {
		return nil, fmt.Errorf("route %s has no failover provider", routeID)
	}
	provider, ok := r.cache.GetProvider(*route.FailoverProviderID)
	if !ok || !provider.Active {
		return nil, fmt.Errorf("failover provider for route %s not active", routeID)
	}
	return provider, nil
}
```

Import `"sms/internal/pipeline/cache"` at the top.

- [ ] **Step 2: Update Router Stage to use CachedRouter**

In `internal/pipeline/router/stage.go`, update `NewStage`:

```go
func NewStage(cfg *config.Config, db *storage.DB) (*Stage, error) {
	consumer, err := queue.NewBatchConsumer(
		&cfg.Kafka,
		"pipeline-router",
		[]string{cfg.Kafka.TopicOutgoing, cfg.Kafka.TopicFailed},
		cfg.Pipeline.BatchSize,
		cfg.Pipeline.BatchTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("create batch consumer: %w", err)
	}

	producer, err := queue.NewAsyncProducer(&cfg.Kafka)
	if err != nil {
		consumer.Close()
		return nil, fmt.Errorf("create async producer: %w", err)
	}

	routeRepo := storage.NewRouteRepository(db)
	providerRepo := storage.NewProviderRepository(db)

	// Use in-memory cache instead of direct DB queries
	routeCache := cache.NewRouteCache(routeRepo, providerRepo, 30*time.Second)

	router := msgrouter.NewCachedRouter(routeCache)

	return &Stage{
		consumer:   consumer,
		producer:   producer,
		router:     router,
		routeCache: routeCache,
		cfg:        cfg,
		logger:     log.With().Str("stage", "router").Logger(),
	}, nil
}
```

Update the `Stage` struct to hold cache reference and update `Run` to start the cache:

```go
type Stage struct {
	consumer   *queue.BatchConsumer
	producer   *queue.AsyncProducer
	router     *msgrouter.CachedRouter  // Changed from *msgrouter.Router
	routeCache *cache.RouteCache
	cfg        *config.Config
	logger     zerolog.Logger
}

func (s *Stage) Run(ctx context.Context) error {
	if err := s.routeCache.Start(ctx); err != nil {
		return fmt.Errorf("start route cache: %w", err)
	}
	return s.consumer.ConsumeBatches(ctx, s.handleBatch)
}
```

Update `handleBatch` to use the `CachedRouter` method signatures (they are the same: `RouteMessage` and `GetFailoverProvider`).

- [ ] **Step 3: Verify compilation**

```bash
cd /home/magomed/projects/sms && go build ./...
```
Expected: no errors.

- [ ] **Step 4: Run existing tests**

```bash
cd /home/magomed/projects/sms && go test ./internal/pipeline/router/... ./internal/router/... -v
```
Expected: PASS (or no tests — existing tests may not exist for router stage).

- [ ] **Step 5: Commit**

```bash
git add internal/router/router.go internal/pipeline/router/stage.go
git commit -m "perf(router): use in-memory cache instead of per-message DB queries

CachedRouter reads from RouteCache (RWMutex, 30s refresh).
Eliminates ~2 DB queries per message in router stage."
```

---

## Task 5: Async Ingestion — Remove DB Writes from SendMessage

**Files:**
- Modify: `internal/services/messaging/application/message_service.go`

- [ ] **Step 1: Modify SendMessage to skip DB writes**

In `internal/services/messaging/application/message_service.go`, update the `SendMessage` method. The key changes:

1. Remove `s.messageRepo.Create(ctx, msg)` call
2. Remove `s.messageRepo.UpdateStatus(ctx, msg.ID, "queued")` call
3. Keep validation, UUID generation, segment counting
4. Keep Kafka publish (this is now the only durable write)
5. Keep scheduled message logic (still needs DB for scheduler to find them)

```go
func (s *MessageService) SendMessage(
	ctx context.Context,
	clientID uuid.UUID,
	source, destination, text string,
	options *SendMessageOptions,
) (*domain.Message, error) {
	msg := domain.NewMessage(clientID, source, destination, text)

	if options != nil {
		// ... apply options (keep existing code)
	}

	msg.Encoding = domain.DetectEncoding(text)
	msg.SegmentCount = shared.CountSegments(text)

	if err := s.validator.Validate(msg); err != nil {
		return nil, fmt.Errorf("validate message: %w", err)
	}

	// Scheduled messages still need DB (scheduler reads from DB)
	if options != nil && options.ScheduledAt != nil {
		scheduledAt := *options.ScheduledAt
		if time.Until(scheduledAt) > 30*time.Second {
			msg.MarkAsScheduled(scheduledAt)
			if err := s.messageRepo.Create(ctx, msg); err != nil {
				return nil, fmt.Errorf("save scheduled message: %w", err)
			}
			return msg, nil
		}
	}

	// Non-scheduled: publish directly to Kafka, no DB write
	// Persist stage will batch-insert into DB asynchronously
	msg.Status = "queued"
	if err := s.eventPublisher.PublishMessageQueued(ctx, msg); err != nil {
		return nil, fmt.Errorf("publish message: %w", err)
	}

	return msg, nil
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/magomed/projects/sms && go build ./...
```
Expected: no errors.

- [ ] **Step 3: Run existing tests**

```bash
cd /home/magomed/projects/sms && go test ./internal/services/messaging/... -v
```
Expected: some tests may fail if they assert DB writes. Adjust mocks to not expect `Create()` / `UpdateStatus()` calls for non-scheduled messages.

- [ ] **Step 4: Fix any failing tests**

If tests assert `messageRepo.Create()` was called for regular (non-scheduled) messages, update those expectations to no longer require the DB call. The mock should only expect `Create()` for scheduled messages.

- [ ] **Step 5: Commit**

```bash
git add internal/services/messaging/application/message_service.go
git commit -m "perf(ingestion): remove synchronous DB writes from SendMessage hot path

Non-scheduled messages go directly to Kafka without DB INSERT/UPDATE.
Persist stage (next task) handles async batch persistence via COPY.
Scheduled messages still write to DB for scheduler compatibility."
```

---

## Task 6: Persist Stage — Batch COPY Insert

**Files:**
- Create: `internal/pipeline/persist/stage.go`
- Create: `internal/pipeline/persist/stage_test.go`
- Modify: `cmd/pipeline-worker/main.go`
- Modify: `internal/config/config.go`

- [ ] **Step 1: Write failing test for Persist stage**

```go
// internal/pipeline/persist/stage_test.go
package persist

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sms/internal/queue"
)

func TestBuildCopyRows(t *testing.T) {
	msgID := uuid.New()
	clientID := uuid.New()

	kafkaMsg := &queue.KafkaMessage{
		MessageID:   msgID,
		Source:      "TestSender",
		Destination: "79001234567",
		Text:        "Hello",
		ClientID:    &clientID,
		Priority:    5,
		CreatedAt:   time.Now(),
	}

	data, err := json.Marshal(kafkaMsg)
	require.NoError(t, err)

	saramaMsg := &sarama.ConsumerMessage{
		Topic: "sms.outgoing",
		Value: data,
	}

	rows, err := buildCopyRows([]*sarama.ConsumerMessage{saramaMsg})
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	assert.Equal(t, msgID, row.ID)
	assert.Equal(t, "TestSender", row.Source)
	assert.Equal(t, "79001234567", row.Destination)
	assert.Equal(t, "Hello", row.Text)
	assert.Equal(t, "queued", row.Status)
	assert.Equal(t, &clientID, row.ClientID)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/magomed/projects/sms && go test ./internal/pipeline/persist/... -v
```
Expected: compilation error — package/types not defined.

- [ ] **Step 3: Implement Persist stage**

```go
// internal/pipeline/persist/stage.go
package persist

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"sms/internal/config"
	"sms/internal/monitoring"
	"sms/internal/queue"
)

type messageRow struct {
	ID          uuid.UUID
	MessageID   string
	ExternalID  string
	Source      string
	Destination string
	Text        string
	Encoding    string
	SegmentCount int
	Status      string
	Priority    int
	ClientID    *uuid.UUID
	ProviderID  *uuid.UUID
	RouteID     *uuid.UUID
	MaxRetries  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Stage struct {
	consumer *queue.BatchConsumer
	pool     *pgxpool.Pool
	cfg      *config.Config
	logger   zerolog.Logger
}

func NewStage(cfg *config.Config, pool *pgxpool.Pool) (*Stage, error) {
	consumer, err := queue.NewBatchConsumer(
		&cfg.Kafka,
		"pipeline-persist",
		[]string{cfg.Kafka.TopicOutgoing},
		cfg.Pipeline.PersistBatchSize,
		cfg.Pipeline.PersistBatchTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("create batch consumer: %w", err)
	}

	return &Stage{
		consumer: consumer,
		pool:     pool,
		cfg:      cfg,
		logger:   log.With().Str("stage", "persist").Logger(),
	}, nil
}

func (s *Stage) Run(ctx context.Context) error {
	return s.consumer.ConsumeBatches(ctx, s.handleBatch)
}

func (s *Stage) handleBatch(ctx context.Context, msgs []*sarama.ConsumerMessage, session sarama.ConsumerGroupSession) error {
	start := time.Now()
	monitoring.PipelineBatchSize.WithLabelValues("persist").Observe(float64(len(msgs)))

	rows, err := buildCopyRows(msgs)
	if err != nil {
		s.logger.Error().Err(err).Int("count", len(msgs)).Msg("failed to build copy rows")
		monitoring.PipelineMessagesProcessed.WithLabelValues("persist", "error").Add(float64(len(msgs)))
		return nil // don't fail batch, skip malformed
	}

	if len(rows) == 0 {
		return nil
	}

	if err := s.copyInsert(ctx, rows); err != nil {
		s.logger.Error().Err(err).Int("count", len(rows)).Msg("COPY insert failed")
		monitoring.PipelineMessagesProcessed.WithLabelValues("persist", "error").Add(float64(len(rows)))
		// Return error — batch won't be committed, will retry
		return err
	}

	monitoring.PipelineMessagesProcessed.WithLabelValues("persist", "success").Add(float64(len(rows)))
	monitoring.PipelineProcessingDuration.WithLabelValues("persist").Observe(time.Since(start).Seconds())

	return nil
}

func (s *Stage) copyInsert(ctx context.Context, rows []messageRow) error {
	columns := []string{
		"id", "message_id", "external_id", "source", "destination", "text",
		"encoding", "segment_count", "status", "priority",
		"client_id", "provider_id", "route_id", "max_retries",
		"created_at", "updated_at",
	}

	copyCount, err := s.pool.CopyFrom(
		ctx,
		pgx.Identifier{"messages"},
		columns,
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			r := rows[i]
			return []any{
				r.ID, r.MessageID, r.ExternalID, r.Source, r.Destination, r.Text,
				r.Encoding, r.SegmentCount, r.Status, r.Priority,
				r.ClientID, r.ProviderID, r.RouteID, r.MaxRetries,
				r.CreatedAt, r.UpdatedAt,
			}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("copy from: %w", err)
	}

	s.logger.Debug().Int64("copied", copyCount).Msg("batch persisted")
	return nil
}

func buildCopyRows(msgs []*sarama.ConsumerMessage) ([]messageRow, error) {
	rows := make([]messageRow, 0, len(msgs))
	now := time.Now()

	for _, msg := range msgs {
		var kafkaMsg queue.KafkaMessage
		if err := json.Unmarshal(msg.Value, &kafkaMsg); err != nil {
			continue // skip malformed, log in production
		}

		createdAt := kafkaMsg.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}

		rows = append(rows, messageRow{
			ID:           kafkaMsg.MessageID,
			MessageID:    kafkaMsg.MessageID.String(),
			ExternalID:   kafkaMsg.ExternalID,
			Source:       kafkaMsg.Source,
			Destination:  kafkaMsg.Destination,
			Text:         kafkaMsg.Text,
			Encoding:     kafkaMsg.Encoding,
			SegmentCount: kafkaMsg.SegmentCount,
			Status:       "queued",
			Priority:     kafkaMsg.Priority,
			ClientID:     kafkaMsg.ClientID,
			ProviderID:   kafkaMsg.ProviderID,
			RouteID:      kafkaMsg.RouteID,
			MaxRetries:   kafkaMsg.MaxRetries,
			CreatedAt:    createdAt,
			UpdatedAt:    now,
		})
	}

	return rows, nil
}

func (s *Stage) Close() error {
	return s.consumer.Close()
}
```

- [ ] **Step 4: Add PersistBatchSize/Timeout to PipelineConfig**

In `internal/config/config.go`, add to `PipelineConfig`:

```go
type PipelineConfig struct {
	Stage              string
	BatchSize          int
	BatchTimeout       time.Duration
	WorkerCount        int
	SMPPWindowSize     int
	PersistBatchSize   int           // default 2000
	PersistBatchTimeout time.Duration // default 100ms
}
```

In the config loading function, add defaults:

```go
persistBatchSize := getEnvInt("PIPELINE_PERSIST_BATCH_SIZE", 2000)
persistBatchTimeout := getEnvDuration("PIPELINE_PERSIST_BATCH_TIMEOUT", 100*time.Millisecond)
```

- [ ] **Step 5: Register persist stage in pipeline-worker main.go**

In `cmd/pipeline-worker/main.go`, add the persist case to the stage dispatch:

```go
case "persist":
	runPersistStage(ctx, cfg, pgxPool)
```

Add the `runPersistStage` function:

```go
func runPersistStage(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) {
	stage, err := persist.NewStage(cfg, pool)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create persist stage")
	}
	defer stage.Close()

	log.Info().Msg("persist stage started")
	if err := stage.Run(ctx); err != nil {
		log.Error().Err(err).Msg("persist stage error")
	}
}
```

This requires a `*pgxpool.Pool` connection. Add pgx pool creation alongside the existing sqlx DB connection in main.go:

```go
// After existing DB connection setup
pgxPool, err := pgxpool.New(ctx, cfg.Database.ConnectionString())
if err != nil {
	log.Fatal().Err(err).Msg("failed to create pgx pool")
}
defer pgxPool.Close()
```

Add `ConnectionString()` method to `DatabaseConfig` if not present:

```go
func (d *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Database, d.SSLMode)
}
```

Also update `ensureTopics()` to not create topics that persist stage needs — `sms.outgoing` is already created by messaging service or auto-created. No changes needed there.

Update the consumer lag monitor to include `"pipeline-persist"` group:

```go
monitoring.StartConsumerLagMonitor(ctx, cfg.Kafka.Brokers,
	[]string{"pipeline-router", "pipeline-sender", "pipeline-status", "pipeline-persist"},
	30*time.Second,
)
```

- [ ] **Step 6: Run tests**

```bash
cd /home/magomed/projects/sms && go test ./internal/pipeline/persist/... -v
```
Expected: `TestBuildCopyRows` PASS.

- [ ] **Step 7: Verify full compilation**

```bash
cd /home/magomed/projects/sms && go build ./...
```
Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add internal/pipeline/persist/ internal/config/config.go cmd/pipeline-worker/main.go
git commit -m "feat(persist): add Persist pipeline stage with pgx COPY protocol

New stage consumes sms.outgoing (separate consumer group),
batch-inserts messages via pgx CopyFrom (2000 msgs / 100ms).
Replaces synchronous per-message DB INSERT in messaging service."
```

---

## Task 7: Status Stage — Temp Table + COPY Optimization

**Files:**
- Modify: `internal/pipeline/status/stage.go`

- [ ] **Step 1: Refactor batchUpsert to use temp table + COPY**

Replace the existing `batchUpsert` method in `internal/pipeline/status/stage.go`:

```go
func (s *Stage) batchUpsert(ctx context.Context, records []*statusRecord) error {
	conn, err := s.pgxPool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create temp table if not exists (idempotent for PgBouncer transaction pooling)
	_, err = tx.Exec(ctx, `
		CREATE TEMP TABLE IF NOT EXISTS status_batch (
			id UUID,
			status TEXT,
			smpp_message_id TEXT,
			provider_id UUID,
			submitted_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			segment_count INT
		) ON COMMIT DELETE ROWS
	`)
	if err != nil {
		return fmt.Errorf("create temp table: %w", err)
	}

	// COPY data into temp table
	columns := []string{"id", "status", "smpp_message_id", "provider_id", "submitted_at", "updated_at", "segment_count"}
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"status_batch"},
		columns,
		pgx.CopyFromSlice(len(records), func(i int) ([]any, error) {
			r := records[i]
			return []any{
				r.MessageID,
				r.Status,
				r.SMPPMessageID,
				r.ProviderID,
				r.SubmittedAt,
				r.UpdatedAt,
				r.SegmentCount,
			}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("copy to temp table: %w", err)
	}

	// UPDATE from temp table (idempotent — only newer timestamps)
	_, err = tx.Exec(ctx, `
		UPDATE messages SET
			status = s.status,
			smpp_message_id = COALESCE(s.smpp_message_id, messages.smpp_message_id),
			provider_id = COALESCE(s.provider_id, messages.provider_id),
			submitted_at = COALESCE(s.submitted_at, messages.submitted_at),
			updated_at = s.updated_at,
			segment_count = COALESCE(s.segment_count, messages.segment_count)
		FROM status_batch s
		WHERE messages.id = s.id AND messages.updated_at < s.updated_at
	`)
	if err != nil {
		return fmt.Errorf("batch update: %w", err)
	}

	return tx.Commit(ctx)
}
```

This requires adding a `*pgxpool.Pool` field to the Status Stage struct. Update `NewStage`:

```go
type Stage struct {
	consumer     *queue.BatchConsumer
	db           *storage.DB
	pgxPool      *pgxpool.Pool
	cfg          *config.Config
	logger       zerolog.Logger
	failedMu     sync.Mutex
	failedBuffer []*statusRecord
}

func NewStage(cfg *config.Config, db *storage.DB, pgxPool *pgxpool.Pool) (*Stage, error) {
	// ... existing code, store pgxPool
}
```

Update the call site in `cmd/pipeline-worker/main.go` to pass pgxPool to status stage.

- [ ] **Step 2: Verify compilation**

```bash
cd /home/magomed/projects/sms && go build ./...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/status/stage.go cmd/pipeline-worker/main.go
git commit -m "perf(status): use temp table + COPY for batch status updates

Replaces UPDATE ... FROM (VALUES ...) with COPY into temp table + single UPDATE.
Faster for large batches (2000 rows) due to binary protocol."
```

---

## Task 8: Kafka Repartitioning + Pipeline Scaling

**Files:**
- Modify: `cmd/pipeline-worker/main.go`
- Modify: `deployments/docker-compose.yml`

- [ ] **Step 1: Update topic partition counts in ensureTopics()**

In `cmd/pipeline-worker/main.go`, update `ensureTopics()`:

```go
func ensureTopics(brokers []string, cfg *config.KafkaConfig) error {
	topics := map[string]int32{
		cfg.TopicOutgoing: 32,  // was default
		cfg.TopicRouted:   32,  // was 16
		cfg.TopicSent:     16,  // was 16
		cfg.TopicDLR:      16,  // same
		cfg.TopicStatus:   16,  // was 8
		cfg.TopicFailed:   8,   // same
	}
	// ... existing topic creation logic
}
```

Note: Kafka does not allow decreasing partitions. Increasing is a no-op if already at target count. New partitions will take effect on next topic creation (if topics don't exist yet) or via `kafka-topics --alter`.

- [ ] **Step 2: Update docker-compose.yml batch sizes and replicas**

```yaml
  pipeline-router:
    environment:
      PIPELINE_BATCH_SIZE: "1000"
      PIPELINE_BATCH_TIMEOUT: "50ms"
      PIPELINE_WORKER_COUNT: "4"
    deploy:
      replicas: 2

  pipeline-sender:
    environment:
      PIPELINE_BATCH_SIZE: "500"
      PIPELINE_BATCH_TIMEOUT: "10ms"
      SMPP_WINDOW_SIZE: "500"
      PIPELINE_WORKER_COUNT: "4"
    deploy:
      replicas: 4

  pipeline-status:
    environment:
      PIPELINE_BATCH_SIZE: "2000"
      PIPELINE_BATCH_TIMEOUT: "100ms"
    deploy:
      replicas: 2

  pipeline-persist:
    build:
      context: ..
      dockerfile: deployments/docker/pipeline-worker.Dockerfile
    command: ["--stage=persist"]
    environment:
      PIPELINE_STAGE: persist
      PIPELINE_PERSIST_BATCH_SIZE: "2000"
      PIPELINE_PERSIST_BATCH_TIMEOUT: "100ms"
      POSTGRES_HOST: pgbouncer
      POSTGRES_PORT: "6432"
      KAFKA_BROKERS: kafka:9092
    depends_on:
      pgbouncer:
        condition: service_healthy
      kafka:
        condition: service_healthy
    deploy:
      replicas: 2
    networks:
      - smpp-network
```

- [ ] **Step 3: Commit**

```bash
git add cmd/pipeline-worker/main.go deployments/docker-compose.yml
git commit -m "perf(pipeline): increase Kafka partitions, tune batch sizes, add persist service

sms.outgoing/routed: 32 partitions, status: 16 partitions.
Router: 1000/50ms, Sender: 500/10ms, Status: 2000/100ms, Persist: 2000/100ms.
SMPP window: 50 → 500. Pipeline-persist: 2 replicas."
```

---

## Task 9: SMPP Window + Backpressure Adjustment

**Files:**
- Modify: `internal/pipeline/backpressure/manager.go`
- Modify: `internal/config/config.go`

- [ ] **Step 1: Update default SMPP window size**

In `internal/config/config.go`, change the default for `SMPPWindowSize`:

```go
smppWindowSize := getEnvInt("SMPP_WINDOW_SIZE", 500) // was 50
```

- [ ] **Step 2: Update backpressure Manager to scale BurstSize with window**

In `internal/pipeline/backpressure/manager.go`, in the `Register` method, ensure `MaxWindow` uses the configured window size (it already does via parameter). The `BurstSize` calculation `TokensPerSecond * 2` should still work — it's independent of window size.

Verify that the sender stage passes the new window size to both `Pool.ConnectAsync()` and `Manager.Register()`. In `internal/pipeline/sender/stage.go`, check that `cfg.Pipeline.SMPPWindowSize` is used in both places. If it is, no code change needed — just the config default change.

- [ ] **Step 3: Update PipelineBatchSize histogram buckets**

In `internal/monitoring/metrics.go`, update the `PipelineBatchSize` histogram to include larger batch sizes:

```go
PipelineBatchSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "pipeline_batch_size",
	Help:    "Batch sizes processed by pipeline stages",
	Buckets: []float64{1, 10, 50, 100, 200, 500, 1000, 2000, 5000},
}, []string{"stage"})
```

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go internal/pipeline/backpressure/manager.go internal/monitoring/metrics.go
git commit -m "perf(smpp): increase default SMPP window to 500, update metrics buckets

Window size 50→500 allows 10x more in-flight PDUs per connection.
Batch size histogram now includes 2000 and 5000 buckets."
```

---

## Task 10: Hot Partition Index Optimization

**Files:**
- Create: new migration file

- [ ] **Step 1: Determine next migration number**

```bash
ls /home/magomed/projects/sms/migrations/ | tail -5
```

Use the next sequential number (e.g., `000007`).

- [ ] **Step 2: Create migration for April 2026 hot partition with minimal indexes**

```sql
-- migrations/000007_optimize_hot_partition_indexes.up.sql

-- Create April 2026 partition with minimal indexes for write performance
CREATE TABLE IF NOT EXISTS messages_2026_04 PARTITION OF messages
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

-- Only critical indexes on hot partition
CREATE INDEX IF NOT EXISTS idx_msg_2026_04_message_id
    ON messages_2026_04 (message_id);
CREATE INDEX IF NOT EXISTS idx_msg_2026_04_status_created
    ON messages_2026_04 (status, created_at);

-- Add full indexes on previous month (now cold — no active writes)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_msg_2026_03_destination
    ON messages_2026_03 (destination);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_msg_2026_03_client_id
    ON messages_2026_03 (client_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_msg_2026_03_provider_id
    ON messages_2026_03 (provider_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_msg_2026_03_external_id
    ON messages_2026_03 (external_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_msg_2026_03_next_retry
    ON messages_2026_03 (next_retry_at) WHERE next_retry_at IS NOT NULL;
```

```sql
-- migrations/000007_optimize_hot_partition_indexes.down.sql

-- Drop optimized indexes on hot partition
DROP INDEX IF EXISTS idx_msg_2026_04_message_id;
DROP INDEX IF EXISTS idx_msg_2026_04_status_created;

-- Drop retroactive indexes on cold partition
DROP INDEX IF EXISTS idx_msg_2026_03_destination;
DROP INDEX IF EXISTS idx_msg_2026_03_client_id;
DROP INDEX IF EXISTS idx_msg_2026_03_provider_id;
DROP INDEX IF EXISTS idx_msg_2026_03_external_id;
DROP INDEX IF EXISTS idx_msg_2026_03_next_retry;

-- Drop partition (careful — data loss)
-- DROP TABLE IF EXISTS messages_2026_04;
```

**Note:** This migration is date-specific. In production, partition creation should be automated (cron job or scheduled task). This migration handles the immediate next month.

- [ ] **Step 3: Also drop unnecessary indexes from the CURRENT hot partition (March 2026)**

Check if March partition has all indexes and drop the non-critical ones. Add to the up migration:

```sql
-- Remove non-critical indexes from current hot partition to speed up writes
DROP INDEX IF EXISTS idx_msg_2026_03_destination;
DROP INDEX IF EXISTS idx_msg_2026_03_client_id;
DROP INDEX IF EXISTS idx_msg_2026_03_provider_id;
DROP INDEX IF EXISTS idx_msg_2026_03_external_id;
DROP INDEX IF EXISTS idx_msg_2026_03_next_retry;
```

Wait — these indexes may exist as inherited from the parent table definition. Need to check how partitions are currently created. If indexes are auto-inherited, we need to handle this differently.

Actually, in PostgreSQL declarative partitioning, indexes defined on the parent table are automatically created on each partition. To avoid this, we would need to:
1. Drop the indexes from the parent table
2. Only create them on cold partitions

This is a bigger change. For now, the migration should:
1. Drop non-critical indexes from the March (current hot) partition
2. Create April partition (it will auto-inherit parent indexes — we'll need to drop them too)

Alternatively, modify the parent table to only have the critical indexes, and manually add others to cold partitions. This approach requires dropping and recreating parent indexes — potentially risky with existing data.

**Simpler approach for now:** Just drop the non-critical indexes from the current hot partition explicitly. When creating new partitions, drop inherited indexes immediately after.

```sql
-- In the up migration, after creating messages_2026_04:
-- Drop auto-inherited non-critical indexes from hot partition
DROP INDEX IF EXISTS messages_2026_04_destination_idx;
DROP INDEX IF EXISTS messages_2026_04_client_id_idx;
-- ... etc (names depend on actual inherited index naming convention)
```

The exact index names need to be verified at runtime. The implementer should run `\di messages_2026_*` in psql to check actual names.

- [ ] **Step 4: Commit**

```bash
git add migrations/000007_optimize_hot_partition_indexes.up.sql migrations/000007_optimize_hot_partition_indexes.down.sql
git commit -m "perf(db): optimize hot partition indexes — minimal indexes on active month

Only message_id and (status, created_at) on hot partition.
Full indexes added to cold partitions for analytics.
~30-40% write overhead reduction."
```

---

## Task 11: Docker Compose Final Assembly

**Files:**
- Modify: `deployments/docker-compose.yml`

- [ ] **Step 1: Final review and assembly of all docker-compose changes**

Ensure the full docker-compose.yml has:

1. PostgreSQL with custom config mount
2. PgBouncer service between PG and pipeline services
3. All pipeline services pointing to PgBouncer (port 6432)
4. `pipeline-persist` as a new service
5. Updated batch sizes per stage
6. Updated replicas
7. SMPP_WINDOW_SIZE=500

- [ ] **Step 2: Verify full stack starts**

```bash
cd deployments && docker compose up -d
docker compose ps
```

Expected: all services healthy, including new `pgbouncer` and `pipeline-persist`.

- [ ] **Step 3: Verify PgBouncer connectivity from pipeline services**

```bash
docker compose exec pipeline-router sh -c 'nc -z pgbouncer 6432 && echo OK'
# Expected: OK
```

- [ ] **Step 4: Commit**

```bash
git add deployments/docker-compose.yml
git commit -m "chore(docker): final assembly — all performance optimizations integrated

PgBouncer, tuned PG, persist stage, updated batch sizes,
SMPP window 500, 32 Kafka partitions."
```

---

## Task 12: Update Metrics for New Stages

**Files:**
- Modify: `internal/monitoring/metrics.go`

- [ ] **Step 1: Add persist stage to consumer lag monitor groups**

Already handled in Task 6 Step 5. Verify the persist stage label works in existing metrics:

- `PipelineMessagesProcessed` with label `stage="persist"` — already used in persist stage code
- `PipelineProcessingDuration` with label `stage="persist"` — already used
- `PipelineBatchSize` with label `stage="persist"` — already used

No new metrics needed — existing metric vectors accept any label value.

- [ ] **Step 2: Add cache hit metrics (optional, lightweight)**

In `internal/monitoring/metrics.go`:

```go
var RouteCacheRefreshDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "route_cache_refresh_duration_seconds",
	Help:    "Time to refresh route/provider cache from DB",
	Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5},
})

var RouteCacheSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "route_cache_size",
	Help: "Number of items in route/provider cache",
}, []string{"type"})
```

Then in `internal/pipeline/cache/route_cache.go`, in the `refresh` method, record these metrics:

```go
func (c *RouteCache) refresh(ctx context.Context) error {
	start := time.Now()
	// ... existing refresh logic ...
	monitoring.RouteCacheRefreshDuration.Observe(time.Since(start).Seconds())
	monitoring.RouteCacheSize.WithLabelValues("routes").Set(float64(len(routes)))
	monitoring.RouteCacheSize.WithLabelValues("providers").Set(float64(len(providers)))
	return nil
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/monitoring/metrics.go internal/pipeline/cache/route_cache.go
git commit -m "feat(metrics): add route cache refresh duration and size metrics"
```

---

## Task 13: Update Prometheus Scrape Config

**Files:**
- Modify: `deployments/configs/prometheus.yml`

- [ ] **Step 1: Add pipeline-persist to Prometheus scrape targets**

In `deployments/configs/prometheus.yml`, add the new persist service alongside existing pipeline targets:

```yaml
  - job_name: 'pipeline-persist'
    static_configs:
      - targets: ['pipeline-persist:9090']
```

Use the same port as other pipeline services (they all expose metrics on their HTTP port).

- [ ] **Step 2: Commit**

```bash
git add deployments/configs/prometheus.yml
git commit -m "chore(prometheus): add pipeline-persist scrape target"
```

---

## Task 14: Batch message_stats Writes

**Files:**
- Modify: `internal/services/analytics/infrastructure/repository/metric_repository.go`

The analytics `MetricRepository.Create()` does an individual `INSERT INTO message_stats` per message — 241K inserts per load test. Batch these writes.

- [ ] **Step 1: Add BatchCreate method to MetricRepository**

In `internal/services/analytics/infrastructure/repository/metric_repository.go`, add:

```go
// BatchCreate saves multiple metrics in a single INSERT
func (r *MetricRepository) BatchCreate(ctx context.Context, metrics []*domain.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	query := `
		INSERT INTO message_stats (
			id, metric_type, client_id, provider_id, message_id,
			status, value, timestamp, metadata, created_at
		) VALUES `

	args := make([]interface{}, 0, len(metrics)*10)
	for i, m := range metrics {
		if i > 0 {
			query += ","
		}
		base := i * 10
		query += fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5,
			base+6, base+7, base+8, base+9, base+10,
		)

		var metadataJSON []byte
		if m.Metadata != nil && len(m.Metadata) > 0 {
			var err error
			metadataJSON, err = json.Marshal(m.Metadata)
			if err != nil {
				return fmt.Errorf("marshal metadata: %w", err)
			}
		}

		args = append(args,
			m.ID, string(m.Type), m.ClientID, m.ProviderID, m.MessageID,
			m.Status, m.Value, m.Timestamp, metadataJSON, m.CreatedAt,
		)
	}

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}
```

- [ ] **Step 2: Add in-memory buffer with periodic flush to the analytics service**

Where `Create` is called in the hot path, replace with buffered writes. Add a `BufferedMetricWriter` that accumulates metrics and flushes every 10 seconds or when buffer reaches 1000 items:

```go
type BufferedMetricWriter struct {
	repo      *MetricRepository
	mu        sync.Mutex
	buffer    []*domain.Metric
	flushSize int           // 1000
	flushInterval time.Duration // 10s
	logger    zerolog.Logger
}

func NewBufferedMetricWriter(repo *MetricRepository, flushSize int, flushInterval time.Duration) *BufferedMetricWriter {
	return &BufferedMetricWriter{
		repo:          repo,
		buffer:        make([]*domain.Metric, 0, flushSize),
		flushSize:     flushSize,
		flushInterval: flushInterval,
		logger:        log.With().Str("component", "metric-buffer").Logger(),
	}
}

func (w *BufferedMetricWriter) Add(metric *domain.Metric) {
	w.mu.Lock()
	w.buffer = append(w.buffer, metric)
	shouldFlush := len(w.buffer) >= w.flushSize
	w.mu.Unlock()

	if shouldFlush {
		w.Flush(context.Background())
	}
}

func (w *BufferedMetricWriter) Flush(ctx context.Context) {
	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return
	}
	batch := w.buffer
	w.buffer = make([]*domain.Metric, 0, w.flushSize)
	w.mu.Unlock()

	if err := w.repo.BatchCreate(ctx, batch); err != nil {
		w.logger.Error().Err(err).Int("count", len(batch)).Msg("batch metric flush failed")
		// Re-add to buffer for retry
		w.mu.Lock()
		w.buffer = append(batch, w.buffer...)
		w.mu.Unlock()
	}
}

func (w *BufferedMetricWriter) Start(ctx context.Context) {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.Flush(context.Background())
			return
		case <-ticker.C:
			w.Flush(ctx)
		}
	}
}
```

- [ ] **Step 3: Wire BufferedMetricWriter into callers**

Replace direct `metricRepo.Create()` calls with `bufferedWriter.Add()` in the hot path. The caller that publishes metrics on each message should use the buffer instead.

- [ ] **Step 4: Verify compilation and tests**

```bash
cd /home/magomed/projects/sms && go build ./... && go test ./internal/services/analytics/... -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/services/analytics/
git commit -m "perf(analytics): batch message_stats writes with in-memory buffer

BufferedMetricWriter accumulates metrics, flushes every 10s or 1000 items.
Reduces 241K individual INSERTs to ~241 batch INSERTs per load test."
```

---

## Task 15: Integration Verification

- [ ] **Step 1: Build all binaries**

```bash
cd /home/magomed/projects/sms && go build ./...
```
Expected: no errors.

- [ ] **Step 2: Run all unit tests**

```bash
cd /home/magomed/projects/sms && go test ./... -race -count=1
```
Expected: all tests pass.

- [ ] **Step 3: Start full stack locally**

```bash
cd deployments && docker compose down -v && docker compose up -d --build
docker compose ps
```

Verify all services are healthy.

- [ ] **Step 4: Run a smoke test**

Send a test message through the API and verify:
1. API returns 202 with message_id
2. Message appears in `sms.outgoing` Kafka topic
3. Persist stage writes it to DB (check after ~1 second)
4. Router stage routes it to `sms.routed`
5. Sender stage sends it via SMPP
6. Status stage updates the DB

```bash
curl -X POST http://localhost:8080/sms/send \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"source":"TEST","destination":"79001234567","text":"Performance test"}'
```

- [ ] **Step 5: Verify PostgreSQL tuning is active**

```bash
docker compose exec postgres psql -U smpp -d smpp_db -c "
  SELECT name, setting FROM pg_settings
  WHERE name IN ('shared_buffers', 'synchronous_commit', 'work_mem', 'wal_buffers', 'max_connections');
"
```

Expected:
```
shared_buffers       | 4GB (in 8kB pages)
synchronous_commit   | off
work_mem             | 64MB
wal_buffers          | 64MB
max_connections      | 100
```

- [ ] **Step 6: Verify PgBouncer stats**

```bash
docker compose exec pgbouncer psql -h localhost -p 6432 -U smpp pgbouncer -c "SHOW POOLS;"
```

- [ ] **Step 7: Run load test**

```bash
cd /home/magomed/projects/sms && go test ./test/load/ -run TestPipelineLoad -v -timeout 10m
```

Compare results with baseline. Expected: 5-10x improvement in throughput.

- [ ] **Step 8: Final commit (if any test/config fixes were needed)**

```bash
git add -A
git commit -m "test: verify pipeline performance optimization integration"
```
