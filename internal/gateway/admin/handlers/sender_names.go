package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type AdminSenderNameHandlers struct {
	client sendernamev1.SenderNameServiceClient
}

func NewAdminSenderNameHandlers(client sendernamev1.SenderNameServiceClient) *AdminSenderNameHandlers {
	return &AdminSenderNameHandlers{client: client}
}

func (h *AdminSenderNameHandlers) ListAllSenderNames(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 20)
	offset := parseIntParam(r, "offset", 0)

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
	resp, err := h.client.RejectSenderName(r.Context(), &sendernamev1.RejectSenderNameRequest{
		Id:      id,
		ActorId: actorID.String(),
		Reason:  req.Reason,
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
	resp, err := h.client.DeactivateSenderName(r.Context(), &sendernamev1.DeactivateSenderNameRequest{
		Id:      id,
		ActorId: actorID.String(),
		Reason:  req.Reason,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, adminSenderNameToJSON(resp.SenderName))
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
