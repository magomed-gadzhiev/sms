package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCORSMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		origin         string
		allowedOrigins []string
		expectedHeader string
		expectedStatus int
	}{
		{
			name:           "allowed origin",
			origin:         "https://example.com",
			allowedOrigins: []string{"https://example.com"},
			expectedHeader: "https://example.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "wildcard origin",
			origin:         "https://example.com",
			allowedOrigins: []string{"*"},
			expectedHeader: "https://example.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "not allowed origin",
			origin:         "https://evil.com",
			allowedOrigins: []string{"https://example.com"},
			expectedHeader: "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "empty allowed origins - allow all",
			origin:         "https://example.com",
			allowedOrigins: []string{},
			expectedHeader: "https://example.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "no origin header",
			origin:         "",
			allowedOrigins: []string{"https://example.com"},
			expectedHeader: "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "preflight request",
			origin:         "https://example.com",
			allowedOrigins: []string{"https://example.com"},
			expectedHeader: "https://example.com",
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := CORSMiddleware(tt.allowedOrigins)

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.name == "preflight request" {
				req.Method = http.MethodOptions
			}

			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if tt.expectedHeader != "" {
				assert.Equal(t, tt.expectedHeader, w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}

			if tt.name == "preflight request" {
				assert.Equal(t, http.StatusNoContent, w.Code)
				assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
				assert.Equal(t, "Content-Type, Authorization, X-API-Key, X-Request-ID", w.Header().Get("Access-Control-Allow-Headers"))
			} else {
				assert.Equal(t, tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestCORSMiddlewareFromConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         string
		expectedOrigins []string
	}{
		{
			name:           "single origin",
			config:         "https://example.com",
			expectedOrigins: []string{"https://example.com"},
		},
		{
			name:           "multiple origins",
			config:         "https://example.com,https://app.example.com",
			expectedOrigins: []string{"https://example.com", "https://app.example.com"},
		},
		{
			name:           "with spaces",
			config:         "https://example.com, https://app.example.com",
			expectedOrigins: []string{"https://example.com", "https://app.example.com"},
		},
		{
			name:           "empty config",
			config:         "",
			expectedOrigins: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := CORSMiddlewareFromConfig(tt.config)

			req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
			req.Header.Set("Origin", "https://example.com")

			w := httptest.NewRecorder()

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(w, req)

			// Проверяем, что middleware работает
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}
