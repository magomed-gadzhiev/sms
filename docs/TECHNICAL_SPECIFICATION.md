# Техническая спецификация: SMPP Server

**Версия:** 1.0
**Дата:** 2026-03-20
**Статус:** Текущая архитектура (по состоянию кодовой базы)

---

## 1. Общее описание

**SMPP Server** — микросервисная платформа для маршрутизации и доставки SMS-сообщений по протоколу SMPP v3.4. Является аналогом связки Kannel + smsbox, реализованным на Go с поддержкой HTTP REST и gRPC API.

**Назначение:** Приём SMS от клиентов (через HTTP/gRPC/SMPP), маршрутизация к SMSC-провайдерам, обработка delivery receipts, биллинг и аналитика.

**Модуль Go:** `github.com/smpp-server/smpp-server`
**Версия Go:** 1.24.0

---

## 2. Архитектура

### 2.1 Архитектурный стиль

- **Микросервисная архитектура** с Domain-Driven Design (DDD)
- **Асинхронная обработка** через Apache Kafka
- **Синхронное межсервисное взаимодействие** через gRPC
- **Клиентский доступ** через HTTP REST и gRPC (gateway-слой)

### 2.2 Компонентная диаграмма

```
                        ┌─────────────────────────────────────────┐
                        │              HAProxy                     │
                        │      (Load Balancer & Router)            │
                        └───┬──────────────┬──────────────┬───────┘
                            │              │              │
                   ┌────────▼────────┐ ┌───▼──────────┐ ┌▼──────────────┐
                   │ Admin Gateway   │ │Client Gateway│ │ SMPP Gateway  │
                   │ HTTP:8081       │ │ HTTP:8080    │ │ SMPP:2775     │
                   │ gRPC:9091       │ │ gRPC:9090    │ │               │
                   └────────┬────────┘ └───┬──────────┘ └┬──────────────┘
                            │              │              │
              ┌─────────────┴──────────────┴──────────────┘
              │         gRPC (синхронно)
    ┌─────────┼─────────────┬──────────────┬──────────────┐
    │         │             │              │              │
┌───▼───┐ ┌──▼──────┐ ┌────▼─────┐ ┌─────▼────┐ ┌──────▼──────┐
│ Auth  │ │Messaging│ │ Routing  │ │ Provider │ │   Client    │
│Service│ │ Service │ │ Service  │ │ Service  │ │  Service    │
│:9101  │ │ :9092   │ │  :9093   │ │  :9094   │ │   :9095     │
└───────┘ └────┬────┘ └─────────-┘ └──────────┘ └─────────────┘
               │
    ┌──────────┼──────────────────────────────┐
    │          │    Kafka (асинхронно)         │
    │   ┌──────▼──────┐    ┌──────────────┐   │
    │   │ Analytics   │    │   Billing     │   │
    │   │ Service     │    │   Service     │   │
    │   │  :9096      │    │    :9097      │   │
    │   └─────────────┘    └──────────────┘   │
    └─────────────────────────────────────────┘
                     │
              ┌──────▼──────┐
              │   Worker    │
              │ (Kafka      │
              │  consumer)  │
              └──────┬──────┘
                     │ SMPP
              ┌──────▼──────┐
              │SMSC Provider│
              └─────────────┘
```

### 2.3 Инфраструктурные зависимости

| Компонент    | Технология     | Назначение                                 |
|-------------|----------------|---------------------------------------------|
| БД          | PostgreSQL 13+ | Основное хранилище (партиционирование)      |
| Кэш         | Redis          | Rate limiting, кэш клиентов, сессии         |
| Очередь     | Apache Kafka   | Асинхронная обработка сообщений             |
| Балансировка| HAProxy 2.8    | Распределение нагрузки между gateway        |
| Мониторинг  | Prometheus     | Сбор и хранение метрик                      |

---

## 3. Сервисы

### 3.1 Gateway-слой

#### 3.1.1 Client Gateway (`cmd/client-gateway/`)

**Назначение:** Точка входа для клиентов — отправка SMS, проверка статусов, баланс.

