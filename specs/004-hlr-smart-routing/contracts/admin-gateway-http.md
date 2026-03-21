# HTTP Contract: Admin Gateway — HLR Management

**Base path**: `/admin/v1` (расширение существующего admin-gateway)
**Auth**: Bearer token (как существующие admin endpoints)

## HLR Provider Management

### POST /admin/v1/hlr/providers

Создать HLR-провайдера.

**Request**:
```json
{
  "name": "TeleSign HLR",
  "adapter_type": "http_rest",
  "config": {
    "base_url": "https://api.telesign.com/v1/phoneid",
    "api_key": "...",
    "timeout_ms": 3000
  },
  "priority": 1,
  "supported_regions": ["RU", "UA", "BY", "KZ", "DE", "FR"],
  "cost_per_lookup": "0.005000"
}
```

**Response 201**: Created HLRProvider object.

### GET /admin/v1/hlr/providers

Список всех HLR-провайдеров.

**Response 200**:
```json
{
  "providers": [
    {
      "id": "uuid",
      "name": "TeleSign HLR",
      "adapter_type": "http_rest",
      "priority": 1,
      "supported_regions": ["RU", "UA", "BY", "KZ", "DE", "FR"],
      "cost_per_lookup": "0.005000",
      "status": "healthy",
      "success_rate": "99.50",
      "last_success_at": "2026-03-21T11:59:00Z",
      "active": true,
      "created_at": "2026-03-01T00:00:00Z"
    }
  ]
}
```

### PUT /admin/v1/hlr/providers/{id}

Обновить конфигурацию провайдера.

### DELETE /admin/v1/hlr/providers/{id}

Удалить провайдера (soft delete — active=false).

### GET /admin/v1/hlr/providers/{id}/health

Текущий health status и метрики провайдера.

---

## Smart Route Weights

### POST /admin/v1/routing/weights

Создать/обновить веса для оператора/региона.

**Request**:
```json
{
  "country_code": "RU",
  "operator_code": "25001",
  "cost_weight": "0.70",
  "quality_weight": "0.30"
}
```

### GET /admin/v1/routing/weights

Список всех настроенных весов.

**Query params**: `country_code`, `operator_code` (опциональные фильтры).

### DELETE /admin/v1/routing/weights/{id}

Удалить кастомные веса (вернуть к defaults).
