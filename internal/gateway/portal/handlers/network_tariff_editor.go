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
//     from the template plans only; `price_override` is always null.
//   - mode=override: {id} is a sub-account id whose parent_client_id equals
//     the caller. Template is resolved via sub_account_template_assignments.
//     Periods are taken from the primary template plan; overrides contribute
//     only cells, not separate periods.
//
// Plan dimensions: `(template_id | sub_account_id, country_id, operator_id,
// sender_category, traffic_type)`. The unique indexes in migration 000098 key
// `operator_id` via `COALESCE(operator_id, NIL_UUID)` — so a plan with
// `operator_id IS NULL` is a per-template wildcard covering any operator not
// already served by an operator-specific plan. The editor resolves each
// `(operator, tier)` cell by:
//
//  1. Look up the operator-specific plan for that operator_id.
//  2. If none exists, fall back to the wildcard plan (operator_id IS NULL).
//  3. In override mode, repeat steps 1–2 across both the template-owned plans
//     and the sub-account override plans; the override plan's price wins when
//     present, else the template price, else unset.
//
// Tier identity across operator-plans: tier rows live under the plan → period
// tree, so two operator-plans have different tier uuids even for the same
// from_count. The matrix columns (`tiers`) are populated from ONE "primary"
// plan — wildcard if present, else the first operator-specific plan ordered
// by operator name. Per-operator cells look up the matching tier in their own
// plan by `from_count`, not by tier uuid. If operator-specific plans have
// diverging periods, the editor currently shows the primary plan's periods
// only; multi-period-per-operator matrices are out of scope for this task.
//
// Tier-set divergence across operator-plans: if an operator-specific plan has
// different `from_count` thresholds than the primary/wildcard plan, cells at
// primary's `from_count` values that aren't present in the operator-plan will
// render as `source="unset"`. This is a known limitation; wildcard fallback is
// suppressed once an operator-specific plan exists at any tier. If this
// becomes user-visible pain, relax the suppression in `priceForOperator` to
// allow wildcard fallback per-tier.
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
	"sort"
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
	Kind           string `json:"kind"` // "template" | "override"
	TemplateID     string `json:"template_id,omitempty"`
	TemplateName   string `json:"template_name,omitempty"`
	SubAccountID   string `json:"sub_account_id,omitempty"`
	SubAccountName string `json:"sub_account_name,omitempty"`
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
			Kind:         "template",
			TemplateID:   tplIDStr,
			TemplateName: tplName,
		},
		Template: &editorTemplateRef{ID: tplIDStr, Name: tplName},
		// zero-value slices below; JSON encoder turns nil into null — we
		// explicitly pre-init to keep empty arrays in the response.
		Periods:   []editorPeriod{},
		Operators: []editorOperator{},
		Tiers:     []tierWithPrice{},
		Cells:     []editorCell{},
	}

	// 2. Find all template plans for the requested dimensions. Map keys:
	//    - operator-specific plans: operator UUID
	//    - wildcard plan (operator_id IS NULL): uuid.Nil
	// If the country row does not exist (by iso_code), no plan can match —
	// bail out with a well-formed empty response.
	if countryMissing {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	tplPlans, err := h.findPlans(ctx, findPlansArgs{
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
	if len(tplPlans) == 0 {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	primary := pickPrimaryPlan(tplPlans)
	resp.Plan = &editorPlan{
		ID:       primary.ID.String(),
		Strategy: primary.Strategy,
		Currency: "RUB", // hardcoded per spec §5.3 (no clients.currency column)
	}

	// 3. Operators (full list; even those without a plan appear in the matrix).
	operators, err := h.loadOperators(ctx)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: load operators failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения операторов"))
		return
	}
	resp.Operators = operators

	// 4. Periods are driven by the primary plan. requestedPeriodID selects a
	// period inside the primary plan; non-primary plans use their own
	// currently-active period (see comment on the type below).
	primaryPeriods, err := h.loadPeriods(ctx, primary.ID)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: load periods failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения периодов"))
		return
	}
	activePeriod := pickActivePeriod(primaryPeriods, requestedPeriodID)
	resp.Periods = periodsToJSON(primaryPeriods, activePeriod)
	if activePeriod != nil {
		ps := activePeriod.ID.String()
		resp.ActivePeriodID = &ps
	}

	if activePeriod == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// 5. Tier columns: from the primary plan's active period.
	primaryTiers, err := h.loadTiers(ctx, activePeriod.ID)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: load tiers failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения ступеней"))
		return
	}
	resp.Tiers = primaryTiers

	// 6. Per-plan price maps keyed by from_count. The primary plan uses the
	// user-selected period (activePeriod); other plans pick their own
	// currently-active period (simplification — see file header).
	pricesByOperator, err := h.loadPlanPricesByOperator(ctx, tplPlans, primary.ID, activePeriod)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: load per-operator tiers failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения цен по операторам"))
		return
	}

	resp.Cells = buildCellsTemplate(operators, primaryTiers, pricesByOperator)

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
			Kind:           "override",
			SubAccountID:   subIDStr,
			SubAccountName: subName,
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

	// 3. Look up template plans (if binding exists) and override plans. Each
	// is returned as a map operator_id → plan, where uuid.Nil keys the
	// wildcard plan (operator_id IS NULL).
	var tplPlans map[uuid.UUID]planRow
	if hasTemplate {
		tplPlans, err = h.findPlans(ctx, findPlansArgs{
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

	ovrPlans, err := h.findPlans(ctx, findPlansArgs{
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

	// 4. Primary plan drives periods + tier columns. Template-bound sub shows
	// template primary; unbound sub (overrides only) shows override primary.
	var primary *planRow
	var isTemplatePrimary bool
	if len(tplPlans) > 0 {
		p := pickPrimaryPlan(tplPlans)
		primary = &p
		isTemplatePrimary = true
	} else if len(ovrPlans) > 0 {
		p := pickPrimaryPlan(ovrPlans)
		primary = &p
	}

	if primary == nil {
		// Neither template nor override has a plan → empty cells but keep
		// scope + (optional) template ref.
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp.Plan = &editorPlan{
		ID:       primary.ID.String(),
		Strategy: primary.Strategy,
		Currency: "RUB",
	}

	primaryPeriods, err := h.loadPeriods(ctx, primary.ID)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: load periods failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения периодов"))
		return
	}
	activePeriod := pickActivePeriod(primaryPeriods, requestedPeriodID)
	resp.Periods = periodsToJSON(primaryPeriods, activePeriod)
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

	primaryTiers, err := h.loadTiers(ctx, activePeriod.ID)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_editor: load tiers failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения ступеней"))
		return
	}
	resp.Tiers = primaryTiers

	// Template prices per operator. The primary plan uses activePeriod; other
	// plans use their currently-active period by date.
	var tplPricesByOperator map[uuid.UUID]map[string]float64
	if len(tplPlans) > 0 {
		var primID uuid.UUID
		if isTemplatePrimary {
			primID = primary.ID
		}
		tplPricesByOperator, err = h.loadPlanPricesByOperator(ctx, tplPlans, primID, activePeriod)
		if err != nil {
			log.Error().Err(err).Msg("network_tariff_editor: load template per-operator tiers failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения цен шаблона"))
			return
		}
	}

	// Override prices per operator. If the primary is the override plan, the
	// override plan already uses activePeriod; otherwise each override plan
	// picks its currently-active period (the one whose bounds cover today —
	// or, preferably, the one aligned to activePeriod).
	var ovrPricesByOperator map[uuid.UUID]map[string]float64
	if len(ovrPlans) > 0 {
		var primID uuid.UUID
		if !isTemplatePrimary {
			primID = primary.ID
		}
		// Prefer override periods that match activePeriod's bounds; fall back
		// to currently-active by date (handled inside).
		ovrPricesByOperator, err = h.loadPlanPricesByOperatorAligned(ctx, ovrPlans, primID, activePeriod)
		if err != nil {
			log.Error().Err(err).Msg("network_tariff_editor: load override per-operator tiers failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения переопределений"))
			return
		}
	}

	resp.Cells = buildCellsOverride(operators, primaryTiers, tplPricesByOperator, ovrPricesByOperator)

	writeJSON(w, http.StatusOK, resp)
}

// ---------- SQL helpers ----------

type planRow struct {
	ID         uuid.UUID
	Strategy   string
	OperatorID uuid.UUID // uuid.Nil = wildcard (operator_id IS NULL)
}

type findPlansArgs struct {
	TemplateID     *uuid.UUID
	SubAccountID   *uuid.UUID
	CountryID      uuid.UUID
	SenderCategory string
	TrafficType    string
}

// findPlans returns all active plans matching the non-operator dimensions,
// keyed by operator_id (uuid.Nil for wildcard rows where operator_id IS NULL).
// Multiple plans can match because `operator_id` is a plan dimension. Returns
// an empty (non-nil) map when no rows match.
//
// country_id can be NULL in the DB (wildcard country); the editor operates
// per-country and matches rows with exact country_id=$country per spec.
func (h *NetworkTariffEditorHandler) findPlans(ctx context.Context, a findPlansArgs) (map[uuid.UUID]planRow, error) {
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
		return nil, errors.New("findPlans: need template_id or sub_account_id")
	}

	q := `
		SELECT id, strategy, operator_id
		FROM reseller_tariff_plans
		WHERE ` + ownerCol + ` = $1
		  AND active
		  AND country_id = $2
		  AND sender_category = $3
		  AND traffic_type = $4`
	rows, err := h.pool.Query(ctx, q, ownerVal, a.CountryID, a.SenderCategory, a.TrafficType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID]planRow)
	for rows.Next() {
		var p planRow
		var opID *uuid.UUID
		if err := rows.Scan(&p.ID, &p.Strategy, &opID); err != nil {
			return nil, err
		}
		if opID != nil {
			p.OperatorID = *opID
		}
		// If duplicates exist (should be prevented by the unique index but
		// defense in depth), the last row wins — deterministic enough for a
		// read endpoint.
		out[p.OperatorID] = p
	}
	return out, rows.Err()
}

