# Routing Hierarchy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Привести систему маршрутизации к unified owner-based иерархии (platform/client/subaccount) с override-семантикой и specificity-ранжированием, аналогично tarification.

**Architecture:** Переводим `client_routes` с плоской client-based модели на owner-based (`owner_type`, `owner_id`). Уничтожаем `route_condition_groups`/`route_conditions` (OR/AND конструкции) в пользу плоских колонок. Matcher проходит уровни `subaccount → client → platform`, на каждом выбирая самое специфичное подошедшее правило; первое найденное побеждает целиком (вместе со своей failover-цепочкой провайдеров).

**Tech Stack:** Go 1.24, PostgreSQL 15 (pgx/v5), gRPC, React 19 (portal). Миграции — SQL. Тесты — testify.

**Spec:** [docs/superpowers/specs/2026-04-21-routing-hierarchy-design.md](../specs/2026-04-21-routing-hierarchy-design.md)

---

## File Structure

**Миграции (новые):**
- `migrations/000107_routing_hierarchy_schema.up.sql` / `.down.sql` — ENUM, колонки, индексы, CHECK.
- `migrations/000108_routing_hierarchy_data.up.sql` / `.down.sql` — backfill `owner_type`/`owner_id`, развёртка condition_groups, seed platform general.
- `migrations/000109_routing_drop_conditions.up.sql` / `.down.sql` — DROP старых таблиц (после валидации).

**Domain (модификации):**
- `internal/services/routing/domain/client_route.go` — убираем `ClientID`, `Groups`, добавляем `OwnerType`, `OwnerID`, `CountryCode`, `TrafficType`, `PaidName`, `Regex`, `ScheduleID`.
- `internal/services/routing/domain/owner_type.go` (NEW) — enum `OwnerType`.
- `internal/services/routing/domain/specificity.go` (NEW) — функция `KeyFieldsFilled(r) int`.

**Application (переработка):**
- `internal/services/routing/application/matcher.go` — полная переработка `MatchContext` и `Match()`.
- `internal/services/routing/application/matcher_test.go` — существующие тесты удаляются/переписываются.

**Repository:**
- `internal/services/routing/infrastructure/route_repo.go` (или эквивалент — уточнить) — SELECT/INSERT/UPDATE с новыми колонками.

**gRPC:**
- `api/proto/routing/routing.proto` — новый `ResolveRoutes` RPC, `route_owner_type` enum, обновлённый `ClientRouteProto`.
- сгенерированные файлы `*.pb.go`.

**Portal handlers:**
- `internal/gateway/portal/handlers/reseller_routing.go` — расширение CRUD для `owner_type IN ('client','subaccount')`.
- `internal/gateway/portal/handlers/sub_account_routing.go` — CRUD для собственных правил суб-аккаунта.
- `internal/gateway/portal/handlers/routing.go` (NEW или модификация существующего) — CRUD для обычного клиента.
- `internal/gateway/admin/handlers/routing.go` — platform-правила, защита singleton.
- `internal/gateway/portal/handlers/routing_authz.go` (NEW) — `assertOwnable(user, ownerType, ownerID)`.

Каждый файл с одной ответственностью. Тесты — рядом с кодом в `*_test.go`.

---

## Phase 0: Pre-flight (обязательно до Phase 1)

### Task 0: Audit текущих данных

**Files:** none (только чтение)

- [ ] **Step 1: Запустить audit-запрос на стенде**

```sql
-- сколько правил используют OR между группами
SELECT count(*) AS multi_group_rules FROM (
  SELECT rule_id FROM route_condition_groups
  GROUP BY rule_id HAVING count(*) > 1
) t;

-- общий объём правил по типам
SELECT route_type, count(*) FROM client_routes GROUP BY route_type;

-- сколько правил с client_id IS NULL (будущие platform)
SELECT count(*) FROM client_routes WHERE client_id IS NULL;

-- сколько условий с типами paid_name/regex/schedule
SELECT type, count(*) FROM route_conditions GROUP BY type;
```

- [ ] **Step 2: Проверить порог**

Если `multi_group_rules > 50` — СТОП. Вернуться к спеку, пересмотреть секцию «Специфичность» (возможно, оставить condition_groups в обновлённом виде). Документировать решение в спеке.

Если ≤ 50 — продолжаем.

- [ ] **Step 3: Зафиксировать результат**

Добавить секцию «Pre-flight результат» в конец spec-файла с датой и числами.

- [ ] **Step 4: Commit (изменение спека)**

```bash
git add docs/superpowers/specs/2026-04-21-routing-hierarchy-design.md
git commit -m "docs(routing): pre-flight audit результаты"
```

---

## Phase 1: Schema Migration

### Task 1: Миграция схемы (columns, ENUM, constraints)

**Files:**
- Create: `migrations/000107_routing_hierarchy_schema.up.sql`
- Create: `migrations/000107_routing_hierarchy_schema.down.sql`

- [ ] **Step 1: Написать up-миграцию**

```sql
-- 000107_routing_hierarchy_schema.up.sql
BEGIN;

CREATE TYPE route_owner_type AS ENUM ('platform','client','subaccount');

ALTER TABLE client_routes
  ADD COLUMN owner_type   route_owner_type,
  ADD COLUMN owner_id     UUID NULL,
  ADD COLUMN country_code CHAR(2) NULL,
  ADD COLUMN traffic_type TEXT NULL,
  ADD COLUMN paid_name    TEXT NULL,
  ADD COLUMN regex        TEXT NULL,
  ADD COLUMN schedule_id  UUID NULL REFERENCES route_schedules(id);

-- operator_id уже NULL-able после 000080. Проверить; если NOT NULL — снять.
-- (Зависит от текущего состояния; если уже nullable — пропустить.)
ALTER TABLE client_routes ALTER COLUMN operator_id DROP NOT NULL;

CREATE INDEX ix_routes_owner_routetype ON client_routes (owner_type, owner_id, route_type);

COMMIT;
```

- [ ] **Step 2: Написать down-миграцию**

```sql
-- 000107_routing_hierarchy_schema.down.sql
BEGIN;

DROP INDEX IF EXISTS ix_routes_owner_routetype;

ALTER TABLE client_routes
  DROP COLUMN IF EXISTS schedule_id,
  DROP COLUMN IF EXISTS regex,
  DROP COLUMN IF EXISTS paid_name,
  DROP COLUMN IF EXISTS traffic_type,
  DROP COLUMN IF EXISTS country_code,
  DROP COLUMN IF EXISTS owner_id,
  DROP COLUMN IF EXISTS owner_type;

DROP TYPE IF EXISTS route_owner_type;

COMMIT;
```

- [ ] **Step 3: Применить локально**

```bash
./scripts/server.sh migrate   # или локальная команда миграции
```

Ожидать: успех, без ошибок. Проверить `\d client_routes` в psql — колонки появились.

- [ ] **Step 4: Откатить/применить ещё раз**

```bash
migrate -database "$DB_URL" -path migrations down 1
migrate -database "$DB_URL" -path migrations up 1
```

Ожидать: down чист, повторный up — без ошибок.

- [ ] **Step 5: Commit**

