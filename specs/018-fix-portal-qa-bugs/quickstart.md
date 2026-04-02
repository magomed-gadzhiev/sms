# Quickstart: 018-fix-portal-qa-bugs

## What this feature fixes

Six QA failures on the production portal (`http://72.56.232.202:18085`):

| # | Symptom | Root cause | Fix |
|---|---------|------------|-----|
| 1 | Dashboard stuck in "Загрузка дашборда..." | Redundant `profileApi.get()` in DashboardPage; no error recovery | Remove redundant call; use `useAuth()`; handle errors with fallback to onboarding view |
| 2 | Empty state /messages | No empty-state component | Add empty-state with CTA |
| 3 | Empty state /campaigns | `load()` not memoised — loading can hang | Wrap in `useCallback` |
| 4 | Empty state /templates | No empty-state component | Add empty-state with CTA |
| 5 | Empty state /contact-lists | `load()` not memoised | Wrap in `useCallback` |
| 6 | Empty state /billing | Balance 404 shown as error; no empty-state for fresh users | Treat 404 as 0-balance |
| 7 | Long API key name silently fails | No client-side validation; server 400 not shown inline | Add `maxLength`, show error next to field |

## Files changed

```text
portal-frontend/src/
├── pages/dashboard/DashboardPage.tsx       — remove profileApi.get(), error fallback
├── pages/messages/MessagesPage.tsx         — add empty state
├── pages/campaigns/CampaignsPage.tsx       — useCallback for load()
├── pages/templates/TemplatesPage.tsx       — add empty state
├── pages/contacts/ContactListsPage.tsx     — useCallback for load()
├── pages/billing/BillingPage.tsx           — 404→0-balance, transactions empty state
└── pages/api-keys/APIKeysPage.tsx          — name validation + inline error
```

## Testing

Run the automated QA script against the production server after deploying. Expected result:
- `stress[Empty state /messages]` → `ok: true`
- `stress[Empty state /campaigns]` → `ok: true`
- `stress[Empty state /templates]` → `ok: true`
- `stress[Empty state /contact-lists]` → `ok: true`
- `stress[Empty state /billing]` → `ok: true`
- `stress[Long API key name]` → `ok: true`
- Dashboard screenshot: no "Загрузка дашборда..." after 5 s
- Console: no repeated 401 errors for `/portal/v1/profile`

## Deploy

```bash
# Push to GitHub, then on server:
scripts/server.sh deploy portal-frontend
```
