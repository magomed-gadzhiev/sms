package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCORSMiddleware(t *testing.T) {
	t.Run("origin handling", func(t *testing.T) {
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

				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				if tt.expectedHeader != "" {
					assert.Equal(t, tt.expectedHeader, w.Header().Get("Access-Control-Allow-Origin"))
				} else {
					assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
				}

				assert.Equal(t, tt.expectedStatus, w.Code)
			})
		}
	})

	t.Run("preflight request", func(t *testing.T) {
		middleware := CORSMiddleware([]string{"https://example.com"})

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodOptions, "/api/v1/sms/status", nil)
		req.Header.Set("Origin", "https://example.com")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "Content-Type, Authorization, X-API-Key, X-Request-ID", w.Header().Get("Access-Control-Allow-Headers"))
	})
}

func TestCORSMiddlewareFromConfig(t *testing.T) {
	tests := []struct {
		name            string
		config          string
		expectedOrigins []string
	}{
		{
			name:            "single origin",
			config:          "https://example.com",
			expectedOrigins: []string{"https://example.com"},
		},
		{
			name:            "multiple origins",
			config:          "https://example.com,https://app.example.com",
			expectedOrigins: []string{"https://example.com", "https://app.example.com"},
		},
		{
			name:            "with spaces",
			config:          "https://example.com, https://app.example.com",
			expectedOrigins: []string{"https://example.com", "https://app.example.com"},
		},
		{
			name:            "empty config",
			config:          "",
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
