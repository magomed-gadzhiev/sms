# DDD Паттерны

## Обзор

Проект использует принципы Domain-Driven Design (DDD) для организации кода микросервисов. Этот документ описывает используемые паттерны и практики.

## Архитектурные слои

### 1. Domain Layer (Доменный слой)

**Ответственность:** Бизнес-логика и правила предметной области.

**Содержит:**
- **Entities** - сущности с идентификаторами
- **Value Objects** - объекты-значения без идентификаторов
- **Aggregates** - агрегаты (коллекции сущностей)
- **Domain Services** - сервисы домена (бизнес-логика)
- **Repository Interfaces** - интерфейсы репозиториев
- **Domain Events** - доменные события

**Пример:**

```go
// internal/services/messaging/domain/message.go
package domain

import (
    "time"
    "github.com/google/uuid"
)

// Message - агрегат сообщения
type Message struct {
    ID            uuid.UUID
    ClientID      uuid.UUID
    Source        string
    Destination   string
    Text          string
    Status        MessageStatus
    CreatedAt     time.Time
    SubmittedAt   *time.Time
    DeliveredAt   *time.Time
}

// MessageStatus - Value Object
type MessageStatus string

const (
    MessageStatusPending   MessageStatus = "pending"
    MessageStatusQueued    MessageStatus = "queued"
    MessageStatusSent      MessageStatus = "sent"
    MessageStatusDelivered MessageStatus = "delivered"
    MessageStatusFailed    MessageStatus = "failed"
)

// NewMessage - конструктор
func NewMessage(clientID uuid.UUID, source, destination, text string) *Message {
    return &Message{
        ID:          uuid.New(),
        ClientID:    clientID,
        Source:      source,
        Destination: destination,
        Text:        text,
        Status:      MessageStatusPending,
        CreatedAt:   time.Now(),
    }
}

// MarkAsQueued - бизнес-логика изменения статуса
func (m *Message) MarkAsQueued() error {
    if m.Status != MessageStatusPending {
        return ErrInvalidStatusTransition
    }
    m.Status = MessageStatusQueued
    return nil
}

// MarkAsSent - бизнес-логика
func (m *Message) MarkAsSent() error {
    if m.Status != MessageStatusQueued {
        return ErrInvalidStatusTransition
    }
    m.Status = MessageStatusSent
    now := time.Now()
    m.SubmittedAt = &now
    return nil
}
```

### 2. Application Layer (Слой приложения)

**Ответственность:** Координация доменных объектов и оркестрация бизнес-процессов.

**Содержит:**
- **Application Services** - сервисы приложения
- **DTOs** - объекты передачи данных
- **Use Cases** - варианты использования

**Пример:**

```go
// internal/services/messaging/application/message_service.go
package application

import (
    "context"
    "github.com/google/uuid"
    "github.com/smpp-server/smpp-server/internal/services/messaging/domain"
)

type MessageService struct {
    messageRepo   domain.MessageRepository
    eventPublisher domain.EventPublisher
}

func NewMessageService(
    messageRepo domain.MessageRepository,
    eventPublisher domain.EventPublisher,
) *MessageService {
    return &MessageService{
        messageRepo:    messageRepo,
        eventPublisher: eventPublisher,
    }
}

func (s *MessageService) SendMessage(
    ctx context.Context,
    clientID uuid.UUID,
    source, destination, text string,
) (*domain.Message, error) {
    // Создание доменного объекта
    message := domain.NewMessage(clientID, source, destination, text)
    
    // Валидация
    if err := message.Validate(); err != nil {
        return nil, err
    }
    
    // Сохранение
    if err := s.messageRepo.Create(ctx, message); err != nil {
        return nil, err
    }
    
    // Публикация события
    if err := s.eventPublisher.PublishMessageCreated(ctx, message); err != nil {
        // Log error but don't fail
    }
    
    return message, nil
}
```

### 3. Infrastructure Layer (Инфраструктурный слой)

**Ответственность:** Технические детали реализации (БД, очереди, внешние API).

**Содержит:**
- **Repository Implementations** - реализация репозиториев
- **Event Publishers** - публикация событий
- **Event Consumers** - потребление событий
- **External Services** - интеграции с внешними сервисами

**Пример:**

