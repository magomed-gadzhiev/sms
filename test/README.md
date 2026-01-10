# Тестирование

## Обзор

Проект включает три уровня тестирования:
1. **Unit тесты** - быстрые тесты отдельных компонентов
2. **Integration тесты** - тесты взаимодействия компонентов с реальными зависимостями
3. **Load тесты** - нагрузочное тестирование для проверки производительности

## Структура тестов

```
test/
├── integration/          # Integration тесты
│   ├── grpc_services_test.go        # Тесты gRPC сервисов
│   ├── service_communication_test.go # Тесты взаимодействия сервисов
│   ├── kafka_test.go                # Тесты Kafka
│   ├── storage_test.go              # Тесты репозиториев
│   └── README.md
├── load/                # Load тесты
│   ├── api_load_test.go             # Базовые load тесты
│   ├── high_performance_test.go     # Тесты для 10K msg/s
│   └── README.md
└── performance/         # Performance тесты
    └── performance_tuning.go        # Тесты оптимизации производительности
```

## Запуск тестов

### Все тесты

```bash
# PowerShell
.\scripts\run-performance-tests.ps1

# Linux/Mac
./scripts/run-performance-tests.sh
```

### Индивидуальные тесты

#### Integration тесты

```bash
# Требуют запущенных сервисов (PostgreSQL, Kafka)
go test -tags=integration -v ./test/integration/...

# Конкретный тест
go test -tags=integration -v ./test/integration/... -run TestAuthService_ValidateToken
```

#### Load тесты

```bash
# Требуют запущенного API Gateway
go test -tags=load -v ./test/load/...

# Тест для 10K msg/s
go test -tags=load -v ./test/load/... -run TestHighPerformanceLoad
```

#### Performance тесты

```bash
# Тесты оптимизации производительности
go test -tags=integration -v ./test/performance/...
```

### K6 Load Testing

Для более продвинутого нагрузочного тестирования используется k6:

```bash
# Установка k6
# Windows: choco install k6
# Linux: https://k6.io/docs/getting-started/installation/

# Запуск теста для 10K msg/s
k6 run scripts/k6_10k_load_test.js

# С кастомными параметрами
BASE_URL=http://localhost:8080 API_KEY=test-key k6 run scripts/k6_10k_load_test.js
```

## Требования для тестов

### Integration тесты

- PostgreSQL (по умолчанию: `localhost:5432`, БД: `smpp_test`)
- Kafka (по умолчанию: `localhost:9092`)
- Redis (опционально, для кэширования)

Переменные окружения:
```bash
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=smpp_test
export TEST_DB_PASSWORD=smpp_test
export TEST_DB_NAME=smpp_test
```

### Load тесты

- API Gateway запущен на `http://localhost:8080`
- Валидный API ключ для аутентификации

Переменные окружения:
```bash
export BASE_URL=http://localhost:8080
export API_KEY=test-api-key
```

### gRPC Service тесты

Требуют запущенных сервисов на следующих портах:
- Auth Service: `localhost:9101`
- Messaging Service: `localhost:9092`
- Routing Service: `localhost:9094`
- Provider Service: `localhost:9093`
- Client Service: `localhost:9095`
- Billing Service: `localhost:9097`

## Использование Docker Compose

Для запуска тестовых сервисов:

```bash
# Запуск инфраструктуры
docker-compose -f deployments/docker-compose.yml up -d postgres kafka redis

# Запуск сервисов
docker-compose -f deployments/docker-compose.yml up -d

# Ожидание готовности
# (проверьте логи сервисов)

# Запуск тестов
go test -tags=integration ./test/integration/...
```

## Типы тестов

### Integration тесты

#### gRPC Services (`grpc_services_test.go`)

Тестируют gRPC API всех микросервисов:
- `TestAuthService_ValidateToken` - валидация токенов
- `TestMessagingService_SendMessage` - отправка сообщений
- `TestRoutingService_GetRoute` - получение маршрутов
- `TestProviderService_ListProviders` - список провайдеров
- `TestClientService_GetClient` - получение клиентов
- `TestBillingService_GetBalance` - получение баланса
- `TestServiceIntegration_EndToEndFlow` - полный E2E поток

#### Service Communication (`service_communication_test.go`)

Тестируют взаимодействие между сервисами:
- `TestKafkaToMessagingServiceFlow` - поток Kafka -> Messaging Service
- `TestGatewayToServiceFlow` - поток Gateway -> Service
- `TestConcurrentRequests` - конкурентные запросы

### Load тесты

#### High Performance (`high_performance_test.go`)

Тесты для достижения 10K сообщений/сек:
- `TestHighPerformanceLoad` - основной тест 10K msg/s
- `TestSustainedLoad` - устойчивая нагрузка в течение 10 минут

#### API Load (`api_load_test.go`)

Базовые нагрузочные тесты:
- `TestAPIGateway_Load` - базовая нагрузка (10 воркеров, 1000 запросов)
- `TestAPIGateway_Stress` - стресс-тест с увеличивающейся нагрузкой
- `TestAPIGateway_SendBatchSMS_Load` - нагрузка на batch endpoint
- `TestAPIGateway_GetStatus_Load` - нагрузка на получение статуса
- `TestAPIGateway_MixedLoad` - смешанная нагрузка на все endpoints

### Performance тесты

#### Performance Tuning (`performance_tuning.go`)

Тесты оптимизации:
- `TestDatabaseConnectionPool` - тестирование различных конфигураций пула БД
- `TestKafkaBatchSize` - оптимизация размера батча Kafka
- `TestConnectionPoolPerformance` - производительность пула соединений
- `BenchmarkDatabasePool` - бенчмарк пула БД
- `BenchmarkBatchProcessing` - бенчмарк батч обработки

## Целевые метрики

### Для 10K msg/s:

- **Throughput**: ≥ 10,000 сообщений/сек
- **Latency**: 
  - p50 < 50ms
  - p95 < 500ms
  - p99 < 1000ms
- **Success Rate**: > 99%
- **Error Rate**: < 1%

## Результаты тестов

Тесты выводят детальную статистику:
- Общее количество запросов
- Количество успешных/неудачных запросов
- Throughput (запросов/сообщений в секунду)
- Latency (p50, p95, p99)
- Success rate (%)

## Troubleshooting

### Проблема: Тесты не находят сервисы

**Решение**: Убедитесь, что все сервисы запущены и доступны:
```bash
# Проверка доступности
curl http://localhost:8080/health
telnet localhost 9092  # Kafka
telnet localhost 5432  # PostgreSQL
```

### Проблема: Timeout при запуске тестов

**Решение**: Увеличьте timeout:
```bash
go test -tags=integration -v ./test/integration/... -timeout 30m
```

### Проблема: Низкий throughput в load тестах

**Решение**: 
1. Проверьте конфигурацию (см. `configs/config.performance.yaml`)
2. Убедитесь, что нет узких мест (CPU, Memory, Network)
3. Увеличьте количество воркеров в тесте

## Дополнительная документация

- [Performance Optimization](../docs/development/performance-optimization.md)
- [Testing Strategy](../docs/development/testing-strategy.md)
- [Testing Guide](../docs/development/testing.md)