package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/stretchr/testify/mock"
)

type MockPrepaidFeeRepository struct {
	mock.Mock
}

func (m *MockPrepaidFeeRepository) Create(ctx context.Context, fee *domain.PrepaidFee) error {
	args := m.Called(ctx, fee)
	return args.Error(0)
}

func (m *MockPrepaidFeeRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PrepaidFee, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PrepaidFee), args.Error(1)
}

func (m *MockPrepaidFeeRepository) GetByPeriodID(ctx context.Context, tariffPeriodID uuid.UUID) (*domain.PrepaidFee, error) {
	args := m.Called(ctx, tariffPeriodID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PrepaidFee), args.Error(1)
}

func (m *MockPrepaidFeeRepository) GetUncharged(ctx context.Context, now time.Time) ([]*domain.PrepaidFee, error) {
	args := m.Called(ctx, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PrepaidFee), args.Error(1)
}

func (m *MockPrepaidFeeRepository) MarkCharged(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
