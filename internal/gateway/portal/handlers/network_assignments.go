package handlers

// /reseller/network/assignments — назначение provider-set'ов и route-set'ов суб-аккаунтам агрегатора.
//
// Plan 2 Task 12: добавлены route_set_id в SRA + материализация route-set'а через RouteSetMaterializer +
// pre-validation pair (provider_set_id, route_set_id) через ConflictValidator. Conflict → 409 в PutOne /
// status='conflict' в Bulk. BulkDryRun — preview без мутаций.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	portal "github.com/smpp-server/smpp-server/internal/gateway/portal"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ProviderMaterializer — узкий interface для подмены ProviderSetMaterializer'а
// в тестах (failing-mock для partial-failure scenarios, Plan 3 Task 4).
// Production реализация: *network.ProviderSetMaterializer удовлетворяет
// автоматически (метод ApplyToClient с такой же сигнатурой).
type ProviderMaterializer interface {
	ApplyToClient(ctx context.Context, clientID uuid.UUID, providerSetID *uuid.UUID) error
}

// RouteMaterializer — узкий interface для подмены RouteSetMaterializer'а
// в тестах. Production реализация: *network.RouteSetMaterializer.
type RouteMaterializer interface {
	ApplyToClient(ctx context.Context, clientID uuid.UUID, routeSetID *uuid.UUID) error
}

// NetworkAssignmentsHandlers обрабатывает /portal/v1/reseller/network/assignments.
type NetworkAssignmentsHandlers struct {
	pool        *pgxpool.Pool
	providerMat ProviderMaterializer
	routeMat    RouteMaterializer
	validator   *network.ConflictValidator
}

// NewNetworkAssignmentsHandlers конструирует handler.
// providerMat / routeMat / validator могут быть nil — тогда соответствующие методы вернут 500
// при попытке материализации/валидации (использовать только в тестах, где конкретный пайплайн не задействован).
func NewNetworkAssignmentsHandlers(
	pool *pgxpool.Pool,
	pm ProviderMaterializer,
	rm RouteMaterializer,
	v *network.ConflictValidator,
) *NetworkAssignmentsHandlers {
	return &NetworkAssignmentsHandlers{pool: pool, providerMat: pm, routeMat: rm, validator: v}
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
	ValidationStatus string  `json:"validation_status"` // 'ok' | 'unassigned' | 'conflict'
	ValidationError  string  `json:"validation_error,omitempty"`
}

