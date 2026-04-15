package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
	"github.com/stretchr/testify/mock"
)

// MockProviderRepository is a mock implementation of domain.ProviderRepository
type MockProviderRepository struct {
	mock.Mock
}

func (m *MockProviderRepository) Create(ctx context.Context, provider *domain.Provider) error {
	args := m.Called(ctx, provider)
	return args.Error(0)
}

func (m *MockProviderRepository) Update(ctx context.Context, provider *domain.Provider) error {
	args := m.Called(ctx, provider)
	return args.Error(0)
}

func (m *MockProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Provider), args.Error(1)
}

func (m *MockProviderRepository) GetByName(ctx context.Context, name string) (*domain.Provider, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Provider), args.Error(1)
}

func (m *MockProviderRepository) List(ctx context.Context, activeOnly bool, limit, offset int) ([]*domain.Provider, int, error) {
	args := m.Called(ctx, activeOnly, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Provider), args.Int(1), args.Error(2)
}

func (m *MockProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProviderRepository) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Provider, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Provider), args.Error(1)
}

func (m *MockProviderRepository) CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error) {
	args := m.Called(ctx, clientID)
	return args.Int(0), args.Error(1)
}

func (m *MockProviderRepository) GetByIDAndClientID(ctx context.Context, id, clientID uuid.UUID) (*domain.Provider, error) {
	args := m.Called(ctx, id, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Provider), args.Error(1)
}

func (m *MockProviderRepository) LinkToClient(ctx context.Context, providerID, clientID uuid.UUID, ownership string) error {
	args := m.Called(ctx, providerID, clientID, ownership)
	return args.Error(0)
}

func (m *MockProviderRepository) UnlinkFromClient(ctx context.Context, providerID, clientID uuid.UUID) error {
	args := m.Called(ctx, providerID, clientID)
	return args.Error(0)
}

// ProviderRepository — псевдоним для использования в тестах application пакета
type ProviderRepository = MockProviderRepository
