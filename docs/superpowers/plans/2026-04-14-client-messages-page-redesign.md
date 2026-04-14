# Client Messages Page Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the client-side "Сообщения" and "Детализация" pages with a single "Сообщения" page featuring advanced filters, configurable columns, active filter chips, export, and a quick-view modal.

**Architecture:** New `MessagesPage` at `/messages` built from scratch using focused sub-components. Backend extends `/portal/v1/detalization` with new query params and response fields, plus adds `/portal/v1/references/operators` and `/portal/v1/references/countries` lookup endpoints.

**Tech Stack:** Go 1.24 + pgx/v5 (backend), TypeScript 5.7 + React 19 + Tailwind CSS 4.2 (frontend), existing `apiFetch` / `exportApi` utilities.

---

## File Map

### Created
- `migrations/0007X_add_messages_channel_operator.up.sql` — add `channel`, `operator_id`, `country_id`, `send_method` columns to messages
- `migrations/0007X_add_messages_channel_operator.down.sql`
- `internal/gateway/portal/handlers/references.go` — `ReferencesHandlers` with operators & countries
- `portal-frontend/src/pages/messages/components/ActiveFilterChips.tsx`
- `portal-frontend/src/pages/messages/components/ColumnConfigurator.tsx`
- `portal-frontend/src/pages/messages/components/MessageModal.tsx`
- `portal-frontend/src/pages/messages/components/MessageFilters.tsx`
- `portal-frontend/src/pages/messages/components/MessageTable.tsx`

### Modified
- `internal/gateway/portal/handlers/detalization.go` — new struct fields, new filters, sort, join with operators/countries/clients
- `internal/gateway/portal/router/router.go` — register `/portal/v1/references` routes
- `portal-frontend/src/api/client.ts` — updated types + new `referencesApi`
- `portal-frontend/src/pages/messages/MessagesPage.tsx` — full rewrite
- `portal-frontend/src/App.tsx` — remove `/detalization` route
- `portal-frontend/src/components/layout/UserLayout.tsx` — remove "Детализация" nav item

### Deleted
- `portal-frontend/src/pages/detalization/DetalizationPage.tsx`

---

## Task 1: DB Migration — add channel/operator columns to messages

**Files:**
- Create: `migrations/000071_add_messages_channel_operator.up.sql`
- Create: `migrations/000071_add_messages_channel_operator.down.sql`

> Note: verify the next migration number is actually 71 by running `ls migrations/*.up.sql | tail -5` before creating.

- [ ] **Step 1: Check next migration number**

```bash
ls c:/projects/sms/migrations/*.up.sql | sort | tail -5
```

Expected: see the highest-numbered migration. Use the next number for yours.

- [ ] **Step 2: Write up migration**

```sql
-- migrations/000071_add_messages_channel_operator.up.sql
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS channel     VARCHAR(20) DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS operator_id UUID        DEFAULT NULL REFERENCES operators(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS country_id  UUID        DEFAULT NULL REFERENCES countries(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS send_method VARCHAR(20) DEFAULT NULL;

COMMENT ON COLUMN messages.channel     IS 'Канал доставки: SMS, MAX, Viber';
COMMENT ON COLUMN messages.operator_id IS 'Оператор абонента (определяется при маршрутизации)';
COMMENT ON COLUMN messages.country_id  IS 'Страна абонента';
COMMENT ON COLUMN messages.send_method IS 'Способ отправки: PORTAL, API, SMPP';

CREATE INDEX IF NOT EXISTS idx_messages_operator_id ON messages(operator_id) WHERE operator_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_channel     ON messages(channel)     WHERE channel IS NOT NULL;
```

- [ ] **Step 3: Write down migration**

```sql
-- migrations/000071_add_messages_channel_operator.down.sql
ALTER TABLE messages
    DROP COLUMN IF EXISTS channel,
    DROP COLUMN IF EXISTS operator_id,
    DROP COLUMN IF EXISTS country_id,
    DROP COLUMN IF EXISTS send_method;
```

- [ ] **Step 4: Apply migration on server**

```bash
scripts/server.sh migrate
```

Expected: migration applies without error.

- [ ] **Step 5: Commit**

```bash
git add migrations/000071_add_messages_channel_operator.up.sql migrations/000071_add_messages_channel_operator.down.sql
git commit -m "feat(db): add channel, operator_id, country_id, send_method to messages"
```

---

## Task 2: Backend — ReferencesHandlers

**Files:**
- Create: `internal/gateway/portal/handlers/references.go`

- [ ] **Step 1: Write references handler**

```go
// internal/gateway/portal/handlers/references.go
package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ReferencesHandlers returns static reference data (operators, countries) for filter dropdowns.
type ReferencesHandlers struct {
	db *pgxpool.Pool
}

// NewReferencesHandlers creates a new ReferencesHandlers.
func NewReferencesHandlers(db *pgxpool.Pool) *ReferencesHandlers {
	return &ReferencesHandlers{db: db}
}

type operatorItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type countryItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	ISOCode string `json:"iso_code"`
}

// ListOperators handles GET /portal/v1/references/operators
func (h *ReferencesHandlers) ListOperators(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, name, code
		FROM operators
		WHERE active = true
		ORDER BY name
	`)
	if err != nil {
		log.Error().Err(err).Msg("references: failed to list operators")
		respondError(w, shared.ErrInternalServer("Ошибка получения операторов"))
		return
	}
	defer rows.Close()

	operators := []operatorItem{}
	for rows.Next() {
		var op operatorItem
		if err := rows.Scan(&op.ID, &op.Name, &op.Code); err != nil {
			continue
		}
		operators = append(operators, op)
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка итерации операторов"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"operators": operators})
}

// ListCountries handles GET /portal/v1/references/countries
func (h *ReferencesHandlers) ListCountries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, name, iso_code
		FROM countries
		ORDER BY name
	`)
	if err != nil {
		log.Error().Err(err).Msg("references: failed to list countries")
		respondError(w, shared.ErrInternalServer("Ошибка получения стран"))
		return
	}
	defer rows.Close()

	countries := []countryItem{}
	for rows.Next() {
		var c countryItem
		if err := rows.Scan(&c.ID, &c.Name, &c.ISOCode); err != nil {
			continue
		}
		countries = append(countries, c)
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка итерации стран"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"countries": countries})
}
```

- [ ] **Step 2: Build to verify compilation**

```bash
cd c:/projects/sms && go build ./internal/gateway/portal/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/references.go
git commit -m "feat(portal): add ReferencesHandlers for operators and countries dropdowns"
```

---

