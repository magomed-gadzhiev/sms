package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// TarificationHandler обрабатывает HTTP запросы для тарификации
type TarificationHandler struct {
	tarificationClient tarificationv1.TarificationServiceClient
}

// NewTarificationHandler создает новый экземпляр TarificationHandler
func NewTarificationHandler(tarificationClient tarificationv1.TarificationServiceClient) *TarificationHandler {
	return &TarificationHandler{
		tarificationClient: tarificationClient,
	}
}

// CreateSenderRegistration обрабатывает POST /admin/v1/tarification/sender-registrations
func (h *TarificationHandler) CreateSenderRegistration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID   string `json:"client_id"`
		OperatorID string `json:"operator_id"`
		SenderName string `json:"sender_name"`
		Type       string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.tarificationClient.CreateSenderRegistration(r.Context(), &tarificationv1.CreateSenderRegistrationRequest{
		ClientId:   req.ClientID,
		OperatorId: req.OperatorID,
		SenderName: req.SenderName,
		Type:       req.Type,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания регистрации имени отправителя")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// ListSenderRegistrations обрабатывает GET /admin/v1/tarification/sender-registrations
func (h *TarificationHandler) ListSenderRegistrations(w http.ResponseWriter, r *http.Request) {
	resp, err := h.tarificationClient.ListSenderRegistrations(r.Context(), &tarificationv1.ListSenderRegistrationsRequest{
		ClientId:   r.URL.Query().Get("client_id"),
		OperatorId: r.URL.Query().Get("operator_id"),
		Limit:      parseIntParam(r, "limit", 50),
		Offset:     parseIntParam(r, "offset", 0),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// UpdateSenderRegistration обрабатывает PUT /admin/v1/tarification/sender-registrations/{id}
func (h *TarificationHandler) UpdateSenderRegistration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		Status string `json:"status"`
		Type   string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.tarificationClient.UpdateSenderRegistration(r.Context(), &tarificationv1.UpdateSenderRegistrationRequest{
		Id:     id,
		Status: req.Status,
		Type:   req.Type,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// CreateTariffPlan обрабатывает POST /admin/v1/tarification/tariff-plans
func (h *TarificationHandler) CreateTariffPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OperatorID     string `json:"operator_id"`
		SenderCategory string `json:"sender_category"`
		Strategy       string `json:"strategy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.tarificationClient.CreateTariffPlan(r.Context(), &tarificationv1.CreateTariffPlanRequest{
		OperatorId:     req.OperatorID,
		SenderCategory: req.SenderCategory,
		Strategy:       req.Strategy,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания тарифного плана")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// ListTariffPlans обрабатывает GET /admin/v1/tarification/tariff-plans
func (h *TarificationHandler) ListTariffPlans(w http.ResponseWriter, r *http.Request) {
	resp, err := h.tarificationClient.ListTariffPlans(r.Context(), &tarificationv1.ListTariffPlansRequest{
		OperatorId: r.URL.Query().Get("operator_id"),
		ActiveOnly: r.URL.Query().Get("active_only") == "true",
		Limit:      parseIntParam(r, "limit", 50),
		Offset:     parseIntParam(r, "offset", 0),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// UpdateTariffPlan обрабатывает PUT /admin/v1/tarification/tariff-plans/{id}
func (h *TarificationHandler) UpdateTariffPlan(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.tarificationClient.UpdateTariffPlan(r.Context(), &tarificationv1.UpdateTariffPlanRequest{
		Id:     id,
		Active: req.Active,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// CreateTariffPeriod обрабатывает POST /admin/v1/tarification/tariff-periods
func (h *TarificationHandler) CreateTariffPeriod(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TariffPlanID string `json:"tariff_plan_id"`
		StartDate    string `json:"start_date"`
		EndDate      string `json:"end_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.tarificationClient.CreateTariffPeriod(r.Context(), &tarificationv1.CreateTariffPeriodRequest{
		TariffPlanId: req.TariffPlanID,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// CreateTariffTier обрабатывает POST /admin/v1/tarification/tariff-tiers
func (h *TarificationHandler) CreateTariffTier(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TariffPeriodID  string `json:"tariff_period_id"`
		FromCount       int32  `json:"from_count"`
		PricePerSegment string `json:"price_per_segment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.tarificationClient.CreateTariffTier(r.Context(), &tarificationv1.CreateTariffTierRequest{
		TariffPeriodId:  req.TariffPeriodID,
		FromCount:       req.FromCount,
		PricePerSegment: req.PricePerSegment,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// UpdateTariffTier обрабатывает PUT /admin/v1/tarification/tariff-tiers/{id}
func (h *TarificationHandler) UpdateTariffTier(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		FromCount       int32  `json:"from_count"`
		PricePerSegment string `json:"price_per_segment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.tarificationClient.UpdateTariffTier(r.Context(), &tarificationv1.UpdateTariffTierRequest{
		Id:              id,
		FromCount:       req.FromCount,
		PricePerSegment: req.PricePerSegment,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// CreatePricingPeriod обрабатывает POST /admin/v1/tarification/pricing-periods
func (h *TarificationHandler) CreatePricingPeriod(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TariffPeriodID string `json:"tariff_period_id"`
		StartDate      string `json:"start_date"`
		EndDate        string `json:"end_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.tarificationClient.CreatePricingPeriod(r.Context(), &tarificationv1.CreatePricingPeriodRequest{
		TariffPeriodId: req.TariffPeriodID,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// CreatePrepaidFee обрабатывает POST /admin/v1/tarification/prepaid-fees
func (h *TarificationHandler) CreatePrepaidFee(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TariffPlanID   string `json:"tariff_plan_id"`
		TariffPeriodID string `json:"tariff_period_id"`
		Amount         string `json:"amount"`
		Currency       string `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
		return
	}

	resp, err := h.tarificationClient.CreatePrepaidFee(r.Context(), &tarificationv1.CreatePrepaidFeeRequest{
		TariffPlanId:   req.TariffPlanID,
		TariffPeriodId: req.TariffPeriodID,
		Amount:         req.Amount,
		Currency:       req.Currency,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// ListUsageCounters обрабатывает GET /admin/v1/tarification/usage
func (h *TarificationHandler) ListUsageCounters(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	resp, err := h.tarificationClient.ListUsageCounters(r.Context(), &tarificationv1.ListUsageCountersRequest{
		ClientId:     clientID,
		TariffPlanId: r.URL.Query().Get("tariff_plan_id"),
		Limit:        parseIntParam(r, "limit", 50),
		Offset:       parseIntParam(r, "offset", 0),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}
