# Aggregator Routing — Plan 1: Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (CLAUDE.md mandates `/execute-with-review` wrapper for any code work — review-gate is non-negotiable, see memory `feedback_review_gate_no_skip`). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Заменить read-only страницу `/network/routing` управлением каталогом провайдеров и provider-set'ами с назначением и override на уровне суб-аккаунта. После плана агрегатор может: добавлять private-провайдеров, делать provider-set'ы, назначать их одному или массово суб-аккаунтам, добавлять провайдеров override'ом в карточке суб-аккаунта.

**Architecture:** Шаблоны (`reseller_provider_sets` + items) хранятся отдельно от рабочих таблиц pipeline'а. При назначении/изменении шаблона `client_providers` материализуется под транзакцией: записи `ownership='inherited'` стираются и пересоздаются из items, `ownership='private'` (override) не трогаются. Hot-path роутера не меняется.

**Tech Stack:** Go 1.24 (gorilla/mux, jackc/pgx/v5, testify, zerolog), React 19 + TS 5.7 (Vite, Radix UI, Tailwind 4.2), Playwright для E2E.

**Spec:** [docs/superpowers/specs/2026-05-04-aggregator-routing-management-design.md](../specs/2026-05-04-aggregator-routing-management-design.md)

**Соглашение по details в 409:** `AppError.Details` — строка. Структурные details кодируем как JSON-строку, фронт декодирует на клиенте. Пример: `shared.ErrConflict("...").WithDetails(string(jsonBytes))`.

