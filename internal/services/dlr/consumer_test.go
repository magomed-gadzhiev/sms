package dlr

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/gateway/smpp/server"
	"github.com/smpp-server/smpp-server/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRedisStore struct {
	messages map[string]*server.MessageMapping
	sessions map[string]*server.SessionBinding
}

func newMockRedisStore() *mockRedisStore {
	return &mockRedisStore{
		messages: make(map[string]*server.MessageMapping),
		sessions: make(map[string]*server.SessionBinding),
	}
}

func (m *mockRedisStore) GetMessageMapping(ctx context.Context, messageID string) (*server.MessageMapping, error) {
	v, ok := m.messages[messageID]
	if !ok {
		return nil, server.ErrRedisKeyNotFound
	}
	return v, nil
}

func (m *mockRedisStore) GetSessionBinding(ctx context.Context, systemID string) (*server.SessionBinding, error) {
	v, ok := m.sessions[systemID]
	if !ok {
		return nil, server.ErrRedisKeyNotFound
	}
	return v, nil
}

type collectingDispatcher struct {
	calls []DispatchRequest
}

func (c *collectingDispatcher) Dispatch(ctx context.Context, req DispatchRequest) (bool, error) {
	c.calls = append(c.calls, req)
	return true, nil
}

func TestProcessStatusUpdate_Delivered(t *testing.T) {
	store := newMockRedisStore()
	dispatcher := &collectingDispatcher{}
	processor := NewProcessor(store, dispatcher, zerolog.Nop())

	msgID := uuid.New()
	store.messages[msgID.String()] = &server.MessageMapping{
		SystemID:   "aggregator1",
		SourceAddr: "MyBrand",
		DestAddr:   "380501234567",
		SubmitDate: time.Date(2026, 4, 15, 10, 30, 0, 0, time.UTC),
	}
	store.sessions["aggregator1"] = &server.SessionBinding{GatewayAddr: "smpp-gateway:9095"}

	err := processor.ProcessStatusUpdate(context.Background(), &pipeline.StatusUpdate{
		MessageID: msgID,
		Status:    "delivered",
	})
	require.NoError(t, err)

	require.Len(t, dispatcher.calls, 1)
	assert.Equal(t, "aggregator1", dispatcher.calls[0].SystemID)
	assert.Contains(t, dispatcher.calls[0].Receipt, "stat:DELIVRD")
	assert.Contains(t, dispatcher.calls[0].Receipt, "id:"+msgID.String())
	assert.Equal(t, "380501234567", dispatcher.calls[0].SourceAddr) // swapped
	assert.Equal(t, "MyBrand", dispatcher.calls[0].DestAddr)       // swapped
}

func TestProcessStatusUpdate_SkipsNonFinalStatus(t *testing.T) {
	store := newMockRedisStore()
	dispatcher := &collectingDispatcher{}
	processor := NewProcessor(store, dispatcher, zerolog.Nop())

	for _, status := range []string{"sent", "queued", "routed"} {
		err := processor.ProcessStatusUpdate(context.Background(), &pipeline.StatusUpdate{
			MessageID: uuid.New(),
			Status:    status,
		})
		require.NoError(t, err)
	}
	assert.Empty(t, dispatcher.calls)
}

func TestProcessStatusUpdate_MessageMappingMiss(t *testing.T) {
	store := newMockRedisStore()
	dispatcher := &collectingDispatcher{}
	processor := NewProcessor(store, dispatcher, zerolog.Nop())

	err := processor.ProcessStatusUpdate(context.Background(), &pipeline.StatusUpdate{
		MessageID: uuid.New(),
		Status:    "delivered",
	})
	require.NoError(t, err)
	assert.Empty(t, dispatcher.calls)
}

func TestProcessStatusUpdate_SessionMiss(t *testing.T) {
	store := newMockRedisStore()
	dispatcher := &collectingDispatcher{}
	processor := NewProcessor(store, dispatcher, zerolog.Nop())

	msgID := uuid.New()
	store.messages[msgID.String()] = &server.MessageMapping{
		SystemID:   "disconnected",
		SourceAddr: "MyBrand",
		DestAddr:   "380501234567",
		SubmitDate: time.Now(),
	}

	err := processor.ProcessStatusUpdate(context.Background(), &pipeline.StatusUpdate{
		MessageID: msgID,
		Status:    "delivered",
	})
	require.NoError(t, err)
	assert.Empty(t, dispatcher.calls)
}
