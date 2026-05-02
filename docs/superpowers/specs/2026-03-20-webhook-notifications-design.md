# Webhook Notifications & Callback API — Design Spec

## Problem

Clients must poll `GET /api/v1/sms/status/{id}` to get delivery statuses. This is inefficient, adds latency, and increases platform load. No enterprise client wants to poll — webhooks are the industry standard.

## Solution Overview

A new **Webhook Service** microservice that consumes existing Kafka DLR events, enriches them via message lookup, and delivers HTTP callbacks to client-registered endpoints with HMAC-SHA256 signature verification and retry logic.

## Scope

### In Scope
- Webhook subscriptions CRUD (Client API + Admin API)
- Delivery status events: `delivered`, `failed`, `expired`, `rejected`
- HMAC-SHA256 payload signing
- Retry with exponential backoff (5 attempts)
- In-process delivery via goroutine worker pool
- Message enrichment via DB lookup (resolve client_id, external_id, etc.)

### Out of Scope
- Billing/system events (future feature)
- Delivery log / history of webhook attempts
- Dead letter queue / replay API
- Auto-disable on repeated failures

## Architecture

### Approach: Embedded Kafka Consumer Service

New microservice `webhook-service` following existing DDD patterns. Consumes from existing Kafka topics (`sms.dlr`, `sms.failed`), enriches events by querying the `messages` table, delivers HTTP POST to client endpoints. Retry managed in-process via `time.AfterFunc`.

**Trade-off accepted:** Pending retries are lost on service restart. Clients use polling (`GET /api/v1/sms/status/{id}`) as fallback. On graceful shutdown (SIGTERM), the service drains the worker pool with a configurable timeout (default 30s), attempting to complete in-flight HTTP requests. Pending `time.AfterFunc` retries are cancelled and lost.

### Service Structure

```
cmd/services/webhook-service/main.go
internal/services/webhook/
  ├── application/
  │   ├── webhook_service.go        # CRUD for subscriptions
  │   └── delivery_service.go       # Kafka consumer + delivery + retry
  ├── domain/
  │   └── models.go                 # Subscription, WebhookEvent
  ├── grpc/
  │   └── server.go                 # gRPC API
  └── infrastructure/
      ├── repository/
      │   ├── subscription_repository.go
      │   └── message_repository.go # Read-only access to messages table
      └── http/
          └── delivery_client.go    # HTTP client for webhook delivery
```

- gRPC port: **9098**
- Metrics port: **2119** (Prometheus, follows existing pattern: billing uses 2118)
- Kafka consumer group: `webhook-service`
- Consumes topics: `sms.dlr`, `sms.failed`

## Event Type Mapping

SMPP DLR `Stat` values map to webhook event types:

| SMPP Stat | Webhook event_type |
|-----------|-------------------|
| `DELIVRD` | `delivered` |
| `UNDELIV` | `failed` |
| `EXPIRED` | `expired` |
| `DELETED` | `failed` |
| `REJECTD` | `rejected` |
| `ACCEPTED` | (ignored, not a final state) |
| `UNKNOWN` | `failed` |

Events from `sms.failed` topic always map to `failed`.

## Message Enrichment

`DLRMessage` and `FailedMessage` Kafka structs do not carry `client_id`, `external_id`, `source`, or `destination`. The webhook service resolves these via a **read-only query to the `messages` table** using `message_id`.

### Flow
1. Receive Kafka event → extract `message_id`
2. Query `messages` table: `SELECT client_id, external_id, source, destination, submitted_at FROM messages WHERE id = $1`
3. If message not found → log warning, skip event
4. Use `client_id` to look up subscriptions, populate payload with enriched fields

### Performance
- Message lookup adds ~1-2ms per event (indexed by PK)
- Subscription cache (in-memory, TTL 60s) avoids repeated DB queries for subscriptions
- Message data is NOT cached — each event does one DB read (messages are unique per event)

This approach requires **no changes to existing Kafka message schemas or services**. The webhook service has read-only access to the shared PostgreSQL database.

## Data Model

### Table: `webhook_subscriptions`

```sql
CREATE TABLE webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    event_types TEXT[] NOT NULL,
    secret TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_subscriptions_client_id ON webhook_subscriptions(client_id);
CREATE INDEX idx_webhook_subscriptions_active ON webhook_subscriptions(client_id, active);
```

