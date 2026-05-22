package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// SystemDefaultsHandlers uses *storage.SystemDefaultsRepository directly,
// so we test only the validation paths that don't hit the DB.
// The handler is constructed with a nil repo; DB calls will panic,
// but validation checks return before reaching them.

func newSystemDefaultsHandler() *SystemDefaultsHandlers {
	return NewSystemDefaultsHandlers(nil)
}

func TestSystemDefaultsHandlers_Set(t *testing.T) {
	t.Run("rejects unknown key", func(t *testing.T) {
		h := newSystemDefaultsHandler()

		body, _ := json.Marshal(map[string]int{"value": 10})
		req := httptest.NewRequest(http.MethodPut, "/admin/v1/system/defaults/unknown_key", bytes.NewReader(body))
		req = mux.SetURLVars(req, map[string]string{"key": "unknown_key"})
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		h.Set(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertErrorContains(t, rr, "неизвестный ключ")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := newSystemDefaultsHandler()

		req := httptest.NewRequest(http.MethodPut, "/admin/v1/system/defaults/rate_limit_per_second", bytes.NewReader([]byte("not-json")))
		req = mux.SetURLVars(req, map[string]string{"key": "rate_limit_per_second"})
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		h.Set(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertErrorContains(t, rr, "неверный формат")
	})

	t.Run("rejects negative value", func(t *testing.T) {
		h := newSystemDefaultsHandler()

		body, _ := json.Marshal(map[string]int{"value": -5})
		req := httptest.NewRequest(http.MethodPut, "/admin/v1/system/defaults/rate_limit_per_second", bytes.NewReader(body))
		req = mux.SetURLVars(req, map[string]string{"key": "rate_limit_per_second"})
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		h.Set(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertErrorContains(t, rr, "отрицательным")
	})

	t.Run("accepts all valid keys up to validation", func(t *testing.T) {
		// These keys pass key validation and JSON parsing but will fail
		// at the DB call (nil repo panic). We use recover to confirm
		// the handler got past all validation.
		validKeys := []string{
			"rate_limit_per_second", "rate_limit_per_minute",
			"rate_limit_per_hour", "default_tps_per_provider",
			"max_providers_per_client", "max_sub_accounts",
		}

		for _, key := range validKeys {
			t.Run(key, func(t *testing.T) {
				h := newSystemDefaultsHandler()

				body, _ := json.Marshal(map[string]int{"value": 100})
				req := httptest.NewRequest(http.MethodPut, "/admin/v1/system/defaults/"+key, bytes.NewReader(body))
				req = mux.SetURLVars(req, map[string]string{"key": key})
				req.Header.Set("Content-Type", "application/json")

				rr := httptest.NewRecorder()

				// The handler will panic on h.repo.Set (nil repo).
				// A panic means validation passed successfully.
				panicked := false
				func() {
					defer func() {
						if r := recover(); r != nil {
							panicked = true
						}
					}()
					h.Set(rr, req)
				}()

				// Either panicked (passed validation, hit nil repo) or returned non-400
				if !panicked {
					assert.NotEqual(t, http.StatusBadRequest, rr.Code,
						"key %q should pass validation", key)
				}
			})
		}
	})
}

// assertErrorContains checks that the JSON error response body contains substr.
func assertErrorContains(t *testing.T, rr *httptest.ResponseRecorder, substr string) {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("response has no 'error' object")
	}
	msg, _ := errObj["message"].(string)
	assert.Contains(t, msg, substr)
}
