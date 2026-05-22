package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// OptOutHandlers handles opt-out list CRUD via direct DB access.
type OptOutHandlers struct {
	pool *pgxpool.Pool
}

// NewOptOutHandlers creates a new OptOutHandlers.
func NewOptOutHandlers(pool *pgxpool.Pool) *OptOutHandlers {
	return &OptOutHandlers{pool: pool}
}

type optOutItem struct {
	ID         string    `json:"id"`
	ClientID   string    `json:"client_id"`
	Phone      string    `json:"phone"`
	Keyword    string    `json:"keyword"`
	OptedOutAt time.Time `json:"opted_out_at"`
}

// ListOptOuts handles GET /opt-out
func (h *OptOutHandlers) ListOptOuts(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	if h.pool == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"items": []interface{}{}, "total": 0})
		return
	}

	q := r.URL.Query()
	search := q.Get("search")
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	if perPage < 1 || perPage > 200 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	ctx := r.Context()

	var total int
	var rows interface{}

	if search != "" {
		if err := h.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM opt_out_list WHERE client_id = $1 AND phone ILIKE $2`,
			clientID, "%"+search+"%",
		).Scan(&total); err != nil {
			log.Error().Err(err).Msg("opt_out count error")
			respondError(w, shared.ErrInternalServer(err.Error()))
			return
		}
	} else {
		if err := h.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM opt_out_list WHERE client_id = $1`,
			clientID,
		).Scan(&total); err != nil {
			log.Error().Err(err).Msg("opt_out count error")
			respondError(w, shared.ErrInternalServer(err.Error()))
			return
		}
	}

	var queryRows interface {
		Next() bool
		Scan(dest ...interface{}) error
		Close()
		Err() error
	}

	var err error
	if search != "" {
		queryRows, err = h.pool.Query(ctx,
			`SELECT id, client_id, phone, COALESCE(keyword,''), opted_out_at
			 FROM opt_out_list WHERE client_id = $1 AND phone ILIKE $2
			 ORDER BY opted_out_at DESC LIMIT $3 OFFSET $4`,
			clientID, "%"+search+"%", perPage, offset,
		)
	} else {
		queryRows, err = h.pool.Query(ctx,
			`SELECT id, client_id, phone, COALESCE(keyword,''), opted_out_at
			 FROM opt_out_list WHERE client_id = $1
			 ORDER BY opted_out_at DESC LIMIT $2 OFFSET $3`,
			clientID, perPage, offset,
		)
	}
	if err != nil {
		log.Error().Err(err).Msg("opt_out query error")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	defer queryRows.Close()

	items := make([]optOutItem, 0, perPage)
	for queryRows.Next() {
		var item optOutItem
		if err := queryRows.Scan(&item.ID, &item.ClientID, &item.Phone, &item.Keyword, &item.OptedOutAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	if err := queryRows.Err(); err != nil {
		log.Error().Err(err).Msg("opt_out rows error")
	}
	_ = rows

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
	})
}

// AddOptOut handles POST /opt-out
func (h *OptOutHandlers) AddOptOut(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	if h.pool == nil {
		respondError(w, shared.ErrInternalServer("DB недоступен"))
		return
	}

	var req struct {
		Phone   string `json:"phone"`
		Keyword string `json:"keyword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	req.Phone = strings.TrimSpace(req.Phone)
	if req.Phone == "" {
		respondError(w, shared.ErrInvalidInput("phone обязателен"))
		return
	}

	ctx := r.Context()
	var item optOutItem
	err := h.pool.QueryRow(ctx, `
		INSERT INTO opt_out_list (client_id, phone, keyword)
		VALUES ($1, $2, $3)
		ON CONFLICT (client_id, phone) DO UPDATE SET keyword = EXCLUDED.keyword, opted_out_at = NOW()
		RETURNING id, client_id, phone, COALESCE(keyword,''), opted_out_at`,
		clientID, req.Phone, req.Keyword,
	).Scan(&item.ID, &item.ClientID, &item.Phone, &item.Keyword, &item.OptedOutAt)
	if err != nil {
		log.Error().Err(err).Msg("opt_out insert error")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	respondJSON(w, http.StatusCreated, item)
}

// RemoveOptOut handles DELETE /opt-out/{id}
func (h *OptOutHandlers) RemoveOptOut(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	if h.pool == nil {
		respondError(w, shared.ErrInternalServer("DB недоступен"))
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("id обязателен"))
		return
	}

	ctx := r.Context()
	result, err := h.pool.Exec(ctx,
		`DELETE FROM opt_out_list WHERE id = $1 AND client_id = $2`,
		id, clientID,
	)
	if err != nil {
		log.Error().Err(err).Msg("opt_out delete error")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	if result.RowsAffected() == 0 {
		respondError(w, shared.ErrNotFound("Запись не найдена"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ImportOptOut handles POST /opt-out/import — CSV list of phones (one per line)
func (h *OptOutHandlers) ImportOptOut(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	if h.pool == nil {
		respondError(w, shared.ErrInternalServer("DB недоступен"))
		return
	}

	var req struct {
		Phones []string `json:"phones"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if len(req.Phones) == 0 {
		respondError(w, shared.ErrInvalidInput("phones обязателен"))
		return
	}
	if len(req.Phones) > 10000 {
		respondError(w, shared.ErrInvalidInput("максимум 10000 телефонов за раз"))
		return
	}

	ctx := r.Context()
	var imported, skipped int
	for _, phone := range req.Phones {
		phone = strings.TrimSpace(phone)
		if phone == "" {
			continue
		}
		tag, err := h.pool.Exec(ctx, `
			INSERT INTO opt_out_list (client_id, phone)
			VALUES ($1, $2)
			ON CONFLICT (client_id, phone) DO NOTHING`,
			clientID, phone,
		)
		if err != nil {
			log.Warn().Err(err).Str("phone", phone).Msg("opt_out import skip")
			skipped++
			continue
		}
		if tag.RowsAffected() > 0 {
			imported++
		} else {
			skipped++
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"imported": imported,
		"skipped":  skipped,
	})
}
