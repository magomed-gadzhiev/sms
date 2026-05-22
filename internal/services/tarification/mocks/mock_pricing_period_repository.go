package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/stretchr/testify/mock"
)

type MockPricingPeriodRepository struct {
	mock.Mock
}

func (m *MockPricingPeriodRepository) Create(ctx context.Context, period *domain.PricingPeriod) error {
	args := m.Called(ctx, period)
	return args.Error(0)
}

func (m *MockPricingPeriodRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PricingPeriod, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PricingPeriod), args.Error(1)
}

func (m *MockPricingPeriodRepository) GetActiveByTariffPeriodID(ctx context.Context, tariffPeriodID uuid.UUID, now time.Time) (*domain.PricingPeriod, error) {
	args := m.Called(ctx, tariffPeriodID, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PricingPeriod), args.Error(1)
}

func (m *MockPricingPeriodRepository) ListByTariffPeriodID(ctx context.Context, tariffPeriodID uuid.UUID) ([]*domain.PricingPeriod, error) {
	args := m.Called(ctx, tariffPeriodID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PricingPeriod), args.Error(1)
}
