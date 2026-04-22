// Package handlers: network_tariff_templates.go implements endpoints for
// reseller-owned tariff templates on the /network/tariffs page.
//
// Task 3 — GET /portal/v1/network/tariff-templates: list templates with
//   plans_count + bound_subaccount_count counters.
//
// Task 4 — POST /portal/v1/network/tariff-templates            — create (optional copy_from_id)
//          POST /portal/v1/network/tariff-templates/{id}/bind  — (re-)assign template to sub-accounts
//          POST /portal/v1/network/tariff-templates/{id}/duplicate — clone template + plans/periods/tiers
//
// All endpoints are gated by is_reseller=true on the caller. Non-resellers get 401,
// same convention as network_tariffs_summary. Writes invalidate the
// `tariffs:summary:<reseller_id>` Redis key so the /subaccounts-summary
// endpoint re-reads from the DB on next call.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// NetworkTariffTemplatesHandler serves the list of tariff templates for
// a reseller on the /network/tariffs page.
type NetworkTariffTemplatesHandler struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}

// NewNetworkTariffTemplatesHandler constructs the handler.
// redisClient may be nil — invalidation is then a no-op (tests w/o Redis).
func NewNetworkTariffTemplatesHandler(pool *pgxpool.Pool, redisClient *redis.Client) *NetworkTariffTemplatesHandler {
	return &NetworkTariffTemplatesHandler{pool: pool, redis: redisClient}
}

// TariffTemplateSummary is the per-row JSON payload.
type TariffTemplateSummary struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	PlansCount           int64     `json:"plans_count"`
	BoundSubaccountCount int64     `json:"bound_subaccount_count"`
	CreatedAt            time.Time `json:"created_at"`
}

// ensureReseller checks the caller is authenticated and is_reseller=true.
// Returns the reseller UUID on success. On failure it writes the response
// and returns (uuid.Nil, false).
func (h *NetworkTariffTemplatesHandler) ensureReseller(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return uuid.Nil, false
	}
	var isReseller bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT is_reseller FROM clients WHERE id = $1`, clientID,
	).Scan(&isReseller); err != nil || !isReseller {
		respondError(w, shared.ErrUnauthorized("доступ только для агрегаторов"))
		return uuid.Nil, false
	}
	return clientID, true
}

// invalidateSummaryCache deletes tariffs:summary:<reseller_id>. Best-effort.
func (h *NetworkTariffTemplatesHandler) invalidateSummaryCache(resellerID uuid.UUID) {
	if h.redis == nil {
		return
	}
	cctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.redis.Del(cctx, summaryCacheKeyPrefix+resellerID.String()).Err(); err != nil {
		log.Warn().Err(err).Msg("network_tariff_templates: redis DEL failed")
	}
}

// List handles GET /portal/v1/network/tariff-templates.
func (h *NetworkTariffTemplatesHandler) List(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.ensureReseller(w, r)
	if !ok {
		return
	}

	// Correlated subqueries return the two counters. reseller_tariff_plans
	// is scoped to active rows; sub_account_template_assignments has no
	// active flag (migration 000098, lines 94–102).
	const q = `
		SELECT tpl.id::text,
		       tpl.name,
		       COALESCE(tpl.description, ''),
		       (SELECT COUNT(*) FROM reseller_tariff_plans
		          WHERE template_id = tpl.id AND active),
		       (SELECT COUNT(*) FROM sub_account_template_assignments
		          WHERE template_id = tpl.id),
		       tpl.created_at
		FROM reseller_tariff_templates tpl
		WHERE tpl.reseller_id = $1 AND tpl.active
		ORDER BY tpl.name
	`

	rows, err := h.pool.Query(r.Context(), q, clientID)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: query failed")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	items := make([]TariffTemplateSummary, 0)
	for rows.Next() {
		var row TariffTemplateSummary
		if err := rows.Scan(
			&row.ID,
			&row.Name,
			&row.Description,
			&row.PlansCount,
			&row.BoundSubaccountCount,
			&row.CreatedAt,
		); err != nil {
			log.Error().Err(err).Msg("network_tariff_templates: scan failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: rows iter failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(items); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: encode failed")
	}
}

// createTemplateRequest — body for Create.
type createTemplateRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	CopyFromID  *string `json:"copy_from_id,omitempty"`
}

// createTemplateResponse — response body for Create/Duplicate.
type createTemplateResponse struct {
	ID string `json:"id"`
}

// Create handles POST /portal/v1/network/tariff-templates.
func (h *NetworkTariffTemplatesHandler) Create(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.ensureReseller(w, r)
	if !ok {
		return
	}

	var req createTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("некорректное тело запроса"))
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("имя шаблона обязательно"))
		return
	}

	var copyFrom *uuid.UUID
	if req.CopyFromID != nil && strings.TrimSpace(*req.CopyFromID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.CopyFromID))
		if err != nil {
			respondError(w, shared.ErrInvalidInput("copy_from_id некорректен"))
			return
		}
		copyFrom = &id
	}

	newID, appErr := h.createTemplateTx(r.Context(), clientID, req.Name, req.Description, copyFrom)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	h.invalidateSummaryCache(clientID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createTemplateResponse{ID: newID.String()})
}

// Duplicate handles POST /portal/v1/network/tariff-templates/{id}/duplicate.
// Equivalent to Create with copy_from_id = {id from URL}.
func (h *NetworkTariffTemplatesHandler) Duplicate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.ensureReseller(w, r)
	if !ok {
		return
	}

	idStr := mux.Vars(r)["id"]
	srcID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("id некорректен"))
		return
	}

	var req createTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("некорректное тело запроса"))
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("имя шаблона обязательно"))
		return
	}

	// Verify source template exists, is active, and belongs to caller — else 404.
	var exists bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(
		   SELECT 1 FROM reseller_tariff_templates
		   WHERE id = $1 AND reseller_id = $2 AND active
		 )`, srcID, clientID).Scan(&exists); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: duplicate existence check failed")
		respondError(w, shared.ErrInternalServer("ошибка проверки шаблона"))
		return
	}
	if !exists {
		respondError(w, shared.ErrNotFound("шаблон"))
		return
	}

	newID, appErr := h.createTemplateTx(r.Context(), clientID, req.Name, req.Description, &srcID)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	h.invalidateSummaryCache(clientID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createTemplateResponse{ID: newID.String()})
}

