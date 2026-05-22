# Разработка сервисов

## Обзор

Это руководство описывает процесс разработки новых микросервисов и их интеграции в систему.

## Структура сервиса (DDD)

Каждый сервис следует структуре Domain-Driven Design:

```
internal/services/{service-name}/
├── domain/                 # Доменная логика
│   ├── {entity}.go        # Сущности домена
│   ├── repository.go      # Интерфейсы репозиториев
│   └── events.go          # Доменные события
├── application/           # Слой приложения
│   ├── {service}_service.go  # Сервисы приложения
│   └── dto.go            # Data Transfer Objects
├── infrastructure/        # Инфраструктурный слой
│   ├── repository/       # Реализация репозиториев
│   │   └── postgres.go
│   ├── queue/           # Kafka интеграция
│   │   └── kafka.go
│   └── cache/           # Redis интеграция
│       └── redis.go
└── grpc/                # gRPC сервер
    └── server.go
```

## Создание нового сервиса

### 1. Создание proto файла

Создайте proto файл в `api/proto/{service-name}/{service-name}.proto`:

```protobuf
syntax = "proto3";

package {service}.v1;

option go_package = "github.com/smpp-server/smpp-server/api/proto/{service}v1";

import "google/protobuf/timestamp.proto";

service {Service}Service {
  rpc SomeMethod(SomeMethodRequest) returns (SomeMethodResponse);
}

message SomeMethodRequest {
  string field = 1;
}

message SomeMethodResponse {
  string result = 1;
}
```

### 2. Генерация кода

```bash
.\scripts\generate-proto.ps1
```

### 3. Создание доменных сущностей

```go
// internal/services/{service}/domain/{entity}.go
package domain

import (
    "time"
    "github.com/google/uuid"
)

type Entity struct {
    ID        uuid.UUID
    Name      string
    CreatedAt time.Time
    UpdatedAt time.Time
}

func NewEntity(name string) *Entity {
    return &Entity{
        ID:        uuid.New(),
        Name:      name,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
}

func (e *Entity) UpdateName(name string) {
    e.Name = name
    e.UpdatedAt = time.Now()
}
```

### 4. Создание репозитория интерфейса

```go
// internal/services/{service}/domain/repository.go
package domain

import (
    "context"
    "github.com/google/uuid"
)

type EntityRepository interface {
    Create(ctx context.Context, entity *Entity) error
    GetByID(ctx context.Context, id uuid.UUID) (*Entity, error)
    Update(ctx context.Context, entity *Entity) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### 5. Создание сервиса приложения

```go
// internal/services/{service}/application/{service}_service.go
package application

import (
    "context"
    "github.com/google/uuid"
    "github.com/smpp-server/smpp-server/internal/services/{service}/domain"
)

type ServiceService struct {
    repo           domain.EntityRepository
    eventPublisher domain.EventPublisher
}

func NewServiceService(
    repo domain.EntityRepository,
    eventPublisher domain.EventPublisher,
) *ServiceService {
    return &ServiceService{
        repo:           repo,
        eventPublisher: eventPublisher,
    }
}

func (s *ServiceService) CreateEntity(
    ctx context.Context,
    name string,
) (*domain.Entity, error) {
    entity := domain.NewEntity(name)
    
    if err := s.repo.Create(ctx, entity); err != nil {
        return nil, err
    }
    
    // Публикация события
    if err := s.eventPublisher.PublishEntityCreated(ctx, entity); err != nil {
        // Log error but don't fail
    }
    
    return entity, nil
}
```

### 6. Реализация репозитория

```go
// internal/services/{service}/infrastructure/repository/postgres.go
package repository

import (
    "context"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/smpp-server/smpp-server/internal/services/{service}/domain"
)

type PostgresRepository struct {
    db *pgx.Conn
}

func NewPostgresRepository(db *pgx.Conn) *PostgresRepository {
    return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(
    ctx context.Context,
    entity *domain.Entity,
) error {
    query := `
        INSERT INTO entities (id, name, created_at, updated_at)
        VALUES ($1, $2, $3, $4)
    `
    
    _, err := r.db.Exec(
        ctx,
        query,
        entity.ID,
        entity.Name,
        entity.CreatedAt,
        entity.UpdatedAt,
    )
    
    return err
}

func (r *PostgresRepository) GetByID(
    ctx context.Context,
    id uuid.UUID,
) (*domain.Entity, error) {
    query := `
        SELECT id, name, created_at, updated_at
        FROM entities
        WHERE id = $1
    `
    
    var entity domain.Entity
    err := r.db.QueryRow(ctx, query, id).Scan(
        &entity.ID,
        &entity.Name,
        &entity.CreatedAt,
        &entity.UpdatedAt,
    )
    
    if err == pgx.ErrNoRows {
        return nil, domain.ErrEntityNotFound
    }
    
    return &entity, err
}
```

### 7. Создание gRPC сервера

```go
// internal/services/{service}/grpc/server.go
package grpc

import (
    "context"
    "net"
    
    "google.golang.org/grpc"
    servicepb "github.com/smpp-server/smpp-server/api/proto/{service}v1"
    "github.com/smpp-server/smpp-server/internal/services/{service}/application"
)

type Server struct {
    servicepb.Unimplemented{Service}ServiceServer
    appService *application.ServiceService
}

func NewServer(appService *application.ServiceService) *Server {
    return &Server{
        appService: appService,
    }
}

func (s *Server) SomeMethod(
    ctx context.Context,
    req *servicepb.SomeMethodRequest,
) (*servicepb.SomeMethodResponse, error) {
    entity, err := s.appService.CreateEntity(ctx, req.Field)
    if err != nil {
        return nil, err
    }
    
    return &servicepb.SomeMethodResponse{
        Result: entity.Name,
    }, nil
}

func (s *Server) Start(addr string) error {
    lis, err := net.Listen("tcp", addr)
    if err != nil {
        return err
    }
    
    server := grpc.NewServer()
    servicepb.Register{Service}ServiceServer(server, s)
    
    return server.Serve(lis)
}
```

### 8. Создание main.go

```go
// cmd/services/{service}-service/main.go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/rs/zerolog/log"
    "github.com/smpp-server/smpp-server/internal/config"
    "github.com/smpp-server/smpp-server/internal/services/{service}/application"
    "github.com/smpp-server/smpp-server/internal/services/{service}/grpc"
    "github.com/smpp-server/smpp-server/internal/services/{service}/infrastructure/repository"
)

