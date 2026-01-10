# Паттерны коммуникации между сервисами

## Обзор

Микросервисная архитектура использует два основных паттерна коммуникации:

1. **Синхронная (gRPC)** - для запросов, требующих немедленного ответа
2. **Асинхронная (Kafka)** - для событий и длительных операций

## Синхронная коммуникация (gRPC)

### Когда использовать

- Gateway → Service (валидация, получение данных)
- Service → Service (запрос данных из другого домена)
- Операции, требующие немедленного ответа
- Транзакционные операции

### Архитектура

```
Gateway (HTTP/gRPC)
    ↓
gRPC Client (connection pool)
    ↓
Service gRPC Server
    ↓
Domain Service
    ↓
Repository
```

### Примеры использования

#### 1. Валидация токена (Client Gateway → Auth Service)

```go
// Client Gateway
authClient := authpb.NewAuthServiceClient(conn)
resp, err := authClient.ValidateToken(ctx, &authpb.ValidateTokenRequest{
    Token: token,
})
```

**Преимущества:**
- Немедленная валидация
- Простая обработка ошибок
- Типобезопасность

#### 2. Получение маршрута (Messaging Service → Routing Service)

```go
// Messaging Service
routingClient := routingpb.NewRoutingServiceClient(conn)
route, err := routingClient.GetRoute(ctx, &routingpb.GetRouteRequest{
    Destination: destination,
    ClientId:    clientID.String(),
})
```

#### 3. Получение клиента (Gateway → Client Service)

```go
// Gateway
clientClient := clientpb.NewClientServiceClient(conn)
client, err := clientClient.GetClient(ctx, &clientpb.GetClientRequest{
    ClientId: clientID.String(),
})
```

### Connection Pooling

Для оптимизации используется connection pooling:

```go
type GRPCClients struct {
    authClient     authpb.AuthServiceClient
    clientClient   clientpb.ClientServiceClient
    routingClient  routingpb.RoutingServiceClient
    providerClient providerpb.ProviderServiceClient
    // ...
}

func NewGRPCClients(cfg *Config) (*GRPCClients, error) {
    authConn, err := grpc.Dial(
        cfg.AuthServiceAddr,
        grpc.WithInsecure(),
        grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(4*1024*1024)),
    )
    // ...
}
```

### Retry и Circuit Breaker

```go
// Retry для временных ошибок
retryPolicy := retry.Do(
    func() error {
        return grpcCall(ctx, req)
    },
    retry.Attempts(3),
    retry.Delay(100*time.Millisecond),
    retry.BackoffStrategy(retry.ExponentialBackoff),
)

// Circuit Breaker для защиты от каскадных отказов
breaker := circuit.New(
    circuit.WithFailureThreshold(5),
    circuit.WithSuccessThreshold(2),
    circuit.WithTimeout(30*time.Second),
)
```

### Обработка ошибок

```go
resp, err := client.SomeMethod(ctx, req)
if err != nil {
    st, ok := status.FromError(err)
    if ok {
        switch st.Code() {
        case codes.DeadlineExceeded:
            // Timeout
        case codes.Unavailable:
            // Service unavailable - retry
        case codes.Internal:
            // Internal error - log and return
        }
    }
}
```

### Таймауты

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := client.SomeMethod(ctx, req)
```

## Асинхронная коммуникация (Kafka)

### Когда использовать

- Event-driven архитектура
- Длительные операции
- Уведомления между сервисами
- Аналитика и биллинг

### Архитектура

```
Service A
    ↓
Kafka Producer
    ↓
Kafka Topic (partitions)
    ↓
Kafka Consumer (consumer group)
    ↓
Service B
```

### Kafka Topics

#### Message Lifecycle Topics

```
sms.message.created
├── Published by: Messaging Service
├── Consumed by: Routing Service, Analytics Service
└── Payload: {message_id, client_id, source, destination, ...}

sms.message.queued
├── Published by: Messaging Service
├── Consumed by: Routing Service
└── Payload: {message_id, ...}

sms.message.routed
├── Published by: Routing Service
├── Consumed by: Provider Service
└── Payload: {message_id, provider_id, route_id, ...}

sms.message.sent
├── Published by: Provider Service
├── Consumed by: Analytics Service, Billing Service
└── Payload: {message_id, provider_id, smpp_message_id, ...}

