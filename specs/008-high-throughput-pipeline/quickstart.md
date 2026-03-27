# Quick Start: High-Throughput Pipeline

**Feature**: 008-high-throughput-pipeline
**Date**: 2026-03-26

## Prerequisites

- Go 1.24.0+
- Docker & Docker Compose
- Работающий стек: PostgreSQL, Redis, Kafka, Zookeeper (через docker-compose)

## 1. Создание Kafka-топиков с правильными партициями

```bash
# Из корня проекта
docker-compose -f deployments/docker-compose.yml exec kafka bash -c "
  kafka-topics --create --topic sms.routed --partitions 16 --replication-factor 1 --bootstrap-server localhost:9092 &&
  kafka-topics --create --topic sms.sent --partitions 16 --replication-factor 1 --bootstrap-server localhost:9092 &&
  kafka-topics --create --topic sms.status --partitions 8 --replication-factor 1 --bootstrap-server localhost:9092 &&
  kafka-topics --alter --topic sms.outgoing --partitions 16 --bootstrap-server localhost:9092 &&
  kafka-topics --alter --topic sms.dlr --partitions 8 --bootstrap-server localhost:9092
"
```

## 2. Сборка pipeline-worker

```bash
go build -o bin/pipeline-worker ./cmd/pipeline-worker/
```

## 3. Запуск стадий pipeline

Каждая стадия запускается как отдельный процесс:

```bash
# Terminal 1: Router stage (1 instance)
./bin/pipeline-worker --stage=router

# Terminal 2-5: Sender stage (4 instances для 10K msg/sec)
./bin/pipeline-worker --stage=sender --worker-id=sender-1
./bin/pipeline-worker --stage=sender --worker-id=sender-2
./bin/pipeline-worker --stage=sender --worker-id=sender-3
./bin/pipeline-worker --stage=sender --worker-id=sender-4

# Terminal 6: Status writer (1 instance)
./bin/pipeline-worker --stage=status
```

## 4. Запуск через Docker Compose

```bash
# Поднять весь стек включая pipeline-worker
docker-compose -f deployments/docker-compose.yml up -d

# Масштабировать sender stage
docker-compose -f deployments/docker-compose.yml up -d --scale pipeline-sender=4
```

## 5. Проверка работоспособности

```bash
# Health check каждой стадии
curl http://localhost:2130/health  # router
curl http://localhost:2131/health  # sender
curl http://localhost:2132/health  # status writer

# Метрики
curl http://localhost:2130/metrics | grep pipeline_
```

## 6. Запуск load test

```bash
# 10K msg/sec load test
cd test/load
go test -v -run TestHighThroughputPipeline -count=1 -timeout=10m
```

## 7. Мониторинг

- Grafana dashboard: http://localhost:3001 → "High-Throughput Pipeline"
- Prometheus: http://localhost:9091 → query `pipeline_messages_processed_total`
- Consumer lag: `pipeline_queue_depth{topic="sms.routed"}`

## Конфигурация (environment variables)

```bash
# Pipeline stage
PIPELINE_STAGE=router          # router | sender | status

# Batch processing
PIPELINE_BATCH_SIZE=500        # messages per batch
PIPELINE_BATCH_TIMEOUT=10ms    # max wait for batch fill
PIPELINE_WORKER_COUNT=4        # goroutines per stage

# Kafka
KAFKA_BROKERS=localhost:9092
KAFKA_TOPIC_ROUTED=sms.routed
KAFKA_TOPIC_SENT=sms.sent
KAFKA_TOPIC_STATUS=sms.status

# SMPP (sender stage only)
SMPP_WINDOW_SIZE=50            # async PDU window per connection
SMPP_CONNECT_TIMEOUT=10s

# Backpressure (sender stage only)
BACKPRESSURE_BURST_MULTIPLIER=2  # burst = throughput_per_second × N
```

## Rollback

Если pipeline-worker не работает корректно, вернуться к старому worker:

```bash
# Остановить pipeline-worker instances
docker-compose -f deployments/docker-compose.yml stop pipeline-router pipeline-sender pipeline-status

# Запустить старый worker
docker-compose -f deployments/docker-compose.yml up -d worker
```

Старый worker (`cmd/worker/`) сохраняется в кодовой базе для rollback.
