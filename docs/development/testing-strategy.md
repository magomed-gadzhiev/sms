# Стратегия тестирования

## Обзор

Этот документ описывает стратегию тестирования для микросервисной архитектуры SMPP сервера.

## Пирамида тестирования

```
        /\
       /  \      E2E Tests (мало)
      /____\     Integration Tests (средне)
     /      \    Unit Tests (много)
    /________\
```

## Уровни тестирования

### 1. Unit Tests (70% покрытия)

**Назначение:** Тестирование отдельных функций и методов изолированно.

**Где:** В том же пакете, что и тестируемый код.

**Что тестировать:**
- Domain логика (entities, value objects, domain services)
- Application services (бизнес-логика)
- Utility функции
- Валидация

**Пример:**

```go
// internal/services/messaging/domain/message_test.go
func TestMessage_MarkAsQueued(t *testing.T) {
    msg := NewMessage(
        uuid.New(),
        "12345",
        "79001234567",
        "Test",
    )
    
    err := msg.MarkAsQueued()
    
    assert.NoError(t, err)
    assert.Equal(t, MessageStatusQueued, msg.Status)
}

func TestMessage_MarkAsQueued_InvalidTransition(t *testing.T) {
    msg := NewMessage(
        uuid.New(),
        "12345",
        "79001234567",
        "Test",
    )
    msg.MarkAsSent()
    
    err := msg.MarkAsQueued()
    
    assert.Error(t, err)
    assert.Equal(t, ErrInvalidStatusTransition, err)
}
```

### 2. Integration Tests (20% покрытия)

**Назначение:** Тестирование взаимодействия между компонентами.

**Где:** `test/integration/`

**Что тестировать:**
- Repository реализация (с реальной БД)
- Kafka producer/consumer
- gRPC клиент-сервер взаимодействие
- Взаимодействие между слоями

**Пример:**

```go
// test/integration/message_repository_test.go
func TestPostgresMessageRepository_Create(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    db := setupTestDB(t)
    defer db.Close()
    
    repo := repository.NewPostgresMessageRepository(db)
    
    msg := domain.NewMessage(
        uuid.New(),
        uuid.New(),
        "12345",
        "79001234567",
        "Test",
    )
    
    err := repo.Create(context.Background(), msg)
    assert.NoError(t, err)
    
    found, err := repo.GetByID(context.Background(), msg.ID)
    assert.NoError(t, err)
    assert.Equal(t, msg.Text, found.Text)
}
```

### 3. E2E Tests (10% покрытия)

**Назначение:** Тестирование полных сценариев использования.

**Где:** `test/e2e/`

**Что тестировать:**
- Полный поток отправки SMS
- Интеграция между сервисами
- Real-world сценарии

**Пример:**

```go
// test/e2e/sms_flow_test.go
func TestSMSFlow_EndToEnd(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping e2e test")
    }
    
    // Setup services
    gatewayClient := setupGatewayClient(t)
    messagingClient := setupMessagingClient(t)
    
    // Send SMS
    resp, err := gatewayClient.SendSMS(context.Background(), &SendSMSRequest{
        Source:      "12345",
        Destination: "79001234567",
        Text:        "Test",
    })
    assert.NoError(t, err)
    
    // Wait for processing
    time.Sleep(2 * time.Second)
    
    // Check status
    status, err := messagingClient.GetMessageStatus(context.Background(), &GetStatusRequest{
        MessageId: resp.MessageId,
    })
    assert.NoError(t, err)
    assert.Equal(t, "sent", status.Status)
}
```

## Типы тестов

### Domain Tests

Тестирование доменной логики без внешних зависимостей:

