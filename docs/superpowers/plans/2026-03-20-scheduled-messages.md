# Scheduled Messages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add scheduled message delivery with `scheduled_at` field, background scheduler, and message cancellation to the existing Messaging Service.

**Architecture:** Extends Messaging Service (no new microservice). Adds `scheduled_at` column to messages table, new `scheduled` and `cancelled` statuses, a scheduler goroutine that polls for ready messages, and a cancel endpoint. Client Gateway passes `scheduled_at` through to Messaging Service.

**Tech Stack:** Go 1.24, PostgreSQL (partitioned tables), Kafka (Sarama), gRPC, gorilla/mux, zerolog

**Spec:** `docs/superpowers/specs/2026-03-20-scheduled-messages-design.md`

---

## File Map

### New files
| File | Responsibility |
|------|---------------|
| `migrations/000010_add_scheduled_messages.up.sql` | Add scheduled_at column + CHECK constraint update |
| `migrations/000010_add_scheduled_messages.down.sql` | Revert migration |
| `internal/services/messaging/application/scheduler.go` | Background scheduler goroutine |

### Modified files
| File | Change |
|------|--------|
| `internal/shared/models.go` | Add ScheduledAt field, scheduled/cancelled status constants |
| `internal/services/messaging/domain/message.go` | Add ScheduledAt field, MarkAsScheduled/MarkAsCancelled, ToShared/FromShared mapping |
| `internal/services/messaging/domain/repository.go` | Add GetScheduledReady, CancelByIDAndStatus methods to interface |
| `internal/services/messaging/application/message_service.go` | Add ScheduledAt to SendMessageOptions, scheduling logic in SendMessage, CancelMessage method |
| `internal/services/messaging/infrastructure/repository/message_repository.go` | Implement new repo methods |
| `internal/storage/message_repository.go` | Add scheduled_at to Create query, new query methods |
| `internal/services/messaging/grpc/server.go` | Handle scheduled_at, implement CancelMessage RPC |
| `api/proto/messaging/messaging.proto` | Add scheduled_at fields, CancelMessage RPC |
| `internal/gateway/client/handlers/sms.go` | Add scheduled_at to request structs, validation, CancelSMS handler |
| `internal/gateway/client/router/router.go` | Add DELETE route |
| `cmd/services/messaging-service/main.go` | Start scheduler goroutine |
| `deployments/docker-compose.yml` | Add scheduler env vars to messaging-service |
| `deployments/docker/service-base.Dockerfile` | Regenerate messagingv1 proto |

---

## Task 1: Database Migration

**Files:**
- Create: `migrations/000010_add_scheduled_messages.up.sql`
- Create: `migrations/000010_add_scheduled_messages.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000010_add_scheduled_messages.up.sql

-- Add scheduled_at column to messages table
ALTER TABLE messages ADD COLUMN scheduled_at TIMESTAMPTZ;

-- Update CHECK constraint to include 'scheduled' and 'cancelled' statuses
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_status_check;
ALTER TABLE messages ADD CONSTRAINT messages_status_check
    CHECK (status IN ('pending', 'queued', 'sent', 'delivered', 'failed', 'expired', 'rejected', 'scheduled', 'cancelled'));

-- Partial index for scheduler queries (only covers scheduled messages)
CREATE INDEX idx_messages_scheduled ON messages(status, scheduled_at)
    WHERE status = 'scheduled';
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000010_add_scheduled_messages.down.sql
DROP INDEX IF EXISTS idx_messages_scheduled;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_status_check;
ALTER TABLE messages ADD CONSTRAINT messages_status_check
    CHECK (status IN ('pending', 'queued', 'sent', 'delivered', 'failed', 'expired', 'rejected'));
ALTER TABLE messages DROP COLUMN IF EXISTS scheduled_at;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000010_add_scheduled_messages.up.sql migrations/000010_add_scheduled_messages.down.sql
git commit -m "feat(scheduled): add migration for scheduled_at column and new statuses"
```

---

## Task 2: Proto Changes + Regeneration

**Files:**
- Modify: `api/proto/messaging/messaging.proto`
- Regenerate: `api/proto/messagingv1/`

- [ ] **Step 1: Modify proto**

