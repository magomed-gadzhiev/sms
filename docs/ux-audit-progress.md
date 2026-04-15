# UX Audit Progress

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
| Нет endpoint `/sub-accounts/{id}/transactions` | ❌ агрегатор не видит историю транзакций суб-аккаунта |

### Остаточные проблемы

| Приоритет | Проблема | Комментарий |
|---|---|---|
| HIGH | Нет endpoint `/sub-accounts/{id}/transactions` | Агрегатор не может видеть историю расходов суб-аккаунта. Нужен backend handler + вкладка «Транзакции» в SubAccountDetailPage |
| MED | CommandCenter показывает только баланс агрегатора | Для реселлера было бы полезно видеть суммарный баланс суб-аккаунтов или количество суб-аккаунтов с низким балансом |
| LOW | Нет обратного перевода (суб-аккаунт → агрегатор) в UI | Форма перевода только в одну сторону. При необходимости возврата — только через удаление суб-аккаунта (автоматический возврат) |

---

## Test Accounts

| Email | Роль | Client | Назначение |
|---|---|---|---|
| loadtest-client@test.local | client | c0000000-0000-0000-0000-000000000001 | Тестирование клиентской панели |
| reseller-admin@test.local | client | Test Reseller Corp (is_reseller=true, max_sub_accounts=5) | Тест суб-аккаунтов |
| Sub-Account Alpha | sub_account | f686cc8e-ecd1-4233-9c79-215a85945806 | Создан реселлером для теста |
