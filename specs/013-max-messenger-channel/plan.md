# Implementation Plan: Max Messenger Channel

**Branch**: `013-max-messenger-channel` | **Date**: 2026-04-01 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/013-max-messenger-channel/spec.md`

## Summary

Добавление нового канала доставки — мессенджера Max — в существующую систему мультиканальной каскадной доставки. Max Messenger интегрируется как HTTP-based channel adapter (по паттерну Flash Call), с поддержкой проверки регистрации получателя, webhook-обработки статусов, retry при rate-limiting и Prometheus-метрик. Не требует новых микросервисов или таблиц БД — расширяет cascade-service.

## Technical Context

**Language/Version**: Go 1.24.0  
**Primary Dependencies**: gorilla/mux (HTTP), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), prometheus/client_golang (metrics), IBM/sarama v1.43.0 (Kafka), crypto/hmac (webhook signature)  
**Storage**: PostgreSQL 15+ (существующие таблицы cascade), Redis 7+ (reachability cache)  
**Testing**: stretchr/testify (unit), go:build functional (functional tests)  
**Target Platform**: Linux server (Docker Compose)  
**Project Type**: Microservice extension (cascade-service)  
**Performance Goals**: Доставка через Max < 20 секунд, проверка регистрации < 1 секунда  
**Constraints**: RUB-only billing, HMAC-SHA256 webhook verification, rate-limit retry 1s→2s→4s  
**Scale/Scope**: Расширение 1 сервиса, 1 новый adapter package, 1 миграция, обновление frontend dropdown

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Принцип | Pre-Design | Post-Design | Комментарий |
|---------|-----------|-------------|-------------|
| I. DDD | PASS | PASS | Новый адаптер в bounded context cascade-service, domain layer не зависит от infrastructure |
| II. Event-Driven | PASS | PASS | Webhook → Kafka `cascade.attempt.result`, биллинг через `cascade.billing` |
| III. Contract-First | PASS | PASS | Контракты в `contracts/` определены до реализации |
| IV. Observability | PASS | PASS | zerolog structured logging, Prometheus метрики, request_id propagation |
| V. Data Safety | PASS | PASS | API keys encrypted в JSONB, idempotent billing via idempotency_key, RUB only |
| VI. Simplicity | PASS | PASS | Расширяем cascade-service, не создаём новый сервис |

## Project Structure

### Documentation (this feature)

```text
specs/013-max-messenger-channel/
├── plan.md                                  # This file
├── spec.md                                  # Feature specification
├── research.md                              # Phase 0: research decisions
├── data-model.md                            # Phase 1: data model
├── quickstart.md                            # Phase 1: quickstart guide
├── contracts/
│   ├── max-messenger-send-api.md            # Outgoing: cascade → Max Bot API
│   └── webhook-max-messenger.md             # Incoming: Max → cascade webhook
└── tasks.md                                 # Phase 2: implementation tasks
```

### Source Code (repository root)

```text
internal/services/cascade/
├── domain/
│   └── channel.go                           # MODIFY: add ChannelMaxMessenger constant
├── channels/
│   ├── sms/adapter.go                       # EXISTING: reference pattern (Kafka-based)
│   ├── flash_call/adapter.go                # EXISTING: reference pattern (HTTP-based)
│   └── max_messenger/                       # NEW: Max Messenger adapter package
│       ├── adapter.go                       # Channel interface implementation
│       ├── metrics.go                       # Channel-specific Prometheus metrics
│       ├── webhook_handler.go               # HTTP handler for incoming webhooks
│       └── reachability.go                  # Max-specific registration check
├── application/
│   ├── cascade_service.go                   # EXISTING: no changes (generic orchestration)
│   ├── reachability_service.go              # MODIFY: integrate Max registration check
│   └── billing_integration.go              # EXISTING: no changes (generic billing)
└── infrastructure/
    └── kafka/orchestrator.go               # EXISTING: no changes (generic event handling)

cmd/services/cascade-service/
└── main.go                                  # MODIFY: register MaxMessengerAdapter

migrations/
└── 000071_max_messenger_channel.up.sql      # NEW: seed channel + operator support
└── 000071_max_messenger_channel.down.sql    # NEW: rollback

portal-frontend/src/
└── pages/channels/ChannelFormModal.tsx       # MODIFY: add 'max_messenger' to channel type dropdown
```

**Structure Decision**: Расширяем существующий cascade-service новым channel adapter package `max_messenger/`. Следуем паттерну `flash_call/` (HTTP-based adapter). Нет новых микросервисов, таблиц или proto-файлов.

## Complexity Tracking

> Нарушений конституции нет — раздел пуст.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
