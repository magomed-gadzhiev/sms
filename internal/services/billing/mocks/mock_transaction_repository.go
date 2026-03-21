package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
	"github.com/stretchr/testify/mock"
)

// MockTransactionRepository is a mock implementation of domain.TransactionRepository.
type MockTransactionRepository struct {
	mock.Mock
}

func (m *MockTransactionRepository) Create(ctx context.Context, transaction *domain.Transaction) error {
	args := m.Called(ctx, transaction)
	return args.Error(0)
}

func (m *MockTransactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	args := m.Called(ctx, id)
	var tx *domain.Transaction
	if v := args.Get(0); v != nil {
		tx = v.(*domain.Transaction)
	}
	return tx, args.Error(1)
}

func (m *MockTransactionRepository) GetByClientID(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.Transaction, error) {
	args := m.Called(ctx, clientID, limit, offset)
	var txs []*domain.Transaction
	if v := args.Get(0); v != nil {
		txs = v.([]*domain.Transaction)
	}
	return txs, args.Error(1)
}

func (m *MockTransactionRepository) GetByClientIDAndType(ctx context.Context, clientID uuid.UUID, transactionType domain.TransactionType, limit, offset int) ([]*domain.Transaction, error) {
	args := m.Called(ctx, clientID, transactionType, limit, offset)
	var txs []*domain.Transaction
	if v := args.Get(0); v != nil {
		txs = v.([]*domain.Transaction)
	}
	return txs, args.Error(1)
}

func (m *MockTransactionRepository) GetByClientIDAndPeriod(ctx context.Context, clientID uuid.UUID, from, to time.Time, limit, offset int) ([]*domain.Transaction, error) {
	args := m.Called(ctx, clientID, from, to, limit, offset)
	var txs []*domain.Transaction
	if v := args.Get(0); v != nil {
		txs = v.([]*domain.Transaction)
	}
	return txs, args.Error(1)
}

func (m *MockTransactionRepository) GetByMessageID(ctx context.Context, messageID uuid.UUID) (*domain.Transaction, error) {
	args := m.Called(ctx, messageID)
	var tx *domain.Transaction
	if v := args.Get(0); v != nil {
		tx = v.(*domain.Transaction)
	}
	return tx, args.Error(1)
}
