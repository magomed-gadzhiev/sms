# Implementation Plan: High-Throughput Pipeline Architecture

**Branch**: `008-high-throughput-pipeline` | **Date**: 2026-03-26 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/008-high-throughput-pipeline/spec.md`

## Summary

Многоступенчатый пайплайн для увеличения пропускной способности SMS-платформы до 10 000+ msg/sec. Текущий монолитный worker (routing + SMPP sending + DLR) заменяется на pipeline-worker — единый бинарник с конфигурируемым режимом стадии. Каждая стадия масштабируется независимо через Kafka consumer groups. Добавляются новые Kafka-топики между стадиями, batch API для клиентов, per-stage Prometheus-метрики и backpressure-механизм на уровне оператора.

## Technical Context

**Language/Version**: Go 1.24.0
**Primary Dependencies**: gorilla/mux (HTTP), google.golang.org/grpc v1.78.0 (gRPC), IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), spf13/viper (config), prometheus/client_golang (metrics), stretchr/testify (testing)
**Storage**: PostgreSQL 15+ (pgx, monthly partitioning для messages/audit_log), Redis 7+ (rate-limiting, cache), Apache Kafka (inter-stage messaging)
**Testing**: go test + testify (unit/integration), k6 + Go load tests (performance)
**Target Platform**: Linux server (Docker Compose, K8s-ready)
**Project Type**: Distributed microservice platform (backend pipeline refactoring)
**Performance Goals**: 10 000+ msg/sec end-to-end при 4 экземплярах sender-стадии; <5ms p99 API response time на приёме
**Constraints**: At-least-once delivery (no message loss), eventual consistency для статусов (≤30s), backpressure per-operator
**Scale/Scope**: Замена 1 сервиса (worker), добавление 3 Kafka-топиков, batch API endpoint, per-stage метрики; ~12 существующих микросервисов без изменений

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Принцип | Статус | Проверка |
|---------|--------|----------|
| I. Domain-Driven Design | ✅ PASS | Pipeline-стадии остаются в bounded context messaging/sending. Единый бинарник pipeline-worker, не новый сервис. gRPC не требуется между стадиями — используется Kafka |
| II. Event-Driven Architecture | ✅ PASS | Новые Kafka-топики (sms.routed, sms.sent, sms.status) между стадиями. Consumers идемпотентны. Схемы версионируются |
| III. Contract-First APIs | ✅ PASS | Batch API (HTTP/gRPC) и Kafka message schemas определяются до реализации в contracts/ |
| IV. Observability | ✅ PASS | Per-stage метрики (throughput, latency, queue depth) через Prometheus. Structured logging zerolog. request_id propagation |
| V. Data Safety | ✅ PASS | At-least-once через Kafka. Идемпотентная запись статусов (upsert по message_id). Партиционирование сохраняется |
| VI. Simplicity | ⚠️ JUSTIFIED | Разбиение worker на стадии: единый бинарник с `--stage` флагом вместо отдельных сервисов. См. Complexity Tracking |

## Project Structure

### Documentation (this feature)

```text
specs/008-high-throughput-pipeline/
├── plan.md              # This file
├── research.md          # Phase 0: research findings
├── data-model.md        # Phase 1: data model changes
├── quickstart.md        # Phase 1: quick start guide
├── contracts/           # Phase 1: API & message contracts
│   ├── batch-api.md     # Batch SMS API contract
│   └── kafka-schemas.md # Inter-stage Kafka message schemas
└── tasks.md             # Phase 2 output (NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
cmd/
├── pipeline-worker/        # NEW: единый бинарник pipeline-worker
│   └── main.go             # --stage=router|sender|status флаг
├── worker/                 # DEPRECATED: старый монолитный worker (сохраняется для rollback)
└── client-gateway/         # MODIFIED: добавление batch endpoint

internal/
├── pipeline/               # NEW: pipeline-специфичная логика
│   ├── router/             # Stage: маршрутизация сообщений
│   │   └── stage.go
│   ├── sender/             # Stage: SMPP отправка (batch, async)
│   │   └── stage.go
│   ├── status/             # Stage: запись статусов
│   │   └── stage.go
│   ├── backpressure/       # Backpressure manager (per-operator)
│   │   └── manager.go
│   └── batch/              # Batch processing utilities
│       └── batcher.go
├── queue/                  # MODIFIED: новые топики, batch consumer
│   ├── consumer.go
│   └── producer.go
├── smsc/                   # MODIFIED: async sending, connection pool optimization
│   ├── pool.go
│   └── sender.go
├── router/                 # REUSED: существующая логика маршрутизации
├── api/                    # MODIFIED: batch endpoint
│   └── http/
│       └── handlers.go
└── monitoring/             # MODIFIED: per-stage метрики
    └── metrics.go

deployments/
├── docker-compose.yml      # MODIFIED: pipeline-worker вместо worker
└── configs/
    └── prometheus.yml      # MODIFIED: scrape pipeline-worker instances

test/
└── load/                   # MODIFIED: pipeline-specific load tests
```

**Structure Decision**: Единый pipeline-worker бинарник (`cmd/pipeline-worker/`) с конфигурируемой стадией через `--stage` флаг. Основная логика стадий в `internal/pipeline/`. Переиспользуются существующие пакеты: `internal/router/`, `internal/smsc/`, `internal/queue/`. Старый `cmd/worker/` сохраняется для rollback.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Разбиение worker на 3 pipeline-стадии | Bottleneck в SMPP sending (I/O bound) при routing (CPU bound). Масштабирование монолитного worker тратит ресурсы на routing при нехватке sender capacity | Горизонтальное масштабирование монолитного worker: масштабирует все стадии одинаково, но bottleneck в sender требует ×4 sender при ×1 router. Единый бинарник с `--stage` минимизирует сложность деплоя |
| 3 новых Kafka-топика (sms.routed, sms.sent, sms.status) | Межстадийная коммуникация требует персистентных очередей для at-least-once гарантий и независимого масштабирования consumer groups | Прямые вызовы между стадиями: теряется независимость масштабирования и fault tolerance. In-memory каналы: теряется персистентность при crash |
