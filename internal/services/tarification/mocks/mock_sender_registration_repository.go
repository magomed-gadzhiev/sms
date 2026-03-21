package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/stretchr/testify/mock"
)

type MockSenderRegistrationRepository struct {
	mock.Mock
}

func (m *MockSenderRegistrationRepository) Create(ctx context.Context, reg *domain.SenderRegistration) error {
	args := m.Called(ctx, reg)
	return args.Error(0)
}

func (m *MockSenderRegistrationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.SenderRegistration, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SenderRegistration), args.Error(1)
}

func (m *MockSenderRegistrationRepository) GetByClientAndOperator(ctx context.Context, clientID, operatorID uuid.UUID) ([]*domain.SenderRegistration, error) {
	args := m.Called(ctx, clientID, operatorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SenderRegistration), args.Error(1)
}

func (m *MockSenderRegistrationRepository) GetActiveByClientOperatorName(ctx context.Context, clientID, operatorID uuid.UUID, senderName string) (*domain.SenderRegistration, error) {
	args := m.Called(ctx, clientID, operatorID, senderName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SenderRegistration), args.Error(1)
}

func (m *MockSenderRegistrationRepository) Update(ctx context.Context, reg *domain.SenderRegistration) error {
	args := m.Called(ctx, reg)
	return args.Error(0)
}

func (m *MockSenderRegistrationRepository) List(ctx context.Context, clientID, operatorID *uuid.UUID, limit, offset int) ([]*domain.SenderRegistration, int, error) {
	args := m.Called(ctx, clientID, operatorID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.SenderRegistration), args.Int(1), args.Error(2)
}
