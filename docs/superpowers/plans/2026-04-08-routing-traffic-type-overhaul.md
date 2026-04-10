# Routing Overhaul: Traffic Type, Conditions, Schedules Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Overhaul the SMS routing system with traffic types on templates, complex condition groups with logical operators, schedule support, and a fully redesigned admin routing UI.

**Architecture:** Extends the existing `client_routes` table with new columns (name, comment, status, share, route_type) and allows `client_id = NULL` for default routes. Adds normalized child tables for condition groups and schedules. A new in-memory `RouteMatcher` evaluates conditions at runtime. The portal frontend gets a complete RoutingPage replacement with a rich RouteModal for creating/editing routes with condition editors and schedule support.

**Tech Stack:** Go 1.24 (pgx/v5, gorilla/mux, zerolog, sarama), PostgreSQL 15+, Redis 7+, TypeScript 5.7 + React 19 + Vite + Tailwind CSS 4.2

**Spec:** `docs/superpowers/specs/2026-04-08-routing-traffic-type-design.md`

**Important codebase notes:**
- `client_routes.id` is `UUID`, not `BIGINT` — all FK references must use `UUID`
- The portal gateway HTTP handlers delegate to the routing gRPC service
- Existing pattern: HTTP handler → gRPC client → gRPC server → repository → PostgreSQL
- Frontend uses `apiFetch()` from `portal-frontend/src/api/client.ts`

---

## Task 1: Database Migration — Schema Extension

**Files:**
- Create: `migrations/000077_routing_overhaul.up.sql`
- Create: `migrations/000077_routing_overhaul.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- migrations/000077_routing_overhaul.up.sql

-- 1. Extend client_routes
ALTER TABLE client_routes
  ADD COLUMN name VARCHAR(255),
  ADD COLUMN comment TEXT,
  ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'active',
  ADD COLUMN share INTEGER NOT NULL DEFAULT 100,
  ADD COLUMN route_type VARCHAR(10) NOT NULL DEFAULT 'sms';

-- Allow client_id to be NULL (NULL = default route)
ALTER TABLE client_routes ALTER COLUMN client_id DROP NOT NULL;

-- Drop the old unique constraint that requires client_id
-- (existing constraint: UNIQUE(client_id, operator_id, provider_id))
ALTER TABLE client_routes DROP CONSTRAINT IF EXISTS client_routes_client_id_operator_id_provider_id_key;

-- 2. Condition groups
CREATE TABLE route_condition_groups (
  id          BIGSERIAL PRIMARY KEY,
  route_id    UUID NOT NULL REFERENCES client_routes(id) ON DELETE CASCADE,
  group_index SMALLINT NOT NULL,
  logic_op    VARCHAR(10) NOT NULL,  -- 'IF', 'AND', 'AND_NOT', 'OR', 'OR_NOT'
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rcg_route ON route_condition_groups(route_id);

-- 3. Conditions
CREATE TABLE route_conditions (
  id              BIGSERIAL PRIMARY KEY,
  group_id        BIGINT NOT NULL REFERENCES route_condition_groups(id) ON DELETE CASCADE,
  condition_type  VARCHAR(30) NOT NULL,  -- 'operator', 'country', 'traffic_type', 'paid_name', 'regex'
  condition_value TEXT NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rc_group ON route_conditions(group_id);

-- 4. Schedules
CREATE TABLE route_schedules (
  id         BIGSERIAL PRIMARY KEY,
  route_id   UUID NOT NULL REFERENCES client_routes(id) ON DELETE CASCADE,
  date_from  DATE,
  date_to    DATE,
  time_from  TIME,
  time_to    TIME,
  weekdays   SMALLINT NOT NULL DEFAULT 127,  -- bitmask: 1=Mon, 2=Tue, 4=Wed, 8=Thu, 16=Fri, 32=Sat, 64=Sun; 127=all
  timezone   VARCHAR(50) NOT NULL DEFAULT 'Europe/Moscow',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rs_route ON route_schedules(route_id);

-- 5. Add traffic_type to templates
ALTER TABLE templates ADD COLUMN traffic_type VARCHAR(20) NOT NULL DEFAULT 'transactional';
```

- [ ] **Step 2: Write the down migration**

```sql
-- migrations/000077_routing_overhaul.down.sql
ALTER TABLE templates DROP COLUMN IF EXISTS traffic_type;
DROP TABLE IF EXISTS route_schedules;
DROP TABLE IF EXISTS route_conditions;
DROP TABLE IF EXISTS route_condition_groups;
ALTER TABLE client_routes
  DROP COLUMN IF EXISTS name,
  DROP COLUMN IF EXISTS comment,
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS share,
  DROP COLUMN IF EXISTS route_type;
ALTER TABLE client_routes ALTER COLUMN client_id SET NOT NULL;
```

- [ ] **Step 3: Run migration on server**

```bash
scripts/server.sh migrate
```

- [ ] **Step 4: Commit**

```bash
git add migrations/000077_routing_overhaul.up.sql migrations/000077_routing_overhaul.down.sql
git commit -m "feat(routing): add schema for condition groups, schedules, traffic type"
```

---

## Task 2: Database Migration — Data Migration for Existing Routes

**Files:**
- Create: `migrations/000078_migrate_existing_routes.up.sql`
- Create: `migrations/000078_migrate_existing_routes.down.sql`

- [ ] **Step 1: Write data migration**

```sql
-- migrations/000078_migrate_existing_routes.up.sql

-- Set name for existing routes based on operator
UPDATE client_routes SET
  name = 'Route ' || id::text,
  status = CASE WHEN active THEN 'active' ELSE 'draft' END,
  share = weight,
  route_type = 'sms'
WHERE name IS NULL;

-- Create IF condition group with operator condition for each existing route
INSERT INTO route_condition_groups (route_id, group_index, logic_op)
SELECT id, 0, 'IF' FROM client_routes WHERE operator_id IS NOT NULL;

-- Create operator condition for each group
INSERT INTO route_conditions (group_id, condition_type, condition_value)
SELECT rcg.id, 'operator', cr.operator_id::text
FROM route_condition_groups rcg
JOIN client_routes cr ON cr.id = rcg.route_id
WHERE rcg.logic_op = 'IF'
  AND NOT EXISTS (SELECT 1 FROM route_conditions rc WHERE rc.group_id = rcg.id);
```

- [ ] **Step 2: Write down migration**

```sql
-- migrations/000078_migrate_existing_routes.down.sql
-- Data migration is not easily reversible; this is a no-op
-- The schema down migration (000077) handles structural rollback
```

- [ ] **Step 3: Run migration**

```bash
scripts/server.sh migrate
```

- [ ] **Step 4: Commit**

```bash
git add migrations/000078_migrate_existing_routes.up.sql migrations/000078_migrate_existing_routes.down.sql
git commit -m "feat(routing): migrate existing routes to condition-based format"
```

---

## Task 3: Backend Domain Types

**Files:**
- Modify: `internal/services/routing/domain/client_route.go`
- Create: `internal/services/routing/domain/route_types.go`

- [ ] **Step 1: Create route_types.go with new domain types**

```go
// internal/services/routing/domain/route_types.go
package domain

import "time"

type TrafficType string

const (
	TrafficTypeAuthorization TrafficType = "authorization"
	TrafficTypeTransactional TrafficType = "transactional"
	TrafficTypeService       TrafficType = "service"
)

func ValidTrafficType(s string) bool {
	switch TrafficType(s) {
	case TrafficTypeAuthorization, TrafficTypeTransactional, TrafficTypeService:
		return true
	}
	return false
}

type RouteStatus string

const (
	RouteStatusActive RouteStatus = "active"
	RouteStatusDraft  RouteStatus = "draft"
)

func ValidRouteStatus(s string) bool {
	switch RouteStatus(s) {
	case RouteStatusActive, RouteStatusDraft:
		return true
	}
	return false
}

type ConditionType string

const (
	ConditionOperator    ConditionType = "operator"
	ConditionCountry     ConditionType = "country"
	ConditionTrafficType ConditionType = "traffic_type"
	ConditionPaidName    ConditionType = "paid_name"
	ConditionRegex       ConditionType = "regex"
)

func ValidConditionType(s string) bool {
	switch ConditionType(s) {
	case ConditionOperator, ConditionCountry, ConditionTrafficType, ConditionPaidName, ConditionRegex:
		return true
	}
	return false
}

type LogicOp string

const (
	LogicIf     LogicOp = "IF"
	LogicAnd    LogicOp = "AND"
	LogicAndNot LogicOp = "AND_NOT"
	LogicOr     LogicOp = "OR"
	LogicOrNot  LogicOp = "OR_NOT"
)

func ValidLogicOp(s string) bool {
	switch LogicOp(s) {
	case LogicIf, LogicAnd, LogicAndNot, LogicOr, LogicOrNot:
		return true
	}
	return false
}

type ConditionGroup struct {
	ID         int64
	GroupIndex int
	LogicOp    LogicOp
	Conditions []Condition
}

type Condition struct {
	ID    int64
	Type  ConditionType
	Value string
}

type Schedule struct {
	ID       int64
	DateFrom *time.Time
	DateTo   *time.Time
	TimeFrom *string
	TimeTo   *string
	Weekdays int
	Timezone string
}
```

- [ ] **Step 2: Extend ClientRoute struct in client_route.go**

Add new fields to the existing `ClientRoute` struct in `internal/services/routing/domain/client_route.go`:

```go
type ClientRoute struct {
	ID         uuid.UUID
	ClientID   *uuid.UUID       // nil = default route
	OperatorID *uuid.UUID       // legacy field, kept for backward compat
	ProviderID uuid.UUID
	Priority   int
	Weight     int
	Active     bool
	Name       string
	Comment    string
	Status     RouteStatus
	Share      int
	RouteType  string           // sms, hlr, max
	Groups     []ConditionGroup
	Schedules  []Schedule
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
```

Update `NewClientRoute` to accept new parameters:

