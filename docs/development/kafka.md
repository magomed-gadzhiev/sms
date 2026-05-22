# Kafka интеграция

## Обзор

Проект использует Apache Kafka для асинхронной обработки SMS сообщений. Kafka обеспечивает гарантированную доставку сообщений и масштабируемость системы.

## Topics

Система использует три основных топика:

- `sms.outgoing` - исходящие SMS сообщения для отправки в SMSC провайдеры
- `sms.dlr` - delivery receipts от SMSC провайдеров
- `sms.failed` - сообщения об ошибках (Dead Letter Queue)

## Producer

Producer используется для публикации сообщений в Kafka топики.

### Инициализация

```go
import (
    "github.com/smpp-server/smpp-server/internal/config"
    "github.com/smpp-server/smpp-server/internal/queue"
)

cfg := &config.KafkaConfig{
    Brokers: []string{"localhost:9092"},
    TopicOutgoing: "sms.outgoing",
    TopicDLR: "sms.dlr",
    TopicFailed: "sms.failed",
    MaxRetries: 3,
    RetryBackoff: 1 * time.Second,
}

producer, err := queue.NewProducer(cfg)
if err != nil {
    log.Fatal(err)
}
defer producer.Close()
```

### Публикация исходящего сообщения

```go
kafkaMsg := queue.FromMessage(message)
err := producer.PublishOutgoing(ctx, kafkaMsg)
if err != nil {
    log.Error().Err(err).Msg("ошибка публикации сообщения")
}
```

### Публикация DLR

```go
dlr := &queue.DLRMessage{
    MessageID:     messageID,
    SMPPMessageID: smppMessageID,
    Stat:          "DELIVRD",
    DoneDate:      &doneDate,
}

err := producer.PublishDLR(ctx, dlr)
if err != nil {
    log.Error().Err(err).Msg("ошибка публикации DLR")
}
```

### Публикация failed сообщения

```go
failed := &queue.FailedMessage{
    MessageID:    messageID,
    KafkaMessage: kafkaMsg,
    Error:        "provider connection failed",
    ErrorCode:    "PROVIDER_ERROR",
    RetryCount:   5,
    FailedAt:     time.Now(),
}

err := producer.PublishFailed(ctx, failed)
if err != nil {
    log.Error().Err(err).Msg("ошибка публикации failed сообщения")
}
```

## Consumer

Consumer используется для чтения и обработки сообщений из Kafka топиков.

### Инициализация

```go
cfg := &config.KafkaConfig{
    Brokers:           []string{"localhost:9092"},
    ConsumerGroup:     "smpp-worker",
    SessionTimeout:    30 * time.Second,
    HeartbeatInterval: 10 * time.Second,
}

// Handler для исходящих сообщений
outgoingHandler := func(ctx context.Context, msg *queue.KafkaMessage) error {
    // Обработка сообщения
    return processMessage(ctx, msg)
}

// Handler для DLR
dlrHandler := func(ctx context.Context, dlr *queue.DLRMessage) error {
    // Обработка DLR
    return processDLR(ctx, dlr)
}

// Handler для failed сообщений
failedHandler := func(ctx context.Context, failed *queue.FailedMessage) error {
    // Обработка failed сообщений
    return processFailed(ctx, failed)
}

consumer, err := queue.NewConsumer(cfg, outgoingHandler, dlrHandler, failedHandler)
if err != nil {
    log.Fatal(err)
}
defer consumer.Close()
```

### Запуск потребления

```go
// Потребление исходящих сообщений
go func() {
    if err := consumer.ConsumeOutgoing(); err != nil {
        log.Error().Err(err).Msg("ошибка потребления outgoing сообщений")
    }
}()

// Потребление DLR
go func() {
    if err := consumer.ConsumeDLR(); err != nil {
        log.Error().Err(err).Msg("ошибка потребления DLR")
    }
}()

// Потребление failed сообщений
go func() {
    if err := consumer.ConsumeFailed(); err != nil {
        log.Error().Err(err).Msg("ошибка потребления failed сообщений")
    }
}()
```

## Структуры сообщений

### KafkaMessage

Основная структура для исходящих SMS сообщений:

```go
type KafkaMessage struct {
    ID          string
    MessageID   uuid.UUID
    Source      string
    Destination string
    Text        string
    ProviderID  *uuid.UUID
    RouteID     *uuid.UUID
    ClientID    *uuid.UUID
    Priority    int
    RetryCount  int
    MaxRetries  int
    CreatedAt   time.Time
    Metadata    map[string]interface{}
}
```

### DLRMessage

Структура для delivery receipts:

```go
type DLRMessage struct {
    MessageID         uuid.UUID
    SMPPMessageID     string
    ProviderID        *uuid.UUID
    ReceiptedMessageID string
    SubmitDate        *time.Time
    DoneDate          *time.Time
    Stat              string
    Err               *int
    Text              string
    Source            string
    Destination       string
    CreatedAt         time.Time
}
```

### FailedMessage

Структура для сообщений об ошибках:

```go
type FailedMessage struct {
    MessageID    uuid.UUID
    KafkaMessage *KafkaMessage
    Error        string
    ErrorCode    string
    RetryCount   int
    FailedAt     time.Time
    Metadata     map[string]interface{}
}
```

## Retry механизм

Producer автоматически повторяет публикацию сообщений при ошибках:

- Максимальное количество попыток настраивается через `MaxRetries`
- Задержка между попытками настраивается через `RetryBackoff`
- Используется экспоненциальная задержка: `backoff = attempt * RetryBackoff`

## Graceful shutdown

Consumer поддерживает graceful shutdown:

```go
// Остановка consumer
consumer.Close() // Блокирует до завершения обработки текущих сообщений
```

## Мониторинг

Producer и Consumer логируют все операции с использованием zerolog:

- Успешная публикация/обработка сообщений (debug уровень)
- Ошибки публикации/обработки (error уровень)
- Метрики производительности (duration, partition, offset)

## Конфигурация

Настройки Kafka находятся в секции `kafka` конфигурационного файла:

```yaml
kafka:
  brokers:
    - localhost:9092
  topic_outgoing: sms.outgoing
  topic_dlr: sms.dlr
  topic_failed: sms.failed
  consumer_group: smpp-worker
  session_timeout: 30s
  heartbeat_interval: 10s
  max_retries: 3
  retry_backoff: 1s
```

## Best Practices

1. **Используйте уникальные consumer groups** для разных сервисов
2. **Обрабатывайте ошибки** в handlers - не помечайте сообщения как обработанные при ошибках
3. **Используйте graceful shutdown** для корректного завершения работы
4. **Мониторьте метрики** Kafka для отслеживания производительности
5. **Настройте retention policy** для топиков в зависимости от требований
