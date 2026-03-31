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

// StubConfigHandlers uses *storage.StubConfigRepository directly,
// so we test only validation paths that don't hit the DB.

func newStubConfigHandler() *StubConfigHandlers {
	return NewStubConfigHandlers(nil)
}

func TestStubConfigHandlers_Get(t *testing.T) {
	t.Run("returns 400 for invalid provider_id", func(t *testing.T) {
		h := newStubConfigHandler()

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/providers/not-a-uuid/stub-config", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "not-a-uuid"})

		rr := httptest.NewRecorder()
		h.Get(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertErrorContains(t, rr, "provider_id")
	})
}

func TestStubConfigHandlers_Upsert(t *testing.T) {
	t.Run("rejects invalid provider_id", func(t *testing.T) {
		h := newStubConfigHandler()

		body, _ := json.Marshal(map[string]interface{}{
			"min_delay_ms":     10,
			"max_delay_ms":     100,
			"failure_rate_pct": 5,
		})
		req := httptest.NewRequest(http.MethodPut, "/admin/v1/providers/bad-id/stub-config", bytes.NewReader(body))
		req = mux.SetURLVars(req, map[string]string{"id": "bad-id"})
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		h.Upsert(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertErrorContains(t, rr, "provider_id")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := newStubConfigHandler()

		req := httptest.NewRequest(http.MethodPut,
			"/admin/v1/providers/550e8400-e29b-41d4-a716-446655440000/stub-config",
			bytes.NewReader([]byte("{")))
		req = mux.SetURLVars(req, map[string]string{"id": "550e8400-e29b-41d4-a716-446655440000"})
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		h.Upsert(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertErrorContains(t, rr, "неверный формат")
	})

	t.Run("rejects failure_rate_pct > 100", func(t *testing.T) {
		h := newStubConfigHandler()

		body, _ := json.Marshal(map[string]interface{}{
			"min_delay_ms":     10,
			"max_delay_ms":     100,
			"failure_rate_pct": 101,
			"dlr_success_rate": 90,
		})
		req := httptest.NewRequest(http.MethodPut,
			"/admin/v1/providers/550e8400-e29b-41d4-a716-446655440000/stub-config",
			bytes.NewReader(body))
		req = mux.SetURLVars(req, map[string]string{"id": "550e8400-e29b-41d4-a716-446655440000"})
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		h.Upsert(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertErrorContains(t, rr, "failure_rate_pct")
	})

	t.Run("rejects negative failure_rate_pct", func(t *testing.T) {
		h := newStubConfigHandler()

		body, _ := json.Marshal(map[string]interface{}{
			"min_delay_ms":     10,
			"max_delay_ms":     100,
			"failure_rate_pct": -1,
			"dlr_success_rate": 90,
		})
		req := httptest.NewRequest(http.MethodPut,
			"/admin/v1/providers/550e8400-e29b-41d4-a716-446655440000/stub-config",
			bytes.NewReader(body))
		req = mux.SetURLVars(req, map[string]string{"id": "550e8400-e29b-41d4-a716-446655440000"})
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		h.Upsert(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertErrorContains(t, rr, "failure_rate_pct")
	})

	t.Run("rejects dlr_success_rate > 100", func(t *testing.T) {
		h := newStubConfigHandler()

		body, _ := json.Marshal(map[string]interface{}{
			"min_delay_ms":     10,
			"max_delay_ms":     100,
			"failure_rate_pct": 5,
			"dlr_success_rate": 150,
		})
		req := httptest.NewRequest(http.MethodPut,
			"/admin/v1/providers/550e8400-e29b-41d4-a716-446655440000/stub-config",
			bytes.NewReader(body))
		req = mux.SetURLVars(req, map[string]string{"id": "550e8400-e29b-41d4-a716-446655440000"})
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		h.Upsert(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertErrorContains(t, rr, "dlr_success_rate")
	})

	t.Run("rejects min_delay_ms > max_delay_ms", func(t *testing.T) {
		h := newStubConfigHandler()

		body, _ := json.Marshal(map[string]interface{}{
			"min_delay_ms":     500,
			"max_delay_ms":     100,
			"failure_rate_pct": 5,
			"dlr_success_rate": 90,
		})
		req := httptest.NewRequest(http.MethodPut,
			"/admin/v1/providers/550e8400-e29b-41d4-a716-446655440000/stub-config",
			bytes.NewReader(body))
		req = mux.SetURLVars(req, map[string]string{"id": "550e8400-e29b-41d4-a716-446655440000"})
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		h.Upsert(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertErrorContains(t, rr, "min_delay_ms")
	})
}
