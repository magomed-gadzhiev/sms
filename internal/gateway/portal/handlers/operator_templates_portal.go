package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type PortalOperatorTemplateHandlers struct {
	pool *pgxpool.Pool
}

func NewPortalOperatorTemplateHandlers(pool *pgxpool.Pool) *PortalOperatorTemplateHandlers {
	return &PortalOperatorTemplateHandlers{pool: pool}
}

// ListPortalOperatorTemplates GET /portal/v1/sender-names/{id}/operator-templates
func (h *PortalOperatorTemplateHandlers) ListPortalOperatorTemplates(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]

	operatorFilter := r.URL.Query().Get("operator_id")

	query := `SELECT ot.id, ot.name, ot.operator_id, o.name AS operator_name,
	                 ot.body, ot.moderation_status, ot.moderator_note,
	                 ot.submitted_at, ot.resolved_at, ot.created_at, ot.updated_at
	          FROM operator_templates ot
	          JOIN operators o ON o.id = ot.operator_id
	          JOIN sender_names sn ON sn.id = ot.sender_name_id
	          WHERE ot.sender_name_id = $1 AND sn.client_id = $2`
	args := []interface{}{senderNameID, clientID}
	if operatorFilter != "" {
		query += " AND ot.operator_id = $3"
		args = append(args, operatorFilter)
	}
	query += " ORDER BY o.name, ot.name"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения operator_templates")
		respondError(w, shared.ErrInternalServer("ошибка получения шаблонов"))
		return
	}
	defer rows.Close()

	type tplJSON struct {
		ID               string     `json:"id"`
		Name             string     `json:"name"`
		OperatorID       string     `json:"operator_id"`
		OperatorName     string     `json:"operator_name"`
		Body             string     `json:"body"`
		ModerationStatus string     `json:"moderation_status"`
		ModeratorNote    *string    `json:"moderator_note"`
		SubmittedAt      *time.Time `json:"submitted_at"`
		ResolvedAt       *time.Time `json:"resolved_at"`
		CreatedAt        time.Time  `json:"created_at"`
		UpdatedAt        time.Time  `json:"updated_at"`
	}
	templates := make([]tplJSON, 0)
	for rows.Next() {
		var t tplJSON
		if err := rows.Scan(
			&t.ID, &t.Name, &t.OperatorID, &t.OperatorName,
			&t.Body, &t.ModerationStatus, &t.ModeratorNote,
			&t.SubmittedAt, &t.ResolvedAt, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка получения шаблонов"))
			return
		}
		templates = append(templates, t)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"templates": templates})
}

// CreatePortalOperatorTemplate POST /portal/v1/sender-names/{id}/operator-templates
func (h *PortalOperatorTemplateHandlers) CreatePortalOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]

	var snStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT status FROM sender_names WHERE id = $1 AND client_id = $2`,
		senderNameID, clientID,
	).Scan(&snStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("имя отправителя не найдено"))
		return
	}
	if snStatus != "approved" {
		respondError(w, shared.ErrInvalidInput("имя отправителя должно быть в статусе approved"))
		return
	}

	var req struct {
		OperatorID string `json:"operator_id"`
		Name       string `json:"name"`
		Body       string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.OperatorID == "" || req.Name == "" || req.Body == "" {
		respondError(w, shared.ErrInvalidInput("operator_id, name, body обязательны"))
		return
	}

	var newID string
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO operator_templates
		    (sender_name_id, operator_id, name, body, variables, status, moderation_status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, '[]', 'active', 'draft', NOW(), NOW())
		 RETURNING id`,
		senderNameID, req.OperatorID, req.Name, req.Body,
	).Scan(&newID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания operator_template")
		respondError(w, shared.ErrInternalServer("ошибка создания шаблона"))
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":                newID,
		"moderation_status": "draft",
	})
}

// UpdatePortalOperatorTemplate PUT /portal/v1/sender-names/{id}/operator-templates/{tid}
func (h *PortalOperatorTemplateHandlers) UpdatePortalOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	tid := mux.Vars(r)["tid"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 WHERE ot.id = $1 AND ot.sender_name_id = $2 AND sn.client_id = $3`,
		tid, senderNameID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "draft" {
		respondError(w, shared.ErrInvalidInput("редактирование возможно только в статусе draft"))
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

	_, err = h.pool.Exec(r.Context(),
		`UPDATE operator_templates SET name = $1, body = $2, updated_at = NOW()
		 WHERE id = $3`,
		req.Name, req.Body, tid,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления шаблона"))
		return
	}
	_ = clientID
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": tid, "status": "updated"})
}

// DeletePortalOperatorTemplate DELETE /portal/v1/sender-names/{id}/operator-templates/{tid}
func (h *PortalOperatorTemplateHandlers) DeletePortalOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	tid := mux.Vars(r)["tid"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 WHERE ot.id = $1 AND ot.sender_name_id = $2 AND sn.client_id = $3`,
		tid, senderNameID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "draft" {
		respondError(w, shared.ErrInvalidInput("удаление возможно только в статусе draft"))
		return
	}

	_, _ = h.pool.Exec(r.Context(), `DELETE FROM operator_templates WHERE id = $1`, tid)
	_ = clientID
	respondJSON(w, http.StatusOK, map[string]interface{}{"deleted": true})
}

// SubmitPortalOperatorTemplate POST /portal/v1/sender-names/{id}/operator-templates/{tid}/submit
func (h *PortalOperatorTemplateHandlers) SubmitPortalOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	tid := mux.Vars(r)["tid"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 WHERE ot.id = $1 AND ot.sender_name_id = $2 AND sn.client_id = $3`,
		tid, senderNameID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "draft" {
		respondError(w, shared.ErrInvalidInput("submit возможен только из статуса draft"))
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE operator_templates
		 SET moderation_status = 'submitted', submitted_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		tid,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления статуса"))
		return
	}
	_ = clientID
	respondJSON(w, http.StatusOK, map[string]interface{}{"moderation_status": "submitted"})
}

// ResubmitPortalOperatorTemplate POST /portal/v1/sender-names/{id}/operator-templates/{tid}/resubmit
func (h *PortalOperatorTemplateHandlers) ResubmitPortalOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	tid := mux.Vars(r)["tid"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 WHERE ot.id = $1 AND ot.sender_name_id = $2 AND sn.client_id = $3`,
		tid, senderNameID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "revision_requested" {
		respondError(w, shared.ErrInvalidInput("resubmit возможен только из статуса revision_requested"))
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE operator_templates
		 SET moderation_status = 'submitted', submitted_at = NOW(),
		     resolved_at = NULL, moderator_note = NULL, updated_at = NOW()
		 WHERE id = $1`,
		tid,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления статуса"))
		return
	}
	_ = clientID
	respondJSON(w, http.StatusOK, map[string]interface{}{"moderation_status": "submitted"})
}
