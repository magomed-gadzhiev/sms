package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantLoggerMiddleware(t *testing.T) {
	t.Run("enriches logger with tenant_id when client_id present in context", func(t *testing.T) {
		var buf bytes.Buffer
		logger := zerolog.New(&buf)

		tenantMiddleware := TenantLoggerMiddleware(logger)

		tenantID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")

		var capturedTenantID string
		handler := tenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Downstream code reads the logger enriched by tenant middleware
			log := zerolog.Ctx(r.Context())
			log.Info().Msg("test message")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
		// Simulate auth middleware having set ClientIDKey
		ctx := context.WithValue(req.Context(), ClientIDKey, tenantID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Parse the logged JSON
		var logEntry map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))

		capturedTenantID, ok := logEntry["tenant_id"].(string)
		require.True(t, ok, "tenant_id field should be present in log entry")
		assert.Equal(t, tenantID.String(), capturedTenantID)
	})

	t.Run("does not add tenant_id to logger when client_id absent from context", func(t *testing.T) {
		var buf bytes.Buffer
		logger := zerolog.New(&buf)

		tenantMiddleware := TenantLoggerMiddleware(logger)

		handler := tenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := zerolog.Ctx(r.Context())
			log.Info().Msg("test message")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
		// No client_id in context
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// If nothing was logged (zerolog.Ctx returns disabled logger when no context logger set),
		// buf may be empty — that's fine. If something was logged, tenant_id must not be present.
		if buf.Len() > 0 {
			var logEntry map[string]interface{}
			require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))
			_, hasTenantID := logEntry["tenant_id"]
			assert.False(t, hasTenantID, "tenant_id should not be present when client_id is absent")
		}
	})

	t.Run("enriched logger is accessible via zerolog.Ctx", func(t *testing.T) {
		var buf bytes.Buffer
		logger := zerolog.New(&buf)

		tenantMiddleware := TenantLoggerMiddleware(logger)

		tenantID := uuid.MustParse("11111111-2222-3333-4444-555555555555")

		handler := tenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctxLogger := zerolog.Ctx(r.Context())
			// The context logger should not be the disabled/nop logger
			assert.NotNil(t, ctxLogger)
			ctxLogger.Info().Msg("downstream log")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/api/v1/sms/send", nil)
		ctx := context.WithValue(req.Context(), ClientIDKey, tenantID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var logEntry map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))
		assert.Equal(t, tenantID.String(), logEntry["tenant_id"])
		assert.Equal(t, "downstream log", logEntry["message"])
	})

	t.Run("passes request to next handler unchanged", func(t *testing.T) {
		logger := zerolog.Nop()
		tenantMiddleware := TenantLoggerMiddleware(logger)

		tenantID := uuid.New()
		var receivedMethod, receivedPath string

		handler := tenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusAccepted)
		}))

		req := httptest.NewRequest("POST", "/api/v1/sms/send", nil)
		ctx := context.WithValue(req.Context(), ClientIDKey, tenantID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Equal(t, "POST", receivedMethod)
		assert.Equal(t, "/api/v1/sms/send", receivedPath)
	})
}
