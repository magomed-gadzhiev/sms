# Quickstart: Multi-tenant Self-Service Portal

**Feature Branch**: `003-self-service-portal`

## Prerequisites

- Go 1.24+
- Node.js 20+ (для frontend)
- Docker & Docker Compose
- Все существующие сервисы из `deployments/docker-compose.yml`

## Запуск

### 1. Применить миграции

```bash
# Из корня проекта
migrate -path migrations -database "postgres://user:pass@localhost:5432/smsgateway?sslmode=disable" up
```

### 2. Запуск backend (portal-gateway)

```bash
# Переменные окружения
export PORTAL_HTTP_PORT=8082
export AUTH_SERVICE_ADDR=localhost:9101
export CLIENT_SERVICE_ADDR=localhost:9095
export BILLING_SERVICE_ADDR=localhost:9097
export MESSAGING_SERVICE_ADDR=localhost:19092
export ANALYTICS_SERVICE_ADDR=localhost:9096
export WEBHOOK_SERVICE_ADDR=localhost:9098
export REDIS_ADDR=localhost:6379
export SESSION_SECRET=your-secret-key-here
export TOTP_ENCRYPTION_KEY=32-byte-hex-key-here
export CSRF_SECRET=your-csrf-secret-here

# Запуск
go run cmd/portal-gateway/main.go
```

### 3. Запуск frontend

```bash
cd portal-frontend
npm install
npm run dev
# Доступен на http://localhost:5173
# Проксирует API на http://localhost:8082
```

### 4. Через Docker Compose

```bash
docker-compose -f deployments/docker-compose.yml up -d
# Portal: http://localhost:8082
# Frontend: http://localhost (через HAProxy)
```

## Тестирование

### Unit-тесты

```bash
go test ./internal/services/auth/... -v
go test ./internal/services/client/... -v
go test ./internal/services/billing/... -v
go test ./internal/gateway/portal/... -v
```

### Integration-тесты

```bash
go test -tags=integration ./internal/services/auth/... -v
go test -tags=integration ./internal/services/client/... -v
go test -tags=integration ./internal/services/billing/... -v
```

### Frontend-тесты

```bash
cd portal-frontend
npm test
```

## Проверка работоспособности

```bash
# Health check
curl http://localhost:8082/health

# Login
curl -X POST http://localhost:8082/portal/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"client@example.com","password":"password123"}' \
  -c cookies.txt

# Dashboard
curl http://localhost:8082/portal/v1/dashboard \
  -b cookies.txt

# Messages
curl "http://localhost:8082/portal/v1/messages?page=1&per_page=10" \
  -b cookies.txt
```

## Ключевые конфигурации

| Variable | Default | Description |
|----------|---------|-------------|
| PORTAL_HTTP_PORT | 8082 | HTTP порт портала |
| SESSION_SECRET | required | Секрет для подписи сессий |
| SESSION_TTL | 24h | Время жизни сессии |
| MAX_SESSIONS_PER_USER | 5 | Макс. одновременных сессий |
| TOTP_ENCRYPTION_KEY | required | AES-256 ключ для TOTP-секретов |
| LOGIN_MAX_ATTEMPTS | 5 | Попыток входа до блокировки |
| LOGIN_LOCKOUT_DURATION | 15m | Время блокировки |
| CORS_ALLOWED_ORIGINS | http://localhost:5173 | Разрешённые origins |
