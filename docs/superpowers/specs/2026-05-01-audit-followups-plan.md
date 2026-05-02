# Audit Follow-ups — Plan

**Дата:** 2026-05-01
**Источник:** итоги скоупа D полного re-audit (`docs/superpowers/specs/2026-04-29-ux-full-reaudit-design.md`, прогресс `docs/ux-audit-progress.md`)
**Цель:** закрыть все открытые после аудита эскалации и накопительные паттерны одним координированным проходом в новом чате.
**Single source of truth для нового чата.** Прочитать его + CLAUDE.md + MEMORY.md + `docs/ux-audit-progress.md` (последние 300 строк).

---

## 1. Структура исполнения

Каждая задача ниже — самостоятельная, имеет:
- **Pre-flight check** (что прочесть/проверить до начала)
- **Точные файлы и строки** для правки
- **Acceptance criteria** (конкретные, проверяемые)
- **Через какой механизм** (`/execute-with-review` обязателен для код-чейнджей; SQL-миграции — стоп+эскалация по §5.4)

Идти **по приоритету** (CRITICAL → HIGH → MEDIUM → накопительные паттерны). Не идти параллельно — lock-механика как в аудите.

**Между задачами:** TodoWrite update, commit (одна задача = один-два коммитa). После каждого блока (BLOCK A/B/C/D) — `./scripts/check.sh` + verification snippet.

**Жертвы признаны явно:** план объёмный (~5-7 рабочих дней). Часть задач — refactor (uuid.Parse 434 вхождения), часть — архитектура (i18n, partner_id mapping). Не принимаем "сделаем как-нибудь" — каждая задача либо доводится до закрытия, либо явно отмечается `BLOCKED: <reason>` и выносится в отдельный spec.

---

## 2. BLOCK A — CRITICAL security (приоритет 1)

### A.1. BUG-83: API-key scope enforcement на `/portal/v1/campaigns/*` — [DONE] 2026-05-02

**Финальная чистка завершена** (commit `d5f0658`): proto-расширение `ValidateTokenResponse.scopes` через X3 pipeline разблокировало архитектурно правильное решение. `DBScopeLoader` workaround удалён (~90 строк), `RequireScopeByMethod` теперь читает scopes из context (положены `APIKeyAuthMiddleware` после ValidateToken). Дополнительный SQL-roundtrip устранён. Подробности: `docs/audit-followups-progress.md`.



**Pre-flight:**
- Прочесть `internal/gateway/portal/middleware/api_key_auth.go:47-122`, `internal/services/auth/domain/api_key.go:73-90` (HasScope), `internal/services/auth/infrastructure/repository/api_key_repository.go:251` (LoadScopes), `api/proto/authv1/auth.proto` (ValidateTokenResponse), `internal/gateway/portal/router/router.go:294-322` (campaigns subrouter).
- Воспроизвести: создать api-key c `scopes=["messages:read"]`, POST /portal/v1/campaigns с Bearer → должно быть 403, сейчас 201 (этап 28 TC-SCOPE-1c).

**Решение (вариант a из BUG-83 — полная реализация):**

1. Расширить `authv1.ValidateTokenResponse`:
   ```proto
   message ValidateTokenResponse {
     bool valid = 1;
     UserInfo user = 2;
     repeated string scopes = 3;  // NEW: scopes API-key, для session — пусто
   }
   ```
   Файл: `api/proto/authv1/auth.proto`. После — `protoc` regen.

2. В `internal/services/auth/application/auth_service.go::ValidateToken` (или эквивалент) — после успешной верификации API-key вызвать `apiKeyRepo.LoadScopes(apiKeyID)` и положить в response.

3. В `internal/gateway/portal/middleware/api_key_auth.go::APIKeyAuthMiddleware` (после строки 110) — положить scopes в context:
   ```go
   const APIKeyScopesKey contextKey = "api_key_scopes"
   ctx = context.WithValue(ctx, APIKeyScopesKey, resp.Scopes)
   ```
   Добавить getter `GetAPIKeyScopes(ctx) []string`.

