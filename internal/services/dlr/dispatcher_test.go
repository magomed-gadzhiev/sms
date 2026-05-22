package dlr

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockGRPCClient struct {
	delivered    bool
	err          string
	callCount    int
	lastSystemID string
	lastReceipt  string
	failUntil    int
}

func (m *mockGRPCClient) DeliverDLR(ctx context.Context, systemID, sourceAddr, destAddr, receipt string) (bool, string, error) {
	m.callCount++
	m.lastSystemID = systemID
	m.lastReceipt = receipt
	if m.callCount <= m.failUntil {
		return false, "unavailable", context.DeadlineExceeded
	}
	return m.delivered, m.err, nil
}

func TestDispatcher_Dispatch_Success(t *testing.T) {
	mock := &mockGRPCClient{delivered: true}
	d := NewDispatcher(mock, zerolog.Nop())

	result, err := d.Dispatch(context.Background(), DispatchRequest{
		SystemID:   "aggregator1",
		SourceAddr: "380501234567",
		DestAddr:   "MyBrand",
		Receipt:    "id:msg-1 stat:DELIVRD",
	})

	require.NoError(t, err)
	assert.True(t, result)
	assert.Equal(t, "aggregator1", mock.lastSystemID)
	assert.Equal(t, "id:msg-1 stat:DELIVRD", mock.lastReceipt)
}

func TestDispatcher_Dispatch_SessionNotFound(t *testing.T) {
	mock := &mockGRPCClient{delivered: false, err: "session not found"}
	d := NewDispatcher(mock, zerolog.Nop())

	result, err := d.Dispatch(context.Background(), DispatchRequest{
		SystemID:   "unknown",
		SourceAddr: "380501234567",
		DestAddr:   "MyBrand",
		Receipt:    "id:msg-1 stat:DELIVRD",
	})

	require.NoError(t, err)
	assert.False(t, result)
}

func TestDispatcher_Dispatch_RetryOnFailure(t *testing.T) {
	mock := &mockGRPCClient{delivered: true, failUntil: 2}
	d := NewDispatcher(mock, zerolog.Nop())
	d.maxRetries = 3
	d.retryBackoff = 10 * time.Millisecond

	result, err := d.Dispatch(context.Background(), DispatchRequest{
		SystemID:   "aggregator1",
		SourceAddr: "380501234567",
		DestAddr:   "MyBrand",
		Receipt:    "id:msg-1 stat:DELIVRD",
	})

	require.NoError(t, err)
	assert.True(t, result)
	assert.Equal(t, 3, mock.callCount)
}

func TestDispatcher_Dispatch_ExhaustedRetries(t *testing.T) {
	mock := &mockGRPCClient{delivered: false, failUntil: 10}
	d := NewDispatcher(mock, zerolog.Nop())
	d.maxRetries = 3
	d.retryBackoff = 10 * time.Millisecond

	_, err := d.Dispatch(context.Background(), DispatchRequest{
		SystemID:   "aggregator1",
		SourceAddr: "380501234567",
		DestAddr:   "MyBrand",
		Receipt:    "id:msg-1 stat:DELIVRD",
	})

	require.Error(t, err)
	assert.Equal(t, 3, mock.callCount)
}
