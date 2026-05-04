package handlers

// /reseller/network/assignments — назначение provider-set'ов суб-аккаунтам агрегатора.
//
// Plan 1: только provider-set часть. route_set_id из тела запросов парсится, но
// игнорируется (валидация owner'а route-set'а и материализация по нему появятся
// в Plan 2 после миграции reseller_route_sets). В таблице subaccount_routing_assignment
// колонка route_set_id уже существует (без FK), поэтому UPSERT всегда выставляет её в NULL,
// чтобы не залипали значения из старых записей.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// NetworkAssignmentsHandlers обрабатывает /portal/v1/reseller/network/assignments.
type NetworkAssignmentsHandlers struct {
	pool         *pgxpool.Pool
	materializer *network.ProviderSetMaterializer
}

// NewNetworkAssignmentsHandlers конструирует handler.
// mat можно передать nil — тогда PutOne/Bulk вернут 500 при попытке материализации
// (использовать только в тестах List, где материализатор не нужен).
func NewNetworkAssignmentsHandlers(pool *pgxpool.Pool, mat *network.ProviderSetMaterializer) *NetworkAssignmentsHandlers {
	return &NetworkAssignmentsHandlers{pool: pool, materializer: mat}
}

// assignmentOut — JSON-форма строки в ответе List.
// ProviderSet/RouteSet указатели → JSON null когда нет назначения.
type assignmentOut struct {
	ClientID         string  `json:"client_id"`
	SubAccountName   string  `json:"sub_account_name"`
	ProviderSetID    *string `json:"provider_set_id"`
	ProviderSetName  *string `json:"provider_set_name"`
	RouteSetID       *string `json:"route_set_id"`
	RouteSetName     *string `json:"route_set_name"`
	HasOverrides     bool    `json:"has_overrides"`
	ValidationStatus string  `json:"validation_status"` // 'ok' | 'unassigned' (Plan 2 добавит 'conflict')
}

