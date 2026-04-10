# Message Detail View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `GET /portal/v1/messages/{id}` and the `MessageDetailPage` frontend to return and display all available message information: source/destination/text (currently missing!), provider name, route name, retry info, raw DLR receipt from operator, and billing data.

**Architecture:** The portal gateway already has a direct `*pgxpool.Pool` connection (used by 8+ handlers). The current `GetMessage` handler calls `GetMessageStatus` gRPC which doesn't return source/destination/text — a pre-existing bug. We fix this by replacing the gRPC call with 3 direct SQL queries: one JOIN query for the message + provider/route names, one for DLR receipt, one for tarification log. No proto changes needed.

**Tech Stack:** Go 1.24 + jackc/pgx/v5 (pgxpool), gorilla/mux, React 19 + TypeScript + Tailwind CSS 4.2

---

## File Map

| File | Action | What changes |
|------|--------|-------------|
| `internal/gateway/portal/handlers/messages.go` | Modify | Add `db *pgxpool.Pool` field; rewrite `GetMessage` with SQL; add `NewMessageHandlersWithDB` constructor |
| `cmd/portal-gateway/main.go` | Modify | Pass `dbPool` to `NewMessageHandlers` |
| `portal-frontend/src/pages/messages/MessageDetailPage.tsx` | Modify | Extend `MessageDetail` interface; add provider/route/retry fields; add DLR card; add billing card |

---

## Task 1: Add DB to MessageHandlers and rewrite GetMessage

**Files:**
- Modify: `internal/gateway/portal/handlers/messages.go`

### Context

Current `GetMessage` (line 205–264) calls `h.messagingClient.GetMessageStatus(...)` which only returns status/timestamps — no `source`, `destination`, or `text`. We replace it with 3 direct pgx queries.

The portal `dbPool` is `*pgxpool.Pool` (jackc/pgx/v5). Other handlers use it the same way, e.g. `settingsHandlers`, `segmentHandlers`, `notificationHandlers`.

`messages.route_id` stores a `client_routes.id` UUID (written by the pipeline router stage). `client_routes.name` was added in migration 000077. `messages.provider_id` references `providers.id`.

- [ ] **Step 1: Add `db` field and update constructor**

In `internal/gateway/portal/handlers/messages.go`, change the struct and constructor:

```go
import (
    // add to existing imports:
    "github.com/jackc/pgx/v5/pgxpool"
)

type MessageHandlers struct {
    messagingClient messagingv1.MessagingServiceClient
    sseHub          *sse.Hub
    db              *pgxpool.Pool  // add this field
}

func NewMessageHandlers(messagingClient messagingv1.MessagingServiceClient) *MessageHandlers {
    return &MessageHandlers{
        messagingClient: messagingClient,
    }
}

// SetDB sets the database pool for direct SQL queries in GetMessage.
func (h *MessageHandlers) SetDB(db *pgxpool.Pool) {
    h.db = db
}
```

- [ ] **Step 2: Rewrite GetMessage handler**

Replace the entire `GetMessage` method (lines 204–264) with:

