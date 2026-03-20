# Quickstart: SMS Gateway Platform

## Prerequisites

- Docker & Docker Compose
- Go 1.24+
- Protocol Buffers compiler (protoc)
- Make

## Start Infrastructure

```bash
cd deployments
docker compose up -d postgres redis kafka zookeeper
```

Wait for all services to be healthy:
```bash
docker compose ps
```

## Run Database Migrations

```bash
# Migrations are auto-applied on service startup
# Or run manually:
cd migrations && psql -h localhost -U postgres -d smpp_server -f *.sql
```

## Start All Services

```bash
docker compose up -d
```

This starts:
- 2x Client Gateway (HTTP:8080, gRPC:9090 via HAProxy)
- 2x Admin Gateway (HTTP:8081, gRPC:9091 via HAProxy)
- SMPP Gateway (SMPP:2775)
- Auth, Messaging, Routing, Provider, Client, Analytics, Billing, Webhook, Template services
- Prometheus (9090) + Grafana (3000)

## Verify Health

```bash
# Check all containers
docker compose ps

# Prometheus targets
curl http://localhost:9090/targets
```

## Send Your First SMS

```bash
# 1. Get API key (via admin API or database)
# 2. Send SMS
curl -X POST http://localhost:8080/api/v1/sms/send \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "source": "+79001234567",
    "destination": "+79007654321",
    "body": "Hello from SMS Gateway!"
  }'

# 3. Check status
curl http://localhost:8080/api/v1/sms/status/{message_id} \
  -H "X-API-Key: your-api-key"
```

## Run Tests

```bash
# Unit tests
go test ./...

# Integration tests (requires running infrastructure)
go test -tags=integration ./test/integration/...
```

## Key Configuration (Environment Variables)

| Variable | Default | Description |
|----------|---------|-------------|
| DB_HOST | localhost | PostgreSQL host |
| DB_PORT | 5432 | PostgreSQL port |
| DB_NAME | smpp_server | Database name |
| REDIS_HOST | localhost | Redis host |
| KAFKA_BROKERS | localhost:9092 | Kafka broker list |
| SCHEDULER_INTERVAL | 10s | Scheduled message check interval |
| SCHEDULER_BATCH_SIZE | 100 | Scheduled message batch size |
| DLR_EXPIRY_TIMEOUT | 24h | DLR timeout before "expired" status |
| DATA_RETENTION_MESSAGES | 90d | Message data retention period |
| DATA_RETENTION_AUDIT | 365d | Audit log retention period |

## Monitoring

- Grafana: http://localhost:3000 (admin/admin)
- Prometheus: http://localhost:9090
- Each service exposes /metrics on its metrics port (2112-2120)
