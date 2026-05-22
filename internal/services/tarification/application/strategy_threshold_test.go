package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

func TestThresholdStrategy(t *testing.T) {
	periodID := uuid.New()

	// Стандартные пороги: 0-999 = 0.10, 1000-4999 = 0.08, 5000+ = 0.05
	makeTiers := func() []*domain.TariffTier {
		return []*domain.TariffTier{
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
				PricePerSegment: "0.080000",
			},
			{
				ID:              uuid.New(),
				TariffPeriodID:  periodID,
				FromCount:       5000,
				PricePerSegment: "0.050000",
			},
		}
	}

	t.Run("Calculate", func(t *testing.T) {
		t.Run("below_threshold", func(t *testing.T) {
			strategy := NewThresholdStrategy()

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 0,
				SegmentCount: 5,
				Tiers:        makeTiers(),
			})

			require.NoError(t, err)
			// 5 сегментов * 0.10 = 0.50
			assert.Equal(t, "0.500000", result.ChargeAmount)
			assert.Equal(t, "0.100000", result.PricePerSegment)
			assert.False(t, result.ThresholdCrossed)
			assert.Empty(t, result.RecalcAmount)
		})

		t.Run("above_threshold", func(t *testing.T) {
			strategy := NewThresholdStrategy()

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 1500,
				SegmentCount: 3,
				Tiers:        makeTiers(),
			})

			require.NoError(t, err)
			// Все 3 сегмента в tier 2 (1000+), цена 0.08
			// 3 * 0.08 = 0.24
			assert.Equal(t, "0.240000", result.ChargeAmount)
			assert.Equal(t, "0.080000", result.PricePerSegment)
			assert.False(t, result.ThresholdCrossed)
		})

		t.Run("crossing_threshold", func(t *testing.T) {
			strategy := NewThresholdStrategy()

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 998,
				SegmentCount: 4,
				Tiers:        makeTiers(),
			})

			require.NoError(t, err)
			// 2 сегмента в tier 1 (0.10) + 2 сегмента в tier 2 (0.08)
			// 2*0.10 + 2*0.08 = 0.20 + 0.16 = 0.36
			assert.Equal(t, "0.360000", result.ChargeAmount)
			assert.True(t, result.ThresholdCrossed)
			// lastPrice -- цена последнего использованного тира
			assert.Equal(t, "0.080000", result.PricePerSegment)
		})

		t.Run("crossing_two_thresholds", func(t *testing.T) {
			strategy := NewThresholdStrategy()

			// Маленькие пороги для теста перехода через два
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
					FromCount:       3,
					PricePerSegment: "0.080000",
				},
				{
					ID:              uuid.New(),
					TariffPeriodID:  periodID,
					FromCount:       6,
					PricePerSegment: "0.050000",
				},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 2,
				SegmentCount: 6,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			// 1 сегмент в tier 1 (0.10) [count 2-3)
			// 3 сегмента в tier 2 (0.08) [count 3-6)
			// 2 сегмента в tier 3 (0.05) [count 6-8)
			// = 0.10 + 0.24 + 0.10 = 0.44
			assert.Equal(t, "0.440000", result.ChargeAmount)
			assert.True(t, result.ThresholdCrossed)
			assert.Equal(t, "0.050000", result.PricePerSegment)
		})

		t.Run("in_highest_tier", func(t *testing.T) {
			strategy := NewThresholdStrategy()

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 10000,
				SegmentCount: 5,
				Tiers:        makeTiers(),
			})

			require.NoError(t, err)
			// 5 * 0.05 = 0.25
			assert.Equal(t, "0.250000", result.ChargeAmount)
			assert.Equal(t, "0.050000", result.PricePerSegment)
			assert.False(t, result.ThresholdCrossed)
		})

		t.Run("no_tiers_returns_error", func(t *testing.T) {
			strategy := NewThresholdStrategy()

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 0,
				SegmentCount: 1,
				Tiers:        []*domain.TariffTier{},
			})

			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrNoActivePeriod)
		})

		t.Run("unsorted_tiers_are_sorted", func(t *testing.T) {
			strategy := NewThresholdStrategy()

			// Передаём пороги в обратном порядке
			tiers := []*domain.TariffTier{
				{
					ID:              uuid.New(),
					TariffPeriodID:  periodID,
					FromCount:       1000,
					PricePerSegment: "0.080000",
				},
				{
					ID:              uuid.New(),
					TariffPeriodID:  periodID,
					FromCount:       0,
					PricePerSegment: "0.100000",
				},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 0,
				SegmentCount: 2,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			// 2 * 0.10 = 0.20
			assert.Equal(t, "0.200000", result.ChargeAmount)
		})
	})
}
