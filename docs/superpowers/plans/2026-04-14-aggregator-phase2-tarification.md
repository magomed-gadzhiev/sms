# Aggregator Phase 2: Tarification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement sub-account tarification — aggregator sets tariffs for sub-accounts, platform performs dual deduction (sub-account virtual balance + aggregator real balance) on each SMS, logs margin.

**Architecture:** Extend `TarifyMessage()` in tarification-service with a sub-account branch that resolves account type from Redis cache, looks up aggregator tariff (personal → default → fallback), then calls new `ChargeMessageDual` in billing-service which locks both accounts atomically in one DB transaction. Pipeline refund path extended to call `RefundMessageDual` for dual charges.

**Tech Stack:** Go 1.24 + pgx/v5 (via sqlx), Redis go-redis/v9, gRPC google.golang.org/grpc, protobuf, React 19 + TypeScript + Tailwind CSS 4.2 + Radix UI

---

## File Map

**New files:**
- `migrations/000097_create_aggregator_tariffs.up.sql`
- `migrations/000097_create_aggregator_tariffs.down.sql`
- `migrations/000098_create_aggregator_margin_log.up.sql`
- `migrations/000098_create_aggregator_margin_log.down.sql`
- `internal/services/aggregator/domain/tariff.go`
- `internal/services/aggregator/domain/margin.go`
- `internal/services/aggregator/repository/tariff_repository.go`
- `internal/services/aggregator/repository/margin_repository.go`
- `internal/services/aggregator/service/tariff_service.go`
- `internal/gateway/portal/handlers/aggregator_tariffs.go`
- `portal-frontend/src/pages/aggregator/TariffsPage.tsx`

**Modified files:**
- `internal/services/aggregator/domain/errors.go` — new tariff errors
- `api/proto/billing/billing.proto` — add ChargeMessageDual, RefundMessageDual
- `api/proto/billingv1/billing.pb.go` — regenerated
- `api/proto/billingv1/billing_grpc.pb.go` — regenerated
- `internal/services/billing/application/billing_service.go` — ChargeMessageDual, RefundMessageDual
- `internal/services/billing/grpc/server.go` — new gRPC handlers
- `api/proto/tarification/tarification.proto` — extend TarifyMessageResponse
- `api/proto/tarificationv1/tarification.pb.go` — regenerated
- `internal/services/tarification/application/tarification_service.go` — sub-account branch
- `internal/services/tarification/application/saga.go` — ChargeDual method
- `internal/services/tarification/grpc/server.go` — pass new response fields
- `internal/pipeline/sender/stage.go` — dual refund path
- `api/proto/aggregator/aggregator.proto` — tariff RPCs
- `api/proto/aggregatorv1/aggregator.pb.go` — regenerated
- `internal/services/aggregator/grpc/server.go` — tariff handlers
- `scripts/generate-proto.sh` — add aggregator + tarification
- `portal-frontend/src/pages/aggregator/tabs/TariffsTab.tsx` — read-only + link

All work is done in worktree `.worktrees/aggregator-phase2/` on branch `feature/aggregator-phase2`.

---

## Task 1: Create Worktree

**Files:** none (setup)

- [ ] **Step 1: Create Phase 2 worktree from Phase 1 branch**

```bash
git worktree add .worktrees/aggregator-phase2 -b feature/aggregator-phase2 feature/aggregator-phase1
```

- [ ] **Step 2: Verify worktree**

```bash
ls .worktrees/aggregator-phase2/internal/services/aggregator/
```
Expected: `domain/  grpc/  repository/  service/`

- [ ] **Step 3: Commit**

```bash
cd .worktrees/aggregator-phase2
git commit --allow-empty -m "chore: start aggregator phase 2 tarification"
```

---

## Task 2: Migration — aggregator_tariffs

**Files:**
- Create: `migrations/000097_create_aggregator_tariffs.up.sql`
- Create: `migrations/000097_create_aggregator_tariffs.down.sql`

All steps run from `.worktrees/aggregator-phase2/`.

- [ ] **Step 1: Write up migration**

```sql
-- migrations/000097_create_aggregator_tariffs.up.sql
CREATE TABLE aggregator_tariffs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    sub_account_id UUID REFERENCES clients(id) ON DELETE CASCADE,
    operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE RESTRICT,
    sender_category VARCHAR(30) NOT NULL,
    price_per_segment NUMERIC(20,6) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (aggregator_id, sub_account_id, operator_id, sender_category)
);

CREATE INDEX idx_aggregator_tariffs_agg ON aggregator_tariffs(aggregator_id);
CREATE INDEX idx_aggregator_tariffs_sub ON aggregator_tariffs(sub_account_id) WHERE sub_account_id IS NOT NULL;
```

- [ ] **Step 2: Write down migration**

```sql
-- migrations/000097_create_aggregator_tariffs.down.sql
DROP TABLE IF EXISTS aggregator_tariffs;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000097_create_aggregator_tariffs.*
git commit -m "feat(migration): create aggregator_tariffs table"
```

---

## Task 3: Migration — aggregator_margin_log

**Files:**
- Create: `migrations/000098_create_aggregator_margin_log.up.sql`
- Create: `migrations/000098_create_aggregator_margin_log.down.sql`

- [ ] **Step 1: Write up migration**

```sql
-- migrations/000098_create_aggregator_margin_log.up.sql
CREATE TABLE aggregator_margin_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id),
    sub_account_id UUID NOT NULL REFERENCES clients(id),
    message_id UUID NOT NULL,
    operator_id UUID NOT NULL REFERENCES operators(id),
    segment_count INTEGER NOT NULL,
    sub_account_price NUMERIC(20,6) NOT NULL,
    aggregator_price NUMERIC(20,6) NOT NULL,
    sub_account_total NUMERIC(20,6) NOT NULL,
    aggregator_total NUMERIC(20,6) NOT NULL,
    margin NUMERIC(20,6) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    idempotency_key VARCHAR(255) NOT NULL
) PARTITION BY RANGE (created_at);

CREATE UNIQUE INDEX idx_agg_margin_log_idempotency ON aggregator_margin_log(idempotency_key);
CREATE INDEX idx_agg_margin_log_agg ON aggregator_margin_log(aggregator_id, created_at);
CREATE INDEX idx_agg_margin_log_sub ON aggregator_margin_log(sub_account_id, created_at);
CREATE INDEX idx_agg_margin_log_operator ON aggregator_margin_log(operator_id);

-- Create partitions for current month + next 11 months
DO $$
DECLARE
    start_date DATE;
    end_date DATE;
    partition_name TEXT;
    i INT;
BEGIN
    FOR i IN 0..11 LOOP
        start_date := DATE_TRUNC('month', NOW()) + (i || ' months')::INTERVAL;
        end_date   := start_date + INTERVAL '1 month';
        partition_name := 'aggregator_margin_log_' || TO_CHAR(start_date, 'YYYY_MM');
        EXECUTE FORMAT(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF aggregator_margin_log FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
    END LOOP;
END $$;
```

- [ ] **Step 2: Write down migration**

```sql
-- migrations/000098_create_aggregator_margin_log.down.sql
DROP TABLE IF EXISTS aggregator_margin_log CASCADE;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000098_create_aggregator_margin_log.*
git commit -m "feat(migration): create aggregator_margin_log partitioned table"
```

---

## Task 4: Domain Models — AggregatorTariff, MarginEntry, Errors

**Files:**
- Create: `internal/services/aggregator/domain/tariff.go`
- Create: `internal/services/aggregator/domain/margin.go`
- Modify: `internal/services/aggregator/domain/errors.go`

- [ ] **Step 1: Write tariff.go**

```go
// internal/services/aggregator/domain/tariff.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// AggregatorTariff represents a tariff set by aggregator for a sub-account.
// sub_account_id == nil means it's a default tariff for all sub-accounts.
type AggregatorTariff struct {
	ID              uuid.UUID
	AggregatorID    uuid.UUID
	SubAccountID    *uuid.UUID // nil = default tariff
	OperatorID      uuid.UUID
	SenderCategory  string
	PricePerSegment string // NUMERIC as string for precision
	Active          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// Joined fields (populated on read)
	OperatorName    string
	SubAccountName  string
}

// IsDefault returns true if this tariff applies to all sub-accounts.
func (t *AggregatorTariff) IsDefault() bool {
	return t.SubAccountID == nil
}
```

- [ ] **Step 2: Write margin.go**

```go
// internal/services/aggregator/domain/margin.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// MarginEntry records margin earned by aggregator on a single message.
type MarginEntry struct {
	ID               uuid.UUID
	AggregatorID     uuid.UUID
	SubAccountID     uuid.UUID
	MessageID        uuid.UUID
	OperatorID       uuid.UUID
	SegmentCount     int
	SubAccountPrice  string // price per segment charged to sub-account
	AggregatorPrice  string // purchase price per segment charged to aggregator
	SubAccountTotal  string // SubAccountPrice * SegmentCount
	AggregatorTotal  string // AggregatorPrice * SegmentCount
	Margin           string // SubAccountTotal - AggregatorTotal
	CreatedAt        time.Time
	IdempotencyKey   string
}
```

- [ ] **Step 3: Add tariff errors to errors.go**

Add these lines to the existing `var (...)` block in `domain/errors.go`:

```go
	ErrTariffNotFound         = errors.New("aggregator tariff not found")
	ErrTariffBelowPurchase    = errors.New("tariff price cannot be below purchase price")
	ErrTariffExceedsMaxMarkup = errors.New("tariff price exceeds maximum markup")
	ErrNoTariffForOperator    = errors.New("no tariff found for this operator and sender category")
```

- [ ] **Step 4: Build to verify no errors**

```bash
go build ./internal/services/aggregator/...
```
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/services/aggregator/domain/
git commit -m "feat(aggregator): add AggregatorTariff, MarginEntry domain models and tariff errors"
```

---

## Task 5: TariffRepository

**Files:**
- Create: `internal/services/aggregator/repository/tariff_repository.go`

- [ ] **Step 1: Write failing test**

```go
// internal/services/aggregator/repository/tariff_repository_test.go
package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/repository"
)

func TestTariffRepository_CreateAndGet(t *testing.T) {
	// Unit test with mock DB — verifies interface contract
	repo := repository.NewMockTariffRepository()

	aggID := uuid.New()
	opID := uuid.New()

	tariff := &domain.AggregatorTariff{
		AggregatorID:    aggID,
		SubAccountID:    nil,
		OperatorID:      opID,
		SenderCategory:  "shared",
		PricePerSegment: "3.500000",
		Active:          true,
	}

	err := repo.Create(context.Background(), tariff)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, tariff.ID)

	got, err := repo.GetByID(context.Background(), tariff.ID)
	require.NoError(t, err)
	require.Equal(t, "3.500000", got.PricePerSegment)
}

