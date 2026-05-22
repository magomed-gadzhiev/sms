# Admin Gateway API

## Обзор

Admin Gateway предоставляет REST API для административных операций. Все запросы требуют аутентификации через JWT токен с ролью `admin`.

**Базовый URL:** `http://localhost:8081`

**Порты:**
- HTTP: 8081
- gRPC: 9091

## Аутентификация

Все запросы требуют JWT токена в заголовке:

```http
Authorization: Bearer <jwt-token>
```

Для получения токена используйте Auth Service:

```bash
POST /auth/v1/authenticate
{
  "username": "admin",
  "password": "password"
}
```

## Клиенты (Clients)

### Создать клиента

```http
POST /admin/v1/clients
Content-Type: application/json

{
  "name": "Client Name",
  "email": "client@example.com",
  "rate_limit_per_second": 100,
  "rate_limit_per_minute": 1000,
  "rate_limit_per_hour": 10000,
  "active": true
}
```

**Ответ:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Client Name",
  "email": "client@example.com",
  "api_key": "sk_live_...",
  "created_at": "2024-01-01T12:00:00Z"
}
```

### Получить список клиентов

```http
GET /admin/v1/clients?limit=100&offset=0&active=true
```

**Параметры запроса:**
- `limit` (опционально): количество записей (по умолчанию 100, максимум 1000)
- `offset` (опционально): смещение (по умолчанию 0)
- `active` (опционально): фильтр по активности (true/false)

**Ответ:**
```json
{
  "clients": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Client Name",
      "email": "client@example.com",
      "active": true,
      "created_at": "2024-01-01T12:00:00Z"
    }
  ],
  "total": 1,
  "limit": 100,
  "offset": 0
}
```

### Получить клиента

```http
GET /admin/v1/clients/:id
```

**Ответ:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Client Name",
  "email": "client@example.com",
  "rate_limit_per_second": 100,
  "rate_limit_per_minute": 1000,
  "rate_limit_per_hour": 10000,
  "active": true,
  "created_at": "2024-01-01T12:00:00Z",
  "updated_at": "2024-01-01T12:00:00Z"
}
```

### Обновить клиента

```http
PUT /admin/v1/clients/:id
Content-Type: application/json

{
  "name": "Updated Client Name",
  "rate_limit_per_second": 200,
  "active": false
}
```

### Удалить клиента

```http
DELETE /admin/v1/clients/:id
```

**Ответ:**
```json
{
  "success": true
}
```

## Провайдеры (Providers)

### Создать провайдера

```http
POST /admin/v1/providers
Content-Type: application/json

{
  "name": "Provider Name",
  "host": "smsc.provider.com",
  "port": 2775,
  "system_id": "username",
  "password": "password",
  "system_type": "",
  "interface_version": 52,
  "addr_ton": 0,
  "addr_npi": 0,
  "address_range": "",
  "priority": 1,
  "max_connections": 10,
  "active": true
}
```

**Ответ:**
```json
{
  "id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "Provider Name",
  "host": "smsc.provider.com",
  "port": 2775,
  "active": true,
  "created_at": "2024-01-01T12:00:00Z"
}
```

### Получить список провайдеров

```http
GET /admin/v1/providers?limit=100&offset=0&active=true
```

**Ответ:**
```json
{
  "providers": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440000",
      "name": "Provider Name",
      "host": "smsc.provider.com",
      "port": 2775,
      "active": true,
      "health_status": "healthy",
      "created_at": "2024-01-01T12:00:00Z"
    }
  ],
  "total": 1,
  "limit": 100,
  "offset": 0
}
```

### Получить провайдера

```http
GET /admin/v1/providers/:id
```

### Обновить провайдера

```http
PUT /admin/v1/providers/:id
Content-Type: application/json

{
  "name": "Updated Provider Name",
  "max_connections": 20,
  "active": false
}
```

### Удалить провайдера

```http
DELETE /admin/v1/providers/:id
```

### Получить здоровье провайдера

```http
GET /admin/v1/providers/:id/health
```

**Ответ:**
```json
{
  "provider_id": "660e8400-e29b-41d4-a716-446655440000",
  "status": "healthy",
  "active_connections": 5,
  "max_connections": 10,
  "messages_sent_today": 10000,
  "messages_failed_today": 5,
  "last_error": null,
  "last_checked": "2024-01-01T12:00:00Z"
}
```

## Маршруты (Routes)

### Создать маршрут

```http
POST /admin/v1/routes
Content-Type: application/json

{
  "name": "Route Name",
  "priority": 1,
  "rules": [
    {
      "prefix": "+7",
      "provider_ids": ["660e8400-e29b-41d4-a716-446655440000"],
      "load_balance_strategy": "round_robin"
    }
  ],
  "active": true
}
```

