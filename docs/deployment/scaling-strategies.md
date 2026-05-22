# Стратегии масштабирования

## Обзор

Этот документ дополняет [scaling.md](scaling.md) и описывает детальные стратегии масштабирования для микросервисной архитектуры.

## Типы масштабирования

### 1. Горизонтальное масштабирование (Scale Out)

**Добавление новых инстансов сервисов.**

**Преимущества:**
- Безлимитное масштабирование
- Высокая доступность
- Распределение нагрузки

**Недостатки:**
- Сложность координации
- Синхронизация состояния

### 2. Вертикальное масштабирование (Scale Up)

**Увеличение ресурсов существующих инстансов.**

**Преимущества:**
- Простота реализации
- Нет проблем с состоянием

**Недостатки:**
- Физические ограничения
- Высокая стоимость

## Стратегии по типам нагрузки

### CPU-интенсивные задачи

**Примеры:**
- Обработка сообщений
- Валидация данных
- Маршрутизация

**Стратегия:**
- Горизонтальное масштабирование
- Увеличение concurrency

```yaml
worker:
  concurrency: 20  # Увеличить параллелизм
```

### I/O-интенсивные задачи

**Примеры:**
- Работа с БД
- Kafka producer/consumer
- HTTP запросы

**Стратегия:**
- Горизонтальное масштабирование
- Connection pooling
- Асинхронная обработка

### Memory-интенсивные задачи

**Примеры:**
- Кэширование
- Буферизация сообщений

**Стратегия:**
- Вертикальное масштабирование (увеличение памяти)
- Внешний кэш (Redis)

## Стратегии по сервисам

### API Gateway

**Характеристики нагрузки:**
- Высокая частота запросов
- Низкая задержка
- Stateless

**Стратегия:**
1. **Горизонтальное масштабирование**
   ```yaml
   replicas: 5
   ```

2. **Connection pooling**
   ```go
   grpc.WithDefaultCallOptions(
       grpc.MaxCallRecvMsgSize(4*1024*1024),
   )
   ```

3. **Load balancing**
   - Round-robin
   - Least connections
   - IP hash (для sticky sessions)

### Messaging Service

**Характеристики нагрузки:**
- Высокая частота создания сообщений
- I/O операции (БД, Kafka)
- CPU для валидации

**Стратегия:**
1. **Горизонтальное масштабирование**
   ```bash
   docker-compose scale messaging-service=5
   ```

2. **Database connection pooling**
   ```go
   maxOpenConns: 100
   maxIdleConns: 10
   ```

3. **Kafka partitioning**
   ```bash
   kafka-topics --alter --topic sms.message.created --partitions 10
   ```

### Routing Service

**Характеристики нагрузки:**
- CPU-интенсивные вычисления
- Кэширование маршрутов
- Низкая задержка

**Стратегия:**
1. **Горизонтальное масштабирование**
   ```yaml
   replicas: 3
   ```

2. **In-memory cache**
   ```go
   cache := freecache.NewCache(100 * 1024 * 1024) // 100MB
   ```

3. **Kafka consumer groups**
   - Автоматическое распределение партиций

### Provider Service

**Характеристики нагрузки:**
- SMPP соединения
- Connection pooling
- I/O операции

**Стратегия:**
1. **Горизонтальное масштабирование**
   ```yaml
   replicas: 3
   ```

2. **Connection pool per provider**
   ```go
   maxConnections: 10
   minConnections: 2
   ```

3. **Load balancing между провайдерами**

### Analytics Service

**Характеристики нагрузки:**
- Агрегация данных
- CPU-интенсивные операции
- Batch processing

**Стратегия:**
1. **Горизонтальное масштабирование**
   ```yaml
   replicas: 2
   ```

2. **Batch processing**
   ```go
   batchSize: 1000
   batchTimeout: 5s
   ```

3. **Timeseries database**
   - Использование специализированной БД

## Автоматическое масштабирование