**Протоколы:** HTTP (`:8080`), gRPC (`:9090`)

**HTTP Endpoints:**
| Метод | Путь                      | Описание                |
|-------|---------------------------|-------------------------|
| POST  | `/api/v1/sms/send`        | Отправка одного SMS     |
| POST  | `/api/v1/sms/batch`       | Пакетная отправка       |
| GET   | `/api/v1/sms/status`      | Статус сообщения по ID  |
| GET   | `/api/v1/sms/history`     | История сообщений       |
| GET   | `/api/v1/account/balance` | Баланс клиента          |
| GET   | `/api/v1/account/stats`   | Статистика клиента      |

**Аутентификация:** API-ключ в заголовке `X-API-Key` или `Authorization: Bearer <key>`

**Middleware-цепочка:** Recovery → Logging → CORS → Authentication → Rate Limiting

**gRPC:** Реализует `MessagingServiceServer` из `messagingv1`

**Зависимости (gRPC-клиенты):** Auth Service, Messaging Service, Analytics Service, Billing Service

#### 3.1.2 Admin Gateway (`cmd/admin-gateway/`)

**Назначение:** API для администраторов — управление клиентами, провайдерами, маршрутами, биллинг, аналитика.

**Протоколы:** HTTP (`:8081`), gRPC (`:9091`)

**HTTP Endpoints:**
| Группа      | Путь                               | Операции           |
|------------|-------------------------------------|--------------------|
| Клиенты    | `/admin/v1/clients[/:id]`          | CRUD               |
| Провайдеры | `/admin/v1/providers[/:id]`        | CRUD + health      |
| Маршруты   | `/admin/v1/routes[/:id]`           | CRUD               |
| Аналитика  | `/admin/v1/analytics/{stats,reports,metrics}` | Read     |
| Биллинг    | `/admin/v1/billing/{transactions,accounts}` | Read + credits |

**Аутентификация:** JWT-токен (`Authorization: Bearer <jwt>`) с ролью `admin`

**Middleware-цепочка:** Recovery → Logging → Authentication (JWT) → Authorization (role check) → RequestID

#### 3.1.3 SMPP Gateway (`cmd/smpp-gateway/`)

**Назначение:** Приём входящих SMPP-соединений от клиентов (протокол SMPP v3.4).

**Протокол:** TCP (`:2775`)

**Поддерживаемые команды:**
- Bind: `bind_receiver`, `bind_transmitter`, `bind_transceiver` + resp
- Сообщения: `submit_sm` / `submit_sm_resp`, `deliver_sm` / `deliver_sm_resp`, `query_sm` / `query_sm_resp`
- Сессии: `unbind` / `unbind_resp`, `enquire_link` / `enquire_link_resp`

**Аутентификация:** `system_id` + `password` из bind-запроса → валидация через Auth Service

**Rate limiting:** Per-session, настраивается через конфигурацию

### 3.2 Доменные сервисы

#### 3.2.1 Auth Service (`cmd/services/auth-service/`, порт `:9101`)

**Домен:** Authentication & Authorization

**gRPC API (proto: `auth/auth.proto`):**
| Метод              | Описание                                  |
|--------------------|--------------------------------------------|
| Authenticate       | Аутентификация (username/password/API key) |
| ValidateToken      | Валидация JWT-токена                       |
| RefreshToken       | Обновление токена                          |
| GetUserInfo        | Информация о пользователе                 |
| CreateAPIKey       | Создание API-ключа                        |
| RevokeAPIKey       | Отзыв API-ключа                           |
| GetPermissions     | Получение разрешений                       |
| CheckPermission    | Проверка конкретного разрешения            |

**Агрегаты:** User, Role, Permission, APIKey

**Реализация:**
- `internal/services/auth/application/token_service.go` — JWT-менеджер (генерация, валидация, refresh)
- `internal/services/auth/grpc/server.go` — gRPC-сервер
- Хранилище: таблицы `users`, `roles`, `permissions`, `api_keys`
- Кэш: Redis (токены, сессии)

