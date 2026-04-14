# Inbound SMS / 2-Way Messaging — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить поддержку входящих SMS (Mobile Originated, MO): принимать MO-сообщения через SMPP Gateway, сохранять в БД, публиковать в Kafka, доставлять клиентам через webhooks. Разблокирует consent management, auto-reply, 2-way conversations.

**Architecture:** SMPP Gateway уже обрабатывает исходящие — нужно добавить MO handler. MO flow: SMPP Gateway → Kafka `sms.inbound` → messaging service (сохранение) → webhook service (доставка клиенту). Входящие сообщения привязаны к virtual number/short code клиента.

**Tech Stack:** Go 1.24, IBM/sarama Kafka, jackc/pgx/v5, gorilla/mux, SMPP protocol (уже есть в SMPP Gateway), redis/go-redis/v9.

---

## Предварительные требования (не входят в план)

- [ ] Договоры с операторами на MO-трафик для конкретных номеров/short codes
- [ ] Виртуальные номера или short codes выделены в БД `virtual_numbers`

---

## File Map

| Файл | Действие | Что делаем |
|---|---|---|
| `migrations/XXXXXX_add_inbound_messages.up.sql` | Create | Таблица `inbound_messages` |
| `internal/services/smpp-gateway/handler/mo_handler.go` | Create | SMPP MO handler |
| `internal/services/smpp-gateway/queue/inbound_publisher.go` | Create | Kafka publisher для sms.inbound |
| `internal/services/messaging/application/inbound_service.go` | Create | Сервис обработки входящих |
| `internal/services/messaging/infrastructure/repository/inbound_repository.go` | Create | Репозиторий inbound_messages |
| `internal/services/messaging/grpc/server.go` | Modify | GetInboundMessages RPC |
| `api/proto/messaging/messaging.proto` | Modify | InboundMessage + GetInboundMessages |
| `internal/gateway/client/handlers/sms.go` | Modify | GET /api/v1/sms/inbound |
| `internal/gateway/client/router/router.go` | Modify | Зарегистрировать маршрут |
| `internal/services/webhook/application/webhook_service.go` | Modify | Добавить inbound событие |
| `api/openapi/openapi.yaml` | Modify | Документировать /sms/inbound + inbound webhook |
| `portal-frontend/src/pages/messages/MessagesPage.tsx` | Modify | Вкладка "Входящие" |

---

## Task 1: DB migration — таблица inbound_messages

**Files:**
- Create: `migrations/XXXXXX_add_inbound_messages.up.sql`
- Create: `migrations/XXXXXX_add_inbound_messages.down.sql`

> **Перед написанием:** прочитай последнюю миграцию в `migrations/` чтобы узнать текущий номер. Назови файл `{следующий_номер}_add_inbound_messages.up.sql`.

- [ ] **Step 1.1: Создать up-миграцию**

```sql
-- migrations/XXXXXX_add_inbound_messages.up.sql
BEGIN;

CREATE TABLE IF NOT EXISTS inbound_messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Привязка к клиенту через virtual number
    virtual_number  VARCHAR(20) NOT NULL,  -- номер/short code, на который пришло MO
    client_id       UUID,                  -- NULL если номер не привязан к клиенту
    -- Данные сообщения
    source          VARCHAR(20) NOT NULL,  -- номер отправителя (телефон пользователя)
    destination     VARCHAR(20) NOT NULL,  -- наш virtual number
    text            TEXT NOT NULL DEFAULT '',
    encoding        VARCHAR(20) DEFAULT 'GSM7',
    -- Метаданные
    smpp_message_id VARCHAR(255),
    provider_id     UUID,
    -- Статус обработки
    status          VARCHAR(50) NOT NULL DEFAULT 'received',
    -- CONSTRAINT: received, processed, failed
    -- Timestamps
    received_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_inbound_messages_client_id
    ON inbound_messages(client_id, received_at DESC);

CREATE INDEX idx_inbound_messages_virtual_number
    ON inbound_messages(virtual_number, received_at DESC);

CREATE INDEX idx_inbound_messages_source
    ON inbound_messages(source, received_at DESC);

-- Таблица virtual numbers (mapping номер → клиент)
CREATE TABLE IF NOT EXISTS virtual_numbers (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    number      VARCHAR(20) NOT NULL UNIQUE,
    client_id   UUID NOT NULL,
    description VARCHAR(255),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_virtual_numbers_client_id ON virtual_numbers(client_id);

COMMIT;
```

