package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessage(t *testing.T) {
	t.Run("creates message with correct defaults", func(t *testing.T) {
		clientID := uuid.New()
		msg := NewMessage(clientID, "SenderName", "+79001234567", "Hello")

		assert.NotEqual(t, uuid.Nil, msg.ID)
		assert.Equal(t, "SenderName", msg.Source)
		assert.Equal(t, "+79001234567", msg.Destination)
		assert.Equal(t, "Hello", msg.Text)
		assert.Equal(t, shared.MessageStatusPending, msg.Status)
		require.NotNil(t, msg.ClientID)
		assert.Equal(t, clientID, *msg.ClientID)
		assert.Equal(t, 5, msg.MaxRetries)
		assert.Equal(t, 1, msg.RegisteredDelivery)
		assert.False(t, msg.CreatedAt.IsZero())
		assert.False(t, msg.UpdatedAt.IsZero())
	})
}

func TestMessage_MarkAsQueued(t *testing.T) {
	t.Run("from PENDING sets status to QUEUED", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		require.Equal(t, shared.MessageStatusPending, msg.Status)

		before := msg.UpdatedAt
		time.Sleep(time.Millisecond)
		msg.MarkAsQueued()

		assert.Equal(t, shared.MessageStatusQueued, msg.Status)
		assert.True(t, msg.UpdatedAt.After(before))
	})

	t.Run("from any status unconditionally sets QUEUED", func(t *testing.T) {
		// The domain method does not enforce state transitions — it sets status directly
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		msg.Status = shared.MessageStatusDelivered

		msg.MarkAsQueued()

		assert.Equal(t, shared.MessageStatusQueued, msg.Status)
	})
}

func TestMessage_MarkAsSent(t *testing.T) {
	t.Run("sets status to SENT and records SMPP message ID", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		msg.MarkAsQueued()

		before := msg.UpdatedAt
		time.Sleep(time.Millisecond)
		msg.MarkAsSent("smpp-12345")

		assert.Equal(t, shared.MessageStatusSent, msg.Status)
		assert.Equal(t, "smpp-12345", msg.SMPPMessageID)
		require.NotNil(t, msg.SubmittedAt)
		assert.False(t, msg.SubmittedAt.IsZero())
		assert.True(t, msg.UpdatedAt.After(before))
	})
}

func TestMessage_MarkAsDelivered(t *testing.T) {
	t.Run("sets status to DELIVERED and records DeliveredAt", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		msg.MarkAsSent("smpp-1")

		before := msg.UpdatedAt
		time.Sleep(time.Millisecond)
		msg.MarkAsDelivered()

		assert.Equal(t, shared.MessageStatusDelivered, msg.Status)
		require.NotNil(t, msg.DeliveredAt)
		assert.False(t, msg.DeliveredAt.IsZero())
		assert.True(t, msg.UpdatedAt.After(before))
	})
}

func TestMessage_MarkAsFailed(t *testing.T) {
	t.Run("from PENDING sets status to FAILED with reason", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		require.Equal(t, shared.MessageStatusPending, msg.Status)

		msg.MarkAsFailed("provider timeout")

		assert.Equal(t, shared.MessageStatusFailed, msg.Status)
		assert.Equal(t, "provider timeout", msg.StatusMessage)
		require.NotNil(t, msg.FailedAt)
		assert.False(t, msg.FailedAt.IsZero())
	})

	t.Run("from any status unconditionally sets FAILED", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		msg.Status = shared.MessageStatusDelivered

		msg.MarkAsFailed("late failure")

		assert.Equal(t, shared.MessageStatusFailed, msg.Status)
		assert.Equal(t, "late failure", msg.StatusMessage)
	})
}

func TestMessage_MarkAsExpired(t *testing.T) {
	t.Run("sets status to EXPIRED and records ExpiredAt", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		msg.MarkAsSent("smpp-1")

		msg.MarkAsExpired()

		assert.Equal(t, shared.MessageStatusExpired, msg.Status)
		require.NotNil(t, msg.ExpiredAt)
		assert.False(t, msg.ExpiredAt.IsZero())
	})
}

func TestMessage_MarkAsRejected(t *testing.T) {
	t.Run("sets status to REJECTED with reason", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")

		msg.MarkAsRejected("blacklisted number")

		assert.Equal(t, shared.MessageStatusRejected, msg.Status)
		assert.Equal(t, "blacklisted number", msg.StatusMessage)
		require.NotNil(t, msg.FailedAt)
	})
}

func TestMessage_MarkAsScheduled(t *testing.T) {
	t.Run("sets status to SCHEDULED with scheduled time", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		scheduledTime := time.Now().Add(1 * time.Hour)

		msg.MarkAsScheduled(scheduledTime)

		assert.Equal(t, shared.MessageStatusScheduled, msg.Status)
		require.NotNil(t, msg.ScheduledAt)
		assert.Equal(t, scheduledTime, *msg.ScheduledAt)
	})
}

