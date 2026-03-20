# Tasks: Operator-Based SMS Tarification System

**Input**: Design documents from `/specs/002-operator-tarification/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Not explicitly requested — test tasks omitted.

**Organization**: Tasks grouped by user story. Note: US6 (Countries & Operators) is moved before US1 because tariff plans require operator_id.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Project initialization — proto files, migrations, service skeleton

- [ ] T001 Create tarification.proto with all service/message definitions in api/proto/tarification/tarification.proto
- [ ] T002 Extend routing.proto with Country, Operator, OperatorPrefix messages and RPCs in api/proto/routing/routing.proto
- [ ] T003 Generate Go code from tarification.proto into api/proto/tarificationv1/
- [ ] T004 Regenerate Go code from updated routing.proto into api/proto/routingv1/
- [ ] T005 [P] Create migration 000012_create_country_operator_tables.up.sql and .down.sql in migrations/
- [ ] T006 [P] Create migration 000013_create_tarification_tables.up.sql and .down.sql in migrations/
- [ ] T007 Create tarification-service main.go skeleton in cmd/services/tarification-service/main.go
- [ ] T008 Add tarification-service to docker-compose.yml with correct ports (gRPC: 9098, metrics: 2119) and dependencies

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Domain entities, repository interfaces, and shared infrastructure for tarification-service

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T009 [P] Create domain error types in internal/services/tarification/domain/errors.go
- [ ] T010 [P] Create SenderRegistration domain entity in internal/services/tarification/domain/sender_registration.go
- [ ] T011 [P] Create TariffPlan domain entity in internal/services/tarification/domain/tariff_plan.go
- [ ] T012 [P] Create TariffPeriod domain entity in internal/services/tarification/domain/tariff_period.go
- [ ] T013 [P] Create TariffTier domain entity in internal/services/tarification/domain/tariff_tier.go
- [ ] T014 [P] Create PricingPeriod domain entity in internal/services/tarification/domain/pricing_period.go
- [ ] T015 [P] Create PrepaidFee domain entity in internal/services/tarification/domain/prepaid_fee.go
- [ ] T016 [P] Create UsageCounter domain entity in internal/services/tarification/domain/usage_counter.go
- [ ] T017 [P] Create TarificationLog domain entity in internal/services/tarification/domain/tarification_log.go
- [ ] T018 Define all repository interfaces in internal/services/tarification/domain/repository.go
- [ ] T019 Define event publisher interface in internal/services/tarification/domain/queue.go
- [ ] T020 [P] Create BillingStrategy interface in internal/services/tarification/application/strategy.go
- [ ] T021 [P] Create Country domain entity in internal/services/routing/domain/country.go
- [ ] T022 [P] Create Operator domain entity in internal/services/routing/domain/operator.go
- [ ] T023 [P] Create OperatorPrefix domain entity in internal/services/routing/domain/operator_prefix.go
- [ ] T024 Add Country/Operator/OperatorPrefix repository interfaces to internal/services/routing/domain/repository.go

**Checkpoint**: All domain entities and interfaces defined — user story implementation can begin

---

## Phase 3: User Story 6 — Управление странами и операторами (Priority: P2, moved first as prerequisite)

**Goal**: Администратор создаёт справочники стран и операторов с привязкой префиксов. Routing-service определяет оператора по номеру.

**Independent Test**: Создание страны, оператора с префиксами через admin API, затем вызов ResolveOperator по номеру.

### Implementation for User Story 6

- [ ] T025 [P] [US6] Implement CountryRepository (CRUD) in internal/services/routing/infrastructure/repository/country_repository.go
- [ ] T026 [P] [US6] Implement OperatorRepository (CRUD + filter by country_id) in internal/services/routing/infrastructure/repository/operator_repository.go
- [ ] T027 [P] [US6] Implement OperatorPrefixRepository (CRUD + prefix lookup) in internal/services/routing/infrastructure/repository/operator_prefix_repository.go
- [ ] T028 [US6] Implement OperatorResolver service (prefix matching with priority, MNP stub) in internal/services/routing/application/operator_resolver.go
- [ ] T029 [US6] Add Country/Operator/Prefix gRPC handlers and ResolveOperator RPC to internal/services/routing/grpc/server.go
- [ ] T030 [US6] Integrate operator resolution into routing flow — enrich message with operator_id and country_id in internal/services/routing/application/routing_service.go
- [ ] T031 [P] [US6] Create CountryHandler (CRUD) in internal/gateway/admin/handlers/country.go
- [ ] T032 [P] [US6] Create OperatorHandler (CRUD + prefixes) in internal/gateway/admin/handlers/operator.go
- [ ] T033 [US6] Register country/operator routes in internal/gateway/admin/router/router.go
- [ ] T034 [US6] Wire routing-service gRPC client for Country/Operator into admin-gateway in cmd/admin-gateway/main.go

**Checkpoint**: Countries, operators, prefixes manageable via admin API. Operator resolution works by prefix.

---

## Phase 4: User Story 1 — Настройка тарификации для оператора (Priority: P1)

**Goal**: Администратор настраивает тарифный план: стратегия, периоды, пороги для оператора + категории имени.

**Independent Test**: Создание тарифного плана через admin API, добавление периода и порогов, проверка уникальности и валидации.

### Implementation for User Story 1

- [ ] T035 [P] [US1] Implement TariffPlanRepository (CRUD + unique constraint validation) in internal/services/tarification/infrastructure/repository/tariff_plan_repository.go
- [ ] T036 [P] [US1] Implement TariffPeriodRepository (CRUD + overlap exclusion check) in internal/services/tarification/infrastructure/repository/tariff_period_repository.go
- [ ] T037 [P] [US1] Implement TariffTierRepository (CRUD + immutability check for active period) in internal/services/tarification/infrastructure/repository/tariff_tier_repository.go
- [ ] T038 [P] [US1] Implement PricingPeriodRepository (CRUD + containment validation) in internal/services/tarification/infrastructure/repository/pricing_period_repository.go
- [ ] T039 [US1] Implement TariffPlanService (create/update/deactivate with active period guard) in internal/services/tarification/application/tariff_plan_service.go
- [ ] T040 [US1] Add TariffPlan/Period/Tier/PricingPeriod gRPC handlers to internal/services/tarification/grpc/server.go
- [ ] T041 [US1] Create TarificationHandler (tariff plan CRUD, period/tier management) in internal/gateway/admin/handlers/tarification.go
- [ ] T042 [US1] Register tarification admin routes in internal/gateway/admin/router/router.go
- [ ] T043 [US1] Wire tarification-service gRPC client into admin-gateway in cmd/admin-gateway/main.go
- [ ] T044 [US1] Wire repositories and services into tarification-service main.go in cmd/services/tarification-service/main.go

**Checkpoint**: Tariff plans, periods, tiers manageable via admin API. Validation rules enforced.

---

## Phase 5: User Story 5 — Регистрация имени отправителя (Priority: P2)

**Goal**: Администратор регистрирует Sender ID (платное/бесплатное) для клиента у оператора.

**Independent Test**: Регистрация имени через admin API, проверка валидации типа (оператор поддерживает/не поддерживает).

### Implementation for User Story 5

- [ ] T045 [P] [US5] Implement SenderRegistrationRepository (CRUD + unique constraint + filter) in internal/services/tarification/infrastructure/repository/sender_registration_repository.go
- [ ] T046 [US5] Implement SenderService (create with operator type validation, update status, determine sender_category) in internal/services/tarification/application/sender_service.go
- [ ] T047 [US5] Add SenderRegistration gRPC handlers to internal/services/tarification/grpc/server.go
- [ ] T048 [US5] Add sender registration endpoints to TarificationHandler in internal/gateway/admin/handlers/tarification.go
- [ ] T049 [US5] Register sender registration routes in internal/gateway/admin/router/router.go

**Checkpoint**: Sender IDs registerable. Category determination (shared/paid/free) works for tarification.

---

## Phase 6: User Story 2 — Тарификация сообщения при отправке (Priority: P1)

**Goal**: При отправке SMS система определяет оператора, находит тарифный план, рассчитывает стоимость по порогу, списывает с баланса.

**Independent Test**: Отправка SMS через API с проверкой корректного списания и обновления счётчика.

### Implementation for User Story 2

- [ ] T050 [P] [US2] Implement UsageCounterRepository (get/increment with SELECT FOR UPDATE) in internal/services/tarification/infrastructure/repository/usage_counter_repository.go
- [ ] T051 [P] [US2] Implement TarificationLogRepository (create + idempotency check + partitioning) in internal/services/tarification/infrastructure/repository/tarification_log_repository.go
- [ ] T052 [US2] Implement FixedStrategy (price_per_segment × segment_count) in internal/services/tarification/application/strategy_fixed.go
- [ ] T053 [US2] Implement ThresholdStrategy (tier lookup, split-segment billing at boundary) in internal/services/tarification/application/strategy_threshold.go
- [ ] T054 [US2] Implement Saga orchestrator (charge via billing-service gRPC, compensate on failure) in internal/services/tarification/application/saga.go
- [ ] T055 [US2] Implement TarificationService.TarifyMessage (sender_category → plan → period → strategy → saga → counter → log) in internal/services/tarification/application/tarification_service.go
- [ ] T056 [US2] Add TarifyMessage gRPC handler to internal/services/tarification/grpc/server.go
- [ ] T057 [US2] Implement Kafka event publisher (tarification.results topic) in internal/services/tarification/infrastructure/queue/event_publisher.go
- [ ] T058 [US2] Integrate tarification into messaging flow — call TarifyMessage before sending in internal/services/messaging/application/ (or routing consumer)
- [ ] T059 [US2] Add Prometheus metrics (tarification_messages_total, tarification_rejections_total, tarification_request_duration_seconds) in cmd/services/tarification-service/main.go

**Checkpoint**: Fixed and threshold strategies work end-to-end. Messages tarified before sending. Billing integration via Saga.

---

## Phase 7: User Story 3 — Пересчёт при переходе порога (Priority: P1)

**Goal**: При стратегии threshold_recalc переход порога пересчитывает все предыдущие сегменты за период по новой цене.

**Independent Test**: Серия сообщений до порога, проверка автоматического пересчёта и корректировки баланса.

### Implementation for User Story 3

- [ ] T060 [US3] Implement ThresholdRecalcStrategy (extends threshold: detect crossing, calculate recalc amount for all prior segments) in internal/services/tarification/application/strategy_threshold_recalc.go
- [ ] T061 [US3] Extend Saga with recalc flow (refund/charge for prior segments, eventual consistency with debt tracking) in internal/services/tarification/application/saga.go
- [ ] T062 [US3] Add recalc_amount to TarificationLog and TarifyMessageResponse in internal/services/tarification/domain/tarification_log.go and grpc/server.go
- [ ] T063 [US3] Implement Kafka publisher for tarification.recalc topic in internal/services/tarification/infrastructure/queue/event_publisher.go
- [ ] T064 [US3] Add Prometheus metrics (tarification_recalc_total, tarification_threshold_crossings_total) in cmd/services/tarification-service/main.go

**Checkpoint**: Threshold recalculation works. Refund/charge for prior segments. Debt tracking on insufficient balance.

---

## Phase 8: User Story 4 — Предоплата за период (Priority: P2)

**Goal**: При стратегии prepaid_threshold абонентская плата списывается автоматически в начале периода. Остаток не возвращается.

**Independent Test**: Создание тарифного плана с предоплатой, наступление периода, проверка автоматического списания.

### Implementation for User Story 4

- [ ] T065 [P] [US4] Implement PrepaidFeeRepository (CRUD + charged flag update) in internal/services/tarification/infrastructure/repository/prepaid_fee_repository.go
- [ ] T066 [US4] Implement PrepaidThresholdStrategy (prepaid fee charge + threshold billing) in internal/services/tarification/application/strategy_prepaid.go
- [ ] T067 [US4] Implement scheduler for prepaid fee collection (cron: check new periods, charge via billing-service, retry on failure) in internal/services/tarification/application/tarification_service.go
- [ ] T068 [US4] Add PrepaidFee gRPC handlers to internal/services/tarification/grpc/server.go
- [ ] T069 [US4] Add prepaid fee admin endpoints to internal/gateway/admin/handlers/tarification.go
- [ ] T070 [US4] Implement Kafka publisher for tarification.prepaid topic in internal/services/tarification/infrastructure/queue/event_publisher.go
- [ ] T071 [US4] Add Prometheus metrics (tarification_prepaid_charges_total, tarification_saga_failures_total) in cmd/services/tarification-service/main.go

**Checkpoint**: Prepaid strategy works. Automatic fee collection at period start. Blocked sends on unpaid fee.

---

## Phase 9: User Story 7 — Просмотр использования и статистики (Priority: P3)

**Goal**: Администратор просматривает счётчики использования клиентов — сегменты за период, текущий порог, стратегию.

**Independent Test**: Отправка нескольких сообщений, проверка корректности счётчиков через admin API.

### Implementation for User Story 7

- [ ] T072 [US7] Add GetUsageCounter and ListUsageCounters gRPC handlers to internal/services/tarification/grpc/server.go
- [ ] T073 [US7] Add usage counter admin endpoints (GET /admin/v1/tarification/usage) to internal/gateway/admin/handlers/tarification.go
- [ ] T074 [US7] Register usage routes in internal/gateway/admin/router/router.go

**Checkpoint**: Admin can view usage counters with current tier and pricing info.

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T075 [P] Implement pending_recalc retry consumer (process failed recalculations) in internal/services/tarification/infrastructure/queue/event_consumer.go
- [ ] T076 [P] Add graceful shutdown handling (OS signals, gRPC server, Kafka producer/consumer) to cmd/services/tarification-service/main.go
- [ ] T077 [P] Add health check endpoint (grpc.health.v1) to internal/services/tarification/grpc/server.go
- [ ] T078 [P] Add tarification_log monthly partition creation to migrations or scheduler
- [ ] T079 [P] Add data purging for tarification_log older than 12 months in scheduler
- [ ] T080 Run quickstart.md validation — full end-to-end flow from country creation to message tarification

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup (T001-T008) — BLOCKS all user stories
- **US6 (Phase 3)**: Depends on Foundational — BLOCKS US1, US2, US5 (provides operator_id)
- **US1 (Phase 4)**: Depends on US6 (needs operators to exist)
- **US5 (Phase 5)**: Depends on US6 (needs operators for sender registration)
- **US2 (Phase 6)**: Depends on US1 (needs tariff plans) + US5 (needs sender category)
- **US3 (Phase 7)**: Depends on US2 (extends tarification flow)
- **US4 (Phase 8)**: Depends on US2 (extends tarification with prepaid)
- **US7 (Phase 9)**: Depends on US2 (needs usage counters populated)
- **Polish (Phase 10)**: Depends on all user stories

### User Story Dependencies

```
US6 (Countries/Operators) ──┬──→ US1 (Tariff Plans) ──┬──→ US2 (Tarification) ──┬──→ US3 (Recalc)
                             │                          │                          ├──→ US4 (Prepaid)
                             └──→ US5 (Sender IDs) ─────┘                          └──→ US7 (Stats)
