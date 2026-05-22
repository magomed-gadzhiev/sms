// internal/gateway/client/handlers/templates.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type TemplateHandlers struct {
	templateClient templatev1.TemplateServiceClient
}

func NewTemplateHandlers(templateClient templatev1.TemplateServiceClient) *TemplateHandlers {
	return &TemplateHandlers{templateClient: templateClient}
}

func (h *TemplateHandlers) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req struct {
		Name string `json:"name"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("name обязателен"))
		return
	}
	if req.Body == "" {
		respondError(w, shared.ErrInvalidInput("body обязателен"))
		return
	}

	resp, err := h.templateClient.CreateTemplate(r.Context(), &templatev1.CreateTemplateRequest{
		ClientId: clientID.String(),
		Name:     req.Name,
		Body:     req.Body,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания шаблона")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, templateToMap(resp.Template))
}

func (h *TemplateHandlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	q := r.URL.Query()
	status := q.Get("status")
	limit := int32(100)
	offset := int32(0)

	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = int32(n)
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = int32(n)
		}
	}

	resp, err := h.templateClient.ListTemplates(r.Context(), &templatev1.ListTemplatesRequest{
		ClientId: clientID.String(),
		Status:   status,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	templates := make([]map[string]interface{}, len(resp.Templates))
	for i, t := range resp.Templates {
		templates[i] = templateToMap(t)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"templates": templates,
		"total":     resp.Total,
		"limit":     resp.Limit,
		"offset":    resp.Offset,
	})
}

func (h *TemplateHandlers) GetTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	resp, err := h.templateClient.GetTemplate(r.Context(), &templatev1.GetTemplateRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, templateToMap(resp.Template))
}

func (h *TemplateHandlers) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]

	var req struct {
		Name *string `json:"name,omitempty"`
		Body *string `json:"body,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	grpcReq := &templatev1.UpdateTemplateRequest{
		Id:       id,
		ClientId: clientID.String(),
		Name:     req.Name,
		Body:     req.Body,
	}

	resp, err := h.templateClient.UpdateTemplate(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, templateToMap(resp.Template))
}

func (h *TemplateHandlers) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	_, err := h.templateClient.DeleteTemplate(r.Context(), &templatev1.DeleteTemplateRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateHandlers) GetTemplateAudit(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]

	// Ownership check: сперва достаём template с clientID — template service
	// возвращает NotFound если template принадлежит другому клиенту.
	// Без этого GetTemplateAuditLog ниже не фильтрует по client_id и любой
	// authenticated клиент мог бы читать audit чужих шаблонов
	// (finding F-C1 из Cycle 3 ревью v2).
	if _, err := h.templateClient.GetTemplate(r.Context(), &templatev1.GetTemplateRequest{
		Id:       id,
		ClientId: clientID.String(),
	}); err != nil {
		respondGRPCError(w, err)
		return
	}

	q := r.URL.Query()
	limit := int32(100)
	offset := int32(0)

	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = int32(n)
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = int32(n)
		}
	}

	resp, err := h.templateClient.GetTemplateAuditLog(r.Context(), &templatev1.GetTemplateAuditLogRequest{
		TemplateId: id,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	entries := make([]map[string]interface{}, len(resp.Entries))
	for i, e := range resp.Entries {
		entries[i] = auditEntryToMap(e)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   resp.Total,
	})
}

func templateToMap(t *templatev1.TemplateInfo) map[string]interface{} {
	result := map[string]interface{}{
		"id":               t.Id,
		"client_id":        t.ClientId,
		"name":             t.Name,
		"body":             t.Body,
		"variables":        t.Variables,
		"status":           t.Status,
		"rejection_reason": t.RejectionReason,
	}
	if t.CreatedAt != nil {
		result["created_at"] = t.CreatedAt.AsTime()
	}
	if t.UpdatedAt != nil {
		result["updated_at"] = t.UpdatedAt.AsTime()
	}
	return result
}

func auditEntryToMap(e *templatev1.AuditEntry) map[string]interface{} {
	result := map[string]interface{}{
		"id":          e.Id,
		"template_id": e.TemplateId,
		"action":      e.Action,
		"old_body":    e.OldBody,
		"new_body":    e.NewBody,
		"actor_id":    e.ActorId,
		"actor_type":  e.ActorType,
		"reason":      e.Reason,
	}
	if e.CreatedAt != nil {
		result["created_at"] = e.CreatedAt.AsTime()
	}
	return result
}
