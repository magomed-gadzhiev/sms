# Implementation Plan: Operator Sender Name Billing

**Branch**: `011-operator-sender-billing` | **Date**: 2026-03-31 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/011-operator-sender-billing/spec.md`

## Summary

Расширить платформу механизмом ежемесячной тарификации платных имён отправителей: добавить тариф оператора (`monthly_tariff_amount`) в домен routing-service, создать сущность `SenderNameBillingRecord` в tarification-service, реализовать cron-планировщик для ежемесячного идемпотентного начисления и отобразить стоимость inline в форме регистрации на клиентском портале.

## Technical Context

**Language/Version**: Go 1.24.0 (backend), TypeScript 5.x + React 19 / Vite (frontend)  
**Primary Dependencies**: gorilla/mux, google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0, jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, prometheus/client_golang  
**Storage**: PostgreSQL 15+ (pgx driver); таблицы `operators` (ALTER), новая `sender_name_billing_records`  
**Testing**: stretchr/testify; build tags `//go:build integration`, `//go:build functional`  
**Target Platform**: Linux server (Docker Compose)  
**Project Type**: Microservice platform extension  
**Performance Goals**: Ежемесячный cron обрабатывает все активные платные регистрации — ожидаемый объём <10 000 записей, линейное время  
**Constraints**: Атомарность начислений; идемпотентность планировщика; валюта RUB only  
**Scale/Scope**: Расширение существующих сервисов (routing-service, tarification-service, admin-gateway, portal-gateway, frontend)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. DDD — bounded contexts | ✅ PASS | routing-service владеет оператором и тарифом; tarification-service владеет billing records |
| II. Event-Driven — Kafka events | ✅ PASS | Начисление локальное внутри tarification-service; cross-service trigger не требуется |
| III. Contract-First | ✅ PASS | Proto обновляется до реализации (plan → tasks → implement) |
| IV. Observability | ✅ PASS | Добавить счётчик billing_records_created_total и billing_scheduler_run_total |
| V. Data Safety — atomic billing, RUB | ✅ PASS | Транзакция при создании billing record; NUMERIC(20,6) RUB |
| VI. Simplicity — extend, not create | ✅ PASS | Новых сервисов нет; расширяются routing-service и tarification-service |

**Post-design re-check**: После Phase 1 переподтвердить, что `sender_name_billing_records` не требует month-partitioning (объём <10k строк/мес — не требует).

## Project Structure

### Documentation (this feature)

```text
specs/011-operator-sender-billing/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── routing.proto.diff.md
│   └── tarification.proto.diff.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
# Backend — изменения в существующих сервисах

internal/services/routing/domain/
└── operator.go                            ← +MonthlyTariffAmount *decimal.Decimal

internal/services/routing/infrastructure/repository/
└── operator_repository.go                 ← UPDATE/SELECT с monthly_tariff_amount

internal/services/routing/grpc/
└── server.go                              ← маппинг нового поля

internal/services/tarification/domain/
├── sender_billing.go                      ← новая сущность SenderNameBillingRecord
└── interfaces.go                          ← +SenderBillingRepository интерфейс

internal/services/tarification/infrastructure/repository/
└── sender_billing_repository.go           ← новый репозиторий

internal/services/tarification/application/
├── sender_billing_service.go              ← CreateBillingRecord, ListBillingRecords
└── billing_scheduler.go                   ← ежемесячный cron-планировщик

internal/services/tarification/grpc/
└── server.go                              ← +CreateSenderBilling, ListSenderBillings

internal/gateway/admin/handlers/
└── operator.go                            ← включить tariff в ответ

internal/gateway/portal/handlers/
└── sender_names.go                        ← GET /operators/:id/tariff; GET /sender-names/:id/billing

api/proto/routing/
└── routing.proto                          ← +monthly_tariff_amount в Operator messages

api/proto/tarification/
└── tarification.proto                     ← +SenderBillingRecord messages + RPCs

migrations/
├── 000065_operator_tariff.up.sql         ← ALTER TABLE operators ADD monthly_tariff_amount
├── 000065_operator_tariff.down.sql
├── 000066_sender_name_billing.up.sql     ← CREATE TABLE sender_name_billing_records
└── 000066_sender_name_billing.down.sql

# Frontend — изменения в portal и admin SPA

frontend/src/pages/admin/operators/
├── OperatorForm.tsx                       ← поля registration_strategy + monthly_tariff
└── OperatorDetail.tsx                     ← отображение тарифа

frontend/src/pages/portal/sender-names/
├── SenderNameCreate.tsx                   ← inline стоимость при выборе paid
├── SenderNameList.tsx                     ← колонка "Стоимость/мес" + следующее начисление
└── SenderNameBillingHistory.tsx           ← история начислений
```

**Structure Decision**: Монорепо, расширение существующих сервисов. Новых сервисов нет.

## Complexity Tracking

> Нарушений конституции нет — секция не требуется.