// List GET /portal/v1/reseller/network/assignments
//
// Включает unassigned суб-аккаунты (LEFT JOIN). Per-row валидация:
// если назначены оба set'а — запускаем ConflictValidator; missing-providers > 0 → 'conflict'.
// Если ни одного — 'unassigned'. Иначе — 'ok'.
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
		       rs.name AS route_set_name,
		       EXISTS(
		         SELECT 1 FROM client_providers cp
		         WHERE cp.client_id = c.id AND cp.ownership='private'
		       ) AS has_overrides
		FROM clients c
		LEFT JOIN subaccount_routing_assignment sra ON sra.client_id = c.id
		LEFT JOIN reseller_provider_sets ps ON ps.id = sra.provider_set_id
		LEFT JOIN reseller_route_sets rs ON rs.id = sra.route_set_id
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
		// Validation: если оба set'а назначены — проверить missing-providers.
		switch {
		case a.ProviderSetID != nil && a.RouteSetID != nil:
			a.ValidationStatus = "ok"
			if h.validator != nil {
				psUUID, errPS := uuid.Parse(*a.ProviderSetID)
				rsUUID, errRS := uuid.Parse(*a.RouteSetID)
				if errPS == nil && errRS == nil {
					missing, vErr := h.validator.MissingProviders(r.Context(), psUUID, rsUUID)
					if vErr != nil {
						log.Error().Err(vErr).Str("client_id", a.ClientID).Msg("assignments list validate")
						a.ValidationStatus = "error"
						a.ValidationError = vErr.Error()
					} else if len(missing) > 0 {
						a.ValidationStatus = "conflict"
						a.ValidationError = fmt.Sprintf("маршрут использует %d провайдер(ов) вне provider-set", len(missing))
					}
				}
			}
		case a.ProviderSetID == nil && a.RouteSetID == nil:
			a.ValidationStatus = "unassigned"
		default:
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

// parseAndVerifySetIDs парсит provider_set_id / route_set_id из тела запроса и
// проверяет, что оба принадлежат текущему reseller'у. nil/"" → возвращает (nil, nil, nil).
// Чужой / несуществующий ID → 404. Невалидный UUID → 400.
func (h *NetworkAssignmentsHandlers) parseAndVerifySetIDs(
	ctx context.Context, resellerID uuid.UUID, psStr, rsStr *string,
) (*uuid.UUID, *uuid.UUID, *shared.AppError) {
	var psUUID, rsUUID *uuid.UUID
	if psStr != nil && *psStr != "" {
		id, err := uuid.Parse(*psStr)
		if err != nil {
			return nil, nil, shared.ErrInvalidInput("provider_set_id")
		}
		var owner uuid.UUID
		if err := h.pool.QueryRow(ctx,
			`SELECT reseller_id FROM reseller_provider_sets WHERE id = $1`, id,
		).Scan(&owner); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, nil, shared.ErrNotFound("provider-set")
			}
			log.Error().Err(err).Str("provider_set_id", id.String()).Msg("verify provider-set ownership")
			return nil, nil, shared.ErrInternalServer("verify provider-set ownership")
		}
		if owner != resellerID {
			return nil, nil, shared.ErrNotFound("provider-set")
		}
		psUUID = &id
	}
	if rsStr != nil && *rsStr != "" {
		id, err := uuid.Parse(*rsStr)
		if err != nil {
			return nil, nil, shared.ErrInvalidInput("route_set_id")
		}
		var owner uuid.UUID
		if err := h.pool.QueryRow(ctx,
			`SELECT reseller_id FROM reseller_route_sets WHERE id = $1`, id,
		).Scan(&owner); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, nil, shared.ErrNotFound("route-set")
			}
			log.Error().Err(err).Str("route_set_id", id.String()).Msg("verify route-set ownership")
			return nil, nil, shared.ErrInternalServer("verify route-set ownership")
		}
		if owner != resellerID {
			return nil, nil, shared.ErrNotFound("route-set")
		}
		rsUUID = &id
	}
	return psUUID, rsUUID, nil
}

