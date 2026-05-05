# Aggregator Routing — Plan 2: Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. CLAUDE.md mandates `/execute-with-review` (memory `feedback_review_gate_no_skip`); каждая task проходит implementer subagent → spec compliance reviewer → `superpowers:code-reviewer` → fix iterations → mark complete. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Достроить раздел «Сеть» агрегатора маршрутами: route-set'ы (правила маршрутизации с условиями и расписаниями), назначение route-set'ов суб-аккаунтам с pre-validation провайдер-конфликтов, override-маршруты в карточке суб-аккаунта, симулятор matching'а и cascade-cleanup при удалении провайдера. Удаляет legacy `/reseller/routing/*` endpoints и `NetworkRoutingPage.tsx`.

**Architecture:** Зеркало модели Plan 1 для маршрутов: `reseller_route_sets` + `reseller_route_set_items` хранят шаблон, `route_set_condition_groups` / `route_set_conditions` / `route_set_schedules` — условия/расписания. `RouteSetMaterializer` под транзакцией копирует шаблон в `client_routes` (source='template') + `route_condition_groups` / `route_conditions` / `route_schedules` (FK на `client_routes.id`). Override-маршруты живут в тех же `client_routes` с `source='override'` и не затрагиваются материализатором — точная семантика как `ownership='private'` для провайдеров.

**Tech Stack:** Go 1.24 (gorilla/mux, jackc/pgx/v5, testify, zerolog), React 19 + TS 5.7 (Vite, Radix UI, Tailwind 4.2), Playwright для E2E, native HTML5 drag&drop (без новых зависимостей).

**Spec:** [docs/superpowers/specs/2026-05-04-aggregator-routing-management-design.md](../specs/2026-05-04-aggregator-routing-management-design.md)

**Plan 1 (DONE, паттерны для копирования):** [docs/superpowers/plans/2026-05-04-aggregator-routing-plan-1-foundation.md](2026-05-04-aggregator-routing-plan-1-foundation.md)

**Соглашение по details в 409:** `shared.AppError.Details` — строка. Структурные details кодируем как JSON-строку (тот же паттерн, что в Plan 1):
- `kind: "route_uses_unavailable_provider"` — provider_id маршрута вне provider-set назначенного suб-аккаунта
- `kind: "provider_used_in_routes"` — попытка удалить провайдера, используемого в route-set'ах или override-маршрутах
- `kind: "route_set_assigned"` — попытка удалить route-set, назначенный суб-аккаунту
- `kind: "items": [...]` — детальный список конфликтов

**Что НЕ делаем (отложено):**
- Версионирование шаблонов / откат (spec §4.6)
- Optimistic concurrency (If-Unmodified-Since/412) (spec §6.5)
- Двухуровневая reseller-иерархия (spec §3.4)
- Property-based testing (spec §7.4)
- Audit-log UI (данные пишем — UI позже, spec §6.8)

**Workflow per task:**
1. Implementer subagent (задача из плана как brief).
2. Spec compliance reviewer subagent (проверяет соответствие спеке).
3. Code quality reviewer (`superpowers:code-reviewer` subagent) (проверяет чистоту, тесты, конвенции).
4. Если CHANGES_REQUESTED — фикс-итерация (макс 3); каждый фикс = новый коммит, не amend.
5. APPROVED → mark task completed.

**Запрет** на `git commit --amend`, force-push, `git add .`/`-A`. Каждый implementer brief должен это явно содержать.

---

## Phase A — Migrations

### Task 1: Создать миграции для reseller_route_sets + items

**Files:**
- Create: `migrations/000133_create_reseller_route_sets.up.sql`
- Create: `migrations/000133_create_reseller_route_sets.down.sql`
- Create: `migrations/000134_create_reseller_route_set_items.up.sql`
- Create: `migrations/000134_create_reseller_route_set_items.down.sql`

- [ ] **Step 1.1: Написать `000133_create_reseller_route_sets.up.sql`**

```sql
CREATE TABLE IF NOT EXISTS reseller_route_sets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reseller_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_reseller_route_sets_name UNIQUE (reseller_id, name)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_reseller_route_sets_default
    ON reseller_route_sets (reseller_id) WHERE is_default = true;

CREATE INDEX IF NOT EXISTS idx_reseller_route_sets_reseller
    ON reseller_route_sets (reseller_id);

CREATE TRIGGER update_reseller_route_sets_updated_at
    BEFORE UPDATE ON reseller_route_sets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

- [ ] **Step 1.2: Написать `000133_create_reseller_route_sets.down.sql`**

```sql
DROP TRIGGER IF EXISTS update_reseller_route_sets_updated_at ON reseller_route_sets;
DROP INDEX IF EXISTS uq_reseller_route_sets_default;
DROP INDEX IF EXISTS idx_reseller_route_sets_reseller;
DROP TABLE IF EXISTS reseller_route_sets;
```

- [ ] **Step 1.3: Написать `000134_create_reseller_route_set_items.up.sql`**

Зеркало `client_routes` без `client_id`/`operator_id` (operator опускаем — operator_id в client_routes сейчас всегда NULL после миграции 080, эту дискуссию ведёт spec). Поля `name, comment, provider_id, priority, share, route_type, status` повторяют client_routes.

```sql
CREATE TABLE IF NOT EXISTS reseller_route_set_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    set_id UUID NOT NULL REFERENCES reseller_route_sets(id) ON DELETE CASCADE,
    name VARCHAR(255),
    comment TEXT,
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
    priority INT NOT NULL DEFAULT 0,
    share INT NOT NULL DEFAULT 100,
    route_type VARCHAR(10) NOT NULL DEFAULT 'sms',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reseller_route_set_items_set
    ON reseller_route_set_items (set_id);
CREATE INDEX IF NOT EXISTS idx_reseller_route_set_items_provider
    ON reseller_route_set_items (provider_id);

CREATE TRIGGER update_reseller_route_set_items_updated_at
    BEFORE UPDATE ON reseller_route_set_items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

- [ ] **Step 1.4: Написать `000134_create_reseller_route_set_items.down.sql`**

```sql
DROP TRIGGER IF EXISTS update_reseller_route_set_items_updated_at ON reseller_route_set_items;
DROP INDEX IF EXISTS idx_reseller_route_set_items_set;
DROP INDEX IF EXISTS idx_reseller_route_set_items_provider;
DROP TABLE IF EXISTS reseller_route_set_items;
```

- [ ] **Step 1.5: Применить миграции на сервере**

Run:
```
git add migrations/000133_create_reseller_route_sets.up.sql migrations/000133_create_reseller_route_sets.down.sql migrations/000134_create_reseller_route_set_items.up.sql migrations/000134_create_reseller_route_set_items.down.sql
git commit -m "feat(migrations): reseller_route_sets + items (Plan 2 Task 1)"
git push origin master
./scripts/server.sh migrate
```
Expected: `migrations applied: 133, 134`. Verify:
```
./scripts/server.sh exec "psql -U smpp -d smpp_db -c '\d reseller_route_set_items'"
```

---

### Task 2: Миграции для condition_groups / conditions / schedules route-set'ов

**Files:**
- Create: `migrations/000135_create_route_set_conditions.up.sql`
- Create: `migrations/000135_create_route_set_conditions.down.sql`

- [ ] **Step 2.1: Написать `000135_create_route_set_conditions.up.sql`**

Зеркало миграции [000077](../../migrations/000077_routing_overhaul.up.sql) — `route_condition_groups` / `route_conditions` / `route_schedules` — но с FK на `reseller_route_set_items.id` вместо `client_routes.id`.

```sql
CREATE TABLE IF NOT EXISTS route_set_condition_groups (
    id          BIGSERIAL PRIMARY KEY,
    item_id     UUID NOT NULL REFERENCES reseller_route_set_items(id) ON DELETE CASCADE,
    group_index SMALLINT NOT NULL,
    logic_op    VARCHAR(10) NOT NULL,  -- 'IF', 'AND', 'AND_NOT', 'OR', 'OR_NOT'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rscg_item ON route_set_condition_groups(item_id);

CREATE TABLE IF NOT EXISTS route_set_conditions (
    id              BIGSERIAL PRIMARY KEY,
    group_id        BIGINT NOT NULL REFERENCES route_set_condition_groups(id) ON DELETE CASCADE,
    condition_type  VARCHAR(30) NOT NULL,  -- 'operator','country','traffic_type','paid_name','regex'
    condition_value TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rsc_group ON route_set_conditions(group_id);

CREATE TABLE IF NOT EXISTS route_set_schedules (
    id         BIGSERIAL PRIMARY KEY,
    item_id    UUID NOT NULL REFERENCES reseller_route_set_items(id) ON DELETE CASCADE,
    date_from  DATE,
    date_to    DATE,
    time_from  TIME,
    time_to    TIME,
    weekdays   SMALLINT NOT NULL DEFAULT 127,
    timezone   VARCHAR(50) NOT NULL DEFAULT 'Europe/Moscow',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rss_item ON route_set_schedules(item_id);
```

- [ ] **Step 2.2: Написать `000135_create_route_set_conditions.down.sql`**

```sql
DROP INDEX IF EXISTS idx_rss_item;
DROP TABLE IF EXISTS route_set_schedules;
DROP INDEX IF EXISTS idx_rsc_group;
DROP TABLE IF EXISTS route_set_conditions;
DROP INDEX IF EXISTS idx_rscg_item;
DROP TABLE IF EXISTS route_set_condition_groups;
```

- [ ] **Step 2.3: Применить + commit**

```
git add migrations/000135_create_route_set_conditions.up.sql migrations/000135_create_route_set_conditions.down.sql
git commit -m "feat(migrations): route_set condition groups + conditions + schedules (Plan 2 Task 2)"
git push origin master
./scripts/server.sh migrate
```
Verify:
```
./scripts/server.sh exec "psql -U smpp -d smpp_db -c '\d route_set_condition_groups'"
```

---

### Task 3: Миграция client_routes.source + FK на subaccount_routing_assignment.route_set_id

**Files:**
- Create: `migrations/000136_client_routes_source_and_sra_fk.up.sql`
- Create: `migrations/000136_client_routes_source_and_sra_fk.down.sql`

- [ ] **Step 3.1: Написать up-миграцию**

```sql
-- Add source column to client_routes (template = materialized from route-set, override = sub-account custom)
ALTER TABLE client_routes
    ADD COLUMN IF NOT EXISTS source VARCHAR(10) NOT NULL DEFAULT 'override'
        CHECK (source IN ('template','override'));

-- Backfill: existing rows are 'override' (no route-set assignments yet, see spec §3.2).
-- The DEFAULT covers them; nothing else to do.

CREATE INDEX IF NOT EXISTS idx_client_routes_client_source
    ON client_routes (client_id, source) WHERE active = true;

-- Add FK on subaccount_routing_assignment.route_set_id (was loose UUID in Plan 1 task 1).
-- ON DELETE SET NULL: при удалении route-set'а assignment теряет ссылку (нужно ручное переназначение).
ALTER TABLE subaccount_routing_assignment
    ADD CONSTRAINT fk_sra_route_set
    FOREIGN KEY (route_set_id) REFERENCES reseller_route_sets(id) ON DELETE SET NULL;
```

- [ ] **Step 3.2: Написать down-миграцию**

```sql
ALTER TABLE subaccount_routing_assignment DROP CONSTRAINT IF EXISTS fk_sra_route_set;
DROP INDEX IF EXISTS idx_client_routes_client_source;
ALTER TABLE client_routes DROP COLUMN IF EXISTS source;
```

- [ ] **Step 3.3: Применить + commit**

```
git add migrations/000136_client_routes_source_and_sra_fk.up.sql migrations/000136_client_routes_source_and_sra_fk.down.sql
git commit -m "feat(migrations): client_routes.source + sra.route_set_id FK (Plan 2 Task 3)"
git push origin master
./scripts/server.sh migrate
```

Verify:
```
./scripts/server.sh exec "psql -U smpp -d smpp_db -c \"\\d client_routes\"" | grep source
./scripts/server.sh exec "psql -U smpp -d smpp_db -c \"\\d subaccount_routing_assignment\"" | grep route_set
```
Expected: колонка `source character varying(10)` присутствует, FK на `reseller_route_sets` существует.

---

## Phase B — Storage layer

### Task 4: Repo `ResellerRouteSetRepository`

**Files:**
- Create: `internal/storage/reseller_route_set_repository.go`
- Create: `internal/storage/reseller_route_set_repository_test.go`

- [ ] **Step 4.1: Failing test для CRUD**

```go
package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestRouteSetRepo_CreateAndGet(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	repo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, resellerID, "Default routes", false)
	require.NoError(t, err)
	require.Equal(t, "Default routes", created.Name)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, resellerID, got.ResellerID)
}

func TestRouteSetRepo_DefaultUniqueness(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	repo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()
	_, err := repo.Create(ctx, resellerID, "A", true)
	require.NoError(t, err)
	_, err = repo.Create(ctx, resellerID, "B", true)
	require.Error(t, err)
}

func TestRouteSetRepo_ListByReseller(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	r1 := storagetest.SeedReseller(t, pool)
	r2 := storagetest.SeedReseller(t, pool)
	repo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()
	repo.Create(ctx, r1, "S1", false)
	repo.Create(ctx, r1, "S2", false)
	repo.Create(ctx, r2, "Other", false)
	list, err := repo.ListByReseller(ctx, r1)
	require.NoError(t, err); require.Len(t, list, 2)
}

func TestRouteSetRepo_UpdateAndDelete(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	repo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()
	s, _ := repo.Create(ctx, resellerID, "old", false)
	require.NoError(t, repo.Update(ctx, s.ID, "new", true))
	got, _ := repo.GetByID(ctx, s.ID)
	require.Equal(t, "new", got.Name); require.True(t, got.IsDefault)

	require.NoError(t, repo.Delete(ctx, s.ID))
	_, err := repo.GetByID(ctx, s.ID)
	require.ErrorIs(t, err, storage.ErrNotFound)
}
```

- [ ] **Step 4.2: Реализация**

`reseller_route_set_repository.go`:
```go
package storage

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ResellerRouteSet struct {
	ID         uuid.UUID
	ResellerID uuid.UUID
	Name       string
	IsDefault  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ResellerRouteSetRepository struct {
	pool *pgxpool.Pool
}

func NewResellerRouteSetRepository(pool *pgxpool.Pool) *ResellerRouteSetRepository {
	return &ResellerRouteSetRepository{pool: pool}
}

func (r *ResellerRouteSetRepository) Create(ctx context.Context, resellerID uuid.UUID, name string, isDefault bool) (*ResellerRouteSet, error) {
	out := &ResellerRouteSet{ResellerID: resellerID, Name: name, IsDefault: isDefault}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO reseller_route_sets (reseller_id, name, is_default)
		 VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`,
		resellerID, name, isDefault,
	).Scan(&out.ID, &out.CreatedAt, &out.UpdatedAt)
	if err != nil { return nil, err }
	return out, nil
}

func (r *ResellerRouteSetRepository) GetByID(ctx context.Context, id uuid.UUID) (*ResellerRouteSet, error) {
	var s ResellerRouteSet
	err := r.pool.QueryRow(ctx,
		`SELECT id, reseller_id, name, is_default, created_at, updated_at
		 FROM reseller_route_sets WHERE id = $1`, id,
	).Scan(&s.ID, &s.ResellerID, &s.Name, &s.IsDefault, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) { return nil, ErrNotFound }
	return &s, err
}

func (r *ResellerRouteSetRepository) ListByReseller(ctx context.Context, resellerID uuid.UUID) ([]ResellerRouteSet, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, reseller_id, name, is_default, created_at, updated_at
		 FROM reseller_route_sets WHERE reseller_id = $1 ORDER BY created_at DESC`,
		resellerID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []ResellerRouteSet
	for rows.Next() {
		var s ResellerRouteSet
		if err := rows.Scan(&s.ID, &s.ResellerID, &s.Name, &s.IsDefault, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ResellerRouteSetRepository) Update(ctx context.Context, id uuid.UUID, name string, isDefault bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE reseller_route_sets SET name=$1, is_default=$2, updated_at=now() WHERE id=$3`,
		name, isDefault, id)
	if err != nil { return err }
	if tag.RowsAffected() == 0 { return ErrNotFound }
	return nil
}

func (r *ResellerRouteSetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM reseller_route_sets WHERE id=$1`, id)
	if err != nil { return err }
	if tag.RowsAffected() == 0 { return ErrNotFound }
	return nil
}
```

- [ ] **Step 4.3: Run tests на сервере**

```
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test ./internal/storage/ -run TestRouteSetRepo -v"
```
Expected: ALL PASS.

- [ ] **Step 4.4: Commit**

```bash
git add internal/storage/reseller_route_set_repository.go internal/storage/reseller_route_set_repository_test.go
git commit -m "feat(storage): reseller_route_sets repository (Plan 2 Task 4)"
```

---

### Task 5: Repo `ResellerRouteSetItemsRepository` с условиями и расписаниями

**Files:**
- Create: `internal/storage/reseller_route_set_items_repository.go`
- Create: `internal/storage/reseller_route_set_items_repository_test.go`

Repo богаче: помимо CRUD на items, грузит/пишет condition_groups + conditions + schedules вложенно. Pattern: `LoadFullByItem` отдаёт полную структуру; `CreateFull` / `UpdateFull` пишут под транзакцией; `Reorder` — bulk-update приоритетов.

- [ ] **Step 5.1: Failing tests**

```go
package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestRouteSetItems_CreateFullAndLoad(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	setRepo := storage.NewResellerRouteSetRepository(pool)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	set, _ := setRepo.Create(ctx, resellerID, "Routes", false)

	in := storage.RouteSetItemFull{
		Name: "RU operators", ProviderID: provA, Priority: 10, Share: 100,
		RouteType: "sms", Status: "active",
		ConditionGroups: []storage.RouteSetConditionGroup{{
			LogicOp: "IF",
			Conditions: []storage.RouteSetCondition{
				{Type: "country", Value: "RU"},
				{Type: "traffic_type", Value: "transactional"},
			},
		}},
		Schedules: []storage.RouteSetSchedule{{Weekdays: 127, Timezone: "Europe/Moscow"}},
	}
	created, err := itemsRepo.CreateFull(ctx, set.ID, in)
	require.NoError(t, err)
	require.NotEqual(t, "", created.ID.String())

	loaded, err := itemsRepo.LoadFullByItem(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "RU operators", loaded.Name)
	require.Len(t, loaded.ConditionGroups, 1)
	require.Len(t, loaded.ConditionGroups[0].Conditions, 2)
	require.Len(t, loaded.Schedules, 1)
	require.EqualValues(t, 127, loaded.Schedules[0].Weekdays)
}

func TestRouteSetItems_ListBySet_OrderedByPriorityDesc(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	prov := storagetest.SeedProvider(t, pool, "P")
	setRepo := storage.NewResellerRouteSetRepository(pool)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	set, _ := setRepo.Create(ctx, resellerID, "S", false)
	itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{Name: "r1", ProviderID: prov, Priority: 10, Share: 100, RouteType: "sms", Status: "active"})
	itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{Name: "r2", ProviderID: prov, Priority: 5, Share: 100, RouteType: "sms", Status: "active"})
	list, err := itemsRepo.ListBySet(ctx, set.ID)
	require.NoError(t, err); require.Len(t, list, 2)
	require.Equal(t, "r1", list[0].Name)
}

func TestRouteSetItems_UpdateFull_ReplacesGroupsAndSchedules(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	prov := storagetest.SeedProvider(t, pool, "P")
	setRepo := storage.NewResellerRouteSetRepository(pool)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	set, _ := setRepo.Create(ctx, resellerID, "S", false)
	created, _ := itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{
		Name: "v1", ProviderID: prov, Priority: 10, Share: 100, RouteType: "sms", Status: "active",
		ConditionGroups: []storage.RouteSetConditionGroup{{LogicOp: "IF", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "RU"}}}},
	})
	require.NoError(t, itemsRepo.UpdateFull(ctx, created.ID, storage.RouteSetItemFull{
		Name: "v2", ProviderID: prov, Priority: 20, Share: 100, RouteType: "sms", Status: "active",
		ConditionGroups: []storage.RouteSetConditionGroup{{LogicOp: "IF", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "KZ"}}}},
	}))
	loaded, _ := itemsRepo.LoadFullByItem(ctx, created.ID)
	require.Equal(t, "v2", loaded.Name)
	require.EqualValues(t, 20, loaded.Priority)
	require.Equal(t, "KZ", loaded.ConditionGroups[0].Conditions[0].Value)
}

func TestRouteSetItems_Reorder(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	prov := storagetest.SeedProvider(t, pool, "P")
	setRepo := storage.NewResellerRouteSetRepository(pool)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	set, _ := setRepo.Create(ctx, resellerID, "S", false)
	a, _ := itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{Name: "a", ProviderID: prov, Priority: 10, Share: 100, RouteType: "sms", Status: "active"})
	b, _ := itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{Name: "b", ProviderID: prov, Priority: 20, Share: 100, RouteType: "sms", Status: "active"})
	require.NoError(t, itemsRepo.Reorder(ctx, set.ID, []storage.RouteSetReorderEntry{
		{ItemID: a.ID, Priority: 100},
		{ItemID: b.ID, Priority: 50},
	}))
	list, _ := itemsRepo.ListBySet(ctx, set.ID)
	require.Equal(t, "a", list[0].Name)
	require.EqualValues(t, 100, list[0].Priority)
}

func TestRouteSetItems_ProvidersInSet_Distinct(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")
	setRepo := storage.NewResellerRouteSetRepository(pool)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	set, _ := setRepo.Create(ctx, resellerID, "S", false)
	itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{ProviderID: provA, Share: 100, RouteType: "sms", Status: "active"})
	itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{ProviderID: provB, Share: 100, RouteType: "sms", Status: "active"})
	itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{ProviderID: provA, Share: 50, RouteType: "sms", Status: "active"})
	ids, err := itemsRepo.ProvidersInSet(ctx, set.ID)
	require.NoError(t, err)
	require.Len(t, ids, 2)
}
```

- [ ] **Step 5.2: Реализация**

`reseller_route_set_items_repository.go`:
```go
package storage

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RouteSetCondition struct {
	Type  string
	Value string
}

type RouteSetConditionGroup struct {
	GroupIndex int16
	LogicOp    string
	Conditions []RouteSetCondition
}

type RouteSetSchedule struct {
	DateFrom *time.Time
	DateTo   *time.Time
	TimeFrom *string
	TimeTo   *string
	Weekdays int16
	Timezone string
}

type RouteSetItemFull struct {
	ID              uuid.UUID
	SetID           uuid.UUID
	Name            string
	Comment         string
	ProviderID      uuid.UUID
	Priority        int
	Share           int
	RouteType       string
	Status          string
	ConditionGroups []RouteSetConditionGroup
	Schedules       []RouteSetSchedule
}

type RouteSetReorderEntry struct {
	ItemID   uuid.UUID
	Priority int
}

type ResellerRouteSetItemsRepository struct {
	pool *pgxpool.Pool
}

func NewResellerRouteSetItemsRepository(pool *pgxpool.Pool) *ResellerRouteSetItemsRepository {
	return &ResellerRouteSetItemsRepository{pool: pool}
}

func (r *ResellerRouteSetItemsRepository) ListBySet(ctx context.Context, setID uuid.UUID) ([]RouteSetItemFull, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, set_id, COALESCE(name,''), COALESCE(comment,''), provider_id,
		        priority, share, route_type, status
		 FROM reseller_route_set_items WHERE set_id = $1
		 ORDER BY priority DESC, name`, setID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []RouteSetItemFull
	for rows.Next() {
		var i RouteSetItemFull
		if err := rows.Scan(&i.ID, &i.SetID, &i.Name, &i.Comment, &i.ProviderID,
			&i.Priority, &i.Share, &i.RouteType, &i.Status); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil { return nil, err }
	for idx := range out {
		groups, err := r.loadGroups(ctx, out[idx].ID)
		if err != nil { return nil, err }
		out[idx].ConditionGroups = groups
		schedules, err := r.loadSchedules(ctx, out[idx].ID)
		if err != nil { return nil, err }
		out[idx].Schedules = schedules
	}
	return out, nil
}

func (r *ResellerRouteSetItemsRepository) LoadFullByItem(ctx context.Context, itemID uuid.UUID) (*RouteSetItemFull, error) {
	var i RouteSetItemFull
	err := r.pool.QueryRow(ctx,
		`SELECT id, set_id, COALESCE(name,''), COALESCE(comment,''), provider_id,
		        priority, share, route_type, status
		 FROM reseller_route_set_items WHERE id = $1`, itemID,
	).Scan(&i.ID, &i.SetID, &i.Name, &i.Comment, &i.ProviderID,
		&i.Priority, &i.Share, &i.RouteType, &i.Status)
	if errors.Is(err, pgx.ErrNoRows) { return nil, ErrNotFound }
	if err != nil { return nil, err }
	groups, err := r.loadGroups(ctx, itemID)
	if err != nil { return nil, err }
	i.ConditionGroups = groups
	schedules, err := r.loadSchedules(ctx, itemID)
	if err != nil { return nil, err }
	i.Schedules = schedules
	return &i, nil
}

func (r *ResellerRouteSetItemsRepository) loadGroups(ctx context.Context, itemID uuid.UUID) ([]RouteSetConditionGroup, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, group_index, logic_op FROM route_set_condition_groups
		 WHERE item_id = $1 ORDER BY group_index`, itemID)
	if err != nil { return nil, err }
	defer rows.Close()
	type row struct { ID int64; GroupIndex int16; LogicOp string }
	var rowsList []row
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.ID, &rr.GroupIndex, &rr.LogicOp); err != nil { return nil, err }
		rowsList = append(rowsList, rr)
	}
	if err := rows.Err(); err != nil { return nil, err }
	out := make([]RouteSetConditionGroup, 0, len(rowsList))
	for _, g := range rowsList {
		conds, err := r.loadConditions(ctx, g.ID)
		if err != nil { return nil, err }
		out = append(out, RouteSetConditionGroup{GroupIndex: g.GroupIndex, LogicOp: g.LogicOp, Conditions: conds})
	}
	return out, nil
}

func (r *ResellerRouteSetItemsRepository) loadConditions(ctx context.Context, groupID int64) ([]RouteSetCondition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT condition_type, condition_value FROM route_set_conditions
		 WHERE group_id = $1 ORDER BY id`, groupID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []RouteSetCondition
	for rows.Next() {
		var c RouteSetCondition
		if err := rows.Scan(&c.Type, &c.Value); err != nil { return nil, err }
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ResellerRouteSetItemsRepository) loadSchedules(ctx context.Context, itemID uuid.UUID) ([]RouteSetSchedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT date_from, date_to, time_from::text, time_to::text, weekdays, timezone
		 FROM route_set_schedules WHERE item_id = $1 ORDER BY id`, itemID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []RouteSetSchedule
	for rows.Next() {
		var s RouteSetSchedule
		if err := rows.Scan(&s.DateFrom, &s.DateTo, &s.TimeFrom, &s.TimeTo, &s.Weekdays, &s.Timezone); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ResellerRouteSetItemsRepository) CreateFull(ctx context.Context, setID uuid.UUID, in RouteSetItemFull) (*RouteSetItemFull, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil { return nil, err }
	defer tx.Rollback(ctx) //nolint:errcheck

	var id uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO reseller_route_set_items (set_id, name, comment, provider_id, priority, share, route_type, status)
		 VALUES ($1, NULLIF($2,''), NULLIF($3,''), $4, $5, $6, $7, $8) RETURNING id`,
		setID, in.Name, in.Comment, in.ProviderID, in.Priority, in.Share, in.RouteType, in.Status,
	).Scan(&id)
	if err != nil { return nil, err }
	if err := writeGroupsAndSchedules(ctx, tx, id, in.ConditionGroups, in.Schedules); err != nil { return nil, err }
	if err := tx.Commit(ctx); err != nil { return nil, err }
	in.ID = id
	in.SetID = setID
	return &in, nil
}

func (r *ResellerRouteSetItemsRepository) UpdateFull(ctx context.Context, itemID uuid.UUID, in RouteSetItemFull) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx) //nolint:errcheck
	tag, err := tx.Exec(ctx,
		`UPDATE reseller_route_set_items
		 SET name=NULLIF($1,''), comment=NULLIF($2,''), provider_id=$3,
		     priority=$4, share=$5, route_type=$6, status=$7, updated_at=now()
		 WHERE id=$8`,
		in.Name, in.Comment, in.ProviderID, in.Priority, in.Share, in.RouteType, in.Status, itemID)
	if err != nil { return err }
	if tag.RowsAffected() == 0 { return ErrNotFound }
	if _, err := tx.Exec(ctx, `DELETE FROM route_set_condition_groups WHERE item_id = $1`, itemID); err != nil { return err }
	if _, err := tx.Exec(ctx, `DELETE FROM route_set_schedules WHERE item_id = $1`, itemID); err != nil { return err }
	if err := writeGroupsAndSchedules(ctx, tx, itemID, in.ConditionGroups, in.Schedules); err != nil { return err }
	return tx.Commit(ctx)
}

