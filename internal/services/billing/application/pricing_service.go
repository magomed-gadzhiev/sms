package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
)

// PricingService предоставляет бизнес-логику для тарификации
type PricingService struct {
	pricingRepo domain.PricingRuleRepository
	logger      zerolog.Logger
}

// NewPricingService создает новый сервис тарификации
func NewPricingService(pricingRepo domain.PricingRuleRepository) *PricingService {
	return &PricingService{
		pricingRepo: pricingRepo,
		logger:      log.With().Str("component", "pricing-service").Logger(),
	}
}

// GetPricingRules получает правила тарификации для клиента
func (s *PricingService) GetPricingRules(ctx context.Context, clientID *uuid.UUID) ([]*domain.PricingRule, error) {
	rules, err := s.pricingRepo.GetByClientID(ctx, clientID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get pricing rules: %w", err)
	}
	return rules, nil
}

// CreatePricingRule создает новое правило тарификации
func (s *PricingService) CreatePricingRule(
	ctx context.Context,
	clientID *uuid.UUID,
	destinationPattern, pricePerMessage, currency string,
	priority int,
	active bool,
) (*domain.PricingRule, error) {
	rule := domain.NewPricingRule(clientID, destinationPattern, pricePerMessage, currency, priority)

	// Устанавливаем активность правила
	if !active {
		rule.Deactivate()
	}

	if err := rule.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pricing rule: %w", err)
	}

	if err := s.pricingRepo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to create pricing rule: %w", err)
	}

	return rule, nil
}

// GetPriceForDestination получает цену для указанного номера назначения
func (s *PricingService) GetPriceForDestination(
	ctx context.Context,
	clientID *uuid.UUID,
	destination string,
) (string, string, error) {
	rule, err := s.pricingRepo.GetMatchingRule(ctx, clientID, destination)
	if err != nil {
		if errors.Is(err, domain.ErrPricingRuleNotFound) {
			// Возвращаем дефолтную цену
			return "0.01", "RUB", nil
		}
		return "", "", fmt.Errorf("failed to get pricing rule: %w", err)
	}

	return rule.PricePerMessage, rule.Currency, nil
}

// UpdatePricingRule обновляет правило тарификации
func (s *PricingService) UpdatePricingRule(ctx context.Context, rule *domain.PricingRule) error {
	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid pricing rule: %w", err)
	}

	rule.UpdatedAt = time.Now()
	if err := s.pricingRepo.Update(ctx, rule); err != nil {
		return fmt.Errorf("failed to update pricing rule: %w", err)
	}

	return nil
}

// DeletePricingRule удаляет правило тарификации
func (s *PricingService) DeletePricingRule(ctx context.Context, ruleID uuid.UUID) error {
	if err := s.pricingRepo.Delete(ctx, ruleID); err != nil {
		return fmt.Errorf("failed to delete pricing rule: %w", err)
	}
	return nil
}
