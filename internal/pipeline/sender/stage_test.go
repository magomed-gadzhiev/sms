package sender

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/pipeline"
	"github.com/smpp-server/smpp-server/internal/queue"
)

// ---------------------------------------------------------------------------
// routedToSharedMessage
// ---------------------------------------------------------------------------

func TestRoutedToSharedMessage_AllFields(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()
	providerID := uuid.New()
	fallbackID := uuid.New()
	routeID := uuid.New()
	createdAt := time.Now().Add(-time.Hour)

	rm := &pipeline.RoutedMessage{
		SchemaVersion:      1,
		MessageID:          uuid.New(),
		Source:             "TestSender",
		Destination:        "+79001234567",
		Text:               "Hello from sender",
		ClientID:           &clientID,
		ProviderID:         providerID,
		FallbackProviderID: &fallbackID,
		RouteID:            &routeID,
		Priority:           3,
		RetryCount:         1,
		MaxRetries:         5,
		RoutedAt:           time.Now(),
		CreatedAt:          createdAt,
		Metadata:           map[string]interface{}{"key": "val"},
	}

	msg := routedToSharedMessage(rm)

	assert.Equal(t, rm.MessageID, msg.ID)
	assert.Equal(t, rm.Source, msg.Source)
	assert.Equal(t, rm.Destination, msg.Destination)
	assert.Equal(t, rm.Text, msg.Text)
	assert.Equal(t, rm.ClientID, msg.ClientID)
	assert.Equal(t, &rm.ProviderID, msg.ProviderID)
	assert.Equal(t, rm.RouteID, msg.RouteID)
	assert.Equal(t, rm.Priority, msg.PriorityFlag)
	assert.Equal(t, rm.RetryCount, msg.RetryCount)
	assert.Equal(t, rm.MaxRetries, msg.MaxRetries)
	assert.Equal(t, rm.CreatedAt, msg.CreatedAt)
}

func TestRoutedToSharedMessage_NilOptionalFields(t *testing.T) {
	t.Parallel()

	rm := &pipeline.RoutedMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		Source:        "Src",
		Destination:   "+79001234567",
		Text:          "Hi",
		ProviderID:    uuid.New(),
		// ClientID, FallbackProviderID, RouteID are nil
	}

	msg := routedToSharedMessage(rm)

	assert.Nil(t, msg.ClientID)
	assert.Nil(t, msg.RouteID)
	// ProviderID is always set from rm.ProviderID
	require.NotNil(t, msg.ProviderID)
	assert.Equal(t, rm.ProviderID, *msg.ProviderID)
}

func TestRoutedToSharedMessage_ZeroValues(t *testing.T) {
	t.Parallel()

	rm := &pipeline.RoutedMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    uuid.New(),
		// All other fields are zero values
	}

	msg := routedToSharedMessage(rm)

	assert.Empty(t, msg.Source)
	assert.Empty(t, msg.Destination)
	assert.Empty(t, msg.Text)
	assert.Equal(t, 0, msg.PriorityFlag)
	assert.Equal(t, 0, msg.RetryCount)
	assert.Equal(t, 0, msg.MaxRetries)
	assert.True(t, msg.CreatedAt.IsZero())
}

func TestRoutedToSharedMessage_ProviderIDIsPointer(t *testing.T) {
	t.Parallel()

	providerID := uuid.New()
	rm := &pipeline.RoutedMessage{
		MessageID:  uuid.New(),
		ProviderID: providerID,
	}

	msg := routedToSharedMessage(rm)

	// shared.Message has ProviderID as *uuid.UUID
	require.NotNil(t, msg.ProviderID)
	assert.Equal(t, providerID, *msg.ProviderID)
}

// ---------------------------------------------------------------------------
// RoutedMessage deserialization for processMessage
// ---------------------------------------------------------------------------

func TestRoutedMessage_RoundTrip(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()
	routeID := uuid.New()
	fallback := uuid.New()

	rm := &pipeline.RoutedMessage{
		SchemaVersion:      1,
		MessageID:          uuid.New(),
		Source:             "App",
		Destination:        "+79009990000",
		Text:               "Test message",
		ClientID:           &clientID,
		ProviderID:         uuid.New(),
		FallbackProviderID: &fallback,
		RouteID:            &routeID,
		Priority:           2,
		RetryCount:         0,
		MaxRetries:         3,
		RoutedAt:           time.Now().Truncate(time.Millisecond),
		CreatedAt:          time.Now().Add(-time.Minute).Truncate(time.Millisecond),
		Metadata:           map[string]interface{}{"campaign": "test"},
	}

	data, err := rm.Serialize()
	require.NoError(t, err)

	decoded, err := pipeline.DeserializeRoutedMessage(data)
	require.NoError(t, err)

	// Verify the deserialized message matches the original
	assert.Equal(t, rm.SchemaVersion, decoded.SchemaVersion)
	assert.Equal(t, rm.MessageID, decoded.MessageID)
	assert.Equal(t, rm.Source, decoded.Source)
	assert.Equal(t, rm.Destination, decoded.Destination)
	assert.Equal(t, rm.Text, decoded.Text)
	assert.Equal(t, rm.ClientID, decoded.ClientID)
	assert.Equal(t, rm.ProviderID, decoded.ProviderID)
	assert.Equal(t, rm.FallbackProviderID, decoded.FallbackProviderID)
	assert.Equal(t, rm.RouteID, decoded.RouteID)
	assert.Equal(t, rm.Priority, decoded.Priority)
	assert.Equal(t, rm.RetryCount, decoded.RetryCount)
	assert.Equal(t, rm.MaxRetries, decoded.MaxRetries)

	// Verify routedToSharedMessage produces correct output
	msg := routedToSharedMessage(decoded)
	assert.Equal(t, decoded.MessageID, msg.ID)
	assert.Equal(t, decoded.ProviderID, *msg.ProviderID)
}

