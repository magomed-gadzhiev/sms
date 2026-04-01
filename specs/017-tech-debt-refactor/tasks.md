# Tasks: Technical Debt Resolution

**Input**: Design documents from `/specs/017-tech-debt-refactor/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, quickstart.md ✅

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US6)

---

## Phase 1: Setup (Baseline Audit)

**Purpose**: Verify baseline compiles and gather data needed before US4 (dead code removal).

- [X] T001 Verify project compiles: `go build ./...` from repo root
- [X] T002 [P] Audit SendMessage usages: `grep -rn "\.SendMessage(" internal/services/provider/` — document whether it is called via the `application.Connection` interface

**Checkpoint**: Baseline confirmed; US4 scope determined by T002 audit result

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: No shared infrastructure changes required — each user story is independently executable.

**⚠️ NOTE**: US3 creates `internal/shared/dlr/status.go` which US5 imports. US3 must complete before US5 modifies `dlr_service.go` to use the new constants.

---

## Phase 3: User Story 1 — Устранение дублирования кода в бэкенде (Priority: P1) 🎯 MVP

**Goal**: Eliminate duplicated `uuid.Parse(req.ClientId)` pattern across 8+ gRPC methods by extracting shared helper functions.

**Independent Test**: `grep -c "uuid.Parse" internal/services/messaging/grpc/server.go` returns 0; all validation routes through the helper.

- [X] T003 [P] [US1] Create helper `parseClientID(raw string) (uuid.UUID, error)` in `internal/services/messaging/grpc/helpers.go` (new file) — returns gRPC status errors (InvalidArgument for empty/invalid)
- [X] T004 [P] [US1] Create helper `parseClientID(raw string) (uuid.UUID, error)` in `internal/services/client/grpc/helpers.go` (new file or add to existing helper file) — same signature and error types
- [X] T005 [US1] Refactor all methods in `internal/services/messaging/grpc/server.go` to call `parseClientID(req.ClientId)` instead of inline uuid.Parse blocks (SendMessage line 52, SendBatch, GetMessageHistory, GetMessageStatus and all other methods)
- [X] T006 [US1] Refactor all methods in `internal/services/client/grpc/server.go` to call `parseClientID(req.ClientId)` instead of inline uuid.Parse blocks (CreateClient, UpdateClient, DeleteClient, GetClient and others)

**Checkpoint**: US1 complete — `grep -c "uuid.Parse" internal/services/messaging/grpc/server.go` returns 0; `go build ./...` passes

---

## Phase 4: User Story 2 — Исправление проблем с производительностью и утечками памяти (Priority: P1)

**Goal**: Replace N+1 GetHealth loop with a single batch SQL query; add PurgeStaleRoutes to prevent unbounded RoundRobin map growth.

**Independent Test**:
- N+1 fix: `grep -n "GetHealth(ctx" internal/services/routing/application/selection_strategy.go` returns 0
- Memory fix: `PurgeStaleRoutes` exists in `RoundRobinSelector` and is called in `RoutingService`

- [X] T007 [US2] Add `GetHealthBatch(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*ProviderHealth, error)` to `ProviderRepository` interface in `internal/services/routing/domain/repository.go`
- [X] T008 [US2] Implement `GetHealthBatch` in `internal/services/routing/infrastructure/postgres/provider_repo.go` using `SELECT ... FROM provider_health WHERE provider_id = ANY($1)` — verify actual table/column names in the file first
- [X] T009 [US2] Replace N+1 `GetHealth` loop with single `GetHealthBatch` call in `LeastLoadedSelector.SelectProvider` in `internal/services/routing/application/selection_strategy.go` (lines 108–117)
- [X] T010 [P] [US2] Add `PurgeStaleRoutes(activeRouteIDs []uuid.UUID)` method to `RoundRobinSelector` in `internal/services/routing/application/selection_strategy.go` — deletes `currentIndex` entries whose key is not in `activeRouteIDs`
- [X] T011 [US2] Call `s.roundRobin.PurgeStaleRoutes(activeRouteIDs)` in `internal/services/routing/application/routing_service.go` after loading the active route list (wherever routes are refreshed)

**Checkpoint**: US2 complete — batch SQL confirmed; `go test ./internal/services/routing/...` passes

---

## Phase 5: User Story 3 — Консолидация констант и устранение магических значений (Priority: P2)

**Goal**: Replace hardcoded DLR status strings ("DELIVRD", "EXPIRED", etc.) with named constants from a central package.

**Independent Test**: `grep -rn '"DELIVRD"\|"EXPIRED"\|"REJECTD"\|"UNDELIV"' internal/ --include="*.go" | grep -v "shared/dlr/status.go"` returns no output.

- [X] T012 [US3] Create `internal/shared/dlr/status.go` with `type DLRStatus string` and constants `DLRStatusDelivered = "DELIVRD"`, `DLRStatusExpired = "EXPIRED"`, `DLRStatusRejected = "REJECTD"`, `DLRStatusUndeliv = "UNDELIV"`; add `IsTerminal(s DLRStatus) bool`
- [X] T013 [US3] Replace all magic DLR string literals in `internal/services/messaging/application/dlr_service.go` (switch block lines 98–103 and any other occurrences) with constants from `internal/shared/dlr` package

**Checkpoint**: US3 complete — grep for hardcoded DLR strings returns 0 outside the new shared package

---

## Phase 6: User Story 4 — Удаление мёртвого кода и заглушек (Priority: P2)

**Goal**: Remove the `SendMessage` stub with its misleading panic-like error from `SMPPConnectionAdapter` and its interface.

**Independent Test**: `grep -rn "not implemented\|use SenderService" internal/services/provider/` returns 0.

**⚠️ PREREQUISITE**: Requires T002 audit result before T014.

- [X] T014 [US4] Based on T002 audit — if `SendMessage` has no callers via `application.Connection` interface: remove `SendMessage` from the interface in `internal/services/provider/application/connection.go`; if callers exist: add `// Deprecated` comment with issue reference
- [X] T015 [US4] Remove stub method `func (a *ConnectionAdapter) SendMessage(...)` from `internal/services/provider/infrastructure/smpp/pool_adapter.go` (lines 186–190)