- [ ] **Step 1.2: Создать down-миграцию**

```sql
-- migrations/XXXXXX_add_inbound_messages.down.sql
BEGIN;
DROP TABLE IF EXISTS inbound_messages;
DROP TABLE IF EXISTS virtual_numbers;
COMMIT;
```

- [ ] **Step 1.3: Применить миграцию**

```bash
# Найди команду в scripts/server.sh или Makefile
make migrate
# или
scripts/server.sh migrate
```

Ожидаем: миграция применена без ошибок.

- [ ] **Step 1.4: Коммит**

```bash
git add migrations/
git commit -m "feat(db): add inbound_messages and virtual_numbers tables"
```

---

## Task 2: Kafka topic + InboundMessage модель

**Files:**
- Modify: `internal/queue/` (найти где определяются topic names)
- Modify или Create: shared message type для inbound

- [ ] **Step 2.1: Добавить Kafka topic константу**

Найди файл с Kafka topic константами (проверь `internal/queue/topics.go`, `internal/queue/producer.go` или конфиг). Добавить:

```go
// Inbound (Mobile Originated) messages from SMPP providers
TopicInbound = "sms.inbound"
```

- [ ] **Step 2.2: Определить InboundMessage структуру**

Найди где определены Kafka message structs (скорее всего в `internal/queue/` или `pkg/shared/`). Добавить:

```go
// InboundKafkaMessage represents a Mobile Originated SMS in Kafka.
type InboundKafkaMessage struct {
    ID            string    `json:"id"`
    VirtualNumber string    `json:"virtual_number"` // наш номер
    ClientID      string    `json:"client_id,omitempty"`
    Source        string    `json:"source"`     // телефон отправителя
    Destination   string    `json:"destination"` // наш номер (дублирует VirtualNumber)
    Text          string    `json:"text"`
    Encoding      string    `json:"encoding"`
    SMPPMessageID string    `json:"smpp_message_id,omitempty"`
    ProviderID    string    `json:"provider_id,omitempty"`
    ReceivedAt    time.Time `json:"received_at"`
    TraceID       string    `json:"trace_id,omitempty"`
}
```

- [ ] **Step 2.3: Добавить PublishInbound в Producer**

В `internal/queue/producer.go` (или аналогичный файл):

```go
func (p *Producer) PublishInbound(ctx context.Context, msg *InboundKafkaMessage) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return fmt.Errorf("marshal inbound message: %w", err)
    }
    kafkaMsg := &sarama.ProducerMessage{
        Topic: p.config.TopicInbound,
        Key:   sarama.StringEncoder(msg.ID),
        Value: sarama.ByteEncoder(data),
    }
    _, _, err = p.producer.SendMessage(kafkaMsg)
    return err
}
```

- [ ] **Step 2.4: Коммит**

```bash
git add internal/queue/
git commit -m "feat(queue): add sms.inbound Kafka topic and InboundKafkaMessage type"
```

---

## Task 3: SMPP Gateway MO handler

**Files:**
- Create: `internal/services/smpp-gateway/handler/mo_handler.go`
- Test: `internal/services/smpp-gateway/handler/mo_handler_test.go`

> **Перед написанием:** прочитай `internal/services/smpp-gateway/` чтобы понять структуру. Найди где обрабатываются исходящие сообщения (MT handler) и используй как образец.

- [ ] **Step 3.1: Написать failing тест**

```go
// internal/services/smpp-gateway/handler/mo_handler_test.go
package handler_test

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"
)

type MockInboundPublisher struct {
    mock.Mock
}

func (m *MockInboundPublisher) PublishInbound(ctx context.Context, msg *queue.InboundKafkaMessage) error {
    args := m.Called(ctx, msg)
    return args.Error(0)
}

func TestMOHandler_Handle(t *testing.T) {
    mockPub := new(MockInboundPublisher)
    handler := NewMOHandler(mockPub)

    mockPub.On("PublishInbound", mock.Anything, mock.MatchedBy(func(msg *queue.InboundKafkaMessage) bool {
        return msg.Source == "+79991234567" &&
            msg.Destination == "+79001112233" &&
            msg.Text == "STOP"
    })).Return(nil)

    err := handler.Handle(context.Background(), MOMessage{
        Source:      "+79991234567",
        Destination: "+79001112233",
        Text:        "STOP",
        ReceivedAt:  time.Now(),
    })

    require.NoError(t, err)
    mockPub.AssertExpectations(t)
}
```

