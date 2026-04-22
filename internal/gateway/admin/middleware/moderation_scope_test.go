package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// injectAuth populates the context with the same keys AdminAuthMiddleware uses.
// This is test-only plumbing — we inject directly to avoid spinning up a real
// auth service.
func injectAuth(ctx context.Context, userID uuid.UUID, role string) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, RoleKey, &authv1.Role{Name: role})
	return ctx
}

func TestModerationScope_AdminGlobal(t *testing.T) {
	adminID := uuid.New()
	ctx := injectAuth(context.Background(), adminID, "admin")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	var captured Scope
	h := ModerationScope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = ScopeFromContext(r.Context())
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.True(t, captured.IsGlobal)
	assert.Nil(t, captured.ResellerID)
}

func TestModerationScope_AggregatorScopedToSelf(t *testing.T) {
	aggID := uuid.New()
	ctx := injectAuth(context.Background(), aggID, "aggregator_moderator")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	var captured Scope
	h := ModerationScope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = ScopeFromContext(r.Context())
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.False(t, captured.IsGlobal)
	require.NotNil(t, captured.ResellerID)
	assert.Equal(t, aggID, *captured.ResellerID)
}

func TestModerationScope_UnknownRoleEmpty(t *testing.T) {
	ctx := injectAuth(context.Background(), uuid.New(), "random_role")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	var captured Scope
	h := ModerationScope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = ScopeFromContext(r.Context())
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.False(t, captured.IsGlobal)
	assert.Nil(t, captured.ResellerID)
}

func TestModerationScope_NoAuthContext(t *testing.T) {
	// Simulates a misconfigured router where ModerationScope is called before
	// auth middleware — no user/role keys in context at all.
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	var captured Scope
	h := ModerationScope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = ScopeFromContext(r.Context())
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.False(t, captured.IsGlobal)
	assert.Nil(t, captured.ResellerID)
}

func TestScopeFromContext_MissingKey(t *testing.T) {
	// No scope injected — returns empty Scope.
	scope := ScopeFromContext(context.Background())
	assert.False(t, scope.IsGlobal)
	assert.Nil(t, scope.ResellerID)
}

func TestModerationScope_SuperadminNotGlobal(t *testing.T) {
	// "superadmin" is handled by AdminAuthMiddleware but ModerationScope only
	// maps "admin" to IsGlobal. Superadmin is not in scope here — this test
	// documents the current intentional behaviour; if superadmin needs global
	// scope, add it to the switch in ModerationScope.
	superID := uuid.New()
	ctx := injectAuth(context.Background(), superID, "superadmin")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	var captured Scope
	h := ModerationScope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = ScopeFromContext(r.Context())
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)

	// Intentionally empty — add "superadmin" to the switch if needed.
	assert.False(t, captured.IsGlobal)
	assert.Nil(t, captured.ResellerID)
}