### Constraints
- Max **10 subscriptions** per client (enforced at application level)
- `event_types`: subset of `['delivered', 'failed', 'expired', 'rejected']`
- `url`: HTTPS only, max 2048 characters, private/internal IP ranges blocked (SSRF prevention)
- `secret`: server-generated, 32 bytes of `crypto/rand` hex-encoded (64 chars), returned only on creation. To rotate, client deletes and recreates the subscription.

### Migration

Up: `migrations/000008_create_webhook_tables.up.sql`
Down: `migrations/000008_create_webhook_tables.down.sql` (`DROP TABLE webhook_subscriptions;`)

## gRPC API

### Proto: `api/proto/webhookv1/webhook.proto`

```protobuf
service WebhookService {
    rpc CreateSubscription(CreateSubscriptionRequest) returns (CreateSubscriptionResponse);
    rpc UpdateSubscription(UpdateSubscriptionRequest) returns (UpdateSubscriptionResponse);
    rpc DeleteSubscription(DeleteSubscriptionRequest) returns (DeleteSubscriptionResponse);
    rpc GetSubscription(GetSubscriptionRequest) returns (GetSubscriptionResponse);
    rpc ListSubscriptions(ListSubscriptionsRequest) returns (ListSubscriptionsResponse);
}
```

**CreateSubscriptionRequest:** `client_id`, `url`, `event_types`
**CreateSubscriptionResponse:** full `subscription` object + `secret` (one-time)
**UpdateSubscriptionRequest:** `id`, `client_id`, `url` (optional), `event_types` (optional), `active` (optional)

## HTTP API

### Client Gateway

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/webhooks` | Create subscription (client_id from auth context) |
| GET | `/api/v1/webhooks` | List own subscriptions |
| GET | `/api/v1/webhooks/{id}` | Get subscription details |
| PUT | `/api/v1/webhooks/{id}` | Update subscription |
| DELETE | `/api/v1/webhooks/{id}` | Delete subscription |

### Admin Gateway

| Method | Path | Description |
|--------|------|-------------|
| POST | `/admin/v1/webhooks` | Create (client_id required in body) |
| GET | `/admin/v1/webhooks` | List (filterable by client_id) |
| GET | `/admin/v1/webhooks/{id}` | Get subscription details |
| PUT | `/admin/v1/webhooks/{id}` | Update subscription |
| DELETE | `/admin/v1/webhooks/{id}` | Delete subscription |

## Webhook Payload

### Format

```json
{
    "event_id": "uuid-v4",
    "event_type": "delivered",
    "timestamp": "2026-03-20T15:04:05Z",
    "data": {
        "message_id": "uuid",
        "external_id": "client-ref-123",
        "source": "+70001234567",
        "destination": "+79991234567",
        "status": "delivered",
        "status_message": "Message delivered to handset",
        "provider_id": "uuid",
        "submitted_at": "2026-03-20T15:03:00Z",
        "delivered_at": "2026-03-20T15:04:05Z"
    }
}
```

Fields `external_id`, `source`, `destination`, `submitted_at` are populated from the message enrichment DB lookup. `delivered_at` / `failed_at` come from the DLR/failed event timestamp. If `external_id` is empty in the original message, it is omitted from the payload (`omitempty`).

### HMAC-SHA256 Signature (replay-protected)

- Signed over `<X-Webhook-Timestamp>` + `"."` + `<raw JSON body>` using subscription `secret`
- Header: `X-Webhook-Signature: sha256=<hex-encoded-hmac>`
- Header: `X-Webhook-Timestamp: <unix-seconds>` (входит в подпись; receiver обязан проверить окно ±5 минут)
- Header: `X-Webhook-Event: <event_type>` (for filtering without parsing body)
- Header: `X-Webhook-ID: <event_id>` (for client-side idempotency)
- Header: `User-Agent: SMS-Platform-Webhook/1.0`

**Receiver verification (reference):**
1. Прочитать `X-Webhook-Timestamp`. Если `|now - ts| > 300` секунд — отклонить (stale/replay).
2. Прочитать body как **raw bytes** до любого JSON-парсинга/нормализации.
   Прочитать `X-Webhook-Signature`, отрезать префикс `sha256=`.
3. Вычислить `expected = HMAC-SHA256(ts + "." + body, secret)`.
   **ВАЖНО:** `ts` обязан быть raw header-string (`r.Header.Get("X-Webhook-Timestamp")`),
   не parsed-then-reformatted. Re-format leading-zeros / целочисленных представлений
   ломает подпись.
   **ВАЖНО:** `body` обязан быть raw bytes как пришли. Re-marshal JSON
   (другой порядок ключей / другой whitespace) даст другой HMAC.
4. Сравнить через `hmac.Equal(sig, expected)` — отклонить если не совпадает.
5. Дополнительно: dedup по `X-Webhook-ID` против повторного применения в окне.

### HTTP Request

- Method: `POST`
- Content-Type: `application/json`
- Timeout: **5 seconds**
- Redirects: **not followed** (prevent SSRF)
- Success: HTTP 2xx response
- Failure: any other status code or timeout → triggers retry

## Delivery Flow

```
1. Kafka consumer receives event from sms.dlr / sms.failed
   ↓
