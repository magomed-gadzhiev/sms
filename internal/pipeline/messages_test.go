package pipeline

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// RoutedMessage
// ---------------------------------------------------------------------------

func TestRoutedMessage_SerializeDeserialize(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()
	fallbackID := uuid.New()
	routeID := uuid.New()

	orig := &RoutedMessage{
		SchemaVersion:      1,
		MessageID:          uuid.New(),
		Source:             "MyApp",
		Destination:        "+79001234567",
		Text:               "Hello, world!",
		ClientID:           &clientID,
		ProviderID:         uuid.New(),
		FallbackProviderID: &fallbackID,
		RouteID:            &routeID,
		Priority:           3,
		RetryCount:         1,
		MaxRetries:         5,
		RoutedAt:           time.Now().Truncate(time.Millisecond),
		CreatedAt:          time.Now().Add(-time.Hour).Truncate(time.Millisecond),
		Metadata:           map[string]interface{}{"key": "value"},
	}

	data, err := orig.Serialize()
	require.NoError(t, err)
	require.NotEmpty(t, data)

	decoded, err := DeserializeRoutedMessage(data)
	require.NoError(t, err)

	assert.Equal(t, orig.SchemaVersion, decoded.SchemaVersion)
	assert.Equal(t, orig.MessageID, decoded.MessageID)
	assert.Equal(t, orig.Source, decoded.Source)
	assert.Equal(t, orig.Destination, decoded.Destination)
	assert.Equal(t, orig.Text, decoded.Text)
	assert.Equal(t, orig.ClientID, decoded.ClientID)
	assert.Equal(t, orig.ProviderID, decoded.ProviderID)
	assert.Equal(t, orig.FallbackProviderID, decoded.FallbackProviderID)
	assert.Equal(t, orig.RouteID, decoded.RouteID)
	assert.Equal(t, orig.Priority, decoded.Priority)
	assert.Equal(t, orig.RetryCount, decoded.RetryCount)
	assert.Equal(t, orig.MaxRetries, decoded.MaxRetries)
	assert.Equal(t, orig.Metadata["key"], decoded.Metadata["key"])
}

func TestRoutedMessage_DeserializeInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := DeserializeRoutedMessage([]byte("not-json"))
	require.Error(t, err)
}

func TestRoutedMessage_WithCountryIDAndChannel_Roundtrip(t *testing.T) {
	t.Parallel()

	// Bug #15: persist stage needs country_id and channel populated at INSERT
	// time, so they must roundtrip through JSON serialization.
	countryID := uuid.New()
	operatorID := uuid.New()

	orig := &RoutedMessage{
		SchemaVersion: 2,
		MessageID:     uuid.New(),
		Source:        "Sender",
		Destination:   "+79001234567",
		Text:          "Hi",
		OperatorID:    &operatorID,
		CountryID:     &countryID,
		Channel:       "sms",
		ProviderID:    uuid.New(),
		RoutedAt:      time.Now().Truncate(time.Millisecond),
		CreatedAt:     time.Now().Truncate(time.Millisecond),
	}

	data, err := orig.Serialize()
	require.NoError(t, err)

	decoded, err := DeserializeRoutedMessage(data)
	require.NoError(t, err)

	require.NotNil(t, decoded.CountryID)
	assert.Equal(t, countryID, *decoded.CountryID)
	require.NotNil(t, decoded.OperatorID)
	assert.Equal(t, operatorID, *decoded.OperatorID)
	assert.Equal(t, "sms", decoded.Channel)
	assert.Equal(t, 2, decoded.SchemaVersion)
}

