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

### [BLOCKED→DONE] D.6 — BUG-80 TOCTOU race + UNIQUE INDEX — сессия 2026-05-01 (вторая)

**Отступление от плана:** план §D.6 предписывал `WHERE active=true`. Возражено и принято: индекс **БЕЗ** active-фильтра, симметричный существующему app-check'у (`ExistsByEmailUnderParent` тоже без active-фильтра). Это сохраняет product-семантику «email забронирован за parent навсегда, включая soft-deleted» (см. комментарий `sub_account_service.go:115-117`). `WHERE active=true` создал бы дыру: после soft-delete можно было бы создать второй active sub-account с тем же email → login-by-email вернул бы 2 строки.

**Изменения:**
- `migrations/000127_clients_parent_email_unique.up.sql/.down.sql` — `CREATE UNIQUE INDEX idx_clients_parent_email ON clients (parent_client_id, lower(email)) WHERE parent_client_id IS NOT NULL AND email IS NOT NULL AND email <> ''`. Pre-flight на локальном docker: 0 дубликатов.
- `internal/services/client/application/sub_account_service.go` — после `clientRepo.Create`: `errors.As(err, &pgErr)` + `pgErr.Code == "23505" && pgErr.ConstraintName == "idx_clients_parent_email"` → `ErrEmailExists`. Закрывает gap между app-check и INSERT.
- `internal/services/client/application/sub_account_service_test.go` — три новых mock-теста: race-fix (23505 на нашем индексе → ErrEmailExists), guard ConstraintName (23505 на api_key_key → bubble up), guard Code (23503 FK → bubble up).

**Review:** 1 итерация → APPROVED.

---

## Открытые observations

- **TC-AGG-5** — partner_id leak не закрыт. Status: **BLOCKED** до отдельного spec'a B.1.
- **D.10 bundle** (api-keys/webhooks 4 минификса): не начат. RotateAPIKey требует proto regen → BLOCKED. UpdateAPIKey errors.Is, audit ClientID="", webhook signature replay test — самостоятельные мелкие задачи.

## Накопительные паттерны (BLOCK C)

### [DONE] C.5 — 401→403 для /reseller/* (этап 27 obs-4) — сессия 2026-05-01 (вторая)

**Изменения:**
- `internal/gateway/portal/middleware/reseller_only.go:51` — `shared.ErrUnauthorized` → `shared.ErrForbidden` для `is_reseller=false`. Текст «доступ только для агрегаторов» сохранён. Случаи `!ok` от GetClientID и `pool == nil` не тронуты (401 и 500 соответственно — корректно).
- `internal/gateway/portal/middleware/reseller_only_test.go` — 3 integration-теста: non-reseller→403, no-clientID→401, reseller→passthrough.

