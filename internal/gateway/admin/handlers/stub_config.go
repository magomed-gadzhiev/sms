package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smsc"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type StubConfigHandlers struct {
	repo *storage.StubConfigRepository
}

func NewStubConfigHandlers(repo *storage.StubConfigRepository) *StubConfigHandlers {
	return &StubConfigHandlers{repo: repo}
}

// GET /admin/v1/providers/{id}/stub-config
func (h *StubConfigHandlers) Get(w http.ResponseWriter, r *http.Request) {
	providerID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("неверный provider_id"))
		return
	}
	cfg, err := h.repo.GetByProviderID(r.Context(), providerID)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка получения конфигурации"))
		return
	}
	if cfg == nil {
		respondError(w, shared.ErrNotFound("конфигурация не найдена"))
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

// PUT /admin/v1/providers/{id}/stub-config
func (h *StubConfigHandlers) Upsert(w http.ResponseWriter, r *http.Request) {
	providerID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("неверный provider_id"))
		return
	}

	var req struct {
		MinDelayMs     int      `json:"min_delay_ms"`
		MaxDelayMs     int      `json:"max_delay_ms"`
		FailureRatePct int      `json:"failure_rate_pct"`
		DLRDelayMs     int      `json:"dlr_delay_ms"`
		DLRSuccessRate int      `json:"dlr_success_rate"`
		DLRStatuses    []string `json:"dlr_statuses"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}
	if req.FailureRatePct < 0 || req.FailureRatePct > 100 {
		respondError(w, shared.ErrInvalidInput("failure_rate_pct должен быть 0-100"))
		return
	}
	if req.DLRSuccessRate < 0 || req.DLRSuccessRate > 100 {
		respondError(w, shared.ErrInvalidInput("dlr_success_rate должен быть 0-100"))
		return
	}
	if req.MinDelayMs > req.MaxDelayMs {
		respondError(w, shared.ErrInvalidInput("min_delay_ms не может быть больше max_delay_ms"))
		return
	}

	cfg := &smsc.StubProviderConfig{
		ProviderID: providerID, MinDelayMs: req.MinDelayMs, MaxDelayMs: req.MaxDelayMs,
		FailureRatePct: req.FailureRatePct, DLRDelayMs: req.DLRDelayMs,
		DLRSuccessRate: req.DLRSuccessRate, DLRStatuses: req.DLRStatuses,
	}
	if err := h.repo.Upsert(r.Context(), cfg); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}
