// Package handlers: network_tariff_editor.go implements the inheritance-aware
// read endpoint for the matrix editor on `/network/tariffs/editor/:id`.
//
// Task 5 — GET /portal/v1/network/tariff-editor/{id}?mode=template|override
//          &channel=sms&country=RU&sender_category=paid_registered
//          &traffic_type=any&period_id=<uuid?>
//
// Gate: is_reseller=true on the caller (same convention as the other
// /network/* endpoints).
//
// Modes:
//   - mode=template: {id} is a template id owned by the caller. `cells` come
//     from the template plan only; `price_override` is always null.
//   - mode=override: {id} is a sub-account id whose parent_client_id equals
//     the caller. Template is resolved via sub_account_template_assignments.
//     Periods are taken from the template plan; overrides contribute only
//     cells, not separate periods.
//
// Note on `channel`: accepted but not filtered. The plan dimensions do not
// include a channel column today; `traffic_type='any'` is the match-all value
// for channel-agnostic pricing. Kept in the query string so the frontend
// contract stays stable when multi-channel plans land.
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// NetworkTariffEditorHandler serves GET /portal/v1/network/tariff-editor/{id}.
type NetworkTariffEditorHandler struct {
	pool *pgxpool.Pool
}

// NewNetworkTariffEditorHandler constructs the handler.
func NewNetworkTariffEditorHandler(pool *pgxpool.Pool) *NetworkTariffEditorHandler {
	return &NetworkTariffEditorHandler{pool: pool}
}

// ---------- Response types ----------

type editorScope struct {
	Kind         string  `json:"kind"` // "template" | "override"
	TemplateID   *string `json:"template_id,omitempty"`
	SubAccountID *string `json:"sub_account_id,omitempty"`
	Name         string  `json:"name"`
}

type editorTemplateRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type editorPlan struct {
	ID       string `json:"id"`
	Strategy string `json:"strategy"`
	Currency string `json:"currency"`
}

type editorPeriod struct {
	ID     string  `json:"id"`
	From   string  `json:"from"`
	To     *string `json:"to"`
	Active bool    `json:"active"`
}

type editorOperator struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Icon *string `json:"icon"`
}

type editorTier struct {
	ID           string `json:"id"`
	FromQuantity int    `json:"from_quantity"`
}

type editorCell struct {
	OperatorID    string   `json:"operator_id"`
	TierID        string   `json:"tier_id"`
	PriceTemplate *float64 `json:"price_template"`
	PriceOverride *float64 `json:"price_override"`
	Effective     *float64 `json:"effective"`
	Source        string   `json:"source"` // "template" | "override" | "unset"
}

type editorResponse struct {
	Scope          editorScope        `json:"scope"`
	Template       *editorTemplateRef `json:"template"`
	Plan           *editorPlan        `json:"plan"`
	Periods        []editorPeriod     `json:"periods"`
	ActivePeriodID *string            `json:"active_period_id"`
	Operators      []editorOperator   `json:"operators"`
	Tiers          []tierWithPrice    `json:"tiers"`
	Cells          []editorCell       `json:"cells"`
}

// ---------- Handler ----------

