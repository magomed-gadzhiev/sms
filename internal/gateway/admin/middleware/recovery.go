package middleware

import (
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// RecoveryMiddleware создает middleware для обработки паник
func RecoveryMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error().
						Interface("error", err).
						Str("method", r.Method).
						Str("path", r.URL.Path).
						Msg("паника при обработке запроса")
					
					respondError(w, shared.ErrInternalServer("Внутренняя ошибка сервера"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
