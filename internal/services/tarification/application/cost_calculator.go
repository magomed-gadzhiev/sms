package application

import (
	"fmt"
	"strconv"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// CostCalculator считает стоимость batch-а сегментов по разрешённому правилу.
// Использует TiersCache для snimания стоимости парсинга JSONB.
type CostCalculator struct {
	cache *TiersCache
}

func NewCostCalculator(cache *TiersCache) *CostCalculator {
	return &CostCalculator{cache: cache}
}

// Calculate возвращает стоимость batch-а сегментов. usageBefore — segments_used
// ДО текущего сообщения (из subaccount_usage_counters). Нужен для корректной
// работы tiered-прогрессивной модели (когда сообщение пересекает границу тира)
// и prepaid_threshold (когда overage начинается в середине батча).
func (c *CostCalculator) Calculate(rr *domain.ResolvedRule, usageBefore, segments int64) (float64, error) {
	switch rr.PriceModel {
	case domain.ModelFixed:
		if rr.PriceValue == nil {
			return 0, fmt.Errorf("fixed rule %s has nil price_value", rr.SourceRuleID)
		}
		v, err := strconv.ParseFloat(*rr.PriceValue, 64)
		if err != nil {
			return 0, fmt.Errorf("parse price_value: %w", err)
		}
		return v * float64(segments), nil

	case domain.ModelTiered:
		spec, err := c.cache.GetOrParseTiered(rr.SourceRuleID, rr.RulesVersion, rr.TiersJSON)
		if err != nil {
			return 0, err
		}
		return walkTiersProgressive(spec.Tiers, usageBefore, segments), nil

	case domain.ModelPrepaidThreshold:
		spec, err := c.cache.GetOrParsePrepaid(rr.SourceRuleID, rr.RulesVersion, rr.TiersJSON)
		if err != nil {
			return 0, err
		}
		return prepaidOverage(spec, usageBefore, segments), nil

	default:
		return 0, fmt.Errorf("unknown price_model: %s", rr.PriceModel)
	}
}

// walkTiersProgressive — прогрессивная модель: каждый сегмент оплачивается по
// тиру, в котором находится. Тиры отсортированы по up_to ASC; последний с
// up_to=nil — бесконечный.
//
// Пример: usageBefore=9950, segments=100, тиры [(10000→1.0), (nil→0.5)]:
// первые 50 по 1.0 + следующие 50 по 0.5 = 75.
func walkTiersProgressive(tiers []TieredTier, usageBefore, segments int64) float64 {
	total := 0.0
	remaining := segments
	cursor := usageBefore
	for _, tier := range tiers {
		if remaining == 0 {
			break
		}
		var capacity int64
		if tier.UpTo == nil {
			capacity = remaining // бесконечный тир — поглощает всё остальное
		} else {
			capacity = *tier.UpTo - cursor
			if capacity < 0 {
				capacity = 0
			}
		}
		take := remaining
		if take > capacity {
			take = capacity
		}
		if take > 0 {
			total += tier.Price * float64(take)
			remaining -= take
			cursor += take
		}
	}
	return total
}

// prepaidOverage считает overage: платим только за сегменты СВЕРХ included_segments.
// Абонплата (PrepaidAmount) списывается отдельным billing_scheduler в начале периода,
// не на горячем пути.
//
// Пример: included=5000, usageBefore=4900, segments=200, overage=0.5:
// 100 бесплатных + 100 × 0.5 = 50.
func prepaidOverage(spec *PrepaidThresholdSpec, usageBefore, segments int64) float64 {
	newUsage := usageBefore + segments
	if newUsage <= spec.IncludedSegments {
		return 0
	}
	var overageSegments int64
	if usageBefore >= spec.IncludedSegments {
		// Уже полностью за лимитом — весь батч платный
		overageSegments = segments
	} else {
		// Пересекаем границу — платная только часть
		overageSegments = newUsage - spec.IncludedSegments
	}
	return spec.OveragePrice * float64(overageSegments)
}
