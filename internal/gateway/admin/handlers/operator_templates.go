package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// OperatorTemplateHandlers обрабатывает CRUD для шаблонов операторов.
type OperatorTemplateHandlers struct {
	db *storage.DB
}

// NewOperatorTemplateHandlers создаёт обработчик.
func NewOperatorTemplateHandlers(db *storage.DB) *OperatorTemplateHandlers {
	return &OperatorTemplateHandlers{db: db}
}

type operatorTemplateRow struct {
	ID             string
	Name           string
	OperatorID     string
	OperatorName   string
	SenderNameID   sql.NullString
	SenderName     sql.NullString
	Body           string
	Variables      string // JSON array
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (r operatorTemplateRow) toJSON() map[string]interface{} {
	vars := []string{}
	_ = json.Unmarshal([]byte(r.Variables), &vars)
	m := map[string]interface{}{
		"id":            r.ID,
		"name":          r.Name,
		"operator_id":   r.OperatorID,
		"operator_name": r.OperatorName,
		"body":          r.Body,
		"variables":     vars,
		"status":        r.Status,
		"created_at":    r.CreatedAt,
		"updated_at":    r.UpdatedAt,
	}
	if r.SenderNameID.Valid {
		m["sender_name_id"] = r.SenderNameID.String
		m["sender_name"] = r.SenderName.String
	}
	return m
}

const operatorTemplateSelectQuery = `
	SELECT
		ot.id::text,
		ot.name,
		ot.operator_id::text,
		COALESCE(op.name, '')   AS operator_name,
		ot.sender_name_id::text,
		sn.name                  AS sender_name_val,
		ot.body,
		ot.variables::text,
		ot.status,
		ot.created_at,
		ot.updated_at
	FROM operator_templates ot
	LEFT JOIN operators op ON op.id = ot.operator_id
	LEFT JOIN sender_names sn ON sn.id = ot.sender_name_id
`

func scanOperatorTemplate(row *sql.Row) (operatorTemplateRow, error) {
	var r operatorTemplateRow
	err := row.Scan(
		&r.ID, &r.Name, &r.OperatorID, &r.OperatorName,
		&r.SenderNameID, &r.SenderName,
		&r.Body, &r.Variables, &r.Status,
		&r.CreatedAt, &r.UpdatedAt,
	)
	return r, err
}

// ListOperatorTemplates обрабатывает GET /admin/v1/operator-templates
func (h *OperatorTemplateHandlers) ListOperatorTemplates(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	operatorID := q.Get("operator_id")
	senderNameID := q.Get("sender_name_id")
	status := q.Get("status")
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	args := []interface{}{}
	conditions := ""
	nextArg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if operatorID != "" {
		conditions += " AND ot.operator_id = " + nextArg(operatorID) + "::uuid"
	}
	if senderNameID != "" {
		conditions += " AND ot.sender_name_id = " + nextArg(senderNameID) + "::uuid"
	}
	if status != "" {
		conditions += " AND ot.status = " + nextArg(status)
	}

	countQuery := `SELECT COUNT(*) FROM operator_templates ot WHERE 1=1` + conditions
	var total int64
	_ = h.db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total)

	listArgs := append(args, limit, offset)
	limitPH := fmt.Sprintf("$%d", len(listArgs)-1)
	offsetPH := fmt.Sprintf("$%d", len(listArgs))

	listQuery := operatorTemplateSelectQuery + `
		WHERE 1=1` + conditions + `
		ORDER BY ot.created_at DESC
		LIMIT ` + limitPH + ` OFFSET ` + offsetPH

	rows, err := h.db.QueryContext(r.Context(), listQuery, listArgs...)
	if err != nil {
		log.Error().Err(err).Msg("operator_templates: ошибка списка")
		respondError(w, shared.ErrInternalServer("Ошибка получения шаблонов операторов"))
		return
	}
	defer rows.Close()

	items := []map[string]interface{}{}
	for rows.Next() {
		var row operatorTemplateRow
		if err := rows.Scan(
			&row.ID, &row.Name, &row.OperatorID, &row.OperatorName,
			&row.SenderNameID, &row.SenderName,
			&row.Body, &row.Variables, &row.Status,
			&row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			continue
		}
		items = append(items, row.toJSON())
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"operator_templates": items,
		"total":              total,
		"limit":              limit,
		"offset":             offset,
	})
}

// GetOperatorTemplate обрабатывает GET /admin/v1/operator-templates/{id}
func (h *OperatorTemplateHandlers) GetOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	row, err := scanOperatorTemplate(h.db.QueryRowContext(r.Context(),
		operatorTemplateSelectQuery+` WHERE ot.id = $1::uuid`, id))
	if err == sql.ErrNoRows {
		respondError(w, shared.ErrNotFound("Шаблон не найден"))
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("operator_templates: ошибка get")
		respondError(w, shared.ErrInternalServer("Ошибка получения"))
		return
	}
	respondJSON(w, http.StatusOK, row.toJSON())
}

