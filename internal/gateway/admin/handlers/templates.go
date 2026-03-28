package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type TemplateHandlers struct {
	templateClient templatev1.TemplateServiceClient
}

func NewTemplateHandlers(templateClient templatev1.TemplateServiceClient) *TemplateHandlers {
	return &TemplateHandlers{templateClient: templateClient}
}

func (h *TemplateHandlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	status := r.URL.Query().Get("status")

	limit := int32(100)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = int32(n)
		}
	}

	offset := int32(0)
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = int32(n)
		}
	}

	resp, err := h.templateClient.ListTemplates(r.Context(), &templatev1.ListTemplatesRequest{
		ClientId: clientID,
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
	id := mux.Vars(r)["id"]
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	resp, err := h.templateClient.GetTemplate(r.Context(), &templatev1.GetTemplateRequest{
		Id:       id,
		ClientId: clientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, templateToMap(resp.Template))
}

func (h *TemplateHandlers) ApproveTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var req struct {
		ActorID string `json:"actor_id"`
	}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
			return
		}
	}

	resp, err := h.templateClient.ApproveTemplate(r.Context(), &templatev1.ApproveTemplateRequest{
		Id:      id,
		ActorId: req.ActorID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, templateToMap(resp.Template))
}

func (h *TemplateHandlers) RejectTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var req struct {
		ActorID string `json:"actor_id"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Reason == "" {
		respondError(w, shared.ErrInvalidInput("reason обязателен"))
		return
	}

	resp, err := h.templateClient.RejectTemplate(r.Context(), &templatev1.RejectTemplateRequest{
		Id:      id,
		ActorId: req.ActorID,
		Reason:  req.Reason,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, templateToMap(resp.Template))
}

func (h *TemplateHandlers) GetTemplateAudit(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	limit := int32(100)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = int32(n)
		}
	}

	offset := int32(0)
	if v := r.URL.Query().Get("offset"); v != "" {
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
		entry := map[string]interface{}{
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
			entry["created_at"] = e.CreatedAt.AsTime()
		}
		entries[i] = entry
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   resp.Total,
	})
}

// AssignReviewer обрабатывает POST /admin/v1/templates/:id/assign
func (h *TemplateHandlers) AssignReviewer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	reviewerID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("не удалось определить пользователя"))
		return
	}

	resp, err := h.templateClient.AssignReviewer(r.Context(), &templatev1.AssignReviewerRequest{
		TemplateId: id,
		ReviewerId: reviewerID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, templateToMap(resp.Template))
}

// RequestRevision обрабатывает POST /admin/v1/templates/:id/request-revision
func (h *TemplateHandlers) RequestRevision(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	reviewerID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("не удалось определить пользователя"))
		return
	}

	var req struct {
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Comment == "" {
		respondError(w, shared.ErrInvalidInput("comment обязателен"))
		return
	}

	resp, err := h.templateClient.RequestRevision(r.Context(), &templatev1.RequestRevisionRequest{
		TemplateId: id,
		ReviewerId: reviewerID.String(),
		Comment:    req.Comment,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, templateToMap(resp.Template))
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
