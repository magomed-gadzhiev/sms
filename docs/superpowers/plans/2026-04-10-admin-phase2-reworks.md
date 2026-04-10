# Admin Panel Phase 2 — Frontend Reworks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Phase 2 of the admin panel gap analysis: rework Statistics page (grouping tabs + table view), add Sender Name detail page with operator registration grid, and enhance the Countries/MCC-MNC page with operator detail view and 2-step creation wizard.

**Architecture:** Pure frontend changes — no new backend endpoints required. All three reworks enhance existing pages using API calls that already exist. New pages/components added alongside existing ones; existing routes and imports remain untouched.

**Tech Stack:** TypeScript 5.7, React 19, Vite 6.0, React Router 7.1, Tailwind CSS 4.2, Radix UI, existing component library (`DataTable`, `FilterBar`, `Modal`, `Badge`, `Button`, `PageHeader`, `TimelineEvent`).

---

## File Map

### REWORK-5: Статистика (Statistics)

| Action | File |
|--------|------|
| Modify | `portal-frontend/src/pages/admin/AnalyticsPage.tsx` |
| Modify | `portal-frontend/src/api/admin.ts` — add `getGroupedStats` |

### REWORK-1: Имена отправителей (Sender Names)

| Action | File |
|--------|------|
| Modify | `portal-frontend/src/pages/admin/SenderNamesAdminPage.tsx` — add row click nav |
| Create | `portal-frontend/src/pages/admin/sender-names/SenderNameDetailPage.tsx` |
| Modify | `portal-frontend/src/api/admin.ts` — add `get`, `operatorRegistrations` to `adminSenderNamesApi` |
| Modify | `portal-frontend/src/App.tsx` — add `/admin/sender-names/:id` route |

### REWORK-3: Справочник MCC/MNC (Countries/Operators)

| Action | File |
|--------|------|
| Modify | `portal-frontend/src/pages/admin/CountriesPage.tsx` — add flat MCC/MNC table tab + operator detail panel |
| Modify | `portal-frontend/src/api/admin.ts` — no new endpoints needed (all exist) |

---

## Task 1: Statistics — Grouped Table View

**Files:**
- Modify: `portal-frontend/src/api/admin.ts`
- Modify: `portal-frontend/src/pages/admin/AnalyticsPage.tsx`

### What to build

Replace the current chart-only view with:
- Three tab buttons: "По дням" | "По операторам" | "По странам"
- A fixed-height (`h-[520px]`) scrollable table with sticky header
- A sticky summary row (totals) at the bottom of the table
- Keep existing stat cards at the top
- Keep existing charts as supplementary (below the table)

### Step-by-step

- [ ] **Step 1: Add `getGroupedStats` to admin API**

Open `portal-frontend/src/api/admin.ts`. After the existing `analyticsAdminApi` object (around line 305), add a new type and extend the API:

```typescript
export interface GroupedStatRow {
  label: string;        // date string for 'day', operator name for 'operator', country name for 'country'
  sent: number;
  delivered: number;
  failed: number;
  delivery_rate: number;
  cost: number;
}

export interface GroupedStatsResponse {
  rows: GroupedStatRow[];
  totals: {
    sent: number;
    delivered: number;
    failed: number;
    delivery_rate: number;
    cost: number;
  };
}
```

Then extend `analyticsAdminApi` — replace the existing `getStats` call signature to also accept `group_by: 'day' | 'operator' | 'country'` (it already does), and add:

```typescript
export const analyticsAdminApi = {
  // existing methods...
  getStats: (params: { client_id?: string; from?: string; to?: string; group_by?: string }) =>
    adminFetch<unknown>(`/analytics/stats${qs(params)}`),
  generateReport: (data: unknown) =>
    adminFetch<unknown>('/analytics/reports', { method: 'POST', body: JSON.stringify(data) }),
  getRealTimeMetrics: () => adminFetch<RealTimeMetrics>('/analytics/metrics/realtime'),
  getProviderPerformance: (id: string, params?: { from?: string; to?: string }) =>
    adminFetch<unknown>(`/analytics/providers/${id}/performance${qs(params || {})}`),
  getGroupedStats: (params: { from: string; to: string; group_by: 'day' | 'operator' | 'country'; client_id?: string }) =>
    adminFetch<GroupedStatsResponse>(`/analytics/stats${qs(params)}`),
};
```

- [ ] **Step 2: Rewrite AnalyticsPage.tsx**

Replace the entire content of `portal-frontend/src/pages/admin/AnalyticsPage.tsx` with:

