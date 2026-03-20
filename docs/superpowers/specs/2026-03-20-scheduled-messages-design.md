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
- Per-message `scheduled_at` in batch (batch-level only for simplicity; per-message is a future enhancement)

## Architecture

### Approach: Embedded Scheduler in Messaging Service

No new microservice. Messaging Service already owns the message lifecycle and writes to the `messages` table. Adding scheduling logic here is natural:

1. **At send time:** if `scheduled_at` is in the future, save message with status `scheduled` instead of publishing to Kafka
2. **Background scheduler:** goroutine polls DB periodically, finds messages with `scheduled_at <= NOW()` and `status = scheduled`, transitions them to `pending` and publishes to Kafka

**Message lifecycle with scheduling:**
```
scheduled → pending → queued → sent → delivered
    ↓                              ↘ failed
 cancelled                         ↘ expired
```

The `scheduled` status is cancellable via DELETE. Once a message transitions to `pending`, the normal delivery flow takes over.

## Data Model

### Migration: `migrations/000010_add_scheduled_messages.up.sql`

```sql
-- Add scheduled_at column
ALTER TABLE messages ADD COLUMN scheduled_at TIMESTAMPTZ;

-- Update CHECK constraint to include 'scheduled' and 'cancelled' statuses
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_status_check;
ALTER TABLE messages ADD CONSTRAINT messages_status_check
    CHECK (status IN ('pending', 'queued', 'sent', 'delivered', 'failed', 'expired', 'rejected', 'scheduled', 'cancelled'));

-- Partial index for scheduler queries
CREATE INDEX idx_messages_scheduled ON messages(status, scheduled_at)
    WHERE status = 'scheduled';
```

Down migration:
```sql
DROP INDEX IF EXISTS idx_messages_scheduled;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_status_check;
ALTER TABLE messages ADD CONSTRAINT messages_status_check
    CHECK (status IN ('pending', 'queued', 'sent', 'delivered', 'failed', 'expired', 'rejected'));
ALTER TABLE messages DROP COLUMN IF EXISTS scheduled_at;
```

### Constraints
- `scheduled_at` is nullable — `NULL` means immediate delivery
- Must be in the future with 30-second tolerance for clock skew
- Must be within 7 days from now (configurable)
- Partial index covers only `scheduled` messages — zero overhead on existing queries

### Cancellation approach: soft-delete

Instead of physically deleting messages, cancellation sets `status = 'cancelled'`. This:
- Avoids complexity of DELETE on partitioned table (composite PK `(id, created_at)` requires `created_at`)
- Preserves audit trail
- Is consistent with existing status-based lifecycle

### Changes to shared models

In `internal/shared/models.go`:
- Add `ScheduledAt *time.Time` field to `Message` struct
- Add `MessageStatusScheduled = "scheduled"` constant
- Add `MessageStatusCancelled = "cancelled"` constant

### Changes to domain models

In `internal/services/messaging/domain/`:
- Add `ScheduledAt *time.Time` to `domain.Message`
- Update `ToShared()` and `MessageFromShared()` conversion methods
- Update `NewMessage()` / `SendMessageOptions` to accept `ScheduledAt`

### Changes to messagingv1 proto

In `api/proto/messaging/messaging.proto`:
- Add `google.protobuf.Timestamp scheduled_at` to `SendMessageRequest` (verify field number)
- Add `google.protobuf.Timestamp scheduled_at` to `SendBatchRequest` (batch-level, applies to all messages)
- Add `google.protobuf.Timestamp scheduled_at` to `SendMessageResponse`
- Add `google.protobuf.Timestamp scheduled_at` to `GetMessageStatusResponse`
- Add new RPC: `rpc CancelMessage(CancelMessageRequest) returns (CancelMessageResponse);`
- `CancelMessageRequest { string message_id = 1; string client_id = 2; }`
- `CancelMessageResponse { bool success = 1; }`

Field numbers must not conflict with existing fields — verify actual proto before implementation.

### Batch scheduling semantics

`scheduled_at` in `SendBatchRequest` is batch-level only. Per-message `scheduled_at` in `SendMessageRequest` is ignored when called via batch. The batch-level value is propagated to each individual message during processing. If batch `scheduled_at` is not set, all messages are sent immediately.

## Scheduler Implementation

### New file: `internal/services/messaging/application/scheduler.go`

```
Scheduler struct {
    messageRepo   MessageRepository (interface)
    kafkaProducer *queue.Producer
    logger        zerolog.Logger
    interval      time.Duration
    batchSize     int
    ctx           context.Context
    cancel        context.CancelFunc
}
```