**Скоуп НЕ Plan 1 (уйдёт в Plan 2):** route-sets, route-set-items, condition-groups, schedules, preview, override-маршруты, страница `/network/assignments` с полным UI (в Plan 1 — упрощённая страница только с provider-set'ами), удаление старого `NetworkRoutingPage.tsx`. До начала Plan 2 старая страница остаётся, новый `/network/providers` и `/network/provider-sets` доступны через сайдбар, `/network/routing` пока неизменён.

---

## Phase A — Migrations

### Task 1: Создать миграции 000129-000131

**Files:**
- Create: `migrations/000129_create_reseller_provider_sets.up.sql`
- Create: `migrations/000129_create_reseller_provider_sets.down.sql`
- Create: `migrations/000130_create_reseller_provider_set_items.up.sql`
- Create: `migrations/000130_create_reseller_provider_set_items.down.sql`
- Create: `migrations/000131_create_subaccount_routing_assignment.up.sql`
- Create: `migrations/000131_create_subaccount_routing_assignment.down.sql`

- [ ] **Step 1.1: Написать `000129_create_reseller_provider_sets.up.sql`**

```sql
CREATE TABLE IF NOT EXISTS reseller_provider_sets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reseller_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_reseller_provider_sets_name UNIQUE (reseller_id, name)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_reseller_provider_sets_default
    ON reseller_provider_sets (reseller_id) WHERE is_default = true;

CREATE INDEX IF NOT EXISTS idx_reseller_provider_sets_reseller
    ON reseller_provider_sets (reseller_id);

CREATE TRIGGER update_reseller_provider_sets_updated_at
    BEFORE UPDATE ON reseller_provider_sets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

- [ ] **Step 1.2: Написать `000129_create_reseller_provider_sets.down.sql`**

```sql
DROP TRIGGER IF EXISTS update_reseller_provider_sets_updated_at ON reseller_provider_sets;
DROP INDEX IF EXISTS uq_reseller_provider_sets_default;
DROP INDEX IF EXISTS idx_reseller_provider_sets_reseller;
DROP TABLE IF EXISTS reseller_provider_sets;
```

- [ ] **Step 1.3: Написать `000130_create_reseller_provider_set_items.up.sql`**

```sql
CREATE TABLE IF NOT EXISTS reseller_provider_set_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    set_id UUID NOT NULL REFERENCES reseller_provider_sets(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
    priority INT NOT NULL DEFAULT 0,
    expose_cost BOOLEAN NOT NULL DEFAULT false,
    expose_provider_name BOOLEAN NOT NULL DEFAULT true,
    CONSTRAINT uq_reseller_provider_set_items UNIQUE (set_id, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_reseller_provider_set_items_set
    ON reseller_provider_set_items (set_id);
CREATE INDEX IF NOT EXISTS idx_reseller_provider_set_items_provider
    ON reseller_provider_set_items (provider_id);
```

- [ ] **Step 1.4: Написать `000130_create_reseller_provider_set_items.down.sql`**

```sql
DROP INDEX IF EXISTS idx_reseller_provider_set_items_set;
DROP INDEX IF EXISTS idx_reseller_provider_set_items_provider;
DROP TABLE IF EXISTS reseller_provider_set_items;
```

- [ ] **Step 1.5: Написать `000131_create_subaccount_routing_assignment.up.sql`**

```sql
CREATE TABLE IF NOT EXISTS subaccount_routing_assignment (
    client_id UUID PRIMARY KEY REFERENCES clients(id) ON DELETE CASCADE,
    provider_set_id UUID REFERENCES reseller_provider_sets(id) ON DELETE SET NULL,
    route_set_id UUID, -- FK добавится в Plan 2 (после создания reseller_route_sets)
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sra_provider_set
    ON subaccount_routing_assignment (provider_set_id);
```

- [ ] **Step 1.6: Написать `000131_create_subaccount_routing_assignment.down.sql`**

```sql
DROP INDEX IF EXISTS idx_sra_provider_set;
DROP TABLE IF EXISTS subaccount_routing_assignment;
```

- [ ] **Step 1.7: Запустить миграции локально (SSH на sms-server)**

Run (с локальной машины):
```
./scripts/server.sh migrate
```
Expected: `migrations applied: 129, 130, 131` (или эквивалент). Проверить:
```
./scripts/server.sh exec "psql -U sms -d sms -c '\d reseller_provider_sets'"
```
Должна вернуться структура таблицы.

- [ ] **Step 1.8: Commit**

```bash
git add migrations/000129_create_reseller_provider_sets.up.sql migrations/000129_create_reseller_provider_sets.down.sql migrations/000130_create_reseller_provider_set_items.up.sql migrations/000130_create_reseller_provider_set_items.down.sql migrations/000131_create_subaccount_routing_assignment.up.sql migrations/000131_create_subaccount_routing_assignment.down.sql
git commit -m "feat(migrations): provider-sets + assignments tables (Plan 1 foundation)"
```

---

## Phase B — Backend storage layer

### Task 2: Repo для provider-sets и items

**Files:**
- Create: `internal/storage/reseller_provider_set_repository.go`
- Create: `internal/storage/reseller_provider_set_repository_test.go`

- [ ] **Step 2.1: Написать failing test для `Create` + `GetByID`**

`reseller_provider_set_repository_test.go`:
```go
package storage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProviderSetRepo_CreateAndGet(t *testing.T) {
	pool, cleanup := setupTestDB(t) // существующий хелпер в этом пакете
	defer cleanup()

	resellerID := seedTestReseller(t, pool)
	repo := NewResellerProviderSetRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, resellerID, "Стандарт", false)
	require.NoError(t, err)
	require.Equal(t, "Стандарт", created.Name)
	require.False(t, created.IsDefault)

	fetched, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.Name, fetched.Name)
	require.Equal(t, resellerID, fetched.ResellerID)
}

func TestProviderSetRepo_DefaultUniqueness(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	resellerID := seedTestReseller(t, pool)
	repo := NewResellerProviderSetRepository(pool)
	ctx := context.Background()

	_, err := repo.Create(ctx, resellerID, "A", true)
	require.NoError(t, err)
	_, err = repo.Create(ctx, resellerID, "B", true)
	require.Error(t, err) // unique constraint
}
```

- [ ] **Step 2.2: Если хелперы `setupTestDB` / `seedTestReseller` отсутствуют в `internal/storage/` — добавить их**

Проверить: `grep -rn "func setupTestDB" c:/projects/sms/internal/storage/`. Если нет — создать `internal/storage/testutil_test.go` с реализацией через `testcontainers-go` (если она используется в проекте) либо через `pgxpool.New` к существующему dev-БД (см. как делает `internal/services/billing/billing_test.go`). **ВАЖНО:** не дублируй существующее — используй паттерн из соседних `_test.go` в storage/.

Run: `go test ./internal/storage/ -run TestProviderSetRepo_CreateAndGet -v`
Expected: FAIL — `NewResellerProviderSetRepository` не существует.

- [ ] **Step 2.3: Реализовать репозиторий**

`reseller_provider_set_repository.go`:
```go
package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ResellerProviderSet — шаблон каталога провайдеров для агрегатора.
type ResellerProviderSet struct {
	ID         uuid.UUID
	ResellerID uuid.UUID
	Name       string
	IsDefault  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ResellerProviderSetRepository struct {
	pool *pgxpool.Pool
}

func NewResellerProviderSetRepository(pool *pgxpool.Pool) *ResellerProviderSetRepository {
	return &ResellerProviderSetRepository{pool: pool}
}

func (r *ResellerProviderSetRepository) Create(ctx context.Context, resellerID uuid.UUID, name string, isDefault bool) (*ResellerProviderSet, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO reseller_provider_sets (reseller_id, name, is_default)
		 VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`,
		resellerID, name, isDefault)
	out := &ResellerProviderSet{ResellerID: resellerID, Name: name, IsDefault: isDefault}
	if err := row.Scan(&out.ID, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ResellerProviderSetRepository) GetByID(ctx context.Context, id uuid.UUID) (*ResellerProviderSet, error) {
	var s ResellerProviderSet
	err := r.pool.QueryRow(ctx,
		`SELECT id, reseller_id, name, is_default, created_at, updated_at
		 FROM reseller_provider_sets WHERE id = $1`, id,
	).Scan(&s.ID, &s.ResellerID, &s.Name, &s.IsDefault, &s.CreatedAt, &s.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return &s, err
}

func (r *ResellerProviderSetRepository) ListByReseller(ctx context.Context, resellerID uuid.UUID) ([]ResellerProviderSet, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, reseller_id, name, is_default, created_at, updated_at
		 FROM reseller_provider_sets WHERE reseller_id = $1
		 ORDER BY created_at DESC`, resellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResellerProviderSet
	for rows.Next() {
		var s ResellerProviderSet
		if err := rows.Scan(&s.ID, &s.ResellerID, &s.Name, &s.IsDefault, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ResellerProviderSetRepository) Update(ctx context.Context, id uuid.UUID, name string, isDefault bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE reseller_provider_sets SET name=$1, is_default=$2, updated_at=now() WHERE id=$3`,
		name, isDefault, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ResellerProviderSetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM reseller_provider_sets WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
```

Если `ErrNotFound` отсутствует в `internal/storage/errors.go` — переиспользовать существующий sentinel или добавить `var ErrNotFound = errors.New("not found")`. Проверить.

- [ ] **Step 2.4: Run tests**

Run: `go test ./internal/storage/ -run TestProviderSetRepo -v`
Expected: PASS обоих кейсов.

- [ ] **Step 2.5: Добавить тесты Update / Delete / ListByReseller**

Дополнить `_test.go`:
```go
func TestProviderSetRepo_Update(t *testing.T) {
	pool, cleanup := setupTestDB(t); defer cleanup()
	resellerID := seedTestReseller(t, pool)
	repo := NewResellerProviderSetRepository(pool)
	ctx := context.Background()
	s, _ := repo.Create(ctx, resellerID, "old", false)
	require.NoError(t, repo.Update(ctx, s.ID, "new", true))
	got, _ := repo.GetByID(ctx, s.ID)
	require.Equal(t, "new", got.Name); require.True(t, got.IsDefault)
}

func TestProviderSetRepo_Delete(t *testing.T) {
	pool, cleanup := setupTestDB(t); defer cleanup()
	resellerID := seedTestReseller(t, pool)
	repo := NewResellerProviderSetRepository(pool)
	ctx := context.Background()
	s, _ := repo.Create(ctx, resellerID, "x", false)
	require.NoError(t, repo.Delete(ctx, s.ID))
	_, err := repo.GetByID(ctx, s.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestProviderSetRepo_ListByReseller(t *testing.T) {
	pool, cleanup := setupTestDB(t); defer cleanup()
	resellerID := seedTestReseller(t, pool)
	otherID := seedTestReseller(t, pool)
	repo := NewResellerProviderSetRepository(pool)
	ctx := context.Background()
	repo.Create(ctx, resellerID, "A", false)
	repo.Create(ctx, resellerID, "B", false)
	repo.Create(ctx, otherID, "Z", false)
	list, err := repo.ListByReseller(ctx, resellerID)
	require.NoError(t, err); require.Len(t, list, 2)
}
```

Run: `go test ./internal/storage/ -run TestProviderSetRepo -v`
Expected: ALL PASS.

- [ ] **Step 2.6: Commit**

```bash
git add internal/storage/reseller_provider_set_repository.go internal/storage/reseller_provider_set_repository_test.go
git commit -m "feat(storage): reseller_provider_sets repository"
```

---

### Task 3: Repo для provider-set items (PUT-replace)

**Files:**
- Create: `internal/storage/reseller_provider_set_items_repository.go`
- Create: `internal/storage/reseller_provider_set_items_repository_test.go`

- [ ] **Step 3.1: Failing test для `ReplaceItems`**

```go
package storage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProviderSetItems_ReplaceAndList(t *testing.T) {
	pool, cleanup := setupTestDB(t); defer cleanup()
	resellerID := seedTestReseller(t, pool)
	setRepo := NewResellerProviderSetRepository(pool)
	itemsRepo := NewResellerProviderSetItemsRepository(pool)
	providerA := seedTestProvider(t, pool, "ProvA")
	providerB := seedTestProvider(t, pool, "ProvB")
	ctx := context.Background()

	set, _ := setRepo.Create(ctx, resellerID, "S", false)

	err := itemsRepo.ReplaceItems(ctx, set.ID, []ProviderSetItemInput{
		{ProviderID: providerA, Priority: 10, ExposeCost: true, ExposeProviderName: true},
		{ProviderID: providerB, Priority: 5, ExposeCost: false, ExposeProviderName: true},
	})
	require.NoError(t, err)

	items, err := itemsRepo.ListBySet(ctx, set.ID)
	require.NoError(t, err); require.Len(t, items, 2)
}

func TestProviderSetItems_ReplaceClearsOld(t *testing.T) {
	pool, cleanup := setupTestDB(t); defer cleanup()
	resellerID := seedTestReseller(t, pool)
	setRepo := NewResellerProviderSetRepository(pool)
	itemsRepo := NewResellerProviderSetItemsRepository(pool)
	provA := seedTestProvider(t, pool, "A")
	provB := seedTestProvider(t, pool, "B")
	ctx := context.Background()
	set, _ := setRepo.Create(ctx, resellerID, "S", false)
	itemsRepo.ReplaceItems(ctx, set.ID, []ProviderSetItemInput{{ProviderID: provA, Priority: 10}})
	itemsRepo.ReplaceItems(ctx, set.ID, []ProviderSetItemInput{{ProviderID: provB, Priority: 5}})
	items, _ := itemsRepo.ListBySet(ctx, set.ID)
	require.Len(t, items, 1); require.Equal(t, provB, items[0].ProviderID)
}
```

Run: FAIL.

- [ ] **Step 3.2: Реализовать**

```go
package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProviderSetItem struct {
	ID                  uuid.UUID
	SetID               uuid.UUID
	ProviderID          uuid.UUID
	Priority            int
	ExposeCost          bool
	ExposeProviderName  bool
}

type ProviderSetItemInput struct {
	ProviderID          uuid.UUID
	Priority            int
	ExposeCost          bool
	ExposeProviderName  bool
}

type ResellerProviderSetItemsRepository struct {
	pool *pgxpool.Pool
}

func NewResellerProviderSetItemsRepository(pool *pgxpool.Pool) *ResellerProviderSetItemsRepository {
	return &ResellerProviderSetItemsRepository{pool: pool}
}

func (r *ResellerProviderSetItemsRepository) ListBySet(ctx context.Context, setID uuid.UUID) ([]ProviderSetItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, set_id, provider_id, priority, expose_cost, expose_provider_name
		 FROM reseller_provider_set_items WHERE set_id = $1 ORDER BY priority DESC, provider_id`, setID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []ProviderSetItem
	for rows.Next() {
		var i ProviderSetItem
		if err := rows.Scan(&i.ID, &i.SetID, &i.ProviderID, &i.Priority, &i.ExposeCost, &i.ExposeProviderName); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// ReplaceItems атомарно заменяет содержимое set'а.
// Pre-validation вызывается на уровне сервиса/handler'а ДО вызова этого метода.
func (r *ResellerProviderSetItemsRepository) ReplaceItems(ctx context.Context, setID uuid.UUID, items []ProviderSetItemInput) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM reseller_provider_set_items WHERE set_id = $1`, setID); err != nil {
		return err
	}
	for _, it := range items {
		_, err := tx.Exec(ctx,
			`INSERT INTO reseller_provider_set_items (set_id, provider_id, priority, expose_cost, expose_provider_name)
			 VALUES ($1, $2, $3, $4, $5)`,
			setID, it.ProviderID, it.Priority, it.ExposeCost, it.ExposeProviderName)
		if err != nil { return err }
	}
	return tx.Commit(ctx)
}

// ListProvidersInSet — для валидации, какие provider_id содержатся в set'е.
// Используется service-уровнем при проверке conflict provider-set ↔ route-set.
func (r *ResellerProviderSetItemsRepository) ListProvidersInSet(ctx context.Context, setID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT provider_id FROM reseller_provider_set_items WHERE set_id = $1`, setID)
	if err != nil { return nil, err }
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil { return nil, err }
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
```

Run: `go test ./internal/storage/ -run TestProviderSetItems -v`
Expected: PASS.

- [ ] **Step 3.3: Commit**

```bash
git add internal/storage/reseller_provider_set_items_repository.go internal/storage/reseller_provider_set_items_repository_test.go
git commit -m "feat(storage): reseller_provider_set_items repository with atomic replace"
```

---

### Task 4: Repo для assignments (subaccount_routing_assignment)

**Files:**
- Create: `internal/storage/subaccount_routing_assignment_repository.go`
- Create: `internal/storage/subaccount_routing_assignment_repository_test.go`

- [ ] **Step 4.1: Failing test**

```go
package storage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSRA_UpsertAndGet(t *testing.T) {
	pool, cleanup := setupTestDB(t); defer cleanup()
	resellerID := seedTestReseller(t, pool)
	subID := seedTestSubAccount(t, pool, resellerID)
	setRepo := NewResellerProviderSetRepository(pool)
	repo := NewSubAccountRoutingAssignmentRepository(pool)
	ctx := context.Background()
	set, _ := setRepo.Create(ctx, resellerID, "S", false)

	err := repo.Upsert(ctx, subID, &set.ID, nil)
	require.NoError(t, err)
	got, err := repo.GetByClient(ctx, subID)
	require.NoError(t, err)
	require.Equal(t, set.ID, *got.ProviderSetID)
	require.Nil(t, got.RouteSetID)
}

func TestSRA_ListByProviderSet(t *testing.T) {
	pool, cleanup := setupTestDB(t); defer cleanup()
	resellerID := seedTestReseller(t, pool)
	sub1 := seedTestSubAccount(t, pool, resellerID)
	sub2 := seedTestSubAccount(t, pool, resellerID)
	setRepo := NewResellerProviderSetRepository(pool)
	repo := NewSubAccountRoutingAssignmentRepository(pool)
	ctx := context.Background()
	set, _ := setRepo.Create(ctx, resellerID, "S", false)

	repo.Upsert(ctx, sub1, &set.ID, nil)
	repo.Upsert(ctx, sub2, &set.ID, nil)

	clients, err := repo.ListClientsByProviderSet(ctx, set.ID)
	require.NoError(t, err); require.Len(t, clients, 2)
}
```

- [ ] **Step 4.2: Реализация**

```go
package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SubAccountRoutingAssignment struct {
	ClientID       uuid.UUID
	ProviderSetID  *uuid.UUID
	RouteSetID     *uuid.UUID
	AssignedAt     time.Time
}

type SubAccountRoutingAssignmentRepository struct {
	pool *pgxpool.Pool
}

func NewSubAccountRoutingAssignmentRepository(pool *pgxpool.Pool) *SubAccountRoutingAssignmentRepository {
	return &SubAccountRoutingAssignmentRepository{pool: pool}
}

func (r *SubAccountRoutingAssignmentRepository) GetByClient(ctx context.Context, clientID uuid.UUID) (*SubAccountRoutingAssignment, error) {
	var a SubAccountRoutingAssignment
	err := r.pool.QueryRow(ctx,
		`SELECT client_id, provider_set_id, route_set_id, assigned_at
		 FROM subaccount_routing_assignment WHERE client_id = $1`, clientID,
	).Scan(&a.ClientID, &a.ProviderSetID, &a.RouteSetID, &a.AssignedAt)
	if err == pgx.ErrNoRows { return nil, ErrNotFound }
	return &a, err
}

// Upsert — UPSERT по client_id. nil = снять назначение.
func (r *SubAccountRoutingAssignmentRepository) Upsert(ctx context.Context, clientID uuid.UUID, providerSetID, routeSetID *uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (client_id) DO UPDATE SET
		   provider_set_id = EXCLUDED.provider_set_id,
		   route_set_id    = EXCLUDED.route_set_id,
		   assigned_at     = now()`,
		clientID, providerSetID, routeSetID)
	return err
}

func (r *SubAccountRoutingAssignmentRepository) ListClientsByProviderSet(ctx context.Context, providerSetID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT client_id FROM subaccount_routing_assignment WHERE provider_set_id = $1`, providerSetID)
	if err != nil { return nil, err }
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil { return nil, err }
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *SubAccountRoutingAssignmentRepository) ListByReseller(ctx context.Context, resellerID uuid.UUID) ([]SubAccountRoutingAssignment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT sra.client_id, sra.provider_set_id, sra.route_set_id, sra.assigned_at
		 FROM subaccount_routing_assignment sra
		 JOIN clients c ON c.id = sra.client_id
		 WHERE c.parent_client_id = $1`, resellerID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []SubAccountRoutingAssignment
	for rows.Next() {
		var a SubAccountRoutingAssignment
		if err := rows.Scan(&a.ClientID, &a.ProviderSetID, &a.RouteSetID, &a.AssignedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
```

`seedTestSubAccount` — добавить в test-helpers, если ещё нет: создаёт `clients` с `parent_client_id = resellerID`.

Run: `go test ./internal/storage/ -run TestSRA -v`
Expected: PASS.

- [ ] **Step 4.3: Commit**

```bash
git add internal/storage/subaccount_routing_assignment_repository.go internal/storage/subaccount_routing_assignment_repository_test.go
git commit -m "feat(storage): subaccount_routing_assignment repository"
```

---

## Phase C — Service layer: материализация

### Task 5: ProviderSetMaterializer

**Files:**
- Create: `internal/services/network/provider_set_materializer.go`
- Create: `internal/services/network/provider_set_materializer_test.go`

- [ ] **Step 5.1: Failing test**

```go
package network

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func TestMaterializer_Apply_CreatesInheritedRows(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t) // экспортируем существующий хелпер
	defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	subID := storage.SeedTestSubAccount(t, pool, resellerID)
	provA := storage.SeedTestProvider(t, pool, "A")
	provB := storage.SeedTestProvider(t, pool, "B")

	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	mat := NewProviderSetMaterializer(pool, setRepo, itemsRepo)

	set, _ := setRepo.Create(context.Background(), resellerID, "S", false)
	itemsRepo.ReplaceItems(context.Background(), set.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10, ExposeProviderName: true},
		{ProviderID: provB, Priority: 5, ExposeProviderName: true},
	})

	require.NoError(t, mat.ApplyToClient(context.Background(), subID, &set.ID))

	// Verify: 2 inherited rows
	var count int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_providers WHERE client_id = $1 AND ownership = 'inherited'`,
		subID).Scan(&count)
	require.Equal(t, 2, count)
}

func TestMaterializer_Apply_PreservesPrivateOverrides(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	subID := storage.SeedTestSubAccount(t, pool, resellerID)
	provA := storage.SeedTestProvider(t, pool, "A")
	provB := storage.SeedTestProvider(t, pool, "B")
	provC := storage.SeedTestProvider(t, pool, "C")

	// Manually insert a private override
	pool.Exec(context.Background(),
		`INSERT INTO client_providers (client_id, provider_id, ownership, shared_priority, active)
		 VALUES ($1, $2, 'private', 99, true)`, subID, provC)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	mat := NewProviderSetMaterializer(pool, setRepo, itemsRepo)
	set, _ := setRepo.Create(context.Background(), resellerID, "S", false)
	itemsRepo.ReplaceItems(context.Background(), set.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10}, {ProviderID: provB, Priority: 5},
	})

	require.NoError(t, mat.ApplyToClient(context.Background(), subID, &set.ID))

	var inheritedCount, privateCount int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_providers WHERE client_id = $1 AND ownership = 'inherited'`,
		subID).Scan(&inheritedCount)
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_providers WHERE client_id = $1 AND ownership = 'private'`,
		subID).Scan(&privateCount)
	require.Equal(t, 2, inheritedCount); require.Equal(t, 1, privateCount)
}

func TestMaterializer_Apply_NilSet_ClearsInherited(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	subID := storage.SeedTestSubAccount(t, pool, resellerID)
	provA := storage.SeedTestProvider(t, pool, "A")
	pool.Exec(context.Background(),
		`INSERT INTO client_providers (client_id, provider_id, ownership, shared_priority, active)
		 VALUES ($1, $2, 'inherited', 10, true)`, subID, provA)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	mat := NewProviderSetMaterializer(pool, setRepo, itemsRepo)

	require.NoError(t, mat.ApplyToClient(context.Background(), subID, nil))

	var count int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_providers WHERE client_id = $1 AND ownership = 'inherited'`,
		subID).Scan(&count)
	require.Equal(t, 0, count)
}
```

`SetupTestDBExposed` / `SeedTestReseller` / `SeedTestSubAccount` / `SeedTestProvider` — экспортированные версии test-helpers (вынести в `internal/storage/testfixtures.go` под `//go:build !production`-tag или просто в обычный файл, поскольку проект уже допускает test-helpers в production-коде). Если в проекте есть `internal/testutil/` — поместить туда.

Run: FAIL.

- [ ] **Step 5.2: Реализация**

```go
package network

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/storage"
)

type ProviderSetMaterializer struct {
	pool      *pgxpool.Pool
	setRepo   *storage.ResellerProviderSetRepository
	itemsRepo *storage.ResellerProviderSetItemsRepository
}

func NewProviderSetMaterializer(pool *pgxpool.Pool, setRepo *storage.ResellerProviderSetRepository, itemsRepo *storage.ResellerProviderSetItemsRepository) *ProviderSetMaterializer {
	return &ProviderSetMaterializer{pool: pool, setRepo: setRepo, itemsRepo: itemsRepo}
}

// ApplyToClient — материализует provider-set в client_providers под транзакцией.
// providerSetID == nil → стереть только inherited-записи.
// Записи ownership='private' (override) не трогаются.
func (m *ProviderSetMaterializer) ApplyToClient(ctx context.Context, clientID uuid.UUID, providerSetID *uuid.UUID) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)

	// 1. Стереть inherited
	if _, err := tx.Exec(ctx,
		`DELETE FROM client_providers WHERE client_id = $1 AND ownership = 'inherited'`,
		clientID); err != nil {
		return err
	}

	// 2. Если providerSetID != nil — вставить новые из items
	if providerSetID != nil {
		// source_client_id = reseller (parent_client_id суб-аккаунта)
		_, err := tx.Exec(ctx, `
			INSERT INTO client_providers
				(client_id, provider_id, ownership, source_client_id, shared_priority,
				 expose_cost, expose_provider_name, active)
			SELECT $1, items.provider_id, 'inherited', c.parent_client_id,
			       items.priority, items.expose_cost, items.expose_provider_name, true
			FROM reseller_provider_set_items items
			JOIN clients c ON c.id = $1
			WHERE items.set_id = $2
			ON CONFLICT (client_id, provider_id) DO UPDATE SET
				ownership = 'inherited',
				shared_priority = EXCLUDED.shared_priority,
				expose_cost = EXCLUDED.expose_cost,
				expose_provider_name = EXCLUDED.expose_provider_name,
				active = true,
				updated_at = now()
			WHERE client_providers.ownership = 'inherited'
		`, clientID, *providerSetID)
		if err != nil { return err }
	}

	return tx.Commit(ctx)
}

// ApplyToAllSubscribers — пересчитать материализацию для всех подписанных на set.
// Используется при изменении items в provider-set.
func (m *ProviderSetMaterializer) ApplyToAllSubscribers(ctx context.Context, providerSetID uuid.UUID) error {
	rows, err := m.pool.Query(ctx,
		`SELECT client_id FROM subaccount_routing_assignment WHERE provider_set_id = $1`,
		providerSetID)
	if err != nil { return err }
	defer rows.Close()
	var clientIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil { return err }
		clientIDs = append(clientIDs, id)
	}
	if err := rows.Err(); err != nil { return err }

	for _, cid := range clientIDs {
		if err := m.ApplyToClient(ctx, cid, &providerSetID); err != nil {
			return err
		}
	}
	return nil
}
```

**Заметка по ON CONFLICT WHERE:** UNIQUE constraint в `client_providers (client_id, provider_id)` — без partial. Когда у клиента уже есть `ownership='private'` для этого провайдера, INSERT с conflict поднимет race: попытка перезаписать private как inherited. Решение: WHERE `client_providers.ownership = 'inherited'` в DO UPDATE — обновляем только если уже было inherited. Если private — ON CONFLICT no-op (private остаётся, inherited не появляется). Это работает в Postgres 15+ (см. WHERE clause в DO UPDATE).

Альтернатива (если WHERE в DO UPDATE окажется проблемным): pre-SELECT'ить provider_id'и, у которых уже `ownership='private'`, исключить их из INSERT. Реализуем альтернативу при тестовом провале.

- [ ] **Step 5.3: Run tests**

Run: `go test ./internal/services/network/ -run TestMaterializer -v`
Expected: ALL PASS. Если падает на `TestMaterializer_Apply_PreservesPrivateOverrides` — переключиться на alt-стратегию (см. заметку).

- [ ] **Step 5.4: Commit**

```bash
git add internal/services/network/provider_set_materializer.go internal/services/network/provider_set_materializer_test.go
git commit -m "feat(network): ProviderSetMaterializer — материализация в client_providers"
```

---

## Phase D — Backend handlers

### Task 6: Handler `/reseller/network/providers` (каталог)

**Files:**
- Create: `internal/gateway/portal/handlers/network_providers.go`
- Create: `internal/gateway/portal/handlers/network_providers_test.go`

- [ ] **Step 6.1: Failing test для GET (пустой список + микс platform/private)**

```go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func TestNetworkProviders_List_ReturnsPlatformAndPrivate(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	platformProv := storage.SeedTestProvider(t, pool, "Platform A") // ownership=platform
	privateProv := storage.SeedTestProviderPrivate(t, pool, "Private B", resellerID) // ownership=private, source_client_id=resellerID

	h := NewNetworkProvidersHandlers(pool)
	req := httptest.NewRequest("GET", "/portal/v1/reseller/network/providers", nil)
	req = req.WithContext(middleware.WithClientID(req.Context(), uuid.MustParse(resellerID.String())))
	w := httptest.NewRecorder()

	h.List(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct{ Providers []map[string]interface{} `json:"providers"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	require.Len(t, resp.Providers, 2)
}
```

Если `SeedTestProviderPrivate` не существует — добавить в test-fixtures.

- [ ] **Step 6.2: Реализация GET (List)**

```go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type NetworkProvidersHandlers struct {
	pool *pgxpool.Pool
}

func NewNetworkProvidersHandlers(pool *pgxpool.Pool) *NetworkProvidersHandlers {
	return &NetworkProvidersHandlers{pool: pool}
}

func (h *NetworkProvidersHandlers) reseller(r *http.Request) (uuid.UUID, error) {
	cid, ok := middleware.GetClientID(r.Context())
	if !ok { return uuid.Nil, shared.ErrUnauthorized("Клиент не найден") }
	return cid, nil
}

type providerOut struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Ownership  string  `json:"ownership"`
	SMPPHost   *string `json:"smpp_host"`
	SMPPPort   *int    `json:"smpp_port"`
	SystemID   *string `json:"system_id"`
	SystemType *string `json:"system_type"`
	Active     bool    `json:"active"`
}

