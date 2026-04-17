# UX Audit Progress

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

## Test Accounts

| Email | Роль | Client | Назначение |
|---|---|---|---|
| loadtest-client@test.local | client | c0000000-0000-0000-0000-000000000001 | Тестирование клиентской панели |
| reseller-admin@test.local | client | Test Reseller Corp (is_reseller=true, max_sub_accounts=5) | Тест суб-аккаунтов |
| Sub-Account Alpha | sub_account | f686cc8e-ecd1-4233-9c79-215a85945806 | Создан реселлером для теста |
