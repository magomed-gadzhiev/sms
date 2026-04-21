package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	cascadekafka "github.com/smpp-server/smpp-server/internal/services/cascade/infrastructure/kafka"
	"github.com/smpp-server/smpp-server/internal/services/cascade/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockChannel is a local testify/mock-based implementation of domain.Channel
type mockChannel struct {
	mock.Mock
	channelType domain.ChannelType
}

func (c *mockChannel) Type() domain.ChannelType {
	return c.channelType
}

func (c *mockChannel) Send(ctx context.Context, attempt *domain.DeliveryAttempt, cfg *domain.ChannelConfig) error {
	args := c.Called(ctx, attempt, cfg)
	return args.Error(0)
}

// newTestService constructs a CascadeService wired with the supplied mocks.
// Pass nil for channelAdapters to use an empty map (no adapters).
func newTestService(
	deliveries *mocks.MockDeliveryRepository,
	attempts *mocks.MockAttemptRepository,
	strategies *mocks.MockStrategyRepository,
	channels *mocks.MockChannelRepository,
	producer *mocks.MockCascadeProducer,
	adapters map[domain.ChannelType]domain.Channel,
) *CascadeService {
	if adapters == nil {
		adapters = map[domain.ChannelType]domain.Channel{}
	}
	return NewCascadeService(
		deliveries,
		attempts,
		strategies,
		channels,
		producer,
		adapters,
		zerolog.Nop(),
	)
}

// ─── CreateDelivery ───────────────────────────────────────────────────────────

func TestCreateDelivery_HappyPath(t *testing.T) {
	ctx := context.Background()

	clientID := uuid.New()
	strategyID := uuid.New()
	recipient := "79001234567"
	text := "Hello"
	senderName := "TestSender"
	requestID := "req-001"

	strategy := &domain.DeliveryStrategy{
		ID:     strategyID,
		Name:   "default",
		Mode:   domain.ModeSequential,
		Active: true,
		Steps:  []domain.StrategyStep{},
	}

	deliveries := &mocks.MockDeliveryRepository{}
	attempts := &mocks.MockAttemptRepository{}
	strategies := &mocks.MockStrategyRepository{}
	channels := &mocks.MockChannelRepository{}
	producer := &mocks.MockCascadeProducer{}

	strategies.On("Get", ctx, strategyID).Return(strategy, nil)
	deliveries.On("Create", ctx, mock.AnythingOfType("*domain.Delivery")).Return(nil)
	producer.On("PublishStart", ctx, mock.AnythingOfType("kafka.CascadeStartEvent")).Return(nil)

	svc := newTestService(deliveries, attempts, strategies, channels, producer, nil)

	delivery, err := svc.CreateDelivery(ctx, clientID, strategyID, recipient, text, senderName, requestID, nil)

	require.NoError(t, err)
	require.NotNil(t, delivery)
	assert.Equal(t, clientID, delivery.ClientID)
	assert.Equal(t, strategyID, delivery.StrategyID)
	assert.Equal(t, recipient, delivery.Recipient)
	assert.Equal(t, text, delivery.Text)
	assert.Equal(t, senderName, delivery.SenderName)
	assert.Equal(t, domain.DeliveryPending, delivery.Status)

	strategies.AssertExpectations(t)
	deliveries.AssertExpectations(t)
	producer.AssertExpectations(t)
}

func TestCreateDelivery_StrategyNotFound(t *testing.T) {
	ctx := context.Background()

	clientID := uuid.New()
	strategyID := uuid.New()

	deliveries := &mocks.MockDeliveryRepository{}
	attempts := &mocks.MockAttemptRepository{}
	strategies := &mocks.MockStrategyRepository{}
	channels := &mocks.MockChannelRepository{}
	producer := &mocks.MockCascadeProducer{}

	strategies.On("Get", ctx, strategyID).Return(nil, domain.ErrStrategyNotFound)

	svc := newTestService(deliveries, attempts, strategies, channels, producer, nil)

	delivery, err := svc.CreateDelivery(ctx, clientID, strategyID, "79001234567", "text", "sender", "req-002", nil)

	require.Error(t, err)
	assert.Nil(t, delivery)
	assert.True(t, errors.Is(err, domain.ErrStrategyNotFound), "expected wrapped ErrStrategyNotFound, got: %v", err)

	strategies.AssertExpectations(t)
	deliveries.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	producer.AssertNotCalled(t, "PublishStart", mock.Anything, mock.Anything)
}

