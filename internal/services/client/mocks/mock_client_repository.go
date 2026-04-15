package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	"github.com/stretchr/testify/mock"
)

// MockClientRepository is a mock implementation matching the methods of
// infrastructure/repository.ClientRepository (concrete struct, no domain interface).
type MockClientRepository struct {
	mock.Mock
}

func (m *MockClientRepository) Create(ctx context.Context, client *domain.Client) error {
	args := m.Called(ctx, client)
	return args.Error(0)
}

func (m *MockClientRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Client, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Client), args.Error(1)
}

func (m *MockClientRepository) Update(ctx context.Context, client *domain.Client) error {
	args := m.Called(ctx, client)
	return args.Error(0)
}

func (m *MockClientRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockClientRepository) List(ctx context.Context, activeOnly bool, search string, limit, offset int) ([]*domain.Client, int, error) {
	args := m.Called(ctx, activeOnly, search, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Client), args.Int(1), args.Error(2)
}

func (m *MockClientRepository) AssignPlan(ctx context.Context, clientID uuid.UUID, planID uuid.UUID) error {
	args := m.Called(ctx, clientID, planID)
	return args.Error(0)
}
