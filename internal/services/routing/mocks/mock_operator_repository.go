package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockOperatorRepository is a mock implementation of domain.OperatorRepository
type MockOperatorRepository struct {
	mock.Mock
}

func (m *MockOperatorRepository) Create(ctx context.Context, operator *domain.Operator) error {
	args := m.Called(ctx, operator)
	return args.Error(0)
}

func (m *MockOperatorRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Operator, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Operator), args.Error(1)
}

func (m *MockOperatorRepository) GetByCode(ctx context.Context, code string) (*domain.Operator, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Operator), args.Error(1)
}

func (m *MockOperatorRepository) Update(ctx context.Context, operator *domain.Operator) error {
	args := m.Called(ctx, operator)
	return args.Error(0)
}

func (m *MockOperatorRepository) List(ctx context.Context, countryID *uuid.UUID, activeOnly bool, limit, offset int) ([]*domain.Operator, int, error) {
	args := m.Called(ctx, countryID, activeOnly, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Operator), args.Int(1), args.Error(2)
}
