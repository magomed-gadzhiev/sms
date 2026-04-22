//go:build integration

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
)

// ---------- test scaffolding (kept local; Task 7 will extract to a shared
// testhelpers file). ----------

func getEditorTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func seedEditorReseller(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var planSubID uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planSubID)
	require.NoError(t, err, "need at least one subscription plan seeded")

	resellerID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, true, $5)`,
		resellerID,
		fmt.Sprintf("reseller-ed-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-reseller-ed-%s", resellerID),
		fmt.Sprintf("reseller-ed-%s@t.local", uuid.NewString()[:8]),
		planSubID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, resellerID)
	})
	return resellerID
}

func seedEditorSubAccount(t *testing.T, pool *pgxpool.Pool, resellerID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var planSubID uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planSubID)
	require.NoError(t, err)

	subID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, parent_client_id, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5, $6)`,
		subID,
		fmt.Sprintf("sub-ed-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-sub-ed-%s", subID),
		fmt.Sprintf("sub-ed-%s@t.local", uuid.NewString()[:8]),
		resellerID, planSubID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, subID)
	})
	return subID
}

// seedEditorCountryRU returns the RU country id. Creates it if missing.
// Rarely actually creates, since RU is usually seeded — but tests run against
// arbitrary dev databases.
func seedEditorCountryRU(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM countries WHERE iso_code = 'RU'`).Scan(&id)
	if err == nil {
		return id
	}
	id = uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO countries (id, name, iso_code, phone_code, currency)
		VALUES ($1, 'Russia', 'RU', '+7', 'RUB')`, id)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM countries WHERE id = $1`, id)
	})
	return id
}

// seedEditorOperators returns two operator ids (MTS, Beeline) with unique
// codes to avoid colliding with pre-seeded operators.
func seedEditorOperators(t *testing.T, pool *pgxpool.Pool, countryID uuid.UUID) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	mts := uuid.New()
	bln := uuid.New()
	suffix := uuid.NewString()[:8]
	_, err := pool.Exec(ctx, `
		INSERT INTO operators (id, country_id, name, code, active)
		VALUES ($1, $2, $3, $4, true), ($5, $2, $6, $7, true)`,
		mts, countryID, fmt.Sprintf("MTS-ed-%s", suffix), fmt.Sprintf("mts-ed-%s", suffix),
		bln, fmt.Sprintf("Beeline-ed-%s", suffix), fmt.Sprintf("bln-ed-%s", suffix))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id IN ($1,$2)`, mts, bln)
	})
	return mts, bln
}

type editorTemplateSeed struct {
	TemplateID uuid.UUID
	PlanID     uuid.UUID
	PeriodID   uuid.UUID
	Tier0ID    uuid.UUID
	Tier1KID   uuid.UUID
}

// seedEditorTemplatePlan creates a template + single plan for the given
// country/sender_category/traffic + a single currently-active period + two
// tiers (from=0 @ price0, from=1000 @ price1k).
func seedEditorTemplatePlan(
	t *testing.T, pool *pgxpool.Pool,
	resellerID, countryID uuid.UUID,
	name string,
	price0, price1k float64,
) editorTemplateSeed {
	t.Helper()
	ctx := context.Background()

	tplID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO reseller_tariff_templates (id, reseller_id, name, active)
		VALUES ($1, $2, $3, true)`, tplID, resellerID, name)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_templates WHERE id = $1`, tplID)
	})

	var planID uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_plans
		  (reseller_id, template_id, country_id, sender_category, traffic_type, strategy, active)
		VALUES ($1, $2, $3, 'paid_registered', 'any', 'threshold', true)
		RETURNING id`, resellerID, tplID, countryID).Scan(&planID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_plans WHERE id = $1`, planID)
	})

	var periodID uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
		VALUES ($1, CURRENT_DATE - INTERVAL '1 day', NULL)
		RETURNING id`, planID).Scan(&periodID)
	require.NoError(t, err)

	var t0, t1k uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
		VALUES ($1, 0, $2) RETURNING id`, periodID, price0).Scan(&t0)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
		VALUES ($1, 1000, $2) RETURNING id`, periodID, price1k).Scan(&t1k)
	require.NoError(t, err)

	return editorTemplateSeed{
		TemplateID: tplID, PlanID: planID, PeriodID: periodID,
		Tier0ID: t0, Tier1KID: t1k,
	}
}

// seedEditorOverridePlan creates a sub-account override plan with matching
// dimensions and an override period aligned to the template period bounds.
// Returns the overrides period id; caller inserts tiers as needed.
func seedEditorOverridePlan(
	t *testing.T, pool *pgxpool.Pool,
	resellerID, subAccountID, countryID uuid.UUID,
) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	var planID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_plans
		  (reseller_id, sub_account_id, country_id, sender_category, traffic_type, strategy, active)
		VALUES ($1, $2, $3, 'paid_registered', 'any', 'threshold', true)
		RETURNING id`, resellerID, subAccountID, countryID).Scan(&planID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_plans WHERE id = $1`, planID)
	})

	var periodID uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
		VALUES ($1, CURRENT_DATE - INTERVAL '1 day', NULL)
		RETURNING id`, planID).Scan(&periodID)
	require.NoError(t, err)

	return planID, periodID
}