- [ ] **Step 3.2: Запустить тест — убедиться, что падает**

```bash
go test ./internal/services/smpp-gateway/handler/... -run TestMOHandler -v
```

Ожидаем: `FAIL`

- [ ] **Step 3.3: Реализовать MO handler**

```go
// internal/services/smpp-gateway/handler/mo_handler.go
package handler

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"
)

// MOMessage represents a Mobile Originated (inbound) SMS received from a provider.
type MOMessage struct {
    Source        string
    Destination   string
    Text          string
    Encoding      string
    SMPPMessageID string
    ProviderID    string
    ReceivedAt    time.Time
}

// InboundPublisher publishes inbound messages to Kafka.
type InboundPublisher interface {
    PublishInbound(ctx context.Context, msg *queue.InboundKafkaMessage) error
}

// MOHandler processes Mobile Originated SMS messages from SMPP providers.
type MOHandler struct {
    publisher InboundPublisher
    logger    zerolog.Logger
}

func NewMOHandler(publisher InboundPublisher) *MOHandler {
    return &MOHandler{publisher: publisher}
}

// Handle processes a single MO message: publishes to Kafka for downstream processing.
func (h *MOHandler) Handle(ctx context.Context, mo MOMessage) error {
    if mo.ReceivedAt.IsZero() {
        mo.ReceivedAt = time.Now().UTC()
    }
    if mo.Encoding == "" {
        mo.Encoding = "GSM7"
    }

    msg := &queue.InboundKafkaMessage{
        ID:            uuid.New().String(),
        VirtualNumber: mo.Destination,
        Source:        mo.Source,
        Destination:   mo.Destination,
        Text:          mo.Text,
        Encoding:      mo.Encoding,
        SMPPMessageID: mo.SMPPMessageID,
        ProviderID:    mo.ProviderID,
        ReceivedAt:    mo.ReceivedAt,
    }

    if err := h.publisher.PublishInbound(ctx, msg); err != nil {
        return fmt.Errorf("publish inbound message: %w", err)
    }

    h.logger.Info().
        Str("source", mo.Source).
        Str("destination", mo.Destination).
        Str("smpp_message_id", mo.SMPPMessageID).
        Msg("MO message published to Kafka")

    return nil
}
```

- [ ] **Step 3.4: Подключить MOHandler к SMPP session**

Найди в `internal/services/smpp-gateway/` место где обрабатывается входящий SMPP PDU (ищи `deliver_sm` или `DeliverSM` handler). Интегрировать `MOHandler.Handle()`:

```go
// В существующем SMPP session handler, при получении deliver_sm PDU:
case *smpp.DeliverSm:
    mo := handler.MOMessage{
        Source:        pdu.SourceAddr,
        Destination:   pdu.DestinationAddr,
        Text:          string(pdu.ShortMessage),
        SMPPMessageID: pdu.MessageID,
        ReceivedAt:    time.Now().UTC(),
    }
    if err := s.moHandler.Handle(ctx, mo); err != nil {
        s.logger.Error().Err(err).Msg("failed to handle MO message")
    }
```

- [ ] **Step 3.5: Запустить тесты**

```bash
go test ./internal/services/smpp-gateway/... -v
```

Ожидаем: `PASS`

- [ ] **Step 3.6: Коммит**

```bash
git add internal/services/smpp-gateway/handler/
git commit -m "feat(smpp-gateway): add MO handler for inbound SMS"
```

---

## Task 4: Inbound Message Service (Kafka consumer)

**Files:**
- Create: `internal/services/messaging/infrastructure/repository/inbound_repository.go`
- Create: `internal/services/messaging/application/inbound_service.go`
- Test: `internal/services/messaging/application/inbound_service_test.go`

- [ ] **Step 4.1: Написать failing тест для сервиса**

