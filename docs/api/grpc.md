# gRPC API

## Обзор

API Gateway предоставляет gRPC API для отправки SMS сообщений. gRPC API использует Protocol Buffers для сериализации данных и обеспечивает высокую производительность и типобезопасность.

## Протобуф определение

Протобуф файл: `api/proto/sms.proto`

## Генерация кода

Перед использованием необходимо сгенерировать Go код из proto файлов:

### Windows (PowerShell)

```powershell
.\scripts\generate-proto.ps1
```

### Linux/Mac (Bash)

```bash
chmod +x scripts/generate-proto.sh
./scripts/generate-proto.sh
```

## Сервис SMSService

### SendSMS

Отправка одного SMS сообщения.

**Запрос:**

```protobuf
message SendSMSRequest {
  string source = 1;              // Отправитель
  string destination = 2;         // Получатель
  string text = 3;                // Текст сообщения
  string external_id = 4;         // Внешний ID (опционально)
  int32 priority = 5;             // Приоритет (0-3)
  bool registered_delivery = 6;    // Требовать DLR
  google.protobuf.Timestamp validity_period = 7; // Срок действия (опционально)
  string service_type = 8;         // Тип сервиса (опционально)
  int32 source_addr_ton = 9;       // TON отправителя (опционально)
  int32 source_addr_npi = 10;      // NPI отправителя (опционально)
  int32 dest_addr_ton = 11;        // TON получателя (опционально)
  int32 dest_addr_npi = 12;        // NPI получателя (опционально)
  int32 data_coding = 13;          // Кодировка данных (опционально)
}
```

**Ответ:**

```protobuf
message SendSMSResponse {
  string message_id = 1;          // ID сообщения
  string status = 2;               // Статус (queued, sent, failed)
  string error = 3;                // Ошибка (если есть)
}
```

**Пример использования (Go):**

```go
import (
    "context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    smsv1 "github.com/smpp-server/smpp-server/api/proto/smsv1"
)

conn, err := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

client := smsv1.NewSMSServiceClient(conn)

req := &smsv1.SendSMSRequest{
    Source:      "12345",
    Destination: "79001234567",
    Text:        "Текст сообщения",
    Priority:     1,
    RegisteredDelivery: true,
}

resp, err := client.SendSMS(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Message ID: %s\n", resp.MessageId)
fmt.Printf("Status: %s\n", resp.Status)
```

**Пример использования (Python):**

```python
import grpc
from api.proto import sms_pb2, sms_pb2_grpc

channel = grpc.insecure_channel('localhost:9090')
stub = sms_pb2_grpc.SMSServiceStub(channel)

request = sms_pb2.SendSMSRequest(
    source="12345",
    destination="79001234567",
    text="Текст сообщения",
    priority=1,
    registered_delivery=True
)

response = stub.SendSMS(request)
print(f"Message ID: {response.message_id}")
print(f"Status: {response.status}")
```

### SendBatchSMS

Пакетная отправка SMS сообщений.

**Запрос:**

```protobuf
message SendBatchRequest {
  repeated SendSMSRequest messages = 1; // Список сообщений
}
```

**Ответ:**

```protobuf
message SendBatchResponse {
  repeated SendSMSResponse results = 1; // Результаты для каждого сообщения
  int32 success_count = 2;              // Количество успешных
  int32 failed_count = 3;                // Количество неудачных
}
```

**Пример использования (Go):**

```go
req := &smsv1.SendBatchRequest{
    Messages: []*smsv1.SendSMSRequest{
        {
            Source:      "12345",
            Destination: "79001234567",
            Text:        "Сообщение 1",
        },
        {
            Source:      "12345",
            Destination: "79001234568",
            Text:        "Сообщение 2",
        },
    },
}

resp, err := client.SendBatchSMS(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Success: %d, Failed: %d\n", resp.SuccessCount, resp.FailedCount)
for i, result := range resp.Results {
    fmt.Printf("Message %d: ID=%s, Status=%s\n", i+1, result.MessageId, result.Status)
}
```

### GetStatus

Получение статуса сообщения по ID.

**Запрос:**

```protobuf
message GetStatusRequest {
  string message_id = 1;           // ID сообщения
}
```

**Ответ:**

