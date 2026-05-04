package handlers

// /sub-accounts/{id}/network/* — overview + provider override CRUD на уровне суб-аккаунта.
//
// Plan 1: только провайдеры. route_set / route_overrides всегда возвращаются null/[]
// до Plan 2 (когда появятся reseller_route_sets).

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
	"github.com/smpp-server/smpp-server/internal/shared"
)

// SubAccountNetworkOverridesHandlers обслуживает /portal/v1/reseller/sub-accounts/{id}/network/*.
type SubAccountNetworkOverridesHandlers struct {
	pool *pgxpool.Pool
}

// NewSubAccountNetworkOverridesHandlers конструирует handler.
func NewSubAccountNetworkOverridesHandlers(pool *pgxpool.Pool) *SubAccountNetworkOverridesHandlers {
	return &SubAccountNetworkOverridesHandlers{pool: pool}
}

// verifyOwnership возвращает 404, если subID не суб-аккаунт текущего reseller'а.
// 404 (а не 403) — чтобы не светить наличие чужих client_id.
// pgx.ErrNoRows → 404; прочие ошибки → 500 (инфраструктура не должна
// маскироваться под "не найдено").
func (h *SubAccountNetworkOverridesHandlers) verifyOwnership(ctx context.Context, resellerID, subID uuid.UUID) *shared.AppError {
	var parent uuid.UUID
	err := h.pool.QueryRow(ctx,
		`SELECT parent_client_id FROM clients WHERE id = $1 AND parent_client_id IS NOT NULL`,
		subID,
	).Scan(&parent)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return shared.ErrNotFound("суб-аккаунт")
		}
		log.Error().Err(err).Str("sub_id", subID.String()).Msg("verifyOwnership query")
		return shared.ErrInternalServer("verify sub-account ownership")
	}
	if parent != resellerID {
		return shared.ErrNotFound("суб-аккаунт")
	}
	return nil
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
//     с details JSON `{"kind":"already_present","ownership":"<...>"}`. Изменение параметров
//     inherited-провайдера должно идти через provider-set, а не через override.
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
	if appErr := h.verifyOwnership(r.Context(), resellerID, subID); appErr != nil {
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

	// Уже ли есть запись (любой ownership) для этого sub+provider?
	var existing string
	err = h.pool.QueryRow(r.Context(),
		`SELECT ownership FROM client_providers WHERE client_id = $1 AND provider_id = $2`,
		subID, provID,
	).Scan(&existing)
	if err == nil {
		respondError(w,
			shared.ErrConflict("Этот провайдер уже доступен через шаблон. Чтобы изменить параметры — измени provider-set.").
				WithDetails(`{"kind":"already_present","ownership":"`+existing+`"}`))
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Error().Err(err).Str("sub_id", subID.String()).Str("provider_id", provID.String()).Msg("existing override lookup")
		respondError(w, shared.ErrInternalServer("existing override lookup"))
		return
	}

	if _, err := h.pool.Exec(r.Context(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, shared_priority, expose_cost, expose_provider_name, active)
		VALUES ($1, $2, 'private', $3, $4, $5, true)`,
		subID, provID, req.Priority, req.ExposeCost, req.ExposeProviderName,
	); err != nil {
		log.Error().Err(err).Str("sub_id", subID.String()).Str("provider_id", provID.String()).Msg("override insert")
		respondError(w, shared.ErrInternalServer("insert override"))
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"client_id":   subID.String(),
		"provider_id": provID.String(),
	})
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
	if appErr := h.verifyOwnership(r.Context(), resellerID, subID); appErr != nil {
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
	if appErr := h.verifyOwnership(r.Context(), resellerID, subID); appErr != nil {
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

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"provider_set":       providerSet,
		"route_set":          nil,           // Plan 2
		"provider_overrides": overrides,
		"route_overrides":    []interface{}{}, // Plan 2
	})
}
