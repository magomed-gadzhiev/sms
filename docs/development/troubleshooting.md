# Решение проблем

## Обзор

Этот документ описывает типичные проблемы и способы их решения при работе с SMPP сервером.

## Общие проблемы

### Сервис не запускается

#### Проблема: Ошибка подключения к базе данных

**Симптомы:**
```
failed to connect to database: connection refused
```

**Решение:**
1. Проверить, что PostgreSQL запущен:
```bash
docker-compose -f deployments/docker-compose.yml ps postgres
```

2. Проверить переменные окружения:
```bash
docker-compose -f deployments/docker-compose.yml exec api-gateway-1 env | grep POSTGRES
```

3. Проверить логи PostgreSQL:
```bash
docker-compose -f deployments/docker-compose.yml logs postgres
```

4. Убедиться, что PostgreSQL готов к подключениям:
```bash
docker-compose -f deployments/docker-compose.yml exec postgres pg_isready -U smpp
```

#### Проблема: Ошибка подключения к Kafka

**Симптомы:**
```
failed to connect to kafka: dial tcp: lookup kafka
```

**Решение:**
1. Проверить, что Kafka запущен:
```bash
docker-compose -f deployments/docker-compose.yml ps kafka
```

2. Проверить логи Kafka:
```bash
docker-compose -f deployments/docker-compose.yml logs kafka
```

3. Убедиться, что Zookeeper запущен (Kafka зависит от него):
```bash
docker-compose -f deployments/docker-compose.yml ps zookeeper
```

4. Проверить переменные окружения:
```bash
docker-compose -f deployments/docker-compose.yml exec worker-1 env | grep KAFKA
```

#### Проблема: Ошибка подключения к Redis

**Симптомы:**
```
failed to connect to redis: connection refused
```

**Решение:**
1. Проверить, что Redis запущен:
```bash
docker-compose -f deployments/docker-compose.yml ps redis
```

2. Проверить логи Redis:
```bash
docker-compose -f deployments/docker-compose.yml logs redis
```

3. Проверить переменные окружения:
```bash
docker-compose -f deployments/docker-compose.yml exec api-gateway-1 env | grep REDIS
```

### Проблемы с миграциями

#### Проблема: Миграции не применяются

**Симптомы:**
```
relation "messages" does not exist
```

**Решение:**
1. Проверить, что миграции существуют:
```bash
ls migrations/
```

2. Применить миграции вручную:
```bash
docker-compose -f deployments/docker-compose.yml exec dev sh
# В контейнере
migrate -path migrations -database "postgres://smpp:${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set}@postgres:5432/smpp_db?sslmode=disable" up
```

3. Проверить статус миграций:
```bash
migrate -path migrations -database "postgres://smpp:${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set}@postgres:5432/smpp_db?sslmode=disable" version
```

## Проблемы API Gateway

### Проблема: 401 Unauthorized

**Симптомы:**
```
HTTP 401 Unauthorized при запросах к API
```

**Решение:**
1. Проверить наличие API ключа в заголовке:
```bash
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/sms/send
```

2. Проверить, что клиент существует в БД:
```bash
docker-compose -f deployments/docker-compose.yml exec postgres psql -U smpp -d smpp_db -c "SELECT * FROM clients;"
```

3. Проверить логи API Gateway:
```bash
docker-compose -f deployments/docker-compose.yml logs api-gateway-1 | grep auth
```

### Проблема: 429 Too Many Requests

**Симптомы:**
```
HTTP 429 Too Many Requests
```

**Решение:**
1. Проверить настройки rate limit для клиента:
```bash
docker-compose -f deployments/docker-compose.yml exec postgres psql -U smpp -d smpp_db -c "SELECT rate_limit_per_second FROM clients WHERE api_key = 'your-api-key';"
```

2. Увеличить rate limit при необходимости:
```sql
UPDATE clients SET rate_limit_per_second = 100 WHERE api_key = 'your-api-key';
```

3. Проверить логи rate limiting:
```bash
docker-compose -f deployments/docker-compose.yml logs api-gateway-1 | grep rate_limit
```

### Проблема: Сообщения не публикуются в Kafka

**Симптомы:**
```
Сообщения создаются, но не обрабатываются Worker
```

**Решение:**
1. Проверить логи публикации:
```bash
docker-compose -f deployments/docker-compose.yml logs api-gateway-1 | grep kafka
```

2. Проверить, что сообщения попадают в Kafka:
```bash
docker-compose -f deployments/docker-compose.yml exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic sms.outgoing --from-beginning --max-messages 10
```

3. Проверить конфигурацию Kafka:
```bash
docker-compose -f deployments/docker-compose.yml exec api-gateway-1 env | grep KAFKA
```

## Проблемы SMPP Server

### Проблема: Клиент не может подключиться

**Симптомы:**
```
SMPP клиент не может установить соединение
```

**Решение:**
1. Проверить, что SMPP Server запущен:
```bash
docker-compose -f deployments/docker-compose.yml ps smpp-server
```

2. Проверить, что порт открыт:
```bash
netstat -an | grep 2775
# или
docker-compose -f deployments/docker-compose.yml port smpp-server 2775
```

3. Проверить логи SMPP Server:
```bash
docker-compose -f deployments/docker-compose.yml logs smpp-server
```

4. Проверить firewall правила (если применимо)

### Проблема: Сессия разрывается

**Симптомы:**
```
SMPP соединение разрывается через некоторое время
```

