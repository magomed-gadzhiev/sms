# DDD Домены

## Обзор

Система SMPP сервера построена на основе Domain-Driven Design (DDD) принципов. Каждый домен представляет отдельную бизнес-область с четко определенными границами и ответственностью.

## Домены системы

### 1. Authentication Domain (Auth Service)

**Ответственность:** Управление аутентификацией и авторизацией пользователей.

**Агрегаты:**
- `User` - пользователь системы (Admin, Client, Operator)
- `Role` - роль пользователя с набором разрешений
- `Permission` - разрешение на выполнение операции
- `APIKey` - API ключ для доступа к API

**Сервисы:**
- `AuthService` - аутентификация пользователей
- `TokenService` - управление JWT токенами

**Хранилище:**
- `users` - пользователи
- `roles` - роли
- `permissions` - разрешения
- `api_keys` - API ключи
- Redis кэш для токенов и сессий

**События:**
- `user.authenticated`
- `user.session.created`
- `api.key.created`

### 2. Messaging Domain (Messaging Service)

**Ответственность:** Управление SMS сообщениями и их жизненным циклом.

**Агрегаты:**
- `Message` - SMS сообщение
- `DLR` - Delivery Receipt (отчет о доставке)
- `MessageStatus` - статус сообщения (pending, queued, sent, delivered, failed, expired, rejected)

**Сервисы:**
- `MessageService` - создание и управление сообщениями
- `DLRService` - обработка delivery receipts

**Хранилище:**
- `messages` - сообщения (партиционирование по датам)
- `dlr_receipts` - delivery receipts

**События (Kafka):**
- Publish: `message.created`, `message.queued`, `dlr.received`
- Subscribe: `message.status.changed`, `message.sent`

**Валидация:**
- Длина сообщения
- Формат номера получателя
- Кодировка текста
- Максимальное количество сегментов

### 3. Routing Domain (Routing Service)

**Ответственность:** Определение маршрута для сообщений и выбор оптимального провайдера.

**Агрегаты:**
- `Route` - правило маршрутизации
- `RouteRule` - условие маршрута (префикс, регулярное выражение, провайдер)
- `LoadBalanceStrategy` - стратегия балансировки нагрузки

**Сервисы:**
- `RoutingService` - определение маршрута
- `ProviderSelector` - выбор провайдера (RoundRobin, LeastLoaded, Cheapest)

**Хранилище:**
- `routes` - правила маршрутизации
- `route_rules` - условия маршрутов

**События (Kafka):**
- Subscribe: `message.queued`
- Publish: `message.routed`

**Стратегии маршрутизации:**
- По префиксу номера
- По провайдеру
- По приоритету
- Failover на резервные маршруты

**Стратегии балансировки:**
- Round Robin - равномерное распределение
- Least Loaded - наименее загруженный провайдер
- Cheapest - самый дешевый провайдер

### 4. Provider Management Domain (Provider Service)

**Ответственность:** Управление SMSC провайдерами и отправка сообщений.

**Агрегаты:**
- `Provider` - SMSC провайдер
- `Connection` - SMPP соединение к провайдеру
- `ConnectionPool` - пул соединений
- `HealthStatus` - статус здоровья провайдера

**Сервисы:**
- `ProviderService` - управление провайдерами
- `ConnectionPoolManager` - управление пулом соединений
- `SenderService` - отправка сообщений в SMSC

**Хранилище:**
- `providers` - провайдеры
- `provider_connections` - активные соединения
- `provider_stats` - статистика провайдеров

**События (Kafka):**
- Subscribe: `message.routed`
- Publish: `message.sent`, `message.failed`, `dlr.received`

**Функции:**
- Управление SMPP соединениями (bind, unbind, enquire_link)
- Health monitoring провайдеров
- Автоматическое переподключение
- Throttling на уровне провайдера
- Retry логика с экспоненциальной задержкой

### 5. Client Management Domain (Client Service)

**Ответственность:** Управление клиентами и их настройками.

**Агрегаты:**
- `Client` - клиент системы
- `ClientConfig` - конфигурация клиента
- `RateLimit` - настройки rate limiting

**Сервисы:**
- `ClientService` - управление клиентами
- `ConfigService` - управление конфигурацией

**Хранилище:**
- `clients` - клиенты
- `client_configs` - конфигурации клиентов
- `client_rate_limits` - лимиты запросов

**Функции:**
- CRUD операции с клиентами
- Настройка rate limiting
- Управление балансом
- Настройка тарифов

### 6. Analytics Domain (Analytics Service)

**Ответственность:** Сбор статистики и генерация отчетов.