#### 3.2.2 Messaging Service (`cmd/services/messaging-service/`, порт `:9092`)

**Домен:** SMS Message Lifecycle

**gRPC API (proto: `messaging/messaging.proto`):**
| Метод             | Описание                             |
|-------------------|-----------------------------------------|
| SendMessage       | Создание и постановка в очередь         |
| SendBatch         | Пакетная отправка                       |
| GetMessageStatus  | Получение статуса по ID                 |
| GetMessageHistory | История с фильтрацией и пагинацией     |
| ProcessDLR        | Обработка delivery receipt              |

**Агрегаты:** Message, DLR, MessageStatus

**Жизненный цикл сообщения:**
```
pending → queued → sent → delivered
                      ↘ failed
                      ↘ expired
                      ↘ rejected
```

**Валидация:** Длина текста, формат номера, кодировка, кол-во сегментов

#### 3.2.3 Routing Service (`cmd/services/routing-service/`, порт `:9093`)

**Домен:** Message Routing & Provider Selection

**gRPC API (proto: `routing/routing.proto`):**
| Метод         | Описание                              |
|---------------|---------------------------------------|
| GetRoute      | Определение маршрута для сообщения    |
| CreateRoute   | Создание правила маршрутизации        |
| UpdateRoute   | Обновление правила                    |
| DeleteRoute   | Удаление правила                      |
| ListRoutes    | Список маршрутов                      |

**Стратегии маршрутизации:**
1. По явному `ProviderID` в сообщении
2. По явному `RouteID`
3. По паттерну номера назначения (prefix, regex, exact)
4. Fallback на первого активного провайдера

**Стратегии балансировки:** Round Robin, Least Loaded, Cheapest

**Failover:** Автоматическое переключение на `failover_provider_id` при недоступности основного

#### 3.2.4 Provider Service (`cmd/services/provider-service/`, порт `:9094`)

**Домен:** SMSC Provider Management

**gRPC API (proto: `provider/provider.proto`):**
| Метод           | Описание                               |
|-----------------|----------------------------------------|
| GetProvider     | Информация о провайдере                |
| CreateProvider  | Регистрация нового провайдера          |
| UpdateProvider  | Обновление конфигурации                |
| DeleteProvider  | Удаление провайдера                    |
| ListProviders   | Список провайдеров                     |
| GetProviderHealth | Состояние здоровья провайдера        |
| SendToProvider  | Отправка сообщения через провайдера    |

**Конфигурация провайдера:**
- Адрес (host, port), credentials (system_id, password)
- Тип bind (TX, RX, TRX), адресные параметры (TON, NPI)
- Max connections, throughput_per_second, priority
- Active/inactive состояние

#### 3.2.5 Client Service (`cmd/services/client-service/`, порт `:9095`)

**Домен:** Client Management

**gRPC API (proto: `client/client.proto`):**
| Метод          | Описание                        |
|----------------|----------------------------------|
| GetClient      | Информация о клиенте             |
| CreateClient   | Создание нового клиента          |
| UpdateClient   | Обновление клиента               |
| DeleteClient   | Удаление клиента                 |
| ListClients    | Список клиентов                  |
| GetClientConfig| Получение конфигурации           |

**Конфигурация клиента:**
- API-ключ + secret
- Rate limits: per second / per minute / per hour
- Allowed source addresses (TEXT[])
- Active/inactive

#### 3.2.6 Analytics Service (`cmd/services/analytics-service/`, порт `:9096`)

**Домен:** Statistics & Reporting

**gRPC API (proto: `analytics/analytics.proto`):**
| Метод             | Описание                            |
|-------------------|--------------------------------------|
| GetStats          | Статистика за период                 |
| GetRealTimeMetrics| Метрики в реальном времени           |
| GetProviderStats  | Производительность провайдеров       |
| GenerateReport    | Генерация отчёта (JSON/CSV/PDF)      |

**Метрики:** Кол-во сообщений по статусам, latency доставки, delivery rate, статистика по клиентам и провайдерам

**Kafka consumer:** Подписка на `message.created`, `message.sent`, `message.delivered`, `message.failed`

