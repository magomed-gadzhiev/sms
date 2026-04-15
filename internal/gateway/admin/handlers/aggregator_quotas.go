package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AggregatorQuotaHandler handles HTTP requests for aggregator quota CRUD.
type AggregatorQuotaHandler struct {
	quotaService *application.QuotaService
}

// NewAggregatorQuotaHandler creates a new AggregatorQuotaHandler.
func NewAggregatorQuotaHandler(qs *application.QuotaService) *AggregatorQuotaHandler {
	return &AggregatorQuotaHandler{quotaService: qs}
}

type createQuotaRequest struct {
	PeriodStart  string `json:"period_start"`
	PeriodEnd    string `json:"period_end"`
	SegmentLimit int64  `json:"segment_limit"`
	OverageRate  string `json:"overage_rate"`
	Currency     string `json:"currency"`
	AutoRenew    bool   `json:"auto_renew"`
}

// CreateQuota handles POST /admin/v1/aggregators/{id}/quotas
func (h *AggregatorQuotaHandler) CreateQuota(w http.ResponseWriter, r *http.Request) {
	aggID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid aggregator id"))
		return
	}

	var req createQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}

	start, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid period_start format, expected YYYY-MM-DD"))
		return
	}
	end, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid period_end format, expected YYYY-MM-DD"))
		return
	}

	currency := req.Currency
	if currency == "" {
		currency = "RUB"
	}

	quota, err := h.quotaService.CreateQuota(r.Context(), aggID, start, end, req.SegmentLimit, req.OverageRate, currency, req.AutoRenew)
	if err != nil {
		log.Error().Err(err).Msg("failed to create quota")
		respondError(w, shared.ErrInternalServer("failed to create quota"))
		return
	}

	respondJSON(w, http.StatusCreated, quota)
}

// ListQuotas handles GET /admin/v1/aggregators/{id}/quotas
func (h *AggregatorQuotaHandler) ListQuotas(w http.ResponseWriter, r *http.Request) {
	aggID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid aggregator id"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	quotas, total, err := h.quotaService.ListQuotas(r.Context(), aggID, limit, offset)
	if err != nil {
		respondError(w, shared.ErrInternalServer("failed to list quotas"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"quotas": quotas,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

type updateQuotaRequest struct {
	SegmentLimit int64  `json:"segment_limit"`
	OverageRate  string `json:"overage_rate"`
	AutoRenew    bool   `json:"auto_renew"`
}

// UpdateQuota handles PUT /admin/v1/aggregators/{id}/quotas/{quota_id}
func (h *AggregatorQuotaHandler) UpdateQuota(w http.ResponseWriter, r *http.Request) {
	quotaID, err := uuid.Parse(mux.Vars(r)["quota_id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid quota id"))
		return
	}

	var req updateQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}

	quota, err := h.quotaService.UpdateQuota(r.Context(), quotaID, req.SegmentLimit, req.OverageRate, req.AutoRenew)
	if err != nil {
		log.Error().Err(err).Msg("failed to update quota")
		respondError(w, shared.ErrInternalServer("failed to update quota"))
		return
	}

	respondJSON(w, http.StatusOK, quota)
}

// GetActiveQuota handles GET /admin/v1/aggregators/{id}/quotas/active
func (h *AggregatorQuotaHandler) GetActiveQuota(w http.ResponseWriter, r *http.Request) {
	aggID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid aggregator id"))
		return
	}

	quota, err := h.quotaService.GetActiveQuota(r.Context(), aggID)
	if err != nil {
		respondError(w, shared.ErrInternalServer("failed to get quota"))
		return
	}
	if quota == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"quota": nil})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"quota": quota})
}
