package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
)

// MockMessageRepository реализует domain.MessageRepository для тестов
type MockMessageRepository struct {
	mock.Mock
}

func (m *MockMessageRepository) Create(ctx context.Context, msg *domain.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) GetByMessageID(ctx context.Context, messageID string) (*domain.Message, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) GetByExternalID(ctx context.Context, externalID string) (*domain.Message, error) {
	args := m.Called(ctx, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) GetBySMPPMessageID(ctx context.Context, smppMessageID string) (*domain.Message, error) {
	args := m.Called(ctx, smppMessageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) Update(ctx context.Context, msg *domain.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockMessageRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, statusMessage string) error {
	args := m.Called(ctx, id, status, statusMessage)
	return args.Error(0)
}

func (m *MockMessageRepository) GetByClientID(ctx context.Context, clientID uuid.UUID, limit, offset int, status *string) ([]*domain.Message, error) {
	args := m.Called(ctx, clientID, limit, offset, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) GetPendingForRetry(ctx context.Context, limit int) ([]*domain.Message, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) GetScheduledReady(ctx context.Context, limit int) ([]*domain.Message, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) GetStuckPending(ctx context.Context, threshold time.Duration, limit int) ([]*domain.Message, error) {
	args := m.Called(ctx, threshold, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) CancelByIDAndStatus(ctx context.Context, id uuid.UUID, clientID uuid.UUID) error {
	args := m.Called(ctx, id, clientID)
	return args.Error(0)
}

func (m *MockMessageRepository) GetSentExpired(ctx context.Context, timeout time.Duration, limit int) ([]*domain.Message, error) {
	args := m.Called(ctx, timeout, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) BulkUpdateStatusToExpired(ctx context.Context, messages []*domain.Message) error {
	args := m.Called(ctx, messages)
	return args.Error(0)
}

func (m *MockMessageRepository) ListScheduled(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.Message, int, error) {
	args := m.Called(ctx, clientID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Message), args.Int(1), args.Error(2)
}
