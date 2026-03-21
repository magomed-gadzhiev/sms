package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/stretchr/testify/mock"
)

type MockTariffPlanRepository struct {
	mock.Mock
}

func (m *MockTariffPlanRepository) Create(ctx context.Context, plan *domain.TariffPlan) error {
	args := m.Called(ctx, plan)
	return args.Error(0)
}

func (m *MockTariffPlanRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.TariffPlan, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TariffPlan), args.Error(1)
}

func (m *MockTariffPlanRepository) GetActiveByOperatorAndCategory(ctx context.Context, operatorID uuid.UUID, category domain.SenderCategory) (*domain.TariffPlan, error) {
	args := m.Called(ctx, operatorID, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TariffPlan), args.Error(1)
}

func (m *MockTariffPlanRepository) Update(ctx context.Context, plan *domain.TariffPlan) error {
	args := m.Called(ctx, plan)
	return args.Error(0)
}

func (m *MockTariffPlanRepository) List(ctx context.Context, operatorID *uuid.UUID, activeOnly bool, limit, offset int) ([]*domain.TariffPlan, int, error) {
	args := m.Called(ctx, operatorID, activeOnly, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.TariffPlan), args.Int(1), args.Error(2)
}
