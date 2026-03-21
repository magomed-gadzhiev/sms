package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// SmartRoutingService selects optimal providers based on weighted scoring
type SmartRoutingService struct {
	weightRepo domain.SmartRouteWeightRepository
	logger     zerolog.Logger
}

// NewSmartRoutingService creates a new smart routing service
func NewSmartRoutingService(weightRepo domain.SmartRouteWeightRepository) *SmartRoutingService {
	return &SmartRoutingService{
		weightRepo: weightRepo,
		logger:     log.With().Str("component", "smart-routing-service").Logger(),
	}
}

// SelectOptimalProvider selects the best provider based on weighted scoring
// Returns the selected provider ID, score, and error
func (s *SmartRoutingService) SelectOptimalProvider(
	ctx context.Context,
	operatorCode string,
	countryCode string,
	candidates []*domain.ProviderCandidate,
) (uuid.UUID, float64, error) {
	if len(candidates) == 0 {
		return uuid.Nil, 0, domain.ErrNoProviders
	}

	// Get weights for this operator/country (or defaults)
	weights, err := s.weightRepo.GetByOperatorAndCountry(ctx, operatorCode, countryCode)
	if err != nil {
		// Use default weights if not found
		weights = domain.DefaultSmartRouteWeight()
		s.logger.Debug().
			Str("operator", operatorCode).
			Str("country", countryCode).
			Msg("используются веса по умолчанию для smart routing")
	}

	providerID, score, err := domain.SelectOptimalProvider(candidates, weights)
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("ошибка выбора оптимального провайдера: %w", err)
	}

	s.logger.Debug().
		Str("provider_id", providerID.String()).
		Float64("score", score).
		Float64("cost_weight", weights.CostWeight).
		Float64("quality_weight", weights.QualityWeight).
		Msg("провайдер выбран smart routing")

	return providerID, score, nil
}

// SetWeights creates or updates weights for an operator/country
func (s *SmartRoutingService) SetWeights(ctx context.Context, operatorCode, countryCode string, costWeight, qualityWeight float64) (*domain.SmartRouteWeight, error) {
	weight := domain.NewSmartRouteWeight(operatorCode, countryCode, costWeight, qualityWeight)
	if err := weight.Validate(); err != nil {
		return nil, err
	}

	if err := s.weightRepo.Upsert(ctx, weight); err != nil {
		return nil, fmt.Errorf("ошибка сохранения весов: %w", err)
	}

	s.logger.Info().
		Str("operator", operatorCode).
		Str("country", countryCode).
		Float64("cost_weight", costWeight).
		Float64("quality_weight", qualityWeight).
		Msg("веса smart routing обновлены")

	return weight, nil
}

// GetWeights gets weights for an operator/country
func (s *SmartRoutingService) GetWeights(ctx context.Context, operatorCode, countryCode string) (*domain.SmartRouteWeight, error) {
	return s.weightRepo.GetByOperatorAndCountry(ctx, operatorCode, countryCode)
}

// ListWeights lists all configured weights
func (s *SmartRoutingService) ListWeights(ctx context.Context, countryCode string) ([]*domain.SmartRouteWeight, error) {
	return s.weightRepo.List(ctx, countryCode)
}

// DeleteWeights deletes weights by ID
func (s *SmartRoutingService) DeleteWeights(ctx context.Context, id uuid.UUID) error {
	return s.weightRepo.Delete(ctx, id)
}