## Task 3: Backend — Register reference routes in router.go

**Files:**
- Modify: `internal/gateway/portal/router/router.go:26-66` (function signature)
- Modify: `internal/gateway/portal/router/router.go` (add routes inside body)

- [ ] **Step 1: Add `referencesHandlers` param to SetupRouter signature**

In `router.go`, find the `SetupRouter` function signature and add:

```go
// Add this parameter to SetupRouter, after companyHandlers:
referencesHandlers *handlers.ReferencesHandlers,
```

- [ ] **Step 2: Add reference routes inside SetupRouter body**

After the `optOut` block (around line 358 in router.go), add:

```go
// References endpoints (operators, countries for dropdowns)
references := protected.PathPrefix("/references").Subrouter()
references.HandleFunc("/operators", referencesHandlers.ListOperators).Methods("GET")
references.HandleFunc("/countries", referencesHandlers.ListCountries).Methods("GET")
```

- [ ] **Step 3: Wire ReferencesHandlers in the gateway main**

Find where `SetupRouter` is called (likely in `cmd/portal-gateway/main.go` or `internal/gateway/portal/gateway.go`). Add:

```bash
grep -r "SetupRouter" c:/projects/sms/internal/gateway/portal/ --include="*.go" -l
```

Open that file, create and pass a `ReferencesHandlers`:

```go
referencesHandlers := handlers.NewReferencesHandlers(db)
// Add referencesHandlers as the last argument to SetupRouter(...)
```

- [ ] **Step 4: Build to verify**

```bash
cd c:/projects/sms && go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/router/router.go
git add $(git diff --name-only HEAD -- internal/gateway/portal/)
git commit -m "feat(portal): register /references/operators and /references/countries routes"
```

---

## Task 4: Backend — Extend DetalizationHandlers with new filters and fields

**Files:**
- Modify: `internal/gateway/portal/handlers/detalization.go`

- [ ] **Step 1: Update `ClientMessage` struct with new fields**

Replace the existing `ClientMessage` struct:

```go
// ClientMessage is a single row returned by ListMessages.
type ClientMessage struct {
	ID           string     `json:"id"`
	Source       string     `json:"source"`
	Destination  string     `json:"destination"`
	TextPreview  string     `json:"text_preview"`
	Status       string     `json:"status"`
	SegmentCount int        `json:"segment_count"`
	CreatedAt    time.Time  `json:"created_at"`
	SubmittedAt  *time.Time `json:"submitted_at,omitempty"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
	FailedAt     *time.Time `json:"failed_at,omitempty"`
	ProviderName string     `json:"provider_name"`
	OperatorName string     `json:"operator_name,omitempty"`
	CountryName  string     `json:"country_name,omitempty"`
	Channel      string     `json:"channel,omitempty"`
	SendMethod   string     `json:"send_method,omitempty"`
	Login        string     `json:"login,omitempty"`
	TotalAmount  string     `json:"total_amount,omitempty"`
}
```

- [ ] **Step 2: Replace `ListMessages` handler with updated version**

Replace the entire `ListMessages` function body. Note: the function signature stays the same.

```go
// ListMessages handles GET /portal/v1/detalization
// Query params: status, source, destination, date_from, date_to, limit, offset,
//               login, operator, sender_name, channel, country, send_method, message_id,
//               sort_by (submitted_at|created_at|status_at|total_amount|segment_count), sort_order (asc|desc)
func (h *DetalizationHandlers) ListMessages(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	q := r.URL.Query()
	status      := q.Get("status")
	source      := q.Get("source")
	destination := q.Get("destination")
	dateFrom    := q.Get("date_from")
	dateTo      := q.Get("date_to")
	login       := q.Get("login")
	operator    := q.Get("operator")
	senderName  := q.Get("sender_name")
	channel     := q.Get("channel")
	country     := q.Get("country")
	sendMethod  := q.Get("send_method")
	messageID   := q.Get("message_id")
	sortBy      := q.Get("sort_by")
	sortOrder   := q.Get("sort_order")

	// Validate sort params
	validSortBy := map[string]string{
		"submitted_at":  "m.submitted_at",
		"created_at":    "m.created_at",
		"status_at":     "COALESCE(m.delivered_at, m.failed_at)",
		"total_amount":  "tl.total_amount",
		"segment_count": "m.segment_count",
	}
	orderCol := "m.created_at"
	if col, ok := validSortBy[sortBy]; ok {
		orderCol = col
	}
	direction := "DESC"
	if sortOrder == "asc" {
		direction = "ASC"
	}

	limit := 20
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 200 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(q.Get("offset")); err == nil && v >= 0 {
		offset = v
	}

	// Build WHERE conditions
	args := []interface{}{clientID.String()}
	// Match messages belonging to this client OR its sub-accounts
	conditions := ` AND (m.client_id = $1::uuid OR cli.parent_client_id = $1::uuid)`
	nextArg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if status != "" {
		conditions += " AND m.status::text = " + nextArg(status)
	}
	if source != "" {
		conditions += " AND m.source ILIKE " + nextArg("%"+source+"%")
	}
	if senderName != "" {
		conditions += " AND m.source ILIKE " + nextArg("%"+senderName+"%")
	}
	if destination != "" {
		conditions += " AND m.destination ILIKE " + nextArg("%"+destination+"%")
	}
	if dateFrom != "" {
		conditions += " AND m.created_at >= " + nextArg(dateFrom) + "::timestamptz"
	}
	if dateTo != "" {
		conditions += " AND m.created_at < (" + nextArg(dateTo) + "::timestamptz + INTERVAL '1 day')"
	}
	if login != "" {
		conditions += " AND cli.name ILIKE " + nextArg("%"+login+"%")
	}
	if operator != "" {
		conditions += " AND op.name ILIKE " + nextArg("%"+operator+"%")
	}
	if channel != "" {
		conditions += " AND m.channel = " + nextArg(channel)
	}
	if country != "" {
		conditions += " AND co.name ILIKE " + nextArg("%"+country+"%")
	}
	if sendMethod != "" {
		conditions += " AND m.send_method = " + nextArg(sendMethod)
	}
	if messageID != "" {
		conditions += " AND m.id::text ILIKE " + nextArg("%"+messageID+"%")
	}

	joins := `
		LEFT JOIN providers p   ON p.id = m.provider_id
		LEFT JOIN operators op  ON op.id = m.operator_id
		LEFT JOIN countries co  ON co.id = m.country_id
		LEFT JOIN clients cli   ON cli.id = m.client_id
		LEFT JOIN LATERAL (
			SELECT total_amount
			FROM tarification_log
			WHERE message_id = m.id
			ORDER BY created_at DESC LIMIT 1
		) tl ON true`

	countQuery := `
		SELECT COUNT(*)
		FROM messages m` + joins + `
		WHERE 1=1` + conditions

	listQuery := `
		SELECT
			m.id::text,
			COALESCE(m.source, '')           AS source,
			COALESCE(m.destination, '')      AS destination,
			LEFT(COALESCE(m.text, ''), 100)  AS text_preview,
			m.status::text,
			COALESCE(m.segment_count, 0)     AS segment_count,
			m.created_at,
			m.submitted_at,
			m.delivered_at,
			m.failed_at,
			COALESCE(p.name, '')             AS provider_name,
			COALESCE(op.name, '')            AS operator_name,
			COALESCE(co.name, '')            AS country_name,
			COALESCE(m.channel, '')          AS channel,
			COALESCE(m.send_method, '')      AS send_method,
			COALESCE(cli.name, '')           AS login,
			COALESCE(tl.total_amount::text, '') AS total_amount
		FROM messages m` + joins + `
		WHERE 1=1` + conditions + `
		ORDER BY ` + orderCol + ` ` + direction + `
		LIMIT ` + strconv.Itoa(limit) + ` OFFSET ` + strconv.Itoa(offset)

	ctx := r.Context()

	var total int64
	if err := h.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		log.Error().Err(err).Msg("detalization: ошибка подсчёта сообщений")
		respondError(w, shared.ErrInternalServer("ошибка подсчёта сообщений"))
		return
	}

	rows, err := h.db.Query(ctx, listQuery, args...)
	if err != nil {
		log.Error().Err(err).Msg("detalization: ошибка запроса сообщений")
		respondError(w, shared.ErrInternalServer("Ошибка получения сообщений"))
		return
	}
	defer rows.Close()

	messages := []ClientMessage{}
	for rows.Next() {
		var (
			id, src, dst, textPreview, st     string
			providerName, operatorName        string
			countryName, channel_, sendMethod_ string
			loginName, totalAmount            string
			segmentCount                      int
			createdAt                         time.Time
			submittedAt, deliveredAt, failedAt *time.Time
		)
		if err := rows.Scan(
			&id, &src, &dst, &textPreview, &st,
			&segmentCount, &createdAt, &submittedAt, &deliveredAt, &failedAt,
			&providerName, &operatorName, &countryName, &channel_, &sendMethod_,
			&loginName, &totalAmount,
		); err != nil {
			log.Error().Err(err).Msg("detalization: ошибка сканирования строки")
			continue
		}
		messages = append(messages, ClientMessage{
			ID:           id,
			Source:       src,
			Destination:  dst,
			TextPreview:  textPreview,
			Status:       st,
			SegmentCount: segmentCount,
			CreatedAt:    createdAt,
			SubmittedAt:  submittedAt,
			DeliveredAt:  deliveredAt,
			FailedAt:     failedAt,
			ProviderName: providerName,
			OperatorName: operatorName,
			CountryName:  countryName,
			Channel:      channel_,
			SendMethod:   sendMethod_,
			Login:        loginName,
			TotalAmount:  totalAmount,
		})
	}

	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка итерации строк"))
		return
	}

	respondJSON(w, http.StatusOK, listMessagesResponse{
		Messages: messages,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	})
}
```

- [ ] **Step 3: Build to verify**

```bash
cd c:/projects/sms && go build ./internal/gateway/portal/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/detalization.go
git commit -m "feat(portal): extend detalization handler with new filters, fields, and sorting"
```

---

## Task 5: Frontend — Update API types in client.ts

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Replace `DetalizationMessage` interface**

Find and replace the `DetalizationMessage` interface (around line 739):

```typescript
export interface DetalizationMessage {
  id: string;
  source: string;
  destination: string;
  text_preview: string;
  status: string;
  segment_count: number;
  created_at: string;
  submitted_at?: string;
  delivered_at?: string;
  failed_at?: string;
  provider_name: string;
  operator_name?: string;
  country_name?: string;
  channel?: string;
  send_method?: string;
  login?: string;
  total_amount?: string;
}
```

- [ ] **Step 2: Update `detalizationApi.list` params**

Find and replace the `detalizationApi` object:

```typescript
export const detalizationApi = {
  list: (params?: {
    status?: string;
    source?: string;
    sender_name?: string;
    destination?: string;
    date_from?: string;
    date_to?: string;
    login?: string;
    operator?: string;
    channel?: string;
    country?: string;
    send_method?: string;
    message_id?: string;
    sort_by?: string;
    sort_order?: string;
    limit?: number;
    offset?: number;
  }) => {
    const filtered: Record<string, string> = {};
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        if (v !== undefined && v !== null && v !== '') filtered[k] = String(v);
      }
    }
    const qs = new URLSearchParams(filtered).toString();
    return apiFetch<{ messages: DetalizationMessage[]; total: number; limit: number; offset: number }>(
      `/detalization${qs ? `?${qs}` : ''}`,
    );
  },
  get: (id: string) => apiFetch<DetalizationMessageDetail>(`/detalization/${id}`),
};
```

- [ ] **Step 3: Add `referencesApi`** after `detalizationApi`:

```typescript
// References API for filter dropdowns
export interface OperatorRef {
  id: string;
  name: string;
  code: string;
}

export interface CountryRef {
  id: string;
  name: string;
  iso_code: string;
}

export const referencesApi = {
  operators: () => apiFetch<{ operators: OperatorRef[] }>('/references/operators'),
  countries: () => apiFetch<{ countries: CountryRef[] }>('/references/countries'),
};
```

- [ ] **Step 4: Build frontend to verify types**

```bash
cd c:/projects/sms/portal-frontend && npx tsc --noEmit 2>&1 | head -30
```

Expected: no new errors related to the changed types.

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat(portal-frontend): update detalization API types and add referencesApi"
```

---

## Task 6: Frontend — ActiveFilterChips component

**Files:**
- Create: `portal-frontend/src/pages/messages/components/ActiveFilterChips.tsx`

- [ ] **Step 1: Create component**

