package domain

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// SmartRouteWeight represents configurable weights for provider selection per operator/region
type SmartRouteWeight struct {
	ID            uuid.UUID
	OperatorCode  string
	CountryCode   string
	CostWeight    float64
	QualityWeight float64
	Active        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// DefaultCostWeight is the default cost weight when no custom weights are configured
const DefaultCostWeight = 0.60

// DefaultQualityWeight is the default quality weight when no custom weights are configured
const DefaultQualityWeight = 0.40

// NewSmartRouteWeight creates a new smart route weight configuration
func NewSmartRouteWeight(operatorCode, countryCode string, costWeight, qualityWeight float64) *SmartRouteWeight {
	now := time.Now()
	return &SmartRouteWeight{
		ID:            uuid.New(),
		OperatorCode:  operatorCode,
		CountryCode:   countryCode,
		CostWeight:    costWeight,
		QualityWeight: qualityWeight,
		Active:        true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// DefaultSmartRouteWeight returns the default weights
func DefaultSmartRouteWeight() *SmartRouteWeight {
	return &SmartRouteWeight{
		CostWeight:    DefaultCostWeight,
		QualityWeight: DefaultQualityWeight,
		Active:        true,
	}
}

// Validate checks that weights sum to 1.0
func (w *SmartRouteWeight) Validate() error {
	sum := w.CostWeight + w.QualityWeight
	if math.Abs(sum-1.0) > 0.001 {
		return ErrInvalidWeights
	}
	if w.CostWeight < 0 || w.CostWeight > 1 {
		return ErrInvalidWeights
	}
	if w.QualityWeight < 0 || w.QualityWeight > 1 {
		return ErrInvalidWeights
	}
	return nil
}

// ProviderCandidate represents a provider candidate for smart routing evaluation
type ProviderCandidate struct {
	ProviderID   uuid.UUID
	Cost         float64 // cost per message for this operator
	DeliveryRate float64 // historical delivery rate (0-100)
	Active       bool
	Healthy      bool // health check status
}

// CalculateScore calculates the weighted score for a provider candidate
// Higher score = better provider choice
// Cost is inverted (lower cost = higher score component)
func CalculateScore(candidate *ProviderCandidate, weights *SmartRouteWeight, maxCost, minCost float64) float64 {
	// Normalize cost (invert: lower cost = higher score)
	var normalizedCost float64
	if maxCost > minCost {
		normalizedCost = 1.0 - (candidate.Cost-minCost)/(maxCost-minCost)
	} else {
		normalizedCost = 1.0
	}

	// Normalize delivery rate (0-100 → 0-1)
	normalizedQuality := candidate.DeliveryRate / 100.0

	return weights.CostWeight*normalizedCost + weights.QualityWeight*normalizedQuality
}

// SelectOptimalProvider selects the provider with the highest weighted score
// Returns the selected provider ID and its score
// Excludes unhealthy/inactive providers
func SelectOptimalProvider(candidates []*ProviderCandidate, weights *SmartRouteWeight) (uuid.UUID, float64, error) {
	// Filter active and healthy candidates
	var eligible []*ProviderCandidate
	for _, c := range candidates {
		if c.Active && c.Healthy {
			eligible = append(eligible, c)
		}
	}

	if len(eligible) == 0 {
		return uuid.Nil, 0, ErrNoProviders
	}

	// Find min/max cost for normalization
	minCost := math.MaxFloat64
	maxCost := 0.0
	for _, c := range eligible {
		if c.Cost < minCost {
			minCost = c.Cost
		}
		if c.Cost > maxCost {
			maxCost = c.Cost
		}
	}

	// Calculate scores and find best
	var bestProvider *ProviderCandidate
	bestScore := -1.0
	for _, c := range eligible {
		score := CalculateScore(c, weights, maxCost, minCost)
		if score > bestScore {
			bestScore = score
			bestProvider = c
		}
	}

	return bestProvider.ProviderID, bestScore, nil
}