```go
// internal/services/messaging/application/inbound_service_test.go
func TestInboundService_ProcessMessage(t *testing.T) {
    mockRepo := new(MockInboundRepository)
    mockWebhook := new(MockWebhookNotifier)
    mockVirtualNumbers := new(MockVirtualNumberRepository)
    svc := NewInboundService(mockRepo, mockWebhook, mockVirtualNumbers)

    clientID := uuid.New()
    msg := &queue.InboundKafkaMessage{
        ID:            uuid.New().String(),
        VirtualNumber: "+79001112233",
        Source:        "+79991234567",
        Destination:   "+79001112233",
        Text:          "Hello",
        ReceivedAt:    time.Now().UTC(),
    }

    // virtual number принадлежит clientID
    mockVirtualNumbers.On("FindByNumber", mock.Anything, "+79001112233").
        Return(&domain.VirtualNumber{ClientID: clientID}, nil)

    mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.InboundMessage) bool {
        return m.Source == "+79991234567" && m.ClientID == clientID
    })).Return(nil)

    mockWebhook.On("NotifyInbound", mock.Anything, clientID, mock.Anything).Return(nil)

    err := svc.ProcessMessage(context.Background(), msg)
    require.NoError(t, err)
    mockRepo.AssertExpectations(t)
    mockWebhook.AssertExpectations(t)
}
```

- [ ] **Step 4.2: Запустить тест — убедиться, что падает**

```bash
go test ./internal/services/messaging/application/... -run TestInboundService -v
```

- [ ] **Step 4.3: Реализовать InboundService**

```go
// internal/services/messaging/application/inbound_service.go
package application

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
)

type InboundService struct {
    inboundRepo    domain.InboundMessageRepository
    webhookNotify  domain.InboundWebhookNotifier
    virtualNumbers domain.VirtualNumberRepository
}

func NewInboundService(
    inboundRepo domain.InboundMessageRepository,
    webhookNotify domain.InboundWebhookNotifier,
    virtualNumbers domain.VirtualNumberRepository,
) *InboundService {
    return &InboundService{
        inboundRepo:    inboundRepo,
        webhookNotify:  webhookNotify,
        virtualNumbers: virtualNumbers,
    }
}

// ProcessMessage handles an inbound Kafka message: saves to DB and triggers webhook.
func (s *InboundService) ProcessMessage(ctx context.Context, kafkaMsg *queue.InboundKafkaMessage) error {
    // Найти клиента по virtual number
    vn, err := s.virtualNumbers.FindByNumber(ctx, kafkaMsg.VirtualNumber)
    if err != nil {
        // Число не привязано к клиенту — сохраняем без client_id, не шлём webhook
        return s.saveWithoutClient(ctx, kafkaMsg)
    }

    inbound := &domain.InboundMessage{
        ID:            uuid.MustParse(kafkaMsg.ID),
        VirtualNumber: kafkaMsg.VirtualNumber,
        ClientID:      vn.ClientID,
        Source:        kafkaMsg.Source,
        Destination:   kafkaMsg.Destination,
        Text:          kafkaMsg.Text,
        Encoding:      kafkaMsg.Encoding,
        SMPPMessageID: kafkaMsg.SMPPMessageID,
        Status:        "received",
        ReceivedAt:    kafkaMsg.ReceivedAt,
        CreatedAt:     time.Now().UTC(),
    }

    if err := s.inboundRepo.Create(ctx, inbound); err != nil {
        return fmt.Errorf("save inbound message: %w", err)
    }

    // Доставить через webhook клиенту (асинхронно — не блокируем pipeline)
    go func() {
        if err := s.webhookNotify.NotifyInbound(context.Background(), vn.ClientID, inbound); err != nil {
            // Webhook delivery failure не должна ломать pipeline
            // Webhook service имеет свой retry mechanism
        }
    }()

    return nil
}

func (s *InboundService) saveWithoutClient(ctx context.Context, kafkaMsg *queue.InboundKafkaMessage) error {
    inbound := &domain.InboundMessage{
        ID:            uuid.MustParse(kafkaMsg.ID),
        VirtualNumber: kafkaMsg.VirtualNumber,
        Source:        kafkaMsg.Source,
        Destination:   kafkaMsg.Destination,
        Text:          kafkaMsg.Text,
        Status:        "received",
        ReceivedAt:    kafkaMsg.ReceivedAt,
        CreatedAt:     time.Now().UTC(),
    }
    return s.inboundRepo.Create(ctx, inbound)
}
```

