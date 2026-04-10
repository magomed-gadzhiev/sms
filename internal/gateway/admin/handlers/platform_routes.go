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

// PlatformRoutesHandlers manages operator-based routes.
type PlatformRoutesHandlers struct {
	db *storage.DB
}

// NewPlatformRoutesHandlers создаёт обработчик.
func NewPlatformRoutesHandlers(db *storage.DB) *PlatformRoutesHandlers {
	return &PlatformRoutesHandlers{db: db}
}

type platformRouteRow struct {
	ID              string
	OperatorID      sql.NullString
	OperatorName    sql.NullString
	ChannelType     string
	ProviderID      string
	ProviderName    string
	LegalEntityID   sql.NullString
	LegalEntityName sql.NullString
	LegalEntityINN  sql.NullString
	Priority        int
	Active          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (row platformRouteRow) toJSON() map[string]interface{} {
	m := map[string]interface{}{
		"id":            row.ID,
		"channel_type":  row.ChannelType,
		"provider_id":   row.ProviderID,
		"provider_name": row.ProviderName,
		"priority":      row.Priority,
		"active":        row.Active,
		"created_at":    row.CreatedAt,
		"updated_at":    row.UpdatedAt,
	}
	if row.OperatorID.Valid && row.OperatorID.String != "" {
		m["operator_id"] = row.OperatorID.String
	} else {
		m["operator_id"] = nil
	}
	if row.OperatorName.Valid && row.OperatorName.String != "" {
		m["operator_name"] = row.OperatorName.String
	} else {
		m["operator_name"] = "All Networks"
	}
	if row.LegalEntityID.Valid && row.LegalEntityID.String != "" {
		m["legal_entity_id"] = row.LegalEntityID.String
		m["legal_entity_name"] = nullableString(&row.LegalEntityName.String)
		m["legal_entity_inn"] = nullableString(&row.LegalEntityINN.String)
	}
	return m
}

// nullableString returns nil for nil pointer or empty string, else the value.
func nullableString(s *string) interface{} {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}

// ListPlatformRoutes обрабатывает GET /admin/v1/platform-routes
func (h *PlatformRoutesHandlers) ListPlatformRoutes(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 200)
	offset := parseIntParam(r, "offset", 0)

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT
		    pr.id::text,
		    pr.operator_id::text,
		    o.name,
		    pr.channel_type,
		    pr.provider_id::text,
		    p.name,
		    pr.legal_entity_id::text,
		    le.name,
		    le.inn,
		    pr.priority,
		    pr.active,
		    pr.created_at,
		    pr.updated_at
		FROM platform_routes pr
		LEFT JOIN operators o  ON o.id = pr.operator_id
		JOIN  providers p      ON p.id = pr.provider_id
		LEFT JOIN legal_entities le ON le.id = pr.legal_entity_id
		ORDER BY pr.priority ASC, pr.created_at ASC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("platform_routes: ошибка списка")
		respondError(w, shared.ErrInternal("Ошибка получения маршрутов"))
		return
	}
	defer rows.Close()

	items := []map[string]interface{}{}
	for rows.Next() {
		var row platformRouteRow
		if err := rows.Scan(
			&row.ID, &row.OperatorID, &row.OperatorName,
			&row.ChannelType, &row.ProviderID, &row.ProviderName,
			&row.LegalEntityID, &row.LegalEntityName, &row.LegalEntityINN,
			&row.Priority, &row.Active, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			continue
		}
		items = append(items, row.toJSON())
	}

	var total int64
	_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM platform_routes`).Scan(&total)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"routes": items,
		"total":  total,
	})
}

type createPlatformRouteRequest struct {
	OperatorID    string `json:"operator_id"`
	ChannelType   string `json:"channel_type"`
	ProviderID    string `json:"provider_id"`
	LegalEntityID string `json:"legal_entity_id"`
	Priority      int    `json:"priority"`
}

// CreatePlatformRoute обрабатывает POST /admin/v1/platform-routes
func (h *PlatformRoutesHandlers) CreatePlatformRoute(w http.ResponseWriter, r *http.Request) {
	var req createPlatformRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.ProviderID == "" {
		respondError(w, shared.ErrInvalidInput("provider_id обязателен"))
		return
	}
	if req.ChannelType == "" {
		req.ChannelType = "sms"
	}
	if req.OperatorID == "" && req.LegalEntityID == "" {
		respondError(w, shared.ErrInvalidInput("При выборе All Networks необходимо указать legal_entity_id"))
		return
	}

	var newID string
	var createdAt time.Time
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO platform_routes (operator_id, channel_type, provider_id, legal_entity_id, priority)
		VALUES (NULLIF($1,'')::uuid, $2, $3::uuid, NULLIF($4,'')::uuid, $5)
		RETURNING id::text, created_at
	`, req.OperatorID, req.ChannelType, req.ProviderID, req.LegalEntityID, req.Priority).Scan(&newID, &createdAt)
	if err != nil {
		log.Error().Err(err).Msg("platform_routes: ошибка создания")
		respondError(w, shared.ErrInternal("Ошибка создания маршрута"))
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         newID,
		"created_at": createdAt,
	})
}

