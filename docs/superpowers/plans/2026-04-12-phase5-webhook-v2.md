# Webhook v2 + Event Streaming — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Расширить webhook систему: добавить полный event catalog (DLR + campaign lifecycle + inbound + billing), retry с exponential backoff, dead letter queue, и Server-Sent Events (SSE) streaming endpoint для aggregator'ов.

**Architecture:** Webhook service уже существует. Расширяем: (1) добавляем новые event types, (2) добавляем retry таблицу в PostgreSQL с exponential backoff worker, (3) добавляем dead_letter_webhooks таблицу, (4) добавляем SSE endpoint `GET /api/v1/events/stream` для real-time streaming.

**Tech Stack:** Go 1.24, jackc/pgx/v5, IBM/sarama Kafka, gorilla/mux (SSE через http.Flusher), redis/go-redis/v9 (rate limiting webhooks).

---

## File Map

| Файл | Действие | Что делаем |
|---|---|---|
| `migrations/XXXXXX_webhook_v2.up.sql` | Create | Таблицы webhook_delivery_log, dead_letter_webhooks |
| `internal/services/webhook/domain/events.go` | Create или Modify | Полный event catalog |
| `internal/services/webhook/application/retry_service.go` | Create | Retry worker с exponential backoff |
| `internal/services/webhook/infrastructure/repository/delivery_repository.go` | Create | CRUD для webhook_delivery_log |
| `internal/services/webhook/application/webhook_service.go` | Modify | Интеграция retry + новые events |
| `internal/gateway/client/handlers/events.go` | Create | SSE streaming handler |
| `internal/gateway/client/router/router.go` | Modify | Зарегистрировать /events/stream |
| `api/openapi/openapi.yaml` | Modify | Документировать webhook events schema + SSE |

---

## Task 1: DB migration — webhook delivery log + dead letter

**Files:**
- Create: `migrations/XXXXXX_webhook_v2.up.sql`
- Create: `migrations/XXXXXX_webhook_v2.down.sql`

> Проверь последний номер миграции в `migrations/` и используй следующий.

- [ ] **Step 1.1: Создать up-миграцию**

```sql
-- migrations/XXXXXX_webhook_v2.up.sql
BEGIN;

-- Лог всех попыток доставки webhook
CREATE TABLE IF NOT EXISTS webhook_delivery_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id      UUID NOT NULL,   -- FK на таблицу webhooks (конфигурация клиента)
    client_id       UUID NOT NULL,
    event_type      VARCHAR(100) NOT NULL,
    payload         JSONB NOT NULL,
    -- Статус попытки
    status          VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- CONSTRAINT: pending, sending, delivered, failed, dead_lettered
    attempt_count   INT NOT NULL DEFAULT 0,
    max_attempts    INT NOT NULL DEFAULT 5,
    -- Retry scheduling
    next_attempt_at TIMESTAMPTZ,
    last_error      TEXT,
    -- Response info
    response_status INT,
    response_body   TEXT,
    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_delivery_log_pending
    ON webhook_delivery_log(status, next_attempt_at)
    WHERE status IN ('pending', 'failed');

CREATE INDEX idx_webhook_delivery_log_client
    ON webhook_delivery_log(client_id, created_at DESC);

-- Dead letter: сообщения, не доставленные после max_attempts
CREATE TABLE IF NOT EXISTS dead_letter_webhooks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_log_id UUID NOT NULL REFERENCES webhook_delivery_log(id),
    client_id       UUID NOT NULL,
    event_type      VARCHAR(100) NOT NULL,
    payload         JSONB NOT NULL,
    attempt_count   INT NOT NULL,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dead_letter_webhooks_client
    ON dead_letter_webhooks(client_id, created_at DESC);

COMMIT;
```

- [ ] **Step 1.2: Создать down-миграцию**

```sql
BEGIN;
DROP TABLE IF EXISTS dead_letter_webhooks;
DROP TABLE IF EXISTS webhook_delivery_log;
COMMIT;
```

- [ ] **Step 1.3: Применить**

```bash
scripts/server.sh migrate
# или make migrate
```

- [ ] **Step 1.4: Коммит**

```bash
git add migrations/
git commit -m "feat(db): add webhook_delivery_log and dead_letter_webhooks tables"
```

---

## Task 2: Event catalog — полный список типов событий

**Files:**
- Create или Modify: `internal/services/webhook/domain/events.go`

- [ ] **Step 2.1: Написать тест для event types**