### Kubernetes HPA (Horizontal Pod Autoscaler)

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: messaging-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: messaging-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 0
      policies:
      - type: Percent
        value: 100
        periodSeconds: 30
      - type: Pods
        value: 2
        periodSeconds: 30
      selectPolicy: Max
```

### Custom Metrics

```yaml
metrics:
- type: Pods
  pods:
    metric:
      name: kafka_consumer_lag
    target:
      type: AverageValue
      averageValue: "1000"
```

## Масштабирование инфраструктуры

### PostgreSQL

**Стратегии:**
1. **Read Replicas**
   ```yaml
   read_replicas:
     - host: postgres-replica-1
     - host: postgres-replica-2
   ```

2. **Connection Pooling**
   - PgBouncer
   - pgpool-II

3. **Партиционирование**
   ```sql
   CREATE TABLE messages_2024_01 PARTITION OF messages
   FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
   ```

### Kafka

**Стратегии:**
1. **Увеличение партиций**
   ```bash
   kafka-topics --alter --topic sms.message.created --partitions 20
   ```

2. **Увеличение replication factor**
   ```bash
   kafka-topics --alter --topic sms.message.created --replication-factor 3
   ```

3. **Добавление брокеров**
   ```yaml
   kafka-1:
     # ...
   kafka-2:
     # ...
   kafka-3:
     # ...
   ```

### Redis

**Стратегии:**
1. **Redis Cluster**
   ```yaml
   redis-cluster:
     nodes:
       - redis-1:6379
       - redis-2:6379
       - redis-3:6379
   ```

2. **Sharding**
   - По client_id
   - По message_id

## Мониторинг для масштабирования

### Ключевые метрики

1. **CPU Usage**
   - Целевое значение: < 70%
   - Порог масштабирования: > 80%

2. **Memory Usage**
   - Целевое значение: < 80%
   - Порог масштабирования: > 90%

3. **Request Rate**
   - Количество запросов в секунду
   - Целевое значение: зависит от сервиса

4. **Latency**
   - p95 latency
   - Целевое значение: < 100ms для API

5. **Queue Size**
   - Kafka lag
   - Целевое значение: < 1000 сообщений

### Алерты

```yaml
- alert: HighCPUUsage
  expr: cpu_usage > 0.8
  for: 5m
  annotations:
    summary: "High CPU usage on {{ $labels.instance }}"

- alert: HighMemoryUsage
  expr: memory_usage > 0.9
  for: 5m
  annotations:
    summary: "High memory usage on {{ $labels.instance }}"

- alert: HighLatency
  expr: http_request_duration_seconds{quantile="0.95"} > 0.5
  for: 5m
  annotations:
    summary: "High latency on {{ $labels.service }}"
```

## Capacity Planning

### Расчет ресурсов

**Формула:**
```
Required Replicas = (Peak Load / Capacity per Replica) * Safety Factor

Safety Factor = 1.2 (20% запас)
```

**Пример:**
- Peak Load: 10,000 req/s
- Capacity per Replica: 2,000 req/s
- Required Replicas: (10,000 / 2,000) * 1.2 = 6 replicas

### Нагрузочное тестирование

```bash
# k6 нагрузочный тест
k6 run --vus 1000 --duration 5m load-test.js
```

## Best Practices

1. **Начинайте с малого** - начните с минимальной конфигурации
2. **Мониторьте метрики** - используйте Prometheus и Grafana
3. **Тестируйте масштабирование** - регулярно проводите нагрузочное тестирование
4. **Планируйте capacity** - планируйте ресурсы заранее
5. **Автоматизируйте** - используйте автоматическое масштабирование
6. **Оптимизируйте** - оптимизируйте код перед масштабированием
7. **Документируйте** - документируйте изменения конфигурации

## Дополнительная документация

- [Масштабирование](scaling.md) - базовая документация
- [Развертывание микросервисов](microservices-deployment.md)
- [Мониторинг](monitoring.md)