func writeGroupsAndSchedules(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, groups []RouteSetConditionGroup, schedules []RouteSetSchedule) error {
	for idx, g := range groups {
		var gid int64
		err := tx.QueryRow(ctx,
			`INSERT INTO route_set_condition_groups (item_id, group_index, logic_op)
			 VALUES ($1, $2, $3) RETURNING id`,
			itemID, int16(idx), g.LogicOp,
		).Scan(&gid)
		if err != nil { return err }
		for _, c := range g.Conditions {
			if _, err := tx.Exec(ctx,
				`INSERT INTO route_set_conditions (group_id, condition_type, condition_value)
				 VALUES ($1, $2, $3)`,
				gid, c.Type, c.Value); err != nil {
				return err
			}
		}
	}
	for _, s := range schedules {
		if _, err := tx.Exec(ctx,
			`INSERT INTO route_set_schedules (item_id, date_from, date_to, time_from, time_to, weekdays, timezone)
			 VALUES ($1, $2, $3, $4::time, $5::time, $6, $7)`,
			itemID, s.DateFrom, s.DateTo, s.TimeFrom, s.TimeTo, s.Weekdays, s.Timezone); err != nil {
			return err
		}
	}
	return nil
}

func (r *ResellerRouteSetItemsRepository) Delete(ctx context.Context, itemID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM reseller_route_set_items WHERE id = $1`, itemID)
	if err != nil { return err }
	if tag.RowsAffected() == 0 { return ErrNotFound }
	return nil
}

func (r *ResellerRouteSetItemsRepository) Reorder(ctx context.Context, setID uuid.UUID, entries []RouteSetReorderEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx) //nolint:errcheck
	for _, e := range entries {
		if _, err := tx.Exec(ctx,
			`UPDATE reseller_route_set_items SET priority=$1, updated_at=now()
			 WHERE id=$2 AND set_id=$3`,
			e.Priority, e.ItemID, setID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *ResellerRouteSetItemsRepository) ProvidersInSet(ctx context.Context, setID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT provider_id FROM reseller_route_set_items WHERE set_id = $1`, setID)
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

- [ ] **Step 5.3: Run tests**

```
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test ./internal/storage/ -run TestRouteSetItems -v"
```
Expected: ALL PASS.

- [ ] **Step 5.4: Commit**

```bash
git add internal/storage/reseller_route_set_items_repository.go internal/storage/reseller_route_set_items_repository_test.go
git commit -m "feat(storage): reseller_route_set_items repository (Plan 2 Task 5)"
```

---

### Task 6: Расширить storagetest fixtures: SeedRouteSet + SeedRouteSetItem

**Files:**
- Modify: `internal/storage/storagetest/fixtures.go`

- [ ] **Step 6.1: Добавить функции в `fixtures.go` после `SeedProviderPrivate`**

```go
// SeedRouteSet вставляет минимальный route-set агрегатора и регистрирует cleanup.
func SeedRouteSet(t *testing.T, pool *pgxpool.Pool, resellerID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO reseller_route_sets (id, reseller_id, name, is_default)
		 VALUES ($1, $2, $3, false)`,
		id, resellerID, name,
	)
	require.NoError(t, err, "SeedRouteSet INSERT failed")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM reseller_route_sets WHERE id = $1`, id)
	})
	return id
}

// SeedRouteSetItem вставляет item с одной группой условий (logic_op='IF').
// conditions: список (type, value); если пуст — item без условий (always-match).
func SeedRouteSetItem(t *testing.T, pool *pgxpool.Pool, setID, providerID uuid.UUID, priority int, conditions [][2]string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck

	itemID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO reseller_route_set_items (id, set_id, name, provider_id, priority, share, route_type, status)
		 VALUES ($1, $2, 'test-item', $3, $4, 100, 'sms', 'active')`,
		itemID, setID, providerID, priority,
	)
	require.NoError(t, err)
	if len(conditions) > 0 {
		var groupID int64
		err = tx.QueryRow(ctx,
			`INSERT INTO route_set_condition_groups (item_id, group_index, logic_op)
			 VALUES ($1, 0, 'IF') RETURNING id`,
			itemID,
		).Scan(&groupID)
		require.NoError(t, err)
		for _, c := range conditions {
			_, err = tx.Exec(ctx,
				`INSERT INTO route_set_conditions (group_id, condition_type, condition_value)
				 VALUES ($1, $2, $3)`, groupID, c[0], c[1])
			require.NoError(t, err)
		}
	}
	require.NoError(t, tx.Commit(ctx))
	// cleanup идёт через CASCADE FK при удалении reseller_route_sets в SeedRouteSet
	return itemID
}
```

- [ ] **Step 6.2: Build check**

```
./scripts/server.sh exec "cd /opt/sms && go build ./internal/storage/storagetest/..."
```

- [ ] **Step 6.3: Commit**

```bash
git add internal/storage/storagetest/fixtures.go
git commit -m "feat(storagetest): SeedRouteSet + SeedRouteSetItem (Plan 2 Task 6)"
```

---

## Phase C — Service layer

### Task 7: `RouteSetMaterializer` — материализация route-set в client_routes

**Files:**
- Create: `internal/services/network/route_set_materializer.go`
- Create: `internal/services/network/route_set_materializer_test.go`

Семантика по аналогии с `ProviderSetMaterializer`:
1. BEGIN
2. DELETE `client_routes WHERE client_id=X AND source='template'` (CASCADE на `route_condition_groups` / `route_conditions` / `route_schedules` через FK)
3. Если `routeSetID != nil` — INSERT новых из `reseller_route_set_items` с `source='template'`, копируя condition_groups + conditions + schedules
4. COMMIT

`source='override'` записи не затрагиваются.

- [ ] **Step 7.1: Failing tests**

```go
package network_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestRouteMaterializer_Apply_CreatesTemplateRows(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")

	setID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, setID, provA, 10, [][2]string{{"country", "RU"}})

	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, itemsRepo)
	require.NoError(t, mat.ApplyToClient(context.Background(), subID, &setID))

	var routesCount, groupsCount, condsCount int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`,
		subID).Scan(&routesCount)
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM route_condition_groups rcg
		 JOIN client_routes cr ON cr.id = rcg.route_id
		 WHERE cr.client_id=$1`, subID).Scan(&groupsCount)
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM route_conditions rc
		 JOIN route_condition_groups rcg ON rcg.id = rc.group_id
		 JOIN client_routes cr ON cr.id = rcg.route_id
		 WHERE cr.client_id=$1`, subID).Scan(&condsCount)
	require.Equal(t, 1, routesCount)
	require.Equal(t, 1, groupsCount)
	require.Equal(t, 1, condsCount)
}

func TestRouteMaterializer_Apply_PreservesOverrides(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")

	// Manually insert an override route
	_, err := pool.Exec(context.Background(),
		`INSERT INTO client_routes (client_id, provider_id, priority, weight, active, name, status, share, route_type, source)
		 VALUES ($1, $2, 99, 1, true, 'override-route', 'active', 100, 'sms', 'override')`,
		subID, provB)
	require.NoError(t, err)

	setID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, setID, provA, 10, nil)

	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, itemsRepo)
	require.NoError(t, mat.ApplyToClient(context.Background(), subID, &setID))

	var template, override int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`, subID).Scan(&template)
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='override'`, subID).Scan(&override)
	require.Equal(t, 1, template)
	require.Equal(t, 1, override)
}

func TestRouteMaterializer_Apply_NilSet_ClearsTemplate(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	setID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, setID, provA, 10, nil)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, itemsRepo)
	require.NoError(t, mat.ApplyToClient(context.Background(), subID, &setID))

	require.NoError(t, mat.ApplyToClient(context.Background(), subID, nil))

	var count int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`, subID).Scan(&count)
	require.Equal(t, 0, count)
}

func TestRouteMaterializer_ApplyToAllSubscribers(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	sub1 := storagetest.SeedSubAccount(t, pool, resellerID)
	sub2 := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	setID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, setID, provA, 10, nil)

	// Subscribe both
	for _, c := range []string{sub1.String(), sub2.String()} {
		_, err := pool.Exec(context.Background(),
			`INSERT INTO subaccount_routing_assignment (client_id, route_set_id) VALUES ($1, $2)
			 ON CONFLICT (client_id) DO UPDATE SET route_set_id = EXCLUDED.route_set_id`,
			c, setID)
		require.NoError(t, err)
	}

	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, itemsRepo)
	require.NoError(t, mat.ApplyToAllSubscribers(context.Background(), setID))

	var c1, c2 int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`, sub1).Scan(&c1)
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`, sub2).Scan(&c2)
	require.Equal(t, 1, c1)
	require.Equal(t, 1, c2)
}
```

- [ ] **Step 7.2: Реализация**

`route_set_materializer.go`:
```go
package network

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/storage"
)

// RouteSetMaterializer материализует route-set агрегатора в client_routes суб-аккаунта.
// При применении: записи source='template' удаляются (CASCADE на условия/расписания) и пересоздаются.
// Записи source='override' не затрагиваются.
type RouteSetMaterializer struct {
	pool      *pgxpool.Pool
	itemsRepo *storage.ResellerRouteSetItemsRepository
}

func NewRouteSetMaterializer(pool *pgxpool.Pool, itemsRepo *storage.ResellerRouteSetItemsRepository) *RouteSetMaterializer {
	return &RouteSetMaterializer{pool: pool, itemsRepo: itemsRepo}
}

// ApplyToClient — материализует route-set в client_routes под транзакцией.
// routeSetID == nil → стереть только source='template' (без вставки новых).
// Записи source='override' не затрагиваются в обоих случаях.
func (m *RouteSetMaterializer) ApplyToClient(ctx context.Context, clientID uuid.UUID, routeSetID *uuid.UUID) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx) //nolint:errcheck

	// Шаг 1: удалить template-записи. CASCADE на route_condition_groups + route_conditions + route_schedules.
	if _, err := tx.Exec(ctx,
		`DELETE FROM client_routes WHERE client_id = $1 AND source = 'template'`,
		clientID,
	); err != nil {
		return err
	}

	// Шаг 2: если route-set указан — загрузить items + копировать в client_routes.
	if routeSetID != nil {
		items, err := m.loadItemsTx(ctx, tx, *routeSetID)
		if err != nil { return err }
		for _, item := range items {
			if err := insertRouteFromItem(ctx, tx, clientID, item); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

// loadItemsTx — загружает items + condition groups + schedules внутри одной транзакции.
// Дублирует логику ListBySet, но работает через tx (важно для read-after-write consistency).
func (m *RouteSetMaterializer) loadItemsTx(ctx context.Context, tx pgx.Tx, setID uuid.UUID) ([]storage.RouteSetItemFull, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, set_id, COALESCE(name,''), COALESCE(comment,''), provider_id,
		        priority, share, route_type, status
		 FROM reseller_route_set_items WHERE set_id = $1 ORDER BY priority DESC`, setID)
	if err != nil { return nil, err }
	var items []storage.RouteSetItemFull
	for rows.Next() {
		var i storage.RouteSetItemFull
		if err := rows.Scan(&i.ID, &i.SetID, &i.Name, &i.Comment, &i.ProviderID,
			&i.Priority, &i.Share, &i.RouteType, &i.Status); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, i)
	}
	rows.Close()
	if err := rows.Err(); err != nil { return nil, err }

	for idx := range items {
		groups, err := loadGroupsTx(ctx, tx, items[idx].ID)
		if err != nil { return nil, err }
		items[idx].ConditionGroups = groups
		schedules, err := loadSchedulesTx(ctx, tx, items[idx].ID)
		if err != nil { return nil, err }
		items[idx].Schedules = schedules
	}
	return items, nil
}

func loadGroupsTx(ctx context.Context, tx pgx.Tx, itemID uuid.UUID) ([]storage.RouteSetConditionGroup, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, group_index, logic_op FROM route_set_condition_groups
		 WHERE item_id = $1 ORDER BY group_index`, itemID)
	if err != nil { return nil, err }
	type row struct { ID int64; GroupIndex int16; LogicOp string }
	var rl []row
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.ID, &rr.GroupIndex, &rr.LogicOp); err != nil {
			rows.Close(); return nil, err
		}
		rl = append(rl, rr)
	}
	rows.Close()
	if err := rows.Err(); err != nil { return nil, err }
	out := make([]storage.RouteSetConditionGroup, 0, len(rl))
	for _, g := range rl {
		crows, err := tx.Query(ctx,
			`SELECT condition_type, condition_value FROM route_set_conditions
			 WHERE group_id = $1 ORDER BY id`, g.ID)
		if err != nil { return nil, err }
		var conds []storage.RouteSetCondition
		for crows.Next() {
			var c storage.RouteSetCondition
			if err := crows.Scan(&c.Type, &c.Value); err != nil {
				crows.Close(); return nil, err
			}
			conds = append(conds, c)
		}
		crows.Close()
		out = append(out, storage.RouteSetConditionGroup{GroupIndex: g.GroupIndex, LogicOp: g.LogicOp, Conditions: conds})
	}
	return out, nil
}

