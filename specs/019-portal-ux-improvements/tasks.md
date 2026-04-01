# Tasks: Portal UX Improvements for Managers

**Input**: Design documents from `/specs/019-portal-ux-improvements/`
**Prerequisites**: plan.md ✅, spec.md ✅, data-model.md ✅, contracts/portal-api.md ✅

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US8)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Database migration — prerequisite for notification backend (US7). All other stories can begin without waiting for this phase.

- [X] T001 Create migration `migrations/000072_notifications.up.sql` with `notifications` table, indexes `idx_notifications_user_id` and `idx_notifications_unread`
- [X] T002 [P] Create migration `migrations/000072_notifications.down.sql` with `DROP TABLE notifications`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Route registration that unlocks new endpoints; must precede any story's backend handlers going live.

**⚠️ CRITICAL**: Register all new routes before implementing handlers to keep compile-time consistency

- [X] T003 Register new routes in `internal/gateway/portal/router/router.go`: `GET /notifications`, `POST /notifications/{id}/read`, `POST /notifications/read-all`, `GET /search`, `POST /export/start`, `GET /export/{job_id}/status`, `GET /export/{job_id}/download`

**Checkpoint**: Foundation ready — all backend handler stubs can now be added and user stories can begin in parallel

---

## Phase 3: User Story 1 — Визуальный дашборд с графиками и трендами (Priority: P1) 🎯 MVP

**Goal**: Dashboard shows metric trend cards and 3 Recharts charts (line timeline, donut status, bar countries). Analytics page gains interactive charts.

**Independent Test**: Open dashboard — LineChart with 7-day timeline, PieChart with status distribution, BarChart with top countries, and trend arrows (+/-%) on metric cards all render with real data. Hovering shows tooltip values.

### Implementation for User Story 1

- [X] T004 [P] [US1] Extend `internal/gateway/portal/handlers/dashboard.go`: add parallel gRPC call to analytics-service via `errgroup` fetching 7-day timeline, status distribution, top countries, and trend deltas; populate new `charts` field in response
- [X] T005 [P] [US1] Add `DashboardChartData`, `TimelineEntry`, `StatusDistribution`, `TopCountry`, `Trend` TypeScript interfaces and update `getDashboard` return type in `portal-frontend/src/api/client.ts`
- [X] T006 [US1] Update `portal-frontend/src/pages/dashboard/DashboardPage.tsx`: add Recharts `LineChart` (sent/delivered over `timeline_7d`), `PieChart` donut (status_distribution), `BarChart` horizontal (top_countries), trend indicators (↑/↓ with %) on existing metric cards, and `animate-pulse` skeleton states while loading

**Checkpoint**: Dashboard fully functional with charts — User Story 1 testable independently

---

## Phase 4: User Story 2 — Массовые действия над элементами списков (Priority: P1)

**Goal**: Every list table gains checkboxes; selecting items reveals BulkActionBar with delete/export/status-change actions and confirmation dialogs.

**Independent Test**: On Messages page, check 3 rows — BulkActionBar appears with counter and "Экспорт" + "Удалить" buttons. Click "Удалить" — confirmation dialog shows count. Confirm — items removed.

### Implementation for User Story 2

- [X] T007 [P] [US2] Extend `portal-frontend/src/components/data/DataTable.tsx`: add optional `bulkActions?: BulkAction<T>[]` prop, `selectedIds: Set<string>` state, header checkbox with indeterminate state when partially selected, per-row checkboxes
- [X] T008 [P] [US2] Create `portal-frontend/src/components/data/BulkActionBar.tsx`: floating bar visible when `selectedIds.size > 0`, shows selected count, renders action buttons from `bulkActions` prop, Radix UI `Dialog` confirmation for actions with `requiresConfirmation: true`
- [X] T009 [US2] Add bulk actions (export CSV inline, delete with confirmation) to `portal-frontend/src/pages/messages/MessagesPage.tsx` by passing `bulkActions` prop to DataTable
- [X] T010 [P] [US2] Add bulk actions (submit for review → status "На проверке", delete with confirmation) to `portal-frontend/src/pages/templates/TemplatesPage.tsx`
- [X] T011 [P] [US2] Add bulk actions (activate, deactivate, delete with confirmation) to `portal-frontend/src/pages/sender-names/SenderNamesPage.tsx`

