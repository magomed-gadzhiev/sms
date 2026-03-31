# Implementation Plan: Multichannel Cascade Delivery

**Branch**: `012-multichannel-cascade` | **Date**: 2026-03-31 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/012-multichannel-cascade/spec.md`

---

## Summary

Реализация Kafka-based event-driven оркестратора каскадной доставки сообщений через несколько
каналов (SMS, flash call, reverse call, мессенджеры). Система позволяет клиентам выбирать
именованную стратегию доставки (упорядоченные шаги с таймаутами), автоматически переключаясь
на следующий канал при недоставке. Оркестрация асинхронна — каждый шаг инициируется Kafka-событием.
Архитектура: новый `cascade-service` с Go-интерфейсом `Channel` (in-process), расширение
`tarification-service` для мультиканальной тарификации, новые admin/client UI страницы.

---

## Technical Context

**Language/Version**: Go 1.24+ (backend), TypeScript 5.x + React 19 (frontend)  
**Primary Dependencies**: gorilla/mux, google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0, jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, prometheus/client_golang  
**Storage**: PostgreSQL 15+ (pgx, monthly partitioning для `deliveries`, `delivery_attempts`), Redis 7+ (reachability cache TTL 1h)  
**Testing**: stretchr/testify (unit), functional tag, integration tag  
**Target Platform**: Linux server (Docker Compose)  
**Project Type**: microservice + frontend SPA extension  
**Performance Goals**: до 1 000 каскадов/сек; 95% deliveries завершены за 30s (SC-002)  
**Constraints**: p95 e2e < 35s; нет двойной тарификации при late_duplicate; RUB валюта

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked post Phase 1 design.*

| Принцип | Статус | Примечание |
|---------|--------|-----------|
| **I. DDD**: cascade-service owns cascade bounded context | ✅ PASS | Отдельный домен: channels, strategies, deliveries, reachability. Нет прямого DB-доступа к чужим таблицам. |
| **I. DDD**: layered structure domain/application/infrastructure/grpc | ✅ PASS | Структура следует паттерну messaging-service, billing-service. |
| **II. Event-Driven**: Kafka для всех state transitions каскада | ✅ PASS | 4 новых Kafka-топика; все переходы состояний через события. |
| **II. Event-Driven**: idempotent consumers | ✅ PASS | Оркестратор проверяет текущий статус delivery/attempt перед обработкой. |
| **II. Event-Driven**: schema_version в событиях | ✅ PASS | Все Kafka-события включают `schema_version: "1"`. |
| **III. Contract-First**: proto + HTTP API до реализации | ✅ PASS | `contracts/cascade.proto`, `contracts/http-api.md` определены. |
| **IV. Observability**: Prometheus metrics, zerolog, request_id, health | ✅ PASS | cascade-service экспонирует `/metrics`, `/health`, `/health/ready`. |
| **V. Data Safety**: monthly partitioning для высоконагруженных таблиц | ✅ PASS | `deliveries`, `delivery_attempts` партиционированы по месяцам. |
| **V. Data Safety**: RUB валюта | ✅ PASS | Все monetary поля в RUB, DEFAULT 'RUB'. |
| **V. Data Safety**: sensitive data encrypted | ✅ PASS | `channel.config` JSON — sensitive поля шифруются server-side. |
| **VI. Simplicity**: новый сервис обоснован | ✅ PASS | *см. Complexity Tracking* |

---

## Project Structure

### Documentation (this feature)

```text
specs/012-multichannel-cascade/
├── plan.md              ← этот файл
├── spec.md
├── research.md          ← Phase 0 ✅
├── data-model.md        ← Phase 1 ✅
├── quickstart.md        ← Phase 1 ✅
├── contracts/
│   ├── cascade.proto    ← Phase 1 ✅
│   └── http-api.md      ← Phase 1 ✅
└── tasks.md             ← Phase 2 (/speckit.tasks)
```

### Source Code

```text
# Backend (Go)
cmd/services/
└── cascade-service/
    └── main.go                        # Entry point (паттерн из messaging-service)

internal/services/cascade/
├── domain/
│   ├── channel.go                     # Channel interface + ChannelType constants
│   ├── channel_config.go              # ChannelConfig entity
│   ├── strategy.go                    # DeliveryStrategy + StrategyStep entities
│   ├── delivery.go                    # Delivery entity + state machine transitions
│   ├── attempt.go                     # DeliveryAttempt entity
│   ├── operator_support.go            # OperatorChannelSupport entity
│   ├── errors.go
│   └── repository.go                  # Repository interfaces
├── application/
│   ├── cascade_service.go             # Orchestration: next step, stop, late_duplicate
│   ├── channel_service.go             # Channel CRUD
│   ├── strategy_service.go            # Strategy CRUD
│   ├── delivery_service.go            # Delivery history + stats queries
│   ├── reachability_service.go        # HLR + OCS matrix check, Redis cache
│   ├── billing_integration.go         # tarification + billing gRPC calls
│   └── scheduler.go                   # Timeout poller (Start/Stop pattern)
├── channels/
│   ├── sms/
│   │   └── adapter.go                 # SMS channel: publishes to sms.outgoing
│   └── flash_call/
│       └── adapter.go                 # Flash call: HTTP provider, webhook result
├── infrastructure/
│   ├── postgres/
│   │   ├── channel_repo.go
│   │   ├── strategy_repo.go
│   │   ├── delivery_repo.go
│   │   └── attempt_repo.go
│   └── kafka/
│       ├── orchestrator.go            # Consumes cascade.attempt.result, drives state machine
│       ├── producer.go                # Publishes cascade.* events
│       └── events.go                  # Event struct definitions (schema_version)
└── grpc/
    └── handler.go                     # Implements CascadeService, ChannelAdminService, StrategyAdminService

