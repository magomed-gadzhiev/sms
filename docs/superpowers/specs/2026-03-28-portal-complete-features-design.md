# Portal Complete Features — Design Spec

**Date:** 2026-03-28
**Status:** Draft
**Approach:** Послойный (Layer-by-layer) — Gateway → Frontend → Backend

## Overview

Довести клиентский портал до полноценного UX, подключив все существующие бэкенд-сервисы и добавив недостающие компоненты. Бэкенд-сервисы (Template, Billing, Messaging, Auth, Tarification, Routing) уже реализованы — основная работа: прокидывание роутов на Portal Gateway, реализация фронтенд-страниц, и создание двух новых backend-компонентов (payment adapter, email notifications).

## Scope — 7 блоков

| # | Блок | Gateway | Frontend | Новый backend |
|---|------|---------|----------|---------------|
| 1 | Шаблоны | +7 роутов | TemplatesPage (CRUD + модерация) | — |
| 2 | Биллинг | +4 роута | BillingPage (транзакции, пополнение) | Payment adapter, email alerts |
| 3 | Сообщения | +2 роута | MessageDetailPage, фильтры, CSV экспорт | — |
| 4 | API-ключи | +2 роута | Просмотр/редактирование ключа | UpdateAPIKey RPC |
| 5 | Тарифы | +4 роута | TariffsPage (план, цены, смена) | Миграция: Free plan |
| 6 | Смена пароля | реализовать handler | Форма в ProfilePage | ChangePassword RPC |
| 7 | Lookup | +3 роута | LookupPage (single + bulk) | — |

## Слои реализации

1. **Portal Gateway** — все новые роуты и хендлеры (+22 новых endpoint, +1 реализация существующего 501)
2. **Proto + gRPC** — два новых RPC (UpdateAPIKey, ChangePassword)
3. **Frontend** — новые и доработанные страницы
4. **Backend** — payment adapter, email notifications, миграции

---

## Block 1: Шаблоны (Templates)

### Portal Gateway Routes

Все под `/portal/v1/templates`, session middleware:

| Метод | Путь | Описание | gRPC |
|-------|------|----------|------|
| POST | /templates | Создать шаблон (статус = pending_review) | TemplateService.CreateTemplate |
| GET | /templates | Список шаблонов клиента | TemplateService.ListTemplates |
| GET | /templates/{id} | Детали шаблона | TemplateService.GetTemplate |
| PUT | /templates/{id} | Обновить (только pending/rejected) | TemplateService.UpdateTemplate |
| DELETE | /templates/{id} | Удалить шаблон | TemplateService.DeleteTemplate |
| POST | /templates/{id}/render | Превью с подстановкой переменных | TemplateService.RenderTemplate |
| GET | /templates/{id}/audit | История изменений | TemplateService.GetTemplateAuditLog |

### Business Logic

- При создании шаблон получает статус `pending_review` — клиент не может одобрить сам
- Редактирование доступно только для шаблонов в статусе `pending` или `rejected`
- Одобрение/отклонение остаётся только в Admin Gateway (уже реализовано)
- Handler фильтрует шаблоны по `client_id` из сессии — клиент видит только свои

### Frontend: TemplatesPage

- Таблица шаблонов: имя, статус (badge: pending/approved/rejected), дата, действия
- Модалка создания/редактирования: имя, текст с плейсхолдерами (`{{name}}`, `{{code}}`), категория
- Превью: кнопка «Предпросмотр» → ввод тестовых значений → показ результата через RenderTemplate
- Статус `rejected` показывает причину отклонения (из audit log)
- Кнопка редактирования скрыта для шаблонов в статусе `approved`

---

## Block 2: Биллинг (Billing)

### Portal Gateway Routes

Под `/portal/v1/billing`:

| Метод | Путь | Описание | gRPC / Adapter |
|-------|------|----------|----------------|
| GET | /billing/balance | Детальный баланс | BillingService.GetBalance |
| GET | /billing/transactions | История транзакций (пагинация, фильтры) | BillingService.GetTransactionHistory |
| POST | /billing/top-up | Инициировать пополнение | PaymentProvider.CreatePayment |
| POST | /billing/top-up/callback | Webhook от платёжной системы (public, без session) | PaymentProvider.HandleCallback |

### Payment Adapter

Абстрактный интерфейс внутри Portal Gateway (`internal/gateway/portal/payment/`):

```go
type PaymentProvider interface {
    CreatePayment(ctx context.Context, req CreatePaymentRequest) (*PaymentSession, error)
    HandleCallback(ctx context.Context, raw []byte, signature string) (*PaymentResult, error)
}

type CreatePaymentRequest struct {
    ClientID  string
    Amount    decimal.Decimal
    Currency  string
    ReturnURL string
}

type PaymentSession struct {
    PaymentID  string
    PaymentURL string  // redirect клиента сюда
    ExpiresAt  time.Time
}

type PaymentResult struct {
    PaymentID string
    Status    string  // success / failed / pending
    Amount    decimal.Decimal
    Currency  string
}
```

**Поток пополнения:**
1. Клиент нажимает «Пополнить» → вводит сумму → `POST /billing/top-up`
2. Gateway вызывает `PaymentProvider.CreatePayment()` → получает `PaymentURL`
3. Клиент редиректится на страницу оплаты
4. Платёжка шлёт callback на `POST /billing/top-up/callback`
5. Gateway проверяет подпись, вызывает `BillingService.AddCredits` при успехе
6. Клиент возвращается на портал, видит обновлённый баланс

**Stub-реализация:** `StubPaymentProvider` — эмулирует успешную оплату для разработки/тестов. Реальный провайдер подключается заменой реализации.

### Email-уведомления о низком балансе

Новый пакет `internal/notifications/`:

```go
type EmailSender interface {
    Send(ctx context.Context, to string, subject string, body string) error
}
```

**Триггер:** При `ChargeMessage` Billing Service проверяет остаток. Если баланс ниже порога — публикует событие в Kafka (`billing.low_balance`). Consumer в worker-сервисе подхватывает и отправляет email.

**Порог:** Настраивается через конфиг клиента. Дефолт — 100 единиц валюты. Повторное уведомление не чаще раза в 24 часа (дедупликация через Redis-ключ `low_balance_notified:{client_id}`).

### Frontend: BillingPage

- Карточка баланса: текущий баланс, валюта, последнее обновление
- Кнопка «Пополнить» → модалка с вводом суммы → редирект на оплату
- Таблица транзакций: дата, тип (charge/credit/refund), сумма, баланс после, описание, message_id (ссылка)
- Фильтры: по дате, типу транзакции
- Пагинация

### Миграция

Добавить `low_balance_threshold DECIMAL DEFAULT 100` в таблицу `clients`.

---

## Block 3: Сообщения (Messages)

### Portal Gateway Routes

| Метод | Путь | Описание | gRPC |
|-------|------|----------|------|
| GET | /messages/{id} | Детали одного сообщения | MessagingService.GetMessageStatus |
| GET | /messages/export | Экспорт в CSV | MessagingService.GetMessageHistory |

Существующие `POST /messages` и `GET /messages` работают без изменений.

### Handler деталей сообщения

`GET /messages/{id}` возвращает:
- Основные поля: source, destination, text, status, segment_count
- Таймлайн доставки: created_at, sent_at, delivered_at, expired_at
- Провайдер: provider_id, route_id, SMPP message ID
- Стоимость: вызов `BillingService.GetTransactionHistory` с фильтром по message_id
- Проверка доступа: handler сверяет `client_id` сообщения с `client_id` из сессии

### CSV экспорт

