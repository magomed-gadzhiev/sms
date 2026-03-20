# Webhook Notifications & Callback API — Design Spec

## Problem

Clients must poll `GET /api/v1/sms/status/{id}` to get delivery statuses. This is inefficient, adds latency, and increases platform load. No enterprise client wants to poll — webhooks are the industry standard.

## Solution Overview

A new **Webhook Service** microservice that consumes existing Kafka DLR events and delivers HTTP callbacks to client-registered endpoints with HMAC-SHA256 signature verification and retry logic.

## Scope

### In Scope
- Webhook subscriptions CRUD (Client API + Admin API)
- Delivery status events: `delivered`, `failed`, `expired`, `rejected`
- HMAC-SHA256 payload signing
- Retry with exponential backoff (5 attempts)
- In-process delivery via goroutine worker pool

### Out of Scope
- Billing/system events (future feature)
- Delivery log / history of webhook attempts
- Dead letter queue / replay API
- Auto-disable on repeated failures

## Architecture

### Approach: Embedded Kafka Consumer Service

New microservice `webhook-service` following existing DDD patterns. Consumes from existing Kafka topics (`sms.dlr`, `sms.failed`), delivers HTTP POST to client endpoints. Retry managed in-process via `time.AfterFunc`.

**Trade-off accepted:** Pending retries are lost on service restart. Clients use polling (`GET /api/v1/sms/status/{id}`) as fallback.

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
      │   └── subscription_repository.go
      └── http/
          └── delivery_client.go    # HTTP client for webhook delivery
```

- gRPC port: **9098**
- Kafka consumer group: `webhook-service`
- Consumes topics: `sms.dlr`, `sms.failed`

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
- Max **10 subscriptions** per client
- `event_types`: subset of `['delivered', 'failed', 'expired', 'rejected']`
- `url`: HTTPS only (validated on create/update)
- `secret`: server-generated, returned only on creation

### Migration

New file: `migrations/000008_create_webhook_tables.up.sql`

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

### HMAC-SHA256 Signature

- Signed over raw JSON body using subscription `secret`
- Header: `X-Webhook-Signature: sha256=<hex-encoded-hmac>`
- Header: `X-Webhook-Event: <event_type>` (for filtering without parsing body)
- Header: `X-Webhook-ID: <event_id>` (for client-side idempotency)

### HTTP Request

- Method: `POST`
- Content-Type: `application/json`
- Timeout: **5 seconds**
- Success: HTTP 2xx response
- Failure: any other status code or timeout → triggers retry

## Delivery Flow

```
1. Kafka consumer receives event from sms.dlr / sms.failed
   ↓
2. Parse DLR, determine event_type (delivered/failed/expired/rejected)
   ↓
3. Query active subscriptions for client matching event_type
   ↓
4. For each subscription: build payload, sign with HMAC-SHA256
   ↓
5. Submit to worker pool (channel-based, configurable pool size)
   ↓
6. Worker executes HTTP POST, 5s timeout
   ↓
7. If 2xx → done
   ↓
8. If error → schedule retry via time.AfterFunc
   ↓
   Attempt 1: after 15s
   Attempt 2: after 30s
   Attempt 3: after 1m
   Attempt 4: after 5m
   Attempt 5: after 15m
   ↓
9. If all 5 retries exhausted → log error, increment webhook_delivery_failed metric
```

### Key Details

- **Kafka offset committed immediately** after submitting to worker pool, not after delivery. Retry must not block processing of new events.
- **In-memory subscription cache** (map by client_id, TTL 60s) to avoid DB query per event.
- **No subscriptions for client** → event silently skipped.

## Integration Points

### Gateway Changes

**Client Gateway:**
- New gRPC client connection to Webhook Service (port 9098)
- New handler file: `internal/gateway/client/handlers/webhooks.go`
- Route registration in `internal/gateway/client/router/router.go`

**Admin Gateway:**
- New gRPC client connection to Webhook Service
- New handler file: `internal/gateway/admin/handlers/webhooks.go`
- Route registration in `internal/gateway/admin/router/router.go`

### Docker Compose

New service `webhook-service` in `deployments/docker-compose.yml`:
- Image: `service-base.Dockerfile`
- Port: 9098
- Dependencies: postgres, kafka

### No Changes to Existing Services

Webhook Service attaches as a new Kafka consumer to existing topics. No modifications needed in Messaging, Billing, Analytics, Provider, Routing, Client, or Auth services.

## Configuration

```yaml
webhook:
  worker_pool_size: 50
  http_timeout: 5s
  max_retries: 5
  retry_backoff: [15s, 30s, 1m, 5m, 15m]
  subscription_cache_ttl: 60s
  max_subscriptions_per_client: 10
```