// pickPrimaryPlan selects the plan whose periods + tiers drive the matrix.
// Precedence: wildcard plan (operator_id=NIL) wins. If none exists, the
// operator-specific plan with the lowest operator_id (deterministic fallback;
// would ideally be by operator name but that requires a join we skip here).
func pickPrimaryPlan(plans map[uuid.UUID]planRow) planRow {
	if p, ok := plans[uuid.Nil]; ok {
		return p
	}
	// Deterministic choice: sort operator ids and pick the first.
	ids := make([]uuid.UUID, 0, len(plans))
	for id := range plans {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i].String() < ids[j].String()
	})
	return plans[ids[0]]
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

// pickActivePeriodByDate returns the period whose [start_date, end_date)
// contains today (end_date NULL = open-ended). Returns nil if none.
func (h *NetworkTariffEditorHandler) pickActivePeriodByDate(ctx context.Context, planID uuid.UUID) (*periodRow, error) {
	periods, err := h.loadPeriods(ctx, planID)
	if err != nil {
		return nil, err
	}
	if len(periods) == 0 {
		return nil, nil
	}
	today := nowUTC()
	for i := range periods {
		p := &periods[i]
		if !p.From.Valid {
			continue
		}
		if !p.From.Time.After(today) && (!p.To.Valid || p.To.Time.After(today)) {
			return p, nil
		}
	}
	return &periods[0], nil
}

