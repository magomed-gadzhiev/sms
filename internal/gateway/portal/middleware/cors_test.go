package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCORSMiddleware(t *testing.T) {
	t.Run("AllowedOrigin_SingleOrigin", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CORSMiddleware("https://portal.example.com")
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/data", nil)
		req.Header.Set("Origin", "https://portal.example.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called")
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "https://portal.example.com", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", rr.Header().Get("Access-Control-Allow-Credentials"))
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), "GET")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), "POST")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "X-CSRF-Token")
	})

	t.Run("AllowedOrigin_MultipleOrigins", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CORSMiddleware("https://portal.example.com, https://admin.example.com")
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/data", nil)
		req.Header.Set("Origin", "https://admin.example.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called)
		assert.Equal(t, "https://admin.example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("DisallowedOrigin", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CORSMiddleware("https://portal.example.com")
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/data", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Запрос все равно проходит, но без Access-Control-Allow-Origin
		assert.True(t, recorder.called, "next handler should still be called")
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"),
			"Access-Control-Allow-Origin should not be set for disallowed origin")
	})

	t.Run("WildcardOrigin", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CORSMiddleware("*")
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/data", nil)
		req.Header.Set("Origin", "https://any-origin.example.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called)
		assert.Equal(t, "https://any-origin.example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("PreflightRequest_AllowedOrigin", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CORSMiddleware("https://portal.example.com")
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodOptions, "/portal/v1/messages", nil)
		req.Header.Set("Origin", "https://portal.example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Content-Type, X-CSRF-Token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Preflight не вызывает следующий обработчик
		assert.False(t, recorder.called, "next handler should NOT be called for OPTIONS preflight")
		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Equal(t, "https://portal.example.com", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), "POST")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "X-CSRF-Token")
		assert.Equal(t, "3600", rr.Header().Get("Access-Control-Max-Age"))
		assert.Equal(t, "true", rr.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("PreflightRequest_DisallowedOrigin", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CORSMiddleware("https://portal.example.com")
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodOptions, "/portal/v1/messages", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called)
		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("NoOriginHeader", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CORSMiddleware("https://portal.example.com")
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/data", nil)
		// No Origin header
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called even without Origin")
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("CORSHeaders_AlwaysPresent", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CORSMiddleware("https://portal.example.com")
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/data", nil)
		req.Header.Set("Origin", "https://portal.example.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Эти заголовки должны присутствовать всегда
		assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Methods"))
		assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Headers"))
		assert.Equal(t, "true", rr.Header().Get("Access-Control-Allow-Credentials"))
		assert.Equal(t, "3600", rr.Header().Get("Access-Control-Max-Age"))
	})
}