// Get handles GET /portal/v1/network/tariff-editor/{id}.
func (h *NetworkTariffEditorHandler) Get(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.ensureReseller(w, r)
	if !ok {
		return
	}

	idStr := mux.Vars(r)["id"]
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("id некорректен"))
		return
	}

	q := r.URL.Query()
	mode := strings.TrimSpace(q.Get("mode"))
	if mode != "template" && mode != "override" {
		respondError(w, shared.ErrInvalidInput("mode должен быть template или override"))
		return
	}
	countryISO := strings.ToUpper(strings.TrimSpace(q.Get("country")))
	if countryISO == "" {
		respondError(w, shared.ErrInvalidInput("country обязателен"))
		return
	}
	senderCategory := strings.TrimSpace(q.Get("sender_category"))
	if senderCategory == "" {
		senderCategory = "paid_registered"
	}
	trafficType := strings.TrimSpace(q.Get("traffic_type"))
	if trafficType == "" {
		trafficType = "any"
	}
	var requestedPeriodID *uuid.UUID
	if s := strings.TrimSpace(q.Get("period_id")); s != "" {
		parsed, err := uuid.Parse(s)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("period_id некорректен"))
			return
		}
		requestedPeriodID = &parsed
	}

	ctx := r.Context()

	// Resolve country_id by iso_code; missing country = empty response with
	// unset cells (same as "no plan found").
	var countryID uuid.UUID
	err = h.pool.QueryRow(ctx,
		`SELECT id FROM countries WHERE iso_code = $1`, countryISO).Scan(&countryID)
	countryMissing := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !countryMissing {
		log.Error().Err(err).Msg("network_tariff_editor: country lookup failed")
		respondError(w, shared.ErrInternalServer("ошибка поиска страны"))
		return
	}

	if mode == "template" {
		h.serveTemplate(w, r, resellerID, id, countryID, countryMissing, senderCategory, trafficType, requestedPeriodID)
		return
	}
	h.serveOverride(w, r, resellerID, id, countryID, countryMissing, senderCategory, trafficType, requestedPeriodID)
}

// ensureReseller — same pattern as NetworkTariffTemplatesHandler.
func (h *NetworkTariffEditorHandler) ensureReseller(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
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

// serveTemplate builds the response for mode=template.
func (h *NetworkTariffEditorHandler) serveTemplate(
	w http.ResponseWriter,
	r *http.Request,
	resellerID, templateID uuid.UUID,
	countryID uuid.UUID, countryMissing bool,
	senderCategory, trafficType string,
	requestedPeriodID *uuid.UUID,
) {
	ctx := r.Context()

	// 1. Load template (scope to reseller; 404 if missing or inactive).
	var tplName string
	err := h.pool.QueryRow(ctx, `
		SELECT name FROM reseller_tariff_templates
		WHERE id = $1 AND reseller_id = $2 AND active`,
		templateID, resellerID).Scan(&tplName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, shared.ErrNotFound("шаблон"))
			return
		}
		log.Error().Err(err).Msg("network_tariff_editor: template lookup failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения шаблона"))
		return
	}

	tplIDStr := templateID.String()
	resp := editorResponse{
		Scope: editorScope{
			Kind:       "template",
			TemplateID: &tplIDStr,
			Name:       tplName,
		},
		Template: &editorTemplateRef{ID: tplIDStr, Name: tplName},
		// zero-value slices below; JSON encoder turns nil into null — we
		// explicitly pre-init to keep empty arrays in the response.
		Periods:   []editorPeriod{},
		Operators: []editorOperator{},
		Tiers:     []tierWithPrice{},
		Cells:     []editorCell{},
	}

	// 2. Find the template plan for the requested dimensions. If the country
	// row does not exist (by iso_code), no plan can match — bail out with a
	// well-formed empty response.
	if countryMissing {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	tplPlan, err := h.findPlan(ctx, findPlanArgs{
		TemplateID:     &templateID,
		CountryID:      countryID,
		SenderCategory: senderCategory,
		TrafficType:    trafficType,
	})
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: template plan lookup failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения плана"))
		return
	}
	if tplPlan == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp.Plan = &editorPlan{
		ID:       tplPlan.ID.String(),
		Strategy: tplPlan.Strategy,
		Currency: "RUB", // hardcoded per spec §5.3 (no clients.currency column)
	}

	// 3. Load periods for the template plan and pick the active one.
	periods, err := h.loadPeriods(ctx, tplPlan.ID)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: load periods failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения периодов"))
		return
	}
	activePeriod := pickActivePeriod(periods, requestedPeriodID)
	resp.Periods = periodsToJSON(periods, activePeriod)
	if activePeriod != nil {
		ps := activePeriod.ID.String()
		resp.ActivePeriodID = &ps
	}

	// 4. Operators + tiers + cells (only if we picked a period).
	operators, err := h.loadOperators(ctx)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: load operators failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения операторов"))
		return
	}
	resp.Operators = operators

	if activePeriod != nil {
		tiers, err := h.loadTiers(ctx, activePeriod.ID)
		if err != nil {
			log.Error().Err(err).Msg("network_tariff_editor: load tiers failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения ступеней"))
			return
		}
		resp.Tiers = tiers

		// Template-price lookup keyed by (operator_id,tier_id). In template
		// mode, tiers belong to the template period, so operator_id comes
		// from the iteration — the tier table does not hold operator_id.
		// Price per (operator,tier) resolves to the tier's price_per_segment
		// regardless of operator because plan dimensions do NOT include
		// operator_id in the RedSMS model for channel-agnostic templates.
		// TODO(multi-operator-plans): when operator_id starts being used on
		// plans, split by operator here.
		tierPrices := map[string]float64{}
		for _, t := range tiers {
			tierPrices[t.ID] = t.price
		}

		resp.Cells = buildCellsTemplate(operators, tiers, tierPrices)
	}

	writeJSON(w, http.StatusOK, resp)
}

