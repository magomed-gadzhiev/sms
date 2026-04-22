package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type ResellerTemplateHandlers struct {
	pool           *pgxpool.Pool
	templateClient templatev1.TemplateServiceClient
}

func NewResellerTemplateHandlers(pool *pgxpool.Pool, templateClient templatev1.TemplateServiceClient) *ResellerTemplateHandlers {
	return &ResellerTemplateHandlers{pool: pool, templateClient: templateClient}
}

func (h *ResellerTemplateHandlers) checkReseller(w http.ResponseWriter, r *http.Request) (string, bool) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return "", false
	}
	var isReseller bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT is_reseller FROM clients WHERE id = $1`, clientID,
	).Scan(&isReseller); err != nil || !isReseller {
		respondError(w, shared.ErrUnauthorized("доступ только для агрегаторов"))
		return "", false
	}
	return clientID.String(), true
}

func (h *ResellerTemplateHandlers) checkOwnershipAndStatus(w http.ResponseWriter, r *http.Request, clientID, templateID string) (string, bool) {
	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT t.status FROM templates t
		 JOIN clients c ON c.id = t.client_id
		 WHERE t.id = $1 AND c.parent_client_id = $2`,
		templateID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return "", false
	}
	return currentStatus, true
}

// ListResellerTemplates GET /portal/v1/reseller/templates
func (h *ResellerTemplateHandlers) ListResellerTemplates(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	statusFilter := r.URL.Query().Get("status")
	query := `SELECT t.id, t.client_id, c.email AS sub_account_email,
	                 t.name, LEFT(t.body, 80) AS body_preview, t.status,
	                 t.rejection_reason, t.created_at
	          FROM templates t
	          JOIN clients c ON c.id = t.client_id
	          WHERE c.parent_client_id = $1`
	args := []interface{}{clientID}

	if statusFilter != "" {
		query += " AND t.status = $2"
		args = append(args, statusFilter)
	}
	query += " ORDER BY t.created_at DESC LIMIT 50"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения шаблонов субаккаунтов")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	type templateJSON struct {
		ID              string    `json:"id"`
		ClientID        string    `json:"client_id"`
		SubAccountEmail string    `json:"sub_account_email"`
		Name            string    `json:"name"`
		BodyPreview     string    `json:"body_preview"`
		Status          string    `json:"status"`
		RejectionReason *string   `json:"rejection_reason"`
		CreatedAt       time.Time `json:"created_at"`
	}
	items := make([]templateJSON, 0)
	for rows.Next() {
		var t templateJSON
		if err := rows.Scan(&t.ID, &t.ClientID, &t.SubAccountEmail,
			&t.Name, &t.BodyPreview, &t.Status, &t.RejectionReason, &t.CreatedAt); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"templates": items, "total": len(items)})
}

// ApproveResellerTemplate POST /portal/v1/reseller/templates/{id}/approve
func (h *ResellerTemplateHandlers) ApproveResellerTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("пользователь не найден"))
		return
	}
	id := mux.Vars(r)["id"]

	status, ok := h.checkOwnershipAndStatus(w, r, clientID, id)
	if !ok {
		return
	}
	if status != "pending" && status != "revision_requested" {
		respondError(w, shared.ErrInvalidInput("approve возможен только из статуса pending или revision_requested"))
		return
	}

	resp, err := h.templateClient.ApproveTemplate(r.Context(), &templatev1.ApproveTemplateRequest{
		Id:      id,
		ActorId: userID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resellerTemplateToJSON(resp.Template))
}

// RejectResellerTemplate POST /portal/v1/reseller/templates/{id}/reject
func (h *ResellerTemplateHandlers) RejectResellerTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("пользователь не найден"))
		return
	}
	id := mux.Vars(r)["id"]

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Reason == "" {
		respondError(w, shared.ErrInvalidInput("reason обязателен"))
		return
	}

	status, ok := h.checkOwnershipAndStatus(w, r, clientID, id)
	if !ok {
		return
	}
	if status != "pending" && status != "revision_requested" {
		respondError(w, shared.ErrInvalidInput("reject возможен только из статуса pending или revision_requested"))
		return
	}

	resp, err := h.templateClient.RejectTemplate(r.Context(), &templatev1.RejectTemplateRequest{
		Id:      id,
		ActorId: userID.String(),
		Reason:  req.Reason,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resellerTemplateToJSON(resp.Template))
}

// RequestRevisionResellerTemplate POST /portal/v1/reseller/templates/{id}/request-revision
func (h *ResellerTemplateHandlers) RequestRevisionResellerTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("пользователь не найден"))
		return
	}
	id := mux.Vars(r)["id"]

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

	status, ok := h.checkOwnershipAndStatus(w, r, clientID, id)
	if !ok {
		return
	}
	if status != "pending" && status != "revision_requested" {
		respondError(w, shared.ErrInvalidInput("request-revision возможен только из статуса pending или revision_requested"))
		return
	}

	resp, err := h.templateClient.RequestRevision(r.Context(), &templatev1.RequestRevisionRequest{
		TemplateId: id,
		ReviewerId: userID.String(),
		Comment:    req.Comment,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resellerTemplateToJSON(resp.Template))
}

func resellerTemplateToJSON(t *templatev1.TemplateInfo) map[string]interface{} {
	if t == nil {
		return nil
	}
	// Canonical shape matches list endpoint (*string → null when empty).
	var rejectionReason interface{}
	if t.RejectionReason != "" {
		rejectionReason = t.RejectionReason
	}
	m := map[string]interface{}{
		"id":               t.Id,
		"client_id":        t.ClientId,
		"name":             t.Name,
		"body":             t.Body,
		"status":           t.Status,
		"rejection_reason": rejectionReason,
	}
	if t.CreatedAt != nil {
		m["created_at"] = t.CreatedAt.AsTime()
	}
	if t.UpdatedAt != nil {
		m["updated_at"] = t.UpdatedAt.AsTime()
	}
	return m
}
