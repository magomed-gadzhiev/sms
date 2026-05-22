# Gateway Слой

## Обзор

Gateway слой предоставляет единые точки входа для различных типов клиентов. Система разделена на три gateway:

1. **Admin Gateway** - для административных операций
2. **Client Gateway** - для клиентских операций
3. **SMPP Gateway** - для SMPP протокола

## Архитектура Gateway

```
┌─────────────┐
│   Clients   │
└──────┬──────┘
       │
       ├──────────────────┬──────────────────┐
       │                  │                  │
┌──────▼──────┐   ┌──────▼──────┐   ┌──────▼──────┐
│ Admin UI    │   │ Client UI   │   │ SMPP Client │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       │                  │                  │
       │                  │                  │
┌──────▼──────────────────▼──────────────────▼──────┐
│                   HAProxy                         │
│          (Load Balancer & Router)                 │
└──────┬──────────────────┬──────────────────┬──────┘
       │                  │                  │
┌──────▼──────┐   ┌──────▼──────┐   ┌──────▼──────┐
│ Admin       │   │ Client      │   │ SMPP        │
│ Gateway     │   │ Gateway     │   │ Gateway     │
│             │   │             │   │             │
│ HTTP: 8081  │   │ HTTP: 8080  │   │ SMPP: 2775  │
│ gRPC: 9091  │   │ gRPC: 9090  │   │             │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       │                  │                  │
       └──────────────────┴──────────────────┘
                          │
           ┌──────────────┼──────────────┐
           │              │              │
      ┌────▼────┐   ┌────▼────┐   ┌────▼────┐
      │ Auth    │   │ Messaging│   │ Routing │
      │ Service │   │ Service  │   │ Service │
      └─────────┘   └──────────┘   └─────────┘
```

## Admin Gateway

### Назначение

API для административных операций. Используется администраторами системы для управления клиентами, провайдерами, маршрутами, просмотра аналитики и управления биллингом.

### Порты

- **HTTP:** 8081
- **gRPC:** 9091
- **Metrics:** 2112

### Функции

1. **Управление клиентами (CRUD)**
   - Создание, обновление, удаление клиентов
   - Просмотр списка клиентов
   - Управление API ключами

2. **Управление провайдерами (CRUD)**
   - Добавление SMSC провайдеров
   - Обновление конфигурации провайдеров
   - Мониторинг здоровья провайдеров

3. **Управление маршрутами (CRUD)**
   - Создание правил маршрутизации
   - Настройка приоритетов маршрутов
   - Управление failover

4. **Аналитика и отчеты**
   - Просмотр статистики по сообщениям
   - Генерация отчетов
   - Мониторинг производительности

5. **Биллинг**
   - Просмотр транзакций
   - Управление балансами
   - Настройка тарифов

### HTTP Endpoints

```
# Клиенты
POST   /admin/v1/clients              # Создать клиента
GET    /admin/v1/clients              # Список клиентов
GET    /admin/v1/clients/:id          # Получить клиента
PUT    /admin/v1/clients/:id          # Обновить клиента
DELETE /admin/v1/clients/:id          # Удалить клиента

# Провайдеры
POST   /admin/v1/providers            # Создать провайдера
GET    /admin/v1/providers            # Список провайдеров
GET    /admin/v1/providers/:id        # Получить провайдера
PUT    /admin/v1/providers/:id        # Обновить провайдера
DELETE /admin/v1/providers/:id        # Удалить провайдера
GET    /admin/v1/providers/:id/health # Здоровье провайдера

# Маршруты
POST   /admin/v1/routes               # Создать маршрут
GET    /admin/v1/routes               # Список маршрутов
GET    /admin/v1/routes/:id           # Получить маршрут
PUT    /admin/v1/routes/:id           # Обновить маршрут
DELETE /admin/v1/routes/:id           # Удалить маршрут

# Аналитика
GET    /admin/v1/analytics/stats      # Статистика
GET    /admin/v1/analytics/reports    # Отчеты
GET    /admin/v1/analytics/metrics    # Метрики

# Биллинг
GET    /admin/v1/billing/transactions # Транзакции
GET    /admin/v1/billing/accounts     # Счета
POST   /admin/v1/billing/accounts/:id/credits # Пополнить счет
```

### Аутентификация

Admin Gateway использует JWT токены с ролью `admin`:

```http
Authorization: Bearer <jwt-token>
```

### Middleware

1. **Authentication** - проверка JWT токена
2. **Authorization** - проверка роли admin
3. **Logging** - структурированное логирование
4. **Recovery** - обработка паник
5. **RequestID** - уникальный ID для каждого запроса

## Client Gateway

