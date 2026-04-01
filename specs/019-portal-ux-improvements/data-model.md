# Data Model: Portal UX Improvements

**Feature**: 019-portal-ux-improvements  
**Phase**: 1 — Design  
**Date**: 2026-04-01

---

## New Database Entities

### 1. `notifications` (PostgreSQL)

Хранит уведомления для пользователей портала. Таблица принадлежит portal-gateway.

```sql
CREATE TABLE notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,          -- ссылка на client_id (из сессии)
    type        VARCHAR(64) NOT NULL,   -- тип события (см. ниже)
    body        TEXT NOT NULL,          -- текст уведомления
    object_type VARCHAR(64),            -- 'campaign' | 'template' | 'sender_name' | 'billing'
    object_id   VARCHAR(255),           -- ID связанного объекта
    is_read     BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user_id ON notifications (user_id, created_at DESC);
CREATE INDEX idx_notifications_unread  ON notifications (user_id, is_read) WHERE is_read = false;
```

**Notification types** (`type` field):
| Value | Description |
|---|---|
| `campaign_completed` | Кампания успешно завершена |
| `campaign_failed` | Кампания завершена с ошибкой |
| `template_approved` | Шаблон одобрен |
| `template_rejected` | Шаблон отклонён |
| `sender_name_approved` | Имя отправителя одобрено |
| `sender_name_rejected` | Имя отправителя отклонено |
| `low_balance` | Баланс ниже порогового значения |
| `export_ready` | Файл экспорта готов к скачиванию |

**Data retention**: Уведомления хранятся 90 дней (автоочистка через `pg_cron` или scheduled job).

**Partition**: Не требуется — ожидаемый объём невысокий (< 1K уведомлений на пользователя в месяц).

---

## Redis Structures (Async Export Jobs)

### Export Job State

**Key**: `export:job:{uuid}`  
**Type**: Hash  
**TTL**: 3600 сек (1 час)

```
Fields:
  status       string   "pending" | "processing" | "ready" | "error"
  format       string   "csv"
  entity_type  string   "messages" | "transactions" | "contacts" | "audit_log"
  filters_json string   JSON-строка с применёнными фильтрами
  file_path    string   /tmp/export_{uuid}.csv  (когда status=ready)
  total_rows   int      количество строк в файле (когда status=ready)
  error_msg    string   сообщение об ошибке (когда status=error)
  created_at   string   RFC3339 timestamp
  user_id      string   UUID пользователя (для авторизации при скачивании)
```

---

## Frontend Data Structures (TypeScript)

### Notification

```typescript
interface Notification {
  id: string;
  type: NotificationType;
  body: string;
  object_type?: string;
  object_id?: string;
  is_read: boolean;
  created_at: string; // ISO 8601
}

type NotificationType =
  | 'campaign_completed'
  | 'campaign_failed'
  | 'template_approved'
  | 'template_rejected'
  | 'sender_name_approved'
  | 'sender_name_rejected'
  | 'low_balance'
  | 'export_ready';

interface NotificationsResponse {
  items: Notification[];
  unread_count: number;
}
```

### Export Job

```typescript
interface ExportJob {
  job_id: string;
  status: 'pending' | 'processing' | 'ready' | 'error';
  total_rows?: number;
  error_msg?: string;
  download_url?: string; // /portal/v1/export/{job_id}/download
}
```

### Dashboard Chart Data

```typescript
// Добавляется к существующему DashboardData
interface DashboardChartData {
  timeline: Array<{
    date: string;     // "YYYY-MM-DD"
    sent: number;
    delivered: number;
    failed: number;
  }>;
  status_distribution: Array<{
    status: string;   // "delivered" | "failed" | "pending" | "expired"
    count: number;
  }>;
  top_countries: Array<{
    country: string;
    sent: number;
  }>;
  trend: {
    messages_change_pct: number;  // % изменение vs предыдущий период
    delivery_rate_change_pct: number;
    cost_change_pct: number;
  };
}
```

### Analytics Comparison Data

```typescript
// Добавляется к существующему AnalyticsData
interface AnalyticsDataExtended extends AnalyticsData {
  previous_timeline?: TimelineEntry[];  // когда compare=true
  cost_by_day?: Array<{
    date: string;
    cost: string;  // RUB, строка для точности
  }>;
  cost_forecast?: string;  // прогноз до конца периода, RUB
}
```

### Command Palette Types

```typescript
interface CommandItem {
  id: string;
  label: string;
  description?: string;
  icon?: string;        // имя иконки секции
  type: 'page' | 'action' | 'record';
  href?: string;        // для page/record
  action?: () => void;  // для action
  category: string;     // для группировки результатов
}
```

### Bulk Action Types

```typescript
interface BulkAction<T = unknown> {
  id: string;
  label: string;
  variant?: 'default' | 'danger';
  handler: (selectedIds: string[], items: T[]) => Promise<void> | void;
  requiresConfirmation?: boolean;
  confirmMessage?: (count: number) => string;
}
```

---

## State Transitions

### Notification Read State

```
unread (is_read=false)
  → [user clicks "Отметить прочитанным" or "Отметить все"]
  → read (is_read=true)
```

### Export Job State Machine

```
pending
  → processing  (фоновая горутина начала обработку)
  → ready       (файл сгенерирован, ссылка доступна)
  → error       (ошибка генерации)
```

---

## Validation Rules (Inline Forms)

| Entity | Field | Rule |
|--------|-------|------|
| Template | name | required, minLength: 3, maxLength: 100 |
| Template | body | required, maxLength: 1600 (10 SMS сегментов) |
| SenderName | name | required, minLength: 3, maxLength: 11, pattern: `^[A-Za-z0-9 \-_.]+$` |
| Campaign | name | required, minLength: 3, maxLength: 200 |
| Webhook | url | required, pattern: HTTPS URL |
| APIKey | name | required, minLength: 1, maxLength: 100 |

**Character counter thresholds** (для SMS body):
- 0–140 символов: зелёный счётчик
- 141–160 символов: жёлтый (1 сегмент, приближение к лимиту)
- 161+ символов: красный (2+ сегментов, показывает кол-во сегментов)

