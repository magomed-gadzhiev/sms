//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/api/middleware"
)

func getTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	// Ping to check connectivity
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Redis not available at %s: %v", addr, err)
	}
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func TestRateLimit_ExceedsPerSecondLimit(t *testing.T) {
	ctx := context.Background()
	rdb := getTestRedis(t)
	clientID := "test-client-" + t.Name()

	// Clean up
	defer rdb.Del(ctx, "rl:"+clientID+":sec")

	limit := 5
	for i := 0; i < limit; i++ {
		exceeded, err := middleware.CheckWindow(ctx, rdb, clientID, "sec", time.Second, limit)
		require.NoError(t, err)
		assert.False(t, exceeded, "request %d of %d should not be rate limited", i+1, limit)
	}

	// Next request should be rate limited
	exceeded, err := middleware.CheckWindow(ctx, rdb, clientID, "sec", time.Second, limit)
	require.NoError(t, err)
	assert.True(t, exceeded, "request %d should be rate limited", limit+1)
}

func TestRateLimit_WindowResets(t *testing.T) {
	ctx := context.Background()
	rdb := getTestRedis(t)
	clientID := "test-reset-" + t.Name()

	defer rdb.Del(ctx, "rl:"+clientID+":sec")

	// Fill up the limit
	limit := 3
	for i := 0; i < limit; i++ {
		exceeded, err := middleware.CheckWindow(ctx, rdb, clientID, "sec", 500*time.Millisecond, limit)
		require.NoError(t, err)
		assert.False(t, exceeded)
	}

	// Verify it's exceeded
	exceeded, err := middleware.CheckWindow(ctx, rdb, clientID, "sec", 500*time.Millisecond, limit)
	require.NoError(t, err)
	assert.True(t, exceeded, "should be limited")

	// Wait for window to expire
	time.Sleep(600 * time.Millisecond)

	// Should be allowed again
	exceeded, err = middleware.CheckWindow(ctx, rdb, clientID, "sec", 500*time.Millisecond, limit)
	require.NoError(t, err)
	assert.False(t, exceeded, "should be allowed after window reset")
}
