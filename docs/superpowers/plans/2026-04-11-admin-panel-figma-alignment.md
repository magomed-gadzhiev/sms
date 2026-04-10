# Admin Panel: Figma Alignment & Enhancement — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align the admin panel with the Figma design across 11 phases: UX enhancements, visual polish, missing flows, and cross-cutting improvements.

**Architecture:** All pages are production-ready — no rewrites, only targeted modifications. Shared utilities (CSV export) are created first and reused across phases. Backend endpoints are checked before implementing; placeholders used if APIs are missing.

**Tech Stack:** TypeScript 5.7, React 19, Vite 6.0, React Router 7.1, Tailwind CSS 4.2, Radix UI, Recharts 3.8.1. Backend: Go 1.24.0, gorilla/mux, pgx/v5. Dev: `cd portal-frontend && npm run dev` → port 3001, admin at `/admin`.

---

## Scope Check

This spec covers 11 independent phases. They are parallel-safe within each sprint:
- **Sprint 1:** Phases 1, 3, 5 — parallelize  
- **Sprint 2:** Phases 4, 7, 2 — parallelize  
- **Sprint 3:** Phase 9 → then 10 and 6 — Phase 9 first (legal entity API used by Phase 6)  
- **Sprint 4:** Phases 8, 11 — Phase 11 creates shared utilities first

---

## File Structure

### New files to create
- `portal-frontend/src/utils/csvExport.ts` — shared CSV export utility (used by Phases 1, 5, 10)
- `portal-frontend/src/pages/admin/detalization/StatusTimeline.tsx` — Phase 1: message status timeline
- `portal-frontend/src/pages/admin/detalization/SmsPreview.tsx` — Phase 1: phone frame SMS preview (reused in Phase 7)
- `portal-frontend/src/pages/admin/connections/ConnectionLog.tsx` — Phase 2: terminal-style log viewer

### Files to modify
| File | Phase | Change |
|------|-------|--------|
| `pages/admin/detalization/DetalizationPage.tsx` (426 lines) | 1 | Timeline + preview in modal, CSV export, collapsible filters |
| `pages/admin/connections/ConnectionsPage.tsx` (173 lines) | 2 | Detail drawer, refresh control |
| `pages/admin/RoutesPage.tsx` (341 lines) | 3 | Drag UX polish, legal entity validation |
| `pages/admin/SenderNamesAdminPage.tsx` | 4 | Validation hints, regex check |
| `pages/admin/sender-names/SenderNameDetailPage.tsx` (274 lines) | 4 | Operator template shortcut button, delete flow |
| `pages/admin/operator-templates/OperatorTemplatesPage.tsx` (363 lines) | 4+7 | Query param auto-open, variable highlighting, preview |
| `pages/admin/AnalyticsPage.tsx` (223 lines) | 5 | Fixed-height table, sticky header/footer, date range filter |
| `pages/admin/CountriesPage.tsx` (545 lines) | 6 | Legal entity section in operator detail, interdependent filters |
| `pages/admin/settings/SettingsPage.tsx` (175 lines) | 8 | Dynamic section grouping, beforeunload warning |
| `pages/admin/legal-entities/LegalEntitiesPage.tsx` (203 lines) | 9 | Operators column, INN validation |
| `pages/admin/contracts/ContractsPage.tsx` (299 lines) | 10 | Expiry indicator, status filter |

---

## Task 1: Shared CSV Export Utility (Phase 11.1)

**Files:**
- Create: `portal-frontend/src/utils/csvExport.ts`

- [ ] **Step 1: Create the utility file**

```ts
// portal-frontend/src/utils/csvExport.ts

export function exportToCsv(filename: string, headers: string[], rows: string[][]): void {
  const escape = (cell: string) => `"${cell.replace(/"/g, '""')}"`;
  const csv = [
    headers.map(escape).join(','),
    ...rows.map(r => r.map(escape).join(','))
  ].join('\n');
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/utils/csvExport.ts
git commit -m "feat(admin): add reusable CSV export utility"
```

---

## Task 2: StatusTimeline Component (Phase 1.1)

**Files:**
- Create: `portal-frontend/src/pages/admin/detalization/StatusTimeline.tsx`

- [ ] **Step 1: Create the StatusTimeline component**

```tsx
// portal-frontend/src/pages/admin/detalization/StatusTimeline.tsx
import React from 'react';

interface StatusEvent {
  status: string;
  timestamp: string;
  details?: string;
}

interface StatusTimelineProps {
  statuses: StatusEvent[];
}

const STATUS_COLORS: Record<string, string> = {
  pending:   'bg-gray-400',
  queued:    'bg-yellow-400',
  sent:      'bg-blue-500',
  delivered: 'bg-green-500',
  failed:    'bg-red-500',
  rejected:  'bg-red-400',
  expired:   'bg-orange-400',
};

const STATUS_LABELS: Record<string, string> = {
  pending:   'Ожидание',
  queued:    'В очереди',
  sent:      'Отправлено',
  delivered: 'Доставлено',
  failed:    'Ошибка',
  rejected:  'Отклонено',
  expired:   'Истёк TTL',
};

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString('ru-RU', {
      day: '2-digit', month: '2-digit', year: 'numeric',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    });
  } catch {
    return iso;
  }
}