```protobuf
message GetStatusResponse {
  string message_id = 1;           // ID сообщения
  string status = 2;               // Статус (pending, queued, sent, delivered, failed, expired, rejected)
  string status_message = 3;       // Сообщение о статусе
  google.protobuf.Timestamp created_at = 4;    // Время создания
  google.protobuf.Timestamp submitted_at = 5;  // Время отправки (опционально)
  google.protobuf.Timestamp delivered_at = 6;   // Время доставки (опционально)
  google.protobuf.Timestamp failed_at = 7;      // Время ошибки (опционально)
  string smpp_message_id = 8;      // SMPP message ID (опционально)
}
```

**Пример использования (Go):**

```go
req := &smsv1.GetStatusRequest{
    MessageId: "550e8400-e29b-41d4-a716-446655440000",
}

resp, err := client.GetStatus(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Status: %s\n", resp.Status)
if resp.DeliveredAt != nil {
    fmt.Printf("Delivered at: %v\n", resp.DeliveredAt.AsTime())
}
```

### StreamDLR

Поток delivery receipts (будущая функциональность).

**Запрос:**

```protobuf
message StreamDLRRequest {
  repeated string message_ids = 1; // ID сообщений для отслеживания (пусто = все)
}
```

**Ответ (stream):**

```protobuf
message DLRUpdate {
  string message_id = 1;           // ID сообщения
  string smpp_message_id = 2;     // SMPP message ID
  string stat = 3;                 // Статус доставки (DELIVRD, UNDELIV, EXPIRED, etc.)
  google.protobuf.Timestamp done_date = 4; // Время доставки
  int32 err = 5;                   // Код ошибки (опционально)
  string text = 6;                 // Текст ошибки (опционально)
}
```

## Аутентификация

gRPC API использует metadata для передачи API ключа:

**Go:**

```go
import "google.golang.org/grpc/metadata"

md := metadata.New(map[string]string{
    "x-api-key": "your-api-key",
})
ctx := metadata.NewOutgoingContext(context.Background(), md)

resp, err := client.SendSMS(ctx, req)
```

**Python:**

```python
import grpc

metadata = [('x-api-key', 'your-api-key')]
response = stub.SendSMS(request, metadata=metadata)
```

## Обработка ошибок

gRPC использует статус коды для ошибок:

- `OK` (0) - успех
- `INVALID_ARGUMENT` (3) - неверные аргументы
- `UNAUTHENTICATED` (16) - не авторизован
- `RESOURCE_EXHAUSTED` (8) - превышен лимит запросов
- `INTERNAL` (13) - внутренняя ошибка сервера
- `NOT_FOUND` (5) - ресурс не найден

**Пример обработки ошибок (Go):**

```go
import (
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/codes"
)

resp, err := client.SendSMS(ctx, req)
if err != nil {
    st, ok := status.FromError(err)
    if ok {
        switch st.Code() {
        case codes.InvalidArgument:
            fmt.Println("Invalid request:", st.Message())
        case codes.Unauthenticated:
            fmt.Println("Authentication failed")
        case codes.ResourceExhausted:
            fmt.Println("Rate limit exceeded")
        default:
            fmt.Println("Error:", st.Message())
        }
    } else {
        fmt.Println("Error:", err)
    }
    return
}
```

## Rate Limiting

Rate limiting применяется так же, как и для HTTP API. При превышении лимита возвращается ошибка `RESOURCE_EXHAUSTED`.

## Метрики

gRPC API экспортирует метрики Prometheus:

- `grpc_requests_total` - общее количество gRPC запросов (метки: method, status)
- `grpc_request_duration_seconds` - длительность gRPC запросов (метки: method)

## Конфигурация

Настройки gRPC сервера в конфигурации:

```yaml
api:
  grpc:
    host: 0.0.0.0
    port: 9090
    max_recv: 4194304  # 4MB
    max_send: 4194304  # 4MB
```

## Запуск

gRPC сервер запускается вместе с HTTP сервером в API Gateway:

```bash
go run cmd/api/main.go
```

Или через Docker:

```bash
docker-compose -f deployments/docker-compose.yml up api-gateway-1
```

## Дополнительная документация

- [API Gateway](api-gateway.md) - общая документация API Gateway
- [SMPP протокол](../development/smpp-protocol.md) - документация SMPP протокола