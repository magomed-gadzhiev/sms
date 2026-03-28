package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	"github.com/stretchr/testify/mock"
)

// MockAPIKeyRepository мок для интерфейса APIKeyRepository
type MockAPIKeyRepository struct {
	mock.Mock
}

func (m *MockAPIKeyRepository) Create(ctx context.Context, apiKey *domain.APIKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) GetByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	args := m.Called(ctx, keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) Update(ctx context.Context, apiKey *domain.APIKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
