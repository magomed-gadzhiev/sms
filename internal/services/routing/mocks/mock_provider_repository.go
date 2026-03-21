package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockProviderRepository is a mock implementation of domain.ProviderRepository
type MockProviderRepository struct {
	mock.Mock
}

func (m *MockProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderInfo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderInfo), args.Error(1)
}

func (m *MockProviderRepository) GetAllActive(ctx context.Context) ([]*domain.ProviderInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ProviderInfo), args.Error(1)
}

func (m *MockProviderRepository) GetHealth(ctx context.Context, id uuid.UUID) (*domain.ProviderHealth, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderHealth), args.Error(1)
}
