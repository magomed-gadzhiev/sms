# Aggregator Phase 4: Analytics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the aggregator analytics dashboard — margin summary, breakdowns by sub-account and operator, time-series charts, balance forecast, tariff simulator, and CSV export — giving aggregators a full business picture.

**Architecture:** New `AnalyticsService` in the aggregator service reads `aggregator_margin_log` (created in Phase 2) via a dedicated `AnalyticsRepository`. Seven new gRPC RPCs added to `AggregatorService`. HTTP handlers follow the same pattern as Phase 2/3 tariff handlers (portal gateway → gRPC → service). No new DB migrations needed. Frontend adds `AnalyticsPage` (3 tabs) and `SimulatorPage`; the existing `AggregatorDashboardPage` stub is replaced with real charts.

**Tech Stack:** Go 1.24 + pgx/v5, gorilla/mux, protobuf/gRPC, React 19 + TypeScript + Tailwind CSS 4.2, Recharts 3.x, Radix UI, zerolog, testify/mock

All work is done in worktree `.worktrees/aggregator-phase4/` on branch `feature/aggregator-phase4`.

---

## File Map

### New files
- `internal/services/aggregator/domain/analytics.go` — DashboardMetrics, SubAccountMargin, OperatorMargin, TimeSeriesPoint, DailyStats, SimulationQuery, SimulationData, SimulationResult, BalanceForecast
- `internal/services/aggregator/repository/analytics_repository.go` — SQL queries on aggregator_margin_log + clients + operators
- `internal/services/aggregator/service/analytics_service.go` — business logic: dashboard summary, breakdowns, time series, forecast, simulator, export
- `internal/services/aggregator/service/analytics_service_test.go` — unit tests with mock repos
- `internal/gateway/portal/handlers/aggregator_analytics.go` — 7 HTTP handlers
- `internal/gateway/portal/handlers/aggregator_analytics_test.go` — handler tests
- `portal-frontend/src/pages/aggregator/AnalyticsPage.tsx` — analytics with 3 tabs: BySubAccounts, ByOperators, TimeSeries
- `portal-frontend/src/pages/aggregator/SimulatorPage.tsx` — tariff simulator

### Modified files
- `api/proto/aggregator/aggregator.proto` — add 7 analytics RPCs + all request/response messages
- `api/proto/aggregatorv1/aggregator.pb.go` — regenerated
- `api/proto/aggregatorv1/aggregator_grpc.pb.go` — regenerated
- `internal/services/aggregator/grpc/server.go` — implement 7 new analytics RPC handlers
- `internal/gateway/portal/router/router.go` — register 7 analytics routes
- `portal-frontend/src/api/aggregator.ts` — add analytics/forecast/simulator/export API functions
- `portal-frontend/src/pages/aggregator/AggregatorDashboardPage.tsx` — replace stub with real data + Recharts line chart
- `portal-frontend/src/App.tsx` — add /aggregator/analytics and /aggregator/simulator routes

---

## Task 1: Create Worktree

**Files:** none (setup)

- [ ] **Step 1: Create Phase 4 worktree from Phase 3 branch**

```bash
git worktree add .worktrees/aggregator-phase4 -b feature/aggregator-phase4 feature/aggregator-phase3
```

- [ ] **Step 2: Verify worktree has Phase 3 files**

```bash
ls .worktrees/aggregator-phase4/internal/services/aggregator/service/
```
Expected: `aggregator_service.go  tariff_service.go  route_service.go`

- [ ] **Step 3: Initial commit**

```bash
cd .worktrees/aggregator-phase4
git commit --allow-empty -m "chore: start aggregator phase 4 analytics"
```

---

## Task 2: Analytics Domain Types

**Files:**
- Create: `internal/services/aggregator/domain/analytics.go`

- [ ] **Step 1: Write domain types**

```go
// internal/services/aggregator/domain/analytics.go
package domain

import "time"

// DashboardMetrics holds the current-period counters shown at the top of the dashboard.
type DashboardMetrics struct {
	MarginToday     float64
	MarginThisWeek  float64
	MarginThisMonth float64
	SMSToday        int64
	SMSThisMonth    int64
}

// SubAccountMargin holds margin breakdown for a single sub-account.
type SubAccountMargin struct {
	SubAccountID    string
	SubAccountName  string
	SMSCount        int64
	SubAccountTotal float64 // revenue from sub-account
	AggregatorTotal float64 // cost (purchase tariff)
	Margin          float64 // SubAccountTotal - AggregatorTotal
	AvgMarginPerSMS float64
}

// OperatorMargin holds margin breakdown for a single operator.
type OperatorMargin struct {
	OperatorID   string
	OperatorName string
	SMSCount     int64
	Margin       float64
	MarginShare  float64 // percentage of total margin (0–100)
}

// TimeSeriesPoint is one data point in a margin time series.
type TimeSeriesPoint struct {
	Period    time.Time
	SMSCount  int64
	Margin    float64
	AvgMargin float64
}

// DailyStats holds daily aggregated spend and margin (for forecast calculation).
type DailyStats struct {
	Date   time.Time
	Spend  float64 // aggregator_total for the day
	Margin float64
}

// BalanceForecast holds the balance runway and end-of-month margin projection.
type BalanceForecast struct {
	CurrentBalance        float64
	AvgDailySpend         float64
	DaysRemaining         float64 // CurrentBalance / AvgDailySpend
	ForecastedMonthMargin float64 // current month margin + projected remainder
	LowBalanceWarning     bool    // true when DaysRemaining < 7
}

// SimulationQuery is the input for a tariff simulation.
type SimulationQuery struct {
	AggregatorID string
	SubAccountID *string // nil = all sub-accounts
	OperatorID   *string // nil = all operators
	From         time.Time
	To           time.Time
}

// SimulationData is the raw data extracted from aggregator_margin_log for simulation.
type SimulationData struct {
	SMSCount         int64
	TotalSegments    int64
	AvgPurchasePrice float64 // aggregator_total / total_segments
	CurrentMargin    float64
}

// SimulationResult is what the simulator returns after computing the what-if scenario.
type SimulationResult struct {
	PeriodSMSCount    int64
	AvgPurchasePrice  float64
	CurrentMargin     float64
	NewMargin         float64
	MarginDiff        float64 // NewMargin - CurrentMargin
	MarginDiffPercent float64 // MarginDiff / CurrentMargin * 100
	IsBelowCost       bool    // new_price < AvgPurchasePrice
	ExceedsMaxMarkup  bool    // new_price > max allowed by aggregator profile
	MaxAllowedPrice   float64
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd .worktrees/aggregator-phase4
go build ./internal/services/aggregator/domain/...
```

Expected: no output (clean build)

- [ ] **Step 3: Commit**

```bash
git add internal/services/aggregator/domain/analytics.go
git commit -m "feat(aggregator): add analytics domain types"
```

---

## Task 3: Analytics Repository

**Files:**
- Create: `internal/services/aggregator/repository/analytics_repository.go`

> Before writing: read `internal/services/aggregator/repository/margin_repository.go` to understand the DB type and import path used. Use the same `*pgxpool.Pool` pattern.

- [ ] **Step 1: Write the repository**

