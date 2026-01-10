# Конфигурация системы

## Обзор

Система использует конфигурационные файлы в формате YAML и переменные окружения для настройки всех компонентов.

## Конфигурационный файл

Основной конфигурационный файл: `configs/config.yaml`

Пример конфигурации: `configs/config.example.yaml`

## Структура конфигурации

### Service

```yaml
service:
  name: smpp-server          # Имя сервиса
  version: 1.0.0             # Версия
  env: development           # Окружение: development, staging, production
```

### Database (PostgreSQL)

```yaml
database:
  host: localhost            # Хост БД
  port: 5432                 # Порт БД
  user: smpp                 # Пользователь
  password: smpp_password     # Пароль
  database: smpp_db          # Имя БД
  ssl_mode: disable          # SSL режим: disable, require, verify-ca, verify-full
  max_open_conns: 25         # Максимум открытых соединений
  max_idle_conns: 5          # Максимум idle соединений
  conn_max_lifetime: 5m      # Максимальное время жизни соединения
  conn_max_idle_time: 10m    # Максимальное время idle соединения
```

### Redis

```yaml
redis:
  host: localhost            # Хост Redis
  port: 6379                 # Порт Redis
  password: ""                # Пароль (пусто для отсутствия пароля)
  db: 0                       # Номер БД
  pool_size: 10               # Размер пула соединений
  min_idle_conns: 5          # Минимум idle соединений
  dial_timeout: 5s            # Таймаут подключения
  read_timeout: 3s            # Таймаут чтения
  write_timeout: 3s           # Таймаут записи
```

### Kafka

```yaml
kafka:
  brokers:                   # Список брокеров
    - localhost:9092
  topic_outgoing: sms.outgoing    # Топик для исходящих сообщений
  topic_dlr: sms.dlr              # Топик для DLR
  topic_failed: sms.failed        # Топик для failed сообщений
  consumer_group: smpp-worker     # Consumer group для Worker
  session_timeout: 30s            # Таймаут сессии
  heartbeat_interval: 10s         # Интервал heartbeat
  max_retries: 3                  # Максимум попыток retry
  retry_backoff: 1s               # Задержка между retry
```

### SMPP Server

```yaml
smpp:
  host: 0.0.0.0              # Хост для прослушивания
  port: 2775                 # Порт SMPP
  read_timeout: 30s          # Таймаут чтения
  write_timeout: 30s         # Таймаут записи
  enquire_link_period: 60s   # Период enquire_link
  max_connections: 1000      # Максимум соединений
  rate_limit_per_sec: 100    # Rate limit в секунду
```

### API Gateway

```yaml
api:
  http:
    host: 0.0.0.0            # Хост HTTP сервера
    port: 8080               # Порт HTTP сервера
    read_timeout: 10s        # Таймаут чтения запроса
    write_timeout: 10s       # Таймаут записи ответа
    idle_timeout: 120s       # Таймаут idle соединения
  grpc:
    host: 0.0.0.0            # Хост gRPC сервера
    port: 9090               # Порт gRPC сервера
    max_recv: 4194304        # Максимальный размер получаемого сообщения (4MB)
    max_send: 4194304        # Максимальный размер отправляемого сообщения (4MB)
  auth:
    api_key_header: X-API-Key    # Заголовок для API ключа
    jwt_secret: ""                # Секрет для JWT (если используется)
    token_expiry: 24h             # Время жизни токена
```

### Worker

```yaml
worker:
  concurrency: 10            # Количество горутин для обработки
  max_retries: 5             # Максимум попыток retry
  retry_backoff_base: 1s     # Базовая задержка для retry
  retry_backoff_max: 60s     # Максимальная задержка для retry
  batch_size: 100            # Размер батча для обработки
  batch_timeout: 5s          # Таймаут для батча
  health_check_period: 30s   # Период проверки здоровья соединений
```

### Monitoring

```yaml
monitoring:
  enabled: true              # Включить мониторинг
  prometheus:
    enabled: true            # Включить Prometheus метрики
    path: /metrics           # Путь к метрикам
  metrics_port: 2112         # Порт для метрик
```

## Переменные окружения

Все параметры конфигурации могут быть переопределены через переменные окружения с префиксом `SMPP_`.

Формат: `SMPP_<SECTION>_<SUBSECTION>_<PARAMETER>`

Примеры:

```bash
SMPP_DATABASE_HOST=postgres
SMPP_DATABASE_PORT=5432
SMPP_DATABASE_USER=smpp
SMPP_DATABASE_PASSWORD=smpp_password
SMPP_DATABASE_DATABASE=smpp_db

SMPP_KAFKA_BROKERS=localhost:9092
SMPP_KAFKA_TOPIC_OUTGOING=sms.outgoing

SMPP_API_HTTP_PORT=8080
SMPP_API_GRPC_PORT=9090

SMPP_WORKER_CONCURRENCY=10
SMPP_WORKER_MAX_RETRIES=5
```

## Приоритет конфигурации

1. **Переменные окружения** (наивысший приоритет)
2. **Конфигурационный файл**
3. **Значения по умолчанию** (низший приоритет)

## Специфичные настройки для сервисов

### API Gateway

Дополнительные переменные окружения:

```bash
SERVICE_NAME=api-gateway-1
HTTP_PORT=8080
GRPC_PORT=9090
```

### SMPP Server

Дополнительные переменные окружения:

```bash
SMPP_PORT=2775
```

### Worker

Дополнительные переменные окружения:

```bash
SERVICE_NAME=worker-1
KAFKA_CONSUMER_GROUP=worker-group
```

## Конфигурация для разных окружений

### Development

```yaml
service:
  env: development

database:
  ssl_mode: disable

monitoring:
  enabled: true
```

### Staging

```yaml
service:
  env: staging

database:
  ssl_mode: require

monitoring:
  enabled: true
```

### Production

```yaml
service:
  env: production

database:
  ssl_mode: verify-full
  max_open_conns: 50
  max_idle_conns: 10

monitoring:
  enabled: true
```

## Валидация конфигурации

Система автоматически валидирует конфигурацию при загрузке:

- Обязательные поля должны быть заполнены
- Порты должны быть в диапазоне 1-65535
- Таймауты должны быть положительными значениями
- Списки брокеров не должны быть пустыми

При ошибке валидации сервис не запустится и выведет ошибку.

## Hot Reload

В текущей версии hot reload конфигурации не поддерживается. Для применения изменений необходимо перезапустить сервис.

## Секреты

**Важно:** Не храните секреты (пароли, API ключи) в конфигурационных файлах, которые попадают в систему контроля версий!

Используйте переменные окружения для секретов:

```bash
SMPP_DATABASE_PASSWORD=secret_password
SMPP_REDIS_PASSWORD=secret_redis_password
```

В production используйте системы управления секретами:
- Docker Secrets
- HashiCorp Vault
- Kubernetes Secrets
- AWS Secrets Manager

## Примеры конфигурации

### Минимальная конфигурация

```yaml
database:
  host: localhost
  port: 5432
  user: smpp
  password: smpp_password
  database: smpp_db

kafka:
  brokers:
    - localhost:9092
  topic_outgoing: sms.outgoing
```

Все остальные параметры будут использовать значения по умолчанию.

### Production конфигурация

```yaml
service:
  name: smpp-server
  version: 1.0.0
  env: production

database:
  host: postgres.example.com
  port: 5432
  user: smpp
  password: ${DB_PASSWORD}  # Из переменной окружения
  database: smpp_db
  ssl_mode: verify-full
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: 15m
  conn_max_idle_time: 10m

redis:
  host: redis.example.com
  port: 6379
  password: ${REDIS_PASSWORD}
  pool_size: 20
  min_idle_conns: 10

kafka:
  brokers:
    - kafka1.example.com:9092
    - kafka2.example.com:9092
    - kafka3.example.com:9092
  topic_outgoing: sms.outgoing
  topic_dlr: sms.dlr
  topic_failed: sms.failed
  consumer_group: smpp-worker
  session_timeout: 30s
  heartbeat_interval: 10s
  max_retries: 5
  retry_backoff: 2s

api:
  http:
    host: 0.0.0.0
    port: 8080
    read_timeout: 30s
    write_timeout: 30s
    idle_timeout: 120s
  grpc:
    host: 0.0.0.0
    port: 9090
    max_recv: 8388608  # 8MB
    max_send: 8388608  # 8MB

worker:
  concurrency: 20
  max_retries: 5
  retry_backoff_base: 2s
  retry_backoff_max: 120s
  batch_size: 200
  batch_timeout: 10s
  health_check_period: 30s

monitoring:
  enabled: true
  prometheus:
    enabled: true
    path: /metrics
  metrics_port: 2112
```

## Дополнительная документация

- [Docker Compose развертывание](docker-compose.md)
- [Масштабирование](scaling.md)
- [Мониторинг](monitoring.md)