// uuidsToStrings — helper для сериализации списка UUID в JSON details.
func uuidsToStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// PutOne PUT /portal/v1/reseller/network/assignments/{client_id}
//
// UPSERT в subaccount_routing_assignment + materialize в client_providers И client_routes.
// Pre-validation: если назначены оба set'а — проверяем provider-инвариант (route-set не должен
// использовать провайдеров вне provider-set'а). Conflict → 409 details {kind: route_uses_unavailable_provider}.
//
// КОНТРАКТ ATOMICITY (как в Plan 1):
//  1. UPSERT SRA — отдельная транзакция.
//  2. providerMat.ApplyToClient — отдельная транзакция.
//  3. routeMat.ApplyToClient — отдельная транзакция.
//  4. Eventual consistency окно: при сбое любой материализации SRA уже committed.
//     Frontend retry'ит PUT — все шаги идемпотентны.
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

	psUUID, rsUUID, appErr := h.parseAndVerifySetIDs(r.Context(), resellerID, req.ProviderSetID, req.RouteSetID)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	// Pre-validation conflict (запускаем всегда, когда задан route-set; nil PS обрабатывается
	// валидатором как "все провайдеры route-set'а — missing", что ловит invariant §6.1:
	// route-set нельзя материализовать без provider-set'а).
	if rsUUID != nil {
		if h.validator == nil {
			log.Error().Str("client_id", clientID.String()).Msg("assignments PutOne: validator is nil")
			respondError(w, shared.ErrInternalServer("validator not configured"))
			return
		}
		missing, err := h.validator.MissingProvidersByIDs(r.Context(), psUUID, rsUUID)
		if err != nil {
			log.Error().Err(err).Str("client_id", clientID.String()).Msg("assignments PutOne validate")
			respondError(w, shared.ErrInternalServer("validate"))
			return
		}
		if len(missing) > 0 {
			details, _ := json.Marshal(map[string]interface{}{
				"kind":              "route_uses_unavailable_provider",
				"missing_providers": uuidsToStrings(missing),
			})
			respondError(w, shared.ErrConflict("маршрут использует провайдер вне provider-set").
				WithDetails(string(details)))
			return
		}
	}

	if _, err := h.pool.Exec(r.Context(), `
		INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (client_id) DO UPDATE SET
		  provider_set_id = EXCLUDED.provider_set_id,
		  route_set_id    = EXCLUDED.route_set_id,
		  assigned_at     = now()`,
		clientID, psUUID, rsUUID,
	); err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("assignments upsert")
		respondError(w, shared.ErrInternalServer("upsert"))
		return
	}

	// Misconfiguration (nil materializer) — это 500 (не partial-failure: пайплайн
	// собран неправильно, retry не поможет). Реальный ApplyToClient'овый сбой,
	// напротив, идемпотентно ретраится → 200 + warnings (Plan 3 Task 4).
	if h.providerMat == nil {
		log.Error().Str("client_id", clientID.String()).Msg("assignments PutOne: providerMat is nil")
		respondError(w, shared.ErrInternalServer("provider materializer not configured"))
		return
	}
	if h.routeMat == nil {
		log.Error().Str("client_id", clientID.String()).Msg("assignments PutOne: routeMat is nil")
		respondError(w, shared.ErrInternalServer("route materializer not configured"))
		return
	}

	warnings := []map[string]string{}
	if err := h.providerMat.ApplyToClient(r.Context(), clientID, psUUID); err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("assignments provider materialize partial-failure")
		portal.MaterializeFailureTotal.WithLabelValues("provider").Inc()
		warnings = append(warnings, map[string]string{
			"step":  "provider_materialize",
			"error": err.Error(),
		})
	}
	if err := h.routeMat.ApplyToClient(r.Context(), clientID, rsUUID); err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("assignments route materialize partial-failure")
		portal.MaterializeFailureTotal.WithLabelValues("route").Inc()
		warnings = append(warnings, map[string]string{
			"step":  "route_materialize",
			"error": err.Error(),
		})
	}

	resp := map[string]interface{}{"client_id": clientID.String()}
	if len(warnings) > 0 {
		resp["warnings"] = warnings
	}
	respondJSON(w, http.StatusOK, resp)
}

// bulkReq — тело POST /reseller/network/assignments/bulk и /bulk/dry-run.
type bulkReq struct {
	ClientIDs     []string `json:"client_ids"`
	ProviderSetID *string  `json:"provider_set_id"`
	RouteSetID    *string  `json:"route_set_id"`
}

// bulkResultItem — элемент response.results.
// Status: "ok" | "partial" | "error" | "conflict".
// "partial" (Plan 3 Task 4) — SRA committed, materialize failed; warnings содержит детали.
type bulkResultItem struct {
	ClientID string              `json:"client_id"`
	Status   string              `json:"status"`
	Error    string              `json:"error,omitempty"`
	Warnings []map[string]string `json:"warnings,omitempty"`
}

