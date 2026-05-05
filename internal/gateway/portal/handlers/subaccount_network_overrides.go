package handlers

// /sub-accounts/{id}/network/* — overview + provider override CRUD на уровне суб-аккаунта.
//
// Plan 2: добавлены route-overrides (POST/PUT/DELETE) и расширение Overview
// полями route_set + route_overrides (JOIN через subaccount_routing_assignment
// и client_routes WHERE source='override').

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
	"github.com/smpp-server/smpp-server/internal/storage"
)

// SubAccountNetworkOverridesHandlers обслуживает /portal/v1/reseller/sub-accounts/{id}/network/*.
type SubAccountNetworkOverridesHandlers struct {
	pool *pgxpool.Pool
}

// NewSubAccountNetworkOverridesHandlers конструирует handler.
func NewSubAccountNetworkOverridesHandlers(pool *pgxpool.Pool) *SubAccountNetworkOverridesHandlers {
	return &SubAccountNetworkOverridesHandlers{pool: pool}
}

// addProviderOverrideReq — тело POST /sub-accounts/{id}/network/provider-overrides.
type addProviderOverrideReq struct {
	ProviderID         string `json:"provider_id"`
	Priority           int    `json:"priority"`
	ExposeCost         bool   `json:"expose_cost"`
	ExposeProviderName bool   `json:"expose_provider_name"`
}