In `api/proto/messaging/messaging.proto`:

Add to service definition (after ProcessDLR):
```protobuf
  // CancelMessage cancels a scheduled message
  rpc CancelMessage(CancelMessageRequest) returns (CancelMessageResponse);
```

Add `scheduled_at` field to `SendMessageRequest` (field 15):
```protobuf
  google.protobuf.Timestamp scheduled_at = 15;  // Время запланированной отправки (опционально)
```

Add `scheduled_at` field to `SendBatchRequest` (field 3):
```protobuf
  google.protobuf.Timestamp scheduled_at = 3;  // Время запланированной отправки для всех сообщений
```

Add `scheduled_at` field to `SendMessageResponse` (field 5):
```protobuf
  google.protobuf.Timestamp scheduled_at = 5;  // Время запланированной отправки
```

Add `scheduled_at` field to `GetMessageStatusResponse` (field 11):
```protobuf
  google.protobuf.Timestamp scheduled_at = 11; // Время запланированной отправки
```

Add new messages at the end of the file:
```protobuf
// CancelMessageRequest - запрос на отмену запланированного сообщения
message CancelMessageRequest {
  string message_id = 1;
  string client_id = 2;
}

// CancelMessageResponse - ответ на отмену сообщения
message CancelMessageResponse {
  bool success = 1;
}
```

- [ ] **Step 2: Regenerate Go code**

```bash
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  -I api/proto \
  api/proto/messaging/messaging.proto
```

If protoc is not available, manually update the generated files following the pattern of the existing `api/proto/messagingv1/` files.

- [ ] **Step 3: Verify build**

```bash
go build ./api/proto/messagingv1/
```

- [ ] **Step 4: Commit**

```bash
git add api/proto/messaging/ api/proto/messagingv1/
git commit -m "feat(scheduled): add scheduled_at fields and CancelMessage RPC to proto"
```

---

## Task 3: Shared Models Update

**Files:**
- Modify: `internal/shared/models.go`

- [ ] **Step 1: Add ScheduledAt to Message struct and new status constants**

In `internal/shared/models.go`, add `ScheduledAt *time.Time` field to the `Message` struct (after `FailedAt`).

Add new status constants:
```go
MessageStatusScheduled  MessageStatus = "scheduled"
MessageStatusCancelled  MessageStatus = "cancelled"
```

- [ ] **Step 2: Commit**

```bash
git add internal/shared/models.go
git commit -m "feat(scheduled): add ScheduledAt field and new status constants to shared models"
```

---

## Task 4: Domain Model Updates

**Files:**
- Modify: `internal/services/messaging/domain/message.go`
- Modify: `internal/services/messaging/domain/repository.go`

- [ ] **Step 1: Add ScheduledAt to domain.Message**

In `internal/services/messaging/domain/message.go`, add `ScheduledAt *time.Time` field to the `Message` struct (after `FailedAt`).

Add two new methods:
```go
// MarkAsScheduled marks the message as scheduled for future delivery
func (m *Message) MarkAsScheduled(scheduledAt time.Time) {
	m.Status = shared.MessageStatusScheduled
	m.ScheduledAt = &scheduledAt
	m.UpdatedAt = time.Now()
}

// MarkAsCancelled marks a scheduled message as cancelled
func (m *Message) MarkAsCancelled() {
	m.Status = shared.MessageStatusCancelled
	m.UpdatedAt = time.Now()
}
```

Update `ToShared()` to map `ScheduledAt`:
```go
ScheduledAt: m.ScheduledAt,
```

Update `MessageFromShared()` to map `ScheduledAt`:
```go
ScheduledAt: msg.ScheduledAt,
```

- [ ] **Step 2: Add new methods to MessageRepository interface**

In `internal/services/messaging/domain/repository.go`, add to the `MessageRepository` interface:

```go
GetScheduledReady(ctx context.Context, limit int) ([]*Message, error)
CancelByIDAndStatus(ctx context.Context, id uuid.UUID, clientID uuid.UUID) error
```

- [ ] **Step 3: Verify build**

```bash
go build ./internal/services/messaging/domain/
```

Note: this will fail until the repository implementation is updated (Task 5). That's expected.

