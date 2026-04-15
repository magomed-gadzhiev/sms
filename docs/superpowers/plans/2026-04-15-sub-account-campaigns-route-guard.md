# Sub-Account Campaigns Endpoint & Route Guard — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow aggregators to view their sub-accounts' campaigns and prevent sub-account users from accessing the `/sub-accounts` route.

**Architecture:** Add `GetSubAccountCampaigns` handler to existing `SubAccountHandlers` (same pattern as messages/analytics/webhooks). Add `RequireReseller` frontend guard component. Wire up the CampaignsTab UI in SubAccountDetailPage.

**Tech Stack:** Go 1.24 (gorilla/mux, gRPC), TypeScript 5.7 + React 19, Tailwind CSS

---

### Task 1: Backend — inject campaignClient into SubAccountHandlers

**Files:**
- Modify: `internal/gateway/portal/handlers/sub_accounts.go:22-51`

- [ ] **Step 1: Add campaignClient field and constructor param**

In `internal/gateway/portal/handlers/sub_accounts.go`, add `campaignClient` to the struct and constructor:

```go
// In SubAccountHandlers struct (line 22), add after webhookClient:
campaignClient  campaignv1.CampaignServiceClient

// In NewSubAccountHandlers function signature (line 33), add param:
campaignClient campaignv1.CampaignServiceClient,

// In the return block (line 42), add:
campaignClient:  campaignClient,
```

Also add the import:

```go
campaignv1 "github.com/smpp-server/smpp-server/api/proto/campaignv1"
```

- [ ] **Step 2: Update router.go constructor call**

In `internal/gateway/portal/router/router.go`, find where `NewSubAccountHandlers` is called and add the `campaignClient` argument. The `campaignHandlers` is already available in the function signature as a param — use the same `campaignv1.CampaignServiceClient` that was passed to `NewCampaignHandlers`. 

Note: The `SetupRouter` receives `campaignHandlers *handlers.CampaignHandlers` (line 47), but we need the raw gRPC client. Check where `SetupRouter` is called (likely in `cmd/` or `main.go`) — the `campaignClient` gRPC conn should be passed through.

**Alternative (simpler):** If `campaignClient` is not directly available in `SetupRouter`, add it as a new parameter to `SetupRouter`.

- [ ] **Step 3: Verify compilation**