func TestCreateDelivery_InactiveStrategy(t *testing.T) {
	ctx := context.Background()

	clientID := uuid.New()
	strategyID := uuid.New()

	strategy := &domain.DeliveryStrategy{
		ID:     strategyID,
		Name:   "inactive",
		Active: false,
	}

	deliveries := &mocks.MockDeliveryRepository{}
	attempts := &mocks.MockAttemptRepository{}
	strategies := &mocks.MockStrategyRepository{}
	channels := &mocks.MockChannelRepository{}
	producer := &mocks.MockCascadeProducer{}

	strategies.On("Get", ctx, strategyID).Return(strategy, nil)

	svc := newTestService(deliveries, attempts, strategies, channels, producer, nil)

	delivery, err := svc.CreateDelivery(ctx, clientID, strategyID, "79001234567", "text", "sender", "req-003", nil)

	require.Error(t, err)
	assert.Nil(t, delivery)
	assert.Contains(t, err.Error(), "not active")

	strategies.AssertExpectations(t)
	deliveries.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ─── StartCascade ─────────────────────────────────────────────────────────────

func TestStartCascade_HappyPath(t *testing.T) {
	ctx := context.Background()

	deliveryID := uuid.New()
	strategyID := uuid.New()
	channelID := uuid.New()
	clientID := uuid.New()

	// Build a strategy with one SMS step at order 1
	strategy := &domain.DeliveryStrategy{
		ID:     strategyID,
		Name:   "sms-only",
		Mode:   domain.ModeSequential,
		Active: true,
		Steps: []domain.StrategyStep{
			{
				ID:          uuid.New(),
				StrategyID:  strategyID,
				ChannelID:   channelID,
				ChannelType: domain.ChannelSMS,
				ChannelName: "sms",
				StepOrder:   1,
				TimeoutS:    30,
				Billable:    true,
			},
		},
	}

	channelCfg := &domain.ChannelConfig{
		ID:          channelID,
		ChannelType: domain.ChannelSMS,
		Name:        "SMS Provider",
		Active:      true,
		Config:      map[string]interface{}{},
	}

	// delivery starts as pending
	delivery := &domain.Delivery{
		ID:         deliveryID,
		ClientID:   clientID,
		StrategyID: strategyID,
		Recipient:  "79001234567",
		Text:       "Hello",
		SenderName: "Sender",
		Status:     domain.DeliveryPending,
		Currency:   "RUB",
		RequestID:  "req-100",
	}

	deliveries := &mocks.MockDeliveryRepository{}
	attempts := &mocks.MockAttemptRepository{}
	strategies := &mocks.MockStrategyRepository{}
	channels := &mocks.MockChannelRepository{}
	producer := &mocks.MockCascadeProducer{}

	// StartCascade: get delivery
	deliveries.On("Get", ctx, deliveryID).Return(delivery, nil)
	// get strategy
	strategies.On("Get", ctx, strategyID).Return(strategy, nil)
	// update delivery status to in_progress (CAS: Pending → InProgress)
	deliveries.On("UpdateStatusCAS", ctx, deliveryID, domain.DeliveryPending, domain.DeliveryInProgress, "").Return(nil)
	// executeNextStep: get channel config
	channels.On("Get", ctx, channelID).Return(channelCfg, nil)
	// create attempt
	attempts.On("Create", ctx, mock.AnythingOfType("*domain.DeliveryAttempt")).Return(nil)
	// update delivery step
	deliveries.On("UpdateStep", ctx, deliveryID, 1).Return(nil)

	// Mock channel adapter for SMS
	smsAdapter := &mockChannel{channelType: domain.ChannelSMS}
	smsAdapter.On("Send", mock.Anything, mock.AnythingOfType("*domain.DeliveryAttempt"), channelCfg).Return(nil)

	adapters := map[domain.ChannelType]domain.Channel{
		domain.ChannelSMS: smsAdapter,
	}

	svc := newTestService(deliveries, attempts, strategies, channels, producer, adapters)

	evt := cascadekafka.CascadeStartEvent{
		DeliveryID: deliveryID.String(),
		ClientID:   clientID.String(),
		StrategyID: strategyID.String(),
		Recipient:  "79001234567",
		Text:       "Hello",
		SenderName: "Sender",
		RequestID:  "req-100",
	}

	err := svc.StartCascade(ctx, evt)

	require.NoError(t, err)

	deliveries.AssertExpectations(t)
	strategies.AssertExpectations(t)
	channels.AssertExpectations(t)
	attempts.AssertExpectations(t)
	smsAdapter.AssertExpectations(t)
}

func TestStartCascade_IdempotentAlreadyInProgress(t *testing.T) {
	ctx := context.Background()

	deliveryID := uuid.New()

	// delivery is already in_progress — CAS returns ErrDeliveryConflict → no-op
	delivery := &domain.Delivery{
		ID:     deliveryID,
		Status: domain.DeliveryInProgress,
	}

	deliveries := &mocks.MockDeliveryRepository{}
	attempts := &mocks.MockAttemptRepository{}
	strategies := &mocks.MockStrategyRepository{}
	channels := &mocks.MockChannelRepository{}
	producer := &mocks.MockCascadeProducer{}

	deliveries.On("Get", ctx, deliveryID).Return(delivery, nil)
	deliveries.On("UpdateStatusCAS", ctx, deliveryID, domain.DeliveryPending, domain.DeliveryInProgress, "").Return(domain.ErrDeliveryConflict)

	svc := newTestService(deliveries, attempts, strategies, channels, producer, nil)

	evt := cascadekafka.CascadeStartEvent{
		DeliveryID: deliveryID.String(),
	}

	err := svc.StartCascade(ctx, evt)

	require.NoError(t, err)

	deliveries.AssertExpectations(t)
	strategies.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
	attempts.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestStartCascade_DeliveryNotFound(t *testing.T) {
	ctx := context.Background()

	deliveryID := uuid.New()

	deliveries := &mocks.MockDeliveryRepository{}
	attempts := &mocks.MockAttemptRepository{}
	strategies := &mocks.MockStrategyRepository{}
	channels := &mocks.MockChannelRepository{}
	producer := &mocks.MockCascadeProducer{}

	deliveries.On("Get", ctx, deliveryID).Return(nil, domain.ErrDeliveryNotFound)

	svc := newTestService(deliveries, attempts, strategies, channels, producer, nil)

	evt := cascadekafka.CascadeStartEvent{
		DeliveryID: deliveryID.String(),
	}

	err := svc.StartCascade(ctx, evt)

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrDeliveryNotFound), "expected wrapped ErrDeliveryNotFound, got: %v", err)

	deliveries.AssertExpectations(t)
}

