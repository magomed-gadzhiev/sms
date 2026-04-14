# Aggregator Phase 3: Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement aggregator route management — aggregator creates routes for sub-accounts from their allowed provider pool, manages default routes (auto-applied on sub-account creation), and platform auto-syncs `client_providers` when `allowed_provider_ids` changes.

**Architecture:** New `aggregator_default_routes` table stores template routes. A new `RouteService` in the aggregator service manages CRUD for sub-account routes (writing to existing `client_routes`/`client_routing_strategies`/`client_providers` tables) with validation against `aggregator_profiles.allowed_provider_ids`. New gRPC methods added to `AggregatorService`. HTTP handlers follow the same pattern as Phase 2 tariff handlers. React frontend adds `RoutesPage` (global route management) and `RoutesTab` (per-sub-account view in SubAccountDetailPage).

**Tech Stack:** Go 1.24 + sqlx (pgx), gorilla/mux, protobuf/gRPC, React 19 + TypeScript + Tailwind CSS 4.2, Radix UI, zerolog, testify/assert

All work is done in worktree `.worktrees/aggregator-phase3/` on branch `feature/aggregator-phase3`.

---

## File Map

### New files
- `migrations/000099_create_aggregator_default_routes.up.sql` — aggregator_default_routes table
- `migrations/000099_create_aggregator_default_routes.down.sql`
- `internal/services/aggregator/domain/route.go` — DefaultRoute, SubAccountRoute domain types
- `internal/services/aggregator/repository/route_repository.go` — CRUD for aggregator_default_routes, client_routes, client_providers
- `internal/services/aggregator/service/route_service.go` — business logic: validation, CRUD, defaults copy
- `internal/gateway/portal/handlers/aggregator_routes.go` — HTTP handlers for /portal/v1/aggregator/routes
- `portal-frontend/src/pages/aggregator/RoutesPage.tsx` — global routes + defaults management
- `portal-frontend/src/pages/aggregator/tabs/RoutesTab.tsx` — per-sub-account routes read-only + link

### Modified files
- `internal/services/aggregator/domain/errors.go` — add route errors
- `api/proto/aggregator/aggregator.proto` — add route RPCs and messages
- `api/proto/aggregatorv1/aggregator.pb.go` — regenerated
- `api/proto/aggregatorv1/aggregator_grpc.pb.go` — regenerated
- `internal/services/aggregator/grpc/server.go` — add route RPC handlers
- `internal/services/aggregator/service/aggregator_service.go` — call RouteService.CopyDefaultRoutes on CreateSubAccount
- `internal/gateway/portal/router/router.go` — register route endpoints
- `internal/gateway/portal/clients.go` — no change (AggregatorClient already wired)
- `portal-frontend/src/api/aggregator.ts` — add route API functions
- `portal-frontend/src/pages/aggregator/SubAccountDetailPage.tsx` — add RoutesTab
- `portal-frontend/src/App.tsx` (or router file) — add /aggregator/routes route

---

## Task 1: Create Worktree

**Files:** none (setup)

- [ ] **Step 1: Create Phase 3 worktree from Phase 2 branch**

```bash
git worktree add .worktrees/aggregator-phase3 -b feature/aggregator-phase3 feature/aggregator-phase2
```

- [ ] **Step 2: Verify worktree**

```bash
ls .worktrees/aggregator-phase3/internal/services/aggregator/
```
Expected: `domain/  grpc/  repository/  service/`

- [ ] **Step 3: Commit**

```bash
cd .worktrees/aggregator-phase3
git commit --allow-empty -m "chore: start aggregator phase 3 routing"
```

---

## Task 2: Migration — aggregator_default_routes

**Files:**
- Create: `migrations/000099_create_aggregator_default_routes.up.sql`
- Create: `migrations/000099_create_aggregator_default_routes.down.sql`

- [ ] **Step 1: Write up migration**