func TestTariffRepository_LookupCascade(t *testing.T) {
	repo := repository.NewMockTariffRepository()

	aggID := uuid.New()
	subID := uuid.New()
	opID := uuid.New()

	// Default tariff
	defaultTariff := &domain.AggregatorTariff{
		AggregatorID: aggID, SubAccountID: nil,
		OperatorID: opID, SenderCategory: "shared",
		PricePerSegment: "3.000000", Active: true,
	}
	require.NoError(t, repo.Create(context.Background(), defaultTariff))

	// Personal tariff
	personalTariff := &domain.AggregatorTariff{
		AggregatorID: aggID, SubAccountID: &subID,
		OperatorID: opID, SenderCategory: "shared",
		PricePerSegment: "4.000000", Active: true,
	}
	require.NoError(t, repo.Create(context.Background(), personalTariff))

	// Personal lookup returns personal tariff
	got, err := repo.LookupTariff(context.Background(), aggID, subID, opID, "shared")
	require.NoError(t, err)
	require.Equal(t, "4.000000", got.PricePerSegment)
}
```

- [ ] **Step 2: Run test — verify FAIL**

```bash
go test ./internal/services/aggregator/repository/... -run TestTariffRepository -v
```
Expected: FAIL — `NewMockTariffRepository` not defined

- [ ] **Step 3: Write tariff_repository.go**

```go
// internal/services/aggregator/repository/tariff_repository.go
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
)

// TariffRepository handles aggregator_tariffs CRUD.
type TariffRepository struct {
	db *sqlx.DB
}

func NewTariffRepository(db *sqlx.DB) *TariffRepository {
	return &TariffRepository{db: db}
}

func (r *TariffRepository) Create(ctx context.Context, t *domain.AggregatorTariff) error {
	t.ID = uuid.New()
	query := `
		INSERT INTO aggregator_tariffs
			(id, aggregator_id, sub_account_id, operator_id, sender_category, price_per_segment, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.db.ExecContext(ctx, query,
		t.ID, t.AggregatorID, t.SubAccountID, t.OperatorID,
		t.SenderCategory, t.PricePerSegment, t.Active)
	if err != nil {
		return fmt.Errorf("create aggregator tariff: %w", err)
	}
	return nil
}

func (r *TariffRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.AggregatorTariff, error) {
	var t domain.AggregatorTariff
	query := `
		SELECT id, aggregator_id, sub_account_id, operator_id, sender_category,
		       price_per_segment, active, created_at, updated_at
		FROM aggregator_tariffs WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.AggregatorID, &t.SubAccountID, &t.OperatorID, &t.SenderCategory,
		&t.PricePerSegment, &t.Active, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrTariffNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get tariff by id: %w", err)
	}
	return &t, nil
}

// LookupTariff finds the most specific tariff: personal first, then default.
func (r *TariffRepository) LookupTariff(ctx context.Context, aggregatorID, subAccountID, operatorID uuid.UUID, senderCategory string) (*domain.AggregatorTariff, error) {
	query := `
		SELECT id, aggregator_id, sub_account_id, operator_id, sender_category,
		       price_per_segment, active, created_at, updated_at
		FROM aggregator_tariffs
		WHERE aggregator_id = $1
		  AND operator_id = $2
		  AND sender_category = $3
		  AND active = true
		  AND (sub_account_id = $4 OR sub_account_id IS NULL)
		ORDER BY sub_account_id NULLS LAST
		LIMIT 1`
	var t domain.AggregatorTariff
	err := r.db.QueryRowContext(ctx, query, aggregatorID, operatorID, senderCategory, subAccountID).Scan(
		&t.ID, &t.AggregatorID, &t.SubAccountID, &t.OperatorID, &t.SenderCategory,
		&t.PricePerSegment, &t.Active, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNoTariffForOperator
	}
	if err != nil {
		return nil, fmt.Errorf("lookup aggregator tariff: %w", err)
	}
	return &t, nil
}

func (r *TariffRepository) Update(ctx context.Context, t *domain.AggregatorTariff) error {
	query := `
		UPDATE aggregator_tariffs
		SET price_per_segment = $1, active = $2, updated_at = NOW()
		WHERE id = $3 AND aggregator_id = $4`
	result, err := r.db.ExecContext(ctx, query, t.PricePerSegment, t.Active, t.ID, t.AggregatorID)
	if err != nil {
		return fmt.Errorf("update aggregator tariff: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTariffNotFound
	}
	return nil
}

func (r *TariffRepository) Delete(ctx context.Context, id, aggregatorID uuid.UUID) error {
	query := `DELETE FROM aggregator_tariffs WHERE id = $1 AND aggregator_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, aggregatorID)
	if err != nil {
		return fmt.Errorf("delete aggregator tariff: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTariffNotFound
	}
	return nil
}

func (r *TariffRepository) List(ctx context.Context, aggregatorID uuid.UUID, subAccountID *uuid.UUID, operatorID *uuid.UUID, defaultsOnly bool, limit, offset int) ([]*domain.AggregatorTariff, int, error) {
	args := []interface{}{aggregatorID}
	where := "WHERE aggregator_id = $1"
	n := 2

	if defaultsOnly {
		where += " AND sub_account_id IS NULL"
	} else if subAccountID != nil {
		where += fmt.Sprintf(" AND sub_account_id = $%d", n)
		args = append(args, *subAccountID)
		n++
	}
	if operatorID != nil {
		where += fmt.Sprintf(" AND operator_id = $%d", n)
		args = append(args, *operatorID)
		n++
	}

	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM aggregator_tariffs "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count aggregator tariffs: %w", err)
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT t.id, t.aggregator_id, t.sub_account_id, t.operator_id, t.sender_category,
		       t.price_per_segment, t.active, t.created_at, t.updated_at,
		       COALESCE(o.name, '') as operator_name,
		       COALESCE(c.name, '') as sub_account_name
		FROM aggregator_tariffs t
		LEFT JOIN operators o ON o.id = t.operator_id
		LEFT JOIN clients c ON c.id = t.sub_account_id
		%s ORDER BY t.created_at DESC LIMIT $%d OFFSET $%d`, where, n, n+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list aggregator tariffs: %w", err)
	}
	defer rows.Close()

	var tariffs []*domain.AggregatorTariff
	for rows.Next() {
		var t domain.AggregatorTariff
		if err := rows.Scan(&t.ID, &t.AggregatorID, &t.SubAccountID, &t.OperatorID, &t.SenderCategory,
			&t.PricePerSegment, &t.Active, &t.CreatedAt, &t.UpdatedAt,
			&t.OperatorName, &t.SubAccountName); err != nil {
			return nil, 0, fmt.Errorf("scan tariff row: %w", err)
		}
		tariffs = append(tariffs, &t)
	}
	return tariffs, total, rows.Err()
}

// MockTariffRepository is an in-memory mock for unit tests.
type MockTariffRepository struct {
	tariffs map[uuid.UUID]*domain.AggregatorTariff
}

func NewMockTariffRepository() *MockTariffRepository {
	return &MockTariffRepository{tariffs: make(map[uuid.UUID]*domain.AggregatorTariff)}
}

func (m *MockTariffRepository) Create(ctx context.Context, t *domain.AggregatorTariff) error {
	t.ID = uuid.New()
	cp := *t
	m.tariffs[t.ID] = &cp
	return nil
}

func (m *MockTariffRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.AggregatorTariff, error) {
	t, ok := m.tariffs[id]
	if !ok {
		return nil, domain.ErrTariffNotFound
	}
	return t, nil
}

func (m *MockTariffRepository) LookupTariff(ctx context.Context, aggregatorID, subAccountID, operatorID uuid.UUID, senderCategory string) (*domain.AggregatorTariff, error) {
	// personal first
	for _, t := range m.tariffs {
		if t.AggregatorID == aggregatorID && t.OperatorID == operatorID &&
			t.SenderCategory == senderCategory && t.Active &&
			t.SubAccountID != nil && *t.SubAccountID == subAccountID {
			return t, nil
		}
	}
	// default
	for _, t := range m.tariffs {
		if t.AggregatorID == aggregatorID && t.OperatorID == operatorID &&
			t.SenderCategory == senderCategory && t.Active && t.SubAccountID == nil {
			return t, nil
		}
	}
	return nil, domain.ErrNoTariffForOperator
}

func (m *MockTariffRepository) Update(ctx context.Context, t *domain.AggregatorTariff) error {
	if _, ok := m.tariffs[t.ID]; !ok {
		return domain.ErrTariffNotFound
	}
	cp := *t
	m.tariffs[t.ID] = &cp
	return nil
}

func (m *MockTariffRepository) Delete(ctx context.Context, id, aggregatorID uuid.UUID) error {
	if _, ok := m.tariffs[id]; !ok {
		return domain.ErrTariffNotFound
	}
	delete(m.tariffs, id)
	return nil
}

func (m *MockTariffRepository) List(ctx context.Context, aggregatorID uuid.UUID, subAccountID *uuid.UUID, operatorID *uuid.UUID, defaultsOnly bool, limit, offset int) ([]*domain.AggregatorTariff, int, error) {
	var result []*domain.AggregatorTariff
	for _, t := range m.tariffs {
		if t.AggregatorID != aggregatorID {
			continue
		}
		result = append(result, t)
	}
	return result, len(result), nil
}
```

- [ ] **Step 4: Run test — verify PASS**

```bash
go test ./internal/services/aggregator/repository/... -run TestTariffRepository -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/aggregator/repository/tariff_repository.go \
        internal/services/aggregator/repository/tariff_repository_test.go
git commit -m "feat(aggregator): add TariffRepository with cascade lookup and mock"
```

---

## Task 6: MarginRepository

**Files:**
- Create: `internal/services/aggregator/repository/margin_repository.go`

- [ ] **Step 1: Write margin_repository.go**

```go
// internal/services/aggregator/repository/margin_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
)

// MarginRepository writes margin entries to aggregator_margin_log.
type MarginRepository struct {
	db *sqlx.DB
}

func NewMarginRepository(db *sqlx.DB) *MarginRepository {
	return &MarginRepository{db: db}
}

