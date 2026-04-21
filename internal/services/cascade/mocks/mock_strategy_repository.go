package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	"github.com/stretchr/testify/mock"
)

// MockStrategyRepository is a mock implementation of domain.StrategyRepository
type MockStrategyRepository struct {
	mock.Mock
}

func (m *MockStrategyRepository) List(ctx context.Context, activeOnly bool) ([]*domain.DeliveryStrategy, error) {
	args := m.Called(ctx, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DeliveryStrategy), args.Error(1)
}

func (m *MockStrategyRepository) Get(ctx context.Context, id uuid.UUID) (*domain.DeliveryStrategy, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DeliveryStrategy), args.Error(1)
}

func (m *MockStrategyRepository) Create(ctx context.Context, s *domain.DeliveryStrategy) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}

func (m *MockStrategyRepository) Update(ctx context.Context, s *domain.DeliveryStrategy) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}

func (m *MockStrategyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