```bash
git add migrations/000107_*
git commit -m "feat(routing): миграция 107 — схема иерархии (owner_type, ключевые поля)"
```

---

### Task 2: Data-миграция (backfill owner_type, flatten groups, seed platform)

**Files:**
- Create: `migrations/000108_routing_hierarchy_data.up.sql`
- Create: `migrations/000108_routing_hierarchy_data.down.sql`

- [ ] **Step 1: Написать up-миграцию**

```sql
-- 000108_routing_hierarchy_data.up.sql
BEGIN;

-- Шаг 1: owner_type для platform (client_id IS NULL)
UPDATE client_routes
  SET owner_type = 'platform', owner_id = NULL
  WHERE client_id IS NULL;

-- Шаг 2: owner_type для client/subaccount
UPDATE client_routes cr SET
  owner_type = CASE WHEN c.parent_client_id IS NULL THEN 'client'::route_owner_type
                    ELSE 'subaccount'::route_owner_type END,
  owner_id = cr.client_id
  FROM clients c
  WHERE c.id = cr.client_id;

-- Шаг 3: развернуть condition_groups в плоские колонки.
-- Для правил с одной группой — UPDATE.
-- Для правил с несколькими группами (OR) — ручная развёртка в N строк
-- через INSERT на основе шаблона. Если pre-flight показал ≤50 мульти-групп,
-- перечислить их явно здесь. Если 0 — пропустить.

-- Одна группа, одно условие на тип:
UPDATE client_routes cr SET
  operator_id  = sub.operator_id,
  country_code = sub.country_code,
  traffic_type = sub.traffic_type,
  paid_name    = sub.paid_name,
  regex        = sub.regex,
  schedule_id  = sub.schedule_id
FROM (
  SELECT
    rcg.rule_id,
    MAX(CASE WHEN rc.type = 'operator'     THEN rc.value::uuid END) AS operator_id,
    MAX(CASE WHEN rc.type = 'country'      THEN rc.value        END) AS country_code,
    MAX(CASE WHEN rc.type = 'traffic_type' THEN rc.value        END) AS traffic_type,
    MAX(CASE WHEN rc.type = 'paid_name'    THEN rc.value        END) AS paid_name,
    MAX(CASE WHEN rc.type = 'regex'        THEN rc.value        END) AS regex,
    MAX(rs.id)                                                       AS schedule_id
  FROM route_condition_groups rcg
  JOIN route_conditions rc ON rc.group_id = rcg.id
  LEFT JOIN route_schedules rs ON rs.rule_id = rcg.rule_id
  GROUP BY rcg.rule_id
  HAVING count(DISTINCT rcg.id) = 1  -- только одна группа
) sub
WHERE cr.id = sub.rule_id;

-- Шаг 4: seed platform general для каждого route_type, если отсутствует.
-- <default_provider> подставляется оператором деплоя (параметр).
-- Если провайдер не указан — миграция падает с явным сообщением.
DO $$
DECLARE
  default_provider UUID;
BEGIN
  -- Подставить ID провайдера по умолчанию. Если нет — ERROR.
  SELECT id INTO default_provider FROM providers WHERE code = 'default' LIMIT 1;
  IF default_provider IS NULL THEN
    RAISE EXCEPTION 'platform general seed: no provider with code=default; миграцию нельзя запускать без default-провайдера';
  END IF;

  INSERT INTO client_routes (id, owner_type, owner_id, route_type, provider_id, priority, active, status, name)
    SELECT gen_random_uuid(), 'platform', NULL, rt, default_provider, 100, true, 'active', 'platform-general-' || rt
    FROM (VALUES ('sms'),('hlr'),('max')) AS t(rt)
    WHERE NOT EXISTS (
      SELECT 1 FROM client_routes
      WHERE owner_type = 'platform'
        AND route_type = t.rt
        AND operator_id IS NULL
        AND country_code IS NULL
        AND traffic_type IS NULL
    );
END$$;

-- Шаг 5: теперь можно поставить NOT NULL на owner_type
ALTER TABLE client_routes ALTER COLUMN owner_type SET NOT NULL;

-- Шаг 6: инвариант owner_id
ALTER TABLE client_routes ADD CONSTRAINT chk_owner_id CHECK (
  (owner_type = 'platform' AND owner_id IS NULL)
  OR
  (owner_type <> 'platform' AND owner_id IS NOT NULL)
);

-- Шаг 7: unique indexes
CREATE UNIQUE INDEX uq_platform_general ON client_routes (route_type)
  WHERE owner_type = 'platform'
    AND operator_id IS NULL AND country_code IS NULL AND traffic_type IS NULL;

CREATE UNIQUE INDEX uq_owner_general ON client_routes (owner_type, owner_id, route_type)
  WHERE owner_type <> 'platform'
    AND operator_id IS NULL AND country_code IS NULL AND traffic_type IS NULL;

CREATE UNIQUE INDEX uq_cell_provider ON client_routes (
  owner_type,
  COALESCE(owner_id,     '00000000-0000-0000-0000-000000000000'::uuid),
  route_type,
  COALESCE(operator_id,  '00000000-0000-0000-0000-000000000000'::uuid),
  COALESCE(country_code, ''),
  COALESCE(traffic_type, ''),
  provider_id
);

COMMIT;
```

- [ ] **Step 2: Написать down-миграцию**

```sql
-- 000108_routing_hierarchy_data.down.sql
BEGIN;

DROP INDEX IF EXISTS uq_cell_provider;
DROP INDEX IF EXISTS uq_owner_general;
DROP INDEX IF EXISTS uq_platform_general;

ALTER TABLE client_routes DROP CONSTRAINT IF EXISTS chk_owner_id;
ALTER TABLE client_routes ALTER COLUMN owner_type DROP NOT NULL;

-- Удалить seed-записи platform general
DELETE FROM client_routes
  WHERE owner_type = 'platform'
    AND name LIKE 'platform-general-%';

-- Откат backfill — опционально, т.к. owner_type/owner_id будут удалены в 107-down
UPDATE client_routes SET owner_type = NULL, owner_id = NULL;
UPDATE client_routes cr SET
  operator_id  = NULL,
  country_code = NULL,
  traffic_type = NULL,
  paid_name    = NULL,
  regex        = NULL,
  schedule_id  = NULL;

COMMIT;
```

- [ ] **Step 3: Применить локально**

```bash
./scripts/server.sh migrate
```

Ожидать: успех. Если falls с `no provider with code=default` — создать провайдера или поправить SELECT под реальный default.

- [ ] **Step 4: Проверить данные**

```sql
SELECT owner_type, count(*) FROM client_routes GROUP BY owner_type;
-- ожидать: все строки имеют owner_type

SELECT count(*) FROM client_routes WHERE owner_type = 'platform' AND operator_id IS NULL;
-- ожидать: 3 (по одному на sms/hlr/max)
```

- [ ] **Step 5: Commit**

```bash
git add migrations/000108_*
git commit -m "feat(routing): миграция 108 — backfill owner_type, развёртка groups, seed platform general"
```

---

## Phase 2: Domain Model Refactor

### Task 3: Добавить enum OwnerType

**Files:**
- Create: `internal/services/routing/domain/owner_type.go`
- Test: `internal/services/routing/domain/owner_type_test.go`

