# UX Audit Progress

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
