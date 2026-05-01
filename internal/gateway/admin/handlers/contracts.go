package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// ContractHandlers обрабатывает CRUD для договоров.
type ContractHandlers struct {
	db *storage.DB
}

// NewContractHandlers создаёт обработчик.
func NewContractHandlers(db *storage.DB) *ContractHandlers {
	return &ContractHandlers{db: db}
}

type contractRow struct {
	ID              string
	ContractNumber  string
	ClientID        string
	ClientName      string
	LegalEntityID   sql.NullString
	LegalEntityINN  sql.NullString
	LegalEntityName sql.NullString
	Status          string
	StartDate       time.Time
	EndDate         sql.NullTime
	Description     sql.NullString
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (row contractRow) toJSON() map[string]interface{} {
	m := map[string]interface{}{
		"id":              row.ID,
		"contract_number": row.ContractNumber,
		"client_id":       row.ClientID,
		"client_name":     row.ClientName,
		"status":          row.Status,
		"start_date":      row.StartDate.Format("2006-01-02"),
		"created_at":      row.CreatedAt,
		"updated_at":      row.UpdatedAt,
	}
	if row.LegalEntityID.Valid {
		m["legal_entity_id"] = row.LegalEntityID.String
		m["legal_entity_inn"] = row.LegalEntityINN.String
		m["legal_entity_name"] = row.LegalEntityName.String
	}
	if row.EndDate.Valid {
		m["end_date"] = row.EndDate.Time.Format("2006-01-02")
	}
	if row.Description.Valid {
		m["description"] = row.Description.String
	}
	return m
}

func scanContract(row *sql.Row) (contractRow, error) {
	var r contractRow
	err := row.Scan(
		&r.ID, &r.ContractNumber, &r.ClientID, &r.ClientName,
		&r.LegalEntityID, &r.LegalEntityINN, &r.LegalEntityName,
		&r.Status, &r.StartDate, &r.EndDate, &r.Description,
		&r.CreatedAt, &r.UpdatedAt,
	)
	return r, err
}

const contractSelectQuery = `
	SELECT
		ct.id::text,
		ct.contract_number,
		ct.client_id::text,
		COALESCE(cl.name, '')  AS client_name,
		ct.legal_entity_id::text,
		le.inn,
		le.name               AS le_name,
		ct.status,
		ct.start_date,
		ct.end_date,
		ct.description,
		ct.created_at,
		ct.updated_at
	FROM contracts ct
	LEFT JOIN clients cl ON cl.id = ct.client_id
	LEFT JOIN legal_entities le ON le.id = ct.legal_entity_id
`

// ListContracts обрабатывает GET /admin/v1/contracts
func (h *ContractHandlers) ListContracts(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	status := r.URL.Query().Get("status")
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	args := []interface{}{}
	conditions := ""
	nextArg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if clientID != "" {
		conditions += " AND ct.client_id = " + nextArg(clientID) + "::uuid"
	}
	if status != "" {
		conditions += " AND ct.status = " + nextArg(status)
	}

	countQuery := `SELECT COUNT(*) FROM contracts ct WHERE 1=1` + conditions
	var total int64
	_ = h.db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total)

	// Добавляем limit и offset последними аргументами
	listArgs := append(args, limit, offset)
	limitPlaceholder := fmt.Sprintf("$%d", len(listArgs)-1)
	offsetPlaceholder := fmt.Sprintf("$%d", len(listArgs))

	listQuery := contractSelectQuery + `
		WHERE 1=1` + conditions + `
		ORDER BY ct.created_at DESC
		LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder

	rows, err := h.db.QueryContext(r.Context(), listQuery, listArgs...)
	if err != nil {
		log.Error().Err(err).Msg("contracts: ошибка списка")
		respondError(w, shared.ErrInternalServer("Ошибка получения договоров"))
		return
	}
	defer rows.Close()

	items := []map[string]interface{}{}
	for rows.Next() {
		var row contractRow
		if err := rows.Scan(
			&row.ID, &row.ContractNumber, &row.ClientID, &row.ClientName,
			&row.LegalEntityID, &row.LegalEntityINN, &row.LegalEntityName,
			&row.Status, &row.StartDate, &row.EndDate, &row.Description,
			&row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			continue
		}
		items = append(items, row.toJSON())
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"contracts": items,
		"total":     total,
	})
}

// GetContract обрабатывает GET /admin/v1/contracts/{id}
func (h *ContractHandlers) GetContract(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	row, err := scanContract(h.db.QueryRowContext(r.Context(),
		contractSelectQuery+` WHERE ct.id = $1::uuid`, id))
	if errors.Is(err, sql.ErrNoRows) {
		respondError(w, shared.ErrNotFound("Договор не найден"))
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("contracts: ошибка get")
		respondError(w, shared.ErrInternalServer("Ошибка получения"))
		return
	}
	respondJSON(w, http.StatusOK, row.toJSON())
}

type contractRequest struct {
	ContractNumber string `json:"contract_number"`
	ClientID       string `json:"client_id"`
	LegalEntityID  string `json:"legal_entity_id"`
	Status         string `json:"status"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	Description    string `json:"description"`
}