**Checkpoint**: US4 complete — no "not implemented" stubs; `go build ./...` passes

---

## Phase 7: User Story 5 — Улучшение обработки ошибок и устранение тихих сбоев (Priority: P2)

**Goal**: All business-critical errors logged at Error level; no silent failures in DLR publisher or webhook drain.

**Independent Test**: Simulate Kafka publish failure — error appears in logs at ERROR level, not WARN.

**⚠️ PREREQUISITE**: US3 (T012–T013) must be complete before modifying `dlr_service.go` further.

- [X] T016 [US5] Change `log.Warn()` to `log.Error()` for Kafka publish failure in `internal/services/messaging/application/dlr_service.go` line 128; add `dlr_event_publish_errors_total` Prometheus counter increment in the same error branch
- [X] T017 [US5] Add error logging for `io.Copy` drain in `internal/services/webhook/infrastructure/http/delivery_client.go` line 72: capture returned error and log at Debug level if non-nil

**Checkpoint**: US5 complete — SC-004 satisfied; `go build ./...` passes

---

## Phase 8: User Story 6 — Улучшение фронтенда: мемоизация и обработка ошибок (Priority: P3)

**Goal**: All DataTable `columns` arrays and row action handlers wrapped in `useMemo`/`useCallback` across 10 page components.

**Independent Test**: `grep -rn "const columns" portal-frontend/src/pages/ --include="*.tsx"` — all occurrences are inside a `useMemo(` call.

All T018–T027 tasks are parallel (different files):

- [X] T018 [P] [US6] Wrap `columns` in `useMemo` and row handlers in `useCallback` in `portal-frontend/src/pages/campaigns/CampaignsPage.tsx` (columns at line 90)
- [X] T019 [P] [US6] Wrap `columns` and action handlers in `useMemo`/`useCallback` in `portal-frontend/src/pages/campaigns/CampaignDetailPage.tsx`
- [X] T020 [P] [US6] Wrap `columns` in `useMemo` and action handlers in `useCallback` in `portal-frontend/src/pages/messages/MessagesPage.tsx`
- [X] T021 [P] [US6] Wrap `columns` and action handlers in `useMemo`/`useCallback` in `portal-frontend/src/pages/contacts/ContactListDetailPage.tsx`
- [X] T022 [P] [US6] Wrap `columns` in `useMemo` in `portal-frontend/src/pages/audit/AuditLogPage.tsx`
- [X] T023 [P] [US6] Wrap `columns` and action handlers in `useMemo`/`useCallback` in `portal-frontend/src/pages/api-keys/APIKeysPage.tsx`
- [X] T024 [P] [US6] Wrap computed chart/analytics data in `useMemo` in `portal-frontend/src/pages/analytics/AnalyticsPage.tsx`
- [X] T025 [P] [US6] Wrap `columns` and action handlers in `useMemo`/`useCallback` in `portal-frontend/src/pages/webhooks/WebhooksPage.tsx`
- [X] T026 [P] [US6] Wrap `columns` and action handlers in `useMemo`/`useCallback` in `portal-frontend/src/pages/settings/DomainsPage.tsx`
- [X] T027 [P] [US6] Wrap support matrix computation in `useMemo` in `portal-frontend/src/pages/delivery-strategies/OperatorSupportMatrix.tsx`