func TestRoutedMessage_BackwardCompatV1_NoCountryNoChannel(t *testing.T) {
	t.Parallel()

	// Messages published by old router pods (schema_version=1, no country_id,
	// no channel) must still deserialize — persist-stage handles nil fields
	// as NULL columns. Guarantees additive-only schema change.
	v1JSON := `{
		"schema_version": 1,
		"message_id": "11111111-1111-1111-1111-111111111111",
		"source": "S",
		"destination": "+79001234567",
		"text": "hi",
		"provider_id": "22222222-2222-2222-2222-222222222222",
		"routed_at": "2026-04-22T00:00:00Z",
		"created_at": "2026-04-22T00:00:00Z"
	}`

	decoded, err := DeserializeRoutedMessage([]byte(v1JSON))
	require.NoError(t, err)
	assert.Nil(t, decoded.CountryID)
	assert.Empty(t, decoded.Channel)
	assert.Equal(t, 1, decoded.SchemaVersion)
}

func TestRoutedMessage_NilOptionalFields(t *testing.T) {
	t.Parallel()

	orig := &RoutedMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		Source:        "Src",
		Destination:   "+79001234567",
		Text:          "Hi",
		ProviderID:    uuid.New(),
		RoutedAt:      time.Now().Truncate(time.Millisecond),
		CreatedAt:     time.Now().Truncate(time.Millisecond),
	}

	data, err := orig.Serialize()
	require.NoError(t, err)

	decoded, err := DeserializeRoutedMessage(data)
	require.NoError(t, err)

	assert.Nil(t, decoded.ClientID)
	assert.Nil(t, decoded.FallbackProviderID)
	assert.Nil(t, decoded.RouteID)
	assert.Nil(t, decoded.Metadata)
}

func TestRoutedMessage_EmptyText(t *testing.T) {
	t.Parallel()

	orig := &RoutedMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    uuid.New(),
		Text:          "",
	}

	data, err := orig.Serialize()
	require.NoError(t, err)

	decoded, err := DeserializeRoutedMessage(data)
	require.NoError(t, err)
	assert.Empty(t, decoded.Text)
}

// ---------------------------------------------------------------------------
// SentMessage
// ---------------------------------------------------------------------------

func TestSentMessage_SerializeDeserialize(t *testing.T) {
	t.Parallel()

	errMsg := "connection timeout"
	errCode := 42

	orig := &SentMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    uuid.New(),
		SMPPMessageID: "SMPP-12345",
		Status:        "sent",
		ErrorCode:     &errCode,
		ErrorMessage:  &errMsg,
		SentAt:        time.Now().Truncate(time.Millisecond),
		SegmentsCount: 3,
		ConnectionID:  "conn-abc",
	}

	data, err := orig.Serialize()
	require.NoError(t, err)

	decoded, err := DeserializeSentMessage(data)
	require.NoError(t, err)

	assert.Equal(t, orig.SchemaVersion, decoded.SchemaVersion)
	assert.Equal(t, orig.MessageID, decoded.MessageID)
	assert.Equal(t, orig.ProviderID, decoded.ProviderID)
	assert.Equal(t, orig.SMPPMessageID, decoded.SMPPMessageID)
	assert.Equal(t, orig.Status, decoded.Status)
	assert.Equal(t, orig.ErrorCode, decoded.ErrorCode)
	assert.Equal(t, orig.ErrorMessage, decoded.ErrorMessage)
	assert.Equal(t, orig.SegmentsCount, decoded.SegmentsCount)
	assert.Equal(t, orig.ConnectionID, decoded.ConnectionID)
}

func TestSentMessage_DeserializeInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := DeserializeSentMessage([]byte("{invalid"))
	require.Error(t, err)
}

func TestSentMessage_NilOptionalFields(t *testing.T) {
	t.Parallel()

	orig := &SentMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    uuid.New(),
		Status:        "failed",
		SentAt:        time.Now().Truncate(time.Millisecond),
	}

	data, err := orig.Serialize()
	require.NoError(t, err)

	decoded, err := DeserializeSentMessage(data)
	require.NoError(t, err)
	assert.Nil(t, decoded.ErrorCode)
	assert.Nil(t, decoded.ErrorMessage)
}

// ---------------------------------------------------------------------------
// StatusUpdate
// ---------------------------------------------------------------------------

