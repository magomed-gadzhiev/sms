package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecoveryMiddleware(t *testing.T) {
	t.Run("PanicHandled_StringPanic", func(t *testing.T) {
		middleware := RecoveryMiddleware()

		panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("something went terribly wrong")
		})

		handler := middleware(panicHandler)

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/crash", nil)
		rr := httptest.NewRecorder()

		// Не должна быть паника
		assert.NotPanics(t, func() {
			handler.ServeHTTP(rr, req)
		})

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var body map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&body)
		assert.NoError(t, err)

		errObj, ok := body["error"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "INTERNAL_ERROR", errObj["code"])
	})

	t.Run("PanicHandled_ErrorPanic", func(t *testing.T) {
		middleware := RecoveryMiddleware()

		panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(assert.AnError)
		})

		handler := middleware(panicHandler)

		req := httptest.NewRequest(http.MethodPost, "/portal/v1/data", nil)
		rr := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			handler.ServeHTTP(rr, req)
		})

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("PanicHandled_NilPanic", func(t *testing.T) {
		middleware := RecoveryMiddleware()

		panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(nil)
		})

		handler := middleware(panicHandler)

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/nil", nil)
		rr := httptest.NewRecorder()

		// panic(nil) в Go 1.21+ вызывает *runtime.PanicNilError, recover() перехватывает
		assert.NotPanics(t, func() {
			handler.ServeHTTP(rr, req)
		})
	})

	t.Run("NoPanic_NormalResponse", func(t *testing.T) {
		middleware := RecoveryMiddleware()

		normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})

		handler := middleware(normalHandler)

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/normal", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "ok")
	})

	t.Run("PanicHandled_IntPanic", func(t *testing.T) {
		middleware := RecoveryMiddleware()

		panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(42)
		})

		handler := middleware(panicHandler)

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/int-panic", nil)
		rr := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			handler.ServeHTTP(rr, req)
		})

		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		var body map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&body)
		assert.NoError(t, err)
		errObj := body["error"].(map[string]interface{})
		assert.Equal(t, "INTERNAL_ERROR", errObj["code"])
	})
}
