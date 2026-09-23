package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// TariffPlanService предоставляет бизнес-логику для управления тарифными планами
type TariffPlanService struct {
	planRepo    domain.TariffPlanRepository
	periodRepo  domain.TariffPeriodRepository
	tierRepo    domain.TariffTierRepository
	pricingRepo domain.PricingPeriodRepository
	prepaidRepo domain.PrepaidFeeRepository
	logger      zerolog.Logger
}

// NewTariffPlanService создает новый сервис тарифных планов
func NewTariffPlanService(
	planRepo domain.TariffPlanRepository,
	periodRepo domain.TariffPeriodRepository,
	tierRepo domain.TariffTierRepository,
	pricingRepo domain.PricingPeriodRepository,
	prepaidRepo domain.PrepaidFeeRepository,
) *TariffPlanService {
	return &TariffPlanService{
		planRepo:    planRepo,
		periodRepo:  periodRepo,
		tierRepo:    tierRepo,
		pricingRepo: pricingRepo,
		prepaidRepo: prepaidRepo,
		logger:      log.With().Str("component", "tariff-plan-service").Logger(),
	}
}

// CreatePlan создает новый тарифный план
func (s *TariffPlanService) CreatePlan(ctx context.Context, operatorID uuid.UUID, category domain.SenderCategory, strategy domain.TarificationStrategy) (*domain.TariffPlan, error) {
	plan := domain.NewTariffPlan(operatorID, category, strategy)
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tariff plan: %w", err)
	}

	// Проверяем дубликат активного плана
	existing, err := s.planRepo.GetActiveByOperatorAndCategory(ctx, operatorID, category)
	if err == nil && existing != nil {
		return nil, domain.ErrTariffPlanDuplicate
	}

	if err := s.planRepo.Create(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to create tariff plan: %w", err)
	}

	s.logger.Info().
		Str("plan_id", plan.ID.String()).
		Str("operator_id", operatorID.String()).
		Str("category", string(category)).
		Str("strategy", string(strategy)).
		Msg("tariff plan created")

	return plan, nil
}

// UpdatePlan обновляет тарифный план (активация/деактивация)
func (s *TariffPlanService) UpdatePlan(ctx context.Context, id uuid.UUID, active bool) (*domain.TariffPlan, error) {
	plan, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get tariff plan: %w", err)
	}

	// Проверяем наличие активного периода при деактивации
	if !active {
		hasActive, err := s.periodRepo.HasActivePeriod(ctx, id, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to check active period: %w", err)
		}
		if hasActive {
			return nil, domain.ErrTariffPlanHasActivePeriod
		}
	}

	plan.Active = active
	plan.UpdatedAt = time.Now()

	if err := s.planRepo.Update(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to update tariff plan: %w", err)
	}

	s.logger.Info().
		Str("plan_id", id.String()).
		Bool("active", active).
		Msg("tariff plan updated")

	return plan, nil
}

// GetPlan получает тарифный план по ID
func (s *TariffPlanService) GetPlan(ctx context.Context, id uuid.UUID) (*domain.TariffPlan, error) {
	plan, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get tariff plan: %w", err)
	}
	return plan, nil
}

// ListPlans получает список тарифных планов
func (s *TariffPlanService) ListPlans(ctx context.Context, operatorID *uuid.UUID, activeOnly bool, limit, offset int) ([]*domain.TariffPlan, int, error) {
	plans, total, err := s.planRepo.List(ctx, operatorID, activeOnly, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tariff plans: %w", err)
	}
	return plans, total, nil
}

// CreatePeriod создает новый тарифный период для плана
func (s *TariffPlanService) CreatePeriod(ctx context.Context, planID uuid.UUID, startDate time.Time, endDate *time.Time) (*domain.TariffPeriod, error) {
	// Проверяем существование плана
	_, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tariff plan: %w", err)
	}

	period := domain.NewTariffPeriod(planID, startDate, endDate)
	if err := period.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tariff period: %w", err)
	}

	if err := s.periodRepo.Create(ctx, period); err != nil {
		return nil, fmt.Errorf("failed to create tariff period: %w", err)
	}

	logEvent := s.logger.Info().
		Str("period_id", period.ID.String()).
		Str("plan_id", planID.String()).
		Time("start_date", startDate)
	if endDate != nil {
		logEvent = logEvent.Time("end_date", *endDate)
	}
	logEvent.Msg("tariff period created")

	return period, nil
}