- [ ] **Step 4.4: Реализовать InboundRepository**

```go
// internal/services/messaging/infrastructure/repository/inbound_repository.go
package repository

import "github.com/jackc/pgx/v5/pgxpool"

type InboundRepository struct {
    db *pgxpool.Pool
}

func NewInboundRepository(db *pgxpool.Pool) *InboundRepository {
    return &InboundRepository{db: db}
}

func (r *InboundRepository) Create(ctx context.Context, msg *domain.InboundMessage) error {
    const q = `
        INSERT INTO inbound_messages
            (id, virtual_number, client_id, source, destination, text, encoding,
             smpp_message_id, provider_id, status, received_at, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

    var clientID interface{}
    if msg.ClientID != uuid.Nil {
        clientID = msg.ClientID
    }

    _, err := r.db.Exec(ctx, q,
        msg.ID, msg.VirtualNumber, clientID,
        msg.Source, msg.Destination, msg.Text, msg.Encoding,
        msg.SMPPMessageID, msg.ProviderID, msg.Status,
        msg.ReceivedAt, msg.CreatedAt,
    )
    return err
}

func (r *InboundRepository) ListByClient(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.InboundMessage, int, error) {
    const countQ = `SELECT COUNT(*) FROM inbound_messages WHERE client_id = $1`
    const listQ = `
        SELECT id, virtual_number, client_id, source, destination, text, encoding,
               smpp_message_id, status, received_at, created_at
        FROM inbound_messages
        WHERE client_id = $1
        ORDER BY received_at DESC
        LIMIT $2 OFFSET $3`

    var total int
    if err := r.db.QueryRow(ctx, countQ, clientID).Scan(&total); err != nil {
        return nil, 0, err
    }

    rows, err := r.db.Query(ctx, listQ, clientID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var messages []*domain.InboundMessage
    for rows.Next() {
        m := &domain.InboundMessage{}
        if err := rows.Scan(
            &m.ID, &m.VirtualNumber, &m.ClientID, &m.Source,
            &m.Destination, &m.Text, &m.Encoding,
            &m.SMPPMessageID, &m.Status, &m.ReceivedAt, &m.CreatedAt,
        ); err != nil {
            return nil, 0, err
        }
        messages = append(messages, m)
    }
    return messages, total, rows.Err()
}
```

- [ ] **Step 4.5: Запустить тесты**

```bash
go test ./internal/services/messaging/application/... -run TestInboundService -v
go test ./internal/services/messaging/infrastructure/... -v
```

Ожидаем: `PASS`

- [ ] **Step 4.6: Коммит**

```bash
git add internal/services/messaging/application/inbound_service.go
git add internal/services/messaging/application/inbound_service_test.go
git add internal/services/messaging/infrastructure/repository/inbound_repository.go
git commit -m "feat(messaging): add InboundService and InboundRepository"
```

---

## Task 5: Kafka consumer для sms.inbound

**Files:**
- Create или Modify: `internal/services/messaging/infrastructure/queue/inbound_consumer.go`

- [ ] **Step 5.1: Создать Kafka consumer для sms.inbound**

> Найди существующий consumer в `internal/services/messaging/infrastructure/queue/` — используй как образец (topic, group ID, error handling).

```go
// internal/services/messaging/infrastructure/queue/inbound_consumer.go
package queue

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/IBM/sarama"
    "github.com/rs/zerolog"
)

type InboundMessageProcessor interface {
    ProcessMessage(ctx context.Context, msg *InboundKafkaMessage) error
}

type InboundConsumer struct {
    consumer  sarama.ConsumerGroup
    topic     string
    processor InboundMessageProcessor
    logger    zerolog.Logger
}

func NewInboundConsumer(
    brokers []string,
    groupID string,
    topic string,
    processor InboundMessageProcessor,
    logger zerolog.Logger,
) (*InboundConsumer, error) {
    config := sarama.NewConfig()
    config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
    config.Consumer.Offsets.Initial = sarama.OffsetNewest

    cg, err := sarama.NewConsumerGroup(brokers, groupID, config)
    if err != nil {
        return nil, fmt.Errorf("create inbound consumer group: %w", err)
    }

    return &InboundConsumer{
        consumer:  cg,
        topic:     topic,
        processor: processor,
        logger:    logger,
    }, nil
}