func loadSchedulesTx(ctx context.Context, tx pgx.Tx, itemID uuid.UUID) ([]storage.RouteSetSchedule, error) {
	rows, err := tx.Query(ctx,
		`SELECT date_from, date_to, time_from::text, time_to::text, weekdays, timezone
		 FROM route_set_schedules WHERE item_id = $1 ORDER BY id`, itemID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []storage.RouteSetSchedule
	for rows.Next() {
		var s storage.RouteSetSchedule
		if err := rows.Scan(&s.DateFrom, &s.DateTo, &s.TimeFrom, &s.TimeTo, &s.Weekdays, &s.Timezone); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// insertRouteFromItem — копирует один item в client_routes + condition_groups + conditions + schedules.
// client_routes.operator_id оставлен NULL (миграция 080 разрешает NULL); матчинг идёт через condition_groups.
func insertRouteFromItem(ctx context.Context, tx pgx.Tx, clientID uuid.UUID, item storage.RouteSetItemFull) error {
	var routeID uuid.UUID
	err := tx.QueryRow(ctx,
		`INSERT INTO client_routes
			(client_id, operator_id, provider_id, priority, weight, active,
			 name, comment, status, share, route_type, source)
		 VALUES ($1, NULL, $2, $3, 1, true, NULLIF($4,''), NULLIF($5,''), $6, $7, $8, 'template')
		 RETURNING id`,
		clientID, item.ProviderID, item.Priority, item.Name, item.Comment,
		item.Status, item.Share, item.RouteType,
	).Scan(&routeID)
	if err != nil { return err }

	for idx, g := range item.ConditionGroups {
		var gid int64
		err := tx.QueryRow(ctx,
			`INSERT INTO route_condition_groups (route_id, group_index, logic_op)
			 VALUES ($1, $2, $3) RETURNING id`,
			routeID, int16(idx), g.LogicOp,
		).Scan(&gid)
		if err != nil { return err }
		for _, c := range g.Conditions {
			if _, err := tx.Exec(ctx,
				`INSERT INTO route_conditions (group_id, condition_type, condition_value)
				 VALUES ($1, $2, $3)`, gid, c.Type, c.Value); err != nil {
				return err
			}
		}
	}
	for _, s := range item.Schedules {
		if _, err := tx.Exec(ctx,
			`INSERT INTO route_schedules (route_id, date_from, date_to, time_from, time_to, weekdays, timezone)
			 VALUES ($1, $2, $3, $4::time, $5::time, $6, $7)`,
			routeID, s.DateFrom, s.DateTo, s.TimeFrom, s.TimeTo, s.Weekdays, s.Timezone); err != nil {
			return err
		}
	}
	return nil
}

// ApplyToAllSubscribers — пересчитать материализацию для всех подписанных на route-set.
// Per-client транзакции (как в ProviderSetMaterializer); partial failure возможен.
func (m *RouteSetMaterializer) ApplyToAllSubscribers(ctx context.Context, routeSetID uuid.UUID) error {
	rows, err := m.pool.Query(ctx,
		`SELECT client_id FROM subaccount_routing_assignment WHERE route_set_id = $1`,
		routeSetID,
	)
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
		if err := m.ApplyToClient(ctx, cid, &routeSetID); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 7.3: Run tests**

```
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test ./internal/services/network/ -run TestRouteMaterializer -v"
```
Expected: ALL PASS.

- [ ] **Step 7.4: Commit**

```bash
git add internal/services/network/route_set_materializer.go internal/services/network/route_set_materializer_test.go
git commit -m "feat(network): RouteSetMaterializer — материализация в client_routes (Plan 2 Task 7)"
```

---

### Task 8: Validator провайдер-конфликтов

**Files:**
- Create: `internal/services/network/conflict_validator.go`
- Create: `internal/services/network/conflict_validator_test.go`

Один сервис, который отвечает на вопрос: "Если суб-аккаунту X назначить provider-set P и route-set R — какие провайдеры в правилах R отсутствуют в P?". Используется:
- При POST/PUT route-set item (для каждого подписанного клиента — проверить совместимость)
- При PUT assignment (новая пара set'ов)
- Bulk dry-run

- [ ] **Step 8.1: Failing tests**

```go
package network_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestConflictValidator_NoConflict(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")

	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	rsRepo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()
	ps, _ := psRepo.Create(ctx, resellerID, "PS", false)
	psItems.ReplaceItems(ctx, ps.ID, []storage.ProviderSetItemInput{{ProviderID: provA, Priority: 0, ExposeProviderName: true}})
	rs, _ := rsRepo.Create(ctx, resellerID, "RS", false)
	storagetest.SeedRouteSetItem(t, pool, rs.ID, provA, 10, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	v := network.NewConflictValidator(psItems, rsItems)
	missing, err := v.MissingProviders(ctx, ps.ID, rs.ID)
	require.NoError(t, err)
	require.Empty(t, missing)
}

func TestConflictValidator_ProviderMissingFromSet(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")
	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	rsRepo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()
	ps, _ := psRepo.Create(ctx, resellerID, "PS", false)
	psItems.ReplaceItems(ctx, ps.ID, []storage.ProviderSetItemInput{{ProviderID: provA, Priority: 0, ExposeProviderName: true}})
	rs, _ := rsRepo.Create(ctx, resellerID, "RS", false)
	storagetest.SeedRouteSetItem(t, pool, rs.ID, provB, 10, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	v := network.NewConflictValidator(psItems, rsItems)
	missing, err := v.MissingProviders(ctx, ps.ID, rs.ID)
	require.NoError(t, err)
	require.Len(t, missing, 1)
	require.Equal(t, provB, missing[0])
}

func TestConflictValidator_NilRouteSet_NoConflict(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	ps, _ := psRepo.Create(ctx, resellerID, "PS", false)

	v := network.NewConflictValidator(psItems, rsItems)
	// nil route-set — конфликта не существует определению
	missing, err := v.MissingProvidersByIDs(ctx, &ps.ID, nil)
	require.NoError(t, err)
	require.Empty(t, missing)
}
```

- [ ] **Step 8.2: Реализация**

`conflict_validator.go`:
```go
package network

import (
	"context"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/storage"
)

// ConflictValidator проверяет провайдер-инвариант (spec §6.1):
// каждый provider_id в правилах route-set должен быть в provider-set.
type ConflictValidator struct {
	psItems *storage.ResellerProviderSetItemsRepository
	rsItems *storage.ResellerRouteSetItemsRepository
}

func NewConflictValidator(ps *storage.ResellerProviderSetItemsRepository, rs *storage.ResellerRouteSetItemsRepository) *ConflictValidator {
	return &ConflictValidator{psItems: ps, rsItems: rs}
}

// MissingProviders — provider_id'ы, упомянутые в route-set, но отсутствующие в provider-set.
// Если provider-set пустой — все провайдеры route-set считаются missing.
func (v *ConflictValidator) MissingProviders(ctx context.Context, providerSetID, routeSetID uuid.UUID) ([]uuid.UUID, error) {
	psIDs, err := v.psItems.ListProvidersInSet(ctx, providerSetID)
	if err != nil { return nil, err }
	rsIDs, err := v.rsItems.ProvidersInSet(ctx, routeSetID)
	if err != nil { return nil, err }

	psSet := make(map[uuid.UUID]struct{}, len(psIDs))
	for _, id := range psIDs { psSet[id] = struct{}{} }
	var missing []uuid.UUID
	for _, id := range rsIDs {
		if _, ok := psSet[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

// MissingProvidersByIDs — то же, но обрабатывает nil-указатели.
// nil route-set → пустой результат (нечего проверять).
// nil provider-set + не-nil route-set → все провайдеры route-set'а — missing.
func (v *ConflictValidator) MissingProvidersByIDs(ctx context.Context, providerSetID, routeSetID *uuid.UUID) ([]uuid.UUID, error) {
	if routeSetID == nil {
		return nil, nil
	}
	rsIDs, err := v.rsItems.ProvidersInSet(ctx, *routeSetID)
	if err != nil { return nil, err }
	if providerSetID == nil {
		return rsIDs, nil
	}
	return v.MissingProviders(ctx, *providerSetID, *routeSetID)
}
```

- [ ] **Step 8.3: Run tests**

```
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test ./internal/services/network/ -run TestConflictValidator -v"
```
Expected: ALL PASS.

- [ ] **Step 8.4: Commit**

```bash
git add internal/services/network/conflict_validator.go internal/services/network/conflict_validator_test.go
git commit -m "feat(network): ConflictValidator — provider-set ↔ route-set invariant (Plan 2 Task 8)"
```

---

## Phase D — Backend handlers

### Task 9: Handler `/reseller/network/route-sets` (CRUD set'ов)

**Files:**
- Create: `internal/gateway/portal/handlers/network_route_sets.go`
- Create: `internal/gateway/portal/handlers/network_route_sets_test.go`

Зеркало `network_provider_sets.go` с заменой типов. Тот же verifyOwnership-паттерн (404 для чужого set'а), тот же 409 при assigned. Используй [network_provider_sets.go](../../internal/gateway/portal/handlers/network_provider_sets.go) как образец.

- [ ] **Step 9.1: Failing tests** (CRUD + delete-409 + ownership-404)

```go
func TestRouteSets_CRUD(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	h := NewNetworkRouteSetsHandlers(pool, nil) // materializer не нужен для CRUD set'а

	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/route-sets",
		strings.NewReader(`{"name":"RS1","is_default":false}`))
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Create(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	var created struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &created)

	w = httptest.NewRecorder()
	h.List(w, httptest.NewRequest("GET", "/portal/v1/reseller/network/route-sets", nil).
		WithContext(middleware.WithClientID(context.Background(), resellerID)))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRouteSets_Delete_409IfAssigned(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	pool.Exec(context.Background(),
		`INSERT INTO subaccount_routing_assignment (client_id, route_set_id) VALUES ($1, $2)
		 ON CONFLICT (client_id) DO UPDATE SET route_set_id=EXCLUDED.route_set_id`,
		subID, rsID)

	h := NewNetworkRouteSetsHandlers(pool, nil)
	req := httptest.NewRequest("DELETE", "/portal/v1/reseller/network/route-sets/"+rsID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
}

func TestRouteSets_OwnershipCheck_404(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerA := storagetest.SeedReseller(t, pool)
	resellerB := storagetest.SeedReseller(t, pool)
	rsA := storagetest.SeedRouteSet(t, pool, resellerA, "A")

	h := NewNetworkRouteSetsHandlers(pool, nil)
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/route-sets/"+rsA.String(),
		strings.NewReader(`{"name":"hijack","is_default":false}`))
	req = mux.SetURLVars(req, map[string]string{"id": rsA.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerB))
	w := httptest.NewRecorder()
	h.Update(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}
```

- [ ] **Step 9.2: Реализация**

`network_route_sets.go`:
```go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type NetworkRouteSetsHandlers struct {
	pool         *pgxpool.Pool
	setRepo      *storage.ResellerRouteSetRepository
	materializer *network.RouteSetMaterializer
}

func NewNetworkRouteSetsHandlers(pool *pgxpool.Pool, mat *network.RouteSetMaterializer) *NetworkRouteSetsHandlers {
	return &NetworkRouteSetsHandlers{
		pool:         pool,
		setRepo:      storage.NewResellerRouteSetRepository(pool),
		materializer: mat,
	}
}

func (h *NetworkRouteSetsHandlers) reseller(r *http.Request) (uuid.UUID, error) {
	cid, ok := middleware.GetClientID(r.Context())
	if !ok { return uuid.Nil, shared.ErrUnauthorized("не авторизован") }
	return cid, nil
}

func (h *NetworkRouteSetsHandlers) verifyOwnership(ctx context.Context, resellerID, setID uuid.UUID) error {
	set, err := h.setRepo.GetByID(ctx, setID)
	if err != nil { return shared.ErrNotFound("route-set") }
	if set.ResellerID != resellerID { return shared.ErrNotFound("route-set") }
	return nil
}

type routeSetOut struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsDefault     bool   `json:"is_default"`
	ItemCount     int    `json:"item_count"`
	AssignedCount int    `json:"assigned_count"`
}

func (h *NetworkRouteSetsHandlers) List(w http.ResponseWriter, r *http.Request) {
	resellerID, err := h.reseller(r)
	if err != nil { respondError(w, err); return }
	rows, err := h.pool.Query(r.Context(), `
		SELECT s.id, s.name, s.is_default,
		   (SELECT count(*) FROM reseller_route_set_items WHERE set_id = s.id),
		   (SELECT count(*) FROM subaccount_routing_assignment WHERE route_set_id = s.id)
		FROM reseller_route_sets s WHERE s.reseller_id = $1 ORDER BY s.created_at DESC`, resellerID)
	if err != nil { respondError(w, shared.ErrInternalServer("list route-sets")); return }
	defer rows.Close()
	out := []routeSetOut{}
	for rows.Next() {
		var p routeSetOut
		if err := rows.Scan(&p.ID, &p.Name, &p.IsDefault, &p.ItemCount, &p.AssignedCount); err != nil {
			respondError(w, shared.ErrInternalServer("scan")); return
		}
		out = append(out, p)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"route_sets": out})
}

func (h *NetworkRouteSetsHandlers) Create(w http.ResponseWriter, r *http.Request) {
	resellerID, err := h.reseller(r)
	if err != nil { respondError(w, err); return }
	var body struct { Name string `json:"name"`; IsDefault bool `json:"is_default"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	if body.Name == "" { respondError(w, shared.ErrInvalidInput("name обязательно")); return }

	tx, err := h.pool.Begin(r.Context())
	if err != nil { respondError(w, shared.ErrInternalServer("tx")); return }
	defer tx.Rollback(r.Context()) //nolint:errcheck
	if body.IsDefault {
		if _, err := tx.Exec(r.Context(),
			`UPDATE reseller_route_sets SET is_default=false WHERE reseller_id=$1 AND is_default=true`,
			resellerID); err != nil {
			respondError(w, shared.ErrInternalServer("clear default")); return
		}
	}
	var id uuid.UUID
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO reseller_route_sets (reseller_id, name, is_default) VALUES ($1,$2,$3) RETURNING id`,
		resellerID, body.Name, body.IsDefault,
	).Scan(&id); err != nil {
		// 23505 unique violation — конфликт имени
		respondError(w, shared.ErrConflict("route-set с таким именем уже существует")); return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("commit")); return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id.String(), "name": body.Name, "is_default": body.IsDefault})
}

func (h *NetworkRouteSetsHandlers) Update(w http.ResponseWriter, r *http.Request) {
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
	defer tx.Rollback(r.Context()) //nolint:errcheck
	if body.IsDefault {
		tx.Exec(r.Context(),
			`UPDATE reseller_route_sets SET is_default=false WHERE reseller_id=$1 AND is_default=true AND id<>$2`,
			resellerID, setID)
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE reseller_route_sets SET name=$1, is_default=$2, updated_at=now() WHERE id=$3`,
		body.Name, body.IsDefault, setID); err != nil {
		respondError(w, shared.ErrConflict("route-set с таким именем уже существует")); return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("commit")); return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": setID.String()})
}

func (h *NetworkRouteSetsHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	resellerID, err := h.reseller(r)
	if err != nil { respondError(w, err); return }
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verifyOwnership(r.Context(), resellerID, setID); err != nil {
		respondError(w, err); return
	}
	var assigned int
	h.pool.QueryRow(r.Context(),
		`SELECT count(*) FROM subaccount_routing_assignment WHERE route_set_id = $1`, setID,
	).Scan(&assigned)
	if assigned > 0 {
		respondError(w, shared.ErrConflict("route-set назначен суб-аккаунтам").
			WithDetails(fmt.Sprintf(`{"kind":"route_set_assigned","count":%d}`, assigned)))
		return
	}
	if _, err := h.pool.Exec(r.Context(),
		`DELETE FROM reseller_route_sets WHERE id = $1`, setID); err != nil {
		respondError(w, shared.ErrInternalServer("delete")); return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 9.3: Run tests + commit**

```
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test ./internal/gateway/portal/handlers/ -run TestRouteSets -v"
git add internal/gateway/portal/handlers/network_route_sets.go internal/gateway/portal/handlers/network_route_sets_test.go
git commit -m "feat(handlers): /reseller/network/route-sets CRUD (Plan 2 Task 9)"
```

---

### Task 10: Handler items route-set'ов (CRUD + duplicate + reorder)

**Files:**
- Create: `internal/gateway/portal/handlers/network_route_set_items.go`
- Create: `internal/gateway/portal/handlers/network_route_set_items_test.go`

Endpoints:
- GET `/reseller/network/route-sets/{id}/items` — список items с условиями + расписаниями
- POST `/reseller/network/route-sets/{id}/items` — создать item; pre-validate provider_id ∈ provider-set каждого подписанного; на конфликт 409 с details `kind: route_uses_unavailable_provider`
- PUT `/reseller/network/route-sets/{id}/items/{item_id}` — full replace (конкретного item целиком), та же валидация
- DELETE `/reseller/network/route-sets/{id}/items/{item_id}`
- POST `/reseller/network/route-sets/{id}/items/{item_id}/duplicate` — копия с инкрементом priority
- PUT `/reseller/network/route-sets/{id}/items/reorder` — bulk-update приоритетов

После любой mutating-операции — `materializer.ApplyToAllSubscribers(setID)`.

- [ ] **Step 10.1: Failing tests** (минимум 5 — list, create, create-409, reorder, duplicate)

```go
func TestRouteSetItems_CreateAndList(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, rsItems)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkRouteSetItemsHandlers(pool, mat, validator)

	body := fmt.Sprintf(`{
		"name":"r1","provider_id":"%s","priority":10,"share":100,"route_type":"sms","status":"active",
		"condition_groups":[{"logic_op":"IF","conditions":[{"type":"country","value":"RU"}]}],
		"schedules":[{"weekdays":127,"timezone":"Europe/Moscow"}]
	}`, provA.String())
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/route-sets/"+rsID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Create(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	w = httptest.NewRecorder()
	listReq := httptest.NewRequest("GET", "/portal/v1/reseller/network/route-sets/"+rsID.String()+"/items", nil)
	listReq = mux.SetURLVars(listReq, map[string]string{"id": rsID.String()})
	listReq = listReq.WithContext(middleware.WithClientID(listReq.Context(), resellerID))
	h.List(w, listReq)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct{ Items []map[string]interface{} `json:"items"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	require.Len(t, resp.Items, 1)
}

func TestRouteSetItems_Create_409_ProviderNotInSet(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")

	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	psID, _ := psRepo.Create(context.Background(), resellerID, "PS", false)
	psItems.ReplaceItems(context.Background(), psID.ID, []storage.ProviderSetItemInput{{ProviderID: provA, ExposeProviderName: true}})
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	pool.Exec(context.Background(),
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id) VALUES ($1, $2, $3)
		 ON CONFLICT (client_id) DO UPDATE SET provider_set_id=EXCLUDED.provider_set_id, route_set_id=EXCLUDED.route_set_id`,
		subID, psID.ID, rsID)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, rsItems)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkRouteSetItemsHandlers(pool, mat, validator)
	body := fmt.Sprintf(`{"name":"x","provider_id":"%s","priority":1,"share":100,"route_type":"sms","status":"active"}`, provB.String())
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/route-sets/"+rsID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Create(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), "route_uses_unavailable_provider")
}

func TestRouteSetItems_Reorder(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	itemA := storagetest.SeedRouteSetItem(t, pool, rsID, provA, 1, nil)
	itemB := storagetest.SeedRouteSetItem(t, pool, rsID, provA, 2, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, rsItems)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkRouteSetItemsHandlers(pool, mat, validator)

	body := fmt.Sprintf(`{"items":[{"item_id":"%s","priority":100},{"item_id":"%s","priority":50}]}`, itemA.String(), itemB.String())
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/route-sets/"+rsID.String()+"/items/reorder", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Reorder(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRouteSetItems_Duplicate(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	src := storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10, [][2]string{{"country", "RU"}})

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, rsItems)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkRouteSetItemsHandlers(pool, mat, validator)

	req := httptest.NewRequest("POST",
		fmt.Sprintf("/portal/v1/reseller/network/route-sets/%s/items/%s/duplicate", rsID.String(), src.String()),
		nil)
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String(), "item_id": src.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Duplicate(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var count int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM reseller_route_set_items WHERE set_id=$1`, rsID).Scan(&count)
	require.Equal(t, 2, count)
}
```

- [ ] **Step 10.2: Реализация**

`network_route_set_items.go`:
```go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type NetworkRouteSetItemsHandlers struct {
	pool         *pgxpool.Pool
	setRepo      *storage.ResellerRouteSetRepository
	itemsRepo    *storage.ResellerRouteSetItemsRepository
	materializer *network.RouteSetMaterializer
	validator    *network.ConflictValidator
}

func NewNetworkRouteSetItemsHandlers(pool *pgxpool.Pool, mat *network.RouteSetMaterializer, validator *network.ConflictValidator) *NetworkRouteSetItemsHandlers {
	return &NetworkRouteSetItemsHandlers{
		pool: pool,
		setRepo: storage.NewResellerRouteSetRepository(pool),
		itemsRepo: storage.NewResellerRouteSetItemsRepository(pool),
		materializer: mat,
		validator: validator,
	}
}

type conditionIn struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
type groupIn struct {
	LogicOp    string        `json:"logic_op"`
	Conditions []conditionIn `json:"conditions"`
}
type scheduleIn struct {
	DateFrom *string `json:"date_from"` // YYYY-MM-DD
	DateTo   *string `json:"date_to"`
	TimeFrom *string `json:"time_from"` // HH:MM или HH:MM:SS
	TimeTo   *string `json:"time_to"`
	Weekdays int16   `json:"weekdays"`
	Timezone string  `json:"timezone"`
}
type itemIn struct {
	Name            string       `json:"name"`
	Comment         string       `json:"comment"`
	ProviderID      string       `json:"provider_id"`
	Priority        int          `json:"priority"`
	Share           int          `json:"share"`
	RouteType       string       `json:"route_type"`
	Status          string       `json:"status"`
	ConditionGroups []groupIn    `json:"condition_groups"`
	Schedules       []scheduleIn `json:"schedules"`
}

type itemOut struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Comment         string                 `json:"comment"`
	ProviderID      string                 `json:"provider_id"`
	ProviderName    string                 `json:"provider_name"`
	Priority        int                    `json:"priority"`
	Share           int                    `json:"share"`
	RouteType       string                 `json:"route_type"`
	Status          string                 `json:"status"`
	ConditionGroups []map[string]interface{} `json:"condition_groups"`
	Schedules       []map[string]interface{} `json:"schedules"`
}

func (h *NetworkRouteSetItemsHandlers) verify(ctx context.Context, resellerID, setID uuid.UUID) error {
	set, err := h.setRepo.GetByID(ctx, setID)
	if err != nil { return shared.ErrNotFound("route-set") }
	if set.ResellerID != resellerID { return shared.ErrNotFound("route-set") }
	return nil
}

func (h *NetworkRouteSetItemsHandlers) List(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verify(r.Context(), resellerID, setID); err != nil { respondError(w, err); return }

	items, err := h.itemsRepo.ListBySet(r.Context(), setID)
	if err != nil { respondError(w, shared.ErrInternalServer("list")); return }

	// Provider names lookup
	providerNames := map[uuid.UUID]string{}
	if len(items) > 0 {
		ids := make([]uuid.UUID, 0, len(items))
		for _, it := range items { ids = append(ids, it.ProviderID) }
		rows, err := h.pool.Query(r.Context(), `SELECT id, name FROM providers WHERE id = ANY($1)`, ids)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id uuid.UUID; var name string
				rows.Scan(&id, &name)
				providerNames[id] = name
			}
		}
	}

	out := make([]itemOut, 0, len(items))
	for _, it := range items {
		out = append(out, itemOut{
			ID: it.ID.String(), Name: it.Name, Comment: it.Comment,
			ProviderID: it.ProviderID.String(), ProviderName: providerNames[it.ProviderID],
			Priority: it.Priority, Share: it.Share, RouteType: it.RouteType, Status: it.Status,
			ConditionGroups: groupsOut(it.ConditionGroups),
			Schedules: schedulesOut(it.Schedules),
		})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"items": out})
}

func groupsOut(groups []storage.RouteSetConditionGroup) []map[string]interface{} {
	out := []map[string]interface{}{}
	for _, g := range groups {
		conds := []map[string]string{}
		for _, c := range g.Conditions {
			conds = append(conds, map[string]string{"type": c.Type, "value": c.Value})
		}
		out = append(out, map[string]interface{}{"logic_op": g.LogicOp, "conditions": conds})
	}
	return out
}

func schedulesOut(scheds []storage.RouteSetSchedule) []map[string]interface{} {
	out := []map[string]interface{}{}
	for _, s := range scheds {
		entry := map[string]interface{}{"weekdays": s.Weekdays, "timezone": s.Timezone}
		if s.DateFrom != nil { entry["date_from"] = s.DateFrom.Format("2006-01-02") }
		if s.DateTo != nil { entry["date_to"] = s.DateTo.Format("2006-01-02") }
		if s.TimeFrom != nil { entry["time_from"] = *s.TimeFrom }
		if s.TimeTo != nil { entry["time_to"] = *s.TimeTo }
		out = append(out, entry)
	}
	return out
}

func parseItemIn(in itemIn) (storage.RouteSetItemFull, error) {
	provID, err := uuid.Parse(in.ProviderID)
	if err != nil { return storage.RouteSetItemFull{}, fmt.Errorf("provider_id") }
	out := storage.RouteSetItemFull{
		Name: in.Name, Comment: in.Comment, ProviderID: provID,
		Priority: in.Priority, Share: in.Share, RouteType: in.RouteType, Status: in.Status,
	}
	if out.RouteType == "" { out.RouteType = "sms" }
	if out.Status == "" { out.Status = "active" }
	if out.Share == 0 { out.Share = 100 }
	for idx, g := range in.ConditionGroups {
		grp := storage.RouteSetConditionGroup{GroupIndex: int16(idx), LogicOp: g.LogicOp}
		for _, c := range g.Conditions {
			grp.Conditions = append(grp.Conditions, storage.RouteSetCondition{Type: c.Type, Value: c.Value})
		}
		out.ConditionGroups = append(out.ConditionGroups, grp)
	}
	for _, s := range in.Schedules {
		sched := storage.RouteSetSchedule{Weekdays: s.Weekdays, Timezone: s.Timezone}
		if sched.Weekdays == 0 { sched.Weekdays = 127 }
		if sched.Timezone == "" { sched.Timezone = "Europe/Moscow" }
		if s.DateFrom != nil {
			d, err := time.Parse("2006-01-02", *s.DateFrom)
			if err != nil { return storage.RouteSetItemFull{}, fmt.Errorf("date_from") }
			sched.DateFrom = &d
		}
		if s.DateTo != nil {
			d, err := time.Parse("2006-01-02", *s.DateTo)
			if err != nil { return storage.RouteSetItemFull{}, fmt.Errorf("date_to") }
			sched.DateTo = &d
		}
		if s.TimeFrom != nil { tf := *s.TimeFrom; sched.TimeFrom = &tf }
		if s.TimeTo != nil { tt := *s.TimeTo; sched.TimeTo = &tt }
		out.Schedules = append(out.Schedules, sched)
	}
	return out, nil
}

