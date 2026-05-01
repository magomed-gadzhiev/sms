# UX Audit Progress

> Активный план аудита: [docs/superpowers/specs/2026-04-29-ux-full-reaudit-design.md](../superpowers/specs/2026-04-29-ux-full-reaudit-design.md). Скоуп D: 30 этапов, fix mode + Infrastructure Check + QA full.

## [DONE] Этап 27/30: Subaccount — портал под parent (subaccount, fix + Infrastructure + QA full, 2026-05-01) — частичный (1 баг исправлено: 1 CRITICAL)

[Summary] 20+ TC прогнаны через API+SQL под subaccount-сессией: TC-0 password mechanism (см. OBSERVATION-1) — admin-only workaround (POST /admin/v1/users + PUT activate=true) сработал, аудит продолжен; TC-1 GET /portal/dashboard под sub → balance="100", не parent's 49700 PASS; TC-2 GET /portal/profile → is_reseller=false, parent_client_id=Reseller-id PASS; TC-3 GET /portal/billing/balance → собственный 100 PASS; TC-4 sub POST /sub-accounts (создать суб-суб) → 401 "client is not a reseller" PASS; TC-5 sub GET /sub-accounts (list children) → 401 PASS; TC-6 sub GET /reseller/dashboard → 401 PASS; TC-7 sub GET /reseller/sender-names → 401 PASS; TC-8 sub GET /reseller/templates → 401 PASS; **TC-9 sub GET /reseller/statistics?period_preset=7d → 200 + KPI parent (revenue/profit/cost) [BUG-82 CRITICAL]**; **TC-9c sub GET /reseller/analytics-summary → 200 [BUG-82]**; **TC-9d sub GET /reseller/monitoring → 200 [BUG-82]**; **TC-9e sub GET /reseller/drilldown → 200 + summary parent [BUG-82]**; **TC-9f sub GET /reseller/views → 200 + 3 templates [BUG-82]**; **TC-9n sub POST /reseller/export → 200 + job_id (write-action!) [BUG-82]**; TC-9b/h/i/j/k/l (analytics, op-registrations, routing/routes, tariffs, /network/tariffs, /network/tariff-templates) → 401 PASS (handler-level checkReseller отрабатывает); TC-10 GET /portal/messages → []; TC-11 sender-names empty; TC-12 templates empty; TC-13 api-keys empty; TC-14 webhooks empty; TC-15 notifications empty PASS; TC-16 GET /portal/billing/transactions → 1 строка transfer_in (initial balance) PASS; TC-17 sub GET /portal/messages/{reseller-msg-id} (cross-tenant message) → 403 access denied PASS; TC-18 sub POST /sub-accounts/{self-id}/transfer (self-loop) → 404 PASS; TC-18b sub POST /sub-accounts/{parent-id}/transfer (steal from parent) → 404 PASS; TC-19 sub PUT /portal/profile с email="admin@example.com" → 200 "обновлено", но БД не изменилась + admin login по-прежнему работает (silent ignore email/name полей — OBSERVATION-2); TC-20 sub POST /auth/logout → 200 + re-login PASS.

1 CRITICAL (BUG-82) исправлен через /execute-with-review (commit f668229, 1 review-цикл, code-reviewer APPROVED с 2 non-blocking замечаниями). Re-test всех 6 уязвимых endpoints под sub → 401, регрессия Demo-Reseller на /reseller/statistics + /reseller/dashboard → 200 PASS.

[BUG LIST]

BUG-82: 10 endpoints под /reseller/* (network-statistics handler-группа) leak parent's KPI и принимают cross-tenant write-action — Severity: CRITICAL — Категория: Security / Authorization bypass / Cross-tenant data leak + write
  Шаги: sub-account под Demo-Reseller (login через admin-workaround) делает GET /portal/v1/reseller/statistics?period_preset=7d, /reseller/analytics-summary, /reseller/monitoring, /reseller/drilldown, /reseller/views, POST /reseller/export
  Ожидалось: 401 UNAUTHORIZED "доступ только для агрегаторов" (по аналогии с reseller_dashboard.go::checkReseller, reseller_routing.go, reseller_tariffs.go и т.д., которые корректно гардят).
  Получалось: 200 + ответы со статистикой parent (kpis, rows[], summary[], previous_kpis, trends), 3 saved-view templates и POST /export → 200 + job_id (write-action прошёл от чужого имени)
  Корень: `internal/gateway/portal/handlers/network_statistics.go` — все 10 хендлеров (GetStatistics, GetAnalytics, GetMonitoring, GetDrillDown, StartExport, GetExportStatus, DownloadExport, ListViews, SaveView, DeleteView) проверяли только `middleware.GetClientID` + `h.checkClient` (наличие gRPC-клиента), но НЕ проверяли `clients.is_reseller=true`. Эта группа handler'ов — единственная в /reseller/ subrouter, которая пропустила handler-level checkReseller. Архитектурная амплификация (отдельная — OBSERVATION-3): partner_id берётся из query (int64), default=0; никакого mapping client_id (UUID) ↔ partner_id (int64) нет; partner_id=0 это глобальный агрегированный bucket.
  Доказательство: TC-9/9c/9d/9e/9f/9n; SQL `SELECT partner_id, count(*) FROM network_stats_hourly GROUP BY 1` → 1 row с partner_id=0 на dev стенде (на проде содержимое sensitive).
  Импакт: 1) leak revenue/profit/cost/sender_name/operator-mix всех клиентов network_stats_hourly к sub-account и обычному user; 2) cross-tenant write-action (POST /export job, POST /views, DELETE /views/{id}); 3) на проде в network_stats_hourly могут быть реальные финансовые данные всех агрегаторов network'а — leak любому авторизованному portal-клиенту.
  Фикс: f668229 — новый файл `internal/gateway/portal/middleware/reseller_only.go::ResellerOnlyMiddleware(pool)`; `reseller := protected.PathPrefix("/reseller").Subrouter(); reseller.Use(middleware.ResellerOnlyMiddleware(dbPool))` в router.go:445-452. Middleware делает `SELECT is_reseller FROM clients WHERE id=$1`, при false/error → 401 "доступ только для агрегаторов". Защищает все 10 endpoints + от регрессии для будущих /reseller/* routes. Fail-closed на nil pool (graceful 500 вместо panic-recovery → 500). Двойная проверка с уже-защищёнными handler-level checkReseller безвредна — middleware возвращает 401 первым.

[OBSERVATION-1] Sub-account password mechanism — feature gap, требует admin-workaround — Severity: HIGH — Категория: Onboarding / UX
  Подтверждение OBSERVATION-6 этапа 26: aggregator-side endpoint для выдачи credentials суб-аккаунту НЕ существует. POST /portal/v1/sub-accounts создаёт только `clients` row + `client_configs`, никогда не создаёт `users` row. Чтобы дать sub-account login, требуется 3-step admin-workaround: 1) aggregator POST /sub-accounts → sub created без login; 2) admin POST /admin/v1/users c {role_id, client_id=<sub-id>, password} → user inactive=false; 3) admin PUT /admin/v1/users/{uid} {"active":true} → activated. Архитектурная feature gap: реселлер не может онбордить своих клиентов без вмешательства superadmin'а. Кандидат на отдельную инициативу (welcome-email-flow / aggregator endpoint POST /sub-accounts/{id}/credentials).

[OBSERVATION-2] PUT /portal/profile silent-ignore полей `email` и `name` — Severity: COSMETIC — Категория: API contract clarity / UX
  TC-19: PUT /portal/profile с body `{"email":"admin@example.com","name":"Pwned"}` → 200 "Профиль успешно обновлён", но в БД ничего не изменилось (handler парсит только `contact_person`/`phone` через `updateProfileRequest`; см. profile.go:170-183). admin@example.com login продолжает работать. Не security-баг (поле игнорируется), но misleading UI: "обновлено" даже когда фактически обновлять нечего. Лёгкий fix: 400 "поля email/name недоступны через этот endpoint" или silent acceptance с прозрачным ответом "ничего не изменено". Bundle с другими "silent ignore" cleanup'ами.

[OBSERVATION-3] network-analytics partner_id архитектурная дыра — Severity: HIGH — Категория: Architecture / Cross-tenant
  `internal/gateway/portal/handlers/network_statistics.go:137-144` partnerIDFromRequest читает partner_id из query (int64), при отсутствии → 0. Никакого mapping client_id (UUID) ↔ partner_id (int64) в коде нет. Bucket partner_id=0 это глобальный агрегированный view. После фикса BUG-82 sub-account и обычный user НЕ могут попасть на /reseller/* (middleware блокирует). Но reseller A теперь может передать `?partner_id=<reseller-B-int64>` (если знает значение) и получить статистику reseller B — текущий guard это не закрывает (он проверяет is_reseller, не filter-by-self). Архитектурный фикс: либо mapping (UUID hash → partner_id deterministic), либо передать client_id в gRPC и фильтровать там, либо хранить UUID-партнёр в network_stats_hourly. Требует решения по data-modeling в network-analytics-service. Эскалирован отдельно.

[OBSERVATION-4] Middleware возвращает 401 вместо 403 для авторизованного-но-без-прав — Severity: COSMETIC — Категория: HTTP semantic / Frontend resilience
  Из review iteration 1 (code-reviewer): semantic правильно 403 Forbidden ("ты есть, но нет прав"), а не 401 Unauthorized. ResellerOnlyMiddleware использует shared.ErrUnauthorized → 401. ВСЕ существующие handler-level checkReseller (reseller_dashboard.go, reseller_routing.go, reseller_tariffs.go, reseller_moderation.go) тоже отдают 401 — менять только middleware создаст рассинхрон. Bundle-fix всех /reseller/* checkReseller с 401→403 + проверка frontend-обработки 403 — отдельный i18n/HTTP-semantic PR.

[OBSERVATION-5] GET /portal/cascade-history → 404 (route не зарегистрирован) — Severity: COSMETIC — Категория: Feature gap
  TC-17b показал что endpoint `/portal/v1/cascade-history` отсутствует в router'е (gorilla default 404 plain-text). Frontend, вероятно, использует `/portal/v1/messages?has_cascade=true` или другой filter. Не блокер, но REST API-contract drift с UI scope этапа 27.

[OBSERVATION-6] Sub-account inherits is_sandbox=false (NOT parent) — Severity: НЕ ПРОВЕРЕНО — Категория: Configuration inheritance
  GET /portal/profile под sub: `is_sandbox=false`. TC-2 не выявил негативный эффект, но если parent-aggregator является sandbox-клиентом, sub-account автоматически выходит из sandbox-режима — это может вести к попадание тестового трафика в реальный pipeline. Не воспроизведено, кандидат на расширение тестов когда стенд получит sandbox-aggregator.

[Success Path]
Aggregator создаёт суб-аккаунт через POST /sub-accounts (initial_balance переводится атомарно с parent). Admin вручную создаёт user через /admin/v1/users + activate (workaround OBSERVATION-1). Sub-account логинится → portal_session — видит /portal/dashboard со своим balance, /portal/profile с parent_client_id, /portal/billing/balance, /portal/messages /portal/sender-names /portal/templates /portal/api-keys /portal/webhooks /portal/notifications (все пустые в свежем sub). Cross-tenant попытки: /portal/messages/{reseller-msg-id} → 403; POST /sub-accounts/{any-id}/transfer → 404; POST /sub-accounts (create grandchild) → 401 "client is not a reseller"; GET /sub-accounts list → 401. /reseller/* и /network/* → 401 "доступ только для агрегаторов" (после BUG-82 фикса все 10 endpoints в network-statistics группе тоже закрыты middleware'ом). Logout → portal_session destroyed.

[Recommendations]
1. **OBSERVATION-1 sub-account credentials onboarding** (HIGH) — архитектурное решение требует продакт-политики: welcome-email с временным паролем или отдельный aggregator endpoint POST /sub-accounts/{id}/credentials. Без этого реселлер не может self-сервисно онбордить клиентов. Эскалация.
2. **OBSERVATION-3 network-analytics partner_id mapping** (HIGH) — middleware закрыл sub-account leak, но между reseller'ами (cross-aggregator через знание partner_id) дыра открыта. Архитектурный refactor mapping UUID ↔ int64 или фильтрация по client_id в gRPC. Эскалация.
3. **Cleanup-PR**: 401→403 для всех /reseller/* checks (OBSERVATION-4) + удаление дублирующих handler-level checkReseller в 4 файлах после ResellerOnlyMiddleware. Сначала проверить frontend на правильную реакцию 403 (не редирект на /login).

[Test Data]
- Создавалось/удалялось: sub-account `AuditSub27` (id `28dc0a1d-…`) под Demo-Reseller через POST /sub-accounts с initial_balance=100; user `auditsub27@demo.local` (id `c6d0e49a-…`, role_id=client) через admin POST /admin/v1/users с client_id=AuditSub27 + PUT activate=true. Все cleanup'нуты (DELETE из users, client_configs, transactions, accounts, clients) — `remaining_subs=0`.
- Demo-Reseller balance: до — 49800 (после этапа 26 cleanup), после initial_balance transfer — 49700, после cleanup sub — 49700 (transaction transfer_in осталась удалённой вместе с sub-account'ом, balance reseller'а не возвращён). Pre-existing seed state не трогали.
- Коммиты: 0d5da3a lock, f668229 fix BUG-82 (ResellerOnlyMiddleware + регистрация на /reseller/* subrouter), ниже close-коммит.

## [DONE] Этап 26/30: Aggregator — /network/* + sub-accounts (aggregator, fix + Infrastructure + QA full, 2026-05-01) — частичный (3 бага исправлено: 3 HIGH)

[Summary] 30+ TC прогнаны через API+SQL: TC-1 list happy PASS; TC-2 cross-aggregator GET /sub-accounts/{other-sub-id} → 404 PASS; TC-3 invalid UUID → 400 PASS; TC-4 parent_client_id-injection в body → silently ignored, sub-account создан под self PASS; TC-5 happy CreateSubAccount + initial_balance transfer PASS; TC-6 max_sub_accounts: лимит 50, не достигли — пропущено; **TC-7 duplicate email POST → 201 [BUG-80]**; TC-7b empty email → 201 PASS (back-compat); **TC-7c invalid email "notanemail" → 201 [BUG-80]**; TC-8 happy balance transfer PASS; TC-9 negative amount → 400 PASS; TC-9b zero → 400 PASS; TC-9c NaN → 400 PASS; TC-10 amount > parent balance → 400 "insufficient balance" PASS; TC-11 cross-aggregator topup → 404 PASS; TC-CR-1..5 cross-aggregator IDOR (messages/analytics/api-keys/UpdateLimits/DELETE) → все 404 PASS; TC-CR-6/7 invalid UUID на /transfer и /limits → 400 PASS; TC-D1 reseller dashboard PASS; TC-D2..D5 sender-names/templates/moderation/op-registrations PASS; **TC-D7 GET /reseller/routing/routes → 500 "ERROR: column cr.country_id does not exist (SQLSTATE 42703)" [BUG-81]**; TC-D8 tariff-overview без sub_account_id → 400 PASS; TC-D11 tariff-overview happy PASS; TC-D14/15 invalid UUID на /reseller/sender-names/{x}/approve и /templates/{x}/approve → 404 "не найдено" вместо 400 (OBSERVATION pattern BUG-64); TC-D17 /network/tariffs/subaccounts-summary PASS; TC-D19 bulk-assign cross-tenant filter PASS; TC-12 transactions PASS; TC-12b/12c messages/analytics PASS; TC-13a/b/c api-keys/webhooks/campaigns sub-resource PASS; TC-L1 negative limits → 400 PASS; TC-L2 happy update → 200 PASS; **TC-L3 GET /sub-accounts (LIST) — daily_limit=0, monthly_limit=0 хотя в БД лимиты выставлены [BUG-79]**; TC-Q1/Q2 quota PASS (null OK для seed без plan); TC-N1..N3 analytics-summary/monitoring/drilldown PASS (drilldown требует slice_type — корректная валидация); TC-S1/S2 dashboard period=invalid и statistics date_from=not-a-date silent default (OBSERVATION).

3 HIGH (BUG-79/80/81) исправлены одним PR через /execute-with-review (commit 59d4c37, 2 review-цикла, code-reviewer APPROVED после iter2 с email-нормализацией). Re-test всех 3 фиксов + регрессия PASS.

[BUG LIST]

BUG-79: GET /sub-accounts (LIST) и GET /sub-accounts/{id} возвращают daily_limit=0, monthly_limit=0 несмотря на установленные лимиты — Severity: HIGH — Категория: Functional / Data display
  Шаги: c2 POST /sub-accounts {"name":"X","daily_limit":1000,"monthly_limit":10000} → 201 OK. Затем GET /sub-accounts/{id} → daily_limit:0, monthly_limit:0.
  Ожидалось: daily_limit:1000, monthly_limit:10000.
  Получалось: 0/0 для всех sub-accounts во всех ответах LIST/GET. БД при этом содержит rate_limit_per_day=1000 и settings={"monthly_limit":"10000"} в client_configs.
  Корень: SubAccountRepository.ListByParentID/GetSubAccount делали SELECT только из `clients`, не из `client_configs` (отдельная таблица). domain.Client.Config оставался nil. internal/services/client/grpc/server.go::domainClientToSubAccount читает client.Config.RateLimitPerDay только если Config != nil → возвращал 0. UpdateLimits возвращает корректное значение, потому что свежесозданный protobuf формирует SubAccount с непустым Config.
  Доказательство: TC-L3 + SQL запрос `SELECT cc.client_id, cc.rate_limit_per_day, cc.settings FROM client_configs cc` показывает реальные значения.
  Импакт: пользователь устанавливает лимиты при создании суб-аккаунта (или через PUT /limits), затем UI всегда показывает 0/0 и не может проверить, что лимиты применены. Под-капотом лимиты в БД присутствуют и применяются на pipeline-уровне, но UX-обманка серьёзная: реселлер думает, что суб-аккаунт без ограничений.
  Фикс: 59d4c37 — LEFT JOIN с client_configs в обоих SELECT (ListByParentID + GetSubAccount), scan rate_limit_per_day и settings (jsonb) → заполнение domain.Client.Config двумя полями, нужными mapper'у. sql.NullInt64/NullString для NULL-safety.

BUG-80: CreateSubAccount принимает невалидный/дубликатный email — Severity: HIGH — Категория: Functional / Data integrity
  Шаги:
    a) c2 POST /sub-accounts {"name":"X","email":"notanemail"} → 201 (email "notanemail" сохранён в БД).
    b) c2 POST /sub-accounts {"name":"X","email":"subby1@demo.local"}, c2 POST {"name":"Y","email":"subby1@demo.local"} → оба 201.
  Ожидалось: a) 400 INVALID_INPUT "invalid email format"; b) второй POST → 409 CONFLICT.
  Получалось: оба сценария принимаются без проверок.
  Корень: handler validates только `name != ""`. SubAccountService не имел email regex и не имел uniqueness check; в БД нет UNIQUE constraint на (parent_client_id, email).
  Доказательство: TC-7/7c — 4 sub-accounts с одинаковым email под одним parent в БД.
  Импакт: 1) email используется для login суб-аккаунта (этап 27 проверка) → ambiguity; 2) email-уведомления (low-balance, security alerts) уйдут на одного из дубликатов случайно; 3) "notanemail" → undefined behaviour любого SMTP-клиента.
  Фикс: 59d4c37 — emailRegex `^[^@\s]+@[^@\s]+\.[^@\s]+$`, ExistsByEmailUnderParent (lower(email) match без active=true filter — soft-deleted "бронирует" слот, иначе delete+recreate даёт 2 строки), trim+lowercase нормализация email до dup-check и до записи в БД (mixed-case "Foo@x" и "foo@x" больше не создают ambiguity). ErrInvalidEmail → codes.InvalidArgument → 400; ErrEmailExists → codes.AlreadyExists → 409.
  TOCTOU race открыт (см. OBSERVATION-1).

BUG-81: GET /portal/v1/reseller/routing/routes → 500 INTERNAL_ERROR на любом запросе — Severity: HIGH — Категория: Functional / SQL schema drift
  Шаги: c2 GET /portal/v1/reseller/routing/routes (без параметров) → 500.
  Ожидалось: 200 + список маршрутов (или []).
  Получалось: 500 INTERNAL_ERROR. portal-gateway лог: `ERROR: column cr.country_id does not exist (SQLSTATE 42703)`.
  Корень: handler SELECT `cr.country_id FROM client_routes cr`, но столбец `country_id` никогда не существовал в client_routes (см. migration 000038); признак страны живёт в `route_condition_groups`. Frontend NetworkRoutingPage.tsx ожидает `country_id: string | null` и рендерит '—' при null.
  Доказательство: TC-D7 + лог + `\d client_routes`.
  Импакт: страница /portal/network/routing полностью сломана для всех агрегаторов, т.к. это GET-запрос без параметров и любой aggregator со 0+ subaccounts и 0+ routes ловит 500. NetworkRoutingPage.tsx рендерит ошибочное состояние.
  Фикс: 59d4c37 — `NULL::uuid AS country_id` в SELECT (backward-compat для frontend, который рендерит '—' при null). Honest JOIN с route_condition_groups → отдельная инициатива (см. OBSERVATION-2: clean JOIN, который выбирает одну страну на route, нетривиален — conditions[] разнородны: country/operator/sender_name/etc).

[OBSERVATION-1] BUG-80 TOCTOU race (UNIQUE INDEX миграция эскалирована) — Severity: MEDIUM — Категория: Concurrency / Data integrity
  Application-level dup-check имеет TOCTOU race: два параллельных POST с одинаковым email оба пройдут ExistsByEmailUnderParent и оба создадут sub-account. Реалистичная вероятность низкая (один оператор UI, один parent, один email, sub-millisecond окно), но это billing/identity surface. Правильное закрытие — partial UNIQUE INDEX `(parent_client_id, lower(email)) WHERE email IS NOT NULL`. Per spec §5.4 миграция БД требует эскалации пользователю — отдельный канал. Перед применением: pre-cleanup существующих дубликатов через `SELECT parent_client_id, lower(email), count(*) FROM clients WHERE parent_client_id IS NOT NULL AND email <> '' GROUP BY 1,2 HAVING count(*)>1`.

[OBSERVATION-2] BUG-81 country_id "always NULL" — Severity: LOW — Категория: Feature gap (not regression)
  После фикса GET /reseller/routing/routes возвращает 200 с `country_id: null` для всех routes — страница /portal/network/routing показывает '—' в колонке "Страна" вместо реальных значений. Это не регрессия (раньше было 500, страница вообще не рендерилась), но silent feature absence. Чтобы вернуть страну, нужен JOIN с route_condition_groups + выбор одной страны на route — нетривиально, поскольку conditions[] могут быть разнородные (country/operator/sender_name/etc). Кандидат на отдельный stage с дизайном.

[OBSERVATION-3] BUG-64 pattern в reseller-moderation — Severity: COSMETIC — Категория: BUG-64 invalid-uuid-handler-precheck pattern
  TC-D14/D15: /reseller/sender-names/not-a-uuid/approve и /reseller/templates/not-a-uuid/approve → 404 "не найдено" вместо 400 INVALID_INPUT. Pre-existing pattern с 434 вхождениями в 48 файлах — отдельный систематический refactor (см. progress-файл, "Накопительные паттерны").

[OBSERVATION-4] /reseller/dashboard period=invalid silent default — Severity: COSMETIC — Категория: Validation
  TC-S1: GET /reseller/dashboard?period=invalid → 200 + period:"today". Без 400 на whitelist — fallback. Не блокер, но непоследовательно с /analytics, который на этапе 24 жёстко 400 на invalid period (whitelist 7d/30d/90d/365d/custom).

[OBSERVATION-5] /reseller/statistics date_from=not-a-date silent ignore — Severity: COSMETIC — Категория: Validation
  TC-S2: GET /reseller/statistics?date_from=not-a-date → 200 (ошибка проигнорирована). Аналог BUG-74 но другой endpoint. Bundle с network-stats cleanup-PR (5 раундов в прошлом — 6-й накопительный pattern).

[OBSERVATION-6] CreateSubAccount не возвращает password или способ логина — Severity: НЕ ПРОВЕРЕНО — Категория: UX / Onboarding
  Response create_sub_account: {id,name,email,daily_limit,monthly_limit,...}. password не возвращается, email не отправляется (наблюдение через docker logs). Sub-account созданный через API не имеет login-credentials до тех пор, пока какой-то отдельный mechanism не присвоит password (welcome-email? отдельный admin-flow?). Это блокер для этапа 27 (subaccount login). Эскалация архитектурной фичи: либо password generates на бэке + email-flow, либо sub-account password-set отдельный endpoint.

[OBSERVATION-7] BUG-67/68 cross-aggregator pattern для tariff/routing override — Severity: НЕ ПРОВЕРЕНО — Категория: Cross-aggregator IDOR
  Tariff override и routing override через /reseller/tariff-templates/{id}/assign и /reseller/routing/bulk-assign — TC-D19 показал ownership-check на bulk-assign (cross-tenant sub_account_id отбрасывается с error "субаккаунт не найден"). Аналогичный check на /reseller/tariff-templates/{id}/assign/{sub_account_id} НЕ ПРОВЕРЕН — данных нет (templates пустые на стенде). Кандидат на расширение тестов когда стенд получит tariff-data.

[OBSERVATION-8] Sub-account suspension не отключает SMPP-сессии и REST-токены — Severity: НЕ ПРОВЕРЕНО — Категория: Security / Session lifecycle
  preexisting risk из плана этапа 26. Не воспроизведено: на стенде sub-accounts не имели активных SMPP сессий или REST API-key. Кандидат на E2E этапа 30.

[OBSERVATION-9] Defense-in-depth: parent_client_id из body silently ignored — Severity: COSMETIC (positive but silent) — Категория: API contract clarity
  TC-4: POST /sub-accounts с {"parent_client_id":"<other-reseller-id>","..."} → 201 + sub-account создан под self (parent из контекста сессии). handler не валидирует body parent_client_id и не возвращает 400 при попытке его передать. Хорошо в плане безопасности (cross-tenant exploit невозможен), плохо в плане API-contract — пользователь думает что параметр работает, не знает что игнорируется. Лёгкий fix: 400 INVALID_INPUT при попытке передать parent_client_id.

[Success Path]
Aggregator (Demo-Reseller) логинится → /portal/network/dashboard показывает сводные KPI, баланс сети, проблемные суб-аккаунты, moderation counts. /sub-accounts LIST/CREATE: создаёт суб-аккаунт с initial_balance (атомарный transfer от parent), email/limits/active state. POST /sub-accounts/{id}/transfer списывает с parent, зачисляет на child, audit event. Cross-aggregator IDOR на все routes (GET single/list, PUT limits, POST transfer, DELETE, sub-resources messages/analytics/api-keys/webhooks/campaigns) → 404. /reseller/sender-names + /reseller/templates moderation queue. /reseller/routing/providers (бывал 500 → теперь 200), /reseller/routing/routes (was 500 → теперь 200, country=NULL). /network/tariffs/subaccounts-summary, /network/tariff-templates, /network/tariff-editor — 200 с пустыми массивами на чистом стенде. /reseller/statistics, /analytics, /analytics-summary, /monitoring — KPI + timeline + previous-period.

[Recommendations]
1. **Эскалация UNIQUE INDEX миграции** (OBSERVATION-1): partial UNIQUE INDEX на (parent_client_id, lower(email)) WHERE email IS NOT NULL — закрывает TOCTOU race в BUG-80. Нужен pre-cleanup query на дубликаты в проде. Спроси у пользователя готовность применять.
2. **OBSERVATION-6 sub-account password flow** — блокер этапа 27. Архитектурное решение: либо генерация password бэк + email-доставка, либо отдельный admin-endpoint set-password. Эскалация перед этапом 27.
3. **Network routing country_id JOIN** (OBSERVATION-2) — фича не работает на UI после BUG-81 фикса. Отдельный stage с дизайном.

[Test Data]
- Создавалось/удалялось: 2 sub-accounts Subby1/Subby2 под Demo-Reseller (через API), Other-Reseller (c0000000-...-099) + Other-Sub (c0000000-...-098) через SQL для cross-aggregator проверок. Всё удалено через `DELETE FROM clients WHERE name IN (...)` после smoke. Также промежуточные дубликаты iter1/iter2 (Pwned, Subby1Dup, NoEmail, BadEmail, EmptyEmail, NewSub, NormCase) — все удалены.
- Demo-Main inactive=f (preexisting state, не трогали).
- Балансы: Demo-Reseller начинал с 50000, после всех transfers и cleanup'а — состояние БД оставлено как есть (sub-accounts удалены, баланс reseller'а как был).
- Коммиты: 0470f36 lock, 59d4c37 fix BUG-79/80/81 (config load + email validation/normalization + cr.country_id NULL mask), ниже close-коммит.

## [DONE] Этап 25/30: User — api-keys + webhooks + notifications (user, fix + Infrastructure + QA full, 2026-05-01) — частичный (3 бага исправлено: 1 CRITICAL + 2 HIGH)

[Summary] 14 TC прогнаны через API+SQL: TC-1 c1 CreateAPIKey happy → 201 + plaintext key (one-shot) PASS; TC-2 c2 CreateAPIKey happy PASS; TC-3 plaintext в БД? — нет, key_hash=SHA-256/64 hex chars PASS; TC-4 c1 GET /api-keys/{c2 id} → 404 PASS (фильтрация через ListByUserID); TC-5 c1 PUT /api-keys/{c2 id} → 403 "not owned" PASS (UpdateAPIKey уже ownership-checked); **TC-6 c1 DELETE /api-keys/{c2 id} → 204 + БД active=false [BUG-76 CRITICAL IDOR]**; TC-7 invalid UUID на /api-keys/{x} DELETE → 400 PASS (auth-grpc проверяет); TC-W1 webhook https://localhost → 500 [BUG-77]; TC-W2/3/4 webhook 169.254/10.0/192.168 → 500 [тот же BUG-77]; TC-W5 webhook file://... → 400 PASS (scheme guard работает); TC-W6 webhook https://example.com — event_type=message.delivered → 400 (whitelist {delivered,failed,expired,rejected}) PASS; TC-W7 webhook example.com event_type=delivered → 201 + secret one-shot PASS; TC-W8 c1 cross-tenant DELETE c2 webhook → 404 PASS (webhook-service применяет client_id фильтр); TC-W9 webhook secret НЕ возвращается в list/get PASS; TC-N1 POST /notifications/not-a-uuid/read → 500 + raw "ERROR: invalid input syntax... (SQLSTATE 22P02)" в body [BUG-78].

3 бага (1 CRITICAL + 2 HIGH) исправлены одним PR через /execute-with-review (commit d709daf, 2 review-цикла, code-reviewer APPROVED). Re-test всех 7 smoke-сценариев (a–g) PASS.

[BUG LIST]

BUG-76: cross-user DELETE /portal/v1/api-keys/{id} (IDOR) — Severity: CRITICAL — Категория: Security / Authorization bypass / Privilege escalation
  Шаги: c1 (`d0000000-0000-0000-0000-000000000002`/Demo-Main) делает DELETE /portal/v1/api-keys/{c2-key-id} где c2-key — ключ Demo-Reseller (`d0000000-0000-0000-0000-000000000003`)
  Ожидалось: 403 FORBIDDEN, ключ остаётся active
  Получалось: 204 NoContent, БД `api_keys.active=false` для чужого ключа → SMPP/REST интеграции жертвы ломаются
  Корень: `RevokeAPIKeyRequest.proto` содержал только `api_key_id`. `AuthService.RevokeAPIKey(ctx, keyID)` напрямую вызывал `apiKeyRepo.Revoke(ctx, keyID)` БЕЗ ownership-проверки. portal-gateway::RevokeAPIKey хендлер брал keyID из URL и слал в gRPC без передачи UserId. Аналогичный UpdateAPIKey уже проверял `key.UserID != userID → ErrAPIKeyNotOwned → PermissionDenied`, но Revoke остался незакрытым.
  Доказательство: TC-6, см. в отчёте этапа выше.
  Импакт: любой авторизованный портальный аккаунт, узнавший UUID чужого API-key (через лог-лик/скрин/socialing/перехват), мгновенно DoS'ит интеграцию жертвы. UUID v4 не подбирается, но boundary-trust между tenants нарушен по дизайну. Эскалировано пользователю ПЕРЕД фиксом per spec §5.4.
  Фикс: d709daf — добавлено `string user_id = 2;` в `RevokeAPIKeyRequest`; `AuthService.RevokeAPIKey(ctx, keyID, userID)` GetByID → проверка `key.UserID != userID` → ErrAPIKeyNotOwned; gRPC server маппит в codes.PermissionDenied; portal-handler передаёт `UserId: userID.String()`. Тесты: новый "not owned returns ErrAPIKeyNotOwned" subtest с AssertNotCalled(t, "Revoke") + обновлены legacy 2-arg тесты до 3-arg сигнатуры (включая test/functional/auth_test.go).

BUG-77: webhook URL → 500 вместо 400 при private/internal IP (BUG-A pattern в webhook-service) — Severity: HIGH — Категория: Functional / Error mapping
  Шаги: POST /portal/v1/webhooks `{"url":"https://localhost:8081/x"}` (или 169.254.169.254, 10.0.0.1, 192.168.1.1, [::1])
  Ожидалось: 400 INVALID_INPUT с понятным сообщением
  Получалось: 500 INTERNAL_ERROR
  Корень: SSRF guard ВЫПОЛНЯЕТСЯ в `internal/services/webhook/infrastructure/http/delivery_client.go::ValidateURL` (literal-IP блокируется через net.ParseIP + IsPrivate/IsLoopback/IsLinkLocalUnicast — 169.254 покрывается). Но возвращался `fmt.Errorf("private/internal …")` вместо domain-ошибки → `mapError` switch fallthrough → codes.Internal → 500.
  Доказательство: webhook-service лог `private/internal IP addresses are not allowed` + curl выше.
  Импакт: SSRF фактически блокируется (good defense-in-depth), но user видит "Внутренняя ошибка сервера" вместо понятной 400. Жертва: literal-IP блок есть, **DNS-rebinding bypass open** — `https://attacker-controlled.com → 169.254.169.254` пройдёт при первом resolve и ударит IMDS на доставке. Архитектурный фикс (re-resolve+check-IP перед каждым HTTP) вынесен в OBSERVATION.
  Фикс: d709daf — добавлены `domain.ErrPrivateURL`, `domain.ErrURLTooLong`. ValidateURL возвращает их вместо fmt.Errorf-строк. mapError маппит обе в codes.InvalidArgument → 400.

BUG-78: POST /portal/v1/notifications/{bad-uuid}/read → 500 + raw PostgreSQL error в response body — Severity: HIGH — Категория: Functional / BUG-64 pattern + Information leak
  Шаги: POST /portal/v1/notifications/not-a-uuid/read с куки клиента
  Ожидалось: 400 INVALID_INPUT
  Получалось: 500 INTERNAL_ERROR + body `"ERROR: invalid input syntax for type uuid: \"not-a-uuid\" (SQLSTATE 22P02)"` — утечка SQLSTATE и схемы в публичный API
  Корень: `notifications.go::MarkNotificationRead` передавал raw string `id` в pgx `WHERE id = $1` без uuid.Parse pre-check. Pgx ::uuid cast-ошибка → `shared.ErrInternalServer(err.Error())` → `err.Error()` шёл прямо в JSON-response.
  Доказательство: TC-N1 вывод выше.
  Импакт: 1) BUG-64 pattern (валидация → 500); 2) info-disclosure (PG-internals в публичном API); 3) аналогичные `err.Error()` ливы в `GetNotifications` и `MarkAllNotificationsRead`.
  Фикс: d709daf — uuid.Parse pre-check в MarkNotificationRead → ErrInvalidInput. SQL получает `notifID uuid.UUID` (binary path, без cast). Все три обработчика (Get/MarkOne/MarkAll) очистили leak: `shared.ErrInternalServer("Ошибка ...")` без `err.Error()` (zerolog log пишет err внутрь, наружу не льёт).

[OBSERVATION-1] webhook-service: DNS-rebinding bypass в ValidateURL — Severity: MEDIUM — Категория: SSRF defense-in-depth
  ValidateURL делает `net.ParseIP(host)` — для hostname возвращает nil → IP-checks пропускаются. Атакующий контролирует DNS-record evil.example.com → resolve в 169.254.169.254 → POST /webhooks принимает URL с 201, на доставке webhook-service делает HTTP к 169.254.169.254 (AWS IMDS) → SSRF реальный. Фикс архитектурный: после Resolve брать IP, проверять IsPrivate/IsLoopback ДО каждого HTTP-вызова (а не только при создании). Также защита через DNS-pinning или whitelist. Отдельная инициатива.

[OBSERVATION-2] API-key scope enforcement отсутствует полностью — Severity: HIGH (potential privilege escalation) — Категория: Architecture
  `APIKeyAuthMiddleware` (internal/gateway/portal/middleware/api_key_auth.go) валидирует токен через authClient.ValidateToken но НЕ проверяет требуемый scope endpoint'а против `key.Scopes`. На практике сейчас mw подключён только к /campaigns subrouter (router.go:296), остальные endpoint'ы portal — session-only (Bearer auth даёт 401 "Сессия не найдена"), поэтому реальный exploit-surface маленький. Но если scope-enforcement не добавить до расширения mw на /messages, /webhooks etc — read_only ключ сможет POST'ить. Архитектурный PR.

[OBSERVATION-3] API-key audit event с ClientID="" — Severity: LOW (preexisting) — Категория: Audit traceability
  api_keys.go::CreateAPIKey/RevokeAPIKey/UpdateAPIKey публикуют audit event с `ClientID=""` (только UserID). audit_log таблица архитектурно пуста (spec-015 дыра). Failed attempts на cross-user revoke ТАКЖЕ не публикуются (audit-publish после `if err != nil { return }` блока). Несколько issues в одном:  empty ClientID, отсутствие отрицательного аудита, отсутствие запоминания неуспешных попыток. Связано с BUG-spec-015.

[OBSERVATION-4] internal/services/auth/grpc/server.go:362-370 (UpdateAPIKey) — preexisting `==` sentinel pattern — Severity: COSMETIC
  Из review iteration 2: UpdateAPIKey hander использует `err == ErrAPIKeyNotFound/NotOwned/Revoked` вместо errors.Is. Не security-проблема, но непоследовательно с RevokeAPIKey после фикса. Bundle с другими cleanup'ами в общий PR (BUG-66 паттерн, delivery_client.go invalid URL format etc).

[OBSERVATION-5] Webhook GET /webhooks/{id} endpoint не зарегистрирован — Severity: COSMETIC
  curl /portal/v1/webhooks/{id} GET → plain-text "404 page not found" (gorilla default), не JSON-error. Нет соответствующего handler'а — только LIST/CREATE/UPDATE/DELETE/TEST. Frontend WebhooksPage не нуждается (LIST даёт всё), но REST-API contract drift.

[OBSERVATION-6] RotateAPIKey endpoint отсутствует (нет proto-RPC, нет handler'а) — Severity: LOW — Категория: Feature gap
  Если фича заявлена в спеке (нет в текущих docs/specs/) — gap. Если не заявлена — не баг. user может workaround: revoke + create new.

[OBSERVATION-7] Webhook secret signing — replay attack window — Severity: NOT TESTED
  Не проверял: содержит ли X-Webhook-Signature timestamp (HMAC-SHA256 over payload+timestamp + window check на receiver). Если только sig(payload) — replay-атака возможна. Webhook-service domain.WebhookEvent имеет `Timestamp` в payload, но клиент должен сам проверять окно — это implementation detail клиента. Кандидат на отдельный security-review этап.

[Success Path]
User создаёт API-key через POST /portal/v1/api-keys → видит plaintext sk_live_... ОДИН раз → ListAPIKeys возвращает только prefix/last_used. Bearer auth на /portal/v1/campaigns endpoint валидирует ключ (но scope игнорируется). DELETE собственного ключа → 204. Cross-user DELETE → 403. Webhook https://example.com/hook + event_types=[delivered] → 201 + secret one-shot. Cross-tenant webhook ops → 404. Notifications happy: GET /notifications → list. POST /notifications/{valid-uuid}/read → 200/404.

[Recommendations]
1. **Запланировать архитектурный PR**: API-key scope enforcement (OBSERVATION-2) и DNS-rebinding защита в webhook (OBSERVATION-1). Оба — существенные security-improvements.
2. **Накопительный cleanup-PR**: errors.Is sweep (OBSERVATION-4 + UpdateAPIKey + другие места), `delivery_client.go::ValidateURL` invalid-URL-format → domain.ErrInvalidURLFormat (вынесено reviewer'ом).
3. **Audit emit на failed authorization**: cross-user revoke attempts должны попадать в audit-log с outcome=denied (после spec-015-фикса).

[Test Data]
- Создавалось/удалялось: 2-3 API-keys (`audit-c1-test`, `audit-c2-test`, `audit-c2-revoke-test`, `audit-c2-iter2`) + 3 webhooks (`https://example.com/hook`, `https://example.com/c2hook`, `https://example.com/regression-test`). Все cleanup'нуты SQL DELETE после smoke (см. финальный шаг).
- Демо-баланс не затронут (никаких mutation на messages/billing).
- Коммиты: 5fcc29e lock, d709daf fix BUG-76/77/78 (proto + handler + tests + duplicate dir cleanup), ниже close-коммит.

## [DONE] Этап 24/30: User — lookup + analytics (user, fix + Infrastructure + QA full, 2026-04-30) — частичный (3 бага исправлено: 3 HIGH)

[Summary] 12 TC прогнаны через API+SQL: TC-1 happy /lookup/history empty PASS; TC-2 /lookup/stats happy PASS; TC-3 POST /lookup happy → 503 (HLR-провайдер не настроен на dev стенде, ожидаемо); TC-4/5/6/7 phone validation (empty/BOGUS/+1/SQL inj) → 400 PASS (e164Regex отрабатывает); TC-A1 /analytics happy default PASS; TC-A2/A3/A4/A5/A6/A7 invalid period/group_by/half-dates/invalid date_from/inverted dates/range>366 → все 400 PASS (whitelist-валидация работает); TC-A8 compare=true PASS; TC-A9 include_cost=true → wrong total_cost [BUG-75]; TC-A10 invalid compare → 400 PASS; TC-A11 group_by=country → 500 [BUG-74]; TC-12 cross-tenant lookup_log → c1 не видит c2 строку PASS; TC-12c c1 list своих → 500 [BUG-73]; TC-13 GET /lookup/history после фикса → 200 с item PASS.

3 HIGH исправлены одним PR через /execute-with-review (commit 669d961, code-reviewer APPROVED). Re-test всех 3 фиксов + cross-tenant регрессия PASS.

[BUG LIST]

BUG-73: GET /portal/v1/lookup/history → 500 при наличии любых строк в lookup_log клиента — Severity: HIGH — Категория: Functional / Silent feature breakage
  Шаги: INSERT в lookup_log одной строки для client_id, GET /portal/v1/lookup/history с куки этого клиента
  Ожидалось: 200 с items[]
  Получалось: 500 "Внутренняя ошибка сервера". В логе: `rpc error: code = Internal desc = ошибка сканирования записи: missing destination name operator_mccmnc in *domain.LookupLogEntry`
  Доказательство (API): TC-12c — после INSERT'а 55555555-... история падает в 500
  Импакт: на проде с реальным HLR-провайдером эта фича вообще не работала бы. На dev стенде HLR-провайдер не настроен, lookup_log пустая, баг был замаскирован. Любой пользователь, успешно сделавший хоть один lookup, видит 500 на странице истории.
  Фикс: 669d961 — `internal/services/routing/domain/hlr.go::LookupLogEntry` добавил db:-теги по схеме `migrations/000024_create_lookup_log.up.sql`.

BUG-74: GET /portal/v1/analytics?group_by=country → 500 — Severity: HIGH — Категория: Functional / Schema mismatch
  Шаги: GET /portal/v1/analytics?period=7d&group_by=country
  Ожидалось: 200 с by_country[] либо 400 (если фича не поддерживается)
  Получалось: 500 INTERNAL_ERROR. Лог analytics-service: `column "country" does not exist (SQLSTATE 42703)`. Backend whitelist принимал "country", SQL валился на отсутствующей колонке messages.country.
  Доказательство (API): TC-A11 вывод выше
  Импакт: frontend dropdown явно предлагает "Страна" с tab "По странам". User кликает → 500.
  Фикс: 669d961 — убрал country из бэкенд-whitelist (`internal/gateway/portal/handlers/analytics.go::allowedAnalyticsGroups`); фронт `AnalyticsPage.tsx` — удалил option "Страна", tab "По странам", CountryEntry, countryColumns. Жертва: by-country фича удалена из UI; для возврата нужен derive country из destination prefix или join по operators (отдельный архитектурный PR).

BUG-75: GET /portal/v1/analytics?include_cost=true возвращал произвольный total_cost (в т.ч. отрицательный) — Severity: HIGH — Категория: Functional / Silent contract break + UX
  Шаги: создать любые credits/charges в transactions для клиента, GET /portal/v1/analytics?period=7d&include_cost=true
  Ожидалось: total_cost = сумма charge-транзакций за период
  Получалось: на dev стенде с 4 credits-транзакциями (-100, +5.5, 0, -100) total_cost="-194.50". Корень: `internal/services/billing/grpc/server.go::GetTransactionHistory:264` принимает proto-поля TransactionType/From/To, но сразу их отбрасывает: `s.billingService.GetTransactionHistory(ctx, clientID, limit, offset)`. analytics-handler наивно суммировал ВСЕ транзакции клиента независимо от типа и даты.
  Доказательство (API): TC-A9 вывод выше; SQL transactions client=c1 показал 4 credits-строки.
  Импакт: financial misrepresentation в user-facing dashboard. Никаких реальных денег не теряется (display-only), но user видит абсурдный total_cost (включая отрицательные значения от credits).
  Фикс: 669d961 — defensive фильтр `tx.Type == "charge"` + диапазон дат в `sumChargeAmount`. Архитектурный фикс billing-service контракта (актуально применять proto-фильтры на стороне billing) вынесен в OBSERVATION-1.

[OBSERVATION-1] billing-service gRPC GetTransactionHistory игнорирует TransactionType/From/To — Severity: MEDIUM — Категория: Contract / API consistency
  Файл: `internal/services/billing/grpc/server.go:244-298`. Прото `GetTransactionHistoryRequest` объявляет TransactionType/From/To как фильтры. Хендлер их парсит из request, но не передаёт в `s.billingService.GetTransactionHistory(ctx, clientID, limit, offset)`. `BillingService.GetTransactionHistory(ctx, clientID, limit, offset)` сам не принимает фильтров. Любой gRPC-вызов с этими фильтрами получит ВСЮ историю клиента. Кандидаты ущерба: analytics (BUG-75 уже defensively обёрнут), любые external integrations через GetTransactionHistory. Фикс архитектурный: расширить сигнатуру BillingService.GetTransactionHistory + repo SQL WHERE-клаузу + handler передаёт filter через. Отдельный атомарный PR.

[OBSERVATION-2] /portal/v1/lookup/history `from`/`to` filter молча игнорируется при невалидной дате — Severity: LOW — Категория: Inconsistent error mapping
  В `internal/gateway/portal/handlers/lookup.go::GetLookupHistory:44-53` если `time.Parse(time.RFC3339, fromStr)` фейлится → fromStr просто не применяется, запрос отдаёт результат без фильтра. Аналитика handler 400'ит на ту же ошибку. Inconsistent UX. Накопительный паттерн с BUG-70 этапа 23.

[OBSERVATION-3] /portal/v1/lookup/stats period silently defaults на "7d" при невалидном — Severity: LOW — Категория: Inconsistent validation
  `lookup.go::GetLookupStats:262-275` принимает любую строку для `period`, не whitelist'ит. Возвращает 200 с period="haha" (проверено в TC-11). Сравните с analytics handler где невалидный period → 400. Cosmetic, можно объединить с OBSERVATION-2 в один cleanup-PR.

[OBSERVATION-4] /portal/v1/lookup/history `page_size>100` молча клампится до 50 — Severity: LOW — Категория: BVA UX
  `lookup.go::GetLookupHistory:73-77` при page_size>100 → ставит default 50 без сообщения. Не критично (silent clamp общепринят), но frontend получает другую страницу чем просил.

[OBSERVATION-5] frontend `LookupPage.tsx::loadHistory:71-72` читает `resp.total` но backend отдаёт `total_count` — Severity: LOW — Категория: Pagination contract drift
  `setHistoryTotal(resp.total || 0)` всегда даёт 0, потому что поле не существует. Pagination на странице lookup не работает корректно. Кандидат на отдельный mini-fix.

[OBSERVATION-6] sumChargeAmount проверяет maxChargeHistoryRows ДО типового фильтра — Severity: LOW — Категория: Pre-existing risk amplified
  Указан reviewer'ом: `analytics.go:294` cap считает все транзакции (включая non-charge которые potом отбрасываются). При большом количестве credits клиент может упереться в cap и недопосчитать total_cost. До OBSERVATION-1-фикса (билинг сам фильтрует) это amplified problem; после — проблема исчезает.

[OBSERVATION-7] After remove "country", `Tabs.Root`/`Tabs.List` в AnalyticsPage.tsx стал degenerate (1 trigger) — Severity: COSMETIC
  Указан reviewer'ом. Можно убрать tabs scaffold целиком, либо оставить (если планируется добавить новые tabs). Не блокирует.

[Success Path]
User /portal/lookup → POST /portal/v1/lookup {phone:"+79991110001"} → 503 на dev (provider not set) ИЛИ 200 с msisdn/operator на проде. GET /portal/v1/lookup/history → 200 с items[]. GET /portal/v1/lookup/stats?period=30d → 200 с total_lookups. User /portal/analytics → GET /portal/v1/analytics?period=7d&group_by=day&include_cost=true → 200 с timeline/summary/total_cost. group_by ∈ {day,week} (country удалён). compare=true → previous_timeline.

[Recommendations]
1. **Fix billing-service GRPC GetTransactionHistory contract** (OBSERVATION-1). Расширить сигнатуру BillingService.GetTransactionHistory(filters) + repo SQL filter + handler пробрасывает proto-поля. После этого можно убрать defensive фильтр в analytics.go (или оставить как defense-in-depth).
2. **Lookup-validation cleanup PR** (OBSERVATION-2/3/4): валидировать from/to RFC3339 в /lookup/history → 400 на bad input; whitelist period в /lookup/stats; вернуть 400 на page_size>100 вместо silent clamp.
3. **Frontend LookupPage.tsx pagination fix** (OBSERVATION-5): `resp.total_count` вместо `resp.total`. Пагинация в lookup-history безмолвно сломана.
4. **Country support как фича** (BUG-74 жертва): если фича нужна — добавить колонку `country` в `messages` через миграцию (получая из destination prefix через country_codes таблицу) либо `LEFT JOIN operators ON messages.provider_id`. Отдельная инициатива.

[Test Data]
- Засеяны 2 lookup_log строки: 44444444-... (c2) и 55555555-... (c1). Оставлены для будущих регрессионных проверок.
- Транзакции c1: 4 credits (-100, +5.5, 0, -100) — pre-existing с предыдущих этапов.
- Балансы клиентов не изменились (никаких mutation'ов на /lookup т.к. провайдер 503).
- Коммиты: d5d6a1d lock, 669d961 fix BUG-73/74/75, ниже close-коммит.



## [DONE] Этап 23/30: User — messages + cascade-history (user, fix + Infrastructure + QA full, 2026-04-30) — частичный (4 бага исправлено: 1 CRITICAL + 3 HIGH)

[Summary] 18 TC прогнаны через API+SQL: TC-1 happy list /detalization PASS; TC-2 cross-tenant /detalization/{c2 id} → 404 PASS; TC-3 invalid UUID на /detalization/{id} → 500 [BUG-69]; TC-4/5 invalid date_from/date_to → 500 [BUG-70]; TC-6 date_from > date_to → 200 empty PASS; TC-7 SQL inj в destination (через ILIKE %% — параметризовано) → пустой PASS; TC-8 status arbitrary → пустой PASS; TC-9 limit=0/-1/99999 → fallback 20 PASS; TC-10 offset=-1 clamp PASS, offset=999999 → empty PASS; TC-11 cross-tenant /portal/v1/messages/{c2 id} → 403 [observation: 403 leak'ает существование, должно быть 404]; TC-12 invalid UUID на /portal/v1/messages/{id} → 500 [BUG-69b]; TC-13 own quick-send GetMessage PASS; TC-14 cascade list /cascade/deliveries empty PASS; TC-15 cascade invalid UUID → 400 PASS (этап 20 фикс работает); TC-16 cascade GET random valid UUID → 500 [BUG-72]; TC-CSV-A legacy /messages/export — formula injection guard работает PASS; TC-CSV-B async /export/{id}/download — formula injection ОТСУТСТВУЕТ [BUG-71 CRITICAL]; TC-SSE without auth → 401 PASS; TC-SSE with auth → "Streaming не поддерживается" 500 (haproxy infra проблема dev-стенда, не аудит-баг).

1 CRITICAL + 3 HIGH исправлены одним PR через /execute-with-review (commit bd106d5, code-reviewer APPROVED with caveats). Re-test всех 4 фиксов + регрессия PASS.

[BUG LIST]

BUG-71: CSV formula injection в async export /portal/v1/export/{id}/download — Severity: CRITICAL — Категория: Security / CVE-class formula injection
  Шаги: засеять message с text='=cmd|''/c calc''!A1' (или любой текст начинающийся с =/+/-/@/tab) → POST /export/start → GET /export/{id}/download
  Ожидалось: ячейка text в CSV префиксована апострофом, чтобы Excel/Calc трактовал как plain-text
  Получалось: `aaaaaaaa-3333,DEMOQS,+79991110003,=cmd|'/c calc'!A1,delivered,...` — без префикса. При открытии экспорт-файла в Excel/Calc формула исполняется.
  Доказательство (API): TC-CSV-B вывод выше; legacy /messages/export применяет csvSanitize, async /export/{id}/download — нет.
  Импакт: атакующий с доступом к API/портал-аккаунту вставляет формулу в SMS-текст или sender, экспортирует, отправляет файл админу/реселлеру/клиенту → исполнение DDE/cmd при открытии.
  Фикс: bd106d5 — `internal/gateway/portal/handlers/export.go::runExportJob` обернул msg.MessageId/Source/Destination/Text/Status через csvSanitize (переиспользована из messages.go того же пакета). Re-test: `'=cmd|...` теперь имеет ' префикс.

BUG-69: GET /portal/v1/detalization/{id} с invalid UUID → 500 — Severity: HIGH — Категория: Functional / Error mapping (BUG-A pattern из этапа 22)
  Шаги: GET /portal/v1/detalization/not-a-uuid
  Ожидалось: 400 INVALID_INPUT
  Получалось: 500 "Ошибка получения сообщения" (pgx cast-error на `WHERE m.id = $1::uuid`)
  Фикс: bd106d5 — uuid.Parse pre-check в DetalizationHandlers::GetMessage → ErrInvalidInput → 400.

BUG-69b: GET /portal/v1/messages/{id} (quick-send путь) с invalid UUID → 500 — Severity: HIGH — Категория: Functional / Error mapping (тот же паттерн)
  Фикс: bd106d5 — uuid.Parse pre-check в MessageHandlers::GetMessage.

BUG-70: GET /portal/v1/detalization?date_from=INVALID → 500 — Severity: HIGH — Категория: Functional / Error mapping
  Шаги: GET /portal/v1/detalization?date_from=INVALID или ?date_to=2025-13-99
  Получалось: 500 "ошибка подсчёта сообщений" (pgx ::timestamptz cast-error в countQuery до listQuery)
  Фикс: bd106d5 — validateDateFilter (2006-01-02 либо RFC3339) в DetalizationHandlers::ListMessages, ErrInvalidInput при невалидном формате. Caveat от reviewer: RFC3339 с временем truncate'ится до даты — frontend шлёт 2006-01-02 (см. getDefaultDateRange в MessagesPage.tsx), регрессии нет.

BUG-72: GET /portal/v1/cascade/deliveries/{random valid UUID} → 500 — Severity: HIGH — Категория: gRPC error mapping
  Шаги: GET /portal/v1/cascade/deliveries/00000000-0000-0000-0000-000000000999
  Получалось: 500 "Внутренняя ошибка сервера". В логе portal-gateway: `rpc error: code = Unknown desc = delivery not found`. Repo возвращал domain.ErrDeliveryNotFound, handler делал `return nil, err` → codes.Unknown → portal маппит в 500.
  Фикс: bd106d5 — `internal/services/cascade/grpc/handler.go::GetDelivery` errors.Is(err, domain.ErrDeliveryNotFound) → codes.NotFound (404). Bad-UUID → codes.InvalidArgument. Cross-tenant и not-exists свёрнуты в один NotFound — иначе 403 vs 404 leak'нет чужие delivery_id (минимизация info-disclosure).

[OBSERVATION-1] /portal/v1/messages/{cross-tenant id} → 403 вместо 404 — Severity: LOW — Категория: Information disclosure
  Шаги: c1 запрашивает /portal/v1/messages/{c2 message id}
  Поведение: SQL fast-path в GetMessage не находит (client_id != $2) → ErrNoRows → fallback getMessageViaGRPC → gRPC GetMessageStatus с clientID-mismatch отдаёт PermissionDenied → handler maps на 403 FORBIDDEN.
  Импакт: атакующий перебором UUID может различить «не существует» от «чужое». UUID v4 — 122 бита случайности → перебор нереальный, но это нарушение принципа least disclosure. Так же как BUG-72 я свернул в 404 для cascade — для messages этот же фикс архитектурный (gRPC GetMessageStatus двухклассовая ошибка), не делал в этом этапе.

[OBSERVATION-2] gRPC GetMessageHistory.Total = len(currentPage), не глобальный — Severity: LOW — Категория: Pagination correctness
  В `internal/services/messaging/grpc/server.go:393-398` `Total: int32(len(protoMessages))` — это длина текущей страницы, не COUNT(*) по фильтру. Затрагивает legacy /portal/v1/messages list (handler messages.go::ListMessages) и /portal/v1/messages/export. Frontend MessagesPage НЕ использует этот endpoint (использует /detalization, у которого правильный total через COUNT(*)). Поэтому UI не сломан, но API контракт врёт. Кандидат на отдельный mini-fix (поправить gRPC server.go + добавить count-query в storage/repo).

[OBSERVATION-3] Прочие http.Error plain-text в SSE handler — Severity: LOW — Категория: BUG-66 паттерн
  `messages.go::StreamMessages` строки 558, 565 используют `http.Error(w, "...", 500)` — plain-text response, не JSON-error envelope. Накопительный паттерн, отдельный PR (по issue этапа 20).

[OBSERVATION-4] CreateDelivery/ListDeliveries/etc в том же `internal/services/cascade/grpc/handler.go` всё ещё возвращают `fmt.Errorf("invalid client_id: %w", err)` — codes.Unknown на bad UUID. Тот же класс что фиксили в GetDelivery. Кандидат на bulk-PR по cascade-сервису (BUG-A pattern).

[OBSERVATION-5] async export TTL — file_path в /tmp/ container'а удаляется после download (line 283 export.go). Если client скачал → ОК. Если не скачал в exportTTL=1h → файл остаётся (zombie /tmp/*.csv). Не критично, но накопится при долгом uptime. Оч. minor.

[Success Path]
User /portal/messages → GET /portal/v1/detalization?date_from=...&date_to=...&status=delivered → 200 с total + messages[]. Click на строку → /portal/messages/{id} → GET /portal/v1/detalization/{id} → 200 с DLR + billing деталями. Click "Экспорт" → POST /portal/v1/export/start → 202 {job_id} → poll /export/{job_id}/status → "ready" → GET /export/{job_id}/download → CSV-файл с применённой formula-injection защитой. Cascade history /portal/cascade-history → GET /portal/v1/cascade/deliveries → 200. Click delivery → GET /cascade/deliveries/{id} → 200 PROD-сценарий ИЛИ 404 если not-exists/cross-tenant. Invalid UUID на любом GetX → 400. Invalid date_from → 400.

[Recommendations]
1. **Свернуть /portal/v1/messages/{id} cross-tenant 403 → 404** (OBSERVATION-1). Архитектурный — gRPC GetMessageStatus в messaging-service сейчас отличает PermissionDenied от NotFound. Минимизация info-disclosure: оба → NotFound, как уже сделано в BUG-72.
2. **Поправить gRPC GetMessageHistory.Total** на реальный COUNT(*) по фильтру (OBSERVATION-2). Затрагивает messaging-service repo + gRPC + handler. Сейчас legacy /messages list/export даёт неправильное total.
3. **Bulk-fix BUG-A pattern в cascade gRPC handler** (OBSERVATION-4). 5+ методов всё ещё `fmt.Errorf` без gRPC-кода → 500 на bad-UUID и других domain-ошибках. Один атомарный PR.

[Test Data]
- Засеяны 3 messages (untracked в demo_seed): aaaaaaaa-1111 (c1, happy text), aaaaaaaa-2222 (c2, для cross-tenant), aaaaaaaa-3333 (c1, formula-injection text). Балансы Demo-Main 99805.50 ₽ / Demo-Reseller 50000.00 ₽ не изменились (никаких отправок не делал, INSERT'ил в БД напрямую — pipeline-sender лежит из-за pgbouncer DNS на dev стенде).
- Коммиты: 272a8d4 lock, bd106d5 fix BUG-69/70/71/72, ниже close-коммит.

## [DONE] Этап 22/30: User — quick-send (user, fix + Infrastructure + QA full, 2026-04-30) — частичный (2 бага исправлено)

[Summary] 13 TC прогнаны через API+SQL (TC-1 happy path, TC-2 cross-tenant sender_name FORBIDDEN, TC-3/4 pending+rejected sender FORBIDDEN, TC-5 empty phone 400, TC-6a/b/c BOGUS/SQL/long phone — раньше 500, теперь 400, TC-6d phone "+1" — раньше 201 (queued), теперь 400, TC-7a/c text empty/over 1600 → 400, TC-8 empty source → 400, TC-9 numeric source bypass sender_names → 201 OK, TC-10 invalid JSON → 400, TC-11 source >20 chars + non-numeric → 403 sender not found, TC-12 idempotency — нет idempotency-key, двойной клик = 2 SMS, TC-13 rate-limit hammer — 30 reqs/sec проходят без 429 при rate_limit_per_second=10).

1 HIGH + 1 MED исправлены через /execute-with-review (commit `0ad7953`, code-reviewer APPROVED). 2 observation (idempotency, rate-limit) — эскалированы пользователю как design-вопросы.

[BUG LIST]
BUG-A: validation→Internal error → HTTP 500 на user-input typos — Severity: HIGH — Категория: error-mapping/UX
  Шаги: POST /portal/v1/messages с destination="BOGUS"
  Ожидалось: 400 INVALID_INPUT (это ошибка пользователя)
  Получалось: 500 "Внутренняя ошибка сервера" (валидатор messaging-service возвращал ошибку, gRPC server заворачивал в codes.Internal вместо InvalidArgument)
  Доказательство (UI/API): TC-6a/b/c — `{"error":{"code":"INTERNAL_ERROR","message":"Внутренняя ошибка сервера"}}` HTTP 500
  Доказательство (DB): N/A — ошибка на handler-уровне до Kafka
  Фикс: `0ad7953` — `internal/services/messaging/domain/validation.go` добавил sentinel `ErrValidation`, обернул финальный error через `%w`. `internal/services/messaging/grpc/server.go` импорт domain + `errors.Is(err, domain.ErrValidation)` → `codes.InvalidArgument`. `respondGRPCError` уже маппит → 400. Re-test: BOGUS / SQL injection / long phone / over-text-limit → все 400.

BUG-B: validateDestination принимал phone с 1 цифрой ("+1") — Severity: MEDIUM — Категория: input-validation/garbage-routing
  Шаги: POST /portal/v1/messages с destination="+1"
  Ожидалось: 400 (E.164 минимум 4 цифры — нижняя граница для коротких кодов)
  Получалось: 201 queued — сообщение уходило в Kafka, роутер не находил оператора → silent drop / DLQ-spam (на dev стенде ничего не персистилось — pipeline-sender'ы упали из-за pgbouncer DNS, но в проде = мусор в DLQ + потенциальный noise в метриках)
  Доказательство (API): TC-6d — `{"message_id":"49ba1c87-...","status":"queued"}` HTTP 201
  Фикс: `0ad7953` — `validateDestination` заменил `hasDigit bool` на `digitCount int`, требует ≥4 цифр. RU короткие коды 100/112 не идут через user quick-send (внутренние/SMPP). UK 4-digit short codes — нижняя граница. Re-test: "+1" → 400 "destination must contain at least 4 digits".

[OBSERVATION-1] Нет idempotency-key (BUG-E) — Severity: MEDIUM
  Шаги: дважды POST /portal/v1/messages с одинаковым body
  Поведение: оба запроса возвращают разные message_id, status=queued. Frontend prevention есть (`disabled={sending}`), но network retry / curl / двойной POST из второй вкладки → 2 SMS = 2 списания.
  Эскалирован: требует дизайн-решение (Idempotency-Key header? requestId в payload? таблица идемпотентности?). Не фиксил в одиночку.

[OBSERVATION-2] Нет rate-limit на REST quick-send (BUG-F) — Severity: MEDIUM
  Шаги: 30 POST /messages с минимальными интервалами при rate_limit_per_second=10 на клиенте
  Поведение: все 201, никаких 429. Rate-limit может применяться на pipeline-уровне (back-pressure), но не на API.
  Эскалирован: нужен middleware на portal-gateway POST /messages с per-client token-bucket из `clients.rate_limit_per_second/minute/hour/day`. Архитектурное решение, требует middleware-стак.

[Success Path] User вводит phone +79991110001, текст "Hello", выбирает approved sender DEMOQS, нажимает «Отправить» → confirm dialog → POST /messages → 201 queued. Frontend поллит /messages/:id каждые 3s до terminal-статуса (или 60 attempts × 3s = 3min timeout).

[Recommendations]
1. Локализация валидаторных сообщений messaging-service. Сейчас юзер видит `"destination must contain at least 4 digits"` в HTTP 400 — лучше чем 500, но всё ещё английский. Reviewer-флаг.
2. Outer-wrap "validation failed: validation failed: ..." (двойной префикс) в `application/message_service.go:101` — косметика, отдельный фикс. Reviewer-флаг.
3. Idempotency-key и rate-limit на /messages POST — обязательно для production. Сейчас защита на frontend disabled-button, что обходится curl/двойной вкладкой/network retry.

[Test Data]
- sender_names: 4 строки (DEMOQS approved, DEMOPEND pending, DEMOREJ rejected — все client_id=c0000000...001; RESLLRQS approved client_id=...002) — для cross-tenant, status-проверок и happy-path.
- Балансы Demo-Main 99805.50 ₽ / Demo-Reseller 50000.00 ₽ не изменились (pipeline-sender лежит на dev стенде из-за pgbouncer DNS, тарификация не отрабатывала — это инфра стенда, не аудиторский баг).
- Лог коммитов: `3176266` lock, `0ad7953` fix BUG-A/B, ниже close-коммит.

## [DONE] Этап 21/30: User — campaigns + campaign-schedules (user, fix + Infrastructure + QA full, 2026-04-30) — частичный (1 CRITICAL + 1 HIGH исправлены)

[Summary] 16 TC прогнаны (TC-1 list+filter+clamp, TC-2 GET edge UUID/404, TC-3 happy create, TC-4 invalid/non-existent contact_list_id, TC-6 schedule past/far-future, TC-7 state-machine pause/resume/cancel в draft, TC-8 update с cross-tenant + name-only, TC-9 schedules CRUD + bad frequency + missing cron, TC-10 cross-tenant contact_list_id, TC-10b cross-tenant template_id, TC-11 cross-tenant template_campaign_id в schedule, TC-12 BUG-65 nil-slice, TC-extended SetVariants/SetRetryConfig/RetryFailed cross-tenant). 1 CRITICAL баг найден (cross-tenant ownership), 1 HIGH (500 на bad UUID), фикс одним PR через /execute-with-review с code-reviewer APPROVED.

[BUG LIST]

BUG-67 + BUG-68: CreateCampaign/UpdateCampaign/SetVariants/SetRetryConfig/RetryFailed принимали любой UUID contact_list_id и template_id без ownership check на client_id — Severity: CRITICAL — Категория: Security / IDOR + Money-flow
  Воспроизведение (на стенде): Demo-Main (`c0000000-...0001`) делал `POST /portal/v1/campaigns -d '{"contact_list_id":"<RESELLER's UUID>"}'` → HTTP 201, кампания записана с `client_id=Demo-Main` + `contact_list_id=Reseller's`. Аналогично для template_id.
  Импакт ($): материализатор `cmd/services/campaign-service/materialize.go:195-203` фетчит контакты `WHERE c.contact_list_id = $1` без client_id-фильтра. На LaunchCampaign → SMS уходят на чужие номера, тарифицируются по Demo-Main (списание баланса атакующего за чужие контакты + leak PII Reseller'а + hijack sender reputation).
  Forensic: на стенде нет уже существующих кросс-тенантских записей (SELECT с JOIN — 0 строк); миграция не требуется.
  Эскалация: пользователю до фикса (как BUG-59); решение «всё на твоё усмотрение, минимальный фикс».
  Фикс: 059720a.
    - 4 новых sentinel errors в `domain/models.go`.
    - 2 helper-метода `ContactListBelongsToClient`/`TemplateBelongsToClient` в `repository/campaign_repository.go` (SELECT EXISTS).
    - Гейт в `application/campaign_service.go::CreateCampaign`+`UpdateCampaign`+`SetVariants`+`SetRetryConfig`+`RetryFailed`.
    - mapError → InvalidArgument (bad UUID), NotFound (cross-tenant).
  Code-review (superpowers:code-reviewer): APPROVED with caveats; reviewer flagged SetVariants/SetRetryConfig/RetryFailed как тот же класс — добавлены в этот же PR.

BUG-А: invalid UUID format в contact_list_id/template_id → 500 ISE — Severity: HIGH — Категория: Functional / Error mapping (BUG-9/13/17/28/50/51 паттерн)
  Шаги: `POST /portal/v1/campaigns -d '{"contact_list_id":"not-a-uuid"}'` → HTTP 500 "Внутренняя ошибка сервера".
  Доказательство: ошибка `uuid.Parse` оборачивалась в `fmt.Errorf("invalid contact_list_id: %w")`, не матчилась в `mapError` → default → `codes.Internal`.
  Фикс: 059720a (sentinel `ErrInvalidContactListID`/`ErrInvalidTemplateID` → `codes.InvalidArgument` → HTTP 400).

[Наблюдения / без фикса в этом этапе]

- TC-6 (scheduledAt в прошлое/далеко в будущее): handler принимает любой timestamp и сохраняет в draft. Реальный риск только при LaunchCampaign — там status-machine не пускает scheduled-кампанию мгновенно (она в draft, не в scheduled). По дизайну это OK, но frontend должен валидировать на форме (CampaignWizardPage) — проверить отдельным UX-этапом.
- Cross-tenant защита в campaign_schedules.go — `Create` нативно делает `EXISTS(SELECT 1 FROM campaigns WHERE id=$1 AND client_id=$2)`. То есть schedule был защищён ИЗНАЧАЛЬНО. Аномалия: кампании менее защищены, чем расписания. Это observation о неконсистентности в кодовой базе.
- Materializer (`cmd/services/campaign-service/materialize.go:195-203`) до сих пор фетчит контакты `WHERE c.contact_list_id = $1` без client_id-фильтра. После закрытия create-path-vector атака невозможна, но defense-in-depth (`AND contact_list_id IN (SELECT id FROM contact_lists WHERE client_id=$2)`) дешёвый и защитит от регрессии. Отдельный mini-PR.
- BUG-65 паттерн (nil-slice): `GET /campaigns` и `GET /campaign-schedules` возвращают `[]` для пустого списка — норма ✓.
- Сообщение об ошибке cross-tenant case `"contact_list_id does not belong to client не найден"` — `respondError` приклеивает «не найден» к message при NOT_FOUND. Косметика, отдельный текст.
- Composite FK `(id, client_id) REFERENCES contact_lists(id, client_id)` — потребует миграции БД (UNIQUE на пару + ALTER TABLE), эскалация. Отложено как follow-up.

[Success Path]
User /portal/campaigns → видит [], total:0. POST /campaigns с валидным contact_list_id → 201, status=draft. Cross-tenant попытка → 404 NOT_FOUND `template_id/contact_list_id does not belong to client`. Bad UUID format → 400 INVALID_INPUT. PUT /campaigns/:id с cross-tenant contact_list → 404. PUT /:id/variants с cross-tenant template_id → 404. POST /:id/retry-config / retry с cross-tenant alt_template → 404. POST /:id/pause-resume-cancel в draft → 400 FailedPrecondition с понятным сообщением. Schedule CRUD: пустой список `{"schedules":[]}`, daily-frequency happy → 201, BOGUS-frequency → 400, custom без cron_expression → 400, cross-tenant template_campaign_id → 404 (защита handler'а нативно).

[Recommendations]
1. **Defense-in-depth materializer (mini-PR)**: добавить `AND contact_list_id IN (SELECT id FROM contact_lists WHERE client_id=$2)` в `processMaterializingCampaigns`. Cheap belt-and-suspenders.
2. **Унифицировать cross-tenant pattern**: вытянуть `XBelongsToClient(ctx, xID, clientID) (bool, error)` в общий helper `internal/shared/security/ownership.go`. Сейчас в schedule handlers тоже EXISTS-логика, в campaign service другая. Систематический cross-tenant аудит handlers/ — кандидат на отдельный security-этап (есть 48 файлов handlers с 434 `uuid.Parse`).
3. **Composite FK или DB CHECK**: ALTER TABLE campaigns ADD CONSTRAINT cl_owned CHECK (...) или композитный FK с UNIQUE. Требует миграции и согласования времени останова — отдельная задача.

[Test Data]
- На время тестов созданы: contact_list `1cc6dee9-...` Demo-Main, template `6bae15a2-...` Demo-Main, template `e513b431-...` Demo-Reseller (victim для cross-tenant), campaign `436aec8c-...` Demo-Main + variants. Все удалены после теста; `SELECT COUNT(*) FROM campaigns; campaign_schedules; contact_lists;` → 0/0/0.

Коммиты:
- e7cd17b docs(audit): этап 21/30 user campaigns+schedules — [IN_PROGRESS]
- 059720a fix(campaign): cross-tenant guard на contact_list_id+template_id [BUG-67/68/A этап 21/30]

## [DONE] Этап 20/30: User — channels + delivery-strategies (admin, fix + Infrastructure + QA full, 2026-04-30) — частичный (cascade hardening)

[Summary] 12 TC прогнаны (5 admin channels/strategies CRUD + edge, 4 user cascade deliveries/stats/strategies, 3 RBAC + clamp). 3 функциональных бага найдены, все исправлены одним PR. Дизайн-док §4 строка 107 ошибочно помечает этот этап как "user" — фактически /admin/channels и /admin/delivery-strategies admin-only с `AdminRoleMiddleware`. User-сторона: только read-only `/cascade/strategies`, `/cascade/deliveries`, `/cascade/stats`.

[BUG LIST]

BUG-64: GetChannel/UpdateChannel/ToggleChannel/GetStrategy/UpdateStrategy/DeleteStrategy/GetDelivery → 500 на invalid UUID — Severity: HIGH — Категория: Functional / Validation
  Шаги: `curl /portal/v1/admin/channels/not-a-uuid` → HTTP 500 "Внутренняя ошибка сервера". 7 точек в трёх handler-файлах (cascade_channels, cascade_strategies, cascade_deliveries).
  Доказательство: handler передавал `mux.Vars(r)["id"]` напрямую в gRPC `&GetChannelRequest{ChannelId: id}`. gRPC возвращал Internal на parse-error.
  Фикс: 51e47df (`uuid.Parse(id)` check на handler-уровне → 400 INVALID_INPUT с понятным сообщением).

BUG-65: ListChannels/ListStrategies/ListStrategiesClient возвращали `null` для пустого списка — Severity: LOW — Категория: Functional / Schema drift (BUG-61 паттерн)
  Шаги: `curl /portal/v1/admin/delivery-strategies` без записей → `{"strategies":null}`.
  Фикс: 51e47df (`if x == nil { x = []*cascadev1.XResponse{} }` в трёх местах).

BUG-66: 8 точек `http.Error(w, "...", 400)` в cascade-handlers возвращали plain-text — Severity: MED — Категория: Functional / Schema drift
  Шаги: `POST /portal/v1/admin/channels -d '{}'` → response `channel_type and name are required` (text/plain), HTTP 400.
  Доказательство: `http.Error()` пишет голую строку с заголовком `text/plain`. Frontend ожидает `application/json` с `{"error":{...}}`, парсит response.json() → крэш.
  Фикс: 51e47df (заменено на `respondError(w, shared.ErrInvalidInput("..."))` → корректный JSON).

[Наблюдения / без фикса в этом этапе]

- TC12: `cascade/deliveries?per_page=99999` не возвращает поле `per_page` в ответе. Frontend узнать клампнутое значение нельзя. Минор UX, отдельный микро-фикс.
- В репо `http.Error()` встречается ещё в 5+ местах handlers (`api_keys`, `notifications`, `quick_send` — не аудировано). Тот же паттерн BUG-66, накопительный PR.
- Аналогично с `uuid.Parse` отсутствием на handler-уровне — в репо 434 вхождения `uuid.Parse` в handlers/, но не везде с pre-check. Систематический аудит — отдельный refactor.
- Дизайн-док §4 строка 107 нужно поправить: "User: channels + delivery-strategies" → "Admin: channels + delivery-strategies (cascade)". Минор-doc-fix.
- Cascade strategies на стенде пусты (0 записей в БД). Создание стратегий не покрыто mutation TC — поскольку нужны существующие channels (есть SMS, Max Messenger, flash_call) и operator-channel-support — слишком цепная подготовка для read-only этапа.

[Success Path]
Admin /admin/channels → видит 3 канала (sms/max_messenger/flash_call), создаёт/обновляет/toggle. /admin/delivery-strategies → CRUD стратегий с шагами (channel_id, step_order, timeout, billable). User /cascade/deliveries (history paged), /cascade/stats (channel-by-channel breakdown), /cascade/strategies (read-only активные). Все list-эндпоинты при пустом результате возвращают `[]` (не `null`). Все Get/Update/Delete возвращают 400 на невалидный UUID и корректный JSON 400 на missing fields.

[Recommendations]
1. **Накопительный PR на http.Error → respondError**: grep `http.Error` показывает остаток в 5+ файлах handlers. Привести к единому стилю.
2. **uuid validation helper**: ввести общий `parseUUIDPath(r, "id") (uuid.UUID, *AppError)` после grep-аудита всех handlers (~48 файлов). Самостоятельный рефактор-этап.
3. **Cascade strategies seed**: для регресса добавить 1-2 стратегии в `demo_seed.sql` (sequential SMS→Max → flash_call) — облегчит smoke-test cascade-feature.

[Test Data]
- 3 канала на стенде в seed (sms, max_messenger, flash_call). Не модифицировал.
- 0 стратегий, 0 deliveries — normal для нового стенда.

Коммиты:
- 16d81bf docs(audit): этап 20/30 user channels+delivery-strategies — [IN_PROGRESS]
- 51e47df fix(portal): cascade — UUID validation, JSON-error responses, nil-slice→[] [BUG-64/65/66 этап 20/30]

## [DONE] Этап 19/30: User — sender-names + templates (user, fix + Infrastructure + QA full, 2026-04-30) — частичный (1 баг исправлен)

[Summary] 18 TC прогнаны (7 sender-names list/get/create/update/edge/RBAC, 11 templates list/CRUD/render/edge/cross-tenant). 1 функциональный баг найден в templates handler, исправлен одним PR. Sender-names handler — robust (clamp limit, валидация ID, INN format checksum, статусы).

[BUG LIST]

BUG-63: CreateTemplate/UpdateTemplate принимали любую строку как traffic_type — Severity: MED — Категория: Functional / Schema validation
  Шаги: `POST /portal/v1/templates -d '{"name":"x","body":"y","traffic_type":"BOGUS"}'` → HTTP 201, шаблон сохранён с `traffic_type=BOGUS` в БД.
  Доказательство: на всех слоях (handler/gRPC/service/domain) валидация отсутствовала. Допустимые значения по docstring `domain.Template.TrafficType`: `authorization, transactional, service`. БД — varchar без CHECK constraint. Подтверждено grep по коду: `routing/domain.ValidTrafficType` уже имеет такой enum (3 значения), pipeline default = `transactional`, frontend `ConditionEditor.tsx` — 3 опции.
  Влияние: тарификация по operator_template фильтрует по traffic_type — мусорные значения не попадают ни в один тариф (silent billing failure). Frontend ожидает enum; UI логика ломается на BOGUS.
  Фикс: f48416b.
    - `domain.AllowedTrafficTypes` map (3 значения, синхронизировано с routing).
    - `domain.ErrInvalidTrafficType` + mapError → InvalidArgument в gRPC server.
    - `application.validateTrafficType`: пустая строка OK (дефолтит в transactional), другое → wrapped error.
    - Применён в CreateTemplate (после validateBody) и UpdateTemplate (внутри ветки `if trafficType != nil && != ""`).
  После фикса: traffic_type=BOGUS → 400 "invalid traffic_type: must be one of authorization, transactional, service"; пустая строка → 201 (дефолтится).

[Наблюдения / без фикса в этом этапе]

- На стенде осталась запись `traffic_type=BOGUS` (template id `a8ebee55-...`, создана в TC16 ДО фикса). Update без явного trafficType её не валидирует — корректное поведение (фикс не должен ломать legacy данные). Cleanup-PR отдельно: либо `UPDATE templates SET traffic_type='transactional' WHERE traffic_type NOT IN (...)`, либо удаление мусорных записей.
- Миграция `ALTER TABLE templates ADD CONSTRAINT traffic_type CHECK ...` — эскалация по §5.4 (миграция БД нужна для defense-in-depth). На стенде её сейчас нет, поэтому грязные данные могут возвращаться через прямой SQL. После cleanup стоит добавить.
- Sender-names handler — `if id == ""` проверка, но без UUID format validation на handler-уровне. gRPC service делает `uuid.Parse` и возвращает корректные 400 — двойная проверка избыточна.
- ListSenderNames clamp работает: per_page=99999 → 100. По parsePagination max 500, но gRPC client'ом видимо ещё клампится. Минор-difference UX, не баг.
- Reseller (aggregator) endpoints `/portal/v1/sender-names/...` через `reseller_sender_names.go` не покрыты в этом этапе — относятся к этапу 26 (aggregator).
- TemplatesPage в UI: рендеринг и редактирование, но не покрыто в TC18 (только API). Frontend проверка отдельная.

[Success Path]
User /portal/sender-names → видит свои зарегистрированные имена с фильтром по статусу. CreateSenderName с автовыбором default-компании (если есть только одна, или явно выбранная default — берётся, иначе ошибка с подсказкой). Update имени, Resubmit отклонённого. /portal/templates → CRUD, RenderTemplate валидирует variables. CreateTemplate теперь rejects невалидные traffic_type.

[Recommendations]
1. **Cleanup-PR на legacy traffic_type**: нормализовать существующий мусор в `templates.traffic_type` (`UPDATE WHERE NOT IN (3 valid)`), затем добавить миграцию CHECK constraint.
2. **Унификация enum traffic_type**: завести общий пакет `internal/shared/types/traffic.go` или явно в `routing/domain` и импортировать в template-domain. Сейчас два места держат отдельные maps — расхождение неизбежно.
3. **Validation для sender_name format**: handler принимает любую длину, gRPC валидирует "1-11 alphanumeric или 1-15 digits". Можно перенести в handler для UX (быстрая ошибка без trip к gRPC). Минор.

[Test Data]
- 1 шаблон с `traffic_type=BOGUS` (id `a8ebee55-939a-445b-8f97-359b3a7fc113`) — создан ДО фикса для регресс-проверки.
- 1 валидный шаблон (id `bbe78035-95b3-496a-b291-5b79f5ebed86`) — после фикса с `transactional`.
- 1 шаблон с пустым traffic_type → дефолт transactional (id `15ebcdf4-5e5c-4226-8bef-78f76a1c852f`).

Коммиты:
- 5d400a0 docs(audit): этап 19/30 user sender-names+templates — [IN_PROGRESS]
- f48416b fix(template): валидация traffic_type enum [BUG-63 этап 19/30]

## [DONE] Этап 18/30: User — companies + contacts + segments (user, fix + Infrastructure + QA full, 2026-04-30) — частичный (segments-only fixes)

[Summary] 15 TC прогнаны (3 list, 4 companies CRUD/edge, 5 segments CRUD/estimate/edge, 2 contacts/contact-lists, 1 RBAC). 2 функциональных бага найдены, оба в segments handler, исправлены одним PR. Companies/contacts handlers — robust, без багов в скоупе.

[BUG LIST]

BUG-61: GET /portal/v1/segments возвращал `{"segments":null}` вместо `{"segments":[]}` — Severity: LOW — Категория: Functional / Schema drift
  Шаги: `curl /portal/v1/segments` для клиента без сегментов → `{"segments":null}`. Frontend ожидает массив для `.map()`/`.filter()` — null крэшит UI.
  Доказательство: `SegmentService.List` возвращает nil-slice, `json.Marshal(nil-slice)` = `null`.
  Фикс: 7642f37 (handler-side `if segments == nil { segments = []*domain.SavedSegment{} }`).

BUG-62: 4 точки в segments handler утекали raw err.Error() в HTTP body — Severity: LOW — Категория: Security / Information disclosure
  Шаги: любая ошибка repo/SQL в `CreateSegment`/`ListSegments`/`UpdateSegment`/`EstimateSegment` возвращала `respondError(w, shared.ErrInternalServer(err.Error()))` — клиент получал внутреннее сообщение (имена таблиц, SQL-конструкции).
  Фикс: 7642f37 (на каждой точке `log.Error().Err(err).Str(context_id)` + абстрактное сообщение `"Не удалось ..."` клиенту). Импорт `github.com/rs/zerolog/log` добавлен.
  Companies и contacts handlers проверены grep'ом — паттерн там не повторяется.

[Наблюдения / без фикса в этом этапе]

- DeleteSegment (`segments.go:158`) игнорирует ошибки `uuid.Parse` и `service.Delete` — тихая 204 даже при ошибке БД. Reviewer-замечание. Это полу-баг (delete idempotent), но дисциплинировать стоит. Follow-up.
- UpdateSegment (`segments.go:137`): `id, _ := uuid.Parse(...)` — невалидный UUID молча превращается в `uuid.Nil`. Аналогичная проблема. Follow-up.
- ListSegments не имеет pagination (per_page/total/page). Frontend `SegmentsPage` paginate в JS (загружает всё). Допустимо для текущего volume, но при росте до ≥1000 сегментов — проблема. Feature gap.
- CreateCompany без явной валидации Name/INN на handler-уровне — но gRPC service валидирует чётко (TC8/9 → 400 с понятным сообщением). Двойная валидация была бы избыточна для простого case'а.
- Companies ownership-check через ListClientCompanies (`companies.go:98-110`) — N+1 при множестве запросов. Минор-perf, scope creep.

[Success Path]
User /portal/companies → видит список своих компаний (default, full_name, INN). Создаёт компанию (валидируется INN checksum, name required). Редактирует (ownership-check, подтверждается принадлежность). SetDefault/Detach по ID — UUID валидируется. /portal/contact-lists → CRUD списков, batch-импорт contacts с tags. /portal/segments → создаёт сегмент с rules + contact_lists, estimate count, обновляет. Пустой список возвращает `[]` (не null).

[Recommendations]
1. **Pagination в ListSegments**: `?page=1&per_page=50` + `clampPagination` (как billing). При росте до >1k сегментов на клиента — критично.
2. **Error handling в DeleteSegment/UpdateSegment**: проверять uuid.Parse error и Delete-возврат. 5 минут работы.
3. **Audit-логирование для company CRUD**: события создания/изменения компании (юр. данные!) сейчас не пишутся в audit-service. Связано с этапом 15 / spec 015.

[Test Data]
- Demo-Main client `c0000000-0000-0000-0000-000000000001`: 1 компания `cc000000-0000-0000-0000-000000000001` (ООО Демо-Главный, default). Не создавал новых. Сегментов 0.
- Никаких mutation-операций на этом этапе (только GET + edge cases на mutation эндпоинтах с invalid UUID/payload).

Коммиты:
- cb3c9f8 docs(audit): этап 18/30 user companies+contacts+segments — [IN_PROGRESS]
- 7642f37 fix(portal): segments — nil-slice → []; не утекаем raw err.Error() в HTTP body [BUG-61/62 этап 18/30]

## [DONE] Этап 17/30: User — dashboard + profile + balance (user, fix + Infrastructure + QA full, 2026-04-30) — частичный (CRITICAL billing fix)

[Summary] 14 TC прогнаны (1 dashboard, 4 profile GET/PUT/password/phone, 9 billing balance/transactions/top-up/threshold + RBAC). 2 функциональных бага найдены, оба исправлены одним PR. BUG-59 (CRITICAL) — реальное списание средств с баланса через user-портал TopUp с отрицательным amount. BUG-60 (HIGH) — accept negative threshold + 500 на non-numeric.

[BUG LIST]

BUG-59: POST /portal/v1/billing/top-up принимал amount=-100/abc/0/Inf и реально списывал баланс через callback — Severity: CRITICAL — Категория: Security / Billing integrity (повтор BUG-41)
  Шаги: `POST /portal/v1/billing/top-up -d '{"amount":"-100"}'` → HTTP 200 с payment_url. Открытие `payment_url` (callback) → HTTP 200. Баланс 99905.50 → 99805.50.
  Доказательство: Stub-провайдер `internal/gateway/portal/payment/stub.go::HandleCallback` возвращает `Amount: req.Amount` без изменений. `BillingHandlers.TopUpCallback` (`internal/gateway/portal/handlers/billing.go:151-177`) вызывает `billingClient.AddCredits(ctx, AddCreditsRequest{Amount: result.Amount})`. **gRPC `s.AddCredits` НЕ валидировал amount** (BUG-41 fix этапа 13 был только на admin HTTP-уровне). Negative amount проходил до `BillingService.AddCredits` → `add(balance, amount)` через big.Float `+ -100` = `-100`.
  Фикс: b109b03 (двухуровневая защита).
    - `internal/services/billing/grpc/server.go::AddCredits`: helper `validatePositiveDecimal` (big.Float SetString → ok, IsInf, Sign() <= 0 → InvalidArgument).
    - `internal/gateway/portal/handlers/billing.go::TopUp`: ранняя валидация `validatePositiveAmount` перед `paymentProvider.CreatePayment` (UX — ошибка возвращается сразу, не через payment session).
    - Регресс-тесты `TestValidatePositiveAmount` (10 кейсов: empty/non-numeric/-100/0/+0/Inf/+Inf/NaN + 3 валидных).
    - После фикса: amount=-100 → 400 "amount должен быть положительным"; amount=Inf → 400 "amount не может быть Inf"; amount=500 → 200 (валидно).

BUG-60: PUT /portal/v1/billing/low-balance-threshold принимал threshold=-100 + 500 на abc — Severity: HIGH — Категория: Functional / Validation
  Шаги: `PUT /low-balance-threshold -d '{"threshold":"-100"}'` → HTTP 200 success. `threshold=abc` → HTTP 500 "Внутренняя ошибка сервера" (паттерн BUG-9/13/17/28).
  Доказательство: gRPC `SetLowBalanceThreshold` пропускал любую строку в `BillingService.SetLowBalanceThreshold` → SQL `numeric "-100"` или `numeric "abc"` → invalid syntax → 500.
  Фикс: b109b03.
    - gRPC: `validateNonNegativeDecimal` (Sign() < 0 → ошибка, NaN/Inf тоже).
    - Portal handler: `validateNonNegativeAmount` ранее.
    - 0 допускается (= выключение уведомлений).
    - После фикса: threshold=-100 → 400 "значение не может быть отрицательным"; threshold=abc → 400 "значение должно быть числом"; threshold=0 → 200 (валидно).

[Наблюдения / без фикса в этом этапе]

- BUG-60a: TopUpCallback (`billing.go:172`) логирует ошибку AddCredits, но всё равно возвращает HTTP 200 status:ok. Если AddCredits падает (например, после моего фикса при amount=-100 от malicious payment provider), provider получит 200 и пометит платёж как успешный — а зачисления не будет. Reviewer-замечание: правильное решение — outbox pattern (callback пишет в `payment_events` → 200 → отдельный воркер ретраит AddCredits). Текущий стенд использует stub-провайдер, в проде эта дыра реальна. Эскалация на отдельный PR.
- ChangePassword не проверяет complexity (только len ≥ 8). Тот же gap, что в admin BUG-49 — known follow-up.
- UpdateProfile.phone проверяет только len ≤ 50, не формат. Минор.
- VerifyTOTP не имеет видимого rate-limit/brute-force защиты на portal-уровне. Защита возможна на auth-service (TODO для security audit).
- На стенде уже остались мусорные транзакции: до фикса BUG-59 проходили top-up amount=-100 → callback зачислил → балас 99805.50 (до фикса баланс был 99905.50). Не восстанавливаю — стенд dev, нет SLA.

[Success Path]
User открывает /portal/dashboard → балас, активные кампании, charts. /portal/profile → редактирует contact_person/phone (валидация >255/>50), меняет пароль (current+new ≥ 8), включает/выключает 2FA с подтверждением паролем. /portal/billing → balance, transactions list (paged), top-up amount > 0 (rejects -/0/abc/Inf/NaN), set low-balance-threshold ≥ 0 (валидируется).

[Recommendations]
1. **Outbox для payment callbacks**: текущий callback path при ошибке AddCredits возвращает 200 — потенциал для silent loss средств (BUG-60a). Outbox-таблица + воркер закроют это правильно.
2. **Password complexity**: добавить zxcvbn в три места — admin CreateUser, user ChangePassword, user Register. Сейчас "12345678" принимается везде.
3. **TopUp UX**: вернуть в payment_url расшифровку платёжного провайдера (карты, СБП) — сейчас всегда stub-callback URL даже в dev.
4. **TOTP rate-limit**: ограничить попытки VerifyTOTP в auth-service (5 попыток на 5 минут).

[Test Data]
- Demo-Main client `c0000000-0000-0000-0000-000000000001`, balance ≈ 99805.50 (изменён ДО фикса BUG-59 через TopUp -100, не восстанавливался).
- Регресс-тесты `TestValidatePositiveAmount` (11 cases с NaN) и `TestValidateNonNegativeAmount` (7 cases).
- Профиль клиента обновлен через TC2: contact_person="Test User", phone="+79001234567".

Коммиты:
- 6149b4a docs(audit): этап 17/30 user dashboard+profile+balance — [IN_PROGRESS]
- b109b03 fix(billing): валидация amount/threshold на gRPC + portal-handler [BUG-59/60 этап 17/30]

## [DONE] Этап 16/30: Admin — detalization (deprecated) (admin, fix + Infrastructure + QA full, 2026-04-30) — корректность deprecation

[Summary] 8 TC прогнаны (1 list, 1 get, 1 invalid filter, 1 limit clamp, 1 RBAC, 1 nav-проверка, 1 doc-grep, 1 deprecation header). Найден один баг (BUG-58: HTTP-уровень не сигнализировал об устаревании), исправлен. Deprecation работает корректно: UI-banner есть, nav-ссылка убрана, HTTP-headers теперь явные. Endpoint оставлен функциональным до отдельного cleanup-PR.

[BUG LIST]

BUG-58: GET /admin/v1/messages* без HTTP `Deprecation`/`Link` headers — Severity: LOW — Категория: Documentation / API contract
  Шаги: `curl -i /admin/v1/messages` → ответ 200 без сигнала об устаревании. Внешние клиенты (если такие есть) не получают информации, что endpoint обслуживает удаляемый раздел /admin/detalization.
  Фикс: 342bcc7 (helper `markDeprecated` ставит `Deprecation: true` + `Link: </admin/v1/messages>; rel="deprecation", </messages>; rel="successor-version"` на ListMessages и GetMessage).

[Наблюдения / без фикса в этом этапе]

- TC4 `/admin/v1/messages?date_from=not-a-date` → HTTP 500 (pgx parse error). Паттерн BUG-9/13/17/28 — но endpoint deprecated, фикс уйдёт в накопительный PR (если переживёт cleanup).
- Reviewer-замечание: `Link: rel="deprecation"` ссылается на сам endpoint вместо документации об устаревании (RFC 8594 рекомендует ссылку на doc resource). Doc-страницы deprecation сейчас нет — будет добавлена при оформлении cleanup-PR.
- Sunset header не ставлю — точной даты удаления нет, фиктивная дата вреднее отсутствия (клиенты могут закешировать).
- Документы (`docs/superpowers/plans/2026-04-10-admin-phase2-reworks.md`, `docs/generate_aggregator_summary.py`, `docs/reports/2026-04-15-code-health.md`) содержат старые упоминания "Детализация" без deprecation-пометок. Это исторические планы, не оперативная документация — не трогаю.
- backend handler работает напрямую с БД (`db *storage.DB`), не через gRPC analytics-service — это ускорит cleanup (нет proto-обязательств), но усложнит миграцию пользовательского трафика на /messages (другой источник данных).

[Success Path]
Admin не видит "Детализация" в nav AdminLayout. По прямому URL `/admin/detalization` страница открывается с amber-banner "Раздел устарел. Используйте /messages". Клик по ссылке ведёт на /messages (новая страница MessagesPage). HTTP-клиент при `GET /admin/v1/messages*` получает корректные данные + headers `Deprecation: true` и `Link: ... rel="successor-version"` указывающий на /messages.

[Recommendations]
1. **Cleanup-PR на удаление endpoint**: после 1-2 циклов сбора метрик использования (логи admin-gateway по path /admin/v1/messages) принять решение об удалении handler'а, route'а, frontend-страницы DetalizationPage и подкомпонентов SmsPreview/StatusTimeline. Удалить миграцию, если есть таблицы/views, использовавшиеся только этим разделом.
2. **Sunset header**: при оформлении cleanup-PR установить дату удаления формата RFC 7231 HTTP-date.
3. **Deprecation doc page**: сделать `/docs/api/deprecation/messages` с описанием миграции и обновить `Link` rel="deprecation" target.

[Test Data]
- Никаких mutation-операций на этом этапе. Семь TC read-only через curl + grep по фронтенду. БД не менялась.

Коммиты:
- 35f19ab docs(audit): этап 16/30 admin detalization (deprecated) — [IN_PROGRESS]
- 342bcc7 fix(admin): detalization endpoints — Deprecation/Link headers [BUG-58 этап 16/30]

## [DONE] Этап 15/30: Admin — monitoring + analytics + audit-log (admin, fix + Infrastructure + QA full, 2026-04-30) — частичный (audit-log fixes, monitoring/analytics OK)

[Summary] 18 TC прогнаны (1 monitoring realtime, 4 analytics stats/period/group_by/RBAC, 4 generate report/perfomance, 9 audit list+date+RBAC+immutability+SQL-проверка). 3 функциональных бага, все три исправлены одним PR (BUG-55/56/57). BUG-55 (HIGH) маскировал инфраструктурную ошибку — wiring AuditClient в admin-gateway полностью отсутствовал в коде main.go. BUG-57 (HIGH) — schema mismatch frontend↔backend: frontend AuditLogPage шёл с одним набором параметров, backend читал другой.

[BUG LIST]

BUG-55: AuditClient в admin-gateway всегда nil — Severity: HIGH — Категория: Infrastructure / Wiring
  Шаги: открыть /admin/audit-log → пустой список без ошибки. `curl /admin/v1/audit` → HTTP 200 `{"entries":[],"total":0}`.
  Доказательство (cmd/admin-gateway/main.go строки 74-84 ДО фикса): структура `admin.ServiceAddresses` инициализировалась без поля `Audit`. В `internal/gateway/admin/clients.go:169` проверка `if addresses.Audit != ""` всегда false → AuditClient оставался nil.
  Фикс: 9f1d779 (добавлено `Audit: config.EnvOrDefault("AUDIT_SERVICE_ADDR", "localhost:9102")`. Также `AUDIT_SERVICE_ADDR=audit-service:9102` пропроброшен в env admin-gateway-1/2 в docker-compose.yml).

BUG-56: AdminAuditHandlers.ListAuditLog тихо возвращал stub при nil-client — Severity: MED — Категория: Functional / UX deception
  Шаги: при `auditClient == nil` handler возвращал `{entries:[],total:0,page:1,total_pages:0}` HTTP 200 без какого-либо сигнала.
  Это маскировало BUG-55: UI показывал "записей нет" вместо реальной ошибки конфигурации. Параметры запроса даже не валидировались.
  Фикс: 9f1d779 (явный 503 SERVICE_UNAVAILABLE + log.Error с подсказкой про AUDIT_SERVICE_ADDR).

BUG-57: Frontend AuditLogPage и backend AuditHandler — schema mismatch — Severity: HIGH — Категория: Functional / Schema drift
  Шаги: frontend `auditAdminApi.list({user_id, action, from, to, limit, offset})`. Backend читает `client_id, action, user_id, date_from, date_to, page, per_page` и возвращает `entries, total, page, total_pages`. Любые попытки фильтрации просто игнорировались.
  Доказательство: api/admin.ts:520-523 ДО фикса слал `from/to/limit/offset`; handler audit.go:39-48 их не читает.
  Фикс: 9f1d779 (api/admin.ts слой переписан на корректные параметры; AuditLogPage получил Select-фильтр Клиент, добавлен empty-state «Выберите клиента» при пустом client_id; обработчик ошибок показывает сообщение от backend, а не generic toast).

[Наблюдения / без фикса в этом этапе]

- audit_log таблица **пустая** в БД (0 строк). При выбранном client_id запрос корректный, но никаких записей не возвращается. Это отдельный архитектурный пробел: ни один admin-mutation handler не пишет в audit-сервис. Spec 015 (architecture-data-flows) предполагает audit-event на каждое изменение, но реально в код event-эмиссия не встроена. Эскалация — отдельный large refactoring tикет.
- Admin global audit view не поддерживается в gRPC AuditService (требует tenant_id). Аналог BUG-38 (admin global webhook view). Если нужно — добавить отдельный gRPC method `ListAllAuditEntries` с pagination или sub-tenant filter.
- TC14 GET /admin/v1/analytics/providers//performance (двойной слеш) → HTTP 301 redirect от mux. Минор UX, не реальный bug — frontend такой URL не делает.
- TC16 generate report с format=bogus → 400 "from and to timestamps are required" (gRPC analytics service первой проверкой требует таймстампы). Сообщение не точное к ошибке, но валидация работает.
- common.go::parsePagination clampит per_page>500 reset to default 50, не до 500. clampPagination из billing.go был бы корректнее (clamp до max). Минор, follow-up.

[Success Path]
Admin открывает /admin/monitoring → realtime-метрики, статус провайдеров, polling 10s. /admin/analytics → корректные ошибки на невалидных датах/UUID, GenerateReport валидирует report_type. /admin/audit-log → выбор клиента в фильтре → загрузка записей этого клиента (если они есть) с фильтрами user_id/action/resource_type/date_from/date_to. Пустой client_id → empty-state с подсказкой «Выберите клиента», без toast-spam.

[Recommendations]
1. **Audit event-emission**: внедрить запись в audit-service во все admin/portal mutation handler'ы (как cross-cutting middleware). Без этого БД остаётся пустой, и весь audit-log UI бесполезен. Это требует архитектурного решения и спеки.
2. **Admin global view**: либо новый gRPC method `ListAllAuditEntries` с pagination, либо принять текущее ownership-модель (как webhook) и оставить tenant_id required.
3. **Унификация pagination** в общей utility: `clampPagination` (billing) vs `parsePagination` (common) делают разные вещи. Привести к единому интерфейсу.

[Test Data]
- Selected client `c0000000-0000-0000-0000-000000000001` (Demo-Main) для проверки audit list — записи 0.
- Никаких тестовых записей в audit_log не создавал (нет endpoint write-side, события не пишутся в принципе).

Коммиты:
- a30e786 docs(audit): этап 15/30 admin monitoring+analytics+audit-log — [IN_PROGRESS]
- 9f1d779 fix(admin): audit-log — wire AuditClient + client_id filter в UI [BUG-55/56/57 этап 15/30]

## [DONE] Этап 14/30: Admin — users + settings (admin, fix + Infrastructure + QA full, 2026-04-30) — частичный (API+infra)

[Summary] 32 TC прогнаны (4 list+roles+permissions, 7 CreateUser/edge, 5 GetUser/UpdateUser, 3 Deactivate/Reset2FA/ResetPassword, 5 SystemDefaults, 3 RBAC, остальные boundary). 8 функциональных багов найдены, 6 исправлены одним PR (BUG-47/48/49/52/53/54), 2 ушли в наблюдения как накопительный паттерн. Среди исправлений два HIGH (BUG-53 кнопка "Сбросить 2FA" в admin была полностью сломана из-за ошибки в SQL repo, BUG-54 страница /admin/settings полностью сломана из-за неявного bytea→jsonb cast в pgx-stdlib).

[BUG LIST]

BUG-47: GET /admin/v1/users и /admin/v1/roles не клампят limit (паттерн BUG-33/46) — Severity: LOW — Категория: Performance / DoS-vector
  Шаги: `curl /admin/v1/users?limit=99999` → HTTP 200 с `"limit":99999`. Параметр уходит в gRPC и SQL без ограничения.
  Фикс: ea16048 (helper `clampPagination` из billing.go применён в `ListUsers` и `ListRoles`; `usersMaxListLimit = 200`).

BUG-48: CreateUser принимает любую строку как email — Severity: HIGH — Категория: Functional / Data integrity
  Шаги: `POST /admin/v1/users -d '{"email":"not-an-email",...}'` → HTTP 201 с битыми данными.
  Доказательство (handler до фикса): только проверка `req.Email == ""` — никакой валидации формата.
  Фикс: ea16048 (`validateUserEmail` через `net/mail.ParseAddress` + `addr.Address != email` для отсечения display-name синтаксиса).

BUG-49: CreateUser принимает пароль из 1 символа — Severity: HIGH — Категория: Security / Authentication
  Шаги: `POST /admin/v1/users -d '{"password":"a",...}'` → HTTP 201.
  Фикс: ea16048 (`validateUserPassword`, минимум 8 символов).
  Known gap: complexity (заглавные/цифры/спецсимволы) не проверяется. "12345678" пройдёт. Follow-up — добавить zxcvbn или regex `[A-Z]&[a-z]&[0-9]`.

BUG-52: PUT /admin/v1/users/:id с partial body (только email) → 500 — Severity: HIGH — Категория: Functional / Schema drift (BUG-35 паттерн)
  Шаги: `PUT /admin/v1/users/<id> -d '{"email":"x@y.com"}'` → HTTP 500.
  Доказательство (auth-service log): `ERROR: insert or update on table "users" violates foreign key constraint "users_role_id_fkey" (SQLSTATE 23503)`. Пустой `role_id=""` уходит в gRPC и SQL UPDATE — auth_service.UpdateUser затирает все поля.
  Фикс: ea16048 (handler декодирует поля как `*string`/`*bool`; pre-fetch текущего user'а через GetUser, незаполненные поля заполняются текущими значениями; client_id защищён nil-guard'ом в auth_service.UpdateUser строка 466).
  Race-condition между GetUser и UpdateUser теоретически возможна, но окно ms — для админ-эндпоинта приемлемо. Корректная фиксация — FieldMask, инфраструктурный вопрос.

BUG-53: POST /admin/v1/users/:id/reset-2fa → 500 для всех пользователей — Severity: HIGH — Категория: Functional / Schema mismatch
  Шаги: `POST /admin/v1/users/<id>/reset-2fa` → HTTP 500 "Внутренняя ошибка сервера".
  Доказательство (auth-service log): `ERROR: relation "totp_secrets" does not exist (SQLSTATE 42P01)`. UserRepository.ResetTOTP делает `DELETE FROM totp_secrets WHERE user_id = $1`. Такой таблицы нет: TOTP-секрет хранится в `users.totp_secret_encrypted` (миграция 000014), а recovery-коды в `totp_recovery_codes`.
  Фикс: ea16048 (`ResetTOTP` теперь транзакция: `UPDATE users SET totp_secret_encrypted = NULL, totp_enabled = FALSE, totp_verified_at = NULL` + `DELETE FROM totp_recovery_codes WHERE user_id`).

BUG-54: PUT /admin/v1/system/defaults/<key> → 500 для всех валидных значений — Severity: HIGH — Категория: Infrastructure / pgx + jsonb
  Шаги: `PUT /admin/v1/system/defaults/rate_limit_per_second -d '{"value":100}'` → HTTP 500. Страница /admin/settings полностью неработающая — любое сохранение валит 500.
  Доказательство (admin-gateway log после фикса логирования): `ERROR: invalid input syntax for type json (SQLSTATE 22P02)`. `database/sql` + pgx-stdlib передают `[]byte` как `bytea`, и Postgres не умеет неявно приводить bytea→jsonb. Прямой DB UPSERT с text-литералом тоже падает: `column "value" is of type jsonb but expression is of type text`.
  Фикс: ea16048 (двойной фикс — `VALUES ($1, $2::jsonb, ...)` + конверсия `[]byte` → `string` в storage repo, чтобы pgx-stdlib шлёт значение как text, и явный cast отрабатывает). Также добавлено `log.Error().Err(err)` в handler — раньше ошибки repo не логировались, отсюда сложность диагностики.

[Наблюдения / без фикса в этом этапе]

- BUG-50: POST /admin/v1/users с дубликатом email → HTTP 500. auth-service: `ERROR: duplicate key value violates unique constraint "users_username_key" (SQLSTATE 23505)`. Должно быть 409 Conflict с конкретным сообщением. Паттерн BUG-9/13/17/28 — накопительный PR.
- BUG-51: POST /admin/v1/users с несуществующим role_id (валидный UUID) → HTTP 500. auth-service: `users_role_id_fkey violation (SQLSTATE 23503)`. Должно быть 404 "роль не найдена" или 400 "invalid role". Паттерн BUG-9/13/17/28 — накопительный PR.
- max_sub_accounts == 0 в seed: возможно случайный default, не баг — конфигурационный вопрос.
- Password complexity gap: после BUG-49 принимается "12345678". Open вопрос: добавить zxcvbn-style проверку или оставить минимум-8-символов?
- 2FA edge: backup-codes и TOTP secret хранятся раздельно — пара UPDATE/DELETE не атомарна на уровне домена. Проверка transaction-level (BUG-53 фикс) делает консистентным.

[Success Path]
Admin открывает /admin/users → видит список (clamp limit) → создаёт пользователя (валидируются email-формат и password ≥ 8) → редактирует partial (PATCH-like, без затирания других полей) → сбрасывает 2FA (корректные UPDATE+DELETE без ссылки на несуществующую таблицу) → деактивирует. /admin/settings → меняет любую настройку → 204 + значение в БД (после фикса bytea→jsonb cast).

[Recommendations]
1. **Накопительный PR на pgx error-mapping**: уже >11 файлов с паттерном (sender_names, client_configs, billing AddCredits/SetCreditLimit, contracts, contact, operator_templates, webhook FK, **users CreateUser duplicate + role_id FK**). Извлечь helper `mapPgxError(err) → AppError` в `internal/shared` и заменить во всех handler/gRPC слоях. CRITICAL: без этого UX будет регулярно валиться в "Внутренняя ошибка сервера" вместо 400/404/409.
2. **Password complexity**: добавить zxcvbn (`github.com/trustelem/zxcvbn`) или хотя бы regex-проверку на наличие хотя бы одной заглавной/цифры/спецсимвола. Текущая проверка "8+ символов" пускает "12345678".
3. **FieldMask для UpdateUserRequest**: текущий `*authv1.UpdateUserRequest` не различает "не передано" и "пустое". Pre-fetch в admin handler — workaround. Добавить `update_mask: google.protobuf.FieldMask` в proto и перейти на правильную семантику PATCH (этап рефакторинга, spec 017).

[Test Data]
- user `a0cb8e0f-d011-40b7-b6c3-23e138342e2f` создан в TC6 (qa+stage14_1777502797@audit.local), модифицирован TC13 (email=updated_partial_*), деактивирован TC17, реактивирован TC18-после-фикса, 2FA сброшен TC19-после-фикса.
- user `e5ec81dc-91d8-4c05-b137-800360c043d0` создан в TC8 ДО фикса BUG-48 с битым email "not-an-email" — оставлен для регресс-проверки.
- user `c743fc10-427e-4eea-9d24-9cf48d554c01` создан в TC10 ДО фикса BUG-49 с паролем "a" — оставлен для регресс-проверки.
- system_defaults `rate_limit_per_second` менялся на 33→10 в TC29-recheck (восстановлено).
- Регресс-тесты `TestValidateUserEmail/*` (8 cases) и `TestValidateUserPassword/*` (5 cases) в `internal/gateway/admin/handlers/users_test.go`.

Коммиты:
- b368174 docs(audit): этап 14/30 admin users+settings — [IN_PROGRESS]
- ea16048 fix(admin): users/settings — clamp limit, email/password validation, partial PUT, ResetTOTP, jsonb cast [BUG-47/48/49/52/53/54 этап 14/30]

## [DONE] Этап 13/30: Admin — webhooks + billing настройки (admin, fix + Infrastructure + QA full, 2026-04-29) — частичный (API+frontend-types)

[Summary] 25 TC прогнаны (12 webhooks + 13 billing). 8 функциональных багов найдены, 5 исправлены одним PR (BUG-38/39/41/45/46), 3 ушли в наблюдения по design-причинам. Один из фиксов — CRITICAL (BUG-41: эндпоинт пополнения принимал отрицательные суммы и фактически списывал баланс), один HIGH (BUG-38: вся страница /admin/webhooks была сломана из-за рассинхрона полей frontend↔backend).

[BUG LIST]

BUG-38: Frontend WebhooksPage полностью сломан из-за рассинхрона полей с бэкендом — Severity: HIGH — Категория: Functional / Schema mismatch
  Шаги: открыть /admin/webhooks под admin-сессией.
  Ожидалось: список вебхуков загружается, можно создать/удалить.
  Получилось:
    - List при mount шёл без фильтра client_id → backend 400 "client_id обязателен" → пустая таблица + toast "Не удалось загрузить вебхуки".
    - Бэкенд (`internal/services/webhook/grpc/server.go::ListSubscriptions`) возвращает `{subscriptions:[{id,event_types,...}]}`, frontend (`portal-frontend/src/pages/admin/WebhooksPage.tsx::columns`) читал `webhooks[].webhook_id` и `events` — даже при правильном фильтре таблица была бы пустой/битой.
    - Create отправлял поле `events` (frontend), бэкенд требует `event_types` → 400 "event_types обязателен".
    - Delete формировал URL из `deleteWebhook.webhook_id` (undefined) → DELETE /webhooks/undefined.
  Доказательство (API):
    `curl /admin/v1/webhooks` → HTTP 400 "client_id обязателен".
    `curl POST /admin/v1/webhooks -d '{"client_id":"...","url":"https://...","events":["delivered"]}'` → HTTP 400 "event_types обязателен".
  Доказательство (схема): `internal/services/webhook/grpc/server.go::subscriptionToProto` лит `Id`/`EventTypes`; `internal/gateway/admin/handlers/webhooks.go::adminSubscriptionToMap` маппит в `id`/`event_types`. Frontend ожидал `webhook_id`/`events`.
  Фикс: 21ea79d.

BUG-39: Placeholder событий в форме создания вебхука вёл в заблуждение — Severity: LOW — Категория: UX / Documentation drift
  Шаги: открыть модалку создания, посмотреть placeholder поля "События".
  Ожидалось: пример с допустимыми типами `delivered, failed, expired, rejected` (см. `internal/services/webhook/domain/models.go::ValidEventTypes`).
  Получилось: placeholder `message.delivered, message.failed` — таких типов в whitelist нет, ввод по примеру → backend 400 "invalid event type: message.delivered".
  Фикс: 21ea79d (placeholder заменён, добавлен client-side whitelist + validation hint).

BUG-41: POST /admin/v1/billing/clients/:id/credits принимает отрицательные суммы и amount=0 — Severity: CRITICAL — Категория: Security / Billing integrity
  Шаги: `POST /admin/v1/billing/clients/<UUID>/credits -d '{"amount":"-100","description":"x"}'`.
  Ожидалось: 400 "amount должен быть положительным".
  Получилось: HTTP 200, `new_balance: 99900.000000` — баланс уменьшен через эндпоинт пополнения.
  Доказательство (DB до/после):
    SELECT balance FROM accounts WHERE client_id='c0000000-...01' → 100000 → 99900 (списан 100 через POST /credits).
    SELECT amount FROM transactions WHERE id='<txid>' → '-100' с type='credit'.
  Доказательство (handler): `BillingHandlers.AddCreditsRequest.Validate` (до фикса) проверял только `Amount==""`, ни знака, ни формата. gRPC layer (`internal/services/billing/grpc/server.go::AddCredits`) — то же. `s.add(balance, amount)` через `math/big.Float` спокойно прибавляет отрицательное.
  Фикс: 21ea79d (validate через `big.Float.SetString` → отказ при !ok; `IsInf()` → 400; `Sign() <= 0` → 400). Регрессионный тест `TestBillingHandlers/AddCredits/rejects_non-positive_and_malformed_amounts (BUG-41)` с 6 кейсами (-100, 0, +0, abc, Inf, +Inf), AssertNotCalled на gRPC. После фикса: regress curl -100 → 400, 0 → 400, Inf → 400, 5.50 → 200.

BUG-45: Frontend TransactionsTab фильтр transaction_type содержит несуществующее значение 'debit' и не покрывает половину допустимых типов — Severity: MEDIUM — Категория: Functional / Schema drift
  Шаги: открыть /admin/billing → таб "Транзакции" → фильтр "Тип" → выбрать "Списание (debit)".
  Ожидалось: показать только charge/списания.
  Получилось: пустой результат — БД (миграция 000006 CHECK + 000119) принимает `charge|credit|refund|adjustment|transfer_in|transfer_out`, типа `debit` не существует.
  Фикс: 21ea79d (опции фильтра приведены к реальным значениям БД, добавлены charge/refund/adjustment/transfer_in/transfer_out).

BUG-46: GET /admin/v1/billing/balances и /transactions не клампят limit (паттерн BUG-33) — Severity: LOW — Категория: Performance / DoS-vector
  Шаги: `curl /admin/v1/billing/balances?limit=99999`.
  Ожидалось: limit ≤ MAX_LIMIT (200).
  Получилось: HTTP 200 с `"limit":99999` — параметр уехал в gRPC и SQL без ограничения.
  Фикс: 21ea79d (helper `clampPagination(limit, offset, def, max)` в `billing.go`, применён в `GetTransactionHistory` и `ListBalances`; `billingMaxListLimit = 200`).

[Наблюдения / без фикса в этом этапе]

- BUG-40: GET /admin/v1/billing/clients/<несуществующий UUID>/balance → HTTP 200 с `balance:"0"`, не 404. Полу-фича: account создаётся при следующем addCredits (см. `BillingService.AddCredits` lines 96-109). Решение пользователя — оставить как есть или 404.
- BUG-42: AddCredits с amount=0 создавал noop-транзакцию (LOW). После фикса BUG-41 закрыто как побочный эффект — `Sign() <= 0` отклоняет ноль. Дыра остаётся в gRPC service (если кто-то вызовет напрямую).
- pgx/SQL ошибки → 500 паттерн (BUG-9/13/17/28): TC8 (webhook FK violation на несуществующий client_id) → 500 INTERNAL вместо 404; TC23 (AddCredits invalid amount string "abc" — был замаскирован, но gRPC service всё ещё может упасть так), TC24 (SetCreditLimit "abc" → 500). Накопительный follow-up.
- operation_kind фильтр в TransactionsTab отсутствует (миграция 121, MVP-feedback №5 dcbab7f добавила колонку `operation_kind`, но frontend не экспонирует). Feature gap, отдельный PR.
- ListSubscriptions требует client_id — admin не имеет глобального view "все вебхуки". В текущем фиксе это закрыто заглушкой "Выберите клиента". Архитектурный вопрос: нужен ли админский global view?
- ListSubscriptions в gRPC требует client_id (см. `internal/services/webhook/grpc/server.go:77-78`), а UpdateSubscription тоже требует client_id в body — означает, что admin должен сначала найти владельца webhook'а. UX-неудобство, но не баг.

[Success Path]
Admin открывает /admin/webhooks → выбирает клиента в фильтре → видит его подписки → создаёт новую (URL https://..., events delivered,failed) → запись появляется → может удалить через rowAction. На /admin/billing → таб "Транзакции" → фильтр по типу charge/refund показывает соответствующие записи. Начисление средств: amount > 0 принимается, amount ≤ 0 / Inf / non-numeric → 400 на уровне handler, не доходит до gRPC.

[Recommendations]
1. **Накопительный PR на pgx error-mapping**: уже >9 файлов с паттерном (sender_names части, client_configs, billing AddCredits/SetCreditLimit, contracts, contact, operator_templates, webhook FK violation). Извлечь helper `mapPgxError(err) → AppError` в `internal/shared` и заменить во всех handler/gRPC слоях. Без этого UX будет регулярно валиться в "Внутренняя ошибка сервера" вместо 400/404.
2. **Operation_kind фильтр в TransactionsTab**: добавить дополнительный select с values `message|sender_name|operator_template|operator_tariff|other`, передавать через query — backend gRPC `GetTransactionHistoryRequest` нужно расширить полем (сейчас только `transaction_type`).
3. **Архитектурное решение по admin global view**: либо новый gRPC method `ListAllSubscriptions` (с pagination + фильтрами), либо принять текущую "выбери клиента" как намеренную ownership-модель. Документировать в spec.

[Test Data]
- webhook id 0c216be9-e51a-4ef7-8c81-03285ec9ab1d у Demo-Main (создан в TC4, удалён в TC11).
- Два мусорных теста-транзакций amount=-100/0 на c0000000-...01 от TC21/22 ДО фикса BUG-41 (баланс был восстановлен через TC happy-pos +5.50 → итого balance ≈ 99905.50).
- Регрессионный тест `TestBillingHandlers/AddCredits/rejects_non-positive_and_malformed_amounts (BUG-41)`.

Коммиты:
- c18e4c3 docs(audit): этап 13/30 admin webhooks+billing — [IN_PROGRESS]
- 21ea79d fix(admin): webhook page schema mismatch + billing AddCredits non-positive amount + clamp limits [BUG-38/39/41/45/46 этап 13/30]

## [DONE] Этап 12/30: Admin — operator-templates (admin, fix + Infrastructure + QA full, 2026-04-29) — частичный (API-only)

[Summary] 23 TC прогнаны (включая регрессии после фикса), 4 функциональных бага найдены и исправлены одним PR (BUG-35/36/37 + BUG-32-pattern). Один из них (BUG-35) — HIGH severity, ломал UI edit любого шаблона без sender_name. System-wide паттерн BUG-9/13/17/28 (pgx errors → 500 вместо 400/404) подтверждён здесь массово (TC12.7/8/9/12/15b/c/21 — 7 кейсов в одном handler) — НЕ фиксим в одиночку, накопительный follow-up.

[BUG LIST]

BUG-35: UpdateOperatorTemplate с пустым sender_name_id падал 500 (SQLSTATE 22P02) — Severity: HIGH — Категория: Logic Gap / UI-blocker
  Шаги: PUT /admin/v1/operator-templates/{id} с body `{"name":"x"}` (или любой partial без sender_name_id) → 500 INTERNAL_ERROR. В UI: openEdit пишет `sender_name_id: item.sender_name_id || ''`, при шаблоне без привязки фронт всегда отправлял пустую строку → 500. Edit любого шаблона без sender_name был полностью сломан через UI.
  Корневая причина: SQL `sender_name_id = CASE WHEN $3::text = '' THEN sender_name_id ELSE $3::uuid END` — pgx определял тип $3 по ELSE-ветке как UUID и валидировал параметр на bind-стадии. Пустая строка не проходила UUID parse → 22P02 ДО исполнения CASE.
  Получалось: 500 Internal на любой partial PUT, и через UI на шаблоны без sender_name.
  Доказательство: TC12.13 partial → 500; TC12.16 update non-existent → тоже 500 (тот же 22P02, не доходит до RowsAffected=0). Логи admin-gateway: `invalid input syntax for type uuid: "" (SQLSTATE 22P02)`.
  Фикс: commit dce420f — pre-обработка в Go (`var senderNameIDArg interface{}; if req.SenderNameID != "" { senderNameIDArg = req.SenderNameID }`), SQL → `COALESCE($3::uuid, sender_name_id)`. Известное ограничение: нельзя сбросить sender_name_id обратно в NULL — это отдельная фича (нужен patch-style API с явным sentinel), не введено фиксом.

BUG-36: Approve/Reject/RequestRevision misleading 404 при существующем шаблоне без sender_name_id — Severity: LOW — Категория: Logic Gap / UX
  Шаги: POST /admin/v1/operator-templates/{id}/approve для шаблона который существует но без sender_name_id → 404 "шаблон не найден". Запутывает: шаблон есть, но кажется удалённым.
  Корневая причина: SELECT с INNER JOIN sender_names — NULL sender_name_id даёт пустой набор → ErrNoRows → 404.
  Фикс: commit dce420f — helper `loadModerationContext` с LEFT JOIN, разделяет (found, eligible, status). 404 только при реальном отсутствии шаблона, 400 "модерация возможна только для шаблонов прямых клиентов с привязкой к sender_name" если шаблон есть но не подходит.

BUG-37: List limit=-1 → 500, limit=99999 без clamp = unbounded — Severity: LOW/MED — Категория: Logic Gap / DoS Risk (BUG-33 паттерн)
  Шаги: GET /admin/v1/operator-templates?limit=-1 → 500 INTERNAL_ERROR (`LIMIT -1` syntax error от Postgres); ?limit=99999 → 200 без clamp (на пустой таблице 0 строк, в проде потенциально вся таблица).
  Фикс: commit dce420f — clamp в начале handler (default 50, max 200, offset≥0). Идентично паттерну BUG-33 для sender-names. Inconsistency с другими admin модулями (sender_names: max 200; common parsePagination: max 500) сохранена — наблюдение.

BUG-32-pattern: Reject/RequestRevision принимали пустой note → audit без причины — Severity: MED — Категория: Logic Gap / Audit Integrity
  Тот же паттерн что BUG-32 sender-names (этап 11, commit 0bbc41c) — рекуррентный для всех modal-with-reason endpoints в админке.
  Фикс: commit dce420f — `validateModeratorNote` (TrimSpace + len 3..500). Approve как и раньше не требует note (consistency со sender-names).

[Наблюдения, не фиксим в одиночку]

- BUG-9/13/17/28 system-wide паттерн **подтверждён массово** в operator_templates handler (отдельные TC):
  - TC12.7: Create с invalid UUID для operator_id → 500 (22P02, должен 400).
  - TC12.8: Create с non-existent operator_id (FK violation) → 500 (23503, должен 400/422).
  - TC12.9: Create с invalid status → 500 (23514 CHECK violation, должен 400).
  - TC12.12: GET с invalid UUID → 500 (22P02, должен 400).
  - TC12.15b/c: Update с FK/UUID violations → 500.
  - TC12.21: List filter с invalid operator_id UUID → 500.
  Корень: handler использует `database/sql` напрямую без error mapping. Накопилось ≥7 файлов с этим паттерном — отдельный PR через `pgxutil.MapErrorToHTTP`-helper или подобное. **НЕ ФИКСИМ ЗДЕСЬ.**

- BUG-16-pattern (jsonb crash): operator_templates.go использует `string(varsJSON)`, не `[]byte` — **этот код безопасен**. Паттерн НЕ повторяется.

- TOCTOU race в loadModerationContext + UPDATE: SELECT и UPDATE раздельны, два конкурентных reject могут оба пройти проверку submitted и оба сделать UPDATE — второй перетрёт moderator_note первого. Был и в старом коде, не регрессия. Reviewer отметил для техдолга. Фикс: один UPDATE с WHERE moderation_status='submitted' + RETURNING + проверка RowsAffected.

- len() в validateModeratorNote считает байты, не руны — кириллица 2 символа = 4 байта проходит (минимум 3 байта). Тот же минор-баг что зафиксирован в этапе 11 sender-names. Накопительный follow-up для всех админ-handlers с reason validation.

- Spec-drift риск: `loadModerationContext` фильтрует `c.parent_client_id IS NULL` — только direct clients, sub-account шаблоны нельзя модерировать. Поведение унаследовано из старого кода. spec 010-sender-names-templates явно не упоминает parent_client_id — поведение реализации, не AC. Открытый вопрос: должны ли модерироваться sub-account templates? Не блокер этапа, требует решения.

- BUG-34 паттерн (admin history endpoint): operator_templates тоже не имеет admin GET /history endpoint для transition-логи. Service-слой operator_template_service.go может уже иметь GetOperatorTemplateHistoryAdmin (не проверял в этом этапе). Тот же follow-up класса что BUG-34.

[Open для решения пользователя]

- (a) BUG-9/13/17/28 system-wide error mapping — когда фиксим? Накопилось 7 файлов: aggregator, sender_names (часть мест), client_configs, billing, contracts, contact, operator_templates. Один PR с helper pgxutil.MapError или per-handler patches.
- (b) Sub-account модерация operator-templates — фича или явный block? (поведение в коде = block, но не в spec).
- (c) UI: невозможность сбросить sender_name_id обратно в NULL после фикса BUG-35. Нужен patch-style API с sentinel "<unset>" или отдельный endpoint /clear-sender-name. Не критично — UI пока этот сценарий не предлагает (Select имеет "Без имени отправителя" → пустая строка → теперь сохранит текущее значение, не сбросит).

[Success Path] Admin открывает /admin/operator-templates, видит таблицу с фильтрами по оператору и статусу. Создаёт шаблон, выбирает оператора (обязательно), опционально sender_name, пишет body с переменными вида {code}. Редактирование partial-friendly. Approve/reject — только для шаблонов прямых клиентов с привязанным sender_name, status=submitted. Reject/request-revision требуют комментарий 3..500 символов.

[Recommendations]
1. **Срочно (next sprint)**: системный фикс error-mapping pgx → http codes. Накопилось ≥7 файлов, BUG-9/13/17/28/часть BUG-Update — это всё один корень, в проде = 500-storm для любого валидного "плохого" запроса (что админ может сделать руками или через интеграцию).
2. **MED**: TOCTOU race в moderation transitions — один UPDATE с условием статуса вместо SELECT+UPDATE.
3. **LOW**: utf8.RuneCountInString для всех reason/note валидаций по платформе. Сейчас минимум 3 байта = 1 символ кириллицы — теоретически приёмлемо, но inconsistent.

[Test Data]
- 4 шаблона создано-удалено в ходе TC: 3 на этапе обнаружения багов, 1 на регрессии. Все clean-up через DELETE 204. operator_templates таблица в текущем состоянии: 0 строк (база чиста).
- Использован operator_id 10000000-0000-0000-0000-000000000001 (МТС) — seed-данные, без модификации.
- sender_names пуст в БД на dev стенде — TC для approve happy path не прогонялись (требуется создание sender_name + перевод шаблона в submitted; вне scope этапа). Регрессия покрывает только negative path approve/reject.

## [DONE] Этап 11/30: Admin — sender-names review/approve

## [DONE] Этап 11/30: Admin — sender-names review/approve (admin, fix + Infrastructure + QA full, 2026-04-29) — частичный (API-only)

[Summary] 27 TC прогнаны (включая регрессии после фикса), 2 функциональных бага найдены и исправлены одним PR (BUG-32/33), 1 follow-up зафиксирован (BUG-34 missing admin history endpoint). Error-mapping в этом сервисе **уже корректен** (NotFound/InvalidArgument/AlreadyExists/FailedPrecondition мапятся правильно через mapError в gRPC handler) — паттерн BUG-9/13/17/28 здесь НЕ повторяется.

[BUG LIST]

BUG-32: RejectSenderName/DeactivateSenderName принимали пустой reason → audit-trail без причины — Severity: MED — Категория: Logic Gap / Audit Integrity
  Шаги: POST /admin/v1/sender-names/{id}/reject с {"reason":""} → 200, sender_name переходит в rejected, sender_name_status_history строка с comment="". Frontend типизирует reason как обязательный (admin.ts:738 `reject(id, reason: string)`), но валидации нет — баг проходит при curl/API direct.
  Получалось: история модерации без причины, клиент не видит почему отклонён, audit-trail неинформативен.
  Корневая причина: handler RejectSenderName/DeactivateSenderName делал Decode → передавал req.Reason as-is в gRPC, который писал в БД и history.
  Доказательство: TC11.9 reject empty → 200; DB.rejection_reason="", history.comment="". После фикса (TC11.9-RE) → 400 с понятным сообщением.
  Фикс: commit 0bbc41c — TrimSpace + проверка len 3..500. Reject и Deactivate имеют идентичную логику, разные сообщения ("Причина отказа..." / "Причина деактивации..."). Константы senderNameMinReasonLen=3, senderNameMaxReasonLen=500.
  Минор-замечание reviewer (не блокер, в follow-up): len() считает байты, не руны — для русского минимум 3 байта = 1.5 символа. Стоит мигрировать на utf8.RuneCountInString при следующей итерации.

BUG-33: ListAllSenderNames пропускал limit=-1 → unbounded dump — Severity: LOW — Категория: Logic Gap / DoS Risk
  Шаги: GET /admin/v1/sender-names?limit=-1 → 200, отдан полный размер таблицы (на стенде 6 строк, в проде потенциально миллионы при росте). `parseIntParam` подменяет default только при ParseError, отрицательное число валидно как Int → транзит до SQL → LIMIT -1 = unbounded.
  Получалось: admin может неинтентом затащить весь sender_names (или DoS-вектор через скрипт).
  Фикс: commit 0bbc41c — clamping в начале handler: `limit ≤ 0 → 20; limit > 200 → 200; offset < 0 → 0`. Константа senderNameMaxListLimit=200.
  Reviewer flagged inconsistency: общий parsePagination (common.go:46) использует max 500 для per_page, новый код — 200 для limit. Это inconsistency существующего шаблона admin pagination, не введена этим PR. Зафиксирована как наблюдение для будущего spec.

BUG-34 (наблюдение, требует feature work): admin не может посмотреть transition history sender-name через API — Severity: MED — Категория: UX / Feature Gap
  Шаги: GET /admin/v1/sender-names/{id}/history → 404 (роут не зарегистрирован).
  Корневая причина: в service-слое `GetSenderNameHistoryAdmin(ctx, id, limit, offset)` уже существует (sender_name_service.go:219) и возвращает []SenderNameStatusHistory. Но gRPC `GetSenderNameHistory` требует ClientId (sender_name_handler.go:151) — admin endpoint без ClientId не существует. Соответственно admin handler в gateway отсутствует и роут не зарегистрирован.
  Получалось: админ не видит кто/когда/почему менял статус sender_name (только текущий rejection_reason). При спорной модерации нельзя восстановить хронологию.
  Фикс: вынесен в follow-up — требует:
    (a) добавить gRPC метод `GetSenderNameHistoryAdmin` (или флаг in `GetSenderNameHistory` для admin-режима — путь как `GetSenderName(ClientId="")`)
    (b) handler в admin gateway → роут `/admin/v1/sender-names/{id}/history`
    (c) frontend integration (детальная страница sender_name с историей)
  Я склоняюсь к (a) flag-вариант (как сделано в GetSenderName) — минимальная инвазивность.

[Test Coverage]
PASS: TC11.1 list all → 200, TC11.2 list filter status=pending → 200, TC11.3 filter client_id → 200, TC11.4 filter name_query → 200, TC11.5/5b/5c RBAC unauth/user→401/403, TC11.6 GET sender-name → 200 wrap {sender_name}, TC11.7 approve happy → 200 + reviewer_id+reviewed_at + history записан (UI=API=DB), TC11.8 reject with reason → 200 + DB consistency, TC11.10 reject NO body → 400 "Неверный формат запроса", TC11.11 approve already-approved → 400 "invalid status transition", TC11.12 reject already-rejected → 400, TC11.13 deactivate happy approved→deactivated → 200, TC11.14 deactivate already-deactivated → 400, TC11.15 reject approved (skip pending) → 400 (transitions matrix корректна), TC11.16 reject ASCII reason → 200, TC11.17 reject UTF-8 кириллица через @file → 200 + DB hex valid UTF-8, TC11.18 operator-registrations empty → 200 {registrations:[]}, TC11.19 invalid uuid format → 400, TC11.20 GET non-existent → 404, TC11.21 approve non-existent → 404, TC11.22 limit=2 → returns 2/total=6, TC11.23 offset=10 out-of-range → 0 items, TC11.27 invalid status filter (silent ignore) → empty.

REGRESSION после фикса:
PASS: TC11.9-RE reject empty → 400, TC11.9b-RE only spaces → 400, TC11.9c-RE 1-char → 400, TC11.9d-RE >500 → 400, TC11.9e-RE valid → 200, TC11.13-RE deactivate empty → 400, TC11.24-RE limit=-1 → clamp 20, TC11.24b-RE limit=99999 → clamp 200, TC11.7-REGR approve happy still works.
FAIL/наблюдения: TC11.25 admin history endpoint (BUG-34, отсутствует).

OBSERVATIONS:
- Error-mapping в этом сервисе УЖЕ корректен (gRPC mapError на NotFound/AlreadyExists/InvalidArgument/FailedPrecondition + GRPCError в shared/response). Не повторяется паттерн BUG-9/13/17/28.
- State transition matrix реализована правильно (`pending→{approved,rejected}, rejected→pending, approved→deactivated, deactivated→{}`). Все нелегальные переходы → 400 "invalid status transition for sender name".
- Response schema admin: list содержит plain объекты с rejection_reason="" (не null) и reviewer_id="" (не null) — frontend optional `?:` это терпит, но конвенция отличается от reviewed_at:null (timestamp). Не критично.
- operator-registrations endpoint silent-fail (200 + empty) при ошибке tarification-service — подавляет диагностику. Не блокер, но в проде admin может не понять что данных нет vs ошибка.
- adminSenderNamesApi.approve/reject/deactivate в TS типизирован как `Promise<AdminSenderNameInfo>` без обёртки {sender_name} — handler возвращает bare adminSenderNameToJSON(...). Это согласовано (отличается от GET, который оборачивает).

[Success Path] Admin: GET /admin/v1/sender-names → list pending модерации с фильтрами (status, client_id, name_query); POST /{id}/approve → 200, sender_name.status=approved + reviewer_id + reviewed_at + history-запись; POST /{id}/reject c reason {3..500 chars} → 200, sender_name.status=rejected + rejection_reason + history; approved sender_name можно деактивировать через /{id}/deactivate с reason. Все state transitions через domain.allowedTransitions matrix; нелегальные → 400.

[Recommendations]
1. (MED) BUG-34 — добавить admin history endpoint (требует feature work, не одно-строчный fix). См. план в BUG-34 описании.
2. (LOW) Reviewer-flagged техдолг: мигрировать `len(reason)` → `utf8.RuneCountInString(reason)` для консистентности UI/API сообщений с человеческим восприятием символов.
3. (LOW) Унифицировать pagination conventions для admin: parsePagination max=500, sender-names limit max=200 — выбрать один максимум для всех list endpoint'ов и зафиксировать в shared helper.
4. (LOW) Frontend ModerationPage: добавить min={3} max={500} validation на поле reason (sync с backend). Пользователь сейчас видит ошибку только после клика.
5. (LOW) Регрессионный тест на reject с пустым reason / deactivate empty — defence-in-depth.

[Test Data] Создан и удалён seed: 6 sender_names (aa000000-...000001..000006) для Demo-Main и Demo-Light. После прогона DELETE'нут вместе с history (DELETE 9 history rows, DELETE 6 sender_names). Состояние БД восстановлено: count=0.

[Commits этапа]
- bf46e0b docs(audit): этап 11/30 admin sender-names — [IN_PROGRESS]
- 0bbc41c fix(sender-names): валидация reason + clamp limit в admin handler [BUG-32/33]
- (этот) docs(audit): этап 11/30 — [DONE] частичный



## [DONE] Этап 10/30: Admin — aggregators (sub-accounts UI) (admin, fix + Infrastructure + QA full, 2026-04-29) — частичный (API-only)

[Summary] 20 TC прогнаны, 4 функциональных бага найдены и исправлены одним PR (BUG-25/26/29/30), 3 follow-up зафиксированы (BUG-27 архитектурный — overlap, BUG-28 паттерн error-mapping, BUG-31 missing UI route).

[BUG LIST]

BUG-25: Admin aggregator-quotas API возвращал CamelCase JSON (нет json-тегов в domain.AggregatorQuota) — Severity: CRITICAL — Категория: Reliability Risk / Spec Drift
  Шаги: GET /admin/v1/aggregators/:id/quotas → response `{"quotas":[{"ID":"...","AggregatorID":"...","PeriodStart":"...","SegmentLimit":1000000,...}]}`. Frontend AggregatorQuotasPage.tsx читает row.segment_limit.toLocaleString() → TypeError на undefined. UI визуально пуст / падает.
  Корневая причина: `domain.AggregatorQuota` поля без `json:"..."` тегов; encoding/json по умолчанию использует имена полей. То же самое в portal /aggregator/quotas/spending (GetQuotaSpending) — отдавал bare domain slice.
  Доказательство: до фикса list возвращал ID/AggregatorID/PeriodStart; после — quota_id/aggregator_id/period_start (см. TC10.1-RE).
  Фикс: commit 60b4e42 — handler-level DTO `quotaResponse` (admin) и `quotaSummary` (portal) с snake_case json-тегами; helpers `toQuotaResponse(q)` / `toQuotaResponses(qs)`. Domain не тронут (доменный слой не должен знать про JSON wire format).

BUG-26: UpdateQuota игнорировал aggregator_id из URL path — cross-tenant дыра — Severity: HIGH — Категория: Logic Gap / Access Control
  Шаги: PUT /admin/v1/aggregators/<random_uuid>/quotas/<real_quota_id> с {"segment_limit":777} → 200, реальная квота другого aggregator обновлена.
  Корневая причина: handler парсил только `quota_id`, не сверял `aggregator_id` с владельцем квоты. Admin-only API, но логически дыра.
  Фикс: commit 60b4e42 — `QuotaService.UpdateQuota(aggregatorID, quotaID, ...)`; ownership-mismatch и not-found возвращают единый sentinel `ErrQuotaNotFound` (не утекает existence информация); handler мапит → 404. После фикса IDOR-PUT → 404 NOT_FOUND, квота не меняется (TC10.17-RE).

BUG-29: CreateQuota/UpdateQuota не оборачивали response в `{quota: ...}` — нарушение контракта с frontend — Severity: MED — Категория: Spec Drift
  Шаги: фронт `aggregatorQuotasApi.create()` типизирован как `Promise<{quota: AggregatorQuota}>` (admin.ts:961), но handler возвращал bare quota. После create UI делает refetch активной квоты, поэтому pad-эффект ослаблен — но контракт нарушен и getActive/portal::GetMyQuota уже использовали обёртку → асимметрия.
  Фикс: commit 60b4e42 — Create/Update оборачивают в `{"quota": toQuotaResponse(q)}`.

BUG-30: Frontend интерфейс ожидает `active: boolean`, поля нет ни в БД, ни в domain — Severity: MED — Категория: Spec Drift
  Шаги: AggregatorQuotasPage отображает Badge `active ? "Активна" : "Архив"` — все строки всегда "Архив" (undefined → false).
  Корневая причина: `aggregator_quotas` schema (`\d aggregator_quotas`) не имеет колонки `active`. Это вычисляемое свойство по периоду.
  Фикс: commit 60b4e42 — DTO добавляет computed `Active = !period_start.After(now) && period_end.After(now)` (та же семантика, что в `repo.GetActive` SQL: `period_start <= NOW() AND period_end > NOW()`). Verified: квоты с period_start=2026-05-01 (today=2026-04-29) → active=false; happy-path active=true для сегодняшней квоты.

BUG-27 (наблюдение, требует решения): нет проверки overlap периодов на CreateQuota — Severity: HIGH — Категория: Logic Gap / Architecture
  Шаги: создать (2026-05-01..2026-05-31) и (2026-05-15..2026-06-15) для одного aggregator → 201/201, обе попадают под GetActive в момент пересечения.
  Корневая причина: UNIQUE constraint только по (aggregator_id, period_start), нет EXCLUDE USING gist по диапазону. ConsumeQuota.GetActive берёт LIMIT 1 ORDER BY period_start DESC — поведение детерминировано, но недокументировано (выбирается **более поздняя** из перекрывающихся).
  Фикс: вынесен в follow-up — нужно policy-решение от пользователя:
    (a) запретить overlap на DB-уровне (миграция с EXCLUDE constraint, требует ENABLE btree_gist + handler 409 на нарушение)
    (b) разрешить overlap явно, документировать "последняя побеждает", возможно добавить флаг archive в UI
  Я склоняюсь к (a) — overlap = баг ввода, не feature.

BUG-28 (повторение системного паттерна BUG-9/13/17): CHECK violations → 500 — Severity: MED — Категория: Logic Gap
  Шаги: POST с segment_limit=-1 / overage_rate=-0.01 / period_end<=period_start / duplicate (aggregator_id, period_start) → все 500 INTERNAL_ERROR.
  Ожидалось: 400 Bad Request (CHECK) / 409 Conflict (UNIQUE) с понятными сообщениями.
  Корневая причина: handler `respondError(w, shared.ErrInternalServer(...))` на любую ошибку из service. Применять fix как часть BUG-9 system-wide error-mapping PR (pgErrorToHTTPCode helper, добавить в shared).
  Фикс: вынесен в follow-up.

BUG-31 (наблюдение): нет UI-страницы `/admin/aggregators` (список агрегаторов) — Severity: MED — Категория: UX / Feature gap
  Шаги: grep `/admin/aggregators` в App.tsx → нет matches; страница `AggregatorQuotasPage` зарегистрирована только nested как `/admin/aggregators/:aggregatorId/quotas`. UI доступен только если ввести URL вручную; нет navigation entry в AdminLayout.
  Получилось: admin не может выйти на список агрегаторов через UI; функциональность (CRUD квот) присутствует на backend и в page-компоненте, но UI-вход отсутствует.
  Фикс: вынесен в follow-up — требует страницы /admin/aggregators с list агрегаторов (clients где is_reseller=true) + ссылки на quotas-страницу.

[Test Coverage]
PASS: TC10.1 list initial empty → 200 (после фикса snake_case), TC10.2 unauth → 401, TC10.3 active none → {quota:null}, TC10.4 user→403, TC10.5 invalid agg UUID → 400, TC10.6 list non-existent agg → 200 empty (расценивается норм для admin), TC10.7 create happy → 201 + DTO + DB consistency, TC10.8 list after create → 200, TC10.9 active before period → null, TC10.14 invalid date format → 400, TC10.17-RE IDOR → 404 (после BUG-26 фикса), TC10.18-RE non-existent quota_id → 404 (после BUG-26 фикса), TC10.19-RE update happy → 200 + wrap {quota}, TC10.20 invalid quota_id format → 400.
FAIL/наблюдения: TC10.10-10.13 (BUG-28 паттерн), TC10.15 overlap (BUG-27), TC10.16 dup period_start (BUG-28).

[Success Path] Admin: GET /admin/v1/aggregators/{aggID}/quotas → list истории; POST /admin/v1/aggregators/{aggID}/quotas с {period_start, period_end, segment_limit>0, overage_rate≥0, currency=RUB, auto_renew} → 201 {quota:{...snake_case...}}; PUT /admin/v1/aggregators/{aggID}/quotas/{quotaID} только с segment_limit/overage_rate/auto_renew → 200 {quota:{...}}; ownership-cross-tenant → 404. Активная квота — `period_start <= today < period_end`.

[Recommendations]
1. (HIGH) BUG-27 overlap policy — нужно решение пользователя; рекомендую DB-EXCLUDE + 409 mapping (вариант a). До решения один aggregator может иметь несколько "active" квот с детерминированным but non-obvious tie-break.
2. (HIGH) BUG-31 — добавить страницу `/admin/aggregators` с listings (clients WHERE is_reseller=true) + переход к quotas. Без UI gateway админ не может найти эту функциональность.
3. (MED) BUG-28 включить в общий error-mapping PR с BUG-9/13/17. У aggregator_quotas четыре CHECK constraints + один UNIQUE — все возвращают 500.
4. (LOW) Регрессионный e2e-тест: round-trip POST→GET с проверкой ключей snake_case (предотвращает повторение BUG-25). Можно добавить в integration_test.

[Test Data] Создано/удалено: 2 квоты для Demo-Reseller (260dbb27..., 338bb7e3...) → DELETE'нуты SQL'ом после прогона.

[Commits этапа]
- d094734 docs(audit): этап 10/30 admin aggregators — [IN_PROGRESS]
- 60b4e42 fix(aggregator-quotas): wire DTO с json-тегами + ownership check в UpdateQuota [BUG-25/26/29/30]
- (этот) docs(audit): этап 10/30 — [DONE] частичный



## [DONE] Этап 9/30: Admin — clients (CRUD + блокировка) (admin, fix + Infrastructure + QA full, 2026-04-29) — частичный (API-only)

[Summary] 13 TC прогнаны, 1 CRITICAL bug (BUG-21) найден и исправлен, 3 follow-up зафиксированы (BUG-22 LOW, BUG-23 HIGH security/policy, BUG-24 MED). Также расширен scope BUG-16 до 7 файлов после доп. grep'a reviewer'ом.

[BUG LIST]

BUG-21: client_configs UpdateRateLimits падал на jsonb (SQLSTATE 22P02) — Severity: CRITICAL — Категория: Reliability Risk
  Шаги: PUT /admin/v1/clients/:id/rate-limits для клиента без записи в client_configs (Demo-Light в seed) → 500.
  Корневая причина: тот же паттерн что BUG-15 (HLR config). UpdateRateLimits через ErrConfigNotFound фоллбэчится в Create, который пишет json.RawMessage напрямую в jsonb колонку → bytea без implicit cast.
  Доказательство: client-service log `ERROR: invalid input syntax for type json (SQLSTATE 22P02)`
  Фикс: commit cec04b8 — `string(config.Settings)` в Create+Update. Verify: после фикса 200, не 500.

BUG-22 (наблюдение): negative rate-limit принимается без валидации — Severity: LOW — Категория: Logic Gap
  Шаги: PUT /admin/v1/clients/:id/rate-limits с rate_limit_per_second=-10 → 200 OK
  Ожидалось: 400 с понятным сообщением "rate_limit must be ≥0"
  Фикс: вынесен в follow-up (handler-level enum/range validation, аналогично BUG-10 для bind_type)

BUG-23: блокировка клиента (active=false) не блокирует login его users — Severity: HIGH — Категория: Access Control
  Шаги: 1) admin делает PUT /admin/v1/clients/<reseller_id> active=false 2) пользователь reseller@demo.local логинится → 200 с user.active=true
  Ожидалось: блокировка clients.active=false должна автоматически отбрасывать login users этого клиента (либо явно через FOREIGN KEY check, либо middleware). Иначе пользователь продолжает видеть UI с фоном-битыми операциями.
  Получилось: login успешен, юзер видит портал, но любой POST/PUT, требующий active client, упадёт.
  Корневая причина: auth/login проверяет только `users.active`, не `clients.active`. Спецификация бизнес-логики: должен ли клиент блокироваться вместе с user — неясно.
  Фикс: вынесен в follow-up — требует дизайн-решения от пользователя (cascade блокировка users при clients.active=false vs middleware-проверка clients.active в каждом endpoint).

BUG-24: PUT /clients/:id/rate-limits возвращает success но values НЕ персистятся — Severity: MED — Категория: Logic Gap
  Шаги: PUT с {rate_limit_per_second:99,...} → 200 {"success":true}; SELECT в client_configs показывает rate_limit_per_second=0; SELECT в clients показывает старые legacy-значения 100/6000.
  Ожидалось: переданные значения сохраняются в client_configs.
  Получилось: SQL UPDATE действительно срабатывает (updated_at changes), но значения = 0. Видимо handler/gRPC mapping в admin-gateway или client-service теряет fields из request body.
  Корневая причина: предположительно proto-mapping в admin/handlers/clients.go или client-service gRPC server.UpdateRateLimits — нужен trace.
  Фикс: вынесен в follow-up (требует deep-dive в gRPC contract mapping)

[Test Coverage]
PASS: 9.1 list clients → 200 (видны клиенты из этапов 2 + demo), 9.2 RBAC user→403/unauth→401, 9.3 get client → 200, 9.4 update contact_person → 200 + DB consistency, 9.5 block (active=false) → 200 + DB confirm, 9.6 unblock → 200, 9.7 freeze billing → 200 (frozen_at returned), 9.8 unfreeze → 200, 9.9 update rate-limits → 200 (после фикса BUG-21), 9.11 non-existent get → 404, 9.12 non-existent update → 404 "client not found".
FAIL/наблюдения: 9.10 negative rate (BUG-22), 9.13 blocked client login (BUG-23), 9.14 rate-limits not persisting (BUG-24).

OBSERVATIONS:
- В DB две таблицы с rate_limit_*: `clients.rate_limit_*` (legacy, видны 100/6000 для Demo-Light из seed) и `client_configs.rate_limit_*` (canonical из 000003). Раздвоение — потенциальный источник rasism, синхронизация неясна. Зафиксирую как **наблюдение для архитектора**.
- Freeze/unfreeze billing работает корректно и независимо от clients.active — это намеренно (биллинг-блок без полной блокировки клиента). UI должен это отображать.

[Recommendations]
1. (HIGH) BUG-23: дизайн-решение по cascade блокировки. Требует ответа: следует ли блокировать users автоматически при clients.active=false?
2. (MED) BUG-24: deep-dive в gRPC client-service.UpdateRateLimits — trace handler params до SQL. Тривиальный регрессион-тест: PUT → GET, expect-equal.
3. (LOW) Унифицировать rate_limit fields: либо удалить из clients (legacy), либо синхронизировать.
4. (MED) Расширить BUG-16 follow-up до 7 файлов: client/config_repository (этим коммитом), contact, provider, campaign — все имеют тот же jsonb-паттерн (источник: review at agent af9e89a0efa4e5798).

[Test Data] Все правки на Demo-Light (rate_limits/contact_person) и Demo-Reseller (active toggle) восстановлены.

[Commits этапа]
- (lock-коммит, см. предыдущий — этап 9 IN_PROGRESS)
- cec04b8 fix(client): передавать settings как string в client_configs jsonb [BUG-21]
- (этот) docs(audit): этап 9/30 — [DONE] частичный



## [DONE] Этап 8/30: Admin — legal-entities + contracts (admin, fix + Infrastructure + QA full, 2026-04-29) — частичный (API-only)

[Summary] 10 TC прогнаны (PASS после fix), 1 HIGH bug найден и исправлен (BUG-18 — JOIN typo, ещё 2 латентных в detalization), 2 минор как follow-up.

[BUG LIST]

BUG-18: SQL JOIN typo `cl.client_id` вместо `cl.id` — Severity: HIGH — Категория: Logic Gap / Spec Drift
  Шаги: GET /admin/v1/contracts → 500
  Корневая причина: `clients.id` это PK; FK-имя `client_id` существует только на referencing-таблицах (contracts, messages, users). Три идентичных typo в `contracts.go:94` (active failure), `detalization.go:96` и `:197` (latent — endpoint deprecated в MVP-feedback №10).
  Доказательство: backend log `ERROR: column cl.client_id does not exist (SQLSTATE 42703)`
  Фикс: commit 38c0d42 — три однобуквенные правки (cl.client_id → cl.id). Verify: list contracts → 200, contract creation+JOIN корректно показывают client_name.

BUG-19 (наблюдение): PUT /admin/v1/contracts/:id с partial body вызывает SQLSTATE 22P02 на legal_entity_id="" — Severity: MED — Категория: Logic Gap
  Шаги: PUT /admin/v1/contracts/:id с body {"status":"terminated"} → 500
  Корневая причина: handler заполняет все поля contracts в UPDATE-запросе, не различая present/absent поля. legal_entity_id из request → "" → передан как UUID → 22P02.
  Фикс: вынесен в follow-up (нужно использовать `*string` или sql.NullString для optional полей в UpdateContract handler/repo)

BUG-20 (наблюдение): DELETE 204 No Content + handler пытается JSON-encode body — Severity: LOW — Категория: Code Quality
  Шаги: DELETE /admin/v1/legal-entities/:id → 204 (правильно), но в логе backend "http: request method or response status code does not allow body".
  Корневая причина: handler делает respondJSON после w.WriteHeader(204).
  Фикс: вынесен в follow-up (dead-code в handler, не фатально для клиента)

[Test Coverage]
PASS: 8.1 list legal-entities → 200, 8.2 RBAC user→403/unauth→401, 8.3 list contracts → 200 (после фикса BUG-18), 8.4 create legal-entity (с правильными полями name+inn+full_name+address) → 201 + DB insert, 8.5 missing required → 400 "inn и name обязательны", 8.6 create contract (с client_id+legal_entity_id+contract_number+start_date) → 201 + DB consistency + JOIN client_name, 8.7 list contracts after create → 200 с full body, 8.9 delete contract → 204, 8.10 delete legal-entity → 204 + DB count=0.

OBSERVATIONS:
- DELETE — hard delete для contracts/legal-entities (не soft, в отличие от HLR providers).
- API CreateLegalEntity ждёт name+inn+full_name+address (не company_name+kpp+legal_address+actual_address+ceo_name как ожидалось от UI). Schema проще, чем кажется по UI.

[Recommendations]
1. (HIGH) Регрессионный smoke integration-тест на /admin/v1/contracts (list+create+update+delete). Тест бы поймал BUG-18 до production. Аналогично для всех list endpoints с JOIN'ами.
2. (MED) BUG-19: рефакторинг UpdateContract handler/repo на partial-update паттерн (или domain-level approach с GetByID + apply changes + Save).
3. (LOW) BUG-20: убрать respondJSON после WriteHeader(204) в delete handlers.

[Test Data] Создан/удалён: legal-entity QA-Legal (id 96071b23-...) → contract QA-001 (id da232978-...) → DELETE'нуты через API endpoint, остатки cleanup'нуты SQL'ом (0 rows).

[Commits этапа]
- (lock коммит — пропущен, sloppy state)
- 38c0d42 fix(admin): contracts/detalization JOIN typo cl.client_id → cl.id [BUG-18]
- (этот) docs(audit): этап 8/30 — [DONE] частичный



## [DONE] Этап 7/30: Admin — tarification (admin, fix + Infrastructure + QA full, 2026-04-29) — частичный (API-only)

[Summary] 13 TC прогнаны (12 PASS, 1 повторение BUG-9 паттерна). Без code-fix'ов (повторение системного паттерна).

[BUG LIST]

BUG-17 (повторение паттерна BUG-9): CHECK constraint violation на sender_category=INVALID возвращает 500 вместо 400 — Severity: MED — Категория: Logic Gap
  Шаги: POST /admin/v1/tarification/tariff-plans с sender_category="INVALID" → HTTP 500
  Ожидалось: 400 с указанием допустимых значений (shared/paid_registered/free_registered)
  Получилось: 500 (raw SQL не утекает благодаря BUG-7 fix, но HTTP-код неправильный)
  Корневая причина: tarification-service возвращает codes.Internal на CHECK constraint violation вместо codes.InvalidArgument. Тот же паттерн что BUG-9/BUG-13.
  Фикс: вынесен в follow-up — общий системный error-mapping PR (вместе с BUG-9/13)

[Test Coverage]
PASS: 7.1 list tariff-plans → 200 (empty в seed), 7.2 RBAC user→403/unauth→401, 7.3 list tariff-periods → 200, 7.4 list tariff-tiers → 200, 7.5 list hierarchical periods → 200, 7.6 list usage без client_id → 400 c понятным сообщением, 7.10 list usage с client_id → 200, 7.7 list sender-registrations → 200, 7.8 create tariff-plan happy (valid sender_category+strategy) → 201, 7.14 duplicate active plan (partial UNIQUE) → 409 с понятным сообщением, 7.15 update PUT active=false → 200 + DB consistency, 7.11 hierarchical periods RBAC user→403, 7.12b create hierarchical period validation (требует country_id) → 400 (correct).

OBSERVATIONS:
- API контракт CreateTariffPlanRequest требует operator_id+sender_category+strategy (не name+description как ожидалось). Schema CHECK constraints: sender_category IN (shared, paid_registered, free_registered), strategy IN (fixed, threshold, threshold_recalc, prepaid_threshold).
- Hierarchical period также требует country_id вместе с operator_id — сложная иерархия.
- /tarification/usage без client_id возвращает 400 (правильно — нельзя смотреть всё-всё, только per-client).
- TC 7.14 показал: tarification-service ПРАВИЛЬНО классифицирует partial UNIQUE violation как 409. Значит BUG-9 не повсеместен — где-то error mapping есть. Это означает, что system-wide fix BUG-9 нужно делать аккуратно, не сломав уже корректные пути.

[Success Path] Admin создаёт tariff-plan: POST с {operator_id, sender_category=shared|paid_registered|free_registered, strategy=fixed|threshold|threshold_recalc|prepaid_threshold, active=true} → 201 + tariff_plan_id. PUT для отключения, GET через /tariff-plans для list. Hierarchical periods для иерархических тарифов с операторами и странами.

[Recommendations]
1. (MED) Объединить BUG-9 + BUG-13 + BUG-17 в один системный PR error-mapping. Перед фиксом проверить какие сервисы УЖЕ корректно возвращают 409/422 — не сломать их.
2. (LOW) Документировать допустимые enum values (sender_category, strategy) в API spec — иначе клиенты будут ловить 500 на CHECK violation.

[Test Data] Создан/удалён: tariff-plan QA для МТС/shared/fixed → переключён на active=false → удалён SQL'ом.

[Commits этапа]
- 59202cf docs(audit): этап 7/30 — [IN_PROGRESS]
- (этот) docs(audit): этап 7/30 — [DONE] частичный



## [DONE] Этап 6/30: Admin — HLR providers (admin, fix + Infrastructure + QA full, 2026-04-29) — частичный (API-only)

[Summary] 9 TC прогнаны (PASS после фикса), 1 CRITICAL bug найден и исправлен + 1 system-wide follow-up зафиксирован.

[BUG LIST]

BUG-15: HLR provider create/update падал SQLSTATE 22P02 — Severity: CRITICAL — Категория: Reliability Risk
  Шаги: POST /admin/v1/hlr/providers с любым валидным config_json → HTTP 500; routing-service health-monitor каждые ~3s пытался UPDATE существующих HLR-Primary/Secondary и спамил Warn-логи.
  Корневая причина: Create/Update передавали `json.Marshal(...) []byte` напрямую в `database/sql.ExecContext` для jsonb-колонки. Драйвер мапит []byte как bytea, Postgres не имеет implicit cast bytea→jsonb. Маскировался demo_seed.sql (raw SQL), которые создавали записи в обход Go-кода.
  Доказательство: routing-service logs `ERROR: invalid input syntax for type json (SQLSTATE 22P02)` каждые ~3s; admin-gateway: HTTP 500 на POST.
  Фикс: commit 07f2e03 — `string(configJSON)` вместо `[]byte`. После фикса verify: 0 errors в 30s window (vs ~10), 3 health-checks (ожидаемая частота).

BUG-16 (наблюдение, system-wide latent): аналогичный []byte→jsonb паттерн в трёх других репозиториях — Severity: HIGH — Категория: Reliability Risk
  Шаги (теоретический): любой Create в analytics/metric_repository.go, billing/transaction_repository.go (HOT PATH — biling), contact/import_repository.go с непустым metadata → 500
  Маскируется: ранний return на nil/empty metadata защищает большинство caller'ов; но первый caller с непустыми metadata выстрелит в проде. billing особенно опасен — Charge с metadata.
  Фикс: вынесен в follow-up (отдельный системный PR — добавить string-cast во все три и интеграционные round-trip тесты)

[Test Coverage]
PASS: 6.1 list providers → 200 (HLR-Primary/Secondary видны), 6.2 user→403/unauth→401, 6.3 create happy → 201 после фикса, 6.5 update → 200 + DB consistency, 6.6 missing adapter_type → 400 с понятным сообщением, 6.9 delete (soft, active=false) → 200 + DB confirm. Health-monitor шум ушёл.

OBSERVATIONS:
- Health-monitor в routing-service ходит к HLR-провайдерам (https://hlr-test.local) и логирует Warn на network timeout — это норма на dev, hosts недоступны.
- DELETE — soft delete (UPDATE active=false), не hard delete. UI должен это знать (показывать deleted records в скрытом виде).

[Recommendations]
1. (HIGH) Системный PR на BUG-16 — поправить все 3 репо, добавить интеграционные round-trip тесты с testutil.GetTestDSN().
2. (MED) Hard delete vs soft delete для HLR-провайдеров — документировать в спеке.
3. (LOW) Health-monitor verbose: WARN per-cycle при network timeout захламляет логи — снизить до DEBUG, либо log-once-per-N-failures.

[Test Data] Создан/удалён: HLR provider QA-HLR → переименован QA-HLR-R → soft-deleted → hard-cleanup'нут SQL-DELETE.

[Commits этапа]
- 665e9cd docs(audit): этап 6/30 — [IN_PROGRESS]
- 07f2e03 fix(hlr): передавать config как string в jsonb [BUG-15]
- (этот) docs(audit): этап 6/30 — [DONE] частичный



## [DONE] Этап 5/30: Admin — routes + client-routes (admin, fix + Infrastructure + QA full, 2026-04-29) — частичный (API-only)

[Summary] 13 TC прогнаны, 12 PASS, 1 наблюдение. Без code-fix'ов (ловить-в-follow-up).

[BUG LIST]

BUG-13 (наблюдение): non-existent provider_id в POST /admin/v1/routes возвращает 500 вместо 404 — повторение паттерна BUG-9 — Severity: MED — Категория: Logic Gap
  Шаги: создание route с provider_ids=[<random uuid>] → HTTP 500 "Внутренняя ошибка сервера"
  Ожидалось: 400/404 с понятным сообщением "provider not found: <uuid>"
  Получилось: 500 (благодаря BUG-7 fix raw SQL не утекает, но HTTP-код неправильный)
  Корневая причина: routing-service возвращает codes.Internal на FK violation `routes.provider_ids` → providers
  Фикс: вынесен в follow-up (часть системного error-mapping pattern из BUG-9)

[Test Coverage]
PASS: 5.1 list routes → 200 (13 records включая I-Digital + 11 demo + 1 default), 5.3 user→403/unauth→401, 5.4 create route happy → 201, 5.5 update PUT priority → 200 + DB consistency, 5.6 missing pattern → 400 "pattern обязателен", 5.7 invalid uuid format в provider_ids → 400 "invalid provider_id format", 5.9 list client-routes per-client → 200 (empty array), 5.11 user RBAC на client-routes → 403, 5.13 unassign non-existent → 404 "client route not found", cleanup DELETE route → 200.

OBSERVATIONS:
- ClientRoute (миграция/UI-тип) — это **per-operator override** (поля operator_id+provider_id обязательны), а НЕ assign-global-route-to-client. Моё изначальное предположение было неверным — я попытался отправить {route_id, priority} и получил 400. Это **не баг**, корректное поведение. Документация UI-типа в admin.ts:683 явно показывает поля.
- Пропущены TC по advanced routing: MCC/MNC pattern matching (миграции 122-123 свежие), failover между провайдерами при недоступности — это требует подключения реального SMSC. Перенесено в этап 30 (E2E).

[Success Path] Admin создаёт route: POST /admin/v1/routes с {name, pattern (prefix), provider_ids, priority, load_balance_strategy, failover_enabled, active} → 201 + route_id. Update — PUT /admin/v1/routes/{id} с partial fields. Delete — DELETE. Per-client overrides — через POST /admin/v1/clients/{cid}/routes с {operator_id, provider_id, priority, weight, active, shared}.

[Recommendations]
1. (MED) Объединить BUG-9 + BUG-13 в одну системную правку: wrapper `pgErrorToGRPCCode` для всех services, который мапит unique-violation → codes.AlreadyExists, FK violation → codes.NotFound (или FailedPrecondition).
2. (LOW) ClientRoute UI и docs/specs должны явно различать "global route assignment" vs "per-operator override" — без явной типологии новый разработчик повторит мою ошибку.

[Test Data] Создан/удалён: route QA-Route → priority изменён → удалён cleanup'ом.

[Commits этапа]
- 4062d20 docs(audit): этап 5/30 — [IN_PROGRESS]
- (этот) docs(audit): этап 5/30 — [DONE] частичный



## [DONE] Этап 4/30: Admin — providers + connections (admin, fix + Infrastructure + QA full, 2026-04-29) — частичный (API-only)

[Summary] 13 TC прогнаны (PASS), 4 баг найдены и зафиксированы как follow-up. Без code-fix'ов (все 4 — системные паттерны через несколько сервисов, лучше делать одним системным PR).

[BUG LIST]

BUG-8: POST /admin/v1/providers вернул 502 Bad Gateway, но провайдер создан в БД — Severity: LOW — Категория: Reliability Risk
  Шаги: первый POST после рестарта admin-gateway → 502, провайдер всё равно появился в БД
  Корневая причина: видимо первый запрос ловит cold-start gRPC connection до provider-service, haproxy timeout. Повторные запросы PASS.
  Доказательство: HTTP 502 от haproxy + DB SELECT showed inserted row
  Фикс: вынесен в follow-up (race на cold start, не блокер)

BUG-9: provider-service возвращает 500 на UNIQUE constraint вместо 409 — Severity: MED — Категория: Logic Gap
  Шаги: POST /admin/v1/providers с дублем name → 500 "Внутренняя ошибка сервера"
  Ожидалось: 409 Conflict с понятным сообщением (как countries → "country with this ISO code already exists")
  Получилось: 500 (благодаря BUG-7 fix raw SQL не утекает, но HTTP-код неправильный)
  Корневая причина: provider-service gRPC server возвращает `status.Error(codes.Internal, err.Error())` на pgx unique-violation, тогда как должен классифицировать как codes.AlreadyExists. Это системный pattern — нужен wrapper `pgErrorToGRPCCode` для всех services с UNIQUE constraints.
  Фикс: вынесен в follow-up (системный — переписать error mapping во всех services)

BUG-10: provider bind_type не валидируется handler-ом, принимаются произвольные значения — Severity: LOW — Категория: Logic Gap
  Шаги: POST с bind_type=99 → 201 Created (БД не имеет CHECK constraint, SMPP-gateway непредсказуемо себя поведёт при подключении)
  Ожидалось: 400 с "bind_type должен быть 1 (receiver) / 2 (transmitter) / 3 (transceiver)"
  Получилось: 201
  Фикс: вынесен в follow-up (handler-level enum validation)

BUG-11: reconnect non-existent connection → 500 вместо 404 — Severity: LOW — Категория: Logic Gap
  Шаги: POST /admin/v1/connections/<random-uuid>/reconnect → 500 "Ошибка отправки команды"
  Ожидалось: 404 Not Found
  Получилось: 500 (после BUG-7 fix не leak'ает SQL, но код неправильный)
  Фикс: вынесен в follow-up (handler/service should check existence before reconnect)

[Test Coverage]
PASS: 4.1 list providers → 200 (8 провайдеров видны), 4.2 user/unauth → 403/401, 4.3 create happy → 201 (после исправления формата bind_type на int), 4.4 get → 200, 4.5 update PUT → 200 + DB consistency, 4.7 missing name → 400, 4.8/4.9 port validation (negative/65536) → 400 с понятным сообщением, 4.11 health → 200 (status=unhealthy, ожидаемо т.к. SMSC недоступен), 4.12 list connections → 200 (пустой список — ни один SMPP-bind не активен), 4.13 connection RBAC → 403/401, 4.15 delete → 200 + DB count=0.

OBSERVATIONS:
- bind_type API использует int (1=receiver, 2=transmitter, 3=transceiver), не строку. Это inconsistent с UI-описанием бизнес-логики (UI отображает текст). Архитектурный выбор, не баг.
- Healthcheck возвращает unhealthy — это норма на dev-стенде без реальных SMSC. Поведение корректное.
- BUG-7 fix верифицирован дважды: TC 4.6 (duplicate) и TC 4.14 (reconnect) — оба возвращают generic "Внутренняя ошибка сервера" вместо raw SQL, как и было задумано.

[Success Path] Admin создаёт провайдер: POST /admin/v1/providers с {name, host, port, system_id, password, bind_type=3, max_connections, active=true} → 201 + provider_id. Затем PUT /admin/v1/providers/{id} меняет имя/статус → 200. GET /providers/{id}/health показывает текущий статус подключения. DELETE удаляет.

[Recommendations]
1. (HIGH) Системный фикс error-mapping в gRPC-сервисах: pgx unique-violation → codes.AlreadyExists, not_found → codes.NotFound. Сейчас в нескольких сервисах raw err идёт в codes.Internal. Без BUG-7 фикса это была бы information disclosure; с фиксом это просто неправильный HTTP-код, но HTTP-код важен для UI и интеграций.
2. (MED) Добавить enum-validation для bind_type в provider handler (1/2/3). Аналогично — для других int-enum полей в БД, которые могут принимать недопустимые значения.
3. (LOW) Cold-start 502 на admin-gateway после rebuild — добавить retry на admin-gateway или подождать в healthcheck до полной готовности gRPC pool.
4. (LOW) connections handler — проверка существования перед reconnect/stop, корректный 404 для несуществующих.

[Test Data] Создано/удалено: provider QA-Test-Provider → переименован → удалён через DELETE; провайдеры X1, X2 (port-validation negative tests, не созданы), X3 (bind_type=99 baseline test, удалён вместе с QA-Renamed cleanup'ом).

[Commits этапа]
- 2b405ed docs(audit): этап 4/30 — [IN_PROGRESS]
- (этот) docs(audit): этап 4/30 — [DONE] частичный (без code-fix; 4 follow-up)



## [DONE] Этап 3/30: Admin — countries + operators (admin, fix + Infrastructure + QA full, 2026-04-29) — частичный (API-only, без UI)

[Summary] 19 TC прогнаны (PASS), 2 баг найдены и исправлены. UI-проверки пропущены (Playwright MCP disconnected) — режим API+SQL. На стенде свежеприменены миграции 122-123 (MCC/MNC + seed СНГ); MCC/MNC у legacy-операторов из 000012 — NULL (заметка для этапа 5).

[BUG LIST]

BUG-6: country handler не валидирует длину/формат полей — SQL-leak в 500 — Severity: MED — Категория: Logic Gap / Information Disclosure
  Шаги: POST /admin/v1/countries с iso_code="TOOLONG" → response 500 + body содержит SQLSTATE 22001 "value too long for type character varying(2)" (раскрытие schema)
  Ожидалось: 400 с понятным сообщением, без leak'а схемы
  Получилось: 500 с raw SQL-ошибкой
  Доказательство: HTTP 500 body `"ERROR: value too long for type character varying(2) (SQLSTATE 22001)"`
  Фикс: commit 068150e — добавил handler-level validation: ISO 3166-1 alpha-2 regex для iso_code, ISO 4217 для currency, `^\+?[0-9]{1,5}$` для phone_code, name max 100 рун (UTF-8). UpdateCountry теперь тоже Validate(), пустой body отвергается 400. Прошёл 2 review-цикла (UTF-8 runes, regex format, empty-body).

BUG-7 (cross-cutting): default-ветка `response.GRPCError` форвардила `st.Message()` для codes.Internal → SQL-leak во ВСЕХ admin/portal/client handlers — Severity: HIGH — Категория: Information Disclosure / Reliability Risk
  Шаги: любая непредусмотренная DB-ошибка (FK violation, deadlock, future-migration constraint) проходила через `respondGRPCError` → `ErrInternalServer(st.Message())` → клиент видел raw SQL-сообщение
  Ожидалось: generic "Внутренняя ошибка сервера" клиенту, детали в server-side log
  Получилось: 17+ admin handlers и 60+ portal/client handlers — все потенциальные SQL-утечки
  Доказательство: BUG-6 — конкретный пример leak'а; grep по `status.Error(codes.Internal, err.Error())` в `internal/services/*/grpc/server.go` показал 47+ call-site'ов с тем же паттерном
  Фикс: commit 068150e — `response.go` default-ветка возвращает static "Внутренняя ошибка сервера", server-side `log.Error().Err(err)` уже сохраняет detail. Закрывает leak system-wide одной правкой.

[Test Coverage]
PASS: 3.1 list countries → 200 (СНГ из seed 123), 3.2 list operators → 200, 3.3 user → 403 "Admin access required", 3.4 unauth → 401, 3.5 create happy → 201, 3.6 duplicate iso_code → 409, 3.7 empty name → 400, 3.8 iso_code TOOLONG → 400 (после фикса), 3.8b iso_code lowercase → 400, 3.8c phone_code 6цифр → 400, 3.8d cyrillic name 88 символов → 201, 3.9 SQLi в name → 400 (parsing fails), 3.10 UpdateCountry empty body → 400 (после фикса), 3.11 list operators with country_id filter → 200, 3.12 create operator → 201, 3.13 create prefix → 201, 3.14 list prefixes → 200, 3.15 create operator с несуществующим country_id → 404 "country not found", 3.16 update operator → 200, 3.17 DB consistency после update → ✓, 3.18 unauth+user POST operators → 401/403, 3.19 empty prefix → 400.

[Success Path] Admin list countries → /admin/v1/countries → 200 со списком СНГ. Admin создаёт страну: POST {name, iso_code (2 ASCII upper), phone_code (digits), currency (3 ASCII upper)} → handler validate → routing-service gRPC → INSERT clients → 201 с full body. Operators: POST /admin/v1/operators c {country_id, name, code} → 201; затем POST /operators/:id/prefixes c {prefix} → 201; список через /operators/:id/prefixes → 200.

[Recommendations]
1. (HIGH) DELETE country/operator endpoints не реализованы (404). Добавить — иначе test-data накапливается без cleanup.
2. (MED) MCC/MNC у legacy-операторов из миграции 000012 — NULL; сидится только новые из 000123. Если route lookup использует MCC/MNC primary, легаси не находится. Проверить в этапе 5 (routes/MCC/MNC).
3. (LOW) Прод-проверка legacy-данных: SELECT количества стран с lowercase currency / нестандартным phone_code, чтобы убедиться что новый regex не сломает UPDATE их через legacy-форму.

[Test Data] Создано/удалено: country YZ (Cyrillic-test), country QA (length-test), operator QA-Operator-Renamed с prefix 7888 — все cleanup'нуты SQL-DELETE'ом в конце прогона.

[Commits этапа]
- aec3ca4 docs(audit): этап 3/30 — [IN_PROGRESS]
- 068150e fix(admin): country length+format validation + GRPC error leak [BUG-6 + BUG-7]
- (этот) docs(audit): этап 3/30 — [DONE] частичный



## [DONE] Этап 2/30: Auth — register / password-reset / 2FA setup (user, fix + Infrastructure + QA full, 2026-04-29) — частичный

[Summary] 14 TC прогнано, 14 PASS после фиксов. 2 CRITICAL bug найдены и исправлены, оба требовали БД-миграций (вариант 3 из эскалации — гибрид: миграция сейчас, рефакторинг кода как follow-up). 6 TC по полному 2FA flow (verify TOTP, ticket reuse, disable, login через TOTP) перенесены в этап 31 (cross-cutting) — требуют генерации TOTP-кодов из секрета, что усложняет автоматизацию.

[BUG LIST]

BUG-4: register падает 500 на отсутствующей колонке clients.account_type — Severity: CRITICAL — Категория: Reliability Risk / Spec Drift
  Шаги: 1) POST /portal/v1/auth/register с валидным телом 2) client-service делает INSERT INTO clients (..., account_type, ...) 3) ERROR: column "account_type" does not exist (SQLSTATE 42703) 4) UI: "Внутренняя ошибка сервера"
  Корневая причина: код client-service ссылается на account_type (write в client_repository.go:43, read в tarification client_info_repository.go:30); миграция, добавляющая колонку, никогда не была написана. Колонка введена в коммите be20324 (2026-04-15) без миграции. Self-service register никогда не работал на этом стенде (4 demo-клиента созданы через seed в обход).
  Доказательство: client-service logs `column "account_type" of relation "clients" does not exist`; проверка `\d clients` — колонки нет; grep по миграциям 000001-000123 — ни одного ALTER TABLE с этим именем
  Фикс: миграция 000124_add_account_type_to_clients (commit b1c1f29) — narrow fix; рефакторинг (удалить колонку, заменить derived parent_client_id IS [NOT] NULL) вынесен в follow-up

BUG-5: триггер create_account_for_new_client падает на NOT NULL company_id — Severity: CRITICAL — Категория: Reliability Risk / Spec Drift
  Шаги: 1) После фикса BUG-4 — повторить register 2) client-service создаёт клиента, AFTER INSERT триггер срабатывает 3) INSERT INTO accounts без company_id → SQLSTATE 23502
  Корневая причина: миграция 000091 сделала accounts.company_id NOT NULL и backfill'нула существующие clients/accounts «Оферта»-компанией, но триггер create_account_for_new_client (введён 000056) обновлён НЕ был. Self-service register сломан с момента применения 091. demo_seed обходил это `ALTER TABLE clients DISABLE TRIGGER`, что маскировало баг от регрессионных тестов.
  Доказательство: client-service logs `null value in column "company_id" of relation "accounts" violates not-null constraint`; `\df create_account_for_new_client` показывает функцию без company_id в INSERT
  Фикс: миграция 000125_fix_create_account_trigger_with_company (commit b1c1f29) — триггер сам создаёт «Оферта»-компанию и client_companies, INSERT account с company_id. Прошёл 2 review-цикла (исправлен silent-orphan path в conflict-handling, добавлен warning о race с idx_client_companies_default).

[Test Coverage]
PASS: 2.1 register happy free → 201 + 3-tier consistency (users→clients→client_companies→companies→accounts), 2.2 register с plan=starter → 201 + plan_id привязан, 2.4 duplicate email → 409 CONFLICT "email already registered", 2.5 password<8 → 400, 2.6 missing password → 400, 2.7 missing company_name → 400, 2.8 company_name>500 → 400, 2.9 invalid email format → 400, 2.10 SQL injection в email → 400 (валидация формата перехватывает), 2.11 password-reset request happy → 202 anti-enum, 2.12 password-reset request несуществующий email → 202 (anti-enumeration работает), 2.13 password-reset с invalid token → 400, 2.13b empty token → 400, 2.16 2FA setup → 200 с QR-URL и 10 recovery codes.

PERENESEN-в-этап-31: 2.17-2.19 2FA verify TOTP / login c 2FA / ticket reuse — требуют генерации TOTP-кодов из секрета, лучше делать при cross-cutting аудите security.

[Success Path] Регистрация: POST /portal/v1/auth/register с {email, password, company_name, [contact_person, phone, plan_name]} → backend валидирует format/длину → создаёт client (с account_type='direct') → trigger создаёт «Оферта»-компанию + client_companies + accounts (company_id, balance=0, RUB) → создаёт user (role=client) → создаёт session → cookies portal_session+csrf_token → 201 с {client_id, user}. Reset password: POST /password/reset-request → всегда 202 (anti-enum); POST /password/reset с {token, new_password} → 200 если token валиден, 400 если просрочен/неверен.

[Recommendations]
1. (HIGH) Follow-up: рефакторинг убрать дублирующую колонку account_type, перенести логику триггера в client-service.CreateClient (явный flow вместо скрытого триггера). Эта пара багов — следствие того что бизнес-логика разбросана между триггером и application code.
2. (HIGH) Добавить регрессионный E2E-тест playwright на register flow — голый smoke (POST → 201 + cookies + login subsequent). Покрытие отсутствует, и оба бага влияли на production.
3. (MED) Рассмотреть e-mail-нотификацию после register (welcome + verification) — сейчас регистрация без email-verify, password-reset без реальной отправки email.
4. (LOW) Глубокий audit миграций 091 → 124: пройти все триггеры/функции, использующие accounts/clients, проверить совместимость со схемой.

[Test Data] Создано 3 тестовых аккаунта через UI register: qa+1777472328@audit.local, qa+s1777472352@audit.local (plan=starter), qa+v1777472560@audit.local (re-verify после доработки миграции). Все с client_companies (Оферта) и accounts (0 RUB).

[Commits этапа]
- 1eb0308 docs(audit): этап 2/30 — [IN_PROGRESS]
- b1c1f29 fix(migrations): 000124+000125 — починить self-service register flow [BUG-4+BUG-5]
- (этот) docs(audit): этап 2/30 — [DONE] частичный



## [DONE] Этап 1/30: Auth — login/logout/session (admin/aggregator/user, fix + Infrastructure + QA full, 2026-04-29) — частичный

[Summary] 22 тест-кейса прогнано (из 31 запланированных), 22 PASS, 0 FAIL после фиксов. 2 бага найдены и исправлены, 1 баг-doc — follow-up. 9 TC перенесены: TC 1.29-1.30 (2FA flow + ticket reuse) → этап 2 (зависит от register-flow с 2FA setup); TC 1.7-1.8/1.11/1.15/1.17/1.19-1.21 — UI-only мелочи (HTML5 required, only spaces, duplicate submit, CSRF tampering, concurrent login, role routing UI), низкоценные после того как backend RBAC и cookie flags подтверждены.

[BUG LIST]

BUG-1: panic в `audit.Publisher.Publish` при `producer == nil` — Severity: CRITICAL — Категория: Reliability Risk
  Шаги: 1) Перезапуск portal-gateway, когда Kafka недоступна 2) кnopка "Войти" 3) panic → 500
  Ожидалось: graceful degradation (no audit, but login works)
  Получилось: `nil pointer dereference` в `publisher.go:46`, login полностью сломан
  Доказательство: portal-gateway logs `panic recovered ... invalid memory address ... audit.(*Publisher).Publish ...`; UI: "Внутренняя ошибка сервера"; API: HTTP 500
  Фикс: commit fe3dfb9 — defensive nil-check в Publish и Close, тесты на nil-producer/nil-receiver

BUG-2: race condition в `LoginPage` useEffect — admin попадал в client portal — Severity: HIGH — Категория: Logic Gap
  Шаги: 1) admin вводит креды, нажимает "Войти" 2) handleLogin делает navigate('/admin'), но useEffect перезаписывает его navigate('/dashboard') → /command-center 3) admin видит client UI вместо admin-панели
  Ожидалось: admin/superadmin → /admin; client → /dashboard
  Получилось: всегда /dashboard → /command-center из-за race
  Доказательство: UI: после login URL `/command-center` под админом, отображается client sidebar; API: `/portal/v1/profile` возвращает `role: "admin"`
  Фикс: commit 1e8c960 — хойстил `destForRole(role)` в `utils/authRedirect.ts` (single source of truth, типизирован UserRole), useEffect и `handleLogin`/`handle2fa` используют helper. Тот же паттерн в RegisterPage пофикшен заодно. После APPROVE верифицировано в браузере: admin → /admin/dashboard.

BUG-Doc-1: расхождение psql credentials в доках — Severity: LOW — Категория: UX Friction
  Шаги: попытка применить seed по инструкции `psql -U sms sms`
  Ожидалось: команда срабатывает
  Получилось: FATAL role "sms" does not exist; реально `-U smpp -d smpp_db`
  Доказательство: CLAUDE.md строка про seed; `docker exec postgres env` → POSTGRES_USER=smpp, POSTGRES_DB=smpp_db
  Фикс: вынесен в follow-up (правка CLAUDE.md одним коммитом в конце аудита)

BUG-3 (наблюдение): notification-handler в portal-gateway ошибается `client_id is required` для admin'а — Severity: LOW — Категория: Logic Gap
  Шаги: admin login → фоновый запрос на нотификации
  Получилось: `rpc error: code = InvalidArgument desc = client_id is required` дважды (для completed и failed campaigns) при каждом dashboard load
  Доказательство: portal-gateway logs
  Фикс: вынесен в follow-up (этап 17 User dashboard или этап 15 Admin monitoring), не блокирует login

[Test Coverage]
PASS: TC 1.1 admin login (UI+API+Redis), 1.2 user login (API), 1.3 aggregator login (API), 1.4 admin logout (UI+API+Redis HSET delete), 1.5 user logout (API), 1.6 empty body→400, 1.9 wrong email→401, 1.10 wrong password→401, 1.12 SQL injection→401 (escaped), 1.13 XSS in email→401, 1.14 10000-char password→401 (no 500), 1.16 tamper session cookie→401, 1.22 unauth /admin/v1/clients→401, 1.23 user→403 "Admin access required", 1.24 admin→200, 1.25 user→403 "not a reseller", 1.25b aggregator→200 sub-accounts, 1.26 Redis HGETALL session structure (user_id/role/client_id/ip/ua/TTL 24h), 1.27 cookie flags (portal_session HttpOnly+SameSite=Lax+24h, csrf_token SameSite=Lax+24h без HttpOnly — by design, double-submit pattern), 1.28 public paths /health/health/live/health/ready→200, 1.31 inactive user→403 "user is inactive".

[Success Path] Admin@example.com/Admin123! → /login → форма ввода → клик "Войти" → POST /portal/v1/auth/login (200, body{user.role=admin}) → cookies portal_session(HttpOnly,SameSite=Lax,24h)+csrf_token → Redis HSET session:<id> с user_id, role=admin, ip, ua, TTL 86400s → AuthContext setUser, useEffect видит role=admin → navigate('/admin') → AdminLayout рендерится. Logout: клик "Выйти" → POST /portal/v1/auth/logout (204) → Redis EXISTS session=0 → редирект на /login.

[Recommendations]
1. (HIGH) Pипелировать BUG-2 backwards: добавить регрессионный E2E-тест playwright на admin login → URL должен содержать /admin (не /dashboard). Текущая регрессия LoginPage.test.tsx — unit-тест, не покрывает race с useEffect.
2. (MED) Зафиксить BUG-3 (notification-handler пропускает client_id для admin'а) — отдельный follow-up.
3. (LOW) Pre-этап вскрыл: portal-gateway не переподключается к Kafka после старта (BUG-2-arch follow-up). Архитектурный фикс — отдельная спека.

[Test Data] Использован существующий demo_seed: admin@example.com / Admin123!, client@demo.local / Admin123!, reseller@demo.local / Admin123!. Никаких новых сущностей не создано.

[Commits этапа]
- 2b531b3 docs(audit): этап 1/30 — [IN_PROGRESS]
- fe3dfb9 fix(audit): defensive nil-check в Publisher.Publish/Close [BUG-1]
- 1e8c960 fix(auth): role-aware redirect после login/register [BUG-2]
- (этот) docs(audit): этап 1/30 — [DONE] частичный



## [DONE] Модуль: Имена отправителей и шаблоны — full sweep (subaccount + aggregator, fix + Infra + QA full, 2026-04-23)

Аккаунты: `subacc@test.local` (`a0000000-...-000000000002`, parent = aggregator) и `aggregator@test.local` (`a0000000-...-000000000001`, is_reseller=t). Полный обход API (`/portal/v1/sender-names`, `/templates`, `/sender-registrations`, `/reseller/*`, `/settings/default-senders`) с прямой верификацией состояния через PostgreSQL.

### Pass/Fail summary

| TC | Зона | Вердикт |
|----|------|---------|
| TC-SN-CR | `POST /sender-names` BVA: пустое, пробелы-only, 11/12 alphanumeric, 15/16 numeric, кириллица, `<script>`, SQL-инъекция, дубль | PASS — формат валидируется, dup → 409 |
| TC-SN-FB | Тот же endpoint без `company_id` (новый поток UI) | **FAIL → FIXED** (BUG-1) |
| TC-SN-RES | Resubmit rejected → pending, повторный approve/reject → 400 | PASS |
| TC-SN-HIST | `GET /sender-names/:id/history` | PASS — записи `system → admin` пишутся |
| TC-SN-CT | Cross-tenant: subaccount читает чужой sender_name | PASS — 404 |
| TC-SN-OP | `bulk operator-registrations` happy + idempotency + блок на pending имени | PASS |
| TC-SN-OPT | nested `operator-templates` create draft | PASS |
| TC-TPL-BVA | `POST /templates` BVA: name/body required, body=1600 ✓, body=1601 → 400 | PASS |
| TC-TPL-CT | template c чужим `sender_name_id` | PASS — 404 |
| TC-TPL-LC | draft → submit → revision_requested → submit → approve | PASS, повторный submit/approve из неверного статуса → 400 |
| TC-AGG-MOD | reject без reason / с reason / approve / повторный | PASS — reason обязателен (400), state-machine блокирует |
| TC-AGG-MOD2 | aggregator approve через `/reseller/sender-names/:not-subaccount-id/approve` | PASS — 404 (но сообщение-дубль исправлено в BUG-3) |
| TC-AGG-CNT | `/reseller/moderation/counts` | PASS — учитывает `pending+revision_requested` для шаблонов и `pending` для имён |
| TC-CROSS-RES | subaccount → `/reseller/*` | PASS — 401 «доступ только для агрегаторов» |
| TC-DEF-CT | `/settings/default-senders`: подмена на чужой sender_name_id | **FAIL → FIXED** (BUG-2) |
| TC-DEF-PEND | `/settings/default-senders` с pending именем | **FAIL → FIXED** (BUG-2) |
| TC-DEF-INV | `/settings/default-senders` с несуществующим UUID | **FAIL → FIXED** (BUG-2) |
| TC-DEF-CH | `/settings/default-senders` с unknown channel `telegram` | **FAIL (500 INTERNAL) → FIXED** (BUG-2: 400) |
| TC-DEF-MAX | `/settings/default-senders` с каналом `max` | **FAIL (500) → FIXED** (BUG-4: миграция + UI) |
| TC-ERR-DUP | сообщения вида `«sender name not found не найден»` | **FAIL → FIXED** (BUG-3) |

### Зафиксированные баги (все исправлены, code-reviewed, задеплоены, верифицированы)

| # | Severity | Файл | Было → Стало |
|---|---|---|---|
| BUG-1 | HIGH (UX) | [sender_names.go:68-103](internal/gateway/portal/handlers/sender_names.go#L68) | `CreateSenderName` без `company_id` падал с «у клиента не найдена компания по умолчанию», даже если у клиента **одна** компания без флага is_default → новый fallback: 0 компаний → «добавьте компанию», 1 → автоматически берём её, 2+ → «выберите компанию или назначьте основную». Verify: `POST /sender-names {name:QaAuto01}` без company_id → 201 |
| BUG-2 | **CRITICAL (security/UX)** | [settings.go:147-227](internal/gateway/portal/handlers/settings.go#L147) | `SetDefaultSenders` принимал любой sender_name_id (включая чужие/несуществующие/pending), unknown channel валился в 500. Добавлена валидация: whitelist `{sms,viber,max}`, ownership-check `client_id=caller`, status-check `approved`, пустой sender_name_id очищает запись. Verify: 5 негативных сценариев → 400 с человеческим сообщением, MAX → 200 |
| BUG-3 | MED (UX) | [errors.go:107-119](internal/shared/errors.go#L107) | `shared.ErrNotFound` всегда добавлял суффикс «не найден», давая «sender name not found не найден» при callsites вида `ErrNotFound("sender name not found")`. Теперь wrapper умнее: если строка уже содержит «не найден/найдена/найдено/not found» — суффикс не добавляется. Обратносовместимо с 30+ callsites вида `ErrNotFound("шаблон")`. Verify: `GET /sender-names/<other>` → `"sender name not found"` (одинарно) |
| BUG-4 | MED (feature gap) | [migrations/000120](migrations/000120_default_senders_add_max_channel.up.sql), [DefaultSendersPage.tsx:6-10](portal-frontend/src/pages/settings/DefaultSendersPage.tsx#L6) | Канал MAX отсутствовал в DB CHECK constraint и в селекторе UI. Добавлены: миграция расширяет `default_sender_names_channel_check` до `[sms,viber,max]`, UI получил пункт «MAX». Verify: запись `(client_id, channel=max, sender_name_id)` в БД, UI селектор показывает 3 канала |

### Проверка инфраструктуры (раздел 7)

- **Таблицы:** `sender_names`, `sender_name_status_history`, `default_sender_names`, `templates`, `operator_templates`, `sender_registrations`, `sender_name_billing_records`, `client_companies` — все на месте, индексы корректные.
- **Миграции:** 000120 применена в проде через `scripts/server.sh migrate`. Down-миграция корректно удаляет `max`-строки перед сужением CHECK.
- **gRPC:** `sendernamev1.SenderNameServiceClient` и `templatev1.TemplateServiceClient` отрабатывают transitions; reseller endpoints выполняют дополнительный `is_reseller=true`-чек прямым SQL до gRPC.
- **CSRF:** `/portal/v1/auth/login` исключён из CSRF, остальные mutating-endpoints требуют `X-CSRF-Token`. Проверено.
- **Rate-limit/Redis:** не затронуты этими фиксами.
- **FK:** `default_sender_names.sender_name_id` **не имеет FK** на `sender_names.id` — фиксится валидацией в handler. Добавлять FK задним числом рискованно (orphan rows на проде из старой логики). Зафиксировано в backlog.

### Остаточный бэклог

| Severity | Где | Что |
|---|---|---|
| MED | `default_sender_names` | Нет FK на `sender_names`. Handler валидирует, но при rare-race можно создать orphan. Накатить FK + cleanup-миграция (требует аудита существующих строк). |
| MED | `SetDefaultSenders` | Цикл по каналам не атомарен: ошибка на 3-м канале оставляет первые 2 применёнными. Обернуть в `pool.BeginTx`. |
| LOW | `sender_name_status_history.actor_type='admin'` для агрегатора | Путаница в аудите: aggregator ≠ admin. Завести отдельный actor_type `aggregator/reseller`. |
| LOW | `templates` отказ approve из `revision_requested` | API сейчас требует subaccount явно делать `submit` после revision; UI это закрывает, но при прямом curl агрегатор может удивиться сообщению `«can only approve pending or review templates»`. Согласовано как корректное поведение, но сообщение можно улучшить. |
| LOW | `sender_names.name` с пробелами | Регэкс принимает `^[A-Za-z0-9 ]{1,11}$`, но `«AB CD»` ≠ `«ABCD»`, оба валидны. Стоит trim/normalize на входе либо warn в UI. |
| LOW | `/settings/default-senders` UI | «Не выбрано» (пустое value) не отправляется на бэкенд → существующий default нельзя очистить через UI (бэкенд это уже умеет). |

### Summary
- **20 тест-кейсов**, **15 PASS**, **5 FAIL → 4 FIXED** (один FAIL — `TC-DEF-MAX` — закрыт двумя независимыми фиксами BUG-2 и BUG-4).
- 1× CRITICAL (security в default-senders), 1× HIGH (UX в CreateSenderName), 2× MED — все исправлены, проверены в проде.
- Deploy: коммит `d67bc80`, миграция 000120, верификация после `docker compose up -d --no-deps --build portal-gateway`.



## [DONE] Модуль: Панель субаккаунта — раунд 2, полный обход (subaccount, fix + инфраструктура + QA full, 2026-04-23)

Тест-аккаунт `subacc@test.local` / `Admin123!` (client `a0000000-...-000000000002`, parent = aggregator). Обошёл все 25 страниц субаккаунта, включая попытки прямого доступа к `/network/*`, `/providers`, `/routing`.

### Pass/Fail summary

| TC | Страница | Вердикт |
|----|----------|---------|
| TC-1 | `/command-center` | PASS — баланс `49 976,10 ₽`, плашка «Сообщения отправляются через инфраструктуру агрегатора», нет кнопки «Управление сетью» |
| TC-2 | `/quick-send` | PASS — отправка `+79990000001` → rejected (БД: `messages` row `c81aa735-8dbe...`, status=rejected) |
| TC-3 | `/campaigns` + `/campaigns/:id` | PASS — 3 кампании, inline-плашка «в рамках вашего аккаунта» |
| TC-4 | `/campaign-schedules` | PASS — empty state c CTA |
| TC-5 | `/templates` | PASS — empty state c CTA |
| TC-6 | `/sender-names` + `/sender-names/:id` | PASS — `Trest` approved, billing next `01.05.2026` |
| TC-7 | `/companies` + `/companies/:id` | PASS — «Test» без ИНН, кнопка «Сделать основной» |
| TC-8 | `/messages` + `/messages/:id` | PASS — 37 сообщений, XSS payload `<script>alert(1)</script>` рендерится как текст |
| TC-9 | `/cascade/history` | PASS — empty state + фильтры стратегий |
| TC-10 | `/analytics` | PASS — timeline + таблица, доставлено 5 из 38 |
| TC-11 | `/contact-lists` + detail + import | PASS — `TestCampaignList` (3 контакта) |
| TC-12 | `/segments` | PASS — empty |
| TC-13 | `/opt-out` | PASS — empty |
| TC-14 | `/billing` | **FAIL → FIXED** (BUG-1, BUG-2, см. ниже) |
| TC-15 | `/tariffs` | PASS (для каждого типа имени/трафика — «Тариф для вашего аккаунта не настроен», ожидаемо, ведь агрегатор управляет) |
| TC-16 | `/api-keys` | PASS — 6 ключей, есть возможность задать scope `sub-accounts:manage` (backend должен 403-ить при использовании, проверка не проведена) |
| TC-17 | `/webhooks` | PASS — empty |
| TC-18 | `/lookup` | PASS — empty, счётчик 0 |
| TC-19 | `/settings/smpp` | PASS — статичный текст про SMPP-провайдеров |
| TC-20 | `/profile` | PASS — email/company, 2FA настройка, смена пароля |
| TC-21 | `/settings/domains` | PASS — empty |
| TC-22 | `/settings/notifications` | PASS — 6 типов уведомлений × (in-app/email) |
| TC-23 | `/settings/default-senders` | PASS — SMS+Viber селекторы (канал MAX отсутствует; у аккаунта действительно нет MAX-имён, но несогласованность с `/sender-names/:id/operators` стоит зафиксировать отдельно) |
| TC-24 | `/audit-log` | PASS-warning — «Данные не найдены» по всем фильтрам. В селекте `Действие` присутствуют опции `Суб-аккаунт создан / удалён / Лимит изменён` — невозможные для роли subaccount действия; стоит отфильтровать в UI по роли (не критично, косметика) |
| TC-25 | `/network/dashboard` прямым URL | PASS — `RequireReseller` редиректит на `/command-center` |
| TC-26 | `/providers` прямым URL | PASS — показывает placeholder «настраивает агрегатор», nav скрывает |
| TC-26 | `/routing` прямым URL | **FAIL → FIXED** (BUG-3) — ранее показывал полный edit-UI с маршрутами/кнопками |

### Зафиксированные баги (все в режиме fix сразу исправлены)

| # | Severity | Файл | Было | Стало |
|---|---|---|---|---|
| BUG-1 | HIGH | [BillingPage.tsx:80-85](portal-frontend/src/pages/billing/BillingPage.tsx#L80) | Транзакции с типом `transfer_in` / `transfer_out` (backend: [transfer.go:10-11](internal/services/billing/domain/transfer.go#L10)) рендерились сырым enum-кодом — в колонке «Тип» badge показывал `transfer_in` вместо локализованного текста | Добавлены метки `Перевод (вх.)`, `Перевод (исх.)` и цвета badge (`success` / `danger`) в `typeLabel` и `typeBadgeVariant`, фильтр `TYPE_OPTIONS` дополнен обоими значениями (вместо старого обобщённого `transfer`) |
| BUG-2 | HIGH | [BillingPage.tsx:107-113](portal-frontend/src/pages/billing/BillingPage.tsx#L107) | Суммы колонки рендерились как `{type === 'charge' ? '-' : '+'}{amount}`. При `transfer_in` с отрицательным amount получалось `+-10,00 RUB`. Знак был прибит к **типу**, а не к знаку числа | Логика пересчитана: выбираем `isOutflow` по (`charge`, `transfer_out`, или `amount<0`); знак берётся из знака числа, отображаем `Math.abs(amount)` с корректным префиксом, цвет суммы идёт от `isOutflow` |
| BUG-3 | HIGH (access/UX) | [RoutingPage.tsx](portal-frontend/src/pages/routing/RoutingPage.tsx) | Субаккаунту прямым URL `/routing` открывался полный редактор маршрутов с кнопками «+ Добавить маршрут», «Изменить», «Удалить» (в БД у субакка сидированы 5 client_routes, backend не блокирует). Несогласованность с `/providers`, где уже есть placeholder | Добавлен ранний return с placeholder «в режиме суб-аккаунта маршрутизацию настраивает агрегатор…» по паттерну `/providers`. Backend пока оставлен без доп. проверок — write-paths через UI теперь не запускаются, а прямые REST-запросы — отдельный backlog-пункт |

### Проверка инфраструктуры (раздел 7)

- **API:** Handler `GET /portal/v1/routing/routes` ([client_routing.go:135-138](internal/gateway/portal/handlers/client_routing.go#L135)) передаёт `ClientId: clientID.String()` — не пропускает parent_client_id. Маршруты субакка это его собственные записи `client_routes`, а не агрегаторские; утечки чужих данных нет.
- **БД:** В `client_routes` у `a0000000-...-000000000002` лежат 5 sid-строк (МТС/Билайн/МегаФон/Default). Это данные сидов, unused в продакшене subaccount-flow.
- **Транзакции:** Backend корректно проставляет `transaction_type = 'transfer_in'` / `'transfer_out'` при переводе — проблема была чисто фронтовая.
- **Kafka/Redis:** не затронуты; регрессий по каналу отправки SMS нет (QuickSend → rejected → транзакции не создаются, баланс не изменился).

### Остаточный бэклог (для следующих раундов, не фикшено)

| Severity | Где | Что |
|---|---|---|
| MED | [client_routing.go:ListRoutes/CreateRoute/UpdateRoute/DeleteRoute](internal/gateway/portal/handlers/client_routing.go) | Backend не отвергает вызовы от клиентов с `parent_client_id`. UI теперь блокирует, но прямой API-запрос через curl/api-key с scope `full_access` вернёт/создаст маршруты. Нужна middleware-проверка «не-reseller subaccount → 403 для write, либо возврат пустого set» |
| LOW | [AuditLogPage.tsx — фильтр Действие](portal-frontend/src/pages/audit/AuditLogPage.tsx) | Опции `Суб-аккаунт создан / удалён / Лимит изменён` показываются роли, которая этих действий не порождает. Косметика |
| LOW | `/audit-log` — пустые логи | Субаккаунт выполнил логин/смена настроек/отправки, но UI показывает «Данные не найдены». Либо backend не пишет client-level audit для subacc, либо handler фильтрует только по actor_id user — нужно расследование |
| LOW | `/settings/default-senders` | Канал MAX отсутствует в UI; у аккаунта нет MAX-имён. Проверить, что селектор появляется автоматически когда имя добавлено, иначе субаккаунт не сможет выбрать default MAX |

### Summary
- **28 тест-кейсов** (25 pages + 3 guard-checks), **25 PASS**, **3 FAIL → FIXED**.
- Критичных багов не найдено. 2× HIGH + 1× HIGH(access/UX) — все исправлены одним патчем.
- Deploy: см. следующий коммит.



## [DONE] Модуль: Управление сетью — раунд 4: /network/tariffs (aggregator, fix, Infrastructure Check, QA full, 2026-04-23)

Претензия пользователя: «нет возможности создавать любые виды тарифов из панели агрегатора».

### Диагноз

- `POST /portal/v1/network/tariff-templates` работает, шаблон создаётся — но после редиректа в `/network/tariffs/editor/:id?mode=template` редактор зависает на empty-state «тарифный план не найден. Создайте план на уровне платформы».
- **«Уровня платформы» не существует**: у reseller/aggregator нет UI для создания `reseller_tariff_plans`. Есть только хендлер `POST /reseller/tariff-plans` — но он ожидает `country_id` (UUID) и не подвязан к `/network/*` маршрутам.
- То же самое при открытии override-редактора для субаккаунта без назначенного шаблона.
- Итог: агрегатор может создать пустой шаблон, но не может задать ни одной цены.

### Исправлено (BUG-14, HIGH UX/Logic Gap)

| Слой | Было | Стало |
|---|---|---|
| [internal/gateway/portal/handlers/network_tariff_bulk.go:455](internal/gateway/portal/handlers/network_tariff_bulk.go#L455) | Нет хендлера | `CreatePlan`: транзакция plan + default period (today, open-ended) + default tier (from_count=0, price=0). ISO-код резолвится в country_id, проверяется ownership шаблона/субаккаунта, invalidates `tariffs:summary:<reseller_id>` |
| [internal/gateway/portal/router/router.go:433](internal/gateway/portal/router/router.go#L433) | — | `POST /network/tariff-plans` |
| [portal-frontend/src/api/client.ts:1618](portal-frontend/src/api/client.ts#L1618) | — | `networkTariffsApi.createPlan({template_id?/sub_account_id?, country, sender_category, traffic_type, strategy})` |
| [portal-frontend/src/pages/network/NetworkTariffEditorPage.tsx:234](portal-frontend/src/pages/network/NetworkTariffEditorPage.tsx#L234) | Empty-state без CTA; текст отсылал к несуществующему «уровню платформы» | Кнопка «+ Создать тарифный план» в empty-state. После успеха — toast + refetch → матрица открывается с одной ступенью |

### Verify (трёхуровневая консистентность)

- **UI**: кнопка «+ Создать тарифный план» на `/network/tariffs/editor/bc578c3b-...?mode=template` → матрица с period «2026-04-23 — бессрочно (активный)», strategy=fixed, 6 операторов × tier 0+ × 0.00 ₽.
- **API**: `GET /portal/v1/network/tariff-editor/bc578c3b-...` после клика возвращает `plan.id=75c5d896-...`, `active_period_id` заполнен, массив `tiers` непустой.
- **БД**: `reseller_tariff_plans` — 1 строка (sender_category=paid_registered, traffic_type=any, strategy=fixed), `reseller_tariff_periods` — 1, `reseller_tariff_tiers` — 1. SQL-вердикт:
  ```
  75c5d896-54fa-449d-b498-4435d5dd5c82|paid_registered|any|fixed|periods=1|tiers=1
  ```

### Infrastructure Check: PASS

- Миграция 000099 сделала `reseller_tariff_periods.end_date` nullable — код вставляет NULL для «бессрочного» периода.
- Таблица `countries` заполнена (RU/KZ/BY резолвятся).
- Unique-index `idx_reseller_plan_template_dims` корректно ловит 409 при повторном создании того же сочетания.

### Deploy

- Commit `7570872` — CreatePlan backend + frontend CTA. Build + docker-compose up завершились (portal-gateway healthy, portal-frontend up).

### Известные ограничения (техдолг)

- Дефолтные цены = 0 — в prod это дыра. На sandbox-сервере ok. Следующий шаг: выбор strategy и стартовых цен в диалоге создания плана, а не дефолт.
- UI не даёт удалять план (`DELETE /network/tariff-plans/{id}` есть как `reseller_tariff_plans.DeletePlan`, но не подвязан к /network/*).

---

## [DONE] Модуль: Управление сетью — раунд 3 (aggregator, /network/*, fix + инфраструктура + QA full, 2026-04-23)

Проверил статус backlog из раундов 1-2. Часть пунктов уже исправлена (SubAccountDetail handleTransfer isNaN-guard — уже стоит, Statistics groupByToSliceType — уже включает time-based). Дожал 3 реальных бага.

### Найдено и исправлено

| # | Severity | Файл | Было | Стало |
|---|---|---|---|---|
| B11 | HIGH reliability | [reseller_moderation.go](internal/gateway/portal/handlers/reseller_moderation.go) Approve/Reject/RequestRevision | UPDATE статуса + INSERT history — два отдельных `pool.Exec`. Ошибка INSERT игнорировалась (`_, _ = ...`) → status был коммитнут, а строка в `operator_registration_history` могла потеряться. Дыра в audit-trail | pgx-транзакция: UPDATE и INSERT коммитятся вместе или откатываются вместе |
| B12 | MED defensive | [NetworkRoutingPage.tsx:100](portal-frontend/src/pages/network/NetworkRoutingPage.tsx#L100) handleBulkAssign | `(result.results as any[]).filter(...)` — если backend вернёт `results: null` / `undefined`, бросает `TypeError: filter of undefined`, модалка зависает в `bulkSubmitting=true` | `Array.isArray(result?.results) ? ... : []` — graceful fallback |
| B13 | LOW UX | [NetworkTariffsListPage.tsx:56](portal-frontend/src/pages/network/NetworkTariffsListPage.tsx#L56) filter | `search.toLowerCase()` — пробел в начале/конце строки → нулевой результат при «видимом» совпадении | `search.trim().toLowerCase()` |

### Проверено и отклонено (уже исправлено раньше или ложная тревога)

- **SubAccountDetailPage handleTransfer**: `isNaN(amount) || amount <= 0` уже стоит на [строке 234](portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx#L234). Backlog из раунда 1 устарел.
- **NetworkStatisticsPage groupByToSliceType**: `SUPPORTED_SLICE_TYPES` уже включает time-based (`5min`, `15min`, `hour`, `day`, `month`, `year`). Drill-down drawer не теряет строки.
- **NetworkTariffEditorPage empty periods**: статический анализ не подтвердил краш — `TariffMatrix` принимает пустой массив без падения.

### Инфраструктура (Infrastructure Check: yes)

- Три handler'а модерации теперь в транзакции. **Не проверены в этом раунде**: `sub_account_service.CreateSubAccount` (multi-step), `reseller_tariff_plans.go` create/update (3+ INSERT).
- **Audit log**: reseller handlers по-прежнему не пишут в `audit_log` — техдолг.

### Deploy

- `461024f` — B11, B12, B13. Build + deploy завершились (portal-gateway healthy, portal-frontend up).

### Итог трёх раундов

- **13 багов исправлено** суммарно (5+5+3).
- **1 CRITICAL/SECURITY** (B6 round 2 — transfer негативных сумм), **7 HIGH**, **3 MED**, **1 LOW**, **1 NAV**.
- **Открытый техдолг**: audit_log не пишется из reseller handlers; handleDelete для tariff templates — stub; per-field error messages для суб-аккаунтов; полный CRUD PR #32; транзакции на create-субаккаунт.

### Честная оценка QA-full покрытия

Полный UI + API + DB цикл пройден на 4-5 страницах из 9. На остальных — статический анализ + «грузится без ошибок». Полный BVA / матрица переходов / трёхуровневая консистентность по всем 9 за один прогон — вне бюджета одной сессии.

---

## [DONE-round2] Модуль: Управление сетью — прогон 9 страниц, раунд 2 (aggregator, /network/*, fix + инфраструктура + QA full, 2026-04-23, продолжение)

Продолжение после `ScheduleWakeup` возврата. Прошёл оставшиеся 5 страниц (SubAccountDetail, Routing, TariffsList, TariffEditor, Statistics) через UI + API.

### Найдено и исправлено в раунде 2

| # | Severity | Файл | Было | Стало |
|---|---|---|---|---|
| B6 | HIGH/SECURITY | [sub_accounts.go:330](internal/gateway/portal/handlers/sub_accounts.go#L330) TransferBalance | `amount="-10"` → 200 OK, деньги переводились В ОБРАТНУЮ сторону (aggregator изымал средства у субакка без согласия). Финансовый бэкдор | ParseFloat → reject `<= 0` → 400 |
| B7 | MED | TransferBalance | `amount="0"` создавал пустой transfer_id | Тот же check `<= 0` |
| B8 | HIGH | TransferBalance | `amount="abc"` → 500 INTERNAL_ERROR от billing | `strconv.ParseFloat` → 400 |
| B9 | HIGH | UpdateLimits + CreateSubAccount | `daily_limit=-1` или `monthly_limit=-100` → 500 INTERNAL_ERROR ниже по стеку. `initial_balance="abc"` тоже | Валидация `< 0` + ParseFloat для initial_balance → 400 |
| B10 | HIGH | [sub_account_repository.go:27-75](internal/services/client/infrastructure/repository/sub_account_repository.go#L27) | `ListByParentID` / `CountByParentID` не фильтровали soft-deleted (active=false). Удалённые сабакки всплывали во всех dropdown'ах и засчитывались в `max_sub_accounts` лимит | `WHERE parent_client_id=$1 AND active=true` |

### Проверено без багов (UI + API)

- **NetworkRoutingPage**: выпадающий bulk-assign список корректен, негативные сценарии не проверены детально (bulk модалка требует установленного провайдера).
- **NetworkTariffEditorPage (PR #32)**: mode validation работает (`mode=garbage` → 400 с внятным сообщением), `id` парсится как UUID (мусор → 400), страница показывает понятное сообщение «тарифный план не найден» при отсутствии overrides. API validation country/sender_category/traffic_type присутствует.
- **NetworkStatisticsPage**: 3 вкладки, 7 периодов, 11 вариантов группировки — стат-таблица загружается, SavedViews работают. Показатели «Выручка/Себестоимость/Прибыль = 0 ₽» — ожидаемо (нет настроенных тарифов). Minor observation: «Ошибки 0,059» без % сбивает с толку — decimal вместо %.

### Обновлённый deploy лог

- `3a61d01` B1, B2
- `194d8f3` B3 миграция
- `b98817e` B4, B5 (255)
- `7184806` B5 финал (100)
- `1cb2f02` progress.md
- `0093745` B6, B7, B8, B9
- `d6d086c` B10

### Итог двух раундов

- **10 багов исправлены** (5 в раунде 1 + 5 в раунде 2). 1 CRITICAL/SECURITY (B6 — финансовый бэкдор), 5 HIGH (B2, B3, B5, B8, B9, B10), 2 MED (B1, B7), 1 NAV (B4).
- **Страниц прошло UI + API верификацию**: 9/9 (пусть и в разной глубине — на TariffEditor пустое состояние без overrides ограничило BVA на ценах).
- **Backlog отложенных багов** (см. раунд 1): handleDelete stub шаблонов, XSS-payload в имени (React по дефолту защищает), однообразные сообщения валидации, tariff CRUD неполный, audit_log не пишется из reseller/tariff handlers.
- **Критичный финансовый bug (B6)** — был самым серьёзным. До фикса любой владелец aggregator-аккаунта мог через `POST /sub-accounts/{id}/transfer {"amount":"-X"}` снять средства с любого своего субакка без согласия владельца. Тип escalation: внутренний abuse (aggregator уже имеет write-доступ к субакку по модели), но семантически «перевод X от A к B» → `-X` не должно инвертировать направление.

---

## [DONE-round1] Модуль: Управление сетью — прогон 9 страниц (aggregator, /network/*, fix + инфраструктура + QA full, 2026-04-23)

Scope: все роуты под `/network`. Полный QA full с UI-верификацией пройден по 4 страницам (Dashboard, SubAccountsList, Moderation, TariffTemplates). Остальные 5 страниц (SubAccountDetail, Routing, TariffsList, TariffEditor, Statistics) — статический анализ кода + браузерная проверка грузится-без-ошибок, но без полноты BVA/state-transition. Честно: обратного контекста не хватило на полный full QA по всем 9.

### Найдено и исправлено

| # | Severity | Файл | Было | Стало | Верификация |
|---|---|---|---|---|---|
| B1 | MED UX | [reseller_dashboard.go:304-310](internal/gateway/portal/handlers/reseller_dashboard.go#L304) | Alert «Проблемные субаккаунты» показывал `detail: "0.000000 руб."` — 6 знаков + суффикс без подписи | `fmt.Sprintf("Баланс: %.2f ₽", f)` — двухзнаковый формат, префикс «Баланс:», правильный символ | API после деплоя возвращает `"Баланс: 0.00 ₽"` ✅ |
| B2 | HIGH | [sub_account_service.go:87-99](internal/services/client/application/sub_account_service.go#L87) | CreateSubAccount создавал client с пустым `api_key`. `clients.api_key` — UNIQUE. Второй субакк → SQLSTATE 23505 → 500 INTERNAL_ERROR в UI | Генерируется `ak-<hex>` + `secret` аналогично `client_service.CreateClient` | POST /sub-accounts с валидным телом вернул 201 ✅ |
| B3 | HIGH | [migrations/000119](migrations/000119_transactions_allow_transfer_types.up.sql) | `transactions_type_check` допускал только `charge/credit/refund/adjustment`. Billing-service пишет `transfer_out/transfer_in` для перевода начального баланса → 500 SQLSTATE 23514. Субаккаунт создавался, но с `balance=0` и `balance_transfer_error` в ответе — silent failure | Миграция: CHECK расширен на `transfer_out`, `transfer_in` | После миграции create с `initial_balance=50` → баланс 50.00, `transfer_error: null` ✅ |
| B4 | MED nav | [UserLayout.tsx:100-101](portal-frontend/src/components/layout/UserLayout.tsx#L100) | Сайдбар «Управление» содержал пункты «Аналитика» (/network/analytics) и «Квота сети» (/network/quota). Роуты убраны после PR #32 (tariffs redesign), ссылки не почищены → клик = 404 | Пункты удалены. Страницы `NetworkAnalyticsPage`, `NetworkQuotaPage` оставлены в коде на случай повторной активации | После перезагрузки frontend — в сайдбаре только 6 пунктов ✅ |
| B5 | HIGH | [network_tariff_templates.go:177-192](internal/gateway/portal/handlers/network_tariff_templates.go#L177) | POST /network/tariff-templates с `name` > 100 символов → 500 SQLSTATE 22001 (`value too long for type character varying(100)`). Описание вообще без лимита → 201 даже на 50KB | `utf8.RuneCountInString(name) > 100` → 400 «имя слишком длинное». Description capped at 10000 chars | Деплой в процессе (build ID booyqu7go) |

### Не исправлено, отложено (honest backlog)

| # | Severity | Место | Проблема | Почему не сегодня |
|---|---|---|---|---|
| B6 | HIGH | NetworkTariffTemplatesPage.tsx:66-68 | `handleDelete` — stub, endpoint `DELETE /network/tariff-templates/{id}` не существует. Кнопка «Удалить» дизейблена, но при JS-патче — no-op | Требует реализовать backend endpoint + confirm dialog. Отдельная задача |
| B7 | MED/XSS | network_tariff_templates.go Create | `<script>alert(1)</script>` принят как имя шаблона (201). React по дефолту экранирует в JSX, но если где-то есть `dangerouslySetInnerHTML` — будет XSS | Санитизация на беке (или явный escape на рендере каждого места) |
| B8 | MED | sub-accounts API | 6 разных негативных кейсов (пустая отправка, xss, negative numbers, long text, duplicate email) возвращают один и тот же текст `"Неверный формат запроса"` — пользователь не понимает что именно не так | Требует рефакторинга валидатора с per-field messages |
| B9 | MED | NetworkTariffsListPage | handleDelete и другие mutation endpoints для tariff-редизайна не реализованы (согласно коду — часть TODO) | На уровне backend несколько endpoints ещё не готовы (удаление, редактирование metadata шаблона) |
| B10 | LOW | Dashboard — API возвращает `problem_sub_accounts` с `detail=0.00` для ВСЕХ сабакков у которых баланс < 100 — включая случаи когда суб-аккаунт в принципе никогда не использовался. Чистого «здесь нет проблемы, просто новый аккаунт» нет | Семантика «проблемный» vs «новый» — отдельное UX-решение |

### 5 страниц — без полной UI-верификации, только статический анализ

**SubAccountDetail** ([SubAccountDetailPage.tsx](portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx)): 7 табов, API-endpoints существуют. Потенциальные риски (не проверены в UI): `handleTransfer` — `parseFloat(transferAmount)` без `isNaN`; hard delete без подтверждения pending операций.

**NetworkRoutingPage**: `bulkAssignProvider` результат парсится как `(result.results as any[]).filter(...)` без null-guard. Race на загрузке `allProviders` vs `providers` для конкретного субакка.

**NetworkTariffsListPage** (PR #32, новая): `search.toLowerCase()` без `trim()`; `onlyOverrides && override_count > 0` — взаимоисключает результат если нет оверрайдов И есть поиск.

**NetworkTariffEditorPage** (PR #32, новая): `activePeriod = data.periods.find(...) ?? data.periods[0]` — если periods пусто, undefined пропускается в TariffMatrix; `unsaved changes guard` работает только при навигации, не при смене фильтров внутри страницы.

**NetworkStatisticsPage**: `groupByToSliceType()` fallback `'provider'` — при `group_by = 'time_5min'` возвращает mismatched rows; window.prompt/confirm для SavedViews — блокирующий UI при медленной сети.

### Инфраструктура (Infrastructure Check: yes)

- **Миграции**: `000098_reseller_tariff_tables.up.sql` создаёт `reseller_tariff_{templates,plans,periods,tiers}` + `sub_account_template_assignments`. `000119` добавлен для `transactions_type_check`.
- **Audit log**: таблица `audit_log` существует, **но reseller/tariff handlers в неё НЕ пишут** — это техдолг для будущего security audit (нет истории: кто создал/удалил/изменил template, approved sender name и т.д.).
- **Redis cache**: `tariffs:summary:<reseller_id>` инвалидируется после коммита транзакций — best-effort (если DEL fails, stale cache, вероятность низкая).
- **Нет транзакций** для multi-table мутаций в `reseller_moderation.go` (UPDATE status + INSERT history отдельно) — при ошибке INSERT `status='approved'` уже закоммичен. Не исправил в этом прогоне — отдельная задача.

### Deploy

- `3a61d01` — B1 + B2
- `194d8f3` — B3 (миграция 000119 применена)
- `b98817e` — B4 + начало B5 (255 — ошибочный лимит)
- `7184806` — B5 финальный (корректный лимит 100)

### Итог

- **5 багов исправлены и 4 из них верифицированы в UI**. B5 — деплой идёт, не завершён на момент написания отчёта.
- **5 багов отложены в backlog** с обоснованием.
- **5 страниц** из 9 **прошли только статическую проверку** — честно признаю, полный `fix full` на всех 9 за один прогон — объём вне бюджета одной сессии. Пользователь выбрал этот режим явно (вариант 3); я его выполнил частично и открыто маркирую недоделанное.
- **Критичный путь workflow** (создать субаккаунт → перевести баланс → создать шаблон тарифа) — **починен end-to-end** (B2 + B3 + B5). До этого этот путь не работал вообще.

---



## [DONE] Модуль: Отправка сообщений и рассылок — спринт 3 (aggregator + subaccount, fix, 2026-04-22)

Scope: хвост UX/security-багов из предыдущих спринтов. Все P2/P3 закрыты, остаётся B5 (500→400 validation codes) и B8 (seed тарифов) — требуют отдельной работы.

### Исправлено (commit 222d824)

| # | Severity | Файл | Было | Стало |
|---|---|---|---|---|
| B2 | HIGH (security) | [handlers/messages.go](internal/gateway/portal/handlers/messages.go) | CSV-экспорт не экранировал `=`/`+`/`-`/`@` в начале ячеек — Excel/Calc исполнит как формулу (CSV formula injection, CVE-класс) | `csvSanitize()` префиксует апострофом ячейки, начинающиеся с триггер-символов. Инъекция в теле SMS теперь безопасна при открытии экспорта |
| B9 | MED (ops) | [scripts/maintenance/fix_historical_pending.sql](scripts/maintenance/fix_historical_pending.sql) | 3 исторических `pending` сообщения QA-теста оставались в pending навсегда, сбивали агрегатное delivery rate | SQL-скрипт с scope по client_id-списку (не тронет load-test) маркирует их как `rejected` + status_message «legacy pre-fix». Применено: 21 запись из тест-аккаунтов |
| B10 | MED (UX) | [CampaignWizardPage.tsx:810](portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx#L810) | Итого `0,00 ₽` для субаккаунта без pricing_rules → misleading, выглядит как «бесплатно» | При `estimated_cost == 0` показываем `—` и amber-панель: «Тариф не настроен. Стоимость будет рассчитана при отправке по тарифу оператора/агрегатора» |
| B11 | MED (UX) | [CampaignDetailPage.tsx:100-114](portal-frontend/src/pages/campaigns/CampaignDetailPage.tsx#L100) | Polling 5с только при `running`/`materializing`. Быстрая рассылка (3 получателя) завершалась за 1-2с — UI залипал на «Подготовка» | 3с интервал, все не-терминальные статусы (`!= completed/cancelled/draft`). Включая `scheduled` и `paused` |
| B12 | LOW (nav) | [CommandCenter.tsx:629](portal-frontend/src/pages/CommandCenter.tsx#L629) | «Управление суб-аккаунтами → /sub-accounts» у reseller — 404 | `/network/sub-accounts` (аналогично B7 спринт 2) |

### Остаток bug-list (не блокирующие, отдельный backlog)

- **B5** (LOW/UX): validation-ошибки возвращаются как `HTTP 500 INTERNAL_ERROR` вместо `400 INVALID_INPUT`. Требует рефакторинга префиксов в messaging-service validator.
- **B8** (LOW/config): в dev-seed нет тарифов агрегатора на все операторы (Default-RU для shared, Ростелеком). Ручной fix применён во время аудита.

### Все задеплоенные коммиты (4 спринта по модулю отправки)

- `7bad29e` — sender publish rejected для детерминированных отказов, transient → Kafka redelivery
- `7b348d1` — persist-vs-status race, retry через failedBuffer
- `4dcbfed` — закрытие спринта 1 в progress.md
- `c910e53` — B3 (длина utf8-рун), B4 (approved-sender validation), B6 (warn-лог подмены), B7 (nav)
- `24ee358` — закрытие спринта 2
- `222d824` — B2 (CSV formula injection), B9 (historical cleanup), B10 (cost UX), B11 (polling), B12 (nav)

### Итоговое состояние модуля отправки

- **Pipeline:** pending-forever устранён (end-to-end через браузер + API). Race с persist — retry-buffer. `status_message` пробрасывается в БД и UI.
- **Backend API /messages:** utf8-лимит text 1600 символов; approved sender validation (403 если не принадлежит клиенту); CSV export защищён от formula injection.
- **Router warn-лог** при подмене sender на fallback для observability.
- **Agg-view:** `/network/dashboard` + `/network/sub-accounts/:id` → «Сообщения» — корректно агрегирует и показывает трафик субаккаунтов. Все навигационные ссылки у reseller ведут на правильные роуты.
- **Campaign flow:** создание→запуск→детализация работает end-to-end. Cost-estimate честный (— вместо обманчивого 0,00 ₽). Polling 3с показывает completed сразу.
- **IDOR изоляция:** subacc не видит чужие сообщения, agg через `/messages/:id` тоже 403 — правильно, сетевой просмотр только через reseller-path.



## [DONE] Модуль: Отправка сообщений и рассылок — спринт 2 (aggregator + subaccount, fix, 2026-04-22)

Scope: валидация API, UX при подмене sender, битая навигация, end-to-end CampaignWizard.

### Исправлено (commit c910e53, задеплоено)

| # | Severity | Файл | Было | Стало |
|---|---|---|---|---|
| B3 | HIGH (billing risk) | [handlers/messages.go](internal/gateway/portal/handlers/messages.go) | API принимал любую длину text. Отправил 2000 символов → принято | `utf8.RuneCountInString > 1600` → HTTP 400 с сообщением «Поле text слишком длинное (N символов, максимум 1600)». Подтверждено curl-ретестом |
| B4 | HIGH (security) | [handlers/messages.go](internal/gateway/portal/handlers/messages.go) | API принимал любой `source`, в том числе несуществующий (`NotApproved`). Router молча подменял на "SMS" | До вызова messaging-service: SELECT из `sender_names WHERE client_id=caller AND name=source`. `pgx.ErrNoRows` → 403 «не принадлежит вашему аккаунту», status!='approved' → 403. Цифровые sender (shortcode) пропускаются |
| B6 | MED (observability) | [router/stage.go](internal/pipeline/router/stage.go) | При подмене sender на fallback ("SMS") — нет логов, оператор не понимает почему клиент видит в БД не то имя | `log.Warn` с полями `client_id, operator_id, original_sender, fallback, reason` в обеих ветках (sub-account без approved, direct client без operator_registrations) |
| B7 | LOW (nav) | [CampaignsPage.tsx](portal-frontend/src/pages/campaigns/CampaignsPage.tsx) | Ссылка «Суб-аккаунты» в инфо-панели reseller вела на `/sub-accounts` (404 для агрегатора) | `/network/sub-accounts` — роут `NetworkLayout`, защищённый `RequireReseller` |

### E2E Campaign Wizard через Playwright (subacc: subacc@test.local)

Путь: `/campaigns/new` → Шаг1 «QA Campaign E2E test message», Trest → Шаг2 `TestCampaignList (3 контакта)` → Шаг3 «Сейчас» → Шаг4 «Отправить» → редирект на `/campaigns/dc24ad86-...`.

**БД после запуска:** `campaigns.status=completed`, `total_recipients=3`, `failed_count=3`, `started_at` заполнен. Wizard-поток работает.

### Найденные на спринте 2 баги (не блокирующие)

| # | Severity | Место | Описание | Предложение |
|---|---|---|---|---|
| B10 | MED | CampaignWizard шаг4 Confirm | «Итого 0,00 ₽» для 3 получателей × 1 сегмент у субаккаунта. Cost-estimate endpoint не учитывает subaccount per-SMS тариф агрегатора | Добавить в `campaignsApi.estimateCost` fallback на `aggregator_tariffs` для subaccount, либо корректное сообщение «Тариф не настроен, стоимость будет рассчитана после запуска» вместо 0,00 ₽ |
| B11 | MED | `/campaigns/:id` страница детализации | После запуска страница показывает статус «Подготовка» 0/0/0/0. БД через 3 секунды уже `completed, failed_count=3`. UI не polling и не обновляется без F5 | Добавить SSE или короткий polling аналогично QuickSend (3s интервал, MAX 60 попыток). QuickSend уже реализовано в [QuickSendPage.tsx:92-128](portal-frontend/src/pages/quick-send/QuickSendPage.tsx#L92) — переиспользовать паттерн |
| B12 | LOW (nav) | [CommandCenter.tsx](portal-frontend/src/pages/CommandCenter.tsx) (reseller view) | Карточка «Низкий баланс» → ссылка «Управление суб-аккаунтами → /sub-accounts» у агрегатора. Должно `/network/sub-accounts` (аналогично B7) | `const href = isReseller ? '/network/sub-accounts' : '/sub-accounts'` |

### Статус остальных багов из спринта 1

- B2 (XSS text): не зафикшено на этом спринте. Риск снижен частично (React escape в таблице), но payload всё ещё попадает в `messages.text` и уходит к провайдеру. Отложено.
- B5 (500 вместо 400 для validation): не зафикшено. Требует рефакторинга префиксов ошибок в messaging-service.
- B8 (отсутствие тарифа на Ростелеком): конфиг, частично закрыт ручным INSERT aggregator_tariffs, но требует seed update.
- B9 (исторические pending сообщения): закрывается одним SQL-скриптом, который можно запустить по запросу.

### Итог

- **Основной UX-баг (pending-forever)** — устранён (спринт 1, commits 7bad29e + 7b348d1).
- **Безопасность отправки** — усилена: лимит длины, валидация sender принадлежности клиенту и approved-статуса, warn-лог при подмене (спринт 2, c910e53).
- **Агрегатор видит трафик субаккаунта** — ✅ через `/network/dashboard` и `/network/sub-accounts/:id` (включая Messages tab).
- **Campaign Wizard end-to-end** — ✅ создаёт/запускает рассылку, БД обновляется. Остались UX-баги отдельного screen'а (cost=0, не-polling detail).

Итоговые задеплоенные коммиты: `7bad29e`, `7b348d1`, `4dcbfed`, `c910e53`.



## [DONE] Модуль: Отправка сообщений и рассылок (aggregator + subaccount, fix + инфраструктура + QA full, 2026-04-22)

Старт/финиш: 2026-04-22. Учётки: subacc@test.local (client a0000000-...-000000000002), aggregator@test.local (client a0000000-...-000000000001, is_reseller=t). Тест-объекты: sender «Trest» (subacc) и «AuditTest» (agg), контактные базы, 2 тарифа агрегатора.

### Критические фиксы (задеплоены)

| # | Severity | Файл | Коммит | Было | Стало |
|---|---|---|---|---|---|
| 1 | CRITICAL | `internal/pipeline/sender/stage.go` | 7bad29e | При `tarification_rejected`/`frozen` sender молча вызывал `session.MarkMessage` → messages.status остаётся `pending` навсегда. UI показывает «Ожидание», клиент не понимает почему сообщение не идёт. Воспроизведено: 3 сообщения субаккаунта и агрегатора застряли в pending ≥30 мин | `publishRejectedStatus()` публикует `SentMessage{Status:"rejected", ErrorMessage}` в `sms.sent` для детерминированных отказов (frozen, tarification_rejected). Для transient (billing_unavailable, tarification_error) — return error → Kafka redelivery вместо потери |
| 2 | CRITICAL (race) | `internal/pipeline/status/stage.go` | 7b348d1 | status-stage получает `sms.sent{rejected}` раньше, чем persist-stage успел INSERT. `UPDATE ... WHERE updated_at < s.updated_at` → 0 rows, rejected-статус теряется | `batchUpsert` возвращает `errUpsertPartial` когда `RowsAffected < len(records)` → `failedBuffer` → retry через `retryLoop` с обновлением `UpdatedAt = time.Now()` (обходит race) |
| 3 | HIGH (ux) | `internal/pipeline/status/stage.go` | 7b348d1 | `ErrorMessage` из `SentMessage` игнорировался status-stage → `messages.status_message` оставался NULL, пользователь видел «Отклонено» без причины | `statusRecord.StatusMessage` пробрасывается через COPY temp-table + UPDATE. `submitted_at` не ставится для rejected (сообщение не уходило провайдеру) |

### Найденные, не исправленные баги (добавлены в отложенный bug-list)

| # | Severity | Place | Описание | Предложение |
|---|---|---|---|---|
| B2 | HIGH (security) | `POST /portal/v1/messages` | Payload `<script>alert(1)</script>` **принимается** и сохраняется в `messages.text` as-is. React рендерит escaped в таблице, но CSV-экспорт и будущие `dangerouslySetInnerHTML` — реальный риск. Также уходит к провайдеру как тело SMS | Санитизировать/отклонять `<script>`, SQL-шаблоны в теле; либо строго enforce «plain text» на backend |
| B3 | HIGH (billing risk) | `POST /portal/v1/messages` | Текст >160 символов принимается API без лимита. Отправил 1000 символов = 7 сегментов, за которые спишется. Frontend ограничивает 765, но API голый | Ввести жёсткий лимит на backend (напр. 1600 симв = 10 сегментов max) с 400 response |
| B4 | MED | `POST /portal/v1/messages` | Непроверенный sender name (`NotApproved`) принимается как queued. Нет валидации «sender ∈ approved_sender_names_of_client» | Проверять `sender_names.name == req.source AND client_id == caller AND status = 'approved'` в handler. 403 если не найдено |
| B5 | LOW (ux) | `POST /portal/v1/messages`, `INTERNAL_ERROR` 500 | Валидационные ошибки (source > 20 символов, нецифровой destination) возвращаются как HTTP 500 `INTERNAL_ERROR` вместо 400 `INVALID_INPUT`. `{"error":{"code":"INTERNAL_ERROR","message":"validation failed: validation failed: validation error for field source..."}}` — два "validation failed" префикса | Wrap validation errors в `shared.ErrInvalidInput` до вызова gRPC; унифицировать префиксы |
| B6 | MED | `/messages` (agg send, source → 'SMS') | Отправил с `source:"AuditTest"` агрегатор, в БД `messages.source = 'SMS'`. Воспроизведено на TC-1 happy retry. Возможно, это нормализация/substitute в messaging-service при некорректной sender_category. Нужно отследить | Добавить строгую проверку в messaging-service: если sender не находится в whitelist → rejected с ясной причиной, а не substitute |
| B7 | LOW | `portal-frontend/src/pages/campaigns/CampaignsPage.tsx` | Инфо-панель "Рассылки суб-аккаунтов доступны в разделе [Суб-аккаунты](/sub-accounts)" — для агрегатора ведёт не туда. Агрегатор живёт в `/network/sub-accounts` | Завязать href на `isReseller`: `/network/sub-accounts` для реселлера, `/sub-accounts` иначе |
| B8 | LOW (config) | Тарификация | В тестовой БД нет тарифа агрегатора для Ростелекома (оператор 10000000-...-000000000005) и нет унифицированного тарифа для sender_category=shared на Default-RU. Результат: все отправки на префиксы 7990-7999 отклоняются. Добавил `aggregator_tariffs` на Ростелеком в QA-проходе — happy-path стал проходить у агрегатора | Прогнать seed чтобы у тестовых агрегаторов был full оператор-matrix. Плюс проработать UX для случая «нет тарифа на оператора» — сейчас просто rejected, без явного намёка клиенту, что нужно обратиться к агрегатору |
| B9 | LOW | Исторические `pending` сообщения | 3 сообщения, отправленных до фикса (03071720, deb41b04, aec884da), остаются в `pending` навсегда — sender их повторно не обработает, запись в `sms.sent{rejected}` для них не публиковалась | Одноразовый migrate-скрипт: `UPDATE messages SET status='rejected', status_message='historical pre-fix' WHERE status='pending' AND created_at < '2026-04-22 19:30 UTC'`. Сделал в ходе аудита вручную |

### Проверка Q (API + БД) и инфраструктуры

| Что | Ожидание | Факт |
|---|---|---|
| `POST /messages` subacc с невалидным тарифом | rejected + status_message | **после фикса**: status=rejected, status_message="no active tariff plan for operator and sender category", submitted_at=NULL ✅ |
| `POST /messages` валидная конфигурация | sent/delivered | Не протестировано до конца — тариф исправлен локально, но есть TC-B6 (source substitute). Отложено |
| QuickSend UI (subacc) — 3 номера батчем | 3 строки «Отклонено» | ✅ UI показал все 3 строки с корректным label «Отклонено» из STATUS_LABELS, polling работает |
| Agg → `/network/dashboard` | Видит суб-аккаунтов, балансы, трафик | ✅ 4 субаккаунта, баланс 137 557,60 RUB (свой 87 581,50 + сеть 49 976,10), топ-5 SMS, DR 38.8% |
| Agg → `/network/sub-accounts/:subId` «Сообщения» | Видит все сообщения субаккаунта | ✅ Видит включая наши QA-отправки (`UI-QA test 1`, `QA-TC1-*`, XSS-payload и др.) |
| IDOR: subacc GET чужое message | 403/404 | 403 ✅ |
| IDOR: agg GET subacc message через `/messages/:id` (не сетевой endpoint) | 403 — у агрегатора отдельный reseller endpoint | 403 ✅ (сетевой просмотр работает только через `/network/sub-accounts/:id/messages`) |
| Container health (25 сервисов) | healthy | 24 healthy, dev-контейнер без health-probe — ожидаемо |
| Kafka топики в потоке | sms.raw→routed→sent→status | ✅ router+persist+sender+status обработали новые сообщения; status retry buffer работает |
| messages CHECK constraint | содержит 'rejected' | ✅ подтверждено `messages_status_check` |

### Итог по scope

- **Главный запрос пользователя** («проверь, что агрегатор корректно видит новые данные») — **PASS**. Reseller dashboard агрегирует балансы и трафик; вкладка «Сообщения» субаккаунта в `/network/sub-accounts/:id` показывает весь трафик субаккаунта (включая только что отправленный в QA-прогоне).
- **Критический UX-баг** (pending-forever) устранён в двух коммитах. Проверено end-to-end через браузер: 3 батчевые отправки субаккаунта → UI показывает «Отклонено» с polling'ом, в БД `status=rejected`, `status_message` заполнен, `submitted_at=NULL`.
- **Race condition persist vs status** (регрессия из предыдущих изменений pipeline) — исправлена через `errUpsertPartial`/retry.
- 7 дополнительных багов (XSS, длина, validation codes, source substitute, битая навигация) зафиксированы в bug-list, не блокирующие.



## [DONE] Модуль: Панель субаккаунта — последовательный обход всех страниц (subaccount, fix mode + инфраструктура + QA full, 2026-04-22)

Запущен: 2026-04-22, тест-аккаунт `subacc@test.local` / `Test1234!`, client_id `a0000000-...-000000000002`, parent `a0000000-...-000000000001`, баланс 49 976,10 ₽. Обошёл 25 модулей через браузер + API + БД.

### Исправлено

| # | Severity | Файл | Было | Стало |
|---|---|---|---|---|
| 1 | MED UX | `portal-frontend/src/pages/CommandCenter.tsx` | Карточка «Провайдеры» на дашборде субаккаунта показывала «Нет провайдеров» — вводит в заблуждение (субаккаунт не владеет SMPP, трафик идёт через агрегатора) | Передаём `isSubAccount` в `HealthMap`, для субаккаунта показываем пояснение «Сообщения отправляются через инфраструктуру агрегатора» |
| 2 | MED UX | `portal-frontend/src/pages/messages/MessagesPage.tsx` | Подзаголовок «Детализация трафика по всем клиентам и каналам» показывался и не-реселлерам. Фильтр «Суб-аккаунт» и колонка «Логин» по умолчанию тоже | Подзаголовок завязан на `isReseller`. Фильтр `login` и дефолтная колонка скрыты для не-реселлеров; два разных дефолтных набора `DEFAULT_VISIBLE_RESELLER`/`DEFAULT_VISIBLE_CLIENT` |
| 3 | MED UX | `portal-frontend/src/pages/messages/components/MessageTable.tsx` | Колонка «Стоимость» рендерила backend-строку `1.800000 ₽` | `parseFloat` + `toLocaleString('ru-RU', {minimumFractionDigits:2, maximumFractionDigits:2})` → `1,80 ₽` |
| 4 | CRITICAL security | `internal/gateway/portal/handlers/tariffs.go` | `POST /portal/v1/tariffs/change` позволял субаккаунту сменить подписочный план через прямой вызов API (UI не показывал, но endpoint не проверял `parent_client_id`) → субаккаунт смог переключить себя на Free/Trial в ходе аудита | Добавил вызов `clientClient.GetClient` в начале хендлера: если `ParentClientId != ""` → `ErrForbidden`. План субаккаунта возвращён в NULL вручную в БД |
| 5 | HIGH UX | `portal-frontend/src/pages/tariffs/TariffsPage.tsx` | Субаккаунту показывался полный грид подписочных планов (Free/Starter/Business/Pro) с кнопками «Выбрать» — бессмысленно и вводит в заблуждение: биллинг субаккаунта per-SMS от агрегатора | Для `parent_client_id != null` раньше return с информационной панелью: «Подписочный тариф не используется, списания идут по per-SMS тарифу агрегатора» |
| 6 | CRITICAL security (IDOR) | `internal/gateway/portal/handlers/routes.go` | `/portal/v1/routes` (ListRoutes / GetRoute / CreateRoute / UpdateRoute / DeleteRoute) защищены только аутентификацией, без скоупинга по client_id и без admin-role middleware. Любой залогиненный пользователь видел все 44 роута других клиентов, мог редактировать и удалять чужие | Добавил `isPrivilegedRole` (admin/superadmin). Non-admin: ListRoutes принудительно `filters.ClientID = callerID`; CreateRoute запрещает `client_id != callerID`; GetRoute/UpdateRoute/DeleteRoute читают запись до мутации и возвращают 404 если `existing.ClientID != callerID`. После фикса субаккаунт видит 5 своих роутов (было 44) |
| 7 | HIGH UX | `portal-frontend/src/pages/providers/ProvidersPage.tsx` | Субаккаунт видел кнопку «+ Добавить провайдера» на пустом экране SMPP-провайдеров, хотя не владеет провайдерами | Для субаккаунта скрываю кнопку + показываю info-панель «SMPP-провайдеры настраивает агрегатор» |
| 8 | MED UX | `portal-frontend/src/components/layout/UserLayout.tsx` | Пункты «Провайдеры» и «Маршрутизация» в левом меню были всегда видны клиенту | `buildOwnNavGroups(isSubAccount)` исключает эти пункты для субаккаунта (роуты остаются доступны напрямую по URL, но меню не приглашает) |

### Проверка изоляции API

| Эндпоинт | Ожидание | Факт |
|---|---|---|
| `GET /portal/v1/sub-accounts` (субаккаунт) | 403 | 403 ✅ |
| `GET /portal/v1/reseller/dashboard` | 401/403 | 401 (минорная непоследовательность с `sub-accounts`, оставлено) |
| `GET /portal/v1/reseller/analytics` | 401/403 | 401 |
| `GET /portal/v1/reseller/routing/routes` | 401/403 | 401 |
| `GET /portal/v1/reseller/moderation/counts` | 401/403 | 401 |
| `GET /portal/v1/routes` | только свои | **после фикса**: 5 (было 44) ✅ |
| `POST /portal/v1/tariffs/change` с чужим plan_id | 403 | **после фикса**: 403 ✅ (было 200 + успешный switch) |
| `POST /portal/v1/routes` с `client_id` другого клиента | 403 | **после фикса**: 403 ✅ |
| `GET /portal/v1/quota` | 200 null | 200 null (эндпоинт технически открыт, но квот нет — низкий риск) |
| `/network/dashboard` через браузер | redirect | `/command-center` ✅ (RequireReseller работает) |

### Трёхуровневая консистентность (выборочно)

- Баланс: UI `49 976,10 ₽` = API `/billing/balance` = `49976.100000` = БД `accounts.balance = 49976.100000` ✅
- Шаблоны: UI пусто; API `/templates` total=0; БД `templates` у client_id=`...002` — 0, у parent — 1 (ожидаемо: нет назначений `sub_account_template_assignments`) ✅
- Sender names: UI «Trest Одобрено», БД `sender_names` 1 строка approved ✅

### Не исправлено (в отложенный bug-list)

| Severity | Место | Описание |
|---|---|---|
| MED | `AuditLogPage` фильтр «Действие» | Опции «Суб-аккаунт создан/удалён», «Лимит суб-аккаунта изменён» бессмысленны для субаккаунта — от них нет записей в его журнале |
| LOW | `ProfilePage` | Субаккаунт не видит имя агрегатора — добавить карточку «Родительский аккаунт: <name>» |
| LOW | `/quota` для субаккаунта | Возвращает 200 + null — семантически должен 403 или «endpoint not applicable», но не утечка данных |
| LOW | `/reseller/*` 401 vs `/sub-accounts` 403 | Непоследовательные коды для одного и того же класса запрета |
| MED | Описания транзакций биллинга | «SMS subaccount» / «SMS tarification: fixed, 1 segments» / «SMS субаккаунт: тариф агрегатора» — 4 разных формата в одной таблице; требует унификации на бэке |
| LOW | Шаблоны (M09) | При отсутствии назначений от агрегатора показывается просто «Шаблоны не созданы» — надо явно сказать субаккаунту «назначает агрегатор» |

### Тест-аккаунты в прогрессе

| Email | Роль | Client | Назначение |
|---|---|---|---|
| `subacc@test.local` | client (subaccount) | a0000000-...-000000000002 | Обход панели субаккаунта 2026-04-22 |



## [DONE] Модуль: Управление сетью — полный повторный аудит (aggregator, /network + все подстраницы, fix mode + инфраструктура + QA full, 2026-04-22)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `portal-frontend/src/App.tsx`, `portal-frontend/src/components/layout/NetworkLayout.tsx`, `portal-frontend/src/components/layout/UserLayout.tsx` | `NetworkQuotaPage` и `NetworkAnalyticsPage` написаны, API (`/quota`, `/reseller/analytics`) работают, но роуты не подключены → мёртвый код, функции недоступны через UI | Добавлены роуты `/network/analytics` и `/network/quota` + пункты меню «Аналитика», «Квота сети» в обоих layout |
| 2 | `internal/gateway/portal/handlers/reseller_dashboard.go` | N+1 gRPC-запросов: `GetBalance` по одному на каждый субаккаунт последовательно, `GetStatistics` ещё два круга N последовательных вызовов → для агрегатора с 100 субаккаунтами один запрос `/reseller/dashboard` делал 300+ RPC | Все три цикла fan-out в goroutine c `sync.WaitGroup` + `sync.Mutex` — теперь одновременно; время ответа O(1) вместо O(N) |
| 3 | `portal-frontend/src/pages/network/NetworkDashboardPage.tsx` | `data.traffic.delivery_rate.toFixed(1)`, `sa.delivery_rate.toFixed(1)`, `data.moderation_counts.*` — краш при null в любом поле ответа | Все поля через `?? 0` / optional chaining; `formatAmount()` хелпер для `parseFloat` с isNaN проверкой |
| 4 | `portal-frontend/src/pages/network/NetworkDashboardPage.tsx` | `parseFloat(data.network_balance.total)` — если `billingClient == nil` на бэке возвращалась пустая строка → `NaN ₽` в UI | `formatAmount()` возвращает `0.00` при NaN/пустой строке; бэкенд при nil billingClient явно заполняет `"0.00"` вместо пустоты |
| 5 | `internal/gateway/portal/handlers/reseller_dashboard.go` | `Detail: bal + " руб."` — hardcoded `руб.` игнорировал `accounts.currency`; низкий баланс USD-аккаунта показывался как «$X.XX руб.» | SELECT тянет `COALESCE(a.currency, 'RUB')`, detail формируется как `"<balance> <currency>"` |
| 6 | `internal/gateway/portal/handlers/reseller_dashboard.go` | `.Scan(&moderation.SenderNames)` (и 4 других места) — ошибки scan и query молча игнорировались → если запрос падал из-за schema drift, пользователь видел «Нет модерации» при реальных заявках | Все 5 scan/query проверяют err, логируют через `zerolog` с `reseller_id`, ряды в цикле continue при scan-ошибке вместо молчаливого пропуска |
| 7 | `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx` | `parseFloat(sa.balance).toLocaleString(...) + ' ₽'` в колонке «Баланс» — если `balance` = null/undefined/"" → `NaN ₽` в таблице | `parseFloat(sa.balance ?? '')` + `isNaN` fallback на 0 |

### Инфраструктура

| Компонент | Статус |
|---|---|
| Роуты `/network/*` (`App.tsx`) | ✅ Полные: dashboard, sub-accounts, sub-accounts/:id, moderation, routing, tariffs, statistics, analytics (новый), quota (новый) |
| `RequireReseller` middleware (фронт) | ✅ Защищает `/network`, пропускает только `is_reseller = true` |
| `checkReseller()` (бэкенд) | ✅ Все handlers `/reseller/*` проверяют `is_reseller` в БД |
| API `/reseller/dashboard` | ✅ Работает, теперь с параллельным fan-out |
| API `/reseller/analytics` | ✅ Подключён роут (строка 469 router.go) |
| API `/quota`, `/quota/history` | ✅ Подключены (строки 509–513 router.go) |
| Таблица `accounts.currency` | ✅ Существует с миграции 000006, default `'RUB'` с 000057 |
| Навигация (`NetworkLayout`, `UserLayout`) | ✅ Все 8 страниц `/network` представлены в сайдбаре |

### Найденные, но отложенные (не критичные для текущего раунда)

- `SubAccountsListPage` рендерит все субаккаунты без пагинации (`pageSize={subAccounts.length}`) — проблема при 500+ аккаунтах (LOW, tech-debt).
- `NetworkQuotaPage` и `NetworkAnalyticsPage` не проходили отдельный полный аудит — нужен следующий раунд по каждой.
- `SubAccountsListPage.balance` колонка hardcoded ₽, не использует `accounts.currency` из API (LOW).

## [DONE] Модуль: Управление сетью (aggregator, /network/*, fix mode + инфраструктура + QA full, 2026-04-15)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `internal/gateway/portal/handlers/reseller_moderation.go` | `regJSON` не содержал `sub_account_email` → вкладка "Регистрации" показывала сырой UUID субаккаунта | Добавлен `c.email AS sub_account_email` в SQL, поле `SubAccountEmail string` в struct |
| 2 | `portal-frontend/src/pages/network/ModerationPage.tsx` | `regColumns[0].key = 'sub_account_id'` → UUID в ячейке таблицы | `key = 'sub_account_email'` → читаемый email субаккаунта |
| 3 | `portal-frontend/src/pages/network/NetworkTariffsPage.tsx` | `editingPrices` кеширован по `operator_id` → при нескольких категориях на оператора данные перезаписывались | Ключ `${operator_id}_${sender_category}` — каждая комбинация хранится независимо |
| 4 | `portal-frontend/src/pages/network/NetworkTariffsPage.tsx` | `handleSave()` всегда отправлял `sender_category: 'standard'` → перезаписывал non-standard категории | Использует оригинальную категорию из `tariffs[]` при сохранении |
| 5 | `portal-frontend/src/pages/network/NetworkTariffsPage.tsx` | `<input>` в строке таблицы привязан к `editingPrices[t.operator_id]` | Привязан к `editingPrices[${t.operator_id}_${t.sender_category}]` |
| 6 | `portal-frontend/src/pages/network/NetworkTariffsPage.tsx` | `handleBulkPrice` использовал сырой `fetch('/portal/v1/references/operators')` без проверки статуса | Заменён на `apiFetch('/references/operators')` с правильной обработкой ошибок |
| 7 | `portal-frontend/src/pages/network/NetworkRoutingPage.tsx` | Модал "Массовое назначение" — поле ID провайдера: сырой UUID-ввод без подсказок | Загружается список уникальных провайдеров из `listNetworkProviders()`, отображается `<select>` с именами провайдеров; fallback на текстовый ввод если список пуст |

### Инфраструктура (проверка)

| Компонент | Статус |
|---|---|
| `GET /reseller/moderation/counts` | ✅ `GetModerationCounts` — проверяет `is_reseller` |
| `GET /reseller/sender-names` | ✅ `ListResellerSenderNames` — фильтр по `parent_client_id` |
| `POST /reseller/sender-names/{id}/approve|reject` | ✅ gRPC → SenderNameService |
| `GET /reseller/templates` | ✅ `ListResellerTemplates` — фильтр по `parent_client_id` |
| `POST /reseller/templates/{id}/approve|reject|request-revision` | ✅ JSON полей совпадает: `reason`/`comment` |
| `GET /reseller/operator-registrations` | ✅ `ListResellerOperatorRegistrations` — исправлено в этом раунде (+`sub_account_email`) |
| `POST /reseller/operator-registrations/{id}/approve|reject|request-revision` | ✅ JSON поля `note` совпадает |
| `GET /reseller/dashboard` | ✅ параллельный fetch billing+analytics+moderation |
| `GET /reseller/routing/providers` | ✅ query `client_providers JOIN clients WHERE parent_client_id = $1` |
| `GET /reseller/routing/routes` | ✅ query `client_routes JOIN clients WHERE parent_client_id = $1` |
| `POST /reseller/routing/bulk-assign` | ✅ gRPC `AssignProviderToClient` с ownership-check |
| `GET /reseller/tariffs` | ✅ query `aggregator_tariffs WHERE aggregator_id = $1` |
| `PUT /reseller/tariffs` | ✅ UPSERT с unique index |
| `POST /reseller/tariffs/copy` | ✅ ownership-check обоих субаккаунтов |
| `GET /reseller/analytics` | ✅ параллельный gRPC GetStatistics по субаккаунтам |
| `aggregator_tariffs` table | ✅ migration 000096 (unique index по aggregator+sub+operator+category) |
| `operator_registrations` + `operator_registration_history` | ✅ migration 000094 (comment column присутствует) |
| `client_providers` + `client_routes` | ✅ migrations 000037, 000038 |
| Auth на всех reseller handlers | ✅ каждый handler вызывает `checkReseller()` → `is_reseller = true` |

## [DONE] Модуль: Повторяющиеся рассылки (client, /campaign-schedules, fix mode + инфраструктура + QA full, 2026-04-15)

## [DONE] Модуль: Сообщения и рассылки — роль аггрегатор (fix mode + инфраструктура, 2026-04-15)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `internal/gateway/portal/handlers/detalization.go` | `GetMessage` доступ только по `m.client_id = $2` — аггрегатор получал 404 на детали сообщений суб-аккаунтов | `LEFT JOIN clients cli … AND (m.client_id = $2 OR cli.parent_client_id = $2)` — аггрегатор видит свои и суб-аккаунтные сообщения |
| 2 | `portal-frontend/src/pages/messages/MessagesPage.tsx` | Нет контекста о том, что аггрегатор видит агрегированный трафик; фильтр «Логин» без пояснения | Баннер «Отображаются сообщения вашего аккаунта и всех суб-аккаунтов» для `is_reseller=true`; лейбл фильтра меняется на «Суб-аккаунт» |
| 3 | `portal-frontend/src/pages/campaigns/CampaignsPage.tsx` | Нет пояснения, что рассылки суб-аккаунтов недоступны на этой странице | Баннер «Отображаются только ваши рассылки. Рассылки суб-аккаунтов доступны в разделе Суб-аккаунты» для `is_reseller=true` |
| 4 | `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx` | `new Date(msg.created_at).toLocaleString()` — без локали, формат зависит от браузера | `toLocaleString('ru-RU')` — консистентный русский формат даты |

### Инфраструктура (проверка)

| Компонент | Статус |
|---|---|
| `GET /detalization` — аггрегатор видит трафик суб-аккаунтов | ✅ `OR cli.parent_client_id = $1::uuid` в ListMessages |
| `GET /detalization/{id}` — аггрегатор открывает детали | ✅ исправлено в этом раунде |
| `GET /sub-accounts/{id}/messages` | ✅ backend проверяет ownership перед запросом |
| `GET /campaigns` | ✅ изолировано по `client_id` (аггрегатор видит только свои) |
| Нет `/sub-accounts/{id}/campaigns` endpoint | ❌ остаточная проблема — см. ниже |

### Остаточные проблемы

| Приоритет | Проблема | Комментарий |
|---|---|---|
| HIGH | Нет endpoint `/sub-accounts/{id}/campaigns` | CampaignsTab в SubAccountDetailPage остаётся placeholder. Требует добавления gRPC ListCampaigns с фильтром по `client_id` суб-аккаунта |

## [DONE] Модуль: Рассылки — роли аггрегатор + суб-аккаунт (fix mode + инфраструктура, 2026-04-15)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `internal/gateway/portal/handlers/profile.go` | `GetProfile` не возвращал `is_reseller`, `parent_client_id`, `max_sub_accounts` | Добавлены три поля из `clientResp.Client` |
| 2 | `portal-frontend/src/api/client.ts` | `ProfileData` не имел `is_reseller`, `parent_client_id`, `max_sub_accounts` | Добавлены в интерфейс |
| 3 | `portal-frontend/src/components/layout/UserLayout.tsx` | "Суб-аккаунты" показывались всем клиентам, включая суб-аккаунты | `buildNavGroups(is_reseller)` — пункт показывается только реселлерам |
| 4 | `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx` | Баланс `1234.56 ₽` без локализации | `toLocaleString('ru-RU')` → `1 234,56 ₽` |
| 5 | `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx` | Нет таба "Кампании" в детальной странице суб-аккаунта | Добавлен таб с информационным placeholder |
| 6 | `portal-frontend/src/pages/campaigns/CampaignsPage.tsx` | Суб-аккаунт не знает, что работает в ограниченном контексте | Информационный баннер "Вы работаете в режиме суб-аккаунта" |

### Остаточные проблемы (требуют backend/доработки API)

| Приоритет | Проблема | Комментарий |
|---|---|---|
| HIGH | Агрегатор не видит реальные кампании суб-аккаунта | Нет endpoint `/sub-accounts/{id}/campaigns` на бэкенде — нужна доработка campaign service |
| MED | `SubAccountsListPage` показывается суб-аккаунтам как пустая страница | Маршрут `/sub-accounts` не защищён по роли — суб-аккаунт видит пустой список |

## [DONE] Модуль: Рассылки (aggregator + sub-account, fix mode + инфраструктура, 2026-04-15)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx` | Нет sender names → общая ошибка «Заполните текст сообщения и выберите имя отправителя» | Контекстные сообщения + подсказка «У вас нет одобренных имён отправителей. Зарегистрировать →» |
| 2 | `portal-frontend/src/pages/campaigns/CampaignDetailPage.tsx` | `total_cost.toFixed(2)` → "1.50" без локализации | `toLocaleString('ru-RU')` → "1,50 RUB" |
| 3 | `portal-frontend/src/pages/campaigns/CampaignDetailPage.tsx` | `delivery_rate: 0.0%` при delivered=1 (бэкенд возвращает 0) | Fallback: если rate=0 но delivered>0, считаем delivered/total_recipients |
| 4 | `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx` | `handleSaveDraft` отправляет пустой `contact_list_id` → серверная ошибка | Валидация перед отправкой + понятная ошибка |

### Остаточные проблемы

| Приоритет | Проблема | Комментарий |
|---|---|---|
| MED | Агрегатор не видит кампании суб-аккаунтов | SubAccountDetailPage не имеет таба «Кампании». Бэкенд не имеет endpoint `/sub-accounts/{id}/campaigns`. Нужна доработка API + UI |
| MED | Stats inconsistency: sent=0 при delivered=1 | Бэкенд `GetCampaignStats` возвращает `sent=0` для завершённых кампаний, хотя `delivered=1`. Нужна проверка SQL-запроса в campaign service |
| LOW | QuickSend sender names — уже обработано | QuickSendPage корректно показывает «Нет одобренных имён» + disabled кнопку |

### Инфраструктура

| Компонент | Статус |
|---|---|
| Все campaign API endpoints (CRUD, launch/pause/resume/cancel, A/B, retry, stats, timeline, heatmap, report) | ✅ Маршруты совпадают frontend ↔ backend |
| `/campaigns/estimate-cost` | ✅ CostEstimateHandlers.Estimate зарегистрирован |
| Campaign schedules | ✅ CampaignScheduleHandlers с client_id изоляцией |
| Proto-контракты `campaignv1` | ✅ Все gRPC-вызовы соответствуют proto |
| company_id в campaigns | N/A — кампании привязаны к client_id, не к company_id |
| Middleware: session_auth | ✅ Все handlers проверяют GetClientID |
| Sub-account campaigns endpoint | ❌ Отсутствует — нет `/sub-accounts/{id}/campaigns` |

---

## [DONE] Аудит: все клиентские модули (client, fix mode + инфраструктура)

Дата: 2026-04-15. Режим: fix + browser testing. Проверка инфраструктуры: yes.

---

## [DONE] Повторный аудит: все клиентские модули (fix mode, 2026-04-15)

Проверено через браузер (Chrome DevTools MCP → http://localhost:3001).

### Исправлено в этом раунде

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `portal-frontend/src/pages/CommandCenter.tsx` | Баланс `95792152.00 RUB` без разделителей | `95 792 152,00 RUB` через `toLocaleString('ru-RU')` |
| 2 | `portal-frontend/src/pages/messages/MessagesPage.tsx` | Начальная загрузка без дат → запрос всех сообщений → бэкенд timeout | Дефолтный диапазон 7 дней (`date_from`/`date_to`) |
| 3 | `portal-frontend/src/api/client.ts` | `fetch()` без таймаута → бесконечное ожидание | 30-секундный AbortController, сообщение "Превышено время ожидания" |
| 4 | `portal-frontend/src/pages/billing/BillingPage.tsx` | Суммы `1.500000 RUB`, `95792152.000000 RUB` | `1,50 RUB`, `95 792 152,00 RUB` через `fmtMoney()` |
| 5 | `internal/gateway/portal/handlers/profile.go` | Email пустой — `clientResp.Client.Email` перезаписывал email из сессии пустой строкой | Guard: записывать email только если `!= ""` |

### Проверенные модули (все OK)

| Модуль | URL | Статус |
|---|---|---|
| Command Center | `/command-center` | OK (баланс исправлен) |
| Quick Send | `/quick-send` | OK |
| Messages | `/messages` | OK (дефолтные даты, таймаут) |
| Campaigns | `/campaigns` | OK (список + детали) |
| Campaign Detail | `/campaigns/:id` | OK (breadcrumb, статистика, A/B) |
| Billing | `/billing` | OK (суммы исправлены) |
| Analytics | `/analytics` | OK (графики, таблица) |
| Contacts | `/contact-lists` | OK |
| Templates | `/templates` | OK |
| Sender Names | `/sender-names` | OK |
| Companies | `/companies` | OK |
| Sub-accounts | `/sub-accounts` | OK (корректно: "недоступны" для не-реселлера) |
| API Keys | `/api-keys` | OK |
| Webhooks | `/webhooks` | OK |
| Profile | `/profile` | OK (email fix в бэкенде) |
| Providers | `/providers` | OK (empty state) |
| Routing | `/routing` | OK (маршруты загрузились) |
| Lookup | `/lookup` | OK |
| Tariffs | `/tariffs` | OK (текущий план + доступные) |
| Audit Log | `/audit-log` | OK |
| Notification Settings | `/settings/notifications` | OK |

### Остаточные проблемы (серверные, не фронтенд)

| Приоритет | Проблема | Комментарий |
|---|---|---|
| HIGH | `/portal/v1/detalization` timeout >30s | Бэкенд не отвечает даже с датами 7 дней. Нужна оптимизация запроса на сервере (индексы, партиции) |
| MED | WebSocket live feed `connecting` | WS не подключается через Vite proxy к продакшн-серверу (ожидаемо при локальной разработке) |
| LOW | `/portal/v1/profile` возвращает `email:""` | Исправлено в коде, но требует деплой |

---

## [DONE] Модуль: Финансы — роль агрегатор + суб-аккаунты (fix mode + инфраструктура, 2026-04-15)

### Цель пользователя
Агрегатор хочет управлять финансами: видеть свой баланс, пополнять, просматривать транзакции (включая переводы суб-аккаунтам), переводить средства суб-аккаунтам и следить за их расходами. Суб-аккаунт хочет видеть свой баланс и историю транзакций.

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `portal-frontend/src/pages/billing/BillingPage.tsx` | `toLocaleString()` без локали — формат даты зависит от браузера (2 места: дата транзакции, дата обновления баланса) | `toLocaleString('ru-RU')` — консистентный формат |
| 2 | `portal-frontend/src/pages/billing/BillingPage.tsx` | Тип транзакции `transfer` (перевод суб-аккаунту) отсутствует в TYPE_OPTIONS, typeLabel, typeBadgeVariant — транзакция показывается как raw "transfer" без локализации и бейджа | Добавлен тип `transfer` → «Перевод» с нейтральным бейджем |
| 3 | `portal-frontend/src/pages/billing/BillingPage.tsx` | Агрегатор (is_reseller=true) не понимает, что видит только свой баланс, нет контекста про суб-аккаунты | Информационный баннер со ссылкой на /sub-accounts |
| 4 | `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx` | Форма перевода: нет указания валюты, нет информации о текущем балансе суб-аккаунта | Показан баланс суб-аккаунта, label «Сумма (₽)» |
| 5 | `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx` | `total_cost` в AnalyticsTab суб-аккаунта без форматирования (`1.500000 RUB`) | `toLocaleString('ru-RU')` — `1,50 RUB` |

### Инфраструктура (проверка)

| Компонент | Статус |
|---|---|
| `GET /billing/balance` — баланс клиента | ✅ handler → gRPC GetBalance + ListBalances (для threshold) |
| `GET /billing/transactions` — транзакции с фильтрацией | ✅ handler → gRPC GetTransactionHistory с date_from/date_to/type |
| `POST /billing/top-up` — пополнение | ✅ handler → PaymentProvider.CreatePayment |
| `GET/POST /billing/top-up/callback` — callback платежа (public) | ✅ handler → PaymentProvider.HandleCallback → AddCredits |
| `PUT /billing/low-balance-threshold` — порог уведомлений | ✅ handler → gRPC SetLowBalanceThreshold |
| `POST /sub-accounts/{id}/transfer` — перевод средств | ✅ handler → проверка ownership → gRPC TransferBalance + audit event |
| `DELETE /sub-accounts/{id}` — возврат остатка при удалении | ✅ автоматический TransferBalance обратно родителю |
| `GET /tariffs/current` / `GET /tariffs/plans` / `POST /tariffs/change` | ✅ все маршруты зарегистрированы |
| Proto `billingv1`: GetBalance, AddCredits, TransferBalance, SetLowBalanceThreshold, ListBalances | ✅ все RPC соответствуют handler-вызовам |
| Таблицы `accounts`, `transactions`, `balance_transfers` | ✅ миграции 000006, 000020, 000053 |
| `GET /sub-accounts/{id}/transactions` — история транзакций суб-аккаунта | ✅ исправлено — добавлен handler + маршрут |

### Доисправлено (2026-04-15)

| # | Файл | Было → Стало |
|---|---|---|
| 6 | `internal/gateway/portal/handlers/sub_accounts.go` + `router.go` | Нет `/sub-accounts/{id}/transactions` → добавлен `GetSubAccountTransactions` с ownership-check, date/type фильтрацией |
| 7 | `portal-frontend/src/api/client.ts` | Нет `subAccountsApi.transactions()` → добавлен |
| 8 | `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx` | Нет вкладки «Транзакции» → добавлена (тип/сумма/баланс-после/дата, форматирование ru-RU) |
| 9 | `portal-frontend/src/pages/CommandCenter.tsx` | CommandCenter не показывал данные суб-аккаунтов → 3 новых KPI-карточки для реселлера: активные, суммарный баланс, низкий баланс |

### Остаточные проблемы

| Приоритет | Проблема | Комментарий |
|---|---|---|
| LOW | Нет обратного перевода (суб-аккаунт → агрегатор) в UI | Форма перевода только в одну сторону. Возврат средств — только автоматически при удалении суб-аккаунта |

---

## [DONE] Модуль: Индивидуальные тарифы (admin, /admin/individual-tariffs, fix mode + инфраструктура, 2026-04-15)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `internal/gateway/admin/handlers/hierarchical_periods.go` + `router.go` | Тиры создавались/читались через `POST/GET /tariff-tiers` → gRPC → `tariff_tiers` (старая таблица), а периоды лежат в `tariff_periods_new` → FK-нарушение, тиры никогда не находились | Новые handlers `ListPeriodTiers`, `CreatePeriodTier`, `UpdatePeriodTier`, `DeletePeriodTier` напрямую работают с `tariff_tiers_new`; маршруты `/periods/{id}/tiers` и `/periods/{id}/tiers/{tier_id}` |
| 2 | `portal-frontend/src/pages/admin/tarification/IndividualTariffsPage.tsx` | `fetchTiers()` вызывал `tarificationApi.listTariffTiers` → старая таблица | Использует `tarificationApi.listPeriodTiers(periodId)` → новые endpoints |
| 3 | `portal-frontend/src/pages/admin/tarification/IndividualTariffsPage.tsx` | `handleSaveTier()` вызывал `createTariffTier`/`updateTariffTier` → gRPC → `tariff_tiers` | Использует `createPeriodTier`/`updatePeriodTier` → `tariff_tiers_new` |
| 4 | `portal-frontend/src/pages/admin/tarification/IndividualTariffsPage.tsx` | `auto_close_warning` из ответа при создании периода полностью игнорировался | `toast.info()` показывает сообщение об автоматически закрытом периоде |
| 5 | `portal-frontend/src/pages/admin/tarification/IndividualTariffsPage.tsx` | `catch` показывал «Не удалось создать период» вне зависимости от ошибки | `apiErrorMessage(err, fallback)` — показывает `AdminApiError.message` (напр. «no active period at parent level») |
| 6 | `portal-frontend/src/pages/admin/tarification/IndividualTariffsPage.tsx` | `STRATEGY_OPTIONS` — raw English: `fixed`, `threshold_recalc` | Русские читаемые подписи: «Фиксированная (fixed)», «Пороговая с пересчётом (threshold_recalc)» |
| 7 | `portal-frontend/src/pages/admin/tarification/IndividualTariffsPage.tsx` | Поле `start_date` без ограничения — можно выбрать прошлое, получить 422 после сабмита | `min={todayISO()}` — браузер блокирует прошлые даты до отправки |
| 8 | `portal-frontend/src/pages/admin/tarification/IndividualTariffsPage.tsx` | Нет кнопки «Удалить» для тира | Кнопка «Удалить» + `ConfirmDialog` + `handleDeleteTier()` через `deletePeriodTier()` |
| 9 | `portal-frontend/src/pages/admin/tarification/IndividualTariffsPage.tsx` | После удаления периода `selectedPeriodId` оставался указывать на удалённый период | Сброс `selectedPeriodId` и `tiers` если удалён выбранный период |
| 10 | `portal-frontend/src/api/admin.ts` | `AutoCloseWarning` имел `closed_period_id`, `old_end_date`, `message` — не совпадало с бэкендом | Исправлено на `period_id`, `new_end_date` (соответствует JSON из Go handler) |
| 11 | `portal-frontend/src/pages/admin/tarification/PeriodsTab.tsx` | `res.auto_close_warning.message` — несуществующее поле | Использует `new_end_date` |

### Инфраструктура (проверка)

| Компонент | Статус |
|---|---|
| `POST /tarification/periods` → `tariff_periods_new` | ✅ `HierarchicalPeriodsHandler.CreatePeriod` |
| `GET /tarification/periods` → фильтр по `client_id` | ✅ `HierarchicalPeriodsHandler.ListPeriods` |
| `PUT /tarification/periods/{id}` | ✅ стратегия + end_date |
| `DELETE /tarification/periods/{id}` → проверка child periods | ✅ |
| `GET /tarification/periods/{id}/tiers` → `tariff_tiers_new` | ✅ добавлено в этом раунде |
| `POST /tarification/periods/{id}/tiers` → `tariff_tiers_new` | ✅ добавлено в этом раунде |
| `PUT /tarification/periods/{id}/tiers/{tier_id}` | ✅ добавлено в этом раунде |
| `DELETE /tarification/periods/{id}/tiers/{tier_id}` | ✅ добавлено в этом раунде |
| `tariff_tiers_new`: UNIQUE(tariff_period_id, from_count), FK ON DELETE CASCADE | ✅ миграция 000081 |
| Admin auth middleware на всех новых маршрутах | ✅ все маршруты внутри `tarification` subrouter |



## [DONE] Модуль: Отправить / Быстрая отправка — Повторный аудит (client, /quick-send, fix mode + инфраструктура + QA full, 2026-04-16)

### Итог (8 TC + BVA, 5 PASS, 2 PARTIAL, 1 FAIL)

| TC | Тип | Описание | Вердикт |
|---|---|---|---|
| TC-1 | happy | Отправить 1 сообщение с валидными полями | FAIL — POST 201 ✅, но DB persist ❌ (Kafka pipeline broken: scheduler ошибка) |
| TC-2 | edge | CharacterCounter BVA (0/140/159/160/161/306/307/320/765) | PARTIAL — логика сегментов правильная ✅; "лишних" → "сверх" исправлено и задеплоено |
| TC-3 | negative | Пустая форма / только пробелы / нет номеров | PASS — все ошибки корректны ✅ |
| TC-4 | negative | Некорректные номера / SQL / XSS / русские форматы | PARTIAL — SQL+XSS безопасны ✅; +7(900)123-45-67 отклонялся (parsePhones) → исправлено и задеплоено |
| TC-5 | negative | Отмена ConfirmDialog | PASS — форма сохраняется, сообщение не отправлено ✅ |
| TC-6 | edge | 1000+ получателей | PASS — добавлен MAX_RECIPIENTS=500 + streaming progress ✅ |
| TC-7 | state | Polling: infinite 404 loop | PASS — MAX_POLL_ATTEMPTS=60, isMessageDone(err-) → исправлено и задеплоено ✅ |
| TC-8 | infra | Endpoints / migrations / gRPC contracts | PASS — все маршруты, таблицы, proto совпадают ✅ |
| BVA-A | — | text: 0/140/159/160/161/306/307/765 chars | PASS — логика верна; contacts: regex /^\+?[0-9]{10,15}$/ ✅ |

### Исправлено в этом раунде (задеплоено)

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | `allDone` не проверял `err-` prefix → infinite polling | `isMessageDone` проверяет `err-` prefix |
| 2 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | Нет лимита polling → бесконечные 404 | `MAX_POLL_ATTEMPTS=60` (3 мин), статус → `expired` |
| 3 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | `parsePhones` убирал только пробелы → `+7(900)123-45-67` отклонялся | Убираем `[\s\-().]` — русский формат проходит |
| 4 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | Silent catch при загрузке sender names | `sendersError` state + кнопка «Повторить» |
| 5 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | "1 сообщений" неверная грамматика | `pluralMessages(n)` → "1 сообщение", "2 сообщения" |
| 6 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | `setSentMessages` после всего цикла → нет прогресса | Streaming: `setSentMessages(prev => [...prev, msg])` внутри цикла |
| 7 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | Нет лимита получателей → 10000+ номеров блокируют | `MAX_RECIPIENTS=500` с подсказкой про Кампании |
| 8 | `portal-frontend/src/components/ui/CharacterCounter.tsx` | "40 лишних" → вводит в заблуждение | "+40 сверх · 2 SMS" |
| 9 | `internal/gateway/portal/handlers/messages.go` | `GetMessage` → 404 для in-flight сообщений | При `pgx.ErrNoRows` + gRPC client → fallback на `getMessageViaGRPC` |

### Остаточные проблемы (backend, не фронтенд)

| Приоритет | Проблема | Комментарий |
|---|---|---|
| CRITICAL | Messaging scheduler: `"missing destination name channel in *[]*shared.Message"` | Постоянная ошибка — scheduler не может обработать pending/scheduled сообщения |
| HIGH | Сообщения не сохраняются в БД | Kafka publisher работает (offset зафиксирован), но consumer не персистит → polling всегда 404 |
| MED | REST polling вместо SSE | `/messages/stream` SSE уже существует; переход на SSE устранит 404-флуд полностью |

## [DONE] Модуль: Отправить / Быстрая отправка (client, /quick-send, fix mode + инфраструктура + QA full, 2026-04-16)

### Тест-кейсы (8 TC, 7 PASS, 1 PARTIAL)

| TC | Тип | Описание | Вердикт |
|---|---|---|---|
| TC-1 | happy | Отправить 1 сообщение с валидными полями | PASS (201, статус queued в UI) |
| TC-2 | edge | CharacterCounter при >160 символов | PARTIAL (работает, но "лишних" → исправлено на "+N сверх") |
| TC-3 | negative | Пустая форма — Submit без заполнения | PASS ("Введите текст сообщения") |
| TC-4 | negative | Только пробелы в тексте | PASS (trim() → "Введите текст сообщения") |
| TC-5 | negative | Некорректный формат номера | PASS (ошибка валидации); дополнительно исправлен BUG-3 (скобки/тире) |
| TC-6 | edge | allDone при failed send (err- prefix) | FAIL → FIXED (polling теперь завершается) |
| TC-7 | state | loadingSenders error silent catch | FAIL → FIXED (добавлен error state + кнопка "Повторить") |
| TC-8 | infra | GET /messages/{id} — polling статуса | FAIL → PARTIAL (добавлен max retry limit + gRPC fallback) |

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | `allDone` не учитывал `err-` prefix сообщений → polling никогда не завершался при ошибках отправки | `isMessageDone()` проверяет и terminal statuses, и `err-` prefix |
| 2 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | Нет лимита polling → бесконечные 404 в консоли (89+ ошибок) | `MAX_POLL_ATTEMPTS=60` (3 мин) + `pollAttemptsRef`, после лимита статус → `expired` |
| 3 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | `status: 'failed: ${msg}'` → StatusBadge не распознавал, показывал серым | Нормализован до `status: 'failed'` |
| 4 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | `parsePhones` убирал только пробелы → `+7(900)123-45-67` отклонялся | Убираем `[\s\-().]` — распространённый русский формат проходит |
| 5 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | `catch(() => {})` при загрузке sender names → пользователь видел "Нет имён" вместо ошибки | Добавлен `sendersError` state + "Повторить" кнопка |
| 6 | `portal-frontend/src/pages/quick-send/QuickSendPage.tsx` | "Будет отправлено 1 сообщений" — неверная русская грамматика | `pluralMessages(n)` → "1 сообщение", "2 сообщения", "5 сообщений" |
| 7 | `portal-frontend/src/components/ui/CharacterCounter.tsx` | "40 лишних" → вводит в заблуждение (символы не обрезаются, а в 2-м сегменте) | "+40 сверх · 2 SMS" |
| 8 | `internal/gateway/portal/handlers/messages.go` | `GetMessage` возвращал 404 для in-flight сообщений (async Kafka pipeline) без fallback | При `pgx.ErrNoRows` и наличии `messagingClient` → fallback на `getMessageViaGRPC` |

### Инфраструктура (проверка)

| Компонент | Статус |
|---|---|
| `POST /messages` → `SendMessage` gRPC | ✅ handler корректен, возвращает 201 + message_id |
| `GET /messages/{id}` → `GetMessage` DB+gRPC | ✅ исправлен в этом раунде (gRPC fallback при 404 DB) |
| `GET /sender-names?status=approved` | ✅ handler + pagination |
| `GET /messages/stream` SSE | ✅ handler зарегистрирован (не используется QuickSend, polling вместо SSE) |
| Messaging service: non-scheduled → Kafka async (no DB write at send time) | ⚠️ By design, но вызывает 404 при polling до persist stage |
| Messaging scheduler: `"missing destination name channel"` error | ❌ Постоянная ошибка — возможно блокирует pipeline |

### Остаточные проблемы

| Приоритет | Проблема | Комментарий |
|---|---|---|
| HIGH | Scheduler `"missing destination name channel in *[]*shared.Message"` | Постоянная ошибка в messaging-service logs — вероятно блокирует обработку сообщений через pipeline |
| MED | QuickSendPage использует REST polling вместо SSE | `/messages/stream` уже существует. Переход на SSE устранит 404-флуд полностью |
| LOW | Нет ограничения количества получателей в форме | Можно вставить 10000 номеров — последовательная отправка заблокирует UI надолго |

## [DONE] Модуль: Рассылки (aggregator + sub-account, /campaigns, fix mode + инфраструктура + QA full, 2026-04-16)

### Итог (8 TC + BVA + матрица состояний, 8 PASS, 0 FAIL)

| TC | Тип | Описание | Вердикт |
|---|---|---|---|
| TC-1 | CRITICAL fix | segment_rules отправлялся как object вместо JSON string → 400 при фильтрах | FIXED |
| TC-2 | HIGH fix | Прямой текст в визарде не передавался в API → кампания без контента | FIXED |
| TC-3 | MED fix | CampaignItem.delivered vs delivered_count → колонка показывала «—» | FIXED |
| TC-4 | MED fix | use_subscriber_timezone отображался активным, но не реализован в backend | FIXED |
| TC-5 | LOW fix | created_at без ru-RU локали + STATUS_CONFIG без scheduled/materializing | FIXED |
| TC-6 | LOW fix | Silent catch для senderNames и contactLists в WizardPage | FIXED |
| TC-7 | infra | Все campaign endpoints, миграции, gRPC contracts | PASS |
| BVA | — | Граничные значения: name required, contact_list_id required UUID, template optional | PASS |
| State | — | Матрица статусов: draft→running→paused→completed→cancelled + scheduled + materializing | PASS |

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `CampaignWizardPage.tsx` | `segment_rules: { ... }` (object) → 400 ошибка | `segment_rules: JSON.stringify({ ... })` — корректная строка |
| 2 | `CampaignWizardPage.tsx` | `canProceed` принимал прямой текст без шаблона → кампания без контента | Требует `templateId` (не пустой) + hint "создать шаблон →" |
| 3 | `CampaignWizardPage.tsx` | A/B вариант B: textarea + `abTextB` (никогда не отправлялось) | Только TemplatePicker; `abTextB` state убран |
| 4 | `CampaignWizardPage.tsx` | `use_subscriber_timezone` checkbox активный (нет в backend) | Disabled + "Функционал в разработке" |
| 5 | `CampaignWizardPage.tsx` | Silent `.catch(() => {})` для senderNames + contactLists | `sendersError`/`contactListsError` state + "Повторить" |
| 6 | `api/client.ts` | `subAccountsApi.campaigns` тип: `delivered: number` | `delivered_count: number` — соответствует полю proto |
| 7 | `SubAccountDetailPage.tsx` | `CampaignItem.delivered`, колонка `key: 'delivered'` | `delivered_count`; дата `toLocaleDateString('ru-RU')` |
| 8 | `SubAccountDetailPage.tsx` | `CAMPAIGN_STATUS_CONFIG` не содержал `scheduled`, `materializing` | Добавлены оба статуса |
| 9 | `CampaignsPage.tsx` | `STATUS_CONFIG` не содержал `scheduled`, `materializing` | Добавлены оба статуса |
| 10 | `CampaignsPage.tsx` | Фильтр статусов не содержал `scheduled`, `materializing` | Добавлены в `<select>` |

### Инфраструктура (проверка)

| Компонент | Статус |
|---|---|
| Все 19 campaign endpoints (CRUD + action + stats + timeline + A/B + report) | ✅ frontend ↔ backend совпадают |
| `GET /sub-accounts/{id}/campaigns` → `GetSubAccountCampaigns` | ✅ ownership check + ListCampaigns |
| `POST /campaigns/estimate-cost` | ✅ отдельный subrouter с session auth |
| `campaigns` table + partitioned `campaign_recipients` | ✅ migration 000043 |
| `campaign_ab_config.metric` CHECK (delivery_rate, click_rate, unique_click_rate) | ✅ migration 000051 расширил constraint |
| `use_subscriber_timezone` колонка в DB | ✅ migration 000090, но gRPC/service не реализован |
| gRPC proto `campaignv1`: все 17 RPC соответствуют handler-вызовам | ✅ |
| Middleware: session_auth + CSRF на всех endpoints | ✅ |

### Остаточные проблемы

| Приоритет | Проблема | Комментарий |
|---|---|---|
| MED | `use_subscriber_timezone` в DB но не в gRPC proto и service | DB column есть, но feature не работает. Нужен proto field + service logic |
| LOW | Stats inconsistency: `sent=0` при `delivered>0` | Известная проблема из предыдущего аудита — нужна проверка SQL в campaign service |

## [DONE] Модуль: Шаблоны (aggregator + sub-account, /templates, fix mode + инфраструктура + QA full, 2026-04-16)

### Итог (6 TC + BVA + матрица состояний, 6 PASS, 0 FAIL)

| TC | Тип | Описание | Вердикт |
|---|---|---|---|
| TC-1 | LOW fix | `created_at` без `ru-RU` локали в таблице | FIXED |
| TC-2 | MED fix | Traffic type labels английские в таблице и форме | FIXED |
| TC-3 | MED fix | Sender names dropdown скрыт, нет подсказки при пустом списке | FIXED |
| TC-4 | MED fix | Silent catch при загрузке sender names — нет ошибки + retry | FIXED |
| TC-5 | MED fix | Submit for review: нет loading state, риск двойного клика | FIXED |
| TC-6 | infra | Все Template endpoints, миграции, gRPC contracts | PASS |

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `portal-frontend/src/pages/templates/TemplatesPage.tsx` | `toLocaleDateString()` без локали | `toLocaleDateString('ru-RU')` |
| 2 | `portal-frontend/src/pages/templates/TemplatesPage.tsx` | Labels: `Transactional`, `Authorization`, `Service` (EN) — в таблице и форме | `Транзакционный`, `Авторизационный`, `Сервисный` (RU) |
| 3 | `portal-frontend/src/pages/templates/TemplatesPage.tsx` | Dropdown sender names скрыт когда `length === 0` — пользователь не понимает почему поля нет | Подсказка «Нет одобренных имён отправителей. Зарегистрировать →» |
| 4 | `portal-frontend/src/pages/templates/TemplatesPage.tsx` | `.catch(() => {})` при загрузке sender names — silent failure | `sendersError` state + кнопка «Повторить» |
| 5 | `portal-frontend/src/pages/templates/TemplatesPage.tsx` | `handleSubmitForReview` без loading state — двойной клик отправлял дважды | `submittingId` state, кнопка disabled + текст «Отправка...» |

### Инфраструктура (проверка)

| Компонент | Статус |
|---|---|
| `POST /templates` → `CreateTemplate` | ✅ |
| `GET /templates` → `ListTemplates` | ✅ |
| `GET /templates/{id}` → `GetTemplate` | ✅ |
| `PUT /templates/{id}` → `UpdateTemplate` | ✅ |
| `DELETE /templates/{id}` → `DeleteTemplate` | ✅ |
| `POST /templates/{id}/render` → `RenderTemplate` | ✅ |
| `GET /templates/{id}/audit` → `GetTemplateAuditLog` | ✅ |
| `POST /templates/{id}/submit` → `SubmitForReview` | ✅ |
| `GET /reseller/templates` → `ListResellerTemplates` | ✅ |
| `POST /reseller/templates/{id}/approve` | ✅ |
| `POST /reseller/templates/{id}/reject` | ✅ |
| `POST /reseller/templates/{id}/request-revision` | ✅ |
| `templates` table: 14 колонок, все миграции применены | ✅ |
| FK: `client_id → clients`, `reviewer_id → users`, `sender_name_id → sender_names` | ✅ |
| status CHECK: `draft,pending,review,revision_requested,approved,rejected` | ✅ |
| gRPC proto `templatev1`: все 12 RPC совпадают с handlers | ✅ |

## [DONE] Модуль: Компании (aggregator + sub-account, /companies, fix mode + инфраструктура + QA full, 2026-04-16)

### Итог (6 TC, 6 исправлений, 0 блокеров)

| TC | Тип | Описание | Вердикт |
|---|---|---|---|
| TC-1 | CRITICAL fix | GetCompany без ownership check — чужая компания по UUID | FIXED |
| TC-2 | CRITICAL fix | UpdateCompany без ownership check — изменение чужой компании | FIXED |
| TC-3 | HIGH fix | CompanyDetailPage.handleSave — нет валидации ИНН перед PUT | FIXED |
| TC-4 | MED fix | Offer-компании (is_offer=true) отображались как редактируемая форма | FIXED |
| TC-5 | MED fix | Нет empty state в CompaniesPage при отсутствии компаний | FIXED |
| TC-6 | MED fix | success message не исчезал автоматически + нет кнопки Detach в UI | FIXED |

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `internal/gateway/portal/handlers/companies.go` | `GetCompany` не проверял принадлежность компании клиенту — любой пользователь мог читать чужие компании по UUID | Добавлен ownership check через `ListClientCompanies`; если компания не в списке → 404 |
| 2 | `internal/gateway/portal/handlers/companies.go` | `UpdateCompany` не проверял принадлежность — любой пользователь мог изменить чужую компанию | Добавлен ownership check через `ListClientCompanies` перед вызовом `UpdateCompany` |
| 3 | `portal-frontend/src/pages/companies/CompanyDetailPage.tsx` | `handleSave()` не валидировал ИНН перед отправкой → бэкенд возвращал «invalid INN checksum» без контекста | Добавлена `validateINN()` + проверка `name.trim()` перед PUT |
| 4 | `portal-frontend/src/pages/companies/CompanyDetailPage.tsx` | Offer-компании (`is_offer=true`) показывали редактируемую форму со всеми полями | `isOffer` → информационный баннер + кнопка «Назад»/«Отвязать» без формы |
| 5 | `portal-frontend/src/pages/companies/CompaniesPage.tsx` | Пустой список без сообщения → DataTable с нулями, непонятно что делать | Empty state с пояснением и кнопкой «Добавить первую компанию» |
| 6 | `portal-frontend/src/pages/companies/CompanyDetailPage.tsx` | `success` state не очищался + нет кнопки «Отвязать компанию» в UI | `setTimeout 4000ms` для auto-clear; кнопка «Отвязать» (скрыта если is_default=true) |

### Инфраструктура (проверка)

| Компонент | Статус |
|---|---|
| `GET /companies` → `ListCompanies` → gRPC `ListClientCompanies` | ✅ фильтр по client_id |
| `POST /companies` → `CreateCompany` → gRPC `CreateCompany` | ✅ с AttachCompany + CreateForCompany |
| `GET /companies/{id}` → `GetCompany` | ✅ исправлено — ownership check добавлен |
| `PUT /companies/{id}` → `UpdateCompany` | ✅ исправлено — ownership check добавлен |
| `POST /companies/{id}/set-default` → `SetDefaultCompany` | ✅ unique index обеспечивает constraint |
| `DELETE /companies/{id}/detach` → `DetachCompany` | ✅ проверяет `ErrCannotDetachDefault` + `ErrCompanyHasSenderNames` |
| `companies` + `client_companies` tables | ✅ migration 000091 |
| Unique index `is_default` per client | ✅ `idx_client_companies_default WHERE is_default = TRUE` |
| gRPC proto `companyv1`: все 7 RPC совпадают | ✅ |
| Auth middleware: все маршруты в protected subrouter | ✅ |

### Остаточные проблемы

| Приоритет | Проблема | Комментарий |
|---|---|---|
| LOW | Frontend validateINN проверяет только длину (не контрольную сумму) | Бэкенд возвращает читаемую ошибку «invalid INN checksum». Полная реализация checksum на фронте — nice-to-have |
| LOW | Агрегатор не видит компании суб-аккаунтов | По дизайну каждый клиент видит только свои компании. Для агрегатора нет отдельного интерфейса компаний суб-аккаунтов |

## [DONE] Модуль: Рассылки (aggregator + sub-account, /campaigns, fix mode + QA full, 2026-04-16 повторный)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `CampaignDetailPage.tsx` | `stats?.sent ?? campaign.sent_count` → `??` не падает на 0, показывался `0` при `sent_count > 0` | `stats?.sent \|\| campaign.sent_count` → корректный fallback на 0 |
| 2 | `CampaignWizardPage.tsx` | `estimated_cost` и `current_balance` → raw строки `"95752141.500000"` без форматирования | `parseFloat(...).toLocaleString('ru-RU', {minimumFractionDigits:2, maximumFractionDigits:2})` → `95 752 141,50 ₽` |
| 3 | `CampaignWizardPage.tsx` | Хардкод `"контактов"` для любого числа (`"1 контактов"`) | `pluralContacts(n)` → `"1 контакт"`, `"2 контакта"`, `"5 контактов"` |
| 4 | `CampaignsPage.tsx` | Пустое состояние без контекста (показывалось `"Рассылки не созданы"` даже при активном фильтре) | Условный рендер: при активном `statusFilter` → `"Рассылок с этим статусом нет"` + кнопка «Сбросить фильтр» |

---

## [DONE] Модуль: Тарифы субаккаунтов (aggregator, /network/tariffs, fix mode + инфраструктура + QA full, 2026-04-16)

### Итог (5 исправлений, все задеплоены и проверены в браузере)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `internal/gateway/portal/handlers/reseller_tariffs.go` | `ListTariffs` SQL при `sub_account_id` возвращал дубликаты: и глобальные (NULL), и sub-account-specific строки для одного оператора+категории → 17 строк вместо 13 | `DISTINCT ON (operator_id, sender_category)` с приоритетом sub-account (`NULLS LAST`) → 13 уникальных строк |
| 2 | `portal-frontend/src/pages/network/NetworkTariffsPage.tsx` | Цена `3.200000` (6 знаков из `NUMERIC(10,6)`) | `formatPrice()` → `3.20` (2 знака) |
| 3 | `portal-frontend/src/pages/network/NetworkTariffsPage.tsx` | Категории на английском: `paid_registered`, `free_registered`, `shared` | `CATEGORY_LABELS` → «Платная регистрация», «Бесплатная регистрация», «Общая», «Стандартная» |
| 4 | `portal-frontend/src/pages/network/NetworkTariffsPage.tsx` | `.catch(() => {})` при загрузке субаккаунтов — silent failure, пустой dropdown без объяснения | `subAccountsError` state + баннер с кнопкой «Повторить» |
| 5 | `portal-frontend/src/pages/network/NetworkTariffsPage.tsx` | `handleSave()` и `handleBulkPrice()` принимали любую строку включая отрицательные и нечисловые значения | Валидация `parseFloat` + проверка `n >= 0` перед отправкой |

### Инфраструктура (проверка)

| Компонент | Статус |
|---|---|
| `GET /reseller/tariffs?sub_account_id=...` → `ListTariffs` | ✅ исправлен DISTINCT ON, ownership check через `checkReseller()` |
| `PUT /reseller/tariffs` → `UpsertTariffs` | ✅ UPSERT с unique index, ownership check субаккаунта |
| `POST /reseller/tariffs/copy` → `CopyTariffs` | ✅ ownership check обоих субаккаунтов |
| `aggregator_tariffs` table | ✅ migration 000096 (unique index по aggregator+sub+operator+category) |
| Auth middleware | ✅ все handlers вызывают `checkReseller()` → `is_reseller = true` |

---

## [DONE] Модуль: Сетевая статистика (aggregator, /network/statistics, fix mode + инфраструктура + QA full, 2026-04-17)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `internal/services/network_analytics/domain/models.go` | `Normalize()` не разрешал `period_preset` в `DateFrom`/`DateTo` → период-фильтры (`Сегодня`, `7 дней`, `30 дней` и т.д.) полностью не работали — бэкенд возвращал ВСЕ данные без фильтрации по дате | `Normalize()` разрешает все пресеты (`today`, `yesterday`, `7d`, `30d`, `month`, `prev_month`, `year`, `15m`, `60m`, `24h`) в конкретные `DateFrom`/`DateTo`; fallback на 7 дней если даты не указаны |
| 2 | `internal/gateway/portal/router/router.go` | `/export/{id}/status` и `/export/{id}/download` — path param `{id}`, но handler читает `mux.Vars(r)["job_id"]` → экспорт статуса ВСЕГДА возвращал ошибку `"job_id обязателен"` | Исправлено на `/export/{job_id}/status` и `/export/{job_id}/download` |
| 3 | `portal-frontend/src/api/networkStats.ts` | `saveView()` отправлял `body: JSON.stringify(view)` (flat), но бэкенд ожидал `{ "view": {...} }` (wrapped) → сохранение видов ВСЕГДА возвращало `"Поле view обязательно"` | `body: JSON.stringify({ view })` |
| 4 | `internal/gateway/portal/handlers/network_statistics.go` | `GetMonitoring` не вызывал `checkClient()` → при отсутствии gRPC-клиента — nil dereference / паника | Добавлен `h.checkClient(w)` — возвращает graceful 503 с пустыми данными |
| 5 | `portal-frontend/src/components/network-stats/DrillDownDrawer.tsx` | `fmt(n)` и `fmtPct(n)` без null-safety → крэш при null/undefined из API | `(n ?? 0)` — null-safe |
| 6 | `portal-frontend/src/components/network-stats/MonitoringKPIGrid.tsx` | `kpi.value / 1000`, `kpi.value.toFixed(0)`, `kpi.name.toLowerCase()` без null-safety → крэш при null | `const v = kpi.value ?? 0`, `(kpi.name ?? '').toLowerCase()` |
| 7 | `portal-frontend/src/components/network-stats/MonitoringKPIGrid.tsx` | KPI-карточки мониторинга показывали английские имена: `throughput`, `dlr_rate`, `errors`, `timeouts`, `unhealthy_providers` | Локализовано: «Пропускная способность», «Доставляемость», «Ошибки», «Таймауты», «Проблемные провайдеры»; `dlr_rate` форматируется как процент |
| 8 | `portal-frontend/src/hooks/useNetworkStats.ts` | Переключение вкладки (Статистика→Аналитика→Мониторинг) не загружало данные — пользователь видел stale данные и должен был нажать «Применить» | `useEffect` по `mode` запускает `fetchData()` автоматически при смене вкладки |
| 9 | `portal-frontend/src/hooks/useNetworkStats.ts` | `loadViews` — `.catch { /* silent */ }` → при ошибке загрузки видов пользователь не получал обратной связи | `viewsError` state + expose через return для отображения ошибки |
| 10 | `portal-frontend/src/api/networkStats.ts` | `date_from`/`date_to` отправлялись как ISO-строки, но бэкенд `parseSharedFilter` ожидал Unix timestamp (int64) → даты парсились как 0 | `filterToParams()` конвертирует ISO-строки в Unix timestamp (секунды) перед отправкой |

### Инфраструктура (провер��а)

| Компонент | Статус |
|---|---|
| `GET /reseller/statistics` → `GetStatistics` gRPC | ✅ |
| `GET /reseller/analytics-summary` ��� `GetAnalyticsSummary` gRPC | ✅ |
| `GET /reseller/monitoring` → `GetMonitoringMetrics` gRPC | ✅ исправлено (checkClient) |
| `GET /reseller/drilldown` → `GetDrillDown` gRPC | ✅ |
| `POST /reseller/export` → `StartExport` gRPC | ✅ |
| `GET /reseller/export/{job_id}/status` → `GetExportStatus` gRPC | ✅ исправлено (path param) |
| `GET /reseller/views` → `ListSavedViews` gRPC | ✅ |
| `POST /reseller/views` → `SaveView` gRPC | ✅ исправлено (body format) |
| `DELETE /reseller/views/{id}` → `DeleteView` gRPC | ✅ |
| `network_stats_hourly` table (partitioned, migration 000102) | ��� |
| `network_monitoring_snapshot` table (migration 000102) | ✅ |
| `saved_views` table + UNIQUE(partner_id, user_id, name) (migration 000102) | ✅ |
| `export_jobs` table + UUID PK (migration 000102) | ✅ |
| Proto `networkanalyticsv1`: все 9 RPC соответствуют handler-вызовам | ✅ |
| Auth middleware: все handlers проверяют `GetClientID` | ✅ |
| Seed views: 3 глобальных шаблона (Владелец, Техподдержка, Менеджер) | ��� |

### Остаточные проблемы

| Приоритет | Проблема | Комментарий |
|---|---|---|
| LOW | Кнопка «Произвольный период» (Calendar icon) — date picker работает, но визуально неочевидна | Пресеты покрывают основные сценарии |

---

## [DONE] Модуль: Сетевая статистика — повторный аудит (aggregator, /network/statistics, fix mode + инфраструктура + QA full, 2026-04-17)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `portal-frontend/src/api/networkStats.ts` | `startExport` POST body отправлял `{ filter, mode, format }` с ISO-строками в `date_from`/`date_to` → Go-хендлер парсил даты как 0 → экспорт без фильтрации по дате | `filterToParams(filter)` конвертирует ISO → Unix timestamps перед JSON.stringify |
| 2 | `portal-frontend/src/components/network-stats/MonitoringTable.tsx` | `r.throughput.toFixed(0)` без null-safety → крэш при null/undefined throughput из API | `(r.throughput ?? 0).toFixed(0)` |
| 3 | `portal-frontend/src/pages/network/NetworkStatisticsPage.tsx` | `referencesApi.operators().catch(() => {})` — silent failure → пустой dropdown операторов без объяснения | `operatorsError` state + баннер «Не удалось загрузить список операторов» + кнопка «Повторить» |
| 4 | `portal-frontend/src/components/network-stats/MonitoringTable.tsx` | Нет pagination controls — компонент принимает `pagination` prop но не рендерит кнопки | Добавлены кнопки ←/→ + «Страница N из M» (аналогично StatisticsTable) |
| 5 | `portal-frontend/src/components/network-stats/DrillDownDrawer.tsx` | Summary KPI показывались через `fmt(kpi.value)` (plain число) → DLR rate 0.95 вместо 95.0%, revenue без ₽ | `fmtKPI()` — type-aware: rate/margin → %, revenue/profit/cost → ₽, остальные → число |
| 6 | `portal-frontend/src/components/network-stats/StatisticsTable.tsx`, `MonitoringTable.tsx`, `MonitoringKPIGrid.tsx` | Заголовок «Pending» на английском в трёх местах | «Ожидание» — русский |
| 7 | `portal-frontend/src/hooks/useNetworkStats.ts` | Export polling без max retry limit → бесконечный `setTimeout(pollExport, 2000)` при зависшем экспорте | `MAX_EXPORT_POLLS=90` (3 мин), error handling для отдельных poll-запросов, показ `status.error` при failed |
| 8 | `portal-frontend/src/components/network-stats/StatisticsKPIStrip.tsx` | `formatValue` и `valueColor` не распознавали английские KPI имена (`revenue`, `profit`, `margin`, `dlr_rate`, `failed`, `delivered`) → деньги без ₽, проценты как десятичные | Добавлены английские варианты имён в условия форматирования |

### Инфраструктура (проверка)

| Компонент | Статус |
|---|---|
| 10 маршрутов (GET statistics/analytics-summary/monitoring/drilldown/views, POST export/views, GET export/{job_id}/status/download, DELETE views/{id}) | ✅ все совпадают frontend ↔ backend |
| Proto `networkanalyticsv1`: 9 RPC ↔ 10 handlers (DownloadExport отдельный) | ✅ |
| Миграция 000102: 4 таблицы + индексы + seed data | ✅ |
| Auth: все handlers проверяют `GetClientID` | ✅ |
| TypeScript build: `tsc --noEmit` OK | ✅ |
| Go build: `go build ./internal/gateway/portal/...` OK | ✅ |

---

## [DONE] Модуль: Сетевая статистика — 3-й раунд (aggregator, /network/statistics, fix mode + инфраструктура + QA full, 2026-04-17)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `portal-frontend/src/components/network-stats/StatisticsFilterBar.tsx` | Клик по пресету периода (`7 дней`, `30 дней` и т.д.) вызывал `onFiltersChange({ period_preset: p.value })` без очистки `date_from`/`date_to` → бэкенд `Normalize()` проверяет `DateFrom.IsZero() && DateTo.IsZero()` и игнорировал пресет если были старые кастомные даты | `onFiltersChange({ period_preset: p.value, date_from: '', date_to: '' })` — старые даты очищаются |
| 2 | `portal-frontend/src/hooks/useNetworkStats.ts` | `handleSort` и `handlePageChange` в таблицах вызывали `onFiltersChange` + `onApply` в одном event handler → `applyFilters` использовал stale closure `filters` → сортировка и пагинация отправляли запрос со СТАРЫМИ значениями | `filtersRef` + `modeRef` — `fetchData` и `applyFilters` всегда читают актуальные значения через ref |
| 3 | `portal-frontend/src/hooks/useNetworkStats.ts` | Изменение фильтров (оператор, канал, статус) не сбрасывало `page` на 1 → на высоких страницах пользователь видел пустую таблицу | `setFilters` автоматически сбрасывает `page=1` при изменении не-пагинационных фильтров |
| 4 | `internal/gateway/portal/handlers/network_statistics.go` | 6 handlers без `checkClient()`: `GetDrillDown`, `StartExport`, `GetExportStatus`, `DownloadExport`, `SaveView`, `DeleteView` → nil dereference panic при отсутствии gRPC-клиента | Добавлен `h.checkClient(w)` во все 6 handlers — graceful 503 вместо паники |
| 5 | `portal-frontend/src/hooks/useNetworkStats.ts` | Смена вкладки в drill-down drawer (По операторам → По статусам → По ошибкам) вызывала `setDrillDownView` без перезапроса → все вкладки показывали одинаковые данные | `setDrillDownView` теперь вызывает `getDrillDown(filters, ..., view)` с новым `detail_view` |

### Инфраструктура (проверка)

| Компонент | Статус |
|---|---|
| 10 маршрутов (statistics/analytics-summary/monitoring/drilldown/export/views) | ✅ все совпадают frontend ↔ backend |
| `checkClient()` во всех 10 handlers | ✅ исправлено в этом раунде |
| Proto `networkanalyticsv1`: 9 RPC ↔ 10 handlers | ✅ |
| Миграция 000102: 4 таблицы + индексы + seed | ✅ |
| Auth: все handlers проверяют `GetClientID` | ✅ |
| TypeScript build: `tsc --noEmit` OK | ✅ |
| Go build: `go build ./internal/gateway/portal/...` OK | ✅ |
| Interaction chain: period preset → API → SQL | ✅ исправлено (date_from/date_to очищаются) |
| Interaction chain: sort/page → API (stale closure) | ✅ исправлено (ref pattern) |
| Interaction chain: drill-down tab → re-fetch | ✅ исправлено (setDrillDownView with fetch) |

## [DONE] Модуль: Сетевая статистика — 5-й раунд (aggregator, /network/statistics, fix mode + инфраструктура + QA full, 2026-04-17)

### Исправлено

| # | Файл / Сервис | Было | Стало |
|---|---|---|---|
| 1 | `internal/services/network_analytics/infrastructure/repository/export_repository.go` | `GetJob` сканировал NULL-колонки `file_path`, `row_count`, `error` в non-pointer Go типы → scan error | COALESCE для всех трёх nullable колонок |
| 2 | `internal/services/network_analytics/grpc/server.go` | `GetExportStatus`: если job не найден, `job.Status` → nil dereference panic | nil guard: возвращает `codes.NotFound` |
| 3 | `internal/gateway/portal/handlers/network_statistics.go` | `mux.Vars(r)["job_id"]` возвращал "" из-за бага gorilla/mux subrouter → `/export/{id}/status` отвечал 400 | `exportJobIDFromRequest` fallback: парсит UUID из `r.URL.Path` |
| 4 | `internal/gateway/portal/handlers/network_statistics.go` | gorilla/mux маршрутизировал `/export/{id}/download` на `GetExportStatus` handler (baг subrouter) → download всегда возвращал status JSON | `GetExportStatus` проверяет `strings.HasSuffix(path, "/download")` и делегирует `DownloadExport` |
| 5 | `internal/services/network_analytics/application/export_worker.go` | Отсутствовал background worker — jobs создавались, но никогда не обрабатывались (вечный `pending`) | Создан `ExportWorker`: poll pending jobs каждые 5s, генерирует CSV из `GetStatistics`/`GetMonitoringMetrics`, сохраняет в `/exports/{job_id}.csv` |
| 6 | `deployments/docker-compose.yml` | network-analytics-service и portal-gateway не имели shared volume → portal-gateway не мог прочитать файлы экспорта | Добавлен named volume `network-exports:/exports` в оба сервиса |
| 7 | `cmd/services/network-analytics-service/main.go` | ExportWorker не запускался в main | `go exportWorker.Run(ctx)` добавлен после AggregationWorker |
| 8 | `portal-frontend/src/hooks/useNetworkStats.ts` | `parseFiltersFromURL` не задавал дефолты → при первом открытии `period_preset` и `group_by` были undefined → API запрос без фильтров → backend GROUP BY operator → пустой Срез | `if (!f.period_preset) f.period_preset = '7d'`; `if (!f.group_by) f.group_by = 'day'` — корректная инициализация состояния |
| 9 | `portal-frontend/src/pages/network/NetworkStatisticsPage.tsx` | Панель saved views отсутствовала в UI — хук `useNetworkStats` полностью поддерживал виды, но компонент не рендерил кнопки | Добавлена фиксированная bottom-bar панель с кнопками загруженных видов и кнопкой «+ Сохранить» при наличии изменений |

### TC итог (раунд 5)

| TC | Описание | Вердикт |
|---|---|---|
| TC-7 | Export CSV: POST /export → polling status → GET /download | PASS ✅ CSV скачивается, content-type: text/csv, данные корректны |
| TC-8 | Начальная загрузка: Срез содержит даты (group_by=day, period_preset=7d) | PASS ✅ API: `?period_preset=7d&group_by=day`, таблица: строки 2026-04-10..17 |
| TC-9 | Saved views panel: кнопки видов отображаются, кнопка «+ Сохранить» при изменении фильтров | PASS ✅ Панель показывает 3 сохранённых вида |

## [DONE] Модуль: Сетевая статистика — 4-й раунд (aggregator, /network/statistics, fix mode + QA full, 2026-04-17)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `portal-frontend/src/components/network-stats/StatisticsKPIStrip.tsx` | KPI полоса показывала английские имена с бэкенда: `Pending` → 8 616 052 (без перевода) | Добавлена `KPI_LABELS` карта переводов: `pending`→«Ожидание», `total`→«Всего», `delivered`→«Доставлено», `failed`/`errors`→«Ошибки», `timeout`→«Таймаут», `revenue`→«Выручка», `profit`→«Прибыль», `cost`→«Себестоимость», `margin`→«Маржа», `dlr_rate`→«Доставляемость» |
| 2 | `portal-frontend/src/pages/network/NetworkStatisticsPage.tsx` | Вкладка Мониторинг: `Показано undefined провайдеров` когда `total_rows=0` (proto `omitempty` — нулевые int32 опускаются в JSON) | `(stats.data as any).pagination.total_rows ?? 0` — null-safe |
| 3 | `portal-frontend/src/api/networkStats.ts` | `filterToParams` копировал все поля фильтра включая пустые строки (`date_from:""`, `date_to:""`, `operator:""`) → POST `/export` body содержал `"date_from":""` → Go `encoding/json` не может декодировать `""` в `int64` → 400 Bad Request | Итерация через `Object.entries(f)` с пропуском пустых строк/undefined/null — только ненулевые поля попадают в тело запроса |
| 4 | `portal-frontend/src/hooks/useNetworkStats.ts` | `setFilters` → `setFiltersState(updater)` не обновлял `filtersRef.current` синхронно → вызов `applyFilters()` сразу после `setFilters` читал СТАРЫЕ значения фильтров из ref → сортировка и пагинация теряли новые значения | `filtersRef.current = next` внутри `setFiltersState` updater — ref обновляется синхронно до следующего рендера |

### Верификация в браузере

| Баг | До | После | Статус |
|---|---|---|---|
| BUG-1 KPI перевод | `Pending: 8 616 052` | `Ожидание: 8 616 052` | ✅ |
| BUG-2 undefined total_rows | `Показано undefined провайдеров` | `Показано 0 провайдеров` | ✅ |
| BUG-3 export 400 с пресетом | `POST /export → 400` (body: `date_from:""`) | `POST /export → 500` (body: `period_preset:"30d"` без date_from) | ✅ (фронтенд исправлен; 500 — отдельная проблема бэкенда экспорта) |
| BUG-4 сортировка stale | Клик по "Всего" не добавлял `sort_by` к запросу | URL: `?sort_by=total&sort_dir=desc`, запрос: `GET /statistics?period_preset=30d&sort_by=total&sort_dir=desc` | ✅ |

### TC итог

| TC | Описание | Вердикт |
|---|---|---|
| TC-7 | DrillDown: открытие + вкладки (По статусам → `detail_view=statuses`) | PASS ✅ |
| TC-9 | Экспорт CSV с пресетом | BUG-3 воспроизведён (400) → исправлен; после деплоя 500 от бэкенда экспорта (не фронтенд) |

---

## [DONE] Модуль: Агрегатор — полный аудит (aggregator, все страницы, fix mode + инфраструктура + QA full, 2026-04-17)

### Исправлено

| # | Файл | Было | Стало |
|---|---|---|---|
| 1 | `portal-frontend/src/pages/network/NetworkDashboardPage.tsx` | `{moderationTotal} заявок` — всегда "заявок" | Правильное склонение: 1 заявка / 2-4 заявки / 5+ заявок |
| 2 | `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx` | `parseFloat(sa.balance).toFixed(2) ₽` — без разделителей | `toLocaleString('ru-RU')` → "49 987,50 ₽" |
| 3 | `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx` | Подсказка баланса в форме создания: `parseFloat(parentBalance).toFixed(2)` | `toLocaleString('ru-RU')` — консистентный формат |
| 4 | `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx` | Breadcrumb `href: '/sub-accounts'` — неправильный путь | `href: '/network/sub-accounts'` |
| 5 | `internal/gateway/portal/handlers/sub_accounts.go` | `GetSubAccountMessages` возвращал raw proto `[]*messagingv1.MessageInfo` → `created_at` сериализовался как `{"seconds":N}` → "Invalid Date" на фронте | DTO-маппинг с `m.CreatedAt.AsTime().Format(time.RFC3339)` → корректная ISO-строка |
| 6 | `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx` | Колонка "Период" в Analytics tab: нет render — показывалась ISO-строка "2026-04-15" | `toLocaleDateString('ru-RU')` → "15.04.2026" |
| 7 | `internal/gateway/portal/handlers/reseller_routing.go` | SQL `cp.priority` (SELECT + ORDER BY) — колонка не существует → 500 | Исправлено на `cp.shared_priority` |
| 8 | `portal-frontend/src/pages/network/components/TariffPlanEditor.tsx` | `plans.length < 5 ? 'плана' : 'планов'` — "0 плана" неверно | Правильное склонение: 0 планов / 1 план / 2-4 плана / 5+ планов |

### Проверенные страницы (все OK после исправлений)

| Страница | URL | Статус |
|---|---|---|
| Network Dashboard | `/network/dashboard` | ✅ "1 заявка" — правильное склонение |
| Sub-accounts List | `/network/sub-accounts` | ✅ "49 987,50 ₽" — ru-RU форматирование |
| Sub-account Detail — Messages | `/network/sub-accounts/:id` → Сообщения | ✅ даты "15.04.2026, 12:43:41" вместо "Invalid Date" |
| Sub-account Detail — Analytics | `/network/sub-accounts/:id` → Аналитика | ✅ период "15.04.2026" вместо ISO |
| Sub-account Detail — Breadcrumb | `/network/sub-accounts/:id` | ✅ breadcrumb ведёт на `/network/sub-accounts` |
| Network Moderation | `/network/moderation` | ✅ данные загружаются |
| Network Routing | `/network/routing` | ✅ нет 500 после исправления `cp.shared_priority` |
| Network Tariffs — Обзор | `/network/tariffs` | ✅ тарифы по операторам загружаются |
| Network Tariffs — Шаблоны | `/network/tariffs` → Шаблоны | ✅ "1 шаблон" |
| Network Tariffs — Переопределения | `/network/tariffs` → Переопределения | ✅ "0 планов" вместо "0 плана" |
| Network Statistics | `/network/statistics` | ✅ данные, фильтры, экспорт работают |

### Инфраструктурная проблема

| Компонент | Статус |
|---|---|
| Docker build cache (96 ГБ) | ⚠️ Заполнял диск, блокировал сборку. Очищен `docker builder prune -af` |

## Test Accounts

| Email | Роль | Client | Назначение |
|---|---|---|---|
| loadtest-client@test.local | client | c0000000-0000-0000-0000-000000000001 | Тестирование клиентской панели |
| reseller-admin@test.local | client | Test Reseller Corp (is_reseller=true, max_sub_accounts=5) | Тест суб-аккаунтов |
| Sub-Account Alpha | sub_account | f686cc8e-ecd1-4233-9c79-215a85945806 | Создан реселлером для теста |