#### 3.2.7 Billing Service (`cmd/services/billing-service/`, порт `:9097`)

**Домен:** Billing & Accounting

**gRPC API (proto: `billing/billing.proto`):**
| Метод              | Описание                        |
|--------------------|---------------------------------|
| GetBalance         | Баланс счёта клиента            |
| ChargeMessage      | Списание за сообщение           |
| CreditAccount      | Пополнение счёта                |
| GetTransactions    | История транзакций              |
| GetPricingRules    | Правила тарификации             |
| SetPricingRule     | Установка тарифа                |

**Типы транзакций:** charge, credit, refund, adjustment

**Тарификация:** Правила по destination_pattern (regex) с приоритетами, глобальные и per-client

**Kafka consumer:** Подписка на `message.delivered`, `message.failed`

### 3.3 Worker (`cmd/worker/`)

**Назначение:** Kafka consumer + SMPP sender — ядро обработки сообщений.

**Kafka topics (consumer group: `smpp-worker`):**
| Топик           | Обработка                                   |
|-----------------|----------------------------------------------|
| `sms.outgoing`  | Маршрутизация → отправка через SMSC Pool    |
| `sms.dlr`       | Обновление статуса по delivery receipt        |
| `sms.failed`    | Финальная маркировка ошибки                  |

**Компоненты:**
- **Router** (`internal/router/router.go`) — выбор провайдера
- **RetryManager** (`internal/router/retry.go`) — экспоненциальный backoff (`base * 2^retryCount`, cap = max)
- **SMSC Pool** (`internal/smsc/pool.go`) — пул SMPP-соединений, keep-alive (`enquire_link`), throttling (token bucket)
- **Sender** (`internal/smsc/sender.go`) — формирование `submit_sm` PDU, отправка, получение `submit_sm_resp`

**Retry-политика:**
- Максимум попыток: настраивается (default 5)
- Backoff: экспоненциальный, base 1s, max 60s
- Permanent errors (invalid address, auth failure) → сразу в DLQ (`sms.failed`)
- Temporary errors → retry с задержкой

**Масштабирование:** Несколько инстансов в одном consumer group; Kafka распределяет партиции автоматически

---

## 4. Модель данных

### 4.1 Схема БД

**7 миграций** (`migrations/`), ключевые таблицы:

#### Core

| Таблица         | Назначение                     | Партиционирование |
|----------------|--------------------------------|---------------------|
| `messages`     | SMS-сообщения (полный lifecycle)| По месяцам (created_at) |
| `dlr_receipts` | Delivery receipts от SMSC      | —                   |
| `providers`    | SMSC-провайдеры                | —                   |
| `routes`       | Правила маршрутизации          | —                   |
| `clients`      | API-клиенты                    | —                   |
| `audit_log`    | Аудит операций                 | По месяцам (created_at) |

#### Auth

| Таблица       | Назначение           |
|--------------|----------------------|
| `users`      | Пользователи системы |
| `roles`      | Роли                 |
| `permissions`| Разрешения           |
| `api_keys`   | API-ключи            |

#### Billing

| Таблица          | Назначение                |
|-----------------|---------------------------|
| `accounts`      | Счета клиентов (balance)  |
| `transactions`  | История транзакций        |
| `pricing_rules` | Правила тарификации       |

### 4.2 Ключевая таблица `messages`

```sql
id (uuid)                  -- PK (composite с created_at для партиционирования)
message_id (varchar)       -- Внешний идентификатор
external_id (varchar)      -- ID от внешней системы
source (varchar)           -- Адрес отправителя
destination (varchar)      -- Адрес получателя
text (text)                -- Текст сообщения
encoding (varchar)         -- GSM7 | UCS2 | ASCII
status (varchar)           -- pending | queued | sent | delivered | failed | expired | rejected
provider_id (uuid, FK)     -- Провайдер, через который отправлено
route_id (uuid, FK)        -- Использованный маршрут
client_id (uuid, FK)       -- Клиент-отправитель
retry_count (int)          -- Текущая попытка
max_retries (int)          -- Максимум попыток
next_retry_at (timestamp)  -- Время следующей попытки
smpp_message_id (varchar)  -- ID от SMSC
submitted_at (timestamp)   -- Время отправки в SMSC
delivered_at (timestamp)   -- Время доставки
failed_at (timestamp)      -- Время ошибки
created_at (timestamp)     -- Время создания
```

