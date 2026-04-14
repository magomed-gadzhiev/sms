# Aggregator Sub-Accounts: Phase 1 — Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the aggregator account type with profile management, virtual sub-account balances, sub-account lifecycle (create/list/get/deactivate), and balance transfer between aggregator and sub-accounts.

**Architecture:** `account_type` column replaces `is_reseller` flag on the `clients` table. A new `aggregator_profiles` table holds platform-controlled limits per aggregator. A new `aggregatorv1` gRPC service implements all aggregator business logic. The existing billing service handles the atomic balance deduction; the aggregator service orchestrates it. Phase 2 (tarification), Phase 3 (routing), Phase 4 (analytics), Phase 5 (control) are separate plans.

**Tech Stack:** Go 1.24 + sqlx (postgres), gorilla/mux, protobuf/gRPC, React 19 + TypeScript + Tailwind CSS 4.2, Radix UI, zerolog, testify/assert

---

## File Map

### New files
- `migrations/000094_add_account_type.up.sql` — account_type column, data migration, drop is_reseller
- `migrations/000094_add_account_type.down.sql`
- `migrations/000095_create_aggregator_profiles.up.sql` — aggregator_profiles table
- `migrations/000095_create_aggregator_profiles.down.sql`
- `migrations/000096_add_virtual_balance.up.sql` — is_virtual + allocated_by on accounts, aggregator_audit_log
- `migrations/000096_add_virtual_balance.down.sql`
- `api/proto/aggregator/aggregator.proto` — AggregatorService proto definition
- `api/proto/aggregatorv1/` — generated Go code (via protoc)
- `internal/services/aggregator/domain/models.go` — AggregatorProfile, SubAccountSummary
- `internal/services/aggregator/domain/errors.go` — domain errors
- `internal/services/aggregator/repository/profile_repository.go` — aggregator_profiles CRUD
- `internal/services/aggregator/repository/balance_repository.go` — virtual balance + audit log
- `internal/services/aggregator/service/aggregator_service.go` — business logic
- `internal/services/aggregator/grpc/server.go` — gRPC server implementation
- `internal/gateway/portal/handlers/aggregator_admin.go` — admin HTTP handlers
- `internal/gateway/portal/handlers/aggregator_portal.go` — aggregator HTTP handlers
- `portal-frontend/src/api/aggregator.ts` — API client
- `portal-frontend/src/pages/aggregator/SubAccountsListPage.tsx`
- `portal-frontend/src/pages/aggregator/SubAccountDetailPage.tsx`
- `portal-frontend/src/pages/aggregator/tabs/BalanceTab.tsx`

### Modified files
- `internal/services/client/domain/client.go` — add AccountType field, remove IsReseller
- `internal/services/client/infrastructure/repository/client_repository.go` — use account_type in queries
- `internal/gateway/portal/middleware/session_auth.go` — store account_type in context
- `internal/gateway/portal/router/router.go` — register aggregator routes
- `internal/gateway/portal/clients.go` — add AggregatorClient field
- `portal-frontend/src/components/layout/Sidebar.tsx` — aggregator nav section

---

## Task 1: DB Migration — account_type

**Files:**
- Create: `migrations/000094_add_account_type.up.sql`
- Create: `migrations/000094_add_account_type.down.sql`

- [ ] **Step 1: Write up migration**

```sql
-- migrations/000094_add_account_type.up.sql
ALTER TABLE clients ADD COLUMN account_type VARCHAR(20) NOT NULL DEFAULT 'direct';

UPDATE clients SET account_type = 'aggregator' WHERE is_reseller = true;
UPDATE clients SET account_type = 'sub_account'
  WHERE parent_client_id IS NOT NULL AND is_reseller = false;

ALTER TABLE clients DROP COLUMN is_reseller;

ALTER TABLE clients ADD CONSTRAINT chk_aggregator_no_parent
  CHECK (account_type != 'aggregator' OR parent_client_id IS NULL);
ALTER TABLE clients ADD CONSTRAINT chk_sub_account_has_parent
  CHECK (account_type != 'sub_account' OR parent_client_id IS NOT NULL);
ALTER TABLE clients ADD CONSTRAINT chk_direct_no_parent
  CHECK (account_type != 'direct' OR parent_client_id IS NULL);

CREATE INDEX idx_clients_account_type ON clients(account_type);
```

- [ ] **Step 2: Write down migration**

```sql
-- migrations/000094_add_account_type.down.sql
ALTER TABLE clients DROP CONSTRAINT IF EXISTS chk_aggregator_no_parent;
ALTER TABLE clients DROP CONSTRAINT IF EXISTS chk_sub_account_has_parent;
ALTER TABLE clients DROP CONSTRAINT IF EXISTS chk_direct_no_parent;
DROP INDEX IF EXISTS idx_clients_account_type;

ALTER TABLE clients ADD COLUMN is_reseller BOOLEAN NOT NULL DEFAULT false;
UPDATE clients SET is_reseller = true WHERE account_type = 'aggregator';
ALTER TABLE clients DROP COLUMN account_type;
```

- [ ] **Step 3: Apply migration**

```bash
cd /opt/sms  # or locally with DB_URL set
migrate -path migrations -database "$DATABASE_URL" up 1
```

Expected: `000094/u add_account_type OK`

- [ ] **Step 4: Verify**

```bash
psql "$DATABASE_URL" -c "\d clients" | grep account_type
psql "$DATABASE_URL" -c "SELECT account_type, count(*) FROM clients GROUP BY account_type"
```

- [ ] **Step 5: Commit**

```bash
git add migrations/000094_add_account_type.up.sql migrations/000094_add_account_type.down.sql
git commit -m "feat(aggregator): add account_type column, migrate from is_reseller"
```

---

## Task 2: DB Migration — aggregator_profiles

**Files:**
- Create: `migrations/000095_create_aggregator_profiles.up.sql`
- Create: `migrations/000095_create_aggregator_profiles.down.sql`

- [ ] **Step 1: Write up migration**

```sql
-- migrations/000095_create_aggregator_profiles.up.sql
CREATE TABLE aggregator_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL UNIQUE REFERENCES clients(id) ON DELETE CASCADE,
    max_markup_percent NUMERIC(5,2),
    max_sub_accounts INTEGER NOT NULL DEFAULT 100,
    pool_monthly_limit INTEGER,
    pool_rate_limit_per_second INTEGER,
    allowed_provider_ids UUID[] NOT NULL DEFAULT '{}',
    auto_freeze_sub_accounts BOOLEAN NOT NULL DEFAULT true,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_aggregator_profiles_client_id ON aggregator_profiles(client_id);

-- Create profiles for existing aggregator accounts
INSERT INTO aggregator_profiles (client_id, max_sub_accounts)
SELECT id, COALESCE(max_sub_accounts, 100)
FROM clients
WHERE account_type = 'aggregator'
ON CONFLICT (client_id) DO NOTHING;
```

- [ ] **Step 2: Write down migration**

```sql
-- migrations/000095_create_aggregator_profiles.down.sql
DROP TABLE IF EXISTS aggregator_profiles;
```

- [ ] **Step 3: Apply migration**

```bash
migrate -path migrations -database "$DATABASE_URL" up 1
```

Expected: `000095/u create_aggregator_profiles OK`

- [ ] **Step 4: Commit**

```bash
git add migrations/000095_create_aggregator_profiles.up.sql migrations/000095_create_aggregator_profiles.down.sql
git commit -m "feat(aggregator): create aggregator_profiles table"
```

---

## Task 3: DB Migration — virtual balance + audit log

**Files:**
- Create: `migrations/000096_add_virtual_balance.up.sql`
- Create: `migrations/000096_add_virtual_balance.down.sql`

- [ ] **Step 1: Write up migration**

```sql
-- migrations/000096_add_virtual_balance.up.sql
ALTER TABLE accounts
  ADD COLUMN is_virtual BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN allocated_by UUID REFERENCES clients(id);

-- Mark existing sub-account accounts as virtual
UPDATE accounts a
SET is_virtual = true, allocated_by = c.parent_client_id
FROM clients c
WHERE a.client_id = c.id AND c.account_type = 'sub_account';

CREATE TABLE aggregator_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id),
    action VARCHAR(50) NOT NULL,
    target_type VARCHAR(30) NOT NULL,
    target_id UUID,
    details JSONB,
    ip_address INET,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (created_at);

CREATE TABLE aggregator_audit_log_2026_04 PARTITION OF aggregator_audit_log
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE aggregator_audit_log_2026_05 PARTITION OF aggregator_audit_log
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE aggregator_audit_log_2026_06 PARTITION OF aggregator_audit_log
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

CREATE INDEX idx_agg_audit_log_agg ON aggregator_audit_log(aggregator_id, created_at);
CREATE INDEX idx_agg_audit_log_action ON aggregator_audit_log(action);
```

- [ ] **Step 2: Write down migration**

```sql
-- migrations/000096_add_virtual_balance.down.sql
DROP TABLE IF EXISTS aggregator_audit_log;
ALTER TABLE accounts DROP COLUMN IF EXISTS is_virtual;
ALTER TABLE accounts DROP COLUMN IF EXISTS allocated_by;
```

- [ ] **Step 3: Apply and verify**

```bash
migrate -path migrations -database "$DATABASE_URL" up 1
psql "$DATABASE_URL" -c "\d accounts" | grep -E "is_virtual|allocated_by"
psql "$DATABASE_URL" -c "\dt aggregator_audit*"
```

- [ ] **Step 4: Commit**

```bash
git add migrations/000096_add_virtual_balance.up.sql migrations/000096_add_virtual_balance.down.sql
git commit -m "feat(aggregator): add virtual balance fields and audit log table"
```

---

## Task 4: Proto Definition

**Files:**
- Create: `api/proto/aggregator/aggregator.proto`

- [ ] **Step 1: Write proto file**