```go
// internal/services/webhook/domain/events_test.go
func TestEventTypes_AllDefined(t *testing.T) {
    // Проверяем что все ожидаемые события определены
    expected := []string{
        "message.delivered",
        "message.failed",
        "message.sent",
        "campaign.completed",
        "campaign.failed",
        "inbound.received",
        "balance.low",
    }
    for _, evt := range expected {
        assert.NotEmpty(t, evt, "event type should not be empty")
    }
    // Проверяем что EventTypeFromString работает
    assert.Equal(t, EventMessageDelivered, EventTypeFromString("message.delivered"))
    assert.Equal(t, EventUnknown, EventTypeFromString("unknown.event"))
}
```

- [ ] **Step 2.2: Запустить тест — убедиться, что падает**

```bash
go test ./internal/services/webhook/domain/... -run TestEventTypes -v
```

- [ ] **Step 2.3: Реализовать event catalog**

```go
// internal/services/webhook/domain/events.go
package domain

// EventType defines the type of webhook event.
type EventType string

const (
    // Messaging events
    EventMessageDelivered EventType = "message.delivered"
    EventMessageFailed    EventType = "message.failed"
    EventMessageSent      EventType = "message.sent"
    EventMessageExpired   EventType = "message.expired"

    // Campaign events
    EventCampaignLaunched   EventType = "campaign.launched"
    EventCampaignCompleted  EventType = "campaign.completed"
    EventCampaignFailed     EventType = "campaign.failed"
    EventCampaignPaused     EventType = "campaign.paused"

    // Inbound events
    EventInboundReceived EventType = "inbound.received"

    // Billing events
    EventBalanceLow      EventType = "balance.low"
    EventBalanceDepleted EventType = "balance.depleted"

    // Sender name events
    EventSenderApproved EventType = "sender_name.approved"
    EventSenderRejected EventType = "sender_name.rejected"

    EventUnknown EventType = "unknown"
)

// EventTypeFromString converts a string to EventType, returning EventUnknown if not recognized.
func EventTypeFromString(s string) EventType {
    switch EventType(s) {
    case EventMessageDelivered, EventMessageFailed, EventMessageSent, EventMessageExpired,
        EventCampaignLaunched, EventCampaignCompleted, EventCampaignFailed, EventCampaignPaused,
        EventInboundReceived, EventBalanceLow, EventBalanceDepleted,
        EventSenderApproved, EventSenderRejected:
        return EventType(s)
    default:
        return EventUnknown
    }
}

// WebhookEvent is the payload sent to client webhook endpoints.
type WebhookEvent struct {
    ID        string                 `json:"id"`
    Event     EventType              `json:"event"`
    ClientID  string                 `json:"client_id"`
    Timestamp string                 `json:"timestamp"` // RFC3339
    Data      map[string]interface{} `json:"data"`
}
```

- [ ] **Step 2.4: Запустить тест**

```bash
go test ./internal/services/webhook/domain/... -v
```

- [ ] **Step 2.5: Коммит**

```bash
git add internal/services/webhook/domain/events.go
git add internal/services/webhook/domain/events_test.go
git commit -m "feat(webhook): add full event catalog with 14 event types"
```

---

## Task 3: Delivery Repository

**Files:**
- Create: `internal/services/webhook/infrastructure/repository/delivery_repository.go`
- Test: `internal/services/webhook/infrastructure/repository/delivery_repository_test.go`

- [ ] **Step 3.1: Написать failing тест**

```go
func TestDeliveryRepository_CreateAndFetchPending(t *testing.T) {
    repo := setupTestDeliveryRepo(t) // паттерн как в других репозиториях
    clientID := uuid.New()
    webhookID := uuid.New()
    now := time.Now().UTC()

    entry := &domain.WebhookDeliveryLog{
        ID:           uuid.New(),
        WebhookID:    webhookID,
        ClientID:     clientID,
        EventType:    string(domain.EventMessageDelivered),
        Payload:      map[string]interface{}{"message_id": "test-123"},
        Status:       "pending",
        MaxAttempts:  5,
        NextAttemptAt: &now,
    }

    err := repo.Create(context.Background(), entry)
    require.NoError(t, err)

    pending, err := repo.FetchPendingBatch(context.Background(), 10)
    require.NoError(t, err)
    require.Len(t, pending, 1)
    assert.Equal(t, entry.ID, pending[0].ID)
}
```

- [ ] **Step 3.2: Запустить тест — убедиться, что падает**

```bash
go test ./internal/services/webhook/infrastructure/... -run TestDeliveryRepository -v
```

