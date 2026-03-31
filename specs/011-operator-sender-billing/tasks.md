---
description: "Task list for 011-operator-sender-billing"
---

# Tasks: Operator Sender Name Billing

**Input**: Design documents from `/specs/011-operator-sender-billing/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Organization**: Tasks grouped by user story — each story is independently testable and deliverable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no shared state)
- **[Story]**: Maps to user story from spec.md (US1–US4)
- Exact file paths included in all descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Verify and prepare shared dependencies before any implementation begins

- [X] T001 Check if `shopspring/decimal` is in `go.mod`; if absent, store `monthly_tariff_amount` as string (`"1500.000000"`) and document the decision in `specs/011-operator-sender-billing/research.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: DB migrations and proto contract changes MUST be complete before any service implementation

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T002 [P] Create `migrations/000065_operator_tariff.up.sql` — `ALTER TABLE operators ADD COLUMN monthly_tariff_amount NUMERIC(20,6) DEFAULT NULL`
- [X] T003 [P] Create `migrations/000065_operator_tariff.down.sql` — `ALTER TABLE operators DROP COLUMN IF EXISTS monthly_tariff_amount`
- [X] T004 [P] Create `migrations/000066_sender_name_billing.up.sql` — full DDL for `sender_name_billing_records` with PRIMARY KEY, UNIQUE(sender_registration_id, billing_month), and four indexes per data-model.md §2
- [X] T005 [P] Create `migrations/000066_sender_name_billing.down.sql` — `DROP TABLE IF EXISTS sender_name_billing_records`
- [X] T006 [P] Update `api/proto/routing/routing.proto` — add `string monthly_tariff_amount = 10` to `Operator`, field 6 to `CreateOperatorRequest`, field 7 to `UpdateOperatorRequest` per `contracts/routing-proto-changes.md`
- [X] T007 Regenerate Go code from `api/proto/routing/routing.proto` via protoc into `api/proto/routingv1/` (depends on T006)
- [X] T008 [P] Update `api/proto/tarification/tarification.proto` — add `SenderNameBillingRecordProto`, `CreateSenderBillingRecordRequest/Response`, `ListSenderBillingRecordsRequest/Response` messages and two new RPCs per `contracts/tarification-proto-changes.md`
- [X] T009 Regenerate Go code from `api/proto/tarification/tarification.proto` via protoc into `api/proto/tarificationv1/` (depends on T008)

**Checkpoint**: Migrations and generated proto stubs are ready — all service implementation can begin

---

## Phase 3: User Story 1 — Настройка стратегии регистрации и тарифа оператора (Priority: P1) 🎯 MVP

**Goal**: Admin can set `monthly_tariff_amount` on any operator via the admin UI and API; system stores and displays the tariff correctly.

**Independent Test**: Create an operator with `supports_paid_sender=true` and set `monthly_tariff_amount=1500.00`; verify the field is stored and returned by GET operator; create another with `supports_paid_sender=false` and verify `monthly_tariff_amount` remains NULL.

- [X] T010 [P] [US1] Add `MonthlyTariffAmount *decimal.Decimal` (or `*string`) to struct in `internal/services/routing/domain/operator.go`; add `HasPaidRegistration()` helper per data-model.md §5
- [X] T011 [P] [US1] Update `internal/services/routing/infrastructure/repository/operator_repository.go` — include `monthly_tariff_amount` in SELECT, INSERT and UPDATE queries
- [X] T012 [US1] Update `internal/services/routing/grpc/server.go` — map `monthly_tariff_amount` between domain model and proto `Operator`/`CreateOperatorRequest`/`UpdateOperatorRequest` messages (depends on T007, T010, T011)
- [X] T013 [US1] Update `internal/gateway/admin/handlers/operator.go` — accept `monthly_tariff_amount` in create/update request body; include it in operator response JSON (depends on T012)
- [X] T014 [P] [US1] Update `frontend/src/pages/admin/operators/OperatorForm.tsx` — add `monthly_tariff_amount` numeric input field; show/hide it based on `supports_paid_sender` toggle
- [X] T015 [P] [US1] Update `frontend/src/pages/admin/operators/OperatorDetail.tsx` — display `monthly_tariff_amount` (formatted as RUB) when operator supports paid registration

**Checkpoint**: US1 is fully functional — admin can configure operator tariffs end-to-end

---

## Phase 4: User Story 2 — Регистрация платного имени отправителя (Priority: P2)

**Goal**: Client registering a paid sender name sees the monthly cost inline before confirming; system creates a billing record for the current calendar month on confirmation.