- [ ] **Step 4: Commit**

```bash
git add internal/services/messaging/domain/message.go internal/services/messaging/domain/repository.go
git commit -m "feat(scheduled): add ScheduledAt to domain model and new repo interface methods"
```

---

## Task 5: Storage + Repository Implementation

**Files:**
- Modify: `internal/storage/message_repository.go`
- Modify: `internal/services/messaging/infrastructure/repository/message_repository.go`

- [ ] **Step 1: Update storage layer Create method**

In `internal/storage/message_repository.go`, modify the `Create` method:
- Add `scheduled_at` to the column list in the INSERT query (after `failed_at`)
- Add `$34` placeholder
- Add `msg.ScheduledAt` to the ExecContext args

Also add a new method to the storage layer:
```go
// GetScheduledReady fetches messages ready for scheduled delivery
func (r *MessageRepository) GetScheduledReady(ctx context.Context, limit int) ([]*shared.Message, error) {
	query := `SELECT * FROM messages
		WHERE status = 'scheduled'
		AND scheduled_at <= NOW()
		AND created_at >= NOW() - INTERVAL '7 days'
		ORDER BY scheduled_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`
	// ... scan and return
}

// CancelByIDAndStatus atomically cancels a scheduled message
func (r *MessageRepository) CancelByIDAndStatus(ctx context.Context, id, clientID uuid.UUID) error {
	query := `UPDATE messages SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1 AND client_id = $2 AND status = 'scheduled'`
	result, err := r.db.ExecContext(ctx, query, id, clientID)
	// check rows affected, return error if 0
}
```

Read the existing storage repository methods carefully to follow the exact scan pattern (it uses `scanMessage` helper).

- [ ] **Step 2: Update messaging-level repository**

In `internal/services/messaging/infrastructure/repository/message_repository.go`, add:

```go
// GetScheduledReady fetches scheduled messages ready for delivery
func (r *MessageRepository) GetScheduledReady(ctx context.Context, limit int) ([]*domain.Message, error) {
	sharedMessages, err := r.repo.GetScheduledReady(ctx, limit)
	if err != nil {
		return nil, err
	}
	messages := make([]*domain.Message, len(sharedMessages))
	for i, sm := range sharedMessages {
		messages[i] = domain.MessageFromShared(sm)
	}
	return messages, nil
}

// CancelByIDAndStatus cancels a scheduled message
func (r *MessageRepository) CancelByIDAndStatus(ctx context.Context, id, clientID uuid.UUID) error {
	return r.repo.CancelByIDAndStatus(ctx, id, clientID)
}
```

- [ ] **Step 3: Verify build**

```bash
go build ./internal/services/messaging/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/storage/message_repository.go internal/services/messaging/infrastructure/repository/message_repository.go
git commit -m "feat(scheduled): add scheduled message storage and repository methods"
```

---

## Task 6: Application Service — Scheduling Logic

**Files:**
- Modify: `internal/services/messaging/application/message_service.go`

- [ ] **Step 1: Add ScheduledAt to SendMessageOptions and SendMessageRequest**

Add `ScheduledAt *time.Time` field to both structs.

- [ ] **Step 2: Modify SendMessage for scheduling**

In the `SendMessage` method, after validation and before the current `Create` + publish flow, add scheduling logic:

```go
// Apply scheduled_at from options
if options != nil && options.ScheduledAt != nil {
    scheduledAt := *options.ScheduledAt
    tolerance := 30 * time.Second

    // If scheduled_at is more than tolerance in the future → schedule it
    if scheduledAt.After(time.Now().Add(tolerance)) {
        // Validate: not more than 7 days ahead
        maxSchedule := time.Now().Add(7 * 24 * time.Hour)
        if scheduledAt.After(maxSchedule) {
            return nil, fmt.Errorf("scheduled_at cannot be more than 7 days in the future")
        }

        msg.MarkAsScheduled(scheduledAt)

        // Save to DB but do NOT publish to Kafka
        if err := s.messageRepo.Create(ctx, msg); err != nil {
            return nil, fmt.Errorf("failed to create scheduled message: %w", err)
        }

        // Skip PublishMessageCreated and PublishMessageQueued
        // These publish to sms.outgoing which would bypass scheduling
        return msg, nil
    }
    // If within tolerance, treat as immediate send (fall through to normal flow)
}

// ... existing Create + Publish flow continues here
```

