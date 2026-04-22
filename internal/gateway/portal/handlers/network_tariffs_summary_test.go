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
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
)

// getTestPool opens a pgx pool against the test database.
// Skips if TEST_DATABASE_URL is not set (matches tests/integration/rls_test.go convention).
func getTestPool(t *testing.T) *pgxpool.Pool {
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

// seedResellerSummaryFixtures creates a reseller, two sub-accounts, a template,
// a template-plan binding (via sub_account_template_assignments), and an override plan.
// Returns the reseller ID and the two sub-account IDs (bound, overridden).
type summaryFixture struct {
	ResellerID      uuid.UUID
	BoundSubID      uuid.UUID
	OverrideSubID   uuid.UUID
	TemplateID      uuid.UUID
	TemplateName    string
	BoundSubName    string
	BoundSubEmail   string
	OverrideSubName string
}

func seedResellerSummaryFixtures(t *testing.T, pool *pgxpool.Pool) summaryFixture {
	t.Helper()
	ctx := context.Background()

	// Get any subscription plan for clients.plan_id NOT NULL constraint.
	var planID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planID)
	require.NoError(t, err, "need at least one subscription plan seeded")

	fx := summaryFixture{
		ResellerID:      uuid.New(),
		BoundSubID:      uuid.New(),
		OverrideSubID:   uuid.New(),
		TemplateID:      uuid.New(),
		TemplateName:    fmt.Sprintf("tpl-%s", uuid.NewString()[:8]),
		BoundSubName:    fmt.Sprintf("sub-bound-%s", uuid.NewString()[:8]),
		BoundSubEmail:   fmt.Sprintf("bound-%s@t.local", uuid.NewString()[:8]),
		OverrideSubName: fmt.Sprintf("sub-over-%s", uuid.NewString()[:8]),
	}

	// Reseller (top-level, is_reseller=true)
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, true, $5)`,
		fx.ResellerID,
		fmt.Sprintf("reseller-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-reseller-%s", fx.ResellerID),
		fmt.Sprintf("reseller-%s@t.local", uuid.NewString()[:8]),
		planID)
	require.NoError(t, err)

	// Sub-account A (bound to template)
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, parent_client_id, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5, $6)`,
		fx.BoundSubID, fx.BoundSubName,
		fmt.Sprintf("apikey-bound-%s", fx.BoundSubID),
		fx.BoundSubEmail, fx.ResellerID, planID)
	require.NoError(t, err)

	// Sub-account B (override only, no template binding)
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, parent_client_id, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5, $6)`,
		fx.OverrideSubID, fx.OverrideSubName,
		fmt.Sprintf("apikey-over-%s", fx.OverrideSubID),
		fmt.Sprintf("over-%s@t.local", uuid.NewString()[:8]),
		fx.ResellerID, planID)
	require.NoError(t, err)

	// Template owned by reseller
	_, err = pool.Exec(ctx, `
		INSERT INTO reseller_tariff_templates (id, reseller_id, name, active)
		VALUES ($1, $2, $3, true)`,
		fx.TemplateID, fx.ResellerID, fx.TemplateName)
	require.NoError(t, err)

	// Template binding for sub-account A
	_, err = pool.Exec(ctx, `
		INSERT INTO sub_account_template_assignments (sub_account_id, template_id)
		VALUES ($1, $2)`, fx.BoundSubID, fx.TemplateID)
	require.NoError(t, err)

	// Override plan for sub-account B (direct sub_account_id, not template-scoped)
	_, err = pool.Exec(ctx, `
		INSERT INTO reseller_tariff_plans
		  (reseller_id, sub_account_id, sender_category, traffic_type, strategy, active)
		VALUES ($1, $2, 'standard', 'any', 'fixed', true)`,
		fx.ResellerID, fx.OverrideSubID)
	require.NoError(t, err)

	// Cleanup (best-effort) — delete in FK-safe order
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM reseller_tariff_plans WHERE reseller_id = $1`, fx.ResellerID)
		_, _ = pool.Exec(ctx, `DELETE FROM sub_account_template_assignments WHERE sub_account_id IN ($1, $2)`,
			fx.BoundSubID, fx.OverrideSubID)
		_, _ = pool.Exec(ctx, `DELETE FROM reseller_tariff_templates WHERE id = $1`, fx.TemplateID)
		_, _ = pool.Exec(ctx, `DELETE FROM clients WHERE id IN ($1, $2, $3)`,
			fx.BoundSubID, fx.OverrideSubID, fx.ResellerID)
	})

	return fx
}