// createTemplateTx executes the full create-with-optional-copy transaction.
// On copy: scopes source to resellerID (403 if mismatch), copies plans then
// periods then tiers with explicit id-maps.
func (h *NetworkTariffTemplatesHandler) createTemplateTx(
	ctx context.Context,
	resellerID uuid.UUID,
	name, description string,
	copyFromID *uuid.UUID,
) (uuid.UUID, *shared.AppError) {
	// Pre-check name uniqueness among active templates of this reseller.
	// The DB has a partial unique index enforcing this; we explicit-check
	// first so we can return a clean 409 without relying on error-string
	// sniffing.
	var dupe bool
	if err := h.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM reseller_tariff_templates
		   WHERE reseller_id = $1 AND active AND name = $2
		 )`, resellerID, name).Scan(&dupe); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: name uniq check failed")
		return uuid.Nil, shared.ErrInternalServer("ошибка проверки имени")
	}
	if dupe {
		return uuid.Nil, shared.ErrConflict("шаблон с таким именем уже существует")
	}

	// Scope copy source to caller (403 on cross-reseller).
	if copyFromID != nil {
		var srcReseller uuid.UUID
		err := h.pool.QueryRow(ctx,
			`SELECT reseller_id FROM reseller_tariff_templates WHERE id = $1`,
			*copyFromID).Scan(&srcReseller)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return uuid.Nil, shared.ErrNotFound("шаблон-источник")
			}
			log.Error().Err(err).Msg("network_tariff_templates: copy-from scoping check failed")
			return uuid.Nil, shared.ErrInternalServer("ошибка проверки источника")
		}
		if srcReseller != resellerID {
			return uuid.Nil, shared.ErrForbidden("шаблон-источник принадлежит другому агрегатору")
		}
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: begin tx failed")
		return uuid.Nil, shared.ErrInternalServer("ошибка транзакции")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var newID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO reseller_tariff_templates (reseller_id, name, description, active)
		VALUES ($1, $2, NULLIF($3, ''), true)
		RETURNING id`, resellerID, name, description).Scan(&newID); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: insert template failed")
		return uuid.Nil, shared.ErrInternalServer("ошибка создания шаблона")
	}

	if copyFromID != nil {
		if appErr := copyTemplateChildren(ctx, tx, *copyFromID, newID, resellerID); appErr != nil {
			return uuid.Nil, appErr
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: commit failed")
		return uuid.Nil, shared.ErrInternalServer("ошибка сохранения")
	}
	return newID, nil
}

