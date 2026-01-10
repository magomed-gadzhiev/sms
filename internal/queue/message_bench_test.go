//go:build !integration

package queue_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/testutil"
)

func BenchmarkSerializeKafkaMessage(b *testing.B) {
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = msg.Serialize()
	}
}

func BenchmarkDeserializeKafkaMessage(b *testing.B) {
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

	data, _ := msg.Serialize()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = queue.Deserialize(data)
	}
}

func BenchmarkFromMessage(b *testing.B) {
	msg := testutil.NewTestMessage()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = queue.FromMessage(msg)
	}
}

func BenchmarkToMessage(b *testing.B) {
	km := &queue.KafkaMessage{
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = km.ToMessage()
	}
}

func BenchmarkDLRMessage_Serialize(b *testing.B) {
	messageID := uuid.New()
	providerID := uuid.New()
	now := time.Now()

	dlr := &queue.DLRMessage{
		MessageID:           messageID,
		SMPPMessageID:       "smpp-123",
		ProviderID:          &providerID,
		ReceiptedMessageID:  "receipt-123",
		SubmitDate:          &now,
		DoneDate:            &now,
		Stat:                "DELIVRD",
		CreatedAt:           now,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dlr.Serialize()
	}
}

func BenchmarkDeserializeDLR(b *testing.B) {
	messageID := uuid.New()
	providerID := uuid.New()
	now := time.Now()

	dlr := &queue.DLRMessage{
		MessageID:           messageID,
		SMPPMessageID:       "smpp-123",
		ProviderID:          &providerID,
		ReceiptedMessageID:  "receipt-123",
		SubmitDate:          &now,
		DoneDate:            &now,
		Stat:                "DELIVRD",
		CreatedAt:           now,
	}

	data, _ := dlr.Serialize()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = queue.DeserializeDLR(data)
	}
}
