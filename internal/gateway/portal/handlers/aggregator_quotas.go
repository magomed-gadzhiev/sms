package handlers

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// quotaSummary mirrors the snake_case wire format consumed by the aggregator
// portal. `active` is derived from the period (period_start <= now <
// period_end), not stored in DB.
type quotaSummary struct {
	ID           string    `json:"id"`
	SegmentLimit int64     `json:"segment_limit"`
	SegmentsUsed int64     `json:"segments_used"`
	OverageRate  string    `json:"overage_rate"`
	Currency     string    `json:"currency"`
	PeriodStart  string    `json:"period_start"`
	PeriodEnd    string    `json:"period_end"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}

func toQuotaSummary(q *domain.AggregatorQuota) quotaSummary {
	now := time.Now().UTC()
	return quotaSummary{
		ID:           q.ID.String(),
		SegmentLimit: q.SegmentLimit,
		SegmentsUsed: q.SegmentsUsed,
		OverageRate:  q.OverageRate,
		Currency:     q.Currency,
		PeriodStart:  q.PeriodStart.Format("2006-01-02"),
		PeriodEnd:    q.PeriodEnd.Format("2006-01-02"),
		Active:       !q.PeriodStart.After(now) && q.PeriodEnd.After(now),
		CreatedAt:    q.CreatedAt,
	}
}

type AggregatorQuotaHandlers struct {
	quotaService *application.QuotaService
}

func NewAggregatorQuotaHandlers(qs *application.QuotaService) *AggregatorQuotaHandlers {
	return &AggregatorQuotaHandlers{quotaService: qs}
}

func (h *AggregatorQuotaHandlers) GetMyQuota(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("client not found"))
		return
	}

	quota, err := h.quotaService.GetActiveQuota(r.Context(), clientID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get quota")
		respondError(w, shared.ErrInternalServer("failed to get quota"))
		return
	}

	if quota == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"quota": nil})
		return
	}

	summary := toQuotaSummary(quota)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"quota": map[string]interface{}{
			"id":               summary.ID,
			"segment_limit":    summary.SegmentLimit,
			"segments_used":    summary.SegmentsUsed,
			"overage_rate":     summary.OverageRate,
			"currency":         summary.Currency,
			"period_start":     summary.PeriodStart,
			"period_end":       summary.PeriodEnd,
			"active":           summary.Active,
			"utilization":      quota.UtilizationPercent(),
			"is_exhausted":     quota.IsExhausted(),
			"overage_segments": quota.OverageSegments(),
		},
	})
}

func (h *AggregatorQuotaHandlers) GetQuotaSpending(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("client not found"))
		return
	}

	quotas, total, err := h.quotaService.ListQuotas(r.Context(), clientID, 12, 0)
	if err != nil {
		log.Error().Err(err).Msg("failed to list quotas")
		respondError(w, shared.ErrInternalServer("failed to list quota history"))
		return
	}

	out := make([]quotaSummary, len(quotas))
	for i, q := range quotas {
		out[i] = toQuotaSummary(q)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"quotas": out,
		"total":  total,
	})
}