// copyTemplateChildren clones all active plans of srcID into dstID, along
// with periods and tiers. Sub-account assignments are NOT copied (§3 spec).
func copyTemplateChildren(ctx context.Context, tx pgx.Tx, srcID, dstID, resellerID uuid.UUID) *shared.AppError {
	// 1. Copy plans. Return old→new mapping via CTE.
	planMap := map[uuid.UUID]uuid.UUID{}
	rows, err := tx.Query(ctx, `
		WITH src AS (
			SELECT id, country_id, operator_id, sender_category, traffic_type, strategy, active
			FROM reseller_tariff_plans
			WHERE template_id = $1 AND active
		),
		ins AS (
			INSERT INTO reseller_tariff_plans
			  (reseller_id, template_id, country_id, operator_id, sender_category, traffic_type, strategy, active)
			SELECT $2, $3, country_id, operator_id, sender_category, traffic_type, strategy, active
			FROM src
			RETURNING id, country_id, operator_id, sender_category, traffic_type
		)
		SELECT s.id, i.id
		FROM src s
		JOIN ins i
		  ON COALESCE(s.country_id, '00000000-0000-0000-0000-000000000000'::uuid)
		   = COALESCE(i.country_id, '00000000-0000-0000-0000-000000000000'::uuid)
		 AND COALESCE(s.operator_id, '00000000-0000-0000-0000-000000000000'::uuid)
		   = COALESCE(i.operator_id, '00000000-0000-0000-0000-000000000000'::uuid)
		 AND s.sender_category = i.sender_category
		 AND s.traffic_type   = i.traffic_type
	`, srcID, resellerID, dstID)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: copy plans failed")
		return shared.ErrInternalServer("ошибка копирования планов")
	}
	for rows.Next() {
		var oldID, newID uuid.UUID
		if err := rows.Scan(&oldID, &newID); err != nil {
			rows.Close()
			log.Error().Err(err).Msg("network_tariff_templates: scan plan map failed")
			return shared.ErrInternalServer("ошибка копирования планов")
		}
		planMap[oldID] = newID
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: plan map iter failed")
		return shared.ErrInternalServer("ошибка копирования планов")
	}

	// 2. For each copied plan, copy its periods and tiers. We do this per
	//    plan so we can produce a period old→new map locally without
	//    relying on a composite natural key on periods (they have only dates).
	for oldPlan, newPlan := range planMap {
		periodMap := map[uuid.UUID]uuid.UUID{}
		prows, err := tx.Query(ctx, `
			WITH src AS (
				SELECT id, start_date, end_date
				FROM reseller_tariff_periods
				WHERE tariff_plan_id = $1
			),
			ins AS (
				INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
				SELECT $2, start_date, end_date FROM src
				RETURNING id, start_date, end_date
			)
			SELECT s.id, i.id
			FROM src s
			JOIN ins i
			  ON s.start_date = i.start_date
			 AND (s.end_date IS NOT DISTINCT FROM i.end_date)
		`, oldPlan, newPlan)
		if err != nil {
			log.Error().Err(err).Msg("network_tariff_templates: copy periods failed")
			return shared.ErrInternalServer("ошибка копирования периодов")
		}
		for prows.Next() {
			var oldPid, newPid uuid.UUID
			if err := prows.Scan(&oldPid, &newPid); err != nil {
				prows.Close()
				log.Error().Err(err).Msg("network_tariff_templates: scan period map failed")
				return shared.ErrInternalServer("ошибка копирования периодов")
			}
			periodMap[oldPid] = newPid
		}
		prows.Close()
		if err := prows.Err(); err != nil {
			log.Error().Err(err).Msg("network_tariff_templates: period map iter failed")
			return shared.ErrInternalServer("ошибка копирования периодов")
		}

		// 3. Copy tiers for each period.
		for oldPid, newPid := range periodMap {
			if _, err := tx.Exec(ctx, `
				INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
				SELECT $1, from_count, price_per_segment
				FROM reseller_tariff_tiers
				WHERE tariff_period_id = $2
			`, newPid, oldPid); err != nil {
				log.Error().Err(err).Msg("network_tariff_templates: copy tiers failed")
				return shared.ErrInternalServer("ошибка копирования тарифных ступеней")
			}
		}
	}

	return nil
}

// bindRequest — body for Bind.
type bindRequest struct {
	SubAccountIDs []string `json:"sub_account_ids"`
}

// bindReplaced — entry in Bind response `replaced` array.
type bindReplaced struct {
	SubAccountID  string `json:"sub_account_id"`
	OldTemplateID string `json:"old_template_id"`
}

