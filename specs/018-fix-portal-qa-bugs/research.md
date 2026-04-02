# Research: 018-fix-portal-qa-bugs

## Bug 1: Dashboard stuck in "Загрузка дашборда..."

### Evidence
- Screenshot `test-screenshots/server-full-qa-1775019984296/01-register.png` confirms: after registration → auto-redirect to `/dashboard`, the page shows "Загрузка дашборда..." indefinitely (sidebar visible with authenticated user, but main content never loads).
- QA `netErrors` records two `401 GET /portal/v1/profile` errors — one from initial AuthContext fetch (before auth), one from DashboardPage's **redundant** `profileApi.get()` call.

### Root Cause
`DashboardPage.tsx` uses `Promise.all([dashboardApi.get(), profileApi.get()])`. Both issues compound:
1. **Redundant call**: `profileApi.get()` is already performed by `AuthContext.useEffect`. Calling it again inside DashboardPage causes a second network request and, critically, can produce a 401 in edge cases (race on session propagation right after registration).
2. **No error recovery**: If either promise never settles (e.g., `dashboardApi.get()` hangs on the server for fresh users with no data), `setLoading(false)` is never called — the dashboard stays forever in loading state.

### Decision
- Remove `profileApi.get()` from DashboardPage; use `useAuth()` to read the already-loaded profile.
- Replace `Promise.all` with a single `dashboardApi.get()` call.
- If `dashboardApi.get()` fails **for any reason**, call `setLoading(false)` and fall through to the onboarding/empty-state view instead of leaving a spinner.

**Why**: Eliminates the redundant network call, fixes the profile-race 401 errors, and prevents infinite loading when the API is slow/unavailable for fresh users. The onboarding view (already coded in DashboardPage) serves as the empty-state fallback for new accounts.

---

## Bug 2: Empty-state pages fail QA

### Evidence
QA stress report marks five pages as `ok: false` for empty-state tests: `/messages`, `/campaigns`, `/templates`, `/contact-lists`, `/billing`.  
Pages that **passed** (`/providers`, `/segments`) both render an explicit empty-state component with visible text when the list is empty.

### Root Cause (per page)

| Page | Current empty-state handling | Why QA fails |
|------|------------------------------|--------------|
| MessagesPage | None — relies on DataTable's generic "Данные не найдены" cell | No informative text + no CTA button |
| CampaignsPage | Has "Рассылки не созданы" div, but conditioned on `!loading && !error && campaigns.length === 0`. The `load()` function is not memoised; if the server hangs `setLoading(false)` is never called. | Loading may stay `true` indefinitely |
| TemplatesPage | `if (loading && templates.length === 0) return <spinner>` — after load, no explicit empty state, just DataTable | No informative text + no CTA |
| ContactListsPage | Has "Контактные базы не созданы" div — same `load()` un-memoised issue as Campaigns | Loading may stay `true` |
| BillingPage | No empty state; `getBalance()` 404 for fresh users → shows error text, not "0 balance" | Error UI instead of empty-state UI |

### Decision
- **MessagesPage, TemplatesPage**: add explicit empty-state components (title + helper text + CTA button).
- **CampaignsPage, ContactListsPage**: memoize `load` with `useCallback` so React doesn't accidentally omit it from the deps graph; the existing empty-state markup is otherwise correct.
- **BillingPage**: catch 404 on `getBalance()` and treat it as "balance = 0.00 RUB" (new account with no billing record yet); keep the error path only for 5xx.

**Alternatives considered**: adding a `setTimeout`-based fallback to force `setLoading(false)` after N seconds. Rejected — treating a server hang as "empty data" is misleading and masks backend bugs. The correct fix is proper error handling (`.catch` already calls `setLoading(false)`); the billing 404 is a legitimate server response that the client should interpret as "no account yet".

---

## Bug 3: API-key creation — no feedback on long name

### Evidence
`netErrors` in QA report: `{ "status": 400, "method": "POST", "url": ".../portal/v1/api-keys" }`.  
QA stress test "Long API key name" → `ok: false`.

### Root Cause
`APIKeysPage.tsx` has no `maxLength` constraint on the name field and no code to map a 400 response to a field-level error. The generic `setError(...)` renders red text at the top of the form — the user cannot tell which field is wrong.

### Decision
- Add `maxLength={100}` (and matching client-side check) on the "Название" input.
- Extract `nameError` state, separate from the generic `error` state.
- On server 400 response during create, set `nameError` to the server message (instead of or in addition to generic error).
- Display `nameError` as a red helper text directly below the name input.

**Why 100 chars**: Spec §Assumptions — "Максимальная длина имени API-ключа задана серверной логикой; если она явно не задокументирована, принимаем 100 символов как разумный лимит".

---

## Bug 4: Repeated 401 profile errors in console

### Root Cause
Same as Bug 1: DashboardPage calls `profileApi.get()` while AuthContext has already called it. The AuthContext call happens before the user is authenticated (initial app load → 401), and the DashboardPage call is a duplicate once the user IS authenticated.

### Decision
Resolved by the same fix as Bug 1: remove `profileApi.get()` from DashboardPage.

---

## Out-of-scope items (per spec §Assumptions)
- Register form field label `Email *` vs `Электронная почта` — localization issue, separate ticket.
- Responsive/mobile layout fixes.
- `/segments` and `/providers` pages — already pass QA.