**Checkpoint**: Bulk selection and actions work independently on all three list pages

---

## Phase 5: User Story 3 — Улучшенный мастер создания рассылки с навигацией по шагам (Priority: P2)

**Goal**: Campaign wizard shows clickable step progress bar; users can jump back to completed steps; invalid steps show red dot markers; final step shows compact summary.

**Independent Test**: Fill wizard steps 1–3, click step 1 in indicator — jumps to step 1 with data preserved. Leave step 2 invalid while on step 4 — red dot on step 2 indicator.

### Implementation for User Story 3

- [X] T012 [P] [US3] Create `portal-frontend/src/components/campaigns/StepIndicator.tsx`: renders step pills with labels, clickable if `stepIndex <= maxReachedStep`, red dot when `validationErrors[stepKey] === true`, active/completed/upcoming visual states using Tailwind classes
- [X] T013 [US3] Update `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx`: integrate `StepIndicator`, track `maxReachedStep`, implement `handleStepClick(i)` guard, add `validationErrors: Record<WizardStep, boolean>` state that marks steps with failed validation
- [X] T014 [US3] Add compact "Подтверждение" summary step to `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx` showing all params (name, contact list count, message text, schedule, retry settings) before final submit

**Checkpoint**: Campaign wizard free navigation and summary step work independently

---

## Phase 6: User Story 4 — Экспорт данных в CSV (Priority: P2)

**Goal**: Async CSV export via backend for large datasets; progress indicator on frontend; export respects active filters.

**Independent Test**: Filter messages by status "Доставлено", click "Экспорт CSV" — progress shown, file downloads with correct UTF-8 headers matching filter.

### Implementation for User Story 4

- [X] T015 [P] [US4] Create `internal/gateway/portal/handlers/export.go` with three handlers: `StartExport` (generates UUID, stores Redis hash `export:job:{uuid}` with TTL 3600, launches goroutine), export goroutine (paginates gRPC, writes UTF-8 CSV without BOM to `/tmp/export_{uuid}.csv`, updates Redis status), `GetExportStatus` (reads Redis hash), `DownloadExport` (validates `user_id` match, streams file, deletes after send, returns 403/404/409 on errors)
- [X] T016 [P] [US4] Add `startExport`, `getExportStatus`, `downloadExport` functions and `ExportJob` type to `portal-frontend/src/api/client.ts`
- [X] T017 [US4] Add "Экспорт CSV" button to `portal-frontend/src/pages/messages/MessagesPage.tsx`: calls `startExport` with current filters, polls `getExportStatus` every 2s, shows progress indicator, triggers download via `downloadExport` when `status === 'ready'`; button disabled when filtered result count is 0

**Checkpoint**: CSV export for messages with active filters works end-to-end

---

## Phase 7: User Story 5 — Быстрый поиск и навигация / Command Palette (Priority: P2)

**Goal**: Ctrl+K opens command palette with static nav items + live DB search (templates, sender names, contact lists); keyboard navigation supported.

**Independent Test**: Press Ctrl+K, type "шабло" — results show "Шаблоны", "Создать шаблон", and matching template records. ArrowUp/Down navigates, Enter activates, Escape closes.

### Implementation for User Story 5

