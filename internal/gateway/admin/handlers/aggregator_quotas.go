package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// quotaResponse is the wire-format DTO for aggregator quotas. Field names match
// the frontend AggregatorQuota interface (portal-frontend/src/api/admin.ts).
// `active` is a derived flag (period_start <= now < period_end) — it does not
// exist as a column in aggregator_quotas; the source of truth is the period.
type quotaResponse struct {
	QuotaID      uuid.UUID `json:"quota_id"`
	AggregatorID uuid.UUID `json:"aggregator_id"`
	PeriodStart  string    `json:"period_start"`
	PeriodEnd    string    `json:"period_end"`
	SegmentLimit int64     `json:"segment_limit"`
	SegmentsUsed int64     `json:"segments_used"`
	OverageRate  string    `json:"overage_rate"`
	Currency     string    `json:"currency"`
	AutoRenew    bool      `json:"auto_renew"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func quotaResponseAt(q *domain.AggregatorQuota, now time.Time) quotaResponse {
	return quotaResponse{
		QuotaID:      q.ID,
		AggregatorID: q.AggregatorID,
		PeriodStart:  q.PeriodStart.Format("2006-01-02"),
		PeriodEnd:    q.PeriodEnd.Format("2006-01-02"),
		SegmentLimit: q.SegmentLimit,
		SegmentsUsed: q.SegmentsUsed,
		OverageRate:  q.OverageRate,
		Currency:     q.Currency,
		AutoRenew:    q.AutoRenew,
		Active:       !q.PeriodStart.After(now) && q.PeriodEnd.After(now),
		CreatedAt:    q.CreatedAt,
		UpdatedAt:    q.UpdatedAt,
	}
}

func toQuotaResponse(q *domain.AggregatorQuota) quotaResponse {
	return quotaResponseAt(q, time.Now().UTC())
}

func toQuotaResponses(qs []*domain.AggregatorQuota) []quotaResponse {
	now := time.Now().UTC()
	out := make([]quotaResponse, len(qs))
	for i, q := range qs {
		out[i] = quotaResponseAt(q, now)
	}
	return out
}

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

	respondJSON(w, http.StatusCreated, map[string]interface{}{"quota": toQuotaResponse(quota)})
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
		"quotas": toQuotaResponses(quotas),
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
	aggID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid aggregator id"))
		return
	}
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

	quota, err := h.quotaService.UpdateQuota(r.Context(), aggID, quotaID, req.SegmentLimit, req.OverageRate, req.AutoRenew)
	if err != nil {
		if errors.Is(err, application.ErrQuotaNotFound) {
			respondError(w, shared.ErrNotFound("quota not found"))
			return
		}
		log.Error().Err(err).Msg("failed to update quota")
		respondError(w, shared.ErrInternalServer("failed to update quota"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"quota": toQuotaResponse(quota)})
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

	respondJSON(w, http.StatusOK, map[string]interface{}{"quota": toQuotaResponse(quota)})
}