func (h *NetworkProvidersHandlers) List(w http.ResponseWriter, r *http.Request) {
	resellerID, err := h.reseller(r)
	if err != nil { respondError(w, err); return }

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, name,
		   COALESCE(ownership, 'platform') AS ownership,
		   smpp_host, smpp_port, system_id, system_type, active
		 FROM providers
		 WHERE COALESCE(ownership, 'platform') = 'platform'
		    OR (ownership = 'private' AND source_client_id = $1)
		 ORDER BY ownership, name`, resellerID)
	if err != nil {
		log.Error().Err(err).Msg("network providers list")
		respondError(w, shared.ErrInternalServer("ошибка получения провайдеров"))
		return
	}
	defer rows.Close()
	out := []providerOut{}
	for rows.Next() {
		var p providerOut
		if err := rows.Scan(&p.ID, &p.Name, &p.Ownership, &p.SMPPHost, &p.SMPPPort, &p.SystemID, &p.SystemType, &p.Active); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		out = append(out, p)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"providers": out})
}
```

**Важно:** в текущей таблице `providers` НЕ ВСЕ записи имеют `ownership` (legacy могут быть NULL). `COALESCE(ownership, 'platform')` обеспечивает совместимость. **Перед началом — проверить grep'ом** в schema, что колонка `ownership` существует в `providers`. Если нет — пропустить добавление и доработать в Plan 2 (или добавить миграцию `000132_providers_ownership.up.sql` ALTER TABLE с DEFAULT 'platform').

Run: `go test ./internal/gateway/portal/handlers/ -run TestNetworkProviders_List -v`
Expected: PASS.

- [ ] **Step 6.3: POST/PUT/DELETE — failing tests + реализация**

Тесты:
```go
func TestNetworkProviders_Create_PrivateOnly(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	h := NewNetworkProvidersHandlers(pool)

	body := `{"name":"My Provider","smpp_host":"smpp.example.com","smpp_port":2775,"system_id":"sys","password":"pwd","system_type":"SMPP"}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/providers", strings.NewReader(body))
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Create(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp providerOut; json.Unmarshal(w.Body.Bytes(), &resp)
	require.Equal(t, "private", resp.Ownership)
}

func TestNetworkProviders_Update_RejectPlatform(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	platformID := storage.SeedTestProvider(t, pool, "Plat") // ownership=platform
	h := NewNetworkProvidersHandlers(pool)

	body := `{"name":"hacked"}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/providers/"+platformID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": platformID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Update(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestNetworkProviders_Delete_409IfUsedInSet(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	priv := storage.SeedTestProviderPrivate(t, pool, "P", resellerID)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	set, _ := setRepo.Create(context.Background(), resellerID, "S", false)
	itemsRepo.ReplaceItems(context.Background(), set.ID, []storage.ProviderSetItemInput{
		{ProviderID: priv, Priority: 10},
	})

	h := NewNetworkProvidersHandlers(pool)
	req := httptest.NewRequest("DELETE", "/portal/v1/reseller/network/providers/"+priv.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": priv.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
}
```

Реализация — добавить в `network_providers.go`:
```go
type providerCreateReq struct {
	Name       string `json:"name"`
	SMPPHost   string `json:"smpp_host"`
	SMPPPort   int    `json:"smpp_port"`
	SystemID   string `json:"system_id"`
	Password   string `json:"password"`
	SystemType string `json:"system_type"`
}

func (h *NetworkProvidersHandlers) Create(w http.ResponseWriter, r *http.Request) {
	resellerID, err := h.reseller(r)
	if err != nil { respondError(w, err); return }
	var req providerCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON")); return
	}
	if req.Name == "" || req.SMPPHost == "" || req.SystemID == "" {
		respondError(w, shared.ErrInvalidInput("name/smpp_host/system_id обязательны")); return
	}
	var id uuid.UUID
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO providers (name, ownership, source_client_id, smpp_host, smpp_port, system_id, password, system_type, active)
		 VALUES ($1, 'private', $2, $3, $4, $5, $6, $7, true) RETURNING id`,
		req.Name, resellerID, req.SMPPHost, req.SMPPPort, req.SystemID, req.Password, req.SystemType,
	).Scan(&id)
	if err != nil {
		log.Error().Err(err).Msg("create private provider")
		respondError(w, shared.ErrInternalServer("ошибка создания провайдера")); return
	}
	respondJSON(w, http.StatusCreated, providerOut{
		ID: id.String(), Name: req.Name, Ownership: "private",
		SMPPHost: &req.SMPPHost, SMPPPort: &req.SMPPPort, SystemID: &req.SystemID,
		SystemType: &req.SystemType, Active: true,
	})
}

func (h *NetworkProvidersHandlers) Update(w http.ResponseWriter, r *http.Request) {
	resellerID, err := h.reseller(r)
	if err != nil { respondError(w, err); return }
	id, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id invalid")); return }

	// Verify ownership
	var ownership string; var sourceClientID *uuid.UUID
	err = h.pool.QueryRow(r.Context(),
		`SELECT COALESCE(ownership,'platform'), source_client_id FROM providers WHERE id=$1`, id,
	).Scan(&ownership, &sourceClientID)
	if err != nil { respondError(w, shared.ErrNotFound("провайдер")); return }
	if ownership == "platform" {
		respondError(w, shared.ErrForbidden("нельзя редактировать платформенный провайдер")); return
	}
	if sourceClientID == nil || *sourceClientID != resellerID {
		respondError(w, shared.ErrNotFound("провайдер")); return
	}
	var req providerCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid JSON")); return
	}
	_, err = h.pool.Exec(r.Context(),
		`UPDATE providers SET name=$1, smpp_host=$2, smpp_port=$3, system_id=$4, password=COALESCE(NULLIF($5,''), password), system_type=$6 WHERE id=$7`,
		req.Name, req.SMPPHost, req.SMPPPort, req.SystemID, req.Password, req.SystemType, id)
	if err != nil { respondError(w, shared.ErrInternalServer("update")); return }
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": id.String()})
}

func (h *NetworkProvidersHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	resellerID, err := h.reseller(r)
	if err != nil { respondError(w, err); return }
	id, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id invalid")); return }

	var ownership string; var sourceClientID *uuid.UUID
	err = h.pool.QueryRow(r.Context(),
		`SELECT COALESCE(ownership,'platform'), source_client_id FROM providers WHERE id=$1`, id,
	).Scan(&ownership, &sourceClientID)
	if err != nil { respondError(w, shared.ErrNotFound("провайдер")); return }
	if ownership != "private" || sourceClientID == nil || *sourceClientID != resellerID {
		respondError(w, shared.ErrForbidden("нельзя удалить")); return
	}

	// Check usage in this reseller's provider-sets
	var usedInSets int
	h.pool.QueryRow(r.Context(),
		`SELECT count(*) FROM reseller_provider_set_items i
		   JOIN reseller_provider_sets s ON s.id = i.set_id
		 WHERE s.reseller_id = $1 AND i.provider_id = $2`,
		resellerID, id).Scan(&usedInSets)
	if usedInSets > 0 {
		respondError(w, shared.ErrConflict(fmt.Sprintf("используется в %d provider-set", usedInSets)).
			WithDetails(`{"kind":"used_in_provider_sets"}`))
		return
	}

	// Check usage in client_providers (override)
	var usedInClients int
	h.pool.QueryRow(r.Context(),
		`SELECT count(*) FROM client_providers cp
		   JOIN clients c ON c.id = cp.client_id
		 WHERE c.parent_client_id = $1 AND cp.provider_id = $2 AND cp.ownership = 'private'`,
		resellerID, id).Scan(&usedInClients)
	if usedInClients > 0 {
		respondError(w, shared.ErrConflict(fmt.Sprintf("используется как override у %d суб-аккаунта(ов)", usedInClients)).
			WithDetails(`{"kind":"used_in_overrides"}`))
		return
	}

	if _, err := h.pool.Exec(r.Context(), `DELETE FROM providers WHERE id=$1`, id); err != nil {
		respondError(w, shared.ErrInternalServer("delete")); return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Run: `go test ./internal/gateway/portal/handlers/ -run TestNetworkProviders -v`
Expected: ALL PASS.

- [ ] **Step 6.4: Commit**

```bash
git add internal/gateway/portal/handlers/network_providers.go internal/gateway/portal/handlers/network_providers_test.go
git commit -m "feat(handlers): /reseller/network/providers — каталог + CRUD private"
```

---

### Task 7: Handler `/reseller/network/provider-sets`

**Files:**
- Create: `internal/gateway/portal/handlers/network_provider_sets.go`
- Create: `internal/gateway/portal/handlers/network_provider_sets_test.go`

- [ ] **Step 7.1: Failing tests**

```go
func TestProviderSets_CRUD(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	h := NewNetworkProviderSetsHandlers(pool, /* materializer */)

	// Create
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/provider-sets", strings.NewReader(`{"name":"S1","is_default":false}`))
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Create(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	var created struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &created)

	// List
	w = httptest.NewRecorder()
	h.List(w, httptest.NewRequest("GET", "/portal/v1/reseller/network/provider-sets", nil).WithContext(middleware.WithClientID(context.Background(), resellerID)))
	require.Equal(t, http.StatusOK, w.Code)

	// Update
	w = httptest.NewRecorder()
	upd := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+created.ID, strings.NewReader(`{"name":"S2","is_default":true}`))
	upd = mux.SetURLVars(upd, map[string]string{"id": created.ID})
	upd = upd.WithContext(middleware.WithClientID(upd.Context(), resellerID))
	h.Update(w, upd)
	require.Equal(t, http.StatusOK, w.Code)

	// Delete
	w = httptest.NewRecorder()
	del := httptest.NewRequest("DELETE", "/portal/v1/reseller/network/provider-sets/"+created.ID, nil)
	del = mux.SetURLVars(del, map[string]string{"id": created.ID})
	del = del.WithContext(middleware.WithClientID(del.Context(), resellerID))
	h.Delete(w, del)
	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestProviderSets_Delete_409IfAssigned(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	subID := storage.SeedTestSubAccount(t, pool, resellerID)
	setRepo := storage.NewResellerProviderSetRepository(pool)
	sraRepo := storage.NewSubAccountRoutingAssignmentRepository(pool)
	set, _ := setRepo.Create(context.Background(), resellerID, "S", false)
	sraRepo.Upsert(context.Background(), subID, &set.ID, nil)

	h := NewNetworkProviderSetsHandlers(pool, nil)
	req := httptest.NewRequest("DELETE", "/portal/v1/reseller/network/provider-sets/"+set.ID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
}

func TestProviderSets_OwnershipCheck_404(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerA := storage.SeedTestReseller(t, pool)
	resellerB := storage.SeedTestReseller(t, pool)
	setRepo := storage.NewResellerProviderSetRepository(pool)
	setA, _ := setRepo.Create(context.Background(), resellerA, "A", false)

	h := NewNetworkProviderSetsHandlers(pool, nil)
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+setA.ID.String(), strings.NewReader(`{"name":"hijack"}`))
	req = mux.SetURLVars(req, map[string]string{"id": setA.ID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerB)) // wrong reseller
	w := httptest.NewRecorder()
	h.Update(w, req)
	require.Equal(t, http.StatusNotFound, w.Code) // не 403, чтобы не утекало существование
}
```

- [ ] **Step 7.2: Реализация**

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type NetworkProviderSetsHandlers struct {
	pool         *pgxpool.Pool
	setRepo      *storage.ResellerProviderSetRepository
	sraRepo      *storage.SubAccountRoutingAssignmentRepository
	materializer *network.ProviderSetMaterializer
}

func NewNetworkProviderSetsHandlers(pool *pgxpool.Pool, mat *network.ProviderSetMaterializer) *NetworkProviderSetsHandlers {
	return &NetworkProviderSetsHandlers{
		pool:         pool,
		setRepo:      storage.NewResellerProviderSetRepository(pool),
		sraRepo:      storage.NewSubAccountRoutingAssignmentRepository(pool),
		materializer: mat,
	}
}

func (h *NetworkProviderSetsHandlers) reseller(r *http.Request) (uuid.UUID, error) {
	cid, ok := middleware.GetClientID(r.Context())
	if !ok { return uuid.Nil, shared.ErrUnauthorized("не авторизован") }
	return cid, nil
}

func (h *NetworkProviderSetsHandlers) verifyOwnership(ctx context.Context, resellerID, setID uuid.UUID) error {
	set, err := h.setRepo.GetByID(ctx, setID)
	if err != nil { return shared.ErrNotFound("provider-set") }
	if set.ResellerID != resellerID { return shared.ErrNotFound("provider-set") }
	return nil
}

type providerSetOut struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsDefault     bool   `json:"is_default"`
	ItemCount     int    `json:"item_count"`
	AssignedCount int    `json:"assigned_count"`
}

func (h *NetworkProviderSetsHandlers) List(w http.ResponseWriter, r *http.Request) {
	resellerID, err := h.reseller(r)
	if err != nil { respondError(w, err); return }
	rows, err := h.pool.Query(r.Context(), `
		SELECT s.id, s.name, s.is_default,
		   (SELECT count(*) FROM reseller_provider_set_items WHERE set_id = s.id) AS item_count,
		   (SELECT count(*) FROM subaccount_routing_assignment WHERE provider_set_id = s.id) AS assigned_count
		FROM reseller_provider_sets s WHERE s.reseller_id = $1 ORDER BY s.created_at DESC`, resellerID)
	if err != nil { respondError(w, shared.ErrInternalServer("list sets")); return }
	defer rows.Close()
	out := []providerSetOut{}
	for rows.Next() {
		var p providerSetOut
		if err := rows.Scan(&p.ID, &p.Name, &p.IsDefault, &p.ItemCount, &p.AssignedCount); err != nil {
			respondError(w, shared.ErrInternalServer("scan")); return
		}
		out = append(out, p)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"sets": out})
}

