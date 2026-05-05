# Plan 2 — Aggregator Routing Management — DONE

**Plan:** `docs/superpowers/plans/2026-05-04-aggregator-routing-plan-2-routing.md`
**Завершён:** 2026-05-05
**Статус:** реализован, развёрнут на sandbox-сервере, smoke-flow зелёный.

---

## Executive summary

Plan 2 закрывает routing-сторону aggregator/reseller-самообслуживания: route-sets как версионируемые шаблоны маршрутов, материализация в `client_routes` с `source='template'`, override-routes на уровне суб-аккаунта (`source='override'`), bulk-назначения с conflict-валидацией и dry-run, cleanup orphan-routes, расширенный network preview-симулятор и UI. Старый монолитный экран `/network/routing` снесён.

Состояние Plan 2:
- Миграции **133-137** применены на sandbox (`schema_migrations.version = 137`).
- 33 commit от `cf2d9f9` до `HEAD` (`d544532`).
- Все 23 задачи плана выполнены, включая UI (Tasks 16-21) и e2e-тесты (Task 22).
- Backend HTTP/gRPC и portal-frontend перекомпилированы и развёрнуты, контейнеры здоровы.
- ESLint baseline после Task 21 снизился до **51 warning** (с 56). Гейты `package.json` и `scripts/check.sh` всё ещё на 56 — отдельный chore-PR ratchet.

---

## Plan 2 commits (cf2d9f9..HEAD)

