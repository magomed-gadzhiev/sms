# Client API Contract

**Date**: 2026-03-20
**Protocol**: HTTP REST (port 8080) + gRPC (port 9090)
**Authentication**: API Key via `X-API-Key` header

## HTTP REST Endpoints

### Send Single SMS

```
POST /api/v1/sms/send
Content-Type: application/json
X-API-Key: {api_key}

Request:
{
  "source": "+79001234567",
  "destination": "+79007654321",
  "body": "Hello, world!",
  "external_id": "client-ref-123",        // optional
  "scheduled_at": "2026-03-21T10:00:00Z",  // optional, max 30 days
  "template_id": "uuid",                   // optional
  "template_variables": {"name": "John"}   // required if template_id set
}

Response 200:
{
  "message_id": "uuid",
  "status": "queued" | "scheduled",
  "segment_count": 1,
  "created_at": "2026-03-20T12:00:00Z"
}

Error 400: { "error": "validation_error", "message": "..." }
Error 401: { "error": "unauthorized", "message": "Invalid API key" }
Error 402: { "error": "insufficient_balance", "message": "...", "balance": 0.00 }
Error 429: { "error": "rate_limit_exceeded", "message": "...", "retry_after": 1 }
```

### Send Batch SMS

```
POST /api/v1/sms/batch
Content-Type: application/json
X-API-Key: {api_key}

Request:
{
  "messages": [
    {
      "source": "+79001234567",
      "destination": "+79007654321",
      "body": "Message 1",
      "external_id": "ref-1"
    },
    ...
  ]
}

Response 200:
{
  "results": [
    { "message_id": "uuid", "status": "queued", "segment_count": 1 },
    { "error": "invalid_destination", "index": 2 }
  ],
  "total": 100,
  "accepted": 98,
  "rejected": 2
}

Error 400: { "error": "batch_too_large", "message": "Maximum 10000 messages per batch" }
```

### Get Message Status

```
GET /api/v1/sms/status/{message_id}
X-API-Key: {api_key}

Response 200:
{
  "message_id": "uuid",
  "external_id": "client-ref-123",
  "status": "delivered",
  "segment_count": 1,
  "source": "+79001234567",
  "destination": "+79007654321",
  "created_at": "2026-03-20T12:00:00Z",
  "sent_at": "2026-03-20T12:00:01Z",
  "delivered_at": "2026-03-20T12:00:05Z"
}

Error 404: { "error": "not_found", "message": "Message not found" }
Error 403: { "error": "forbidden", "message": "Access denied" }
```

### Get Message History

```
GET /api/v1/sms/history?status=delivered&from=2026-03-01&to=2026-03-20&page=1&per_page=50
X-API-Key: {api_key}

Response 200:
{
  "messages": [...],
  "pagination": {
    "page": 1,
    "per_page": 50,
    "total": 1234
  }
}
```

### Cancel Scheduled Message

```
POST /api/v1/sms/cancel/{message_id}
X-API-Key: {api_key}

Response 200:
{
  "message_id": "uuid",
  "status": "cancelled"
}

Error 409: { "error": "conflict", "message": "Message is not in scheduled status" }
```

### Get Account Balance

```
GET /api/v1/account/balance
X-API-Key: {api_key}

Response 200:
{
  "balance": 1500.00,
  "currency": "RUB"
}
```

### Webhook Management

```
POST   /api/v1/webhooks          - Create subscription
GET    /api/v1/webhooks          - List subscriptions
GET    /api/v1/webhooks/{id}     - Get subscription
PUT    /api/v1/webhooks/{id}     - Update subscription
DELETE /api/v1/webhooks/{id}     - Delete subscription
```

## Webhook Callback Contract

```
POST {client_webhook_url}
Content-Type: application/json
X-Signature: HMAC-SHA256({secret}, {body})

{
  "event": "message.delivered",
  "message_id": "uuid",
  "external_id": "client-ref-123",
  "status": "delivered",
  "source": "+79001234567",
  "destination": "+79007654321",
  "segment_count": 1,
  "timestamp": "2026-03-20T12:00:05Z"
}

Expected response: 2xx status code
Retry policy: exponential backoff, max 5 attempts
```

## gRPC Service (proto: messaging.proto)

```protobuf
service MessagingService {
  rpc SendMessage(SendMessageRequest) returns (SendMessageResponse);
  rpc SendBatch(SendBatchRequest) returns (SendBatchResponse);
  rpc GetMessageStatus(GetStatusRequest) returns (GetStatusResponse);
  rpc CancelMessage(CancelMessageRequest) returns (CancelMessageResponse);
}
```
