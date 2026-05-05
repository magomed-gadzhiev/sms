package handlers

// /reseller/network/route-sets — CRUD над route-set'ами агрегатора.
//
// Зеркало network_provider_sets.go: тот же verifyOwnership-паттерн (404 для
// чужого set'а), тот же 409 при assigned. Materializer держим в поле для
// будущих задач (PUT items / assign), в CRUD set'а он не вызывается.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// NetworkRouteSetsHandlers обрабатывает /portal/v1/reseller/network/route-sets.
type NetworkRouteSetsHandlers struct {
	pool         *pgxpool.Pool
	setRepo      *storage.ResellerRouteSetRepository
	materializer *network.RouteSetMaterializer
}

// NewNetworkRouteSetsHandlers конструирует handler.
func NewNetworkRouteSetsHandlers(pool *pgxpool.Pool, mat *network.RouteSetMaterializer) *NetworkRouteSetsHandlers {
	return &NetworkRouteSetsHandlers{
		pool:         pool,
		setRepo:      storage.NewResellerRouteSetRepository(pool),
		materializer: mat,
	}
}

// reseller извлекает client_id из контекста (положенного session middleware).
func (h *NetworkRouteSetsHandlers) reseller(r *http.Request) (uuid.UUID, *shared.AppError) {
	cid, ok := middleware.GetClientID(r.Context())
	if !ok || cid == uuid.Nil {
		return uuid.Nil, shared.ErrUnauthorized("Клиент не найден")
	}
	return cid, nil
}

// verifyOwnership загружает set и проверяет, что он принадлежит resellerID.
// Возвращает 404 (а не 403) для чужого set'а — не светим существование.
func (h *NetworkRouteSetsHandlers) verifyOwnership(r *http.Request, resellerID, setID uuid.UUID) (*storage.ResellerRouteSet, *shared.AppError) {
	set, err := h.setRepo.GetByID(r.Context(), setID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, shared.ErrNotFound("route-set")
		}
		log.Error().Err(err).Msg("route-sets ownership lookup")
		return nil, shared.ErrInternalServer("ошибка чтения route-set")
	}
	if set.ResellerID != resellerID {
		return nil, shared.ErrNotFound("route-set")
	}
	return set, nil
}

// routeSetOut — JSON-форма route-set'а с подсчётом items / assigned.
type routeSetOut struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsDefault     bool   `json:"is_default"`
	ItemCount     int    `json:"item_count"`
	AssignedCount int    `json:"assigned_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// routeSetReq — тело POST/PUT.
type routeSetReq struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

// List GET /portal/v1/reseller/network/route-sets
func (h *NetworkRouteSetsHandlers) List(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT s.id, s.name, s.is_default, s.created_at, s.updated_at,
		        (SELECT count(*) FROM reseller_route_set_items i WHERE i.set_id = s.id) AS item_count,
		        (SELECT count(*) FROM subaccount_routing_assignment sra WHERE sra.route_set_id = s.id) AS assigned_count
		   FROM reseller_route_sets s
		  WHERE s.reseller_id = $1
		  ORDER BY s.created_at DESC`, resellerID)
	if err != nil {
		log.Error().Err(err).Msg("route-sets list")
		respondError(w, shared.ErrInternalServer("ошибка получения route-set'ов"))
		return
	}
	defer rows.Close()

	out := []routeSetOut{}
	for rows.Next() {
		var (
			id                   uuid.UUID
			name                 string
			isDefault            bool
			createdAt, updatedAt time.Time
			itemCount, assigned  int
		)
		if err := rows.Scan(&id, &name, &isDefault, &createdAt, &updatedAt, &itemCount, &assigned); err != nil {
			log.Error().Err(err).Msg("route-sets scan")
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		out = append(out, routeSetOut{
			ID:            id.String(),
			Name:          name,
			IsDefault:     isDefault,
			ItemCount:     itemCount,
			AssignedCount: assigned,
			CreatedAt:     createdAt.Format(time.RFC3339),
			UpdatedAt:     updatedAt.Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("route-sets rows.Err")
		respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"route_sets": out})
}

// Create POST /portal/v1/reseller/network/route-sets
func (h *NetworkRouteSetsHandlers) Create(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	var req routeSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("name обязателен"))
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("route-sets create begin")
		respondError(w, shared.ErrInternalServer("ошибка создания"))
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	if req.IsDefault {
		if _, err := tx.Exec(r.Context(),
			`UPDATE reseller_route_sets SET is_default=false, updated_at=now()
			 WHERE reseller_id=$1 AND is_default=true`, resellerID); err != nil {
			log.Error().Err(err).Msg("route-sets create reset default")
			respondError(w, shared.ErrInternalServer("ошибка сброса default"))
			return
		}
	}

	var (
		id                   uuid.UUID
		createdAt, updatedAt time.Time
	)
	err = tx.QueryRow(r.Context(),
		`INSERT INTO reseller_route_sets (reseller_id, name, is_default)
		 VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`,
		resellerID, req.Name, req.IsDefault,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(w, shared.ErrConflict("route-set с таким именем уже существует"))
			return
		}
		log.Error().Err(err).Msg("route-sets create insert")
		respondError(w, shared.ErrInternalServer("ошибка создания route-set"))
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Error().Err(err).Msg("route-sets create commit")
		respondError(w, shared.ErrInternalServer("ошибка commit"))
		return
	}

	respondJSON(w, http.StatusCreated, routeSetOut{
		ID:            id.String(),
		Name:          req.Name,
		IsDefault:     req.IsDefault,
		ItemCount:     0,
		AssignedCount: 0,
		CreatedAt:     createdAt.Format(time.RFC3339),
		UpdatedAt:     updatedAt.Format(time.RFC3339),
	})
}