- [X] T018 [P] [US5] Create `internal/gateway/portal/handlers/search.go`: validates `q` param (min 2 chars), runs 3 parallel gRPC queries (templates, sender_names, contact_lists — ILIKE `%q%`, LIMIT 3 each), merges into `CommandItem` array, returns JSON response within 200ms target
- [X] T019 [P] [US5] Create `portal-frontend/src/hooks/useCommandPalette.ts`: `isOpen` state, `query` with 300ms debounce, `results: CommandItem[]` from `GET /search`, `activeIndex` for keyboard nav, `open/close/navigate/activate` actions
- [X] T020 [P] [US5] Create `portal-frontend/src/components/ui/CommandPalette.tsx`: Radix UI `Dialog`, search input auto-focused on open, static registry of all `NAV_GROUPS` pages + quick-create actions, dynamic results from hook appended below static matches, grouped by `category`, keyboard `ArrowUp/Down` moves `activeIndex`, `Enter` navigates/executes selected item
- [X] T021 [US5] Add `searchRecords` function to `portal-frontend/src/api/client.ts`; add `CommandPalette` to `portal-frontend/src/components/layout/UserLayout.tsx` with `keydown` listener for `Ctrl+K` / `Cmd+K`; filter static items by user role

**Checkpoint**: Command palette opens, searches, and navigates independently of other stories

---

## Phase 8: User Story 6 — Inline-валидация форм с визуальной обратной связью (Priority: P3)

**Goal**: Forms show validation errors on blur (then on each keystroke after first touch); required fields marked; SMS character counter with color thresholds; scroll to first error on submit.

**Independent Test**: Open template creation form, type 1 char in "Имя" — red border + "Минимум 3 символа". Type 150 SMS chars — counter turns yellow. Submit empty required field — all invalid fields highlighted, page scrolls to first.

### Implementation for User Story 6

- [X] T022 [P] [US6] Create `portal-frontend/src/hooks/useFormValidation.ts`: accepts `ValidationRules` map per field (`required`, `minLength`, `maxLength`, `pattern`), tracks `touched` and `errors` state, validates on `onBlur` and on `onChange` if already touched, exposes `fieldProps(name)` returning `{ onChange, onBlur, 'aria-invalid', 'aria-describedby' }`, `validateAll()` for submit (marks all fields touched), `scrollToFirstError()` helper
- [X] T023 [P] [US6] Create `portal-frontend/src/components/ui/CharacterCounter.tsx`: accepts `current`, `max` props; green when `current <= 140`, yellow when `141–160`, red when `161+`; shows remaining count and segment count when multi-segment
- [X] T024 [US6] Apply `useFormValidation` and `CharacterCounter` to `portal-frontend/src/pages/templates/TemplatesPage.tsx`: name field (required, min 3, max 100), body field (required, max 1600) with `CharacterCounter`, call `validateAll()` + `scrollToFirstError()` on submit
- [X] T025 [P] [US6] Apply `useFormValidation` to `portal-frontend/src/pages/sender-names/SenderNamesPage.tsx`: name field (required, min 3, max 11, pattern `^[A-Za-z0-9 \-_.]+$`), call `validateAll()` + `scrollToFirstError()` on submit

**Checkpoint**: Inline validation works in template and sender-name forms independently

---

## Phase 9: User Story 7 — Центр уведомлений (Priority: P3)

**Purpose**: Notifications stored in DB, polled every 30s, shown as bell badge + dropdown panel with read/read-all actions.

**Independent Test**: Complete a campaign, open portal — bell shows "1". Click bell — notification "Рассылка X завершена: 950/1000 доставлено" visible. Click "Отметить все прочитанными" — badge disappears.

### Implementation for User Story 7

*(Requires Phase 1 migration to be applied before backend can run)*