func TestSubaccountsSummary_ReturnsOwnReseller(t *testing.T) {
	pool := getTestPool(t)
	fx := seedResellerSummaryFixtures(t, pool)

	h := NewNetworkTariffsSummaryHandler(pool, nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariffs/subaccounts-summary", nil)
	ctx := context.WithValue(req.Context(), middleware.ClientIDKey, fx.ResellerID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var items []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Len(t, items, 2, "expected 2 sub-accounts for the reseller, got: %s", w.Body.String())

	byID := map[string]map[string]interface{}{}
	for _, it := range items {
		byID[it["sub_account_id"].(string)] = it
	}

	bound, ok := byID[fx.BoundSubID.String()]
	require.True(t, ok, "bound sub-account missing: %+v", byID)
	assert.Equal(t, fx.BoundSubName, bound["sub_account_name"])
	assert.Equal(t, fx.BoundSubEmail, bound["sub_account_email"])
	assert.Equal(t, fx.TemplateID.String(), bound["template_id"])
	assert.Equal(t, fx.TemplateName, bound["template_name"])
	assert.EqualValues(t, 0, bound["override_count"])
	assert.Nil(t, bound["avg_price_per_sms"]) // Task 1: not yet computed
	assert.Equal(t, "RUB", bound["currency"])

	overRow, ok := byID[fx.OverrideSubID.String()]
	require.True(t, ok, "override sub-account missing: %+v", byID)
	assert.Equal(t, fx.OverrideSubName, overRow["sub_account_name"])
	assert.Nil(t, overRow["template_id"])
	assert.Nil(t, overRow["template_name"])
	assert.EqualValues(t, 1, overRow["override_count"])
	assert.Nil(t, overRow["avg_price_per_sms"])
	assert.Equal(t, "RUB", overRow["currency"])
}

func TestSubaccountsSummary_CrossResellerIsolation(t *testing.T) {
	pool := getTestPool(t)
	ctx := context.Background()

	var planID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planID)
	require.NoError(t, err, "need at least one subscription plan seeded")

	resellerAID := uuid.New()
	resellerBID := uuid.New()
	subAID := uuid.New()
	subBID := uuid.New()
	subAName := fmt.Sprintf("A-child-%s", uuid.NewString()[:8])
	subBName := fmt.Sprintf("B-child-%s", uuid.NewString()[:8])

	// Reseller A
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, true, $5)`,
		resellerAID,
		fmt.Sprintf("reseller-A-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-reseller-A-%s", resellerAID),
		fmt.Sprintf("reseller-A-%s@t.local", uuid.NewString()[:8]),
		planID)
	require.NoError(t, err)

	// Reseller B
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, true, $5)`,
		resellerBID,
		fmt.Sprintf("reseller-B-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-reseller-B-%s", resellerBID),
		fmt.Sprintf("reseller-B-%s@t.local", uuid.NewString()[:8]),
		planID)
	require.NoError(t, err)

	// Sub-account under reseller A
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, parent_client_id, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5, $6)`,
		subAID, subAName,
		fmt.Sprintf("apikey-A-child-%s", subAID),
		fmt.Sprintf("A-child-%s@t.local", uuid.NewString()[:8]),
		resellerAID, planID)
	require.NoError(t, err)

	// Sub-account under reseller B
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, parent_client_id, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5, $6)`,
		subBID, subBName,
		fmt.Sprintf("apikey-B-child-%s", subBID),
		fmt.Sprintf("B-child-%s@t.local", uuid.NewString()[:8]),
		resellerBID, planID)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM clients WHERE id IN ($1, $2, $3, $4)`,
			subAID, subBID, resellerAID, resellerBID)
	})

	h := NewNetworkTariffsSummaryHandler(pool, nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariffs/subaccounts-summary", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, resellerAID))

	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var items []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Len(t, items, 1, "expected exactly 1 sub-account for reseller A, got: %s", w.Body.String())

	assert.Equal(t, subAID.String(), items[0]["sub_account_id"])
	assert.Equal(t, subAName, items[0]["sub_account_name"])

	for _, it := range items {
		assert.NotEqual(t, subBName, it["sub_account_name"], "reseller B's sub-account leaked into reseller A's view")
		assert.NotEqual(t, subBID.String(), it["sub_account_id"], "reseller B's sub-account id leaked into reseller A's view")
	}
}

func TestSubaccountsSummary_Unauthorized(t *testing.T) {
	pool := getTestPool(t)

	h := NewNetworkTariffsSummaryHandler(pool, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariffs/subaccounts-summary", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSubaccountsSummary_EmptyForResellerWithNoSubs(t *testing.T) {
	pool := getTestPool(t)
	ctx := context.Background()

	var planID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planID)
	require.NoError(t, err, "need at least one subscription plan seeded")

	resellerID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, true, $5)`,
		resellerID,
		fmt.Sprintf("reseller-empty-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-reseller-empty-%s", resellerID),
		fmt.Sprintf("reseller-empty-%s@t.local", uuid.NewString()[:8]),
		planID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, resellerID)
	})

	h := NewNetworkTariffsSummaryHandler(pool, nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariffs/subaccounts-summary", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, resellerID))

	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, "[]\n", w.Body.String(), "expected JSON empty array, not null")
}