- [ ] **Step 3: Add CancelMessage method**

```go
// CancelMessage cancels a scheduled message
func (s *MessageService) CancelMessage(ctx context.Context, messageID, clientID uuid.UUID) error {
	return s.messageRepo.CancelByIDAndStatus(ctx, messageID, clientID)
}
```

- [ ] **Step 4: Update SendBatch to propagate batch-level scheduled_at**

In the `SendBatch` method, accept a `scheduledAt *time.Time` parameter. Propagate it to each individual message's options:

The `SendBatch` signature changes from:
```go
func (s *MessageService) SendBatch(ctx context.Context, clientID uuid.UUID, requests []*SendMessageRequest) ([]*BatchResult, error)
```
to:
```go
func (s *MessageService) SendBatch(ctx context.Context, clientID uuid.UUID, requests []*SendMessageRequest, scheduledAt *time.Time) ([]*BatchResult, error)
```

Inside the loop, set `options.ScheduledAt = scheduledAt` for each message.

- [ ] **Step 5: Verify build**

```bash
go build ./internal/services/messaging/...
```

Note: gRPC server will fail to build until updated (Task 8). Expected.

- [ ] **Step 6: Commit**

```bash
git add internal/services/messaging/application/message_service.go
git commit -m "feat(scheduled): add scheduling logic and CancelMessage to message service"
```

---

## Task 7: Scheduler Goroutine

**Files:**
- Create: `internal/services/messaging/application/scheduler.go`

- [ ] **Step 1: Create scheduler**

```go
package application

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
)

type Scheduler struct {
	messageRepo    domain.MessageRepository
	eventPublisher domain.EventPublisher
	logger         zerolog.Logger
	interval       time.Duration
	batchSize      int
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewScheduler(
	messageRepo domain.MessageRepository,
	eventPublisher domain.EventPublisher,
	interval time.Duration,
	batchSize int,
) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		messageRepo:    messageRepo,
		eventPublisher: eventPublisher,
		logger:         log.With().Str("component", "scheduler").Logger(),
		interval:       interval,
		batchSize:      batchSize,
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (s *Scheduler) Start() {
	s.logger.Info().
		Dur("interval", s.interval).
		Int("batch_size", s.batchSize).
		Msg("scheduler started")

	go s.run()
}

func (s *Scheduler) Stop() {
	s.cancel()
	s.logger.Info().Msg("scheduler stopped")
}

func (s *Scheduler) run() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.processBatch()
		}
	}
}

func (s *Scheduler) processBatch() {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	messages, err := s.messageRepo.GetScheduledReady(ctx, s.batchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to fetch scheduled messages")
		return
	}

	if len(messages) == 0 {
		return
	}

	s.logger.Info().Int("count", len(messages)).Msg("processing scheduled messages")

	for _, msg := range messages {
		// Update status to pending
		if err := s.messageRepo.UpdateStatus(ctx, msg.ID, "pending", ""); err != nil {
			s.logger.Error().Err(err).Str("message_id", msg.ID.String()).Msg("failed to update status to pending")
			continue
		}

		// Publish to Kafka (PublishMessageCreated sends to sms.outgoing)
		msg.Status = "pending"
		if err := s.eventPublisher.PublishMessageCreated(ctx, msg); err != nil {
			s.logger.Error().Err(err).Str("message_id", msg.ID.String()).Msg("failed to publish to Kafka, reverting to scheduled")
			// Revert status back to scheduled
			if revertErr := s.messageRepo.UpdateStatus(ctx, msg.ID, "scheduled", ""); revertErr != nil {
				s.logger.Error().Err(revertErr).Str("message_id", msg.ID.String()).Msg("failed to revert status to scheduled")
			}
			continue
		}

		// Update status to queued
		if err := s.messageRepo.UpdateStatus(ctx, msg.ID, "queued", ""); err != nil {
			s.logger.Warn().Err(err).Str("message_id", msg.ID.String()).Msg("failed to update status to queued (message already in Kafka)")
		}

		s.logger.Debug().Str("message_id", msg.ID.String()).Msg("scheduled message dispatched")
	}
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/services/messaging/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/messaging/application/scheduler.go
git commit -m "feat(scheduled): add background scheduler goroutine"
```

