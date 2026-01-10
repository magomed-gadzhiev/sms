# Оптимизация производительности

## Обзор

Этот документ описывает настройки и рекомендации для достижения производительности **10,000 сообщений в секунду**.

## Ключевые метрики

### Целевые показатели

- **Throughput**: 10,000+ сообщений/сек
- **Latency**: 
  - p50 < 50ms
  - p95 < 500ms
  - p99 < 1000ms
- **Success Rate**: > 99%
- **Error Rate**: < 1%

## Конфигурация для высокой производительности

### 1. База данных (PostgreSQL)

```yaml
database:
  max_open_conns: 100        # Критически важно для 10K msg/s
  max_idle_conns: 50         # Поддержание горячих соединений
  conn_max_lifetime: 30m     # Переиспользование соединений
  conn_max_idle_time: 5m     # Оптимальное время idle
```

**Рекомендации:**

- Используйте connection pooling (PgBouncer или встроенный пул)
- Настройте PostgreSQL на высокую нагрузку:
  ```sql
  max_connections = 200
  shared_buffers = 4GB
  effective_cache_size = 12GB
  work_mem = 16MB
  maintenance_work_mem = 512MB
  ```

### 2. Redis

```yaml
redis:
  pool_size: 50              # Увеличенный пул соединений
  min_idle_conns: 25         # Горячие соединения
```

**Рекомендации:**

- Используйте Redis Cluster для горизонтального масштабирования
- Настройте persistence для надежности
- Мониторьте память и используйте eviction policies

### 3. Kafka

```yaml
kafka:
  producer:
    batch_size: 100           # Размер батча
    batch_timeout: 10ms       # Таймаут батча
    compression: snappy       # Сжатие
    acks: 1                   # Баланс производительности/надежности
  consumer:
    fetch_min_bytes: 1024
    fetch_max_wait_ms: 100
```

**Рекомендации:**

- Используйте партиционирование (10+ партиций для topic)
- Настройте replication factor = 3 для надежности
- Оптимизируйте retention и cleanup policies
- Используйте batch processing на стороне consumer

### 4. HTTP/gRPC серверы

```yaml
api:
  http:
    max_connections: 10000    # Максимум одновременных соединений
    read_timeout: 10s
    write_timeout: 10s
    idle_timeout: 120s
  grpc:
    max_concurrent_streams: 1000
    initial_window_size: 1048576
```

**Рекомендации:**

- Используйте connection pooling на стороне клиентов
- Включите HTTP/2 для gRPC
- Настройте keep-alive для переиспользования соединений

### 5. Worker (обработка сообщений)

```yaml
worker:
  concurrency: 50             # Количество параллельных воркеров
  batch_size: 100             # Размер батча для обработки
  batch_timeout: 5s           # Таймаут формирования батча
  message_buffer_size: 1000   # Буфер сообщений
```

**Рекомендации:**

- Увеличьте concurrency в зависимости от CPU
- Используйте батч обработку для снижения overhead
- Мониторьте размер очереди сообщений

## Оптимизация кода

### 1. Connection Pooling

Всегда используйте пулы соединений вместо создания новых:

```go
// Хорошо
db.SetMaxOpenConns(100)
db.SetMaxIdleConns(50)

// Плохо
db, _ := sql.Open(...) // без настройки пула
```

### 2. Batch Processing

Группируйте операции для снижения overhead:

```go
// Хорошо
batch := make([]Message, 0, 100)
for _, msg := range messages {
    batch = append(batch, msg)
    if len(batch) >= 100 {
        processBatch(batch)
        batch = batch[:0]
    }
}

// Плохо
for _, msg := range messages {
    processMessage(msg) // Отдельный запрос для каждого сообщения
}
```

### 3. Кэширование

Используйте кэш для часто запрашиваемых данных:

```go
// Кэширование клиентов, провайдеров, маршрутов
client, err := cache.Get(key)
if err != nil {
    client, err = db.GetClient(id)
    cache.Set(key, client, 5*time.Minute)
}
```

### 4. Асинхронная обработка

Используйте Kafka для асинхронной обработки:

```go
// Публикация в Kafka (быстро, не блокирует)
producer.Publish(message)

// Обработка в фоне через consumer
consumer.Consume() // Обработка в отдельном воркере
```

## Мониторинг производительности

### Метрики для отслеживания

1. **Throughput** (req/s, msg/s)
2. **Latency** (p50, p95, p99)
3. **Error Rate**
4. **Connection Pool Utilization**
5. **Queue Depth** (Kafka)
6. **Database Connection Wait Time**
7. **GC Pause Time**

### Prometheus Queries

```promql
# Throughput
rate(http_requests_total[1m])

# Latency
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Error Rate
rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m])

# Connection Pool
db_pool_open_connections / db_pool_max_connections
```

## Нагрузочное тестирование

### Запуск тестов

```bash
# Go load tests
go test -tags=load ./test/load/... -v

# K6 тест для 10K msg/s
k6 run scripts/k6_10k_load_test.js
```

### Анализ результатов

1. **Throughput** - достигли ли цели 10K msg/s?
2. **Latency** - соответствуют ли p95/p99 требованиям?
3. **Error Rate** - меньше 1%?
4. **Resource Usage** - CPU, Memory, Network

## Troubleshooting

### Проблема: Низкий throughput (< 10K msg/s)

**Возможные причины:**
- Недостаточно connection pool в БД
- Узкое место в Kafka (недостаточно партиций)
- Неоптимальный batch size
- Bottleneck в обработке сообщений

**Решение:**
- Увеличьте `max_open_conns` в БД
- Увеличьте количество партиций в Kafka topic
- Оптимизируйте batch processing
- Увеличьте concurrency воркеров

### Проблема: Высокая latency (> 500ms p95)

**Возможные причины:**
- Медленные запросы к БД
- Очередь сообщений в Kafka
- GC паузы
- Network latency

**Решение:**
- Оптимизируйте SQL запросы (индексы, query plan)
- Увеличьте количество consumer'ов
- Настройте GC (GOGC=100)
- Используйте более быструю сеть

### Проблема: Высокий error rate (> 1%)

**Возможные причины:**
- Timeout'ы на запросах
- Переполнение connection pool
- Ошибки обработки сообщений

**Решение:**
- Увеличьте timeout'ы
- Увеличьте размер пула соединений
- Улучшите обработку ошибок и retry логику

## Рекомендации по масштабированию

### Горизонтальное масштабирование

1. **Gateway Layer**: Масштабируйте через HAProxy/Load Balancer
2. **Services**: Масштабируйте каждый сервис независимо
3. **Workers**: Масштабируйте количество worker инстансов
4. **Kafka**: Увеличьте количество партиций и consumer'ов

### Вертикальное масштабирование

1. **CPU**: Минимум 4 cores для обработки 10K msg/s
2. **Memory**: Минимум 8GB RAM
3. **Network**: Высокая пропускная способность сети
4. **Storage**: SSD для БД и Kafka logs

## Чеклист для production

- [ ] PostgreSQL настроен на высокую нагрузку
- [ ] Connection pool настроен оптимально (100+ connections)
- [ ] Kafka topic имеет достаточное количество партиций (10+)
- [ ] Redis пул настроен (50+ connections)
- [ ] Batch processing включен (batch_size: 100)
- [ ] Мониторинг настроен (Prometheus + Grafana)
- [ ] Load testing пройден успешно (10K msg/s)
- [ ] Latency соответствует требованиям (p95 < 500ms)
- [ ] Error rate < 1%
- [ ] Health checks настроены
- [ ] Alerting настроен для критических метрик