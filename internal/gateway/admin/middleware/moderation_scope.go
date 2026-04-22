package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// Scope represents what rows the current caller may moderate.
type Scope struct {
	IsGlobal   bool       // admin (and compatible roles) sees everything
	ResellerID *uuid.UUID // aggregator_moderator: scope to clients where reseller_id = this
}

// scopeCtxKey is the private context key for Scope.
type scopeCtxKey struct{}

// ScopeFromContext retrieves the Scope injected by ModerationScope middleware.
// Returns an empty Scope (neither global nor scoped) if not present; handlers
// should treat an empty Scope as unauthorized.
func ScopeFromContext(ctx context.Context) Scope {
	if s, ok := ctx.Value(scopeCtxKey{}).(Scope); ok {
		return s
	}
	return Scope{}
}

// ModerationScope must be installed AFTER the auth middleware so that user/role
// claims are already present in the context.
//
//   - "admin", "superadmin" → Scope{IsGlobal: true}
//   - "aggregator_moderator" → Scope{ResellerID: &<their user ID>}
//   - any other role → empty Scope (handlers must reject)
func ModerationScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, role := extractScopeAuth(r.Context())
		var scope Scope
		switch role {
		case "admin", "superadmin":
			scope = Scope{IsGlobal: true}
		case "aggregator_moderator":
			id := userID
			scope = Scope{ResellerID: &id}
		}
		ctx := context.WithValue(r.Context(), scopeCtxKey{}, scope)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractScopeAuth is a thin helper that reads the already-injected auth claims
// from the context (set by AdminAuthMiddleware). It calls the package-level
// GetUserID and GetRole helpers — no duplication of context key plumbing.
func extractScopeAuth(ctx context.Context) (uuid.UUID, string) {
	userID, _ := GetUserID(ctx)
	roleObj, _ := GetRole(ctx)
	if roleObj == nil {
		return userID, ""
	}
	return userID, roleObj.Name
}

// RequireScope returns the scope from context, panicking if it is neither
// global nor reseller-scoped. Use at the start of a handler that relies on
// ModerationScope being wired: a misconfigured router (missing middleware)
// will surface as a loud dev-time panic instead of silent data exposure.
func RequireScope(ctx context.Context) Scope {
	s := ScopeFromContext(ctx)
	if !s.IsGlobal && s.ResellerID == nil {
		panic("moderation scope middleware not wired: empty Scope — check router")
	}
	return s
}