func (r *MarginRepository) Create(ctx context.Context, e *domain.MarginEntry) error {
	e.ID = uuid.New()
	query := `
		INSERT INTO aggregator_margin_log
			(id, aggregator_id, sub_account_id, message_id, operator_id, segment_count,
			 sub_account_price, aggregator_price, sub_account_total, aggregator_total, margin, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (idempotency_key) DO NOTHING`
	_, err := r.db.ExecContext(ctx, query,
		e.ID, e.AggregatorID, e.SubAccountID, e.MessageID, e.OperatorID, e.SegmentCount,
		e.SubAccountPrice, e.AggregatorPrice, e.SubAccountTotal, e.AggregatorTotal,
		e.Margin, e.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("create margin entry: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Build check**

```bash
go build ./internal/services/aggregator/...
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/services/aggregator/repository/margin_repository.go
git commit -m "feat(aggregator): add MarginRepository"
```

---

## Task 7: TariffService

**Files:**
- Create: `internal/services/aggregator/service/tariff_service.go`

- [ ] **Step 1: Write failing test**

```go
// internal/services/aggregator/service/tariff_service_test.go
package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/repository"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/service"
)

func TestTariffService_CreateTariff_BelowPurchase(t *testing.T) {
	repo := repository.NewMockTariffRepository()
	// purchase price for operator = "2.000000" (from mock)
	svc := service.NewTariffService(repo, service.MockPurchasePriceFn("2.000000", nil))

	aggID := uuid.New()
	opID := uuid.New()

	// price below purchase — should fail
	err := svc.SetTariff(context.Background(), &domain.AggregatorTariff{
		AggregatorID:    aggID,
		SubAccountID:    nil,
		OperatorID:      opID,
		SenderCategory:  "shared",
		PricePerSegment: "1.500000",
	}, nil)
	require.ErrorIs(t, err, domain.ErrTariffBelowPurchase)
}

func TestTariffService_CreateTariff_ExceedsMaxMarkup(t *testing.T) {
	repo := repository.NewMockTariffRepository()
	svc := service.NewTariffService(repo, service.MockPurchasePriceFn("2.000000", nil))

	aggID := uuid.New()
	opID := uuid.New()
	maxMarkup := "100.00" // +100% => max price = 4.00

	err := svc.SetTariff(context.Background(), &domain.AggregatorTariff{
		AggregatorID:    aggID,
		SubAccountID:    nil,
		OperatorID:      opID,
		SenderCategory:  "shared",
		PricePerSegment: "5.000000", // above 4.00
	}, &maxMarkup)
	require.ErrorIs(t, err, domain.ErrTariffExceedsMaxMarkup)
}

func TestTariffService_CreateTariff_Valid(t *testing.T) {
	repo := repository.NewMockTariffRepository()
	svc := service.NewTariffService(repo, service.MockPurchasePriceFn("2.000000", nil))

	aggID := uuid.New()
	opID := uuid.New()
	maxMarkup := "100.00"

	err := svc.SetTariff(context.Background(), &domain.AggregatorTariff{
		AggregatorID:    aggID,
		SubAccountID:    nil,
		OperatorID:      opID,
		SenderCategory:  "shared",
		PricePerSegment: "3.500000",
	}, &maxMarkup)
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run test — verify FAIL**

```bash
go test ./internal/services/aggregator/service/... -run TestTariffService -v
```
Expected: FAIL — `NewTariffService` not defined

- [ ] **Step 3: Write tariff_service.go**

```go
// internal/services/aggregator/service/tariff_service.go
package service

import (
	"context"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
)

// PurchasePriceFn returns the platform purchase tariff price per segment for a given operator+category.
type PurchasePriceFn func(ctx context.Context, operatorID uuid.UUID, senderCategory string) (string, error)

// MockPurchasePriceFn returns a fixed price for tests.
func MockPurchasePriceFn(price string, err error) PurchasePriceFn {
	return func(ctx context.Context, operatorID uuid.UUID, senderCategory string) (string, error) {
		return price, err
	}
}

// TariffRepo is the minimal interface TariffService needs.
type TariffRepo interface {
	Create(ctx context.Context, t *domain.AggregatorTariff) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.AggregatorTariff, error)
	Update(ctx context.Context, t *domain.AggregatorTariff) error
	Delete(ctx context.Context, id, aggregatorID uuid.UUID) error
	LookupTariff(ctx context.Context, aggregatorID, subAccountID, operatorID uuid.UUID, senderCategory string) (*domain.AggregatorTariff, error)
	List(ctx context.Context, aggregatorID uuid.UUID, subAccountID *uuid.UUID, operatorID *uuid.UUID, defaultsOnly bool, limit, offset int) ([]*domain.AggregatorTariff, int, error)
}

// TariffService manages aggregator tariffs and enforces markup limits.
type TariffService struct {
	repo          TariffRepo
	purchasePrice PurchasePriceFn
}

func NewTariffService(repo TariffRepo, purchasePrice PurchasePriceFn) *TariffService {
	return &TariffService{repo: repo, purchasePrice: purchasePrice}
}

// SetTariff creates or updates a tariff. maxMarkupPercent may be nil (no cap).
func (s *TariffService) SetTariff(ctx context.Context, t *domain.AggregatorTariff, maxMarkupPercent *string) error {
	purchasePrice, err := s.purchasePrice(ctx, t.OperatorID, t.SenderCategory)
	if err != nil {
		return fmt.Errorf("lookup purchase price: %w", err)
	}

	if err := s.validatePrice(t.PricePerSegment, purchasePrice, maxMarkupPercent); err != nil {
		return err
	}

	t.Active = true
	if t.ID == uuid.Nil {
		return s.repo.Create(ctx, t)
	}
	return s.repo.Update(ctx, t)
}

func (s *TariffService) DeleteTariff(ctx context.Context, id, aggregatorID uuid.UUID) error {
	return s.repo.Delete(ctx, id, aggregatorID)
}

func (s *TariffService) ListTariffs(ctx context.Context, aggregatorID uuid.UUID, subAccountID *uuid.UUID, operatorID *uuid.UUID, defaultsOnly bool, limit, offset int) ([]*domain.AggregatorTariff, int, error) {
	return s.repo.List(ctx, aggregatorID, subAccountID, operatorID, defaultsOnly, limit, offset)
}

func (s *TariffService) LookupTariff(ctx context.Context, aggregatorID, subAccountID, operatorID uuid.UUID, senderCategory string) (*domain.AggregatorTariff, error) {
	return s.repo.LookupTariff(ctx, aggregatorID, subAccountID, operatorID, senderCategory)
}

func (s *TariffService) validatePrice(price, purchasePrice string, maxMarkupPercent *string) error {
	p, _, err := big.ParseFloat(price, 10, 128, big.ToNearestEven)
	if err != nil {
		return fmt.Errorf("invalid price: %w", err)
	}
	purchase, _, err := big.ParseFloat(purchasePrice, 10, 128, big.ToNearestEven)
	if err != nil {
		return fmt.Errorf("invalid purchase price: %w", err)
	}

	if p.Cmp(purchase) < 0 {
		return domain.ErrTariffBelowPurchase
	}

	if maxMarkupPercent != nil {
		markup, _, err := big.ParseFloat(*maxMarkupPercent, 10, 128, big.ToNearestEven)
		if err != nil {
			return fmt.Errorf("invalid max_markup_percent: %w", err)
		}
		// maxPrice = purchase * (1 + markup/100)
		hundred := new(big.Float).SetFloat64(100)
		factor := new(big.Float).Add(
			new(big.Float).SetFloat64(1),
			new(big.Float).Quo(markup, hundred),
		)
		maxPrice := new(big.Float).Mul(purchase, factor)
		if p.Cmp(maxPrice) > 0 {
			return domain.ErrTariffExceedsMaxMarkup
		}
	}

	return nil
}
```

- [ ] **Step 4: Run test — verify PASS**

```bash
go test ./internal/services/aggregator/service/... -run TestTariffService -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/aggregator/service/tariff_service.go \
        internal/services/aggregator/service/tariff_service_test.go
git commit -m "feat(aggregator): add TariffService with markup validation"
```

---

## Task 8: Billing Proto — ChargeMessageDual & RefundMessageDual

**Files:**
- Modify: `api/proto/billing/billing.proto`
- Modify: `scripts/generate-proto.sh` — add billing + tarification + aggregator
- Regenerate: `api/proto/billingv1/`

- [ ] **Step 1: Add RPCs and messages to billing.proto**

In `api/proto/billing/billing.proto`, add to the `BillingService` service block (after `ListBalances`):

```protobuf
  // ChargeMessageDual атомарно списывает с виртуального баланса субаккаунта и реального баланса агрегатора
  rpc ChargeMessageDual(ChargeMessageDualRequest) returns (ChargeMessageDualResponse);

  // RefundMessageDual атомарно возвращает средства на оба баланса
  rpc RefundMessageDual(RefundMessageDualRequest) returns (RefundMessageDualResponse);
```

Add message definitions at the end of the file:

```protobuf
// ChargeMessageDualRequest запрос на двойное списание
message ChargeMessageDualRequest {
  string sub_account_id = 1;
  string aggregator_id = 2;
  string sub_account_amount = 3;   // сумма для виртуального баланса субаккаунта
  string aggregator_amount = 4;    // сумма для реального баланса агрегатора
  string message_id = 5;
  string currency = 6;
  string description = 7;
}

// ChargeMessageDualResponse ответ на двойное списание
message ChargeMessageDualResponse {
  bool success = 1;
  string sub_account_transaction_id = 2;
  string aggregator_transaction_id = 3;
  string sub_account_new_balance = 4;
  string aggregator_new_balance = 5;
  string error = 6;
  string rejection_reason = 7;  // insufficient_sub_balance | insufficient_agg_balance | frozen
}

// RefundMessageDualRequest запрос на двойной возврат
message RefundMessageDualRequest {
  string sub_account_id = 1;
  string aggregator_id = 2;
  string sub_account_amount = 3;
  string aggregator_amount = 4;
  string message_id = 5;
  string currency = 6;
  string description = 7;
}

// RefundMessageDualResponse ответ на двойной возврат
message RefundMessageDualResponse {
  bool success = 1;
  string sub_account_transaction_id = 2;
  string aggregator_transaction_id = 3;
  string error = 4;
}
```

- [ ] **Step 2: Add aggregator + tarification to generate-proto.sh**

In `scripts/generate-proto.sh`, add these lines to the `proto_files` array:

```bash
    "api/proto/tarification/tarification.proto:api/proto/tarificationv1"
    "api/proto/aggregator/aggregator.proto:api/proto/aggregatorv1"
```

- [ ] **Step 3: Regenerate billing proto**

```bash
bash scripts/generate-proto.sh
```
Expected: all protos regenerated successfully

- [ ] **Step 4: Build check**

```bash
go build ./api/proto/billingv1/...
```
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add api/proto/billing/billing.proto \
        api/proto/billingv1/ \
        scripts/generate-proto.sh
git commit -m "feat(proto): add ChargeMessageDual and RefundMessageDual to billing service"
```

---

## Task 9: Billing Service — ChargeMessageDual & RefundMessageDual

**Files:**
- Modify: `internal/services/billing/application/billing_service.go`

- [ ] **Step 1: Write failing test**

```go
// internal/services/billing/application/billing_service_dual_test.go
package application_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/billing/application"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
)

func TestBillingService_ChargeMessageDual_InsufficientSubBalance(t *testing.T) {
	svc, mockAccRepo, mockTxRepo := newTestBillingService()

	subID := uuid.New()
	aggID := uuid.New()

	mockAccRepo.SetBalance(subID, "1.000000") // sub has only 1
	mockAccRepo.SetBalance(aggID, "100.000000")

	_, err := svc.ChargeMessageDual(context.Background(),
		subID, aggID,
		"5.000000",  // sub charge > sub balance
		"3.000000",
		uuid.New(), "RUB", "test")
	require.ErrorIs(t, err, domain.ErrInsufficientBalance)
	_ = mockTxRepo
}

func TestBillingService_ChargeMessageDual_Success(t *testing.T) {
	svc, mockAccRepo, _ := newTestBillingService()

	subID := uuid.New()
	aggID := uuid.New()
	msgID := uuid.New()

	mockAccRepo.SetBalance(subID, "100.000000")
	mockAccRepo.SetBalance(aggID, "50.000000")

	subTx, aggTx, err := svc.ChargeMessageDual(context.Background(),
		subID, aggID, "5.000000", "3.000000", msgID, "RUB", "test")
	require.NoError(t, err)
	require.NotNil(t, subTx)
	require.NotNil(t, aggTx)
	require.Equal(t, "95.000000", mockAccRepo.GetBalance(subID))
	require.Equal(t, "47.000000", mockAccRepo.GetBalance(aggID))
}
```

- [ ] **Step 2: Run test — verify FAIL**

```bash
go test ./internal/services/billing/application/... -run TestBillingService_ChargeMessageDual -v
```
Expected: FAIL — `ChargeMessageDual` not defined

- [ ] **Step 3: Add ChargeMessageDual and RefundMessageDual to billing_service.go**

Add these methods to `BillingService` in `internal/services/billing/application/billing_service.go`:

```go
// ChargeMessageDual атомарно списывает с виртуального баланса субаккаунта и реального баланса агрегатора.
// Оба счёта блокируются в одной DB-транзакции в строго упорядоченном порядке (по UUID) для предотвращения deadlock.
func (s *BillingService) ChargeMessageDual(
	ctx context.Context,
	subAccountID, aggregatorID uuid.UUID,
	subAmount, aggAmount string,
	messageID uuid.UUID,
	currency, description string,
) (*domain.Transaction, *domain.Transaction, error) {
	type txRepo interface {
		BeginTx(ctx context.Context) (*sqlx.Tx, error)
		GetByClientIDForUpdate(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID) (*domain.Account, error)
		UpdateBalanceTx(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID, newBalance string) error
	}
	type txTxnRepo interface {
		CreateTx(ctx context.Context, tx *sqlx.Tx, transaction *domain.Transaction) error
	}

	accRepo, ok := s.accountRepo.(txRepo)
	if !ok {
		return nil, nil, fmt.Errorf("account repository does not support transactions")
	}
	txnRepo, _ := s.transactionRepo.(txTxnRepo)

	tx, err := accRepo.BeginTx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin dual charge tx: %w", err)
	}
	defer tx.Rollback()

	// Lock accounts in consistent UUID order to prevent deadlocks
	first, second := subAccountID, aggregatorID
	firstAmt, secondAmt := subAmount, aggAmount
	if subAccountID.String() > aggregatorID.String() {
		first, second = aggregatorID, subAccountID
		firstAmt, secondAmt = aggAmount, subAmount
	}

	firstAcc, err := accRepo.GetByClientIDForUpdate(ctx, tx, first)
	if err != nil {
		return nil, nil, fmt.Errorf("lock first account: %w", err)
	}
	secondAcc, err := accRepo.GetByClientIDForUpdate(ctx, tx, second)
	if err != nil {
		return nil, nil, fmt.Errorf("lock second account: %w", err)
	}

	// Re-map to semantic names
	var subAcc, aggAcc *domain.Account
	if first == subAccountID {
		subAcc, aggAcc = firstAcc, secondAcc
		_ = firstAmt
		_ = secondAmt
	} else {
		subAcc, aggAcc = secondAcc, firstAcc
	}

	if subAcc.Frozen || aggAcc.Frozen {
		return nil, nil, domain.ErrAccountFrozen
	}

	newSubBalance, err := s.subtract(subAcc.Balance, subAmount)
	if err != nil || s.isNegative(newSubBalance) {
		return nil, nil, domain.ErrInsufficientBalance
	}
	newAggBalance, err := s.subtract(aggAcc.Balance, aggAmount)
	if err != nil || s.isNegative(newAggBalance) {
		return nil, nil, domain.ErrInsufficientBalance
	}

	if err := accRepo.UpdateBalanceTx(ctx, tx, subAccountID, newSubBalance); err != nil {
		return nil, nil, fmt.Errorf("update sub balance: %w", err)
	}
	if err := accRepo.UpdateBalanceTx(ctx, tx, aggregatorID, newAggBalance); err != nil {
		return nil, nil, fmt.Errorf("update agg balance: %w", err)
	}

	subTx := domain.NewTransaction(subAccountID, domain.TransactionTypeCharge, subAmount, subAcc.Balance, newSubBalance, currency).
		WithMessageID(messageID).WithDescription(description)
	aggTx := domain.NewTransaction(aggregatorID, domain.TransactionTypeCharge, aggAmount, aggAcc.Balance, newAggBalance, currency).
		WithMessageID(messageID).WithDescription(description + " (aggregator)")

	if txnRepo != nil {
		if err := txnRepo.CreateTx(ctx, tx, subTx); err != nil {
			return nil, nil, fmt.Errorf("create sub transaction: %w", err)
		}
		if err := txnRepo.CreateTx(ctx, tx, aggTx); err != nil {
			return nil, nil, fmt.Errorf("create agg transaction: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit dual charge: %w", err)
	}

	return subTx, aggTx, nil
}

// RefundMessageDual атомарно возвращает средства на виртуальный баланс субаккаунта и реальный баланс агрегатора.
func (s *BillingService) RefundMessageDual(
	ctx context.Context,
	subAccountID, aggregatorID uuid.UUID,
	subAmount, aggAmount string,
	messageID uuid.UUID,
	currency, description string,
) (*domain.Transaction, *domain.Transaction, error) {
	type txRepo interface {
		BeginTx(ctx context.Context) (*sqlx.Tx, error)
		GetByClientIDForUpdate(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID) (*domain.Account, error)
		UpdateBalanceTx(ctx context.Context, tx *sqlx.Tx, clientID uuid.UUID, newBalance string) error
	}
	type txTxnRepo interface {
		CreateTx(ctx context.Context, tx *sqlx.Tx, transaction *domain.Transaction) error
	}

	accRepo, ok := s.accountRepo.(txRepo)
	if !ok {
		return nil, nil, fmt.Errorf("account repository does not support transactions")
	}
	txnRepo, _ := s.transactionRepo.(txTxnRepo)

	tx, err := accRepo.BeginTx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin dual refund tx: %w", err)
	}
	defer tx.Rollback()

	// Lock in consistent order
	first, second := subAccountID, aggregatorID
	if subAccountID.String() > aggregatorID.String() {
		first, second = aggregatorID, subAccountID
	}

	firstAcc, err := accRepo.GetByClientIDForUpdate(ctx, tx, first)
	if err != nil {
		return nil, nil, fmt.Errorf("lock first account for refund: %w", err)
	}
	secondAcc, err := accRepo.GetByClientIDForUpdate(ctx, tx, second)
	if err != nil {
		return nil, nil, fmt.Errorf("lock second account for refund: %w", err)
	}

	var subAcc, aggAcc *domain.Account
	if first == subAccountID {
		subAcc, aggAcc = firstAcc, secondAcc
	} else {
		subAcc, aggAcc = secondAcc, firstAcc
	}

	newSubBalance, err := s.add(subAcc.Balance, subAmount)
	if err != nil {
		return nil, nil, fmt.Errorf("calculate sub refund: %w", err)
	}
	newAggBalance, err := s.add(aggAcc.Balance, aggAmount)
	if err != nil {
		return nil, nil, fmt.Errorf("calculate agg refund: %w", err)
	}

	if err := accRepo.UpdateBalanceTx(ctx, tx, subAccountID, newSubBalance); err != nil {
		return nil, nil, fmt.Errorf("update sub balance for refund: %w", err)
	}
	if err := accRepo.UpdateBalanceTx(ctx, tx, aggregatorID, newAggBalance); err != nil {
		return nil, nil, fmt.Errorf("update agg balance for refund: %w", err)
	}

	subTx := domain.NewTransaction(subAccountID, domain.TransactionTypeCredit, subAmount, subAcc.Balance, newSubBalance, currency).
		WithMessageID(messageID).WithDescription(description)
	aggTx := domain.NewTransaction(aggregatorID, domain.TransactionTypeCredit, aggAmount, aggAcc.Balance, newAggBalance, currency).
		WithMessageID(messageID).WithDescription(description + " (aggregator)")

	if txnRepo != nil {
		if err := txnRepo.CreateTx(ctx, tx, subTx); err != nil {
			return nil, nil, fmt.Errorf("create sub refund transaction: %w", err)
		}
		if err := txnRepo.CreateTx(ctx, tx, aggTx); err != nil {
			return nil, nil, fmt.Errorf("create agg refund transaction: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit dual refund: %w", err)
	}

	return subTx, aggTx, nil
}
```

- [ ] **Step 4: Run test — verify PASS**

```bash
go test ./internal/services/billing/application/... -run TestBillingService_ChargeMessageDual -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/billing/application/billing_service.go \
        internal/services/billing/application/billing_service_dual_test.go
git commit -m "feat(billing): add ChargeMessageDual and RefundMessageDual atomic methods"
```

---

## Task 10: Billing gRPC Server — New Handlers

**Files:**
- Modify: `internal/services/billing/grpc/server.go`

- [ ] **Step 1: Add ChargeMessageDual handler to server.go**

Add these methods to the billing gRPC `Server`:

```go
// ChargeMessageDual атомарно списывает с двух аккаунтов
func (s *Server) ChargeMessageDual(ctx context.Context, req *billingv1.ChargeMessageDualRequest) (*billingv1.ChargeMessageDualResponse, error) {
	if req.SubAccountId == "" || req.AggregatorId == "" {
		return nil, status.Error(codes.InvalidArgument, "sub_account_id and aggregator_id are required")
	}
	if req.SubAccountAmount == "" || req.AggregatorAmount == "" {
		return nil, status.Error(codes.InvalidArgument, "amounts are required")
	}
	if req.MessageId == "" {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}

	subID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sub_account_id")
	}
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid aggregator_id")
	}
	msgID, err := uuid.Parse(req.MessageId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid message_id")
	}

	currency := req.Currency
	if currency == "" {
		currency = "RUB"
	}
	description := req.Description
	if description == "" {
		description = "dual charge for sub-account SMS"
	}

	subTx, aggTx, err := s.billingService.ChargeMessageDual(ctx, subID, aggID,
		req.SubAccountAmount, req.AggregatorAmount, msgID, currency, description)
	if err != nil {
		if errors.Is(err, domain.ErrInsufficientBalance) {
			return &billingv1.ChargeMessageDualResponse{
				Success:         false,
				RejectionReason: "insufficient_sub_balance",
				Error:           err.Error(),
			}, nil
		}
		if errors.Is(err, domain.ErrAccountFrozen) {
			return &billingv1.ChargeMessageDualResponse{
				Success:         false,
				RejectionReason: "frozen",
				Error:           err.Error(),
			}, nil
		}
		s.logger.Error().Err(err).Msg("dual charge failed")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &billingv1.ChargeMessageDualResponse{
		Success:                   true,
		SubAccountTransactionId:   subTx.ID.String(),
		AggregatorTransactionId:   aggTx.ID.String(),
	}, nil
}

// RefundMessageDual атомарно возвращает средства на оба аккаунта
func (s *Server) RefundMessageDual(ctx context.Context, req *billingv1.RefundMessageDualRequest) (*billingv1.RefundMessageDualResponse, error) {
	if req.SubAccountId == "" || req.AggregatorId == "" {
		return nil, status.Error(codes.InvalidArgument, "sub_account_id and aggregator_id are required")
	}

	subID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sub_account_id")
	}
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid aggregator_id")
	}
	msgID, err := uuid.Parse(req.MessageId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid message_id")
	}

	currency := req.Currency
	if currency == "" {
		currency = "RUB"
	}
	description := req.Description
	if description == "" {
		description = "dual refund for failed sub-account SMS"
	}

	subTx, aggTx, err := s.billingService.RefundMessageDual(ctx, subID, aggID,
		req.SubAccountAmount, req.AggregatorAmount, msgID, currency, description)
	if err != nil {
		s.logger.Error().Err(err).Msg("dual refund failed")
		return &billingv1.RefundMessageDualResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &billingv1.RefundMessageDualResponse{
		Success:                   true,
		SubAccountTransactionId:   subTx.ID.String(),
		AggregatorTransactionId:   aggTx.ID.String(),
	}, nil
}
```

- [ ] **Step 2: Build check**

```bash
go build ./internal/services/billing/...
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/services/billing/grpc/server.go
git commit -m "feat(billing): add ChargeMessageDual and RefundMessageDual gRPC handlers"
```

---

## Task 11: Tarification Proto — Extend TarifyMessageResponse

**Files:**
- Modify: `api/proto/tarification/tarification.proto`
- Regenerate: `api/proto/tarificationv1/`

- [ ] **Step 1: Extend TarifyMessageResponse in tarification.proto**

Add three fields to the existing `TarifyMessageResponse` message (keep existing fields unchanged):

```protobuf
message TarifyMessageResponse {
  bool approved = 1;
  string total_amount = 2;
  string currency = 3;
  string strategy = 4;
  string tariff_plan_id = 5;
  string rejection_reason = 6;
  bool threshold_crossed = 7;
  string recalc_amount = 8;
  // Dual charge fields (populated only when client is a sub-account)
  bool is_dual_charge = 9;
  string aggregator_id = 10;
  string aggregator_amount = 11;
}
```

- [ ] **Step 2: Regenerate**

```bash
bash scripts/generate-proto.sh
```
Expected: successful

- [ ] **Step 3: Build check**

```bash
go build ./api/proto/tarificationv1/...
```
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add api/proto/tarification/tarification.proto api/proto/tarificationv1/
git commit -m "feat(proto): extend TarifyMessageResponse with dual charge fields"
```

---

## Task 12: Tarification Service — Sub-Account Branch

**Files:**
- Modify: `internal/services/tarification/application/saga.go`
- Modify: `internal/services/tarification/application/tarification_service.go`

- [ ] **Step 1: Add ChargeDual to saga.go**

Add this method to `SagaOrchestrator` in `saga.go`:

```go
// DualChargeResult результат двойного списания
type DualChargeResult struct {
	SubAccountTransactionID string
	AggregatorTransactionID string
	Success                 bool
	RejectionReason         string
	Error                   string
}

// ChargeDual вызывает ChargeMessageDual в billing-service
func (s *SagaOrchestrator) ChargeDual(ctx context.Context, subAccountID, aggregatorID, messageID, subAmount, aggAmount, currency, description string) (*DualChargeResult, error) {
	resp, err := s.billingClient.ChargeMessageDual(ctx, &billingv1.ChargeMessageDualRequest{
		SubAccountId:     subAccountID,
		AggregatorId:     aggregatorID,
		SubAccountAmount: subAmount,
		AggregatorAmount: aggAmount,
		MessageId:        messageID,
		Currency:         currency,
		Description:      description,
	})
	if err != nil {
		return nil, fmt.Errorf("dual billing charge failed: %w", err)
	}
	return &DualChargeResult{
		SubAccountTransactionID: resp.SubAccountTransactionId,
		AggregatorTransactionID: resp.AggregatorTransactionId,
		Success:                 resp.Success,
		RejectionReason:         resp.RejectionReason,
		Error:                   resp.Error,
	}, nil
}
```

- [ ] **Step 2: Add account info cache type and sub-account branch to tarification_service.go**

First, add a new type and constructor parameter at the top of `tarification_service.go`:

```go
// AccountInfo содержит тип аккаунта и ID родителя, кешируется в Redis
type AccountInfo struct {
	AccountType    string
	ParentClientID *uuid.UUID
}

// ClientAccountRepo минимальный интерфейс для получения типа аккаунта
type ClientAccountRepo interface {
	GetAccountType(ctx context.Context, clientID uuid.UUID) (*AccountInfo, error)
}

// AggregatorTariffLookup минимальный интерфейс для поиска тарифа агрегатора
type AggregatorTariffLookup interface {
	LookupTariff(ctx context.Context, aggregatorID, subAccountID, operatorID uuid.UUID, senderCategory string) (pricePerSegment string, err error)
}
```

Add two new optional fields to `TarificationService`:

```go
type TarificationService struct {
	// ... existing fields ...
	clientRepo    ClientAccountRepo      // nil until wired (optional: sub-account support)
	aggTariffLookup AggregatorTariffLookup // nil until wired (optional: sub-account support)
}
```

Add wiring methods:

```go
// SetClientAccountRepo wires sub-account support into tarification.
func (s *TarificationService) SetClientAccountRepo(repo ClientAccountRepo) {
	s.clientRepo = repo
}

// SetAggregatorTariffLookup wires aggregator tariff lookup.
func (s *TarificationService) SetAggregatorTariffLookup(lookup AggregatorTariffLookup) {
	s.aggTariffLookup = lookup
}
```

- [ ] **Step 3: Add sub-account branch at the start of TarifyMessage**

In `TarifyMessage()`, after the idempotency check (after `if existing != nil { return ... }`), add:

```go
	// Sub-account branch: dual deduction when client is a sub-account
	if s.clientRepo != nil && s.aggTariffLookup != nil {
		info, infoErr := s.clientRepo.GetAccountType(ctx, req.ClientID)
		if infoErr == nil && info != nil && info.AccountType == "sub_account" && info.ParentClientID != nil {
			return s.tarifySubAccount(ctx, req, info.ParentClientID)
		}
	}
```

- [ ] **Step 4: Add tarifySubAccount method**

Add this method to `TarificationService` in `tarification_service.go`:

```go
// tarifySubAccount handles tarification for a sub-account message.
// It performs dual deduction: sub-account virtual balance + aggregator real balance.
func (s *TarificationService) tarifySubAccount(ctx context.Context, req *TarifyMessageRequest, aggregatorID *uuid.UUID) (*TarifyMessageResponse, error) {
	// 1. Determine sender category
	category, err := s.determineSenderCategory(ctx, req.ClientID, req.OperatorID, req.SenderName)
	if err != nil {
		return nil, fmt.Errorf("sub-account sender category: %w", err)
	}

	// 2. Lookup aggregator tariff (personal → default → fallback to purchase)
	var subAccountPrice string
	aggTariff, tariffErr := s.aggTariffLookup.LookupTariff(ctx, *aggregatorID, req.ClientID, req.OperatorID, string(category))
	if tariffErr != nil {
		// Fallback: use aggregator's purchase tariff (margin = 0)
		purchasePlan, planErr := s.planRepo.GetActiveByOperatorAndCategory(ctx, req.OperatorID, category)
		if planErr != nil {
			return &TarifyMessageResponse{Approved: false, RejectionReason: domain.ErrNoActiveTariffPlan.Error()}, nil
		}
		purchasePeriod, periodErr := s.periodRepo.GetActiveByPlanID(ctx, purchasePlan.ID, time.Now())
		if periodErr != nil {
			return &TarifyMessageResponse{Approved: false, RejectionReason: "no active tariff period"}, nil
		}
		tiers, tiersErr := s.tierRepo.ListByPeriodID(ctx, purchasePeriod.ID)
		if tiersErr != nil || len(tiers) == 0 {
			return &TarifyMessageResponse{Approved: false, RejectionReason: "no tariff tiers"}, nil
		}
		subAccountPrice = tiers[0].PricePerSegment // first tier as fallback
	} else {
		subAccountPrice = aggTariff
	}

	// 3. Lookup aggregator purchase tariff (for dual deduction)
	purchasePlan, err := s.planRepo.GetActiveByOperatorAndCategory(ctx, req.OperatorID, category)
	if err != nil {
		return &TarifyMessageResponse{Approved: false, RejectionReason: domain.ErrNoActiveTariffPlan.Error()}, nil
	}
	purchasePeriod, err := s.periodRepo.GetActiveByPlanID(ctx, purchasePlan.ID, time.Now())
	if err != nil {
		return &TarifyMessageResponse{Approved: false, RejectionReason: "no active purchase period"}, nil
	}
	tiers, err := s.tierRepo.ListByPeriodID(ctx, purchasePeriod.ID)
	if err != nil || len(tiers) == 0 {
		return &TarifyMessageResponse{Approved: false, RejectionReason: "no purchase tiers"}, nil
	}
	aggregatorPrice := tiers[0].PricePerSegment

	// 4. Calculate amounts
	segments := new(big.Float).SetInt64(int64(req.SegmentCount))
	subPrice, _, _ := big.ParseFloat(subAccountPrice, 10, 128, big.ToNearestEven)
	aggPrice, _, _ := big.ParseFloat(aggregatorPrice, 10, 128, big.ToNearestEven)
	subTotal := new(big.Float).Mul(subPrice, segments)
	aggTotal := new(big.Float).Mul(aggPrice, segments)

	subAmountStr := subTotal.Text('f', 6)
	aggAmountStr := aggTotal.Text('f', 6)

	// 5. Dual charge via saga
	result, err := s.saga.ChargeDual(ctx,
		req.ClientID.String(), aggregatorID.String(), req.MessageID.String(),
		subAmountStr, aggAmountStr, purchasePlan.Currency,
		fmt.Sprintf("SMS charge sub-account %s", req.MessageID))
	if err != nil {
		return nil, fmt.Errorf("dual charge failed: %w", err)
	}
	if !result.Success {
		return &TarifyMessageResponse{Approved: false, RejectionReason: "insufficient funds"}, nil
	}

	// 6. Write tarification_log for sub-account
	marginF := new(big.Float).Sub(subTotal, aggTotal)

	logEntry := &domain.TarificationLog{
		ID:             uuid.New(),
		ClientID:       req.ClientID,
		MessageID:      req.MessageID,
		OperatorID:     req.OperatorID,
		SenderCategory: string(category),
		Strategy:       string(domain.StrategyFixed),
		TariffPlanID:   purchasePlan.ID,
		TariffPeriodID: purchasePeriod.ID,
		SegmentCount:   req.SegmentCount,
		PricePerSegment: subAccountPrice,
		TotalAmount:    subAmountStr,
		IdempotencyKey: req.IdempotencyKey,
	}
	if err := s.logRepo.Create(ctx, logEntry); err != nil {
		log.Warn().Err(err).Msg("failed to write tarification log for sub-account")
	}

	// 7. Write margin log
	if s.marginRepo != nil {
		margin := marginF.Text('f', 6)
		entry := &MarginEntry{
			AggregatorID:    *aggregatorID,
			SubAccountID:    req.ClientID,
			MessageID:       req.MessageID,
			OperatorID:      req.OperatorID,
			SegmentCount:    req.SegmentCount,
			SubAccountPrice: subAccountPrice,
			AggregatorPrice: aggregatorPrice,
			SubAccountTotal: subAmountStr,
			AggregatorTotal: aggAmountStr,
			Margin:          margin,
			IdempotencyKey:  req.IdempotencyKey,
		}
		if err := s.marginRepo.Create(ctx, entry); err != nil {
			log.Warn().Err(err).Msg("failed to write margin log")
		}
	}

	// 8. Publish event
	if s.eventPublisher != nil {
		_ = s.eventPublisher.PublishTarificationResult(ctx, req.ClientID.String(), req.MessageID.String(), subAmountStr, purchasePlan.Currency)
	}

	return &TarifyMessageResponse{
		Approved:       true,
		TotalAmount:    subAmountStr,
		Currency:       purchasePlan.Currency,
		Strategy:       string(domain.StrategyFixed),
		TariffPlanID:   purchasePlan.ID.String(),
		IsDualCharge:   true,
		AggregatorID:   aggregatorID.String(),
		AggregatorAmount: aggAmountStr,
	}, nil
}
```

Add `marginRepo` and `IsDualCharge`/`AggregatorID`/`AggregatorAmount` fields. Add `marginRepo` to `TarificationService` struct:

```go
	marginRepo MarginEntryRepo // optional, wired by SetMarginRepo
```

Define interface and wiring:

```go
type MarginEntryRepo interface {
	Create(ctx context.Context, e *MarginEntry) error
}

type MarginEntry struct {
	AggregatorID    uuid.UUID
	SubAccountID    uuid.UUID
	MessageID       uuid.UUID
	OperatorID      uuid.UUID
	SegmentCount    int
	SubAccountPrice string
	AggregatorPrice string
	SubAccountTotal string
	AggregatorTotal string
	Margin          string
	IdempotencyKey  string
}

func (s *TarificationService) SetMarginRepo(repo MarginEntryRepo) {
	s.marginRepo = repo
}
```

Also extend `TarifyMessageResponse` struct:

```go
type TarifyMessageResponse struct {
	Approved         bool
	TotalAmount      string
	Currency         string
	Strategy         string
	TariffPlanID     string
	RejectionReason  string
	ThresholdCrossed bool
	RecalcAmount     string
	// Dual charge (sub-account only)
	IsDualCharge     bool
	AggregatorID     string
	AggregatorAmount string
}
```

- [ ] **Step 5: Build check**

```bash
go build ./internal/services/tarification/...
```
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add internal/services/tarification/application/saga.go \
        internal/services/tarification/application/tarification_service.go
git commit -m "feat(tarification): add sub-account dual deduction branch in TarifyMessage"
```

---

## Task 13: Tarification gRPC Server — Return Dual Charge Fields

**Files:**
- Modify: `internal/services/tarification/grpc/server.go`

- [ ] **Step 1: Find TarifyMessage handler and add dual fields to response**

In `internal/services/tarification/grpc/server.go`, find the `TarifyMessage` RPC handler. After mapping `resp.RecalcAmount`, add:

```go
	response := &tarificationv1.TarifyMessageResponse{
		Approved:         resp.Approved,
		TotalAmount:      resp.TotalAmount,
		Currency:         resp.Currency,
		Strategy:         resp.Strategy,
		TariffPlanId:     resp.TariffPlanID,
		RejectionReason:  resp.RejectionReason,
		ThresholdCrossed: resp.ThresholdCrossed,
		RecalcAmount:     resp.RecalcAmount,
		IsDualCharge:     resp.IsDualCharge,
		AggregatorId:     resp.AggregatorID,
		AggregatorAmount: resp.AggregatorAmount,
	}
```

- [ ] **Step 2: Build check**

```bash
go build ./internal/services/tarification/...
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/services/tarification/grpc/server.go
git commit -m "feat(tarification): return dual charge fields in TarifyMessage gRPC response"
```

---

## Task 14: Pipeline — Dual Refund Path

**Files:**
- Modify: `internal/pipeline/sender/stage.go`

- [ ] **Step 1: Add billingClient dual refund call**

In `internal/pipeline/sender/stage.go`, in the stage struct, verify `billingClient` field exists (it does). Now find the stored charge state (around line 364-372 where `chargedAmount` is stored) and add dual charge fields:

```go
// After tarification response, around line 364:
chargedAmount = tarifyResp.TotalAmount
chargedCurrency = tarifyResp.Currency
// ADD: store dual charge info
isDualCharge := tarifyResp.IsDualCharge
aggregatorID := tarifyResp.AggregatorId
aggregatorAmount := tarifyResp.AggregatorAmount
```

Find the refund path (around line 425-446) and replace the existing `AddCredits` call with:

```go
if sendErr != nil && routedMsg.RetryCount >= routedMsg.MaxRetries {
	if s.billingClient != nil && chargedAmount != "" {
		if isDualCharge && aggregatorID != "" && aggregatorAmount != "" {
			// Dual refund: return to both sub-account and aggregator
			_, refundErr := s.billingClient.RefundMessageDual(refundCtx, &billingv1.RefundMessageDualRequest{
				SubAccountId:     routedMsg.ClientID.String(),
				AggregatorId:     aggregatorID,
				SubAccountAmount: chargedAmount,
				AggregatorAmount: aggregatorAmount,
				MessageId:        routedMsg.MessageID.String(),
				Currency:         chargedCurrency,
				Description:      fmt.Sprintf("refund: send failed after %d retries", routedMsg.RetryCount),
			})
			if refundErr != nil {
				s.logger.Warn().Err(refundErr).Str("message_id", routedMsg.MessageID.String()).Msg("dual refund failed")
			}
		} else {
			// Standard single refund
			_, refundErr := s.billingClient.AddCredits(refundCtx, &billingv1.AddCreditsRequest{
				ClientId:    routedMsg.ClientID.String(),
				Amount:      chargedAmount,
				Currency:    chargedCurrency,
				Description: fmt.Sprintf("refund: send failed after %d retries", routedMsg.RetryCount),
			})
			if refundErr != nil {
				s.logger.Warn().Err(refundErr).Str("message_id", routedMsg.MessageID.String()).Msg("refund failed")
			}
		}
	}
}
```

- [ ] **Step 2: Build check**

```bash
go build ./internal/pipeline/...
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/sender/stage.go
git commit -m "feat(pipeline): dual refund path for sub-account messages"
```

---

## Task 15: Aggregator Proto — Tariff RPCs

**Files:**
- Modify: `api/proto/aggregator/aggregator.proto`
- Regenerate: `api/proto/aggregatorv1/`

- [ ] **Step 1: Add tariff RPCs to aggregator.proto**

In `api/proto/aggregator/aggregator.proto`, add to the `AggregatorService` service:

```protobuf
  // Tariff management (aggregator sets tariffs for sub-accounts)
  rpc SetSubAccountTariff(SetSubAccountTariffRequest) returns (AggregatorTariff);
  rpc SetDefaultTariff(SetDefaultTariffRequest) returns (AggregatorTariff);
  rpc ListTariffs(ListTariffsRequest) returns (ListTariffsResponse);
  rpc DeleteTariff(DeleteTariffRequest) returns (google.protobuf.Empty);
```

Add message definitions at the end of the file:

```protobuf
message AggregatorTariff {
  string id = 1;
  string aggregator_id = 2;
  string sub_account_id = 3;      // empty string if default tariff
  string operator_id = 4;
  string sender_category = 5;
  string price_per_segment = 6;
  bool active = 7;
  string created_at = 8;
  string updated_at = 9;
  string operator_name = 10;
  string sub_account_name = 11;
}

message SetSubAccountTariffRequest {
  string aggregator_id = 1;
  string sub_account_id = 2;
  string operator_id = 3;
  string sender_category = 4;
  string price_per_segment = 5;
  string tariff_id = 6;           // if set, update existing; if empty, create new
}

message SetDefaultTariffRequest {
  string aggregator_id = 1;
  string operator_id = 2;
  string sender_category = 3;
  string price_per_segment = 4;
  string tariff_id = 5;           // if set, update existing
}

message ListTariffsRequest {
  string aggregator_id = 1;
  string sub_account_id = 2;      // optional filter
  string operator_id = 3;         // optional filter
  bool defaults_only = 4;
  int32 limit = 5;
  int32 offset = 6;
}

message ListTariffsResponse {
  repeated AggregatorTariff tariffs = 1;
  int32 total = 2;
}

message DeleteTariffRequest {
  string aggregator_id = 1;
  string tariff_id = 2;
}
```

- [ ] **Step 2: Regenerate aggregator proto**

```bash
bash scripts/generate-proto.sh
```
Expected: successful

- [ ] **Step 3: Build check**

```bash
go build ./api/proto/aggregatorv1/...
```
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add api/proto/aggregator/aggregator.proto api/proto/aggregatorv1/
git commit -m "feat(proto): add tariff RPCs to AggregatorService"
```

---

## Task 16: Aggregator gRPC Server — Tariff Handlers

**Files:**
- Modify: `internal/services/aggregator/grpc/server.go`

- [ ] **Step 1: Add TariffService to Server struct and constructor**

In `internal/services/aggregator/grpc/server.go`, add `tariffs *service.TariffService` field to the `Server` struct and update `NewServer` to accept it.

Current constructor pattern:
```go
type Server struct {
    aggregatorv1.UnimplementedAggregatorServiceServer
    svc          *service.Service
    clientClient clientv1.ClientServiceClient
}
```

After:
```go
type Server struct {
    aggregatorv1.UnimplementedAggregatorServiceServer
    svc          *service.Service
    clientClient clientv1.ClientServiceClient
    tariffs      *service.TariffService
}

func NewServer(svc *service.Service, clientClient clientv1.ClientServiceClient, tariffs *service.TariffService) *Server {
    return &Server{svc: svc, clientClient: clientClient, tariffs: tariffs}
}
```

- [ ] **Step 2: Add tariff handlers**

Add these methods to `Server`:

```go
func (s *Server) SetSubAccountTariff(ctx context.Context, req *aggregatorv1.SetSubAccountTariffRequest) (*aggregatorv1.AggregatorTariff, error) {
	if req.AggregatorId == "" || req.SubAccountId == "" || req.OperatorId == "" {
		return nil, status.Error(codes.InvalidArgument, "aggregator_id, sub_account_id, operator_id are required")
	}
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid aggregator_id")
	}
	subID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sub_account_id")
	}
	opID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid operator_id")
	}

	profile, err := s.svc.GetProfile(ctx, aggID)
	if err != nil {
		return nil, mapDomainError(err, "get profile")
	}
	var maxMarkup *string
	if profile.MaxMarkupPercent != nil {
		maxMarkup = profile.MaxMarkupPercent
	}

	t := &domain.AggregatorTariff{
		AggregatorID:    aggID,
		SubAccountID:    &subID,
		OperatorID:      opID,
		SenderCategory:  req.SenderCategory,
		PricePerSegment: req.PricePerSegment,
	}
	if req.TariffId != "" {
		t.ID, _ = uuid.Parse(req.TariffId)
	}

	if err := s.tariffs.SetTariff(ctx, t, maxMarkup); err != nil {
		if errors.Is(err, domain.ErrTariffBelowPurchase) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, domain.ErrTariffExceedsMaxMarkup) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return tariffToProto(t), nil
}

