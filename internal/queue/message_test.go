package queue_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/testutil"
)

func TestKafkaMessage(t *testing.T) {
	t.Run("Serialize", func(t *testing.T) {
		t.Run("serializes and deserializes correctly", func(t *testing.T) {
			msg := &queue.KafkaMessage{
				ID:          "test-id",
				MessageID:   uuid.New(),
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Test message",
				Priority:    0,
				RetryCount:  0,
				MaxRetries:  5,
				CreatedAt:   time.Now(),
			}

			data, err := msg.Serialize()

			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Проверяем, что можно десериализовать обратно
			var decoded queue.KafkaMessage
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, msg.ID, decoded.ID)
			assert.Equal(t, msg.MessageID, decoded.MessageID)
			assert.Equal(t, msg.Source, decoded.Source)
			assert.Equal(t, msg.Destination, decoded.Destination)
			assert.Equal(t, msg.Text, decoded.Text)
		})
	})

	t.Run("ToMessage", func(t *testing.T) {
		t.Run("converts to shared.Message with all fields", func(t *testing.T) {
			providerID := uuid.New()
			routeID := uuid.New()
			clientID := uuid.New()

			km := &queue.KafkaMessage{
				ID:          "test-id",
				MessageID:   uuid.New(),
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Test message",
				ProviderID:  &providerID,
				RouteID:     &routeID,
				ClientID:    &clientID,
				Priority:    1,
				RetryCount:  2,
				MaxRetries:  5,
				CreatedAt:   time.Now(),
			}

			msg := km.ToMessage()

			assert.Equal(t, km.MessageID, msg.ID)
			assert.Equal(t, km.Source, msg.Source)
			assert.Equal(t, km.Destination, msg.Destination)
			assert.Equal(t, km.Text, msg.Text)
			assert.Equal(t, km.ProviderID, msg.ProviderID)
			assert.Equal(t, km.RouteID, msg.RouteID)
			assert.Equal(t, km.ClientID, msg.ClientID)
			assert.Equal(t, km.Priority, msg.PriorityFlag)
			assert.Equal(t, km.RetryCount, msg.RetryCount)
			assert.Equal(t, km.MaxRetries, msg.MaxRetries)
			assert.Equal(t, shared.MessageStatusQueued, msg.Status)
		})

		t.Run("sets default max retries when not specified", func(t *testing.T) {
			km := &queue.KafkaMessage{
				ID:          "test-id",
				MessageID:   uuid.New(),
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Test message",
				MaxRetries:  0, // не установлено
				CreatedAt:   time.Now(),
			}

			msg := km.ToMessage()

			assert.Equal(t, 5, msg.MaxRetries) // должно быть установлено значение по умолчанию
		})
	})
}

func TestFromMessage(t *testing.T) {
	t.Run("converts from shared.Message with all fields", func(t *testing.T) {
		msg := testutil.NewTestMessage()
		providerID := uuid.New()
		routeID := uuid.New()
		clientID := uuid.New()

		msg.ProviderID = &providerID
		msg.RouteID = &routeID
		msg.ClientID = &clientID
		msg.PriorityFlag = 1
		msg.RetryCount = 2
		msg.MaxRetries = 5

		km := queue.FromMessage(msg)

		assert.Equal(t, msg.ID.String(), km.ID)
		assert.Equal(t, msg.ID, km.MessageID)
		assert.Equal(t, msg.Source, km.Source)
		assert.Equal(t, msg.Destination, km.Destination)
		assert.Equal(t, msg.Text, km.Text)
		assert.Equal(t, msg.ProviderID, km.ProviderID)
		assert.Equal(t, msg.RouteID, km.RouteID)
		assert.Equal(t, msg.ClientID, km.ClientID)
		assert.Equal(t, msg.PriorityFlag, km.Priority)
		assert.Equal(t, msg.RetryCount, km.RetryCount)
		assert.Equal(t, msg.MaxRetries, km.MaxRetries)
	})
}

func TestDeserialize(t *testing.T) {
	t.Run("deserializes valid JSON", func(t *testing.T) {
		original := &queue.KafkaMessage{
			ID:          "test-id",
			MessageID:   uuid.New(),
			Source:      "12345",
			Destination: "79001234567",
			Text:        "Test message",
			Priority:    0,
			RetryCount:  0,
			MaxRetries:  5,
			CreatedAt:   time.Now(),
		}

		data, err := original.Serialize()
		require.NoError(t, err)

		decoded, err := queue.Deserialize(data)

		require.NoError(t, err)
		assert.Equal(t, original.ID, decoded.ID)
		assert.Equal(t, original.MessageID, decoded.MessageID)
		assert.Equal(t, original.Source, decoded.Source)
		assert.Equal(t, original.Destination, decoded.Destination)
		assert.Equal(t, original.Text, decoded.Text)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		invalidData := []byte("{invalid json}")

		_, err := queue.Deserialize(invalidData)

		assert.Error(t, err)
	})
}