### 4.3 Индексирование

- Btree по `status`, `client_id`, `created_at`, `provider_id`, `active`, `priority`
- Composite: `(client_id, created_at)` для запросов истории
- Unique: `clients.api_key`, `users.username`, `users.email`, `api_keys.key_hash`

---

## 5. Потоки данных

### 5.1 Отправка SMS (HTTP/gRPC)

```
Client → HAProxy → Client Gateway → Auth Service (validate)
                                   → Messaging Service (create, status=pending)
                                   → Kafka [sms.outgoing]
                                   → Response {message_id, status: "queued"}
         Worker ← Kafka [sms.outgoing]
         Worker → Router (select provider)
         Worker → SMSC Pool (get connection)
         Worker → SMSC Provider (submit_sm PDU)
         Worker → DB (status=sent, save smpp_message_id)
```

### 5.2 Отправка SMS (SMPP)

```
SMPP Client → SMPP Gateway (bind, submit_sm)
              → Auth Service (validate system_id/password)
              → DB (status=pending)
              → Kafka [sms.outgoing]
              → submit_sm_resp {message_id}
              → далее аналогично п.5.1 (Worker обработка)
```

### 5.3 Delivery Receipt (DLR)

```
SMSC Provider → Worker (deliver_sm PDU)
               → Parse DLR (stat, err, timestamp)
               → Kafka [sms.dlr]
               → DB (status=delivered|failed|expired, save dlr_receipt)
               → SMPP Gateway → SMPP Client (deliver_sm, если подключён)
```

### 5.4 Обработка ошибок

```
Worker: submit_sm fails
  → RetryManager.ShouldRetry()?
    → Permanent error (invalid addr, auth) → Kafka [sms.failed] → DB (status=failed)
    → Transient error + retries < max → ScheduleRetry (exponential backoff) → Kafka [sms.outgoing]
    → Transient error + retries >= max → Kafka [sms.failed] → DB (status=failed)
```

---

## 6. Межсервисное взаимодействие

### 6.1 Синхронное (gRPC)

**Когда:** Gateway → Service, Service → Service для получения данных / валидации.

| Вызывающий       | Вызываемый       | Цель                          |
|-------------------|-------------------|-------------------------------|
| Client Gateway    | Auth Service      | Валидация API-ключа/JWT       |
| Client Gateway    | Messaging Service | Создание/статус сообщения     |
| Client Gateway    | Billing Service   | Баланс клиента                |
| Admin Gateway     | Client Service    | CRUD клиентов                 |
| Admin Gateway     | Provider Service  | CRUD провайдеров              |
| Admin Gateway     | Routing Service   | CRUD маршрутов                |
| Admin Gateway     | Analytics Service | Статистика и отчёты           |
| Messaging Service | Routing Service   | Определение маршрута          |
| Routing Service   | Provider Service  | Информация о провайдере       |

### 6.2 Асинхронное (Kafka)

| Топик                    | Publisher         | Consumer(s)                    |
|--------------------------|-------------------|--------------------------------|
| `sms.outgoing`           | Gateway / Retry   | Worker                         |
| `sms.dlr`                | Worker            | Messaging Service              |
| `sms.failed`             | Worker            | Analytics, Billing             |
| `sms.message.created`    | Messaging Service | Routing, Analytics             |
| `sms.message.routed`     | Routing Service   | Provider Service               |
| `sms.message.sent`       | Provider Service  | Analytics, Billing             |
| `sms.message.delivered`  | Provider Service  | Analytics, Billing             |
| `billing.balance.changed`| Billing Service   | Client Service (cache invalidation) |

### 6.3 Паттерны

