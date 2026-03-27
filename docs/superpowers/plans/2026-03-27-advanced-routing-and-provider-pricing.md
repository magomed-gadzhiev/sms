# Advanced Routing & Provider Pricing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-client operator→provider routing with priority/weighted strategies, provider cost accounting (4 strategies), and visibility controls for subclient hierarchies.

**Architecture:** Three independent layers — (1) Routing: client_providers + client_routes + client_routing_strategies with capacity tracking in Redis; (2) Cost Accounting: provider_tariff_plans mirroring client tarification with async cost logging; (3) Visibility: expose_cost/expose_provider_name flags on inherited providers. The routing engine checks `clients.routing_mode` to decide legacy vs new pipeline.

**Tech Stack:** Go 1.24.0, pgx/v5, gorilla/mux, gRPC (google.golang.org/grpc), redis/go-redis/v9, zerolog, testify

---

## File Structure

### Database Migrations
- Create: `migrations/000037_create_client_providers.up.sql` — client_providers table
- Create: `migrations/000037_create_client_providers.down.sql`
- Create: `migrations/000038_create_client_routing.up.sql` — client_routing_strategies + client_routes
- Create: `migrations/000038_create_client_routing.down.sql`
- Create: `migrations/000039_create_provider_tarification.up.sql` — provider tariff tables + partitioned log
- Create: `migrations/000039_create_provider_tarification.down.sql`
- Create: `migrations/000040_extend_providers_capacity.up.sql` — rename throughput_per_second → tps_limit, add quotas
- Create: `migrations/000040_extend_providers_capacity.down.sql`
- Create: `migrations/000041_extend_clients_routing.up.sql` — routing_mode + cost_visibility_enabled on clients
- Create: `migrations/000041_extend_clients_routing.down.sql`

### Routing Domain Layer
- Create: `internal/services/routing/domain/client_provider.go` — ClientProvider model
- Create: `internal/services/routing/domain/client_route.go` — ClientRoute model
- Create: `internal/services/routing/domain/client_routing_strategy.go` — ClientRoutingStrategy model
- Create: `internal/services/routing/domain/routing_repository.go` — repository interfaces for new entities

### Routing Infrastructure Layer
- Create: `internal/services/routing/infrastructure/client_provider_repo.go` — PostgreSQL repo
- Create: `internal/services/routing/infrastructure/client_route_repo.go` — PostgreSQL repo
- Create: `internal/services/routing/infrastructure/client_routing_strategy_repo.go` — PostgreSQL repo
- Create: `internal/services/routing/infrastructure/capacity_tracker.go` — Redis-based TPS/quota tracker

### Routing Application Layer
- Create: `internal/services/routing/application/client_provider_service.go` — business logic
- Create: `internal/services/routing/application/client_route_service.go` — business logic
- Create: `internal/services/routing/application/new_routing_engine.go` — new routing pipeline (priority + weighted strategies)

### Provider Tarification Domain
- Create: `internal/services/tarification/domain/provider_tariff_plan.go` — ProviderTariffPlan + related models
- Create: `internal/services/tarification/domain/provider_tarification_log.go` — ProviderTarificationLog model
- Create: `internal/services/tarification/domain/provider_usage_counter.go` — ProviderUsageCounter model
- Create: `internal/services/tarification/domain/provider_repository.go` — provider tarification repo interfaces

### Provider Tarification Infrastructure
- Create: `internal/services/tarification/infrastructure/provider_tariff_plan_repo.go`
- Create: `internal/services/tarification/infrastructure/provider_tariff_period_repo.go`
- Create: `internal/services/tarification/infrastructure/provider_tariff_tier_repo.go`
- Create: `internal/services/tarification/infrastructure/provider_usage_counter_repo.go`
- Create: `internal/services/tarification/infrastructure/provider_tarification_log_repo.go`

### Provider Tarification Application
- Create: `internal/services/tarification/application/provider_tarification_service.go` — cost calculation service

### Proto Definitions
- Modify: `api/proto/routing/routing.proto` — add new RPCs for client providers, routes, strategies
- Modify: `api/proto/tarification/tarification.proto` — add provider tariff + margin report RPCs

### gRPC Servers
- Modify: `internal/services/routing/grpc/server.go` — add new RPC implementations
- Modify: `internal/services/tarification/grpc/server.go` — add provider tarification RPCs

### REST API
- Create: `internal/gateway/admin/handlers/client_routing.go` — handlers for client providers, routes, strategies, margin
- Modify: `internal/gateway/admin/router/router.go` — register new endpoints

### Routing Engine Integration
- Modify: `internal/router/router.go` — add NewRoutingEngine that checks routing_mode
- Modify: `internal/shared/models.go` — add RoutingMode + CostVisibilityEnabled to Client

### Service Wiring
- Modify: `cmd/services/routing-service/main.go` — wire new repos + services
- Modify: `cmd/services/tarification-service/main.go` — wire provider tarification repos + service

---

## Task 1: Database Migrations — Client Providers

**Files:**
- Create: `migrations/000037_create_client_providers.up.sql`
- Create: `migrations/000037_create_client_providers.down.sql`

- [ ] **Step 1: Write migration UP**

```sql
-- migrations/000037_create_client_providers.up.sql

CREATE TABLE IF NOT EXISTS client_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id),
    provider_id UUID NOT NULL REFERENCES providers(id),
    ownership VARCHAR(20) NOT NULL CHECK (ownership IN ('platform', 'private', 'inherited')),
    source_client_id UUID REFERENCES clients(id),
    shared_priority INT NOT NULL DEFAULT 0,
    expose_cost BOOLEAN NOT NULL DEFAULT false,
    expose_provider_name BOOLEAN NOT NULL DEFAULT true,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(client_id, provider_id)
);

CREATE INDEX idx_client_providers_client ON client_providers(client_id) WHERE active = true;
CREATE INDEX idx_client_providers_provider ON client_providers(provider_id) WHERE active = true;
CREATE INDEX idx_client_providers_source ON client_providers(source_client_id) WHERE source_client_id IS NOT NULL;

CREATE TRIGGER update_client_providers_updated_at
    BEFORE UPDATE ON client_providers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

- [ ] **Step 2: Write migration DOWN**

```sql
-- migrations/000037_create_client_providers.down.sql

DROP TRIGGER IF EXISTS update_client_providers_updated_at ON client_providers;
DROP TABLE IF EXISTS client_providers;
```

- [ ] **Step 3: Verify migration applies**

Run: `cd /home/magomed/projects/sms && cat migrations/000037_create_client_providers.up.sql`
Expected: SQL matches above

- [ ] **Step 4: Commit**

```bash
git add migrations/000037_create_client_providers.up.sql migrations/000037_create_client_providers.down.sql
git commit -m "feat(routing): add client_providers migration (000037)"
```

---

## Task 2: Database Migrations — Client Routing

**Files:**
- Create: `migrations/000038_create_client_routing.up.sql`
- Create: `migrations/000038_create_client_routing.down.sql`

- [ ] **Step 1: Write migration UP**

```sql
-- migrations/000038_create_client_routing.up.sql

CREATE TABLE IF NOT EXISTS client_routing_strategies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id),
    operator_id UUID REFERENCES operators(id),
    strategy VARCHAR(20) NOT NULL CHECK (strategy IN ('priority', 'weighted', 'smart')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(client_id, operator_id)
);

-- Partial unique for default account strategy (operator_id IS NULL)
CREATE UNIQUE INDEX uq_client_routing_strategy_default
    ON client_routing_strategies (client_id)
    WHERE operator_id IS NULL;

CREATE TABLE IF NOT EXISTS client_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id),
    operator_id UUID NOT NULL REFERENCES operators(id),
    provider_id UUID NOT NULL REFERENCES providers(id),
    priority INT NOT NULL DEFAULT 0,
    weight INT NOT NULL DEFAULT 1,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(client_id, operator_id, provider_id)
);

CREATE INDEX idx_client_routes_lookup ON client_routes(client_id, operator_id) WHERE active = true;

CREATE TRIGGER update_client_routing_strategies_updated_at
    BEFORE UPDATE ON client_routing_strategies
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_client_routes_updated_at
    BEFORE UPDATE ON client_routes
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

- [ ] **Step 2: Write migration DOWN**

```sql
-- migrations/000038_create_client_routing.down.sql

DROP TRIGGER IF EXISTS update_client_routes_updated_at ON client_routes;
DROP TRIGGER IF EXISTS update_client_routing_strategies_updated_at ON client_routing_strategies;
DROP TABLE IF EXISTS client_routes;
DROP TABLE IF EXISTS client_routing_strategies;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000038_create_client_routing.up.sql migrations/000038_create_client_routing.down.sql
git commit -m "feat(routing): add client_routing_strategies and client_routes migrations (000038)"
```

---

## Task 3: Database Migrations — Provider Tarification

**Files:**
- Create: `migrations/000039_create_provider_tarification.up.sql`
- Create: `migrations/000039_create_provider_tarification.down.sql`

- [ ] **Step 1: Write migration UP**

```sql
-- migrations/000039_create_provider_tarification.up.sql

CREATE TABLE IF NOT EXISTS provider_tariff_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id),
    operator_id UUID NOT NULL REFERENCES operators(id),
    strategy VARCHAR(50) NOT NULL CHECK (strategy IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold')),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_provider_tariff_plans_unique_active
    ON provider_tariff_plans(provider_id, operator_id) WHERE active = true;

CREATE TABLE IF NOT EXISTS provider_tariff_periods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_tariff_plan_id UUID NOT NULL REFERENCES provider_tariff_plans(id) ON DELETE CASCADE,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_date > start_date)
);

ALTER TABLE provider_tariff_periods ADD CONSTRAINT provider_tariff_periods_no_overlap
    EXCLUDE USING gist (provider_tariff_plan_id WITH =, daterange(start_date, end_date, '[]') WITH &&);

CREATE INDEX idx_provider_tariff_periods_plan ON provider_tariff_periods(provider_tariff_plan_id);

CREATE TABLE IF NOT EXISTS provider_tariff_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_tariff_period_id UUID NOT NULL REFERENCES provider_tariff_periods(id) ON DELETE CASCADE,
    from_count INT NOT NULL DEFAULT 0 CHECK (from_count >= 0),
    price_per_segment NUMERIC(20,6) NOT NULL CHECK (price_per_segment >= 0),
    UNIQUE(provider_tariff_period_id, from_count)
);

CREATE TABLE IF NOT EXISTS provider_usage_counters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_tariff_plan_id UUID NOT NULL REFERENCES provider_tariff_plans(id),
    provider_tariff_period_id UUID NOT NULL REFERENCES provider_tariff_periods(id),
    segment_count INT NOT NULL DEFAULT 0 CHECK (segment_count >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider_tariff_plan_id, provider_tariff_period_id)
);

-- Provider tarification log with monthly partitioning
CREATE TABLE IF NOT EXISTS provider_tarification_log (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    provider_id UUID NOT NULL,
    operator_id UUID NOT NULL,
    client_id UUID NOT NULL,
    message_id UUID NOT NULL,
    segment_count INT NOT NULL,
    price_per_segment NUMERIC(20,6) NOT NULL,
    total_cost NUMERIC(20,6) NOT NULL,
    strategy VARCHAR(50) NOT NULL,
    provider_tariff_plan_id UUID NOT NULL,
    provider_tariff_period_id UUID NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- 12 monthly partitions for 2026
CREATE TABLE provider_tarification_log_2026_01 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE provider_tarification_log_2026_02 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE provider_tarification_log_2026_03 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE provider_tarification_log_2026_04 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE provider_tarification_log_2026_05 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE provider_tarification_log_2026_06 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE provider_tarification_log_2026_07 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE provider_tarification_log_2026_08 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE provider_tarification_log_2026_09 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE provider_tarification_log_2026_10 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE provider_tarification_log_2026_11 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE provider_tarification_log_2026_12 PARTITION OF provider_tarification_log
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

CREATE INDEX idx_provider_tarification_log_provider ON provider_tarification_log(provider_id, created_at);
CREATE INDEX idx_provider_tarification_log_client ON provider_tarification_log(client_id, created_at);
CREATE INDEX idx_provider_tarification_log_message ON provider_tarification_log(message_id);
CREATE UNIQUE INDEX idx_provider_tarification_log_idempotency ON provider_tarification_log(idempotency_key, created_at);

CREATE TRIGGER update_provider_tariff_plans_updated_at
    BEFORE UPDATE ON provider_tariff_plans
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_provider_usage_counters_updated_at
    BEFORE UPDATE ON provider_usage_counters
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

- [ ] **Step 2: Write migration DOWN**

```sql
-- migrations/000039_create_provider_tarification.down.sql