func TestMessage_MarkAsCancelled(t *testing.T) {
	t.Run("from SCHEDULED sets status to CANCELLED", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		msg.MarkAsScheduled(time.Now().Add(1 * time.Hour))

		before := msg.UpdatedAt
		time.Sleep(time.Millisecond)
		msg.MarkAsCancelled()

		assert.Equal(t, shared.MessageStatusCancelled, msg.Status)
		assert.True(t, msg.UpdatedAt.After(before))
	})

	t.Run("from SENT unconditionally sets CANCELLED", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		msg.MarkAsSent("smpp-1")

		msg.MarkAsCancelled()

		assert.Equal(t, shared.MessageStatusCancelled, msg.Status)
	})
}

func TestMessage_IncrementRetry(t *testing.T) {
	t.Run("increments retry count and sets next retry time", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		assert.Equal(t, 0, msg.RetryCount)

		nextRetry := time.Now().Add(30 * time.Second)
		msg.IncrementRetry(nextRetry)

		assert.Equal(t, 1, msg.RetryCount)
		require.NotNil(t, msg.NextRetryAt)
		assert.Equal(t, nextRetry, *msg.NextRetryAt)
	})

	t.Run("increments multiple times", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")

		for i := 0; i < 3; i++ {
			msg.IncrementRetry(time.Now().Add(time.Duration(i+1) * time.Minute))
		}

		assert.Equal(t, 3, msg.RetryCount)
	})
}

func TestMessage_CanRetry(t *testing.T) {
	t.Run("returns true when FAILED and retries remaining and next retry is in past", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		msg.MarkAsFailed("timeout")
		pastRetry := time.Now().Add(-1 * time.Second)
		msg.NextRetryAt = &pastRetry

		assert.True(t, msg.CanRetry())
	})

	t.Run("returns true when FAILED and NextRetryAt is nil", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		msg.MarkAsFailed("timeout")
		msg.NextRetryAt = nil

		assert.True(t, msg.CanRetry())
	})

	t.Run("returns false when max retries reached", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		msg.MarkAsFailed("timeout")
		msg.RetryCount = msg.MaxRetries

		assert.False(t, msg.CanRetry())
	})

	t.Run("returns false when status is not FAILED", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		// Status is PENDING
		assert.False(t, msg.CanRetry())
	})

	t.Run("returns false when next retry is in the future", func(t *testing.T) {
		msg := NewMessage(uuid.New(), "Src", "+79001234567", "text")
		msg.MarkAsFailed("timeout")
		futureRetry := time.Now().Add(10 * time.Minute)
		msg.NextRetryAt = &futureRetry

		assert.False(t, msg.CanRetry())
	})
}

func TestMessage_ToShared(t *testing.T) {
	t.Run("converts domain message to shared message", func(t *testing.T) {
		clientID := uuid.New()
		msg := NewMessage(clientID, "Src", "+79001234567", "Hello")
		msg.MessageID = "msg-001"
		msg.ExternalID = "ext-001"

		sharedMsg := msg.ToShared()

		assert.Equal(t, msg.ID, sharedMsg.ID)
		assert.Equal(t, shared.NullString(msg.MessageID), sharedMsg.MessageID)
		assert.Equal(t, shared.NullString(msg.ExternalID), sharedMsg.ExternalID)
		assert.Equal(t, msg.Source, sharedMsg.Source)
		assert.Equal(t, msg.Destination, sharedMsg.Destination)
		assert.Equal(t, msg.Text, sharedMsg.Text)
		assert.Equal(t, msg.Status, sharedMsg.Status)
		assert.Equal(t, msg.MaxRetries, sharedMsg.MaxRetries)
	})
}

func TestMessageFromShared(t *testing.T) {
	t.Run("converts shared message to domain message", func(t *testing.T) {
		clientID := uuid.New()
		sharedMsg := &shared.Message{
			ID:          uuid.New(),
			MessageID:   shared.NullString("msg-002"),
			Source:      "Src",
			Destination: "+79001234567",
			Text:        "Hello",
			Status:      shared.MessageStatusSent,
			ClientID:    &clientID,
			MaxRetries:  3,
		}

		msg := MessageFromShared(sharedMsg)

		assert.Equal(t, sharedMsg.ID, msg.ID)
		assert.Equal(t, string(sharedMsg.MessageID), msg.MessageID)
		assert.Equal(t, sharedMsg.Source, msg.Source)
		assert.Equal(t, sharedMsg.Destination, msg.Destination)
		assert.Equal(t, sharedMsg.Text, msg.Text)
		assert.Equal(t, sharedMsg.Status, msg.Status)
		assert.Equal(t, sharedMsg.MaxRetries, msg.MaxRetries)
	})
}
