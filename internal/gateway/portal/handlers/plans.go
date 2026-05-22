package handlers

import (
	"net/http"

	"github.com/rs/zerolog/log"

	clientv1 "github.com/smpp-server/smpp-server/api/proto/clientv1"
)

// PlansHandlers содержит handlers для публичных эндпоинтов тарифных планов
type PlansHandlers struct {
	clientService clientv1.ClientServiceClient
}

// NewPlansHandlers создает новый PlansHandlers
func NewPlansHandlers(client clientv1.ClientServiceClient) *PlansHandlers {
	return &PlansHandlers{clientService: client}
}

// ListPlans обрабатывает GET /portal/v1/plans — публичный эндпоинт, auth не требуется
func (h *PlansHandlers) ListPlans(w http.ResponseWriter, r *http.Request) {
	resp, err := h.clientService.ListPlans(r.Context(), &clientv1.ListPlansRequest{})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка тарифных планов")
		respondGRPCError(w, err)
		return
	}

	plans := make([]map[string]interface{}, 0, len(resp.Plans))
	for _, p := range resp.Plans {
		plans = append(plans, map[string]interface{}{
			"id":                   p.Id,
			"name":                 p.Name,
			"display_name":         p.DisplayName,
			"monthly_price_rub":    p.MonthlyPriceRub,
			"max_sms_per_month":    p.MaxSmsPerMonth,
			"max_smpp_connections": p.MaxSmppConnections,
			"max_users":            p.MaxUsers,
			"features":             p.Features,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"plans": plans,
	})
}
