package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
)

// MockDLRRepository реализует domain.DLRRepository для тестов
type MockDLRRepository struct {
	mock.Mock
}

func (m *MockDLRRepository) Create(ctx context.Context, dlr *domain.DLRReceipt) error {
	args := m.Called(ctx, dlr)
	return args.Error(0)
}

func (m *MockDLRRepository) GetByMessageID(ctx context.Context, messageID uuid.UUID) ([]*domain.DLRReceipt, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DLRReceipt), args.Error(1)
}

func (m *MockDLRRepository) GetBySMPPMessageID(ctx context.Context, smppMessageID string) (*domain.DLRReceipt, error) {
	args := m.Called(ctx, smppMessageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DLRReceipt), args.Error(1)
}
