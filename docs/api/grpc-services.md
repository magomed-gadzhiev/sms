# gRPC Сервисы

## Обзор

Система предоставляет несколько gRPC сервисов для взаимодействия между gateway и микросервисами. Все сервисы используют Protocol Buffers v3 для определения API.

**Базовый порт:** Зависит от сервиса (см. ниже)

## Генерация кода

Перед использованием необходимо сгенерировать Go код из proto файлов:

```bash
# Windows PowerShell
.\scripts\generate-proto.ps1

# Linux/Mac
./scripts/generate-proto.sh
```

## Сервисы

### 1. Auth Service

**Порт:** Зависит от развертывания (по умолчанию 50051)

**Файл:** `api/proto/auth/auth.proto`

#### Методы

**Authenticate** - аутентификация пользователя
```protobuf
rpc Authenticate(AuthenticateRequest) returns (AuthenticateResponse);
```

**ValidateToken** - валидация токена
```protobuf
rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);
```

**RefreshToken** - обновление токена
```protobuf
rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse);
```

**GetPermissions** - получение прав доступа
```protobuf
rpc GetPermissions(GetPermissionsRequest) returns (GetPermissionsResponse);
```

**CreateAPIKey** - создание API ключа
```protobuf
rpc CreateAPIKey(CreateAPIKeyRequest) returns (CreateAPIKeyResponse);
```

**RevokeAPIKey** - отзыв API ключа
```protobuf
rpc RevokeAPIKey(RevokeAPIKeyRequest) returns (RevokeAPIKeyResponse);
```

**ListAPIKeys** - список API ключей
```protobuf
rpc ListAPIKeys(ListAPIKeysRequest) returns (ListAPIKeysResponse);
```

### 2. Messaging Service

**Порт:** Зависит от развертывания (по умолчанию 50052)

**Файл:** `api/proto/messaging/messaging.proto`

#### Методы

**SendMessage** - отправка одного SMS сообщения
```protobuf
rpc SendMessage(SendMessageRequest) returns (SendMessageResponse);
```

**SendBatch** - пакетная отправка SMS
```protobuf
rpc SendBatch(SendBatchRequest) returns (SendBatchResponse);
```

**GetMessageStatus** - получение статуса сообщения
```protobuf
rpc GetMessageStatus(GetMessageStatusRequest) returns (GetMessageStatusResponse);
```

**GetMessageHistory** - получение истории сообщений
```protobuf
rpc GetMessageHistory(GetMessageHistoryRequest) returns (GetMessageHistoryResponse);
```

**ProcessDLR** - обработка delivery receipt
```protobuf
rpc ProcessDLR(ProcessDLRRequest) returns (ProcessDLRResponse);
```

### 3. Routing Service

**Порт:** Зависит от развертывания (по умолчанию 50053)

**Файл:** `api/proto/routing/routing.proto`

#### Методы

**GetRoute** - получение маршрута для сообщения
```protobuf
rpc GetRoute(GetRouteRequest) returns (GetRouteResponse);
```

**SelectProvider** - выбор оптимального провайдера
```protobuf
rpc SelectProvider(SelectProviderRequest) returns (SelectProviderResponse);
```

**CreateRoute** - создание правила маршрутизации
```protobuf
rpc CreateRoute(CreateRouteRequest) returns (CreateRouteResponse);
```

**UpdateRoute** - обновление правила маршрутизации
```protobuf
rpc UpdateRoute(UpdateRouteRequest) returns (UpdateRouteResponse);
```

**DeleteRoute** - удаление правила маршрутизации
```protobuf
rpc DeleteRoute(DeleteRouteRequest) returns (DeleteRouteResponse);
```

**ListRoutes** - список правил маршрутизации
```protobuf
rpc ListRoutes(ListRoutesRequest) returns (ListRoutesResponse);
```

### 4. Provider Service

**Порт:** Зависит от развертывания (по умолчанию 50054)

**Файл:** `api/proto/provider/provider.proto`

#### Методы

**CreateProvider** - создание провайдера
```protobuf
rpc CreateProvider(CreateProviderRequest) returns (CreateProviderResponse);
```

**UpdateProvider** - обновление провайдера
```protobuf
rpc UpdateProvider(UpdateProviderRequest) returns (UpdateProviderResponse);
```

**GetProvider** - получение информации о провайдере
```protobuf
rpc GetProvider(GetProviderRequest) returns (GetProviderResponse);
```

**ListProviders** - список провайдеров
```protobuf
rpc ListProviders(ListProvidersRequest) returns (ListProvidersResponse);
```

