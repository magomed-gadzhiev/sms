# Aggregator Routing — Plan 3: Hardening & Polish — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Закрыть hardening-долги после Plans 1+2 — устранить override-конфликт `uq_cell_provider`, добавить defence-in-depth middleware, перевести материализаторы на idempotent retry-pattern, дедуплицировать helper'ы, обновить ESLint baseline и закрыть мелкие гигиенические follow-ups.

**Architecture:** Точечные правки поверх существующей routing-инфраструктуры. Без новых сервисов, без новых таблиц (кроме одной trigger-функции для orphan-cleanup и одной миграции для drop UNIQUE). Каждая задача self-contained, проходит через `/execute-with-review` (implementer → spec-reviewer → code-quality reviewer).

**Tech Stack:** Go 1.24.0, gorilla/mux, pgx/v5, PostgreSQL 15+, React 19/TypeScript 5.7, Vite 6, ESLint, testify, Docker Compose dev container на sandbox-server.

**Spec:** `docs/superpowers/specs/2026-05-04-aggregator-routing-management-design.md`
**Предыдущие планы:** `2026-05-04-aggregator-routing-plan-1-DONE.md`, `2026-05-04-aggregator-routing-plan-2-DONE.md`

**Скоуп (согласовано):**
- A1 — Drop `uq_cell_provider` + pre-check дубликата в override handler
- A2 — Mount ResellerOnlyMiddleware на `/sub-accounts/*` subrouter
- A3 — Materialiser partial-failure → 200+warnings вместо 500
- B4 — Trigger orphan-cleanup в `subaccount_routing_assignment`
- B5 — Извлечь `verifySubAccountOwnership` в shared helper
- C8 — Tooltip-предупреждение про operator-conditions в Preview UI
- D14 — Lockdown ESLint baseline 56→51
- D15 — Migration gap audit (DB diff vs migrations)
- E16 — Materializer concurrency stress test
- E17 — Unskip Bulk-conflict E2E

**OUT of scope:** C7 audit-log UI (deferred §6.7), D13 Redis hardening (отдельный security-план), B6/C9/E18 (WONTFIX).

**Testing pattern (наследуем из Plan 2):**
```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/...path..."
```

**Mandatory rules (из памяти feedback_*):**
- Каждая task через `/execute-with-review` wrapper. Никогда не пропускать review-gate.
- Никогда `git commit --amend` или force-push на master. Каждый fix — новый commit.
- `git add` только нужные файлы; никогда `git add .` или `-A`.
- Verify before asserting: implementer обязан читать актуальный source перед написанием тест-assertion'ов.
- No code blocks в чате — код только в plan/spec документах.
- Frontend: no `any` в TS; ESLint baseline после Task 8 = 51.

---

## Task 1: Drop `uq_cell_provider` UNIQUE constraint (миграция)

**Контекст:** `uq_cell_provider` существует только в sandbox DB (не создан ни одной миграцией репо — см. `migrations/000137_client_routes_owner_type.down.sql:8`). Constraint блокирует override на тот же `(client_id, provider_id, route_type)`, что в template, потому что cell-колонки (`operator_id`, `country_code`, `traffic_type`, `number_from`, `number_to`) при материализации все NULL. Решение — добавить миграцию `DROP CONSTRAINT IF EXISTS`, чтобы привести sandbox в соответствие с дизайном «матчинг через condition_groups, не через cell-колонки».

**Files:**
- Create: `migrations/000138_drop_client_routes_uq_cell_provider.up.sql`
- Create: `migrations/000138_drop_client_routes_uq_cell_provider.down.sql`

- [ ] **Step 1: Написать UP-миграцию**

Файл `migrations/000138_drop_client_routes_uq_cell_provider.up.sql`:

```sql
-- Drop sandbox-only UNIQUE constraint uq_cell_provider.
--
-- Контекст: constraint существовал только в sandbox DB (не создавался миграциями
-- репо, см. 000137 down-комментарий). Блокировал override-маршруты на тот же
-- (provider_id, route_type), что в template, потому что cell-колонки
-- (operator_id, country_code, traffic_type, number_from, number_to) при
-- материализации route-set'а пишутся NULL — матчинг идёт через
-- route_condition_groups + route_conditions.
--
-- Защита от смысловых дубликатов переезжает в handler уровень
-- (Task 2: pre-check signature в AddRouteOverride / UpdateRouteOverride).
ALTER TABLE client_routes DROP CONSTRAINT IF EXISTS uq_cell_provider;
```

- [ ] **Step 2: Написать DOWN-миграцию**

Файл `migrations/000138_drop_client_routes_uq_cell_provider.down.sql`:

```sql
-- Восстановить sandbox-only constraint (best-effort; в чистом dev этой строки
-- никогда не было). UNIQUE-выражение точно совпадает со sandbox-формой,
-- зафиксированной в \d client_routes на 2026-05-05.
ALTER TABLE client_routes ADD CONSTRAINT uq_cell_provider UNIQUE
  (owner_type, COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid),
   route_type, COALESCE(operator_id, '00000000-0000-0000-0000-000000000000'::uuid),
   COALESCE(country_code, ''::bpchar), COALESCE(traffic_type, ''::text),
   COALESCE(number_from, '-1'::integer::bigint), COALESCE(number_to, '-1'::integer::bigint),
   provider_id);
```

- [ ] **Step 3: Применить миграцию на sandbox**

```bash
git add migrations/000138_drop_client_routes_uq_cell_provider.up.sql migrations/000138_drop_client_routes_uq_cell_provider.down.sql
git commit -m "migrate: drop sandbox-only uq_cell_provider on client_routes (Plan 3 Task 1)"
git push origin master
./scripts/server.sh sync
./scripts/server.sh migrate
```

Expected: миграция применена, в `\d client_routes` constraint `uq_cell_provider` отсутствует.

- [ ] **Step 4: Verify constraint dropped**

```bash
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T postgres psql -U smpp -d smpp_db -c \"\\d client_routes\" | grep -i uq_cell || echo 'CONSTRAINT GONE'"
```

Expected output: `CONSTRAINT GONE`.

---

## Task 2: Pre-check дубликата в AddRouteOverride / UpdateRouteOverride

**Контекст:** Constraint снят (Task 1) — теперь handler единственный защитник от смысловых дубликатов. Сценарий: двойной POST `/route-overrides` из UI создаст две одинаковые строки в `client_routes`, обе будут матчить router → ambiguous priority. Решение: pre-check на дубликат по signature `(provider_id, route_type, condition_groups_canonicalized)` в той же транзакции. Дубликат → 409 с указанием конфликтующего route_id.

**Files:**
- Create: `internal/services/network/route_signature.go`
- Create: `internal/services/network/route_signature_test.go`
- Modify: `internal/gateway/portal/handlers/subaccount_network_overrides.go` (AddRouteOverride, UpdateRouteOverride)
- Modify: `internal/gateway/portal/handlers/subaccount_network_overrides_test.go`

- [ ] **Step 1: Написать failing test для signature canonicalization**

Файл `internal/services/network/route_signature_test.go`:

```go
package network

import (
	"testing"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/storage"
)

func TestRouteSignature_StableForReorderedConditions(t *testing.T) {
	provID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	a := storage.RouteSetItemFull{
		ProviderID: provID,
		RouteType:  "sms",
		ConditionGroups: []storage.RouteSetConditionGroup{
			{LogicOp: "AND", Conditions: []storage.RouteSetCondition{
				{Type: "country", Value: "RU"},
				{Type: "operator", Value: "megafon"},
			}},
		},
	}
	b := storage.RouteSetItemFull{
		ProviderID: provID,
		RouteType:  "sms",
		ConditionGroups: []storage.RouteSetConditionGroup{
			{LogicOp: "AND", Conditions: []storage.RouteSetCondition{
				{Type: "operator", Value: "megafon"},
				{Type: "country", Value: "RU"},
			}},
		},
	}
	if RouteSignature(a) != RouteSignature(b) {
		t.Fatalf("signature must be order-independent within group: %s vs %s",
			RouteSignature(a), RouteSignature(b))
	}
}

func TestRouteSignature_DifferentProvidersDiffer(t *testing.T) {
	a := storage.RouteSetItemFull{
		ProviderID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		RouteType:  "sms",
	}
	b := storage.RouteSetItemFull{
		ProviderID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		RouteType:  "sms",
	}
	if RouteSignature(a) == RouteSignature(b) {
		t.Fatal("different provider_id must produce different signature")
	}
}

func TestRouteSignature_GroupOrderMatters(t *testing.T) {
	// AND-of-OR vs OR-of-AND — разная семантика, signature должна различаться.
	provID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	a := storage.RouteSetItemFull{
		ProviderID: provID, RouteType: "sms",
		ConditionGroups: []storage.RouteSetConditionGroup{
			{LogicOp: "AND", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "RU"}}},
			{LogicOp: "OR", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "BY"}}},
		},
	}
	b := storage.RouteSetItemFull{
		ProviderID: provID, RouteType: "sms",
		ConditionGroups: []storage.RouteSetConditionGroup{
			{LogicOp: "OR", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "BY"}}},
			{LogicOp: "AND", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "RU"}}},
		},
	}
	if RouteSignature(a) == RouteSignature(b) {
		t.Fatal("different group order must produce different signature")
	}
}

func TestRouteSignature_EmptyConditionsStable(t *testing.T) {
	provID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	a := storage.RouteSetItemFull{ProviderID: provID, RouteType: "sms"}
	b := storage.RouteSetItemFull{ProviderID: provID, RouteType: "sms"}
	if RouteSignature(a) != RouteSignature(b) {
		t.Fatal("two empty-condition routes must share signature")
	}
}
```