// validateProviderForSubscribers — для каждого подписанного на этот route-set client'а проверяет,
// что provider_id присутствует в его provider-set. Возвращает details для 409 или nil.
func (h *NetworkRouteSetItemsHandlers) validateProviderForSubscribers(ctx context.Context, routeSetID, providerID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT sra.client_id, COALESCE(c.name, c.email), sra.provider_set_id, ps.name
		FROM subaccount_routing_assignment sra
		JOIN clients c ON c.id = sra.client_id
		LEFT JOIN reseller_provider_sets ps ON ps.id = sra.provider_set_id
		WHERE sra.route_set_id = $1`, routeSetID)
	if err != nil { return nil, err }
	defer rows.Close()
	var conflicts []map[string]interface{}
	for rows.Next() {
		var clientID uuid.UUID; var name string
		var psID *uuid.UUID; var psName *string
		if err := rows.Scan(&clientID, &name, &psID, &psName); err != nil { return nil, err }
		if psID == nil {
			conflicts = append(conflicts, map[string]interface{}{
				"client_id": clientID.String(), "client_name": name,
				"current_provider_set": nil, "missing": providerID.String(),
			})
			continue
		}
		var exists bool
		h.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM reseller_provider_set_items WHERE set_id=$1 AND provider_id=$2)`,
			*psID, providerID).Scan(&exists)
		if !exists {
			cps := ""
			if psName != nil { cps = *psName }
			conflicts = append(conflicts, map[string]interface{}{
				"client_id": clientID.String(), "client_name": name,
				"current_provider_set": cps, "missing": providerID.String(),
			})
		}
	}
	return conflicts, nil
}

func (h *NetworkRouteSetItemsHandlers) Create(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verify(r.Context(), resellerID, setID); err != nil { respondError(w, err); return }

	var in itemIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	full, err := parseItemIn(in)
	if err != nil { respondError(w, shared.ErrInvalidInput(err.Error())); return }

	conflicts, err := h.validateProviderForSubscribers(r.Context(), setID, full.ProviderID)
	if err != nil { respondError(w, shared.ErrInternalServer("validate")); return }
	if len(conflicts) > 0 {
		details, _ := json.Marshal(map[string]interface{}{"kind": "route_uses_unavailable_provider", "items": conflicts})
		respondError(w, shared.ErrConflict("provider правила отсутствует в provider-set подписанного суб-аккаунта").
			WithDetails(string(details)))
		return
	}

	created, err := h.itemsRepo.CreateFull(r.Context(), setID, full)
	if err != nil { respondError(w, shared.ErrInternalServer("create")); return }

	if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
		respondError(w, shared.ErrInternalServer("rematerialize")); return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": created.ID.String()})
}

func (h *NetworkRouteSetItemsHandlers) Update(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verify(r.Context(), resellerID, setID); err != nil { respondError(w, err); return }
	itemID, perr := uuid.Parse(mux.Vars(r)["item_id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("item_id")); return }

	var in itemIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	full, err := parseItemIn(in)
	if err != nil { respondError(w, shared.ErrInvalidInput(err.Error())); return }

	conflicts, err := h.validateProviderForSubscribers(r.Context(), setID, full.ProviderID)
	if err != nil { respondError(w, shared.ErrInternalServer("validate")); return }
	if len(conflicts) > 0 {
		details, _ := json.Marshal(map[string]interface{}{"kind": "route_uses_unavailable_provider", "items": conflicts})
		respondError(w, shared.ErrConflict("provider правила отсутствует в provider-set подписанного суб-аккаунта").
			WithDetails(string(details)))
		return
	}

	if err := h.itemsRepo.UpdateFull(r.Context(), itemID, full); err != nil {
		respondError(w, shared.ErrInternalServer("update")); return
	}
	if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
		respondError(w, shared.ErrInternalServer("rematerialize")); return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": itemID.String()})
}

func (h *NetworkRouteSetItemsHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verify(r.Context(), resellerID, setID); err != nil { respondError(w, err); return }
	itemID, perr := uuid.Parse(mux.Vars(r)["item_id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("item_id")); return }

	if err := h.itemsRepo.Delete(r.Context(), itemID); err != nil {
		respondError(w, shared.ErrInternalServer("delete")); return
	}
	if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
		respondError(w, shared.ErrInternalServer("rematerialize")); return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NetworkRouteSetItemsHandlers) Duplicate(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verify(r.Context(), resellerID, setID); err != nil { respondError(w, err); return }
	srcID, perr := uuid.Parse(mux.Vars(r)["item_id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("item_id")); return }

	src, err := h.itemsRepo.LoadFullByItem(r.Context(), srcID)
	if err != nil { respondError(w, shared.ErrNotFound("item")); return }
	src.Name = src.Name + " (копия)"
	src.Priority = src.Priority + 1
	created, err := h.itemsRepo.CreateFull(r.Context(), setID, *src)
	if err != nil { respondError(w, shared.ErrInternalServer("dup")); return }
	if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
		respondError(w, shared.ErrInternalServer("rematerialize")); return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": created.ID.String()})
}

type reorderEntry struct {
	ItemID   string `json:"item_id"`
	Priority int    `json:"priority"`
}

func (h *NetworkRouteSetItemsHandlers) Reorder(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verify(r.Context(), resellerID, setID); err != nil { respondError(w, err); return }

	var body struct{ Items []reorderEntry `json:"items"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	entries := make([]storage.RouteSetReorderEntry, 0, len(body.Items))
	for _, e := range body.Items {
		id, err := uuid.Parse(e.ItemID)
		if err != nil { respondError(w, shared.ErrInvalidInput("item_id")); return }
		entries = append(entries, storage.RouteSetReorderEntry{ItemID: id, Priority: e.Priority})
	}
	if err := h.itemsRepo.Reorder(r.Context(), setID, entries); err != nil {
		respondError(w, shared.ErrInternalServer("reorder")); return
	}
	if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
		respondError(w, shared.ErrInternalServer("rematerialize")); return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"set_id": setID.String()})
}
```

- [ ] **Step 10.3: Run tests + commit**

```
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test ./internal/gateway/portal/handlers/ -run TestRouteSetItems -v"
git add internal/gateway/portal/handlers/network_route_set_items.go internal/gateway/portal/handlers/network_route_set_items_test.go
git commit -m "feat(handlers): /reseller/network/route-sets/{id}/items CRUD + duplicate + reorder (Plan 2 Task 10)"
```

---

### Task 11: Handler `/reseller/network/route-sets/{id}/preview` — симулятор matching'а

**Files:**
- Create: `internal/gateway/portal/handlers/network_route_preview.go`
- Create: `internal/gateway/portal/handlers/network_route_preview_test.go`

POST `/preview` body: `{phone, sender_id, traffic_type}` → возвращает `[{matched_item_id, item_name, provider_id, provider_name, priority}]` в порядке приоритета. Учитывает условия (country по prefix телефона, traffic_type — точное совпадение, regex — match) и расписание (текущее время/weekday/timezone).

**Жертва:** для country prefix используем простой mapping в коде (RU=+7, KZ=+7-7..., и так далее) — этого достаточно для preview, реальный matching в pipeline берёт из operators-таблицы. Если хочется точнее — отложено.

- [ ] **Step 11.1: Failing tests**

```go
func TestPreview_MatchesByCountry(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10, [][2]string{{"country", "RU"}})

	h := NewNetworkRoutePreviewHandlers(pool)
	body := `{"phone":"79991234567","sender_id":"TestSender","traffic_type":"transactional"}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/route-sets/"+rsID.String()+"/preview", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Preview(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct{ Matches []struct{ ItemID string `json:"matched_item_id"` } `json:"matches"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	require.Len(t, resp.Matches, 1)
}

func TestPreview_NoMatch_DifferentCountry(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10, [][2]string{{"country", "RU"}})

	h := NewNetworkRoutePreviewHandlers(pool)
	body := `{"phone":"491234567890","sender_id":"X","traffic_type":"transactional"}` // DE
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/route-sets/"+rsID.String()+"/preview", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Preview(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct{ Matches []interface{} `json:"matches"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	require.Empty(t, resp.Matches)
}
```

- [ ] **Step 11.2: Реализация**

`network_route_preview.go`:
```go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type NetworkRoutePreviewHandlers struct {
	pool      *pgxpool.Pool
	setRepo   *storage.ResellerRouteSetRepository
	itemsRepo *storage.ResellerRouteSetItemsRepository
}

func NewNetworkRoutePreviewHandlers(pool *pgxpool.Pool) *NetworkRoutePreviewHandlers {
	return &NetworkRoutePreviewHandlers{
		pool: pool,
		setRepo:   storage.NewResellerRouteSetRepository(pool),
		itemsRepo: storage.NewResellerRouteSetItemsRepository(pool),
	}
}

// countryByPhonePrefix — упрощённое определение страны по началу номера. Для preview достаточно;
// реальный pipeline берёт из таблицы operators.
var phonePrefixes = map[string]string{
	"7": "RU", "375": "BY", "380": "UA", "996": "KG",
	"998": "UZ", "992": "TJ", "993": "TM", "994": "AZ",
	"995": "GE", "374": "AM", "1": "US", "44": "GB", "49": "DE",
	"33": "FR", "39": "IT", "34": "ES", "86": "CN", "91": "IN",
}
func countryByPhone(phone string) string {
	clean := strings.TrimPrefix(strings.TrimSpace(phone), "+")
	for prefix, code := range phonePrefixes {
		if strings.HasPrefix(clean, prefix) {
			// Уточнение для KZ vs RU: оба +7, но KZ начинается с 76/77
			if code == "RU" && len(clean) >= 2 {
				if clean[1] == '6' || clean[1] == '7' {
					return "KZ"
				}
			}
			return code
		}
	}
	return ""
}

type previewReq struct {
	Phone       string `json:"phone"`
	SenderID    string `json:"sender_id"`
	TrafficType string `json:"traffic_type"`
}

type previewMatch struct {
	ItemID       string `json:"matched_item_id"`
	ItemName     string `json:"item_name"`
	ProviderID   string `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	Priority     int    `json:"priority"`
}

func (h *NetworkRoutePreviewHandlers) Preview(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	set, err := h.setRepo.GetByID(r.Context(), setID)
	if err != nil || set.ResellerID != resellerID {
		respondError(w, shared.ErrNotFound("route-set")); return
	}
	var req previewReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	items, err := h.itemsRepo.ListBySet(r.Context(), setID)
	if err != nil { respondError(w, shared.ErrInternalServer("load")); return }

	now := time.Now()
	country := countryByPhone(req.Phone)
	matches := []previewMatch{}
	for _, it := range items {
		if it.Status != "active" { continue }
		if !matchesConditions(it.ConditionGroups, country, req.TrafficType, req.SenderID, req.Phone) {
			continue
		}
		if !matchesSchedules(it.Schedules, now) {
			continue
		}
		// Provider name lookup
		var pname string
		h.pool.QueryRow(r.Context(), `SELECT name FROM providers WHERE id = $1`, it.ProviderID).Scan(&pname)
		matches = append(matches, previewMatch{
			ItemID: it.ID.String(), ItemName: it.Name,
			ProviderID: it.ProviderID.String(), ProviderName: pname, Priority: it.Priority,
		})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"matches": matches})
}

func matchesConditions(groups []storage.RouteSetConditionGroup, country, trafficType, senderID, phone string) bool {
	if len(groups) == 0 { return true } // нет условий — match всегда
	// Простая стратегия: каждая группа оценивается, результаты комбинируются согласно logic_op первой группы:
	// IF — все группы должны быть true; OR — хоть одна true; AND_NOT — все должны быть НЕ-match.
	// В практике используется одна группа с logic_op=IF (см. spec §5.4).
	for _, g := range groups {
		groupResult := true
		for _, c := range g.Conditions {
			if !evalCondition(c, country, trafficType, senderID, phone) {
				groupResult = false
				break
			}
		}
		switch g.LogicOp {
		case "IF", "AND":
			if !groupResult { return false }
		case "AND_NOT":
			if groupResult { return false }
		case "OR":
			if groupResult { return true }
		case "OR_NOT":
			if !groupResult { return true }
		}
	}
	return true
}

func evalCondition(c storage.RouteSetCondition, country, trafficType, senderID, phone string) bool {
	switch c.Type {
	case "country":
		return c.Value == country
	case "traffic_type":
		return c.Value == trafficType
	case "paid_name":
		return c.Value == senderID
	case "regex":
		re, err := regexp.Compile(c.Value)
		if err != nil { return false }
		return re.MatchString(phone)
	case "operator":
		// Operator-matching требует таблицы operators. В preview игнорируем (всегда true).
		return true
	}
	return false
}

func matchesSchedules(scheds []storage.RouteSetSchedule, now time.Time) bool {
	if len(scheds) == 0 { return true }
	for _, s := range scheds {
		loc, err := time.LoadLocation(s.Timezone)
		if err != nil { loc = time.UTC }
		t := now.In(loc)
		if s.DateFrom != nil && t.Before(*s.DateFrom) { continue }
		if s.DateTo != nil && t.After(s.DateTo.AddDate(0, 0, 1)) { continue }
		// weekdays bitmask: 1=Mon,2=Tue,4=Wed,8=Thu,16=Fri,32=Sat,64=Sun
		wd := int(t.Weekday())
		if wd == 0 { wd = 7 } // Sun
		mask := int16(1) << (wd - 1)
		if s.Weekdays&mask == 0 { continue }
		// Time check (skipped if either time bound is nil)
		if s.TimeFrom != nil && s.TimeTo != nil {
			cur := t.Format("15:04:05")
			if cur < *s.TimeFrom || cur > *s.TimeTo { continue }
		}
		return true
	}
	return false
}

// silence unused-import warning for context if compiler complains
var _ = context.TODO
```

- [ ] **Step 11.3: Run tests + commit**

```
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test ./internal/gateway/portal/handlers/ -run TestPreview -v"
git add internal/gateway/portal/handlers/network_route_preview.go internal/gateway/portal/handlers/network_route_preview_test.go
git commit -m "feat(handlers): /reseller/network/route-sets/{id}/preview — matching simulator (Plan 2 Task 11)"
```

---

### Task 12: Расширить `/reseller/network/assignments` route_set_id + validation_status='conflict'

**Files:**
- Modify: `internal/gateway/portal/handlers/network_assignments.go`
- Modify: `internal/gateway/portal/handlers/network_assignments_test.go`

Изменения:
1. List endpoint: дополнительно JOIN с `reseller_route_sets` для `route_set_name`; для каждой строки запускаем валидатор и пишем `validation_status='conflict'` если missing-providers > 0; `validation_error` — текст для tooltip.
2. PutOne: декодирует `route_set_id`, валидирует pair (provider_set_id, route_set_id) перед материализацией; при missing → 409 с details. После успеха — UPSERT assignment + материализация **обоих** материализаторов (provider + route).
3. Bulk: то же per-client; в response status='conflict' с per-client error.

- [ ] **Step 12.1: Failing tests**

```go
func TestAssignments_List_ValidationStatus_Conflict(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")

	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	ps, _ := psRepo.Create(context.Background(), resellerID, "PS", false)
	psItems.ReplaceItems(context.Background(), ps.ID, []storage.ProviderSetItemInput{{ProviderID: provA, ExposeProviderName: true}})

	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, rsID, provB, 10, nil) // provB не в provider-set

	pool.Exec(context.Background(),
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id) VALUES ($1, $2, $3)
		 ON CONFLICT (client_id) DO UPDATE SET provider_set_id=EXCLUDED.provider_set_id, route_set_id=EXCLUDED.route_set_id`,
		subID, ps.ID, rsID)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	provMat := network.NewProviderSetMaterializer(pool)
	routeMat := network.NewRouteSetMaterializer(pool, rsItems)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, provMat, routeMat, validator)
	req := httptest.NewRequest("GET", "/portal/v1/reseller/network/assignments", nil)
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.List(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct{ Assignments []struct{ ValidationStatus string `json:"validation_status"` } `json:"assignments"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	require.Equal(t, "conflict", resp.Assignments[0].ValidationStatus)
}

func TestAssignments_PutOne_409OnMissingProvider(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")
	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	ps, _ := psRepo.Create(context.Background(), resellerID, "PS", false)
	psItems.ReplaceItems(context.Background(), ps.ID, []storage.ProviderSetItemInput{{ProviderID: provA, ExposeProviderName: true}})
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, rsID, provB, 10, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	provMat := network.NewProviderSetMaterializer(pool)
	routeMat := network.NewRouteSetMaterializer(pool, rsItems)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, provMat, routeMat, validator)
	body := fmt.Sprintf(`{"provider_set_id":"%s","route_set_id":"%s"}`, ps.ID.String(), rsID.String())
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/assignments/"+subID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"client_id": subID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.PutOne(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), "route_uses_unavailable_provider")
}

func TestAssignments_PutOne_BothMaterializersRun(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	ps, _ := psRepo.Create(context.Background(), resellerID, "PS", false)
	psItems.ReplaceItems(context.Background(), ps.ID, []storage.ProviderSetItemInput{{ProviderID: provA, ExposeProviderName: true}})
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	provMat := network.NewProviderSetMaterializer(pool)
	routeMat := network.NewRouteSetMaterializer(pool, rsItems)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, provMat, routeMat, validator)
	body := fmt.Sprintf(`{"provider_set_id":"%s","route_set_id":"%s"}`, ps.ID.String(), rsID.String())
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/assignments/"+subID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"client_id": subID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.PutOne(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var providers, routes int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_providers WHERE client_id=$1 AND ownership='inherited'`, subID).Scan(&providers)
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`, subID).Scan(&routes)
	require.Equal(t, 1, providers)
	require.Equal(t, 1, routes)
}

func TestAssignments_BulkDryRun(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	sub1 := storagetest.SeedSubAccount(t, pool, resellerID)
	sub2 := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")
	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	ps, _ := psRepo.Create(context.Background(), resellerID, "PS", false)
	psItems.ReplaceItems(context.Background(), ps.ID, []storage.ProviderSetItemInput{{ProviderID: provA, ExposeProviderName: true}})
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, rsID, provB, 10, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, nil, nil, validator)
	body := fmt.Sprintf(`{"client_ids":["%s","%s"],"provider_set_id":"%s","route_set_id":"%s"}`,
		sub1.String(), sub2.String(), ps.ID.String(), rsID.String())
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/assignments/bulk/dry-run", strings.NewReader(body))
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.BulkDryRun(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct{ Results []struct{ Status string `json:"status"` } `json:"results"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	require.Len(t, resp.Results, 2)
	require.Equal(t, "conflict", resp.Results[0].Status)
}
```

- [ ] **Step 12.2: Реализация — модифицировать `network_assignments.go`**

Изменения в существующем файле (см. [network_assignments.go](../../internal/gateway/portal/handlers/network_assignments.go)):

1. Расширить конструктор:
```go
type NetworkAssignmentsHandlers struct {
	pool          *pgxpool.Pool
	providerMat   *network.ProviderSetMaterializer
	routeMat      *network.RouteSetMaterializer
	validator     *network.ConflictValidator
}

func NewNetworkAssignmentsHandlers(pool *pgxpool.Pool, pm *network.ProviderSetMaterializer, rm *network.RouteSetMaterializer, v *network.ConflictValidator) *NetworkAssignmentsHandlers {
	return &NetworkAssignmentsHandlers{pool: pool, providerMat: pm, routeMat: rm, validator: v}
}
```

2. List — дополнить JOIN'ами и валидацией:
```go
func (h *NetworkAssignmentsHandlers) List(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	rows, err := h.pool.Query(r.Context(), `
		SELECT c.id, COALESCE(c.name, c.email) AS name,
		       sra.provider_set_id, ps.name,
		       sra.route_set_id, rs.name,
		       EXISTS(SELECT 1 FROM client_providers cp WHERE cp.client_id = c.id AND cp.ownership='private'
		              UNION ALL SELECT 1 FROM client_routes cr WHERE cr.client_id = c.id AND cr.source='override') AS has_overrides
		FROM clients c
		LEFT JOIN subaccount_routing_assignment sra ON sra.client_id = c.id
		LEFT JOIN reseller_provider_sets ps ON ps.id = sra.provider_set_id
		LEFT JOIN reseller_route_sets rs ON rs.id = sra.route_set_id
		WHERE c.parent_client_id = $1
		ORDER BY name`, resellerID)
	if err != nil { respondError(w, shared.ErrInternalServer("query")); return }
	defer rows.Close()
	out := []assignmentOut{}
	for rows.Next() {
		var a assignmentOut
		var psID, psName, rsID, rsName *string
		if err := rows.Scan(&a.ClientID, &a.SubAccountName, &psID, &psName, &rsID, &rsName, &a.HasOverrides); err != nil {
			respondError(w, shared.ErrInternalServer("scan")); return
		}
		a.ProviderSetID = psID
		a.ProviderSetName = psName
		a.RouteSetID = rsID
		a.RouteSetName = rsName
		// Validation: если назначены оба set'а — проверить missing-providers
		if psID != nil && rsID != nil {
			psUUID, _ := uuid.Parse(*psID)
			rsUUID, _ := uuid.Parse(*rsID)
			missing, _ := h.validator.MissingProviders(r.Context(), psUUID, rsUUID)
			if len(missing) > 0 {
				a.ValidationStatus = "conflict"
				a.ValidationError = fmt.Sprintf("маршрут использует %d провайдер(ов) вне provider-set", len(missing))
			} else {
				a.ValidationStatus = "ok"
			}
		} else if psID == nil && rsID == nil {
			a.ValidationStatus = "unassigned"
		} else {
			a.ValidationStatus = "ok"
		}
		out = append(out, a)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"assignments": out})
}
```

Добавить поле в `assignmentOut`:
```go
type assignmentOut struct {
	// ... существующие поля
	ValidationError string `json:"validation_error,omitempty"`
}
```

3. PutOne — обработать `route_set_id` и pre-validate:
```go
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
	psUUID, rsUUID, err := h.parseAndVerifySetIDs(r.Context(), resellerID, req.ProviderSetID, req.RouteSetID)
	if err != nil { respondError(w, err); return }

	// Pre-validation conflict
	missing, err := h.validator.MissingProvidersByIDs(r.Context(), psUUID, rsUUID)
	if err != nil { respondError(w, shared.ErrInternalServer("validate")); return }
	if len(missing) > 0 {
		details, _ := json.Marshal(map[string]interface{}{
			"kind": "route_uses_unavailable_provider",
			"missing_providers": uuidsToStrings(missing),
		})
		respondError(w, shared.ErrConflict("маршрут использует провайдер вне provider-set").
			WithDetails(string(details)))
		return
	}

	if _, err := h.pool.Exec(r.Context(), `
		INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (client_id) DO UPDATE SET
		  provider_set_id = EXCLUDED.provider_set_id,
		  route_set_id    = EXCLUDED.route_set_id,
		  assigned_at = now()`,
		clientID, psUUID, rsUUID); err != nil {
		respondError(w, shared.ErrInternalServer("upsert")); return
	}
	if err := h.providerMat.ApplyToClient(r.Context(), clientID, psUUID); err != nil {
		respondError(w, shared.ErrInternalServer("provider materialize")); return
	}
	if err := h.routeMat.ApplyToClient(r.Context(), clientID, rsUUID); err != nil {
		respondError(w, shared.ErrInternalServer("route materialize")); return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"client_id": clientID.String()})
}

// helper
func (h *NetworkAssignmentsHandlers) parseAndVerifySetIDs(ctx context.Context, resellerID uuid.UUID, psStr, rsStr *string) (*uuid.UUID, *uuid.UUID, error) {
	var psUUID, rsUUID *uuid.UUID
	if psStr != nil && *psStr != "" {
		id, err := uuid.Parse(*psStr)
		if err != nil { return nil, nil, shared.ErrInvalidInput("provider_set_id") }
		var owner uuid.UUID
		if err := h.pool.QueryRow(ctx, `SELECT reseller_id FROM reseller_provider_sets WHERE id=$1`, id).Scan(&owner); err != nil || owner != resellerID {
			return nil, nil, shared.ErrNotFound("provider-set")
		}
		psUUID = &id
	}
	if rsStr != nil && *rsStr != "" {
		id, err := uuid.Parse(*rsStr)
		if err != nil { return nil, nil, shared.ErrInvalidInput("route_set_id") }
		var owner uuid.UUID
		if err := h.pool.QueryRow(ctx, `SELECT reseller_id FROM reseller_route_sets WHERE id=$1`, id).Scan(&owner); err != nil || owner != resellerID {
			return nil, nil, shared.ErrNotFound("route-set")
		}
		rsUUID = &id
	}
	return psUUID, rsUUID, nil
}

func uuidsToStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids { out[i] = id.String() }
	return out
}
```

4. Bulk — то же per-client + validation; status='conflict' для конфликтов:
```go
func (h *NetworkAssignmentsHandlers) Bulk(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	var req bulkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	psUUID, rsUUID, err := h.parseAndVerifySetIDs(r.Context(), resellerID, req.ProviderSetID, req.RouteSetID)
	if err != nil { respondError(w, err); return }

	missing, _ := h.validator.MissingProvidersByIDs(r.Context(), psUUID, rsUUID)
	conflictExists := len(missing) > 0

	results := make([]bulkResultItem, 0, len(req.ClientIDs))
	for _, idStr := range req.ClientIDs {
		cid, err := uuid.Parse(idStr)
		if err != nil { results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "invalid id"}); continue }
		if err := h.verifySubAccountOwnership(r.Context(), resellerID, cid); err != nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "not your sub-account"}); continue
		}
		if conflictExists {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "conflict", Error: "missing providers"}); continue
		}
		if _, err := h.pool.Exec(r.Context(), `
			INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
			VALUES ($1, $2, $3, now())
			ON CONFLICT (client_id) DO UPDATE SET
			  provider_set_id = EXCLUDED.provider_set_id,
			  route_set_id    = EXCLUDED.route_set_id,
			  assigned_at = now()`,
			cid, psUUID, rsUUID); err != nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: err.Error()}); continue
		}
		if err := h.providerMat.ApplyToClient(r.Context(), cid, psUUID); err != nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: err.Error()}); continue
		}
		if err := h.routeMat.ApplyToClient(r.Context(), cid, rsUUID); err != nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: err.Error()}); continue
		}
		results = append(results, bulkResultItem{ClientID: idStr, Status: "ok"})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}
