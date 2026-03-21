package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/smpp-server/smpp-server/internal/shared"
)

func TestRateLimitMiddleware(t *testing.T) {
	clientID := uuid.New()
	client := &shared.Client{
		ID:                clientID,
		RateLimitPerSecond: 10,
		RateLimitPerMinute: 100,
		RateLimitPerHour:   1000,
	}

	t.Run("skip endpoints", func(t *testing.T) {
		tests := []struct {
			name           string
			request        func() *http.Request
			redisClient    *redis.Client
			expectedStatus int
		}{
			{
				name: "skip health endpoint",
				request: func() *http.Request {
					return httptest.NewRequest("GET", "/health", nil)
				},
				redisClient:    nil,
				expectedStatus: http.StatusOK,
			},
			{
				name: "skip metrics endpoint",
				request: func() *http.Request {
					return httptest.NewRequest("GET", "/metrics", nil)
				},
				redisClient:    nil,
				expectedStatus: http.StatusOK,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				middleware := RateLimitMiddleware(tt.redisClient)

				handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))

				req := tt.request()
				ctx := context.WithValue(req.Context(), ClientKey, client)
				req = req.WithContext(ctx)

				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				assert.Equal(t, tt.expectedStatus, w.Code)
			})
		}
	})

	t.Run("no client in context - skip rate limit", func(t *testing.T) {
		middleware := RateLimitMiddleware(nil)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestCheckRateLimit(t *testing.T) {
	// Этот тест требует реального Redis или testcontainers
	// Для unit тестов используем упрощенную проверку
	t.Run("limit not set - should pass", func(t *testing.T) {
		err := checkRateLimit(context.Background(), nil, "test-client", "sec", 0, 0)
		assert.NoError(t, err)
	})
}

func TestRateLimitError(t *testing.T) {
	t.Run("error contains correct message", func(t *testing.T) {
		err := ErrRateLimitExceeded
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Превышен лимит запросов")

		var rateLimitErr *RateLimitError
		require.ErrorAs(t, err, &rateLimitErr)
		assert.Equal(t, "Превышен лимит запросов", rateLimitErr.Message)
	})
}