```protobuf
// api/proto/aggregator/aggregator.proto
syntax = "proto3";

package aggregator;

option go_package = "github.com/smpp-server/smpp-server/api/proto/aggregatorv1";

import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

message AggregatorProfile {
    string id = 1;
    string client_id = 2;
    optional double max_markup_percent = 3;
    int32 max_sub_accounts = 4;
    optional int32 pool_monthly_limit = 5;
    optional int32 pool_rate_limit_per_second = 6;
    repeated string allowed_provider_ids = 7;
    bool auto_freeze_sub_accounts = 8;
    string notes = 9;
    google.protobuf.Timestamp created_at = 10;
    google.protobuf.Timestamp updated_at = 11;
}

message CreateAggregatorProfileRequest {
    string client_id = 1;
    optional double max_markup_percent = 2;
    int32 max_sub_accounts = 3;
    optional int32 pool_monthly_limit = 4;
    optional int32 pool_rate_limit_per_second = 5;
    repeated string allowed_provider_ids = 6;
    bool auto_freeze_sub_accounts = 7;
    string notes = 8;
}

message UpdateAggregatorProfileRequest {
    string client_id = 1;
    optional double max_markup_percent = 2;
    optional int32 max_sub_accounts = 3;
    optional int32 pool_monthly_limit = 4;
    optional int32 pool_rate_limit_per_second = 5;
    repeated string allowed_provider_ids = 6;
    optional bool auto_freeze_sub_accounts = 7;
    optional string notes = 8;
}

message GetAggregatorProfileRequest {
    string client_id = 1;
}

message ListAggregatorProfilesRequest {
    int32 limit = 1;
    int32 offset = 2;
}

message ListAggregatorProfilesResponse {
    repeated AggregatorProfile profiles = 1;
    int32 total = 2;
}

message SubAccountInfo {
    string id = 1;
    string aggregator_id = 2;
    string name = 3;
    string email = 4;
    string contact_person = 5;
    bool active = 6;
    string virtual_balance = 7;
    string currency = 8;
    int32 daily_limit = 9;
    int32 monthly_limit = 10;
    int32 monthly_sms_count = 11;
    google.protobuf.Timestamp created_at = 12;
}

message CreateSubAccountRequest {
    string aggregator_id = 1;
    string name = 2;
    string email = 3;
    string contact_person = 4;
    string initial_balance = 5;
    string currency = 6;
    int32 daily_limit = 7;
    int32 monthly_limit = 8;
}

message UpdateSubAccountRequest {
    string id = 1;
    string aggregator_id = 2;
    string name = 3;
    string contact_person = 4;
    int32 daily_limit = 5;
    int32 monthly_limit = 6;
}

message GetSubAccountRequest {
    string id = 1;
    string aggregator_id = 2;
}

message DeactivateSubAccountRequest {
    string id = 1;
    string aggregator_id = 2;
}

message ListSubAccountsRequest {
    string aggregator_id = 1;
    bool active_only = 2;
    int32 limit = 3;
    int32 offset = 4;
}

message ListSubAccountsResponse {
    repeated SubAccountInfo sub_accounts = 1;
    int32 total = 2;
}

message TransferBalanceRequest {
    string aggregator_id = 1;
    string sub_account_id = 2;
    string amount = 3;
    string currency = 4;
}

message TransferBalanceResponse {
    string aggregator_balance = 1;
    string sub_account_balance = 2;
}

message WithdrawBalanceRequest {
    string aggregator_id = 1;
    string sub_account_id = 2;
    string amount = 3;
    string currency = 4;
}

message UnfreezeAllSubAccountsRequest {
    string aggregator_id = 1;
}

message UnfreezeAllSubAccountsResponse {
    int32 unfrozen_count = 1;
}

service AggregatorService {
    // Admin: aggregator profile management
    rpc CreateAggregatorProfile(CreateAggregatorProfileRequest) returns (AggregatorProfile);
    rpc UpdateAggregatorProfile(UpdateAggregatorProfileRequest) returns (AggregatorProfile);
    rpc GetAggregatorProfile(GetAggregatorProfileRequest) returns (AggregatorProfile);
    rpc ListAggregatorProfiles(ListAggregatorProfilesRequest) returns (ListAggregatorProfilesResponse);

    // Aggregator: sub-account management
    rpc CreateSubAccount(CreateSubAccountRequest) returns (SubAccountInfo);
    rpc UpdateSubAccount(UpdateSubAccountRequest) returns (SubAccountInfo);
    rpc DeactivateSubAccount(DeactivateSubAccountRequest) returns (SubAccountInfo);
    rpc GetSubAccount(GetSubAccountRequest) returns (SubAccountInfo);
    rpc ListSubAccounts(ListSubAccountsRequest) returns (ListSubAccountsResponse);

    // Aggregator: balance operations
    rpc TransferBalance(TransferBalanceRequest) returns (TransferBalanceResponse);
    rpc WithdrawBalance(WithdrawBalanceRequest) returns (TransferBalanceResponse);
    rpc UnfreezeAllSubAccounts(UnfreezeAllSubAccountsRequest) returns (UnfreezeAllSubAccountsResponse);
}
```

- [ ] **Step 2: Generate Go code**

```bash
cd c:/projects/sms
protoc --go_out=. --go-grpc_out=. api/proto/aggregator/aggregator.proto
```

Expected: `api/proto/aggregatorv1/aggregator.pb.go` and `api/proto/aggregatorv1/aggregator_grpc.pb.go` created.

If protoc is not configured locally, add to the Makefile target and generate on the server, or copy the pattern from an existing `*v1` directory.

- [ ] **Step 3: Commit**

```bash
git add api/proto/aggregator/ api/proto/aggregatorv1/
git commit -m "feat(aggregator): add aggregatorv1 proto definition and generated code"
```

---

## Task 5: Domain Models

**Files:**
- Create: `internal/services/aggregator/domain/models.go`
- Create: `internal/services/aggregator/domain/errors.go`

- [ ] **Step 1: Write domain models**

```go
// internal/services/aggregator/domain/models.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// AggregatorProfile contains platform-controlled settings for an aggregator account.
type AggregatorProfile struct {
	ID                    uuid.UUID
	ClientID              uuid.UUID
	MaxMarkupPercent      *float64
	MaxSubAccounts        int
	PoolMonthlyLimit      *int
	PoolRateLimitPerSec   *int
	AllowedProviderIDs    []uuid.UUID
	AutoFreezeSubAccounts bool
	Notes                 string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// SubAccountSummary is a lightweight read model for listing sub-accounts with balance.
type SubAccountSummary struct {
	ID              uuid.UUID
	AggregatorID    uuid.UUID
	Name            string
	Email           string
	ContactPerson   string
	Active          bool
	VirtualBalance  string
	Currency        string
	DailyLimit      int32
	MonthlyLimit    int32
	MonthlySMSCount int32
	CreatedAt       time.Time
}

// AuditEntry represents a single aggregator action for the audit log.
type AuditEntry struct {
	AggregatorID uuid.UUID
	Action       string
	TargetType   string
	TargetID     *uuid.UUID
	Details      map[string]interface{}
	IPAddress    string
}
```

- [ ] **Step 2: Write domain errors**

```go
// internal/services/aggregator/domain/errors.go
package domain

import "errors"

var (
	ErrProfileNotFound       = errors.New("aggregator profile not found")
	ErrProfileAlreadyExists  = errors.New("aggregator profile already exists for this client")
	ErrNotAggregator         = errors.New("client is not an aggregator")
	ErrSubAccountNotFound    = errors.New("sub-account not found")
	ErrSubAccountNotOwned    = errors.New("sub-account does not belong to this aggregator")
	ErrMaxSubAccountsReached = errors.New("maximum sub-accounts limit reached")
	ErrInsufficientBalance   = errors.New("insufficient aggregator balance for transfer")
	ErrInvalidAmount         = errors.New("amount must be a positive number")
	ErrSubAccountFrozen      = errors.New("sub-account is frozen")
)
```

- [ ] **Step 3: Write tests for errors (ensure they are distinct)**

```go
// internal/services/aggregator/domain/errors_test.go
package domain_test

import (
	"errors"
	"testing"

	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/stretchr/testify/assert"
)

func TestErrorsAreDistinct(t *testing.T) {
	errs := []error{
		domain.ErrProfileNotFound,
		domain.ErrProfileAlreadyExists,
		domain.ErrNotAggregator,
		domain.ErrSubAccountNotFound,
		domain.ErrSubAccountNotOwned,
		domain.ErrMaxSubAccountsReached,
		domain.ErrInsufficientBalance,
		domain.ErrInvalidAmount,
	}
	for i, a := range errs {
		for j, b := range errs {
			if i != j {
				assert.False(t, errors.Is(a, b), "errors[%d] and errors[%d] should be distinct", i, j)
			}
		}
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd c:/projects/sms
go test ./internal/services/aggregator/domain/... -v
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/services/aggregator/domain/
git commit -m "feat(aggregator): add domain models and errors"
```

---

## Task 6: Repository — Aggregator Profile

**Files:**
- Create: `internal/services/aggregator/repository/profile_repository.go`

- [ ] **Step 1: Write failing test**

```go
// internal/services/aggregator/repository/profile_repository_test.go
package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/repository"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.New(database.Config{
		DSN: "postgres://postgres:postgres@localhost:5432/sms_test?sslmode=disable",
	})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestProfileRepository_CreateAndGet(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewProfileRepository(db)
	ctx := context.Background()

	// Need a real aggregator client_id — use a known fixture or insert one
	clientID := uuid.MustParse("00000000-0000-0000-0000-000000000001") // seed this in test DB

	maxMarkup := 150.0
	profile := &domain.AggregatorProfile{
		ClientID:              clientID,
		MaxMarkupPercent:      &maxMarkup,
		MaxSubAccounts:        50,
		AllowedProviderIDs:    []uuid.UUID{},
		AutoFreezeSubAccounts: true,
		Notes:                 "test aggregator",
	}

	err := repo.Create(ctx, profile)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, profile.ID)

	got, err := repo.GetByClientID(ctx, clientID)
	require.NoError(t, err)
	assert.Equal(t, clientID, got.ClientID)
	assert.Equal(t, 50, got.MaxSubAccounts)
	assert.InDelta(t, 150.0, *got.MaxMarkupPercent, 0.001)
}
```

- [ ] **Step 2: Run test — verify it fails**

```bash
go test ./internal/services/aggregator/repository/... -run TestProfileRepository_CreateAndGet -v
```

Expected: `FAIL — cannot find package`

- [ ] **Step 3: Implement profile repository**