// Update PUT /portal/v1/reseller/network/route-sets/{id}
func (h *NetworkRouteSetsHandlers) Update(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	id, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}
	if _, ownErr := h.verifyOwnership(r, resellerID, id); ownErr != nil {
		respondError(w, ownErr)
		return
	}

	var req routeSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid JSON"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("name обязателен"))
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("route-sets update begin")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	if req.IsDefault {
		if _, err := tx.Exec(r.Context(),
			`UPDATE reseller_route_sets SET is_default=false, updated_at=now()
			 WHERE reseller_id=$1 AND is_default=true AND id <> $2`, resellerID, id); err != nil {
			log.Error().Err(err).Msg("route-sets update reset default")
			respondError(w, shared.ErrInternalServer("ошибка сброса default"))
			return
		}
	}

	tag, err := tx.Exec(r.Context(),
		`UPDATE reseller_route_sets SET name=$1, is_default=$2, updated_at=now() WHERE id=$3`,
		req.Name, req.IsDefault, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(w, shared.ErrConflict("route-set с таким именем уже существует"))
			return
		}
		log.Error().Err(err).Msg("route-sets update")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	if tag.RowsAffected() == 0 {
		respondError(w, shared.ErrNotFound("route-set"))
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Error().Err(err).Msg("route-sets update commit")
		respondError(w, shared.ErrInternalServer("ошибка commit"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": id.String()})
}

// Delete DELETE /portal/v1/reseller/network/route-sets/{id}
//
// Чужой set → 404. Если set назначен хотя бы одному суб-аккаунту → 409 с
// details {"kind":"route_set_assigned","count":N}. Иначе DELETE и 204.
func (h *NetworkRouteSetsHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	id, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}
	if _, ownErr := h.verifyOwnership(r, resellerID, id); ownErr != nil {
		respondError(w, ownErr)
		return
	}

	var assigned int
	if err := h.pool.QueryRow(r.Context(),
		`SELECT count(*) FROM subaccount_routing_assignment WHERE route_set_id=$1`, id,
	).Scan(&assigned); err != nil {
		log.Error().Err(err).Msg("route-sets delete count assignments")
		respondError(w, shared.ErrInternalServer("ошибка проверки назначений"))
		return
	}
	if assigned > 0 {
		respondError(w, shared.ErrConflict(fmt.Sprintf("route-set назначен %d суб-аккаунту(ам)", assigned)).
			WithDetails(fmt.Sprintf(`{"kind":"route_set_assigned","count":%d}`, assigned)))
		return
	}

	if err := h.setRepo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			respondError(w, shared.ErrNotFound("route-set"))
			return
		}
		log.Error().Err(err).Msg("route-sets delete")
		respondError(w, shared.ErrInternalServer("ошибка удаления"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
