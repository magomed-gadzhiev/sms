package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	clientv1 "github.com/smpp-server/smpp-server/api/proto/clientv1"
	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// TariffHandlers содержит handlers для управления тарифами
type TariffHandlers struct {
	clientClient       clientv1.ClientServiceClient
	tarificationClient tarificationv1.TarificationServiceClient
	billingClient      billingv1.BillingServiceClient
}

// NewTariffHandlers создает новый TariffHandlers
func NewTariffHandlers(clientClient clientv1.ClientServiceClient, tarificationClient tarificationv1.TarificationServiceClient, billingClient billingv1.BillingServiceClient) *TariffHandlers {
	return &TariffHandlers{clientClient: clientClient, tarificationClient: tarificationClient, billingClient: billingClient}
}

// GetCurrentTariff обрабатывает GET /portal/v1/tariffs/current
func (h *TariffHandlers) GetCurrentTariff(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	clientResp, err := h.clientClient.GetClient(r.Context(), &clientv1.GetClientRequest{ClientId: clientID.String()})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения информации о клиенте")
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"client_id":         clientResp.Client.ClientId,
		"plan_id":           clientResp.Client.PlanId,
		"monthly_sms_count": clientResp.Client.MonthlySmsCount,
	}

	if clientResp.Client.Plan != nil {
		result["plan"] = map[string]interface{}{
			"id":                   clientResp.Client.Plan.Id,
			"name":                 clientResp.Client.Plan.Name,
			"display_name":         clientResp.Client.Plan.DisplayName,
			"monthly_price_rub":    clientResp.Client.Plan.MonthlyPriceRub,
			"max_sms_per_month":    clientResp.Client.Plan.MaxSmsPerMonth,
			"max_smpp_connections": clientResp.Client.Plan.MaxSmppConnections,
			"max_users":            clientResp.Client.Plan.MaxUsers,
			"features":             clientResp.Client.Plan.Features,
		}
	}

	if clientResp.Client.PlanId != "" && h.tarificationClient != nil {
		usageResp, err := h.tarificationClient.ListUsageCounters(r.Context(), &tarificationv1.ListUsageCountersRequest{
			ClientId: clientID.String(),
			Limit:    100,
			Offset:   0,
		})
		if err == nil {
			counters := make([]map[string]interface{}, 0, len(usageResp.Counters))
			for _, c := range usageResp.Counters {
				counter := map[string]interface{}{
					"id":              c.Id,
					"tariff_plan_id":  c.TariffPlanId,
					"segment_count":   c.SegmentCount,
				}
				if c.UpdatedAt != nil {
					counter["updated_at"] = c.UpdatedAt.AsTime()
				}
				counters = append(counters, counter)
			}
			result["usage_counters"] = counters
		}
	}

	respondJSON(w, http.StatusOK, result)
}

// ListAvailablePlans обрабатывает GET /portal/v1/tariffs/plans
func (h *TariffHandlers) ListAvailablePlans(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	resp, err := h.clientClient.ListPlans(r.Context(), &clientv1.ListPlansRequest{})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка планов")
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

	respondJSON(w, http.StatusOK, map[string]interface{}{"plans": plans})
}

// changePlanRequest представляет запрос на смену тарифного плана
type changePlanRequest struct {
	PlanID string `json:"plan_id"`
}

// ChangePlan обрабатывает POST /portal/v1/tariffs/change
func (h *TariffHandlers) ChangePlan(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req changePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.PlanID == "" {
		respondError(w, shared.ErrInvalidInput("Поле plan_id обязательно"))
		return
	}

	clientResp, err := h.clientClient.GetClient(r.Context(), &clientv1.GetClientRequest{ClientId: clientID.String()})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения информации о клиенте при смене тарифа")
		respondGRPCError(w, err)
		return
	}
	if clientResp.GetClient().GetParentClientId() != "" {
		respondError(w, shared.ErrForbidden("Суб-аккаунт не может менять тарифный план — он управляется агрегатором"))
		return
	}

	// Получаем информацию о новом плане для списания средств
	plansResp, err := h.clientClient.ListPlans(r.Context(), &clientv1.ListPlansRequest{})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка планов")
		respondGRPCError(w, err)
		return
	}

	var selectedPlan *clientv1.SubscriptionPlan
	for _, p := range plansResp.Plans {
		if p.Id == req.PlanID {
			selectedPlan = p
			break
		}
	}
	if selectedPlan == nil {
		respondError(w, shared.ErrNotFound("Тарифный план не найден"))
		return
	}

	// Списываем стоимость тарифного плана (если цена > 0 и billingClient доступен)
	if h.billingClient != nil && selectedPlan.MonthlyPriceRub > 0 {
		amount := fmt.Sprintf("%.2f", selectedPlan.MonthlyPriceRub)
		_, err := h.billingClient.DeductCredits(r.Context(), &billingv1.DeductCreditsRequest{
			ClientId:    clientID.String(),
			Amount:      amount,
			Currency:    "RUB",
			Description: fmt.Sprintf("Подключение тарифного плана «%s»", selectedPlan.DisplayName),
		})
		if err != nil {
			log.Error().Err(err).Str("plan_id", req.PlanID).Str("amount", amount).Msg("ошибка списания средств за тарифный план")
			respondGRPCError(w, err)
			return
		}
	}

	_, err = h.clientClient.AssignPlan(r.Context(), &clientv1.AssignPlanRequest{
		ClientId: clientID.String(),
		PlanId:   req.PlanID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"message": "Тарифный план успешно изменён"})
}

// GetUsage обрабатывает GET /portal/v1/tariffs/usage
func (h *TariffHandlers) GetUsage(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	if h.tarificationClient == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"counters": []interface{}{}})
		return
	}

	resp, err := h.tarificationClient.ListUsageCounters(r.Context(), &tarificationv1.ListUsageCountersRequest{
		ClientId: clientID.String(),
		Limit:    100,
		Offset:   0,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения usage counters")
		respondGRPCError(w, err)
		return
	}

	counters := make([]map[string]interface{}, 0, len(resp.Counters))
	for _, c := range resp.Counters {
		counter := map[string]interface{}{
			"id":             c.Id,
			"client_id":      c.ClientId,
			"tariff_plan_id": c.TariffPlanId,
			"segment_count":  c.SegmentCount,
		}
		if c.UpdatedAt != nil {
			counter["updated_at"] = c.UpdatedAt.AsTime()
		}
		counters = append(counters, counter)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"counters": counters, "total": resp.Total})
}
