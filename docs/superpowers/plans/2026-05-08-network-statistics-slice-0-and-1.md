# Network Statistics — Slice 0 (Diagnostic Hotfix) + Slice 1 (Выручка) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop the UI from rendering misleading `0 ₽` and unitless «Ошибки 0,17», fix the 500-on-template-DELETE bug, then make `network_stats_hourly.revenue` flow from `tarification_log.total_amount` and backfill April-now.

**Architecture:** Slice 0 lands a small UI/API rendering fix that does NOT touch data sources — only stops the visual lying. Slice 1 connects the SQL aggregator to `tarification_log` via JOIN, switches `UpsertHourlyStats` to replace-semantics so the backfill is idempotent, and runs a one-shot CLI to backfill historical revenue. Spec: [docs/superpowers/specs/2026-05-08-network-statistics-overhaul-design.md](docs/superpowers/specs/2026-05-08-network-statistics-overhaul-design.md).

**Plan revision history:**
- 2026-05-08 v2: removed original Tasks 1–4 (D2 regression bisect, point-fix, reseed CLI, reseed run). Subagent static analysis + sandbox SQL evidence proved the audit's "regression" hypothesis was wrong — 250/271 "missing tarifications" turned out to be a SQL-injected test seed from manual testing on 2026-05-04 (not real traffic). Real pipeline tarifies at 100%. See spec section 1 item 1(в) and the conversation log. Old Tasks 5–11 renumbered to 1–7.

**Tech Stack:** Go 1.24 (gRPC, pgx, sqlx for legacy repo), PostgreSQL 15+ with monthly partitioning, React 19 + TypeScript 5.7 + Vite + Tailwind, stretchr/testify for Go tests, Vitest + React Testing Library for frontend.

---

## File Structure

### Created files

- `cmd/network-stats-backfill/main.go` — CLI that calls `aggregation_worker.BackfillWindow(from, to)`. Optional `--truncate` (with stdin confirmation) for blank-slate backfill, `--partner` for narrowed scope.
- `scripts/run-backfill-april.sh` — wrapper that invokes `network-stats-backfill --from 2026-04-01T00:00:00Z --to now --truncate` after stdin confirmation.
- `internal/services/network_analytics/domain/saved_view_errors.go` — sentinel errors `ErrViewNotFound`, `ErrViewIsTemplate`, `ErrViewForbidden`.
- New regression tests as listed per task.

### Modified files

- `internal/services/network_analytics/application/aggregation_worker.go` — `rawAggQuery` gets a `LEFT JOIN tarification_log` and `COALESCE(SUM(t.total_amount), 0) AS revenue`.
- `internal/services/network_analytics/infrastructure/repository/stats_repository.go` — `UpsertHourlyStats` switches counter columns from `add` to `replace` semantics.
- `internal/services/network_analytics/infrastructure/repository/views_repository.go` — `Delete` returns sentinel errors instead of swallowing `RowsAffected=0`.
- `internal/gateway/portal/handlers/network_statistics.go` — `DeleteView` handler maps sentinel errors to 403/404.
- `internal/services/network_analytics/domain/models.go` — `KPI` struct gains `Format string`, `Currency string` fields.
- `internal/services/network_analytics/application/service.go` — `buildStatKPIs` populates `Format` and skips `Value` for empty money KPIs.
- `internal/services/network_analytics/grpc/server.go` (and proto) — KPI proto carries `format` and `currency` fields.
- `api/proto/network_analytics/network_analytics.proto` + regen `*.pb.go`.
- `portal-frontend/src/pages/network/NetworkStatisticsPage.tsx` and `NetworkAnalyticsPage.tsx` — KPI renderer reads `format`, renders `—` when `value` is missing.
- `portal-frontend/src/api/networkStats.ts` — TS types for KPI gain `format?: 'count' | 'percent' | 'currency'`, `currency?: string`, `value?: number`.

---

## SLICE 0 — Diagnostic Hotfix (0.3 day) — F1 + F2 + F3 only

After the 2026-05-08 fact-check, original Tasks 1–4 (D2 regression bisect, point-fix, reseed CLI, reseed run) were removed because the audit's "regression" hypothesis was disproven. Slice 0 now lands three small UI/API rendering fixes only.

### Task 1: Add sentinel errors and 403/404 mapping for `DeleteView`

**Files:**
- Create: `internal/services/network_analytics/domain/saved_view_errors.go`
- Modify: `internal/services/network_analytics/infrastructure/repository/views_repository.go`
- Modify: `internal/gateway/portal/handlers/network_statistics.go` (search for `DeleteView` handler — file already known from spec).
- Test: `internal/services/network_analytics/infrastructure/repository/views_repository_test.go`

- [ ] **Step 1: Define sentinel errors**

Create `internal/services/network_analytics/domain/saved_view_errors.go`:

```go
package domain

import "errors"

// ErrViewNotFound is returned when a saved view with the given id does not
// exist for the requesting partner.
var ErrViewNotFound = errors.New("saved view not found")

// ErrViewIsTemplate is returned when the requested operation is forbidden
// because the view is a system template (is_template=true). Templates are
// shared across all partners and immutable; clone via /views/{id}/clone.
var ErrViewIsTemplate = errors.New("saved view is a system template, clone instead")

// ErrViewForbidden is returned when the view exists but belongs to a
// different (partner_id, user_id) than the requester.
var ErrViewForbidden = errors.New("saved view access forbidden")
```

- [ ] **Step 2: Write the failing test for `Delete` semantics**

Open `internal/services/network_analytics/infrastructure/repository/views_repository_test.go` (create if absent) and add:

```go
func TestViewsRepo_Delete_TemplateReturnsErrViewIsTemplate(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.NewViewsRepo(pool)

	// Pick the seeded template id=1 (Владелец — полный обзор), is_template=true, user_id IS NULL.
	err = repo.Delete(context.Background(), 1, /*partner=*/51, /*user=*/42)
	require.ErrorIs(t, err, domain.ErrViewIsTemplate)
}

func TestViewsRepo_Delete_NonexistentReturnsErrViewNotFound(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.NewViewsRepo(pool)

	err = repo.Delete(context.Background(), 999999, 51, 42)
	require.ErrorIs(t, err, domain.ErrViewNotFound)
}

func TestViewsRepo_Delete_OtherPartnerReturnsErrViewForbidden(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.NewViewsRepo(pool)

	// Seed a user-view owned by partner=99, user=1.
	created, err := repo.Save(context.Background(), &domain.SavedView{
		PartnerID: 99, UserID: 1, Name: "test-foreign",
		Mode: "analytics", Filters: "{}", SortDir: "desc",
		Columns: []string{"slice", "total"},
	})
	require.NoError(t, err)
	defer pool.Exec(context.Background(), "DELETE FROM saved_views WHERE id=$1", created.ID)

	// Try to delete from partner=51 — must be forbidden.
	err = repo.Delete(context.Background(), created.ID, 51, 42)
	require.ErrorIs(t, err, domain.ErrViewForbidden)
}
```