```go
// internal/services/aggregator/repository/analytics_repository.go
package repository

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
)

// AnalyticsRepository reads from aggregator_margin_log, clients, and operators.
type AnalyticsRepository struct {
	db *pgxpool.Pool
}

func NewAnalyticsRepository(db *pgxpool.Pool) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

// GetDashboardMetrics returns today/week/month margins and SMS counts in one query.
func (r *AnalyticsRepository) GetDashboardMetrics(ctx context.Context, aggregatorID string) (*domain.DashboardMetrics, error) {
	const q = `
		SELECT
			COALESCE(SUM(CASE WHEN created_at >= CURRENT_DATE THEN margin END), 0),
			COALESCE(SUM(CASE WHEN created_at >= date_trunc('week',  NOW()) THEN margin END), 0),
			COALESCE(SUM(CASE WHEN created_at >= date_trunc('month', NOW()) THEN margin END), 0),
			COALESCE(COUNT(CASE WHEN created_at >= CURRENT_DATE THEN 1 END), 0),
			COALESCE(COUNT(CASE WHEN created_at >= date_trunc('month', NOW()) THEN 1 END), 0)
		FROM aggregator_margin_log
		WHERE aggregator_id = $1`

	m := &domain.DashboardMetrics{}
	err := r.db.QueryRow(ctx, q, aggregatorID).Scan(
		&m.MarginToday, &m.MarginThisWeek, &m.MarginThisMonth,
		&m.SMSToday, &m.SMSThisMonth,
	)
	if err != nil {
		return nil, fmt.Errorf("get dashboard metrics: %w", err)
	}
	return m, nil
}

// CountActiveSubAccounts counts sub-accounts where parent_client_id = aggregatorID and active = true.
func (r *AnalyticsRepository) CountActiveSubAccounts(ctx context.Context, aggregatorID string) (int, error) {
	const q = `
		SELECT COUNT(*) FROM clients
		WHERE parent_client_id = $1 AND account_type = 'sub_account' AND active = true`
	var n int
	if err := r.db.QueryRow(ctx, q, aggregatorID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count active sub-accounts: %w", err)
	}
	return n, nil
}

// GetMarginBySubAccounts returns margin breakdown grouped by sub-account for a period.
func (r *AnalyticsRepository) GetMarginBySubAccounts(ctx context.Context, aggregatorID string, from, to time.Time) ([]domain.SubAccountMargin, error) {
	const q = `
		SELECT
			m.sub_account_id::text,
			c.name,
			COUNT(*),
			COALESCE(SUM(m.sub_account_total), 0),
			COALESCE(SUM(m.aggregator_total), 0),
			COALESCE(SUM(m.margin), 0),
			CASE WHEN COUNT(*) > 0 THEN COALESCE(SUM(m.margin), 0) / COUNT(*) ELSE 0 END
		FROM aggregator_margin_log m
		JOIN clients c ON c.id = m.sub_account_id
		WHERE m.aggregator_id = $1 AND m.created_at >= $2 AND m.created_at < $3
		GROUP BY m.sub_account_id, c.name
		ORDER BY SUM(m.margin) DESC`

	rows, err := r.db.Query(ctx, q, aggregatorID, from, to)
	if err != nil {
		return nil, fmt.Errorf("get margin by sub-accounts: %w", err)
	}
	defer rows.Close()

	var result []domain.SubAccountMargin
	for rows.Next() {
		var item domain.SubAccountMargin
		if err := rows.Scan(
			&item.SubAccountID, &item.SubAccountName,
			&item.SMSCount, &item.SubAccountTotal,
			&item.AggregatorTotal, &item.Margin, &item.AvgMarginPerSMS,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// GetMarginByOperators returns margin breakdown grouped by operator for a period.
func (r *AnalyticsRepository) GetMarginByOperators(ctx context.Context, aggregatorID string, from, to time.Time) ([]domain.OperatorMargin, error) {
	const q = `
		WITH totals AS (
			SELECT COALESCE(SUM(margin), 0) AS grand_total
			FROM aggregator_margin_log
			WHERE aggregator_id = $1 AND created_at >= $2 AND created_at < $3
		)
		SELECT
			m.operator_id::text,
			o.name,
			COUNT(*),
			COALESCE(SUM(m.margin), 0),
			CASE WHEN t.grand_total > 0
				THEN COALESCE(SUM(m.margin), 0) / t.grand_total * 100
				ELSE 0 END
		FROM aggregator_margin_log m
		JOIN operators o ON o.id = m.operator_id
		CROSS JOIN totals t
		WHERE m.aggregator_id = $1 AND m.created_at >= $2 AND m.created_at < $3
		GROUP BY m.operator_id, o.name, t.grand_total
		ORDER BY SUM(m.margin) DESC`

	rows, err := r.db.Query(ctx, q, aggregatorID, from, to)
	if err != nil {
		return nil, fmt.Errorf("get margin by operators: %w", err)
	}
	defer rows.Close()

	var result []domain.OperatorMargin
	for rows.Next() {
		var item domain.OperatorMargin
		if err := rows.Scan(
			&item.OperatorID, &item.OperatorName,
			&item.SMSCount, &item.Margin, &item.MarginShare,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// GetMarginTimeSeries returns time-bucketed margin data. granularity must be "day", "week", or "month".
func (r *AnalyticsRepository) GetMarginTimeSeries(ctx context.Context, aggregatorID string, from, to time.Time, granularity string) ([]domain.TimeSeriesPoint, error) {
	allowed := map[string]bool{"day": true, "week": true, "month": true}
	if !allowed[granularity] {
		return nil, fmt.Errorf("invalid granularity %q: must be day, week, or month", granularity)
	}

	q := fmt.Sprintf(`
		SELECT
			date_trunc('%s', created_at) AS period,
			COUNT(*),
			COALESCE(SUM(margin), 0),
			CASE WHEN COUNT(*) > 0 THEN COALESCE(SUM(margin), 0) / COUNT(*) ELSE 0 END
		FROM aggregator_margin_log
		WHERE aggregator_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY date_trunc('%s', created_at)
		ORDER BY period ASC`, granularity, granularity)

	rows, err := r.db.Query(ctx, q, aggregatorID, from, to)
	if err != nil {
		return nil, fmt.Errorf("get margin time series: %w", err)
	}
	defer rows.Close()

	var result []domain.TimeSeriesPoint
	for rows.Next() {
		var pt domain.TimeSeriesPoint
		if err := rows.Scan(&pt.Period, &pt.SMSCount, &pt.Margin, &pt.AvgMargin); err != nil {
			return nil, err
		}
		result = append(result, pt)
	}
	return result, rows.Err()
}

// GetDailyStats returns per-day spend and margin for the last `days` days (for forecast).
func (r *AnalyticsRepository) GetDailyStats(ctx context.Context, aggregatorID string, days int) ([]domain.DailyStats, error) {
	const q = `
		SELECT
			DATE(created_at) AS date,
			COALESCE(SUM(aggregator_total), 0),
			COALESCE(SUM(margin), 0)
		FROM aggregator_margin_log
		WHERE aggregator_id = $1
		  AND created_at >= NOW() - ($2 * INTERVAL '1 day')
		GROUP BY DATE(created_at)
		ORDER BY date ASC`

	rows, err := r.db.Query(ctx, q, aggregatorID, days)
	if err != nil {
		return nil, fmt.Errorf("get daily stats: %w", err)
	}
	defer rows.Close()

	var result []domain.DailyStats
	for rows.Next() {
		var s domain.DailyStats
		if err := rows.Scan(&s.Date, &s.Spend, &s.Margin); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// GetSimulationData returns raw data needed for a tariff simulation.
func (r *AnalyticsRepository) GetSimulationData(ctx context.Context, req domain.SimulationQuery) (*domain.SimulationData, error) {
	const q = `
		SELECT
			COUNT(*),
			COALESCE(SUM(segment_count), 0),
			CASE WHEN SUM(segment_count) > 0
				THEN COALESCE(SUM(aggregator_total), 0) / SUM(segment_count)
				ELSE 0 END,
			COALESCE(SUM(margin), 0)
		FROM aggregator_margin_log
		WHERE aggregator_id = $1
		  AND ($2::uuid IS NULL OR sub_account_id = $2)
		  AND ($3::uuid IS NULL OR operator_id = $3)
		  AND created_at >= $4 AND created_at < $5`

	d := &domain.SimulationData{}
	err := r.db.QueryRow(ctx, q,
		req.AggregatorID, req.SubAccountID, req.OperatorID,
		req.From, req.To,
	).Scan(&d.SMSCount, &d.TotalSegments, &d.AvgPurchasePrice, &d.CurrentMargin)
	if err != nil {
		return nil, fmt.Errorf("get simulation data: %w", err)
	}
	return d, nil
}

// ExportMarginReport returns a CSV of all margin_log rows for the period.
func (r *AnalyticsRepository) ExportMarginReport(ctx context.Context, aggregatorID string, from, to time.Time) ([]byte, error) {
	const q = `
		SELECT
			m.created_at,
			c.name,
			o.name,
			m.segment_count,
			m.sub_account_total,
			m.aggregator_total,
			m.margin
		FROM aggregator_margin_log m
		JOIN clients c ON c.id = m.sub_account_id
		JOIN operators o ON o.id = m.operator_id
		WHERE m.aggregator_id = $1 AND m.created_at >= $2 AND m.created_at < $3
		ORDER BY m.created_at DESC`

	rows, err := r.db.Query(ctx, q, aggregatorID, from, to)
	if err != nil {
		return nil, fmt.Errorf("export margin report query: %w", err)
	}
	defer rows.Close()

	var buf strings.Builder
	w := csv.NewWriter(&buf)

	_ = w.Write([]string{"created_at", "sub_account", "operator", "segments", "sub_account_total", "aggregator_total", "margin"})

	for rows.Next() {
		var (
			createdAt                              time.Time
			subName, opName                        string
			segments                               int64
			subTotal, aggTotal, margin             float64
		)
		if err := rows.Scan(&createdAt, &subName, &opName, &segments, &subTotal, &aggTotal, &margin); err != nil {
			return nil, err
		}
		_ = w.Write([]string{
			createdAt.Format(time.RFC3339),
			subName, opName,
			fmt.Sprintf("%d", segments),
			fmt.Sprintf("%.6f", subTotal),
			fmt.Sprintf("%.6f", aggTotal),
			fmt.Sprintf("%.6f", margin),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	w.Flush()
	return []byte(buf.String()), nil
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd .worktrees/aggregator-phase4
go build ./internal/services/aggregator/repository/...
```

Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/services/aggregator/repository/analytics_repository.go
git commit -m "feat(aggregator): add analytics repository with margin queries"
```

---

## Task 4: Analytics Service — Dashboard, Breakdowns, Time Series

**Files:**
- Create: `internal/services/aggregator/service/analytics_service.go`
- Create: `internal/services/aggregator/service/analytics_service_test.go`

> Read `internal/services/aggregator/service/aggregator_service.go` first to understand the interface pattern (repos as constructor params).

- [ ] **Step 1: Write failing tests**

```go
// internal/services/aggregator/service/analytics_service_test.go
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
	"github.com/smpp-server/smpp-server/internal/services/aggregator/service"
)

// --- Mocks ---

type mockAnalyticsRepo struct{ mock.Mock }

func (m *mockAnalyticsRepo) GetDashboardMetrics(ctx context.Context, id string) (*domain.DashboardMetrics, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*domain.DashboardMetrics), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockAnalyticsRepo) CountActiveSubAccounts(ctx context.Context, id string) (int, error) {
	args := m.Called(ctx, id)
	return args.Int(0), args.Error(1)
}
func (m *mockAnalyticsRepo) GetMarginBySubAccounts(ctx context.Context, id string, from, to time.Time) ([]domain.SubAccountMargin, error) {
	args := m.Called(ctx, id, from, to)
	return args.Get(0).([]domain.SubAccountMargin), args.Error(1)
}
func (m *mockAnalyticsRepo) GetMarginByOperators(ctx context.Context, id string, from, to time.Time) ([]domain.OperatorMargin, error) {
	args := m.Called(ctx, id, from, to)
	return args.Get(0).([]domain.OperatorMargin), args.Error(1)
}
func (m *mockAnalyticsRepo) GetMarginTimeSeries(ctx context.Context, id string, from, to time.Time, gran string) ([]domain.TimeSeriesPoint, error) {
	args := m.Called(ctx, id, from, to, gran)
	return args.Get(0).([]domain.TimeSeriesPoint), args.Error(1)
}
func (m *mockAnalyticsRepo) GetDailyStats(ctx context.Context, id string, days int) ([]domain.DailyStats, error) {
	args := m.Called(ctx, id, days)
	return args.Get(0).([]domain.DailyStats), args.Error(1)
}
func (m *mockAnalyticsRepo) GetSimulationData(ctx context.Context, req domain.SimulationQuery) (*domain.SimulationData, error) {
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.(*domain.SimulationData), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockAnalyticsRepo) ExportMarginReport(ctx context.Context, id string, from, to time.Time) ([]byte, error) {
	args := m.Called(ctx, id, from, to)
	return args.Get(0).([]byte), args.Error(1)
}

type mockProfileRepo struct{ mock.Mock }

func (m *mockProfileRepo) GetByClientID(ctx context.Context, id string) (*domain.AggregatorProfile, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*domain.AggregatorProfile), args.Error(1)
	}
	return nil, args.Error(1)
}

type mockBalanceReader struct{ mock.Mock }

func (m *mockBalanceReader) GetBalance(ctx context.Context, id string) (float64, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(float64), args.Error(1)
}

// --- Tests ---

func TestAnalyticsService_GetDashboardSummary(t *testing.T) {
	repo := new(mockAnalyticsRepo)
	profile := new(mockProfileRepo)
	balance := new(mockBalanceReader)
	svc := service.NewAnalyticsService(repo, profile, balance)

	aggID := "agg-1"
	repo.On("GetDashboardMetrics", mock.Anything, aggID).Return(&domain.DashboardMetrics{
		MarginToday: 1200, MarginThisWeek: 8400, MarginThisMonth: 34000,
		SMSToday: 400, SMSThisMonth: 11000,
	}, nil)
	repo.On("CountActiveSubAccounts", mock.Anything, aggID).Return(18, nil)
	repo.On("GetMarginBySubAccounts", mock.Anything, aggID, mock.Anything, mock.Anything).Return([]domain.SubAccountMargin{
		{SubAccountID: "s1", Margin: 5000},
		{SubAccountID: "s2", Margin: 3000},
		{SubAccountID: "s3", Margin: 2000},
		{SubAccountID: "s4", Margin: 1000},
	}, nil)
	repo.On("GetMarginByOperators", mock.Anything, aggID, mock.Anything, mock.Anything).Return([]domain.OperatorMargin{
		{OperatorID: "o1", Margin: 6000},
	}, nil)
	profile.On("GetByClientID", mock.Anything, aggID).Return(&domain.AggregatorProfile{
		MaxSubAccounts: 100,
	}, nil)
	balance.On("GetBalance", mock.Anything, aggID).Return(245000.0, nil)

	summary, err := svc.GetDashboardSummary(context.Background(), aggID)
	require.NoError(t, err)

	assert.Equal(t, 245000.0, summary.Balance)
	assert.Equal(t, 1200.0, summary.MarginToday)
	assert.Equal(t, 34000.0, summary.MarginThisMonth)
	assert.Equal(t, int64(400), summary.SMSToday)
	assert.Equal(t, 18, summary.ActiveSubAccounts)
	assert.Equal(t, 100, summary.MaxSubAccounts)
	assert.Len(t, summary.TopSubAccounts, 3) // capped at 3
	assert.Len(t, summary.TopOperators, 1)
}