func TestSubaccountsSummary_NotReseller(t *testing.T) {
	pool := getTestPool(t)
	ctx := context.Background()

	var planID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planID)
	require.NoError(t, err)

	nonResellerID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, active, is_reseller, plan_id)
		VALUES ($1, 'non-reseller', $2, 'secret', true, false, $3)`,
		nonResellerID, fmt.Sprintf("apikey-%s", nonResellerID), planID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, nonResellerID)
	})

	h := NewNetworkTariffsSummaryHandler(pool, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariffs/subaccounts-summary", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, nonResellerID))
	w := httptest.NewRecorder()
	h.List(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// seedAvgPriceFixture creates a reseller + sub-account with two override plans
// (MTS tier0=3.0, MegaFon tier0=3.6) for RU × paid_registered. Returns
// reseller and sub-account IDs. Cleans up via t.Cleanup.
func seedAvgPriceFixture(t *testing.T, pool *pgxpool.Pool) (resellerID, subID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	var planSubID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planSubID)
	require.NoError(t, err)

	// Get or create RU country + two operators.
	var ruID uuid.UUID
	err = pool.QueryRow(ctx, `SELECT id FROM countries WHERE iso_code = 'RU'`).Scan(&ruID)
	if err != nil {
		ruID = uuid.New()
		_, err = pool.Exec(ctx, `INSERT INTO countries (id, name, iso_code, phone_code, currency)
			VALUES ($1, 'Russia', 'RU', '7', 'RUB')`, ruID)
		require.NoError(t, err)
	}

	// Use unique operator codes so we don't collide with seeded ones.
	mtsID := uuid.New()
	megaID := uuid.New()
	mtsCode := fmt.Sprintf("MTS-t-%s", uuid.NewString()[:6])
	megaCode := fmt.Sprintf("MEG-t-%s", uuid.NewString()[:6])
	_, err = pool.Exec(ctx, `INSERT INTO operators (id, country_id, name, code) VALUES ($1, $2, 'MTS', $3), ($4, $2, 'MegaFon', $5)`,
		mtsID, ruID, mtsCode, megaID, megaCode)
	require.NoError(t, err)

	resellerID = uuid.New()
	subID = uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, true, $5)`,
		resellerID,
		fmt.Sprintf("reseller-avg-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-avg-res-%s", resellerID),
		fmt.Sprintf("avg-res-%s@t.local", uuid.NewString()[:8]),
		planSubID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, parent_client_id, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5, $6)`,
		subID,
		fmt.Sprintf("sub-avg-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-avg-sub-%s", subID),
		fmt.Sprintf("avg-sub-%s@t.local", uuid.NewString()[:8]),
		resellerID, planSubID)
	require.NoError(t, err)

	// Two override plans (one per operator).
	var plan1, plan2 uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_plans
		  (reseller_id, sub_account_id, country_id, operator_id, sender_category, traffic_type, strategy, active)
		VALUES ($1, $2, $3, $4, 'paid_registered', 'any', 'fixed', true) RETURNING id`,
		resellerID, subID, ruID, mtsID).Scan(&plan1)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_plans
		  (reseller_id, sub_account_id, country_id, operator_id, sender_category, traffic_type, strategy, active)
		VALUES ($1, $2, $3, $4, 'paid_registered', 'any', 'fixed', true) RETURNING id`,
		resellerID, subID, ruID, megaID).Scan(&plan2)
	require.NoError(t, err)

	// Active period (start yesterday, no end) + tier0 prices.
	for _, pp := range []struct {
		planID uuid.UUID
		price  float64
	}{
		{plan1, 3.0},
		{plan2, 3.6},
	} {
		var periodID uuid.UUID
		err = pool.QueryRow(ctx, `
			INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
			VALUES ($1, CURRENT_DATE - INTERVAL '1 day', NULL) RETURNING id`, pp.planID).Scan(&periodID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
			VALUES ($1, 0, $2)`, periodID, pp.price)
		require.NoError(t, err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM reseller_tariff_plans WHERE reseller_id = $1`, resellerID)
		_, _ = pool.Exec(ctx, `DELETE FROM clients WHERE id IN ($1, $2)`, subID, resellerID)
		_, _ = pool.Exec(ctx, `DELETE FROM operators WHERE id IN ($1, $2)`, mtsID, megaID)
	})

	return resellerID, subID
}

func TestSubaccountsSummary_AvgPrice(t *testing.T) {
	pool := getTestPool(t)
	resellerID, subID := seedAvgPriceFixture(t, pool)

	h := NewNetworkTariffsSummaryHandler(pool, nil)

	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariffs/subaccounts-summary", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, resellerID))

	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var items []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Len(t, items, 1)
	require.Equal(t, subID.String(), items[0]["sub_account_id"])

	avg, ok := items[0]["avg_price_per_sms"].(float64)
	require.True(t, ok, "expected numeric avg_price_per_sms, got %#v", items[0]["avg_price_per_sms"])
	assert.InDelta(t, 3.3, avg, 0.0001, "expected (3.0 + 3.6)/2 = 3.3")
}

func TestSubaccountsSummary_CachesResult(t *testing.T) {
	pool := getTestPool(t)
	resellerID, subID := seedAvgPriceFixture(t, pool)

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	h := NewNetworkTariffsSummaryHandler(pool, rc)

	// First call — populates cache from DB.
	req1 := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariffs/subaccounts-summary", nil)
	req1 = req1.WithContext(context.WithValue(req1.Context(), middleware.ClientIDKey, resellerID))
	w1 := httptest.NewRecorder()
	h.List(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code, "body=%s", w1.Body.String())
	body1 := w1.Body.Bytes()

	// Mutate underlying tier so a fresh DB read would return a different avg.
	_, err := pool.Exec(context.Background(), `
		UPDATE reseller_tariff_tiers SET price_per_segment = 99.99
		WHERE tariff_period_id IN (
			SELECT pr.id FROM reseller_tariff_periods pr
			JOIN reseller_tariff_plans p ON p.id = pr.tariff_plan_id
			WHERE p.sub_account_id = $1
		) AND from_count = 0`, subID)
	require.NoError(t, err)

	// Second call — must come from cache, so identical bytes despite DB change.
	req2 := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariffs/subaccounts-summary", nil)
	req2 = req2.WithContext(context.WithValue(req2.Context(), middleware.ClientIDKey, resellerID))
	w2 := httptest.NewRecorder()
	h.List(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, body1, w2.Body.Bytes(), "cache hit must return byte-identical body")

	// Sanity: after TTL expiry, the next read should hit DB again and reflect the mutation.
	mr.FastForward(6 * time.Minute)
	req3 := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariffs/subaccounts-summary", nil)
	req3 = req3.WithContext(context.WithValue(req3.Context(), middleware.ClientIDKey, resellerID))
	w3 := httptest.NewRecorder()
	h.List(w3, req3)
	require.Equal(t, http.StatusOK, w3.Code)
	assert.NotEqual(t, body1, w3.Body.Bytes(), "after TTL expiry cache should miss and reflect DB changes")
}