```

5. BulkDryRun — новая функция:
```go
func (h *NetworkAssignmentsHandlers) BulkDryRun(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	var req bulkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	psUUID, rsUUID, err := h.parseAndVerifySetIDs(r.Context(), resellerID, req.ProviderSetID, req.RouteSetID)
	if err != nil { respondError(w, err); return }
	missing, _ := h.validator.MissingProvidersByIDs(r.Context(), psUUID, rsUUID)
	results := make([]bulkResultItem, 0, len(req.ClientIDs))
	for _, idStr := range req.ClientIDs {
		cid, err := uuid.Parse(idStr)
		if err != nil { results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "invalid id"}); continue }
		if err := h.verifySubAccountOwnership(r.Context(), resellerID, cid); err != nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "not your sub-account"}); continue
		}
		if len(missing) > 0 {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "conflict",
				Error: fmt.Sprintf("маршрут использует %d провайдер(ов) вне provider-set", len(missing))})
			continue
		}
		results = append(results, bulkResultItem{ClientID: idStr, Status: "ok"})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}
```

6. Расширить `bulkResultItem.Status` enum в комментарии: `"ok" | "error" | "conflict"`.

- [ ] **Step 12.3: Run tests + commit**

```
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test ./internal/gateway/portal/handlers/ -run TestAssignments -v"
git add internal/gateway/portal/handlers/network_assignments.go internal/gateway/portal/handlers/network_assignments_test.go
git commit -m "feat(handlers): assignments — route_set_id support + conflict validation + dry-run (Plan 2 Task 12)"
```

---

### Task 13: Cleanup-orphans endpoint

**Files:**
- Create: `internal/gateway/portal/handlers/network_cleanup.go`
- Create: `internal/gateway/portal/handlers/network_cleanup_test.go`

POST `/reseller/network/route-cleanup` body `{provider_id}` → удаляет:
1. Все `reseller_route_set_items` этого reseller'а с `provider_id = X`
2. Все `client_routes` (source='override') суб-аккаунтов этого reseller'а с `provider_id = X`
3. После — материализация всех затронутых route-set'ов

Возвращает `{removed_route_set_items: N, removed_overrides: M}`.

- [ ] **Step 13.1: Failing test**

```go
func TestRouteCleanup_RemovesOrphanItems(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10, nil)
	pool.Exec(context.Background(),
		`INSERT INTO client_routes (client_id, provider_id, priority, weight, active, name, status, share, route_type, source)
		 VALUES ($1, $2, 10, 1, true, 'override-r', 'active', 100, 'sms', 'override')`,
		subID, provA)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, rsItems)
	h := NewNetworkCleanupHandlers(pool, mat)
	body := fmt.Sprintf(`{"provider_id":"%s"}`, provA.String())
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/route-cleanup", strings.NewReader(body))
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.RouteCleanup(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		RemovedRouteSetItems int `json:"removed_route_set_items"`
		RemovedOverrides     int `json:"removed_overrides"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	require.Equal(t, 1, resp.RemovedRouteSetItems)
	require.Equal(t, 1, resp.RemovedOverrides)
}
```

- [ ] **Step 13.2: Реализация**

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type NetworkCleanupHandlers struct {
	pool *pgxpool.Pool
	mat  *network.RouteSetMaterializer
}

func NewNetworkCleanupHandlers(pool *pgxpool.Pool, mat *network.RouteSetMaterializer) *NetworkCleanupHandlers {
	return &NetworkCleanupHandlers{pool: pool, mat: mat}
}

type cleanupReq struct {
	ProviderID string `json:"provider_id"`
}

func (h *NetworkCleanupHandlers) RouteCleanup(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	var req cleanupReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	provID, err := uuid.Parse(req.ProviderID)
	if err != nil { respondError(w, shared.ErrInvalidInput("provider_id")); return }

	tx, err := h.pool.Begin(r.Context())
	if err != nil { respondError(w, shared.ErrInternalServer("tx")); return }
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Find affected route-sets перед удалением items (для materialize after commit)
	var affectedSetIDs []uuid.UUID
	rows, err := tx.Query(r.Context(),
		`SELECT DISTINCT i.set_id FROM reseller_route_set_items i
		 JOIN reseller_route_sets s ON s.id = i.set_id
		 WHERE s.reseller_id = $1 AND i.provider_id = $2`,
		resellerID, provID)
	if err != nil { respondError(w, shared.ErrInternalServer("query")); return }
	for rows.Next() {
		var id uuid.UUID
		rows.Scan(&id)
		affectedSetIDs = append(affectedSetIDs, id)
	}
	rows.Close()

	tag1, err := tx.Exec(r.Context(), `
		DELETE FROM reseller_route_set_items i
		 USING reseller_route_sets s
		 WHERE i.set_id = s.id AND s.reseller_id = $1 AND i.provider_id = $2`,
		resellerID, provID)
	if err != nil { respondError(w, shared.ErrInternalServer("delete items")); return }

	tag2, err := tx.Exec(r.Context(), `
		DELETE FROM client_routes cr
		 USING clients c
		 WHERE cr.client_id = c.id AND c.parent_client_id = $1
		   AND cr.provider_id = $2 AND cr.source = 'override'`,
		resellerID, provID)
	if err != nil { respondError(w, shared.ErrInternalServer("delete overrides")); return }

	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("commit")); return
	}

	// Re-materialize affected route-sets (вне транзакции — каждый под своей tx)
	for _, sid := range affectedSetIDs {
		if err := h.mat.ApplyToAllSubscribers(r.Context(), sid); err != nil {
			// Не падаем — items уже удалены, лог поможет диагностике
			continue
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"removed_route_set_items": tag1.RowsAffected(),
		"removed_overrides":       tag2.RowsAffected(),
	})
}
```

- [ ] **Step 13.3: Run tests + commit**

```
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test ./internal/gateway/portal/handlers/ -run TestRouteCleanup -v"
git add internal/gateway/portal/handlers/network_cleanup.go internal/gateway/portal/handlers/network_cleanup_test.go
git commit -m "feat(handlers): /reseller/network/route-cleanup — cascade-cleanup orphan routes (Plan 2 Task 13)"
```

---

### Task 14: Override-routes для суб-аккаунта + расширение Overview

**Files:**
- Modify: `internal/gateway/portal/handlers/subaccount_network_overrides.go`
- Modify: `internal/gateway/portal/handlers/subaccount_network_overrides_test.go`

Endpoints:
- POST `/sub-accounts/{id}/network/route-overrides` — создать override-маршрут (source='override')
- PUT `/sub-accounts/{id}/network/route-overrides/{route_id}` — full replace
- DELETE `/sub-accounts/{id}/network/route-overrides/{route_id}`
- Расширить `Overview` GET: вернуть `route_set` (id, name) и `route_overrides: [...]`

**Pre-validation override-маршрута (spec §6.2 Точка 4):** provider_id должен быть в `client_providers` суб-аккаунта (любого ownership). Иначе 409 `kind: route_provider_not_available`.

- [ ] **Step 14.1: Failing tests**

```go
func TestRouteOverride_Add_OK(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	pool.Exec(context.Background(),
		`INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		 VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := fmt.Sprintf(`{
		"name":"override-1","provider_id":"%s","priority":50,"share":100,"route_type":"sms","status":"active",
		"condition_groups":[{"logic_op":"IF","conditions":[{"type":"country","value":"RU"}]}]
	}`, provA.String())
	req := httptest.NewRequest("POST", "/portal/v1/sub-accounts/"+subID.String()+"/network/route-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var count int
	pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='override'`, subID).Scan(&count)
	require.Equal(t, 1, count)
}

func TestRouteOverride_Add_409IfProviderUnavailable(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provB := storagetest.SeedProvider(t, pool, "B") // не привязан

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := fmt.Sprintf(`{"name":"x","provider_id":"%s","priority":1,"share":100,"route_type":"sms","status":"active"}`, provB.String())
	req := httptest.NewRequest("POST", "/portal/v1/sub-accounts/"+subID.String()+"/network/route-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
}

func TestOverview_IncludesRouteSetAndOverrides(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	pool.Exec(context.Background(),
		`INSERT INTO subaccount_routing_assignment (client_id, route_set_id) VALUES ($1, $2)
		 ON CONFLICT (client_id) DO UPDATE SET route_set_id=EXCLUDED.route_set_id`,
		subID, rsID)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	req := httptest.NewRequest("GET", "/portal/v1/sub-accounts/"+subID.String()+"/network/overview", nil)
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Overview(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	require.NotNil(t, resp["route_set"])
}
```

- [ ] **Step 14.2: Реализация (доп. в `subaccount_network_overrides.go`)**

```go
type addRouteOverrideReq struct {
	itemIn // переиспользуем структуру из network_route_set_items.go (Task 10)
}

func (h *SubAccountNetworkOverridesHandlers) AddRouteOverride(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	subID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("id")); return }
	if err := h.verifyOwnership(r.Context(), resellerID, subID); err != nil { respondError(w, err); return }

	var req addRouteOverrideReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	full, err := parseItemIn(req.itemIn)
	if err != nil { respondError(w, shared.ErrInvalidInput(err.Error())); return }

	// Provider должен быть в client_providers суб-аккаунта
	var exists bool
	h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM client_providers WHERE client_id=$1 AND provider_id=$2 AND active=true)`,
		subID, full.ProviderID).Scan(&exists)
	if !exists {
		respondError(w, shared.ErrConflict("провайдер недоступен этому суб-аккаунту").
			WithDetails(`{"kind":"route_provider_not_available"}`))
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil { respondError(w, shared.ErrInternalServer("tx")); return }
	defer tx.Rollback(r.Context()) //nolint:errcheck

	var routeID uuid.UUID
	err = tx.QueryRow(r.Context(),
		`INSERT INTO client_routes
		   (client_id, operator_id, provider_id, priority, weight, active,
		    name, comment, status, share, route_type, source)
		 VALUES ($1, NULL, $2, $3, 1, true, NULLIF($4,''), NULLIF($5,''), $6, $7, $8, 'override')
		 RETURNING id`,
		subID, full.ProviderID, full.Priority, full.Name, full.Comment,
		full.Status, full.Share, full.RouteType,
	).Scan(&routeID)
	if err != nil { respondError(w, shared.ErrInternalServer("insert")); return }

	for idx, g := range full.ConditionGroups {
		var gid int64
		err := tx.QueryRow(r.Context(),
			`INSERT INTO route_condition_groups (route_id, group_index, logic_op)
			 VALUES ($1, $2, $3) RETURNING id`,
			routeID, int16(idx), g.LogicOp,
		).Scan(&gid)
		if err != nil { respondError(w, shared.ErrInternalServer("group")); return }
		for _, c := range g.Conditions {
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO route_conditions (group_id, condition_type, condition_value)
				 VALUES ($1, $2, $3)`, gid, c.Type, c.Value); err != nil {
				respondError(w, shared.ErrInternalServer("cond")); return
			}
		}
	}
	for _, s := range full.Schedules {
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO route_schedules (route_id, date_from, date_to, time_from, time_to, weekdays, timezone)
			 VALUES ($1, $2, $3, $4::time, $5::time, $6, $7)`,
			routeID, s.DateFrom, s.DateTo, s.TimeFrom, s.TimeTo, s.Weekdays, s.Timezone); err != nil {
			respondError(w, shared.ErrInternalServer("sched")); return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("commit")); return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": routeID.String()})
}

func (h *SubAccountNetworkOverridesHandlers) UpdateRouteOverride(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	subID, _ := uuid.Parse(mux.Vars(r)["id"])
	routeID, perr := uuid.Parse(mux.Vars(r)["route_id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("route_id")); return }
	if err := h.verifyOwnership(r.Context(), resellerID, subID); err != nil { respondError(w, err); return }

	// Verify route belongs to this sub-account and is override
	var srcCheck string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT source FROM client_routes WHERE id=$1 AND client_id=$2`, routeID, subID,
	).Scan(&srcCheck); err != nil || srcCheck != "override" {
		respondError(w, shared.ErrNotFound("override")); return
	}

	var req addRouteOverrideReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("JSON")); return
	}
	full, err := parseItemIn(req.itemIn)
	if err != nil { respondError(w, shared.ErrInvalidInput(err.Error())); return }

	tx, err := h.pool.Begin(r.Context())
	if err != nil { respondError(w, shared.ErrInternalServer("tx")); return }
	defer tx.Rollback(r.Context()) //nolint:errcheck
	if _, err := tx.Exec(r.Context(),
		`UPDATE client_routes SET provider_id=$1, priority=$2, name=NULLIF($3,''), comment=NULLIF($4,''),
		     status=$5, share=$6, route_type=$7, updated_at=now()
		 WHERE id=$8`,
		full.ProviderID, full.Priority, full.Name, full.Comment,
		full.Status, full.Share, full.RouteType, routeID); err != nil {
		respondError(w, shared.ErrInternalServer("update")); return
	}
	// CASCADE удалит condition_groups+conditions+schedules; явно для прозрачности
	if _, err := tx.Exec(r.Context(), `DELETE FROM route_condition_groups WHERE route_id=$1`, routeID); err != nil {
		respondError(w, shared.ErrInternalServer("del groups")); return
	}
	if _, err := tx.Exec(r.Context(), `DELETE FROM route_schedules WHERE route_id=$1`, routeID); err != nil {
		respondError(w, shared.ErrInternalServer("del scheds")); return
	}
	for idx, g := range full.ConditionGroups {
		var gid int64
		err := tx.QueryRow(r.Context(),
			`INSERT INTO route_condition_groups (route_id, group_index, logic_op)
			 VALUES ($1, $2, $3) RETURNING id`,
			routeID, int16(idx), g.LogicOp,
		).Scan(&gid)
		if err != nil { respondError(w, shared.ErrInternalServer("ins group")); return }
		for _, c := range g.Conditions {
			if _, err := tx.Exec(r.Context(),
				`INSERT INTO route_conditions (group_id, condition_type, condition_value)
				 VALUES ($1, $2, $3)`, gid, c.Type, c.Value); err != nil {
				respondError(w, shared.ErrInternalServer("ins cond")); return
			}
		}
	}
	for _, s := range full.Schedules {
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO route_schedules (route_id, date_from, date_to, time_from, time_to, weekdays, timezone)
			 VALUES ($1, $2, $3, $4::time, $5::time, $6, $7)`,
			routeID, s.DateFrom, s.DateTo, s.TimeFrom, s.TimeTo, s.Weekdays, s.Timezone); err != nil {
			respondError(w, shared.ErrInternalServer("ins sched")); return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("commit")); return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": routeID.String()})
}

func (h *SubAccountNetworkOverridesHandlers) DeleteRouteOverride(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok { respondError(w, shared.ErrUnauthorized("")); return }
	subID, _ := uuid.Parse(mux.Vars(r)["id"])
	routeID, perr := uuid.Parse(mux.Vars(r)["route_id"])
	if perr != nil { respondError(w, shared.ErrInvalidInput("route_id")); return }
	if err := h.verifyOwnership(r.Context(), resellerID, subID); err != nil { respondError(w, err); return }
	tag, err := h.pool.Exec(r.Context(),
		`DELETE FROM client_routes WHERE id=$1 AND client_id=$2 AND source='override'`,
		routeID, subID)
	if err != nil { respondError(w, shared.ErrInternalServer("delete")); return }
	if tag.RowsAffected() == 0 { respondError(w, shared.ErrNotFound("override")); return }
	w.WriteHeader(http.StatusNoContent)
}
```

Расширить `Overview` (существующий метод):
```go
// В Overview после загрузки provider_set добавить:
var routeSet *setRef
var rsID, rsName string
err = h.pool.QueryRow(r.Context(), `
	SELECT rs.id::text, rs.name
	FROM subaccount_routing_assignment sra
	JOIN reseller_route_sets rs ON rs.id = sra.route_set_id
	WHERE sra.client_id = $1`, subID).Scan(&rsID, &rsName)
if err == nil {
	routeSet = &setRef{ID: rsID, Name: rsName}
} else if !errors.Is(err, pgx.ErrNoRows) {
	respondError(w, shared.ErrInternalServer("query route_set")); return
}

// Загрузить route_overrides
overrideRows, err := h.pool.Query(r.Context(), `
	SELECT cr.id::text, COALESCE(cr.name,''), cr.provider_id::text, p.name,
	       cr.priority, cr.status
	FROM client_routes cr
	JOIN providers p ON p.id = cr.provider_id
	WHERE cr.client_id = $1 AND cr.source='override'
	ORDER BY cr.priority DESC`, subID)
if err != nil { respondError(w, shared.ErrInternalServer("query overrides")); return }
defer overrideRows.Close()
routeOverrides := []map[string]interface{}{}
for overrideRows.Next() {
	var rid, name, pid, pname, status string
	var priority int
	if err := overrideRows.Scan(&rid, &name, &pid, &pname, &priority, &status); err != nil {
		respondError(w, shared.ErrInternalServer("scan")); return
	}
	routeOverrides = append(routeOverrides, map[string]interface{}{
		"id": rid, "name": name, "provider_id": pid, "provider_name": pname,
		"priority": priority, "status": status,
	})
}

// в финальном respondJSON заменить:
// "route_set": nil → routeSet
// "route_overrides": []interface{}{} → routeOverrides
```

- [ ] **Step 14.3: Run tests + commit**

```
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test ./internal/gateway/portal/handlers/ -run TestRouteOverride -v -run TestOverview -v"
git add internal/gateway/portal/handlers/subaccount_network_overrides.go internal/gateway/portal/handlers/subaccount_network_overrides_test.go
git commit -m "feat(handlers): subaccount route-overrides + Overview расширение (Plan 2 Task 14)"
```

---

### Task 14b: Расширить Plan 1 handlers — provider DELETE + provider-set PUT items conflict checks для маршрутов

**Files:**
- Modify: `internal/gateway/portal/handlers/network_providers.go` (Delete handler)
- Modify: `internal/gateway/portal/handlers/network_provider_sets_items.go` или `network_provider_set_items.go` (PutItems handler)
- Modify: соответствующие тест-файлы

Закрывает spec §6.2 Точки 1 и 5, которые Plan 1 не покрыл (тогда route-set'ов не было).

**Точка 1 — PUT items provider-set:**
Перед `ReplaceItems` выясняем какие провайдеры удаляются (diff old−new). Для каждого такого провайдера проверяем:
- Есть ли он в `reseller_route_set_items` любого route-set'а этого reseller'а, где route-set назначен какому-либо суб-аккаунту, у которого этот provider-set подписан?
- Есть ли он в `client_routes` (source='override') суб-аккаунтов, подписанных на этот provider-set?

Если да → 409 с `kind: provider_used_in_routes` + детали.

**Точка 5 — DELETE private provider:**
Дополнить existing проверку (из Plan 1) на:
- Используется в `reseller_route_set_items` этого reseller'а
- Используется в `client_routes` (source='override') суб-аккаунтов этого reseller'а

- [ ] **Step 14b.1: Failing tests**

```go
// network_providers_test.go (доп.)
func TestProviders_Delete_409IfUsedInRouteSet(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	priv := storagetest.SeedProviderPrivate(t, pool, "P", resellerID)
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, rsID, priv, 10, nil)

	h := NewNetworkProvidersHandlers(pool)
	req := httptest.NewRequest("DELETE", "/portal/v1/reseller/network/providers/"+priv.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": priv.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), "provider_used_in_routes")
}

func TestProviders_Delete_409IfUsedInOverrideRoute(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	priv := storagetest.SeedProviderPrivate(t, pool, "P", resellerID)
	pool.Exec(context.Background(),
		`INSERT INTO client_routes (client_id, provider_id, priority, weight, active, name, status, share, route_type, source)
		 VALUES ($1, $2, 10, 1, true, 'over', 'active', 100, 'sms', 'override')`,
		subID, priv)

	h := NewNetworkProvidersHandlers(pool)
	req := httptest.NewRequest("DELETE", "/portal/v1/reseller/network/providers/"+priv.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": priv.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
}

// network_provider_set_items_test.go (доп.)
func TestProviderSetItems_PUT_409IfRemovingProviderUsedInSubscriberRoutes(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t); defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")

	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	ps, _ := psRepo.Create(context.Background(), resellerID, "PS", false)
	psItems.ReplaceItems(context.Background(), ps.ID,
		[]storage.ProviderSetItemInput{
			{ProviderID: provA, ExposeProviderName: true},
			{ProviderID: provB, ExposeProviderName: true},
		})
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, rsID, provB, 10, nil)
	pool.Exec(context.Background(),
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id) VALUES ($1, $2, $3)
		 ON CONFLICT (client_id) DO UPDATE SET provider_set_id=EXCLUDED.provider_set_id, route_set_id=EXCLUDED.route_set_id`,
		subID, ps.ID, rsID)

	mat := network.NewProviderSetMaterializer(pool)
	h := NewNetworkProviderSetItemsHandlers(pool, mat)

	// Пытаемся убрать provB из provider-set — он используется в route-set подписанного клиента
	body := fmt.Sprintf(`{"items":[{"provider_id":"%s","priority":1,"expose_provider_name":true}]}`, provA.String())
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+ps.ID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": ps.ID.String()})
	req = req.WithContext(middleware.WithClientID(req.Context(), resellerID))
	w := httptest.NewRecorder()
	h.PutItems(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), "provider_used_in_routes")
}
```

- [ ] **Step 14b.2: Изменить `network_providers.go` Delete handler**

В существующей `Delete` функции (Plan 1 task 6) после двух existing checks (`usedInSets`, `usedInClients`) добавить:

```go
// Used in this reseller's route-set items?
var usedInRouteItems int
h.pool.QueryRow(r.Context(), `
	SELECT count(*) FROM reseller_route_set_items i
	  JOIN reseller_route_sets rs ON rs.id = i.set_id
	 WHERE rs.reseller_id = $1 AND i.provider_id = $2`,
	resellerID, id).Scan(&usedInRouteItems)
