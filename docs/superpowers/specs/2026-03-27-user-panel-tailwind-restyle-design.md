# User Panel Tailwind Restyle

**Date:** 2026-03-27
**Scope:** Visual-only refactoring — replace inline styles with Tailwind CSS and shared UI components across the entire user panel.

## Problem

Auth pages and admin panel use Tailwind CSS + shared UI components (`Sidebar`, `PageHeader`, `DataTable`, etc.) and display correctly. User panel pages (Layout + 11 pages) use old inline `style={{}}` and raw HTML, resulting in inconsistent and broken-looking UI.

## Approach

Restyle the user panel to match the admin panel's visual style using the same Tailwind classes and shared component library. No functional changes — only markup and styling.

## Scope

### 1. Layout (App.tsx, lines 56–96)

**Current:** Inline-styled sidebar with raw `<nav>`, `<ul>`, `<Link>` elements.
**Target:** Use the shared `Sidebar` component (from `components/layout/Sidebar.tsx`) exactly as `AdminLayout` does. Extract Layout into its own file `components/layout/UserLayout.tsx`.

Navigation items remain the same:
- Dashboard, Messages, Providers, API Keys, Webhooks, Analytics, Sub-accounts, Profile, Audit Log

Footer: user email + logout button (same pattern as AdminLayout).

### 2. Pages — Pattern Mapping

Each page follows the same transformation pattern:

| Inline Pattern | Replacement |
|----------------|-------------|
| `<h2>Title</h2>` | `<PageHeader title="..." subtitle="..." actions={...} />` |
| Raw `<table>` with inline styles | `<DataTable columns={...} data={...} />` with pagination props |
| Raw `<input>`, `<select>`, `<textarea>` | `<Input>`, `<Select>` components |
| Raw `<button>` with inline styles | `<Button>` component (variant: primary/secondary/ghost/danger) |
| Inline filter forms | `<FilterBar filters={...} values={...} onChange={...} />` |
| `window.confirm()` / custom confirm divs | `<ConfirmDialog>` component |
| Custom modal divs | `<Modal>` component |
| Status text with colored inline styles | `<StatusBadge>` component |
| `style={{ color: 'red' }}` error divs | `<div className="text-danger ...">` |
| Flex/grid layouts with inline styles | Tailwind utility classes (`flex`, `grid`, `gap-*`, etc.) |

### 3. Pages — Individual Notes

1. **DashboardPage** (125 lines) — Sandbox banner → Tailwind alert. Stat cards → Tailwind grid with rounded cards.
2. **MessagesPage** (186 lines) — Filters → `FilterBar`. Table → `DataTable`. Pagination → DataTable built-in.
3. **ProvidersPage** (106 lines) — Table → `DataTable`. `alert()` delete → `ConfirmDialog`.
4. **ProviderWizardPage** (98 lines) — Uses own wizard components (WizardProgress, WizardNav, Steps). Only restyle the outer container and step wrapper divs.
5. **APIKeysPage** (335 lines) — Create form → `Modal` + `Input`. Keys table → `DataTable`. Revoke → `ConfirmDialog`. Scopes checkboxes → Tailwind styled.
6. **WebhooksPage** (459 lines) — Create/edit forms → `Modal` + `Input`/`Select`. Table → `DataTable`. Delete → `ConfirmDialog`. Test result → Tailwind alert.
7. **AnalyticsPage** (238 lines) — Period selector → Tailwind button group. Cards → Tailwind grid. Tables → `DataTable`.
8. **SubAccountsListPage** (256 lines) — Create form → `Modal` + `Input`. Table → `DataTable`. Status dots → `StatusBadge`.
9. **SubAccountDetailPage** (660 lines) — Tab navigation → Tailwind tab bar. Each tab's content → Tailwind + shared components. This is the largest page; sub-component functions (OverviewTab, MessagesTab, etc.) stay as local functions but get restyled.
10. **ProfilePage** (215 lines) — Form fields → `Input`. Buttons → `Button`. 2FA QR section → Tailwind card.
11. **AuditLogPage** (218 lines) — Filters → `FilterBar`. Table → `DataTable`. JSON details → Tailwind `<pre>` styling.

## Out of Scope

- No new features or API changes
- No changes to auth pages (already restyled)
- No changes to admin pages (already use Tailwind)
- No changes to shared UI components themselves
- No routing changes
- Wizard step components (Step1–Step6) — internal styling only if they use inline styles, otherwise leave as-is

## Constraints

- Preserve all existing functionality: API calls, state management, form validation, accessibility attributes (`aria-*`, `role`, focus traps)
- Use existing shared components without modification
- If a shared component doesn't fit a use case exactly, use Tailwind utilities directly (don't create new components for one-off cases)
