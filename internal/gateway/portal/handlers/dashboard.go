package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// DashboardHandlers содержит handlers для дашборда
type DashboardHandlers struct {
	billingClient   billingv1.BillingServiceClient
	analyticsClient analyticsv1.AnalyticsServiceClient
	authClient      authv1.AuthServiceClient
	webhookClient   webhookv1.WebhookServiceClient
}

// NewDashboardHandlers создает новый DashboardHandlers
func NewDashboardHandlers(
	billingClient billingv1.BillingServiceClient,
	analyticsClient analyticsv1.AnalyticsServiceClient,
	authClient authv1.AuthServiceClient,
	webhookClient webhookv1.WebhookServiceClient,
) *DashboardHandlers {
	return &DashboardHandlers{
		billingClient:   billingClient,
		analyticsClient: analyticsClient,
		authClient:      authClient,
		webhookClient:   webhookClient,
	}
}

// GetDashboard обрабатывает GET /dashboard
func (h *DashboardHandlers) GetDashboard(w http.ResponseWriter, r *http.Request) {
	clientID, _ := middleware.GetClientID(r.Context())

	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	ctx := r.Context()

	// Параллельно запрашиваем данные из нескольких сервисов
	var (
		wg sync.WaitGroup

		balance  string
		currency string

		messagesToday          int64
		messagesDeliveredToday int64
		deliveryRateToday      int32

		activeAPIKeys  int
		activeWebhooks int
	)

	// 1. Получаем баланс
	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.billingClient == nil {
			return
		}
		resp, err := h.billingClient.GetBalance(ctx, &billingv1.GetBalanceRequest{
			ClientId: clientID.String(),
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения баланса для дашборда")
			return
		}
		balance = resp.Balance
		currency = resp.Currency
	}()

	// 2. Получаем статистику за сегодня
	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.analyticsClient == nil {
			return
		}
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		resp, err := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
			ClientId: clientID.String(),
			From:     timestamppb.New(startOfDay),
			To:       timestamppb.New(now),
			GroupBy:  "day",
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения статистики для дашборда")
			return
		}
		if resp.Totals != nil {
			messagesToday = resp.Totals.TotalSent
			messagesDeliveredToday = resp.Totals.TotalDelivered
			deliveryRateToday = resp.Totals.SuccessRate
		}
	}()

	// 3. Получаем количество активных API ключей
	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.authClient == nil {
			return
		}
		resp, err := h.authClient.ListAPIKeys(ctx, &authv1.ListAPIKeysRequest{
			UserId: userID.String(),
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения API ключей для дашборда")
			return
		}
		for _, key := range resp.Keys {
			if key.Active {
				activeAPIKeys++
			}
		}
	}()

	// 4. Получаем количество активных вебхуков
	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.webhookClient == nil {
			return
		}
		resp, err := h.webhookClient.ListSubscriptions(ctx, &webhookv1.ListSubscriptionsRequest{
			ClientId: clientID.String(),
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения вебхуков для дашборда")
			return
		}
		for _, sub := range resp.Subscriptions {
			if sub.Active {
				activeWebhooks++
			}
		}
	}()

	// 5. Получаем данные для графиков (7-дневный timeline)
	var chartTimeline []map[string]interface{}
	var statusDistribution []map[string]interface{}
	var trendDelta int32

	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.analyticsClient == nil {
			return
		}
		now := time.Now()
		sevenDaysAgo := now.AddDate(0, 0, -7)

		resp, err := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
			ClientId: clientID.String(),
			From:     timestamppb.New(sevenDaysAgo),
			To:       timestamppb.New(now),
			GroupBy:  "day",
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения данных для графиков дашборда")
			return
		}

		for _, g := range resp.Groups {
			entry := map[string]interface{}{"date": g.Key}
			if g.Stats != nil {
				entry["sent"] = g.Stats.TotalSent
				entry["delivered"] = g.Stats.TotalDelivered
				entry["failed"] = g.Stats.TotalFailed
			}
			chartTimeline = append(chartTimeline, entry)
		}

		if resp.Totals != nil {
			trendDelta = resp.Totals.SuccessRate
			statusDistribution = []map[string]interface{}{
				{"status": "delivered", "count": resp.Totals.TotalDelivered, "label": "Доставлено"},
				{"status": "failed", "count": resp.Totals.TotalFailed, "label": "Ошибка"},
				{"status": "pending", "count": resp.Totals.TotalPending, "label": "В обработке"},
			}
		}
	}()

	wg.Wait()

	if chartTimeline == nil {
		chartTimeline = []map[string]interface{}{}
	}
	if statusDistribution == nil {
		statusDistribution = []map[string]interface{}{}
	}

	response := map[string]interface{}{
		"balance":                  balance,
		"currency":                 currency,
		"messages_today":           messagesToday,
		"messages_delivered_today": messagesDeliveredToday,
		"delivery_rate_today":      deliveryRateToday,
		"active_api_keys":          activeAPIKeys,
		"active_webhooks":          activeWebhooks,
		"charts": map[string]interface{}{
			"timeline_7d":         chartTimeline,
			"status_distribution": statusDistribution,
			"delivery_rate_trend": trendDelta,
		},
	}

	respondJSON(w, http.StatusOK, response)
}