```go
func NewRoute(clientID *uuid.UUID, providerID uuid.UUID, name string, routeType string, priority, share int, status RouteStatus) *ClientRoute {
	now := time.Now()
	return &ClientRoute{
		ID:         uuid.New(),
		ClientID:   clientID,
		ProviderID: providerID,
		Name:       name,
		RouteType:  routeType,
		Priority:   priority,
		Share:      share,
		Status:     status,
		Weight:     share,
		Active:     status == RouteStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/routing/domain/route_types.go internal/services/routing/domain/client_route.go
git commit -m "feat(routing): add domain types for conditions, schedules, traffic type"
```

---

## Task 4: Backend Repository — Route CRUD with Conditions & Schedules

**Files:**
- Create: `internal/services/routing/infrastructure/route_repo.go`

This is a new repository that handles the full route lifecycle (route + condition groups + conditions + schedules) in transactions.

- [ ] **Step 1: Create route_repo.go**

```go
// internal/services/routing/infrastructure/route_repo.go
package infrastructure

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

type RouteRepo struct {
	pool *pgxpool.Pool
}

func NewRouteRepo(pool *pgxpool.Pool) *RouteRepo {
	return &RouteRepo{pool: pool}
}

// Create inserts a route with its condition groups, conditions, and schedules in a single transaction.
func (r *RouteRepo) Create(ctx context.Context, route *domain.ClientRoute) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO client_routes (id, client_id, operator_id, provider_id, priority, weight, active, name, comment, status, share, route_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		route.ID, route.ClientID, route.OperatorID, route.ProviderID,
		route.Priority, route.Weight, route.Active,
		route.Name, route.Comment, route.Status, route.Share, route.RouteType,
		route.CreatedAt, route.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert route: %w", err)
	}

	if err := r.insertGroupsAndConditions(ctx, tx, route.ID, route.Groups); err != nil {
		return err
	}
	if err := r.insertSchedules(ctx, tx, route.ID, route.Schedules); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Update replaces the route and its children (full replace strategy).
func (r *RouteRepo) Update(ctx context.Context, route *domain.ClientRoute) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE client_routes SET client_id=$1, operator_id=$2, provider_id=$3, priority=$4, weight=$5, active=$6,
		name=$7, comment=$8, status=$9, share=$10, route_type=$11, updated_at=now()
		WHERE id=$12`,
		route.ClientID, route.OperatorID, route.ProviderID,
		route.Priority, route.Weight, route.Active,
		route.Name, route.Comment, route.Status, route.Share, route.RouteType,
		route.ID,
	)
	if err != nil {
		return fmt.Errorf("update route: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientRouteNotFound
	}

	// Delete old children (CASCADE handles conditions via group FK)
	_, _ = tx.Exec(ctx, `DELETE FROM route_condition_groups WHERE route_id = $1`, route.ID)
	_, _ = tx.Exec(ctx, `DELETE FROM route_schedules WHERE route_id = $1`, route.ID)

	if err := r.insertGroupsAndConditions(ctx, tx, route.ID, route.Groups); err != nil {
		return err
	}
	if err := r.insertSchedules(ctx, tx, route.ID, route.Schedules); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetByID loads a route with all condition groups, conditions, and schedules.
func (r *RouteRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ClientRoute, error) {
	route := &domain.ClientRoute{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, client_id, operator_id, provider_id, priority, weight, active,
		 name, comment, status, share, route_type, created_at, updated_at
		 FROM client_routes WHERE id = $1`, id,
	).Scan(
		&route.ID, &route.ClientID, &route.OperatorID, &route.ProviderID,
		&route.Priority, &route.Weight, &route.Active,
		&route.Name, &route.Comment, &route.Status, &route.Share, &route.RouteType,
		&route.CreatedAt, &route.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrClientRouteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get route: %w", err)
	}

	groups, err := r.loadGroups(ctx, id)
	if err != nil {
		return nil, err
	}
	route.Groups = groups

	schedules, err := r.loadSchedules(ctx, id)
	if err != nil {
		return nil, err
	}
	route.Schedules = schedules

	return route, nil
}

// List returns routes with optional filters. Does NOT load children (use GetByID for full detail).
func (r *RouteRepo) List(ctx context.Context, filters RouteFilters) ([]*domain.ClientRoute, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if filters.ClientID != nil {
		where += fmt.Sprintf(" AND client_id = $%d", argIdx)
		args = append(args, *filters.ClientID)
		argIdx++
	} else if filters.DefaultOnly {
		where += " AND client_id IS NULL"
	}
	if filters.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filters.Status)
		argIdx++
	}
	if filters.RouteType != "" {
		where += fmt.Sprintf(" AND route_type = $%d", argIdx)
		args = append(args, filters.RouteType)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM client_routes " + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count routes: %w", err)
	}

	query := `SELECT id, client_id, operator_id, provider_id, priority, weight, active,
		name, comment, status, share, route_type, created_at, updated_at
		FROM client_routes ` + where + " ORDER BY priority ASC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list routes: %w", err)
	}
	defer rows.Close()

	var routes []*domain.ClientRoute
	for rows.Next() {
		route := &domain.ClientRoute{}
		if err := rows.Scan(
			&route.ID, &route.ClientID, &route.OperatorID, &route.ProviderID,
			&route.Priority, &route.Weight, &route.Active,
			&route.Name, &route.Comment, &route.Status, &route.Share, &route.RouteType,
			&route.CreatedAt, &route.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, route)
	}

	return routes, total, nil
}

// Delete removes a route (CASCADE deletes children).
func (r *RouteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM client_routes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete route: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrClientRouteNotFound
	}
	return nil
}

// LoadAllActive loads all active routes with full children for the RouteMatcher.
func (r *RouteRepo) LoadAllActive(ctx context.Context) ([]*domain.ClientRoute, error) {
	query := `SELECT id, client_id, operator_id, provider_id, priority, weight, active,
		name, comment, status, share, route_type, created_at, updated_at
		FROM client_routes WHERE status = 'active' ORDER BY priority ASC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load active routes: %w", err)
	}
	defer rows.Close()

	var routes []*domain.ClientRoute
	for rows.Next() {
		route := &domain.ClientRoute{}
		if err := rows.Scan(
			&route.ID, &route.ClientID, &route.OperatorID, &route.ProviderID,
			&route.Priority, &route.Weight, &route.Active,
			&route.Name, &route.Comment, &route.Status, &route.Share, &route.RouteType,
			&route.CreatedAt, &route.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, route)
	}

	// Load children for each route
	for _, route := range routes {
		groups, err := r.loadGroups(ctx, route.ID)
		if err != nil {
			return nil, err
		}
		route.Groups = groups

		schedules, err := r.loadSchedules(ctx, route.ID)
		if err != nil {
			return nil, err
		}
		route.Schedules = schedules
	}

	return routes, nil
}

// --- private helpers ---

func (r *RouteRepo) insertGroupsAndConditions(ctx context.Context, tx pgx.Tx, routeID uuid.UUID, groups []domain.ConditionGroup) error {
	for _, g := range groups {
		var groupID int64
		err := tx.QueryRow(ctx,
			`INSERT INTO route_condition_groups (route_id, group_index, logic_op) VALUES ($1, $2, $3) RETURNING id`,
			routeID, g.GroupIndex, g.LogicOp,
		).Scan(&groupID)
		if err != nil {
			return fmt.Errorf("insert condition group: %w", err)
		}
		for _, c := range g.Conditions {
			_, err := tx.Exec(ctx,
				`INSERT INTO route_conditions (group_id, condition_type, condition_value) VALUES ($1, $2, $3)`,
				groupID, c.Type, c.Value,
			)
			if err != nil {
				return fmt.Errorf("insert condition: %w", err)
			}
		}
	}
	return nil
}

func (r *RouteRepo) insertSchedules(ctx context.Context, tx pgx.Tx, routeID uuid.UUID, schedules []domain.Schedule) error {
	for _, s := range schedules {
		_, err := tx.Exec(ctx,
			`INSERT INTO route_schedules (route_id, date_from, date_to, time_from, time_to, weekdays, timezone)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			routeID, s.DateFrom, s.DateTo, s.TimeFrom, s.TimeTo, s.Weekdays, s.Timezone,
		)
		if err != nil {
			return fmt.Errorf("insert schedule: %w", err)
		}
	}
	return nil
}

func (r *RouteRepo) loadGroups(ctx context.Context, routeID uuid.UUID) ([]domain.ConditionGroup, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, group_index, logic_op FROM route_condition_groups WHERE route_id = $1 ORDER BY group_index`, routeID)
	if err != nil {
		return nil, fmt.Errorf("load groups: %w", err)
	}
	defer rows.Close()

	var groups []domain.ConditionGroup
	for rows.Next() {
		var g domain.ConditionGroup
		if err := rows.Scan(&g.ID, &g.GroupIndex, &g.LogicOp); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, g)
	}

	for i := range groups {
		conds, err := r.loadConditions(ctx, groups[i].ID)
		if err != nil {
			return nil, err
		}
		groups[i].Conditions = conds
	}

	return groups, nil
}

func (r *RouteRepo) loadConditions(ctx context.Context, groupID int64) ([]domain.Condition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, condition_type, condition_value FROM route_conditions WHERE group_id = $1`, groupID)
	if err != nil {
		return nil, fmt.Errorf("load conditions: %w", err)
	}
	defer rows.Close()

	var conds []domain.Condition
	for rows.Next() {
		var c domain.Condition
		if err := rows.Scan(&c.ID, &c.Type, &c.Value); err != nil {
			return nil, fmt.Errorf("scan condition: %w", err)
		}
		conds = append(conds, c)
	}
	return conds, nil
}

func (r *RouteRepo) loadSchedules(ctx context.Context, routeID uuid.UUID) ([]domain.Schedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, date_from, date_to, time_from, time_to, weekdays, timezone
		FROM route_schedules WHERE route_id = $1`, routeID)
	if err != nil {
		return nil, fmt.Errorf("load schedules: %w", err)
	}
	defer rows.Close()

	var scheds []domain.Schedule
	for rows.Next() {
		var s domain.Schedule
		if err := rows.Scan(&s.ID, &s.DateFrom, &s.DateTo, &s.TimeFrom, &s.TimeTo, &s.Weekdays, &s.Timezone); err != nil {
			return nil, fmt.Errorf("scan schedule: %w", err)
		}
		scheds = append(scheds, s)
	}
	return scheds, nil
}

