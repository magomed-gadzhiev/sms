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
//   - "admin" → Scope{IsGlobal: true}
//   - "aggregator_moderator" → Scope{ResellerID: &<their user ID>}
//   - any other role → empty Scope (handlers must reject)
func ModerationScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, role := extractScopeAuth(r.Context())
		var scope Scope
		switch role {
		case "admin":
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
