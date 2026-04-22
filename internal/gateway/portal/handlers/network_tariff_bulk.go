// Package handlers: network_tariff_bulk.go implements two writer endpoints
// for the /network/tariffs matrix editor.
//
// Task 6.1 — PATCH /portal/v1/network/tariff-plans/{plan_id}/bulk
//            Batch upsert/delete of tiers + cells on a single plan.
// Task 6.2 — POST  /portal/v1/network/tariff-plans/{plan_id}/periods
//            Create a new period (optionally copying tiers from an existing one).
//
// Gate: is_reseller=true on the caller. Plan ownership is verified through
// the plan's template_id → reseller_tariff_templates.reseller_id for
// template-scoped plans, or directly via reseller_tariff_plans.reseller_id
// for sub-account override plans.
//
// ## Design decision: bulk PATCH targets ONE plan
//
// The URL is `/tariff-plans/{plan_id}/bulk`, singular. The brief notes that
// cells have an operator_id but plans are already per-operator — so a single
// bulk call cannot span multiple operators. We enforce this honestly: each
// `cells_upsert` / `cells_delete` entry's operator_id MUST equal plan.operator_id
// (or the plan must be the wildcard plan with operator_id IS NULL, in which
// case cell.operator_id is currently accepted as the plan's intended operator
// but still validated against the plan's scope — see below). Scope=override
// cells may optionally point at a different target plan for this sub-account
// (the override plan for the same dimensions + sub_account_id); we find-or-
// create that override plan lazily and write there.
//
// Frontend orchestrates N PATCHes for N operator-plans — one per operator.
//
// ## Transaction semantics
//
// Validation runs first (no DB state mutated). If any per-cell validation
// fails, we return 400 with an `errors` array and leave the DB untouched.
// Once validation passes, all tier/cell writes happen in one transaction.
//
// ## Cache invalidation
//
// After a successful commit, `tariffs:summary:<reseller_id>` is DEL'd. The
// next `/subaccounts-summary` read rebuilds the cache.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// NetworkTariffBulkHandler serves PATCH .../bulk and POST .../periods.
type NetworkTariffBulkHandler struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}

// NewNetworkTariffBulkHandler constructs the handler.
// redisClient may be nil — cache invalidation is then a no-op.
func NewNetworkTariffBulkHandler(pool *pgxpool.Pool, redisClient *redis.Client) *NetworkTariffBulkHandler {
	return &NetworkTariffBulkHandler{pool: pool, redis: redisClient}
}

// ---------- Bulk PATCH ----------

type bulkTierUpsert struct {
	ID           *string `json:"id"` // nil → INSERT new
	FromQuantity int     `json:"from_quantity"`
}

type bulkCellUpsert struct {
	OperatorID   string  `json:"operator_id"`
	TierID       string  `json:"tier_id"`
	Price        float64 `json:"price"`
	Scope        string  `json:"scope"` // "template" | "override"
	SubAccountID *string `json:"sub_account_id,omitempty"`
}

type bulkCellDelete struct {
	OperatorID   string  `json:"operator_id"`
	TierID       string  `json:"tier_id"`
	Scope        string  `json:"scope"`
	SubAccountID *string `json:"sub_account_id,omitempty"`
}

type bulkPatchRequest struct {
	PeriodID     string           `json:"period_id"`
	TiersUpsert  []bulkTierUpsert `json:"tiers_upsert"`
	TiersDelete  []string         `json:"tiers_delete"`
	CellsUpsert  []bulkCellUpsert `json:"cells_upsert"`
	CellsDelete  []bulkCellDelete `json:"cells_delete"`
}

type bulkError struct {
	OperatorID string `json:"operator_id,omitempty"`
	TierID     string `json:"tier_id,omitempty"`
	Reason     string `json:"reason"`
}

type bulkResponse struct {
	OK     bool        `json:"ok"`
	Errors []bulkError `json:"errors,omitempty"`
}

const (
	bulkPriceMin = 0.0
	bulkPriceMax = 999.0
)

