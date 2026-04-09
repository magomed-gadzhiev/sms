package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// TarificationHandler обрабатывает HTTP запросы для тарификации
type TarificationHandler struct {
	tarificationClient tarificationv1.TarificationServiceClient
	db                 *storage.DB
}

// NewTarificationHandler создает новый экземпляр TarificationHandler
func NewTarificationHandler(tarificationClient tarificationv1.TarificationServiceClient, db *storage.DB) *TarificationHandler {
	return &TarificationHandler{
		tarificationClient: tarificationClient,
		db:                 db,
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
	if req.OperatorID == "" || req.SenderCategory == "" || req.Strategy == "" {
		respondError(w, shared.ErrInvalidInput("operator_id, sender_category и strategy обязательны"))
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

	name := resp.GetSenderCategory() + " / " + resp.GetStrategy()
	respondJSON(w, http.StatusCreated, tariffPlanDTO{
		TariffPlanID:   resp.GetId(),
		Name:           name,
		OperatorID:     resp.GetOperatorId(),
		SenderCategory: resp.GetSenderCategory(),
		Strategy:       resp.GetStrategy(),
		Active:         resp.GetActive(),
	})
}

// tariffPlanDTO — формат тарифного плана для фронтенда
type tariffPlanDTO struct {
	TariffPlanID   string `json:"tariff_plan_id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	OperatorID     string `json:"operator_id"`
	SenderCategory string `json:"sender_category"`
	Strategy       string `json:"strategy"`
	Active         bool   `json:"active"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
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

	plans := make([]tariffPlanDTO, 0, len(resp.GetPlans()))
	for _, p := range resp.GetPlans() {
		name := p.GetSenderCategory() + " / " + p.GetStrategy()
		var createdAt, updatedAt string
		if ts := p.GetCreatedAt(); ts != nil {
			createdAt = ts.AsTime().Format(time.RFC3339)
		}
		if ts := p.GetUpdatedAt(); ts != nil {
			updatedAt = ts.AsTime().Format(time.RFC3339)
		}
		plans = append(plans, tariffPlanDTO{
			TariffPlanID:   p.GetId(),
			Name:           name,
			Description:    "",
			OperatorID:     p.GetOperatorId(),
			SenderCategory: p.GetSenderCategory(),
			Strategy:       p.GetStrategy(),
			Active:         p.GetActive(),
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tariff_plans": plans,
		"total":        resp.GetTotal(),
	})
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

// ListTariffPeriods обрабатывает GET /admin/v1/tarification/tariff-periods
func (h *TarificationHandler) ListTariffPeriods(w http.ResponseWriter, r *http.Request) {
	tariffPlanID := r.URL.Query().Get("tariff_plan_id")

	type periodRow struct {
		ID           string `json:"id"`
		TariffPlanID string `json:"tariff_plan_id"`
		StartDate    string `json:"start_date"`
		EndDate      string `json:"end_date"`
		CreatedAt    string `json:"created_at"`
	}

	var query string
	var args []interface{}
	if tariffPlanID != "" {
		query = `SELECT id::text, tariff_plan_id::text, start_date::text, end_date::text, created_at FROM tariff_periods WHERE tariff_plan_id = $1 ORDER BY start_date DESC`
		args = []interface{}{tariffPlanID}
	} else {
		query = `SELECT id::text, tariff_plan_id::text, start_date::text, end_date::text, created_at FROM tariff_periods ORDER BY start_date DESC`
	}

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения тарифных периодов")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	periods := make([]periodRow, 0)
	for rows.Next() {
		var p periodRow
		var createdAt time.Time
		if err := rows.Scan(&p.ID, &p.TariffPlanID, &p.StartDate, &p.EndDate, &createdAt); err != nil {
			log.Error().Err(err).Msg("ошибка чтения тарифного периода")
			continue
		}
		p.CreatedAt = createdAt.Format(time.RFC3339)
		periods = append(periods, p)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"periods": periods,
		"total":   len(periods),
	})
}

// ListTariffTiers обрабатывает GET /admin/v1/tarification/tariff-tiers
func (h *TarificationHandler) ListTariffTiers(w http.ResponseWriter, r *http.Request) {
	tariffPeriodID := r.URL.Query().Get("tariff_period_id")

	type tierRow struct {
		ID             string `json:"id"`
		TariffPeriodID string `json:"tariff_period_id"`
		FromCount      int    `json:"from_count"`
		PricePerSegment string `json:"price_per_segment"`
	}

	var query string
	var args []interface{}
	if tariffPeriodID != "" {
		query = `SELECT id::text, tariff_period_id::text, from_count, price_per_segment::text FROM tariff_tiers WHERE tariff_period_id = $1 ORDER BY from_count ASC`
		args = []interface{}{tariffPeriodID}
	} else {
		query = `SELECT id::text, tariff_period_id::text, from_count, price_per_segment::text FROM tariff_tiers ORDER BY from_count ASC`
	}

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения тарифных тиров")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	tiers := make([]tierRow, 0)
	for rows.Next() {
		var t tierRow
		if err := rows.Scan(&t.ID, &t.TariffPeriodID, &t.FromCount, &t.PricePerSegment); err != nil {
			log.Error().Err(err).Msg("ошибка чтения тарифного тира")
			continue
		}
		tiers = append(tiers, t)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tiers": tiers,
		"total": len(tiers),
	})
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