sms.message.delivered
├── Published by: Provider Service (via DLR)
├── Consumed by: Analytics Service, Billing Service
└── Payload: {message_id, delivery_time, ...}

sms.message.failed
├── Published by: Provider Service
├── Consumed by: Analytics Service, Billing Service
└── Payload: {message_id, error_code, error_message, ...}
```

#### DLR Topics

```
sms.dlr.received
├── Published by: Provider Service
├── Consumed by: Messaging Service
└── Payload: {message_id, stat, error_code, ...}
```

#### Billing Topics

```
billing.transaction.created
├── Published by: Billing Service
├── Consumed by: Analytics Service
└── Payload: {transaction_id, client_id, amount, type, ...}

billing.balance.changed
├── Published by: Billing Service
├── Consumed by: Client Service (cache invalidation)
└── Payload: {client_id, new_balance, ...}
```

### Producer Pattern

```go
type EventPublisher interface {
    PublishMessageCreated(ctx context.Context, msg *Message) error
    PublishMessageQueued(ctx context.Context, msgID uuid.UUID) error
    // ...
}

type KafkaEventPublisher struct {
    producer sarama.SyncProducer
}

func (p *KafkaEventPublisher) PublishMessageCreated(
    ctx context.Context,
    msg *Message,
) error {
    event := MessageCreatedEvent{
        MessageID:   msg.ID.String(),
        ClientID:    msg.ClientID.String(),
        Source:      msg.Source,
        Destination: msg.Destination,
        CreatedAt:   msg.CreatedAt,
    }
    
    data, err := json.Marshal(event)
    if err != nil {
        return err
    }
    
    _, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
        Topic: "sms.message.created",
        Key:   sarama.StringEncoder(msg.ID.String()),
        Value: sarama.ByteEncoder(data),
    })
    
    return err
}
```

### Consumer Pattern

```go
type KafkaConsumer struct {
    consumer sarama.ConsumerGroup
    handler  MessageHandler
}

func (c *KafkaConsumer) Consume(ctx context.Context) error {
    handler := &consumerGroupHandler{
        handler: c.handler,
    }
    
    for {
        err := c.consumer.Consume(ctx, []string{"sms.message.created"}, handler)
        if err != nil {
            return err
        }
        
        if ctx.Err() != nil {
            return ctx.Err()
        }
    }
}

type consumerGroupHandler struct {
    handler MessageHandler
}

func (h *consumerGroupHandler) ConsumeClaim(
    session sarama.ConsumerGroupSession,
    claim sarama.ConsumerGroupClaim,
) error {
    for {
        select {
        case message := <-claim.Messages():
            var event MessageCreatedEvent
            if err := json.Unmarshal(message.Value, &event); err != nil {
                continue
            }
            
            if err := h.handler.HandleMessageCreated(session.Context(), &event); err != nil {
                // Обработка ошибки (retry, DLQ)
            }
            
            session.MarkMessage(message, "")
            
        case <-session.Context().Done():
            return nil
        }
    }
}
```

### Consumer Groups

Для масштабирования используются consumer groups:

```yaml
# Service instances
worker-1 (consumer group: routing-service)
worker-2 (consumer group: routing-service)
worker-3 (consumer group: routing-service)

# Kafka автоматически распределяет партиции между инстансами
Topic: sms.message.queued (10 partitions)
├── Partition 0,1,2,3 → worker-1
├── Partition 4,5,6 → worker-2
└── Partition 7,8,9 → worker-3
```

### Error Handling и Retry

```go
func (h *Handler) HandleMessage(ctx context.Context, event *Event) error {
    maxRetries := 5
    backoff := 1 * time.Second
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        err := h.processMessage(ctx, event)
        if err == nil {
            return nil
        }
        
        // Permanent error - отправка в DLQ
        if isPermanentError(err) {
            return h.sendToDLQ(ctx, event, err)
        }
        
        // Temporary error - retry
        if attempt < maxRetries-1 {
            time.Sleep(backoff)
            backoff *= 2 // Exponential backoff
        }
    }
    
    // Max retries exceeded - DLQ
    return h.sendToDLQ(ctx, event, errors.New("max retries exceeded"))
}
```

### Dead Letter Queue (DLQ)

Для обработки неудачных сообщений:

```go
func (h *Handler) sendToDLQ(ctx context.Context, event *Event, err error) error {
    dlqEvent := DLQEvent{
        OriginalEvent: event,
        Error:         err.Error(),
        Timestamp:     time.Now(),
        RetryCount:    h.retryCount,
    }
    
    data, _ := json.Marshal(dlqEvent)
    _, _, sendErr := h.producer.SendMessage(&sarama.ProducerMessage{
        Topic: "sms.message.failed",
        Value: sarama.ByteEncoder(data),
    })
    
    return sendErr
}
```

## Комбинированные паттерны

### Request-Reply через Events

Для асинхронного request-reply:

```go
// Request
correlationID := uuid.New().String()
reqEvent := RouteRequestEvent{
    CorrelationID: correlationID,
    Destination:   destination,
}