if usedInRouteItems > 0 {
	respondError(w, shared.ErrConflict(fmt.Sprintf("используется в %d правилах route-set", usedInRouteItems)).
		WithDetails(`{"kind":"provider_used_in_routes","scope":"route_set"}`))
	return
}

// Used in override routes of this reseller's sub-accounts?
var usedInOverrides int
h.pool.QueryRow(r.Context(), `
	SELECT count(*) FROM client_routes cr
	  JOIN clients c ON c.id = cr.client_id
	 WHERE c.parent_client_id = $1 AND cr.provider_id = $2 AND cr.source = 'override'`,
	resellerID, id).Scan(&usedInOverrides)
if usedInOverrides > 0 {
	respondError(w, shared.ErrConflict(fmt.Sprintf("используется в %d override-маршрутах", usedInOverrides)).
		WithDetails(`{"kind":"provider_used_in_routes","scope":"override"}`))
	return
}
```

- [ ] **Step 14b.3: Изменить `network_provider_set_items.go` PutItems handler**

В `PutItems` после parsing нового списка items, перед `ReplaceItems`, добавить diff-проверку:

```go
// Diff: какие provider_id будут удалены этим PUT
existingItems, err := h.itemsRepo.ListBySet(r.Context(), setID)
if err != nil { respondError(w, shared.ErrInternalServer("list existing")); return }
newProviders := map[uuid.UUID]struct{}{}
for _, it := range req.Items {
	pid, _ := uuid.Parse(it.ProviderID)
	newProviders[pid] = struct{}{}
}
var removingIDs []uuid.UUID
for _, it := range existingItems {
	if _, kept := newProviders[it.ProviderID]; !kept {
		removingIDs = append(removingIDs, it.ProviderID)
	}
}

// Для каждого удаляемого — проверить usage в route-set'ах подписанных + override-маршрутах
type conflict struct {
	ProviderID string `json:"provider_id"`
	ClientID   string `json:"client_id"`
	ClientName string `json:"client_name"`
	RouteName  string `json:"route_name"`
	Source     string `json:"source"` // "route_set_item" | "override"
}
var conflicts []conflict
for _, rid := range removingIDs {
	// route-set items, через subscribers этого provider-set
	rows, err := h.pool.Query(r.Context(), `
		SELECT $1::uuid, sra.client_id, COALESCE(c.name, c.email),
		       COALESCE(i.name, '(без имени)'), 'route_set_item'
		FROM subaccount_routing_assignment sra
		JOIN clients c ON c.id = sra.client_id
		JOIN reseller_route_set_items i ON i.set_id = sra.route_set_id
		WHERE sra.provider_set_id = $2 AND i.provider_id = $1`,
		rid, setID)
	if err == nil {
		for rows.Next() {
			var c conflict
			rows.Scan(&c.ProviderID, &c.ClientID, &c.ClientName, &c.RouteName, &c.Source)
			conflicts = append(conflicts, c)
		}
		rows.Close()
	}
	// override-routes у subscribers этого provider-set
	rows2, err := h.pool.Query(r.Context(), `
		SELECT $1::uuid, sra.client_id, COALESCE(c.name, c.email),
		       COALESCE(cr.name, '(override)'), 'override'
		FROM subaccount_routing_assignment sra
		JOIN clients c ON c.id = sra.client_id
		JOIN client_routes cr ON cr.client_id = sra.client_id
		WHERE sra.provider_set_id = $2 AND cr.provider_id = $1 AND cr.source = 'override'`,
		rid, setID)
	if err == nil {
		for rows2.Next() {
			var c conflict
			rows2.Scan(&c.ProviderID, &c.ClientID, &c.ClientName, &c.RouteName, &c.Source)
			conflicts = append(conflicts, c)
		}
		rows2.Close()
	}
}
if len(conflicts) > 0 {
	details, _ := json.Marshal(map[string]interface{}{"kind": "provider_used_in_routes", "items": conflicts})
	respondError(w, shared.ErrConflict("удаляемые провайдеры используются в маршрутах подписанных суб-аккаунтов").
		WithDetails(string(details)))
	return
}

