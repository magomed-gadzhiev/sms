# Развертывание микросервисов

## Обзор

Это руководство описывает процесс развертывания микросервисной архитектуры SMPP сервера.

## Архитектура развертывания

```
┌─────────────────────────────────────────┐
│         Load Balancer (HAProxy)         │
└─────────────────┬───────────────────────┘
                  │
        ┌─────────┼─────────┐
        │         │         │
┌───────▼──┐ ┌───▼────┐ ┌──▼──────┐
│ Admin    │ │ Client │ │ SMPP    │
│ Gateway  │ │ Gateway│ │ Gateway │
│ (3x)     │ │ (3x)   │ │ (2x)    │
└──────┬───┘ └───┬────┘ └───┬─────┘
       │         │          │
       └─────────┼──────────┘
                 │
    ┌────────────┼────────────┐
    │            │            │
┌───▼────┐  ┌───▼────┐  ┌───▼────┐
│ Auth   │  │Messaging│ │Routing │
│Service │  │ Service │ │Service │
│ (2x)   │  │  (5x)   │ │  (3x)  │
└────────┘  └─────────┘ └────────┘
    │            │            │
    └────────────┼────────────┘
                 │
    ┌────────────┼────────────┐
    │            │            │
┌───▼────┐  ┌───▼────┐  ┌───▼────┐
│Provider│  │Client  │  │Analytics│
│Service │  │Service │  │Service │
│ (3x)   │  │ (2x)   │  │ (2x)   │
└────────┘  └────────┘  └────────┘
    │            │            │
    └────────────┼────────────┘
                 │
    ┌────────────┼────────────┐
    │            │            │
┌───▼────┐  ┌───▼────┐  ┌────────┐
│Billing │  │Kafka   │  │PostgreSQL│
│Service │  │ (3x)   │  │ (1x)   │
│ (2x)   │  └────────┘  └────────┘
└────────┘       │
            ┌────┴────┐
            │ Redis   │
            │ (3x)    │
            └─────────┘
```

## Docker Compose развертывание

### Базовый docker-compose.yml

```yaml
version: '3.8'

services:
  # PostgreSQL
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: smpp
      POSTGRES_PASSWORD: smpp
      POSTGRES_DB: smpp
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  # Kafka
  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000

  kafka-1:
    image: confluentinc/cp-kafka:latest
    depends_on:
      - zookeeper
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 3

  # Redis
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  # Gateway services
  admin-gateway-1:
    build:
      context: .
      dockerfile: deployments/docker/admin-gateway.Dockerfile
    ports:
      - "8081:8081"
      - "9091:9091"
    environment:
      - DB_HOST=postgres
      - KAFKA_BROKERS=kafka-1:9092
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - kafka-1
      - redis

  client-gateway-1:
    build:
      context: .
      dockerfile: deployments/docker/client-gateway.Dockerfile
    ports:
      - "8080:8080"
      - "9090:9090"
    environment:
      - DB_HOST=postgres
      - KAFKA_BROKERS=kafka-1:9092
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - kafka-1
      - redis

  # Microservices
  auth-service-1:
    build:
      context: .
      dockerfile: deployments/docker/service-base.Dockerfile
    command: ["./auth-service"]
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - redis

  messaging-service-1:
    build:
      context: .
      dockerfile: deployments/docker/service-base.Dockerfile
    command: ["./messaging-service"]
    environment:
      - DB_HOST=postgres
      - KAFKA_BROKERS=kafka-1:9092
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - kafka-1
      - redis

volumes:
  postgres_data:
```

### Запуск

```bash
docker-compose -f deployments/docker-compose.yml up -d
```

### Масштабирование

```bash
# Масштабирование messaging-service до 3 инстансов
docker-compose -f deployments/docker-compose.yml up -d --scale messaging-service-1=3
```

## Kubernetes развертывание

### Deployment для сервиса

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: messaging-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: messaging-service
  template:
    metadata:
      labels:
        app: messaging-service
    spec:
      containers:
      - name: messaging-service
        image: smpp-server/messaging-service:latest
        ports:
        - containerPort: 50052
          name: grpc
        env:
        - name: DB_HOST
          value: postgres-service
        - name: KAFKA_BROKERS
          value: kafka-service:9092
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        livenessProbe:
          exec:
            command:
            - /bin/grpc_health_probe
            - -addr=:50052
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          exec:
            command:
            - /bin/grpc_health_probe
            - -addr=:50052
          initialDelaySeconds: 10
          periodSeconds: 5
