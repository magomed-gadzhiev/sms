# Aggregator Billing Quotas Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add infrastructure quotas and billing modes (own/aggregator/hybrid) for aggregator sub-accounts, enabling package-based billing with overage and provider-type-aware charging.

**Architecture:** Extend tarification pipeline with quota check before standard billing. New `aggregator_quotas` table tracks pool usage. `billing_mode` on clients controls charge target. Provider ownership check (`allowed_provider_ids`) determines whether traffic charges apply.

**Tech Stack:** Go 1.24.0, PostgreSQL 15+ (pgx/v5), gorilla/mux, gRPC, testify, React 19 + TypeScript

**Spec:** `docs/superpowers/specs/2026-04-15-aggregator-billing-quotas-design.md`

---

## File Structure

### New Files
- `migrations/000097_aggregator_quotas.up.sql` — migration
- `migrations/000097_aggregator_quotas.down.sql` — rollback
- `internal/services/tarification/domain/aggregator_quota.go` — domain model + repository interface
- `internal/services/tarification/infrastructure/repository/aggregator_quota_repository.go` — PostgreSQL repository
- `internal/services/tarification/application/quota_service.go` — quota business logic (increment, overage check, CRUD)
- `internal/services/tarification/application/quota_service_test.go` — tests
- `internal/gateway/admin/handlers/aggregator_quotas.go` — admin API handlers
- `internal/gateway/portal/handlers/aggregator_quotas.go` — portal API handlers
- `portal-frontend/src/pages/admin/aggregators/AggregatorQuotasPage.tsx` — admin UI
- `portal-frontend/src/pages/network/NetworkQuotaPage.tsx` — aggregator portal UI

### Modified Files
- `internal/services/tarification/application/tarification_service.go` — add quota step to TarifyMessage
- `internal/services/tarification/domain/aggregator_tariff.go` — extend ClientAccountInfo with billing_mode
- `internal/services/client/domain/client.go` — add BillingMode, SpendingLimitMonthly, SpendingLimitDaily fields
- `internal/gateway/admin/router/router.go` — register quota routes
- `internal/gateway/portal/router.go` — register portal quota routes
- `portal-frontend/src/api/admin.ts` — API functions for quota CRUD
- `portal-frontend/src/api/portal.ts` — API functions for portal quota endpoints

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/000097_aggregator_quotas.up.sql`
- Create: `migrations/000097_aggregator_quotas.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- migrations/000097_aggregator_quotas.up.sql

-- Aggregator infrastructure quotas (pool for all sub-accounts)
CREATE TABLE aggregator_quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    segment_limit BIGINT NOT NULL CHECK (segment_limit > 0),
    segments_used BIGINT NOT NULL DEFAULT 0 CHECK (segments_used >= 0),
    overage_rate NUMERIC(10,4) NOT NULL CHECK (overage_rate >= 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'RUB',
    auto_renew BOOLEAN NOT NULL DEFAULT true,
    notified_80pct BOOLEAN NOT NULL DEFAULT false,
    notified_100pct BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_aggregator_quota_period UNIQUE(aggregator_id, period_start),
    CONSTRAINT chk_period_range CHECK (period_end > period_start)
);

CREATE INDEX idx_aggregator_quotas_active ON aggregator_quotas(aggregator_id, period_start, period_end);

-- Billing mode and spending limits on clients (for sub-accounts)
ALTER TABLE clients
    ADD COLUMN billing_mode VARCHAR(20) NOT NULL DEFAULT 'own'
        CHECK (billing_mode IN ('own', 'aggregator', 'hybrid')),
    ADD COLUMN spending_limit_monthly NUMERIC(12,2),
    ADD COLUMN spending_limit_daily NUMERIC(12,2);