func (h *NetworkProviderSetsHandlers) Create(w http.ResponseWriter, r *http.Request) {
	resellerID, err := h.reseller(r)
	if err != nil { respondError(w, err); return }
	var body struct { Name string `json:"name"`; IsDefault bool `json:"is_default"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	if body.Name == "" { respondError(w, shared.ErrInvalidInput("name обязательно")); return }

	tx, err := h.pool.Begin(r.Context())
	if err != nil { respondError(w, shared.ErrInternalServer("tx")); return }
	defer tx.Rollback(r.Context())
	if body.IsDefault {
		// Снять флаг с предыдущего default
		if _, err := tx.Exec(r.Context(),
			`UPDATE reseller_provider_sets SET is_default=false WHERE reseller_id=$1 AND is_default=true`,
			resellerID); err != nil {
			respondError(w, shared.ErrInternalServer("clear default")); return
		}
	}
	var id uuid.UUID
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO reseller_provider_sets (reseller_id, name, is_default) VALUES ($1, $2, $3) RETURNING id`,
		resellerID, body.Name, body.IsDefault,
	).Scan(&id); err != nil {
		respondError(w, shared.ErrInternalServer("insert")); return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("commit")); return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id.String(), "name": body.Name, "is_default": body.IsDefault})
}

func (h *NetworkProviderSetsHandlers) Update(w http.ResponseWriter, r *http.Request) {
	resellerID, err := h.reseller(r)
	if err != nil { respondError(w, err); return }
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verifyOwnership(r.Context(), resellerID, setID); err != nil {
		respondError(w, err); return
	}
	var body struct { Name string `json:"name"`; IsDefault bool `json:"is_default"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil { respondError(w, shared.ErrInternalServer("tx")); return }
	defer tx.Rollback(r.Context())
	if body.IsDefault {
		tx.Exec(r.Context(), `UPDATE reseller_provider_sets SET is_default=false WHERE reseller_id=$1 AND is_default=true AND id<>$2`, resellerID, setID)
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE reseller_provider_sets SET name=$1, is_default=$2, updated_at=now() WHERE id=$3`,
		body.Name, body.IsDefault, setID); err != nil {
		respondError(w, shared.ErrInternalServer("update")); return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("commit")); return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": setID.String()})
}

