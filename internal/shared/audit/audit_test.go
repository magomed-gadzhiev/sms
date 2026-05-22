package audit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- AuditEvent tests ---

func TestNewAuditEvent(t *testing.T) {
	event := NewAuditEvent("tenant-1", "user-1", ActionLogin, ResourceAuth, "res-1")

	assert.NotEmpty(t, event.EventID, "EventID should be generated")
	assert.Equal(t, "tenant-1", event.TenantID)
	assert.Equal(t, "user-1", event.UserID)
	assert.Equal(t, ActionLogin, event.Action)
	assert.Equal(t, ResourceAuth, event.ResourceType)
	assert.Equal(t, "res-1", event.ResourceID)
	assert.WithinDuration(t, time.Now().UTC(), event.Timestamp, 2*time.Second)
	assert.Nil(t, event.Details)
	assert.Empty(t, event.IPAddress)
}

func TestNewAuditEvent_UniqueIDs(t *testing.T) {
	e1 := NewAuditEvent("t", "u", ActionLogin, ResourceAuth, "r")
	e2 := NewAuditEvent("t", "u", ActionLogin, ResourceAuth, "r")

	assert.NotEqual(t, e1.EventID, e2.EventID, "Each event should have a unique ID")
}

func TestAuditEvent_JSONRoundTrip(t *testing.T) {
	event := NewAuditEvent("tenant-1", "user-1", ActionAPIKeyCreated, ResourceAPIKey, "key-1")
	event.Details = map[string]any{"key_name": "test-key"}
	event.IPAddress = "192.168.1.1"

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded AuditEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.EventID, decoded.EventID)
	assert.Equal(t, event.TenantID, decoded.TenantID)
	assert.Equal(t, event.UserID, decoded.UserID)
	assert.Equal(t, event.Action, decoded.Action)
	assert.Equal(t, event.ResourceType, decoded.ResourceType)
	assert.Equal(t, event.ResourceID, decoded.ResourceID)
	assert.Equal(t, event.IPAddress, decoded.IPAddress)
	assert.Equal(t, "test-key", decoded.Details["key_name"])
}

