package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// RateLimitMiddleware создает middleware для ограничения частоты запросов
func RateLimitMiddleware(redisClient *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пропускаем health check и metrics endpoints
			if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			// Получаем клиента из контекста
			client, ok := GetClient(r.Context())
			if !ok {
				// Если клиент не найден, пропускаем (auth middleware должен был его установить)
				next.ServeHTTP(w, r)
				return
			}

			clientID := client.ID.String()

			// Проверяем лимиты по секундам, минутам и часам
			if err := checkRateLimit(r.Context(), redisClient, clientID, "sec", client.RateLimitPerSecond, time.Second); err != nil {
				if err == ErrRateLimitExceeded {
					w.Header().Set("X-RateLimit-Limit", strconv.Itoa(client.RateLimitPerSecond))
					w.Header().Set("X-RateLimit-Remaining", "0")
					w.Header().Set("Retry-After", "1")
					respondError(w, shared.ErrTooManyRequests("Превышен лимит запросов в секунду"))
					return
				}
				log.Error().Err(err).Msg("ошибка проверки rate limit")
			}

			if err := checkRateLimit(r.Context(), redisClient, clientID, "min", client.RateLimitPerMinute, time.Minute); err != nil {
				if err == ErrRateLimitExceeded {
					w.Header().Set("X-RateLimit-Limit", strconv.Itoa(client.RateLimitPerMinute))
					w.Header().Set("X-RateLimit-Remaining", "0")
					w.Header().Set("Retry-After", "60")
					respondError(w, shared.ErrTooManyRequests("Превышен лимит запросов в минуту"))
					return
				}
				log.Error().Err(err).Msg("ошибка проверки rate limit")
			}

			if err := checkRateLimit(r.Context(), redisClient, clientID, "hour", client.RateLimitPerHour, time.Hour); err != nil {
				if err == ErrRateLimitExceeded {
					w.Header().Set("X-RateLimit-Limit", strconv.Itoa(client.RateLimitPerHour))
					w.Header().Set("X-RateLimit-Remaining", "0")
					w.Header().Set("Retry-After", "3600")
					respondError(w, shared.ErrTooManyRequests("Превышен лимит запросов в час"))
					return
				}
				log.Error().Err(err).Msg("ошибка проверки rate limit")
			}

			next.ServeHTTP(w, r)
		})
	}
}

var ErrRateLimitExceeded = &RateLimitError{Message: "Превышен лимит запросов"}

type RateLimitError struct {
	Message string
}

func (e *RateLimitError) Error() string {
	return e.Message
}

// checkRateLimit проверяет лимит запросов для клиента
func checkRateLimit(ctx context.Context, redisClient *redis.Client, clientID, period string, limit int, window time.Duration) error {
	if limit <= 0 {
		// Лимит не установлен
		return nil
	}

	key := "rate_limit:" + clientID + ":" + period

	// Используем Redis pipeline для атомарной операции
	pipe := redisClient.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	results, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	// Получаем текущее значение счетчика
	count := results[0].(*redis.IntCmd).Val()

	if count > int64(limit) {
		return ErrRateLimitExceeded
	}

	return nil
}
