package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// RateLimitMiddleware создает middleware для ограничения частоты запросов через Redis
func RateLimitMiddleware(redisClient *redis.Client, path string, maxAttempts int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Определяем идентификатор клиента (IP-адрес)
			identifier := extractIP(r)

			ctx := r.Context()
			key := fmt.Sprintf("rate_limit:%s:%s", path, identifier)

			// Увеличиваем счетчик
			count, err := redisClient.Incr(ctx, key).Result()
			if err != nil {
				log.Error().Err(err).Str("key", key).Msg("ошибка увеличения счетчика rate limit")
				// При ошибке Redis пропускаем запрос
				next.ServeHTTP(w, r)
				return
			}

			// Устанавливаем TTL при первом запросе
			if count == 1 {
				redisClient.Expire(ctx, key, window)
			}

			// Проверяем лимит
			if count > int64(maxAttempts) {
				// Получаем оставшееся время TTL
				ttl, err := redisClient.TTL(ctx, key).Result()
				if err != nil || ttl < 0 {
					ttl = window
				}

				retryAfter := int(ttl.Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}

				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				respondError(w, shared.ErrTooManyRequests("Превышен лимит запросов. Повторите через "+strconv.Itoa(retryAfter)+" сек."))
				return
			}

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