- [X] T026 [P] [US7] Create `internal/gateway/portal/handlers/notifications.go`: `GetNotifications` (SELECT with pagination + COUNT unread WHERE user_id), `MarkNotificationRead` (UPDATE WHERE id AND user_id, return 404 if not found), `MarkAllNotificationsRead` (UPDATE all WHERE user_id)
- [X] T027 [P] [US7] Create `internal/gateway/portal/notifications/scheduler.go`: Start/Stop goroutine with `time.NewTicker(5 * time.Minute)`, each tick queries campaign-service gRPC for campaigns completed in last 10 min, checks `SELECT EXISTS` for existing notification per campaign, inserts `campaign_completed`/`campaign_failed` notification if absent
- [X] T028 [P] [US7] Create `portal-frontend/src/hooks/useNotifications.ts`: fetches `GET /notifications`, stores `items` and `unread_count` in state, sets `setInterval(30_000)` for polling, re-fetches on `document.visibilitychange` when tab becomes visible, exposes `markRead(id)` and `markAllRead()` mutation functions
- [X] T029 [P] [US7] Create `portal-frontend/src/components/ui/NotificationBell.tsx`: bell icon button with badge showing `unread_count` (hidden when 0), uses `useNotifications` hook
- [X] T030 [P] [US7] Create `portal-frontend/src/components/ui/NotificationPanel.tsx`: Radix UI `DropdownMenu`, lists notifications sorted newest-first with type icons, "Отметить все прочитанными" button, each item navigates to linked object (`object_type` + `object_id`) on click, read items shown with muted style
- [X] T031 [US7] Add `NotificationBell` to header in `portal-frontend/src/components/layout/UserLayout.tsx`; initialize `Scheduler.Start()` in `cmd/portal-gateway/main.go` and `Scheduler.Stop()` on graceful shutdown

**Checkpoint**: Notification polling, display, and read actions work end-to-end independently

---

## Phase 10: User Story 8 — Улучшенная аналитика с разбивками и сравнениями (Priority: P3)

**Goal**: Analytics page gains period comparison (two overlaid timelines), breakdowns by country/operator with sortable table, and a Cost tab with daily spend chart and forecast.

**Independent Test**: Enable "Сравнить с предыдущим периодом" — two data series appear on chart with legend. Open "Стоимость" tab — daily bar chart + "Прогноз до конца месяца: X руб." calculated as 7-day rolling average × remaining days.

### Implementation for User Story 8

- [X] T032 [P] [US8] Extend `internal/gateway/portal/handlers/analytics.go`: when `compare=true`, fire second parallel gRPC request for previous period of same length via `errgroup`, merge into `previous_timeline` + `comparison` fields; when `include_cost=true`, fetch cost-by-day from billing-service and calculate `cost_forecast` as (7-day avg) × remaining days
- [X] T033 [P] [US8] Add `AnalyticsDataExtended`, `ComparisonData`, `CostByDay` TypeScript interfaces and update `getAnalytics` params to include `compare?: boolean` and `includeCost?: boolean` in `portal-frontend/src/api/client.ts`
- [X] T034 [US8] Update `portal-frontend/src/pages/analytics/AnalyticsPage.tsx`: add "Сравнение с предыдущим периодом" toggle (passes `compare` param, renders second `Line` on existing chart with legend), add "По странам" tab with sortable table (country, count, %, cost), add "Стоимость" tab with Recharts `BarChart` daily costs + text forecast block

**Checkpoint**: Analytics comparison, country breakdown, and cost forecast all work independently

---

## Phase 11: Polish & Cross-Cutting Concerns

**Purpose**: Observability and final validation

- [X] T035 [P] Add Prometheus counters `portal_notifications_created_total` and `portal_export_jobs_total` to `internal/gateway/portal/handlers/notifications.go` and `internal/gateway/portal/handlers/export.go`
- [X] T036 [P] Add zerolog request logging to new handlers in `internal/gateway/portal/handlers/search.go`, `internal/gateway/portal/handlers/export.go`, `internal/gateway/portal/handlers/notifications.go`
- [ ] T037 Run quickstart.md validation scenarios end-to-end to verify all 8 user stories function correctly

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately (only blocks US7 backend)
- **Foundational (Phase 2)**: Depends on Phase 1 — blocks router registration
- **US1, US2 (P1 stories)**: Can start after Phase 2 — no dependency on each other
- **US3, US4, US5 (P2 stories)**: Can start after Phase 2 — no dependency on P1 stories
- **US6, US7, US8 (P3 stories)**: Can start after Phase 2 — US7 needs Phase 1 migration applied
- **Polish (Phase 11)**: Depends on all desired user stories complete