---

## Task 8: gRPC Server Updates

**Files:**
- Modify: `internal/services/messaging/grpc/server.go`

- [ ] **Step 1: Handle scheduled_at in SendMessage**

In the `SendMessage` gRPC method, after building options:

```go
if req.ScheduledAt != nil {
    scheduledAt := req.ScheduledAt.AsTime()
    options.ScheduledAt = &scheduledAt
}
```

In the response, add `scheduled_at` if present:
```go
resp := &messagingv1.SendMessageResponse{
    MessageId: msg.ID.String(),
    Status:    string(msg.Status),
    CreatedAt: timestamppb.New(msg.CreatedAt),
}
if msg.ScheduledAt != nil {
    resp.ScheduledAt = timestamppb.New(*msg.ScheduledAt)
}
return resp, nil
```

- [ ] **Step 2: Handle scheduled_at in SendBatch**

In the `SendBatch` gRPC method, extract batch-level `scheduled_at`:

```go
var scheduledAt *time.Time
if req.ScheduledAt != nil {
    t := req.ScheduledAt.AsTime()
    scheduledAt = &t
}
```

Pass it to `s.messageService.SendBatch(ctx, clientID, requests, scheduledAt)`.

- [ ] **Step 3: Handle scheduled_at in GetMessageStatus response**

After building the response, add:
```go
if msg.ScheduledAt != nil {
    response.ScheduledAt = timestamppb.New(*msg.ScheduledAt)
}
```

- [ ] **Step 4: Implement CancelMessage RPC**

```go
func (s *Server) CancelMessage(ctx context.Context, req *messagingv1.CancelMessageRequest) (*messagingv1.CancelMessageResponse, error) {
    if req.MessageId == "" {
        return nil, status.Error(codes.InvalidArgument, "message_id is required")
    }
    if req.ClientId == "" {
        return nil, status.Error(codes.InvalidArgument, "client_id is required")
    }

    messageID, err := uuid.Parse(req.MessageId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid message_id format")
    }
    clientID, err := uuid.Parse(req.ClientId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
    }

    if err := s.messageService.CancelMessage(ctx, messageID, clientID); err != nil {
        // If no rows affected, the message was not in scheduled status
        return nil, status.Error(codes.FailedPrecondition, "message not found or not in scheduled status")
    }

    return &messagingv1.CancelMessageResponse{Success: true}, nil
}
```

- [ ] **Step 5: Verify build**

```bash
go build ./cmd/services/messaging-service/
```

- [ ] **Step 6: Commit**

```bash
git add internal/services/messaging/grpc/server.go
git commit -m "feat(scheduled): handle scheduled_at in gRPC server and add CancelMessage RPC"
```

---

## Task 9: Client Gateway — SMS Handler Updates

**Files:**
- Modify: `internal/gateway/client/handlers/sms.go`
- Modify: `internal/gateway/client/router/router.go`

- [ ] **Step 1: Add scheduled_at to request structs**

In `sms.go`, add to `SendSMSRequest`:
```go
ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
```

Add to `SendBatchSMSRequest`:
```go
ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
```
(This is the existing struct that wraps `[]SendSMSRequest` with a batch-level field.)

- [ ] **Step 2: Pass scheduled_at in SendSMS handler**

In the `SendSMS` handler, after building protoReq, add:
```go
if req.ScheduledAt != nil {
    protoReq.ScheduledAt = timestamppb.New(*req.ScheduledAt)
}
```

In the response mapping, add:
```go
if resp.ScheduledAt != nil {
    response["scheduled_at"] = resp.ScheduledAt.AsTime()
}
```

- [ ] **Step 3: Pass scheduled_at in SendBatch handler**

In the `SendBatch` handler, pass the batch-level `scheduled_at` to the gRPC request:
```go
if req.ScheduledAt != nil {
    protoReq.ScheduledAt = timestamppb.New(*req.ScheduledAt)
}
```

- [ ] **Step 4: Add CancelSMS handler**