```go
// internal/services/messaging/infrastructure/repository/postgres.go
package repository

import (
    "context"
    "github.com/jackc/pgx/v5"
    "github.com/smpp-server/smpp-server/internal/services/messaging/domain"
)

type PostgresMessageRepository struct {
    db *pgx.Conn
}

func (r *PostgresMessageRepository) Create(
    ctx context.Context,
    msg *domain.Message,
) error {
    query := `
        INSERT INTO messages (id, client_id, source, destination, text, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `
    _, err := r.db.Exec(
        ctx,
        query,
        msg.ID, msg.ClientID, msg.Source, msg.Destination,
        msg.Text, msg.Status, msg.CreatedAt,
    )
    return err
}
```

## Основные паттерны

### Aggregate Pattern

**Назначение:** Группировка связанных сущностей в единое целое с четкими границами.

**Правила:**
- Агрегат имеет корневую сущность (Aggregate Root)
- Доступ к сущностям агрегата только через корень
- Транзакционные границы совпадают с границами агрегата

**Пример:**

```go
// Message - Aggregate Root
type Message struct {
    ID            uuid.UUID
    // ... другие поля
    DLRs          []*DLR  // Дочерние сущности
}

// Доступ к DLR только через Message
func (m *Message) AddDLR(dlr *DLR) error {
    // Бизнес-правила
    if m.Status == MessageStatusDelivered {
        return ErrMessageAlreadyDelivered
    }
    m.DLRs = append(m.DLRs, dlr)
    return nil
}
```

### Repository Pattern

**Назначение:** Абстракция доступа к данным.

**Правила:**
- Интерфейс в domain слое
- Реализация в infrastructure слое
- Используется только для агрегатов

**Пример:**

```go
// Domain interface
type MessageRepository interface {
    Create(ctx context.Context, msg *Message) error
    GetByID(ctx context.Context, id uuid.UUID) (*Message, error)
    Update(ctx context.Context, msg *Message) error
}

// Infrastructure implementation
type PostgresMessageRepository struct {
    db *pgx.Conn
}
```

### Domain Events Pattern

**Назначение:** Уведомление о важных событиях в домене.

**Правила:**
- События генерируются в domain слое
- Публикуются через infrastructure слой
- Иммутабельные объекты

**Пример:**

```go
// Domain event
type MessageCreatedEvent struct {
    MessageID   uuid.UUID
    ClientID    uuid.UUID
    Destination string
    CreatedAt   time.Time
}

// В domain
func (m *Message) MarkAsQueued() error {
    m.Status = MessageStatusQueued
    m.Events = append(m.Events, MessageQueuedEvent{
        MessageID: m.ID,
        ClientID:  m.ClientID,
    })
    return nil
}

// В application
func (s *MessageService) ProcessMessage(ctx context.Context, msg *Message) error {
    if err := msg.MarkAsQueued(); err != nil {
        return err
    }
    
    // Публикация событий
    for _, event := range msg.Events {
        if err := s.eventPublisher.Publish(ctx, event); err != nil {
            // Log
        }
    }
    
    msg.ClearEvents()
    return s.messageRepo.Update(ctx, msg)
}
```

### Value Object Pattern

**Назначение:** Объекты без идентификатора, определяемые значениями.

**Правила:**
- Иммутабельные
- Равенство по значениям
- Без идентификатора

**Пример:**

```go
// Value Object
type PhoneNumber struct {
    value string
}

func NewPhoneNumber(value string) (*PhoneNumber, error) {
    if !isValidPhoneNumber(value) {
        return nil, ErrInvalidPhoneNumber
    }
    return &PhoneNumber{value: normalizePhoneNumber(value)}, nil
}

func (p *PhoneNumber) String() string {
    return p.value
}

func (p *PhoneNumber) Equals(other *PhoneNumber) bool {
    return p.value == other.value
}
```

### Domain Service Pattern

**Назначение:** Бизнес-логика, которая не принадлежит одной сущности.

**Пример:**

```go
// Domain Service
type MessageValidator struct{}

func (v *MessageValidator) Validate(msg *Message) error {
    if len(msg.Text) == 0 {
        return ErrEmptyMessage
    }
    if len(msg.Text) > 1600 {
        return ErrMessageTooLong
    }
    if !isValidPhoneNumber(msg.Destination) {
        return ErrInvalidDestination
    }
    return nil
}

// Использование
validator := domain.NewMessageValidator()
if err := validator.Validate(message); err != nil {
    return err
}
```

### Factory Pattern

**Назначение:** Создание сложных агрегатов.

**Пример:**

```go
type MessageFactory struct {
    validator *MessageValidator
}

func (f *MessageFactory) CreateMessage(
    clientID uuid.UUID,
    source, destination, text string,
) (*Message, error) {
    msg := &Message{
        ID:          uuid.New(),
        ClientID:    clientID,
        Source:      source,
        Destination: destination,
        Text:        text,
        Status:      MessageStatusPending,
        CreatedAt:   time.Now(),
    }
    
    if err := f.validator.Validate(msg); err != nil {
        return nil, err
    }
    
    return msg, nil
}
```

## Принципы

### 1. Ubiquitous Language

Используйте термины предметной области в коде:

```go
// ✅ Хорошо
type Message struct {
    Destination string
    Text        string
}

// ❌ Плохо
type SMS struct {
    To   string
    Body string
}
```

### 2. Bounded Context

Каждый микросервис = один bounded context с четкими границами.

### 3. Dependency Inversion

Domain слой не зависит от infrastructure:

```
Application Layer
    ↓ (depends on)
Domain Layer
    ↑ (implemented by)
Infrastructure Layer
```

### 4. Separation of Concerns

- Domain: бизнес-логика
- Application: координация
- Infrastructure: технические детали

## Best Practices

1. **Инкапсуляция** - бизнес-логика внутри сущностей
2. **Неизменяемость** - value objects иммутабельны
3. **Явные зависимости** - зависимости через конструктор
4. **Fail Fast** - валидация на ранних этапах
5. **События** - используйте события для слабой связанности

## Дополнительная документация

- [DDD домены](../architecture/ddd-domains.md)
- [Разработка сервисов](service-development.md)
- [Тестирование](testing.md)