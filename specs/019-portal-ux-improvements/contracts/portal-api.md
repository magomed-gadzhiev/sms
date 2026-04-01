# Portal API Contracts: Portal UX Improvements

**Feature**: 019-portal-ux-improvements  
**Base URL**: `/portal/v1`  
**Auth**: Session cookie + CSRF token (все эндпоинты)

---

## New Endpoints

### Notifications

#### `GET /notifications`

Получить список уведомлений пользователя.

**Query params**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 20 | Максимум записей |
| `offset` | int | 0 | Смещение |

**Response 200**:
```json
{
  "items": [
    {
      "id": "uuid",
      "type": "campaign_completed",
      "body": "Кампания «Летняя акция» завершена: 950/1000 доставлено",
      "object_type": "campaign",
      "object_id": "uuid",
      "is_read": false,
      "created_at": "2026-04-01T12:00:00Z"
    }
  ],
  "unread_count": 3
}
```

---

#### `POST /notifications/{id}/read`

Отметить уведомление как прочитанное.

**Path params**: `id` — UUID уведомления

**Response 204**: No content

**Error 404**: Уведомление не найдено или не принадлежит текущему пользователю

---

#### `POST /notifications/read-all`

Отметить все уведомления как прочитанные.

**Response 204**: No content

---

### Global Search (Command Palette)

#### `GET /search`

Поиск по записям БД для палитры команд.

**Query params**:
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `q` | string | yes | Поисковый запрос (минимум 2 символа) |

**Response 200**:
```json
{
  "items": [
    {
      "id": "uuid",
      "label": "Летняя акция 2026",
      "type": "record",
      "category": "templates",
      "href": "/templates/uuid"
    },
    {
      "id": "uuid",
      "label": "SMS_REKLAMA",
      "type": "record",
      "category": "sender_names",
      "href": "/sender-names/uuid"
    }
  ]
}
```

**Searched entities**: templates (name), sender_names (name), contact_lists (name)  
**Max results**: 10 (3 per category)  
**Response time target**: < 200ms (indexed name fields)

---

### Async Export

#### `POST /export/start`

Запустить асинхронную генерацию файла экспорта.

**Request body**:
```json
{
  "entity_type": "messages",
  "format": "csv",
  "filters": {
    "status": "delivered",
    "from": "2026-03-01",
    "to": "2026-03-31"
  }
}
```

**entity_type values**: `messages`, `transactions`, `contacts`, `audit_log`

**Response 202**:
```json
{
  "job_id": "uuid",
  "status": "pending"
}
```

---

#### `GET /export/{job_id}/status`

Получить статус задачи экспорта.

**Response 200**:
```json
{
  "job_id": "uuid",
  "status": "ready",
  "total_rows": 15432,
  "download_url": "/portal/v1/export/uuid/download"
}
```

**status values**: `pending`, `processing`, `ready`, `error`

---

#### `GET /export/{job_id}/download`

Скачать готовый файл экспорта.

**Response 200**: 
- `Content-Type: text/csv; charset=utf-8`
- `Content-Disposition: attachment; filename=messages_2026-04-01.csv`

**Error 404**: Задача не найдена или истёк TTL (1 час)  
**Error 403**: Задача принадлежит другому пользователю  
**Error 409**: Задача ещё не готова (status != ready)

---

## Modified Endpoints

### `GET /analytics` (расширение)

Добавляются новые query params:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `compare` | bool | false | Включить сравнение с предыдущим периодом |
| `include_cost` | bool | false | Включить данные стоимости по дням |

**Response 200** (дополнительные поля при `compare=true`):
```json
{
  "summary": { ... },
  "timeline": [ ... ],
  "by_country": [ ... ],
  "previous_timeline": [
    {
      "period": "2026-02-01",
      "sent": 1200,
      "delivered": 1080,
      "failed": 120,
      "delivery_rate": 90.0
    }
  ],
  "comparison": {
    "sent_change_pct": 15.3,
    "delivered_change_pct": 12.1,
    "delivery_rate_change_pct": -2.5
  }
}
```

**Response 200** (дополнительные поля при `include_cost=true`):
```json
{
  "cost_by_day": [
    { "date": "2026-03-25", "cost": "1250.50" }
  ],
  "cost_forecast": "38500.00"
}
```

---

### `GET /dashboard` (расширение)

Добавляется поле `charts` с данными для дашборда:

**Response 200** (новые поля):
```json
{
  "balance": "...",
  "currency": "RUB",
  "messages_today": 150,
  "charts": {
    "timeline_7d": [
      { "date": "2026-03-26", "sent": 120, "delivered": 110, "failed": 10 }
    ],
    "status_distribution": [
      { "status": "delivered", "count": 4500 },
      { "status": "failed", "count": 300 },
      { "status": "expired", "count": 50 }
    ],
    "top_countries": [
      { "country": "RU", "sent": 3200 },
      { "country": "KZ", "sent": 800 }
    ],
    "trend": {
      "messages_change_pct": 12.5,
      "delivery_rate_change_pct": -1.2,
      "cost_change_pct": 8.3
    }
  }
}
```

---

## Error Format (unchanged)

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Уведомление не найдено"
  }
}
```

