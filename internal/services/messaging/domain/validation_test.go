package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/stretchr/testify/assert"
)

func TestMessageValidator_Validate(t *testing.T) {
	newValidMessage := func() *Message {
		now := time.Now()
		return &Message{
			ID:          uuid.New(),
			Source:      "SenderName",
			Destination: "+79001234567",
			Text:        "Hello, world!",
			Status:      shared.MessageStatusPending,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}

	v := NewMessageValidator()

	t.Run("valid message returns nil", func(t *testing.T) {
		msg := newValidMessage()
		err := v.Validate(msg)
		assert.NoError(t, err)
	})

	t.Run("empty source returns error", func(t *testing.T) {
		msg := newValidMessage()
		msg.Source = ""

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source")
	})

	t.Run("source exceeding 20 characters returns error", func(t *testing.T) {
		msg := newValidMessage()
		msg.Source = strings.Repeat("A", 21)

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source")
	})

	t.Run("empty destination returns error", func(t *testing.T) {
		msg := newValidMessage()
		msg.Destination = ""

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "destination")
	})

	t.Run("destination exceeding 20 characters returns error", func(t *testing.T) {
		msg := newValidMessage()
		msg.Destination = "+" + strings.Repeat("1", 20)

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "destination")
	})

	t.Run("destination with invalid characters returns error", func(t *testing.T) {
		msg := newValidMessage()
		msg.Destination = "abc123"

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "destination")
	})

	t.Run("destination with no digits returns error", func(t *testing.T) {
		msg := newValidMessage()
		msg.Destination = "+++"

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "destination")
	})

	t.Run("destination with valid special characters is accepted", func(t *testing.T) {
		msg := newValidMessage()
		msg.Destination = "+7 (900) 123-45"

		err := v.Validate(msg)
		assert.NoError(t, err)
	})

	t.Run("empty text returns error", func(t *testing.T) {
		msg := newValidMessage()
		msg.Text = ""

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "text")
	})

	t.Run("text exceeding 1600 characters returns error", func(t *testing.T) {
		msg := newValidMessage()
		msg.Text = strings.Repeat("A", 1601)

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "text")
	})

	t.Run("text at exactly 1600 characters is accepted", func(t *testing.T) {
		msg := newValidMessage()
		msg.Text = strings.Repeat("A", 1600)

		err := v.Validate(msg)
		assert.NoError(t, err)
	})

	t.Run("invalid priority returns error", func(t *testing.T) {
		msg := newValidMessage()
		msg.PriorityFlag = 4

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "priority")
	})

	t.Run("negative priority returns error", func(t *testing.T) {
		msg := newValidMessage()
		msg.PriorityFlag = -1

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "priority")
	})

	t.Run("valid priorities 0-3 are accepted", func(t *testing.T) {
		for _, p := range []int{0, 1, 2, 3} {
			msg := newValidMessage()
			msg.PriorityFlag = p

			err := v.Validate(msg)
			assert.NoError(t, err, "priority %d should be valid", p)
		}
	})

	t.Run("validity period in the past returns error", func(t *testing.T) {
		msg := newValidMessage()
		pastTime := msg.CreatedAt.Add(-1 * time.Hour)
		msg.ValidityPeriod = &pastTime

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validity_period")
	})

	t.Run("validity period in the future is accepted", func(t *testing.T) {
		msg := newValidMessage()
		futureTime := msg.CreatedAt.Add(1 * time.Hour)
		msg.ValidityPeriod = &futureTime

		err := v.Validate(msg)
		assert.NoError(t, err)
	})

	t.Run("nil validity period is accepted", func(t *testing.T) {
		msg := newValidMessage()
		msg.ValidityPeriod = nil

		err := v.Validate(msg)
		assert.NoError(t, err)
	})

	t.Run("multiple errors are reported together", func(t *testing.T) {
		msg := newValidMessage()
		msg.Source = ""
		msg.Destination = ""
		msg.Text = ""

		err := v.Validate(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source")
		assert.Contains(t, err.Error(), "destination")
		assert.Contains(t, err.Error(), "text")
	})
}

func TestDetectEncoding(t *testing.T) {
	t.Run("ASCII text returns GSM7", func(t *testing.T) {
		enc := DetectEncoding("Hello, world!")
		assert.Equal(t, shared.MessageEncodingGSM7, enc)
	})

	t.Run("text with non-ASCII characters returns UCS2", func(t *testing.T) {
		enc := DetectEncoding("Привет, мир!")
		assert.Equal(t, shared.MessageEncodingUCS2, enc)
	})

	t.Run("empty text returns GSM7", func(t *testing.T) {
		enc := DetectEncoding("")
		assert.Equal(t, shared.MessageEncodingGSM7, enc)
	})

	t.Run("text with emoji returns UCS2", func(t *testing.T) {
		enc := DetectEncoding("Hello 😀")
		assert.Equal(t, shared.MessageEncodingUCS2, enc)
	})
}

func TestValidationError(t *testing.T) {
	t.Run("Error returns formatted string", func(t *testing.T) {
		err := &ValidationError{Field: "source", Message: "source is required"}
		assert.Equal(t, "validation error for field source: source is required", err.Error())
	})
}