**Ответ:**
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440000",
  "name": "Route Name",
  "priority": 1,
  "active": true,
  "created_at": "2024-01-01T12:00:00Z"
}
```

### Получить список маршрутов

```http
GET /admin/v1/routes?limit=100&offset=0&active=true
```

### Получить маршрут

```http
GET /admin/v1/routes/:id
```

### Обновить маршрут

```http
PUT /admin/v1/routes/:id
Content-Type: application/json

{
  "priority": 2,
  "active": false
}
```

### Удалить маршрут

```http
DELETE /admin/v1/routes/:id
```

## Аналитика (Analytics)

### Получить статистику

```http
GET /admin/v1/analytics/stats?from=2024-01-01T00:00:00Z&to=2024-01-31T23:59:59Z&client_id=...
```

**Параметры запроса:**
- `from` (обязательно): начало периода
- `to` (обязательно): конец периода
- `client_id` (опционально): фильтр по клиенту
- `provider_id` (опционально): фильтр по провайдеру

**Ответ:**
```json
{
  "period": {
    "from": "2024-01-01T00:00:00Z",
    "to": "2024-01-31T23:59:59Z"
  },
  "messages": {
    "total": 1000000,
    "sent": 990000,
    "delivered": 980000,
    "failed": 10000,
    "pending": 0
  },
  "delivery_rate": 98.99,
  "average_delivery_time": 5.2
}
```

### Генерация отчета

```http
POST /admin/v1/analytics/reports
Content-Type: application/json

{
  "report_type": "daily",
  "from": "2024-01-01T00:00:00Z",
  "to": "2024-01-31T23:59:59Z",
  "client_id": "550e8400-e29b-41d4-a716-446655440000",
  "format": "json"
}
```

**Типы отчетов:**
- `daily` - ежедневный отчет
- `weekly` - еженедельный отчет
- `monthly` - месячный отчет
- `custom` - пользовательский отчет

**Форматы:**
- `json` - JSON формат
- `csv` - CSV формат
- `pdf` - PDF формат (будущая функциональность)

### Получить метрики в реальном времени

```http
GET /admin/v1/analytics/metrics/realtime
```

**Ответ:**
```json
{
  "messages_per_second": 100,
  "active_connections": 50,
  "queue_size": 1000,
  "providers": [
    {
      "provider_id": "660e8400-e29b-41d4-a716-446655440000",
      "status": "healthy",
      "throughput": 50
    }
  ]
}
```

## Биллинг (Billing)

### Получить транзакции

```http
GET /admin/v1/billing/transactions?client_id=...&limit=100&offset=0
```

**Параметры запроса:**
- `client_id` (опционально): фильтр по клиенту
- `type` (опционально): тип транзакции (charge, credit, refund)
- `from` (опционально): начало периода
- `to` (опционально): конец периода
- `limit` (опционально): количество записей
- `offset` (опционально): смещение

**Ответ:**
```json
{
  "transactions": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440000",
      "client_id": "550e8400-e29b-41d4-a716-446655440000",
      "type": "charge",
      "amount": "0.05",
      "currency": "RUB",
      "message_id": "990e8400-e29b-41d4-a716-446655440000",
      "created_at": "2024-01-01T12:00:00Z"
    }
  ],
  "total": 1,
  "limit": 100,
  "offset": 0
}
```

### Получить счета

```http
GET /admin/v1/billing/accounts?client_id=...
```

**Ответ:**
```json
{
  "accounts": [
    {
      "id": "aa0e8400-e29b-41d4-a716-446655440000",
      "client_id": "550e8400-e29b-41d4-a716-446655440000",
      "balance": "1000.00",
      "currency": "RUB",
      "updated_at": "2024-01-01T12:00:00Z"
    }
  ]
}
```

### Пополнить счет

```http
POST /admin/v1/billing/accounts/:id/credits
Content-Type: application/json

{
  "amount": "100.00",
  "currency": "RUB",
  "description": "Manual credit"
}
```

**Ответ:**
```json
{
  "transaction_id": "bb0e8400-e29b-41d4-a716-446655440000",
  "new_balance": "1100.00",
  "created_at": "2024-01-01T12:00:00Z"
}
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

- `401 Unauthorized` - неверный или отсутствующий токен
- `403 Forbidden` - недостаточно прав
- `404 Not Found` - ресурс не найден
- `400 Bad Request` - неверный запрос
- `429 Too Many Requests` - превышен лимит запросов
- `500 Internal Server Error` - внутренняя ошибка сервера

### Пример ошибки

```json
{
  "error": "not_found",
  "message": "Client with id 550e8400-e29b-41d4-a716-446655440000 not found"
}
```

## Rate Limiting

Admin Gateway использует rate limiting на уровне токена:

- **По умолчанию:** 1000 запросов в минуту
- При превышении возвращается HTTP 429

**Заголовок ответа:**
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1640995200
Retry-After: 60
```

## Дополнительная документация

- [Gateway слой](../architecture/gateway-layer.md)
- [gRPC сервисы](grpc-services.md)