package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// OperatorHandler обрабатывает HTTP запросы для управления операторами
type OperatorHandler struct {
	routingClient routingv1.RoutingServiceClient
}

// NewOperatorHandler создает новый экземпляр OperatorHandler
func NewOperatorHandler(routingClient routingv1.RoutingServiceClient) *OperatorHandler {
	return &OperatorHandler{
		routingClient: routingClient,
	}
}

// CreateOperator обрабатывает POST /admin/v1/operators
func (h *OperatorHandler) CreateOperator(w http.ResponseWriter, r *http.Request) {
	var req CreateOperatorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, err.(*shared.AppError))
		return
	}

	grpcReq := &routingv1.CreateOperatorRequest{
		CountryId:           req.CountryID,
		Name:                req.Name,
		Code:                req.Code,
		SupportsPaidSender:  req.SupportsPaidSender,
		SupportsFreeSender:  req.SupportsFreeSender,
		MonthlyTariffAmount: req.MonthlyTariffAmount,
	}

	resp, err := h.routingClient.CreateOperator(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания оператора")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, operatorProtoToResponse(resp))
}

// GetOperator обрабатывает GET /admin/v1/operators/{id}
func (h *OperatorHandler) GetOperator(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("id обязателен"))
		return
	}

	resp, err := h.routingClient.GetOperator(r.Context(), &routingv1.GetOperatorRequest{
		Id: id,
	})
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("ошибка получения оператора")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, operatorProtoToResponse(resp))
}

// ListOperators обрабатывает GET /admin/v1/operators
func (h *OperatorHandler) ListOperators(w http.ResponseWriter, r *http.Request) {
	countryID := r.URL.Query().Get("country_id")
	activeOnly := r.URL.Query().Get("active_only") == "true"
	limit := parseInt(r.URL.Query().Get("limit"), 50)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	resp, err := h.routingClient.ListOperators(r.Context(), &routingv1.ListOperatorsRequest{
		CountryId:  countryID,
		ActiveOnly: activeOnly,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка операторов")
		respondGRPCError(w, err)
		return
	}

	operators := make([]OperatorResponse, len(resp.Operators))
	for i, op := range resp.Operators {
		operators[i] = operatorProtoToResponse(op)
	}

	respondJSON(w, http.StatusOK, ListOperatorsResponse{
		Operators: operators,
		Total:     int(resp.Total),
		Limit:     limit,
		Offset:    offset,
	})
}

// UpdateOperator обрабатывает PUT /admin/v1/operators/{id}
func (h *OperatorHandler) UpdateOperator(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("id обязателен"))
		return
	}

	var req UpdateOperatorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	grpcReq := &routingv1.UpdateOperatorRequest{
		Id:                  id,
		Name:                req.Name,
		Code:                req.Code,
		SupportsPaidSender:  req.SupportsPaidSender,
		SupportsFreeSender:  req.SupportsFreeSender,
		Active:              req.Active,
		MonthlyTariffAmount: req.MonthlyTariffAmount,
	}

	resp, err := h.routingClient.UpdateOperator(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("ошибка обновления оператора")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, operatorProtoToResponse(resp))
}

// CreateOperatorPrefix обрабатывает POST /admin/v1/operators/{id}/prefixes
func (h *OperatorHandler) CreateOperatorPrefix(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	operatorID := vars["id"]
	if operatorID == "" {
		respondError(w, shared.ErrInvalidInput("operator id обязателен"))
		return
	}

	var req CreateOperatorPrefixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Prefix == "" {
		respondError(w, shared.ErrInvalidInput("prefix обязателен"))
		return
	}

	grpcReq := &routingv1.CreateOperatorPrefixRequest{
		OperatorId: operatorID,
		Prefix:     req.Prefix,
		Priority:   req.Priority,
	}

	resp, err := h.routingClient.CreateOperatorPrefix(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Str("operator_id", operatorID).Msg("ошибка создания префикса")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, operatorPrefixProtoToResponse(resp))
}

// ListOperatorPrefixes обрабатывает GET /admin/v1/operators/{id}/prefixes
func (h *OperatorHandler) ListOperatorPrefixes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	operatorID := vars["id"]
	if operatorID == "" {
		respondError(w, shared.ErrInvalidInput("operator id обязателен"))
		return
	}

	resp, err := h.routingClient.ListOperatorPrefixes(r.Context(), &routingv1.ListOperatorPrefixesRequest{
		OperatorId: operatorID,
	})
	if err != nil {
		log.Error().Err(err).Str("operator_id", operatorID).Msg("ошибка получения списка префиксов")
		respondGRPCError(w, err)
		return
	}

	prefixes := make([]OperatorPrefixResponse, len(resp.Prefixes))
	for i, p := range resp.Prefixes {
		prefixes[i] = operatorPrefixProtoToResponse(p)
	}

	respondJSON(w, http.StatusOK, ListOperatorPrefixesResponse{
		Prefixes: prefixes,
	})
}

