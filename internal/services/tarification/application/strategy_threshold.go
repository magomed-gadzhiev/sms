package application

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// ThresholdStrategy реализует стратегию порогов без пересчёта
type ThresholdStrategy struct{}

// NewThresholdStrategy создает стратегию с порогами
func NewThresholdStrategy() *ThresholdStrategy {
	return &ThresholdStrategy{}
}

// Calculate вычисляет стоимость сообщения по порогам
func (s *ThresholdStrategy) Calculate(_ context.Context, params CalculationParams) (*CalculationResult, error) {
	if len(params.Tiers) == 0 {
		return nil, domain.ErrNoActivePeriod
	}

	// Сортируем пороги по from_count ASC
	tiers := make([]*domain.TariffTier, len(params.Tiers))
	copy(tiers, params.Tiers)
	sort.Slice(tiers, func(i, j int) bool {
		return tiers[i].FromCount < tiers[j].FromCount
	})

	total := new(big.Float).SetFloat64(0)
	remaining := params.SegmentCount
	currentCount := params.CurrentCount
	thresholdCrossed := false
	lastPrice := tiers[0].PricePerSegment

	for remaining > 0 {
		tier := findTier(tiers, currentCount)
		lastPrice = tier.PricePerSegment

		price, _, err := big.ParseFloat(tier.PricePerSegment, 10, 128, big.ToNearestEven)
		if err != nil {
			return nil, fmt.Errorf("invalid price: %w", err)
		}

		// Определяем, сколько сегментов попадает в текущий порог
		nextTier := findNextTier(tiers, tier.FromCount)
		segmentsInTier := remaining
		if nextTier != nil && currentCount+remaining > nextTier.FromCount {
			segmentsInTier = nextTier.FromCount - currentCount
			if segmentsInTier <= 0 {
				currentCount = nextTier.FromCount
				thresholdCrossed = true
				continue
			}
			thresholdCrossed = true
		}

		segmentsBig := new(big.Float).SetFloat64(float64(segmentsInTier))
		total.Add(total, new(big.Float).Mul(price, segmentsBig))

		remaining -= segmentsInTier
		currentCount += segmentsInTier
	}

	return &CalculationResult{
		ChargeAmount:     total.Text('f', 6),
		ThresholdCrossed: thresholdCrossed,
		RecalcAmount:     "",
		PricePerSegment:  lastPrice,
	}, nil
}

// findTier находит порог для заданного количества сегментов
func findTier(tiers []*domain.TariffTier, count int) *domain.TariffTier {
	var result *domain.TariffTier
	for _, t := range tiers {
		if t.FromCount <= count {
			result = t
		} else {
			break
		}
	}
	if result == nil {
		return tiers[0]
	}
	return result
}

// findNextTier находит следующий порог после указанного from_count
func findNextTier(tiers []*domain.TariffTier, currentFromCount int) *domain.TariffTier {
	for _, t := range tiers {
		if t.FromCount > currentFromCount {
			return t
		}
	}
	return nil
}