func (c *InboundConsumer) Start(ctx context.Context) error {
    handler := &inboundHandler{processor: c.processor, logger: c.logger}
    for {
        if err := c.consumer.Consume(ctx, []string{c.topic}, handler); err != nil {
            if ctx.Err() != nil {
                return nil // graceful shutdown
            }
            c.logger.Error().Err(err).Msg("inbound consumer error")
        }
    }
}

func (c *InboundConsumer) Close() error {
    return c.consumer.Close()
}

type inboundHandler struct {
    processor InboundMessageProcessor
    logger    zerolog.Logger
}

func (h *inboundHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *inboundHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *inboundHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    for msg := range claim.Messages() {
        var km InboundKafkaMessage
        if err := json.Unmarshal(msg.Value, &km); err != nil {
            h.logger.Error().Err(err).Msg("failed to unmarshal inbound kafka message")
            session.MarkMessage(msg, "")
            continue
        }

        if err := h.processor.ProcessMessage(session.Context(), &km); err != nil {
            h.logger.Error().Err(err).
                Str("id", km.ID).
                Str("source", km.Source).
                Msg("failed to process inbound message")
            // Commit anyway — bad messages go to monitoring; don't block the consumer
        }

        session.MarkMessage(msg, "")
    }
    return nil
}
```

- [ ] **Step 5.2: Зарегистрировать consumer в messaging service main**

В `cmd/services/messaging-service/main.go` добавить инициализацию и запуск InboundConsumer рядом с другими consumers:

```go
inboundConsumer, err := queue.NewInboundConsumer(
    cfg.Kafka.Brokers,
    "messaging-service-inbound",
    cfg.Kafka.TopicInbound,
    inboundService,
    logger,
)
if err != nil {
    logger.Fatal().Err(err).Msg("failed to create inbound consumer")
}

go func() {
    if err := inboundConsumer.Start(ctx); err != nil {
        logger.Error().Err(err).Msg("inbound consumer stopped")
    }
}()

// В shutdown:
defer inboundConsumer.Close()
```

- [ ] **Step 5.3: Собрать проект**

```bash
go build ./...
```

- [ ] **Step 5.4: Коммит**

```bash
git add internal/services/messaging/infrastructure/queue/inbound_consumer.go
git add cmd/services/messaging-service/main.go
git commit -m "feat(messaging): add Kafka consumer for sms.inbound topic"
```

---

## Task 6: HTTP API — GET /api/v1/sms/inbound

**Files:**
- Modify: `internal/services/messaging/grpc/server.go` + proto
- Modify: `internal/gateway/client/handlers/sms.go`
- Modify: `internal/gateway/client/router/router.go`

> Следуй точно такому же паттерну как в Phase 1 Task 3-5 (ListScheduledMessages). Создай proto RPC `GetInboundMessages`, gRPC handler, HTTP handler, зарегистрируй роут.

- [ ] **Step 6.1: Добавить proto RPC GetInboundMessages**

В `api/proto/messaging/messaging.proto`:

```protobuf
rpc GetInboundMessages(GetInboundMessagesRequest) returns (GetInboundMessagesResponse);

message GetInboundMessagesRequest {
  string client_id = 1;
  int32 limit = 2;
  int32 offset = 3;
  string source = 4;        // optional: filter by sender number
  google.protobuf.Timestamp from = 5; // optional: start date
  google.protobuf.Timestamp to = 6;   // optional: end date
}

message GetInboundMessagesResponse {
  repeated InboundMessageInfo messages = 1;
  int32 total = 2;
  int32 limit = 3;
  int32 offset = 4;
}

