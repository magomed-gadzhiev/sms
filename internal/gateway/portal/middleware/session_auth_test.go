package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nextHandlerRecorder записывает, был ли вызван следующий обработчик
type nextHandlerRecorder struct {
	called bool
	ctx    context.Context
}

func (h *nextHandlerRecorder) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.called = true
		h.ctx = r.Context()
		w.WriteHeader(http.StatusOK)
	})
}

// setupTestRedis создает клиент mini-redis для тестов.
// Если Redis недоступен, тест пропускается.
func setupTestRedis(t *testing.T) *redis.Client {
	t.Helper()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // используем DB 15 для тестов, чтобы не конфликтовать
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available at localhost:6379: %v", err)
	}

	// Очищаем тестовую базу
	client.FlushDB(ctx)

	t.Cleanup(func() {
		client.FlushDB(context.Background())
		client.Close()
	})

	return client
}

func TestSessionAuthMiddleware(t *testing.T) {
	t.Run("ValidSession", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}
		ctx := context.Background()

		userID := uuid.New().String()
		clientID := uuid.New().String()
		sessionID := "test-session-valid"

		// Создаем сессию в Redis
		redisClient.HSet(ctx, "session:"+sessionID, map[string]interface{}{
			"user_id":    userID,
			"client_id":  clientID,
			"role":       "owner",
			"expires_at": time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		})

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/dashboard", nil)
		req.AddCookie(&http.Cookie{Name: "portal_session", Value: sessionID})
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called")
		assert.Equal(t, http.StatusOK, rr.Code)

		// Проверяем, что user_id и client_id установлены в контексте
		ctxUserID, ok := GetUserID(recorder.ctx)
		assert.True(t, ok)
		assert.Equal(t, userID, ctxUserID.String())

		ctxClientID, ok := GetClientID(recorder.ctx)
		assert.True(t, ok)
		assert.Equal(t, clientID, ctxClientID.String())

		role, ok := GetRole(recorder.ctx)
		assert.True(t, ok)
		assert.Equal(t, "owner", role)
	})

	t.Run("ExpiredSession", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}
		ctx := context.Background()

		sessionID := "test-session-expired"

		// Создаем сессию с истекшим сроком
		redisClient.HSet(ctx, "session:"+sessionID, map[string]interface{}{
			"user_id":    uuid.New().String(),
			"client_id":  uuid.New().String(),
			"role":       "owner",
			"expires_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		})

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/dashboard", nil)
		req.AddCookie(&http.Cookie{Name: "portal_session", Value: sessionID})
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called for expired session")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		var body map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Contains(t, body, "error")
	})

	t.Run("MissingCookie", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/dashboard", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called without session cookie")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("EmptyCookieValue", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/dashboard", nil)
		req.AddCookie(&http.Cookie{Name: "portal_session", Value: ""})
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called with empty session cookie")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("SessionNotInRedis", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/dashboard", nil)
		req.AddCookie(&http.Cookie{Name: "portal_session", Value: "nonexistent-session"})
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called for nonexistent session")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("SessionMissingUserID", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}
		ctx := context.Background()

		sessionID := "test-session-no-userid"

		redisClient.HSet(ctx, "session:"+sessionID, map[string]interface{}{
			"client_id":  uuid.New().String(),
			"role":       "owner",
			"expires_at": time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		})

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/dashboard", nil)
		req.AddCookie(&http.Cookie{Name: "portal_session", Value: sessionID})
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called without user_id in session")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("SessionMissingClientID", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}
		ctx := context.Background()

		sessionID := "test-session-no-clientid"

		redisClient.HSet(ctx, "session:"+sessionID, map[string]interface{}{
			"user_id":    uuid.New().String(),
			"role":       "owner",
			"expires_at": time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		})

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/dashboard", nil)
		req.AddCookie(&http.Cookie{Name: "portal_session", Value: sessionID})
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called without client_id in session")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("PublicPath_Login_SkipsAuth", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called for public login path")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("PublicPath_Health_SkipsAuth", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called for health path")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("SessionWithNoExpiresAt_StillValid", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}
		ctx := context.Background()

		userID := uuid.New().String()
		clientID := uuid.New().String()
		sessionID := "test-session-no-expires"

		// Сессия без expires_at
		redisClient.HSet(ctx, "session:"+sessionID, map[string]interface{}{
			"user_id":   userID,
			"client_id": clientID,
			"role":      "owner",
		})

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/dashboard", nil)
		req.AddCookie(&http.Cookie{Name: "portal_session", Value: sessionID})
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called for session without expires_at")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("InvalidUserIDFormat", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}
		ctx := context.Background()

		sessionID := "test-session-invalid-uuid"

		redisClient.HSet(ctx, "session:"+sessionID, map[string]interface{}{
			"user_id":    "not-a-uuid",
			"client_id":  uuid.New().String(),
			"role":       "owner",
			"expires_at": time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		})

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/dashboard", nil)
		req.AddCookie(&http.Cookie{Name: "portal_session", Value: sessionID})
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called for invalid user_id")
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("PublicPath_PasswordReset", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		recorder := &nextHandlerRecorder{}

		middleware := SessionAuthMiddleware(redisClient)
		handler := middleware(recorder.handler())

		paths := []string{
			"/portal/v1/auth/password/reset-request",
			"/portal/v1/auth/password/reset",
		}

		for _, path := range paths {
			recorder.called = false
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.True(t, recorder.called, "next handler should be called for public path: %s", path)
			assert.Equal(t, http.StatusOK, rr.Code)
		}
	})
}
