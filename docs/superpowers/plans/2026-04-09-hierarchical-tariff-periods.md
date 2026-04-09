# Hierarchical Tariff Periods Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat `tariff_plans` + `tariff_periods` model with dimension-based hierarchical periods supporting global/country/operator/sender_category/traffic_type/client overrides with priority-based lookup, auto-close, containment validation, and open-ended periods.

**Architecture:** New `tariff_periods_new` table (renamed to `tariff_periods` post-migration) carries dimension columns and computed scope_key/scope_priority. A `PeriodService` in admin-gateway handles CRUD business logic (auto-close, containment validation, child protection) via direct DB access. The tarification service's `TarifyMessage` lookup is updated to use the hierarchical algorithm. Old tables kept during transition until data migration completes.

**Tech Stack:** Go 1.24.0 + `database/sql`/pgx v5, PostgreSQL 15+ (btree_gist), gorilla/mux, TypeScript 5.7 + React 19, Radix UI, Tailwind CSS 4.2

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `migrations/000081_hierarchical_tariff_periods.up.sql` | Create | New tables: tariff_periods_new, tariff_tiers_new, provider_cost_periods, provider_cost_tiers |
| `migrations/000081_hierarchical_tariff_periods.down.sql` | Create | Drop new tables |
| `migrations/000082_migrate_tariff_plans_to_periods.up.sql` | Create | Copy old tariff_plans+tariff_periods → tariff_periods_new |
| `migrations/000082_migrate_tariff_plans_to_periods.down.sql` | Create | Truncate tariff_periods_new |
| `internal/gateway/admin/services/period_service.go` | Create | Business logic: scope_key/priority, auto-close, validation, CRUD |
| `internal/gateway/admin/services/period_service_test.go` | Create | Unit tests for PeriodService |
| `internal/gateway/admin/handlers/hierarchical_periods.go` | Create | HTTP handlers for /admin/v1/tarification/periods and /provider-costs |
| `internal/gateway/admin/router/router.go` | Modify | Register new routes |
| `internal/services/tarification/infrastructure/repository/hierarchical_period_repository.go` | Create | Hierarchical lookup for TarifyMessage |
| `internal/services/tarification/application/tarification_service.go` | Modify | Use hierarchical lookup in findActiveTariff |
| `portal-frontend/src/api/admin.ts` | Modify | New types + API methods for hierarchical periods |
| `portal-frontend/src/pages/admin/tarification/PeriodsTab.tsx` | Modify | Rewrite with dimension filters + hierarchy display |

---

### Task 1: DB Migration — new hierarchical tables

**Files:**
- Create: `migrations/000081_hierarchical_tariff_periods.up.sql`
- Create: `migrations/000081_hierarchical_tariff_periods.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- migrations/000081_hierarchical_tariff_periods.up.sql

-- btree_gist already enabled in migration 000013

CREATE TABLE IF NOT EXISTS tariff_periods_new (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  -- Dimensions (all nullable, NULL = "any")
  country_id      UUID REFERENCES countries(id),
  operator_id     UUID REFERENCES operators(id),
  sender_category TEXT CHECK (sender_category IN ('shared', 'paid_registered', 'free_registered')),
  traffic_type    TEXT CHECK (traffic_type IN ('authorization', 'transactional', 'service', 'extensible')),
  client_id       UUID REFERENCES accounts(id),

  -- Scope (computed on write, never edited)
  scope_key       TEXT NOT NULL,
  scope_priority  INT  NOT NULL,

  -- Strategy
  strategy        TEXT NOT NULL CHECK (strategy IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold')),

  -- Dates
  start_date      DATE NOT NULL,
  end_date        DATE,  -- NULL = open-ended

  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT valid_dates CHECK (end_date IS NULL OR end_date > start_date)
);

-- Overlap prevention: treat open-ended as far-future date for exclusion
ALTER TABLE tariff_periods_new ADD CONSTRAINT no_overlap_in_scope
  EXCLUDE USING gist (
    scope_key WITH =,
    daterange(start_date, COALESCE(end_date, '9999-12-31'::date), '[]') WITH &&
  );

CREATE INDEX idx_tariff_periods_new_lookup ON tariff_periods_new (
  country_id, operator_id, sender_category, traffic_type, client_id,
  start_date, end_date
);

CREATE INDEX idx_tariff_periods_new_scope ON tariff_periods_new (scope_priority, scope_key);

-- Tiers for new periods (same structure, new FK)
CREATE TABLE IF NOT EXISTS tariff_tiers_new (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tariff_period_id  UUID NOT NULL REFERENCES tariff_periods_new(id) ON DELETE CASCADE,
  from_count        INT NOT NULL CHECK (from_count >= 0),
  price_per_segment NUMERIC(20,6) NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tariff_period_id, from_count)
);

-- Provider cost periods
CREATE TABLE IF NOT EXISTS provider_cost_periods (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider_id     UUID NOT NULL REFERENCES providers(id),
  country_id      UUID REFERENCES countries(id),
  operator_id     UUID REFERENCES operators(id),
  traffic_type    TEXT CHECK (traffic_type IN ('authorization', 'transactional', 'service', 'extensible')),

  scope_key       TEXT NOT NULL,
  scope_priority  INT  NOT NULL,
  strategy        TEXT NOT NULL CHECK (strategy IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold')),

  start_date      DATE NOT NULL,
  end_date        DATE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT valid_cost_dates CHECK (end_date IS NULL OR end_date > start_date)
);

ALTER TABLE provider_cost_periods ADD CONSTRAINT no_cost_overlap_in_scope
  EXCLUDE USING gist (
    scope_key WITH =,
    daterange(start_date, COALESCE(end_date, '9999-12-31'::date), '[]') WITH &&
  );

CREATE INDEX idx_provider_cost_periods_lookup ON provider_cost_periods (
  provider_id, country_id, operator_id, traffic_type, start_date, end_date
);

CREATE TABLE IF NOT EXISTS provider_cost_tiers (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cost_period_id   UUID NOT NULL REFERENCES provider_cost_periods(id) ON DELETE CASCADE,
  from_count       INT NOT NULL CHECK (from_count >= 0),
  cost_per_segment NUMERIC(20,6) NOT NULL,
  UNIQUE (cost_period_id, from_count)
);
```

- [ ] **Step 2: Write the down migration**

```sql
-- migrations/000081_hierarchical_tariff_periods.down.sql
DROP TABLE IF EXISTS provider_cost_tiers;
DROP TABLE IF EXISTS provider_cost_periods;
DROP TABLE IF EXISTS tariff_tiers_new;
DROP TABLE IF EXISTS tariff_periods_new;
```

- [ ] **Step 3: Apply and verify**

Run: `migrate -path migrations -database "$DATABASE_URL" up 1`

Then verify with psql:
```sql
\d tariff_periods_new
\d tariff_tiers_new
\d provider_cost_periods
\d provider_cost_tiers
```
Expected: 4 tables with correct columns and exclusion constraints.

- [ ] **Step 4: Commit**

```bash
git add migrations/000081_hierarchical_tariff_periods.up.sql \
        migrations/000081_hierarchical_tariff_periods.down.sql
git commit -m "feat(db): add hierarchical tariff_periods_new and provider_cost_periods tables"
```

---

### Task 2: Admin gateway PeriodService — scope logic + business rules

**Files:**
- Create: `internal/gateway/admin/services/period_service.go`
- Create: `internal/gateway/admin/services/period_service_test.go`

- [ ] **Step 1: Write failing tests for scope_key and scope_priority computation**

