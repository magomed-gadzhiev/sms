# Admin Panel: Figma Design Alignment & Enhancement Spec

> **Date:** 2026-04-11
> **Figma:** https://www.figma.com/design/FeU5OG5accz9vnsNZRaYpW/ (page "MVP", root node `4555:19912`)
> **Purpose:** Self-contained reference for AI agents implementing Figma alignment, UX improvements, and missing features across admin panel pages.

---

## Table of Contents

1. [Context & Current State](#1-context--current-state)
2. [Tech Stack](#2-tech-stack)
3. [Architecture Overview](#3-architecture-overview)
4. [Figma Section Map](#4-figma-section-map)
5. [Phase 1: Detalization UX Enhancement](#phase-1-detalization-ux-enhancement)
6. [Phase 2: Connections Page Enhancement](#phase-2-connections-page-enhancement)
7. [Phase 3: Routing Visual Alignment](#phase-3-routing-visual-alignment)
8. [Phase 4: Sender Names — Figma Alignment](#phase-4-sender-names--figma-alignment)
9. [Phase 5: Statistics — Figma Alignment](#phase-5-statistics--figma-alignment)
10. [Phase 6: MCC/MNC — Figma Alignment](#phase-6-mccmnc--figma-alignment)
11. [Phase 7: Operator Templates — Variable Editor](#phase-7-operator-templates--variable-editor)
12. [Phase 8: Settings Page — Figma Alignment](#phase-8-settings-page--figma-alignment)
13. [Phase 9: Legal Entities — Operator Association](#phase-9-legal-entities--operator-association)
14. [Phase 10: Contracts — Enhanced UX](#phase-10-contracts--enhanced-ux)
15. [Phase 11: Cross-Cutting Improvements](#phase-11-cross-cutting-improvements)
16. [Implementation Order](#implementation-order)

---

## 1. Context & Current State

The admin panel is a **production-grade React SPA** with 24+ pages, 38+ page files, and full CRUD coverage for all entities. The codebase is mature — no TODOs, no stubs, all API integrations are wired.

**What this document covers:** Visual/UX alignment with Figma design, enhancement of existing pages, and cross-cutting improvements. This is NOT a greenfield build — all pages and backend APIs already exist.

### Key Principle for Agents

> **Do not rewrite pages from scratch.** All pages are functional. Modify existing files to align with Figma. Preserve existing functionality while enhancing UX.

---

## 2. Tech Stack

- **Frontend:** TypeScript 5.7, React 19, Vite 6.0, React Router 7.1, Tailwind CSS 4.2, Radix UI (Dialog, Tabs, DropdownMenu, Toast), Recharts 3.8.1
- **Backend:** Go 1.24.0, gorilla/mux (REST), gRPC (inter-service), pgx/v5, go-redis/v9
- **Database:** PostgreSQL 15+ (monthly-partitioned `messages`, `audit_log`, `tarification_log`), Redis 7+
- **Dev server:** Vite port 3001, admin API proxy → `localhost:8081`

### Component Library (reuse these, do NOT create new primitives)

| Component | Path | Purpose |
|-----------|------|---------|
| `Button` | `components/ui/Button.tsx` | Variants: primary, secondary, danger, ghost; Sizes: sm, md, lg |
| `Input` | `components/ui/Input.tsx` | Text inputs with labels, validation states |
| `Select` | `components/ui/Select.tsx` | Dropdown select |
| `Modal` | `components/ui/Modal.tsx` | Dialog modals with title/footer |
| `Badge` | `components/ui/Badge.tsx` | Status badges: success, danger, warning, info |
| `Toast` | `components/ui/Toast.tsx` | Notifications via `useToast()` |
| `ConfirmDialog` | `components/ui/ConfirmDialog.tsx` | Confirmation dialogs with danger variant |
| `MultiSelect` | `components/ui/MultiSelect.tsx` | Multi-select dropdown |
| `SearchableSelect` | `components/ui/SearchableSelect.tsx` | Searchable dropdown |
| `DataTable` | `components/data/DataTable.tsx` | Reusable table with pagination, sorting, bulk actions |
| `FilterBar` | `components/data/FilterBar.tsx` | Searchable filter controls |
| `StatCard` | `components/data/StatCard.tsx` | KPI stat card |
| `PageHeader` | `components/layout/PageHeader.tsx` | Page title, breadcrumbs, action buttons |
| `TimelineEvent` | `components/data/TimelineEvent.tsx` | Timeline visualization element |

### API Layer

All admin APIs are in `portal-frontend/src/api/admin.ts` (~950 lines). Key API objects:

| API Object | Methods |
|------------|---------|
| `clientsApi` | list, get, create, update, delete, getConfig, updateConfig, updateRateLimits |
| `providersApi` | list, get, create, update, delete, listHealth |
| `platformRoutesApi` | list, create, update, delete, reorder |
| `connectionsApi` | list, get, reconnect, stop |
| `billingApi` | getBalance, addCredits, getTransactions, listPricingRules, createPricingRule, freeze, unfreeze, setCreditLimit, getBalances |
| `analyticsAdminApi` | getStats, generateReport, getRealTimeMetrics, getProviderPerformance, getGroupedStats |
| `tarificationApi` | Full CRUD for plans, periods, tiers, sender registrations, hierarchical periods |
| `messagesApi` | list, get |
| `legalEntitiesApi` | list, get, create, update, delete |
| `contractsApi` | list, get, create, update, delete |
| `operatorTemplatesApi` | list, get, create, update, delete |
| `systemDefaultsApi` | getAll, set |
| `clientRoutesApi` | list, create, update, delete (scoped by clientId) |
| `adminSenderNamesApi` | list, approve, reject, deactivate, get, operatorRegistrations |
| `countriesApi` | list, create |
| `operatorsApi` | list, create, update, listPrefixes, addPrefix, deletePrefix |

---

## 3. Architecture Overview

```
portal-frontend/src/
  App.tsx                              — Router config, all admin routes registered
  api/admin.ts                         — All admin API functions (~950 lines)
  contexts/AuthContext.tsx              — Global auth state, hasPermission()
  components/
    layout/AdminSidebar.tsx            — Sidebar nav (already matches Figma grouping)
    layout/AdminLayout.tsx             — Admin wrapper
    layout/PageHeader.tsx              — Page header + breadcrumbs
    ui/                                — 13 reusable UI primitives
    data/                              — 6 data display components
  pages/admin/
    dashboard/DashboardPage.tsx        — Live dashboard with metrics/charts/alerts
    ClientsPage.tsx                    — Client management
    SenderNamesAdminPage.tsx           — Sender names list
    sender-names/SenderNameDetailPage.tsx — Sender name detail + operator registrations
    operator-templates/OperatorTemplatesPage.tsx — Operator template CRUD
    templates/TemplatesPage.tsx         — Client template moderation workflow
    billing/BillingPage.tsx            — 4-tab billing (Balances, Transactions, Credits, Pricing)
    tarification/TarificationPage.tsx  — General tariff management (Plans, Periods, Tiers, SenderRegs)
    tarification/IndividualTariffsPage.tsx — Client-specific tariffs
    ProvidersPage.tsx                  — Provider CRUD
    RoutesPage.tsx                     — Platform routes with drag-drop reorder
    connections/ConnectionsPage.tsx    — SMPP connection monitoring
    client-routes/ClientRoutesPage.tsx — Per-client routing rules
    channels/ChannelsPage.tsx          — Channel types
    delivery-strategies/DeliveryStrategiesPage.tsx — Delivery strategies
    HLRPage.tsx                        — HLR providers
    CountriesPage.tsx                  — MCC/MNC reference (dual view + wizard)
    legal-entities/LegalEntitiesPage.tsx — Legal entity CRUD
    contracts/ContractsPage.tsx        — Contract management
    settings/SettingsPage.tsx          — System defaults
    AnalyticsPage.tsx                  — Statistics with grouped tabs
    detalization/DetalizationPage.tsx  — Message log search + detail
    MonitoringPage.tsx                 — System health monitoring
    AuditLogPage.tsx                   — Audit trail
    users/UsersPage.tsx                — User management
    users/RolesPage.tsx                — Role/permission management
    WebhooksPage.tsx                   — Webhook configuration
```

---

## 4. Figma Section Map

Reference for agents when they need to check the original Figma design.

| # | Section | Figma Node | Status | Current Page | Phase |
|---|---------|------------|--------|--------------|-------|
| 1 | Клиенты | `9731:31575` | Done | ClientsPage.tsx | — (minimal gap) |
| 2 | Имена отправителей | `9808:29236` | Done | SenderNamesAdminPage.tsx + SenderNameDetailPage.tsx | Phase 4 |
| 3 | Шаблоны операторов | `12077:58755` | Done | OperatorTemplatesPage.tsx | Phase 7 |
| 4 | Справочник MCC/MNC | `10226:33465` | WIP | CountriesPage.tsx | Phase 6 |
| 5 | Маршрутизация | `10226:34013` | Done | RoutesPage.tsx | Phase 3 |
| 6 | Авторизация | `10026:8997` | Done | Login page | — (minimal gap) |
| 7 | Детализация | `11108:24254` | Done | DetalizationPage.tsx | Phase 1 |
| 8 | Статистика | `11237:40043` | Done | AnalyticsPage.tsx | Phase 5 |
| 9 | Поставщики | `11839:68503` | Done | ProvidersPage.tsx | — (minimal gap) |
| 10 | Каналы | `12146:83183` | Done | ChannelsPage.tsx | — (minimal gap) |
| 11 | Список подключений | `12149:95586` | Done | ConnectionsPage.tsx | Phase 2 |
| 12 | Инд. маршруты | `13199:93902` | Done | ClientRoutesPage.tsx | — (minimal gap) |
| 13 | Настройки | `13199:93903` | WIP | SettingsPage.tsx | Phase 8 |
| 14 | Тарифы общие | `13533:136681` | Done | TarificationPage.tsx | — (minimal gap) |
| 15 | Тарифы индивидуальные | `13607:102013` | Done | IndividualTariffsPage.tsx | — (minimal gap) |
| 16 | Юр. лица | `15915:123972` | Done | LegalEntitiesPage.tsx | Phase 9 |
| 17 | Договоры | `17427:155489` | Done | ContractsPage.tsx | Phase 10 |
| 18 | Статусы сообщений | `11033:35066` | Done | Part of DetalizationPage.tsx | Phase 1 |

**How to use Figma nodes:** Open `https://www.figma.com/design/FeU5OG5accz9vnsNZRaYpW/?node-id=<NODE_ID>` replacing `:` with `-` in node ID. Example: node `9731:31575` → URL param `node-id=9731-31575`.

---

## Phase 1: Detalization UX Enhancement

**File:** `portal-frontend/src/pages/admin/detalization/DetalizationPage.tsx` (427 lines)
**Figma:** Node `11108:24254` (Детализация) + `11033:35066` (Статусы сообщений)
**Backend API:** `messagesApi.list()`, `messagesApi.get(id)` — both exist

### Current State
- FilterBar with 7 filters (client, status, provider, source, destination, date range)
- DataTable with pagination (50 per page)
- Modal detail view with basic info, DLR data, billing breakdown
- Read-only — appropriate for this page

### What Figma Shows (from screenshot analysis)
1. **Message status timeline** — visual pipeline: pending → queued → sent → delivered/failed with timestamps at each step
2. **Mobile phone preview** — mockup of SMS on a phone screen showing how the message looks
3. **Advanced filter panel** — expandable "Все фильтры" panel separate from basic filters
4. **Color-coded status legend** — "Статусы сообщений" reference showing all possible statuses with colors
5. **Export button** — export filtered results to CSV

### Changes Required

#### 1.1 Add StatusTimeline component
Create a visual timeline showing message lifecycle. Reuse `TimelineEvent` component.

**New file:** `portal-frontend/src/pages/admin/detalization/StatusTimeline.tsx`

```
Props: { statuses: Array<{ status: string; timestamp: string; details?: string }> }
```

Visual: vertical timeline with colored dots (grey=pending, blue=sent, green=delivered, red=failed, yellow=queued) connected by lines. Each node shows status label + relative time.

Place this inside the existing detail modal, replacing or supplementing the current flat status display.

#### 1.2 Add SMS preview component
Small inline "phone frame" showing message text as it would appear on a mobile device.

**New file:** `portal-frontend/src/pages/admin/detalization/SmsPreview.tsx`

```
Props: { sender: string; body: string; timestamp: string }
```

Visual: rounded rectangle mimicking a phone screen with chat bubble styling. Sender name at top, message body in bubble, timestamp below.

Place in the detail modal alongside StatusTimeline.

#### 1.3 Add CSV export
Add "Экспорт" button next to PageHeader actions. On click: fetch current filtered data (up to 10,000 rows) and trigger CSV download client-side.

Modify `DetalizationPage.tsx`:
- Add export button in PageHeader actions
- Implement `exportToCsv()` function that converts current data to CSV blob
- Use `URL.createObjectURL()` + `<a download>` pattern

#### 1.4 Enhance filter UX
Current FilterBar has 7 filters shown inline. Figma shows a collapsible "Все фильтры" pattern.

Modify `DetalizationPage.tsx`:
- Show 3 primary filters inline (date_from, date_to, status)
- Add "Все фильтры" toggle button that expands remaining 4 filters
- Use simple `useState<boolean>` for expanded state with `Transition` for animation

### Acceptance Criteria
- [ ] StatusTimeline renders chronological status history with colored dots
- [ ] SmsPreview shows phone mockup with message text
- [ ] CSV export works with current filters applied
- [ ] "Все фильтры" toggles additional filters visibility
- [ ] No existing functionality broken

---

## Phase 2: Connections Page Enhancement

**File:** `portal-frontend/src/pages/admin/connections/ConnectionsPage.tsx` (174 lines)
**Figma:** Node `12149:95586` (Список подключений)
**Backend API:** `connectionsApi.list()`, `connectionsApi.get(id)`, `connectionsApi.reconnect(id)`, `connectionsApi.stop(id)`

### Current State
- Read-only table with 8 columns (provider, host:port, system_id, bind, status, sessions, success%, messages_24h)
- Auto-refresh every 10 seconds
- Action buttons: Reconnect, Stop
- No detail view, no create/edit

### What Figma Shows (from screenshot)
1. **4 sections:** Общий список, Добавление нового подключения, Просмотр подключения, Редактирование подключения
2. **Connection detail panel** — expanded view with full metrics (TPS, uptime, error log, throughput stats)
3. **Terminal/log viewer** — dark-themed console showing connection logs
4. **Edit connection form** — modify runtime params

### Changes Required

#### 2.1 Add connection detail drawer
When user clicks a table row, show a slide-in drawer (or modal) with connection details from `connectionsApi.get(id)`.

Modify `ConnectionsPage.tsx`:
- Add `selectedConnection` state
- On row click: fetch `connectionsApi.get(id)` and show detail
- Detail sections: General info, Performance metrics, Recent errors

#### 2.2 Add refresh control
Replace hard-coded 10s interval with user-controllable refresh.

Modify `ConnectionsPage.tsx`:
- Add refresh toggle button: "Автообновление: 10с" (click to pause/resume)
- Add manual "Обновить" button
- Use `usePolling` hook (already exists in `hooks/usePolling.ts`)

#### 2.3 Add connection log viewer
Figma shows a terminal-style dark panel with connection logs.

**New file:** `portal-frontend/src/pages/admin/connections/ConnectionLog.tsx`

```
Props: { connectionId: string }
```

Visual: dark background (`bg-gray-900`), monospace font, scrollable div with log lines. Each line: `[timestamp] level: message`. Auto-scroll to bottom. Max 100 lines retained.

**Backend requirement:** If `GET /admin/v1/connections/{id}/logs` does not exist, add it. Check `internal/gateway/admin/handlers/connections.go` first. If the endpoint exists, wire it up. If not, this component should display a "Логи недоступны" placeholder until the endpoint is created.

### Acceptance Criteria
- [ ] Row click opens connection detail view
- [ ] Refresh is controllable (pause/resume/manual)
- [ ] Log viewer displays connection events (or placeholder if API missing)
- [ ] Existing reconnect/stop functionality preserved

---

## Phase 3: Routing Visual Alignment

**File:** `portal-frontend/src/pages/admin/RoutesPage.tsx` (342 lines)
**Figma:** Node `10226:34013` (Маршрутизация)
**Backend API:** `platformRoutesApi.list()`, `.create()`, `.update()`, `.delete()`, `.reorder()`

### Current State
- Custom HTML table with drag-and-drop reordering
- Columns: Operator, Channel, Provider, Legal Entity, Status, Actions
- Create/edit modal with conditional Legal Entity field (shown when "All Networks")
- Uses `platformRoutesApi` — matches Figma model

### What Figma Shows (from screenshot)
1. **Drag handle** explicitly visible (⠿ icon) — current implementation has this
2. **"Маршрут перемещён"** toast after reorder — confirm this exists
3. **Route creation form** — with conditional юр.лицо field
4. **Designer note:** "Если выбрано All Networks, то появляется доп.селект с выбором юр.лица. Обязателен для заполнения."

### Changes Required

#### 3.1 Visual polish
Mostly aligned. Verify and fix:

- Drag handle cursor: ensure `cursor: grab` on hover, `cursor: grabbing` while dragging
- Row highlight during drag: add `bg-indigo-50` to the row being dragged
- Toast after reorder: verify `toast.success('Маршрут перемещён')` fires after successful reorder API call
- Empty state: show "Нет маршрутов" message when list is empty

#### 3.2 Legal entity validation
Figma specifies legal entity is **required** when operator is "All Networks".

Modify `RoutesPage.tsx` create/edit modal:
- If `operator_id` is null/empty (All Networks): mark legal_entity_id as required, show red border if empty on submit
- If specific operator selected: hide legal_entity_id field entirely (not just optional)

### Acceptance Criteria
- [ ] Drag UX has proper cursors and highlight
- [ ] Toast confirms reorder
- [ ] Legal entity is strictly required for "All Networks" and hidden otherwise
- [ ] Empty state is user-friendly

---

## Phase 4: Sender Names — Figma Alignment

**Files:**
- `portal-frontend/src/pages/admin/SenderNamesAdminPage.tsx`
- `portal-frontend/src/pages/admin/sender-names/SenderNameDetailPage.tsx` (274 lines)

**Figma:** Node `9808:29236` (Имена отправителей)

### Current State
- List page with approve/reject/deactivate actions
- Detail page with: info card, operator registration grid (4-col), action modals
- 274 lines, production-grade

### What Figma Shows (from screenshot)
1. **5 sub-flows:** Список, Добавить имя, Просмотр имени, Удаление, Добавление шаблона оператора со страницы имени
2. **Add name modal** with inline validation rules:
   - "Имя должно совпадать с названием организации, ИП, товарным знаком или доменом"
   - "Латиница, не более 11 символов. Можно использовать цифры и знаки . _ —"
   - "Пробелы запрещены"
3. **Detail page** — operator registration grid matches current implementation
4. **"Добавить шаблон оператора"** button on detail page — navigates to operator template creation with sender name pre-filled

### Changes Required

#### 4.1 Add validation hints to create form
Modify `SenderNamesAdminPage.tsx` — in the create/add modal, add hint text below the name input:

```
<p className="text-xs text-gray-500 mt-1">
  Латиница, не более 11 символов. Можно использовать цифры и знаки . _ —
</p>
<p className="text-xs text-gray-500">Пробелы запрещены</p>
```

Add client-side validation: regex `/^[A-Za-z0-9._-]{1,11}$/`, show error if invalid.

#### 4.2 Add "Create operator template" shortcut
Modify `SenderNameDetailPage.tsx`:
- Add "Добавить шаблон оператора" button in PageHeader actions
- On click: navigate to `/admin/operator-templates?sender_name_id={id}&sender_name={name}` (pass as query params)

Modify `OperatorTemplatesPage.tsx`:
- Read `sender_name_id` from URL query params
- If present: auto-open create modal with sender_name pre-selected

#### 4.3 Enhanced delete flow
Figma shows delete from within detail page with redirect to list.

Verify in `SenderNameDetailPage.tsx`:
- Delete button exists in PageHeader → on confirm → call API → `navigate('/admin/sender-names')` → toast success
- If not: add this flow

### Acceptance Criteria
- [ ] Name validation shows inline hints and rejects invalid input
- [ ] "Добавить шаблон оператора" button works on detail page
- [ ] Delete redirects to list with success toast

---

## Phase 5: Statistics — Figma Alignment

**File:** `portal-frontend/src/pages/admin/AnalyticsPage.tsx` (224 lines)
**Figma:** Node `11237:40043` (Статистика)

### Current State
- FilterBar (period: 7d/30d/90d, client_id)
- 5 StatCards (sent, delivered, failed, delivery_rate, cost)
- 3 tabs: "По дням", "По операторам", "По странам"
- Scrollable table with sticky header + totals row
- LineChart + BarChart below table

### What Figma Shows
1. **Designer note:** "Фиксированная высота контейнера с таблицей. скролл внутри контейнера. Прилипающая шапка и саммари снизу"
2. **Fixed-height table container** — `h-[520px]` or similar
3. **Sticky header + sticky summary row** — totals pinned at bottom
4. **Tab group** — same as current

### Changes Required

#### 5.1 Verify fixed-height table
Check if current implementation has:
- Container with fixed height and `overflow-y: auto`
- `position: sticky; top: 0` on `<thead>`
- Summary row with `position: sticky; bottom: 0`

If not, modify `AnalyticsPage.tsx`:
```tsx
<div className="h-[520px] overflow-y-auto relative border rounded-lg">
  <table className="w-full">
    <thead className="sticky top-0 bg-white z-10 shadow-sm">...</thead>
    <tbody>...</tbody>
    <tfoot className="sticky bottom-0 bg-gray-50 font-semibold border-t-2">...</tfoot>
  </table>
</div>
```

#### 5.2 Add date range filter
Current: only period dropdown (7d/30d/90d).
Figma implies custom date range selection.

Add to FilterBar:
- `date_from` (type: date)
- `date_to` (type: date)
- When custom dates are set, period dropdown is ignored

### Acceptance Criteria
- [ ] Table has fixed height container with internal scroll
- [ ] Header sticks to top on scroll
- [ ] Summary row sticks to bottom
- [ ] Custom date range filter works alongside period presets

---

## Phase 6: MCC/MNC — Figma Alignment

**File:** `portal-frontend/src/pages/admin/CountriesPage.tsx` (545 lines)
**Figma:** Node `10226:33465` (Справочник MCC/MNC)

### Current State
- Dual view: "По странам" (hierarchical 3-column) + "MCC/MNC таблица" (flat)
- 2-step wizard for operator creation (Step 1: name/MCC/MNC, Step 2: settings)
- Operator detail panel with settings and prefixes
- 545 lines — the largest admin page

### What Figma Shows
1. **Operator detail** — different for Russia vs other countries (Russia shows юр.лица)
2. **Legal entity management** per operator — add/remove from operator detail
3. **Interdependent filters** — "Фильтр страны и оператора взаимозависимые"
4. **Settings pre-filled from backend** — "Настройки уже будут предзаполнены, с бэка"

### Changes Required

#### 6.1 Add legal entity section to operator detail
In the operator detail panel (right column of hierarchical view), add a "Юр. лица" section below existing settings:

```
Юр. лица
┌──────────────────────────────────────┐
│ ООО "Мегафон" (ИНН: 7812345678)  ✕  │
│ ООО "Билайн"  (ИНН: 9876543210)  ✕  │
│                                      │
│ [+ Добавить юр. лицо]               │
└──────────────────────────────────────┘
```

**Backend requirement:** Need endpoints:
- `GET /admin/v1/operators/{id}/legal-entities` — list associated legal entities
- `POST /admin/v1/operators/{id}/legal-entities` — associate (body: `{ legal_entity_id }`)
- `DELETE /admin/v1/operators/{id}/legal-entities/{le_id}` — disassociate

Check if these endpoints exist in `internal/gateway/admin/router/router.go`. If not:
1. Add junction table `operator_legal_entities` (migration)
2. Add handler methods in `internal/gateway/admin/handlers/operators.go` or new file
3. Add gRPC methods to routing-service

If backend endpoints do NOT exist, create the frontend UI with a placeholder message: "Управление юр. лицами оператора — ожидает backend API". The frontend component should be ready to wire up.

#### 6.2 Interdependent filters in flat view
Modify flat view filters:
- When country is selected → operator dropdown shows only operators from that country
- When operator is selected → country auto-selects to matching country
- Implement via `useEffect` watching filter changes

### Acceptance Criteria
- [ ] Operator detail shows legal entities section (or placeholder)
- [ ] Legal entities can be added/removed from operator (if backend exists)
- [ ] Flat view filters are interdependent
- [ ] No regression in hierarchical view or wizard

---

## Phase 7: Operator Templates — Variable Editor

**File:** `portal-frontend/src/pages/admin/operator-templates/OperatorTemplatesPage.tsx` (363 lines)
**Figma:** Node `12077:58755` (Шаблоны операторов)

### Current State
- DataTable with filters (operator, status)
- Create/edit modal with textarea + variable helper buttons
- Variables: `{code}`, `{name}`, `{date}`, `{sum}`, `{link}`
- CRUD fully wired

### What Figma Shows
1. **Inline variable insertion** — "Вставить переменную:" dropdown/buttons
2. **Variable highlighting** in textarea — variables shown in different color
3. **Template preview** — shows how template looks with sample values
4. **Advanced filter panel** — expandable filter section

### Changes Required

#### 7.1 Variable highlighting in textarea
Replace plain `<textarea>` with a `contentEditable` div or overlay approach:
- Display variable placeholders `{variable}` in blue/purple color
- Use a transparent textarea overlaid on a colored div (CSS overlay pattern)
- This avoids contentEditable complexity

Simple approach: show a "Preview" section below the textarea that renders the body with variables highlighted using `<span className="text-indigo-600 bg-indigo-50 px-1 rounded">`.

#### 7.2 Template preview with sample values
Add "Предпросмотр" toggle below the template body editor:
- Replace `{code}` → `1234`, `{name}` → `Иван`, `{date}` → `11.04.2026`, `{sum}` → `1 500 ₽`, `{link}` → `https://example.com/abc`
- Show in a styled card mimicking SMS appearance

**Dependency:** If Phase 1 (Detalization) is completed first, reuse `SmsPreview` component from `pages/admin/detalization/SmsPreview.tsx`. If not, create a standalone preview card inline (simple rounded div with chat-bubble styling). Later consolidate into shared component if both exist.

### Acceptance Criteria
- [ ] Variables are visually highlighted in preview
- [ ] Sample value preview renders realistic SMS content
- [ ] Existing CRUD functionality preserved

---

## Phase 8: Settings Page — Figma Alignment

**File:** `portal-frontend/src/pages/admin/settings/SettingsPage.tsx` (175 lines)
**Figma:** Node `13199:93903` (Настройки) — status: WIP in Figma

### Current State
- 2 sections with form fields (rate limits, provider/client settings)
- Per-section save/reset buttons
- Unsaved changes indicator (yellow border)
- Uses `systemDefaultsApi.getAll()` and `systemDefaultsApi.set(key, value)`

### What Figma Shows
- "Настройки--default" and "Настройки-- внесены изменения" — two states
- Extensive settings (26069x10617 px section in Figma — very large)
- Section-based layout with change indicators

### Changes Required

#### 8.1 Expand settings sections
Current implementation has only 2 sections. Add more based on `system_defaults` keys available from backend:

Suggested sections (verify which keys exist in DB):
1. **Общие** — platform name, timezone, default language
2. **SMS по умолчанию** — default encoding, max retries, retry interval, default TTL
3. **Rate Limits** — global default TPS, messages per minute/hour
4. **Провайдеры** — default connection timeout, max connections per provider
5. **Уведомления** — low balance threshold, queue depth alert threshold, provider degraded threshold
6. **Безопасность** — session timeout, max login attempts, password min length

**Implementation:** Read all keys from `systemDefaultsApi.getAll()`, group them by prefix (e.g., `sms.`, `rate.`, `security.`), render dynamically.

#### 8.2 Global "unsaved changes" warning
Add `beforeunload` event listener when any section has unsaved changes:
```ts
useEffect(() => {
  const handler = (e: BeforeUnloadEvent) => { e.preventDefault(); };
  if (hasChanges) window.addEventListener('beforeunload', handler);
  return () => window.removeEventListener('beforeunload', handler);
}, [hasChanges]);
```

### Acceptance Criteria
- [ ] All system_defaults keys are displayed grouped by category
- [ ] Each section independently saveable
- [ ] Browser warns on navigation with unsaved changes
- [ ] Change indicators work per-section

---

## Phase 9: Legal Entities — Operator Association

**File:** `portal-frontend/src/pages/admin/legal-entities/LegalEntitiesPage.tsx` (203 lines)
**Figma:** Node `15915:123972` (Юр. лица)

### Current State
- DataTable with INN (monospace), name, full_name, address
- CRUD modal (INN + name required)
- Status badge (active/inactive)

### What Figma Shows
1. **"All Networks" pool concept** — unassigned entities available for any operator
2. **Linked operators column** — show which operators are associated
3. **Designer note:** "Добавить второе и последующие юр.лица можно только из списка all networks. Если юр.лицо удаляется, то оно возвращается обратно в all networks"

### Changes Required

#### 9.1 Add "Связанные операторы" column
Modify `LegalEntitiesPage.tsx`:
- Add column "Операторы" showing count or comma-separated list of associated operator names
- Requires new API: `GET /admin/v1/legal-entities/{id}/operators` or include in list response
- If API doesn't return this data: show column with "—" placeholder

#### 9.2 Add INN validation
Russian INN validation (10 or 12 digits with checksum):
```ts
function validateINN(inn: string): boolean {
  if (!/^\d{10}$|^\d{12}$/.test(inn)) return false;
  // Checksum validation for 10-digit and 12-digit INNs
  // ... (standard algorithm)
  return true;
}
```

Add to create/edit modal with inline error: "Некорректный ИНН"

### Acceptance Criteria
- [ ] "Операторы" column shows associations (or placeholder)
- [ ] INN validation prevents saving invalid values
- [ ] Existing CRUD preserved

---

## Phase 10: Contracts — Enhanced UX

**File:** `portal-frontend/src/pages/admin/contracts/ContractsPage.tsx` (299 lines)
**Figma:** Node `17427:155489` (Договоры)

### Current State
- DataTable: contract_number, client_name, legal_entity, status, dates
- CRUD modal with SearchableSelect for client/legal_entity
- Status badge (active/expired/terminated)

### Changes Required

#### 10.1 Auto-expire contracts
Add visual indicator for contracts approaching expiration:
- If `end_date` is within 30 days: show warning badge "Истекает"
- If `end_date` is past: auto-show "Истёк" badge
- Computed on frontend from dates

#### 10.2 Filter by status
Add status filter to the page:
- Options: Все, Активные, Истекающие, Истёкшие, Расторгнутые
- "Истекающие" = end_date within 30 days

### Acceptance Criteria
- [ ] Expiring contracts highlighted visually
- [ ] Status filter works
- [ ] Existing CRUD preserved

---

## Phase 11: Cross-Cutting Improvements

These apply across the entire admin panel.

### 11.1 CSV Export Pattern (reusable)

Create a reusable export utility:

**New file:** `portal-frontend/src/utils/csvExport.ts`

```ts
export function exportToCsv(filename: string, headers: string[], rows: string[][]): void {
  const csv = [headers.join(','), ...rows.map(r => r.map(cell => `"${cell}"`).join(','))].join('\n');
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
```

Use in: DetalizationPage, AnalyticsPage, ContractsPage, BillingPage (transactions).

### 11.2 SSE for Real-Time Updates (optional, backend-dependent)

Replace polling with Server-Sent Events for:
- Dashboard metrics (currently 30s polling)
- Connections status (currently 10s polling)
- Message detail status updates

**Backend requirement:** Add SSE endpoint `GET /admin/v1/events/stream` with event types:
- `metrics_update` — real-time metrics
- `connection_status` — connection state changes
- `message_status` — individual message status changes

**Frontend:** Create `useEventStream` hook:
```ts
function useEventStream(url: string, onEvent: (type: string, data: any) => void): { connected: boolean; close: () => void }
```

**Note:** This is optional and backend-dependent. Skip if SSE endpoint doesn't exist.

### 11.3 Keyboard Shortcuts

The codebase already has `CommandPalette.tsx` and `useCommandPalette.ts`. Verify these work and add admin-specific commands:
- `Ctrl+K` — open command palette
- `/clients` → navigate to clients
- `/routes` → navigate to routes
- `/settings` → navigate to settings

### 11.4 Empty States

Audit all DataTable usages for proper empty states. Each page should show:
```tsx
<div className="text-center py-12 text-gray-500">
  <p className="text-lg font-medium">Нет данных</p>
  <p className="text-sm mt-1">Описание что делать, чтобы данные появились</p>
  <Button onClick={openCreateModal} className="mt-4">Создать первую запись</Button>
</div>
```

Pages to check: all 12 CRUD pages.

---

## Implementation Order

Phases are ordered by **impact** and **independence** (can be done in parallel).

### Sprint 1: High-Impact UX (Phases 1, 3, 5)
- **Phase 1:** Detalization — StatusTimeline, SmsPreview, CSV export, filter UX
- **Phase 3:** Routing — visual polish, legal entity validation
- **Phase 5:** Statistics — fixed-height table, sticky header/footer, date range

These 3 are independent and can be **parallelized across agents**.

### Sprint 2: Detail Pages (Phases 4, 7, 2)
- **Phase 4:** Sender Names — validation hints, operator template shortcut
- **Phase 7:** Operator Templates — variable highlighting, preview
- **Phase 2:** Connections — detail drawer, refresh control, log viewer

These 3 are independent and can be **parallelized across agents**.

### Sprint 3: Entity Enhancement (Phases 9, 10, 6)
- **Phase 9:** Legal Entities — operator association column, INN validation
- **Phase 10:** Contracts — auto-expire indicator, status filter
- **Phase 6:** MCC/MNC — legal entity section in operator detail, interdependent filters

Phase 6 depends on Phase 9 (legal entity ↔ operator backend). Do Phase 9 first, then Phase 6.

### Sprint 4: Settings & Cross-Cutting (Phases 8, 11)
- **Phase 8:** Settings — expand sections, unsaved changes warning
- **Phase 11:** CSV export utility, empty states, keyboard shortcuts

### Sprint 5: Optional Backend-Dependent
- SSE for real-time updates (Phase 11.2)
- Operator ↔ Legal Entity backend endpoints (Phase 6.1 backend)
- Connection logs endpoint (Phase 2.3 backend)

---

## Notes for AI Agents

1. **Read before modifying.** Always read the full file before making changes. Pages are 174-545 lines — manageable in one read.

2. **Preserve existing patterns.** The codebase uses consistent patterns:
   - `useState` + `useCallback` + `useEffect` for data fetching
   - `useToast()` for notifications
   - `Modal` + form state for create/edit
   - `ConfirmDialog` for destructive actions
   - `PageHeader` with breadcrumbs and action buttons

3. **Do not add new dependencies** without explicit approval. The existing stack (Radix UI, Recharts, Tailwind) covers all needs.

4. **Test by reading the dev server.** Run `cd portal-frontend && npm run dev` to start on port 3001. Admin panel at `http://localhost:3001/admin`.

5. **Backend proxy.** Vite dev server proxies `/admin/v1/*` to `http://localhost:8081`. Ensure admin-gateway is running.

6. **Figma reference.** When visual details are unclear, open the Figma URL with the node ID from the section map (Section 4). Convert `:` to `-` in node IDs for URLs.

7. **One phase = one commit.** Each phase should be a single, atomic commit with a descriptive message.

8. **No refactoring beyond scope.** Do not restructure files, extract components, or "improve" code outside the specific phase requirements.
