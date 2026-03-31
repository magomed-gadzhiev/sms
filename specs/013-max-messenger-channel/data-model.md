# Data Model: Max Messenger Channel

**Feature**: 013-max-messenger-channel  
**Date**: 2026-04-01

## Изменения в существующих сущностях

### 1. ChannelType (domain enum)

**Файл**: `internal/services/cascade/domain/channel.go`

Добавить новую константу:

```go
const (
    ChannelSMS           ChannelType = "sms"
    ChannelFlashCall     ChannelType = "flash_call"
    ChannelReverseCall   ChannelType = "reverse_call"
    ChannelMessenger     ChannelType = "messenger"
    ChannelMaxMessenger  ChannelType = "max_messenger"  // NEW
)
```

Добавить в `validChannelTypes`:
```go
ChannelMaxMessenger: true,
```

### 2. delivery_channels (seed data)

Новая запись при миграции:

| Поле | Значение |
|------|----------|
| channel_type | `max_messenger` |
| name | `Max Messenger` |
| description | `Мессенджер Max — доставка сообщений через Bot API` |
| config | `{"provider_url": "", "api_key": "", "bot_id": "", "webhook_secret": "", "check_registration_url": ""}` |
| active | `false` (включается администратором после настройки) |

### 3. operator_channel_support (seed data)

Max Messenger не зависит от мобильного оператора (работает через интернет), поэтому seed `supported=true` для всех операторов:

```sql
INSERT INTO operator_channel_support (operator_id, channel_type, supported)
SELECT id, 'max_messenger', true FROM operators
ON CONFLICT (operator_id, channel_type) DO NOTHING;
```

## Новые сущности

### Нет новых таблиц

Max Messenger полностью интегрируется в существующую схему каскадной доставки:
- `delivery_channels` — запись с `channel_type = 'max_messenger'`
- `delivery_strategy_steps` — шаги стратегии с channel_id канала Max
- `delivery_attempts` — попытки с `channel_type = 'max_messenger'`
- `operator_channel_support` — записи поддержки для всех операторов

## Новые Go-структуры

### MaxMessengerAdapter

**Пакет**: `internal/services/cascade/channels/max_messenger`

```go
type Adapter struct {
    httpClient *http.Client
    logger     zerolog.Logger
    metrics    *MaxMessengerMetrics
}

// Реализует domain.Channel interface
func (a *Adapter) Type() domain.ChannelType    // → ChannelMaxMessenger
func (a *Adapter) Send(ctx, attempt, cfg) error // → HTTP POST к Max Bot API
```

### MaxMessengerMetrics (channel-specific)

```go
type MaxMessengerMetrics struct {
    WebhooksReceived    *prometheus.CounterVec   // {status="valid"|"invalid_signature"|"unknown_attempt"}
    ReachabilityChecks  *prometheus.CounterVec   // {result="registered"|"not_registered"|"error"|"cache_hit"}
    SendLatency         *prometheus.HistogramVec // {status="success"|"rate_limited"|"error"}
    ActiveAttempts      prometheus.Gauge
}
```

### MaxMessengerWebhookPayload (incoming)

```go
type WebhookPayload struct {
    AttemptID  string `json:"attempt_id"`
    MessageID  string `json:"message_id"`   // Max internal message ID
    Status     string `json:"status"`       // "delivered" | "read" | "error"
    Error      string `json:"error,omitempty"`
    Timestamp  string `json:"timestamp"`
}
```

### MaxMessengerSendRequest (outgoing)

```go
type SendRequest struct {
    RecipientMSISDN string          `json:"recipient"`
    Text            string          `json:"text,omitempty"`
    ImageURL        string          `json:"image_url,omitempty"`
    InlineKeyboard  [][]ButtonItem  `json:"inline_keyboard,omitempty"`
    CallbackURL     string          `json:"callback_url"`    // webhook URL for status
    ExternalID      string          `json:"external_id"`     // attempt_id
}

type ButtonItem struct {
    Text string `json:"text"`
    URL  string `json:"url,omitempty"`
    Data string `json:"callback_data,omitempty"`
}
```

### MaxMessengerCheckRequest (reachability)

```go
type CheckRegistrationRequest struct {
    MSISDN string `json:"msisdn"`
}

type CheckRegistrationResponse struct {
    Registered bool   `json:"registered"`
    UserID     string `json:"user_id,omitempty"`
}
```

## Валидация

### ChannelConfig для Max Messenger

При создании/обновлении канала через admin API:

| Поле | Обязательное | Валидация |
|------|-------------|-----------|
| provider_url | Да | Валидный URL, HTTPS |
| api_key | Да | Непустая строка |
| bot_id | Да | Непустая строка |
| webhook_secret | Да | Мин. 32 символа |
| check_registration_url | Да | Валидный URL, HTTPS |

## State Transitions

Используются существующие transition-ы `DeliveryAttempt`:

```
pending  ──→ sent              (Send() вернул nil, HTTP 200 от Max API)
         ──→ failed            (Send() вернул ошибку, включая retry exhaustion)
         ──→ skipped           (CheckReachability → not registered)

sent     ──→ delivered         (webhook: status=delivered)
         ──→ failed            (webhook: status=error)
         ──→ timeout           (scheduler: step timeout exceeded)
         ──→ late_duplicate    (webhook после завершения delivery)
```

## Кеширование (Redis)

### Reachability cache

```
Key:    reachability:{msisdn}:max_messenger
Value:  "1" (registered) | "0" (not registered)
TTL:    3600 секунд (1 час, FR-004)
```

Интегрируется в существующий `ReachabilityService`. Для Max Messenger добавляется специфичная проверка: вызов `check_registration_url` из конфига канала, если оператор поддерживает канал.

## Миграции

### 000071_max_messenger_channel.up.sql

```sql
-- Seed Max Messenger channel (inactive by default)
INSERT INTO delivery_channels (channel_type, name, description, config, active)
VALUES (
    'max_messenger',
    'Max Messenger',
    'Мессенджер Max — доставка сообщений через Bot API',
    '{}',
    false
) ON CONFLICT (channel_type) DO NOTHING;

-- Enable Max Messenger support for all operators (internet-based, operator-independent)
INSERT INTO operator_channel_support (operator_id, channel_type, supported)
SELECT id, 'max_messenger', true FROM operators
ON CONFLICT (operator_id, channel_type) DO NOTHING;
```

### 000071_max_messenger_channel.down.sql

```sql
DELETE FROM operator_channel_support WHERE channel_type = 'max_messenger';
DELETE FROM delivery_channels WHERE channel_type = 'max_messenger';
```
