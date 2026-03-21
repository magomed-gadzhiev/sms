package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
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
			// Пропускаем публичные пути
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Получаем session ID из cookie
			cookie, err := r.Cookie("portal_session")
			if err != nil || cookie.Value == "" {
				respondError(w, shared.ErrUnauthorized("Сессия не найдена"))
				return
			}

			sessionID := cookie.Value

			// Получаем данные сессии из Redis
			ctx := r.Context()
			redisKey := "session:" + sessionID

			sessionData, err := redisClient.HGetAll(ctx, redisKey).Result()
			if err != nil {
				log.Error().Err(err).Str("session_id", sessionID).Msg("ошибка получения сессии из Redis")
				respondError(w, shared.ErrInternalServer("Ошибка проверки сессии"))
				return
			}

			if len(sessionData) == 0 {
				respondError(w, shared.ErrUnauthorized("Сессия недействительна или истекла"))
				return
			}

			// Проверяем срок действия сессии
			expiresAt, ok := sessionData["expires_at"]
			if ok && expiresAt != "" {
				expTime, err := time.Parse(time.RFC3339, expiresAt)
				if err == nil && time.Now().After(expTime) {
					// Удаляем истекшую сессию
					redisClient.Del(ctx, redisKey)
					respondError(w, shared.ErrUnauthorized("Сессия истекла"))
					return
				}
			}

			// Парсим user_id
			userIDStr, ok := sessionData["user_id"]
			if !ok || userIDStr == "" {
				respondError(w, shared.ErrUnauthorized("Некорректные данные сессии"))
				return
			}
			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				log.Error().Err(err).Str("user_id", userIDStr).Msg("ошибка парсинга user_id из сессии")
				respondError(w, shared.ErrInternalServer("Ошибка обработки сессии"))
				return
			}

			// Парсим client_id
			clientIDStr, ok := sessionData["client_id"]
			if !ok || clientIDStr == "" {
				respondError(w, shared.ErrUnauthorized("Некорректные данные сессии"))
				return
			}
			clientID, err := uuid.Parse(clientIDStr)
			if err != nil {
				log.Error().Err(err).Str("client_id", clientIDStr).Msg("ошибка парсинга client_id из сессии")
				respondError(w, shared.ErrInternalServer("Ошибка обработки сессии"))
				return
			}

			// Получаем роль
			role := sessionData["role"]

			// Добавляем данные в контекст
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, ClientIDKey, clientID)
			ctx = context.WithValue(ctx, RoleKey, role)

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
