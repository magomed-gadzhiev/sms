package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/stretchr/testify/mock"
)

type MockTariffTierRepository struct {
	mock.Mock
}

func (m *MockTariffTierRepository) Create(ctx context.Context, tier *domain.TariffTier) error {
	args := m.Called(ctx, tier)
	return args.Error(0)
}

func (m *MockTariffTierRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.TariffTier, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TariffTier), args.Error(1)
}

func (m *MockTariffTierRepository) Update(ctx context.Context, tier *domain.TariffTier) error {
	args := m.Called(ctx, tier)
	return args.Error(0)
}

func (m *MockTariffTierRepository) ListByPeriodID(ctx context.Context, periodID uuid.UUID) ([]*domain.TariffTier, error) {
	args := m.Called(ctx, periodID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TariffTier), args.Error(1)
}
