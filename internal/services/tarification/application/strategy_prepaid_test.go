package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

func TestPrepaidStrategy(t *testing.T) {
	periodID := uuid.New()

	t.Run("Calculate", func(t *testing.T) {
		t.Run("delegates_to_threshold_strategy", func(t *testing.T) {
			strategy := NewPrepaidThresholdStrategy()

			tiers := []*domain.TariffTier{
				{
					ID:              uuid.New(),
					TariffPeriodID:  periodID,
					FromCount:       0,
					PricePerSegment: "0.100000",
				},
				{
					ID:              uuid.New(),
					TariffPeriodID:  periodID,
					FromCount:       1000,
					PricePerSegment: "0.070000",
				},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 0,
				SegmentCount: 5,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			// 5 * 0.10 = 0.50 (всё в первом тире)
			assert.Equal(t, "0.500000", result.ChargeAmount)
			assert.Equal(t, "0.100000", result.PricePerSegment)
			assert.False(t, result.ThresholdCrossed)
		})

		t.Run("crossing_threshold_same_as_threshold_strategy", func(t *testing.T) {
			strategy := NewPrepaidThresholdStrategy()

			tiers := []*domain.TariffTier{
				{
					ID:              uuid.New(),
					TariffPeriodID:  periodID,
					FromCount:       0,
					PricePerSegment: "0.100000",
				},
				{
					ID:              uuid.New(),
					TariffPeriodID:  periodID,
					FromCount:       100,
					PricePerSegment: "0.060000",
				},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 98,
				SegmentCount: 4,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			// 2 сегмента в tier 1 (0.10) + 2 сегмента в tier 2 (0.06)
			// 2*0.10 + 2*0.06 = 0.20 + 0.12 = 0.32
			assert.Equal(t, "0.320000", result.ChargeAmount)
			assert.True(t, result.ThresholdCrossed)
			assert.Equal(t, "0.060000", result.PricePerSegment)
		})

		t.Run("no_tiers_returns_error", func(t *testing.T) {
			strategy := NewPrepaidThresholdStrategy()

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 0,
				SegmentCount: 1,
				Tiers:        []*domain.TariffTier{},
			})

			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrNoActivePeriod)
		})
	})
}