// DeleteOperatorPrefix обрабатывает DELETE /admin/v1/operators/{id}/prefixes/{prefix_id}
func (h *OperatorHandler) DeleteOperatorPrefix(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	prefixID := vars["prefix_id"]
	if prefixID == "" {
		respondError(w, shared.ErrInvalidInput("prefix_id обязателен"))
		return
	}

	resp, err := h.routingClient.DeleteOperatorPrefix(r.Context(), &routingv1.DeleteOperatorPrefixRequest{
		Id: prefixID,
	})
	if err != nil {
		log.Error().Err(err).Str("prefix_id", prefixID).Msg("ошибка удаления префикса")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, DeleteOperatorPrefixResponse{
		Success: resp.Success,
	})
}

// Типы запросов и ответов

type CreateOperatorRequest struct {
	CountryID           string `json:"country_id"`
	Name                string `json:"name"`
	Code                string `json:"code"`
	SupportsPaidSender  bool   `json:"supports_paid_sender"`
	SupportsFreeSender  bool   `json:"supports_free_sender"`
	MonthlyTariffAmount string `json:"monthly_tariff_amount,omitempty"`
}

func (r *CreateOperatorRequest) Validate() error {
	if r.CountryID == "" {
		return shared.ErrInvalidInput("country_id обязателен")
	}
	if r.Name == "" {
		return shared.ErrInvalidInput("name обязателен")
	}
	if r.Code == "" {
		return shared.ErrInvalidInput("code обязателен")
	}
	return nil
}

type UpdateOperatorRequest struct {
	Name                string `json:"name,omitempty"`
	Code                string `json:"code,omitempty"`
	SupportsPaidSender  bool   `json:"supports_paid_sender"`
	SupportsFreeSender  bool   `json:"supports_free_sender"`
	Active              bool   `json:"active"`
	MonthlyTariffAmount string `json:"monthly_tariff_amount,omitempty"`
}

type OperatorResponse struct {
	ID                  string    `json:"id"`
	CountryID           string    `json:"country_id"`
	Name                string    `json:"name"`
	Code                string    `json:"code"`
	SupportsPaidSender  bool      `json:"supports_paid_sender"`
	SupportsFreeSender  bool      `json:"supports_free_sender"`
	MonthlyTariffAmount string    `json:"monthly_tariff_amount,omitempty"`
	Active              bool      `json:"active"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type ListOperatorsResponse struct {
	Operators []OperatorResponse `json:"operators"`
	Total     int                `json:"total"`
	Limit     int                `json:"limit"`
	Offset    int                `json:"offset"`
}

type CreateOperatorPrefixRequest struct {
	Prefix   string `json:"prefix"`
	Priority int32  `json:"priority"`
}

type OperatorPrefixResponse struct {
	ID         string    `json:"id"`
	OperatorID string    `json:"operator_id"`
	Prefix     string    `json:"prefix"`
	Priority   int32     `json:"priority"`
	CreatedAt  time.Time `json:"created_at"`
}

type ListOperatorPrefixesResponse struct {
	Prefixes []OperatorPrefixResponse `json:"prefixes"`
}

type DeleteOperatorPrefixResponse struct {
	Success bool `json:"success"`
}

func operatorProtoToResponse(op *routingv1.Operator) OperatorResponse {
	resp := OperatorResponse{
		ID:                  op.Id,
		CountryID:           op.CountryId,
		Name:                op.Name,
		Code:                op.Code,
		SupportsPaidSender:  op.SupportsPaidSender,
		SupportsFreeSender:  op.SupportsFreeSender,
		MonthlyTariffAmount: op.MonthlyTariffAmount,
		Active:              op.Active,
	}
	if op.CreatedAt != nil {
		resp.CreatedAt = op.CreatedAt.AsTime()
	}
	if op.UpdatedAt != nil {
		resp.UpdatedAt = op.UpdatedAt.AsTime()
	}
	return resp
}

func operatorPrefixProtoToResponse(p *routingv1.OperatorPrefix) OperatorPrefixResponse {
	resp := OperatorPrefixResponse{
		ID:         p.Id,
		OperatorID: p.OperatorId,
		Prefix:     p.Prefix,
		Priority:   p.Priority,
	}
	if p.CreatedAt != nil {
		resp.CreatedAt = p.CreatedAt.AsTime()
	}
	return resp
}