func (s *Server) SetDefaultTariff(ctx context.Context, req *aggregatorv1.SetDefaultTariffRequest) (*aggregatorv1.AggregatorTariff, error) {
	if req.AggregatorId == "" || req.OperatorId == "" {
		return nil, status.Error(codes.InvalidArgument, "aggregator_id and operator_id are required")
	}
	aggID, _ := uuid.Parse(req.AggregatorId)
	opID, _ := uuid.Parse(req.OperatorId)

	profile, err := s.svc.GetProfile(ctx, aggID)
	if err != nil {
		return nil, mapDomainError(err, "get profile")
	}

	t := &domain.AggregatorTariff{
		AggregatorID:    aggID,
		OperatorID:      opID,
		SenderCategory:  req.SenderCategory,
		PricePerSegment: req.PricePerSegment,
	}
	if req.TariffId != "" {
		t.ID, _ = uuid.Parse(req.TariffId)
	}

	if err := s.tariffs.SetTariff(ctx, t, profile.MaxMarkupPercent); err != nil {
		if errors.Is(err, domain.ErrTariffBelowPurchase) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, domain.ErrTariffExceedsMaxMarkup) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return tariffToProto(t), nil
}

func (s *Server) ListTariffs(ctx context.Context, req *aggregatorv1.ListTariffsRequest) (*aggregatorv1.ListTariffsResponse, error) {
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid aggregator_id")
	}
	var subID *uuid.UUID
	if req.SubAccountId != "" {
		id, _ := uuid.Parse(req.SubAccountId)
		subID = &id
	}
	var opID *uuid.UUID
	if req.OperatorId != "" {
		id, _ := uuid.Parse(req.OperatorId)
		opID = &id
	}

	limit := int(req.Limit)
	if limit == 0 {
		limit = 50
	}

	tariffs, total, err := s.tariffs.ListTariffs(ctx, aggID, subID, opID, req.DefaultsOnly, limit, int(req.Offset))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protos := make([]*aggregatorv1.AggregatorTariff, 0, len(tariffs))
	for _, t := range tariffs {
		protos = append(protos, tariffToProto(t))
	}
	return &aggregatorv1.ListTariffsResponse{Tariffs: protos, Total: int32(total)}, nil
}

