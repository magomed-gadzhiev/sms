package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/analytics/domain"
	"github.com/smpp-server/smpp-server/internal/services/analytics/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsService(t *testing.T) {
	t.Run("RecordMetric", func(t *testing.T) {
		t.Run("RecordMessageCreated", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAnalyticsService(metricRepo)

			ctx := context.Background()
			messageID := uuid.New().String()
			clientID := uuid.New().String()
			providerID := uuid.New().String()
			var timestamp int64 = 1700000000

			metricRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Metric) bool {
				return m.Type == domain.MetricTypeMessageCreated &&
					m.Status == "created" &&
					m.Value == 1
			})).Return(nil)

			err := svc.RecordMessageCreated(ctx, messageID, &clientID, &providerID, timestamp)

			require.NoError(t, err)
			metricRepo.AssertExpectations(t)
		})

		t.Run("RecordMessageSent", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAnalyticsService(metricRepo)

			ctx := context.Background()
			messageID := uuid.New().String()
			clientID := uuid.New().String()
			providerID := uuid.New().String()
			var timestamp int64 = 1700000000

			metricRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Metric) bool {
				return m.Type == domain.MetricTypeMessageSent &&
					m.Status == "sent" &&
					m.Value == 1
			})).Return(nil)

			err := svc.RecordMessageSent(ctx, messageID, &clientID, &providerID, timestamp)

			require.NoError(t, err)
			metricRepo.AssertExpectations(t)
		})

		t.Run("RecordMessageDelivered", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAnalyticsService(metricRepo)

			ctx := context.Background()
			messageID := uuid.New().String()
			clientID := uuid.New().String()
			providerID := uuid.New().String()
			var timestamp int64 = 1700000000

			metricRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Metric) bool {
				return m.Type == domain.MetricTypeMessageDelivered &&
					m.Status == "delivered" &&
					m.Value == 1
			})).Return(nil)

			err := svc.RecordMessageDelivered(ctx, messageID, &clientID, &providerID, timestamp)

			require.NoError(t, err)
			metricRepo.AssertExpectations(t)
		})

		t.Run("RecordMessageFailed_with_reason", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAnalyticsService(metricRepo)

			ctx := context.Background()
			messageID := uuid.New().String()
			clientID := uuid.New().String()
			providerID := uuid.New().String()
			var timestamp int64 = 1700000000

			metricRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Metric) bool {
				return m.Type == domain.MetricTypeMessageFailed &&
					m.Status == "failed" &&
					m.Value == 1 &&
					m.Metadata["reason"] == "timeout"
			})).Return(nil)

			err := svc.RecordMessageFailed(ctx, messageID, &clientID, &providerID, timestamp, "timeout")

			require.NoError(t, err)
			metricRepo.AssertExpectations(t)
		})

		t.Run("RecordMessageCreated_nil_optional_fields", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAnalyticsService(metricRepo)

			ctx := context.Background()
			messageID := uuid.New().String()
			var timestamp int64 = 1700000000

			metricRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Metric) bool {
				return m.Type == domain.MetricTypeMessageCreated &&
					m.ClientID == nil &&
					m.ProviderID == nil
			})).Return(nil)

			err := svc.RecordMessageCreated(ctx, messageID, nil, nil, timestamp)

			require.NoError(t, err)
			metricRepo.AssertExpectations(t)
		})
	})

	t.Run("GetStatistics", func(t *testing.T) {
		t.Run("returns_stats", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAnalyticsService(metricRepo)

			ctx := context.Background()
			filters := &domain.StatisticsFilters{
				GroupBy: "day",
			}

			expectedStats := &domain.Statistics{
				Totals: &domain.TotalStats{
					TotalSent:      100,
					TotalDelivered: 90,
					TotalFailed:    10,
					SuccessRate:    90,
				},
				Groups: []*domain.StatisticGroup{
					{
						Key: "2024-01-01",
						Stats: &domain.TotalStats{
							TotalSent:      50,
							TotalDelivered: 45,
							TotalFailed:    5,
						},
					},
				},
			}

			metricRepo.On("GetStatistics", ctx, filters).Return(expectedStats, nil)

			stats, err := svc.GetStatistics(ctx, filters)

			require.NoError(t, err)
			assert.Equal(t, expectedStats, stats)
			assert.Equal(t, int64(100), stats.Totals.TotalSent)
			assert.Equal(t, int64(90), stats.Totals.TotalDelivered)
			assert.Equal(t, int64(10), stats.Totals.TotalFailed)
			assert.Equal(t, int32(90), stats.Totals.SuccessRate)
			assert.Len(t, stats.Groups, 1)
			metricRepo.AssertExpectations(t)
		})

		t.Run("returns_error", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAnalyticsService(metricRepo)

			ctx := context.Background()
			filters := &domain.StatisticsFilters{}

			metricRepo.On("GetStatistics", ctx, filters).Return(nil, assert.AnError)

			stats, err := svc.GetStatistics(ctx, filters)

			require.Error(t, err)
			assert.Nil(t, stats)
			metricRepo.AssertExpectations(t)
		})
	})

	t.Run("GetProviderPerformance", func(t *testing.T) {
		t.Run("returns_performance_data", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAnalyticsService(metricRepo)

			ctx := context.Background()
			providerID := uuid.New()
			from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

			expectedPerf := &domain.ProviderPerformance{
				ProviderID:        providerID,
				TotalSent:         500,
				TotalDelivered:    450,
				TotalFailed:       50,
				SuccessRate:       90,
				AvgDeliveryTimeMs: 120,
				StatusBreakdown: map[string]int64{
					"delivered": 450,
					"failed":   50,
				},
				Points: []*domain.PerformancePoint{
					{
						Timestamp:   from,
						Sent:        250,
						Delivered:   225,
						Failed:      25,
						SuccessRate: 90,
					},
				},
			}

			metricRepo.On("GetProviderPerformance", ctx, providerID, from, to).Return(expectedPerf, nil)

			perf, err := svc.GetProviderPerformance(ctx, providerID, from, to)

			require.NoError(t, err)
			assert.Equal(t, expectedPerf, perf)
			assert.Equal(t, providerID, perf.ProviderID)
			assert.Equal(t, int64(500), perf.TotalSent)
			assert.Equal(t, int64(450), perf.TotalDelivered)
			assert.Equal(t, int32(90), perf.SuccessRate)
			assert.Len(t, perf.Points, 1)
			metricRepo.AssertExpectations(t)
		})

		t.Run("repository_error", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAnalyticsService(metricRepo)

			ctx := context.Background()
			providerID := uuid.New()
			from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

			metricRepo.On("GetProviderPerformance", ctx, providerID, from, to).Return(nil, assert.AnError)

			perf, err := svc.GetProviderPerformance(ctx, providerID, from, to)

			require.Error(t, err)
			assert.Nil(t, perf)
			metricRepo.AssertExpectations(t)
		})
	})

	t.Run("RecordMessageFailed", func(t *testing.T) {
		t.Run("with_empty_reason", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAnalyticsService(metricRepo)

			ctx := context.Background()
			messageID := uuid.New().String()
			clientID := uuid.New().String()
			providerID := uuid.New().String()
			var timestamp int64 = 1700000000

			metricRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Metric) bool {
				return m.Type == domain.MetricTypeMessageFailed &&
					m.Status == "failed" &&
					m.Value == 1 &&
					len(m.Metadata) == 0
			})).Return(nil)

			err := svc.RecordMessageFailed(ctx, messageID, &clientID, &providerID, timestamp, "")

			require.NoError(t, err)
			metricRepo.AssertExpectations(t)
		})
	})
}
