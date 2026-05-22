package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/stretchr/testify/mock"
)

type MockUsageCounterRepository struct {
	mock.Mock
}

func (m *MockUsageCounterRepository) GetOrCreate(ctx context.Context, clientID, tariffPlanID, tariffPeriodID uuid.UUID) (*domain.UsageCounter, error) {
	args := m.Called(ctx, clientID, tariffPlanID, tariffPeriodID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UsageCounter), args.Error(1)
}

func (m *MockUsageCounterRepository) IncrementAndGet(ctx context.Context, clientID, tariffPlanID, tariffPeriodID uuid.UUID, segments int) (*domain.UsageCounter, error) {
	args := m.Called(ctx, clientID, tariffPlanID, tariffPeriodID, segments)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UsageCounter), args.Error(1)
}

func (m *MockUsageCounterRepository) GetByClient(ctx context.Context, clientID uuid.UUID, tariffPlanID *uuid.UUID, limit, offset int) ([]*domain.UsageCounter, int, error) {
	args := m.Called(ctx, clientID, tariffPlanID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.UsageCounter), args.Int(1), args.Error(2)
}