func TestStartCascade_InvalidDeliveryID(t *testing.T) {
	ctx := context.Background()

	deliveries := &mocks.MockDeliveryRepository{}
	attempts := &mocks.MockAttemptRepository{}
	strategies := &mocks.MockStrategyRepository{}
	channels := &mocks.MockChannelRepository{}
	producer := &mocks.MockCascadeProducer{}

	svc := newTestService(deliveries, attempts, strategies, channels, producer, nil)

	evt := cascadekafka.CascadeStartEvent{
		DeliveryID: "not-a-valid-uuid",
	}

	err := svc.StartCascade(ctx, evt)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse delivery_id")

	deliveries.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
}

// ─── CAS idempotency tests ────────────────────────────────────────────────────

func TestStartCascade_CASConflict(t *testing.T) {
	ctx := context.Background()

	deliveryID := uuid.New()
	strategyID := uuid.New()

	delivery := &domain.Delivery{
		ID:         deliveryID,
		StrategyID: strategyID,
		Status:     domain.DeliveryPending,
	}

	deliveries := &mocks.MockDeliveryRepository{}
	attempts := &mocks.MockAttemptRepository{}
	strategies := &mocks.MockStrategyRepository{}
	channels := &mocks.MockChannelRepository{}
	producer := &mocks.MockCascadeProducer{}

	deliveries.On("Get", ctx, deliveryID).Return(delivery, nil)
	// Simulates a second concurrent consumer winning the CAS race
	deliveries.On("UpdateStatusCAS", ctx, deliveryID, domain.DeliveryPending, domain.DeliveryInProgress, "").Return(domain.ErrDeliveryConflict)

	svc := newTestService(deliveries, attempts, strategies, channels, producer, nil)

	evt := cascadekafka.CascadeStartEvent{DeliveryID: deliveryID.String()}
	err := svc.StartCascade(ctx, evt)

	require.NoError(t, err)
	deliveries.AssertExpectations(t)
	attempts.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	strategies.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
}