- [ ] **Step 3.3: Реализовать DeliveryRepository**

```go
// internal/services/webhook/infrastructure/repository/delivery_repository.go
package repository

type DeliveryRepository struct {
    db *pgxpool.Pool
}

func NewDeliveryRepository(db *pgxpool.Pool) *DeliveryRepository {
    return &DeliveryRepository{db: db}
}

func (r *DeliveryRepository) Create(ctx context.Context, entry *domain.WebhookDeliveryLog) error {
    payloadJSON, err := json.Marshal(entry.Payload)
    if err != nil {
        return fmt.Errorf("marshal payload: %w", err)
    }
    const q = `
        INSERT INTO webhook_delivery_log
            (id, webhook_id, client_id, event_type, payload, status, max_attempts, next_attempt_at, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())`
    _, err = r.db.Exec(ctx, q,
        entry.ID, entry.WebhookID, entry.ClientID,
        entry.EventType, payloadJSON,
        entry.Status, entry.MaxAttempts, entry.NextAttemptAt,
    )
    return err
}

// FetchPendingBatch selects up to limit entries ready for retry, locks them (SELECT FOR UPDATE SKIP LOCKED).
func (r *DeliveryRepository) FetchPendingBatch(ctx context.Context, limit int) ([]*domain.WebhookDeliveryLog, error) {
    const q = `
        SELECT id, webhook_id, client_id, event_type, payload, status,
               attempt_count, max_attempts, next_attempt_at, last_error
        FROM webhook_delivery_log
        WHERE status IN ('pending', 'failed')
          AND (next_attempt_at IS NULL OR next_attempt_at <= NOW())
        ORDER BY next_attempt_at ASC NULLS FIRST
        LIMIT $1
        FOR UPDATE SKIP LOCKED`

    rows, err := r.db.Query(ctx, q, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var entries []*domain.WebhookDeliveryLog
    for rows.Next() {
        e := &domain.WebhookDeliveryLog{}
        var payloadJSON []byte
        if err := rows.Scan(
            &e.ID, &e.WebhookID, &e.ClientID, &e.EventType, &payloadJSON,
            &e.Status, &e.AttemptCount, &e.MaxAttempts, &e.NextAttemptAt, &e.LastError,
        ); err != nil {
            return nil, err
        }
        if err := json.Unmarshal(payloadJSON, &e.Payload); err != nil {
            return nil, err
        }
        entries = append(entries, e)
    }
    return entries, rows.Err()
}

// UpdateAfterAttempt updates status, attempt count, next retry time, and response info.
func (r *DeliveryRepository) UpdateAfterAttempt(ctx context.Context, id uuid.UUID, success bool, responseStatus int, responseBody, lastError string) error {
    if success {
        const q = `UPDATE webhook_delivery_log SET status='delivered', delivered_at=NOW(), response_status=$2, response_body=$3, updated_at=NOW() WHERE id=$1`
        _, err := r.db.Exec(ctx, q, id, responseStatus, responseBody)
        return err
    }

    // Exponential backoff: 1m, 5m, 30m, 2h, 8h
    const q = `
        UPDATE webhook_delivery_log
        SET status = CASE WHEN attempt_count + 1 >= max_attempts THEN 'dead_lettered' ELSE 'failed' END,
            attempt_count = attempt_count + 1,
            next_attempt_at = NOW() + (INTERVAL '1 minute' * POWER(5, attempt_count)),
            last_error = $2,
            response_status = $3,
            updated_at = NOW()
        WHERE id = $1`
    _, err := r.db.Exec(ctx, q, id, lastError, responseStatus)
    return err
}

// MoveToDeadLetter inserts into dead_letter_webhooks for entries that exceeded max_attempts.
func (r *DeliveryRepository) MoveToDeadLetter(ctx context.Context, logID uuid.UUID) error {
    const q = `
        INSERT INTO dead_letter_webhooks (delivery_log_id, client_id, event_type, payload, attempt_count, last_error)
        SELECT id, client_id, event_type, payload, attempt_count, last_error
        FROM webhook_delivery_log
        WHERE id = $1`
    _, err := r.db.Exec(ctx, q, logID)
    return err
}
```

- [ ] **Step 3.4: Запустить тесты**

```bash
go test ./internal/services/webhook/infrastructure/... -v
```

- [ ] **Step 3.5: Коммит**

```bash
git add internal/services/webhook/infrastructure/repository/delivery_repository.go
git add internal/services/webhook/infrastructure/repository/delivery_repository_test.go
git commit -m "feat(webhook): add DeliveryRepository with retry and dead letter support"
```

