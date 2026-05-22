package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/stretchr/testify/mock"
)

type MockTariffPeriodRepository struct {
	mock.Mock
}

func (m *MockTariffPeriodRepository) Create(ctx context.Context, period *domain.TariffPeriod) error {
	args := m.Called(ctx, period)
	return args.Error(0)
}

func (m *MockTariffPeriodRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.TariffPeriod, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TariffPeriod), args.Error(1)
}

func (m *MockTariffPeriodRepository) GetActiveByPlanID(ctx context.Context, planID uuid.UUID, now time.Time) (*domain.TariffPeriod, error) {
	args := m.Called(ctx, planID, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TariffPeriod), args.Error(1)
}

func (m *MockTariffPeriodRepository) ListByPlanID(ctx context.Context, planID uuid.UUID) ([]*domain.TariffPeriod, error) {
	args := m.Called(ctx, planID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TariffPeriod), args.Error(1)
}

func (m *MockTariffPeriodRepository) HasActivePeriod(ctx context.Context, planID uuid.UUID, now time.Time) (bool, error) {
	args := m.Called(ctx, planID, now)
	return args.Bool(0), args.Error(1)
}