api/proto/cascade/
└── cascade.proto                      # Contract (copy from specs/contracts/)
api/proto/cascadev1/
└── cascade.pb.go                      # Generated (protoc)
    cascade_grpc.pb.go

internal/gateway/portal/handlers/
├── cascade_channels.go                # Admin: GET/POST/PUT channels, toggle
├── cascade_strategies.go              # Admin: CRUD strategies + OCS matrix
└── cascade_webhook.go                 # POST /webhooks/cascade/flash-call/{attempt_id}

internal/gateway/client/handlers/
└── cascade.go                         # Client: POST /cascade/deliveries, GET history + stats

internal/gateway/portal/router/
└── router.go                          # Mount /admin/channels, /admin/delivery-strategies, etc.

internal/gateway/client/router/        # Mount /cascade/* routes

# Migrations
migrations/
├── 000067_cascade_channels.up.sql
├── 000067_cascade_channels.down.sql
├── 000068_delivery_strategies.up.sql
├── 000068_delivery_strategies.down.sql
├── 000069_deliveries.up.sql           # deliveries + delivery_attempts (monthly partitioned)
├── 000069_deliveries.down.sql
├── 000070_operator_channel_support.up.sql
└── 000070_operator_channel_support.down.sql

# Frontend
portal-frontend/src/
├── api/
│   └── cascade.ts                     # API client (channels, strategies, deliveries)
├── pages/
│   ├── channels/
│   │   ├── ChannelsPage.tsx           # Admin: list + toggle channels
│   │   └── ChannelFormModal.tsx       # Admin: create/edit channel
│   ├── delivery-strategies/
│   │   ├── DeliveryStrategiesPage.tsx # Admin: list strategies
│   │   ├── StrategyFormModal.tsx      # Admin: create/edit strategy + steps
│   │   └── OperatorSupportMatrix.tsx  # Admin: OCS matrix editor
│   └── cascade-history/
│       ├── CascadeHistoryPage.tsx     # Client: delivery list with filters
│       └── CascadeDeliveryDetail.tsx  # Client: delivery detail (all attempts)
└── App.tsx                            # Add routes for new pages

# Config
configs/config.example.yaml            # Add cascade_service_addr + 4 Kafka topics

# Compose
deployments/docker-compose.yml         # Add cascade-service container
```

**Structure Decision**: Monorepo. Новый `cascade-service` в `cmd/services/cascade-service/`
с бизнес-логикой в `internal/services/cascade/` следует установленному паттерну всех сервисов
платформы (messaging-service, billing-service, routing-service). Frontend-страницы добавляются
в существующий `portal-frontend`.

---

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Новый `cascade-service` (Constitution VI — prefer extending existing) | 1) Distinct bounded context (стратегии, каналы, state machine, reachability) не относится ни к messaging (отправка), ни к routing (маршрутизация). 2) Отдельный lifecycle данных — `deliveries`+`delivery_attempts` с monthly partitioning. 3) Channel adapters (Go-интерфейс) должны жить в одном процессе с оркестратором. 4) Scaling profile: оркестратор масштабируется независимо при росте каскадов/сек | Расширение `messaging-service`: нарушит bounded context (DDD), smешает delivery pipeline с cascade state machine, усложнит тестирование и scaling |
| 4 новых Kafka-топика | Каскадная оркестрация требует отдельных топиков для изоляции от SMS pipeline | Использование существующих `sms.*` топиков: разные схемы данных, consumers, retention policy; смешение нарушит idempotency SMS pipeline |

---

## Post-Design Constitution Re-Check

После создания артефактов Phase 1:

- **data-model.md**: `deliveries` + `delivery_attempts` — monthly partitioning ✅; `schema_version` в Kafka events ✅; все цены в RUB ✅.
- **cascade.proto**: Contract-First ✅; три сервиса (CascadeService, ChannelAdminService, StrategyAdminService) с чёткими bounded context ✅.
- **http-api.md**: все endpoints auth-protected, admin роль для admin routes ✅.
- **Simplicity**: channel adapters — минимальный интерфейс (1 метод `Send`); нет premature abstractions ✅.

**All gates PASS**. Готово к `/speckit.tasks`.