- [ ] **Step 1: Написать failing-тест**

```go
// owner_type_test.go
package domain_test

import (
    "testing"

    "github.com/stretchr/testify/require"
    "github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

func TestOwnerType_Valid(t *testing.T) {
    require.True(t, domain.OwnerPlatform.Valid())
    require.True(t, domain.OwnerClient.Valid())
    require.True(t, domain.OwnerSubaccount.Valid())
    require.False(t, domain.OwnerType("bogus").Valid())
}

func TestOwnerType_String(t *testing.T) {
    require.Equal(t, "platform", string(domain.OwnerPlatform))
}
```

- [ ] **Step 2: Запустить — должно упасть**

```bash
go test ./internal/services/routing/domain/ -run TestOwnerType -v
```

Ожидать: compile error (типа нет).

- [ ] **Step 3: Реализация**

```go
// owner_type.go
package domain

type OwnerType string

const (
    OwnerPlatform   OwnerType = "platform"
    OwnerClient     OwnerType = "client"
    OwnerSubaccount OwnerType = "subaccount"
)

func (o OwnerType) Valid() bool {
    switch o {
    case OwnerPlatform, OwnerClient, OwnerSubaccount:
        return true
    }
    return false
}
```

- [ ] **Step 4: Запустить — должно пройти**

```bash
go test ./internal/services/routing/domain/ -run TestOwnerType -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/services/routing/domain/owner_type.go internal/services/routing/domain/owner_type_test.go
git commit -m "feat(routing): domain.OwnerType enum"
```

---

### Task 4: Refactor ClientRoute struct

**Files:**
- Modify: `internal/services/routing/domain/client_route.go`
- Test: `internal/services/routing/domain/client_route_test.go` (may already exist or create)

- [ ] **Step 1: Написать тест новой структуры**

```go
// client_route_test.go
package domain_test

import (
    "testing"

    "github.com/google/uuid"
    "github.com/stretchr/testify/require"
    "github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

func TestClientRoute_IsPlatform(t *testing.T) {
    r := &domain.ClientRoute{OwnerType: domain.OwnerPlatform, OwnerID: nil}
    require.True(t, r.IsPlatform())
}

func TestClientRoute_IsGeneral(t *testing.T) {
    r := &domain.ClientRoute{}   // все ключевые поля nil
    require.True(t, r.IsGeneral())

    op := uuid.New()
    r.OperatorID = &op
    require.False(t, r.IsGeneral())
}
```

- [ ] **Step 2: Запустить — упадёт**

```bash
go test ./internal/services/routing/domain/ -run TestClientRoute -v
```

- [ ] **Step 3: Переписать структуру**

Полная замена содержимого `client_route.go`:

```go
package domain

import (
    "errors"
    "time"

    "github.com/google/uuid"
)

type ClientRoute struct {
    ID          uuid.UUID
    OwnerType   OwnerType
    OwnerID     *uuid.UUID       // NULL только для platform
    RouteType   string           // sms | hlr | max

    // Ключевые поля (участвуют в specificity)
    OperatorID  *uuid.UUID
    CountryCode *string
    TrafficType *string

    // Фильтры (не участвуют в specificity)
    PaidName    *string
    Regex       *string
    ScheduleID  *uuid.UUID

    ProviderID  uuid.UUID
    Priority    int
    Weight      int
    Share       int
    Active      bool
    Status      RouteStatus
    Name        string
    Comment     string

    CreatedAt time.Time
    UpdatedAt time.Time
}

func (r *ClientRoute) IsPlatform() bool { return r.OwnerType == OwnerPlatform }

func (r *ClientRoute) IsGeneral() bool {
    return r.OperatorID == nil && r.CountryCode == nil && r.TrafficType == nil
}

var (
    ErrClientRouteNotFound      = errors.New("client route not found")
    ErrClientRouteAlreadyExists = errors.New("route already exists for this owner/route_type/cell/provider")
)
```

Удалены: `ClientID`, `Groups`, `Schedules`, `NewClientRoute`, `NewManagedRoute` (конструкторы заменит billable CRUD).

- [ ] **Step 4: Запустить — домен компилируется, тест проходит**

```bash
go test ./internal/services/routing/domain/ -v
```

**Ожидать:** на этом этапе application/repository слои НЕ компилируются. Это нормально — следующие задачи чинят.

- [ ] **Step 5: Commit**

```bash
git add internal/services/routing/domain/client_route.go internal/services/routing/domain/client_route_test.go
git commit -m "refactor(routing): ClientRoute под owner-based модель (ломает application/repo — чинится следующими задачами)"
```

---

### Task 5: Функция specificity

**Files:**
- Create: `internal/services/routing/domain/specificity.go`
- Test: `internal/services/routing/domain/specificity_test.go`

- [ ] **Step 1: Failing-test**

```go
// specificity_test.go
package domain_test

import (
    "testing"

    "github.com/google/uuid"
    "github.com/stretchr/testify/require"
    "github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

func TestKeyFieldsFilled(t *testing.T) {
    op := uuid.New()
    cc := "RU"
    tt := "transactional"

    tests := []struct {
        name  string
        route *domain.ClientRoute
        want  int
    }{
        {"general", &domain.ClientRoute{}, 0},
        {"operator only", &domain.ClientRoute{OperatorID: &op}, 1},
        {"operator+country", &domain.ClientRoute{OperatorID: &op, CountryCode: &cc}, 2},
        {"all three", &domain.ClientRoute{OperatorID: &op, CountryCode: &cc, TrafficType: &tt}, 3},
        {"paid_name не считается", &domain.ClientRoute{PaidName: strPtr("bank")}, 0},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            require.Equal(t, tc.want, domain.KeyFieldsFilled(tc.route))
        })
    }
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: Run — fails**

```bash
go test ./internal/services/routing/domain/ -run TestKeyFieldsFilled -v
```

- [ ] **Step 3: Реализация**

```go
// specificity.go
package domain

