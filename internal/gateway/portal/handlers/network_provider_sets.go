package handlers

// /reseller/network/provider-sets — CRUD над provider-set'ами агрегатора.
//
// Provider-set — шаблон каталога провайдеров, который агрегатор создаёт у себя
// и привязывает к суб-аккаунтам через subaccount_routing_assignment. Один из
// set'ов может быть помечен is_default — он применяется к новым суб-аккаунтам
// без явного выбора. Уникальность default'а гарантируется частичным индексом
// uq_reseller_provider_sets_default; смена default'а делается транзакционно.

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

// NetworkProviderSetsHandlers обрабатывает /portal/v1/reseller/network/provider-sets.
//
// Materializer держим в поле, чтобы Task 8 (PUT items) и Task 9-10 (assignments,
// overrides) могли переиспользовать тот же handler-объект; в текущих CRUD-методах
// он не вызывается.
type NetworkProviderSetsHandlers struct {
	pool         *pgxpool.Pool
	setRepo      *storage.ResellerProviderSetRepository
	sraRepo      *storage.SubAccountRoutingAssignmentRepository
	materializer *network.ProviderSetMaterializer
}

// NewNetworkProviderSetsHandlers конструирует handler.
func NewNetworkProviderSetsHandlers(pool *pgxpool.Pool, mat *network.ProviderSetMaterializer) *NetworkProviderSetsHandlers {
	return &NetworkProviderSetsHandlers{
		pool:         pool,
		setRepo:      storage.NewResellerProviderSetRepository(pool),
		sraRepo:      storage.NewSubAccountRoutingAssignmentRepository(pool),
		materializer: mat,
	}
}

// reseller извлекает client_id из контекста (положенного session middleware).
func (h *NetworkProviderSetsHandlers) reseller(r *http.Request) (uuid.UUID, *shared.AppError) {
	cid, ok := middleware.GetClientID(r.Context())
	if !ok || cid == uuid.Nil {
		return uuid.Nil, shared.ErrUnauthorized("Клиент не найден")
	}
	return cid, nil
}

// verifyOwnership загружает set и проверяет, что он принадлежит resellerID.
// Возвращает 404 (а не 403) для чужого set'а — не светим существование.
func (h *NetworkProviderSetsHandlers) verifyOwnership(r *http.Request, resellerID, setID uuid.UUID) (*storage.ResellerProviderSet, *shared.AppError) {
	set, err := h.setRepo.GetByID(r.Context(), setID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, shared.ErrNotFound("provider-set")
		}
		log.Error().Err(err).Msg("provider-sets ownership lookup")
		return nil, shared.ErrInternalServer("ошибка чтения provider-set")
	}
	if set.ResellerID != resellerID {
		return nil, shared.ErrNotFound("provider-set")
	}
	return set, nil
}

// providerSetOut — JSON-форма provider-set'а с подсчётом items / assigned.
type providerSetOut struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsDefault     bool   `json:"is_default"`
	ItemCount     int    `json:"item_count"`
	AssignedCount int    `json:"assigned_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// providerSetReq — тело POST/PUT.
type providerSetReq struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

// List GET /portal/v1/reseller/network/provider-sets
//
// Возвращает все provider-set'ы текущего агрегатора с подсчётом items
// и количества назначенных суб-аккаунтов.
func (h *NetworkProviderSetsHandlers) List(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT s.id, s.name, s.is_default, s.created_at, s.updated_at,
		        (SELECT count(*) FROM reseller_provider_set_items i WHERE i.set_id = s.id) AS item_count,
		        (SELECT count(*) FROM subaccount_routing_assignment sra WHERE sra.provider_set_id = s.id) AS assigned_count
		   FROM reseller_provider_sets s
		  WHERE s.reseller_id = $1
		  ORDER BY s.created_at DESC`, resellerID)
	if err != nil {
		log.Error().Err(err).Msg("provider-sets list")
		respondError(w, shared.ErrInternalServer("ошибка получения provider-set'ов"))
		return
	}
	defer rows.Close()

	out := []providerSetOut{}
	for rows.Next() {
		var (
			id                   uuid.UUID
			name                 string
			isDefault            bool
			createdAt, updatedAt time.Time
			itemCount, assigned  int
		)
		if err := rows.Scan(&id, &name, &isDefault, &createdAt, &updatedAt, &itemCount, &assigned); err != nil {
			log.Error().Err(err).Msg("provider-sets scan")
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		out = append(out, providerSetOut{
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
		log.Error().Err(err).Msg("provider-sets rows.Err")
		respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"provider_sets": out})
}

// Create POST /portal/v1/reseller/network/provider-sets
//
// Создаёт новый set. Если is_default=true — внутри транзакции снимает старый
// default у того же reseller'а, чтобы не нарушить uq_reseller_provider_sets_default.
func (h *NetworkProviderSetsHandlers) Create(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	var req providerSetReq
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
		log.Error().Err(err).Msg("provider-sets create begin")
		respondError(w, shared.ErrInternalServer("ошибка создания"))
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	if req.IsDefault {
		if _, err := tx.Exec(r.Context(),
			`UPDATE reseller_provider_sets SET is_default=false, updated_at=now()
			 WHERE reseller_id=$1 AND is_default=true`, resellerID); err != nil {
			log.Error().Err(err).Msg("provider-sets create reset default")
			respondError(w, shared.ErrInternalServer("ошибка сброса default"))
			return
		}
	}

	var (
		id                   uuid.UUID
		createdAt, updatedAt time.Time
	)
	err = tx.QueryRow(r.Context(),
		`INSERT INTO reseller_provider_sets (reseller_id, name, is_default)
		 VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`,
		resellerID, req.Name, req.IsDefault,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(w, shared.ErrConflict("provider-set с таким именем уже существует"))
			return
		}
		log.Error().Err(err).Msg("provider-sets create insert")
		respondError(w, shared.ErrInternalServer("ошибка создания provider-set"))
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Error().Err(err).Msg("provider-sets create commit")
		respondError(w, shared.ErrInternalServer("ошибка commit"))
		return
	}

	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "create",
		ResourceType: "provider_set",
		ResourceID:   id.String(),
		Details:      map[string]interface{}{"name": req.Name, "is_default": req.IsDefault},
		IPAddress:    r.RemoteAddr,
	})

	respondJSON(w, http.StatusCreated, providerSetOut{
		ID:            id.String(),
		Name:          req.Name,
		IsDefault:     req.IsDefault,
		ItemCount:     0,
		AssignedCount: 0,
		CreatedAt:     createdAt.Format(time.RFC3339),
		UpdatedAt:     updatedAt.Format(time.RFC3339),
	})
}

