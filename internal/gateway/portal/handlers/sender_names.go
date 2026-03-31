package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type SenderNameHandlers struct {
	client         sendernamev1.SenderNameServiceClient
	routingClient  routingv1.RoutingServiceClient
	tariffClient   tarificationv1.TarificationServiceClient
}

func NewSenderNameHandlers(client sendernamev1.SenderNameServiceClient) *SenderNameHandlers {
	return &SenderNameHandlers{client: client}
}

// SetBillingClients устанавливает gRPC клиенты для тарификации
func (h *SenderNameHandlers) SetBillingClients(routingClient routingv1.RoutingServiceClient, tariffClient tarificationv1.TarificationServiceClient) {
	h.routingClient = routingClient
	h.tariffClient = tariffClient
}

type createSenderNameRequest struct {
	Name string `json:"name"`
}

func (h *SenderNameHandlers) CreateSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var req createSenderNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}
	resp, err := h.client.CreateSenderName(r.Context(), &sendernamev1.CreateSenderNameRequest{
		ClientId: clientID.String(),
		Name:     req.Name,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания имени отправителя")
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, senderNameToJSON(resp.SenderName))
}

func (h *SenderNameHandlers) ListSenderNames(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	statusFilter := r.URL.Query().Get("status")

	resp, err := h.client.ListSenderNames(r.Context(), &sendernamev1.ListSenderNamesRequest{
		ClientId: clientID.String(),
		Status:   statusFilter,
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка имён отправителей")
		respondGRPCError(w, err)
		return
	}

	items := make([]map[string]interface{}, 0, len(resp.SenderNames))
	for _, sn := range resp.SenderNames {
		items = append(items, senderNameToJSON(sn))
	}
	totalPages := int32(0)
	if perPage > 0 && resp.Total > 0 {
		totalPages = (resp.Total + perPage - 1) / perPage
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sender_names": items,
		"total":        resp.Total,
		"page":         page,
		"per_page":     perPage,
		"total_pages":  totalPages,
	})
}

func (h *SenderNameHandlers) GetSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	resp, err := h.client.GetSenderName(r.Context(), &sendernamev1.GetSenderNameRequest{
		Id: id, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, senderNameToJSON(resp.SenderName))
}

type updateSenderNameRequest struct {
	Name string `json:"name"`
}

func (h *SenderNameHandlers) UpdateSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	var req updateSenderNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}
	resp, err := h.client.UpdateSenderName(r.Context(), &sendernamev1.UpdateSenderNameRequest{
		Id: id, ClientId: clientID.String(), Name: req.Name,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, senderNameToJSON(resp.SenderName))
}

func (h *SenderNameHandlers) ResubmitSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	resp, err := h.client.ResubmitSenderName(r.Context(), &sendernamev1.ResubmitSenderNameRequest{
		Id: id, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, senderNameToJSON(resp.SenderName))
}

func (h *SenderNameHandlers) GetSenderNameHistory(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage

	resp, err := h.client.GetSenderNameHistory(r.Context(), &sendernamev1.GetSenderNameHistoryRequest{
		SenderNameId: id,
		ClientId:     clientID.String(),
		Limit:        perPage,
		Offset:       offset,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	entries := make([]map[string]interface{}, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		entries = append(entries, historyEntryToJSON(e))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   resp.Total,
	})
}

func senderNameToJSON(sn *sendernamev1.SenderNameInfo) map[string]interface{} {
	m := map[string]interface{}{
		"id":               sn.Id,
		"client_id":        sn.ClientId,
		"name":             sn.Name,
		"status":           sn.Status,
		"rejection_reason": sn.RejectionReason,
		"reviewer_id":      sn.ReviewerId,
	}
	if sn.ReviewedAt != nil {
		m["reviewed_at"] = sn.ReviewedAt.AsTime()
	} else {
		m["reviewed_at"] = nil
	}
	if sn.CreatedAt != nil {
		m["created_at"] = sn.CreatedAt.AsTime()
	}
	if sn.UpdatedAt != nil {
		m["updated_at"] = sn.UpdatedAt.AsTime()
	}
	return m
}

// CreateSenderRegistration создаёт регистрацию имени отправителя у оператора.
// POST /api/sender-registrations
// Для платных регистраций (type=paid) автоматически создаёт billing record за текущий месяц.
func (h *SenderNameHandlers) CreateSenderRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req struct {
		OperatorID string `json:"operator_id"`
		SenderName string `json:"sender_name"`
		Type       string `json:"type"` // "paid" | "free"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.OperatorID == "" {
		respondError(w, shared.ErrInvalidInput("operator_id обязателен"))
		return
	}
	if req.SenderName == "" {
		respondError(w, shared.ErrInvalidInput("sender_name обязателен"))
		return
	}
	if req.Type == "" {
		req.Type = "free"
	}

	if h.tariffClient == nil {
		respondError(w, shared.ErrInternalServer("tarification service недоступен"))
		return
	}

	regResp, err := h.tariffClient.CreateSenderRegistration(r.Context(), &tarificationv1.CreateSenderRegistrationRequest{
		ClientId:   clientID.String(),
		OperatorId: req.OperatorID,
		SenderName: req.SenderName,
		Type:       req.Type,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания sender registration")
		respondGRPCError(w, err)
		return
	}

	// Для платных регистраций создаём billing record за текущий месяц
	if req.Type == "paid" && h.routingClient != nil {
		op, err := h.routingClient.GetOperator(r.Context(), &routingv1.GetOperatorRequest{Id: req.OperatorID})
		if err != nil {
			log.Error().Err(err).Str("operator_id", req.OperatorID).Msg("не удалось получить тариф оператора для billing")
		} else if op.MonthlyTariffAmount != "" {
			_, billingErr := h.tariffClient.CreateSenderBillingRecord(r.Context(), &tarificationv1.CreateSenderBillingRecordRequest{
				SenderRegistrationId: regResp.Id,
				ClientId:             clientID.String(),
				OperatorId:           req.OperatorID,
				Amount:               op.MonthlyTariffAmount,
			})
			if billingErr != nil {
				log.Error().Err(billingErr).Str("registration_id", regResp.Id).Msg("не удалось создать billing record")
			}
		}
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":          regResp.Id,
		"client_id":   regResp.ClientId,
		"operator_id": regResp.OperatorId,
		"sender_name": regResp.SenderName,
		"type":        regResp.Type,
		"status":      regResp.Status,
	})
}