// далее — existing ReplaceItems + ApplyToAllSubscribers
```

- [ ] **Step 14b.4: Run tests + commit**

```
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test ./internal/gateway/portal/handlers/ -run 'TestProviders_Delete|TestProviderSetItems_PUT_409' -v"
git add internal/gateway/portal/handlers/network_providers.go internal/gateway/portal/handlers/network_providers_test.go internal/gateway/portal/handlers/network_provider_set_items.go internal/gateway/portal/handlers/network_provider_set_items_test.go
git commit -m "feat(handlers): provider DELETE + provider-set PUT items — route-conflict checks (Plan 2 Task 14b)"
```

---

## Phase E — Router wiring + legacy removal

### Task 15: Wire новых handlers + удалить legacy `/reseller/routing/*`

**Files:**
- Modify: `internal/gateway/portal/router/router.go`
- Modify: `cmd/portal-gateway/main.go`
- Delete: `internal/gateway/portal/handlers/reseller_routing.go`
- Delete: `internal/gateway/portal/handlers/reseller_routing_test.go` (если существует)

- [ ] **Step 15.1: Расширить `SetupRouter` сигнатуру**

Добавить в параметры:
```go
networkRouteSetsHandlers *handlers.NetworkRouteSetsHandlers,
networkRouteSetItemsHandlers *handlers.NetworkRouteSetItemsHandlers,
networkRoutePreviewHandlers *handlers.NetworkRoutePreviewHandlers,
networkCleanupHandlers *handlers.NetworkCleanupHandlers,
```

Удалить параметр `resellerRoutingHandlers *handlers.ResellerRoutingHandlers` — больше не используется.

- [ ] **Step 15.2: Зарегистрировать новые routes + удалить legacy block**

В блоке `network := reseller.PathPrefix("/network").Subrouter()` (router.go:502) добавить **после** registrations provider-sets + assignments:

```go
// Route-sets CRUD
network.HandleFunc("/route-sets", networkRouteSetsHandlers.List).Methods("GET")
network.HandleFunc("/route-sets", networkRouteSetsHandlers.Create).Methods("POST")
network.HandleFunc("/route-sets/{id}", networkRouteSetsHandlers.Update).Methods("PUT")
network.HandleFunc("/route-sets/{id}", networkRouteSetsHandlers.Delete).Methods("DELETE")

// Items: специфичные пути ПЕРЕД /{item_id}
network.HandleFunc("/route-sets/{id}/items/reorder", networkRouteSetItemsHandlers.Reorder).Methods("PUT")
network.HandleFunc("/route-sets/{id}/items/{item_id}/duplicate", networkRouteSetItemsHandlers.Duplicate).Methods("POST")
network.HandleFunc("/route-sets/{id}/items/{item_id}", networkRouteSetItemsHandlers.Update).Methods("PUT")
network.HandleFunc("/route-sets/{id}/items/{item_id}", networkRouteSetItemsHandlers.Delete).Methods("DELETE")
network.HandleFunc("/route-sets/{id}/items", networkRouteSetItemsHandlers.List).Methods("GET")
network.HandleFunc("/route-sets/{id}/items", networkRouteSetItemsHandlers.Create).Methods("POST")

// Preview
network.HandleFunc("/route-sets/{id}/preview", networkRoutePreviewHandlers.Preview).Methods("POST")

// Bulk dry-run + cleanup. /assignments/bulk/dry-run и /assignments/bulk оба должны быть до /assignments/{client_id}.
network.HandleFunc("/assignments/bulk/dry-run", networkAssignmentsHandlers.BulkDryRun).Methods("POST")
network.HandleFunc("/route-cleanup", networkCleanupHandlers.RouteCleanup).Methods("POST")
```

**Удалить** блок (router.go:522-526):
```
// Reseller routing overview
resellerRouting := reseller.PathPrefix("/routing").Subrouter()
resellerRouting.HandleFunc("/providers", resellerRoutingHandlers.ListNetworkProviders).Methods("GET")
resellerRouting.HandleFunc("/routes", resellerRoutingHandlers.ListNetworkRoutes).Methods("GET")
resellerRouting.HandleFunc("/bulk-assign", resellerRoutingHandlers.BulkAssignProvider).Methods("POST")
```

В `subAccounts` блоке (router.go:202-207) добавить override-routes endpoints **ПЕРЕД** existing provider-overrides registrations (специфичные пути с `{route_id}` идут первыми):

```go
subAccounts.HandleFunc("/{id}/network/route-overrides/{route_id}", subAccountNetworkOverridesHandlers.UpdateRouteOverride).Methods("PUT")
subAccounts.HandleFunc("/{id}/network/route-overrides/{route_id}", subAccountNetworkOverridesHandlers.DeleteRouteOverride).Methods("DELETE")
subAccounts.HandleFunc("/{id}/network/route-overrides", subAccountNetworkOverridesHandlers.AddRouteOverride).Methods("POST")
```

- [ ] **Step 15.3: Modify `cmd/portal-gateway/main.go`**

Добавить после существующих constructor-вызовов (после `subAccountNetworkOverridesHandlers := ...`):
```go
routeSetItemsRepo := storage.NewResellerRouteSetItemsRepository(dbPool)
routeMaterializer := network.NewRouteSetMaterializer(dbPool, routeSetItemsRepo)
conflictValidator := network.NewConflictValidator(
	storage.NewResellerProviderSetItemsRepository(dbPool),
	routeSetItemsRepo,
)
networkRouteSetsHandlers := handlers.NewNetworkRouteSetsHandlers(dbPool, routeMaterializer)
networkRouteSetItemsHandlers := handlers.NewNetworkRouteSetItemsHandlers(dbPool, routeMaterializer, conflictValidator)
networkRoutePreviewHandlers := handlers.NewNetworkRoutePreviewHandlers(dbPool)
networkCleanupHandlers := handlers.NewNetworkCleanupHandlers(dbPool, routeMaterializer)
```

Обновить `NewNetworkAssignmentsHandlers(...)` вызов на новую сигнатуру с 4 параметрами:
```go
networkAssignmentsHandlers := handlers.NewNetworkAssignmentsHandlers(dbPool, materializer, routeMaterializer, conflictValidator)
```
(где `materializer` — это `network.NewProviderSetMaterializer(dbPool)`, существующий из Plan 1)

В `router.SetupRouter(...)` вызове:
- удалить `resellerRoutingHandlers,`
- добавить `networkRouteSetsHandlers, networkRouteSetItemsHandlers, networkRoutePreviewHandlers, networkCleanupHandlers,` в нужном порядке

Удалить строку:
```go
resellerRoutingHandlers := handlers.NewResellerRoutingHandlers(dbPool, serviceClients.RoutingClient)
```

- [ ] **Step 15.4: Удалить legacy handler-файлы**

```
git rm internal/gateway/portal/handlers/reseller_routing.go
# если существует:
git rm internal/gateway/portal/handlers/reseller_routing_test.go 2>/dev/null || true
```

- [ ] **Step 15.5: Build + tests на сервере**

```
./scripts/server.sh exec "cd /opt/sms && go vet ./... && go build ./..."
./scripts/server.sh exec "cd /opt/sms && TEST_DATABASE_URL=postgres://smpp:smpp_password@localhost:5432/smpp_db go test -short ./internal/..."
```
Expected: clean vet/build, all tests pass.

- [ ] **Step 15.6: Commit**

```bash
git add internal/gateway/portal/router/router.go cmd/portal-gateway/main.go
git rm internal/gateway/portal/handlers/reseller_routing.go
git commit -m "feat(routing): wire route-sets/preview/cleanup + remove legacy /reseller/routing/* (Plan 2 Task 15)"
```

---

## Phase F — Frontend

### Task 16: networkApi types + extensions

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 16.1: Добавить новые типы и расширить existing types в `client.ts`**

После `interface NetworkSubAccountOverview` добавить:
```ts
export interface RouteCondition {
  type: 'operator' | 'country' | 'traffic_type' | 'paid_name' | 'regex';
  value: string;
}

export interface RouteConditionGroup {
  logic_op: 'IF' | 'AND' | 'AND_NOT' | 'OR' | 'OR_NOT';
  conditions: RouteCondition[];
}

export interface RouteSchedule {
  date_from?: string | null;
  date_to?: string | null;
  time_from?: string | null;
  time_to?: string | null;
  weekdays: number;
  timezone: string;
}

export interface NetworkRouteSet {
  id: string;
  name: string;
  is_default: boolean;
  item_count: number;
  assigned_count: number;
}

export interface NetworkRouteSetItem {
  id: string;
  name: string;
  comment: string;
  provider_id: string;
  provider_name: string;
  priority: number;
  share: number;
  route_type: string;
  status: 'active' | 'inactive';
  condition_groups: RouteConditionGroup[];
  schedules: RouteSchedule[];
}

export interface RoutePreviewMatch {
  matched_item_id: string;
  item_name: string;
  provider_id: string;
  provider_name: string;
  priority: number;
}

export interface NetworkRouteOverride {
  id: string;
  name: string;
  provider_id: string;
  provider_name: string;
  priority: number;
  status: string;
}
```

Расширить `NetworkSubAccountOverview` (заменить `route_set: null` и `route_overrides: unknown[]`):
```ts
export interface NetworkSubAccountOverview {
  provider_set: { id: string; name: string } | null;
  route_set: { id: string; name: string } | null;
  provider_overrides: NetworkProviderOverride[];
  route_overrides: NetworkRouteOverride[];
}
```

Добавить в `NetworkBulkAssignResult`:
```ts
export interface NetworkBulkAssignResult {
  client_id: string;
  status: 'ok' | 'error' | 'conflict';
  error?: string;
}
```

Расширить `NetworkAssignment`:
```ts
export interface NetworkAssignment {
  client_id: string;
  sub_account_name: string;
  provider_set_id: string | null;
  provider_set_name: string | null;
  route_set_id: string | null;
  route_set_name: string | null;
  has_overrides: boolean;
  validation_status: 'ok' | 'unassigned' | 'conflict';
  validation_error?: string;
}
```

Добавить в `networkApi` (после `deleteProviderOverride`):
```ts
  // ── Route Sets ─────────────────────────────────────────────────────────────

  listRouteSets: () =>
    apiFetch<{ route_sets: NetworkRouteSet[] }>('/reseller/network/route-sets'),
  createRouteSet: (data: { name: string; is_default?: boolean }) =>
    apiFetch<{ id: string; name: string; is_default: boolean }>(
      '/reseller/network/route-sets',
      { method: 'POST', body: JSON.stringify(data) },
    ),
  updateRouteSet: (id: string, data: { name: string; is_default: boolean }) =>
    apiFetch<{ id: string }>(`/reseller/network/route-sets/${id}`, {
      method: 'PUT', body: JSON.stringify(data),
    }),
  deleteRouteSet: (id: string) =>
    apiFetch<void>(`/reseller/network/route-sets/${id}`, { method: 'DELETE' }),

  // ── Route Set Items ────────────────────────────────────────────────────────

  listRouteSetItems: (setId: string) =>
    apiFetch<{ items: NetworkRouteSetItem[] }>(
      `/reseller/network/route-sets/${setId}/items`,
    ),
  createRouteSetItem: (setId: string, data: Omit<NetworkRouteSetItem, 'id' | 'provider_name'>) =>
    apiFetch<{ id: string }>(`/reseller/network/route-sets/${setId}/items`, {
      method: 'POST', body: JSON.stringify(data),
    }),
  updateRouteSetItem: (setId: string, itemId: string, data: Omit<NetworkRouteSetItem, 'id' | 'provider_name'>) =>
    apiFetch<{ id: string }>(`/reseller/network/route-sets/${setId}/items/${itemId}`, {
      method: 'PUT', body: JSON.stringify(data),
    }),
  deleteRouteSetItem: (setId: string, itemId: string) =>
    apiFetch<void>(`/reseller/network/route-sets/${setId}/items/${itemId}`, {
      method: 'DELETE',
    }),
  duplicateRouteSetItem: (setId: string, itemId: string) =>
    apiFetch<{ id: string }>(`/reseller/network/route-sets/${setId}/items/${itemId}/duplicate`, {
      method: 'POST',
    }),
  reorderRouteSetItems: (setId: string, items: Array<{ item_id: string; priority: number }>) =>
    apiFetch<{ set_id: string }>(`/reseller/network/route-sets/${setId}/items/reorder`, {
      method: 'PUT', body: JSON.stringify({ items }),
    }),

  // ── Preview ────────────────────────────────────────────────────────────────

  previewRouteSet: (setId: string, data: { phone: string; sender_id: string; traffic_type: string }) =>
    apiFetch<{ matches: RoutePreviewMatch[] }>(
      `/reseller/network/route-sets/${setId}/preview`,
      { method: 'POST', body: JSON.stringify(data) },
    ),

  // ── Bulk Dry Run ───────────────────────────────────────────────────────────

  bulkAssignDryRun: (data: {
    client_ids: string[];
    provider_set_id: string | null;
    route_set_id?: string | null;
  }) =>
    apiFetch<{ results: NetworkBulkAssignResult[] }>(
      '/reseller/network/assignments/bulk/dry-run',
      { method: 'POST', body: JSON.stringify(data) },
    ),

  // ── Cleanup ────────────────────────────────────────────────────────────────

  routeCleanup: (data: { provider_id: string }) =>
    apiFetch<{ removed_route_set_items: number; removed_overrides: number }>(
      '/reseller/network/route-cleanup',
      { method: 'POST', body: JSON.stringify(data) },
    ),

  // ── Sub-account route overrides ────────────────────────────────────────────

  addRouteOverride: (subAccountId: string, data: Omit<NetworkRouteSetItem, 'id' | 'provider_name'>) =>
    apiFetch<{ id: string }>(
      `/sub-accounts/${subAccountId}/network/route-overrides`,
      { method: 'POST', body: JSON.stringify(data) },
    ),
  updateRouteOverride: (subAccountId: string, routeId: string, data: Omit<NetworkRouteSetItem, 'id' | 'provider_name'>) =>
    apiFetch<{ id: string }>(
      `/sub-accounts/${subAccountId}/network/route-overrides/${routeId}`,
      { method: 'PUT', body: JSON.stringify(data) },
    ),
  deleteRouteOverride: (subAccountId: string, routeId: string) =>
    apiFetch<void>(
      `/sub-accounts/${subAccountId}/network/route-overrides/${routeId}`,
      { method: 'DELETE' },
    ),
};
```

(Закрывающая `}` уже есть в существующем networkApi — заменить её на новый блок с дополнительными методами.)

- [ ] **Step 16.2: tsc + eslint**

```
cd portal-frontend && npx tsc --noEmit && npx eslint . --max-warnings=69
```

- [ ] **Step 16.3: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat(api): networkApi — route-sets, items, preview, overrides, cleanup (Plan 2 Task 16)"
```

---

### Task 17: `RouteConditionsEditor` + `RouteSchedulesEditor` + `RouteRuleDrawer` компоненты

**Files:**
- Create: `portal-frontend/src/components/network/RouteConditionsEditor.tsx`
- Create: `portal-frontend/src/components/network/RouteSchedulesEditor.tsx`
- Create: `portal-frontend/src/components/network/RouteRuleDrawer.tsx`

`RouteRuleDrawer` — общий компонент для route-set items и override-маршрутов в карточке суб-аккаунта.

- [ ] **Step 17.1: Реализация `RouteConditionsEditor.tsx`**

```tsx
import type { RouteCondition, RouteConditionGroup } from '../../api/client';

const TYPES: Array<RouteCondition['type']> = ['operator', 'country', 'traffic_type', 'paid_name', 'regex'];
const LOGIC_OPS: Array<RouteConditionGroup['logic_op']> = ['IF', 'AND', 'AND_NOT', 'OR', 'OR_NOT'];

interface Props {
  groups: RouteConditionGroup[];
  onChange: (groups: RouteConditionGroup[]) => void;
}

export function RouteConditionsEditor({ groups, onChange }: Props) {
  const addGroup = () => {
    onChange([...groups, { logic_op: 'IF', conditions: [{ type: 'country', value: '' }] }]);
  };
  const removeGroup = (idx: number) => {
    onChange(groups.filter((_, i) => i !== idx));
  };
  const updateGroup = (idx: number, patch: Partial<RouteConditionGroup>) => {
    onChange(groups.map((g, i) => (i === idx ? { ...g, ...patch } : g)));
  };
  const addCondition = (gIdx: number) => {
    updateGroup(gIdx, { conditions: [...groups[gIdx].conditions, { type: 'country', value: '' }] });
  };
  const removeCondition = (gIdx: number, cIdx: number) => {
    updateGroup(gIdx, { conditions: groups[gIdx].conditions.filter((_, i) => i !== cIdx) });
  };
  const updateCondition = (gIdx: number, cIdx: number, patch: Partial<RouteCondition>) => {
    const next = [...groups[gIdx].conditions];
    next[cIdx] = { ...next[cIdx], ...patch };
    updateGroup(gIdx, { conditions: next });
  };

  return (
    <div className="space-y-3">
      <div className="text-sm font-medium text-gray-700">Условия</div>
      {groups.length === 0 && (
        <div className="text-xs text-gray-400">Без условий — правило применяется ко всем сообщениям</div>
      )}
      {groups.map((g, gIdx) => (
        <div key={gIdx} className="border border-gray-200 rounded p-3">
          <div className="flex items-center gap-2 mb-2">
            <select
              value={g.logic_op}
              onChange={(e) => updateGroup(gIdx, { logic_op: e.target.value as RouteConditionGroup['logic_op'] })}
              className="text-xs border border-gray-300 rounded px-2 py-1"
            >
              {LOGIC_OPS.map((op) => <option key={op} value={op}>{op}</option>)}
            </select>
            <button onClick={() => removeGroup(gIdx)} className="text-xs text-red-600 hover:underline ml-auto">
              Удалить группу
            </button>
          </div>
          <div className="space-y-2">
            {g.conditions.map((c, cIdx) => (
              <div key={cIdx} className="flex items-center gap-2">
                <select
                  value={c.type}
                  onChange={(e) => updateCondition(gIdx, cIdx, { type: e.target.value as RouteCondition['type'] })}
                  className="text-xs border border-gray-300 rounded px-2 py-1"
                >
                  {TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
                </select>
                <input
                  type="text"
                  value={c.value}
                  onChange={(e) => updateCondition(gIdx, cIdx, { value: e.target.value })}
                  className="flex-1 text-sm border border-gray-300 rounded px-2 py-1"
                  placeholder={c.type === 'country' ? 'RU, KZ, UZ ...' : c.type === 'regex' ? '^7.*' : ''}
                />
                <button onClick={() => removeCondition(gIdx, cIdx)} className="text-xs text-red-600 hover:underline">×</button>
              </div>
            ))}
            <button onClick={() => addCondition(gIdx)} className="text-xs text-blue-600 hover:underline">+ Условие</button>
          </div>
        </div>
      ))}
      <button onClick={addGroup} className="text-sm text-blue-600 hover:underline">+ Группа условий</button>
    </div>
  );
}
```

- [ ] **Step 17.2: Реализация `RouteSchedulesEditor.tsx`**

```tsx
import type { RouteSchedule } from '../../api/client';

const WEEKDAYS = [
  { mask: 1, label: 'Пн' },
  { mask: 2, label: 'Вт' },
  { mask: 4, label: 'Ср' },
  { mask: 8, label: 'Чт' },
  { mask: 16, label: 'Пт' },
  { mask: 32, label: 'Сб' },
  { mask: 64, label: 'Вс' },
];

interface Props {
  schedules: RouteSchedule[];
  onChange: (schedules: RouteSchedule[]) => void;
}

export function RouteSchedulesEditor({ schedules, onChange }: Props) {
  const add = () => {
    onChange([...schedules, { weekdays: 127, timezone: 'Europe/Moscow' }]);
  };
  const remove = (idx: number) => {
    onChange(schedules.filter((_, i) => i !== idx));
  };
  const update = (idx: number, patch: Partial<RouteSchedule>) => {
    onChange(schedules.map((s, i) => (i === idx ? { ...s, ...patch } : s)));
  };
  const toggleWeekday = (idx: number, mask: number) => {
    const cur = schedules[idx].weekdays;
    const next = (cur & mask) ? (cur & ~mask) : (cur | mask);
    update(idx, { weekdays: next });
  };

  return (
    <div className="space-y-3">
      <div className="text-sm font-medium text-gray-700">Расписания</div>
      {schedules.length === 0 && (
        <div className="text-xs text-gray-400">Без расписания — правило активно постоянно</div>
      )}
      {schedules.map((s, idx) => (
        <div key={idx} className="border border-gray-200 rounded p-3 space-y-2">
          <div className="flex items-center gap-2">
            <label className="text-xs text-gray-500">с</label>
            <input type="date" value={s.date_from || ''} onChange={(e) => update(idx, { date_from: e.target.value || null })} className="text-xs border rounded px-2 py-1" />
            <label className="text-xs text-gray-500">по</label>
            <input type="date" value={s.date_to || ''} onChange={(e) => update(idx, { date_to: e.target.value || null })} className="text-xs border rounded px-2 py-1" />
            <button onClick={() => remove(idx)} className="text-xs text-red-600 hover:underline ml-auto">Удалить</button>
          </div>
          <div className="flex items-center gap-2">
            <label className="text-xs text-gray-500">время с</label>
            <input type="time" value={s.time_from || ''} onChange={(e) => update(idx, { time_from: e.target.value || null })} className="text-xs border rounded px-2 py-1" />
            <label className="text-xs text-gray-500">по</label>
            <input type="time" value={s.time_to || ''} onChange={(e) => update(idx, { time_to: e.target.value || null })} className="text-xs border rounded px-2 py-1" />
          </div>
          <div className="flex items-center gap-1">
            {WEEKDAYS.map((d) => (
              <label key={d.mask} className="flex items-center gap-1 text-xs">
                <input
                  type="checkbox"
                  checked={(s.weekdays & d.mask) !== 0}
                  onChange={() => toggleWeekday(idx, d.mask)}
                />
                {d.label}
              </label>
            ))}
          </div>
          <input
            type="text"
            value={s.timezone}
            onChange={(e) => update(idx, { timezone: e.target.value })}
            placeholder="Europe/Moscow"
            className="text-xs border rounded px-2 py-1 w-48"
          />
        </div>
      ))}
      <button onClick={add} className="text-sm text-blue-600 hover:underline">+ Расписание</button>
    </div>
  );
}
```

- [ ] **Step 17.3: Реализация `RouteRuleDrawer.tsx`**

```tsx
import { useState, useEffect } from 'react';
import { Drawer } from '../ui/Drawer';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { useToast } from '../ui/Toast';
import { ApiError, type NetworkRouteSetItem, type NetworkProvider } from '../../api/client';
import { RouteConditionsEditor } from './RouteConditionsEditor';
import { RouteSchedulesEditor } from './RouteSchedulesEditor';

type RuleData = Omit<NetworkRouteSetItem, 'id' | 'provider_name'>;

const EMPTY: RuleData = {
  name: '',
  comment: '',
  provider_id: '',
  priority: 0,
  share: 100,
  route_type: 'sms',
  status: 'active',
  condition_groups: [],
  schedules: [],
};

interface Props {
  open: boolean;
  onClose: () => void;
  initial?: NetworkRouteSetItem | null;
  providers: NetworkProvider[];
  onSubmit: (data: RuleData) => Promise<void>;
  title: string;
}

export function RouteRuleDrawer({ open, onClose, initial, providers, onSubmit, title }: Props) {
  const toast = useToast();
  const [form, setForm] = useState<RuleData>(EMPTY);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      if (initial) {
        setForm({
          name: initial.name,
          comment: initial.comment,
          provider_id: initial.provider_id,
          priority: initial.priority,
          share: initial.share,
          route_type: initial.route_type,
          status: initial.status,
          condition_groups: initial.condition_groups,
          schedules: initial.schedules,
        });
      } else {
        setForm(EMPTY);
      }
    }
  }, [open, initial]);

  const submit = async () => {
    if (!form.provider_id) { toast.error('Выберите провайдера'); return; }
    setSubmitting(true);
    try {
      await onSubmit(form);
      onClose();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка сохранения');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Drawer open={open} onClose={onClose} title={title} width="lg">
      <div className="space-y-4">
        <Input label="Имя" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
        <Input label="Комментарий" value={form.comment} onChange={(e) => setForm({ ...form, comment: e.target.value })} />

        <div>
          <label className="block text-sm text-gray-600 mb-1">Провайдер</label>
          <select
            value={form.provider_id}
            onChange={(e) => setForm({ ...form, provider_id: e.target.value })}
            className="w-full border border-gray-300 rounded px-3 py-2 text-sm"
          >
            <option value="">— выбрать —</option>
            {providers.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.ownership === 'private' ? 'свой' : 'платформа'})
              </option>
            ))}
          </select>
        </div>

        <div className="grid grid-cols-3 gap-3">
          <Input label="Приоритет" type="number" value={String(form.priority)} onChange={(e) => setForm({ ...form, priority: parseInt(e.target.value) || 0 })} />
          <Input label="Доля %" type="number" value={String(form.share)} onChange={(e) => setForm({ ...form, share: parseInt(e.target.value) || 100 })} />
          <div>
            <label className="block text-sm text-gray-600 mb-1">Статус</label>
            <select value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value as 'active' | 'inactive' })} className="w-full border rounded px-3 py-2 text-sm">
              <option value="active">Активен</option>
              <option value="inactive">Выключен</option>
            </select>
          </div>
        </div>

        <RouteConditionsEditor
          groups={form.condition_groups}
          onChange={(groups) => setForm({ ...form, condition_groups: groups })}
        />

        <RouteSchedulesEditor
          schedules={form.schedules}
          onChange={(schedules) => setForm({ ...form, schedules })}
        />

        <div className="flex gap-2 pt-2">
          <Button onClick={submit} disabled={submitting}>{submitting ? 'Сохранение...' : 'Сохранить'}</Button>
          <Button variant="secondary" onClick={onClose}>Отмена</Button>
        </div>
      </div>
    </Drawer>
  );
}
```

- [ ] **Step 17.4: Build + tsc + eslint, commit**

```
cd portal-frontend && npx tsc --noEmit && npx eslint . --max-warnings=69
git add portal-frontend/src/components/network/RouteConditionsEditor.tsx portal-frontend/src/components/network/RouteSchedulesEditor.tsx portal-frontend/src/components/network/RouteRuleDrawer.tsx
git commit -m "feat(portal): RouteConditionsEditor + RouteSchedulesEditor + RouteRuleDrawer components (Plan 2 Task 17)"
```

---

### Task 18: `RouteSetsPage` — master-detail редактор с drag-reorder и preview

**Files:**
- Create: `portal-frontend/src/pages/network/RouteSetsPage.tsx`
- Modify: `portal-frontend/src/App.tsx` (route)
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx` (sidebar)

Структура: левая колонка — список set'ов; правая — заголовок с inline-rename, default-toggle, удаление; collapsible preview-блок; таблица правил с native HTML5 drag-handle для приоритета; «+ Правило» открывает `RouteRuleDrawer` (создание); клик по строке — RouteRuleDrawer (редактирование).

- [ ] **Step 18.1: Реализация `RouteSetsPage.tsx`**

```tsx
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { useToast } from '../../components/ui/Toast';
import { RouteRuleDrawer } from '../../components/network/RouteRuleDrawer';
import {
  networkApi, ApiError,
  type NetworkRouteSet, type NetworkRouteSetItem, type NetworkProvider, type RoutePreviewMatch,
} from '../../api/client';

export function RouteSetsPage() {
  const toast = useToast();
  const [sets, setSets] = useState<NetworkRouteSet[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [items, setItems] = useState<NetworkRouteSetItem[]>([]);
  const [providers, setProviders] = useState<NetworkProvider[]>([]);

  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createDefault, setCreateDefault] = useState(false);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<NetworkRouteSetItem | null>(null);

  const [confirmDelete, setConfirmDelete] = useState<NetworkRouteSet | null>(null);

  // Preview
  const [previewOpen, setPreviewOpen] = useState(false);
  const [previewForm, setPreviewForm] = useState({ phone: '', sender_id: '', traffic_type: 'transactional' });
  const [previewMatches, setPreviewMatches] = useState<RoutePreviewMatch[]>([]);

  const reloadSets = useCallback(
    () => networkApi.listRouteSets().then((r) => setSets(r.route_sets)).catch(() => toast.error('Ошибка')),
    [toast],
  );
  const reloadItems = useCallback(
    (id: string) => networkApi.listRouteSetItems(id).then((r) => setItems(r.items)),
    [],
  );

  useEffect(() => {
    reloadSets();
    networkApi.listProviders().then((r) => setProviders(r.providers || []));
  }, [reloadSets]);

  useEffect(() => {
    if (selectedID) reloadItems(selectedID);
  }, [selectedID, reloadItems]);

  const selected = sets.find((s) => s.id === selectedID);

  const create = async () => {
    if (!createName) return;
    try {
      const r = await networkApi.createRouteSet({ name: createName, is_default: createDefault });
      toast.success('Создан');
      setCreateOpen(false); setCreateName(''); setCreateDefault(false);
      await reloadSets();
      setSelectedID(r.id);
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };

  const renameSelected = async (newName: string) => {
    if (!selected) return;
    try {
      await networkApi.updateRouteSet(selected.id, { name: newName, is_default: selected.is_default });
      reloadSets();
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };

  const toggleDefault = async () => {
    if (!selected) return;
    try {
      await networkApi.updateRouteSet(selected.id, { name: selected.name, is_default: !selected.is_default });
      reloadSets();
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };

  const deleteSet = async () => {
    if (!confirmDelete) return;
    try {
      await networkApi.deleteRouteSet(confirmDelete.id);
      toast.success('Удалён'); setConfirmDelete(null);
      if (selectedID === confirmDelete.id) setSelectedID(null);
      reloadSets();
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); setConfirmDelete(null); }
  };

  const submitItem = async (data: Omit<NetworkRouteSetItem, 'id' | 'provider_name'>) => {
    if (!selected) return;
    if (editingItem) {
      await networkApi.updateRouteSetItem(selected.id, editingItem.id, data);
      toast.success('Обновлено');
    } else {
      await networkApi.createRouteSetItem(selected.id, data);
      toast.success('Создано');
    }
    reloadItems(selected.id);
    reloadSets();
  };

  const deleteItem = async (itemId: string) => {
    if (!selected) return;
    try {
      await networkApi.deleteRouteSetItem(selected.id, itemId);
      toast.success('Удалено');
      reloadItems(selected.id);
      reloadSets();
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };

  const duplicateItem = async (itemId: string) => {
    if (!selected) return;
    try {
      await networkApi.duplicateRouteSetItem(selected.id, itemId);
      toast.success('Дублировано');
      reloadItems(selected.id);
      reloadSets();
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };

  // Native HTML5 drag-and-drop reorder
  const [dragIdx, setDragIdx] = useState<number | null>(null);
  const onDragStart = (idx: number) => () => setDragIdx(idx);
  const onDragOver = (idx: number) => (e: React.DragEvent) => { e.preventDefault(); };
  const onDrop = (idx: number) => async (e: React.DragEvent) => {
    e.preventDefault();
    if (dragIdx === null || dragIdx === idx || !selected) return;
    const next = [...items];
    const [moved] = next.splice(dragIdx, 1);
    next.splice(idx, 0, moved);
    setItems(next);
    setDragIdx(null);
    // Recompute priorities: top of list = highest priority. Step = 10 для удобства.
    const reorderPayload = next.map((it, i) => ({ item_id: it.id, priority: (next.length - i) * 10 }));
    try {
      await networkApi.reorderRouteSetItems(selected.id, reorderPayload);
      reloadItems(selected.id);
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); reloadItems(selected.id); }
  };

  const runPreview = async () => {
    if (!selected) return;
    try {
      const r = await networkApi.previewRouteSet(selected.id, previewForm);
      setPreviewMatches(r.matches);
    } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
  };

  return (
    <div className="max-w-7xl">
      <PageHeader title="Route-sets" actions={<Button onClick={() => setCreateOpen(true)}>+ Создать</Button>} />
      <div className="grid grid-cols-12 gap-4">
        <div className="col-span-4 bg-white border border-gray-200 rounded-lg overflow-hidden">
          {sets.length === 0 ? (
            <div className="p-6 text-center text-gray-400 text-sm">Нет шаблонов</div>
          ) : sets.map((s) => (
            <div
              key={s.id}
              onClick={() => setSelectedID(s.id)}
              className={`p-3 border-b border-gray-100 cursor-pointer ${selectedID === s.id ? 'bg-blue-50' : 'hover:bg-gray-50'}`}
            >
              <div className="flex items-center justify-between">
                <span className="font-medium">{s.name}</span>
                {s.is_default && <span className="px-2 text-xs bg-amber-100 text-amber-700 rounded">default</span>}
              </div>
              <div className="text-xs text-gray-500 mt-1">{s.item_count} правил · {s.assigned_count} назначений</div>
            </div>
          ))}
        </div>

        <div className="col-span-8 bg-white border border-gray-200 rounded-lg p-4">
          {!selected ? (
            <div className="text-center text-gray-400 py-12">Выбери шаблон или создай новый</div>
          ) : (
            <>
              <div className="flex items-center justify-between mb-4">
                <Input
                  defaultValue={selected.name}
                  onBlur={(e) => { if (e.target.value !== selected.name) renameSelected(e.target.value); }}
                  className="text-lg font-semibold border-none focus:border-gray-300"
                />
                <div className="flex gap-3 items-center">
                  <label className="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={selected.is_default} onChange={toggleDefault} />
                    По умолчанию
                  </label>
                  <button onClick={() => setConfirmDelete(selected)} className="text-red-600 hover:underline text-sm">Удалить</button>
                </div>
              </div>

              <details className="mb-4 border border-gray-200 rounded">
                <summary className="cursor-pointer p-3 text-sm text-gray-600 hover:bg-gray-50" onClick={() => setPreviewOpen((v) => !v)}>
                  Превью маршрутизации
                </summary>
                <div className="p-3 space-y-2 border-t border-gray-100">
                  <div className="grid grid-cols-3 gap-2">
                    <Input label="Телефон" value={previewForm.phone} onChange={(e) => setPreviewForm({ ...previewForm, phone: e.target.value })} placeholder="79991234567" />
                    <Input label="Sender ID" value={previewForm.sender_id} onChange={(e) => setPreviewForm({ ...previewForm, sender_id: e.target.value })} />
                    <div>
                      <label className="block text-sm text-gray-600 mb-1">Traffic type</label>
                      <select value={previewForm.traffic_type} onChange={(e) => setPreviewForm({ ...previewForm, traffic_type: e.target.value })} className="w-full border rounded px-3 py-2 text-sm">
                        <option value="transactional">transactional</option>
                        <option value="promo">promo</option>
                        <option value="service">service</option>
                      </select>
                    </div>
                  </div>
                  <Button onClick={runPreview}>Симулировать</Button>
                  {previewMatches.length === 0 ? (
                    <div className="text-xs text-gray-400">Нет совпадений</div>
                  ) : (
                    <ul className="text-sm">
                      {previewMatches.map((m) => (
                        <li key={m.matched_item_id} className="border-l-2 border-blue-500 pl-2 mt-1">
                          <strong>{m.item_name}</strong> → {m.provider_name} (приоритет {m.priority})
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </details>

              <table className="w-full text-sm">
                <thead><tr className="text-xs text-gray-500 uppercase">
                  <th className="text-left p-2">№</th>
                  <th className="text-left p-2">Имя</th>
                  <th className="text-left p-2">Условия</th>
                  <th className="text-left p-2">Провайдер</th>
                  <th className="text-center p-2">Доля</th>
                  <th className="text-center p-2">Статус</th>
                  <th></th>
                </tr></thead>
                <tbody>{items.map((it, idx) => (
                  <tr
                    key={it.id}
                    draggable
                    onDragStart={onDragStart(idx)}
                    onDragOver={onDragOver(idx)}
                    onDrop={onDrop(idx)}
                    className="border-t border-gray-100 hover:bg-gray-50"
                  >
                    <td className="p-2 text-gray-400 cursor-move">⋮⋮ {idx + 1}</td>
                    <td className="p-2 cursor-pointer" onClick={() => { setEditingItem(it); setDrawerOpen(true); }}>{it.name || '—'}</td>
                    <td className="p-2 text-xs text-gray-500">
                      {it.condition_groups.length === 0 ? 'все' :
                        it.condition_groups.flatMap((g) => g.conditions.map((c) => `${c.type}=${c.value}`)).join(', ')}
                    </td>
                    <td className="p-2">{it.provider_name}</td>
                    <td className="p-2 text-center">{it.share}%</td>
                    <td className="p-2 text-center">
                      <span className={`text-xs px-2 py-0.5 rounded ${it.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>{it.status === 'active' ? 'активен' : 'выкл'}</span>
                    </td>
                    <td className="p-2 text-right">
                      <button onClick={() => duplicateItem(it.id)} className="text-blue-600 hover:underline text-xs mr-2">Дубль</button>
                      <button onClick={() => deleteItem(it.id)} className="text-red-600 hover:underline text-xs">Удалить</button>
                    </td>
                  </tr>
                ))}</tbody>
              </table>
              <Button variant="secondary" onClick={() => { setEditingItem(null); setDrawerOpen(true); }} className="mt-3">+ Правило</Button>
            </>
          )}
        </div>
      </div>

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="Создать route-set">
        <div className="space-y-3">
          <Input label="Имя" value={createName} onChange={(e) => setCreateName(e.target.value)} />
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={createDefault} onChange={(e) => setCreateDefault(e.target.checked)} />
            По умолчанию для новых суб-аккаунтов
          </label>
          <Button onClick={create}>Создать</Button>
        </div>
      </Modal>

      <RouteRuleDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        initial={editingItem}
        providers={providers}
        onSubmit={submitItem}
        title={editingItem ? 'Редактировать правило' : 'Новое правило'}
      />

      <ConfirmDialog
        open={!!confirmDelete}
        onClose={() => setConfirmDelete(null)}
        onConfirm={deleteSet}
        title="Удалить route-set?"
        message={`«${confirmDelete?.name}» будет удалён. Если назначен суб-аккаунтам — операция вернёт 409.`}
      />
    </div>
  );
}
```

- [ ] **Step 18.2: Route в App.tsx**

В блоке `<Route path="/network" ...>` добавить:
```tsx
<Route path="route-sets" element={<RouteSetsPage />} />
```
И импорт:
```tsx
import { RouteSetsPage } from './pages/network/RouteSetsPage';
```

- [ ] **Step 18.3: Sidebar в UserLayout.tsx**

Найти место `{ path: '/network/provider-sets', label: 'Provider-sets' },` (UserLayout.tsx:96) и **сразу после** добавить:
```tsx
{ path: '/network/route-sets', label: 'Route-sets' },
```

- [ ] **Step 18.4: tsc + eslint + commit**

```
cd portal-frontend && npx tsc --noEmit && npx eslint . --max-warnings=69
git add portal-frontend/src/pages/network/RouteSetsPage.tsx portal-frontend/src/App.tsx portal-frontend/src/components/layout/UserLayout.tsx
git commit -m "feat(portal): /network/route-sets — master-detail editor + drag-reorder + preview (Plan 2 Task 18)"
```

---

### Task 19: Расширить `AssignmentsPage` route-set колонкой + dry-run + конфликт-modal

**Files:**
- Modify: `portal-frontend/src/pages/network/AssignmentsPage.tsx`

Изменения:
1. Загружать `route_sets` через `networkApi.listRouteSets()`.
2. Новая колонка «Route-set» с inline-select.
3. Bulk-panel: dropdown «Назначить route-set» + dry-run (запускается автоматически на выбор).
4. Validation колонка: показать `validation_status='conflict'` с tooltip из `validation_error`.
5. При bulk применении с конфликтами — modal со списком конфликтных клиентов и опцией «Всё равно применить».

- [ ] **Step 19.1: Изменения в `AssignmentsPage.tsx`**

```tsx
// Дополнить state:
const [routeSets, setRouteSets] = useState<NetworkRouteSet[]>([]);
const [bulkRouteSetID, setBulkRouteSetID] = useState('');
const [dryRunResults, setDryRunResults] = useState<NetworkBulkAssignResult[]>([]);
const [confirmConflict, setConfirmConflict] = useState(false);

// Дополнить useEffect:
useEffect(() => {
  reload();
  networkApi.listProviderSets().then((r) => setProviderSets(r.provider_sets));
  networkApi.listRouteSets().then((r) => setRouteSets(r.route_sets));
}, []);

// Расширить setOne — принимать route_set_id:
const setOne = async (clientID: string, providerSetID: string | null, routeSetID: string | null) => {
  try {
    await networkApi.putAssignment(clientID, { provider_set_id: providerSetID, route_set_id: routeSetID });
    toast.success('Назначено');
    reload();
  } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
};

// Dry-run автоматически при изменении выбора set'ов в bulk-panel:
useEffect(() => {
  if (selected.size === 0 || (!bulkSetID && !bulkRouteSetID)) {
    setDryRunResults([]);
    return;
  }
  networkApi.bulkAssignDryRun({
    client_ids: Array.from(selected),
    provider_set_id: bulkSetID || null,
    route_set_id: bulkRouteSetID || null,
  }).then((r) => setDryRunResults(r.results)).catch(() => {});
}, [selected, bulkSetID, bulkRouteSetID]);

// bulkApply — проверить наличие конфликтов; если есть — modal:
const bulkApply = async () => {
  const conflicts = dryRunResults.filter((r) => r.status === 'conflict');
  if (conflicts.length > 0 && !confirmConflict) {
    setConfirmConflict(true);
    return;
  }
  setConfirmConflict(false);
  try {
    const result = await networkApi.bulkAssign({
      client_ids: Array.from(selected),
      provider_set_id: bulkSetID || null,
      route_set_id: bulkRouteSetID || null,
    });
    const ok = result.results.filter((r) => r.status === 'ok').length;
    const conflictCount = result.results.filter((r) => r.status === 'conflict').length;
    toast.success(`Назначено: ${ok} из ${selected.size}; конфликтов: ${conflictCount}`);
    setSelected(new Set());
    setBulkRouteSetID(''); setBulkSetID('');
    reload();
  } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
};

// JSX расширения:
// 1) Bulk-panel — добавить второй select:
{selected.size > 0 && (
  <div className="sticky top-0 bg-blue-50 border border-blue-200 rounded p-3 mb-4 flex items-center gap-3 z-10 flex-wrap">
    <span className="font-medium">Выбрано: {selected.size}</span>
    <select value={bulkSetID} onChange={(e) => setBulkSetID(e.target.value)} className="border rounded px-2 py-1 text-sm">
      <option value="">— provider-set —</option>
      {providerSets.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
    </select>
    <select value={bulkRouteSetID} onChange={(e) => setBulkRouteSetID(e.target.value)} className="border rounded px-2 py-1 text-sm">
      <option value="">— route-set —</option>
      {routeSets.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
    </select>
    {dryRunResults.filter((r) => r.status === 'conflict').length > 0 && (
      <span className="text-xs text-amber-700">
        Прогноз: {dryRunResults.filter((r) => r.status === 'conflict').length} конфликт(а)
      </span>
    )}
    <Button onClick={bulkApply}>Применить</Button>
    <Button variant="secondary" onClick={() => setSelected(new Set())}>Отменить</Button>
  </div>
)}

// 2) Таблица — добавить колонку Route-set + изменить validation cell:
<thead>
  <tr className="bg-gray-50 text-gray-500 text-xs uppercase">
    <th className="p-3">{/* checkbox */}</th>
    <th className="text-left p-3">Суб-аккаунт</th>
    <th className="text-left p-3">Provider-set</th>
    <th className="text-left p-3">Route-set</th>
    <th className="text-center p-3">Override</th>
    <th className="text-center p-3">Статус</th>
  </tr>
