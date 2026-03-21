# Portal API Contract

**Base URL**: `/portal/v1`
**Auth**: Session cookie (`portal_session`) + CSRF token (`X-CSRF-Token` header for mutations)
**Format**: JSON request/response, `Content-Type: application/json`

## Authentication

### POST /auth/login

Login с email/паролем. Если 2FA активна — возвращает `requires_2fa: true` без создания сессии.

**Request**:
```json
{
  "email": "user@example.com",
  "password": "string"
}
```

**Response 200** (без 2FA):
```json
{
  "user": {
    "id": "uuid",
    "email": "string",
    "username": "string",
    "role": "client",
    "client_id": "uuid",
    "is_reseller": false,
    "is_sub_account": false
  }
}
```
Set-Cookie: `portal_session=...; HttpOnly; Secure; SameSite=Strict; Max-Age=86400`
Set-Cookie: `csrf_token=...; SameSite=Strict; Max-Age=86400`

**Response 200** (с 2FA):
```json
{
  "requires_2fa": true,
  "login_ticket": "string"
}
```

**Errors**: 401 (invalid credentials), 429 (too many attempts, Retry-After: 900)

### POST /auth/login/2fa

Завершение логина с TOTP-кодом.

**Request**:
```json
{
  "login_ticket": "string",
  "totp_code": "123456"
}
```

**Response 200**: Same as login success.

**Errors**: 401 (invalid code), 410 (ticket expired)

### POST /auth/logout

**Response 204**: No content. Удаляет сессию.

### POST /auth/password/reset-request

**Request**:
```json
{
  "email": "user@example.com"
}
```

**Response 202**: Accepted (всегда, для предотвращения email enumeration).

### POST /auth/password/reset

**Request**:
```json
{
  "token": "string",
  "new_password": "string"
}
```

**Response 200**: `{"message": "Password updated"}`

**Errors**: 400 (weak password), 410 (token expired/used)

## Profile & Security

### GET /profile

**Response 200**:
```json
{
  "id": "uuid",
  "email": "string",
  "username": "string",
  "contact_person": "string",
  "phone": "string",
  "totp_enabled": false,
  "client": {
    "id": "uuid",
    "name": "string",
    "is_reseller": false,
    "is_sub_account": false,
    "parent_client_name": "string | null"
  }
}
```

### PUT /profile

**Request**:
```json
{
  "contact_person": "string",
  "phone": "string"
}
```

**Response 200**: Updated profile.

### POST /profile/2fa/setup

Инициирует настройку 2FA. Возвращает QR-код.

**Response 200**:
```json
{
  "secret": "BASE32STRING",
  "qr_code_url": "otpauth://totp/SMSPortal:user@example.com?secret=...",
  "recovery_codes": ["code1", "code2", "..."]
}
```

### POST /profile/2fa/verify

Подтверждает настройку 2FA кодом из приложения.

**Request**:
```json
{
  "totp_code": "123456"
}
```

**Response 200**: `{"totp_enabled": true}`

**Errors**: 400 (invalid code)

### DELETE /profile/2fa

Отключает 2FA (требует текущий пароль).

**Request**:
```json
{
  "password": "string"
}
```

**Response 200**: `{"totp_enabled": false}`

## Dashboard

### GET /dashboard

**Response 200**:
```json
{
  "balance": "1500.00",
  "currency": "RUB",
  "messages_today": 150,
  "messages_delivered_today": 142,
  "delivery_rate_today": 94.7,
  "active_api_keys": 3,
  "active_webhooks": 2
}
```

## Messages

### GET /messages

**Query params**: `page` (int, default 1), `per_page` (int, default 50, max 100), `status` (string), `date_from` (ISO 8601), `date_to` (ISO 8601), `destination` (string, prefix match)

**Response 200**:
```json
{
  "data": [
    {
      "id": "uuid",
      "message_id": "string",
      "source": "+79001234567",
      "destination": "+79007654321",
      "text": "Hello...",
      "status": "delivered",
      "segment_count": 1,
      "cost": "2.50",
      "currency": "RUB",
      "submitted_at": "2026-03-21T10:00:00Z",
      "delivered_at": "2026-03-21T10:00:05Z"
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 50,
    "total": 500,
    "total_pages": 10
  }
}
```

## API Keys

### GET /api-keys

