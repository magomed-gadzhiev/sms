package mocks

import (
	"context"
	"time"

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

func (m *MockAccountRepository) FreezeAccount(ctx context.Context, clientID uuid.UUID, adminID uuid.UUID) (time.Time, error) {
	args := m.Called(ctx, clientID, adminID)
	return args.Get(0).(time.Time), args.Error(1)
}

func (m *MockAccountRepository) UnfreezeAccount(ctx context.Context, clientID uuid.UUID) error {
	args := m.Called(ctx, clientID)
	return args.Error(0)
}

func (m *MockAccountRepository) SetCreditLimit(ctx context.Context, clientID uuid.UUID, limit string) error {
	args := m.Called(ctx, clientID, limit)
	return args.Error(0)
}

func (m *MockAccountRepository) SetLowBalanceThreshold(ctx context.Context, clientID uuid.UUID, threshold string) error {
	args := m.Called(ctx, clientID, threshold)
	return args.Error(0)
}

func (m *MockAccountRepository) ListBalances(ctx context.Context, search string, status string, belowThreshold bool, limit, offset int32) ([]domain.BalanceInfo, int32, error) {
	args := m.Called(ctx, search, status, belowThreshold, limit, offset)
	var result []domain.BalanceInfo
	if v := args.Get(0); v != nil {
		result = v.([]domain.BalanceInfo)
	}
	return result, args.Get(1).(int32), args.Error(2)
}
