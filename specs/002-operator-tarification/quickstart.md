# Quickstart: Operator-Based SMS Tarification System

## Prerequisites

- Go 1.24.0+
- PostgreSQL 15+
- Redis 7+
- Apache Kafka
- protoc + protoc-gen-go + protoc-gen-go-grpc
- Docker Compose (for full stack)

## Database Setup

Apply new migrations:

```bash
# From project root
migrate -path migrations -database "postgres://..." up
```

Migrations `000012` (countries/operators) and `000013` (tarification tables) will be applied.

## Proto Generation

After modifying/adding proto files:

```bash
# Generate tarification-service proto
protoc --go_out=. --go-grpc_out=. api/proto/tarification/tarification.proto

# Regenerate routing-service proto (extended with Country/Operator RPCs)
protoc --go_out=. --go-grpc_out=. api/proto/routing/routing.proto
```

## Running the Service

### With Docker Compose

Add `tarification-service` to `docker-compose.yml`:

```yaml
tarification-service:
  build:
    context: .
    dockerfile: Dockerfile
    args:
      SERVICE: tarification-service
  environment:
    - DB_HOST=postgres
    - DB_PORT=5432
    - DB_NAME=sms
    - GRPC_PORT=9098
    - METRICS_PORT=2119
    - KAFKA_BROKERS=kafka:9092
    - BILLING_GRPC_ADDR=billing-service:9097
  ports:
    - "9098:9098"
    - "2119:2119"
  depends_on:
    - postgres
    - kafka
    - billing-service
```

### Standalone

```bash
go run cmd/services/tarification-service/main.go
```

## Configuration

| Env Variable | Default | Description |
|-------------|---------|-------------|
| GRPC_PORT | 9098 | gRPC server port |
| METRICS_PORT | 2119 | Prometheus metrics HTTP port |
| DB_HOST | localhost | PostgreSQL host |
| DB_PORT | 5432 | PostgreSQL port |
| DB_NAME | sms | Database name |
| KAFKA_BROKERS | localhost:9092 | Kafka broker addresses |
| BILLING_GRPC_ADDR | localhost:9097 | Billing-service gRPC address |

## Verification

### 1. Create a country

```bash
curl -X POST http://localhost:8080/admin/v1/countries \
  -H "Content-Type: application/json" \
  -d '{"name": "Russia", "iso_code": "RU", "phone_code": "+7", "currency": "RUB"}'
```

### 2. Create an operator

```bash
curl -X POST http://localhost:8080/admin/v1/operators \
  -H "Content-Type: application/json" \
  -d '{"country_id": "<country_uuid>", "name": "MTS", "code": "mts-ru", "supports_paid_sender": true, "supports_free_sender": true}'
```

### 3. Add prefix

```bash
curl -X POST http://localhost:8080/admin/v1/operators/<operator_uuid>/prefixes \
  -H "Content-Type: application/json" \
  -d '{"prefix": "+7900", "priority": 0}'
```

### 4. Create tariff plan

```bash
curl -X POST http://localhost:8080/admin/v1/tarification/tariff-plans \
  -H "Content-Type: application/json" \
  -d '{"operator_id": "<operator_uuid>", "sender_category": "shared", "strategy": "fixed"}'
```

### 5. Create period with tier

```bash
curl -X POST http://localhost:8080/admin/v1/tarification/tariff-periods \
  -H "Content-Type: application/json" \
  -d '{"tariff_plan_id": "<plan_uuid>", "start_date": "2026-03-01", "end_date": "2026-04-01"}'

curl -X POST http://localhost:8080/admin/v1/tarification/tariff-tiers \
  -H "Content-Type: application/json" \
  -d '{"tariff_period_id": "<period_uuid>", "from_count": 0, "price_per_segment": "2.000000"}'
```

### 6. Send an SMS and verify tarification

Send a message to a number matching the operator prefix and verify:
- Correct amount was charged via billing-service
- UsageCounter was incremented
- TarificationLog entry was created