4. Создать `internal/gateway/portal/middleware/scope.go::RequireScope(scope string)` middleware:
   - Если `GetAuthMethod(ctx) != AuthMethodAPIKey` — пропустить (session-auth не имеет scope-ограничений).
   - Иначе если `scope` не в `GetAPIKeyScopes(ctx)` — `response.Error(w, shared.ErrForbidden("scope " + scope + " required"))`.

5. Применить на `campaigns` subrouter (`internal/gateway/portal/router/router.go:297-322`) **per-method**:
   - GET `/campaigns`, GET `/campaigns/{id}`, GET `/campaigns/{id}/stats|timeline|variants/compare|heatmap|optimal-time|report` — `RequireScope("messages:read")`
   - POST `/campaigns`, PUT `/campaigns/{id}`, POST `/campaigns/{id}/launch|pause|resume|cancel|select-winner|retry`, PUT `/campaigns/{id}/variants|ab-config|retry-config` — `RequireScope("messages:send")`
   - DELETE `/campaigns/{id}` — `RequireScope("messages:send")` (нет отдельного `messages:delete` в UI — пусть send включает delete)
   - POST `/campaigns/templates/preview` — `RequireScope("messages:read")`
   
   Реализация: вместо полностью subrouter-wide middleware (т.к. разные методы), оборачивать каждый `HandleFunc` в helper или использовать gorilla middleware с method-matching.
   
   Альтернатива: один middleware на campaigns subrouter, который смотрит на `r.Method` и сам выбирает required-scope (read для GET, send для POST/PUT/DELETE). Проще и достаточно.

6. Обновить UI `portal-frontend/src/pages/api-keys/APIKeysPage.tsx:11-17` — оставить `AVAILABLE_SCOPES` как есть, но добавить hint "messages:send включает create/update/delete/launch кампаний".

7. Юнит-тесты:
   - `internal/gateway/portal/middleware/scope_test.go`: 4 случая — session bypass, api-key с правильным scope OK, api-key без scope 403, api-key с пустыми scopes 403.
   - Расширить существующий `api_key_auth_test.go` — проверить scopes попадают в context.
   - Расширить `internal/services/auth/application/*_test.go` — ValidateToken возвращает scopes для api-key, не возвращает для session JWT.

**Acceptance:**
- TC-SCOPE-1c (этап 28) перестаёт воспроизводиться: POST /campaigns с Bearer scope=read → 403.
- Существующие campaigns-тесты (`tests/...campaigns...`) проходят — backward compat для session-auth.
- `./scripts/check.sh --with-tests` PASS.
- На стенде: создать новый api-key с scope=messages:read → POST → 403; создать с scope=messages:send → POST → 201.

**Риски:**
- Existing seeded api-keys (`e0000000-...0001` имеет messages:send + messages:read + messages:delete) — должны работать. Проверить демо-seed.
- Если кто-то использовал api-key без scopes (legacy create), POST упадёт. Migration: `UPDATE api_key_scopes` или backfill. Проверить, есть ли api-keys без scopes на проде (запрос `SELECT api_key_id, count(*) FROM api_key_scopes GROUP BY 1 HAVING count(*)=0` или LEFT JOIN). Если да — добавить миграцию.

**Через:** `/execute-with-review`. Объём работы: ~6-8 файлов, ~2-3 review-цикла. Если миграция api_key_scopes понадобится — стоп, эскалация (миграция БД).

---

## 3. BLOCK B — HIGH архитектурные

### B.1. partner_id leak cross-aggregator (network-analytics)

**Pre-flight:**
- Прочесть `internal/gateway/portal/handlers/network_statistics.go:137-144` (partnerIDFromRequest), `internal/services/network_analytics/...` (gRPC server), миграции `migrations/*.sql` для `network_stats_hourly` (схема таблицы).
- Воспроизвести: создать reseller2, login → GET /portal/v1/reseller/statistics → видит KPI Demo-Reseller через partner_id=0 общий bucket.

**Brainstorming решений** (каждое имеет trade-off, нужно решение пользователя ДО кода):

**Вариант 1: client_id-фильтр в gRPC (минимально-инвазивный)**
- В gRPC NetworkAnalyticsService.GetStatistics добавить required field `client_id` (UUID).
- В query `WHERE partner_id IN (SELECT partner_id FROM ??? WHERE client_id=$1)` — но если partner_id mapping нет, фильтрация невозможна.
- Цена: требует добавить mapping table `client_partner_mapping (client_id, partner_id)` или хранить `client_id` непосредственно в `network_stats_hourly`. Schema migration.

