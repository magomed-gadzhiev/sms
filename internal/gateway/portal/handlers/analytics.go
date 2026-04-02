package handlers

import (
	"context"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

const (
	analyticsTimeout            = 8 * time.Second
	billingTimeout              = 6 * time.Second
	maxAnalyticsRangeDays       = 366
	maxChargeHistoryRows  int   = 5000
	chargeHistoryPageSize int32 = 500
)

var (
	allowedAnalyticsPeriods = map[string]struct{}{
		"7d":  {},
		"30d": {},
		"90d": {},
	}
	allowedAnalyticsGroups = map[string]struct{}{
		"day":     {},
		"week":    {},
		"country": {},
	}
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
	if h.analyticsClient == nil {
		respondError(w, shared.ErrServiceUnavailable("Сервис аналитики временно недоступен"))
		return
	}

	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	ctx := r.Context()
	query := r.URL.Query()

	// Парсим период
	now := time.Now().UTC()
	var dateFrom, dateTo time.Time
	var err error

	fromParam := query.Get("date_from")
	toParam := query.Get("date_to")
	if (fromParam == "" && toParam != "") || (fromParam != "" && toParam == "") {
		respondError(w, shared.ErrInvalidInput("Поля date_from и date_to должны передаваться вместе"))
		return
	}

	if fromParam != "" {
		dateFrom, err = time.Parse("2006-01-02", fromParam)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат date_from, ожидается YYYY-MM-DD"))
			return
		}
		parsedTo, parseErr := time.Parse("2006-01-02", toParam)
		if parseErr != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат date_to, ожидается YYYY-MM-DD"))
			return
		}
		dateTo = parsedTo.Add(24*time.Hour - time.Second) // конец дня
		if dateTo.Before(dateFrom) {
			respondError(w, shared.ErrInvalidInput("date_to не может быть раньше date_from"))
			return
		}
		if dateTo.Sub(dateFrom) > maxAnalyticsRangeDays*24*time.Hour {
			respondError(w, shared.ErrInvalidInput("Максимальный диапазон дат — 366 дней"))
			return
		}
	}

	// Если даты не заданы, используем period
	if dateFrom.IsZero() || dateTo.IsZero() {
		period := query.Get("period")
		if period == "" {
			period = "7d"
		}
		if _, exists := allowedAnalyticsPeriods[period]; !exists {
			respondError(w, shared.ErrInvalidInput("Параметр period должен быть одним из: 7d, 30d, 90d"))
			return
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
	if _, exists := allowedAnalyticsGroups[groupBy]; !exists {
		respondError(w, shared.ErrInvalidInput("Параметр group_by должен быть одним из: day, week, country"))
		return
	}

	compare := false
	if compareParam := query.Get("compare"); compareParam != "" {
		compare, err = strconv.ParseBool(compareParam)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Параметр compare должен быть true или false"))
			return
		}
	}

	includeCost := false
	if includeCostParam := query.Get("include_cost"); includeCostParam != "" {
		includeCost, err = strconv.ParseBool(includeCostParam)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Параметр include_cost должен быть true или false"))
			return
		}
	}

	// Запрашиваем статистику из analytics service
	statsCtx, cancelStats := context.WithTimeout(ctx, analyticsTimeout)
	defer cancelStats()
	statsResp, err := h.analyticsClient.GetStatistics(statsCtx, &analyticsv1.GetStatisticsRequest{
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

	// Получаем стоимость за период (только при include_cost=true)
	var totalCost string
	var currency string
	if includeCost && h.billingClient != nil {
		costCtx, cancelCost := context.WithTimeout(ctx, billingTimeout)
		defer cancelCost()
		totalCost, currency, err = h.sumChargeAmount(costCtx, clientID.String(), dateFrom, dateTo)
		if err != nil {
			log.Error().Err(err).Msg("ошибка расчёта стоимости аналитики")
		}
	}
	if currency == "" {
		currency = "RUB"
	}

	// Формируем summary
	summary := map[string]interface{}{
		"total_sent":      int64(0),
		"total_delivered": int64(0),
		"total_failed":    int64(0),
		"total_expired":   int64(0),
		"delivery_rate":   int32(0),
		"total_cost":      totalCost,
		"currency":        currency,
	}

	if statsResp.Totals != nil {
		summary["total_sent"] = statsResp.Totals.TotalSent
		summary["total_delivered"] = statsResp.Totals.TotalDelivered
		summary["total_failed"] = statsResp.Totals.TotalFailed
		summary["total_expired"] = int64(0)
		summary["delivery_rate"] = statsResp.Totals.SuccessRate
	}

	// Формируем timeline из groups
	timeline := make([]map[string]interface{}, 0)
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

	// Получаем разбивку по странам (group_by=country не поддерживается напрямую,
	// но если group_by=country был запрошен, groups уже содержит данные по странам)
	byCountry := make([]map[string]interface{}, 0)
	if groupBy == "country" {
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
	if compare {
		prevDuration := dateTo.Sub(dateFrom)
		prevTo := dateFrom.Add(-time.Second)
		prevFrom := prevTo.Add(-prevDuration)

		compareCtx, cancelCompare := context.WithTimeout(ctx, analyticsTimeout)
		defer cancelCompare()
		prevResp, err := h.analyticsClient.GetStatistics(compareCtx, &analyticsv1.GetStatisticsRequest{
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

func (h *AnalyticsHandlers) sumChargeAmount(
	ctx context.Context,
	clientID string,
	dateFrom time.Time,
	dateTo time.Time,
) (string, string, error) {
	total := new(big.Rat)
	var currency string
	var offset int32
	rowsRead := 0

	for {
		resp, err := h.billingClient.GetTransactionHistory(ctx, &billingv1.GetTransactionHistoryRequest{
			ClientId:        clientID,
			From:            timestamppb.New(dateFrom),
			To:              timestamppb.New(dateTo),
			TransactionType: "charge",
			Limit:           chargeHistoryPageSize,
			Offset:          offset,
		})
		if err != nil {
			return "", "", err
		}
		if len(resp.Transactions) == 0 {
			break
		}

		for _, tx := range resp.Transactions {
			rowsRead++
			if rowsRead > maxChargeHistoryRows {
				log.Warn().Int("rows", rowsRead).Msg("лимит чтения истории транзакций для аналитики достигнут")
				return total.FloatString(2), currency, nil
			}

			if currency == "" && tx.Currency != "" {
				currency = tx.Currency
			}
			if tx.Amount == "" {
				continue
			}
			amount, ok := new(big.Rat).SetString(tx.Amount)
			if !ok {
				continue
			}
			total.Add(total, amount)
		}

		offset += int32(len(resp.Transactions))
		if offset >= resp.Total {
			break
		}
	}

	return total.FloatString(2), currency, nil
}