- **Saga** — распределённые транзакции (Create → Route → Send → Charge) с компенсирующими действиями
- **CQRS** — разделение операций чтения и записи
- **Event Sourcing** (опционально) — хранение событий для восстановления состояния
- **Dead Letter Queue** — `sms.failed` для необработанных сообщений
- **Idempotency** — дедупликация по `external_id`
- **Backpressure** — rate limiting в consumer'ах

---

## 7. Конфигурация

### 7.1 Источники (по приоритету)

1. Переменные окружения (`SMPP_*` + Docker env: `POSTGRES_*`, `KAFKA_*`, `REDIS_*`)
2. YAML-файл (`./config.yaml`)
3. Значения по умолчанию в коде

### 7.2 Структура конфигурации

```yaml
service:
  name: smpp-server
  version: 1.0.0
  env: development          # development | staging | production

database:
  host: localhost
  port: 5432
  user: smpp
  password: change-me
  database: smpp_db
  max_open_conns: 25        # Макс. открытых соединений
  max_idle_conns: 5         # Макс. idle соединений
  conn_max_lifetime: 5m     # Время жизни соединения

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
  pool_size: 10

kafka:
  brokers: ["localhost:9092"]
  topic_outgoing: sms.outgoing
  topic_dlr: sms.dlr
  topic_failed: sms.failed
  consumer_group: smpp-worker

smpp:
  host: 0.0.0.0
  port: 2775
  read_timeout: 30s
  write_timeout: 30s
  enquire_link_period: 60s
  max_connections: 1000
  rate_limit_per_sec: 100

api:
  http:
    port: 8080
    read_timeout: 10s
    write_timeout: 10s
    idle_timeout: 120s
  grpc:
    port: 9090
    max_recv: 4194304       # 4MB
    max_send: 4194304
  auth:
    jwt_secret: <secret>
    token_expiry: 24h
    refresh_expiry: 168h
    api_key_header: X-API-Key

worker:
  concurrency: 10
  max_retries: 5
  retry_backoff_base: 1s
  retry_backoff_max: 60s
  batch_size: 100
  batch_timeout: 5s
  health_check_period: 30s

monitoring:
  enabled: true
  prometheus:
    enabled: true
    path: /metrics
  metrics_port: 2112
```

---

## 8. Развёртывание

### 8.1 Docker Compose

Описание в `deployments/docker-compose.yml`. Все сервисы контейнеризированы.

**Инстансы:**
| Сервис            | Кол-во | Образ                   |
|-------------------|--------|-------------------------|
| HAProxy           | 1      | haproxy:2.8             |
| Client Gateway    | 2      | client-gateway          |
| Admin Gateway     | 2      | admin-gateway           |
| Auth Service      | 1      | service-base (auth)     |
| Messaging Service | 1      | service-base (messaging)|
| Client Service    | 1      | service-base (client)   |
| Provider Service  | 1      | service-base (provider) |
| Routing Service   | 1      | service-base (routing)  |
| Analytics Service | 1      | service-base (analytics)|
| Billing Service   | 1      | service-base (billing)  |
| PostgreSQL        | 1      | postgres                |
| Redis             | 1      | redis                   |
| Kafka + Zookeeper | 1+1    | confluentinc/cp-kafka   |
| Prometheus        | 1      | prom/prometheus          |

### 8.2 Dockerfiles

- `client-gateway.Dockerfile` — multi-stage build (Go builder → alpine runtime)
- `admin-gateway.Dockerfile` — аналогично
- `smpp-gateway.Dockerfile` — аналогично
- `service-base.Dockerfile` — универсальный образ для доменных сервисов

### 8.3 Порты

| Порт  | Сервис                     |
|-------|----------------------------|
| 2775  | SMPP Gateway               |
| 8080  | Client Gateway HTTP        |
| 8081  | Admin Gateway HTTP         |
| 9090  | Client Gateway gRPC        |
| 9091  | Admin Gateway gRPC         |
| 9092  | Messaging Service gRPC     |
| 9093  | Routing Service gRPC       |
| 9094  | Provider Service gRPC      |
| 9095  | Client Service gRPC        |
| 9096  | Analytics Service gRPC     |
| 9097  | Billing Service gRPC       |
| 9101  | Auth Service gRPC          |
| 2112  | Prometheus metrics (все)   |