**Вариант 2: Хранить UUID в network_stats_hourly (radical refactor)**
- Заменить `partner_id INT` на `client_id UUID` в `network_stats_hourly`.
- Миграция БД (large table, потенциально миллионы записей на проде).
- Все upstream'ы (pipeline-aggregator, dlr-delivery, что бы туда ни писало) — переписать на UUID.
- Цена: >1 модуля, БД миграция → стоп, отдельный spec.

**Вариант 3: Deterministic UUID→int64 hash mapping**
- При регистрации reseller'а вычислять `partner_id = hash64(client_id)`.
- Сохранять mapping в `clients.partner_id INT` или отдельной таблице.
- Pipeline всегда читает `client_id` → resolves через mapping → пишет partner_id.
- Цена: новый field/таблица, легче чем V2.

**Рекомендация:** Вариант 3 (mapping в clients-table) — minimum migration cost, держит текущие int64 partner_id stable.

**Действие в плане:**
1. **СТОП — эскалация решения пользователю**: какой вариант (1/2/3).
2. После выбора — отдельный spec `2026-05-XX-network-analytics-tenant-isolation-design.md` со схемой миграции.
3. До закрытия: добавить временную защиту в `network_statistics.go::partnerIDFromRequest` — игнорировать `?partner_id=` query param и брать только из session `clientID` (если mapping есть) или возвращать `403 not implemented` для агрегаторов вне seed.

**Acceptance временной защиты:**
- TC-AGG-5 (этап 28): reseller A с `?partner_id=<reseller-B-int>` → 403 или возвращает данные partner_A, не partner_B.

**Через:** Stop. Эскалация. После решения — `/execute-with-review`.

### B.2. i18n: portal/admin hardcoded RU

**Pre-flight:**
- Прочесть `portal-frontend/src/i18n/index.ts` + `ru.json` + `en.json`.
- `grep -rE "[А-Яа-я]{4,}" portal-frontend/src/pages/messages portal-frontend/src/pages/admin --include="*.tsx" -l | wc -l` → 158 TSX.

**Brainstorming:**

**Вариант 1: incremental — extract по странице**
- Pageлям 30+ страниц портала, по 1 на этап (как audit). 
- За PR: прочитать TSX → каждую RU-строку извлечь в `ru.json` под ключом → импортировать `useTranslation()` → заменить.
- Цена: длинно (30+ PR), но контролируемо.

**Вариант 2: machine-extract + manual review**
- Скрипт парсит TSX, ищет `>[А-Яа-я]+<` или `"[А-Яа-я]+"` в JSX-text/JSX-attr → генерит ключ типа `pages.messages.title` → переписывает TSX → дополняет ru.json + en.json (en.json пустой "").
- Один большой PR, потом по странице ревью + перевод EN.
- Цена: рискованный auto-rewrite (можно сломать interpolation, conditional rendering); требует careful review.

**Вариант 3: stop, признать что EN-портал не приоритет**
- Зафиксировать в `MEMORY.md` "portal/admin RU-only by design"; OBS-5 закрывается как WONTFIX.
- Цена: 0. Но если когда-нибудь EN понадобится — это будет boom-проект.

**Рекомендация:** Вариант 3 (stop) если EN-портал не в roadmap пользователя. Спросить.

**Действие:**
1. **СТОП — эскалация:** запланирован ли EN-портал в roadmap? Если нет — закрыть OBS-5 как WONTFIX в `MEMORY.md`.
2. Если да — отдельный spec `2026-05-XX-portal-i18n-extraction-design.md` (выбрать V1 или V2, оценить объём).

**Через:** Stop. Эскалация.

### B.3. Dev-стенд E2E pipeline (OBS-1 этапа 30)

**Pre-flight:**
- Прочесть `test/load/fixtures/demo_seed.sql` (untracked? проверить `git ls-files | grep demo_seed`).
- Понять resolver: где destination prefix → operator_id resolved? `internal/pipeline/router/...` или `internal/router/...` — найти `operator_id` lookup.
- Воспроизвести: текущее состояние providers (5 active=false), platform_routes пусто, попытка POST /messages → DLQ "no_route".