func TestStartCascade_AttemptAlreadyExists(t *testing.T) {
	ctx := context.Background()

	deliveryID := uuid.New()
	strategyID := uuid.New()
	channelID := uuid.New()
	clientID := uuid.New()

	strategy := &domain.DeliveryStrategy{
		ID:     strategyID,
		Name:   "sms-only",
		Mode:   domain.ModeSequential,
		Active: true,
		Steps: []domain.StrategyStep{
			{
				ID:          uuid.New(),
				StrategyID:  strategyID,
				ChannelID:   channelID,
				ChannelType: domain.ChannelSMS,
				ChannelName: "sms",
				StepOrder:   1,
				TimeoutS:    30,
				Billable:    true,
			},
		},
	}

	channelCfg := &domain.ChannelConfig{
		ID:          channelID,
		ChannelType: domain.ChannelSMS,
		Name:        "SMS Provider",
		Active:      true,
		Config:      map[string]interface{}{},
	}

	delivery := &domain.Delivery{
		ID:         deliveryID,
		ClientID:   clientID,
		StrategyID: strategyID,
		Recipient:  "79001234567",
		Text:       "Hello",
		SenderName: "Sender",
		Status:     domain.DeliveryPending,
		Currency:   "RUB",
		RequestID:  "req-200",
	}

	deliveries := &mocks.MockDeliveryRepository{}
	attempts := &mocks.MockAttemptRepository{}
	strategies := &mocks.MockStrategyRepository{}
	channels := &mocks.MockChannelRepository{}
	producer := &mocks.MockCascadeProducer{}

	deliveries.On("Get", ctx, deliveryID).Return(delivery, nil)
	deliveries.On("UpdateStatusCAS", ctx, deliveryID, domain.DeliveryPending, domain.DeliveryInProgress, "").Return(nil)
	strategies.On("Get", ctx, strategyID).Return(strategy, nil)
	channels.On("Get", ctx, channelID).Return(channelCfg, nil)
	// Second consumer already inserted this attempt
	attempts.On("Create", ctx, mock.AnythingOfType("*domain.DeliveryAttempt")).Return(domain.ErrAttemptAlreadyExists)

	smsAdapter := &mockChannel{channelType: domain.ChannelSMS}
	adapters := map[domain.ChannelType]domain.Channel{domain.ChannelSMS: smsAdapter}

	svc := newTestService(deliveries, attempts, strategies, channels, producer, adapters)

	evt := cascadekafka.CascadeStartEvent{
		DeliveryID: deliveryID.String(),
		ClientID:   clientID.String(),
		StrategyID: strategyID.String(),
		Recipient:  "79001234567",
	}
	err := svc.StartCascade(ctx, evt)

	require.NoError(t, err)
	deliveries.AssertExpectations(t)
	attempts.AssertExpectations(t)
	// adapter.Send must NOT be called — attempt was not created by us
	smsAdapter.AssertNotCalled(t, "Send", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessAttemptResult_DeliveredCASConflict(t *testing.T) {
	ctx := context.Background()

	deliveryID := uuid.New()
	attemptID := uuid.New()
	strategyID := uuid.New()
	clientID := uuid.New()

	now := time.Now()
	attempt := &domain.DeliveryAttempt{
		ID:          attemptID,
		DeliveryID:  deliveryID,
		ChannelType: "sms",
		StepOrder:   1,
		Status:      domain.AttemptSent,
		SentAt:      &now,
	}

	delivery := &domain.Delivery{
		ID:         deliveryID,
		ClientID:   clientID,
		StrategyID: strategyID,
		Status:     domain.DeliveryInProgress,
		Currency:   "RUB",
	}

	deliveries := &mocks.MockDeliveryRepository{}
	attempts := &mocks.MockAttemptRepository{}
	strategies := &mocks.MockStrategyRepository{}
	channels := &mocks.MockChannelRepository{}
	producer := &mocks.MockCascadeProducer{}

	attempts.On("Get", ctx, attemptID).Return(attempt, nil)
	deliveries.On("Get", ctx, deliveryID).Return(delivery, nil)
	resultAt := mock.MatchedBy(func(t *time.Time) bool { return t != nil })
	attempts.On("UpdateStatus", ctx, attemptID, domain.AttemptDelivered, "prov-ref", "", resultAt).Return(nil)
	// Another consumer already finalized the delivery
	deliveries.On("UpdateStatusCAS", ctx, deliveryID, domain.DeliveryInProgress, domain.DeliveryDelivered, "sms").Return(domain.ErrDeliveryConflict)

	svc := newTestService(deliveries, attempts, strategies, channels, producer, nil)

	evt := cascadekafka.CascadeAttemptResultEvent{
		AttemptID:   attemptID.String(),
		DeliveryID:  deliveryID.String(),
		Status:      "delivered",
		ProviderRef: "prov-ref",
	}
	err := svc.ProcessAttemptResult(ctx, evt)

	require.NoError(t, err)
	deliveries.AssertExpectations(t)
	attempts.AssertExpectations(t)
	// No billing published — delivery finalization was skipped
	producer.AssertNotCalled(t, "PublishBilling", mock.Anything, mock.Anything)
}
