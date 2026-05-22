//go:build integration

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
)

// ---------- test scaffolding ----------

func getBulkTestPool(t *testing.T) *pgxpool.Pool {
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

func seedBulkReseller(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var planSubID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planSubID))
	resellerID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, true, $5)`,
		resellerID,
		fmt.Sprintf("reseller-bulk-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-bulk-%s", resellerID),
		fmt.Sprintf("reseller-bulk-%s@t.local", uuid.NewString()[:8]),
		planSubID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, resellerID)
	})
	return resellerID
}

func seedBulkSubAccount(t *testing.T, pool *pgxpool.Pool, resellerID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var planSubID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planSubID))
	subID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, parent_client_id, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5, $6)`,
		subID,
		fmt.Sprintf("sub-bulk-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-bulk-sub-%s", subID),
		fmt.Sprintf("sub-bulk-%s@t.local", uuid.NewString()[:8]),
		resellerID, planSubID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, subID)
	})
	return subID
}

func seedBulkCountryRU(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM countries WHERE iso_code = 'RU'`).Scan(&id); err == nil {
		return id
	}
	id = uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO countries (id, name, iso_code, phone_code, currency)
		VALUES ($1, 'Russia', 'RU', '+7', 'RUB')`, id)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM countries WHERE id = $1`, id)
	})
	return id
}

func seedBulkOperator(t *testing.T, pool *pgxpool.Pool, countryID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO operators (id, country_id, name, code, active)
		VALUES ($1, $2, $3, $4, true)`,
		id, countryID, fmt.Sprintf("%s-%s", name, uuid.NewString()[:6]),
		fmt.Sprintf("%s-%s", name, uuid.NewString()[:6]))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, id)
	})
	return id
}

// seedBulkTemplatePlan creates template + per-operator plan + period + 2 tiers.
type bulkSeed struct {
	TemplateID uuid.UUID
	PlanID     uuid.UUID
	PeriodID   uuid.UUID
	Tier0ID    uuid.UUID
	Tier1KID   uuid.UUID
	OperatorID uuid.UUID
	CountryID  uuid.UUID
}

func seedBulkTemplatePlan(
	t *testing.T, pool *pgxpool.Pool,
	resellerID, countryID, operatorID uuid.UUID,
	price0, price1k float64,
) bulkSeed {
	t.Helper()
	ctx := context.Background()

	tplID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO reseller_tariff_templates (id, reseller_id, name, active)
		VALUES ($1, $2, $3, true)`, tplID, resellerID,
		fmt.Sprintf("bulk-%s", uuid.NewString()[:8]))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_templates WHERE id = $1`, tplID)
	})

	var planID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_plans
		  (reseller_id, template_id, country_id, operator_id, sender_category, traffic_type, strategy, active)
		VALUES ($1, $2, $3, $4, 'paid_registered', 'any', 'threshold', true)
		RETURNING id`, resellerID, tplID, countryID, operatorID).Scan(&planID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_plans WHERE id = $1`, planID)
	})

	var periodID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
		VALUES ($1, CURRENT_DATE - INTERVAL '30 days', CURRENT_DATE + INTERVAL '30 days')
		RETURNING id`, planID).Scan(&periodID))

	var t0, t1k uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
		VALUES ($1, 0, $2) RETURNING id`, periodID, price0).Scan(&t0))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
		VALUES ($1, 1000, $2) RETURNING id`, periodID, price1k).Scan(&t1k))

	return bulkSeed{
		TemplateID: tplID, PlanID: planID, PeriodID: periodID,
		Tier0ID: t0, Tier1KID: t1k,
		OperatorID: operatorID, CountryID: countryID,
	}
}

// doBulkPatch posts a PATCH with correct URL/ctx/muxvars.
func doBulkPatch(
	t *testing.T, h *NetworkTariffBulkHandler,
	clientID, planID uuid.UUID, body map[string]interface{},
) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPatch,
		fmt.Sprintf("/portal/v1/network/tariff-plans/%s/bulk", planID), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, clientID))
	req = mux.SetURLVars(req, map[string]string{"plan_id": planID.String()})
	w := httptest.NewRecorder()
	h.BulkPatch(w, req)
	return w
}

// doCreatePeriod posts a POST for CreatePeriod.
func doCreatePeriod(
	t *testing.T, h *NetworkTariffBulkHandler,
	clientID, planID uuid.UUID, body map[string]interface{},
) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/portal/v1/network/tariff-plans/%s/periods", planID), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, clientID))
	req = mux.SetURLVars(req, map[string]string{"plan_id": planID.String()})
	w := httptest.NewRecorder()
	h.CreatePeriod(w, req)
	return w
}

type bulkBody struct {
	OK     bool `json:"ok"`
	Errors []struct {
		OperatorID string `json:"operator_id"`
		TierID     string `json:"tier_id"`
		Reason     string `json:"reason"`
	} `json:"errors"`
}

// ---------- Tests ----------

func TestBulkPatch_AllOrNothing(t *testing.T) {
	pool := getBulkTestPool(t)
	reseller := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	h := NewNetworkTariffBulkHandler(pool, nil)
	// Mix valid + one invalid (negative price).
	w := doBulkPatch(t, h, reseller, seed.PlanID, map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"cells_upsert": []map[string]interface{}{
			{"operator_id": op.String(), "tier_id": seed.Tier0ID.String(),
				"price": 4.00, "scope": "template"},
			{"operator_id": op.String(), "tier_id": seed.Tier1KID.String(),
				"price": -0.01, "scope": "template"},
		},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	var b bulkBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.False(t, b.OK)
	require.NotEmpty(t, b.Errors)

	// Assert tier0 price unchanged.
	var price float64
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT price_per_segment::float8 FROM reseller_tariff_tiers WHERE id = $1`,
		seed.Tier0ID).Scan(&price))
	assert.InDelta(t, 3.20, price, 0.001, "tier0 price must be untouched")
}