// GetOperatorSenderTariff возвращает тариф оператора для отображения перед регистрацией платного имени.
// GET /api/operators/:id/sender-tariff
func (h *SenderNameHandlers) GetOperatorSenderTariff(w http.ResponseWriter, r *http.Request) {
	if h.routingClient == nil {
		respondError(w, shared.ErrInternalServer("routing service недоступен"))
		return
	}
	operatorID := mux.Vars(r)["id"]
	if operatorID == "" {
		respondError(w, shared.ErrInvalidInput("operator id обязателен"))
		return
	}

	op, err := h.routingClient.GetOperator(r.Context(), &routingv1.GetOperatorRequest{Id: operatorID})
	if err != nil {
		log.Error().Err(err).Str("operator_id", operatorID).Msg("ошибка получения оператора")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"operator_id":            op.Id,
		"monthly_tariff_amount":  op.MonthlyTariffAmount,
		"currency":               "RUB",
		"current_month_amount":   op.MonthlyTariffAmount,
	})
}

// GetSenderRegistrationBilling возвращает историю начислений по регистрации.
// GET /api/sender-registrations/:id/billing
func (h *SenderNameHandlers) GetSenderRegistrationBilling(w http.ResponseWriter, r *http.Request) {
	if h.tariffClient == nil {
		respondError(w, shared.ErrInternalServer("tarification service недоступен"))
		return
	}
	regID := mux.Vars(r)["id"]
	if regID == "" {
		respondError(w, shared.ErrInvalidInput("registration id обязателен"))
		return
	}

	limit := 50
	offset := 0

	resp, err := h.tariffClient.ListSenderBillingRecords(r.Context(), &tarificationv1.ListSenderBillingRecordsRequest{
		SenderRegistrationId: regID,
		Limit:                int32(limit),
		Offset:               int32(offset),
	})
	if err != nil {
		log.Error().Err(err).Str("registration_id", regID).Msg("ошибка получения billing records")
		respondGRPCError(w, err)
		return
	}

	records := make([]map[string]interface{}, len(resp.Records))
	for i, rec := range resp.Records {
		r := map[string]interface{}{
			"id":            rec.Id,
			"billing_month": rec.BillingMonth,
			"amount":        rec.Amount,
		}
		if rec.CreatedAt != nil {
			r["created_at"] = rec.CreatedAt.AsTime()
		}
		records[i] = r
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"records": records,
		"total":   resp.Total,
	})
}

func historyEntryToJSON(e *sendernamev1.SenderNameHistoryEntry) map[string]interface{} {
	m := map[string]interface{}{
		"id":             e.Id,
		"sender_name_id": e.SenderNameId,
		"old_status":     e.OldStatus,
		"new_status":     e.NewStatus,
		"actor_id":       e.ActorId,
		"actor_type":     e.ActorType,
		"comment":        e.Comment,
	}
	if e.CreatedAt != nil {
		m["created_at"] = e.CreatedAt.AsTime()
	}
	return m
}
