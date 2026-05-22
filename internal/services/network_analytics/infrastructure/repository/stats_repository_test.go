package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network_analytics/domain"
)

// findKPI returns the KPI with the given name, or nil if absent.
func findKPI(kpis []domain.KPI, name string) *domain.KPI {
	for i := range kpis {
		if kpis[i].Name == name {
			return &kpis[i]
		}
	}
	return nil
}

// TestComputeKPIs_NoMoneyDataRendersAsNil mirrors the application-layer test
// for buildStatKPIs: when revenue and cost are both zero, Прибыль's Value is
// nil so the UI renders "—" rather than a misleading "0 ₽".
func TestComputeKPIs_NoMoneyDataRendersAsNil(t *testing.T) {
	kpis := computeKPIs(100, 80, 10, 0, 0)

	profit := findKPI(kpis, "Прибыль")
	if profit == nil {
		t.Fatalf("Прибыль: KPI missing")
	}
	if profit.Value != nil {
		t.Errorf("Прибыль: expected Value == nil when no money data, got %v", *profit.Value)
	}
	if profit.Format != "currency" {
		t.Errorf("Прибыль: expected Format=\"currency\", got %q", profit.Format)
	}
	if profit.Currency != "RUB" {
		t.Errorf("Прибыль: expected Currency=\"RUB\", got %q", profit.Currency)
	}

	dlr := findKPI(kpis, "Доставляемость")
	if dlr == nil || dlr.Value == nil {
		t.Fatalf("Доставляемость: expected non-nil Value")
	}
	if dlr.Format != "percent" {
		t.Errorf("Доставляемость: expected Format=\"percent\", got %q", dlr.Format)
	}

	total := findKPI(kpis, "Всего")
	if total == nil || total.Value == nil {
		t.Fatalf("Всего: expected non-nil Value")
	}
	if total.Format != "count" {
		t.Errorf("Всего: expected Format=\"count\", got %q", total.Format)
	}
	if *total.Value != 100 {
		t.Errorf("Всего: expected Value=100, got %v", *total.Value)
	}
}

// TestComputeKPIs_BreakevenRendersAsZero is the regression guard against the
// previous moneyValue closure that suppressed any zero — including a genuine
// breakeven (revenue == cost > 0, profit == 0) — and incorrectly rendered it
// as "—". Profit must point to 0.0 here, not nil.
func TestComputeKPIs_BreakevenRendersAsZero(t *testing.T) {
	kpis := computeKPIs(100, 80, 10, 1500.0, 1500.0)

	profit := findKPI(kpis, "Прибыль")
	if profit == nil {
		t.Fatalf("Прибыль: KPI missing")
	}
	if profit.Value == nil {
		t.Fatalf("Прибыль: expected non-nil Value at breakeven (revenue == cost > 0), got nil")
	}
	if *profit.Value != 0.0 {
		t.Errorf("Прибыль: expected 0.0 at breakeven, got %v", *profit.Value)
	}
	if profit.Format != "currency" {
		t.Errorf("Прибыль: expected Format=\"currency\", got %q", profit.Format)
	}
	if profit.Currency != "RUB" {
		t.Errorf("Прибыль: expected Currency=\"RUB\", got %q", profit.Currency)
	}
}

// TestStatsRepo_UpsertHourlyStats_DoubleRunDoesNotDouble guards the replace
// semantics of UpsertHourlyStats. The aggregator (and the backfill CLI) must
// be idempotent: re-running the same hour with the same source data should
// yield the same row, not double the counters. Previously the SQL did
// `total = network_stats_hourly.total + EXCLUDED.total` which broke
// idempotency. Now all counter columns use `EXCLUDED.<col>` directly.
//
// throughput_max keeps GREATEST(...) because it represents an in-hour peak
// observed by the aggregator's scan window — replacing it would lose history
// when multiple runs catch different peaks. This test does not exercise that
// branch; it only guards the additive→replace flip on counters and money.
func TestStatsRepo_UpsertHourlyStats_DoubleRunDoesNotDouble(t *testing.T) {
	pool := setupViewsTestDB(t)
	repo := NewStatsRepo(pool)

	hour := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	const partnerID int64 = 1_000_000_000

	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(),
			`DELETE FROM network_stats_hourly WHERE partner_id=$1 AND hour=$2`,
			partnerID, hour)
		require.NoError(t, err, "cleanup network_stats_hourly")
	})

	row := domain.HourlyStatsRow{
		PartnerID:     partnerID,
		Hour:          hour,
		ProviderID:    0,
		Operator:      "TEST_OP",
		Country:       "RU",
		Channel:       "sms",
		Login:         "test_login",
		SenderName:    "TestSender",
		TrafficType:   "transactional",
		Method:        "submit_sm",
		Total:         100,
		Sent:          100,
		Delivered:     80,
		Failed:        10,
		Pending:       5,
		Timeout:       5,
		Error:         10,
		Revenue:       150.0,
		Cost:          60.0,
		DLRLatencySum: 8000,
		DLRLatencyCnt: 80,
		DLRLatencyP50: 100,
		DLRLatencyP95: 200,
		ThroughputMax: 12.5,
	}

	err := repo.UpsertHourlyStats(context.Background(), []domain.HourlyStatsRow{row})
	require.NoError(t, err, "first upsert")

	err = repo.UpsertHourlyStats(context.Background(), []domain.HourlyStatsRow{row})
	require.NoError(t, err, "second upsert (same row) must be idempotent")

	var (
		total, sent, delivered, failed, pending, timeoutCnt, errCnt int
		revenue, cost                                               float64
		dlrLatencySum                                               int64
		dlrLatencyCnt                                               int
	)
	err = pool.QueryRow(context.Background(), `
		SELECT total, sent, delivered, failed, pending, timeout, error,
		       revenue, cost, dlr_latency_sum, dlr_latency_cnt
		FROM network_stats_hourly
		WHERE partner_id=$1 AND hour=$2`,
		partnerID, hour,
	).Scan(&total, &sent, &delivered, &failed, &pending, &timeoutCnt, &errCnt,
		&revenue, &cost, &dlrLatencySum, &dlrLatencyCnt)
	require.NoError(t, err, "select aggregated row")

	require.Equal(t, 100, total, "total must stay at 100 (replace semantics), not double to 200")
	require.Equal(t, 100, sent, "sent must stay at 100")
	require.Equal(t, 80, delivered, "delivered must stay at 80")
	require.Equal(t, 10, failed, "failed must stay at 10")
	require.Equal(t, 5, pending, "pending must stay at 5")
	require.Equal(t, 5, timeoutCnt, "timeout must stay at 5")
	require.Equal(t, 10, errCnt, "error must stay at 10")
	require.InDelta(t, 150.0, revenue, 0.0001, "revenue must stay at 150.0 (replace), not double to 300.0")
	require.InDelta(t, 60.0, cost, 0.0001, "cost must stay at 60.0")
	require.Equal(t, int64(8000), dlrLatencySum, "dlr_latency_sum must stay at 8000")
	require.Equal(t, 80, dlrLatencyCnt, "dlr_latency_cnt must stay at 80")
}
