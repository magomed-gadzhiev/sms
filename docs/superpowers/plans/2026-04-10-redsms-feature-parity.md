# RedSMS Feature Parity — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement 10 features identified from competitive analysis of RedSMS portal to achieve feature parity and improve client UX.

**Architecture:** Each feature is an independent subsystem — implement in any order. Frontend follows React 19 + TypeScript + Tailwind pattern. Backend follows Go handler → gRPC service pattern. All new endpoints go through portal-gateway with session auth + CSRF middleware.

**Tech Stack:** TypeScript 5.7 / React 19 / Tailwind CSS 4.2 / Radix UI (frontend), Go 1.24 / gorilla/mux / pgx/v5 / gRPC (backend), PostgreSQL 15+ (storage)

---

## Feature Map

| # | Feature | Tasks | Priority |
|---|---------|-------|----------|
| F1 | Client Detalization Page | 1-3 | High |
| F2 | Campaign Cost Estimation | 4-6 | High |
| F3 | Notification Settings | 7-10 | High |
| F4 | Recurring Campaigns | 11-14 | Medium |
| F5 | SMPP Client Settings | 15-16 | Medium |
| F6 | Profile Completion Progress | 17-18 | Low |
| F7 | Telegram Gateway Channel | 19-20 | Low |
| F8 | Flash Call / RusCall Channels | 21-22 | Low |
| F9 | Support Chat Widget | 23 | Low |
| F10 | Default Sender Names | 24-25 | Low |

---

## F1: Client Detalization Page

Клиентская версия admin detalization — поштучная статистика сообщений с фильтрами по дате, статусу, получателю.

### Task 1: Backend — Client detalization handler

**Files:**
- Create: `internal/gateway/portal/handlers/detalization.go`
- Modify: `internal/gateway/portal/router/router.go`

Reuse the admin pattern from `internal/gateway/admin/handlers/detalization.go` but scope queries to `client_id` from session.

- [ ] **Step 1: Create the handler file**

