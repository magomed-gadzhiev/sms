# API Gateway

API Gateway предоставляет HTTP и gRPC API для отправки SMS сообщений.

## Генерация proto кода

Перед сборкой проекта необходимо сгенерировать код из proto файлов:

### Windows (PowerShell)

```powershell
.\scripts\generate-proto.ps1
```

### Linux/Mac (Bash)

```bash
chmod +x scripts/generate-proto.sh
./scripts/generate-proto.sh
```

### Требования

- `protoc` - Protocol Buffers compiler
- `protoc-gen-go` - Go plugin для protoc
- `protoc-gen-go-grpc` - gRPC Go plugin для protoc

Установка плагинов:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

## HTTP API

### Endpoints

#### POST /api/v1/sms/send

Отправка одного SMS сообщения.

**Заголовки:**
- `X-API-Key`: API ключ клиента
- `Content-Type`: application/json

**Тело запроса:**
```json
{
  "source": "12345",
  "destination": "79001234567",
  "text": "Текст сообщения",
  "external_id": "optional-external-id",
  "priority": 0,
  "registered_delivery": true,
  "validity_period": "2024-12-31T23:59:59Z",
  "service_type": "",
  "source_addr_ton": 0,
  "source_addr_npi": 0,
  "dest_addr_ton": 0,
  "dest_addr_npi": 0,
  "data_coding": 0
}
```

**Ответ:**
```json
{
  "message_id": "uuid",
  "status": "queued"
}
```

#### POST /api/v1/sms/batch

Пакетная отправка SMS сообщений.

**Тело запроса:**
```json
{
  "messages": [
    {
      "source": "12345",
      "destination": "79001234567",
      "text": "Сообщение 1"
    },
    {
      "source": "12345",
      "destination": "79001234568",
      "text": "Сообщение 2"
    }
  ]
}
```

**Ответ:**
```json
{
  "results": [
    {
      "message_id": "uuid1",
      "status": "queued"
    },
    {
      "message_id": "uuid2",
      "status": "queued"
    }
  ],
  "success_count": 2,
  "failed_count": 0
}
```

#### GET /api/v1/sms/status?id={message_id}

Получение статуса сообщения.

**Ответ:**
```json
{
  "message_id": "uuid",
  "status": "delivered",
  "status_message": "",
  "created_at": "2024-01-01T12:00:00Z",
  "submitted_at": "2024-01-01T12:00:01Z",
  "delivered_at": "2024-01-01T12:00:05Z",
  "smpp_message_id": "smpp-id"
}
```

#### GET /api/v1/sms/history?limit=100&offset=0&status=sent

Получение истории сообщений.

**Параметры запроса:**
- `limit` (опционально): количество сообщений (по умолчанию 100, максимум 1000)
- `offset` (опционально): смещение (по умолчанию 0)
- `status` (опционально): фильтр по статусу

**Ответ:**
```json
{
  "messages": [
    {
      "message_id": "uuid",
      "status": "sent",
      "created_at": "2024-01-01T12:00:00Z"
    }
  ],
  "limit": 100,
  "offset": 0,
  "count": 1
}
```

#### GET /health

Health check endpoint (без аутентификации).

**Ответ:**
```json
{
  "status": "ok",
  "service": "api-gateway"
}
```

#### GET /metrics

Prometheus metrics endpoint (без аутентификации).

## gRPC API

### Сервис SMSService

#### SendSMS

Отправка одного SMS сообщения.

```protobuf
rpc SendSMS(SendSMSRequest) returns (SendSMSResponse);
```

#### SendBatchSMS

Пакетная отправка SMS сообщений.

```protobuf
rpc SendBatchSMS(SendBatchRequest) returns (SendBatchResponse);
```

#### GetStatus

Получение статуса сообщения.

```protobuf
rpc GetStatus(GetStatusRequest) returns (GetStatusResponse);
```

#### StreamDLR

Поток delivery receipts (пока не реализован).

```protobuf
rpc StreamDLR(StreamDLRRequest) returns (stream DLRUpdate);
```

## Аутентификация

API Gateway использует аутентификацию по API ключу. Ключ передается в заголовке `X-API-Key` или в `Authorization: Bearer <key>`.

## Rate Limiting

Rate limiting настраивается для каждого клиента отдельно:
- `rate_limit_per_second` - лимит запросов в секунду
- `rate_limit_per_minute` - лимит запросов в минуту
- `rate_limit_per_hour` - лимит запросов в час

При превышении лимита возвращается HTTP 429 (Too Many Requests).

## Middleware

API Gateway использует следующие middleware:

1. **Recovery** - восстановление после паник
2. **Logging** - логирование запросов
3. **CORS** - поддержка CORS (настраивается через переменную окружения `CORS_ALLOWED_ORIGINS`)
4. **Authentication** - аутентификация по API ключу
5. **Rate Limiting** - ограничение частоты запросов

## Мониторинг

API Gateway экспортирует Prometheus метрики на endpoint `/metrics`:

- `http_requests_total` - общее количество HTTP запросов
- `http_request_duration_seconds` - длительность HTTP запросов
- `grpc_requests_total` - общее количество gRPC запросов
- `grpc_request_duration_seconds` - длительность gRPC запросов
- `sms_messages_received_total` - количество полученных SMS
- `sms_messages_queued_total` - количество сообщений в очереди
- `sms_messages_failed_total` - количество неудачных сообщений
- `kafka_publish_duration_seconds` - длительность публикации в Kafka
- `kafka_publish_errors_total` - ошибки публикации в Kafka
- `rate_limit_hits_total` - срабатывания rate limit

## Конфигурация

Конфигурация API Gateway настраивается через файл `config.yaml` или переменные окружения:

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
    jwt_secret: ""
    token_expiry: 24h
```

## Запуск

```bash
go run cmd/api/main.go
```

Или через Docker:

```bash
docker-compose up api-gateway-1
```
