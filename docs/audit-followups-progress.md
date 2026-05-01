# Audit Follow-ups — Progress

**Старт:** 2026-05-01. План: `docs/superpowers/specs/2026-05-01-audit-followups-plan.md`.

Lock-механика: `[IN_PROGRESS]` перед началом задачи, `[DONE] task X.Y — N коммитов` после закрытия. `[BLOCKED]` для задач, требующих отдельного spec'a.

---

## Сессия 2026-05-01

### Эскалации §7
- A.1 BUG-83: вариант (a) — подтверждён.
- B.1 partner_id: V3 (deterministic UUID→int64 mapping в clients.partner_id) — подтверждён.
- B.2 i18n: WONTFIX — подтверждён, сохранён в memory `project_i18n_wontfix.md`.
- D.6 BUG-80 UNIQUE INDEX: применять — подтверждён.

### [DONE] A.1 BUG-83 — API-key scope enforcement на /portal/v1/campaigns/* — 1 коммит
**Commit:** `d6b41de fix(portal): BUG-83 — enforce API-key scope on /campaigns/* by HTTP method`
- middleware/scope.go: `DBScopeLoader` (gateway-side, hash token → DB) + `RequireScopeByMethod`.
- middleware/scope_test.go: 12 тестов — session bypass, GET/HEAD/OPTIONS=read, POST/PUT/DELETE/PATCH=write, пустые scopes 403, key not found 401, loader error 500, wildcard не подменяет exact-match, симметричный denied-loop по write-методам.
- router/router.go: middleware применён на campaigns subrouter после csrf.
- Defense-in-depth: SQL-предикат `expires_at > NOW()` дублирует auth-service.

**Архитектурное отступление от плана §2:** proto-расширение `ValidateTokenResponse.scopes` заблокировано — protoc недоступен (Device Guard на Windows + нет binary). Pivot на gateway-side loader. Семантически эквивалентно. TODO в коде: переехать на proto-вариант, когда окружение позволит regen, и удалить DBScopeLoader.

**Цена:** +1 DB-roundtrip на API-key запрос (некешированный). Кеш — отдельный follow-up если задержится.

**TC-SCOPE-1c (этап 28) — закрыт unit-тестом** `TestRequireScopeByMethod_APIKey_POST_RequiresSendScope/HasOnlyReadScope_POST_403`. Production smoke не выполнен (нужен real стенд).

### [DONE] BLOCK D batch — 4 quick PR'а — 1 коммит
**Commit:** `682c9f5 fix(portal): BLOCK D quick fixes — toast on RBAC denial, 404 page, profile validation`