// findPeriodByBounds returns the period (within plan planID) whose start_date
// and end_date match the given bounds exactly. Returns nil if no match.
func (h *NetworkTariffEditorHandler) findPeriodByBounds(
	ctx context.Context,
	planID uuid.UUID,
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
		LIMIT 1`, planID, nullableTime(from), nullableTime(to)).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// loadTierPricesByFromCount returns map from_count_string → price for the
// given period. Useful for looking up prices across plans where the tier
// uuids differ but the volume thresholds align.
func (h *NetworkTariffEditorHandler) loadTierPricesByFromCount(
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

// loadPlanPricesByOperator returns, for each plan in `plans`, a map
// from_count_string → price. The primary plan (id == primaryPlanID) uses the
// already-chosen activePeriod; other plans use their currently-active period
// by date. `primaryPlanID == uuid.Nil` means no plan in `plans` is the
// primary (i.e., we're loading template plans but the primary is the override
// plan, or vice versa) — in that case every plan uses its own currently-
// active period.
//
// Returns a map keyed by operator_id (uuid.Nil for the wildcard plan).
func (h *NetworkTariffEditorHandler) loadPlanPricesByOperator(
	ctx context.Context,
	plans map[uuid.UUID]planRow,
	primaryPlanID uuid.UUID,
	activePeriod *periodRow,
) (map[uuid.UUID]map[string]float64, error) {
	out := make(map[uuid.UUID]map[string]float64, len(plans))
	for opID, plan := range plans {
		var periodID uuid.UUID
		if plan.ID == primaryPlanID && activePeriod != nil {
			periodID = activePeriod.ID
		} else {
			pr, err := h.pickActivePeriodByDate(ctx, plan.ID)
			if err != nil {
				return nil, err
			}
			if pr == nil {
				continue
			}
			periodID = pr.ID
		}
		prices, err := h.loadTierPricesByFromCount(ctx, periodID)
		if err != nil {
			return nil, err
		}
		out[opID] = prices
	}
	return out, nil
}

// loadPlanPricesByOperatorAligned is like loadPlanPricesByOperator but for
// each non-primary plan it first tries to find a period aligned to
// activePeriod's bounds (exact match on start_date/end_date) — the behaviour
// the old single-plan override code relied on to line tier rows up with the
// template period. Falls back to the currently-active-by-date period when no
// aligned period exists.
func (h *NetworkTariffEditorHandler) loadPlanPricesByOperatorAligned(
	ctx context.Context,
	plans map[uuid.UUID]planRow,
	primaryPlanID uuid.UUID,
	activePeriod *periodRow,
) (map[uuid.UUID]map[string]float64, error) {
	out := make(map[uuid.UUID]map[string]float64, len(plans))
	for opID, plan := range plans {
		var periodID uuid.UUID
		switch {
		case plan.ID == primaryPlanID && activePeriod != nil:
			periodID = activePeriod.ID
		case activePeriod != nil:
			// Try to align by bounds first.
			aligned, err := h.findPeriodByBounds(ctx, plan.ID, activePeriod.From, activePeriod.To)
			if err != nil {
				return nil, err
			}
			if aligned != nil {
				periodID = *aligned
			} else {
				pr, err := h.pickActivePeriodByDate(ctx, plan.ID)
				if err != nil {
					return nil, err
				}
				if pr == nil {
					continue
				}
				periodID = pr.ID
			}
		default:
			pr, err := h.pickActivePeriodByDate(ctx, plan.ID)
			if err != nil {
				return nil, err
			}
			if pr == nil {
				continue
			}
			periodID = pr.ID
		}
		prices, err := h.loadTierPricesByFromCount(ctx, periodID)
		if err != nil {
			return nil, err
		}
		out[opID] = prices
	}
	return out, nil
}

// priceForOperator resolves the template or override price for (operator,
// from_count) with operator-specific-plan-wins-over-wildcard semantics.
// Returns (price, true) if a match exists, (0, false) otherwise.
func priceForOperator(
	pricesByOperator map[uuid.UUID]map[string]float64,
	operatorID uuid.UUID,
	fromCount int,
) (float64, bool) {
	if pricesByOperator == nil {
		return 0, false
	}
	key := fromCountKey(fromCount)
	if m, ok := pricesByOperator[operatorID]; ok {
		if p, ok2 := m[key]; ok2 {
			return p, true
		}
	}
	// Wildcard fallback.
	if m, ok := pricesByOperator[uuid.Nil]; ok {
		if p, ok2 := m[key]; ok2 {
			return p, true
		}
	}
	return 0, false
}

// buildCellsTemplate paints one cell per (operator × tier) in template mode.
// Price resolution per operator: operator-specific plan > wildcard plan > unset.
// Tiers are keyed by from_count across plans because tier uuids differ.
func buildCellsTemplate(
	operators []editorOperator,
	tiers []tierWithPrice,
	tplPricesByOperator map[uuid.UUID]map[string]float64,
) []editorCell {
	out := make([]editorCell, 0, len(operators)*len(tiers))
	for _, op := range operators {
		opUUID, err := uuid.Parse(op.ID)
		if err != nil {
			continue
		}
		for _, t := range tiers {
			cell := editorCell{
				OperatorID:    op.ID,
				TierID:        t.ID,
				PriceOverride: nil,
			}
			if p, ok := priceForOperator(tplPricesByOperator, opUUID, t.FromQuantity); ok {
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

// buildCellsOverride paints cells for mode=override with per-operator plan
// semantics. For each (operator, tier): price_template from the operator's
// template plan (falling back to the wildcard template plan), price_override
// from the operator's override plan (falling back to the wildcard override).
// effective = override ?? template; source = "override" | "template" | "unset".
func buildCellsOverride(
	operators []editorOperator,
	tiers []tierWithPrice,
	tplPricesByOperator map[uuid.UUID]map[string]float64,
	ovrPricesByOperator map[uuid.UUID]map[string]float64,
) []editorCell {
	out := make([]editorCell, 0, len(operators)*len(tiers))
	for _, op := range operators {
		opUUID, err := uuid.Parse(op.ID)
		if err != nil {
			continue
		}
		for _, t := range tiers {
			cell := editorCell{OperatorID: op.ID, TierID: t.ID}
			if p, ok := priceForOperator(tplPricesByOperator, opUUID, t.FromQuantity); ok {
				tp := p
				cell.PriceTemplate = &tp
			}
			if p, ok := priceForOperator(ovrPricesByOperator, opUUID, t.FromQuantity); ok {
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