2. Extract message_id from event
   ↓
3. Query messages table to enrich: client_id, external_id, source, destination, submitted_at
   ↓
4. Map SMPP stat to event_type (see Event Type Mapping table)
   ↓
5. Look up active subscriptions for client_id matching event_type (from cache or DB)
   ↓
6. For each subscription: build payload, sign with HMAC-SHA256
   ↓
7. Submit to worker pool (channel-based, configurable pool size)
   ↓
8. Worker executes HTTP POST, 5s timeout
   ↓
9. If 2xx → done
   ↓
10. If error → schedule retry via time.AfterFunc
    ↓
    Attempt 1: after 15s
    Attempt 2: after 30s
    Attempt 3: after 1m
    Attempt 4: after 5m
    Attempt 5: after 15m
    ↓
11. If all 5 retries exhausted → log error, increment webhook_delivery_failed metric
```

### Key Details

- **Kafka offset committed immediately** after submitting to worker pool, not after delivery. Retry must not block processing of new events.
- **In-memory subscription cache** (map by client_id, TTL 60s) to avoid DB query per event. Stale data for up to 60s after subscription changes is an accepted trade-off for an MVP. Cache is invalidated on CRUD operations within the same instance.
- **No subscriptions for client** → event silently skipped.
- **Message not found** → event logged and skipped (can happen if message was purged from partitioned table).

## Integration Points

### Gateway Changes

**Client Gateway:**
- New gRPC client connection to Webhook Service (port 9098)
- New handler file: `internal/gateway/client/handlers/webhooks.go`
- Route registration in `internal/gateway/client/router/router.go` — add `webhookHandlers` parameter to `SetupRouter`
- New `WEBHOOK_SERVICE_ADDR` environment variable in `cmd/client-gateway/main.go`

**Admin Gateway:**
- New gRPC client connection to Webhook Service
- New handler file: `internal/gateway/admin/handlers/webhooks.go`
- Route registration in `internal/gateway/admin/router/router.go` — add `webhookHandlers` parameter to `SetupRouter`
- New `WEBHOOK_SERVICE_ADDR` environment variable in `cmd/admin-gateway/main.go`

### Docker Compose

New service `webhook-service` in `deployments/docker-compose.yml`:
- Image: `service-base.Dockerfile`
- Port: 9098 (gRPC), 2119 (metrics)
- Dependencies: postgres, kafka
- Environment: `DATABASE_URL`, `KAFKA_BROKERS`

Add `WEBHOOK_SERVICE_ADDR=webhook-service:9098` to both `client-gateway` and `admin-gateway` services.

### No Changes to Existing Services

Webhook Service attaches as a new Kafka consumer to existing topics and reads from the shared `messages` table. No modifications needed in Messaging, Billing, Analytics, Provider, Routing, Client, or Auth services. No Kafka schema changes.

## Metrics (Prometheus)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `webhook_deliveries_total` | Counter | `event_type`, `status` (success/failed) | Total delivery attempts |
| `webhook_delivery_duration_seconds` | Histogram | `event_type` | HTTP request duration |
| `webhook_retry_total` | Counter | `attempt` (1-5) | Retry attempts |
| `webhook_delivery_failed` | Counter | `event_type` | Exhausted all retries |
| `webhook_worker_pool_size` | Gauge | — | Current worker pool utilization |
| `webhook_subscriptions_total` | Gauge | — | Active subscriptions count |

Exposed on port **2119** at `/metrics`. Added to Prometheus scrape config.

## Configuration

```yaml
webhook:
  grpc_port: 9098
  metrics_port: 2119
  worker_pool_size: 50
  http_timeout: 5s
  max_retries: 5
  retry_backoff: [15s, 30s, 1m, 5m, 15m]
  subscription_cache_ttl: 60s
  max_subscriptions_per_client: 10
  shutdown_drain_timeout: 30s
```
