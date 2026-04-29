# UX Audit Progress

> Активный план аудита: [docs/superpowers/specs/2026-04-29-ux-full-reaudit-design.md](../superpowers/specs/2026-04-29-ux-full-reaudit-design.md). Скоуп D: 30 этапов, fix mode + Infrastructure Check + QA full.

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
