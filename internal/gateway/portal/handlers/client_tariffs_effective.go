// Package handlers: client_tariffs_effective.go serves the read-only
// effective-price matrix for the authenticated client on /tariffs
// (spec §5.1.8).
//
// GET /portal/v1/client/tariffs/effective
//   query: channel=sms&country=RU&sender_category=paid_registered&traffic_type=any
//
// Scope: caller's own sub_account. No is_reseller gate — any authenticated
// client can read their own effective tariffs. Resellers (no parent_client_id)
// and clients without a template assignment/override plans simply get an
// empty `cells` array; no 403.
//
// Response omits inheritance markers:
//   - cells have only `effective` (no price_template/price_override/source)
//   - single active period (no history list)
//
// Implementation reuses NetworkTariffEditorHandler's private helpers
// (findPlans, pickPrimaryPlan, loadPeriods, pickActivePeriod, loadOperators,
// loadTiers, loadPlanPricesByOperator / Aligned, priceForOperator) — both
// handlers are in the same package.
package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ClientTariffsEffectiveHandler serves GET /portal/v1/client/tariffs/effective.
type ClientTariffsEffectiveHandler struct {
	pool   *pgxpool.Pool
	editor *NetworkTariffEditorHandler // reuse private helpers
}

// NewClientTariffsEffectiveHandler constructs the handler.
func NewClientTariffsEffectiveHandler(pool *pgxpool.Pool) *ClientTariffsEffectiveHandler {
	return &ClientTariffsEffectiveHandler{
		pool:   pool,
		editor: NewNetworkTariffEditorHandler(pool),
	}
}

// ---------- Response shape (spec §5.1.8) ----------

type clientTariffsPlanRef struct {
	ID       string `json:"id"`
	Strategy string `json:"strategy"`
	Currency string `json:"currency"`
}

type clientTariffsPeriodRef struct {
	ID   string  `json:"id"`
	From string  `json:"from"`
	To   *string `json:"to"`
}

type clientTariffsCell struct {
	OperatorID string   `json:"operator_id"`
	TierID     string   `json:"tier_id"`
	Effective  *float64 `json:"effective"`
}

type clientTariffsResponse struct {
	Plan      *clientTariffsPlanRef   `json:"plan"`
	Period    *clientTariffsPeriodRef `json:"period"`
	Operators []editorOperator        `json:"operators"`
	Tiers     []editorTier            `json:"tiers"`
	Cells     []clientTariffsCell     `json:"cells"`
}

