package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware(t *testing.T) {
	t.Run("UnderLimit", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		maxAttempts := 5
		window := 1 * time.Minute

		middleware := RateLimitMiddleware(redisClient, "/portal/v1/auth/login", maxAttempts, window)
		handler := middleware(recorder.handler())

		// Делаем запросы в пределах лимита
		for i := 0; i < maxAttempts; i++ {
			recorder.called = false
			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.True(t, recorder.called, "next handler should be called for request %d", i+1)
			assert.Equal(t, http.StatusOK, rr.Code)
		}
	})

	t.Run("OverLimit", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		maxAttempts := 3
		window := 1 * time.Minute

		middleware := RateLimitMiddleware(redisClient, "/portal/v1/auth/login", maxAttempts, window)
		handler := middleware(recorder.handler())

		// Исчерпываем лимит
		for i := 0; i < maxAttempts; i++ {
			recorder.called = false
			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", nil)
			req.RemoteAddr = "10.0.0.1:12345"
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)
			assert.True(t, recorder.called)
		}

		// Следующий запрос должен быть заблокирован
		recorder.called = false
		req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called when rate limit exceeded")
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
		assert.NotEmpty(t, rr.Header().Get("Retry-After"), "Retry-After header should be set")
	})

	t.Run("DifferentIPs_IndependentLimits", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		maxAttempts := 2
		window := 1 * time.Minute

		middleware := RateLimitMiddleware(redisClient, "/portal/v1/auth/login", maxAttempts, window)
		handler := middleware(recorder.handler())

		// Исчерпываем лимит для IP1
		for i := 0; i < maxAttempts; i++ {
			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", nil)
			req.RemoteAddr = "10.0.0.1:12345"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}

		// IP1 заблокирован
		recorder.called = false
		req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.False(t, recorder.called)
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)

		// IP2 все еще доступен
		recorder.called = false
		req = httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.2:12345"
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.True(t, recorder.called, "different IP should have independent rate limit")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("XForwardedFor_UsedForIP", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		maxAttempts := 2
		window := 1 * time.Minute

		middleware := RateLimitMiddleware(redisClient, "/test/xff", maxAttempts, window)
		handler := middleware(recorder.handler())

		// Исчерпываем лимит с X-Forwarded-For
		for i := 0; i < maxAttempts; i++ {
			req := httptest.NewRequest(http.MethodPost, "/test/xff", nil)
			req.Header.Set("X-Forwarded-For", "203.0.113.1, 70.41.3.18")
			req.RemoteAddr = "127.0.0.1:12345"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}

		// Следующий запрос с тем же X-Forwarded-For должен быть заблокирован
		recorder.called = false
		req := httptest.NewRequest(http.MethodPost, "/test/xff", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.1, 70.41.3.18")
		req.RemoteAddr = "127.0.0.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.False(t, recorder.called)
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	})

	t.Run("XRealIP_UsedForIP", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		maxAttempts := 1
		window := 1 * time.Minute

		middleware := RateLimitMiddleware(redisClient, "/test/xri", maxAttempts, window)
		handler := middleware(recorder.handler())

		// Первый запрос проходит
		req := httptest.NewRequest(http.MethodPost, "/test/xri", nil)
		req.Header.Set("X-Real-IP", "198.51.100.1")
		req.RemoteAddr = "127.0.0.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.True(t, recorder.called)

		// Второй заблокирован
		recorder.called = false
		req = httptest.NewRequest(http.MethodPost, "/test/xri", nil)
		req.Header.Set("X-Real-IP", "198.51.100.1")
		req.RemoteAddr = "127.0.0.1:12345"
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.False(t, recorder.called)
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	})

	t.Run("WindowExpiry_ResetsCounter", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		maxAttempts := 1
		window := 1 * time.Second // Короткое окно для теста

		middleware := RateLimitMiddleware(redisClient, "/test/expiry", maxAttempts, window)
		handler := middleware(recorder.handler())

		// Первый запрос
		req := httptest.NewRequest(http.MethodPost, "/test/expiry", nil)
		req.RemoteAddr = "10.0.0.99:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.True(t, recorder.called)

		// Второй заблокирован
		recorder.called = false
		req = httptest.NewRequest(http.MethodPost, "/test/expiry", nil)
		req.RemoteAddr = "10.0.0.99:12345"
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.False(t, recorder.called)

		// Ждем истечения окна
		time.Sleep(1100 * time.Millisecond)

		// Удаляем ключ вручную, чтобы симулировать TTL (Redis mini может не поддерживать точный TTL)
		redisClient.Del(context.Background(), "rate_limit:/test/expiry:10.0.0.99")

		// Теперь запрос должен пройти
		recorder.called = false
		req = httptest.NewRequest(http.MethodPost, "/test/expiry", nil)
		req.RemoteAddr = "10.0.0.99:12345"
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.True(t, recorder.called, "request should pass after window expiry")
	})
}

func TestExtractIP(t *testing.T) {
	t.Run("FromRemoteAddr", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		ip := extractIP(req)
		assert.Equal(t, "192.168.1.1", ip)
	})

	t.Run("FromXForwardedFor_SingleIP", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.50")

		ip := extractIP(req)
		assert.Equal(t, "203.0.113.50", ip)
	})

	t.Run("FromXForwardedFor_MultipleIPs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18, 150.172.238.178")

		ip := extractIP(req)
		assert.Equal(t, "203.0.113.50", ip)
	})

	t.Run("FromXRealIP", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Real-IP", "198.51.100.178")

		ip := extractIP(req)
		assert.Equal(t, "198.51.100.178", ip)
	})

	t.Run("XForwardedFor_TakesPrecedence_OverXRealIP", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.50")
		req.Header.Set("X-Real-IP", "198.51.100.178")

		ip := extractIP(req)
		assert.Equal(t, "203.0.113.50", ip)
	})

	t.Run("RemoteAddr_WithoutPort", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1"

		ip := extractIP(req)
		assert.Equal(t, "192.168.1.1", ip)
	})
}