```tsx
// portal-frontend/src/pages/messages/components/ActiveFilterChips.tsx
import type { FilterDef } from './MessageFilters';

interface Props {
  filterDefs: FilterDef[];
  values: Record<string, string>;
  onRemove: (key: string) => void;
}

export function ActiveFilterChips({ filterDefs, values, onRemove }: Props) {
  const active = filterDefs.filter((f) => values[f.key] && values[f.key] !== '');
  if (active.length === 0) return null;

  return (
    <div className="flex flex-wrap gap-2 mb-3">
      {active.map((f) => {
        const raw = values[f.key];
        const label = f.options?.find((o) => o.value === raw)?.label ?? raw;
        return (
          <span
            key={f.key}
            className="inline-flex items-center gap-1 rounded-full bg-primary/10 text-primary px-3 py-0.5 text-sm"
          >
            <span className="text-gray-500 text-xs">{f.label}:</span>
            <span>{label}</span>
            <button
              onClick={() => onRemove(f.key)}
              className="ml-1 rounded-full hover:bg-primary/20 p-0.5"
              aria-label={`Убрать фильтр ${f.label}`}
            >
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </span>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/messages/components/ActiveFilterChips.tsx
git commit -m "feat(portal-frontend): add ActiveFilterChips component"
```

---

## Task 7: Frontend — ColumnConfigurator component

**Files:**
- Create: `portal-frontend/src/pages/messages/components/ColumnConfigurator.tsx`

- [ ] **Step 1: Create component**

```tsx
// portal-frontend/src/pages/messages/components/ColumnConfigurator.tsx
import { useState } from 'react';
import { Button } from '../../../components/ui/Button';

export interface ColumnDef {
  key: string;
  label: string;
  defaultVisible: boolean;
}

interface Props {
  columns: ColumnDef[];
  visible: Set<string>;
  onChange: (visible: Set<string>) => void;
}

export function ColumnConfigurator({ columns, visible, onChange }: Props) {
  const [open, setOpen] = useState(false);

  const toggle = (key: string) => {
    const next = new Set(visible);
    if (next.has(key)) {
      next.delete(key);
    } else {
      next.add(key);
    }
    onChange(next);
  };

  return (
    <div className="relative">
      <Button variant="secondary" onClick={() => setOpen((v) => !v)}>
        <svg className="w-4 h-4 mr-1 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
        </svg>
        Настройка таблицы
      </Button>

      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute right-0 top-full mt-1 z-20 w-56 bg-white border border-gray-200 rounded-lg shadow-lg p-3">
            <p className="text-xs font-medium text-gray-500 mb-2 uppercase tracking-wide">Столбцы</p>
            <div className="space-y-1">
              {columns.map((col) => (
                <label key={col.key} className="flex items-center gap-2 text-sm cursor-pointer hover:bg-gray-50 rounded px-1 py-0.5">
                  <input
                    type="checkbox"
                    checked={visible.has(col.key)}
                    onChange={() => toggle(col.key)}
                    className="rounded"
                  />
                  {col.label}
                </label>
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/messages/components/ColumnConfigurator.tsx
git commit -m "feat(portal-frontend): add ColumnConfigurator component"
```

---

## Task 8: Frontend — MessageModal component

**Files:**
- Create: `portal-frontend/src/pages/messages/components/MessageModal.tsx`

- [ ] **Step 1: Create component**

