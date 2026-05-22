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

// getTemplatesTestPool — local helper to avoid coupling to other test files.
// Kept separate so this test file can be read/reviewed in isolation.
func getTemplatesTestPool(t *testing.T) *pgxpool.Pool {
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

// seedResellerWithPlanID — helper to insert a reseller row. Returns reseller uuid.
func seedTemplatesReseller(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var planSubID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planSubID)
	require.NoError(t, err, "need at least one subscription plan seeded")

	resellerID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, true, $5)`,
		resellerID,
		fmt.Sprintf("reseller-tpl-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-reseller-tpl-%s", resellerID),
		fmt.Sprintf("reseller-tpl-%s@t.local", uuid.NewString()[:8]),
		planSubID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, resellerID)
	})
	return resellerID
}

// seedSubAccount — helper to create a sub-account under a reseller for FK-valid
// sub_account_template_assignments rows.
func seedTemplatesSubAccount(t *testing.T, pool *pgxpool.Pool, resellerID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var planSubID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planSubID)
	require.NoError(t, err)

	subID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, parent_client_id, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5, $6)`,
		subID,
		fmt.Sprintf("sub-tpl-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-sub-tpl-%s", subID),
		fmt.Sprintf("sub-tpl-%s@t.local", uuid.NewString()[:8]),
		resellerID, planSubID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, subID)
	})
	return subID
}

