package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/analytics/domain"
	"github.com/smpp-server/smpp-server/internal/services/analytics/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregationService(t *testing.T) {
	t.Run("Aggregate", func(t *testing.T) {
		t.Run("UpdateAggregatedMetric_creates_aggregated_metric", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAggregationService(metricRepo)

			ctx := context.Background()
			clientID := uuid.New()
			providerID := uuid.New()
			now := time.Now().Truncate(time.Hour)

			metric := domain.NewAggregatedMetric(
				"hour",
				now,
				now.Add(time.Hour),
				&clientID,
				&providerID,
				"sent",
				42,
			)

			metricRepo.On("CreateAggregated", ctx, metric).Return(nil)

			err := svc.UpdateAggregatedMetric(ctx, metric)

			require.NoError(t, err)
			metricRepo.AssertExpectations(t)
		})

		t.Run("UpdateAggregatedMetric_returns_error", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAggregationService(metricRepo)

			ctx := context.Background()
			now := time.Now().Truncate(time.Hour)

			metric := domain.NewAggregatedMetric(
				"hour",
				now,
				now.Add(time.Hour),
				nil,
				nil,
				"sent",
				10,
			)

			metricRepo.On("CreateAggregated", ctx, metric).Return(assert.AnError)

			err := svc.UpdateAggregatedMetric(ctx, metric)

			require.Error(t, err)
			assert.Equal(t, assert.AnError, err)
			metricRepo.AssertExpectations(t)
		})

		t.Run("GetAggregatedMetrics_returns_metrics", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAggregationService(metricRepo)

			ctx := context.Background()
			clientID := uuid.New()
			now := time.Now().Truncate(time.Hour)

			filters := &domain.AggregateFilters{
				Period:      "hour",
				PeriodStart: now.Add(-24 * time.Hour),
				PeriodEnd:   now,
				ClientID:    &clientID,
			}

			expectedMetrics := []*domain.AggregatedMetric{
				domain.NewAggregatedMetric("hour", now.Add(-2*time.Hour), now.Add(-time.Hour), &clientID, nil, "sent", 100),
				domain.NewAggregatedMetric("hour", now.Add(-time.Hour), now, &clientID, nil, "sent", 150),
			}

			metricRepo.On("GetAggregated", ctx, filters).Return(expectedMetrics, nil)

			metrics, err := svc.GetAggregatedMetrics(ctx, filters)

			require.NoError(t, err)
			assert.Len(t, metrics, 2)
			assert.Equal(t, int64(100), metrics[0].Count)
			assert.Equal(t, int64(150), metrics[1].Count)
			metricRepo.AssertExpectations(t)
		})

		t.Run("GetAggregatedMetrics_returns_aggregated_data", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAggregationService(metricRepo)

			ctx := context.Background()
			clientID := uuid.New()
			providerID := uuid.New()
			now := time.Now().Truncate(time.Hour)

			filters := &domain.AggregateFilters{
				Period:      "day",
				PeriodStart: now.Add(-7 * 24 * time.Hour),
				PeriodEnd:   now,
				ClientID:    &clientID,
				ProviderID:  &providerID,
			}

			expectedMetrics := []*domain.AggregatedMetric{
				domain.NewAggregatedMetric("day", now.Add(-3*24*time.Hour), now.Add(-2*24*time.Hour), &clientID, &providerID, "sent", 500),
				domain.NewAggregatedMetric("day", now.Add(-2*24*time.Hour), now.Add(-24*time.Hour), &clientID, &providerID, "sent", 600),
				domain.NewAggregatedMetric("day", now.Add(-24*time.Hour), now, &clientID, &providerID, "sent", 700),
			}

			metricRepo.On("GetAggregated", ctx, filters).Return(expectedMetrics, nil)

			metrics, err := svc.GetAggregatedMetrics(ctx, filters)

			require.NoError(t, err)
			assert.Len(t, metrics, 3)
			assert.Equal(t, int64(500), metrics[0].Count)
			assert.Equal(t, int64(600), metrics[1].Count)
			assert.Equal(t, int64(700), metrics[2].Count)
			assert.Equal(t, "day", metrics[0].Period)
			metricRepo.AssertExpectations(t)
		})

		t.Run("GetAggregatedMetrics_repository_error", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAggregationService(metricRepo)

			ctx := context.Background()
			now := time.Now().Truncate(time.Hour)

			filters := &domain.AggregateFilters{
				Period:      "hour",
				PeriodStart: now.Add(-24 * time.Hour),
				PeriodEnd:   now,
			}

			metricRepo.On("GetAggregated", ctx, filters).Return(nil, assert.AnError)

			metrics, err := svc.GetAggregatedMetrics(ctx, filters)

			require.Error(t, err)
			assert.Nil(t, metrics)
			assert.Equal(t, assert.AnError, err)
			metricRepo.AssertExpectations(t)
		})

		t.Run("AggregateMetrics_runs_without_error", func(t *testing.T) {
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewAggregationService(metricRepo)

			ctx := context.Background()
			now := time.Now()

			err := svc.AggregateMetrics(ctx, "hour", now.Add(-time.Hour), now)

			require.NoError(t, err)
		})
	})
}
