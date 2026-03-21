package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockEventConsumer is a mock implementation of domain.EventConsumer
type MockEventConsumer struct {
	mock.Mock
}

func (m *MockEventConsumer) ConsumeMessageCreated(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64) error) error {
	args := m.Called(ctx, handler)
	return args.Error(0)
}

func (m *MockEventConsumer) ConsumeMessageSent(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64) error) error {
	args := m.Called(ctx, handler)
	return args.Error(0)
}

func (m *MockEventConsumer) ConsumeMessageDelivered(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64) error) error {
	args := m.Called(ctx, handler)
	return args.Error(0)
}

func (m *MockEventConsumer) ConsumeMessageFailed(ctx context.Context, handler func(messageID string, clientID *string, providerID *string, timestamp int64, reason string) error) error {
	args := m.Called(ctx, handler)
	return args.Error(0)
}