func (s *Server) DeleteTariff(ctx context.Context, req *aggregatorv1.DeleteTariffRequest) (*emptypb.Empty, error) {
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid aggregator_id")
	}
	tariffID, err := uuid.Parse(req.TariffId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tariff_id")
	}
	if err := s.tariffs.DeleteTariff(ctx, tariffID, aggID); err != nil {
		if errors.Is(err, domain.ErrTariffNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &emptypb.Empty{}, nil
}

func tariffToProto(t *domain.AggregatorTariff) *aggregatorv1.AggregatorTariff {
	proto := &aggregatorv1.AggregatorTariff{
		Id:              t.ID.String(),
		AggregatorId:    t.AggregatorID.String(),
		OperatorId:      t.OperatorID.String(),
		SenderCategory:  t.SenderCategory,
		PricePerSegment: t.PricePerSegment,
		Active:          t.Active,
		CreatedAt:       t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       t.UpdatedAt.Format(time.RFC3339),
		OperatorName:    t.OperatorName,
		SubAccountName:  t.SubAccountName,
	}
	if t.SubAccountID != nil {
		proto.SubAccountId = t.SubAccountID.String()
	}
	return proto
}
```

- [ ] **Step 3: Build check**

```bash
go build ./internal/services/aggregator/...
```
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/services/aggregator/grpc/server.go
git commit -m "feat(aggregator): add tariff gRPC handlers to aggregator server"
```

---

## Task 17: HTTP Handlers — aggregator_tariffs.go

**Files:**
- Create: `internal/gateway/portal/handlers/aggregator_tariffs.go`

- [ ] **Step 1: Write aggregator_tariffs.go**

```go
// internal/gateway/portal/handlers/aggregator_tariffs.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	aggregatorv1 "github.com/smpp-server/smpp-server/api/proto/aggregatorv1"
)

// AggregatorTariffsHandlers handles /api/v1/aggregator/tariffs endpoints
type AggregatorTariffsHandlers struct {
	aggClient aggregatorv1.AggregatorServiceClient
}

func NewAggregatorTariffsHandlers(aggClient aggregatorv1.AggregatorServiceClient) *AggregatorTariffsHandlers {
	return &AggregatorTariffsHandlers{aggClient: aggClient}
}

// ListTariffs GET /api/v1/aggregator/tariffs
func (h *AggregatorTariffsHandlers) ListTariffs(w http.ResponseWriter, r *http.Request) {
	aggregatorID := r.Context().Value("client_id").(string)

	req := &aggregatorv1.ListTariffsRequest{
		AggregatorId:  aggregatorID,
		SubAccountId:  r.URL.Query().Get("sub_account_id"),
		OperatorId:    r.URL.Query().Get("operator_id"),
		DefaultsOnly:  r.URL.Query().Get("defaults_only") == "true",
		Limit:         50,
	}

	resp, err := h.aggClient.ListTariffs(r.Context(), req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// CreateTariff POST /api/v1/aggregator/tariffs
func (h *AggregatorTariffsHandlers) CreateTariff(w http.ResponseWriter, r *http.Request) {
	aggregatorID := r.Context().Value("client_id").(string)

	var body struct {
		SubAccountID   string `json:"sub_account_id"`
		OperatorID     string `json:"operator_id"`
		SenderCategory string `json:"sender_category"`
		PricePerSegment string `json:"price_per_segment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.OperatorID == "" || body.SenderCategory == "" || body.PricePerSegment == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "operator_id, sender_category, price_per_segment are required"})
		return
	}

	req := &aggregatorv1.SetSubAccountTariffRequest{
		AggregatorId:    aggregatorID,
		SubAccountId:    body.SubAccountID,
		OperatorId:      body.OperatorID,
		SenderCategory:  body.SenderCategory,
		PricePerSegment: body.PricePerSegment,
	}
	tariff, err := h.aggClient.SetSubAccountTariff(r.Context(), req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, tariff)
}

