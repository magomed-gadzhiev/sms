package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAggregateFilters_ZeroValue(t *testing.T) {
	var f AggregateFilters
	assert.Equal(t, "", f.Period)
	assert.True(t, f.PeriodStart.IsZero())
	assert.True(t, f.PeriodEnd.IsZero())
	assert.Nil(t, f.ClientID)
	assert.Nil(t, f.ProviderID)
	assert.Nil(t, f.Status)
}

func TestAggregateFilters_WithAllFields(t *testing.T) {
	clientID := uuid.New()
	providerID := uuid.New()
	status := "delivered"

	f := AggregateFilters{
		Period:      "day",
		PeriodStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		ClientID:    &clientID,
		ProviderID:  &providerID,
		Status:      &status,
	}

	assert.Equal(t, "day", f.Period)
	assert.Equal(t, &clientID, f.ClientID)
	assert.Equal(t, &providerID, f.ProviderID)
	assert.Equal(t, &status, f.Status)
}

func TestStatisticsFilters_ZeroValue(t *testing.T) {
	var f StatisticsFilters
	assert.Nil(t, f.ClientID)
	assert.True(t, f.From.IsZero())
	assert.True(t, f.To.IsZero())
	assert.Equal(t, "", f.GroupBy)
	assert.Nil(t, f.ProviderIDs)
}

func TestStatisticsFilters_WithProviderIDs(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	f := StatisticsFilters{
		ProviderIDs: ids,
		GroupBy:     "provider",
	}

	assert.Len(t, f.ProviderIDs, 2)
	assert.Equal(t, "provider", f.GroupBy)
}

func TestTotalStats_ZeroValue(t *testing.T) {
	var ts TotalStats
	assert.Equal(t, int64(0), ts.TotalSent)
	assert.Equal(t, int64(0), ts.TotalDelivered)
	assert.Equal(t, int64(0), ts.TotalFailed)
	assert.Equal(t, int64(0), ts.TotalPending)
	assert.Equal(t, int64(0), ts.TotalQueued)
	assert.Equal(t, int64(0), ts.TotalSegments)
	assert.Equal(t, int32(0), ts.SuccessRate)
	assert.Equal(t, int64(0), ts.AvgDeliveryTimeMs)
}

func TestTotalStats_PopulatedValues(t *testing.T) {
	ts := TotalStats{
		TotalSent:         1000,
		TotalDelivered:    950,
		TotalFailed:       50,
		TotalPending:      0,
		TotalQueued:       0,
		TotalSegments:     1200,
		SuccessRate:       95,
		AvgDeliveryTimeMs: 1500,
	}

	assert.Equal(t, int64(1000), ts.TotalSent)
	assert.Equal(t, int64(950), ts.TotalDelivered)
	assert.Equal(t, int64(50), ts.TotalFailed)
	assert.Equal(t, int32(95), ts.SuccessRate)
	assert.Equal(t, int64(1500), ts.AvgDeliveryTimeMs)
}

func TestStatistics_WithGroups(t *testing.T) {
	stats := Statistics{
		Groups: []*StatisticGroup{
			{
				Key:   "2025-01-01",
				Stats: &TotalStats{TotalSent: 100, TotalDelivered: 90},
			},
			{
				Key:   "2025-01-02",
				Stats: &TotalStats{TotalSent: 200, TotalDelivered: 180},
			},
		},
		Totals: &TotalStats{TotalSent: 300, TotalDelivered: 270},
	}

	assert.Len(t, stats.Groups, 2)
	assert.Equal(t, "2025-01-01", stats.Groups[0].Key)
	assert.Equal(t, int64(100), stats.Groups[0].Stats.TotalSent)
	assert.Equal(t, int64(300), stats.Totals.TotalSent)
}

func TestStatistics_NilGroupsAndTotals(t *testing.T) {
	var stats Statistics
	assert.Nil(t, stats.Groups)
	assert.Nil(t, stats.Totals)
}

func TestProviderPerformance_ZeroValue(t *testing.T) {
	var pp ProviderPerformance
	assert.Equal(t, uuid.Nil, pp.ProviderID)
	assert.Equal(t, int64(0), pp.TotalSent)
	assert.Nil(t, pp.StatusBreakdown)
	assert.Nil(t, pp.Points)
}

func TestProviderPerformance_WithData(t *testing.T) {
	provID := uuid.New()
	pp := ProviderPerformance{
		ProviderID:        provID,
		TotalSent:         500,
		TotalDelivered:    480,
		TotalFailed:       20,
		SuccessRate:       96,
		AvgDeliveryTimeMs: 800,
		StatusBreakdown:   map[string]int64{"delivered": 480, "failed": 20},
		Points: []*PerformancePoint{
			{
				Timestamp:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				Sent:        100,
				Delivered:   96,
				Failed:      4,
				SuccessRate: 96,
			},
		},
	}

	assert.Equal(t, provID, pp.ProviderID)
	assert.Equal(t, int64(500), pp.TotalSent)
	assert.Len(t, pp.StatusBreakdown, 2)
	assert.Equal(t, int64(480), pp.StatusBreakdown["delivered"])
	assert.Len(t, pp.Points, 1)
	assert.Equal(t, int64(100), pp.Points[0].Sent)
}

func TestPerformancePoint_Fields(t *testing.T) {
	ts := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	pp := PerformancePoint{
		Timestamp:   ts,
		Sent:        200,
		Delivered:   190,
		Failed:      10,
		SuccessRate: 95,
	}

	assert.Equal(t, ts, pp.Timestamp)
	assert.Equal(t, int64(200), pp.Sent)
	assert.Equal(t, int64(190), pp.Delivered)
	assert.Equal(t, int64(10), pp.Failed)
	assert.Equal(t, int32(95), pp.SuccessRate)
}

func TestReportFilters_ZeroValue(t *testing.T) {
	var f ReportFilters
	assert.Nil(t, f.ReportType)
	assert.Nil(t, f.ClientID)
	assert.Nil(t, f.ProviderID)
	assert.Nil(t, f.From)
	assert.Nil(t, f.To)
	assert.Equal(t, 0, f.Limit)
	assert.Equal(t, 0, f.Offset)
}

func TestReportFilters_WithAllFields(t *testing.T) {
	rt := ReportTypeDaily
	clientID := uuid.New()
	providerID := uuid.New()
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	f := ReportFilters{
		ReportType: &rt,
		ClientID:   &clientID,
		ProviderID: &providerID,
		From:       &from,
		To:         &to,
		Limit:      50,
		Offset:     10,
	}

	assert.Equal(t, &rt, f.ReportType)
	assert.Equal(t, &clientID, f.ClientID)
	assert.Equal(t, 50, f.Limit)
	assert.Equal(t, 10, f.Offset)
}
