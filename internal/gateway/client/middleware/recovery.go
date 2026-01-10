package middleware

import (
	"encoding/json"
	"net/http"

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
						Str("method", r.Method).
						Str("path", r.URL.Path).
						Msg("паника в HTTP handler")

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"code":    "INTERNAL_ERROR",
							"message": "Внутренняя ошибка сервера",
						},
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