func TestStatusUpdate_SerializeDeserialize(t *testing.T) {
	t.Parallel()

	providerID := uuid.New()
	errCode := 10
	errMsg := "expired"
	submitDate := time.Now().Add(-time.Minute).Truncate(time.Millisecond)
	doneDate := time.Now().Truncate(time.Millisecond)

	orig := &StatusUpdate{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		Status:        "delivered",
		SMPPMessageID: "smpp-xyz",
		ProviderID:    &providerID,
		ErrorCode:     &errCode,
		ErrorMessage:  &errMsg,
		DLRStat:       "DELIVRD",
		SubmitDate:    &submitDate,
		DoneDate:      &doneDate,
		UpdatedAt:     time.Now().Truncate(time.Millisecond),
	}

	data, err := orig.Serialize()
	require.NoError(t, err)

	decoded, err := DeserializeStatusUpdate(data)
	require.NoError(t, err)

	assert.Equal(t, orig.SchemaVersion, decoded.SchemaVersion)
	assert.Equal(t, orig.MessageID, decoded.MessageID)
	assert.Equal(t, orig.Status, decoded.Status)
	assert.Equal(t, orig.SMPPMessageID, decoded.SMPPMessageID)
	assert.Equal(t, orig.ProviderID, decoded.ProviderID)
	assert.Equal(t, orig.ErrorCode, decoded.ErrorCode)
	assert.Equal(t, orig.ErrorMessage, decoded.ErrorMessage)
	assert.Equal(t, orig.DLRStat, decoded.DLRStat)
}

func TestStatusUpdate_DeserializeInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := DeserializeStatusUpdate([]byte(""))
	require.Error(t, err)
}

func TestStatusUpdate_NilOptionalFields(t *testing.T) {
	t.Parallel()

	orig := &StatusUpdate{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		Status:        "sent",
		UpdatedAt:     time.Now().Truncate(time.Millisecond),
	}

	data, err := orig.Serialize()
	require.NoError(t, err)

	decoded, err := DeserializeStatusUpdate(data)
	require.NoError(t, err)

	assert.Nil(t, decoded.ProviderID)
	assert.Nil(t, decoded.ErrorCode)
	assert.Nil(t, decoded.ErrorMessage)
	assert.Nil(t, decoded.SubmitDate)
	assert.Nil(t, decoded.DoneDate)
	assert.Empty(t, decoded.SMPPMessageID)
	assert.Empty(t, decoded.DLRStat)
}

// ---------------------------------------------------------------------------
// JSON schema compliance
// ---------------------------------------------------------------------------

func TestRoutedMessage_JSONFieldNames(t *testing.T) {
	t.Parallel()

	msg := &RoutedMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    uuid.New(),
	}

	data, err := msg.Serialize()
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Contains(t, raw, "schema_version")
	assert.Contains(t, raw, "message_id")
	assert.Contains(t, raw, "provider_id")
	assert.Contains(t, raw, "routed_at")
	assert.Contains(t, raw, "created_at")
}

func TestSentMessage_JSONFieldNames(t *testing.T) {
	t.Parallel()

	msg := &SentMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    uuid.New(),
		Status:        "sent",
	}

	data, err := msg.Serialize()
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Contains(t, raw, "schema_version")
	assert.Contains(t, raw, "message_id")
	assert.Contains(t, raw, "provider_id")
	assert.Contains(t, raw, "smpp_message_id")
	assert.Contains(t, raw, "status")
	assert.Contains(t, raw, "sent_at")
	assert.Contains(t, raw, "segments_count")
	assert.Contains(t, raw, "connection_id")
}

func TestStatusUpdate_JSONFieldNames(t *testing.T) {
	t.Parallel()

	msg := &StatusUpdate{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		Status:        "delivered",
	}

	data, err := msg.Serialize()
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Contains(t, raw, "schema_version")
	assert.Contains(t, raw, "message_id")
	assert.Contains(t, raw, "status")
	assert.Contains(t, raw, "updated_at")
}
