package middleware

import (
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimitMiddleware создает middleware для ограничения частоты запросов через Redis
// LOAD TEST MODE: rate limiting отключен для нагрузочного тестирования
func RateLimitMiddleware(redisClient *redis.Client, path string, maxAttempts int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

// extractIP извлекает IP-адрес клиента из запроса
func extractIP(r *http.Request) string {
	// Проверяем X-Forwarded-For
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Берем первый IP из списка
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Проверяем X-Real-IP
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Используем RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
