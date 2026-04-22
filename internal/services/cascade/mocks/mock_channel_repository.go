package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	"github.com/stretchr/testify/mock"
)

// MockChannelRepository is a mock implementation of domain.ChannelRepository
type MockChannelRepository struct {
	mock.Mock
}

func (m *MockChannelRepository) List(ctx context.Context) ([]*domain.ChannelConfig, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ChannelConfig), args.Error(1)
}

func (m *MockChannelRepository) Get(ctx context.Context, id uuid.UUID) (*domain.ChannelConfig, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ChannelConfig), args.Error(1)
}

func (m *MockChannelRepository) GetByType(ctx context.Context, ct domain.ChannelType) (*domain.ChannelConfig, error) {
	args := m.Called(ctx, ct)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ChannelConfig), args.Error(1)
}

func (m *MockChannelRepository) Create(ctx context.Context, ch *domain.ChannelConfig) error {
	args := m.Called(ctx, ch)
	return args.Error(0)
}

func (m *MockChannelRepository) Update(ctx context.Context, ch *domain.ChannelConfig) error {
	args := m.Called(ctx, ch)
	return args.Error(0)
}

func (m *MockChannelRepository) Toggle(ctx context.Context, id uuid.UUID, active bool) error {
	args := m.Called(ctx, id, active)
	return args.Error(0)
}
