package handlers

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AnalyticsHandlers содержит handlers для аналитики
type AnalyticsHandlers struct {
	analyticsClient analyticsv1.AnalyticsServiceClient
	billingClient   billingv1.BillingServiceClient
}

// NewAnalyticsHandlers создает новый AnalyticsHandlers
func NewAnalyticsHandlers(
	analyticsClient analyticsv1.AnalyticsServiceClient,
	billingClient billingv1.BillingServiceClient,
) *AnalyticsHandlers {
	return &AnalyticsHandlers{
		analyticsClient: analyticsClient,
		billingClient:   billingClient,
	}
}

// GetAnalytics обрабатывает GET /analytics
func (h *AnalyticsHandlers) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	ctx := r.Context()
	query := r.URL.Query()

	// Парсим период
	now := time.Now()
	var dateFrom, dateTo time.Time

	if from := query.Get("date_from"); from != "" {
		parsed, err := time.Parse("2006-01-02", from)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат date_from, ожидается YYYY-MM-DD"))
			return
		}
		dateFrom = parsed
	}
	if to := query.Get("date_to"); to != "" {
		parsed, err := time.Parse("2006-01-02", to)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат date_to, ожидается YYYY-MM-DD"))
			return
		}
		dateTo = parsed.Add(24*time.Hour - time.Second) // конец дня
	}

	// Если даты не заданы, используем period
	if dateFrom.IsZero() || dateTo.IsZero() {
		period := query.Get("period")
		if period == "" {
			period = "7d"
		}
		dateTo = now
		switch period {
		case "30d":
			dateFrom = now.AddDate(0, 0, -30)
		case "90d":
			dateFrom = now.AddDate(0, 0, -90)
		default: // "7d"
			dateFrom = now.AddDate(0, 0, -7)
		}
	}

	// Группировка
	groupBy := query.Get("group_by")
	if groupBy == "" {
		groupBy = "day"
	}

	// Запрашиваем статистику из analytics service
	var statsResp *analyticsv1.GetStatisticsResponse
	if h.analyticsClient != nil {
		var err error
		statsResp, err = h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
			ClientId: clientID.String(),
			From:     timestamppb.New(dateFrom),
			To:       timestamppb.New(dateTo),
			GroupBy:  groupBy,
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения статистики")
			respondGRPCError(w, err)
			return
		}
	}

	// Получаем баланс для total_cost
	var totalCost string
	var currency string
	if h.billingClient != nil {
		balResp, err := h.billingClient.GetBalance(ctx, &billingv1.GetBalanceRequest{
			ClientId: clientID.String(),
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения баланса для аналитики")
		} else {
			totalCost = balResp.Balance
			currency = balResp.Currency
		}
	}

	// Формируем summary
	summary := map[string]interface{}{
		"total_sent":      int64(0),
		"total_delivered":  int64(0),
		"total_failed":     int64(0),
		"total_expired":    int64(0),
		"delivery_rate":    int32(0),
		"total_cost":       totalCost,
		"currency":         currency,
	}

	if statsResp != nil && statsResp.Totals != nil {
		summary["total_sent"] = statsResp.Totals.TotalSent
		summary["total_delivered"] = statsResp.Totals.TotalDelivered
		summary["total_failed"] = statsResp.Totals.TotalFailed
		summary["total_expired"] = statsResp.Totals.TotalPending // используем pending как expired placeholder
		summary["delivery_rate"] = statsResp.Totals.SuccessRate
	}

	// Формируем timeline из groups
	timeline := make([]map[string]interface{}, 0)
	if statsResp != nil {
		for _, group := range statsResp.Groups {
			entry := map[string]interface{}{
				"period": group.Key,
			}
			if group.Stats != nil {
				entry["sent"] = group.Stats.TotalSent
				entry["delivered"] = group.Stats.TotalDelivered
				entry["failed"] = group.Stats.TotalFailed
				entry["delivery_rate"] = group.Stats.SuccessRate
			}
			timeline = append(timeline, entry)
		}
	}

	// Получаем разбивку по странам (group_by=country не поддерживается напрямую,
	// но если group_by=country был запрошен, groups уже содержит данные по странам)
	byCountry := make([]map[string]interface{}, 0)
	if groupBy == "country" && statsResp != nil {
		for _, group := range statsResp.Groups {
			entry := map[string]interface{}{
				"country": group.Key,
			}
			if group.Stats != nil {
				entry["sent"] = group.Stats.TotalSent
				entry["delivered"] = group.Stats.TotalDelivered
				entry["failed"] = group.Stats.TotalFailed
				entry["delivery_rate"] = group.Stats.SuccessRate
			}
			byCountry = append(byCountry, entry)
		}
	}

	// Comparison period (compare=true)
	var prevTimeline []map[string]interface{}
	compareParam := query.Get("compare")
	if compareParam == "true" && h.analyticsClient != nil {
		prevDuration := dateTo.Sub(dateFrom)
		prevFrom := dateFrom.Add(-prevDuration)
		prevTo := dateFrom.Add(-time.Second)

		prevResp, err := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
			ClientId: clientID.String(),
			From:     timestamppb.New(prevFrom),
			To:       timestamppb.New(prevTo),
			GroupBy:  groupBy,
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения данных сравнения")
		} else {
			for _, g := range prevResp.Groups {
				entry := map[string]interface{}{"period": g.Key}
				if g.Stats != nil {
					entry["sent"] = g.Stats.TotalSent
					entry["delivered"] = g.Stats.TotalDelivered
				}
				prevTimeline = append(prevTimeline, entry)
			}
		}
	}
	if prevTimeline == nil {
		prevTimeline = []map[string]interface{}{}
	}

	response := map[string]interface{}{
		"summary":           summary,
		"timeline":          timeline,
		"previous_timeline": prevTimeline,
		"by_country":        byCountry,
	}

	respondJSON(w, http.StatusOK, response)
}