```go
// GetMessage обрабатывает GET /messages/{id}
// Returns full message detail including provider/route names, DLR receipt, and billing info.
func (h *MessageHandlers) GetMessage(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID сообщения обязателен"))
		return
	}

	if h.db == nil {
		// Fallback to gRPC-only (no source/destination/text)
		h.getMessageViaGRPC(w, r, id, clientID.String())
		return
	}

	ctx := r.Context()

	// ── 1. Main message query with provider and route names ──────────────────
	const msgQuery = `
		SELECT
			m.id::text,
			COALESCE(m.source, '')        AS source,
			COALESCE(m.destination, '')   AS destination,
			COALESCE(m.text, '')          AS text,
			COALESCE(m.encoding, 'GSM7')  AS encoding,
			m.status::text,
			COALESCE(m.status_message, '') AS status_message,
			COALESCE(m.external_id, '')    AS external_id,
			COALESCE(m.segment_count, 0)   AS segment_count,
			COALESCE(m.retry_count, 0)     AS retry_count,
			COALESCE(m.max_retries, 0)     AS max_retries,
			COALESCE(m.provider_id::text, '') AS provider_id,
			COALESCE(m.route_id::text, '')    AS route_id,
			COALESCE(m.smpp_message_id, '')   AS smpp_message_id,
			m.created_at,
			m.submitted_at,
			m.delivered_at,
			m.failed_at,
			m.scheduled_at,
			m.expired_at,
			COALESCE(p.name, '') AS provider_name,
			COALESCE(r.name, '') AS route_name
		FROM messages m
		LEFT JOIN providers p ON p.id = m.provider_id
		LEFT JOIN client_routes r ON r.id = m.route_id
		WHERE m.id = $1::uuid AND m.client_id = $2::uuid
		ORDER BY m.created_at DESC
		LIMIT 1
	`

	type msgRow struct {
		ID            string
		Source        string
		Destination   string
		Text          string
		Encoding      string
		Status        string
		StatusMessage string
		ExternalID    string
		SegmentCount  int
		RetryCount    int
		MaxRetries    int
		ProviderID    string
		RouteID       string
		SMPPMessageID string
		CreatedAt     time.Time
		SubmittedAt   *time.Time
		DeliveredAt   *time.Time
		FailedAt      *time.Time
		ScheduledAt   *time.Time
		ExpiredAt     *time.Time
		ProviderName  string
		RouteName     string
	}

	row := h.db.QueryRow(ctx, msgQuery, id, clientID.String())
	var m msgRow
	err := row.Scan(
		&m.ID, &m.Source, &m.Destination, &m.Text, &m.Encoding,
		&m.Status, &m.StatusMessage, &m.ExternalID,
		&m.SegmentCount, &m.RetryCount, &m.MaxRetries,
		&m.ProviderID, &m.RouteID, &m.SMPPMessageID,
		&m.CreatedAt, &m.SubmittedAt, &m.DeliveredAt, &m.FailedAt,
		&m.ScheduledAt, &m.ExpiredAt,
		&m.ProviderName, &m.RouteName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, shared.ErrNotFound("Сообщение не найдено"))
			return
		}
		log.Error().Err(err).Str("id", id).Msg("ошибка запроса сообщения")
		respondError(w, shared.ErrInternal("Ошибка получения сообщения"))
		return
	}

	result := map[string]interface{}{
		"message_id":     m.ID,
		"source":         m.Source,
		"destination":    m.Destination,
		"text":           m.Text,
		"encoding":       m.Encoding,
		"status":         m.Status,
		"segment_count":  m.SegmentCount,
		"retry_count":    m.RetryCount,
		"max_retries":    m.MaxRetries,
		"created_at":     m.CreatedAt,
	}
	if m.StatusMessage != "" {
		result["status_message"] = m.StatusMessage
	}
	if m.ExternalID != "" {
		result["external_id"] = m.ExternalID
	}
	if m.SMPPMessageID != "" {
		result["smpp_message_id"] = m.SMPPMessageID
	}
	if m.ProviderID != "" {
		result["provider_id"] = m.ProviderID
		result["provider_name"] = m.ProviderName
	}
	if m.RouteID != "" {
		result["route_id"] = m.RouteID
		result["route_name"] = m.RouteName
	}
	if m.SubmittedAt != nil {
		result["submitted_at"] = m.SubmittedAt
	}
	if m.DeliveredAt != nil {
		result["delivered_at"] = m.DeliveredAt
	}
	if m.FailedAt != nil {
		result["failed_at"] = m.FailedAt
	}
	if m.ScheduledAt != nil {
		result["scheduled_at"] = m.ScheduledAt
	}
	if m.ExpiredAt != nil {
		result["expired_at"] = m.ExpiredAt
	}

	// ── 2. DLR receipt (most recent) ─────────────────────────────────────────
	const dlrQuery = `
		SELECT stat, COALESCE(err, 0), COALESCE(text, ''),
		       submit_date, done_date,
		       COALESCE(receipted_message_id, '')
		FROM dlr_receipts
		WHERE message_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT 1
	`
	dlrRow := h.db.QueryRow(ctx, dlrQuery, id)
	var dlrStat, dlrText, dlrReceiptedID string
	var dlrErr int
	var dlrSubmitDate, dlrDoneDate *time.Time
	if err := dlrRow.Scan(&dlrStat, &dlrErr, &dlrText, &dlrSubmitDate, &dlrDoneDate, &dlrReceiptedID); err == nil {
		dlr := map[string]interface{}{
			"stat": dlrStat,
			"err":  dlrErr,
			"text": dlrText,
		}
		if dlrSubmitDate != nil {
			dlr["submit_date"] = dlrSubmitDate
		}
		if dlrDoneDate != nil {
			dlr["done_date"] = dlrDoneDate
		}
		if dlrReceiptedID != "" {
			dlr["receipted_message_id"] = dlrReceiptedID
		}
		result["dlr"] = dlr
	}

	// ── 3. Billing / tarification log ────────────────────────────────────────
	const billingQuery = `
		SELECT segment_count, price_per_segment, total_amount,
		       tariff_plan_id::text, created_at
		FROM tarification_log
		WHERE message_id = $1::uuid
		LIMIT 1
	`
	billingRow := h.db.QueryRow(ctx, billingQuery, id)
	var bSegments int
	var bPricePerSeg, bTotal, bPlanID string
	var bCreatedAt time.Time
	if err := billingRow.Scan(&bSegments, &bPricePerSeg, &bTotal, &bPlanID, &bCreatedAt); err == nil {
		result["billing"] = map[string]interface{}{
			"segment_count":     bSegments,
			"price_per_segment": bPricePerSeg,
			"total_amount":      bTotal,
			"tariff_plan_id":    bPlanID,
			"billed_at":         bCreatedAt,
		}
	}

	respondJSON(w, http.StatusOK, result)
}

// getMessageViaGRPC is the fallback when dbPool is not available.
func (h *MessageHandlers) getMessageViaGRPC(w http.ResponseWriter, r *http.Request, id, clientID string) {
	if h.messagingClient == nil {
		respondError(w, shared.ErrServiceUnavailable("Сервис недоступен"))
		return
	}
	resp, err := h.messagingClient.GetMessageStatus(r.Context(), &messagingv1.GetMessageStatusRequest{
		MessageId: id,
		ClientId:  clientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	result := map[string]interface{}{
		"message_id":    resp.MessageId,
		"status":        resp.Status,
		"segment_count": resp.SegmentCount,
	}
	if resp.StatusMessage != "" {
		result["status_message"] = resp.StatusMessage
	}
	if resp.SmppMessageId != "" {
		result["smpp_message_id"] = resp.SmppMessageId
	}
	if resp.ErrorCode != "" {
		result["error_code"] = resp.ErrorCode
	}
	if resp.ErrorMessage != "" {
		result["error_message"] = resp.ErrorMessage
	}
	if resp.CreatedAt != nil {
		result["created_at"] = resp.CreatedAt.AsTime()
	}
	if resp.SubmittedAt != nil {
		result["submitted_at"] = resp.SubmittedAt.AsTime()
	}
	if resp.DeliveredAt != nil {
		result["delivered_at"] = resp.DeliveredAt.AsTime()
	}
	if resp.FailedAt != nil {
		result["failed_at"] = resp.FailedAt.AsTime()
	}
	if resp.ScheduledAt != nil {
		result["scheduled_at"] = resp.ScheduledAt.AsTime()
	}
	if resp.ExpiredAt != nil {
		result["expired_at"] = resp.ExpiredAt.AsTime()
	}
	respondJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 3: Add missing imports**

Ensure the import block in `messages.go` includes:

```go
import (
    "encoding/csv"
    "encoding/json"
    "errors"       // add
    "fmt"
    "net/http"
    "time"

    "github.com/gorilla/mux"
    "github.com/jackc/pgx/v5"        // add — for pgx.ErrNoRows
    "github.com/jackc/pgx/v5/pgxpool" // add
    "github.com/rs/zerolog/log"
    "google.golang.org/protobuf/types/known/timestamppb"

    "github.com/smpp-server/smpp-server/api/proto/messagingv1"
    "github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
    "github.com/smpp-server/smpp-server/internal/gateway/portal/sse"
    "github.com/smpp-server/smpp-server/internal/shared"
)
```

- [ ] **Step 4: Verify the file compiles**

```bash
cd /c/projects/sms && go build ./internal/gateway/portal/handlers/...
```

Expected: no output (success).

- [ ] **Step 5: Commit**

```bash
cd /c/projects/sms
git add internal/gateway/portal/handlers/messages.go
git commit -m "feat(portal): rewrite GetMessage handler with full SQL detail query