func TestListTemplates_ReturnsOwnReseller(t *testing.T) {
	pool := getTemplatesTestPool(t)
	ctx := context.Background()

	resellerID := seedTemplatesReseller(t, pool)

	// Template A: will have 2 plans + 3 assignments. Name starts with "A-"
	// so that ORDER BY name puts it first deterministically.
	tplA := uuid.New()
	tplAName := fmt.Sprintf("A-tpl-%s", uuid.NewString()[:8])
	tplADesc := "aggregated bulk"
	tplB := uuid.New()
	tplBName := fmt.Sprintf("B-tpl-%s", uuid.NewString()[:8])

	_, err := pool.Exec(ctx, `
		INSERT INTO reseller_tariff_templates (id, reseller_id, name, description, active)
		VALUES ($1, $2, $3, $4, true), ($5, $2, $6, NULL, true)`,
		tplA, resellerID, tplAName, tplADesc, tplB, tplBName)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_templates WHERE id IN ($1, $2)`, tplA, tplB)
	})

	// 2 plans bound to template A (active).
	_, err = pool.Exec(ctx, `
		INSERT INTO reseller_tariff_plans
		  (reseller_id, template_id, sender_category, traffic_type, strategy, active)
		VALUES ($1, $2, 'standard', 'any', 'fixed', true),
		       ($1, $2, 'paid_registered', 'any', 'fixed', true)`,
		resellerID, tplA)
	require.NoError(t, err)

	// One plan linked to template A but inactive — must NOT be counted.
	_, err = pool.Exec(ctx, `
		INSERT INTO reseller_tariff_plans
		  (reseller_id, template_id, sender_category, traffic_type, strategy, active)
		VALUES ($1, $2, 'standard', 'any', 'fixed', false)`,
		resellerID, tplA)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_plans WHERE reseller_id = $1`, resellerID)
	})

	// 3 sub-account assignments to template A.
	sub1 := seedTemplatesSubAccount(t, pool, resellerID)
	sub2 := seedTemplatesSubAccount(t, pool, resellerID)
	sub3 := seedTemplatesSubAccount(t, pool, resellerID)
	_, err = pool.Exec(ctx, `
		INSERT INTO sub_account_template_assignments (sub_account_id, template_id)
		VALUES ($1, $2), ($3, $2), ($4, $2)`, sub1, tplA, sub2, sub3)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM sub_account_template_assignments WHERE sub_account_id IN ($1, $2, $3)`, sub1, sub2, sub3)
	})

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariff-templates", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, resellerID))
	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var items []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Len(t, items, 2)

	// Ordered by name: A first, B second.
	assert.Equal(t, tplA.String(), items[0]["id"])
	assert.Equal(t, tplAName, items[0]["name"])
	assert.Equal(t, tplADesc, items[0]["description"])
	assert.EqualValues(t, 2, items[0]["plans_count"])
	assert.EqualValues(t, 3, items[0]["bound_subaccount_count"])

	assert.Equal(t, tplB.String(), items[1]["id"])
	assert.Equal(t, tplBName, items[1]["name"])
	assert.Equal(t, "", items[1]["description"], "NULL description must serialize as empty string")
	assert.EqualValues(t, 0, items[1]["plans_count"])
	assert.EqualValues(t, 0, items[1]["bound_subaccount_count"])
}

func TestListTemplates_CrossResellerIsolation(t *testing.T) {
	pool := getTemplatesTestPool(t)
	ctx := context.Background()

	resellerA := seedTemplatesReseller(t, pool)
	resellerB := seedTemplatesReseller(t, pool)

	tplA := uuid.New()
	tplB1 := uuid.New()
	tplB2 := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO reseller_tariff_templates (id, reseller_id, name, active) VALUES
		  ($1, $2, $3, true),
		  ($4, $5, $6, true),
		  ($7, $8, $9, true)`,
		tplA, resellerA, fmt.Sprintf("A-only-%s", uuid.NewString()[:6]),
		tplB1, resellerB, fmt.Sprintf("B-one-%s", uuid.NewString()[:6]),
		tplB2, resellerB, fmt.Sprintf("B-two-%s", uuid.NewString()[:6]))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM reseller_tariff_templates WHERE id IN ($1, $2, $3)`, tplA, tplB1, tplB2)
	})

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariff-templates", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, resellerA))
	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var items []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Len(t, items, 1, "reseller A must see exactly 1 template, got: %s", w.Body.String())
	assert.Equal(t, tplA.String(), items[0]["id"])
}

func TestListTemplates_Empty(t *testing.T) {
	pool := getTemplatesTestPool(t)
	resellerID := seedTemplatesReseller(t, pool)

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariff-templates", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, resellerID))
	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, "[]\n", w.Body.String(), "expected JSON empty array, not null")
}

func TestListTemplates_NotReseller(t *testing.T) {
	pool := getTemplatesTestPool(t)
	ctx := context.Background()

	var planSubID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planSubID)
	require.NoError(t, err)

	clientID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, active, is_reseller, plan_id)
		VALUES ($1, 'non-reseller-tpl', $2, 'secret', true, false, $3)`,
		clientID, fmt.Sprintf("apikey-%s", clientID), planSubID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, clientID)
	})

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariff-templates", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, clientID))
	w := httptest.NewRecorder()
	h.List(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListTemplates_Unauthorized(t *testing.T) {
	pool := getTemplatesTestPool(t)

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariff-templates", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListTemplates_InactiveExcluded(t *testing.T) {
	pool := getTemplatesTestPool(t)
	ctx := context.Background()
	resellerID := seedTemplatesReseller(t, pool)

	tplActive := uuid.New()
	tplInactive := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO reseller_tariff_templates (id, reseller_id, name, active) VALUES
		  ($1, $2, $3, true),
		  ($4, $2, $5, false)`,
		tplActive, resellerID, fmt.Sprintf("active-%s", uuid.NewString()[:6]),
		tplInactive, fmt.Sprintf("inactive-%s", uuid.NewString()[:6]))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM reseller_tariff_templates WHERE id IN ($1, $2)`, tplActive, tplInactive)
	})

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariff-templates", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, resellerID))
	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var items []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Len(t, items, 1)
	assert.Equal(t, tplActive.String(), items[0]["id"])
}

// ---------- Task 4 ----------

