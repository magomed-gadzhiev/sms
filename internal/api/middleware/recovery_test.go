package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecoveryMiddleware(t *testing.T) {
	middleware := RecoveryMiddleware()

	t.Run("normal handler - no panic", func(t *testing.T) {
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))

		req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
		w := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			handler.ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "OK")
	})

	t.Run("handler with panic", func(t *testing.T) {
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}))

		req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
		w := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			handler.ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), `{"error":{"code":"INTERNAL_ERROR","message":"Внутренняя ошибка сервера"}}`)
	})

	t.Run("handler with runtime error panic", func(t *testing.T) {
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("runtime error: invalid memory address")
		}))

		req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
		w := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			handler.ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), `{"error":{"code":"INTERNAL_ERROR","message":"Внутренняя ошибка сервера"}}`)
	})
}
