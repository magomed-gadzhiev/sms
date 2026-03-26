# Tasks: UX/UI and Accessibility Audit

**Input**: Design documents from `/specs/006-ux-a11y-audit/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Tests**: Not requested in spec — no test tasks generated. Manual validation via axe DevTools + screenreader per quickstart.md.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: User story label (US1–US4)
- All paths relative to repository root

## Path Conventions

- Frontend SPA: `portal-frontend/src/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create new shared utilities used across multiple user stories. No dependencies — start immediately.

- [X] T001 Create useFocusTrap hook in portal-frontend/src/hooks/useFocusTrap.ts (FR-005; captures previouslyFocusedElement, traps Tab/Shift+Tab, restores focus on deactivation)
- [X] T002 [P] Create SkipLink component in portal-frontend/src/components/SkipLink.tsx (FR-009; visually hidden by default, visible on :focus, links to `#main-content`)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Update App.tsx with navigation landmark and skip-link integration — underpins ALL user stories.

**⚠️ CRITICAL**: Depends on T002 (SkipLink must exist). No user story work can begin until T003 is complete.

- [X] T003 Update portal-frontend/src/App.tsx: render `<SkipLink targetId="main-content" />` as first child of root, add `aria-label="Main navigation"` to `<nav>`, add `id="main-content"` to `<main>`, add `const location = useLocation()` from react-router-dom, add `aria-current={location.pathname === item.path ? "page" : undefined}` to each nav `<Link>`, add `borderLeft: "3px solid #1976d2"` as active visual marker, add `marginTop: "16px"` to Logout button wrapper to visually separate it from nav items (FR-004, FR-007, FR-009, FR-010, FR-012, FR-013)

**Checkpoint**: Navigation accessible — skip-link functional, active page announced as "current page", all user stories can now proceed.

---

## Phase 3: User Story 1 — Вход и навигация (Priority: P1) 🎯 MVP

**Goal**: Форма входа и навигация полностью доступны с клавиатуры и скринридера.

**Independent Test**: Открыть `/login`, отправить форму с ошибкой — ошибка озвучена немедленно через `role="alert"`. После входа — Tab по навигации, активный пункт объявляется как "current page".

### Implementation

- [X] T004 [P] [US1] Update portal-frontend/src/pages/auth/LoginPage.tsx: assign `id="login-error"` to error `<p>`, set `role="alert"` on it, add `aria-describedby="login-error"` to email and password inputs, change loading `<div>` to `<div role="status">` (FR-001, FR-008; Contract 1)
- [X] T005 [P] [US1] Update portal-frontend/src/pages/auth/PasswordResetRequestPage.tsx: assign `id="reset-request-error"` + `role="alert"` to error element, add `aria-describedby="reset-request-error"` on email input, change loading `<div>` to `<div role="status">` (FR-001, FR-008)
- [X] T006 [P] [US1] Update portal-frontend/src/pages/auth/PasswordResetPage.tsx: assign `id="reset-error"` + `role="alert"` to error element, add `aria-describedby="reset-error"` on password and confirm inputs, change loading `<div>` to `<div role="status">` (FR-001, FR-008)
- [X] T007 [US1] Update portal-frontend/src/pages/dashboard/DashboardPage.tsx: change loading `<div>` to `<div role="status">`, convert metric stat `<div>` cards to `<a href="/messages">` / `<a href="/api-keys">` etc. links with appropriate labels (FR-008, FR-011)

**Checkpoint**: User Story 1 complete — login flow and navigation fully accessible with keyboard and screenreader.

---

## Phase 4: User Story 2 — Просмотр и фильтрация сообщений (Priority: P2)

**Goal**: Таблица Messages, пагинация и пустые состояния доступны для AT без нарушений WCAG.

**Independent Test**: Открыть `/messages` — скринридер объявляет "Messages list, table". Ячейки с обрезанным текстом имеют `title` с полным текстом. Кнопки пагинации несут `aria-label`.

### Implementation

- [X] T008 [US2] Update portal-frontend/src/pages/messages/MessagesPage.tsx: add `<caption>Messages list</caption>` as first child of `<table>`, add `title={message.body}` to truncated message `<td>` cells, add `aria-label="Previous page"` and `aria-label="Next page"` to pagination buttons, add `aria-disabled="true"` to disabled pagination buttons, wrap page-info text in `<span aria-live="polite">`, change loading `<div>` to `<div role="status">`, ensure empty-state element uses `color: #767676` not `#999` (FR-006, FR-008, FR-014, FR-016, FR-017; Contracts 4, 7)