```go
func TestMessage_Validation(t *testing.T) {
    tests := []struct {
        name    string
        message *Message
        wantErr bool
    }{
        {
            name: "valid message",
            message: NewMessage(uuid.New(), "12345", "79001234567", "Test"),
            wantErr: false,
        },
        {
            name: "empty text",
            message: NewMessage(uuid.New(), "12345", "79001234567", ""),
            wantErr: true,
        },
        {
            name: "invalid destination",
            message: NewMessage(uuid.New(), "12345", "invalid", "Test"),
            wantErr: true,
        },
    }
    
    validator := NewMessageValidator()
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validator.Validate(tt.message)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Service Tests

Тестирование application services с моками:

```go
func TestMessageService_SendMessage(t *testing.T) {
    mockRepo := &MockMessageRepository{}
    mockPublisher := &MockEventPublisher{}
    
    service := application.NewMessageService(mockRepo, mockPublisher)
    
    msg, err := service.SendMessage(
        context.Background(),
        uuid.New(),
        "12345",
        "79001234567",
        "Test",
    )
    
    assert.NoError(t, err)
    assert.NotNil(t, msg)
    assert.True(t, mockRepo.CreateCalled)
    assert.True(t, mockPublisher.PublishCalled)
}
```

### Repository Tests

Тестирование репозиториев с тестовой БД:

```go
func TestPostgresMessageRepository(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()
    
    repo := repository.NewPostgresMessageRepository(db)
    
    t.Run("Create and Get", func(t *testing.T) {
        msg := domain.NewMessage(
            uuid.New(),
            uuid.New(),
            "12345",
            "79001234567",
            "Test",
        )
        
        err := repo.Create(context.Background(), msg)
        assert.NoError(t, err)
        
        found, err := repo.GetByID(context.Background(), msg.ID)
        assert.NoError(t, err)
        assert.Equal(t, msg.Text, found.Text)
    })
    
    t.Run("Update", func(t *testing.T) {
        // ...
    })
    
    t.Run("Delete", func(t *testing.T) {
        // ...
    })
}
```

### gRPC Tests

Тестирование gRPC handlers:

```go
func TestMessagingService_SendMessage(t *testing.T) {
    mockService := &MockMessageService{}
    server := grpc.NewServer(messagingpb.RegisterMessagingServiceServer(server, NewGRPCHandler(mockService)))
    
    conn, err := grpc.Dial("", grpc.WithInsecure(), grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
        return lis.Dial()
    }))
    // ...
    
    client := messagingpb.NewMessagingServiceClient(conn)
    
    resp, err := client.SendMessage(context.Background(), &messagingpb.SendMessageRequest{
        ClientId:    uuid.New().String(),
        Source:      "12345",
        Destination: "79001234567",
        Text:        "Test",
    })
    
    assert.NoError(t, err)
    assert.NotEmpty(t, resp.MessageId)
}
```

### Kafka Tests

Тестирование Kafka интеграции:

```go
func TestKafkaEventPublisher_Publish(t *testing.T) {
    producer := setupTestKafkaProducer(t)
    defer producer.Close()
    
    publisher := queue.NewKafkaEventPublisher(producer)
    
    msg := domain.NewMessage(
        uuid.New(),
        uuid.New(),
        "12345",
        "79001234567",
        "Test",
    )
    
    err := publisher.PublishMessageCreated(context.Background(), msg)
    assert.NoError(t, err)
    
    // Verify message in Kafka
    consumer := setupTestKafkaConsumer(t)
    // ...
}
```

## Mock и Stubs

### Mock Objects

Используйте моки для изоляции тестов:

```go
type MockMessageRepository struct {
    CreateFunc func(ctx context.Context, msg *domain.Message) error
    GetByIDFunc func(ctx context.Context, id uuid.UUID) (*domain.Message, error)
}

func (m *MockMessageRepository) Create(ctx context.Context, msg *domain.Message) error {
    if m.CreateFunc != nil {
        return m.CreateFunc(ctx, msg)
    }
    return nil
}

func (m *MockMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
    if m.GetByIDFunc != nil {
        return m.GetByIDFunc(ctx, id)
    }
    return nil, domain.ErrMessageNotFound
}
```

### Test Fixtures

Используйте фикстуры для тестовых данных:

```go
// internal/testutil/fixtures.go
func NewTestMessage() *domain.Message {
    return domain.NewMessage(
        uuid.New(),
        uuid.New(),
        "12345",
        "79001234567",
        "Test message",
    )
}

func NewTestClient() *domain.Client {
    return &domain.Client{
        ID:    uuid.New(),
        Name:  "Test Client",
        Email: "test@example.com",
    }
}
```

## Testcontainers

Для интеграционных тестов используйте testcontainers:

```go
func setupTestDB(t *testing.T) *pgx.Conn {
    ctx := context.Background()
    
    req := testcontainers.ContainerRequest{
        Image:        "postgres:15-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_USER":     "test",
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_DB":       "test",
        },
        WaitingFor: wait.ForLog("database system is ready"),
    }
    
    postgresC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)
    
    t.Cleanup(func() {
        postgresC.Terminate(ctx)
    })
    
    // Get connection string
    // ...
}
```

## Покрытие кода

### Цель покрытия

- **Domain слой:** 90%+
- **Application слой:** 80%+
- **Infrastructure слой:** 70%+
- **Общее покрытие:** 75%+

### Генерация отчета

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Performance Tests

Тестирование производительности:

```go
func BenchmarkMessageService_SendMessage(b *testing.B) {
    service := setupService(b)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := service.SendMessage(
            context.Background(),
            uuid.New(),
            "12345",
            "79001234567",
            "Test",
        )
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## Best Practices

1. **AAA Pattern** - Arrange, Act, Assert
2. **One Assert Per Test** - где возможно
3. **Descriptive Names** - понятные имена тестов
4. **Test Isolation** - независимые тесты
5. **Fast Tests** - быстрые unit тесты
6. **Cleanup** - очистка после тестов

## Дополнительная документация

- [Тестирование](testing.md) - общая документация по тестированию
- [Разработка сервисов](service-development.md)
- [DDD паттерны](ddd-patterns.md)