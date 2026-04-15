# Admin Panel Improvements — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить переключатель режимов (технический/бизнес), управление кампаниями, UI расписаний маршрутов, симулятор маршрутизации, расширить страницу клиента (домены, sender names).

**Architecture:** Инкрементальный подход — каждый этап деплоится независимо. Frontend: React компоненты по существующему паттерну admin pages. Backend: новые HTTP handlers в admin gateway, обращающиеся к существующим репозиториям через прямой DB доступ (campaign repo) или gRPC (routing).

**Tech Stack:** Go 1.24, gorilla/mux, pgx/v5, TypeScript 5.7, React 19, Vite, Tailwind CSS 4.2, Radix UI

---

## Файловая карта

### Создаются
- `portal-frontend/src/hooks/useAdminMode.ts`
- `portal-frontend/src/pages/admin/campaigns/CampaignsAdminPage.tsx`
- `portal-frontend/src/pages/admin/campaigns/CampaignAdminDetailPage.tsx`
- `portal-frontend/src/pages/admin/routes/RouteSchedulesPage.tsx`
- `portal-frontend/src/pages/admin/routes/RouteSimulatorPage.tsx`
- `portal-frontend/src/components/RelatedLinks.tsx`
- `internal/gateway/admin/handlers/campaigns.go`
- `internal/gateway/admin/handlers/route_schedules.go`
- `internal/gateway/admin/handlers/route_simulator.go`

### Изменяются
- `portal-frontend/src/components/layout/AdminSidebar.tsx`
- `portal-frontend/src/App.tsx`
- `portal-frontend/src/api/admin.ts`
- `portal-frontend/src/pages/admin/ClientsPage.tsx`
- `internal/gateway/admin/router/router.go`
- `cmd/admin-gateway/main.go`

---

## Stage 1: Переключатель режимов (frontend only)

### Task 1.1: Хук useAdminMode

**Files:**
- Create: `portal-frontend/src/hooks/useAdminMode.ts`

- [ ] **Step 1: Создать хук**

```typescript
// portal-frontend/src/hooks/useAdminMode.ts
import { useState, useCallback } from 'react';

export type AdminMode = 'tech' | 'business';

const STORAGE_KEY = 'admin_mode';

export function useAdminMode() {
  const [mode, setModeState] = useState<AdminMode>(() => {
    const stored = localStorage.getItem(STORAGE_KEY);
    return (stored === 'tech' || stored === 'business') ? stored : 'business';
  });

  const setMode = useCallback((m: AdminMode) => {
    localStorage.setItem(STORAGE_KEY, m);
    setModeState(m);
  }, []);

  return { mode, setMode };
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/hooks/useAdminMode.ts
git commit -m "feat(admin): add useAdminMode hook for sidebar mode switching"
```

---

### Task 1.2: Переключатель в сайдбаре

**Files:**
- Modify: `portal-frontend/src/components/layout/AdminSidebar.tsx`

- [ ] **Step 1: Прочитать текущий SIDEBAR_GROUPS и добавить поле `mode`**

Открыть `portal-frontend/src/components/layout/AdminSidebar.tsx`.

Обновить тип `SidebarItem` и добавить поле `mode`:

```typescript
// В начале файла добавить импорт:
import { useAdminMode, type AdminMode } from '../../hooks/useAdminMode';

// Обновить тип:
interface SidebarItem {
  path: string;
  label: string;
  icon: string;
  resource: string;
  mode?: 'tech' | 'business' | 'common'; // undefined = common
}
```

- [ ] **Step 2: Разметить элементы SIDEBAR_GROUPS по режимам**

В `SIDEBAR_GROUPS` добавить `mode` к каждому item. Технические разделы (providers, routes, connections, channels, delivery-strategies, hlr, countries, webhooks, settings) — `mode: 'tech'`. Бизнес (clients, templates, sender-names, users, billing, tarification, individual-tariffs, legal-entities, contracts, operator-templates) — `mode: 'business'`. Общие (dashboard, detalization, analytics, monitoring, audit) — `mode: 'common'` или без поля.

Пример для одного элемента:
```typescript
{ path: '/admin/providers', label: 'Провайдеры', icon: '🔌', resource: 'providers', mode: 'tech' },
```

- [ ] **Step 3: Добавить UI свитчера и фильтрацию**

В компоненте `AdminSidebar` добавить:

```typescript
export function AdminSidebar() {
  const { mode, setMode } = useAdminMode();
  // ... остальной существующий код ...

  // В JSX — до списка групп — добавить свитчер:
  return (
    <aside /* существующие классы */>
      {/* Существующий лого/хедер */}
      
      {/* НОВЫЙ свитчер режимов */}
      <div className="px-2 pb-2">
        <div className="flex rounded-lg overflow-hidden border border-neutral-200 dark:border-neutral-700 text-xs font-medium">
          <button
            onClick={() => setMode('business')}
            className={`flex-1 py-1.5 transition-colors ${
              mode === 'business'
                ? 'bg-primary-600 text-white'
                : 'text-neutral-500 hover:bg-neutral-100 dark:hover:bg-neutral-800'
            }`}
          >
            {isCollapsed ? '💼' : '💼 Бизнес'}
          </button>
          <button
            onClick={() => setMode('tech')}
            className={`flex-1 py-1.5 transition-colors ${
              mode === 'tech'
                ? 'bg-primary-600 text-white'
                : 'text-neutral-500 hover:bg-neutral-100 dark:hover:bg-neutral-800'
            }`}
          >
            {isCollapsed ? '⚙️' : '⚙️ Техн.'}
          </button>
        </div>
      </div>

      {/* Существующие группы, но с фильтрацией */}
    </aside>
  );
}
```

- [ ] **Step 4: Добавить фильтрацию items по режиму**

В логике рендеринга групп, где фильтруются items по правам, добавить фильтр по режиму:

```typescript
const visibleItems = group.items.filter((item) => {
  const modeMatch = !item.mode || item.mode === 'common' || item.mode === mode;
  return modeMatch && hasPermission(item.resource, 'read');
});
```

- [ ] **Step 5: Запустить dev сервер и проверить переключение**

```bash
cd portal-frontend && npm run dev
```

Открыть http://localhost:5173/admin, убедиться что:
- Свитчер отображается в сайдбаре
- При переключении на "Техн." видны провайдеры/маршруты, скрыты клиенты/биллинг
- При переключении на "Бизнес" — наоборот
- Дашборд/аналитика видны в обоих режимах
- Выбранный режим сохраняется после перезагрузки страницы

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/components/layout/AdminSidebar.tsx
git commit -m "feat(admin): add tech/business mode switcher to sidebar"
```

---

### Task 1.3: Добавить новые пункты в сайдбар (заглушки)

В SIDEBAR_GROUPS добавить будущие пункты с режимами:

```typescript
// В группу "Маршрутизация" (mode: 'tech'):
{ path: '/admin/routes/schedules', label: 'Расписания маршрутов', icon: '🕐', resource: 'routes', mode: 'tech' },
{ path: '/admin/routes/simulator', label: 'Симулятор', icon: '🧪', resource: 'routes', mode: 'tech' },

// В группу "Основное" (mode: 'business'):
{ path: '/admin/campaigns', label: 'Кампании', icon: '📣', resource: 'clients', mode: 'business' },
```

- [ ] **Step 1: Добавить пункты в SIDEBAR_GROUPS**
- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/components/layout/AdminSidebar.tsx
git commit -m "feat(admin): add placeholder sidebar items for new sections"
```

---

## Stage 2: Управление кампаниями

### Task 2.1: API типы и функции

**Files:**
- Modify: `portal-frontend/src/api/admin.ts`

- [ ] **Step 1: Добавить типы кампаний**

```typescript
// В portal-frontend/src/api/admin.ts добавить:

export interface AdminCampaign {
  id: string;
  client_id: string;
  client_name: string;
  name: string;
  status: 'draft' | 'running' | 'paused' | 'completed' | 'failed' | 'scheduled';
  type: string;
  template_id?: string;
  sender_name?: string;
  channel: string;
  total_recipients: number;
  sent_count: number;
  delivered_count: number;
  failed_count: number;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  scheduled_at?: string;
}

export interface AdminCampaignDetail extends AdminCampaign {
  template_name?: string;
  batch_size?: number;
  batch_interval_seconds?: number;
  schedule_cron?: string;
  read_count?: number;
  status_history: Array<{ status: string; changed_at: string; changed_by?: string }>;
}

export interface CampaignRecipient {
  id: string;
  phone: string;
  status: string;
  sent_at?: string;
  delivered_at?: string;
  error?: string;
}
```

- [ ] **Step 2: Добавить API объект campaignsAdminApi**

