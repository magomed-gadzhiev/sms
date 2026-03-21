# HTTP Contract: Client Gateway — Lookup API

**Base path**: `/api/v1` (расширение существующего client-gateway)
**Auth**: API key в заголовке (как существующие endpoints)

## POST /api/v1/lookup

Выполняет HLR-lookup для одного номера.

**Request**:
```json
{
  "msisdn": "+79001234567",
  "force_refresh": false
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| msisdn | string | yes | Номер в формате E.164 |
| force_refresh | bool | no | Принудительный HLR-запрос (default: false) |

**Response 200**:
```json
{
  "msisdn": "+79001234567",
  "operator": {
    "mccmnc": "25001",
    "name": "MTS"
  },
  "status": "active",
  "country": "RU",
  "number_type": "mobile",
  "is_ported": true,
  "original_operator": {
    "mccmnc": "25002",
    "name": "MegaFon"
  },
  "cached": true,
  "queried_at": "2026-03-21T10:00:00Z"
}
```

**Response 400** (invalid number format):
```json
{
  "error": "invalid_msisdn",
  "message": "Invalid phone number format. Use E.164 format (e.g., +79001234567)"
}
```

**Response 402** (insufficient balance):
```json
{
  "error": "insufficient_balance",
  "message": "Not enough balance to perform lookup"
}
```

**Response 429** (rate limit exceeded):
```json
{
  "error": "rate_limit_exceeded",
  "message": "Rate limit exceeded",
  "retry_after": 30
}
```

---

## POST /api/v1/lookup/bulk

Пакетный lookup до 1000 номеров.

**Request**:
```json
{
  "msisdns": ["+79001234567", "+49151234567", "+380501234567"],
  "force_refresh": false
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| msisdns | string[] | yes | Номера (max 1000) |
| force_refresh | bool | no | Bypass cache (default: false) |

**Response 200**:
```json
{
  "results": [
    {
      "msisdn": "+79001234567",
      "operator": { "mccmnc": "25001", "name": "MTS" },
      "status": "active",
      "country": "RU",
      "number_type": "mobile",
      "is_ported": true,
      "original_operator": { "mccmnc": "25002", "name": "MegaFon" },
      "cached": true,
      "queried_at": "2026-03-21T10:00:00Z"
    },
    {
      "msisdn": "+49151234567",
      "operator": { "mccmnc": "26201", "name": "T-Mobile" },
      "status": "active",
      "country": "DE",
      "number_type": "mobile",
      "is_ported": false,
      "cached": false,
      "queried_at": "2026-03-21T12:05:30Z"
    },
    {
      "msisdn": "+380501234567",
      "error": "lookup_failed",
      "message": "HLR query failed for this number"
    }
  ],
  "summary": {
    "total": 3,
    "success": 2,
    "failed": 1
  }
}
```

**Response 400** (validation error):
```json
{
  "error": "validation_error",
  "message": "Too many numbers in batch (max 1000)"
}
```

**Response 402** (insufficient balance for full batch):
```json
{
  "error": "insufficient_balance",
  "message": "Balance sufficient for 150 of 1000 lookups",
  "available_lookups": 150
}
```

---

## GET /api/v1/lookup/history

История lookup-запросов клиента.

**Query params**:
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| from | ISO 8601 | no | Начало периода |
| to | ISO 8601 | no | Конец периода |
| msisdn | string | no | Фильтр по номеру |
| source | string | no | sms_routing / api_lookup |
| page | int | no | Страница (default: 1) |
| page_size | int | no | Размер страницы (default: 50, max: 100) |

**Response 200**:
```json
{
  "items": [
    {
      "id": "uuid",
      "msisdn": "+79001234567",
      "operator_mccmnc": "25001",
      "operator_name": "MTS",
      "status": "active",
      "country": "RU",
      "number_type": "mobile",
      "is_ported": true,
      "source": "api_lookup",
      "cached": false,
      "latency_ms": 145,
      "created_at": "2026-03-21T12:00:00Z"
    }
  ],
  "total": 1500,
  "page": 1,
  "page_size": 50
}
```
