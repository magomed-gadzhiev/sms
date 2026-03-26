package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimitMiddleware создает middleware для ограничения частоты запросов
// LOAD TEST MODE: rate limiting отключен для нагрузочного тестирования
func RateLimitMiddleware(redisClient *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