**Зачем 403:** `portal-frontend/src/api/client.ts:37` редиректит на `/login` ровно при `status === 401`. Sub-account, попавший на `/reseller/*`, до C.5 ловил 401 → редирект → /login → дашборд → /reseller/* → бесконечный цикл.

**Review:** 1 итерация → APPROVED.

---

### [DONE] C.4 — handler-level checkReseller cleanup — сессия 2026-05-01 (вторая)

**Отступление от плана:** план §C.4 acceptance говорил «`grep checkReseller` → 0 callsites». Возражено: callsites используют возвращаемый clientID, простое удаление сломало бы handler'ы. Удалена только дублирующая SQL-проверка `is_reseller` (это и есть «мёртвый код» — middleware уже её делает). Метод `checkReseller` сохранён под историческим именем; переименование в `clientIDFromContext` — отдельный stylistic PR (§C.4.1).

**Изменения:**
- 8 файлов под `internal/gateway/portal/handlers/` (`reseller_dashboard.go`, `reseller_routing.go`, `reseller_tariffs.go`, `reseller_moderation.go`, `reseller_sender_names.go`, `reseller_templates.go`, `reseller_analytics.go`, `reseller_tariff_plans.go`) — удалён блок `var isReseller bool ... SELECT is_reseller ... return X, false`. Метод теперь только извлекает clientID. Canonical-комментарий в `reseller_dashboard.go`, остальные — short-pointer.
- `reseller_analytics.go` + `cmd/portal-gateway/main.go:306` — удалено dead `pool *pgxpool.Pool` поле + параметр конструктора (после удаления is_reseller SQL pool там больше не используется).

**Файлы, которые я НЕ трогал** (защита!): `network_tariff_bulk.go`, `network_tariff_editor.go`, `network_tariff_templates.go`, `network_tariffs_summary.go` — они под `protected` subrouter (router.go:428-452), не под `/reseller/*`. Их `checkReseller` — единственная защита, удаление сняло бы её.

**Review:** 1 итерация → APPROVED with HIGH-note (deviation от plan acceptance — задокументировано выше) + MEDIUM (dead pool — пофикшен в той же итерации) + LOW (`reseller_moderation.go` interface{} return type — pre-existing, отдельный issue).

---

## Накопительные паттерны (BLOCK C) — остатки

- C.1 uuid.Parse без handler-pre-check (434 callsites, ~3-4 рабочих дня)
- ~~C.2 errors.Is sweep для sql/pgx/redis sentinel'ов~~ — DONE (см. ниже)
- ~~C.3 http.Error plain-text sweep~~ — DONE (см. ниже)
- C.6 validation→500 в gRPC servers
- ~~C.7 errors.Is sweep (application/domain)~~ — DONE (см. ниже)
- C.8 403 vs 404 info-disclosure unify
- ~~C.9 DNS-rebinding bypass~~ — DONE (см. ниже)
- C.10 jsonb []byte serialization sweep
- C.2-wrap (separate task): WrapNotFound helper + переход на shared.ErrNotFound — НЕ сделано (это «pgx error→500» из основного описания плана §C.2; текущий sweep закрыл только sentinel-comparison, не error-mapping)

Объём — основной BLOCK плана. Делать batch'ами в отдельных сессиях.

---

### [DONE] C.3 — http.Error plain-text sweep (5 callsites) — сессия 2026-05-01 (третья)

**Изменения:**
- `internal/gateway/portal/handlers/auth.go:326` — CSRF-ген ошибка → `shared.ErrInternalServer`.
- `internal/gateway/portal/handlers/cascade_webhook.go:43,64` — invalid body → `ErrInvalidInput`; publish-error → `ErrInternalServer`. + import `shared`.
- `internal/gateway/portal/handlers/messages.go:563,570` — SSE недоступен → `ErrServiceUnavailable`; flusher-fail → `ErrInternalServer`.
- `internal/gateway/portal/handlers/ws_messages.go:53` — pre-upgrade unauthorized → `ErrUnauthorized`. + import `shared`.

**Acceptance:** `grep -rn "http\.Error(" internal/gateway/portal/ --include="*.go"` → **0**.

**Review:** APPROVED 1 итерация.

---

### [DONE] C.9 — DNS-rebinding защита через runtime safe-DialContext — сессия 2026-05-01 (третья)

**Отступление от плана (§4 C.9):** план предписывал хранить resolved IP в `webhook_subscriptions.resolved_ip` (БД-миграция). Реализовано лучше — runtime защита без миграции:
- `safeDialContext` resolve-ит hostname **на каждый dial** через `net.DefaultResolver.LookupIPAddr`, отбраковывает любой private/loopback/link-local/unspecified IP, далее dial по IP-литералу.
- TLS SNI и HTTP Host header сохраняются (URL.Host остаётся hostname).
- Закрывает rebinding-gap полностью (проверка происходит в момент TCP-dial).
- Бонус: легитимные публичные домены с rotated IP не блокируются устаревшим saved_ip.

**Изменения:**
- `internal/services/webhook/infrastructure/http/delivery_client.go` — добавлены `safeDialContext`, `isUnsafeIP` helper, `WithAllowPrivateIPs()` test-only option (variadic). `NewDeliveryClient` по умолчанию ставит safe-dial. `ValidateURL` оставлен синтаксическим (HTTPS-схема, длина, IP-literal blocklist) — DNS-resolve вынесен на dial-time, single source of truth.
- `internal/services/webhook/infrastructure/http/delivery_client_test.go` — добавлены 4 теста: `TestSafeDialContext_RejectsLoopbackIPLiteral`, `TestSafeDialContext_RejectsRFC1918IPLiterals` (включая 169.254.169.254 metadata + IPv6 ::1), `TestSafeDialContext_RejectsHostnameResolvingToLoopback`, `TestNewDeliveryClient_DefaultRejectsLoopback` (regression-guard на default-strict). Существующие httptest-based тесты получили `WithAllowPrivateIPs()` (httptest всегда на 127.0.0.1).

**Жертвы:** +1 DNS-resolve на каждое webhook-delivery (ранее resolve был только разово в Transport-internal). Performance hit ~ms — приемлемо.

**Review:** APPROVED 1 итерация.

---

### [DONE] D.10a — UpdateAPIKey gRPC handler errors.Is — сессия 2026-05-01 (третья)

**Изменения:**
- `internal/services/auth/grpc/server.go:362-368` — три `err == application.ErrAPIKey*` → `errors.Is(err, application.ErrAPIKey*)`. Скоуп точечный (D.10 пункт), полный sweep по auth/grpc и client/grpc (~20 callsites) — это C.7, отдельная задача.

**Review:** APPROVED 1 итерация.

---

### [DONE] D.10b — API-key audit ClientID="" + ActionAPIKeyUpdated — сессия 2026-05-01 (третья)

**Изменения:**
- `internal/gateway/portal/handlers/api_keys.go` — Create/Revoke/Update теперь подтягивают clientID из `middleware.GetClientID(ctx)` и передают в `audit.NewAuditEvent`. Empty-string fallback оставлен (admin без clientID — реальный кейс по `session_auth.go:93`).
- `internal/gateway/portal/handlers/api_keys.go::UpdateAPIKey` — string-literal action `"api_key.updated"` → константа `audit.ActionAPIKeyUpdated`. Игнорирование `Publish` error → логгирование (как в Create/Revoke, единообразие).
- `internal/shared/audit/event.go:20` — добавлена `ActionAPIKeyUpdated = "api_key.updated"` константа.

**Review:** APPROVED 1 итерация.

---

## Открытые observations (после сессии 2026-05-02)

- **D.10 bundle остаток:** RotateAPIKey endpoint (BLOCKED — нужен proto regen), webhook signature replay test (нужен time-travel mock или integration-стенд).
- **proto regen блокер:** A.1 финальная чистка (DBScopeLoader → proto-вариант), D.1 (is_reseller/max_sub_accounts mapping), RotateAPIKey — все упёрлись в недоступность protoc на Windows под Device Guard. Нужно стратегическое решение (CI-regen / accept gateway-side / разовый Linux-regen).
- **C.2-wrap (отдельный остаток C.2):** WrapNotFound helper + переход на `shared.ErrNotFound` для GET /portal/v1/messages/{несуществующий-uuid} → 404 (а не 500). Текущий C.2 sweep закрыл только sentinel-comparison паттерн, не HTTP error-mapping. Скоуп: ~10 PR-фрагментов по репозиториям + handlers.
- **authrepo.ErrUserNotFound (~2 callsite в auth/grpc/server.go):** repo-level sentinel, остался после C.7. Можно подобрать к C.2-wrap или как отдельный мини-PR.

---

### [DONE] C.7 — application/domain `errors.Is` sweep — сессия 2026-05-01 (четвёртая)

**Скоуп:** `err == application.Err*` и `err == domain.Err*` в gRPC servers и application services. Из плана §4 C.7 — этот блок про sentinel'ы из application/domain, без `sql.ErrNoRows`/`pgx.ErrNoRows`/`redis.Nil` (это C.2, ~75 callsites, отдельный block).

**Изменения:**
- `internal/services/auth/grpc/server.go` — 16 sentinel-callsites (application: ErrAPIKeyInvalid, ErrInvalidCredentials×2, ErrUserInactive×2, ErrInvalidToken, ErrCurrentPasswordWrong, ErrPasswordTooShort, ErrPasswordSameAsOld, ErrPasswordMismatch, ErrPasswordResetRateLimit, ErrPasswordResetInvalid; domain: ErrTOTPAlreadyEnabled×2, ErrTOTPNotEnabled×2, ErrInvalidTOTPCode, ErrPasswordResetTokenUsed, ErrPasswordResetTokenExpired).
- `internal/services/client/grpc/server.go` — 14 sentinel-callsites (ErrInvalidClientData, ErrClientNotFound×8, ErrConfigNotFound×2, ErrNotReseller, ErrSubAccountNotFound×3). + import `errors`.
- `internal/services/routing/grpc/server.go` — 22 sentinel-callsites (ErrNoMatchingRoute×2, ErrNoProviders×2, ErrRouteNotFound×2, ErrCountryNotFound×3, ErrOperatorNotFound×3, ErrPrefixNotFound×2, ErrInvalidMSISDN, ErrHLRProviderUnavailable+ErrHLRLookupFailed compound, ErrHLRProviderNotFound×3, ErrNumberInvalid). + import `errors`.
- `internal/services/billing/application/billing_service.go` — 2 callsites (compound с `&& firstID/secondID == toClientID`). + import `errors`.
- `internal/services/billing/application/pricing_service.go` — 1 callsite (ErrPricingRuleNotFound). + import `errors`.
- `internal/services/tarification/application/sender_service.go` — 1 callsite (ErrSenderRegistrationNotFound). + import `errors`.

Total: ~56 callsites в 6 файлах.

**Acceptance:** `grep -rE "err == application\.Err\w+|err == domain\.Err\w+" internal/ --include="*.go"` → **0**.

**Bonus-наблюдение reviewer'а:** `err == authrepo.ErrUserNotFound:421,529` — repo-sentinel, вне application/domain scope, не трогали (это потенциально часть C.2 или отдельный repo-cleanup).

**Review:** APPROVED 1 итерация (reviewer проверил completeness, compound-case `||`/`&&` preservation, единичный `errors` import per file).

---

### [DONE] C.2 — sql.ErrNoRows / pgx.ErrNoRows / redis.Nil sweep — сессия 2026-05-02

**Скоуп:** все `if err == X { ... }` для трёх runtime sentinel'ов: `sql.ErrNoRows`, `pgx.ErrNoRows`, `redis.Nil`. Pre-flight grep дал 91 callsite в 54 файлах internal/.

**Стратегия:** разбито на 3 batch'а по слоям, каждый — отдельный коммит + субагент-ревью + check.sh PASS + push.

**Batch 1** (commit `1b3bb09`): `internal/storage/*` — 6 файлов legacy `database/sql` репозиториев, 15 callsites (всё sql.ErrNoRows). Добавлен `errors` import во все 6 файлов.

**Batch 2** (commit `5024b1e`): `internal/services/*/infrastructure/repository/*` — 36 файлов DDD-репозиториев, 62 callsites (всё sql.ErrNoRows). Добавлен `errors` import в 27 файлов (9 уже имели). Затронуты модули: auth, billing, campaign, client, contact, messaging, routing, tarification, template, webhook.

**Batch 3** (commit `8a1bf3a`): остаток — admin/handlers (5 файлов, 7× sql.ErrNoRows), portal sse/schedules (2× pgx.ErrNoRows), routing route_repo/hlr_cache (1× pgx + 1× redis.Nil), session_manager + usage_tracker + shared/cache (3× redis.Nil). 12 файлов, 14 callsites. Добавлен `errors` import в 11 файлов (session_manager уже имел).

**Acceptance** (главное):
```
grep -rE "err == sql\.ErrNoRows|err == pgx\.ErrNoRows|err == redis\.Nil" internal/ --include="*.go" | wc -l
0
```

**Скоуп явно НЕ затронул** (зафиксировано в каждом review-брифе как out-of-scope):
- `application.Err*` / `domain.Err*` — закрыто C.7.
- `authrepo.ErrUserNotFound` (auth/grpc/server.go:421,529) — repo-level sentinel, отдельная задача.
- WrapNotFound helper / shared.ErrNotFound mapping — другая задача из основного описания плана §C.2 («pgx error→500»). Этот sweep закрыл только sentinel-comparison паттерн, не error-mapping в HTTP-ответ. Открытый item.

**Особо ценно для redis.Nil:** go-redis v9 в pipeline-режимах оборачивает Nil — `errors.Is` это документированная идиома, sweep даёт защиту от silent skip cache-miss веток.

**Жертвы:** один монолитный коммит (~91 правки в 54 файлах) был бы непрожёвываем для review-субагента. Разбиение на 3 batch'а добавило +2 ревью-цикла, но каждый прошёл APPROVED с первой итерации, completeness-grep отработал в каждом.

**Review:** APPROVED 1 итерация × 3 batch'а.

---

### [DONE] C.6 — validation→500 в cascade/grpc — сессия 2026-05-02

**Pre-flight:** grep подтвердил скоуп резко уже плана. Validation→500 паттерн (`fmt.Errorf("invalid X: %w", err)` без gRPC-mapping) присутствует ТОЛЬКО в `internal/services/cascade/grpc/handler.go` (17 callsites). billing/grpc, tarification/grpc — чисты. `internal/pipeline/` — это Kafka-стадии, не gRPC сервер; их errors идут в DLQ, к gRPC-клиенту не доходят, **out-of-scope C.6** (план §4 C.6 ошибочно их упомянул).

**Изменения** (commit `6a2380a`):
- `internal/services/cascade/grpc/handler.go` — 16 callsites переведены на `status.Error(codes.InvalidArgument, ...)`. Showcase pattern (был только в `GetDelivery`, line 88, 92) распространён на остальные методы: CreateDelivery, ListDeliveries, GetDeliveryStats, GetChannel, CreateChannel, UpdateChannel, ToggleChannel, GetStrategy, CreateStrategy (mode + step), UpdateStrategy (id + step), DeleteStrategy, GetOperatorChannelSupport, UpdateOperatorChannelSupport.
  - UUID-parse errors: message без `%v err` detail (uuid err'ы тривиальны: "invalid UUID format/length"). 13 callsites.
  - Enum errors (`ChannelTypeFromString`, `mode.IsValid()`): `%v err` сохраняет bad value в message. 3 callsites.
- `internal/services/cascade/grpc/handler_test.go` (новый) — `TestHandler_ValidationReturnsInvalidArgument`, table-driven, 17 sub-cases. Server{} с nil-сервисами работает: validation срабатывает до вызова application-сервисов.

**Out-of-scope (явно)**:
- `handler.go:79` — `fmt.Errorf("create delivery: %w", err)` это application-error fall-through от `s.cascade.CreateDelivery`, не validation. Требует `errors.Is` mapping для `domain.Err*` sentinel'ов.
- Множество `return nil, err` в файле для Get/Update/Toggle/Delete операций (Channel/Strategy/OCS) — также пропускают application-level domain-sentinel'ы как `codes.Unknown` → HTTP 500 вместо корректного 404/409. Это аналогичная проблема, но scope другой: **открытое наблюдение, не в C.6**, см. ниже.

**Acceptance:** `grep -rE "fmt\.Errorf.*[Ii]nvalid" internal/services/cascade/grpc/` → 0.

**Review:** APPROVED 1 итерация (reviewer подтвердил semantic-выбор кодов, justified line 79 как out-of-scope, тест покрывает все changed callsites).

---

## Открытые observations (после сессии 2026-05-02, дополнение)

- **cascade/grpc/handler.go: domain-sentinel mapping** — [DONE] 2026-05-02 commit `7c1ed16`. Helper `mapCascadeErr` (12 sentinels по категориям NotFound/AlreadyExists/FailedPrecondition/Internal) + 14 callsites переписаны на `return nil, mapCascadeErr(err)`. TestMapCascadeErr с 15 sub-cases (включая wrapped через `%w`).

- **proto-инфра baseline-regen ожидает X3-коммит 2** — после установки Docker-based proto-regen pipeline (см. секцию ниже) smoke-test показал, что регенерация `auth.proto`+`client.proto` даёт **~1100 строк diff'а в .pb.go**: (a) cosmetic rename header `source:` (`api/proto/auth/auth.proto` → `auth.proto`, `client/client.proto` → `client.proto`), (b) cosmetic rename внутренних helper'ов (`file_api_proto_auth_auth_proto_*` → `file_auth_proto_*`, `file_client_client_proto_*` → `file_client_proto_*`), (c) **dead-RPC enablement**: `IncrementMonthlySMSUsage` определён в `api/proto/client/client.proto:60`, message types сгенерированы в `client.pb.go:2314+`, но gRPC-stub отсутствует в `client_grpc.pb.go` — забытый частичный регенерат прошлой сессии. Никто не вызывает этот RPC в Go-коде; `internal/services/client/grpc/server.go:21` embed'ит `UnimplementedClientServiceServer`, поэтому при добавлении RPC ответ будет `codes.Unimplemented` без break'а. Этот baseline-regen уйдёт отдельным коммитом (X3-2), затем content-изменения для A.1/D.1/RotateAPIKey пойдут как чистые diff'ы.

- **Headers `source:` не воспроизводимы стандартным protoc** — текущие .pb.go файлы имели разнобой headers (`api/proto/auth/auth.proto`, `client/client.proto`, `auth.proto`, ...) — следствие того, что в прошлом регены делались разными командами/инструментами (возможно `buf` или ручное перемещение файлов). Реверс этих headers через `protoc --go_opt=paths=source_relative` невозможен. После baseline-regen все headers нормализуются на `<basename>.proto`. После этого `proto-regen.sh` идемпотентен.

- **18 других .proto файлов не нормализованы** — текущий baseline-regen покрывает только auth и client (целевая работа A.1/D.1/RotateAPIKey). Остальные 18 (cascade, billing, tarification, webhook, audit, company, link, messaging, routing, provider, smpp, analytics, contact, campaign, template, sms, network_analytics, sender-name) остаются на разных версиях protoc-gen-go (v1.34.1, v1.36.11) и protoc (v3.21.12, v4.25.1, v6.31.1). `network_analytics` и `sender-name` вообще не имеют сгенерированных `.pb.go`. Если когда-нибудь захочется унифицировать — отдельная задача (Variant B/C из стратегического разговора), не блокирующая.

---

### [DONE] A.1 финальная чистка — DBScopeLoader removal — сессия 2026-05-02

**Pre-flight:** TODO в `internal/gateway/portal/middleware/scope.go:40-44` явно описывал желаемое архитектурное решение — scopes из `ValidateTokenResponse.scopes` вместо второго SQL-запроса через `DBScopeLoader`. Блокер был proto regen (Device Guard), который разблокирован X3-1/X3-2.

**Архитектурный выбор:** Variant B2 (scopes в context) + tuple-сигнатура AuthenticateByAPIKey (a). Подтверждено пользователем. Альтернативы — gRPC-side loader (B1, второй gRPC-roundtrip) и AuthResult-struct (b, premature abstraction) — отклонены.

**Изменения** (commit `d5f0658`):
- `api/proto/auth/auth.proto`: добавлено поле `repeated string scopes = 4;` в `ValidateTokenResponse`. Регенерировано через `./scripts/proto-regen.sh auth` — diff +14/-5, чистый additive.
- `internal/services/auth/application/auth_service.go::AuthenticateByAPIKey`: сигнатура `(*User, error)` → `(*User, []string, error)`. Scopes уже загружались `apiKeyRepo.GetByKeyHash`, теперь возвращаются наружу. Cache hit/miss обновлены.
- `internal/services/auth/grpc/server.go::ValidateToken`: после `AuthenticateByAPIKey` передаёт scopes в response. JWT path: scopes пусты (JWT не имеет scope-концепции).
- `internal/gateway/portal/middleware/api_key_auth.go`: после `ValidateToken` кладёт `resp.Scopes` в context под `APIKeyScopesKey`.
- `internal/gateway/portal/middleware/scope.go`: удалены `ScopeLoader` interface, `DBScopeLoader` struct, `NewDBScopeLoader`, `Load` (~90 строк). Удалены imports `crypto/sha256`, `encoding/hex`, `pgxpool`. `RequireScopeByMethod(loader, ...)` → `RequireScopeByMethod(...)` без loader-параметра. Read scopes — через `GetAPIKeyScopes(ctx)`. Контракт ошибок: scopes отсутствуют в context при `auth_method=api_key` → 401 (safe-default; инвариант `APIKeyAuthMiddleware` всегда обязан класть scopes).
- `internal/gateway/portal/router/router.go:305`: `campaignsScopeLoader := middleware.NewDBScopeLoader(dbPool)` удалён.
- `internal/gateway/portal/middleware/scope_test.go`: переписан с stub-loader на context-based setup. `KeyNotFound_401` → `NoScopesInContext_401` (regression-guard на инвариант), `LoaderError_500` удалён.
- `internal/services/auth/application/auth_service_test.go`: 6 callsites обновлены под новую сигнатуру. Happy-path расширен `apiKey.Scopes = [...]` + assertion на возвращаемые scopes.
- `test/functional/auth_test.go`: 3 callsites обновлены, добавлен `ElementsMatch` assertion на возвращаемый scopes (functional-тест на новый контракт; найден code-reviewer'ом, был bypass'ен под `//go:build functional` тегом и не виден в стандартном `./scripts/check.sh`).

**Архитектурный выигрыш:** 0 дополнительных round-trip'ов. Scope'ы — часть auth-результата, едут в context рядом с `User`/`AuthMethod`/`APIKeyID`.

**Жертвы:** смена контракта `RequireScopeByMethod` (loader → context). Тесты переписаны (12 sub-cases). Inлyrant: APIKeyAuthMiddleware обязан класть scopes — нарушение даёт 401 на любом auth_method=api_key запросе.

**Quality gates:** `./scripts/check.sh` PASS, `go test` для затронутых пакетов PASS (auth/application, auth/grpc, portal/middleware).

**Pre-existing наблюдение** (не блокер): `test/functional/tarification_test.go:411` имеет vet error `futureEnd time.Time vs *time.Time` — существует на чистом master без A.1 изменений (verified через `git stash` + `vet`). Кандидат на отдельный housekeeping fix.

**Review:** APPROVED после 1 цикла CHANGES_REQUESTED → fix functional-test callsites.

---

### [DONE] D.1 — is_reseller/max_sub_accounts mapping в admin client API — сессия 2026-05-02

**Pre-flight:** этап 28 obs-6 показал silent-ignore — admin POST принимал поля, БД их не сохраняла. Корень: proto `CreateClientRequest`/`UpdateClientRequest` не имели полей. БД-столбцы (`migrations/000016`), domain.Client, repository, proto `ClientInfo` (read path) — всё уже было; missing был только write path в proto.

**Архитектурные решения:**
- proto Create: plain `bool`/`int32` (полное состояние при создании).
- proto Update: `optional bool`/`optional int32` (proto3 has-accessor — pointer-style в Go, nil = не менять).
- Business rule `is_reseller=true ⇒ max_sub_accounts >= 1` — application слой (БД CHECK ловит только top-level invariant).
- Frontend admin UI вне scope D.1 (план явно "в handler добавить proper field mapping").

**Изменения** (commit `4092250`):
- `api/proto/client/client.proto`: +`is_reseller=8`/`max_sub_accounts=9` в Create (plain) и Update (optional).
- `application.CreateClient`: расширена сигнатура. Валидация: negative + reseller-без-слотов → `ErrInvalidClientData`.
- `application.UpdateClient`: расширена сигнатура (pointer-style). Validation на итоговом состоянии после применения PATCH.
- `internal/services/client/grpc/server.go::UpdateClient`: добавлена ветка `errors.Is(err, ErrInvalidClientData) → InvalidArgument` (раньше 500; симметрично с CreateClient).
- `internal/gateway/admin/handlers/clients.go`: +`IsReseller`/`MaxSubAccounts` в JSON DTO (Create/Update/ClientInfo). `Validate()` дублирует business rule для CreateClient short-circuit. UpdateClient handler short-circuit'ит на negative; reseller-инвариант делегирован application слою (handler не знает итогового состояния).
- `clientInfoToResponse`: пробрасывает в GET response.
- 9 test callsites обновлены под новую arity. 5 новых regression-тестов (Create + Update × negative + reseller-без-слотов).

**Quality gates:** check.sh PASS, go test для затронутых пакетов PASS.

**Review:** APPROVED после 1 итерации CHANGES_REQUESTED (нашёл missing `ErrInvalidClientData` mapping в UpdateClient gRPC + предложил business rule для reseller-без-слотов; оба исправлены).

**Pre-existing observations (не блокеры D.1, зафиксированы как housekeeping):**
- ~~`api/proto/clientv1/client/client.pb.go` — stale duplicate (унаследовано от X3-2)~~ — [DONE] commit `da450a7` (2026-05-02). `git rm -r api/proto/clientv1/client/`. Pre-flight: 0 импортов в .go/.sh/.proto, оба файла tracked dead code (37 типов vs 39 в актуальном). Reviewer APPROVED 1 итерация: подтвердил, что proto-regen.sh + Docker pipeline не воссоздают подкаталог.
- ~~admin UpdateClient: `var active bool` always non-nil → PATCH без `active` сбрасывает active в false. Pre-existing, не D.1 регрессия~~ — [DONE] commit `3e52acb` (2026-05-02). Variant A: `bool active` → `optional bool active` в proto + regen (wire-compatible: synthetic oneof, varint tag-6 не меняется). gRPC server и admin handler переведены на pointer pass-through. 2 новых regression-теста (active=nil preserves, active=false applies). Reviewer APPROVED 1 итерация: подтвердил wire-format, strict mock pin, консистентность с D.1.
- ~~DB CHECK leak (parent_client_id+is_reseller) → 500 вместо 400. Pre-existing~~ — [DONE] commit `35b7ea5` (2026-05-02). Variant A: application-level pre-validation в UpdateClient (`if client.IsReseller && client.ParentClientID != nil → ErrInvalidClientData`), тот же паттерн что reseller-без-слотов. ErrInvalidClientData → codes.InvalidArgument → HTTP 400. CreateClient/CreateSubAccount не нуждаются (по конструкции не нарушают CHECK). 1 regression-тест с AssertNotCalled(Update) — доказывает short-circuit. Reviewer APPROVED 1 итерация: подтвердил impossibility-of-legacy-violation (CHECK создан той же миграцией что и колонки).
- `updates_fields` test coverage gap для IsReseller/MaxSubAccounts unchanged-when-nil.

---

### [DONE] D.10 (часть) — RotateAPIKey endpoint — сессия 2026-05-02

**Pre-flight:** этап 25 obs-6 — endpoint отсутствовал. После X3 разблокировки proto regen реализуется end-to-end.

**Архитектурный выбор пользователя:** B (soft rotate с grace period) + II (sequential без транзакций).

**Изменения** (commit `731d906`):
- `migrations/000128`: + `api_keys.revoke_at TIMESTAMPTZ NULL` + partial index. Применена на sandbox (миграция 128/u).
- `api/proto/auth/auth.proto`: + RPC `RotateAPIKey`, messages `RotateAPIKeyRequest/Response`, + `revoke_at` в `APIKeyInfo`.
- `domain.APIKey`: + `RevokeAt *time.Time`, + `IsRevoked()`. `IsValid()` теперь учитывает revoke_at — lazy invalidation, без background worker'а.
- `repo`: + `revoke_at` во всех SELECT/INSERT, + методы `SetRevokeAt`/`Delete` (Delete только для compensating-rollback).
- `application.RotateAPIKey`: возвращает `(newKey, rawKey, oldRevokeAt, err)`. Sequential semantic: Create new → SetRevokeAt(old, now+24h). При падении SetRevokeAt — best-effort `Delete(new)`.
- `gRPC server.RotateAPIKey`: маппит errors → InvalidArgument/NotFound/PermissionDenied/FailedPrecondition.
- `portal HTTP`: `POST /api-keys/{id}/rotate` + audit `ActionAPIKeyRotated`. + `revoke_at` в ListAPIKeys JSON.
- `frontend`: `apiKeysApi.rotate` + кнопка Rotate (disabled при `revoke_at != null`) + ConfirmDialog с предупреждением о 24h + Result modal с copy-кнопкой и timestamp'ом отключения.
- 6 mock-файлов обновлены под новые interface-методы.
- 4 unit-теста на `application.RotateAPIKey`: success (assert на oldRevokeAt ≈ now+24h), not_owned, revoked, compensating_delete.

**Архитектурные жертвы:**
- Soft rotate захардкожен на 24h (`APIKeyRotateGrace`). Конфигурируемость — отдельная задача.
- `authCache` (TTL ~60s) не инвалидируется при rotate. Reviewer подтвердил: 60s << 24h grace, безопасно. Если grace когда-нибудь сократят до < TTL — нужна явная инвалидация.
- Sequential без транзакций: window секунд "оба ключа активны" если Create→SetRevokeAt падает между шагами. Митигация — compensating Delete.

**Quality gates:** check.sh PASS, targeted go test PASS.

**Bundle статус (план §D.10):**
- D.10a (UpdateAPIKey errors.Is) — DONE (commit 73194e7).
- D.10b (audit ClientID="" + ActionAPIKeyUpdated) — DONE (73194e7).
- RotateAPIKey — DONE этим коммитом.
- Остаётся: webhook signature replay test (требует time-travel mock или integration-стенд) — отдельная задача.

**Review:** APPROVED 1 итерация. Единственный nit (drift `OldKeyRevokeAt` от `time.Now()` recompute) исправлен в этом же коммите через возврат точного `revokeAt` из application.

---

### [DONE] D.10 (последний item) — Webhook signature replay test — сессия 2026-05-02

**Pre-flight находка:** `signPayload` = `HMAC-SHA256(payload, secret)` без timestamp/nonce. Replay protection отсутствует на отправителе. Атакующий с перехваченным запросом может реплеить с валидной подписью; защита возможна только на receiver-side через `X-Webhook-ID` dedup.

**Архитектурный выбор пользователя:** Вариант 1 — закрыть буквальную формулировку плана (тест на существующее поведение) + явная SECURITY LIMITATION в коде/доках. Полноценный fix (Stripe-style timestamp в подписи) — отдельная security-задача с migration window (breaking change для existing receiver verification кода).

**Изменения** (commit `db79690`):
- `delivery_client.go::signPayload`: расширенный docstring с разделом SECURITY LIMITATION — документирует отсутствие replay protection, митигацию через X-Webhook-ID dedup, план полноценного fix.
- `delivery_client_test.go`: + `TestSignPayload_DeterministicAcrossTime` (100 вызовов → одинаковый output) + `TestDeliver_ReplayProducesIdenticalSignature` (3 Deliver вызова → 3 идентичных X-Webhook-Signature). Оба теста явно описывают что они подтверждают **limitation**, не корректность защиты.

**Открытое security-наблюдение (не блокер audit-followups, отдельная задача):**
- Полноценная replay protection через timestamp в подписи (Stripe-style: `HMAC(timestamp + "." + payload)`, `X-Webhook-Timestamp` header, ±5 мин окно на receiver). Breaking change для existing webhook-получателей. Требует migration window (например dual-sign: оба формата параллельно неделя, потом cutover).

**Review:** APPROVED 1 итерация.

---

## D.10 bundle — финальный статус

Все 4 items закрыты:
- D.10a — UpdateAPIKey errors.Is (commit 73194e7).
- D.10b — audit ClientID="" + ActionAPIKeyUpdated (73194e7).
- RotateAPIKey endpoint (731d906) — soft rotate с 24h grace.
- Webhook signature replay test (db79690) — regression-guard + SECURITY LIMITATION documented.

---

## Proto-regen инфраструктура (Variant A, 2026-05-02)

Создан Docker-based pipeline для regen'а .proto файлов с pinned версиями инструментов.

**Версии (фиксированы в `deployments/docker/proto-gen.Dockerfile`):**
- `protoc` = v25.1 (=v4.25.1 в Go-плагин-нотации; release tag "v25.1" после major bump)
- `protoc-gen-go` = v1.36.11
- `protoc-gen-go-grpc` = v1.6.1

Совпадают с текущими headers в `api/proto/{auth,client}v1/*.pb.go`.

**Wrapper:** `scripts/proto-regen.sh`. Регистр target'ов в массиве `TARGETS`. Использование:
- `./scripts/proto-regen.sh` — regen всех target'ов из реестра.
- `./scripts/proto-regen.sh auth client` — regen конкретных target'ов.
- `./scripts/proto-regen.sh --build` — пересобрать Docker-образ (после изменения Dockerfile).

**Стратегия `proto_path_arg`:** для каждого target'а указываем директорию .proto (`api/proto/<dir>`) и basename (`<file>.proto`). Это даёт идемпотентный regen без подкаталогов в output. Жертва — header `source:` нормализуется на `<basename>.proto` (одностроковый cosmetic diff при первом regen).

**MSYS-quirk fix:** на Windows/MSYS bash автоматически конвертирует POSIX-пути в Windows-пути, ломая `-w /src` Docker. Wrapper использует `MSYS_NO_PATHCONV=1` + `pwd -W` для корректной передачи Windows-form.

**Расширение реестра:** добавить новый target = добавить строку `name|proto_relative_file|proto_path_arg|out_dir` в массив `TARGETS`. Пример для расширения на cascade: `"cascade|cascade.proto|api/proto/cascade|api/proto/cascadev1"`.

---

## Quality gates по сессии

- `./scripts/check.sh` — PASS на всех коммитах. ESLint warnings без изменений (56, baseline 69).
- Go-чеки: skipped локально (Device Guard). CI валидирует строго.
- C.2 sweep (3 batch'а, 91 правка в 54 файлах): check.sh PASS после каждого batch'а. Final acceptance grep → 0.
- C.6 (cascade validation→InvalidArgument): check.sh PASS, acceptance grep → 0 в cascade/grpc.

---

## Memory updates

- Создан `project_i18n_wontfix.md` + добавлен в MEMORY.md index.