// Get handles GET /portal/v1/client/tariffs/effective.
func (h *ClientTariffsEffectiveHandler) Get(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	q := r.URL.Query()
	countryISO := strings.ToUpper(strings.TrimSpace(q.Get("country")))
	if countryISO == "" {
		countryISO = "RU"
	}
	senderCategory := strings.TrimSpace(q.Get("sender_category"))
	if senderCategory == "" {
		senderCategory = "paid_registered"
	}
	trafficType := strings.TrimSpace(q.Get("traffic_type"))
	if trafficType == "" {
		trafficType = "any"
	}
	// `channel` accepted but not filtered — plan dimensions are channel-agnostic
	// today (see editor header note).

	ctx := r.Context()

	// Empty-but-well-formed default response.
	resp := clientTariffsResponse{
		Operators: []editorOperator{},
		Tiers:     []editorTier{},
		Cells:     []clientTariffsCell{},
	}

	// Resolve parent reseller. Resellers themselves (no parent) return empty:
	// they're not a sub-account, and this endpoint is scoped to sub-accounts.
	var parentID *uuid.UUID
	err := h.pool.QueryRow(ctx,
		`SELECT parent_client_id FROM clients WHERE id = $1`, clientID,
	).Scan(&parentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, resp)
			return
		}
		log.Error().Err(err).Msg("client_tariffs_effective: client lookup failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения клиента"))
		return
	}
	if parentID == nil {
		// Reseller or standalone client — no sub-account plans to read.
		writeJSON(w, resp)
		return
	}

	// Resolve country_id.
	var countryID uuid.UUID
	err = h.pool.QueryRow(ctx,
		`SELECT id FROM countries WHERE iso_code = $1`, countryISO).Scan(&countryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, resp)
			return
		}
		log.Error().Err(err).Msg("client_tariffs_effective: country lookup failed")
		respondError(w, shared.ErrInternalServer("ошибка поиска страны"))
		return
	}

	// Find assigned template (if any).
	var tplID uuid.UUID
	err = h.pool.QueryRow(ctx, `
		SELECT t.id
		FROM sub_account_template_assignments a
		JOIN reseller_tariff_templates t ON t.id = a.template_id AND t.active
		WHERE a.sub_account_id = $1`, clientID).Scan(&tplID)
	hasTemplate := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Error().Err(err).Msg("client_tariffs_effective: template assignment lookup failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения шаблона"))
		return
	}

	// Template + override plans for the requested dimensions.
	var tplPlans map[uuid.UUID]planRow
	if hasTemplate {
		tplPlans, err = h.editor.findPlans(ctx, findPlansArgs{
			TemplateID:     &tplID,
			CountryID:      countryID,
			SenderCategory: senderCategory,
			TrafficType:    trafficType,
		})
		if err != nil {
			log.Error().Err(err).Msg("client_tariffs_effective: template plan lookup failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения плана"))
			return
		}
	}

	ovrPlans, err := h.editor.findPlans(ctx, findPlansArgs{
		SubAccountID:   &clientID,
		CountryID:      countryID,
		SenderCategory: senderCategory,
		TrafficType:    trafficType,
	})
	if err != nil {
		log.Error().Err(err).Msg("client_tariffs_effective: override plan lookup failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения переопределений"))
		return
	}

	// Primary plan: template wins (it drives periods/tier columns); fall back
	// to override-only if no template assignment.
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
		writeJSON(w, resp)
		return
	}

	resp.Plan = &clientTariffsPlanRef{
		ID:       primary.ID.String(),
		Strategy: primary.Strategy,
		Currency: "RUB", // hardcoded per spec §5.3 (same as editor)
	}

	// Pick currently-active period only (no period_id query param here).
	primaryPeriods, err := h.editor.loadPeriods(ctx, primary.ID)
	if err != nil {
		log.Error().Err(err).Msg("client_tariffs_effective: load periods failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения периодов"))
		return
	}
	activePeriod := pickActivePeriod(primaryPeriods, nil)
	if activePeriod != nil {
		pr := &clientTariffsPeriodRef{ID: activePeriod.ID.String()}
		if activePeriod.From.Valid {
			pr.From = activePeriod.From.Time.Format("2006-01-02")
		}
		if activePeriod.To.Valid {
			s := activePeriod.To.Time.Format("2006-01-02")
			pr.To = &s
		}
		resp.Period = pr
	}

	operators, err := h.editor.loadOperators(ctx)
	if err != nil {
		log.Error().Err(err).Msg("client_tariffs_effective: load operators failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения операторов"))
		return
	}
	resp.Operators = operators

	if activePeriod == nil {
		writeJSON(w, resp)
		return
	}

	primaryTiers, err := h.editor.loadTiers(ctx, activePeriod.ID)
	if err != nil {
		log.Error().Err(err).Msg("client_tariffs_effective: load tiers failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения ступеней"))
		return
	}
	resp.Tiers = make([]editorTier, 0, len(primaryTiers))
	for _, t := range primaryTiers {
		resp.Tiers = append(resp.Tiers, t.editorTier)
	}

	// Template prices per operator (use activePeriod if primary is template).
	var tplPricesByOperator map[uuid.UUID]map[string]float64
	if len(tplPlans) > 0 {
		var primID uuid.UUID
		if isTemplatePrimary {
			primID = primary.ID
		}
		tplPricesByOperator, err = h.editor.loadPlanPricesByOperator(ctx, tplPlans, primID, activePeriod)
		if err != nil {
			log.Error().Err(err).Msg("client_tariffs_effective: load template per-operator tiers failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения цен шаблона"))
			return
		}
	}

	// Override prices per operator (aligned to activePeriod's bounds when
	// possible — same behaviour as the editor in override mode).
	var ovrPricesByOperator map[uuid.UUID]map[string]float64
	if len(ovrPlans) > 0 {
		var primID uuid.UUID
		if !isTemplatePrimary {
			primID = primary.ID
		}
		ovrPricesByOperator, err = h.editor.loadPlanPricesByOperatorAligned(ctx, ovrPlans, primID, activePeriod)
		if err != nil {
			log.Error().Err(err).Msg("client_tariffs_effective: load override per-operator tiers failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения переопределений"))
			return
		}
	}

	// Build effective-only cells: override wins over template per (op, tier).
	cells := make([]clientTariffsCell, 0, len(operators)*len(primaryTiers))
	for _, op := range operators {
		opUUID, err := uuid.Parse(op.ID)
		if err != nil {
			continue
		}
		for _, t := range primaryTiers {
			c := clientTariffsCell{OperatorID: op.ID, TierID: t.ID}
			if p, ok := priceForOperator(ovrPricesByOperator, opUUID, t.FromQuantity); ok {
				v := p
				c.Effective = &v
			} else if p, ok := priceForOperator(tplPricesByOperator, opUUID, t.FromQuantity); ok {
				v := p
				c.Effective = &v
			}
			cells = append(cells, c)
		}
	}
	resp.Cells = cells

	writeJSON(w, resp)
}