// Bulk POST /portal/v1/reseller/network/assignments/bulk
//
// Не атомарен per-client. Pre-validation pair (PS, RS) выполняется один раз;
// при конфликте — все client'ы получают status='conflict' (мутация не делается).
// Чужой sub-account / упавший UPSERT / упавший materialize → status='error' для конкретного.
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

	psUUID, rsUUID, appErr := h.parseAndVerifySetIDs(r.Context(), resellerID, req.ProviderSetID, req.RouteSetID)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	var (
		conflictExists bool
		missingCount   int
	)
	if rsUUID != nil {
		if h.validator == nil {
			respondError(w, shared.ErrInternalServer("validator not configured"))
			return
		}
		missing, err := h.validator.MissingProvidersByIDs(r.Context(), psUUID, rsUUID)
		if err != nil {
			log.Error().Err(err).Msg("assignments bulk validate")
			respondError(w, shared.ErrInternalServer("validate"))
			return
		}
		if len(missing) > 0 {
			conflictExists = true
			missingCount = len(missing)
		}
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
		if conflictExists {
			results = append(results, bulkResultItem{
				ClientID: idStr, Status: "conflict",
				Error: fmt.Sprintf("маршрут использует %d провайдер(ов) вне provider-set", missingCount),
			})
			continue
		}
		if _, err := h.pool.Exec(r.Context(), `
			INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
			VALUES ($1, $2, $3, now())
			ON CONFLICT (client_id) DO UPDATE SET
			  provider_set_id = EXCLUDED.provider_set_id,
			  route_set_id    = EXCLUDED.route_set_id,
			  assigned_at     = now()`,
			cid, psUUID, rsUUID,
		); err != nil {
			log.Error().Err(err).Str("client_id", cid.String()).Msg("assignments bulk upsert")
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "upsert failed"})
			continue
		}
		// Misconfiguration (nil) → status="error" (config issue, не partial-failure).
		if h.providerMat == nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "provider materializer not configured"})
			continue
		}
		if h.routeMat == nil {
			results = append(results, bulkResultItem{ClientID: idStr, Status: "error", Error: "route materializer not configured"})
			continue
		}
		// Plan 3 Task 4: materialize-сбой после committed SRA → status="partial"+warnings,
		// Prometheus counter alerter'у. Frontend ретраит идемпотентно.
		warnings := []map[string]string{}
		if err := h.providerMat.ApplyToClient(r.Context(), cid, psUUID); err != nil {
			log.Error().Err(err).Str("client_id", cid.String()).Msg("assignments bulk provider materialize partial-failure")
			portal.MaterializeFailureTotal.WithLabelValues("provider").Inc()
			warnings = append(warnings, map[string]string{"step": "provider_materialize", "error": err.Error()})
		}
		if err := h.routeMat.ApplyToClient(r.Context(), cid, rsUUID); err != nil {
			log.Error().Err(err).Str("client_id", cid.String()).Msg("assignments bulk route materialize partial-failure")
			portal.MaterializeFailureTotal.WithLabelValues("route").Inc()
			warnings = append(warnings, map[string]string{"step": "route_materialize", "error": err.Error()})
		}
		status := "ok"
		if len(warnings) > 0 {
			status = "partial"
		}
		results = append(results, bulkResultItem{ClientID: idStr, Status: status, Warnings: warnings})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}

// BulkDryRun POST /portal/v1/reseller/network/assignments/bulk/dry-run
//
// Preview: те же проверки, что и Bulk (ownership PS/RS, ownership sub-accounts, conflict-validation),
// но без UPSERT и без материализации. Возвращает per-client статусы.
func (h *NetworkAssignmentsHandlers) BulkDryRun(w http.ResponseWriter, r *http.Request) {
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
	psUUID, rsUUID, appErr := h.parseAndVerifySetIDs(r.Context(), resellerID, req.ProviderSetID, req.RouteSetID)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	var (
		conflictExists bool
		missingCount   int
	)
	if rsUUID != nil {
		if h.validator == nil {
			respondError(w, shared.ErrInternalServer("validator not configured"))
			return
		}
		missing, err := h.validator.MissingProvidersByIDs(r.Context(), psUUID, rsUUID)
		if err != nil {
			log.Error().Err(err).Msg("assignments bulk dry-run validate")
			respondError(w, shared.ErrInternalServer("validate"))
			return
		}
		if len(missing) > 0 {
			conflictExists = true
			missingCount = len(missing)
		}
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
		if conflictExists {
			results = append(results, bulkResultItem{
				ClientID: idStr, Status: "conflict",
				Error: fmt.Sprintf("маршрут использует %d провайдер(ов) вне provider-set", missingCount),
			})
			continue
		}
		results = append(results, bulkResultItem{ClientID: idStr, Status: "ok"})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}
