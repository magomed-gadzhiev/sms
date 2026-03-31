# Webhook Contract: Max Messenger → Cascade Service

## Endpoint

```
POST /webhooks/cascade/max_messenger
Content-Type: application/json
X-Max-Signature: <HMAC-SHA256 hex digest>
```

## Authentication

HMAC-SHA256 подпись тела запроса. Секрет хранится в `delivery_channels.config.webhook_secret`.

Проверка:
```
expected = HMAC-SHA256(webhook_secret, request_body)
actual   = request.Header["X-Max-Signature"]
valid    = hmac.Equal(expected, actual)
```

## Request Body

```json
{
  "attempt_id": "uuid",
  "message_id": "max-internal-msg-id",
  "status": "delivered",
  "error": "",
  "timestamp": "2026-04-01T10:00:00Z"
}
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| attempt_id | string (UUID) | Yes | ID попытки доставки (external_id из SendRequest) |
| message_id | string | Yes | Внутренний ID сообщения в Max |
| status | string | Yes | `delivered` / `read` / `error` |
| error | string | No | Описание ошибки (при status=error) |
| timestamp | string (RFC3339) | Yes | Время события в Max |

### Status Mapping

| Max Status | Attempt Status |
|------------|---------------|
| `delivered` | `delivered` |
| `read` | `delivered` (read — подтверждение доставки) |
| `error` | `failed` |

## Response

### Success (200)
```json
{"ok": true}
```

### Invalid Signature (401)
```json
{"error": "invalid signature"}
```

### Unknown Attempt (404)
```json
{"error": "attempt not found"}
```

## Late Duplicate Handling

Если `delivery.status` уже terminal (`delivered`, `failed`, `cancelled`):
- attempt помечается `late_duplicate`
- биллинг НЕ выполняется
- HTTP 200 возвращается (подтверждение получения)
