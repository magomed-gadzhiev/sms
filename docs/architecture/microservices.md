# Описание микросервисов

## API Gateway

### Обзор

API Gateway является единой точкой входа для клиентов, предоставляя HTTP REST API и gRPC API для отправки SMS сообщений.

### Технологии

- **Язык:** Go
- **HTTP фреймворк:** Gin
- **gRPC:** стандартная библиотека Go
- **База данных:** PostgreSQL (через pgx)
- **Очередь:** Kafka (через sarama)
- **Кэш:** Redis (через go-redis)

### Основные компоненты

#### HTTP Handlers (`internal/api/http/`)

- `POST /api/v1/sms/send` - отправка одного SMS
- `POST /api/v1/sms/batch` - пакетная отправка SMS
- `GET /api/v1/sms/status` - получение статуса сообщения
- `GET /api/v1/sms/history` - история сообщений
- `GET /health` - health check
- `GET /metrics` - Prometheus метрики

#### gRPC Handlers (`internal/api/grpc/`)

- `SendSMS` - отправка одного SMS
- `SendBatchSMS` - пакетная отправка SMS
- `GetStatus` - получение статуса сообщения
- `StreamDLR` - поток delivery receipts (будущая функциональность)

#### Middleware (`internal/api/middleware/`)

- **Authentication** - проверка API ключей
- **Rate Limiting** - ограничение частоты запросов
- **Logging** - структурированное логирование
- **Recovery** - обработка паник
- **CORS** - поддержка CORS

### Конфигурация

```yaml
api:
  http:
    host: 0.0.0.0
    port: 8080
    read_timeout: 10s
    write_timeout: 10s
    idle_timeout: 120s
  grpc:
    host: 0.0.0.0
    port: 9090
    max_recv: 4194304  # 4MB
    max_send: 4194304  # 4MB
  auth:
    api_key_header: X-API-Key
```

### Масштабирование

- Множественные инстансы за HAProxy
- Stateless сервис (не хранит состояние)
- Балансировка через round-robin

### Метрики

- `http_requests_total` - количество HTTP запросов
- `http_request_duration_seconds` - длительность HTTP запросов
- `grpc_requests_total` - количество gRPC запросов
- `grpc_request_duration_seconds` - длительность gRPC запросов
- `sms_messages_received_total` - полученные SMS
- `sms_messages_queued_total` - сообщения в очереди
- `rate_limit_hits_total` - срабатывания rate limit

## SMPP Server

### Обзор

SMPP Server принимает входящие SMPP соединения от клиентов и обрабатывает SMPP протокол версии 3.4.

### Технологии

- **Язык:** Go
- **Протокол:** SMPP v3.4
- **База данных:** PostgreSQL
- **Очередь:** Kafka

### Основные компоненты

#### SMPP Protocol (`internal/smpp/protocol/`)

- Структуры PDU для всех команд SMPP
- Кодирование/декодирование PDU
- Валидация PDU

#### SMPP Server (`internal/smpp/server/`)

- TCP сервер для входящих соединений
- Управление сессиями (bind, unbind)
- Обработка submit_sm
- Отправка deliver_sm (DLR)
- Enquire link для keep-alive
- Rate limiting на уровне сессии

### Поддерживаемые команды

- `bind_receiver` / `bind_receiver_resp`
- `bind_transmitter` / `bind_transmitter_resp`
- `bind_transceiver` / `bind_transceiver_resp`
- `unbind` / `unbind_resp`
- `submit_sm` / `submit_sm_resp`
- `deliver_sm` / `deliver_sm_resp`
- `enquire_link` / `enquire_link_resp`
- `query_sm` / `query_sm_resp`

### Конфигурация

```yaml
smpp:
  host: 0.0.0.0
  port: 2775
  read_timeout: 30s
  write_timeout: 30s
  enquire_link_period: 60s
  max_connections: 1000
  rate_limit_per_sec: 100
```

### Масштабирование

- Один инстанс по умолчанию
- Можно масштабировать при необходимости (требует настройки балансировки)

### Метрики

- `smpp_messages_received_total` - полученные SMS через SMPP
- `smpp_connections_active` - активные SMPP соединения
- `smpp_processing_duration_seconds` - длительность обработки
- `smpp_messages_failed_total` - неудачные сообщения