// UpdateTariff PUT /api/v1/aggregator/tariffs/:id
func (h *AggregatorTariffsHandlers) UpdateTariff(w http.ResponseWriter, r *http.Request) {
	aggregatorID := r.Context().Value("client_id").(string)
	tariffID := mux.Vars(r)["id"]

	var body struct {
		SubAccountID    string `json:"sub_account_id"`
		OperatorID      string `json:"operator_id"`
		SenderCategory  string `json:"sender_category"`
		PricePerSegment string `json:"price_per_segment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	req := &aggregatorv1.SetSubAccountTariffRequest{
		AggregatorId:    aggregatorID,
		SubAccountId:    body.SubAccountID,
		OperatorId:      body.OperatorID,
		SenderCategory:  body.SenderCategory,
		PricePerSegment: body.PricePerSegment,
		TariffId:        tariffID,
	}
	tariff, err := h.aggClient.SetSubAccountTariff(r.Context(), req)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, tariff)
}

// DeleteTariff DELETE /api/v1/aggregator/tariffs/:id
func (h *AggregatorTariffsHandlers) DeleteTariff(w http.ResponseWriter, r *http.Request) {
	aggregatorID := r.Context().Value("client_id").(string)
	tariffID := mux.Vars(r)["id"]

	_, err := h.aggClient.DeleteTariff(r.Context(), &aggregatorv1.DeleteTariffRequest{
		AggregatorId: aggregatorID,
		TariffId:     tariffID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListDefaultTariffs GET /api/v1/aggregator/tariffs/defaults
func (h *AggregatorTariffsHandlers) ListDefaultTariffs(w http.ResponseWriter, r *http.Request) {
	aggregatorID := r.Context().Value("client_id").(string)

	resp, err := h.aggClient.ListTariffs(r.Context(), &aggregatorv1.ListTariffsRequest{
		AggregatorId: aggregatorID,
		DefaultsOnly: true,
		Limit:        100,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// CreateDefaultTariff POST /api/v1/aggregator/tariffs/defaults
func (h *AggregatorTariffsHandlers) CreateDefaultTariff(w http.ResponseWriter, r *http.Request) {
	aggregatorID := r.Context().Value("client_id").(string)

	var body struct {
		OperatorID      string `json:"operator_id"`
		SenderCategory  string `json:"sender_category"`
		PricePerSegment string `json:"price_per_segment"`
		TariffID        string `json:"tariff_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.OperatorID == "" || body.SenderCategory == "" || body.PricePerSegment == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "operator_id, sender_category, price_per_segment are required"})
		return
	}

	tariff, err := h.aggClient.SetDefaultTariff(r.Context(), &aggregatorv1.SetDefaultTariffRequest{
		AggregatorId:    aggregatorID,
		OperatorId:      body.OperatorID,
		SenderCategory:  body.SenderCategory,
		PricePerSegment: body.PricePerSegment,
		TariffId:        body.TariffID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, tariff)
}
```

- [ ] **Step 2: Register routes**

In the portal router file (find it with `grep -rn "aggregator\|/api/v1/aggregator" internal/gateway/portal/ | grep "Route\|router\|mux" | head -10`), add tariff routes in the aggregator section:

```go
tariffHandlers := handlers.NewAggregatorTariffsHandlers(aggClient)
aggRouter.HandleFunc("/tariffs", tariffHandlers.ListTariffs).Methods("GET")
aggRouter.HandleFunc("/tariffs", tariffHandlers.CreateTariff).Methods("POST")
aggRouter.HandleFunc("/tariffs/defaults", tariffHandlers.ListDefaultTariffs).Methods("GET")
aggRouter.HandleFunc("/tariffs/defaults", tariffHandlers.CreateDefaultTariff).Methods("POST")
aggRouter.HandleFunc("/tariffs/{id}", tariffHandlers.UpdateTariff).Methods("PUT")
aggRouter.HandleFunc("/tariffs/{id}", tariffHandlers.DeleteTariff).Methods("DELETE")
```

- [ ] **Step 3: Build check**

```bash
go build ./internal/gateway/portal/...
```
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/aggregator_tariffs.go \
        internal/gateway/portal/
git commit -m "feat(portal): add aggregator tariff HTTP handlers and routes"
```

---

## Task 18: Frontend — TariffsPage

**Files:**
- Create: `portal-frontend/src/pages/aggregator/TariffsPage.tsx`

- [ ] **Step 1: Write TariffsPage.tsx**

```tsx
// portal-frontend/src/pages/aggregator/TariffsPage.tsx
import { useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import * as Tabs from '@radix-ui/react-tabs'
import { useSearchParams } from 'react-router-dom'

interface AggregatorTariff {
  id: string
  sub_account_id: string
  sub_account_name: string
  operator_id: string
  operator_name: string
  sender_category: string
  price_per_segment: string
  active: boolean
}

interface TariffFormData {
  sub_account_id: string
  operator_id: string
  sender_category: string
  price_per_segment: string
}

const SENDER_CATEGORIES = [
  { value: 'shared', label: 'Shared' },
  { value: 'paid_registered', label: 'Paid registered' },
  { value: 'free_registered', label: 'Free registered' },
]

export default function TariffsPage() {
  const [searchParams] = useSearchParams()
  const preFilterSubId = searchParams.get('sub_account_id') ?? ''

  const [tab, setTab] = useState('personal')
  const [tariffs, setTariffs] = useState<AggregatorTariff[]>([])
  const [loading, setLoading] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editTariff, setEditTariff] = useState<AggregatorTariff | null>(null)
  const [form, setForm] = useState<TariffFormData>({
    sub_account_id: preFilterSubId,
    operator_id: '',
    sender_category: 'shared',
    price_per_segment: '',
  })
  const [error, setError] = useState('')

  const isDefaults = tab === 'defaults'

  async function fetchTariffs() {
    setLoading(true)
    const params = new URLSearchParams()
    if (isDefaults) params.set('defaults_only', 'true')
    if (preFilterSubId && !isDefaults) params.set('sub_account_id', preFilterSubId)
    const res = await fetch(`/api/v1/aggregator/tariffs?${params}`)
    const data = await res.json()
    setTariffs(data.tariffs ?? [])
    setLoading(false)
  }

  useState(() => { fetchTariffs() })

  async function handleSave() {
    setError('')
    const url = isDefaults
      ? '/api/v1/aggregator/tariffs/defaults'
      : '/api/v1/aggregator/tariffs'
    const method = editTariff ? 'PUT' : 'POST'
    const path = editTariff ? `${url}/${editTariff.id}` : url
    const body = isDefaults
      ? { operator_id: form.operator_id, sender_category: form.sender_category, price_per_segment: form.price_per_segment }
      : form

    const res = await fetch(method === 'PUT' ? `/api/v1/aggregator/tariffs/${editTariff!.id}` : url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      const data = await res.json()
      setError(data.error ?? 'Failed to save tariff')
      return
    }
    setDialogOpen(false)
    fetchTariffs()
  }

  async function handleDelete(id: string) {
    if (!confirm('Delete this tariff?')) return
    await fetch(`/api/v1/aggregator/tariffs/${id}`, { method: 'DELETE' })
    fetchTariffs()
  }

  function openCreate() {
    setEditTariff(null)
    setForm({ sub_account_id: preFilterSubId, operator_id: '', sender_category: 'shared', price_per_segment: '' })
    setError('')
    setDialogOpen(true)
  }

  function openEdit(t: AggregatorTariff) {
    setEditTariff(t)
    setForm({ sub_account_id: t.sub_account_id, operator_id: t.operator_id, sender_category: t.sender_category, price_per_segment: t.price_per_segment })
    setError('')
    setDialogOpen(true)
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Tariffs</h1>
        <button
          onClick={openCreate}
          className="px-4 py-2 bg-blue-600 text-white rounded-md text-sm hover:bg-blue-700"
        >
          Add tariff
        </button>
      </div>

      <Tabs.Root value={tab} onValueChange={(v) => { setTab(v); setTimeout(fetchTariffs, 0) }}>
        <Tabs.List className="flex border-b mb-4">
          <Tabs.Trigger value="personal" className="px-4 py-2 text-sm font-medium data-[state=active]:border-b-2 data-[state=active]:border-blue-600">
            Personal tariffs
          </Tabs.Trigger>
          <Tabs.Trigger value="defaults" className="px-4 py-2 text-sm font-medium data-[state=active]:border-b-2 data-[state=active]:border-blue-600">
            Default tariffs
          </Tabs.Trigger>
        </Tabs.List>

        <Tabs.Content value="personal">
          {!isDefaults && (
            <p className="text-sm text-gray-500 mb-3">Personal tariffs override defaults for specific sub-accounts.</p>
          )}
          <TariffTable tariffs={tariffs} loading={loading} onEdit={openEdit} onDelete={handleDelete} showSubAccount />
        </Tabs.Content>

        <Tabs.Content value="defaults">
          <p className="text-sm text-gray-500 mb-3">Default tariffs apply to all sub-accounts without a personal tariff.</p>
          <TariffTable tariffs={tariffs} loading={loading} onEdit={openEdit} onDelete={handleDelete} showSubAccount={false} />
        </Tabs.Content>
      </Tabs.Root>

      <Dialog.Root open={dialogOpen} onOpenChange={setDialogOpen}>
        <Dialog.Portal>
          <Dialog.Overlay className="fixed inset-0 bg-black/40" />
          <Dialog.Content className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 bg-white rounded-lg p-6 w-full max-w-md shadow-xl space-y-4">
            <Dialog.Title className="text-lg font-semibold">
              {editTariff ? 'Edit tariff' : 'Add tariff'}
            </Dialog.Title>

            {!isDefaults && (
              <label className="block">
                <span className="text-sm font-medium text-gray-700">Sub-account ID</span>
                <input
                  className="mt-1 w-full border rounded px-3 py-2 text-sm"
                  value={form.sub_account_id}
                  onChange={(e) => setForm({ ...form, sub_account_id: e.target.value })}
                  placeholder="UUID of sub-account"
                />
              </label>
            )}

            <label className="block">
              <span className="text-sm font-medium text-gray-700">Operator ID</span>
              <input
                className="mt-1 w-full border rounded px-3 py-2 text-sm"
                value={form.operator_id}
                onChange={(e) => setForm({ ...form, operator_id: e.target.value })}
                placeholder="UUID of operator"
              />
            </label>

            <label className="block">
              <span className="text-sm font-medium text-gray-700">Sender category</span>
              <select
                className="mt-1 w-full border rounded px-3 py-2 text-sm"
                value={form.sender_category}
                onChange={(e) => setForm({ ...form, sender_category: e.target.value })}
              >
                {SENDER_CATEGORIES.map((c) => (
                  <option key={c.value} value={c.value}>{c.label}</option>
                ))}
              </select>
            </label>

            <label className="block">
              <span className="text-sm font-medium text-gray-700">Price per segment (RUB)</span>
              <input
                className="mt-1 w-full border rounded px-3 py-2 text-sm"
                type="number"
                step="0.01"
                min="0"
                value={form.price_per_segment}
                onChange={(e) => setForm({ ...form, price_per_segment: e.target.value })}
                placeholder="e.g. 3.50"
              />
            </label>

            {error && <p className="text-red-600 text-sm">{error}</p>}

            <div className="flex justify-end gap-2 pt-2">
              <Dialog.Close asChild>
                <button className="px-4 py-2 text-sm border rounded hover:bg-gray-50">Cancel</button>
              </Dialog.Close>
              <button
                onClick={handleSave}
                className="px-4 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700"
              >
                Save
              </button>
            </div>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  )
}

function TariffTable({ tariffs, loading, onEdit, onDelete, showSubAccount }: {
  tariffs: AggregatorTariff[]
  loading: boolean
  onEdit: (t: AggregatorTariff) => void
  onDelete: (id: string) => void
  showSubAccount: boolean
}) {
  if (loading) return <p className="text-gray-500 text-sm">Loading...</p>
  if (!tariffs.length) return <p className="text-gray-500 text-sm">No tariffs yet.</p>

  return (
    <table className="w-full text-sm border-collapse">
      <thead>
        <tr className="border-b text-left text-gray-500">
          {showSubAccount && <th className="pb-2 pr-4">Sub-account</th>}
          <th className="pb-2 pr-4">Operator</th>
          <th className="pb-2 pr-4">Category</th>
          <th className="pb-2 pr-4">Price/segment</th>
          <th className="pb-2" />
        </tr>
      </thead>
      <tbody>
        {tariffs.map((t) => (
          <tr key={t.id} className="border-b hover:bg-gray-50">
            {showSubAccount && <td className="py-2 pr-4">{t.sub_account_name || t.sub_account_id || '—'}</td>}
            <td className="py-2 pr-4">{t.operator_name || t.operator_id}</td>
            <td className="py-2 pr-4">{t.sender_category}</td>
            <td className="py-2 pr-4 font-mono">{t.price_per_segment}</td>
            <td className="py-2 flex gap-2 justify-end">
              <button onClick={() => onEdit(t)} className="text-blue-600 hover:underline text-xs">Edit</button>
              <button onClick={() => onDelete(t.id)} className="text-red-500 hover:underline text-xs">Delete</button>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
```

- [ ] **Step 2: Build check**

```bash
cd portal-frontend && npx tsc --noEmit
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/aggregator/TariffsPage.tsx
git commit -m "feat(portal): add TariffsPage for aggregator tariff management"
```

---

## Task 19: Frontend — TariffsTab (Read-only)

**Files:**
- Modify: `portal-frontend/src/pages/aggregator/tabs/TariffsTab.tsx`

- [ ] **Step 1: Rewrite TariffsTab as read-only with link**

Replace the content of `TariffsTab.tsx` with:

```tsx
// portal-frontend/src/pages/aggregator/tabs/TariffsTab.tsx
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'

interface AggregatorTariff {
  id: string
  operator_name: string
  sender_category: string
  price_per_segment: string
  sub_account_id: string
}

export default function TariffsTab() {
  const { id: subAccountId } = useParams<{ id: string }>()
  const [tariffs, setTariffs] = useState<AggregatorTariff[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!subAccountId) return
    fetch(`/api/v1/aggregator/tariffs?sub_account_id=${subAccountId}`)
      .then((r) => r.json())
      .then((data) => setTariffs(data.tariffs ?? []))
      .finally(() => setLoading(false))
  }, [subAccountId])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="font-medium text-gray-800">Effective tariffs</h3>
        <Link
          to={`/aggregator/tariffs?sub_account_id=${subAccountId}`}
          className="text-sm text-blue-600 hover:underline"
        >
          Manage tariffs →
        </Link>
      </div>

      {loading ? (
        <p className="text-sm text-gray-500">Loading...</p>
      ) : tariffs.length === 0 ? (
        <p className="text-sm text-gray-500">
          No personal tariffs. Default tariffs will apply.{' '}
          <Link to={`/aggregator/tariffs?sub_account_id=${subAccountId}`} className="text-blue-600 hover:underline">
            Add tariffs →
          </Link>
        </p>
      ) : (
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="border-b text-left text-gray-500">
              <th className="pb-2 pr-4">Operator</th>
              <th className="pb-2 pr-4">Category</th>
              <th className="pb-2 pr-4">Price/segment</th>
              <th className="pb-2">Type</th>
            </tr>
          </thead>
          <tbody>
            {tariffs.map((t) => (
              <tr key={t.id} className="border-b hover:bg-gray-50">
                <td className="py-2 pr-4">{t.operator_name || '—'}</td>
                <td className="py-2 pr-4">{t.sender_category}</td>
                <td className="py-2 pr-4 font-mono">{t.price_per_segment}</td>
                <td className="py-2 text-xs text-gray-500">
                  {t.sub_account_id ? 'personal' : 'default'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Build check**

```bash
cd portal-frontend && npx tsc --noEmit
```
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/aggregator/tabs/TariffsTab.tsx
git commit -m "feat(portal): update TariffsTab to read-only with link to TariffsPage"
```

---

## Task 20: Wire Everything + Final Build

- [ ] **Step 1: Wire tariff service into aggregator gRPC server**

Find the aggregator service initialization (likely in `cmd/aggregator-service/main.go` or similar). Add:

```go
tariffRepo := repository.NewTariffRepository(db)
marginRepo := repository.NewMarginRepository(db)
tariffSvc := service.NewTariffService(tariffRepo, func(ctx context.Context, opID uuid.UUID, cat string) (string, error) {
    // Lookup purchase tariff via tarification service client
    resp, err := tarificationClient.TarifyLookup(ctx, &tarificationv1.TarifyLookupRequest{
        // Use existing operator/category lookup
    })
    if err != nil {
        return "0", err
    }
    return resp.CostPerNumber, nil
})
aggGRPCServer := grpc.NewServer(aggSvc, clientClient, tariffSvc)
```

Wire tarification service with sub-account support:

```go
tarificationSvc.SetClientAccountRepo(clientAccountAdapter)  // adapter over client service
tarificationSvc.SetAggregatorTariffLookup(tariffRepo)
tarificationSvc.SetMarginRepo(marginRepo)
```

- [ ] **Step 2: Full project build**

```bash
go build ./...
```
Expected: no errors

- [ ] **Step 3: Run all tests**

```bash
go test ./internal/services/aggregator/... ./internal/services/billing/... ./internal/services/tarification/... -v
```
Expected: all PASS

- [ ] **Step 4: Frontend build**

```bash
cd portal-frontend && npm run build
```
Expected: successful

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat(aggregator-phase2): wire tariff service, margin repo, and sub-account cache into tarification"
```