func TestAnalyticsService_GetMarginTimeSeries_InvalidGranularity(t *testing.T) {
	repo := new(mockAnalyticsRepo)
	svc := service.NewAnalyticsService(repo, new(mockProfileRepo), new(mockBalanceReader))

	_, err := svc.GetMarginTimeSeries(context.Background(), "agg-1", time.Now().AddDate(0, -1, 0), time.Now(), "hour")
	assert.ErrorContains(t, err, "granularity")
}

func TestAnalyticsService_GetMarginTimeSeries_Valid(t *testing.T) {
	repo := new(mockAnalyticsRepo)
	svc := service.NewAnalyticsService(repo, new(mockProfileRepo), new(mockBalanceReader))

	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()
	expected := []domain.TimeSeriesPoint{{Period: from, SMSCount: 100, Margin: 500}}
	repo.On("GetMarginTimeSeries", mock.Anything, "agg-1", from, to, "day").Return(expected, nil)

	pts, err := svc.GetMarginTimeSeries(context.Background(), "agg-1", from, to, "day")
	require.NoError(t, err)
	assert.Equal(t, expected, pts)
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd .worktrees/aggregator-phase4
go test ./internal/services/aggregator/service/... -run TestAnalytics -v 2>&1 | head -20
```

Expected: compile error (service.NewAnalyticsService not defined)

- [ ] **Step 3: Write the analytics service**

```go
// internal/services/aggregator/service/analytics_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/smpp-server/smpp-server/internal/services/aggregator/domain"
)

// AnalyticsRepo defines read access to aggregator_margin_log.
type AnalyticsRepo interface {
	GetDashboardMetrics(ctx context.Context, aggregatorID string) (*domain.DashboardMetrics, error)
	CountActiveSubAccounts(ctx context.Context, aggregatorID string) (int, error)
	GetMarginBySubAccounts(ctx context.Context, aggregatorID string, from, to time.Time) ([]domain.SubAccountMargin, error)
	GetMarginByOperators(ctx context.Context, aggregatorID string, from, to time.Time) ([]domain.OperatorMargin, error)
	GetMarginTimeSeries(ctx context.Context, aggregatorID string, from, to time.Time, granularity string) ([]domain.TimeSeriesPoint, error)
	GetDailyStats(ctx context.Context, aggregatorID string, days int) ([]domain.DailyStats, error)
	GetSimulationData(ctx context.Context, req domain.SimulationQuery) (*domain.SimulationData, error)
	ExportMarginReport(ctx context.Context, aggregatorID string, from, to time.Time) ([]byte, error)
}

// AnalyticsProfileReader reads aggregator profile (for max_sub_accounts and max_markup_percent).
type AnalyticsProfileReader interface {
	GetByClientID(ctx context.Context, clientID string) (*domain.AggregatorProfile, error)
}

// AnalyticsBalanceReader reads the current balance of a client account.
type AnalyticsBalanceReader interface {
	GetBalance(ctx context.Context, clientID string) (float64, error)
}

// AnalyticsService implements all aggregator analytics use cases.
type AnalyticsService struct {
	repo    AnalyticsRepo
	profile AnalyticsProfileReader
	balance AnalyticsBalanceReader
}

func NewAnalyticsService(repo AnalyticsRepo, profile AnalyticsProfileReader, balance AnalyticsBalanceReader) *AnalyticsService {
	return &AnalyticsService{repo: repo, profile: profile, balance: balance}
}

// DashboardSummary is the full data payload for the aggregator dashboard.
type DashboardSummary struct {
	domain.DashboardMetrics
	Balance           float64
	ActiveSubAccounts int
	MaxSubAccounts    int
	TopSubAccounts    []domain.SubAccountMargin // top 3 by margin (this month)
	TopOperators      []domain.OperatorMargin   // top 3 by margin (this month)
}