- **D.2 + D.4** (этап 29 obs-2 + obs-4): RequireRole/RequireReseller получили useEffect → toast.error при denied + "Loading..." → "Загрузка..." для консистентности.
- **D.3** (этап 29 obs-1): NotFoundPage.tsx + Route `*` → NotFoundPage.
- **D.7** (этап 27 obs-2): PUT /portal/profile теперь возвращает 400 при попытке передать email/name (раньше silent-ignore).
- Бонус: Toast.tsx context value мемоизирован через useMemo — без этого RequireRole/RequireReseller useEffect зацикливался в StrictMode (CRITICAL ловлен code-reviewer'ом).
- Бонус: App.tsx admin Suspense fallback "Loading admin..." → "Загрузка...".

### [DONE] D.9 — GET /portal/v1/webhooks/{id} — 1 коммит
**Commit:** `baaddba fix(portal): D.9 — register GET /portal/v1/webhooks/{id}`
- Добавлен handler GetWebhook (использует webhookv1.GetSubscription gRPC, был доступен) + регистрация route. Раньше — 404 plain-text.

### [WONTFIX] D.5 — ErrorBoundary i18n
План говорил wrap в `useTranslation`. По решению B.2 (EN-портал не в roadmap) — RU-only by design. ErrorBoundary остаётся как есть.

### [SKIP] D.8 — /portal/cascade-history endpoint
План §D.8 говорил «UI ходит на `/portal/v1/cascade-history` → 404». При проверке: UI (`portal-frontend/src/api/cascade.ts:175-198`) реально использует `/cascade/deliveries`, `/cascade/strategies`, `/cascade/stats` — все зарегистрированы (`router.go:631-636`). Регрессии нет, аудит-отчёт §D.8 был fals positive. Закрываем без действий.

### [BLOCKED] D.1 — /admin/v1/clients silent-ignore is_reseller/max_sub_accounts
`clientv1.CreateClientRequest` и `UpdateClientRequest` proto не имеют полей `is_reseller`, `max_sub_accounts` (см. `api/proto/client/client.proto:64-72, 81-89`). Чистый фикс требует proto regen — тот же блокер, что A.1. Альтернатива — direct DB-update из gateway (layering violation, но прецедент есть в DBScopeLoader). Откладываю до сессии с рабочим protoc или явного решения о gateway-side подходе.

### [BLOCKED→DONE] B.1 — partner_id leak (V3 sequence mapping) — сессия 2026-05-01 (вторая)
Принято: V3 SEQUENCE (вместо hash64), без отдельного spec'a — прямой код.

**Изменения:**
- `migrations/000126_clients_partner_id.up.sql/.down.sql` — добавлена `clients.partner_id BIGINT NOT NULL` через `SEQUENCE clients_partner_id_seq START WITH 1`, UNIQUE INDEX, backfill через nextval, DELETE junk-bucket `network_stats_hourly WHERE partner_id=0`.
- `internal/services/network_analytics/application/aggregation_worker.go` — rawAggQuery теперь `COALESCE(p.partner_id, c.partner_id, 0) AS partner_id` с LEFT JOIN parent. Hierarchy resolution: sub-account aggregates под partner_id парента.
- `internal/gateway/portal/handlers/network_statistics.go` — новый метод `resolvePartnerID` с тем же hierarchy SQL, в struct добавлен `pool *pgxpool.Pool`. Все 8 callsites `partnerIDFromRequest(r)` → `partnerID, ok := h.resolvePartnerID(...)`. **Query-param `?partner_id=` полностью удалён.**
- `cmd/portal-gateway/main.go:307` — проброс `dbPool` в конструктор.
- `internal/gateway/portal/handlers/network_statistics_test.go` — 4 integration-тестa (build-tag `integration`): existing client → правильный partner_id; query-param ignored; sub-account → parent's partner_id; not-found → 401.

**Review:** 3 итерации `superpowers:code-reviewer`.
- Итерация 1: CRITICAL — sub-account stats регресс (variant 2 принят) + HIGH stale-data в network_stats_hourly.
- Итерация 2: hierarchy fix принят, новый HIGH — TRUNCATE в миграции destructive на проде.
- Итерация 3: APPROVED — TRUNCATE → точечный DELETE WHERE partner_id=0.

**Жертвы:** +1 SQL roundtrip на каждый network-statistics запрос (cache отложен); sequence раскрывает порядок регистрации (partner_id internal); down-migration теряет partner_id данные; тесты integration-only.

**TC-AGG-5 закрыт** — query-param утечка устранена и hierarchy aggregation работает.

### [BLOCKED] D.6 — BUG-80 TOCTOU race + UNIQUE INDEX миграция
БД-миграция: `CREATE UNIQUE INDEX ... ON clients (parent_client_id, lower(email)) WHERE active=true`. Применять — подтверждено пользователем.
Не выполнил в этой сессии — миграции БД делаются осознанно, через `scripts/server.sh migrate` с проверкой данных перед апликацией. Создать миграционный файл + проверить что `(parent_client_id, lower(email))` не дублируется в активных строках сейчас (если дубли — миграция упадёт, нужен сначала dedup).

**Action для следующей сессии:** написать миграцию + idempotent-fix существующих дубликатов (если есть).

---

## Открытые observations

- **TC-AGG-5** — partner_id leak не закрыт. Status: **BLOCKED** до отдельного spec'a B.1.
- **D.10 bundle** (api-keys/webhooks 4 минификса): не начат. RotateAPIKey требует proto regen → BLOCKED. UpdateAPIKey errors.Is, audit ClientID="", webhook signature replay test — самостоятельные мелкие задачи.

## Накопительные паттерны (BLOCK C) — не начаты

- C.1 uuid.Parse без handler-pre-check (434 callsites, ~3-4 рабочих дня)
- C.2 pgx error→500 sweep
- C.3 http.Error plain-text sweep (5 callsites)
- C.4 Дублирующие handler-level checkReseller cleanup
- C.5 401→403 для /reseller/*
- C.6 validation→500 в gRPC servers
- C.7 errors.Is sweep
- C.8 403 vs 404 info-disclosure unify
- C.9 DNS-rebinding bypass в webhook ValidateURL
- C.10 jsonb []byte serialization sweep

Объём — основной BLOCK плана. Делать batch'ами в отдельных сессиях.

---

## Quality gates по сессии

- `./scripts/check.sh` — PASS на всех коммитах. ESLint warnings без изменений (56, baseline 69).
- Go-чеки: skipped локально (Device Guard). CI валидирует строго.

---

## Memory updates

- Создан `project_i18n_wontfix.md` + добавлен в MEMORY.md index.
