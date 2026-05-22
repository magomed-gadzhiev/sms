package mocks

import (
	"context"

	cascadekafka "github.com/smpp-server/smpp-server/internal/services/cascade/infrastructure/kafka"
	"github.com/stretchr/testify/mock"
)

// MockCascadeProducer is a mock implementation of application.CascadeProducer
type MockCascadeProducer struct {
	mock.Mock
}

func (m *MockCascadeProducer) PublishStart(ctx context.Context, evt cascadekafka.CascadeStartEvent) error {
	args := m.Called(ctx, evt)
	return args.Error(0)
}

func (m *MockCascadeProducer) PublishAttemptSend(ctx context.Context, cmd cascadekafka.CascadeAttemptSendCommand) error {
	args := m.Called(ctx, cmd)
	return args.Error(0)
}

func (m *MockCascadeProducer) PublishAttemptResult(ctx context.Context, evt cascadekafka.CascadeAttemptResultEvent) error {
	args := m.Called(ctx, evt)
	return args.Error(0)
}

func (m *MockCascadeProducer) PublishBilling(ctx context.Context, cmd cascadekafka.CascadeBillingCommand) error {
	args := m.Called(ctx, cmd)
	return args.Error(0)
}