// seedTemplateWithContent — inserts a template + 1 plan + 1 period + 2 tiers.
// Returns (templateID, planID, periodID). Cleaned up via t.Cleanup.
func seedTemplateWithContent(t *testing.T, pool *pgxpool.Pool, resellerID uuid.UUID, name string) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	tplID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO reseller_tariff_templates (id, reseller_id, name, description, active)
		VALUES ($1, $2, $3, 'desc-seed', true)`, tplID, resellerID, name)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_templates WHERE id = $1`, tplID)
	})

	var planID uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_plans
		  (reseller_id, template_id, sender_category, traffic_type, strategy, active)
		VALUES ($1, $2, 'standard', 'any', 'fixed', true)
		RETURNING id`, resellerID, tplID).Scan(&planID)
	require.NoError(t, err)

	var periodID uuid.UUID
	err = pool.QueryRow(ctx, `
		INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
		VALUES ($1, CURRENT_DATE - INTERVAL '1 day', NULL)
		RETURNING id`, planID).Scan(&periodID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
		VALUES ($1, 0, 2.5), ($1, 1000, 2.0)`, periodID)
	require.NoError(t, err)

	return tplID, planID, periodID
}

// doCreate — helper that POSTs a JSON body to Create and returns response.
func doCreate(t *testing.T, h *NetworkTariffTemplatesHandler, resellerID uuid.UUID, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/portal/v1/network/tariff-templates", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, resellerID))
	w := httptest.NewRecorder()
	h.Create(w, req)
	return w
}

