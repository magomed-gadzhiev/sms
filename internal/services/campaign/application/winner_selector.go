package application

import (
	"math"

	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
)

const minSamplePerVariant = 100
const tieThreshold = 0.01 // 1%

// selectBestVariant picks the best variant by the given metric.
func selectBestVariant(variants []domain.Variant, metric string) domain.Variant {
	if len(variants) == 0 {
		return domain.Variant{}
	}

	best := variants[0]
	bestScore := variantScore(best, metric)

	for _, v := range variants[1:] {
		score := variantScore(v, metric)
		diff := math.Abs(score - bestScore)

		if diff < tieThreshold {
			// Tie — pick larger sample
			if v.SentCount > best.SentCount {
				best = v
				bestScore = score
			}
		} else if score > bestScore {
			best = v
			bestScore = score
		}
	}
	return best
}

func variantScore(v domain.Variant, metric string) float64 {
	if v.SentCount == 0 {
		return 0
	}
	switch metric {
	case "delivery_rate":
		return float64(v.DeliveredCount) / float64(v.SentCount)
	case "click_rate":
		return float64(v.ClickCount) / float64(v.SentCount)
	case "unique_click_rate":
		return float64(v.UniqueClickCount) / float64(v.SentCount)
	default:
		return float64(v.DeliveredCount) / float64(v.SentCount)
	}
}

// hasMinSampleSize checks if all variants have enough sent messages.
func hasMinSampleSize(variants []domain.Variant) bool {
	for _, v := range variants {
		if v.SentCount < minSamplePerVariant {
			return false
		}
	}
	return true
}