Run: `cd /c/projects/sms && go build ./internal/gateway/portal/...`
Expected: compiles without errors.

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/sub_accounts.go internal/gateway/portal/router/router.go
git commit -m "feat: inject campaignClient into SubAccountHandlers"
```

---

### Task 2: Backend — add GetSubAccountCampaigns handler

**Files:**
- Modify: `internal/gateway/portal/handlers/sub_accounts.go`
- Modify: `internal/gateway/portal/router/router.go:170`

- [ ] **Step 1: Add GetSubAccountCampaigns method**

Add to `internal/gateway/portal/handlers/sub_accounts.go` (after the existing `GetSubAccountWebhooks` method):

```go
// GetSubAccountCampaigns обрабатывает GET /sub-accounts/{id}/campaigns
func (h *SubAccountHandlers) GetSubAccountCampaigns(w http.ResponseWriter, r *http.Request) {
	parentClientID, appErr := h.getParentClientID(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	vars := mux.Vars(r)
	subAccountID := vars["id"]
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("ID суб-аккаунта обязателен"))
		return
	}

	// Verify ownership
	_, err := h.clientClient.GetSubAccount(r.Context(), &clientv1.GetSubAccountRequest{
		SubAccountId:   subAccountID,
		ParentClientId: parentClientID,
	})
	if err != nil {
		log.Error().Err(err).Str("sub_account_id", subAccountID).Msg("суб-аккаунт не найден или не принадлежит текущему клиенту")
		respondGRPCError(w, err)
		return
	}

	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	status := r.URL.Query().Get("status")

	resp, err := h.campaignClient.ListCampaigns(r.Context(), &campaignv1.ListCampaignsRequest{
		ClientId: subAccountID,
		Status:   status,
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		log.Error().Err(err).Str("sub_account_id", subAccountID).Msg("ошибка получения кампаний суб-аккаунта")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Register route**

In `internal/gateway/portal/router/router.go`, after line 170 (`subAccounts.HandleFunc("/{id}/webhooks", ...)`), add:

```go
subAccounts.HandleFunc("/{id}/campaigns", subAccountHandlers.GetSubAccountCampaigns).Methods("GET")
```

- [ ] **Step 3: Verify compilation**

Run: `cd /c/projects/sms && go build ./internal/gateway/portal/...`
Expected: compiles without errors.

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/sub_accounts.go internal/gateway/portal/router/router.go
git commit -m "feat: add GET /sub-accounts/{id}/campaigns endpoint"
```

---

### Task 3: Frontend — add campaigns API method in client.ts

**Files:**
- Modify: `portal-frontend/src/api/client.ts:229`

- [ ] **Step 1: Add campaigns method to subAccountsApi**

In `portal-frontend/src/api/client.ts`, add after the `webhooks` method (line 229) inside `subAccountsApi`:

```ts
campaigns: (id: string, params?: Record<string, string>) => {
  const qs = params ? new URLSearchParams(params).toString() : '';
  return apiFetch<{ campaigns: Array<{ id: string; name: string; status: string; total_recipients: number; delivered: number; created_at: string }>; total: number }>(`/sub-accounts/${id}/campaigns${qs ? `?${qs}` : ''}`);
},
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat: add subAccountsApi.campaigns method"
```

---

### Task 4: Frontend — implement CampaignsTab in SubAccountDetailPage

**Files:**
- Modify: `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx:536-551`

- [ ] **Step 1: Replace placeholder CampaignsTab**

In `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx`, replace the `CampaignsTab` function (lines 536-551) with:

```tsx
const STATUS_CONFIG: Record<string, { variant: 'default' | 'info' | 'warning' | 'success' | 'danger'; label: string }> = {
  draft: { variant: 'default', label: 'Черновик' },
  running: { variant: 'info', label: 'Запущена' },
  paused: { variant: 'warning', label: 'На паузе' },
  completed: { variant: 'success', label: 'Завершена' },
  cancelled: { variant: 'danger', label: 'Отменена' },
};

interface CampaignItem {
  id: string;
  name: string;
  status: string;
  total_recipients: number;
  delivered: number;
  created_at: string;
}

const campaignColumns: Column<CampaignItem>[] = [
  { key: 'name', header: 'Название' },
  {
    key: 'status',
    header: 'Статус',
    render: (c) => {
      const cfg = STATUS_CONFIG[c.status] || { variant: 'default' as const, label: c.status };
      return <Badge variant={cfg.variant}>{cfg.label}</Badge>;
    },
  },
  {
    key: 'total_recipients',
    header: 'Получатели',
    render: (c) => <>{c.total_recipients?.toLocaleString() ?? '—'}</>,
  },
  {
    key: 'delivered',
    header: 'Доставлено',
    render: (c) => <>{c.delivered?.toLocaleString() ?? '—'}</>,
  },
  {
    key: 'created_at',
    header: 'Создана',
    render: (c) => <>{new Date(c.created_at).toLocaleDateString()}</>,
  },
];

function CampaignsTab({ subAccountId }: { subAccountId: string }) {
  const [campaigns, setCampaigns] = useState<CampaignItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    setLoading(true);
    setError('');
    subAccountsApi
      .campaigns(subAccountId, { page: String(page), per_page: '20' })
      .then((data) => {
        setCampaigns((data as { campaigns: CampaignItem[]; total: number }).campaigns || []);
        setTotal((data as { campaigns: CampaignItem[]; total: number }).total || 0);
      })
      .catch((err) => setError(err instanceof ApiError ? err.message : 'Ошибка загрузки кампаний'))
      .finally(() => setLoading(false));
  }, [subAccountId, page]);

  if (loading) return <div className="py-10 text-center text-gray-400">Загрузка кампаний...</div>;
  if (error) return <div className="py-10 text-center text-red-500">{error}</div>;
  if (campaigns.length === 0) {
    return (
      <div className="border border-dashed border-gray-300 rounded-lg p-10 text-center">
        <p className="text-gray-500 font-medium mb-1">Нет кампаний</p>
        <p className="text-sm text-gray-400">У этого суб-аккаунта пока нет кампаний.</p>
      </div>
    );
  }

  return (
    <DataTable<CampaignItem>
      columns={campaignColumns}
      data={campaigns}
      total={total}
      page={page}
      pageSize={20}
      onPageChange={setPage}
      keyField="id"
    />
  );
}
```

- [ ] **Step 2: Update CampaignsTab invocation to pass subAccountId**

Find where `<CampaignsTab />` is rendered (in the tab switch), and update to pass the sub-account ID:

```tsx
// Change:  {activeTab === 'campaigns' && <CampaignsTab />}
// To:      {activeTab === 'campaigns' && <CampaignsTab subAccountId={id!} />}
```

(`id` comes from `useParams` already defined at the top of the component.)

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /c/projects/sms/portal-frontend && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx
git commit -m "feat: implement CampaignsTab with real data in SubAccountDetailPage"
```

---

### Task 5: Frontend — create RequireReseller guard

**Files:**
- Create: `portal-frontend/src/components/RequireReseller.tsx`
- Modify: `portal-frontend/src/App.tsx:129-130`

- [ ] **Step 1: Create RequireReseller component**

Create `portal-frontend/src/components/RequireReseller.tsx`:

```tsx
import { Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

interface RequireResellerProps {
  children: React.ReactNode;
}

export function RequireReseller({ children }: RequireResellerProps) {
  const { isAuthenticated, user, loading } = useAuth();

  if (loading) return <div className="p-8 text-center text-gray-400">Loading...</div>;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  if (!user?.is_reseller) return <Navigate to="/dashboard" replace />;

  return <>{children}</>;
}
```

- [ ] **Step 2: Import and apply in App.tsx**

In `portal-frontend/src/App.tsx`:

Add import (after line 4):
```tsx
import { RequireReseller } from './components/RequireReseller';
```

Replace lines 129-130:
```tsx
// Before:
<Route path="/sub-accounts" element={<SubAccountsListPage />} />
<Route path="/sub-accounts/:id" element={<SubAccountDetailPage />} />

// After:
<Route path="/sub-accounts" element={<RequireReseller><SubAccountsListPage /></RequireReseller>} />
<Route path="/sub-accounts/:id" element={<RequireReseller><SubAccountDetailPage /></RequireReseller>} />
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /c/projects/sms/portal-frontend && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/components/RequireReseller.tsx portal-frontend/src/App.tsx
git commit -m "feat: add RequireReseller guard for /sub-accounts routes"
```