**Агрегаты:**
- `Metric` - метрика (сообщений в секунду, успешность доставки)
- `Report` - отчет
- `AggregatedMetric` - агрегированная метрика

**Сервисы:**
- `AnalyticsService` - сбор и агрегация метрик
- `ReportService` - генерация отчетов

**Хранилище:**
- `message_stats` - статистика сообщений
- `aggregated_metrics` - агрегированные метрики
- `reports` - отчеты
- Timeseries БД для временных рядов

**События (Kafka):**
- Subscribe: `message.created`, `message.sent`, `message.delivered`, `message.failed`

**Метрики:**
- Количество сообщений по статусам
- Время доставки (latency)
- Успешность доставки (delivery rate)
- Производительность провайдеров
- Статистика по клиентам

### 7. Billing Domain (Billing Service)

**Ответственность:** Тарификация сообщений и управление балансами.

**Агрегаты:**
- `Account` - счет клиента
- `Transaction` - транзакция (charge, credit, refund)
- `PricingRule` - правило тарификации

**Сервисы:**
- `BillingService` - тарификация и управление балансом
- `PricingService` - расчет стоимости сообщений

**Хранилище:**
- `accounts` - счета клиентов
- `transactions` - транзакции
- `pricing_rules` - правила тарификации

**События (Kafka):**
- Subscribe: `message.delivered`, `message.failed`
- Publish: `balance.changed`, `transaction.completed`

**Операции:**
- Получение баланса клиента
- Списание средств за сообщение
- Пополнение счета
- История транзакций
- Расчет стоимости по правилам тарификации

## Взаимодействие доменов

### Пример: Отправка сообщения

1. **Client Gateway** → `Auth Service` - валидация токена
2. **Client Gateway** → `Messaging Service` - создание сообщения
3. **Messaging Service** → Kafka - публикация `message.created`
4. **Routing Service** (Kafka consumer) → получение `message.queued`
5. **Routing Service** → `Provider Service` (gRPC) - выбор провайдера
6. **Routing Service** → Kafka - публикация `message.routed`
7. **Provider Service** (Kafka consumer) → отправка в SMSC
8. **Provider Service** → Kafka - публикация `message.sent`
9. **Analytics Service** (Kafka consumer) → обновление статистики
10. **Billing Service** (Kafka consumer) → тарификация после доставки

## Границы контекстов (Bounded Contexts)

Каждый домен изолирован и общается с другими доменами через:

1. **gRPC** - синхронные запросы для получения данных
2. **Kafka** - асинхронные события для уведомлений
3. **Shared Database** - общая БД (в будущем можно разделить)

### Анти-паттерны (избегать)

- ❌ Прямые зависимости между доменами (кроме через интерфейсы)
- ❌ Прямой доступ к данным другого домена
- ❌ Дублирование бизнес-логики между доменами
- ❌ Нарушение границ агрегатов

### Правильные практики

- ✅ Использование событий для асинхронной коммуникации
- ✅ Использование gRPC для синхронных запросов
- ✅ Инкапсуляция бизнес-логики в доменных сервисах
- ✅ Четкие интерфейсы для взаимодействия

## Модель данных

### Shared Database Schema

```
users
├── id (uuid, PK)
├── username (varchar)
├── email (varchar)
├── role_id (uuid, FK -> roles.id)
└── ...

roles
├── id (uuid, PK)
├── name (varchar)
└── ...

permissions
├── id (uuid, PK)
├── name (varchar)
└── ...

clients
├── id (uuid, PK)
├── name (varchar)
├── account_id (uuid, FK -> accounts.id)
└── ...

messages
├── id (uuid, PK)
├── client_id (uuid, FK -> clients.id)
├── source (varchar)
├── destination (varchar)
├── text (text)
├── status (varchar)
├── created_at (timestamp)
└── ... (партиционирование по created_at)

providers
├── id (uuid, PK)
├── name (varchar)
├── host (varchar)
├── port (int)
└── ...

routes
├── id (uuid, PK)
├── name (varchar)
├── priority (int)
└── ...

accounts
├── id (uuid, PK)
├── client_id (uuid, FK -> clients.id)
├── balance (decimal)
└── ...

transactions
├── id (uuid, PK)
├── account_id (uuid, FK -> accounts.id)
├── type (varchar)
├── amount (decimal)
├── message_id (uuid, FK -> messages.id)
└── ...
```

## Дополнительная документация

- [Обзор архитектуры](overview.md)
- [Микросервисы](microservices.md)
- [Паттерны коммуникации](communication-patterns.md)
- [DDD паттерны](../development/ddd-patterns.md)