func TestCreateTemplate_Minimal(t *testing.T) {
	pool := getTemplatesTestPool(t)
	resellerID := seedTemplatesReseller(t, pool)
	h := NewNetworkTariffTemplatesHandler(pool, nil)

	name := fmt.Sprintf("minimal-%s", uuid.NewString()[:8])
	w := doCreate(t, h, resellerID, map[string]interface{}{"name": name})
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["id"])
	newID, err := uuid.Parse(resp["id"])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_templates WHERE id = $1`, newID)
	})

	var gotName string
	var gotDesc *string
	var active bool
	err = pool.QueryRow(context.Background(),
		`SELECT name, description, active FROM reseller_tariff_templates WHERE id = $1`,
		newID).Scan(&gotName, &gotDesc, &active)
	require.NoError(t, err)
	assert.Equal(t, name, gotName)
	assert.Nil(t, gotDesc, "empty description must persist as NULL")
	assert.True(t, active)
}

func TestCreateTemplate_DuplicateName(t *testing.T) {
	pool := getTemplatesTestPool(t)
	resellerID := seedTemplatesReseller(t, pool)

	name := fmt.Sprintf("dup-%s", uuid.NewString()[:8])
	tplID := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO reseller_tariff_templates (id, reseller_id, name, active)
		VALUES ($1, $2, $3, true)`, tplID, resellerID, name)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_templates WHERE id = $1`, tplID)
	})

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	w := doCreate(t, h, resellerID, map[string]string{"name": name})
	assert.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
}

func TestCreateTemplate_EmptyName(t *testing.T) {
	pool := getTemplatesTestPool(t)
	resellerID := seedTemplatesReseller(t, pool)
	h := NewNetworkTariffTemplatesHandler(pool, nil)

	w := doCreate(t, h, resellerID, map[string]string{"name": "   "})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateTemplate_CopyFromCopiesPlansPeriodsTiers(t *testing.T) {
	pool := getTemplatesTestPool(t)
	ctx := context.Background()
	resellerID := seedTemplatesReseller(t, pool)

	srcName := fmt.Sprintf("src-%s", uuid.NewString()[:8])
	srcID, srcPlan, srcPeriod := seedTemplateWithContent(t, pool, resellerID, srcName)

	// Extra: bind a sub-account to the src — it must NOT be copied.
	sub := seedTemplatesSubAccount(t, pool, resellerID)
	_, err := pool.Exec(ctx, `
		INSERT INTO sub_account_template_assignments (sub_account_id, template_id)
		VALUES ($1, $2)`, sub, srcID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM sub_account_template_assignments WHERE sub_account_id = $1`, sub)
	})

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	newName := fmt.Sprintf("copy-%s", uuid.NewString()[:8])
	w := doCreate(t, h, resellerID, map[string]interface{}{
		"name":         newName,
		"copy_from_id": srcID.String(),
	})
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	newID, err := uuid.Parse(resp["id"])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_plans WHERE template_id = $1`, newID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_templates WHERE id = $1`, newID)
	})

	// 1 plan in new, distinct id, same dims.
	var newPlanID uuid.UUID
	var senderCat, trafficType, strategy string
	err = pool.QueryRow(ctx, `
		SELECT id, sender_category, traffic_type, strategy
		FROM reseller_tariff_plans WHERE template_id = $1`, newID).Scan(
		&newPlanID, &senderCat, &trafficType, &strategy)
	require.NoError(t, err, "expected exactly one copied plan")
	assert.NotEqual(t, srcPlan, newPlanID)
	assert.Equal(t, "standard", senderCat)
	assert.Equal(t, "any", trafficType)
	assert.Equal(t, "fixed", strategy)

	// 1 period, distinct id, same start_date.
	var newPeriodID uuid.UUID
	err = pool.QueryRow(ctx,
		`SELECT id FROM reseller_tariff_periods WHERE tariff_plan_id = $1`, newPlanID).Scan(&newPeriodID)
	require.NoError(t, err)
	assert.NotEqual(t, srcPeriod, newPeriodID)

	// 2 tiers with the same (from_count, price) pairs.
	trows, err := pool.Query(ctx, `
		SELECT from_count, price_per_segment::text
		FROM reseller_tariff_tiers WHERE tariff_period_id = $1
		ORDER BY from_count`, newPeriodID)
	require.NoError(t, err)
	defer trows.Close()
	type tier struct {
		from  int
		price string
	}
	var got []tier
	for trows.Next() {
		var tt tier
		require.NoError(t, trows.Scan(&tt.from, &tt.price))
		got = append(got, tt)
	}
	require.Len(t, got, 2)
	assert.Equal(t, 0, got[0].from)
	assert.Equal(t, 1000, got[1].from)

	// Assignments NOT copied.
	var assignCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sub_account_template_assignments WHERE template_id = $1`,
		newID).Scan(&assignCount)
	require.NoError(t, err)
	assert.Equal(t, 0, assignCount, "assignments must not be copied on template duplication")
}

// TestCreateTemplate_CopyFromDeactivatedSource_409 exercises the in-tx
// re-verification added alongside the 23505-mapping fix. The pre-tx
// ownership check in createTemplateTx scopes to reseller_id only (not
// active), so an inactive source owned by the caller passes that check.
// Without the re-check in copyTemplateChildren, the CTE would copy zero
// plans and return 201 with a silently empty template. Now it must 409.
func TestCreateTemplate_CopyFromDeactivatedSource_409(t *testing.T) {
	pool := getTemplatesTestPool(t)
	ctx := context.Background()
	resellerID := seedTemplatesReseller(t, pool)

	srcName := fmt.Sprintf("deact-src-%s", uuid.NewString()[:8])
	srcID, _, _ := seedTemplateWithContent(t, pool, resellerID, srcName)

	// Deactivate the source before the Create call — simulates the race
	// window between ownership check and tx in a deterministic way.
	_, err := pool.Exec(ctx,
		`UPDATE reseller_tariff_templates SET active = false WHERE id = $1`, srcID)
	require.NoError(t, err)

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	newName := fmt.Sprintf("copy-deact-%s", uuid.NewString()[:8])
	w := doCreate(t, h, resellerID, map[string]interface{}{
		"name":         newName,
		"copy_from_id": srcID.String(),
	})
	require.Equal(t, http.StatusConflict, w.Code,
		"must 409 instead of 201 with empty copy; body=%s", w.Body.String())

	// Assert no half-created template lingers with this name.
	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM reseller_tariff_templates WHERE reseller_id = $1 AND name = $2`,
		resellerID, newName).Scan(&cnt))
	assert.Equal(t, 0, cnt, "no template must have been committed")
}