```typescript
import { useState, useEffect, useCallback } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar } from 'recharts';
import { PageHeader } from '../../components/layout/PageHeader';
import { StatCard } from '../../components/data/StatCard';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { analyticsAdminApi, type GroupedStatRow, type GroupedStatsResponse } from '../../api/admin';

type GroupBy = 'day' | 'operator' | 'country';

const GROUP_TABS: { value: GroupBy; label: string }[] = [
  { value: 'day', label: 'По дням' },
  { value: 'operator', label: 'По операторам' },
  { value: 'country', label: 'По странам' },
];

const filters: FilterDef[] = [
  {
    key: 'period',
    label: 'Период',
    type: 'select',
    options: [
      { value: '7d', label: 'Последние 7 дней' },
      { value: '30d', label: 'Последние 30 дней' },
      { value: '90d', label: 'Последние 90 дней' },
    ],
  },
  { key: 'client_id', label: 'ID клиента', type: 'text', placeholder: 'Все клиенты' },
];

function periodToRange(period: string): { from: string; to: string } {
  const to = new Date();
  const from = new Date();
  from.setDate(from.getDate() - (period === '90d' ? 90 : period === '30d' ? 30 : 7));
  return { from: from.toISOString().slice(0, 10), to: to.toISOString().slice(0, 10) };
}

function fmtNum(n: number) {
  return n.toLocaleString('ru-RU');
}

function fmtRate(n: number) {
  return `${n.toFixed(1)}%`;
}

function fmtCost(n: number) {
  return `${n.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ₽`;
}

interface StatsResponse {
  summary?: {
    total_sent?: number;
    total_delivered?: number;
    total_failed?: number;
    delivery_rate?: number;
    total_cost?: number;
  };
  timeline?: { date: string; sent: number; delivered: number; failed: number }[];
  by_provider?: { provider: string; sent: number; delivered: number; success_rate: number }[];
}

export function AnalyticsPage() {
  const toast = useToast();
  const [filterValues, setFilterValues] = useState<Record<string, string>>({ period: '7d' });
  const [groupBy, setGroupBy] = useState<GroupBy>('day');
  const [stats, setStats] = useState<StatsResponse | null>(null);
  const [grouped, setGrouped] = useState<GroupedStatsResponse | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchStats = useCallback(async () => {
    setLoading(true);
    try {
      const { from, to } = periodToRange(filterValues.period || '7d');
      const clientId = filterValues.client_id || undefined;

      // Fetch summary + chart data
      const res = await analyticsAdminApi.getStats({ from, to, client_id: clientId, group_by: 'day' });
      setStats(res as StatsResponse);

      // Fetch grouped table data
      const groupedRes = await analyticsAdminApi.getGroupedStats({ from, to, group_by: groupBy, client_id: clientId });
      setGrouped(groupedRes);
    } catch {
      toast.error('Ошибка загрузки аналитики');
    } finally {
      setLoading(false);
    }
  }, [filterValues, groupBy, toast]);

  useEffect(() => { fetchStats(); }, [fetchStats]);

  // Re-fetch grouped data when tab changes (summary stays)
  const handleGroupChange = useCallback(async (g: GroupBy) => {
    setGroupBy(g);
    const { from, to } = periodToRange(filterValues.period || '7d');
    const clientId = filterValues.client_id || undefined;
    try {
      const res = await analyticsAdminApi.getGroupedStats({ from, to, group_by: g, client_id: clientId });
      setGrouped(res);
    } catch {
      toast.error('Ошибка загрузки данных');
    }
  }, [filterValues, toast]);

  const summary = stats?.summary;
  const rows = grouped?.rows ?? [];
  const totals = grouped?.totals;

  const labelHeader = groupBy === 'day' ? 'Дата' : groupBy === 'operator' ? 'Оператор' : 'Страна';

  return (
    <>
      <PageHeader
        title="Статистика"
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Статистика' }]}
        actions={<Button variant="secondary" onClick={fetchStats}>Обновить</Button>}
      />

      <FilterBar filters={filters} values={filterValues} onChange={setFilterValues} onReset={() => setFilterValues({ period: '7d' })} />

      {/* Stat cards */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
        <StatCard title="Всего отправлено" value={summary?.total_sent?.toLocaleString() ?? '-'} />
        <StatCard title="Доставлено" value={summary?.total_delivered?.toLocaleString() ?? '-'} />
        <StatCard title="Ошибки" value={summary?.total_failed?.toLocaleString() ?? '-'} />
        <StatCard title="Доставляемость" value={summary?.delivery_rate != null ? `${summary.delivery_rate}%` : '-'} />
        <StatCard title="Общая стоимость" value={summary?.total_cost != null ? `${summary.total_cost} ₽` : '-'} />
      </div>

      {/* Grouped table */}
      <div className="bg-white border border-gray-200 rounded-lg mb-6">
        {/* Tab bar */}
        <div className="flex items-center gap-1 px-4 pt-4 pb-0 border-b border-gray-200">
          {GROUP_TABS.map((tab) => (
            <button
              key={tab.value}
              onClick={() => handleGroupChange(tab.value)}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                groupBy === tab.value
                  ? 'border-primary text-primary'
                  : 'border-transparent text-gray-500 hover:text-gray-700'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Fixed-height scrollable table */}
        <div className="relative">
          <div className="h-[520px] overflow-y-auto">
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-gray-50 z-10">
                <tr className="border-b border-gray-200">
                  <th className="text-left px-4 py-2 font-medium text-gray-600">{labelHeader}</th>
                  <th className="text-right px-4 py-2 font-medium text-gray-600">Отправлено</th>
                  <th className="text-right px-4 py-2 font-medium text-gray-600">Доставлено</th>
                  <th className="text-right px-4 py-2 font-medium text-gray-600">Ошибки</th>
                  <th className="text-right px-4 py-2 font-medium text-gray-600">Доставляемость</th>
                  <th className="text-right px-4 py-2 font-medium text-gray-600">Стоимость</th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td colSpan={6} className="text-center py-12 text-gray-400">Загрузка...</td>
                  </tr>
                )}
                {!loading && rows.length === 0 && (
                  <tr>
                    <td colSpan={6} className="text-center py-12 text-gray-400">Нет данных за выбранный период</td>
                  </tr>
                )}
                {!loading && rows.map((row: GroupedStatRow, i: number) => (
                  <tr key={i} className="border-b border-gray-100 hover:bg-gray-50">
                    <td className="px-4 py-2 text-gray-900">{row.label}</td>
                    <td className="px-4 py-2 text-right text-gray-700">{fmtNum(row.sent)}</td>
                    <td className="px-4 py-2 text-right text-green-700">{fmtNum(row.delivered)}</td>
                    <td className="px-4 py-2 text-right text-red-600">{fmtNum(row.failed)}</td>
                    <td className="px-4 py-2 text-right text-gray-700">{fmtRate(row.delivery_rate)}</td>
                    <td className="px-4 py-2 text-right text-gray-700">{fmtCost(row.cost)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Sticky summary row at bottom */}
          {!loading && totals && rows.length > 0 && (
            <div className="border-t-2 border-gray-300 bg-gray-50">
              <table className="w-full text-sm">
                <tbody>
                  <tr className="font-semibold text-gray-900">
                    <td className="px-4 py-2 w-[30%]">Итого</td>
                    <td className="px-4 py-2 text-right">{fmtNum(totals.sent)}</td>
                    <td className="px-4 py-2 text-right text-green-700">{fmtNum(totals.delivered)}</td>
                    <td className="px-4 py-2 text-right text-red-600">{fmtNum(totals.failed)}</td>
                    <td className="px-4 py-2 text-right">{fmtRate(totals.delivery_rate)}</td>
                    <td className="px-4 py-2 text-right">{fmtCost(totals.cost)}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      {/* Supplementary charts */}
      {!loading && stats?.timeline && stats.timeline.length > 0 && (
        <div className="bg-white border border-gray-200 rounded-lg p-4 mb-6">
          <h3 className="text-sm font-medium text-gray-600 mb-3">Объём сообщений</h3>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={stats.timeline}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip />
              <Line type="monotone" dataKey="delivered" stroke="#4caf50" strokeWidth={2} name="Доставлено" />
              <Line type="monotone" dataKey="failed" stroke="#d32f2f" strokeWidth={2} name="Ошибки" />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
      {!loading && stats?.by_provider && stats.by_provider.length > 0 && (
        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <h3 className="text-sm font-medium text-gray-600 mb-3">По провайдерам</h3>
          <ResponsiveContainer width="100%" height={250}>
            <BarChart data={stats.by_provider}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="provider" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} />
              <Tooltip />
              <Bar dataKey="delivered" fill="#4caf50" name="Доставлено" />
              <Bar dataKey="sent" fill="#1976d2" name="Отправлено" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </>
  );
}
```

- [ ] **Step 3: Verify page still renders at /admin/analytics**

Run: `cd portal-frontend && npx tsc --noEmit`
Expected: No type errors related to AnalyticsPage or analyticsAdminApi.

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/admin/AnalyticsPage.tsx portal-frontend/src/api/admin.ts
git commit -m "feat(admin): rework Statistics page with grouping tabs and fixed-height table"
```

---

## Task 2: Sender Names — Add API get + operator registrations

**Files:**
- Modify: `portal-frontend/src/api/admin.ts`

- [ ] **Step 1: Extend AdminSenderNameInfo type and API**

In `portal-frontend/src/api/admin.ts`, find the `AdminSenderNameInfo` interface (around line 606) and add a `get` method and operator registration types to `adminSenderNamesApi`. 

First add new types before the `adminSenderNamesApi` definition:

```typescript
export interface SenderNameOperatorRegistration {
  operator_id: string;
  operator_name: string;
  mcc: string;
  mnc: string;
  status: 'not_registered' | 'pending' | 'registered' | 'rejected';
  registered_at?: string;
}
```

Then extend `adminSenderNamesApi` — add two methods (`get` and `operatorRegistrations`) after the existing `deactivate` method:

```typescript
export const adminSenderNamesApi = {
  list: (params?: { client_id?: string; status?: string; name_query?: string; limit?: number; offset?: number }) =>
    adminFetch<{ sender_names: AdminSenderNameInfo[]; total: number; limit: number; offset: number }>(
      `/sender-names${qs(params || {})}`,
    ),
  get: (id: string) =>
    adminFetch<{ sender_name: AdminSenderNameInfo }>(`/sender-names/${id}`),
  approve: (id: string) =>
    adminFetch<AdminSenderNameInfo>(`/sender-names/${id}/approve`, { method: 'POST' }),
  reject: (id: string, reason: string) =>
    adminFetch<AdminSenderNameInfo>(`/sender-names/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),
  deactivate: (id: string, reason: string) =>
    adminFetch<AdminSenderNameInfo>(`/sender-names/${id}/deactivate`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),
  operatorRegistrations: (id: string) =>
    adminFetch<{ registrations: SenderNameOperatorRegistration[] }>(`/sender-names/${id}/operator-registrations`),
};
```

- [ ] **Step 2: Verify types compile**

Run: `cd portal-frontend && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 3: Commit API changes**

```bash
git add portal-frontend/src/api/admin.ts
git commit -m "feat(admin): add sender name get/operator-registrations API methods"
```

---

## Task 3: Sender Names — Detail Page

**Files:**
- Create: `portal-frontend/src/pages/admin/sender-names/SenderNameDetailPage.tsx`
- Modify: `portal-frontend/src/pages/admin/SenderNamesAdminPage.tsx`
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Create detail page**

Create `portal-frontend/src/pages/admin/sender-names/SenderNameDetailPage.tsx`:

```typescript
import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { PageHeader } from '../../../components/layout/PageHeader';
import { Badge } from '../../../components/ui/Badge';
import { Button } from '../../../components/ui/Button';
import { Modal } from '../../../components/ui/Modal';
import { Input } from '../../../components/ui/Input';
import { useToast } from '../../../components/ui/Toast';
import {
  adminSenderNamesApi,
  type AdminSenderNameInfo,
  type SenderNameOperatorRegistration,
  AdminApiError,
} from '../../../api/admin';

const STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  pending: { variant: 'warning', label: 'На модерации' },
  approved: { variant: 'success', label: 'Одобрено' },
  rejected: { variant: 'danger', label: 'Отклонено' },
  deactivated: { variant: 'default', label: 'Деактивировано' },
};

const REG_STATUS_BADGE: Record<string, { variant: 'warning' | 'success' | 'danger' | 'default'; label: string }> = {
  not_registered: { variant: 'default', label: 'Не зарегистрировано' },
  pending: { variant: 'warning', label: 'Ожидает' },
  registered: { variant: 'success', label: 'Зарегистрировано' },
  rejected: { variant: 'danger', label: 'Отклонено' },
};

function formatDate(dt: string) {
  return new Date(dt).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

export function SenderNameDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const toast = useToast();

  const [senderName, setSenderName] = useState<AdminSenderNameInfo | null>(null);
  const [registrations, setRegistrations] = useState<SenderNameOperatorRegistration[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Modals
  const [showReject, setShowReject] = useState(false);
  const [rejectReason, setRejectReason] = useState('');
  const [showDeactivate, setShowDeactivate] = useState(false);
  const [deactivateReason, setDeactivateReason] = useState('');
  const [showDelete, setShowDelete] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const [snRes, regRes] = await Promise.all([
        adminSenderNamesApi.get(id),
        adminSenderNamesApi.operatorRegistrations(id).catch(() => ({ registrations: [] })),
      ]);
      setSenderName(snRes.sender_name);
      setRegistrations(regRes.registrations);
    } catch (e) {
      setError(e instanceof AdminApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const handleApprove = async () => {
    if (!senderName) return;
    setSubmitting(true);
    try {
      await adminSenderNamesApi.approve(senderName.id);
      toast.success('Имя одобрено');
      load();
    } catch (e) {
      toast.error(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false); }
  };

  const handleReject = async (e: FormEvent) => {
    e.preventDefault();
    if (!senderName || !rejectReason.trim()) return;
    setSubmitting(true);
    try {
      await adminSenderNamesApi.reject(senderName.id, rejectReason.trim());
      toast.success('Имя отклонено');
      setShowReject(false);
      setRejectReason('');
      load();
    } catch (e) {
      toast.error(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeactivate = async (e: FormEvent) => {
    e.preventDefault();
    if (!senderName) return;
    setSubmitting(true);
    try {
      await adminSenderNamesApi.deactivate(senderName.id, deactivateReason.trim());
      toast.success('Имя деактивировано');
      setShowDeactivate(false);
      setDeactivateReason('');
      load();
    } catch (e) {
      toast.error(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async () => {
    // Деактивация используется как удаление (soft delete)
    if (!senderName) return;
    setSubmitting(true);
    try {
      await adminSenderNamesApi.deactivate(senderName.id, 'Удалено администратором');
      toast.success('Имя удалено');
      navigate('/admin/sender-names');
    } catch (e) {
      toast.error(e instanceof AdminApiError ? e.message : 'Ошибка');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="text-center py-12 text-gray-400">Загрузка...</div>
    );
  }

  if (error || !senderName) {
    return (
      <div className="p-6">
        <div className="p-3 bg-red-50 text-red-700 rounded-md text-sm mb-4">{error || 'Не найдено'}</div>
        <Button variant="secondary" onClick={() => navigate('/admin/sender-names')}>← Назад</Button>
      </div>
    );
  }

  const statusInfo = STATUS_BADGE[senderName.status] ?? { variant: 'default' as const, label: senderName.status };

  return (
    <>
      <PageHeader
        title={`Имя отправителя: ${senderName.name}`}
        breadcrumbs={[
          { label: 'Админ', href: '/admin/dashboard' },
          { label: 'Имена отправителей', href: '/admin/sender-names' },
          { label: senderName.name },
        ]}
        actions={
          <div className="flex gap-2">
            {senderName.status === 'pending' && (
              <>
                <Button variant="secondary" onClick={handleApprove} disabled={submitting}>Одобрить</Button>
                <Button variant="ghost" onClick={() => { setRejectReason(''); setShowReject(true); }}>Отклонить</Button>
              </>
            )}
            {senderName.status === 'approved' && (
              <Button variant="ghost" onClick={() => { setDeactivateReason(''); setShowDeactivate(true); }}>Деактивировать</Button>
            )}
            <Button variant="ghost" onClick={() => setShowDelete(true)}>Удалить</Button>
          </div>
        }
      />

      {/* General info */}
      <div className="bg-white border border-gray-200 rounded-lg p-6 mb-6">
        <h2 className="text-base font-semibold text-gray-900 mb-4">Общая информация</h2>
        <dl className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div>
            <dt className="text-xs text-gray-500 mb-1">Имя</dt>
            <dd className="font-mono font-medium text-gray-900">{senderName.name}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 mb-1">Клиент</dt>
            <dd className="text-sm text-gray-700">{senderName.client_id}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 mb-1">Статус</dt>
            <dd><Badge variant={statusInfo.variant}>{statusInfo.label}</Badge></dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 mb-1">Создано</dt>
            <dd className="text-sm text-gray-700">{formatDate(senderName.created_at)}</dd>
          </div>
          {senderName.rejection_reason && (
            <div className="col-span-2 md:col-span-4">
              <dt className="text-xs text-gray-500 mb-1">Причина отклонения</dt>
              <dd className="text-sm text-red-700">{senderName.rejection_reason}</dd>
            </div>
          )}
        </dl>
      </div>

      {/* Operator registration grid */}
      <div className="bg-white border border-gray-200 rounded-lg p-6">
        <h2 className="text-base font-semibold text-gray-900 mb-4">Регистрация у операторов</h2>
        {registrations.length === 0 ? (
          <p className="text-sm text-gray-400">Данные о регистрации у операторов отсутствуют</p>
        ) : (
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
            {registrations.map((reg) => {
              const regStatus = REG_STATUS_BADGE[reg.status] ?? { variant: 'default' as const, label: reg.status };
              return (
                <div key={reg.operator_id} className="border border-gray-200 rounded-lg p-3">
                  <div className="text-sm font-medium text-gray-900 mb-1 truncate">{reg.operator_name}</div>
                  <div className="text-xs text-gray-400 mb-2">MCC {reg.mcc} / MNC {reg.mnc}</div>
                  <Badge variant={regStatus.variant}>{regStatus.label}</Badge>
                  {reg.registered_at && (
                    <div className="text-xs text-gray-400 mt-1">{formatDate(reg.registered_at)}</div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Reject modal */}
      <Modal open={showReject} onClose={() => setShowReject(false)} title="Отклонить имя отправителя">
        <form onSubmit={handleReject} className="space-y-4">
          <p className="text-sm text-gray-600">Имя: <strong className="font-mono">{senderName.name}</strong></p>
          <Input
            label="Причина отклонения"
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
            placeholder="Укажите причину..."
            required
          />
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setShowReject(false)}>Отмена</Button>
            <Button type="submit" disabled={submitting || !rejectReason.trim()}>Отклонить</Button>
          </div>
        </form>
      </Modal>

      {/* Deactivate modal */}
      <Modal open={showDeactivate} onClose={() => setShowDeactivate(false)} title="Деактивировать имя отправителя">
        <form onSubmit={handleDeactivate} className="space-y-4">
          <p className="text-sm text-gray-600">Имя: <strong className="font-mono">{senderName.name}</strong></p>
          <Input
            label="Причина (опционально)"
            value={deactivateReason}
            onChange={(e) => setDeactivateReason(e.target.value)}
            placeholder="Укажите причину..."
          />
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setShowDeactivate(false)}>Отмена</Button>
            <Button type="submit" disabled={submitting}>Деактивировать</Button>
          </div>
        </form>
      </Modal>

      {/* Delete confirmation */}
      <Modal open={showDelete} onClose={() => setShowDelete(false)} title="Удалить имя отправителя">
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Вы уверены, что хотите удалить имя <strong className="font-mono">{senderName.name}</strong>?
            Это действие необратимо.
          </p>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setShowDelete(false)}>Отмена</Button>
            <Button variant="secondary" onClick={handleDelete} disabled={submitting}>Удалить</Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
```

- [ ] **Step 2: Add click-to-detail navigation in SenderNamesAdminPage**

In `portal-frontend/src/pages/admin/SenderNamesAdminPage.tsx`:

1. Add `useNavigate` import from `react-router-dom`:
```typescript
import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { adminSenderNamesApi, AdminApiError, type AdminSenderNameInfo } from '../../api/admin';
// ...rest of imports unchanged
```

2. Inside the `SenderNamesAdminPage` function, add after the state declarations:
```typescript
const navigate = useNavigate();
```

3. Add `onRowClick` to the `DataTable`:
```typescript
<DataTable
  columns={columns}
  data={items}
  loading={loading}
  total={total}
  page={page}
  pageSize={PAGE_SIZE}
  onPageChange={(p) => setOffset((p - 1) * PAGE_SIZE)}
  onRowClick={(sn) => navigate(`/admin/sender-names/${sn.id}`)}
/>
```

- [ ] **Step 3: Register route in App.tsx**

In `portal-frontend/src/App.tsx`:

1. Add lazy import after `AdminSenderNamesPage`:
```typescript
const AdminSenderNameDetailPage = lazy(() => import('./pages/admin/sender-names/SenderNameDetailPage').then((m) => ({ default: m.SenderNameDetailPage })));
```

2. Add route after the `sender-names` route:
```typescript
<Route path="sender-names/:id" element={<Suspense fallback={null}><AdminSenderNameDetailPage /></Suspense>} />
```

- [ ] **Step 4: Verify types compile**

Run: `cd portal-frontend && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/admin/sender-names/SenderNameDetailPage.tsx \
        portal-frontend/src/pages/admin/SenderNamesAdminPage.tsx \
        portal-frontend/src/App.tsx
git commit -m "feat(admin): add sender name detail page with operator registration grid"
```

---

## Task 4: MCC/MNC — Flat Table View + Operator Detail Panel

**Files:**
- Modify: `portal-frontend/src/pages/admin/CountriesPage.tsx`

### What to build

Add a view toggle at the top of the page:
- **"По странам"** — existing 3-column hierarchical view (keep as-is)
- **"MCC/MNC таблица"** — flat table: MCC | MNC | Operator | Country | Status

The operator detail "panel" (third column) is enhanced with:
- Settings section: paid/free sender toggles (editable inline)
- Prefixes section (existing)

Also add 2-step operator creation wizard (Step 1: basic info, Step 2: settings) replacing the single-step modal.

- [ ] **Step 1: Replace CountriesPage.tsx**

Replace the entire content of `portal-frontend/src/pages/admin/CountriesPage.tsx`:

```typescript
import { useState, useEffect, useCallback } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';
import { Badge, StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import { countriesApi, operatorsApi, type CountryInfo, type OperatorInfo, type OperatorPrefix } from '../../api/admin';

const PAGE_SIZE = 20;

type ViewMode = 'hierarchical' | 'flat';

const WIZARD_STEPS = ['Основная информация', 'Настройки'];

const emptyOperatorStep1 = { name: '', mcc: '', mnc: '' };
const emptyOperatorStep2 = { supports_paid_sender: false, supports_free_sender: true, monthly_tariff_amount: '' };

export function CountriesPage() {
  const toast = useToast();

  // View mode
  const [viewMode, setViewMode] = useState<ViewMode>('hierarchical');

  // Countries
  const [countries, setCountries] = useState<CountryInfo[]>([]);
  const [countryTotal, setCountryTotal] = useState(0);
  const [countryPage, setCountryPage] = useState(1);
  const [loading, setLoading] = useState(true);

  // Country create modal
  const [showCreateCountry, setShowCreateCountry] = useState(false);
  const [countryForm, setCountryForm] = useState({ name: '', code: '', phone_code: '' });

  // Selected state
  const [selectedCountry, setSelectedCountry] = useState<CountryInfo | null>(null);
  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [operatorsLoading, setOperatorsLoading] = useState(false);
  const [selectedOperator, setSelectedOperator] = useState<OperatorInfo | null>(null);
  const [prefixes, setPrefixes] = useState<OperatorPrefix[]>([]);
  const [prefixInput, setPrefixInput] = useState('');
  const [saving, setSaving] = useState(false);

  // Flat view: all operators
  const [allOperators, setAllOperators] = useState<(OperatorInfo & { country_name?: string })[]>([]);
  const [allOperatorsLoading, setAllOperatorsLoading] = useState(false);
  const [allOperatorsPage, setAllOperatorsPage] = useState(1);
  const [allOperatorsTotal, setAllOperatorsTotal] = useState(0);
  const [mccFilter, setMccFilter] = useState('');
  const [mncFilter, setMncFilter] = useState('');

  // Operator wizard (2-step)
  const [showWizard, setShowWizard] = useState(false);
  const [wizardStep, setWizardStep] = useState(0);
  const [wizardStep1, setWizardStep1] = useState(emptyOperatorStep1);
  const [wizardStep2, setWizardStep2] = useState(emptyOperatorStep2);

  // Country filter for flat view
  const [countryFilterForFlat, setCountryFilterForFlat] = useState('');

  const fetchCountries = useCallback(async () => {
    setLoading(true);
    try {
      const res = await countriesApi.list({ limit: PAGE_SIZE, offset: (countryPage - 1) * PAGE_SIZE });
      setCountries(res.countries || []);
      setCountryTotal(res.total);
    } catch {
      toast.error('Ошибка загрузки стран');
    } finally {
      setLoading(false);
    }
  }, [countryPage, toast]);

  useEffect(() => { fetchCountries(); }, [fetchCountries]);

  const fetchAllOperators = useCallback(async () => {
    setAllOperatorsLoading(true);
    try {
      const res = await operatorsApi.list({ limit: PAGE_SIZE, offset: (allOperatorsPage - 1) * PAGE_SIZE });
      // Enrich with country name from local countries list if available
      const enriched = (res.operators || []).map((op) => ({
        ...op,
        country_name: countries.find((c) => c.country_id === op.country_id)?.name ?? op.country_id,
      }));
      setAllOperators(enriched);
      setAllOperatorsTotal(res.total);
    } catch {
      toast.error('Ошибка загрузки операторов');
    } finally {
      setAllOperatorsLoading(false);
    }
  }, [allOperatorsPage, countries, toast]);

  useEffect(() => {
    if (viewMode === 'flat') fetchAllOperators();
  }, [viewMode, fetchAllOperators]);

  const fetchOperators = useCallback(async (countryId: string) => {
    setOperatorsLoading(true);
    try {
      const res = await operatorsApi.list({ country_id: countryId, limit: 100, offset: 0 });
      setOperators(res.operators || []);
    } catch {
      toast.error('Ошибка загрузки операторов');
    } finally {
      setOperatorsLoading(false);
    }
  }, [toast]);

  const fetchPrefixes = useCallback(async (operatorId: string) => {
    try {
      const res = await operatorsApi.listPrefixes(operatorId);
      setPrefixes(res.prefixes || []);
    } catch {
      toast.error('Ошибка загрузки префиксов');
    }
  }, [toast]);

  const selectCountry = (country: CountryInfo) => {
    setSelectedCountry(country);
    setSelectedOperator(null);
    setPrefixes([]);
    fetchOperators(country.country_id);
  };

  const selectOperator = (op: OperatorInfo) => {
    setSelectedOperator(op);
    fetchPrefixes(op.operator_id);
  };

  const handleCreateCountry = async () => {
    setSaving(true);
    try {
      await countriesApi.create(countryForm);
      toast.success('Страна создана');
      setShowCreateCountry(false);
      setCountryForm({ name: '', code: '', phone_code: '' });
      fetchCountries();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setSaving(false);
    }
  };

  const handleWizardFinish = async () => {
    if (!selectedCountry) return;
    setSaving(true);
    try {
      const data: Partial<OperatorInfo> = {
        ...wizardStep1,
        ...wizardStep2,
        country_id: selectedCountry.country_id,
        active: true,
      };
      if (!wizardStep2.supports_paid_sender) delete data.monthly_tariff_amount;
      await operatorsApi.create(data);
      toast.success('Оператор создан');
      setShowWizard(false);
      setWizardStep(0);
      setWizardStep1(emptyOperatorStep1);
      setWizardStep2(emptyOperatorStep2);
      fetchOperators(selectedCountry.country_id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setSaving(false);
    }
  };

  const handleUpdateOperatorSettings = async () => {
    if (!selectedOperator) return;
    setSaving(true);
    try {
      await operatorsApi.update(selectedOperator.operator_id, {
        supports_paid_sender: selectedOperator.supports_paid_sender,
        supports_free_sender: selectedOperator.supports_free_sender,
        monthly_tariff_amount: selectedOperator.monthly_tariff_amount,
      });
      toast.success('Настройки сохранены');
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setSaving(false);
    }
  };

  const handleAddPrefix = async () => {
    if (!selectedOperator || !prefixInput) return;
    setSaving(true);
    try {
      await operatorsApi.createPrefix(selectedOperator.operator_id, { prefix: prefixInput });
      toast.success('Префикс добавлен');
      setPrefixInput('');
      fetchPrefixes(selectedOperator.operator_id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setSaving(false);
    }
  };

  const handleDeletePrefix = async (prefixId: string) => {
    if (!selectedOperator) return;
    try {
      await operatorsApi.deletePrefix(selectedOperator.operator_id, prefixId);
      toast.success('Префикс удалён');
      fetchPrefixes(selectedOperator.operator_id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    }
  };

  // Filtered flat list (client-side filter by mcc/mnc)
  const filteredOperators = allOperators.filter((op) => {
    if (mccFilter && !op.mcc.includes(mccFilter)) return false;
    if (mncFilter && !op.mnc.includes(mncFilter)) return false;
    if (countryFilterForFlat && !op.country_name?.toLowerCase().includes(countryFilterForFlat.toLowerCase())) return false;
    return true;
  });

  const countryColumns: Column<CountryInfo>[] = [
    { key: 'name', header: 'Страна', sortable: true },
    { key: 'code', header: 'Код' },
    { key: 'phone_code', header: 'Тел. код' },
  ];

  const operatorColumns: Column<OperatorInfo>[] = [
    { key: 'name', header: 'Оператор' },
    { key: 'mcc', header: 'MCC' },
    { key: 'mnc', header: 'MNC' },
    { key: 'active', header: 'Статус', render: (o) => <StatusBadge status={o.active ? 'active' : 'inactive'} /> },
  ];

  const flatColumns: Column<OperatorInfo & { country_name?: string }>[] = [
    { key: 'mcc', header: 'MCC' },
    { key: 'mnc', header: 'MNC' },
    { key: 'name', header: 'Оператор' },
    { key: 'country_name' as keyof OperatorInfo, header: 'Страна', render: (o) => <span>{(o as OperatorInfo & { country_name?: string }).country_name}</span> },
    { key: 'supports_paid_sender', header: 'Платные', render: (o) => o.supports_paid_sender ? <Badge variant="success">Да</Badge> : <Badge variant="default">Нет</Badge> },
    { key: 'active', header: 'Статус', render: (o) => <StatusBadge status={o.active ? 'active' : 'inactive'} /> },
  ];

  return (
    <>
      <PageHeader
        title="Справочник MCC/MNC"
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'MCC/MNC' }]}
      />

      {/* View toggle */}
      <div className="flex gap-1 mb-6 border-b border-gray-200">
        <button
          onClick={() => setViewMode('hierarchical')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            viewMode === 'hierarchical' ? 'border-primary text-primary' : 'border-transparent text-gray-500 hover:text-gray-700'
          }`}
        >
          По странам
        </button>
        <button
          onClick={() => setViewMode('flat')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            viewMode === 'flat' ? 'border-primary text-primary' : 'border-transparent text-gray-500 hover:text-gray-700'
          }`}
        >
          MCC/MNC таблица
        </button>
      </div>

      {/* Flat MCC/MNC table view */}
      {viewMode === 'flat' && (
        <>
          <div className="flex flex-wrap gap-3 mb-4">
            <Input
              label="MCC"
              value={mccFilter}
              onChange={(e) => setMccFilter(e.target.value)}
              placeholder="Фильтр MCC"
            />
            <Input
              label="MNC"
              value={mncFilter}
              onChange={(e) => setMncFilter(e.target.value)}
              placeholder="Фильтр MNC"
            />
            <Input
              label="Страна"
              value={countryFilterForFlat}
              onChange={(e) => setCountryFilterForFlat(e.target.value)}
              placeholder="Поиск по стране"
            />
          </div>
          <DataTable
            columns={flatColumns as Column<OperatorInfo>[]}
            data={filteredOperators as OperatorInfo[]}
            total={allOperatorsTotal}
            page={allOperatorsPage}
            pageSize={PAGE_SIZE}
            onPageChange={setAllOperatorsPage}
            loading={allOperatorsLoading}
            keyField="operator_id"
          />
        </>
      )}

      {/* Hierarchical 3-column view */}
      {viewMode === 'hierarchical' && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Column 1: Countries */}
          <div>
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-lg font-semibold">Страны</h2>
              <Button size="sm" onClick={() => { setCountryForm({ name: '', code: '', phone_code: '' }); setShowCreateCountry(true); }}>
                + Добавить
              </Button>
            </div>
            <DataTable
              columns={countryColumns}
              data={countries}
              total={countryTotal}
              page={countryPage}
              pageSize={PAGE_SIZE}
              onPageChange={setCountryPage}
              loading={loading}
              keyField="country_id"
              onRowClick={selectCountry}
            />
          </div>

          {/* Column 2: Operators */}
          <div>
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-lg font-semibold">
                {selectedCountry ? `Операторы — ${selectedCountry.name}` : 'Операторы'}
              </h2>
              {selectedCountry && (
                <Button
                  size="sm"
                  onClick={() => { setWizardStep(0); setWizardStep1(emptyOperatorStep1); setWizardStep2(emptyOperatorStep2); setShowWizard(true); }}
                >
                  + Добавить
                </Button>
              )}
            </div>
            {selectedCountry ? (
              <DataTable
                columns={operatorColumns}
                data={operators}
                total={operators.length}
                page={1}
                pageSize={100}
                onPageChange={() => {}}
                loading={operatorsLoading}
                keyField="operator_id"
                onRowClick={selectOperator}
              />
            ) : (
              <div className="text-sm text-gray-400 p-4 border border-gray-200 rounded-lg">
                Выберите страну
              </div>
            )}
          </div>

          {/* Column 3: Operator detail + prefixes */}
          <div>
            <h2 className="text-lg font-semibold mb-3">
              {selectedOperator ? selectedOperator.name : 'Детали'}
            </h2>
            {selectedOperator ? (
              <>
                {/* Settings section */}
                <div className="bg-white border border-gray-200 rounded-lg p-4 mb-4">
                  <h3 className="text-sm font-medium text-gray-700 mb-3">Настройки</h3>
                  <div className="space-y-2">
                    <label className="flex items-center gap-2 text-sm cursor-pointer">
                      <input
                        type="checkbox"
                        checked={selectedOperator.supports_free_sender ?? true}
                        onChange={(e) => setSelectedOperator({ ...selectedOperator, supports_free_sender: e.target.checked })}
                        className="rounded"
                      />
                      Бесплатная регистрация имени
                    </label>
                    <label className="flex items-center gap-2 text-sm cursor-pointer">
                      <input
                        type="checkbox"
                        checked={selectedOperator.supports_paid_sender ?? false}
                        onChange={(e) => setSelectedOperator({ ...selectedOperator, supports_paid_sender: e.target.checked, monthly_tariff_amount: e.target.checked ? selectedOperator.monthly_tariff_amount : '' })}
                        className="rounded"
                      />
                      Платная регистрация имени
                    </label>
                    {selectedOperator.supports_paid_sender && (
                      <Input
                        label="Ежемесячный тариф (RUB)"
                        type="number"
                        min="0"
                        step="0.01"
                        value={selectedOperator.monthly_tariff_amount ?? ''}
                        onChange={(e) => setSelectedOperator({ ...selectedOperator, monthly_tariff_amount: e.target.value })}
                        placeholder="1500.00"
                      />
                    )}
                  </div>
                  <div className="mt-3">
                    <Button size="sm" onClick={handleUpdateOperatorSettings} disabled={saving}>
                      Сохранить
                    </Button>
                  </div>
                </div>

                {/* Prefixes section */}
                <div className="bg-white border border-gray-200 rounded-lg p-4">
                  <h3 className="text-sm font-medium text-gray-700 mb-3">Префиксы</h3>
                  <div className="flex gap-2 mb-3">
                    <Input
                      value={prefixInput}
                      onChange={(e) => setPrefixInput(e.target.value)}
                      placeholder="+7921"
                    />
                    <Button size="sm" onClick={handleAddPrefix} disabled={saving || !prefixInput}>
                      Добавить
                    </Button>
                  </div>
                  <ul className="space-y-1">
                    {prefixes.map((p) => (
                      <li key={p.prefix_id} className="flex items-center justify-between py-1 px-2 bg-gray-50 rounded text-sm">
                        <span className="font-mono">{p.prefix}</span>
                        <button
                          className="text-xs text-danger hover:underline"
                          onClick={() => handleDeletePrefix(p.prefix_id)}
                        >
                          Удалить
                        </button>
                      </li>
                    ))}
                    {prefixes.length === 0 && (
                      <li className="text-sm text-gray-400">Нет префиксов</li>
                    )}
                  </ul>
                </div>
              </>
            ) : (
              <div className="text-sm text-gray-400 p-4 border border-gray-200 rounded-lg">
                Выберите оператора
              </div>
            )}
          </div>
        </div>
      )}

      {/* Create Country modal */}
      <Modal open={showCreateCountry} onClose={() => setShowCreateCountry(false)} title="Добавить страну">
        <div className="space-y-4">
          <Input label="Название" value={countryForm.name} onChange={(e) => setCountryForm({ ...countryForm, name: e.target.value })} required />
          <Input label="Код (ISO 2)" value={countryForm.code} onChange={(e) => setCountryForm({ ...countryForm, code: e.target.value })} required placeholder="RU" maxLength={2} />
          <Input label="Телефонный код" value={countryForm.phone_code} onChange={(e) => setCountryForm({ ...countryForm, phone_code: e.target.value })} required placeholder="+7" />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => setShowCreateCountry(false)}>Отмена</Button>
            <Button onClick={handleCreateCountry} disabled={saving}>{saving ? 'Создание...' : 'Создать'}</Button>
          </div>
        </div>
      </Modal>

      {/* Operator 2-step wizard */}
      <Modal open={showWizard} onClose={() => setShowWizard(false)} title={`Добавить оператора — Шаг ${wizardStep + 1}: ${WIZARD_STEPS[wizardStep]}`}>
        {wizardStep === 0 && (
          <div className="space-y-4">
            <Input label="Название" value={wizardStep1.name} onChange={(e) => setWizardStep1({ ...wizardStep1, name: e.target.value })} required />
            <Input label="MCC" value={wizardStep1.mcc} onChange={(e) => setWizardStep1({ ...wizardStep1, mcc: e.target.value })} required placeholder="250" />
            <Input label="MNC" value={wizardStep1.mnc} onChange={(e) => setWizardStep1({ ...wizardStep1, mnc: e.target.value })} required placeholder="01" />
            <div className="flex justify-end gap-3 pt-2">
              <Button variant="secondary" onClick={() => setShowWizard(false)}>Отмена</Button>
              <Button
                onClick={() => setWizardStep(1)}
                disabled={!wizardStep1.name || !wizardStep1.mcc || !wizardStep1.mnc}
              >
                Далее →
              </Button>
            </div>
          </div>
        )}
        {wizardStep === 1 && (
          <div className="space-y-4">
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input
                type="checkbox"
                checked={wizardStep2.supports_free_sender}
                onChange={(e) => setWizardStep2({ ...wizardStep2, supports_free_sender: e.target.checked })}
                className="rounded"
              />
              Поддержка бесплатной регистрации имени
            </label>
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input
                type="checkbox"
                checked={wizardStep2.supports_paid_sender}
                onChange={(e) => setWizardStep2({ ...wizardStep2, supports_paid_sender: e.target.checked, monthly_tariff_amount: e.target.checked ? wizardStep2.monthly_tariff_amount : '' })}
                className="rounded"
              />
              Поддержка платной регистрации имени
            </label>
            {wizardStep2.supports_paid_sender && (
              <Input
                label="Ежемесячный тариф (RUB)"
                type="number"
                min="0"
                step="0.01"
                value={wizardStep2.monthly_tariff_amount}
                onChange={(e) => setWizardStep2({ ...wizardStep2, monthly_tariff_amount: e.target.value })}
                placeholder="1500.00"
              />
            )}
            <div className="flex justify-between pt-2">
              <Button variant="secondary" onClick={() => setWizardStep(0)}>← Назад</Button>
              <div className="flex gap-2">
                <Button variant="secondary" onClick={() => setShowWizard(false)}>Отмена</Button>
                <Button onClick={handleWizardFinish} disabled={saving}>{saving ? 'Создание...' : 'Создать'}</Button>
              </div>
            </div>
          </div>
        )}
      </Modal>
    </>
  );
}
```

- [ ] **Step 2: Verify types compile**

Run: `cd portal-frontend && npx tsc --noEmit`
Expected: No type errors.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/admin/CountriesPage.tsx
git commit -m "feat(admin): enhance MCC/MNC page with flat table view, operator detail, 2-step wizard"
```

---

## Task 5: Sidebar — Add "Детализация" entry (sidebar update per spec)

**Files:**
- Modify: `portal-frontend/src/components/layout/AdminSidebar.tsx`

The spec says sidebar updates happen incrementally with each phase. Phase 2 sidebar change: add "Детализация" under "Отчёты" (it will be a placeholder pointing to `/admin/detalization` — Phase 3 will implement the page itself).

- [ ] **Step 1: Add Детализация to sidebar**

In `portal-frontend/src/components/layout/AdminSidebar.tsx`, find the `'Отчёты'` group (currently has "Статистика", "Мониторинг", "Аудит") and add "Детализация" as the first item:

```typescript
  {
    title: 'Отчёты',
    items: [
      { path: '/admin/detalization', label: 'Детализация', icon: '🔍', resource: 'analytics' },
      { path: '/admin/analytics', label: 'Статистика', icon: '📈', resource: 'analytics' },
      { path: '/admin/monitoring', label: 'Мониторинг', icon: '⚡', resource: 'analytics' },
      { path: '/admin/audit', label: 'Аудит', icon: '📜', resource: 'audit' },
    ],
  },
```

Note: The current sidebar file uses unicode escapes — keep that pattern. Replace the `\u041E\u0442\u0447\u0451\u0442\u044B` group block (it's the "Отчёты" group, currently at lines ~60-66) with updated version that includes the new entry. The exact items in the current file use unicode escapes; write the replacement using actual Unicode characters since the file already has some mixed usage.

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/components/layout/AdminSidebar.tsx
git commit -m "feat(admin): add Детализация entry to sidebar (Phase 2 nav update)"
```

---

## Self-Review Checklist

**Spec coverage:**
- [x] REWORK-5: Statistics — grouping tabs (По дням / По операторам / По странам) ✓
- [x] REWORK-5: Fixed-height container with internal scroll ✓
- [x] REWORK-5: Sticky summary row at bottom ✓
- [x] REWORK-5: Keep stat cards ✓
- [x] REWORK-1: Click-to-detail navigation from list ✓
- [x] REWORK-1: Detail page — "Общая информация" section ✓
- [x] REWORK-1: Detail page — operator registration grid with badges ✓
- [x] REWORK-1: Approve/Reject/Deactivate actions on detail page ✓
- [x] REWORK-1: Delete with redirect to list ✓
- [x] REWORK-3: Flat MCC/MNC table view ✓
- [x] REWORK-3: Country/operator/MCC/MNC filters for flat view ✓
- [x] REWORK-3: Operator detail panel with editable settings ✓
- [x] REWORK-3: 2-step operator creation wizard ✓
- [x] Sidebar navigation update ✓

**Type consistency:**
- `GroupedStatRow` and `GroupedStatsResponse` defined in Task 1 Step 1 and consumed in Task 1 Step 2 ✓
- `SenderNameOperatorRegistration` defined in Task 2 Step 1 and consumed in Task 3 Step 1 ✓
- All API methods (`adminSenderNamesApi.get`, `.operatorRegistrations`) defined before use ✓
- `AdminSenderNameInfo` interface unchanged — detail page reads `.sender_name` from `get()` response ✓