type RouteFilters struct {
	ClientID    *uuid.UUID
	DefaultOnly bool
	Status      string
	RouteType   string
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /c/projects/sms && go build ./internal/services/routing/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/routing/infrastructure/route_repo.go
git commit -m "feat(routing): add RouteRepo with transactional CRUD for routes, conditions, schedules"
```

---

## Task 5: Portal HTTP Handlers for Route CRUD

**Files:**
- Create: `internal/gateway/portal/handlers/routes.go`

These handlers manage routes directly via the `RouteRepo` (no gRPC for this new API, since the portal-gateway already has a pgxpool connection). This follows the same pattern as `GetRoutingMode`/`SetRoutingMode` which query the DB directly.

- [ ] **Step 1: Create routes.go**

```go
// internal/gateway/portal/handlers/routes.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/infrastructure"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type RouteHandlers struct {
	repo *infrastructure.RouteRepo
}

func NewRouteHandlers(repo *infrastructure.RouteRepo) *RouteHandlers {
	return &RouteHandlers{repo: repo}
}

// --- JSON request/response types ---

type routeRequest struct {
	ClientID   *string              `json:"client_id"`
	Name       string               `json:"name"`
	Comment    string               `json:"comment"`
	Status     string               `json:"status"`
	RouteType  string               `json:"route_type"`
	OperatorID *string              `json:"operator_id"`
	ProviderID string               `json:"provider_id"`
	Priority   int                  `json:"priority"`
	Share      int                  `json:"share"`
	Groups     []conditionGroupJSON `json:"condition_groups"`
	Schedules  []scheduleJSON       `json:"schedules"`
}

type conditionGroupJSON struct {
	LogicOp    string          `json:"logic_op"`
	Conditions []conditionJSON `json:"conditions"`
}

type conditionJSON struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type scheduleJSON struct {
	DateFrom *string `json:"date_from"`
	DateTo   *string `json:"date_to"`
	TimeFrom *string `json:"time_from"`
	TimeTo   *string `json:"time_to"`
	Weekdays int     `json:"weekdays"`
	Timezone string  `json:"timezone"`
}

type routeResponse struct {
	ID              string               `json:"id"`
	ClientID        *string              `json:"client_id"`
	Name            string               `json:"name"`
	Comment         string               `json:"comment"`
	Status          string               `json:"status"`
	RouteType       string               `json:"route_type"`
	OperatorID      *string              `json:"operator_id,omitempty"`
	ProviderID      string               `json:"provider_id"`
	Priority        int                  `json:"priority"`
	Share           int                  `json:"share"`
	ConditionGroups []conditionGroupJSON `json:"condition_groups"`
	Schedules       []scheduleJSON       `json:"schedules"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

type routeListItem struct {
	ID              string   `json:"id"`
	ClientID        *string  `json:"client_id"`
	Name            string   `json:"name"`
	Status          string   `json:"status"`
	RouteType       string   `json:"route_type"`
	ProviderID      string   `json:"provider_id"`
	Priority        int      `json:"priority"`
	Share           int      `json:"share"`
	Comment         string   `json:"comment"`
	ConditionTags   []string `json:"condition_tags"`
	ScheduleSummary string   `json:"schedule_summary"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateRoute POST /portal/v1/routes
func (h *RouteHandlers) CreateRoute(w http.ResponseWriter, r *http.Request) {
	var req routeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if err := validateRouteRequest(&req); err != nil {
		respondError(w, err)
		return
	}

	route, err := requestToRoute(&req)
	if err != nil {
		respondError(w, shared.ErrInvalidInput(err.Error()))
		return
	}

	if err := h.repo.Create(r.Context(), route); err != nil {
		log.Error().Err(err).Msg("create route failed")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	// Reload with full data
	created, err := h.repo.GetByID(r.Context(), route.ID)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	respondJSON(w, http.StatusCreated, routeToResponse(created))
}

// ListRoutes GET /portal/v1/routes
func (h *RouteHandlers) ListRoutes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filters := infrastructure.RouteFilters{
		Status:    q.Get("status"),
		RouteType: q.Get("route_type"),
	}
	if cid := q.Get("client_id"); cid != "" {
		parsed, err := uuid.Parse(cid)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("invalid client_id"))
			return
		}
		filters.ClientID = &parsed
	} else {
		filters.DefaultOnly = q.Get("default") == "true"
	}

	routes, total, err := h.repo.List(r.Context(), filters)
	if err != nil {
		log.Error().Err(err).Msg("list routes failed")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	// Load condition tags for list display
	items := make([]routeListItem, 0, len(routes))
	for _, route := range routes {
		full, err := h.repo.GetByID(r.Context(), route.ID)
		if err != nil {
			continue
		}
		items = append(items, routeToListItem(full))
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"routes": items,
		"total":  total,
	})
}

// GetRoute GET /portal/v1/routes/{id}
func (h *RouteHandlers) GetRoute(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid route id"))
		return
	}

	route, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, shared.ErrNotFound("маршрут не найден"))
		return
	}
	respondJSON(w, http.StatusOK, routeToResponse(route))
}

// UpdateRoute PUT /portal/v1/routes/{id}
func (h *RouteHandlers) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid route id"))
		return
	}

	var req routeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if err := validateRouteRequest(&req); err != nil {
		respondError(w, err)
		return
	}

	route, err := requestToRoute(&req)
	if err != nil {
		respondError(w, shared.ErrInvalidInput(err.Error()))
		return
	}
	route.ID = id

	if err := h.repo.Update(r.Context(), route); err != nil {
		if err == domain.ErrClientRouteNotFound {
			respondError(w, shared.ErrNotFound("маршрут не найден"))
			return
		}
		log.Error().Err(err).Msg("update route failed")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	updated, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, routeToResponse(updated))
}

// DeleteRoute DELETE /portal/v1/routes/{id}
func (h *RouteHandlers) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid route id"))
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		if err == domain.ErrClientRouteNotFound {
			respondError(w, shared.ErrNotFound("маршрут не найден"))
			return
		}
		log.Error().Err(err).Msg("delete route failed")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetReferences GET /portal/v1/routes/references
func (h *RouteHandlers) GetReferences(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"traffic_types": []string{"authorization", "transactional", "service"},
		"route_types":   []string{"sms", "hlr", "max"},
		"statuses":      []string{"active", "draft"},
		"logic_ops":     []string{"IF", "AND", "AND_NOT", "OR", "OR_NOT"},
		"condition_types": []string{"operator", "country", "traffic_type", "paid_name", "regex"},
	})
}

// --- helper functions ---