func KeyFieldsFilled(r *ClientRoute) int {
    n := 0
    if r.OperatorID != nil {
        n++
    }
    if r.CountryCode != nil {
        n++
    }
    if r.TrafficType != nil {
        n++
    }
    return n
}
```

- [ ] **Step 4: Run — passes**

```bash
go test ./internal/services/routing/domain/ -run TestKeyFieldsFilled -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/services/routing/domain/specificity.go internal/services/routing/domain/specificity_test.go
git commit -m "feat(routing): domain.KeyFieldsFilled для ранжирования специфичности"
```

---

## Phase 3: Repository Refactor

### Task 6: Обновить SELECT в route_repo

**Files:**
- Modify: `internal/services/routing/infrastructure/route_repo.go` (файл уточнить через grep)
- Test: `internal/services/routing/infrastructure/route_repo_test.go` (integration — нужна тестовая БД)

- [ ] **Step 1: Найти репозиторий**

```bash
grep -rn "LoadAllActive" internal/services/routing/
```

Запомнить путь, использовать его ниже (условно `internal/services/routing/infrastructure/route_repo.go`).

- [ ] **Step 2: Integration-тест LoadAllActive (с pgx)**

```go
// route_repo_test.go
func TestLoadAllActive_ReturnsOwnerBasedRoutes(t *testing.T) {
    db := testDB(t)  // тестовый pgx pool, миграции применены
    seed(t, db, `
      INSERT INTO client_routes (id, owner_type, owner_id, route_type, provider_id, priority, active, status)
      VALUES (gen_random_uuid(), 'platform', NULL, 'sms', $1, 0, true, 'active')
    `, providerID)

    repo := infrastructure.NewRouteRepo(db)
    routes, err := repo.LoadAllActive(context.Background())
    require.NoError(t, err)
    require.Len(t, routes, 1)
    require.Equal(t, domain.OwnerPlatform, routes[0].OwnerType)
    require.Nil(t, routes[0].OwnerID)
    require.Equal(t, "sms", routes[0].RouteType)
}
```

- [ ] **Step 3: Run — fails (компиляция или поведение)**

- [ ] **Step 4: Переписать SELECT**

```go
func (r *RouteRepo) LoadAllActive(ctx context.Context) ([]*domain.ClientRoute, error) {
    query := `
      SELECT id, owner_type, owner_id, route_type,
             operator_id, country_code, traffic_type,
             paid_name, regex, schedule_id,
             provider_id, priority, weight, share, active, status,
             name, comment, created_at, updated_at
      FROM client_routes
      WHERE active = true
    `
    rows, err := r.pool.Query(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var out []*domain.ClientRoute
    for rows.Next() {
        var cr domain.ClientRoute
        var ownerType string
        if err := rows.Scan(
            &cr.ID, &ownerType, &cr.OwnerID, &cr.RouteType,
            &cr.OperatorID, &cr.CountryCode, &cr.TrafficType,
            &cr.PaidName, &cr.Regex, &cr.ScheduleID,
            &cr.ProviderID, &cr.Priority, &cr.Weight, &cr.Share, &cr.Active, &cr.Status,
            &cr.Name, &cr.Comment, &cr.CreatedAt, &cr.UpdatedAt,
        ); err != nil {
            return nil, err
        }
        cr.OwnerType = domain.OwnerType(ownerType)
        out = append(out, &cr)
    }
    return out, rows.Err()
}
```

- [ ] **Step 5: Run — passes**

```bash
go test ./internal/services/routing/infrastructure/... -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/services/routing/infrastructure/route_repo.go internal/services/routing/infrastructure/route_repo_test.go
git commit -m "refactor(routing): LoadAllActive под owner-based схему"
```

---

### Task 7: Обновить Insert/Update/Delete в route_repo

**Files:**
- Modify: `internal/services/routing/infrastructure/route_repo.go`
- Test: `internal/services/routing/infrastructure/route_repo_test.go`

- [ ] **Step 1: Тест Insert**

```go
func TestInsert_PlatformGeneral_RespectsSingletonUnique(t *testing.T) {
    db := testDB(t)
    repo := infrastructure.NewRouteRepo(db)

    r1 := &domain.ClientRoute{OwnerType: domain.OwnerPlatform, OwnerID: nil, RouteType: "sms", ProviderID: provider1, Active: true}
    require.NoError(t, repo.Insert(ctx, r1))

    r2 := &domain.ClientRoute{OwnerType: domain.OwnerPlatform, OwnerID: nil, RouteType: "sms", ProviderID: provider2, Active: true}
    err := repo.Insert(ctx, r2)
    require.Error(t, err, "ожидаем unique violation на uq_platform_general")
}

func TestInsert_SubaccountCell_Ok(t *testing.T) {
    // валидный кейс subaccount-правила
    r := &domain.ClientRoute{
        OwnerType: domain.OwnerSubaccount, OwnerID: &subID, RouteType: "sms",
        OperatorID: &opID, CountryCode: strPtr("RU"), ProviderID: provID, Active: true,
    }
    require.NoError(t, repo.Insert(ctx, r))
}
```

- [ ] **Step 2: Run — fails**

- [ ] **Step 3: Реализация Insert**

```go
func (r *RouteRepo) Insert(ctx context.Context, cr *domain.ClientRoute) error {
    if cr.ID == uuid.Nil {
        cr.ID = uuid.New()
    }
    _, err := r.pool.Exec(ctx, `
      INSERT INTO client_routes
        (id, owner_type, owner_id, route_type,
         operator_id, country_code, traffic_type, paid_name, regex, schedule_id,
         provider_id, priority, weight, share, active, status, name, comment, created_at, updated_at)
      VALUES
        ($1, $2, $3, $4,
         $5, $6, $7, $8, $9, $10,
         $11, $12, $13, $14, $15, $16, $17, $18, NOW(), NOW())
    `,
        cr.ID, string(cr.OwnerType), cr.OwnerID, cr.RouteType,
        cr.OperatorID, cr.CountryCode, cr.TrafficType, cr.PaidName, cr.Regex, cr.ScheduleID,
        cr.ProviderID, cr.Priority, cr.Weight, cr.Share, cr.Active, cr.Status, cr.Name, cr.Comment,
    )
    return err
}
```

Аналогично Update (по ID) и Delete (с защитой platform singleton — см. Task 15).

- [ ] **Step 4: Run — passes**

- [ ] **Step 5: Commit**

```bash
git commit -m "refactor(routing): Insert/Update/Delete под owner-based схему"
```

---

## Phase 4: Matcher Rewrite

### Task 8: Переписать MatchContext

**Files:**
- Modify: `internal/services/routing/application/matcher.go:18-28`

- [ ] **Step 1: Изменить структуру**

```go
type MatchContext struct {
    RouteType    string
    SubaccountID *uuid.UUID    // nil если клиент не суб-аккаунт
    ClientID     uuid.UUID     // для суб-аккаунта = parent, иначе сам клиент

    OperatorID   *uuid.UUID
    CountryCode  string
    TrafficType  string
    PaidName     string
    PhoneNumber  string
    SenderName   string
    Time         time.Time
}

// OwnerChain возвращает цепочку (OwnerType, OwnerID) от самого частного к общему.
func (c MatchContext) OwnerChain() []struct {
    Type domain.OwnerType
    ID   *uuid.UUID
} {
    var chain []struct {
        Type domain.OwnerType
        ID   *uuid.UUID
    }
    if c.SubaccountID != nil {
        chain = append(chain, struct {
            Type domain.OwnerType
            ID   *uuid.UUID
        }{domain.OwnerSubaccount, c.SubaccountID})
    }
    chain = append(chain, struct {
        Type domain.OwnerType
        ID   *uuid.UUID
    }{domain.OwnerClient, &c.ClientID})
    chain = append(chain, struct {
        Type domain.OwnerType
        ID   *uuid.UUID
    }{domain.OwnerPlatform, nil})
    return chain
}
```

Поле `ClientID uuid.UUID` → `SubaccountID`+`ClientID`. Вызывающие будут исправлены в Task 16 (pipeline integration).

- [ ] **Step 2: Проверить компиляцию**

```bash
go build ./internal/services/routing/application/
```

Скорее всего упадёт на matcher_test.go и pipeline. Исправляется следующими задачами.

- [ ] **Step 3: Commit (частичный)**

Допустимо закоммитить компилирующуюся часть как WIP:

```bash
git add internal/services/routing/application/matcher.go
git commit -m "refactor(routing): MatchContext с OwnerChain (WIP — следующие задачи чинят matcher)"
```

---

### Task 9: Переписать Match() с иерархическим fallback

**Files:**
- Modify: `internal/services/routing/application/matcher.go:93-123` (функция `Match`)

- [ ] **Step 1: Тест базового случая (subaccount specific > client general)**

```go
// matcher_test.go (новый блок, остальные тесты удалим в Task 13)
func TestMatch_SubaccountSpecific_BeatsClientGeneral(t *testing.T) {
    m := setupMatcher(t)
    subID := uuid.New()
    clID := uuid.New()
    opID := uuid.New()

    // Client-level general (catch-all)
    m.routes = append(m.routes, &domain.ClientRoute{
        ID: uuid.New(), OwnerType: domain.OwnerClient, OwnerID: &clID,
        RouteType: "sms", ProviderID: clientProv, Active: true,
    })
    // Subaccount-level specific (operator+country)
    m.routes = append(m.routes, &domain.ClientRoute{
        ID: uuid.New(), OwnerType: domain.OwnerSubaccount, OwnerID: &subID,
        RouteType: "sms", OperatorID: &opID, CountryCode: strPtr("RU"),
        ProviderID: subProv, Active: true,
    })

    chain := m.Match(MatchContext{
        RouteType: "sms", SubaccountID: &subID, ClientID: clID,
        OperatorID: &opID, CountryCode: "RU",
    })
    require.Len(t, chain, 1)
    require.Equal(t, subProv, chain[0].ProviderID)
}
```

- [ ] **Step 2: Run — fails**

- [ ] **Step 3: Реализация Match**

```go
func (m *RouteMatcher) Match(ctx MatchContext) []*domain.ClientRoute {
    m.mu.RLock()
    defer m.mu.RUnlock()

    for _, level := range ctx.OwnerChain() {
        candidates := m.filterByOwnerAndMatch(level.Type, level.ID, ctx)
        if len(candidates) == 0 {
            continue
        }

        // Выбираем самое специфичное
        maxSpec := 0
        for _, c := range candidates {
            if s := domain.KeyFieldsFilled(c); s > maxSpec {
                maxSpec = s
            }
        }
        top := candidates[:0]
        for _, c := range candidates {
            if domain.KeyFieldsFilled(c) == maxSpec {
                top = append(top, c)
            }
        }
        // Тай-брейкер: priority ASC, created_at ASC
        sort.SliceStable(top, func(i, j int) bool {
            if top[i].Priority != top[j].Priority {
                return top[i].Priority < top[j].Priority
            }
            return top[i].CreatedAt.Before(top[j].CreatedAt)
        })
        winner := top[0]

        // Возвращаем всю failover-цепочку ячейки победителя
        return m.cellChain(winner)
    }
    return nil
}

func (m *RouteMatcher) filterByOwnerAndMatch(ot domain.OwnerType, oid *uuid.UUID, ctx MatchContext) []*domain.ClientRoute {
    var out []*domain.ClientRoute
    for _, r := range m.routes {
        if r.OwnerType != ot {
            continue
        }
        if ot != domain.OwnerPlatform && (r.OwnerID == nil || *r.OwnerID != *oid) {
            continue
        }
        if r.RouteType != ctx.RouteType {
            continue
        }
        if !fieldsMatch(r, ctx) {
            continue
        }
        out = append(out, r)
    }
    return out
}

func fieldsMatch(r *domain.ClientRoute, ctx MatchContext) bool {
    // Ключевые поля: NULL = wildcard
    if r.OperatorID != nil && (ctx.OperatorID == nil || *r.OperatorID != *ctx.OperatorID) {
        return false
    }
    if r.CountryCode != nil && *r.CountryCode != ctx.CountryCode {
        return false
    }
    if r.TrafficType != nil && *r.TrafficType != ctx.TrafficType {
        return false
    }
    // Фильтры
    if r.PaidName != nil && *r.PaidName != ctx.PaidName {
        return false
    }
    if r.Regex != nil {
        re, err := regexp.Compile(*r.Regex)
        if err != nil || !re.MatchString(ctx.PhoneNumber) {
            return false
        }
    }
    // Schedule — реюз существующей логики из текущего matcher, если есть
    return true
}

func (m *RouteMatcher) cellChain(winner *domain.ClientRoute) []*domain.ClientRoute {
    var out []*domain.ClientRoute
    for _, r := range m.routes {
        if r.OwnerType != winner.OwnerType ||
           !uuidEq(r.OwnerID, winner.OwnerID) ||
           r.RouteType != winner.RouteType ||
           !uuidEq(r.OperatorID, winner.OperatorID) ||
           !strEq(r.CountryCode, winner.CountryCode) ||
           !strEq(r.TrafficType, winner.TrafficType) {
            continue
        }
        out = append(out, r)
    }
    sort.SliceStable(out, func(i, j int) bool { return out[i].Priority < out[j].Priority })
    return out
}

func uuidEq(a, b *uuid.UUID) bool {
    if a == nil && b == nil {
        return true
    }
    if a == nil || b == nil {
        return false
    }
    return *a == *b
}

func strEq(a, b *string) bool {
    if a == nil && b == nil {
        return true
    }
    if a == nil || b == nil {
        return false
    }
    return *a == *b
}
```

- [ ] **Step 4: Run — passes**

```bash
go test ./internal/services/routing/application/ -run TestMatch_SubaccountSpecific_BeatsClientGeneral -v
```

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(routing): иерархический matcher с specificity-ранжированием"
```

---

### Task 10: Тест — уровень сильнее специфичности

**Files:**
- Modify: `internal/services/routing/application/matcher_test.go`

- [ ] **Step 1: Test**

```go
func TestMatch_SubaccountGeneral_BeatsClientSpecific(t *testing.T) {
    m := setupMatcher(t)
    subID := uuid.New()
    clID := uuid.New()
    opID := uuid.New()

    // Client-level SPECIFIC (operator+country)
    m.routes = append(m.routes, &domain.ClientRoute{
        OwnerType: domain.OwnerClient, OwnerID: &clID, RouteType: "sms",
        OperatorID: &opID, CountryCode: strPtr("RU"), ProviderID: clProv, Active: true,
    })
    // Subaccount-level GENERAL (catch-all)
    m.routes = append(m.routes, &domain.ClientRoute{
        OwnerType: domain.OwnerSubaccount, OwnerID: &subID, RouteType: "sms",
        ProviderID: subProv, Active: true,
    })

    chain := m.Match(MatchContext{
        RouteType: "sms", SubaccountID: &subID, ClientID: clID,
        OperatorID: &opID, CountryCode: "RU",
    })
    require.Len(t, chain, 1)
    require.Equal(t, subProv, chain[0].ProviderID, "уровень важнее специфичности")
}
```

- [ ] **Step 2: Run — должно пройти (алгоритм уже покрывает этот случай)**

- [ ] **Step 3: Commit**

```bash
git commit -m "test(routing): уровень сильнее специфичности"
```

---

### Task 11: Тесты — fallback к platform, failover-цепочка, route_type isolation

**Files:**
- Modify: `internal/services/routing/application/matcher_test.go`

- [ ] **Step 1: Написать три теста**

```go
func TestMatch_FallsBackToPlatform_WhenNoOwnerRules(t *testing.T) {
    m := setupMatcher(t)
    clID := uuid.New()
    m.routes = append(m.routes, &domain.ClientRoute{
        OwnerType: domain.OwnerPlatform, OwnerID: nil, RouteType: "sms",
        ProviderID: platformProv, Active: true,
    })

    chain := m.Match(MatchContext{RouteType: "sms", ClientID: clID, OperatorID: &opID})
    require.Len(t, chain, 1)
    require.Equal(t, platformProv, chain[0].ProviderID)
}

func TestMatch_FailoverChain_InSameCell(t *testing.T) {
    m := setupMatcher(t)
    subID := uuid.New()
    clID := uuid.New()
    for i, prov := range []uuid.UUID{provA, provB, provC} {
        m.routes = append(m.routes, &domain.ClientRoute{
            OwnerType: domain.OwnerSubaccount, OwnerID: &subID, RouteType: "sms",
            OperatorID: &opID, CountryCode: strPtr("RU"),
            ProviderID: prov, Priority: i, Active: true,
        })
    }

    chain := m.Match(MatchContext{RouteType: "sms", SubaccountID: &subID, ClientID: clID, OperatorID: &opID, CountryCode: "RU"})
    require.Len(t, chain, 3)
    require.Equal(t, provA, chain[0].ProviderID)
    require.Equal(t, provB, chain[1].ProviderID)
    require.Equal(t, provC, chain[2].ProviderID)
}

func TestMatch_RouteType_Isolates(t *testing.T) {
    m := setupMatcher(t)
    clID := uuid.New()
    m.routes = append(m.routes, &domain.ClientRoute{
        OwnerType: domain.OwnerClient, OwnerID: &clID, RouteType: "hlr",
        ProviderID: hlrProv, Active: true,
    })

    chain := m.Match(MatchContext{RouteType: "sms", ClientID: clID})
    require.Empty(t, chain, "HLR-правило не применяется к SMS-сообщению")
}
```

- [ ] **Step 2: Run — passes**

```bash
go test ./internal/services/routing/application/ -run TestMatch_ -v
```

- [ ] **Step 3: Commit**

```bash
git commit -m "test(routing): platform fallback, failover chain, route_type isolation"
```

---

### Task 12: Тесты — override целиком (subaccount не подмешивает agg)

**Files:**
- Modify: `internal/services/routing/application/matcher_test.go`

- [ ] **Step 1: Test**

```go
func TestMatch_SubaccountCellReplacesAggregatorCell(t *testing.T) {
    m := setupMatcher(t)
    subID := uuid.New()
    clID := uuid.New()

    // Агрегатор: 3 провайдера в ячейке
    for i, p := range []uuid.UUID{aggA, aggB, aggC} {
        m.routes = append(m.routes, &domain.ClientRoute{
            OwnerType: domain.OwnerClient, OwnerID: &clID, RouteType: "sms",
            OperatorID: &opID, CountryCode: strPtr("RU"),
            ProviderID: p, Priority: i, Active: true,
        })
    }
    // Суб-аккаунт: один провайдер в той же ячейке
    m.routes = append(m.routes, &domain.ClientRoute{
        OwnerType: domain.OwnerSubaccount, OwnerID: &subID, RouteType: "sms",
        OperatorID: &opID, CountryCode: strPtr("RU"),
        ProviderID: subProv, Priority: 0, Active: true,
    })

    chain := m.Match(MatchContext{
        RouteType: "sms", SubaccountID: &subID, ClientID: clID,
        OperatorID: &opID, CountryCode: "RU",
    })
    require.Len(t, chain, 1, "цепочка агрегатора должна быть полностью отброшена")
    require.Equal(t, subProv, chain[0].ProviderID)
}
```

- [ ] **Step 2: Run — passes**

- [ ] **Step 3: Commit**

```bash
git commit -m "test(routing): override replace-cell — агрегаторская failover-цепочка отбрасывается"
```

---

### Task 13: Удалить устаревшие matcher-тесты

**Files:**
- Modify: `internal/services/routing/application/matcher_test.go`

- [ ] **Step 1: Удалить тесты бинарной модели**

Удалить (или пометить `t.Skip`):
- `TestMatch_ClientSpecificRoute_TakesPrecedenceOverDefault`
- `TestMatch_FallsBackToDefaultWhenNoClientRouteMatches`
- `TestMatch_PriorityOrdering_LowerPriorityFirst` (если покрывает плоскую priority-логику)
- `TestMatchWithDetails_ClientRouteExistsButNoMatch_FallsBackToDefault`

Эквивалентное покрытие уже дают Task 9–12.

- [ ] **Step 2: Run полный suite**

```bash
go test ./internal/services/routing/... -v
```

Ожидать: PASS.

- [ ] **Step 3: Commit**

```bash
git commit -m "test(routing): удалить тесты бинарной client/default модели"
```

---

## Phase 5: Portal / gRPC

### Task 14: Обновить routing.proto

**Files:**
- Modify: `api/proto/routing/routing.proto`
- Generated: `api/proto/routing/*.pb.go` (regen)

- [ ] **Step 1: Обновить proto**

```protobuf
enum RouteOwnerType {
  ROUTE_OWNER_UNSPECIFIED = 0;
  ROUTE_OWNER_PLATFORM    = 1;
  ROUTE_OWNER_CLIENT      = 2;
  ROUTE_OWNER_SUBACCOUNT  = 3;
}

message ClientRouteProto {
  string id = 1;
  RouteOwnerType owner_type = 2;
  string owner_id = 3;        // empty для platform
  string route_type = 4;

  string operator_id = 5;     // empty = wildcard
  string country_code = 6;
  string traffic_type = 7;
  string paid_name = 8;
  string regex = 9;
  string schedule_id = 10;

  string provider_id = 11;
  int32 priority = 12;
  // ... existing fields (name, active, etc.)
}

message ResolveRoutesRequest {
  string route_type = 1;
  string subaccount_id = 2;
  string client_id = 3;
  string operator_id = 4;
  string country_code = 5;
  string traffic_type = 6;
  string paid_name = 7;
  string phone_number = 8;
  google.protobuf.Timestamp at = 9;
}

message ResolveRoutesResponse {
  repeated ClientRouteProto chain = 1;
  string matched_rule_id = 2;
  RouteOwnerType matched_owner = 3;
}

service RoutingService {
  rpc ResolveRoutes(ResolveRoutesRequest) returns (ResolveRoutesResponse);
  // + существующие CRUD (остаются)
}
```

- [ ] **Step 2: Regen**

```bash
buf generate
# или: protoc ...
```

- [ ] **Step 3: Компиляция**

```bash
go build ./...
```

Ожидать: handlers могут падать — исправляются в следующих задачах.

- [ ] **Step 4: Commit**

```bash
git add api/proto/routing/
git commit -m "feat(routing): proto — RouteOwnerType enum, ResolveRoutes RPC"
```

---

### Task 15: assertOwnable — единая проверка RBAC

**Files:**
- Create: `internal/gateway/portal/handlers/routing_authz.go`
- Test: `internal/gateway/portal/handlers/routing_authz_test.go`

- [ ] **Step 1: Test**

```go
func TestAssertOwnable(t *testing.T) {
    platformAdmin := User{Role: "admin"}
    reseller := User{Role: "reseller", ClientID: resellerID}
    subaccount := User{Role: "client", ClientID: subID, ParentClientID: &resellerID}
    regular := User{Role: "client", ClientID: regularID}

    tests := []struct {
        user      User
        ownerType domain.OwnerType
        ownerID   *uuid.UUID
        wantErr   bool
    }{
        {platformAdmin, domain.OwnerPlatform, nil, false},
        {reseller, domain.OwnerClient, &resellerID, false},
        {reseller, domain.OwnerSubaccount, &subID, false},       // subID — его суб-аккаунт
        {reseller, domain.OwnerSubaccount, &otherSubID, true},   // чужой суб-аккаунт
        {subaccount, domain.OwnerSubaccount, &subID, false},
        {subaccount, domain.OwnerClient, &resellerID, true},     // суб-аккаунт не управляет родителем
        {regular, domain.OwnerClient, &regularID, false},
        {regular, domain.OwnerPlatform, nil, true},
    }
    for _, tc := range tests {
        err := AssertOwnable(ctx, db, tc.user, tc.ownerType, tc.ownerID)
        if tc.wantErr {
            require.Error(t, err)
        } else {
            require.NoError(t, err)
        }
    }
}
```

- [ ] **Step 2: Run — fails**

- [ ] **Step 3: Реализация**

```go
package handlers

import (
    "context"
    "errors"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

var ErrForbiddenOwner = errors.New("user cannot manage rules for this owner")

func AssertOwnable(ctx context.Context, db *pgxpool.Pool, user User, ownerType domain.OwnerType, ownerID *uuid.UUID) error {
    switch ownerType {
    case domain.OwnerPlatform:
        if user.Role != "admin" {
            return ErrForbiddenOwner
        }
        return nil
    case domain.OwnerClient:
        if ownerID == nil || *ownerID != user.ClientID {
            return ErrForbiddenOwner
        }
        return nil
    case domain.OwnerSubaccount:
        if ownerID == nil {
            return ErrForbiddenOwner
        }
        // Либо сам суб-аккаунт, либо его родитель-реселлер
        if *ownerID == user.ClientID {
            return nil
        }
        var parentID uuid.UUID
        err := db.QueryRow(ctx,
            `SELECT parent_client_id FROM clients WHERE id = $1 AND parent_client_id IS NOT NULL`,
            *ownerID,
        ).Scan(&parentID)
        if err != nil {
            return ErrForbiddenOwner
        }
        if parentID != user.ClientID {
            return ErrForbiddenOwner
        }
        return nil
    }
    return ErrForbiddenOwner
}
```

- [ ] **Step 4: Run — passes**

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(routing): AssertOwnable — единая проверка RBAC для routing handlers"
```

---

### Task 16: CRUD handler reseller (client + subaccount rules)

**Files:**
- Modify: `internal/gateway/portal/handlers/reseller_routing.go`

- [ ] **Step 1: Test — создание правила своего суб-аккаунта**

```go
func TestResellerRouting_CreateRule_ForOwnSubaccount_Ok(t *testing.T) {
    // reseller аутентифицирован как self.ClientID
    body := `{"owner_type":"subaccount","owner_id":"<subID>","route_type":"sms","operator_id":"<opID>","provider_id":"<provID>"}`
    resp := POST(t, "/portal/reseller/routing/rules", body, resellerToken)
    require.Equal(t, 201, resp.StatusCode)
}

func TestResellerRouting_CreateRule_ForOtherResellersSub_Forbidden(t *testing.T) {
    body := `{"owner_type":"subaccount","owner_id":"<otherSubID>",...}`
    resp := POST(t, "/portal/reseller/routing/rules", body, resellerToken)
    require.Equal(t, 403, resp.StatusCode)
}
```

- [ ] **Step 2: Run — fails**

- [ ] **Step 3: Реализация**

```go
func (h *ResellerRoutingHandlers) CreateRule(w http.ResponseWriter, r *http.Request) {
    var req CreateRuleRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), 400)
        return
    }
    user := CurrentUser(r)
    if err := AssertOwnable(r.Context(), h.pool, user, req.OwnerType, req.OwnerID); err != nil {
        http.Error(w, err.Error(), 403)
        return
    }
    route := req.ToDomain()
    if err := h.repo.Insert(r.Context(), route); err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    writeJSON(w, 201, route)
}

