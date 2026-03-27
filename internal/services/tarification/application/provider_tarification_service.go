package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// ProviderTarificationService handles provider cost accounting (Layer 2).
type ProviderTarificationService struct {
	planRepo   domain.ProviderTariffPlanRepository
	periodRepo domain.ProviderTariffPeriodRepository
	tierRepo   domain.ProviderTariffTierRepository
	usageRepo  domain.ProviderUsageCounterRepository
	logRepo    domain.ProviderTarificationLogRepository
	strategies map[domain.TarificationStrategy]BillingStrategy
}

func NewProviderTarificationService(
	planRepo domain.ProviderTariffPlanRepository,
	periodRepo domain.ProviderTariffPeriodRepository,
	tierRepo domain.ProviderTariffTierRepository,
	usageRepo domain.ProviderUsageCounterRepository,
	logRepo domain.ProviderTarificationLogRepository,
) *ProviderTarificationService {
	return &ProviderTarificationService{
		planRepo:   planRepo,
		periodRepo: periodRepo,
		tierRepo:   tierRepo,
		usageRepo:  usageRepo,
		logRepo:    logRepo,
		strategies: map[domain.TarificationStrategy]BillingStrategy{
			domain.StrategyFixed:            NewFixedStrategy(),
			domain.StrategyThreshold:        NewThresholdStrategy(),
			domain.StrategyThresholdRecalc:  NewThresholdRecalcStrategy(),
			domain.StrategyPrepaidThreshold: NewPrepaidThresholdStrategy(),
		},
	}
}

// TarifyProviderCostRequest is the input for provider cost accounting.
type TarifyProviderCostRequest struct {
	ProviderID     uuid.UUID
	OperatorID     uuid.UUID
	ClientID       uuid.UUID
	MessageID      uuid.UUID
	SegmentCount   int
	IdempotencyKey string
}

// TarifyProviderCost calculates and logs the provider cost for a sent message.
// This is called asynchronously after successful delivery.
func (s *ProviderTarificationService) TarifyProviderCost(ctx context.Context, req *TarifyProviderCostRequest) error {
	// 1. Idempotency check
	existing, err := s.logRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("idempotency check: %w", err)
	}
	if existing != nil {
		return nil // already processed
	}

	// 2. Find active provider tariff plan
	plan, err := s.planRepo.GetActiveByProviderAndOperator(ctx, req.ProviderID, req.OperatorID)
	if err != nil {
		log.Warn().Err(err).
			Str("provider_id", req.ProviderID.String()).
			Str("operator_id", req.OperatorID.String()).
			Msg("no active provider tariff plan, skipping cost accounting")
		return nil // no plan = no cost tracking (graceful)
	}

	// 3. Find active period
	now := time.Now()
	period, err := s.periodRepo.GetActiveByPlanID(ctx, plan.ID, now)
	if err != nil {
		log.Warn().Err(err).Str("plan_id", plan.ID.String()).Msg("no active provider tariff period")
		return nil
	}

	// 4. Get tiers
	tiers, err := s.tierRepo.ListByPeriodID(ctx, period.ID)
	if err != nil {
		return fmt.Errorf("get provider tiers: %w", err)
	}
	if len(tiers) == 0 {
		log.Warn().Str("period_id", period.ID.String()).Msg("no tiers for provider tariff period")
		return nil
	}

	// 5. Get usage counter
	counter, err := s.usageRepo.GetOrCreate(ctx, plan.ID, period.ID)
	if err != nil {
		return fmt.Errorf("get provider usage counter: %w", err)
	}

	// 6. Convert ProviderTariffTier to domain.TariffTier for strategy calculation
	domainTiers := make([]*domain.TariffTier, len(tiers))
	for i, t := range tiers {
		domainTiers[i] = &domain.TariffTier{
			ID:              t.ID,
			TariffPeriodID:  t.ProviderTariffPeriodID,
			FromCount:       t.FromCount,
			PricePerSegment: t.PricePerSegment,
		}
	}

	// 7. Calculate cost using same strategies as client tarification
	strategy, ok := s.strategies[plan.Strategy]
	if !ok {
		return fmt.Errorf("unknown provider strategy: %s", plan.Strategy)
	}

	result, err := strategy.Calculate(ctx, CalculationParams{
		CurrentCount: counter.SegmentCount,
		SegmentCount: req.SegmentCount,
		Tiers:        domainTiers,
	})
	if err != nil {
		return fmt.Errorf("provider cost calculation: %w", err)
	}

	// 8. Increment usage counter
	if _, err := s.usageRepo.IncrementAndGet(ctx, plan.ID, period.ID, req.SegmentCount); err != nil {
		log.Error().Err(err).Msg("failed to increment provider usage counter")
	}

	// 9. Write to provider_tarification_log
	logEntry := domain.NewProviderTarificationLog(
		req.ProviderID, req.OperatorID, req.ClientID, req.MessageID,
		plan.ID, period.ID, plan.Strategy,
		req.SegmentCount, result.PricePerSegment, result.ChargeAmount,
		req.IdempotencyKey,
	)
	if err := s.logRepo.Create(ctx, logEntry); err != nil {
		return fmt.Errorf("create provider tarification log: %w", err)
	}

	return nil
}
