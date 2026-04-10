package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// LegalEntityHandlers обрабатывает CRUD для юридических лиц.
type LegalEntityHandlers struct {
	db *storage.DB
}

// NewLegalEntityHandlers создаёт обработчик.
func NewLegalEntityHandlers(db *storage.DB) *LegalEntityHandlers {
	return &LegalEntityHandlers{db: db}
}

type legalEntityRow struct {
	ID        string
	INN       string
	Name      string
	FullName  sql.NullString
	Address   sql.NullString
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (r legalEntityRow) toJSON() map[string]interface{} {
	m := map[string]interface{}{
		"id":         r.ID,
		"inn":        r.INN,
		"name":       r.Name,
		"active":     r.Active,
		"created_at": r.CreatedAt,
		"updated_at": r.UpdatedAt,
	}
	if r.FullName.Valid {
		m["full_name"] = r.FullName.String
	}
	if r.Address.Valid {
		m["address"] = r.Address.String
	}
	return m
}

// ListLegalEntities обрабатывает GET /admin/v1/legal-entities
func (h *LegalEntityHandlers) ListLegalEntities(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id::text, inn, name, full_name, address, active, created_at, updated_at
		FROM legal_entities
		ORDER BY name
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("legal_entities: ошибка списка")
		respondError(w, shared.ErrInternalServer("Ошибка получения юридических лиц"))
		return
	}
	defer rows.Close()

	items := []map[string]interface{}{}
	for rows.Next() {
		var row legalEntityRow
		if err := rows.Scan(
			&row.ID, &row.INN, &row.Name, &row.FullName, &row.Address,
			&row.Active, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			continue
		}
		items = append(items, row.toJSON())
	}

	var total int64
	_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM legal_entities`).Scan(&total)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"legal_entities": items,
		"total":          total,
	})
}

// GetLegalEntity обрабатывает GET /admin/v1/legal-entities/{id}
func (h *LegalEntityHandlers) GetLegalEntity(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var row legalEntityRow
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id::text, inn, name, full_name, address, active, created_at, updated_at
		FROM legal_entities WHERE id = $1::uuid
	`, id).Scan(
		&row.ID, &row.INN, &row.Name, &row.FullName, &row.Address,
		&row.Active, &row.CreatedAt, &row.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		respondError(w, shared.ErrNotFound("Юридическое лицо не найдено"))
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("legal_entities: ошибка get")
		respondError(w, shared.ErrInternalServer("Ошибка получения"))
		return
	}
	respondJSON(w, http.StatusOK, row.toJSON())
}

type createLegalEntityRequest struct {
	INN      string `json:"inn"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Address  string `json:"address"`
}

// CreateLegalEntity обрабатывает POST /admin/v1/legal-entities
func (h *LegalEntityHandlers) CreateLegalEntity(w http.ResponseWriter, r *http.Request) {
	var req createLegalEntityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.INN == "" || req.Name == "" {
		respondError(w, shared.ErrInvalidInput("inn и name обязательны"))
		return
	}

	var row legalEntityRow
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO legal_entities (inn, name, full_name, address)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''))
		RETURNING id::text, inn, name, full_name, address, active, created_at, updated_at
	`, req.INN, req.Name, req.FullName, req.Address).Scan(
		&row.ID, &row.INN, &row.Name, &row.FullName, &row.Address,
		&row.Active, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		log.Error().Err(err).Msg("legal_entities: ошибка создания")
		respondError(w, shared.ErrInternalServer("Ошибка создания"))
		return
	}
	respondJSON(w, http.StatusCreated, row.toJSON())
}

// UpdateLegalEntity обрабатывает PUT /admin/v1/legal-entities/{id}
func (h *LegalEntityHandlers) UpdateLegalEntity(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		INN      string `json:"inn"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Address  string `json:"address"`
		Active   *bool  `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	var row legalEntityRow
	err := h.db.QueryRowContext(r.Context(), `
		UPDATE legal_entities
		SET inn       = COALESCE(NULLIF($2,''), inn),
		    name      = COALESCE(NULLIF($3,''), name),
		    full_name = NULLIF($4,''),
		    address   = NULLIF($5,''),
		    active    = COALESCE($6, active),
		    updated_at = now()
		WHERE id = $1::uuid
		RETURNING id::text, inn, name, full_name, address, active, created_at, updated_at
	`, id, req.INN, req.Name, req.FullName, req.Address, req.Active).Scan(
		&row.ID, &row.INN, &row.Name, &row.FullName, &row.Address,
		&row.Active, &row.CreatedAt, &row.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		respondError(w, shared.ErrNotFound("Юридическое лицо не найдено"))
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("legal_entities: ошибка обновления")
		respondError(w, shared.ErrInternalServer("Ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, row.toJSON())
}

// DeleteLegalEntity обрабатывает DELETE /admin/v1/legal-entities/{id}
func (h *LegalEntityHandlers) DeleteLegalEntity(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	res, err := h.db.ExecContext(r.Context(),
		`DELETE FROM legal_entities WHERE id = $1::uuid`, id)
	if err != nil {
		log.Error().Err(err).Msg("legal_entities: ошибка удаления")
		respondError(w, shared.ErrInternalServer("Ошибка удаления"))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondError(w, shared.ErrNotFound("Юридическое лицо не найдено"))
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
