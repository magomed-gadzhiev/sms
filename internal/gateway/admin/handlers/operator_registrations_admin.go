package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// AdminOperatorRegistrationHandlers handles moderation of operator_registrations for direct users.
type AdminOperatorRegistrationHandlers struct {
	db *storage.DB
}

func NewAdminOperatorRegistrationHandlers(db *storage.DB) *AdminOperatorRegistrationHandlers {
	return &AdminOperatorRegistrationHandlers{db: db}
}

// ListAdminOperatorRegistrations GET /admin/v1/operator-registrations
// Returns submissions from direct users (parent_client_id IS NULL) only.
func (h *AdminOperatorRegistrationHandlers) ListAdminOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	operatorFilter := r.URL.Query().Get("operator_id")
	clientFilter := r.URL.Query().Get("client_id")

	query := `SELECT or2.id, or2.sender_name_id, sn.name AS sender_name,
	                 or2.operator_id, o.name AS operator_name,
	                 or2.registration_type, or2.status, or2.approved_type,
	                 or2.moderator_note, or2.submitted_at, or2.resolved_at,
	                 sn.client_id
	          FROM operator_registrations or2
	          JOIN sender_names sn ON sn.id = or2.sender_name_id
	          JOIN operators o ON o.id = or2.operator_id
	          JOIN clients c ON c.id = sn.client_id
	          WHERE c.parent_client_id IS NULL`
	args := []interface{}{}
	i := 1
	if statusFilter != "" {
		query += fmt.Sprintf(" AND or2.status = $%d", i)
		args = append(args, statusFilter)
		i++
	}
	if operatorFilter != "" {
		query += fmt.Sprintf(" AND or2.operator_id = $%d", i)
		args = append(args, operatorFilter)
		i++
	}
	if clientFilter != "" {
		query += fmt.Sprintf(" AND sn.client_id = $%d", i)
		args = append(args, clientFilter)
		i++
	}
	query += " ORDER BY or2.submitted_at DESC LIMIT 100"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения operator_registrations")
		respondError(w, shared.ErrInternalServer("ошибка получения очереди"))
		return
	}
	defer rows.Close()

	type regJSON struct {
		ID               string     `json:"id"`
		SenderNameID     string     `json:"sender_name_id"`
		SenderName       string     `json:"sender_name"`
		OperatorID       string     `json:"operator_id"`
		OperatorName     string     `json:"operator_name"`
		RegistrationType string     `json:"registration_type"`
		Status           string     `json:"status"`
		ApprovedType     *string    `json:"approved_type"`
		ModeratorNote    *string    `json:"moderator_note"`
		SubmittedAt      time.Time  `json:"submitted_at"`
		ResolvedAt       *time.Time `json:"resolved_at"`
		ClientID         string     `json:"client_id"`
	}
	regs := make([]regJSON, 0)
	for rows.Next() {
		var reg regJSON
		if err := rows.Scan(
			&reg.ID, &reg.SenderNameID, &reg.SenderName,
			&reg.OperatorID, &reg.OperatorName,
			&reg.RegistrationType, &reg.Status, &reg.ApprovedType,
			&reg.ModeratorNote, &reg.SubmittedAt, &reg.ResolvedAt,
			&reg.ClientID,
		); err != nil {
			log.Error().Err(err).Msg("ошибка сканирования")
			respondError(w, shared.ErrInternalServer("ошибка получения очереди"))
			return
		}
		regs = append(regs, reg)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"registrations": regs})
}

// ApproveAdminOperatorRegistration POST /admin/v1/operator-registrations/{id}/approve
func (h *AdminOperatorRegistrationHandlers) ApproveAdminOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена или не принадлежит прямому пользователю"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("approve возможен только из статуса submitted"))
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'approved', approved_type = registration_type,
		     approved_at = NOW(), resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id,
	)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("ошибка approve")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}

	_, _ = h.db.ExecContext(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_type, created_at)
		 VALUES ($1, 'submitted', 'approved', 'admin', NOW())`,
		id,
	)

	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "approved"})
}

// RejectAdminOperatorRegistration POST /admin/v1/operator-registrations/{id}/reject
func (h *AdminOperatorRegistrationHandlers) RejectAdminOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("reject возможен только из статуса submitted"))
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	note := adminNullStr(req.Note)
	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'rejected', moderator_note = $2, resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, note,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}

	_, _ = h.db.ExecContext(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_type, comment, created_at)
		 VALUES ($1, 'submitted', 'rejected', 'admin', $2, NOW())`,
		id, note,
	)

	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "rejected"})
}

// RequestRevisionAdminOperatorRegistration POST /admin/v1/operator-registrations/{id}/request-revision
func (h *AdminOperatorRegistrationHandlers) RequestRevisionAdminOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("request-revision возможен только из статуса submitted"))
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	note := adminNullStr(req.Note)
	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'revision_requested', moderator_note = $2,
		     resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, note,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}

	_, _ = h.db.ExecContext(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_type, comment, created_at)
		 VALUES ($1, 'submitted', 'revision_requested', 'admin', $2, NOW())`,
		id, note,
	)

	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "revision_requested"})
}

// adminNullStr returns nil for empty strings (for nullable DB fields).
func adminNullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