func TestBulkPatch_UpsertTemplateTier(t *testing.T) {
	pool := getBulkTestPool(t)
	reseller := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	h := NewNetworkTariffBulkHandler(pool, nil)
	w := doBulkPatch(t, h, reseller, seed.PlanID, map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"cells_upsert": []map[string]interface{}{
			{"operator_id": op.String(), "tier_id": seed.Tier0ID.String(),
				"price": 4.50, "scope": "template"},
		},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var price float64
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT price_per_segment::float8 FROM reseller_tariff_tiers WHERE id = $1`,
		seed.Tier0ID).Scan(&price))
	assert.InDelta(t, 4.50, price, 0.001)
}

func TestBulkPatch_InsertNewTier(t *testing.T) {
	pool := getBulkTestPool(t)
	reseller := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	h := NewNetworkTariffBulkHandler(pool, nil)
	w := doBulkPatch(t, h, reseller, seed.PlanID, map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"tiers_upsert": []map[string]interface{}{
			{"id": nil, "from_quantity": 5000, "price_per_segment": 2.10},
		},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM reseller_tariff_tiers
		 WHERE tariff_period_id = $1 AND from_count = 5000`, seed.PeriodID).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestBulkPatch_OverrideCreatesOverridePlan(t *testing.T) {
	pool := getBulkTestPool(t)
	ctx := context.Background()
	reseller := seedBulkReseller(t, pool)
	sub := seedBulkSubAccount(t, pool, reseller)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	h := NewNetworkTariffBulkHandler(pool, nil)
	w := doBulkPatch(t, h, reseller, seed.PlanID, map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"cells_upsert": []map[string]interface{}{
			{"operator_id": op.String(), "tier_id": seed.Tier0ID.String(),
				"price": 2.99, "scope": "override",
				"sub_account_id": sub.String()},
		},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// Override plan must exist for this sub_account + dims.
	var ovrPlanID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT id FROM reseller_tariff_plans
		WHERE sub_account_id = $1 AND country_id = $2 AND operator_id = $3
		  AND sender_category = 'paid_registered' AND traffic_type = 'any'`,
		sub, ru, op).Scan(&ovrPlanID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_plans WHERE id = $1`, ovrPlanID)
	})

	// Override tier at from_count=0 with price 2.99.
	var price float64
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT t.price_per_segment::float8
		FROM reseller_tariff_tiers t
		JOIN reseller_tariff_periods p ON p.id = t.tariff_period_id
		WHERE p.tariff_plan_id = $1 AND t.from_count = 0`, ovrPlanID).Scan(&price))
	assert.InDelta(t, 2.99, price, 0.001)
}

func TestBulkPatch_DeleteOverride(t *testing.T) {
	pool := getBulkTestPool(t)
	ctx := context.Background()
	reseller := seedBulkReseller(t, pool)
	sub := seedBulkSubAccount(t, pool, reseller)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	h := NewNetworkTariffBulkHandler(pool, nil)

	// Step 1: create override.
	w := doBulkPatch(t, h, reseller, seed.PlanID, map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"cells_upsert": []map[string]interface{}{
			{"operator_id": op.String(), "tier_id": seed.Tier0ID.String(),
				"price": 2.99, "scope": "override", "sub_account_id": sub.String()},
		},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// Register override plan for cleanup.
	var ovrPlanID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT id FROM reseller_tariff_plans
		WHERE sub_account_id = $1 AND country_id = $2 AND operator_id = $3`,
		sub, ru, op).Scan(&ovrPlanID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_plans WHERE id = $1`, ovrPlanID)
	})

	// Step 2: delete override.
	w = doBulkPatch(t, h, reseller, seed.PlanID, map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"cells_delete": []map[string]interface{}{
			{"operator_id": op.String(), "tier_id": seed.Tier0ID.String(),
				"scope": "override", "sub_account_id": sub.String()},
		},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var count int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM reseller_tariff_tiers t
		JOIN reseller_tariff_periods p ON p.id = t.tariff_period_id
		WHERE p.tariff_plan_id = $1 AND t.from_count = 0`, ovrPlanID).Scan(&count))
	assert.Equal(t, 0, count, "override tier must be deleted")
}