// Update PUT /portal/v1/reseller/network/provider-sets/{id}
//
// Меняет name + is_default. Чужой set → 404. Если is_default=true — снимаем
// старый default транзакционно (за исключением самого обновляемого set'а).
func (h *NetworkProviderSetsHandlers) Update(w http.ResponseWriter, r *http.Request) {
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

	var req providerSetReq
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
		log.Error().Err(err).Msg("provider-sets update begin")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	if req.IsDefault {
		if _, err := tx.Exec(r.Context(),
			`UPDATE reseller_provider_sets SET is_default=false, updated_at=now()
			 WHERE reseller_id=$1 AND is_default=true AND id <> $2`, resellerID, id); err != nil {
			log.Error().Err(err).Msg("provider-sets update reset default")
			respondError(w, shared.ErrInternalServer("ошибка сброса default"))
			return
		}
	}

	tag, err := tx.Exec(r.Context(),
		`UPDATE reseller_provider_sets SET name=$1, is_default=$2, updated_at=now() WHERE id=$3`,
		req.Name, req.IsDefault, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(w, shared.ErrConflict("provider-set с таким именем уже существует"))
			return
		}
		log.Error().Err(err).Msg("provider-sets update")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	if tag.RowsAffected() == 0 {
		// Очень маловероятно после verifyOwnership, но обработаем гонку (DELETE параллельно).
		respondError(w, shared.ErrNotFound("provider-set"))
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Error().Err(err).Msg("provider-sets update commit")
		respondError(w, shared.ErrInternalServer("ошибка commit"))
		return
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "update",
		ResourceType: "provider_set",
		ResourceID:   id.String(),
		Details:      map[string]interface{}{"name": req.Name, "is_default": req.IsDefault},
		IPAddress:    r.RemoteAddr,
	})
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": id.String()})
}

// Delete DELETE /portal/v1/reseller/network/provider-sets/{id}
//
// Чужой set → 404. Если set назначен хотя бы одному суб-аккаунту → 409 с
// details {"kind":"set_assigned","count":N}. Иначе DELETE и 204.
func (h *NetworkProviderSetsHandlers) Delete(w http.ResponseWriter, r *http.Request) {
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
		`SELECT count(*) FROM subaccount_routing_assignment WHERE provider_set_id=$1`, id,
	).Scan(&assigned); err != nil {
		log.Error().Err(err).Msg("provider-sets delete count assignments")
		respondError(w, shared.ErrInternalServer("ошибка проверки назначений"))
		return
	}
	if assigned > 0 {
		respondError(w, shared.ErrConflict(fmt.Sprintf("provider-set назначен %d суб-аккаунту(ам)", assigned)).
			WithDetails(fmt.Sprintf(`{"kind":"set_assigned","count":%d}`, assigned)))
		return
	}

	if err := h.setRepo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			respondError(w, shared.ErrNotFound("provider-set"))
			return
		}
		log.Error().Err(err).Msg("provider-sets delete")
		respondError(w, shared.ErrInternalServer("ошибка удаления"))
		return
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "delete",
		ResourceType: "provider_set",
		ResourceID:   id.String(),
		Details:      map[string]interface{}{"id": id.String()},
		IPAddress:    r.RemoteAddr,
	})
	w.WriteHeader(http.StatusNoContent)
}
