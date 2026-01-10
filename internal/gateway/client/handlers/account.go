package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AccountHandlers содержит handlers для операций с аккаунтом
type AccountHandlers struct {
	billingClient   billingv1.BillingServiceClient
	analyticsClient analyticsv1.AnalyticsServiceClient
}

// NewAccountHandlers создает новый AccountHandlers
func NewAccountHandlers(
	billingClient billingv1.BillingServiceClient,
	analyticsClient analyticsv1.AnalyticsServiceClient,
) *AccountHandlers {
	return &AccountHandlers{
		billingClient:   billingClient,
		analyticsClient: analyticsClient,
	}
}

// GetBalance обрабатывает запрос на получение баланса
func (h *AccountHandlers) GetBalance(w http.ResponseWriter, r *http.Request) {
	// Получаем client_id из контекста
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Вызываем Billing Service
	resp, err := h.billingClient.GetBalance(r.Context(), &billingv1.GetBalanceRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения баланса через Billing Service")
		respondGRPCError(w, err)
		return
	}

	// Формируем ответ
	response := map[string]interface{}{
		"client_id":  resp.ClientId,
		"balance":    resp.Balance,
		"currency":   resp.Currency,
		"updated_at": resp.UpdatedAt.AsTime(),
	}

	respondJSON(w, http.StatusOK, response)
}

// GetStats обрабатывает запрос на получение статистики клиента
func (h *AccountHandlers) GetStats(w http.ResponseWriter, r *http.Request) {
	// Получаем client_id из контекста
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Парсим query параметры для периода
	query := r.URL.Query()
	
	// По умолчанию за последние 30 дней
	to := time.Now()
	from := to.AddDate(0, 0, -30)

	if fromStr := query.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}
	if toStr := query.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	// Группировка по умолчанию - по дням
	groupBy := query.Get("group_by")
	if groupBy == "" {
		groupBy = "day"
	}

	// Вызываем Analytics Service
	resp, err := h.analyticsClient.GetStatistics(r.Context(), &analyticsv1.GetStatisticsRequest{
		ClientId: clientID.String(),
		From:     timestamppb.New(from),
		To:       timestamppb.New(to),
		GroupBy:  groupBy,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения статистики через Analytics Service")
		respondGRPCError(w, err)
		return
	}

	// Формируем ответ
	groups := make([]map[string]interface{}, 0, len(resp.Groups))
	for _, group := range resp.Groups {
		g := map[string]interface{}{
			"key": group.Key,
			"stats": map[string]interface{}{
				"total_sent":            group.Stats.TotalSent,
				"total_delivered":       group.Stats.TotalDelivered,
				"total_failed":          group.Stats.TotalFailed,
				"total_pending":         group.Stats.TotalPending,
				"total_queued":          group.Stats.TotalQueued,
				"success_rate":          group.Stats.SuccessRate,
				"avg_delivery_time_ms":  group.Stats.AvgDeliveryTimeMs,
			},
		}
		groups = append(groups, g)
	}

	totals := map[string]interface{}{
		"total_sent":           resp.Totals.TotalSent,
		"total_delivered":      resp.Totals.TotalDelivered,
		"total_failed":         resp.Totals.TotalFailed,
		"total_pending":        resp.Totals.TotalPending,
		"total_queued":         resp.Totals.TotalQueued,
		"success_rate":         resp.Totals.SuccessRate,
		"avg_delivery_time_ms": resp.Totals.AvgDeliveryTimeMs,
	}

	response := map[string]interface{}{
		"client_id": clientID.String(),
		"from":      from,
		"to":        to,
		"group_by":  groupBy,
		"groups":    groups,
		"totals":    totals,
	}

	respondJSON(w, http.StatusOK, response)
}