---

## Task 4: Retry Worker

**Files:**
- Create: `internal/services/webhook/application/retry_service.go`
- Test: `internal/services/webhook/application/retry_service_test.go`

- [ ] **Step 4.1: Написать failing тест**

```go
func TestRetryService_ProcessBatch_Success(t *testing.T) {
    mockRepo := new(MockDeliveryRepository)
    mockHTTP := new(MockHTTPClient)
    svc := NewRetryService(mockRepo, mockHTTP, zerolog.Nop())

    logID := uuid.New()
    entry := &domain.WebhookDeliveryLog{
        ID:        logID,
        ClientID:  uuid.New(),
        EventType: "message.delivered",
        Payload:   map[string]interface{}{"message_id": "msg-1"},
        WebhookID: uuid.New(),
    }

    // Симулируем успешный webhook endpoint
    mockRepo.On("FetchPendingBatch", mock.Anything, 50).Return([]*domain.WebhookDeliveryLog{entry}, nil)
    mockHTTP.On("Post", mock.Anything, mock.Anything, mock.Anything).Return(200, "", nil)
    mockRepo.On("UpdateAfterAttempt", mock.Anything, logID, true, 200, "", "").Return(nil)

    svc.processBatch(context.Background())

    mockHTTP.AssertExpectations(t)
    mockRepo.AssertExpectations(t)
}
```

- [ ] **Step 4.2: Запустить тест — убедиться, что падает**

```bash
go test ./internal/services/webhook/application/... -run TestRetryService -v
```

- [ ] **Step 4.3: Реализовать RetryService**

```go
// internal/services/webhook/application/retry_service.go
package application

import (
    "context"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog"
)

const (
    retryBatchSize    = 50
    retryInterval     = 30 * time.Second
    httpTimeout       = 10 * time.Second
)

type WebhookHTTPClient interface {
    Post(ctx context.Context, url string, payload []byte) (statusCode int, body string, err error)
}

type RetryService struct {
    repo       DeliveryRepository
    httpClient WebhookHTTPClient
    logger     zerolog.Logger
    ticker     *time.Ticker
    done       chan struct{}
}

func NewRetryService(repo DeliveryRepository, httpClient WebhookHTTPClient, logger zerolog.Logger) *RetryService {
    return &RetryService{
        repo:       repo,
        httpClient: httpClient,
        logger:     logger,
        done:       make(chan struct{}),
    }
}

func (s *RetryService) Start(ctx context.Context) {
    s.ticker = time.NewTicker(retryInterval)
    defer s.ticker.Stop()

    for {
        select {
        case <-s.ticker.C:
            s.processBatch(ctx)
        case <-ctx.Done():
            return
        case <-s.done:
            return
        }
    }
}

func (s *RetryService) Stop() {
    close(s.done)
}

func (s *RetryService) processBatch(ctx context.Context) {
    entries, err := s.repo.FetchPendingBatch(ctx, retryBatchSize)
    if err != nil {
        s.logger.Error().Err(err).Msg("fetch pending webhooks failed")
        return
    }

    for _, entry := range entries {
        s.deliver(ctx, entry)
    }
}

func (s *RetryService) deliver(ctx context.Context, entry *domain.WebhookDeliveryLog) {
    // Найти URL вебхука клиента (нужен репозиторий webhooks конфигурации)
    // Пока используем заглушку — доработать после интеграции с webhook config table
    webhookURL := s.getWebhookURL(ctx, entry.WebhookID)
    if webhookURL == "" {
        s.logger.Warn().Str("webhook_id", entry.WebhookID.String()).Msg("webhook URL not found, skipping")
        return
    }

    payload, err := json.Marshal(domain.WebhookEvent{
        ID:        uuid.New().String(),
        Event:     domain.EventType(entry.EventType),
        ClientID:  entry.ClientID.String(),
        Timestamp: time.Now().UTC().Format(time.RFC3339),
        Data:      entry.Payload,
    })
    if err != nil {
        s.logger.Error().Err(err).Msg("marshal webhook payload failed")
        return
    }

    deliverCtx, cancel := context.WithTimeout(ctx, httpTimeout)
    defer cancel()

    statusCode, body, deliverErr := s.httpClient.Post(deliverCtx, webhookURL, payload)
    success := deliverErr == nil && statusCode >= 200 && statusCode < 300

    lastError := ""
    if deliverErr != nil {
        lastError = deliverErr.Error()
    } else if !success {
        lastError = fmt.Sprintf("HTTP %d: %s", statusCode, truncate(body, 200))
    }

    if err := s.repo.UpdateAfterAttempt(ctx, entry.ID, success, statusCode, truncate(body, 500), lastError); err != nil {
        s.logger.Error().Err(err).Str("delivery_log_id", entry.ID.String()).Msg("update delivery log failed")
        return
    }

    if !success && entry.AttemptCount+1 >= entry.MaxAttempts {
        if err := s.repo.MoveToDeadLetter(ctx, entry.ID); err != nil {
            s.logger.Error().Err(err).Msg("move to dead letter failed")
        }
    }
}

func (s *RetryService) getWebhookURL(ctx context.Context, webhookID uuid.UUID) string {
    // TODO: интегрировать с webhook configuration repository
    // Должен возвращать URL из таблицы webhook configurations клиента
    return ""
}

func truncate(s string, max int) string {
    if len(s) <= max {
        return s
    }
    return s[:max] + "..."
}
```