```go
// internal/services/aggregator/repository/profile_repository.go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/lib/pq"
)

// ProfileRepository handles CRUD for aggregator_profiles.
type ProfileRepository struct {
	db *sqlx.DB
}

func NewProfileRepository(db *database.DB) *ProfileRepository {
	return &ProfileRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

// Create inserts a new aggregator profile. Sets profile.ID and timestamps on success.
func (r *ProfileRepository) Create(ctx context.Context, p *domain.AggregatorProfile) error {
	p.ID = uuid.New()
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	providerIDs := make([]string, len(p.AllowedProviderIDs))
	for i, id := range p.AllowedProviderIDs {
		providerIDs[i] = id.String()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO aggregator_profiles
			(id, client_id, max_markup_percent, max_sub_accounts, pool_monthly_limit,
			 pool_rate_limit_per_second, allowed_provider_ids, auto_freeze_sub_accounts, notes,
			 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		p.ID, p.ClientID, p.MaxMarkupPercent, p.MaxSubAccounts, p.PoolMonthlyLimit,
		p.PoolRateLimitPerSec, pq.Array(providerIDs), p.AutoFreezeSubAccounts, p.Notes,
		p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// GetByClientID returns the aggregator profile for the given client.
// Returns domain.ErrProfileNotFound if not found.
func (r *ProfileRepository) GetByClientID(ctx context.Context, clientID uuid.UUID) (*domain.AggregatorProfile, error) {
	var row struct {
		ID                    uuid.UUID      `db:"id"`
		ClientID              uuid.UUID      `db:"client_id"`
		MaxMarkupPercent      *float64       `db:"max_markup_percent"`
		MaxSubAccounts        int            `db:"max_sub_accounts"`
		PoolMonthlyLimit      *int           `db:"pool_monthly_limit"`
		PoolRateLimitPerSec   *int           `db:"pool_rate_limit_per_second"`
		AllowedProviderIDs    pq.StringArray `db:"allowed_provider_ids"`
		AutoFreezeSubAccounts bool           `db:"auto_freeze_sub_accounts"`
		Notes                 string         `db:"notes"`
		CreatedAt             time.Time      `db:"created_at"`
		UpdatedAt             time.Time      `db:"updated_at"`
	}
	err := r.db.GetContext(ctx, &row, `
		SELECT id, client_id, max_markup_percent, max_sub_accounts, pool_monthly_limit,
		       pool_rate_limit_per_second, allowed_provider_ids, auto_freeze_sub_accounts, notes,
		       created_at, updated_at
		FROM aggregator_profiles WHERE client_id = $1`, clientID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}

	p := &domain.AggregatorProfile{
		ID:                    row.ID,
		ClientID:              row.ClientID,
		MaxMarkupPercent:      row.MaxMarkupPercent,
		MaxSubAccounts:        row.MaxSubAccounts,
		PoolMonthlyLimit:      row.PoolMonthlyLimit,
		PoolRateLimitPerSec:   row.PoolRateLimitPerSec,
		AutoFreezeSubAccounts: row.AutoFreezeSubAccounts,
		Notes:                 row.Notes,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
	for _, s := range row.AllowedProviderIDs {
		id, err := uuid.Parse(s)
		if err == nil {
			p.AllowedProviderIDs = append(p.AllowedProviderIDs, id)
		}
	}
	return p, nil
}

// Update saves changes to an existing aggregator profile.
func (r *ProfileRepository) Update(ctx context.Context, p *domain.AggregatorProfile) error {
	p.UpdatedAt = time.Now()

	providerIDs := make([]string, len(p.AllowedProviderIDs))
	for i, id := range p.AllowedProviderIDs {
		providerIDs[i] = id.String()
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE aggregator_profiles
		SET max_markup_percent=$1, max_sub_accounts=$2, pool_monthly_limit=$3,
		    pool_rate_limit_per_second=$4, allowed_provider_ids=$5,
		    auto_freeze_sub_accounts=$6, notes=$7, updated_at=$8
		WHERE client_id=$9`,
		p.MaxMarkupPercent, p.MaxSubAccounts, p.PoolMonthlyLimit,
		p.PoolRateLimitPerSec, pq.Array(providerIDs),
		p.AutoFreezeSubAccounts, p.Notes, p.UpdatedAt, p.ClientID,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.ErrProfileNotFound
	}
	return nil
}

// List returns all aggregator profiles with pagination.
func (r *ProfileRepository) List(ctx context.Context, limit, offset int) ([]*domain.AggregatorProfile, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM aggregator_profiles`); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, client_id, max_markup_percent, max_sub_accounts, pool_monthly_limit,
		       pool_rate_limit_per_second, allowed_provider_ids, auto_freeze_sub_accounts, notes,
		       created_at, updated_at
		FROM aggregator_profiles ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var profiles []*domain.AggregatorProfile
	for rows.Next() {
		var row struct {
			ID                    uuid.UUID      `db:"id"`
			ClientID              uuid.UUID      `db:"client_id"`
			MaxMarkupPercent      *float64       `db:"max_markup_percent"`
			MaxSubAccounts        int            `db:"max_sub_accounts"`
			PoolMonthlyLimit      *int           `db:"pool_monthly_limit"`
			PoolRateLimitPerSec   *int           `db:"pool_rate_limit_per_second"`
			AllowedProviderIDs    pq.StringArray `db:"allowed_provider_ids"`
			AutoFreezeSubAccounts bool           `db:"auto_freeze_sub_accounts"`
			Notes                 string         `db:"notes"`
			CreatedAt             time.Time      `db:"created_at"`
			UpdatedAt             time.Time      `db:"updated_at"`
		}
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, err
		}
		p := &domain.AggregatorProfile{
			ID: row.ID, ClientID: row.ClientID, MaxMarkupPercent: row.MaxMarkupPercent,
			MaxSubAccounts: row.MaxSubAccounts, PoolMonthlyLimit: row.PoolMonthlyLimit,
			PoolRateLimitPerSec: row.PoolRateLimitPerSec, AutoFreezeSubAccounts: row.AutoFreezeSubAccounts,
			Notes: row.Notes, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
		for _, s := range row.AllowedProviderIDs {
			if id, err := uuid.Parse(s); err == nil {
				p.AllowedProviderIDs = append(p.AllowedProviderIDs, id)
			}
		}
		profiles = append(profiles, p)
	}
	return profiles, total, rows.Err()
}

// CountSubAccounts returns the current number of active sub-accounts for an aggregator.
func (r *ProfileRepository) CountSubAccounts(ctx context.Context, aggregatorID uuid.UUID) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM clients WHERE parent_client_id=$1 AND active=true AND account_type='sub_account'`,
		aggregatorID)
	return count, err
}
```

- [ ] **Step 4: Run test**

```bash
go test ./internal/services/aggregator/repository/... -run TestProfileRepository_CreateAndGet -v
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/services/aggregator/repository/profile_repository.go \
        internal/services/aggregator/repository/profile_repository_test.go
git commit -m "feat(aggregator): add profile repository with CRUD and sub-account count"
```

---

## Task 7: Repository — Balance + Audit Log

**Files:**
- Create: `internal/services/aggregator/repository/balance_repository.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/services/aggregator/repository/balance_repository_test.go
package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBalanceRepository_TransferAndGet(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewBalanceRepository(db)
	ctx := context.Background()

	// Assumes aggregator and sub-account exist in test DB with known IDs
	aggregatorID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	subAccountID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	err := repo.Transfer(ctx, aggregatorID, subAccountID, "100.00", "RUB")
	require.NoError(t, err)

	aggBal, err := repo.GetBalance(ctx, aggregatorID)
	require.NoError(t, err)
	assert.NotEmpty(t, aggBal)

	subBal, err := repo.GetBalance(ctx, subAccountID)
	require.NoError(t, err)
	assert.NotEmpty(t, subBal)
}

func TestBalanceRepository_WriteAuditLog(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewBalanceRepository(db)
	ctx := context.Background()

	aggregatorID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	targetID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	entry := &domain.AuditEntry{
		AggregatorID: aggregatorID,
		Action:       "transfer_balance",
		TargetType:   "sub_account",
		TargetID:     &targetID,
		Details:      map[string]interface{}{"amount": "100.00", "currency": "RUB"},
		IPAddress:    "127.0.0.1",
	}
	err := repo.WriteAuditLog(ctx, entry)
	assert.NoError(t, err)
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/services/aggregator/repository/... -run TestBalanceRepository -v
```

Expected: `FAIL — NewBalanceRepository undefined`

- [ ] **Step 3: Implement balance repository**

```go
// internal/services/aggregator/repository/balance_repository.go
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

// BalanceRepository handles atomic balance transfers and audit logging.
type BalanceRepository struct {
	db *sqlx.DB
}

func NewBalanceRepository(db *database.DB) *BalanceRepository {
	return &BalanceRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

// GetBalance returns the current balance string for a client.
func (r *BalanceRepository) GetBalance(ctx context.Context, clientID uuid.UUID) (string, error) {
	var balance string
	err := r.db.GetContext(ctx, &balance,
		`SELECT balance::text FROM accounts WHERE client_id=$1`, clientID)
	if err == sql.ErrNoRows {
		return "0", nil
	}
	return balance, err
}

// Transfer atomically moves amount from aggregator real balance to sub-account virtual balance.
// Also records balance_transfers and two transaction rows.
func (r *BalanceRepository) Transfer(ctx context.Context, aggregatorID, subAccountID uuid.UUID, amount, currency string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Lock both rows in consistent order to prevent deadlocks
	first, second := aggregatorID, subAccountID
	if aggregatorID.String() > subAccountID.String() {
		first, second = subAccountID, aggregatorID
	}
	var dummy string
	if err := tx.QueryRowContext(ctx,
		`SELECT balance FROM accounts WHERE client_id=$1 FOR UPDATE`, first).Scan(&dummy); err != nil {
		return fmt.Errorf("lock aggregator balance: %w", err)
	}
	if err := tx.QueryRowContext(ctx,
		`SELECT balance FROM accounts WHERE client_id=$1 FOR UPDATE`, second).Scan(&dummy); err != nil {
		return fmt.Errorf("lock sub-account balance: %w", err)
	}

	// Check aggregator has enough
	var aggBalance string
	if err := tx.QueryRowContext(ctx,
		`SELECT balance::text FROM accounts WHERE client_id=$1`, aggregatorID).Scan(&aggBalance); err != nil {
		return fmt.Errorf("get aggregator balance: %w", err)
	}
	// Deduct from aggregator
	result, err := tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance - $1::numeric, updated_at=NOW()
		 WHERE client_id=$2 AND balance >= $1::numeric AND NOT frozen`,
		amount, aggregatorID)
	if err != nil {
		return fmt.Errorf("deduct aggregator balance: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return domain.ErrInsufficientBalance
	}

	// Credit sub-account
	if _, err := tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance + $1::numeric, updated_at=NOW()
		 WHERE client_id=$2`,
		amount, subAccountID); err != nil {
		return fmt.Errorf("credit sub-account: %w", err)
	}

	// Record transactions
	now := time.Now()
	txID := uuid.New()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO transactions (id, client_id, type, amount, currency, created_at)
		 VALUES ($1,$2,'transfer_out',$3,$4,$5)`,
		txID, aggregatorID, amount, currency, now); err != nil {
		return fmt.Errorf("record aggregator transaction: %w", err)
	}
	rxID := uuid.New()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO transactions (id, client_id, type, amount, currency, created_at)
		 VALUES ($1,$2,'transfer_in',$3,$4,$5)`,
		rxID, subAccountID, amount, currency, now); err != nil {
		return fmt.Errorf("record sub-account transaction: %w", err)
	}

	// Record balance_transfer
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO balance_transfers (id, from_client_id, to_client_id, amount, currency,
		  debit_transaction_id, credit_transaction_id, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		uuid.New(), aggregatorID, subAccountID, amount, currency, txID, rxID, now); err != nil {
		return fmt.Errorf("record balance_transfer: %w", err)
	}

	return tx.Commit()
}