// CreateTier создает новый тарифный уровень для периода
func (s *TariffPlanService) CreateTier(ctx context.Context, periodID uuid.UUID, fromCount int, pricePerSegment string) (*domain.TariffTier, error) {
	// Проверяем существование периода
	period, err := s.periodRepo.GetByID(ctx, periodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tariff period: %w", err)
	}

	// Проверяем, что период не активен
	if period.IsActive(time.Now()) {
		return nil, domain.ErrTariffTierPeriodActive
	}

	tier := domain.NewTariffTier(periodID, fromCount, pricePerSegment)
	if err := tier.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tariff tier: %w", err)
	}

	if err := s.tierRepo.Create(ctx, tier); err != nil {
		return nil, fmt.Errorf("failed to create tariff tier: %w", err)
	}

	s.logger.Info().
		Str("tier_id", tier.ID.String()).
		Str("period_id", periodID.String()).
		Int("from_count", fromCount).
		Str("price_per_segment", pricePerSegment).
		Msg("tariff tier created")

	return tier, nil
}

// UpdateTier обновляет тарифный уровень
func (s *TariffPlanService) UpdateTier(ctx context.Context, id uuid.UUID, fromCount int, pricePerSegment string) (*domain.TariffTier, error) {
	tier, err := s.tierRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get tariff tier: %w", err)
	}

	// Проверяем, что период не активен
	period, err := s.periodRepo.GetByID(ctx, tier.TariffPeriodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tariff period: %w", err)
	}
	if period.IsActive(time.Now()) {
		return nil, domain.ErrTariffTierPeriodActive
	}

	tier.FromCount = fromCount
	tier.PricePerSegment = pricePerSegment

	if err := tier.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tariff tier: %w", err)
	}

	if err := s.tierRepo.Update(ctx, tier); err != nil {
		return nil, fmt.Errorf("failed to update tariff tier: %w", err)
	}

	s.logger.Info().
		Str("tier_id", id.String()).
		Int("from_count", fromCount).
		Str("price_per_segment", pricePerSegment).
		Msg("tariff tier updated")

	return tier, nil
}

// CreatePricingPeriod создает новый период тарификации внутри тарифного периода
func (s *TariffPlanService) CreatePricingPeriod(ctx context.Context, tariffPeriodID uuid.UUID, startDate, endDate time.Time) (*domain.PricingPeriod, error) {
	// Проверяем существование тарифного периода
	tariffPeriod, err := s.periodRepo.GetByID(ctx, tariffPeriodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tariff period: %w", err)
	}

	// Проверяем, что pricing period входит в границы tariff period
	if startDate.Before(tariffPeriod.StartDate) || (tariffPeriod.EndDate != nil && endDate.After(*tariffPeriod.EndDate)) {
		return nil, domain.ErrPricingPeriodOutOfBounds
	}

	period := domain.NewPricingPeriod(tariffPeriodID, startDate, endDate)
	if err := period.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pricing period: %w", err)
	}

	if err := s.pricingRepo.Create(ctx, period); err != nil {
		return nil, fmt.Errorf("failed to create pricing period: %w", err)
	}

	s.logger.Info().
		Str("pricing_period_id", period.ID.String()).
		Str("tariff_period_id", tariffPeriodID.String()).
		Time("start_date", startDate).
		Time("end_date", endDate).
		Msg("pricing period created")

	return period, nil
}

// CreatePrepaidFee создает новый предоплаченный сбор
func (s *TariffPlanService) CreatePrepaidFee(ctx context.Context, planID, periodID uuid.UUID, amount, currency string) (*domain.PrepaidFee, error) {
	// Проверяем существование плана
	_, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tariff plan: %w", err)
	}

	// Проверяем существование периода
	_, err = s.periodRepo.GetByID(ctx, periodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tariff period: %w", err)
	}

	fee := domain.NewPrepaidFee(planID, periodID, amount, currency)
	if err := fee.Validate(); err != nil {
		return nil, fmt.Errorf("invalid prepaid fee: %w", err)
	}

	if err := s.prepaidRepo.Create(ctx, fee); err != nil {
		return nil, fmt.Errorf("failed to create prepaid fee: %w", err)
	}

	s.logger.Info().
		Str("fee_id", fee.ID.String()).
		Str("plan_id", planID.String()).
		Str("period_id", periodID.String()).
		Str("amount", amount).
		Str("currency", currency).
		Msg("prepaid fee created")

	return fee, nil
}