`GET /messages/export?from=...&to=...&status=...&destination=...`:
- Те же фильтры что в `GET /messages`
- Gateway вызывает `GetMessageHistory` с лимитом 50 000 записей
- Формирует CSV streaming через `csv.Writer` → `http.ResponseWriter`
- Headers: `Content-Type: text/csv`, `Content-Disposition: attachment; filename=messages_YYYY-MM-DD.csv`
- Колонки: id, source, destination, text, status, segment_count, created_at, delivered_at, provider, cost
- Лимит: максимум 50 000 строк. Если больше — клиент сужает фильтры.

### Frontend

Доработка `MessagesPage`:
- Клик по строке → переход на `/messages/{id}` (новая `MessageDetailPage`)
- `MessageDetailPage`: карточка с деталями, визуальный таймлайн статусов (created → sent → delivered/failed), стоимость
- Панель фильтров: по статусу (multiselect), дате (date range picker), номеру получателя
- Кнопка «Экспорт CSV» — скачивает файл с текущими фильтрами

---

## Block 4: API-ключи (API Keys)

### Portal Gateway Routes

| Метод | Путь | Описание | gRPC |
|-------|------|----------|------|
| GET | /api-keys/{id} | Детали ключа | AuthService.ListAPIKeys + фильтр по id |
| PUT | /api-keys/{id} | Обновить ключ | AuthService.UpdateAPIKey (новый RPC) |

Существующие `POST`, `GET`, `DELETE` работают.

### Новый RPC: UpdateAPIKey

Добавить в `auth.proto`:

```protobuf
rpc UpdateAPIKey(UpdateAPIKeyRequest) returns (UpdateAPIKeyResponse);

message UpdateAPIKeyRequest {
  string key_id = 1;
  string user_id = 2;
  string name = 3;
  repeated string scopes = 4;
  repeated string allowed_ips = 5;
  google.protobuf.Timestamp expires_at = 6;
}

message UpdateAPIKeyResponse {
  APIKeyInfo key = 1;
}
```

Реализация в Auth Service: обновление полей в `api_keys` / `api_key_scopes`. Сам ключ (hash) не меняется.

### Frontend

Доработка `APIKeysPage`:
- Клик по ключу → модалка деталей: имя, prefix, scopes, allowed IPs, expires_at, last_used_at
- Кнопка «Редактировать» → форма: имя, scopes (checkbox list), IP whitelist (textarea), срок действия (date picker)
- Валидация: IP в формате CIDR или одиночный, минимум один scope

---

## Block 5: Тарифы (Tariffs)

### Portal Gateway Routes

Под `/portal/v1/tariffs`:

| Метод | Путь | Описание | gRPC |
|-------|------|----------|------|
| GET | /tariffs/current | Текущий план + цены по направлениям | ClientService.GetClient + TarificationService.GetTariffPlan |
| GET | /tariffs/plans | Доступные планы | ClientService.ListPlans |
| POST | /tariffs/change | Сменить план | ClientService.AssignPlan |
| GET | /tariffs/usage | Usage counters | TarificationService.ListUsageCounters |

### Handler текущего тарифа

`GET /tariffs/current` агрегирует:
1. `ClientService.GetClient(client_id)` → `plan_id`, данные `SubscriptionPlan`
2. `TarificationService.GetTariffPlan(plan_id)` → тиры, цены по направлениям, периоды

Возвращает: название плана, месячная стоимость, лимиты (SMS/месяц, SMPP connections, users), таблица цен по направлениям (страна → оператор → цена за сегмент), текущий usage.

### Смена плана

`POST /tariffs/change` с `{ "plan_id": "..." }`:
1. Проверяет что план существует и активен
2. Проверяет что это не текущий план
3. `ClientService.AssignPlan(client_id, plan_id)`
4. Запись в audit log

Смена вступает в силу немедленно. Без пропорционального перерасчёта (V1).

### Миграция: дефолтный Free план

- INSERT в `subscription_plans`: plan "Free" (monthly_price = 0, базовые лимиты)
- UPDATE клиентов без `plan_id` → назначить Free
- Логика `RegisterClient`: назначать Free план по умолчанию

### Frontend: TariffsPage