// Withdraw atomically moves amount from sub-account virtual balance back to aggregator.
func (r *BalanceRepository) Withdraw(ctx context.Context, aggregatorID, subAccountID uuid.UUID, amount, currency string) error {
	// Same as Transfer but reversed direction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance - $1::numeric, updated_at=NOW()
		 WHERE client_id=$2 AND balance >= $1::numeric AND NOT frozen AND is_virtual=true`,
		amount, subAccountID)
	if err != nil {
		return fmt.Errorf("deduct sub-account balance: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return domain.ErrInsufficientBalance
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance + $1::numeric, updated_at=NOW() WHERE client_id=$2`,
		amount, aggregatorID); err != nil {
		return fmt.Errorf("credit aggregator: %w", err)
	}

	now := time.Now()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO transactions (id, client_id, type, amount, currency, created_at)
		 VALUES ($1,$2,'transfer_out',$3,$4,$5)`,
		uuid.New(), subAccountID, amount, currency, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO transactions (id, client_id, type, amount, currency, created_at)
		 VALUES ($1,$2,'transfer_in',$3,$4,$5)`,
		uuid.New(), aggregatorID, amount, currency, now); err != nil {
		return err
	}

	return tx.Commit()
}

// WriteAuditLog inserts a row into aggregator_audit_log. Best-effort: errors are logged but not fatal.
func (r *BalanceRepository) WriteAuditLog(ctx context.Context, e *domain.AuditEntry) error {
	details, _ := json.Marshal(e.Details)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO aggregator_audit_log
		 (id, aggregator_id, action, target_type, target_id, details, ip_address, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7::inet,$8)`,
		uuid.New(), e.AggregatorID, e.Action, e.TargetType, e.TargetID,
		details, nilIfEmpty(e.IPAddress), time.Now())
	return err
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/services/aggregator/repository/... -run TestBalanceRepository -v
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/services/aggregator/repository/balance_repository.go \
        internal/services/aggregator/repository/balance_repository_test.go
git commit -m "feat(aggregator): add balance repository — atomic transfer and audit log"
```

---

## Task 8: Service Layer

**Files:**
- Create: `internal/services/aggregator/service/aggregator_service.go`

- [ ] **Step 1: Write failing service tests**

```go
// internal/services/aggregator/service/aggregator_service_test.go
package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type mockProfileRepo struct{ mock.Mock }

func (m *mockProfileRepo) Create(ctx context.Context, p *domain.AggregatorProfile) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockProfileRepo) GetByClientID(ctx context.Context, id uuid.UUID) (*domain.AggregatorProfile, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AggregatorProfile), args.Error(1)
}
func (m *mockProfileRepo) Update(ctx context.Context, p *domain.AggregatorProfile) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockProfileRepo) List(ctx context.Context, limit, offset int) ([]*domain.AggregatorProfile, int, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*domain.AggregatorProfile), args.Int(1), args.Error(2)
}
func (m *mockProfileRepo) CountSubAccounts(ctx context.Context, id uuid.UUID) (int, error) {
	args := m.Called(ctx, id)
	return args.Int(0), args.Error(1)
}

type mockBalanceRepo struct{ mock.Mock }

func (m *mockBalanceRepo) GetBalance(ctx context.Context, id uuid.UUID) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}
func (m *mockBalanceRepo) Transfer(ctx context.Context, aggID, subID uuid.UUID, amount, currency string) error {
	return m.Called(ctx, aggID, subID, amount, currency).Error(0)
}
func (m *mockBalanceRepo) Withdraw(ctx context.Context, aggID, subID uuid.UUID, amount, currency string) error {
	return m.Called(ctx, aggID, subID, amount, currency).Error(0)
}
func (m *mockBalanceRepo) WriteAuditLog(ctx context.Context, e *domain.AuditEntry) error {
	return m.Called(ctx, e).Error(0)
}

// --- Tests ---

func TestService_CreateProfile_SetsDefaults(t *testing.T) {
	profileRepo := &mockProfileRepo{}
	balanceRepo := &mockBalanceRepo{}
	svc := service.New(profileRepo, balanceRepo)

	clientID := uuid.New()
	req := &domain.AggregatorProfile{
		ClientID:       clientID,
		MaxSubAccounts: 0, // should default to 100
	}
	profileRepo.On("Create", mock.Anything, mock.MatchedBy(func(p *domain.AggregatorProfile) bool {
		return p.MaxSubAccounts == 100
	})).Return(nil)

	err := svc.CreateProfile(context.Background(), req)
	require.NoError(t, err)
	profileRepo.AssertExpectations(t)
}

func TestService_TransferBalance_InvalidAmount(t *testing.T) {
	svc := service.New(&mockProfileRepo{}, &mockBalanceRepo{})
	err := svc.TransferBalance(context.Background(), uuid.New(), uuid.New(), "-50", "RUB", "")
	assert.ErrorIs(t, err, domain.ErrInvalidAmount)
}

func TestService_TransferBalance_ExceedsLimit(t *testing.T) {
	profileRepo := &mockProfileRepo{}
	balanceRepo := &mockBalanceRepo{}
	svc := service.New(profileRepo, balanceRepo)

	aggID := uuid.New()
	subID := uuid.New()
	profileRepo.On("CountSubAccounts", mock.Anything, aggID).Return(0, nil)
	balanceRepo.On("Transfer", mock.Anything, aggID, subID, "100.00", "RUB").
		Return(domain.ErrInsufficientBalance)
	balanceRepo.On("WriteAuditLog", mock.Anything, mock.Anything).Return(nil)

	err := svc.TransferBalance(context.Background(), aggID, subID, "100.00", "RUB", "")
	assert.ErrorIs(t, err, domain.ErrInsufficientBalance)
}
```

- [ ] **Step 2: Run test — verify fails**

```bash
go test ./internal/services/aggregator/service/... -v
```

Expected: `FAIL — package not found`

- [ ] **Step 3: Implement service**

```go
// internal/services/aggregator/service/aggregator_service.go
package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
)

// ProfileRepo defines profile storage operations.
type ProfileRepo interface {
	Create(ctx context.Context, p *domain.AggregatorProfile) error
	GetByClientID(ctx context.Context, clientID uuid.UUID) (*domain.AggregatorProfile, error)
	Update(ctx context.Context, p *domain.AggregatorProfile) error
	List(ctx context.Context, limit, offset int) ([]*domain.AggregatorProfile, int, error)
	CountSubAccounts(ctx context.Context, aggregatorID uuid.UUID) (int, error)
}

// BalanceRepo defines balance and audit operations.
type BalanceRepo interface {
	GetBalance(ctx context.Context, clientID uuid.UUID) (string, error)
	Transfer(ctx context.Context, aggregatorID, subAccountID uuid.UUID, amount, currency string) error
	Withdraw(ctx context.Context, aggregatorID, subAccountID uuid.UUID, amount, currency string) error
	WriteAuditLog(ctx context.Context, e *domain.AuditEntry) error
}

// Service provides aggregator business logic.
type Service struct {
	profiles ProfileRepo
	balances BalanceRepo
}

func New(profiles ProfileRepo, balances BalanceRepo) *Service {
	return &Service{profiles: profiles, balances: balances}
}

// CreateProfile creates an aggregator profile with sane defaults.
func (s *Service) CreateProfile(ctx context.Context, p *domain.AggregatorProfile) error {
	if p.MaxSubAccounts <= 0 {
		p.MaxSubAccounts = 100
	}
	if p.AllowedProviderIDs == nil {
		p.AllowedProviderIDs = []uuid.UUID{}
	}
	return s.profiles.Create(ctx, p)
}

// GetProfile returns the aggregator profile for clientID.
func (s *Service) GetProfile(ctx context.Context, clientID uuid.UUID) (*domain.AggregatorProfile, error) {
	return s.profiles.GetByClientID(ctx, clientID)
}

// UpdateProfile saves changes to an existing profile.
func (s *Service) UpdateProfile(ctx context.Context, p *domain.AggregatorProfile) error {
	return s.profiles.Update(ctx, p)
}

// ListProfiles returns all aggregator profiles.
func (s *Service) ListProfiles(ctx context.Context, limit, offset int) ([]*domain.AggregatorProfile, int, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.profiles.List(ctx, limit, offset)
}

// TransferBalance moves amount from aggregator to sub-account.
// ipAddress is used for audit log (may be empty).
func (s *Service) TransferBalance(ctx context.Context, aggregatorID, subAccountID uuid.UUID, amount, currency, ipAddress string) error {
	if err := validateAmount(amount); err != nil {
		return err
	}
	if currency == "" {
		currency = "RUB"
	}

	if err := s.balances.Transfer(ctx, aggregatorID, subAccountID, amount, currency); err != nil {
		return err
	}

	targetID := subAccountID
	s.writeAudit(ctx, &domain.AuditEntry{
		AggregatorID: aggregatorID,
		Action:       "transfer_balance",
		TargetType:   "sub_account",
		TargetID:     &targetID,
		Details:      map[string]interface{}{"amount": amount, "currency": currency},
		IPAddress:    ipAddress,
	})
	return nil
}

// WithdrawBalance moves amount from sub-account back to aggregator.
func (s *Service) WithdrawBalance(ctx context.Context, aggregatorID, subAccountID uuid.UUID, amount, currency, ipAddress string) error {
	if err := validateAmount(amount); err != nil {
		return err
	}
	if currency == "" {
		currency = "RUB"
	}

	if err := s.balances.Withdraw(ctx, aggregatorID, subAccountID, amount, currency); err != nil {
		return err
	}

	targetID := subAccountID
	s.writeAudit(ctx, &domain.AuditEntry{
		AggregatorID: aggregatorID,
		Action:       "withdraw_balance",
		TargetType:   "sub_account",
		TargetID:     &targetID,
		Details:      map[string]interface{}{"amount": amount, "currency": currency},
		IPAddress:    ipAddress,
	})
	return nil
}

func (s *Service) writeAudit(ctx context.Context, e *domain.AuditEntry) {
	if err := s.balances.WriteAuditLog(ctx, e); err != nil {
		log.Error().Err(err).Str("action", e.Action).Msg("aggregator audit log write failed")
	}
}

func validateAmount(amount string) error {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return domain.ErrInvalidAmount
	}
	f, err := strconv.ParseFloat(amount, 64)
	if err != nil || f <= 0 {
		return fmt.Errorf("%w: got %q", domain.ErrInvalidAmount, amount)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/services/aggregator/service/... -v
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/services/aggregator/service/ 
git commit -m "feat(aggregator): add service layer — profile management and balance transfer"
```

---

## Task 9: Update Client Domain Model

The `Client` domain struct still has `IsReseller` — replace with `AccountType`.

**Files:**
- Modify: `internal/services/client/domain/client.go`
- Modify: `internal/services/client/infrastructure/repository/client_repository.go`

- [ ] **Step 1: Update Client domain struct**

Find the field `IsReseller bool` in `internal/services/client/domain/client.go` and replace with:

```go
AccountType string // "direct", "aggregator", "sub_account"
```

Remove any references to `IsReseller` in the same file.

- [ ] **Step 2: Update client repository queries**

In `internal/services/client/infrastructure/repository/client_repository.go`, update the `Create` INSERT to use `account_type` instead of `is_reseller`:

```go
// In Create():
query := `
    INSERT INTO clients (
        id, name, api_key, secret, email, contact_person, phone, active, metadata,
        parent_client_id, account_type, max_sub_accounts, is_sandbox, created_at, updated_at
    ) VALUES (
        $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
    )
`
// pass client.AccountType instead of client.IsReseller
```

Update the `GetByID` SELECT similarly — replace `c.is_reseller` with `c.account_type` and scan into `client.AccountType`.

- [ ] **Step 3: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no errors. Fix any remaining references to `IsReseller`.

- [ ] **Step 4: Commit**

```bash
git add internal/services/client/
git commit -m "refactor(client): replace IsReseller with AccountType field"
```

---

## Task 10: gRPC Server

**Files:**
- Create: `internal/services/aggregator/grpc/server.go`

- [ ] **Step 1: Implement gRPC server**

```go
// internal/services/aggregator/grpc/server.go
package grpc

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	aggregatorv1 "github.com/smpp-server/smpp-server/api/proto/aggregatorv1"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements aggregatorv1.AggregatorServiceServer.
type Server struct {
	aggregatorv1.UnimplementedAggregatorServiceServer
	svc *service.Service
}

func NewServer(svc *service.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) CreateAggregatorProfile(ctx context.Context, req *aggregatorv1.CreateAggregatorProfileRequest) (*aggregatorv1.AggregatorProfile, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}

	p := &domain.AggregatorProfile{
		ClientID:              clientID,
		MaxSubAccounts:        int(req.MaxSubAccounts),
		AutoFreezeSubAccounts: req.AutoFreezeSubAccounts,
		Notes:                 req.Notes,
	}
	if req.MaxMarkupPercent != nil {
		v := *req.MaxMarkupPercent
		p.MaxMarkupPercent = &v
	}
	if req.PoolMonthlyLimit != nil {
		v := int(*req.PoolMonthlyLimit)
		p.PoolMonthlyLimit = &v
	}
	if req.PoolRateLimitPerSecond != nil {
		v := int(*req.PoolRateLimitPerSecond)
		p.PoolRateLimitPerSec = &v
	}
	for _, idStr := range req.AllowedProviderIds {
		if id, err := uuid.Parse(idStr); err == nil {
			p.AllowedProviderIDs = append(p.AllowedProviderIDs, id)
		}
	}

	if err := s.svc.CreateProfile(ctx, p); err != nil {
		log.Error().Err(err).Msg("CreateAggregatorProfile failed")
		return nil, status.Errorf(codes.Internal, "create profile: %v", err)
	}
	return profileToProto(p), nil
}

func (s *Server) GetAggregatorProfile(ctx context.Context, req *aggregatorv1.GetAggregatorProfileRequest) (*aggregatorv1.AggregatorProfile, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id")
	}
	p, err := s.svc.GetProfile(ctx, clientID)
	if err == domain.ErrProfileNotFound {
		return nil, status.Errorf(codes.NotFound, "aggregator profile not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return profileToProto(p), nil
}

func (s *Server) UpdateAggregatorProfile(ctx context.Context, req *aggregatorv1.UpdateAggregatorProfileRequest) (*aggregatorv1.AggregatorProfile, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id")
	}
	p, err := s.svc.GetProfile(ctx, clientID)
	if err == domain.ErrProfileNotFound {
		return nil, status.Errorf(codes.NotFound, "aggregator profile not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	if req.MaxMarkupPercent != nil {
		v := *req.MaxMarkupPercent
		p.MaxMarkupPercent = &v
	}
	if req.MaxSubAccounts != nil {
		p.MaxSubAccounts = int(*req.MaxSubAccounts)
	}
	if req.PoolMonthlyLimit != nil {
		v := int(*req.PoolMonthlyLimit)
		p.PoolMonthlyLimit = &v
	}
	if req.PoolRateLimitPerSecond != nil {
		v := int(*req.PoolRateLimitPerSecond)
		p.PoolRateLimitPerSec = &v
	}
	if req.AllowedProviderIds != nil {
		p.AllowedProviderIDs = nil
		for _, idStr := range req.AllowedProviderIds {
			if id, err := uuid.Parse(idStr); err == nil {
				p.AllowedProviderIDs = append(p.AllowedProviderIDs, id)
			}
		}
	}
	if req.AutoFreezeSubAccounts != nil {
		p.AutoFreezeSubAccounts = *req.AutoFreezeSubAccounts
	}
	if req.Notes != nil {
		p.Notes = *req.Notes
	}

	if err := s.svc.UpdateProfile(ctx, p); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return profileToProto(p), nil
}

func (s *Server) ListAggregatorProfiles(ctx context.Context, req *aggregatorv1.ListAggregatorProfilesRequest) (*aggregatorv1.ListAggregatorProfilesResponse, error) {
	profiles, total, err := s.svc.ListProfiles(ctx, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	resp := &aggregatorv1.ListAggregatorProfilesResponse{Total: int32(total)}
	for _, p := range profiles {
		resp.Profiles = append(resp.Profiles, profileToProto(p))
	}
	return resp, nil
}

func (s *Server) TransferBalance(ctx context.Context, req *aggregatorv1.TransferBalanceRequest) (*aggregatorv1.TransferBalanceResponse, error) {
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id")
	}
	subID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid sub_account_id")
	}

	if err := s.svc.TransferBalance(ctx, aggID, subID, req.Amount, req.Currency, ""); err != nil {
		switch err {
		case domain.ErrInsufficientBalance:
			return nil, status.Errorf(codes.FailedPrecondition, "insufficient balance")
		case domain.ErrInvalidAmount:
			return nil, status.Errorf(codes.InvalidArgument, "invalid amount")
		default:
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}
	return &aggregatorv1.TransferBalanceResponse{}, nil
}

func (s *Server) WithdrawBalance(ctx context.Context, req *aggregatorv1.WithdrawBalanceRequest) (*aggregatorv1.TransferBalanceResponse, error) {
	aggID, err := uuid.Parse(req.AggregatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid aggregator_id")
	}
	subID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid sub_account_id")
	}

	if err := s.svc.WithdrawBalance(ctx, aggID, subID, req.Amount, req.Currency, ""); err != nil {
		switch err {
		case domain.ErrInsufficientBalance:
			return nil, status.Errorf(codes.FailedPrecondition, "insufficient balance")
		case domain.ErrInvalidAmount:
			return nil, status.Errorf(codes.InvalidArgument, "invalid amount")
		default:
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}
	return &aggregatorv1.TransferBalanceResponse{}, nil
}

// profileToProto converts domain model to proto message.
func profileToProto(p *domain.AggregatorProfile) *aggregatorv1.AggregatorProfile {
	pb := &aggregatorv1.AggregatorProfile{
		Id:                    p.ID.String(),
		ClientId:              p.ClientID.String(),
		MaxSubAccounts:        int32(p.MaxSubAccounts),
		AutoFreezeSubAccounts: p.AutoFreezeSubAccounts,
		Notes:                 p.Notes,
		CreatedAt:             timestamppb.New(p.CreatedAt),
		UpdatedAt:             timestamppb.New(p.UpdatedAt),
	}
	if p.MaxMarkupPercent != nil {
		v := *p.MaxMarkupPercent
		pb.MaxMarkupPercent = &v
	}
	if p.PoolMonthlyLimit != nil {
		v := int32(*p.PoolMonthlyLimit)
		pb.PoolMonthlyLimit = &v
	}
	if p.PoolRateLimitPerSec != nil {
		v := int32(*p.PoolRateLimitPerSec)
		pb.PoolRateLimitPerSecond = &v
	}
	for _, id := range p.AllowedProviderIDs {
		pb.AllowedProviderIds = append(pb.AllowedProviderIds, id.String())
	}
	return pb
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/services/aggregator/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/services/aggregator/grpc/
git commit -m "feat(aggregator): add gRPC server implementation"
```

---

## Task 11: Admin HTTP Handlers

**Files:**
- Create: `internal/gateway/portal/handlers/aggregator_admin.go`

- [ ] **Step 1: Implement admin handlers**

```go
// internal/gateway/portal/handlers/aggregator_admin.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	aggregatorv1 "github.com/smpp-server/smpp-server/api/proto/aggregatorv1"
	"github.com/smpp-server/smpp-server/internal/api/http/response"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AggregatorAdminHandlers handles platform admin endpoints for aggregator management.
type AggregatorAdminHandlers struct {
	aggregatorClient aggregatorv1.AggregatorServiceClient
}

func NewAggregatorAdminHandlers(client aggregatorv1.AggregatorServiceClient) *AggregatorAdminHandlers {
	return &AggregatorAdminHandlers{aggregatorClient: client}
}

// ListAggregatorProfiles GET /api/v1/admin/aggregators
func (h *AggregatorAdminHandlers) ListAggregatorProfiles(w http.ResponseWriter, r *http.Request) {
	resp, err := h.aggregatorClient.ListAggregatorProfiles(r.Context(), &aggregatorv1.ListAggregatorProfilesRequest{
		Limit:  50,
		Offset: 0,
	})
	if err != nil {
		response.Error(w, shared.ErrInternal("failed to list aggregator profiles"))
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// CreateAggregatorProfile POST /api/v1/admin/aggregators
func (h *AggregatorAdminHandlers) CreateAggregatorProfile(w http.ResponseWriter, r *http.Request) {
	var req aggregatorv1.CreateAggregatorProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	if req.ClientId == "" {
		response.Error(w, shared.ErrInvalidInput("client_id is required"))
		return
	}

	profile, err := h.aggregatorClient.CreateAggregatorProfile(r.Context(), &req)
	if err != nil {
		response.Error(w, shared.ErrInternal("failed to create aggregator profile"))
		return
	}
	response.JSON(w, http.StatusCreated, profile)
}

// GetAggregatorProfile GET /api/v1/admin/aggregators/:client_id
func (h *AggregatorAdminHandlers) GetAggregatorProfile(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["client_id"]
	profile, err := h.aggregatorClient.GetAggregatorProfile(r.Context(), &aggregatorv1.GetAggregatorProfileRequest{
		ClientId: clientID,
	})
	if err != nil {
		response.Error(w, shared.ErrNotFound("aggregator profile not found"))
		return
	}
	response.JSON(w, http.StatusOK, profile)
}

// UpdateAggregatorProfile PUT /api/v1/admin/aggregators/:client_id
func (h *AggregatorAdminHandlers) UpdateAggregatorProfile(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["client_id"]
	var req aggregatorv1.UpdateAggregatorProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	req.ClientId = clientID

	profile, err := h.aggregatorClient.UpdateAggregatorProfile(r.Context(), &req)
	if err != nil {
		response.Error(w, shared.ErrInternal("failed to update aggregator profile"))
		return
	}
	response.JSON(w, http.StatusOK, profile)
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/gateway/portal/handlers/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/aggregator_admin.go
git commit -m "feat(aggregator): add admin HTTP handlers for aggregator profile management"
```

---

## Task 12: Portal HTTP Handlers (Aggregator Self-Service)

**Files:**
- Create: `internal/gateway/portal/handlers/aggregator_portal.go`

- [ ] **Step 1: Implement portal handlers**

```go
// internal/gateway/portal/handlers/aggregator_portal.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	aggregatorv1 "github.com/smpp-server/smpp-server/api/proto/aggregatorv1"
	"github.com/smpp-server/smpp-server/internal/api/http/response"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AggregatorPortalHandlers handles endpoints accessible by aggregator role users.
type AggregatorPortalHandlers struct {
	aggregatorClient aggregatorv1.AggregatorServiceClient
}

func NewAggregatorPortalHandlers(client aggregatorv1.AggregatorServiceClient) *AggregatorPortalHandlers {
	return &AggregatorPortalHandlers{aggregatorClient: client}
}

// ListSubAccounts GET /api/v1/aggregator/sub-accounts
func (h *AggregatorPortalHandlers) ListSubAccounts(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		response.Error(w, shared.ErrUnauthorized("not authenticated"))
		return
	}

	resp, err := h.aggregatorClient.ListSubAccounts(r.Context(), &aggregatorv1.ListSubAccountsRequest{
		AggregatorId: aggID.String(),
		Limit:        100,
	})
	if err != nil {
		response.Error(w, shared.ErrInternal("failed to list sub-accounts"))
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// CreateSubAccount POST /api/v1/aggregator/sub-accounts
func (h *AggregatorPortalHandlers) CreateSubAccount(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		response.Error(w, shared.ErrUnauthorized("not authenticated"))
		return
	}

	var body struct {
		Name           string `json:"name"`
		Email          string `json:"email"`
		ContactPerson  string `json:"contact_person"`
		InitialBalance string `json:"initial_balance"`
		Currency       string `json:"currency"`
		DailyLimit     int32  `json:"daily_limit"`
		MonthlyLimit   int32  `json:"monthly_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	if body.Name == "" || body.Email == "" {
		response.Error(w, shared.ErrInvalidInput("name and email are required"))
		return
	}

	sub, err := h.aggregatorClient.CreateSubAccount(r.Context(), &aggregatorv1.CreateSubAccountRequest{
		AggregatorId:   aggID.String(),
		Name:           body.Name,
		Email:          body.Email,
		ContactPerson:  body.ContactPerson,
		InitialBalance: body.InitialBalance,
		Currency:       body.Currency,
		DailyLimit:     body.DailyLimit,
		MonthlyLimit:   body.MonthlyLimit,
	})
	if err != nil {
		response.Error(w, shared.ErrInternal("failed to create sub-account"))
		return
	}
	response.JSON(w, http.StatusCreated, sub)
}

