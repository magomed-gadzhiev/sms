# User Panel Tailwind Restyle — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace all inline styles in the user panel (layout + 11 pages) with Tailwind CSS classes and shared UI components to match the admin panel's visual style.

**Architecture:** Extract the inline-styled `Layout` from `App.tsx` into a dedicated `UserLayout` component that uses the shared `Sidebar`. Each page replaces raw HTML tables/forms/buttons with `DataTable`, `FilterBar`, `PageHeader`, `Button`, `Input`, `Select`, `Modal`, `ConfirmDialog`, `StatusBadge`, and `StatCard` components. No functional changes — only markup and styling.

**Tech Stack:** React 19, TypeScript, Tailwind CSS, existing shared UI components

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `src/components/layout/UserLayout.tsx` | User panel layout with Sidebar |
| Modify | `src/App.tsx` | Remove inline Layout, import UserLayout |
| Modify | `src/pages/dashboard/DashboardPage.tsx` | Tailwind cards + alert |
| Modify | `src/pages/messages/MessagesPage.tsx` | FilterBar + DataTable |
| Modify | `src/pages/providers/ProvidersPage.tsx` | PageHeader + DataTable + ConfirmDialog |
| Modify | `src/pages/providers/ProviderWizardPage.tsx` | Tailwind wrapper only |
| Modify | `src/pages/api-keys/APIKeysPage.tsx` | Modal + DataTable + ConfirmDialog |
| Modify | `src/pages/webhooks/WebhooksPage.tsx` | Modal + DataTable + ConfirmDialog |
| Modify | `src/pages/analytics/AnalyticsPage.tsx` | StatCard + DataTable + Tailwind button group |
| Modify | `src/pages/sub-accounts/SubAccountsListPage.tsx` | Modal + DataTable + StatusBadge |
| Modify | `src/pages/sub-accounts/SubAccountDetailPage.tsx` | Tailwind tabs + shared components per tab |
| Modify | `src/pages/profile/ProfilePage.tsx` | Input + Button + Tailwind card for 2FA |
| Modify | `src/pages/audit/AuditLogPage.tsx` | FilterBar + DataTable |

---

### Task 1: UserLayout + App.tsx

**Files:**
- Create: `src/components/layout/UserLayout.tsx`
- Modify: `src/App.tsx:44-96`

- [ ] **Step 1: Create UserLayout.tsx** — mirrors AdminLayout pattern with Sidebar, SkipLink, user footer
- [ ] **Step 2: Update App.tsx** — remove inline Layout function, NAV_ITEMS, import UserLayout, use it in RequireAuth
- [ ] **Step 3: Verify build** — `npm run build`
- [ ] **Step 4: Commit**

### Task 2: DashboardPage

**Files:**
- Modify: `src/pages/dashboard/DashboardPage.tsx`

- [ ] **Step 1:** Replace sandbox banner with Tailwind alert (`bg-amber-50 border border-amber-400 rounded-lg p-3`)
- [ ] **Step 2:** Replace `<h2>` with `<PageHeader title="Dashboard" />`
- [ ] **Step 3:** Replace inline stat cards with Tailwind grid + card styling matching StatCard pattern
- [ ] **Step 4:** Replace error div with `text-red-600`
- [ ] **Step 5: Commit**

### Task 3: MessagesPage

**Files:**
- Modify: `src/pages/messages/MessagesPage.tsx`

- [ ] **Step 1:** Replace `<h2>` with `<PageHeader title="Messages" />`
- [ ] **Step 2:** Replace filter form with `<FilterBar>` (status select, date_from, date_to, destination text)
- [ ] **Step 3:** Replace raw table + pagination with `<DataTable>` using columns array and render functions
- [ ] **Step 4: Commit**

### Task 4: ProvidersPage

**Files:**
- Modify: `src/pages/providers/ProvidersPage.tsx`

- [ ] **Step 1:** Replace header with `<PageHeader title="SMPP Providers" actions={<Button>+ Add Provider</Button>} />`
- [ ] **Step 2:** Replace table with `<DataTable>` + `<StatusBadge>` for active/inactive
- [ ] **Step 3:** Replace `confirm()` delete with `<ConfirmDialog>`
- [ ] **Step 4: Commit**

### Task 5: ProviderWizardPage