- Add dbPool field + SetDB() setter to MessageHandlers
- Replace gRPC-only GetMessage with 3 direct SQL queries:
  messages JOIN providers/client_routes, dlr_receipts, tarification_log
- Add gRPC fallback for when dbPool is nil
- Fixes missing source/destination/text in message detail"
```

---

## Task 2: Wire dbPool into MessageHandlers in main.go

**Files:**
- Modify: `cmd/portal-gateway/main.go`

- [ ] **Step 1: Pass dbPool to NewMessageHandlers**

Find line 168 in `cmd/portal-gateway/main.go`:

```go
messageHandlers := handlers.NewMessageHandlers(serviceClients.MessagingClient)
```

Replace with:

```go
messageHandlers := handlers.NewMessageHandlers(serviceClients.MessagingClient)
if dbPool != nil {
    messageHandlers.SetDB(dbPool)
}
```

- [ ] **Step 2: Verify the file compiles**

```bash
cd /c/projects/sms && go build ./cmd/portal-gateway/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
cd /c/projects/sms
git add cmd/portal-gateway/main.go
git commit -m "feat(portal): wire dbPool into MessageHandlers for full message detail"
```

---

## Task 3: Frontend — extend MessageDetailPage with all new fields

**Files:**
- Modify: `portal-frontend/src/pages/messages/MessageDetailPage.tsx`

### Context

Current `MessageDetail` interface (lines 8–21) is missing: `source`, `destination`, `text` (all returned now), plus new fields: `encoding`, `external_id`, `retry_count`, `max_retries`, `provider_id`, `provider_name`, `route_id`, `route_name`, `smpp_message_id`, `status_message`, `scheduled_at`, `expired_at`, and nested `dlr`/`billing` objects.

The page currently has 2 cards: detail card + timeline. We add a 3rd card (DLR) and 4th card (billing), both only shown when data is present.

- [ ] **Step 1: Update MessageDetail interface**

Replace the `interface MessageDetail` block (lines 8–21):

```typescript
interface DlrInfo {
  stat: string;           // DELIVRD, UNDELIV, EXPIRED, REJECTD
  err: number;
  text: string;
  submit_date?: string;
  done_date?: string;
  receipted_message_id?: string;
}

