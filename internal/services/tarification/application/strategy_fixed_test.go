package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

func TestFixedStrategy(t *testing.T) {
	t.Run("Calculate", func(t *testing.T) {
		t.Run("single_tier", func(t *testing.T) {
			strategy := NewFixedStrategy()
			periodID := uuid.New()

			tiers := []*domain.TariffTier{
				{
					ID:              uuid.New(),
					TariffPeriodID:  periodID,
					FromCount:       0,
					PricePerSegment: "0.050000",
				},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 0,
				SegmentCount: 3,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			assert.Equal(t, "0.150000", result.ChargeAmount)
			assert.Equal(t, "0.050000", result.PricePerSegment)
			assert.False(t, result.ThresholdCrossed)
			assert.Empty(t, result.RecalcAmount)
		})

		t.Run("single_segment", func(t *testing.T) {
			strategy := NewFixedStrategy()
			periodID := uuid.New()

			tiers := []*domain.TariffTier{
				{
					ID:              uuid.New(),
					TariffPeriodID:  periodID,
					FromCount:       0,
					PricePerSegment: "1.200000",
				},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 100,
				SegmentCount: 1,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			assert.Equal(t, "1.200000", result.ChargeAmount)
			assert.Equal(t, "1.200000", result.PricePerSegment)
			assert.False(t, result.ThresholdCrossed)
		})

		t.Run("no_tiers_returns_error", func(t *testing.T) {
			strategy := NewFixedStrategy()

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 0,
				SegmentCount: 1,
				Tiers:        []*domain.TariffTier{},
			})

			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrNoActivePeriod)
		})

		t.Run("uses_first_tier_only", func(t *testing.T) {
			strategy := NewFixedStrategy()
			periodID := uuid.New()

			// Даже если передано несколько тиров, fixed стратегия использует первый
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
					PricePerSegment: "0.050000",
				},
			}

			result, err := strategy.Calculate(context.Background(), CalculationParams{
				CurrentCount: 500,
				SegmentCount: 2,
				Tiers:        tiers,
			})

			require.NoError(t, err)
			// 2 * 0.1 = 0.2 (использует только первый тир)
			assert.Equal(t, "0.200000", result.ChargeAmount)
			assert.Equal(t, "0.100000", result.PricePerSegment)
		})
	})
}