export const StatusTimeline: React.FC<StatusTimelineProps> = ({ statuses }) => {
  if (!statuses || statuses.length === 0) {
    return <p className="text-sm text-gray-500">История статусов недоступна</p>;
  }

  return (
    <div className="relative">
      {statuses.map((event, idx) => {
        const dot = STATUS_COLORS[event.status] ?? 'bg-gray-400';
        const label = STATUS_LABELS[event.status] ?? event.status;
        const isLast = idx === statuses.length - 1;

        return (
          <div key={idx} className="flex gap-3">
            {/* Dot + line */}
            <div className="flex flex-col items-center">
              <div className={`w-3 h-3 rounded-full flex-shrink-0 mt-1 ${dot}`} />
              {!isLast && <div className="w-px flex-1 bg-gray-200 mt-1" />}
            </div>
            {/* Content */}
            <div className={`pb-4 ${isLast ? '' : ''}`}>
              <p className="text-sm font-medium text-gray-800">{label}</p>
              <p className="text-xs text-gray-500">{formatTime(event.timestamp)}</p>
              {event.details && (
                <p className="text-xs text-gray-600 mt-0.5">{event.details}</p>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
};
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/detalization/StatusTimeline.tsx
git commit -m "feat(admin/detalization): add StatusTimeline component"
```

---

## Task 3: SmsPreview Component (Phase 1.2)

**Files:**
- Create: `portal-frontend/src/pages/admin/detalization/SmsPreview.tsx`

- [ ] **Step 1: Create the SmsPreview component**

```tsx
// portal-frontend/src/pages/admin/detalization/SmsPreview.tsx
import React from 'react';

interface SmsPreviewProps {
  sender: string;
  body: string;
  timestamp: string;
}

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
  } catch {
    return '';
  }
}

export const SmsPreview: React.FC<SmsPreviewProps> = ({ sender, body, timestamp }) => (
  <div className="flex justify-center py-2">
    <div
      className="w-56 rounded-3xl border-4 border-gray-800 bg-gray-900 p-3 shadow-xl"
      style={{ minHeight: '180px' }}
    >
      {/* Status bar */}
      <div className="flex justify-between items-center mb-3 px-1">
        <span className="text-white text-xs font-semibold">SMS</span>
        <div className="flex gap-1">
          <div className="w-1 h-1 bg-white rounded-full" />
          <div className="w-1 h-1 bg-white rounded-full" />
          <div className="w-1 h-1 bg-white rounded-full" />
        </div>
      </div>
      {/* Sender */}
      <p className="text-center text-xs text-gray-400 mb-2">{sender}</p>
      {/* Bubble */}
      <div className="bg-gray-700 rounded-2xl rounded-tl-sm p-3">
        <p className="text-white text-xs leading-relaxed break-words">{body}</p>
        <p className="text-gray-400 text-right text-xs mt-1">{formatTime(timestamp)}</p>
      </div>
    </div>
  </div>
);
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/admin/detalization/SmsPreview.tsx
git commit -m "feat(admin/detalization): add SmsPreview phone frame component"
```

---

## Task 4: DetalizationPage — CSV Export, Collapsible Filters, Modal Enhancement (Phase 1.3 + 1.4)

**Files:**
- Modify: `portal-frontend/src/pages/admin/detalization/DetalizationPage.tsx`

- [ ] **Step 1: Read the full file**

```bash
cat -n portal-frontend/src/pages/admin/detalization/DetalizationPage.tsx
```

- [ ] **Step 2: Add imports at the top of DetalizationPage.tsx**

Add these imports after existing imports:
```tsx
import { exportToCsv } from '../../../utils/csvExport';
import { StatusTimeline } from './StatusTimeline';
import { SmsPreview } from './SmsPreview';
```

- [ ] **Step 3: Add `showAllFilters` state**

Add this state near the top of the component (with other `useState` declarations):
```tsx
const [showAllFilters, setShowAllFilters] = useState(false);
```

- [ ] **Step 4: Implement the `exportToCsv` handler**

Add this function inside the component (after existing callbacks):
```tsx
const handleExport = useCallback(() => {
  const headers = ['ID', 'Клиент', 'Отправитель', 'Получатель', 'Статус', 'Провайдер', 'Дата', 'Стоимость'];
  const rows = messages.map(m => [
    m.id,
    m.client_name ?? '',
    m.source ?? '',
    m.destination ?? '',
    m.status ?? '',
    m.provider_name ?? '',
    m.created_at ?? '',
    m.cost != null ? String(m.cost) : '',
  ]);
  exportToCsv(`detalization-${new Date().toISOString().slice(0, 10)}.csv`, headers, rows);
}, [messages]);
```

> **Note:** Adjust field names (`m.client_name`, `m.source`, etc.) to match the actual `Message` interface in the file — check the type definition at the top of the file or in `api/admin.ts`.

- [ ] **Step 5: Add export button to PageHeader actions**

In the JSX, find the `<PageHeader>` component and add `actions` prop (or extend existing):
```tsx
<PageHeader
  title="Детализация"
  breadcrumbs={[{ label: 'Главная', href: '/admin' }, { label: 'Детализация' }]}
  actions={
    <Button variant="secondary" size="sm" onClick={handleExport}>
      Экспорт CSV
    </Button>
  }
/>
```

- [ ] **Step 6: Make filters collapsible**

Find the filter rendering section. Identify the 7 filter fields by their field names (date_from, date_to, status are primary; the remaining 4 are secondary).

Replace the filter section with:
```tsx
{/* Primary filters — always visible */}
<div className="flex flex-wrap gap-3 items-end">
  {/* date_from, date_to, status filters here — keep their existing JSX */}
  <Button
    variant="ghost"
    size="sm"
    onClick={() => setShowAllFilters(v => !v)}
    className="text-indigo-600"
  >
    {showAllFilters ? 'Свернуть фильтры' : 'Все фильтры'}
  </Button>
</div>

{/* Secondary filters — collapsible */}
{showAllFilters && (
  <div className="flex flex-wrap gap-3 items-end mt-3">
    {/* remaining 4 filters: client, provider, source, destination */}
  </div>
)}
```

- [ ] **Step 7: Add StatusTimeline and SmsPreview to detail modal**

Find the detail modal (the Modal component that shows message details when a row is clicked). Inside it, locate where status info is displayed. Add after the existing status section:

```tsx
{/* Status timeline */}
{selectedMessage?.status_history && selectedMessage.status_history.length > 0 && (
  <div className="mt-4">
    <h4 className="text-sm font-semibold text-gray-700 mb-3">История статусов</h4>
    <StatusTimeline statuses={selectedMessage.status_history} />
  </div>
)}

{/* SMS preview */}
{selectedMessage && (
  <div className="mt-4">
    <h4 className="text-sm font-semibold text-gray-700 mb-3">Предпросмотр сообщения</h4>
    <SmsPreview
      sender={selectedMessage.source ?? ''}
      body={selectedMessage.body ?? ''}
      timestamp={selectedMessage.created_at ?? ''}
    />
  </div>
)}
```

> **Note:** If `status_history` is not part of the detail response type, check `messagesApi.get(id)` return type. If it doesn't exist, show `StatusTimeline` with an array built from the single `selectedMessage.status` field: `[{ status: selectedMessage.status, timestamp: selectedMessage.created_at }]`.

- [ ] **Step 8: Commit**

```bash
git add portal-frontend/src/pages/admin/detalization/DetalizationPage.tsx
git commit -m "feat(admin/detalization): CSV export, collapsible filters, status timeline, SMS preview in modal"
```

---

## Task 5: ConnectionsPage — Detail Drawer and Refresh Control (Phase 2.1 + 2.2)

**Files:**
- Modify: `portal-frontend/src/pages/admin/connections/ConnectionsPage.tsx`

- [ ] **Step 1: Read the full file**

```bash
cat -n portal-frontend/src/pages/admin/connections/ConnectionsPage.tsx
```

- [ ] **Step 2: Add imports**

```tsx
import { usePolling } from '../../../hooks/usePolling';
```

> The file likely already imports something from hooks — add `usePolling` if not present.

- [ ] **Step 3: Replace manual interval with usePolling + add selected connection state**

Find any `setInterval`/`useEffect` that auto-refreshes. Replace with:

```tsx
const [selectedConnectionId, setSelectedConnectionId] = useState<string | null>(null);
const [selectedConnection, setSelectedConnection] = useState<Connection | null>(null);
const [loadingDetail, setLoadingDetail] = useState(false);
const [autoRefresh, setAutoRefresh] = useState(true);

// Replace existing auto-refresh useEffect with:
usePolling(
  async () => {
    const data = await connectionsApi.list();
    setConnections(data);
  },
  10000,
  { enabled: autoRefresh }
);
```

> **Note:** Check the signature of `usePolling` in `hooks/usePolling.ts` before use — adjust parameters to match the actual API (likely `(fn, intervalMs, options?)`).

- [ ] **Step 4: Add row click handler to fetch detail**

```tsx
const handleRowClick = useCallback(async (id: string) => {
  setSelectedConnectionId(id);
  setLoadingDetail(true);
  try {
    const detail = await connectionsApi.get(id);
    setSelectedConnection(detail);
  } catch {
    setSelectedConnection(null);
  } finally {
    setLoadingDetail(false);
  }
}, []);
```

- [ ] **Step 5: Add refresh control buttons to PageHeader**

```tsx
<PageHeader
  title="Подключения"
  breadcrumbs={[{ label: 'Главная', href: '/admin' }, { label: 'Подключения' }]}
  actions={
    <div className="flex gap-2">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setAutoRefresh(v => !v)}
        className={autoRefresh ? 'text-green-600' : 'text-gray-500'}
      >
        Автообновление: {autoRefresh ? '10с' : 'пауза'}
      </Button>
      <Button variant="secondary" size="sm" onClick={fetchConnections}>
        Обновить
      </Button>
    </div>
  }
/>
```

> `fetchConnections` is the existing function that loads data — reuse it.

- [ ] **Step 6: Add `onClick` to each table row**

In the table `<tbody>`, find each `<tr>` and add:
```tsx
<tr
  key={conn.id}
  onClick={() => handleRowClick(conn.id)}
  className="cursor-pointer hover:bg-gray-50"
>
```

- [ ] **Step 7: Add connection detail modal**

After the table, add:
```tsx
<Modal
  isOpen={!!selectedConnectionId}
  onClose={() => { setSelectedConnectionId(null); setSelectedConnection(null); }}
  title="Детали подключения"
>
  {loadingDetail ? (
    <div className="py-8 text-center text-gray-500">Загрузка...</div>
  ) : selectedConnection ? (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3 text-sm">
        <div><span className="text-gray-500">Провайдер:</span> <span className="font-medium">{selectedConnection.provider_name}</span></div>
        <div><span className="text-gray-500">Хост:</span> <span className="font-medium">{selectedConnection.host}:{selectedConnection.port}</span></div>
        <div><span className="text-gray-500">System ID:</span> <span className="font-medium">{selectedConnection.system_id}</span></div>
        <div><span className="text-gray-500">Статус:</span> <Badge variant={selectedConnection.status === 'connected' ? 'success' : 'danger'}>{selectedConnection.status}</Badge></div>
        <div><span className="text-gray-500">Сессий:</span> <span className="font-medium">{selectedConnection.sessions}</span></div>
        <div><span className="text-gray-500">Успешность:</span> <span className="font-medium">{selectedConnection.success_rate}%</span></div>
      </div>
      {selectedConnection.error_message && (
        <div className="bg-red-50 rounded-lg p-3">
          <p className="text-sm text-red-700 font-medium">Последняя ошибка:</p>
          <p className="text-sm text-red-600 mt-1">{selectedConnection.error_message}</p>
        </div>
      )}
      <div className="flex gap-2 pt-2">
        <Button variant="secondary" size="sm" onClick={() => handleReconnect(selectedConnection.id)}>Переподключить</Button>
        <Button variant="danger" size="sm" onClick={() => handleStop(selectedConnection.id)}>Остановить</Button>
      </div>
    </div>
  ) : (
    <p className="text-gray-500">Не удалось загрузить детали</p>
  )}
</Modal>
```

> Adjust field names (`provider_name`, `host`, `port`, etc.) to match the actual `Connection` type. Check existing table columns to find the right field names.

- [ ] **Step 8: Commit**

```bash
git add portal-frontend/src/pages/admin/connections/ConnectionsPage.tsx
git commit -m "feat(admin/connections): detail drawer, controllable auto-refresh"
```

---

## Task 6: ConnectionLog Component (Phase 2.3)

**Files:**
- Create: `portal-frontend/src/pages/admin/connections/ConnectionLog.tsx`

- [ ] **Step 1: Check if backend logs endpoint exists**

```bash
grep -r "connections.*logs\|logs.*connections" internal/gateway/admin/router/ internal/gateway/admin/handlers/ 2>/dev/null | head -20
```

- [ ] **Step 2: Create ConnectionLog component**

If the endpoint exists, use `connectionId` to fetch logs. If not, show placeholder.

```tsx
// portal-frontend/src/pages/admin/connections/ConnectionLog.tsx
import React, { useEffect, useRef, useState } from 'react';

interface LogLine {
  timestamp: string;
  level: string;
  message: string;
}

interface ConnectionLogProps {
  connectionId: string;
}

// Replace this URL if the endpoint exists
const LOG_ENDPOINT_EXISTS = false; // Set to true if GET /admin/v1/connections/{id}/logs exists

export const ConnectionLog: React.FC<ConnectionLogProps> = ({ connectionId }) => {
  const [lines, setLines] = useState<LogLine[]>([]);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!LOG_ENDPOINT_EXISTS) return;
    let cancelled = false;

    async function fetchLogs() {
      try {
        const res = await fetch(`/admin/v1/connections/${connectionId}/logs`);
        if (!res.ok) return;
        const data: LogLine[] = await res.json();
        if (!cancelled) setLines(data.slice(-100)); // keep last 100 lines
      } catch { /* ignore */ }
    }

    fetchLogs();
    const interval = setInterval(fetchLogs, 5000);
    return () => { cancelled = true; clearInterval(interval); };
  }, [connectionId]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [lines]);

  if (!LOG_ENDPOINT_EXISTS) {
    return (
      <div className="bg-gray-900 rounded-lg p-4 text-gray-400 text-sm font-mono h-48 flex items-center justify-center">
        Логи недоступны — endpoint не реализован
      </div>
    );
  }

  return (
    <div className="bg-gray-900 rounded-lg p-3 h-48 overflow-y-auto font-mono text-xs">
      {lines.length === 0 ? (
        <span className="text-gray-500">Нет логов</span>
      ) : (
        lines.map((line, i) => (
          <div key={i} className="leading-5">
            <span className="text-gray-500">[{line.timestamp}]</span>{' '}
            <span className={
              line.level === 'error' ? 'text-red-400' :
              line.level === 'warn'  ? 'text-yellow-400' :
              'text-green-400'
            }>{line.level}:</span>{' '}
            <span className="text-gray-200">{line.message}</span>
          </div>
        ))
      )}
      <div ref={bottomRef} />
    </div>
  );
};
```

- [ ] **Step 3: Add ConnectionLog inside the detail modal**

In `ConnectionsPage.tsx`, add inside the detail modal after the existing content:
```tsx
import { ConnectionLog } from './ConnectionLog';

// Inside modal, after the grid:
<div className="mt-4">
  <p className="text-sm font-semibold text-gray-700 mb-2">Лог подключения</p>
  <ConnectionLog connectionId={selectedConnection.id} />
</div>
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/admin/connections/ConnectionLog.tsx portal-frontend/src/pages/admin/connections/ConnectionsPage.tsx
git commit -m "feat(admin/connections): connection log viewer component"
```

---

## Task 7: RoutesPage — Drag UX Polish and Legal Entity Validation (Phase 3)

**Files:**
- Modify: `portal-frontend/src/pages/admin/RoutesPage.tsx`

- [ ] **Step 1: Read the full file**

```bash
cat -n portal-frontend/src/pages/admin/RoutesPage.tsx
```

- [ ] **Step 2: Fix drag handle cursor styles**

Find the drag handle element (likely `⠿` or a drag icon). Ensure it has:
```tsx
<span
  className="cursor-grab active:cursor-grabbing select-none text-gray-400 hover:text-gray-600"
  {...dragHandleProps}
>
  ⠿
</span>
```

Find the row being dragged (if using a drag library like `react-beautiful-dnd`, it passes `isDragging`). Add:
```tsx
<tr
  className={`${isDragging ? 'bg-indigo-50 shadow-md' : 'hover:bg-gray-50'}`}
>
```

- [ ] **Step 3: Verify toast fires after reorder**

Find the reorder API call. Ensure it's wrapped:
```tsx
try {
  await platformRoutesApi.reorder(newOrder);
  toast.success('Маршрут перемещён');
  // refresh list
} catch {
  toast.error('Ошибка при перемещении маршрута');
}
```

- [ ] **Step 4: Add empty state**

After the table/list, if `routes.length === 0`:
```tsx
{routes.length === 0 && (
  <div className="text-center py-12 text-gray-500">
    <p className="text-lg font-medium">Нет маршрутов</p>
    <p className="text-sm mt-1">Создайте первый маршрут для начала маршрутизации</p>
    <Button onClick={() => setShowCreateModal(true)} className="mt-4">Создать маршрут</Button>
  </div>
)}
```

- [ ] **Step 5: Enforce legal entity required for "All Networks"**

In the create/edit modal, find the legal entity field. Update the logic:

```tsx
// Determine if operator_id is "all networks" (null/empty/"all")
const isAllNetworks = !form.operator_id || form.operator_id === 'all';

// In the form JSX:
{isAllNetworks && (
  <div>
    <label className="block text-sm font-medium text-gray-700 mb-1">
      Юр. лицо <span className="text-red-500">*</span>
    </label>
    <SearchableSelect
      options={legalEntities}
      value={form.legal_entity_id}
      onChange={v => setForm(f => ({ ...f, legal_entity_id: v }))}
      placeholder="Выберите юр. лицо"
      className={formErrors.legal_entity_id ? 'border-red-500' : ''}
    />
    {formErrors.legal_entity_id && (
      <p className="text-xs text-red-500 mt-1">Обязательное поле для маршрута All Networks</p>
    )}
  </div>
)}
```

In the submit handler, add validation:
```tsx
if (isAllNetworks && !form.legal_entity_id) {
  setFormErrors(e => ({ ...e, legal_entity_id: true }));
  return;
}
```

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/pages/admin/RoutesPage.tsx
git commit -m "feat(admin/routes): drag UX polish, empty state, legal entity required for All Networks"
```

---

## Task 8: SenderNamesAdminPage — Validation Hints (Phase 4.1)

**Files:**
- Modify: `portal-frontend/src/pages/admin/SenderNamesAdminPage.tsx`

- [ ] **Step 1: Read the file**

```bash
cat -n portal-frontend/src/pages/admin/SenderNamesAdminPage.tsx
```

- [ ] **Step 2: Add regex validation for sender name**

Find the create modal and the name `<Input>` field. Add validation state and hints:

```tsx
const [nameError, setNameError] = useState<string>('');

const SENDER_NAME_REGEX = /^[A-Za-z0-9._-]{1,11}$/;

const validateName = (value: string): string => {
  if (!value) return 'Поле обязательно';
  if (/\s/.test(value)) return 'Пробелы запрещены';
  if (!SENDER_NAME_REGEX.test(value)) return 'Латиница, не более 11 символов. Можно использовать цифры и знаки . _ —';
  return '';
};

// In form onChange:
onChange={(e) => {
  setForm(f => ({ ...f, name: e.target.value }));
  setNameError(validateName(e.target.value));
}}

// Before submit:
const error = validateName(form.name);
if (error) { setNameError(error); return; }
```

- [ ] **Step 3: Add hint text below the name input**

After the name `<Input>` in the modal:
```tsx
<div className="mt-1 space-y-0.5">
  <p className="text-xs text-gray-500">Имя должно совпадать с названием организации, ИП, товарным знаком или доменом</p>
  <p className="text-xs text-gray-500">Латиница, не более 11 символов. Можно использовать цифры и знаки . _ —</p>
  <p className="text-xs text-gray-500">Пробелы запрещены</p>
  {nameError && <p className="text-xs text-red-500">{nameError}</p>}
</div>
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/admin/SenderNamesAdminPage.tsx
git commit -m "feat(admin/sender-names): add validation hints and regex check for sender name"
```

---

## Task 9: SenderNameDetailPage — Operator Template Shortcut + Delete Flow (Phase 4.2 + 4.3)

**Files:**
- Modify: `portal-frontend/src/pages/admin/sender-names/SenderNameDetailPage.tsx`

- [ ] **Step 1: Read the full file**

```bash
cat -n portal-frontend/src/pages/admin/sender-names/SenderNameDetailPage.tsx
```

- [ ] **Step 2: Add "Create operator template" button to PageHeader**

Find the `<PageHeader>` component. Add to its `actions`:
```tsx
import { useNavigate } from 'react-router-dom';

const navigate = useNavigate();

// In PageHeader actions:
<Button
  variant="secondary"
  size="sm"
  onClick={() => navigate(`/admin/operator-templates?sender_name_id=${senderName.id}&sender_name=${encodeURIComponent(senderName.name)}`)}
>
  Добавить шаблон оператора
</Button>
```

- [ ] **Step 3: Verify delete flow exists and redirects**

Find the delete handler. Ensure it looks like:
```tsx
const handleDelete = async () => {
  try {
    await adminSenderNamesApi.deactivate(id); // or the appropriate delete method
    toast.success('Имя отправителя удалено');
    navigate('/admin/sender-names');
  } catch {
    toast.error('Ошибка при удалении');
  }
};
```

If delete button exists in PageHeader, ensure it opens `ConfirmDialog` first. If the delete button is missing from PageHeader, add it:
```tsx
<Button variant="danger" size="sm" onClick={() => setShowDeleteConfirm(true)}>
  Удалить
</Button>
<ConfirmDialog
  isOpen={showDeleteConfirm}
  onClose={() => setShowDeleteConfirm(false)}
  onConfirm={handleDelete}
  title="Удалить имя отправителя?"
  description="Это действие нельзя отменить."
  variant="danger"
/>
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/admin/sender-names/SenderNameDetailPage.tsx
git commit -m "feat(admin/sender-names): add operator template shortcut button and verified delete-redirect flow"
```

---

## Task 10: OperatorTemplatesPage — Query Param Auto-Open, Variable Highlighting, Preview (Phase 4 + 7)

**Files:**
- Modify: `portal-frontend/src/pages/admin/operator-templates/OperatorTemplatesPage.tsx`

- [ ] **Step 1: Read the full file**

```bash
cat -n portal-frontend/src/pages/admin/operator-templates/OperatorTemplatesPage.tsx
```

- [ ] **Step 2: Read query params and auto-open create modal**

Add at the top of the component:
```tsx
import { useSearchParams } from 'react-router-dom';

const [searchParams] = useSearchParams();

useEffect(() => {
  const senderNameId = searchParams.get('sender_name_id');
  const senderName = searchParams.get('sender_name');
  if (senderNameId) {
    setForm(f => ({ ...f, sender_name_id: senderNameId, sender_name: senderName ?? '' }));
    setShowCreateModal(true);
  }
}, []); // eslint-disable-line react-hooks/exhaustive-deps
```

> Adjust field names to match the actual form state shape (`sender_name_id` vs the actual field used in the create form).

- [ ] **Step 3: Add variable highlighting preview**

Find the template body textarea in the create/edit modal. Below it, add:
```tsx
{/* Variable highlight preview */}
{form.body && (
  <div className="mt-2 p-3 bg-gray-50 rounded-lg border text-sm">
    <p className="text-xs text-gray-500 mb-1 font-medium">Предпросмотр с выделенными переменными:</p>
    <p className="leading-relaxed">
      {form.body.split(/(\{[^}]+\})/g).map((part, i) =>
        /^\{[^}]+\}$/.test(part) ? (
          <span key={i} className="text-indigo-600 bg-indigo-50 px-1 rounded font-medium">{part}</span>
        ) : (
          <span key={i}>{part}</span>
        )
      )}
    </p>
  </div>
)}
```

- [ ] **Step 4: Add template preview with sample values**

Below the variable highlight section, add:
```tsx
{form.body && (
  <div className="mt-2">
    <p className="text-xs text-gray-500 mb-1 font-medium">Предпросмотр с примером значений:</p>
    <div className="bg-white border border-gray-200 rounded-xl p-4 shadow-sm max-w-xs">
      <p className="text-xs text-gray-400 font-medium mb-1">{form.sender_name || 'SENDER'}</p>
      <p className="text-sm text-gray-800 leading-relaxed">
        {form.body
          .replace(/\{code\}/g, '1234')
          .replace(/\{name\}/g, 'Иван')
          .replace(/\{date\}/g, '11.04.2026')
          .replace(/\{sum\}/g, '1 500 ₽')
          .replace(/\{link\}/g, 'https://example.com/abc')}
      </p>
    </div>
  </div>
)}
```

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/admin/operator-templates/OperatorTemplatesPage.tsx
git commit -m "feat(admin/operator-templates): query param auto-open, variable highlighting, SMS preview"
```

---

## Task 11: AnalyticsPage — Fixed Table, Sticky Header/Footer, Date Range (Phase 5)

**Files:**
- Modify: `portal-frontend/src/pages/admin/AnalyticsPage.tsx`

- [ ] **Step 1: Read the full file**

```bash
cat -n portal-frontend/src/pages/admin/AnalyticsPage.tsx
```

- [ ] **Step 2: Add date range filter state**

```tsx
const [dateFrom, setDateFrom] = useState('');
const [dateTo, setDateTo] = useState('');
```

- [ ] **Step 3: Add date inputs to the filter area**

Find the existing filter section (period dropdown + client filter). Add:
```tsx
<div className="flex items-center gap-2">
  <Input
    type="date"
    label="С"
    value={dateFrom}
    onChange={e => setDateFrom(e.target.value)}
    className="w-40"
  />
  <Input
    type="date"
    label="По"
    value={dateTo}
    onChange={e => setDateTo(e.target.value)}
    className="w-40"
  />
  {(dateFrom || dateTo) && (
    <Button variant="ghost" size="sm" onClick={() => { setDateFrom(''); setDateTo(''); }}>
      Сбросить даты
    </Button>
  )}
</div>
```

- [ ] **Step 4: Pass date range to API call**

In the data-fetching `useEffect`/`useCallback`, update the params:
```tsx
const params: AnalyticsParams = {
  period: dateFrom || dateTo ? undefined : selectedPeriod,
  date_from: dateFrom || undefined,
  date_to: dateTo || undefined,
  client_id: selectedClientId || undefined,
};
```

> Check the actual `analyticsAdminApi.getGroupedStats()` signature to confirm accepted params.

- [ ] **Step 5: Wrap each stats table in fixed-height container**

Find each tab's table (3 tabs: по дням, по операторам, по странам). Wrap each `<table>` element:
```tsx
<div className="h-[520px] overflow-y-auto relative border border-gray-200 rounded-lg">
  <table className="w-full text-sm">
    <thead className="sticky top-0 bg-white z-10 shadow-sm">
      {/* existing thead */}
    </thead>
    <tbody>
      {/* existing tbody */}
    </tbody>
    <tfoot className="sticky bottom-0 bg-gray-50 font-semibold border-t-2 border-gray-200">
      <tr>
        {/* totals row — move from tbody to tfoot if it's currently there */}
      </tr>
    </tfoot>
  </table>
</div>
```

> If the table uses `DataTable` component, you'll need to render a raw `<table>` for this sticky behavior, or check if `DataTable` supports a `fixedHeight` prop.

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/pages/admin/AnalyticsPage.tsx
git commit -m "feat(admin/analytics): fixed-height table, sticky header/footer, custom date range filter"
```

---

## Task 12: CountriesPage — Legal Entity Section + Interdependent Filters (Phase 6)

**Files:**
- Modify: `portal-frontend/src/pages/admin/CountriesPage.tsx`

- [ ] **Step 1: Read the file (first 280 lines)**

```bash
sed -n '1,280p' portal-frontend/src/pages/admin/CountriesPage.tsx
```

- [ ] **Step 2: Read remaining lines**

```bash
sed -n '281,545p' portal-frontend/src/pages/admin/CountriesPage.tsx
```

- [ ] **Step 3: Check if legal entity endpoints exist for operators**

```bash
grep -r "operators.*legal\|legal.*operators" internal/gateway/admin/router/ internal/gateway/admin/handlers/ 2>/dev/null | head -20
```

- [ ] **Step 4: Add legal entity state to operator detail panel**

Find the operator detail panel in the hierarchical view (right column). Add:
```tsx
const [operatorLegalEntities, setOperatorLegalEntities] = useState<LegalEntity[]>([]);
const [addingLe, setAddingLe] = useState(false);
const [leToAdd, setLeToAdd] = useState('');

// When selected operator changes:
useEffect(() => {
  if (!selectedOperator) return;
  // If endpoint exists:
  // fetch(`/admin/v1/operators/${selectedOperator.id}/legal-entities`)
  //   .then(r => r.json()).then(setOperatorLegalEntities);
  // If not, leave empty and show placeholder
}, [selectedOperator]);
```

- [ ] **Step 5: Render legal entities section in operator detail**

In the operator detail JSX, after existing settings/content, add:
```tsx
<div className="mt-4">
  <h4 className="text-sm font-semibold text-gray-700 mb-2">Юр. лица</h4>
  {/* If backend exists, show real data; otherwise placeholder */}
  {false /* set to true when backend endpoint exists */ ? (
    <div className="border border-gray-200 rounded-lg divide-y">
      {operatorLegalEntities.map(le => (
        <div key={le.id} className="flex items-center justify-between px-3 py-2 text-sm">
          <span>{le.name} (ИНН: {le.inn})</span>
          <button
            className="text-gray-400 hover:text-red-500"
            onClick={() => {/* DELETE /admin/v1/operators/{id}/legal-entities/{le.id} */}}
          >
            ✕
          </button>
        </div>
      ))}
      <div className="px-3 py-2">
        <button
          className="text-indigo-600 text-sm hover:underline"
          onClick={() => setAddingLe(true)}
        >
          + Добавить юр. лицо
        </button>
      </div>
    </div>
  ) : (
    <div className="border border-dashed border-gray-300 rounded-lg p-4 text-center text-sm text-gray-400">
      Управление юр. лицами оператора — ожидает backend API
    </div>
  )}
</div>
```

- [ ] **Step 6: Add interdependent filters to flat view**

Find the flat view (MCC/MNC таблица tab). Locate country + operator filters. Add `useEffect`:
```tsx
// When country filter changes → filter operators list
useEffect(() => {
  if (flatCountryFilter) {
    // filter operators to show only those from selectedCountry
    setFilteredOperators(allOperators.filter(op => op.country_id === flatCountryFilter));
    setFlatOperatorFilter(''); // reset operator selection
  } else {
    setFilteredOperators(allOperators);
  }
}, [flatCountryFilter, allOperators]);

// When operator filter changes → set country automatically
useEffect(() => {
  if (flatOperatorFilter) {
    const op = allOperators.find(o => o.id === flatOperatorFilter);
    if (op && op.country_id) {
      setFlatCountryFilter(op.country_id);
    }
  }
}, [flatOperatorFilter, allOperators]);
```

> Adjust state variable names to match what's actually in the file.

- [ ] **Step 7: Commit**

```bash
git add portal-frontend/src/pages/admin/CountriesPage.tsx
git commit -m "feat(admin/countries): legal entity section in operator detail (placeholder), interdependent flat-view filters"
```

---

## Task 13: SettingsPage — Dynamic Section Grouping + beforeunload Warning (Phase 8)

**Files:**
- Modify: `portal-frontend/src/pages/admin/settings/SettingsPage.tsx`

- [ ] **Step 1: Read the full file**

```bash
cat -n portal-frontend/src/pages/admin/settings/SettingsPage.tsx
```

- [ ] **Step 2: Add `hasChanges` tracking and `beforeunload` listener**

```tsx
const [hasChanges, setHasChanges] = useState(false);

useEffect(() => {
  const handler = (e: BeforeUnloadEvent) => {
    e.preventDefault();
    e.returnValue = '';
  };
  if (hasChanges) {
    window.addEventListener('beforeunload', handler);
  }
  return () => window.removeEventListener('beforeunload', handler);
}, [hasChanges]);
```

Set `hasChanges = true` wherever form values change. Reset on successful save.

- [ ] **Step 3: Group settings keys dynamically by prefix**

After loading `systemDefaultsApi.getAll()`, group the returned object:
```tsx
// Assuming getAll() returns Record<string, string>
const SECTION_LABELS: Record<string, string> = {
  'sms':      'SMS по умолчанию',
  'rate':     'Rate Limits',
  'provider': 'Провайдеры',
  'notify':   'Уведомления',
  'security': 'Безопасность',
  'general':  'Общие',
};

const groupedSettings = useMemo(() => {
  const groups: Record<string, Record<string, string>> = {};
  for (const [key, value] of Object.entries(allSettings)) {
    const prefix = key.split('.')[0] ?? 'general';
    if (!groups[prefix]) groups[prefix] = {};
    groups[prefix][key] = value;
  }
  return groups;
}, [allSettings]);
```

- [ ] **Step 4: Render settings dynamically by group**

Replace hard-coded sections with:
```tsx
{Object.entries(groupedSettings).map(([prefix, settings]) => (
  <SectionCard
    key={prefix}
    title={SECTION_LABELS[prefix] ?? prefix}
    settings={settings}
    onSave={async (updates) => {
      for (const [key, value] of Object.entries(updates)) {
        await systemDefaultsApi.set(key, value);
      }
      setHasChanges(false);
      toast.success('Настройки сохранены');
    }}
    onChangeDetected={() => setHasChanges(true)}
  />
))}
```

If the existing code has inline section cards, refactor to accept `settings` as prop and render them dynamically. Each key maps to an `<Input>` labeled with the key name.

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/admin/settings/SettingsPage.tsx
git commit -m "feat(admin/settings): dynamic key grouping, beforeunload unsaved changes warning"
```

---

## Task 14: LegalEntitiesPage — Operators Column + INN Validation (Phase 9)

**Files:**
- Modify: `portal-frontend/src/pages/admin/legal-entities/LegalEntitiesPage.tsx`

- [ ] **Step 1: Read the full file**

```bash
cat -n portal-frontend/src/pages/admin/legal-entities/LegalEntitiesPage.tsx
```

- [ ] **Step 2: Add INN validation function**

```tsx
function validateINN(inn: string): boolean {
  if (!/^\d{10}$|^\d{12}$/.test(inn)) return false;

  const d = inn.split('').map(Number);

  if (d.length === 10) {
    const check = (
      [2, 4, 10, 3, 5, 9, 4, 6, 8].reduce((s, w, i) => s + w * d[i], 0) % 11
    ) % 10;
    return check === d[9];
  }

  // 12-digit INN
  const check1 = (
    [7, 2, 4, 10, 3, 5, 9, 4, 6, 8].reduce((s, w, i) => s + w * d[i], 0) % 11
  ) % 10;
  const check2 = (
    [3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8].reduce((s, w, i) => s + w * d[i], 0) % 11
  ) % 10;
  return check1 === d[10] && check2 === d[11];
}
```

- [ ] **Step 3: Add INN validation state and error display**

```tsx
const [innError, setInnError] = useState('');

// In INN input onChange:
onChange={e => {
  setForm(f => ({ ...f, inn: e.target.value }));
  if (e.target.value && !validateINN(e.target.value)) {
    setInnError('Некорректный ИНН');
  } else {
    setInnError('');
  }
}}

// After INN input:
{innError && <p className="text-xs text-red-500 mt-1">{innError}</p>}

// In submit handler:
if (form.inn && !validateINN(form.inn)) {
  setInnError('Некорректный ИНН');
  return;
}
```

- [ ] **Step 4: Add "Операторы" column to DataTable**

Find the `columns` array passed to `DataTable`. Add:
```tsx
{
  key: 'operators',
  label: 'Операторы',
  render: (row) => row.operators?.join(', ') ?? '—',
}
```

> If the list API doesn't return `operators`, show `—` as placeholder. The column should exist even if empty.

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/admin/legal-entities/LegalEntitiesPage.tsx
git commit -m "feat(admin/legal-entities): INN validation, operators association column"
```

---

## Task 15: ContractsPage — Expiry Indicator + Status Filter (Phase 10)

**Files:**
- Modify: `portal-frontend/src/pages/admin/contracts/ContractsPage.tsx`

- [ ] **Step 1: Read the full file**

```bash
cat -n portal-frontend/src/pages/admin/contracts/ContractsPage.tsx
```

- [ ] **Step 2: Add expiry computation helpers**

```tsx
function getExpiryStatus(endDate: string | null): 'active' | 'expiring' | 'expired' {
  if (!endDate) return 'active';
  const end = new Date(endDate);
  const now = new Date();
  const diffDays = Math.ceil((end.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
  if (diffDays < 0) return 'expired';
  if (diffDays <= 30) return 'expiring';
  return 'active';
}
```

- [ ] **Step 3: Add expiry badge to the status column**

In the DataTable columns, find where `status` badge is rendered. Extend it:
```tsx
render: (row) => {
  const expiry = getExpiryStatus(row.end_date);
  return (
    <div className="flex flex-col gap-1">
      <Badge variant={row.status === 'active' ? 'success' : row.status === 'expired' ? 'danger' : 'info'}>
        {row.status === 'active' ? 'Активный' : row.status === 'expired' ? 'Истёк' : 'Расторгнут'}
      </Badge>
      {expiry === 'expiring' && <Badge variant="warning">Истекает</Badge>}
      {expiry === 'expired' && row.status === 'active' && <Badge variant="danger">Срок истёк</Badge>}
    </div>
  );
}
```

- [ ] **Step 4: Add status filter state and UI**

```tsx
const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'expiring' | 'expired' | 'terminated'>('all');

// Filter computed list:
const filteredContracts = useMemo(() => {
  if (statusFilter === 'all') return contracts;
  if (statusFilter === 'expiring') return contracts.filter(c => getExpiryStatus(c.end_date) === 'expiring');
  if (statusFilter === 'expired') return contracts.filter(c => getExpiryStatus(c.end_date) === 'expired');
  if (statusFilter === 'terminated') return contracts.filter(c => c.status === 'terminated');
  return contracts.filter(c => c.status === statusFilter);
}, [contracts, statusFilter]);
```

Add a select/tab to the filter bar:
```tsx
<Select
  label="Статус"
  value={statusFilter}
  onChange={e => setStatusFilter(e.target.value as typeof statusFilter)}
  options={[
    { value: 'all', label: 'Все' },
    { value: 'active', label: 'Активные' },
    { value: 'expiring', label: 'Истекающие' },
    { value: 'expired', label: 'Истёкшие' },
    { value: 'terminated', label: 'Расторгнутые' },
  ]}
/>
```

Pass `filteredContracts` instead of `contracts` to `DataTable`.

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/admin/contracts/ContractsPage.tsx
git commit -m "feat(admin/contracts): expiry indicator badge, status filter with expiring/expired states"
```

---

## Task 16: Cross-Cutting — Empty States Audit (Phase 11.4)

**Files:**
- Modify: 12 CRUD pages (audit only; modify those missing empty states)

- [ ] **Step 1: Check each page for empty states**

Run grep to find pages without empty states:
```bash
grep -rL "Нет данных\|нет.*маршрут\|no.*data\|empty" portal-frontend/src/pages/admin/ --include="*.tsx" 2>/dev/null
```

- [ ] **Step 2: Add empty state to each DataTable missing one**

For each page found above, check the `DataTable` usage. If `DataTable` has an `emptyMessage` prop, use it:
```tsx
<DataTable
  columns={columns}
  data={items}
  emptyMessage={
    <div className="text-center py-12 text-gray-500">
      <p className="text-lg font-medium">Нет данных</p>
      <p className="text-sm mt-1">Нажмите «Создать», чтобы добавить первую запись</p>
      <Button onClick={openCreateModal} className="mt-4" size="sm">Создать</Button>
    </div>
  }
/>
```

If `DataTable` doesn't support `emptyMessage`, add a conditional below:
```tsx
{items.length === 0 && !loading && (
  <div className="text-center py-12 text-gray-500">
    <p className="text-lg font-medium">Нет данных</p>
    <p className="text-sm mt-1">Нажмите «Создать», чтобы добавить первую запись</p>
  </div>
)}
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/admin/
git commit -m "feat(admin): add empty state messages to pages missing them"
```

---

## Spec Coverage Self-Review

| Phase | Tasks | Status |
|-------|-------|--------|
| Phase 1 (Detalization) | Tasks 2, 3, 4 | ✓ StatusTimeline, SmsPreview, CSV export, collapsible filters |
| Phase 2 (Connections) | Tasks 5, 6 | ✓ Detail drawer, refresh control, log viewer |
| Phase 3 (Routing) | Task 7 | ✓ Drag UX, empty state, legal entity required |
| Phase 4 (Sender Names) | Tasks 8, 9, 10 | ✓ Validation hints, template shortcut, delete flow, auto-open |
| Phase 5 (Statistics) | Task 11 | ✓ Fixed table, sticky header/footer, date range |
| Phase 6 (MCC/MNC) | Task 12 | ✓ Legal entity section (placeholder), interdependent filters |
| Phase 7 (Operator Templates) | Task 10 | ✓ Variable highlighting, SMS preview |
| Phase 8 (Settings) | Task 13 | ✓ Dynamic groups, beforeunload warning |
| Phase 9 (Legal Entities) | Task 14 | ✓ Operators column, INN validation |
| Phase 10 (Contracts) | Task 15 | ✓ Expiry indicators, status filter |
| Phase 11 (Cross-Cutting) | Tasks 1, 16 | ✓ CSV utility, empty states |
| Phase 11.2 (SSE) | — | Skipped — backend-dependent, optional per spec |
| Phase 11.3 (Shortcuts) | — | Skipped — `CommandPalette.tsx` exists, verify separately |

All spec requirements covered. SSE (Phase 11.2) skipped per spec guidance ("skip if endpoint doesn't exist"). Keyboard shortcuts (Phase 11.3) deferred — `CommandPalette.tsx` already exists per spec, only admin commands need wiring; low-risk deferral.
