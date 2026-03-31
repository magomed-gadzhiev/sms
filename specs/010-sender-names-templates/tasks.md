# Tasks: Sender Names & Message Templates Registration

**Input**: Design documents from `/specs/010-sender-names-templates/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/sender_name.proto ✅, quickstart.md ✅

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. No test tasks — not requested in spec.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no shared state)
- **[Story]**: Which user story this task belongs to
- Each task includes exact file path

---

## Phase 1: Setup (Proto & Migrations)

**Purpose**: Create gRPC contract and DB migration files before any Go code is written. These are the canonical source of truth for all downstream work.

- [ ] T001 Create api/proto/sender-name/sender_name.proto from specs/010-sender-names-templates/contracts/sender_name.proto (copy verbatim)
- [ ] T002 [P] Modify api/proto/template/template.proto — add `optional string sender_name_id = 4` to CreateTemplateRequest; add `string sender_name_id = 13` and `string sender_name = 14` to TemplateInfo
- [ ] T003 Create migrations/000064_sender_names.up.sql — CREATE TABLE sender_names, CREATE TABLE sender_name_status_history, ALTER TABLE templates ADD COLUMN sender_name_id, and all indexes per data-model.md
- [ ] T004 Create migrations/000064_sender_names.down.sql — DROP INDEX / ALTER TABLE templates DROP COLUMN / DROP TABLE sender_name_status_history / DROP TABLE sender_names (reverse order)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core domain, persistence, business logic, and gRPC server registration. All user stories depend on this phase.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T005 Run `protoc` to generate Go code from api/proto/sender-name/sender_name.proto into api/proto/sender-name-v1/ (add to Makefile / generation script if one exists)
- [ ] T006 Create internal/services/template/domain/sender_name.go — SenderName struct, SenderNameStatusHistory struct, status constants (pending/approved/rejected/deactivated), actor type constants (client/admin/system), ValidateSenderName func (alphanumeric `^[A-Za-z0-9 ]{1,11}$` and numeric `^\d{1,15}$` per GSM 03.40), IsValidTransition func that enforces allowed state machine transitions
- [ ] T007 Add SenderNameRepository interface to internal/services/template/application/ports.go — methods: Create, GetByID, GetByClientAndName, ListByClient, UpdateStatus, ListAll, AddHistoryEntry, GetHistory
- [ ] T008 Create internal/services/template/infrastructure/repository/sender_name_repo.go — pgx implementation of SenderNameRepository; use uuid.UUID for IDs; handle UNIQUE(client_id, name) conflict → ErrDuplicateSenderName
- [ ] T009 Create internal/services/template/application/sender_name_service.go — RegisterSenderName (validate format + uniqueness, insert, write history entry, publish Kafka `sender_name.status_changed`), UpdateSenderName (only if status=rejected), ResubmitSenderName (rejected→pending + history + Kafka), ApproveSenderName (pending→approved + history + Kafka), RejectSenderName (pending→rejected + reason + history + Kafka), DeactivateSenderName (approved→deactivated + history + Kafka), GetSenderName, ListSenderNames, ListAllSenderNames, GetSenderNameHistory
- [ ] T010 Create internal/services/template/grpc/sender_name_handler.go — implement all 10 SenderNameService gRPC RPCs delegating to SenderNameService; map domain errors to gRPC status codes (codes.AlreadyExists, codes.InvalidArgument, codes.NotFound, codes.FailedPrecondition)
- [ ] T011 Register SenderNameService gRPC server in cmd/services/template-service/main.go — call `sendernamev1.RegisterSenderNameServiceServer(grpcServer, senderNameHandler)`

**Checkpoint**: `go build ./...` passes; template-service starts and exposes SenderNameService gRPC

---

## Phase 3: User Story 1 — Регистрация имени отправителя (Priority: P1) 🎯 MVP

**Goal**: Operator can register a sender name and see it in the list with "pending" status.

**Independent Test**: POST /api/v1/sender-names creates a record; GET /api/v1/sender-names returns it; invalid names (>11 chars, Cyrillic, duplicate) return 422.

- [ ] T012 [US1] Add `SenderNameServiceClient` field to internal/gateway/portal/clients.go — instantiate gRPC client to template-service (reuse existing connection or add new dial)
- [ ] T013 [US1] Create internal/gateway/portal/handlers/sender_names.go — handlers: `CreateSenderName` (POST /api/v1/sender-names), `ListSenderNames` (GET /api/v1/sender-names?status=&limit=&offset=); extract client_id from JWT claims
- [ ] T014 [US1] Register routes in internal/gateway/portal/router/router.go: `POST /api/v1/sender-names` → CreateSenderName, `GET /api/v1/sender-names` → ListSenderNames
- [ ] T015 [P] [US1] Add senderNames API functions to portal-frontend/src/api/client.ts — `createSenderName(name: string)`, `listSenderNames(params)` returning typed response with SenderNameInfo[]
- [ ] T016 [US1] Create portal-frontend/src/pages/sender-names/SenderNamesPage.tsx — table of sender names with status badges (pending/approved/rejected/deactivated), "Зарегистрировать имя" button → Modal with input field; client-side validation: max 11 chars alphanumeric or max 15 digits, no pure-space names; show API error messages inline
- [ ] T017 [US1] Add /sender-names route and navigation link to portal frontend router and sidebar nav

**Checkpoint**: Operator can register a sender name and see it listed with status "pending"

---

## Phase 4: User Story 2 — Рассмотрение заявок администратором (Priority: P2)

**Goal**: Admin can view all sender name requests and approve/reject them with a reason.

**Independent Test**: Admin calls POST /api/v1/admin/sender-names/{id}/approve → status becomes "approved"; POST /reject with reason → status "rejected" and reason visible.

- [ ] T018 [US2] Add `SenderNameServiceClient` field to internal/gateway/admin/clients.go
- [ ] T019 [US2] Create internal/gateway/admin/handlers/sender_names.go — handlers: `ListAllSenderNames` (GET /api/v1/admin/sender-names?status=&client_id=&name_query=), `ApproveSenderName` (POST /api/v1/admin/sender-names/{id}/approve), `RejectSenderName` (POST /api/v1/admin/sender-names/{id}/reject, body: `{"reason": "..."}`), `DeactivateSenderName` (POST /api/v1/admin/sender-names/{id}/deactivate, body: `{"reason": "..."}`)
- [ ] T020 [US2] Register admin routes in internal/gateway/admin/router/router.go: GET/POST routes for /api/v1/admin/sender-names and sub-paths
- [ ] T021 [P] [US2] Add admin senderNames API to admin frontend API client (same pattern as portal client.ts)
- [ ] T022 [US2] Create portal-frontend/src/pages/admin/SenderNamesAdminPage.tsx — data table with all sender name requests, status filter dropdown (all/pending/approved/rejected/deactivated), columns: name, client, status, created_at, reviewed_at; action buttons: Одобрить, Отклонить (with reason input modal), Деактивировать (with reason input modal)
- [ ] T023 [US2] Add /admin/sender-names route and navigation link in admin frontend

**Checkpoint**: Admin can see pending requests and approve/reject them; status change visible to operator

---

## Phase 5: User Story 3 — Управление шаблонами сообщений (Priority: P3)

**Goal**: Operator can create templates linked to an approved sender name; template requires admin approval before use.

**Independent Test**: With an approved sender name, operator creates a template with sender_name_id; template is created with pending status; linking to a non-approved or foreign sender name returns 422.

- [ ] T024 [US3] Modify internal/services/template/application/template_service.go — in CreateTemplate and UpdateTemplate: if sender_name_id is provided, call SenderNameRepository.GetByID, verify same client_id and status=approved; return ErrSenderNameNotApproved otherwise
- [ ] T025 [US3] Modify internal/services/template/infrastructure/repository/template_repo.go — update ListTemplates and GetTemplate queries to LEFT JOIN sender_names ON templates.sender_name_id = sender_names.id and return sender_name string for display
- [ ] T026 [US3] Update portal-gateway template HTTP handler (internal/gateway/portal/handlers/templates.go) to forward sender_name_id in create/update requests and include sender_name in list/detail responses
- [ ] T027 [P] [US3] Add `getApprovedSenderNames()` API function to portal-frontend/src/api/client.ts (calls GET /api/v1/sender-names?status=approved)
- [ ] T028 [US3] Add sender name dropdown to template creation/edit modal in portal-frontend/src/pages/templates/TemplatesPage.tsx — load approved sender names; show selected sender name in template list row; display template approval status badge

**Checkpoint**: Operator with an approved sender name can create templates linked to it; templates show sender name; linking to non-approved sender name returns error

---

## Phase 6: User Story 4 — Просмотр статусов и истории (Priority: P4)

**Goal**: Operator can view status history for each sender name and resubmit rejected names after editing.

**Independent Test**: GET /api/v1/sender-names/{id}/history returns all history entries; PUT /api/v1/sender-names/{id} updates name (only if rejected); POST /api/v1/sender-names/{id}/resubmit changes status to pending.

- [ ] T029 [US4] Add handlers to internal/gateway/portal/handlers/sender_names.go — `GetSenderName` (GET /api/v1/sender-names/{id}), `GetSenderNameHistory` (GET /api/v1/sender-names/{id}/history), `UpdateSenderName` (PUT /api/v1/sender-names/{id}, only rejected), `ResubmitSenderName` (POST /api/v1/sender-names/{id}/resubmit)
- [ ] T030 [US4] Register detail/history/resubmit routes in internal/gateway/portal/router/router.go
- [ ] T031 [P] [US4] Add API functions to portal-frontend/src/api/client.ts — `getSenderName(id)`, `getSenderNameHistory(id)`, `updateSenderName(id, name)`, `resubmitSenderName(id)`
- [ ] T032 [US4] Extend portal-frontend/src/pages/sender-names/SenderNamesPage.tsx — clicking a row opens detail panel/modal with: current status + rejection_reason for rejected names, "Редактировать и повторить" button (shows edit form, calls PUT then POST /resubmit), status history timeline showing old_status→new_status, actor_type, comment, date for each entry

**Checkpoint**: Operator sees rejection reason, can edit rejected name and resubmit; history timeline shows all status changes

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Observability, navigation consistency, and final build validation.

- [ ] T033 [P] Add Prometheus counters/histograms to internal/services/template/application/sender_name_service.go — `sender_name_registrations_total`, `sender_name_status_changes_total{status}`, `sender_name_grpc_duration_seconds`
- [ ] T034 [P] Add admin frontend SenderNamesAdminPage to admin panel navigation (sidebar link and route guard)
- [ ] T035 Run `go build ./...` and `go vet ./...` to verify no compilation errors; run `go test ./internal/services/template/...` to verify unit tests pass

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately; T001, T002, T003, T004 all parallelizable
- **Phase 2 (Foundational)**: Depends on Phase 1 (needs proto file and migration files) — **BLOCKS** all user stories
  - T005 depends on T001 (proto file must exist to generate)
  - T006–T009 depend on T003 (migration confirms schema)
  - T010 depends on T005 (generated proto types) and T009 (service)
  - T011 depends on T010
- **Phase 3 (US1)**: Depends on Phase 2 completion
- **Phase 4 (US2)**: Depends on Phase 2 completion; independent of Phase 3
- **Phase 5 (US3)**: Depends on Phase 2; requires US1+US2 approved path to exist for end-to-end test
- **Phase 6 (US4)**: Depends on Phase 2; resubmit logic already in T009 (service); only gateway handlers and frontend are new
- **Phase 7 (Polish)**: Depends on all desired stories being complete

### User Story Dependencies

- **US1 (P1)**: After Phase 2 — no US dependencies
- **US2 (P2)**: After Phase 2 — no US dependencies (can work in parallel with US1)
- **US3 (P3)**: After Phase 2 — needs at least one approved sender name (US1+US2) to test end-to-end
- **US4 (P4)**: After Phase 2 — no US dependencies for backend; needs US1 data for meaningful UI test

### Within Each Phase

- Domain model (T006) → repository interface (T007) → repository impl (T008) → service (T009) → gRPC handler (T010) → service registration (T011)
- Gateway handler → router registration → frontend API → frontend UI

---

## Parallel Execution Examples

### Phase 2 Parallel Start

```bash
# After T001 (proto file created):
Task: T005 — generate Go protobuf code
Task: T006 — create domain model

