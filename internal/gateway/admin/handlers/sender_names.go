package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type AdminSenderNameHandlers struct {
	client        sendernamev1.SenderNameServiceClient
	routingClient routingv1.RoutingServiceClient
	tariffClient  tarificationv1.TarificationServiceClient
}

func NewAdminSenderNameHandlers(client sendernamev1.SenderNameServiceClient) *AdminSenderNameHandlers {
	return &AdminSenderNameHandlers{client: client}
}

// SetClients устанавливает дополнительные gRPC-клиенты для обогащения данных.
func (h *AdminSenderNameHandlers) SetClients(
	routingClient routingv1.RoutingServiceClient,
	tariffClient tarificationv1.TarificationServiceClient,
) {
	h.routingClient = routingClient
	h.tariffClient = tariffClient
}

const (
	senderNameMaxListLimit  = 200
	senderNameMinReasonLen  = 3
	senderNameMaxReasonLen  = 500
)

func (h *AdminSenderNameHandlers) ListAllSenderNames(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 20)
	offset := parseIntParam(r, "offset", 0)
	if limit <= 0 {
		limit = 20
	}
	if limit > senderNameMaxListLimit {
		limit = senderNameMaxListLimit
	}
	if offset < 0 {
		offset = 0
	}

	resp, err := h.client.ListAllSenderNames(r.Context(), &sendernamev1.ListAllSenderNamesRequest{
		ClientId:  r.URL.Query().Get("client_id"),
		Status:    r.URL.Query().Get("status"),
		NameQuery: r.URL.Query().Get("name_query"),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка имён отправителей")
		respondGRPCError(w, err)
		return
	}

	items := make([]map[string]interface{}, 0, len(resp.SenderNames))
	for _, sn := range resp.SenderNames {
		items = append(items, adminSenderNameToJSON(sn))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sender_names": items,
		"total":        resp.Total,
		"limit":        resp.Limit,
		"offset":       resp.Offset,
	})
}

func (h *AdminSenderNameHandlers) ApproveSenderName(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	actorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}
	resp, err := h.client.ApproveSenderName(r.Context(), &sendernamev1.ApproveSenderNameRequest{
		Id:      id,
		ActorId: actorID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, adminSenderNameToJSON(resp.SenderName))
}

type rejectSenderNameRequest struct {
	Reason string `json:"reason"`
}

func (h *AdminSenderNameHandlers) RejectSenderName(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	actorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}
	var req rejectSenderNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if len(reason) < senderNameMinReasonLen {
		respondError(w, shared.ErrInvalidInput("Причина отказа обязательна (минимум 3 символа)"))
		return
	}
	if len(reason) > senderNameMaxReasonLen {
		respondError(w, shared.ErrInvalidInput("Причина отказа слишком длинная (максимум 500 символов)"))
		return
	}
	resp, err := h.client.RejectSenderName(r.Context(), &sendernamev1.RejectSenderNameRequest{
		Id:      id,
		ActorId: actorID.String(),
		Reason:  reason,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, adminSenderNameToJSON(resp.SenderName))
}

type deactivateSenderNameRequest struct {
	Reason string `json:"reason"`
}

func (h *AdminSenderNameHandlers) DeactivateSenderName(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	actorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}
	var req deactivateSenderNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if len(reason) < senderNameMinReasonLen {
		respondError(w, shared.ErrInvalidInput("Причина деактивации обязательна (минимум 3 символа)"))
		return
	}
	if len(reason) > senderNameMaxReasonLen {
		respondError(w, shared.ErrInvalidInput("Причина деактивации слишком длинная (максимум 500 символов)"))
		return
	}
	resp, err := h.client.DeactivateSenderName(r.Context(), &sendernamev1.DeactivateSenderNameRequest{
		Id:      id,
		ActorId: actorID.String(),
		Reason:  reason,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, adminSenderNameToJSON(resp.SenderName))
}

// GetSenderNameAdmin возвращает имя отправителя по ID без проверки владельца.
// GET /admin/v1/sender-names/{id}
func (h *AdminSenderNameHandlers) GetSenderNameAdmin(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	// Пустой ClientId = режим администратора (без проверки владельца)
	resp, err := h.client.GetSenderName(r.Context(), &sendernamev1.GetSenderNameRequest{Id: id})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sender_name": adminSenderNameToJSON(resp.SenderName),
	})
}

// GetSenderNameOperatorRegistrations возвращает список регистраций у операторов.
// GET /admin/v1/sender-names/{id}/operator-registrations
func (h *AdminSenderNameHandlers) GetSenderNameOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}

	// Получаем имя отправителя для client_id и name
	snResp, err := h.client.GetSenderName(r.Context(), &sendernamev1.GetSenderNameRequest{Id: id})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	empty := map[string]interface{}{"registrations": []interface{}{}}

	if h.tariffClient == nil {
		respondJSON(w, http.StatusOK, empty)
		return
	}

	regsResp, err := h.tariffClient.ListSenderRegistrations(r.Context(), &tarificationv1.ListSenderRegistrationsRequest{
		ClientId: snResp.SenderName.ClientId,
		Limit:    100,
	})
	if err != nil {
		log.Error().Err(err).Str("sender_name_id", id).Msg("ошибка получения регистраций ��тправителей")
		respondJSON(w, http.StatusOK, empty)
		return
	}

	registrations := make([]map[string]interface{}, 0)
	for _, reg := range regsResp.Registrations {
		if reg.SenderName != snResp.SenderName.Name {
			continue
		}
		item := map[string]interface{}{
			"operator_id":   reg.OperatorId,
			"operator_name": reg.OperatorId,
			"mcc":           "",
			"mnc":           "",
			"status":        reg.Status,
			"registered_at": nil,
		}
		if reg.CreatedAt != nil {
			item["registered_at"] = reg.CreatedAt.AsTime()
		}
		// Обогащаем данными оператора
		if h.routingClient != nil {
			op, opErr := h.routingClient.GetOperator(r.Context(), &routingv1.GetOperatorRequest{Id: reg.OperatorId})
			if opErr == nil {
				item["operator_name"] = op.Name
				item["mcc"] = op.Code
				item["mnc"] = op.CountryId
			}
		}
		registrations = append(registrations, item)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"registrations": registrations})
}

func adminSenderNameToJSON(sn *sendernamev1.SenderNameInfo) map[string]interface{} {
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
