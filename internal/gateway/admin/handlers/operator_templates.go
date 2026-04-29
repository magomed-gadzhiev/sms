package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

const (
	operatorTemplateMaxListLimit  = 200
	operatorTemplateDefaultLimit  = 50
	operatorTemplateMinNoteLen    = 3
	operatorTemplateMaxNoteLen    = 500
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
	limit := parseIntParam(r, "limit", operatorTemplateDefaultLimit)
	offset := parseIntParam(r, "offset", 0)
	if limit <= 0 {
		limit = operatorTemplateDefaultLimit
	}
	if limit > operatorTemplateMaxListLimit {
		limit = operatorTemplateMaxListLimit
	}
	if offset < 0 {
		offset = 0
	}

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

type operatorTemplateCreateRequest struct {
	Name         string   `json:"name"`
	OperatorID   string   `json:"operator_id"`
	SenderNameID string   `json:"sender_name_id"`
	Body         string   `json:"body"`
	Variables    []string `json:"variables"`
	Status       string   `json:"status"`
}

// operatorTemplateUpdateRequest использует *string для sender_name_id чтобы
// различать три случая в PUT:
//   - поле отсутствует / null → keep текущее значение
//   - поле "" (пустая строка) → сбросить sender_name_id в NULL
//   - поле "<uuid>" → установить новое значение
type operatorTemplateUpdateRequest struct {
	Name         string   `json:"name"`
	OperatorID   string   `json:"operator_id"`
	SenderNameID *string  `json:"sender_name_id"`
	Body         string   `json:"body"`
	Variables    []string `json:"variables"`
	Status       string   `json:"status"`
}

// CreateOperatorTemplate обрабатывает POST /admin/v1/operator-templates
func (h *OperatorTemplateHandlers) CreateOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	var req operatorTemplateCreateRequest
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
	var req operatorTemplateUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	varsJSON, _ := json.Marshal(req.Variables)

	// Трёхзначная логика sender_name_id (см. doc у operatorTemplateUpdateRequest):
	//   nil → senderNameAction='keep', SQL ветка оставит текущее значение
	//   *"" → senderNameAction='clear', SQL ветка установит NULL
	//   *"<uuid>" → senderNameAction='set', SQL ветка кастит и присвоит
	var (
		senderNameAction = "keep"
		senderNameValue  interface{}
	)
	if req.SenderNameID != nil {
		if *req.SenderNameID == "" {
			senderNameAction = "clear"
		} else {
			senderNameAction = "set"
			senderNameValue = *req.SenderNameID
		}
	}

	res, err := h.db.ExecContext(r.Context(), `
		UPDATE operator_templates
		SET name           = COALESCE(NULLIF($2,''), name),
		    sender_name_id = CASE
		        WHEN $3 = 'clear' THEN NULL
		        WHEN $3 = 'set'   THEN $4::uuid
		        ELSE sender_name_id
		    END,
		    body           = COALESCE(NULLIF($5,''), body),
		    variables      = CASE WHEN $6 = '[]' OR $6 = 'null' THEN variables ELSE $6::jsonb END,
		    status         = COALESCE(NULLIF($7,''), status),
		    updated_at     = now()
		WHERE id = $1::uuid
	`, id, req.Name, senderNameAction, senderNameValue, req.Body, string(varsJSON), req.Status)
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

// loadModerationContext загружает текущий статус модерации шаблона + проверяет
// что он привязан к sender_name прямого клиента. Возвращает (status, found, eligible, error).
//
// BUG-36: разделяем три случая, чтобы не возвращать 404 при существующем шаблоне.
//   - found=false → 404 "шаблон не найден"
//   - eligible=false → 400 "требует привязки к sender_name прямого клиента"
//   - eligible=true → проверка currentStatus == 'submitted' на стороне вызывающего
func (h *OperatorTemplateHandlers) loadModerationContext(r *http.Request, id string) (string, bool, bool, error) {
	var (
		status   sql.NullString
		eligible bool
	)
	err := h.db.QueryRowContext(r.Context(), `
		SELECT
			ot.moderation_status,
			(sn.id IS NOT NULL AND c.parent_client_id IS NULL) AS eligible
		FROM operator_templates ot
		LEFT JOIN sender_names sn ON sn.id = ot.sender_name_id
		LEFT JOIN clients c       ON c.id = sn.client_id
		WHERE ot.id = $1
	`, id).Scan(&status, &eligible)
	if err == sql.ErrNoRows {
		return "", false, false, nil
	}
	if err != nil {
		return "", false, false, err
	}
	return status.String, true, eligible, nil
}

func validateModeratorNote(note string) (string, *shared.AppError) {
	trimmed := strings.TrimSpace(note)
	if len(trimmed) < operatorTemplateMinNoteLen {
		return "", shared.ErrInvalidInput(fmt.Sprintf("Комментарий модератора обязателен (минимум %d символов)", operatorTemplateMinNoteLen))
	}
	if len(trimmed) > operatorTemplateMaxNoteLen {
		return "", shared.ErrInvalidInput(fmt.Sprintf("Комментарий модератора слишком длинный (максимум %d символов)", operatorTemplateMaxNoteLen))
	}
	return trimmed, nil
}

// ApproveOperatorTemplate POST /admin/v1/operator-templates/{id}/approve
func (h *OperatorTemplateHandlers) ApproveOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	currentStatus, found, eligible, err := h.loadModerationContext(r, id)
	if err != nil {
		log.Error().Err(err).Msg("operator_templates: ошибка чтения moderation context")
		respondError(w, shared.ErrInternalServer("ошибка чтения шаблона"))
		return
	}
	if !found {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if !eligible {
		respondError(w, shared.ErrInvalidInput("модерация возможна только для шаблонов прямых клиентов с привязкой к sender_name"))
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
	h.transitionWithNote(w, r, "rejected", "reject")
}

// RequestRevisionOperatorTemplate POST /admin/v1/operator-templates/{id}/request-revision
func (h *OperatorTemplateHandlers) RequestRevisionOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	h.transitionWithNote(w, r, "revision_requested", "request-revision")
}

func (h *OperatorTemplateHandlers) transitionWithNote(w http.ResponseWriter, r *http.Request, newStatus, action string) {
	id := mux.Vars(r)["id"]

	var req struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	note, vErr := validateModeratorNote(req.Note)
	if vErr != nil {
		respondError(w, vErr)
		return
	}

	currentStatus, found, eligible, err := h.loadModerationContext(r, id)
	if err != nil {
		log.Error().Err(err).Msg("operator_templates: ошибка чтения moderation context")
		respondError(w, shared.ErrInternalServer("ошибка чтения шаблона"))
		return
	}
	if !found {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if !eligible {
		respondError(w, shared.ErrInvalidInput("модерация возможна только для шаблонов прямых клиентов с привязкой к sender_name"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput(action+" возможен только из статуса submitted"))
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_templates
		 SET moderation_status = $2, moderator_note = $3,
		     resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, newStatus, note,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"moderation_status": newStatus})
}