interface BillingInfo {
  segment_count: number;
  price_per_segment: string;
  total_amount: string;
  tariff_plan_id: string;
  billed_at: string;
}

interface MessageDetail {
  message_id: string;
  source: string;
  destination: string;
  text: string;
  encoding?: string;
  status: string;
  status_message?: string;
  external_id?: string;
  segment_count: number;
  retry_count?: number;
  max_retries?: number;
  provider_id?: string;
  provider_name?: string;
  route_id?: string;
  route_name?: string;
  smpp_message_id?: string;
  created_at?: string;
  submitted_at?: string;
  delivered_at?: string;
  failed_at?: string;
  scheduled_at?: string;
  expired_at?: string;
  dlr?: DlrInfo;
  billing?: BillingInfo;
}
```

- [ ] **Step 2: Add DLR stat label helper**

Add after `STATUS_LABEL` (after line 43):

```typescript
const DLR_STAT_LABEL: Record<string, string> = {
  DELIVRD: 'Доставлено',
  UNDELIV: 'Не доставлено',
  EXPIRED: 'Истекло',
  REJECTD: 'Отклонено',
  ACCEPTD: 'Принято',
  DELETED: 'Удалено',
  UNKNOWN: 'Неизвестно',
};

const DLR_STAT_VARIANT: Record<string, 'success' | 'danger' | 'warning' | 'default'> = {
  DELIVRD: 'success',
  UNDELIV: 'danger',
  EXPIRED: 'danger',
  REJECTD: 'danger',
  ACCEPTD: 'warning',
  DELETED: 'warning',
  UNKNOWN: 'default',
};
```

- [ ] **Step 3: Extend the detail card with new fields**

In the `return (...)` JSX, find the detail card grid (the `<div className="grid grid-cols-1 sm:grid-cols-2 gap-4">` that starts around line 149). After the existing 6 grid items (ID, Status, Sender, Recipient, Segments, Created), add:

```tsx
{message.encoding && (
  <div>
    <span className="text-sm text-gray-500">Кодировка</span>
    <p className="text-sm mt-0.5 font-mono">{message.encoding}</p>
  </div>
)}
{message.external_id && (
  <div>
    <span className="text-sm text-gray-500">Внешний ID</span>
    <p className="text-sm mt-0.5 font-mono break-all">{message.external_id}</p>
  </div>
)}
{message.provider_name && (
  <div>
    <span className="text-sm text-gray-500">Провайдер</span>
    <p className="text-sm mt-0.5">{message.provider_name}</p>
  </div>
)}
{message.route_name && (
  <div>
    <span className="text-sm text-gray-500">Маршрут</span>
    <p className="text-sm mt-0.5">{message.route_name}</p>
  </div>
)}
{message.smpp_message_id && (
  <div>
    <span className="text-sm text-gray-500">SMPP ID</span>
    <p className="text-sm mt-0.5 font-mono break-all">{message.smpp_message_id}</p>
  </div>
)}
{(message.retry_count !== undefined && message.retry_count > 0) && (
  <div>
    <span className="text-sm text-gray-500">Попытки</span>
    <p className="text-sm mt-0.5">
      {message.retry_count} / {message.max_retries ?? '—'}
    </p>
  </div>
)}
{message.scheduled_at && (
  <div>
    <span className="text-sm text-gray-500">Запланировано</span>
    <p className="text-sm mt-0.5">{formatTimestamp(message.scheduled_at)}</p>
  </div>
)}
{message.expired_at && (
  <div>
    <span className="text-sm text-gray-500">Истекает</span>
    <p className="text-sm mt-0.5">{formatTimestamp(message.expired_at)}</p>
  </div>
)}
```

- [ ] **Step 4: Add DLR card**

After the closing `</div>` of the timeline card (after line 239), add:

```tsx
{/* DLR receipt from operator */}
{message.dlr && (
  <div className="bg-white border border-gray-200 rounded-lg p-6 mt-6">
    <h2 className="text-base font-semibold text-gray-900 mb-4">Ответ оператора (DLR)</h2>
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div>
        <span className="text-sm text-gray-500">Статус оператора</span>
        <div className="mt-0.5">
          <Badge variant={DLR_STAT_VARIANT[message.dlr.stat] ?? 'default'}>
            {DLR_STAT_LABEL[message.dlr.stat] ?? message.dlr.stat}
          </Badge>
        </div>
      </div>
      {message.dlr.err !== 0 && (
        <div>
          <span className="text-sm text-gray-500">Код ошибки</span>
          <p className="text-sm mt-0.5 font-mono text-red-700">{message.dlr.err}</p>
        </div>
      )}
      {message.dlr.submit_date && (
        <div>
          <span className="text-sm text-gray-500">Принято оператором</span>
          <p className="text-sm mt-0.5">{formatTimestamp(message.dlr.submit_date)}</p>
        </div>
      )}
      {message.dlr.done_date && (
        <div>
          <span className="text-sm text-gray-500">Статус от оператора</span>
          <p className="text-sm mt-0.5">{formatTimestamp(message.dlr.done_date)}</p>
        </div>
      )}
      {message.dlr.receipted_message_id && (
        <div className="sm:col-span-2">
          <span className="text-sm text-gray-500">ID оператора</span>
          <p className="text-sm mt-0.5 font-mono break-all">{message.dlr.receipted_message_id}</p>
        </div>
      )}
    </div>
    {message.dlr.text && (
      <div className="mt-4 pt-4 border-t border-gray-100">
        <span className="text-sm text-gray-500">Текст от оператора</span>
        <p className="text-sm mt-1 whitespace-pre-wrap bg-gray-50 rounded p-3 font-mono">{message.dlr.text}</p>
      </div>
    )}
  </div>
)}
```

- [ ] **Step 5: Add billing card**

After the DLR card, add:

```tsx
{/* Billing information */}
{message.billing && (
  <div className="bg-white border border-gray-200 rounded-lg p-6 mt-6">
    <h2 className="text-base font-semibold text-gray-900 mb-4">Биллинг</h2>
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
      <div>
        <span className="text-sm text-gray-500">Сегментов</span>
        <p className="text-sm mt-0.5 font-semibold">{message.billing.segment_count}</p>
      </div>
      <div>
        <span className="text-sm text-gray-500">Цена за сегмент</span>
        <p className="text-sm mt-0.5 font-semibold">{message.billing.price_per_segment} ₽</p>
      </div>
      <div>
        <span className="text-sm text-gray-500">Итого</span>
        <p className="text-sm mt-0.5 font-semibold text-blue-700">{message.billing.total_amount} ₽</p>
      </div>
    </div>
    <div className="mt-3 text-xs text-gray-400">
      Списание: {formatTimestamp(message.billing.billed_at)}
    </div>
  </div>
)}
```

- [ ] **Step 6: Build frontend to check for TypeScript errors**

```bash
cd /c/projects/sms/portal-frontend && npm run build 2>&1 | tail -20
```

Expected: build completes with no TypeScript errors.

- [ ] **Step 7: Commit**

```bash
cd /c/projects/sms
git add portal-frontend/src/pages/messages/MessageDetailPage.tsx
git commit -m "feat(portal-ui): show full message detail — provider, route, DLR, billing

