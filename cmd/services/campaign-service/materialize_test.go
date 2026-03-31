package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: staleRow is defined in materialize.go — no re-declaration needed here.

func TestBuildStaleKafkaMsg_ValidRow(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()
	msgID := uuid.New()
	now := time.Now()

	row := staleRow{
		ID:          msgID.String(),
		Source:      "TestSender",
		Destination: "+79001234567",
		Text:        "Hello",
		ClientID:    clientID.String(),
		CreatedAt:   now,
	}

	msg, err := buildStaleKafkaMsg(row)
	require.NoError(t, err)

	assert.Equal(t, msgID.String(), msg.ID)
	assert.Equal(t, msgID, msg.MessageID)
	assert.Equal(t, "TestSender", msg.Source)
	assert.Equal(t, "+79001234567", msg.Destination)
	assert.Equal(t, "Hello", msg.Text)
	require.NotNil(t, msg.ClientID)
	assert.Equal(t, clientID, *msg.ClientID)
	assert.Equal(t, now, msg.CreatedAt)
	assert.Equal(t, 5, msg.MaxRetries)
}

func TestBuildStaleKafkaMsg_InvalidMessageID(t *testing.T) {
	t.Parallel()

	row := staleRow{
		ID:          "not-a-uuid",
		Source:      "S",
		Destination: "+7",
		Text:        "X",
		ClientID:    uuid.New().String(),
		CreatedAt:   time.Now(),
	}

	_, err := buildStaleKafkaMsg(row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid message_id")
}

func TestBuildStaleKafkaMsg_InvalidClientID(t *testing.T) {
	t.Parallel()

	row := staleRow{
		ID:          uuid.New().String(),
		Source:      "S",
		Destination: "+7",
		Text:        "X",
		ClientID:    "not-a-uuid",
		CreatedAt:   time.Now(),
	}

	_, err := buildStaleKafkaMsg(row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client_id")
}
