# Data Model: Live Load Test Dashboard

**Feature**: 007-grafana-live-dashboard | **Date**: 2026-03-26

## Обзор

Фича не создаёт новых таблиц или сущностей. Дашборд потребляет данные из двух источников:
1. **Prometheus** — time-series метрики (счётчики, гистограммы, gauges)
2. **PostgreSQL** — бизнес-данные (балансы, транзакции, сообщения)

## Используемые сущности PostgreSQL

### accounts (Billing)

| Поле | Тип | Описание | Панель дашборда |
|------|-----|----------|-----------------|
| `client_id` | UUID | FK на clients | Фильтр |
| `balance` | NUMERIC(20,6) | Текущий баланс | Balance (stat) |
| `currency` | VARCHAR(3) | Валюта | Balance (suffix) |

**Запрос**: `SELECT balance, currency FROM accounts WHERE client_id = $client_id`

---

### transactions (Billing)

| Поле | Тип | Описание | Панель дашборда |
|------|-----|----------|-----------------|
| `client_id` | UUID | FK на clients | Фильтр |
| `type` | VARCHAR(20) | charge/credit/refund | Фильтр (type='charge') |
| `amount` | NUMERIC(20,6) | Сумма транзакции | Accumulated Cost |
| `balance_after` | NUMERIC(20,6) | Баланс после | Balance Over Time graph |
| `created_at` | TIMESTAMPTZ | Время транзакции | Time axis |

**Запрос для накопленной стоимости**:
```sql
SELECT SUM(amount) FROM transactions
WHERE client_id = '$client_id' AND type = 'charge'
  AND created_at >= NOW() - INTERVAL '${session_duration}'
```

**Запрос для графика баланса**:
```sql
SELECT created_at AS time, balance_after AS balance
FROM transactions
WHERE client_id = '$client_id'
  AND created_at >= NOW() - INTERVAL '${time_range}'
ORDER BY created_at
```

---

### messages (Messaging, partitioned)

| Поле | Тип | Описание | Панель дашборда |
|------|-----|----------|-----------------|
| `id` | UUID | PK | — |
| `destination` | VARCHAR(20) | Номер получателя | Feed (маскированный) |
| `status` | VARCHAR(50) | pending/queued/sent/delivered/failed/expired/rejected | Counters, Delivery Rate |
| `provider_id` | UUID | FK на providers | Provider breakdown |
| `client_id` | UUID | FK на clients | Фильтр |
| `created_at` | TIMESTAMPTZ | Время создания | Time axis, Feed |
| `submitted_at` | TIMESTAMPTZ | Время отправки | Latency calc |
| `delivered_at` | TIMESTAMPTZ | Время доставки | Latency calc |

**Запрос для счётчиков по статусам**:
```sql
SELECT status, COUNT(*) as count
FROM messages
WHERE client_id = '$client_id'
  AND created_at >= NOW() - INTERVAL '${session_duration}'
GROUP BY status
```

**Запрос для delivery rate**:
```sql
SELECT
  ROUND(
    COUNT(*) FILTER (WHERE status = 'delivered')::numeric /
    NULLIF(COUNT(*) FILTER (WHERE status IN ('delivered', 'failed')), 0) * 100, 2
  ) as delivery_rate
FROM messages
WHERE client_id = '$client_id'
  AND created_at >= NOW() - INTERVAL '${session_duration}'
```

**Запрос для ленты сообщений**:
```sql
SELECT
  created_at AS "Time",
  CONCAT(LEFT(destination, 3), '***', RIGHT(destination, 4)) AS "Destination",
  status AS "Status",
  p.name AS "Provider",
  EXTRACT(EPOCH FROM (COALESCE(delivered_at, updated_at) - created_at))::int AS "Latency (s)"
FROM messages m
LEFT JOIN providers p ON m.provider_id = p.id
WHERE m.client_id = '$client_id'
ORDER BY m.created_at DESC
LIMIT 20
```

---

### tarification_log (Tarification, partitioned)