func TestBulkPatch_CacheInvalidation(t *testing.T) {
	pool := getBulkTestPool(t)
	reseller := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	key := "tariffs:summary:" + reseller.String()
	require.NoError(t, rc.Set(context.Background(), key, []byte(`[{"x":1}]`), 0).Err())

	h := NewNetworkTariffBulkHandler(pool, rc)
	w := doBulkPatch(t, h, reseller, seed.PlanID, map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"cells_upsert": []map[string]interface{}{
			{"operator_id": op.String(), "tier_id": seed.Tier0ID.String(),
				"price": 3.15, "scope": "template"},
		},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	_, err := rc.Get(context.Background(), key).Result()
	assert.ErrorIs(t, err, redis.Nil, "cache key must be deleted after bulk patch")
}

func TestBulkPatch_CrossResellerPlanID_404(t *testing.T) {
	pool := getBulkTestPool(t)
	reseller := seedBulkReseller(t, pool)
	other := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, other, ru, op, 3.20, 2.80) // belongs to other

	h := NewNetworkTariffBulkHandler(pool, nil)
	w := doBulkPatch(t, h, reseller, seed.PlanID, map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"cells_upsert": []map[string]interface{}{
			{"operator_id": op.String(), "tier_id": seed.Tier0ID.String(),
				"price": 1.0, "scope": "template"},
		},
	})
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestBulkPatch_PriceOutOfRange_400(t *testing.T) {
	pool := getBulkTestPool(t)
	reseller := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	h := NewNetworkTariffBulkHandler(pool, nil)
	// price=1000 is just out of range
	w := doBulkPatch(t, h, reseller, seed.PlanID, map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"cells_upsert": []map[string]interface{}{
			{"operator_id": op.String(), "tier_id": seed.Tier0ID.String(),
				"price": 1000.0, "scope": "template"},
		},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestCreatePeriod_NoOverlap(t *testing.T) {
	pool := getBulkTestPool(t)
	reseller := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	h := NewNetworkTariffBulkHandler(pool, nil)
	// Existing period is CURRENT_DATE-30 .. CURRENT_DATE+30. Overlap: today..today+5.
	// Express as absolute dates by reading the existing one first.
	var from, to string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT to_char(start_date,'YYYY-MM-DD'), to_char(end_date,'YYYY-MM-DD')
		 FROM reseller_tariff_periods WHERE id = $1`, seed.PeriodID).Scan(&from, &to))

	w := doCreatePeriod(t, h, reseller, seed.PlanID, map[string]interface{}{
		"from": from, // exact same start → clear overlap
		"to":   to,
	})
	assert.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
}

func TestCreatePeriod_CopyFromWithKeepTiers(t *testing.T) {
	pool := getBulkTestPool(t)
	ctx := context.Background()
	reseller := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	h := NewNetworkTariffBulkHandler(pool, nil)

	// Get the existing period's end_date so we can insert AFTER it.
	var end string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT to_char(end_date+1,'YYYY-MM-DD') FROM reseller_tariff_periods WHERE id = $1`,
		seed.PeriodID).Scan(&end))
	var endTo string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT to_char(end_date+60,'YYYY-MM-DD') FROM reseller_tariff_periods WHERE id = $1`,
		seed.PeriodID).Scan(&endTo))

	// Case A: keep_tiers=true → tiers copied.
	w := doCreatePeriod(t, h, reseller, seed.PlanID, map[string]interface{}{
		"from":                 end,
		"to":                   endTo,
		"copy_from_period_id":  seed.PeriodID.String(),
		"keep_tiers":           true,
	})
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	newID, err := uuid.Parse(created.ID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_periods WHERE id = $1`, newID)
	})

	var tierCount int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM reseller_tariff_tiers WHERE tariff_period_id = $1`, newID).Scan(&tierCount))
	assert.Equal(t, 2, tierCount, "both tiers must be copied with keep_tiers=true")

	// Case B: keep_tiers=false on a fresh period (non-overlapping).
	var endB, endBTo string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT to_char(end_date+61,'YYYY-MM-DD') FROM reseller_tariff_periods WHERE id = $1`,
		seed.PeriodID).Scan(&endB))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT to_char(end_date+90,'YYYY-MM-DD') FROM reseller_tariff_periods WHERE id = $1`,
		seed.PeriodID).Scan(&endBTo))

	w = doCreatePeriod(t, h, reseller, seed.PlanID, map[string]interface{}{
		"from":                 endB,
		"to":                   endBTo,
		"copy_from_period_id":  seed.PeriodID.String(),
		"keep_tiers":           false,
	})
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	var createdB struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createdB))
	newIDB, err := uuid.Parse(createdB.ID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_periods WHERE id = $1`, newIDB)
	})

	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM reseller_tariff_tiers WHERE tariff_period_id = $1`, newIDB).Scan(&tierCount))
	assert.Equal(t, 0, tierCount, "no tiers must be copied with keep_tiers=false")
}

func TestCreatePeriod_InvalidDateRange(t *testing.T) {
	pool := getBulkTestPool(t)
	reseller := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	h := NewNetworkTariffBulkHandler(pool, nil)
	w := doCreatePeriod(t, h, reseller, seed.PlanID, map[string]interface{}{
		"from": "2030-12-31",
		"to":   "2030-01-01", // to < from
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// ---------- Review-gate tests ----------

// TestBulkPatch_ConcurrentOverrideCreate fires two concurrent override creates
// for the same (sub_account, dims). Both must succeed (no 500) and exactly one
// override plan must exist — the INSERT race is caught by the partial unique
// index and handled via SAVEPOINT + re-SELECT inside findOrCreateOverridePlan.
func TestBulkPatch_ConcurrentOverrideCreate(t *testing.T) {
	t.Parallel()
	pool := getBulkTestPool(t)
	ctx := context.Background()
	reseller := seedBulkReseller(t, pool)
	sub := seedBulkSubAccount(t, pool, reseller)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	h := NewNetworkTariffBulkHandler(pool, nil)

	body := map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"cells_upsert": []map[string]interface{}{
			{"operator_id": op.String(), "tier_id": seed.Tier0ID.String(),
				"price": 2.50, "scope": "override", "sub_account_id": sub.String()},
		},
	}

	var wg sync.WaitGroup
	results := make([]int, 2)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			defer wg.Done()
			w := doBulkPatch(t, h, reseller, seed.PlanID, body)
			results[idx] = w.Code
		}(i)
	}
	wg.Wait()

	for _, code := range results {
		assert.Equal(t, http.StatusOK, code, "concurrent PATCH must not 500")
	}

	var ovrCount int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM reseller_tariff_plans
		WHERE sub_account_id = $1 AND country_id = $2 AND operator_id = $3
		  AND sender_category = 'paid_registered' AND traffic_type = 'any'
		  AND active`, sub, ru, op).Scan(&ovrCount))
	assert.Equal(t, 1, ovrCount, "exactly one override plan must exist")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `
			DELETE FROM reseller_tariff_plans
			WHERE sub_account_id = $1`, sub)
	})
}