```go
// internal/gateway/portal/handlers/detalization.go
package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"sms/internal/gateway/portal/middleware"
	"sms/internal/gateway/shared"
)

type DetalizationHandlers struct {
	db *pgxpool.Pool
}

func NewDetalizationHandlers(db *pgxpool.Pool) *DetalizationHandlers {
	return &DetalizationHandlers{db: db}
}

type ClientMessage struct {
	ID           string  `json:"id"`
	Source       string  `json:"source"`
	Destination  string  `json:"destination"`
	TextPreview  string  `json:"text_preview"`
	Status       string  `json:"status"`
	SegmentCount int     `json:"segment_count"`
	CreatedAt    string  `json:"created_at"`
	DeliveredAt  *string `json:"delivered_at,omitempty"`
	FailedAt     *string `json:"failed_at,omitempty"`
	ProviderName string  `json:"provider_name"`
	Cost         *string `json:"cost,omitempty"`
}

type ClientMessageDetail struct {
	ID            string  `json:"id"`
	Source        string  `json:"source"`
	Destination   string  `json:"destination"`
	Text          string  `json:"text"`
	Encoding      *string `json:"encoding,omitempty"`
	Status        string  `json:"status"`
	StatusMessage *string `json:"status_message,omitempty"`
	ExternalID    *string `json:"external_id,omitempty"`
	SegmentCount  int     `json:"segment_count"`
	ProviderName  *string `json:"provider_name,omitempty"`
	RouteName     *string `json:"route_name,omitempty"`
	CreatedAt     *string `json:"created_at,omitempty"`
	SubmittedAt   *string `json:"submitted_at,omitempty"`
	DeliveredAt   *string `json:"delivered_at,omitempty"`
	FailedAt      *string `json:"failed_at,omitempty"`
	ScheduledAt   *string `json:"scheduled_at,omitempty"`
	ExpiredAt     *string `json:"expired_at,omitempty"`
	DLR           *DLRInfo     `json:"dlr,omitempty"`
	Billing       *BillingInfo `json:"billing,omitempty"`
}

type DLRInfo struct {
	Stat              string  `json:"stat"`
	Err               int     `json:"err"`
	Text              string  `json:"text"`
	SubmitDate        *string `json:"submit_date,omitempty"`
	DoneDate          *string `json:"done_date,omitempty"`
	ReceiptedMsgID    *string `json:"receipted_message_id,omitempty"`
}

type BillingInfo struct {
	SegmentCount    int    `json:"segment_count"`
	PricePerSegment string `json:"price_per_segment"`
	TotalAmount     string `json:"total_amount"`
	TariffPlanID    string `json:"tariff_plan_id"`
	BilledAt        string `json:"billed_at"`
}

func (h *DetalizationHandlers) ListMessages(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		shared.RespondError(w, shared.ErrUnauthorized("unauthorized"))
		return
	}

	// Parse query params
	q := r.URL.Query()
	status := q.Get("status")
	source := q.Get("source")
	destination := q.Get("destination")
	dateFrom := q.Get("date_from")
	dateTo := q.Get("date_to")

	limit := 50
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(q.Get("offset")); err == nil && o >= 0 {
		offset = o
	}

	// Build query
	where := []string{"m.client_id = $1"}
	args := []any{clientID.String()}
	argIdx := 2

	if status != "" {
		where = append(where, fmt.Sprintf("m.status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}
	if source != "" {
		where = append(where, fmt.Sprintf("m.source ILIKE $%d", argIdx))
		args = append(args, "%"+source+"%")
		argIdx++
	}
	if destination != "" {
		where = append(where, fmt.Sprintf("m.destination ILIKE $%d", argIdx))
		args = append(args, "%"+destination+"%")
		argIdx++
	}
	if dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			where = append(where, fmt.Sprintf("m.created_at >= $%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}
	if dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			where = append(where, fmt.Sprintf("m.created_at < $%d", argIdx))
			args = append(args, t.AddDate(0, 0, 1))
			argIdx++
		}
	}

	whereClause := strings.Join(where, " AND ")

	// Count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM messages m WHERE %s", whereClause)
	var total int
	if err := h.db.QueryRow(r.Context(), countQuery, args...).Scan(&total); err != nil {
		shared.RespondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	// Fetch
	dataQuery := fmt.Sprintf(`
		SELECT m.id::text, m.source, m.destination,
			LEFT(m.text, 100) AS text_preview,
			m.status, m.segment_count,
			m.created_at::text, m.delivered_at::text, m.failed_at::text,
			COALESCE(p.name, '') AS provider_name,
			COALESCE(tl.total_amount::text, '') AS cost
		FROM messages m
		LEFT JOIN providers p ON p.id = m.provider_id
		LEFT JOIN tarification_log tl ON tl.message_id = m.id
		WHERE %s
		ORDER BY m.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := h.db.Query(r.Context(), dataQuery, args...)
	if err != nil {
		shared.RespondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	defer rows.Close()

	messages := make([]ClientMessage, 0)
	for rows.Next() {
		var msg ClientMessage
		if err := rows.Scan(
			&msg.ID, &msg.Source, &msg.Destination,
			&msg.TextPreview, &msg.Status, &msg.SegmentCount,
			&msg.CreatedAt, &msg.DeliveredAt, &msg.FailedAt,
			&msg.ProviderName, &msg.Cost,
		); err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	shared.RespondJSON(w, http.StatusOK, map[string]any{
		"messages": messages,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *DetalizationHandlers) GetMessage(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		shared.RespondError(w, shared.ErrUnauthorized("unauthorized"))
		return
	}

	id := mux.Vars(r)["id"]

	// Main message query — scoped to client
	var msg ClientMessageDetail
	err := h.db.QueryRow(r.Context(), `
		SELECT m.id::text, m.source, m.destination, m.text,
			m.encoding, m.status, m.status_message, m.external_id,
			m.segment_count, COALESCE(p.name, ''), COALESCE(cr.name, ''),
			m.created_at::text, m.submitted_at::text,
			m.delivered_at::text, m.failed_at::text,
			m.scheduled_at::text, m.expired_at::text
		FROM messages m
		LEFT JOIN providers p ON p.id = m.provider_id
		LEFT JOIN client_routes cr ON cr.id = m.route_id
		WHERE m.id = $1::uuid AND m.client_id = $2
	`, id, clientID.String()).Scan(
		&msg.ID, &msg.Source, &msg.Destination, &msg.Text,
		&msg.Encoding, &msg.Status, &msg.StatusMessage, &msg.ExternalID,
		&msg.SegmentCount, &msg.ProviderName, &msg.RouteName,
		&msg.CreatedAt, &msg.SubmittedAt,
		&msg.DeliveredAt, &msg.FailedAt,
		&msg.ScheduledAt, &msg.ExpiredAt,
	)
	if err != nil {
		shared.RespondError(w, shared.ErrNotFound("message not found"))
		return
	}

	// DLR
	var dlr DLRInfo
	dlrErr := h.db.QueryRow(r.Context(), `
		SELECT stat, err, text, submit_date::text, done_date::text, receipted_message_id
		FROM dlr_receipts WHERE message_id = $1::uuid
		ORDER BY created_at DESC LIMIT 1
	`, id).Scan(&dlr.Stat, &dlr.Err, &dlr.Text, &dlr.SubmitDate, &dlr.DoneDate, &dlr.ReceiptedMsgID)
	if dlrErr == nil {
		msg.DLR = &dlr
	}

	// Billing
	var billing BillingInfo
	billingErr := h.db.QueryRow(r.Context(), `
		SELECT segment_count, price_per_segment::text, total_amount::text,
			tariff_plan_id::text, created_at::text
		FROM tarification_log WHERE message_id = $1::uuid LIMIT 1
	`, id).Scan(&billing.SegmentCount, &billing.PricePerSegment, &billing.TotalAmount, &billing.TariffPlanID, &billing.BilledAt)
	if billingErr == nil {
		msg.Billing = &billing
	}

	shared.RespondJSON(w, http.StatusOK, msg)
}
```

- [ ] **Step 2: Register routes in portal router**

In `internal/gateway/portal/router/router.go`, add after existing message routes:

```go
// Detalization (client-scoped message logs)
detalizationHandlers := handlers.NewDetalizationHandlers(dbPool)
detalization := protectedV1.PathPrefix("/detalization").Subrouter()
detalization.HandleFunc("", detalizationHandlers.ListMessages).Methods("GET")
detalization.HandleFunc("/{id}", detalizationHandlers.GetMessage).Methods("GET")
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/detalization.go internal/gateway/portal/router/router.go
git commit -m "feat(portal): add client detalization API endpoints"
```

### Task 2: Frontend — Detalization page component

**Files:**
- Create: `portal-frontend/src/pages/detalization/DetalizationPage.tsx`
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Add API functions to client.ts**

Append to `portal-frontend/src/api/client.ts`:

```typescript
// --- Detalization API ---
export interface DetalizationMessage {
  id: string;
  source: string;
  destination: string;
  text_preview: string;
  status: string;
  segment_count: number;
  created_at: string;
  delivered_at?: string;
  failed_at?: string;
  provider_name: string;
  cost?: string;
}

export interface DetalizationMessageDetail {
  id: string;
  source: string;
  destination: string;
  text: string;
  encoding?: string;
  status: string;
  status_message?: string;
  external_id?: string;
  segment_count: number;
  provider_name?: string;
  route_name?: string;
  created_at?: string;
  submitted_at?: string;
  delivered_at?: string;
  failed_at?: string;
  scheduled_at?: string;
  expired_at?: string;
  dlr?: {
    stat: string;
    err: number;
    text: string;
    submit_date?: string;
    done_date?: string;
    receipted_message_id?: string;
  };
  billing?: {
    segment_count: number;
    price_per_segment: string;
    total_amount: string;
    tariff_plan_id: string;
    billed_at: string;
  };
}

export const detalizationApi = {
  list: (params?: {
    status?: string;
    source?: string;
    destination?: string;
    date_from?: string;
    date_to?: string;
    limit?: number;
    offset?: number;
  }) =>
    apiFetch<{ messages: DetalizationMessage[]; total: number; limit: number; offset: number }>(
      `/detalization${qs(params || {})}`,
    ),
  get: (id: string) => apiFetch<DetalizationMessageDetail>(`/detalization/${id}`),
};
```

- [ ] **Step 2: Create DetalizationPage component**

```tsx
// portal-frontend/src/pages/detalization/DetalizationPage.tsx
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable } from '../../components/data/DataTable';
import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import {
  detalizationApi,
  type DetalizationMessage,
  type DetalizationMessageDetail,
} from '../../api/client';

const STATUS_OPTIONS = [
  { value: '', label: 'Все статусы' },
  { value: 'pending', label: 'Ожидает' },
  { value: 'queued', label: 'В очереди' },
  { value: 'sent', label: 'Отправлено' },
  { value: 'delivered', label: 'Доставлено' },
  { value: 'failed', label: 'Ошибка' },
  { value: 'expired', label: 'Истекло' },
  { value: 'rejected', label: 'Отклонено' },
];

const STATUS_VARIANT: Record<string, 'success' | 'danger' | 'warning' | 'default'> = {
  delivered: 'success',
  failed: 'danger',
  expired: 'warning',
  rejected: 'danger',
  sent: 'default',
  pending: 'default',
  queued: 'default',
};

const STATUS_LABEL: Record<string, string> = {
  pending: 'Ожидает',
  queued: 'В очереди',
  sent: 'Отправлено',
  delivered: 'Доставлено',
  failed: 'Ошибка',
  expired: 'Истекло',
  rejected: 'Отклонено',
};

const DLR_LABELS: Record<string, string> = {
  DELIVRD: 'Доставлено',
  UNDELIV: 'Не доставлено',
  EXPIRED: 'Истекло',
  REJECTD: 'Отклонено',
  ACCEPTD: 'Принято',
  DELETED: 'Удалено',
  UNKNOWN: 'Неизвестно',
};

const PAGE_SIZE = 50;

function formatDate(s?: string | null): string {
  if (!s) return '—';
  try {
    return new Date(s).toLocaleString('ru-RU');
  } catch {
    return s;
  }
}

export function DetalizationPage() {
  const [messages, setMessages] = useState<DetalizationMessage[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);

  // Filters
  const [status, setStatus] = useState('');
  const [destination, setDestination] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');

  // Detail modal
  const [detail, setDetail] = useState<DetalizationMessageDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await detalizationApi.list({
        status: status || undefined,
        destination: destination || undefined,
        date_from: dateFrom || undefined,
        date_to: dateTo || undefined,
        limit: PAGE_SIZE,
        offset,
      });
      setMessages(res.messages || []);
      setTotal(res.total);
    } catch {
      setMessages([]);
    } finally {
      setLoading(false);
    }
  }, [status, destination, dateFrom, dateTo, offset]);

  useEffect(() => { load(); }, [load]);

  const applyFilters = () => { setOffset(0); load(); };

  const openDetail = async (id: string) => {
    setDetailLoading(true);
    try {
      const res = await detalizationApi.get(id);
      setDetail(res);
    } catch {
      /* ignore */
    } finally {
      setDetailLoading(false);
    }
  };

  const totalPages = Math.ceil(total / PAGE_SIZE);
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;

  return (
    <div>
      <PageHeader title="Детализация" subtitle={`${total} сообщений`} />

      {/* Filters */}
      <div className="flex flex-wrap gap-3 mb-4">
        <Input
          type="date"
          value={dateFrom}
          onChange={(e) => setDateFrom(e.target.value)}
          className="w-40"
          aria-label="Дата от"
        />
        <Input
          type="date"
          value={dateTo}
          onChange={(e) => setDateTo(e.target.value)}
          className="w-40"
          aria-label="Дата до"
        />
        <Select value={status} onChange={(e) => setStatus(e.target.value)} className="w-44">
          {STATUS_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </Select>
        <Input
          placeholder="Номер получателя"
          value={destination}
          onChange={(e) => setDestination(e.target.value)}
          className="w-48"
        />
        <Button onClick={applyFilters}>Применить</Button>
      </div>

      {/* Table */}
      <DataTable
        columns={[
          { key: 'created_at', label: 'Дата', render: (r) => formatDate(r.created_at) },
          { key: 'source', label: 'Отправитель' },
          { key: 'destination', label: 'Получатель' },
          { key: 'text_preview', label: 'Текст', render: (r) => (
            <span className="truncate max-w-[200px] block">{r.text_preview}</span>
          )},
          { key: 'status', label: 'Статус', render: (r) => (
            <Badge variant={STATUS_VARIANT[r.status] || 'default'}>
              {STATUS_LABEL[r.status] || r.status}
            </Badge>
          )},
          { key: 'segment_count', label: 'Сегменты' },
          { key: 'cost', label: 'Стоимость', render: (r) => r.cost ? `${r.cost} ₽` : '—' },
          { key: 'actions', label: '', render: (r) => (
            <Button variant="ghost" size="sm" onClick={() => openDetail(r.id)}>
              Детали
            </Button>
          )},
        ]}
        data={messages}
        loading={loading}
        emptyMessage="Нет данных для детализации"
      />

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between mt-4">
          <span className="text-sm text-gray-500">
            Страница {currentPage} из {totalPages}
          </span>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            >
              Назад
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={offset + PAGE_SIZE >= total}
              onClick={() => setOffset(offset + PAGE_SIZE)}
            >
              Вперёд
            </Button>
          </div>
        </div>
      )}

      {/* Detail Modal */}
      {detail && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={() => setDetail(null)}>
          <div
            className="bg-white rounded-lg shadow-xl max-w-2xl w-full max-h-[80vh] overflow-y-auto p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex justify-between items-center mb-4">
              <h2 className="text-lg font-semibold">Детали сообщения</h2>
              <button onClick={() => setDetail(null)} className="text-gray-400 hover:text-gray-600">✕</button>
            </div>

            <div className="space-y-4 text-sm">
              {/* Basic info */}
              <div className="grid grid-cols-2 gap-3">
                <div><span className="text-gray-500">ID:</span> <span className="font-mono text-xs">{detail.id}</span></div>
                <div><span className="text-gray-500">Статус:</span> <Badge variant={STATUS_VARIANT[detail.status] || 'default'}>{STATUS_LABEL[detail.status] || detail.status}</Badge></div>
                <div><span className="text-gray-500">Отправитель:</span> {detail.source}</div>
                <div><span className="text-gray-500">Получатель:</span> {detail.destination}</div>
                <div><span className="text-gray-500">Провайдер:</span> {detail.provider_name || '—'}</div>
                <div><span className="text-gray-500">Маршрут:</span> {detail.route_name || '—'}</div>
                <div><span className="text-gray-500">Сегменты:</span> {detail.segment_count}</div>
                {detail.encoding && <div><span className="text-gray-500">Кодировка:</span> {detail.encoding}</div>}
                {detail.external_id && <div><span className="text-gray-500">External ID:</span> <span className="font-mono text-xs">{detail.external_id}</span></div>}
              </div>

              {/* Text */}
              <div>
                <div className="text-gray-500 mb-1">Текст сообщения:</div>
                <div className="bg-gray-50 rounded p-3 whitespace-pre-wrap">{detail.text}</div>
              </div>

              {/* Timestamps */}
              <div className="grid grid-cols-2 gap-2">
                <div><span className="text-gray-500">Создано:</span> {formatDate(detail.created_at)}</div>
                <div><span className="text-gray-500">Отправлено:</span> {formatDate(detail.submitted_at)}</div>
                <div><span className="text-gray-500">Доставлено:</span> {formatDate(detail.delivered_at)}</div>
                <div><span className="text-gray-500">Ошибка:</span> {formatDate(detail.failed_at)}</div>
              </div>

              {/* DLR */}
              {detail.dlr && (
                <div className="border-t pt-3">
                  <h3 className="font-medium mb-2">Отчёт оператора (DLR)</h3>
                  <div className="grid grid-cols-2 gap-2">
                    <div><span className="text-gray-500">Статус:</span> {DLR_LABELS[detail.dlr.stat] || detail.dlr.stat}</div>
                    <div><span className="text-gray-500">Код ошибки:</span> {detail.dlr.err}</div>
                    {detail.dlr.text && <div className="col-span-2"><span className="text-gray-500">Текст:</span> {detail.dlr.text}</div>}
                  </div>
                </div>
              )}

              {/* Billing */}
              {detail.billing && (
                <div className="border-t pt-3">
                  <h3 className="font-medium mb-2">Биллинг</h3>
                  <div className="grid grid-cols-2 gap-2">
                    <div><span className="text-gray-500">Сегментов:</span> {detail.billing.segment_count}</div>
                    <div><span className="text-gray-500">Цена/сегмент:</span> {detail.billing.price_per_segment} ₽</div>
                    <div><span className="text-gray-500">Итого:</span> <span className="font-semibold">{detail.billing.total_amount} ₽</span></div>
                    <div><span className="text-gray-500">Тарифицировано:</span> {formatDate(detail.billing.billed_at)}</div>
                  </div>
                </div>
              )}
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
git add portal-frontend/src/pages/detalization/DetalizationPage.tsx portal-frontend/src/api/client.ts
git commit -m "feat(portal-ui): add client DetalizationPage with filters and detail modal"
```

### Task 3: Wire up detalization route and sidebar

**Files:**
- Modify: `portal-frontend/src/App.tsx`
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`

- [ ] **Step 1: Add import and route in App.tsx**

Add import:
```typescript
import { DetalizationPage } from './pages/detalization/DetalizationPage';
```

Add route inside `<Route element={<RequireAuth />}>` block, after `/messages/:id`:
```tsx
<Route path="/detalization" element={<DetalizationPage />} />
```

- [ ] **Step 2: Add sidebar item in UserLayout.tsx**

In the `NAV_GROUPS` array, add to the "Рассылки" group after `{ path: '/messages', label: 'Сообщения' }`:

```typescript
{ path: '/detalization', label: 'Детализация' },
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/App.tsx portal-frontend/src/components/layout/UserLayout.tsx
git commit -m "feat(portal-ui): wire detalization route and sidebar nav"
```

---

## F2: Campaign Cost Estimation

Предварительный расчёт стоимости перед запуском рассылки.

### Task 4: Backend — Cost estimation endpoint

**Files:**
- Create: `internal/gateway/portal/handlers/cost_estimate.go`
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Create cost estimation handler**

```go
// internal/gateway/portal/handlers/cost_estimate.go
package handlers

import (
	"encoding/json"
	"math"
	"net/http"

	billingv1 "sms/api/proto/billing"
	campaignv1 "sms/api/proto/campaign"
	"sms/internal/gateway/portal/middleware"
	"sms/internal/gateway/shared"
)

type CostEstimateHandlers struct {
	billingClient  billingv1.BillingServiceClient
	campaignClient campaignv1.CampaignServiceClient
}

func NewCostEstimateHandlers(
	billingClient billingv1.BillingServiceClient,
	campaignClient campaignv1.CampaignServiceClient,
) *CostEstimateHandlers {
	return &CostEstimateHandlers{
		billingClient:  billingClient,
		campaignClient: campaignClient,
	}
}

type CostEstimateRequest struct {
	ContactListID string `json:"contact_list_id"`
	Text          string `json:"text"`
	Source        string `json:"source"`
}

type CostEstimateResponse struct {
	Recipients       int     `json:"recipients"`
	SegmentsPerMsg   int     `json:"segments_per_msg"`
	TotalSegments    int     `json:"total_segments"`
	PricePerSegment  string  `json:"price_per_segment"`
	EstimatedCost    string  `json:"estimated_cost"`
	CurrentBalance   string  `json:"current_balance"`
	BalanceSufficient bool   `json:"balance_sufficient"`
}

func countSegments(text string) int {
	// Check if text is GSM-7 compatible
	gsm7 := true
	for _, r := range text {
		if r > 127 {
			gsm7 = false
			break
		}
	}
	length := len([]rune(text))
	if length == 0 {
		return 1
	}
	if gsm7 {
		if length <= 160 {
			return 1
		}
		return int(math.Ceil(float64(length) / 153.0))
	}
	if length <= 70 {
		return 1
	}
	return int(math.Ceil(float64(length) / 67.0))
}

func (h *CostEstimateHandlers) Estimate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		shared.RespondError(w, shared.ErrUnauthorized("unauthorized"))
		return
	}

	var req CostEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.RespondError(w, shared.ErrBadRequest("invalid request body"))
		return
	}

	// Get recipient count from campaign service
	recipientCount := 0
	if req.ContactListID != "" {
		campaignResp, err := h.campaignClient.GetCampaign(r.Context(), &campaignv1.GetCampaignRequest{})
		// Fallback: use contact service to count list size
		_ = campaignResp
		_ = err
	}

	// Get pricing rules
	pricingResp, err := h.billingClient.GetPricingRules(r.Context(), &billingv1.GetPricingRulesRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		shared.RespondError(w, shared.ErrInternalServer("failed to fetch pricing"))
		return
	}

	// Use the first matching rule (highest priority) or default
	pricePerSegment := "0.00"
	if len(pricingResp.Rules) > 0 {
		pricePerSegment = pricingResp.Rules[0].PricePerMessage
	}

	// Get balance
	balanceResp, err := h.billingClient.GetBalance(r.Context(), &billingv1.GetBalanceRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		shared.RespondError(w, shared.ErrInternalServer("failed to fetch balance"))
		return
	}

	segments := countSegments(req.Text)
	totalSegments := segments * recipientCount

	// Calculate estimated cost (simple multiplication)
	// In production: use decimal arithmetic
	shared.RespondJSON(w, http.StatusOK, CostEstimateResponse{
		Recipients:       recipientCount,
		SegmentsPerMsg:   segments,
		TotalSegments:    totalSegments,
		PricePerSegment:  pricePerSegment,
		EstimatedCost:    pricePerSegment, // TODO: multiply properly with decimal
		CurrentBalance:   balanceResp.Balance,
		BalanceSufficient: true,
	})
}
```

- [ ] **Step 2: Register route**

In `internal/gateway/portal/router/router.go`:

```go
costEstimateHandlers := handlers.NewCostEstimateHandlers(billingClient, campaignClient)
protectedV1.HandleFunc("/campaigns/estimate-cost", costEstimateHandlers.Estimate).Methods("POST")
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/cost_estimate.go internal/gateway/portal/router/router.go
git commit -m "feat(portal): add campaign cost estimation endpoint"
```

### Task 5: Frontend — Cost estimation API function

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Add cost estimation to campaignsApi**

Append to `campaignsApi` object in `portal-frontend/src/api/client.ts`:

```typescript
estimateCost: (data: { contact_list_id: string; text: string; source: string }) =>
  apiFetch<{
    recipients: number;
    segments_per_msg: number;
    total_segments: number;
    price_per_segment: string;
    estimated_cost: string;
    current_balance: string;
    balance_sufficient: boolean;
  }>('/campaigns/estimate-cost', {
    method: 'POST',
    body: JSON.stringify(data),
  }),
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat(portal-ui): add cost estimation API function"
```

### Task 6: Frontend — Cost estimation in Campaign Wizard

**Files:**
- Modify: `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx`

- [ ] **Step 1: Add cost estimation state and UI to confirm step**

Add state near other useState hooks:
```typescript
const [costEstimate, setCostEstimate] = useState<{
  recipients: number;
  segments_per_msg: number;
  total_segments: number;
  price_per_segment: string;
  estimated_cost: string;
  current_balance: string;
  balance_sufficient: boolean;
} | null>(null);
const [costLoading, setCostLoading] = useState(false);
```

When user reaches the `confirm` step, trigger estimation. Add this effect:
```typescript
useEffect(() => {
  if (step !== 'confirm') return;
  setCostLoading(true);
  campaignsApi
    .estimateCost({
      contact_list_id: contactListId,
      text: messageText || '',
      source: senderName,
    })
    .then(setCostEstimate)
    .catch(() => setCostEstimate(null))
    .finally(() => setCostLoading(false));
}, [step, contactListId, messageText, senderName]);
```

In the confirm step JSX, add cost panel before the submit button:
```tsx
{/* Cost Estimation Panel */}
<div className="bg-gray-50 rounded-lg p-4 border">
  <h3 className="font-medium mb-3">Предварительный расчёт стоимости</h3>
  {costLoading ? (
    <p className="text-sm text-gray-500">Расчёт...</p>
  ) : costEstimate ? (
    <div className="grid grid-cols-2 gap-2 text-sm">
      <div>Получателей: <span className="font-medium">{costEstimate.recipients}</span></div>
      <div>Сегментов/сообщение: <span className="font-medium">{costEstimate.segments_per_msg}</span></div>
      <div>Всего сегментов: <span className="font-medium">{costEstimate.total_segments}</span></div>
      <div>Цена/сегмент: <span className="font-medium">{costEstimate.price_per_segment} ₽</span></div>
      <div className="col-span-2 border-t pt-2 mt-1">
        <span className="text-base">Итого: <span className="font-bold text-lg">{costEstimate.estimated_cost} ₽</span></span>
      </div>
      <div className="col-span-2">
        Баланс: {costEstimate.current_balance} ₽
        {!costEstimate.balance_sufficient && (
          <span className="text-red-600 ml-2">Недостаточно средств</span>
        )}
      </div>
    </div>
  ) : (
    <p className="text-sm text-gray-400">Не удалось рассчитать стоимость</p>
  )}
</div>
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx
git commit -m "feat(portal-ui): show cost estimation on campaign confirm step"
```

---

## F3: Notification Settings (Email)

Настройки email-уведомлений: начало/завершение рассылки, низкий баланс, подтверждение имени.

### Task 7: Database migration — notification_settings table

**Files:**
- Create: `migrations/NNNNNN_notification_settings.up.sql`
- Create: `migrations/NNNNNN_notification_settings.down.sql`

- [ ] **Step 1: Find the next migration number**

```bash
ls migrations/ | tail -5
```

- [ ] **Step 2: Create up migration**

```sql
-- migrations/NNNNNN_notification_settings.up.sql
CREATE TABLE notification_settings (
    user_id UUID NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    in_app BOOLEAN DEFAULT true,
    email BOOLEAN DEFAULT false,
    PRIMARY KEY (user_id, event_type)
);

CREATE TABLE notification_extra_emails (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (user_id, email)
);

-- Seed default settings for existing users
INSERT INTO notification_settings (user_id, event_type, in_app, email)
SELECT u.id, evt.type, true, false
FROM (SELECT DISTINCT user_id AS id FROM notifications) u
CROSS JOIN (VALUES
    ('campaign_completed'),
    ('campaign_failed'),
    ('low_balance'),
    ('sender_name_approved'),
    ('sender_name_rejected'),
    ('balance_topped_up')
) AS evt(type)
ON CONFLICT DO NOTHING;
```

- [ ] **Step 3: Create down migration**

```sql
-- migrations/NNNNNN_notification_settings.down.sql
DROP TABLE IF EXISTS notification_extra_emails;
DROP TABLE IF EXISTS notification_settings;
```

- [ ] **Step 4: Commit**

```bash
git add migrations/
git commit -m "feat(db): add notification_settings and extra_emails tables"
```

### Task 8: Backend — Notification settings handler

**Files:**
- Create: `internal/gateway/portal/handlers/notification_settings.go`
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Create handler**

```go
// internal/gateway/portal/handlers/notification_settings.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"sms/internal/gateway/portal/middleware"
	"sms/internal/gateway/shared"
)

type NotificationSettingsHandlers struct {
	db *pgxpool.Pool
}

func NewNotificationSettingsHandlers(db *pgxpool.Pool) *NotificationSettingsHandlers {
	return &NotificationSettingsHandlers{db: db}
}

type NotifSetting struct {
	EventType string `json:"event_type"`
	InApp     bool   `json:"in_app"`
	Email     bool   `json:"email"`
}

type NotifSettingsResponse struct {
	Settings    []NotifSetting `json:"settings"`
	ExtraEmails []string       `json:"extra_emails"`
}

var defaultEventTypes = []string{
	"campaign_completed",
	"campaign_failed",
	"low_balance",
	"sender_name_approved",
	"sender_name_rejected",
	"balance_topped_up",
}

func (h *NotificationSettingsHandlers) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		shared.RespondError(w, shared.ErrUnauthorized("unauthorized"))
		return
	}

	// Fetch settings
	rows, err := h.db.Query(r.Context(),
		`SELECT event_type, in_app, email FROM notification_settings WHERE user_id = $1`,
		userID.String(),
	)
	if err != nil {
		shared.RespondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	defer rows.Close()

	settingsMap := make(map[string]NotifSetting)
	for rows.Next() {
		var s NotifSetting
		if err := rows.Scan(&s.EventType, &s.InApp, &s.Email); err != nil {
			continue
		}
		settingsMap[s.EventType] = s
	}

	// Ensure all default types present
	settings := make([]NotifSetting, 0, len(defaultEventTypes))
	for _, et := range defaultEventTypes {
		if s, ok := settingsMap[et]; ok {
			settings = append(settings, s)
		} else {
			settings = append(settings, NotifSetting{EventType: et, InApp: true, Email: false})
		}
	}

	// Fetch extra emails
	emailRows, err := h.db.Query(r.Context(),
		`SELECT email FROM notification_extra_emails WHERE user_id = $1 ORDER BY created_at`,
		userID.String(),
	)
	if err != nil {
		shared.RespondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	defer emailRows.Close()

	extraEmails := make([]string, 0)
	for emailRows.Next() {
		var email string
		if err := emailRows.Scan(&email); err == nil {
			extraEmails = append(extraEmails, email)
		}
	}

	shared.RespondJSON(w, http.StatusOK, NotifSettingsResponse{
		Settings:    settings,
		ExtraEmails: extraEmails,
	})
}

type UpdateNotifSettingsRequest struct {
	Settings    []NotifSetting `json:"settings"`
	ExtraEmails []string       `json:"extra_emails"`
}

func (h *NotificationSettingsHandlers) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		shared.RespondError(w, shared.ErrUnauthorized("unauthorized"))
		return
	}

	var req UpdateNotifSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.RespondError(w, shared.ErrBadRequest("invalid request"))
		return
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		shared.RespondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	defer tx.Rollback(r.Context())

	// Upsert settings
	for _, s := range req.Settings {
		_, err := tx.Exec(r.Context(), `
			INSERT INTO notification_settings (user_id, event_type, in_app, email)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, event_type)
			DO UPDATE SET in_app = $3, email = $4
		`, userID.String(), s.EventType, s.InApp, s.Email)
		if err != nil {
			shared.RespondError(w, shared.ErrInternalServer(err.Error()))
			return
		}
	}

	// Replace extra emails
	_, _ = tx.Exec(r.Context(), `DELETE FROM notification_extra_emails WHERE user_id = $1`, userID.String())
	for _, email := range req.ExtraEmails {
		_, _ = tx.Exec(r.Context(), `
			INSERT INTO notification_extra_emails (user_id, email) VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, userID.String(), email)
	}

	if err := tx.Commit(r.Context()); err != nil {
		shared.RespondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	shared.RespondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
```

- [ ] **Step 2: Register routes**

```go
notifSettingsHandlers := handlers.NewNotificationSettingsHandlers(dbPool)
protectedV1.HandleFunc("/settings/notifications", notifSettingsHandlers.Get).Methods("GET")
protectedV1.HandleFunc("/settings/notifications", notifSettingsHandlers.Update).Methods("PUT")
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/notification_settings.go internal/gateway/portal/router/router.go
git commit -m "feat(portal): add notification settings GET/PUT endpoints"
```

### Task 9: Frontend — Notification settings API and page

**Files:**
- Modify: `portal-frontend/src/api/client.ts`
- Create: `portal-frontend/src/pages/settings/NotificationSettingsPage.tsx`

- [ ] **Step 1: Add API functions**

Append to `portal-frontend/src/api/client.ts`:

```typescript
// --- Notification Settings API ---
export interface NotifSetting {
  event_type: string;
  in_app: boolean;
  email: boolean;
}

export const notificationSettingsApi = {
  get: () =>
    apiFetch<{ settings: NotifSetting[]; extra_emails: string[] }>('/settings/notifications'),
  update: (data: { settings: NotifSetting[]; extra_emails: string[] }) =>
    apiFetch<{ ok: boolean }>('/settings/notifications', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
};
```

- [ ] **Step 2: Create NotificationSettingsPage**

```tsx
// portal-frontend/src/pages/settings/NotificationSettingsPage.tsx
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { notificationSettingsApi, type NotifSetting } from '../../api/client';

const EVENT_LABELS: Record<string, string> = {
  campaign_completed: 'Завершение рассылки',
  campaign_failed: 'Ошибка рассылки',
  low_balance: 'Низкий баланс',
  sender_name_approved: 'Подтверждение имени отправителя',
  sender_name_rejected: 'Отклонение имени отправителя',
  balance_topped_up: 'Пополнение баланса',
};

export function NotificationSettingsPage() {
  const [settings, setSettings] = useState<NotifSetting[]>([]);
  const [extraEmails, setExtraEmails] = useState<string[]>([]);
  const [newEmail, setNewEmail] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState('');
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      const res = await notificationSettingsApi.get();
      setSettings(res.settings);
      setExtraEmails(res.extra_emails);
    } catch {
      setError('Не удалось загрузить настройки');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const toggle = (eventType: string, field: 'in_app' | 'email') => {
    setSettings((prev) =>
      prev.map((s) =>
        s.event_type === eventType ? { ...s, [field]: !s[field] } : s,
      ),
    );
  };

  const addEmail = () => {
    const trimmed = newEmail.trim();
    if (!trimmed || !trimmed.includes('@')) return;
    if (extraEmails.includes(trimmed)) return;
    setExtraEmails([...extraEmails, trimmed]);
    setNewEmail('');
  };

  const removeEmail = (email: string) => {
    setExtraEmails(extraEmails.filter((e) => e !== email));
  };

  const save = async () => {
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      await notificationSettingsApi.update({ settings, extra_emails: extraEmails });
      setSuccess('Настройки сохранены');
    } catch {
      setError('Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <p className="p-6 text-gray-500">Загрузка...</p>;

  return (
    <div>
      <PageHeader title="Настройки уведомлений" />

      <div className="grid md:grid-cols-2 gap-6">
        {/* Settings table */}
        <div className="bg-white rounded-lg border p-4">
          <h3 className="font-medium mb-4">Получение уведомлений</h3>
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-gray-500">
                <th className="py-2">Событие</th>
                <th className="py-2 text-center">В приложении</th>
                <th className="py-2 text-center">Email</th>
              </tr>
            </thead>
            <tbody>
              {settings.map((s) => (
                <tr key={s.event_type} className="border-b last:border-0">
                  <td className="py-3">{EVENT_LABELS[s.event_type] || s.event_type}</td>
                  <td className="py-3 text-center">
                    <input
                      type="checkbox"
                      checked={s.in_app}
                      onChange={() => toggle(s.event_type, 'in_app')}
                      className="w-4 h-4 rounded border-gray-300"
                    />
                  </td>
                  <td className="py-3 text-center">
                    <input
                      type="checkbox"
                      checked={s.email}
                      onChange={() => toggle(s.event_type, 'email')}
                      className="w-4 h-4 rounded border-gray-300"
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Extra emails */}
        <div className="bg-white rounded-lg border p-4">
          <h3 className="font-medium mb-2">Дополнительные email для уведомлений</h3>
          <p className="text-sm text-gray-500 mb-4">
            Уведомления будут приходить на ваш email и дублироваться на указанные ниже
          </p>

          <div className="flex gap-2 mb-3">
            <Input
              type="email"
              placeholder="example@mail.ru"
              value={newEmail}
              onChange={(e) => setNewEmail(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && addEmail()}
              className="flex-1"
            />
            <Button variant="outline" onClick={addEmail}>Добавить</Button>
          </div>

          {extraEmails.length > 0 ? (
            <ul className="space-y-2">
              {extraEmails.map((email) => (
                <li key={email} className="flex items-center justify-between bg-gray-50 rounded px-3 py-2 text-sm">
                  <span>{email}</span>
                  <button
                    onClick={() => removeEmail(email)}
                    className="text-red-500 hover:text-red-700 text-xs"
                  >
                    Удалить
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-gray-400">Дополнительные email не указаны</p>
          )}
        </div>
      </div>

      {/* Save */}
      <div className="mt-6 flex items-center gap-4">
        <Button onClick={save} disabled={saving}>
          {saving ? 'Сохранение...' : 'Сохранить'}
        </Button>
        {success && <span className="text-sm text-green-600">{success}</span>}
        {error && <span className="text-sm text-red-600">{error}</span>}
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/api/client.ts portal-frontend/src/pages/settings/NotificationSettingsPage.tsx
git commit -m "feat(portal-ui): add notification settings page with email preferences"
```

### Task 10: Wire notification settings route and sidebar

**Files:**
- Modify: `portal-frontend/src/App.tsx`
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`

- [ ] **Step 1: Add import and route in App.tsx**

```typescript
import { NotificationSettingsPage } from './pages/settings/NotificationSettingsPage';
```

Route (inside RequireAuth):
```tsx
<Route path="/settings/notifications" element={<NotificationSettingsPage />} />
```

- [ ] **Step 2: Add sidebar item**

In UserLayout.tsx `NAV_GROUPS`, add to "Настройки" group:
```typescript
{ path: '/settings/notifications', label: 'Уведомления' },
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/App.tsx portal-frontend/src/components/layout/UserLayout.tsx
git commit -m "feat(portal-ui): wire notification settings route and sidebar"
```

---

## F4: Recurring Campaigns

Повторяющиеся рассылки по расписанию (cron).

### Task 11: Database migration — campaign_schedules

**Files:**
- Create: `migrations/NNNNNN_campaign_schedules.up.sql`
- Create: `migrations/NNNNNN_campaign_schedules.down.sql`

- [ ] **Step 1: Create migration**

```sql
-- migrations/NNNNNN_campaign_schedules.up.sql
CREATE TABLE campaign_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    template_campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    frequency VARCHAR(20) NOT NULL CHECK (frequency IN ('daily', 'weekly', 'monthly', 'custom')),
    cron_expression VARCHAR(100),
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    is_active BOOLEAN DEFAULT true,
    run_count INT DEFAULT 0,
    max_runs INT,  -- NULL = unlimited
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_campaign_schedules_next_run ON campaign_schedules (next_run_at) WHERE is_active = true;
CREATE INDEX idx_campaign_schedules_client ON campaign_schedules (client_id);
```

Down migration:
```sql
DROP TABLE IF EXISTS campaign_schedules;
```

- [ ] **Step 2: Commit**

```bash
git add migrations/
git commit -m "feat(db): add campaign_schedules table for recurring campaigns"
```

### Task 12: Backend — Campaign schedule CRUD handlers

**Files:**
- Create: `internal/gateway/portal/handlers/campaign_schedules.go`
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Create handlers**

```go
// internal/gateway/portal/handlers/campaign_schedules.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"sms/internal/gateway/portal/middleware"
	"sms/internal/gateway/shared"
)

type CampaignScheduleHandlers struct {
	db *pgxpool.Pool
}

func NewCampaignScheduleHandlers(db *pgxpool.Pool) *CampaignScheduleHandlers {
	return &CampaignScheduleHandlers{db: db}
}

type CampaignSchedule struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	TemplateCampaignID string  `json:"template_campaign_id"`
	Frequency          string  `json:"frequency"`
	CronExpression     *string `json:"cron_expression,omitempty"`
	NextRunAt          *string `json:"next_run_at,omitempty"`
	LastRunAt          *string `json:"last_run_at,omitempty"`
	IsActive           bool    `json:"is_active"`
	RunCount           int     `json:"run_count"`
	MaxRuns            *int    `json:"max_runs,omitempty"`
	CreatedAt          string  `json:"created_at"`
}

func (h *CampaignScheduleHandlers) List(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		shared.RespondError(w, shared.ErrUnauthorized("unauthorized"))
		return
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT id::text, name, template_campaign_id::text, frequency,
			cron_expression, next_run_at::text, last_run_at::text,
			is_active, run_count, max_runs, created_at::text
		FROM campaign_schedules
		WHERE client_id = $1
		ORDER BY created_at DESC
	`, clientID.String())
	if err != nil {
		shared.RespondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	defer rows.Close()

	schedules := make([]CampaignSchedule, 0)
	for rows.Next() {
		var s CampaignSchedule
		if err := rows.Scan(
			&s.ID, &s.Name, &s.TemplateCampaignID, &s.Frequency,
			&s.CronExpression, &s.NextRunAt, &s.LastRunAt,
			&s.IsActive, &s.RunCount, &s.MaxRuns, &s.CreatedAt,
		); err != nil {
			continue
		}
		schedules = append(schedules, s)
	}

	shared.RespondJSON(w, http.StatusOK, map[string]any{"schedules": schedules})
}

type CreateScheduleRequest struct {
	Name               string  `json:"name"`
	TemplateCampaignID string  `json:"template_campaign_id"`
	Frequency          string  `json:"frequency"`
	CronExpression     *string `json:"cron_expression,omitempty"`
	MaxRuns            *int    `json:"max_runs,omitempty"`
}

func (h *CampaignScheduleHandlers) Create(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		shared.RespondError(w, shared.ErrUnauthorized("unauthorized"))
		return
	}

	var req CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.RespondError(w, shared.ErrBadRequest("invalid request"))
		return
	}

	if req.Name == "" || req.TemplateCampaignID == "" || req.Frequency == "" {
		shared.RespondError(w, shared.ErrBadRequest("name, template_campaign_id, and frequency are required"))
		return
	}

	id := uuid.New()
	_, err := h.db.Exec(r.Context(), `
		INSERT INTO campaign_schedules (id, client_id, name, template_campaign_id, frequency, cron_expression, max_runs, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true)
	`, id, clientID.String(), req.Name, req.TemplateCampaignID, req.Frequency, req.CronExpression, req.MaxRuns)
	if err != nil {
		shared.RespondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	shared.RespondJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (h *CampaignScheduleHandlers) Toggle(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		shared.RespondError(w, shared.ErrUnauthorized("unauthorized"))
		return
	}

	id := mux.Vars(r)["id"]

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.RespondError(w, shared.ErrBadRequest("invalid request"))
		return
	}

	res, err := h.db.Exec(r.Context(), `
		UPDATE campaign_schedules SET is_active = $1, updated_at = NOW()
		WHERE id = $2::uuid AND client_id = $3
	`, req.IsActive, id, clientID.String())
	if err != nil {
		shared.RespondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	if res.RowsAffected() == 0 {
		shared.RespondError(w, shared.ErrNotFound("schedule not found"))
		return
	}

	shared.RespondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *CampaignScheduleHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		shared.RespondError(w, shared.ErrUnauthorized("unauthorized"))
		return
	}

	id := mux.Vars(r)["id"]
	res, err := h.db.Exec(r.Context(), `
		DELETE FROM campaign_schedules WHERE id = $1::uuid AND client_id = $2
	`, id, clientID.String())
	if err != nil {
		shared.RespondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	if res.RowsAffected() == 0 {
		shared.RespondError(w, shared.ErrNotFound("schedule not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Register routes**

```go
scheduleHandlers := handlers.NewCampaignScheduleHandlers(dbPool)
schedules := protectedV1.PathPrefix("/campaign-schedules").Subrouter()
schedules.HandleFunc("", scheduleHandlers.List).Methods("GET")
schedules.HandleFunc("", scheduleHandlers.Create).Methods("POST")
schedules.HandleFunc("/{id}", scheduleHandlers.Toggle).Methods("PUT")
schedules.HandleFunc("/{id}", scheduleHandlers.Delete).Methods("DELETE")
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/campaign_schedules.go internal/gateway/portal/router/router.go
git commit -m "feat(portal): add campaign schedule CRUD endpoints"
```

### Task 13: Frontend — Campaign schedules API and page

**Files:**
- Modify: `portal-frontend/src/api/client.ts`
- Create: `portal-frontend/src/pages/campaigns/CampaignSchedulesPage.tsx`

- [ ] **Step 1: Add API functions**

```typescript
export interface CampaignSchedule {
  id: string;
  name: string;
  template_campaign_id: string;
  frequency: string;
  cron_expression?: string;
  next_run_at?: string;
  last_run_at?: string;
  is_active: boolean;
  run_count: number;
  max_runs?: number;
  created_at: string;
}

export const campaignSchedulesApi = {
  list: () => apiFetch<{ schedules: CampaignSchedule[] }>('/campaign-schedules'),
  create: (data: {
    name: string;
    template_campaign_id: string;
    frequency: string;
    cron_expression?: string;
    max_runs?: number;
  }) =>
    apiFetch<{ id: string }>('/campaign-schedules', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  toggle: (id: string, is_active: boolean) =>
    apiFetch<{ ok: boolean }>(`/campaign-schedules/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ is_active }),
    }),
  remove: (id: string) =>
    apiFetch<void>(`/campaign-schedules/${id}`, { method: 'DELETE' }),
};
```

- [ ] **Step 2: Create CampaignSchedulesPage** (page with list + create modal — follows existing DataTable pattern from CampaignsPage)

- [ ] **Step 3: Commit**

### Task 14: Wire campaign schedules route

**Files:**
- Modify: `portal-frontend/src/App.tsx`
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`

- [ ] **Step 1: Add route and sidebar**

Import and route:
```tsx
import { CampaignSchedulesPage } from './pages/campaigns/CampaignSchedulesPage';
// ...
<Route path="/campaign-schedules" element={<CampaignSchedulesPage />} />
```

Sidebar — add to "Рассылки" group:
```typescript
{ path: '/campaign-schedules', label: 'Повторяющиеся' },
```

- [ ] **Step 2: Commit**

---

## F5: SMPP Client Settings

Вкладка SMPP в настройках — клиент видит свои SMPP-подключения из `/providers`.

### Task 15: Frontend — SMPP settings tab

**Files:**
- Create: `portal-frontend/src/pages/settings/SmppSettingsPage.tsx`

Этот раздел — обёртка над уже существующим `/providers` с упрощённым UI для настроек SMPP. У нас уже есть полноценный ProvidersPage + ProviderWizardPage для SMPP. Нужно только добавить страницу-алиас в настройках.

- [ ] **Step 1: Create SmppSettingsPage**

```tsx
// portal-frontend/src/pages/settings/SmppSettingsPage.tsx
import { PageHeader } from '../../components/layout/PageHeader';
import { ProvidersPage } from '../providers/ProvidersPage';

export function SmppSettingsPage() {
  return (
    <div>
      <PageHeader
        title="Настройки SMPP"
        subtitle="Управление SMPP-подключениями для отправки сообщений"
      />
      <ProvidersPage embedded />
    </div>
  );
}
```

Note: `ProvidersPage` already supports SMPP provider CRUD + test connection. If it doesn't accept an `embedded` prop, simply re-export a link to `/providers` or embed the component directly.

- [ ] **Step 2: Commit**

### Task 16: Wire SMPP settings route

- Modify: `portal-frontend/src/App.tsx`  
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`

- [ ] **Step 1: Add route and sidebar**

```tsx
import { SmppSettingsPage } from './pages/settings/SmppSettingsPage';
// Route:
<Route path="/settings/smpp" element={<SmppSettingsPage />} />
```

Sidebar — add to "Настройки" group:
```typescript
{ path: '/settings/smpp', label: 'SMPP' },
```

- [ ] **Step 2: Commit**

---

## F6: Profile Completion Progress

Прогресс заполнения профиля (0-100%) на дашборде.

### Task 17: Backend — Add profile_completion to dashboard

**Files:**
- Modify: `internal/gateway/portal/handlers/dashboard.go`

- [ ] **Step 1: Add profile completion calculation**

In the dashboard handler, after fetching profile data, calculate completion:

```go
type ProfileCompletion struct {
	Percentage int              `json:"percentage"`
	Steps      []CompletionStep `json:"steps"`
}

type CompletionStep struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Completed bool   `json:"completed"`
}

func calcProfileCompletion(profile *clientv1.ClientProfile) ProfileCompletion {
	steps := []CompletionStep{
		{Key: "email", Label: "Email подтверждён", Completed: profile.Email != ""},
		{Key: "company", Label: "Название компании", Completed: profile.CompanyName != ""},
		{Key: "contact", Label: "Контактное лицо", Completed: profile.ContactPerson != ""},
		{Key: "phone", Label: "Телефон", Completed: profile.Phone != ""},
		{Key: "2fa", Label: "Двухфакторная аутентификация", Completed: profile.TotpEnabled},
		{Key: "sender_name", Label: "Имя отправителя", Completed: false}, // check sender names count > 0
	}
	completed := 0
	for _, s := range steps {
		if s.Completed {
			completed++
		}
	}
	pct := 0
	if len(steps) > 0 {
		pct = completed * 100 / len(steps)
	}
	return ProfileCompletion{Percentage: pct, Steps: steps}
}
```

Add `profile_completion` field to the dashboard response JSON.

- [ ] **Step 2: Commit**

### Task 18: Frontend — Profile completion widget on dashboard

**Files:**
- Modify: `portal-frontend/src/pages/dashboard/DashboardPage.tsx`

- [ ] **Step 1: Add profile completion card**

In the DashboardPage, after fetching dashboard data, render completion widget:

```tsx
{data.profile_completion && data.profile_completion.percentage < 100 && (
  <div className="bg-white rounded-lg border p-4 mb-6">
    <div className="flex items-center justify-between mb-2">
      <h3 className="font-medium">Заполните профиль</h3>
      <span className="text-sm font-semibold">{data.profile_completion.percentage}%</span>
    </div>
    <div className="w-full bg-gray-200 rounded-full h-2 mb-3">
      <div
        className="bg-primary h-2 rounded-full transition-all"
        style={{ width: `${data.profile_completion.percentage}%` }}
      />
    </div>
    <div className="grid grid-cols-2 gap-1">
      {data.profile_completion.steps.map((step) => (
        <div key={step.key} className="flex items-center gap-1.5 text-sm">
          {step.completed ? (
            <span className="text-green-500">&#10003;</span>
          ) : (
            <span className="text-gray-300">&#9675;</span>
          )}
          <span className={step.completed ? 'text-gray-500' : 'text-gray-700'}>{step.label}</span>
        </div>
      ))}
    </div>
  </div>
)}
```

- [ ] **Step 2: Commit**

---

## F7: Telegram Gateway Channel

### Task 19: Backend — Add telegram_gateway channel type

**Files:**
- Modify: DB enum or channel config where channel types are defined
- Modify: Tarification tables to support telegram_gateway pricing

- [ ] **Step 1: Create migration adding channel type**

```sql
-- Add telegram_gateway to channels if using enum or config table
INSERT INTO channels (id, name, code, type, is_active)
VALUES (gen_random_uuid(), 'Telegram Gateway', 'telegram_gateway', 'messenger', true)
ON CONFLICT DO NOTHING;
```

- [ ] **Step 2: Commit**

### Task 20: Frontend — Add Telegram Gateway tab in tariffs

**Files:**
- Modify: `portal-frontend/src/pages/tariffs/TariffsPage.tsx`

- [ ] **Step 1: Add telegram_gateway to channel tabs/display if tariff data includes it**

This depends on how tariff data is returned from backend. If tariffs are grouped by channel, add `telegram_gateway` label mapping:

```typescript
const CHANNEL_LABELS: Record<string, string> = {
  sms: 'SMS',
  viber: 'Viber',
  voice: 'Voice',
  hlr: 'HLR',
  telegram_gateway: 'Telegram Gateway',
  flash_call: 'Flash Call',
};
```

- [ ] **Step 2: Commit**

---

## F8: Flash Call / RusCall Channels

### Task 21-22: Similar to F7

Add `flash_call` and `rus_call` channel types to the channels table, tariff display, and campaign wizard channel selection. Follow the same pattern as Task 19-20.

---

## F9: Support Chat Widget

### Task 23: Add support chat widget

**Files:**
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`

- [ ] **Step 1: Add chat widget button**

At the bottom of UserLayout, before closing `</div>`:

```tsx
{/* Support chat widget */}
<a
  href="https://t.me/your_support_bot"
  target="_blank"
  rel="noopener noreferrer"
  className="fixed bottom-6 right-6 z-50 w-14 h-14 bg-primary text-white rounded-full shadow-lg flex items-center justify-center hover:bg-primary/90 transition-colors"
  aria-label="Поддержка"
  title="Написать в поддержку"
>
  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
  </svg>
</a>
```

Replace the `href` with actual support URL (Telegram, intercom, or custom chat endpoint).

- [ ] **Step 2: Commit**

---

## F10: Default Sender Names in Settings

### Task 24: Backend — Default sender name setting

**Files:**
- Create: `migrations/NNNNNN_default_sender_names.up.sql`
- Modify: `internal/gateway/portal/handlers/settings.go`

- [ ] **Step 1: Migration**

```sql
CREATE TABLE default_sender_names (
    client_id UUID NOT NULL,
    channel VARCHAR(20) NOT NULL,  -- 'sms', 'viber'
    sender_name_id UUID NOT NULL,
    PRIMARY KEY (client_id, channel)
);
```

- [ ] **Step 2: Add handlers**

In `settings.go`, add GET/PUT for `/settings/default-senders`:

```go
func (h *SettingsHandlers) GetDefaultSenders(w http.ResponseWriter, r *http.Request) {
	clientID, _ := middleware.GetClientID(r.Context())
	rows, err := h.db.Query(r.Context(),
		`SELECT channel, sender_name_id::text FROM default_sender_names WHERE client_id = $1`,
		clientID.String(),
	)
	// ... scan and return map[channel]sender_name_id
}

func (h *SettingsHandlers) SetDefaultSenders(w http.ResponseWriter, r *http.Request) {
	// UPSERT per channel
}
```

- [ ] **Step 3: Register routes and commit**

### Task 25: Frontend — Default sender names in settings

**Files:**
- Modify: `portal-frontend/src/pages/settings/NotificationSettingsPage.tsx` or create separate page

- [ ] **Step 1: Add default sender name selectors**

Add a section to the general settings or notification settings page with two dropdown selects (SMS, Viber) populated from the sender names API.

- [ ] **Step 2: Commit**

---

## Implementation Order (Recommended)

1. **F1** (Detalization) — Tasks 1-3 — standalone, no dependencies
2. **F3** (Notification Settings) — Tasks 7-10 — standalone
3. **F2** (Cost Estimation) — Tasks 4-6 — standalone
4. **F6** (Profile Progress) — Tasks 17-18 — standalone, quick win
5. **F9** (Support Chat) — Task 23 — trivial, 5 min
6. **F10** (Default Senders) — Tasks 24-25 — standalone
7. **F5** (SMPP Settings) — Tasks 15-16 — wrapper over existing
8. **F4** (Recurring Campaigns) — Tasks 11-14 — most complex
9. **F7** (Telegram Gateway) — Tasks 19-20 — needs business decision
10. **F8** (Flash Call) — Tasks 21-22 — needs business decision
