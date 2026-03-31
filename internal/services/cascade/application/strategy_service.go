package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
)

// StrategyService управляет стратегиями каскадной доставки
type StrategyService struct {
	strategies domain.StrategyRepository
	deliveries domain.DeliveryRepository
}

func NewStrategyService(strategies domain.StrategyRepository, deliveries domain.DeliveryRepository) *StrategyService {
	return &StrategyService{
		strategies: strategies,
		deliveries: deliveries,
	}
}

func (s *StrategyService) List(ctx context.Context, activeOnly bool) ([]*domain.DeliveryStrategy, error) {
	return s.strategies.List(ctx, activeOnly)
}

func (s *StrategyService) Get(ctx context.Context, id uuid.UUID) (*domain.DeliveryStrategy, error) {
	return s.strategies.Get(ctx, id)
}

type CreateStrategyInput struct {
	Name        string
	Description string
	Mode        domain.StrategyMode
	Steps       []CreateStrategyStepInput
}

type CreateStrategyStepInput struct {
	ChannelID uuid.UUID
	StepOrder int
	TimeoutS  int
	Billable  bool
}

func (s *StrategyService) Create(ctx context.Context, input CreateStrategyInput) (*domain.DeliveryStrategy, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("strategy name is required")
	}
	if !input.Mode.IsValid() {
		return nil, fmt.Errorf("invalid strategy mode: %s", input.Mode)
	}
	if len(input.Steps) == 0 {
		return nil, fmt.Errorf("strategy must have at least one step")
	}

	strategy := domain.NewDeliveryStrategy(input.Name, input.Description, input.Mode)
	for _, step := range input.Steps {
		strategy.AddStep(step.ChannelID, "", "", step.StepOrder, step.TimeoutS, step.Billable)
	}

	if err := strategy.Validate(); err != nil {
		return nil, fmt.Errorf("invalid strategy: %w", err)
	}

	if err := s.strategies.Create(ctx, strategy); err != nil {
		return nil, fmt.Errorf("create strategy: %w", err)
	}
	return strategy, nil
}

func (s *StrategyService) Update(ctx context.Context, id uuid.UUID, input CreateStrategyInput) (*domain.DeliveryStrategy, error) {
	strategy, err := s.strategies.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Нельзя изменять стратегию, если есть активные доставки
	hasActive, err := s.deliveries.HasActiveByStrategy(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("check active deliveries: %w", err)
	}
	if hasActive {
		return nil, domain.ErrStrategyHasActiveDeliveries
	}

	if input.Name != "" {
		strategy.Name = input.Name
	}
	if input.Description != "" {
		strategy.Description = input.Description
	}
	if input.Mode.IsValid() {
		strategy.Mode = input.Mode
	}
	if len(input.Steps) > 0 {
		strategy.Steps = make([]domain.StrategyStep, 0, len(input.Steps))
		for _, step := range input.Steps {
			strategy.AddStep(step.ChannelID, "", "", step.StepOrder, step.TimeoutS, step.Billable)
		}
	}

	if err := s.strategies.Update(ctx, strategy); err != nil {
		return nil, fmt.Errorf("update strategy: %w", err)
	}
	return strategy, nil
}

func (s *StrategyService) Delete(ctx context.Context, id uuid.UUID) error {
	hasActive, err := s.deliveries.HasActiveByStrategy(ctx, id)
	if err != nil {
		return fmt.Errorf("check active deliveries: %w", err)
	}
	if hasActive {
		return domain.ErrStrategyHasActiveDeliveries
	}
	return s.strategies.Delete(ctx, id)
}