type operatorTemplateRequest struct {
	Name         string   `json:"name"`
	OperatorID   string   `json:"operator_id"`
	SenderNameID string   `json:"sender_name_id"`
	Body         string   `json:"body"`
	Variables    []string `json:"variables"`
	Status       string   `json:"status"`
}

// CreateOperatorTemplate обрабатывает POST /admin/v1/operator-templates
func (h *OperatorTemplateHandlers) CreateOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	var req operatorTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" || req.OperatorID == "" || req.Body == "" {
		respondError(w, shared.ErrInvalidInput("name, operator_id и body обязательны"))
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	varsJSON, _ := json.Marshal(req.Variables)

	var newID string
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO operator_templates (name, operator_id, sender_name_id, body, variables, status)
		VALUES ($1, $2::uuid, NULLIF($3,'')::uuid, $4, $5::jsonb, $6)
		RETURNING id::text
	`, req.Name, req.OperatorID, req.SenderNameID, req.Body, string(varsJSON), status).Scan(&newID)
	if err != nil {
		log.Error().Err(err).Msg("operator_templates: ошибка создания")
		respondError(w, shared.ErrInternalServer("Ошибка создания шаблона"))
		return
	}

	row, err := scanOperatorTemplate(h.db.QueryRowContext(r.Context(),
		operatorTemplateSelectQuery+` WHERE ot.id = $1::uuid`, newID))
	if err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка получения созданного шаблона"))
		return
	}
	respondJSON(w, http.StatusCreated, row.toJSON())
}

// UpdateOperatorTemplate обрабатывает PUT /admin/v1/operator-templates/{id}
func (h *OperatorTemplateHandlers) UpdateOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req operatorTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	varsJSON, _ := json.Marshal(req.Variables)

	res, err := h.db.ExecContext(r.Context(), `
		UPDATE operator_templates
		SET name           = COALESCE(NULLIF($2,''), name),
		    sender_name_id = CASE WHEN $3::text = '' THEN sender_name_id ELSE $3::uuid END,
		    body           = COALESCE(NULLIF($4,''), body),
		    variables      = CASE WHEN $5 = '[]' OR $5 = 'null' THEN variables ELSE $5::jsonb END,
		    status         = COALESCE(NULLIF($6,''), status),
		    updated_at     = now()
		WHERE id = $1::uuid
	`, id, req.Name, req.SenderNameID, req.Body, string(varsJSON), req.Status)
	if err != nil {
		log.Error().Err(err).Msg("operator_templates: ошибка обновления")
		respondError(w, shared.ErrInternalServer("Ошибка обновления"))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondError(w, shared.ErrNotFound("Шаблон не найден"))
		return
	}

	row, err := scanOperatorTemplate(h.db.QueryRowContext(r.Context(),
		operatorTemplateSelectQuery+` WHERE ot.id = $1::uuid`, id))
	if err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка получения обновлённого шаблона"))
		return
	}
	respondJSON(w, http.StatusOK, row.toJSON())
}

// DeleteOperatorTemplate обрабатывает DELETE /admin/v1/operator-templates/{id}
func (h *OperatorTemplateHandlers) DeleteOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	res, err := h.db.ExecContext(r.Context(),
		`DELETE FROM operator_templates WHERE id = $1::uuid`, id)
	if err != nil {
		log.Error().Err(err).Msg("operator_templates: ошибка удаления")
		respondError(w, shared.ErrInternalServer("Ошибка удаления"))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondError(w, shared.ErrNotFound("Шаблон не найден"))
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

// ApproveOperatorTemplate POST /admin/v1/operator-templates/{id}/approve
func (h *OperatorTemplateHandlers) ApproveOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE ot.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("approve возможен только из статуса submitted"))
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_templates
		 SET moderation_status = 'approved', resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"moderation_status": "approved"})
}

// RejectOperatorTemplate POST /admin/v1/operator-templates/{id}/reject
func (h *OperatorTemplateHandlers) RejectOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE ot.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
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

	note := opTplNullStr(req.Note)
	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_templates
		 SET moderation_status = 'rejected', moderator_note = $2,
		     resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, note,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"moderation_status": "rejected"})
}

// RequestRevisionOperatorTemplate POST /admin/v1/operator-templates/{id}/request-revision
func (h *OperatorTemplateHandlers) RequestRevisionOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE ot.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
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

	note := opTplNullStr(req.Note)
	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_templates
		 SET moderation_status = 'revision_requested', moderator_note = $2,
		     resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, note,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"moderation_status": "revision_requested"})
}

func opTplNullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