**Run loop:**
1. Sleep for `interval` (default 10s, configurable)
2. Query: `SELECT * FROM messages WHERE status = 'scheduled' AND scheduled_at <= NOW() AND created_at >= NOW() - INTERVAL '7 days' ORDER BY scheduled_at ASC LIMIT $batchSize FOR UPDATE SKIP LOCKED`
3. For each message:
   - Update status to `pending` in the same transaction
   - Publish to Kafka `sms.outgoing` (same format as normal send)
   - If Kafka publish fails: revert status back to `scheduled`, log error
4. Commit transaction
5. Repeat

**Partition pruning:** The `created_at >= NOW() - INTERVAL '7 days'` filter enables PostgreSQL to skip old partitions. This aligns with the 7-day max schedule window.

**Concurrency safety:** `SELECT ... FOR UPDATE SKIP LOCKED` ensures multiple Messaging Service instances can run schedulers without double-processing. Locked rows are skipped, not blocked on.

**Stuck message recovery:** The scheduler also picks up messages with `status = 'pending'` AND `scheduled_at IS NOT NULL` AND `updated_at < NOW() - INTERVAL '5 minutes'` — these are messages where Kafka publish may have failed after status update. They are re-published to Kafka.

**Graceful shutdown:** Scheduler respects context cancellation, finishes current batch, then stops.

### Changes to `internal/services/messaging/application/messaging_service.go`

In `SendMessage`:
1. If `scheduled_at` is not nil and more than 30 seconds in the future:
   - Set `status = scheduled`
   - Save to DB
   - Publish `MessageCreated` event (for observability) but NOT `MessageQueued`
   - Return response with `status: "scheduled"` and `scheduled_at`
2. If `scheduled_at` is nil or within 30 seconds of now:
   - Current behavior (status `pending`, publish to Kafka)

Validation:
- `scheduled_at` more than 7 days ahead → error
- `scheduled_at` is set but NULL → treated as immediate (no error)

New method `CancelMessage(ctx, messageID, clientID)`:
- Atomic update: `UPDATE messages SET status = 'cancelled' WHERE id = $1 AND client_id = $2 AND status = 'scheduled'`
- If no rows affected → error (message not found or not in scheduled status)

### Changes to `SendMessageOptions` / `SendMessageRequest`

Add `ScheduledAt *time.Time` to both `SendMessageOptions` and internal `SendMessageRequest` structs.

### Event publishing behavior for scheduled messages

- `PublishMessageCreated` → YES (message exists in DB)
- `PublishMessageQueued` → NO (not yet in Kafka queue)
- When scheduler picks up and transitions to pending → `PublishMessageQueued` fires normally

### Changes to message repository

In `internal/services/messaging/infrastructure/repository/message_repository.go`:
- Add `GetScheduledReady(ctx, limit int) ([]*Message, error)` — fetches ready messages with `FOR UPDATE SKIP LOCKED` and `created_at` filter for partition pruning
- Add `CancelByIDAndStatus(ctx, id, clientID uuid.UUID) error` — atomic `UPDATE ... SET status = 'cancelled' WHERE status = 'scheduled'`
- Modify existing `Create` method to persist `scheduled_at`
- Add `GetStuckPending(ctx, threshold time.Duration, limit int) ([]*Message, error)` — for stuck message recovery

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

- Cancels a scheduled message (soft-delete: sets status to `cancelled`)
- Only works for `status = scheduled`
- Other statuses → HTTP 409 Conflict ("message already in processing or delivered")
- Atomic operation — no race condition with scheduler
- Client gateway validates ownership via `client_id` from auth context

## Integration Points

### Messaging Service (modified)
- New `scheduler.go` in application layer
- Modified `messaging_service.go` — scheduled_at logic in SendMessage, new CancelMessage method
- Modified domain models — ScheduledAt field, conversion methods
- Modified `message_repository.go` — new query methods (GetScheduledReady, CancelByIDAndStatus, GetStuckPending)
- Modified `grpc/server.go` — handle scheduled_at field, implement CancelMessage RPC
- Modified `main.go` — launch scheduler goroutine

### Client Gateway (modified)
- Modified `handlers/sms.go` — add `scheduled_at` to SendSMSRequest/SendBatchSMSRequest, validate (future, max 7 days), pass to gRPC
- New handler method `CancelSMS` for `DELETE /api/v1/sms/{id}`
- Modified `router.go` — add DELETE route

### Proto (modified)
- Modified `api/proto/messaging/messaging.proto` — new fields + CancelMessage RPC
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
    stuck_threshold: 5m
```

Environment variables for Docker:
- `SCHEDULER_INTERVAL=10s`
- `SCHEDULER_BATCH_SIZE=100`
- `SCHEDULER_MAX_DAYS=7`
- `SCHEDULER_STUCK_THRESHOLD=5m`