// AddProviderOverride POST /portal/v1/reseller/sub-accounts/{id}/network/provider-overrides.
//
// Семантика:
//   - провайдер должен принадлежать reseller'у (platform OR private с source_client_id == reseller).
//   - чужой private → 403; несуществующий провайдер → 404; чужой суб-аккаунт → 404.
//   - если запись (cp.client_id, cp.provider_id) уже существует (любой ownership) → 409
//     с details JSON `{"kind":"already_present","ownership":"<...>"}`. Message branched:
//     для inherited подсказываем менять provider-set; для private — нейтральное "уже добавлен".
//   - INSERT ownership='private' active=true.
func (h *SubAccountNetworkOverridesHandlers) AddProviderOverride(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok || resellerID == uuid.Nil {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	subID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id"))
		return
	}
	if appErr := middleware.VerifySubAccountOwnership(r.Context(), h.pool, resellerID, subID); appErr != nil {
		respondError(w, appErr)
		return
	}

	var req addProviderOverrideReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}
	provID, err := uuid.Parse(req.ProviderID)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("provider_id"))
		return
	}

	// Verify provider exists and принадлежит reseller'у.
	var ownership string
	var sourceClientID *uuid.UUID
	err = h.pool.QueryRow(r.Context(),
		`SELECT COALESCE(ownership, 'platform'), source_client_id FROM providers WHERE id = $1`,
		provID,
	).Scan(&ownership, &sourceClientID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, shared.ErrNotFound("провайдер"))
			return
		}
		log.Error().Err(err).Str("provider_id", provID.String()).Msg("provider lookup")
		respondError(w, shared.ErrInternalServer("provider lookup"))
		return
	}
	if ownership == "private" && (sourceClientID == nil || *sourceClientID != resellerID) {
		respondError(w, shared.ErrForbidden("чужой провайдер"))
		return
	}

	// Atomic upsert-style: пытаемся вставить ON CONFLICT DO NOTHING RETURNING id.
	// Если RETURNING пустой (pgx.ErrNoRows) — конфликт; делаем второй запрос
	// чтобы узнать ownership и вернуть нужный 409 message. Это устраняет TOCTOU
	// race между pre-check SELECT и INSERT.
	var insertedID uuid.UUID
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, shared_priority, expose_cost, expose_provider_name, active)
		VALUES ($1, $2, 'private', $3, $4, $5, true)
		ON CONFLICT (client_id, provider_id) DO NOTHING
		RETURNING id`,
		subID, provID, req.Priority, req.ExposeCost, req.ExposeProviderName,
	).Scan(&insertedID)
	if err == nil {
		_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
			TenantID:     resellerID,
			UserID:       userIDFromCtx(r.Context()),
			Action:       "create",
			ResourceType: "provider_override",
			ResourceID:   subID.String() + ":" + provID.String(),
			Details: map[string]interface{}{
				"sub_account_id":       subID.String(),
				"provider_id":          provID.String(),
				"priority":             req.Priority,
				"expose_cost":          req.ExposeCost,
				"expose_provider_name": req.ExposeProviderName,
			},
			IPAddress: r.RemoteAddr,
		})
		respondJSON(w, http.StatusCreated, map[string]interface{}{
			"client_id":   subID.String(),
			"provider_id": provID.String(),
		})
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		// Конфликт — fetch existing ownership для details/message branching.
		var existing string
		if err2 := h.pool.QueryRow(r.Context(),
			`SELECT ownership FROM client_providers WHERE client_id = $1 AND provider_id = $2`,
			subID, provID,
		).Scan(&existing); err2 != nil {
			log.Error().Err(err2).Str("sub_id", subID.String()).Str("provider_id", provID.String()).Msg("conflict lookup")
			respondError(w, shared.ErrInternalServer("conflict lookup"))
			return
		}
		msg := "Провайдер уже добавлен этому суб-аккаунту"
		if existing == "inherited" {
			msg = "Этот провайдер уже доступен через шаблон. Чтобы изменить параметры — измени provider-set."
		}
		respondError(w,
			shared.ErrConflict(msg).
				WithDetails(`{"kind":"already_present","ownership":"`+existing+`"}`))
		return
	}
	log.Error().Err(err).Str("sub_id", subID.String()).Str("provider_id", provID.String()).Msg("override insert")
	respondError(w, shared.ErrInternalServer("insert override"))
}

// DeleteProviderOverride DELETE /portal/v1/reseller/sub-accounts/{id}/network/provider-overrides/{provider_id}.
//
// Удаляет ТОЛЬКО private (override) запись. Inherited записи через override не трогаются —
// их нужно убирать через изменение provider-set.
func (h *SubAccountNetworkOverridesHandlers) DeleteProviderOverride(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok || resellerID == uuid.Nil {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	subID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id"))
		return
	}
	provID, perr := uuid.Parse(mux.Vars(r)["provider_id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("provider_id"))
		return
	}
	if appErr := middleware.VerifySubAccountOwnership(r.Context(), h.pool, resellerID, subID); appErr != nil {
		respondError(w, appErr)
		return
	}

	tag, err := h.pool.Exec(r.Context(),
		`DELETE FROM client_providers WHERE client_id = $1 AND provider_id = $2 AND ownership = 'private'`,
		subID, provID,
	)
	if err != nil {
		log.Error().Err(err).Str("sub_id", subID.String()).Str("provider_id", provID.String()).Msg("override delete")
		respondError(w, shared.ErrInternalServer("delete override"))
		return
	}
	if tag.RowsAffected() == 0 {
		respondError(w, shared.ErrNotFound("override"))
		return
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "delete",
		ResourceType: "provider_override",
		ResourceID:   subID.String() + ":" + provID.String(),
		Details: map[string]interface{}{
			"sub_account_id": subID.String(),
			"provider_id":    provID.String(),
		},
		IPAddress: r.RemoteAddr,
	})
	w.WriteHeader(http.StatusNoContent)
}

// setRef — компактная ссылка на provider-set / route-set в overview.
type setRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// providerOverrideOut — JSON-форма строки в provider_overrides.
type providerOverrideOut struct {
	ProviderID string `json:"provider_id"`
	Name       string `json:"name"`
	Priority   int    `json:"priority"`
	Ownership  string `json:"ownership"`
}

// Overview GET /portal/v1/reseller/sub-accounts/{id}/network/overview.
//
// Возвращает текущий provider-set (или null) и список private-провайдеров.
// route_set и route_overrides — null/[] (Plan 2).
func (h *SubAccountNetworkOverridesHandlers) Overview(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok || resellerID == uuid.Nil {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	subID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id"))
		return
	}
	if appErr := middleware.VerifySubAccountOwnership(r.Context(), h.pool, resellerID, subID); appErr != nil {
		respondError(w, appErr)
		return
	}

	var providerSet *setRef
	var psID, psName string
	err := h.pool.QueryRow(r.Context(), `
		SELECT ps.id::text, ps.name
		FROM subaccount_routing_assignment sra
		JOIN reseller_provider_sets ps ON ps.id = sra.provider_set_id
		WHERE sra.client_id = $1`, subID,
	).Scan(&psID, &psName)
	if err == nil {
		providerSet = &setRef{ID: psID, Name: psName}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		log.Error().Err(err).Str("sub_id", subID.String()).Msg("overview provider_set query")
		respondError(w, shared.ErrInternalServer("query provider_set"))
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT cp.provider_id::text, p.name, cp.shared_priority, cp.ownership
		FROM client_providers cp
		JOIN providers p ON p.id = cp.provider_id
		WHERE cp.client_id = $1 AND cp.ownership = 'private'
		ORDER BY p.name`, subID)
	if err != nil {
		log.Error().Err(err).Str("sub_id", subID.String()).Msg("overview overrides query")
		respondError(w, shared.ErrInternalServer("query overrides"))
		return
	}
	defer rows.Close()

	overrides := []providerOverrideOut{}
	for rows.Next() {
		var o providerOverrideOut
		if err := rows.Scan(&o.ProviderID, &o.Name, &o.Priority, &o.Ownership); err != nil {
			log.Error().Err(err).Str("sub_id", subID.String()).Msg("overview overrides scan")
			respondError(w, shared.ErrInternalServer("scan"))
			return
		}
		overrides = append(overrides, o)
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Str("sub_id", subID.String()).Msg("overview overrides rows.Err")
		respondError(w, shared.ErrInternalServer("rows"))
		return
	}

	// route_set — текущий назначенный route-set (или nil).
	var routeSet *setRef
	var rsID, rsName string
	err = h.pool.QueryRow(r.Context(), `
		SELECT rs.id::text, rs.name
		FROM subaccount_routing_assignment sra
		JOIN reseller_route_sets rs ON rs.id = sra.route_set_id
		WHERE sra.client_id = $1`, subID,
	).Scan(&rsID, &rsName)
	if err == nil {
		routeSet = &setRef{ID: rsID, Name: rsName}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		log.Error().Err(err).Str("sub_id", subID.String()).Msg("overview route_set query")
		respondError(w, shared.ErrInternalServer("query route_set"))
		return
	}

	// route_overrides — список override-маршрутов суб-аккаунта.
	overrideRows, err := h.pool.Query(r.Context(), `
		SELECT cr.id::text, COALESCE(cr.name, ''), cr.provider_id::text, p.name,
		       cr.priority, cr.status
		FROM client_routes cr
		JOIN providers p ON p.id = cr.provider_id
		WHERE cr.client_id = $1 AND cr.source = 'override'
		ORDER BY cr.priority DESC`, subID)
	if err != nil {
		log.Error().Err(err).Str("sub_id", subID.String()).Msg("overview route_overrides query")
		respondError(w, shared.ErrInternalServer("query route_overrides"))
		return
	}
	defer overrideRows.Close()
	routeOverrides := []map[string]interface{}{}
	for overrideRows.Next() {
		var rid, name, pid, pname, status string
		var priority int
		if err := overrideRows.Scan(&rid, &name, &pid, &pname, &priority, &status); err != nil {
			log.Error().Err(err).Str("sub_id", subID.String()).Msg("overview route_overrides scan")
			respondError(w, shared.ErrInternalServer("scan route_overrides"))
			return
		}
		routeOverrides = append(routeOverrides, map[string]interface{}{
			"id":            rid,
			"name":          name,
			"provider_id":   pid,
			"provider_name": pname,
			"priority":      priority,
			"status":        status,
		})
	}
	if err := overrideRows.Err(); err != nil {
		log.Error().Err(err).Str("sub_id", subID.String()).Msg("overview route_overrides rows.Err")
		respondError(w, shared.ErrInternalServer("rows route_overrides"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"provider_set":       providerSet,
		"route_set":          routeSet,
		"provider_overrides": overrides,
		"route_overrides":    routeOverrides,
	})
}