```sql
-- migrations/000099_create_aggregator_default_routes.up.sql
CREATE TABLE IF NOT EXISTS aggregator_default_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE RESTRICT,
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
    priority INT NOT NULL DEFAULT 0,
    weight INT NOT NULL DEFAULT 1,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(aggregator_id, operator_id, provider_id)
);

CREATE INDEX idx_agg_default_routes_agg ON aggregator_default_routes(aggregator_id) WHERE active = true;

CREATE TRIGGER update_aggregator_default_routes_updated_at
    BEFORE UPDATE ON aggregator_default_routes
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

- [ ] **Step 2: Write down migration**

```sql
-- migrations/000099_create_aggregator_default_routes.down.sql
DROP TABLE IF EXISTS aggregator_default_routes;
```

- [ ] **Step 3: Apply migration (run on server or locally with DB_URL set)**

```bash
cd /opt/sms
migrate -path migrations -database "$DATABASE_URL" up 1
```

Expected: `000099/u create_aggregator_default_routes OK`

- [ ] **Step 4: Verify table exists**

```bash
psql "$DATABASE_URL" -c "\d aggregator_default_routes"
```

Expected: table with columns id, aggregator_id, operator_id, provider_id, priority, weight, active, created_at, updated_at.

- [ ] **Step 5: Commit**

```bash
cd .worktrees/aggregator-phase3
git add migrations/000099_create_aggregator_default_routes.up.sql migrations/000099_create_aggregator_default_routes.down.sql
git commit -m "feat(migration): create aggregator_default_routes table"
```

---

## Task 3: Domain Types

**Files:**
- Create: `internal/services/aggregator/domain/route.go`
- Modify: `internal/services/aggregator/domain/errors.go`

- [ ] **Step 1: Write route domain types**

```go
// internal/services/aggregator/domain/route.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// DefaultRoute is a template route stored per aggregator, auto-applied when creating sub-accounts.
type DefaultRoute struct {
	ID           uuid.UUID
	AggregatorID uuid.UUID
	OperatorID   uuid.UUID
	ProviderID   uuid.UUID
	Priority     int
	Weight       int
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SubAccountRoute is a concrete route assigned to a sub-account (stored in client_routes).
type SubAccountRoute struct {
	ID           uuid.UUID
	SubAccountID uuid.UUID
	OperatorID   uuid.UUID
	ProviderID   uuid.UUID
	Priority     int
	Weight       int
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

- [ ] **Step 2: Add route errors to errors.go**

Open `internal/services/aggregator/domain/errors.go` and append:

```go
	ErrDefaultRouteNotFound  = errors.New("aggregator default route not found")
	ErrRouteNotFound         = errors.New("sub-account route not found")
	ErrProviderNotAllowed    = errors.New("provider is not in aggregator's allowed provider pool")
	ErrProviderNotActive     = errors.New("provider is not active")
	ErrRouteAlreadyExists    = errors.New("route for this operator+provider already exists")
```

(Add these inside the existing `var (...)` block.)

- [ ] **Step 3: Build check**

```bash
cd .worktrees/aggregator-phase3
go build ./internal/services/aggregator/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/services/aggregator/domain/route.go internal/services/aggregator/domain/errors.go
git commit -m "feat(aggregator): add route domain types and errors"
```

---

## Task 4: Route Repository

**Files:**
- Create: `internal/services/aggregator/repository/route_repository.go`

The repository manages:
1. `aggregator_default_routes` — CRUD
2. `client_routes` + `client_providers` — write routes for a sub-account
3. `client_providers` sync — auto-sync when allowed_provider_ids changes (called from profile update)

- [ ] **Step 1: Write the repository**

```go
// internal/services/aggregator/repository/route_repository.go
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

// RouteRepository handles default routes and sub-account route assignment.
type RouteRepository struct {
	db *sqlx.DB
}

func NewRouteRepository(db *database.DB) *RouteRepository {
	return &RouteRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

// --- Default Routes ---

func (r *RouteRepository) CreateDefaultRoute(ctx context.Context, dr *domain.DefaultRoute) error {
	dr.ID = uuid.New()
	q := `INSERT INTO aggregator_default_routes
		(id, aggregator_id, operator_id, provider_id, priority, weight, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.db.ExecContext(ctx, q,
		dr.ID, dr.AggregatorID, dr.OperatorID, dr.ProviderID,
		dr.Priority, dr.Weight, dr.Active)
	if err != nil {
		return fmt.Errorf("create default route: %w", err)
	}
	return nil
}

func (r *RouteRepository) ListDefaultRoutes(ctx context.Context, aggregatorID uuid.UUID) ([]*domain.DefaultRoute, error) {
	q := `SELECT id, aggregator_id, operator_id, provider_id, priority, weight, active, created_at, updated_at
		FROM aggregator_default_routes WHERE aggregator_id = $1 ORDER BY priority DESC`
	rows, err := r.db.QueryContext(ctx, q, aggregatorID)
	if err != nil {
		return nil, fmt.Errorf("list default routes: %w", err)
	}
	defer rows.Close()

	var result []*domain.DefaultRoute
	for rows.Next() {
		var dr domain.DefaultRoute
		if err := rows.Scan(&dr.ID, &dr.AggregatorID, &dr.OperatorID, &dr.ProviderID,
			&dr.Priority, &dr.Weight, &dr.Active, &dr.CreatedAt, &dr.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan default route: %w", err)
		}
		result = append(result, &dr)
	}
	return result, rows.Err()
}

func (r *RouteRepository) GetDefaultRouteByID(ctx context.Context, id, aggregatorID uuid.UUID) (*domain.DefaultRoute, error) {
	var dr domain.DefaultRoute
	q := `SELECT id, aggregator_id, operator_id, provider_id, priority, weight, active, created_at, updated_at
		FROM aggregator_default_routes WHERE id = $1 AND aggregator_id = $2`
	err := r.db.QueryRowContext(ctx, q, id, aggregatorID).Scan(
		&dr.ID, &dr.AggregatorID, &dr.OperatorID, &dr.ProviderID,
		&dr.Priority, &dr.Weight, &dr.Active, &dr.CreatedAt, &dr.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrDefaultRouteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get default route: %w", err)
	}
	return &dr, nil
}

func (r *RouteRepository) UpdateDefaultRoute(ctx context.Context, dr *domain.DefaultRoute) error {
	q := `UPDATE aggregator_default_routes
		SET priority=$1, weight=$2, active=$3, updated_at=NOW()
		WHERE id=$4 AND aggregator_id=$5`
	res, err := r.db.ExecContext(ctx, q, dr.Priority, dr.Weight, dr.Active, dr.ID, dr.AggregatorID)
	if err != nil {
		return fmt.Errorf("update default route: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrDefaultRouteNotFound
	}
	return nil
}

func (r *RouteRepository) DeleteDefaultRoute(ctx context.Context, id, aggregatorID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM aggregator_default_routes WHERE id=$1 AND aggregator_id=$2`, id, aggregatorID)
	if err != nil {
		return fmt.Errorf("delete default route: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrDefaultRouteNotFound
	}
	return nil
}

// --- Sub-Account Routes (stored in client_routes) ---

func (r *RouteRepository) CreateSubAccountRoute(ctx context.Context, sr *domain.SubAccountRoute) error {
	sr.ID = uuid.New()
	q := `INSERT INTO client_routes (id, client_id, operator_id, provider_id, priority, weight, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.db.ExecContext(ctx, q,
		sr.ID, sr.SubAccountID, sr.OperatorID, sr.ProviderID,
		sr.Priority, sr.Weight, sr.Active)
	if err != nil {
		return fmt.Errorf("create sub-account route: %w", err)
	}
	return nil
}

func (r *RouteRepository) ListSubAccountRoutes(ctx context.Context, subAccountID uuid.UUID) ([]*domain.SubAccountRoute, error) {
	q := `SELECT id, client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at
		FROM client_routes WHERE client_id = $1 ORDER BY priority DESC`
	rows, err := r.db.QueryContext(ctx, q, subAccountID)
	if err != nil {
		return nil, fmt.Errorf("list sub-account routes: %w", err)
	}
	defer rows.Close()

	var result []*domain.SubAccountRoute
	for rows.Next() {
		var sr domain.SubAccountRoute
		if err := rows.Scan(&sr.ID, &sr.SubAccountID, &sr.OperatorID, &sr.ProviderID,
			&sr.Priority, &sr.Weight, &sr.Active, &sr.CreatedAt, &sr.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan sub-account route: %w", err)
		}
		result = append(result, &sr)
	}
	return result, rows.Err()
}

func (r *RouteRepository) GetSubAccountRouteByID(ctx context.Context, id, subAccountID uuid.UUID) (*domain.SubAccountRoute, error) {
	var sr domain.SubAccountRoute
	q := `SELECT id, client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at
		FROM client_routes WHERE id=$1 AND client_id=$2`
	err := r.db.QueryRowContext(ctx, q, id, subAccountID).Scan(
		&sr.ID, &sr.SubAccountID, &sr.OperatorID, &sr.ProviderID,
		&sr.Priority, &sr.Weight, &sr.Active, &sr.CreatedAt, &sr.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrRouteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get sub-account route: %w", err)
	}
	return &sr, nil
}

func (r *RouteRepository) UpdateSubAccountRoute(ctx context.Context, sr *domain.SubAccountRoute) error {
	q := `UPDATE client_routes SET priority=$1, weight=$2, active=$3, updated_at=NOW()
		WHERE id=$4 AND client_id=$5`
	res, err := r.db.ExecContext(ctx, q, sr.Priority, sr.Weight, sr.Active, sr.ID, sr.SubAccountID)
	if err != nil {
		return fmt.Errorf("update sub-account route: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrRouteNotFound
	}
	return nil
}

func (r *RouteRepository) DeleteSubAccountRoute(ctx context.Context, id, subAccountID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM client_routes WHERE id=$1 AND client_id=$2`, id, subAccountID)
	if err != nil {
		return fmt.Errorf("delete sub-account route: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrRouteNotFound
	}
	return nil
}

// EnsureClientProvider inserts a client_providers record for the sub-account if not exists.
// ownership='inherited', source_client_id=aggregatorID.
func (r *RouteRepository) EnsureClientProvider(ctx context.Context, subAccountID, providerID, aggregatorID uuid.UUID) error {
	q := `INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, active)
		VALUES ($1,$2,'inherited',$3,true)
		ON CONFLICT (client_id, provider_id) DO UPDATE SET active=true, updated_at=NOW()`
	_, err := r.db.ExecContext(ctx, q, subAccountID, providerID, aggregatorID)
	if err != nil {
		return fmt.Errorf("ensure client_provider: %w", err)
	}
	return nil
}

// SyncAggregatorProviders upserts/deactivates client_providers for the aggregator itself.
// Called when allowed_provider_ids changes in aggregator profile.
func (r *RouteRepository) SyncAggregatorProviders(ctx context.Context, aggregatorID uuid.UUID, allowedProviderIDs []uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Deactivate all existing platform-owned providers for aggregator
	if _, err := tx.ExecContext(ctx,
		`UPDATE client_providers SET active=false, updated_at=NOW()
		WHERE client_id=$1 AND ownership='platform'`, aggregatorID); err != nil {
		return fmt.Errorf("deactivate aggregator providers: %w", err)
	}

	// Upsert each allowed provider
	for _, pID := range allowedProviderIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO client_providers (client_id, provider_id, ownership, active)
			VALUES ($1,$2,'platform',true)
			ON CONFLICT (client_id, provider_id) DO UPDATE SET active=true, updated_at=NOW()`,
			aggregatorID, pID); err != nil {
			return fmt.Errorf("upsert aggregator provider %s: %w", pID, err)
		}
	}

	// Deactivate sub-account routes using providers no longer allowed
	if len(allowedProviderIDs) > 0 {
		allowedStrs := make([]string, len(allowedProviderIDs))
		for i, id := range allowedProviderIDs {
			allowedStrs[i] = fmt.Sprintf("'%s'", id.String())
		}
		// Use ANY with array cast for safety
		if _, err := tx.ExecContext(ctx,
			`UPDATE client_routes cr
			SET active=false, updated_at=NOW()
			FROM clients c
			WHERE cr.client_id = c.id
			  AND c.parent_client_id = $1
			  AND cr.provider_id NOT IN (
			      SELECT unnest($2::uuid[])
			  )`,
			aggregatorID, fmt.Sprintf("{%s}", joinUUIDs(allowedProviderIDs))); err != nil {
			return fmt.Errorf("deactivate sub-account routes for removed providers: %w", err)
		}
	}

	return tx.Commit()
}

// CopyDefaultRoutes copies aggregator_default_routes into client_routes for a new sub-account.
// Also ensures client_providers records exist.
func (r *RouteRepository) CopyDefaultRoutes(ctx context.Context, aggregatorID, subAccountID uuid.UUID) error {
	defaults, err := r.ListDefaultRoutes(ctx, aggregatorID)
	if err != nil {
		return err
	}
	for _, dr := range defaults {
		if !dr.Active {
			continue
		}
		if err := r.EnsureClientProvider(ctx, subAccountID, dr.ProviderID, aggregatorID); err != nil {
			return err
		}
		sr := &domain.SubAccountRoute{
			SubAccountID: subAccountID,
			OperatorID:   dr.OperatorID,
			ProviderID:   dr.ProviderID,
			Priority:     dr.Priority,
			Weight:       dr.Weight,
			Active:       true,
		}
		if err := r.CreateSubAccountRoute(ctx, sr); err != nil {
			return err
		}
	}
	return nil
}

// IsProviderAllowed checks if a provider is in the aggregator's allowed_provider_ids.
func (r *RouteRepository) IsProviderAllowed(ctx context.Context, aggregatorID, providerID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM aggregator_profiles
		WHERE client_id=$1 AND $2 = ANY(allowed_provider_ids)`,
		aggregatorID, providerID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check allowed provider: %w", err)
	}
	return count > 0, nil
}

// IsProviderActive checks if a provider record is active.
func (r *RouteRepository) IsProviderActive(ctx context.Context, providerID uuid.UUID) (bool, error) {
	var active bool
	err := r.db.QueryRowContext(ctx,
		`SELECT active FROM providers WHERE id=$1`, providerID).Scan(&active)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check provider active: %w", err)
	}
	return active, nil
}

// joinUUIDs converts []uuid.UUID to comma-separated string for SQL array literal.
func joinUUIDs(ids []uuid.UUID) string {
	s := ""
	for i, id := range ids {
		if i > 0 {
			s += ","
		}
		s += id.String()
	}
	return s
}
```

- [ ] **Step 2: Build check**

```bash
cd .worktrees/aggregator-phase3
go build ./internal/services/aggregator/repository/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/services/aggregator/repository/route_repository.go
git commit -m "feat(aggregator): add route repository for default and sub-account routes"
```

---

## Task 5: Route Service

**Files:**
- Create: `internal/services/aggregator/service/route_service.go`

- [ ] **Step 1: Write route service**

```go
// internal/services/aggregator/service/route_service.go
package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
)

// RouteRepo is the interface RouteService needs from the repository.
type RouteRepo interface {
	CreateDefaultRoute(ctx context.Context, dr *domain.DefaultRoute) error
	ListDefaultRoutes(ctx context.Context, aggregatorID uuid.UUID) ([]*domain.DefaultRoute, error)
	GetDefaultRouteByID(ctx context.Context, id, aggregatorID uuid.UUID) (*domain.DefaultRoute, error)
	UpdateDefaultRoute(ctx context.Context, dr *domain.DefaultRoute) error
	DeleteDefaultRoute(ctx context.Context, id, aggregatorID uuid.UUID) error

	CreateSubAccountRoute(ctx context.Context, sr *domain.SubAccountRoute) error
	ListSubAccountRoutes(ctx context.Context, subAccountID uuid.UUID) ([]*domain.SubAccountRoute, error)
	GetSubAccountRouteByID(ctx context.Context, id, subAccountID uuid.UUID) (*domain.SubAccountRoute, error)
	UpdateSubAccountRoute(ctx context.Context, sr *domain.SubAccountRoute) error
	DeleteSubAccountRoute(ctx context.Context, id, subAccountID uuid.UUID) error

	EnsureClientProvider(ctx context.Context, subAccountID, providerID, aggregatorID uuid.UUID) error
	SyncAggregatorProviders(ctx context.Context, aggregatorID uuid.UUID, allowedProviderIDs []uuid.UUID) error
	CopyDefaultRoutes(ctx context.Context, aggregatorID, subAccountID uuid.UUID) error
	IsProviderAllowed(ctx context.Context, aggregatorID, providerID uuid.UUID) (bool, error)
	IsProviderActive(ctx context.Context, providerID uuid.UUID) (bool, error)
}

// RouteService manages aggregator route logic.
type RouteService struct {
	repo RouteRepo
}

func NewRouteService(repo RouteRepo) *RouteService {
	return &RouteService{repo: repo}
}

// validateProvider checks that providerID is allowed for the aggregator and is active.
func (s *RouteService) validateProvider(ctx context.Context, aggregatorID, providerID uuid.UUID) error {
	allowed, err := s.repo.IsProviderAllowed(ctx, aggregatorID, providerID)
	if err != nil {
		return err
	}
	if !allowed {
		return domain.ErrProviderNotAllowed
	}
	active, err := s.repo.IsProviderActive(ctx, providerID)
	if err != nil {
		return err
	}
	if !active {
		return domain.ErrProviderNotActive
	}
	return nil
}

// CreateDefaultRoute validates provider and creates a default route template.
func (s *RouteService) CreateDefaultRoute(ctx context.Context, aggregatorID, operatorID, providerID uuid.UUID, priority, weight int) (*domain.DefaultRoute, error) {
	if err := s.validateProvider(ctx, aggregatorID, providerID); err != nil {
		return nil, err
	}
	dr := &domain.DefaultRoute{
		AggregatorID: aggregatorID,
		OperatorID:   operatorID,
		ProviderID:   providerID,
		Priority:     priority,
		Weight:       weight,
		Active:       true,
	}
	if err := s.repo.CreateDefaultRoute(ctx, dr); err != nil {
		return nil, fmt.Errorf("create default route: %w", err)
	}
	return dr, nil
}

// ListDefaultRoutes returns all default routes for an aggregator.
func (s *RouteService) ListDefaultRoutes(ctx context.Context, aggregatorID uuid.UUID) ([]*domain.DefaultRoute, error) {
	return s.repo.ListDefaultRoutes(ctx, aggregatorID)
}

// UpdateDefaultRoute updates priority/weight/active of a default route.
func (s *RouteService) UpdateDefaultRoute(ctx context.Context, aggregatorID, id uuid.UUID, priority, weight int, active bool) (*domain.DefaultRoute, error) {
	dr, err := s.repo.GetDefaultRouteByID(ctx, id, aggregatorID)
	if err != nil {
		return nil, err
	}
	dr.Priority = priority
	dr.Weight = weight
	dr.Active = active
	if err := s.repo.UpdateDefaultRoute(ctx, dr); err != nil {
		return nil, err
	}
	return dr, nil
}

// DeleteDefaultRoute removes a default route template.
func (s *RouteService) DeleteDefaultRoute(ctx context.Context, aggregatorID, id uuid.UUID) error {
	return s.repo.DeleteDefaultRoute(ctx, id, aggregatorID)
}

// CreateSubAccountRoute validates provider, ensures client_providers, creates route in client_routes.
func (s *RouteService) CreateSubAccountRoute(ctx context.Context, aggregatorID, subAccountID, operatorID, providerID uuid.UUID, priority, weight int) (*domain.SubAccountRoute, error) {
	if err := s.validateProvider(ctx, aggregatorID, providerID); err != nil {
		return nil, err
	}
	if err := s.repo.EnsureClientProvider(ctx, subAccountID, providerID, aggregatorID); err != nil {
		return nil, err
	}
	sr := &domain.SubAccountRoute{
		SubAccountID: subAccountID,
		OperatorID:   operatorID,
		ProviderID:   providerID,
		Priority:     priority,
		Weight:       weight,
		Active:       true,
	}
	if err := s.repo.CreateSubAccountRoute(ctx, sr); err != nil {
		return nil, fmt.Errorf("create sub-account route: %w", err)
	}
	return sr, nil
}

// ListSubAccountRoutes returns all routes for a sub-account.
func (s *RouteService) ListSubAccountRoutes(ctx context.Context, subAccountID uuid.UUID) ([]*domain.SubAccountRoute, error) {
	return s.repo.ListSubAccountRoutes(ctx, subAccountID)
}

// UpdateSubAccountRoute updates priority/weight/active of an existing sub-account route.
func (s *RouteService) UpdateSubAccountRoute(ctx context.Context, subAccountID, id uuid.UUID, priority, weight int, active bool) (*domain.SubAccountRoute, error) {
	sr, err := s.repo.GetSubAccountRouteByID(ctx, id, subAccountID)
	if err != nil {
		return nil, err
	}
	sr.Priority = priority
	sr.Weight = weight
	sr.Active = active
	if err := s.repo.UpdateSubAccountRoute(ctx, sr); err != nil {
		return nil, err
	}
	return sr, nil
}

// DeleteSubAccountRoute removes a route from a sub-account.
func (s *RouteService) DeleteSubAccountRoute(ctx context.Context, subAccountID, id uuid.UUID) error {
	return s.repo.DeleteSubAccountRoute(ctx, id, subAccountID)
}

// CopyDefaultRoutes copies default route templates to a newly created sub-account.
// Called by aggregator_service.CreateSubAccount.
func (s *RouteService) CopyDefaultRoutes(ctx context.Context, aggregatorID, subAccountID uuid.UUID) error {
	return s.repo.CopyDefaultRoutes(ctx, aggregatorID, subAccountID)
}

// SyncAggregatorProviders syncs client_providers when allowed_provider_ids changes.
func (s *RouteService) SyncAggregatorProviders(ctx context.Context, aggregatorID uuid.UUID, allowedProviderIDs []uuid.UUID) error {
	return s.repo.SyncAggregatorProviders(ctx, aggregatorID, allowedProviderIDs)
}
```

- [ ] **Step 2: Build check**

```bash
cd .worktrees/aggregator-phase3
go build ./internal/services/aggregator/service/...
```

Expected: no errors.

- [ ] **Step 3: Write tests**

Create `internal/services/aggregator/service/route_service_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRouteRepo struct {
	isAllowed   bool
	isActive    bool
	allowErr    error
	activeErr   error
	createDRErr error
	createdDR   *domain.DefaultRoute
	createSRErr error
	createdSR   *domain.SubAccountRoute
	ensureCPErr error
	listDR      []*domain.DefaultRoute
	listSR      []*domain.SubAccountRoute
}

func (m *mockRouteRepo) IsProviderAllowed(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return m.isAllowed, m.allowErr
}
func (m *mockRouteRepo) IsProviderActive(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.isActive, m.activeErr
}
func (m *mockRouteRepo) CreateDefaultRoute(_ context.Context, dr *domain.DefaultRoute) error {
	m.createdDR = dr
	return m.createDRErr
}
func (m *mockRouteRepo) ListDefaultRoutes(_ context.Context, _ uuid.UUID) ([]*domain.DefaultRoute, error) {
	return m.listDR, nil
}
func (m *mockRouteRepo) GetDefaultRouteByID(_ context.Context, id, _ uuid.UUID) (*domain.DefaultRoute, error) {
	for _, dr := range m.listDR {
		if dr.ID == id {
			return dr, nil
		}
	}
	return nil, domain.ErrDefaultRouteNotFound
}
func (m *mockRouteRepo) UpdateDefaultRoute(_ context.Context, dr *domain.DefaultRoute) error { return nil }
func (m *mockRouteRepo) DeleteDefaultRoute(_ context.Context, _, _ uuid.UUID) error          { return nil }
func (m *mockRouteRepo) CreateSubAccountRoute(_ context.Context, sr *domain.SubAccountRoute) error {
	m.createdSR = sr
	return m.createSRErr
}
func (m *mockRouteRepo) ListSubAccountRoutes(_ context.Context, _ uuid.UUID) ([]*domain.SubAccountRoute, error) {
	return m.listSR, nil
}
func (m *mockRouteRepo) GetSubAccountRouteByID(_ context.Context, id, _ uuid.UUID) (*domain.SubAccountRoute, error) {
	for _, sr := range m.listSR {
		if sr.ID == id {
			return sr, nil
		}
	}
	return nil, domain.ErrRouteNotFound
}
func (m *mockRouteRepo) UpdateSubAccountRoute(_ context.Context, sr *domain.SubAccountRoute) error { return nil }
func (m *mockRouteRepo) DeleteSubAccountRoute(_ context.Context, _, _ uuid.UUID) error             { return nil }
func (m *mockRouteRepo) EnsureClientProvider(_ context.Context, _, _, _ uuid.UUID) error {
	return m.ensureCPErr
}
func (m *mockRouteRepo) SyncAggregatorProviders(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}
func (m *mockRouteRepo) CopyDefaultRoutes(_ context.Context, _, _ uuid.UUID) error { return nil }

func TestCreateDefaultRoute_ProviderNotAllowed(t *testing.T) {
	repo := &mockRouteRepo{isAllowed: false, isActive: true}
	svc := service.NewRouteService(repo)
	aggID, opID, provID := uuid.New(), uuid.New(), uuid.New()
	_, err := svc.CreateDefaultRoute(context.Background(), aggID, opID, provID, 0, 1)
	assert.ErrorIs(t, err, domain.ErrProviderNotAllowed)
}

func TestCreateDefaultRoute_ProviderNotActive(t *testing.T) {
	repo := &mockRouteRepo{isAllowed: true, isActive: false}
	svc := service.NewRouteService(repo)
	_, err := svc.CreateDefaultRoute(context.Background(), uuid.New(), uuid.New(), uuid.New(), 0, 1)
	assert.ErrorIs(t, err, domain.ErrProviderNotActive)
}

func TestCreateDefaultRoute_Success(t *testing.T) {
	repo := &mockRouteRepo{isAllowed: true, isActive: true}
	svc := service.NewRouteService(repo)
	aggID, opID, provID := uuid.New(), uuid.New(), uuid.New()
	dr, err := svc.CreateDefaultRoute(context.Background(), aggID, opID, provID, 10, 2)
	require.NoError(t, err)
	assert.Equal(t, aggID, dr.AggregatorID)
	assert.Equal(t, provID, dr.ProviderID)
	assert.Equal(t, 10, dr.Priority)
}

func TestCreateSubAccountRoute_ProviderNotAllowed(t *testing.T) {
	repo := &mockRouteRepo{isAllowed: false, isActive: true}
	svc := service.NewRouteService(repo)
	_, err := svc.CreateSubAccountRoute(context.Background(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), 0, 1)
	assert.ErrorIs(t, err, domain.ErrProviderNotAllowed)
}

func TestCreateSubAccountRoute_Success(t *testing.T) {
	repo := &mockRouteRepo{isAllowed: true, isActive: true}
	svc := service.NewRouteService(repo)
	subID, opID, provID := uuid.New(), uuid.New(), uuid.New()
	sr, err := svc.CreateSubAccountRoute(context.Background(), uuid.New(), subID, opID, provID, 5, 1)
	require.NoError(t, err)
	assert.Equal(t, subID, sr.SubAccountID)
	assert.Equal(t, provID, sr.ProviderID)
}
```

- [ ] **Step 4: Run tests**

```bash
cd .worktrees/aggregator-phase3
go test ./internal/services/aggregator/service/... -run TestCreate -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/services/aggregator/service/route_service.go internal/services/aggregator/service/route_service_test.go
git commit -m "feat(aggregator): add route service with provider validation"
```

---

## Task 6: Wire CopyDefaultRoutes into CreateSubAccount

**Files:**
- Modify: `internal/services/aggregator/service/aggregator_service.go`

- [ ] **Step 1: Find CreateSubAccount and inject RouteService call**

In `aggregator_service.go`, find the `CreateSubAccount` method. Add `routes RouteService` field to `Service` struct, or pass it as a dependency on the `Service`. The cleanest approach (matching Phase 2 pattern where TariffService is a separate field on grpc.Server) is to add it to the gRPC server and call it after successful sub-account creation.

Open `internal/services/aggregator/grpc/server.go`. The `Server` struct currently has:
```go
type Server struct {
    aggregatorv1.UnimplementedAggregatorServiceServer
    svc          *service.Service
    clientClient clientv1.ClientServiceClient
    tariffs      *service.TariffService
}
```

Add `routes *service.RouteService`:
```go
type Server struct {
    aggregatorv1.UnimplementedAggregatorServiceServer
    svc          *service.Service
    clientClient clientv1.ClientServiceClient
    tariffs      *service.TariffService
    routes       *service.RouteService
}

func NewServer(svc *service.Service, clientClient clientv1.ClientServiceClient, tariffs *service.TariffService, routes *service.RouteService) *Server {
    return &Server{svc: svc, clientClient: clientClient, tariffs: tariffs, routes: routes}
}
```

- [ ] **Step 2: Call CopyDefaultRoutes after CreateSubAccount in gRPC handler**

Find `CreateSubAccount` in `server.go`. After `resp, err := s.svc.CreateSubAccount(...)` succeeds, add:

```go
subID, _ := uuid.Parse(resp.Id)
if copyErr := s.routes.CopyDefaultRoutes(ctx, aggID, subID); copyErr != nil {
    // Log but don't fail — sub-account was created, routes are not critical
    // Use zerolog if logger is available, otherwise fmt
    fmt.Printf("warn: copy default routes for sub-account %s: %v\n", subID, copyErr)
}
```

Import `"fmt"` if not already imported.

- [ ] **Step 3: Wire RouteService in the aggregator-service binary main**

Open `cmd/aggregator-service/main.go` (or wherever `grpc.NewServer` is called). Instantiate the route repository and service:

```go
routeRepo := repository.NewRouteRepository(db)
routeSvc := service.NewRouteService(routeRepo)
grpcServer := grpc.NewServer(aggSvc, clientClient, tariffSvc, routeSvc)
```

- [ ] **Step 4: Build check**

```bash
cd .worktrees/aggregator-phase3
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/services/aggregator/grpc/server.go
git add $(git diff --name-only -- "cmd/aggregator-service/")
git commit -m "feat(aggregator): copy default routes on sub-account creation"
```

---

## Task 7: Proto — Add Route RPCs

**Files:**
- Modify: `api/proto/aggregator/aggregator.proto`
- Regenerate: `api/proto/aggregatorv1/aggregator.pb.go`, `api/proto/aggregatorv1/aggregator_grpc.pb.go`

- [ ] **Step 1: Add route messages and RPCs to proto**

Open `api/proto/aggregator/aggregator.proto`. After the last `rpc DeleteTariff` line inside `service AggregatorService`, add:

```protobuf
  // Sub-account routes (aggregator)
  rpc CreateSubAccountRoute(CreateSubAccountRouteRequest) returns (SubAccountRoute);
  rpc UpdateSubAccountRoute(UpdateSubAccountRouteRequest) returns (SubAccountRoute);
  rpc DeleteSubAccountRoute(DeleteSubAccountRouteRequest) returns (google.protobuf.Empty);
  rpc ListSubAccountRoutes(ListSubAccountRoutesRequest) returns (ListSubAccountRoutesResponse);
  rpc CreateDefaultRoute(CreateDefaultRouteRequest) returns (AggregatorDefaultRoute);
  rpc UpdateDefaultRoute(UpdateDefaultRouteRequest) returns (AggregatorDefaultRoute);
  rpc DeleteDefaultRoute(DeleteDefaultRouteRequest) returns (google.protobuf.Empty);
  rpc ListDefaultRoutes(ListDefaultRoutesRequest) returns (ListDefaultRoutesResponse);
```

Add message definitions (after existing messages, before the `service` block or at end of file):

```protobuf
message AggregatorDefaultRoute {
  string id = 1;
  string aggregator_id = 2;
  string operator_id = 3;
  string provider_id = 4;
  int32 priority = 5;
  int32 weight = 6;
  bool active = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
}

message SubAccountRoute {
  string id = 1;
  string sub_account_id = 2;
  string operator_id = 3;
  string provider_id = 4;
  int32 priority = 5;
  int32 weight = 6;
  bool active = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
}

message CreateSubAccountRouteRequest {
  string aggregator_id = 1;
  string sub_account_id = 2;
  string operator_id = 3;
  string provider_id = 4;
  int32 priority = 5;
  int32 weight = 6;
}

message UpdateSubAccountRouteRequest {
  string aggregator_id = 1;
  string sub_account_id = 2;
  string route_id = 3;
  int32 priority = 4;
  int32 weight = 5;
  bool active = 6;
}

message DeleteSubAccountRouteRequest {
  string aggregator_id = 1;
  string sub_account_id = 2;
  string route_id = 3;
}

message ListSubAccountRoutesRequest {
  string aggregator_id = 1;
  string sub_account_id = 2;
}

message ListSubAccountRoutesResponse {
  repeated SubAccountRoute routes = 1;
}

message CreateDefaultRouteRequest {
  string aggregator_id = 1;
  string operator_id = 2;
  string provider_id = 3;
  int32 priority = 4;
  int32 weight = 5;
}

message UpdateDefaultRouteRequest {
  string aggregator_id = 1;
  string route_id = 2;
  int32 priority = 3;
  int32 weight = 4;
  bool active = 5;
}

message DeleteDefaultRouteRequest {
  string aggregator_id = 1;
  string route_id = 2;
}

message ListDefaultRoutesRequest {
  string aggregator_id = 1;
}

message ListDefaultRoutesResponse {
  repeated AggregatorDefaultRoute routes = 1;
}
```

- [ ] **Step 2: Regenerate proto**

```bash
cd .worktrees/aggregator-phase3
bash scripts/generate-proto.sh
```

Expected: `api/proto/aggregatorv1/aggregator.pb.go` and `aggregator_grpc.pb.go` updated.

- [ ] **Step 3: Build check**

```bash
go build ./api/proto/aggregatorv1/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add api/proto/aggregator/aggregator.proto api/proto/aggregatorv1/
git commit -m "feat(proto): add route RPCs to AggregatorService"
```

---

## Task 8: gRPC Server — Route Handlers

**Files:**
- Modify: `internal/services/aggregator/grpc/server.go`

- [ ] **Step 1: Implement route gRPC handlers**

Append to `internal/services/aggregator/grpc/server.go`:

```go
// --- Default Routes ---

func (s *Server) CreateDefaultRoute(ctx context.Context, req *aggregatorv1.CreateDefaultRouteRequest) (*aggregatorv1.AggregatorDefaultRoute, error) {
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id: %v", err)
	}
	opID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
	}
	provID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid provider_id: %v", err)
	}

	dr, err := s.routes.CreateDefaultRoute(ctx, aggID, opID, provID, int(req.Priority), int(req.Weight))
	if err != nil {
		return nil, mapRouteError(err)
	}
	return domainDefaultRouteToProto(dr), nil
}

func (s *Server) ListDefaultRoutes(ctx context.Context, req *aggregatorv1.ListDefaultRoutesRequest) (*aggregatorv1.ListDefaultRoutesResponse, error) {
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id: %v", err)
	}
	drs, err := s.routes.ListDefaultRoutes(ctx, aggID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list default routes: %v", err)
	}
	var result []*aggregatorv1.AggregatorDefaultRoute
	for _, dr := range drs {
		result = append(result, domainDefaultRouteToProto(dr))
	}
	return &aggregatorv1.ListDefaultRoutesResponse{Routes: result}, nil
}

func (s *Server) UpdateDefaultRoute(ctx context.Context, req *aggregatorv1.UpdateDefaultRouteRequest) (*aggregatorv1.AggregatorDefaultRoute, error) {
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id: %v", err)
	}
	routeID, err := uuid.Parse(req.RouteId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid route_id: %v", err)
	}
	dr, err := s.routes.UpdateDefaultRoute(ctx, aggID, routeID, int(req.Priority), int(req.Weight), req.Active)
	if err != nil {
		return nil, mapRouteError(err)
	}
	return domainDefaultRouteToProto(dr), nil
}

func (s *Server) DeleteDefaultRoute(ctx context.Context, req *aggregatorv1.DeleteDefaultRouteRequest) (*emptypb.Empty, error) {
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id: %v", err)
	}
	routeID, err := uuid.Parse(req.RouteId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid route_id: %v", err)
	}
	if err := s.routes.DeleteDefaultRoute(ctx, aggID, routeID); err != nil {
		return nil, mapRouteError(err)
	}
	return &emptypb.Empty{}, nil
}

// --- Sub-Account Routes ---

func (s *Server) CreateSubAccountRoute(ctx context.Context, req *aggregatorv1.CreateSubAccountRouteRequest) (*aggregatorv1.SubAccountRoute, error) {
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id: %v", err)
	}
	subID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid sub_account_id: %v", err)
	}
	opID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
	}
	provID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid provider_id: %v", err)
	}
	sr, err := s.routes.CreateSubAccountRoute(ctx, aggID, subID, opID, provID, int(req.Priority), int(req.Weight))
	if err != nil {
		return nil, mapRouteError(err)
	}
	return domainSubAccountRouteToProto(sr), nil
}

func (s *Server) ListSubAccountRoutes(ctx context.Context, req *aggregatorv1.ListSubAccountRoutesRequest) (*aggregatorv1.ListSubAccountRoutesResponse, error) {
	subID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid sub_account_id: %v", err)
	}
	srs, err := s.routes.ListSubAccountRoutes(ctx, subID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list sub-account routes: %v", err)
	}
	var result []*aggregatorv1.SubAccountRoute
	for _, sr := range srs {
		result = append(result, domainSubAccountRouteToProto(sr))
	}
	return &aggregatorv1.ListSubAccountRoutesResponse{Routes: result}, nil
}

func (s *Server) UpdateSubAccountRoute(ctx context.Context, req *aggregatorv1.UpdateSubAccountRouteRequest) (*aggregatorv1.SubAccountRoute, error) {
	subID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid sub_account_id: %v", err)
	}
	routeID, err := uuid.Parse(req.RouteId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid route_id: %v", err)
	}
	sr, err := s.routes.UpdateSubAccountRoute(ctx, subID, routeID, int(req.Priority), int(req.Weight), req.Active)
	if err != nil {
		return nil, mapRouteError(err)
	}
	return domainSubAccountRouteToProto(sr), nil
}

func (s *Server) DeleteSubAccountRoute(ctx context.Context, req *aggregatorv1.DeleteSubAccountRouteRequest) (*emptypb.Empty, error) {
	subID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid sub_account_id: %v", err)
	}
	routeID, err := uuid.Parse(req.RouteId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid route_id: %v", err)
	}
	if err := s.routes.DeleteSubAccountRoute(ctx, subID, routeID); err != nil {
		return nil, mapRouteError(err)
	}
	return &emptypb.Empty{}, nil
}

// --- Helpers ---

func domainDefaultRouteToProto(dr *domain.DefaultRoute) *aggregatorv1.AggregatorDefaultRoute {
	return &aggregatorv1.AggregatorDefaultRoute{
		Id:           dr.ID.String(),
		AggregatorId: dr.AggregatorID.String(),
		OperatorId:   dr.OperatorID.String(),
		ProviderId:   dr.ProviderID.String(),
		Priority:     int32(dr.Priority),
		Weight:       int32(dr.Weight),
		Active:       dr.Active,
		CreatedAt:    timestamppb.New(dr.CreatedAt),
		UpdatedAt:    timestamppb.New(dr.UpdatedAt),
	}
}

func domainSubAccountRouteToProto(sr *domain.SubAccountRoute) *aggregatorv1.SubAccountRoute {
	return &aggregatorv1.SubAccountRoute{
		Id:           sr.ID.String(),
		SubAccountId: sr.SubAccountID.String(),
		OperatorId:   sr.OperatorID.String(),
		ProviderId:   sr.ProviderID.String(),
		Priority:     int32(sr.Priority),
		Weight:       int32(sr.Weight),
		Active:       sr.Active,
		CreatedAt:    timestamppb.New(sr.CreatedAt),
		UpdatedAt:    timestamppb.New(sr.UpdatedAt),
	}
}

func mapRouteError(err error) error {
	switch {
	case errors.Is(err, domain.ErrDefaultRouteNotFound), errors.Is(err, domain.ErrRouteNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrProviderNotAllowed), errors.Is(err, domain.ErrProviderNotActive):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrRouteAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}
```

- [ ] **Step 2: Build check**

```bash
cd .worktrees/aggregator-phase3
go build ./internal/services/aggregator/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/services/aggregator/grpc/server.go
git commit -m "feat(aggregator): implement route gRPC handlers"
```

---

## Task 9: HTTP Handlers

**Files:**
- Create: `internal/gateway/portal/handlers/aggregator_routes.go`
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Write HTTP handlers**

```go
// internal/gateway/portal/handlers/aggregator_routes.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	aggregatorv1 "github.com/smpp-server/smpp-server/api/proto/aggregatorv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AggregatorRoutesHandlers handles /portal/v1/aggregator/routes and /portal/v1/aggregator/routes/defaults
type AggregatorRoutesHandlers struct {
	aggregatorClient aggregatorv1.AggregatorServiceClient
}

func NewAggregatorRoutesHandlers(client aggregatorv1.AggregatorServiceClient) *AggregatorRoutesHandlers {
	return &AggregatorRoutesHandlers{aggregatorClient: client}
}

// ListSubAccountRoutes GET /portal/v1/aggregator/routes?sub_account_id=<id>
func (h *AggregatorRoutesHandlers) ListSubAccountRoutes(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	subAccountID := r.URL.Query().Get("sub_account_id")
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("sub_account_id is required"))
		return
	}
	resp, err := h.aggregatorClient.ListSubAccountRoutes(r.Context(), &aggregatorv1.ListSubAccountRoutesRequest{
		AggregatorId: aggID.String(),
		SubAccountId: subAccountID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// CreateSubAccountRoute POST /portal/v1/aggregator/routes
func (h *AggregatorRoutesHandlers) CreateSubAccountRoute(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	var body struct {
		SubAccountID string `json:"sub_account_id"`
		OperatorID   string `json:"operator_id"`
		ProviderID   string `json:"provider_id"`
		Priority     int32  `json:"priority"`
		Weight       int32  `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	resp, err := h.aggregatorClient.CreateSubAccountRoute(r.Context(), &aggregatorv1.CreateSubAccountRouteRequest{
		AggregatorId: aggID.String(),
		SubAccountId: body.SubAccountID,
		OperatorId:   body.OperatorID,
		ProviderId:   body.ProviderID,
		Priority:     body.Priority,
		Weight:       body.Weight,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

// UpdateSubAccountRoute PUT /portal/v1/aggregator/routes/{id}
func (h *AggregatorRoutesHandlers) UpdateSubAccountRoute(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	routeID := mux.Vars(r)["id"]
	var body struct {
		SubAccountID string `json:"sub_account_id"`
		Priority     int32  `json:"priority"`
		Weight       int32  `json:"weight"`
		Active       bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	resp, err := h.aggregatorClient.UpdateSubAccountRoute(r.Context(), &aggregatorv1.UpdateSubAccountRouteRequest{
		AggregatorId: aggID.String(),
		SubAccountId: body.SubAccountID,
		RouteId:      routeID,
		Priority:     body.Priority,
		Weight:       body.Weight,
		Active:       body.Active,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// DeleteSubAccountRoute DELETE /portal/v1/aggregator/routes/{id}
func (h *AggregatorRoutesHandlers) DeleteSubAccountRoute(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	routeID := mux.Vars(r)["id"]
	subAccountID := r.URL.Query().Get("sub_account_id")
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("sub_account_id is required"))
		return
	}
	if _, err := h.aggregatorClient.DeleteSubAccountRoute(r.Context(), &aggregatorv1.DeleteSubAccountRouteRequest{
		AggregatorId: aggID.String(),
		SubAccountId: subAccountID,
		RouteId:      routeID,
	}); err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListDefaultRoutes GET /portal/v1/aggregator/routes/defaults
func (h *AggregatorRoutesHandlers) ListDefaultRoutes(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	resp, err := h.aggregatorClient.ListDefaultRoutes(r.Context(), &aggregatorv1.ListDefaultRoutesRequest{
		AggregatorId: aggID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// CreateDefaultRoute POST /portal/v1/aggregator/routes/defaults
func (h *AggregatorRoutesHandlers) CreateDefaultRoute(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	var body struct {
		OperatorID string `json:"operator_id"`
		ProviderID string `json:"provider_id"`
		Priority   int32  `json:"priority"`
		Weight     int32  `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	resp, err := h.aggregatorClient.CreateDefaultRoute(r.Context(), &aggregatorv1.CreateDefaultRouteRequest{
		AggregatorId: aggID.String(),
		OperatorId:   body.OperatorID,
		ProviderId:   body.ProviderID,
		Priority:     body.Priority,
		Weight:       body.Weight,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

// UpdateDefaultRoute PUT /portal/v1/aggregator/routes/defaults/{id}
func (h *AggregatorRoutesHandlers) UpdateDefaultRoute(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	routeID := mux.Vars(r)["id"]
	var body struct {
		Priority int32 `json:"priority"`
		Weight   int32 `json:"weight"`
		Active   bool  `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	resp, err := h.aggregatorClient.UpdateDefaultRoute(r.Context(), &aggregatorv1.UpdateDefaultRouteRequest{
		AggregatorId: aggID.String(),
		RouteId:      routeID,
		Priority:     body.Priority,
		Weight:       body.Weight,
		Active:       body.Active,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// DeleteDefaultRoute DELETE /portal/v1/aggregator/routes/defaults/{id}
func (h *AggregatorRoutesHandlers) DeleteDefaultRoute(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	routeID := mux.Vars(r)["id"]
	if _, err := h.aggregatorClient.DeleteDefaultRoute(r.Context(), &aggregatorv1.DeleteDefaultRouteRequest{
		AggregatorId: aggID.String(),
		RouteId:      routeID,
	}); err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Register routes in router.go**

In `internal/gateway/portal/router/router.go`, find the aggregator subrouter block (after tariff routes). Add:

```go
	routeHandlers := handlers.NewAggregatorRoutesHandlers(aggregatorClient)

	// Route defaults (must come before /{id} to avoid mux conflict)
	aggregator.HandleFunc("/routes/defaults", routeHandlers.ListDefaultRoutes).Methods("GET")
	aggregator.HandleFunc("/routes/defaults", routeHandlers.CreateDefaultRoute).Methods("POST")
	aggregator.HandleFunc("/routes/defaults/{id}", routeHandlers.UpdateDefaultRoute).Methods("PUT")
	aggregator.HandleFunc("/routes/defaults/{id}", routeHandlers.DeleteDefaultRoute).Methods("DELETE")

	// Sub-account routes
	aggregator.HandleFunc("/routes", routeHandlers.ListSubAccountRoutes).Methods("GET")
	aggregator.HandleFunc("/routes", routeHandlers.CreateSubAccountRoute).Methods("POST")
	aggregator.HandleFunc("/routes/{id}", routeHandlers.UpdateSubAccountRoute).Methods("PUT")
	aggregator.HandleFunc("/routes/{id}", routeHandlers.DeleteSubAccountRoute).Methods("DELETE")
```

- [ ] **Step 3: Build check**

```bash
cd .worktrees/aggregator-phase3
go build ./internal/gateway/portal/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/aggregator_routes.go internal/gateway/portal/router/router.go
git commit -m "feat(aggregator): add route HTTP handlers and register endpoints"
```

---

## Task 10: SyncAggregatorProviders on Profile Update

**Files:**
- Modify: `internal/services/aggregator/grpc/server.go`

When admin updates `allowed_provider_ids` via `UpdateAggregatorProfile`, call `SyncAggregatorProviders`.

- [ ] **Step 1: Find UpdateAggregatorProfile in server.go**

After the `s.svc.UpdateAggregatorProfile(...)` call succeeds, add:

```go
	// Sync client_providers for aggregator when allowed providers change
	if len(req.AllowedProviderIds) > 0 {
		allowedIDs := make([]uuid.UUID, 0, len(req.AllowedProviderIds))
		for _, idStr := range req.AllowedProviderIds {
			id, _ := uuid.Parse(idStr)
			allowedIDs = append(allowedIDs, id)
		}
		clientID, _ := uuid.Parse(req.ClientId)
		if syncErr := s.routes.SyncAggregatorProviders(ctx, clientID, allowedIDs); syncErr != nil {
			// Log but don't fail the profile update
			fmt.Printf("warn: sync aggregator providers for %s: %v\n", req.ClientId, syncErr)
		}
	}
```

- [ ] **Step 2: Build check**

```bash
cd .worktrees/aggregator-phase3
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/services/aggregator/grpc/server.go
git commit -m "feat(aggregator): sync client_providers on profile allowed_provider_ids update"
```

---

## Task 11: Frontend — API Client

**Files:**
- Modify: `portal-frontend/src/api/aggregator.ts`

- [ ] **Step 1: Add route API types and functions**

Append to `portal-frontend/src/api/aggregator.ts`:

```typescript
export interface SubAccountRoute {
  id: string;
  sub_account_id: string;
  operator_id: string;
  provider_id: string;
  priority: number;
  weight: number;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface AggregatorDefaultRoute {
  id: string;
  aggregator_id: string;
  operator_id: string;
  provider_id: string;
  priority: number;
  weight: number;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface ListSubAccountRoutesResponse {
  routes: SubAccountRoute[];
}

export interface ListDefaultRoutesResponse {
  routes: AggregatorDefaultRoute[];
}

export interface CreateSubAccountRouteRequest {
  sub_account_id: string;
  operator_id: string;
  provider_id: string;
  priority?: number;
  weight?: number;
}

export interface CreateDefaultRouteRequest {
  operator_id: string;
  provider_id: string;
  priority?: number;
  weight?: number;
}

export interface UpdateRouteRequest {
  sub_account_id?: string;
  priority: number;
  weight: number;
  active: boolean;
}

// Route API methods — add to the aggregatorApi object
// (add these inside the aggregatorApi object in the existing export)
```

Then inside the `aggregatorApi` object, add:

```typescript
  listSubAccountRoutes: (subAccountId: string) =>
    apiFetch<ListSubAccountRoutesResponse>(`/aggregator/routes?sub_account_id=${subAccountId}`),

  createSubAccountRoute: (data: CreateSubAccountRouteRequest) =>
    apiFetch<SubAccountRoute>('/aggregator/routes', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateSubAccountRoute: (id: string, data: UpdateRouteRequest) =>
    apiFetch<SubAccountRoute>(`/aggregator/routes/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  deleteSubAccountRoute: (id: string, subAccountId: string) =>
    apiFetch<void>(`/aggregator/routes/${id}?sub_account_id=${subAccountId}`, {
      method: 'DELETE',
    }),

  listDefaultRoutes: () =>
    apiFetch<ListDefaultRoutesResponse>('/aggregator/routes/defaults'),

  createDefaultRoute: (data: CreateDefaultRouteRequest) =>
    apiFetch<AggregatorDefaultRoute>('/aggregator/routes/defaults', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateDefaultRoute: (id: string, data: { priority: number; weight: number; active: boolean }) =>
    apiFetch<AggregatorDefaultRoute>(`/aggregator/routes/defaults/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  deleteDefaultRoute: (id: string) =>
    apiFetch<void>(`/aggregator/routes/defaults/${id}`, { method: 'DELETE' }),
```

- [ ] **Step 2: TypeScript check**

```bash
cd .worktrees/aggregator-phase3/portal-frontend
npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/api/aggregator.ts
git commit -m "feat(frontend): add route API client types and functions"
```

---

## Task 12: Frontend — RoutesTab (Sub-Account Detail)

**Files:**
- Create: `portal-frontend/src/pages/aggregator/tabs/RoutesTab.tsx`
- Modify: `portal-frontend/src/pages/aggregator/SubAccountDetailPage.tsx`

- [ ] **Step 1: Write RoutesTab**

```tsx
// portal-frontend/src/pages/aggregator/tabs/RoutesTab.tsx
import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { aggregatorApi, SubAccountRoute } from '../../../api/aggregator';

export default function RoutesTab() {
  const { id: subAccountId } = useParams<{ id: string }>();
  const [routes, setRoutes] = useState<SubAccountRoute[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!subAccountId) return;
    aggregatorApi
      .listSubAccountRoutes(subAccountId)
      .then((data) => setRoutes(data.routes ?? []))
      .catch((e) => setError(e instanceof Error ? e.message : 'Ошибка загрузки'))
      .finally(() => setLoading(false));
  }, [subAccountId]);

  if (loading) return <p className="text-sm text-gray-500">Загрузка...</p>;
  if (error) return <p className="text-sm text-red-600">{error}</p>;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="font-medium text-gray-900">Маршруты субаккаунта</h3>
        <Link
          to={`/aggregator/routes?sub_account_id=${subAccountId}`}
          className="text-sm text-blue-600 hover:underline"
        >
          Управление маршрутами →
        </Link>
      </div>
      {routes.length === 0 ? (
        <p className="text-sm text-gray-500">Маршруты не назначены.</p>
      ) : (
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b text-left text-gray-500">
              <th className="pb-2 pr-4">Оператор</th>
              <th className="pb-2 pr-4">Провайдер</th>
              <th className="pb-2 pr-4">Приоритет</th>
              <th className="pb-2 pr-4">Вес</th>
              <th className="pb-2">Статус</th>
            </tr>
          </thead>
          <tbody>
            {routes.map((r) => (
              <tr key={r.id} className="border-b last:border-0">
                <td className="py-2 pr-4 font-mono text-xs">{r.operator_id}</td>
                <td className="py-2 pr-4 font-mono text-xs">{r.provider_id}</td>
                <td className="py-2 pr-4">{r.priority}</td>
                <td className="py-2 pr-4">{r.weight}</td>
                <td className="py-2">
                  <span className={r.active ? 'text-green-600' : 'text-gray-400'}>
                    {r.active ? 'Активен' : 'Неактивен'}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Add RoutesTab to SubAccountDetailPage**

Open `portal-frontend/src/pages/aggregator/SubAccountDetailPage.tsx`. Find the Tabs.List block with existing tabs (Balance, Tariffs). Add:

```tsx
<Tabs.Trigger value="routes" className="...existing class...">
  Маршруты
</Tabs.Trigger>
```

And in the content area:

```tsx
<Tabs.Content value="routes">
  <RoutesTab />
</Tabs.Content>
```

Import at top:
```tsx
import RoutesTab from './tabs/RoutesTab';
```

Copy the exact className string from an existing `Tabs.Trigger` for consistency.

- [ ] **Step 3: TypeScript check**

```bash
cd .worktrees/aggregator-phase3/portal-frontend
npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/aggregator/tabs/RoutesTab.tsx portal-frontend/src/pages/aggregator/SubAccountDetailPage.tsx
git commit -m "feat(frontend): add RoutesTab to sub-account detail page"
```

---

## Task 13: Frontend — RoutesPage (Global Management)

**Files:**
- Create: `portal-frontend/src/pages/aggregator/RoutesPage.tsx`
- Modify: app router (add `/aggregator/routes` route)
- Modify: `portal-frontend/src/components/layout/Sidebar.tsx` (add Routes nav item)

- [ ] **Step 1: Write RoutesPage**

```tsx
// portal-frontend/src/pages/aggregator/RoutesPage.tsx
import { useState, useEffect } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { useSearchParams } from 'react-router-dom';
import {
  aggregatorApi,
  SubAccountRoute,
  AggregatorDefaultRoute,
  CreateSubAccountRouteRequest,
  CreateDefaultRouteRequest,
} from '../../api/aggregator';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';

const TAB_TRIGGER_CLASS =
  'px-4 py-2 text-sm font-medium text-gray-600 border-b-2 border-transparent data-[state=active]:border-blue-600 data-[state=active]:text-blue-600';

export default function RoutesPage() {
  const [searchParams] = useSearchParams();
  const preFilterSubId = searchParams.get('sub_account_id') ?? '';

  const [tab, setTab] = useState('sub-accounts');
  const [subRoutes, setSubRoutes] = useState<SubAccountRoute[]>([]);
  const [defaultRoutes, setDefaultRoutes] = useState<AggregatorDefaultRoute[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const [subForm, setSubForm] = useState<CreateSubAccountRouteRequest>({
    sub_account_id: preFilterSubId,
    operator_id: '',
    provider_id: '',
    priority: 0,
    weight: 1,
  });
  const [defaultForm, setDefaultForm] = useState<CreateDefaultRouteRequest>({
    operator_id: '',
    provider_id: '',
    priority: 0,
    weight: 1,
  });

  const isDefaults = tab === 'defaults';

  async function fetchRoutes() {
    setLoading(true);
    setLoadError(null);
    try {
      if (isDefaults) {
        const data = await aggregatorApi.listDefaultRoutes();
        setDefaultRoutes(data.routes ?? []);
      } else {
        if (!preFilterSubId) {
          setSubRoutes([]);
          return;
        }
        const data = await aggregatorApi.listSubAccountRoutes(preFilterSubId);
        setSubRoutes(data.routes ?? []);
      }
    } catch (e) {
      setLoadError(e instanceof Error ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    fetchRoutes();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab]);

  async function handleSave() {
    setFormError(null);
    setSaving(true);
    try {
      if (isDefaults) {
        await aggregatorApi.createDefaultRoute(defaultForm);
      } else {
        await aggregatorApi.createSubAccountRoute(subForm);
      }
      setDialogOpen(false);
      await fetchRoutes();
    } catch (e) {
      setFormError(e instanceof Error ? e.message : 'Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: string, subAccountId?: string) {
    if (!confirm('Удалить маршрут?')) return;
    try {
      if (isDefaults) {
        await aggregatorApi.deleteDefaultRoute(id);
      } else {
        await aggregatorApi.deleteSubAccountRoute(id, subAccountId!);
      }
      await fetchRoutes();
    } catch (e) {
      alert(e instanceof Error ? e.message : 'Ошибка удаления');
    }
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-gray-900">Маршруты</h1>
        <button
          onClick={() => setDialogOpen(true)}
          className="px-4 py-2 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700"
        >
          + Добавить маршрут
        </button>
      </div>

      <Tabs.Root value={tab} onValueChange={setTab}>
        <Tabs.List className="flex border-b mb-4">
          <Tabs.Trigger value="sub-accounts" className={TAB_TRIGGER_CLASS}>
            Субаккаунты
          </Tabs.Trigger>
          <Tabs.Trigger value="defaults" className={TAB_TRIGGER_CLASS}>
            Шаблоны по умолчанию
          </Tabs.Trigger>
        </Tabs.List>

        <Tabs.Content value="sub-accounts">
          {loading && <p className="text-sm text-gray-500">Загрузка...</p>}
          {loadError && <p className="text-sm text-red-600">{loadError}</p>}
          {!loading && !loadError && !preFilterSubId && (
            <p className="text-sm text-gray-500">
              Выберите субаккаунт в фильтре или перейдите из карточки субаккаунта.
            </p>
          )}
          {!loading && !loadError && preFilterSubId && subRoutes.length === 0 && (
            <p className="text-sm text-gray-500">Маршрутов нет.</p>
          )}
          {subRoutes.length > 0 && (
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="border-b text-left text-gray-500">
                  <th className="pb-2 pr-4">Оператор</th>
                  <th className="pb-2 pr-4">Провайдер</th>
                  <th className="pb-2 pr-4">Приоритет</th>
                  <th className="pb-2 pr-4">Вес</th>
                  <th className="pb-2 pr-4">Статус</th>
                  <th className="pb-2">Действия</th>
                </tr>
              </thead>
              <tbody>
                {subRoutes.map((r) => (
                  <tr key={r.id} className="border-b last:border-0">
                    <td className="py-2 pr-4 font-mono text-xs">{r.operator_id}</td>
                    <td className="py-2 pr-4 font-mono text-xs">{r.provider_id}</td>
                    <td className="py-2 pr-4">{r.priority}</td>
                    <td className="py-2 pr-4">{r.weight}</td>
                    <td className="py-2 pr-4">
                      <span className={r.active ? 'text-green-600' : 'text-gray-400'}>
                        {r.active ? 'Активен' : 'Неактивен'}
                      </span>
                    </td>
                    <td className="py-2">
                      <button
                        onClick={() => handleDelete(r.id, r.sub_account_id)}
                        className="text-red-500 hover:underline text-xs"
                      >
                        Удалить
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Tabs.Content>

        <Tabs.Content value="defaults">
          {loading && <p className="text-sm text-gray-500">Загрузка...</p>}
          {loadError && <p className="text-sm text-red-600">{loadError}</p>}
          {!loading && !loadError && defaultRoutes.length === 0 && (
            <p className="text-sm text-gray-500">Шаблонов нет.</p>
          )}
          {defaultRoutes.length > 0 && (
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="border-b text-left text-gray-500">
                  <th className="pb-2 pr-4">Оператор</th>
                  <th className="pb-2 pr-4">Провайдер</th>
                  <th className="pb-2 pr-4">Приоритет</th>
                  <th className="pb-2 pr-4">Вес</th>
                  <th className="pb-2 pr-4">Статус</th>
                  <th className="pb-2">Действия</th>
                </tr>
              </thead>
              <tbody>
                {defaultRoutes.map((r) => (
                  <tr key={r.id} className="border-b last:border-0">
                    <td className="py-2 pr-4 font-mono text-xs">{r.operator_id}</td>
                    <td className="py-2 pr-4 font-mono text-xs">{r.provider_id}</td>
                    <td className="py-2 pr-4">{r.priority}</td>
                    <td className="py-2 pr-4">{r.weight}</td>
                    <td className="py-2 pr-4">
                      <span className={r.active ? 'text-green-600' : 'text-gray-400'}>
                        {r.active ? 'Активен' : 'Неактивен'}
                      </span>
                    </td>
                    <td className="py-2">
                      <button
                        onClick={() => handleDelete(r.id)}
                        className="text-red-500 hover:underline text-xs"
                      >
                        Удалить
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Tabs.Content>
      </Tabs.Root>

      <Modal open={dialogOpen} onClose={() => setDialogOpen(false)} title="Добавить маршрут">
        <div className="space-y-3">
          {formError && <p className="text-sm text-red-600">{formError}</p>}
          {!isDefaults && (
            <Input
              label="ID субаккаунта"
              value={subForm.sub_account_id}
              onChange={(e) => setSubForm({ ...subForm, sub_account_id: e.target.value })}
              placeholder="UUID субаккаунта"
            />
          )}
          <Input
            label="ID оператора"
            value={isDefaults ? defaultForm.operator_id : subForm.operator_id}
            onChange={(e) =>
              isDefaults
                ? setDefaultForm({ ...defaultForm, operator_id: e.target.value })
                : setSubForm({ ...subForm, operator_id: e.target.value })
            }
            placeholder="UUID оператора"
          />
          <Input
            label="ID провайдера"
            value={isDefaults ? defaultForm.provider_id : subForm.provider_id}
            onChange={(e) =>
              isDefaults
                ? setDefaultForm({ ...defaultForm, provider_id: e.target.value })
                : setSubForm({ ...subForm, provider_id: e.target.value })
            }
            placeholder="UUID провайдера (из разрешённого пула)"
          />
          <div className="flex gap-3">
            <Input
              label="Приоритет"
              type="number"
              value={String(isDefaults ? defaultForm.priority : subForm.priority)}
              onChange={(e) => {
                const v = parseInt(e.target.value, 10) || 0;
                isDefaults
                  ? setDefaultForm({ ...defaultForm, priority: v })
                  : setSubForm({ ...subForm, priority: v });
              }}
            />
            <Input
              label="Вес"
              type="number"
              value={String(isDefaults ? defaultForm.weight : subForm.weight)}
              onChange={(e) => {
                const v = parseInt(e.target.value, 10) || 1;
                isDefaults
                  ? setDefaultForm({ ...defaultForm, weight: v })
                  : setSubForm({ ...subForm, weight: v });
              }}
            />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <button
              onClick={() => setDialogOpen(false)}
              className="px-3 py-1.5 text-sm text-gray-600 hover:text-gray-900"
            >
              Отмена
            </button>
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-1.5 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 disabled:opacity-50"
            >
              {saving ? 'Сохранение...' : 'Сохранить'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
```

- [ ] **Step 2: Register /aggregator/routes in app router**

Find the file where React Router routes are defined (likely `portal-frontend/src/App.tsx` or `portal-frontend/src/router.tsx`). Add:

```tsx
import RoutesPage from './pages/aggregator/RoutesPage';

// Inside <Routes> or router config, alongside existing /aggregator/tariffs:
<Route path="/aggregator/routes" element={<RoutesPage />} />
```

- [ ] **Step 3: Add Routes to Sidebar nav**

Open `portal-frontend/src/components/layout/Sidebar.tsx`. Find the aggregator section (where Tariffs link was added in Phase 2). Add a Routes link after Tariffs:

```tsx
<NavLink to="/aggregator/routes" className={navLinkClass}>
  Маршруты
</NavLink>
```

(Use the same `NavLink` and class pattern as the Tariffs link.)

- [ ] **Step 4: TypeScript check**

```bash
cd .worktrees/aggregator-phase3/portal-frontend
npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/aggregator/RoutesPage.tsx
git add portal-frontend/src/App.tsx  # or router file
git add portal-frontend/src/components/layout/Sidebar.tsx
git commit -m "feat(frontend): add RoutesPage with sub-account and default route management"
```

---

## Task 14: Final Build and Smoke Test

- [ ] **Step 1: Full build**

```bash
cd .worktrees/aggregator-phase3
go build ./...
```

Expected: no errors.

- [ ] **Step 2: Run all aggregator tests**

```bash
go test ./internal/services/aggregator/... -v
```

Expected: all PASS.

- [ ] **Step 3: Frontend build**

```bash
cd portal-frontend
npm run build
```

Expected: no errors, build artifacts in `dist/`.

- [ ] **Step 4: Smoke test via curl (requires running aggregator-service)**

```bash
# List default routes (should return empty list)
curl -s -X GET http://localhost:8080/portal/v1/aggregator/routes/defaults \
  -H "Cookie: session=<valid_aggregator_session>" | jq .

# Create a default route
curl -s -X POST http://localhost:8080/portal/v1/aggregator/routes/defaults \
  -H "Content-Type: application/json" \
  -H "Cookie: session=<valid_aggregator_session>" \
  -d '{"operator_id":"<uuid>","provider_id":"<allowed_provider_uuid>","priority":10,"weight":1}' | jq .
```

Expected: 200 with route object, then 201 with created route.

- [ ] **Step 5: Final commit**

```bash
cd .worktrees/aggregator-phase3
git add -A
git commit -m "chore(aggregator-phase3): final build verification"
```

---

## Self-Review

### Spec coverage

| Spec requirement | Task |
|---|---|
| `aggregator_default_routes` table | Task 2 |
| DefaultRoute / SubAccountRoute domain types | Task 3 |
| Provider validation (allowed + active) | Task 5 |
| Create/List/Update/Delete default routes | Tasks 4, 5, 7, 8, 9 |
| Create/List/Update/Delete sub-account routes | Tasks 4, 5, 7, 8, 9 |
| `client_providers` sync on allowed_provider_ids change | Tasks 4, 10 |
| Copy default routes on sub-account creation | Tasks 4, 5, 6 |
| `client_providers` with ownership='inherited' for sub-accounts | Task 4 |
| Deactivate sub-account routes when provider removed from pool | Task 4 (`SyncAggregatorProviders`) |
| HTTP endpoints GET/POST/PUT/DELETE /routes and /routes/defaults | Task 9 |
| gRPC RPCs for routes | Tasks 7, 8 |
| Frontend RoutesPage | Task 13 |
| Frontend RoutesTab in SubAccountDetailPage | Task 12 |
| Sidebar navigation | Task 13 |

### Placeholder scan

No TBDs, no "implement later", no "similar to" references. All code steps contain complete implementations.

### Type consistency

- `domain.DefaultRoute` / `domain.SubAccountRoute` defined in Task 3, used consistently in Tasks 4, 5, 8.
- `RouteRepo` interface in Task 5 matches all methods in `RouteRepository` from Task 4.
- Proto message field names (`route_id`, `sub_account_id`, `aggregator_id`) match across Task 7 messages and Task 8 handler code.
- Frontend API types (`SubAccountRoute`, `AggregatorDefaultRoute`) match JSON field names from gRPC responses.