### 8.4 Масштабирование

| Компонент       | Горизонтальное масштабирование            |
|-----------------|-------------------------------------------|
| Client Gateway  | Несколько инстансов за HAProxy (stateless)|
| Admin Gateway   | Несколько инстансов за HAProxy (stateless)|
| SMPP Gateway    | Несколько инстансов (требует балансировки)|
| Worker          | Consumer group (автораспределение партиций)|
| Доменные сервисы| Несколько инстансов + gRPC load balancing |

---

## 9. Мониторинг и наблюдаемость

### 9.1 Health Checks

Каждый сервис предоставляет:
| Endpoint        | Назначение                                      |
|-----------------|--------------------------------------------------|
| `/health`       | Полная проверка (с зависимостями: DB, Redis, Kafka) |
| `/health/live`  | Liveness probe (сервис запущен)                  |
| `/health/ready` | Readiness probe (сервис может обрабатывать запросы) |

### 9.2 Prometheus Metrics

**HTTP/gRPC:**
- `http_requests_total{method, path, status}`
- `http_request_duration_seconds{method, path}`
- `grpc_requests_total{method, status}`
- `grpc_request_duration_seconds{method}`

**SMPP:**
- `smpp_messages_sent_total`, `smpp_messages_failed_total`
- `smpp_connections_active`
- `smpp_processing_duration_seconds`

**Worker:**
- `worker_messages_processed_total`
- `worker_processing_duration_seconds`

**Business:**
- `sms_messages_received_total`, `sms_messages_queued_total`
- `rate_limit_hits_total{client_id}`
- `smpp_provider_throughput`

### 9.3 Логирование

- Библиотека: zerolog (structured JSON)
- Уровни: debug, info, warn, error
- Контекстные поля: request_id, client_id, message_id, provider_id

---

## 10. Безопасность

| Аспект                  | Реализация                                     |
|-------------------------|-------------------------------------------------|
| Аутентификация клиентов | API-ключи (Client Gateway) / JWT (Admin Gateway)|
| Аутентификация SMPP     | system_id + password через Auth Service         |
| Авторизация             | Ролевая модель (RBAC): admin, client, operator  |
| Rate limiting           | Per-client (second/minute/hour), per-session (SMPP), Redis-backed |
| Сетевая изоляция        | Docker networks                                 |
| Шифрование              | SSL/TLS (опционально) для внешних соединений    |
| Аудит                   | Полный audit log (партиционированный)           |

---

## 11. Технологический стек

| Категория        | Технология                       |
|------------------|----------------------------------|
| Язык             | Go 1.24                         |
| Протоколы        | SMPP v3.4, HTTP/1.1, gRPC       |
| HTTP Router      | gorilla/mux                      |
| gRPC             | google.golang.org/grpc           |
| Serialization    | Protocol Buffers, JSON           |
| БД               | PostgreSQL (pgx/v5, sqlx)        |
| Кэш              | Redis (go-redis/v9)              |
| Очередь          | Apache Kafka (Sarama v1.43)      |
| Конфигурация     | Viper (YAML + env vars)          |
| Логирование      | zerolog                          |
| Мониторинг       | Prometheus client_golang         |
| Тестирование     | testify                          |
| Контейнеризация  | Docker, Docker Compose           |
| Load Balancing   | HAProxy 2.8                      |

---

## 12. Структура кодовой базы