// List — фильтр по owner:
// WHERE (owner_type='client' AND owner_id=$self)
//    OR (owner_type='subaccount' AND owner_id IN (SELECT id FROM clients WHERE parent_client_id=$self))
```

- [ ] **Step 4: Run — passes**

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(routing): reseller handler CRUD для client+subaccount rules"
```

---

### Task 17: CRUD handler subaccount/regular-client

**Files:**
- Modify: `internal/gateway/portal/handlers/sub_account_routing.go` (или общий `routing.go` для обычных клиентов)

- [ ] **Step 1: Test**

```go
func TestSubaccountRouting_CreateOwnRule_Ok(t *testing.T) {
    body := `{"route_type":"sms","operator_id":"<opID>","provider_id":"<provID>"}`
    // owner_type и owner_id выводятся из токена: OwnerSubaccount + self.ID
    resp := POST(t, "/portal/routing/rules", body, subaccountToken)
    require.Equal(t, 201, resp.StatusCode)
}

func TestSubaccountRouting_TryCreatePlatformRule_Forbidden(t *testing.T) {
    body := `{"owner_type":"platform",...}`
    resp := POST(t, "/portal/routing/rules", body, subaccountToken)
    require.Equal(t, 403, resp.StatusCode)
}
```

- [ ] **Step 2-4: Аналогично Task 16.** Handler выводит owner_type/owner_id из контекста пользователя (не из body), но body может переопределить — и тогда AssertOwnable проверит.

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(routing): subaccount/regular-client CRUD handler"
```

---

### Task 18: Admin handler platform rules + защита singleton

**Files:**
- Modify: `internal/gateway/admin/handlers/routing.go`

- [ ] **Step 1: Test на защиту singleton**

```go
func TestAdminRouting_DeletePlatformGeneral_Forbidden(t *testing.T) {
    // существующее правило platform-general-sms
    resp := DELETE(t, "/admin/routing/rules/"+platformGeneralID.String(), adminToken)
    require.Equal(t, 409, resp.StatusCode, "удалять platform general запрещено")
}
```

- [ ] **Step 2: Run — fails (если handler ещё не защищает)**

- [ ] **Step 3: Реализация защиты**

```go
func (h *AdminRoutingHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    route, err := h.repo.GetByID(r.Context(), uuid.MustParse(id))
    if err != nil {
        http.Error(w, err.Error(), 404)
        return
    }
    if route.OwnerType == domain.OwnerPlatform && route.IsGeneral() {
        // Проверить, что это последний platform-general для этого route_type
        count, _ := h.repo.CountPlatformGeneral(r.Context(), route.RouteType)
        if count <= 1 {
            http.Error(w, "cannot delete last platform-general rule for route_type", 409)
            return
        }
    }
    if err := h.repo.Delete(r.Context(), route.ID); err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    w.WriteHeader(204)
}
```

В репозитории добавить `CountPlatformGeneral(ctx, routeType) (int, error)`.

- [ ] **Step 4: Run — passes**

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(routing): admin handler с защитой singleton platform-general"
```

