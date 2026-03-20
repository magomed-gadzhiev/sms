package application

import (
	"context"
	"fmt"
	"math/big"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// FixedStrategy реализует стратегию фиксированной цены за сегмент
type FixedStrategy struct{}

// NewFixedStrategy создает стратегию фиксированной цены
func NewFixedStrategy() *FixedStrategy {
	return &FixedStrategy{}
}

// Calculate вычисляет стоимость сообщения по фиксированной цене
func (s *FixedStrategy) Calculate(_ context.Context, params CalculationParams) (*CalculationResult, error) {
	if len(params.Tiers) == 0 {
		return nil, domain.ErrNoActivePeriod
	}

	// Фиксированная цена — один порог с from_count = 0
	tier := params.Tiers[0]
	price, _, err := big.ParseFloat(tier.PricePerSegment, 10, 128, big.ToNearestEven)
	if err != nil {
		return nil, fmt.Errorf("invalid price: %w", err)
	}

	segments := big.NewFloat(float64(params.SegmentCount))
	total := new(big.Float).Mul(price, segments)

	return &CalculationResult{
		ChargeAmount:    total.Text('f', 6),
		ThresholdCrossed: false,
		RecalcAmount:    "",
		PricePerSegment: tier.PricePerSegment,
	}, nil
}