```
cmd/
├── api/                         # Legacy API Gateway
├── client-gateway/              # Client Gateway (HTTP + gRPC)
├── admin-gateway/               # Admin Gateway (HTTP + gRPC)
├── smpp-gateway/                # SMPP Gateway
├── worker/                      # Kafka consumer + SMPP sender
└── services/
    ├── auth-service/            # Auth domain service
    ├── messaging-service/       # Messaging domain service
    ├── routing-service/         # Routing domain service
    ├── provider-service/        # Provider domain service
    ├── client-service/          # Client domain service
    ├── analytics-service/       # Analytics domain service
    └── billing-service/         # Billing domain service

internal/
├── config/                      # Конфигурация (Viper)
├── shared/                      # Общие модели, ошибки, логирование
├── storage/                     # Репозитории (PostgreSQL)
│   ├── db.go                   # Подключение к БД
│   ├── client_repository.go
│   ├── message_repository.go
│   ├── provider_repository.go
│   └── route_repository.go
├── queue/                       # Kafka producer/consumer
│   ├── producer.go
│   ├── consumer.go
│   └── messages.go             # Структуры сообщений
├── router/                      # Маршрутизация и retry
│   ├── router.go
│   └── retry.go
├── smsc/                        # SMPP connection pool
│   ├── pool.go                 # Пул соединений, throttler
│   └── sender.go               # Отправка submit_sm
├── monitoring/                  # Health checks, Prometheus metrics
├── gateway/
│   ├── client/                  # Client Gateway implementation
│   │   ├── handlers/           # HTTP handlers
│   │   ├── grpc/               # gRPC server
│   │   ├── middleware/         # Auth, CORS, logging, recovery
│   │   └── clients.go          # gRPC service clients
│   ├── admin/                   # Admin Gateway implementation
│   └── smpp/                    # SMPP Gateway implementation
│       ├── server/             # TCP server, PDU handler
│       └── session/            # Session management, rate limiter
├── api/                         # Legacy API handlers
│   ├── http/                   # HTTP handlers
│   ├── grpc/                   # gRPC interceptors
│   └── middleware/             # Rate limiting, auth, CORS
└── services/                    # Domain service implementations
    ├── auth/
    │   ├── application/        # Token service, business logic
    │   └── grpc/               # gRPC server
    ├── messaging/
    ├── routing/
    ├── provider/
    ├── client/
    ├── analytics/
    └── billing/

api/proto/                       # Protocol Buffer definitions
├── sms.proto                   # SMS service (legacy)
├── auth/auth.proto
├── messaging/messaging.proto
├── routing/routing.proto
├── provider/provider.proto
├── client/client.proto
├── analytics/analytics.proto
└── billing/billing.proto

migrations/                      # PostgreSQL migrations (000001-000007)
configs/                         # Configuration examples
deployments/                     # Docker, docker-compose, HAProxy
```

---

## 13. DDD-домены и границы контекстов

| Домен          | Bounded Context        | Сервис            | Хранилище                        |
|----------------|------------------------|-------------------|----------------------------------|
| Authentication | Пользователи и доступ  | Auth Service      | users, roles, permissions, api_keys |
| Messaging      | Жизненный цикл SMS     | Messaging Service | messages, dlr_receipts           |
| Routing        | Маршрутизация          | Routing Service   | routes                           |
| Provider Mgmt  | Управление провайдерами| Provider Service  | providers                        |
| Client Mgmt    | Управление клиентами   | Client Service    | clients                          |
| Analytics      | Статистика и отчёты    | Analytics Service | message_stats, aggregated_metrics|
| Billing        | Тарификация и баланс   | Billing Service   | accounts, transactions, pricing_rules |

**Текущее ограничение:** Все домены используют общую БД PostgreSQL. В документации отмечена возможность разделения на отдельные БД в будущем.

**Межконтекстное взаимодействие:**
- gRPC — синхронные запросы между сервисами
- Kafka — асинхронные события для уведомлений и обработки

---

## 14. Ограничения и известные особенности

1. **Общая БД** — все сервисы подключены к одному PostgreSQL; нет data isolation между доменами
2. **SMPP Gateway** — по умолчанию один инстанс; масштабирование требует дополнительной настройки балансировки TCP-соединений
3. **SSL/TLS** — опциональный, не включён по умолчанию
4. **Directus** — интеграция с CMS Directus для администрирования (миграция 000007); степень интеграции ограничена
5. **StreamDLR** — streaming DLR через gRPC отмечен как будущая функциональность
6. **Кодировки** — поддержка GSM7, UCS2, ASCII; обработка multipart (UDH) через SMPP
