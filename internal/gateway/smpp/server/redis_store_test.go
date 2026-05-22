package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRedisClient struct {
	store map[string]string
}

func newMockRedis() *mockRedisClient {
	return &mockRedisClient{store: make(map[string]string)}
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.store[key] = value.(string)
	return nil
}

func (m *mockRedisClient) Get(ctx context.Context, key string) (string, error) {
	v, ok := m.store[key]
	if !ok {
		return "", ErrRedisKeyNotFound
	}
	return v, nil
}

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		delete(m.store, k)
	}
	return nil
}

func (m *mockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return nil
}

func TestRedisStore_SaveAndGetMessageMapping(t *testing.T) {
	mock := newMockRedis()
	store := NewRedisStore(mock, 24*time.Hour, 5*time.Minute)
	ctx := context.Background()

	mapping := &MessageMapping{
		SystemID:   "aggregator1",
		SourceAddr: "MyBrand",
		DestAddr:   "380501234567",
		SubmitDate: time.Date(2026, 4, 15, 10, 30, 0, 0, time.UTC),
	}

	err := store.SaveMessageMapping(ctx, "msg-123", mapping)
	require.NoError(t, err)

	got, err := store.GetMessageMapping(ctx, "msg-123")
	require.NoError(t, err)
	assert.Equal(t, "aggregator1", got.SystemID)
	assert.Equal(t, "MyBrand", got.SourceAddr)
	assert.Equal(t, "380501234567", got.DestAddr)
}

func TestRedisStore_GetMessageMapping_NotFound(t *testing.T) {
	mock := newMockRedis()
	store := NewRedisStore(mock, 24*time.Hour, 5*time.Minute)
	ctx := context.Background()

	_, err := store.GetMessageMapping(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrRedisKeyNotFound)
}

func TestRedisStore_SaveAndGetSessionBinding(t *testing.T) {
	mock := newMockRedis()
	store := NewRedisStore(mock, 24*time.Hour, 5*time.Minute)
	ctx := context.Background()

	binding := &SessionBinding{GatewayAddr: "smpp-gateway:9095"}
	err := store.SaveSessionBinding(ctx, "aggregator1", binding)
	require.NoError(t, err)

	got, err := store.GetSessionBinding(ctx, "aggregator1")
	require.NoError(t, err)
	assert.Equal(t, "smpp-gateway:9095", got.GatewayAddr)
}

func TestRedisStore_DeleteSessionBinding(t *testing.T) {
	mock := newMockRedis()
	store := NewRedisStore(mock, 24*time.Hour, 5*time.Minute)
	ctx := context.Background()

	binding := &SessionBinding{GatewayAddr: "smpp-gateway:9095"}
	store.SaveSessionBinding(ctx, "aggregator1", binding)

	err := store.DeleteSessionBinding(ctx, "aggregator1")
	require.NoError(t, err)

	_, err = store.GetSessionBinding(ctx, "aggregator1")
	assert.ErrorIs(t, err, ErrRedisKeyNotFound)
}

func TestRedisStore_RefreshSessionTTL(t *testing.T) {
	mock := newMockRedis()
	store := NewRedisStore(mock, 24*time.Hour, 5*time.Minute)
	ctx := context.Background()

	binding := &SessionBinding{GatewayAddr: "smpp-gateway:9095"}
	store.SaveSessionBinding(ctx, "aggregator1", binding)

	err := store.RefreshSessionTTL(ctx, "aggregator1")
	require.NoError(t, err)
}