// serveOverride builds the response for mode=override.
func (h *NetworkTariffEditorHandler) serveOverride(
	w http.ResponseWriter,
	r *http.Request,
	resellerID, subAccountID uuid.UUID,
	countryID uuid.UUID, countryMissing bool,
	senderCategory, trafficType string,
	requestedPeriodID *uuid.UUID,
) {
	ctx := r.Context()

	// 1. Load sub-account (verify parent_client_id=reseller). 404 otherwise.
	var subName string
	err := h.pool.QueryRow(ctx, `
		SELECT name FROM clients
		WHERE id = $1 AND parent_client_id = $2`,
		subAccountID, resellerID).Scan(&subName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, shared.ErrNotFound("субаккаунт"))
			return
		}
		log.Error().Err(err).Msg("network_tariff_editor: sub-account lookup failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения субаккаунта"))
		return
	}

	subIDStr := subAccountID.String()
	resp := editorResponse{
		Scope: editorScope{
			Kind:         "override",
			SubAccountID: &subIDStr,
			Name:         subName,
		},
		Periods:   []editorPeriod{},
		Operators: []editorOperator{},
		Tiers:     []tierWithPrice{},
		Cells:     []editorCell{},
	}

	// 2. Load assigned template (if any).
	var tplID uuid.UUID
	var tplName string
	err = h.pool.QueryRow(ctx, `
		SELECT t.id, t.name
		FROM sub_account_template_assignments a
		JOIN reseller_tariff_templates t ON t.id = a.template_id AND t.active
		WHERE a.sub_account_id = $1`, subAccountID).Scan(&tplID, &tplName)
	hasTemplate := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Error().Err(err).Msg("network_tariff_editor: template assignment lookup failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения шаблона"))
		return
	}
	if hasTemplate {
		tpls := tplID.String()
		resp.Template = &editorTemplateRef{ID: tpls, Name: tplName}
	}

	if countryMissing {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// 3. Look up template plan (if binding exists) and override plan.
	var tplPlan *planRow
	if hasTemplate {
		tplPlan, err = h.findPlan(ctx, findPlanArgs{
			TemplateID:     &tplID,
			CountryID:      countryID,
			SenderCategory: senderCategory,
			TrafficType:    trafficType,
		})
		if err != nil {
			log.Error().Err(err).Msg("network_tariff_editor: template plan lookup failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения плана"))
			return
		}
	}

	ovrPlan, err := h.findPlan(ctx, findPlanArgs{
		SubAccountID:   &subAccountID,
		CountryID:      countryID,
		SenderCategory: senderCategory,
		TrafficType:    trafficType,
	})
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: override plan lookup failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения плана переопределений"))
		return
	}

	// 4. Periods are driven by the template plan. If there is no template
	// plan (unbound sub-account with only overrides), fall back to the
	// override plan's periods.
	var periodsSource *planRow
	switch {
	case tplPlan != nil:
		periodsSource = tplPlan
	case ovrPlan != nil:
		periodsSource = ovrPlan
	}

	if periodsSource == nil {
		// Neither template nor override has a plan → empty cells but keep
		// scope + (optional) template ref.
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// `plan` in the response reflects the template plan when present, else
	// override plan. Strategy is per-plan in the current schema.
	resp.Plan = &editorPlan{
		ID:       periodsSource.ID.String(),
		Strategy: periodsSource.Strategy,
		Currency: "RUB",
	}

	periods, err := h.loadPeriods(ctx, periodsSource.ID)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: load periods failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения периодов"))
		return
	}
	activePeriod := pickActivePeriod(periods, requestedPeriodID)
	resp.Periods = periodsToJSON(periods, activePeriod)
	if activePeriod != nil {
		ps := activePeriod.ID.String()
		resp.ActivePeriodID = &ps
	}

	operators, err := h.loadOperators(ctx)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: load operators failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения операторов"))
		return
	}
	resp.Operators = operators

	if activePeriod == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Tiers: `tiers` listed in the response are the template plan's tiers
	// for the active period (if template is bound). If no template binding
	// exists, we fall back to the override plan's tiers so the matrix has
	// rows to render.
	tiers, err := h.loadTiers(ctx, activePeriod.ID)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: load tiers failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения ступеней"))
		return
	}
	resp.Tiers = tiers

	// Template prices (if template plan is bound). Keyed by tier.ID — these
	// ids are the template-plan tier ids when template is bound; otherwise
	// they are the override-plan tier ids and there are no template prices.
	tplTierPricesByID := map[string]float64{}
	if tplPlan != nil {
		for _, t := range tiers {
			tplTierPricesByID[t.ID] = t.price
		}
	}

	// Override prices: keyed by from_count, matched to the override period
	// that aligns with the active period's bounds. When template is unbound,
	// the active period IS an override period — we can load its tiers
	// directly by id, but for consistency with the template-bound path we
	// still use the from_count key so buildCellsOverride doesn't branch.
	var ovrTierPrices map[string]float64
	switch {
	case ovrPlan != nil && tplPlan != nil:
		// Template-bound: match override period by bounds.
		ovrPeriodID, err := h.findOverridePeriod(ctx, ovrPlan.ID, activePeriod.From, activePeriod.To)
		if err != nil {
			log.Error().Err(err).Msg("network_tariff_editor: find override period failed")
			respondError(w, shared.ErrInternalServer("ошибка сопоставления периода переопределений"))
			return
		}
		if ovrPeriodID != nil {
			ovrTierPrices, err = h.loadOverrideTierPricesByFromCount(ctx, *ovrPeriodID)
			if err != nil {
				log.Error().Err(err).Msg("network_tariff_editor: load override tiers failed")
				respondError(w, shared.ErrInternalServer("ошибка чтения переопределений"))
				return
			}
		}
	case ovrPlan != nil && tplPlan == nil:
		// Unbound: active period is the override period; its tiers are the
		// override prices.
		ovrTierPrices = map[string]float64{}
		for _, t := range tiers {
			ovrTierPrices[fromCountKey(t.FromQuantity)] = t.price
		}
	}

	// Build cells. Operator dimension currently shares the price across all
	// operators — see TODO in buildCellsTemplate.
	resp.Cells = buildCellsOverride(operators, tiers, tplTierPricesByID, ovrTierPrices)

	writeJSON(w, http.StatusOK, resp)
}