```go
// CancelSMS cancels a scheduled message
func (h *SMSHandlers) CancelSMS(w http.ResponseWriter, r *http.Request) {
    clientID, ok := middleware.GetClientID(r.Context())
    if !ok {
        respondError(w, shared.ErrUnauthorized("Клиент не найден"))
        return
    }

    vars := mux.Vars(r)
    messageID := vars["id"]
    if messageID == "" {
        respondError(w, shared.ErrInvalidInput("ID сообщения обязателен"))
        return
    }

    _, err := h.messagingClient.CancelMessage(r.Context(), &messagingv1.CancelMessageRequest{
        MessageId: messageID,
        ClientId:  clientID.String(),
    })
    if err != nil {
        respondGRPCError(w, err)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}
```

Note: `CancelSMS` uses `h.messagingClient` (not a template client). The `SMSHandlers` struct already has `messagingClient`.

- [ ] **Step 5: Add DELETE route**

In `internal/gateway/client/router/router.go`, add to the SMS subrouter:
```go
sms.HandleFunc("/{id}", smsHandlers.CancelSMS).Methods("DELETE")
```

- [ ] **Step 6: Verify build**

```bash
go build ./cmd/client-gateway/
```

- [ ] **Step 7: Commit**

```bash
git add internal/gateway/client/handlers/sms.go internal/gateway/client/router/router.go
git commit -m "feat(scheduled): add scheduled_at to gateway SMS handlers and cancel endpoint"
```

---

## Task 10: Messaging Service Main — Start Scheduler

**Files:**
- Modify: `cmd/services/messaging-service/main.go`

- [ ] **Step 1: Add scheduler startup**

After creating `messageService` and `dlrService`, add:

```go
// Start scheduler for scheduled messages
schedulerInterval := 10 * time.Second
if intervalStr := os.Getenv("SCHEDULER_INTERVAL"); intervalStr != "" {
    if d, err := time.ParseDuration(intervalStr); err == nil {
        schedulerInterval = d
    }
}
schedulerBatchSize := 100
if batchStr := os.Getenv("SCHEDULER_BATCH_SIZE"); batchStr != "" {
    if n, err := strconv.Atoi(batchStr); err == nil && n > 0 {
        schedulerBatchSize = n
    }
}

scheduler := application.NewScheduler(messageRepo, eventPublisher, schedulerInterval, schedulerBatchSize)
scheduler.Start()
```

Add to the import block: `"strconv"` (if not already imported).

In the graceful shutdown section (before grpcServer.GracefulStop()), add:
```go
scheduler.Stop()
logger.Info().Msg("scheduler остановлен")
```

- [ ] **Step 2: Verify build**

```bash
go build ./cmd/services/messaging-service/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/services/messaging-service/main.go
git commit -m "feat(scheduled): start scheduler goroutine in messaging service main"
```

---

## Task 11: Docker Compose + Dockerfile

**Files:**
- Modify: `deployments/docker-compose.yml`
- Modify: `deployments/docker/service-base.Dockerfile`

- [ ] **Step 1: Add scheduler env vars to messaging-service in docker-compose**

In `deployments/docker-compose.yml`, find the `messaging-service` service and add to its environment:
```yaml
      - SCHEDULER_INTERVAL=10s
      - SCHEDULER_BATCH_SIZE=100
```

- [ ] **Step 2: Regenerate messagingv1 proto in Dockerfile**

The existing Dockerfile already has a protoc step for `api/proto/messaging/messaging.proto`. The modified proto file will be automatically regenerated during Docker build. No changes needed to the Dockerfile.

- [ ] **Step 3: Commit**

```bash
git add deployments/docker-compose.yml
git commit -m "feat(scheduled): add scheduler configuration to docker-compose"
```

---

## Task 12: Build Verification

- [ ] **Step 1: Run go mod tidy**

```bash
go mod tidy
```

- [ ] **Step 2: Build all**

```bash
go build ./...
```

- [ ] **Step 3: Run go vet**

```bash
go vet ./...
```

- [ ] **Step 4: Fix any issues**

- [ ] **Step 5: Commit if needed**

```bash
git add -A
git commit -m "chore: run go mod tidy after scheduled messages implementation"
```
