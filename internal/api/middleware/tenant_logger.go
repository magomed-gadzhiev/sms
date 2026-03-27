package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// TenantLoggerMiddleware создает middleware для обогащения логгера полем tenant_id.
// Должен применяться ПОСЛЕ AuthMiddleware, чтобы ClientIDKey уже был установлен в контексте.
// Downstream-код может использовать zerolog.Ctx(ctx) для получения обогащённого логгера.
func TenantLoggerMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID, ok := GetClientID(r.Context())
			if ok && clientID != uuid.Nil {
				enriched := logger.With().Str("tenant_id", clientID.String()).Logger()
				ctx := enriched.WithContext(r.Context())
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}