```

### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: messaging-service
spec:
  selector:
    app: messaging-service
  ports:
  - port: 50052
    targetPort: grpc
    protocol: TCP
  type: ClusterIP
```

### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: messaging-service-config
data:
  config.yaml: |
    database:
      host: postgres-service
      port: 5432
      user: smpp
      password: smpp
      database: smpp
    kafka:
      brokers:
        - kafka-service:9092
```

### Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: messaging-service-secrets
type: Opaque
stringData:
  db-password: smpp
  api-key: secret-key
```

## Health Checks

### gRPC Health Check

```go
import (
    "google.golang.org/grpc/health"
    "google.golang.org/grpc/health/grpc_health_v1"
)

healthServer := health.NewServer()
grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

// Установка статуса
healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
```

### HTTP Health Check

```go
func (s *Server) HealthCheck(c *gin.Context) {
    status := gin.H{
        "status": "ok",
        "service": s.name,
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

## Graceful Shutdown

```go
func main() {
    ctx, cancel := signal.NotifyContext(
        context.Background(),
        os.Interrupt,
        syscall.SIGTERM,
    )
    defer cancel()
    
    // Запуск сервера
    go func() {
        if err := server.Start(); err != nil {
            log.Fatal().Err(err).Msg("Server failed")
        }
    }()
    
    // Graceful shutdown
    <-ctx.Done()
    log.Info().Msg("Shutting down...")
    
    shutdownCtx, shutdownCancel := context.WithTimeout(
        context.Background(),
        30*time.Second,
    )
    defer shutdownCancel()
    
    if err := server.Shutdown(shutdownCtx); err != nil {
        log.Error().Err(err).Msg("Shutdown error")
    }
    
    log.Info().Msg("Shutdown complete")
}
```

## Мониторинг

### Prometheus метрики

Все сервисы экспортируют метрики на `/metrics`:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

http.Handle("/metrics", promhttp.Handler())
```

### Service Discovery

В Kubernetes используйте DNS для service discovery:

```go
// Подключение к сервису
conn, err := grpc.Dial(
    "messaging-service:50052",
    grpc.WithInsecure(),
)
```

## Развертывание по этапам

### Этап 1: Инфраструктура

1. Развернуть PostgreSQL
2. Развернуть Kafka
3. Развернуть Redis
4. Проверить connectivity

### Этап 2: Основные сервисы

1. Развернуть Auth Service
2. Развернуть Client Service
3. Проверить работоспособность

### Этап 3: Gateway

1. Развернуть Admin Gateway
2. Развернуть Client Gateway
3. Настроить HAProxy
4. Проверить маршрутизацию

### Этап 4: Остальные сервисы

1. Развернуть Messaging Service
2. Развернуть Routing Service
3. Развернуть Provider Service
4. Развернуть Analytics Service
5. Развернуть Billing Service

### Этап 5: SMPP Gateway

1. Развернуть SMPP Gateway
2. Настроить балансировку
3. Проверить соединения

## Откат (Rollback)

### Docker Compose

```bash
# Откат к предыдущей версии
docker-compose -f deployments/docker-compose.yml down
docker-compose -f deployments/docker-compose.yml.old up -d
```

### Kubernetes

```bash
# Откат deployment
kubectl rollout undo deployment/messaging-service

# Просмотр истории
kubectl rollout history deployment/messaging-service
```

## Best Practices

1. **Health Checks** - все сервисы должны иметь health checks
2. **Graceful Shutdown** - корректное завершение работы
3. **Resource Limits** - установка лимитов ресурсов
4. **Secrets Management** - использование секретов для чувствительных данных
5. **Monitoring** - мониторинг всех сервисов
6. **Logging** - централизованное логирование
7. **Versioning** - версионирование API и сервисов

## Дополнительная документация

- [Docker Compose](docker-compose.md)
- [Конфигурация](configuration.md)
- [Масштабирование](scaling.md)
- [Мониторинг](monitoring.md)