**DeleteProvider** - удаление провайдера
```protobuf
rpc DeleteProvider(DeleteProviderRequest) returns (DeleteProviderRequest);
```

**GetProviderHealth** - получение статуса здоровья провайдера
```protobuf
rpc GetProviderHealth(GetProviderHealthRequest) returns (GetProviderHealthResponse);
```

**SendToProvider** - отправка сообщения через провайдера
```protobuf
rpc SendToProvider(SendToProviderRequest) returns (SendToProviderResponse);
```

### 5. Client Service

**Порт:** Зависит от развертывания (по умолчанию 50055)

**Файл:** `api/proto/client/client.proto`

#### Методы

**CreateClient** - создание клиента
```protobuf
rpc CreateClient(CreateClientRequest) returns (CreateClientResponse);
```

**UpdateClient** - обновление клиента
```protobuf
rpc UpdateClient(UpdateClientRequest) returns (UpdateClientResponse);
```

**GetClient** - получение информации о клиенте
```protobuf
rpc GetClient(GetClientRequest) returns (GetClientResponse);
```

**ListClients** - список клиентов
```protobuf
rpc ListClients(ListClientsRequest) returns (ListClientsResponse);
```

**DeleteClient** - удаление клиента
```protobuf
rpc DeleteClient(DeleteClientRequest) returns (DeleteClientResponse);
```

**GetClientConfig** - получение конфигурации клиента
```protobuf
rpc GetClientConfig(GetClientConfigRequest) returns (GetClientConfigResponse);
```

**UpdateClientConfig** - обновление конфигурации клиента
```protobuf
rpc UpdateClientConfig(UpdateClientConfigRequest) returns (UpdateClientConfigResponse);
```

**UpdateClientRateLimits** - обновление rate limits клиента
```protobuf
rpc UpdateClientRateLimits(UpdateClientRateLimitsRequest) returns (UpdateClientRateLimitsResponse);
```

### 6. Analytics Service

**Порт:** Зависит от развертывания (по умолчанию 50056)

**Файл:** `api/proto/analytics/analytics.proto`

#### Методы

**GetStatistics** - получение статистики
```protobuf
rpc GetStatistics(GetStatisticsRequest) returns (GetStatisticsResponse);
```

**GenerateReport** - генерация отчета
```protobuf
rpc GenerateReport(GenerateReportRequest) returns (GenerateReportResponse);
```

**GetRealtimeMetrics** - получение метрик в реальном времени
```protobuf
rpc GetRealtimeMetrics(GetRealtimeMetricsRequest) returns (GetRealtimeMetricsResponse);
```

**GetProviderPerformance** - получение производительности провайдера
```protobuf
rpc GetProviderPerformance(GetProviderPerformanceRequest) returns (GetProviderPerformanceResponse);
```

### 7. Billing Service

**Порт:** Зависит от развертывания (по умолчанию 50057)

**Файл:** `api/proto/billing/billing.proto`

#### Методы

**GetBalance** - получение баланса клиента
```protobuf
rpc GetBalance(GetBalanceRequest) returns (GetBalanceResponse);
```

**ChargeMessage** - списание средств за сообщение
```protobuf
rpc ChargeMessage(ChargeMessageRequest) returns (ChargeMessageResponse);
```

**AddCredits** - добавление средств на счет
```protobuf
rpc AddCredits(AddCreditsRequest) returns (AddCreditsResponse);
```

**DeductCredits** - списание средств со счета
```protobuf
rpc DeductCredits(DeductCreditsRequest) returns (DeductCreditsResponse);
```

**GetTransactionHistory** - получение истории транзакций
```protobuf
rpc GetTransactionHistory(GetTransactionHistoryRequest) returns (GetTransactionHistoryResponse);
```

**GetPricingRules** - получение правил тарификации
```protobuf
rpc GetPricingRules(GetPricingRulesRequest) returns (GetPricingRulesResponse);
```

**CreatePricingRule** - создание правила тарификации
```protobuf
rpc CreatePricingRule(CreatePricingRuleRequest) returns (CreatePricingRuleResponse);
```

## Примеры использования (Go)

### Создание клиента

```go
import (
    "context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    messagingpb "github.com/smpp-server/smpp-server/api/proto/messagingv1"
)

// Создание соединения
conn, err := grpc.Dial(
    "localhost:50052",
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// Создание клиента
client := messagingpb.NewMessagingServiceClient(conn)

// Отправка сообщения
req := &messagingpb.SendMessageRequest{
    ClientId:   "550e8400-e29b-41d4-a716-446655440000",
    Source:     "12345",
    Destination: "79001234567",
    Text:       "Test message",
}

resp, err := client.SendMessage(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Message ID: %s\n", resp.MessageId)
```

