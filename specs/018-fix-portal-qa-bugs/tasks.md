---
description: "Task list for 018-fix-portal-qa-bugs"
---

# Tasks: Исправление багов портала по результатам QA

**Input**: Design documents from `/specs/018-fix-portal-qa-bugs/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, quickstart.md ✅  
**Tests**: Not requested — frontend-only bugfixes validated by existing QA script on production.  
**Organization**: Tasks grouped by user story; all US2 fixes are independent of each other (different files).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 / US2 / US3

---

## Phase 1: Setup

**Purpose**: Confirm working state before making changes.

- [X] T001 Verify portal-frontend builds without errors: `cd portal-frontend && npm run build`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: N/A — all fixes are independent edits to existing page components; no shared infrastructure changes required.

**⚠️ CRITICAL**: No foundational work needed. All user story phases can begin immediately after T001.

---

## Phase 3: User Story 1 — Дашборд загружается после входа (Priority: P1) 🎯 MVP

**Goal**: Eliminate infinite "Загрузка дашборда…" spinner by removing the redundant `profileApi.get()` call and ensuring the loading state always resolves.

**Independent Test**: Login → navigate to `/dashboard` → within 5 seconds see either stats or onboarding empty screen; no "Загрузка дашборда…" stuck state.

### Implementation for User Story 1

- [X] T002 [US1] Fix `portal-frontend/src/pages/dashboard/DashboardPage.tsx`: import `useAuth` and replace `const { user: profile } = useAuth()`; remove `profileApi.get()` from `Promise.all`; replace `Promise.all([dashboardApi.get(), profileApi.get()])` with a single `dashboardApi.get()` call; confirm `.finally(() => setLoading(false))` is present; on catch, call `setLoading(false)` (already handled by `.finally`) and render existing onboarding/empty-state view instead of error-only text

**Checkpoint**: Dashboard always exits loading state — on success, on error, and for new accounts with no data. No duplicate 401 errors for `/portal/v1/profile` in browser console.

---

## Phase 4: User Story 2 — Страницы с пустыми данными отображаются корректно (Priority: P1)

**Goal**: All five previously-failing pages render an informative empty-state component (not a generic DataTable cell or broken layout) when the user has no records.

**Independent Test**: Login with a fresh account → open `/messages`, `/campaigns`, `/templates`, `/contact-lists`, `/billing` in sequence → each page renders without errors and shows a human-readable empty-state message.

### Implementation for User Story 2

All T003–T007 tasks touch different files and can run in parallel.

- [X] T003 [P] [US2] Fix `portal-frontend/src/pages/messages/MessagesPage.tsx`: add empty-state block `{!loading && !error && (data?.messages?.length ?? 0) === 0 && (<div className="text-center py-12 text-gray-500">…Сообщений пока нет…<Button onClick={() => setShowSendModal(true)}>Отправить SMS</Button></div>)}`; render `<DataTable>` only when `loading || (data?.messages?.length ?? 0) > 0`

- [X] T004 [P] [US2] Fix `portal-frontend/src/pages/campaigns/CampaignsPage.tsx`: wrap `load` function in `useCallback([page])` and update `useEffect` dependency to `[load]` so loading state always resolves and React tracks the dependency correctly

- [X] T005 [P] [US2] Fix `portal-frontend/src/pages/templates/TemplatesPage.tsx`: add empty-state block `{!loading && !error && templates.length === 0 && (<div className="text-center py-12 text-gray-500">…Шаблоны не созданы…<Button onClick={openCreateForm}>Создать шаблон</Button></div>)}`; render `<DataTable>` only when `loading || templates.length > 0`

- [X] T006 [P] [US2] Fix `portal-frontend/src/pages/contacts/ContactListsPage.tsx`: same `useCallback` fix as T004 — wrap `load` in `useCallback([page])` and update `useEffect([load])`

- [X] T007 [US2] Fix `portal-frontend/src/pages/billing/BillingPage.tsx`: in the `getBalance()` catch block, check `err instanceof ApiError && err.status === 404` → set `setBalance({ client_id: '', balance: '0.00', currency: 'RUB' })` instead of `setBalanceError`; add transactions empty-state `{!loading && !error && (data?.transactions?.length ?? 0) === 0 && (<div className="text-center py-8 text-gray-500 text-sm">История транзакций пуста</div>)}`; render `<DataTable>` for transactions only when `loading || (data?.transactions?.length ?? 0) > 0`

**Checkpoint**: All 5 pages render cleanly for a fresh account. No unhandled errors in browser console. QA stress tests for empty-state scenarios all pass.

---

## Phase 5: User Story 3 — Валидация имени API-ключа с понятной ошибкой (Priority: P2)

**Goal**: User creating an API key with a too-long name receives an immediate, field-level error message rather than a silent failure.

**Independent Test**: Open API Keys page → attempt to create a key with a name longer than 100 characters → see "Имя не должно превышать 100 символов" displayed next to the name field, without a network request being sent.

### Implementation for User Story 3

- [X] T008 [US3] Fix `portal-frontend/src/pages/api-keys/APIKeysPage.tsx`: add `const [nameError, setNameError] = useState('')`; at start of `handleCreate` reset `setNameError('')`; add client-side guard `if (name.length > 100) { setNameError('Имя не должно превышать 100 символов'); setCreating(false); return; }`; in `.catch` block, if `err instanceof ApiError && err.status === 400` call `setNameError(err.message)` (instead of or in addition to generic error); add `maxLength={100}` to the "Название" `<Input>`; pass `error={nameError || undefined}` prop to the Input so the inline error renders below the field

**Checkpoint**: Client-side validation fires instantly for long names. Server-side 400 response surfaces as inline field error. Successful creation still works. QA test "Long API key name" → `ok: true`.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final verification and deployment.

- [X] T009 [P] Verify TypeScript compiles without errors across all 7 changed files: `cd portal-frontend && npx tsc --noEmit`
- [ ] T010 Deploy to production and run automated QA script — confirm all 6 previously-failing tests now pass: empty-state for /messages, /campaigns, /templates, /contact-lists, /billing, and "Long API key name" → `ok: true`; dashboard screenshot shows no "Загрузка дашборда…" after 5 s; no repeated 401 errors in console

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Start immediately
- **Foundational (Phase 2)**: N/A — no blocking prerequisites
- **User Stories (Phase 3, 4, 5)**: Can start after T001 passes
  - US1 (Phase 3) and US2 (Phase 4) and US3 (Phase 5) are independent of each other — can proceed in parallel
- **Polish (Phase 6)**: After all desired stories are complete

### User Story Dependencies

- **US1 (P1)**: Independent — single file edit
- **US2 (P1)**: Independent of US1 — five independent file edits (T003–T007 are parallelizable)
- **US3 (P2)**: Independent of US1 and US2 — single file edit

### Within Each User Story

- US2: T003, T004, T005, T006 can all run in parallel (different files); T007 is also independent but touches two concerns in one file
- US1 and US3: single-task stories, no internal ordering needed

### Parallel Opportunities

- After T001: US1 (T002), all US2 tasks (T003–T007), and US3 (T008) can all run in parallel
- T009 and T010 must run after all implementation tasks complete

---

## Parallel Example: User Story 2

```bash
# Launch all US2 fixes simultaneously (different files, no conflicts):
Task: T003 — MessagesPage.tsx empty state
Task: T004 — CampaignsPage.tsx useCallback
Task: T005 — TemplatesPage.tsx empty state
Task: T006 — ContactListsPage.tsx useCallback
Task: T007 — BillingPage.tsx 404→0-balance + empty state
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete T001 (setup)
2. Complete T002 (US1 — dashboard fix)
3. **STOP and VALIDATE**: Open `/dashboard` after login — confirm no infinite spinner
4. Deploy if dashboard fix alone is urgent

### Incremental Delivery

1. T001 → T002 (dashboard fix) → Deploy → Validate dashboard
2. T003–T007 in parallel (empty-state fixes) → Deploy → Validate 5 pages
3. T008 (API key validation) → Deploy → Validate QA test

### Parallel Team Strategy

With multiple developers:

- Developer A: T002 (US1 — DashboardPage)
- Developer B: T003 + T004 + T005 (US2 — Messages, Campaigns, Templates)
- Developer C: T006 + T007 + T008 (US2 — Contacts, Billing + US3 — APIKeys)
- All merge → T009 → T010

---

## Notes

- No new files — all tasks are edits to existing page components
- No new dependencies — Tailwind, Radix UI, useAuth already available
- Each fix is surgical: change only what's listed in plan.md Fix Specifications
- Commit after each logical group (e.g., after US1, after all US2, after US3)
- Validate with `npx tsc --noEmit` before deploying