# T005 and T006 can run simultaneously (different files)
```

### Phase 3 + Phase 4 Parallel (after Phase 2)

```bash
# Two developers can work in parallel:
Developer A: T012 → T013 → T014 → T015 → T016 → T017  (US1 — portal operator flow)
Developer B: T018 → T019 → T020 → T021 → T022 → T023  (US2 — admin flow)
```

### Within US1

```bash
# After T013 (handler created) and T014 (routes registered):
Task: T015 — add API functions to frontend client.ts
Task: (can start T016 UI work in parallel with T015)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (proto + migrations)
2. Complete Phase 2: Foundational — domain, repo, service, gRPC, registration
3. Complete Phase 3: US1 — portal operator create + list flow
4. **STOP and VALIDATE**: Operator can register name, see status "pending" in list
5. Deploy/demo if ready

### Incremental Delivery

1. Phase 1 + 2 → Core backend ready
2. + Phase 3 → Operator can register names (MVP!)
3. + Phase 4 → Admin can approve/reject (full workflow end-to-end)
4. + Phase 5 → Templates linked to sender names
5. + Phase 6 → History and resubmit flow
6. + Phase 7 → Metrics and polish

---

## Notes

- Proto file already designed in specs/contracts/sender_name.proto — copy to api/proto/ location in T001
- Template variable format is `{{var}}` (double braces) — consistent with existing template-service; spec mentions `{var}` but research confirms `{{var}}`
- Sender name UNIQUE constraint is across all statuses — a client cannot re-register a name that exists in any state (including rejected/deactivated); they must resubmit
- ON DELETE SET NULL on templates.sender_name_id — templates survive sender name deactivation
- sender_name_status_history is append-only — never update or delete rows
- Kafka topic: `sender_name.status_changed` — publish on every status transition including initial creation (system, null→pending)
