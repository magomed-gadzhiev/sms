# Data Model: 018-fix-portal-qa-bugs

## Scope

This feature contains **frontend-only bug fixes**. No new database tables, migrations, or backend changes are required. All changes are in `portal-frontend/src/`.

## Affected Component State

### DashboardPage — state before / after

| State field | Before | After |
|-------------|--------|-------|
| `profile` | `useState<ProfileData \| null>(null)` — populated via redundant `profileApi.get()` | **Removed** — profile read directly from `useAuth()` |
| `loading` | `true` until `Promise.all` settles | `true` until `dashboardApi.get()` settles (or fails) |
| `data` | Set only on success | Set on success; remains `null` on failure (shows onboarding) |
| `error` | Set on any failure | Set on any failure; page still transitions out of loading state |

### APIKeysPage — new state

| State field | Type | Purpose |
|-------------|------|---------|
| `nameError` | `string` | Validation message shown below the "Название" input field |

### BillingPage — fetch error handling

| Scenario | Before | After |
|----------|--------|-------|
| `getBalance()` returns 404 | Shows "Не удалось загрузить данные биллинга" error | Shows balance as 0.00 RUB (empty state) |
| `getBalance()` returns 5xx | Shows error | Shows error (unchanged) |

## No Schema Changes

No PostgreSQL migrations, no Kafka event changes, no gRPC proto changes, no Redis key changes. This feature is purely UI state management and error handling.