// List GET /portal/v1/reseller/network/assignments
//
// Включает unassigned суб-аккаунты (LEFT JOIN). validation_status = 'unassigned'
// если provider_set_id NULL, иначе 'ok'. route_set_id и route_set_name всегда null
// (Plan 2 добавит JOIN на reseller_route_sets).
func (h *NetworkAssignmentsHandlers) List(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok || resellerID == uuid.Nil {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT c.id::text,
		       COALESCE(NULLIF(c.name, ''), c.email) AS name,
		       sra.provider_set_id::text,
		       ps.name AS provider_set_name,
		       sra.route_set_id::text,
		       NULL::text AS route_set_name,
		       EXISTS(
		         SELECT 1 FROM client_providers cp
		         WHERE cp.client_id = c.id AND cp.ownership='private'
		       ) AS has_overrides
		FROM clients c
		LEFT JOIN subaccount_routing_assignment sra ON sra.client_id = c.id
		LEFT JOIN reseller_provider_sets ps ON ps.id = sra.provider_set_id
		WHERE c.parent_client_id = $1
		ORDER BY name`, resellerID)
	if err != nil {
		log.Error().Err(err).Msg("assignments list query")
		respondError(w, shared.ErrInternalServer("query"))
		return
	}
	defer rows.Close()

	out := []assignmentOut{}
	for rows.Next() {
		var a assignmentOut
		if err := rows.Scan(
			&a.ClientID, &a.SubAccountName,
			&a.ProviderSetID, &a.ProviderSetName,
			&a.RouteSetID, &a.RouteSetName,
			&a.HasOverrides,
		); err != nil {
			log.Error().Err(err).Msg("assignments list scan")
			respondError(w, shared.ErrInternalServer("scan"))
			return
		}
		if a.ProviderSetID == nil {
			a.ValidationStatus = "unassigned"
		} else {
			a.ValidationStatus = "ok"
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("assignments list rows.Err")
		respondError(w, shared.ErrInternalServer("rows"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"assignments": out})
}

// putAssignmentReq — тело PUT /reseller/network/assignments/{client_id}.
type putAssignmentReq struct {
	ProviderSetID *string `json:"provider_set_id"`
	RouteSetID    *string `json:"route_set_id"`
}

// verifySubAccountOwnership возвращает 404, если client_id не суб-аккаунт текущего
// reseller'а. 404 (не 403) — чтобы не светить наличие чужих client_id.
// Connection / scan errors отделяются и возвращают 500, чтобы не маскировать
// инфраструктурные сбои под "не найдено".
func (h *NetworkAssignmentsHandlers) verifySubAccountOwnership(ctx context.Context, resellerID, clientID uuid.UUID) *shared.AppError {
	var parent uuid.UUID
	err := h.pool.QueryRow(ctx,
		`SELECT parent_client_id FROM clients WHERE id = $1 AND parent_client_id IS NOT NULL`,
		clientID,
	).Scan(&parent)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return shared.ErrNotFound("суб-аккаунт")
		}
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("verifySubAccountOwnership query")
		return shared.ErrInternalServer("verify sub-account ownership")
	}
	if parent != resellerID {
		return shared.ErrNotFound("суб-аккаунт")
	}
	return nil
}

// verifyProviderSetOwnership возвращает 404, если provider-set не принадлежит reseller'у.
// Connection / scan errors отделяются и возвращают 500.
func (h *NetworkAssignmentsHandlers) verifyProviderSetOwnership(ctx context.Context, resellerID, setID uuid.UUID) *shared.AppError {
	var owner uuid.UUID
	err := h.pool.QueryRow(ctx,
		`SELECT reseller_id FROM reseller_provider_sets WHERE id = $1`,
		setID,
	).Scan(&owner)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return shared.ErrNotFound("provider-set")
		}
		log.Error().Err(err).Str("provider_set_id", setID.String()).Msg("verifyProviderSetOwnership query")
		return shared.ErrInternalServer("verify provider-set ownership")
	}
	if owner != resellerID {
		return shared.ErrNotFound("provider-set")
	}
	return nil
}

// PutOne PUT /portal/v1/reseller/network/assignments/{client_id}
//
// UPSERT в subaccount_routing_assignment + materialize в client_providers.
// Если provider_set_id == null — стираем inherited записи у суб-аккаунта.
// route_set_id игнорируется (Plan 2).
//
// КОНТРАКТ ATOMICITY:
//  1. UPSERT subaccount_routing_assignment — отдельная транзакция.
//  2. materializer.ApplyToClient — отдельная транзакция (сама атомарна,
//     см. provider_set_materializer.go).
//  3. Между шагами есть eventual-consistency окно: если materialize падает,
//     SRA row уже committed → ответ 500. Frontend должен retry'ить тот же PUT —
//     операция идемпотентна (UPSERT не меняется, ApplyToClient повторно очищает
//     inherited и переписывает заново).
//  4. Пока retry не сделан — `validation_status='ok'` в List врёт. Принимаем
//     как ограничение Plan 1; в Plan 2 — обернём в общую tx с savepoints, либо
//     добавим background reconciler.
func (h *NetworkAssignmentsHandlers) PutOne(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok || resellerID == uuid.Nil {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	clientID, perr := uuid.Parse(mux.Vars(r)["client_id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("client_id"))
		return
	}
	if appErr := h.verifySubAccountOwnership(r.Context(), resellerID, clientID); appErr != nil {
		respondError(w, appErr)
		return
	}

	var req putAssignmentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}

	var providerSetUUID *uuid.UUID
	if req.ProviderSetID != nil && *req.ProviderSetID != "" {
		psid, err := uuid.Parse(*req.ProviderSetID)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("provider_set_id"))
			return
		}
		if appErr := h.verifyProviderSetOwnership(r.Context(), resellerID, psid); appErr != nil {
			respondError(w, appErr)
			return
		}
		providerSetUUID = &psid
	}
	// route_set_id игнорируется в Plan 1.

	// TODO(plan2): сохранять route_set_id из EXCLUDED при PATCH-семантике, когда добавится route-set
	if _, err := h.pool.Exec(r.Context(), `
		INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
		VALUES ($1, $2, NULL, now())
		ON CONFLICT (client_id) DO UPDATE SET
		  provider_set_id = EXCLUDED.provider_set_id,
		  route_set_id    = NULL,
		  assigned_at     = now()`,
		clientID, providerSetUUID,
	); err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("assignments upsert")
		respondError(w, shared.ErrInternalServer("upsert"))
		return
	}

	if h.materializer == nil {
		log.Error().Str("client_id", clientID.String()).Msg("assignments PutOne: materializer is nil")
		respondError(w, shared.ErrInternalServer("materializer not configured"))
		return
	}
	if err := h.materializer.ApplyToClient(r.Context(), clientID, providerSetUUID); err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("assignments materialize")
		respondError(w, shared.ErrInternalServer("materialize"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"client_id": clientID.String()})
}

// bulkReq — тело POST /reseller/network/assignments/bulk.
type bulkReq struct {
	ClientIDs     []string `json:"client_ids"`
	ProviderSetID *string  `json:"provider_set_id"`
	RouteSetID    *string  `json:"route_set_id"`
}

// bulkResultItem — элемент response.results.
type bulkResultItem struct {
	ClientID string `json:"client_id"`
	Status   string `json:"status"` // "ok" | "error"
	Error    string `json:"error,omitempty"`
}

// Bulk POST /portal/v1/reseller/network/assignments/bulk
//
// Не атомарен per-client: каждый ID обрабатывается независимо.
// Чужой sub-account / упавший UPSERT / упавший materialize → status='error' для конкретного,
// остальные продолжают обрабатываться. Provider-set ownership проверяется один раз в начале.
//
// УПОЛНОМОЧЕННАЯ JIT-консистентность per-client (см. PutOne): для каждого client_id
// UPSERT в SRA и materialize ApplyToClient — две независимые транзакции. При сбое
// materialize SRA уже committed → этот client получает status='error', но row
// остаётся. Frontend должен retry'ить bulk (или PutOne для упавших) — операция
// идемпотентна. Plan 2: общая tx с savepoints либо background reconciler.
func (h *NetworkAssignmentsHandlers) Bulk(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok || resellerID == uuid.Nil {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req bulkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}

	var providerSetUUID *uuid.UUID
	if req.ProviderSetID != nil && *req.ProviderSetID != "" {
		psid, err := uuid.Parse(*req.ProviderSetID)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("provider_set_id"))
			return
		}
		if appErr := h.verifyProviderSetOwnership(r.Context(), resellerID, psid); appErr != nil {
			respondError(w, appErr)
			return
		}
		providerSetUUID = &psid
	}

	results := make([]bulkResultItem, 0, len(req.ClientIDs))
	for _, idStr := range req.ClientIDs {
		cid, err := uuid.Parse(idStr)
		if err != nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "invalid id"})
			continue
		}
		if appErr := h.verifySubAccountOwnership(r.Context(), resellerID, cid); appErr != nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "not your sub-account"})
			continue
		}
		// TODO(plan2): сохранять route_set_id из EXCLUDED при PATCH-семантике, когда добавится route-set
		if _, err := h.pool.Exec(r.Context(), `
			INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
			VALUES ($1, $2, NULL, now())
			ON CONFLICT (client_id) DO UPDATE SET
			  provider_set_id = EXCLUDED.provider_set_id,
			  route_set_id    = NULL,
			  assigned_at     = now()`,
			cid, providerSetUUID,
		); err != nil {
			log.Error().Err(err).Str("client_id", cid.String()).Msg("assignments bulk upsert")
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "upsert failed"})
			continue
		}
		if h.materializer == nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "materializer not configured"})
			continue
		}
		if err := h.materializer.ApplyToClient(r.Context(), cid, providerSetUUID); err != nil {
			log.Error().Err(err).Str("client_id", cid.String()).Msg("assignments bulk materialize")
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "materialize failed"})
			continue
		}
		results = append(results, bulkResultItem{ClientID: idStr, Status: "ok"})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}
