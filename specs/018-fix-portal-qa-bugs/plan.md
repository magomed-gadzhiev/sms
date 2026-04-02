# Implementation Plan: Исправление багов портала по результатам QA

**Branch**: `018-fix-portal-qa-bugs` | **Date**: 2026-04-01 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/018-fix-portal-qa-bugs/spec.md`

## Summary

Исправление 6 QA-багов в портале самообслуживания: застрявший дашборд (избыточный `profileApi.get()` без защиты от зависания), отсутствующие empty-state компоненты на 5 страницах, молчащая ошибка при создании API-ключа с длинным именем. Все изменения — frontend-only в `portal-frontend/src/pages/`.

## Technical Context

**Language/Version**: TypeScript 5.7 + React 19, Vite 6.0  
**Primary Dependencies**: React Router 7.1, Tailwind CSS 4.2, Radix UI  
**Storage**: N/A (frontend-only changes)  
**Testing**: Playwright (QA automated test script on production server)  
**Target Platform**: SPA в Docker-контейнере за nginx, production-сервер `72.56.232.202:18085`  
**Project Type**: Web SPA (portal-frontend)  
**Performance Goals**: Дашборд должен завершать загрузку в течение 5 секунд (FR-001)  
**Constraints**: Нет новых зависимостей; только bugfix изменения, не рефакторинг  
**Scale/Scope**: 7 файлов затронуты, все изменения строго в пределах page-компонентов

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. DDD | ✅ Pass | Frontend-only, нет изменений в domain/application слоях |
| II. Event-Driven | ✅ Pass | Нет Kafka-изменений |
| III. Contract-First APIs | ✅ Pass | API-контракты не меняются |
| IV. Observability | ✅ Pass | Нет изменений в метриках/логировании |
| V. Data Safety | ✅ Pass | Нет изменений в логике хранения данных |
| VI. Simplicity | ✅ Pass | Минимальные изменения; нет новых абстракций |

**No violations. No complexity tracking required.**

## Project Structure

### Documentation (this feature)

```text
specs/018-fix-portal-qa-bugs/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output — root cause analysis, decisions
├── data-model.md        # Phase 1 output — state changes, no schema migrations
├── quickstart.md        # Phase 1 output — what changes, how to test, how to deploy
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
portal-frontend/
└── src/
    └── pages/
        ├── dashboard/DashboardPage.tsx       # Bug 1: remove profileApi.get(), error fallback
        ├── messages/MessagesPage.tsx         # Bug 2: add empty-state component
        ├── campaigns/CampaignsPage.tsx       # Bug 3: useCallback for load()
        ├── templates/TemplatesPage.tsx       # Bug 4: add empty-state component
        ├── contacts/ContactListsPage.tsx     # Bug 5: useCallback for load()
        └── billing/BillingPage.tsx           # Bug 6: 404→0-balance, empty state
        └── api-keys/APIKeysPage.tsx          # Bug 7: name validation, inline error
```

**Structure Decision**: Single-project web application (frontend SPA). Only `portal-frontend/src/pages/` is affected. No new files needed — all changes are edits to existing page components.

---

## Fix Specifications

### Fix 1 — DashboardPage.tsx

**Problem**: `Promise.all([dashboardApi.get(), profileApi.get()])` — redundant profile fetch causes repeated 401 errors; if either promise hangs, `setLoading(false)` never executes.

**Change**:
1. Import `useAuth` and replace the `profile` state + `profileApi.get()` call with `const { user: profile } = useAuth()`.
2. Replace `Promise.all(...)` with a single `dashboardApi.get()` call.
3. The `.catch()` handler already calls `setError`; add `setData({} as DashboardData)` **or** simply let `data` remain `null` — the `if (!data)` branch already shows "Нет данных" (improve this message to show the onboarding checklist instead).
4. Ensure `.finally(() => setLoading(false))` is present (it already is).

**Result**: Dashboard always exits loading state — on success (shows stats), on error (shows onboarding/empty screen), on hanging server (catch fires eventually via browser timeout, or we add `Promise.race` with a timeout).

> Optional: wrap `dashboardApi.get()` in a `Promise.race` with a 10 s timeout promise that resolves to `null`, so the page shows the onboarding view within 10 seconds even if the server never responds. Implement only if the simple catch approach is insufficient.

---

### Fix 2 — MessagesPage.tsx

**Problem**: No explicit empty-state component — page shows DataTable's generic "Данные не найдены" cell.

**Change**: After `{error && ...}` block, add:
```tsx
{!loading && !error && (data?.messages?.length ?? 0) === 0 && (
  <div className="text-center py-12 text-gray-500">
    <p className="mb-2 font-medium">Сообщений пока нет</p>
    <p className="text-sm mb-4">Отправьте первое SMS через API или форму ниже</p>
    <Button variant="ghost" onClick={() => setShowSendModal(true)}>
      Отправить SMS
    </Button>
  </div>
)}
```
Render `<DataTable>` only when `loading || (data?.messages?.length ?? 0) > 0` — same pattern as CampaignsPage.

---

### Fix 3 — CampaignsPage.tsx

**Problem**: `load()` is a plain async function inside the component, not memoised. With React 18 concurrent mode, the function reference changes each render; `useEffect(() => { load(); }, [page])` is missing `load` in deps, which can cause stale-closure bugs. More importantly, if the server hangs, the effect's cleanup (when `page` changes) does not cancel the in-flight request — loading state can become stale.

**Change**:
```tsx
const load = useCallback(async () => {
  setLoading(true);
  setError('');
  try {
    const resp = await campaignsApi.list(page, perPage);
    setCampaigns(resp.campaigns ?? []);
    setTotal(resp.total ?? 0);
  } catch (err) {
    setError(err instanceof ApiError ? err.message : 'Не удалось загрузить рассылки');
  } finally {
    setLoading(false);
  }
}, [page]);

