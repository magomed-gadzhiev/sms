package shared

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithRequestID_GetRequestID(t *testing.T) {
	t.Run("set and get request ID", func(t *testing.T) {
		ctx := context.Background()
		requestID := "test-request-123"
		ctx = WithRequestID(ctx, requestID)
		assert.Equal(t, requestID, GetRequestID(ctx))
	})

	t.Run("missing request ID returns empty string", func(t *testing.T) {
		ctx := context.Background()
		assert.Equal(t, "", GetRequestID(ctx))
	})

	t.Run("overwrites previous request ID", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "first")
		ctx = WithRequestID(ctx, "second")
		assert.Equal(t, "second", GetRequestID(ctx))
	})
}

func TestWithUserID_GetUserID(t *testing.T) {
	t.Run("set and get user ID", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.New()
		ctx = WithUserID(ctx, userID)

		got, ok := GetUserID(ctx)
		require.True(t, ok)
		assert.Equal(t, userID, got)
	})

	t.Run("missing user ID returns zero uuid and ok=false", func(t *testing.T) {
		ctx := context.Background()
		got, ok := GetUserID(ctx)
		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, got)
	})

	t.Run("nil UUID is stored and retrieved correctly", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithUserID(ctx, uuid.Nil)
		got, ok := GetUserID(ctx)
		require.True(t, ok)
		assert.Equal(t, uuid.Nil, got)
	})
}

func TestWithClientID_GetClientID(t *testing.T) {
	t.Run("set and get client ID", func(t *testing.T) {
		ctx := context.Background()
		clientID := uuid.New()
		ctx = WithClientID(ctx, clientID)

		got, ok := GetClientID(ctx)
		require.True(t, ok)
		assert.Equal(t, clientID, got)
	})

	t.Run("missing client ID returns zero uuid and ok=false", func(t *testing.T) {
		ctx := context.Background()
		got, ok := GetClientID(ctx)
		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, got)
	})
}

func TestWithServiceName_GetServiceName(t *testing.T) {
	t.Run("set and get service name", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithServiceName(ctx, "my-service")
		assert.Equal(t, "my-service", GetServiceName(ctx))
	})

	t.Run("missing service name returns empty string", func(t *testing.T) {
		ctx := context.Background()
		assert.Equal(t, "", GetServiceName(ctx))
	})
}

func TestNewRequestContext(t *testing.T) {
	t.Run("generates a non-empty request ID", func(t *testing.T) {
		ctx := context.Background()
		ctx = NewRequestContext(ctx)
		requestID := GetRequestID(ctx)
		assert.NotEmpty(t, requestID)
	})

	t.Run("each call generates a unique request ID", func(t *testing.T) {
		ctx := context.Background()
		ctx1 := NewRequestContext(ctx)
		ctx2 := NewRequestContext(ctx)
		assert.NotEqual(t, GetRequestID(ctx1), GetRequestID(ctx2))
	})

	t.Run("generated request ID is a valid UUID", func(t *testing.T) {
		ctx := NewRequestContext(context.Background())
		requestID := GetRequestID(ctx)
		_, err := uuid.Parse(requestID)
		assert.NoError(t, err, "request ID should be a valid UUID")
	})
}

func TestWithTimeout(t *testing.T) {
	t.Run("context expires after timeout", func(t *testing.T) {
		ctx := context.Background()
		ctx, cancel := WithTimeout(ctx, 10*time.Millisecond)
		defer cancel()

		// Wait for expiry
		<-ctx.Done()
		assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	})

	t.Run("cancel function cancels context early", func(t *testing.T) {
		ctx := context.Background()
		ctx, cancel := WithTimeout(ctx, 10*time.Second)
		cancel()
		assert.Error(t, ctx.Err())
	})
}

func TestWithDeadline(t *testing.T) {
	t.Run("context expires at deadline", func(t *testing.T) {
		ctx := context.Background()
		deadline := time.Now().Add(10 * time.Millisecond)
		ctx, cancel := WithDeadline(ctx, deadline)
		defer cancel()

		<-ctx.Done()
		assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	})

	t.Run("past deadline expires immediately", func(t *testing.T) {
		ctx := context.Background()
		past := time.Now().Add(-1 * time.Second)
		ctx, cancel := WithDeadline(ctx, past)
		defer cancel()
		assert.Error(t, ctx.Err())
	})
}

func TestIsCancelled(t *testing.T) {
	t.Run("fresh context is not cancelled", func(t *testing.T) {
		ctx := context.Background()
		assert.False(t, IsCancelled(ctx))
	})

	t.Run("cancelled context returns true", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		assert.True(t, IsCancelled(ctx))
	})

	t.Run("timed-out context returns true", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(5 * time.Millisecond)
		assert.True(t, IsCancelled(ctx))
	})
}

func TestContextIndependence(t *testing.T) {
	t.Run("user ID and client ID are stored independently", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.New()
		clientID := uuid.New()
		ctx = WithUserID(ctx, userID)
		ctx = WithClientID(ctx, clientID)

		gotUser, okUser := GetUserID(ctx)
		gotClient, okClient := GetClientID(ctx)

		require.True(t, okUser)
		require.True(t, okClient)
		assert.Equal(t, userID, gotUser)
		assert.Equal(t, clientID, gotClient)
		assert.NotEqual(t, gotUser, gotClient)
	})

	t.Run("child context inherits parent values", func(t *testing.T) {
		parent := WithRequestID(context.Background(), "parent-req")
		child, cancel := context.WithCancel(parent)
		defer cancel()

		assert.Equal(t, "parent-req", GetRequestID(child))
	})
}
