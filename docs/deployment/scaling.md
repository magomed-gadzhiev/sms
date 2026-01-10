# Масштабирование системы

## Обзор

Система SMPP сервера спроектирована для горизонтального масштабирования. Каждый сервис может масштабироваться независимо в зависимости от нагрузки.

## Стратегии масштабирования

### Горизонтальное масштабирование

Добавление новых инстансов сервисов для распределения нагрузки.

### Вертикальное масштабирование

Увеличение ресурсов (CPU, память) существующих инстансов.

## Масштабирование API Gateway

### Горизонтальное масштабирование

API Gateway является stateless сервисом и легко масштабируется горизонтально.

#### Docker Compose

```bash
# Масштабирование до 3 инстансов
docker-compose -f deployments/docker-compose.yml up -d --scale api-gateway-1=3

# Или добавить новые инстансы в docker-compose.yml
```

#### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-gateway
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: api-gateway
        image: smpp-server/api-gateway:latest
```

### Вертикальное масштабирование

Увеличение ресурсов контейнера:

```yaml
services:
  api-gateway-1:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
```

### Рекомендации

- **Минимум:** 2 инстанса для высокой доступности
- **Оптимально:** 3-5 инстансов для средних нагрузок
- **Высокая нагрузка:** 10+ инстансов

### Метрики для масштабирования

- CPU использование > 70%
- Память использование > 80%
- HTTP request latency p95 > 500ms
- Rate limit hits > 10% запросов

## Масштабирование SMPP Server

### Горизонтальное масштабирование

SMPP Server может масштабироваться, но требует настройки балансировки соединений.

#### Вариант 1: Балансировка на уровне TCP

Использование HAProxy или аналогичного балансировщика для распределения TCP соединений:

```yaml
haproxy:
  backend smpp_backend
    balance roundrobin
    server smpp1 smpp-server-1:2775 check
    server smpp2 smpp-server-2:2775 check
    server smpp3 smpp-server-3:2775 check
```

#### Вариант 2: Клиентское подключение к нескольким серверам

Клиенты подключаются к разным серверам вручную.

### Вертикальное масштабирование

Увеличение ресурсов для обработки большего количества соединений:

```yaml
smpp:
  max_connections: 5000  # Увеличить с 1000
```

### Рекомендации

- **Низкая нагрузка:** 1 инстанс
- **Средняя нагрузка:** 2-3 инстанса
- **Высокая нагрузка:** 5+ инстансов

### Метрики для масштабирования

- Активные соединения > 80% от max_connections
- CPU использование > 70%
- Память использование > 80%
- Очередь сообщений растет

## Масштабирование Worker

### Горизонтальное масштабирование

Worker использует Kafka consumer groups для автоматического распределения нагрузки.

#### Docker Compose

```bash
# Масштабирование до 5 инстансов
docker-compose -f deployments/docker-compose.yml up -d --scale worker-1=5
```

#### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
spec:
  replicas: 5
  template:
    spec:
      containers:
      - name: worker
        image: smpp-server/worker:latest
```

### Вертикальное масштабирование

Увеличение concurrency для обработки большего количества сообщений параллельно:

```yaml
worker:
  concurrency: 20  # Увеличить с 10
```

### Рекомендации

- **Минимум:** 2 инстанса для высокой доступности
- **Оптимально:** 5-10 инстансов для средних нагрузок
- **Высокая нагрузка:** 20+ инстансов

### Метрики для масштабирования

- Размер очереди Kafka > 10000 сообщений
- Lag consumer group > 1000 сообщений
- CPU использование > 70%
- Память использование > 80%
- Processing duration p95 > 1s

## Масштабирование инфраструктуры

### PostgreSQL

#### Вертикальное масштабирование

Увеличение ресурсов:

```yaml
services:
  postgres:
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
```

#### Горизонтальное масштабирование

- **Read replicas** для чтения (если требуется)
- **Sharding** по датам (партиционирование уже реализовано)

#### Рекомендации

- Мониторить количество соединений
- Настроить connection pooling
- Использовать индексы для оптимизации запросов

### Kafka

#### Горизонтальное масштабирование

Добавление новых брокеров:

```yaml
services:
  kafka-1:
    # ...
  kafka-2:
    # ...
  kafka-3:
    # ...
```

#### Увеличение партиций

Увеличить количество партиций в топиках для лучшего распределения:

```bash
kafka-topics --alter --topic sms.outgoing --partitions 10 --bootstrap-server localhost:9092
```

#### Рекомендации

- Минимум 3 брокера для production
- Количество партиций = количество worker инстансов × 2
- Настроить replication factor = 3

### Redis

#### Вертикальное масштабирование

Увеличение памяти:

```yaml
services:
  redis:
    command: redis-server --maxmemory 2gb --maxmemory-policy allkeys-lru
```

#### Горизонтальное масштабирование

- **Redis Cluster** для распределения данных
- **Redis Sentinel** для высокой доступности

## Автоматическое масштабирование

### Kubernetes HPA (Horizontal Pod Autoscaler)

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api-gateway-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: api-gateway
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
```

### Docker Swarm

```bash
docker service scale smpp-api-gateway=5
```

## Мониторинг масштабирования

### Ключевые метрики

1. **API Gateway**
   - `http_requests_total` - количество запросов
   - `http_request_duration_seconds` - latency
   - CPU и память использование

2. **SMPP Server**
   - `smpp_connections_active` - активные соединения
   - `smpp_messages_received_total` - полученные сообщения
   - CPU и память использование

3. **Worker**
   - `smpp_queue_size` - размер очереди Kafka
   - `worker_messages_processed_total` - обработанные сообщения
   - Consumer lag
   - CPU и память использование

4. **Инфраструктура**
   - PostgreSQL connections
   - Kafka partition lag
   - Redis memory usage

### Алерты для масштабирования

Настроить алерты в Prometheus/Grafana:

- CPU использование > 70% в течение 5 минут
- Память использование > 80% в течение 5 минут
- Kafka lag > 1000 сообщений
- HTTP latency p95 > 500ms
- Очередь сообщений растет

## Best Practices

1. **Начинайте с малого**
   - Начните с минимальной конфигурации
   - Масштабируйте по мере необходимости

2. **Мониторьте метрики**
   - Используйте Prometheus и Grafana
   - Настройте алерты

3. **Тестируйте масштабирование**
   - Проводите нагрузочное тестирование
   - Проверяйте поведение при масштабировании

4. **Используйте graceful shutdown**
   - Все сервисы поддерживают graceful shutdown
   - Это важно при масштабировании вниз

5. **Балансируйте нагрузку**
   - Используйте HAProxy для API Gateway
   - Используйте Kafka consumer groups для Worker

6. **Оптимизируйте конфигурацию**
   - Настройте connection pools
   - Оптимизируйте concurrency
   - Настройте batch sizes

## Дополнительная документация

- [Docker Compose развертывание](docker-compose.md)
- [Конфигурация](configuration.md)
- [Мониторинг](monitoring.md)