// addRouteOverrideReq — тело POST/PUT route-override.
// Переиспользует itemIn из network_route_set_items.go: тот же набор полей
// (provider_id, priority, share, route_type, status, condition_groups, schedules).
type addRouteOverrideReq struct {
	itemIn
}

// AddRouteOverride POST /portal/v1/reseller/sub-accounts/{id}/network/route-overrides.
//
// Создаёт client_routes row с source='override', owner_type='subaccount', owner_id=subID,
// operator_id=NULL. Provider_id обязан присутствовать в client_providers данного суб-аккаунта
// (любой ownership, active=true) — иначе 409 {kind:"route_provider_not_available"}.
func (h *SubAccountNetworkOverridesHandlers) AddRouteOverride(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok || resellerID == uuid.Nil {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	subID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id"))
		return
	}
	if appErr := middleware.VerifySubAccountOwnership(r.Context(), h.pool, resellerID, subID); appErr != nil {
		respondError(w, appErr)
		return
	}

	var req addRouteOverrideReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}
	full, err := parseItemIn(req.itemIn)
	if err != nil {
		respondError(w, shared.ErrInvalidInput(err.Error()))
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("route override begin tx")
		respondError(w, shared.ErrInternalServer("tx"))
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Pre-validation (spec §6.2 точка 4): провайдер должен быть доступен суб-аккаунту.
	// Проверка внутри транзакции — устраняет TOCTOU между check и INSERT.
	var available bool
	if err := tx.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM client_providers
		              WHERE client_id = $1 AND provider_id = $2 AND active = true)`,
		subID, full.ProviderID,
	).Scan(&available); err != nil {
		log.Error().Err(err).Str("sub_id", subID.String()).Str("provider_id", full.ProviderID.String()).Msg("route override availability check")
		respondError(w, shared.ErrInternalServer("availability check"))
		return
	}
	if !available {
		respondError(w, shared.ErrConflict("провайдер недоступен этому суб-аккаунту").
			WithDetails(`{"kind":"route_provider_not_available"}`))
		return
	}

	// Pre-check duplicate signature: после drop'а uq_cell_provider (миграция 000138/000139)
	// единственная защита от смыслового дубликата — handler-уровень. Считаем signature
	// от (provider_id, route_type, condition_groups) и ищем существующий override
	// с тем же signature внутри транзакции (TOCTOU-safe).
	sig := network.RouteSignature(full)
	existingID, dupErr := findDuplicateOverrideSignature(r.Context(), tx, subID, sig, uuid.Nil)
	if dupErr != nil {
		log.Error().Err(dupErr).Str("sub_id", subID.String()).Msg("route override duplicate-check")
		respondError(w, shared.ErrInternalServer("duplicate-check"))
		return
	}
	if existingID != uuid.Nil {
		details, _ := json.Marshal(map[string]interface{}{
			"kind":        "duplicate_route_signature",
			"existing_id": existingID.String(),
		})
		respondError(w, shared.ErrConflict("Такой override уже существует").WithDetails(string(details)))
		return
	}

	// owner_type='subaccount', owner_id=subID — обязательны (chk_owner_id, миграция 000137).
	var routeID uuid.UUID
	err = tx.QueryRow(r.Context(),
		`INSERT INTO client_routes
		   (client_id, operator_id, provider_id, priority, weight, active,
		    name, comment, status, share, route_type, source,
		    owner_type, owner_id)
		 VALUES ($1, NULL, $2, $3, 1, true, NULLIF($4,''), NULLIF($5,''), $6, $7, $8, 'override',
		         'subaccount', $1)
		 RETURNING id`,
		subID, full.ProviderID, full.Priority, full.Name, full.Comment,
		full.Status, full.Share, full.RouteType,
	).Scan(&routeID)
	if err != nil {
		log.Error().Err(err).Str("sub_id", subID.String()).Msg("route override insert")
		respondError(w, shared.ErrInternalServer("insert"))
		return
	}

	if err := writeRouteGroupsAndSchedules(r.Context(), tx, routeID, full); err != nil {
		log.Error().Err(err).Str("route_id", routeID.String()).Msg("route override children insert")
		respondError(w, shared.ErrInternalServer("groups/schedules"))
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Error().Err(err).Msg("route override commit")
		respondError(w, shared.ErrInternalServer("commit"))
		return
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "create",
		ResourceType: "route_override",
		ResourceID:   routeID.String(),
		Details: map[string]interface{}{
			"sub_account_id": subID.String(),
			"provider_id":    full.ProviderID.String(),
			"priority":       full.Priority,
		},
		IPAddress: r.RemoteAddr,
	})
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": routeID.String()})
}

// UpdateRouteOverride PUT /portal/v1/reseller/sub-accounts/{id}/network/route-overrides/{route_id}.
//
// Full replace: UPDATE поля + DELETE+re-INSERT condition_groups/schedules.
// 404 если route не принадлежит этому суб-аккаунту или source != 'override'.
func (h *SubAccountNetworkOverridesHandlers) UpdateRouteOverride(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok || resellerID == uuid.Nil {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	subID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id"))
		return
	}
	routeID, perr := uuid.Parse(mux.Vars(r)["route_id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("route_id"))
		return
	}
	if appErr := middleware.VerifySubAccountOwnership(r.Context(), h.pool, resellerID, subID); appErr != nil {
		respondError(w, appErr)
		return
	}

	// Verify route принадлежит этому sub-account и source='override'.
	var src string
	err := h.pool.QueryRow(r.Context(),
		`SELECT source FROM client_routes WHERE id = $1 AND client_id = $2`,
		routeID, subID,
	).Scan(&src)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, shared.ErrNotFound("override"))
			return
		}
		log.Error().Err(err).Str("route_id", routeID.String()).Msg("route override lookup")
		respondError(w, shared.ErrInternalServer("lookup"))
		return
	}
	if src != "override" {
		// template-маршруты редактируются только через route-set; не светим существование.
		respondError(w, shared.ErrNotFound("override"))
		return
	}

	var req addRouteOverrideReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}
	full, err := parseItemIn(req.itemIn)
	if err != nil {
		respondError(w, shared.ErrInvalidInput(err.Error()))
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("route override update begin tx")
		respondError(w, shared.ErrInternalServer("tx"))
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Provider availability check внутри транзакции (TOCTOU fix).
	var available bool
	if err := tx.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM client_providers
		              WHERE client_id = $1 AND provider_id = $2 AND active = true)`,
		subID, full.ProviderID,
	).Scan(&available); err != nil {
		log.Error().Err(err).Msg("route override update availability check")
		respondError(w, shared.ErrInternalServer("availability check"))
		return
	}
	if !available {
		respondError(w, shared.ErrConflict("провайдер недоступен этому суб-аккаунту").
			WithDetails(`{"kind":"route_provider_not_available"}`))
		return
	}

	// Pre-check duplicate signature (исключая текущий routeID, который мы апдейтим).
	sig := network.RouteSignature(full)
	existingID, dupErr := findDuplicateOverrideSignature(r.Context(), tx, subID, sig, routeID)
	if dupErr != nil {
		log.Error().Err(dupErr).Str("sub_id", subID.String()).Msg("route override update duplicate-check")
		respondError(w, shared.ErrInternalServer("duplicate-check"))
		return
	}
	if existingID != uuid.Nil {
		details, _ := json.Marshal(map[string]interface{}{
			"kind":        "duplicate_route_signature",
			"existing_id": existingID.String(),
		})
		respondError(w, shared.ErrConflict("Такой override уже существует").WithDetails(string(details)))
		return
	}

	// owner_type/owner_id уже выставлены при INSERT — не трогаем.
	// Defensive scoping: AND client_id + AND source='override' (как в Delete) —
	// гарантирует, что concurrent delete или скрещенный route_id ≠ нашему sub
	// не приведут к UPDATE чужой строки.
	tag, err := tx.Exec(r.Context(),
		`UPDATE client_routes
		   SET provider_id = $1, priority = $2, name = NULLIF($3,''), comment = NULLIF($4,''),
		       status = $5, share = $6, route_type = $7, updated_at = now()
		 WHERE id = $8 AND client_id = $9 AND source = 'override'`,
		full.ProviderID, full.Priority, full.Name, full.Comment,
		full.Status, full.Share, full.RouteType, routeID, subID,
	)
	if err != nil {
		log.Error().Err(err).Str("route_id", routeID.String()).Msg("route override update")
		respondError(w, shared.ErrInternalServer("update"))
		return
	}
	if tag.RowsAffected() == 0 {
		// concurrent delete или строка не подходит под scoped WHERE — 404.
		respondError(w, shared.ErrNotFound("override"))
		return
	}

	// route_conditions удалятся CASCADE при DELETE route_condition_groups.
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM route_condition_groups WHERE route_id = $1`, routeID); err != nil {
		log.Error().Err(err).Msg("route override delete groups")
		respondError(w, shared.ErrInternalServer("del groups"))
		return
	}
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM route_schedules WHERE route_id = $1`, routeID); err != nil {
		log.Error().Err(err).Msg("route override delete schedules")
		respondError(w, shared.ErrInternalServer("del schedules"))
		return
	}
	if err := writeRouteGroupsAndSchedules(r.Context(), tx, routeID, full); err != nil {
		log.Error().Err(err).Str("route_id", routeID.String()).Msg("route override children re-insert")
		respondError(w, shared.ErrInternalServer("groups/schedules"))
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Error().Err(err).Msg("route override update commit")
		respondError(w, shared.ErrInternalServer("commit"))
		return
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "update",
		ResourceType: "route_override",
		ResourceID:   routeID.String(),
		Details: map[string]interface{}{
			"sub_account_id": subID.String(),
			"provider_id":    full.ProviderID.String(),
			"priority":       full.Priority,
		},
		IPAddress: r.RemoteAddr,
	})
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": routeID.String()})
}

