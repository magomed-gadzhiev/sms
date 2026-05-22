package middleware

import (
	"net/http"
	"strings"

	"github.com/smpp-server/smpp-server/internal/config"
)

// CORSMiddleware создает middleware для CORS
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Проверяем, разрешен ли origin
			allowed := false
			if len(allowedOrigins) == 0 {
				// Если список пуст, разрешаем все
				allowed = true
			} else {
				for _, allowedOrigin := range allowedOrigins {
					if allowedOrigin == "*" || origin == allowedOrigin {
						allowed = true
						break
					}
				}
			}

			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Request-ID")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", config.DefaultCORSMaxAge)
			}

			// Обрабатываем preflight запросы
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddlewareFromConfig создает CORS middleware из конфигурации
func CORSMiddlewareFromConfig(allowedOriginsStr string) func(http.Handler) http.Handler {
	var allowedOrigins []string
	if allowedOriginsStr != "" {
		allowedOrigins = strings.Split(allowedOriginsStr, ",")
		for i := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
		}
	}
	return CORSMiddleware(allowedOrigins)
}
