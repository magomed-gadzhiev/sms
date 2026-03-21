package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockEventPublisher is a mock implementation of domain.EventPublisher.
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) PublishBalanceChanged(ctx context.Context, clientID string, balance, currency string) error {
	args := m.Called(ctx, clientID, balance, currency)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishTransactionCompleted(ctx context.Context, transactionID, clientID, transactionType, amount, currency string) error {
	args := m.Called(ctx, transactionID, clientID, transactionType, amount, currency)
	return args.Error(0)
}
