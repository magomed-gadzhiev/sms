package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockOperatorPrefixRepository is a mock implementation of domain.OperatorPrefixRepository
type MockOperatorPrefixRepository struct {
	mock.Mock
}

func (m *MockOperatorPrefixRepository) Create(ctx context.Context, prefix *domain.OperatorPrefix) error {
	args := m.Called(ctx, prefix)
	return args.Error(0)
}

func (m *MockOperatorPrefixRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOperatorPrefixRepository) ListByOperatorID(ctx context.Context, operatorID uuid.UUID) ([]*domain.OperatorPrefix, error) {
	args := m.Called(ctx, operatorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.OperatorPrefix), args.Error(1)
}

func (m *MockOperatorPrefixRepository) FindByNumber(ctx context.Context, phoneNumber string) (*domain.OperatorPrefix, error) {
	args := m.Called(ctx, phoneNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.OperatorPrefix), args.Error(1)
}
