# Implementation Plan: Operator-Based SMS Tarification System

**Branch**: `002-operator-tarification` | **Date**: 2026-03-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/002-operator-tarification/spec.md`

## Summary

Система тарификации SMS на основе операторов с 4 стратегиями биллинга. Расширяется routing-service (новые сущности Country, Operator, OperatorPrefix) и создаётся новый tarification-service (TariffPlan, TariffPeriod, TariffTier, PricingPeriod, PrepaidFee, SenderRegistration, UsageCounter, TarificationLog). Tarification-service взаимодействует с billing-service через gRPC с Saga-паттерном для обеспечения консистентности. Billing-service остаётся без изменений в модели данных.

## Technical Context

**Language/Version**: Go 1.24.0
**Primary Dependencies**: gorilla/mux (HTTP), google.golang.org/grpc v1.78.0 (gRPC), IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), spf13/viper (config), prometheus/client_golang (metrics), stretchr/testify (testing)
**Storage**: PostgreSQL 15+ (pgx driver, monthly partitioning for high-volume tables), Redis 7+ (caching)
**Testing**: go test, stretchr/testify, integration tests with `//go:build integration` build tag
**Target Platform**: Linux server (Docker Compose + HAProxy)
**Project Type**: Microservices (gRPC + HTTP REST)
**Performance Goals**: Тарификация не должна замедлять существующий поток отправки SMS
**Constraints**: Атомарность балансовых операций, идемпотентность тарификации, eventual consistency при пересчёте
**Scale/Scope**: Масштаб существующей платформы — расширение routing-service + новый tarification-service

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Domain-Driven Design | PASS | Новый tarification-service имеет собственный bounded context. Routing-service расширяется в рамках своего контекста (Country, Operator — часть маршрутизации). Структура domain/application/infrastructure/grpc соблюдена |
| II. Event-Driven Architecture | PASS | Tarification-service публикует события в Kafka (tarification.results, tarification.recalc, tarification.prepaid). Consumers идемпотентны через idempotency_key |
| III. Contract-First APIs | PASS | Новые proto-файлы для tarification-service, расширение routing.proto. Proto определяются до реализации |
| IV. Observability | PASS | Prometheus-метрики для tarification-service (tarification_messages_total, tarification_rejections_total и др.). Zerolog для логирования. request_id в трейсинге |
| V. Data Safety | PASS | Идемпотентность через уникальный ключ в tarification_log. Атомарность через SELECT FOR UPDATE на usage_counters. Saga с компенсацией для billing. Retention: 12 мес для логов |
| VI. Simplicity | JUSTIFIED | Новый tarification-service вместо расширения billing-service — обосновано: billing-service остаётся простым "кошельком", а тарификационная логика (4 стратегии, пороги, периоды, пересчёт) значительно сложнее и заслуживает отдельного контекста. См. Complexity Tracking |

## Project Structure

### Documentation (this feature)

```text
specs/002-operator-tarification/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
# Новый сервис: tarification-service
cmd/services/tarification-service/
└── main.go

internal/services/tarification/
├── domain/
│   ├── errors.go
│   ├── sender_registration.go
│   ├── tariff_plan.go
│   ├── tariff_period.go
│   ├── tariff_tier.go
│   ├── pricing_period.go
│   ├── prepaid_fee.go
│   ├── usage_counter.go
│   ├── tarification_log.go
│   ├── repository.go
│   └── queue.go
├── application/
│   ├── tarification_service.go
│   ├── strategy.go              # Strategy interface
│   ├── strategy_fixed.go
│   ├── strategy_threshold.go
│   ├── strategy_threshold_recalc.go
│   ├── strategy_prepaid.go
│   ├── sender_service.go
│   ├── tariff_plan_service.go
│   └── saga.go                  # Saga orchestration
├── grpc/
│   └── server.go
└── infrastructure/
    ├── queue/
    │   ├── event_publisher.go
    │   └── event_consumer.go
    └── repository/
        ├── sender_registration_repository.go
        ├── tariff_plan_repository.go
        ├── tariff_period_repository.go
        ├── tariff_tier_repository.go
        ├── pricing_period_repository.go
        ├── prepaid_fee_repository.go
        ├── usage_counter_repository.go
        └── tarification_log_repository.go

# Расширение routing-service
internal/services/routing/
├── domain/
│   ├── country.go               # NEW
│   ├── operator.go              # NEW
│   └── operator_prefix.go       # NEW
├── application/
│   └── operator_resolver.go     # NEW — определение оператора по номеру
└── infrastructure/
    └── repository/
        ├── country_repository.go      # NEW
        ├── operator_repository.go     # NEW
        └── operator_prefix_repository.go  # NEW

# Расширение admin-gateway
internal/gateway/admin/
├── handlers/
│   ├── tarification.go          # NEW
│   ├── country.go               # NEW
│   └── operator.go              # NEW
└── router/
    └── router.go                # MODIFY — добавить маршруты

# Proto-определения
api/proto/
├── tarification/
│   └── tarification.proto       # NEW
├── tarificationv1/              # NEW — generated
├── routing/
│   └── routing.proto            # MODIFY — добавить Country/Operator RPCs

# Миграции
migrations/
├── 000012_create_country_operator_tables.up.sql    # NEW
├── 000012_create_country_operator_tables.down.sql  # NEW
├── 000013_create_tarification_tables.up.sql        # NEW
└── 000013_create_tarification_tables.down.sql      # NEW
```

**Structure Decision**: Новый tarification-service следует существующему паттерну DDD (domain/application/infrastructure/grpc). Routing-service расширяется новыми доменными сущностями в рамках своего bounded context. Admin-gateway получает новые handlers для CRUD операций.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Новый tarification-service вместо расширения billing-service | 4 стратегии тарификации, пороги, периоды, пересчёт, Saga — значительная доменная сложность | Billing-service стал бы перегруженным двумя bounded contexts (баланс + тарификация), что нарушает принцип I (DDD). Tarification-service отвечает за ценообразование, billing-service — за баланс и транзакции |