- [ ] **Step 3: Run the tests to verify they FAIL**

Run: `cd internal/services/network_analytics/infrastructure/repository && TEST_DB_DSN=... go test -run TestViewsRepo_Delete -v`

Expected: all three FAIL with current `Delete` returning either `nil` (on RowsAffected=0) or generic SQL errors.

- [ ] **Step 4: Modify `views_repository.Delete` to differentiate cases**

Replace the existing `Delete` method body in `internal/services/network_analytics/infrastructure/repository/views_repository.go` with:

```go
// Delete removes a saved view, returning a sentinel error when the view
// is missing, is a system template, or belongs to a different owner.
func (r *ViewsRepo) Delete(ctx context.Context, id, partnerID, userID int64) error {
	// First, look up the view to differentiate not-found / template / forbidden.
	var (
		isTemplate bool
		ownerPartner sql.NullInt64
		ownerUser sql.NullInt64
	)
	err := r.db.QueryRow(ctx, `
		SELECT is_template, partner_id, user_id
		FROM saved_views
		WHERE id = $1`, id).Scan(&isTemplate, &ownerPartner, &ownerUser)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrViewNotFound
		}
		return fmt.Errorf("lookup view %d: %w", id, err)
	}
	if isTemplate {
		return domain.ErrViewIsTemplate
	}
	if !ownerPartner.Valid || ownerPartner.Int64 != partnerID || !ownerUser.Valid || ownerUser.Int64 != userID {
		return domain.ErrViewForbidden
	}
	// All checks passed — perform the actual delete.
	_, err = r.db.Exec(ctx, `DELETE FROM saved_views WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete view %d: %w", id, err)
	}
	return nil
}
```

Adjust imports: ensure `database/sql`, `github.com/jackc/pgx/v5`, and the `domain` package are imported.

- [ ] **Step 5: Run the tests to verify they PASS**

Run: `cd internal/services/network_analytics/infrastructure/repository && TEST_DB_DSN=... go test -run TestViewsRepo_Delete -v`

Expected: all three PASS.

- [ ] **Step 6: Update the gRPC layer to propagate sentinel errors**

Find the gRPC `DeleteView` method in `internal/services/network_analytics/grpc/server.go`. Map domain errors to gRPC codes:

```go
func (s *Server) DeleteView(ctx context.Context, req *networkanalyticsv1.DeleteViewRequest) (*networkanalyticsv1.DeleteViewResponse, error) {
	if err := s.svc.DeleteView(ctx, req.Id, req.PartnerId, req.UserId); err != nil {
		switch {
		case errors.Is(err, domain.ErrViewNotFound):
			return nil, status.Errorf(codes.NotFound, "view not found")
		case errors.Is(err, domain.ErrViewIsTemplate):
			return nil, status.Errorf(codes.PermissionDenied, "system template — clone instead")
		case errors.Is(err, domain.ErrViewForbidden):
			return nil, status.Errorf(codes.PermissionDenied, "access forbidden")
		default:
			return nil, status.Errorf(codes.Internal, "delete view: %v", err)
		}
	}
	return &networkanalyticsv1.DeleteViewResponse{}, nil
}
```

- [ ] **Step 7: Update the HTTP handler in `network_statistics.go` to map gRPC codes to HTTP**

Find the `DeleteView` handler (will be near other `/views` handlers in `internal/gateway/portal/handlers/network_statistics.go`). Map gRPC codes to HTTP status:

```go
func (h *NetworkStatisticsHandler) DeleteView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid view id")
		return
	}
	partnerID, userID, err := h.resolveSession(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}
	_, err = h.client.DeleteView(r.Context(), &networkanalyticsv1.DeleteViewRequest{
		Id: id, PartnerId: partnerID, UserId: userID,
	})
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.NotFound:
			writeError(w, http.StatusNotFound, "VIEW_NOT_FOUND", "view not found")
		case codes.PermissionDenied:
			// Differentiate template vs ownership — message conveys which.
			if strings.Contains(st.Message(), "template") {
				writeError(w, http.StatusForbidden, "VIEW_IS_TEMPLATE",
					"Системный пресет нельзя удалить, можно клонировать")
			} else {
				writeError(w, http.StatusForbidden, "VIEW_FORBIDDEN", "access forbidden")
			}
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Внутренняя ошибка сервера")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

