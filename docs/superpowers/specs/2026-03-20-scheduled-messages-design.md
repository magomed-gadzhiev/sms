# Scheduled Messages — Design Spec

## Problem

No way to schedule SMS delivery for a specific time. Clients must implement their own scheduling logic and call the API at the right moment, adding complexity and unreliability on their side.

## Solution Overview

Extend the existing **Messaging Service** with a `scheduled_at` field in SendMessage/SendBatch APIs and a background scheduler goroutine that picks up scheduled messages when their time arrives and publishes them to Kafka for delivery.

## Scope

### In Scope
- `scheduled_at` field in SendMessage and SendBatch APIs
- New message status: `scheduled`
- Background scheduler goroutine in Messaging Service
- Cancel scheduled messages via `DELETE /api/v1/sms/{id}`
- Configurable scheduler interval and batch size

### Out of Scope
- Recurring/repeating schedules
- Campaign management (separate feature)
- Time zone conversion (client sends UTC)
- Admin API for managing scheduled messages

## Architecture

### Approach: Embedded Scheduler in Messaging Service

No new microservice. Messaging Service already owns the message lifecycle and writes to the `messages` table. Adding scheduling logic here is natural:

1. **At send time:** if `scheduled_at` is in the future, save message with status `scheduled` instead of publishing to Kafka
2. **Background scheduler:** goroutine polls DB periodically, finds messages with `scheduled_at <= NOW()` and `status = scheduled`, transitions them to `pending` and publishes to Kafka

**Message lifecycle with scheduling:**
```
scheduled → pending → queued → sent → delivered
                                  ↘ failed
                                  ↘ expired
```

The `scheduled` status is cancellable. Once a message transitions to `pending`, the normal delivery flow takes over.

## Data Model

### Migration: `migrations/000010_add_scheduled_at_to_messages.up.sql`

```sql
ALTER TABLE messages ADD COLUMN scheduled_at TIMESTAMPTZ;

CREATE INDEX idx_messages_scheduled ON messages(status, scheduled_at)
    WHERE status = 'scheduled';
```

Down migration drops the index and column.

### Constraints
- `scheduled_at` is nullable — `NULL` means immediate delivery
- Must be in the future (validated at API level, not DB constraint)
- Must be within 7 days from now (configurable)
- Partial index covers only `scheduled` messages — zero overhead on existing queries

### Changes to shared models

In `internal/shared/models.go`:
- Add `ScheduledAt *time.Time` field to `Message` struct
- Add `MessageStatusScheduled = "scheduled"` constant

### Changes to messagingv1 proto

In `api/proto/messaging/messaging.proto`:
- Add `google.protobuf.Timestamp scheduled_at = 20;` to `SendMessageRequest`
- Add `google.protobuf.Timestamp scheduled_at = 5;` to `SendBatchRequest` (batch-level, applies to all messages)
- Add `google.protobuf.Timestamp scheduled_at = 15;` to `GetMessageStatusResponse`
- Add new RPC: `rpc DeleteMessage(DeleteMessageRequest) returns (DeleteMessageResponse);`
- Add `DeleteMessageRequest { string message_id = 1; string client_id = 2; }`
- Add `DeleteMessageResponse { bool success = 1; }`

Field numbers must not conflict with existing fields — verify actual proto before implementation.

## Scheduler Implementation

### New file: `internal/services/messaging/application/scheduler.go`

```
Scheduler struct {
    messageRepo  *repository.MessageRepository
    kafkaProducer *queue.Producer
    logger       zerolog.Logger
    interval     time.Duration
    batchSize    int
    ctx          context.Context
    cancel       context.CancelFunc
}
```

**Run loop:**
1. Sleep for `interval` (default 10s, configurable)
2. Query: `SELECT * FROM messages WHERE status = 'scheduled' AND scheduled_at <= NOW() ORDER BY scheduled_at ASC LIMIT $batchSize`
3. For each message:
   - Update status to `pending`
   - Publish to Kafka `sms.outgoing` (same format as normal send)
   - If Kafka publish fails: revert status back to `scheduled`, log error
4. Repeat

**Concurrency safety:** Only one Messaging Service instance should run the scheduler to avoid double-sends. Options:
- Use `SELECT ... FOR UPDATE SKIP LOCKED` to safely handle multiple instances
- This allows horizontal scaling without a distributed lock

**Graceful shutdown:** Scheduler respects context cancellation, finishes current batch, then stops.

### Changes to `internal/services/messaging/application/messaging_service.go`

In `SendMessage`:
1. If `scheduled_at` is not nil and in the future:
   - Set `status = scheduled`
   - Save to DB
   - Do NOT publish to Kafka
   - Return response with `status: "scheduled"`
2. If `scheduled_at` is nil or in the past:
   - Current behavior (status `pending`, publish to Kafka)

Validation:
- `scheduled_at` in the past → error
- `scheduled_at` more than 7 days ahead → error

### Changes to message repository

In `internal/services/messaging/infrastructure/repository/message_repository.go`:
- Add `GetScheduledReady(ctx, limit int) ([]*shared.Message, error)` — fetches scheduled messages ready for delivery using `FOR UPDATE SKIP LOCKED`
- Add `DeleteByIDAndStatus(ctx, id, clientID uuid.UUID, status string) error` — deletes message only if it matches the given status (for cancel)
- Modify existing `Create` method to persist `scheduled_at`

## HTTP API

### Modified: `POST /api/v1/sms/send`

New optional field:
```json
{
    "source": "+70001234567",
    "destination": "+79991234567",
    "text": "Hello!",
    "scheduled_at": "2026-03-21T10:00:00Z"
}
```

Response when scheduled:
```json
{
    "message_id": "uuid",
    "status": "scheduled",
    "scheduled_at": "2026-03-21T10:00:00Z",
    "created_at": "2026-03-20T15:00:00Z"
}
```

### Modified: `POST /api/v1/sms/batch`

New batch-level field (applies to all messages):
```json
{
    "scheduled_at": "2026-03-21T10:00:00Z",
    "messages": [...]
}
```

### New: `DELETE /api/v1/sms/{id}`

- Cancels a scheduled message
- Only works for `status = scheduled`
- Other statuses → HTTP 409 Conflict ("message already in processing")
- Physically deletes the record from DB
- Client gateway validates ownership via `client_id` from auth context

## Integration Points

### Messaging Service (modified)
- New `scheduler.go` in application layer
- Modified `messaging_service.go` — scheduled_at logic in SendMessage
- Modified `message_repository.go` — new query methods
- Modified `grpc/server.go` — handle scheduled_at field, implement DeleteMessage RPC
- Started in `main.go` — launch scheduler goroutine

### Client Gateway (modified)
- Modified `handlers/sms.go` — add `scheduled_at` to SendSMSRequest/SendBatchSMSRequest, validate, pass to gRPC
- New handler method `DeleteSMS` for `DELETE /api/v1/sms/{id}`
- Modified `router.go` — add DELETE route

### Proto (modified)
- Modified `api/proto/messaging/messaging.proto` — new fields + DeleteMessage RPC
- Regenerate `api/proto/messagingv1/`

### Docker Compose — No changes (no new services)

### Dockerfile
- Regenerate messagingv1 proto (already handled by existing protoc step)

## Configuration

```yaml
messaging:
  scheduler:
    enabled: true
    interval: 10s
    batch_size: 100
    max_schedule_days: 7
```

Environment variables for Docker:
- `SCHEDULER_INTERVAL=10s`
- `SCHEDULER_BATCH_SIZE=100`
- `SCHEDULER_MAX_DAYS=7`