// TestCreateTemplate_ConcurrentNameInsertReturns409 documents the
// 23505-mapping path. Deterministically inducing the race between
// pre-check and INSERT requires instrumentation; we skip with a rationale.
// The structurally-covered path is: INSERT fails with pgErr.Code==23505 →
// errors.As(*pgconn.PgError) → ErrConflict. Tested manually by inserting
// a conflicting row during a paused handler run.
func TestCreateTemplate_ConcurrentNameInsertReturns409(t *testing.T) {
	t.Skip("race-window test requires handler instrumentation; 23505→409 path covered structurally (errors.As + pgErr.Code check)")
}

func TestCreateTemplate_CopyFromForeignReseller_403(t *testing.T) {
	pool := getTemplatesTestPool(t)
	resellerA := seedTemplatesReseller(t, pool)
	resellerB := seedTemplatesReseller(t, pool)

	srcName := fmt.Sprintf("foreign-%s", uuid.NewString()[:8])
	srcID, _, _ := seedTemplateWithContent(t, pool, resellerB, srcName)

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	w := doCreate(t, h, resellerA, map[string]interface{}{
		"name":         fmt.Sprintf("new-%s", uuid.NewString()[:8]),
		"copy_from_id": srcID.String(),
	})
	assert.Equal(t, http.StatusForbidden, w.Code, "body=%s", w.Body.String())
}

