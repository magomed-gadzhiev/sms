package handlers

// /reseller/network/route-cleanup — каскадная зачистка orphan-маршрутов после удаления провайдера.
//
// Plan 2 Task 13: при удалении провайдера у reseller'а в его route-set'ах могут остаться
// items с этим provider_id, и у суб-аккаунтов — override-routes. Endpoint удаляет оба под
// одной транзакцией, после коммита перематериализует затронутые route-set'ы (каждый под
// своей транзакцией внутри материализатора).

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// NetworkCleanupHandlers — handler для /reseller/network/route-cleanup.
type NetworkCleanupHandlers struct {
	pool *pgxpool.Pool
	mat  *network.RouteSetMaterializer
}

// NewNetworkCleanupHandlers конструирует handler.
// mat может быть nil только в тестах, где материализация не нужна (после удаления items
// re-materialize пропускается, но удаление само по себе работает).
func NewNetworkCleanupHandlers(pool *pgxpool.Pool, mat *network.RouteSetMaterializer) *NetworkCleanupHandlers {
	return &NetworkCleanupHandlers{pool: pool, mat: mat}
}

// cleanupReq — тело POST /reseller/network/route-cleanup.
type cleanupReq struct {
	ProviderID string `json:"provider_id"`
}

// RouteCleanup POST /portal/v1/reseller/network/route-cleanup
//
// Удаляет:
//  1. reseller_route_set_items с provider_id=X в любом route-set'е этого reseller'а.
//  2. client_routes (source='override') суб-аккаунтов этого reseller'а с provider_id=X.
//
// Обе операции — под одной транзакцией. После commit'а — re-materialize затронутых
// route-set'ов (каждый под своей tx через ApplyToAllSubscribers). Re-materialize errors
// логируются, но не падают endpoint — items уже удалены.
func (h *NetworkCleanupHandlers) RouteCleanup(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok || resellerID == uuid.Nil {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req cleanupReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}
	provID, err := uuid.Parse(req.ProviderID)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("provider_id"))
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("route-cleanup begin tx")
		respondError(w, shared.ErrInternalServer("tx"))
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Найти затронутые route-set'ы перед удалением items — нужны для re-materialize after commit.
	var affectedSetIDs []uuid.UUID
	rows, err := tx.Query(r.Context(),
		`SELECT DISTINCT i.set_id FROM reseller_route_set_items i
		 JOIN reseller_route_sets s ON s.id = i.set_id
		 WHERE s.reseller_id = $1 AND i.provider_id = $2`,
		resellerID, provID)
	if err != nil {
		log.Error().Err(err).Msg("route-cleanup query affected sets")
		respondError(w, shared.ErrInternalServer("query"))
		return
	}
	func() {
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return
			}
			affectedSetIDs = append(affectedSetIDs, id)
		}
	}()
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("route-cleanup affected sets rows.Err")
		respondError(w, shared.ErrInternalServer("rows"))
		return
	}

	tag1, err := tx.Exec(r.Context(), `
		DELETE FROM reseller_route_set_items i
		 USING reseller_route_sets s
		 WHERE i.set_id = s.id AND s.reseller_id = $1 AND i.provider_id = $2`,
		resellerID, provID)
	if err != nil {
		log.Error().Err(err).Msg("route-cleanup delete items")
		respondError(w, shared.ErrInternalServer("delete items"))
		return
	}

	tag2, err := tx.Exec(r.Context(), `
		DELETE FROM client_routes cr
		 USING clients c
		 WHERE cr.client_id = c.id AND c.parent_client_id = $1
		   AND cr.provider_id = $2 AND cr.source = 'override'`,
		resellerID, provID)
	if err != nil {
		log.Error().Err(err).Msg("route-cleanup delete overrides")
		respondError(w, shared.ErrInternalServer("delete overrides"))
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Error().Err(err).Msg("route-cleanup commit")
		respondError(w, shared.ErrInternalServer("commit"))
		return
	}

	// Re-materialize — вне транзакции, каждый set под своей tx внутри материализатора.
	// Ошибки логируем, но 200 OK всё равно: items уже удалены, retry endpoint'а идемпотентен.
	if h.mat != nil {
		for _, sid := range affectedSetIDs {
			if err := h.mat.ApplyToAllSubscribers(r.Context(), sid); err != nil {
				log.Error().Err(err).Str("set_id", sid.String()).Msg("route-cleanup re-materialize")
				continue
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"removed_route_set_items": tag1.RowsAffected(),
		"removed_overrides":       tag2.RowsAffected(),
	})
}