DROP TRIGGER IF EXISTS update_provider_usage_counters_updated_at ON provider_usage_counters;
DROP TRIGGER IF EXISTS update_provider_tariff_plans_updated_at ON provider_tariff_plans;
DROP TABLE IF EXISTS provider_tarification_log;
DROP TABLE IF EXISTS provider_usage_counters;
DROP TABLE IF EXISTS provider_tariff_tiers;
DROP TABLE IF EXISTS provider_tariff_periods;
DROP TABLE IF EXISTS provider_tariff_plans;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000039_create_provider_tarification.up.sql migrations/000039_create_provider_tarification.down.sql
git commit -m "feat(tarification): add provider tarification tables migration (000039)"
```

---

## Task 4: Database Migrations — Provider Capacity & Client Extensions

**Files:**
- Create: `migrations/000040_extend_providers_capacity.up.sql`
- Create: `migrations/000040_extend_providers_capacity.down.sql`
- Create: `migrations/000041_extend_clients_routing.up.sql`
- Create: `migrations/000041_extend_clients_routing.down.sql`

- [ ] **Step 1: Write providers capacity migration UP**

```sql
-- migrations/000040_extend_providers_capacity.up.sql

-- Note: throughput_per_second was already renamed to tps_limit in migration 000033 via the tps_limit column.
-- The providers table already has tps_limit from 000033. We only add quotas.
ALTER TABLE providers ADD COLUMN IF NOT EXISTS daily_quota INT;
ALTER TABLE providers ADD COLUMN IF NOT EXISTS monthly_quota INT;
```

- [ ] **Step 2: Write providers capacity migration DOWN**

```sql
-- migrations/000040_extend_providers_capacity.down.sql

ALTER TABLE providers DROP COLUMN IF EXISTS monthly_quota;
ALTER TABLE providers DROP COLUMN IF EXISTS daily_quota;
```

- [ ] **Step 3: Write clients routing extension migration UP**

```sql
-- migrations/000041_extend_clients_routing.up.sql

ALTER TABLE clients ADD COLUMN IF NOT EXISTS routing_mode VARCHAR(20) NOT NULL DEFAULT 'legacy'
    CHECK (routing_mode IN ('legacy', 'new', 'hybrid'));
ALTER TABLE clients ADD COLUMN IF NOT EXISTS cost_visibility_enabled BOOLEAN NOT NULL DEFAULT false;
```

- [ ] **Step 4: Write clients routing extension migration DOWN**

```sql
-- migrations/000041_extend_clients_routing.down.sql

ALTER TABLE clients DROP COLUMN IF EXISTS cost_visibility_enabled;
ALTER TABLE clients DROP COLUMN IF EXISTS routing_mode;
```

- [ ] **Step 5: Commit**

```bash
git add migrations/000040_extend_providers_capacity.up.sql migrations/000040_extend_providers_capacity.down.sql \
      migrations/000041_extend_clients_routing.up.sql migrations/000041_extend_clients_routing.down.sql
git commit -m "feat: add provider capacity quotas and client routing_mode migrations (000040-041)"
```

---

## Task 5: Routing Domain Models

**Files:**
- Create: `internal/services/routing/domain/client_provider.go`
- Create: `internal/services/routing/domain/client_route.go`
- Create: `internal/services/routing/domain/client_routing_strategy.go`
- Create: `internal/services/routing/domain/routing_repository.go`

- [ ] **Step 1: Write ClientProvider domain model**

```go
// internal/services/routing/domain/client_provider.go
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ProviderOwnership string

const (
	OwnershipPlatform  ProviderOwnership = "platform"
	OwnershipPrivate   ProviderOwnership = "private"
	OwnershipInherited ProviderOwnership = "inherited"
)