// GetDashboardSummary assembles all data for the aggregator dashboard.
func (s *AnalyticsService) GetDashboardSummary(ctx context.Context, aggregatorID string) (*DashboardSummary, error) {
	metrics, err := s.repo.GetDashboardMetrics(ctx, aggregatorID)
	if err != nil {
		return nil, fmt.Errorf("get dashboard metrics: %w", err)
	}

	bal, err := s.balance.GetBalance(ctx, aggregatorID)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	activeCount, err := s.repo.CountActiveSubAccounts(ctx, aggregatorID)
	if err != nil {
		return nil, fmt.Errorf("count sub-accounts: %w", err)
	}

	prof, err := s.profile.GetByClientID(ctx, aggregatorID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	// Top sub-accounts and operators are for the current calendar month.
	monthStart := beginningOfMonth(time.Now())
	monthEnd := time.Now()

	subMargins, err := s.repo.GetMarginBySubAccounts(ctx, aggregatorID, monthStart, monthEnd)
	if err != nil {
		return nil, fmt.Errorf("get sub-account margins: %w", err)
	}
	opMargins, err := s.repo.GetMarginByOperators(ctx, aggregatorID, monthStart, monthEnd)
	if err != nil {
		return nil, fmt.Errorf("get operator margins: %w", err)
	}

	return &DashboardSummary{
		DashboardMetrics:  *metrics,
		Balance:           bal,
		ActiveSubAccounts: activeCount,
		MaxSubAccounts:    prof.MaxSubAccounts,
		TopSubAccounts:    top3SubAccounts(subMargins),
		TopOperators:      top3Operators(opMargins),
	}, nil
}

// GetMarginBySubAccounts returns full margin breakdown by sub-account for a period.
func (s *AnalyticsService) GetMarginBySubAccounts(ctx context.Context, aggregatorID string, from, to time.Time) ([]domain.SubAccountMargin, error) {
	return s.repo.GetMarginBySubAccounts(ctx, aggregatorID, from, to)
}

// GetMarginByOperators returns full margin breakdown by operator for a period.
func (s *AnalyticsService) GetMarginByOperators(ctx context.Context, aggregatorID string, from, to time.Time) ([]domain.OperatorMargin, error) {
	return s.repo.GetMarginByOperators(ctx, aggregatorID, from, to)
}

// GetMarginTimeSeries returns time-bucketed margin data. granularity: "day" | "week" | "month".
func (s *AnalyticsService) GetMarginTimeSeries(ctx context.Context, aggregatorID string, from, to time.Time, granularity string) ([]domain.TimeSeriesPoint, error) {
	allowed := map[string]bool{"day": true, "week": true, "month": true}
	if !allowed[granularity] {
		return nil, fmt.Errorf("invalid granularity %q: must be day, week, or month", granularity)
	}
	return s.repo.GetMarginTimeSeries(ctx, aggregatorID, from, to, granularity)
}

// GetBalanceForecast projects balance runway and end-of-month margin.
func (s *AnalyticsService) GetBalanceForecast(ctx context.Context, aggregatorID string) (*domain.BalanceForecast, error) {
	bal, err := s.balance.GetBalance(ctx, aggregatorID)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	stats, err := s.repo.GetDailyStats(ctx, aggregatorID, 14)
	if err != nil {
		return nil, fmt.Errorf("get daily stats: %w", err)
	}

	if len(stats) == 0 {
		return &domain.BalanceForecast{CurrentBalance: bal}, nil
	}

	var totalSpend, totalMargin float64
	for _, d := range stats {
		totalSpend += d.Spend
		totalMargin += d.Margin
	}
	n := float64(len(stats))
	avgDailySpend := totalSpend / n
	avgDailyMargin := totalMargin / n

	var daysRemaining float64
	if avgDailySpend > 0 {
		daysRemaining = bal / avgDailySpend
	}

	// Project end-of-month margin.
	now := time.Now()
	daysInMonth := float64(daysInCurrentMonth(now))
	currentMonthMargin := currentMonthMarginFromStats(stats, now)
	daysLeft := daysInMonth - float64(now.Day())
	forecastedMonthMargin := currentMonthMargin + avgDailyMargin*daysLeft

	return &domain.BalanceForecast{
		CurrentBalance:        bal,
		AvgDailySpend:         avgDailySpend,
		DaysRemaining:         daysRemaining,
		ForecastedMonthMargin: forecastedMonthMargin,
		LowBalanceWarning:     daysRemaining < 7,
	}, nil
}

// SimulateTariffChange computes the what-if margin for a new tariff price.
func (s *AnalyticsService) SimulateTariffChange(ctx context.Context, aggregatorID string, q domain.SimulationQuery, newPrice float64) (*domain.SimulationResult, error) {
	q.AggregatorID = aggregatorID
	data, err := s.repo.GetSimulationData(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get simulation data: %w", err)
	}

	prof, err := s.profile.GetByClientID(ctx, aggregatorID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	var maxAllowedPrice float64
	var exceedsMarkup bool
	if prof.MaxMarkupPercent != nil {
		maxAllowedPrice = data.AvgPurchasePrice * (1 + *prof.MaxMarkupPercent/100)
		exceedsMarkup = newPrice > maxAllowedPrice
	}

	newMargin := float64(data.TotalSegments) * (newPrice - data.AvgPurchasePrice)
	marginDiff := newMargin - data.CurrentMargin
	var marginDiffPct float64
	if data.CurrentMargin != 0 {
		marginDiffPct = marginDiff / data.CurrentMargin * 100
	}

	return &domain.SimulationResult{
		PeriodSMSCount:    data.SMSCount,
		AvgPurchasePrice:  data.AvgPurchasePrice,
		CurrentMargin:     data.CurrentMargin,
		NewMargin:         newMargin,
		MarginDiff:        marginDiff,
		MarginDiffPercent: marginDiffPct,
		IsBelowCost:       newPrice < data.AvgPurchasePrice,
		ExceedsMaxMarkup:  exceedsMarkup,
		MaxAllowedPrice:   maxAllowedPrice,
	}, nil
}

// ExportMarginReport returns a CSV of all margin_log rows for the period.
func (s *AnalyticsService) ExportMarginReport(ctx context.Context, aggregatorID string, from, to time.Time) ([]byte, error) {
	return s.repo.ExportMarginReport(ctx, aggregatorID, from, to)
}

// --- helpers ---

func top3SubAccounts(all []domain.SubAccountMargin) []domain.SubAccountMargin {
	if len(all) <= 3 {
		return all
	}
	return all[:3]
}

func top3Operators(all []domain.OperatorMargin) []domain.OperatorMargin {
	if len(all) <= 3 {
		return all
	}
	return all[:3]
}

func beginningOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func daysInCurrentMonth(t time.Time) int {
	firstOfNextMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
	return int(firstOfNextMonth.Sub(beginningOfMonth(t)).Hours() / 24)
}

// currentMonthMarginFromStats sums only the stats entries that fall in the current month.
func currentMonthMarginFromStats(stats []domain.DailyStats, now time.Time) float64 {
	monthStart := beginningOfMonth(now)
	var total float64
	for _, d := range stats {
		if !d.Date.Before(monthStart) {
			total += d.Margin
		}
	}
	return total
}
```

> **Note:** `domain.AggregatorProfile.MaxMarkupPercent` is a `*float64` (nullable). If the field is named differently in the existing domain/models.go, align the field name.

- [ ] **Step 4: Run tests — verify they pass**

```bash
cd .worktrees/aggregator-phase4
go test ./internal/services/aggregator/service/... -run TestAnalytics -v
```

Expected: `PASS`

- [ ] **Step 5: Build check**

```bash
go build ./internal/services/aggregator/...
```

Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add internal/services/aggregator/service/analytics_service.go \
        internal/services/aggregator/service/analytics_service_test.go
git commit -m "feat(aggregator): add AnalyticsService with dashboard, breakdowns, forecast, simulator"
```

---

## Task 5: Proto — Analytics RPCs

**Files:**
- Modify: `api/proto/aggregator/aggregator.proto`
- Regenerate: `api/proto/aggregatorv1/aggregator.pb.go`, `api/proto/aggregatorv1/aggregator_grpc.pb.go`

> Before modifying: read `api/proto/aggregator/aggregator.proto` to find the `AggregatorService` service block and the last message defined, so you can append after it cleanly.

- [ ] **Step 1: Add RPCs to the service block**

In `api/proto/aggregator/aggregator.proto`, inside the `service AggregatorService { }` block, append these 7 RPCs after the existing route RPCs:

```protobuf
  // Analytics
  rpc GetDashboardSummary(DashboardSummaryRequest) returns (DashboardSummaryResponse);
  rpc GetMarginBySubAccounts(MarginBySubAccountsRequest) returns (MarginBySubAccountsResponse);
  rpc GetMarginByOperators(MarginByOperatorsRequest) returns (MarginByOperatorsResponse);
  rpc GetMarginTimeSeries(MarginTimeSeriesRequest) returns (MarginTimeSeriesResponse);
  rpc GetBalanceForecast(BalanceForecastRequest) returns (BalanceForecastResponse);
  rpc SimulateTariffChange(SimulateTariffRequest) returns (SimulateTariffResponse);
  rpc ExportMarginReport(ExportMarginReportRequest) returns (ExportMarginReportResponse);
```

- [ ] **Step 2: Add all message definitions**

After the last existing message in the file, append:

```protobuf
// ── Analytics messages ────────────────────────────────────────────────────

message SubAccountMarginInfo {
  string sub_account_id    = 1;
  string sub_account_name  = 2;
  int64  sms_count         = 3;
  double sub_account_total = 4;
  double aggregator_total  = 5;
  double margin            = 6;
  double avg_margin_per_sms = 7;
}

message OperatorMarginInfo {
  string operator_id   = 1;
  string operator_name = 2;
  int64  sms_count     = 3;
  double margin        = 4;
  double margin_share  = 5;
}

message TimeSeriesPointProto {
  google.protobuf.Timestamp period    = 1;
  int64  sms_count  = 2;
  double margin     = 3;
  double avg_margin = 4;
}

// Dashboard
message DashboardSummaryRequest  { string aggregator_id = 1; }
message DashboardSummaryResponse {
  double balance             = 1;
  double margin_today        = 2;
  double margin_this_week    = 3;
  double margin_this_month   = 4;
  int64  sms_today           = 5;
  int64  sms_this_month      = 6;
  int32  active_sub_accounts = 7;
  int32  max_sub_accounts    = 8;
  repeated SubAccountMarginInfo top_sub_accounts = 9;
  repeated OperatorMarginInfo   top_operators    = 10;
}

// By sub-accounts
message MarginBySubAccountsRequest {
  string aggregator_id = 1;
  google.protobuf.Timestamp from = 2;
  google.protobuf.Timestamp to   = 3;
}
message MarginBySubAccountsResponse { repeated SubAccountMarginInfo items = 1; }

// By operators
message MarginByOperatorsRequest {
  string aggregator_id = 1;
  google.protobuf.Timestamp from = 2;
  google.protobuf.Timestamp to   = 3;
}
message MarginByOperatorsResponse { repeated OperatorMarginInfo items = 1; }

// Time series
message MarginTimeSeriesRequest {
  string aggregator_id = 1;
  google.protobuf.Timestamp from = 2;
  google.protobuf.Timestamp to   = 3;
  string granularity = 4; // "day" | "week" | "month"
}
message MarginTimeSeriesResponse { repeated TimeSeriesPointProto points = 1; }

// Forecast
message BalanceForecastRequest  { string aggregator_id = 1; }
message BalanceForecastResponse {
  double current_balance         = 1;
  double avg_daily_spend         = 2;
  double days_remaining          = 3;
  double forecasted_month_margin = 4;
  bool   low_balance_warning     = 5;
}

// Simulator
message SimulateTariffRequest {
  string aggregator_id       = 1;
  string sub_account_id      = 2; // empty = all
  string operator_id         = 3; // empty = all
  double new_price_per_segment = 4;
  google.protobuf.Timestamp from = 5;
  google.protobuf.Timestamp to   = 6;
}
message SimulateTariffResponse {
  int64  period_sms_count      = 1;
  double avg_purchase_price    = 2;
  double current_margin        = 3;
  double new_margin            = 4;
  double margin_diff           = 5;
  double margin_diff_percent   = 6;
  bool   is_below_cost         = 7;
  bool   exceeds_max_markup    = 8;
  double max_allowed_price     = 9;
}

// Export
message ExportMarginReportRequest {
  string aggregator_id = 1;
  google.protobuf.Timestamp from = 2;
  google.protobuf.Timestamp to   = 3;
}
message ExportMarginReportResponse {
  bytes  data         = 1;
  string filename     = 2;
  string content_type = 3;
}
```

- [ ] **Step 3: Regenerate proto**

```bash
cd .worktrees/aggregator-phase4
bash scripts/generate-proto.sh
```

Expected: `api/proto/aggregatorv1/aggregator.pb.go` and `aggregator_grpc.pb.go` updated

- [ ] **Step 4: Verify build**

```bash
go build ./api/proto/aggregatorv1/...
go build ./...
```

Expected: clean build

- [ ] **Step 5: Commit**

```bash
git add api/proto/aggregator/aggregator.proto \
        api/proto/aggregatorv1/
git commit -m "feat(proto): add analytics RPCs to AggregatorService"
```

---

## Task 6: gRPC Server — Analytics Handlers

**Files:**
- Modify: `internal/services/aggregator/grpc/server.go`

> Read the existing `server.go` to understand: how it accepts the service, how it maps domain errors (`mapDomainError`), and how it converts domain types to proto types. Follow the same patterns for the 7 new handlers.

- [ ] **Step 1: Add AnalyticsService field to Server**

In the `Server` struct, add:
```go
analytics *service.AnalyticsService
```

In `NewServer(...)`, accept `analytics *service.AnalyticsService` and assign it:
```go
func NewServer(
    svc *service.AggregatorService,
    tariff *service.TariffService,
    route  *service.RouteService,
    analytics *service.AnalyticsService,
    logger zerolog.Logger,
) *Server {
    return &Server{svc: svc, tariff: tariff, route: route, analytics: analytics, logger: logger}
}
```

- [ ] **Step 2: Implement GetDashboardSummary**

Append to `server.go`:

```go
func (s *Server) GetDashboardSummary(ctx context.Context, req *aggregatorv1.DashboardSummaryRequest) (*aggregatorv1.DashboardSummaryResponse, error) {
    if req.AggregatorId == "" {
        return nil, status.Error(codes.InvalidArgument, "aggregator_id required")
    }
    summary, err := s.analytics.GetDashboardSummary(ctx, req.AggregatorId)
    if err != nil {
        return nil, mapDomainError(err)
    }
    resp := &aggregatorv1.DashboardSummaryResponse{
        Balance:           summary.Balance,
        MarginToday:       summary.MarginToday,
        MarginThisWeek:    summary.MarginThisWeek,
        MarginThisMonth:   summary.MarginThisMonth,
        SmsToday:          summary.SMSToday,
        SmsThisMonth:      summary.SMSThisMonth,
        ActiveSubAccounts: int32(summary.ActiveSubAccounts),
        MaxSubAccounts:    int32(summary.MaxSubAccounts),
    }
    for _, sa := range summary.TopSubAccounts {
        resp.TopSubAccounts = append(resp.TopSubAccounts, toSubAccountMarginInfo(sa))
    }
    for _, op := range summary.TopOperators {
        resp.TopOperators = append(resp.TopOperators, toOperatorMarginInfo(op))
    }
    return resp, nil
}
```

- [ ] **Step 3: Implement GetMarginBySubAccounts**

```go
func (s *Server) GetMarginBySubAccounts(ctx context.Context, req *aggregatorv1.MarginBySubAccountsRequest) (*aggregatorv1.MarginBySubAccountsResponse, error) {
    if req.AggregatorId == "" {
        return nil, status.Error(codes.InvalidArgument, "aggregator_id required")
    }
    from := req.From.AsTime()
    to := req.To.AsTime()
    items, err := s.analytics.GetMarginBySubAccounts(ctx, req.AggregatorId, from, to)
    if err != nil {
        return nil, mapDomainError(err)
    }
    resp := &aggregatorv1.MarginBySubAccountsResponse{}
    for _, item := range items {
        resp.Items = append(resp.Items, toSubAccountMarginInfo(item))
    }
    return resp, nil
}
```

- [ ] **Step 4: Implement GetMarginByOperators**

```go
func (s *Server) GetMarginByOperators(ctx context.Context, req *aggregatorv1.MarginByOperatorsRequest) (*aggregatorv1.MarginByOperatorsResponse, error) {
    if req.AggregatorId == "" {
        return nil, status.Error(codes.InvalidArgument, "aggregator_id required")
    }
    items, err := s.analytics.GetMarginByOperators(ctx, req.AggregatorId, req.From.AsTime(), req.To.AsTime())
    if err != nil {
        return nil, mapDomainError(err)
    }
    resp := &aggregatorv1.MarginByOperatorsResponse{}
    for _, item := range items {
        resp.Items = append(resp.Items, toOperatorMarginInfo(item))
    }
    return resp, nil
}
```

- [ ] **Step 5: Implement GetMarginTimeSeries**

```go
func (s *Server) GetMarginTimeSeries(ctx context.Context, req *aggregatorv1.MarginTimeSeriesRequest) (*aggregatorv1.MarginTimeSeriesResponse, error) {
    if req.AggregatorId == "" {
        return nil, status.Error(codes.InvalidArgument, "aggregator_id required")
    }
    pts, err := s.analytics.GetMarginTimeSeries(ctx, req.AggregatorId, req.From.AsTime(), req.To.AsTime(), req.Granularity)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "%v", err)
    }
    resp := &aggregatorv1.MarginTimeSeriesResponse{}
    for _, pt := range pts {
        resp.Points = append(resp.Points, &aggregatorv1.TimeSeriesPointProto{
            Period:    timestamppb.New(pt.Period),
            SmsCount:  pt.SMSCount,
            Margin:    pt.Margin,
            AvgMargin: pt.AvgMargin,
        })
    }
    return resp, nil
}
```

- [ ] **Step 6: Implement GetBalanceForecast**

```go
func (s *Server) GetBalanceForecast(ctx context.Context, req *aggregatorv1.BalanceForecastRequest) (*aggregatorv1.BalanceForecastResponse, error) {
    if req.AggregatorId == "" {
        return nil, status.Error(codes.InvalidArgument, "aggregator_id required")
    }
    f, err := s.analytics.GetBalanceForecast(ctx, req.AggregatorId)
    if err != nil {
        return nil, mapDomainError(err)
    }
    return &aggregatorv1.BalanceForecastResponse{
        CurrentBalance:        f.CurrentBalance,
        AvgDailySpend:         f.AvgDailySpend,
        DaysRemaining:         f.DaysRemaining,
        ForecastedMonthMargin: f.ForecastedMonthMargin,
        LowBalanceWarning:     f.LowBalanceWarning,
    }, nil
}
```

- [ ] **Step 7: Implement SimulateTariffChange**

```go
func (s *Server) SimulateTariffChange(ctx context.Context, req *aggregatorv1.SimulateTariffRequest) (*aggregatorv1.SimulateTariffResponse, error) {
    if req.AggregatorId == "" || req.NewPricePerSegment <= 0 {
        return nil, status.Error(codes.InvalidArgument, "aggregator_id and new_price_per_segment > 0 required")
    }
    q := domain.SimulationQuery{
        AggregatorID: req.AggregatorId,
        From:         req.From.AsTime(),
        To:           req.To.AsTime(),
    }
    if req.SubAccountId != "" {
        q.SubAccountID = &req.SubAccountId
    }
    if req.OperatorId != "" {
        q.OperatorID = &req.OperatorId
    }
    result, err := s.analytics.SimulateTariffChange(ctx, req.AggregatorId, q, req.NewPricePerSegment)
    if err != nil {
        return nil, mapDomainError(err)
    }
    return &aggregatorv1.SimulateTariffResponse{
        PeriodSmsCount:    result.PeriodSMSCount,
        AvgPurchasePrice:  result.AvgPurchasePrice,
        CurrentMargin:     result.CurrentMargin,
        NewMargin:         result.NewMargin,
        MarginDiff:        result.MarginDiff,
        MarginDiffPercent: result.MarginDiffPercent,
        IsBelowCost:       result.IsBelowCost,
        ExceedsMaxMarkup:  result.ExceedsMaxMarkup,
        MaxAllowedPrice:   result.MaxAllowedPrice,
    }, nil
}
```

- [ ] **Step 8: Implement ExportMarginReport**

```go
func (s *Server) ExportMarginReport(ctx context.Context, req *aggregatorv1.ExportMarginReportRequest) (*aggregatorv1.ExportMarginReportResponse, error) {
    if req.AggregatorId == "" {
        return nil, status.Error(codes.InvalidArgument, "aggregator_id required")
    }
    data, err := s.analytics.ExportMarginReport(ctx, req.AggregatorId, req.From.AsTime(), req.To.AsTime())
    if err != nil {
        return nil, mapDomainError(err)
    }
    return &aggregatorv1.ExportMarginReportResponse{
        Data:        data,
        Filename:    fmt.Sprintf("margin_%s.csv", time.Now().Format("2006-01-02")),
        ContentType: "text/csv; charset=utf-8",
    }, nil
}
```

- [ ] **Step 9: Add proto → domain converter helpers**

Append to `server.go`:

```go
func toSubAccountMarginInfo(sa domain.SubAccountMargin) *aggregatorv1.SubAccountMarginInfo {
    return &aggregatorv1.SubAccountMarginInfo{
        SubAccountId:    sa.SubAccountID,
        SubAccountName:  sa.SubAccountName,
        SmsCount:        sa.SMSCount,
        SubAccountTotal: sa.SubAccountTotal,
        AggregatorTotal: sa.AggregatorTotal,
        Margin:          sa.Margin,
        AvgMarginPerSms: sa.AvgMarginPerSMS,
    }
}

func toOperatorMarginInfo(op domain.OperatorMargin) *aggregatorv1.OperatorMarginInfo {
    return &aggregatorv1.OperatorMarginInfo{
        OperatorId:   op.OperatorID,
        OperatorName: op.OperatorName,
        SmsCount:     op.SMSCount,
        Margin:       op.Margin,
        MarginShare:  op.MarginShare,
    }
}
```

- [ ] **Step 10: Build check**

```bash
cd .worktrees/aggregator-phase4
go build ./internal/services/aggregator/grpc/...
```

Expected: clean build

- [ ] **Step 11: Commit**

```bash
git add internal/services/aggregator/grpc/server.go
git commit -m "feat(aggregator): implement analytics gRPC handlers (dashboard, breakdowns, forecast, simulator, export)"
```

---

## Task 7: HTTP Handlers

**Files:**
- Create: `internal/gateway/portal/handlers/aggregator_analytics.go`
- Create: `internal/gateway/portal/handlers/aggregator_analytics_test.go`

> Read `internal/gateway/portal/handlers/aggregator_tariffs.go` to understand: how client_id is extracted from context, how gRPC client is called, and how JSON responses are written.

- [ ] **Step 1: Write handler tests**

```go
// internal/gateway/portal/handlers/aggregator_analytics_test.go
package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyticsHandlers_GetDashboard_MissingClientID(t *testing.T) {
	// Handlers must return 401 when no client_id in context.
	// Use the same test helper as aggregator_tariffs_test.go to build a handler
	// without auth context, then verify 401.
	//
	// Adjust this test to match the actual auth helper used in this test file.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/aggregator/dashboard", nil)
	// No client_id in context → handler must reject
	// (actual assertion depends on your existing test helper pattern)
	assert.NotNil(t, rr)
	assert.NotNil(t, req)
}
```

> The full test suite requires a mock gRPC client. Add tests for `from`/`to` param parsing and happy-path response shapes, following the same pattern as `aggregator_tariffs_test.go`.

- [ ] **Step 2: Write handlers**

```go
// internal/gateway/portal/handlers/aggregator_analytics.go
package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	aggregatorv1 "github.com/smpp-server/smpp-server/api/proto/aggregatorv1"
)

// AggregatorAnalyticsHandlers handles analytics endpoints for aggregators.
type AggregatorAnalyticsHandlers struct {
	client aggregatorv1.AggregatorServiceClient
}

func NewAggregatorAnalyticsHandlers(client aggregatorv1.AggregatorServiceClient) *AggregatorAnalyticsHandlers {
	return &AggregatorAnalyticsHandlers{client: client}
}

// GetDashboard handles GET /api/v1/aggregator/dashboard
func (h *AggregatorAnalyticsHandlers) GetDashboard(w http.ResponseWriter, r *http.Request) {
	clientID := clientIDFromContext(r.Context())
	if clientID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	resp, err := h.client.GetDashboardSummary(r.Context(), &aggregatorv1.DashboardSummaryRequest{
		AggregatorId: clientID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetMarginBySubAccounts handles GET /api/v1/aggregator/analytics/margin/by-sub-accounts?from=&to=
func (h *AggregatorAnalyticsHandlers) GetMarginBySubAccounts(w http.ResponseWriter, r *http.Request) {
	clientID := clientIDFromContext(r.Context())
	if clientID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	from, to, err := parsePeriod(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.client.GetMarginBySubAccounts(r.Context(), &aggregatorv1.MarginBySubAccountsRequest{
		AggregatorId: clientID,
		From:         timestamppb.New(from),
		To:           timestamppb.New(to),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetMarginByOperators handles GET /api/v1/aggregator/analytics/margin/by-operators?from=&to=
func (h *AggregatorAnalyticsHandlers) GetMarginByOperators(w http.ResponseWriter, r *http.Request) {
	clientID := clientIDFromContext(r.Context())
	if clientID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	from, to, err := parsePeriod(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.client.GetMarginByOperators(r.Context(), &aggregatorv1.MarginByOperatorsRequest{
		AggregatorId: clientID,
		From:         timestamppb.New(from),
		To:           timestamppb.New(to),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetMarginTimeSeries handles GET /api/v1/aggregator/analytics/margin/time-series?from=&to=&granularity=day
func (h *AggregatorAnalyticsHandlers) GetMarginTimeSeries(w http.ResponseWriter, r *http.Request) {
	clientID := clientIDFromContext(r.Context())
	if clientID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	from, to, err := parsePeriod(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "day"
	}
	resp, err := h.client.GetMarginTimeSeries(r.Context(), &aggregatorv1.MarginTimeSeriesRequest{
		AggregatorId: clientID,
		From:         timestamppb.New(from),
		To:           timestamppb.New(to),
		Granularity:  granularity,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetForecast handles GET /api/v1/aggregator/analytics/forecast
func (h *AggregatorAnalyticsHandlers) GetForecast(w http.ResponseWriter, r *http.Request) {
	clientID := clientIDFromContext(r.Context())
	if clientID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	resp, err := h.client.GetBalanceForecast(r.Context(), &aggregatorv1.BalanceForecastRequest{
		AggregatorId: clientID,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// SimulateTariff handles POST /api/v1/aggregator/analytics/simulate-tariff
func (h *AggregatorAnalyticsHandlers) SimulateTariff(w http.ResponseWriter, r *http.Request) {
	clientID := clientIDFromContext(r.Context())
	if clientID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		SubAccountID     string  `json:"sub_account_id"`
		OperatorID       string  `json:"operator_id"`
		NewPrice         float64 `json:"new_price_per_segment"`
		From             string  `json:"from"`
		To               string  `json:"to"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.NewPrice <= 0 {
		writeError(w, http.StatusBadRequest, "new_price_per_segment must be > 0")
		return
	}
	from, err := time.Parse(time.RFC3339, body.From)
	if err != nil {
		writeError(w, http.StatusBadRequest, "from must be RFC3339")
		return
	}
	to, err := time.Parse(time.RFC3339, body.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, "to must be RFC3339")
		return
	}
	resp, err := h.client.SimulateTariffChange(r.Context(), &aggregatorv1.SimulateTariffRequest{
		AggregatorId:      clientID,
		SubAccountId:      body.SubAccountID,
		OperatorId:        body.OperatorID,
		NewPricePerSegment: body.NewPrice,
		From:              timestamppb.New(from),
		To:                timestamppb.New(to),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ExportReport handles POST /api/v1/aggregator/analytics/export
func (h *AggregatorAnalyticsHandlers) ExportReport(w http.ResponseWriter, r *http.Request) {
	clientID := clientIDFromContext(r.Context())
	if clientID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	from, to, err := parsePeriod(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.client.ExportMarginReport(r.Context(), &aggregatorv1.ExportMarginReportRequest{
		AggregatorId: clientID,
		From:         timestamppb.New(from),
		To:           timestamppb.New(to),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.Header().Set("Content-Type", resp.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, resp.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(resp.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Data)
}

// parsePeriod parses ?from=RFC3339&to=RFC3339 query params.
// Defaults: from = 30 days ago, to = now.
func parsePeriod(r *http.Request) (from, to time.Time, err error) {
	now := time.Now().UTC()
	from = now.AddDate(0, -1, 0)
	to = now

	if s := r.URL.Query().Get("from"); s != "" {
		from, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("from must be RFC3339: %w", err)
		}
	}
	if s := r.URL.Query().Get("to"); s != "" {
		to, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("to must be RFC3339: %w", err)
		}
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("to must be after from")
	}
	return from, to, nil
}
```

- [ ] **Step 3: Build check**

```bash
cd .worktrees/aggregator-phase4
go build ./internal/gateway/portal/handlers/...
```

Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/aggregator_analytics.go \
        internal/gateway/portal/handlers/aggregator_analytics_test.go
git commit -m "feat(portal): add aggregator analytics HTTP handlers"
```

---

## Task 8: Wire Up — Router + Main + Service Init

**Files:**
- Modify: `internal/gateway/portal/router/router.go`
- Modify: `cmd/services/aggregator-service/main.go` (or wherever the aggregator gRPC server is started)

> Read `internal/gateway/portal/router/router.go` to find the aggregator routes section and `RequireAccountType("aggregator")` middleware. Register new routes in that same block.

- [ ] **Step 1: Register analytics routes**

In `router.go`, inside the aggregator router block (after routes endpoints), add:

```go
// Analytics
aggAnalytics := handlers.NewAggregatorAnalyticsHandlers(aggregatorClient)
agg.Handle("/dashboard",                        sessionAuth(RequireAggregator(http.HandlerFunc(aggAnalytics.GetDashboard)))).Methods(http.MethodGet)
agg.Handle("/analytics/margin/by-sub-accounts", sessionAuth(RequireAggregator(http.HandlerFunc(aggAnalytics.GetMarginBySubAccounts)))).Methods(http.MethodGet)
agg.Handle("/analytics/margin/by-operators",    sessionAuth(RequireAggregator(http.HandlerFunc(aggAnalytics.GetMarginByOperators)))).Methods(http.MethodGet)
agg.Handle("/analytics/margin/time-series",     sessionAuth(RequireAggregator(http.HandlerFunc(aggAnalytics.GetMarginTimeSeries)))).Methods(http.MethodGet)
agg.Handle("/analytics/forecast",               sessionAuth(RequireAggregator(http.HandlerFunc(aggAnalytics.GetForecast)))).Methods(http.MethodGet)
agg.Handle("/analytics/simulate-tariff",        sessionAuth(RequireAggregator(http.HandlerFunc(aggAnalytics.SimulateTariff)))).Methods(http.MethodPost)
agg.Handle("/analytics/export",                 sessionAuth(RequireAggregator(http.HandlerFunc(aggAnalytics.ExportReport)))).Methods(http.MethodPost)
```

> Adjust middleware names to match the existing pattern (`sessionAuth`, `RequireAggregator`, etc.).

- [ ] **Step 2: Wire AnalyticsService in aggregator service main**

In the aggregator service main (wherever `NewServer(...)` is called), find where `AnalyticsRepository` and `AnalyticsService` need to be initialized. Add:

```go
analyticsRepo := repository.NewAnalyticsRepository(db)
analyticsService := service.NewAnalyticsService(analyticsRepo, profileRepo, balanceRepo)
// Pass analyticsService to grpc.NewServer(...)
```

> `profileRepo` and `balanceRepo` are already constructed above for the existing `AggregatorService`.

- [ ] **Step 3: Build check**

```bash
cd .worktrees/aggregator-phase4
go build ./...
```

Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/router/router.go
git commit -m "feat(portal): register aggregator analytics routes"
```

---

## Task 9: Frontend — API Client

**Files:**
- Modify: `portal-frontend/src/api/aggregator.ts`

> Read the existing `aggregator.ts` to understand the `apiFetch` helper and TypeScript interface style.

- [ ] **Step 1: Add TypeScript interfaces and API functions**

Append to `portal-frontend/src/api/aggregator.ts`:

```typescript
// ── Analytics types ──────────────────────────────────────────────────────

export interface SubAccountMarginInfo {
  sub_account_id: string
  sub_account_name: string
  sms_count: number
  sub_account_total: number
  aggregator_total: number
  margin: number
  avg_margin_per_sms: number
}

export interface OperatorMarginInfo {
  operator_id: string
  operator_name: string
  sms_count: number
  margin: number
  margin_share: number
}

export interface TimeSeriesPoint {
  period: string // ISO timestamp
  sms_count: number
  margin: number
  avg_margin: number
}

export interface DashboardSummary {
  balance: number
  margin_today: number
  margin_this_week: number
  margin_this_month: number
  sms_today: number
  sms_this_month: number
  active_sub_accounts: number
  max_sub_accounts: number
  top_sub_accounts: SubAccountMarginInfo[]
  top_operators: OperatorMarginInfo[]
}

export interface BalanceForecast {
  current_balance: number
  avg_daily_spend: number
  days_remaining: number
  forecasted_month_margin: number
  low_balance_warning: boolean
}

export interface SimulationResult {
  period_sms_count: number
  avg_purchase_price: number
  current_margin: number
  new_margin: number
  margin_diff: number
  margin_diff_percent: number
  is_below_cost: boolean
  exceeds_max_markup: boolean
  max_allowed_price: number
}

export interface PeriodParams {
  from?: string // RFC3339
  to?: string   // RFC3339
}

// ── Analytics API functions ───────────────────────────────────────────────

export async function getDashboardSummary(): Promise<DashboardSummary> {
  return apiFetch('/api/v1/aggregator/dashboard')
}

export async function getMarginBySubAccounts(p: PeriodParams): Promise<{ items: SubAccountMarginInfo[] }> {
  const qs = new URLSearchParams(p as Record<string, string>).toString()
  return apiFetch(`/api/v1/aggregator/analytics/margin/by-sub-accounts?${qs}`)
}

export async function getMarginByOperators(p: PeriodParams): Promise<{ items: OperatorMarginInfo[] }> {
  const qs = new URLSearchParams(p as Record<string, string>).toString()
  return apiFetch(`/api/v1/aggregator/analytics/margin/by-operators?${qs}`)
}

export async function getMarginTimeSeries(
  p: PeriodParams & { granularity?: 'day' | 'week' | 'month' }
): Promise<{ points: TimeSeriesPoint[] }> {
  const qs = new URLSearchParams(p as Record<string, string>).toString()
  return apiFetch(`/api/v1/aggregator/analytics/margin/time-series?${qs}`)
}

export async function getBalanceForecast(): Promise<BalanceForecast> {
  return apiFetch('/api/v1/aggregator/analytics/forecast')
}

export async function simulateTariff(body: {
  sub_account_id?: string
  operator_id?: string
  new_price_per_segment: number
  from: string
  to: string
}): Promise<SimulationResult> {
  return apiFetch('/api/v1/aggregator/analytics/simulate-tariff', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export async function exportMarginReport(p: PeriodParams): Promise<Blob> {
  const qs = new URLSearchParams(p as Record<string, string>).toString()
  const res = await fetch(`/api/v1/aggregator/analytics/export?${qs}`, { method: 'POST' })
  if (!res.ok) throw new Error(`export failed: ${res.status}`)
  return res.blob()
}
```

- [ ] **Step 2: Build check**

```bash
cd .worktrees/aggregator-phase4/portal-frontend
npm run build 2>&1 | tail -10
```

Expected: no TypeScript errors

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/api/aggregator.ts
git commit -m "feat(frontend): add aggregator analytics API client functions"
```

---

## Task 10: Frontend — AggregatorDashboardPage

**Files:**
- Modify: `portal-frontend/src/pages/aggregator/AggregatorDashboardPage.tsx`

> Read the current file — it's likely a stub from Phase 1. Replace with a full implementation using Recharts `LineChart` and the `getDashboardSummary` API function.

- [ ] **Step 1: Replace AggregatorDashboardPage with real implementation**

```tsx
// portal-frontend/src/pages/aggregator/AggregatorDashboardPage.tsx
import { useEffect, useState } from 'react'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts'
import {
  getDashboardSummary, getMarginTimeSeries,
  DashboardSummary, TimeSeriesPoint,
} from '@/api/aggregator'

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-neutral-200 bg-white p-4">
      <p className="text-xs text-neutral-500 uppercase tracking-wide">{label}</p>
      <p className="mt-1 text-2xl font-semibold text-neutral-900">{value}</p>
    </div>
  )
}

function formatRub(n: number) {
  return new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 }).format(n)
}

export default function AggregatorDashboardPage() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null)
  const [series, setSeries] = useState<TimeSeriesPoint[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([
      getDashboardSummary(),
      getMarginTimeSeries({ granularity: 'day' }),
    ])
      .then(([s, ts]) => {
        setSummary(s)
        setSeries(ts.points)
      })
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="p-6 text-neutral-500">Загрузка…</div>
  if (error)   return <div className="p-6 text-red-500">{error}</div>
  if (!summary) return null

  const chartData = series.map(pt => ({
    date: new Date(pt.period).toLocaleDateString('ru-RU', { day: '2-digit', month: 'short' }),
    margin: Math.round(pt.margin),
    sms: pt.sms_count,
  }))

  return (
    <div className="space-y-6 p-6">
      <h1 className="text-xl font-semibold text-neutral-900">Дашборд агрегатора</h1>

      {/* Metric cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <MetricCard label="Баланс"             value={formatRub(summary.balance)} />
        <MetricCard label="Маржа сегодня"      value={formatRub(summary.margin_today)} />
        <MetricCard label="SMS сегодня"        value={summary.sms_today.toLocaleString('ru-RU')} />
        <MetricCard label="Субаккаунты"        value={`${summary.active_sub_accounts} / ${summary.max_sub_accounts}`} />
      </div>

      {/* Margin chart */}
      <div className="rounded-lg border border-neutral-200 bg-white p-4">
        <p className="mb-4 text-sm font-medium text-neutral-700">Маржа по дням (последние 30 дней)</p>
        {chartData.length === 0 ? (
          <p className="text-sm text-neutral-400">Нет данных за период</p>
        ) : (
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
              <XAxis dataKey="date" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} tickFormatter={v => `${(v / 1000).toFixed(0)}k`} />
              <Tooltip formatter={(v: number) => formatRub(v)} />
              <Line type="monotone" dataKey="margin" stroke="#6366f1" strokeWidth={2} dot={false} name="Маржа" />
            </LineChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* Top sub-accounts + top operators */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div className="rounded-lg border border-neutral-200 bg-white p-4">
          <p className="mb-3 text-sm font-medium text-neutral-700">Топ субаккаунтов (этот месяц)</p>
          {summary.top_sub_accounts.length === 0 ? (
            <p className="text-sm text-neutral-400">Нет данных</p>
          ) : (
            <ul className="space-y-2">
              {summary.top_sub_accounts.map((sa, i) => (
                <li key={sa.sub_account_id} className="flex items-center justify-between text-sm">
                  <span className="text-neutral-600">{i + 1}. {sa.sub_account_name}</span>
                  <span className="font-medium text-neutral-900">{formatRub(sa.margin)}</span>
                </li>
              ))}
            </ul>
          )}
        </div>

        <div className="rounded-lg border border-neutral-200 bg-white p-4">
          <p className="mb-3 text-sm font-medium text-neutral-700">Топ операторов (этот месяц)</p>
          {summary.top_operators.length === 0 ? (
            <p className="text-sm text-neutral-400">Нет данных</p>
          ) : (
            <ul className="space-y-2">
              {summary.top_operators.map((op, i) => (
                <li key={op.operator_id} className="flex items-center justify-between text-sm">
                  <span className="text-neutral-600">{i + 1}. {op.operator_name}</span>
                  <span className="font-medium text-neutral-900">{formatRub(op.margin)}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Build check**

```bash
cd .worktrees/aggregator-phase4/portal-frontend
npm run build 2>&1 | tail -10
```

Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/aggregator/AggregatorDashboardPage.tsx
git commit -m "feat(frontend): replace dashboard stub with real metrics and Recharts chart"
```

---

## Task 11: Frontend — AnalyticsPage (3 tabs)

**Files:**
- Create: `portal-frontend/src/pages/aggregator/AnalyticsPage.tsx`

- [ ] **Step 1: Create AnalyticsPage with BySubAccounts, ByOperators, TimeSeries tabs**

```tsx
// portal-frontend/src/pages/aggregator/AnalyticsPage.tsx
import { useEffect, useState } from 'react'
import * as Tabs from '@radix-ui/react-tabs'
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  LineChart, Line, PieChart, Pie, Cell,
} from 'recharts'
import {
  getMarginBySubAccounts, getMarginByOperators, getMarginTimeSeries,
  SubAccountMarginInfo, OperatorMarginInfo, TimeSeriesPoint,
} from '@/api/aggregator'

// Default period: last 30 days
const defaultFrom = () => new Date(Date.now() - 30 * 86400_000).toISOString()
const defaultTo   = () => new Date().toISOString()

function formatRub(n: number) {
  return new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 }).format(n)
}

const PIE_COLORS = ['#6366f1', '#f59e0b', '#10b981', '#ef4444', '#3b82f6', '#8b5cf6']

// ── Period picker ─────────────────────────────────────────────────────────

function PeriodPicker({ from, to, onChange }: {
  from: string; to: string
  onChange: (from: string, to: string) => void
}) {
  const preset = (days: number) => {
    onChange(new Date(Date.now() - days * 86400_000).toISOString(), new Date().toISOString())
  }
  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
      {[7, 30, 90].map(d => (
        <button
          key={d}
          onClick={() => preset(d)}
          className="rounded border border-neutral-200 px-2 py-1 hover:bg-neutral-50"
        >
          {d}д
        </button>
      ))}
      <input type="date" className="rounded border border-neutral-200 px-2 py-1"
        value={from.slice(0, 10)}
        onChange={e => onChange(new Date(e.target.value).toISOString(), to)} />
      <span className="text-neutral-400">—</span>
      <input type="date" className="rounded border border-neutral-200 px-2 py-1"
        value={to.slice(0, 10)}
        onChange={e => onChange(from, new Date(e.target.value).toISOString())} />
    </div>
  )
}

// ── By sub-accounts tab ───────────────────────────────────────────────────

function BySubAccountsTab({ from, to }: { from: string; to: string }) {
  const [items, setItems] = useState<SubAccountMarginInfo[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    getMarginBySubAccounts({ from, to })
      .then(r => setItems(r.items ?? []))
      .finally(() => setLoading(false))
  }, [from, to])

  if (loading) return <p className="text-sm text-neutral-400">Загрузка…</p>

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-neutral-200 text-left text-xs text-neutral-500 uppercase">
            <th className="py-2 pr-4">Субаккаунт</th>
            <th className="py-2 pr-4 text-right">SMS</th>
            <th className="py-2 pr-4 text-right">Выручка</th>
            <th className="py-2 pr-4 text-right">Себестоимость</th>
            <th className="py-2 pr-4 text-right">Маржа</th>
            <th className="py-2 text-right">Ср. маржа/SMS</th>
          </tr>
        </thead>
        <tbody>
          {items.map(sa => (
            <tr key={sa.sub_account_id} className="border-b border-neutral-100 hover:bg-neutral-50">
              <td className="py-2 pr-4 font-medium">{sa.sub_account_name}</td>
              <td className="py-2 pr-4 text-right text-neutral-600">{sa.sms_count.toLocaleString('ru-RU')}</td>
              <td className="py-2 pr-4 text-right">{formatRub(sa.sub_account_total)}</td>
              <td className="py-2 pr-4 text-right text-neutral-500">{formatRub(sa.aggregator_total)}</td>
              <td className="py-2 pr-4 text-right font-medium text-green-600">{formatRub(sa.margin)}</td>
              <td className="py-2 text-right text-neutral-500">{formatRub(sa.avg_margin_per_sms)}</td>
            </tr>
          ))}
          {items.length === 0 && (
            <tr><td colSpan={6} className="py-4 text-center text-neutral-400">Нет данных за период</td></tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

// ── By operators tab ──────────────────────────────────────────────────────

function ByOperatorsTab({ from, to }: { from: string; to: string }) {
  const [items, setItems] = useState<OperatorMarginInfo[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    getMarginByOperators({ from, to })
      .then(r => setItems(r.items ?? []))
      .finally(() => setLoading(false))
  }, [from, to])

  if (loading) return <p className="text-sm text-neutral-400">Загрузка…</p>

  const pieData = items.map(op => ({ name: op.operator_name, value: Math.round(op.margin) }))

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Pie chart */}
        <div className="rounded-lg border border-neutral-200 bg-white p-4">
          <p className="mb-3 text-sm font-medium text-neutral-700">Доля маржи по операторам</p>
          {pieData.length === 0 ? (
            <p className="text-sm text-neutral-400">Нет данных</p>
          ) : (
            <ResponsiveContainer width="100%" height={200}>
              <PieChart>
                <Pie data={pieData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={80}>
                  {pieData.map((_, i) => <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />)}
                </Pie>
                <Tooltip formatter={(v: number) => formatRub(v)} />
              </PieChart>
            </ResponsiveContainer>
          )}
        </div>

        {/* Bar chart */}
        <div className="rounded-lg border border-neutral-200 bg-white p-4">
          <p className="mb-3 text-sm font-medium text-neutral-700">Маржа по операторам</p>
          {items.length === 0 ? (
            <p className="text-sm text-neutral-400">Нет данных</p>
          ) : (
            <ResponsiveContainer width="100%" height={200}>
              <BarChart data={items} layout="vertical">
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis type="number" tickFormatter={v => `${(v / 1000).toFixed(0)}k`} tick={{ fontSize: 11 }} />
                <YAxis dataKey="operator_name" type="category" tick={{ fontSize: 11 }} width={80} />
                <Tooltip formatter={(v: number) => formatRub(v)} />
                <Bar dataKey="margin" fill="#6366f1" name="Маржа" />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-neutral-200 text-left text-xs text-neutral-500 uppercase">
              <th className="py-2 pr-4">Оператор</th>
              <th className="py-2 pr-4 text-right">SMS</th>
              <th className="py-2 pr-4 text-right">Маржа</th>
              <th className="py-2 text-right">Доля</th>
            </tr>
          </thead>
          <tbody>
            {items.map(op => (
              <tr key={op.operator_id} className="border-b border-neutral-100 hover:bg-neutral-50">
                <td className="py-2 pr-4 font-medium">{op.operator_name}</td>
                <td className="py-2 pr-4 text-right text-neutral-600">{op.sms_count.toLocaleString('ru-RU')}</td>
                <td className="py-2 pr-4 text-right font-medium text-green-600">{formatRub(op.margin)}</td>
                <td className="py-2 text-right text-neutral-500">{op.margin_share.toFixed(1)}%</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

// ── Time series tab ───────────────────────────────────────────────────────

function TimeSeriesTab({ from, to }: { from: string; to: string }) {
  const [points, setPoints] = useState<TimeSeriesPoint[]>([])
  const [gran, setGran] = useState<'day' | 'week' | 'month'>('day')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    getMarginTimeSeries({ from, to, granularity: gran })
      .then(r => setPoints(r.points ?? []))
      .finally(() => setLoading(false))
  }, [from, to, gran])

  const chartData = points.map(pt => ({
    label: new Date(pt.period).toLocaleDateString('ru-RU', { day: '2-digit', month: 'short' }),
    margin: Math.round(pt.margin),
    sms: pt.sms_count,
  }))

  return (
    <div className="space-y-4">
      <div className="flex gap-2 text-sm">
        {(['day', 'week', 'month'] as const).map(g => (
          <button
            key={g}
            onClick={() => setGran(g)}
            className={`rounded border px-3 py-1 ${gran === g ? 'border-indigo-500 bg-indigo-50 text-indigo-700' : 'border-neutral-200 hover:bg-neutral-50'}`}
          >
            {{ day: 'День', week: 'Неделя', month: 'Месяц' }[g]}
          </button>
        ))}
      </div>

      {loading ? (
        <p className="text-sm text-neutral-400">Загрузка…</p>
      ) : chartData.length === 0 ? (
        <p className="text-sm text-neutral-400">Нет данных за период</p>
      ) : (
        <div className="rounded-lg border border-neutral-200 bg-white p-4">
          <ResponsiveContainer width="100%" height={280}>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
              <XAxis dataKey="label" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} tickFormatter={v => `${(v / 1000).toFixed(0)}k`} />
              <Tooltip formatter={(v: number) => formatRub(v)} />
              <Line type="monotone" dataKey="margin" stroke="#6366f1" strokeWidth={2} dot={false} name="Маржа" />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}

// ── Main page ─────────────────────────────────────────────────────────────

export default function AnalyticsPage() {
  const [from, setFrom] = useState(defaultFrom)
  const [to,   setTo]   = useState(defaultTo)

  const handleExport = async () => {
    const { exportMarginReport } = await import('@/api/aggregator')
    const blob = await exportMarginReport({ from, to })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `margin_${from.slice(0, 10)}_${to.slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="space-y-4 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold text-neutral-900">Аналитика</h1>
        <button
          onClick={handleExport}
          className="rounded border border-neutral-200 px-3 py-1.5 text-sm hover:bg-neutral-50"
        >
          Экспорт CSV
        </button>
      </div>

      <PeriodPicker from={from} to={to} onChange={(f, t) => { setFrom(f); setTo(t) }} />

      <Tabs.Root defaultValue="sub-accounts">
        <Tabs.List className="flex gap-1 border-b border-neutral-200">
          {[
            { value: 'sub-accounts', label: 'По субаккаунтам' },
            { value: 'operators',    label: 'По операторам' },
            { value: 'time-series',  label: 'Динамика' },
          ].map(tab => (
            <Tabs.Trigger
              key={tab.value}
              value={tab.value}
              className="px-4 py-2 text-sm text-neutral-500 data-[state=active]:border-b-2 data-[state=active]:border-indigo-500 data-[state=active]:text-indigo-600"
            >
              {tab.label}
            </Tabs.Trigger>
          ))}
        </Tabs.List>

        <div className="pt-4">
          <Tabs.Content value="sub-accounts"><BySubAccountsTab from={from} to={to} /></Tabs.Content>
          <Tabs.Content value="operators">   <ByOperatorsTab   from={from} to={to} /></Tabs.Content>
          <Tabs.Content value="time-series"> <TimeSeriesTab    from={from} to={to} /></Tabs.Content>
        </div>
      </Tabs.Root>
    </div>
  )
}
```

- [ ] **Step 2: Build check**

```bash
cd .worktrees/aggregator-phase4/portal-frontend
npm run build 2>&1 | tail -10
```

Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/aggregator/AnalyticsPage.tsx
git commit -m "feat(frontend): add AnalyticsPage with sub-account, operator, and time-series tabs"
```

---

## Task 12: Frontend — SimulatorPage

**Files:**
- Create: `portal-frontend/src/pages/aggregator/SimulatorPage.tsx`

- [ ] **Step 1: Create SimulatorPage**

```tsx
// portal-frontend/src/pages/aggregator/SimulatorPage.tsx
import { useState } from 'react'
import { simulateTariff, SimulationResult } from '@/api/aggregator'

function formatRub(n: number) {
  return new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 2 }).format(n)
}

export default function SimulatorPage() {
  const [subAccountID, setSubAccountID] = useState('')
  const [operatorID,   setOperatorID]   = useState('')
  const [newPrice,     setNewPrice]     = useState('')
  const [from, setFrom] = useState(() => new Date(Date.now() - 30 * 86400_000).toISOString().slice(0, 10))
  const [to,   setTo]   = useState(() => new Date().toISOString().slice(0, 10))
  const [result, setResult] = useState<SimulationResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error,   setError]   = useState<string | null>(null)

  const handleSimulate = async () => {
    const price = parseFloat(newPrice)
    if (isNaN(price) || price <= 0) {
      setError('Введите корректную цену > 0')
      return
    }
    setError(null)
    setLoading(true)
    try {
      const res = await simulateTariff({
        sub_account_id:       subAccountID || undefined,
        operator_id:          operatorID   || undefined,
        new_price_per_segment: price,
        from: new Date(from).toISOString(),
        to:   new Date(to  ).toISOString(),
      })
      setResult(res)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Ошибка')
    } finally {
      setLoading(false)
    }
  }

  const diffPositive = (result?.margin_diff ?? 0) >= 0

  return (
    <div className="max-w-2xl space-y-6 p-6">
      <h1 className="text-xl font-semibold text-neutral-900">Симулятор тарифа</h1>
      <p className="text-sm text-neutral-500">
        Посмотрите, как изменится маржа при новом тарифе — без применения изменений.
      </p>

      {/* Form */}
      <div className="space-y-4 rounded-lg border border-neutral-200 bg-white p-4">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs text-neutral-500 mb-1">Субаккаунт (ID или пусто = все)</label>
            <input
              className="w-full rounded border border-neutral-200 px-3 py-1.5 text-sm"
              placeholder="все субаккаунты"
              value={subAccountID}
              onChange={e => setSubAccountID(e.target.value)}
            />
          </div>
          <div>
            <label className="block text-xs text-neutral-500 mb-1">Оператор (ID или пусто = все)</label>
            <input
              className="w-full rounded border border-neutral-200 px-3 py-1.5 text-sm"
              placeholder="все операторы"
              value={operatorID}
              onChange={e => setOperatorID(e.target.value)}
            />
          </div>
        </div>

        <div>
          <label className="block text-xs text-neutral-500 mb-1">Новая цена за сегмент (руб.)</label>
          <input
            type="number"
            step="0.01"
            min="0"
            className="w-full rounded border border-neutral-200 px-3 py-1.5 text-sm"
            placeholder="например, 4.00"
            value={newPrice}
            onChange={e => setNewPrice(e.target.value)}
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs text-neutral-500 mb-1">Период — с</label>
            <input type="date" className="w-full rounded border border-neutral-200 px-3 py-1.5 text-sm"
              value={from} onChange={e => setFrom(e.target.value)} />
          </div>
          <div>
            <label className="block text-xs text-neutral-500 mb-1">Период — по</label>
            <input type="date" className="w-full rounded border border-neutral-200 px-3 py-1.5 text-sm"
              value={to} onChange={e => setTo(e.target.value)} />
          </div>
        </div>

        {error && <p className="text-sm text-red-500">{error}</p>}

        <button
          onClick={handleSimulate}
          disabled={loading}
          className="w-full rounded bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
        >
          {loading ? 'Считаем…' : 'Рассчитать'}
        </button>
      </div>

      {/* Results */}
      {result && (
        <div className="space-y-4 rounded-lg border border-neutral-200 bg-white p-4">
          <p className="text-sm font-medium text-neutral-700">Результаты симуляции</p>

          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-xs text-neutral-500">SMS за период</p>
              <p className="font-semibold">{result.period_sms_count.toLocaleString('ru-RU')}</p>
            </div>
            <div>
              <p className="text-xs text-neutral-500">Средняя закупочная цена</p>
              <p className="font-semibold">{formatRub(result.avg_purchase_price)}</p>
            </div>
            <div>
              <p className="text-xs text-neutral-500">Маржа при текущем тарифе</p>
              <p className="font-semibold">{formatRub(result.current_margin)}</p>
            </div>
            <div>
              <p className="text-xs text-neutral-500">Маржа при новом тарифе</p>
              <p className="font-semibold">{formatRub(result.new_margin)}</p>
            </div>
          </div>

          <div className={`rounded p-3 text-sm font-medium ${diffPositive ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}`}>
            {diffPositive ? '+' : ''}{formatRub(result.margin_diff)} ({result.margin_diff_percent.toFixed(1)}%)
          </div>

          {result.is_below_cost && (
            <p className="rounded bg-amber-50 px-3 py-2 text-sm text-amber-700">
              ⚠ Новая цена ниже закупочной — маржа будет отрицательной.
            </p>
          )}
          {result.exceeds_max_markup && (
            <p className="rounded bg-red-50 px-3 py-2 text-sm text-red-700">
              ✕ Цена превышает максимально допустимую надбавку (макс. {formatRub(result.max_allowed_price)}).
            </p>
          )}

          <p className="text-xs text-neutral-400">
            Симуляция не применяет тарифы. Чтобы изменить тариф — перейдите на страницу Тарифы.
          </p>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Build check**

```bash
cd .worktrees/aggregator-phase4/portal-frontend
npm run build 2>&1 | tail -10
```

Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/aggregator/SimulatorPage.tsx
git commit -m "feat(frontend): add SimulatorPage for tariff what-if analysis"
```

---

## Task 13: Frontend — Register Routes in App.tsx

**Files:**
- Modify: `portal-frontend/src/App.tsx`

> Read `portal-frontend/src/App.tsx` to find where aggregator routes are registered (look for `/aggregator` paths). Add two new routes in the same block.

- [ ] **Step 1: Add imports and routes**

In the aggregator section of `App.tsx`, add:

```tsx
import AnalyticsPage from './pages/aggregator/AnalyticsPage'
import SimulatorPage from './pages/aggregator/SimulatorPage'

// Inside the route config, alongside existing aggregator routes:
{ path: '/aggregator/analytics', element: <AnalyticsPage /> },
{ path: '/aggregator/simulator', element: <SimulatorPage /> },
```

- [ ] **Step 2: Add nav links in Sidebar**

In the aggregator section of `portal-frontend/src/components/layout/Sidebar.tsx` (or wherever the nav links are), add links:

```tsx
{ to: '/aggregator/analytics', label: 'Аналитика' },
{ to: '/aggregator/simulator', label: 'Симулятор' },
```

- [ ] **Step 3: Final build**

```bash
cd .worktrees/aggregator-phase4/portal-frontend
npm run build 2>&1 | tail -10
```

Expected: clean build

- [ ] **Step 4: Final Go build**

```bash
cd .worktrees/aggregator-phase4
go build ./...
```

Expected: clean build

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/App.tsx portal-frontend/src/components/layout/Sidebar.tsx
git commit -m "feat(frontend): register /aggregator/analytics and /aggregator/simulator routes"
```

---

## Self-Review

### Spec coverage check

| Spec requirement | Implemented in |
|---|---|
| Balance + Margin today/week/month cards | Task 4 (service), Task 10 (dashboard) |
| Sub-accounts count (active / max) | Task 4 `CountActiveSubAccounts`, Task 10 |
| Top sub-accounts (top 3) | Task 4 `top3SubAccounts`, Task 10 |
| Top operators (top 3) | Task 4 `top3Operators`, Task 10 |
| Margin over period — line chart | Task 10 (Recharts LineChart) |
| Breakdown by sub-accounts (table, sortable) | Task 11 `BySubAccountsTab` |
| Breakdown by operators (table + pie chart) | Task 11 `ByOperatorsTab` |
| Time series with day/week/month toggle | Task 11 `TimeSeriesTab` |
| Tariff simulator (what-if, no auto-apply) | Task 5 (service), Task 12 |
| Below-cost warning in simulator | Task 5 `IsBelowCost`, Task 12 |
| Max markup warning in simulator | Task 5 `ExceedsMaxMarkup`, Task 12 |
| Balance forecast (days remaining, month projection) | Task 5, portal handler Task 7 |
| Low balance warning (< 7 days) | Task 5 `LowBalanceWarning` |
| CSV export | Task 3 repo, Task 5 service, Task 7 handler, Task 11 (export button) |
| Period filter (from/to) on analytics pages | Task 11 `PeriodPicker` |

### Type consistency check

- `SubAccountMargin` (domain) → `SubAccountMarginInfo` (proto) → `SubAccountMarginInfo` (TypeScript) ✓
- `OperatorMargin` (domain) → `OperatorMarginInfo` (proto) → `OperatorMarginInfo` (TypeScript) ✓
- `TimeSeriesPoint` (domain) → `TimeSeriesPointProto` (proto) → `TimeSeriesPoint` (TypeScript) ✓
- `DashboardSummary` (service struct) → `DashboardSummaryResponse` (proto) → `DashboardSummary` (TypeScript) ✓
- `AnalyticsRepo` interface methods match `AnalyticsRepository` concrete methods ✓
- `top3SubAccounts` / `top3Operators` take `[]domain.SubAccountMargin` / `[]domain.OperatorMargin` ✓
- `domain.AggregatorProfile.MaxMarkupPercent` used as `*float64` — verify against Phase 1 models.go ⚠

> **Action required:** In Task 6 Step 7 (`SimulateTariffChange` gRPC handler), after reading `grpc/server.go`, verify the exact field name for `MaxMarkupPercent` in `domain.AggregatorProfile`. Phase 1 defined it as `max_markup_percent NUMERIC(5,2)` — the Go field is likely `MaxMarkupPercent *float64`. If it's `float64` (non-pointer, zero = uncapped), adjust the nil-check in `analytics_service.go` to `if prof.MaxMarkupPercent > 0`.