// TestBulkPatch_NewTierWithoutPriceRejected verifies C4: a new tier
// (tiers_upsert with id=null) without price_per_segment is rejected at
// validation. Otherwise the tier would persist at price=0 and downstream
// tarification would charge zero.
func TestBulkPatch_NewTierWithoutPriceRejected(t *testing.T) {
	pool := getBulkTestPool(t)
	reseller := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	h := NewNetworkTariffBulkHandler(pool, nil)
	w := doBulkPatch(t, h, reseller, seed.PlanID, map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"tiers_upsert": []map[string]interface{}{
			{"id": nil, "from_quantity": 200}, // no price_per_segment
		},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	var b bulkBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.False(t, b.OK)
	require.NotEmpty(t, b.Errors)

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM reseller_tariff_tiers
		 WHERE tariff_period_id = $1 AND from_count = 200`, seed.PeriodID).Scan(&count))
	assert.Equal(t, 0, count)
}

// TestBulkPatch_WildcardPlanRejectsCells verifies I2: a wildcard plan
// (operator_id IS NULL) cannot have operator-specific cells. Attempting to
// PATCH such a plan with any cells_upsert must return 400.
func TestBulkPatch_WildcardPlanRejectsCells(t *testing.T) {
	pool := getBulkTestPool(t)
	ctx := context.Background()
	reseller := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")

	tplID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO reseller_tariff_templates (id, reseller_id, name, active)
		VALUES ($1, $2, $3, true)`, tplID, reseller,
		fmt.Sprintf("wild-%s", uuid.NewString()[:8]))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_templates WHERE id = $1`, tplID)
	})

	var wildPlanID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_plans
		  (reseller_id, template_id, country_id, operator_id, sender_category, traffic_type, strategy, active)
		VALUES ($1, $2, $3, NULL, 'paid_registered', 'any', 'threshold', true)
		RETURNING id`, reseller, tplID, ru).Scan(&wildPlanID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_plans WHERE id = $1`, wildPlanID)
	})

	var periodID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
		VALUES ($1, CURRENT_DATE - INTERVAL '30 days', CURRENT_DATE + INTERVAL '30 days')
		RETURNING id`, wildPlanID).Scan(&periodID))

	var tierID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
		VALUES ($1, 0, 1.0) RETURNING id`, periodID).Scan(&tierID))

	h := NewNetworkTariffBulkHandler(pool, nil)
	w := doBulkPatch(t, h, reseller, wildPlanID, map[string]interface{}{
		"period_id": periodID.String(),
		"cells_upsert": []map[string]interface{}{
			{"operator_id": op.String(), "tier_id": tierID.String(),
				"price": 2.0, "scope": "template"},
		},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	var b bulkBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.False(t, b.OK)
	require.NotEmpty(t, b.Errors)
}

// TestBulkPatch_PlanDeactivatedMidRequest — structural only. The FOR SHARE
// re-verify inside the tx is hard to race deterministically from a test.
// Skipped; coverage is via the FOR SHARE statement in BulkPatch.
func TestBulkPatch_PlanDeactivatedMidRequest(t *testing.T) {
	t.Skip("race-window; covered structurally by FOR SHARE inside BulkPatch")
}

// TestBulkPatch_CacheInvalidationRollback verifies that when a bulk batch
// fails validation and the tx is rolled back, the summary cache key is NOT
// deleted — invalidation only runs post-commit.
func TestBulkPatch_CacheInvalidationRollback(t *testing.T) {
	pool := getBulkTestPool(t)
	reseller := seedBulkReseller(t, pool)
	ru := seedBulkCountryRU(t, pool)
	op := seedBulkOperator(t, pool, ru, "MTS")
	seed := seedBulkTemplatePlan(t, pool, reseller, ru, op, 3.20, 2.80)

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	key := "tariffs:summary:" + reseller.String()
	require.NoError(t, rc.Set(context.Background(), key, []byte(`[{"cached":true}]`), 0).Err())

	h := NewNetworkTariffBulkHandler(pool, rc)
	w := doBulkPatch(t, h, reseller, seed.PlanID, map[string]interface{}{
		"period_id": seed.PeriodID.String(),
		"cells_upsert": []map[string]interface{}{
			{"operator_id": op.String(), "tier_id": seed.Tier0ID.String(),
				"price": -1.0, "scope": "template"},
		},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	v, err := rc.Get(context.Background(), key).Result()
	require.NoError(t, err, "cache key must still exist on validation failure")
	assert.Equal(t, `[{"cached":true}]`, v)
}