// CreateContract обрабатывает POST /admin/v1/contracts
func (h *ContractHandlers) CreateContract(w http.ResponseWriter, r *http.Request) {
	var req contractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.ContractNumber == "" || req.ClientID == "" || req.StartDate == "" {
		respondError(w, shared.ErrInvalidInput("contract_number, client_id и start_date обязательны"))
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}

	var newID string
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO contracts (contract_number, client_id, legal_entity_id, status, start_date, end_date, description)
		VALUES ($1, $2::uuid, NULLIF($3,'')::uuid, $4, $5::date, NULLIF($6,'')::date, NULLIF($7,''))
		RETURNING id::text
	`, req.ContractNumber, req.ClientID, req.LegalEntityID, status, req.StartDate, req.EndDate, req.Description).Scan(&newID)
	if err != nil {
		log.Error().Err(err).Msg("contracts: ошибка создания")
		respondError(w, shared.ErrInternalServer("Ошибка создания договора"))
		return
	}

	row, err := scanContract(h.db.QueryRowContext(r.Context(),
		contractSelectQuery+` WHERE ct.id = $1::uuid`, newID))
	if err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка получения созданного договора"))
		return
	}
	respondJSON(w, http.StatusCreated, row.toJSON())
}

// UpdateContract обрабатывает PUT /admin/v1/contracts/{id}
func (h *ContractHandlers) UpdateContract(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req contractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	res, err := h.db.ExecContext(r.Context(), `
		UPDATE contracts
		SET contract_number = COALESCE(NULLIF($2,''), contract_number),
		    legal_entity_id = CASE WHEN $3::text = '' THEN legal_entity_id ELSE $3::uuid END,
		    status          = COALESCE(NULLIF($4,''), status),
		    start_date      = CASE WHEN $5::text = '' THEN start_date ELSE $5::date END,
		    end_date        = NULLIF($6,'')::date,
		    description     = NULLIF($7,''),
		    updated_at      = now()
		WHERE id = $1::uuid
	`, id, req.ContractNumber, req.LegalEntityID, req.Status, req.StartDate, req.EndDate, req.Description)
	if err != nil {
		log.Error().Err(err).Msg("contracts: ошибка обновления")
		respondError(w, shared.ErrInternalServer("Ошибка обновления"))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondError(w, shared.ErrNotFound("Договор не найден"))
		return
	}

	row, err := scanContract(h.db.QueryRowContext(r.Context(),
		contractSelectQuery+` WHERE ct.id = $1::uuid`, id))
	if err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка получения обновлённого договора"))
		return
	}
	respondJSON(w, http.StatusOK, row.toJSON())
}

// DeleteContract обрабатывает DELETE /admin/v1/contracts/{id}
func (h *ContractHandlers) DeleteContract(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	res, err := h.db.ExecContext(r.Context(), `DELETE FROM contracts WHERE id = $1::uuid`, id)
	if err != nil {
		log.Error().Err(err).Msg("contracts: ошибка удаления")
		respondError(w, shared.ErrInternalServer("Ошибка удаления"))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondError(w, shared.ErrNotFound("Договор не найден"))
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
