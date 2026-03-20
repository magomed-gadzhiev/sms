# Implementation Plan: SMS Gateway Platform

**Branch**: `001-sms-gateway-platform` | **Date**: 2026-03-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-sms-gateway-platform/spec.md`

## Summary

Платформа SMS-шлюза на базе микросервисной архитектуры для приёма, маршрутизации и доставки SMS-сообщений через SMSC-провайдеры. Большая часть функциональности уже реализована. Основные доработки: поддержка multipart SMS (UDH), механизм истечения DLR, автоматическая очистка данных и посегментная тарификация.

## Technical Context

**Language/Version**: Go 1.24.0
**Primary Dependencies**: gorilla/mux (HTTP), google.golang.org/grpc v1.78.0 (gRPC), IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), spf13/viper (config), golang-jwt/jwt/v5 (auth), prometheus/client_golang (metrics), stretchr/testify (testing)
**Storage**: PostgreSQL 15 (partitioned tables), Redis 7 (cache/rate limiting), Apache Kafka (event streaming)
**Testing**: testify v1.11.1, build tags для integration tests
**Target Platform**: Linux server (Docker/Docker Compose)
**Project Type**: Microservices platform (12 services + 3 gateways)
**Performance Goals**: <1s submission latency, 10k concurrent clients, 10k batch size
**Constraints**: 99.95% uptime, 30-day scheduling horizon, 90-day data retention
**Scale/Scope**: 10,000 concurrent clients, multipart SMS up to 10 segments

## Constitution Check

*Конституция проекта не настроена (шаблон). Проверка пропущена.*

## Project Structure

### Documentation (this feature)

```text
specs/001-sms-gateway-platform/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0: research findings
├── data-model.md        # Phase 1: data model design
├── quickstart.md        # Phase 1: quickstart guide
├── contracts/           # Phase 1: API contracts
│   ├── client-api.md    # Client HTTP/gRPC API
│   ├── admin-api.md     # Admin HTTP/gRPC API
│   └── smpp-protocol.md # SMPP v3.4 protocol
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/
├── client-gateway/          # HTTP + gRPC client API gateway
├── admin-gateway/           # HTTP + gRPC admin API gateway
├── smpp-gateway/            # SMPP v3.4 protocol gateway
├── worker/                  # Kafka consumer → SMSC delivery
└── services/
    ├── auth-service/        # JWT/API key authentication
    ├── messaging-service/   # Message lifecycle + scheduler
    ├── routing-service/     # Provider routing rules
    ├── provider-service/    # SMSC provider management
    ├── client-service/      # Client account management
    ├── analytics-service/   # Statistics aggregation
    ├── billing-service/     # Balance + charging
    ├── webhook-service/     # Delivery notifications
    └── template-service/    # Message templates

internal/
├── gateway/                 # Gateway implementations (admin, client, smpp)
├── services/                # Domain services (DDD: domain/application/infrastructure/grpc)
├── smpp/protocol/           # Custom SMPP PDU encoder/decoder
├── smpp/server/             # SMPP server logic
├── queue/                   # Kafka producer/consumer
├── shared/                  # Common models, errors, logging, cache, database
├── storage/                 # Database repositories
├── monitoring/              # Prometheus metrics
└── testutil/                # Test fixtures & mocks

api/proto/                   # Protobuf definitions
deployments/                 # Docker Compose
migrations/                  # PostgreSQL migrations
configs/                     # Service configurations
test/                        # Integration & load tests
```

**Structure Decision**: Существующая микросервисная структура с DDD-паттерном внутри каждого сервиса. Код организован по `cmd/` (точки входа), `internal/` (бизнес-логика), `api/` (контракты). Изменения вносятся в существующую структуру без создания новых сервисов.

## Implementation Gaps (Research Findings)

На основе анализа кодовой базы и спецификации выявлены следующие доработки:

### Gap 1: Multipart SMS (UDH)

**Текущее состояние**: SMPP протокол (`internal/smpp/protocol/`) обрабатывает одиночные PDU без поддержки UDH.
**Требуется**: Добавить разбиение сообщений >160 символов на сегменты с UDH-заголовками. Обновить модель `Message` (поле `segment_count`). Обновить worker для отправки нескольких `submit_sm` PDU.

### Gap 2: DLR Expiry

**Текущее состояние**: Сообщения в статусе "sent" остаются навсегда без DLR.
**Требуется**: Фоновый процесс (аналог scheduler) для перевода "sent" → "expired" через 24ч. Новый статус "expired" в lifecycle. Конфигурация `DLR_EXPIRY_TIMEOUT`.

### Gap 3: Data Purging

**Текущее состояние**: Данные хранятся бессрочно. Таблицы `messages` и `audit_log` уже партиционированы по месяцам.
**Требуется**: Автоматическое удаление партиций старше 90 дней (messages) и 1 года (audit_log). Можно реализовать как cron-задачу или фоновый процесс.

### Gap 4: Per-Segment Billing

**Текущее состояние**: Биллинг рассчитывается за сообщение.
**Требуется**: Рассчитывать стоимость на основе `segment_count` × цена за сегмент. Обновить billing-service и API баланса.

## Complexity Tracking

Нарушений конституции нет (конституция не настроена). Сложность оправдана масштабом платформы (12 микросервисов).
