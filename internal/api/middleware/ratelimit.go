package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// RateLimitMiddleware создает middleware для ограничения частоты запросов по Redis-окнам
func RateLimitMiddleware(redisClient *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			clientID, ok := GetClientID(ctx)
			if !ok || clientID == uuid.Nil {
				next.ServeHTTP(w, r)
				return
			}

			client, ok := GetClient(ctx)
			if !ok || client == nil {
				next.ServeHTTP(w, r)
				return
			}

			type windowDef struct {
				suffix string
				window time.Duration
				limit  int
			}

			windows := []windowDef{
				{"sec", time.Second, client.RateLimitPerSecond},
				{"min", time.Minute, client.RateLimitPerMinute},
				{"hour", time.Hour, client.RateLimitPerHour},
			}

			idStr := clientID.String()

			for _, wd := range windows {
				exceeded, err := CheckWindow(ctx, redisClient, idStr, wd.suffix, wd.window, wd.limit)
				if err != nil {
					log.Error().Err(err).Str("client_id", idStr).Str("window", wd.suffix).
						Msg("rate limit redis error, failing open")
					continue
				}
				if exceeded {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusTooManyRequests)
					json.NewEncoder(w).Encode(map[string]string{
						"error":  "rate_limit_exceeded",
						"window": wd.suffix,
					})
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CheckWindow проверяет одно временное окно rate-limit через Redis INCR + ExpireNX.
// Возвращает (exceeded, error). При ошибке Redis — fail open (не блокирует).
func CheckWindow(ctx context.Context, rdb *redis.Client, clientID, suffix string, window time.Duration, limit int) (bool, error) {
	if limit <= 0 {
		return false, nil
	}

	key := "rl:" + clientID + ":" + suffix

	pipe := rdb.Pipeline()
	incrCmd := pipe.IncrBy(ctx, key, 1)
	pipe.ExpireNX(ctx, key, window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count := incrCmd.Val()
	return count > int64(limit), nil
}

var ErrRateLimitExceeded = &RateLimitError{Message: "Превышен лимит запросов"}

type RateLimitError struct {
	Message string
}

func (e *RateLimitError) Error() string {
	return e.Message
}
