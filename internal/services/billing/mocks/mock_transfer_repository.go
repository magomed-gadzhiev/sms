package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
	"github.com/stretchr/testify/mock"
)

// MockTransferRepository is a mock implementation of domain.TransferRepository.
type MockTransferRepository struct {
	mock.Mock
}

func (m *MockTransferRepository) Create(ctx context.Context, transfer *domain.BalanceTransfer) error {
	args := m.Called(ctx, transfer)
	return args.Error(0)
}

func (m *MockTransferRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.BalanceTransfer, error) {
	args := m.Called(ctx, id)
	var bt *domain.BalanceTransfer
	if v := args.Get(0); v != nil {
		bt = v.(*domain.BalanceTransfer)
	}
	return bt, args.Error(1)
}