(Adjust `writeError` and `resolveSession` to match existing helpers in the handlers package — these names mimic typical patterns and the engineer should align with what's already there.)

- [ ] **Step 8: Run the API integration test against sandbox**

A simple curl-based check:
```bash
COOKIE=$(curl -s -c - -X POST 'http://72.56.232.202:18085/portal/v1/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"email":"aggregator@test.local","password":"Admin123!"}' \
  | grep session_token | awk '{print $7}')

curl -i -X DELETE 'http://72.56.232.202:18085/portal/v1/reseller/views/1' \
  -H "Cookie: session_token=$COOKIE"
```

Expected: `HTTP/1.1 403 Forbidden` with body `{"error":{"code":"VIEW_IS_TEMPLATE","message":"Системный пресет нельзя удалить, можно клонировать"}}`.

```bash
curl -i -X DELETE 'http://72.56.232.202:18085/portal/v1/reseller/views/9999999' \
  -H "Cookie: session_token=$COOKIE"
```

Expected: `HTTP/1.1 404 Not Found` with body `{"error":{"code":"VIEW_NOT_FOUND","message":"view not found"}}`.

- [ ] **Step 9: Commit**

```bash
git add internal/services/network_analytics/domain/saved_view_errors.go \
        internal/services/network_analytics/infrastructure/repository/views_repository.go \
        internal/services/network_analytics/infrastructure/repository/views_repository_test.go \
        internal/services/network_analytics/grpc/server.go \
        internal/gateway/portal/handlers/network_statistics.go
git commit -m "fix(network-analytics): proper status codes for DeleteView

Previously DELETE /reseller/views/{id} returned 500 INTERNAL_ERROR for
any non-happy path (template, foreign owner, missing). Now:
- missing → 404 VIEW_NOT_FOUND
- template → 403 VIEW_IS_TEMPLATE with clone hint
- foreign owner → 403 VIEW_FORBIDDEN

Refs: docs/superpowers/specs/2026-05-08-network-statistics-overhaul-design.md#f1"
```

---

### Task 2: Add `Format` field to KPI proto/domain and populate it on backend

**Files:**
- Modify: `api/proto/network_analytics/network_analytics.proto`
- Regenerate: `api/proto/networkanalyticsv1/network_analytics.pb.go`
- Modify: `internal/services/network_analytics/domain/models.go` (KPI struct)
- Modify: `internal/services/network_analytics/application/service.go` (`buildStatKPIs`, `buildMonitoringKPIs`)
- Modify: `internal/services/network_analytics/infrastructure/repository/stats_repository.go` (`computeKPIs`)
- Modify: `internal/services/network_analytics/grpc/server.go` (KPI marshaling)

- [ ] **Step 1: Add `format` and `currency` to the proto KPI message**

Edit `api/proto/network_analytics/network_analytics.proto`. Find the KPI message definition (likely `message KPI` or `message Kpi`) and add fields:

```proto
message KPI {
  string name = 1;
  optional double value = 2;     // already optional or use wrapper; if currently not optional, switch
  string status = 3;
  optional double delta = 4;
  string format = 5;             // new: "count", "percent", "currency"
  string currency = 6;           // new: "RUB", populated only when format=="currency"
}
```

(If KPI's `value` is currently `double value = 2;` without `optional`, change to `optional double value = 2;` so absence is distinguishable from zero.)

- [ ] **Step 2: Regenerate Go protobuf code**

Run: `cd c:/projects/sms && ./scripts/proto-gen.sh` (or whatever the project's proto-gen script is; check `Makefile` or `scripts/`). If no script exists, run:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/network_analytics/network_analytics.proto
```

Expected: `api/proto/networkanalyticsv1/network_analytics.pb.go` regenerated with new fields.

- [ ] **Step 3: Update domain `KPI` struct**

Edit `internal/services/network_analytics/domain/models.go`. Find the `KPI` struct, add fields:

```go
type KPI struct {
	Name     string  `json:"name"`
	Value    *float64 `json:"value,omitempty"`  // pointer so nil ⇒ omitted in JSON
	Status   string  `json:"status,omitempty"`
	Delta    float64 `json:"delta,omitempty"`
	Format   string  `json:"format,omitempty"`   // "count" | "percent" | "currency"
	Currency string  `json:"currency,omitempty"`
}
```

(Switching `Value` to `*float64` is a breaking-shape change — all KPI builders below must use `floatPtr()` helper or assign `&v`.)

Add a helper at the bottom of `models.go`:

```go
// f64p returns a pointer to f. Used for KPI.Value when the metric has a value;
// pass nil literally when the metric has no data and UI should render "—".
func f64p(f float64) *float64 { return &f }
```

- [ ] **Step 4: Update `buildStatKPIs` in `service.go`**

Replace the existing `buildStatKPIs` body in `internal/services/network_analytics/application/service.go:63-110` with:

```go
func buildStatKPIs(rows []domain.StatRow) []domain.KPI {
	var totalMsgs, delivered, failed, pending, timeout int64
	var revenue, cost float64

	for _, r := range rows {
		totalMsgs += r.Total
		delivered += r.Delivered
		failed += r.Failed
		pending += r.Pending
		timeout += r.Timeout
		revenue += r.Revenue
		cost += r.Cost
	}

	dlrRate := float64(0)
	if totalMsgs > 0 {
		dlrRate = float64(delivered) / float64(totalMsgs)
	}
	errorRate := float64(0)
	if totalMsgs > 0 {
		errorRate = float64(failed+timeout) / float64(totalMsgs)
	}
	profit := revenue - cost

	dlrStatus := domain.HealthOK
	if dlrRate < domain.DLRRateDanger {
		dlrStatus = domain.HealthDanger
	} else if dlrRate < domain.DLRRateWarning {
		dlrStatus = domain.HealthWarning
	}
	errStatus := domain.HealthOK
	if errorRate > domain.ErrorRateDanger {
		errStatus = domain.HealthDanger
	} else if errorRate > domain.ErrorRateWarning {
		errStatus = domain.HealthWarning
	}

	// Money KPIs: when revenue == 0 we treat it as "no data" (Slice 0/1 truth:
	// either tarification didn't run, or the period has no traffic). Show
	// nil value so UI renders "—" instead of a misleading "0 ₽ green".
	moneyValue := func(v float64) *float64 {
		if v == 0 {
			return nil
		}
		return &v
	}

	totalVal := float64(totalMsgs)
	pendingVal := float64(pending)

	return []domain.KPI{
		{Name: "Всего", Value: &totalVal, Status: domain.HealthOK, Format: "count"},
		{Name: "Доставляемость", Value: &dlrRate, Status: dlrStatus, Format: "percent"},
		{Name: "Ошибки", Value: &errorRate, Status: errStatus, Format: "percent"},
		{Name: "Выручка", Value: moneyValue(revenue), Status: domain.HealthOK, Format: "currency", Currency: "RUB"},
		{Name: "Себестоимость", Value: moneyValue(cost), Status: domain.HealthOK, Format: "currency", Currency: "RUB"},
		{Name: "Прибыль", Value: moneyValue(profit), Status: domain.HealthOK, Format: "currency", Currency: "RUB"},
		{Name: "Pending", Value: &pendingVal, Status: domain.HealthOK, Format: "count"},
	}
}
```

- [ ] **Step 5: Update `computeKPIs` in `stats_repository.go:266` similarly**

Same pattern — convert `Value` to `*float64`, add `Format`, treat zero money as `nil`. The function signature stays the same.

- [ ] **Step 6: Update `buildMonitoringKPIs` in `service.go:113`**

Add `Format` to each KPI: `throughput` → `count` (msg/s; for Slice 4 we'll add a `rate` format, for now use `count` with the unit baked into rendering), `dlr_rate` → `percent`, `pending`/`errors`/`timeouts`/`unhealthy_providers` → `count`.

- [ ] **Step 7: Update gRPC server marshaling to forward new fields**

In `internal/services/network_analytics/grpc/server.go`, find where domain KPIs are mapped to proto KPIs (likely a helper like `kpiToProto`). Add:

```go
func kpiToProto(k domain.KPI) *networkanalyticsv1.KPI {
	out := &networkanalyticsv1.KPI{
		Name:     k.Name,
		Status:   k.Status,
		Delta:    proto.Float64(k.Delta),
		Format:   k.Format,
		Currency: k.Currency,
	}
	if k.Value != nil {
		out.Value = proto.Float64(*k.Value)
	}
	return out
}
```

- [ ] **Step 8: Update `network_statistics.go` HTTP handler to forward new fields to JSON**

Find where the gRPC response is mapped to JSON. Ensure `format`, `currency`, and the optional `value` are forwarded. The TS DTO will pick them up via `value?: number` in next task.

- [ ] **Step 9: Write a unit test for `buildStatKPIs` covering "no money data" case**

Add to `internal/services/network_analytics/application/service_test.go` (create if absent):

```go
func TestBuildStatKPIs_NoMoneyDataRendersAsNil(t *testing.T) {
	rows := []domain.StatRow{
		{Total: 100, Delivered: 80, Failed: 10, Revenue: 0, Cost: 0},
	}
	kpis := buildStatKPIs(rows)
	for _, k := range kpis {
		switch k.Name {
		case "Выручка", "Себестоимость", "Прибыль":
			require.Nil(t, k.Value, "money KPI %q must have nil value when source is 0", k.Name)
			require.Equal(t, "currency", k.Format)
			require.Equal(t, "RUB", k.Currency)
		case "Доставляемость":
			require.NotNil(t, k.Value)
			require.InDelta(t, 0.8, *k.Value, 0.001)
			require.Equal(t, "percent", k.Format)
		case "Ошибки":
			require.NotNil(t, k.Value)
			require.InDelta(t, 0.1, *k.Value, 0.001)
			require.Equal(t, "percent", k.Format)
		case "Всего":
			require.NotNil(t, k.Value)
			require.Equal(t, float64(100), *k.Value)
			require.Equal(t, "count", k.Format)
		}
	}
}

func TestBuildStatKPIs_WithMoneyDataRendersValues(t *testing.T) {
	rows := []domain.StatRow{
		{Total: 100, Delivered: 80, Failed: 10, Revenue: 1500.0, Cost: 600.0},
	}
	kpis := buildStatKPIs(rows)
	for _, k := range kpis {
		switch k.Name {
		case "Выручка":
			require.NotNil(t, k.Value)
			require.InDelta(t, 1500.0, *k.Value, 0.01)
		case "Прибыль":
			require.NotNil(t, k.Value)
			require.InDelta(t, 900.0, *k.Value, 0.01)
		}
	}
}
```

Run: `cd internal/services/network_analytics/application && go test -run TestBuildStatKPIs -v`

Expected: PASS.

- [ ] **Step 10: Build and verify everything compiles**

Run: `cd c:/projects/sms && go build ./... && ./scripts/check.sh`

Expected: clean build, all tests green.

- [ ] **Step 11: Commit**

```bash
git add api/proto/network_analytics/network_analytics.proto \
        api/proto/networkanalyticsv1/ \
        internal/services/network_analytics/domain/models.go \
        internal/services/network_analytics/application/service.go \
        internal/services/network_analytics/application/service_test.go \
        internal/services/network_analytics/infrastructure/repository/stats_repository.go \
        internal/services/network_analytics/grpc/server.go \
        internal/gateway/portal/handlers/network_statistics.go
git commit -m "feat(network-analytics): KPI format & nullable value

- Add format (count/percent/currency) and currency fields to KPI proto/domain
- KPI.Value becomes pointer; nil ⇒ no data (will render as '—' on frontend)
- Money KPIs (Выручка, Себестоимость, Прибыль) now return nil when source is 0,
  preventing the misleading '0 ₽ green' UI display
- Tests: buildStatKPIs covers both with-money and no-money cases

Refs: docs/superpowers/specs/2026-05-08-network-statistics-overhaul-design.md#f2-f3"
```

---

### Task 3: Frontend KPI renderer reads `format` and renders `—` for nil values

**Files:**
- Modify: `portal-frontend/src/api/networkStats.ts` — TS types for KPI
- Modify: `portal-frontend/src/pages/network/NetworkStatisticsPage.tsx` — KPI rendering
- Modify: `portal-frontend/src/pages/network/NetworkAnalyticsPage.tsx` — KPI rendering (same pattern)
- Create: `portal-frontend/src/components/network/KPICard.tsx` — extracted shared component if not present
- Create: `portal-frontend/src/components/network/formatKPIValue.ts` — pure formatting helper
- Test: `portal-frontend/src/components/network/formatKPIValue.test.ts`

- [ ] **Step 1: Update TS KPI type**

Edit `portal-frontend/src/api/networkStats.ts`. Find the existing `KPI` type (or named differently — search for it). Update:

```typescript
export type KPIFormat = 'count' | 'percent' | 'currency';

export interface KPI {
  name: string;
  value?: number;        // optional: undefined ⇒ no data, render '—'
  status?: 'ok' | 'warning' | 'danger' | 'unknown';
  delta?: number;
  format?: KPIFormat;
  currency?: string;     // present only when format === 'currency'
}
```

- [ ] **Step 2: Write the failing test for the formatter**

Create `portal-frontend/src/components/network/formatKPIValue.test.ts`:

```typescript
import { describe, it, expect } from 'vitest';
import { formatKPIValue } from './formatKPIValue';
import type { KPI } from '../../api/networkStats';

describe('formatKPIValue', () => {
  it('returns "—" when value is undefined', () => {
    const k: KPI = { name: 'Выручка', format: 'currency', currency: 'RUB' };
    expect(formatKPIValue(k)).toBe('—');
  });

  it('returns "—" when value is null (defensive — backend may serialize nil as null)', () => {
    const k: KPI = { name: 'Выручка', value: null as unknown as undefined, format: 'currency', currency: 'RUB' };
    expect(formatKPIValue(k)).toBe('—');
  });

  it('renders 0 as "0" (zero is a valid value, not "no data")', () => {
    const k: KPI = { name: 'Pending', value: 0, format: 'count' };
    expect(formatKPIValue(k)).toBe('0');
  });

  it('renders count with thousand separators (ru-RU locale)', () => {
    const k: KPI = { name: 'Всего', value: 12345, format: 'count' };
    // expect '12 345' or '12,345' depending on locale; the test pins ru-RU which uses NBSP/space.
    expect(formatKPIValue(k)).toMatch(/^12[\s ]345$/);
  });

  it('renders percent: API returns share 0..1, UI shows N%', () => {
    const k: KPI = { name: 'Доставляемость', value: 0.7084870848708487, format: 'percent' };
    expect(formatKPIValue(k)).toBe('70,8%');
  });

  it('renders percent for "Ошибки" with sign — fixes the "0,17" bug', () => {
    const k: KPI = { name: 'Ошибки', value: 0.16974169741697417, format: 'percent' };
    expect(formatKPIValue(k)).toBe('17,0%');
  });

  it('renders currency with symbol and locale', () => {
    const k: KPI = { name: 'Выручка', value: 1500.5, format: 'currency', currency: 'RUB' };
    // Expected like '1 500,50 ₽' or '1 500,5 ₽' depending on Intl behavior.
    expect(formatKPIValue(k)).toMatch(/1[\s ]500[,.]50?\s?₽/);
  });

  it('falls back to plain toString when format is missing', () => {
    const k: KPI = { name: 'Unknown', value: 42 };
    expect(formatKPIValue(k)).toBe('42');
  });
});
```

- [ ] **Step 3: Run the test to verify it FAILS (formatter does not exist)**

Run: `cd portal-frontend && npm run test -- formatKPIValue --run`

Expected: FAIL with "Cannot find module './formatKPIValue'".

- [ ] **Step 4: Implement the formatter**

Create `portal-frontend/src/components/network/formatKPIValue.ts`:

```typescript
import type { KPI } from '../../api/networkStats';

const numberFmt = new Intl.NumberFormat('ru-RU');
const percentFmt = new Intl.NumberFormat('ru-RU', {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
  style: 'decimal', // we append '%' manually so that 0.17 ⇒ '17,0%' not Intl's default
});

const currencyFmtCache = new Map<string, Intl.NumberFormat>();
function getCurrencyFmt(currency: string): Intl.NumberFormat {
  let f = currencyFmtCache.get(currency);
  if (!f) {
    f = new Intl.NumberFormat('ru-RU', {
      style: 'currency',
      currency,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
    currencyFmtCache.set(currency, f);
  }
  return f;
}

/**
 * Formats a KPI for display. The single source of truth for KPI rendering rules:
 *
 *  - value is undefined or null      → '—'  (no data; status should be 'unknown')
 *  - value === 0                     → '0' or '0%' or '0 ₽'  (zero is a valid value)
 *  - format === 'count'              → integer with locale separators
 *  - format === 'percent'            → value × 100, one decimal, '%' suffix
 *  - format === 'currency'           → value, two decimals, currency symbol
 *  - format missing                  → fallback to String(value)
 *
 * This function fixes the F2/F3 audit findings:
 *  - F2: "Ошибки 0,17" rendered without '%' (was buggy; now rendered as 17,0%)
 *  - F3: missing value rendered as "0 ₽ green" (was lying; now '—')
 */
export function formatKPIValue(k: KPI): string {
  if (k.value === undefined || k.value === null) {
    return '—';
  }
  switch (k.format) {
    case 'count':
      return numberFmt.format(k.value);
    case 'percent':
      return percentFmt.format(k.value * 100) + '%';
    case 'currency': {
      const cur = k.currency ?? 'RUB';
      return getCurrencyFmt(cur).format(k.value);
    }
    default:
      return String(k.value);
  }
}
```

- [ ] **Step 5: Run the test to verify it PASSES**

Run: `cd portal-frontend && npm run test -- formatKPIValue --run`

Expected: all 8 cases PASS.

- [ ] **Step 6: Wire the formatter into the actual KPI rendering**

Open `portal-frontend/src/pages/network/NetworkStatisticsPage.tsx`. Find where KPI cards are rendered (search for `kpis.map` or similar). Replace the inline value-rendering with a call to `formatKPIValue(kpi)`.

If there's no shared `KPICard` component yet, extract one to `portal-frontend/src/components/network/KPICard.tsx`:

```typescript
import type { KPI } from '../../api/networkStats';
import { formatKPIValue } from './formatKPIValue';

interface Props {
  kpi: KPI;
}

const statusToBg: Record<string, string> = {
  ok: 'bg-emerald-50 border-emerald-200',
  warning: 'bg-amber-50 border-amber-200',
  danger: 'bg-rose-50 border-rose-200',
  unknown: 'bg-neutral-50 border-neutral-200',
};

export function KPICard({ kpi }: Props) {
  // No-data: render with neutral status regardless of backend status field,
  // because '—' must NEVER look like a "good" green badge (F3 fix).
  const effectiveStatus = (kpi.value === undefined || kpi.value === null) ? 'unknown' : (kpi.status ?? 'ok');
  const bg = statusToBg[effectiveStatus] ?? statusToBg.unknown;

  return (
    <div className={`rounded-lg border p-4 ${bg}`}>
      <div className="text-xs text-neutral-600">{kpi.name}</div>
      <div className="text-2xl font-semibold mt-1">{formatKPIValue(kpi)}</div>
    </div>
  );
}
```

Then in `NetworkStatisticsPage.tsx`, swap the existing inline KPI rendering for `<KPICard kpi={k} />`.

Apply the same swap in `NetworkAnalyticsPage.tsx`.

- [ ] **Step 7: Type-check the frontend**

Run: `cd portal-frontend && npm run typecheck` (or `npx tsc --noEmit`).

Expected: clean.

- [ ] **Step 8: Lint**

Run: `cd portal-frontend && npm run lint`

Expected: ≤ 51 warnings (ratchet baseline; do NOT increase).

- [ ] **Step 9: Manual smoke on sandbox**

After deploying both backend and frontend, log in as `aggregator@test.local`, navigate to `/network/statistics`. Confirm:
- KPI «Ошибки» displays `17,0%` (was `0,17`).
- KPI «Выручка», «Себестоимость», «Прибыль» display `—` with neutral grey background (was `0 ₽` green).
- KPI «Всего» displays `271` (unchanged).
- KPI «Доставляемость» displays `70,8%` (unchanged).

- [ ] **Step 10: Commit**

```bash
git add portal-frontend/src/api/networkStats.ts \
        portal-frontend/src/components/network/KPICard.tsx \
        portal-frontend/src/components/network/formatKPIValue.ts \
        portal-frontend/src/components/network/formatKPIValue.test.ts \
        portal-frontend/src/pages/network/NetworkStatisticsPage.tsx \
        portal-frontend/src/pages/network/NetworkAnalyticsPage.tsx
git commit -m "fix(portal): KPI renderer — '—' for nil, '%' for percent

Fixes audit findings F2 and F3:
- F2: 'Ошибки 0,17' was missing the '%' suffix; now displays '17,0%'.
- F3: missing money KPIs were rendered as '0 ₽ green' (a lie); now '—'
  with neutral status. Zero is still rendered as '0'/'0%'/'0 ₽'.

Renderer is driven by KPI.format from the backend (count/percent/currency).

Refs: docs/superpowers/specs/2026-05-08-network-statistics-overhaul-design.md#f2-f3"
```

---

## SLICE 1 — Выручка (3 days)

### Task 4: Extend `rawAggQuery` to JOIN `tarification_log`

**Files:**
- Modify: `internal/services/network_analytics/application/aggregation_worker.go:43-71`
- Test: `internal/services/network_analytics/application/aggregation_worker_test.go`

- [ ] **Step 1: Write a failing integration test that asserts revenue flows through**

Open `internal/services/network_analytics/application/aggregation_worker_test.go` (create if absent). Add:

```go
package application_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network_analytics/application"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/infrastructure/repository"
	"github.com/rs/zerolog"
)

// TestAggregationWorker_BackfillJoinsTarificationLog verifies that after
// the rawAggQuery extension lands, network_stats_hourly.revenue equals the
// sum of total_amount in tarification_log for the same message set.
func TestAggregationWorker_BackfillJoinsTarificationLog(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	// 1. Seed: client (partner_id=999), operator, 5 messages with status=delivered
	//    in a known hour, each with one tarification_log row at 1.5 RUB.
	ctx := context.Background()
	hour := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC) // arbitrary historical hour
	clientID := uuid.New()
	operatorID := uuid.New()

	_, err = pool.Exec(ctx, `INSERT INTO clients (id, partner_id, name, email, password_hash) VALUES ($1, 999, 'test-agg', 'test-agg@x', '')`, clientID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO operators (id, name, country_id, mcc) VALUES ($1, 'TestOpAgg', NULL, '999')`, operatorID)
	require.NoError(t, err)

	defer func() {
		pool.Exec(ctx, `DELETE FROM tarification_log WHERE client_id=$1`, clientID)
		pool.Exec(ctx, `DELETE FROM messages WHERE client_id=$1`, clientID)
		pool.Exec(ctx, `DELETE FROM operators WHERE id=$1`, operatorID)
		pool.Exec(ctx, `DELETE FROM clients WHERE id=$1`, clientID)
		pool.Exec(ctx, `DELETE FROM network_stats_hourly WHERE partner_id=999`)
	}()

	for i := 0; i < 5; i++ {
		msgID := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO messages (id, source, destination, text, status, client_id, operator_id, created_at, channel, segment_count)
			VALUES ($1, 'TEST', '79991234567', 'hi', 'delivered', $2, $3, $4, 'sms', 1)`,
			msgID, clientID, operatorID, hour.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO tarification_log (id, client_id, message_id, operator_id, sender_category, strategy, segment_count, price_per_segment, total_amount, idempotency_key, created_at)
			VALUES (gen_random_uuid(), $1, $2, $3, 'shared', 'fixed', 1, 1.5, 1.5, $4, $5)`,
			clientID, msgID, operatorID, "test-"+msgID.String(), hour.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}

	// 2. Run BackfillWindow over the test hour.
	statsRepo := repository.NewStatsRepo(pool)
	monRepo := repository.NewMonitoringRepo(pool, nil) // monitoring not under test here
	worker := application.NewAggregationWorker(pool, statsRepo, monRepo, zerolog.Nop())

	err = worker.BackfillWindow(ctx, hour, hour.Add(time.Hour))
	require.NoError(t, err)

	// 3. Assert: network_stats_hourly for partner=999, hour=hour has revenue=7.5
	var totalRevenue float64
	err = pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(revenue), 0)
		FROM network_stats_hourly
		WHERE partner_id=999 AND hour=$1`, hour).Scan(&totalRevenue)
	require.NoError(t, err)
	require.InDelta(t, 7.5, totalRevenue, 0.001, "revenue must be sum(tarification_log.total_amount)")
}
```

- [ ] **Step 2: Run the test — verify it FAILS with revenue=0**

Run: `cd internal/services/network_analytics/application && TEST_DB_DSN=... go test -run TestAggregationWorker_BackfillJoinsTarificationLog -v`

Expected: FAIL with `revenue must be sum(tarification_log.total_amount): expected 7.5, got 0`.

- [ ] **Step 3: Replace `rawAggQuery` to LEFT JOIN tarification_log**

In `internal/services/network_analytics/application/aggregation_worker.go`, replace the `rawAggQuery` constant (lines 43-71):

```go
const rawAggQuery = `
SELECT
    COALESCE(p.partner_id, c.partner_id, 0)                    AS partner_id,
    date_trunc('hour', m.created_at)                           AS hour,
    0::bigint                                                  AS provider_id,
    COALESCE(op.name, '')                                      AS operator,
    COALESCE(co.iso_code, '')                                  AS country,
    COALESCE(NULLIF(m.channel, ''), 'sms')                     AS channel,
    COALESCE(c.name, c.email, '')                              AS login,
    COALESCE(m.source, '')                                     AS sender_name,
    COALESCE(m.service_type, '')                               AS traffic_type,
    COALESCE(m.send_method, '')                                AS method,
    COUNT(*)                                                   AS total,
    COUNT(*) FILTER (WHERE m.status = 'sent')                  AS sent,
    COUNT(*) FILTER (WHERE m.status = 'delivered')             AS delivered,
    COUNT(*) FILTER (WHERE m.status = 'failed')                AS failed,
    COUNT(*) FILTER (WHERE m.status = 'pending')               AS pending,
    COUNT(*) FILTER (WHERE m.status = 'expired')               AS timeout,
    COUNT(*) FILTER (WHERE m.status IN ('failed','rejected'))  AS error,
    COALESCE(SUM(t.total_amount), 0)                           AS revenue,
    0                                                          AS cost
FROM messages m
LEFT JOIN clients   c  ON c.id  = m.client_id
LEFT JOIN clients   p  ON p.id  = c.parent_client_id
LEFT JOIN operators op ON op.id = m.operator_id
LEFT JOIN countries co ON co.id = m.country_id
LEFT JOIN tarification_log t ON t.message_id = m.id
WHERE m.created_at >= $1 AND m.created_at < $2
GROUP BY 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
`
```

(`cost` stays at 0 — that's Slice 2's responsibility.)

- [ ] **Step 4: Run the test to verify it PASSES**

Run: `cd internal/services/network_analytics/application && TEST_DB_DSN=... go test -run TestAggregationWorker_BackfillJoinsTarificationLog -v`

Expected: PASS — revenue = 7.5.

- [ ] **Step 5: Build and run the full test suite to catch any nearby regressions**

Run: `cd c:/projects/sms && go build ./... && go test ./internal/services/network_analytics/...`

Expected: green.

- [ ] **Step 6: Commit**

```bash
git add internal/services/network_analytics/application/aggregation_worker.go \
        internal/services/network_analytics/application/aggregation_worker_test.go
git commit -m "feat(network-analytics): aggregator JOINs tarification_log for revenue

rawAggQuery now LEFT JOINs tarification_log on message_id. Revenue is
SUM(tarification_log.total_amount) per hourly bucket per dimension.
Cost stays at 0 (Slice 2 will add provider_tarification_log JOIN).

Test: 5 seeded messages × 1.5 RUB → network_stats_hourly.revenue = 7.5.

Refs: docs/superpowers/specs/2026-05-08-network-statistics-overhaul-design.md#d1"
```

---

### Task 5: Switch `UpsertHourlyStats` from add to replace semantics

**Files:**
- Modify: `internal/services/network_analytics/infrastructure/repository/stats_repository.go:721-773`
- Test: `internal/services/network_analytics/infrastructure/repository/stats_repository_test.go`

- [ ] **Step 1: Write a failing test that asserts double-aggregation does not double values**

Add to `stats_repository_test.go`:

```go
func TestStatsRepo_UpsertHourlyStats_DoubleRunDoesNotDouble(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.NewStatsRepo(pool)

	hour := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC) // never-touched hour
	row := domain.HourlyStatsRow{
		PartnerID: 999, Hour: hour, ProviderID: 0,
		Operator: "TEST", Country: "RU", Channel: "sms",
		Login: "agg-test", SenderName: "TEST", TrafficType: "", Method: "",
		Total: 100, Sent: 100, Delivered: 80, Failed: 10, Pending: 5, Timeout: 5, Error: 10,
		Revenue: 150.0, Cost: 60.0,
		DLRLatencySum: 1000, DLRLatencyCnt: 80, DLRLatencyP50: 100, DLRLatencyP95: 250,
		ThroughputMax: 10,
	}

	defer pool.Exec(context.Background(),
		`DELETE FROM network_stats_hourly WHERE partner_id=999 AND hour=$1`, hour)

	// First upsert: inserts new row.
	err = repo.UpsertHourlyStats(context.Background(), []domain.HourlyStatsRow{row})
	require.NoError(t, err)

	// Second upsert with the SAME row: must NOT double counters.
	err = repo.UpsertHourlyStats(context.Background(), []domain.HourlyStatsRow{row})
	require.NoError(t, err)

	var total int64
	var revenue float64
	err = pool.QueryRow(context.Background(), `
		SELECT total, revenue FROM network_stats_hourly
		WHERE partner_id=999 AND hour=$1`, hour).Scan(&total, &revenue)
	require.NoError(t, err)
	require.Equal(t, int64(100), total, "total must NOT be doubled (replace semantics)")
	require.InDelta(t, 150.0, revenue, 0.001, "revenue must NOT be doubled")
}
```

- [ ] **Step 2: Run the test — verify it FAILS (current behavior doubles)**

Run: `cd internal/services/network_analytics/infrastructure/repository && TEST_DB_DSN=... go test -run TestStatsRepo_UpsertHourlyStats_DoubleRunDoesNotDouble -v`

Expected: FAIL with `total must NOT be doubled: expected 100, got 200`.

- [ ] **Step 3: Change `UpsertHourlyStats` to replace semantics**

In `internal/services/network_analytics/infrastructure/repository/stats_repository.go:721-773`, replace the `ON CONFLICT ... DO UPDATE SET` clause:

```go
ON CONFLICT (partner_id, hour, provider_id, operator, country, channel, login, sender_name, traffic_type, method)
DO UPDATE SET
    total           = EXCLUDED.total,
    sent            = EXCLUDED.sent,
    delivered       = EXCLUDED.delivered,
    failed          = EXCLUDED.failed,
    pending         = EXCLUDED.pending,
    timeout         = EXCLUDED.timeout,
    error           = EXCLUDED.error,
    revenue         = EXCLUDED.revenue,
    cost            = EXCLUDED.cost,
    dlr_latency_sum = EXCLUDED.dlr_latency_sum,
    dlr_latency_cnt = EXCLUDED.dlr_latency_cnt,
    dlr_latency_p50 = EXCLUDED.dlr_latency_p50,
    dlr_latency_p95 = EXCLUDED.dlr_latency_p95,
    throughput_max  = GREATEST(network_stats_hourly.throughput_max, EXCLUDED.throughput_max)
```

(Note: `throughput_max` keeps `GREATEST(...)` semantics because it represents "peak in this hour" — replacement would lose history. All other counters become full replace.)

- [ ] **Step 4: Run the test — verify it PASSES**

Run: `cd internal/services/network_analytics/infrastructure/repository && TEST_DB_DSN=... go test -run TestStatsRepo_UpsertHourlyStats_DoubleRunDoesNotDouble -v`

Expected: PASS.

- [ ] **Step 5: Run all stats_repository tests to catch nearby regressions**

Run: `cd internal/services/network_analytics/infrastructure/repository && TEST_DB_DSN=... go test -v`

Expected: green.

- [ ] **Step 6: Search for other callers of `UpsertHourlyStats` to confirm replace-semantics is safe**

Run: `Grep -n 'UpsertHourlyStats' c:/projects/sms`. Expected: only `aggregation_worker.go` calls it. If any other caller exists, evaluate whether they expect additive behavior — if so, that caller is the bug, not this fix.

(From spec section 7 risk #2: I confirmed the only caller is aggregation_worker, but execute the grep at implementation time to validate.)

- [ ] **Step 7: Commit**

```bash
git add internal/services/network_analytics/infrastructure/repository/stats_repository.go \
        internal/services/network_analytics/infrastructure/repository/stats_repository_test.go
git commit -m "fix(stats): UpsertHourlyStats replace semantics for idempotent backfill

Counters previously accumulated on conflict (network_stats_hourly.total
+= EXCLUDED.total). This made re-aggregating the same hour double the
values, blocking idempotent backfills.

Switched all counters to EXCLUDED-only (replace). throughput_max keeps
GREATEST() because it represents an in-hour peak.

Refs: docs/superpowers/specs/2026-05-08-network-statistics-overhaul-design.md#decision-12"
```

---

### Task 6: Build the `network-stats-backfill` CLI

**Files:**
- Create: `cmd/network-stats-backfill/main.go`
- Create: `scripts/run-backfill-april.sh`

- [ ] **Step 1: Scaffold the CLI**

Create `cmd/network-stats-backfill/main.go`:

```go
// Command network-stats-backfill re-runs the hourly aggregator over a
// historical window. Used to repopulate network_stats_hourly after schema
// or aggregation changes (e.g., revenue JOIN added in Slice 1).
//
// Usage:
//   ./network-stats-backfill \
//     --from 2026-04-01T00:00:00Z \
//     --to   2026-05-09T00:00:00Z \
//     --truncate
//
// --truncate first deletes network_stats_hourly rows in [from, to) — required
// to avoid stale dimensions sticking around when source data changes.
// Without --truncate, the CLI relies on UpsertHourlyStats's replace
// semantics (Slice 1 Task 5 in this plan) but cannot remove rows for dimensions that
// no longer have any messages.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/network_analytics/application"
	"github.com/smpp-server/smpp-server/internal/services/network_analytics/infrastructure/repository"
)

func main() {
	var (
		from     string
		to       string
		truncate bool
		yes      bool
	)
	flag.StringVar(&from, "from", "", "start of window, RFC3339 (required)")
	flag.StringVar(&to, "to", "", "end of window, RFC3339, or 'now' (required)")
	flag.BoolVar(&truncate, "truncate", false, "DELETE network_stats_hourly rows in [from,to) before backfill")
	flag.BoolVar(&yes, "yes", false, "skip stdin confirmation for --truncate")
	flag.Parse()

	if from == "" || to == "" {
		log.Fatal().Msg("--from and --to are required")
	}
	fromT, err := time.Parse(time.RFC3339, from)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid --from")
	}
	var toT time.Time
	if to == "now" {
		toT = time.Now().UTC()
	} else {
		toT, err = time.Parse(time.RFC3339, to)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid --to")
		}
	}
	if !toT.After(fromT) {
		log.Fatal().Msg("--to must be after --from")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal().Msg("DATABASE_URL required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("pgxpool")
	}
	defer pool.Close()

	if truncate {
		if !yes {
			fmt.Printf("About to DELETE FROM network_stats_hourly WHERE hour >= %s AND hour < %s\n", fromT, toT)
			fmt.Print("Type YES to proceed: ")
			r := bufio.NewReader(os.Stdin)
			line, _ := r.ReadString('\n')
			if strings.TrimSpace(line) != "YES" {
				log.Fatal().Msg("aborted")
			}
		}
		tag, err := pool.Exec(ctx, `DELETE FROM network_stats_hourly WHERE hour >= $1 AND hour < $2`, fromT, toT)
		if err != nil {
			log.Fatal().Err(err).Msg("truncate failed")
		}
		log.Info().Int64("rows_deleted", tag.RowsAffected()).Msg("truncate done")
	}

	statsRepo := repository.NewStatsRepo(pool)
	monRepo := repository.NewMonitoringRepo(pool, nil)
	worker := application.NewAggregationWorker(pool, statsRepo, monRepo, zerolog.New(os.Stdout))

	start := time.Now()
	if err := worker.BackfillWindow(ctx, fromT, toT); err != nil {
		log.Fatal().Err(err).Msg("backfill failed")
	}
	log.Info().Dur("elapsed", time.Since(start)).
		Time("from", fromT).Time("to", toT).
		Msg("backfill done")
}
```

- [ ] **Step 2: Build the binary locally**

Run: `cd c:/projects/sms && go build -o /tmp/network-stats-backfill ./cmd/network-stats-backfill`

Expected: clean build.

- [ ] **Step 3: Create the wrapper script**

Create `scripts/run-backfill-april.sh`:

```bash
#!/usr/bin/env bash
# Backfill network_stats_hourly from 2026-04-01 to now. Run on sandbox after
# Slice 1 deploy. Truncates the window first; values will be repopulated by
# the aggregator with revenue from tarification_log.
#
# Usage (from sandbox shell):
#   DATABASE_URL=postgres://smpp:smpp@postgres:5432/smpp_db?sslmode=disable \
#     scripts/run-backfill-april.sh
set -euo pipefail
exec /opt/sms/bin/network-stats-backfill \
  --from 2026-04-01T00:00:00Z \
  --to now \
  --truncate \
  --yes
