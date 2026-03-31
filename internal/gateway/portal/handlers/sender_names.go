package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type SenderNameHandlers struct {
	client sendernamev1.SenderNameServiceClient
}

func NewSenderNameHandlers(client sendernamev1.SenderNameServiceClient) *SenderNameHandlers {
	return &SenderNameHandlers{client: client}
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