// doEditorGet executes the handler with a crafted URL, setting client id and
// mux vars.
func doEditorGet(
	t *testing.T, h *NetworkTariffEditorHandler, clientID uuid.UUID, pathID uuid.UUID, query string,
) *httptest.ResponseRecorder {
	t.Helper()
	url := fmt.Sprintf("/portal/v1/network/tariff-editor/%s?%s", pathID, query)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, clientID))
	req = mux.SetURLVars(req, map[string]string{"id": pathID.String()})
	w := httptest.NewRecorder()
	h.Get(w, req)
	return w
}

// editorBody mirrors the response shape for easier assertion. Uses
// map[string]json.RawMessage for fields we just want to inspect manually.
type editorBody struct {
	Scope struct {
		Kind         string  `json:"kind"`
		TemplateID   *string `json:"template_id"`
		SubAccountID *string `json:"sub_account_id"`
		Name         string  `json:"name"`
	} `json:"scope"`
	Template *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"template"`
	Plan *struct {
		ID       string `json:"id"`
		Strategy string `json:"strategy"`
		Currency string `json:"currency"`
	} `json:"plan"`
	Periods []struct {
		ID     string  `json:"id"`
		From   string  `json:"from"`
		To     *string `json:"to"`
		Active bool    `json:"active"`
	} `json:"periods"`
	ActivePeriodID *string `json:"active_period_id"`
	Operators      []struct {
		ID   string  `json:"id"`
		Name string  `json:"name"`
		Icon *string `json:"icon"`
	} `json:"operators"`
	Tiers []struct {
		ID           string `json:"id"`
		FromQuantity int    `json:"from_quantity"`
	} `json:"tiers"`
	Cells []struct {
		OperatorID    string   `json:"operator_id"`
		TierID        string   `json:"tier_id"`
		PriceTemplate *float64 `json:"price_template"`
		PriceOverride *float64 `json:"price_override"`
		Effective     *float64 `json:"effective"`
		Source        string   `json:"source"`
	} `json:"cells"`
}

func parseEditorBody(t *testing.T, w *httptest.ResponseRecorder) editorBody {
	t.Helper()
	var b editorBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b),
		"unmarshal editor body failed; raw=%s", w.Body.String())
	return b
}

// ---------- Tests ----------

func TestEditor_TemplateMode_ReturnsPlain(t *testing.T) {
	pool := getEditorTestPool(t)
	reseller := seedEditorReseller(t, pool)
	ruID := seedEditorCountryRU(t, pool)
	mts, bln := seedEditorOperators(t, pool, ruID)
	seed := seedEditorTemplatePlan(t, pool, reseller, ruID,
		fmt.Sprintf("tpl-plain-%s", uuid.NewString()[:6]), 3.20, 2.80)

	h := NewNetworkTariffEditorHandler(pool)
	w := doEditorGet(t, h, reseller, seed.TemplateID,
		"mode=template&channel=sms&country=RU&sender_category=paid_registered&traffic_type=any")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	b := parseEditorBody(t, w)
	assert.Equal(t, "template", b.Scope.Kind)
	require.NotNil(t, b.Scope.TemplateID)
	assert.Equal(t, seed.TemplateID.String(), *b.Scope.TemplateID)
	require.NotNil(t, b.Plan)
	assert.Equal(t, seed.PlanID.String(), b.Plan.ID)
	assert.Equal(t, "threshold", b.Plan.Strategy)
	assert.Equal(t, "RUB", b.Plan.Currency)
	require.NotNil(t, b.ActivePeriodID)
	assert.Equal(t, seed.PeriodID.String(), *b.ActivePeriodID)
	require.Len(t, b.Tiers, 2)

	// Expect 2 operators × 2 tiers = 4 cells; all source=="template",
	// price_override always nil, price_template non-nil.
	want := map[string]bool{mts.String(): true, bln.String(): true}
	seenOps := map[string]int{}
	for _, c := range b.Cells {
		if !want[c.OperatorID] {
			continue
		}
		seenOps[c.OperatorID]++
		assert.Equal(t, "template", c.Source, "cell op=%s tier=%s", c.OperatorID, c.TierID)
		assert.Nil(t, c.PriceOverride)
		require.NotNil(t, c.PriceTemplate)
		require.NotNil(t, c.Effective)
		assert.Equal(t, *c.PriceTemplate, *c.Effective)
	}
	assert.Equal(t, 2, seenOps[mts.String()], "MTS should appear in 2 tier cells")
	assert.Equal(t, 2, seenOps[bln.String()], "Beeline should appear in 2 tier cells")
}

func TestEditor_OverrideMode_ReturnsInheritanceMarkers(t *testing.T) {
	pool := getEditorTestPool(t)
	ctx := context.Background()
	reseller := seedEditorReseller(t, pool)
	sub := seedEditorSubAccount(t, pool, reseller)
	ruID := seedEditorCountryRU(t, pool)
	mts, bln := seedEditorOperators(t, pool, ruID)

	seed := seedEditorTemplatePlan(t, pool, reseller, ruID,
		fmt.Sprintf("tpl-ov-%s", uuid.NewString()[:6]), 3.20, 2.80)

	// Bind template to sub.
	_, err := pool.Exec(ctx, `
		INSERT INTO sub_account_template_assignments (sub_account_id, template_id)
		VALUES ($1, $2)`, sub, seed.TemplateID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM sub_account_template_assignments WHERE sub_account_id = $1`, sub)
	})

	// Override plan + period aligned to template period bounds.
	_, ovrPeriod := seedEditorOverridePlan(t, pool, reseller, sub, ruID)

	// Override tier at from_count=0 with price 3.00 — only tier-0 is
	// overridden; tier-1k is not.
	_, err = pool.Exec(ctx, `
		INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
		VALUES ($1, 0, 3.00)`, ovrPeriod)
	require.NoError(t, err)

	h := NewNetworkTariffEditorHandler(pool)
	w := doEditorGet(t, h, reseller, sub,
		"mode=override&channel=sms&country=RU&sender_category=paid_registered&traffic_type=any")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	b := parseEditorBody(t, w)
	assert.Equal(t, "override", b.Scope.Kind)
	require.NotNil(t, b.Scope.SubAccountID)
	assert.Equal(t, sub.String(), *b.Scope.SubAccountID)
	require.NotNil(t, b.Template)
	assert.Equal(t, seed.TemplateID.String(), b.Template.ID)

	// Find cells for tier0 (from_quantity=0) and tier1k (from_quantity=1000).
	var tier0ID, tier1kID string
	for _, t := range b.Tiers {
		switch t.FromQuantity {
		case 0:
			tier0ID = t.ID
		case 1000:
			tier1kID = t.ID
		}
	}
	require.NotEmpty(t, tier0ID)
	require.NotEmpty(t, tier1kID)

	byKey := map[string]struct {
		source                      string
		priceTpl, priceOvr, effect  *float64
	}{}
	for _, c := range b.Cells {
		if c.OperatorID != mts.String() && c.OperatorID != bln.String() {
			continue
		}
		byKey[c.OperatorID+"|"+c.TierID] = struct {
			source                      string
			priceTpl, priceOvr, effect  *float64
		}{c.Source, c.PriceTemplate, c.PriceOverride, c.Effective}
	}

	// MTS tier0: source=override (we inserted an override tier at from_count=0).
	mtsT0 := byKey[mts.String()+"|"+tier0ID]
	assert.Equal(t, "override", mtsT0.source, "MTS tier0 must be override")
	require.NotNil(t, mtsT0.priceTpl, "template price present")
	require.NotNil(t, mtsT0.priceOvr, "override price present")
	require.NotNil(t, mtsT0.effect)
	assert.InDelta(t, 3.20, *mtsT0.priceTpl, 0.001)
	assert.InDelta(t, 3.00, *mtsT0.priceOvr, 0.001)
	assert.InDelta(t, 3.00, *mtsT0.effect, 0.001, "effective = override")

	// MTS tier1k: source=template (no override at from_count=1000).
	mtsT1k := byKey[mts.String()+"|"+tier1kID]
	assert.Equal(t, "template", mtsT1k.source)
	assert.Nil(t, mtsT1k.priceOvr)
	require.NotNil(t, mtsT1k.priceTpl)
	assert.InDelta(t, 2.80, *mtsT1k.priceTpl, 0.001)

	// Beeline tier0: in current schema overrides shared across operators
	// (same as template semantics). Accept either "override" or "template"
	// but assert the prices line up with the rule in buildCellsOverride.
	blnT0 := byKey[bln.String()+"|"+tier0ID]
	assert.Equal(t, "override", blnT0.source,
		"override tiers currently shared across operators — update when per-operator plans land")
}

