package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type SystemDefaultsHandlers struct {
	repo *storage.SystemDefaultsRepository
}

func NewSystemDefaultsHandlers(repo *storage.SystemDefaultsRepository) *SystemDefaultsHandlers {
	return &SystemDefaultsHandlers{repo: repo}
}

// GET /admin/v1/system/defaults
func (h *SystemDefaultsHandlers) GetAll(w http.ResponseWriter, r *http.Request) {
	all, err := h.repo.GetAll(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка получения system_defaults"))
		return
	}
	respondJSON(w, http.StatusOK, all)
}

// PUT /admin/v1/system/defaults/{key}
func (h *SystemDefaultsHandlers) Set(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]
	validKeys := map[string]bool{
		"rate_limit_per_second": true, "rate_limit_per_minute": true,
		"rate_limit_per_hour": true, "default_tps_per_provider": true,
		"max_providers_per_client": true, "max_sub_accounts": true,
	}
	if !validKeys[key] {
		respondError(w, shared.ErrInvalidInput("неизвестный ключ: "+key))
		return
	}

	var req struct {
		Value int `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}
	if req.Value < 0 {
		respondError(w, shared.ErrInvalidInput("значение не может быть отрицательным"))
		return
	}

	if err := h.repo.Set(r.Context(), key, req.Value, nil); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