// ---------- SQL helpers ----------

type planRow struct {
	ID       uuid.UUID
	Strategy string
}

type findPlanArgs struct {
	TemplateID     *uuid.UUID
	SubAccountID   *uuid.UUID
	CountryID      uuid.UUID
	SenderCategory string
	TrafficType    string
}

// findPlan returns the single active plan matching the dimensions, or nil if
// no row matches. `country_id` can be NULL in the DB (wildcard); for this
// endpoint we match only rows with exact country_id=$country per spec — the
// editor operates per-country.
func (h *NetworkTariffEditorHandler) findPlan(ctx context.Context, a findPlanArgs) (*planRow, error) {
	var ownerCol string
	var ownerVal uuid.UUID
	switch {
	case a.TemplateID != nil:
		ownerCol = "template_id"
		ownerVal = *a.TemplateID
	case a.SubAccountID != nil:
		ownerCol = "sub_account_id"
		ownerVal = *a.SubAccountID
	default:
		return nil, errors.New("findPlan: need template_id or sub_account_id")
	}

	var p planRow
	q := `
		SELECT id, strategy FROM reseller_tariff_plans
		WHERE ` + ownerCol + ` = $1
		  AND active
		  AND country_id = $2
		  AND sender_category = $3
		  AND traffic_type = $4
		LIMIT 1`
	err := h.pool.QueryRow(ctx, q, ownerVal, a.CountryID, a.SenderCategory, a.TrafficType).
		Scan(&p.ID, &p.Strategy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

type periodRow struct {
	ID   uuid.UUID
	From sql.NullTime // start_date
	To   sql.NullTime // end_date
}

func (h *NetworkTariffEditorHandler) loadPeriods(ctx context.Context, planID uuid.UUID) ([]periodRow, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT id, start_date, end_date
		FROM reseller_tariff_periods
		WHERE tariff_plan_id = $1
		ORDER BY start_date`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []periodRow
	for rows.Next() {
		var p periodRow
		if err := rows.Scan(&p.ID, &p.From, &p.To); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// pickActivePeriod implements the spec precedence: requested period_id wins
// if it belongs to the list; else the currently-active one by date; else the
// first (earliest) period.
func pickActivePeriod(periods []periodRow, requested *uuid.UUID) *periodRow {
	if len(periods) == 0 {
		return nil
	}
	if requested != nil {
		for i := range periods {
			if periods[i].ID == *requested {
				return &periods[i]
			}
		}
		// Requested id not in the list: fall through to date-based default
		// rather than 404 — keeps the editor robust across stale bookmarks.
	}
	// Currently-active: start_date <= today AND (end_date IS NULL OR end_date > today).
	today := nowUTC()
	for i := range periods {
		p := &periods[i]
		if !p.From.Valid {
			continue
		}
		if !p.From.Time.After(today) && (!p.To.Valid || p.To.Time.After(today)) {
			return p
		}
	}
	return &periods[0]
}

// periodsToJSON marks the chosen period with active=true.
func periodsToJSON(periods []periodRow, active *periodRow) []editorPeriod {
	out := make([]editorPeriod, 0, len(periods))
	for i := range periods {
		p := &periods[i]
		j := editorPeriod{ID: p.ID.String()}
		if p.From.Valid {
			j.From = p.From.Time.Format("2006-01-02")
		}
		if p.To.Valid {
			s := p.To.Time.Format("2006-01-02")
			j.To = &s
		}
		if active != nil && active.ID == p.ID {
			j.Active = true
		}
		out = append(out, j)
	}
	return out
}

// loadOperators returns active operators. No `icon` column exists in the
// `operators` table today (migration 000012) — we emit null for that field.
func (h *NetworkTariffEditorHandler) loadOperators(ctx context.Context) ([]editorOperator, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT id::text, name FROM operators
		WHERE active = true
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]editorOperator, 0)
	for rows.Next() {
		var op editorOperator
		if err := rows.Scan(&op.ID, &op.Name); err != nil {
			return nil, err
		}
		op.Icon = nil
		out = append(out, op)
	}
	return out, rows.Err()
}

type tierWithPrice struct {
	editorTier
	price float64
}

func (h *NetworkTariffEditorHandler) loadTiers(ctx context.Context, periodID uuid.UUID) ([]tierWithPrice, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT id::text, from_count, price_per_segment::float8
		FROM reseller_tariff_tiers
		WHERE tariff_period_id = $1
		ORDER BY from_count`, periodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]tierWithPrice, 0)
	for rows.Next() {
		var t tierWithPrice
		if err := rows.Scan(&t.ID, &t.FromQuantity, &t.price); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// findOverridePeriod returns the override plan's period matching the template
// period by exact (start_date, end_date). Returns nil if no match.
func (h *NetworkTariffEditorHandler) findOverridePeriod(
	ctx context.Context,
	ovrPlanID uuid.UUID,
	from, to sql.NullTime,
) (*uuid.UUID, error) {
	var id uuid.UUID
	// IS NOT DISTINCT FROM treats NULL=NULL as true — necessary because
	// end_date can be NULL (open-ended).
	err := h.pool.QueryRow(ctx, `
		SELECT id FROM reseller_tariff_periods
		WHERE tariff_plan_id = $1
		  AND start_date IS NOT DISTINCT FROM $2
		  AND end_date   IS NOT DISTINCT FROM $3
		LIMIT 1`, ovrPlanID, nullableTime(from), nullableTime(to)).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// loadOverrideTierPricesByFromCount returns a map from_count → price for the
// given override period. We key by from_count (not by tier-id) because
// override tiers are a different row-set from template tiers; they are
// matched back to template tiers by their volume threshold.
func (h *NetworkTariffEditorHandler) loadOverrideTierPricesByFromCount(
	ctx context.Context, periodID uuid.UUID,
) (map[string]float64, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT from_count, price_per_segment::float8
		FROM reseller_tariff_tiers
		WHERE tariff_period_id = $1`, periodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]float64)
	for rows.Next() {
		var fromCount int
		var price float64
		if err := rows.Scan(&fromCount, &price); err != nil {
			return nil, err
		}
		out[fromCountKey(fromCount)] = price
	}
	return out, rows.Err()
}

// buildCellsTemplate paints one cell per (operator × tier). In template
// mode: price_template = tier price (shared across operators — see TODO),
// price_override always null, effective = template price, source = template
// or unset if price is missing.
func buildCellsTemplate(
	operators []editorOperator,
	tiers []tierWithPrice,
	tierPrices map[string]float64,
) []editorCell {
	out := make([]editorCell, 0, len(operators)*len(tiers))
	for _, op := range operators {
		for _, t := range tiers {
			cell := editorCell{
				OperatorID:    op.ID,
				TierID:        t.ID,
				PriceOverride: nil,
			}
			if p, ok := tierPrices[t.ID]; ok {
				tp := p
				cell.PriceTemplate = &tp
				eff := p
				cell.Effective = &eff
				cell.Source = "template"
			} else {
				cell.Source = "unset"
			}
			out = append(out, cell)
		}
	}
	return out
}

// buildCellsOverride paints cells for mode=override. For each (operator,
// tier): price_template from the template tier price (if any), price_override
// from the override tier indexed by from_count (if any). Effective=override
// ?? template; source = "override" if override present, "template" if only
// template, "unset" if neither.
func buildCellsOverride(
	operators []editorOperator,
	tiers []tierWithPrice,
	tplPricesByTierID map[string]float64,
	ovrPricesByFromCount map[string]float64,
) []editorCell {
	out := make([]editorCell, 0, len(operators)*len(tiers))
	for _, op := range operators {
		for _, t := range tiers {
			cell := editorCell{OperatorID: op.ID, TierID: t.ID}
			if p, ok := tplPricesByTierID[t.ID]; ok {
				tp := p
				cell.PriceTemplate = &tp
			}
			if p, ok := ovrPricesByFromCount[fromCountKey(t.FromQuantity)]; ok {
				op := p
				cell.PriceOverride = &op
			}
			switch {
			case cell.PriceOverride != nil:
				eff := *cell.PriceOverride
				cell.Effective = &eff
				cell.Source = "override"
			case cell.PriceTemplate != nil:
				eff := *cell.PriceTemplate
				cell.Effective = &eff
				cell.Source = "template"
			default:
				cell.Source = "unset"
			}
			out = append(out, cell)
		}
	}
	return out
}

// ---------- tiny utilities ----------

func fromCountKey(n int) string {
	return strconv.Itoa(n)
}

// nullableTime returns interface{} so pgx can send NULL or a time.Time.
func nullableTime(t sql.NullTime) interface{} {
	if !t.Valid {
		return nil
	}
	return t.Time
}

// nowUTC returns today's date in UTC, truncated to 00:00:00. Exposed as a var
// so tests can stub a fixed date if needed.
var nowUTC = func() time.Time {
	n := time.Now().UTC()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// writeJSON is a tiny local helper — we want raw json.Encoder behavior rather
// than respondJSON's map-typed signature.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: encode failed")
	}
}
