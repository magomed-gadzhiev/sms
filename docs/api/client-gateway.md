# Client Gateway API

## Обзор

Client Gateway предоставляет REST API и gRPC API для клиентских операций. Используется клиентами для отправки SMS, получения статусов сообщений и просмотра статистики.

**Базовый URL:** `http://localhost:8080`

**Порты:**
- HTTP: 8080
- gRPC: 9090

## Аутентификация

Все запросы требуют API ключа в заголовке:

```http
X-API-Key: <api-key>
```

Или через Bearer token:

```http
Authorization: Bearer <api-key>
```

API ключ можно получить через Admin Gateway после создания клиента.

## Отправка SMS

### Отправить одно SMS сообщение

```http
POST /api/v1/sms/send
Content-Type: application/json
X-API-Key: <api-key>

{
  "source": "12345",
  "destination": "79001234567",
  "text": "Текст сообщения",
  "external_id": "optional-external-id",
  "priority": 0,
  "registered_delivery": true,
  "validity_period": "2024-12-31T23:59:59Z",
  "service_type": "",
  "source_addr_ton": 0,
  "source_addr_npi": 0,
  "dest_addr_ton": 0,
  "dest_addr_npi": 0,
  "data_coding": 0
}
```

**Параметры:**
- `source` (обязательно): номер отправителя (SMS sender)
- `destination` (обязательно): номер получателя
- `text` (обязательно): текст сообщения (максимум 1600 символов для одного SMS, автоматически разбивается на части)
- `external_id` (опционально): внешний ID для отслеживания (максимум 64 символа)
- `priority` (опционально): приоритет (0-3, по умолчанию 0)
- `registered_delivery` (опционально): требовать отчет о доставке (по умолчанию false)
- `validity_period` (опционально): срок действия сообщения (ISO 8601)
- `service_type` (опционально): тип сервиса
- `source_addr_ton` (опционально): Type of Number отправителя (0-6)
- `source_addr_npi` (опционально): Numbering Plan Indicator отправителя (0-18)
- `dest_addr_ton` (опционально): Type of Number получателя
- `dest_addr_npi` (опционально): Numbering Plan Indicator получателя
- `data_coding` (опционально): кодировка данных (0=default, 8=UTF-16)

**Ответ:**
```json
{
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued",
  "created_at": "2024-01-01T12:00:00Z"
}
```

**Статусы:**
- `queued` - сообщение поставлено в очередь
- `sent` - сообщение отправлено в SMSC
- `failed` - ошибка при отправке

### Пакетная отправка SMS

```http
POST /api/v1/sms/batch
Content-Type: application/json
X-API-Key: <api-key>

{
  "messages": [
    {
      "source": "12345",
      "destination": "79001234567",
      "text": "Сообщение 1",
      "external_id": "msg-1"
    },
    {
      "source": "12345",
      "destination": "79001234568",
      "text": "Сообщение 2",
      "external_id": "msg-2"
    }
  ]
}
```

**Ограничения:**
- Максимум 1000 сообщений в одном запросе
- Каждое сообщение валидируется отдельно
- Частичная успешность допустима

**Ответ:**
```json
{
  "results": [
    {
      "message_id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "queued",
      "external_id": "msg-1"
    },
    {
      "message_id": "550e8400-e29b-41d4-a716-446655440001",
      "status": "queued",
      "external_id": "msg-2"
    }
  ],
  "success_count": 2,
  "failed_count": 0
}
```

## Статусы сообщений

### Получить статус сообщения

```http
GET /api/v1/sms/status?id=550e8400-e29b-41d4-a716-446655440000
X-API-Key: <api-key>
```

**Параметры запроса:**
- `id` (обязательно): ID сообщения

**Ответ:**
```json
{
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "delivered",
  "status_message": "",
  "created_at": "2024-01-01T12:00:00Z",
  "submitted_at": "2024-01-01T12:00:01Z",
  "delivered_at": "2024-01-01T12:00:05Z",
  "smpp_message_id": "smpp-12345",
  "error_code": null,
  "error_message": null
}
```

**Статусы:**
- `pending` - ожидание обработки
- `queued` - в очереди
- `sent` - отправлено в SMSC
- `delivered` - доставлено получателю
- `failed` - ошибка доставки
- `expired` - истек срок действия
- `rejected` - отклонено SMSC

### Получить историю сообщений

```http
GET /api/v1/sms/history?limit=100&offset=0&status=sent&from=2024-01-01T00:00:00Z&to=2024-01-31T23:59:59Z
X-API-Key: <api-key>
```

**Параметры запроса:**
- `limit` (опционально): количество сообщений (по умолчанию 100, максимум 1000)
- `offset` (опционально): смещение для пагинации (по умолчанию 0)
- `status` (опционально): фильтр по статусу
- `from` (опционально): начало периода (ISO 8601)
- `to` (опционально): конец периода (ISO 8601)
- `destination` (опционально): фильтр по номеру получателя

**Ответ:**
```json
{
  "messages": [
    {
      "message_id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "delivered",
      "source": "12345",
      "destination": "79001234567",
      "text": "Текст сообщения",
      "created_at": "2024-01-01T12:00:00Z",
      "delivered_at": "2024-01-01T12:00:05Z"
    }
  ],
  "total": 1,
  "limit": 100,
  "offset": 0
}
```

## Аккаунт

### Получить баланс