useEffect(() => {
  load();
}, [load]);
```

The existing empty-state JSX (`!loading && !error && campaigns.length === 0 && ...`) is correct and unchanged.

---

### Fix 4 — TemplatesPage.tsx

**Problem**: When `loading=false, templates=[]`, the component falls through to render `<DataTable>` with empty data — no dedicated empty-state component.

**Change**: Before `<DataTable>`, add:
```tsx
{!loading && !error && templates.length === 0 && (
  <div className="text-center py-12 text-gray-500">
    <p className="mb-2 font-medium">Шаблоны не созданы</p>
    <p className="text-sm mb-4">Создайте первый шаблон для массовых рассылок</p>
    <Button variant="ghost" onClick={openCreateForm}>
      Создать шаблон
    </Button>
  </div>
)}
```
Render `<DataTable>` only when `loading || templates.length > 0`.

Also memoize `fetchTemplates` dependency — it already uses `useCallback`, so `useEffect([fetchTemplates])` is correct.

---

### Fix 5 — ContactListsPage.tsx

**Problem**: Same as CampaignsPage — `load()` is not memoised.

**Change**: Same `useCallback` pattern as Fix 3.

The existing empty-state JSX is correct and unchanged.

---

### Fix 6 — BillingPage.tsx

**Problem A**: `billingApi.getBalance()` returns 404 for fresh users (no billing account yet). The catch calls `setBalanceError('Не удалось загрузить данные биллинга')`, displaying an error in the balance card instead of showing "0.00 ₽".

**Problem B**: Transactions DataTable relies on DataTable's generic "Данные не найдены" — no page-level empty state context.

**Change A** (balance):
```tsx
.catch((err) => {
  if (err instanceof ApiError && err.status === 404) {
    // New account — no billing record yet; show zero balance
    setBalance({ client_id: '', balance: '0.00', currency: 'RUB' });
  } else {
    setBalanceError('Не удалось загрузить данные биллинга');
  }
})
```

**Change B** (transactions): After FilterBar, add:
```tsx
{!loading && !error && (data?.transactions?.length ?? 0) === 0 && (
  <div className="text-center py-8 text-gray-500 text-sm">
    История транзакций пуста
  </div>
)}
```
Render `<DataTable>` only when `loading || (data?.transactions?.length ?? 0) > 0`.

---

### Fix 7 — APIKeysPage.tsx

**Problem**: No client-side name length validation; server 400 is caught by the generic `setError(...)`, shown as red text at the top of the form — not associated with the "Название" field.

**Change**:
1. Add `const [nameError, setNameError] = useState('')` state.
2. In `handleCreate`:
   - Before the API call: `if (name.length > 100) { setNameError('Имя не должно превышать 100 символов'); setCreating(false); return; }`
   - In `.catch`: if the API returns 400, set `setNameError(err.message)` instead of (or in addition to) the generic error.
3. In the "Название" `<Input>`:
   - Add `maxLength={100}` attribute.
   - Add `error={nameError || undefined}` prop so the Input component renders the error inline.
4. Reset `nameError` at the start of `handleCreate` (after `setCreating(true)`).

---

## Complexity Tracking

No violations to justify.

---

## Post-Design Constitution Check

| Principle | Status |
|-----------|--------|
| VI. Simplicity | ✅ — no new abstractions; fixes are minimal surgical changes |
| All others | ✅ — unchanged |