```tsx
// portal-frontend/src/pages/messages/components/MessageModal.tsx
import { Link } from 'react-router-dom';
import { Modal } from '../../../components/ui/Modal';
import { Button } from '../../../components/ui/Button';
import type { DetalizationMessage } from '../../../api/client';

const STATUS_LABELS: Record<string, string> = {
  pending: 'Ожидание',
  queued: 'В очереди',
  sent: 'Отправлено',
  delivered: 'Доставлено',
  failed: 'Ошибка',
  expired: 'Истекло',
  rejected: 'Отклонено',
  scheduled: 'Запланировано',
};

const STATUS_COLORS: Record<string, string> = {
  delivered: 'bg-green-100 text-green-800',
  sent: 'bg-blue-100 text-blue-800',
  failed: 'bg-red-100 text-red-800',
  rejected: 'bg-red-100 text-red-800',
  expired: 'bg-gray-100 text-gray-600',
  queued: 'bg-yellow-100 text-yellow-800',
  pending: 'bg-yellow-100 text-yellow-800',
  scheduled: 'bg-purple-100 text-purple-800',
};

function fmt(s?: string | null) {
  if (!s) return '—';
  return new Date(s).toLocaleString('ru-RU');
}

interface Props {
  message: DetalizationMessage;
  onClose: () => void;
}

export function MessageModal({ message, onClose }: Props) {
  const statusLabel = STATUS_LABELS[message.status] ?? message.status;
  const statusCls = STATUS_COLORS[message.status] ?? 'bg-gray-100 text-gray-700';

  const rows: Array<{ label: string; value: React.ReactNode }> = [
    { label: 'ID', value: <span className="font-mono text-xs break-all">{message.id}</span> },
    {
      label: 'Статус',
      value: (
        <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${statusCls}`}>
          {statusLabel}
        </span>
      ),
    },
    { label: 'Отправитель', value: message.source || '—' },
    { label: 'Получатель', value: message.destination || '—' },
    { label: 'Оператор', value: message.operator_name || '—' },
    { label: 'Канал', value: message.channel || '—' },
    {
      label: 'Текст',
      value: (
        <span className="whitespace-pre-wrap text-sm">{message.text_preview}</span>
      ),
    },
    { label: 'Дата отправки', value: fmt(message.submitted_at) },
    {
      label: 'Стоимость',
      value: message.total_amount ? `${message.total_amount} ₽` : '—',
    },
  ];

  return (
    <Modal open onClose={onClose} title="Сообщение" description="Краткая информация о сообщении">
      <div className="space-y-1 mb-4">
        {rows.map((row) => (
          <div key={row.label} className="flex gap-2 py-1 border-b border-gray-100 last:border-0">
            <span className="w-36 shrink-0 text-sm text-gray-500">{row.label}</span>
            <span className="text-sm text-gray-900">{row.value}</span>
          </div>
        ))}
      </div>

      <div className="flex justify-between items-center pt-2">
        <Button variant="secondary" onClick={onClose}>
          Закрыть
        </Button>
        <Link
          to={`/messages/${message.id}`}
          className="inline-flex items-center gap-1 text-sm text-primary hover:underline font-medium"
          onClick={onClose}
        >
          Подробнее
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
          </svg>
        </Link>
      </div>
    </Modal>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/messages/components/MessageModal.tsx
git commit -m "feat(portal-frontend): add MessageModal quick-view component"
```

---

## Task 9: Frontend — MessageFilters component

**Files:**
- Create: `portal-frontend/src/pages/messages/components/MessageFilters.tsx`

- [ ] **Step 1: Create component**

```tsx
// portal-frontend/src/pages/messages/components/MessageFilters.tsx
import { useState } from 'react';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';

export interface FilterOption {
  value: string;
  label: string;
}

export interface FilterDef {
  key: string;
  label: string;
  type: 'text' | 'select' | 'date';
  placeholder?: string;
  options?: FilterOption[];
}

interface Props {
  primary: FilterDef[];
  secondary: FilterDef[];
  values: Record<string, string>;
  onSearch: (values: Record<string, string>) => void;
  onReset: () => void;
}

export function MessageFilters({ primary, secondary, values, onSearch, onReset }: Props) {
  const [draft, setDraft] = useState<Record<string, string>>(values);
  const [showExtra, setShowExtra] = useState(false);

  // Sync when parent resets
  const handleReset = () => {
    const empty: Record<string, string> = {};
    [...primary, ...secondary].forEach((f) => { empty[f.key] = ''; });
    setDraft(empty);
    onReset();
  };

  const set = (key: string, val: string) => setDraft((prev) => ({ ...prev, [key]: val }));

  const renderField = (f: FilterDef) => {
    if (f.type === 'select') {
      return (
        <div key={f.key} className="flex flex-col gap-1">
          <label className="text-xs text-gray-500 font-medium">{f.label}</label>
          <select
            value={draft[f.key] ?? ''}
            onChange={(e) => set(f.key, e.target.value)}
            className="rounded border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
          >
            <option value="">Все</option>
            {f.options?.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </div>
      );
    }
    return (
      <div key={f.key} className="flex flex-col gap-1">
        <label className="text-xs text-gray-500 font-medium">{f.label}</label>
        <input
          type={f.type === 'date' ? 'date' : 'text'}
          value={draft[f.key] ?? ''}
          onChange={(e) => set(f.key, e.target.value)}
          placeholder={f.placeholder}
          className="rounded border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
        />
      </div>
    );
  };

  return (
    <div className="bg-white border border-gray-200 rounded-lg p-4 mb-4">
      {/* Primary filters */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-3 mb-3">
        {primary.map(renderField)}
      </div>

      {/* Secondary filters */}
      {showExtra && (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3 mb-3 pt-3 border-t border-gray-100">
          {secondary.map(renderField)}
        </div>
      )}

      {/* Actions row */}
      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={() => setShowExtra((v) => !v)}
          className="text-sm text-gray-500 hover:text-gray-700 flex items-center gap-1"
        >
          <svg
            className={`w-4 h-4 transition-transform ${showExtra ? 'rotate-180' : ''}`}
            fill="none" stroke="currentColor" viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
          {showExtra ? 'Скрыть дополнительные' : 'Дополнительные фильтры'}
        </button>

        <div className="flex gap-2">
          <Button variant="secondary" onClick={handleReset}>Сбросить</Button>
          <Button onClick={() => onSearch(draft)}>Найти</Button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/messages/components/MessageFilters.tsx
git commit -m "feat(portal-frontend): add MessageFilters component with expand/collapse"
```

---

## Task 10: Frontend — MessageTable component

**Files:**
- Create: `portal-frontend/src/pages/messages/components/MessageTable.tsx`

- [ ] **Step 1: Create component**

```tsx
// portal-frontend/src/pages/messages/components/MessageTable.tsx
import { Link } from 'react-router-dom';
import type { DetalizationMessage } from '../../../api/client';

const STATUS_LABELS: Record<string, string> = {
  pending: 'Ожидание', queued: 'В очереди', sent: 'Отправлено',
  delivered: 'Доставлено', failed: 'Ошибка', expired: 'Истекло',
  rejected: 'Отклонено', scheduled: 'Запланировано',
};
const STATUS_COLORS: Record<string, string> = {
  delivered: 'bg-green-100 text-green-800', sent: 'bg-blue-100 text-blue-800',
  failed: 'bg-red-100 text-red-800', rejected: 'bg-red-100 text-red-800',
  expired: 'bg-gray-100 text-gray-600', queued: 'bg-yellow-100 text-yellow-800',
  pending: 'bg-yellow-100 text-yellow-800', scheduled: 'bg-purple-100 text-purple-800',
};

function fmt(s?: string | null) {
  if (!s) return '—';
  return new Date(s).toLocaleString('ru-RU');
}

type SortField = 'submitted_at' | 'created_at' | 'status_at' | 'total_amount' | 'segment_count';

interface SortState { field: SortField; order: 'asc' | 'desc' }

interface Props {
  data: DetalizationMessage[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (size: number) => void;
  visibleColumns: Set<string>;
  sort: SortState;
  onSort: (field: SortField) => void;
  onRowClick: (msg: DetalizationMessage) => void;
  loading: boolean;
}

interface ColSpec {
  key: string;
  header: string;
  sortField?: SortField;
  render: (msg: DetalizationMessage) => React.ReactNode;
}

const ALL_COLUMNS: ColSpec[] = [
  { key: 'login', header: 'Логин', render: (m) => m.login || '—' },
  { key: 'destination', header: 'Номер телефона', render: (m) => m.destination },
  { key: 'operator_name', header: 'Оператор', render: (m) => m.operator_name || '—' },
  { key: 'channel', header: 'Канал', render: (m) => m.channel || '—' },
  {
    key: 'text_preview', header: 'Текст',
    render: (m) => <span className="block max-w-[200px] truncate text-sm" title={m.text_preview}>{m.text_preview}</span>,
  },
  {
    key: 'submitted_at', header: 'Дата отправки', sortField: 'submitted_at',
    render: (m) => <span className="text-xs whitespace-nowrap">{fmt(m.submitted_at)}</span>,
  },
  {
    key: 'status', header: 'Статус',
    render: (m) => (
      <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${STATUS_COLORS[m.status] ?? 'bg-gray-100 text-gray-700'}`}>
        {STATUS_LABELS[m.status] ?? m.status}
      </span>
    ),
  },
  {
    key: 'total_amount', header: 'Стоимость', sortField: 'total_amount',
    render: (m) => m.total_amount ? `${m.total_amount} ₽` : '—',
  },
  // optional columns
  { key: 'id', header: 'ID', render: (m) => <span className="font-mono text-xs">{m.id.substring(0, 8)}...</span> },
  { key: 'source', header: 'Имя отправителя', render: (m) => m.source || '—' },
  {
    key: 'created_at', header: 'Дата создания', sortField: 'created_at',
    render: (m) => <span className="text-xs whitespace-nowrap">{fmt(m.created_at)}</span>,
  },
  {
    key: 'status_at', header: 'Дата статуса', sortField: 'status_at',
    render: (m) => <span className="text-xs whitespace-nowrap">{fmt(m.delivered_at ?? m.failed_at)}</span>,
  },
  {
    key: 'segment_count', header: 'Сегменты', sortField: 'segment_count',
    render: (m) => m.segment_count,
  },
  { key: 'provider_name', header: 'Провайдер', render: (m) => m.provider_name || '—' },
  { key: 'country_name', header: 'Страна', render: (m) => m.country_name || '—' },
  { key: 'send_method', header: 'Способ отправки', render: (m) => m.send_method || '—' },
];

function SortIcon({ active, order }: { active: boolean; order: 'asc' | 'desc' }) {
  return (
    <svg className={`w-3 h-3 inline ml-1 ${active ? 'text-primary' : 'text-gray-300'}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {order === 'asc' || !active
        ? <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
        : <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
      }
    </svg>
  );
}

export function MessageTable({
  data, total, page, pageSize, onPageChange, onPageSizeChange,
  visibleColumns, sort, onSort, onRowClick, loading,
}: Props) {
  const cols = ALL_COLUMNS.filter((c) => visibleColumns.has(c.key));
  const totalPages = Math.ceil(total / pageSize);

  return (
    <div>
      <div className="overflow-x-auto rounded-lg border border-gray-200">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-gray-50 border-b border-gray-200">
              {cols.map((col) => (
                <th
                  key={col.key}
                  className={`text-left px-3 py-2 text-xs font-medium text-gray-500 whitespace-nowrap ${col.sortField ? 'cursor-pointer hover:text-gray-900 select-none' : ''}`}
                  onClick={col.sortField ? () => onSort(col.sortField!) : undefined}
                >
                  {col.header}
                  {col.sortField && (
                    <SortIcon
                      active={sort.field === col.sortField}
                      order={sort.field === col.sortField ? sort.order : 'desc'}
                    />
                  )}
                </th>
              ))}
              <th className="px-3 py-2 text-xs font-medium text-gray-500 text-right">
                <span className="sr-only">Действия</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr>
                <td colSpan={cols.length + 1} className="text-center py-8 text-gray-400 text-sm">
                  Загрузка...
                </td>
              </tr>
            )}
            {!loading && data.length === 0 && (
              <tr>
                <td colSpan={cols.length + 1} className="text-center py-12 text-gray-400 text-sm">
                  Сообщений не найдено
                </td>
              </tr>
            )}
            {!loading && data.map((msg) => (
              <tr
                key={msg.id}
                onClick={() => onRowClick(msg)}
                className="border-b border-gray-100 hover:bg-gray-50 cursor-pointer"
              >
                {cols.map((col) => (
                  <td key={col.key} className="px-3 py-2 text-gray-800">
                    {col.render(msg)}
                  </td>
                ))}
                <td className="px-3 py-2 text-right">
                  <Link
                    to={`/messages/${msg.id}`}
                    onClick={(e) => e.stopPropagation()}
                    className="text-xs text-primary hover:underline whitespace-nowrap"
                    title="Открыть детальную страницу"
                  >
                    Детали →
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <div className="flex items-center justify-between mt-3">
        <div className="flex items-center gap-2 text-sm text-gray-500">
          <span>Строк на странице:</span>
          <select
            value={pageSize}
            onChange={(e) => onPageSizeChange(Number(e.target.value))}
            className="border border-gray-300 rounded px-1 py-0.5 text-sm"
          >
            <option value={20}>20</option>
            <option value={50}>50</option>
            <option value={100}>100</option>
          </select>
        </div>

        <div className="flex items-center gap-1">
          <button
            disabled={page <= 1}
            onClick={() => onPageChange(page - 1)}
            className="px-2 py-1 rounded border border-gray-200 text-sm disabled:opacity-40 hover:bg-gray-50"
          >
            ←
          </button>
          <span className="px-3 py-1 text-sm text-gray-600">
            {page} / {totalPages || 1}
          </span>
          <button
            disabled={page >= totalPages}
            onClick={() => onPageChange(page + 1)}
            className="px-2 py-1 rounded border border-gray-200 text-sm disabled:opacity-40 hover:bg-gray-50"
          >
            →
          </button>
        </div>
      </div>
    </div>
  );
}

export { ALL_COLUMNS };
export type { SortField, SortState, ColSpec };
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/messages/components/MessageTable.tsx
git commit -m "feat(portal-frontend): add MessageTable with configurable columns and sorting"
```

---

## Task 11: Frontend — New MessagesPage

**Files:**
- Modify: `portal-frontend/src/pages/messages/MessagesPage.tsx` (full rewrite)

- [ ] **Step 1: Rewrite MessagesPage.tsx**

```tsx
// portal-frontend/src/pages/messages/MessagesPage.tsx
import { useCallback, useEffect, useRef, useState } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { detalizationApi, exportApi, referencesApi, senderNamesApi } from '../../api/client';
import type { DetalizationMessage } from '../../api/client';
import { MessageFilters } from './components/MessageFilters';
import type { FilterDef } from './components/MessageFilters';
import { ActiveFilterChips } from './components/ActiveFilterChips';
import { MessageTable, ALL_COLUMNS } from './components/MessageTable';
import type { SortField, SortState } from './components/MessageTable';
import { MessageModal } from './components/MessageModal';
import { ColumnConfigurator } from './components/ColumnConfigurator';
import type { ColumnDef } from './components/ColumnConfigurator';

const STORAGE_KEY = 'messages_visible_columns';
const DEFAULT_VISIBLE = new Set([
  'login', 'destination', 'operator_name', 'channel',
  'text_preview', 'submitted_at', 'status', 'total_amount',
]);

function loadVisibleColumns(): Set<string> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return new Set(JSON.parse(raw) as string[]);
  } catch { /* ignore */ }
  return new Set(DEFAULT_VISIBLE);
}

function saveVisibleColumns(cols: Set<string>) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify([...cols]));
  } catch { /* ignore */ }
}

const STATUS_OPTIONS = [
  { value: 'pending', label: 'Ожидание' },
  { value: 'queued', label: 'В очереди' },
  { value: 'sent', label: 'Отправлено' },
  { value: 'delivered', label: 'Доставлено' },
  { value: 'failed', label: 'Ошибка' },
  { value: 'expired', label: 'Истекло' },
  { value: 'rejected', label: 'Отклонено' },
  { value: 'scheduled', label: 'Запланировано' },
];

const CHANNEL_OPTIONS = [
  { value: 'SMS', label: 'SMS' },
  { value: 'MAX', label: 'MAX' },
  { value: 'Viber', label: 'Viber' },
];

const SEND_METHOD_OPTIONS = [
  { value: 'PORTAL', label: 'ЛК' },
  { value: 'API', label: 'API' },
  { value: 'SMPP', label: 'SMPP' },
];

const EMPTY_FILTERS: Record<string, string> = {
  date_from: '', date_to: '', destination: '', login: '',
  status: '', operator: '', sender_name: '', channel: '',
  message_id: '', send_method: '', country: '',
};

export function MessagesPage() {
  const [data, setData] = useState<{ messages: DetalizationMessage[]; total: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [appliedFilters, setAppliedFilters] = useState<Record<string, string>>(EMPTY_FILTERS);
  const [sort, setSort] = useState<SortState>({ field: 'submitted_at', order: 'desc' });
  const [selectedMsg, setSelectedMsg] = useState<DetalizationMessage | null>(null);
  const [visibleColumns, setVisibleColumns] = useState<Set<string>>(loadVisibleColumns);

  // Export
  const [exportJobId, setExportJobId] = useState<string | null>(null);
  const [exportStatus, setExportStatus] = useState<string | null>(null);
  const exportPollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Reference data for dropdowns
  const [operatorOptions, setOperatorOptions] = useState<Array<{ value: string; label: string }>>([]);
  const [countryOptions, setCountryOptions] = useState<Array<{ value: string; label: string }>>([]);
  const [senderNameOptions, setSenderNameOptions] = useState<Array<{ value: string; label: string }>>([]);

  useEffect(() => {
    referencesApi.operators().then((r) =>
      setOperatorOptions(r.operators.map((o) => ({ value: o.name, label: o.name })))
    ).catch(() => {});
    referencesApi.countries().then((r) =>
      setCountryOptions(r.countries.map((c) => ({ value: c.name, label: c.name })))
    ).catch(() => {});
    senderNamesApi.listApproved().then((r) => {
      const sns = (r as { sender_names: Array<{ name: string }> }).sender_names ?? [];
      setSenderNameOptions(sns.map((s) => ({ value: s.name, label: s.name })));
    }).catch(() => {});
  }, []);

  const primaryFilters: FilterDef[] = [
    { key: 'date_from', label: 'Дата от', type: 'date' },
    { key: 'date_to', label: 'Дата до', type: 'date' },
    { key: 'destination', label: 'Номер', type: 'text', placeholder: '+7...' },
    { key: 'login', label: 'Логин', type: 'text', placeholder: 'Суб-аккаунт' },
    { key: 'status', label: 'Статус', type: 'select', options: STATUS_OPTIONS },
    { key: 'operator', label: 'Оператор', type: 'select', options: operatorOptions },
    { key: 'sender_name', label: 'Имя отправителя', type: 'select', options: senderNameOptions },
    { key: 'channel', label: 'Канал', type: 'select', options: CHANNEL_OPTIONS },
  ];

  const secondaryFilters: FilterDef[] = [
    { key: 'message_id', label: 'ID сообщения', type: 'text', placeholder: 'Поиск по ID' },
    { key: 'send_method', label: 'Способ отправки', type: 'select', options: SEND_METHOD_OPTIONS },
    { key: 'country', label: 'Страна', type: 'select', options: countryOptions },
  ];

  const allFilterDefs = [...primaryFilters, ...secondaryFilters];

  const fetchData = useCallback(() => {
    setLoading(true);
    setError(null);
    detalizationApi.list({
      ...Object.fromEntries(Object.entries(appliedFilters).filter(([, v]) => v !== '')),
      limit: pageSize,
      offset: (page - 1) * pageSize,
      sort_by: sort.field,
      sort_order: sort.order,
    }).then((r) => setData({ messages: r.messages, total: r.total }))
      .catch((err) => setError(err.message || 'Ошибка загрузки'))
      .finally(() => setLoading(false));
  }, [appliedFilters, page, pageSize, sort]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleSearch = (values: Record<string, string>) => {
    setAppliedFilters(values);
    setPage(1);
  };

  const handleReset = () => {
    setAppliedFilters(EMPTY_FILTERS);
    setPage(1);
  };

  const handleRemoveChip = (key: string) => {
    const next = { ...appliedFilters, [key]: '' };
    setAppliedFilters(next);
    setPage(1);
  };

  const handleSort = (field: SortField) => {
    setSort((prev) =>
      prev.field === field
        ? { field, order: prev.order === 'asc' ? 'desc' : 'asc' }
        : { field, order: 'desc' }
    );
    setPage(1);
  };

  const handleColumnsChange = (cols: Set<string>) => {
    setVisibleColumns(cols);
    saveVisibleColumns(cols);
  };

  const handleExport = useCallback(async () => {
    setExportStatus('pending');
    const filters: Record<string, string> = {};
    Object.entries(appliedFilters).forEach(([k, v]) => { if (v) filters[k] = v; });
    try {
      const { job_id } = await exportApi.start(filters);
      setExportJobId(job_id);
      exportPollRef.current = setInterval(async () => {
        const job = await exportApi.getStatus(job_id);
        setExportStatus(job.status);
        if (job.status === 'ready') {
          clearInterval(exportPollRef.current!);
          const res = await exportApi.download(job_id);
          const blob = await res.blob();
          const url = URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = `messages_export_${job_id.slice(0, 8)}.csv`;
          a.click();
          URL.revokeObjectURL(url);
          setExportStatus(null);
          setExportJobId(null);
        } else if (job.status === 'error') {
          clearInterval(exportPollRef.current!);
          setExportStatus(null);
        }
      }, 2000);
    } catch { setExportStatus(null); }
  }, [appliedFilters]);

  const columnDefs: ColumnDef[] = ALL_COLUMNS.map((c) => ({
    key: c.key,
    label: c.header,
    defaultVisible: DEFAULT_VISIBLE.has(c.key),
  }));

  return (
    <div>
      <PageHeader
        title="Сообщения"
        subtitle="Детализация трафика по всем клиентам и каналам"
      />

      <MessageFilters
        primary={primaryFilters}
        secondary={secondaryFilters}
        values={appliedFilters}
        onSearch={handleSearch}
        onReset={handleReset}
      />

      <ActiveFilterChips
        filterDefs={allFilterDefs}
        values={appliedFilters}
        onRemove={handleRemoveChip}
      />

      {error && <div className="text-red-600 mb-3 text-sm">Ошибка: {error}</div>}

      {/* Table toolbar */}
      <div className="flex items-center justify-between mb-2">
        <span className="text-sm text-gray-500">
          {!loading && data != null && (
            <>Найдено: <strong>{data.total}</strong> сообщений</>
          )}
        </span>
        <div className="flex gap-2">
          {exportStatus && exportStatus !== 'ready' && (
            <span className="text-sm text-gray-400 self-center">Экспорт...</span>
          )}
          <Button
            variant="secondary"
            onClick={handleExport}
            disabled={!!exportJobId || (data?.total ?? 0) === 0}
          >
            Экспорт
          </Button>
          <ColumnConfigurator
            columns={columnDefs}
            visible={visibleColumns}
            onChange={handleColumnsChange}
          />
        </div>
      </div>

      <MessageTable
        data={data?.messages ?? []}
        total={data?.total ?? 0}
        page={page}
        pageSize={pageSize}
        onPageChange={setPage}
        onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
        visibleColumns={visibleColumns}
        sort={sort}
        onSort={handleSort}
        onRowClick={setSelectedMsg}
        loading={loading}
      />

      {selectedMsg && (
        <MessageModal message={selectedMsg} onClose={() => setSelectedMsg(null)} />
      )}
    </div>
  );
}
```

- [ ] **Step 2: Check that `PageHeader` accepts a `subtitle` prop**

```bash
grep -n "subtitle" c:/projects/sms/portal-frontend/src/components/layout/PageHeader.tsx
```

If no `subtitle` prop exists, look at PageHeader interface and add support:

```bash
cat c:/projects/sms/portal-frontend/src/components/layout/PageHeader.tsx
```

If `subtitle` is not supported, add it to the PageHeader component:
- Find the `interface` for `PageHeader` props and add `subtitle?: string`
- In the JSX, render `{subtitle && <p className="text-sm text-muted-foreground mt-0.5">{subtitle}</p>}` below the title

- [ ] **Step 3: Check that `senderNamesApi.listApproved` exists**

```bash
grep -n "listApproved" c:/projects/sms/portal-frontend/src/api/client.ts
```

Expected: should find it (line ~461). If function name differs, update the import in MessagesPage.tsx.

- [ ] **Step 4: TypeScript check**

```bash
cd c:/projects/sms/portal-frontend && npx tsc --noEmit 2>&1 | head -40
```

Fix any type errors.

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/messages/MessagesPage.tsx
git add portal-frontend/src/pages/messages/components/
git add portal-frontend/src/components/layout/PageHeader.tsx
git commit -m "feat(portal-frontend): new MessagesPage with advanced filters, configurable table, and export"
```

---

## Task 12: Frontend — Update routing and navigation

**Files:**
- Modify: `portal-frontend/src/App.tsx`
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`

- [ ] **Step 1: Remove `/detalization` route from App.tsx**

In `App.tsx`:
1. Remove the import: `import { DetalizationPage } from './pages/detalization/DetalizationPage';`
2. Remove the route: `<Route path="/detalization" element={<DetalizationPage />} />`

- [ ] **Step 2: Remove "Детализация" from UserLayout sidebar**

In `UserLayout.tsx`, in the `NAV_GROUPS` array, find the group "Отследить" and remove the `{ path: '/detalization', label: 'Детализация' }` item:

Before:
```ts
{
  label: 'Отследить',
  items: [
    { path: '/messages', label: 'Сообщения' },
    { path: '/detalization', label: 'Детализация' },
    { path: '/cascade/history', label: 'История каскадов' },
  ],
},
```

After:
```ts
{
  label: 'Отследить',
  items: [
    { path: '/messages', label: 'Сообщения' },
    { path: '/cascade/history', label: 'История каскадов' },
  ],
},
```

- [ ] **Step 3: TypeScript check**

```bash
cd c:/projects/sms/portal-frontend && npx tsc --noEmit 2>&1 | head -20
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/App.tsx portal-frontend/src/components/layout/UserLayout.tsx
git commit -m "feat(portal-frontend): update routing and navigation for merged messages page"
```

---

## Task 13: Cleanup — Delete old files

**Files:**
- Delete: `portal-frontend/src/pages/detalization/DetalizationPage.tsx`
- Delete (if empty): `portal-frontend/src/pages/detalization/` directory

- [ ] **Step 1: Delete old files**

```bash
rm c:/projects/sms/portal-frontend/src/pages/detalization/DetalizationPage.tsx
rmdir c:/projects/sms/portal-frontend/src/pages/detalization/ 2>/dev/null || true
```

- [ ] **Step 2: Verify no remaining imports**

```bash
grep -r "DetalizationPage\|/detalization" c:/projects/sms/portal-frontend/src --include="*.tsx" --include="*.ts"
```

Expected: no matches (or only admin detalization which is intentionally kept).

- [ ] **Step 3: Final TypeScript check**

```bash
cd c:/projects/sms/portal-frontend && npx tsc --noEmit 2>&1 | head -20
```

Expected: no errors.

- [ ] **Step 4: Build frontend**

```bash
cd c:/projects/sms/portal-frontend && npm run build 2>&1 | tail -20
```

Expected: build succeeds.

- [ ] **Step 5: Build backend**

```bash
cd c:/projects/sms && go build ./...
```

Expected: no errors.

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "chore(portal-frontend): remove obsolete DetalizationPage"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|-----------------|------|
| Переименовать "Детализация" → "Сообщения" | Task 12 (routing), Task 11 (PageHeader) |
| Подзаголовок text-muted "Детализация трафика..." | Task 11 (PageHeader subtitle) |
| Фильтры основные (Номер, Логин, Статус, Оператор, Имя отправителя, Канал) | Task 9 |
| Фильтры дополнительные (ID, Способ отправки, Страна) | Task 9 |
| Даты в основных фильтрах | Task 9 |
| Активные фильтры → чипсы | Task 6, Task 11 |
| Кнопки "Найти" и "Сбросить" | Task 9 |
| Кнопка "Экспорт" | Task 11 |
| Пагинация по умолчанию 20 строк | Task 10, Task 11 |
| Столбцы таблицы по умолчанию | Task 10 |
| Кнопка "Настройка таблицы" | Task 7, Task 11 |
| "Найдено: n сообщений" | Task 11 |
| Сортировка по столбцам | Task 10, Task 4 |
| Модалка с кратким обзором | Task 8 |
| Ссылка "Подробнее" в модалке | Task 8 |
| Ссылка "Детали →" в таблице | Task 10 |
| Бэкенд новые поля и фильтры | Tasks 1, 4 |
| Справочники операторов и стран | Tasks 2, 3 |
| Убрать старую страницу "Сообщения" (отправка SMS) | Task 12 (old MessagesPage replaced), Task 11 |
| Убрать "Детализация" из навигации | Task 12 |

All spec requirements are covered. ✓