| Поле | Тип | Описание | Панель дашборда |
|------|-----|----------|-----------------|
| `client_id` | UUID | FK на clients | Фильтр |
| `total_amount` | NUMERIC(20,6) | Стоимость | Accumulated Cost |
| `segment_count` | INTEGER | Количество сегментов | — |
| `price_per_segment` | NUMERIC(20,6) | Цена за сегмент | — |
| `created_at` | TIMESTAMPTZ | Время | Time axis |

**Запрос для накопленной стоимости** (альтернатива через tarification_log):
```sql
SELECT SUM(total_amount) as total_cost
FROM tarification_log
WHERE client_id = '$client_id'
  AND created_at >= NOW() - INTERVAL '${session_duration}'
```

---

### aggregated_metrics (Analytics)

| Поле | Тип | Описание | Панель дашборда |
|------|-----|----------|-----------------|
| `period` | VARCHAR(20) | minute/hour/day | Фильтр |
| `period_start` | TIMESTAMPTZ | Начало периода | Time axis |
| `client_id` | UUID | FK | Фильтр |
| `provider_id` | UUID | FK | Provider breakdown |
| `status` | VARCHAR(50) | Статус сообщения | Counters |
| `count` | BIGINT | Количество | All metric panels |

**Запрос для графика delivery rate по времени**:
```sql
SELECT
  period_start AS time,
  SUM(count) FILTER (WHERE status = 'delivered') AS delivered,
  SUM(count) FILTER (WHERE status = 'failed') AS failed,
  ROUND(
    SUM(count) FILTER (WHERE status = 'delivered')::numeric /
    NULLIF(SUM(count) FILTER (WHERE status IN ('delivered', 'failed')), 0) * 100, 2
  ) AS delivery_rate
FROM aggregated_metrics
WHERE period = 'minute'
  AND period_start >= NOW() - INTERVAL '${time_range}'
GROUP BY period_start
ORDER BY period_start
```

---

### providers (Provider)

| Поле | Тип | Описание | Панель дашборда |
|------|-----|----------|-----------------|
| `id` | UUID | PK | Join key |
| `name` | VARCHAR | Имя провайдера | Provider labels |
| `active` | BOOLEAN | Активен ли | Provider status |

Используется только для JOIN с messages и для отображения имён провайдеров.

---

## Prometheus метрики (используемые)

### Counters (монотонно растущие)

| Метрика | Labels | Панель |
|---------|--------|--------|
| `smpp_messages_sent_total` | provider_id, provider_name, status | msg/s graph, provider breakdown |
| `smpp_messages_failed_total` | provider_id, provider_name, reason | Failed counter, provider errors |
| `sms_messages_queued_total` | client_id | Queued counter |
| `sms_messages_received_total` | source, client_id | Received counter |

### Histograms

| Метрика | Labels | Панель |
|---------|--------|--------|
| `smpp_processing_duration_seconds` | operation | Latency graph (p50, p95, p99) |

### Gauges

| Метрика | Labels | Панель |
|---------|--------|--------|
| `smpp_provider_throughput` | provider_id, provider_name | Provider throughput |
| `smpp_connections_active` | type | Connection status |
| `smpp_queue_size` | topic, consumer_group | Queue depth |

---

## Переменные дашборда (Grafana Template Variables)

| Переменная | Тип | Источник | Значение по умолчанию |
|------------|-----|----------|----------------------|
| `$client_id` | Query | PostgreSQL: `SELECT id AS __value, name AS __text FROM clients` | Первый клиент |
| `$time_range` | Interval | Custom: 5m, 15m, 30m, 1h | 30m |

---

## Связи между сущностями

```
accounts.client_id ──→ clients.id
transactions.client_id ──→ clients.id
transactions.message_id ──→ messages.id
messages.client_id ──→ clients.id
messages.provider_id ──→ providers.id
tarification_log.client_id ──→ clients.id
tarification_log.message_id ──→ messages.id
aggregated_metrics.client_id ──→ clients.id
aggregated_metrics.provider_id ──→ providers.id
```