func TestEditor_OverrideMode_NoTemplateBinding(t *testing.T) {
	pool := getEditorTestPool(t)
	ctx := context.Background()
	reseller := seedEditorReseller(t, pool)
	sub := seedEditorSubAccount(t, pool, reseller)
	ruID := seedEditorCountryRU(t, pool)
	mts, _ := seedEditorOperators(t, pool, ruID)

	// No template binding — just an override plan with a tier.
	_, ovrPeriod := seedEditorOverridePlan(t, pool, reseller, sub, ruID)
	_, err := pool.Exec(ctx, `
		INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
		VALUES ($1, 0, 4.00)`, ovrPeriod)
	require.NoError(t, err)

	h := NewNetworkTariffEditorHandler(pool)
	w := doEditorGet(t, h, reseller, sub,
		"mode=override&channel=sms&country=RU&sender_category=paid_registered&traffic_type=any")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	b := parseEditorBody(t, w)
	assert.Nil(t, b.Template, "no binding → template is null")
	require.NotNil(t, b.Plan, "override plan still supplies plan metadata")
	require.Len(t, b.Tiers, 1, "only override tier exists")

	// Walk cells; MTS at the one tier should have source=override (no
	// template price), price_template nil, price_override set.
	for _, c := range b.Cells {
		if c.OperatorID != mts.String() {
			continue
		}
		assert.Equal(t, "override", c.Source)
		assert.Nil(t, c.PriceTemplate, "no template plan → no template price")
		require.NotNil(t, c.PriceOverride)
		assert.InDelta(t, 4.00, *c.PriceOverride, 0.001)
	}
}

