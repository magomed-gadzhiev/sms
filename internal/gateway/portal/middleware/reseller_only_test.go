//go:build integration

package middleware

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
)

func resellerOnlyTestPool(t *testing.T) *pgxpool.Pool {
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

// TestResellerOnlyMiddleware_NonReseller_Returns403 пишет sub-account в БД и
// проверяет, что middleware возвращает 403 (а не 401, как до C.5). 401 на
// /reseller/* приводил к редирект-циклу для уже залогиненного юзера в
// portal-frontend/src/api/client.ts:37.
func TestResellerOnlyMiddleware_NonReseller_Returns403(t *testing.T) {
	pool := resellerOnlyTestPool(t)
	ctx := context.Background()

	clientID := uuid.New()
	var planID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planID))

	_, err := pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, false, $5)`,
		clientID,
		fmt.Sprintf("non-reseller-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-nr-%s", clientID),
		fmt.Sprintf("nr-%s@t.local", uuid.NewString()[:8]),
		planID,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, clientID) })

	mw := ResellerOnlyMiddleware(pool)
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	req := httptest.NewRequest(http.MethodGet, "/portal/v1/reseller/dashboard", nil)
	req = req.WithContext(context.WithValue(req.Context(), ClientIDKey, clientID))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "non-reseller must get 403, not 401 (frontend redirects on 401)")
	assert.False(t, called, "next handler must not be invoked")

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Contains(t, fmt.Sprintf("%v", body), "доступ только для агрегаторов")
}

// TestResellerOnlyMiddleware_NoClientID_Returns401 — отсутствие client_id в
// контексте означает реально не аутентифицированный запрос; 401 здесь верен.
func TestResellerOnlyMiddleware_NoClientID_Returns401(t *testing.T) {
	pool := resellerOnlyTestPool(t)
	mw := ResellerOnlyMiddleware(pool)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("unreachable") }))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/portal/v1/reseller/dashboard", nil))

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestResellerOnlyMiddleware_Reseller_Passthrough — позитивный путь.
func TestResellerOnlyMiddleware_Reseller_Passthrough(t *testing.T) {
	pool := resellerOnlyTestPool(t)
	ctx := context.Background()

	clientID := uuid.New()
	var planID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planID))

	_, err := pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, true, $5)`,
		clientID,
		fmt.Sprintf("reseller-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-r-%s", clientID),
		fmt.Sprintf("r-%s@t.local", uuid.NewString()[:8]),
		planID,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, clientID) })

	mw := ResellerOnlyMiddleware(pool)
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest(http.MethodGet, "/portal/v1/reseller/dashboard", nil)
	req = req.WithContext(context.WithValue(req.Context(), ClientIDKey, clientID))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestResellerOnly_AppliedToSubAccountsRouterPattern — гарантирует, что
// ResellerOnlyMiddleware можно навесить на subrouter в стиле /sub-accounts/*
// и он действительно блокирует non-reseller'ов. Регрессионный тест для Plan 3
// Task 3 (defence-in-depth: до этого защита /sub-accounts/* шла только через
// per-handler verifyOwnership).
func TestResellerOnly_AppliedToSubAccountsRouterPattern(t *testing.T) {
	pool := resellerOnlyTestPool(t)
	ctx := context.Background()

	// Обычный клиент (не reseller) — мirror seeding pattern из
	// TestResellerOnlyMiddleware_NonReseller_Returns403.
	clientID := uuid.New()
	var planID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planID))

	_, err := pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, false, $5)`,
		clientID,
		fmt.Sprintf("non-reseller-sub-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-nrs-%s", clientID),
		fmt.Sprintf("nrs-%s@t.local", uuid.NewString()[:8]),
		planID,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, clientID) })

	r := mux.NewRouter()
	subRouter := r.PathPrefix("/sub-accounts").Subrouter()
	subRouter.Use(ResellerOnlyMiddleware(pool))
	called := false
	subRouter.HandleFunc("", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	req := httptest.NewRequest(http.MethodGet, "/sub-accounts", nil)
	req = req.WithContext(context.WithValue(req.Context(), ClientIDKey, clientID))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusForbidden, rr.Code, "non-reseller must be 403 on /sub-accounts subrouter")
	assert.False(t, called, "next handler must not be invoked for non-reseller")
}