### Назначение

API для клиентских операций. Используется клиентами системы для отправки SMS, получения статусов сообщений, просмотра истории и статистики.

### Порты

- **HTTP:** 8080
- **gRPC:** 9090
- **Metrics:** 2112

### Функции

1. **Отправка SMS**
   - Отправка одного сообщения
   - Пакетная отправка
   - Валидация сообщений

2. **Статусы сообщений**
   - Получение статуса по ID
   - Просмотр истории сообщений

3. **Аккаунт**
   - Просмотр баланса
   - Личная статистика

### HTTP Endpoints

```
# Отправка SMS
POST   /api/v1/sms/send               # Отправить SMS
POST   /api/v1/sms/batch              # Пакетная отправка

# Статусы
GET    /api/v1/sms/status             # Статус сообщения
GET    /api/v1/sms/history            # История сообщений

# Аккаунт
GET    /api/v1/account/balance        # Баланс
GET    /api/v1/account/stats          # Статистика
```

### Аутентификация

Client Gateway использует API ключи:

```http
X-API-Key: <api-key>
```

Или через Bearer token:

```http
Authorization: Bearer <api-key>
```

### Middleware

1. **Authentication** - проверка API ключа
2. **Rate Limiting** - ограничение частоты запросов (настраивается per client)
3. **Logging** - структурированное логирование
4. **Recovery** - обработка паник
5. **RequestID** - уникальный ID для каждого запроса

### Rate Limiting

Rate limiting настраивается для каждого клиента:

```yaml
rate_limit:
  per_second: 100
  per_minute: 1000
  per_hour: 10000
```

При превышении лимита возвращается HTTP 429:

```json
{
  "error": "rate_limit_exceeded",
  "message": "Rate limit exceeded. Please try again later.",
  "retry_after": 60
}
```

## SMPP Gateway

### Назначение

Прием входящих SMPP соединений от клиентов. Обрабатывает SMPP протокол версии 3.4.

### Порты

- **SMPP:** 2775
- **Metrics:** 2112

### Функции

1. **SMPP соединения**
   - Прием bind_receiver, bind_transmitter, bind_transceiver
   - Управление сессиями
   - Enquire link для keep-alive

2. **Обработка сообщений**
   - Прием submit_sm
   - Отправка submit_sm_resp
   - Публикация в Kafka

3. **Delivery Receipts**
   - Отправка deliver_sm (DLR) клиентам
   - Обработка deliver_sm_resp

4. **Query операций**
   - query_sm для получения статуса сообщения

### Поддерживаемые команды

```
Bind Operations:
- bind_receiver / bind_receiver_resp
- bind_transmitter / bind_transmitter_resp
- bind_transceiver / bind_transceiver_resp

Message Operations:
- submit_sm / submit_sm_resp
- deliver_sm / deliver_sm_resp
- query_sm / query_sm_resp

Session Management:
- unbind / unbind_resp
- enquire_link / enquire_link_resp
```

### Аутентификация

SMPP Gateway использует system_id и password из bind запроса:

```
system_id: "client1"
password:  "secret123"
```

### Session Management

```go
type Session struct {
    SystemID   string
    BindType   BindType // Receiver, Transmitter, Transceiver
    Connection net.Conn
    LastActivity time.Time
    RateLimiter *rate.Limiter
}

func (s *Session) HandleSubmitSM(pdu *SubmitSMPDU) (*SubmitSMRespPDU, error) {
    // Проверка rate limit
    if !s.RateLimiter.Allow() {
        return &SubmitSMRespPDU{
            CommandStatus: ESME_RSUBMITFAIL,
        }, nil
    }
    
    // Создание сообщения
    msg := &Message{
        Source:      pdu.SourceAddr,
        Destination: pdu.DestinationAddr,
        Text:        string(pdu.ShortMessage),
    }
    
    // Сохранение в БД
    // ...
    
    // Публикация в Kafka
    // ...
    
    return &SubmitSMRespPDU{
        MessageID: msg.ID.String(),
        CommandStatus: ESME_ROK,
    }, nil
}
```

### Rate Limiting

Rate limiting настраивается per session:

```yaml
smpp:
  rate_limit_per_sec: 100
  max_connections: 1000
```

## HAProxy Configuration

HAProxy используется для балансировки нагрузки между gateway инстансами.

### Admin Gateway Routing

