package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type ResellerModerationHandlers struct {
	pool *pgxpool.Pool
}

func NewResellerModerationHandlers(pool *pgxpool.Pool) *ResellerModerationHandlers {
	return &ResellerModerationHandlers{pool: pool}
}

// checkReseller — см. C.4 cleanup в reseller_dashboard.go.
func (h *ResellerModerationHandlers) checkReseller(w http.ResponseWriter, r *http.Request) (interface{}, bool) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return nil, false
	}
	return clientID, true
}

// ListResellerOperatorRegistrations GET /portal/v1/reseller/operator-registrations
func (h *ResellerModerationHandlers) ListResellerOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	statusFilter := r.URL.Query().Get("status")
	subAccountFilter := r.URL.Query().Get("sub_account_id")

	query := `SELECT or2.id, or2.sender_name_id, sn.name AS sender_name,
	                 or2.operator_id, o.name AS operator_name,
	                 or2.registration_type, or2.status, or2.approved_type,
	                 or2.moderator_note, or2.submitted_at, or2.resolved_at,
	                 sn.client_id AS sub_account_id, c.email AS sub_account_email
	          FROM operator_registrations or2
	          JOIN sender_names sn ON sn.id = or2.sender_name_id
	          JOIN operators o ON o.id = or2.operator_id
	          JOIN clients c ON c.id = sn.client_id
	          WHERE c.parent_client_id = $1`
	args := []interface{}{clientID}
	i := 2
	if statusFilter != "" {
		query += fmt.Sprintf(" AND or2.status = $%d", i)
		args = append(args, statusFilter)
		i++
	}
	if subAccountFilter != "" {
		query += fmt.Sprintf(" AND sn.client_id = $%d", i)
		args = append(args, subAccountFilter)
		i++
	}
	query += " ORDER BY or2.submitted_at DESC LIMIT 100"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения очереди субаккаунтов")
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
		SubAccountID     string     `json:"sub_account_id"`
		SubAccountEmail  string     `json:"sub_account_email"`
	}
	regs := make([]regJSON, 0)
	for rows.Next() {
		var reg regJSON
		if err := rows.Scan(
			&reg.ID, &reg.SenderNameID, &reg.SenderName,
			&reg.OperatorID, &reg.OperatorName,
			&reg.RegistrationType, &reg.Status, &reg.ApprovedType,
			&reg.ModeratorNote, &reg.SubmittedAt, &reg.ResolvedAt,
			&reg.SubAccountID, &reg.SubAccountEmail,
		); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		regs = append(regs, reg)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"registrations": regs})
}

// ApproveResellerOperatorRegistration POST /portal/v1/reseller/operator-registrations/{id}/approve
func (h *ResellerModerationHandlers) ApproveResellerOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id = $2`,
		id, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("approve возможен только из статуса submitted"))
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'approved', approved_type = registration_type,
		     approved_at = NOW(), resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id,
	); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	if _, err = tx.Exec(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_id, actor_type, created_at)
		 VALUES ($1, 'submitted', 'approved', $2, 'aggregator', NOW())`,
		id, clientID,
	); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка записи истории"))
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "approved"})
}

// RejectResellerOperatorRegistration POST /portal/v1/reseller/operator-registrations/{id}/reject
func (h *ResellerModerationHandlers) RejectResellerOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id = $2`,
		id, clientID,
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
	note := resellerNullStr(req.Note)

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'rejected', moderator_note = $2, resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, note,
	); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	if _, err = tx.Exec(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_id, actor_type, comment, created_at)
		 VALUES ($1, 'submitted', 'rejected', $2, 'aggregator', $3, NOW())`,
		id, clientID, note,
	); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка записи истории"))
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "rejected"})
}

// RequestRevisionResellerOperatorRegistration POST /portal/v1/reseller/operator-registrations/{id}/request-revision
func (h *ResellerModerationHandlers) RequestRevisionResellerOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id = $2`,
		id, clientID,
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
	note := resellerNullStr(req.Note)

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'revision_requested', moderator_note = $2,
		     resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, note,
	); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	if _, err = tx.Exec(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_id, actor_type, comment, created_at)
		 VALUES ($1, 'submitted', 'revision_requested', $2, 'aggregator', $3, NOW())`,
		id, clientID, note,
	); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка записи истории"))
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "revision_requested"})
}

// GetModerationCounts GET /portal/v1/reseller/moderation/counts
func (h *ResellerModerationHandlers) GetModerationCounts(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	var snCount, tplCount, regCount int
	err := h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM sender_names sn
		 JOIN clients c ON c.id = sn.client_id
		 WHERE c.parent_client_id = $1 AND sn.status = 'pending'`,
		clientID,
	).Scan(&snCount)
	if err != nil {
		snCount = 0
	}

	err = h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM templates t
		 JOIN clients c ON c.id = t.client_id
		 WHERE c.parent_client_id = $1 AND (t.status = 'pending' OR t.status = 'revision_requested')`,
		clientID,
	).Scan(&tplCount)
	if err != nil {
		tplCount = 0
	}

	err = h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE c.parent_client_id = $1 AND or2.status = 'submitted'`,
		clientID,
	).Scan(&regCount)
	if err != nil {
		regCount = 0
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sender_names":  snCount,
		"templates":     tplCount,
		"registrations": regCount,
	})
}

func resellerNullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