```go
// internal/gateway/admin/services/period_service_test.go
package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/services"
	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T { return &v }

func TestComputeScopeKey(t *testing.T) {
	kz := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	bee := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	acme := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	cases := []struct {
		name     string
		dims     services.PeriodDimensions
		wantKey  string
		wantPrio int
	}{
		{
			name:     "global",
			dims:     services.PeriodDimensions{},
			wantKey:  "global",
			wantPrio: 0,
		},
		{
			name:     "country only",
			dims:     services.PeriodDimensions{CountryID: &kz},
			wantKey:  "country:" + kz.String(),
			wantPrio: 10,
		},
		{
			name:     "country + operator",
			dims:     services.PeriodDimensions{CountryID: &kz, OperatorID: &bee},
			wantKey:  "country:" + kz.String() + "|operator:" + bee.String(),
			wantPrio: 20,
		},
		{
			name:     "country + operator + sender_category",
			dims:     services.PeriodDimensions{CountryID: &kz, OperatorID: &bee, SenderCategory: ptr("paid_registered")},
			wantKey:  "country:" + kz.String() + "|operator:" + bee.String() + "|sender_category:paid_registered",
			wantPrio: 30,
		},
		{
			name: "country + operator + sender_category + traffic_type",
			dims: services.PeriodDimensions{
				CountryID: &kz, OperatorID: &bee,
				SenderCategory: ptr("paid_registered"), TrafficType: ptr("transactional"),
			},
			wantKey:  "country:" + kz.String() + "|operator:" + bee.String() + "|sender_category:paid_registered|traffic_type:transactional",
			wantPrio: 40,
		},
		{
			name:     "country + client",
			dims:     services.PeriodDimensions{CountryID: &kz, ClientID: &acme},
			wantKey:  "client:" + acme.String() + "|country:" + kz.String(),
			wantPrio: 110,
		},
		{
			name:     "global + client",
			dims:     services.PeriodDimensions{ClientID: &acme},
			wantKey:  "client:" + acme.String(),
			wantPrio: 100,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantKey, services.ComputeScopeKey(tc.dims))
			assert.Equal(t, tc.wantPrio, services.ComputeScopePriority(tc.dims))
		})
	}
}

func TestValidateDimensionHierarchy(t *testing.T) {
	id := uuid.New()

	assert.NoError(t, services.ValidateDimensionHierarchy(services.PeriodDimensions{}))
	assert.NoError(t, services.ValidateDimensionHierarchy(services.PeriodDimensions{CountryID: &id}))
	assert.NoError(t, services.ValidateDimensionHierarchy(services.PeriodDimensions{CountryID: &id, OperatorID: &id}))

	// operator requires country
	err := services.ValidateDimensionHierarchy(services.PeriodDimensions{OperatorID: &id})
	assert.ErrorIs(t, err, services.ErrMissingCountry)

	// sender_category requires operator
	err = services.ValidateDimensionHierarchy(services.PeriodDimensions{CountryID: &id, SenderCategory: ptr("shared")})
	assert.ErrorIs(t, err, services.ErrMissingOperator)

	// traffic_type requires sender_category
	err = services.ValidateDimensionHierarchy(services.PeriodDimensions{CountryID: &id, OperatorID: &id, TrafficType: ptr("transactional")})
	assert.ErrorIs(t, err, services.ErrMissingSenderCategory)
}

func ptr[T any](v T) *T { return &v }
```

- [ ] **Step 2: Run test — verify it fails**

Run: `cd c:/projects/sms && go test ./internal/gateway/admin/services/... -v`
Expected: FAIL — package not found / ComputeScopeKey undefined.

- [ ] **Step 3: Implement period_service.go**

```go
// internal/gateway/admin/services/period_service.go
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// sentinel errors
var (
	ErrMissingCountry        = errors.New("operator_id requires country_id")
	ErrMissingOperator       = errors.New("sender_category requires operator_id")
	ErrMissingSenderCategory = errors.New("traffic_type requires sender_category")
	ErrNoParentPeriod        = errors.New("no active period at parent level for these dates")
	ErrChildPeriodExceedsParent = errors.New("period exceeds parent period bounds")
	ErrChildPeriodsBlock     = errors.New("cannot close period: active child periods exist")
	ErrPeriodOverlap         = errors.New("period overlaps with existing period in this scope")
	ErrHasChildPeriods       = errors.New("cannot delete: child periods exist")
	ErrPeriodNotFound        = errors.New("period not found")
	ErrRetroactiveStart      = errors.New("start_date must be today or in the future")
	ErrInvalidStrategy       = errors.New("invalid strategy")
	ErrInvalidSenderCategory = errors.New("invalid sender_category")
	ErrInvalidTrafficType    = errors.New("invalid traffic_type")
)

// PeriodDimensions holds all optional dimension values for a hierarchical period.
type PeriodDimensions struct {
	CountryID      *uuid.UUID
	OperatorID     *uuid.UUID
	SenderCategory *string
	TrafficType    *string
	ClientID       *uuid.UUID
}

// HierarchicalPeriod is the full period record returned from DB.
type HierarchicalPeriod struct {
	ID            uuid.UUID
	PeriodDimensions
	ScopeKey      string
	ScopePriority int
	Strategy      string
	StartDate     time.Time
	EndDate       *time.Time // nil = open-ended
	CreatedAt     time.Time
}

// CreatePeriodInput is the validated input for period creation.
type CreatePeriodInput struct {
	PeriodDimensions
	Strategy  string
	StartDate time.Time
	EndDate   *time.Time
}

// UpdatePeriodInput allows editing end_date and strategy only.
type UpdatePeriodInput struct {
	EndDate  *time.Time // nil = open-ended
	Strategy *string
}

// AutoCloseWarning describes a period that will be auto-closed.
type AutoCloseWarning struct {
	PeriodID    uuid.UUID
	OldEndDate  *time.Time // nil = was open-ended
	NewEndDate  time.Time
}

// ComputeScopeKey builds the canonical scope key from filled dimensions,
// sorted alphabetically so the key is stable regardless of insertion order.
func ComputeScopeKey(d PeriodDimensions) string {
	parts := map[string]string{}
	if d.ClientID != nil {
		parts["client"] = d.ClientID.String()
	}
	if d.CountryID != nil {
		parts["country"] = d.CountryID.String()
	}
	if d.OperatorID != nil {
		parts["operator"] = d.OperatorID.String()
	}
	if d.SenderCategory != nil {
		parts["sender_category"] = *d.SenderCategory
	}
	if d.TrafficType != nil {
		parts["traffic_type"] = *d.TrafficType
	}
	if len(parts) == 0 {
		return "global"
	}
	keys := make([]string, 0, len(parts))
	for k := range parts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	segments := make([]string, 0, len(keys))
	for _, k := range keys {
		segments = append(segments, k+":"+parts[k])
	}
	return strings.Join(segments, "|")
}

// ComputeScopePriority returns the numeric priority based on the most specific
// dimension filled (excluding client_id), plus 100 if client_id is set.
func ComputeScopePriority(d PeriodDimensions) int {
	base := 0
	if d.CountryID != nil {
		base = 10
	}
	if d.OperatorID != nil {
		base = 20
	}
	if d.SenderCategory != nil {
		base = 30
	}
	if d.TrafficType != nil {
		base = 40
	}
	if d.ClientID != nil {
		base += 100
	}
	return base
}

// ValidateDimensionHierarchy enforces the strict hierarchy rule:
// operator requires country, sender_category requires operator, traffic_type requires sender_category.
func ValidateDimensionHierarchy(d PeriodDimensions) error {
	if d.OperatorID != nil && d.CountryID == nil {
		return ErrMissingCountry
	}
	if d.SenderCategory != nil && d.OperatorID == nil {
		return ErrMissingOperator
	}
	if d.TrafficType != nil && d.SenderCategory == nil {
		return ErrMissingSenderCategory
	}
	return nil
}

// parentScopeKey computes the parent's scope key by removing the most specific dimension.
// Returns ("", false) for global scope (no parent).
func parentScopeKey(d PeriodDimensions) (string, bool) {
	// Client is a parallel axis — parent is same scope without client
	if d.ClientID != nil {
		noClient := d
		noClient.ClientID = nil
		return ComputeScopeKey(noClient), true
	}
	// Remove most specific non-client dimension
	if d.TrafficType != nil {
		noTraffic := d
		noTraffic.TrafficType = nil
		return ComputeScopeKey(noTraffic), true
	}
	if d.SenderCategory != nil {
		noCat := d
		noCat.SenderCategory = nil
		return ComputeScopeKey(noCat), true
	}
	if d.OperatorID != nil {
		noOp := d
		noOp.OperatorID = nil
		return ComputeScopeKey(noOp), true
	}
	if d.CountryID != nil {
		return "global", true
	}
	// Already global
	return "", false
}

// PeriodService implements business logic for hierarchical tariff periods.
type PeriodService struct {
	db     *storage.DB
	logger zerolog.Logger
}

// NewPeriodService creates a new PeriodService.
func NewPeriodService(db *storage.DB) *PeriodService {
	return &PeriodService{
		db:     db,
		logger: log.With().Str("component", "period-service").Logger(),
	}
}

// validateStrategy checks that the strategy value is one of the allowed values.
func validateStrategy(s string) error {
	switch s {
	case "fixed", "threshold", "threshold_recalc", "prepaid_threshold":
		return nil
	}
	return ErrInvalidStrategy
}

// validateSenderCategory checks the sender_category value.
func validateSenderCategory(s string) error {
	switch s {
	case "shared", "paid_registered", "free_registered":
		return nil
	}
	return ErrInvalidSenderCategory
}

// validateTrafficType checks the traffic_type value.
func validateTrafficType(s string) error {
	switch s {
	case "authorization", "transactional", "service", "extensible":
		return nil
	}
	return ErrInvalidTrafficType
}

// ListPeriods returns periods filtered by any combination of dimensions.
func (s *PeriodService) ListPeriods(ctx context.Context, filter PeriodDimensions) ([]*HierarchicalPeriod, error) {
	query := `
		SELECT id, country_id, operator_id, sender_category, traffic_type, client_id,
		       scope_key, scope_priority, strategy, start_date, end_date, created_at
		FROM tariff_periods_new
		WHERE ($1::uuid IS NULL OR country_id = $1)
		  AND ($2::uuid IS NULL OR operator_id = $2)
		  AND ($3::text IS NULL OR sender_category = $3)
		  AND ($4::text IS NULL OR traffic_type = $4)
		  AND ($5::uuid IS NULL OR client_id = $5)
		ORDER BY scope_priority DESC, start_date DESC`

	rows, err := s.db.QueryContext(ctx, query,
		filter.CountryID, filter.OperatorID, filter.SenderCategory, filter.TrafficType, filter.ClientID)
	if err != nil {
		return nil, fmt.Errorf("list periods: %w", err)
	}
	defer rows.Close()

	var result []*HierarchicalPeriod
	for rows.Next() {
		p, err := scanPeriod(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// GetPeriod returns a single period by ID.
func (s *PeriodService) GetPeriod(ctx context.Context, id uuid.UUID) (*HierarchicalPeriod, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, country_id, operator_id, sender_category, traffic_type, client_id,
		       scope_key, scope_priority, strategy, start_date, end_date, created_at
		FROM tariff_periods_new WHERE id = $1`, id)
	p, err := scanPeriod(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPeriodNotFound
	}
	return p, err
}