</thead>
<tbody>{filtered.map((a) => (
  <tr key={a.client_id} className="border-t border-gray-100">
    <td className="p-3 text-center"><input type="checkbox" checked={selected.has(a.client_id)} onChange={() => toggle(a.client_id)} /></td>
    <td className="p-3"><a href={`/network/sub-accounts/${a.client_id}`} className="text-blue-600 hover:underline">{a.sub_account_name}</a></td>
    <td className="p-3">
      <select value={a.provider_set_id || ''} onChange={(e) => setOne(a.client_id, e.target.value || null, a.route_set_id)} className="border rounded px-2 py-1 text-sm">
        <option value="">—</option>
        {providerSets.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
      </select>
    </td>
    <td className="p-3">
      <select value={a.route_set_id || ''} onChange={(e) => setOne(a.client_id, a.provider_set_id, e.target.value || null)} className="border rounded px-2 py-1 text-sm">
        <option value="">—</option>
        {routeSets.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
      </select>
    </td>
    <td className="p-3 text-center">{a.has_overrides ? <span className="text-amber-700 text-xs">override</span> : '—'}</td>
    <td className="p-3 text-center">
      {a.validation_status === 'ok' ? '✓' :
       a.validation_status === 'unassigned' ? '—' :
       <span title={a.validation_error || 'конфликт'} className="text-amber-600 cursor-help">⚠</span>}
    </td>
  </tr>
))}</tbody>

// 3) Confirm-modal для конфликтов в bulk:
<Modal open={confirmConflict} onClose={() => setConfirmConflict(false)} title="Найдены конфликты">
  <div className="space-y-3">
    <p className="text-sm">У {dryRunResults.filter((r) => r.status === 'conflict').length} клиентов provider-set не содержит провайдеров из route-set. Эти клиенты будут пропущены.</p>
    <ul className="text-xs text-gray-600 max-h-40 overflow-y-auto">
      {dryRunResults.filter((r) => r.status === 'conflict').map((r) => (
        <li key={r.client_id}>{r.client_id.slice(0, 8)}... — {r.error}</li>
      ))}
    </ul>
    <div className="flex gap-2">
      <Button onClick={bulkApply}>Всё равно применить</Button>
      <Button variant="secondary" onClick={() => setConfirmConflict(false)}>Отмена</Button>
    </div>
  </div>
</Modal>
```

Импорты:
```tsx
import { Modal } from '../../components/ui/Modal';
import type { NetworkRouteSet, NetworkBulkAssignResult } from '../../api/client';
```

- [ ] **Step 19.2: tsc + eslint + commit**

```
cd portal-frontend && npx tsc --noEmit && npx eslint . --max-warnings=69
git add portal-frontend/src/pages/network/AssignmentsPage.tsx
git commit -m "feat(portal): AssignmentsPage — route-set column + dry-run + conflict modal (Plan 2 Task 19)"
```

---

### Task 20: Расширить `SubAccountNetworkSection` блоком route-set + override-маршрутами

**Files:**
- Modify: `portal-frontend/src/pages/sub-accounts/SubAccountNetworkSection.tsx`

Добавить:
1. Блок «Route-set» (по аналогии с провайдерским) — текущий + кнопка «Изменить».
2. Блок «Кастомные маршруты» — таблица override-routes + «+ Добавить» открывает `RouteRuleDrawer`.

- [ ] **Step 20.1: Изменения**

```tsx
// Дополнить state:
const [routeSets, setRouteSets] = useState<NetworkRouteSet[]>([]);
const [changeRouteSetOpen, setChangeRouteSetOpen] = useState(false);
const [newRouteSetID, setNewRouteSetID] = useState<string>('');
const [savingRouteSet, setSavingRouteSet] = useState(false);

const [routeDrawerOpen, setRouteDrawerOpen] = useState(false);
const [editingRouteOverride, setEditingRouteOverride] = useState<NetworkRouteSetItem | null>(null);

// Дополнить useEffect (load route-sets):
useEffect(() => {
  // существующий + добавить:
  networkApi.listRouteSets().then((r) => setRouteSets(r.route_sets || [])).catch(() => {});
}, []);

const changeRouteSet = async () => {
  setSavingRouteSet(true);
  try {
    await networkApi.putAssignment(subAccountID, {
      provider_set_id: overview?.provider_set?.id || null,
      route_set_id: newRouteSetID || null,
    });
    toast.success('Изменено');
    setChangeRouteSetOpen(false);
    loadOverview();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Ошибка');
  } finally { setSavingRouteSet(false); }
};

// override-route handlers:
const submitRouteOverride = async (data: Omit<NetworkRouteSetItem, 'id' | 'provider_name'>) => {
  if (editingRouteOverride) {
    await networkApi.updateRouteOverride(subAccountID, editingRouteOverride.id, data);
    toast.success('Обновлён');
  } else {
    await networkApi.addRouteOverride(subAccountID, data);
    toast.success('Создан');
  }
  loadOverview();
};

const removeRouteOverride = async (routeID: string) => {
  try {
    await networkApi.deleteRouteOverride(subAccountID, routeID);
    toast.success('Удалён');
    loadOverview();
  } catch (e) { toast.error(e instanceof ApiError ? e.message : 'Ошибка'); }
};

// JSX добавления (после блока «Provider-set», перед «Кастомные провайдеры»):
<div className="bg-white border border-gray-200 rounded-lg p-4">
  <div className="flex items-center justify-between mb-2">
    <h3 className="font-semibold">Route-set</h3>
    <Button variant="secondary" onClick={() => { setNewRouteSetID(overview?.route_set?.id || ''); setChangeRouteSetOpen(true); }}>
      Изменить
    </Button>
  </div>
  <div className="text-sm text-gray-600">{overview?.route_set?.name || 'Не назначен'}</div>
</div>

// Block Кастомные маршруты (после Кастомные провайдеры):
<div className="bg-white border border-gray-200 rounded-lg p-4">
  <div className="flex items-center justify-between mb-3">
    <h3 className="font-semibold">Кастомные маршруты (overrides)</h3>
    <Button variant="secondary" onClick={() => { setEditingRouteOverride(null); setRouteDrawerOpen(true); }}>+ Добавить</Button>
  </div>
  <p className="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded p-2 mb-3">
    Эти маршруты переопределяют шаблон. Изменения шаблона их не затрагивают.
  </p>
  {overview && overview.route_overrides.length === 0 ? (
    <div className="text-sm text-gray-400">Override-маршрутов нет</div>
  ) : (
    <table className="w-full text-sm">
      <thead><tr className="text-xs text-gray-500 uppercase">
        <th className="text-left p-2">Имя</th>
        <th className="text-left p-2">Провайдер</th>
        <th className="text-center p-2">Приоритет</th>
        <th className="text-center p-2">Статус</th>
        <th></th>
      </tr></thead>
      <tbody>{overview?.route_overrides.map((o) => (
        <tr key={o.id} className="border-t border-gray-100">
          <td className="p-2">{o.name || '—'}</td>
          <td className="p-2">{o.provider_name}</td>
          <td className="p-2 text-center">{o.priority}</td>
          <td className="p-2 text-center">{o.status}</td>
          <td className="p-2 text-right"><button onClick={() => removeRouteOverride(o.id)} className="text-red-600 hover:underline text-xs">Удалить</button></td>
        </tr>
      ))}</tbody>
    </table>
  )}
</div>

// Modal Route-set change:
<Modal open={changeRouteSetOpen} onClose={() => setChangeRouteSetOpen(false)} title={`Route-set для ${subAccountName}`}>
  <div className="space-y-3">
    <select value={newRouteSetID} onChange={(e) => setNewRouteSetID(e.target.value)} className="w-full border rounded px-3 py-2 text-sm">
      <option value="">— снять —</option>
      {routeSets.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
    </select>
    <Button onClick={changeRouteSet} disabled={savingRouteSet}>Применить</Button>
  </div>
</Modal>

// RouteRuleDrawer для override (доступные провайдеры — те, что в client_providers суб-аккаунта;
// получаем из overview.provider_set + provider_overrides + inherited через listProviders ResellerCatalog).
// Pragmatic: передаём весь providers list — backend отфильтрует через 409 если что-то не подходит.
<RouteRuleDrawer
  open={routeDrawerOpen}
  onClose={() => setRouteDrawerOpen(false)}
  initial={editingRouteOverride}
  providers={providers}
  onSubmit={submitRouteOverride}
  title={editingRouteOverride ? 'Редактировать override' : 'Новый override-маршрут'}
/>
```

Импорты:
```tsx
import { RouteRuleDrawer } from '../../components/network/RouteRuleDrawer';
import type { NetworkRouteSet, NetworkRouteSetItem } from '../../api/client';
```

- [ ] **Step 20.2: tsc + eslint + commit**

```
cd portal-frontend && npx tsc --noEmit && npx eslint . --max-warnings=69
git add portal-frontend/src/pages/sub-accounts/SubAccountNetworkSection.tsx
git commit -m "feat(portal): SubAccountNetworkSection — route-set block + override routes (Plan 2 Task 20)"
```

---

## Phase G — Cleanup dead code

### Task 21: Удалить legacy frontend и dead-code файлы

**Files:**
- Delete: `portal-frontend/src/pages/network/NetworkRoutingPage.tsx`
- Delete: `portal-frontend/src/components/layout/NetworkLayout.tsx`
- Delete: `e2e/pages/NetworkRoutingPage.ts` (если использовался только под legacy — verify через grep)
- Modify: `portal-frontend/src/App.tsx` (удалить импорт + редирект уже есть, проверить что не сломается)

- [ ] **Step 21.1: Verify usage**

```
grep -rn "NetworkRoutingPage\|NetworkLayout" portal-frontend/src/ e2e/
```

Допустимый результат:
- `NetworkRoutingPage.tsx` — только в самом файле (импорт удаляется в App.tsx)
- `NetworkLayout.tsx` — только в самом файле (sidebar давно в UserLayout)
- `e2e/pages/NetworkRoutingPage.ts` — если есть references в других spec'ах (из Plan 1 e2e), оставить и тоже modify ниже; иначе удалить

- [ ] **Step 21.2: Удалить файлы**

```
git rm portal-frontend/src/pages/network/NetworkRoutingPage.tsx
git rm portal-frontend/src/components/layout/NetworkLayout.tsx
# e2e: только если grep показал отсутствие использований:
git rm e2e/pages/NetworkRoutingPage.ts
```

Если в App.tsx остался unused-импорт `NetworkRoutingPage` — удалить.

- [ ] **Step 21.3: Build + tsc + eslint + commit**

```
cd portal-frontend && npx tsc --noEmit && npx eslint . --max-warnings=69
git add portal-frontend/src/App.tsx
git commit -m "chore(portal): remove dead NetworkRoutingPage + NetworkLayout (Plan 2 Task 21)"
```

---

## Phase H — E2E

### Task 22: Расширить e2e сценарии route-set'ами + override + bulk-conflict + preview

**Files:**
- Modify: `e2e/tests/reseller/network-routing.spec.ts`
- Create: `e2e/pages/RouteSetsPage.ts`

- [ ] **Step 22.1: Page-object `RouteSetsPage.ts`**

```ts
import type { Page } from '@playwright/test';

export class RouteSetsPage {
  constructor(private page: Page) {}
  async goto() { await this.page.goto('/network/route-sets'); }
  async create(name: string) {
    await this.page.click('button:has-text("+ Создать")');
    await this.page.fill('label:has-text("Имя") + input, input[type="text"]', name);
    await this.page.click('button:has-text("Создать")');
  }
  async select(name: string) { await this.page.click(`text=${name}`); }
  async addRule(opts: { name: string; provider: string; country?: string }) {
    await this.page.click('button:has-text("+ Правило")');
    await this.page.fill('label:has-text("Имя") + input', opts.name);
    await this.page.selectOption('select:near(:text("Провайдер"))', { label: new RegExp(opts.provider) });
    if (opts.country) {
      // Add condition
      await this.page.click('button:has-text("+ Группа условий")');
      await this.page.selectOption('select:near(:text("country"))', 'country');
      await this.page.fill('input[placeholder*="RU"]', opts.country);
    }
    await this.page.click('button:has-text("Сохранить")');
  }
  async expectRuleRow(name: string) {
    await this.page.locator(`tr:has-text("${name}")`).waitFor();
  }
  async runPreview(phone: string, sender: string, traffic: string) {
    await this.page.click('summary:has-text("Превью маршрутизации")');
    await this.page.fill('label:has-text("Телефон") + input', phone);
    await this.page.fill('label:has-text("Sender ID") + input', sender);
    await this.page.selectOption('select:near(:text("Traffic type"))', traffic);
    await this.page.click('button:has-text("Симулировать")');
  }
}
```

- [ ] **Step 22.2: Расширить spec**

```ts
// e2e/tests/reseller/network-routing.spec.ts (дополнить)
import { test, expect } from '@playwright/test';
import { loginAsAggregator } from '../../helpers/auth';
import { ProvidersCatalogPage } from '../../pages/ProvidersCatalogPage';
import { ProviderSetsPage } from '../../pages/ProviderSetsPage';
import { RouteSetsPage } from '../../pages/RouteSetsPage';
import { AssignmentsPage } from '../../pages/AssignmentsPage';

test.describe('Aggregator network routing — Plan 2', () => {
  test('full flow: provider-set + route-set + assign + verify', async ({ page }) => {
    await loginAsAggregator(page);

    // Создаём provider-set с провайдером
    const ps = new ProviderSetsPage(page);
    await ps.goto();
    await ps.create('E2E PS Plan2');
    await ps.addItemByName(/I-Digital|MaximusSMS/);

    // Создаём route-set с правилом на этого провайдера
    const rs = new RouteSetsPage(page);
    await rs.goto();
    await rs.create('E2E RS Plan2');
    await rs.addRule({ name: 'RU rule', provider: 'I-Digital', country: 'RU' });
    await rs.expectRuleRow('RU rule');

    // Назначаем оба set'а
    const a = new AssignmentsPage(page);
    await a.goto();
    await a.assignFirstSubAccount('E2E PS Plan2', 'E2E RS Plan2');
    await expect(page.locator('text=Назначено')).toBeVisible();
  });

  test('preview matches by country', async ({ page }) => {
    await loginAsAggregator(page);
    const rs = new RouteSetsPage(page);
    await rs.goto();
    await rs.select('E2E RS Plan2'); // создан в предыдущем тесте
    await rs.runPreview('79991234567', 'Test', 'transactional');
    await expect(page.locator('text=RU rule')).toBeVisible();
  });

  test('override route in sub-account card', async ({ page }) => {
    await loginAsAggregator(page);
    const a = new AssignmentsPage(page);
    await a.goto();
    await page.locator('a:has-text("QATestAudit")').first().click();
    await page.click('button:has-text("Сеть")'); // taby-button
    await page.click('h3:has-text("Кастомные маршруты") + * button:has-text("+ Добавить")');
    await page.fill('label:has-text("Имя") + input', 'override-rule');
    await page.selectOption('select:near(:text("Провайдер"))', { index: 1 });
    await page.click('button:has-text("Сохранить")');
    await expect(page.locator('tr:has-text("override-rule")')).toBeVisible();
  });

  test('bulk-assign with conflict shows modal', async ({ page }) => {
    await loginAsAggregator(page);
    // Этот тест требует preset state: provider-set "Empty", route-set с провайдером не из Empty.
    // Pragmatic: тест требует подготовки — ставим mark skip если data-prereq не выполняется.
    // Документируем reproducer вручную (CLI seeded test data — TODO Plan 3 fixtures).
    test.skip(true, 'requires conflict-preset; manual smoke covers this');
  });

  test('redirect from old /network/routing', async ({ page }) => {
    await loginAsAggregator(page);
    await page.goto('/network/routing');
    await expect(page).toHaveURL(/\/network\/assignments/);
  });
});
```

`AssignmentsPage.assignFirstSubAccount` — расширить page-object: принимает `providerSetName` и опционально `routeSetName`, выбирает оба inline-select'а первой строки таблицы.

- [ ] **Step 22.3: Запустить локально**

```
cd e2e && npx playwright test reseller/network-routing.spec.ts
```
Expected: PASS (skipped тест с conflict — допустим, документирован).

- [ ] **Step 22.4: Commit**

```bash
git add e2e/tests/reseller/network-routing.spec.ts e2e/pages/RouteSetsPage.ts e2e/pages/AssignmentsPage.ts
git commit -m "test(e2e): aggregator network Plan 2 — route-sets + preview + override (Plan 2 Task 22)"
```

---

## Phase I — Deploy + smoke

### Task 23: Deploy + полный smoke flow на сервере

**Files:** none (server-only)

- [ ] **Step 23.1: Push + deploy**

```
git push origin master
./scripts/server.sh migrate   # должен применить 133-136
./scripts/server.sh deploy
./scripts/server.sh status
```
Expected: контейнеры зелёные, 4 миграции применены.

- [ ] **Step 23.2: Полный flow smoke (через chrome-devtools-mcp или curl)**

Через curl + cookie session (см. Plan 1 smoke pattern):

1. **Login:**
```bash
./scripts/server.sh exec "curl -sX POST http://localhost:18085/portal/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{\"email\":\"aggregator@test.local\",\"password\":\"Admin123!\"}' \
  -c /tmp/cookies.txt"
```

2. **Создать route-set + правило:**
```bash
./scripts/server.sh exec "curl -sX POST http://localhost:18085/portal/v1/reseller/network/route-sets \
  -b /tmp/cookies.txt -H 'Content-Type: application/json' \
  -d '{\"name\":\"smoke-rs\",\"is_default\":false}'"
# captures route_set_id
```

3. **Назначить subaccount + verify materialised:**
```bash
./scripts/server.sh exec "psql -U smpp -d smpp_db -c \"
  SELECT count(*) FROM client_routes WHERE source='template';
  SELECT count(*) FROM route_condition_groups rcg
    JOIN client_routes cr ON cr.id=rcg.route_id WHERE cr.source='template';
\""
```
Expected: `>= 1` для обоих.

4. **Override-route:** добавить override через API; verify запись с `source='override'`.

5. **Удалить route-set с активным assignment:**
```bash
./scripts/server.sh exec "curl -sX DELETE http://localhost:18085/portal/v1/reseller/network/route-sets/<id> -b /tmp/cookies.txt -w '%{http_code}\n'"
```
Expected: `409`, response содержит `route_set_assigned`.

6. **Удалить provider используемый в route:** должно вернуть 409 с `kind: provider_used_in_routes` (или `used_in_provider_sets` — зависит от того что первое срабатывает).

7. **Bulk dry-run:** POST `/assignments/bulk/dry-run` с парой set'ов имеющих конфликт; ожидаем `status: 'conflict'` для каждого client_id.

8. **Cleanup-orphans:** POST `/route-cleanup` с provider_id; verify `removed_route_set_items > 0`.

9. **Cleanup test-data:** удалить созданные set'ы, override-маршруты.

- [ ] **Step 23.3: Verify legacy endpoints удалены**

```
./scripts/server.sh exec "curl -sX GET http://localhost:18085/portal/v1/reseller/routing/providers -b /tmp/cookies.txt -w '%{http_code}\n'"
```
Expected: `404` (route больше не зарегистрирован).

- [ ] **Step 23.4: Финальная отметка в DONE-документе**

Создать `docs/superpowers/plans/2026-05-04-aggregator-routing-plan-2-DONE.md` со списком всех 23 commit SHA + smoke-результат таблицей + open follow-ups (см. ниже).

```bash
git add docs/superpowers/plans/2026-05-04-aggregator-routing-plan-2-DONE.md
git commit -m "docs(plan): Plan 2 DONE — aggregator routing management complete"
git push origin master
```

---

## Open follow-ups (move to Plan 3 если придётся)

- **Orphan-assignment cleanup**: при `route_set_id IS NULL` после ON DELETE SET NULL у `subaccount_routing_assignment` запись остаётся. Решить: cron-cleanup или auto-DELETE при null+null.
- **Redis hardening** на sandbox-server (memory `project_redis_hijack_2026_05_04`): requirepass + firewall — не блокирует Plan 2, но нужно перед prod-rollout.
- **`verifySubAccountOwnership` dedupe**: helper повторяется в `network_assignments.go` и `subaccount_network_overrides.go`. Вынести в shared package если появится третий consumer.
- **Operator-condition matching в preview**: сейчас `condition_type='operator'` всегда true. Реальный matching через таблицу `operators` — отложено.
- **Country-prefix mapping**: hardcoded в `network_route_preview.go`. Лучше вынести в JSON / БД при появлении новых стран.
- **Audit-log UI**: данные пишутся в `audit_log`, но страницы «История изменений» нет (spec §6.7).
- **Differential preview** «что изменится при применении нового шаблона» — отложено per spec §4.6.
- **Optimistic concurrency** (If-Unmodified-Since/412) — отложено per spec §6.5.
- **Двухуровневая reseller-иерархия** UI — отложено per spec §3.4.