// UpdatePlatformRoute обрабатывает PUT /admin/v1/platform-routes/{id}
func (h *PlatformRoutesHandlers) UpdatePlatformRoute(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		OperatorID    *string `json:"operator_id"`
		ChannelType   string  `json:"channel_type"`
		ProviderID    string  `json:"provider_id"`
		LegalEntityID *string `json:"legal_entity_id"`
		Priority      *int    `json:"priority"`
		Active        *bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	res, err := h.db.ExecContext(r.Context(), `
		UPDATE platform_routes
		SET operator_id     = CASE WHEN $2 IS NULL THEN operator_id ELSE $2::uuid END,
		    channel_type    = COALESCE(NULLIF($3,''), channel_type),
		    provider_id     = COALESCE(NULLIF($4, '')::uuid, provider_id),
		    legal_entity_id = CASE WHEN $5 IS NULL THEN legal_entity_id ELSE $5::uuid END,
		    priority        = COALESCE($6, priority),
		    active          = COALESCE($7, active),
		    updated_at      = now()
		WHERE id = $1::uuid
	`, id, nullableString(req.OperatorID), req.ChannelType, req.ProviderID, nullableString(req.LegalEntityID), req.Priority, req.Active)
	if err != nil {
		log.Error().Err(err).Msg("platform_routes: ошибка обновления")
		respondError(w, shared.ErrInternal("Ошибка обновления маршрута"))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondError(w, shared.ErrNotFound("Маршрут не найден"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"updated": true,
	})
}

// DeletePlatformRoute обрабатывает DELETE /admin/v1/platform-routes/{id}
func (h *PlatformRoutesHandlers) DeletePlatformRoute(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	res, err := h.db.ExecContext(r.Context(), `DELETE FROM platform_routes WHERE id = $1::uuid`, id)
	if err != nil {
		log.Error().Err(err).Msg("platform_routes: ошибка удаления")
		respondError(w, shared.ErrInternal("Ошибка удаления маршрута"))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondError(w, shared.ErrNotFound("Маршрут не найден"))
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

type reorderItem struct {
	ID       string `json:"id"`
	Priority int    `json:"priority"`
}

// ReorderPlatformRoutes обрабатывает PUT /admin/v1/platform-routes/reorder
func (h *PlatformRoutesHandlers) ReorderPlatformRoutes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []reorderItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if len(req.Items) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{"updated": 0})
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Error().Err(err).Msg("platform_routes: ошибка начала транзакции")
		respondError(w, shared.ErrInternal("Ошибка переупорядочивания маршрутов"))
		return
	}
	defer func() { _ = tx.Rollback() }()

	var updated int64
	for _, item := range req.Items {
		res, err := tx.ExecContext(r.Context(), `
			UPDATE platform_routes SET priority = $2, updated_at = now() WHERE id = $1::uuid
		`, item.ID, item.Priority)
		if err != nil {
			log.Error().Err(err).Str("route_id", item.ID).Msg("platform_routes: ошибка reorder")
			respondError(w, shared.ErrInternal("Ошибка переупорядочивания маршрутов"))
			return
		}
		n, _ := res.RowsAffected()
		updated += n
	}

	if err := tx.Commit(); err != nil {
		log.Error().Err(err).Msg("platform_routes: ошибка commit")
		respondError(w, shared.ErrInternal("Ошибка сохранения порядка маршрутов"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"updated": updated,
	})
}