---

### Task 19: ResolveRoutes gRPC endpoint

**Files:**
- Modify: `internal/services/routing/grpc/server.go` (или эквивалент — уточнить)
- Test: `internal/services/routing/grpc/server_test.go`

- [ ] **Step 1: Test — ResolveRoutes возвращает цепочку**

```go
func TestResolveRoutes_ReturnsFailoverChain(t *testing.T) {
    srv := setupGRPCServer(t)
    resp, err := srv.ResolveRoutes(ctx, &routingv1.ResolveRoutesRequest{
        RouteType: "sms",
        SubaccountId: subID.String(),
        ClientId: clID.String(),
        OperatorId: opID.String(),
        CountryCode: "RU",
    })
    require.NoError(t, err)
    require.Len(t, resp.Chain, 3)
    require.Equal(t, routingv1.RouteOwnerType_ROUTE_OWNER_SUBACCOUNT, resp.MatchedOwner)
}
```

- [ ] **Step 2: Run — fails**

- [ ] **Step 3: Реализация**

```go
func (s *Server) ResolveRoutes(ctx context.Context, req *routingv1.ResolveRoutesRequest) (*routingv1.ResolveRoutesResponse, error) {
    mctx := application.MatchContext{
        RouteType:   req.RouteType,
        ClientID:    uuid.MustParse(req.ClientId),
        OperatorID:  parseUUIDPtr(req.OperatorId),
        CountryCode: req.CountryCode,
        TrafficType: req.TrafficType,
        PaidName:    req.PaidName,
        PhoneNumber: req.PhoneNumber,
    }
    if req.SubaccountId != "" {
        sub := uuid.MustParse(req.SubaccountId)
        mctx.SubaccountID = &sub
    }
    chain := s.matcher.Match(mctx)
    if len(chain) == 0 {
        return nil, status.Error(codes.NotFound, "no route found")
    }
    return &routingv1.ResolveRoutesResponse{
        Chain:         mapToProto(chain),
        MatchedRuleId: chain[0].ID.String(),
        MatchedOwner:  toProtoOwnerType(chain[0].OwnerType),
    }, nil
}
```