func main() {
    cfg := config.Load()
    
    // Инициализация БД
    db := initDB(cfg)
    defer db.Close()
    
    // Инициализация репозитория
    repo := repository.NewPostgresRepository(db)
    
    // Инициализация event publisher
    eventPublisher := initEventPublisher(cfg)
    
    // Инициализация сервиса
    appService := application.NewServiceService(repo, eventPublisher)
    
    // Инициализация gRPC сервера
    grpcServer := grpc.NewServer(appService)
    
    // Запуск сервера
    ctx, cancel := signal.NotifyContext(
        context.Background(),
        os.Interrupt,
        syscall.SIGTERM,
    )
    defer cancel()
    
    go func() {
        if err := grpcServer.Start(cfg.GRPC.Addr); err != nil {
            log.Fatal().Err(err).Msg("Failed to start gRPC server")
        }
    }()
    
    <-ctx.Done()
    log.Info().Msg("Shutting down...")
}
```

## Интеграция с Kafka

### Создание Event Publisher

```go
// internal/services/{service}/infrastructure/queue/kafka.go
package queue

import (
    "context"
    "encoding/json"
    
    "github.com/IBM/sarama"
    "github.com/smpp-server/smpp-server/internal/services/{service}/domain"
)

type KafkaEventPublisher struct {
    producer sarama.SyncProducer
}

func NewKafkaEventPublisher(producer sarama.SyncProducer) *KafkaEventPublisher {
    return &KafkaEventPublisher{producer: producer}
}

func (p *KafkaEventPublisher) PublishEntityCreated(
    ctx context.Context,
    entity *domain.Entity,
) error {
    event := EntityCreatedEvent{
        EntityID:  entity.ID.String(),
        Name:      entity.Name,
        CreatedAt: entity.CreatedAt,
    }
    
    data, err := json.Marshal(event)
    if err != nil {
        return err
    }
    
    _, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
        Topic: "{service}.entity.created",
        Key:   sarama.StringEncoder(entity.ID.String()),
        Value: sarama.ByteEncoder(data),
    })
    
    return err
}
```

### Создание Consumer

```go
// internal/services/{service}/infrastructure/queue/consumer.go
package queue

import (
    "context"
    "encoding/json"
    
    "github.com/IBM/sarama"
)

type Consumer struct {
    consumer sarama.ConsumerGroup
    handler  EventHandler
}

func NewConsumer(consumer sarama.ConsumerGroup, handler EventHandler) *Consumer {
    return &Consumer{
        consumer: consumer,
        handler:  handler,
    }
}

func (c *Consumer) Consume(ctx context.Context, topics []string) error {
    handler := &consumerGroupHandler{handler: c.handler}
    
    for {
        err := c.consumer.Consume(ctx, topics, handler)
        if err != nil {
            return err
        }
        
        if ctx.Err() != nil {
            return ctx.Err()
        }
    }
}
```

## Тестирование

### Unit тесты

```go
// internal/services/{service}/application/{service}_service_test.go
package application

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/smpp-server/smpp-server/internal/services/{service}/domain"
)

func TestServiceService_CreateEntity(t *testing.T) {
    repo := &MockRepository{}
    eventPublisher := &MockEventPublisher{}
    
    service := NewServiceService(repo, eventPublisher)
    
    entity, err := service.CreateEntity(context.Background(), "test")
    
    assert.NoError(t, err)
    assert.NotNil(t, entity)
    assert.Equal(t, "test", entity.Name)
}
```

### Integration тесты

```go
func TestServiceService_CreateEntity_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    db := setupTestDB(t)
    defer db.Close()
    
    repo := repository.NewPostgresRepository(db)
    eventPublisher := &MockEventPublisher{}
    
    service := NewServiceService(repo, eventPublisher)
    
    entity, err := service.CreateEntity(context.Background(), "test")
    
    assert.NoError(t, err)
    
    // Проверка в БД
    found, err := repo.GetByID(context.Background(), entity.ID)
    assert.NoError(t, err)
    assert.Equal(t, entity.Name, found.Name)
}
```

## Best Practices

1. **Инкапсуляция логики** - бизнес-логика должна быть в domain слое
2. **Интерфейсы** - используйте интерфейсы для зависимостей
3. **Обработка ошибок** - возвращайте доменные ошибки
4. **События** - публикуйте события для асинхронной коммуникации
5. **Тестируемость** - пишите тесты для каждого слоя

## Дополнительная документация

- [DDD паттерны](ddd-patterns.md)
- [Тестирование](testing.md)
- [DDD домены](../architecture/ddd-domains.md)