**Checkpoint**: User Story 2 complete — Messages table fully navigable by AT.

---

## Phase 5: User Story 3 — Создание API Key (Priority: P2)

**Goal**: Inline-форма создания ключа реализует focus trap; банер нового ключа озвучивается немедленно; кнопки действий несут контекстные метки.

**Independent Test**: Открыть `/api-keys`, нажать "Create API Key" — фокус перемещается в форму, Tab замкнут, Escape возвращает фокус на кнопку. Созданный ключ озвучен скринридером как alert.

### Implementation

- [X] T009 [P] [US3] Update portal-frontend/src/pages/api-keys/APIKeysPage.tsx: add `role="alert"` to new-key banner div, attach `useFocusTrap(formRef, isFormOpen)` to inline create-form container, add `aria-label={\`Revoke \${key.name}\`}` and `aria-label={\`Delete \${key.name}\`}` to table action buttons, add `<caption>API Keys</caption>` to `<table>`, wrap status text in `<span aria-label={\`Status: \${key.active ? "Active" : "Revoked"}\`}>`, change loading `<div>` to `<div role="status">`, update action button padding to `"8px 12px"` (FR-002, FR-003, FR-005, FR-006, FR-016, FR-018; Contracts 2, 5)
- [X] T010 [P] [US3] Update portal-frontend/src/pages/webhooks/WebhooksPage.tsx: add `role="alert"` to webhook-secret banner div, attach `useFocusTrap(formRef, isFormOpen)` to inline create/edit-form container, add contextual `aria-label` to "Delete", "Test", and "Edit" buttons (e.g. `aria-label={\`Delete \${wh.url}\`}`), add `<caption>Webhook subscriptions</caption>`, wrap status text in `<span aria-label={...}>`, change loading `<div>` to `<div role="status">`, update action button padding to `"8px 12px"` (FR-002, FR-003, FR-005, FR-006, FR-016, FR-018)

**Checkpoint**: User Story 3 complete — API Key creation flow keyboard-accessible with screenreader announcements.

---

## Phase 6: User Story 4 — Настройка 2FA в профиле (Priority: P3)

**Goal**: 2FA setup flow в Profile доступен для AT: сообщения озвучиваются через `role="status"`, TOTP-секрет можно скопировать кнопкой.

**Independent Test**: Открыть `/profile`, включить 2FA — "2FA enabled successfully" озвучен без ручного перемещения фокуса. Рядом с `<code>` TOTP-секрета присутствует кнопка "Copy secret".

### Implementation

- [X] T011 [US4] Update portal-frontend/src/pages/profile/ProfilePage.tsx: assign `id="profile-error"` + `role="alert"` to error elements, add `aria-describedby="profile-error"` on edited form inputs, change success messages ("Profile updated", "2FA enabled successfully", "2FA disabled") to `<p role="status">`, add a "Copy secret" `<button>` adjacent to TOTP `<code>` block with `onClick` calling `navigator.clipboard.writeText(secret)`, change loading `<div>` to `<div role="status">` (FR-001, FR-002, FR-008; Contract 6)

**Checkpoint**: User Story 4 complete — Profile 2FA flow fully accessible.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Remaining pages not covered by individual user stories, and global contrast/touch-target fixes.