**Действие:**

1. Найти resolver:
   ```bash
   grep -rn "operator_id\|resolveOperator\|lookupOperator" internal/pipeline/router/ internal/router/ internal/services/router/
   ```
   Скорее всего читает из `operator_prefixes` или `route_condition_groups`.

2. Расширить `test/load/fixtures/demo_seed.sql`:
   - Добавить активный stub-provider (`UPDATE providers SET active=true WHERE id='a0000000-...0001'` или INSERT новый).
   - INSERT в `stub_provider_config (provider_id) VALUES (...)` — default rates = success.
   - INSERT в `platform_routes (operator_id, channel_type, provider_id, priority, active)` — соответствующий `operators.id` (например `10000000-0000-0000-0000-000000000001` МТС).
   - INSERT в `operator_prefixes (operator_id, prefix)` — `+7999` → МТС, чтобы resolver сработал.

3. Написать e2e-test: `tests/e2e/full_pipeline_test.go` или `e2e/full_pipeline.spec.ts` (Playwright), который:
   - Создаёт клиента
   - Top-up
   - User login + sender create + admin approve
   - POST /messages
   - Wait DLR (~5s)
   - Assert message status=delivered
   - Assert balance debited
   - Assert transaction in /portal/billing/transactions

4. Подключить к CI (если есть e2e job в `.github/workflows/`).

**Acceptance:**
- На свежем стенде после `psql < test/load/fixtures/demo_seed.sql` POST /messages → status=delivered через ≤10s.
- E2E test PASS локально и в CI.

**Через:** `/execute-with-review`. БД-миграция формально не нужна (это test fixture), но изменение seed может ломать существующие тесты — стоп, ревью.

---

## 4. BLOCK C — MEDIUM накопительные паттерны (refactor)

### C.1. uuid.Parse без handler-pre-check (BUG-64 sweep)

**Pre-flight:** `grep -rE "uuid\.Parse\(" internal/gateway/ --include="*.go" | wc -l` → 434 вхождений в 48 файлов handlers/.

**Действие:**
1. Создать utility `internal/shared/httputil/parse_uuid_param.go::ParseUUIDParam(r *http.Request, key string) (uuid.UUID, *shared.AppError)` — читает из mux.Vars или query, парсит, при ошибке возвращает `shared.ErrInvalidInput("неверный формат " + key)`. (Возможно уже есть — проверить `internal/shared/`.)
2. Sweep по 48 файлов: `mux.Vars(r)["id"]; uuid.Parse(id)` → `id, err := httputil.ParseUUIDParam(r, "id"); if err != nil { respondError(w, err); return }`.
3. По одному файлу = один коммит, через `/execute-with-review`. ~48 PR-фрагментов.

**Acceptance:**
- На любом handler с UUID-параметром: `curl /portal/v1/messages/not-a-uuid` → 400 INVALID_INPUT, не 500.
- `grep -rE "uuid\.Parse\(.*mux\.Vars" internal/gateway/ --include="*.go" | wc -l` → 0.

**Через:** Может быть подхвачен subagent'ом параллельно (`Agent superpowers:executing-plans` per-file). Объём ~3-4 рабочих дня.

### C.2. pgx error→500 (BUG-9/13/17/28/50/51 sweep)

**Pre-flight:** Найти все callsites `repo.Get*` где `errors.Is(err, pgx.ErrNoRows)` не обрабатывается → возвращается 500. ≥12 файлов накопилось.