```

Make executable: `chmod +x scripts/run-backfill-april.sh`.

- [ ] **Step 4: Commit**

```bash
git add cmd/network-stats-backfill/main.go scripts/run-backfill-april.sh
git commit -m "feat(network-stats-backfill): one-shot CLI for re-aggregation

Replays aggregation_worker.BackfillWindow over a window, optionally
truncating network_stats_hourly first. Used to repopulate revenue
after Slice 1 lands. Idempotent thanks to UpsertHourlyStats replace
semantics (Task 5 in this plan).

Refs: docs/superpowers/specs/2026-05-08-network-statistics-overhaul-design.md#slice-1"
```

---

### Task 7: Run backfill on sandbox and verify revenue surfaces in UI

**Files:** none (operational).

- [ ] **Step 1: Deploy backend (aggregator + backfill CLI) to sandbox**

Run: `./scripts/server.sh deploy`

Expected: containers rebuild, network-analytics-service restarts with the new aggregator. The CLI is built into the image.

- [ ] **Step 2: Capture pre-backfill revenue**

Run:
```bash
./scripts/server.sh exec "docker exec postgres psql -U smpp -d smpp_db -c \"
  SELECT date_trunc('day', hour)::date AS day, SUM(revenue) AS revenue
  FROM network_stats_hourly
  WHERE hour >= '2026-04-01'
  GROUP BY 1 ORDER BY 1
