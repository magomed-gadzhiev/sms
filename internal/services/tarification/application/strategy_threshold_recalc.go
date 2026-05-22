package application

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// ThresholdRecalcStrategy реализует стратегию порогов с пересчётом
// При переходе порога все предыдущие сегменты пересчитываются по новой цене
type ThresholdRecalcStrategy struct{}

// NewThresholdRecalcStrategy создает стратегию с порогами и пересчётом
func NewThresholdRecalcStrategy() *ThresholdRecalcStrategy {
	return &ThresholdRecalcStrategy{}
}

// Calculate вычисляет стоимость с учётом пересчёта при переходе порога
func (s *ThresholdRecalcStrategy) Calculate(_ context.Context, params CalculationParams) (*CalculationResult, error) {
	if len(params.Tiers) == 0 {
		return nil, domain.ErrNoActivePeriod
	}

	tiers := make([]*domain.TariffTier, len(params.Tiers))
	copy(tiers, params.Tiers)
	sort.Slice(tiers, func(i, j int) bool {
		return tiers[i].FromCount < tiers[j].FromCount
	})

	// Считаем стоимость текущего сообщения как в threshold (без пересчёта)
	thresholdStrategy := &ThresholdStrategy{}
	result, err := thresholdStrategy.Calculate(nil, params)
	if err != nil {
		return nil, err
	}

	// Определяем, произошёл ли переход порога
	oldTier := findTier(tiers, params.CurrentCount)
	newCount := params.CurrentCount + params.SegmentCount
	newTier := findTier(tiers, newCount-1) // -1 потому что newCount — следующая позиция

	if oldTier.FromCount == newTier.FromCount {
		// Порог не пересечён — пересчёта нет
		return result, nil
	}

	// Порог пересечён — пересчитываем все предыдущие сегменты
	result.ThresholdCrossed = true

	oldPrice, _, err := big.ParseFloat(oldTier.PricePerSegment, 10, 128, big.ToNearestEven)
	if err != nil {
		return nil, fmt.Errorf("invalid old price: %w", err)
	}

	newPrice, _, err := big.ParseFloat(newTier.PricePerSegment, 10, 128, big.ToNearestEven)
	if err != nil {
		return nil, fmt.Errorf("invalid new price: %w", err)
	}

	// Разница цен × количество предыдущих сегментов
	priceDiff := new(big.Float).Sub(newPrice, oldPrice)
	prevSegments := new(big.Float).SetFloat64(float64(params.CurrentCount))
	recalc := new(big.Float).Mul(priceDiff, prevSegments)

	result.RecalcAmount = recalc.Text('f', 6)
	result.PricePerSegment = newTier.PricePerSegment

	return result, nil
}
