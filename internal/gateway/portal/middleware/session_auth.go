package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
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
// LOAD TEST MODE: сессионная авторизация отключена для нагрузочного тестирования
func SessionAuthMiddleware(redisClient *redis.Client) func(http.Handler) http.Handler {
	dummyID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, dummyID)
			ctx = context.WithValue(ctx, ClientIDKey, dummyID)
			ctx = context.WithValue(ctx, RoleKey, "client")
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

// respondError отправляет ошибку в формате JSON
func respondError(w http.ResponseWriter, err *shared.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		},
	}

	if err.Details != "" {
		response["error"].(map[string]interface{})["details"] = err.Details
	}

	json.NewEncoder(w).Encode(response)
}