- Карточка текущего плана: название, цена, лимиты, прогресс-бар usage (X из Y SMS)
- Таблица цен по направлениям: страна, оператор, цена за сегмент — с поиском
- Блок «Сменить план»: карточки доступных планов (pricing cards), кнопка «Выбрать» → ConfirmDialog

---

## Block 6: Смена пароля (Change Password)

### Portal Gateway

Роут `PUT /portal/v1/profile/password` уже зарегистрирован, но handler возвращает 501.

### Новый RPC: ChangePassword

Добавить в `auth.proto`:

```protobuf
rpc ChangePassword(ChangePasswordRequest) returns (ChangePasswordResponse);

message ChangePasswordRequest {
  string user_id = 1;
  string current_password = 2;
  string new_password = 3;
}

message ChangePasswordResponse {
  bool success = 1;
}
```

### Логика в Auth Service

1. Проверить текущий пароль через bcrypt
2. Валидация нового пароля (минимум 8 символов, не совпадает с текущим)
3. Хеширование нового → UPDATE в БД
4. Инвалидация всех сессий кроме текущей
5. Запись в audit log

### Frontend

Блок в `ProfilePage` — «Сменить пароль»: текущий пароль, новый пароль, подтверждение. Валидация совпадения на клиенте.

---

## Block 7: Lookup (HLR/MNP)

### Portal Gateway Routes

Под `/portal/v1/lookup`:

| Метод | Путь | Описание | gRPC |
|-------|------|----------|------|
| POST | /lookup | Одиночный lookup | RoutingService.NumberLookup |
| POST | /lookup/bulk | Массовый lookup (CSV файл) | RoutingService.BulkNumberLookup |
| GET | /lookup/{id} | Статус массового lookup | RoutingService.GetLookupStatus |

Существующие `GET /lookup/history` и `GET /lookup/stats` работают.

### Одиночный lookup

`POST /lookup` с `{ "phone": "+79161234567" }`:
- Вызывает `RoutingService.NumberLookup`
- Возвращает: оператор, страна, MNP-статус, сеть, статус номера (active/inactive)
- Стоимость: `TarificationService.TarifyLookup` для списания

### Массовый lookup

`POST /lookup/bulk` — multipart/form-data с CSV:
1. Парсинг CSV (одна колонка — номера)
2. Валидация: максимум 10 000 номеров, формат E.164
3. Вызов `RoutingService.BulkNumberLookup` → job ID
4. Клиент поллит `GET /lookup/{id}` для статуса
5. По завершении — результат как CSV для скачивания

Хранение результата: Redis с TTL 24 часа.

### Frontend: LookupPage

- Вкладка «Одиночный»: поле ввода номера → «Проверить» → карточка результата (оператор, страна, MNP, статус)
- Вкладка «Массовый»: drag-and-drop для CSV → прогресс-бар → «Скачать результат»
- Таблица истории lookup (из `GET /lookup/history`)
- Статистика (из `GET /lookup/stats`)

---

## Новые Proto-определения (сводка)

### auth.proto — 2 новых RPC

```protobuf
rpc UpdateAPIKey(UpdateAPIKeyRequest) returns (UpdateAPIKeyResponse);
rpc ChangePassword(ChangePasswordRequest) returns (ChangePasswordResponse);
```

### Kafka — 1 новый топик

- `billing.low_balance` — событие при падении баланса ниже порога

### Миграции — 2 новых

- `000025_add_low_balance_threshold.up.sql` — поле `low_balance_threshold` в clients
- `000026_add_free_plan.up.sql` — INSERT Free план, UPDATE клиентов без плана

## Sidebar Navigation Update

Добавить пункты в `Sidebar.tsx`:
- Шаблоны (иконка: документ)
- Биллинг (иконка: кошелёк)
- Тарифы (иконка: ценник)
- Lookup (иконка: поиск)

Существующие пункты (Сообщения, API-ключи, Профиль) — без изменений, страницы доработаны.