// BulkPatch handles PATCH /portal/v1/network/tariff-plans/{plan_id}/bulk.
func (h *NetworkTariffBulkHandler) BulkPatch(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.ensureReseller(w, r)
	if !ok {
		return
	}

	planID, err := uuid.Parse(mux.Vars(r)["plan_id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("plan_id некорректен"))
		return
	}

	var req bulkPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("некорректное тело запроса"))
		return
	}

	periodID, err := uuid.Parse(strings.TrimSpace(req.PeriodID))
	if err != nil {
		respondError(w, shared.ErrInvalidInput("period_id некорректен"))
		return
	}

	ctx := r.Context()

	// 1. Load plan + verify ownership.
	plan, err := h.loadPlanScoped(ctx, planID, resellerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, shared.ErrNotFound("план"))
			return
		}
		log.Error().Err(err).Msg("network_tariff_bulk: load plan failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения плана"))
		return
	}

	// 2. Verify period belongs to plan.
	var periodPlanID uuid.UUID
	if err := h.pool.QueryRow(ctx,
		`SELECT tariff_plan_id FROM reseller_tariff_periods WHERE id = $1`, periodID,
	).Scan(&periodPlanID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeBulkError(w, http.StatusBadRequest, "period_id не найден")
			return
		}
		log.Error().Err(err).Msg("network_tariff_bulk: load period failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения периода"))
		return
	}
	if periodPlanID != planID {
		writeBulkError(w, http.StatusBadRequest, "period не принадлежит этому плану")
		return
	}

	// 3. Validate all input BEFORE opening tx. Collects errors per-cell.
	validationErrs := h.validateBulkInput(ctx, resellerID, plan, &req)
	if len(validationErrs) > 0 {
		writeBulkErrors(w, http.StatusBadRequest, validationErrs)
		return
	}

	// 4. Single transaction: tiers_upsert → tiers_delete → cells_upsert → cells_delete.
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: begin tx failed")
		respondError(w, shared.ErrInternalServer("ошибка транзакции"))
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Tiers (live on periodID, which belongs to the URL-path plan).
	for _, t := range req.TiersUpsert {
		if t.ID == nil || strings.TrimSpace(*t.ID) == "" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
				VALUES ($1, $2, 0)`, periodID, t.FromQuantity); err != nil {
				if isBulkUniqueViolation(err) {
					writeBulkError(w, http.StatusBadRequest,
						fmt.Sprintf("tier from_count=%d уже существует", t.FromQuantity))
					return
				}
				log.Error().Err(err).Msg("network_tariff_bulk: insert tier failed")
				respondError(w, shared.ErrInternalServer("ошибка создания ступени"))
				return
			}
		} else {
			tierID, err := uuid.Parse(*t.ID)
			if err != nil {
				writeBulkError(w, http.StatusBadRequest, "tier id некорректен")
				return
			}
			ct, err := tx.Exec(ctx, `
				UPDATE reseller_tariff_tiers SET from_count = $1
				WHERE id = $2 AND tariff_period_id = $3`, t.FromQuantity, tierID, periodID)
			if err != nil {
				if isBulkUniqueViolation(err) {
					writeBulkError(w, http.StatusBadRequest,
						fmt.Sprintf("tier from_count=%d уже существует", t.FromQuantity))
					return
				}
				log.Error().Err(err).Msg("network_tariff_bulk: update tier failed")
				respondError(w, shared.ErrInternalServer("ошибка обновления ступени"))
				return
			}
			if ct.RowsAffected() == 0 {
				writeBulkError(w, http.StatusBadRequest, "ступень не принадлежит периоду")
				return
			}
		}
	}

	if len(req.TiersDelete) > 0 {
		tierIDs := make([]uuid.UUID, 0, len(req.TiersDelete))
		for _, s := range req.TiersDelete {
			id, err := uuid.Parse(strings.TrimSpace(s))
			if err != nil {
				writeBulkError(w, http.StatusBadRequest, "tier id некорректен: "+s)
				return
			}
			tierIDs = append(tierIDs, id)
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM reseller_tariff_tiers
			WHERE id = ANY($1::uuid[]) AND tariff_period_id = $2`,
			tierIDs, periodID); err != nil {
			log.Error().Err(err).Msg("network_tariff_bulk: delete tiers failed")
			respondError(w, shared.ErrInternalServer("ошибка удаления ступеней"))
			return
		}
	}

	// Cells: upsert — write price_per_segment on the resolved (plan, period,
	// tier) row. Template scope writes to the URL plan's tier. Override scope
	// writes to the override plan for the given sub_account + dimensions
	// (find-or-create the override plan + aligned period + tier as needed).
	for _, c := range req.CellsUpsert {
		if appErr := h.applyCellUpsert(ctx, tx, plan, periodID, c); appErr != nil {
			respondError(w, appErr)
			return
		}
	}

	for _, c := range req.CellsDelete {
		if appErr := h.applyCellDelete(ctx, tx, plan, periodID, c); appErr != nil {
			respondError(w, appErr)
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: commit failed")
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}

	h.invalidateSummaryCache(resellerID)

	writeJSON(w, http.StatusOK, bulkResponse{OK: true})
}

