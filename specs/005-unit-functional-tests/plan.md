# Implementation Plan: Unit & Functional Tests

**Branch**: `005-unit-functional-tests` | **Date**: 2026-03-21 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/005-unit-functional-tests/spec.md`

## Summary

Реализация комплексного набора юнит-тестов и функциональных тестов для всех сервисов SMS-платформы. Юнит-тесты покрывают application, domain, gRPC и gateway слои с изоляцией через testify/mock. Функциональные тесты проверяют 4 ключевые бизнес-цепочки (SMS, биллинг, auth, HLR) с тестовой PostgreSQL и транзакционной изоляцией. Существующие 28 тест-файлов рефакторятся для единообразной Describe/Context/It структуры.

## Technical Context

**Language/Version**: Go 1.24.0
**Primary Dependencies**: testify/assert, testify/require, testify/mock, pgx/v5, go-redis/v9, gorilla/mux, google.golang.org/grpc, zerolog
**Storage**: PostgreSQL 15+ (тестовая БД через pgx), Redis 7+ (тестовый для сессий/кеша)
**Testing**: go test, testify (assert/require/mock), httptest, bufconn (gRPC in-process)
**Target Platform**: Linux server
**Project Type**: Microservices platform (7 сервисов, 3 gateway)
**Performance Goals**: Юнит-тесты < 30 секунд, покрытие application >= 70%
**Constraints**: Юнит-тесты без внешних зависимостей. Kafka мокируется через интерфейсы. Функциональные тесты — транзакция с rollback
**Scale/Scope**: 43 интерфейса для мокирования, 7 сервисов, 3 gateway, 28 существующих тест-файлов для рефакторинга

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Domain-Driven Design | PASS | Тесты следуют DDD-структуре: моки в `mocks/` внутри каждого сервиса, тесты рядом с тестируемым кодом |
| II. Event-Driven Architecture | PASS | Kafka EventPublisher/EventConsumer мокируются через domain-интерфейсы, идемпотентность проверяется в тестах |
| III. Contract-First APIs | PASS | gRPC-серверы тестируются через bufconn, маппинг ошибок → gRPC codes проверяется |
| IV. Observability | PASS | Не добавляем новые метрики, тестируем существующие через mock logger |
| V. Data Safety | PASS | Транзакционная изоляция для тестовой БД, проверка атомарности balance операций |
| VI. Simplicity | PASS | testify/mock — стандартный инструмент Go, без новых зависимостей. Расширяем существующий testutil |

**Post-Phase 1 re-check**: Все гейты пройдены. Моки в `mocks/` подпакетах каждого сервиса — соответствует bounded context (DDD). Нет нарушений конституции.

## Project Structure

### Documentation (this feature)

```text
specs/005-unit-functional-tests/
├── plan.md              # This file
├── research.md          # Phase 0: decisions on mocking, isolation, structure
├── data-model.md        # Phase 1: mock organization, fixture map, interface mapping
├── quickstart.md        # Phase 1: how to run tests
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── services/
│   ├── auth/
│   │   ├── application/
│   │   │   ├── auth_service.go
│   │   │   ├── auth_service_test.go          # NEW: юнит-тесты AuthService
│   │   │   ├── token_service.go
│   │   │   ├── token_service_test.go         # NEW: юнит-тесты TokenService
│   │   │   ├── totp_service.go
│   │   │   ├── totp_service_test.go          # NEW: юнит-тесты TOTPService
│   │   │   ├── password_reset_service.go
│   │   │   └── password_reset_service_test.go # NEW: юнит-тесты PasswordResetService
│   │   ├── domain/
│   │   │   ├── user.go
│   │   │   ├── user_test.go                  # NEW: domain-тесты User
│   │   │   ├── api_key.go
│   │   │   ├── api_key_test.go               # NEW: domain-тесты APIKey
│   │   │   ├── role.go
│   │   │   └── role_test.go                  # NEW: domain-тесты Role/Permission
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   └── server_test.go                # NEW: gRPC маппинг тесты
│   │   └── mocks/                            # NEW: testify/mock implementations
│   │       ├── mock_password_hasher.go
│   │       └── mock_api_key_generator.go
│   ├── messaging/
│   │   ├── application/
│   │   │   ├── message_service.go
│   │   │   ├── message_service_test.go       # NEW
│   │   │   ├── dlr_service.go
│   │   │   ├── dlr_service_test.go           # NEW
│   │   │   ├── scheduler.go
│   │   │   └── scheduler_test.go             # NEW
│   │   ├── domain/
│   │   │   ├── message.go
│   │   │   ├── message_test.go               # NEW: state transitions
│   │   │   ├── validation.go
│   │   │   └── validation_test.go            # NEW
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   └── server_test.go                # NEW
│   │   └── mocks/                            # NEW
│   │       ├── mock_message_repository.go
│   │       ├── mock_dlr_repository.go
│   │       └── mock_event_publisher.go
│   ├── routing/
│   │   ├── application/
│   │   │   ├── hlr_service.go
│   │   │   ├── hlr_service_test.go           # NEW: HLR lookup, cache, failover
│   │   │   ├── routing_service.go
│   │   │   ├── routing_service_test.go       # NEW
│   │   │   ├── smart_routing_service.go
│   │   │   ├── smart_routing_service_test.go # NEW
│   │   │   ├── operator_resolver.go
│   │   │   └── operator_resolver_test.go     # NEW
│   │   ├── domain/
│   │   │   ├── hlr.go
│   │   │   ├── hlr_test.go                   # NEW: IsDeliverable, IsInvalid
│   │   │   ├── route.go
│   │   │   └── route_test.go                 # NEW
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   └── server_test.go                # NEW
│   │   └── mocks/                            # NEW (12 mock files)
│   ├── billing/
│   │   ├── application/
│   │   │   ├── billing_service.go
│   │   │   ├── billing_service_test.go       # NEW: charge, refund, insufficient funds
│   │   │   ├── pricing_service.go
│   │   │   └── pricing_service_test.go       # NEW
│   │   ├── domain/
│   │   │   ├── account.go
│   │   │   ├── account_test.go               # NEW
│   │   │   ├── transaction.go
│   │   │   └── transaction_test.go           # NEW
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   └── server_test.go                # NEW
│   │   └── mocks/                            # NEW (5 mock files)
│   ├── tarification/
│   │   ├── application/
│   │   │   ├── tarification_service.go
│   │   │   ├── tarification_service_test.go  # NEW
│   │   │   ├── strategy_fixed.go
│   │   │   ├── strategy_fixed_test.go        # NEW
│   │   │   ├── strategy_threshold.go
│   │   │   ├── strategy_threshold_test.go    # NEW
│   │   │   ├── strategy_prepaid.go
│   │   │   ├── strategy_prepaid_test.go      # NEW
│   │   │   ├── saga.go
│   │   │   └── saga_test.go                  # NEW
│   │   ├── domain/
│   │   │   ├── tariff_plan.go
│   │   │   ├── tariff_plan_test.go           # NEW
│   │   │   ├── usage_counter.go
│   │   │   └── usage_counter_test.go         # NEW
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   └── server_test.go                # NEW
│   │   └── mocks/                            # NEW (10 mock files)
│   ├── analytics/
│   │   ├── application/
│   │   │   ├── analytics_service.go
│   │   │   ├── analytics_service_test.go     # NEW
│   │   │   ├── aggregation_service.go
│   │   │   ├── aggregation_service_test.go   # NEW
│   │   │   ├── report_service.go
│   │   │   └── report_service_test.go        # NEW
│   │   ├── grpc/
│   │   │   ├── server.go
│   │   │   └── server_test.go                # NEW
│   │   └── mocks/                            # NEW (3 mock files)
│   └── client/
│       ├── application/
│       │   ├── client_service.go
│       │   └── client_service_test.go        # NEW
│       ├── grpc/
│       │   ├── server.go
│       │   └── server_test.go                # NEW
│       └── mocks/                            # NEW (2 mock files)
├── gateway/
│   ├── admin/
│   │   ├── handlers/
│   │   │   ├── *_test.go                     # NEW: тесты для каждого handler
│   │   └── middleware/
│   │       └── *_test.go                     # NEW/REFACTOR
│   ├── client/
│   │   ├── handlers/
│   │   │   ├── *_test.go                     # NEW
│   │   └── middleware/
│   │       └── *_test.go                     # NEW/REFACTOR
│   └── portal/
│       ├── handlers/
│       │   ├── *_test.go                     # NEW
│       └── middleware/
│           └── *_test.go                     # NEW
├── testutil/
│   ├── mocks.go                              # EXISTING (не трогаем)
│   ├── fixtures.go                           # EXTEND: новые фикстуры
│   ├── testdb.go                             # EXTEND: BeginTestTx()
│   └── service_fixtures.go                   # NEW: фикстуры для domain entities
├── router/
│   ├── router_test.go                        # REFACTOR: Describe/Context/It
│   └── retry_test.go                         # REFACTOR
├── api/
│   ├── http/handlers_test.go                 # REFACTOR
│   ├── grpc/server_test.go                   # REFACTOR
│   └── middleware/*_test.go                  # REFACTOR
├── shared/
│   └── errors_test.go                        # REFACTOR
├── smpp/protocol/
│   ├── decoder_test.go                       # REFACTOR
│   ├── encoder_test.go                       # REFACTOR
│   └── validator_test.go                     # REFACTOR
├── monitoring/
│   └── metrics_test.go                       # REFACTOR
└── smsc/
    └── pool_test.go                          # REFACTOR

test/
├── functional/                               # NEW
│   ├── messaging_test.go                     # SMS send → DLR → cancel chain
│   ├── billing_test.go                       # charge → refund → insufficient funds
│   ├── auth_test.go                          # login → token → TOTP → API key
│   ├── hlr_routing_test.go                   # lookup → cache → failover → route
│   └── helpers_test.go                       # shared functional test utilities
├── integration/
│   ├── grpc_services_test.go                 # REFACTOR
│   ├── kafka_test.go                         # REFACTOR
│   ├── storage_test.go                       # REFACTOR
│   ├── service_communication_test.go         # REFACTOR
│   └── directus_test.go                      # REFACTOR
└── load/
    ├── api_load_test.go                      # REFACTOR
    └── high_performance_test.go              # REFACTOR
```

**Structure Decision**: Тесты размещаются рядом с тестируемым кодом (Go convention `*_test.go` в том же пакете). Моки — в подпакете `mocks/` каждого сервиса. Функциональные тесты — в `test/functional/`. Существующие тесты рефакторятся in-place.

## Complexity Tracking

Нарушений конституции нет. Таблица не заполняется.