## Worker

### Обзор

Worker обрабатывает очередь сообщений из Kafka и отправляет их в SMSC провайдеры через SMPP протокол.

### Технологии

- **Язык:** Go
- **Очередь:** Kafka (через sarama)
- **База данных:** PostgreSQL
- **Кэш:** Redis
- **Протокол:** SMPP v3.4

### Основные компоненты

#### Kafka Consumer (`internal/queue/consumer.go`)

- Потребление сообщений из `sms.outgoing`
- Потребление DLR из `sms.dlr`
- Потребление failed сообщений из `sms.failed`
- Consumer groups для масштабирования

#### SMSC Connection Pool (`internal/smsc/pool.go`)

- Пул SMPP соединений к провайдерам
- Health monitoring соединений
- Автоматическое переподключение
- Load balancing между провайдерами
- Throttling на уровне провайдера

#### Message Router (`internal/router/router.go`)

- Маршрутизация по правилам (префикс номера, провайдер)
- Failover на резервные провайдеры
- Load balancing стратегии

#### Retry Manager (`internal/router/retry.go`)

- Экспоненциальная задержка
- Максимум 5 попыток
- DLQ для permanent failures

### Конфигурация

```yaml
worker:
  concurrency: 10
  max_retries: 5
  retry_backoff_base: 1s
  retry_backoff_max: 60s
  batch_size: 100
  batch_timeout: 5s
  health_check_period: 30s
```

### Масштабирование

- Множественные инстансы в одном consumer group
- Автоматическое распределение партиций между инстансами
- Увеличение concurrency для обработки большего количества сообщений

### Метрики

- `worker_messages_processed_total` - обработанные сообщения
- `worker_processing_duration_seconds` - длительность обработки
- `smpp_messages_sent_total` - отправленные сообщения
- `smpp_messages_failed_total` - неудачные сообщения
- `smpp_provider_throughput` - пропускная способность провайдера

## Общие компоненты

### Конфигурация (`internal/config/`)

- Загрузка из YAML файлов и переменных окружения
- Валидация конфигурации
- Значения по умолчанию

### Логирование (`internal/shared/logger.go`)

- Структурированное логирование через zerolog
- Уровни логирования (debug, info, warn, error)
- Контекстные поля

### Ошибки (`internal/shared/errors.go`)

- Типизированные ошибки приложения
- Коды ошибок для клиентов
- Логирование ошибок

### Мониторинг (`internal/monitoring/`)

- Prometheus метрики
- Health checks
- Интеграция с Prometheus

### Хранилище (`internal/storage/`)

- Репозитории для работы с БД
- Message repository
- Provider repository
- Client repository
- Route repository

## Взаимодействие между сервисами

### API Gateway → Kafka

- Публикация исходящих сообщений в `sms.outgoing`
- Асинхронная обработка (не блокирует ответ клиенту)

### SMPP Server → Kafka

- Публикация исходящих сообщений в `sms.outgoing`
- Публикация DLR в `sms.dlr`

### Kafka → Worker

- Потребление сообщений из `sms.outgoing`
- Потребление DLR из `sms.dlr`
- Потребление failed сообщений из `sms.failed`

### Worker → SMSC Providers

- SMPP соединения к провайдерам
- Отправка submit_sm
- Получение deliver_sm (DLR)

### Worker → PostgreSQL

- Обновление статусов сообщений
- Сохранение DLR
- Аудит операций

### Worker → Redis

- Кэширование данных провайдеров
- Rate limiting счетчики

## Зависимости

### API Gateway зависит от:

- PostgreSQL (чтение/запись сообщений и клиентов)
- Redis (кэш и rate limiting)
- Kafka (публикация сообщений)

### SMPP Server зависит от:

- PostgreSQL (чтение/запись сообщений)
- Kafka (публикация сообщений)

### Worker зависит от:

- PostgreSQL (чтение/запись сообщений и провайдеров)
- Redis (кэш)
- Kafka (потребление сообщений)
- SMSC Providers (внешние SMPP соединения)

## Дополнительная документация

- [Обзор архитектуры](overview.md)
- [Потоки данных](data-flow.md)
- [API Gateway документация](../api/api-gateway.md)
- [SMPP протокол](../development/smpp-protocol.md)