package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/rs/zerolog/log"
)

// RecoveryMiddleware создает middleware для восстановления после паник
func RecoveryMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error().
						Interface("error", err).
						Str("path", r.URL.Path).
						Str("method", r.Method).
						Bytes("stack", debug.Stack()).
						Msg("panic recovered")

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"Внутренняя ошибка сервера"}}`))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