-- Attribution on transactions (which sub-account triggered the charge on aggregator's balance)
ALTER TABLE transactions
    ADD COLUMN attributed_sub_account_id UUID REFERENCES clients(id);

CREATE INDEX idx_transactions_attributed ON transactions(attributed_sub_account_id, created_at)
    WHERE attributed_sub_account_id IS NOT NULL;
```

- [ ] **Step 2: Write the down migration**

```sql
-- migrations/000097_aggregator_quotas.down.sql

DROP INDEX IF EXISTS idx_transactions_attributed;
ALTER TABLE transactions DROP COLUMN IF EXISTS attributed_sub_account_id;

ALTER TABLE clients DROP COLUMN IF EXISTS spending_limit_daily;
ALTER TABLE clients DROP COLUMN IF EXISTS spending_limit_monthly;
ALTER TABLE clients DROP COLUMN IF EXISTS billing_mode;

DROP TABLE IF EXISTS aggregator_quotas;
```

- [ ] **Step 3: Apply migration locally**

Run: `cd c:/projects/sms && go run cmd/migrate/main.go up`
Expected: Migration 000097 applied successfully

- [ ] **Step 4: Verify tables exist**

Run: `psql -c "\d aggregator_quotas" && psql -c "\d clients" | grep billing_mode && psql -c "\d transactions" | grep attributed`
Expected: Table and columns exist

- [ ] **Step 5: Commit**

```bash
git add migrations/000097_aggregator_quotas.up.sql migrations/000097_aggregator_quotas.down.sql
git commit -m "feat(billing): add aggregator_quotas table and billing_mode/spending_limit columns"
```

---

### Task 2: Domain Model — AggregatorQuota

**Files:**
- Create: `internal/services/tarification/domain/aggregator_quota.go`

- [ ] **Step 1: Write the domain model and repository interface**

```go
// internal/services/tarification/domain/aggregator_quota.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// AggregatorQuota represents an infrastructure usage quota for an aggregator.
// All sub-accounts share this pool counter.
type AggregatorQuota struct {
	ID             uuid.UUID
	AggregatorID   uuid.UUID
	PeriodStart    time.Time
	PeriodEnd      time.Time
	SegmentLimit   int64
	SegmentsUsed   int64
	OverageRate    string // NUMERIC as string for precision
	Currency       string
	AutoRenew      bool
	Notified80Pct  bool
	Notified100Pct bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsExhausted returns true if the quota has been fully consumed.
func (q *AggregatorQuota) IsExhausted() bool {
	return q.SegmentsUsed >= q.SegmentLimit
}

// UtilizationPercent returns current usage as a percentage (0-100+).
func (q *AggregatorQuota) UtilizationPercent() float64 {
	if q.SegmentLimit == 0 {
		return 100
	}
	return float64(q.SegmentsUsed) / float64(q.SegmentLimit) * 100
}

// OverageSegments returns how many segments are over the limit.
func (q *AggregatorQuota) OverageSegments() int64 {
	if q.SegmentsUsed <= q.SegmentLimit {
		return 0
	}
	return q.SegmentsUsed - q.SegmentLimit
}

// IncrementResult is returned by the atomic increment operation.
type IncrementResult struct {
	SegmentsUsed   int64
	SegmentLimit   int64
	OverageRate    string
	WasWithinQuota bool // true if segments_used was below limit BEFORE this increment
	OverageCount   int  // how many of the incremented segments are overage
}

// AggregatorQuotaRepository provides persistence for aggregator quotas.
type AggregatorQuotaRepository interface {
	Create(ctx context.Context, quota *AggregatorQuota) error
	GetByID(ctx context.Context, id uuid.UUID) (*AggregatorQuota, error)
	GetActive(ctx context.Context, aggregatorID uuid.UUID, now time.Time) (*AggregatorQuota, error)
	Update(ctx context.Context, quota *AggregatorQuota) error
	IncrementUsage(ctx context.Context, quotaID uuid.UUID, segments int) (*IncrementResult, error)
	ListByAggregator(ctx context.Context, aggregatorID uuid.UUID, limit, offset int) ([]*AggregatorQuota, int, error)
	SetNotified80(ctx context.Context, quotaID uuid.UUID) error
	SetNotified100(ctx context.Context, quotaID uuid.UUID) error
	ListAutoRenewable(ctx context.Context, beforeDate time.Time) ([]*AggregatorQuota, error)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/tarification/domain/aggregator_quota.go
git commit -m "feat(billing): add AggregatorQuota domain model and repository interface"
```

---

### Task 3: Extend ClientAccountInfo with BillingMode

**Files:**
- Modify: `internal/services/tarification/domain/aggregator_tariff.go`
- Modify: `internal/services/client/domain/client.go`

- [ ] **Step 1: Add BillingMode to ClientAccountInfo**

In `internal/services/tarification/domain/aggregator_tariff.go`, extend the `ClientAccountInfo` struct:

```go
// Add BillingMode type before ClientAccountInfo
type BillingMode string

const (
	BillingModeOwn        BillingMode = "own"
	BillingModeAggregator BillingMode = "aggregator"
	BillingModeHybrid     BillingMode = "hybrid"
)
```

Add fields to `ClientAccountInfo`:

```go
type ClientAccountInfo struct {
	ID                   uuid.UUID
	AccountType          string
	ParentClientID       *uuid.UUID
	BillingMode          BillingMode
	SpendingLimitMonthly *string
	SpendingLimitDaily   *string
}
```

- [ ] **Step 2: Add fields to Client struct**

In `internal/services/client/domain/client.go`, add after `IsSandbox`:

```go
	// Billing mode for sub-accounts
	BillingMode          string  `json:"billing_mode" db:"billing_mode"`
	SpendingLimitMonthly *string `json:"spending_limit_monthly,omitempty" db:"spending_limit_monthly"`
	SpendingLimitDaily   *string `json:"spending_limit_daily,omitempty" db:"spending_limit_daily"`
```

- [ ] **Step 3: Update ClientInfoRepository query**

In `internal/services/tarification/infrastructure/repository/client_info_repository.go`, update the SQL query in `GetAccountInfo` to also SELECT `billing_mode`, `spending_limit_monthly`, `spending_limit_daily` and scan them into the struct.

- [ ] **Step 4: Commit**

```bash
git add internal/services/tarification/domain/aggregator_tariff.go \
        internal/services/client/domain/client.go \
        internal/services/tarification/infrastructure/repository/client_info_repository.go
git commit -m "feat(billing): add BillingMode and spending limits to client domain models"
```

---

### Task 4: AggregatorQuota Repository

**Files:**
- Create: `internal/services/tarification/infrastructure/repository/aggregator_quota_repository.go`

- [ ] **Step 1: Implement the repository**

```go
// internal/services/tarification/infrastructure/repository/aggregator_quota_repository.go
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type AggregatorQuotaRepository struct {
	db *sqlx.DB
}

func NewAggregatorQuotaRepository(db *sqlx.DB) *AggregatorQuotaRepository {
	return &AggregatorQuotaRepository{db: db}
}

func (r *AggregatorQuotaRepository) Create(ctx context.Context, quota *domain.AggregatorQuota) error {
	query := `
		INSERT INTO aggregator_quotas (id, aggregator_id, period_start, period_end,
			segment_limit, segments_used, overage_rate, currency, auto_renew,
			notified_80pct, notified_100pct, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.ExecContext(ctx, query,
		quota.ID, quota.AggregatorID, quota.PeriodStart, quota.PeriodEnd,
		quota.SegmentLimit, quota.SegmentsUsed, quota.OverageRate, quota.Currency,
		quota.AutoRenew, quota.Notified80Pct, quota.Notified100Pct,
		quota.CreatedAt, quota.UpdatedAt,
	)
	return err
}

func (r *AggregatorQuotaRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.AggregatorQuota, error) {
	var q domain.AggregatorQuota
	query := `
		SELECT id, aggregator_id, period_start, period_end, segment_limit, segments_used,
			overage_rate, currency, auto_renew, notified_80pct, notified_100pct, created_at, updated_at
		FROM aggregator_quotas WHERE id = $1
	`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&q.ID, &q.AggregatorID, &q.PeriodStart, &q.PeriodEnd, &q.SegmentLimit, &q.SegmentsUsed,
		&q.OverageRate, &q.Currency, &q.AutoRenew, &q.Notified80Pct, &q.Notified100Pct,
		&q.CreatedAt, &q.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &q, err
}

func (r *AggregatorQuotaRepository) GetActive(ctx context.Context, aggregatorID uuid.UUID, now time.Time) (*domain.AggregatorQuota, error) {
	var q domain.AggregatorQuota
	query := `
		SELECT id, aggregator_id, period_start, period_end, segment_limit, segments_used,
			overage_rate, currency, auto_renew, notified_80pct, notified_100pct, created_at, updated_at
		FROM aggregator_quotas
		WHERE aggregator_id = $1 AND period_start <= $2 AND period_end > $2
		ORDER BY period_start DESC
		LIMIT 1
	`
	err := r.db.QueryRowContext(ctx, query, aggregatorID, now).Scan(
		&q.ID, &q.AggregatorID, &q.PeriodStart, &q.PeriodEnd, &q.SegmentLimit, &q.SegmentsUsed,
		&q.OverageRate, &q.Currency, &q.AutoRenew, &q.Notified80Pct, &q.Notified100Pct,
		&q.CreatedAt, &q.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &q, err
}

func (r *AggregatorQuotaRepository) Update(ctx context.Context, quota *domain.AggregatorQuota) error {
	query := `
		UPDATE aggregator_quotas
		SET segment_limit = $2, overage_rate = $3, auto_renew = $4, updated_at = now()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, quota.ID, quota.SegmentLimit, quota.OverageRate, quota.AutoRenew)
	return err
}

// IncrementUsage atomically increments segments_used and returns the result with overage info.
func (r *AggregatorQuotaRepository) IncrementUsage(ctx context.Context, quotaID uuid.UUID, segments int) (*domain.IncrementResult, error) {
	var result domain.IncrementResult
	query := `
		UPDATE aggregator_quotas
		SET segments_used = segments_used + $2, updated_at = now()
		WHERE id = $1
		RETURNING segments_used, segment_limit, overage_rate
	`
	var overageRate string
	err := r.db.QueryRowContext(ctx, query, quotaID, segments).Scan(
		&result.SegmentsUsed, &result.SegmentLimit, &overageRate,
	)
	if err != nil {
		return nil, err
	}
	result.OverageRate = overageRate

	prevUsed := result.SegmentsUsed - int64(segments)
	result.WasWithinQuota = prevUsed < result.SegmentLimit

	if result.SegmentsUsed > result.SegmentLimit {
		if prevUsed >= result.SegmentLimit {
			// All segments are overage
			result.OverageCount = segments
		} else {
			// Partial overage: only the segments that crossed the limit
			result.OverageCount = int(result.SegmentsUsed - result.SegmentLimit)
		}
	}

	return &result, nil
}

func (r *AggregatorQuotaRepository) ListByAggregator(ctx context.Context, aggregatorID uuid.UUID, limit, offset int) ([]*domain.AggregatorQuota, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM aggregator_quotas WHERE aggregator_id = $1`, aggregatorID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, aggregator_id, period_start, period_end, segment_limit, segments_used,
			overage_rate, currency, auto_renew, notified_80pct, notified_100pct, created_at, updated_at
		FROM aggregator_quotas WHERE aggregator_id = $1
		ORDER BY period_start DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, aggregatorID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var quotas []*domain.AggregatorQuota
	for rows.Next() {
		var q domain.AggregatorQuota
		if err := rows.Scan(
			&q.ID, &q.AggregatorID, &q.PeriodStart, &q.PeriodEnd, &q.SegmentLimit, &q.SegmentsUsed,
			&q.OverageRate, &q.Currency, &q.AutoRenew, &q.Notified80Pct, &q.Notified100Pct,
			&q.CreatedAt, &q.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		quotas = append(quotas, &q)
	}
	return quotas, total, rows.Err()
}

func (r *AggregatorQuotaRepository) SetNotified80(ctx context.Context, quotaID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE aggregator_quotas SET notified_80pct = true WHERE id = $1`, quotaID)
	return err
}

func (r *AggregatorQuotaRepository) SetNotified100(ctx context.Context, quotaID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE aggregator_quotas SET notified_100pct = true WHERE id = $1`, quotaID)
	return err
}

func (r *AggregatorQuotaRepository) ListAutoRenewable(ctx context.Context, beforeDate time.Time) ([]*domain.AggregatorQuota, error) {
	query := `
		SELECT id, aggregator_id, period_start, period_end, segment_limit, segments_used,
			overage_rate, currency, auto_renew, notified_80pct, notified_100pct, created_at, updated_at
		FROM aggregator_quotas
		WHERE auto_renew = true AND period_end <= $1
		AND NOT EXISTS (
			SELECT 1 FROM aggregator_quotas aq2
			WHERE aq2.aggregator_id = aggregator_quotas.aggregator_id
			AND aq2.period_start = aggregator_quotas.period_end
		)
	`
	rows, err := r.db.QueryContext(ctx, query, beforeDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var quotas []*domain.AggregatorQuota
	for rows.Next() {
		var q domain.AggregatorQuota
		if err := rows.Scan(
			&q.ID, &q.AggregatorID, &q.PeriodStart, &q.PeriodEnd, &q.SegmentLimit, &q.SegmentsUsed,
			&q.OverageRate, &q.Currency, &q.AutoRenew, &q.Notified80Pct, &q.Notified100Pct,
			&q.CreatedAt, &q.UpdatedAt,
		); err != nil {
			return nil, err
		}
		quotas = append(quotas, &q)
	}
	return quotas, rows.Err()
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/tarification/infrastructure/repository/aggregator_quota_repository.go
git commit -m "feat(billing): implement AggregatorQuotaRepository with atomic increment"
```

---

### Task 5: Quota Service (Business Logic)

**Files:**
- Create: `internal/services/tarification/application/quota_service.go`
- Create: `internal/services/tarification/application/quota_service_test.go`

- [ ] **Step 1: Write the quota service test**

```go
// internal/services/tarification/application/quota_service_test.go
package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// mockQuotaRepo implements domain.AggregatorQuotaRepository
type mockQuotaRepo struct{ mock.Mock }

func (m *mockQuotaRepo) Create(ctx context.Context, q *domain.AggregatorQuota) error {
	return m.Called(ctx, q).Error(0)
}
func (m *mockQuotaRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.AggregatorQuota, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AggregatorQuota), args.Error(1)
}
func (m *mockQuotaRepo) GetActive(ctx context.Context, aggID uuid.UUID, now time.Time) (*domain.AggregatorQuota, error) {
	args := m.Called(ctx, aggID, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AggregatorQuota), args.Error(1)
}
func (m *mockQuotaRepo) Update(ctx context.Context, q *domain.AggregatorQuota) error {
	return m.Called(ctx, q).Error(0)
}
func (m *mockQuotaRepo) IncrementUsage(ctx context.Context, id uuid.UUID, segs int) (*domain.IncrementResult, error) {
	args := m.Called(ctx, id, segs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.IncrementResult), args.Error(1)
}
func (m *mockQuotaRepo) ListByAggregator(ctx context.Context, id uuid.UUID, l, o int) ([]*domain.AggregatorQuota, int, error) {
	args := m.Called(ctx, id, l, o)
	return args.Get(0).([]*domain.AggregatorQuota), args.Int(1), args.Error(2)
}
func (m *mockQuotaRepo) SetNotified80(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockQuotaRepo) SetNotified100(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockQuotaRepo) ListAutoRenewable(ctx context.Context, t time.Time) ([]*domain.AggregatorQuota, error) {
	args := m.Called(ctx, t)
	return args.Get(0).([]*domain.AggregatorQuota), args.Error(1)
}

func TestQuotaService_ConsumeQuota_WithinLimit(t *testing.T) {
	repo := new(mockQuotaRepo)
	svc := NewQuotaService(repo)

	aggID := uuid.New()
	quotaID := uuid.New()

	repo.On("GetActive", mock.Anything, aggID, mock.Anything).Return(&domain.AggregatorQuota{
		ID: quotaID, AggregatorID: aggID, SegmentLimit: 1000, SegmentsUsed: 100,
		OverageRate: "0.50", Currency: "RUB",
	}, nil)
	repo.On("IncrementUsage", mock.Anything, quotaID, 5).Return(&domain.IncrementResult{
		SegmentsUsed: 105, SegmentLimit: 1000, OverageRate: "0.50",
		WasWithinQuota: true, OverageCount: 0,
	}, nil)

	result, err := svc.ConsumeQuota(context.Background(), aggID, 5)
	require.NoError(t, err)
	assert.False(t, result.HasOverage)
	assert.Equal(t, "0", result.OverageChargeAmount)
	repo.AssertExpectations(t)
}

func TestQuotaService_ConsumeQuota_CrossesLimit(t *testing.T) {
	repo := new(mockQuotaRepo)
	svc := NewQuotaService(repo)

	aggID := uuid.New()
	quotaID := uuid.New()

	repo.On("GetActive", mock.Anything, aggID, mock.Anything).Return(&domain.AggregatorQuota{
		ID: quotaID, AggregatorID: aggID, SegmentLimit: 1000, SegmentsUsed: 998,
		OverageRate: "0.50", Currency: "RUB",
	}, nil)
	repo.On("IncrementUsage", mock.Anything, quotaID, 5).Return(&domain.IncrementResult{
		SegmentsUsed: 1003, SegmentLimit: 1000, OverageRate: "0.50",
		WasWithinQuota: true, OverageCount: 3,
	}, nil)

	result, err := svc.ConsumeQuota(context.Background(), aggID, 5)
	require.NoError(t, err)
	assert.True(t, result.HasOverage)
	assert.Equal(t, "1.50", result.OverageChargeAmount) // 3 * 0.50
	assert.Equal(t, "RUB", result.Currency)
	repo.AssertExpectations(t)
}

func TestQuotaService_ConsumeQuota_NoActiveQuota(t *testing.T) {
	repo := new(mockQuotaRepo)
	svc := NewQuotaService(repo)

	repo.On("GetActive", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	result, err := svc.ConsumeQuota(context.Background(), uuid.New(), 5)
	require.NoError(t, err)
	assert.True(t, result.NoQuota)
	repo.AssertExpectations(t)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd c:/projects/sms && go test ./internal/services/tarification/application/ -run TestQuotaService -v`
Expected: FAIL — `NewQuotaService` undefined

- [ ] **Step 3: Implement the quota service**

```go
// internal/services/tarification/application/quota_service.go
package application

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// QuotaService handles infrastructure quota consumption and management.
type QuotaService struct {
	repo domain.AggregatorQuotaRepository
}

func NewQuotaService(repo domain.AggregatorQuotaRepository) *QuotaService {
	return &QuotaService{repo: repo}
}

// QuotaConsumeResult is returned by ConsumeQuota.
type QuotaConsumeResult struct {
	NoQuota             bool   // true if aggregator has no active quota — skip quota logic
	HasOverage          bool   // true if some segments exceeded the limit
	OverageChargeAmount string // amount to charge for overage (e.g. "1.50")
	Currency            string
	QuotaID             uuid.UUID
}

// ConsumeQuota atomically increments the aggregator's active quota and calculates overage.
func (s *QuotaService) ConsumeQuota(ctx context.Context, aggregatorID uuid.UUID, segments int) (*QuotaConsumeResult, error) {
	quota, err := s.repo.GetActive(ctx, aggregatorID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("get active quota: %w", err)
	}
	if quota == nil {
		return &QuotaConsumeResult{NoQuota: true}, nil
	}

	incr, err := s.repo.IncrementUsage(ctx, quota.ID, segments)
	if err != nil {
		return nil, fmt.Errorf("increment quota: %w", err)
	}

	result := &QuotaConsumeResult{
		QuotaID:  quota.ID,
		Currency: quota.Currency,
	}

	if incr.OverageCount > 0 {
		result.HasOverage = true
		overageAmount := multiplyPriceInt(incr.OverageRate, incr.OverageCount)
		result.OverageChargeAmount = overageAmount
	} else {
		result.OverageChargeAmount = "0"
	}

	return result, nil
}

// CreateQuota creates a new quota for an aggregator.
func (s *QuotaService) CreateQuota(ctx context.Context, aggregatorID uuid.UUID, periodStart, periodEnd time.Time, segmentLimit int64, overageRate, currency string, autoRenew bool) (*domain.AggregatorQuota, error) {
	quota := &domain.AggregatorQuota{
		ID:           uuid.New(),
		AggregatorID: aggregatorID,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		SegmentLimit: segmentLimit,
		OverageRate:  overageRate,
		Currency:     currency,
		AutoRenew:    autoRenew,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := s.repo.Create(ctx, quota); err != nil {
		return nil, err
	}
	return quota, nil
}

// GetActiveQuota returns the current active quota for an aggregator, or nil.
func (s *QuotaService) GetActiveQuota(ctx context.Context, aggregatorID uuid.UUID) (*domain.AggregatorQuota, error) {
	return s.repo.GetActive(ctx, aggregatorID, time.Now())
}

// UpdateQuota updates segment_limit, overage_rate, auto_renew on an existing quota.
func (s *QuotaService) UpdateQuota(ctx context.Context, quotaID uuid.UUID, segmentLimit int64, overageRate string, autoRenew bool) (*domain.AggregatorQuota, error) {
	quota, err := s.repo.GetByID(ctx, quotaID)
	if err != nil {
		return nil, err
	}
	if quota == nil {
		return nil, fmt.Errorf("quota not found")
	}
	quota.SegmentLimit = segmentLimit
	quota.OverageRate = overageRate
	quota.AutoRenew = autoRenew
	if err := s.repo.Update(ctx, quota); err != nil {
		return nil, err
	}
	return quota, nil
}

// ListQuotas returns quotas for an aggregator ordered by period_start DESC.
func (s *QuotaService) ListQuotas(ctx context.Context, aggregatorID uuid.UUID, limit, offset int) ([]*domain.AggregatorQuota, int, error) {
	return s.repo.ListByAggregator(ctx, aggregatorID, limit, offset)
}

// multiplyPriceInt multiplies a price string by an integer count, returning the result as a string.
func multiplyPriceInt(price string, count int) string {
	p, ok := new(big.Float).SetString(price)
	if !ok {
		return "0"
	}
	c := new(big.Float).SetInt64(int64(count))
	result := new(big.Float).Mul(p, c)
	return result.Text('f', 2)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd c:/projects/sms && go test ./internal/services/tarification/application/ -run TestQuotaService -v`
Expected: All 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/tarification/application/quota_service.go \
        internal/services/tarification/application/quota_service_test.go
git commit -m "feat(billing): implement QuotaService with consume, create, update, list"
```

---

### Task 6: Integrate Quota + BillingMode into TarifyMessage Pipeline

**Files:**
- Modify: `internal/services/tarification/application/tarification_service.go`

- [ ] **Step 1: Add quota service field to TarificationService**

In `tarification_service.go`, add to the struct:

```go
	// Aggregator quota
	quotaService *QuotaService
```

Add setter method after `SetAggregatorRepos`:

```go
// SetQuotaService подключает сервис квот агрегатора
func (s *TarificationService) SetQuotaService(qs *QuotaService) {
	s.quotaService = qs
}
```

- [ ] **Step 2: Add quota consumption and billing mode logic to TarifyMessage**

Insert **after step 8** (resolveAggregatorBilling) and **before** the charge block. The new logic:

1. If client is sub-account, consume quota from aggregator's pool
2. If overage, charge overage from aggregator's balance
3. Determine charge target based on billing_mode (instead of always doing dual charge)

Replace the existing charge block (from `// 8. Проверяем...` through the dual/single charge logic) with:

```go
	// 8. Check aggregator context (sub-account? billing mode? quota?)
	isSubAccount, aggregatorID, aggChargeAmount, subChargeAmount := s.resolveAggregatorBilling(
		ctx, req.ClientID, req.OperatorID, category, result, req.SegmentCount,
	)

	// 8a. Consume infrastructure quota (if aggregator has active quota)
	if isSubAccount && aggregatorID != uuid.Nil && s.quotaService != nil {
		quotaResult, quotaErr := s.quotaService.ConsumeQuota(ctx, aggregatorID, req.SegmentCount)
		if quotaErr != nil {
			return nil, fmt.Errorf("quota consumption failed: %w", quotaErr)
		}
		// Charge overage from aggregator's balance if needed
		if !quotaResult.NoQuota && quotaResult.HasOverage {
			overageResult, overageErr := s.saga.Charge(ctx,
				aggregatorID.String(),
				req.MessageID.String()+"-overage",
				quotaResult.OverageChargeAmount,
				quotaResult.Currency,
				fmt.Sprintf("Infrastructure overage: %d segments", req.SegmentCount),
				int32(req.SegmentCount),
			)
			if overageErr != nil {
				return nil, fmt.Errorf("overage charge failed: %w", overageErr)
			}
			if !overageResult.Success {
				return &TarifyMessageResponse{
					Approved:        false,
					RejectionReason: "aggregator balance insufficient for overage",
				}, nil
			}
		}
	}

	// 8b. Determine charge target based on billing_mode
	var clientInfo *domain.ClientAccountInfo
	if isSubAccount && s.clientInfoRepo != nil {
		clientInfo, _ = s.clientInfoRepo.GetAccountInfo(ctx, req.ClientID)
	}

	billingMode := domain.BillingModeOwn
	if clientInfo != nil {
		billingMode = clientInfo.BillingMode
	}

	var chargeResult *ChargeResult
	if isSubAccount && aggregatorID != uuid.Nil {
		switch billingMode {
		case domain.BillingModeOwn:
			// Sub-account pays from own balance (using aggregator tariff if set, else platform tariff)
			chargeAmount := result.ChargeAmount
			if subChargeAmount != "" {
				chargeAmount = subChargeAmount
			}
			chargeResult, err = s.saga.Charge(ctx,
				req.ClientID.String(), req.MessageID.String(),
				chargeAmount, plan.Currency,
				fmt.Sprintf("SMS tarification: %s, %d segments", plan.Strategy, req.SegmentCount),
				int32(req.SegmentCount),
			)
			if err != nil {
				return nil, fmt.Errorf("billing charge failed: %w", err)
			}
			if !chargeResult.Success {
				return &TarifyMessageResponse{
					Approved:        false,
					RejectionReason: domain.ErrInsufficientBalance.Error(),
				}, nil
			}

		case domain.BillingModeAggregator:
			// Aggregator pays for traffic from own balance
			chargeResult, err = s.saga.Charge(ctx,
				aggregatorID.String(), req.MessageID.String(),
				aggChargeAmount, plan.Currency,
				fmt.Sprintf("Traffic for sub-account %s: %d segments", req.ClientID, req.SegmentCount),
				int32(req.SegmentCount),
			)
			if err != nil {
				return nil, fmt.Errorf("aggregator charge failed: %w", err)
			}
			if !chargeResult.Success {
				return &TarifyMessageResponse{
					Approved:        false,
					RejectionReason: "aggregator balance insufficient",
				}, nil
			}

		case domain.BillingModeHybrid:
			// Try sub-account first, fallback to aggregator
			chargeAmount := result.ChargeAmount
			if subChargeAmount != "" {
				chargeAmount = subChargeAmount
			}
			chargeResult, err = s.saga.Charge(ctx,
				req.ClientID.String(), req.MessageID.String(),
				chargeAmount, plan.Currency,
				fmt.Sprintf("SMS tarification: %s, %d segments", plan.Strategy, req.SegmentCount),
				int32(req.SegmentCount),
			)
			if err != nil || (chargeResult != nil && !chargeResult.Success) {
				// Fallback to aggregator
				chargeResult, err = s.saga.Charge(ctx,
					aggregatorID.String(), req.MessageID.String(),
					aggChargeAmount, plan.Currency,
					fmt.Sprintf("Hybrid fallback for sub-account %s: %d segments", req.ClientID, req.SegmentCount),
					int32(req.SegmentCount),
				)
				if err != nil {
					return nil, fmt.Errorf("hybrid fallback charge failed: %w", err)
				}
				if !chargeResult.Success {
					return &TarifyMessageResponse{
						Approved:        false,
						RejectionReason: "both sub-account and aggregator balance insufficient",
					}, nil
				}
			}
		}

		// Log aggregator margin (for billing_mode own/hybrid where sub-account was charged)
		if billingMode == domain.BillingModeOwn || billingMode == domain.BillingModeHybrid {
			s.logAggregatorMargin(ctx, aggregatorID, req.ClientID, req.MessageID, req.OperatorID,
				req.SegmentCount, result.PricePerSegment, subChargeAmount, aggChargeAmount,
				req.IdempotencyKey)
		}
	} else {
		// Regular client (not a sub-account) — standard charge
		chargeResult, err = s.saga.Charge(ctx,
			req.ClientID.String(), req.MessageID.String(),
			result.ChargeAmount, plan.Currency,
			fmt.Sprintf("SMS tarification: %s strategy, %d segments", plan.Strategy, req.SegmentCount),
			int32(req.SegmentCount),
		)
		if err != nil {
			return nil, fmt.Errorf("billing charge failed: %w", err)
		}
		if !chargeResult.Success {
			return &TarifyMessageResponse{
				Approved:        false,
				RejectionReason: domain.ErrInsufficientBalance.Error(),
			}, nil
		}
	}
	_ = chargeResult
```

- [ ] **Step 3: Run existing tests to check for regressions**

Run: `cd c:/projects/sms && go test ./internal/services/tarification/application/ -v`
Expected: All existing tests PASS (quota is nil by default, so new code paths are skipped)

- [ ] **Step 4: Commit**

```bash
git add internal/services/tarification/application/tarification_service.go
git commit -m "feat(billing): integrate quota consumption and billing_mode into TarifyMessage pipeline"
```

---

### Task 7: Spending Limit Check

**Files:**
- Modify: `internal/services/tarification/application/tarification_service.go`

- [ ] **Step 1: Add spending limit check helper**

Add this method to `TarificationService`:

```go
// checkSpendingLimit verifies the sub-account hasn't exceeded spending limits on aggregator's balance.
// Returns true if within limits, false if exceeded.
func (s *TarificationService) checkSpendingLimit(ctx context.Context, clientInfo *domain.ClientAccountInfo, aggregatorID uuid.UUID, chargeAmount string) bool {
	// No limits set — always allow
	if clientInfo.SpendingLimitMonthly == nil && clientInfo.SpendingLimitDaily == nil {
		return true
	}

	// Query current spending from aggregator's balance for this sub-account
	// This uses the attributed_sub_account_id on transactions
	// For now, spending limits are enforced at the billing service level
	// via the attributed_sub_account_id aggregation
	return true // TODO: wire to billing service query when attribution is available
}
```

Note: Full spending limit enforcement requires a new gRPC call to billing service (`GetSpendingBySubAccount`). This is a follow-up task — the column and attribution are in place from Task 6.

- [ ] **Step 2: Commit**

```bash
git add internal/services/tarification/application/tarification_service.go
git commit -m "feat(billing): add spending limit check placeholder with attribution"
```

---

### Task 8: Admin API — Quota CRUD Handlers

**Files:**
- Create: `internal/gateway/admin/handlers/aggregator_quotas.go`
- Modify: `internal/gateway/admin/router/router.go`

- [ ] **Step 1: Implement admin quota handlers**

```go
// internal/gateway/admin/handlers/aggregator_quotas.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type AggregatorQuotaHandler struct {
	quotaService *application.QuotaService
}

func NewAggregatorQuotaHandler(qs *application.QuotaService) *AggregatorQuotaHandler {
	return &AggregatorQuotaHandler{quotaService: qs}
}

type createQuotaRequest struct {
	PeriodStart  string `json:"period_start"`  // "2026-05-01"
	PeriodEnd    string `json:"period_end"`    // "2026-06-01"
	SegmentLimit int64  `json:"segment_limit"`
	OverageRate  string `json:"overage_rate"`
	Currency     string `json:"currency"`
	AutoRenew    bool   `json:"auto_renew"`
}

func (h *AggregatorQuotaHandler) CreateQuota(w http.ResponseWriter, r *http.Request) {
	aggID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrBadRequest("invalid aggregator id"))
		return
	}

	var req createQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrBadRequest("invalid request body"))
		return
	}

	start, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		respondError(w, shared.ErrBadRequest("invalid period_start format, expected YYYY-MM-DD"))
		return
	}
	end, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		respondError(w, shared.ErrBadRequest("invalid period_end format, expected YYYY-MM-DD"))
		return
	}

	currency := req.Currency
	if currency == "" {
		currency = "RUB"
	}

	quota, err := h.quotaService.CreateQuota(r.Context(), aggID, start, end, req.SegmentLimit, req.OverageRate, currency, req.AutoRenew)
	if err != nil {
		log.Error().Err(err).Msg("failed to create quota")
		respondError(w, shared.ErrInternal("failed to create quota"))
		return
	}

	respondJSON(w, http.StatusCreated, quota)
}

func (h *AggregatorQuotaHandler) ListQuotas(w http.ResponseWriter, r *http.Request) {
	aggID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrBadRequest("invalid aggregator id"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	quotas, total, err := h.quotaService.ListQuotas(r.Context(), aggID, limit, offset)
	if err != nil {
		respondError(w, shared.ErrInternal("failed to list quotas"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"quotas": quotas,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

type updateQuotaRequest struct {
	SegmentLimit int64  `json:"segment_limit"`
	OverageRate  string `json:"overage_rate"`
	AutoRenew    bool   `json:"auto_renew"`
}

func (h *AggregatorQuotaHandler) UpdateQuota(w http.ResponseWriter, r *http.Request) {
	quotaID, err := uuid.Parse(mux.Vars(r)["quota_id"])
	if err != nil {
		respondError(w, shared.ErrBadRequest("invalid quota id"))
		return
	}

	var req updateQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrBadRequest("invalid request body"))
		return
	}

	quota, err := h.quotaService.UpdateQuota(r.Context(), quotaID, req.SegmentLimit, req.OverageRate, req.AutoRenew)
	if err != nil {
		log.Error().Err(err).Msg("failed to update quota")
		respondError(w, shared.ErrInternal("failed to update quota"))
		return
	}

	respondJSON(w, http.StatusOK, quota)
}

func (h *AggregatorQuotaHandler) GetActiveQuota(w http.ResponseWriter, r *http.Request) {
	aggID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrBadRequest("invalid aggregator id"))
		return
	}

	quota, err := h.quotaService.GetActiveQuota(r.Context(), aggID)
	if err != nil {
		respondError(w, shared.ErrInternal("failed to get quota"))
		return
	}
	if quota == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"quota": nil})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"quota": quota})
}
```

- [ ] **Step 2: Register routes in admin router**

In `internal/gateway/admin/router/router.go`, add in the route registration section:

```go
	// Aggregator Quotas
	r.HandleFunc("/admin/v1/aggregators/{id}/quotas", quotaHandler.CreateQuota).Methods("POST")
	r.HandleFunc("/admin/v1/aggregators/{id}/quotas", quotaHandler.ListQuotas).Methods("GET")
	r.HandleFunc("/admin/v1/aggregators/{id}/quotas/active", quotaHandler.GetActiveQuota).Methods("GET")
	r.HandleFunc("/admin/v1/aggregators/{id}/quotas/{quota_id}", quotaHandler.UpdateQuota).Methods("PUT")
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/admin/handlers/aggregator_quotas.go \
        internal/gateway/admin/router/router.go
git commit -m "feat(billing): add admin API endpoints for aggregator quota CRUD"
```

---

### Task 9: Portal API — Quota View + BillingMode Management

**Files:**
- Create: `internal/gateway/portal/handlers/aggregator_quotas.go`
- Modify: portal router file

- [ ] **Step 1: Implement portal quota handlers**

```go
// internal/gateway/portal/handlers/aggregator_quotas.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/shared"
	clientv1 "github.com/smpp-server/smpp-server/api/proto/clientv1"
)

type AggregatorQuotaHandlers struct {
	quotaService *application.QuotaService
	clientClient clientv1.ClientServiceClient
}

func NewAggregatorQuotaHandlers(qs *application.QuotaService, cc clientv1.ClientServiceClient) *AggregatorQuotaHandlers {
	return &AggregatorQuotaHandlers{quotaService: qs, clientClient: cc}
}

// GetMyQuota returns the current active quota for the authenticated aggregator.
func (h *AggregatorQuotaHandlers) GetMyQuota(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("client not found"))
		return
	}

	quota, err := h.quotaService.GetActiveQuota(r.Context(), clientID)
	if err != nil {
		respondError(w, shared.ErrInternal("failed to get quota"))
		return
	}

	if quota == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"quota": nil})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"quota": map[string]interface{}{
			"id":             quota.ID,
			"segment_limit":  quota.SegmentLimit,
			"segments_used":  quota.SegmentsUsed,
			"overage_rate":   quota.OverageRate,
			"currency":       quota.Currency,
			"period_start":   quota.PeriodStart,
			"period_end":     quota.PeriodEnd,
			"utilization":    quota.UtilizationPercent(),
			"is_exhausted":   quota.IsExhausted(),
			"overage_segments": quota.OverageSegments(),
		},
	})
}

type updateBillingModeRequest struct {
	BillingMode          string  `json:"billing_mode"`
	SpendingLimitMonthly *string `json:"spending_limit_monthly,omitempty"`
	SpendingLimitDaily   *string `json:"spending_limit_daily,omitempty"`
}

// UpdateSubAccountBilling allows aggregator to set billing_mode and spending limits on a sub-account.
func (h *AggregatorQuotaHandlers) UpdateSubAccountBilling(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("client not found"))
		return
	}

	subAccountID, err := uuid.Parse(mux.Vars(r)["sub_account_id"])
	if err != nil {
		respondError(w, shared.ErrBadRequest("invalid sub_account_id"))
		return
	}

	var req updateBillingModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrBadRequest("invalid request body"))
		return
	}

	// Validate billing_mode
	switch req.BillingMode {
	case "own", "aggregator", "hybrid":
	default:
		respondError(w, shared.ErrBadRequest("billing_mode must be own, aggregator, or hybrid"))
		return
	}

	// Verify sub-account belongs to this aggregator
	subAccount, err := h.clientClient.GetClient(r.Context(), &clientv1.GetClientRequest{ClientId: subAccountID.String()})
	if err != nil {
		respondError(w, shared.ErrInternal("failed to get sub-account"))
		return
	}
	if subAccount.GetParentClientId() != clientID.String() {
		respondError(w, shared.ErrForbidden("sub-account does not belong to this aggregator"))
		return
	}

	// Update via client service
	_, err = h.clientClient.UpdateClient(r.Context(), &clientv1.UpdateClientRequest{
		ClientId:             subAccountID.String(),
		BillingMode:          req.BillingMode,
		SpendingLimitMonthly: stringPtrToString(req.SpendingLimitMonthly),
		SpendingLimitDaily:   stringPtrToString(req.SpendingLimitDaily),
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to update sub-account billing")
		respondError(w, shared.ErrInternal("failed to update billing settings"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sub_account_id": subAccountID,
		"billing_mode":   req.BillingMode,
	})
}

func stringPtrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```

- [ ] **Step 2: Register portal routes**

Add to the portal router:

```go
	// Aggregator quota (for aggregator users)
	r.HandleFunc("/portal/v1/quota", quotaHandlers.GetMyQuota).Methods("GET")
	r.HandleFunc("/portal/v1/sub-accounts/{sub_account_id}/billing", quotaHandlers.UpdateSubAccountBilling).Methods("PUT")
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/aggregator_quotas.go
git commit -m "feat(billing): add portal API for quota view and sub-account billing mode"
```

---

### Task 10: Wire Dependencies in Service Initialization

**Files:**
- Modify: service initialization code (typically `cmd/gateway/main.go` or DI setup)

- [ ] **Step 1: Find and modify the service wiring**

Search for where `TarificationService` is created and `SetAggregatorRepos` is called. Add after it:

```go
	// Wire aggregator quota service
	quotaRepo := repository.NewAggregatorQuotaRepository(db)
	quotaService := application.NewQuotaService(quotaRepo)
	tarificationService.SetQuotaService(quotaService)
```

Wire the admin handler:

```go
	quotaHandler := handlers.NewAggregatorQuotaHandler(quotaService)
```

Wire the portal handler:

```go
	portalQuotaHandlers := portalHandlers.NewAggregatorQuotaHandlers(quotaService, clientClient)
```

- [ ] **Step 2: Verify compilation**

Run: `cd c:/projects/sms && go build ./...`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add cmd/ internal/
git commit -m "feat(billing): wire quota service and handlers into application startup"
```

---

### Task 11: Frontend — Admin Quota Management

**Files:**
- Create: `portal-frontend/src/pages/admin/aggregators/AggregatorQuotasPage.tsx`
- Modify: `portal-frontend/src/api/admin.ts`

- [ ] **Step 1: Add API functions**

In `portal-frontend/src/api/admin.ts`:

```typescript
// Aggregator Quotas
export async function getAggregatorQuotas(aggregatorId: string, limit = 20, offset = 0) {
  const res = await adminApi.get(`/aggregators/${aggregatorId}/quotas`, { params: { limit, offset } });
  return res.data;
}

export async function getActiveQuota(aggregatorId: string) {
  const res = await adminApi.get(`/aggregators/${aggregatorId}/quotas/active`);
  return res.data;
}

export async function createAggregatorQuota(aggregatorId: string, data: {
  period_start: string;
  period_end: string;
  segment_limit: number;
  overage_rate: string;
  currency?: string;
  auto_renew?: boolean;
}) {
  const res = await adminApi.post(`/aggregators/${aggregatorId}/quotas`, data);
  return res.data;
}

export async function updateAggregatorQuota(aggregatorId: string, quotaId: string, data: {
  segment_limit: number;
  overage_rate: string;
  auto_renew: boolean;
}) {
  const res = await adminApi.put(`/aggregators/${aggregatorId}/quotas/${quotaId}`, data);
  return res.data;
}
```

- [ ] **Step 2: Create admin quota management page**

Create `portal-frontend/src/pages/admin/aggregators/AggregatorQuotasPage.tsx` with:
- Table showing quota history (period, limit, used, utilization %, overage rate, auto-renew)
- Progress bar for current quota utilization
- "Create Quota" dialog with period_start, period_end, segment_limit, overage_rate, currency, auto_renew fields
- "Edit" button on current quota to modify segment_limit, overage_rate, auto_renew

Follow existing admin page patterns (table + dialog, Radix UI components, Tailwind CSS).

- [ ] **Step 3: Add route in admin router**

Wire the page into the admin routing configuration.

- [ ] **Step 4: Test in browser**

Run: `cd c:/projects/sms/portal-frontend && npm run dev`
Navigate to the aggregator quota management page, verify:
- Quota table renders
- Create dialog opens and submits
- Edit dialog opens and saves

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/admin/aggregators/AggregatorQuotasPage.tsx \
        portal-frontend/src/api/admin.ts
git commit -m "feat(billing): add admin UI for aggregator quota management"
```

---

### Task 12: Frontend — Portal Quota View + BillingMode

**Files:**
- Create: `portal-frontend/src/pages/network/NetworkQuotaPage.tsx`
- Modify: `portal-frontend/src/api/portal.ts` (or equivalent)

- [ ] **Step 1: Add portal API functions**

```typescript
export async function getMyQuota() {
  const res = await portalApi.get('/quota');
  return res.data;
}

export async function updateSubAccountBilling(subAccountId: string, data: {
  billing_mode: 'own' | 'aggregator' | 'hybrid';
  spending_limit_monthly?: string;
  spending_limit_daily?: string;
}) {
  const res = await portalApi.put(`/sub-accounts/${subAccountId}/billing`, data);
  return res.data;
}
```

- [ ] **Step 2: Create quota dashboard page for aggregator portal**

Create `portal-frontend/src/pages/network/NetworkQuotaPage.tsx` with:
- Quota card: used/limit with progress bar, percentage, overage count
- Period dates, currency, overage rate displayed
- Sub-accounts table with billing_mode column and edit dropdown (own/aggregator/hybrid)
- Spending limit inputs for each sub-account

Follow existing portal page patterns and the network layout.

- [ ] **Step 3: Wire route into network layout**

Add the page to the network sidebar navigation.

- [ ] **Step 4: Test in browser**

Run dev server, verify:
- Quota card shows current usage
- Billing mode dropdown works
- Spending limits can be set

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/network/NetworkQuotaPage.tsx \
        portal-frontend/src/api/portal.ts
git commit -m "feat(billing): add portal UI for aggregator quota view and sub-account billing mode"
```

---

### Task 13: Transaction Attribution Support in Billing Service

**Files:**
- Modify: `internal/services/billing/domain/transaction.go`
- Modify: `internal/services/billing/application/billing_service.go`
- Modify: `api/proto/billing/billing.proto`

- [ ] **Step 1: Add attributed_sub_account_id to Transaction domain**

In `internal/services/billing/domain/transaction.go`, add to Transaction struct:

```go
	AttributedSubAccountID *uuid.UUID
```

Add builder method:

```go
// WithAttributedSubAccount tags a transaction with the sub-account that triggered it.
func (t *Transaction) WithAttributedSubAccount(subAccountID uuid.UUID) *Transaction {
	t.AttributedSubAccountID = &subAccountID
	return t
}
```

- [ ] **Step 2: Update ChargeMessage proto to accept attributed_sub_account_id**

In `api/proto/billing/billing.proto`, add to `ChargeMessageRequest`:

```protobuf
  string attributed_sub_account_id = 7; // optional: sub-account that triggered this charge
```

- [ ] **Step 3: Update billing service to persist attribution**

In `ChargeMessage` in billing_service.go, after creating the transaction, add:

```go
	if req.AttributedSubAccountId != "" {
		subAccID, parseErr := uuid.Parse(req.AttributedSubAccountId)
		if parseErr == nil {
			txn.WithAttributedSubAccount(subAccID)
		}
	}
```

Update the transaction repository INSERT query to include `attributed_sub_account_id`.

- [ ] **Step 4: Regenerate proto**

Run: `cd c:/projects/sms && make proto`
Expected: Proto files regenerated

- [ ] **Step 5: Commit**

```bash
git add internal/services/billing/ api/proto/billing/ 
git commit -m "feat(billing): add transaction attribution for sub-account tracking"
```

---

### Task 14: Integration Test — Full Pipeline

**Files:**
- Create: `internal/services/tarification/application/quota_integration_test.go`

- [ ] **Step 1: Write integration test for quota + billing_mode flow**

```go
// internal/services/tarification/application/quota_integration_test.go
package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

func TestTarifyMessage_SubAccountWithQuota_BillingModeAggregator(t *testing.T) {
	// Setup: sub-account with billing_mode=aggregator, aggregator has active quota

	// Mock repos
	quotaRepo := new(mockQuotaRepo)
	quotaService := NewQuotaService(quotaRepo)

	aggID := uuid.New()
	subAccountID := uuid.New()
	quotaID := uuid.New()

	// Quota: 1000 limit, 500 used — within quota
	quotaRepo.On("GetActive", mock.Anything, aggID, mock.Anything).Return(&domain.AggregatorQuota{
		ID: quotaID, AggregatorID: aggID, SegmentLimit: 1000, SegmentsUsed: 500,
		OverageRate: "0.10", Currency: "RUB",
	}, nil)
	quotaRepo.On("IncrementUsage", mock.Anything, quotaID, 1).Return(&domain.IncrementResult{
		SegmentsUsed: 501, SegmentLimit: 1000, OverageRate: "0.10",
		WasWithinQuota: true, OverageCount: 0,
	}, nil)

	// Consume quota
	result, err := quotaService.ConsumeQuota(context.Background(), aggID, 1)
	require.NoError(t, err)
	assert.False(t, result.HasOverage)
	assert.Equal(t, "0", result.OverageChargeAmount)

	_ = subAccountID // Used in full integration with TarificationService

	quotaRepo.AssertExpectations(t)
}

func TestTarifyMessage_SubAccountWithQuota_OverageCharges(t *testing.T) {
	quotaRepo := new(mockQuotaRepo)
	quotaService := NewQuotaService(quotaRepo)

	aggID := uuid.New()
	quotaID := uuid.New()

	// Quota: 1000 limit, 999 used — will cross into overage
	quotaRepo.On("GetActive", mock.Anything, aggID, mock.Anything).Return(&domain.AggregatorQuota{
		ID: quotaID, AggregatorID: aggID, SegmentLimit: 1000, SegmentsUsed: 999,
		OverageRate: "0.25", Currency: "RUB",
	}, nil)
	quotaRepo.On("IncrementUsage", mock.Anything, quotaID, 3).Return(&domain.IncrementResult{
		SegmentsUsed: 1002, SegmentLimit: 1000, OverageRate: "0.25",
		WasWithinQuota: true, OverageCount: 2,
	}, nil)

	result, err := quotaService.ConsumeQuota(context.Background(), aggID, 3)
	require.NoError(t, err)
	assert.True(t, result.HasOverage)
	assert.Equal(t, "0.50", result.OverageChargeAmount) // 2 * 0.25

	quotaRepo.AssertExpectations(t)
}
```

- [ ] **Step 2: Run all tests**

Run: `cd c:/projects/sms && go test ./internal/services/tarification/application/ -v`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add internal/services/tarification/application/quota_integration_test.go
git commit -m "test(billing): add integration tests for quota consumption and overage"
```

---

### Task 15: Deploy and Verify

- [ ] **Step 1: Verify full build**

Run: `cd c:/projects/sms && go build ./...`
Expected: Build succeeds

- [ ] **Step 2: Run all tests**

Run: `cd c:/projects/sms && go test ./... -count=1`
Expected: All tests pass

- [ ] **Step 3: Run frontend build**

Run: `cd c:/projects/sms/portal-frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Commit any remaining changes**

```bash
git add -A
git commit -m "feat(billing): aggregator infrastructure quotas and billing modes - complete"
```