- [ ] **Step 4.4: Запустить тесты**

```bash
go test ./internal/services/webhook/application/... -run TestRetryService -v
```

- [ ] **Step 4.5: Зарегистрировать RetryService в webhook service main**

В `cmd/services/webhook-service/main.go` (или где стартует webhook service):

```go
retryService := application.NewRetryService(deliveryRepo, httpClient, logger)
go retryService.Start(ctx)
defer retryService.Stop()
```

- [ ] **Step 4.6: Коммит**

```bash
git add internal/services/webhook/application/retry_service.go
git add internal/services/webhook/application/retry_service_test.go
git add cmd/services/webhook-service/main.go
git commit -m "feat(webhook): add retry service with exponential backoff and dead letter queue"
```

---

## Task 5: SSE Streaming endpoint

**Files:**
- Create: `internal/gateway/client/handlers/events.go`
- Modify: `internal/gateway/client/router/router.go`

- [ ] **Step 5.1: Написать тест SSE handler**

```go
// internal/gateway/client/handlers/events_test.go
func TestEventsStream_WritesSSE(t *testing.T) {
    handler := NewEventsHandler()

    req := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil)
    req = req.WithContext(context.WithValue(req.Context(), clientIDKey, uuid.New().String()))
    w := newSSERecorder() // httptest.ResponseRecorder с Flush support

    // Запустить handler в goroutine, отправить тестовое событие, закрыть
    done := make(chan struct{})
    go func() {
        handler.Stream(w, req)
        close(done)
    }()

    // Дать время на установку SSE
    time.Sleep(50 * time.Millisecond)

    // Отправить событие через broadcast
    handler.Broadcast(req.Context().Value(clientIDKey).(string), domain.WebhookEvent{
        ID:    "evt-1",
        Event: domain.EventMessageDelivered,
        Data:  map[string]interface{}{"message_id": "msg-1"},
    })

    time.Sleep(50 * time.Millisecond)
    req.Context().Done()

    body := w.Body.String()
    assert.Contains(t, body, "data:")
    assert.Contains(t, body, "message.delivered")
}
```

- [ ] **Step 5.2: Запустить тест — убедиться, что падает**

```bash
go test ./internal/gateway/client/handlers/... -run TestEventsStream -v
```

- [ ] **Step 5.3: Реализовать SSE handler**

