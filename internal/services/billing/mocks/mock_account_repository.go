package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
	"github.com/stretchr/testify/mock"
)

// MockAccountRepository is a mock implementation of domain.AccountRepository.
type MockAccountRepository struct {
	mock.Mock
}

func (m *MockAccountRepository) Create(ctx context.Context, account *domain.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAccountRepository) GetByClientID(ctx context.Context, clientID uuid.UUID) (*domain.Account, error) {
	args := m.Called(ctx, clientID)
	var acc *domain.Account
	if v := args.Get(0); v != nil {
		acc = v.(*domain.Account)
	}
	return acc, args.Error(1)
}

func (m *MockAccountRepository) Update(ctx context.Context, account *domain.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAccountRepository) UpdateBalance(ctx context.Context, clientID uuid.UUID, newBalance string) error {
	args := m.Called(ctx, clientID, newBalance)
	return args.Error(0)
}