```http
GET /api/v1/account/balance
X-API-Key: <api-key>
```

**Ответ:**
```json
{
  "balance": "1000.50",
  "currency": "USD",
  "updated_at": "2024-01-01T12:00:00Z"
}
```

### Получить статистику

```http
GET /api/v1/account/stats?from=2024-01-01T00:00:00Z&to=2024-01-31T23:59:59Z
X-API-Key: <api-key>
```

**Параметры запроса:**
- `from` (опционально): начало периода
- `to` (опционально): конец периода

**Ответ:**
```json
{
  "period": {
    "from": "2024-01-01T00:00:00Z",
    "to": "2024-01-31T23:59:59Z"
  },
  "messages": {
    "total": 100000,
    "sent": 99000,
    "delivered": 98000,
    "failed": 1000
  },
  "delivery_rate": 98.99,
  "average_delivery_time": 5.2,
  "spent": "5000.00",
  "currency": "USD"
}
```

## Rate Limiting

Rate limiting настраивается для каждого клиента индивидуально и применяется на уровне API Gateway.

**Лимиты (пример):**
- `rate_limit_per_second`: 100 запросов в секунду
- `rate_limit_per_minute`: 1000 запросов в минуту
- `rate_limit_per_hour`: 10000 запросов в час

При превышении лимита возвращается HTTP 429:

```json
{
  "error": "rate_limit_exceeded",
  "message": "Rate limit exceeded. Please try again later.",
  "retry_after": 60
}
```

**Заголовки ответа:**
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1640995200
Retry-After: 60
```

## Обработка ошибок

Все ошибки возвращаются в формате:

```json
{
  "error": "error_code",
  "message": "Human readable error message",
  "details": {}
}
```

### Коды ошибок

- `401 Unauthorized` - неверный или отсутствующий API ключ
- `400 Bad Request` - неверный запрос (валидация)
- `404 Not Found` - ресурс не найден
- `429 Too Many Requests` - превышен лимит запросов
- `500 Internal Server Error` - внутренняя ошибка сервера

### Примеры ошибок

**Неверный API ключ:**
```json
{
  "error": "unauthorized",
  "message": "Invalid API key"
}
```

**Неверный формат номера:**
```json
{
  "error": "validation_error",
  "message": "Invalid destination format",
  "details": {
    "field": "destination",
    "value": "invalid-number"
  }
}
```

**Превышен лимит:**
```json
{
  "error": "rate_limit_exceeded",
  "message": "Rate limit exceeded. Please try again later.",
  "retry_after": 60
}
```

**Сообщение не найдено:**
```json
{
  "error": "not_found",
  "message": "Message with id 550e8400-e29b-41d4-a716-446655440000 not found"
}
```

## Валидация данных

### Номер получателя/отправителя

- Минимум 1 символ, максимум 21 символ
- Может содержать цифры, +, -, пробелы
- Рекомендуется международный формат (начинается с +)

### Текст сообщения

- Не может быть пустым
- Максимум 1600 символов для одного SMS
- При превышении автоматически разбивается на несколько SMS (UCS-2 кодировка)
- Поддерживаются Unicode символы

### External ID

- Максимум 64 символа
- Должен быть уникальным для клиента (опционально)

## Webhooks (будущая функциональность)

В будущем будет доступна возможность настройки webhooks для получения уведомлений о статусах сообщений:

```
POST <webhook-url>
Content-Type: application/json

{
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "delivered",
  "timestamp": "2024-01-01T12:00:05Z"
}
```

## Примеры использования

### cURL

```bash
# Отправить SMS
curl -X POST http://localhost:8080/api/v1/sms/send \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "source": "12345",
    "destination": "79001234567",
    "text": "Test message"
  }'

# Получить статус
curl -X GET "http://localhost:8080/api/v1/sms/status?id=550e8400-e29b-41d4-a716-446655440000" \
  -H "X-API-Key: your-api-key"
```

### Python

```python
import requests

API_KEY = "your-api-key"
BASE_URL = "http://localhost:8080/api/v1"

headers = {
    "Content-Type": "application/json",
    "X-API-Key": API_KEY
}

# Отправить SMS
response = requests.post(
    f"{BASE_URL}/sms/send",
    headers=headers,
    json={
        "source": "12345",
        "destination": "79001234567",
        "text": "Test message"
    }
)
print(response.json())

# Получить статус
message_id = response.json()["message_id"]
response = requests.get(
    f"{BASE_URL}/sms/status",
    headers=headers,
    params={"id": message_id}
)
print(response.json())
```

### JavaScript (Node.js)

```javascript
const axios = require('axios');

const API_KEY = 'your-api-key';
const BASE_URL = 'http://localhost:8080/api/v1';

const headers = {
  'Content-Type': 'application/json',
  'X-API-Key': API_KEY
};

// Отправить SMS
axios.post(`${BASE_URL}/sms/send`, {
  source: '12345',
  destination: '79001234567',
  text: 'Test message'
}, { headers })
  .then(response => {
    console.log(response.data);
    return response.data.message_id;
  })
  .then(messageId => {
    // Получить статус
    return axios.get(`${BASE_URL}/sms/status`, {
      headers,
      params: { id: messageId }
    });
  })
  .then(response => {
    console.log(response.data);
  });
```

## Дополнительная документация

- [Gateway слой](../architecture/gateway-layer.md)
- [gRPC API](grpc.md)
- [gRPC сервисы](grpc-services.md)