\""
```

Expected: all rows show `revenue=0` (pre-Slice-1 state).

- [ ] **Step 3: Run the backfill**

```bash
./scripts/server.sh exec "
  DATABASE_URL=postgres://smpp:smpp@postgres:5432/smpp_db?sslmode=disable \
  /opt/sms/scripts/run-backfill-april.sh
"
```

Expected: log lines `truncate done rows_deleted=N`, `backfill done elapsed=X minutes`.

- [ ] **Step 4: Capture post-backfill revenue and cross-check**

Re-run the SQL from Step 2. Expected: all rows now have non-zero `revenue` matching `tarification_log` for the same days.

Cross-check against tarification_log:
```bash
./scripts/server.sh exec "docker exec postgres psql -U smpp -d smpp_db -c \"
  WITH agg_rev AS (
    SELECT date_trunc('day', hour)::date AS day, SUM(revenue) AS rev
    FROM network_stats_hourly WHERE hour >= '2026-04-01' GROUP BY 1
  ),
  log_rev AS (
    SELECT date_trunc('day', t.created_at)::date AS day, SUM(t.total_amount) AS rev
    FROM tarification_log t
    WHERE t.created_at >= '2026-04-01' GROUP BY 1
  )
  SELECT a.day, a.rev AS agg_rev, l.rev AS log_rev, a.rev - l.rev AS diff
  FROM agg_rev a FULL OUTER JOIN log_rev l ON a.day = l.day
  ORDER BY a.day