// bindResponse — response body for Bind.
type bindResponse struct {
	Bound    []string       `json:"bound"`
	Replaced []bindReplaced `json:"replaced"`
}

// Bind handles POST /portal/v1/network/tariff-templates/{id}/bind.
func (h *NetworkTariffTemplatesHandler) Bind(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.ensureReseller(w, r)
	if !ok {
		return
	}

	idStr := mux.Vars(r)["id"]
	tplID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("id некорректен"))
		return
	}

	var req bindRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("некорректное тело запроса"))
		return
	}

	// Parse and dedupe sub-account ids.
	seen := map[uuid.UUID]struct{}{}
	subIDs := make([]uuid.UUID, 0, len(req.SubAccountIDs))
	for _, s := range req.SubAccountIDs {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			respondError(w, shared.ErrInvalidInput("sub_account_id некорректен: "+s))
			return
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		subIDs = append(subIDs, id)
	}
	if len(subIDs) == 0 {
		respondError(w, shared.ErrInvalidInput("sub_account_ids не должен быть пустым"))
		return
	}

	// 1. Verify target template is active and belongs to caller.
	var tplExists bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(
		   SELECT 1 FROM reseller_tariff_templates
		   WHERE id = $1 AND reseller_id = $2 AND active
		 )`, tplID, clientID).Scan(&tplExists); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: bind template check failed")
		respondError(w, shared.ErrInternalServer("ошибка проверки шаблона"))
		return
	}
	if !tplExists {
		respondError(w, shared.ErrNotFound("шаблон"))
		return
	}

	// 2. Verify every sub_account_id is a child of caller. Collect bad ids.
	validSubs := map[uuid.UUID]struct{}{}
	{
		vrows, err := h.pool.Query(r.Context(),
			`SELECT id FROM clients WHERE parent_client_id = $1 AND id = ANY($2::uuid[])`,
			clientID, subIDs)
		if err != nil {
			log.Error().Err(err).Msg("network_tariff_templates: bind sub-account check failed")
			respondError(w, shared.ErrInternalServer("ошибка проверки суб-аккаунтов"))
			return
		}
		for vrows.Next() {
			var id uuid.UUID
			if err := vrows.Scan(&id); err != nil {
				vrows.Close()
				respondError(w, shared.ErrInternalServer("ошибка проверки суб-аккаунтов"))
				return
			}
			validSubs[id] = struct{}{}
		}
		vrows.Close()
	}
	bad := make([]string, 0)
	for _, id := range subIDs {
		if _, ok := validSubs[id]; !ok {
			bad = append(bad, id.String())
		}
	}
	if len(bad) > 0 {
		respondError(w, shared.ErrInvalidInput("sub_account не принадлежит агрегатору: "+strings.Join(bad, ", ")))
		return
	}

	// 3. Transaction: collect prior bindings → delete → insert new.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: bind begin tx failed")
		respondError(w, shared.ErrInternalServer("ошибка транзакции"))
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	replaced := make([]bindReplaced, 0)
	rrows, err := tx.Query(r.Context(),
		`SELECT sub_account_id::text, template_id::text
		 FROM sub_account_template_assignments
		 WHERE sub_account_id = ANY($1::uuid[])`, subIDs)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: bind select prior failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения прежних привязок"))
		return
	}
	for rrows.Next() {
		var e bindReplaced
		if err := rrows.Scan(&e.SubAccountID, &e.OldTemplateID); err != nil {
			rrows.Close()
			respondError(w, shared.ErrInternalServer("ошибка чтения прежних привязок"))
			return
		}
		replaced = append(replaced, e)
	}
	rrows.Close()
	if err := rrows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка чтения прежних привязок"))
		return
	}

	if _, err := tx.Exec(r.Context(),
		`DELETE FROM sub_account_template_assignments WHERE sub_account_id = ANY($1::uuid[])`,
		subIDs); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: bind delete failed")
		respondError(w, shared.ErrInternalServer("ошибка удаления прежних привязок"))
		return
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO sub_account_template_assignments (sub_account_id, template_id)
		 SELECT unnest($1::uuid[]), $2::uuid`,
		subIDs, tplID); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: bind insert failed")
		respondError(w, shared.ErrInternalServer("ошибка создания привязок"))
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: bind commit failed")
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}

	h.invalidateSummaryCache(clientID)

	bound := make([]string, 0, len(subIDs))
	for _, id := range subIDs {
		bound = append(bound, id.String())
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(bindResponse{Bound: bound, Replaced: replaced})
}