| SHA | Task | Описание |
|---|---|---|
| `cf2d9f9` | — | docs(plan): aggregator routing Plan 2 — routing |
| `b29e249` | 1 | feat(migrations): reseller_route_sets + items |
| `a3af2a2` | 2 | feat(migrations): route_set condition groups + conditions + schedules |
| `7043bf4` | 3 | feat(migrations): client_routes.source + sra.route_set_id FK |
| `c5dbb0a` | 4 | feat(storage): reseller_route_sets repository |
| `11c566c` | 4-fix | test(storage): route-set repo — proper error checks + not-found cases |
| `40f3ee0` | 5 | feat(storage): reseller_route_set_items repository |
| `bdeed39` | 6 | feat(storagetest): SeedRouteSet + SeedRouteSetItem |
| `a9c82fb` | 7 | feat(network): RouteSetMaterializer — материализация в client_routes |
| `4f49c00` | 7-fix | fix(network): RouteSetMaterializer — owner_type/owner_id |
| `ba8d528` | 7-fix | fix(network): RouteSetMaterializer — owner_type migration + schedule test |
| `d95620e` | 8 | feat(network): ConflictValidator — provider-set ↔ route-set invariant |
| `3835e7f` | 8-fix | fix(network): ConflictValidator — single-fetch + nil-providerSet test |
| `9622c05` | 9 | feat(handlers): /reseller/network/route-sets CRUD |
| `1d3badd` | 10 | feat(handlers): /reseller/network/route-sets/{id}/items CRUD + duplicate + reorder |
| `a0f1e6f` | 10-fix | fix(route-set-items): scope Delete/UpdateFull by set_id + materialization test |
| `1704299` | 11 | feat(handlers): /reseller/network/route-sets/{id}/preview — matching simulator |
| `edc63b3` | 11-fix | fix(handlers): preview — log rows.Err() on partial provider names read |
| `35760b1` | 12 | feat(handlers): assignments — route_set_id support + conflict validation + dry-run |
| `8990412` | 12-fix | fix(assignments): route bulk/dry-run + validator error surfacing + nil-PS conflict |
| `201aa4b` | 13 | feat(handlers): /reseller/network/route-cleanup — cascade-cleanup orphan routes |
| `37d00a9` | 13-fix | fix(handlers): cleanup — propagate Scan err + cross-reseller test |
| `8b4d6e9` | 14 | feat(handlers): subaccount route-overrides + Overview расширение |
| `b420321` | 14-fix | fix(handlers): subaccount route-override — scoped UPDATE + intra-tx provider check + tests |
| `95c6b98` | 14b | feat(handlers): provider DELETE + provider-set PUT items — route-conflict checks |
| `0245f3e` | 14b-fix | fix(handlers): provider-set-items — check rows.Err() in route-conflict scan |
| `8efe160` | 15 | feat(routing): wire route-sets/preview/cleanup + remove legacy /reseller/routing/* |
| `324bf65` | 16 | feat(api): networkApi — route-sets, items, preview, overrides, cleanup |
| `4e99ce4` | 17 | feat(portal): RouteConditionsEditor + RouteSchedulesEditor + RouteRuleDrawer components |
| `f187b1a` | 18 | feat(portal): /network/route-sets — master-detail editor + drag-reorder + preview |
| `9282eab` | 19 | feat(portal): AssignmentsPage — route-set column + dry-run + conflict modal |
| `8e4ede4` | 20 | feat(portal): SubAccountNetworkSection — route-set block + override routes |
| `b093ad2` | 21 | chore(portal): remove dead NetworkRoutingPage + NetworkLayout |
| `d544532` | 22 | test(e2e): aggregator network Plan 2 — route-sets + preview + override |

Total: **34 commits** (включая plan doc).

---

## Smoke flow (Task 23.2) — sandbox `localhost:18085`

Выполнено через `./scripts/server.sh exec "curl ..."` от имени `aggregator@test.local`.
Test sub-account: `f6c4b3fd-f0d3-4d73-a59a-b6c43a312f8d` (Test Clean A).

| Шаг | Endpoint / действие | Ожидание | Результат |
|---|---|---|---|
| 1 | `POST /auth/login` | 200 + `csrf_token` cookie | **PASS** — 200, session+csrf cookies выставлены |
| 2 | `POST /reseller/network/providers` (private) | 201 | **PASS** — id=`e08529dd…` |
| 3 | `POST /reseller/network/provider-sets` + `PUT items` | 201, items count=1 | **PASS** — id=`7e5faa0e…`, count=1 |
| 4 | `POST /reseller/network/route-sets` | 201 | **PASS** — id=`990cea42…` |
| 5 | `POST /reseller/network/route-sets/{id}/items` (RU country condition) | 201 | **PASS** — id=`da6ce428…` |
| 6 | `PUT /reseller/network/assignments/{client_id}` (PS+RS) | 200 | **PASS** |
| 7 | `psql client_routes WHERE source='template'` (≥1) + condition_groups (≥1) | оба ≥1 | **PASS** — 1 + 1 |
| 8 | `POST /sub-accounts/{id}/network/route-overrides` (PROV2) | 201 | **PASS** — id=`606fd5da…`, source='override' |
| 9 | `DELETE /reseller/network/route-sets/{id}` (assigned) | 409 + `route_set_assigned` | **PASS** — `{"kind":"route_set_assigned","count":1}` |
| 10 | `DELETE /reseller/network/providers/{id}` (used) | 409 | **PASS** — `{"kind":"used_in_provider_sets"}` (срабатывает раньше `provider_used_in_routes`) |
| 11 | `POST /reseller/network/assignments/bulk/dry-run` (PS-без-провайдера + RS) | `status: "conflict"` | **PASS** — `"маршрут использует 1 провайдер(ов) вне provider-set"` |
| 12 | `POST /reseller/network/route-cleanup` (provider) | `removed_route_set_items > 0` | **PASS** — `{"removed_overrides":0,"removed_route_set_items":1}` |
| 13 | Cleanup test data (DELETE override, unassign, DELETE RS/PS/providers) | все 200/204 | **PASS** — все коды 204/200 |

### Step 23.3 — legacy URL

| Endpoint | Ожидание | Результат |
|---|---|---|
| `GET /reseller/routing/providers` | 404 | **PASS** — `404 page not found` |

### Step 23.1 — deploy/migrate/status

- `git push` — `Everything up-to-date` (Plan 2 уже был в master).
- `./scripts/server.sh migrate` — `no change` (миграции 133-137 уже применены на sandbox в ходе разработки; что подтверждает заметку Task 7 о ранне-применённых owner_type-колонках).
- `schema_migrations.version=137`, `dirty=false`.
- `./scripts/server.sh deploy` — все контейнеры пересобраны, `docker ps` показывает healthy/Up для `auth-service`, `routing-service`, `portal-gateway`, `client-gateway-1/2`, `admin-gateway-1/2`, всех pipeline-* и т.д.

### Заметки

- CSRF: cookie `csrf_token` выставляется на `/auth/login`; запросы отправлялись с заголовком `X-CSRF-Token`.
- При попытке override-route на тот же провайдер, что уже материализован шаблоном, INSERT падает на `uq_cell_provider` (operator+region+provider+...). В smoke использовался второй провайдер. Это не баг, а следствие текущего design — override должен отличаться по cell-key. См. follow-up «country/operator/number_range на override».
- `parseItemIn` (`network_route_set_items.go:159`) не пробрасывает `country_code` / `operator_id` / `number_range` / `traffic_type` из условий в `client_routes` — материализуется default cell. Для preview-симулятора этого достаточно (matching на условиях), но override-cell разрешения зависят только от `provider_id`. Это не было в scope Plan 2 — но стоит вынести в Plan 3 (см. follow-up).

---

## Open follow-ups

### Из плана (lines 5100-5110)

- **Orphan-assignment cleanup**: при `route_set_id IS NULL` после `ON DELETE SET NULL` запись `subaccount_routing_assignment` остаётся. Решить cron-cleanup или auto-DELETE при `provider_set_id IS NULL AND route_set_id IS NULL`.
- **Redis hardening на sandbox** (memory `project_redis_hijack_2026_05_04`): `requirepass` + firewall-правила. Не блокирует Plan 2, нужно перед prod-rollout.
- **`verifySubAccountOwnership` dedupe**: helper повторяется в `network_assignments.go` и `subaccount_network_overrides.go`. Вынести в shared package при появлении третьего consumer'а.
- **Operator-condition matching в preview**: `condition_type='operator'` всегда `true`. Реальный matching через таблицу `operators`.
- **Country-prefix mapping**: hardcoded в `network_route_preview.go`. Вынести в JSON / БД.
- **Audit-log UI**: данные пишутся в `audit_log`, страницы «История изменений» нет (spec §6.7).
- **Differential preview** «что изменится при применении нового шаблона» — отложено per spec §4.6.
- **Optimistic concurrency** (If-Unmodified-Since / 412) — отложено per spec §6.5.
- **Двухуровневая reseller-иерархия** UI — отложено per spec §3.4.

### Новые (из Plan 2 review-findings)

- **`ResellerOnlyMiddleware` не на `/sub-accounts/*` subrouter**: route-overrides защищены косвенно, через `verifyOwnership` (404 при несовпадении reseller↔subaccount). Defence-by-accident; для defence-in-depth добавить middleware на subrouter `/sub-accounts`.
- **Materialiser partial-failure post-commit (Tasks 10 + 12)**: `tx.Commit()` идёт раньше `RouteSetMaterializer.Materialize()`. При падении материализации DB-состояние уже зафиксировано — eventual consistency опирается на retry/repaint. Атомарность DB+materialise не гарантирована. Кандидат на refactor: материализация внутри той же транзакции либо outbox-pattern.
- **Group ordering `int16(idx)`**: в `parseItemIn` (`network_route_set_items.go:184`) `GroupIndex = int16(idx)` вместо `g.GroupIndex` из входа. Согласовано с writer'ом, но теряет порядок при неконтiguous значениях из API. Внешне выглядит как «ordered as received», что и нужно. Документировать или принять.
- **`country_code` / `operator_id` / `number_range` на override-cells**: override на тот же провайдер, что в template, падает на `uq_cell_provider`. Без cell-различающих полей в `parseItemIn` override-семантика «затереть провайдера для конкретной cell» не работает.

### Tooling chore

- **ESLint baseline ratchet**: после Task 21 фактическое число warnings = **51**. Поднять `package.json` и `scripts/check.sh` с 56 → 51.

---

## Verification artefacts

- Sandbox: `claude@72.56.232.202`, `/opt/sms`, master @ `d544532`.
- Postgres: `schema_migrations.version=137 dirty=false`.
- Containers: 40+, все healthy/Up на момент 2026-05-05 ~10:36 UTC.