func TestEditor_TemplateNotFound_404(t *testing.T) {
	pool := getEditorTestPool(t)
	reseller := seedEditorReseller(t, pool)
	_ = seedEditorCountryRU(t, pool)

	h := NewNetworkTariffEditorHandler(pool)
	w := doEditorGet(t, h, reseller, uuid.New(),
		"mode=template&country=RU&sender_category=paid_registered&traffic_type=any")
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestEditor_CrossResellerIsolation(t *testing.T) {
	pool := getEditorTestPool(t)
	resellerA := seedEditorReseller(t, pool)
	resellerB := seedEditorReseller(t, pool)
	ruID := seedEditorCountryRU(t, pool)
	seedB := seedEditorTemplatePlan(t, pool, resellerB, ruID,
		fmt.Sprintf("tpl-iso-%s", uuid.NewString()[:6]), 1.0, 1.0)

	h := NewNetworkTariffEditorHandler(pool)
	// Reseller A tries to read reseller B's template → 404.
	w := doEditorGet(t, h, resellerA, seedB.TemplateID,
		"mode=template&country=RU&sender_category=paid_registered&traffic_type=any")
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestEditor_ActivePeriodSelection(t *testing.T) {
	pool := getEditorTestPool(t)
	ctx := context.Background()
	reseller := seedEditorReseller(t, pool)
	ruID := seedEditorCountryRU(t, pool)
	_, _ = seedEditorOperators(t, pool, ruID)
	seed := seedEditorTemplatePlan(t, pool, reseller, ruID,
		fmt.Sprintf("tpl-per-%s", uuid.NewString()[:6]), 2.0, 1.8)

	// Close the existing open-ended period and add a past closed period
	// BEFORE it. We cannot have two open-ended periods; shape is:
	//   past:    [today-60d, today-30d)   (end_date set, strictly past)
	//   current: [today-29d, today-1d?)   (we rewrite the existing one)
	// We tighten the existing current period to start yesterday so it stays
	// active, and insert the past one with explicit end_date = today-30d.
	_, err := pool.Exec(ctx, `
		UPDATE reseller_tariff_periods
		SET start_date = CURRENT_DATE - INTERVAL '29 days'
		WHERE id = $1`, seed.PeriodID)
	require.NoError(t, err)

	var pastPeriodID uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
		VALUES ($1, CURRENT_DATE - INTERVAL '60 days', CURRENT_DATE - INTERVAL '30 days')
		RETURNING id`, seed.PlanID).Scan(&pastPeriodID)
	require.NoError(t, err)

	h := NewNetworkTariffEditorHandler(pool)

	// 1) Without period_id → currently-active is picked (not the past one).
	w := doEditorGet(t, h, reseller, seed.TemplateID,
		"mode=template&country=RU&sender_category=paid_registered&traffic_type=any")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	b := parseEditorBody(t, w)
	require.NotNil(t, b.ActivePeriodID)
	assert.Equal(t, seed.PeriodID.String(), *b.ActivePeriodID,
		"no period_id → active period must be the currently-valid one, not past")

	// 2) With period_id=past → past is returned.
	w2 := doEditorGet(t, h, reseller, seed.TemplateID, fmt.Sprintf(
		"mode=template&country=RU&sender_category=paid_registered&traffic_type=any&period_id=%s",
		pastPeriodID))
	require.Equal(t, http.StatusOK, w2.Code, "body=%s", w2.Body.String())
	b2 := parseEditorBody(t, w2)
	require.NotNil(t, b2.ActivePeriodID)
	assert.Equal(t, pastPeriodID.String(), *b2.ActivePeriodID,
		"period_id=past → active period must be the past one")
}
