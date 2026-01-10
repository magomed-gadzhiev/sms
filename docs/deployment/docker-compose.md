# Docker Compose развертывание

## Обзор

Проект использует Docker Compose для оркестрации всех сервисов и инфраструктурных компонентов. Все сервисы запускаются в отдельных контейнерах и взаимодействуют через Docker network.

## Требования

- Docker Engine 20.10+
- Docker Compose 2.0+
- Минимум 4GB RAM
- Минимум 10GB свободного места на диске

## Структура Docker Compose

Файл `deployments/docker-compose.yml` содержит следующие сервисы:

### Сервисы приложения

- `api-gateway-1`, `api-gateway-2` - инстансы API Gateway
- `smpp-server` - SMPP сервер
- `worker-1`, `worker-2` - инстансы Worker

### Инфраструктурные сервисы

- `haproxy` - балансировщик нагрузки
- `kafka` - Apache Kafka
- `zookeeper` - Zookeeper для Kafka
- `postgres` - PostgreSQL база данных
- `redis` - Redis кэш
- `prometheus` - Prometheus мониторинг
- `grafana` - Grafana дашборды

### Разработка

- `dev` - контейнер для разработки

## Запуск

### Первый запуск

1. **Клонировать репозиторий**

```bash
git clone <repository-url>
cd smpp-server
```

2. **Создать конфигурационный файл**

```bash
cp configs/config.example.yaml configs/config.yaml
# Отредактировать configs/config.yaml под ваше окружение
```

3. **Запустить миграции БД**

```bash
docker-compose -f deployments/docker-compose.yml up -d postgres
# Подождать пока PostgreSQL запустится
docker-compose -f deployments/docker-compose.yml exec dev sh
# В контейнере:
go run cmd/migrate/main.go up
```

4. **Запустить все сервисы**

```bash
docker-compose -f deployments/docker-compose.yml up -d
```

5. **Проверить статус**

```bash
docker-compose -f deployments/docker-compose.yml ps
```

### Остановка

```bash
docker-compose -f deployments/docker-compose.yml down
```

### Остановка с удалением volumes

```bash
docker-compose -f deployments/docker-compose.yml down -v
```

**Внимание:** Это удалит все данные (БД, Kafka, Redis)!

## Порты

### Внешние порты (доступны с хоста)

- `8080` - HTTP API (через HAProxy)
- `9090` - gRPC API (через HAProxy)
- `2775` - SMPP протокол
- `8404` - HAProxy stats
- `3000` - Grafana
- `9091` - Prometheus
- `5432` - PostgreSQL
- `6379` - Redis
- `9092` - Kafka

### Внутренние порты (только в Docker network)

- `8081`, `8082` - API Gateway HTTP (внутренние)
- `9091`, `9092` - API Gateway gRPC (внутренние)
- `2112` - Metrics порт для всех сервисов

## Переменные окружения

Все сервисы настраиваются через переменные окружения. Основные переменные:

### API Gateway

```yaml
SERVICE_NAME: api-gateway-1
HTTP_PORT: 8080
GRPC_PORT: 9090
KAFKA_BROKERS: kafka:9092
POSTGRES_HOST: postgres
POSTGRES_PORT: 5432
POSTGRES_USER: smpp
POSTGRES_PASSWORD: smpp_password
POSTGRES_DB: smpp_db
REDIS_HOST: redis
REDIS_PORT: 6379
```

### SMPP Server

```yaml
SMPP_PORT: 2775
KAFKA_BROKERS: kafka:9092
POSTGRES_HOST: postgres
POSTGRES_PORT: 5432
POSTGRES_USER: smpp
POSTGRES_PASSWORD: smpp_password
POSTGRES_DB: smpp_db
```

### Worker

```yaml
SERVICE_NAME: worker-1
KAFKA_BROKERS: kafka:9092
KAFKA_CONSUMER_GROUP: worker-group
POSTGRES_HOST: postgres
POSTGRES_PORT: 5432
POSTGRES_USER: smpp
POSTGRES_PASSWORD: smpp_password
POSTGRES_DB: smpp_db
REDIS_HOST: redis
REDIS_PORT: 6379
```

## Volumes

Docker Compose создает следующие volumes для персистентности данных:

- `postgres-data` - данные PostgreSQL
- `redis-data` - данные Redis
- `kafka-data` - данные Kafka
- `zookeeper-data` - данные Zookeeper
- `prometheus-data` - данные Prometheus
- `grafana-data` - данные Grafana

## Health Checks

Все сервисы имеют настроенные health checks:

- **API Gateway:** `wget http://localhost:8080/health`
- **SMPP Server:** `wget http://localhost:2112/health`
- **Worker:** `wget http://localhost:2112/health`
- **PostgreSQL:** `pg_isready -U smpp`
- **Redis:** `redis-cli ping`
- **Kafka:** `kafka-broker-api-versions --bootstrap-server localhost:9092`

## Масштабирование сервисов

### Масштабирование API Gateway

```bash
docker-compose -f deployments/docker-compose.yml up -d --scale api-gateway-1=3
```

### Масштабирование Worker

```bash
docker-compose -f deployments/docker-compose.yml up -d --scale worker-1=5
```

**Примечание:** HAProxy автоматически обнаружит новые инстансы через health checks.

## Логи

### Просмотр логов всех сервисов

```bash
docker-compose -f deployments/docker-compose.yml logs -f
```

### Просмотр логов конкретного сервиса

```bash
docker-compose -f deployments/docker-compose.yml logs -f api-gateway-1
docker-compose -f deployments/docker-compose.yml logs -f worker-1
docker-compose -f deployments/docker-compose.yml logs -f smpp-server
```

### Просмотр последних 100 строк

```bash
docker-compose -f deployments/docker-compose.yml logs --tail=100 api-gateway-1
```

## Пересборка образов

### Пересборка всех образов

```bash
docker-compose -f deployments/docker-compose.yml build
```

### Пересборка конкретного сервиса

```bash
docker-compose -f deployments/docker-compose.yml build api-gateway-1
```

### Пересборка и перезапуск

```bash
docker-compose -f deployments/docker-compose.yml up -d --build api-gateway-1
```

## Обновление сервисов

1. **Остановить сервис**

```bash
docker-compose -f deployments/docker-compose.yml stop api-gateway-1
```

2. **Пересобрать образ**

```bash
docker-compose -f deployments/docker-compose.yml build api-gateway-1
```

3. **Запустить сервис**

```bash
docker-compose -f deployments/docker-compose.yml up -d api-gateway-1
```

## Доступ к сервисам

### Доступ к PostgreSQL

```bash
docker-compose -f deployments/docker-compose.yml exec postgres psql -U smpp -d smpp_db
```

### Доступ к Redis

```bash
docker-compose -f deployments/docker-compose.yml exec redis redis-cli
```

### Доступ к Kafka

```bash
docker-compose -f deployments/docker-compose.yml exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic sms.outgoing --from-beginning
```

### Доступ к dev контейнеру

```bash
docker-compose -f deployments/docker-compose.yml exec dev sh
```

## Мониторинг

### Prometheus

Доступен по адресу: http://localhost:9091

### Grafana

Доступна по адресу: http://localhost:3000
- Логин: `admin`
- Пароль: `admin`

### HAProxy Stats

Доступны по адресу: http://localhost:8404/stats

## Troubleshooting

### Сервис не запускается

1. Проверить логи:
```bash
docker-compose -f deployments/docker-compose.yml logs <service-name>
```

2. Проверить health check:
```bash
docker-compose -f deployments/docker-compose.yml ps
```

3. Проверить зависимости:
```bash
docker-compose -f deployments/docker-compose.yml ps
# Убедиться что все зависимости запущены
```

### Проблемы с сетью

1. Проверить сеть:
```bash
docker network ls
docker network inspect smpp-server_smpp-network
```

2. Пересоздать сеть:
```bash
docker-compose -f deployments/docker-compose.yml down
docker-compose -f deployments/docker-compose.yml up -d
```

### Проблемы с volumes

1. Проверить volumes:
```bash
docker volume ls
docker volume inspect <volume-name>
```

2. Очистить volumes (удалит данные!):
```bash
docker-compose -f deployments/docker-compose.yml down -v
```

## Production рекомендации

Для production окружения рекомендуется:

1. **Использовать внешние сервисы:**
   - Внешний PostgreSQL (managed database)
   - Внешний Kafka (managed Kafka)
   - Внешний Redis (managed Redis)

2. **Настроить SSL/TLS:**
   - SSL сертификаты для HAProxy
   - SSL для PostgreSQL соединений
   - SSL для Kafka

3. **Настроить резервное копирование:**
   - Автоматические бэкапы PostgreSQL
   - Бэкапы конфигурации

4. **Мониторинг и алерты:**
   - Настроить алерты в Prometheus/Grafana
   - Интеграция с системами уведомлений

5. **Логирование:**
   - Централизованное логирование (ELK, Loki)
   - Ротация логов

6. **Безопасность:**
   - Использовать секреты (Docker secrets, Vault)
   - Ограничить доступ к портам
   - Настроить firewall правила

## Дополнительная документация

- [Конфигурация](configuration.md)
- [Масштабирование](scaling.md)
- [Мониторинг](monitoring.md)