type ClientProvider struct {
	ID                 uuid.UUID
	ClientID           uuid.UUID
	ProviderID         uuid.UUID
	Ownership          ProviderOwnership
	SourceClientID     *uuid.UUID
	SharedPriority     int
	ExposeCost         bool
	ExposeProviderName bool
	Active             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

var (
	ErrClientProviderNotFound     = errors.New("client provider not found")
	ErrClientProviderAlreadyExists = errors.New("client already has this provider")
	ErrInvalidOwnership           = errors.New("invalid ownership type")
	ErrSourceClientRequired       = errors.New("source_client_id required for inherited ownership")
	ErrProviderNotAssigned        = errors.New("provider not assigned to client")
)

func NewClientProvider(clientID, providerID uuid.UUID, ownership ProviderOwnership) *ClientProvider {
	now := time.Now()
	return &ClientProvider{
		ID:                 uuid.New(),
		ClientID:           clientID,
		ProviderID:         providerID,
		Ownership:          ownership,
		SharedPriority:     0,
		ExposeCost:         false,
		ExposeProviderName: true,
		Active:             true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func (cp *ClientProvider) Validate() error {
	switch cp.Ownership {
	case OwnershipPlatform, OwnershipPrivate, OwnershipInherited:
	default:
		return ErrInvalidOwnership
	}
	if cp.Ownership == OwnershipInherited && cp.SourceClientID == nil {
		return ErrSourceClientRequired
	}
	return nil
}
```

- [ ] **Step 2: Write ClientRoute domain model**

```go
// internal/services/routing/domain/client_route.go
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ClientRoute struct {
	ID         uuid.UUID
	ClientID   uuid.UUID
	OperatorID uuid.UUID
	ProviderID uuid.UUID
	Priority   int
	Weight     int
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	ErrClientRouteNotFound     = errors.New("client route not found")
	ErrClientRouteAlreadyExists = errors.New("route already exists for this client/operator/provider")
)

func NewClientRoute(clientID, operatorID, providerID uuid.UUID, priority, weight int) *ClientRoute {
	now := time.Now()
	return &ClientRoute{
		ID:         uuid.New(),
		ClientID:   clientID,
		OperatorID: operatorID,
		ProviderID: providerID,
		Priority:   priority,
		Weight:     weight,
		Active:     true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
```

- [ ] **Step 3: Write ClientRoutingStrategy domain model**

```go
// internal/services/routing/domain/client_routing_strategy.go
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type RoutingStrategy string

const (
	StrategyPriority RoutingStrategy = "priority"
	StrategyWeighted RoutingStrategy = "weighted"
	StrategySmart    RoutingStrategy = "smart"
)

type ClientRoutingStrategy struct {
	ID         uuid.UUID
	ClientID   uuid.UUID
	OperatorID *uuid.UUID // nil = account default
	Strategy   RoutingStrategy
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	ErrInvalidRoutingStrategy = errors.New("invalid routing strategy")
	ErrRoutingStrategyNotFound = errors.New("routing strategy not found")
)

func NewClientRoutingStrategy(clientID uuid.UUID, operatorID *uuid.UUID, strategy RoutingStrategy) *ClientRoutingStrategy {
	now := time.Now()
	return &ClientRoutingStrategy{
		ID:         uuid.New(),
		ClientID:   clientID,
		OperatorID: operatorID,
		Strategy:   strategy,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func (s *ClientRoutingStrategy) Validate() error {
	switch s.Strategy {
	case StrategyPriority, StrategyWeighted, StrategySmart:
		return nil
	default:
		return ErrInvalidRoutingStrategy
	}
}
```

- [ ] **Step 4: Write repository interfaces**

```go
// internal/services/routing/domain/routing_repository.go
package domain

import (
	"context"

	"github.com/google/uuid"
)

type ClientProviderRepository interface {
	Create(ctx context.Context, cp *ClientProvider) error
	GetByID(ctx context.Context, id uuid.UUID) (*ClientProvider, error)
	GetByClientAndProvider(ctx context.Context, clientID, providerID uuid.UUID) (*ClientProvider, error)
	ListByClient(ctx context.Context, clientID uuid.UUID, activeOnly bool) ([]*ClientProvider, error)
	ListBySourceClient(ctx context.Context, sourceClientID uuid.UUID) ([]*ClientProvider, error)
	Update(ctx context.Context, cp *ClientProvider) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ClientRouteRepository interface {
	Create(ctx context.Context, route *ClientRoute) error
	GetByID(ctx context.Context, id uuid.UUID) (*ClientRoute, error)
	ListByClientAndOperator(ctx context.Context, clientID, operatorID uuid.UUID, activeOnly bool) ([]*ClientRoute, error)
	ListByClient(ctx context.Context, clientID uuid.UUID) ([]*ClientRoute, error)
	Update(ctx context.Context, route *ClientRoute) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ClientRoutingStrategyRepository interface {
	Upsert(ctx context.Context, strategy *ClientRoutingStrategy) error
	Get(ctx context.Context, clientID uuid.UUID, operatorID *uuid.UUID) (*ClientRoutingStrategy, error)
	Delete(ctx context.Context, clientID uuid.UUID, operatorID *uuid.UUID) error
}
```

- [ ] **Step 5: Commit**

```bash
git add internal/services/routing/domain/client_provider.go \
      internal/services/routing/domain/client_route.go \
      internal/services/routing/domain/client_routing_strategy.go \
      internal/services/routing/domain/routing_repository.go
git commit -m "feat(routing): add domain models for client providers, routes, and strategies"
```

---

## Task 6: Routing Infrastructure — ClientProvider Repository

**Files:**
- Create: `internal/services/routing/infrastructure/client_provider_repo.go`

- [ ] **Step 1: Write ClientProvider PostgreSQL repository**

```go
// internal/services/routing/infrastructure/client_provider_repo.go
package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

type ClientProviderRepo struct {
	pool *pgxpool.Pool
}

func NewClientProviderRepo(pool *pgxpool.Pool) *ClientProviderRepo {
	return &ClientProviderRepo{pool: pool}
}

func (r *ClientProviderRepo) Create(ctx context.Context, cp *domain.ClientProvider) error {
	query := `INSERT INTO client_providers (id, client_id, provider_id, ownership, source_client_id, shared_priority, expose_cost, expose_provider_name, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := r.pool.Exec(ctx, query,
		cp.ID, cp.ClientID, cp.ProviderID, cp.Ownership, cp.SourceClientID,
		cp.SharedPriority, cp.ExposeCost, cp.ExposeProviderName, cp.Active,
		cp.CreatedAt, cp.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create client_provider: %w", err)
	}
	return nil
}

func (r *ClientProviderRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ClientProvider, error) {
	query := `SELECT id, client_id, provider_id, ownership, source_client_id, shared_priority, expose_cost, expose_provider_name, active, created_at, updated_at
		FROM client_providers WHERE id = $1`
	return r.scanOne(ctx, query, id)
}

func (r *ClientProviderRepo) GetByClientAndProvider(ctx context.Context, clientID, providerID uuid.UUID) (*domain.ClientProvider, error) {
	query := `SELECT id, client_id, provider_id, ownership, source_client_id, shared_priority, expose_cost, expose_provider_name, active, created_at, updated_at
		FROM client_providers WHERE client_id = $1 AND provider_id = $2`
	return r.scanOne(ctx, query, clientID, providerID)
}

func (r *ClientProviderRepo) ListByClient(ctx context.Context, clientID uuid.UUID, activeOnly bool) ([]*domain.ClientProvider, error) {
	query := `SELECT id, client_id, provider_id, ownership, source_client_id, shared_priority, expose_cost, expose_provider_name, active, created_at, updated_at
		FROM client_providers WHERE client_id = $1`
	if activeOnly {
		query += " AND active = true"
	}
	query += " ORDER BY shared_priority DESC"
	return r.scanMany(ctx, query, clientID)
}

func (r *ClientProviderRepo) ListBySourceClient(ctx context.Context, sourceClientID uuid.UUID) ([]*domain.ClientProvider, error) {
	query := `SELECT id, client_id, provider_id, ownership, source_client_id, shared_priority, expose_cost, expose_provider_name, active, created_at, updated_at
		FROM client_providers WHERE source_client_id = $1 ORDER BY created_at`
	return r.scanMany(ctx, query, sourceClientID)
}

func (r *ClientProviderRepo) Update(ctx context.Context, cp *domain.ClientProvider) error {
	query := `UPDATE client_providers SET shared_priority = $1, expose_cost = $2, expose_provider_name = $3, active = $4
		WHERE id = $5`
	tag, err := r.pool.Exec(ctx, query, cp.SharedPriority, cp.ExposeCost, cp.ExposeProviderName, cp.Active, cp.ID)
	if err != nil {
		return fmt.Errorf("update client_provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientProviderNotFound
	}
	return nil
}

func (r *ClientProviderRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM client_providers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete client_provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientProviderNotFound
	}
	return nil
}

func (r *ClientProviderRepo) scanOne(ctx context.Context, query string, args ...interface{}) (*domain.ClientProvider, error) {
	cp := &domain.ClientProvider{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&cp.ID, &cp.ClientID, &cp.ProviderID, &cp.Ownership, &cp.SourceClientID,
		&cp.SharedPriority, &cp.ExposeCost, &cp.ExposeProviderName, &cp.Active,
		&cp.CreatedAt, &cp.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrClientProviderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan client_provider: %w", err)
	}
	return cp, nil
}

func (r *ClientProviderRepo) scanMany(ctx context.Context, query string, args ...interface{}) ([]*domain.ClientProvider, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query client_providers: %w", err)
	}
	defer rows.Close()

	var result []*domain.ClientProvider
	for rows.Next() {
		cp := &domain.ClientProvider{}
		if err := rows.Scan(
			&cp.ID, &cp.ClientID, &cp.ProviderID, &cp.Ownership, &cp.SourceClientID,
			&cp.SharedPriority, &cp.ExposeCost, &cp.ExposeProviderName, &cp.Active,
			&cp.CreatedAt, &cp.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan client_provider row: %w", err)
		}
		result = append(result, cp)
	}
	return result, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/routing/infrastructure/client_provider_repo.go
git commit -m "feat(routing): add ClientProvider PostgreSQL repository"
```

---

## Task 7: Routing Infrastructure — ClientRoute & Strategy Repositories

**Files:**
- Create: `internal/services/routing/infrastructure/client_route_repo.go`
- Create: `internal/services/routing/infrastructure/client_routing_strategy_repo.go`

- [ ] **Step 1: Write ClientRoute PostgreSQL repository**

```go
// internal/services/routing/infrastructure/client_route_repo.go
package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

type ClientRouteRepo struct {
	pool *pgxpool.Pool
}

func NewClientRouteRepo(pool *pgxpool.Pool) *ClientRouteRepo {
	return &ClientRouteRepo{pool: pool}
}

func (r *ClientRouteRepo) Create(ctx context.Context, route *domain.ClientRoute) error {
	query := `INSERT INTO client_routes (id, client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.pool.Exec(ctx, query,
		route.ID, route.ClientID, route.OperatorID, route.ProviderID,
		route.Priority, route.Weight, route.Active, route.CreatedAt, route.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create client_route: %w", err)
	}
	return nil
}

func (r *ClientRouteRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ClientRoute, error) {
	query := `SELECT id, client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at
		FROM client_routes WHERE id = $1`
	route := &domain.ClientRoute{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&route.ID, &route.ClientID, &route.OperatorID, &route.ProviderID,
		&route.Priority, &route.Weight, &route.Active, &route.CreatedAt, &route.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrClientRouteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get client_route: %w", err)
	}
	return route, nil
}

func (r *ClientRouteRepo) ListByClientAndOperator(ctx context.Context, clientID, operatorID uuid.UUID, activeOnly bool) ([]*domain.ClientRoute, error) {
	query := `SELECT id, client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at
		FROM client_routes WHERE client_id = $1 AND operator_id = $2`
	if activeOnly {
		query += " AND active = true"
	}
	query += " ORDER BY priority DESC, weight DESC"
	return r.scanMany(ctx, query, clientID, operatorID)
}

func (r *ClientRouteRepo) ListByClient(ctx context.Context, clientID uuid.UUID) ([]*domain.ClientRoute, error) {
	query := `SELECT id, client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at
		FROM client_routes WHERE client_id = $1 ORDER BY operator_id, priority DESC`
	return r.scanMany(ctx, query, clientID)
}

func (r *ClientRouteRepo) Update(ctx context.Context, route *domain.ClientRoute) error {
	query := `UPDATE client_routes SET priority = $1, weight = $2, active = $3 WHERE id = $4`
	tag, err := r.pool.Exec(ctx, query, route.Priority, route.Weight, route.Active, route.ID)
	if err != nil {
		return fmt.Errorf("update client_route: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientRouteNotFound
	}
	return nil
}

func (r *ClientRouteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM client_routes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete client_route: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientRouteNotFound
	}
	return nil
}

func (r *ClientRouteRepo) scanMany(ctx context.Context, query string, args ...interface{}) ([]*domain.ClientRoute, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query client_routes: %w", err)
	}
	defer rows.Close()

	var result []*domain.ClientRoute
	for rows.Next() {
		route := &domain.ClientRoute{}
		if err := rows.Scan(
			&route.ID, &route.ClientID, &route.OperatorID, &route.ProviderID,
			&route.Priority, &route.Weight, &route.Active, &route.CreatedAt, &route.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan client_route row: %w", err)
		}
		result = append(result, route)
	}
	return result, nil
}
```

- [ ] **Step 2: Write ClientRoutingStrategy PostgreSQL repository**

```go
// internal/services/routing/infrastructure/client_routing_strategy_repo.go
package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

type ClientRoutingStrategyRepo struct {
	pool *pgxpool.Pool
}

func NewClientRoutingStrategyRepo(pool *pgxpool.Pool) *ClientRoutingStrategyRepo {
	return &ClientRoutingStrategyRepo{pool: pool}
}

func (r *ClientRoutingStrategyRepo) Upsert(ctx context.Context, s *domain.ClientRoutingStrategy) error {
	query := `INSERT INTO client_routing_strategies (id, client_id, operator_id, strategy, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (client_id, operator_id) WHERE operator_id IS NOT NULL
		DO UPDATE SET strategy = EXCLUDED.strategy, updated_at = EXCLUDED.updated_at`

	// Handle NULL operator_id separately for the partial unique index
	if s.OperatorID == nil {
		query = `INSERT INTO client_routing_strategies (id, client_id, operator_id, strategy, created_at, updated_at)
			VALUES ($1, $2, NULL, $4, $5, $6)
			ON CONFLICT (client_id) WHERE operator_id IS NULL
			DO UPDATE SET strategy = EXCLUDED.strategy, updated_at = EXCLUDED.updated_at`
	}

	_, err := r.pool.Exec(ctx, query, s.ID, s.ClientID, s.OperatorID, s.Strategy, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert client_routing_strategy: %w", err)
	}
	return nil
}

func (r *ClientRoutingStrategyRepo) Get(ctx context.Context, clientID uuid.UUID, operatorID *uuid.UUID) (*domain.ClientRoutingStrategy, error) {
	var query string
	var args []interface{}

	if operatorID != nil {
		query = `SELECT id, client_id, operator_id, strategy, created_at, updated_at
			FROM client_routing_strategies WHERE client_id = $1 AND operator_id = $2`
		args = []interface{}{clientID, *operatorID}
	} else {
		query = `SELECT id, client_id, operator_id, strategy, created_at, updated_at
			FROM client_routing_strategies WHERE client_id = $1 AND operator_id IS NULL`
		args = []interface{}{clientID}
	}

	s := &domain.ClientRoutingStrategy{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&s.ID, &s.ClientID, &s.OperatorID, &s.Strategy, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrRoutingStrategyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get client_routing_strategy: %w", err)
	}
	return s, nil
}

func (r *ClientRoutingStrategyRepo) Delete(ctx context.Context, clientID uuid.UUID, operatorID *uuid.UUID) error {
	var query string
	var args []interface{}

	if operatorID != nil {
		query = `DELETE FROM client_routing_strategies WHERE client_id = $1 AND operator_id = $2`
		args = []interface{}{clientID, *operatorID}
	} else {
		query = `DELETE FROM client_routing_strategies WHERE client_id = $1 AND operator_id IS NULL`
		args = []interface{}{clientID}
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete client_routing_strategy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRoutingStrategyNotFound
	}
	return nil
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/routing/infrastructure/client_route_repo.go \
      internal/services/routing/infrastructure/client_routing_strategy_repo.go
git commit -m "feat(routing): add ClientRoute and ClientRoutingStrategy PostgreSQL repositories"
```

---

## Task 8: Redis Capacity Tracker

**Files:**
- Create: `internal/services/routing/infrastructure/capacity_tracker.go`

- [ ] **Step 1: Write Redis-based capacity tracker**

```go
// internal/services/routing/infrastructure/capacity_tracker.go
package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type CapacityTracker struct {
	rdb *redis.Client
}

func NewCapacityTracker(rdb *redis.Client) *CapacityTracker {
	return &CapacityTracker{rdb: rdb}
}

// CheckAndIncrement atomically checks if provider has capacity and increments counters.
// Returns true if capacity is available, false if any limit exceeded.
func (t *CapacityTracker) CheckAndIncrement(ctx context.Context, providerID uuid.UUID, tpsLimit, dailyQuota, monthlyQuota int) (bool, error) {
	pid := providerID.String()

	// Check TPS with sliding window (1 second TTL)
	if tpsLimit > 0 {
		tpsKey := fmt.Sprintf("provider:%s:tps_current", pid)
		val, err := t.rdb.Incr(ctx, tpsKey).Result()
		if err != nil {
			return false, fmt.Errorf("tps incr: %w", err)
		}
		if val == 1 {
			t.rdb.Expire(ctx, tpsKey, time.Second)
		}
		if int(val) > tpsLimit {
			t.rdb.Decr(ctx, tpsKey)
			return false, nil
		}
	}

	// Check daily quota
	if dailyQuota > 0 {
		dailyKey := fmt.Sprintf("provider:%s:daily_count", pid)
		val, err := t.rdb.Incr(ctx, dailyKey).Result()
		if err != nil {
			return false, fmt.Errorf("daily incr: %w", err)
		}
		if val == 1 {
			// Expire at midnight UTC
			now := time.Now().UTC()
			midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
			t.rdb.ExpireAt(ctx, dailyKey, midnight)
		}
		if int(val) > dailyQuota {
			t.rdb.Decr(ctx, dailyKey)
			// Rollback TPS if we checked it
			if tpsLimit > 0 {
				tpsKey := fmt.Sprintf("provider:%s:tps_current", pid)
				t.rdb.Decr(ctx, tpsKey)
			}
			return false, nil
		}
	}

	// Check monthly quota
	if monthlyQuota > 0 {
		monthlyKey := fmt.Sprintf("provider:%s:monthly_count", pid)
		val, err := t.rdb.Incr(ctx, monthlyKey).Result()
		if err != nil {
			return false, fmt.Errorf("monthly incr: %w", err)
		}
		if val == 1 {
			// Expire at 1st of next month UTC
			now := time.Now().UTC()
			nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
			t.rdb.ExpireAt(ctx, monthlyKey, nextMonth)
		}
		if int(val) > monthlyQuota {
			t.rdb.Decr(ctx, monthlyKey)
			if dailyQuota > 0 {
				t.rdb.Decr(ctx, fmt.Sprintf("provider:%s:daily_count", pid))
			}
			if tpsLimit > 0 {
				t.rdb.Decr(ctx, fmt.Sprintf("provider:%s:tps_current", pid))
			}
			return false, nil
		}
	}

	return true, nil
}

// GetCurrentUsage returns current TPS, daily, and monthly usage for a provider.
func (t *CapacityTracker) GetCurrentUsage(ctx context.Context, providerID uuid.UUID) (tps, daily, monthly int64, err error) {
	pid := providerID.String()
	pipe := t.rdb.Pipeline()

	tpsCmd := pipe.Get(ctx, fmt.Sprintf("provider:%s:tps_current", pid))
	dailyCmd := pipe.Get(ctx, fmt.Sprintf("provider:%s:daily_count", pid))
	monthlyCmd := pipe.Get(ctx, fmt.Sprintf("provider:%s:monthly_count", pid))

	_, _ = pipe.Exec(ctx)

	tps, _ = tpsCmd.Int64()
	daily, _ = dailyCmd.Int64()
	monthly, _ = monthlyCmd.Int64()

	return tps, daily, monthly, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/routing/infrastructure/capacity_tracker.go
git commit -m "feat(routing): add Redis-based provider capacity tracker"
```

---

## Task 9: New Routing Engine

**Files:**
- Create: `internal/services/routing/application/new_routing_engine.go`

- [ ] **Step 1: Write the new routing engine with priority and weighted strategies**

```go
// internal/services/routing/application/new_routing_engine.go
package application

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/infrastructure"
	"github.com/smpp-server/smpp-server/internal/shared"
)

var (
	ErrNoRouteFound      = errors.New("no route found for operator")
	ErrNoAvailableProvider = errors.New("no available provider after filtering")
)

// ProviderInfo holds provider data needed for routing decisions.
type ProviderInfo struct {
	ID           uuid.UUID
	Active       bool
	TPSLimit     int
	DailyQuota   int
	MonthlyQuota int
}

// ProviderLookup resolves provider info by ID.
type ProviderLookup interface {
	GetProviderInfo(ctx context.Context, id uuid.UUID) (*ProviderInfo, error)
}

// NewRoutingEngine implements the new client-based routing pipeline.
type NewRoutingEngine struct {
	strategyRepo domain.ClientRoutingStrategyRepository
	routeRepo    domain.ClientRouteRepository
	providerRepo domain.ClientProviderRepository
	capacity     *infrastructure.CapacityTracker
	providers    ProviderLookup
	logger       zerolog.Logger
}

func NewNewRoutingEngine(
	strategyRepo domain.ClientRoutingStrategyRepository,
	routeRepo domain.ClientRouteRepository,
	providerRepo domain.ClientProviderRepository,
	capacity *infrastructure.CapacityTracker,
	providers ProviderLookup,
) *NewRoutingEngine {
	return &NewRoutingEngine{
		strategyRepo: strategyRepo,
		routeRepo:    routeRepo,
		providerRepo: providerRepo,
		capacity:     capacity,
		providers:    providers,
		logger:       log.With().Str("component", "new-routing-engine").Logger(),
	}
}

// SelectProvider selects a provider for the given client and operator using the new routing system.
func (e *NewRoutingEngine) SelectProvider(ctx context.Context, clientID, operatorID uuid.UUID) (*uuid.UUID, error) {
	// 1. Resolve routing strategy: (client, operator) → (client, nil) → default priority
	strategy := e.resolveStrategy(ctx, clientID, operatorID)

	// 2. Get active routes for client + operator
	routes, err := e.routeRepo.ListByClientAndOperator(ctx, clientID, operatorID, true)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	if len(routes) == 0 {
		return nil, ErrNoRouteFound
	}

	// 3. Filter and select by strategy
	switch strategy {
	case domain.StrategyWeighted:
		return e.selectWeighted(ctx, routes)
	default: // priority is the default
		return e.selectPriority(ctx, routes)
	}
}

func (e *NewRoutingEngine) resolveStrategy(ctx context.Context, clientID, operatorID uuid.UUID) domain.RoutingStrategy {
	// Try (client, operator)
	s, err := e.strategyRepo.Get(ctx, clientID, &operatorID)
	if err == nil {
		return s.Strategy
	}

	// Fallback: (client, nil) — account default
	s, err = e.strategyRepo.Get(ctx, clientID, nil)
	if err == nil {
		return s.Strategy
	}

	// Global default
	return domain.StrategyPriority
}

func (e *NewRoutingEngine) selectPriority(ctx context.Context, routes []*domain.ClientRoute) (*uuid.UUID, error) {
	// Routes already sorted by priority DESC from repo
	for _, route := range routes {
		available, err := e.isProviderAvailable(ctx, route.ProviderID)
		if err != nil {
			e.logger.Warn().Err(err).Str("provider_id", route.ProviderID.String()).Msg("provider availability check failed, skipping")
			continue
		}
		if available {
			id := route.ProviderID
			return &id, nil
		}
		e.logger.Debug().Str("provider_id", route.ProviderID.String()).Int("priority", route.Priority).Msg("provider unavailable, trying next")
	}
	return nil, ErrNoAvailableProvider
}

func (e *NewRoutingEngine) selectWeighted(ctx context.Context, routes []*domain.ClientRoute) (*uuid.UUID, error) {
	// Filter to available providers first
	var available []*domain.ClientRoute
	for _, route := range routes {
		ok, err := e.isProviderAvailable(ctx, route.ProviderID)
		if err != nil {
			e.logger.Warn().Err(err).Str("provider_id", route.ProviderID.String()).Msg("provider availability check failed, skipping")
			continue
		}
		if ok {
			available = append(available, route)
		}
	}
	if len(available) == 0 {
		return nil, ErrNoAvailableProvider
	}

	// Weighted random selection
	totalWeight := 0
	for _, r := range available {
		totalWeight += r.Weight
	}

	pick := rand.Intn(totalWeight)
	cumulative := 0
	for _, r := range available {
		cumulative += r.Weight
		if pick < cumulative {
			id := r.ProviderID
			return &id, nil
		}
	}

	// Fallback (shouldn't reach here)
	id := available[0].ProviderID
	return &id, nil
}

func (e *NewRoutingEngine) isProviderAvailable(ctx context.Context, providerID uuid.UUID) (bool, error) {
	info, err := e.providers.GetProviderInfo(ctx, providerID)
	if err != nil {
		return false, err
	}
	if !info.Active {
		return false, nil
	}

	// Check capacity
	ok, err := e.capacity.CheckAndIncrement(ctx, providerID, info.TPSLimit, info.DailyQuota, info.MonthlyQuota)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// RouteMessage routes a message using client's routing_mode to decide legacy vs new pipeline.
// Returns providerID. If routing_mode is "legacy", returns nil to signal legacy fallback.
func RouteMessage(ctx context.Context, client *shared.Client, operatorID uuid.UUID, engine *NewRoutingEngine) (*uuid.UUID, error) {
	switch client.RoutingMode {
	case "legacy":
		return nil, nil // signal to use legacy router
	case "new":
		return engine.SelectProvider(ctx, client.ID, operatorID)
	case "hybrid":
		providerID, err := engine.SelectProvider(ctx, client.ID, operatorID)
		if errors.Is(err, ErrNoRouteFound) {
			return nil, nil // fallback to legacy
		}
		return providerID, err
	default:
		return nil, nil // unknown mode, use legacy
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/routing/application/new_routing_engine.go
git commit -m "feat(routing): add new routing engine with priority/weighted strategies and capacity checks"
```

---

## Task 10: Extend Shared Models — Client RoutingMode

**Files:**
- Modify: `internal/shared/models.go`

- [ ] **Step 1: Add RoutingMode and CostVisibilityEnabled to Client struct**

Add after the existing `Client` struct fields (after `UpdatedAt`):

```go
// In the Client struct, add these fields:
	RoutingMode            string // "legacy", "new", "hybrid"
	CostVisibilityEnabled  bool
```

- [ ] **Step 2: Commit**

```bash
git add internal/shared/models.go
git commit -m "feat: add RoutingMode and CostVisibilityEnabled to shared Client model"
```

---

## Task 11: Provider Tarification Domain Models

**Files:**
- Create: `internal/services/tarification/domain/provider_tariff_plan.go`
- Create: `internal/services/tarification/domain/provider_tarification_log.go`
- Create: `internal/services/tarification/domain/provider_usage_counter.go`
- Create: `internal/services/tarification/domain/provider_repository.go`

- [ ] **Step 1: Write ProviderTariffPlan domain model + related structs**

```go
// internal/services/tarification/domain/provider_tariff_plan.go
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ProviderTariffPlan struct {
	ID         uuid.UUID
	ProviderID uuid.UUID
	OperatorID uuid.UUID
	Strategy   TarificationStrategy
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ProviderTariffPeriod struct {
	ID                    uuid.UUID
	ProviderTariffPlanID  uuid.UUID
	StartDate             time.Time
	EndDate               time.Time
	CreatedAt             time.Time
}

type ProviderTariffTier struct {
	ID                       uuid.UUID
	ProviderTariffPeriodID   uuid.UUID
	FromCount                int
	PricePerSegment          string
}

var (
	ErrProviderTariffPlanNotFound   = errors.New("provider tariff plan not found")
	ErrProviderTariffPeriodNotFound = errors.New("provider tariff period not found")
	ErrNoActiveProviderTariffPlan   = errors.New("no active provider tariff plan")
	ErrNoActiveProviderTariffPeriod = errors.New("no active provider tariff period")
)

func NewProviderTariffPlan(providerID, operatorID uuid.UUID, strategy TarificationStrategy) *ProviderTariffPlan {
	now := time.Now()
	return &ProviderTariffPlan{
		ID:         uuid.New(),
		ProviderID: providerID,
		OperatorID: operatorID,
		Strategy:   strategy,
		Active:     true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func (p *ProviderTariffPlan) Validate() error {
	if p.ProviderID == uuid.Nil {
		return ErrProviderTariffPlanNotFound
	}
	if p.OperatorID == uuid.Nil {
		return errors.New("operator_id is required")
	}
	switch p.Strategy {
	case StrategyFixed, StrategyThreshold, StrategyThresholdRecalc, StrategyPrepaidThreshold:
		return nil
	default:
		return ErrTariffPlanInvalidStrategy
	}
}
```

- [ ] **Step 2: Write ProviderTarificationLog domain model**

```go
// internal/services/tarification/domain/provider_tarification_log.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

type ProviderTarificationLog struct {
	ID                      uuid.UUID
	ProviderID              uuid.UUID
	OperatorID              uuid.UUID
	ClientID                uuid.UUID
	MessageID               uuid.UUID
	SegmentCount            int
	PricePerSegment         string
	TotalCost               string
	Strategy                TarificationStrategy
	ProviderTariffPlanID    uuid.UUID
	ProviderTariffPeriodID  uuid.UUID
	IdempotencyKey          string
	CreatedAt               time.Time
}

func NewProviderTarificationLog(
	providerID, operatorID, clientID, messageID, planID, periodID uuid.UUID,
	strategy TarificationStrategy,
	segmentCount int,
	pricePerSegment, totalCost string,
	idempotencyKey string,
) *ProviderTarificationLog {
	return &ProviderTarificationLog{
		ID:                     uuid.New(),
		ProviderID:             providerID,
		OperatorID:             operatorID,
		ClientID:               clientID,
		MessageID:              messageID,
		SegmentCount:           segmentCount,
		PricePerSegment:        pricePerSegment,
		TotalCost:              totalCost,
		Strategy:               strategy,
		ProviderTariffPlanID:   planID,
		ProviderTariffPeriodID: periodID,
		IdempotencyKey:         idempotencyKey,
		CreatedAt:              time.Now(),
	}
}
```

- [ ] **Step 3: Write ProviderUsageCounter domain model**

```go
// internal/services/tarification/domain/provider_usage_counter.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

type ProviderUsageCounter struct {
	ID                      uuid.UUID
	ProviderTariffPlanID    uuid.UUID
	ProviderTariffPeriodID  uuid.UUID
	SegmentCount            int
	UpdatedAt               time.Time
}

func NewProviderUsageCounter(planID, periodID uuid.UUID) *ProviderUsageCounter {
	return &ProviderUsageCounter{
		ID:                     uuid.New(),
		ProviderTariffPlanID:   planID,
		ProviderTariffPeriodID: periodID,
		SegmentCount:           0,
		UpdatedAt:              time.Now(),
	}
}
```

- [ ] **Step 4: Write provider tarification repository interfaces**

```go
// internal/services/tarification/domain/provider_repository.go
package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ProviderTariffPlanRepository interface {
	Create(ctx context.Context, plan *ProviderTariffPlan) error
	GetByID(ctx context.Context, id uuid.UUID) (*ProviderTariffPlan, error)
	GetActiveByProviderAndOperator(ctx context.Context, providerID, operatorID uuid.UUID) (*ProviderTariffPlan, error)
	Update(ctx context.Context, plan *ProviderTariffPlan) error
	List(ctx context.Context, providerID *uuid.UUID, activeOnly bool, limit, offset int) ([]*ProviderTariffPlan, int, error)
}

type ProviderTariffPeriodRepository interface {
	Create(ctx context.Context, period *ProviderTariffPeriod) error
	GetByID(ctx context.Context, id uuid.UUID) (*ProviderTariffPeriod, error)
	GetActiveByPlanID(ctx context.Context, planID uuid.UUID, now time.Time) (*ProviderTariffPeriod, error)
	ListByPlanID(ctx context.Context, planID uuid.UUID) ([]*ProviderTariffPeriod, error)
}

type ProviderTariffTierRepository interface {
	Create(ctx context.Context, tier *ProviderTariffTier) error
	GetByID(ctx context.Context, id uuid.UUID) (*ProviderTariffTier, error)
	Update(ctx context.Context, tier *ProviderTariffTier) error
	ListByPeriodID(ctx context.Context, periodID uuid.UUID) ([]*ProviderTariffTier, error)
}

type ProviderUsageCounterRepository interface {
	GetOrCreate(ctx context.Context, planID, periodID uuid.UUID) (*ProviderUsageCounter, error)
	IncrementAndGet(ctx context.Context, planID, periodID uuid.UUID, segments int) (*ProviderUsageCounter, error)
}

type ProviderTarificationLogRepository interface {
	Create(ctx context.Context, log *ProviderTarificationLog) error
	GetByIdempotencyKey(ctx context.Context, key string) (*ProviderTarificationLog, error)
}

// MarginReportEntry represents a single row in the margin report.
type MarginReportEntry struct {
	OperatorID   uuid.UUID
	OperatorName string
	ProviderID   uuid.UUID
	ProviderName string
	Segments     int
	Revenue      string
	Cost         string
	Margin       string
}

type MarginReportRepository interface {
	GetMarginReport(ctx context.Context, clientID uuid.UUID, from, to time.Time) ([]*MarginReportEntry, error)
}
```

- [ ] **Step 5: Commit**

```bash
git add internal/services/tarification/domain/provider_tariff_plan.go \
      internal/services/tarification/domain/provider_tarification_log.go \
      internal/services/tarification/domain/provider_usage_counter.go \
      internal/services/tarification/domain/provider_repository.go
git commit -m "feat(tarification): add provider tarification domain models and repository interfaces"
```

---

## Task 12: Provider Tarification Infrastructure — Repositories

**Files:**
- Create: `internal/services/tarification/infrastructure/provider_tariff_plan_repo.go`
- Create: `internal/services/tarification/infrastructure/provider_tariff_period_repo.go`
- Create: `internal/services/tarification/infrastructure/provider_tariff_tier_repo.go`
- Create: `internal/services/tarification/infrastructure/provider_usage_counter_repo.go`
- Create: `internal/services/tarification/infrastructure/provider_tarification_log_repo.go`
- Create: `internal/services/tarification/infrastructure/margin_report_repo.go`

- [ ] **Step 1: Write ProviderTariffPlan repository**

```go
// internal/services/tarification/infrastructure/provider_tariff_plan_repo.go
package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type ProviderTariffPlanRepo struct {
	pool *pgxpool.Pool
}

func NewProviderTariffPlanRepo(pool *pgxpool.Pool) *ProviderTariffPlanRepo {
	return &ProviderTariffPlanRepo{pool: pool}
}

func (r *ProviderTariffPlanRepo) Create(ctx context.Context, plan *domain.ProviderTariffPlan) error {
	query := `INSERT INTO provider_tariff_plans (id, provider_id, operator_id, strategy, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.pool.Exec(ctx, query, plan.ID, plan.ProviderID, plan.OperatorID, plan.Strategy, plan.Active, plan.CreatedAt, plan.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create provider_tariff_plan: %w", err)
	}
	return nil
}

func (r *ProviderTariffPlanRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderTariffPlan, error) {
	query := `SELECT id, provider_id, operator_id, strategy, active, created_at, updated_at FROM provider_tariff_plans WHERE id = $1`
	p := &domain.ProviderTariffPlan{}
	err := r.pool.QueryRow(ctx, query, id).Scan(&p.ID, &p.ProviderID, &p.OperatorID, &p.Strategy, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrProviderTariffPlanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get provider_tariff_plan: %w", err)
	}
	return p, nil
}

func (r *ProviderTariffPlanRepo) GetActiveByProviderAndOperator(ctx context.Context, providerID, operatorID uuid.UUID) (*domain.ProviderTariffPlan, error) {
	query := `SELECT id, provider_id, operator_id, strategy, active, created_at, updated_at
		FROM provider_tariff_plans WHERE provider_id = $1 AND operator_id = $2 AND active = true`
	p := &domain.ProviderTariffPlan{}
	err := r.pool.QueryRow(ctx, query, providerID, operatorID).Scan(&p.ID, &p.ProviderID, &p.OperatorID, &p.Strategy, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNoActiveProviderTariffPlan
	}
	if err != nil {
		return nil, fmt.Errorf("get active provider_tariff_plan: %w", err)
	}
	return p, nil
}

func (r *ProviderTariffPlanRepo) Update(ctx context.Context, plan *domain.ProviderTariffPlan) error {
	query := `UPDATE provider_tariff_plans SET strategy = $1, active = $2 WHERE id = $3`
	tag, err := r.pool.Exec(ctx, query, plan.Strategy, plan.Active, plan.ID)
	if err != nil {
		return fmt.Errorf("update provider_tariff_plan: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProviderTariffPlanNotFound
	}
	return nil
}

func (r *ProviderTariffPlanRepo) List(ctx context.Context, providerID *uuid.UUID, activeOnly bool, limit, offset int) ([]*domain.ProviderTariffPlan, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	argN := 1

	if providerID != nil {
		where += fmt.Sprintf(" AND provider_id = $%d", argN)
		args = append(args, *providerID)
		argN++
	}
	if activeOnly {
		where += " AND active = true"
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM provider_tariff_plans %s", where)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count provider_tariff_plans: %w", err)
	}

	query := fmt.Sprintf(`SELECT id, provider_id, operator_id, strategy, active, created_at, updated_at
		FROM provider_tariff_plans %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, argN, argN+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list provider_tariff_plans: %w", err)
	}
	defer rows.Close()

	var plans []*domain.ProviderTariffPlan
	for rows.Next() {
		p := &domain.ProviderTariffPlan{}
		if err := rows.Scan(&p.ID, &p.ProviderID, &p.OperatorID, &p.Strategy, &p.Active, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan provider_tariff_plan: %w", err)
		}
		plans = append(plans, p)
	}
	return plans, total, nil
}
```

- [ ] **Step 2: Write ProviderTariffPeriod repository**

```go
// internal/services/tarification/infrastructure/provider_tariff_period_repo.go
package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type ProviderTariffPeriodRepo struct {
	pool *pgxpool.Pool
}

func NewProviderTariffPeriodRepo(pool *pgxpool.Pool) *ProviderTariffPeriodRepo {
	return &ProviderTariffPeriodRepo{pool: pool}
}

func (r *ProviderTariffPeriodRepo) Create(ctx context.Context, period *domain.ProviderTariffPeriod) error {
	query := `INSERT INTO provider_tariff_periods (id, provider_tariff_plan_id, start_date, end_date, created_at)
		VALUES ($1, $2, $3, $4, $5)`
	_, err := r.pool.Exec(ctx, query, period.ID, period.ProviderTariffPlanID, period.StartDate, period.EndDate, period.CreatedAt)
	if err != nil {
		return fmt.Errorf("create provider_tariff_period: %w", err)
	}
	return nil
}

func (r *ProviderTariffPeriodRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderTariffPeriod, error) {
	query := `SELECT id, provider_tariff_plan_id, start_date, end_date, created_at FROM provider_tariff_periods WHERE id = $1`
	p := &domain.ProviderTariffPeriod{}
	err := r.pool.QueryRow(ctx, query, id).Scan(&p.ID, &p.ProviderTariffPlanID, &p.StartDate, &p.EndDate, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrProviderTariffPeriodNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get provider_tariff_period: %w", err)
	}
	return p, nil
}

func (r *ProviderTariffPeriodRepo) GetActiveByPlanID(ctx context.Context, planID uuid.UUID, now time.Time) (*domain.ProviderTariffPeriod, error) {
	query := `SELECT id, provider_tariff_plan_id, start_date, end_date, created_at
		FROM provider_tariff_periods WHERE provider_tariff_plan_id = $1 AND start_date <= $2 AND end_date >= $2`
	p := &domain.ProviderTariffPeriod{}
	err := r.pool.QueryRow(ctx, query, planID, now).Scan(&p.ID, &p.ProviderTariffPlanID, &p.StartDate, &p.EndDate, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNoActiveProviderTariffPeriod
	}
	if err != nil {
		return nil, fmt.Errorf("get active provider_tariff_period: %w", err)
	}
	return p, nil
}

func (r *ProviderTariffPeriodRepo) ListByPlanID(ctx context.Context, planID uuid.UUID) ([]*domain.ProviderTariffPeriod, error) {
	query := `SELECT id, provider_tariff_plan_id, start_date, end_date, created_at
		FROM provider_tariff_periods WHERE provider_tariff_plan_id = $1 ORDER BY start_date`
	rows, err := r.pool.Query(ctx, query, planID)
	if err != nil {
		return nil, fmt.Errorf("list provider_tariff_periods: %w", err)
	}
	defer rows.Close()

	var result []*domain.ProviderTariffPeriod
	for rows.Next() {
		p := &domain.ProviderTariffPeriod{}
		if err := rows.Scan(&p.ID, &p.ProviderTariffPlanID, &p.StartDate, &p.EndDate, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan provider_tariff_period: %w", err)
		}
		result = append(result, p)
	}
	return result, nil
}
```

- [ ] **Step 3: Write ProviderTariffTier, ProviderUsageCounter, ProviderTarificationLog, and MarginReport repositories**

```go
// internal/services/tarification/infrastructure/provider_tariff_tier_repo.go
package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type ProviderTariffTierRepo struct {
	pool *pgxpool.Pool
}

func NewProviderTariffTierRepo(pool *pgxpool.Pool) *ProviderTariffTierRepo {
	return &ProviderTariffTierRepo{pool: pool}
}

func (r *ProviderTariffTierRepo) Create(ctx context.Context, tier *domain.ProviderTariffTier) error {
	query := `INSERT INTO provider_tariff_tiers (id, provider_tariff_period_id, from_count, price_per_segment)
		VALUES ($1, $2, $3, $4)`
	_, err := r.pool.Exec(ctx, query, tier.ID, tier.ProviderTariffPeriodID, tier.FromCount, tier.PricePerSegment)
	if err != nil {
		return fmt.Errorf("create provider_tariff_tier: %w", err)
	}
	return nil
}

func (r *ProviderTariffTierRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderTariffTier, error) {
	query := `SELECT id, provider_tariff_period_id, from_count, price_per_segment FROM provider_tariff_tiers WHERE id = $1`
	t := &domain.ProviderTariffTier{}
	err := r.pool.QueryRow(ctx, query, id).Scan(&t.ID, &t.ProviderTariffPeriodID, &t.FromCount, &t.PricePerSegment)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("provider tariff tier not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get provider_tariff_tier: %w", err)
	}
	return t, nil
}

func (r *ProviderTariffTierRepo) Update(ctx context.Context, tier *domain.ProviderTariffTier) error {
	query := `UPDATE provider_tariff_tiers SET from_count = $1, price_per_segment = $2 WHERE id = $3`
	tag, err := r.pool.Exec(ctx, query, tier.FromCount, tier.PricePerSegment, tier.ID)
	if err != nil {
		return fmt.Errorf("update provider_tariff_tier: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("provider tariff tier not found")
	}
	return nil
}

func (r *ProviderTariffTierRepo) ListByPeriodID(ctx context.Context, periodID uuid.UUID) ([]*domain.ProviderTariffTier, error) {
	query := `SELECT id, provider_tariff_period_id, from_count, price_per_segment
		FROM provider_tariff_tiers WHERE provider_tariff_period_id = $1 ORDER BY from_count ASC`
	rows, err := r.pool.Query(ctx, query, periodID)
	if err != nil {
		return nil, fmt.Errorf("list provider_tariff_tiers: %w", err)
	}
	defer rows.Close()

	var result []*domain.ProviderTariffTier
	for rows.Next() {
		t := &domain.ProviderTariffTier{}
		if err := rows.Scan(&t.ID, &t.ProviderTariffPeriodID, &t.FromCount, &t.PricePerSegment); err != nil {
			return nil, fmt.Errorf("scan provider_tariff_tier: %w", err)
		}
		result = append(result, t)
	}
	return result, nil
}
```

```go
// internal/services/tarification/infrastructure/provider_usage_counter_repo.go
package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type ProviderUsageCounterRepo struct {
	pool *pgxpool.Pool
}

func NewProviderUsageCounterRepo(pool *pgxpool.Pool) *ProviderUsageCounterRepo {
	return &ProviderUsageCounterRepo{pool: pool}
}

func (r *ProviderUsageCounterRepo) GetOrCreate(ctx context.Context, planID, periodID uuid.UUID) (*domain.ProviderUsageCounter, error) {
	query := `SELECT id, provider_tariff_plan_id, provider_tariff_period_id, segment_count, updated_at
		FROM provider_usage_counters WHERE provider_tariff_plan_id = $1 AND provider_tariff_period_id = $2`
	c := &domain.ProviderUsageCounter{}
	err := r.pool.QueryRow(ctx, query, planID, periodID).Scan(&c.ID, &c.ProviderTariffPlanID, &c.ProviderTariffPeriodID, &c.SegmentCount, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		c = domain.NewProviderUsageCounter(planID, periodID)
		insertQ := `INSERT INTO provider_usage_counters (id, provider_tariff_plan_id, provider_tariff_period_id, segment_count, updated_at)
			VALUES ($1, $2, $3, $4, $5)`
		_, err = r.pool.Exec(ctx, insertQ, c.ID, c.ProviderTariffPlanID, c.ProviderTariffPeriodID, c.SegmentCount, c.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("create provider_usage_counter: %w", err)
		}
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get provider_usage_counter: %w", err)
	}
	return c, nil
}

func (r *ProviderUsageCounterRepo) IncrementAndGet(ctx context.Context, planID, periodID uuid.UUID, segments int) (*domain.ProviderUsageCounter, error) {
	query := `UPDATE provider_usage_counters SET segment_count = segment_count + $1
		WHERE provider_tariff_plan_id = $2 AND provider_tariff_period_id = $3
		RETURNING id, provider_tariff_plan_id, provider_tariff_period_id, segment_count, updated_at`
	c := &domain.ProviderUsageCounter{}
	err := r.pool.QueryRow(ctx, query, segments, planID, periodID).Scan(
		&c.ID, &c.ProviderTariffPlanID, &c.ProviderTariffPeriodID, &c.SegmentCount, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("increment provider_usage_counter: %w", err)
	}
	return c, nil
}
```

```go
// internal/services/tarification/infrastructure/provider_tarification_log_repo.go
package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type ProviderTarificationLogRepo struct {
	pool *pgxpool.Pool
}

func NewProviderTarificationLogRepo(pool *pgxpool.Pool) *ProviderTarificationLogRepo {
	return &ProviderTarificationLogRepo{pool: pool}
}

func (r *ProviderTarificationLogRepo) Create(ctx context.Context, log *domain.ProviderTarificationLog) error {
	query := `INSERT INTO provider_tarification_log
		(id, provider_id, operator_id, client_id, message_id, segment_count, price_per_segment, total_cost, strategy, provider_tariff_plan_id, provider_tariff_period_id, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	_, err := r.pool.Exec(ctx, query,
		log.ID, log.ProviderID, log.OperatorID, log.ClientID, log.MessageID,
		log.SegmentCount, log.PricePerSegment, log.TotalCost, log.Strategy,
		log.ProviderTariffPlanID, log.ProviderTariffPeriodID, log.IdempotencyKey, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create provider_tarification_log: %w", err)
	}
	return nil
}

func (r *ProviderTarificationLogRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.ProviderTarificationLog, error) {
	query := `SELECT id, provider_id, operator_id, client_id, message_id, segment_count, price_per_segment, total_cost, strategy, provider_tariff_plan_id, provider_tariff_period_id, idempotency_key, created_at
		FROM provider_tarification_log WHERE idempotency_key = $1`
	l := &domain.ProviderTarificationLog{}
	err := r.pool.QueryRow(ctx, query, key).Scan(
		&l.ID, &l.ProviderID, &l.OperatorID, &l.ClientID, &l.MessageID,
		&l.SegmentCount, &l.PricePerSegment, &l.TotalCost, &l.Strategy,
		&l.ProviderTariffPlanID, &l.ProviderTariffPeriodID, &l.IdempotencyKey, &l.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get provider_tarification_log by key: %w", err)
	}
	return l, nil
}
```

```go
// internal/services/tarification/infrastructure/margin_report_repo.go
package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type MarginReportRepo struct {
	pool *pgxpool.Pool
}

func NewMarginReportRepo(pool *pgxpool.Pool) *MarginReportRepo {
	return &MarginReportRepo{pool: pool}
}

func (r *MarginReportRepo) GetMarginReport(ctx context.Context, clientID uuid.UUID, from, to time.Time) ([]*domain.MarginReportEntry, error) {
	query := `
		SELECT
			COALESCE(rev.operator_id, cost.operator_id) AS operator_id,
			COALESCE(o.name, '') AS operator_name,
			COALESCE(cost.provider_id, '00000000-0000-0000-0000-000000000000') AS provider_id,
			COALESCE(p.name, '') AS provider_name,
			COALESCE(rev.segments, 0) + COALESCE(cost.segments, 0) AS segments,
			COALESCE(rev.revenue, 0) AS revenue,
			COALESCE(cost.total_cost, 0) AS cost,
			COALESCE(rev.revenue, 0) - COALESCE(cost.total_cost, 0) AS margin
		FROM (
			SELECT operator_id, SUM(segment_count) AS segments, SUM(total_amount) AS revenue
			FROM tarification_log
			WHERE client_id = $1 AND created_at >= $2 AND created_at < $3
			GROUP BY operator_id
		) rev
		FULL OUTER JOIN (
			SELECT operator_id, provider_id, SUM(segment_count) AS segments, SUM(total_cost) AS total_cost
			FROM provider_tarification_log
			WHERE client_id = $1 AND created_at >= $2 AND created_at < $3
			GROUP BY operator_id, provider_id
		) cost ON rev.operator_id = cost.operator_id
		LEFT JOIN operators o ON o.id = COALESCE(rev.operator_id, cost.operator_id)
		LEFT JOIN providers p ON p.id = cost.provider_id
		ORDER BY operator_name, provider_name`

	rows, err := r.pool.Query(ctx, query, clientID, from, to)
	if err != nil {
		return nil, fmt.Errorf("margin report query: %w", err)
	}
	defer rows.Close()

	var result []*domain.MarginReportEntry
	for rows.Next() {
		e := &domain.MarginReportEntry{}
		if err := rows.Scan(&e.OperatorID, &e.OperatorName, &e.ProviderID, &e.ProviderName, &e.Segments, &e.Revenue, &e.Cost, &e.Margin); err != nil {
			return nil, fmt.Errorf("scan margin report row: %w", err)
		}
		result = append(result, e)
	}
	return result, nil
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/services/tarification/infrastructure/provider_tariff_plan_repo.go \
      internal/services/tarification/infrastructure/provider_tariff_period_repo.go \
      internal/services/tarification/infrastructure/provider_tariff_tier_repo.go \
      internal/services/tarification/infrastructure/provider_usage_counter_repo.go \
      internal/services/tarification/infrastructure/provider_tarification_log_repo.go \
      internal/services/tarification/infrastructure/margin_report_repo.go
git commit -m "feat(tarification): add provider tarification PostgreSQL repositories"
```

---

## Task 13: Provider Tarification Service

**Files:**
- Create: `internal/services/tarification/application/provider_tarification_service.go`

- [ ] **Step 1: Write provider cost calculation service**

```go
// internal/services/tarification/application/provider_tarification_service.go
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// ProviderTarificationService handles provider cost accounting (Layer 2).
type ProviderTarificationService struct {
	planRepo    domain.ProviderTariffPlanRepository
	periodRepo  domain.ProviderTariffPeriodRepository
	tierRepo    domain.ProviderTariffTierRepository
	usageRepo   domain.ProviderUsageCounterRepository
	logRepo     domain.ProviderTarificationLogRepository
	strategies  map[domain.TarificationStrategy]BillingStrategy
}

func NewProviderTarificationService(
	planRepo domain.ProviderTariffPlanRepository,
	periodRepo domain.ProviderTariffPeriodRepository,
	tierRepo domain.ProviderTariffTierRepository,
	usageRepo domain.ProviderUsageCounterRepository,
	logRepo domain.ProviderTarificationLogRepository,
) *ProviderTarificationService {
	return &ProviderTarificationService{
		planRepo:   planRepo,
		periodRepo: periodRepo,
		tierRepo:   tierRepo,
		usageRepo:  usageRepo,
		logRepo:    logRepo,
		strategies: map[domain.TarificationStrategy]BillingStrategy{
			domain.StrategyFixed:            NewFixedStrategy(),
			domain.StrategyThreshold:        NewThresholdStrategy(),
			domain.StrategyThresholdRecalc:  NewThresholdRecalcStrategy(),
			domain.StrategyPrepaidThreshold: NewPrepaidThresholdStrategy(),
		},
	}
}

// TarifyProviderCostRequest is the input for provider cost accounting.
type TarifyProviderCostRequest struct {
	ProviderID     uuid.UUID
	OperatorID     uuid.UUID
	ClientID       uuid.UUID
	MessageID      uuid.UUID
	SegmentCount   int
	IdempotencyKey string
}

// TarifyProviderCost calculates and logs the provider cost for a sent message.
// This is called asynchronously after successful delivery.
func (s *ProviderTarificationService) TarifyProviderCost(ctx context.Context, req *TarifyProviderCostRequest) error {
	// 1. Idempotency check
	existing, err := s.logRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("idempotency check: %w", err)
	}
	if existing != nil {
		return nil // already processed
	}

	// 2. Find active provider tariff plan
	plan, err := s.planRepo.GetActiveByProviderAndOperator(ctx, req.ProviderID, req.OperatorID)
	if err != nil {
		log.Warn().Err(err).
			Str("provider_id", req.ProviderID.String()).
			Str("operator_id", req.OperatorID.String()).
			Msg("no active provider tariff plan, skipping cost accounting")
		return nil // no plan = no cost tracking (graceful)
	}

	// 3. Find active period
	now := time.Now()
	period, err := s.periodRepo.GetActiveByPlanID(ctx, plan.ID, now)
	if err != nil {
		log.Warn().Err(err).Str("plan_id", plan.ID.String()).Msg("no active provider tariff period")
		return nil
	}

	// 4. Get tiers
	tiers, err := s.tierRepo.ListByPeriodID(ctx, period.ID)
	if err != nil {
		return fmt.Errorf("get provider tiers: %w", err)
	}
	if len(tiers) == 0 {
		log.Warn().Str("period_id", period.ID.String()).Msg("no tiers for provider tariff period")
		return nil
	}

	// 5. Get usage counter
	counter, err := s.usageRepo.GetOrCreate(ctx, plan.ID, period.ID)
	if err != nil {
		return fmt.Errorf("get provider usage counter: %w", err)
	}

	// 6. Convert ProviderTariffTier to domain.TariffTier for strategy calculation
	domainTiers := make([]*domain.TariffTier, len(tiers))
	for i, t := range tiers {
		domainTiers[i] = &domain.TariffTier{
			ID:             t.ID,
			TariffPeriodID: t.ProviderTariffPeriodID,
			FromCount:      t.FromCount,
			PricePerSegment: t.PricePerSegment,
		}
	}

	// 7. Calculate cost using same strategies as client tarification
	strategy, ok := s.strategies[plan.Strategy]
	if !ok {
		return fmt.Errorf("unknown provider strategy: %s", plan.Strategy)
	}

	result, err := strategy.Calculate(ctx, CalculationParams{
		CurrentCount: counter.SegmentCount,
		SegmentCount: req.SegmentCount,
		Tiers:        domainTiers,
	})
	if err != nil {
		return fmt.Errorf("provider cost calculation: %w", err)
	}

	// 8. Increment usage counter
	if _, err := s.usageRepo.IncrementAndGet(ctx, plan.ID, period.ID, req.SegmentCount); err != nil {
		log.Error().Err(err).Msg("failed to increment provider usage counter")
	}

	// 9. Write to provider_tarification_log
	logEntry := domain.NewProviderTarificationLog(
		req.ProviderID, req.OperatorID, req.ClientID, req.MessageID,
		plan.ID, period.ID, plan.Strategy,
		req.SegmentCount, result.PricePerSegment, result.ChargeAmount,
		req.IdempotencyKey,
	)
	if err := s.logRepo.Create(ctx, logEntry); err != nil {
		return fmt.Errorf("create provider tarification log: %w", err)
	}

	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/tarification/application/provider_tarification_service.go
git commit -m "feat(tarification): add ProviderTarificationService for async cost accounting"
```

---

## Task 14: Proto Definitions — Routing Extensions

**Files:**
- Modify: `api/proto/routing/routing.proto`

- [ ] **Step 1: Add new message types and RPCs to routing.proto**

Add at the end of the `RoutingService` service definition (before the closing brace):

```protobuf
  // Client Providers
  rpc AssignProviderToClient(AssignProviderRequest) returns (ClientProviderProto);
  rpc RevokeProviderFromClient(RevokeProviderRequest) returns (google.protobuf.Empty);
  rpc ListClientProviders(ListClientProvidersRequest) returns (ListClientProvidersResponse);
  rpc UpdateClientProvider(UpdateClientProviderRequest) returns (ClientProviderProto);

  // Provider Sharing
  rpc ShareProviderWithChild(ShareProviderRequest) returns (ClientProviderProto);
  rpc RevokeSharedProvider(RevokeSharedProviderRequest) returns (google.protobuf.Empty);

  // Client Routes
  rpc CreateClientRoute(CreateClientRouteRequest) returns (ClientRouteProto);
  rpc UpdateClientRoute(UpdateClientRouteRequest) returns (ClientRouteProto);
  rpc DeleteClientRoute(DeleteClientRouteRequest) returns (google.protobuf.Empty);
  rpc ListClientRoutes(ListClientRoutesRequest) returns (ListClientRoutesResponse);

  // Routing Strategy
  rpc SetRoutingStrategy(SetRoutingStrategyRequest) returns (ClientRoutingStrategyProto);
  rpc GetRoutingStrategy(GetRoutingStrategyRequest) returns (ClientRoutingStrategyProto);
  rpc DeleteRoutingStrategy(DeleteRoutingStrategyRequest) returns (google.protobuf.Empty);
```

Add message definitions at the bottom of the file:

```protobuf
// Client Provider messages
message ClientProviderProto {
  string id = 1;
  string client_id = 2;
  string provider_id = 3;
  string ownership = 4;
  string source_client_id = 5;
  int32 shared_priority = 6;
  bool expose_cost = 7;
  bool expose_provider_name = 8;
  bool active = 9;
  google.protobuf.Timestamp created_at = 10;
  google.protobuf.Timestamp updated_at = 11;
}

message AssignProviderRequest {
  string client_id = 1;
  string provider_id = 2;
  string ownership = 3;
  int32 shared_priority = 4;
}

message RevokeProviderRequest {
  string client_id = 1;
  string provider_id = 2;
}

message ListClientProvidersRequest {
  string client_id = 1;
  bool active_only = 2;
}

message ListClientProvidersResponse {
  repeated ClientProviderProto providers = 1;
}

message UpdateClientProviderRequest {
  string id = 1;
  int32 shared_priority = 2;
  bool expose_cost = 3;
  bool expose_provider_name = 4;
  bool active = 5;
}

message ShareProviderRequest {
  string parent_client_id = 1;
  string child_client_id = 2;
  string provider_id = 3;
  bool expose_cost = 4;
  bool expose_provider_name = 5;
}

message RevokeSharedProviderRequest {
  string id = 1;
}

// Client Route messages
message ClientRouteProto {
  string id = 1;
  string client_id = 2;
  string operator_id = 3;
  string provider_id = 4;
  int32 priority = 5;
  int32 weight = 6;
  bool active = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
}

message CreateClientRouteRequest {
  string client_id = 1;
  string operator_id = 2;
  string provider_id = 3;
  int32 priority = 4;
  int32 weight = 5;
}

message UpdateClientRouteRequest {
  string id = 1;
  int32 priority = 2;
  int32 weight = 3;
  bool active = 4;
}

message DeleteClientRouteRequest {
  string id = 1;
}

message ListClientRoutesRequest {
  string client_id = 1;
  string operator_id = 2;
}

message ListClientRoutesResponse {
  repeated ClientRouteProto routes = 1;
}

// Routing Strategy messages
message ClientRoutingStrategyProto {
  string id = 1;
  string client_id = 2;
  string operator_id = 3;
  string strategy = 4;
  google.protobuf.Timestamp created_at = 5;
  google.protobuf.Timestamp updated_at = 6;
}

message SetRoutingStrategyRequest {
  string client_id = 1;
  string operator_id = 2;
  string strategy = 3;
}

message GetRoutingStrategyRequest {
  string client_id = 1;
  string operator_id = 2;
}

message DeleteRoutingStrategyRequest {
  string client_id = 1;
  string operator_id = 2;
}
```

- [ ] **Step 2: Regenerate Go protobuf code**

Run: `cd /home/magomed/projects/sms && make proto` (or the equivalent protoc command)

- [ ] **Step 3: Commit**

```bash
git add api/proto/routing/routing.proto api/proto/routingv1/
git commit -m "feat(routing): add gRPC proto definitions for client providers, routes, strategies"
```

---

## Task 15: Proto Definitions — Tarification Extensions

**Files:**
- Modify: `api/proto/tarification/tarification.proto`

- [ ] **Step 1: Add provider tariff and margin report RPCs and messages**

Add to `TarificationService` service definition:

```protobuf
  // Provider Tariff Plans
  rpc CreateProviderTariffPlan(CreateProviderTariffPlanRequest) returns (ProviderTariffPlanProto);
  rpc GetProviderTariffPlan(GetProviderTariffPlanRequest) returns (ProviderTariffPlanProto);
  rpc ListProviderTariffPlans(ListProviderTariffPlansRequest) returns (ListProviderTariffPlansResponse);
  rpc UpdateProviderTariffPlan(UpdateProviderTariffPlanRequest) returns (ProviderTariffPlanProto);

  rpc CreateProviderTariffPeriod(CreateProviderTariffPeriodRequest) returns (ProviderTariffPeriodProto);
  rpc CreateProviderTariffTier(CreateProviderTariffTierRequest) returns (ProviderTariffTierProto);
  rpc UpdateProviderTariffTier(UpdateProviderTariffTierRequest) returns (ProviderTariffTierProto);

  // Margin Analytics
  rpc GetMarginReport(MarginReportRequest) returns (MarginReportResponse);
```

Add message definitions:

```protobuf
// Provider Tariff Plan messages
message ProviderTariffPlanProto {
  string id = 1;
  string provider_id = 2;
  string operator_id = 3;
  string strategy = 4;
  bool active = 5;
  google.protobuf.Timestamp created_at = 6;
  google.protobuf.Timestamp updated_at = 7;
}

message CreateProviderTariffPlanRequest {
  string provider_id = 1;
  string operator_id = 2;
  string strategy = 3;
}

message GetProviderTariffPlanRequest {
  string id = 1;
}

message ListProviderTariffPlansRequest {
  string provider_id = 1;
  bool active_only = 2;
  int32 limit = 3;
  int32 offset = 4;
}

message ListProviderTariffPlansResponse {
  repeated ProviderTariffPlanProto plans = 1;
  int32 total = 2;
}

message UpdateProviderTariffPlanRequest {
  string id = 1;
  bool active = 2;
}

message ProviderTariffPeriodProto {
  string id = 1;
  string provider_tariff_plan_id = 2;
  string start_date = 3;
  string end_date = 4;
  google.protobuf.Timestamp created_at = 5;
}

message CreateProviderTariffPeriodRequest {
  string provider_tariff_plan_id = 1;
  string start_date = 2;
  string end_date = 3;
}

message ProviderTariffTierProto {
  string id = 1;
  string provider_tariff_period_id = 2;
  int32 from_count = 3;
  string price_per_segment = 4;
}

message CreateProviderTariffTierRequest {
  string provider_tariff_period_id = 1;
  int32 from_count = 2;
  string price_per_segment = 3;
}

message UpdateProviderTariffTierRequest {
  string id = 1;
  int32 from_count = 2;
  string price_per_segment = 3;
}

// Margin Report messages
message MarginReportRequest {
  string client_id = 1;
  string from_date = 2;
  string to_date = 3;
}

message MarginReportResponse {
  repeated MarginReportEntry entries = 1;
  string total_revenue = 2;
  string total_cost = 3;
  string total_margin = 4;
}

message MarginReportEntry {
  string operator_id = 1;
  string operator_name = 2;
  string provider_id = 3;
  string provider_name = 4;
  int32 segments = 5;
  string revenue = 6;
  string cost = 7;
  string margin = 8;
}
```

- [ ] **Step 2: Regenerate Go protobuf code**

Run: `cd /home/magomed/projects/sms && make proto`

- [ ] **Step 3: Commit**

```bash
git add api/proto/tarification/tarification.proto api/proto/tarificationv1/
git commit -m "feat(tarification): add gRPC proto definitions for provider tariffs and margin reports"
```

---

## Task 16: gRPC Server — Routing Extensions

**Files:**
- Modify: `internal/services/routing/grpc/server.go`

- [ ] **Step 1: Add new dependencies to Server struct and NewServer**

Add fields to `Server` struct:
```go
	clientProviderRepo  domain.ClientProviderRepository
	clientRouteRepo     domain.ClientRouteRepository
	clientStrategyRepo  domain.ClientRoutingStrategyRepository
```

Update `NewServer` to accept these parameters and assign them.

- [ ] **Step 2: Implement AssignProviderToClient RPC**

```go
func (s *Server) AssignProviderToClient(ctx context.Context, req *routingv1.AssignProviderRequest) (*routingv1.ClientProviderProto, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}
	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid provider_id: %v", err)
	}

	cp := domain.NewClientProvider(clientID, providerID, domain.ProviderOwnership(req.Ownership))
	cp.SharedPriority = int(req.SharedPriority)

	if err := cp.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	if err := s.clientProviderRepo.Create(ctx, cp); err != nil {
		return nil, status.Errorf(codes.Internal, "create failed: %v", err)
	}

	return clientProviderToProto(cp), nil
}
```

- [ ] **Step 3: Implement remaining client provider RPCs (RevokeProviderFromClient, ListClientProviders, UpdateClientProvider, ShareProviderWithChild, RevokeSharedProvider)**

Each follows the pattern: parse UUIDs from request, call repository, convert to proto response.

`ShareProviderWithChild` creates a `ClientProvider` with `ownership=inherited` and `source_client_id` set to the parent, after verifying the parent has the provider.

- [ ] **Step 4: Implement client route RPCs (CreateClientRoute, UpdateClientRoute, DeleteClientRoute, ListClientRoutes)**

`CreateClientRoute` must verify that the provider is assigned to the client via `clientProviderRepo.GetByClientAndProvider` (application-level constraint).

- [ ] **Step 5: Implement routing strategy RPCs (SetRoutingStrategy, GetRoutingStrategy, DeleteRoutingStrategy)**

- [ ] **Step 6: Add proto conversion helpers**

```go
func clientProviderToProto(cp *domain.ClientProvider) *routingv1.ClientProviderProto {
	proto := &routingv1.ClientProviderProto{
		Id:                 cp.ID.String(),
		ClientId:           cp.ClientID.String(),
		ProviderId:         cp.ProviderID.String(),
		Ownership:          string(cp.Ownership),
		SharedPriority:     int32(cp.SharedPriority),
		ExposeCost:         cp.ExposeCost,
		ExposeProviderName: cp.ExposeProviderName,
		Active:             cp.Active,
		CreatedAt:          timestamppb.New(cp.CreatedAt),
		UpdatedAt:          timestamppb.New(cp.UpdatedAt),
	}
	if cp.SourceClientID != nil {
		proto.SourceClientId = cp.SourceClientID.String()
	}
	return proto
}

func clientRouteToProto(r *domain.ClientRoute) *routingv1.ClientRouteProto {
	return &routingv1.ClientRouteProto{
		Id:         r.ID.String(),
		ClientId:   r.ClientID.String(),
		OperatorId: r.OperatorID.String(),
		ProviderId: r.ProviderID.String(),
		Priority:   int32(r.Priority),
		Weight:     int32(r.Weight),
		Active:     r.Active,
		CreatedAt:  timestamppb.New(r.CreatedAt),
		UpdatedAt:  timestamppb.New(r.UpdatedAt),
	}
}

func clientStrategyToProto(s *domain.ClientRoutingStrategy) *routingv1.ClientRoutingStrategyProto {
	proto := &routingv1.ClientRoutingStrategyProto{
		Id:        s.ID.String(),
		ClientId:  s.ClientID.String(),
		Strategy:  string(s.Strategy),
		CreatedAt: timestamppb.New(s.CreatedAt),
		UpdatedAt: timestamppb.New(s.UpdatedAt),
	}
	if s.OperatorID != nil {
		proto.OperatorId = s.OperatorID.String()
	}
	return proto
}
```

- [ ] **Step 7: Commit**

```bash
git add internal/services/routing/grpc/server.go
git commit -m "feat(routing): implement gRPC server for client providers, routes, and strategies"
```

---

## Task 17: gRPC Server — Tarification Extensions

**Files:**
- Modify: `internal/services/tarification/grpc/server.go`

- [ ] **Step 1: Add provider tarification dependencies to Server struct**

Add fields:
```go
	providerPlanRepo    domain.ProviderTariffPlanRepository
	providerPeriodRepo  domain.ProviderTariffPeriodRepository
	providerTierRepo    domain.ProviderTariffTierRepository
	marginRepo          domain.MarginReportRepository
	providerTarification *application.ProviderTarificationService
```

- [ ] **Step 2: Implement provider tariff plan CRUD RPCs**

Follow the exact pattern from existing `CreateTariffPlan`, `ListTariffPlans`, `UpdateTariffPlan` — parse request, call repo, return proto. Same for period and tier.

- [ ] **Step 3: Implement GetMarginReport RPC**

```go
func (s *Server) GetMarginReport(ctx context.Context, req *tarificationv1.MarginReportRequest) (*tarificationv1.MarginReportResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id")
	}
	from, err := time.Parse("2006-01-02", req.FromDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid from_date format (YYYY-MM-DD)")
	}
	to, err := time.Parse("2006-01-02", req.ToDate)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid to_date format (YYYY-MM-DD)")
	}

	entries, err := s.marginRepo.GetMarginReport(ctx, clientID, from, to)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "margin report: %v", err)
	}

	resp := &tarificationv1.MarginReportResponse{}
	var totalRevenue, totalCost, totalMargin float64
	for _, e := range entries {
		resp.Entries = append(resp.Entries, &tarificationv1.MarginReportEntry{
			OperatorId:   e.OperatorID.String(),
			OperatorName: e.OperatorName,
			ProviderId:   e.ProviderID.String(),
			ProviderName: e.ProviderName,
			Segments:     int32(e.Segments),
			Revenue:      e.Revenue,
			Cost:         e.Cost,
			Margin:       e.Margin,
		})
		// Parse and sum for totals
		// (use strconv.ParseFloat or decimal library as appropriate)
	}
	resp.TotalRevenue = fmt.Sprintf("%.6f", totalRevenue)
	resp.TotalCost = fmt.Sprintf("%.6f", totalCost)
	resp.TotalMargin = fmt.Sprintf("%.6f", totalMargin)

	return resp, nil
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/services/tarification/grpc/server.go
git commit -m "feat(tarification): implement gRPC server for provider tariffs and margin reports"
```

---

## Task 18: REST API Handlers

**Files:**
- Create: `internal/gateway/admin/handlers/client_routing.go`
- Modify: `internal/gateway/admin/router/router.go`

- [ ] **Step 1: Write REST handlers for client routing**

```go
// internal/gateway/admin/handlers/client_routing.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ClientRoutingHandlers handles HTTP requests for client routing management.
type ClientRoutingHandlers struct {
	routingClient      routingv1.RoutingServiceClient
	tarificationClient tarificationv1.TarificationServiceClient
}

func NewClientRoutingHandlers(
	routingClient routingv1.RoutingServiceClient,
	tarificationClient tarificationv1.TarificationServiceClient,
) *ClientRoutingHandlers {
	return &ClientRoutingHandlers{
		routingClient:      routingClient,
		tarificationClient: tarificationClient,
	}
}

// AssignProvider handles POST /admin/v1/clients/{id}/providers
func (h *ClientRoutingHandlers) AssignProvider(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	var req struct {
		ProviderID     string `json:"provider_id"`
		Ownership      string `json:"ownership"`
		SharedPriority int32  `json:"shared_priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.AssignProviderToClient(r.Context(), &routingv1.AssignProviderRequest{
		ClientId:       clientID,
		ProviderId:     req.ProviderID,
		Ownership:      req.Ownership,
		SharedPriority: req.SharedPriority,
	})
	if err != nil {
		log.Error().Err(err).Msg("assign provider to client failed")
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

// ListProviders handles GET /admin/v1/clients/{id}/providers
func (h *ClientRoutingHandlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	resp, err := h.routingClient.ListClientProviders(r.Context(), &routingv1.ListClientProvidersRequest{
		ClientId:   clientID,
		ActiveOnly: r.URL.Query().Get("active_only") == "true",
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// RevokeProvider handles DELETE /admin/v1/clients/{id}/providers/{pid}
func (h *ClientRoutingHandlers) RevokeProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	_, err := h.routingClient.RevokeProviderFromClient(r.Context(), &routingv1.RevokeProviderRequest{
		ClientId:   vars["id"],
		ProviderId: vars["pid"],
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ShareProvider handles POST /admin/v1/clients/{id}/providers/share
func (h *ClientRoutingHandlers) ShareProvider(w http.ResponseWriter, r *http.Request) {
	parentID := mux.Vars(r)["id"]
	var req struct {
		ChildClientID      string `json:"child_client_id"`
		ProviderID         string `json:"provider_id"`
		ExposeCost         bool   `json:"expose_cost"`
		ExposeProviderName bool   `json:"expose_provider_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.ShareProviderWithChild(r.Context(), &routingv1.ShareProviderRequest{
		ParentClientId:     parentID,
		ChildClientId:      req.ChildClientID,
		ProviderId:         req.ProviderID,
		ExposeCost:         req.ExposeCost,
		ExposeProviderName: req.ExposeProviderName,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

// RevokeShared handles DELETE /admin/v1/clients/{id}/providers/share/{sid}
func (h *ClientRoutingHandlers) RevokeShared(w http.ResponseWriter, r *http.Request) {
	sid := mux.Vars(r)["sid"]
	_, err := h.routingClient.RevokeSharedProvider(r.Context(), &routingv1.RevokeSharedProviderRequest{
		Id: sid,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateRoute handles POST /admin/v1/clients/{id}/routes
func (h *ClientRoutingHandlers) CreateRoute(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
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
		ClientId:   clientID,
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

// ListRoutes handles GET /admin/v1/clients/{id}/routes
func (h *ClientRoutingHandlers) ListRoutes(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	resp, err := h.routingClient.ListClientRoutes(r.Context(), &routingv1.ListClientRoutesRequest{
		ClientId:   clientID,
		OperatorId: r.URL.Query().Get("operator_id"),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// UpdateRoute handles PUT /admin/v1/clients/{id}/routes/{rid}
func (h *ClientRoutingHandlers) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	rid := mux.Vars(r)["rid"]
	var req struct {
		Priority int32 `json:"priority"`
		Weight   int32 `json:"weight"`
		Active   bool  `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.UpdateClientRoute(r.Context(), &routingv1.UpdateClientRouteRequest{
		Id:       rid,
		Priority: req.Priority,
		Weight:   req.Weight,
		Active:   req.Active,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// DeleteRoute handles DELETE /admin/v1/clients/{id}/routes/{rid}
func (h *ClientRoutingHandlers) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	rid := mux.Vars(r)["rid"]
	_, err := h.routingClient.DeleteClientRoute(r.Context(), &routingv1.DeleteClientRouteRequest{
		Id: rid,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetStrategy handles PUT /admin/v1/clients/{id}/routing-strategy
func (h *ClientRoutingHandlers) SetStrategy(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	var req struct {
		OperatorID string `json:"operator_id"`
		Strategy   string `json:"strategy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.routingClient.SetRoutingStrategy(r.Context(), &routingv1.SetRoutingStrategyRequest{
		ClientId:   clientID,
		OperatorId: req.OperatorID,
		Strategy:   req.Strategy,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// GetStrategy handles GET /admin/v1/clients/{id}/routing-strategy
func (h *ClientRoutingHandlers) GetStrategy(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	resp, err := h.routingClient.GetRoutingStrategy(r.Context(), &routingv1.GetRoutingStrategyRequest{
		ClientId:   clientID,
		OperatorId: r.URL.Query().Get("operator_id"),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// GetMarginReport handles GET /admin/v1/clients/{id}/analytics/margin
func (h *ClientRoutingHandlers) GetMarginReport(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	resp, err := h.tarificationClient.GetMarginReport(r.Context(), &tarificationv1.MarginReportRequest{
		ClientId: clientID,
		FromDate: r.URL.Query().Get("from"),
		ToDate:   r.URL.Query().Get("to"),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Register new routes in admin router**

Add to `internal/gateway/admin/router/router.go` — add `clientRoutingHandlers *handlers.ClientRoutingHandlers` parameter to `SetupRouter` and register routes:

```go
	// Client Routing endpoints (nested under clients)
	clientRouting := clients.PathPrefix("/{id}").Subrouter()
	clientRouting.HandleFunc("/providers", clientRoutingHandlers.AssignProvider).Methods("POST")
	clientRouting.HandleFunc("/providers", clientRoutingHandlers.ListProviders).Methods("GET")
	clientRouting.HandleFunc("/providers/{pid}", clientRoutingHandlers.RevokeProvider).Methods("DELETE")
	clientRouting.HandleFunc("/providers/share", clientRoutingHandlers.ShareProvider).Methods("POST")
	clientRouting.HandleFunc("/providers/share/{sid}", clientRoutingHandlers.RevokeShared).Methods("DELETE")
	clientRouting.HandleFunc("/routes", clientRoutingHandlers.CreateRoute).Methods("POST")
	clientRouting.HandleFunc("/routes", clientRoutingHandlers.ListRoutes).Methods("GET")
	clientRouting.HandleFunc("/routes/{rid}", clientRoutingHandlers.UpdateRoute).Methods("PUT")
	clientRouting.HandleFunc("/routes/{rid}", clientRoutingHandlers.DeleteRoute).Methods("DELETE")
	clientRouting.HandleFunc("/routing-strategy", clientRoutingHandlers.SetStrategy).Methods("PUT")
	clientRouting.HandleFunc("/routing-strategy", clientRoutingHandlers.GetStrategy).Methods("GET")
	clientRouting.HandleFunc("/analytics/margin", clientRoutingHandlers.GetMarginReport).Methods("GET")
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/admin/handlers/client_routing.go internal/gateway/admin/router/router.go
git commit -m "feat(api): add REST handlers for client routing, providers, strategies, and margin reports"
```

---

## Task 19: Service Wiring — Routing Service

**Files:**
- Modify: `cmd/services/routing-service/main.go`

- [ ] **Step 1: Wire new repositories and pass to gRPC server**

In `main()`, after existing repo initialization, add:

```go
	// New routing repositories
	clientProviderRepo := infrastructure.NewClientProviderRepo(pool)
	clientRouteRepo := infrastructure.NewClientRouteRepo(pool)
	clientStrategyRepo := infrastructure.NewClientRoutingStrategyRepo(pool)
	capacityTracker := infrastructure.NewCapacityTracker(redisClient)
```

Update `NewServer` call to pass the new repos.

- [ ] **Step 2: Commit**

```bash
git add cmd/services/routing-service/main.go
git commit -m "feat(routing): wire new routing repositories in routing-service main"
```

---

## Task 20: Service Wiring — Tarification Service

**Files:**
- Modify: `cmd/services/tarification-service/main.go`

- [ ] **Step 1: Wire provider tarification repositories and service**

In `main()`, after existing repo initialization, add:

```go
	// Provider tarification repositories
	providerPlanRepo := infrastructure.NewProviderTariffPlanRepo(pool)
	providerPeriodRepo := infrastructure.NewProviderTariffPeriodRepo(pool)
	providerTierRepo := infrastructure.NewProviderTariffTierRepo(pool)
	providerUsageRepo := infrastructure.NewProviderUsageCounterRepo(pool)
	providerLogRepo := infrastructure.NewProviderTarificationLogRepo(pool)
	marginRepo := infrastructure.NewMarginReportRepo(pool)

	providerTarificationService := application.NewProviderTarificationService(
		providerPlanRepo, providerPeriodRepo, providerTierRepo,
		providerUsageRepo, providerLogRepo,
	)
```

Update gRPC server constructor to pass new repos and service.

- [ ] **Step 2: Commit**

```bash
git add cmd/services/tarification-service/main.go
git commit -m "feat(tarification): wire provider tarification repos and service in tarification-service main"
```

---

## Task 21: Build Verification

- [ ] **Step 1: Run Go build**

Run: `cd /home/magomed/projects/sms && go build ./...`
Expected: Build succeeds with no errors

- [ ] **Step 2: Run existing tests**

Run: `cd /home/magomed/projects/sms && go test ./internal/... -count=1 -short`
Expected: All existing tests pass

- [ ] **Step 3: Commit any fixups if needed**

```bash
git add -A
git commit -m "fix: resolve build issues from advanced routing integration"
```

---

## Task 22: Admin Gateway Wiring

**Files:**
- Modify: `cmd/admin-gateway/main.go`

- [ ] **Step 1: Create ClientRoutingHandlers and pass to SetupRouter**

```go
	clientRoutingHandlers := handlers.NewClientRoutingHandlers(routingClient, tarificationClient)
```

Add `clientRoutingHandlers` to the `SetupRouter` call.

- [ ] **Step 2: Commit**

```bash
git add cmd/admin-gateway/main.go
git commit -m "feat(api): wire ClientRoutingHandlers in admin-gateway"
```
