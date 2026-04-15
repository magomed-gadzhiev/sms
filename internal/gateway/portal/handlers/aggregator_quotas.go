package handlers

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/shared"
)

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

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"quota": map[string]interface{}{
			"id":               quota.ID,
			"segment_limit":    quota.SegmentLimit,
			"segments_used":    quota.SegmentsUsed,
			"overage_rate":     quota.OverageRate,
			"currency":         quota.Currency,
			"period_start":     quota.PeriodStart,
			"period_end":       quota.PeriodEnd,
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

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"quotas": quotas,
		"total":  total,
	})
}
