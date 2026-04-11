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

type profileCompletion struct {
	Percentage int              `json:"percentage"`
	Steps      []completionStep `json:"steps"`
}

type completionStep struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Completed bool   `json:"completed"`
}

func calcProfileCompletion(hasEmail, hasCompany, hasContact, hasPhone, has2FA, hasSender bool) profileCompletion {
	steps := []completionStep{
		{Key: "email", Label: "Email подтверждён", Completed: hasEmail},
		{Key: "company", Label: "Название компании", Completed: hasCompany},
		{Key: "contact", Label: "Контактное лицо", Completed: hasContact},
		{Key: "phone", Label: "Телефон", Completed: hasPhone},
		{Key: "2fa", Label: "Двухфакторная аутентификация", Completed: has2FA},
		{Key: "sender", Label: "Имя отправителя", Completed: hasSender},
	}
	count := 0
	for _, s := range steps {
		if s.Completed {
			count++
		}
	}
	pct := 0
	if len(steps) > 0 {
		pct = count * 100 / len(steps)
	}
	return profileCompletion{Percentage: pct, Steps: steps}
}

// sparkline1HBuckets returns 24 per-hour message-sent counts for the last 24 hours.
// Returns a slice of 24 zeros if the analytics client is unavailable.
// Note: "minute" groupBy is not supported by the analytics service; hourly buckets are used instead.
func sparkline1HBuckets(groups []*analyticsv1.StatisticGroup, now time.Time) []int64 {
	buckets := make(map[string]int64)
	for _, g := range groups {
		if g.Stats != nil {
			buckets[g.Key] = g.Stats.TotalSent
		}
	}
	result := make([]int64, 24)
	for i := 0; i < 24; i++ {
		t := now.Add(-time.Duration(24-i) * time.Hour)
		key := t.Format("2006-01-02T15")
		result[i] = buckets[key]
	}
	return result
}

// msgPerSec estimates current messages/sec from the last-hour bucket.
func msgPerSec(sparkline []int64) float64 {
	if len(sparkline) == 0 {
		return 0
	}
	return float64(sparkline[len(sparkline)-1]) / 3600.0
}

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

		hasEmail bool
		has2FA   bool
	)

	var (
		sparkline         []int64
		deliveryRate24h   int32
		deliveryRateTrend int32
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

	// 5. Получаем данные пользователя для расчёта заполненности профиля
	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.authClient == nil {
			return
		}
		resp, err := h.authClient.GetUser(ctx, &authv1.GetUserRequest{
			UserId: userID.String(),
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения пользователя для профиля дашборда")
			return
		}
		if resp.User != nil {
			hasEmail = resp.User.Email != ""
			has2FA = resp.User.TotpEnabled
		}
	}()

	// 6. Получаем данные для графиков (7-дневный timeline)
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

	// 7. Получаем sparkline (почасовые данные за последние 24 часа)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.analyticsClient == nil {
			return
		}
		now := time.Now()
		oneDayAgo := now.Add(-24 * time.Hour)
		resp, err := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
			ClientId: clientID.String(),
			From:     timestamppb.New(oneDayAgo),
			To:       timestamppb.New(now),
			GroupBy:  "hour",
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения спарклайна дашборда")
			return
		}
		sparkline = sparkline1HBuckets(resp.Groups, now)
	}()

	// 8. Получаем delivery rate за 24h + тренд за неделю
	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.analyticsClient == nil {
			return
		}
		now := time.Now()

		// 24h delivery rate
		resp, err := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
			ClientId: clientID.String(),
			From:     timestamppb.New(now.Add(-24 * time.Hour)),
			To:       timestamppb.New(now),
			GroupBy:  "day",
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения delivery rate 24h")
			return
		}
		if resp.Totals != nil {
			deliveryRate24h = resp.Totals.SuccessRate
		}

		// Trend vs same 24h last week
		weekAgo := now.AddDate(0, 0, -7)
		resp2, err2 := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
			ClientId: clientID.String(),
			From:     timestamppb.New(weekAgo),
			To:       timestamppb.New(weekAgo.Add(24 * time.Hour)),
			GroupBy:  "day",
		})
		if err2 == nil && resp2.Totals != nil {
			deliveryRateTrend = deliveryRate24h - resp2.Totals.SuccessRate
		}
	}()

	wg.Wait()

	if chartTimeline == nil {
		chartTimeline = []map[string]interface{}{}
	}
	if statusDistribution == nil {
		statusDistribution = []map[string]interface{}{}
	}
	if sparkline == nil {
		sparkline = make([]int64, 24)
	}

	completion := calcProfileCompletion(hasEmail, false, false, false, has2FA, false)

	response := map[string]interface{}{
		"balance":                  balance,
		"currency":                 currency,
		"messages_today":           messagesToday,
		"messages_delivered_today": messagesDeliveredToday,
		"delivery_rate_today":      deliveryRateToday,
		"active_api_keys":          activeAPIKeys,
		"active_webhooks":          activeWebhooks,
		"profile_completion":       completion,
		"charts": map[string]interface{}{
			"timeline_7d":         chartTimeline,
			"status_distribution": statusDistribution,
			"delivery_rate_trend": trendDelta,
		},
		"msg_per_sec":             msgPerSec(sparkline),
		"msg_per_sec_trend_pct":   int32(0), // TODO: compare vs yesterday
		"delivery_rate_24h":       deliveryRate24h,
		"delivery_rate_trend_pct": deliveryRateTrend,
		"burn_rate_per_hour":      "0.00", // TODO: compute from billing transactions
		"forecast_hours":          0,      // TODO: compute from balance / burn_rate
		"sparkline_1h":            sparkline,
		"active_campaigns":        []interface{}{}, // TODO: wire to campaign service
	}

	respondJSON(w, http.StatusOK, response)
}
