# Мониторинг SMPP сервера

## Обзор

Система мониторинга включает:
- **Prometheus** - сбор и хранение метрик
- **Grafana** - визуализация метрик и дашборды
- **Health checks** - проверка здоровья сервисов

## Prometheus метрики

### API Gateway метрики

- `http_requests_total` - общее количество HTTP запросов (метки: method, path, status)
- `http_request_duration_seconds` - длительность HTTP запросов
- `grpc_requests_total` - общее количество gRPC запросов (метки: method, status)
- `grpc_request_duration_seconds` - длительность gRPC запросов
- `sms_messages_received_total` - полученные SMS сообщения (метки: source, client_id)
- `sms_messages_queued_total` - сообщения, добавленные в очередь (метки: client_id)
- `sms_messages_failed_total` - неудачные сообщения (метки: client_id, reason)
- `rate_limit_hits_total` - срабатывания rate limit (метки: client_id, period)

### SMPP Server метрики

- `smpp_messages_received_total` - полученные SMS через SMPP (метки: client_id, session_id)
- `smpp_connections_active` - активные SMPP соединения (метки: type: bound/unbound/total)
- `smpp_processing_duration_seconds` - длительность обработки (метки: operation)
- `smpp_messages_failed_total` - неудачные сообщения (метки: client_id, session_id, reason)

### Worker метрики

- `worker_messages_processed_total` - обработанные сообщения (метки: worker_id, status)
- `worker_processing_duration_seconds` - длительность обработки (метки: worker_id, operation)
- `smpp_messages_sent_total` - отправленные сообщения (метки: provider_id, provider_name, status)
- `smpp_messages_failed_total` - неудачные сообщения (метки: provider_id, provider_name, reason)
- `smpp_provider_throughput` - пропускная способность провайдера (метки: provider_id, provider_name)
- `smpp_processing_duration_seconds` - длительность отправки (метки: operation: send_to_provider)

### Kafka метрики

- `kafka_publish_duration_seconds` - длительность публикации (метки: topic)
- `kafka_publish_errors_total` - ошибки публикации (метки: topic)
- `smpp_queue_size` - размер очереди Kafka (метки: topic, consumer_group)

### Database метрики

- `database_connections_active` - активные соединения к БД
- `database_query_duration_seconds` - длительность запросов (метки: operation)

## Health Checks

Все сервисы предоставляют следующие endpoints:

- `GET /health` - полная проверка здоровья с проверкой зависимостей
- `GET /health/live` - liveness probe (простая проверка, что сервис жив)
- `GET /health/ready` - readiness probe (проверка готовности принимать трафик)

### Формат ответа health check

```json
{
  "status": "ok|degraded|error",
  "service": "api-gateway|smpp-server|worker",
  "version": "1.0.0",
  "timestamp": "2024-01-01T00:00:00Z",
  "checks": {
    "database": {
      "status": "ok",
      "response_time_ms": 5
    },
    "redis": {
      "status": "ok",
      "response_time_ms": 2
    },
    "kafka": {
      "status": "ok",
      "response_time_ms": 10
    }
  }
}
```

## Prometheus конфигурация

Конфигурация Prometheus находится в `deployments/configs/prometheus.yml`.

Сервисы для сбора метрик:
- `api-gateway-1:8080` - API Gateway instance 1
- `api-gateway-2:8080` - API Gateway instance 2
- `smpp-server:2112` - SMPP Server (metrics port)
- `worker-1:2112` - Worker instance 1 (metrics port)
- `worker-2:2112` - Worker instance 2 (metrics port)

## Grafana дашборды

Созданы следующие дашборды:

1. **Overview** (`overview.json`) - общий обзор системы
   - SMPP Messages Received/Sent/Failed
   - Active Connections
   - Processing Duration
   - Kafka Queue Size

2. **API Gateway** (`api-gateway.json`) - метрики API Gateway
   - HTTP/gRPC Requests Rate
   - Request Duration
   - SMS Messages Queued
   - Rate Limit Hits

3. **Worker** (`worker.json`) - метрики Worker
   - Messages Processed
   - Processing Duration
   - Provider Throughput
   - Messages Sent/Failed by Provider

4. **Provider Health** (`provider-health.json`) - здоровье провайдеров
   - Provider Throughput
   - Success/Error Rate
   - Processing Duration by Provider

### Импорт дашбордов в Grafana

1. Откройте Grafana (http://localhost:3000)
2. Войдите (admin/admin по умолчанию)
3. Перейдите в Dashboards → Import
4. Загрузите JSON файлы из `deployments/configs/grafana/dashboards/`

## Настройка мониторинга

### Переменные окружения

- `MONITORING_ENABLED=true` - включить мониторинг
- `MONITORING_PROMETHEUS_ENABLED=true` - включить Prometheus метрики
- `MONITORING_PROMETHEUS_PATH=/metrics` - путь к метрикам
- `MONITORING_METRICS_PORT=2112` - порт для метрик

### Docker Compose

Все сервисы уже настроены в `deployments/docker-compose.yml`:
- Prometheus доступен на порту 9091
- Grafana доступна на порту 3000
- Health checks настроены для всех сервисов

## Алерты (будущая функциональность)

Рекомендуется настроить алерты на:
- Высокий процент ошибок (> 5%)
- Высокую задержку обработки (p95 > 1s)
- Недоступность сервисов
- Переполнение очереди Kafka
- Недоступность провайдеров