\""
```

Expected: `diff` close to 0 for each day. Small differences (<1 RUB) are acceptable — they reflect rows in `tarification_log` whose corresponding `messages` row has `partner_id=NULL` (orphan rows aggregator can't bucket).

- [ ] **Step 5: Manual UI smoke**

Log in as `aggregator@test.local` at `http://72.56.232.202:18085/network/statistics`. Switch period to «Месяц». Confirm:
- KPI «Выручка» shows a non-zero ruble value (matches the April daily sum from Step 4).
- KPI «Себестоимость» and «Прибыль» still show `—` (Slice 2 will fix cost).
- Table column "Выручка" shows non-zero numbers per day.

- [ ] **Step 6: No commit — operational verification.**

Document the cross-check numbers in a comment on the PR for traceability.

---

## Plan Self-Review

Before handing off to execution, the plan author confirms:

**Spec coverage (after 2026-05-08 v2 revision):**

| Spec section | Plan tasks |
|---|---|
| Slice 0 — F1 (DeleteView 403/404) | Task 1 |
| Slice 0 — F2 (KPI format field) | Task 2 |
| Slice 0 — F3 (KPI nullable rendering) | Task 3 |
| Slice 1 — D1 (rawAggQuery JOIN) | Task 4 |
| Slice 1 — UpsertHourlyStats replace | Task 5 |
| Slice 1 — Backfill utility | Task 6 |
| Slice 1 — Backfill execution + verify | Task 7 |

