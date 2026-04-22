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
	"github.com/jackc/pgx/v5/pgxpool"
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

	h := NewNetworkTariffTemplatesHandler(pool)
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

	h := NewNetworkTariffTemplatesHandler(pool)
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

	h := NewNetworkTariffTemplatesHandler(pool)
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

	h := NewNetworkTariffTemplatesHandler(pool)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/network/tariff-templates", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, clientID))
	w := httptest.NewRecorder()
	h.List(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListTemplates_Unauthorized(t *testing.T) {
	pool := getTemplatesTestPool(t)

	h := NewNetworkTariffTemplatesHandler(pool)
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

	h := NewNetworkTariffTemplatesHandler(pool)
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
