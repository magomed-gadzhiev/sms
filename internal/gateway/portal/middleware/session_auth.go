package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/api/http/response"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	ClientIDKey contextKey = "client_id"
	RoleKey     contextKey = "role"
)

// publicPaths — пути, которые не требуют аутентификации
var publicPaths = []string{
	"/health",
	"/health/live",
	"/health/ready",
	"/portal/v1/auth/login",
	"/portal/v1/auth/password/reset-request",
	"/portal/v1/auth/password/reset",
}

// isPublicPath проверяет, является ли путь публичным
func isPublicPath(path string) bool {
	for _, p := range publicPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// SessionAuthMiddleware создает middleware для сессионной аутентификации через Redis
func SessionAuthMiddleware(redisClient *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("portal_session")
			if err != nil || cookie.Value == "" {
				response.Error(w, shared.ErrUnauthorized("Сессия не найдена"))
				return
			}

			sessionKey := "session:" + cookie.Value
			data, err := redisClient.HGetAll(r.Context(), sessionKey).Result()
			if err != nil || len(data) == 0 {
				response.Error(w, shared.ErrUnauthorized("Сессия не найдена или истекла"))
				return
			}

			userIDStr, ok := data["user_id"]
			if !ok || userIDStr == "" {
				response.Error(w, shared.ErrUnauthorized("Некорректная сессия"))
				return
			}

			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				response.Error(w, shared.ErrUnauthorized("Некорректный user_id в сессии"))
				return
			}

			role := data["role"]
			if role == "" {
				role = "client"
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, RoleKey, role)

			// client_id может быть нулевым (для admin без клиента)
			clientIDStr := data["client_id"]
			if clientIDStr != "" {
				clientID, err := uuid.Parse(clientIDStr)
				if err == nil {
					ctx = context.WithValue(ctx, ClientIDKey, clientID)
				}
			}

			log.Debug().
				Str("user_id", userID.String()).
				Str("role", role).
				Str("client_id", clientIDStr).
				Msg("session authenticated")

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID извлекает ID пользователя из контекста
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

// GetClientID извлекает ID клиента из контекста
func GetClientID(ctx context.Context) (uuid.UUID, bool) {
	clientID, ok := ctx.Value(ClientIDKey).(uuid.UUID)
	return clientID, ok
}

// GetRole извлекает роль пользователя из контекста
func GetRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(RoleKey).(string)
	return role, ok
}

