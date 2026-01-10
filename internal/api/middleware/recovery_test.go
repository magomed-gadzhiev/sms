package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecoveryMiddleware(t *testing.T) {
	middleware := RecoveryMiddleware()

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "normal handler - no panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "handler with panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":{"code":"INTERNAL_ERROR","message":"Внутренняя ошибка сервера"}}`,
		},
		{
			name: "handler with panic and error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("runtime error: invalid memory address")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":{"code":"INTERNAL_ERROR","message":"Внутренняя ошибка сервера"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := middleware(tt.handler)

			req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
			w := httptest.NewRecorder()

			// Не должно быть паники
			assert.NotPanics(t, func() {
				handler.ServeHTTP(w, req)
			})

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedBody)
		})
	}
}
