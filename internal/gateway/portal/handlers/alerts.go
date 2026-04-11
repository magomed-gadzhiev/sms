package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	cpv1 "github.com/smpp-server/smpp-server/api/proto/clientproviderv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AlertsHandlers содержит handlers для умных оповещений Command Center
type AlertsHandlers struct {
	billingClient  billingv1.BillingServiceClient
	providerClient cpv1.ClientProviderServiceClient
	pool           *pgxpool.Pool
}

// NewAlertsHandlers создаёт AlertsHandlers
func NewAlertsHandlers(
	billingClient billingv1.BillingServiceClient,
	providerClient cpv1.ClientProviderServiceClient,
	pool *pgxpool.Pool,
) *AlertsHandlers {
	return &AlertsHandlers{
		billingClient:  billingClient,
		providerClient: providerClient,
		pool:           pool,
	}
}

type alertItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // critical | warning | success | info
	Title       string `json:"title"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// GetAlerts обрабатывает GET /portal/v1/alerts
// Возвращает до 5 актуальных оповещений: проблемы провайдеров, низкий баланс,
// последние системные уведомления.
func (h *AlertsHandlers) GetAlerts(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	ctx := r.Context()
	now := time.Now().UTC().Format(time.RFC3339)
	alerts := make([]alertItem, 0, 5)

	// 1. Check provider health for degraded providers
	// TODO: wire to provider metrics — ClientProvider proto does not expose SuccessRate;
	// once a metrics/health field is added to the proto, filter by success rate < 90%.
	if h.providerClient != nil {
		resp, err := h.providerClient.ListClientProviders(ctx, &cpv1.ListClientProvidersRequest{
			ClientId: clientID.String(),
		})
		if err != nil {
			log.Warn().Err(err).Msg("alerts: не удалось получить провайдеры")
		} else {
			for _, p := range resp.Providers {
				if !p.Active {
					alerts = append(alerts, alertItem{
						ID:          "provider-" + p.Id,
						Type:        "warning",
						Title:       "Провайдер неактивен",
						Description: fmt.Sprintf("Провайдер %s отключён", p.Name),
						CreatedAt:   now,
					})
				}
			}
		}
	}

	// 2. Check balance: if < 1000 → warning
	if h.billingClient != nil {
		resp, err := h.billingClient.GetBalance(ctx, &billingv1.GetBalanceRequest{
			ClientId: clientID.String(),
		})
		if err != nil {
			log.Warn().Err(err).Msg("alerts: не удалось получить баланс")
		} else {
			balance, _ := strconv.ParseFloat(resp.Balance, 64)
			if balance < 1000 {
				alerts = append(alerts, alertItem{
					ID:          "low-balance",
					Type:        "warning",
					Title:       "Низкий баланс",
					Description: fmt.Sprintf("Осталось %.2f %s", balance, resp.Currency),
					CreatedAt:   now,
				})
			}
		}
	}

	// 3. Recent unread notifications from DB (up to 3)
	if h.pool != nil {
		userID, ok := middleware.GetUserID(ctx)
		if ok {
			rows, err := h.pool.Query(ctx,
				`SELECT id, type, body, created_at FROM notifications
				 WHERE user_id = $1 AND is_read = false
				 ORDER BY created_at DESC LIMIT 3`,
				userID,
			)
			if err != nil {
				log.Warn().Err(err).Msg("alerts: не удалось получить уведомления")
			} else {
				func() {
					defer rows.Close()
					for rows.Next() {
						var id, typ, body string
						var createdAt interface{}
						if err := rows.Scan(&id, &typ, &body, &createdAt); err != nil {
							continue
						}
						ts := now
						if t, ok2 := createdAt.(interface{ Format(string) string }); ok2 {
							ts = t.Format(time.RFC3339)
						}
						aType := "info"
						switch typ {
						case "warning", "low_balance":
							aType = "warning"
						case "critical", "provider_degraded":
							aType = "critical"
						case "campaign_completed", "template_approved":
							aType = "success"
						}
						alerts = append(alerts, alertItem{
							ID:          "notif-" + id,
							Type:        aType,
							Title:       typ,
							Description: body,
							CreatedAt:   ts,
						})
					}
				}()
			}
		}
	}

	// Cap at 5
	if len(alerts) > 5 {
		alerts = alerts[:5]
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"items": alerts})
}