func TestAuditEvent_JSONOmitsEmpty(t *testing.T) {
	event := &AuditEvent{
		EventID:      "id-1",
		TenantID:     "t-1",
		Action:       ActionLogout,
		ResourceType: ResourceAuth,
		Timestamp:    time.Now().UTC(),
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	// Fields with omitempty should be absent when empty
	_, hasUserID := raw["user_id"]
	assert.False(t, hasUserID, "user_id should be omitted when empty")
	_, hasResourceID := raw["resource_id"]
	assert.False(t, hasResourceID, "resource_id should be omitted when empty")
	_, hasDetails := raw["details"]
	assert.False(t, hasDetails, "details should be omitted when nil")
	_, hasIPAddress := raw["ip_address"]
	assert.False(t, hasIPAddress, "ip_address should be omitted when empty")
}

func TestAuditActionConstants(t *testing.T) {
	actions := []string{
		ActionLogin, ActionLogout, ActionRegister, ActionPasswordReset,
		ActionTOTPEnabled, ActionTOTPDisabled,
		ActionAPIKeyCreated, ActionAPIKeyRevoked,
		ActionWebhookCreated, ActionWebhookUpdated, ActionWebhookDeleted, ActionWebhookTestSent,
		ActionSubAccountCreated, ActionSubAccountDeleted, ActionSubAccountLimitUpdated,
		ActionBalanceTransferOut, ActionBalanceTransferIn,
		ActionProfileUpdated,
	}

	seen := make(map[string]bool)
	for _, a := range actions {
		assert.NotEmpty(t, a)
		assert.False(t, seen[a], "duplicate action constant: %s", a)
		seen[a] = true
	}
}

func TestAuditResourceConstants(t *testing.T) {
	resources := []string{
		ResourceAuth, ResourceAPIKey, ResourceWebhook,
		ResourceSubAccount, ResourceBalance, ResourceProfile,
	}

	seen := make(map[string]bool)
	for _, r := range resources {
		assert.NotEmpty(t, r)
		assert.False(t, seen[r], "duplicate resource constant: %s", r)
		seen[r] = true
	}
}

// --- Publisher tests ---

func TestNewPublisher_DefaultTopic(t *testing.T) {
	producer := mocks.NewSyncProducer(t, nil)
	logger := zerolog.Nop()

	pub := NewPublisher(producer, "", logger)
	assert.Equal(t, defaultTopic, pub.topic)
}

func TestNewPublisher_CustomTopic(t *testing.T) {
	producer := mocks.NewSyncProducer(t, nil)
	logger := zerolog.Nop()

	pub := NewPublisher(producer, "custom.topic", logger)
	assert.Equal(t, "custom.topic", pub.topic)
}

func TestPublisher_Publish_Success(t *testing.T) {
	producer := mocks.NewSyncProducer(t, nil)
	producer.ExpectSendMessageAndSucceed()

	logger := zerolog.Nop()
	pub := NewPublisher(producer, "test.topic", logger)

	event := NewAuditEvent("tenant-1", "user-1", ActionLogin, ResourceAuth, "")

	err := pub.Publish(context.Background(), event)
	assert.NoError(t, err)
}

func TestPublisher_Publish_ProducerError(t *testing.T) {
	producer := mocks.NewSyncProducer(t, nil)
	producer.ExpectSendMessageWithCheckerFunctionAndFail(nil, errors.New("kafka down"))

	logger := zerolog.Nop()
	pub := NewPublisher(producer, "test.topic", logger)

	event := NewAuditEvent("tenant-1", "user-1", ActionLogin, ResourceAuth, "")

	err := pub.Publish(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "audit: publish event")
}

func TestPublisher_Publish_MessageFormat(t *testing.T) {
	producer := mocks.NewSyncProducer(t, nil)
	producer.ExpectSendMessageWithCheckerFunctionAndSucceed(func(val []byte) error {
		var event AuditEvent
		if err := json.Unmarshal(val, &event); err != nil {
			return err
		}
		if event.TenantID != "tenant-1" {
			return errors.New("wrong tenant_id")
		}
		if event.Action != ActionAPIKeyCreated {
			return errors.New("wrong action")
		}
		return nil
	})

	logger := zerolog.Nop()
	pub := NewPublisher(producer, "test.topic", logger)

	event := NewAuditEvent("tenant-1", "user-1", ActionAPIKeyCreated, ResourceAPIKey, "key-1")

	err := pub.Publish(context.Background(), event)
	assert.NoError(t, err)
}

// TestPublisher_Publish_NilProducerInjected mirrors cmd/portal-gateway/main.go:
// if Kafka is unhealthy at startup, kafkaProducer is nil but the bootstrap
// continues. NewPublisher still returns a non-nil *Publisher with producer=nil.
// Publish must not panic in that state — that's the production scenario this
// fix exists for. Removing this test re-opens BUG-1.
func TestPublisher_Publish_NilProducerInjected(t *testing.T) {
	logger := zerolog.Nop()
	pub := NewPublisher(nil, "test.topic", logger)

	event := NewAuditEvent("tenant-1", "user-1", ActionLogin, ResourceAuth, "")

	require.NotPanics(t, func() {
		err := pub.Publish(context.Background(), event)
		assert.NoError(t, err, "Publish with nil producer should be a no-op, not an error")
	})
}

func TestPublisher_Publish_NilReceiver(t *testing.T) {
	var pub *Publisher
	event := NewAuditEvent("t", "u", ActionLogin, ResourceAuth, "")

	require.NotPanics(t, func() {
		err := pub.Publish(context.Background(), event)
		assert.NoError(t, err, "Publish on a nil receiver should be a no-op, not panic")
	})
}

func TestPublisher_Close_NilProducer(t *testing.T) {
	pub := NewPublisher(nil, "test.topic", zerolog.Nop())
	require.NotPanics(t, func() {
		assert.NoError(t, pub.Close())
	})
}

func TestPublisher_Close_NilReceiver(t *testing.T) {
	var pub *Publisher
	require.NotPanics(t, func() {
		assert.NoError(t, pub.Close())
	})
}

func TestPublisher_Close(t *testing.T) {
	producer := mocks.NewSyncProducer(t, nil)
	logger := zerolog.Nop()
	pub := NewPublisher(producer, "", logger)

	err := pub.Close()
	assert.NoError(t, err)
}

// --- Consumer unit tests (buffer logic, no real Kafka) ---

func TestConsumer_FlushBuffer_EmptyBuffer(t *testing.T) {
	c := &Consumer{
		buffer: make([]*AuditEvent, 0),
		logger: zerolog.Nop(),
	}

	err := c.flushBuffer(context.Background())
	assert.NoError(t, err, "flushing empty buffer should be a no-op")
}

func TestConsumer_Setup(t *testing.T) {
	c := &Consumer{
		logger: zerolog.Nop(),
	}
	err := c.Setup(nil)
	assert.NoError(t, err)
}

func TestConsumer_Cleanup_EmptyBuffer(t *testing.T) {
	c := &Consumer{
		buffer: make([]*AuditEvent, 0),
		logger: zerolog.Nop(),
	}
	err := c.Cleanup(nil)
	assert.NoError(t, err)
}

func TestConsumer_Constants(t *testing.T) {
	assert.Equal(t, 100, defaultBatchSize)
	assert.Equal(t, 5*time.Second, defaultFlushInterval)
	assert.Equal(t, "audit-consumer-group", defaultGroupID)
	assert.Equal(t, "audit.events", defaultTopic)
}

// --- Consumer message checker via sarama mock ---

func TestPublisher_Publish_KeyIsEventID(t *testing.T) {
	producer := mocks.NewSyncProducer(t, nil)

	var capturedMsg *sarama.ProducerMessage
	producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
		capturedMsg = msg
		return nil
	})

	logger := zerolog.Nop()
	pub := NewPublisher(producer, "test.topic", logger)

	event := NewAuditEvent("t", "u", ActionLogin, ResourceAuth, "r")
	err := pub.Publish(context.Background(), event)
	require.NoError(t, err)

	key, err := capturedMsg.Key.Encode()
	require.NoError(t, err)
	assert.Equal(t, event.EventID, string(key))
	assert.Equal(t, "test.topic", capturedMsg.Topic)
}
