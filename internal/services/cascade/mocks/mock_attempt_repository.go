package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	"github.com/stretchr/testify/mock"
)

// MockAttemptRepository is a mock implementation of domain.AttemptRepository
type MockAttemptRepository struct {
	mock.Mock
}

func (m *MockAttemptRepository) Create(ctx context.Context, a *domain.DeliveryAttempt) error {
	args := m.Called(ctx, a)
	return args.Error(0)
}

func (m *MockAttemptRepository) Get(ctx context.Context, id uuid.UUID) (*domain.DeliveryAttempt, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DeliveryAttempt), args.Error(1)
}

func (m *MockAttemptRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.AttemptStatus, providerRef string, errMsg string, resultAt *time.Time) error {
	args := m.Called(ctx, id, status, providerRef, errMsg, resultAt)
	return args.Error(0)
}

func (m *MockAttemptRepository) UpdateCost(ctx context.Context, id uuid.UUID, cost float64) error {
	args := m.Called(ctx, id, cost)
	return args.Error(0)
}

func (m *MockAttemptRepository) ListByDelivery(ctx context.Context, deliveryID uuid.UUID) ([]*domain.DeliveryAttempt, error) {
	args := m.Called(ctx, deliveryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DeliveryAttempt), args.Error(1)
}

func (m *MockAttemptRepository) FindPendingTimedOut(ctx context.Context) ([]*domain.DeliveryAttempt, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DeliveryAttempt), args.Error(1)
}