**Действие:**
1. Утилита `internal/shared/dberr/wrap.go::WrapNotFound(err error, entity string) error` — мапит `pgx.ErrNoRows` → `shared.ErrNotFound(entity + " не найден")`, всё остальное → как есть (или wrap'ит как InternalServer).
2. Sweep по storage repo'ям + service-handler'ам: каждый `if err != nil { return err }` после `pool.QueryRow(...).Scan(...)` → `if err != nil { return dberr.WrapNotFound(err, "сообщение") }`.
3. Документировать паттерн в CLAUDE.md или `internal/shared/dberr/README.md`.

**Acceptance:**
- GET /portal/v1/messages/{несуществующий-uuid} → 404, не 500.
- `grep -rE "pgx\.ErrNoRows" internal/storage/ internal/services/` shows wrap pattern везде.

**Через:** `/execute-with-review`. По одному репозиторию = один коммит. ~10 PR-фрагментов.

### C.3. http.Error plain-text (BUG-66 sweep)

**Pre-flight:** `grep -rn "http\.Error(" internal/gateway/portal/ --include="*.go"` → 5+ остатков (api_keys, notifications, SSE StreamMessages).

**Действие:** Заменить каждый `http.Error(w, "msg", code)` → `respondError(w, shared.ErrXXX("msg"))`. Через `/execute-with-review`. ~5 файлов.

**Acceptance:** `grep -rn "http\.Error(" internal/gateway/portal/ --include="*.go"` → 0 (только stub-handler'ы и health-check).

### C.4. Дублирующие handler-level checkReseller (cleanup-PR)

**Pre-flight:** ResellerOnlyMiddleware (этап 27) уже защищает /reseller/*. Handler-level `checkReseller` в 7 файлах — мёртвый код:
- `internal/gateway/portal/handlers/reseller_dashboard.go`
- `reseller_routing.go`
- `reseller_tariffs.go`
- `reseller_moderation.go`
- `reseller_sender_names.go`
- `reseller_templates.go`
- `reseller_analytics.go`

**Действие:** Удалить `checkReseller` calls из этих 7 файлов; функцию `checkReseller` оставить как private utility если на ней есть тесты, иначе удалить. Проверить что ResellerOnlyMiddleware точно покрывает все routes этих handler'ов в router.go.

**Acceptance:** `grep -rn "checkReseller" internal/gateway/portal/handlers/` → 0 callsites (или только в одном файле как private utility).

**Через:** `/execute-with-review`. Один PR.

### C.5. 401 → 403 semantic для /reseller/* (этап 27 obs-4)

**Pre-flight:** ResellerOnlyMiddleware (`reseller_only.go:51`) возвращает `shared.ErrUnauthorized` (401). Семантически правильно `403 Forbidden` для авторизованного-но-без-прав. Но frontend может редиректить на /login при 401 (нужно проверить!).

**Действие:**
1. Проверить frontend handler 401: `grep -rn "401\|status === 401" portal-frontend/src/api/`
2. Если есть глобальный 401-redirect: добавить exception для path `/reseller/*` ИЛИ проверить отдельно 403 без redirect.
3. Изменить `reseller_only.go::ResellerOnlyMiddleware` → `shared.ErrForbidden("доступ только для агрегаторов")`.
4. Сохранить exact текст сообщения (тесты на него опираются).

**Acceptance:** TC-RBAC-5/6 (этап 28): user/sub → /reseller/dashboard → 403 (не 401). Frontend не редиректит на /login. UX корректен.

**Через:** `/execute-with-review`. Маленький PR.

### C.6. validation→500 (BUG-A bulk fix) — [DONE] 2026-05-02 (commit `6a2380a`)

Скоуп оказался уже плана: только cascade/grpc/handler.go (16 callsites). billing/tarification gRPC — чисты. internal/pipeline/ — Kafka-стадии, не gRPC, errors → DLQ (план ошибочно их упомянул). Подробности: `docs/audit-followups-progress.md`.



**Pre-flight:** В pipeline/billing/tarification/cascade gRPC-server'ах — некоторые методы возвращают `fmt.Errorf("validation failed: %w", err)` без `codes.InvalidArgument` mapping → клиент получает 500. Этап 23 obs-4 содержит список (5+ методов в cascade).

**Действие:**
1. Аудит: `grep -rn "fmt\.Errorf.*validation" internal/services/cascade/ internal/services/pipeline/ internal/services/billing/ internal/services/tarification/`.
2. Для каждого callsite: завернуть в `status.Errorf(codes.InvalidArgument, ...)`.
3. Существующий handler `respondGRPCError(w, err)` уже мапит InvalidArgument → 400, НотFound → 404 и т.д.

**Acceptance:** TC где invalid input на cascade/billing/pipeline gRPC-API → 400, не 500.

**Через:** `/execute-with-review`. Один PR на сервис.

### C.7. errors.Is sweep (этап 25 obs-4)

**Pre-flight:** Несколько мест используют `==` для сравнения с sentinel errors вместо `errors.Is`. Этап 25 obs-4: например `UpdateAPIKey err == ErrXXX` вместо `errors.Is(err, ErrXXX)`.

**Действие:** `grep -rE "err == [A-Z]\w+Error|err == Err\w+" internal/` → fix всем callsites. Один PR.

### C.8. 403 vs 404 info-disclosure (этап 23 obs-1, этап 28 obs-1)

**Pre-flight:** На некоторых cross-tenant resource accesses сейчас 403 раскрывает существование ресурса, на других 404 — скрывает. Несогласованно. Примеры:
- `/portal/v1/api-keys/{cross-tenant-id}` PUT/DELETE → 403 "API key does not belong to user"; GET → 404. ОБS этапа 28.
- `/portal/v1/messages/{cross-tenant-id}` → 403 "access denied" (этап 23). 

**Решение:** Унифицировать на 404 для всех cross-tenant попыток. В internal/services/auth/grpc/server.go::RevokeAPIKey, UpdateAPIKey — изменить ErrForbidden → ErrNotFound. Аналогично messages. Проверить что не ломает существующие 403-handler'ы (frontend ожидание).

**Через:** `/execute-with-review`. Маленький PR.

### C.9. DNS-rebinding bypass в webhook ValidateURL (этап 25 obs-1)

**Pre-flight:** `internal/services/webhook/domain/validation.go::ValidateURL` валидирует hostname через blocklist (localhost, 127.0.0.1, internal IPs), но DNS resolution может выдавать разные ответы при validate vs delivery (DNS rebinding). Атака: домен резолвится в публичный IP при validate → в 169.254.169.254 при delivery (cloud metadata).

**Действие:**
1. Resolve URL hostname в IP при validate → сохранять resolved IP в `webhook_subscriptions.resolved_ip`.
2. При delivery: подключаться к saved IP (override DNS).
3. Альтернатива: pin-DNS для каждой webhook delivery, проверять что resolved IP == saved IP.

**Через:** `/execute-with-review`. Объём средний (~3-4 файла).

### C.10. jsonb []byte serialization (BUG-16 sweep)

**Pre-flight:** В 7+ файлов `pool.QueryRow(...).Scan(&jsonbField)` где jsonbField — `[]byte`, потом `json.Unmarshal(jsonbField, &target)`. Должно быть `pgtype.JSONB` или `*json.RawMessage`.

**Действие:** sweep + unify. По файлу = один коммит.

---

## 5. BLOCK D — открытые observations (1-2 PR)

### D.1. /admin/v1/clients silent-ignore is_reseller/max_sub_accounts (этап 28 obs-6) — [DONE] 2026-05-02 (commit `4092250`)

Закрыто после X3 разблокировки proto regen. proto CreateClientRequest получил plain `bool/int32`, UpdateClientRequest — proto3 `optional` (pointer-style nil = не менять). Application+gRPC+admin handler+JSON DTO+ClientInfo response — все слои пробрасывают. Plus business rule `is_reseller=true ⇒ max_sub_accounts >= 1` (БД CHECK не ловит). Подробности: `docs/audit-followups-progress.md`.

### D.2. /admin denial silent-redirect без toast (этап 29 obs-2)

`portal-frontend/src/components/RequireRole.tsx:20` + `RequireReseller.tsx:14` — silent Navigate. Действие: добавить `useToast().error("Доступ запрещён: требуется роль admin")` перед Navigate. Один PR на 2 файла.

### D.3. Catch-all → /command-center без 404 страницы (этап 29 obs-1)

`App.tsx:251`. Действие: создать `pages/NotFoundPage.tsx` (RU-text, кнопка "На главную"), заменить `<Navigate to="/command-center" replace />` на `<NotFoundPage />`. Один PR.

### D.4. RequireAuth/RequireRole/RequireReseller loading inconsistency (этап 29 obs-4)

"Loading..." vs "Загрузка...". Действие: унифицировать на "Загрузка...". 3 файла, один PR.

### D.5. ErrorBoundary без useTranslation (этап 29 obs-3)

`ErrorBoundary.tsx:24-30` hardcoded RU. Действие: импорт `useTranslation`, ключи в ru.json `errors.boundary.*`. Один PR.

### D.6. BUG-80 TOCTOU race + UNIQUE INDEX миграция (этап 26 obs-1)

CreateSubAccount email uniqueness check + INSERT не атомарно. Action: добавить UNIQUE INDEX `(parent_client_id, lower(email)) WHERE active=true`. **Стоп — БД миграция, эскалация по §5.4.**

### D.7. PUT /portal/profile silent-ignore email/name (этап 27 obs-2)

Действие: вернуть 400 "поля email/name недоступны". Один PR.

### D.8. /portal/cascade-history endpoint не зарегистрирован (этап 27 obs-5)

UI ходит на `/portal/v1/cascade-history` → 404. Frontend нужно править на правильный endpoint (`/cascade/deliveries`?) ИЛИ зарегистрировать alias. Investigation + PR.

### D.9. GET /portal/v1/webhooks/{id} не зарегистрирован (этап 25 obs-5)

Sub → 404 plain-text. Действие: зарегистрировать handler если он есть в коде; иначе создать минимальный. Один PR.

### D.10. RotateAPIKey endpoint отсутствует (этап 25 obs-6), UpdateAPIKey errors.Is (obs-4), API-key audit ClientID="" (obs-3), Webhook signature replay не тестировалось (obs-7)

Bundle PR на api-keys/webhooks: 4 минификса.

---

## 6. Порядок исполнения

```
1. BLOCK A.1 (BUG-83)         — security CRITICAL, без него аудит не закрыт
2. BLOCK B.1 (partner_id)     — стоп, эскалация решения (вариант 1/2/3) → отдельный spec
3. BLOCK B.2 (i18n)           — стоп, эскалация решения roadmap'a
4. BLOCK B.3 (E2E seed)       — после A.1, до накопительных
5. BLOCK D (10 small PR'ов)   — параллельно с C, по одному в день
6. BLOCK C (накопительные)    — самый объёмный, делать batch'ами
   — C.1 uuid.Parse: subagent-driven (parallel agents per-file)
   — C.2 pgx wrap: linearly per-repo
   — C.3 http.Error: один PR
   — C.4 dup checkReseller cleanup: один PR
   — C.5 401→403: один PR (после verify frontend)
   — C.6 validation→500: один PR на сервис
   — C.7 errors.Is: один PR
   — C.8 403/404 unify: один PR
   — C.9 DNS-rebinding: один PR
   — C.10 jsonb []byte: sweep
```

**Lock-механика (как в аудите):** перед началом каждой задачи — `[IN_PROGRESS]` заголовок в новом файле `docs/audit-followups-progress.md`. После закрытия — `[DONE] task X.Y — N коммитов`.

---

## 7. Эскалации перед началом

В новом чате ДО открытия задачи попросить пользователя:
1. **BUG-83 (A.1)**: подтвердить вариант (a) полная реализация scope enforcement — ОК?
2. **partner_id (B.1)**: выбрать вариант 1 / 2 / 3.
3. **i18n (B.2)**: EN-портал в roadmap?  WONTFIX или открыть отдельный spec?
4. **D.6 (BUG-80 UNIQUE INDEX)**: миграция БД — применять?

---

## 8. Критерии успеха

- BLOCK A.1 (BUG-83) закрыт, TC-SCOPE-1c → 403.
- BLOCK B.1 (partner_id) — либо закрыт фиксом, либо явный spec открыт + временная защита установлена.
- BLOCK B.2 (i18n) — решение зафиксировано в `MEMORY.md` (WONTFIX или открыт spec).
- BLOCK B.3 (E2E) — `psql < demo_seed.sql` + POST /messages → status=delivered через ≤10s.
- BLOCK C.1-C.10 — каждый паттерн помечен в CLAUDE.md как RESOLVED или явно RECURRING (если >5 PR ушло в backlog).
- BLOCK D.1-D.10 — все 10 PR закрыты или явно ROLLED-BACK с причиной.
- 0 строк `[IN_PROGRESS]` в `docs/audit-followups-progress.md`.
- Final commit: `docs(followups): complete — all P1/P2 audit items closed`.

---

## 9. Открытые вопросы (заполняется в ходе)

- *(пусто на старте; заполняется при эскалациях)*