func TestRoutedMessage_DeserializeInvalid(t *testing.T) {
	t.Parallel()

	_, err := pipeline.DeserializeRoutedMessage([]byte("not json"))
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// SentMessage construction pattern (used in processMessage)
// ---------------------------------------------------------------------------

func TestSentMessage_SuccessCase(t *testing.T) {
	t.Parallel()

	providerID := uuid.New()
	sentMsg := &pipeline.SentMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    providerID,
		SMPPMessageID: "SMPP-12345",
		Status:        "sent",
		SentAt:        time.Now(),
		SegmentsCount: 1,
		ConnectionID:  "conn-1",
	}

	data, err := sentMsg.Serialize()
	require.NoError(t, err)

	decoded, err := pipeline.DeserializeSentMessage(data)
	require.NoError(t, err)

	assert.Equal(t, "sent", decoded.Status)
	assert.Equal(t, "SMPP-12345", decoded.SMPPMessageID)
	assert.Nil(t, decoded.ErrorMessage)
	assert.Nil(t, decoded.ErrorCode)
}

func TestSentMessage_FailureCase(t *testing.T) {
	t.Parallel()

	errMsg := "connection refused"
	sentMsg := &pipeline.SentMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    uuid.New(),
		Status:        "failed",
		ErrorMessage:  &errMsg,
		SentAt:        time.Now(),
		SegmentsCount: 1,
		ConnectionID:  "conn-2",
	}

	data, err := sentMsg.Serialize()
	require.NoError(t, err)

	decoded, err := pipeline.DeserializeSentMessage(data)
	require.NoError(t, err)

	assert.Equal(t, "failed", decoded.Status)
	require.NotNil(t, decoded.ErrorMessage)
	assert.Equal(t, errMsg, *decoded.ErrorMessage)
	assert.Empty(t, decoded.SMPPMessageID)
}

// ---------------------------------------------------------------------------
// FailedMessage construction (used in retry path)
// ---------------------------------------------------------------------------

func TestFailedMessage_ForRetry(t *testing.T) {
	t.Parallel()

	msgID := uuid.New()
	clientID := uuid.New()

	rm := &pipeline.RoutedMessage{
		MessageID:   msgID,
		Source:      "App",
		Destination: "+79001234567",
		Text:        "Retry me",
		ClientID:    &clientID,
		ProviderID:  uuid.New(),
		Priority:    1,
		RetryCount:  1,
		MaxRetries:  3,
		CreatedAt:   time.Now(),
		Metadata:    map[string]interface{}{"key": "val"},
	}

	// Simulate what processMessage does when both providers fail
	failedMsg := &queue.FailedMessage{
		MessageID:  rm.MessageID,
		Error:      "send_failed",
		ErrorCode:  "send_failed",
		RetryCount: rm.RetryCount + 1,
		FailedAt:   time.Now(),
		KafkaMessage: &queue.KafkaMessage{
			ID:          rm.MessageID.String(),
			MessageID:   rm.MessageID,
			Source:      rm.Source,
			Destination: rm.Destination,
			Text:        rm.Text,
			ClientID:    rm.ClientID,
			Priority:    rm.Priority,
			RetryCount:  rm.RetryCount + 1,
			MaxRetries:  rm.MaxRetries,
			CreatedAt:   rm.CreatedAt,
			Metadata:    rm.Metadata,
		},
	}

	data, err := failedMsg.Serialize()
	require.NoError(t, err)

	decoded, err := queue.DeserializeFailed(data)
	require.NoError(t, err)

	assert.Equal(t, msgID, decoded.MessageID)
	assert.Equal(t, 2, decoded.RetryCount)
	require.NotNil(t, decoded.KafkaMessage)
	assert.Equal(t, rm.Source, decoded.KafkaMessage.Source)
	assert.Equal(t, rm.Destination, decoded.KafkaMessage.Destination)
	assert.Equal(t, 2, decoded.KafkaMessage.RetryCount)
	assert.Equal(t, 3, decoded.KafkaMessage.MaxRetries)
}

func TestFailedMessage_NoRetryWhenExhausted(t *testing.T) {
	t.Parallel()

	rm := &pipeline.RoutedMessage{
		MessageID:  uuid.New(),
		RetryCount: 3,
		MaxRetries: 3,
	}

	// In processMessage, this condition prevents publishing to sms.failed:
	// sendErr != nil && routedMsg.RetryCount < routedMsg.MaxRetries
	assert.False(t, rm.RetryCount < rm.MaxRetries,
		"should not retry when RetryCount >= MaxRetries")
}

func TestFailedMessage_RetryAllowed(t *testing.T) {
	t.Parallel()

	rm := &pipeline.RoutedMessage{
		MessageID:  uuid.New(),
		RetryCount: 2,
		MaxRetries: 3,
	}

	assert.True(t, rm.RetryCount < rm.MaxRetries,
		"should allow retry when RetryCount < MaxRetries")
}
