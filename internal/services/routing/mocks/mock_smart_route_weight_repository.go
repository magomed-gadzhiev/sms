package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockSmartRouteWeightRepository is a mock implementation of domain.SmartRouteWeightRepository
type MockSmartRouteWeightRepository struct {
	mock.Mock
}

func (m *MockSmartRouteWeightRepository) Upsert(ctx context.Context, weight *domain.SmartRouteWeight) error {
	args := m.Called(ctx, weight)
	return args.Error(0)
}

func (m *MockSmartRouteWeightRepository) GetByOperatorAndCountry(ctx context.Context, operatorCode, countryCode string) (*domain.SmartRouteWeight, error) {
	args := m.Called(ctx, operatorCode, countryCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SmartRouteWeight), args.Error(1)
}

func (m *MockSmartRouteWeightRepository) List(ctx context.Context, countryCode string) ([]*domain.SmartRouteWeight, error) {
	args := m.Called(ctx, countryCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SmartRouteWeight), args.Error(1)
}

func (m *MockSmartRouteWeightRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
