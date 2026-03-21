package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

func TestThresholdRecalcStrategy(t *testing.T) {
	periodID := uuid.New()

	t.Run("Calculate", func(t *testing.T) {
		t.Run("no_threshold_crossed_same_as_threshold", func(t *testing.T) {
			strategy := NewThresholdRecalcStrategy()

			tiers := []*domain.TariffTier{
				{ID: uuid.New(), TariffPeriodID: periodID, FromCount: 0, PricePerSegment: "0.100000"},
				{ID: uuid.New(), TariffPeriodID: periodID, FromCount: 1000, PricePerSegment: "0.070000"},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 50,
				SegmentCount: 3,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			// 3 * 0.10 = 0.30, no threshold crossed
			assert.Equal(t, "0.300000", result.ChargeAmount)
			assert.Equal(t, "0.100000", result.PricePerSegment)
			assert.False(t, result.ThresholdCrossed)
			assert.Empty(t, result.RecalcAmount)
		})

		t.Run("threshold_crossed_recalc_positive", func(t *testing.T) {
			strategy := NewThresholdRecalcStrategy()

			tiers := []*domain.TariffTier{
				{ID: uuid.New(), TariffPeriodID: periodID, FromCount: 0, PricePerSegment: "0.100000"},
				{ID: uuid.New(), TariffPeriodID: periodID, FromCount: 100, PricePerSegment: "0.150000"},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 98,
				SegmentCount: 4,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			assert.True(t, result.ThresholdCrossed)
			// The charge is calculated by threshold strategy: 2*0.10 + 2*0.15 = 0.50
			// Recalc: (0.15 - 0.10) * 98 = 0.05 * 98 = 4.90
			assert.Equal(t, "4.900000", result.RecalcAmount)
			assert.Equal(t, "0.150000", result.PricePerSegment)
		})

		t.Run("threshold_crossed_recalc_negative", func(t *testing.T) {
			strategy := NewThresholdRecalcStrategy()

			// New tier has LOWER price (unusual but valid)
			tiers := []*domain.TariffTier{
				{ID: uuid.New(), TariffPeriodID: periodID, FromCount: 0, PricePerSegment: "0.100000"},
				{ID: uuid.New(), TariffPeriodID: periodID, FromCount: 10, PricePerSegment: "0.060000"},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 8,
				SegmentCount: 4,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			assert.True(t, result.ThresholdCrossed)
			// Recalc: (0.06 - 0.10) * 8 = -0.04 * 8 = -0.32
			assert.Equal(t, "-0.320000", result.RecalcAmount)
			assert.Equal(t, "0.060000", result.PricePerSegment)
		})

		t.Run("no_tiers_returns_error", func(t *testing.T) {
			strategy := NewThresholdRecalcStrategy()

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 0,
				SegmentCount: 1,
				Tiers:        []*domain.TariffTier{},
			})

			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrNoActivePeriod)
		})

		t.Run("already_in_higher_tier_no_crossing", func(t *testing.T) {
			strategy := NewThresholdRecalcStrategy()

			tiers := []*domain.TariffTier{
				{ID: uuid.New(), TariffPeriodID: periodID, FromCount: 0, PricePerSegment: "0.100000"},
				{ID: uuid.New(), TariffPeriodID: periodID, FromCount: 1000, PricePerSegment: "0.080000"},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 5000,
				SegmentCount: 5,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			// 5 * 0.08 = 0.40
			assert.Equal(t, "0.400000", result.ChargeAmount)
			assert.False(t, result.ThresholdCrossed)
			assert.Empty(t, result.RecalcAmount)
		})

		t.Run("unsorted_tiers_are_sorted", func(t *testing.T) {
			strategy := NewThresholdRecalcStrategy()

			// Tiers in reverse order
			tiers := []*domain.TariffTier{
				{ID: uuid.New(), TariffPeriodID: periodID, FromCount: 100, PricePerSegment: "0.080000"},
				{ID: uuid.New(), TariffPeriodID: periodID, FromCount: 0, PricePerSegment: "0.100000"},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 0,
				SegmentCount: 2,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			assert.Equal(t, "0.200000", result.ChargeAmount)
			assert.False(t, result.ThresholdCrossed)
		})

		t.Run("single_tier", func(t *testing.T) {
			strategy := NewThresholdRecalcStrategy()

			tiers := []*domain.TariffTier{
				{ID: uuid.New(), TariffPeriodID: periodID, FromCount: 0, PricePerSegment: "0.050000"},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 500,
				SegmentCount: 10,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			// 10 * 0.05 = 0.50
			assert.Equal(t, "0.500000", result.ChargeAmount)
			assert.False(t, result.ThresholdCrossed)
			assert.Empty(t, result.RecalcAmount)
		})
	})
}