### Валидация токена

```go
import (
    authpb "github.com/smpp-server/smpp-server/api/proto/authv1"
)

authClient := authpb.NewAuthServiceClient(conn)

req := &authpb.ValidateTokenRequest{
    Token: "jwt-token-here",
}

resp, err := authClient.ValidateToken(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

if resp.Valid {
    fmt.Printf("User: %s\n", resp.User.Username)
}
```

### Получение маршрута

```go
import (
    routingpb "github.com/smpp-server/smpp-server/api/proto/routingv1"
)

routingClient := routingpb.NewRoutingServiceClient(conn)

req := &routingpb.GetRouteRequest{
    Destination: "79001234567",
    ClientId:    "550e8400-e29b-41d4-a716-446655440000",
}

resp, err := routingClient.GetRoute(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Route ID: %s\n", resp.Route.RouteId)
```

## Примеры использования (Python)

### Установка зависимостей

```bash
pip install grpcio grpcio-tools
```

### Генерация кода

```bash
python -m grpc_tools.protoc \
    --python_out=. \
    --grpc_python_out=. \
    --proto_path=api/proto \
    api/proto/messaging/messaging.proto
```

### Использование

```python
import grpc
from api.proto import messaging_pb2, messaging_pb2_grpc

# Создание соединения
channel = grpc.insecure_channel('localhost:50052')
stub = messaging_pb2_grpc.MessagingServiceStub(channel)

# Отправка сообщения
request = messaging_pb2.SendMessageRequest(
    client_id='550e8400-e29b-41d4-a716-446655440000',
    source='12345',
    destination='79001234567',
    text='Test message'
)

response = stub.SendMessage(request)
print(f"Message ID: {response.message_id}")
```

## Обработка ошибок

gRPC использует статус коды для ошибок:

```go
import (
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/codes"
)

resp, err := client.SomeMethod(ctx, req)
if err != nil {
    st, ok := status.FromError(err)
    if ok {
        switch st.Code() {
        case codes.InvalidArgument:
            fmt.Println("Invalid request:", st.Message())
        case codes.NotFound:
            fmt.Println("Resource not found")
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
}
```

## Таймауты и контекст

```go
import (
    "context"
    "time"
)

// Создание контекста с таймаутом
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Использование контекста
resp, err := client.SomeMethod(ctx, req)
```

## Connection Pooling

Для оптимизации рекомендуется использовать connection pooling:

```go
type ServiceClients struct {
    conn   *grpc.ClientConn
    Auth   authpb.AuthServiceClient
    Messaging messagingpb.MessagingServiceClient
    // ...
}

func NewServiceClients(addrs map[string]string) (*ServiceClients, error) {
    conn, err := grpc.Dial(
        addrs["messaging"],
        grpc.WithInsecure(),
        grpc.WithDefaultCallOptions(
            grpc.MaxCallRecvMsgSize(4*1024*1024), // 4MB
            grpc.MaxCallSendMsgSize(4*1024*1024),
        ),
    )
    if err != nil {
        return nil, err
    }
    
    return &ServiceClients{
        conn:      conn,
        Auth:      authpb.NewAuthServiceClient(conn),
        Messaging: messagingpb.NewMessagingServiceClient(conn),
    }, nil
}
```

## Retry и Circuit Breaker

Рекомендуется использовать retry для временных ошибок:

```go
import (
    "github.com/cenkalti/backoff/v4"
)

op := func() error {
    resp, err := client.SomeMethod(ctx, req)
    if err != nil {
        if st, ok := status.FromError(err); ok {
            if st.Code() == codes.Unavailable {
                return err // Retry
            }
        }
        return backoff.Permanent(err) // Don't retry
    }
    return nil
}

err := backoff.Retry(op, backoff.NewExponentialBackOff())
```

## Метрики

Все gRPC сервисы экспортируют метрики Prometheus:

```
grpc_requests_total{method="SendMessage", service="messaging", status="ok"} 1000
grpc_request_duration_seconds{method="SendMessage", service="messaging", quantile="0.95"} 0.1
grpc_errors_total{method="SendMessage", service="messaging", code="UNAVAILABLE"} 5
```

## Дополнительная документация

- [Gateway слой](../architecture/gateway-layer.md)
- [Паттерны коммуникации](../architecture/communication-patterns.md)
- [gRPC API](grpc.md) - документация по gRPC API Gateway