```haproxy
frontend admin_http
    bind *:8081
    mode http
    default_backend admin_gateway_http

backend admin_gateway_http
    mode http
    balance roundrobin
    option httpchk GET /health
    server admin-gw-1 admin-gateway-1:8081 check
    server admin-gw-2 admin-gateway-2:8081 check
    server admin-gw-3 admin-gateway-3:8081 check

frontend admin_grpc
    bind *:9091
    mode tcp
    default_backend admin_gateway_grpc

backend admin_gateway_grpc
    mode tcp
    balance roundrobin
    server admin-gw-1 admin-gateway-1:9091 check
    server admin-gw-2 admin-gateway-2:9091 check
    server admin-gw-3 admin-gateway-3:9091 check
```

### Client Gateway Routing

```haproxy
frontend client_http
    bind *:8080
    mode http
    default_backend client_gateway_http

backend client_gateway_http
    mode http
    balance roundrobin
    option httpchk GET /health
    server client-gw-1 client-gateway-1:8080 check
    server client-gw-2 client-gateway-2:8080 check
    server client-gw-3 client-gateway-3:8080 check

frontend client_grpc
    bind *:9090
    mode tcp
    default_backend client_gateway_grpc

backend client_gateway_grpc
    mode tcp
    balance roundrobin
    server client-gw-1 client-gateway-1:9090 check
    server client-gw-2 client-gateway-2:9090 check
    server client-gw-3 client-gateway-3:9090 check
```

### SMPP Gateway Routing

```haproxy
frontend smpp
    bind *:2775
    mode tcp
    default_backend smpp_gateway

backend smpp_gateway
    mode tcp
    balance roundrobin
    server smpp-gw-1 smpp-gateway-1:2775 check
    server smpp-gw-2 smpp-gateway-2:2775 check
    server smpp-gw-3 smpp-gateway-3:2775 check
```

## Общие компоненты Gateway

### gRPC Client Pool

Переиспользуемые gRPC клиенты для взаимодействия с сервисами:

```go
type ServiceClients struct {
    Auth      authpb.AuthServiceClient
    Messaging messagingpb.MessagingServiceClient
    Routing   routingpb.RoutingServiceClient
    Provider  providerpb.ProviderServiceClient
    Client    clientpb.ClientServiceClient
    Analytics analyticspb.AnalyticsServiceClient
    Billing   billingpb.BillingServiceClient
}

func NewServiceClients(cfg *Config) (*ServiceClients, error) {
    // Connection pooling
    // ...
}
```

### Error Handling

Унифицированная обработка ошибок:

```go
func (h *Handler) handleError(c *gin.Context, err error) {
    if st, ok := status.FromError(err); ok {
        switch st.Code() {
        case codes.Unauthenticated:
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        case codes.NotFound:
            c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
        case codes.InvalidArgument:
            c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
        default:
            c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
        }
    } else {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
    }
}
```

### Request Context

Передача контекста через весь запрос:

```go
type RequestContext struct {
    RequestID  string
    ClientID   uuid.UUID
    UserID     uuid.UUID
    StartTime  time.Time
}

func (h *Handler) SetRequestContext(c *gin.Context) {
    ctx := &RequestContext{
        RequestID: uuid.New().String(),
        StartTime: time.Now(),
    }
    
    if clientID, ok := c.Get("client_id"); ok {
        ctx.ClientID = clientID.(uuid.UUID)
    }
    
    c.Set("request_context", ctx)
    c.Set("logger", h.logger.With().Str("request_id", ctx.RequestID).Logger())
}
```

## Мониторинг Gateway

### Метрики

Все gateway экспортируют метрики Prometheus:

```
# HTTP метрики
http_requests_total{method="POST", path="/api/v1/sms/send", status="200"}
http_request_duration_seconds{method="POST", path="/api/v1/sms/send", quantile="0.95"}

# gRPC метрики
grpc_requests_total{method="SendSMS", status="ok"}
grpc_request_duration_seconds{method="SendSMS", quantile="0.95"}

# SMPP метрики
smpp_messages_received_total{bind_type="transceiver"}
smpp_connections_active
smpp_processing_duration_seconds

# Rate limiting
rate_limit_hits_total{client_id="..."}
```

### Health Checks

```go
func (s *Server) HealthCheck(c *gin.Context) {
    status := gin.H{
        "status":  "ok",
        "service": s.name,
        "uptime":  time.Since(s.startTime).String(),
    }
    
    // Проверка зависимостей
    if err := s.checkDependencies(); err != nil {
        status["status"] = "degraded"
        status["error"] = err.Error()
        c.JSON(http.StatusServiceUnavailable, status)
        return
    }
    
    c.JSON(http.StatusOK, status)
}
```

## Дополнительная документация

- [Обзор архитектуры](overview.md)
- [Admin Gateway API](../api/admin-gateway.md)
- [Client Gateway API](../api/client-gateway.md)
- [gRPC сервисы](../api/grpc-services.md)