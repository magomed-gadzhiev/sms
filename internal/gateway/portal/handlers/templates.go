package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type TemplateHandlers struct {
	templateClient templatev1.TemplateServiceClient
}

func NewTemplateHandlers(templateClient templatev1.TemplateServiceClient) *TemplateHandlers {
	return &TemplateHandlers{templateClient: templateClient}
}

type createTemplateRequest struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

func (h *TemplateHandlers) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var req createTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}
	if req.Body == "" {
		respondError(w, shared.ErrInvalidInput("Поле body обязательно"))
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
	respondJSON(w, http.StatusCreated, templateToJSON(resp.Template))
}

func (h *TemplateHandlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	statusFilter := r.URL.Query().Get("status")
	resp, err := h.templateClient.ListTemplates(r.Context(), &templatev1.ListTemplatesRequest{
		ClientId: clientID.String(),
		Status:   statusFilter,
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка шаблонов")
		respondGRPCError(w, err)
		return
	}
	templates := make([]map[string]interface{}, 0, len(resp.Templates))
	for _, t := range resp.Templates {
		templates = append(templates, templateToJSON(t))
	}
	totalPages := int32(0)
	if perPage > 0 && resp.Total > 0 {
		totalPages = (resp.Total + perPage - 1) / perPage
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"templates":   templates,
		"total":       resp.Total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

func (h *TemplateHandlers) GetTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID шаблона обязателен"))
		return
	}
	resp, err := h.templateClient.GetTemplate(r.Context(), &templatev1.GetTemplateRequest{
		Id: id, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, templateToJSON(resp.Template))
}

type updateTemplateRequest struct {
	Name *string `json:"name,omitempty"`
	Body *string `json:"body,omitempty"`
}

func (h *TemplateHandlers) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID шаблона обязателен"))
		return
	}
	var req updateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	grpcReq := &templatev1.UpdateTemplateRequest{Id: id, ClientId: clientID.String()}
	if req.Name != nil {
		grpcReq.Name = req.Name
	}
	if req.Body != nil {
		grpcReq.Body = req.Body
	}
	resp, err := h.templateClient.UpdateTemplate(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, templateToJSON(resp.Template))
}

func (h *TemplateHandlers) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID шаблона обязателен"))
		return
	}
	_, err := h.templateClient.DeleteTemplate(r.Context(), &templatev1.DeleteTemplateRequest{
		Id: id, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type renderTemplateRequest struct {
	Variables map[string]string `json:"variables"`
}

func (h *TemplateHandlers) RenderTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID шаблона обязателен"))
		return
	}
	var req renderTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	resp, err := h.templateClient.RenderTemplate(r.Context(), &templatev1.RenderTemplateRequest{
		TemplateId: id, ClientId: clientID.String(), Variables: req.Variables,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"rendered_text": resp.RenderedText,
		"template_name": resp.TemplateName,
	})
}

func (h *TemplateHandlers) GetTemplateAuditLog(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID шаблона обязателен"))
		return
	}
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	resp, err := h.templateClient.GetTemplateAuditLog(r.Context(), &templatev1.GetTemplateAuditLogRequest{
		TemplateId: id, Limit: perPage, Offset: offset,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	entries := make([]map[string]interface{}, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		entry := map[string]interface{}{
			"id": e.Id, "template_id": e.TemplateId, "action": e.Action,
			"actor_id": e.ActorId, "actor_type": e.ActorType,
		}
		if e.OldBody != "" {
			entry["old_body"] = e.OldBody
		}
		if e.NewBody != "" {
			entry["new_body"] = e.NewBody
		}
		if e.Reason != "" {
			entry["reason"] = e.Reason
		}
		if e.CreatedAt != nil {
			entry["created_at"] = e.CreatedAt.AsTime()
		}
		entries = append(entries, entry)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"entries": entries, "total": resp.Total})
}

func (h *TemplateHandlers) SubmitForReview(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID шаблона обязателен"))
		return
	}
	resp, err := h.templateClient.SubmitForReview(r.Context(), &templatev1.SubmitForReviewRequest{
		TemplateId: id,
		ClientId:   clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, templateToJSON(resp.Template))
}

func templateToJSON(t *templatev1.TemplateInfo) map[string]interface{} {
	if t == nil {
		return nil
	}
	m := map[string]interface{}{
		"id": t.Id, "client_id": t.ClientId, "name": t.Name, "body": t.Body,
		"variables": t.Variables, "status": t.Status,
	}
	if t.RejectionReason != "" {
		m["rejection_reason"] = t.RejectionReason
	}
	if t.CreatedAt != nil {
		m["created_at"] = t.CreatedAt.AsTime()
	}
	if t.UpdatedAt != nil {
		m["updated_at"] = t.UpdatedAt.AsTime()
	}
	return m
}