func (h *NetworkProviderSetsHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	resellerID, err := h.reseller(r)
	if err != nil { respondError(w, err); return }
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verifyOwnership(r.Context(), resellerID, setID); err != nil {
		respondError(w, err); return
	}
	var assigned int
	h.pool.QueryRow(r.Context(),
		`SELECT count(*) FROM subaccount_routing_assignment WHERE provider_set_id = $1`, setID,
	).Scan(&assigned)
	if assigned > 0 {
		respondError(w, shared.ErrConflict("provider-set назначен суб-аккаунтам").
			WithDetails(`{"kind":"set_assigned","count":` + fmt.Sprint(assigned) + `}`))
		return
	}
	if _, err := h.pool.Exec(r.Context(),
		`DELETE FROM reseller_provider_sets WHERE id = $1`, setID); err != nil {
		respondError(w, shared.ErrInternalServer("delete")); return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Run: `go test ./internal/gateway/portal/handlers/ -run TestProviderSets -v`
Expected: ALL PASS.

- [ ] **Step 7.3: Commit**

```bash
git add internal/gateway/portal/handlers/network_provider_sets.go internal/gateway/portal/handlers/network_provider_sets_test.go
git commit -m "feat(handlers): /reseller/network/provider-sets — CRUD + ownership check"
```

---

### Task 8: Handler items provider-set'а (PUT-replace)

**Files:**
- Create: `internal/gateway/portal/handlers/network_provider_set_items.go`
- Create: `internal/gateway/portal/handlers/network_provider_set_items_test.go`

- [ ] **Step 8.1: Failing tests**

Тесты на:
1. GET items — пустой массив для нового set'а
2. PUT items — добавляет; повторный PUT — атомарно заменяет
3. PUT items с дубликатом provider_id → 400
4. PUT items с provider_id, который не принадлежит reseller'у (чужой private) → 403
5. После PUT — материализация: у subscriber'ов появились inherited rows

Код тестов аналогичен предыдущим (см. паттерн), не повторяю целиком — engineer видит всё в файле. Конкретный кейс материализации:

```go
func TestProviderSetItems_PUT_TriggersMaterialization(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	subID := storage.SeedTestSubAccount(t, pool, resellerID)
	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	sraRepo := storage.NewSubAccountRoutingAssignmentRepository(pool)
	set, _ := setRepo.Create(context.Background(), resellerID, "S", false)
	sraRepo.Upsert(context.Background(), subID, &set.ID, nil)
	provA := storage.SeedTestProvider(t, pool, "A")

	mat := network.NewProviderSetMaterializer(pool, setRepo, itemsRepo)
	h := NewNetworkProviderSetItemsHandlers(pool, mat)

	body := `{"items":[{"provider_id":"` + provA.String() + `","priority":10,"expose_provider_name":true}]}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+set.ID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.PutItems(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var count int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_providers WHERE client_id=$1 AND ownership='inherited'`,
		subID).Scan(&count)
	require.Equal(t, 1, count)
}
```

- [ ] **Step 8.2: Реализация**

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type NetworkProviderSetItemsHandlers struct {
	pool         *pgxpool.Pool
	setRepo      *storage.ResellerProviderSetRepository
	itemsRepo    *storage.ResellerProviderSetItemsRepository
	materializer *network.ProviderSetMaterializer
}

func NewNetworkProviderSetItemsHandlers(pool *pgxpool.Pool, mat *network.ProviderSetMaterializer) *NetworkProviderSetItemsHandlers {
	return &NetworkProviderSetItemsHandlers{
		pool: pool,
		setRepo:   storage.NewResellerProviderSetRepository(pool),
		itemsRepo: storage.NewResellerProviderSetItemsRepository(pool),
		materializer: mat,
	}
}

type itemOut struct {
	ID                  string `json:"id"`
	ProviderID          string `json:"provider_id"`
	ProviderName        string `json:"provider_name"`
	Priority            int    `json:"priority"`
	ExposeCost          bool   `json:"expose_cost"`
	ExposeProviderName  bool   `json:"expose_provider_name"`
}

func (h *NetworkProviderSetItemsHandlers) ListItems(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	setID, _ := uuid.Parse(mux.Vars(r)["id"])
	set, err := h.setRepo.GetByID(r.Context(), setID)
	if err != nil || set.ResellerID != resellerID {
		respondError(w, shared.ErrNotFound("provider-set")); return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT i.id, i.provider_id, p.name, i.priority, i.expose_cost, i.expose_provider_name
		FROM reseller_provider_set_items i
		JOIN providers p ON p.id = i.provider_id
		WHERE i.set_id = $1 ORDER BY i.priority DESC, p.name`, setID)
	if err != nil { respondError(w, shared.ErrInternalServer("query")); return }
	defer rows.Close()
	out := []itemOut{}
	for rows.Next() {
		var i itemOut
		if err := rows.Scan(&i.ID, &i.ProviderID, &i.ProviderName, &i.Priority, &i.ExposeCost, &i.ExposeProviderName); err != nil {
			respondError(w, shared.ErrInternalServer("scan")); return
		}
		out = append(out, i)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"items": out})
}

type putItemsReq struct {
	Items []struct {
		ProviderID         string `json:"provider_id"`
		Priority           int    `json:"priority"`
		ExposeCost         bool   `json:"expose_cost"`
		ExposeProviderName bool   `json:"expose_provider_name"`
	} `json:"items"`
}

func (h *NetworkProviderSetItemsHandlers) PutItems(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	set, err := h.setRepo.GetByID(r.Context(), setID)
	if err != nil || set.ResellerID != resellerID {
		respondError(w, shared.ErrNotFound("provider-set")); return
	}
	var req putItemsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	// Дубликаты provider_id?
	seen := map[string]bool{}
	for _, it := range req.Items {
		if seen[it.ProviderID] { respondError(w, shared.ErrInvalidInput("дубликат provider_id")); return }
		seen[it.ProviderID] = true
	}
	// Каждый provider_id принадлежит reseller'у (platform либо его private)
	for _, it := range req.Items {
		var ownership string; var sourceClientID *uuid.UUID
		err := h.pool.QueryRow(r.Context(),
			`SELECT COALESCE(ownership,'platform'), source_client_id FROM providers WHERE id=$1`,
			it.ProviderID).Scan(&ownership, &sourceClientID)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("неизвестный provider_id "+it.ProviderID)); return
		}
		if ownership == "private" && (sourceClientID == nil || *sourceClientID != resellerID) {
			respondError(w, shared.ErrForbidden("чужой провайдер")); return
		}
	}

	// Build typed input
	input := make([]storage.ProviderSetItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		pid := uuid.MustParse(it.ProviderID)
		input = append(input, storage.ProviderSetItemInput{
			ProviderID: pid, Priority: it.Priority,
			ExposeCost: it.ExposeCost, ExposeProviderName: it.ExposeProviderName,
		})
	}
	if err := h.itemsRepo.ReplaceItems(r.Context(), setID, input); err != nil {
		respondError(w, shared.ErrInternalServer("replace")); return
	}

	// Re-materialize for all subscribers
	if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
		respondError(w, shared.ErrInternalServer("rematerialize")); return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"set_id": setID.String(), "count": len(input)})
}
```

Run: `go test ./internal/gateway/portal/handlers/ -run TestProviderSetItems -v`
Expected: PASS.

- [ ] **Step 8.3: Commit**

```bash
git add internal/gateway/portal/handlers/network_provider_set_items.go internal/gateway/portal/handlers/network_provider_set_items_test.go
git commit -m "feat(handlers): /reseller/network/provider-sets/{id}/items — atomic PUT-replace + materialize"
```

---

### Task 9: Handler `/reseller/network/assignments` (provider-set часть)

**Files:**
- Create: `internal/gateway/portal/handlers/network_assignments.go`
- Create: `internal/gateway/portal/handlers/network_assignments_test.go`

- [ ] **Step 9.1: Failing tests** (List, PUT one, BulkAssign — все только с provider-set, route-set игнорируем в Plan 1)

```go
func TestAssignments_List(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	sub1 := storage.SeedTestSubAccount(t, pool, resellerID)
	sub2 := storage.SeedTestSubAccount(t, pool, resellerID)
	setRepo := storage.NewResellerProviderSetRepository(pool)
	sraRepo := storage.NewSubAccountRoutingAssignmentRepository(pool)
	set, _ := setRepo.Create(context.Background(), resellerID, "S", false)
	sraRepo.Upsert(context.Background(), sub1, &set.ID, nil)

	h := NewNetworkAssignmentsHandlers(pool, nil) // mat не нужен для List
	req := httptest.NewRequest("GET", "/portal/v1/reseller/network/assignments", nil)
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.List(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct{ Assignments []map[string]interface{} `json:"assignments"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	require.Len(t, resp.Assignments, 2) // sub2 без назначения тоже в списке
}

func TestAssignments_PutOne_Materializes(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	subID := storage.SeedTestSubAccount(t, pool, resellerID)
	provA := storage.SeedTestProvider(t, pool, "A")
	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	mat := network.NewProviderSetMaterializer(pool, setRepo, itemsRepo)
	set, _ := setRepo.Create(context.Background(), resellerID, "S", false)
	itemsRepo.ReplaceItems(context.Background(), set.ID, []storage.ProviderSetItemInput{{ProviderID: provA, Priority: 10}})

	h := NewNetworkAssignmentsHandlers(pool, mat)
	body := `{"provider_set_id":"` + set.ID.String() + `","route_set_id":null}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/assignments/"+subID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"client_id": subID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.PutOne(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var count int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_providers WHERE client_id=$1 AND ownership='inherited'`, subID).Scan(&count)
	require.Equal(t, 1, count)
}

func TestAssignments_Bulk_PartialSuccess(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	sub1 := storage.SeedTestSubAccount(t, pool, resellerID)
	sub2 := storage.SeedTestSubAccount(t, pool, resellerID)
	otherReseller := storage.SeedTestReseller(t, pool)
	subOther := storage.SeedTestSubAccount(t, pool, otherReseller)
	provA := storage.SeedTestProvider(t, pool, "A")
	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	mat := network.NewProviderSetMaterializer(pool, setRepo, itemsRepo)
	set, _ := setRepo.Create(context.Background(), resellerID, "S", false)
	itemsRepo.ReplaceItems(context.Background(), set.ID, []storage.ProviderSetItemInput{{ProviderID: provA, Priority: 10}})

	h := NewNetworkAssignmentsHandlers(pool, mat)
	body := `{"client_ids":["` + sub1.String() + `","` + sub2.String() + `","` + subOther.String() + `"],"provider_set_id":"` + set.ID.String() + `","route_set_id":null}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/assignments/bulk", strings.NewReader(body))
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Bulk(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct{ Results []struct{ Status string `json:"status"` } `json:"results"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	require.Len(t, resp.Results, 3)
	// Two ok, one error (not your sub-account)
	okCount := 0; errCount := 0
	for _, r := range resp.Results {
		if r.Status == "ok" { okCount++ } else { errCount++ }
	}
	require.Equal(t, 2, okCount); require.Equal(t, 1, errCount)
}
```

- [ ] **Step 9.2: Реализация**

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type NetworkAssignmentsHandlers struct {
	pool         *pgxpool.Pool
	materializer *network.ProviderSetMaterializer
}

func NewNetworkAssignmentsHandlers(pool *pgxpool.Pool, mat *network.ProviderSetMaterializer) *NetworkAssignmentsHandlers {
	return &NetworkAssignmentsHandlers{pool: pool, materializer: mat}
}

type assignmentOut struct {
	ClientID         string  `json:"client_id"`
	SubAccountName   string  `json:"sub_account_name"`
	ProviderSetID    *string `json:"provider_set_id"`
	ProviderSetName  *string `json:"provider_set_name"`
	RouteSetID       *string `json:"route_set_id"`
	RouteSetName     *string `json:"route_set_name"`
	HasOverrides     bool    `json:"has_overrides"`
	ValidationStatus string  `json:"validation_status"` // 'ok' | 'unassigned' (Plan 1: 'conflict' появится в Plan 2)
}

func (h *NetworkAssignmentsHandlers) List(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }

	rows, err := h.pool.Query(r.Context(), `
		SELECT c.id, COALESCE(c.name, c.email) AS name,
		       sra.provider_set_id, ps.name AS provider_set_name,
		       sra.route_set_id, NULL::text AS route_set_name,
		       EXISTS(SELECT 1 FROM client_providers cp WHERE cp.client_id = c.id AND cp.ownership='private') AS has_overrides
		FROM clients c
		LEFT JOIN subaccount_routing_assignment sra ON sra.client_id = c.id
		LEFT JOIN reseller_provider_sets ps ON ps.id = sra.provider_set_id
		WHERE c.parent_client_id = $1
		ORDER BY name`, resellerID)
	if err != nil { respondError(w, shared.ErrInternalServer("query")); return }
	defer rows.Close()
	out := []assignmentOut{}
	for rows.Next() {
		var a assignmentOut
		if err := rows.Scan(&a.ClientID, &a.SubAccountName, &a.ProviderSetID, &a.ProviderSetName, &a.RouteSetID, &a.RouteSetName, &a.HasOverrides); err != nil {
			respondError(w, shared.ErrInternalServer("scan")); return
		}
		if a.ProviderSetID == nil { a.ValidationStatus = "unassigned" } else { a.ValidationStatus = "ok" }
		out = append(out, a)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"assignments": out})
}

type putAssignmentReq struct {
	ProviderSetID *string `json:"provider_set_id"`
	RouteSetID    *string `json:"route_set_id"`
}

func (h *NetworkAssignmentsHandlers) verifySubAccountOwnership(ctx context.Context, resellerID, clientID uuid.UUID) error {
	var parent uuid.UUID
	err := h.pool.QueryRow(ctx,
		`SELECT parent_client_id FROM clients WHERE id=$1 AND parent_client_id IS NOT NULL`, clientID,
	).Scan(&parent)
	if err != nil || parent != resellerID { return shared.ErrNotFound("суб-аккаунт") }
	return nil
}

func (h *NetworkAssignmentsHandlers) verifyProviderSetOwnership(ctx context.Context, resellerID, setID uuid.UUID) error {
	var owner uuid.UUID
	err := h.pool.QueryRow(ctx,
		`SELECT reseller_id FROM reseller_provider_sets WHERE id=$1`, setID,
	).Scan(&owner)
	if err != nil || owner != resellerID { return shared.ErrNotFound("provider-set") }
	return nil
}

func (h *NetworkAssignmentsHandlers) PutOne(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	clientID, perr := uuid.Parse(mux.Vars(r)["client_id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("client_id")); return }
	if err := h.verifySubAccountOwnership(r.Context(), resellerID, clientID); err != nil {
		respondError(w, err); return
	}
	var req putAssignmentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	var providerSetUUID *uuid.UUID
	if req.ProviderSetID != nil && *req.ProviderSetID != "" {
		psid, err := uuid.Parse(*req.ProviderSetID)
		if err != nil { respondError(w, shared.ErrInvalidInput("provider_set_id")); return }
		if err := h.verifyProviderSetOwnership(r.Context(), resellerID, psid); err != nil {
			respondError(w, err); return
		}
		providerSetUUID = &psid
	}
	// route_set_id игнорируется в Plan 1 — будет в Plan 2

	// UPSERT assignment
	if _, err := h.pool.Exec(r.Context(), `
		INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
		VALUES ($1, $2, NULL, now())
		ON CONFLICT (client_id) DO UPDATE SET
		  provider_set_id = EXCLUDED.provider_set_id,
		  assigned_at = now()`,
		clientID, providerSetUUID); err != nil {
		respondError(w, shared.ErrInternalServer("upsert")); return
	}
	if err := h.materializer.ApplyToClient(r.Context(), clientID, providerSetUUID); err != nil {
		respondError(w, shared.ErrInternalServer("materialize")); return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"client_id": clientID.String()})
}

type bulkReq struct {
	ClientIDs     []string `json:"client_ids"`
	ProviderSetID *string  `json:"provider_set_id"`
	RouteSetID    *string  `json:"route_set_id"`
}

type bulkResultItem struct {
	ClientID string `json:"client_id"`
	Status   string `json:"status"` // ok | error
	Error    string `json:"error,omitempty"`
}

func (h *NetworkAssignmentsHandlers) Bulk(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	var req bulkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	var providerSetUUID *uuid.UUID
	if req.ProviderSetID != nil && *req.ProviderSetID != "" {
		psid, err := uuid.Parse(*req.ProviderSetID)
		if err != nil { respondError(w, shared.ErrInvalidInput("provider_set_id")); return }
		if err := h.verifyProviderSetOwnership(r.Context(), resellerID, psid); err != nil {
			respondError(w, err); return
		}
		providerSetUUID = &psid
	}

	results := make([]bulkResultItem, 0, len(req.ClientIDs))
	for _, idStr := range req.ClientIDs {
		cid, err := uuid.Parse(idStr)
		if err != nil { results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "invalid id"}); continue }
		if err := h.verifySubAccountOwnership(r.Context(), resellerID, cid); err != nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "not your sub-account"}); continue
		}
		// Upsert + materialize per client (each its own transaction)
		if _, err := h.pool.Exec(r.Context(), `
			INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
			VALUES ($1, $2, NULL, now())
			ON CONFLICT (client_id) DO UPDATE SET
			  provider_set_id = EXCLUDED.provider_set_id, assigned_at = now()`,
			cid, providerSetUUID); err != nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: err.Error()}); continue
		}
		if err := h.materializer.ApplyToClient(r.Context(), cid, providerSetUUID); err != nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: err.Error()}); continue
		}
		results = append(results, bulkResultItem{ClientID: idStr, Status: "ok"})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}
```

Run: `go test ./internal/gateway/portal/handlers/ -run TestAssignments -v`
Expected: ALL PASS.

- [ ] **Step 9.3: Commit**

```bash
git add internal/gateway/portal/handlers/network_assignments.go internal/gateway/portal/handlers/network_assignments_test.go
git commit -m "feat(handlers): /reseller/network/assignments — list + PUT + bulk (provider-set only)"
```

---

### Task 10: Override-handlers (`/sub-accounts/{id}/network/provider-overrides`)

**Files:**
- Create: `internal/gateway/portal/handlers/subaccount_network_overrides.go`
- Create: `internal/gateway/portal/handlers/subaccount_network_overrides_test.go`

- [ ] **Step 10.1: Failing tests**

```go
func TestProviderOverride_Add_409IfInherited(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	subID := storage.SeedTestSubAccount(t, pool, resellerID)
	provA := storage.SeedTestProvider(t, pool, "A")
	// emulate inherited
	pool.Exec(context.Background(),
		`INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		 VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := `{"provider_id":"` + provA.String() + `","priority":50,"expose_provider_name":true}`
	req := httptest.NewRequest("POST", "/portal/v1/sub-accounts/"+subID.String()+"/network/provider-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.AddProviderOverride(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
}

func TestProviderOverride_Add_OK(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t); defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	subID := storage.SeedTestSubAccount(t, pool, resellerID)
	provB := storage.SeedTestProvider(t, pool, "B")
	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := `{"provider_id":"` + provB.String() + `","priority":50,"expose_provider_name":true}`
	req := httptest.NewRequest("POST", "/portal/v1/sub-accounts/"+subID.String()+"/network/provider-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.AddProviderOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
}

func TestProviderOverview(t *testing.T) {
	// Тест GET /sub-accounts/{id}/network/overview — что возвращает provider_set + overrides
	// (только провайдеры в Plan 1; route-секция возвращает пусто/null)
	// ... (паттерн как выше)
}
```

- [ ] **Step 10.2: Реализация**

```go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type SubAccountNetworkOverridesHandlers struct { pool *pgxpool.Pool }

func NewSubAccountNetworkOverridesHandlers(pool *pgxpool.Pool) *SubAccountNetworkOverridesHandlers {
	return &SubAccountNetworkOverridesHandlers{pool: pool}
}

func (h *SubAccountNetworkOverridesHandlers) verifyOwnership(ctx context.Context, resellerID, subID uuid.UUID) error {
	var parent uuid.UUID
	err := h.pool.QueryRow(ctx, `SELECT parent_client_id FROM clients WHERE id=$1 AND parent_client_id IS NOT NULL`, subID).Scan(&parent)
	if err != nil || parent != resellerID { return shared.ErrNotFound("суб-аккаунт") }
	return nil
}

type addProviderOverrideReq struct {
	ProviderID         string `json:"provider_id"`
	Priority           int    `json:"priority"`
	ExposeCost         bool   `json:"expose_cost"`
	ExposeProviderName bool   `json:"expose_provider_name"`
}

func (h *SubAccountNetworkOverridesHandlers) AddProviderOverride(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	subID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verifyOwnership(r.Context(), resellerID, subID); err != nil { respondError(w, err); return }

	var req addProviderOverrideReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	provID, err := uuid.Parse(req.ProviderID)
	if err != nil { respondError(w, shared.ErrInvalidInput("provider_id")); return }

	// Verify provider belongs to reseller (platform OR private of this reseller)
	var ownership string; var sourceClientID *uuid.UUID
	if err := h.pool.QueryRow(r.Context(),
		`SELECT COALESCE(ownership,'platform'), source_client_id FROM providers WHERE id=$1`, provID,
	).Scan(&ownership, &sourceClientID); err != nil {
		respondError(w, shared.ErrNotFound("провайдер")); return
	}
	if ownership == "private" && (sourceClientID == nil || *sourceClientID != resellerID) {
		respondError(w, shared.ErrForbidden("чужой провайдер")); return
	}

	// Check: уже ли есть запись (любой ownership) для этого sub+provider?
	var existing string
	err = h.pool.QueryRow(r.Context(),
		`SELECT ownership FROM client_providers WHERE client_id=$1 AND provider_id=$2`,
		subID, provID).Scan(&existing)
	if err == nil {
		// Already exists: reject with explanation
		respondError(w, shared.ErrConflict("Этот провайдер уже доступен через шаблон. Чтобы изменить параметры — измени provider-set.").
			WithDetails(`{"kind":"already_present","ownership":"`+existing+`"}`))
		return
	}

	if _, err := h.pool.Exec(r.Context(),
		`INSERT INTO client_providers (client_id, provider_id, ownership, shared_priority, expose_cost, expose_provider_name, active)
		 VALUES ($1, $2, 'private', $3, $4, $5, true)`,
		subID, provID, req.Priority, req.ExposeCost, req.ExposeProviderName); err != nil {
		respondError(w, shared.ErrInternalServer("insert")); return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"client_id": subID.String(), "provider_id": provID.String()})
}

func (h *SubAccountNetworkOverridesHandlers) DeleteProviderOverride(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	subID, _ := uuid.Parse(mux.Vars(r)["id"])
	provID, perr := uuid.Parse(mux.Vars(r)["provider_id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("provider_id")); return }
	if err := h.verifyOwnership(r.Context(), resellerID, subID); err != nil { respondError(w, err); return }

	tag, err := h.pool.Exec(r.Context(),
		`DELETE FROM client_providers WHERE client_id=$1 AND provider_id=$2 AND ownership='private'`,
		subID, provID)
	if err != nil { respondError(w, shared.ErrInternalServer("delete")); return }
	if tag.RowsAffected() == 0 { respondError(w, shared.ErrNotFound("override")); return }
	w.WriteHeader(http.StatusNoContent)
}

type setRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Overview — текущий шаблон + список private-провайдеров (overrides).
func (h *SubAccountNetworkOverridesHandlers) Overview(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	subID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verifyOwnership(r.Context(), resellerID, subID); err != nil { respondError(w, err); return }

	var providerSet *setRef
	var psID, psName string
	err := h.pool.QueryRow(r.Context(), `
		SELECT ps.id::text, ps.name
		FROM subaccount_routing_assignment sra
		JOIN reseller_provider_sets ps ON ps.id = sra.provider_set_id
		WHERE sra.client_id = $1`, subID).Scan(&psID, &psName)
	if err == nil {
		providerSet = &setRef{ID: psID, Name: psName}
	} else if err != pgx.ErrNoRows {
		respondError(w, shared.ErrInternalServer("query provider_set")); return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT cp.provider_id::text, p.name, cp.shared_priority, cp.ownership
		FROM client_providers cp
		JOIN providers p ON p.id = cp.provider_id
		WHERE cp.client_id = $1 AND cp.ownership = 'private'`, subID)
	if err != nil { respondError(w, shared.ErrInternalServer("query overrides")); return }
	defer rows.Close()
	overrides := []map[string]interface{}{}
	for rows.Next() {
		var pid, pname, ownership string
		var priority int
		if err := rows.Scan(&pid, &pname, &priority, &ownership); err != nil {
			respondError(w, shared.ErrInternalServer("scan")); return
		}
		overrides = append(overrides, map[string]interface{}{
			"provider_id": pid, "name": pname, "priority": priority, "ownership": ownership,
		})
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("rows")); return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"provider_set":       providerSet,
		"route_set":          nil, // Plan 2
		"provider_overrides": overrides,
		"route_overrides":    []interface{}{},
	})
}
```

Run: `go test ./internal/gateway/portal/handlers/ -run TestProviderOverride -v`
Expected: PASS.

- [ ] **Step 10.3: Commit**

```bash
git add internal/gateway/portal/handlers/subaccount_network_overrides.go internal/gateway/portal/handlers/subaccount_network_overrides_test.go
git commit -m "feat(handlers): /sub-accounts/{id}/network — overview + provider override CRUD"
```

---

### Task 11: Wire handlers в роутер и main

**Files:**
- Modify: `internal/gateway/portal/router/router.go`
- Modify: `cmd/portal-gateway/main.go` (или там где собираются handlers — найти grep'ом `NewResellerRoutingHandlers`)

- [ ] **Step 11.1: Добавить параметры в SetupRouter**

В сигнатуре функции добавить:
```go
networkProvidersHandlers *handlers.NetworkProvidersHandlers,
networkProviderSetsHandlers *handlers.NetworkProviderSetsHandlers,
networkProviderSetItemsHandlers *handlers.NetworkProviderSetItemsHandlers,
networkAssignmentsHandlers *handlers.NetworkAssignmentsHandlers,
subAccountNetworkOverridesHandlers *handlers.SubAccountNetworkOverridesHandlers,
```

- [ ] **Step 11.2: Зарегистрировать роуты**

После строки `// Reseller routing overview` (router.go:489-493) — заменить блок:
```go
// Reseller network management (Plan 1: providers + provider-sets + assignments + overrides)
network := reseller.PathPrefix("/network").Subrouter()
network.HandleFunc("/providers", networkProvidersHandlers.List).Methods("GET")
network.HandleFunc("/providers", networkProvidersHandlers.Create).Methods("POST")
network.HandleFunc("/providers/{id}", networkProvidersHandlers.Update).Methods("PUT")
network.HandleFunc("/providers/{id}", networkProvidersHandlers.Delete).Methods("DELETE")

network.HandleFunc("/provider-sets", networkProviderSetsHandlers.List).Methods("GET")
network.HandleFunc("/provider-sets", networkProviderSetsHandlers.Create).Methods("POST")
network.HandleFunc("/provider-sets/{id}", networkProviderSetsHandlers.Update).Methods("PUT")
network.HandleFunc("/provider-sets/{id}", networkProviderSetsHandlers.Delete).Methods("DELETE")
network.HandleFunc("/provider-sets/{id}/items", networkProviderSetItemsHandlers.ListItems).Methods("GET")
network.HandleFunc("/provider-sets/{id}/items", networkProviderSetItemsHandlers.PutItems).Methods("PUT")

network.HandleFunc("/assignments", networkAssignmentsHandlers.List).Methods("GET")
network.HandleFunc("/assignments/{client_id}", networkAssignmentsHandlers.PutOne).Methods("PUT")
network.HandleFunc("/assignments/bulk", networkAssignmentsHandlers.Bulk).Methods("POST")

// СТАРЫЕ ROUTES /reseller/routing/* остаются (удаление в Plan 2)
resellerRouting := reseller.PathPrefix("/routing").Subrouter()
resellerRouting.HandleFunc("/providers", resellerRoutingHandlers.ListNetworkProviders).Methods("GET")
resellerRouting.HandleFunc("/routes", resellerRoutingHandlers.ListNetworkRoutes).Methods("GET")
resellerRouting.HandleFunc("/bulk-assign", resellerRoutingHandlers.BulkAssignProvider).Methods("POST")
```

И secondary route в защищённый блок (вне `/reseller/`, потому что override-endpoints на `/sub-accounts/{id}/...`):
```go
// Sub-account network management (override level)
subAccounts.HandleFunc("/{id}/network/overview", subAccountNetworkOverridesHandlers.Overview).Methods("GET")
subAccounts.HandleFunc("/{id}/network/provider-overrides", subAccountNetworkOverridesHandlers.AddProviderOverride).Methods("POST")
subAccounts.HandleFunc("/{id}/network/provider-overrides/{provider_id}", subAccountNetworkOverridesHandlers.DeleteProviderOverride).Methods("DELETE")
```

- [ ] **Step 11.3: Wire в main.go (cmd/portal-gateway/main.go)**

Найти место, где создаются handlers (`handlers.New*`), добавить после `resellerRoutingHandlers`:
```go
matRepo := storage.NewResellerProviderSetRepository(dbPool)
matItemsRepo := storage.NewResellerProviderSetItemsRepository(dbPool)
materializer := network.NewProviderSetMaterializer(dbPool, matRepo, matItemsRepo)
networkProvidersHandlers := handlers.NewNetworkProvidersHandlers(dbPool)
networkProviderSetsHandlers := handlers.NewNetworkProviderSetsHandlers(dbPool, materializer)
networkProviderSetItemsHandlers := handlers.NewNetworkProviderSetItemsHandlers(dbPool, materializer)
networkAssignmentsHandlers := handlers.NewNetworkAssignmentsHandlers(dbPool, materializer)
subAccountNetworkOverridesHandlers := handlers.NewSubAccountNetworkOverridesHandlers(dbPool)
```

Передать в `router.SetupRouter(...)`.

Импорт: `"github.com/smpp-server/smpp-server/internal/services/network"`.

- [ ] **Step 11.4: Build + run all tests**

Run:
```
./scripts/check.sh --with-tests
```
Expected: PASS (vet, build, tests, tsc, eslint).

- [ ] **Step 11.5: Commit**

```bash
git add internal/gateway/portal/router/router.go cmd/portal-gateway/main.go
git commit -m "feat(routing): wire /reseller/network/* routes and subaccount network endpoints"
```

---

## Phase E — Frontend infrastructure

### Task 12: Generic `Drawer` компонент

**Files:**
- Create: `portal-frontend/src/components/ui/Drawer.tsx`

- [ ] **Step 12.1: Реализация**

```tsx
import { type ReactNode } from 'react';
import * as Dialog from '@radix-ui/react-dialog';

interface DrawerProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  width?: 'sm' | 'md' | 'lg';
  children: ReactNode;
}

const widthClass = {
  sm: 'w-[420px]',
  md: 'w-[560px]',
  lg: 'w-[720px]',
};

export function Drawer({ open, onClose, title, description, width = 'md', children }: DrawerProps) {
  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/40 z-40" />
        <Dialog.Content
          className={`fixed top-0 right-0 h-full ${widthClass[width]} max-w-[90vw]
            bg-white shadow-xl z-50 flex flex-col
            data-[state=open]:animate-in data-[state=closed]:animate-out`}
        >
          <div className="px-6 py-4 border-b border-gray-200 flex items-start justify-between">
            <div>
              <Dialog.Title className="text-lg font-semibold">{title}</Dialog.Title>
              {description && (
                <Dialog.Description className="text-sm text-gray-500 mt-1">{description}</Dialog.Description>
              )}
            </div>
            <Dialog.Close asChild>
              <button className="text-gray-400 hover:text-gray-600" aria-label="Закрыть">✕</button>
            </Dialog.Close>
          </div>
          <div className="flex-1 overflow-y-auto px-6 py-4">{children}</div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
```

- [ ] **Step 12.2: Commit**

```bash
git add portal-frontend/src/components/ui/Drawer.tsx
git commit -m "feat(ui): generic Drawer component (right-aligned slide-in)"
```

---

### Task 13: API-методы в `api/client.ts`

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 13.1: Добавить namespace `networkApi`**

В `client.ts` найти существующий `resellerApi` (там же `bulkAssignProvider`). Добавить после:

```ts
export const networkApi = {
  // Provider catalog
  listProviders: () =>
    apiCall<{ providers: Array<{ id: string; name: string; ownership: 'platform' | 'private'; smpp_host: string|null; smpp_port: number|null; system_id: string|null; system_type: string|null; active: boolean }> }>(
      'GET', '/portal/v1/reseller/network/providers'),
  createProvider: (body: { name: string; smpp_host: string; smpp_port: number; system_id: string; password: string; system_type: string }) =>
    apiCall('POST', '/portal/v1/reseller/network/providers', body),
  updateProvider: (id: string, body: any) =>
    apiCall('PUT', `/portal/v1/reseller/network/providers/${id}`, body),
  deleteProvider: (id: string) =>
    apiCall('DELETE', `/portal/v1/reseller/network/providers/${id}`),

  // Provider-sets
  listProviderSets: () =>
    apiCall<{ sets: Array<{ id: string; name: string; is_default: boolean; item_count: number; assigned_count: number }> }>(
      'GET', '/portal/v1/reseller/network/provider-sets'),
  createProviderSet: (body: { name: string; is_default: boolean }) =>
    apiCall<{ id: string }>('POST', '/portal/v1/reseller/network/provider-sets', body),
  updateProviderSet: (id: string, body: { name: string; is_default: boolean }) =>
    apiCall('PUT', `/portal/v1/reseller/network/provider-sets/${id}`, body),
  deleteProviderSet: (id: string) =>
    apiCall('DELETE', `/portal/v1/reseller/network/provider-sets/${id}`),
  listProviderSetItems: (id: string) =>
    apiCall<{ items: Array<{ id: string; provider_id: string; provider_name: string; priority: number; expose_cost: boolean; expose_provider_name: boolean }> }>(
      'GET', `/portal/v1/reseller/network/provider-sets/${id}/items`),
  putProviderSetItems: (id: string, items: Array<{ provider_id: string; priority: number; expose_cost: boolean; expose_provider_name: boolean }>) =>
    apiCall('PUT', `/portal/v1/reseller/network/provider-sets/${id}/items`, { items }),

  // Assignments
  listAssignments: () =>
    apiCall<{ assignments: Array<{ client_id: string; sub_account_name: string; provider_set_id: string|null; provider_set_name: string|null; route_set_id: string|null; route_set_name: string|null; has_overrides: boolean; validation_status: 'ok'|'unassigned'|'conflict' }> }>(
      'GET', '/portal/v1/reseller/network/assignments'),
  putAssignment: (clientID: string, body: { provider_set_id: string|null; route_set_id: string|null }) =>
    apiCall('PUT', `/portal/v1/reseller/network/assignments/${clientID}`, body),
  bulkAssign: (body: { client_ids: string[]; provider_set_id: string|null; route_set_id: string|null }) =>
    apiCall<{ results: Array<{ client_id: string; status: 'ok'|'error'; error?: string }> }>(
      'POST', '/portal/v1/reseller/network/assignments/bulk', body),

  // Sub-account overrides
  getSubAccountNetworkOverview: (subID: string) =>
    apiCall<{ provider_set: { id: string; name: string }|null; route_set: any; provider_overrides: Array<{ provider_id: string; name: string; priority: number; ownership: string }>; route_overrides: any[] }>(
      'GET', `/portal/v1/sub-accounts/${subID}/network/overview`),
  addProviderOverride: (subID: string, body: { provider_id: string; priority: number; expose_cost: boolean; expose_provider_name: boolean }) =>
    apiCall('POST', `/portal/v1/sub-accounts/${subID}/network/provider-overrides`, body),
  deleteProviderOverride: (subID: string, providerID: string) =>
    apiCall('DELETE', `/portal/v1/sub-accounts/${subID}/network/provider-overrides/${providerID}`),
};
```

`apiCall` — существующий хелпер в `client.ts`; если сигнатура отличается — адаптировать. Engineer проверит соседний код.

- [ ] **Step 13.2: tsc + eslint**

Run: `cd portal-frontend && npx tsc --noEmit && npx eslint . --max-warnings=69`
Expected: PASS.

- [ ] **Step 13.3: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat(api): networkApi — providers, provider-sets, assignments, overrides"
```

---

## Phase F — Frontend pages

### Task 14: `/network/providers` — каталог провайдеров

**Files:**
- Create: `portal-frontend/src/pages/network/ProvidersCatalogPage.tsx`
- Modify: `portal-frontend/src/App.tsx` (route)
- Modify: `portal-frontend/src/components/layout/Sidebar.tsx` (или где живёт сайдбар — `grep -rn "Маршрутизация" portal-frontend/src/`)

- [ ] **Step 14.1: Реализация ProvidersCatalogPage**

```tsx
import { useState, useEffect } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Drawer } from '../../components/ui/Drawer';
import { Input } from '../../components/ui/Input';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { useToast } from '../../components/ui/Toast';
import { networkApi, ApiError } from '../../api/client';

interface Provider {
  id: string; name: string; ownership: 'platform' | 'private';
  smpp_host: string|null; smpp_port: number|null; system_id: string|null; system_type: string|null; active: boolean;
}

export function ProvidersCatalogPage() {
  const toast = useToast();
  const [list, setList] = useState<Provider[]>([]);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<Provider | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<Provider | null>(null);
  const [form, setForm] = useState({ name: '', smpp_host: '', smpp_port: 2775, system_id: '', password: '', system_type: 'SMPP' });
  const [submitting, setSubmitting] = useState(false);

  const reload = () => networkApi.listProviders().then(r => setList(r.providers)).catch(() => toast.error('Ошибка загрузки'));
  useEffect(() => { reload(); }, []);

  const openCreate = () => { setEditing(null); setForm({ name: '', smpp_host: '', smpp_port: 2775, system_id: '', password: '', system_type: 'SMPP' }); setDrawerOpen(true); };
  const openEdit = (p: Provider) => {
    setEditing(p);
    setForm({ name: p.name, smpp_host: p.smpp_host||'', smpp_port: p.smpp_port||2775, system_id: p.system_id||'', password: '', system_type: p.system_type||'SMPP' });
    setDrawerOpen(true);
  };
  const submit = async () => {
    setSubmitting(true);
    try {
      if (editing) await networkApi.updateProvider(editing.id, form);
      else await networkApi.createProvider(form);
      toast.success(editing ? 'Обновлён' : 'Создан');
      setDrawerOpen(false); reload();
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
    finally { setSubmitting(false); }
  };
  const doDelete = async () => {
    if (!confirmDelete) return;
    try {
      await networkApi.deleteProvider(confirmDelete.id);
      toast.success('Удалён'); setConfirmDelete(null); reload();
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); setConfirmDelete(null); }
  };

  return (
    <div className="max-w-6xl">
      <PageHeader title="Провайдеры" actions={<Button onClick={openCreate}>+ Добавить своего</Button>} />
      <table className="w-full text-sm bg-white rounded-lg border border-gray-200">
        <thead><tr className="bg-gray-50 text-gray-500 text-xs uppercase">
          <th className="text-left p-3">Имя</th><th className="text-left p-3">Ownership</th>
          <th className="text-left p-3">SMPP host</th><th className="text-center p-3">Активен</th>
          <th className="text-right p-3">Действия</th>
        </tr></thead>
        <tbody>{list.map(p => (
          <tr key={p.id} className="border-t border-gray-100">
            <td className="p-3 font-medium">{p.name}</td>
            <td className="p-3"><span className={`px-2 py-0.5 text-xs rounded ${p.ownership === 'private' ? 'bg-blue-100 text-blue-700' : 'bg-gray-100 text-gray-600'}`}>{p.ownership === 'private' ? 'Свой' : 'Платформенный'}</span></td>
            <td className="p-3 text-gray-500">{p.smpp_host || '—'}{p.smpp_port ? `:${p.smpp_port}` : ''}</td>
            <td className="p-3 text-center"><span className={`inline-block w-2 h-2 rounded-full ${p.active ? 'bg-green-500' : 'bg-gray-300'}`} /></td>
            <td className="p-3 text-right">
              {p.ownership === 'private' && (
                <>
                  <button onClick={() => openEdit(p)} className="text-blue-600 hover:underline mr-3">Редактировать</button>
                  <button onClick={() => setConfirmDelete(p)} className="text-red-600 hover:underline">Удалить</button>
                </>
              )}
            </td>
          </tr>
        ))}</tbody>
      </table>

      <Drawer open={drawerOpen} onClose={() => setDrawerOpen(false)} title={editing ? 'Редактировать провайдера' : 'Добавить своего провайдера'} width="md">
        <div className="space-y-4">
          <Input label="Имя" value={form.name} onChange={e => setForm({...form, name: e.target.value})} />
          <Input label="SMPP host" value={form.smpp_host} onChange={e => setForm({...form, smpp_host: e.target.value})} />
          <Input label="SMPP port" type="number" value={String(form.smpp_port)} onChange={e => setForm({...form, smpp_port: parseInt(e.target.value)||2775})} />
          <Input label="System ID" value={form.system_id} onChange={e => setForm({...form, system_id: e.target.value})} />
          <Input label="Password" type="password" value={form.password} onChange={e => setForm({...form, password: e.target.value})} placeholder={editing ? 'оставьте пустым, чтобы не менять' : ''} />
          <Input label="System type" value={form.system_type} onChange={e => setForm({...form, system_type: e.target.value})} />
          <Button onClick={submit} disabled={submitting}>{submitting ? 'Сохранение...' : 'Сохранить'}</Button>
        </div>
      </Drawer>

      <ConfirmDialog open={!!confirmDelete} onClose={() => setConfirmDelete(null)} onConfirm={doDelete} title="Удалить провайдера?" message={`«${confirmDelete?.name}» будет удалён. Если он используется в provider-set'ах или override'ах — операция вернёт 409.`} />
    </div>
  );
}
```

- [ ] **Step 14.2: Добавить route в App.tsx**

Найти блок маршрутизации (`<Route path="/network/...`). Добавить:
```tsx
<Route path="/network/providers" element={<ProvidersCatalogPage />} />
```
И импорт: `import { ProvidersCatalogPage } from './pages/network/ProvidersCatalogPage';`.

- [ ] **Step 14.3: Добавить пункт в сайдбар**

Найти sidebar config (предположительно `Sidebar.tsx` или `layout/AppShell.tsx`). Добавить пункт «Провайдеры» под «Сеть» с `to="/network/providers"`. Engineer найдёт паттерн через `grep -n "Маршрутизация" portal-frontend/src/`.

- [ ] **Step 14.4: tsc + eslint + dev-server smoke**

Run: `cd portal-frontend && npx tsc --noEmit && npx eslint . --max-warnings=69 && npm run dev`
В браузере открыть `/network/providers` — должна показаться таблица. Для теста создать provider — в БД появится запись. Если backend недоступен локально — перенести проверку на сервер через `./scripts/server.sh deploy`.

- [ ] **Step 14.5: Commit**

```bash
git add portal-frontend/src/pages/network/ProvidersCatalogPage.tsx portal-frontend/src/App.tsx portal-frontend/src/components/layout/Sidebar.tsx
git commit -m "feat(portal): /network/providers — каталог провайдеров с CRUD private"
```

---

### Task 15: `/network/provider-sets` — master-detail редактор

**Files:**
- Create: `portal-frontend/src/pages/network/ProviderSetsPage.tsx`
- Modify: `portal-frontend/src/App.tsx`
- Modify: sidebar

- [ ] **Step 15.1: Реализация**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { useToast } from '../../components/ui/Toast';
import { networkApi, ApiError } from '../../api/client';

interface ProviderSet { id: string; name: string; is_default: boolean; item_count: number; assigned_count: number; }
interface SetItem { id: string; provider_id: string; provider_name: string; priority: number; expose_cost: boolean; expose_provider_name: boolean; }
interface Provider { id: string; name: string; ownership: string; }

export function ProviderSetsPage() {
  const toast = useToast();
  const [sets, setSets] = useState<ProviderSet[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [items, setItems] = useState<SetItem[]>([]);
  const [allProviders, setAllProviders] = useState<Provider[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createDefault, setCreateDefault] = useState(false);
  const [addItemOpen, setAddItemOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<ProviderSet | null>(null);

  const reloadSets = useCallback(() => networkApi.listProviderSets().then(r => setSets(r.sets)).catch(() => toast.error('Ошибка')), [toast]);
  const reloadItems = useCallback((id: string) => networkApi.listProviderSetItems(id).then(r => setItems(r.items)), []);

  useEffect(() => { reloadSets(); networkApi.listProviders().then(r => setAllProviders(r.providers)); }, [reloadSets]);
  useEffect(() => { if (selectedID) reloadItems(selectedID); }, [selectedID, reloadItems]);

  const selected = sets.find(s => s.id === selectedID);
  const usedIDs = new Set(items.map(i => i.provider_id));
  const availableProviders = allProviders.filter(p => !usedIDs.has(p.id));

  const create = async () => {
    if (!createName) return;
    try { const r = await networkApi.createProviderSet({ name: createName, is_default: createDefault });
      toast.success('Создан'); setCreateOpen(false); setCreateName(''); setCreateDefault(false);
      await reloadSets(); setSelectedID(r.id);
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };
  const renameSelected = async (newName: string) => {
    if (!selected) return;
    try { await networkApi.updateProviderSet(selected.id, { name: newName, is_default: selected.is_default }); reloadSets(); }
    catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };
  const toggleDefault = async () => {
    if (!selected) return;
    try { await networkApi.updateProviderSet(selected.id, { name: selected.name, is_default: !selected.is_default }); reloadSets(); }
    catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };
  const addItem = async (providerID: string) => {
    if (!selected) return;
    const newItems = [...items.map(i => ({ provider_id: i.provider_id, priority: i.priority, expose_cost: i.expose_cost, expose_provider_name: i.expose_provider_name })), {
      provider_id: providerID, priority: 0, expose_cost: false, expose_provider_name: true,
    }];
    try { await networkApi.putProviderSetItems(selected.id, newItems); setAddItemOpen(false); reloadItems(selected.id); reloadSets(); }
    catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };
  const removeItem = async (providerID: string) => {
    if (!selected) return;
    const newItems = items.filter(i => i.provider_id !== providerID).map(i => ({ provider_id: i.provider_id, priority: i.priority, expose_cost: i.expose_cost, expose_provider_name: i.expose_provider_name }));
    try { await networkApi.putProviderSetItems(selected.id, newItems); reloadItems(selected.id); reloadSets(); }
    catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };
  const updateItemField = async (providerID: string, field: 'priority'|'expose_cost'|'expose_provider_name', value: number|boolean) => {
    if (!selected) return;
    const newItems = items.map(i => i.provider_id === providerID
      ? { provider_id: i.provider_id, priority: field === 'priority' ? Number(value) : i.priority, expose_cost: field === 'expose_cost' ? Boolean(value) : i.expose_cost, expose_provider_name: field === 'expose_provider_name' ? Boolean(value) : i.expose_provider_name }
      : { provider_id: i.provider_id, priority: i.priority, expose_cost: i.expose_cost, expose_provider_name: i.expose_provider_name });
    try { await networkApi.putProviderSetItems(selected.id, newItems); reloadItems(selected.id); }
    catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };
  const deleteSet = async () => {
    if (!confirmDelete) return;
    try { await networkApi.deleteProviderSet(confirmDelete.id); toast.success('Удалён'); setConfirmDelete(null); if (selectedID === confirmDelete.id) setSelectedID(null); reloadSets(); }
    catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); setConfirmDelete(null); }
  };

  return (
    <div className="max-w-7xl">
      <PageHeader title="Provider-sets" actions={<Button onClick={() => setCreateOpen(true)}>+ Создать</Button>} />
      <div className="grid grid-cols-12 gap-4">
        <div className="col-span-4 bg-white border border-gray-200 rounded-lg overflow-hidden">
          {sets.length === 0 ? (
            <div className="p-6 text-center text-gray-400 text-sm">Нет шаблонов</div>
          ) : sets.map(s => (
            <div key={s.id} onClick={() => setSelectedID(s.id)}
                 className={`p-3 border-b border-gray-100 cursor-pointer ${selectedID === s.id ? 'bg-blue-50' : 'hover:bg-gray-50'}`}>
              <div className="flex items-center justify-between">
                <span className="font-medium">{s.name}</span>
                {s.is_default && <span className="px-2 text-xs bg-amber-100 text-amber-700 rounded">default</span>}
              </div>
              <div className="text-xs text-gray-500 mt-1">{s.item_count} провайдеров · {s.assigned_count} назначений</div>
            </div>
          ))}
        </div>

        <div className="col-span-8 bg-white border border-gray-200 rounded-lg p-4">
          {!selected ? (
            <div className="text-center text-gray-400 py-12">Выбери шаблон или создай новый</div>
          ) : (
            <>
              <div className="flex items-center justify-between mb-4">
                <Input value={selected.name} onChange={e => { /* inline rename via blur */ }}
                       onBlur={e => { if (e.target.value !== selected.name) renameSelected(e.target.value); }}
                       className="text-lg font-semibold border-none focus:border-gray-300" />
                <div className="flex gap-3 items-center">
                  <label className="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={selected.is_default} onChange={toggleDefault} />
                    По умолчанию для новых суб-аккаунтов
                  </label>
                  <button onClick={() => setConfirmDelete(selected)} className="text-red-600 hover:underline text-sm">Удалить</button>
                </div>
              </div>
              <table className="w-full text-sm">
                <thead><tr className="text-xs text-gray-500 uppercase">
                  <th className="text-left p-2">Провайдер</th>
                  <th className="text-center p-2">Приоритет</th>
                  <th className="text-center p-2">Expose cost</th>
                  <th className="text-center p-2">Expose name</th>
                  <th></th>
                </tr></thead>
                <tbody>{items.map(i => (
                  <tr key={i.id} className="border-t border-gray-100">
                    <td className="p-2">{i.provider_name}</td>
                    <td className="p-2 text-center">
                      <input type="number" defaultValue={i.priority} onBlur={e => updateItemField(i.provider_id, 'priority', parseInt(e.target.value)||0)} className="w-16 border border-gray-200 rounded px-1 text-center" />
                    </td>
                    <td className="p-2 text-center"><input type="checkbox" checked={i.expose_cost} onChange={e => updateItemField(i.provider_id, 'expose_cost', e.target.checked)} /></td>
                    <td className="p-2 text-center"><input type="checkbox" checked={i.expose_provider_name} onChange={e => updateItemField(i.provider_id, 'expose_provider_name', e.target.checked)} /></td>
                    <td className="p-2 text-right"><button onClick={() => removeItem(i.provider_id)} className="text-red-600 hover:underline text-xs">Удалить</button></td>
                  </tr>
                ))}</tbody>
              </table>
              <Button variant="secondary" onClick={() => setAddItemOpen(true)} className="mt-3">+ Добавить провайдера</Button>
            </>
          )}
        </div>
      </div>

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="Создать provider-set">
        <div className="space-y-3">
          <Input label="Имя" value={createName} onChange={e => setCreateName(e.target.value)} />
          <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={createDefault} onChange={e => setCreateDefault(e.target.checked)} /> По умолчанию для новых суб-аккаунтов</label>
          <Button onClick={create}>Создать</Button>
        </div>
      </Modal>

      <Modal open={addItemOpen} onClose={() => setAddItemOpen(false)} title="Добавить провайдера в шаблон">
        <div className="max-h-80 overflow-y-auto">
          {availableProviders.length === 0 ? (
            <div className="text-sm text-gray-500">Все провайдеры уже добавлены</div>
          ) : availableProviders.map(p => (
            <button key={p.id} onClick={() => addItem(p.id)} className="block w-full text-left p-2 hover:bg-gray-50 rounded">
              <span className="font-medium">{p.name}</span>
              <span className={`ml-2 text-xs px-1.5 rounded ${p.ownership === 'private' ? 'bg-blue-100 text-blue-700' : 'bg-gray-100 text-gray-600'}`}>{p.ownership === 'private' ? 'свой' : 'платформа'}</span>
            </button>
          ))}
        </div>
      </Modal>

      <ConfirmDialog open={!!confirmDelete} onClose={() => setConfirmDelete(null)} onConfirm={deleteSet} title="Удалить provider-set?" message={`«${confirmDelete?.name}» будет удалён. Если он назначен суб-аккаунтам — операция вернёт 409.`} />
    </div>
  );
}
```

- [ ] **Step 15.2: Route + sidebar**

App.tsx:
```tsx
<Route path="/network/provider-sets" element={<ProviderSetsPage />} />
```
Sidebar: «Provider-sets» → `/network/provider-sets`.

- [ ] **Step 15.3: tsc + eslint**

Run: `cd portal-frontend && npx tsc --noEmit && npx eslint . --max-warnings=69`

- [ ] **Step 15.4: Commit**

```bash
git add portal-frontend/src/pages/network/ProviderSetsPage.tsx portal-frontend/src/App.tsx portal-frontend/src/components/layout/Sidebar.tsx
git commit -m "feat(portal): /network/provider-sets — master-detail editor"
```

---

### Task 16: Упрощённая `/network/assignments` (Plan 1: только provider-set)

**Files:**
- Create: `portal-frontend/src/pages/network/AssignmentsPage.tsx`
- Modify: `portal-frontend/src/App.tsx`
- Modify: sidebar

- [ ] **Step 16.1: Реализация**

```tsx
import { useState, useEffect } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { networkApi, ApiError } from '../../api/client';

interface Assignment { client_id: string; sub_account_name: string; provider_set_id: string|null; provider_set_name: string|null; route_set_id: string|null; route_set_name: string|null; has_overrides: boolean; validation_status: 'ok'|'unassigned'|'conflict'; }
interface ProviderSet { id: string; name: string; }

export function AssignmentsPage() {
  const toast = useToast();
  const [list, setList] = useState<Assignment[]>([]);
  const [providerSets, setProviderSets] = useState<ProviderSet[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState('');
  const [bulkSetID, setBulkSetID] = useState('');

  const reload = () => networkApi.listAssignments().then(r => setList(r.assignments)).catch(() => toast.error('Ошибка'));
  useEffect(() => { reload(); networkApi.listProviderSets().then(r => setProviderSets(r.sets)); }, []);

  const filtered = list.filter(a => a.sub_account_name.toLowerCase().includes(search.toLowerCase()));

  const setOne = async (clientID: string, providerSetID: string|null) => {
    try { await networkApi.putAssignment(clientID, { provider_set_id: providerSetID, route_set_id: null }); toast.success('Назначено'); reload(); }
    catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };

  const bulkApply = async () => {
    if (selected.size === 0) return;
    try {
      const result = await networkApi.bulkAssign({
        client_ids: Array.from(selected),
        provider_set_id: bulkSetID || null, route_set_id: null,
      });
      const ok = result.results.filter(r => r.status === 'ok').length;
      toast.success(`Назначено: ${ok} из ${selected.size}`);
      setSelected(new Set()); reload();
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };

  const toggleAll = (checked: boolean) => setSelected(checked ? new Set(filtered.map(a => a.client_id)) : new Set());
  const toggle = (id: string) => { const next = new Set(selected); if (next.has(id)) next.delete(id); else next.add(id); setSelected(next); };

  return (
    <div className="max-w-6xl">
      <PageHeader title="Назначения суб-аккаунтам" />
      {selected.size > 0 && (
        <div className="sticky top-0 bg-blue-50 border border-blue-200 rounded p-3 mb-4 flex items-center gap-3 z-10">
          <span className="font-medium">Выбрано: {selected.size}</span>
          <select value={bulkSetID} onChange={e => setBulkSetID(e.target.value)} className="border rounded px-2 py-1 text-sm">
            <option value="">— снять provider-set —</option>
            {providerSets.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
          <Button onClick={bulkApply}>Применить</Button>
          <Button variant="secondary" onClick={() => setSelected(new Set())}>Отменить</Button>
        </div>
      )}
      <div className="mb-3"><input type="text" placeholder="Поиск..." value={search} onChange={e => setSearch(e.target.value)} className="border border-gray-300 rounded px-3 py-2 text-sm w-64" /></div>
      <table className="w-full text-sm bg-white rounded-lg border border-gray-200">
        <thead><tr className="bg-gray-50 text-gray-500 text-xs uppercase">
          <th className="p-3"><input type="checkbox" checked={selected.size === filtered.length && filtered.length > 0} onChange={e => toggleAll(e.target.checked)} /></th>
          <th className="text-left p-3">Суб-аккаунт</th>
          <th className="text-left p-3">Provider-set</th>
          <th className="text-center p-3">Override</th>
          <th className="text-center p-3">Статус</th>
        </tr></thead>
        <tbody>{filtered.map(a => (
          <tr key={a.client_id} className="border-t border-gray-100">
            <td className="p-3 text-center"><input type="checkbox" checked={selected.has(a.client_id)} onChange={() => toggle(a.client_id)} /></td>
            <td className="p-3"><a href={`/network/sub-accounts/${a.client_id}`} className="text-blue-600 hover:underline">{a.sub_account_name}</a></td>
            <td className="p-3">
              <select value={a.provider_set_id || ''} onChange={e => setOne(a.client_id, e.target.value || null)} className="border rounded px-2 py-1 text-sm">
                <option value="">—</option>
                {providerSets.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
              </select>
            </td>
            <td className="p-3 text-center">{a.has_overrides ? <span className="text-amber-700 text-xs">override</span> : '—'}</td>
            <td className="p-3 text-center">{a.validation_status === 'ok' ? '✓' : a.validation_status === 'unassigned' ? '—' : '⚠'}</td>
          </tr>
        ))}</tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 16.2: Route + sidebar + redirect старого `/network/routing`**

В App.tsx:
```tsx
<Route path="/network/assignments" element={<AssignmentsPage />} />
<Route path="/network/routing" element={<Navigate to="/network/assignments" replace />} />
```

- [ ] **Step 16.3: tsc + eslint, commit**

```bash
git add portal-frontend/src/pages/network/AssignmentsPage.tsx portal-frontend/src/App.tsx portal-frontend/src/components/layout/Sidebar.tsx
git commit -m "feat(portal): /network/assignments — provider-set assignments + bulk + redirect from /network/routing"
```

---

### Task 17: Секция «Сеть» в карточке суб-аккаунта (override провайдеров)

**Files:**
- Create: `portal-frontend/src/pages/sub-accounts/SubAccountNetworkSection.tsx`
- Modify: `portal-frontend/src/pages/sub-accounts/SubAccountPage.tsx` (или соседний — найти grep'ом «суб-аккаунт» / «sub-account»)

- [ ] **Step 17.1: Найти место и встроить секцию**

```bash
grep -n "sub-account\|SubAccount" portal-frontend/src/App.tsx | head -10
```
Найти страницу карточки суб-аккаунта. Если её ещё нет — Engineer создаёт минимальный route `/network/sub-accounts/:id` с импортом этой секции.

- [ ] **Step 17.2: Реализация**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { Modal } from '../../components/ui/Modal';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { useToast } from '../../components/ui/Toast';
import { networkApi, ApiError } from '../../api/client';

interface Props { subAccountID: string; subAccountName: string; }

export function SubAccountNetworkSection({ subAccountID, subAccountName }: Props) {
  const toast = useToast();
  const [overview, setOverview] = useState<any>(null);
  const [providers, setProviders] = useState<Array<{id:string;name:string;ownership:string}>>([]);
  const [providerSets, setProviderSets] = useState<Array<{id:string;name:string}>>([]);
  const [addOpen, setAddOpen] = useState(false);
  const [changeSetOpen, setChangeSetOpen] = useState(false);
  const [newSetID, setNewSetID] = useState('');
  const [addForm, setAddForm] = useState({ provider_id: '', priority: 0, expose_cost: false, expose_provider_name: true });

  const reload = useCallback(() => networkApi.getSubAccountNetworkOverview(subAccountID).then(setOverview), [subAccountID]);
  useEffect(() => {
    reload();
    networkApi.listProviders().then(r => setProviders(r.providers));
    networkApi.listProviderSets().then(r => setProviderSets(r.sets));
  }, [reload]);

  const changeSet = async () => {
    try { await networkApi.putAssignment(subAccountID, { provider_set_id: newSetID || null, route_set_id: null }); toast.success('Изменено'); setChangeSetOpen(false); reload(); }
    catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };
  const addOverride = async () => {
    if (!addForm.provider_id) return;
    try { await networkApi.addProviderOverride(subAccountID, addForm); toast.success('Добавлено'); setAddOpen(false); reload(); setAddForm({ provider_id: '', priority: 0, expose_cost: false, expose_provider_name: true }); }
    catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };
  const removeOverride = async (providerID: string) => {
    try { await networkApi.deleteProviderOverride(subAccountID, providerID); toast.success('Удалено'); reload(); }
    catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };

  if (!overview) return <div>Загрузка...</div>;

  return (
    <div className="space-y-6">
      <div className="bg-white border border-gray-200 rounded-lg p-4">
        <div className="flex items-center justify-between mb-2">
          <h3 className="font-semibold">Provider-set</h3>
          <Button variant="secondary" onClick={() => { setNewSetID(overview.provider_set?.id || ''); setChangeSetOpen(true); }}>Изменить</Button>
        </div>
        <div className="text-sm text-gray-600">{overview.provider_set?.name || 'Не назначен'}</div>
      </div>

      <div className="bg-white border border-gray-200 rounded-lg p-4">
        <div className="flex items-center justify-between mb-3">
          <h3 className="font-semibold">Кастомные провайдеры (overrides)</h3>
          <Button variant="secondary" onClick={() => setAddOpen(true)}>+ Добавить</Button>
        </div>
        <p className="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded p-2 mb-3">Эти настройки переопределяют шаблон. Изменения шаблона не затронут эти записи.</p>
        {overview.provider_overrides.length === 0 ? (
          <div className="text-sm text-gray-400">Override-провайдеров нет</div>
        ) : (
          <table className="w-full text-sm">
            <thead><tr className="text-xs text-gray-500 uppercase"><th className="text-left p-2">Провайдер</th><th>Приоритет</th><th></th></tr></thead>
            <tbody>{overview.provider_overrides.map((o: any) => (
              <tr key={o.provider_id} className="border-t border-gray-100">
                <td className="p-2">{o.name}</td>
                <td className="p-2 text-center">{o.priority}</td>
                <td className="p-2 text-right"><button onClick={() => removeOverride(o.provider_id)} className="text-red-600 hover:underline text-xs">Удалить</button></td>
              </tr>
            ))}</tbody>
          </table>
        )}
      </div>

      <Modal open={changeSetOpen} onClose={() => setChangeSetOpen(false)} title={`Provider-set для ${subAccountName}`}>
        <select value={newSetID} onChange={e => setNewSetID(e.target.value)} className="border rounded px-3 py-2 text-sm w-full mb-3">
          <option value="">— снять —</option>
          {providerSets.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select>
        <Button onClick={changeSet}>Применить</Button>
      </Modal>

      <Modal open={addOpen} onClose={() => setAddOpen(false)} title="Добавить провайдер override">
        <div className="space-y-3">
          <select value={addForm.provider_id} onChange={e => setAddForm({...addForm, provider_id: e.target.value})} className="border rounded px-3 py-2 text-sm w-full">
            <option value="">— выбрать —</option>
            {providers.map(p => <option key={p.id} value={p.id}>{p.name} ({p.ownership === 'private' ? 'свой' : 'платформа'})</option>)}
          </select>
          <Input label="Приоритет" type="number" value={String(addForm.priority)} onChange={e => setAddForm({...addForm, priority: parseInt(e.target.value)||0})} />
          <Button onClick={addOverride} disabled={!addForm.provider_id}>Добавить</Button>
        </div>
      </Modal>
    </div>
  );
}
```

- [ ] **Step 17.3: Встроить в страницу карточки суб-аккаунта**

Найти `SubAccountPage.tsx` или эквивалент. Добавить вкладку «Сеть» (или секцию) с `<SubAccountNetworkSection subAccountID={...} subAccountName={...} />`.

- [ ] **Step 17.4: tsc + eslint + commit**

```bash
git add portal-frontend/src/pages/sub-accounts/SubAccountNetworkSection.tsx portal-frontend/src/pages/sub-accounts/SubAccountPage.tsx
git commit -m "feat(portal): SubAccountNetworkSection — provider override management в карточке"
```

---

## Phase G — E2E

### Task 18: E2E — providers + provider-sets + assignment flow

**Files:**
- Modify: `e2e/tests/reseller/network-routing.spec.ts`
- Create: `e2e/pages/ProvidersCatalogPage.ts`
- Create: `e2e/pages/ProviderSetsPage.ts`
- Create: `e2e/pages/AssignmentsPage.ts`

- [ ] **Step 18.1: Page-objects**

```ts
// e2e/pages/ProvidersCatalogPage.ts
import type { Page } from '@playwright/test';

export class ProvidersCatalogPage {
  constructor(private page: Page) {}
  async goto() { await this.page.goto('/network/providers'); }
  async addPrivate(name: string, host: string) {
    await this.page.click('text=+ Добавить своего');
    await this.page.fill('label:has-text("Имя") + input, input[placeholder*="имя"], input', name);
    await this.page.fill('label:has-text("SMPP host") + input', host);
    await this.page.fill('label:has-text("System ID") + input', 'sys');
    await this.page.fill('label:has-text("Password") + input', 'pwd');
    await this.page.click('button:has-text("Сохранить")');
  }
  async expectRow(name: string) { await this.page.locator(`tr:has-text("${name}")`).waitFor(); }
}
```

Аналогично `ProviderSetsPage.ts` и `AssignmentsPage.ts` — engineer пишет page-objects по той же схеме (см. существующие в `e2e/pages/` для образца паттернов).

- [ ] **Step 18.2: Spec-сценарий**

```ts
// e2e/tests/reseller/network-routing.spec.ts (заменить содержимое)
import { test, expect } from '@playwright/test';
import { loginAsAggregator } from '../../helpers/auth';
import { ProvidersCatalogPage } from '../../pages/ProvidersCatalogPage';
import { ProviderSetsPage } from '../../pages/ProviderSetsPage';
import { AssignmentsPage } from '../../pages/AssignmentsPage';

test.describe('Aggregator network routing — Plan 1', () => {
  test('full flow: create private provider → add to set → assign to subaccount', async ({ page }) => {
    await loginAsAggregator(page);
    const cat = new ProvidersCatalogPage(page);
    await cat.goto();
    await cat.addPrivate('E2E Provider', 'smpp.example.com');
    await cat.expectRow('E2E Provider');

    const sets = new ProviderSetsPage(page);
    await sets.goto();
    await sets.create('E2E Set');
    await sets.addItemByName('E2E Provider');
    await sets.expectItemRow('E2E Provider');

    const assigns = new AssignmentsPage(page);
    await assigns.goto();
    await assigns.assignFirstSubAccount('E2E Set');
    await expect(page.locator('text=Назначено')).toBeVisible();
  });

  test('redirect from old /network/routing', async ({ page }) => {
    await loginAsAggregator(page);
    await page.goto('/network/routing');
    await expect(page).toHaveURL(/\/network\/assignments/);
  });
});
```

`loginAsAggregator` использует тестовые креды `aggregator@test.local / Admin123!` (см. `project_sandbox_test_credentials`). Если такого helper'а нет — добавить в `e2e/helpers/auth.ts`.

- [ ] **Step 18.3: Запустить E2E локально**

Run:
```
cd e2e && npx playwright test reseller/network-routing.spec.ts
```
Expected: PASS. Если падает на specific selector — engineer уточняет селектор после визуальной проверки в браузере.

- [ ] **Step 18.4: Commit**

```bash
git add e2e/tests/reseller/network-routing.spec.ts e2e/pages/ProvidersCatalogPage.ts e2e/pages/ProviderSetsPage.ts e2e/pages/AssignmentsPage.ts
git commit -m "test(e2e): aggregator network Plan 1 — providers + provider-sets + assignment flow"
```

---

## Phase H — Server smoke

### Task 19: Deploy и smoke на sms-server

**Files:** none

- [ ] **Step 19.1: Push + deploy**

```
git push origin master
./scripts/server.sh deploy
./scripts/server.sh migrate
./scripts/server.sh status
```

Expected: контейнеры зелёные, миграции применены.

- [ ] **Step 19.2: Браузер-smoke (chrome-devtools-mcp или вручную)**

Открыть `http://72.56.232.202:18085`, войти как `aggregator@test.local / Admin123!`. Проверить:
1. `/network/providers` — список существующих провайдеров; добавление private — успех
2. `/network/provider-sets` — создание шаблона + добавление провайдера + удаление
3. `/network/assignments` — назначение шаблона одному, bulk — нескольким
4. Карточка суб-аккаунта → секция «Сеть» — изменение шаблона + добавление override
5. `/network/routing` → редирект на `/network/assignments`

Проверка через psql:
```
./scripts/server.sh exec "psql -U sms -d sms -c \"SELECT count(*) FROM client_providers WHERE ownership='inherited'\""
```
После назначения шаблона из 2 провайдеров на 1 суб-аккаунт — должно быть `>= 2`.

- [ ] **Step 19.3: Commit smoke-результат**

Если найдены баги — фиксить в новых задачах follow-up'ами с явными commit-сообщениями.

---

## Plan 1 Done

После Task 19:
- Агрегатор управляет каталогом провайдеров (включая private)
- Создаёт provider-set'ы и назначает их одному или массово суб-аккаунтам
- Override провайдеров в карточке суб-аккаунта
- Старая страница `/network/routing` → редирект на `/network/assignments`
- Старые backend endpoints `/reseller/routing/*` ОСТАЮТСЯ (удаление в Plan 2 после готовности route-set'ов)

**Не включено (Plan 2):**
- route-sets, route-set-items, condition-groups, schedules
- preview маршрута
- override route'ов
- удаление `NetworkRoutingPage.tsx` и `/reseller/routing/*` endpoints
- secutity tightening cross-reseller (если выявят на smoke)
