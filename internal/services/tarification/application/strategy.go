package application

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// BillingStrategy defines the interface for tarification strategies
type BillingStrategy interface {
	// Calculate computes the charge amount and returns (chargeAmount, thresholdCrossed, recalcAmount)
	Calculate(ctx context.Context, params CalculationParams) (*CalculationResult, error)
}

// CalculationParams contains inputs for strategy calculation
type CalculationParams struct {
	CurrentCount int                  // current segment count before this message
	SegmentCount int                  // segments in the current message
	Tiers        []*domain.TariffTier // sorted by FromCount ASC
}

// CalculationResult contains the result of a strategy calculation
type CalculationResult struct {
	ChargeAmount     string // total amount to charge for this message
	ThresholdCrossed bool   // whether a threshold was crossed
	RecalcAmount     string // recalculation amount (positive=charge, negative=refund), empty if none
	PricePerSegment  string // effective price per segment (last tier used)
}