// DeleteRouteOverride DELETE /portal/v1/reseller/sub-accounts/{id}/network/route-overrides/{route_id}.
//
// Удаляет ТОЛЬКО override-маршрут (template — только через route-set).
func (h *SubAccountNetworkOverridesHandlers) DeleteRouteOverride(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok || resellerID == uuid.Nil {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	subID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id"))
		return
	}
	routeID, perr := uuid.Parse(mux.Vars(r)["route_id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("route_id"))
		return
	}
	if appErr := middleware.VerifySubAccountOwnership(r.Context(), h.pool, resellerID, subID); appErr != nil {
		respondError(w, appErr)
		return
	}
	tag, err := h.pool.Exec(r.Context(),
		`DELETE FROM client_routes
		 WHERE id = $1 AND client_id = $2 AND source = 'override'`,
		routeID, subID,
	)
	if err != nil {
		log.Error().Err(err).Str("route_id", routeID.String()).Msg("route override delete")
		respondError(w, shared.ErrInternalServer("delete"))
		return
	}
	if tag.RowsAffected() == 0 {
		respondError(w, shared.ErrNotFound("override"))
		return
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "delete",
		ResourceType: "route_override",
		ResourceID:   routeID.String(),
		Details:      map[string]interface{}{"sub_account_id": subID.String()},
		IPAddress:    r.RemoteAddr,
	})
	w.WriteHeader(http.StatusNoContent)
}