// CreatePeriod creates a new hierarchical period with full lifecycle validation.
// Returns the created period and an optional AutoCloseWarning if a previous period was closed.
func (s *PeriodService) CreatePeriod(ctx context.Context, in CreatePeriodInput) (*HierarchicalPeriod, *AutoCloseWarning, error) {
	// 1. Validate enum fields
	if err := validateStrategy(in.Strategy); err != nil {
		return nil, nil, err
	}
	if in.SenderCategory != nil {
		if err := validateSenderCategory(*in.SenderCategory); err != nil {
			return nil, nil, err
		}
	}
	if in.TrafficType != nil {
		if err := validateTrafficType(*in.TrafficType); err != nil {
			return nil, nil, err
		}
	}

	// 2. Validate dimension hierarchy
	if err := ValidateDimensionHierarchy(in.PeriodDimensions); err != nil {
		return nil, nil, err
	}

	// 3. start_date must not be in the past
	today := time.Now().Truncate(24 * time.Hour)
	if in.StartDate.Before(today) {
		return nil, nil, ErrRetroactiveStart
	}

	scopeKey := ComputeScopeKey(in.PeriodDimensions)
	scopePriority := ComputeScopePriority(in.PeriodDimensions)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// 4. Containment check (skip for global scope)
	if scopePriority > 0 {
		parentKey, hasParent := parentScopeKey(in.PeriodDimensions)
		if hasParent {
			parent, err := findActivePeriodInScope(ctx, tx, parentKey, in.StartDate)
			if err != nil || parent == nil {
				return nil, nil, ErrNoParentPeriod
			}
			if in.StartDate.Before(parent.StartDate) {
				return nil, nil, fmt.Errorf("%w: starts before parent (%s)", ErrChildPeriodExceedsParent, parent.StartDate.Format("2006-01-02"))
			}
			if parent.EndDate != nil && in.EndDate != nil && in.EndDate.After(*parent.EndDate) {
				return nil, nil, fmt.Errorf("%w: ends after parent (%s)", ErrChildPeriodExceedsParent, parent.EndDate.Format("2006-01-02"))
			}
			if parent.EndDate != nil && in.EndDate == nil {
				return nil, nil, fmt.Errorf("%w: parent is not open-ended, child cannot be open-ended", ErrChildPeriodExceedsParent)
			}
		}
	}

	// 5. Auto-close: find open or overlapping period in same scope
	var warning *AutoCloseWarning
	prev, err := findOpenOrFuturePeriodInScope(ctx, tx, scopeKey, in.StartDate)
	if err != nil {
		return nil, nil, fmt.Errorf("check existing periods: %w", err)
	}
	if prev != nil {
		if prev.EndDate != nil {
			// Closed period that overlaps — cannot auto-close, this is an overlap error
			return nil, nil, fmt.Errorf("%w with period %s–%s", ErrPeriodOverlap, prev.StartDate.Format("2006-01-02"), prev.EndDate.Format("2006-01-02"))
		}
		// Open-ended: check children before closing
		if err := assertNoActiveChildren(ctx, tx, prev.ID, in.StartDate); err != nil {
			return nil, nil, err
		}
		// Close it: end_date = start_date - 1 day
		newEnd := in.StartDate.AddDate(0, 0, -1)
		_, err = tx.ExecContext(ctx, `UPDATE tariff_periods_new SET end_date = $1 WHERE id = $2`, newEnd, prev.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("auto-close previous period: %w", err)
		}
		warning = &AutoCloseWarning{PeriodID: prev.ID, OldEndDate: prev.EndDate, NewEndDate: newEnd}
	}

	// 6. Insert new period
	id := uuid.New()
	now := time.Now()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tariff_periods_new
		  (id, country_id, operator_id, sender_category, traffic_type, client_id,
		   scope_key, scope_priority, strategy, start_date, end_date, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		id,
		in.CountryID, in.OperatorID, in.SenderCategory, in.TrafficType, in.ClientID,
		scopeKey, scopePriority, in.Strategy, in.StartDate, in.EndDate, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no_overlap_in_scope") {
			return nil, nil, ErrPeriodOverlap
		}
		return nil, nil, fmt.Errorf("insert period: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	s.logger.Info().
		Str("id", id.String()).
		Str("scope_key", scopeKey).
		Int("scope_priority", scopePriority).
		Msg("hierarchical period created")

	period := &HierarchicalPeriod{
		ID:            id,
		PeriodDimensions: in.PeriodDimensions,
		ScopeKey:      scopeKey,
		ScopePriority: scopePriority,
		Strategy:      in.Strategy,
		StartDate:     in.StartDate,
		EndDate:       in.EndDate,
		CreatedAt:     now,
	}
	return period, warning, nil
}

// UpdatePeriod allows editing end_date (with child containment check) and strategy.
func (s *PeriodService) UpdatePeriod(ctx context.Context, id uuid.UUID, in UpdatePeriodInput) (*HierarchicalPeriod, error) {
	period, err := s.GetPeriod(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Strategy != nil {
		if err := validateStrategy(*in.Strategy); err != nil {
			return nil, err
		}
		period.Strategy = *in.Strategy
	}

	if in.EndDate != period.EndDate { // end_date is changing
		// Validate no child period exceeds new end_date
		if in.EndDate != nil {
			if err := assertNoChildExceedsDate(ctx, s.db, id, period.ScopeKey, *in.EndDate); err != nil {
				return nil, err
			}
		}
		period.EndDate = in.EndDate
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE tariff_periods_new SET strategy = $1, end_date = $2 WHERE id = $3`,
		period.Strategy, period.EndDate, id)
	if err != nil {
		return nil, fmt.Errorf("update period: %w", err)
	}

	return period, nil
}

// DeletePeriod deletes a period if it has no child periods. Cascades to tiers.
func (s *PeriodService) DeletePeriod(ctx context.Context, id uuid.UUID) error {
	period, err := s.GetPeriod(ctx, id)
	if err != nil {
		return err
	}

	count, err := countChildPeriods(ctx, s.db, period.ScopeKey, period.ScopePriority)
	if err != nil {
		return fmt.Errorf("check child periods: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w (%d)", ErrHasChildPeriods, count)
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM tariff_periods_new WHERE id = $1`, id)
	return err
}

// --- helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func scanPeriod(row scanner) (*HierarchicalPeriod, error) {
	var p HierarchicalPeriod
	var countryID, operatorID, clientID sql.NullString
	var senderCategory, trafficType sql.NullString
	var endDate sql.NullTime

	err := row.Scan(
		&p.ID, &countryID, &operatorID, &senderCategory, &trafficType, &clientID,
		&p.ScopeKey, &p.ScopePriority, &p.Strategy, &p.StartDate, &endDate, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if countryID.Valid {
		id := uuid.MustParse(countryID.String)
		p.CountryID = &id
	}
	if operatorID.Valid {
		id := uuid.MustParse(operatorID.String)
		p.OperatorID = &id
	}
	if clientID.Valid {
		id := uuid.MustParse(clientID.String)
		p.ClientID = &id
	}
	if senderCategory.Valid {
		p.SenderCategory = &senderCategory.String
	}
	if trafficType.Valid {
		p.TrafficType = &trafficType.String
	}
	if endDate.Valid {
		t := endDate.Time
		p.EndDate = &t
	}
	return &p, nil
}

func findActivePeriodInScope(ctx context.Context, tx *sql.Tx, scopeKey string, date time.Time) (*HierarchicalPeriod, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, country_id, operator_id, sender_category, traffic_type, client_id,
		       scope_key, scope_priority, strategy, start_date, end_date, created_at
		FROM tariff_periods_new
		WHERE scope_key = $1
		  AND start_date <= $2
		  AND (end_date IS NULL OR end_date >= $2)
		LIMIT 1`, scopeKey, date)
	p, err := scanPeriod(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// findOpenOrFuturePeriodInScope finds an open-ended period OR a closed period that overlaps [startDate, ∞).
func findOpenOrFuturePeriodInScope(ctx context.Context, tx *sql.Tx, scopeKey string, startDate time.Time) (*HierarchicalPeriod, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, country_id, operator_id, sender_category, traffic_type, client_id,
		       scope_key, scope_priority, strategy, start_date, end_date, created_at
		FROM tariff_periods_new
		WHERE scope_key = $1
		  AND (end_date IS NULL OR end_date >= $2)
		LIMIT 1`, scopeKey, startDate)
	p, err := scanPeriod(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func assertNoActiveChildren(ctx context.Context, tx *sql.Tx, parentID uuid.UUID, newEnd time.Time) error {
	// Children have higher scope_priority and their scope_key contains the parent's scope_key as prefix
	// We check child periods that would extend beyond newEnd
	var childNames []string
	rows, err := tx.QueryContext(ctx, `
		SELECT scope_key
		FROM tariff_periods_new p
		WHERE p.scope_priority > (SELECT scope_priority FROM tariff_periods_new WHERE id = $1)
		  AND p.scope_key LIKE '%' || (SELECT substring(scope_key from 1) FROM tariff_periods_new WHERE id = $1) || '%'
		  AND (p.end_date IS NULL OR p.end_date > $2)`, parentID, newEnd)
	if err != nil {
		return fmt.Errorf("check children: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return err
		}
		childNames = append(childNames, k)
	}
	if len(childNames) > 0 {
		return fmt.Errorf("%w: %s", ErrChildPeriodsBlock, strings.Join(childNames, ", "))
	}
	return nil
}

func assertNoChildExceedsDate(ctx context.Context, db *storage.DB, parentID uuid.UUID, scopeKey string, newEnd time.Time) error {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tariff_periods_new
		WHERE scope_key LIKE $1 || '%'
		  AND scope_key != $1
		  AND (end_date IS NULL OR end_date > $2)`, scopeKey, newEnd).Scan(&count)
	if err != nil {
		return fmt.Errorf("check child end dates: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w: %d child periods extend beyond new end_date", ErrChildPeriodExceedsParent, count)
	}
	return nil
}

func countChildPeriods(ctx context.Context, db *storage.DB, scopeKey string, scopePriority int) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tariff_periods_new
		WHERE scope_priority > $1
		  AND scope_key LIKE $2 || '%'
		  AND scope_key != $2`, scopePriority, scopeKey).Scan(&count)
	return count, err
}
```

- [ ] **Step 4: Run tests — verify they pass**

Run: `cd c:/projects/sms && go test ./internal/gateway/admin/services/... -v -run TestComputeScopeKey`
Expected: PASS (scope_key and scope_priority tests).

Run: `cd c:/projects/sms && go test ./internal/gateway/admin/services/... -v -run TestValidateDimensionHierarchy`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/admin/services/period_service.go \
        internal/gateway/admin/services/period_service_test.go
git commit -m "feat(tarification): add PeriodService with hierarchical scope logic and lifecycle rules"
```

---

### Task 3: Admin gateway HTTP handlers for new periods API

**Files:**
- Create: `internal/gateway/admin/handlers/hierarchical_periods.go`

- [ ] **Step 1: Write the handlers**

```go
// internal/gateway/admin/handlers/hierarchical_periods.go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/services"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// HierarchicalPeriodsHandler handles the new dimension-based periods API.
type HierarchicalPeriodsHandler struct {
	svc *services.PeriodService
}

// NewHierarchicalPeriodsHandler creates the handler.
func NewHierarchicalPeriodsHandler(db *storage.DB) *HierarchicalPeriodsHandler {
	return &HierarchicalPeriodsHandler{svc: services.NewPeriodService(db)}
}

// periodResponse is the JSON shape returned to the frontend.
type periodResponse struct {
	ID             string  `json:"id"`
	CountryID      *string `json:"country_id"`
	OperatorID     *string `json:"operator_id"`
	SenderCategory *string `json:"sender_category"`
	TrafficType    *string `json:"traffic_type"`
	ClientID       *string `json:"client_id"`
	ScopeKey       string  `json:"scope_key"`
	ScopePriority  int     `json:"scope_priority"`
	Strategy       string  `json:"strategy"`
	StartDate      string  `json:"start_date"`
	EndDate        *string `json:"end_date"`
	CreatedAt      string  `json:"created_at"`
}

func periodToResponse(p *services.HierarchicalPeriod) periodResponse {
	r := periodResponse{
		ID:            p.ID.String(),
		ScopeKey:      p.ScopeKey,
		ScopePriority: p.ScopePriority,
		Strategy:      p.Strategy,
		StartDate:     p.StartDate.Format("2006-01-02"),
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
	}
	if p.CountryID != nil {
		s := p.CountryID.String(); r.CountryID = &s
	}
	if p.OperatorID != nil {
		s := p.OperatorID.String(); r.OperatorID = &s
	}
	if p.ClientID != nil {
		s := p.ClientID.String(); r.ClientID = &s
	}
	r.SenderCategory = p.SenderCategory
	r.TrafficType = p.TrafficType
	if p.EndDate != nil {
		s := p.EndDate.Format("2006-01-02"); r.EndDate = &s
	}
	return r
}

// parseOptionalUUID returns nil if the string is empty, otherwise parses it.
func parseOptionalUUID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// parseOptionalString returns nil if empty.
func parseOptionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ListPeriods handles GET /admin/v1/tarification/periods
func (h *HierarchicalPeriodsHandler) ListPeriods(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	countryID, err := parseOptionalUUID(q.Get("country_id"))
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid country_id"))
		return
	}
	operatorID, err := parseOptionalUUID(q.Get("operator_id"))
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid operator_id"))
		return
	}
	clientID, err := parseOptionalUUID(q.Get("client_id"))
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid client_id"))
		return
	}

	filter := services.PeriodDimensions{
		CountryID:      countryID,
		OperatorID:     operatorID,
		SenderCategory: parseOptionalString(q.Get("sender_category")),
		TrafficType:    parseOptionalString(q.Get("traffic_type")),
		ClientID:       clientID,
	}

	periods, err := h.svc.ListPeriods(r.Context(), filter)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	resp := make([]periodResponse, 0, len(periods))
	for _, p := range periods {
		resp = append(resp, periodToResponse(p))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"periods": resp,
		"total":   len(resp),
	})
}

// createPeriodRequest is the POST body.
type createPeriodRequest struct {
	CountryID      *string `json:"country_id"`
	OperatorID     *string `json:"operator_id"`
	SenderCategory *string `json:"sender_category"`
	TrafficType    *string `json:"traffic_type"`
	ClientID       *string `json:"client_id"`
	Strategy       string  `json:"strategy"`
	StartDate      string  `json:"start_date"` // YYYY-MM-DD
	EndDate        *string `json:"end_date"`   // YYYY-MM-DD or null
}

// CreatePeriod handles POST /admin/v1/tarification/periods
func (h *HierarchicalPeriodsHandler) CreatePeriod(w http.ResponseWriter, r *http.Request) {
	var req createPeriodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	if req.Strategy == "" || req.StartDate == "" {
		respondError(w, shared.ErrInvalidInput("strategy and start_date are required"))
		return
	}

	dims, err := parseDimensions(req.CountryID, req.OperatorID, req.SenderCategory, req.TrafficType, req.ClientID)
	if err != nil {
		respondError(w, shared.ErrInvalidInput(err.Error()))
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid start_date format, use YYYY-MM-DD"))
		return
	}

	var endDate *time.Time
	if req.EndDate != nil {
		d, err := time.Parse("2006-01-02", *req.EndDate)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("invalid end_date format, use YYYY-MM-DD"))
			return
		}
		endDate = &d
	}

	period, warning, err := h.svc.CreatePeriod(r.Context(), services.CreatePeriodInput{
		PeriodDimensions: dims,
		Strategy:         req.Strategy,
		StartDate:        startDate,
		EndDate:          endDate,
	})
	if err != nil {
		respondPeriodError(w, err)
		return
	}

	body := map[string]interface{}{
		"period": periodToResponse(period),
	}
	if warning != nil {
		body["auto_close_warning"] = map[string]interface{}{
			"period_id":    warning.PeriodID.String(),
			"new_end_date": warning.NewEndDate.Format("2006-01-02"),
		}
	}
	respondJSON(w, http.StatusCreated, body)
}

// updatePeriodRequest is the PUT body.
type updatePeriodRequest struct {
	EndDate  *string `json:"end_date"` // YYYY-MM-DD or null
	Strategy *string `json:"strategy"`
}

// UpdatePeriod handles PUT /admin/v1/tarification/periods/{id}
func (h *HierarchicalPeriodsHandler) UpdatePeriod(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid period id"))
		return
	}

	var req updatePeriodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}

	in := services.UpdatePeriodInput{Strategy: req.Strategy}
	if req.EndDate != nil {
		d, err := time.Parse("2006-01-02", *req.EndDate)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("invalid end_date format"))
			return
		}
		in.EndDate = &d
	}

	period, err := h.svc.UpdatePeriod(r.Context(), id, in)
	if err != nil {
		respondPeriodError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"period": periodToResponse(period)})
}

// DeletePeriod handles DELETE /admin/v1/tarification/periods/{id}
func (h *HierarchicalPeriodsHandler) DeletePeriod(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid period id"))
		return
	}
	if err := h.svc.DeletePeriod(r.Context(), id); err != nil {
		respondPeriodError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseDimensions converts nullable string pointers to uuid.UUID pointers.
func parseDimensions(countryStr, operatorStr, senderCat, trafficType, clientStr *string) (services.PeriodDimensions, error) {
	d := services.PeriodDimensions{
		SenderCategory: senderCat,
		TrafficType:    trafficType,
	}
	var err error
	if countryStr != nil {
		d.CountryID, err = parseOptionalUUID(*countryStr)
		if err != nil {
			return d, fmt.Errorf("invalid country_id: %w", err)
		}
	}
	if operatorStr != nil {
		d.OperatorID, err = parseOptionalUUID(*operatorStr)
		if err != nil {
			return d, fmt.Errorf("invalid operator_id: %w", err)
		}
	}
	if clientStr != nil {
		d.ClientID, err = parseOptionalUUID(*clientStr)
		if err != nil {
			return d, fmt.Errorf("invalid client_id: %w", err)
		}
	}
	return d, nil
}

// respondPeriodError maps service errors to HTTP status codes.
func respondPeriodError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrPeriodNotFound):
		respondError(w, shared.ErrNotFound(err.Error()))
	case errors.Is(err, services.ErrMissingCountry),
		errors.Is(err, services.ErrMissingOperator),
		errors.Is(err, services.ErrMissingSenderCategory),
		errors.Is(err, services.ErrRetroactiveStart),
		errors.Is(err, services.ErrInvalidStrategy),
		errors.Is(err, services.ErrInvalidSenderCategory),
		errors.Is(err, services.ErrInvalidTrafficType):
		respondError(w, shared.ErrInvalidInput(err.Error()))
	case errors.Is(err, services.ErrNoParentPeriod),
		errors.Is(err, services.ErrChildPeriodExceedsParent),
		errors.Is(err, services.ErrChildPeriodsBlock),
		errors.Is(err, services.ErrHasChildPeriods),
		errors.Is(err, services.ErrPeriodOverlap):
		respondError(w, shared.ErrConflict(err.Error()))
	default:
		respondError(w, shared.ErrInternalServer(err.Error()))
	}
}
```

Note: the handler imports `fmt` — add it to the import block. Also check that `shared.ErrConflict` exists; if not use `shared.ErrInvalidInput`.

- [ ] **Step 2: Check shared error helpers exist**

Run: `grep -r "ErrConflict\|ErrNotFound\|ErrInvalidInput\|ErrInternalServer" c:/projects/sms/internal/shared/ --include="*.go" -l`

If `ErrConflict` is missing, replace it with `shared.ErrInvalidInput` in `respondPeriodError`.

- [ ] **Step 3: Verify it compiles**

Run: `cd c:/projects/sms && go build ./internal/gateway/admin/...`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/admin/handlers/hierarchical_periods.go
git commit -m "feat(admin-gateway): add HTTP handlers for hierarchical tariff periods"
```

---

### Task 4: Register new routes in admin gateway router

**Files:**
- Modify: `internal/gateway/admin/router/router.go`

- [ ] **Step 1: Add handler instantiation and routes**

Find where `TarificationHandler` is created in `router.go`. Add immediately after (or in the same section):

```go
// In the function signature or constructor where db *storage.DB is available:
hierarchicalPeriodsHandler := handlers.NewHierarchicalPeriodsHandler(db)
```

Then in the tarification subrouter block (around line 151), add:

```go
// New hierarchical periods endpoints
tarification.HandleFunc("/periods", hierarchicalPeriodsHandler.ListPeriods).Methods("GET")
tarification.HandleFunc("/periods", hierarchicalPeriodsHandler.CreatePeriod).Methods("POST")
tarification.HandleFunc("/periods/{id}", hierarchicalPeriodsHandler.UpdatePeriod).Methods("PUT")
tarification.HandleFunc("/periods/{id}", hierarchicalPeriodsHandler.DeletePeriod).Methods("DELETE")
```

Read `internal/gateway/admin/router/router.go` first to see the exact constructor signature and where `db` is available.

- [ ] **Step 2: Verify compilation**

Run: `cd c:/projects/sms && go build ./internal/gateway/admin/...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/admin/router/router.go
git commit -m "feat(admin-gateway): register hierarchical periods routes"
```

---

### Task 5: Hierarchical lookup in tarification service

**Files:**
- Create: `internal/services/tarification/infrastructure/repository/hierarchical_period_repository.go`
- Modify: `internal/services/tarification/application/tarification_service.go`

The tarification service currently finds the tariff via:
1. `planRepo.GetActiveByOperatorAndCategory(ctx, operatorID, category)` → gets TariffPlan
2. `periodRepo.GetActiveByPlanID(ctx, planID, now)` → gets TariffPeriod
3. `tierRepo.ListByPeriodID(ctx, periodID)` → gets tiers

Replace this with the hierarchical lookup from the new table.

- [ ] **Step 1: Write the hierarchical lookup query in a new repository**

```go
// internal/services/tarification/infrastructure/repository/hierarchical_period_repository.go
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// HierarchicalPeriodLookupResult is the result of a hierarchical tariff lookup.
type HierarchicalPeriodLookupResult struct {
	PeriodID      uuid.UUID
	Strategy      string
	ScopePriority int
}

// HierarchicalTierResult is a tier from the new table.
type HierarchicalTierResult struct {
	FromCount       int
	PricePerSegment string
}

// HierarchicalPeriodRepository provides lookup queries against tariff_periods_new.
type HierarchicalPeriodRepository struct {
	db *sqlx.DB
}

// NewHierarchicalPeriodRepository creates the repository.
func NewHierarchicalPeriodRepository(db *sqlx.DB) *HierarchicalPeriodRepository {
	return &HierarchicalPeriodRepository{db: db}
}

// FindBestPeriod returns the highest-priority matching period for the given dimensions and date.
// Any nil dimension means "match any value in that dimension".
func (r *HierarchicalPeriodRepository) FindBestPeriod(
	ctx context.Context,
	countryID, operatorID, clientID *uuid.UUID,
	senderCategory, trafficType *string,
	date time.Time,
) (*HierarchicalPeriodLookupResult, error) {
	var result HierarchicalPeriodLookupResult
	err := r.db.QueryRowContext(ctx, `
		SELECT id, strategy, scope_priority
		FROM tariff_periods_new
		WHERE (country_id = $1 OR (country_id IS NULL AND $1 IS NULL) OR country_id IS NULL)
		  AND (operator_id = $2 OR (operator_id IS NULL AND $2 IS NULL) OR operator_id IS NULL)
		  AND (sender_category = $3 OR sender_category IS NULL)
		  AND (traffic_type = $4 OR traffic_type IS NULL)
		  AND (client_id = $5 OR client_id IS NULL)
		  AND start_date <= $6
		  AND (end_date IS NULL OR end_date >= $6)
		  -- prefer exact match on client over "any client" match
		  AND (client_id = $5 OR ($5 IS NULL AND client_id IS NULL) OR client_id IS NULL)
		ORDER BY scope_priority DESC
		LIMIT 1`,
		countryID, operatorID, senderCategory, trafficType, clientID, date,
	).Scan(&result.PeriodID, &result.Strategy, &result.ScopePriority)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FindTiersWithFallback returns tiers for the best-matching period that HAS tiers.
// This implements tier inheritance: walk down scope_priority until tiers are found.
func (r *HierarchicalPeriodRepository) FindTiersWithFallback(
	ctx context.Context,
	countryID, operatorID, clientID *uuid.UUID,
	senderCategory, trafficType *string,
	date time.Time,
) ([]HierarchicalTierResult, string /* strategy from highest-priority period */, error) {
	// First get strategy from the highest-priority period (even if it has no tiers)
	best, err := r.FindBestPeriod(ctx, countryID, operatorID, clientID, senderCategory, trafficType, date)
	if err != nil || best == nil {
		return nil, "", err
	}

	// Then find tiers from the highest-priority period that has them
	rows, err := r.db.QueryContext(ctx, `
		WITH matched AS (
		  SELECT id, scope_priority
		  FROM tariff_periods_new
		  WHERE (country_id = $1 OR country_id IS NULL)
		    AND (operator_id = $2 OR operator_id IS NULL)
		    AND (sender_category = $3 OR sender_category IS NULL)
		    AND (traffic_type = $4 OR traffic_type IS NULL)
		    AND (client_id = $5 OR client_id IS NULL)
		    AND start_date <= $6
		    AND (end_date IS NULL OR end_date >= $6)
		  ORDER BY scope_priority DESC
		)
		SELECT tt.from_count, tt.price_per_segment::text
		FROM matched m
		JOIN tariff_tiers_new tt ON tt.tariff_period_id = m.id
		WHERE EXISTS (SELECT 1 FROM tariff_tiers_new WHERE tariff_period_id = m.id)
		ORDER BY m.scope_priority DESC, tt.from_count ASC
		LIMIT 100`,
		countryID, operatorID, senderCategory, trafficType, clientID, date,
	)
	if err != nil {
		return nil, best.Strategy, err
	}
	defer rows.Close()

	var tiers []HierarchicalTierResult
	for rows.Next() {
		var t HierarchicalTierResult
		if err := rows.Scan(&t.FromCount, &t.PricePerSegment); err != nil {
			return nil, best.Strategy, err
		}
		tiers = append(tiers, t)
	}
	return tiers, best.Strategy, rows.Err()
}
```

- [ ] **Step 2: Verify compilation of repository**

Run: `cd c:/projects/sms && go build ./internal/services/tarification/infrastructure/repository/...`
Expected: no errors.

- [ ] **Step 3: Read the current findActiveTariff / TarifyMessage flow**

Read `internal/services/tarification/application/tarification_service.go` lines 60–150 to understand exactly how it currently calls `planRepo` and `periodRepo`.

- [ ] **Step 4: Add hierarchicalRepo field and update constructor**

In `internal/services/tarification/application/tarification_service.go`, add a new field to `TarificationService`:

```go
// Add to TarificationService struct:
hierarchicalRepo HierarchicalPeriodLookup

// Add interface (put in same file or domain/repository.go):
type HierarchicalPeriodLookup interface {
    FindTiersWithFallback(ctx context.Context, countryID, operatorID, clientID *uuid.UUID, senderCategory, trafficType *string, date time.Time) ([]HierarchicalTierItem, string, error)
}

// HierarchicalTierItem is the lookup result tier.
type HierarchicalTierItem struct {
    FromCount       int
    PricePerSegment string
}
```

Update `NewTarificationService` to accept and store the `HierarchicalPeriodLookup` (add as last parameter).

- [ ] **Step 5: Update TarifyMessage to try hierarchical lookup first**

In the `TarifyMessage` method, before calling `planRepo.GetActiveByOperatorAndCategory`, attempt the hierarchical lookup. If the new table has a result, use it; otherwise fall back to the old path.

```go
// In TarifyMessage, find where it resolves the tariff. Add before existing plan lookup:
if s.hierarchicalRepo != nil {
    tiers, strategy, err := s.hierarchicalRepo.FindTiersWithFallback(
        ctx,
        &req.OperatorCountryID, // you need country_id — add to TarifyMessageRequest
        &req.OperatorID,
        &req.ClientID,
        (*string)(&req.SenderCategory),
        nil, // traffic_type: add to request if needed
        time.Now(),
    )
    if err == nil && len(tiers) > 0 {
        // use tiers and strategy from hierarchical lookup
        // ... apply billing strategy using the matched tiers
    }
}
// Fall through to old path if hierarchical lookup returned nothing
```

**Note:** Adding country_id to `TarifyMessageRequest` requires updating the gRPC proto and callers. This is a large change. Implement the hierarchical lookup as a parallel path alongside the old path, gated by a non-nil `hierarchicalRepo`. Wire it up in `cmd/services/tarification-service/main.go` once the migration is done.

- [ ] **Step 6: Compile the tarification service**

Run: `cd c:/projects/sms && go build ./cmd/services/tarification-service/...`
Fix any compilation errors from the struct/interface changes.

- [ ] **Step 7: Commit**

```bash
git add internal/services/tarification/infrastructure/repository/hierarchical_period_repository.go \
        internal/services/tarification/application/tarification_service.go
git commit -m "feat(tarification): add hierarchical period lookup alongside legacy plan lookup"
```

---

### Task 6: Data migration — copy old tariff_plans + tariff_periods into new table

**Files:**
- Create: `migrations/000082_migrate_tariff_plans_to_periods.up.sql`
- Create: `migrations/000082_migrate_tariff_plans_to_periods.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- migrations/000082_migrate_tariff_plans_to_periods.up.sql
-- Copies existing tariff_plans + tariff_periods into tariff_periods_new.
-- Each old (operator_id, sender_category, strategy) → country_id looked up from operators table.
-- Each old tariff_period row → one row in tariff_periods_new with operator-level scope.

INSERT INTO tariff_periods_new (
  id, country_id, operator_id, sender_category, traffic_type, client_id,
  scope_key, scope_priority, strategy,
  start_date, end_date, created_at
)
SELECT
  tp.id,
  o.country_id,
  plan.operator_id,
  plan.sender_category,
  NULL AS traffic_type,
  NULL AS client_id,
  -- scope_key: country:{country_id}|operator:{operator_id}|sender_category:{val}
  'country:' || o.country_id::text || '|operator:' || plan.operator_id::text
    || '|sender_category:' || plan.sender_category AS scope_key,
  30 AS scope_priority,  -- operator + sender_category level
  plan.strategy,
  tp.start_date,
  tp.end_date,
  tp.created_at
FROM tariff_periods tp
JOIN tariff_plans plan ON plan.id = tp.tariff_plan_id
JOIN operators o ON o.id = plan.operator_id
ON CONFLICT DO NOTHING;

-- Migrate tiers: tariff_tiers → tariff_tiers_new (same period id, since we preserved it)
INSERT INTO tariff_tiers_new (id, tariff_period_id, from_count, price_per_segment, created_at)
SELECT t.id, t.tariff_period_id, t.from_count, t.price_per_segment, now()
FROM tariff_tiers t
WHERE EXISTS (SELECT 1 FROM tariff_periods_new WHERE id = t.tariff_period_id)
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: Write the down migration**

```sql
-- migrations/000082_migrate_tariff_plans_to_periods.down.sql
-- Remove rows that came from old tariff_plans (identified by scope_priority = 30 and operator_id IS NOT NULL)
DELETE FROM tariff_tiers_new
WHERE tariff_period_id IN (
  SELECT id FROM tariff_periods_new WHERE scope_priority = 30 AND operator_id IS NOT NULL
);
DELETE FROM tariff_periods_new
WHERE scope_priority = 30 AND operator_id IS NOT NULL;
```

- [ ] **Step 3: Apply and verify**

Run: `migrate -path migrations -database "$DATABASE_URL" up 1`

Then verify:
```sql
SELECT COUNT(*) FROM tariff_periods_new;
SELECT COUNT(*) FROM tariff_tiers_new;
-- Should match counts from old tariff_periods and tariff_tiers
SELECT COUNT(*) FROM tariff_periods;
SELECT COUNT(*) FROM tariff_tiers;
```

- [ ] **Step 4: Commit**

```bash
git add migrations/000082_migrate_tariff_plans_to_periods.up.sql \
        migrations/000082_migrate_tariff_plans_to_periods.down.sql
git commit -m "feat(db): migrate existing tariff_plans+periods into hierarchical tariff_periods_new"
```

---

### Task 7: Frontend — new types, API methods, and PeriodsTab rewrite

**Files:**
- Modify: `portal-frontend/src/api/admin.ts`
- Modify: `portal-frontend/src/pages/admin/tarification/PeriodsTab.tsx`

- [ ] **Step 1: Add new types and API methods to admin.ts**

Add these types alongside existing `TariffPeriod` (keep old interface for backward compat):

```typescript
// portal-frontend/src/api/admin.ts — add after existing TariffPeriod interface

export interface HierarchicalPeriod {
  id: string;
  country_id: string | null;
  operator_id: string | null;
  sender_category: string | null;
  traffic_type: string | null;
  client_id: string | null;
  scope_key: string;
  scope_priority: number;
  strategy: string;
  start_date: string; // YYYY-MM-DD
  end_date: string | null; // YYYY-MM-DD or null = open-ended
  created_at: string;
}

export interface AutoCloseWarning {
  period_id: string;
  new_end_date: string;
}

export interface CreateHierarchicalPeriodRequest {
  country_id?: string | null;
  operator_id?: string | null;
  sender_category?: string | null;
  traffic_type?: string | null;
  client_id?: string | null;
  strategy: string;
  start_date: string;
  end_date?: string | null;
}

export interface UpdateHierarchicalPeriodRequest {
  end_date?: string | null;
  strategy?: string;
}
```

Then add to the `tarificationApi` object:

```typescript
// Add to tarificationApi:
listPeriods: (params?: {
  country_id?: string;
  operator_id?: string;
  sender_category?: string;
  traffic_type?: string;
  client_id?: string;
}) =>
  adminFetch<{ periods: HierarchicalPeriod[]; total: number }>(
    `/tarification/periods${qs(params || {})}`,
  ),

createPeriod: (data: CreateHierarchicalPeriodRequest) =>
  adminFetch<{ period: HierarchicalPeriod; auto_close_warning?: AutoCloseWarning }>(
    '/tarification/periods',
    { method: 'POST', body: JSON.stringify(data) },
  ),

updatePeriod: (id: string, data: UpdateHierarchicalPeriodRequest) =>
  adminFetch<{ period: HierarchicalPeriod }>(
    `/tarification/periods/${id}`,
    { method: 'PUT', body: JSON.stringify(data) },
  ),

deletePeriod: (id: string) =>
  adminFetch<void>(`/tarification/periods/${id}`, { method: 'DELETE' }),
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd c:/projects/sms/portal-frontend && npx tsc --noEmit`
Expected: no type errors related to new additions.

- [ ] **Step 3: Rewrite PeriodsTab.tsx**

Replace the entire contents of `portal-frontend/src/pages/admin/tarification/PeriodsTab.tsx`:

```tsx
import { useState, useEffect, useCallback, useMemo } from 'react';
import { DataTable, type Column } from '../../../components/data/DataTable';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  tarificationApi,
  type HierarchicalPeriod,
  type CreateHierarchicalPeriodRequest,
} from '../../../api/admin';

const SENDER_CATEGORIES = [
  { value: 'shared', label: 'Shared' },
  { value: 'paid_registered', label: 'Paid Registered' },
  { value: 'free_registered', label: 'Free Registered' },
];

const TRAFFIC_TYPES = [
  { value: 'authorization', label: 'Authorization' },
  { value: 'transactional', label: 'Transactional' },
  { value: 'service', label: 'Service' },
  { value: 'extensible', label: 'Extensible' },
];

const STRATEGIES = [
  { value: 'fixed', label: 'Fixed' },
  { value: 'threshold', label: 'Threshold' },
  { value: 'threshold_recalc', label: 'Threshold Recalc' },
  { value: 'prepaid_threshold', label: 'Prepaid Threshold' },
];

function scopeLabel(p: HierarchicalPeriod): string {
  if (p.scope_key === 'global') return 'Global';
  const parts: string[] = [];
  if (p.country_id) parts.push('Country');
  if (p.operator_id) parts.push('Operator');
  if (p.sender_category) parts.push(p.sender_category);
  if (p.traffic_type) parts.push(p.traffic_type);
  if (p.client_id) parts.push('Client');
  return parts.join(' › ');
}

function computeStatus(p: HierarchicalPeriod): { label: string; variant: 'success' | 'warning' | 'default' } {
  const now = new Date();
  const start = new Date(p.start_date);
  if (start > now) return { label: 'Запланирован', variant: 'warning' };
  if (!p.end_date) return { label: 'Активен (open)', variant: 'success' };
  const end = new Date(p.end_date);
  if (end < now) return { label: 'Истёк', variant: 'default' };
  return { label: 'Активен', variant: 'success' };
}

interface FilterState {
  country_id: string;
  operator_id: string;
  sender_category: string;
  traffic_type: string;
  client_id: string;
}

interface FormState {
  country_id: string;
  operator_id: string;
  sender_category: string;
  traffic_type: string;
  client_id: string;
  strategy: string;
  start_date: string;
  end_date: string;
}

const emptyForm: FormState = {
  country_id: '',
  operator_id: '',
  sender_category: '',
  traffic_type: '',
  client_id: '',
  strategy: 'fixed',
  start_date: '',
  end_date: '',
};

export function PeriodsTab() {
  const toast = useToast();
  const [periods, setPeriods] = useState<HierarchicalPeriod[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<FilterState>({
    country_id: '', operator_id: '', sender_category: '', traffic_type: '', client_id: '',
  });
  const [showCreate, setShowCreate] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<FormState>(emptyForm);

  const fetchPeriods = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string> = {};
      if (filter.country_id) params.country_id = filter.country_id;
      if (filter.operator_id) params.operator_id = filter.operator_id;
      if (filter.sender_category) params.sender_category = filter.sender_category;
      if (filter.traffic_type) params.traffic_type = filter.traffic_type;
      if (filter.client_id) params.client_id = filter.client_id;
      const res = await tarificationApi.listPeriods(params);
      setPeriods(res.periods || []);
    } catch {
      toast.error('Не удалось загрузить периоды');
    } finally {
      setLoading(false);
    }
  }, [filter]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => { fetchPeriods(); }, [fetchPeriods]);

  const handleCreate = async () => {
    if (!form.strategy || !form.start_date) {
      toast.error('Strategy и Start date обязательны');
      return;
    }
    setSaving(true);
    try {
      const req: CreateHierarchicalPeriodRequest = {
        strategy: form.strategy,
        start_date: form.start_date,
        end_date: form.end_date || null,
        country_id: form.country_id || null,
        operator_id: form.operator_id || null,
        sender_category: form.sender_category || null,
        traffic_type: form.traffic_type || null,
        client_id: form.client_id || null,
      };
      const res = await tarificationApi.createPeriod(req);
      if (res.auto_close_warning) {
        toast.success(`Период создан. Предыдущий период закрыт: ${res.auto_close_warning.new_end_date}`);
      } else {
        toast.success('Период создан');
      }
      setShowCreate(false);
      setForm(emptyForm);
      fetchPeriods();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка создания');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Удалить период? Действие необратимо.')) return;
    try {
      await tarificationApi.deletePeriod(id);
      toast.success('Период удалён');
      fetchPeriods();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка удаления');
    }
  };

  const columns: Column<HierarchicalPeriod>[] = useMemo(() => [
    {
      key: 'scope',
      header: 'Scope',
      render: (p) => (
        <div>
          <span className="font-medium">{scopeLabel(p)}</span>
          <Badge variant="default" className="ml-2 text-xs">{p.scope_priority}</Badge>
        </div>
      ),
    },
    { key: 'strategy', header: 'Strategy', render: (p) => p.strategy },
    {
      key: 'start_date',
      header: 'Начало',
      render: (p) => new Date(p.start_date).toLocaleDateString('ru-RU'),
      sortable: true,
    },
    {
      key: 'end_date',
      header: 'Окончание',
      render: (p) => p.end_date ? new Date(p.end_date).toLocaleDateString('ru-RU') : '—',
    },
    {
      key: 'status',
      header: 'Статус',
      render: (p) => {
        const s = computeStatus(p);
        return <Badge variant={s.variant}>{s.label}</Badge>;
      },
    },
    {
      key: 'actions',
      header: '',
      render: (p) => (
        <Button size="sm" variant="destructive" onClick={() => handleDelete(p.id)}>
          Удалить
        </Button>
      ),
    },
  ], []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Периоды тарификации (иерархические)</h2>
        <Button size="sm" onClick={() => { setForm(emptyForm); setShowCreate(true); }}>
          Создать период
        </Button>
      </div>

      {/* Filters */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-2 mb-4">
        <Input
          placeholder="Country ID"
          value={filter.country_id}
          onChange={(e) => setFilter({ ...filter, country_id: e.target.value })}
        />
        <Input
          placeholder="Operator ID"
          value={filter.operator_id}
          onChange={(e) => setFilter({ ...filter, operator_id: e.target.value })}
        />
        <Select
          options={[{ value: '', label: 'Все категории' }, ...SENDER_CATEGORIES]}
          value={filter.sender_category}
          onChange={(v) => setFilter({ ...filter, sender_category: v })}
        />
        <Select
          options={[{ value: '', label: 'Все типы трафика' }, ...TRAFFIC_TYPES]}
          value={filter.traffic_type}
          onChange={(v) => setFilter({ ...filter, traffic_type: v })}
        />
        <Input
          placeholder="Client ID"
          value={filter.client_id}
          onChange={(e) => setFilter({ ...filter, client_id: e.target.value })}
        />
      </div>
      <div className="mb-4">
        <Button variant="secondary" size="sm" onClick={() => setFilter({ country_id: '', operator_id: '', sender_category: '', traffic_type: '', client_id: '' })}>
          Сбросить фильтры
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={periods}
        total={periods.length}
        page={1}
        pageSize={200}
        onPageChange={() => {}}
        loading={loading}
        keyField="id"
      />

      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Создать период">
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Заполните только нужные dimension-поля. Незаполненные = «любой». Соблюдайте иерархию:
            для Operator нужен Country; для Sender category нужен Operator.
          </p>

          <Input
            label="Country ID (UUID, опционально)"
            value={form.country_id}
            onChange={(e) => setForm({ ...form, country_id: e.target.value })}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
          />
          <Input
            label="Operator ID (UUID, опционально)"
            value={form.operator_id}
            onChange={(e) => setForm({ ...form, operator_id: e.target.value })}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
          />
          <Select
            label="Sender Category (опционально)"
            options={[{ value: '', label: 'Не указано' }, ...SENDER_CATEGORIES]}
            value={form.sender_category}
            onChange={(v) => setForm({ ...form, sender_category: v })}
          />
          <Select
            label="Traffic Type (опционально)"
            options={[{ value: '', label: 'Не указано' }, ...TRAFFIC_TYPES]}
            value={form.traffic_type}
            onChange={(v) => setForm({ ...form, traffic_type: v })}
          />
          <Input
            label="Client ID (UUID, опционально — клиентский override)"
            value={form.client_id}
            onChange={(e) => setForm({ ...form, client_id: e.target.value })}
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
          />

          <Select
            label="Strategy *"
            options={STRATEGIES}
            value={form.strategy}
            onChange={(v) => setForm({ ...form, strategy: v })}
          />
          <Input
            label="Дата начала *"
            type="date"
            value={form.start_date}
            onChange={(e) => setForm({ ...form, start_date: e.target.value })}
            required
          />
          <Input
            label="Дата окончания (пусто = бессрочно)"
            type="date"
            value={form.end_date}
            onChange={(e) => setForm({ ...form, end_date: e.target.value })}
          />

          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreate(false)}>Отмена</Button>
            <Button onClick={handleCreate} disabled={saving || !form.strategy || !form.start_date}>
              {saving ? 'Создание...' : 'Создать'}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
```

- [ ] **Step 4: Verify TypeScript compiles**

Run: `cd c:/projects/sms/portal-frontend && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 5: Verify frontend builds**

Run: `cd c:/projects/sms/portal-frontend && npm run build`
Expected: build succeeds with no errors.

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/api/admin.ts \
        portal-frontend/src/pages/admin/tarification/PeriodsTab.tsx
git commit -m "feat(portal): add hierarchical periods UI with dimension filters and auto-close warning"
```

---

### Task 8: End-to-end smoke test

- [ ] **Step 1: Build all Go services**

Run: `cd c:/projects/sms && go build ./...`
Expected: no errors.

- [ ] **Step 2: Run existing tarification tests**

Run: `cd c:/projects/sms && go test ./internal/services/tarification/... -v -count=1`
Expected: all tests pass.

- [ ] **Step 3: Run admin gateway tests**

Run: `cd c:/projects/sms && go test ./internal/gateway/admin/... -v -count=1`
Expected: all tests pass (including new period_service_test.go).

- [ ] **Step 4: Apply both migrations on dev server (if available)**

```bash
scripts/server.sh migrate
```
Expected: migrations 000081 and 000082 applied cleanly.

- [ ] **Step 5: Verify API via curl**

```bash
# List periods (should return migrated data)
curl -s http://localhost:8080/admin/v1/tarification/periods | jq '.total'

# Create a global period
curl -s -X POST http://localhost:8080/admin/v1/tarification/periods \
  -H 'Content-Type: application/json' \
  -d '{"strategy":"fixed","start_date":"2026-05-01"}' | jq '.period.scope_key'
# Expected: "global"
```

- [ ] **Step 6: Final commit**

```bash
git add .
git commit -m "feat: hierarchical tariff periods — end-to-end verified"
```

---

## Self-Review Against Spec

| Spec requirement | Task |
|-----------------|------|
| tariff_periods table with all dimension columns | Task 1 |
| scope_key computation (alphabetical canonical) | Task 2 |
| scope_priority computation (base + client +100) | Task 2 |
| Strict hierarchy validation (operator→country etc.) | Task 2 |
| tariff_tiers_new table | Task 1 |
| provider_cost_periods + provider_cost_tiers | Task 1 |
| Auto-close previous open-ended period | Task 2 (CreatePeriod) |
| Containment validation (new.start >= parent.start) | Task 2 (CreatePeriod) |
| Child protection when closing parent | Task 2 (assertNoActiveChildren) |
| No retroactive periods | Task 2 (ErrRetroactiveStart) |
| Overlap exclusion constraint | Task 1 (EXCLUDE USING gist) |
| REST CRUD for /tarification/periods | Tasks 3, 4 |
| Limited PUT (end_date + strategy only) | Task 3 (UpdatePeriod) |
| DELETE only if no children | Task 2 (DeletePeriod) |
| Tariff lookup algorithm (hierarchical) | Task 5 |
| Tier inheritance (walk down if no tiers) | Task 5 (FindTiersWithFallback) |
| Data migration from old model | Task 6 |
| UI: dimension filters | Task 7 (PeriodsTab filters) |
| UI: scope label + priority badge | Task 7 (scopeLabel, Badge) |
| UI: auto-close warning on create | Task 7 (handleCreate shows warning) |
| UI: open-ended status display | Task 7 (computeStatus) |

**Gaps / deferred:**
- Redis caching for tariff lookup (spec §Caching) — add as follow-up after lookup is wired in
- provider_cost_periods CRUD API and UI — analogous to periods, add as follow-up
- Dropping old `tariff_plans` / `tariff_periods` tables — migration 000083, after verifying TarifyMessage uses new path
- Searchable country/operator dropdowns in create modal (spec §UI) — follow-up UX improvement
- `traffic_type` field in `TarifyMessageRequest` proto — required to use traffic_type dimension in lookup
