//go:build integration

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
)

// TestResolvePartnerID_Existing checks that resolvePartnerID returns the
// sequence-backed partner_id of the authenticated client (not 0, not the
// `?partner_id=` query parameter — the latter is now ignored).
func TestResolvePartnerID_Existing(t *testing.T) {
	pool := getTestPool(t)
	ctx := context.Background()

	clientID := uuid.New()
	var planID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5)`,
		clientID,
		fmt.Sprintf("partner-test-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-%s", clientID),
		fmt.Sprintf("%s@t.local", uuid.NewString()[:8]),
		planID,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, clientID) })

	var expected int64
	err = pool.QueryRow(ctx, `SELECT partner_id FROM clients WHERE id = $1`, clientID).Scan(&expected)
	require.NoError(t, err)
	require.NotZero(t, expected, "sequence default should have assigned a non-zero partner_id")

	h := NewNetworkStatisticsHandlers(nil, pool)
	rec := httptest.NewRecorder()

	got, ok := h.resolvePartnerID(ctx, rec, clientID)
	require.True(t, ok, "resolvePartnerID should succeed for existing client")
	assert.Equal(t, expected, got, "should return the sequence-assigned partner_id")
}

// TestResolvePartnerID_IgnoresQueryParam checks that even if a request smuggles
// `?partner_id=999` it never affects the resolved partner_id (TC-AGG-5 / B.1).
func TestResolvePartnerID_IgnoresQueryParam(t *testing.T) {
	pool := getTestPool(t)
	ctx := context.Background()

	clientID := uuid.New()
	var planID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planID))

	_, err := pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5)`,
		clientID,
		fmt.Sprintf("partner-qparam-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-q-%s", clientID),
		fmt.Sprintf("q-%s@t.local", uuid.NewString()[:8]),
		planID,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, clientID) })

	var expected int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT partner_id FROM clients WHERE id = $1`, clientID).Scan(&expected))

	// Request smuggles a foreign partner_id in the query string. resolvePartnerID
	// receives only ctx + clientID and must not consult the URL — query-param is
	// dead by design.
	req := httptest.NewRequest(http.MethodGet, "/network/statistics?partner_id=999999", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, clientID))
	rec := httptest.NewRecorder()

	h := NewNetworkStatisticsHandlers(nil, pool)
	got, ok := h.resolvePartnerID(req.Context(), rec, clientID)
	require.True(t, ok)
	assert.Equal(t, expected, got, "query-param ?partner_id=999999 must be ignored")
	assert.NotEqual(t, int64(999999), got)
}

// TestResolvePartnerID_SubAccountResolvesToParent checks that a sub-account
// (parent_client_id IS NOT NULL) resolves to its parent's partner_id, so
// reseller analytics aggregate the whole tree under one bucket.
func TestResolvePartnerID_SubAccountResolvesToParent(t *testing.T) {
	pool := getTestPool(t)
	ctx := context.Background()

	parentID := uuid.New()
	subID := uuid.New()
	var planID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`).Scan(&planID))

	_, err := pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, true, $5)`,
		parentID,
		fmt.Sprintf("parent-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-parent-%s", parentID),
		fmt.Sprintf("p-%s@t.local", uuid.NewString()[:8]),
		planID,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM clients WHERE id IN ($1, $2)`, subID, parentID) })

	_, err = pool.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, email, active, parent_client_id, plan_id)
		VALUES ($1, $2, $3, 'secret', $4, true, $5, $6)`,
		subID,
		fmt.Sprintf("sub-%s", uuid.NewString()[:8]),
		fmt.Sprintf("apikey-sub-%s", subID),
		fmt.Sprintf("s-%s@t.local", uuid.NewString()[:8]),
		parentID,
		planID,
	)
	require.NoError(t, err)

	var parentPartnerID, subOwnPartnerID int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT partner_id FROM clients WHERE id = $1`, parentID).Scan(&parentPartnerID))
	require.NoError(t, pool.QueryRow(ctx, `SELECT partner_id FROM clients WHERE id = $1`, subID).Scan(&subOwnPartnerID))
	require.NotEqual(t, parentPartnerID, subOwnPartnerID, "sub-account must have its own UNIQUE partner_id at the row level")

	h := NewNetworkStatisticsHandlers(nil, pool)
	rec := httptest.NewRecorder()

	got, ok := h.resolvePartnerID(ctx, rec, subID)
	require.True(t, ok)
	assert.Equal(t, parentPartnerID, got, "sub-account should resolve to parent's partner_id")
	assert.NotEqual(t, subOwnPartnerID, got, "must NOT return sub-account's own partner_id")
}

// TestResolvePartnerID_NotFound checks 401 path when clientID has no DB row.
func TestResolvePartnerID_NotFound(t *testing.T) {
	pool := getTestPool(t)

	h := NewNetworkStatisticsHandlers(nil, pool)
	rec := httptest.NewRecorder()

	got, ok := h.resolvePartnerID(context.Background(), rec, uuid.New())
	assert.False(t, ok)
	assert.Equal(t, int64(0), got)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, fmt.Sprintf("%v", body), "не найден")
}