// Подписка на reply
replyChan := make(chan RouteReplyEvent)
h.subscribe(correlationID, replyChan)

// Публикация request
publisher.PublishRouteRequest(ctx, &reqEvent)

// Ожидание reply
select {
case reply := <-replyChan:
    return reply.Route
case <-ctx.Done():
    return nil, ctx.Err()
}
```

### Saga Pattern

Для распределенных транзакций:

```go
// Saga: Create Message → Route → Send → Charge
func (s *Saga) Execute(ctx context.Context, cmd *SendMessageCommand) error {
    // Step 1: Create message
    msg, err := s.messagingService.CreateMessage(ctx, cmd)
    if err != nil {
        return err
    }
    
    // Step 2: Route message (async event)
    s.eventPublisher.PublishMessageQueued(ctx, msg.ID)
    
    // Если нужно откатить
    // s.messagingService.CancelMessage(ctx, msg.ID)
    
    return nil
}
```

## Best Practices

### 1. Idempotency

Все операции должны быть идемпотентными:

```go
// Проверка на дубликаты
if msg, exists := s.getMessage(ctx, externalID); exists {
    return msg, nil // Возвращаем существующее
}

// Создание нового
msg := s.createMessage(ctx, cmd)
```

### 2. Event Sourcing (опционально)

Хранение всех событий для восстановления состояния:

```go
type EventStore interface {
    Append(ctx context.Context, aggregateID uuid.UUID, event Event) error
    GetEvents(ctx context.Context, aggregateID uuid.UUID) ([]Event, error)
}
```

### 3. CQRS (Command Query Responsibility Segregation)

Разделение операций чтения и записи:

```go
// Command (write)
func (s *Service) SendMessage(ctx context.Context, cmd *SendMessageCommand) error {
    // ...
}

// Query (read)
func (s *Service) GetMessageStatus(ctx context.Context, query *GetStatusQuery) (*Status, error) {
    // ...
}
```

### 4. Backpressure

Ограничение нагрузки при перегрузке:

```go
// Rate limiting в consumer
rateLimiter := rate.NewLimiter(100, 1000) // 100 events/sec

func (h *Handler) ConsumeClaim(...) error {
    for message := range claim.Messages() {
        if err := rateLimiter.Wait(ctx); err != nil {
            continue
        }
        // Process message
    }
}
```

## Мониторинг коммуникации

### gRPC Metrics

```go
// Request count
grpc_requests_total{method="SendMessage", status="ok"} 1000

// Request duration
grpc_request_duration_seconds{method="SendMessage", quantile="0.95"} 0.1

// Errors
grpc_errors_total{method="SendMessage", code="UNAVAILABLE"} 5
```

### Kafka Metrics

```go
// Producer
kafka_producer_messages_total{topic="sms.message.created"} 10000
kafka_producer_errors_total{topic="sms.message.created"} 2

// Consumer
kafka_consumer_messages_total{topic="sms.message.queued", group="routing-service"} 9500
kafka_consumer_lag{topic="sms.message.queued", group="routing-service"} 500
kafka_consumer_errors_total{topic="sms.message.queued", group="routing-service"} 1
```

## Дополнительная документация

- [Обзор архитектуры](overview.md)
- [DDD домены](ddd-domains.md)
- [Gateway слой](gateway-layer.md)
- [Kafka интеграция](../development/kafka.md)