**Independent Test**: Use a paid-strategy operator; open sender name registration form — verify cost displays before submit; submit and verify a `sender_name_billing_records` row is created with correct `billing_month` (first of current month) and `amount` matching operator tariff. Then attempt registration with a free-only operator — verify error.

- [X] T016 [P] [US2] Create `internal/services/tarification/domain/sender_billing.go` — `SenderNameBillingRecord` struct and `BillingMonthKey(t time.Time) time.Time` helper per data-model.md §3
- [X] T017 [P] [US2] Update `internal/services/tarification/domain/interfaces.go` — add `SenderBillingRepository` interface with `Create`, `CreateIfNotExists`, `ListByRegistration`, `ListByClient` methods per data-model.md §3
- [X] T018 [US2] Create `internal/services/tarification/infrastructure/repository/sender_billing_repository.go` — implement `SenderBillingRepository`; `CreateIfNotExists` uses `INSERT ... ON CONFLICT DO NOTHING` (depends on T004, T016, T017)
- [X] T019 [US2] Create `internal/services/tarification/application/sender_billing_service.go` — `CreateBillingRecord` and `ListBillingRecords` methods (depends on T016, T017, T018)
- [X] T020 [US2] Update `internal/services/tarification/grpc/server.go` — implement `CreateSenderBillingRecord` and `ListSenderBillingRecords` RPCs per `contracts/tarification-proto-changes.md` (depends on T009, T019)
- [X] T021 [US2] Update `internal/gateway/portal/handlers/sender_names.go` — add `GET /api/operators/:id/sender-tariff` endpoint: call routing-service gRPC `GetOperator`, return `monthly_tariff_amount` + `currency: "RUB"` per `contracts/tarification-proto-changes.md` §HTTP REST
- [X] T022 [US2] Update `internal/gateway/portal/handlers/sender_names.go` — on paid sender name registration success, call tarification-service gRPC `CreateSenderBillingRecord` with `sender_registration_id`, `client_id`, `operator_id`, `amount` (operator tariff) (depends on T020, T021)
- [X] T023 [US2] Update `frontend/src/pages/portal/sender-names/SenderNameCreate.tsx` — when paid type selected, fetch `GET /api/operators/:id/sender-tariff` and display `monthly_tariff_amount` inline (e.g. "Стоимость: 1 500,00 ₽/мес") before submit button

**Checkpoint**: US2 is fully functional — paid registration creates billing record and shows cost inline

---

## Phase 5: User Story 3 — Ежемесячное продление оплаты за платные имена (Priority: P2)

**Goal**: Monthly cron goroutine inside tarification-service automatically charges all active paid sender names at the start of each new calendar month; process is idempotent.

**Independent Test**: Insert active paid `sender_registrations` rows; manually invoke the scheduler run function; verify `sender_name_billing_records` rows created for current month. Run again — verify no duplicate rows (INSERT ON CONFLICT DO NOTHING). Mark one registration inactive; run — verify no billing record created for it.

- [X] T024 [US3] Create `internal/services/tarification/application/billing_scheduler.go` — `BillingScheduler` struct with `Start()` / `Stop()` / `run()` following pattern from `messaging-service/scheduler.go`; `run()` queries all `sender_registrations WHERE type='paid' AND status='active'`, fetches operator tariff via routing-service gRPC, calls `SenderBillingRepository.CreateIfNotExists` for current `BillingMonthKey`; sleeps until next 1st of month 00:01 UTC (depends on T019)
- [X] T025 [US3] Wire `BillingScheduler` into tarification-service startup/shutdown — call `scheduler.Start()` in `main.go` or service initializer and `scheduler.Stop()` on signal (depends on T024)

**Checkpoint**: US3 is fully functional — monthly billing runs automatically and is idempotent

---

## Phase 6: User Story 4 — Просмотр информации о тарификации имён (Priority: P3)

**Goal**: Clients see monthly cost and next billing date per paid sender name in the list; billing history is accessible per registration.

**Independent Test**: With existing billing records, open sender names list — verify `amount` column and next billing date displayed. Open `SenderNameBillingHistory` page — verify all billing records listed in DESC order by `billing_month`.

- [X] T026 [P] [US4] Update `internal/gateway/portal/handlers/sender_names.go` — add `GET /api/sender-registrations/:id/billing` endpoint: call tarification-service gRPC `ListSenderBillingRecords`, return paginated JSON per `contracts/tarification-proto-changes.md` §HTTP REST (depends on T020)
- [X] T027 [P] [US4] Update `frontend/src/pages/portal/sender-names/SenderNameList.tsx` — add "Стоимость/мес" column (from operator tariff) and "Следующее начисление" column (first day of next month) for paid sender names
- [X] T028 [US4] Create `frontend/src/pages/portal/sender-names/SenderNameBillingHistory.tsx` — page listing billing records for a specific sender registration; fetches `GET /api/sender-registrations/:id/billing`; shows `billing_month` (formatted), `amount` (RUB), `created_at` (depends on T026)