// findDuplicateOverrideSignature ищет в транзакции override-маршруты sub-аккаунта
// с тем же signature, что и кандидат на INSERT/UPDATE. excludeID — route, который
// мы апдейтим (исключается из поиска); uuid.Nil = ничего не исключать.
//
// Возвращает (uuid.Nil, nil) если дубликата нет.
func findDuplicateOverrideSignature(ctx context.Context, tx pgx.Tx, subID uuid.UUID, candidateSig string, excludeID uuid.UUID) (uuid.UUID, error) {
	rows, err := tx.Query(ctx, `
		SELECT cr.id, cr.provider_id, cr.route_type,
		       COALESCE(json_agg(json_build_object(
		         'group_index', g.group_index,
		         'logic_op',    g.logic_op,
		         'conditions',  COALESCE((SELECT json_agg(json_build_object('type', c.condition_type, 'value', c.condition_value) ORDER BY c.condition_type, c.condition_value) FROM route_conditions c WHERE c.group_id = g.id), '[]'::json)
		       ) ORDER BY g.group_index) FILTER (WHERE g.id IS NOT NULL), '[]'::json)
		FROM client_routes cr
		LEFT JOIN route_condition_groups g ON g.route_id = cr.id
		WHERE cr.client_id = $1 AND cr.source = 'override' AND ($2::uuid IS NULL OR cr.id <> $2)
		GROUP BY cr.id, cr.provider_id, cr.route_type`,
		subID, nullableUUID(excludeID),
	)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, provID uuid.UUID
		var routeType string
		var groupsJSON []byte
		if err := rows.Scan(&id, &provID, &routeType, &groupsJSON); err != nil {
			return uuid.Nil, err
		}
		var rawGroups []struct {
			GroupIndex int16  `json:"group_index"`
			LogicOp    string `json:"logic_op"`
			Conditions []struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"conditions"`
		}
		if err := json.Unmarshal(groupsJSON, &rawGroups); err != nil {
			return uuid.Nil, err
		}
		item := storage.RouteSetItemFull{ProviderID: provID, RouteType: routeType}
		for _, g := range rawGroups {
			grp := storage.RouteSetConditionGroup{GroupIndex: g.GroupIndex, LogicOp: g.LogicOp}
			for _, c := range g.Conditions {
				grp.Conditions = append(grp.Conditions, storage.RouteSetCondition{Type: c.Type, Value: c.Value})
			}
			item.ConditionGroups = append(item.ConditionGroups, grp)
		}
		if network.RouteSignature(item) == candidateSig {
			return id, nil
		}
	}
	return uuid.Nil, rows.Err()
}

// nullableUUID возвращает nil interface если id == uuid.Nil, иначе сам id.
// Нужно чтобы $2::uuid IS NULL ветка корректно работала.
func nullableUUID(id uuid.UUID) interface{} {
	if id == uuid.Nil {
		return nil
	}
	return id
}

// writeRouteGroupsAndSchedules вставляет condition_groups/conditions + schedules.
// Используется и в Add, и в Update (после DELETE старых дочерних строк).
func writeRouteGroupsAndSchedules(ctx context.Context, tx pgx.Tx, routeID uuid.UUID, full storage.RouteSetItemFull) error {
	for idx, g := range full.ConditionGroups {
		var gid int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO route_condition_groups (route_id, group_index, logic_op)
			 VALUES ($1, $2, $3) RETURNING id`,
			routeID, int16(idx), g.LogicOp,
		).Scan(&gid); err != nil {
			return err
		}
		for _, c := range g.Conditions {
			if _, err := tx.Exec(ctx,
				`INSERT INTO route_conditions (group_id, condition_type, condition_value)
				 VALUES ($1, $2, $3)`, gid, c.Type, c.Value); err != nil {
				return err
			}
		}
	}
	for _, s := range full.Schedules {
		if _, err := tx.Exec(ctx,
			`INSERT INTO route_schedules (route_id, date_from, date_to, time_from, time_to, weekdays, timezone)
			 VALUES ($1, $2, $3, $4::time, $5::time, $6, $7)`,
			routeID, s.DateFrom, s.DateTo, s.TimeFrom, s.TimeTo, s.Weekdays, s.Timezone); err != nil {
			return err
		}
	}
	return nil
}
