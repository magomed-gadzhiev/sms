package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type ResellerAnalyticsHandlers struct {
	analyticsClient analyticsv1.AnalyticsServiceClient
	clientClient    clientv1.ClientServiceClient
}

// NewResellerAnalyticsHandlers конструктор. До C.4 принимал ещё pgxpool,
// но единственный pool-консьюмер (SQL-проверка is_reseller в checkReseller)
// был удалён как дубль ResellerOnlyMiddleware — поле больше не нужно.
func NewResellerAnalyticsHandlers(
	analyticsClient analyticsv1.AnalyticsServiceClient,
	clientClient clientv1.ClientServiceClient,
) *ResellerAnalyticsHandlers {
	return &ResellerAnalyticsHandlers{
		analyticsClient: analyticsClient,
		clientClient:    clientClient,
	}
}

// checkReseller — см. C.4 cleanup в reseller_dashboard.go.
func (h *ResellerAnalyticsHandlers) checkReseller(w http.ResponseWriter, r *http.Request) (string, bool) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return "", false
	}
	return clientID.String(), true
}

// GetNetworkAnalytics GET /portal/v1/reseller/analytics?period=7d|30d|90d&sub_account_id=...&group_by=day|week|month
func (h *ResellerAnalyticsHandlers) GetNetworkAnalytics(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	// Parse params
	now := time.Now().UTC()
	period := r.URL.Query().Get("period")
	var dateFrom time.Time
	switch period {
	case "90d":
		dateFrom = now.AddDate(0, 0, -90)
	case "30d":
		dateFrom = now.AddDate(0, 0, -30)
	default:
		period = "7d"
		dateFrom = now.AddDate(0, 0, -7)
	}

	groupBy := r.URL.Query().Get("group_by")
	if groupBy == "" {
		groupBy = "day"
	}

	subAccountFilter := r.URL.Query().Get("sub_account_id")

	// Get sub-account list
	subResp, err := h.clientClient.ListSubAccounts(ctx, &clientv1.ListSubAccountsRequest{
		ParentClientId: resellerID,
	})
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка получения субаккаунтов"))
		return
	}

	// Filter to specific sub-account if requested
	type subInfo struct {
		id   string
		name string
	}
	var subs []subInfo
	for _, sa := range subResp.SubAccounts {
		if subAccountFilter != "" && sa.Id != subAccountFilter {
			continue
		}
		name := sa.Name
		if name == "" {
			name = sa.Email
		}
		subs = append(subs, subInfo{id: sa.Id, name: name})
	}

	if len(subs) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"summary":        map[string]interface{}{"total_sent": 0, "total_delivered": 0, "total_failed": 0, "delivery_rate": 0},
			"timeline":       []interface{}{},
			"by_sub_account": []interface{}{},
			"period":         period,
		})
		return
	}

	// Fetch analytics per sub-account in parallel
	type saAnalytics struct {
		id        string
		name      string
		sent      int64
		delivered int64
		failed    int64
		groups    []*analyticsv1.StatisticGroup
	}

	results := make([]saAnalytics, len(subs))
	var wg sync.WaitGroup

	for i, sub := range subs {
		wg.Add(1)
		go func(idx int, s subInfo) {
			defer wg.Done()
			resp, err := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
				ClientId: s.id,
				From:     timestamppb.New(dateFrom),
				To:       timestamppb.New(now),
				GroupBy:  groupBy,
			})
			if err != nil {
				log.Error().Err(err).Str("sub_account_id", s.id).Msg("ошибка аналитики субаккаунта")
				return
			}
			r := saAnalytics{id: s.id, name: s.name, groups: resp.Groups}
			if resp.Totals != nil {
				r.sent = resp.Totals.TotalSent
				r.delivered = resp.Totals.TotalDelivered
				r.failed = resp.Totals.TotalFailed
			}
			results[idx] = r
		}(i, sub)
	}
	wg.Wait()

	// Aggregate totals
	var totalSent, totalDelivered, totalFailed int64
	for _, r := range results {
		totalSent += r.sent
		totalDelivered += r.delivered
		totalFailed += r.failed
	}
	deliveryRate := float64(0)
	if totalSent > 0 {
		deliveryRate = float64(totalDelivered) / float64(totalSent) * 100
	}

	// Aggregate timeline (merge groups by key)
	timelineMap := make(map[string]struct{ sent, delivered, failed int64 })
	for _, r := range results {
		for _, g := range r.groups {
			if g.Stats == nil {
				continue
			}
			entry := timelineMap[g.Key]
			entry.sent += g.Stats.TotalSent
			entry.delivered += g.Stats.TotalDelivered
			entry.failed += g.Stats.TotalFailed
			timelineMap[g.Key] = entry
		}
	}

	// Sort timeline keys
	timelineKeys := make([]string, 0, len(timelineMap))
	for k := range timelineMap {
		timelineKeys = append(timelineKeys, k)
	}
	// Simple sort
	for i := 0; i < len(timelineKeys); i++ {
		for j := i + 1; j < len(timelineKeys); j++ {
			if timelineKeys[j] < timelineKeys[i] {
				timelineKeys[i], timelineKeys[j] = timelineKeys[j], timelineKeys[i]
			}
		}
	}

	timeline := make([]map[string]interface{}, 0, len(timelineKeys))
	for _, key := range timelineKeys {
		entry := timelineMap[key]
		rate := float64(0)
		if entry.sent > 0 {
			rate = float64(entry.delivered) / float64(entry.sent) * 100
		}
		timeline = append(timeline, map[string]interface{}{
			"period":        key,
			"sent":          entry.sent,
			"delivered":     entry.delivered,
			"failed":        entry.failed,
			"delivery_rate": rate,
		})
	}

	// Per sub-account breakdown
	bySubAccount := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		if r.id == "" {
			continue
		}
		rate := float64(0)
		if r.sent > 0 {
			rate = float64(r.delivered) / float64(r.sent) * 100
		}
		bySubAccount = append(bySubAccount, map[string]interface{}{
			"id":              r.id,
			"name":            r.name,
			"total_sent":      r.sent,
			"total_delivered": r.delivered,
			"total_failed":    r.failed,
			"delivery_rate":   rate,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"summary": map[string]interface{}{
			"total_sent":      totalSent,
			"total_delivered": totalDelivered,
			"total_failed":    totalFailed,
			"delivery_rate":   deliveryRate,
		},
		"timeline":       timeline,
		"by_sub_account": bySubAccount,
		"period":         period,
	})
}