**Checkpoint**: US4 complete — all billing information is visible to clients and admins

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Observability and final validation

- [X] T029 [P] Add Prometheus counter `billing_records_created_total` (label: `type=initial|monthly`) in `internal/services/tarification/application/sender_billing_service.go` and `billing_scheduler.go`
- [X] T030 [P] Add Prometheus counter `billing_scheduler_run_total` (label: `status=success|error`) in `internal/services/tarification/application/billing_scheduler.go`
- [X] T031 Run quickstart.md validation scenarios manually: apply migrations, deploy routing-service + tarification-service + gateways, verify end-to-end flow per quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — **BLOCKS all user stories**
- **Phase 3 (US1)**: Depends on Phase 2 (T007 for proto, T002 for migration)
- **Phase 4 (US2)**: Depends on Phase 2 (T004, T009 for proto); requires US1 for operator tariff to exist
- **Phase 5 (US3)**: Depends on Phase 4 (T019 SenderBillingService)
- **Phase 6 (US4)**: Depends on Phase 4 (T020 gRPC server)
- **Phase 7 (Polish)**: Depends on all prior phases

### User Story Dependencies

| Story | Depends on | Can parallelize with |
|-------|-----------|---------------------|
| US1 (P1) | Phase 2 complete | — |
| US2 (P2) | Phase 2 complete; US1 for operator tariff | US1 backend after T012 |
| US3 (P2) | US2 (T019 SenderBillingService) | US4 frontend |
| US4 (P3) | US2 (T020 gRPC) | US3 |

### Within Each Story

- Domain/interface tasks before repository and service tasks
- Proto regeneration before gRPC server implementation
- Backend gRPC server before gateway handler
- Gateway handler before frontend integration

---

## Parallel Opportunities

```bash
# Phase 2 — all T002–T005 in parallel (different files):
Task: "migrations/000065_operator_tariff.up.sql"
Task: "migrations/000065_operator_tariff.down.sql"
Task: "migrations/000066_sender_name_billing.up.sql"
Task: "migrations/000066_sender_name_billing.down.sql"

# Phase 2 — proto changes in parallel (different services):
Task: "api/proto/routing/routing.proto (T006)"
Task: "api/proto/tarification/tarification.proto (T008)"

# Phase 3 — US1 domain + repo in parallel:
Task: "internal/services/routing/domain/operator.go (T010)"
Task: "internal/services/routing/infrastructure/repository/operator_repository.go (T011)"

# Phase 3 — US1 frontend in parallel:
Task: "frontend/.../OperatorForm.tsx (T014)"
Task: "frontend/.../OperatorDetail.tsx (T015)"

# Phase 4 — US2 domain + interface in parallel:
Task: "internal/services/tarification/domain/sender_billing.go (T016)"
Task: "internal/services/tarification/domain/interfaces.go (T017)"

# Phase 7 — metrics in parallel:
Task: "billing_records_created_total counter (T029)"
Task: "billing_scheduler_run_total counter (T030)"
```

---

## Implementation Strategy

### MVP First (User Story 1 + 2)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational (T002–T009) — **cannot skip**
3. Complete Phase 3: US1 (T010–T015)
4. **VALIDATE**: Admin can configure operator tariffs end-to-end
5. Complete Phase 4: US2 (T016–T023)
6. **VALIDATE**: Paid registration creates billing record; cost shown inline
7. **DEMO/DEPLOY** — core monetization flow is live

### Incremental Delivery

1. Phase 1 + 2 → Foundation ready
2. Phase 3 (US1) → Admin tariff config ✅
3. Phase 4 (US2) → Paid registration + billing record ✅ (MVP!)
4. Phase 5 (US3) → Monthly auto-billing ✅
5. Phase 6 (US4) → Billing history views ✅
6. Phase 7 → Observability ✅

---

## Notes

- **No test tasks generated** — tests are optional and not requested in spec.md
- Idempotency guaranteed by UNIQUE(sender_registration_id, billing_month) at DB level — no app-level checks needed
- `shopspring/decimal` vs string: resolve in T001 before any domain implementation
- Billing scheduler pattern: follow `messaging-service/scheduler.go` (Start/Stop/run) per research.md §3
- Proto wire format: all decimal amounts transmitted as strings (e.g. `"1500.000000"`) — no float64 in proto