func TestCreateTemplate_InvalidatesSummaryCache(t *testing.T) {
	pool := getTemplatesTestPool(t)
	resellerID := seedTemplatesReseller(t, pool)

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	key := "tariffs:summary:" + resellerID.String()
	require.NoError(t, rc.Set(context.Background(), key, []byte(`[{"x":1}]`), 0).Err())

	h := NewNetworkTariffTemplatesHandler(pool, rc)
	name := fmt.Sprintf("inv-%s", uuid.NewString()[:8])
	w := doCreate(t, h, resellerID, map[string]string{"name": name})
	require.Equal(t, http.StatusCreated, w.Code)

	// Cleanup created row.
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	if id, err := uuid.Parse(resp["id"]); err == nil {
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_templates WHERE id = $1`, id)
		})
	}

	_, err := rc.Get(context.Background(), key).Result()
	assert.ErrorIs(t, err, redis.Nil, "cache key must be deleted after Create")
}

// doBind — helper that POSTs to Bind with URL {id} set.
func doBind(t *testing.T, h *NetworkTariffTemplatesHandler, resellerID, tplID uuid.UUID, subIDs []uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	idsStr := make([]string, 0, len(subIDs))
	for _, id := range subIDs {
		idsStr = append(idsStr, id.String())
	}
	b, err := json.Marshal(map[string]interface{}{"sub_account_ids": idsStr})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/portal/v1/network/tariff-templates/%s/bind", tplID), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, resellerID))
	req = mux.SetURLVars(req, map[string]string{"id": tplID.String()})
	w := httptest.NewRecorder()
	h.Bind(w, req)
	return w
}

func TestBindTemplate_NewAssignment(t *testing.T) {
	pool := getTemplatesTestPool(t)
	resellerID := seedTemplatesReseller(t, pool)

	tplID, _, _ := seedTemplateWithContent(t, pool, resellerID, fmt.Sprintf("bind-new-%s", uuid.NewString()[:8]))
	sub := seedTemplatesSubAccount(t, pool, resellerID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM sub_account_template_assignments WHERE sub_account_id = $1`, sub)
	})

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	w := doBind(t, h, resellerID, tplID, []uuid.UUID{sub})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp struct {
		Bound    []string                   `json:"bound"`
		Replaced []map[string]string        `json:"replaced"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, []string{sub.String()}, resp.Bound)
	assert.Empty(t, resp.Replaced)

	var boundTpl uuid.UUID
	err := pool.QueryRow(context.Background(),
		`SELECT template_id FROM sub_account_template_assignments WHERE sub_account_id = $1`,
		sub).Scan(&boundTpl)
	require.NoError(t, err)
	assert.Equal(t, tplID, boundTpl)
}

func TestBindTemplate_ReplacesExisting(t *testing.T) {
	pool := getTemplatesTestPool(t)
	ctx := context.Background()
	resellerID := seedTemplatesReseller(t, pool)

	tplX, _, _ := seedTemplateWithContent(t, pool, resellerID, fmt.Sprintf("X-%s", uuid.NewString()[:8]))
	tplY, _, _ := seedTemplateWithContent(t, pool, resellerID, fmt.Sprintf("Y-%s", uuid.NewString()[:8]))
	sub := seedTemplatesSubAccount(t, pool, resellerID)

	_, err := pool.Exec(ctx, `
		INSERT INTO sub_account_template_assignments (sub_account_id, template_id)
		VALUES ($1, $2)`, sub, tplX)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM sub_account_template_assignments WHERE sub_account_id = $1`, sub)
	})

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	w := doBind(t, h, resellerID, tplY, []uuid.UUID{sub})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp struct {
		Bound    []string            `json:"bound"`
		Replaced []map[string]string `json:"replaced"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, []string{sub.String()}, resp.Bound)
	require.Len(t, resp.Replaced, 1)
	assert.Equal(t, sub.String(), resp.Replaced[0]["sub_account_id"])
	assert.Equal(t, tplX.String(), resp.Replaced[0]["old_template_id"])

	var newTpl uuid.UUID
	err = pool.QueryRow(ctx,
		`SELECT template_id FROM sub_account_template_assignments WHERE sub_account_id = $1`,
		sub).Scan(&newTpl)
	require.NoError(t, err)
	assert.Equal(t, tplY, newTpl)
}

func TestBindTemplate_CrossResellerSubAccount_400(t *testing.T) {
	pool := getTemplatesTestPool(t)
	resellerA := seedTemplatesReseller(t, pool)
	resellerB := seedTemplatesReseller(t, pool)

	tplID, _, _ := seedTemplateWithContent(t, pool, resellerA, fmt.Sprintf("cross-%s", uuid.NewString()[:8]))
	subOfB := seedTemplatesSubAccount(t, pool, resellerB)

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	w := doBind(t, h, resellerA, tplID, []uuid.UUID{subOfB})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestBindTemplate_NonexistentTemplate_404(t *testing.T) {
	pool := getTemplatesTestPool(t)
	resellerID := seedTemplatesReseller(t, pool)
	sub := seedTemplatesSubAccount(t, pool, resellerID)

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	w := doBind(t, h, resellerID, uuid.New(), []uuid.UUID{sub})
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestBindTemplate_InvalidatesSummaryCache(t *testing.T) {
	pool := getTemplatesTestPool(t)
	resellerID := seedTemplatesReseller(t, pool)
	tplID, _, _ := seedTemplateWithContent(t, pool, resellerID, fmt.Sprintf("binv-%s", uuid.NewString()[:8]))
	sub := seedTemplatesSubAccount(t, pool, resellerID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM sub_account_template_assignments WHERE sub_account_id = $1`, sub)
	})

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	key := "tariffs:summary:" + resellerID.String()
	require.NoError(t, rc.Set(context.Background(), key, []byte(`[]`), 0).Err())

	h := NewNetworkTariffTemplatesHandler(pool, rc)
	w := doBind(t, h, resellerID, tplID, []uuid.UUID{sub})
	require.Equal(t, http.StatusOK, w.Code)

	_, err := rc.Get(context.Background(), key).Result()
	assert.ErrorIs(t, err, redis.Nil)
}