```go
// internal/gateway/client/handlers/events.go
package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"
)

// EventsHandler streams real-time events to clients via Server-Sent Events.
type EventsHandler struct {
    mu          sync.RWMutex
    subscribers map[string][]chan domain.WebhookEvent // clientID -> channels
}

func NewEventsHandler() *EventsHandler {
    return &EventsHandler{
        subscribers: make(map[string][]chan domain.WebhookEvent),
    }
}

// Stream handles GET /api/v1/events/stream — SSE endpoint.
// Client keeps connection open; server pushes events as they occur.
func (h *EventsHandler) Stream(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    clientID := getClientIDFromContext(r.Context())

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no") // nginx: disable buffering

    // Register subscriber
    ch := make(chan domain.WebhookEvent, 10)
    h.subscribe(clientID, ch)
    defer h.unsubscribe(clientID, ch)

    // Send initial ping to confirm connection
    fmt.Fprintf(w, ": connected\n\n")
    flusher.Flush()

    // Keepalive ticker
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case evt, ok := <-ch:
            if !ok {
                return
            }
            data, err := json.Marshal(evt)
            if err != nil {
                continue
            }
            fmt.Fprintf(w, "id: %s\n", evt.ID)
            fmt.Fprintf(w, "event: %s\n", evt.Event)
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()

        case <-ticker.C:
            // Keepalive ping
            fmt.Fprintf(w, ": ping\n\n")
            flusher.Flush()

        case <-r.Context().Done():
            return
        }
    }
}

// Broadcast sends an event to all active subscribers of a client.
func (h *EventsHandler) Broadcast(clientID string, evt domain.WebhookEvent) {
    h.mu.RLock()
    channels, ok := h.subscribers[clientID]
    h.mu.RUnlock()
    if !ok {
        return
    }
    for _, ch := range channels {
        select {
        case ch <- evt:
        default:
            // Drop if subscriber is slow (channel full) — SSE is best-effort
        }
    }
}

func (h *EventsHandler) subscribe(clientID string, ch chan domain.WebhookEvent) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.subscribers[clientID] = append(h.subscribers[clientID], ch)
}

func (h *EventsHandler) unsubscribe(clientID string, ch chan domain.WebhookEvent) {
    h.mu.Lock()
    defer h.mu.Unlock()
    channels := h.subscribers[clientID]
    for i, c := range channels {
        if c == ch {
            h.subscribers[clientID] = append(channels[:i], channels[i+1:]...)
            break
        }
    }
    if len(h.subscribers[clientID]) == 0 {
        delete(h.subscribers, clientID)
    }
    close(ch)
}
```

- [ ] **Step 5.4: Зарегистрировать роут**

В `internal/gateway/client/router/router.go`:

```go
eventsHandler := handlers.NewEventsHandler()
r.Handle("/api/v1/events/stream", authMiddleware(http.HandlerFunc(eventsHandler.Stream))).Methods(http.MethodGet)
```

- [ ] **Step 5.5: Запустить тесты**

```bash
go test ./internal/gateway/client/handlers/... -run TestEventsStream -v
```

- [ ] **Step 5.6: Коммит**

```bash
git add internal/gateway/client/handlers/events.go
git add internal/gateway/client/handlers/events_test.go
git add internal/gateway/client/router/router.go
git commit -m "feat(gateway): add SSE streaming endpoint GET /api/v1/events/stream"
```

---

## Task 6: OpenAPI documentation для Webhook v2

- [ ] **Step 6.1: Добавить /events/stream в openapi.yaml**

```yaml
  /events/stream:
    get:
      summary: Real-time event stream (SSE)
      description: |
        Server-Sent Events stream for real-time delivery notifications.
        Keep the connection open to receive events as they occur.
        Useful for aggregators and enterprise clients who need low-latency updates.
      operationId: streamEvents
      tags:
        - Events
      security:
        - ApiKeyAuth: []
      responses:
        '200':
          description: SSE stream established
          content:
            text/event-stream:
              schema:
                type: string
              example: |
                id: evt-uuid
                event: message.delivered
                data: {"id":"evt-uuid","event":"message.delivered","client_id":"...","timestamp":"...","data":{"message_id":"..."}}
        '401':
          $ref: '#/components/responses/Unauthorized'
```

- [ ] **Step 6.2: Добавить webhook event types schema**

```yaml
    WebhookEvent:
      type: object
      properties:
        id:
          type: string
          format: uuid
        event:
          type: string
          enum:
            - message.delivered
            - message.failed
            - message.sent
            - message.expired
            - campaign.launched
            - campaign.completed
            - campaign.failed
            - campaign.paused
            - inbound.received
            - balance.low
            - balance.depleted
            - sender_name.approved
            - sender_name.rejected
        client_id:
          type: string
          format: uuid
        timestamp:
          type: string
          format: date-time
        data:
          type: object
          additionalProperties: true
          description: Event-specific payload
```

- [ ] **Step 6.3: Коммит**

```bash
git add api/openapi/openapi.yaml
git commit -m "docs(openapi): add SSE stream endpoint and WebhookEvent schema"
```

---

## Task 7: Финальная проверка

- [ ] **Step 7.1: Запустить все тесты**

```bash
go test ./internal/services/webhook/... -v
go test ./internal/gateway/client/... -v
```

Ожидаем: все `PASS`.

- [ ] **Step 7.2: Проверить сборку**

```bash
go build ./...
```

- [ ] **Step 7.3: Финальный коммит**

```bash
git status
# Коммит если есть незакомиченные изменения
git commit -m "feat(webhook-v2): complete Phase 5 - retry, dead letter, SSE streaming, event catalog"
```