message InboundMessageInfo {
  string id = 1;
  string virtual_number = 2;
  string source = 3;
  string destination = 4;
  string text = 5;
  string status = 6;
  google.protobuf.Timestamp received_at = 7;
}
```

- [ ] **Step 6.2: Регенерировать pb.go**

```bash
make proto  # или аналогичная команда из Makefile
```

- [ ] **Step 6.3: Написать тест gRPC handler**

По образцу из Phase 1 Task 4.1. Проверить:
- Валидный запрос возвращает список сообщений
- Невалидный client_id возвращает `codes.InvalidArgument`

- [ ] **Step 6.4: Реализовать gRPC handler** 

По образцу `ListScheduledMessages` в `server.go`.

- [ ] **Step 6.5: Написать тест HTTP handler**

По образцу из Phase 1 Task 5.1. Маршрут: `GET /api/v1/sms/inbound`.

- [ ] **Step 6.6: Реализовать HTTP handler + роут**

```go
// GET /api/v1/sms/inbound?limit=100&offset=0&source=%2B7999...&from=...&to=...
func (h *SMSHandlers) GetInbound(w http.ResponseWriter, r *http.Request) {
    // Аналогично GetHistory, но вызывает GetInboundMessages RPC
    // Возвращает JSON с полями: messages[], total, limit, offset
}
```

Зарегистрировать роут:
```go
r.Handle("/api/v1/sms/inbound", authMiddleware(http.HandlerFunc(h.sms.GetInbound))).Methods(http.MethodGet)
```

- [ ] **Step 6.7: Коммит**

```bash
git add api/proto/ internal/services/messaging/grpc/server.go
git add internal/gateway/client/handlers/sms.go internal/gateway/client/router/router.go
git commit -m "feat(messaging): add GET /api/v1/sms/inbound endpoint"
```

---

## Task 7: Webhook для входящих сообщений

**Files:**
- Modify: `internal/services/webhook/application/webhook_service.go`

- [ ] **Step 7.1: Добавить тип события inbound**

Найди где определены webhook event types. Добавить:

```go
const EventInboundMessage = "inbound_message"
```

- [ ] **Step 7.2: Написать тест NotifyInbound**

```go
func TestWebhookService_NotifyInbound(t *testing.T) {
    // Используй существующий паттерн тестов webhook service
    mockDelivery := new(MockWebhookDelivery)
    svc := NewWebhookService(mockDelivery)

    clientID := uuid.New()
    msg := &domain.InboundMessage{
        ID:          uuid.New(),
        Source:      "+79991234567",
        Destination: "+79001112233",
        Text:        "Hello",
        ReceivedAt:  time.Now(),
    }

    mockDelivery.On("Deliver", mock.Anything, clientID, EventInboundMessage, mock.MatchedBy(func(payload map[string]interface{}) bool {
        return payload["event"] == "inbound_message" &&
            payload["source"] == "+79991234567"
    })).Return(nil)

    err := svc.NotifyInbound(context.Background(), clientID, msg)
    require.NoError(t, err)
    mockDelivery.AssertExpectations(t)
}
```

- [ ] **Step 7.3: Реализовать NotifyInbound**

```go
func (s *WebhookService) NotifyInbound(ctx context.Context, clientID uuid.UUID, msg *domain.InboundMessage) error {
    payload := map[string]interface{}{
        "event":          EventInboundMessage,
        "id":             msg.ID.String(),
        "source":         msg.Source,
        "destination":    msg.Destination,
        "text":           msg.Text,
        "virtual_number": msg.VirtualNumber,
        "received_at":    msg.ReceivedAt.Format(time.RFC3339),
    }
    return s.delivery.Deliver(ctx, clientID, EventInboundMessage, payload)
}
```

- [ ] **Step 7.4: Запустить тесты webhook**

```bash
go test ./internal/services/webhook/... -v
```

- [ ] **Step 7.5: Коммит**

```bash
git add internal/services/webhook/
git commit -m "feat(webhook): add inbound_message webhook event"
```

---

## Task 8: OpenAPI spec + frontend

- [ ] **Step 8.1: Добавить /sms/inbound в openapi.yaml**

По образцу `/sms/history`. Включить query params: `limit`, `offset`, `source`, `from`, `to`.

- [ ] **Step 8.2: Frontend — вкладка "Входящие"**

Найти страницу сообщений в `portal-frontend/src/pages/messages/`. Добавить вкладку "Входящие" (tab) рядом с "Исходящие", которая:
- Вызывает `GET /api/v1/sms/inbound`
- Показывает таблицу: Откуда, Наш номер, Текст, Получено
- Поддерживает фильтрацию по источнику и дате

- [ ] **Step 8.3: Финальная сборка**

```bash
go build ./...
cd portal-frontend && npm run build
```

- [ ] **Step 8.4: Финальный коммит**

```bash
git add api/openapi/openapi.yaml portal-frontend/src/
git commit -m "feat: complete inbound SMS Phase 4 - API, consumer, webhook, frontend"
```
