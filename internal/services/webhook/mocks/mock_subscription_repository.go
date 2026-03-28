package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
	"github.com/stretchr/testify/mock"
)

// MockSubscriptionRepository is a mock implementation of application.SubscriptionRepository.
type MockSubscriptionRepository struct {
	mock.Mock
}

func (m *MockSubscriptionRepository) Create(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error) {
	args := m.Called(ctx, sub)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.Subscription, error) {
	args := m.Called(ctx, id, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepository) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Subscription, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepository) Update(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error) {
	args := m.Called(ctx, sub)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	args := m.Called(ctx, id, clientID)
	return args.Error(0)
}

func (m *MockSubscriptionRepository) CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error) {
	args := m.Called(ctx, clientID)
	return args.Int(0), args.Error(1)
}
