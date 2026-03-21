package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockHLRProviderRepository is a mock implementation of domain.HLRProviderRepository
type MockHLRProviderRepository struct {
	mock.Mock
}

func (m *MockHLRProviderRepository) Create(ctx context.Context, provider *domain.HLRProvider) error {
	args := m.Called(ctx, provider)
	return args.Error(0)
}

func (m *MockHLRProviderRepository) Update(ctx context.Context, provider *domain.HLRProvider) error {
	args := m.Called(ctx, provider)
	return args.Error(0)
}

func (m *MockHLRProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockHLRProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.HLRProvider, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.HLRProvider), args.Error(1)
}

func (m *MockHLRProviderRepository) ListActive(ctx context.Context) ([]*domain.HLRProvider, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.HLRProvider), args.Error(1)
}

func (m *MockHLRProviderRepository) GetByPriority(ctx context.Context, countryCode string) ([]*domain.HLRProvider, error) {
	args := m.Called(ctx, countryCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.HLRProvider), args.Error(1)
}