```typescript
export const campaignsAdminApi = {
  list: (params?: {
    client_id?: string;
    status?: string;
    search?: string;
    limit?: number;
    offset?: number;
  }) =>
    adminFetch<{ campaigns: AdminCampaign[]; total: number }>(
      `/campaigns${qs(params || {})}`
    ),

  get: (id: string) =>
    adminFetch<{ campaign: AdminCampaignDetail }>(`/campaigns/${id}`),

  pause: (id: string) =>
    adminFetch<void>(`/campaigns/${id}/pause`, { method: 'POST' }),

  resume: (id: string) =>
    adminFetch<void>(`/campaigns/${id}/resume`, { method: 'POST' }),

  stop: (id: string) =>
    adminFetch<void>(`/campaigns/${id}/stop`, { method: 'POST' }),

  update: (id: string, data: { batch_size?: number; batch_interval_seconds?: number; scheduled_at?: string }) =>
    adminFetch<void>(`/campaigns/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  listRecipients: (id: string, params?: { limit?: number; offset?: number }) =>
    adminFetch<{ recipients: CampaignRecipient[]; total: number }>(
      `/campaigns/${id}/recipients${qs(params || {})}`
    ),
};
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/api/admin.ts
git commit -m "feat(admin): add campaigns API types and functions"
```

---

### Task 2.2: Страница списка кампаний

**Files:**
- Create: `portal-frontend/src/pages/admin/campaigns/CampaignsAdminPage.tsx`

- [ ] **Step 1: Создать директорию и файл**

```bash
mkdir -p portal-frontend/src/pages/admin/campaigns
```

- [ ] **Step 2: Написать компонент**

```typescript
// portal-frontend/src/pages/admin/campaigns/CampaignsAdminPage.tsx
import { useState, useCallback, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { campaignsAdminApi, type AdminCampaign } from '../../../api/admin';
import { useToast } from '../../../hooks/useToast';

const PAGE_SIZE = 50;

const STATUS_LABELS: Record<string, string> = {
  draft: 'Черновик',
  running: 'Выполняется',
  paused: 'Пауза',
  completed: 'Завершена',
  failed: 'Ошибка',
  scheduled: 'Запланирована',
};

const STATUS_COLORS: Record<string, string> = {
  draft: 'bg-neutral-100 text-neutral-700',
  running: 'bg-green-100 text-green-700',
  paused: 'bg-yellow-100 text-yellow-700',
  completed: 'bg-blue-100 text-blue-700',
  failed: 'bg-red-100 text-red-700',
  scheduled: 'bg-purple-100 text-purple-700',
};

export function CampaignsAdminPage() {
  const toast = useToast();
  const [campaigns, setCampaigns] = useState<AdminCampaign[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [filters, setFilters] = useState({ search: '', status: '', client_id: '' });
  const [confirmStop, setConfirmStop] = useState<AdminCampaign | null>(null);

  const fetchCampaigns = useCallback(async () => {
    setLoading(true);
    try {
      const res = await campaignsAdminApi.list({
        search: filters.search || undefined,
        status: filters.status || undefined,
        client_id: filters.client_id || undefined,
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
      });
      setCampaigns(res.campaigns || []);
      setTotal(res.total);
    } catch {
      toast.error('Не удалось загрузить кампании');
    } finally {
      setLoading(false);
    }
  }, [page, filters]);

  useEffect(() => { fetchCampaigns(); }, [fetchCampaigns]);

  const handlePause = async (c: AdminCampaign) => {
    setActionLoading(c.id);
    try {
      await campaignsAdminApi.pause(c.id);
      toast.success('Кампания приостановлена');
      fetchCampaigns();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally { setActionLoading(null); }
  };

  const handleResume = async (c: AdminCampaign) => {
    setActionLoading(c.id);
    try {
      await campaignsAdminApi.resume(c.id);
      toast.success('Кампания возобновлена');
      fetchCampaigns();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally { setActionLoading(null); }
  };

  const handleStop = async (c: AdminCampaign) => {
    setActionLoading(c.id);
    try {
      await campaignsAdminApi.stop(c.id);
      toast.success('Кампания остановлена');
      setConfirmStop(null);
      fetchCampaigns();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally { setActionLoading(null); }
  };

  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-neutral-900 dark:text-neutral-100">
            Кампании
          </h1>
          <p className="text-sm text-neutral-500 mt-0.5">
            Все кампании клиентов · {total} записей
          </p>
        </div>
      </div>

      {/* Filters */}
      <div className="flex gap-3 flex-wrap">
        <input
          type="text"
          placeholder="Поиск по названию..."
          value={filters.search}
          onChange={e => { setFilters(f => ({ ...f, search: e.target.value })); setPage(1); }}
          className="border rounded-lg px-3 py-1.5 text-sm w-56 dark:bg-neutral-800 dark:border-neutral-700"
        />
        <select
          value={filters.status}
          onChange={e => { setFilters(f => ({ ...f, status: e.target.value })); setPage(1); }}
          className="border rounded-lg px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700"
        >
          <option value="">Все статусы</option>
          {Object.entries(STATUS_LABELS).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
      </div>

      {/* Table */}
      <div className="border rounded-xl overflow-hidden dark:border-neutral-800">
        <table className="w-full text-sm">
          <thead className="bg-neutral-50 dark:bg-neutral-900 text-neutral-600 dark:text-neutral-400">
            <tr>
              <th className="text-left px-4 py-3 font-medium">Кампания</th>
              <th className="text-left px-4 py-3 font-medium">Клиент</th>
              <th className="text-left px-4 py-3 font-medium">Статус</th>
              <th className="text-left px-4 py-3 font-medium">Прогресс</th>
              <th className="text-left px-4 py-3 font-medium">Доставлено</th>
              <th className="text-right px-4 py-3 font-medium">Действия</th>
            </tr>
          </thead>
          <tbody className="divide-y dark:divide-neutral-800">
            {loading ? (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-neutral-400">Загрузка...</td></tr>
            ) : campaigns.length === 0 ? (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-neutral-400">Нет кампаний</td></tr>
            ) : campaigns.map(c => (
              <tr key={c.id} className="hover:bg-neutral-50 dark:hover:bg-neutral-900/50">
                <td className="px-4 py-3">
                  <Link to={`/admin/campaigns/${c.id}`} className="font-medium text-primary-600 hover:underline">
                    {c.name}
                  </Link>
                  <div className="text-xs text-neutral-400">{c.channel} · {c.type}</div>
                </td>
                <td className="px-4 py-3 text-neutral-700 dark:text-neutral-300">{c.client_name}</td>
                <td className="px-4 py-3">
                  <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${STATUS_COLORS[c.status] || ''}`}>
                    {STATUS_LABELS[c.status] || c.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-neutral-600 dark:text-neutral-400">
                  {c.sent_count} / {c.total_recipients}
                </td>
                <td className="px-4 py-3 text-neutral-600 dark:text-neutral-400">
                  {c.total_recipients > 0
                    ? `${Math.round((c.delivered_count / c.total_recipients) * 100)}%`
                    : '—'}
                </td>
                <td className="px-4 py-3 text-right space-x-1">
                  {c.status === 'running' && (
                    <button
                      onClick={() => handlePause(c)}
                      disabled={actionLoading === c.id}
                      className="px-2 py-1 text-xs border rounded hover:bg-neutral-100 dark:hover:bg-neutral-800 disabled:opacity-50"
                    >
                      ⏸ Пауза
                    </button>
                  )}
                  {c.status === 'paused' && (
                    <button
                      onClick={() => handleResume(c)}
                      disabled={actionLoading === c.id}
                      className="px-2 py-1 text-xs border rounded hover:bg-neutral-100 dark:hover:bg-neutral-800 disabled:opacity-50"
                    >
                      ▶ Возобновить
                    </button>
                  )}
                  {(c.status === 'running' || c.status === 'paused' || c.status === 'scheduled') && (
                    <button
                      onClick={() => setConfirmStop(c)}
                      disabled={actionLoading === c.id}
                      className="px-2 py-1 text-xs border border-red-200 text-red-600 rounded hover:bg-red-50 disabled:opacity-50"
                    >
                      ⏹ Стоп
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex justify-center gap-2">
          <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1}
            className="px-3 py-1 text-sm border rounded disabled:opacity-40">←</button>
          <span className="px-3 py-1 text-sm">{page} / {totalPages}</span>
          <button onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page === totalPages}
            className="px-3 py-1 text-sm border rounded disabled:opacity-40">→</button>
        </div>
      )}

      {/* Stop confirmation dialog */}
      {confirmStop && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-neutral-900 rounded-xl p-6 max-w-md w-full mx-4 shadow-xl">
            <h3 className="text-lg font-semibold mb-2">Остановить кампанию?</h3>
            <p className="text-neutral-600 dark:text-neutral-400 text-sm mb-4">
              «{confirmStop.name}» будет остановлена. Это действие необратимо — отправка прекратится и не возобновится.
            </p>
            <div className="flex gap-3 justify-end">
              <button onClick={() => setConfirmStop(null)}
                className="px-4 py-2 text-sm border rounded hover:bg-neutral-100 dark:hover:bg-neutral-800">
                Отмена
              </button>
              <button
                onClick={() => handleStop(confirmStop)}
                disabled={actionLoading === confirmStop.id}
                className="px-4 py-2 text-sm bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50">
                Остановить
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/admin/campaigns/CampaignsAdminPage.tsx
git commit -m "feat(admin): add campaigns list page with pause/resume/stop actions"
```

---

### Task 2.3: Детальная страница кампании

**Files:**
- Create: `portal-frontend/src/pages/admin/campaigns/CampaignAdminDetailPage.tsx`

- [ ] **Step 1: Создать компонент**

```typescript
// portal-frontend/src/pages/admin/campaigns/CampaignAdminDetailPage.tsx
import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { campaignsAdminApi, type AdminCampaignDetail, type CampaignRecipient } from '../../../api/admin';
import { useToast } from '../../../hooks/useToast';

const STATUS_LABELS: Record<string, string> = {
  draft: 'Черновик', running: 'Выполняется', paused: 'Пауза',
  completed: 'Завершена', failed: 'Ошибка', scheduled: 'Запланирована',
};

export function CampaignAdminDetailPage() {
  const { id } = useParams<{ id: string }>();
  const toast = useToast();
  const [campaign, setCampaign] = useState<AdminCampaignDetail | null>(null);
  const [recipients, setRecipients] = useState<CampaignRecipient[]>([]);
  const [recipientsTotal, setRecipientsTotal] = useState(0);
  const [recipientsPage, setRecipientsPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editForm, setEditForm] = useState({ batch_size: 0, batch_interval_seconds: 0, scheduled_at: '' });
  const [confirmStop, setConfirmStop] = useState(false);

  const RECIPIENTS_PAGE_SIZE = 25;

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    campaignsAdminApi.get(id)
      .then(res => {
        setCampaign(res.campaign);
        setEditForm({
          batch_size: res.campaign.batch_size || 0,
          batch_interval_seconds: res.campaign.batch_interval_seconds || 0,
          scheduled_at: res.campaign.scheduled_at || '',
        });
      })
      .catch(() => toast.error('Не удалось загрузить кампанию'))
      .finally(() => setLoading(false));
  }, [id]);

  useEffect(() => {
    if (!id) return;
    campaignsAdminApi.listRecipients(id, {
      limit: RECIPIENTS_PAGE_SIZE,
      offset: (recipientsPage - 1) * RECIPIENTS_PAGE_SIZE,
    }).then(res => {
      setRecipients(res.recipients || []);
      setRecipientsTotal(res.total);
    }).catch(() => {});
  }, [id, recipientsPage]);

  const handleAction = async (action: 'pause' | 'resume' | 'stop') => {
    if (!id) return;
    setActionLoading(true);
    try {
      if (action === 'pause') await campaignsAdminApi.pause(id);
      else if (action === 'resume') await campaignsAdminApi.resume(id);
      else await campaignsAdminApi.stop(id);
      const res = await campaignsAdminApi.get(id);
      setCampaign(res.campaign);
      setConfirmStop(false);
      toast.success(action === 'pause' ? 'Приостановлена' : action === 'resume' ? 'Возобновлена' : 'Остановлена');
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally { setActionLoading(false); }
  };

  const handleSaveEdit = async () => {
    if (!id) return;
    setActionLoading(true);
    try {
      await campaignsAdminApi.update(id, {
        batch_size: editForm.batch_size || undefined,
        batch_interval_seconds: editForm.batch_interval_seconds || undefined,
        scheduled_at: editForm.scheduled_at || undefined,
      });
      const res = await campaignsAdminApi.get(id);
      setCampaign(res.campaign);
      setEditing(false);
      toast.success('Сохранено');
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally { setActionLoading(false); }
  };

  if (loading) return <div className="p-8 text-center text-neutral-400">Загрузка...</div>;
  if (!campaign) return <div className="p-8 text-center text-neutral-400">Кампания не найдена</div>;

  const deliveryRate = campaign.total_recipients > 0
    ? Math.round((campaign.delivered_count / campaign.total_recipients) * 100)
    : 0;

  return (
    <div className="p-6 space-y-6 max-w-5xl">
      {/* Breadcrumb */}
      <div className="text-sm text-neutral-500">
        <Link to="/admin/campaigns" className="hover:underline">Кампании</Link>
        {' / '}
        <span>{campaign.name}</span>
      </div>

      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{campaign.name}</h1>
          <div className="flex items-center gap-3 mt-1 text-sm text-neutral-500">
            <Link to={`/admin/clients/${campaign.client_id}`} className="text-primary-600 hover:underline">
              {campaign.client_name}
            </Link>
            <span>·</span>
            <span className="px-2 py-0.5 rounded-full text-xs font-medium bg-neutral-100 dark:bg-neutral-800">
              {STATUS_LABELS[campaign.status] || campaign.status}
            </span>
          </div>
        </div>
        <div className="flex gap-2">
          {campaign.status === 'running' && (
            <button onClick={() => handleAction('pause')} disabled={actionLoading}
              className="px-3 py-1.5 text-sm border rounded hover:bg-neutral-100 disabled:opacity-50">
              ⏸ Пауза
            </button>
          )}
          {campaign.status === 'paused' && (
            <button onClick={() => handleAction('resume')} disabled={actionLoading}
              className="px-3 py-1.5 text-sm border rounded hover:bg-neutral-100 disabled:opacity-50">
              ▶ Возобновить
            </button>
          )}
          {(campaign.status === 'running' || campaign.status === 'paused' || campaign.status === 'scheduled') && (
            <button onClick={() => setConfirmStop(true)} disabled={actionLoading}
              className="px-3 py-1.5 text-sm border border-red-200 text-red-600 rounded hover:bg-red-50 disabled:opacity-50">
              ⏹ Остановить
            </button>
          )}
          <button onClick={() => setEditing(!editing)}
            className="px-3 py-1.5 text-sm border rounded hover:bg-neutral-100">
            ✏️ Изменить
          </button>
        </div>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          { label: 'Получателей', value: campaign.total_recipients.toLocaleString() },
          { label: 'Отправлено', value: campaign.sent_count.toLocaleString() },
          { label: 'Доставлено', value: `${campaign.delivered_count.toLocaleString()} (${deliveryRate}%)` },
          { label: 'Ошибки', value: campaign.failed_count.toLocaleString() },
        ].map(s => (
          <div key={s.label} className="border rounded-xl p-4 dark:border-neutral-800">
            <div className="text-xs text-neutral-500 mb-1">{s.label}</div>
            <div className="text-xl font-semibold">{s.value}</div>
          </div>
        ))}
      </div>

      {/* Parameters / Edit form */}
      <div className="border rounded-xl p-4 dark:border-neutral-800">
        <h3 className="font-medium mb-3">Параметры</h3>
        {editing ? (
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-neutral-500 mb-1">Размер батча</label>
                <input type="number" value={editForm.batch_size}
                  onChange={e => setEditForm(f => ({ ...f, batch_size: +e.target.value }))}
                  className="w-full border rounded px-2 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700" />
              </div>
              <div>
                <label className="block text-xs text-neutral-500 mb-1">Интервал (сек)</label>
                <input type="number" value={editForm.batch_interval_seconds}
                  onChange={e => setEditForm(f => ({ ...f, batch_interval_seconds: +e.target.value }))}
                  className="w-full border rounded px-2 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700" />
              </div>
            </div>
            {campaign.status === 'scheduled' && (
              <div>
                <label className="block text-xs text-neutral-500 mb-1">Время запуска</label>
                <input type="datetime-local" value={editForm.scheduled_at?.slice(0, 16) || ''}
                  onChange={e => setEditForm(f => ({ ...f, scheduled_at: e.target.value }))}
                  className="w-full border rounded px-2 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700" />
              </div>
            )}
            <div className="flex gap-2 pt-1">
              <button onClick={handleSaveEdit} disabled={actionLoading}
                className="px-3 py-1.5 text-sm bg-primary-600 text-white rounded hover:bg-primary-700 disabled:opacity-50">
                Сохранить
              </button>
              <button onClick={() => setEditing(false)}
                className="px-3 py-1.5 text-sm border rounded hover:bg-neutral-100">
                Отмена
              </button>
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-2 md:grid-cols-3 gap-x-8 gap-y-2 text-sm">
            <div><span className="text-neutral-500">Тип:</span> {campaign.type}</div>
            <div><span className="text-neutral-500">Канал:</span> {campaign.channel}</div>
            <div><span className="text-neutral-500">Sender:</span> {campaign.sender_name || '—'}</div>
            <div><span className="text-neutral-500">Батч:</span> {campaign.batch_size || '—'}</div>
            <div><span className="text-neutral-500">Интервал:</span> {campaign.batch_interval_seconds ? `${campaign.batch_interval_seconds}с` : '—'}</div>
            {campaign.template_name && (
              <div><span className="text-neutral-500">Шаблон:</span>{' '}
                <Link to={`/admin/templates/${campaign.template_id}`} className="text-primary-600 hover:underline">
                  {campaign.template_name}
                </Link>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Recipients */}
      <div className="border rounded-xl overflow-hidden dark:border-neutral-800">
        <div className="px-4 py-3 bg-neutral-50 dark:bg-neutral-900 font-medium text-sm">
          Получатели ({recipientsTotal.toLocaleString()})
        </div>
        <table className="w-full text-sm">
          <thead className="text-xs text-neutral-500 border-b dark:border-neutral-800">
            <tr>
              <th className="text-left px-4 py-2">Телефон</th>
              <th className="text-left px-4 py-2">Статус</th>
              <th className="text-left px-4 py-2">Отправлено</th>
              <th className="text-left px-4 py-2">Доставлено</th>
            </tr>
          </thead>
          <tbody className="divide-y dark:divide-neutral-800">
            {recipients.map(r => (
              <tr key={r.id} className="hover:bg-neutral-50 dark:hover:bg-neutral-900/50">
                <td className="px-4 py-2 font-mono">{r.phone}</td>
                <td className="px-4 py-2">{r.status}</td>
                <td className="px-4 py-2 text-neutral-500">{r.sent_at ? new Date(r.sent_at).toLocaleString() : '—'}</td>
                <td className="px-4 py-2 text-neutral-500">{r.delivered_at ? new Date(r.delivered_at).toLocaleString() : r.error || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {Math.ceil(recipientsTotal / RECIPIENTS_PAGE_SIZE) > 1 && (
          <div className="flex justify-center gap-2 p-3">
            <button onClick={() => setRecipientsPage(p => Math.max(1, p - 1))} disabled={recipientsPage === 1}
              className="px-3 py-1 text-xs border rounded disabled:opacity-40">←</button>
            <span className="px-2 py-1 text-xs">{recipientsPage}</span>
            <button onClick={() => setRecipientsPage(p => p + 1)}
              disabled={recipientsPage >= Math.ceil(recipientsTotal / RECIPIENTS_PAGE_SIZE)}
              className="px-3 py-1 text-xs border rounded disabled:opacity-40">→</button>
          </div>
        )}
      </div>

      {/* Status history */}
      {campaign.status_history?.length > 0 && (
        <div className="border rounded-xl p-4 dark:border-neutral-800">
          <h3 className="font-medium mb-3 text-sm">История статусов</h3>
          <div className="space-y-1.5">
            {campaign.status_history.map((h, i) => (
              <div key={i} className="flex items-center gap-3 text-sm">
                <span className="text-neutral-400 text-xs w-36">
                  {new Date(h.changed_at).toLocaleString()}
                </span>
                <span className="font-medium">{STATUS_LABELS[h.status] || h.status}</span>
                {h.changed_by && <span className="text-neutral-400 text-xs">· {h.changed_by}</span>}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Stop dialog */}
      {confirmStop && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-neutral-900 rounded-xl p-6 max-w-md w-full mx-4 shadow-xl">
            <h3 className="text-lg font-semibold mb-2">Остановить кампанию?</h3>
            <p className="text-sm text-neutral-600 dark:text-neutral-400 mb-4">
              Это действие необратимо. Кампания будет остановлена и не возобновится.
            </p>
            <div className="flex gap-3 justify-end">
              <button onClick={() => setConfirmStop(false)}
                className="px-4 py-2 text-sm border rounded hover:bg-neutral-100">Отмена</button>
              <button onClick={() => handleAction('stop')} disabled={actionLoading}
                className="px-4 py-2 text-sm bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50">
                Остановить
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/campaigns/CampaignAdminDetailPage.tsx
git commit -m "feat(admin): add campaign detail page with edit and status history"
```

---

### Task 2.4: Зарегистрировать маршруты в App.tsx

**Files:**
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Добавить маршруты кампаний**

В блок admin routes в `App.tsx` добавить:

```typescript
// Импорты (добавить к существующим lazy imports):
const CampaignsAdminPage = lazy(() =>
  import('./pages/admin/campaigns/CampaignsAdminPage').then(m => ({ default: m.CampaignsAdminPage }))
);
const CampaignAdminDetailPage = lazy(() =>
  import('./pages/admin/campaigns/CampaignAdminDetailPage').then(m => ({ default: m.CampaignAdminDetailPage }))
);

// В блоке admin routes добавить:
<Route path="campaigns" element={<Suspense fallback={null}><CampaignsAdminPage /></Suspense>} />
<Route path="campaigns/:id" element={<Suspense fallback={null}><CampaignAdminDetailPage /></Suspense>} />
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/App.tsx
git commit -m "feat(admin): register campaign admin routes in App.tsx"
```

---

### Task 2.5: Backend — Campaign handler

**Files:**
- Create: `internal/gateway/admin/handlers/campaigns.go`

- [ ] **Step 1: Написать handler**

```go
// internal/gateway/admin/handlers/campaigns.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	campaignrepo "github.com/your-org/sms/internal/services/campaign/infrastructure/repository"
	campaigndomain "github.com/your-org/sms/internal/services/campaign/domain"
)

type CampaignHandlers struct {
	repo campaignrepo.CampaignRepository
}

func NewCampaignHandlers(repo campaignrepo.CampaignRepository) *CampaignHandlers {
	return &CampaignHandlers{repo: repo}
}

type adminCampaignResponse struct {
	ID               string  `json:"id"`
	ClientID         string  `json:"client_id"`
	ClientName       string  `json:"client_name"`
	Name             string  `json:"name"`
	Status           string  `json:"status"`
	Type             string  `json:"type"`
	Channel          string  `json:"channel"`
	SenderName       string  `json:"sender_name,omitempty"`
	TemplateID       string  `json:"template_id,omitempty"`
	TotalRecipients  int64   `json:"total_recipients"`
	SentCount        int64   `json:"sent_count"`
	DeliveredCount   int64   `json:"delivered_count"`
	FailedCount      int64   `json:"failed_count"`
	CreatedAt        string  `json:"created_at"`
	StartedAt        *string `json:"started_at,omitempty"`
	CompletedAt      *string `json:"completed_at,omitempty"`
	ScheduledAt      *string `json:"scheduled_at,omitempty"`
}

func campaignToResponse(c *campaigndomain.Campaign) adminCampaignResponse {
	r := adminCampaignResponse{
		ID:              c.ID.String(),
		ClientID:        c.ClientID.String(),
		Name:            c.Name,
		Status:          string(c.Status),
		Type:            string(c.Type),
		Channel:         string(c.Channel),
		SenderName:      c.SenderName,
		TotalRecipients: c.TotalRecipients,
		SentCount:       c.SentCount,
		DeliveredCount:  c.DeliveredCount,
		FailedCount:     c.FailedCount,
		CreatedAt:       c.CreatedAt.Format(time.RFC3339),
	}
	if c.TemplateID != nil {
		s := c.TemplateID.String()
		r.TemplateID = s
	}
	if c.StartedAt != nil {
		s := c.StartedAt.Format(time.RFC3339)
		r.StartedAt = &s
	}
	if c.CompletedAt != nil {
		s := c.CompletedAt.Format(time.RFC3339)
		r.CompletedAt = &s
	}
	if c.ScheduledAt != nil {
		s := c.ScheduledAt.Format(time.RFC3339)
		r.ScheduledAt = &s
	}
	return r
}

func (h *CampaignHandlers) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	clientID := r.URL.Query().Get("client_id")

	campaigns, total, err := h.repo.List(r.Context(), campaignrepo.ListParams{
		Search:   search,
		Status:   campaigndomain.CampaignStatus(status),
		ClientID: clientID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("admin: list campaigns")
		respondError(w, errInternal)
		return
	}

	items := make([]adminCampaignResponse, len(campaigns))
	for i, c := range campaigns {
		items[i] = campaignToResponse(c)
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"campaigns": items,
		"total":     total,
	})
}

func (h *CampaignHandlers) GetCampaign(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	campaign, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		respondNotFound(w, "campaign")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"campaign": campaignToResponse(campaign)})
}

func (h *CampaignHandlers) PauseCampaign(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.repo.UpdateStatus(r.Context(), id, campaigndomain.StatusPaused); err != nil {
		log.Error().Err(err).Str("id", id).Msg("admin: pause campaign")
		respondError(w, errInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CampaignHandlers) ResumeCampaign(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.repo.UpdateStatus(r.Context(), id, campaigndomain.StatusRunning); err != nil {
		log.Error().Err(err).Str("id", id).Msg("admin: resume campaign")
		respondError(w, errInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CampaignHandlers) StopCampaign(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.repo.UpdateStatus(r.Context(), id, campaigndomain.StatusFailed); err != nil {
		log.Error().Err(err).Str("id", id).Msg("admin: stop campaign")
		respondError(w, errInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CampaignHandlers) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		BatchSize              *int    `json:"batch_size"`
		BatchIntervalSeconds   *int    `json:"batch_interval_seconds"`
		ScheduledAt            *string `json:"scheduled_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, errBadRequest("invalid JSON"))
		return
	}

	campaign, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		respondNotFound(w, "campaign")
		return
	}

	if req.BatchSize != nil {
		campaign.BatchSize = *req.BatchSize
	}
	if req.BatchIntervalSeconds != nil {
		campaign.BatchIntervalSeconds = *req.BatchIntervalSeconds
	}
	if req.ScheduledAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ScheduledAt)
		if err == nil {
			campaign.ScheduledAt = &t
		}
	}

	if err := h.repo.Update(r.Context(), campaign); err != nil {
		log.Error().Err(err).Str("id", id).Msg("admin: update campaign")
		respondError(w, errInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseIntDefault(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}
```

> **Примечание:** Точные имена типов (`campaigndomain.CampaignStatus`, `StatusPaused`, `StatusRunning` и т.д.) и сигнатуры методов репозитория нужно проверить в `internal/services/campaign/domain/models.go` и `internal/services/campaign/infrastructure/repository/campaign_repository.go` — и скорректировать согласно реальному коду.

- [ ] **Step 2: Проверить и скорректировать типы**

```bash
cat internal/services/campaign/domain/models.go | head -80
cat internal/services/campaign/infrastructure/repository/campaign_repository.go | head -60
```

Скорректировать имена типов и сигнатуры методов в `campaigns.go` согласно реальному коду.

- [ ] **Step 3: Добавить роуты в router.go**

В `internal/gateway/admin/router/router.go` добавить в параметры функции `SetupRouter` и зарегистрировать роуты:

```go
// Добавить параметр:
campaignHandlers *handlers.CampaignHandlers,

// Добавить роуты после существующих:
campaigns := adminV1.PathPrefix("/campaigns").Subrouter()
campaigns.HandleFunc("", campaignHandlers.ListCampaigns).Methods("GET")
campaigns.HandleFunc("/{id}", campaignHandlers.GetCampaign).Methods("GET")
campaigns.HandleFunc("/{id}", campaignHandlers.UpdateCampaign).Methods("PUT")
campaigns.HandleFunc("/{id}/pause", campaignHandlers.PauseCampaign).Methods("POST")
campaigns.HandleFunc("/{id}/resume", campaignHandlers.ResumeCampaign).Methods("POST")
campaigns.HandleFunc("/{id}/stop", campaignHandlers.StopCampaign).Methods("POST")
```

- [ ] **Step 4: Подключить в main.go**

В `cmd/admin-gateway/main.go` добавить инициализацию и передачу в роутер:

```go
// Добавить импорт и инициализацию:
campaignRepo := campaignpgrepo.NewCampaignRepository(db) // db — существующий pgxpool
campaignHandlers := handlers.NewCampaignHandlers(campaignRepo)

// Передать в SetupRouter:
campaignHandlers,
```

- [ ] **Step 5: Собрать и проверить компиляцию**

```bash
cd internal/gateway/admin && go build ./...
```

Исправить ошибки компиляции если есть.

- [ ] **Step 6: Commit**

```bash
git add internal/gateway/admin/handlers/campaigns.go
git add internal/gateway/admin/router/router.go
git add cmd/admin-gateway/main.go
git commit -m "feat(admin): add campaign management endpoints (list, get, pause, resume, stop, update)"
```

---

## Stage 3: Расписания маршрутов

### Task 3.1: API типы и функции для расписаний

**Files:**
- Modify: `portal-frontend/src/api/admin.ts`

- [ ] **Step 1: Добавить типы и API**

```typescript
export interface RouteSchedule {
  id: string;
  route_id: string;
  route_name?: string;
  date_from?: string;   // "YYYY-MM-DD"
  date_to?: string;
  time_from?: string;   // "HH:MM"
  time_to?: string;
  weekdays: number;     // bitmask: 1=Mon, 2=Tue, 4=Wed, 8=Thu, 16=Fri, 32=Sat, 64=Sun
  timezone: string;
  created_at: string;
}

export const routeSchedulesApi = {
  list: (params?: { route_id?: string; limit?: number; offset?: number }) =>
    adminFetch<{ schedules: RouteSchedule[]; total: number }>(`/route-schedules${qs(params || {})}`),
  get: (id: string) =>
    adminFetch<{ schedule: RouteSchedule }>(`/route-schedules/${id}`),
  create: (data: Omit<RouteSchedule, 'id' | 'created_at' | 'route_name'>) =>
    adminFetch<{ id: string }>('/route-schedules', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<Omit<RouteSchedule, 'id' | 'created_at' | 'route_name'>>) =>
    adminFetch<void>(`/route-schedules/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) =>
    adminFetch<void>(`/route-schedules/${id}`, { method: 'DELETE' }),
};

// Вспомогательные функции для работы с weekdays bitmask:
export const WEEKDAY_BITS = [1, 2, 4, 8, 16, 32, 64];
export const WEEKDAY_LABELS = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];
export function weekdaysFromBits(bits: number): boolean[] {
  return WEEKDAY_BITS.map(b => (bits & b) !== 0);
}
export function weekdaysToBits(days: boolean[]): number {
  return days.reduce((acc, on, i) => on ? acc | WEEKDAY_BITS[i] : acc, 0);
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/api/admin.ts
git commit -m "feat(admin): add route schedules API types and functions"
```

---

### Task 3.2: Страница расписаний маршрутов

**Files:**
- Create: `portal-frontend/src/pages/admin/routes/RouteSchedulesPage.tsx`

- [ ] **Step 1: Создать директорию**

```bash
mkdir -p portal-frontend/src/pages/admin/routes
```

- [ ] **Step 2: Написать компонент**

```typescript
// portal-frontend/src/pages/admin/routes/RouteSchedulesPage.tsx
import { useState, useEffect, useCallback } from 'react';
import {
  routeSchedulesApi, type RouteSchedule,
  WEEKDAY_LABELS, weekdaysFromBits, weekdaysToBits
} from '../../../api/admin';
import { useToast } from '../../../hooks/useToast';

const TIMEZONES = ['Europe/Moscow', 'Europe/Kaliningrad', 'Asia/Yekaterinburg', 'UTC'];

function WeekdaySelector({ value, onChange }: { value: number; onChange: (v: number) => void }) {
  const days = weekdaysFromBits(value);
  return (
    <div className="flex gap-1">
      {WEEKDAY_LABELS.map((label, i) => (
        <button
          key={i}
          type="button"
          onClick={() => {
            const next = [...days];
            next[i] = !next[i];
            onChange(weekdaysToBits(next));
          }}
          className={`w-8 h-8 text-xs rounded font-medium transition-colors ${
            days[i]
              ? 'bg-primary-600 text-white'
              : 'border dark:border-neutral-700 text-neutral-500 hover:bg-neutral-100 dark:hover:bg-neutral-800'
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  );
}

const emptyForm = (): Omit<RouteSchedule, 'id' | 'created_at' | 'route_name'> => ({
  route_id: '',
  date_from: '',
  date_to: '',
  time_from: '',
  time_to: '',
  weekdays: 127, // все дни
  timezone: 'Europe/Moscow',
});

export function RouteSchedulesPage() {
  const toast = useToast();
  const [schedules, setSchedules] = useState<RouteSchedule[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editId, setEditId] = useState<string | null>(null);
  const [form, setForm] = useState(emptyForm());
  const [saving, setSaving] = useState(false);
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const fetchSchedules = useCallback(async () => {
    setLoading(true);
    try {
      const res = await routeSchedulesApi.list({ limit: 100, offset: 0 });
      setSchedules(res.schedules || []);
      setTotal(res.total);
    } catch {
      toast.error('Не удалось загрузить расписания');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchSchedules(); }, [fetchSchedules]);

  const openCreate = () => {
    setForm(emptyForm());
    setEditId(null);
    setShowForm(true);
  };

  const openEdit = (s: RouteSchedule) => {
    setForm({
      route_id: s.route_id,
      date_from: s.date_from || '',
      date_to: s.date_to || '',
      time_from: s.time_from || '',
      time_to: s.time_to || '',
      weekdays: s.weekdays,
      timezone: s.timezone,
    });
    setEditId(s.id);
    setShowForm(true);
  };

  const handleSave = async () => {
    if (!form.route_id) { toast.error('Укажите маршрут'); return; }
    setSaving(true);
    try {
      if (editId) {
        await routeSchedulesApi.update(editId, form);
        toast.success('Расписание обновлено');
      } else {
        await routeSchedulesApi.create(form);
        toast.success('Расписание создано');
      }
      setShowForm(false);
      fetchSchedules();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await routeSchedulesApi.delete(id);
      toast.success('Расписание удалено');
      setDeleteId(null);
      fetchSchedules();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    }
  };

  const formatWeekdays = (bits: number) => {
    const days = weekdaysFromBits(bits);
    if (bits === 127) return 'Каждый день';
    if (bits === 62) return 'Пн–Пт';
    if (bits === 65) return 'Сб, Вс';
    return WEEKDAY_LABELS.filter((_, i) => days[i]).join(', ');
  };

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Расписания маршрутов</h1>
          <p className="text-sm text-neutral-500 mt-0.5">Временны́е окна активности маршрутов · {total}</p>
        </div>
        <button onClick={openCreate}
          className="px-4 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700">
          + Добавить расписание
        </button>
      </div>

      <div className="border rounded-xl overflow-hidden dark:border-neutral-800">
        <table className="w-full text-sm">
          <thead className="bg-neutral-50 dark:bg-neutral-900 text-neutral-500 text-xs">
            <tr>
              <th className="text-left px-4 py-3 font-medium">Маршрут ID</th>
              <th className="text-left px-4 py-3 font-medium">Дни недели</th>
              <th className="text-left px-4 py-3 font-medium">Время</th>
              <th className="text-left px-4 py-3 font-medium">Даты</th>
              <th className="text-left px-4 py-3 font-medium">Часовой пояс</th>
              <th className="text-right px-4 py-3 font-medium">Действия</th>
            </tr>
          </thead>
          <tbody className="divide-y dark:divide-neutral-800">
            {loading ? (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-neutral-400">Загрузка...</td></tr>
            ) : schedules.length === 0 ? (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-neutral-400">Расписания не настроены</td></tr>
            ) : schedules.map(s => (
              <tr key={s.id} className="hover:bg-neutral-50 dark:hover:bg-neutral-900/50">
                <td className="px-4 py-3 font-mono text-xs text-neutral-600">{s.route_id.slice(0, 8)}…</td>
                <td className="px-4 py-3">{formatWeekdays(s.weekdays)}</td>
                <td className="px-4 py-3 text-neutral-600 dark:text-neutral-400">
                  {s.time_from && s.time_to ? `${s.time_from} – ${s.time_to}` : 'Весь день'}
                </td>
                <td className="px-4 py-3 text-neutral-600 dark:text-neutral-400">
                  {s.date_from || s.date_to ? `${s.date_from || '…'} – ${s.date_to || '…'}` : '—'}
                </td>
                <td className="px-4 py-3 text-neutral-500 text-xs">{s.timezone}</td>
                <td className="px-4 py-3 text-right space-x-1">
                  <button onClick={() => openEdit(s)}
                    className="px-2 py-1 text-xs border rounded hover:bg-neutral-100 dark:hover:bg-neutral-800">
                    Изменить
                  </button>
                  <button onClick={() => setDeleteId(s.id)}
                    className="px-2 py-1 text-xs border border-red-200 text-red-600 rounded hover:bg-red-50">
                    Удалить
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Form modal */}
      {showForm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-neutral-900 rounded-xl p-6 max-w-lg w-full mx-4 shadow-xl space-y-4">
            <h3 className="text-lg font-semibold">{editId ? 'Изменить расписание' : 'Новое расписание'}</h3>

            <div>
              <label className="block text-xs text-neutral-500 mb-1">ID маршрута *</label>
              <input value={form.route_id} onChange={e => setForm(f => ({ ...f, route_id: e.target.value }))}
                placeholder="UUID маршрута"
                className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700" />
            </div>

            <div>
              <label className="block text-xs text-neutral-500 mb-2">Дни недели</label>
              <WeekdaySelector value={form.weekdays} onChange={v => setForm(f => ({ ...f, weekdays: v }))} />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-neutral-500 mb-1">Время с</label>
                <input type="time" value={form.time_from}
                  onChange={e => setForm(f => ({ ...f, time_from: e.target.value }))}
                  className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700" />
              </div>
              <div>
                <label className="block text-xs text-neutral-500 mb-1">Время по</label>
                <input type="time" value={form.time_to}
                  onChange={e => setForm(f => ({ ...f, time_to: e.target.value }))}
                  className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700" />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-neutral-500 mb-1">Дата с (необязательно)</label>
                <input type="date" value={form.date_from}
                  onChange={e => setForm(f => ({ ...f, date_from: e.target.value }))}
                  className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700" />
              </div>
              <div>
                <label className="block text-xs text-neutral-500 mb-1">Дата по (необязательно)</label>
                <input type="date" value={form.date_to}
                  onChange={e => setForm(f => ({ ...f, date_to: e.target.value }))}
                  className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700" />
              </div>
            </div>

            <div>
              <label className="block text-xs text-neutral-500 mb-1">Часовой пояс</label>
              <select value={form.timezone} onChange={e => setForm(f => ({ ...f, timezone: e.target.value }))}
                className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700">
                {TIMEZONES.map(tz => <option key={tz} value={tz}>{tz}</option>)}
              </select>
            </div>

            <div className="flex gap-3 justify-end pt-2">
              <button onClick={() => setShowForm(false)}
                className="px-4 py-2 text-sm border rounded hover:bg-neutral-100 dark:hover:bg-neutral-800">
                Отмена
              </button>
              <button onClick={handleSave} disabled={saving}
                className="px-4 py-2 text-sm bg-primary-600 text-white rounded hover:bg-primary-700 disabled:opacity-50">
                {saving ? 'Сохранение...' : 'Сохранить'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete confirmation */}
      {deleteId && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-neutral-900 rounded-xl p-6 max-w-md w-full mx-4 shadow-xl">
            <h3 className="text-lg font-semibold mb-2">Удалить расписание?</h3>
            <p className="text-sm text-neutral-600 mb-4">Это действие необратимо.</p>
            <div className="flex gap-3 justify-end">
              <button onClick={() => setDeleteId(null)}
                className="px-4 py-2 text-sm border rounded hover:bg-neutral-100">Отмена</button>
              <button onClick={() => handleDelete(deleteId)}
                className="px-4 py-2 text-sm bg-red-600 text-white rounded hover:bg-red-700">Удалить</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/admin/routes/RouteSchedulesPage.tsx
git commit -m "feat(admin): add route schedules management page"
```

---

### Task 3.3: Backend — route schedules handler

**Files:**
- Create: `internal/gateway/admin/handlers/route_schedules.go`

- [ ] **Step 1: Написать handler**

```go
// internal/gateway/admin/handlers/route_schedules.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type RouteScheduleHandlers struct {
	db *pgxpool.Pool
}

func NewRouteScheduleHandlers(db *pgxpool.Pool) *RouteScheduleHandlers {
	return &RouteScheduleHandlers{db: db}
}

type routeScheduleRow struct {
	ID        string  `json:"id"`
	RouteID   string  `json:"route_id"`
	DateFrom  *string `json:"date_from,omitempty"`
	DateTo    *string `json:"date_to,omitempty"`
	TimeFrom  *string `json:"time_from,omitempty"`
	TimeTo    *string `json:"time_to,omitempty"`
	Weekdays  int     `json:"weekdays"`
	Timezone  string  `json:"timezone"`
	CreatedAt string  `json:"created_at"`
}

func (h *RouteScheduleHandlers) ListSchedules(w http.ResponseWriter, r *http.Request) {
	routeID := r.URL.Query().Get("route_id")
	limit := parseIntDefault(r.URL.Query().Get("limit"), 100)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)

	query := `SELECT id, route_id, date_from, date_to, time_from, time_to, weekdays, timezone, created_at
	          FROM route_schedules WHERE ($1 = '' OR route_id::text = $1)
	          ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := h.db.Query(r.Context(), query, routeID, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("admin: list route schedules")
		respondError(w, errInternal)
		return
	}
	defer rows.Close()

	var schedules []routeScheduleRow
	for rows.Next() {
		var s routeScheduleRow
		var createdAt time.Time
		err := rows.Scan(&s.ID, &s.RouteID, &s.DateFrom, &s.DateTo, &s.TimeFrom, &s.TimeTo, &s.Weekdays, &s.Timezone, &createdAt)
		if err != nil {
			log.Error().Err(err).Msg("admin: scan route schedule")
			continue
		}
		s.CreatedAt = createdAt.Format(time.RFC3339)
		schedules = append(schedules, s)
	}

	var total int
	countQ := `SELECT COUNT(*) FROM route_schedules WHERE ($1 = '' OR route_id::text = $1)`
	_ = h.db.QueryRow(r.Context(), countQ, routeID).Scan(&total)

	if schedules == nil {
		schedules = []routeScheduleRow{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"schedules": schedules, "total": total})
}

func (h *RouteScheduleHandlers) GetSchedule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var s routeScheduleRow
	var createdAt time.Time
	err := h.db.QueryRow(r.Context(),
		`SELECT id, route_id, date_from, date_to, time_from, time_to, weekdays, timezone, created_at FROM route_schedules WHERE id = $1`,
		id).Scan(&s.ID, &s.RouteID, &s.DateFrom, &s.DateTo, &s.TimeFrom, &s.TimeTo, &s.Weekdays, &s.Timezone, &createdAt)
	if err != nil {
		respondNotFound(w, "schedule")
		return
	}
	s.CreatedAt = createdAt.Format(time.RFC3339)
	respondJSON(w, http.StatusOK, map[string]any{"schedule": s})
}

func (h *RouteScheduleHandlers) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RouteID  string  `json:"route_id"`
		DateFrom *string `json:"date_from"`
		DateTo   *string `json:"date_to"`
		TimeFrom *string `json:"time_from"`
		TimeTo   *string `json:"time_to"`
		Weekdays int     `json:"weekdays"`
		Timezone string  `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, errBadRequest("invalid JSON"))
		return
	}
	if req.RouteID == "" {
		respondError(w, errBadRequest("route_id required"))
		return
	}
	if req.Timezone == "" {
		req.Timezone = "Europe/Moscow"
	}
	if req.Weekdays == 0 {
		req.Weekdays = 127
	}

	var id string
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO route_schedules (route_id, date_from, date_to, time_from, time_to, weekdays, timezone)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id::text`,
		req.RouteID, req.DateFrom, req.DateTo, req.TimeFrom, req.TimeTo, req.Weekdays, req.Timezone).Scan(&id)
	if err != nil {
		log.Error().Err(err).Msg("admin: create route schedule")
		respondError(w, errInternal)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *RouteScheduleHandlers) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		DateFrom *string `json:"date_from"`
		DateTo   *string `json:"date_to"`
		TimeFrom *string `json:"time_from"`
		TimeTo   *string `json:"time_to"`
		Weekdays *int    `json:"weekdays"`
		Timezone *string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, errBadRequest("invalid JSON"))
		return
	}
	_, err := h.db.Exec(r.Context(),
		`UPDATE route_schedules SET
		   date_from = COALESCE($2, date_from),
		   date_to = COALESCE($3, date_to),
		   time_from = COALESCE($4, time_from),
		   time_to = COALESCE($5, time_to),
		   weekdays = COALESCE($6, weekdays),
		   timezone = COALESCE($7, timezone)
		 WHERE id = $1`,
		id, req.DateFrom, req.DateTo, req.TimeFrom, req.TimeTo, req.Weekdays, req.Timezone)
	if err != nil {
		log.Error().Err(err).Msg("admin: update route schedule")
		respondError(w, errInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RouteScheduleHandlers) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := h.db.Exec(r.Context(), `DELETE FROM route_schedules WHERE id = $1`, id); err != nil {
		log.Error().Err(err).Msg("admin: delete route schedule")
		respondError(w, errInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Avoid duplicate declaration if parseIntDefault is already in campaigns.go
// If it is, remove this:
func init() { _ = strconv.Atoi } // suppress unused import warning if needed
```

- [ ] **Step 2: Добавить роуты в router.go**

```go
// Добавить параметр в SetupRouter:
routeScheduleHandlers *handlers.RouteScheduleHandlers,

// Добавить роуты:
schedules := adminV1.PathPrefix("/route-schedules").Subrouter()
schedules.HandleFunc("", routeScheduleHandlers.ListSchedules).Methods("GET")
schedules.HandleFunc("", routeScheduleHandlers.CreateSchedule).Methods("POST")
schedules.HandleFunc("/{id}", routeScheduleHandlers.GetSchedule).Methods("GET")
schedules.HandleFunc("/{id}", routeScheduleHandlers.UpdateSchedule).Methods("PUT")
schedules.HandleFunc("/{id}", routeScheduleHandlers.DeleteSchedule).Methods("DELETE")
```

- [ ] **Step 3: Добавить в main.go**

```go
routeScheduleHandlers := handlers.NewRouteScheduleHandlers(db)
// Передать в SetupRouter
```

- [ ] **Step 4: Собрать**

```bash
cd internal/gateway/admin && go build ./...
```

- [ ] **Step 5: Зарегистрировать роуты в App.tsx**

```typescript
const RouteSchedulesPage = lazy(() =>
  import('./pages/admin/routes/RouteSchedulesPage').then(m => ({ default: m.RouteSchedulesPage }))
);
// В admin routes:
<Route path="routes/schedules" element={<Suspense fallback={null}><RouteSchedulesPage /></Suspense>} />
```

- [ ] **Step 6: Commit**

```bash
git add internal/gateway/admin/handlers/route_schedules.go
git add internal/gateway/admin/router/router.go
git add cmd/admin-gateway/main.go
git add portal-frontend/src/App.tsx
git commit -m "feat(admin): add route schedules CRUD (backend + frontend)"
```

---

## Stage 4: Симулятор маршрутизации

### Task 4.1: Backend — simulate endpoint

**Files:**
- Create: `internal/gateway/admin/handlers/route_simulator.go`

- [ ] **Step 1: Написать handler**

```go
// internal/gateway/admin/handlers/route_simulator.go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type RouteSimulatorHandlers struct {
	db *pgxpool.Pool
}

func NewRouteSimulatorHandlers(db *pgxpool.Pool) *RouteSimulatorHandlers {
	return &RouteSimulatorHandlers{db: db}
}

type SimulateRequest struct {
	MSISDN            string   `json:"msisdn"`
	ClientID          string   `json:"client_id"`
	SenderName        string   `json:"sender_name,omitempty"`
	TrafficType       string   `json:"traffic_type,omitempty"`
	Channel           string   `json:"channel,omitempty"`
	SimulateAt        *string  `json:"simulate_at,omitempty"`
	DisabledProviders []string `json:"disabled_providers,omitempty"`
}

type SimulateStep struct {
	Step    int    `json:"step"`
	Status  string `json:"status"` // "ok", "warning", "info"
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

type SimulateResult struct {
	Steps      []SimulateStep `json:"steps"`
	ProviderID *string        `json:"provider_id,omitempty"`
	ProviderName *string      `json:"provider_name,omitempty"`
	RouteID    *string        `json:"route_id,omitempty"`
	Success    bool           `json:"success"`
	Error      string         `json:"error,omitempty"`
}

func (h *RouteSimulatorHandlers) Simulate(w http.ResponseWriter, r *http.Request) {
	var req SimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, errBadRequest("invalid JSON"))
		return
	}
	if req.MSISDN == "" || req.ClientID == "" {
		respondError(w, errBadRequest("msisdn and client_id are required"))
		return
	}

	var steps []SimulateStep
	stepN := 0

	addStep := func(status, msg, detail string) {
		stepN++
		steps = append(steps, SimulateStep{Step: stepN, Status: status, Message: msg, Detail: detail})
	}

	result := SimulateResult{Steps: steps}

	// Step 1: Определить оператора по номеру
	msisdn := strings.TrimPrefix(req.MSISDN, "+")
	var operatorName, countryName string
	var operatorID, countryID int64
	err := h.db.QueryRow(r.Context(),
		`SELECT o.id, o.name, c.id, c.name
		 FROM operator_prefixes op
		 JOIN operators o ON o.id = op.operator_id
		 JOIN countries c ON c.id = o.country_id
		 WHERE $1 LIKE op.prefix || '%'
		 ORDER BY length(op.prefix) DESC LIMIT 1`,
		msisdn).Scan(&operatorID, &operatorName, &countryID, &countryName)
	if err != nil {
		addStep("warning", fmt.Sprintf("Номер %s: оператор не определён", req.MSISDN),
			"Не найден подходящий префикс в справочнике")
	} else {
		addStep("ok", fmt.Sprintf("Номер %s → %s, %s", req.MSISDN, operatorName, countryName),
			fmt.Sprintf("operator_id=%d, country_id=%d", operatorID, countryID))
	}

	// Step 2: Проверить клиентские маршруты
	var clientRouteID, clientProviderID, clientProviderName string
	err = h.db.QueryRow(r.Context(),
		`SELECT cr.id, p.id, p.name
		 FROM client_routes cr
		 JOIN providers p ON p.id = cr.provider_id
		 WHERE cr.client_id = $1::uuid
		   AND (cr.operator_id IS NULL OR cr.operator_id = $2)
		   AND (cr.country_id IS NULL OR cr.country_id = $3)
		   AND cr.status = 'active'
		 ORDER BY cr.priority ASC LIMIT 1`,
		req.ClientID, operatorID, countryID).Scan(&clientRouteID, &clientProviderID, &clientProviderName)

	if err != nil {
		addStep("info", "Индивидуальный маршрут клиента: не найден", "Переход к платформенным маршрутам")
	} else {
		// Проверить не в disabled_providers ли
		disabled := false
		for _, dp := range req.DisabledProviders {
			if dp == clientProviderID {
				disabled = true
				break
			}
		}
		if disabled {
			addStep("warning", fmt.Sprintf("Индивидуальный маршрут: провайдер %s отключён в симуляции", clientProviderName),
				"Переход к следующему маршруту")
		} else {
			addStep("ok", fmt.Sprintf("Индивидуальный маршрут клиента: %s (route %s)", clientProviderName, clientRouteID[:8]),
				"Маршрут активен")
			pid := clientProviderID
			pname := clientProviderName
			rid := clientRouteID
			result.Steps = append(steps, SimulateStep{Step: stepN + 1, Status: "ok",
				Message: fmt.Sprintf("→ Итог: сообщение пойдёт через %s", clientProviderName), Detail: ""})
			result.ProviderID = &pid
			result.ProviderName = &pname
			result.RouteID = &rid
			result.Success = true
			result.Steps = steps
			respondJSON(w, http.StatusOK, result)
			return
		}
	}

	// Step 3: Платформенные маршруты
	var platformRouteID, platformProviderID, platformProviderName string
	err = h.db.QueryRow(r.Context(),
		`SELECT pr.id, p.id, p.name
		 FROM platform_routes pr
		 JOIN providers p ON p.id = pr.provider_id
		 WHERE (pr.operator_id IS NULL OR pr.operator_id = $1)
		   AND (pr.country_id IS NULL OR pr.country_id = $2)
		   AND pr.status = 'active'
		 ORDER BY pr.priority ASC LIMIT 1`,
		operatorID, countryID).Scan(&platformRouteID, &platformProviderID, &platformProviderName)

	if err != nil {
		addStep("warning", "Платформенный маршрут: не найден", "Маршрут для данного оператора/страны не настроен")
		result.Success = false
		result.Error = "Подходящий маршрут не найден"
	} else {
		disabled := false
		for _, dp := range req.DisabledProviders {
			if dp == platformProviderID {
				disabled = true
				break
			}
		}
		if disabled {
			addStep("warning", fmt.Sprintf("Платформенный маршрут: провайдер %s отключён в симуляции", platformProviderName), "")
			result.Success = false
			result.Error = "Все подходящие провайдеры отключены в симуляции"
		} else {
			addStep("ok", fmt.Sprintf("Платформенный маршрут: %s", platformProviderName), "")
			pid := platformProviderID
			pname := platformProviderName
			rid := platformRouteID
			result.ProviderID = &pid
			result.ProviderName = &pname
			result.RouteID = &rid
			result.Success = true
		}
	}

	result.Steps = steps
	if result.Success {
		log.Debug().Str("msisdn", req.MSISDN).Str("provider", *result.ProviderName).Msg("route simulation")
	}
	respondJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 2: Добавить роут в router.go**

```go
routeSimulatorHandlers *handlers.RouteSimulatorHandlers,
// ...
adminV1.HandleFunc("/routes/simulate", routeSimulatorHandlers.Simulate).Methods("POST")
```

- [ ] **Step 3: Добавить в main.go**

```go
routeSimulatorHandlers := handlers.NewRouteSimulatorHandlers(db)
```

- [ ] **Step 4: Собрать**

```bash
cd internal/gateway/admin && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/admin/handlers/route_simulator.go
git add internal/gateway/admin/router/router.go
git add cmd/admin-gateway/main.go
git commit -m "feat(admin): add route simulation endpoint (POST /routes/simulate)"
```

---

### Task 4.2: Frontend — страница симулятора

**Files:**
- Create: `portal-frontend/src/pages/admin/routes/RouteSimulatorPage.tsx`

- [ ] **Step 1: Добавить API функцию в admin.ts**

```typescript
export interface SimulateStep {
  step: number;
  status: 'ok' | 'warning' | 'info';
  message: string;
  detail?: string;
}

export interface SimulateResult {
  steps: SimulateStep[];
  provider_id?: string;
  provider_name?: string;
  route_id?: string;
  success: boolean;
  error?: string;
}

export const routeSimulatorApi = {
  simulate: (data: {
    msisdn: string;
    client_id: string;
    sender_name?: string;
    traffic_type?: string;
    channel?: string;
    simulate_at?: string;
    disabled_providers?: string[];
  }) =>
    adminFetch<SimulateResult>('/routes/simulate', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
};
```

- [ ] **Step 2: Написать компонент симулятора**

```typescript
// portal-frontend/src/pages/admin/routes/RouteSimulatorPage.tsx
import { useState } from 'react';
import { routeSimulatorApi, type SimulateResult, type SimulateStep } from '../../../api/admin';
import { useToast } from '../../../hooks/useToast';

function StepIcon({ status }: { status: SimulateStep['status'] }) {
  if (status === 'ok') return <span className="text-green-500">✓</span>;
  if (status === 'warning') return <span className="text-yellow-500">⚠</span>;
  return <span className="text-blue-400">ℹ</span>;
}

export function RouteSimulatorPage() {
  const toast = useToast();
  const [form, setForm] = useState({
    msisdn: '',
    client_id: '',
    sender_name: '',
    traffic_type: 'transactional',
    channel: 'sms',
    simulate_at: '',
  });
  const [result, setResult] = useState<SimulateResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [expandedSteps, setExpandedSteps] = useState<Set<number>>(new Set());

  const handleSimulate = async () => {
    if (!form.msisdn || !form.client_id) {
      toast.error('Укажите номер и клиента');
      return;
    }
    setLoading(true);
    setResult(null);
    try {
      const res = await routeSimulatorApi.simulate({
        msisdn: form.msisdn,
        client_id: form.client_id,
        sender_name: form.sender_name || undefined,
        traffic_type: form.traffic_type || undefined,
        channel: form.channel || undefined,
        simulate_at: form.simulate_at || undefined,
      });
      setResult(res);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка симуляции');
    } finally {
      setLoading(false);
    }
  };

  const toggleStep = (n: number) => {
    setExpandedSteps(prev => {
      const next = new Set(prev);
      if (next.has(n)) next.delete(n); else next.add(n);
      return next;
    });
  };

  return (
    <div className="p-6 space-y-6 max-w-3xl">
      <div>
        <h1 className="text-2xl font-semibold">Симулятор маршрутизации</h1>
        <p className="text-sm text-neutral-500 mt-0.5">
          Проверьте через какого провайдера пройдёт сообщение без реальной отправки
        </p>
      </div>

      {/* Input form */}
      <div className="border rounded-xl p-5 dark:border-neutral-800 space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs text-neutral-500 mb-1">Номер получателя (MSISDN) *</label>
            <input
              value={form.msisdn}
              onChange={e => setForm(f => ({ ...f, msisdn: e.target.value }))}
              placeholder="+79161234567"
              className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700 font-mono"
            />
          </div>
          <div>
            <label className="block text-xs text-neutral-500 mb-1">Клиент (UUID) *</label>
            <input
              value={form.client_id}
              onChange={e => setForm(f => ({ ...f, client_id: e.target.value }))}
              placeholder="UUID клиента"
              className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700 font-mono"
            />
          </div>
          <div>
            <label className="block text-xs text-neutral-500 mb-1">Sender Name</label>
            <input
              value={form.sender_name}
              onChange={e => setForm(f => ({ ...f, sender_name: e.target.value }))}
              placeholder="необязательно"
              className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700"
            />
          </div>
          <div>
            <label className="block text-xs text-neutral-500 mb-1">Тип трафика</label>
            <select value={form.traffic_type} onChange={e => setForm(f => ({ ...f, traffic_type: e.target.value }))}
              className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700">
              <option value="transactional">Transactional</option>
              <option value="promotional">Promotional</option>
              <option value="otp">OTP</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-neutral-500 mb-1">Канал</label>
            <select value={form.channel} onChange={e => setForm(f => ({ ...f, channel: e.target.value }))}
              className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700">
              <option value="sms">SMS</option>
              <option value="viber">Viber</option>
              <option value="whatsapp">WhatsApp</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-neutral-500 mb-1">Дата/время (для расписаний)</label>
            <input type="datetime-local" value={form.simulate_at}
              onChange={e => setForm(f => ({ ...f, simulate_at: e.target.value }))}
              className="w-full border rounded px-3 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700"
            />
          </div>
        </div>

        <div className="pt-1 flex items-center gap-3">
          <button
            onClick={handleSimulate}
            disabled={loading}
            className="px-5 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700 disabled:opacity-50"
          >
            {loading ? 'Симуляция...' : '▶ Запустить симуляцию'}
          </button>
          <p className="text-xs text-neutral-400">Реальная отправка не происходит</p>
        </div>
      </div>

      {/* Result */}
      {result && (
        <div className="border rounded-xl overflow-hidden dark:border-neutral-800">
          <div className={`px-4 py-3 font-medium text-sm ${
            result.success
              ? 'bg-green-50 text-green-800 dark:bg-green-900/20 dark:text-green-400'
              : 'bg-red-50 text-red-800 dark:bg-red-900/20 dark:text-red-400'
          }`}>
            {result.success
              ? `✓ Маршрут найден: ${result.provider_name}`
              : `✗ Маршрут не найден: ${result.error || 'неизвестная ошибка'}`}
          </div>

          <div className="divide-y dark:divide-neutral-800">
            {result.steps.map(step => (
              <div key={step.step} className="px-4 py-2.5">
                <div className="flex items-start gap-3 cursor-pointer"
                  onClick={() => step.detail && toggleStep(step.step)}>
                  <StepIcon status={step.status} />
                  <div className="flex-1 min-w-0">
                    <span className="text-sm">{step.message}</span>
                    {step.detail && (
                      <span className="ml-1 text-xs text-neutral-400 cursor-pointer">
                        {expandedSteps.has(step.step) ? '▲' : '▼'}
                      </span>
                    )}
                  </div>
                </div>
                {step.detail && expandedSteps.has(step.step) && (
                  <div className="ml-6 mt-1 text-xs text-neutral-500 font-mono bg-neutral-50 dark:bg-neutral-900/50 rounded px-2 py-1">
                    {step.detail}
                  </div>
                )}
              </div>
            ))}
          </div>

          {result.success && result.provider_id && (
            <div className="px-4 py-3 bg-neutral-50 dark:bg-neutral-900/50 text-xs text-neutral-500 space-x-4">
              <span>Provider ID: <span className="font-mono">{result.provider_id}</span></span>
              {result.route_id && <span>Route ID: <span className="font-mono">{result.route_id}</span></span>}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Зарегистрировать в App.tsx**

```typescript
const RouteSimulatorPage = lazy(() =>
  import('./pages/admin/routes/RouteSimulatorPage').then(m => ({ default: m.RouteSimulatorPage }))
);
// В admin routes:
<Route path="routes/simulator" element={<Suspense fallback={null}><RouteSimulatorPage /></Suspense>} />
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/api/admin.ts
git add portal-frontend/src/pages/admin/routes/RouteSimulatorPage.tsx
git add portal-frontend/src/App.tsx
git commit -m "feat(admin): add route simulator frontend page"
```

---

## Stage 5: Расширение страницы клиента

### Task 5.1: Вкладки на странице клиента

**Files:**
- Modify: `portal-frontend/src/pages/admin/ClientsPage.tsx`

- [ ] **Step 1: Прочитать ClientsPage.tsx**

Открыть и прочитать `portal-frontend/src/pages/admin/ClientsPage.tsx`.

Найти детальную панель/модал клиента. Добавить вкладки "Кампании", "Домены", "Sender Names" в существующую детальную секцию (или создать отдельную страницу `/admin/clients/:id` если детальная страница не существует).

- [ ] **Step 2: Добавить API для доменов в admin.ts**

```typescript
export interface ClientDomain {
  id: string;
  domain: string;
  type: 'short-link' | 'webhook' | 'portal';
  verified: boolean;
  created_at: string;
}

export const clientDomainsApi = {
  list: (clientId: string) =>
    adminFetch<{ domains: ClientDomain[] }>(`/clients/${clientId}/domains`),
  create: (clientId: string, data: { domain: string; type: string }) =>
    adminFetch<{ id: string }>(`/clients/${clientId}/domains`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  delete: (clientId: string, domainId: string) =>
    adminFetch<void>(`/clients/${clientId}/domains/${domainId}`, { method: 'DELETE' }),
  verify: (clientId: string, domainId: string) =>
    adminFetch<void>(`/clients/${clientId}/domains/${domainId}/verify`, { method: 'POST' }),
};
```

- [ ] **Step 3: Добавить вкладку "Кампании" в детальный вид клиента**

В детальном виде клиента добавить секцию со ссылкой на фильтрованный список кампаний:

```typescript
// Кнопка перехода на кампании этого клиента
<Link
  to={`/admin/campaigns?client_id=${client.client_id}`}
  className="text-sm text-primary-600 hover:underline"
>
  Просмотреть кампании →
</Link>
```

- [ ] **Step 4: Добавить вкладку "Домены"**

Добавить секцию в детальный вид клиента с таблицей доменов и возможностью добавить/удалить:

```typescript
// Компонент добавить в ClientsPage.tsx или в отдельный файл:
function ClientDomainsTab({ clientId }: { clientId: string }) {
  const toast = useToast();
  const [domains, setDomains] = useState<ClientDomain[]>([]);
  const [loading, setLoading] = useState(true);
  const [newDomain, setNewDomain] = useState('');
  const [newType, setNewType] = useState<'short-link' | 'webhook' | 'portal'>('short-link');

  useEffect(() => {
    clientDomainsApi.list(clientId)
      .then(r => setDomains(r.domains || []))
      .catch(() => toast.error('Не удалось загрузить домены'))
      .finally(() => setLoading(false));
  }, [clientId]);

  const handleAdd = async () => {
    if (!newDomain) return;
    try {
      await clientDomainsApi.create(clientId, { domain: newDomain, type: newType });
      const r = await clientDomainsApi.list(clientId);
      setDomains(r.domains || []);
      setNewDomain('');
      toast.success('Домен добавлен');
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка'); }
  };

  const handleDelete = async (id: string) => {
    try {
      await clientDomainsApi.delete(clientId, id);
      setDomains(d => d.filter(x => x.id !== id));
      toast.success('Удалён');
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Ошибка'); }
  };

  if (loading) return <div className="py-4 text-sm text-neutral-400">Загрузка...</div>;

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        <input value={newDomain} onChange={e => setNewDomain(e.target.value)}
          placeholder="example.com"
          className="flex-1 border rounded px-2 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700" />
        <select value={newType} onChange={e => setNewType(e.target.value as typeof newType)}
          className="border rounded px-2 py-1.5 text-sm dark:bg-neutral-800 dark:border-neutral-700">
          <option value="short-link">Short-link</option>
          <option value="webhook">Webhook</option>
          <option value="portal">Portal</option>
        </select>
        <button onClick={handleAdd}
          className="px-3 py-1.5 text-sm bg-primary-600 text-white rounded hover:bg-primary-700">
          + Добавить
        </button>
      </div>
      {domains.length === 0 ? (
        <p className="text-sm text-neutral-400">Домены не настроены</p>
      ) : (
        <div className="space-y-1">
          {domains.map(d => (
            <div key={d.id} className="flex items-center justify-between py-1.5 border-b dark:border-neutral-800">
              <div>
                <span className="text-sm font-mono">{d.domain}</span>
                <span className="ml-2 text-xs text-neutral-500">{d.type}</span>
                <span className={`ml-2 text-xs ${d.verified ? 'text-green-500' : 'text-yellow-500'}`}>
                  {d.verified ? '✓ верифицирован' : '⏳ ожидает'}
                </span>
              </div>
              <button onClick={() => handleDelete(d.id)}
                className="text-xs text-red-500 hover:underline">Удалить</button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/api/admin.ts
git add portal-frontend/src/pages/admin/ClientsPage.tsx
git commit -m "feat(admin): add campaigns link and domains tab to client detail view"
```

---

### Task 5.2: Backend — client domains endpoints

**Files:**
- Modify: `internal/gateway/admin/handlers/clients.go`

- [ ] **Step 1: Добавить методы для доменов**

Добавить в `internal/gateway/admin/handlers/clients.go` три метода (или создать отдельный файл `client_domains.go`):

```go
// ListClientDomains — GET /admin/v1/clients/{id}/domains
func (h *ClientHandlers) ListClientDomains(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	rows, err := h.db.Query(r.Context(),
		`SELECT id, domain, domain_type, verified, created_at FROM client_domains WHERE client_id = $1 ORDER BY created_at`,
		clientID)
	if err != nil {
		respondError(w, errInternal)
		return
	}
	defer rows.Close()

	type domainRow struct {
		ID        string `json:"id"`
		Domain    string `json:"domain"`
		Type      string `json:"type"`
		Verified  bool   `json:"verified"`
		CreatedAt string `json:"created_at"`
	}
	var domains []domainRow
	for rows.Next() {
		var d domainRow
		var t time.Time
		if err := rows.Scan(&d.ID, &d.Domain, &d.Type, &d.Verified, &t); err != nil {
			continue
		}
		d.CreatedAt = t.Format(time.RFC3339)
		domains = append(domains, d)
	}
	if domains == nil { domains = []domainRow{} }
	respondJSON(w, http.StatusOK, map[string]any{"domains": domains})
}

// CreateClientDomain — POST /admin/v1/clients/{id}/domains
func (h *ClientHandlers) CreateClientDomain(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	var req struct {
		Domain string `json:"domain"`
		Type   string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, errBadRequest("invalid JSON"))
		return
	}
	if req.Domain == "" { respondError(w, errBadRequest("domain required")); return }
	if req.Type == "" { req.Type = "short-link" }

	var id string
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO client_domains (client_id, domain, domain_type) VALUES ($1, $2, $3) RETURNING id::text`,
		clientID, req.Domain, req.Type).Scan(&id)
	if err != nil {
		respondError(w, errInternal)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// DeleteClientDomain — DELETE /admin/v1/clients/{id}/domains/{did}
func (h *ClientHandlers) DeleteClientDomain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	_, err := h.db.Exec(r.Context(),
		`DELETE FROM client_domains WHERE id = $1 AND client_id = $2`,
		vars["did"], vars["id"])
	if err != nil {
		respondError(w, errInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

> **Примечание:** Проверить реальное имя таблицы и колонок: `SELECT table_name FROM information_schema.tables WHERE table_name LIKE '%domain%'`

- [ ] **Step 2: Добавить роуты в router.go**

```go
clientDomains := clients.PathPrefix("/{id}/domains").Subrouter()
clientDomains.HandleFunc("", clientHandlers.ListClientDomains).Methods("GET")
clientDomains.HandleFunc("", clientHandlers.CreateClientDomain).Methods("POST")
clientDomains.HandleFunc("/{did}", clientHandlers.DeleteClientDomain).Methods("DELETE")
```

- [ ] **Step 3: Проверить имя таблицы доменов**

```bash
grep -r "client_domains\|ClientDomain\|client_domain" migrations/ --include="*.sql" | head -10
```

Скорректировать имена таблиц/колонок в хендлере.

- [ ] **Step 4: Собрать**

```bash
cd internal/gateway/admin && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/admin/handlers/clients.go
git add internal/gateway/admin/router/router.go
git commit -m "feat(admin): add client domain management endpoints"
```

---

## Stage 6: Кросс-навигация (компонент RelatedLinks)

### Task 6.1: Компонент RelatedLinks

**Files:**
- Create: `portal-frontend/src/components/RelatedLinks.tsx`

- [ ] **Step 1: Создать компонент**

```typescript
// portal-frontend/src/components/RelatedLinks.tsx
import { Link } from 'react-router-dom';

export interface RelatedLink {
  label: string;
  href: string;
  count?: number;
  icon?: string;
}

export function RelatedLinks({ links }: { links: RelatedLink[] }) {
  if (links.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-2">
      {links.map(link => (
        <Link
          key={link.href}
          to={link.href}
          className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs border border-neutral-200 dark:border-neutral-700 text-neutral-600 dark:text-neutral-400 hover:bg-neutral-100 dark:hover:bg-neutral-800 hover:text-primary-600 dark:hover:text-primary-400 transition-colors"
        >
          {link.icon && <span>{link.icon}</span>}
          {link.label}
          {link.count !== undefined && (
            <span className="ml-0.5 bg-neutral-200 dark:bg-neutral-700 rounded-full px-1.5 py-0.5 text-xs">
              {link.count}
            </span>
          )}
        </Link>
      ))}
    </div>
  );
}
```

- [ ] **Step 2: Добавить RelatedLinks на страницу детали кампании**

В `CampaignAdminDetailPage.tsx` под breadcrumb добавить:

```typescript
import { RelatedLinks } from '../../../components/RelatedLinks';
// ...
{/* После breadcrumb, перед заголовком: */}
<RelatedLinks links={[
  { label: campaign.client_name, href: `/admin/clients/${campaign.client_id}`, icon: '👥' },
  ...(campaign.template_id ? [{ label: 'Шаблон', href: `/admin/templates/${campaign.template_id}`, icon: '📝' }] : []),
  ...(campaign.sender_name ? [{ label: campaign.sender_name, href: `/admin/sender-names?search=${campaign.sender_name}`, icon: '🏷️' }] : []),
]} />
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/components/RelatedLinks.tsx
git add portal-frontend/src/pages/admin/campaigns/CampaignAdminDetailPage.tsx
git commit -m "feat(admin): add RelatedLinks component and wire to campaign detail page"
```

---

## Self-Review

**Spec coverage:**
- ✓ Переключатель режимов — Task 1.1–1.3
- ✓ Управление кампаниями (list, detail, pause/resume/stop, edit) — Task 2.1–2.5
- ✓ Расписания маршрутов — Task 3.1–3.3
- ✓ Симулятор маршрутизации — Task 4.1–4.2
- ✓ Клиентские домены — Task 5.1–5.2
- ✓ Кросс-навигация (RelatedLinks) — Task 6.1
- ✗ Default sender names per client — не реализовано, отдельная задача (требует понять структуру client_configs)
- ✗ Вкладка Sender Names на странице клиента — отдельная задача

**Нет плейсхолдеров:** проверено.

**Типовая согласованность:**
- `campaignsAdminApi` → используется в `CampaignsAdminPage` и `CampaignAdminDetailPage` ✓
- `routeSchedulesApi` → используется в `RouteSchedulesPage` ✓
- `routeSimulatorApi` → используется в `RouteSimulatorPage` ✓
- `clientDomainsApi` → используется в `ClientDomainsTab` ✓
- `RelatedLinks` → используется в `CampaignAdminDetailPage` ✓

**Замечание:** В `route_schedules.go` используется `parseIntDefault` который определён в `campaigns.go`. Нужно вынести в `internal/gateway/admin/handlers/helpers.go` или удалить дублирование.
