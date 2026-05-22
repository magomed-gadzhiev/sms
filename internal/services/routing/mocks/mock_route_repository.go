package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockRouteRepository is a mock implementation of domain.RouteRepository
type MockRouteRepository struct {
	mock.Mock
}

func (m *MockRouteRepository) Create(ctx context.Context, route *domain.Route) error {
	args := m.Called(ctx, route)
	return args.Error(0)
}

func (m *MockRouteRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Route, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Route), args.Error(1)
}

func (m *MockRouteRepository) Update(ctx context.Context, route *domain.Route) error {
	args := m.Called(ctx, route)
	return args.Error(0)
}

func (m *MockRouteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRouteRepository) List(ctx context.Context, activeOnly bool, limit, offset int) ([]*domain.Route, int, error) {
	args := m.Called(ctx, activeOnly, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Route), args.Int(1), args.Error(2)
}

func (m *MockRouteRepository) GetActiveByDestination(ctx context.Context, destination string) ([]*domain.Route, error) {
	args := m.Called(ctx, destination)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Route), args.Error(1)
}