- [ ] **Step 4: Run — passes**

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(routing): gRPC ResolveRoutes endpoint"
```

---

### Task 20: Интеграция с pipeline (вызов ResolveRoutes из tarification/outbound)

**Files:**
- Modify: пайплайн — уточнить через `grep -rn "routing.*Match" internal/`

- [ ] **Step 1: Найти вызывающую сторону**

```bash
grep -rn "matcher.Match\|MatchContext{" internal/ --include="*.go" | grep -v _test.go
```

- [ ] **Step 2: Обновить вызов**

Убедиться, что в вызывающем коде есть и `SubaccountID` и `ClientID`. Если есть только `ClientID` — дополнить из БД (`SELECT parent_client_id FROM clients WHERE id = $1`).

- [ ] **Step 3: Тест pipeline (если есть интеграционный)**

```bash
go test ./internal/pipeline/... -v
```

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(routing): pipeline передаёт SubaccountID в MatchContext"
```

---

## Phase 6: Cleanup

### Task 21: Drop старых таблиц

**Files:**
- Create: `migrations/000109_routing_drop_conditions.up.sql`
- Create: `migrations/000109_routing_drop_conditions.down.sql`

- [ ] **Step 1: Grep — нет ли живого кода с condition_groups**

```bash
grep -rn "route_condition_groups\|route_conditions\|ConditionGroup" internal/ api/
```