```

### Within Each User Story

- Repositories before services
- Services before gRPC handlers
- gRPC handlers before admin-gateway handlers
- Admin-gateway handlers before route registration

### Parallel Opportunities

- T005/T006: Migrations can be written in parallel
- T009-T017: All domain entities in parallel
- T021-T023: Routing domain entities in parallel
- T025-T027: Routing repositories in parallel
- T031-T032: Admin handlers in parallel
- T035-T038: Tarification repositories in parallel
- T050-T051: Usage counter + log repositories in parallel
- T052-T053: Strategy implementations in parallel
- US3 and US4 can run in parallel after US2

---

## Parallel Example: User Story 2

```bash
# Launch repositories in parallel:
Task: "Implement UsageCounterRepository in internal/services/tarification/infrastructure/repository/usage_counter_repository.go"
Task: "Implement TarificationLogRepository in internal/services/tarification/infrastructure/repository/tarification_log_repository.go"

# Launch strategies in parallel:
Task: "Implement FixedStrategy in internal/services/tarification/application/strategy_fixed.go"
Task: "Implement ThresholdStrategy in internal/services/tarification/application/strategy_threshold.go"
```

---

## Implementation Strategy

### MVP First (US6 + US1 + US2)

1. Complete Phase 1: Setup (proto, migrations, skeleton)
2. Complete Phase 2: Foundational (domain entities, interfaces)
3. Complete Phase 3: US6 — Countries & Operators
4. Complete Phase 4: US1 — Tariff Plan Setup
5. Complete Phase 6: US2 — Basic Tarification (fixed + threshold)
6. **STOP and VALIDATE**: End-to-end flow — create country → operator → tariff plan → send SMS → verify billing

### Incremental Delivery

1. Setup + Foundational → Infrastructure ready
2. US6 → Operator management works → Demo
3. US1 → Tariff plan configuration works → Demo
4. US5 → Sender ID registration works → Demo
5. US2 → Core tarification works (fixed + threshold) → **MVP Release**
6. US3 → Recalculation added → Release
7. US4 → Prepaid strategy added → Release
8. US7 → Monitoring dashboard → Release
9. Polish → Production-ready → Final Release

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US6 is P2 in spec but moved to Phase 3 because US1/US2 depend on operators existing
- Saga implementation (T054) is critical path — validates billing integration pattern
- Total: 80 tasks across 10 phases
