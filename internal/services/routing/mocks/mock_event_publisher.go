package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockEventPublisher is a mock implementation of domain.EventPublisher
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) PublishMessageRouted(ctx context.Context, messageID uuid.UUID, routeID uuid.UUID, providerID uuid.UUID) error {
	args := m.Called(ctx, messageID, routeID, providerID)
	return args.Error(0)
}
