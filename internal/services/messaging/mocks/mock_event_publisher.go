package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
)

// MockEventPublisher реализует domain.EventPublisher для тестов
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) PublishMessageCreated(ctx context.Context, msg *domain.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishMessageQueued(ctx context.Context, msg *domain.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishMessageStatusChanged(ctx context.Context, msg *domain.Message, oldStatus string) error {
	args := m.Called(ctx, msg, oldStatus)
	return args.Error(0)
}