// ---------- Period create ----------

type createPeriodRequest struct {
	From              string  `json:"from"`
	To                *string `json:"to"`
	CopyFromPeriodID  *string `json:"copy_from_period_id"`
	KeepTiers         bool    `json:"keep_tiers"`
}

type createPeriodResponse struct {
	ID string `json:"id"`
}

// CreatePeriod handles POST /portal/v1/network/tariff-plans/{plan_id}/periods.
func (h *NetworkTariffBulkHandler) CreatePeriod(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.ensureReseller(w, r)
	if !ok {
		return
	}

	planID, err := uuid.Parse(mux.Vars(r)["plan_id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("plan_id некорректен"))
		return
	}

	var req createPeriodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("некорректное тело запроса"))
		return
	}

	from, err := time.Parse("2006-01-02", strings.TrimSpace(req.From))
	if err != nil {
		respondError(w, shared.ErrInvalidInput("from должен быть в формате YYYY-MM-DD"))
		return
	}
	var to *time.Time
	if req.To != nil && strings.TrimSpace(*req.To) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*req.To))
		if err != nil {
			respondError(w, shared.ErrInvalidInput("to должен быть в формате YYYY-MM-DD"))
			return
		}
		if !parsed.After(from) {
			respondError(w, shared.ErrInvalidInput("to должен быть позже from"))
			return
		}
		to = &parsed
	}

	var copyFromID *uuid.UUID
	if req.CopyFromPeriodID != nil && strings.TrimSpace(*req.CopyFromPeriodID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.CopyFromPeriodID))
		if err != nil {
			respondError(w, shared.ErrInvalidInput("copy_from_period_id некорректен"))
			return
		}
		copyFromID = &id
	}

	ctx := r.Context()

	// Verify plan ownership.
	if _, err := h.loadPlanScoped(ctx, planID, resellerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, shared.ErrNotFound("план"))
			return
		}
		log.Error().Err(err).Msg("network_tariff_bulk: load plan failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения плана"))
		return
	}

	// If copy_from_period_id is provided, verify it belongs to the same plan.
	if copyFromID != nil {
		var cfPlan uuid.UUID
		if err := h.pool.QueryRow(ctx,
			`SELECT tariff_plan_id FROM reseller_tariff_periods WHERE id = $1`, *copyFromID,
		).Scan(&cfPlan); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				respondError(w, shared.ErrNotFound("период-источник"))
				return
			}
			log.Error().Err(err).Msg("network_tariff_bulk: load copy-from period failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения периода"))
			return
		}
		if cfPlan != planID {
			respondError(w, shared.ErrInvalidInput("copy_from_period_id не принадлежит этому плану"))
			return
		}
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: begin tx failed")
		respondError(w, shared.ErrInternalServer("ошибка транзакции"))
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var newPeriodID uuid.UUID
	var toArg interface{}
	if to != nil {
		toArg = *to
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
		VALUES ($1, $2, $3)
		RETURNING id`, planID, from, toArg).Scan(&newPeriodID); err != nil {
		// Overlap exclusion constraint → 409.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23P01" || pgErr.ConstraintName == "reseller_periods_no_overlap" {
				respondError(w, shared.ErrConflict("период пересекается с существующим"))
				return
			}
		}
		log.Error().Err(err).Msg("network_tariff_bulk: insert period failed")
		respondError(w, shared.ErrInternalServer("ошибка создания периода"))
		return
	}

	if copyFromID != nil && req.KeepTiers {
		if _, err := tx.Exec(ctx, `
			INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
			SELECT $1, from_count, price_per_segment
			FROM reseller_tariff_tiers
			WHERE tariff_period_id = $2`, newPeriodID, *copyFromID); err != nil {
			log.Error().Err(err).Msg("network_tariff_bulk: copy tiers failed")
			respondError(w, shared.ErrInternalServer("ошибка копирования ступеней"))
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: commit period failed")
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}

	h.invalidateSummaryCache(resellerID)

	writeJSON(w, http.StatusCreated, createPeriodResponse{ID: newPeriodID.String()})
}

// ---------- helpers ----------

type bulkPlan struct {
	ID             uuid.UUID
	ResellerID     uuid.UUID
	TemplateID     *uuid.UUID
	SubAccountID   *uuid.UUID
	CountryID      *uuid.UUID
	OperatorID     *uuid.UUID // nil = wildcard
	SenderCategory string
	TrafficType    string
	Strategy       string
}

// loadPlanScoped loads a plan and verifies caller ownership. For template
// plans, reseller is derived via template.reseller_id. For override plans,
// reseller_tariff_plans.reseller_id is checked directly. Returns
// pgx.ErrNoRows if the plan is missing or scoped to another reseller.
func (h *NetworkTariffBulkHandler) loadPlanScoped(
	ctx context.Context, planID, resellerID uuid.UUID,
) (*bulkPlan, error) {
	var p bulkPlan
	err := h.pool.QueryRow(ctx, `
		SELECT p.id, p.reseller_id, p.template_id, p.sub_account_id,
		       p.country_id, p.operator_id, p.sender_category, p.traffic_type, p.strategy
		FROM reseller_tariff_plans p
		LEFT JOIN reseller_tariff_templates t ON t.id = p.template_id
		WHERE p.id = $1 AND p.active
		  AND (
		        (p.template_id    IS NOT NULL AND t.reseller_id = $2 AND t.active) OR
		        (p.sub_account_id IS NOT NULL AND p.reseller_id = $2)
		      )
	`, planID, resellerID).Scan(
		&p.ID, &p.ResellerID, &p.TemplateID, &p.SubAccountID,
		&p.CountryID, &p.OperatorID, &p.SenderCategory, &p.TrafficType, &p.Strategy,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// validateBulkInput checks prices, scopes, operator_id alignment, and
// sub_account ownership. Returns a (possibly empty) list of per-cell errors.
func (h *NetworkTariffBulkHandler) validateBulkInput(
	ctx context.Context,
	resellerID uuid.UUID,
	plan *bulkPlan,
	req *bulkPatchRequest,
) []bulkError {
	var errs []bulkError

	for _, t := range req.TiersUpsert {
		if t.FromQuantity < 0 {
			errs = append(errs, bulkError{Reason: "from_quantity не может быть отрицательным"})
		}
	}

	// Collect sub_account_ids that need ownership check across cells.
	subsToCheck := map[uuid.UUID]struct{}{}

	validateCell := func(operatorID, tierID, scope string, sub *string, price *float64) {
		opID, err := uuid.Parse(operatorID)
		if err != nil {
			errs = append(errs, bulkError{OperatorID: operatorID, TierID: tierID, Reason: "operator_id некорректен"})
			return
		}
		if _, err := uuid.Parse(tierID); err != nil {
			errs = append(errs, bulkError{OperatorID: operatorID, TierID: tierID, Reason: "tier_id некорректен"})
			return
		}
		if scope != "template" && scope != "override" {
			errs = append(errs, bulkError{OperatorID: operatorID, TierID: tierID, Reason: "scope должен быть template или override"})
			return
		}
		// Operator alignment: if the URL plan has an operator_id set, cells
		// must match it (else frontend batched to wrong plan). If plan is
		// wildcard (operator_id IS NULL), any operator_id is accepted.
		if plan.OperatorID != nil && *plan.OperatorID != opID {
			errs = append(errs, bulkError{
				OperatorID: operatorID, TierID: tierID,
				Reason: "operator_id не соответствует плану",
			})
			return
		}
		if scope == "override" {
			if sub == nil || strings.TrimSpace(*sub) == "" {
				errs = append(errs, bulkError{OperatorID: operatorID, TierID: tierID, Reason: "sub_account_id обязателен для scope=override"})
				return
			}
			subID, err := uuid.Parse(strings.TrimSpace(*sub))
			if err != nil {
				errs = append(errs, bulkError{OperatorID: operatorID, TierID: tierID, Reason: "sub_account_id некорректен"})
				return
			}
			subsToCheck[subID] = struct{}{}
		}
		if price != nil {
			if *price < bulkPriceMin || *price > bulkPriceMax {
				errs = append(errs, bulkError{
					OperatorID: operatorID, TierID: tierID,
					Reason: fmt.Sprintf("price должен быть в диапазоне [%g, %g]", bulkPriceMin, bulkPriceMax),
				})
				return
			}
		}
	}

	for _, c := range req.CellsUpsert {
		p := c.Price
		validateCell(c.OperatorID, c.TierID, c.Scope, c.SubAccountID, &p)
	}
	for _, c := range req.CellsDelete {
		validateCell(c.OperatorID, c.TierID, c.Scope, c.SubAccountID, nil)
	}

	// Ownership check for sub-accounts, batched.
	if len(subsToCheck) > 0 {
		ids := make([]uuid.UUID, 0, len(subsToCheck))
		for id := range subsToCheck {
			ids = append(ids, id)
		}
		valid := map[uuid.UUID]struct{}{}
		rows, err := h.pool.Query(ctx,
			`SELECT id FROM clients WHERE parent_client_id = $1 AND id = ANY($2::uuid[])`,
			resellerID, ids)
		if err != nil {
			log.Error().Err(err).Msg("network_tariff_bulk: sub ownership check failed")
			errs = append(errs, bulkError{Reason: "ошибка проверки суб-аккаунтов"})
			return errs
		}
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				errs = append(errs, bulkError{Reason: "ошибка проверки суб-аккаунтов"})
				return errs
			}
			valid[id] = struct{}{}
		}
		rows.Close()
		// Attach per-cell errors for offending sub_account_ids.
		check := func(scope string, sub *string, opID, tierID string) {
			if scope != "override" || sub == nil {
				return
			}
			id, err := uuid.Parse(strings.TrimSpace(*sub))
			if err != nil {
				return
			}
			if _, ok := valid[id]; !ok {
				errs = append(errs, bulkError{
					OperatorID: opID, TierID: tierID,
					Reason: "sub_account_id не принадлежит этому агрегатору",
				})
			}
		}
		for _, c := range req.CellsUpsert {
			check(c.Scope, c.SubAccountID, c.OperatorID, c.TierID)
		}
		for _, c := range req.CellsDelete {
			check(c.Scope, c.SubAccountID, c.OperatorID, c.TierID)
		}
	}

	return errs
}

// applyCellUpsert writes price_per_segment for one cell. For scope=template,
// the write targets (periodID, tierID) on the URL plan. For scope=override,
// we resolve-or-create an override plan + aligned period + tier for the
// sub_account + dimensions, then write there.
func (h *NetworkTariffBulkHandler) applyCellUpsert(
	ctx context.Context, tx pgx.Tx,
	plan *bulkPlan, periodID uuid.UUID, c bulkCellUpsert,
) *shared.AppError {
	tierID, _ := uuid.Parse(c.TierID) // already validated

	if c.Scope == "template" {
		// Update existing tier on periodID, keyed by tier_id. Validation
		// already enforced operator alignment; however a template-scope cell
		// updating a tier that doesn't belong to periodID would silently
		// no-op — we check rows_affected for a clean error.
		ct, err := tx.Exec(ctx, `
			UPDATE reseller_tariff_tiers
			SET price_per_segment = $1
			WHERE id = $2 AND tariff_period_id = $3`, c.Price, tierID, periodID)
		if err != nil {
			log.Error().Err(err).Msg("network_tariff_bulk: update template cell failed")
			return shared.ErrInternalServer("ошибка записи цены")
		}
		if ct.RowsAffected() == 0 {
			return shared.ErrInvalidInput("tier не найден в периоде")
		}
		return nil
	}

	// scope == "override": need the override plan for this sub_account +
	// the same dims as URL plan, and a period aligned to the URL period's
	// bounds. Find or create both, then upsert the tier.
	subID, _ := uuid.Parse(strings.TrimSpace(*c.SubAccountID))
	overridePlanID, appErr := h.findOrCreateOverridePlan(ctx, tx, plan, subID)
	if appErr != nil {
		return appErr
	}

	// Load URL period's bounds.
	var pFromT time.Time
	var pToPtr *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT start_date, end_date FROM reseller_tariff_periods WHERE id = $1`,
		periodID).Scan(&pFromT, &pToPtr); err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: read period bounds failed")
		return shared.ErrInternalServer("ошибка чтения периода")
	}
	var pFrom interface{} = pFromT
	var pTo interface{}
	if pToPtr != nil {
		pTo = *pToPtr
	}

	// Find or create override period with matching bounds.
	var ovrPeriodID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM reseller_tariff_periods
		WHERE tariff_plan_id = $1
		  AND start_date IS NOT DISTINCT FROM $2
		  AND end_date   IS NOT DISTINCT FROM $3`,
		overridePlanID, pFrom, pTo,
	).Scan(&ovrPeriodID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `
			INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
			VALUES ($1, $2, $3)
			RETURNING id`, overridePlanID, pFrom, pTo).Scan(&ovrPeriodID); err != nil {
			log.Error().Err(err).Msg("network_tariff_bulk: create override period failed")
			return shared.ErrInternalServer("ошибка создания override-периода")
		}
	} else if err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: find override period failed")
		return shared.ErrInternalServer("ошибка поиска override-периода")
	}
	// URL-plan tier → its from_count → same from_count in override period.
	var fromCount int
	if err := tx.QueryRow(ctx, `
		SELECT from_count FROM reseller_tariff_tiers WHERE id = $1 AND tariff_period_id = $2`,
		tierID, periodID).Scan(&fromCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return shared.ErrInvalidInput("tier не найден в периоде")
		}
		log.Error().Err(err).Msg("network_tariff_bulk: read tier from_count failed")
		return shared.ErrInternalServer("ошибка чтения ступени")
	}

	// UPSERT (override period, from_count, price).
	if _, err := tx.Exec(ctx, `
		INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
		VALUES ($1, $2, $3)
		ON CONFLICT (tariff_period_id, from_count)
		DO UPDATE SET price_per_segment = EXCLUDED.price_per_segment`,
		ovrPeriodID, fromCount, c.Price); err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: upsert override tier failed")
		return shared.ErrInternalServer("ошибка записи override-цены")
	}

	return nil
}

// applyCellDelete deletes a cell. For scope=template, it removes the tier row
// on the URL plan's period (cell becomes "unset"). For scope=override, it
// removes the tier on the override plan's aligned period; if the override
// plan has no more tiers after the delete, we leave the plan in place
// (harmless empty shell; re-used on future writes).
func (h *NetworkTariffBulkHandler) applyCellDelete(
	ctx context.Context, tx pgx.Tx,
	plan *bulkPlan, periodID uuid.UUID, c bulkCellDelete,
) *shared.AppError {
	tierID, _ := uuid.Parse(c.TierID)

	if c.Scope == "template" {
		if _, err := tx.Exec(ctx, `
			DELETE FROM reseller_tariff_tiers
			WHERE id = $1 AND tariff_period_id = $2`, tierID, periodID); err != nil {
			log.Error().Err(err).Msg("network_tariff_bulk: delete template cell failed")
			return shared.ErrInternalServer("ошибка удаления цены")
		}
		return nil
	}

	subID, _ := uuid.Parse(strings.TrimSpace(*c.SubAccountID))

	// Find the override plan + aligned period. If none, nothing to delete.
	var ovrPlanID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM reseller_tariff_plans
		WHERE sub_account_id = $1 AND active
		  AND COALESCE(country_id, '00000000-0000-0000-0000-000000000000'::uuid)
		    = COALESCE($2, '00000000-0000-0000-0000-000000000000'::uuid)
		  AND COALESCE(operator_id, '00000000-0000-0000-0000-000000000000'::uuid)
		    = COALESCE($3, '00000000-0000-0000-0000-000000000000'::uuid)
		  AND sender_category = $4 AND traffic_type = $5`,
		subID, plan.CountryID, plan.OperatorID, plan.SenderCategory, plan.TrafficType,
	).Scan(&ovrPlanID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: find override plan failed")
		return shared.ErrInternalServer("ошибка поиска override-плана")
	}

	var pFromT time.Time
	var pToT *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT start_date, end_date FROM reseller_tariff_periods WHERE id = $1`,
		periodID).Scan(&pFromT, &pToT); err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: read period bounds failed")
		return shared.ErrInternalServer("ошибка чтения периода")
	}
	var pTo interface{}
	if pToT != nil {
		pTo = *pToT
	}

	var ovrPeriodID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM reseller_tariff_periods
		WHERE tariff_plan_id = $1
		  AND start_date IS NOT DISTINCT FROM $2
		  AND end_date   IS NOT DISTINCT FROM $3`,
		ovrPlanID, pFromT, pTo).Scan(&ovrPeriodID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: find override period failed")
		return shared.ErrInternalServer("ошибка поиска override-периода")
	}

	// Need from_count from the URL-plan tier to key into override period.
	var fromCount int
	if err := tx.QueryRow(ctx, `
		SELECT from_count FROM reseller_tariff_tiers
		WHERE id = $1 AND tariff_period_id = $2`,
		tierID, periodID).Scan(&fromCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Tier no longer exists on URL plan — nothing to key by.
			return nil
		}
		log.Error().Err(err).Msg("network_tariff_bulk: read tier from_count failed")
		return shared.ErrInternalServer("ошибка чтения ступени")
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM reseller_tariff_tiers
		WHERE tariff_period_id = $1 AND from_count = $2`,
		ovrPeriodID, fromCount); err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: delete override tier failed")
		return shared.ErrInternalServer("ошибка удаления override-цены")
	}
	return nil
}

// findOrCreateOverridePlan returns an override plan id for (sub_account, URL
// plan dims). If no such plan exists, one is created with the same country /
// operator / sender_category / traffic_type / strategy as the URL plan.
func (h *NetworkTariffBulkHandler) findOrCreateOverridePlan(
	ctx context.Context, tx pgx.Tx, plan *bulkPlan, subAccountID uuid.UUID,
) (uuid.UUID, *shared.AppError) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM reseller_tariff_plans
		WHERE sub_account_id = $1 AND active
		  AND COALESCE(country_id, '00000000-0000-0000-0000-000000000000'::uuid)
		    = COALESCE($2, '00000000-0000-0000-0000-000000000000'::uuid)
		  AND COALESCE(operator_id, '00000000-0000-0000-0000-000000000000'::uuid)
		    = COALESCE($3, '00000000-0000-0000-0000-000000000000'::uuid)
		  AND sender_category = $4 AND traffic_type = $5`,
		subAccountID, plan.CountryID, plan.OperatorID, plan.SenderCategory, plan.TrafficType,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Error().Err(err).Msg("network_tariff_bulk: find override plan failed")
		return uuid.Nil, shared.ErrInternalServer("ошибка поиска override-плана")
	}
	// Create.
	if err := tx.QueryRow(ctx, `
		INSERT INTO reseller_tariff_plans
		  (reseller_id, sub_account_id, country_id, operator_id,
		   sender_category, traffic_type, strategy, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true)
		RETURNING id`,
		plan.ResellerID, subAccountID, plan.CountryID, plan.OperatorID,
		plan.SenderCategory, plan.TrafficType, plan.Strategy,
	).Scan(&id); err != nil {
		log.Error().Err(err).Msg("network_tariff_bulk: create override plan failed")
		return uuid.Nil, shared.ErrInternalServer("ошибка создания override-плана")
	}
	return id, nil
}

func (h *NetworkTariffBulkHandler) ensureReseller(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
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

func (h *NetworkTariffBulkHandler) invalidateSummaryCache(resellerID uuid.UUID) {
	if h.redis == nil {
		return
	}
	cctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.redis.Del(cctx, summaryCacheKeyPrefix+resellerID.String()).Err(); err != nil {
		log.Warn().Err(err).Msg("network_tariff_bulk: redis DEL failed")
	}
}

func isBulkUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func writeBulkError(w http.ResponseWriter, status int, reason string) {
	writeBulkErrors(w, status, []bulkError{{Reason: reason}})
}

func writeBulkErrors(w http.ResponseWriter, status int, errs []bulkError) {
	writeJSON(w, status, bulkResponse{OK: false, Errors: errs})
}