- [X] T012 [P] Update portal-frontend/src/pages/analytics/AnalyticsPage.tsx: wrap date/period filter controls in `<fieldset><legend>Filters</legend>…</fieldset>`, add `<caption>Message statistics by period</caption>` and `<caption>Message statistics by country</caption>` to respective tables, change loading `<div>` to `<div role="status">` (FR-006, FR-008, FR-015)
- [X] T013 [P] Update portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx: add `<caption>Sub-accounts</caption>`, add `aria-label={\`Status: \${sa.active ? "Active" : "Inactive"}\`}` to status cells, add contextual `aria-label` to action buttons, attach `useFocusTrap(formRef, isFormOpen)` to inline sub-account form, change loading `<div>` to `<div role="status">` (FR-003, FR-005, FR-006, FR-008, FR-016)
- [X] T014 [P] Update portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx: add any missing table `<caption>`, change loading `<div>` to `<div role="status">` (FR-006, FR-008)
- [X] T015 [P] Update portal-frontend/src/pages/audit/AuditLogPage.tsx: add `<caption>Audit log entries</caption>`, add `aria-label="Previous page"/"Next page"` to pagination buttons, add `aria-disabled="true"` to disabled buttons, wrap page-info text in `<span aria-live="polite">`, change loading `<div>` to `<div role="status">` (FR-006, FR-008, FR-017)
- [X] T016 [P] Fix colour contrast across portal-frontend/src/pages/: replace `#999` with `#767676` in empty-state text, replace `#f44336` with `#d32f2f` in error/revoked text, replace `color: '#4caf50'` (Active/green status) with `#388e3c` in status indicators (search all pages; research.md §8 — FR-016, SC-005, SC-006)
- [X] T017 [P] Increase action button padding to minimum `"8px 12px"` in portal-frontend/src/pages/messages/MessagesPage.tsx and portal-frontend/src/pages/audit/AuditLogPage.tsx (FR-018; pages not already updated in T008/T015)
- [ ] T018 Run axe DevTools validation per quickstart.md on all 10 pages — confirm 0 critical/serious violations, verify all status indicators pass SC-005 (non-color conveyance) and colour contrast SC-006 (SC-001, SC-005, SC-006)
- [ ] T019 Manual keyboard navigation walkthrough per quickstart.md checklist: Tab order, skip-link, focus trap in forms, screenreader announcements for errors/banners/status (SC-002, SC-003, SC-004, SC-007)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — T001 and T002 start immediately in parallel
- **Foundational (Phase 2)**: T003 depends on T002 (SkipLink) — **BLOCKS all user stories**
- **US1 (Phase 3)**: After T003 — T004, T005, T006 are parallel; T007 sequential (no deps on T004–T006)
- **US2 (Phase 4)**: After T003 — fully independent of US1, US3, US4
- **US3 (Phase 5)**: After T001 (useFocusTrap) and T003 — T009 and T010 are parallel
- **US4 (Phase 6)**: After T003 — fully independent of US1, US2, US3
- **Polish (Phase 7)**: After all story phases — T012–T017 are parallel; T018–T019 after T012–T017

### User Story Dependencies

- **US1 (P1)**: Phase 2 → no inter-story deps
- **US2 (P2)**: Phase 2 → no inter-story deps
- **US3 (P2)**: Phase 1 (T001) + Phase 2 → no inter-story deps
- **US4 (P3)**: Phase 2 → no inter-story deps

### Within Each User Story

- Auth pages (T004–T006) are independent of each other — run in parallel
- T007 (Dashboard) is independent of T004–T006
- T009 (API Keys) and T010 (Webhooks) are independent — run in parallel

---

## Parallel Example: User Story 3

```bash
# T009 and T010 can run concurrently (separate files):
Task A: "Update APIKeysPage.tsx — role=alert, useFocusTrap, aria-labels, caption, status badges"
Task B: "Update WebhooksPage.tsx — role=alert, useFocusTrap, aria-labels, caption, status badges"
```

## Parallel Example: Polish Phase

```bash
# T012–T017 can all run concurrently (separate files / global pass):
Task A: "Update AnalyticsPage.tsx — fieldset, captions, loading"
Task B: "Update SubAccountsListPage.tsx — caption, status aria-labels, focus trap"
Task C: "Update SubAccountDetailPage.tsx — caption, loading"
Task D: "Update AuditLogPage.tsx — caption, pagination, loading"
Task E: "Fix colour contrast across all pages (#999→#767676, #f44336→#d32f2f)"
Task F: "Increase button padding in MessagesPage + AuditLogPage"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: T001 + T002 (parallel)
2. Complete Phase 2: T003
3. Complete Phase 3: T004–T007
4. **STOP and VALIDATE**: Login + keyboard nav manual test per quickstart.md
5. Demo: accessible login flow and navigation

### Incremental Delivery

1. Phase 1 + 2 → Foundation ready (skip-link, nav landmark, aria-current)
2. US1 → Accessible login/nav → Validate → Demo
3. US2 + US3 in parallel → Messages + API Keys → Validate
4. US4 → Profile 2FA → Validate
5. Polish → Remaining pages + contrast + touch targets → axe audit (T018) + manual (T019)

### Parallel Team Strategy

After Phase 1 + 2:
- Developer A: US1 (LoginPage, PasswordReset×2, DashboardPage)
- Developer B: US2 (MessagesPage)
- Developer C: US3 (APIKeysPage + WebhooksPage)
- Developer D: US4 (ProfilePage)

---

## Notes

- [P] tasks = different files, safe to parallelise
- No automated test tasks — spec does not request them; use axe DevTools + manual screenreader testing per quickstart.md
- Each task cites the FR code(s) and/or Contract it satisfies for full traceability
- No new npm dependencies — all changes use native HTML attributes, ARIA, and React hooks
- Commit after each completed task or logical group
- Stop at every Checkpoint to validate the user story independently before proceeding
