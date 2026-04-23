package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type ResellerDashboardHandlers struct {
	pool            *pgxpool.Pool
	billingClient   billingv1.BillingServiceClient
	analyticsClient analyticsv1.AnalyticsServiceClient
	clientClient    clientv1.ClientServiceClient
}

func NewResellerDashboardHandlers(
	pool *pgxpool.Pool,
	billingClient billingv1.BillingServiceClient,
	analyticsClient analyticsv1.AnalyticsServiceClient,
	clientClient clientv1.ClientServiceClient,
) *ResellerDashboardHandlers {
	return &ResellerDashboardHandlers{
		pool:            pool,
		billingClient:   billingClient,
		analyticsClient: analyticsClient,
		clientClient:    clientClient,
	}
}

func (h *ResellerDashboardHandlers) checkReseller(w http.ResponseWriter, r *http.Request) (string, bool) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return "", false
	}
	var isReseller bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT is_reseller FROM clients WHERE id = $1`, clientID,
	).Scan(&isReseller); err != nil || !isReseller {
		respondError(w, shared.ErrUnauthorized("доступ только для агрегаторов"))
		return "", false
	}
	return clientID.String(), true
}

// GetResellerDashboard GET /portal/v1/reseller/dashboard?period=today|7d|30d
func (h *ResellerDashboardHandlers) GetResellerDashboard(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	// Parse period
	now := time.Now().UTC()
	period := r.URL.Query().Get("period")
	var dateFrom time.Time
	switch period {
	case "30d":
		dateFrom = now.AddDate(0, 0, -30)
	case "7d":
		dateFrom = now.AddDate(0, 0, -7)
	default:
		period = "today"
		dateFrom = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	// Get sub-account list first (needed by other queries)
	subResp, err := h.clientClient.ListSubAccounts(ctx, &clientv1.ListSubAccountsRequest{
		ParentClientId: resellerID,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения субаккаунтов для дашборда")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	subIDs := make([]string, len(subResp.SubAccounts))
	subNames := make(map[string]string)
	for i, sa := range subResp.SubAccounts {
		subIDs[i] = sa.Id
		subNames[sa.Id] = sa.Name
		if sa.Name == "" {
			subNames[sa.Id] = sa.Email
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// --- 1. Network balance ---
	type balanceData struct {
		OwnBalance string `json:"own_balance"`
		SubBalance string `json:"sub_balance"`
		Total      string `json:"total"`
		Currency   string `json:"currency"`
	}
	var balances balanceData

	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.billingClient == nil {
			return
		}
		ownResp, err := h.billingClient.GetBalance(ctx, &billingv1.GetBalanceRequest{ClientId: resellerID})
		if err != nil {
			log.Error().Err(err).Msg("ошибка получения баланса агрегатора")
			return
		}
		ownBal, _ := strconv.ParseFloat(ownResp.Balance, 64)
		balances.Currency = ownResp.Currency

		var subTotal float64
		for _, sid := range subIDs {
			resp, err := h.billingClient.GetBalance(ctx, &billingv1.GetBalanceRequest{ClientId: sid})
			if err == nil {
				v, _ := strconv.ParseFloat(resp.Balance, 64)
				subTotal += v
			}
		}
		mu.Lock()
		balances.OwnBalance = strconv.FormatFloat(ownBal, 'f', 2, 64)
		balances.SubBalance = strconv.FormatFloat(subTotal, 'f', 2, 64)
		balances.Total = strconv.FormatFloat(ownBal+subTotal, 'f', 2, 64)
		mu.Unlock()
	}()

	// --- 2+3. Traffic & delivery rate ---
	type trafficData struct {
		TotalSent      int64   `json:"total_sent"`
		TotalDelivered int64   `json:"total_delivered"`
		TotalFailed    int64   `json:"total_failed"`
		DeliveryRate   float64 `json:"delivery_rate"`
	}
	var traffic trafficData

	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.analyticsClient == nil {
			return
		}
		var totalSent, totalDelivered, totalFailed int64
		for _, sid := range subIDs {
			resp, err := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
				ClientId: sid,
				From:     timestamppb.New(dateFrom),
				To:       timestamppb.New(now),
				GroupBy:  "day",
			})
			if err != nil {
				continue
			}
			if resp.Totals != nil {
				totalSent += resp.Totals.TotalSent
				totalDelivered += resp.Totals.TotalDelivered
				totalFailed += resp.Totals.TotalFailed
			}
		}
		rate := float64(0)
		if totalSent > 0 {
			rate = float64(totalDelivered) / float64(totalSent) * 100
		}
		mu.Lock()
		traffic = trafficData{
			TotalSent:      totalSent,
			TotalDelivered: totalDelivered,
			TotalFailed:    totalFailed,
			DeliveryRate:   rate,
		}
		mu.Unlock()
	}()

	// --- 4. Moderation counts ---
	type moderationData struct {
		SenderNames   int `json:"sender_names"`
		Templates     int `json:"templates"`
		Registrations int `json:"registrations"`
	}
	var moderation moderationData

	wg.Add(1)
	go func() {
		defer wg.Done()
		h.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM sender_names sn JOIN clients c ON c.id = sn.client_id WHERE c.parent_client_id = $1 AND sn.status = 'pending'`,
			resellerID,
		).Scan(&moderation.SenderNames)

		h.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM templates t JOIN clients c ON c.id = t.client_id WHERE c.parent_client_id = $1 AND (t.status = 'pending' OR t.status = 'revision_requested')`,
			resellerID,
		).Scan(&moderation.Templates)

		h.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM operator_registrations or2 JOIN sender_names sn ON sn.id = or2.sender_name_id JOIN clients c ON c.id = sn.client_id WHERE c.parent_client_id = $1 AND or2.status = 'submitted'`,
			resellerID,
		).Scan(&moderation.Registrations)
	}()

	// --- 5. Top 5 sub-accounts by traffic (current month) ---
	type topSubAccount struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		TotalSent    int64   `json:"total_sent"`
		DeliveryRate float64 `json:"delivery_rate"`
	}
	var topSubAccounts []topSubAccount

	wg.Add(1)
	go func() {
		defer wg.Done()
		if h.analyticsClient == nil || len(subIDs) == 0 {
			return
		}
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		type saStats struct {
			id        string
			sent      int64
			delivered int64
		}
		allStats := make([]saStats, 0, len(subIDs))
		for _, sid := range subIDs {
			resp, err := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
				ClientId: sid,
				From:     timestamppb.New(monthStart),
				To:       timestamppb.New(now),
				GroupBy:  "day",
			})
			if err != nil || resp.Totals == nil {
				continue
			}
			if resp.Totals.TotalSent > 0 {
				allStats = append(allStats, saStats{id: sid, sent: resp.Totals.TotalSent, delivered: resp.Totals.TotalDelivered})
			}
		}
		for i := 0; i < len(allStats) && i < 5; i++ {
			maxIdx := i
			for j := i + 1; j < len(allStats); j++ {
				if allStats[j].sent > allStats[maxIdx].sent {
					maxIdx = j
				}
			}
			allStats[i], allStats[maxIdx] = allStats[maxIdx], allStats[i]
		}
		limit := 5
		if len(allStats) < limit {
			limit = len(allStats)
		}
		mu.Lock()
		topSubAccounts = make([]topSubAccount, limit)
		for i := 0; i < limit; i++ {
			rate := float64(0)
			if allStats[i].sent > 0 {
				rate = float64(allStats[i].delivered) / float64(allStats[i].sent) * 100
			}
			topSubAccounts[i] = topSubAccount{
				ID:           allStats[i].id,
				Name:         subNames[allStats[i].id],
				TotalSent:    allStats[i].sent,
				DeliveryRate: rate,
			}
		}
		mu.Unlock()
	}()

	// --- 6. Problem sub-accounts ---
	type problemAlert struct {
		SubAccountID string `json:"sub_account_id"`
		Name         string `json:"name"`
		Type         string `json:"type"`
		Detail       string `json:"detail"`
	}
	var problems []problemAlert

	wg.Add(1)
	go func() {
		defer wg.Done()
		localProblems := make([]problemAlert, 0)

		rows, err := h.pool.Query(ctx,
			`SELECT c.id, c.name, a.balance::text
			 FROM clients c
			 JOIN accounts a ON a.client_id = c.id
			 WHERE c.parent_client_id = $1 AND c.active = true AND a.balance < 100
			 ORDER BY a.balance ASC LIMIT 10`,
			resellerID,
		)
		if err == nil {
			for rows.Next() {
				var id, name, bal string
				if rows.Scan(&id, &name, &bal) == nil {
					balFloat, _ := strconv.ParseFloat(bal, 64)
					localProblems = append(localProblems, problemAlert{
						SubAccountID: id,
						Name:         name,
						Type:         "low_balance",
						Detail:       fmt.Sprintf("Баланс: %.2f ₽", balFloat),
					})
				}
			}
			rows.Close()
		}

		rows2, err := h.pool.Query(ctx,
			`SELECT c.id, c.name, c.daily_limit, c.messages_today, c.monthly_limit, c.messages_this_month
			 FROM clients c
			 WHERE c.parent_client_id = $1 AND c.active = true
			   AND ((c.daily_limit > 0 AND c.messages_today >= c.daily_limit)
			    OR  (c.monthly_limit > 0 AND c.messages_this_month >= c.monthly_limit))
			 LIMIT 10`,
			resellerID,
		)
		if err == nil {
			for rows2.Next() {
				var id, name string
				var dailyLimit, messagesToday, monthlyLimit, messagesMonth int32
				if rows2.Scan(&id, &name, &dailyLimit, &messagesToday, &monthlyLimit, &messagesMonth) == nil {
					detail := ""
					if dailyLimit > 0 && messagesToday >= dailyLimit {
						detail = "дневной лимит исчерпан"
					} else {
						detail = "месячный лимит исчерпан"
					}
					localProblems = append(localProblems, problemAlert{
						SubAccountID: id,
						Name:         name,
						Type:         "limit_exhausted",
						Detail:       detail,
					})
				}
			}
			rows2.Close()
		}

		mu.Lock()
		problems = localProblems
		mu.Unlock()
	}()

	wg.Wait()

	if topSubAccounts == nil {
		topSubAccounts = []topSubAccount{}
	}
	if problems == nil {
		problems = []problemAlert{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"network_balance":      balances,
		"traffic":              traffic,
		"moderation_counts":    moderation,
		"top_sub_accounts":     topSubAccounts,
		"problem_sub_accounts": problems,
		"period":               period,
		"revenue": map[string]interface{}{
			"available": false,
			"message":   "Доступно после настройки тарифов",
		},
	})
}