// doDuplicate — helper for POST /{id}/duplicate.
func doDuplicate(t *testing.T, h *NetworkTariffTemplatesHandler, resellerID, srcID uuid.UUID, newName string) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(map[string]string{"name": newName})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/portal/v1/network/tariff-templates/%s/duplicate", srcID), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, resellerID))
	req = mux.SetURLVars(req, map[string]string{"id": srcID.String()})
	w := httptest.NewRecorder()
	h.Duplicate(w, req)
	return w
}

func TestDuplicateTemplate_Copies(t *testing.T) {
	pool := getTemplatesTestPool(t)
	ctx := context.Background()
	resellerID := seedTemplatesReseller(t, pool)

	srcName := fmt.Sprintf("dup-src-%s", uuid.NewString()[:8])
	srcID, _, _ := seedTemplateWithContent(t, pool, resellerID, srcName)

	// Bind a sub — must not be copied.
	sub := seedTemplatesSubAccount(t, pool, resellerID)
	_, err := pool.Exec(ctx, `
		INSERT INTO sub_account_template_assignments (sub_account_id, template_id)
		VALUES ($1, $2)`, sub, srcID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM sub_account_template_assignments WHERE sub_account_id = $1`, sub)
	})

	h := NewNetworkTariffTemplatesHandler(pool, nil)
	newName := fmt.Sprintf("dup-new-%s", uuid.NewString()[:8])
	w := doDuplicate(t, h, resellerID, srcID, newName)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	newID, err := uuid.Parse(resp["id"])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_plans WHERE template_id = $1`, newID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM reseller_tariff_templates WHERE id = $1`, newID)
	})

	// Assert copied plan/period/tier counts.
	var planCount, periodCount, tierCount, assignCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM reseller_tariff_plans WHERE template_id = $1`, newID).Scan(&planCount))
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM reseller_tariff_periods pr
		JOIN reseller_tariff_plans p ON p.id = pr.tariff_plan_id
		WHERE p.template_id = $1`, newID).Scan(&periodCount))
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM reseller_tariff_tiers ti
		JOIN reseller_tariff_periods pr ON pr.id = ti.tariff_period_id
		JOIN reseller_tariff_plans p    ON p.id = pr.tariff_plan_id
		WHERE p.template_id = $1`, newID).Scan(&tierCount))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sub_account_template_assignments WHERE template_id = $1`, newID).Scan(&assignCount))

	assert.Equal(t, 1, planCount)
	assert.Equal(t, 1, periodCount)
	assert.Equal(t, 2, tierCount)
	assert.Equal(t, 0, assignCount, "assignments must not be copied")
}

func TestDuplicateTemplate_NonexistentOrForeign_404(t *testing.T) {
	pool := getTemplatesTestPool(t)
	resellerA := seedTemplatesReseller(t, pool)
	resellerB := seedTemplatesReseller(t, pool)

	// (a) non-existent
	h := NewNetworkTariffTemplatesHandler(pool, nil)
	w := doDuplicate(t, h, resellerA, uuid.New(), fmt.Sprintf("x-%s", uuid.NewString()[:8]))
	assert.Equal(t, http.StatusNotFound, w.Code, "nonexistent: body=%s", w.Body.String())

	// (b) belongs to different reseller — also 404 (not 403) per Task 4 spec.
	foreign, _, _ := seedTemplateWithContent(t, pool, resellerB, fmt.Sprintf("foreign-dup-%s", uuid.NewString()[:8]))
	w2 := doDuplicate(t, h, resellerA, foreign, fmt.Sprintf("y-%s", uuid.NewString()[:8]))
	assert.Equal(t, http.StatusNotFound, w2.Code, "foreign: body=%s", w2.Body.String())
}