**Files:**
- Modify: `src/pages/providers/ProviderWizardPage.tsx`

- [ ] **Step 1:** Replace `<h2>` with `<PageHeader>`
- [ ] **Step 2:** Replace outer container inline styles with Tailwind (`max-w-2xl mx-auto`)
- [ ] **Step 3:** Replace step wrapper with `border border-gray-200 rounded-lg p-6`
- [ ] **Step 4:** Replace error `<p>` with `text-red-600`
- [ ] **Step 5: Commit**

### Task 6: APIKeysPage

**Files:**
- Modify: `src/pages/api-keys/APIKeysPage.tsx`

- [ ] **Step 1:** Replace header with `<PageHeader>` + `<Button>`
- [ ] **Step 2:** Replace create form div with `<Modal>` + `<Input>` components
- [ ] **Step 3:** Replace created key banner with Tailwind success alert
- [ ] **Step 4:** Replace keys table with `<DataTable>` + `<StatusBadge>`
- [ ] **Step 5:** Replace inline revoke confirm with `<ConfirmDialog>`
- [ ] **Step 6: Commit**

### Task 7: WebhooksPage

**Files:**
- Modify: `src/pages/webhooks/WebhooksPage.tsx`

- [ ] **Step 1:** Replace header with `<PageHeader>` + `<Button>`
- [ ] **Step 2:** Replace create/edit forms with `<Modal>` + form fields
- [ ] **Step 3:** Replace secret/test banners with Tailwind alerts
- [ ] **Step 4:** Replace table with `<DataTable>` + `<Badge>` for event types + `<StatusBadge>`
- [ ] **Step 5:** Replace inline delete confirm with `<ConfirmDialog>`
- [ ] **Step 6: Commit**

### Task 8: AnalyticsPage

**Files:**
- Modify: `src/pages/analytics/AnalyticsPage.tsx`

- [ ] **Step 1:** Replace `<h2>` with `<PageHeader>`
- [ ] **Step 2:** Replace period buttons with Tailwind button group
- [ ] **Step 3:** Replace summary cards with `<StatCard>` grid
- [ ] **Step 4:** Replace timeline/country tables with `<DataTable>`
- [ ] **Step 5: Commit**

### Task 9: SubAccountsListPage

**Files:**
- Modify: `src/pages/sub-accounts/SubAccountsListPage.tsx`

- [ ] **Step 1:** Replace header with `<PageHeader>` + `<Button>`
- [ ] **Step 2:** Replace create form with `<Modal>` + `<Input>` fields
- [ ] **Step 3:** Replace table with `<DataTable>` + `<StatusBadge>`
- [ ] **Step 4: Commit**

### Task 10: SubAccountDetailPage

**Files:**
- Modify: `src/pages/sub-accounts/SubAccountDetailPage.tsx`

- [ ] **Step 1:** Replace back button + name header with `<PageHeader>` + breadcrumbs + `<StatusBadge>`
- [ ] **Step 2:** Replace tab navigation with Tailwind tab bar
- [ ] **Step 3:** Restyle OverviewTab — StatCard grid, Tailwind card forms, ConfirmDialog for delete
- [ ] **Step 4:** Restyle MessagesTab — DataTable
- [ ] **Step 5:** Restyle AnalyticsTab — Tailwind button group + StatCard + DataTable
- [ ] **Step 6:** Restyle APIKeysTab — DataTable + StatusBadge
- [ ] **Step 7:** Restyle WebhooksTab — DataTable + Badge + StatusBadge
- [ ] **Step 8: Commit**

### Task 11: ProfilePage

**Files:**
- Modify: `src/pages/profile/ProfilePage.tsx`

- [ ] **Step 1:** Replace `<h2>` with `<PageHeader>`
- [ ] **Step 2:** Replace form fields with `<Input>` + `<Button>` components
- [ ] **Step 3:** Restyle 2FA section with Tailwind card (`border rounded-lg p-6`)
- [ ] **Step 4: Commit**

### Task 12: AuditLogPage

**Files:**
- Modify: `src/pages/audit/AuditLogPage.tsx`

- [ ] **Step 1:** Replace `<h2>` with `<PageHeader>`
- [ ] **Step 2:** Replace filter form with `<FilterBar>`
- [ ] **Step 3:** Replace table + pagination with `<DataTable>`
- [ ] **Step 4: Commit**