Ожидать: остались только комментарии/миграции. Если нашёлся живой код — исправить (отдельной задачей) и только потом продолжать.

- [ ] **Step 2: Up-миграция**

```sql
-- 000109_routing_drop_conditions.up.sql
BEGIN;

DROP TABLE IF EXISTS route_conditions CASCADE;
DROP TABLE IF EXISTS route_condition_groups CASCADE;

COMMIT;
```

- [ ] **Step 3: Down-миграция**

Down делает ресторацию таблиц, если прод откатывается. Реалистично — dumpsql их схемы из 000077 и положить сюда обратно.

```sql
-- 000109_routing_drop_conditions.down.sql
BEGIN;

CREATE TABLE route_condition_groups ( ... );  -- из 000077
CREATE TABLE route_conditions ( ... );

COMMIT;
```

- [ ] **Step 4: Применить, проверить**

```bash
./scripts/server.sh migrate
go test ./... -short
```

- [ ] **Step 5: Commit**

```bash
git commit -m "chore(routing): миграция 109 — DROP route_conditions/route_condition_groups"
```

---

### Task 22: Финальный прогон и docs

**Files:**
- Modify: `CLAUDE.md` (Recent Changes секция)

- [ ] **Step 1: Полный тест-прогон**

```bash
./scripts/check.sh --with-tests
```

Ожидать: PASS.

- [ ] **Step 2: Обновить CLAUDE.md**

Добавить запись в Recent Changes:

```
- routing: hierarchy refactor (platform/client/subaccount), override-by-cell, specificity-based ranking
```

- [ ] **Step 3: Commit**

```bash
git commit -m "chore: routing hierarchy — CLAUDE.md recent changes"
```

---

## Self-Review

**Spec coverage (сверка с разделами спека):**

- ✅ Уровни platform/client/subaccount — Tasks 3, 4, 15
- ✅ Модель владения (owner_type/owner_id) — Tasks 1, 4
- ✅ Ключ specificity — Tasks 5, 9
- ✅ Семантика replace-cell — Tasks 9, 12
- ✅ Platform general singleton — Tasks 2 (constraint), 18 (защита delete)
- ✅ Route type изоляция — Tasks 9, 11
- ✅ Матчинг алгоритм — Task 9
- ✅ Схема БД — Tasks 1, 2
- ✅ Миграция данных — Task 2, 21
- ✅ Единая RPC ResolveRoutes — Tasks 14, 19
- ✅ CRUD handlers per role — Tasks 16, 17, 18
- ✅ assertOwnable — Task 15
- ✅ Тесты иерархии — Tasks 10, 11, 12
- ✅ Pre-flight check — Task 0

**Placeholder scan:** чисто. `<default_provider>` в Task 2 — параметр оператора, описан явно с fallback поведением (миграция падает с сообщением).

**Type consistency:**
- `OwnerType` — одинаково в domain, proto, handler.
- `KeyFieldsFilled` — одна функция, используется в matcher.
- `MatchContext.OwnerChain()` — возвращает slice, consumer — `Match()` — итерируется по нему.
- `AssertOwnable` — сигнатура совпадает во всех местах вызова.

Зазор: в Task 6 я сослался на `internal/services/routing/infrastructure/route_repo.go` как путь, но по результатам `ls` точный файл репозитория не проверил — исполнитель должен начать с `grep -rn "LoadAllActive" internal/services/routing/`, это в Task 6 Step 1 явно указано.