func TestDLRMessage(t *testing.T) {
	t.Run("Serialize", func(t *testing.T) {
		t.Run("serializes and deserializes correctly", func(t *testing.T) {
			messageID := uuid.New()
			providerID := uuid.New()
			now := time.Now()

			dlr := &queue.DLRMessage{
				MessageID:          messageID,
				SMPPMessageID:      "smpp-123",
				ProviderID:         &providerID,
				ReceiptedMessageID: "receipt-123",
				SubmitDate:         &now,
				DoneDate:           &now,
				Stat:               "DELIVRD",
				Err:                nil,
				Text:               "",
				Source:             "79001234567",
				Destination:        "12345",
				CreatedAt:          now,
			}

			data, err := dlr.Serialize()

			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Проверяем десериализацию
			decoded, err := queue.DeserializeDLR(data)
			require.NoError(t, err)
			assert.Equal(t, dlr.MessageID, decoded.MessageID)
			assert.Equal(t, dlr.SMPPMessageID, decoded.SMPPMessageID)
			assert.Equal(t, dlr.Stat, decoded.Stat)
		})
	})

	t.Run("Deserialize", func(t *testing.T) {
		t.Run("deserializes valid DLR", func(t *testing.T) {
			original := &queue.DLRMessage{
				MessageID:     uuid.New(),
				SMPPMessageID: "smpp-123",
				Stat:          "DELIVRD",
				CreatedAt:     time.Now(),
			}

			data, err := original.Serialize()
			require.NoError(t, err)

			decoded, err := queue.DeserializeDLR(data)

			require.NoError(t, err)
			assert.Equal(t, original.MessageID, decoded.MessageID)
			assert.Equal(t, original.SMPPMessageID, decoded.SMPPMessageID)
			assert.Equal(t, original.Stat, decoded.Stat)
		})

		t.Run("returns error for invalid JSON", func(t *testing.T) {
			invalidData := []byte("{invalid json}")

			_, err := queue.DeserializeDLR(invalidData)

			assert.Error(t, err)
		})
	})
}

func TestFailedMessage(t *testing.T) {
	t.Run("Serialize", func(t *testing.T) {
		t.Run("serializes and deserializes correctly", func(t *testing.T) {
			messageID := uuid.New()
			kafkaMsg := &queue.KafkaMessage{
				ID:          "test-id",
				MessageID:   messageID,
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Test message",
			}

			failed := &queue.FailedMessage{
				MessageID:    messageID,
				KafkaMessage: kafkaMsg,
				Error:        "Test error",
				ErrorCode:    "TEST_ERROR",
				RetryCount:   3,
				FailedAt:     time.Now(),
			}

			data, err := failed.Serialize()

			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Проверяем десериализацию
			decoded, err := queue.DeserializeFailed(data)
			require.NoError(t, err)
			assert.Equal(t, failed.MessageID, decoded.MessageID)
			assert.Equal(t, failed.Error, decoded.Error)
			assert.Equal(t, failed.ErrorCode, decoded.ErrorCode)
		})
	})

	t.Run("Deserialize", func(t *testing.T) {
		t.Run("deserializes valid failed message", func(t *testing.T) {
			original := &queue.FailedMessage{
				MessageID:  uuid.New(),
				Error:      "Test error",
				ErrorCode:  "TEST_ERROR",
				RetryCount: 3,
				FailedAt:   time.Now(),
			}

			data, err := original.Serialize()
			require.NoError(t, err)

			decoded, err := queue.DeserializeFailed(data)

			require.NoError(t, err)
			assert.Equal(t, original.MessageID, decoded.MessageID)
			assert.Equal(t, original.Error, decoded.Error)
			assert.Equal(t, original.ErrorCode, decoded.ErrorCode)
		})

		t.Run("returns error for invalid JSON", func(t *testing.T) {
			invalidData := []byte("{invalid json}")

			_, err := queue.DeserializeFailed(invalidData)

			assert.Error(t, err)
		})
	})
}