D2 (regression find/fix) and reseed CLI are intentionally NOT in this plan — see header revision note. Slice 2/3/4 are explicitly OUT of this plan (per the user's choice C: Slice 0 + Slice 1 together).

**Placeholder scan:** No "TBD"/"TODO" remain after D2 task removal.

**Type consistency:**
- `KPI.Value` is `*float64` in domain (Task 2 step 3) and `value?: number` in TS (Task 3 step 1). Consistent.
- `KPI.Format` field name same across proto/Go/TS. ✓
- `domain.ErrViewNotFound` / `ErrViewIsTemplate` / `ErrViewForbidden` defined in Task 1 step 1, used in Task 1 step 4 (`Delete`) and step 6 (gRPC mapping). ✓
- `aggregation_worker.BackfillWindow` signature: `func (w *AggregationWorker) BackfillWindow(ctx context.Context, from, to time.Time) error` — used in Task 6 step 1. Matches existing `aggregation_worker.go:101` signature. ✓
- `repository.NewStatsRepo(pool)` and `repository.NewMonitoringRepo(pool, nil)` constructors used in Tasks 4/6/7. The monitoring constructor accepts `(*pgxpool.Pool, *redis.Client)` per `monitoring_repo.go:21` — passing `nil` for redis is safe because the backfill path doesn't read live metrics.

**Coverage gaps:** Slice 0 has no explicit task to verify the audit's finding that the audit-original UI displays "Ошибки 0,17" — this is implicitly verified in Task 3 step 9 ("manual smoke on sandbox"). Acceptable.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-08-network-statistics-slice-0-and-1.md`.**

Two execution options:

1. **Subagent-Driven (recommended)** — fresh subagent per task, review between tasks, fast iteration. Best for tasks with clear acceptance criteria and the regression-test-first pattern this plan uses heavily.
2. **Inline Execution** — execute tasks in the current session using executing-plans, batch execution with checkpoints for review.

Per project rule: **all code-touching execution must go through `/execute-with-review`** (CLAUDE.md "Mandatory code review for any work with code"). The wrapper enforces a code-reviewer subagent on every diff and 3-iteration retry. This applies regardless of which execution mode you pick.

**Which approach?**