### User Story Dependencies

- **US1** (Dashboard Charts): Independent after Phase 2
- **US2** (Bulk Actions): Independent after Phase 2; DataTable.tsx modification shared with no other story
- **US3** (Wizard Navigation): Independent — only touches CampaignWizardPage
- **US4** (CSV Export): Independent; export goroutine uses Redis (no DB dependency)
- **US5** (Command Palette): Independent; search.go is a new file
- **US6** (Inline Validation): Independent; new hook + component + form wiring
- **US7** (Notifications): Depends on Phase 1 migration being applied
- **US8** (Analytics): Independent — only extends existing analytics.go and AnalyticsPage

### Within Each User Story

- Backend handler before frontend API client update
- API client types before page/component consuming them
- Hooks before components that use them
- Components before page-level integration

### Parallel Opportunities

- T001 + T002 (migrations): fully parallel
- T004 + T005 (US1 backend + US1 API types): parallel (different files)
- T007 + T008 (US2 DataTable + BulkActionBar): parallel (different files)
- T010 + T011 (US2 templates + sender-names bulk): parallel (different files)
- T012 (US3 StepIndicator) + T015 (US4 export.go) + T018 (US5 search.go): parallel across stories
- T019 + T020 (US5 hook + component): parallel (different files)
- T022 + T023 (US6 hook + CharacterCounter): parallel (different files)
- T026 + T027 + T028 + T029 + T030 (US7 handlers, scheduler, hook, components): all parallel (different files)
- T032 + T033 (US8 backend + types): parallel

---

## Parallel Example: User Story 7 (Notifications)

```bash
# All these tasks touch different files — launch simultaneously:
Task T026: Create internal/gateway/portal/handlers/notifications.go
Task T027: Create internal/gateway/portal/notifications/scheduler.go
Task T028: Create portal-frontend/src/hooks/useNotifications.ts
Task T029: Create portal-frontend/src/components/ui/NotificationBell.tsx
Task T030: Create portal-frontend/src/components/ui/NotificationPanel.tsx
# Then after all complete:
Task T031: Wire NotificationBell into UserLayout + Scheduler into main.go
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 Only)

1. Complete Phase 1: DB migration (T001–T002)
2. Complete Phase 2: Router registration (T003)
3. Complete Phase 3: US1 Dashboard charts (T004–T006)
4. Complete Phase 4: US2 Bulk actions (T007–T011)
5. **STOP and VALIDATE**: Both P1 user stories independently testable
6. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → start US1 + US2 in parallel (P1 priorities)
2. US1 done → deploy dashboard charts as standalone feature
3. US2 done → deploy bulk actions
4. US3 + US4 + US5 → deploy wizard, export, command palette (P2)
5. US6 + US7 + US8 → deploy validation, notifications, analytics (P3)
6. Polish → metrics, logging, validation

### Parallel Team Strategy

With multiple developers after Phase 2:
- Dev A: US1 (T004–T006) + US8 (T032–T034) — analytics/visualization track
- Dev B: US2 (T007–T011) + US6 (T022–T025) — tables/forms track
- Dev C: US4 (T015–T017) + US7 (T026–T031) — backend-heavy track
- Dev D: US3 (T012–T014) + US5 (T018–T021) — navigation/UX track

---

## Notes

- [P] tasks = different files, no shared state dependencies
- [USn] label maps task to user story for traceability
- Phase 1 migration must be **applied** (not just written) before US7 backend handlers run
- UTF-8 without BOM for all CSV output (explicitly required in spec)
- Recharts is confirmed already in project (per assumptions in spec.md)
- Radix UI Dialog/DropdownMenu already used — no new dependencies for CommandPalette or NotificationPanel
- `export:job:{uuid}` Redis TTL is 1 hour — goroutine cleanup of `/tmp` files happens in DownloadExport handler