// GetSubAccount GET /api/v1/aggregator/sub-accounts/:id
func (h *AggregatorPortalHandlers) GetSubAccount(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		response.Error(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	subID := mux.Vars(r)["id"]

	sub, err := h.aggregatorClient.GetSubAccount(r.Context(), &aggregatorv1.GetSubAccountRequest{
		Id:           subID,
		AggregatorId: aggID.String(),
	})
	if err != nil {
		response.Error(w, shared.ErrNotFound("sub-account not found"))
		return
	}
	response.JSON(w, http.StatusOK, sub)
}

// UpdateSubAccount PUT /api/v1/aggregator/sub-accounts/:id
func (h *AggregatorPortalHandlers) UpdateSubAccount(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		response.Error(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	subID := mux.Vars(r)["id"]

	var body struct {
		Name          string `json:"name"`
		ContactPerson string `json:"contact_person"`
		DailyLimit    int32  `json:"daily_limit"`
		MonthlyLimit  int32  `json:"monthly_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, shared.ErrInvalidInput("invalid request body"))
		return
	}

	sub, err := h.aggregatorClient.UpdateSubAccount(r.Context(), &aggregatorv1.UpdateSubAccountRequest{
		Id:            subID,
		AggregatorId:  aggID.String(),
		Name:          body.Name,
		ContactPerson: body.ContactPerson,
		DailyLimit:    body.DailyLimit,
		MonthlyLimit:  body.MonthlyLimit,
	})
	if err != nil {
		response.Error(w, shared.ErrInternal("failed to update sub-account"))
		return
	}
	response.JSON(w, http.StatusOK, sub)
}

// DeactivateSubAccount POST /api/v1/aggregator/sub-accounts/:id/deactivate
func (h *AggregatorPortalHandlers) DeactivateSubAccount(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		response.Error(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	subID := mux.Vars(r)["id"]

	sub, err := h.aggregatorClient.DeactivateSubAccount(r.Context(), &aggregatorv1.DeactivateSubAccountRequest{
		Id:           subID,
		AggregatorId: aggID.String(),
	})
	if err != nil {
		response.Error(w, shared.ErrInternal("failed to deactivate sub-account"))
		return
	}
	response.JSON(w, http.StatusOK, sub)
}

// TransferBalance POST /api/v1/aggregator/sub-accounts/:id/transfer
func (h *AggregatorPortalHandlers) TransferBalance(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		response.Error(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	subID := mux.Vars(r)["id"]

	var body struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, shared.ErrInvalidInput("invalid request body"))
		return
	}

	resp, err := h.aggregatorClient.TransferBalance(r.Context(), &aggregatorv1.TransferBalanceRequest{
		AggregatorId: aggID.String(),
		SubAccountId: subID,
		Amount:       body.Amount,
		Currency:     body.Currency,
	})
	if err != nil {
		response.Error(w, shared.ErrInternal("transfer failed"))
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// WithdrawBalance POST /api/v1/aggregator/sub-accounts/:id/withdraw
func (h *AggregatorPortalHandlers) WithdrawBalance(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		response.Error(w, shared.ErrUnauthorized("not authenticated"))
		return
	}
	subID := mux.Vars(r)["id"]

	var body struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, shared.ErrInvalidInput("invalid request body"))
		return
	}

	resp, err := h.aggregatorClient.WithdrawBalance(r.Context(), &aggregatorv1.WithdrawBalanceRequest{
		AggregatorId: aggID.String(),
		SubAccountId: subID,
		Amount:       body.Amount,
		Currency:     body.Currency,
	})
	if err != nil {
		response.Error(w, shared.ErrInternal("withdraw failed"))
		return
	}
	response.JSON(w, http.StatusOK, resp)
}

// UnfreezeAllSubAccounts POST /api/v1/aggregator/sub-accounts/unfreeze-all
func (h *AggregatorPortalHandlers) UnfreezeAllSubAccounts(w http.ResponseWriter, r *http.Request) {
	aggID, ok := middleware.GetClientID(r.Context())
	if !ok {
		response.Error(w, shared.ErrUnauthorized("not authenticated"))
		return
	}

	resp, err := h.aggregatorClient.UnfreezeAllSubAccounts(r.Context(), &aggregatorv1.UnfreezeAllSubAccountsRequest{
		AggregatorId: aggID.String(),
	})
	if err != nil {
		response.Error(w, shared.ErrInternal("unfreeze failed"))
		return
	}
	response.JSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/gateway/portal/handlers/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/aggregator_portal.go
git commit -m "feat(aggregator): add portal HTTP handlers for aggregator self-service"
```

---

## Task 13: Wire Up — Clients, Router, account_type in Context

**Files:**
- Modify: `internal/gateway/portal/clients.go`
- Modify: `internal/gateway/portal/router/router.go`
- Modify: `internal/gateway/portal/middleware/session_auth.go`

- [ ] **Step 1: Add AggregatorClient to ServiceClients**

In `internal/gateway/portal/clients.go`, add to the `ServiceClients` struct:

```go
AggregatorClient aggregatorv1.AggregatorServiceClient
```

Add import: `aggregatorv1 "github.com/smpp-server/smpp-server/api/proto/aggregatorv1"`

In the `Connect` or `NewServiceClients` function, add:

```go
sc.AggregatorClient = aggregatorv1.NewAggregatorServiceClient(aggregatorConn)
```

(Use the same gRPC connection pattern as existing clients in that file — connect to the aggregator service address.)

Also add to `ServiceAddresses`:

```go
Aggregator string
```

- [ ] **Step 2: Add account_type to auth context**

In `internal/gateway/portal/middleware/session_auth.go`, add a new context key and populate it from the session data:

```go
AccountTypeKey contextKey = "account_type"

// In GetAccountType:
func GetAccountType(ctx context.Context) (string, bool) {
    v, ok := ctx.Value(AccountTypeKey).(string)
    return v, ok
}
```

In the session validation block (where `client_id` is extracted from `data`), add:

```go
accountType := data["account_type"]
if accountType == "" {
    accountType = "direct"
}
ctx = context.WithValue(ctx, AccountTypeKey, accountType)
```

The session data needs to include `account_type` when the session is created. Find where the session is written (likely in the auth service) and add `account_type` to the session payload. Look in `internal/services/auth/` for session creation.

- [ ] **Step 3: Add aggregator routes to router**

In `internal/gateway/portal/router/router.go`, add parameters for the new handlers in `SetupRouter`:

```go
aggregatorPortalHandlers *handlers.AggregatorPortalHandlers,
aggregatorAdminHandlers *handlers.AggregatorAdminHandlers,
```

Add the route groups inside `SetupRouter`:

```go
// Aggregator self-service routes (requires account_type=aggregator)
aggregatorAPI := protected.PathPrefix("/aggregator").Subrouter()
aggregatorAPI.Use(middleware.RequireAccountType("aggregator"))
aggregatorAPI.HandleFunc("/sub-accounts", aggregatorPortalHandlers.ListSubAccounts).Methods("GET")
aggregatorAPI.HandleFunc("/sub-accounts", aggregatorPortalHandlers.CreateSubAccount).Methods("POST")
aggregatorAPI.HandleFunc("/sub-accounts/unfreeze-all", aggregatorPortalHandlers.UnfreezeAllSubAccounts).Methods("POST")
aggregatorAPI.HandleFunc("/sub-accounts/{id}", aggregatorPortalHandlers.GetSubAccount).Methods("GET")
aggregatorAPI.HandleFunc("/sub-accounts/{id}", aggregatorPortalHandlers.UpdateSubAccount).Methods("PUT")
aggregatorAPI.HandleFunc("/sub-accounts/{id}/deactivate", aggregatorPortalHandlers.DeactivateSubAccount).Methods("POST")
aggregatorAPI.HandleFunc("/sub-accounts/{id}/transfer", aggregatorPortalHandlers.TransferBalance).Methods("POST")
aggregatorAPI.HandleFunc("/sub-accounts/{id}/withdraw", aggregatorPortalHandlers.WithdrawBalance).Methods("POST")

// Admin aggregator management
adminAPI := protected.PathPrefix("/admin/aggregators").Subrouter()
adminAPI.Use(middleware.RequireRole("admin"))
adminAPI.HandleFunc("", aggregatorAdminHandlers.ListAggregatorProfiles).Methods("GET")
adminAPI.HandleFunc("", aggregatorAdminHandlers.CreateAggregatorProfile).Methods("POST")
adminAPI.HandleFunc("/{client_id}", aggregatorAdminHandlers.GetAggregatorProfile).Methods("GET")
adminAPI.HandleFunc("/{client_id}", aggregatorAdminHandlers.UpdateAggregatorProfile).Methods("PUT")
```

- [ ] **Step 4: Add RequireAccountType middleware**

In `internal/gateway/portal/middleware/session_auth.go`, add:

```go
// RequireAccountType returns middleware that checks the account_type in context.
func RequireAccountType(required string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            accountType, ok := GetAccountType(r.Context())
            if !ok || accountType != required {
                response.Error(w, shared.ErrForbidden("доступ запрещён для данного типа аккаунта"))
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

- [ ] **Step 5: Build the full project**

```bash
go build ./...
```

Fix any remaining compile errors (missing imports, wrong parameter types, etc.).

- [ ] **Step 6: Commit**

```bash
git add internal/gateway/portal/clients.go \
        internal/gateway/portal/router/router.go \
        internal/gateway/portal/middleware/session_auth.go
git commit -m "feat(aggregator): wire up gRPC client, router routes, and account_type middleware"
```

---

## Task 14: Frontend — API Client

**Files:**
- Create: `portal-frontend/src/api/aggregator.ts`

- [ ] **Step 1: Write API client**

```typescript
// portal-frontend/src/api/aggregator.ts

const BASE = '/api/v1/aggregator';

export interface SubAccountInfo {
  id: string;
  aggregator_id: string;
  name: string;
  email: string;
  contact_person: string;
  active: boolean;
  virtual_balance: string;
  currency: string;
  daily_limit: number;
  monthly_limit: number;
  monthly_sms_count: number;
  created_at: string;
}

export interface CreateSubAccountRequest {
  name: string;
  email: string;
  contact_person?: string;
  initial_balance?: string;
  currency?: string;
  daily_limit?: number;
  monthly_limit?: number;
}

export interface TransferBalanceRequest {
  amount: string;
  currency?: string;
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: { message: res.statusText } }));
    throw new Error(err?.error?.message ?? res.statusText);
  }
  return res.json();
}