- [ ] **Step 2: Запустить тест — должен FAIL (no such function)**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/services/network/ -run RouteSignature -v"
```

Expected: undefined: `RouteSignature`.

- [ ] **Step 3: Реализовать RouteSignature**

Файл `internal/services/network/route_signature.go`:

```go
package network

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/smpp-server/smpp-server/internal/storage"
)

// RouteSignature вычисляет стабильный hash от смысловой подписи маршрута:
// (provider_id, route_type, ordered list of (group_index, logic_op, sorted conditions)).
//
// Внутри каждой группы условия сортируются по (type, value) — порядок ввода в UI
// не влияет на signature (две одинаковые правила в разном порядке = дубликат).
// Порядок групп сохраняется (group_index важен — AND/OR композиция различна).
//
// Используется в handler-pre-check'е для детекции смыслового дубликата
// override-маршрута до INSERT'а (см. AddRouteOverride / UpdateRouteOverride).
func RouteSignature(item storage.RouteSetItemFull) string {
	var b strings.Builder
	b.WriteString(item.ProviderID.String())
	b.WriteString("|")
	b.WriteString(item.RouteType)
	b.WriteString("|")
	for idx, g := range item.ConditionGroups {
		b.WriteString("g")
		// Используем idx, а не g.GroupIndex — для override item'ов без БД-id поле GroupIndex = 0
		// у всех групп; idx из массива — единственный надёжный порядок.
		b.WriteString(itoa(idx))
		b.WriteString(":")
		b.WriteString(g.LogicOp)
		b.WriteString("(")
		conds := make([]storage.RouteSetCondition, len(g.Conditions))
		copy(conds, g.Conditions)
		sort.Slice(conds, func(i, j int) bool {
			if conds[i].Type != conds[j].Type {
				return conds[i].Type < conds[j].Type
			}
			return conds[i].Value < conds[j].Value
		})
		for _, c := range conds {
			b.WriteString(c.Type)
			b.WriteString("=")
			b.WriteString(c.Value)
			b.WriteString(",")
		}
		b.WriteString(")")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func itoa(n int) string {
	// Локальная мини-itoa, чтобы не тащить strconv (микро-аллокация).
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
```

- [ ] **Step 4: Запустить тест — должен PASS**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/services/network/ -run RouteSignature -v"
```

Expected: 4 PASS.

- [ ] **Step 5: Написать failing integration test для pre-check в AddRouteOverride**

Файл `internal/gateway/portal/handlers/subaccount_network_overrides_test.go` (добавить test-функцию в конец файла):

```go
func TestAddRouteOverride_DuplicateSignature_Returns409(t *testing.T) {
	pool, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	resellerID := testutil.SeedReseller(t, pool, "reseller@test.local")
	subID := testutil.SeedSubAccount(t, pool, resellerID, "sub@test.local")
	provID := testutil.SeedProvider(t, pool, "test-provider", "platform")
	testutil.SeedClientProvider(t, pool, subID, provID, "private", true)

	h := NewSubAccountNetworkOverridesHandlers(pool)

	body := `{"provider_id":"` + provID.String() + `","priority":10,"share":100,"route_type":"sms","status":"active","condition_groups":[{"logic_op":"AND","conditions":[{"type":"country","value":"RU"}]}]}`

	// Первый POST — 201.
	r1 := httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides", strings.NewReader(body))
	r1 = mux.SetURLVars(r1, map[string]string{"id": subID.String()})
	r1 = r1.WithContext(middleware.WithClientID(r1.Context(), resellerID))
	w1 := httptest.NewRecorder()
	h.AddRouteOverride(w1, r1)
	require.Equal(t, http.StatusCreated, w1.Code, "first POST: %s", w1.Body.String())

	// Второй POST с теми же полями — 409 с kind=duplicate_route_signature.
	r2 := httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides", strings.NewReader(body))
	r2 = mux.SetURLVars(r2, map[string]string{"id": subID.String()})
	r2 = r2.WithContext(middleware.WithClientID(r2.Context(), resellerID))
	w2 := httptest.NewRecorder()
	h.AddRouteOverride(w2, r2)
	require.Equal(t, http.StatusConflict, w2.Code, "second POST: %s", w2.Body.String())
	require.Contains(t, w2.Body.String(), "duplicate_route_signature")
}

func TestAddRouteOverride_DifferentConditionsSameProvider_Returns201(t *testing.T) {
	pool, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	resellerID := testutil.SeedReseller(t, pool, "reseller@test.local")
	subID := testutil.SeedSubAccount(t, pool, resellerID, "sub@test.local")
	provID := testutil.SeedProvider(t, pool, "test-provider", "platform")
	testutil.SeedClientProvider(t, pool, subID, provID, "private", true)

	h := NewSubAccountNetworkOverridesHandlers(pool)

	mk := func(country string) string {
		return `{"provider_id":"` + provID.String() + `","priority":10,"share":100,"route_type":"sms","status":"active","condition_groups":[{"logic_op":"AND","conditions":[{"type":"country","value":"` + country + `"}]}]}`
	}

	for _, country := range []string{"RU", "BY"} {
		r := httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides", strings.NewReader(mk(country)))
		r = mux.SetURLVars(r, map[string]string{"id": subID.String()})
		r = r.WithContext(middleware.WithClientID(r.Context(), resellerID))
		w := httptest.NewRecorder()
		h.AddRouteOverride(w, r)
		require.Equal(t, http.StatusCreated, w.Code, "country=%s: %s", country, w.Body.String())
	}
}
```

**Verify before asserting:** перед запуском проверь существование `testutil.SeedReseller`, `SeedSubAccount`, `SeedProvider`, `SeedClientProvider`, `middleware.WithClientID`, `middleware.GetClientID`. Если каких-то helper'ов нет — найди эквивалент в `internal/testutil/` через `Grep` и адаптируй вызовы. Не угадывай сигнатуры.

```bash
grep -rn "func SeedReseller\|func SeedSubAccount\|func SeedProvider\|func SeedClientProvider\|func WithClientID\|func SetupTestDB" internal/testutil/ internal/gateway/portal/middleware/
```

- [ ] **Step 6: Запустить тест — должен FAIL (старый код не возвращает 409 на дубликат)**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/gateway/portal/handlers/ -run TestAddRouteOverride_Duplicate -v"
```

Expected: FAIL — второй POST возвращает 201 вместо 409 (после Task 1 UNIQUE снят, дубликат проходит).

- [ ] **Step 7: Реализовать pre-check в AddRouteOverride**

Modify `internal/gateway/portal/handlers/subaccount_network_overrides.go` (метод `AddRouteOverride`, после `parseItemIn` и `tx, err := h.pool.Begin`, до INSERT'а):

```go
	// Pre-check duplicate signature: после drop'а uq_cell_provider (миграция 000138)
	// единственная защита от смыслового дубликата — handler-уровень. Считаем signature
	// от (provider_id, route_type, condition_groups) и ищем существующий override
	// с тем же signature внутри транзакции (TOCTOU-safe).
	sig := network.RouteSignature(full)
	if existingID, dupErr := findDuplicateOverrideSignature(r.Context(), tx, subID, sig); dupErr != nil {
		log.Error().Err(dupErr).Str("sub_id", subID.String()).Msg("route override duplicate-check")
		respondError(w, shared.ErrInternalServer("duplicate-check"))
		return
	} else if existingID != uuid.Nil {
		details, _ := json.Marshal(map[string]interface{}{
			"kind":         "duplicate_route_signature",
			"existing_id":  existingID.String(),
		})
		respondError(w, shared.ErrConflict("Такой override уже существует").WithDetails(string(details)))
		return
	}
```

И добавить helper в тот же файл (рядом с `writeRouteGroupsAndSchedules`):

```go
// findDuplicateOverrideSignature ищет в транзакции override-маршруты sub-аккаунта
// с тем же signature, что и кандидат на INSERT/UPDATE. excludeID — route, который
// мы апдейтим (исключается из поиска); uuid.Nil = ничего не исключать.
//
// Возвращает (uuid.Nil, nil) если дубликата нет.
func findDuplicateOverrideSignature(ctx context.Context, tx pgx.Tx, subID uuid.UUID, candidateSig string) (uuid.UUID, error) {
	rows, err := tx.Query(ctx, `
		SELECT cr.id, cr.provider_id, cr.route_type,
		       COALESCE(json_agg(json_build_object(
		         'group_index', g.group_index,
		         'logic_op',    g.logic_op,
		         'conditions',  COALESCE((SELECT json_agg(json_build_object('type', c.condition_type, 'value', c.condition_value) ORDER BY c.condition_type, c.condition_value) FROM route_conditions c WHERE c.group_id = g.id), '[]'::json)
		       ) ORDER BY g.group_index) FILTER (WHERE g.id IS NOT NULL), '[]'::json)
		FROM client_routes cr
		LEFT JOIN route_condition_groups g ON g.route_id = cr.id
		WHERE cr.client_id = $1 AND cr.source = 'override'
		GROUP BY cr.id, cr.provider_id, cr.route_type`,
		subID,
	)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, provID uuid.UUID
		var routeType string
		var groupsJSON []byte
		if err := rows.Scan(&id, &provID, &routeType, &groupsJSON); err != nil {
			return uuid.Nil, err
		}
		var rawGroups []struct {
			GroupIndex int16  `json:"group_index"`
			LogicOp    string `json:"logic_op"`
			Conditions []struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"conditions"`
		}
		if err := json.Unmarshal(groupsJSON, &rawGroups); err != nil {
			return uuid.Nil, err
		}
		item := storage.RouteSetItemFull{ProviderID: provID, RouteType: routeType}
		for _, g := range rawGroups {
			grp := storage.RouteSetConditionGroup{GroupIndex: g.GroupIndex, LogicOp: g.LogicOp}
			for _, c := range g.Conditions {
				grp.Conditions = append(grp.Conditions, storage.RouteSetCondition{Type: c.Type, Value: c.Value})
			}
			item.ConditionGroups = append(item.ConditionGroups, grp)
		}
		if network.RouteSignature(item) == candidateSig {
			return id, nil
		}
	}
	return uuid.Nil, rows.Err()
}
```

И добавить импорт `"github.com/smpp-server/smpp-server/internal/services/network"` если его нет.

- [ ] **Step 8: Тот же pre-check в UpdateRouteOverride с excludeID = routeID**

В `UpdateRouteOverride`, после `parseItemIn(req.itemIn)` и `tx, err := h.pool.Begin`, до `UPDATE client_routes`:

```go
	// Pre-check duplicate signature (исключая текущий route — он сам не должен считаться дубликатом своего нового состояния).
	sig := network.RouteSignature(full)
	if existingID, dupErr := findDuplicateOverrideSignatureExcept(r.Context(), tx, subID, sig, routeID); dupErr != nil {
		log.Error().Err(dupErr).Str("sub_id", subID.String()).Msg("route override update duplicate-check")
		respondError(w, shared.ErrInternalServer("duplicate-check"))
		return
	} else if existingID != uuid.Nil {
		details, _ := json.Marshal(map[string]interface{}{
			"kind":        "duplicate_route_signature",
			"existing_id": existingID.String(),
		})
		respondError(w, shared.ErrConflict("Такой override уже существует").WithDetails(string(details)))
		return
	}
```

И второй helper (или адаптировать первый — на твой выбор; ниже отдельная функция чтобы избежать `if excludeID != uuid.Nil`-ветки):

```go
func findDuplicateOverrideSignatureExcept(ctx context.Context, tx pgx.Tx, subID uuid.UUID, candidateSig string, excludeID uuid.UUID) (uuid.UUID, error) {
	// То же что findDuplicateOverrideSignature, но исключаем excludeID из выборки.
	rows, err := tx.Query(ctx, `
		SELECT cr.id, cr.provider_id, cr.route_type,
		       COALESCE(json_agg(json_build_object(
		         'group_index', g.group_index,
		         'logic_op',    g.logic_op,
		         'conditions',  COALESCE((SELECT json_agg(json_build_object('type', c.condition_type, 'value', c.condition_value) ORDER BY c.condition_type, c.condition_value) FROM route_conditions c WHERE c.group_id = g.id), '[]'::json)
		       ) ORDER BY g.group_index) FILTER (WHERE g.id IS NOT NULL), '[]'::json)
		FROM client_routes cr
		LEFT JOIN route_condition_groups g ON g.route_id = cr.id
		WHERE cr.client_id = $1 AND cr.source = 'override' AND cr.id <> $2
		GROUP BY cr.id, cr.provider_id, cr.route_type`,
		subID, excludeID,
	)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, provID uuid.UUID
		var routeType string
		var groupsJSON []byte
		if err := rows.Scan(&id, &provID, &routeType, &groupsJSON); err != nil {
			return uuid.Nil, err
		}
		var rawGroups []struct {
			GroupIndex int16  `json:"group_index"`
			LogicOp    string `json:"logic_op"`
			Conditions []struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"conditions"`
		}
		if err := json.Unmarshal(groupsJSON, &rawGroups); err != nil {
			return uuid.Nil, err
		}
		item := storage.RouteSetItemFull{ProviderID: provID, RouteType: routeType}
		for _, g := range rawGroups {
			grp := storage.RouteSetConditionGroup{GroupIndex: g.GroupIndex, LogicOp: g.LogicOp}
			for _, c := range g.Conditions {
				grp.Conditions = append(grp.Conditions, storage.RouteSetCondition{Type: c.Type, Value: c.Value})
			}
			item.ConditionGroups = append(item.ConditionGroups, grp)
		}
		if network.RouteSignature(item) == candidateSig {
			return id, nil
		}
	}
	return uuid.Nil, rows.Err()
}
```

- [ ] **Step 9: Запустить тесты — duplicate FAIL → PASS, different-conditions PASS**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/gateway/portal/handlers/ -run TestAddRouteOverride -v"
```

Expected: PASS оба. Также прогнать `./internal/services/network/` ещё раз — RouteSignature тесты остаются PASS.

- [ ] **Step 10: Local check + commit**

```bash
./scripts/check.sh
git add migrations/000138_drop_client_routes_uq_cell_provider.up.sql migrations/000138_drop_client_routes_uq_cell_provider.down.sql internal/services/network/route_signature.go internal/services/network/route_signature_test.go internal/gateway/portal/handlers/subaccount_network_overrides.go internal/gateway/portal/handlers/subaccount_network_overrides_test.go
git status
git commit -m "feat(network): pre-check duplicate signature in route-override handlers (Plan 3 Task 2)

После Task 1 (drop uq_cell_provider) handler — единственный защитник от смысловых
дубликатов override-маршрутов. RouteSignature считает стабильный hash от
(provider_id, route_type, condition_groups), pre-check внутри tx ищет существующий
override с тем же signature → 409 duplicate_route_signature."
```

Expected: pre-commit PASS, коммит создан.

---

## Task 3: Mount ResellerOnlyMiddleware на /sub-accounts/* subrouter

**Контекст:** `/sub-accounts/*` subrouter (`router.go:191`) сейчас под `protected` без `ResellerOnlyMiddleware`. Защита идёт через `verifyOwnership` в каждом handler'е (404 на чужих). Это defence-by-accident: один пропущенный verify в новом endpoint'е = leak. Спецификация Plan 1 §6.1 предписывает middleware-уровень. Решение — `subAccounts.Use(middleware.ResellerOnlyMiddleware(dbPool))`.

**Files:**
- Modify: `internal/gateway/portal/router/router.go` (line 191 area)
- Modify: `internal/gateway/portal/router/router_test.go` (add coverage if file exists; otherwise extend существующий test)

- [ ] **Step 1: Найти существующий router-test pattern**

```bash
grep -rn "ResellerOnlyMiddleware\|TestRouter\|sub-accounts" internal/gateway/portal/router/
```

Если router-test'ов нет (вероятно), используем integration-pattern из `internal/gateway/portal/middleware/reseller_only_test.go` как образец, но для конкретного `/sub-accounts/*` endpoint'а.

- [ ] **Step 2: Написать failing test — non-reseller получает 403 на /sub-accounts**

Добавить в `internal/gateway/portal/middleware/reseller_only_test.go` (новый test-кейс в конец файла):

```go
// TestResellerOnly_AppliedToSubAccountsRouter — гарантирует, что middleware
// навешен на /sub-accounts/* subrouter. Регрессионный тест для Plan 3 Task 3.
//
// Проверка через router setup: создаём минимальный subrouter с middleware.Use(),
// регистрируем /sub-accounts endpoint, шлём запрос с client_id обычного пользователя
// (не reseller, не sub-account) → expect 403.
func TestResellerOnly_AppliedToSubAccountsRouter(t *testing.T) {
	pool, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	clientID := testutil.SeedClient(t, pool, "regular@test.local") // обычный клиент, не reseller

	r := mux.NewRouter()
	subRouter := r.PathPrefix("/sub-accounts").Subrouter()
	subRouter.Use(ResellerOnlyMiddleware(pool))
	subRouter.HandleFunc("", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	req := httptest.NewRequest("GET", "/sub-accounts", nil)
	req = req.WithContext(WithClientID(req.Context(), clientID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code, "non-reseller must be 403")
}
```

**Verify before asserting:** проверь что `testutil.SeedClient` существует (или адаптируй под фактический helper). Проверь, что `WithClientID` экспортируется из package middleware (если нет — найди actual signature через grep).

- [ ] **Step 3: Запустить — должен FAIL (test не падает на текущем коде, потому что router-setup в тесте уже навешивает middleware; этот тест ловит будущую регрессию, не текущую)**

Замечание: этот test проверяет middleware *в изоляции*, не реальный production router. Он PASS уже сегодня. Чтобы поймать реальный production gap, добавим второй test, который собирает productive router и проверяет на нём.

Добавить в `internal/gateway/portal/router/router_test.go` (создать файл если не существует):

```go
package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/testutil"
)

// TestSubAccountsRouter_NonResellerForbidden — проверяет, что productive
// router-setup защищает /sub-accounts/* через ResellerOnlyMiddleware.
func TestSubAccountsRouter_NonResellerForbidden(t *testing.T) {
	pool, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	clientID := testutil.SeedClient(t, pool, "regular@test.local")

	// Build router with all dependencies. Используем существующий конструктор.
	// Если он требует много nil-ables — собираем минимум, остальное nil.
	router := NewRouter(testutil.MinimalRouterDeps(pool)) // адаптируй под реальный конструктор

	req := httptest.NewRequest("GET", "/portal/v1/sub-accounts", nil)
	req = req.WithContext(middleware.WithClientID(req.Context(), clientID))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code, "non-reseller на /sub-accounts: %s", w.Body.String())
}
```

**Verify before asserting:** прочитать сигнатуру `NewRouter` в `internal/gateway/portal/router/router.go`. Если конструктор тяжёлый (десятки параметров) — НЕ создавать `MinimalRouterDeps`, вместо этого использовать тот же приём, что в Step 2 (изолированный mini-subrouter), и закрыть production gap единственным проверочным запросом через minimal e2e против реального запущенного сервиса (см. Step 5 ниже).

- [ ] **Step 4: Применить middleware в router.go**

Modify `internal/gateway/portal/router/router.go` около строки 191:

```go
	// Sub-accounts endpoints
	subAccounts := protected.PathPrefix("/sub-accounts").Subrouter()
	// Plan 3 Task 3: defence-in-depth — middleware на subrouter'е гарантирует,
	// что новые endpoint'ы под /sub-accounts/* автоматически получают reseller-guard.
	// До Plan 3 защита шла только через verifyOwnership в каждом handler'е (404
	// на чужих) — defence-by-accident, один пропущенный verify = leak.
	subAccounts.Use(middleware.ResellerOnlyMiddleware(dbPool))
```

- [ ] **Step 5: Запустить тесты — должны PASS**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/gateway/portal/middleware/ -run TestResellerOnly -v"
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/gateway/portal/router/... -v"
```

Expected: PASS обе test-функции.

- [ ] **Step 6: Smoke-проверка через реальный sandbox (non-reseller на /sub-accounts → 403)**

```bash
# Создать тест-юзера с ролью обычного клиента (не reseller, не sub-account) или использовать существующий
# Залогиниться, получить session token, дёрнуть GET /portal/v1/sub-accounts.
# Expected: HTTP 403 Forbidden.
```

Если такого test-account'а нет — пропустить smoke и положиться на test из Step 3 + ручную проверку через прямой curl с заголовком Cookie.

- [ ] **Step 7: Local check + commit**

```bash
./scripts/check.sh
git add internal/gateway/portal/router/router.go internal/gateway/portal/middleware/reseller_only_test.go
# Если router_test.go был создан — добавить и его.
git status
git commit -m "fix(portal): mount ResellerOnlyMiddleware on /sub-accounts subrouter (Plan 3 Task 3)

Defence-in-depth: до этого защита /sub-accounts/* шла только через
verifyOwnership в каждом handler'е (404 на чужих). Один пропущенный verify
в новом endpoint'е = leak. Middleware на subrouter'е гарантирует, что новые
endpoint'ы автоматически получают reseller-guard."
```

---

## Task 4: Materialiser partial-failure → 200+warnings (PutOne, Bulk)

**Контекст:** Сейчас `PutOne` возвращает 500 при сбое `providerMat.ApplyToClient` или `routeMat.ApplyToClient` после committed UPSERT'а в `subaccount_routing_assignment`. Frontend ретраит PUT целиком — UPSERT идемпотентен, материализация тоже (DELETE+INSERT по `source='template'`), но 500 поднимает alert и портит UX. Решение: при сбое одной из материализаций — вернуть 200 OK с body `{"client_id": "...", "warnings": [{"step": "provider_materialize", "error": "..."}]}` и пометить SRA timestamp `last_materialize_error_at` (новая колонка) — чтобы фоновый retry/cron мог досматривать. Bulk — аналогично, status="partial" + warnings array.

**Минимальная версия (без cron):** вернуть 200+warnings, без новой колонки. Cron retry — отложить на Plan 4. Для prod-safety нужно хотя бы logging на ERROR + metric counter, чтобы alert на partial-failure был.

**Files:**
- Modify: `internal/gateway/portal/handlers/network_assignments.go` (PutOne, Bulk)
- Modify: `internal/gateway/portal/handlers/network_assignments_test.go`
- Modify: `internal/gateway/portal/metrics.go` (добавить counter `MaterializeFailureTotal`)

- [ ] **Step 1: Прочитать существующий metrics.go**

```bash
cat internal/gateway/portal/metrics.go | head -100
```

Цель — увидеть pattern регистрации Prometheus counter'ов, чтобы добавить новый `MaterializeFailureTotal{operation="provider"|"route"}` в том же стиле.

- [ ] **Step 2: Написать failing test — PutOne при сбое provider materializer возвращает 200+warnings**

Добавить в `internal/gateway/portal/handlers/network_assignments_test.go`:

```go
// fakeFailingProviderMat — мок materializer, который всегда возвращает ошибку.
type fakeFailingProviderMat struct{}

func (fakeFailingProviderMat) ApplyToClient(ctx context.Context, clientID uuid.UUID, psID *uuid.UUID) error {
	return fmt.Errorf("simulated provider materialize failure")
}

// TestPutOne_ProviderMaterializeFailure_Returns200WithWarning — partial-failure
// pattern: SRA committed, materialize упала → 200 + warnings, не 500.
// Frontend ретраит PUT (идемпотентно), background-retry/cron может досматривать.
func TestPutOne_ProviderMaterializeFailure_Returns200WithWarning(t *testing.T) {
	pool, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	resellerID := testutil.SeedReseller(t, pool, "reseller@test.local")
	subID := testutil.SeedSubAccount(t, pool, resellerID, "sub@test.local")
	psID := testutil.SeedProviderSet(t, pool, resellerID, "ps1")

	// Подменяем providerMat на failing-mock, оставляем routeMat = nil (rsUUID не передаётся).
	// NB: NewNetworkAssignmentsHandlers принимает конкретный *network.ProviderSetMaterializer.
	// Если interface ещё не вытащен — рефакторинг handler'а на interface = часть Step 4.
	h := NewNetworkAssignmentsHandlers(pool, fakeFailingProviderMat{}, nil, nil)

	body := `{"provider_set_id":"` + psID.String() + `"}`
	r := httptest.NewRequest("PUT", "/portal/v1/reseller/network/assignments/"+subID.String(), strings.NewReader(body))
	r = mux.SetURLVars(r, map[string]string{"client_id": subID.String()})
	r = r.WithContext(middleware.WithClientID(r.Context(), resellerID))
	w := httptest.NewRecorder()
	h.PutOne(w, r)

	require.Equal(t, http.StatusOK, w.Code, "partial failure must be 200, got %d body=%s", w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "warnings")
	require.Contains(t, w.Body.String(), "provider_materialize")

	// Verify SRA committed despite materialize failure.
	var psRow uuid.UUID
	err := pool.QueryRow(context.Background(),
		`SELECT provider_set_id FROM subaccount_routing_assignment WHERE client_id = $1`,
		subID,
	).Scan(&psRow)
	require.NoError(t, err)
	require.Equal(t, psID, psRow, "SRA must be committed even when materialize fails")
}
```

**Verify before asserting:** прочитать актуальную сигнатуру `NewNetworkAssignmentsHandlers` (`internal/gateway/portal/handlers/network_assignments.go:38`). Сейчас принимает конкретный `*network.ProviderSetMaterializer` (не interface). Для теста нужен interface. Это значит **Step 4 включает рефакторинг сигнатуры** (вытащить interface `ProviderMaterializer`/`RouteMaterializer`).

- [ ] **Step 3: Запустить тест — должен FAIL (компилируется не корректно или возвращает 500)**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/gateway/portal/handlers/ -run TestPutOne_ProviderMaterializeFailure -v"
```

Expected: либо compile error (нет interface), либо PASS на 500. В обоих случаях — реализация не готова.

- [ ] **Step 4: Вытащить interface для materializer'ов**

Modify `internal/gateway/portal/handlers/network_assignments.go`:

```go
// ProviderMaterializer — interface для подмены в тестах.
// Production реализация: *network.ProviderSetMaterializer.
type ProviderMaterializer interface {
	ApplyToClient(ctx context.Context, clientID uuid.UUID, psID *uuid.UUID) error
}

// RouteMaterializer — interface для подмены в тестах.
type RouteMaterializer interface {
	ApplyToClient(ctx context.Context, clientID uuid.UUID, rsID *uuid.UUID) error
}

// NetworkAssignmentsHandlers ...
type NetworkAssignmentsHandlers struct {
	pool        *pgxpool.Pool
	providerMat ProviderMaterializer
	routeMat    RouteMaterializer
	validator   *network.ConflictValidator
}

func NewNetworkAssignmentsHandlers(
	pool *pgxpool.Pool,
	pm ProviderMaterializer,
	rm RouteMaterializer,
	v *network.ConflictValidator,
) *NetworkAssignmentsHandlers {
	return &NetworkAssignmentsHandlers{pool: pool, providerMat: pm, routeMat: rm, validator: v}
}
```

Conctete `*network.ProviderSetMaterializer` и `*network.RouteSetMaterializer` уже имеют метод `ApplyToClient(ctx, clientID, *uuid.UUID) error` (см. `internal/services/network/route_set_materializer.go:29` и аналогично для provider) — interface satisfied автоматически.

- [ ] **Step 5: Реализовать warnings-pattern в PutOne**

Заменить блок `if err := h.providerMat.ApplyToClient(...); err != nil { return 500 }` и аналогичный для route на:

```go
	warnings := []map[string]string{}

	if h.providerMat == nil {
		log.Error().Str("client_id", clientID.String()).Msg("PutOne: providerMat is nil")
		respondError(w, shared.ErrInternalServer("provider materializer not configured"))
		return
	}
	if err := h.providerMat.ApplyToClient(r.Context(), clientID, psUUID); err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("provider materialize partial-failure")
		MaterializeFailureTotal.WithLabelValues("provider").Inc()
		warnings = append(warnings, map[string]string{
			"step":  "provider_materialize",
			"error": err.Error(),
		})
	}

	if h.routeMat == nil {
		log.Error().Str("client_id", clientID.String()).Msg("PutOne: routeMat is nil")
		respondError(w, shared.ErrInternalServer("route materializer not configured"))
		return
	}
	if err := h.routeMat.ApplyToClient(r.Context(), clientID, rsUUID); err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("route materialize partial-failure")
		MaterializeFailureTotal.WithLabelValues("route").Inc()
		warnings = append(warnings, map[string]string{
			"step":  "route_materialize",
			"error": err.Error(),
		})
	}

	resp := map[string]interface{}{"client_id": clientID.String()}
	if len(warnings) > 0 {
		resp["warnings"] = warnings
	}
	respondJSON(w, http.StatusOK, resp)
```

- [ ] **Step 6: Аналогично в Bulk: status="partial" + warnings**

В цикле Bulk заменить два `Status: "error"` для materialize-сбоев на:

```go
		warnings := []map[string]string{}
		if err := h.providerMat.ApplyToClient(r.Context(), cid, psUUID); err != nil {
			log.Error().Err(err).Str("client_id", cid.String()).Msg("bulk provider materialize partial-failure")
			MaterializeFailureTotal.WithLabelValues("provider").Inc()
			warnings = append(warnings, map[string]string{"step": "provider_materialize", "error": err.Error()})
		}
		if err := h.routeMat.ApplyToClient(r.Context(), cid, rsUUID); err != nil {
			log.Error().Err(err).Str("client_id", cid.String()).Msg("bulk route materialize partial-failure")
			MaterializeFailureTotal.WithLabelValues("route").Inc()
			warnings = append(warnings, map[string]string{"step": "route_materialize", "error": err.Error()})
		}
		status := "ok"
		if len(warnings) > 0 {
			status = "partial"
		}
		results = append(results, bulkResultItem{ClientID: idStr, Status: status})
```

И расширить `bulkResultItem` чтобы поддерживать optional warnings:

```go
type bulkResultItem struct {
	ClientID string              `json:"client_id"`
	Status   string              `json:"status"`
	Error    string              `json:"error,omitempty"`
	Warnings []map[string]string `json:"warnings,omitempty"`
}
```

И прокинуть `warnings` в добавление результата:

```go
		results = append(results, bulkResultItem{ClientID: idStr, Status: status, Warnings: warnings})
```

- [ ] **Step 7: Добавить MaterializeFailureTotal в metrics.go**

Modify `internal/gateway/portal/metrics.go`:

```go
// MaterializeFailureTotal — счётчик partial-failure'ов в materializer'ах
// (provider_set / route_set ApplyToClient). Plan 3 Task 4: SRA committed,
// material failed → 200+warnings; этот counter alerter'у говорит про деградацию.
var MaterializeFailureTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "smpp_portal",
		Name:      "materialize_failure_total",
		Help:      "Materializer ApplyToClient partial-failures (SRA committed, materialize failed)",
	},
	[]string{"operation"}, // "provider" | "route"
)
```

(Адаптировать namespace/registry под фактический pattern в файле — прочитать перед написанием.)

- [ ] **Step 8: Запустить тест — должен PASS**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/gateway/portal/handlers/ -run TestPutOne -v"
```

Expected: PASS включая новый partial-failure тест.

- [ ] **Step 9: Local check + commit**

```bash
./scripts/check.sh
git add internal/gateway/portal/handlers/network_assignments.go internal/gateway/portal/handlers/network_assignments_test.go internal/gateway/portal/metrics.go
git status
git commit -m "feat(network): partial-failure pattern in assignments materializers (Plan 3 Task 4)

PutOne/Bulk: при сбое provider/route materializer'а после committed UPSERT'а
SRA — возвращаем 200 + warnings вместо 500. Frontend ретраит идемпотентно;
Prometheus counter MaterializeFailureTotal {operation} alerter'у говорит про
деградацию. Решает A3 из Plan 3 hardening list."
```

---

## Task 5: Materializer concurrency stress test

**Контекст:** Теперь когда partial-failure pattern (Task 4) на месте, нужен тест: два параллельных PUT'а на один SRA с разными подписчиками не должны ломать друг друга. Race может проявиться если два PUT'а пишут одновременно в `client_routes` для одного `client_id` — DELETE+INSERT-pattern в materializer'е не serializable.

**Files:**
- Create: `internal/services/network/concurrency_stress_test.go`

- [ ] **Step 1: Написать stress-test**

```go
package network_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/testutil"
)

// TestRouteSetMaterializer_ConcurrentApplyToClient — два параллельных вызова
// ApplyToClient для одного client_id должны завершиться без deadlock'а или
// порчи данных. Финальное состояние client_routes должно соответствовать
// одному из двух вариантов (last-writer-wins или интерливинг — оба валидны).
func TestRouteSetMaterializer_ConcurrentApplyToClient(t *testing.T) {
	pool, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	resellerID := testutil.SeedReseller(t, pool, "reseller@test.local")
	clientID := testutil.SeedSubAccount(t, pool, resellerID, "sub@test.local")
	provA := testutil.SeedProvider(t, pool, "provA", "platform")
	provB := testutil.SeedProvider(t, pool, "provB", "platform")
	rsA := testutil.SeedRouteSet(t, pool, resellerID, "rs-a", []uuid.UUID{provA})
	rsB := testutil.SeedRouteSet(t, pool, resellerID, "rs-b", []uuid.UUID{provB})

	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, itemsRepo)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = mat.ApplyToClient(context.Background(), clientID, &rsA)
	}()
	go func() {
		defer wg.Done()
		errs[1] = mat.ApplyToClient(context.Background(), clientID, &rsB)
	}()
	wg.Wait()

	require.NoError(t, errs[0], "concurrent ApplyToClient #1")
	require.NoError(t, errs[1], "concurrent ApplyToClient #2")

	// Финальное состояние — один из двух вариантов: только provA-route'ы или только provB-route'ы.
	// Главное: НЕ оба одновременно (это бы означало DELETE одного transaction'а не сработал).
	rows, err := pool.Query(context.Background(),
		`SELECT DISTINCT provider_id FROM client_routes WHERE client_id = $1 AND source = 'template'`,
		clientID,
	)
	require.NoError(t, err)
	defer rows.Close()
	var providerIDs []uuid.UUID
	for rows.Next() {
		var pid uuid.UUID
		require.NoError(t, rows.Scan(&pid))
		providerIDs = append(providerIDs, pid)
	}
	require.LessOrEqual(t, len(providerIDs), 1, "concurrent ApplyToClient must not interleave: got %d distinct providers", len(providerIDs))
}
```

**Verify before asserting:** проверить существование `testutil.SeedRouteSet`. Если нет — создать helper в testutil (отдельным мини-step'ом) или адаптировать тест под прямые SQL-вставки.

- [ ] **Step 2: Запустить тест**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/services/network/ -run TestRouteSetMaterializer_Concurrent -v -race"
```

Expected: PASS. Если FAIL — race-condition реальная, нужен fix (advisory_lock per-client_id в начале tx). Если фикс нужен — добавь его как Step 3, иначе — Step 3 = commit.

- [ ] **Step 3: Если test FAIL — добавить advisory lock в ApplyToClient**

Modify `internal/services/network/route_set_materializer.go` `ApplyToClient`, после `tx.Begin`:

```go
	// Per-client_id advisory lock сериализует concurrent ApplyToClient,
	// чтобы DELETE+INSERT-pattern не мог интерливиться.
	// pg_try_advisory_xact_lock автоматически освобождается на COMMIT/ROLLBACK.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, clientID.String()); err != nil {
		return err
	}
```

Аналогично в `provider_set_materializer.go`. Перезапустить test → PASS.

- [ ] **Step 4: Local check + commit**

```bash
./scripts/check.sh
git add internal/services/network/concurrency_stress_test.go
# Если advisory_lock добавлен:
git add internal/services/network/route_set_materializer.go internal/services/network/provider_set_materializer.go
git status
git commit -m "test(network): materializer concurrency stress test + advisory_lock if needed (Plan 3 Task 5)"
```

---

## Task 6: Trigger orphan-cleanup в subaccount_routing_assignment

**Контекст:** ON DELETE SET NULL на FK `provider_set_id` / `route_set_id` оставляет SRA-row с обоими NULL после удаления обоих set'ов. List/Overview показывают «пустую» строку. Решение: trigger AFTER UPDATE, который DELETE'ит row если оба поля стали NULL.

**Files:**
- Create: `migrations/000139_subaccount_routing_assignment_orphan_cleanup.up.sql`
- Create: `migrations/000139_subaccount_routing_assignment_orphan_cleanup.down.sql`
- Create: `internal/services/network/orphan_cleanup_test.go`

- [ ] **Step 1: Написать failing test**

```go
package network_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/testutil"
)

// TestSRAOrphanCleanup_BothNull_DeletesRow — после установки обоих set_id в NULL
// trigger должен удалить SRA-row автоматически (миграция 000139).
func TestSRAOrphanCleanup_BothNull_DeletesRow(t *testing.T) {
	pool, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	resellerID := testutil.SeedReseller(t, pool, "reseller@test.local")
	clientID := testutil.SeedSubAccount(t, pool, resellerID, "sub@test.local")
	psID := testutil.SeedProviderSet(t, pool, resellerID, "ps")

	_, err := pool.Exec(context.Background(),
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
		 VALUES ($1, $2, NULL, now())`, clientID, psID)
	require.NoError(t, err)

	// Сбросить provider_set_id в NULL — оба теперь NULL → trigger DELETE'ит row.
	_, err = pool.Exec(context.Background(),
		`UPDATE subaccount_routing_assignment SET provider_set_id = NULL WHERE client_id = $1`, clientID)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM subaccount_routing_assignment WHERE client_id = $1`, clientID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "orphan SRA row must be deleted by trigger")
}

func TestSRAOrphanCleanup_OneNull_KeepsRow(t *testing.T) {
	pool, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	resellerID := testutil.SeedReseller(t, pool, "reseller@test.local")
	clientID := testutil.SeedSubAccount(t, pool, resellerID, "sub@test.local")
	psID := testutil.SeedProviderSet(t, pool, resellerID, "ps")
	rsID := testutil.SeedRouteSet(t, pool, resellerID, "rs", nil)

	_, err := pool.Exec(context.Background(),
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
		 VALUES ($1, $2, $3, now())`, clientID, psID, rsID)
	require.NoError(t, err)

	_, _ = uuid.MustParse("00000000-0000-0000-0000-000000000000"), error(nil)

	_, err = pool.Exec(context.Background(),
		`UPDATE subaccount_routing_assignment SET route_set_id = NULL WHERE client_id = $1`, clientID)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM subaccount_routing_assignment WHERE client_id = $1`, clientID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "row with one non-NULL set must survive")
}
```

- [ ] **Step 2: Запустить — должен FAIL**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/services/network/ -run TestSRAOrphanCleanup -v"
```

Expected: первый FAIL (count=1 вместо 0), второй PASS (по совпадению).

- [ ] **Step 3: Написать миграцию**

Файл `migrations/000139_subaccount_routing_assignment_orphan_cleanup.up.sql`:

```sql
-- Trigger AFTER UPDATE: автоматически DELETE'ит SRA-row, если оба set_id стали NULL.
-- Контекст: ON DELETE SET NULL на FK provider_set_id/route_set_id оставляет
-- (NULL, NULL) row, который засоряет List/Overview. Plan 3 Task 6 (B4).
CREATE OR REPLACE FUNCTION subaccount_routing_assignment_orphan_cleanup() RETURNS trigger AS $$
BEGIN
    IF NEW.provider_set_id IS NULL AND NEW.route_set_id IS NULL THEN
        DELETE FROM subaccount_routing_assignment WHERE client_id = NEW.client_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sra_orphan_cleanup ON subaccount_routing_assignment;
CREATE TRIGGER trg_sra_orphan_cleanup
AFTER UPDATE ON subaccount_routing_assignment
FOR EACH ROW EXECUTE FUNCTION subaccount_routing_assignment_orphan_cleanup();
```

DOWN:

```sql
DROP TRIGGER IF EXISTS trg_sra_orphan_cleanup ON subaccount_routing_assignment;
DROP FUNCTION IF EXISTS subaccount_routing_assignment_orphan_cleanup();
```

- [ ] **Step 4: Применить миграцию + перезапустить тест**

```bash
git add migrations/000139_subaccount_routing_assignment_orphan_cleanup.up.sql migrations/000139_subaccount_routing_assignment_orphan_cleanup.down.sql
git commit -m "migrate: orphan-cleanup trigger on subaccount_routing_assignment (Plan 3 Task 6)"
git push origin master
./scripts/server.sh sync
./scripts/server.sh migrate

./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/services/network/ -run TestSRAOrphanCleanup -v"
```

Expected: оба PASS.

- [ ] **Step 5: Commit test**

```bash
git add internal/services/network/orphan_cleanup_test.go
git status
git commit -m "test(network): SRA orphan-cleanup trigger coverage (Plan 3 Task 6)"
```

---

## Task 7: Извлечь verifySubAccountOwnership в shared helper

**Контекст:** Дубликат метода в `network_assignments.go:154` и `subaccount_network_overrides.go:40`. Identical query, identical error mapping. Решение — вынести в `internal/gateway/portal/middleware/sub_account_ownership.go` как функцию (не middleware — это helper, вызывается из handler'ов).

**Files:**
- Create: `internal/gateway/portal/middleware/sub_account_ownership.go`
- Modify: `internal/gateway/portal/handlers/network_assignments.go` (удалить duplicate, использовать middleware.VerifySubAccountOwnership)
- Modify: `internal/gateway/portal/handlers/subaccount_network_overrides.go` (то же)

- [ ] **Step 1: Создать helper**

```go
package middleware

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// VerifySubAccountOwnership возвращает 404, если subID не суб-аккаунт текущего
// reseller'а. 404 (а не 403) — чтобы не светить наличие чужих client_id.
// Connection / scan errors отделяются и возвращают 500, чтобы не маскировать
// инфраструктурные сбои под "не найдено".
//
// Ранее — duplicate в network_assignments.go и subaccount_network_overrides.go.
// Plan 3 Task 7 (B5): extracted здесь, чтобы будущие endpoint'ы под
// /sub-accounts/* использовали единый источник правды.
func VerifySubAccountOwnership(ctx context.Context, pool *pgxpool.Pool, resellerID, subID uuid.UUID) *shared.AppError {
	var parent uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT parent_client_id FROM clients WHERE id = $1 AND parent_client_id IS NOT NULL`,
		subID,
	).Scan(&parent)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return shared.ErrNotFound("суб-аккаунт")
		}
		log.Error().Err(err).Str("sub_id", subID.String()).Msg("VerifySubAccountOwnership query")
		return shared.ErrInternalServer("verify sub-account ownership")
	}
	if parent != resellerID {
		return shared.ErrNotFound("суб-аккаунт")
	}
	return nil
}
```

- [ ] **Step 2: Заменить вызовы в network_assignments.go**

Удалить метод `verifySubAccountOwnership` (строки 150-171). Заменить все `h.verifySubAccountOwnership(r.Context(), resellerID, ...)` на `middleware.VerifySubAccountOwnership(r.Context(), h.pool, resellerID, ...)`.

- [ ] **Step 3: Заменить в subaccount_network_overrides.go**

Удалить метод `verifyOwnership` (строки 40-57). Заменить вызовы `h.verifyOwnership(...)` на `middleware.VerifySubAccountOwnership(r.Context(), h.pool, resellerID, ...)`. Импорт `middleware` уже есть.

- [ ] **Step 4: Запустить все тесты handler'ов — должны PASS**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/gateway/portal/handlers/ -run 'TestPutOne|TestAddProviderOverride|TestAddRouteOverride|TestOverview' -v"
```

Expected: PASS.

- [ ] **Step 5: Local check + commit**

```bash
./scripts/check.sh
git add internal/gateway/portal/middleware/sub_account_ownership.go internal/gateway/portal/handlers/network_assignments.go internal/gateway/portal/handlers/subaccount_network_overrides.go
git status
git commit -m "refactor(portal): extract VerifySubAccountOwnership to middleware helper (Plan 3 Task 7)"
```

---

## Task 8: Tooltip в Preview UI про operator-conditions

**Контекст:** `network_route_preview.go` для condition_type='operator' всегда возвращает true (нет JOIN'а к operators). Без UI-индикации клиент думает, что preview точно показывает матчинг по operator. Решение: tooltip-предупреждение в preview-блоке RouteSetsPage.

**Files:**
- Modify: `portal-frontend/src/pages/aggregator/RouteSetsPage.tsx` (или там где preview рендерится)

- [ ] **Step 1: Найти preview-блок**

```bash
grep -rn "preview\|RoutePreview\|operator" portal-frontend/src/pages/aggregator/RouteSetsPage.tsx
```

Найти место рендера preview-результатов.

- [ ] **Step 2: Добавить tooltip (Radix Tooltip уже в стеке)**

Добавить рядом с заголовком preview-секции (адаптировать под фактический JSX):

```tsx
import { InfoIcon } from "lucide-react";
import * as Tooltip from "@radix-ui/react-tooltip";

// Внутри preview-блока, рядом с заголовком "Preview":
<Tooltip.Provider>
  <Tooltip.Root>
    <Tooltip.Trigger asChild>
      <button type="button" className="inline-flex items-center text-amber-600" aria-label="Ограничения preview">
        <InfoIcon className="w-4 h-4" />
      </button>
    </Tooltip.Trigger>
    <Tooltip.Portal>
      <Tooltip.Content className="bg-gray-900 text-white px-3 py-2 rounded text-xs max-w-xs">
        Preview не проверяет соответствие условий по оператору и country-prefix
        в полном объёме. В реальном пайплайне маршрутизация может отличаться.
        Используйте preview как ориентир, не как enforcement.
        <Tooltip.Arrow className="fill-gray-900" />
      </Tooltip.Content>
    </Tooltip.Portal>
  </Tooltip.Root>
</Tooltip.Provider>
```

**Verify before asserting:** проверить, что Radix Tooltip уже импортируется где-то в проекте (`grep -rn "@radix-ui/react-tooltip" portal-frontend/src/`). Если нет — установить или использовать существующий tooltip-компонент (`grep "Tooltip" portal-frontend/src/components/`).

- [ ] **Step 3: TS + ESLint check**

```bash
cd portal-frontend && npx tsc --noEmit && npx eslint . --max-warnings=51
```

Expected: PASS.

- [ ] **Step 4: Smoke в браузере на sandbox**

После deploy: открыть `/network/route-sets`, выбрать любой route-set, проверить preview-блок → tooltip отображается при hover на info-иконке, текст совпадает.

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/aggregator/RouteSetsPage.tsx
git status
git commit -m "feat(portal): preview tooltip about operator/country approximations (Plan 3 Task 8)"
```

---

## Task 9: Lockdown ESLint baseline 56 → 51

**Контекст:** После Plans 1+2 фактический warning count = 51. Baseline в `package.json`/`scripts/check.sh`/`ci.yml`/`CLAUDE.md` остался 56 (по плану 2 был 69 → 56, в реальности — 51). Plan 3 фиксирует ratchet.

**Files:**
- Modify: `portal-frontend/package.json` (line 11)
- Modify: `scripts/check.sh` (lines 63-64)
- Modify: `.github/workflows/ci.yml` (line 56)
- Modify: `CLAUDE.md` (раздел Quality Gates, строка про baseline)

- [ ] **Step 1: Verify current count**

```bash
cd portal-frontend && npx --no-install eslint . 2>&1 | tail -3
```

Expected: `51 problems (0 errors, 51 warnings)`.

- [ ] **Step 2: Update package.json**

Заменить `"lint": "eslint . --max-warnings=56"` на `"lint": "eslint . --max-warnings=51"`.

- [ ] **Step 3: Update scripts/check.sh**

Заменить:
```
    # max-warnings=56 is the frozen baseline from 2026-04-22 — ratchet down as warnings are fixed.
    step "eslint"       npx --no-install eslint . --max-warnings=56
```
на:
```
    # max-warnings=51 is the frozen baseline from 2026-05-05 (Plan 3 Task 9) — ratchet down as warnings are fixed.
    step "eslint"       npx --no-install eslint . --max-warnings=51
```

- [ ] **Step 4: Update .github/workflows/ci.yml**

Заменить `--max-warnings=56` на `--max-warnings=51`.

- [ ] **Step 5: Update CLAUDE.md**

Заменить:
```
Baseline 2026-04-18: 69 warnings (в основном `no-explicit-any`, `exhaustive-deps`).

Гейт в `package.json` lint-команде и в `scripts/check.sh` настроен на `--max-warnings=69`.
```
на:
```
Baseline 2026-05-05: 51 warnings (после Plan 3 hardening).

Гейт в `package.json` lint-команде, `scripts/check.sh` и `.github/workflows/ci.yml` настроен на `--max-warnings=51`.
```

- [ ] **Step 6: Local check**

```bash
./scripts/check.sh
```

Expected: PASS (eslint точно =51).

- [ ] **Step 7: Commit**

```bash
git add portal-frontend/package.json scripts/check.sh .github/workflows/ci.yml CLAUDE.md
git status
git commit -m "chore: ratchet ESLint baseline 56 → 51 (Plan 3 Task 9)"
```

---

## Task 10: Migration gap audit (DB schema diff vs migrations)

**Контекст:** `migrations/000137` down-комментарий упоминает «sandbox-only constraint, не создававшийся миграциями». Это значит в sandbox DB могут быть out-of-tree схема-объекты (constraints, indexes, columns), которых нет в `migrations/`. Если завтра поднимать чистый dev — расхождения. Goal: найти gap, написать gap-миграции.

**Files:**
- Create: `scripts/audit_db_schema_gap.sh`
- Create: `migrations/0001<N>_*` (по факту найденного gap'а; N = 140+)

- [ ] **Step 1: Написать скрипт аудита**

Файл `scripts/audit_db_schema_gap.sh`:

```bash
#!/usr/bin/env bash
# Сравнивает schema sandbox-DB с тем, что должно получиться при чистом
# применении migrations/. Используется для детекции out-of-tree объектов.
#
# Стратегия: pg_dump --schema-only sandbox-DB → А. Поднять temp-DB, применить
# все миграции из migrations/ → pg_dump --schema-only → B. diff А B.
# Расхождения = gap.

set -euo pipefail

if ! command -v pg_dump >/dev/null; then
    echo "pg_dump required" >&2; exit 1
fi

SANDBOX_URL="${SANDBOX_DB_URL:-postgres://smpp:smpp_password@72.56.232.202:5432/smpp_db}"
TEMP_DB="smpp_audit_$(date +%s)"
LOCAL_URL="postgres://smpp:smpp_password@localhost:5432/$TEMP_DB"

echo "Dumping sandbox schema..."
pg_dump --schema-only --no-owner --no-acl "$SANDBOX_URL" \
    | grep -v -E '^(--|SET|SELECT pg_catalog)' > /tmp/sandbox_schema.sql

echo "Creating fresh temp DB and applying migrations..."
psql "postgres://smpp:smpp_password@localhost:5432/postgres" -c "CREATE DATABASE $TEMP_DB"
migrate -path migrations/ -database "$LOCAL_URL?sslmode=disable" up
pg_dump --schema-only --no-owner --no-acl "$LOCAL_URL" \
    | grep -v -E '^(--|SET|SELECT pg_catalog)' > /tmp/migrated_schema.sql
psql "postgres://smpp:smpp_password@localhost:5432/postgres" -c "DROP DATABASE $TEMP_DB"

echo "Diff (sandbox - migrations):"
diff /tmp/sandbox_schema.sql /tmp/migrated_schema.sql || true
```

- [ ] **Step 2: Запустить аудит**

Запуск только локально (на dev-машине с pg_dump + локальным postgres). Если на Windows проблемы — запустить эквивалент через WSL/git-bash или адаптировать под сравнение через server-side `pg_dump | ssh`.

```bash
chmod +x scripts/audit_db_schema_gap.sh
./scripts/audit_db_schema_gap.sh > /tmp/db_gap.diff
cat /tmp/db_gap.diff
```

Expected output: список расхождений. Каждое расхождение классифицировать:
- **+sandbox-only** (отсутствует в migrations) → нужна gap-миграция (либо `CREATE`, либо `DROP IF EXISTS` если объект больше не нужен)
- **-migration-only** (отсутствует в sandbox) → миграция не накатилась в sandbox; запустить `./scripts/server.sh migrate`

- [ ] **Step 3: Написать gap-миграции для каждого расхождения**

Для каждой строки расхождения создать миграцию `migrations/0001<N>_<slug>.up.sql/.down.sql` с одной строкой `CREATE ... IF NOT EXISTS` или `DROP ... IF EXISTS`. Numbering — последовательное, начиная с 000140 (или с 000139+1 если Task 6 уже занял 000139).

Пример типового gap: out-of-tree index `ix_routes_owner_routetype` — если он есть в sandbox но не в migrations, миграция:

```sql
-- 000140_add_ix_routes_owner_routetype.up.sql
CREATE INDEX IF NOT EXISTS ix_routes_owner_routetype ON client_routes (owner_type, owner_id, route_type);
```

```sql
-- 000140_add_ix_routes_owner_routetype.down.sql
DROP INDEX IF EXISTS ix_routes_owner_routetype;
```

- [ ] **Step 4: Применить миграции на чистом temp-DB и подтвердить, что diff пустой**

```bash
./scripts/audit_db_schema_gap.sh > /tmp/db_gap_after.diff
wc -l /tmp/db_gap_after.diff
```

Expected: 0 строк (или только pg_dump-noise, не реальные расхождения).

- [ ] **Step 5: Применить миграции на sandbox**

```bash
git add scripts/audit_db_schema_gap.sh migrations/0001*
git commit -m "migrate: gap-migrations from DB-vs-tree audit (Plan 3 Task 10)"
git push origin master
./scripts/server.sh sync
./scripts/server.sh migrate
```

Большинство миграций будут no-op на sandbox (`IF NOT EXISTS` / `IF EXISTS`).

- [ ] **Step 6: Verify clean rebuild simulating prod**

Опционально, но желательно: на dev-машине `dropdb smpp_test && createdb smpp_test && migrate up` — должно пройти без ошибок и без обращения к out-of-tree objects.

---

## Task 11: Unskip Bulk-conflict E2E test

**Контекст:** В Plan 2 был test, помеченный `test.skip`, потому что не было fixture с conflict-preset. Сейчас — добавить fixture через API в setup-хуке теста.

**Files:**
- Modify: `e2e/aggregator-network.spec.ts` (или другой файл где лежит skipped test)

- [ ] **Step 1: Найти skipped test**

```bash
grep -rn "test.skip\|test\.skip" e2e/
```

- [ ] **Step 2: Прочитать его текущее состояние**

```bash
# Read e2e file around the skip
```

- [ ] **Step 3: Добавить setup-хук, создающий conflict через API**

Заменить `test.skip(...)` на полную реализацию: создать provider-set без provider X, route-set с правилом, использующим provider X, попытка PUT assignment → 409 conflict modal должен открыться. Псевдо-структура (адаптировать под актуальный playwright pattern в spec'е):

```typescript
test('bulk-assign показывает conflict-modal при route-set вне provider-set', async ({ page, request }) => {
  // Setup: создать через API два набора с заведомо несовместимыми provider-set'ами.
  const psID = await createProviderSet(request, { name: 'ps-no-X', providers: ['providerA'] });
  const rsID = await createRouteSet(request, { name: 'rs-uses-X', items: [{ provider_id: 'providerX', conditions: [...] }] });
  const subID = await getFirstSubAccountID(request);

  await page.goto('/network/assignments');
  await page.getByRole('row', { name: subID }).getByRole('button', { name: /назначить/i }).click();
  await page.getByLabel(/provider-set/i).selectOption(psID);
  await page.getByLabel(/route-set/i).selectOption(rsID);
  await page.getByRole('button', { name: /применить/i }).click();

  // Conflict modal должен открыться с описанием.
  await expect(page.getByRole('dialog', { name: /конфликт/i })).toBeVisible();
  await expect(page.getByText(/маршрут использует.*провайдер.*вне provider-set/i)).toBeVisible();
});
```

**Verify before asserting:** прочитать actual e2e helper'ы (`createProviderSet`, `createRouteSet`, etc) — если их нет, либо использовать прямые `request.post(...)`, либо пропустить тест с TODO. **Не** угадывать сигнатуры.

- [ ] **Step 4: Запустить e2e против sandbox**

```bash
cd e2e && npm test -- --grep "conflict-modal"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add e2e/aggregator-network.spec.ts
git status
git commit -m "test(e2e): unskip bulk-assign conflict-modal coverage (Plan 3 Task 11)"
```

---

## Task 12: Final smoke + DONE marker

**Files:**
- Create: `docs/superpowers/plans/2026-05-04-aggregator-routing-plan-3-DONE.md`

- [ ] **Step 1: Full check.sh**

```bash
./scripts/check.sh
```

Expected: PASS.

- [ ] **Step 2: Полный test-suite на sandbox**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/services/network/... ./internal/gateway/portal/handlers/... ./internal/gateway/portal/middleware/..."
```

Expected: PASS.

- [ ] **Step 3: Manual smoke против sandbox UI**

- Открыть `/network/route-sets` → создать route-set → preview работает + tooltip виден.
- Открыть `/network/assignments` → bulk-assign на 2+ суб-аккаунта → результат включает status и (если materialize упал) warnings array.
- Открыть `/sub-accounts/{id}/network` → попробовать дважды добавить override с одинаковыми условиями → второй раз 409 duplicate_route_signature в UI.
- Залогиниться обычным клиентом (не reseller) → попытка GET `/portal/v1/sub-accounts` → 403.

- [ ] **Step 4: Написать DONE-файл**

Файл `docs/superpowers/plans/2026-05-04-aggregator-routing-plan-3-DONE.md`:

```markdown
# Aggregator Routing — Plan 3: Hardening & Polish — DONE

**Дата завершения:** 2026-05-XX
**Спецификация:** docs/superpowers/specs/2026-05-04-aggregator-routing-management-design.md
**План:** docs/superpowers/plans/2026-05-04-aggregator-routing-plan-3-hardening.md
**Предыдущие планы:** plan-1-DONE, plan-2-DONE

## Сделано

- Task 1: Drop sandbox-only `uq_cell_provider` (миграция 000138)
- Task 2: Pre-check duplicate signature в AddRouteOverride/UpdateRouteOverride (RouteSignature helper + 409 duplicate_route_signature)
- Task 3: ResellerOnlyMiddleware на /sub-accounts/* subrouter
- Task 4: Materializer partial-failure → 200+warnings + Prometheus counter MaterializeFailureTotal
- Task 5: Concurrency stress test для RouteSetMaterializer (+ advisory_lock если потребовался)
- Task 6: Trigger orphan-cleanup в subaccount_routing_assignment (миграция 000139)
- Task 7: Extracted middleware.VerifySubAccountOwnership
- Task 8: Tooltip в Preview UI про operator/country approximations
- Task 9: ESLint baseline 56 → 51 (lockdown)
- Task 10: Migration gap audit — N gap-миграций добавлены
- Task 11: Unskipped bulk-conflict E2E test
- Task 12: Final smoke + DONE marker

## Следующие шаги (Plan 4 кандидаты)

- C7: Audit-log UI (отложено per spec §6.7, после первой prod-итерации)
- D13: Redis hardening (requirepass + firewall) — отдельный security-план
- A3 расширение: cron retry для committed-but-unmaterialized SRA-rows
- B6: group_index семантика для non-contiguous source

## Memory updates

После DONE:
- Обновить project memory про aggregator routing — отметить, что pre-check signature заменил БД-инвариант
- Обновить feedback memory про ESLint baseline = 51
```

- [ ] **Step 5: Commit DONE marker**

```bash
git add docs/superpowers/plans/2026-05-04-aggregator-routing-plan-3-DONE.md
git status
git commit -m "docs(plan): Plan 3 DONE — aggregator routing hardening complete"
git push origin master
```

---

## Self-Review (выполнить после написания плана, перед execution handoff)

**Spec coverage:**
- A1 (uq_cell_provider override conflict) → Task 1 + Task 2 ✓
- A2 (ResellerOnlyMiddleware) → Task 3 ✓
- A3 (Materialiser partial-failure) → Task 4 ✓
- B4 (Orphan-assignment cleanup) → Task 6 ✓
- B5 (verifySubAccountOwnership дубликат) → Task 7 ✓
- C8 (Operator-condition tooltip) → Task 8 ✓
- D14 (ESLint lockdown 51) → Task 9 ✓
- D15 (Migration gap audit) → Task 10 ✓
- E16 (Concurrency stress) → Task 5 ✓
- E17 (Bulk-conflict E2E unskip) → Task 11 ✓

Все согласованные пункты A-E покрыты.

**Placeholder scan:** в плане нет `TBD`, `implement later`, `add appropriate error handling`. Каждая задача с конкретными файлами, кодом, командами.

**Type consistency:**
- `RouteSignature(item storage.RouteSetItemFull) string` — используется в Task 2 helper'ах одинаково
- `ProviderMaterializer` / `RouteMaterializer` interface — добавлен в Task 4, тест в Step 2 использует `fakeFailingProviderMat` который реализует тот же interface
- `MaterializeFailureTotal.WithLabelValues("provider"|"route")` — labels стабильны между Step 5 и Step 6 в Task 4
- `findDuplicateOverrideSignature` (Task 2 Step 7) и `findDuplicateOverrideSignatureExcept` (Step 8) — два варианта по дизайну, оба возвращают `(uuid.UUID, error)` с `uuid.Nil` для «нет дубликата»

**Migration numbering:** 000138 (Task 1), 000139 (Task 6), 000140+ (Task 10 gap-миграции). Numbering linear, без collision.

Issues caught: нет.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-04-aggregator-routing-plan-3-hardening.md`.

Two execution options:

**1. Subagent-Driven (recommended)** — fresh subagent per task, review между tasks, fast iteration. Pattern идентичный Plans 1+2.

**2. Inline Execution** — выполнение в этой сессии через executing-plans, batch с checkpoints.

Which approach?