**Response 200**:
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "Production",
      "key_prefix": "sk_live_abc1",
      "active": true,
      "allowed_ips": ["192.168.1.0/24"],
      "scopes": ["messages:write", "messages:read"],
      "created_at": "2026-03-01T00:00:00Z",
      "last_used_at": "2026-03-21T09:00:00Z",
      "expires_at": null
    }
  ]
}
```

### POST /api-keys

**Request**:
```json
{
  "name": "Production",
  "allowed_ips": ["192.168.1.0/24"],
  "scopes": ["messages:write", "messages:read"],
  "expires_at": "2027-03-01T00:00:00Z"
}
```

**Response 201**:
```json
{
  "id": "uuid",
  "name": "Production",
  "key": "sk_live_abc123def456...",
  "key_prefix": "sk_live_abc1",
  "allowed_ips": ["192.168.1.0/24"],
  "scopes": ["messages:write", "messages:read"],
  "created_at": "2026-03-21T10:00:00Z",
  "expires_at": "2027-03-01T00:00:00Z"
}
```

**Note**: `key` возвращается ТОЛЬКО в этом ответе.

### DELETE /api-keys/{id}

**Response 204**: No content.

## Webhooks

### GET /webhooks

**Response 200**:
```json
{
  "data": [
    {
      "id": "uuid",
      "url": "https://example.com/dlr",
      "event_types": ["delivered", "failed"],
      "active": true,
      "failure_count": 0,
      "created_at": "2026-03-01T00:00:00Z"
    }
  ]
}
```

### POST /webhooks

**Request**:
```json
{
  "url": "https://example.com/dlr",
  "event_types": ["delivered", "failed", "expired"]
}
```

**Response 201**: Created webhook with `secret` (HMAC key, shown once).

### PUT /webhooks/{id}

**Request**:
```json
{
  "url": "https://example.com/dlr-v2",
  "event_types": ["delivered", "failed"]
}
```

**Response 200**: Updated webhook.

### DELETE /webhooks/{id}

**Response 204**: No content.

### POST /webhooks/{id}/test

**Response 200**:
```json
{
  "success": true,
  "status_code": 200,
  "response_time_ms": 150
}
```

## Analytics

### GET /analytics

**Query params**: `period` (7d, 30d, 90d, custom), `date_from`, `date_to`, `group_by` (day, week, country)

**Response 200**:
```json
{
  "summary": {
    "total_sent": 5000,
    "total_delivered": 4750,
    "total_failed": 200,
    "total_expired": 50,
    "delivery_rate": 95.0,
    "total_cost": "12500.00",
    "currency": "RUB"
  },
  "timeline": [
    {
      "date": "2026-03-20",
      "sent": 200,
      "delivered": 190,
      "failed": 8,
      "expired": 2,
      "cost": "500.00"
    }
  ],
  "by_country": [
    {
      "country": "RU",
      "sent": 4000,
      "delivered": 3850,
      "delivery_rate": 96.25,
      "cost": "10000.00"
    }
  ]
}
```

## Sub-accounts (только для реселлеров)

### GET /sub-accounts

**Response 200**:
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "Компания А",
      "email": "company-a@example.com",
      "active": true,
      "balance": "1000.00",
      "currency": "RUB",
      "daily_limit": 500,
      "monthly_limit": 10000,
      "messages_today": 150,
      "messages_this_month": 3200,
      "created_at": "2026-03-01T00:00:00Z"
    }
  ],
  "limits": {
    "max_sub_accounts": 50,
    "current_count": 5
  }
}
```

### POST /sub-accounts

**Request**:
```json
{
  "name": "Компания А",
  "email": "admin@company-a.com",
  "contact_person": "Иван Иванов",
  "initial_balance": "1000.00",
  "daily_limit": 500,
  "monthly_limit": 10000
}
```

**Response 201**: Created sub-account.

**Errors**: 400 (insufficient balance, limit reached), 403 (not a reseller)

### GET /sub-accounts/{id}

**Response 200**: Full sub-account details + statistics.

### PUT /sub-accounts/{id}/limits

**Request**:
```json
{
  "daily_limit": 1000,
  "monthly_limit": 20000
}
```

**Response 200**: Updated limits.

### POST /sub-accounts/{id}/transfer

**Request**:
```json
{
  "amount": "500.00"
}
```

**Response 200**:
```json
{
  "transfer_id": "uuid",
  "from_balance": "9500.00",
  "to_balance": "1500.00"
}
```

**Errors**: 400 (insufficient balance)

### DELETE /sub-accounts/{id}

**Response 200**:
```json
{
  "returned_balance": "500.00",
  "message": "Sub-account deleted, balance returned"
}
```

### GET /sub-accounts/{id}/messages

Same format as GET /messages, filtered by sub-account.

### GET /sub-accounts/{id}/analytics

Same format as GET /analytics, filtered by sub-account.

### GET /sub-accounts/{id}/api-keys

Same format as GET /api-keys, filtered by sub-account.

### GET /sub-accounts/{id}/webhooks

Same format as GET /webhooks, filtered by sub-account.

## Audit Log

### GET /audit-log

**Query params**: `page`, `per_page`, `action` (string), `date_from`, `date_to`, `user_id` (uuid)

**Response 200**:
```json
{
  "data": [
    {
      "id": "uuid",
      "action": "api_key.created",
      "resource_type": "api_key",
      "resource_id": "uuid",
      "user": {
        "id": "uuid",
        "email": "user@example.com"
      },
      "details": {"key_name": "Production"},
      "ip_address": "192.168.1.1",
      "created_at": "2026-03-21T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 50,
    "total": 200,
    "total_pages": 4
  }
}
```

## Common Error Format

```json
{
  "error": {
    "code": "INSUFFICIENT_BALANCE",
    "message": "Insufficient balance for this operation",
    "details": {
      "required": "1000.00",
      "available": "500.00"
    }
  }
}
```

## HTTP Status Codes

| Code | Usage |
|------|-------|
| 200  | Success |
| 201  | Created |
| 202  | Accepted (async) |
| 204  | No content (delete) |
| 400  | Bad request / validation |
| 401  | Unauthorized |
| 403  | Forbidden |
| 404  | Not found |
| 409  | Conflict |
| 410  | Gone (expired token) |
| 429  | Rate limited |
| 500  | Internal server error |