export const aggregatorApi = {
  listSubAccounts: () =>
    request<{ sub_accounts: SubAccountInfo[]; total: number }>(`${BASE}/sub-accounts`),

  getSubAccount: (id: string) =>
    request<SubAccountInfo>(`${BASE}/sub-accounts/${id}`),

  createSubAccount: (data: CreateSubAccountRequest) =>
    request<SubAccountInfo>(`${BASE}/sub-accounts`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateSubAccount: (id: string, data: Partial<CreateSubAccountRequest & { daily_limit: number; monthly_limit: number }>) =>
    request<SubAccountInfo>(`${BASE}/sub-accounts/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  deactivateSubAccount: (id: string) =>
    request<SubAccountInfo>(`${BASE}/sub-accounts/${id}/deactivate`, { method: 'POST' }),

  transferBalance: (id: string, data: TransferBalanceRequest) =>
    request<{ aggregator_balance: string; sub_account_balance: string }>(
      `${BASE}/sub-accounts/${id}/transfer`,
      { method: 'POST', body: JSON.stringify(data) }
    ),

  withdrawBalance: (id: string, data: TransferBalanceRequest) =>
    request<{ aggregator_balance: string; sub_account_balance: string }>(
      `${BASE}/sub-accounts/${id}/withdraw`,
      { method: 'POST', body: JSON.stringify(data) }
    ),

  unfreezeAll: () =>
    request<{ unfrozen_count: number }>(`${BASE}/sub-accounts/unfreeze-all`, { method: 'POST' }),
};
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/api/aggregator.ts
git commit -m "feat(aggregator): add frontend API client"
```

---

## Task 15: Frontend — SubAccountsListPage

**Files:**
- Create: `portal-frontend/src/pages/aggregator/SubAccountsListPage.tsx`

- [ ] **Step 1: Create page**

```tsx
// portal-frontend/src/pages/aggregator/SubAccountsListPage.tsx
import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { aggregatorApi, SubAccountInfo, CreateSubAccountRequest } from '../../api/aggregator';
import * as Dialog from '@radix-ui/react-dialog';

export default function SubAccountsListPage() {
  const [subAccounts, setSubAccounts] = useState<SubAccountInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState<CreateSubAccountRequest>({ name: '', email: '' });

  useEffect(() => {
    aggregatorApi.listSubAccounts()
      .then(r => setSubAccounts(r.sub_accounts ?? []))
      .catch(e => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setCreating(true);
    try {
      const sub = await aggregatorApi.createSubAccount(form);
      setSubAccounts(prev => [sub, ...prev]);
      setShowCreate(false);
      setForm({ name: '', email: '' });
    } catch (e: any) {
      setError(e.message);
    } finally {
      setCreating(false);
    }
  }

  if (loading) return <div className="p-6 text-sm text-gray-500">Загрузка...</div>;

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Субаккаунты</h1>
          <p className="text-sm text-gray-500 mt-1">Управление клиентами вашего пула</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700"
        >
          + Создать субаккаунт
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-red-700 text-sm">{error}</div>
      )}

      <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 border-b border-gray-200">
            <tr>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Название</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Email</th>
              <th className="text-right px-4 py-3 font-medium text-gray-600">Баланс</th>
              <th className="text-right px-4 py-3 font-medium text-gray-600">SMS / мес</th>
              <th className="text-center px-4 py-3 font-medium text-gray-600">Статус</th>
              <th className="px-4 py-3" />
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {subAccounts.length === 0 && (
              <tr>
                <td colSpan={6} className="text-center py-10 text-gray-400">
                  Субаккаунты не найдены. Создайте первый.
                </td>
              </tr>
            )}
            {subAccounts.map(sub => (
              <tr key={sub.id} className="hover:bg-gray-50 transition-colors">
                <td className="px-4 py-3 font-medium text-gray-900">{sub.name}</td>
                <td className="px-4 py-3 text-gray-500">{sub.email}</td>
                <td className="px-4 py-3 text-right font-mono text-gray-900">
                  {Number(sub.virtual_balance).toLocaleString('ru-RU', { minimumFractionDigits: 2 })} ₽
                </td>
                <td className="px-4 py-3 text-right text-gray-700">
                  {sub.monthly_sms_count.toLocaleString('ru-RU')}
                </td>
                <td className="px-4 py-3 text-center">
                  <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                    sub.active ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'
                  }`}>
                    {sub.active ? 'Активен' : 'Неактивен'}
                  </span>
                </td>
                <td className="px-4 py-3 text-right">
                  <Link
                    to={`/aggregator/sub-accounts/${sub.id}`}
                    className="text-blue-600 hover:underline text-xs"
                  >
                    Управление →
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Dialog.Root open={showCreate} onOpenChange={setShowCreate}>
        <Dialog.Portal>
          <Dialog.Overlay className="fixed inset-0 bg-black/40" />
          <Dialog.Content className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 bg-white rounded-xl shadow-xl p-6 w-full max-w-md">
            <Dialog.Title className="text-lg font-semibold mb-4">Новый субаккаунт</Dialog.Title>
            <form onSubmit={handleCreate} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Название *</label>
                <input
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                  value={form.name}
                  onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Email *</label>
                <input
                  type="email"
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                  value={form.email}
                  onChange={e => setForm(f => ({ ...f, email: e.target.value }))}
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Начальный баланс (₽)</label>
                <input
                  type="number"
                  min="0"
                  step="0.01"
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                  value={form.initial_balance ?? ''}
                  onChange={e => setForm(f => ({ ...f, initial_balance: e.target.value }))}
                />
              </div>
              <div className="flex gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreate(false)}
                  className="flex-1 border border-gray-300 rounded-lg px-4 py-2 text-sm hover:bg-gray-50"
                >
                  Отмена
                </button>
                <button
                  type="submit"
                  disabled={creating}
                  className="flex-1 bg-blue-600 text-white rounded-lg px-4 py-2 text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
                >
                  {creating ? 'Создание...' : 'Создать'}
                </button>
              </div>
            </form>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/aggregator/SubAccountsListPage.tsx
git commit -m "feat(aggregator): add SubAccountsListPage with create dialog"
```

---

## Task 16: Frontend — SubAccountDetailPage + BalanceTab

**Files:**
- Create: `portal-frontend/src/pages/aggregator/tabs/BalanceTab.tsx`
- Create: `portal-frontend/src/pages/aggregator/SubAccountDetailPage.tsx`

- [ ] **Step 1: Create BalanceTab**

```tsx
// portal-frontend/src/pages/aggregator/tabs/BalanceTab.tsx
import { useState } from 'react';
import { aggregatorApi, SubAccountInfo } from '../../../api/aggregator';

interface Props {
  sub: SubAccountInfo;
  onBalanceChanged: (sub: SubAccountInfo) => void;
}

export default function BalanceTab({ sub, onBalanceChanged }: Props) {
  const [transferAmount, setTransferAmount] = useState('');
  const [withdrawAmount, setWithdrawAmount] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  async function handleTransfer(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setSuccess(null);
    try {
      await aggregatorApi.transferBalance(sub.id, { amount: transferAmount, currency: 'RUB' });
      setSuccess(`Переведено ${transferAmount} ₽`);
      setTransferAmount('');
      const updated = await aggregatorApi.getSubAccount(sub.id);
      onBalanceChanged(updated);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }

  async function handleWithdraw(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setSuccess(null);
    try {
      await aggregatorApi.withdrawBalance(sub.id, { amount: withdrawAmount, currency: 'RUB' });
      setSuccess(`Изъято ${withdrawAmount} ₽`);
      setWithdrawAmount('');
      const updated = await aggregatorApi.getSubAccount(sub.id);
      onBalanceChanged(updated);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-6">
      <div className="bg-gray-50 rounded-xl p-4 flex items-center gap-4">
        <div>
          <p className="text-sm text-gray-500">Виртуальный баланс</p>
          <p className="text-2xl font-semibold text-gray-900">
            {Number(sub.virtual_balance).toLocaleString('ru-RU', { minimumFractionDigits: 2 })} ₽
          </p>
        </div>
      </div>

      {error && <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-red-700 text-sm">{error}</div>}
      {success && <div className="p-3 bg-green-50 border border-green-200 rounded-lg text-green-700 text-sm">{success}</div>}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <form onSubmit={handleTransfer} className="bg-white border border-gray-200 rounded-xl p-4 space-y-3">
          <h3 className="font-medium text-gray-900">Перевести баланс</h3>
          <p className="text-sm text-gray-500">Пополнить виртуальный баланс субаккаунта из вашего баланса</p>
          <input
            type="number"
            min="0.01"
            step="0.01"
            placeholder="Сумма в рублях"
            className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            value={transferAmount}
            onChange={e => setTransferAmount(e.target.value)}
            required
          />
          <button
            type="submit"
            disabled={busy || !transferAmount}
            className="w-full bg-blue-600 text-white rounded-lg px-4 py-2 text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
          >
            Перевести
          </button>
        </form>

        <form onSubmit={handleWithdraw} className="bg-white border border-gray-200 rounded-xl p-4 space-y-3">
          <h3 className="font-medium text-gray-900">Изъять баланс</h3>
          <p className="text-sm text-gray-500">Вернуть средства с виртуального баланса субаккаунта на ваш баланс</p>
          <input
            type="number"
            min="0.01"
            step="0.01"
            placeholder="Сумма в рублях"
            className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            value={withdrawAmount}
            onChange={e => setWithdrawAmount(e.target.value)}
            required
          />
          <button
            type="submit"
            disabled={busy || !withdrawAmount}
            className="w-full border border-gray-300 text-gray-700 rounded-lg px-4 py-2 text-sm font-medium hover:bg-gray-50 disabled:opacity-50"
          >
            Изъять
          </button>
        </form>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Create SubAccountDetailPage**

```tsx
// portal-frontend/src/pages/aggregator/SubAccountDetailPage.tsx
import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { aggregatorApi, SubAccountInfo } from '../../api/aggregator';
import BalanceTab from './tabs/BalanceTab';

type Tab = 'balance';

export default function SubAccountDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [sub, setSub] = useState<SubAccountInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>('balance');
  const [deactivating, setDeactivating] = useState(false);

  useEffect(() => {
    if (!id) return;
    aggregatorApi.getSubAccount(id)
      .then(setSub)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false));
  }, [id]);

  async function handleDeactivate() {
    if (!sub || !confirm(`Деактивировать ${sub.name}?`)) return;
    setDeactivating(true);
    try {
      const updated = await aggregatorApi.deactivateSubAccount(sub.id);
      setSub(updated);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setDeactivating(false);
    }
  }

  if (loading) return <div className="p-6 text-sm text-gray-500">Загрузка...</div>;
  if (error) return <div className="p-6 text-red-600 text-sm">{error}</div>;
  if (!sub) return null;

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="flex items-center gap-2 text-sm text-gray-500 mb-4">
        <Link to="/aggregator/sub-accounts" className="hover:text-gray-900">Субаккаунты</Link>
        <span>/</span>
        <span className="text-gray-900">{sub.name}</span>
      </div>

      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">{sub.name}</h1>
          <p className="text-sm text-gray-500 mt-0.5">{sub.email}</p>
        </div>
        <div className="flex items-center gap-3">
          <span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium ${
            sub.active ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'
          }`}>
            {sub.active ? 'Активен' : 'Неактивен'}
          </span>
          {sub.active && (
            <button
              onClick={handleDeactivate}
              disabled={deactivating}
              className="text-sm text-red-600 hover:text-red-700 border border-red-200 rounded-lg px-3 py-1.5 hover:bg-red-50 disabled:opacity-50"
            >
              Деактивировать
            </button>
          )}
        </div>
      </div>

      <div className="flex gap-1 border-b border-gray-200 mb-6">
        {(['balance'] as Tab[]).map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === tab
                ? 'border-blue-600 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-900'
            }`}
          >
            {tab === 'balance' ? 'Баланс' : tab}
          </button>
        ))}
      </div>

      {activeTab === 'balance' && (
        <BalanceTab sub={sub} onBalanceChanged={setSub} />
      )}
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/aggregator/
git commit -m "feat(aggregator): add SubAccountDetailPage with BalanceTab"
```

---

## Task 17: Frontend — Routing + Sidebar

**Files:**
- Modify: `portal-frontend/src/App.tsx` (or router file)
- Modify: `portal-frontend/src/components/layout/Sidebar.tsx`

- [ ] **Step 1: Add routes**

Find where React Router routes are defined (likely `App.tsx` or `router.tsx`). Add:

```tsx
import SubAccountsListPage from './pages/aggregator/SubAccountsListPage';
import SubAccountDetailPage from './pages/aggregator/SubAccountDetailPage';

// Inside the route definitions (protected routes):
<Route path="/aggregator/sub-accounts" element={<SubAccountsListPage />} />
<Route path="/aggregator/sub-accounts/:id" element={<SubAccountDetailPage />} />
```

- [ ] **Step 2: Add sidebar section for aggregators**

In `portal-frontend/src/components/layout/Sidebar.tsx`, find where nav items are rendered. Add a conditional section that shows only when `accountType === 'aggregator'`:

```tsx
// The accountType should come from auth context/store.
// Find how `role` or user data is accessed in the existing Sidebar, use the same pattern.

{accountType === 'aggregator' && (
  <div className="mt-4">
    <p className="px-3 py-1 text-xs font-semibold text-gray-400 uppercase tracking-wider">
      Мои клиенты
    </p>
    <nav className="space-y-0.5">
      <NavLink to="/aggregator/sub-accounts" icon={UsersIcon}>
        Субаккаунты
      </NavLink>
    </nav>
  </div>
)}
```

Use the same `NavLink` component or pattern that already exists in the Sidebar.

- [ ] **Step 3: Build frontend**

```bash
cd portal-frontend
npm run build
```

Expected: build succeeds with no TypeScript errors.

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/
git commit -m "feat(aggregator): add aggregator routes and sidebar navigation"
```

---

## Task 18: End-to-End Smoke Test

- [ ] **Step 1: Start the stack**

```bash
scripts/server.sh status
# or locally:
docker compose up -d
```

- [ ] **Step 2: Create an aggregator via admin API**

```bash
# 1. Login as admin
TOKEN=$(curl -s -X POST http://localhost:8080/portal/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"admin"}' | jq -r '.token')

# 2. Create a client with account_type=aggregator (via existing admin client endpoint)
AGG_ID=$(curl -s -X POST http://localhost:8080/admin/v1/clients \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Test Aggregator","email":"agg@test.com","account_type":"aggregator"}' | jq -r '.id')
echo "Aggregator ID: $AGG_ID"

# 3. Create aggregator profile
curl -s -X POST http://localhost:8080/api/v1/admin/aggregators \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"client_id\":\"$AGG_ID\",\"max_sub_accounts\":10,\"auto_freeze_sub_accounts\":true}" | jq .
```

Expected: `200 OK` with profile JSON.

- [ ] **Step 3: Login as aggregator and create sub-account**

```bash
AGG_TOKEN=$(curl -s -X POST http://localhost:8080/portal/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"agg@test.com","password":"changeme"}' | jq -r '.token')

SUB_ID=$(curl -s -X POST http://localhost:8080/api/v1/aggregator/sub-accounts \
  -H "Authorization: Bearer $AGG_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Client One","email":"client1@test.com"}' | jq -r '.id')
echo "Sub-account ID: $SUB_ID"
```

Expected: `201 Created` with sub-account JSON.

- [ ] **Step 4: List sub-accounts**

```bash
curl -s http://localhost:8080/api/v1/aggregator/sub-accounts \
  -H "Authorization: Bearer $AGG_TOKEN" | jq '.sub_accounts | length'
```

Expected: `1`

- [ ] **Step 5: Commit final state**

```bash
git add -A
git commit -m "feat(aggregator): Phase 1 complete — core account types, profiles, sub-accounts, balance transfer"
```

---

## Self-Review

**Spec coverage check:**
- ✅ account_type replaces is_reseller — Task 1, Task 9
- ✅ aggregator_profiles table — Task 2
- ✅ virtual balance (is_virtual, allocated_by) — Task 3
- ✅ aggregator_audit_log — Task 3
- ✅ Proto definition — Task 4
- ✅ Domain models + errors — Task 5
- ✅ Profile CRUD repository — Task 6
- ✅ Balance transfer + withdraw + audit — Task 7
- ✅ Service layer with validation — Task 8
- ✅ gRPC server — Task 10
- ✅ Admin HTTP handlers — Task 11
- ✅ Portal HTTP handlers — Task 12
- ✅ account_type in auth context + RequireAccountType middleware — Task 13
- ✅ Router registration — Task 13
- ✅ Frontend API client — Task 14
- ✅ SubAccountsListPage — Task 15
- ✅ SubAccountDetailPage + BalanceTab — Task 16
- ✅ Routing + Sidebar — Task 17

**Not in this plan (separate plans):**
- Phase 2: Tarification (aggregator_tariffs, dual deduction, margin log)
- Phase 3: Routing (aggregator route management, default routes)
- Phase 4: Analytics (margin dashboard, simulator, export)
- Phase 5: Control (pool rate-limiting, pool monthly limits)
- UnfreezeAllSubAccounts gRPC implementation in service layer (stub exists, needs `billing.UnfreezeAccount` calls per sub-account)
- CreateSubAccount / DeactivateSubAccount gRPC implementations (Task 10 stubs, need client service calls)

> **Note on Task 10:** The gRPC server in Task 10 implements profile management and balance operations fully. `CreateSubAccount`, `GetSubAccount`, `ListSubAccounts`, `DeactivateSubAccount` call the existing `clientv1.ClientServiceClient` under the hood (create client with `account_type=sub_account`, list by `parent_client_id`, set active=false). Add a `clientClient clientv1.ClientServiceClient` dependency to the gRPC Server struct and delegate accordingly — the pattern is identical to how `internal/gateway/portal/handlers/sub_accounts.go` already works.