func validateRouteRequest(req *routeRequest) *shared.AppError {
	if req.ProviderID == "" {
		return shared.ErrInvalidInput("provider_id обязателен")
	}
	if req.RouteType == "" {
		req.RouteType = "sms"
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if !domain.ValidRouteStatus(req.Status) {
		return shared.ErrInvalidInput("status должен быть active или draft")
	}
	if req.Share <= 0 {
		req.Share = 100
	}
	for i, g := range req.Groups {
		if !domain.ValidLogicOp(g.LogicOp) {
			return shared.ErrInvalidInput(fmt.Sprintf("condition_groups[%d].logic_op недопустимое значение: %s", i, g.LogicOp))
		}
		for j, c := range g.Conditions {
			if !domain.ValidConditionType(c.Type) {
				return shared.ErrInvalidInput(fmt.Sprintf("condition_groups[%d].conditions[%d].type недопустимый: %s", i, j, c.Type))
			}
		}
	}
	return nil
}

func requestToRoute(req *routeRequest) (*domain.ClientRoute, error) {
	providerID, err := uuid.Parse(req.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("invalid provider_id: %w", err)
	}

	var clientID *uuid.UUID
	if req.ClientID != nil && *req.ClientID != "" {
		parsed, err := uuid.Parse(*req.ClientID)
		if err != nil {
			return nil, fmt.Errorf("invalid client_id: %w", err)
		}
		clientID = &parsed
	}

	var operatorID *uuid.UUID
	if req.OperatorID != nil && *req.OperatorID != "" {
		parsed, err := uuid.Parse(*req.OperatorID)
		if err != nil {
			return nil, fmt.Errorf("invalid operator_id: %w", err)
		}
		operatorID = &parsed
	}

	route := domain.NewRoute(clientID, providerID, req.Name, req.RouteType, req.Priority, req.Share, domain.RouteStatus(req.Status))
	route.OperatorID = operatorID
	route.Comment = req.Comment

	for i, g := range req.Groups {
		group := domain.ConditionGroup{
			GroupIndex: i,
			LogicOp:    domain.LogicOp(g.LogicOp),
		}
		for _, c := range g.Conditions {
			group.Conditions = append(group.Conditions, domain.Condition{
				Type:  domain.ConditionType(c.Type),
				Value: c.Value,
			})
		}
		route.Groups = append(route.Groups, group)
	}

	for _, s := range req.Schedules {
		sched := domain.Schedule{
			Weekdays: s.Weekdays,
			Timezone: s.Timezone,
		}
		if s.DateFrom != nil {
			t, err := time.Parse("2006-01-02", *s.DateFrom)
			if err == nil {
				sched.DateFrom = &t
			}
		}
		if s.DateTo != nil {
			t, err := time.Parse("2006-01-02", *s.DateTo)
			if err == nil {
				sched.DateTo = &t
			}
		}
		sched.TimeFrom = s.TimeFrom
		sched.TimeTo = s.TimeTo
		if sched.Timezone == "" {
			sched.Timezone = "Europe/Moscow"
		}
		if sched.Weekdays == 0 {
			sched.Weekdays = 127
		}
		route.Schedules = append(route.Schedules, sched)
	}

	return route, nil
}

func routeToResponse(r *domain.ClientRoute) routeResponse {
	resp := routeResponse{
		ID:         r.ID.String(),
		Name:       r.Name,
		Comment:    r.Comment,
		Status:     string(r.Status),
		RouteType:  r.RouteType,
		ProviderID: r.ProviderID.String(),
		Priority:   r.Priority,
		Share:      r.Share,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if r.ClientID != nil {
		s := r.ClientID.String()
		resp.ClientID = &s
	}
	if r.OperatorID != nil {
		s := r.OperatorID.String()
		resp.OperatorID = &s
	}
	resp.ConditionGroups = make([]conditionGroupJSON, 0, len(r.Groups))
	for _, g := range r.Groups {
		gj := conditionGroupJSON{LogicOp: string(g.LogicOp)}
		for _, c := range g.Conditions {
			gj.Conditions = append(gj.Conditions, conditionJSON{Type: string(c.Type), Value: c.Value})
		}
		resp.ConditionGroups = append(resp.ConditionGroups, gj)
	}
	resp.Schedules = make([]scheduleJSON, 0, len(r.Schedules))
	for _, s := range r.Schedules {
		sj := scheduleJSON{Weekdays: s.Weekdays, Timezone: s.Timezone}
		if s.DateFrom != nil {
			v := s.DateFrom.Format("2006-01-02")
			sj.DateFrom = &v
		}
		if s.DateTo != nil {
			v := s.DateTo.Format("2006-01-02")
			sj.DateTo = &v
		}
		sj.TimeFrom = s.TimeFrom
		sj.TimeTo = s.TimeTo
		resp.Schedules = append(resp.Schedules, sj)
	}
	return resp
}

func routeToListItem(r *domain.ClientRoute) routeListItem {
	item := routeListItem{
		ID:         r.ID.String(),
		Name:       r.Name,
		Status:     string(r.Status),
		RouteType:  r.RouteType,
		ProviderID: r.ProviderID.String(),
		Priority:   r.Priority,
		Share:      r.Share,
		Comment:    r.Comment,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if r.ClientID != nil {
		s := r.ClientID.String()
		item.ClientID = &s
	}

	// Build condition tags
	tags := []string{r.RouteType}
	for _, g := range r.Groups {
		for _, c := range g.Conditions {
			switch c.Type {
			case domain.ConditionCountry:
				tags = append(tags, c.Value)
			case domain.ConditionOperator:
				tags = append(tags, c.Value)
			case domain.ConditionTrafficType:
				tags = append(tags, c.Value)
			case domain.ConditionRegex:
				tags = append(tags, "Regex: "+c.Value)
			case domain.ConditionPaidName:
				if c.Value == "true" {
					tags = append(tags, "Paid name")
				}
			}
		}
	}
	item.ConditionTags = tags

	// Build schedule summary
	if len(r.Schedules) > 0 {
		s := r.Schedules[0]
		item.ScheduleSummary = formatScheduleSummary(s)
	}

	return item
}

func formatScheduleSummary(s domain.Schedule) string {
	days := ""
	weekdayNames := []string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"}
	first := -1
	last := -1
	for i := 0; i < 7; i++ {
		if s.Weekdays&(1<<i) != 0 {
			if first == -1 {
				first = i
			}
			last = i
		}
	}
	if s.Weekdays == 127 {
		days = "Пн-Вс"
	} else if s.Weekdays == 31 {
		days = "Пн-Пт"
	} else if first >= 0 && last >= 0 {
		days = weekdayNames[first] + "-" + weekdayNames[last]
	}
	result := days
	if s.TimeFrom != nil && s.TimeTo != nil {
		result += ", " + *s.TimeFrom + "-" + *s.TimeTo
	}
	return result
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /c/projects/sms && go build ./internal/gateway/portal/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/routes.go
git commit -m "feat(routing): add portal HTTP handlers for route CRUD with conditions and schedules"
```

---

## Task 6: Register New Routes in Portal Router

**Files:**
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Add RouteRepo and RouteHandlers initialization**

In the router setup function, add after existing handler initialization:

```go
routeRepo := infrastructure.NewRouteRepo(pool)
routeHandlers := handlers.NewRouteHandlers(routeRepo)
```

Add the import:
```go
"github.com/smpp-server/smpp-server/internal/services/routing/infrastructure"
```

- [ ] **Step 2: Register new route endpoints**

Add after the existing routing subrouter block (after line ~173):

```go
// Route management (admin-managed default & client routes)
routes := protected.PathPrefix("/routes").Subrouter()
routes.HandleFunc("", routeHandlers.CreateRoute).Methods("POST")
routes.HandleFunc("", routeHandlers.ListRoutes).Methods("GET")
routes.HandleFunc("/references", routeHandlers.GetReferences).Methods("GET")
routes.HandleFunc("/{id}", routeHandlers.GetRoute).Methods("GET")
routes.HandleFunc("/{id}", routeHandlers.UpdateRoute).Methods("PUT")
routes.HandleFunc("/{id}", routeHandlers.DeleteRoute).Methods("DELETE")
```

- [ ] **Step 3: Verify it compiles**

```bash
cd /c/projects/sms && go build ./cmd/portal-gateway/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/router/router.go
git commit -m "feat(routing): register /portal/v1/routes endpoints in portal router"
```

---

## Task 7: RouteMatcher — In-Memory Matching Engine

**Files:**
- Create: `internal/services/routing/application/matcher.go`

- [ ] **Step 1: Create matcher.go**

```go
// internal/services/routing/application/matcher.go
package application

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/infrastructure"
)

type MatchContext struct {
	RouteType   string
	ClientID    uuid.UUID
	OperatorID  *uuid.UUID
	CountryCode string
	TrafficType domain.TrafficType
	PaidName    bool
	MessageBody string
	SenderName  string
}

type RouteMatcher struct {
	mu           sync.RWMutex
	routes       []*domain.ClientRoute
	regexCache   map[string]*regexp.Regexp
	repo         *infrastructure.RouteRepo
}

func NewRouteMatcher(repo *infrastructure.RouteRepo) *RouteMatcher {
	return &RouteMatcher{
		repo:       repo,
		regexCache: make(map[string]*regexp.Regexp),
	}
}

// Load fetches all active routes from the database and compiles regex patterns.
func (m *RouteMatcher) Load(ctx context.Context) error {
	routes, err := m.repo.LoadAllActive(ctx)
	if err != nil {
		return err
	}

	regexCache := make(map[string]*regexp.Regexp)
	for _, route := range routes {
		for _, g := range route.Groups {
			for _, c := range g.Conditions {
				if c.Type == domain.ConditionRegex {
					pattern := c.Value
					// Strip surrounding slashes and flags like /(pattern)/ui
					pattern = stripRegexDelimiters(pattern)
					re, err := regexp.Compile("(?i)" + pattern)
					if err != nil {
						log.Warn().Str("pattern", c.Value).Err(err).Msg("invalid regex in route condition")
						continue
					}
					regexCache[c.Value] = re
				}
			}
		}
	}

	m.mu.Lock()
	m.routes = routes
	m.regexCache = regexCache
	m.mu.Unlock()

	log.Info().Int("count", len(routes)).Msg("RouteMatcher loaded routes")
	return nil
}

// Invalidate reloads all routes from the database.
func (m *RouteMatcher) Invalidate(ctx context.Context) {
	if err := m.Load(ctx); err != nil {
		log.Error().Err(err).Msg("RouteMatcher invalidation failed")
	}
}

// Match finds matching routes for the given context, sorted by priority (lowest = highest).
func (m *RouteMatcher) Match(ctx MatchContext) []*domain.ClientRoute {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Step 1: Filter by route_type
	var candidates []*domain.ClientRoute
	for _, r := range m.routes {
		if r.RouteType != ctx.RouteType {
			continue
		}
		candidates = append(candidates, r)
	}

	// Step 2: Try client-specific routes first
	var clientRoutes []*domain.ClientRoute
	for _, r := range candidates {
		if r.ClientID != nil && *r.ClientID == ctx.ClientID {
			clientRoutes = append(clientRoutes, r)
		}
	}

	matched := m.filterMatching(clientRoutes, ctx)
	if len(matched) > 0 {
		return matched
	}

	// Step 3: Fallback to default routes (client_id IS NULL)
	var defaultRoutes []*domain.ClientRoute
	for _, r := range candidates {
		if r.ClientID == nil {
			defaultRoutes = append(defaultRoutes, r)
		}
	}

	return m.filterMatching(defaultRoutes, ctx)
}

func (m *RouteMatcher) filterMatching(routes []*domain.ClientRoute, ctx MatchContext) []*domain.ClientRoute {
	var result []*domain.ClientRoute
	for _, r := range routes {
		if m.evaluateConditions(r.Groups, ctx) && m.checkSchedules(r.Schedules) {
			result = append(result, r)
		}
	}
	// Already sorted by priority ASC from DB query
	return result
}

// evaluateConditions evaluates condition groups with logical operators.
// Groups are evaluated in order: IF → AND/AND_NOT/OR/OR_NOT
func (m *RouteMatcher) evaluateConditions(groups []domain.ConditionGroup, ctx MatchContext) bool {
	if len(groups) == 0 {
		return true // No conditions = match all
	}

	result := false
	for _, g := range groups {
		groupMatch := m.evaluateGroup(g, ctx)

		switch g.LogicOp {
		case domain.LogicIf:
			result = groupMatch
		case domain.LogicAnd:
			result = result && groupMatch
		case domain.LogicAndNot:
			result = result && !groupMatch
		case domain.LogicOr:
			result = result || groupMatch
		case domain.LogicOrNot:
			result = result || !groupMatch
		}
	}
	return result
}

// evaluateGroup evaluates a single condition group.
// Multiple conditions of the same type within a group are OR-joined.
// Different types are AND-joined.
func (m *RouteMatcher) evaluateGroup(g domain.ConditionGroup, ctx MatchContext) bool {
	// Group by condition type
	byType := make(map[domain.ConditionType][]domain.Condition)
	for _, c := range g.Conditions {
		byType[c.Type] = append(byType[c.Type], c)
	}

	for condType, conds := range byType {
		anyMatch := false
		for _, c := range conds {
			if m.evaluateCondition(c, ctx) {
				anyMatch = true
				break
			}
		}
		if !anyMatch {
			// If no condition of this type matches, the group fails (AND between types)
			_ = condType
			return false
		}
	}
	return true
}

func (m *RouteMatcher) evaluateCondition(c domain.Condition, ctx MatchContext) bool {
	switch c.Type {
	case domain.ConditionOperator:
		if ctx.OperatorID == nil {
			return false
		}
		// Match by UUID or by operator name/code
		return c.Value == ctx.OperatorID.String()
	case domain.ConditionCountry:
		return strings.EqualFold(c.Value, ctx.CountryCode)
	case domain.ConditionTrafficType:
		return strings.EqualFold(c.Value, string(ctx.TrafficType))
	case domain.ConditionPaidName:
		return (c.Value == "true") == ctx.PaidName
	case domain.ConditionRegex:
		re, ok := m.regexCache[c.Value]
		if !ok {
			return false
		}
		return re.MatchString(ctx.MessageBody) || re.MatchString(ctx.SenderName)
	}
	return false
}

func (m *RouteMatcher) checkSchedules(schedules []domain.Schedule) bool {
	if len(schedules) == 0 {
		return true // No schedule = always active
	}

	for _, s := range schedules {
		if m.matchSchedule(s) {
			return true // Any schedule matching is sufficient
		}
	}
	return false
}

func (m *RouteMatcher) matchSchedule(s domain.Schedule) bool {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)

	// Check date range
	if s.DateFrom != nil && now.Before(*s.DateFrom) {
		return false
	}
	if s.DateTo != nil {
		endOfDay := s.DateTo.Add(24*time.Hour - time.Nanosecond)
		if now.After(endOfDay) {
			return false
		}
	}

	// Check weekday (1=Mon, 2=Tue, 4=Wed, 8=Thu, 16=Fri, 32=Sat, 64=Sun)
	// time.Weekday: 0=Sun, 1=Mon ... 6=Sat
	wd := now.Weekday()
	var bit int
	if wd == time.Sunday {
		bit = 64
	} else {
		bit = 1 << (wd - 1)
	}
	if s.Weekdays&bit == 0 {
		return false
	}

	// Check time range
	if s.TimeFrom != nil && s.TimeTo != nil {
		nowTime := now.Format("15:04")
		if nowTime < *s.TimeFrom || nowTime > *s.TimeTo {
			return false
		}
	}

	return true
}

func stripRegexDelimiters(pattern string) string {
	if len(pattern) < 2 {
		return pattern
	}
	if pattern[0] == '/' {
		// Find last /
		lastSlash := strings.LastIndex(pattern[1:], "/")
		if lastSlash >= 0 {
			return pattern[1 : lastSlash+1]
		}
	}
	return pattern
}
```

Note: add `"context"` to the imports.

- [ ] **Step 2: Verify it compiles**

```bash
cd /c/projects/sms && go build ./internal/services/routing/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/routing/application/matcher.go
git commit -m "feat(routing): add in-memory RouteMatcher with condition evaluation and schedule checking"
```

---

## Task 8: Template Traffic Type Extension

**Files:**
- Modify: `internal/services/template/domain/models.go` — add `TrafficType` field
- Modify: `api/proto/template/template.proto` — add `traffic_type` to TemplateInfo and CreateTemplateRequest
- Modify: `internal/gateway/portal/handlers/templates.go` — pass traffic_type through

- [ ] **Step 1: Add TrafficType to template domain model**

In `internal/services/template/domain/models.go`, add to the Template struct:

```go
TrafficType string  // authorization, transactional, service
```

- [ ] **Step 2: Add traffic_type to template proto**

In `api/proto/template/template.proto`, add to `TemplateInfo` message:

```protobuf
string traffic_type = 15;
```

Add to `CreateTemplateRequest`:

```protobuf
string traffic_type = 5;
```

Add to `UpdateTemplateRequest`:

```protobuf
optional string traffic_type = 5;
```

- [ ] **Step 3: Regenerate proto**

```bash
cd /c/projects/sms && make proto
```

Or if no make target, run protoc manually for template proto.

- [ ] **Step 4: Update portal template handlers**

In `internal/gateway/portal/handlers/templates.go`:

Update `createTemplateRequest`:
```go
type createTemplateRequest struct {
	Name         string  `json:"name"`
	Body         string  `json:"body"`
	SenderNameID *string `json:"sender_name_id,omitempty"`
	TrafficType  string  `json:"traffic_type,omitempty"`
}
```

In `CreateTemplate`, pass it through:
```go
resp, err := h.templateClient.CreateTemplate(r.Context(), &templatev1.CreateTemplateRequest{
	ClientId:     clientID.String(),
	Name:         req.Name,
	Body:         req.Body,
	SenderNameId: req.SenderNameID,
	TrafficType:  req.TrafficType,
})
```

Update `templateToJSON` to include `traffic_type`:
```go
if t.TrafficType != "" {
	m["traffic_type"] = t.TrafficType
} else {
	m["traffic_type"] = "transactional"
}
```

Also update `updateTemplateRequest` and `UpdateTemplate` handler similarly.

- [ ] **Step 5: Update template service gRPC server to persist traffic_type**

In the template service's gRPC server and repository, ensure `traffic_type` is read/written to the database.

- [ ] **Step 6: Verify it compiles**

```bash
cd /c/projects/sms && go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add api/proto/template/ internal/services/template/ internal/gateway/portal/handlers/templates.go
git commit -m "feat(templates): add traffic_type field (authorization/transactional/service)"
```

---

## Task 9: Frontend — Types and API Client

**Files:**
- Create: `portal-frontend/src/pages/routing/types.ts`
- Modify: `portal-frontend/src/api/client.ts` — add new routing API

- [ ] **Step 1: Create types.ts**

```typescript
// portal-frontend/src/pages/routing/types.ts

export interface ConditionJSON {
  type: 'operator' | 'country' | 'traffic_type' | 'paid_name' | 'regex';
  value: string;
}

export interface ConditionGroupJSON {
  logic_op: 'IF' | 'AND' | 'AND_NOT' | 'OR' | 'OR_NOT';
  conditions: ConditionJSON[];
}

export interface ScheduleJSON {
  date_from?: string;
  date_to?: string;
  time_from?: string;
  time_to?: string;
  weekdays: number;
  timezone: string;
}

export interface RouteFormData {
  client_id?: string | null;
  name: string;
  comment: string;
  status: 'active' | 'draft';
  route_type: 'sms' | 'hlr' | 'max';
  operator_id?: string;
  provider_id: string;
  priority: number;
  share: number;
  condition_groups: ConditionGroupJSON[];
  schedules: ScheduleJSON[];
}

export interface RouteDetail {
  id: string;
  client_id: string | null;
  name: string;
  comment: string;
  status: string;
  route_type: string;
  operator_id?: string;
  provider_id: string;
  priority: number;
  share: number;
  condition_groups: ConditionGroupJSON[];
  schedules: ScheduleJSON[];
  created_at: string;
  updated_at: string;
}

export interface RouteListItem {
  id: string;
  client_id: string | null;
  name: string;
  status: string;
  route_type: string;
  provider_id: string;
  priority: number;
  share: number;
  comment: string;
  condition_tags: string[];
  schedule_summary: string;
  created_at: string;
  updated_at: string;
}

export interface RoutesListResponse {
  routes: RouteListItem[];
  total: number;
}

export interface RouteReferences {
  traffic_types: string[];
  route_types: string[];
  statuses: string[];
  logic_ops: string[];
  condition_types: string[];
}

export const WEEKDAY_LABELS = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'] as const;
export const WEEKDAY_BITS = [1, 2, 4, 8, 16, 32, 64] as const;

export const LOGIC_OP_LABELS: Record<string, string> = {
  IF: 'ЕСЛИ',
  AND: 'И',
  AND_NOT: 'И НЕ',
  OR: 'ИЛИ',
  OR_NOT: 'ИЛИ НЕ',
};

export const CONDITION_TYPE_LABELS: Record<string, string> = {
  operator: 'Оператор',
  country: 'Страна',
  traffic_type: 'Тип трафика',
  paid_name: 'Платное имя',
  regex: 'Regex',
};
```

- [ ] **Step 2: Add routes API to client.ts**

In `portal-frontend/src/api/client.ts`, add after the existing `routingApi`:

```typescript
// Route management API (admin default routes)
export const routesApi = {
  list: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ routes: RouteListItem[]; total: number }>(`/routes?${qs}`);
  },
  get: (id: string) => apiFetch<RouteDetail>(`/routes/${id}`),
  create: (data: RouteFormData) =>
    apiFetch<RouteDetail>('/routes', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: RouteFormData) =>
    apiFetch<RouteDetail>(`/routes/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  remove: (id: string) => apiFetch<void>(`/routes/${id}`, { method: 'DELETE' }),
  references: () => apiFetch<RouteReferences>('/routes/references'),
};
```

Add the necessary imports at the top of client.ts:

```typescript
import type { RouteListItem, RouteDetail, RouteFormData, RouteReferences } from '../pages/routing/types';
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/routing/types.ts portal-frontend/src/api/client.ts
git commit -m "feat(frontend): add TypeScript types and API client for route management"
```

---

## Task 10: Frontend — RoutingPage Replacement

**Files:**
- Rewrite: `portal-frontend/src/pages/routing/RoutingPage.tsx`
- Create: `portal-frontend/src/pages/routing/components/RouteTable.tsx`
- Create: `portal-frontend/src/pages/routing/components/RouteFilters.tsx`

- [ ] **Step 1: Create RouteFilters.tsx**

```tsx
// portal-frontend/src/pages/routing/components/RouteFilters.tsx
import { useState } from 'react';

interface Props {
  onSearch: (query: string) => void;
  onFilterChange: (filters: { route_type?: string; status?: string }) => void;
}

export function RouteFilters({ onSearch, onFilterChange }: Props) {
  const [search, setSearch] = useState('');
  const [activeFilters, setActiveFilters] = useState<Record<string, string>>({});

  function toggleFilter(key: string, value: string) {
    const next = { ...activeFilters };
    if (next[key] === value) {
      delete next[key];
    } else {
      next[key] = value;
    }
    setActiveFilters(next);
    onFilterChange(next);
  }

  return (
    <div className="flex items-center justify-between gap-3 flex-wrap px-5 py-4 border-b border-gray-200">
      <div className="flex items-center gap-3 flex-wrap">
        <input
          type="text"
          placeholder="Поиск по оператору, стране, sender, regex"
          value={search}
          onChange={(e) => { setSearch(e.target.value); onSearch(e.target.value); }}
          className="min-w-[260px] border border-gray-300 rounded-lg px-3 py-2.5 text-sm"
        />
        {(['sms', 'hlr', 'max'] as const).map((t) => (
          <button
            key={t}
            onClick={() => toggleFilter('route_type', t)}
            className={`px-3 py-2 rounded-full text-xs font-semibold border ${
              activeFilters.route_type === t
                ? 'bg-blue-50 text-blue-600 border-blue-200'
                : 'bg-white text-gray-600 border-gray-300'
            }`}
          >
            {t.toUpperCase()}
          </button>
        ))}
        {(['active', 'draft'] as const).map((s) => (
          <button
            key={s}
            onClick={() => toggleFilter('status', s)}
            className={`px-3 py-2 rounded-full text-xs font-semibold border ${
              activeFilters.status === s
                ? 'bg-blue-50 text-blue-600 border-blue-200'
                : 'bg-white text-gray-600 border-gray-300'
            }`}
          >
            {s === 'active' ? 'Активные' : 'Черновики'}
          </button>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Create RouteTable.tsx**

```tsx
// portal-frontend/src/pages/routing/components/RouteTable.tsx
import type { RouteListItem } from '../types';

interface Props {
  routes: RouteListItem[];
  loading: boolean;
  onEdit: (id: string) => void;
  onDelete: (id: string) => void;
}

export function RouteTable({ routes, loading, onEdit, onDelete }: Props) {
  if (loading) {
    return <div className="p-6 text-gray-500">Загрузка...</div>;
  }

  if (routes.length === 0) {
    return <div className="p-6 text-gray-500">Нет маршрутов</div>;
  }

  return (
    <div className="overflow-auto">
      <table className="w-full min-w-[1080px] border-collapse">
        <thead>
          <tr>
            <th className="text-left px-4 py-3 text-sm font-semibold text-gray-600 border-b">Условия</th>
            <th className="text-left px-4 py-3 text-sm font-semibold text-gray-600 border-b">Провайдер</th>
            <th className="text-left px-4 py-3 text-sm font-semibold text-gray-600 border-b">Приоритет</th>
            <th className="text-left px-4 py-3 text-sm font-semibold text-gray-600 border-b">Доля</th>
            <th className="text-left px-4 py-3 text-sm font-semibold text-gray-600 border-b">Статус</th>
            <th className="px-4 py-3 border-b"></th>
          </tr>
        </thead>
        <tbody>
          {routes.map((route) => (
            <tr key={route.id} className="hover:bg-gray-50">
              <td className="px-4 py-4 border-b align-top">
                <div className="flex flex-wrap gap-1.5 max-w-[460px]">
                  {route.condition_tags.map((tag, i) => (
                    <span key={i} className="inline-flex px-2.5 py-1 rounded-full bg-blue-50 text-blue-600 text-xs font-medium">
                      {tag}
                    </span>
                  ))}
                </div>
                {route.schedule_summary && (
                  <div className="text-xs text-gray-500 mt-1.5">{route.schedule_summary}</div>
                )}
              </td>
              <td className="px-4 py-4 border-b align-top text-sm">{route.provider_id}</td>
              <td className="px-4 py-4 border-b align-top text-base font-bold">{route.priority}</td>
              <td className="px-4 py-4 border-b align-top text-sm font-semibold">{route.share}%</td>
              <td className="px-4 py-4 border-b align-top">
                <span className={`inline-flex px-3 py-1.5 rounded-full text-sm font-semibold ${
                  route.status === 'active'
                    ? 'bg-green-50 text-green-700'
                    : 'bg-gray-100 text-gray-500'
                }`}>
                  {route.status === 'active' ? 'Активен' : 'Черновик'}
                </span>
              </td>
              <td className="px-4 py-4 border-b align-top">
                <div className="flex gap-2">
                  <button
                    onClick={() => onEdit(route.id)}
                    className="w-9 h-9 rounded-lg border border-gray-300 bg-white text-gray-500 hover:bg-gray-50 flex items-center justify-center text-sm"
                  >
                    &#9998;
                  </button>
                  <button
                    onClick={() => onDelete(route.id)}
                    className="w-9 h-9 rounded-lg border border-gray-300 bg-white text-red-500 hover:bg-red-50 flex items-center justify-center text-sm"
                  >
                    &times;
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 3: Rewrite RoutingPage.tsx**

```tsx
// portal-frontend/src/pages/routing/RoutingPage.tsx
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { routesApi, ApiError } from '../../api/client';
import { RouteTable } from './components/RouteTable';
import { RouteFilters } from './components/RouteFilters';
import { RouteModal } from './RouteModal';
import type { RouteListItem } from './types';

export function RoutingPage() {
  const [routes, setRoutes] = useState<RouteListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [modalOpen, setModalOpen] = useState(false);
  const [editId, setEditId] = useState<string | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const params: Record<string, string> = { default: 'true', ...filters };
      const resp = await routesApi.list(params);
      setRoutes(resp.routes ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить маршруты');
    } finally {
      setLoading(false);
    }
  }, [filters]);

  useEffect(() => { load(); }, [load]);

  const filteredRoutes = search
    ? routes.filter((r) =>
        r.condition_tags.some((t) => t.toLowerCase().includes(search.toLowerCase())) ||
        r.name?.toLowerCase().includes(search.toLowerCase())
      )
    : routes;

  function handleEdit(id: string) {
    setEditId(id);
    setModalOpen(true);
  }

  function handleAdd() {
    setEditId(null);
    setModalOpen(true);
  }

  async function handleDelete() {
    if (!deleteId) return;
    setDeleting(true);
    try {
      await routesApi.remove(deleteId);
      setRoutes((prev) => prev.filter((r) => r.id !== deleteId));
      setDeleteId(null);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Не удалось удалить маршрут');
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div>
      <PageHeader
        title="Общие маршруты"
        subtitle="Дефолтные маршруты, применяемые когда у клиента нет индивидуального маршрута"
      />
      <div className="flex items-center justify-end gap-3 mb-4">
        <Button onClick={handleAdd}>+ Добавить маршрут</Button>
      </div>

      {error && <p className="text-red-600 mb-4">{error}</p>}

      <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
        <RouteFilters onSearch={setSearch} onFilterChange={setFilters} />
        <RouteTable
          routes={filteredRoutes}
          loading={loading}
          onEdit={handleEdit}
          onDelete={setDeleteId}
        />
        <div className="px-5 py-3 text-sm text-gray-500 border-t">
          Всего: {filteredRoutes.length} маршрутов
        </div>
      </div>

      {modalOpen && (
        <RouteModal
          routeId={editId}
          onClose={() => { setModalOpen(false); setEditId(null); }}
          onSaved={load}
        />
      )}

      <ConfirmDialog
        open={deleteId !== null}
        onConfirm={handleDelete}
        onCancel={() => setDeleteId(null)}
        title="Удалить маршрут"
        description="Вы уверены, что хотите удалить этот маршрут? Это действие нельзя отменить."
        confirmLabel="Удалить"
        variant="danger"
        loading={deleting}
      />
    </div>
  );
}
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/routing/
git commit -m "feat(frontend): replace RoutingPage with new route table, filters, and layout"
```

---

## Task 11: Frontend — RouteModal with Condition & Schedule Editors

**Files:**
- Create: `portal-frontend/src/pages/routing/RouteModal.tsx`
- Create: `portal-frontend/src/pages/routing/components/ConditionEditor.tsx`
- Create: `portal-frontend/src/pages/routing/components/ScheduleEditor.tsx`
- Create: `portal-frontend/src/pages/routing/components/RouteSummary.tsx`

- [ ] **Step 1: Create ConditionEditor.tsx**

```tsx
// portal-frontend/src/pages/routing/components/ConditionEditor.tsx
import type { ConditionGroupJSON, ConditionJSON } from '../types';
import { LOGIC_OP_LABELS, CONDITION_TYPE_LABELS } from '../types';

interface Props {
  groups: ConditionGroupJSON[];
  onChange: (groups: ConditionGroupJSON[]) => void;
}

export function ConditionEditor({ groups, onChange }: Props) {
  function addGroup() {
    const logicOp = groups.length === 0 ? 'IF' : 'AND';
    onChange([...groups, { logic_op: logicOp, conditions: [{ type: 'operator', value: '' }] }]);
  }

  function removeGroup(idx: number) {
    onChange(groups.filter((_, i) => i !== idx));
  }

  function updateGroup(idx: number, group: ConditionGroupJSON) {
    const next = [...groups];
    next[idx] = group;
    onChange(next);
  }

  function updateLogicOp(idx: number, op: string) {
    const next = [...groups];
    next[idx] = { ...next[idx], logic_op: op as ConditionGroupJSON['logic_op'] };
    onChange(next);
  }

  function addCondition(groupIdx: number) {
    const next = [...groups];
    next[groupIdx] = {
      ...next[groupIdx],
      conditions: [...next[groupIdx].conditions, { type: 'operator', value: '' }],
    };
    onChange(next);
  }

  function removeCondition(groupIdx: number, condIdx: number) {
    const next = [...groups];
    next[groupIdx] = {
      ...next[groupIdx],
      conditions: next[groupIdx].conditions.filter((_, i) => i !== condIdx),
    };
    onChange(next);
  }

  function updateCondition(groupIdx: number, condIdx: number, cond: ConditionJSON) {
    const next = [...groups];
    const conditions = [...next[groupIdx].conditions];
    conditions[condIdx] = cond;
    next[groupIdx] = { ...next[groupIdx], conditions };
    onChange(next);
  }

  return (
    <div className="space-y-3">
      {groups.map((group, gi) => (
        <div key={gi} className="border border-gray-200 rounded-lg bg-gray-50 p-3">
          <div className="flex items-center gap-2 mb-2">
            {gi === 0 ? (
              <span className="text-xs font-bold text-gray-500 uppercase tracking-wider">ЕСЛИ</span>
            ) : (
              <select
                value={group.logic_op}
                onChange={(e) => updateLogicOp(gi, e.target.value)}
                className="border border-gray-300 rounded px-2 py-1 text-xs font-bold uppercase"
              >
                {Object.entries(LOGIC_OP_LABELS).filter(([k]) => k !== 'IF').map(([k, v]) => (
                  <option key={k} value={k}>{v}</option>
                ))}
              </select>
            )}
            <div className="flex-1" />
            <button onClick={() => removeGroup(gi)} className="text-red-500 text-sm hover:underline">
              Удалить группу
            </button>
          </div>

          {group.conditions.map((cond, ci) => (
            <div key={ci} className="grid grid-cols-[160px_1fr_36px] gap-2 mb-2 items-center">
              <select
                value={cond.type}
                onChange={(e) => updateCondition(gi, ci, { ...cond, type: e.target.value as ConditionJSON['type'] })}
                className="border border-gray-300 rounded px-2 py-2 text-sm"
              >
                {Object.entries(CONDITION_TYPE_LABELS).map(([k, v]) => (
                  <option key={k} value={k}>{v}</option>
                ))}
              </select>
              <input
                type="text"
                value={cond.value}
                onChange={(e) => updateCondition(gi, ci, { ...cond, value: e.target.value })}
                placeholder={cond.type === 'regex' ? '/(pattern)/i' : 'Значение'}
                className="border border-gray-300 rounded px-2 py-2 text-sm"
              />
              <button
                onClick={() => removeCondition(gi, ci)}
                className="w-9 h-9 rounded border border-gray-300 bg-white text-red-500 flex items-center justify-center"
              >
                &times;
              </button>
            </div>
          ))}

          <button
            onClick={() => addCondition(gi)}
            className="text-blue-600 text-sm hover:underline"
          >
            + Условие
          </button>
        </div>
      ))}

      <button
        onClick={addGroup}
        className="w-full py-2 border border-dashed border-gray-300 rounded-lg text-sm text-gray-500 hover:bg-gray-50"
      >
        + Добавить группу условий
      </button>
    </div>
  );
}
```

- [ ] **Step 2: Create ScheduleEditor.tsx**

```tsx
// portal-frontend/src/pages/routing/components/ScheduleEditor.tsx
import type { ScheduleJSON } from '../types';
import { WEEKDAY_LABELS, WEEKDAY_BITS } from '../types';

interface Props {
  schedules: ScheduleJSON[];
  onChange: (schedules: ScheduleJSON[]) => void;
}

const DEFAULT_SCHEDULE: ScheduleJSON = {
  weekdays: 127,
  timezone: 'Europe/Moscow',
};

export function ScheduleEditor({ schedules, onChange }: Props) {
  const enabled = schedules.length > 0;

  function toggle() {
    if (enabled) {
      onChange([]);
    } else {
      onChange([{ ...DEFAULT_SCHEDULE }]);
    }
  }

  function update(idx: number, patch: Partial<ScheduleJSON>) {
    const next = [...schedules];
    next[idx] = { ...next[idx], ...patch };
    onChange(next);
  }

  function toggleWeekday(idx: number, bit: number) {
    const current = schedules[idx].weekdays;
    update(idx, { weekdays: current ^ bit });
  }

  return (
    <div>
      <div className="flex items-center justify-between p-3 border border-gray-200 rounded-lg bg-gray-50 mb-3">
        <div>
          <div className="font-semibold text-sm">Расписание</div>
          <div className="text-xs text-gray-500 mt-1">Ограничить маршрут по дням и времени</div>
        </div>
        <button
          onClick={toggle}
          className={`w-12 h-7 rounded-full relative transition-colors ${enabled ? 'bg-blue-600' : 'bg-gray-300'}`}
        >
          <span className={`absolute top-0.5 w-6 h-6 rounded-full bg-white shadow transition-transform ${enabled ? 'left-[22px]' : 'left-0.5'}`} />
        </button>
      </div>

      {enabled && schedules.map((sched, si) => (
        <div key={si} className="space-y-3 border border-gray-200 rounded-lg p-3 bg-white">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <label className="text-xs font-semibold text-gray-500">Дата начала</label>
              <input
                type="date"
                value={sched.date_from ?? ''}
                onChange={(e) => update(si, { date_from: e.target.value || undefined })}
                className="w-full border border-gray-300 rounded px-2 py-2 text-sm"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-gray-500">Дата окончания</label>
              <input
                type="date"
                value={sched.date_to ?? ''}
                onChange={(e) => update(si, { date_to: e.target.value || undefined })}
                className="w-full border border-gray-300 rounded px-2 py-2 text-sm"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <label className="text-xs font-semibold text-gray-500">Время с</label>
              <input
                type="time"
                value={sched.time_from ?? ''}
                onChange={(e) => update(si, { time_from: e.target.value || undefined })}
                className="w-full border border-gray-300 rounded px-2 py-2 text-sm"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-gray-500">Время до</label>
              <input
                type="time"
                value={sched.time_to ?? ''}
                onChange={(e) => update(si, { time_to: e.target.value || undefined })}
                className="w-full border border-gray-300 rounded px-2 py-2 text-sm"
              />
            </div>
          </div>

          <div className="space-y-1">
            <label className="text-xs font-semibold text-gray-500">Дни недели</label>
            <div className="flex gap-2">
              {WEEKDAY_LABELS.map((label, i) => (
                <button
                  key={i}
                  onClick={() => toggleWeekday(si, WEEKDAY_BITS[i])}
                  className={`px-3 py-1.5 rounded-full text-xs font-semibold border ${
                    sched.weekdays & WEEKDAY_BITS[i]
                      ? 'bg-blue-50 text-blue-600 border-blue-200'
                      : 'bg-white text-gray-500 border-gray-300'
                  }`}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>

          <div className="space-y-1">
            <label className="text-xs font-semibold text-gray-500">Часовой пояс</label>
            <select
              value={sched.timezone}
              onChange={(e) => update(si, { timezone: e.target.value })}
              className="w-full border border-gray-300 rounded px-2 py-2 text-sm"
            >
              <option value="Europe/Moscow">Europe/Moscow (MSK)</option>
              <option value="Europe/Kiev">Europe/Kiev (EET)</option>
              <option value="Asia/Almaty">Asia/Almaty (+6)</option>
              <option value="UTC">UTC</option>
            </select>
          </div>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 3: Create RouteSummary.tsx**

```tsx
// portal-frontend/src/pages/routing/components/RouteSummary.tsx
import type { RouteFormData } from '../types';
import { LOGIC_OP_LABELS, CONDITION_TYPE_LABELS, WEEKDAY_LABELS, WEEKDAY_BITS } from '../types';

interface Props {
  data: RouteFormData;
}

export function RouteSummary({ data }: Props) {
  const weekdayStr = data.schedules[0]
    ? WEEKDAY_LABELS.filter((_, i) => data.schedules[0].weekdays & WEEKDAY_BITS[i]).join(', ')
    : 'Все дни';

  return (
    <div className="sticky top-4 border border-gray-200 rounded-xl bg-white p-4 shadow-sm space-y-3">
      <h2 className="text-lg font-semibold">Предпросмотр</h2>
      <p className="text-sm text-gray-500">Маршрут обновляется автоматически</p>

      <div className="space-y-2">
        <SummaryItem label="Тип" value={data.route_type.toUpperCase()} />
        <SummaryItem label="Приоритет" value={String(data.priority)} />
        <SummaryItem label="Доля" value={`${data.share}%`} />
        <SummaryItem label="Статус" value={data.status === 'active' ? 'Активен' : 'Черновик'} />
      </div>

      {data.condition_groups.length > 0 && (
        <div className="pt-2 border-t">
          <div className="text-xs font-semibold text-gray-500 mb-2">Условия</div>
          {data.condition_groups.map((g, i) => (
            <div key={i} className="text-sm mb-1">
              <span className="font-bold text-gray-400 text-xs uppercase mr-1">
                {LOGIC_OP_LABELS[g.logic_op]}
              </span>
              {g.conditions.map((c, j) => (
                <span key={j} className="inline-flex px-2 py-0.5 rounded-full bg-blue-50 text-blue-600 text-xs mr-1">
                  {CONDITION_TYPE_LABELS[c.type]}: {c.value}
                </span>
              ))}
            </div>
          ))}
        </div>
      )}

      {data.schedules.length > 0 && (
        <div className="pt-2 border-t">
          <div className="text-xs font-semibold text-gray-500 mb-1">Расписание</div>
          <div className="text-sm">{weekdayStr}</div>
          {data.schedules[0]?.time_from && data.schedules[0]?.time_to && (
            <div className="text-sm text-gray-500">
              {data.schedules[0].time_from}–{data.schedules[0].time_to}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function SummaryItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between items-center px-3 py-2 border border-gray-200 rounded-lg bg-gray-50">
      <span className="text-sm font-medium">{label}</span>
      <span className="text-sm text-gray-500">{value}</span>
    </div>
  );
}
```

- [ ] **Step 4: Create RouteModal.tsx**

```tsx
// portal-frontend/src/pages/routing/RouteModal.tsx
import { useState, useEffect } from 'react';
import { Modal } from '../../components/ui/Modal';
import { Button } from '../../components/ui/Button';
import { Select } from '../../components/ui/Select';
import { routesApi, providersApi, routingApi, ApiError, type Provider, type OperatorInfo } from '../../api/client';
import { ConditionEditor } from './components/ConditionEditor';
import { ScheduleEditor } from './components/ScheduleEditor';
import { RouteSummary } from './components/RouteSummary';
import type { RouteFormData, ConditionGroupJSON, ScheduleJSON } from './types';

interface Props {
  routeId: string | null;
  onClose: () => void;
  onSaved: () => void;
}

const EMPTY_FORM: RouteFormData = {
  name: '',
  comment: '',
  status: 'active',
  route_type: 'sms',
  provider_id: '',
  priority: 10,
  share: 100,
  condition_groups: [],
  schedules: [],
};

export function RouteModal({ routeId, onClose, onSaved }: Props) {
  const [form, setForm] = useState<RouteFormData>({ ...EMPTY_FORM });
  const [providers, setProviders] = useState<Provider[]>([]);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(!!routeId);
  const [error, setError] = useState('');

  useEffect(() => {
    Promise.all([
      providersApi.list(),
      routingApi.listOperators(),
    ]).then(([provResp, opsResp]) => {
      setProviders(provResp.providers ?? []);
      setOperators(opsResp.operators ?? []);
    });

    if (routeId) {
      routesApi.get(routeId).then((detail) => {
        setForm({
          client_id: detail.client_id,
          name: detail.name,
          comment: detail.comment,
          status: detail.status as 'active' | 'draft',
          route_type: detail.route_type as 'sms' | 'hlr' | 'max',
          operator_id: detail.operator_id,
          provider_id: detail.provider_id,
          priority: detail.priority,
          share: detail.share,
          condition_groups: detail.condition_groups,
          schedules: detail.schedules,
        });
        setLoading(false);
      }).catch(() => {
        setError('Не удалось загрузить маршрут');
        setLoading(false);
      });
    }
  }, [routeId]);

  function update(patch: Partial<RouteFormData>) {
    setForm((prev) => ({ ...prev, ...patch }));
  }

  async function handleSave(status: 'active' | 'draft') {
    setError('');
    if (!form.provider_id) {
      setError('Выберите провайдера');
      return;
    }
    setSaving(true);
    const payload = { ...form, status };
    try {
      if (routeId) {
        await routesApi.update(routeId, payload);
      } else {
        await routesApi.create(payload);
      }
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <Modal open onClose={onClose} title="Загрузка...">
        <div className="p-6 text-gray-500">Загрузка маршрута...</div>
      </Modal>
    );
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={routeId ? 'Редактирование маршрута' : 'Добавление маршрута'}
      description="Настройте условия, провайдера и расписание"
      size="lg"
    >
      <div className="grid grid-cols-[1fr_330px] gap-5 p-1">
        {/* Left: form sections */}
        <div className="space-y-5">
          {/* Section: Основное */}
          <section className="border border-gray-200 rounded-xl overflow-hidden">
            <div className="px-5 py-4 border-b border-gray-200 flex items-center justify-between">
              <h3 className="text-lg font-semibold">Основное</h3>
              <span className={`px-3 py-1 rounded-full text-xs font-semibold ${
                form.status === 'active' ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'
              }`}>
                {form.status === 'active' ? 'Активен' : 'Черновик'}
              </span>
            </div>
            <div className="p-5 space-y-4">
              <div className="space-y-1">
                <label className="text-sm font-semibold text-gray-600">Название</label>
                <input
                  type="text"
                  value={form.name}
                  onChange={(e) => update({ name: e.target.value })}
                  placeholder="Название маршрута"
                  className="w-full border border-gray-300 rounded-lg px-3 py-2.5 text-sm"
                />
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div className="space-y-1">
                  <label className="text-sm font-semibold text-gray-600">Тип</label>
                  <div className="flex gap-2">
                    {(['sms', 'hlr', 'max'] as const).map((t) => (
                      <button
                        key={t}
                        onClick={() => update({ route_type: t })}
                        className={`flex-1 py-2 rounded-full text-sm font-medium border ${
                          form.route_type === t
                            ? 'bg-blue-50 text-blue-600 border-blue-200'
                            : 'bg-white text-gray-500 border-gray-300'
                        }`}
                      >
                        {t.toUpperCase()}
                      </button>
                    ))}
                  </div>
                </div>
                <div className="space-y-1">
                  <label className="text-sm font-semibold text-gray-600">Приоритет</label>
                  <input
                    type="number"
                    value={form.priority}
                    onChange={(e) => update({ priority: parseInt(e.target.value) || 0 })}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2.5 text-sm"
                    min="0"
                  />
                  <span className="text-xs text-gray-400">Меньше = выше приоритет</span>
                </div>
                <div className="space-y-1">
                  <label className="text-sm font-semibold text-gray-600">Доля %</label>
                  <input
                    type="number"
                    value={form.share}
                    onChange={(e) => update({ share: parseInt(e.target.value) || 100 })}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2.5 text-sm"
                    min="1"
                    max="100"
                  />
                </div>
              </div>
              <div className="space-y-1">
                <label className="text-sm font-semibold text-gray-600">Комментарий</label>
                <textarea
                  value={form.comment}
                  onChange={(e) => update({ comment: e.target.value })}
                  placeholder="Описание маршрута"
                  className="w-full border border-gray-300 rounded-lg px-3 py-2.5 text-sm min-h-[80px] resize-y"
                />
              </div>
            </div>
          </section>

          {/* Section: Условия срабатывания */}
          <section className="border border-gray-200 rounded-xl overflow-hidden">
            <div className="px-5 py-4 border-b border-gray-200">
              <h3 className="text-lg font-semibold">Условия срабатывания</h3>
              <p className="text-sm text-gray-500 mt-1">Логические группы: IF, AND, AND NOT, OR, OR NOT</p>
            </div>
            <div className="p-5">
              <ConditionEditor
                groups={form.condition_groups}
                onChange={(groups: ConditionGroupJSON[]) => update({ condition_groups: groups })}
              />
            </div>
          </section>

          {/* Section: Канал доставки */}
          <section className="border border-gray-200 rounded-xl overflow-hidden">
            <div className="px-5 py-4 border-b border-gray-200">
              <h3 className="text-lg font-semibold">Канал доставки</h3>
            </div>
            <div className="p-5">
              <Select
                label="Провайдер"
                value={form.provider_id}
                onChange={(v: string) => update({ provider_id: v })}
                placeholder="Выберите провайдера"
                options={providers.filter((p) => p.active).map((p) => ({ value: p.id, label: p.name }))}
              />
            </div>
          </section>

          {/* Section: Расписание */}
          <section className="border border-gray-200 rounded-xl overflow-hidden">
            <div className="px-5 py-4 border-b border-gray-200">
              <h3 className="text-lg font-semibold">Расписание</h3>
            </div>
            <div className="p-5">
              <ScheduleEditor
                schedules={form.schedules}
                onChange={(schedules: ScheduleJSON[]) => update({ schedules })}
              />
            </div>
          </section>
        </div>

        {/* Right: Preview sidebar */}
        <RouteSummary data={form} />
      </div>

      {error && <p className="text-red-600 text-sm px-5 mt-2">{error}</p>}

      {/* Footer */}
      <div className="flex justify-between items-center mt-5 pt-4 border-t border-gray-200">
        <Button variant="ghost" onClick={onClose}>Отмена</Button>
        <div className="flex gap-2">
          <Button variant="ghost" onClick={() => handleSave('draft')} disabled={saving}>
            Сохранить как черновик
          </Button>
          <Button onClick={() => handleSave('active')} disabled={saving}>
            {saving ? 'Сохранение...' : 'Сохранить маршрут'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
```

- [ ] **Step 5: Verify frontend builds**

```bash
cd /c/projects/sms/portal-frontend && npm run build
```

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/pages/routing/
git commit -m "feat(frontend): add RouteModal with ConditionEditor, ScheduleEditor, and RouteSummary"
```

---

## Task 12: Frontend — Template Traffic Type Extension

**Files:**
- Modify: `portal-frontend/src/api/client.ts` — add traffic_type to TemplateInfo and create/update
- Modify: `portal-frontend/src/pages/templates/TemplatesPage.tsx` — add traffic_type select and tag display

- [ ] **Step 1: Update TemplateInfo interface**

In `portal-frontend/src/api/client.ts`, add `traffic_type` to `TemplateInfo`:

```typescript
export interface TemplateInfo {
  id: string;
  client_id: string;
  name: string;
  body: string;
  variables: string[];
  status: string;
  rejection_reason?: string;
  sender_name_id?: string;
  sender_name?: string;
  traffic_type?: string; // NEW
  created_at: string;
  updated_at: string;
}
```

Update `templatesApi.create` and `templatesApi.update` to accept `traffic_type`:

```typescript
create: (data: { name: string; body: string; sender_name_id?: string; traffic_type?: string }) =>
  apiFetch<TemplateInfo>('/templates', { method: 'POST', body: JSON.stringify(data) }),
update: (id: string, data: { name?: string; body?: string; sender_name_id?: string; traffic_type?: string }) =>
  apiFetch<TemplateInfo>(`/templates/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
```

- [ ] **Step 2: Update TemplatesPage**

Find the template create/edit form in `portal-frontend/src/pages/templates/TemplatesPage.tsx` and add:

1. A `<Select>` for traffic_type in the create/edit form:
```tsx
<Select
  label="Тип трафика"
  value={formTrafficType}
  onChange={setFormTrafficType}
  options={[
    { value: 'transactional', label: 'Transactional' },
    { value: 'authorization', label: 'Authorization' },
    { value: 'service', label: 'Service' },
  ]}
/>
```

2. Add a column in the templates table to display the traffic type as a tag.

- [ ] **Step 3: Verify build**

```bash
cd /c/projects/sms/portal-frontend && npm run build
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/api/client.ts portal-frontend/src/pages/templates/
git commit -m "feat(frontend): add traffic_type to template create/edit form and table"
```

---

## Task 13: Build, Deploy, and Verify

- [ ] **Step 1: Verify Go backend compiles**

```bash
cd /c/projects/sms && go build ./...
```

- [ ] **Step 2: Verify frontend builds**

```bash
cd /c/projects/sms/portal-frontend && npm run build
```

- [ ] **Step 3: Push and deploy**

```bash
git push origin master
scripts/server.sh deploy
scripts/server.sh migrate
```

- [ ] **Step 4: Verify API endpoints**

Test the new routes API on the server:

```bash
# List routes
curl -s https://your-portal/portal/v1/routes?default=true | jq .

# Get references
curl -s https://your-portal/portal/v1/routes/references | jq .

# Create a test route
curl -s -X POST https://your-portal/portal/v1/routes \
  -H 'Content-Type: application/json' \
  -d '{"name":"Test Route","route_type":"sms","provider_id":"<provider-uuid>","priority":10,"share":100,"status":"draft","condition_groups":[{"logic_op":"IF","conditions":[{"type":"country","value":"RU"}]}],"schedules":[]}' | jq .
```

- [ ] **Step 5: Verify frontend UI**

Open the portal in a browser, navigate to "Общие маршруты", and verify:
- Route table loads and displays
- Filters work
- Create route modal opens and all sections render
- Condition editor allows adding/removing groups and conditions
- Schedule editor toggle works
- Preview sidebar updates reactively
- Save creates the route
- Edit loads existing route data
- Delete removes route