**Checkpoint**: US6 complete — `cd portal-frontend && npm run build` passes; no bare `const columns` without `useMemo`

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Final validation per quickstart.md checklist

- [X] T028 Run full backend compilation: `go build ./...` from repo root — must have 0 errors
- [X] T029 [P] Run backend tests: `go test ./internal/services/routing/... ./internal/services/messaging/... ./internal/shared/...`
- [X] T030 [P] Verify no hardcoded DLR strings: `grep -rn '"DELIVRD"\|"EXPIRED"\|"REJECTD"\|"UNDELIV"' internal/ --include="*.go" | grep -v "shared/dlr/status.go"` — expect 0 results
- [X] T031 [P] Verify N+1 eliminated: `grep -n "GetHealth(ctx" internal/services/routing/application/selection_strategy.go` — expect 0 results
- [X] T032 [P] Run frontend build: `cd portal-frontend && npm run build` — must pass TypeScript compilation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 3 (US1)**: Independent — can start after Phase 1
- **Phase 4 (US2)**: Independent — can start after Phase 1
- **Phase 5 (US3)**: Independent — can start after Phase 1; **must complete before Phase 7**
- **Phase 6 (US4)**: Requires T002 audit result from Phase 1
- **Phase 7 (US5)**: Requires Phase 5 (US3) complete (modifies the same `dlr_service.go`)
- **Phase 8 (US6)**: Fully independent — can start after Phase 1 (frontend only)
- **Phase 9 (Polish)**: Depends on all desired stories complete

### User Story Dependencies

- **US1** (P1): No dependencies on other stories
- **US2** (P1): No dependencies on other stories
- **US3** (P2): No dependencies; creates shared package consumed by US5
- **US4** (P2): Requires T002 audit; otherwise independent
- **US5** (P2): Depends on **US3** — imports `internal/shared/dlr`
- **US6** (P3): Fully independent (frontend)

### Within Each User Story

- US2: T007 → T008 → T009 sequential; T010 parallel with T007–T009
- US1: T003/T004 parallel; T005/T006 sequential after T003/T004 respectively
- US6: T018–T027 all parallel

### Parallel Opportunities

- US1, US2, US3, US6 can all start in parallel after Phase 1
- US4 can start after T002 completes
- US5 can start after US3 completes
- Within US6: all 10 component tasks (T018–T027) are fully parallel
- Phase 9 tasks T029–T032 are all parallel after T028

---

## Parallel Execution Examples

### Backend P1 stories in parallel (US1 + US2 + US3)

```
Developer A (US1):     Developer B (US2):      Developer C (US3):
T003 helpers.go        T007 repo interface     T012 shared/dlr/status.go
T004 helpers.go        T008 SQL impl           T013 dlr_service constants
T005 messaging server  T009 LeastLoaded fix
T006 client server     T010 PurgeStaleRoutes
                       T011 RoutingService call
```

### Frontend US6 — all 10 files in parallel

```
T018 CampaignsPage        T019 CampaignDetailPage   T020 MessagesPage
T021 ContactListDetail    T022 AuditLogPage          T023 APIKeysPage
T024 AnalyticsPage        T025 WebhooksPage          T026 DomainsPage
T027 OperatorSupportMatrix
```

---

## Implementation Strategy

### MVP First (US1 + US2 only)

1. Complete Phase 1: audit + baseline verify
2. Complete Phase 3 (US1): gRPC helper — `go build` gate
3. Complete Phase 4 (US2): batch health + round-robin — `go build` + `go test` gate
4. **STOP and VALIDATE**: routing tests pass, no N+1 in selection_strategy.go

### Incremental Delivery

1. Phase 1 → US1 → US2 → validate (backend P1 complete, deploy-ready)
2. Add US3 → US4 → US5 → validate (P2 backend complete)
3. Add US6 → validate (frontend P3 complete)
4. Phase 9 polish → final green build

---

## Notes

- No DB migrations, no proto changes, no new Kafka topics
- `go build ./...` must pass after every phase — use as a gate
- T002 (SendMessage audit) is a prerequisite for US4 — do not delete without confirming no callers
- Scheduler Warn at `scheduler.go:113` is intentionally left at Warn (see research.md — architecturally justified)
- `internal/shared/dlr` package — verify Go module import path after creation with `go build ./internal/shared/...`
