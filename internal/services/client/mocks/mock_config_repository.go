package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	"github.com/stretchr/testify/mock"
)

// MockConfigRepository is a mock implementation matching the methods of
// infrastructure/repository.ConfigRepository (concrete struct, no domain interface).
type MockConfigRepository struct {
	mock.Mock
}

func (m *MockConfigRepository) Create(ctx context.Context, config *domain.ClientConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockConfigRepository) GetByClientID(ctx context.Context, clientID uuid.UUID) (*domain.ClientConfig, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ClientConfig), args.Error(1)
}

func (m *MockConfigRepository) Update(ctx context.Context, config *domain.ClientConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockConfigRepository) Upsert(ctx context.Context, config *domain.ClientConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockConfigRepository) UpdateRateLimits(ctx context.Context, clientID uuid.UUID, limits *domain.RateLimits) error {
	args := m.Called(ctx, clientID, limits)
	return args.Error(0)
}
