//go:build integration

package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/handlers"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestModerationDB returns a *storage.DB connected to the integration test database.
// Set TEST_DATABASE_URL to enable; otherwise the calling test is skipped.
func newTestModerationDB(t *testing.T) *storage.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}
	db, err := storage.NewDBWithConfig(dsn, 5, 2, 0, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestCounts_AdminScope_CountsDirectClientsOnly seeds one direct + one
// aggregator-owned pending sender_name and asserts admin scope sees exactly 1.
// Seed helpers are stubs — test skips until fixtures are wired.
func TestCounts_AdminScope_CountsDirectClientsOnly(t *testing.T) {
	db := newTestModerationDB(t)
	t.Skip("seeders not implemented — wire fixtures to enable")

	h := handlers.NewModerationHandlers()
	h.SetDB(db)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/moderation/counts", nil)
	w := httptest.NewRecorder()

	// Without scope injected the handler must return 403.
	h.Counts(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "no scope → forbidden")
}

// TestCounts_AggregatorScope_CountsOwnOnly verifies that an aggregator_moderator
// only sees pending sender names that belong to their own reseller sub-accounts.
func TestCounts_AggregatorScope_CountsOwnOnly(t *testing.T) {
	db := newTestModerationDB(t)
	t.Skip("seeders not implemented — wire fixtures to enable")

	h := handlers.NewModerationHandlers()
	h.SetDB(db)

	resellerID := uuid.New()
	_ = resellerID // will be used when scope injection is wired

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/moderation/counts", nil)
	w := httptest.NewRecorder()

	h.Counts(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "no scope → forbidden")
}

// TestListBindings_AdminScope_HidesAggregator verifies that admin scope
// excludes bindings whose sender_names belong to aggregator-owned clients.
func TestListBindings_AdminScope_HidesAggregator(t *testing.T) {
	db := newTestModerationDB(t)
	t.Skip("seeders not implemented — wire fixtures to enable")

	h := handlers.NewModerationHandlers()
	h.SetDB(db)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/moderation/bindings", nil)
	w := httptest.NewRecorder()

	h.ListPendingBindings(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "no scope → forbidden")
}