**Решение:**
1. Проверить настройки enquire_link:
```yaml
smpp:
  enquire_link_period: 60s
```

2. Проверить таймауты:
```yaml
smpp:
  read_timeout: 30s
  write_timeout: 30s
```

3. Проверить логи на ошибки:
```bash
docker-compose -f deployments/docker-compose.yml logs smpp-server | grep error
```

## Проблемы Worker

### Проблема: Сообщения не обрабатываются

**Симптомы:**
```
Сообщения в Kafka, но Worker их не обрабатывает
```

**Решение:**
1. Проверить, что Worker запущен:
```bash
docker-compose -f deployments/docker-compose.yml ps worker-1
```

2. Проверить логи Worker:
```bash
docker-compose -f deployments/docker-compose.yml logs worker-1
```

3. Проверить consumer group lag:
```bash
docker-compose -f deployments/docker-compose.yml exec kafka kafka-consumer-groups --bootstrap-server localhost:9092 --group worker-group --describe
```

4. Проверить конфигурацию consumer group:
```bash
docker-compose -f deployments/docker-compose.yml exec worker-1 env | grep KAFKA_CONSUMER_GROUP
```

### Проблема: Ошибки отправки в SMSC

**Симптомы:**
```
Сообщения не доставляются в SMSC провайдеры
```

**Решение:**
1. Проверить логи Worker на ошибки:
```bash
docker-compose -f deployments/docker-compose.yml logs worker-1 | grep error
```

2. Проверить соединения с провайдерами:
```bash
docker-compose -f deployments/docker-compose.yml exec worker-1 env | grep PROVIDER
```

3. Проверить конфигурацию провайдеров в БД:
```bash
docker-compose -f deployments/docker-compose.yml exec postgres psql -U smpp -d smpp_db -c "SELECT * FROM providers;"
```

4. Проверить health checks провайдеров:
```bash
# Проверить метрики
curl http://localhost:2113/metrics | grep provider
```

### Проблема: Высокий lag в Kafka

**Симптомы:**
```
Consumer lag растет, сообщения накапливаются
```

**Решение:**
1. Увеличить количество Worker инстансов:
```bash
docker-compose -f deployments/docker-compose.yml up -d --scale worker-1=5
```

2. Увеличить concurrency:
```yaml
worker:
  concurrency: 20
```

3. Увеличить количество партиций в топике:
```bash
docker-compose -f deployments/docker-compose.yml exec kafka kafka-topics --alter --topic sms.outgoing --partitions 10 --bootstrap-server localhost:9092
```

## Проблемы с базой данных

### Проблема: Медленные запросы

**Симптомы:**
```
Запросы к БД выполняются медленно
```

**Решение:**
1. Проверить индексы:
```sql
SELECT * FROM pg_indexes WHERE tablename = 'messages';
```

2. Проверить статистику запросов:
```sql
SELECT * FROM pg_stat_statements ORDER BY total_time DESC LIMIT 10;
```

3. Оптимизировать запросы (EXPLAIN ANALYZE):
```sql
EXPLAIN ANALYZE SELECT * FROM messages WHERE client_id = '...';
```

4. Увеличить connection pool:
```yaml
database:
  max_open_conns: 50
  max_idle_conns: 10
```

### Проблема: Нехватка соединений

**Симптомы:**
```
too many connections
```

**Решение:**
1. Проверить текущие соединения:
```sql
SELECT count(*) FROM pg_stat_activity;
```

2. Уменьшить max_open_conns в конфигурации
3. Проверить connection pooling настройки

## Проблемы с мониторингом

### Проблема: Метрики не собираются

**Симптомы:**
```
Prometheus не видит метрики сервисов
```

**Решение:**
1. Проверить, что Prometheus запущен:
```bash
docker-compose -f deployments/docker-compose.yml ps prometheus
```

2. Проверить конфигурацию Prometheus:
```bash
docker-compose -f deployments/docker-compose.yml exec prometheus cat /etc/prometheus/prometheus.yml
```

3. Проверить, что метрики доступны:
```bash
curl http://localhost:8080/metrics
curl http://localhost:2112/metrics
```

4. Проверить targets в Prometheus:
```
http://localhost:9091/targets
```

### Проблема: Grafana не показывает данные

**Решение:**
1. Проверить, что Prometheus является источником данных в Grafana
2. Проверить запросы в дашбордах
3. Проверить временной диапазон

## Отладка

### Включение debug логирования

Установить уровень логирования:

```bash
export LOG_LEVEL=debug
```

Или в конфигурации:

```yaml
service:
  log_level: debug
```

### Просмотр логов в реальном времени

```bash
docker-compose -f deployments/docker-compose.yml logs -f api-gateway-1
```

### Проверка метрик

```bash
# API Gateway
curl http://localhost:8080/metrics

# SMPP Server
curl http://localhost:2112/metrics

# Worker
curl http://localhost:2113/metrics
```

### Проверка health checks

```bash
# API Gateway
curl http://localhost:8080/health

# SMPP Server
curl http://localhost:2112/health

# Worker
curl http://localhost:2113/health
```

## Получение помощи

Если проблема не решена:

1. Проверьте логи всех связанных сервисов
2. Соберите информацию о проблеме:
   - Версия системы
   - Конфигурация
   - Логи
   - Метрики
3. Создайте issue в репозитории с описанием проблемы

## Дополнительная документация

- [Docker Compose развертывание](../deployment/docker-compose.md)
- [Конфигурация](../deployment/configuration.md)
- [Мониторинг](../deployment/monitoring.md)