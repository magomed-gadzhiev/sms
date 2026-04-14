# UX Audit Progress

## [DONE] Аудит: все клиентские модули (client, fix mode + инфраструктура)

Дата: 2026-04-15. Режим: fix + browser testing. Проверка инфраструктуры: yes.

---

## Test Accounts

| Email | Роль | Client | Назначение |
|---|---|---|---|
| reseller-admin@test.local | client | Test Reseller Corp (is_reseller=true, max_sub_accounts=5) | Тест суб-аккаунтов |
| Sub-Account Alpha | sub_account | f686cc8e-ecd1-4233-9c79-215a85945806 | Создан реселлером для теста |

---

## Критические баги (FIXED)

### [CRITICAL] 401 на всех admin API — uuid.Parse("") для admin-сессий
- **Файл:** `internal/services/auth/infrastructure/session_manager.go`
- **Причина:** `uuid.Parse(result["client_id"])` вызывался даже когда `client_id = ""` (у admin-пользователей)
- **Fix:** Guard `if clientIDStr != "" { ... }` перед parse
- **Commit:** `390b484`

### [CRITICAL] account_type нарушение constraint при создании суб-аккаунта
- **Файл:** `internal/services/client/infrastructure/repository/client_repository.go`
- **Причина:** INSERT в `clients` не устанавливал `account_type`. Дефолт `'direct'` нарушал `chk_direct_no_parent` когда `parent_client_id IS NOT NULL`
- **Fix:** Вычислять `accountType = "sub_account"` если `ParentClientID != nil`
- **Commit:** `be20324`

### [CRITICAL] accounts.company_id NOT NULL нарушает триггер создания аккаунта
- **Файл:** DB (триггер `trg_create_account_for_new_client`)
- **Причина:** Триггер не задаёт `company_id`, а колонка была NOT NULL
- **Fix:** `ALTER TABLE accounts ALTER COLUMN company_id DROP NOT NULL` (частичный индекс `WHERE company_id IS NOT NULL` уже существовал)

### [CRITICAL] is_reseller колонка отсутствует на сервере
- **Причина:** Миграция 000016 не была применена на продакшн-сервере
- **Fix:** `ALTER TABLE clients ADD COLUMN IF NOT EXISTS is_reseller BOOLEAN NOT NULL DEFAULT false`

---

## Исправления frontend (30+)

### Суб-аккаунты
- Loading spinner с `role="status"` в SubAccountsListPage
- Email validation regex в форме создания
- Guard против отрицательных лимитов
- Empty state с иконкой и текстом
- `toast.success` после успешного создания
- `min="0"` на числовых полях
- Баланс теперь отображается как "0.00 ₽" (было "0.000000")
- Предупреждение при неудачном переводе начального баланса (`balance_transfer_error`)

### SubAccountDetailPage
- Spinner с `role="status"` при загрузке
- `role="alert"` + кнопка Retry при ошибке
- Error states во вкладках Overview/Messages/Analytics
- Toast для сохранения лимитов и перевода баланса
- Валидация перевода (`min="0.01"`)
- Empty states для API Keys и Webhooks вкладок
- Info-баннеры с пояснениями read-only вкладок

### CommandCenter
- Skeleton loading с `role="status"`
- Субтитры для метрик
- `role="status"` и `aria-label` для live feed

### Quick Send
- CharacterCounter для сообщения
- ConfirmDialog перед отправкой
- `toast.success`/`toast.error` после результата

### Campaigns
- `toast.error` вместо `alert()`
- `role="alert"` на ошибках
- Empty state с SVG иконкой

### CampaignDetail
- Skeleton loading с `role="status"`
- `role="alert"` на ошибках
- `toast.error` вместо `alert()`

### Billing
- Валидация максимальной суммы (1,000,000 ₽)
- `role="alert"` на error/success состояниях
- Empty state для транзакций

### Messages / MessageTable
- `role="alert"` на ошибках
- Spinner с `role="status"`
- Empty state с "измените фильтры"

### API (client.ts)
- Добавлены методы `subAccountsApi.apiKeys()` и `subAccountsApi.webhooks()`

---

## Инфраструктурные расхождения

| Статус | Компонент | Проблема |
|---|---|---|
| FIXED | Auth service | uuid.Parse("") для пустого client_id в admin-сессиях |
| FIXED | DB Schema | account_type не устанавливался при создании sub-account |
| FIXED | DB Schema | accounts.company_id NOT NULL блокировало создание клиентов |
| FIXED | DB Schema | is_reseller колонка отсутствовала |
| OPEN | Admin gateway | `/admin/v1/audit` маршрут не найден (404) |
| OPEN | Analytics API | Backend возвращает `groups`, frontend ожидает `rows` |
| OPEN | Portal gateway | Notifications polling: `client_id is required` для sub-account сессий |

---

## Остаточные UX-проблемы (не исправлены)

| Приоритет | Модуль | Описание |
|---|---|---|
| HIGH | Admin Users | Форма создания пользователя не имеет поля `client_id` — нельзя связать user с client через UI |
| HIGH | Admin Audit | `/admin/v1/audit` маршрут отсутствует — страница журнала аудита не работает |
| HIGH | Analytics | Формат ответа API (`groups` vs `rows`) не совпадает — аналитика пустая |
| MED | Sub-accounts | Начальный баланс не валидируется против текущего баланса реселлера |
| LOW | Sub-accounts | contact_person не возвращается в `formatSubAccount()` в admin handler |