- Add DlrInfo and BillingInfo interfaces
- Extend MessageDetail with encoding, external_id, provider/route names,
  smpp_message_id, retry_count, scheduled_at, expired_at, dlr, billing
- Add DLR receipt card with operator stat, error code, timestamps
- Add billing card with segment count, price per segment, total
- Add DLR_STAT_LABEL/VARIANT helpers for operator status display"
```

---

## Self-Review

### Spec coverage

| Requirement | Covered by |
|-------------|-----------|
| source, destination, text (currently missing) | Task 1 SQL query |
| encoding | Task 1 SQL query + Task 3 interface |
| external_id | Task 1 SQL query + Task 3 interface |
| provider name | Task 1 SQL JOIN on providers + Task 3 |
| route name | Task 1 SQL JOIN on client_routes + Task 3 |
| smpp_message_id | Task 1 SQL + Task 3 |
| retry_count, max_retries | Task 1 SQL + Task 3 |
| DLR receipt (stat, err, text, dates) | Task 1 dlrQuery + Task 3 DLR card |
| Billing (price_per_segment, total, tariff_plan) | Task 1 billingQuery + Task 3 billing card |
| scheduled_at, expired_at | Task 1 SQL + Task 3 |

### Placeholder scan

No TBD/TODO placeholders. All SQL, Go, and TSX code is complete.

### Type consistency

- `dlr` in Go `result` map → `dlr?: DlrInfo` in TS — match.
- `billing` in Go `result` map → `billing?: BillingInfo` in TS — match.
- All field names consistent between Go map keys and TS interface: `billed_at`, `receipted_message_id`, `total_amount` etc. — verified.
- `provider_name` / `route_name` in both Go and TS — match.
