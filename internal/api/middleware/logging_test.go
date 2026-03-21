package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestLoggingMiddleware(t *testing.T) {
	t.Run("generates request ID when not provided", func(t *testing.T) {
		logger := zerolog.Nop()

		middleware := LoggingMiddleware(logger)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))

		req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "OK", w.Body.String())

		// Проверяем, что request ID был добавлен
		requestID := w.Header().Get("X-Request-ID")
		assert.NotEmpty(t, requestID)
	})

	t.Run("uses provided request ID", func(t *testing.T) {
		logger := zerolog.Nop()

		middleware := LoggingMiddleware(logger)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
		req.Header.Set("X-Request-ID", "custom-request-id")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Проверяем, что используется переданный request ID
		requestID := w.Header().Get("X-Request-ID")
		assert.Equal(t, "custom-request-id", requestID)
	})
}

func TestLoggingResponseWriter(t *testing.T) {
	t.Run("tracks status code and response size", func(t *testing.T) {
		w := httptest.NewRecorder()
		lw := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Тест WriteHeader
		lw.WriteHeader(http.StatusNotFound)
		assert.Equal(t, http.StatusNotFound, lw.statusCode)
		assert.Equal(t, http.StatusNotFound, w.Code)

		// Тест Write
		data := []byte("test data")
		size, err := lw.Write(data)
		assert.NoError(t, err)
		assert.Equal(t, len(data), size)
		assert.Equal(t, len(data), lw.size)
	})
}
