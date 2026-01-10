package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UserKey     contextKey = "user"
	RoleKey     contextKey = "role"
	PermissionsKey contextKey = "permissions"
)

// AdminAuthMiddleware создает middleware для аутентификации администраторов
func AdminAuthMiddleware(authClient authv1.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пропускаем health check и metrics endpoints
			if strings.HasPrefix(r.URL.Path, "/health") || 
			   strings.HasPrefix(r.URL.Path, "/metrics") {
				next.ServeHTTP(w, r)
				return
			}

			// Получаем токен из заголовка
			var token string
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					token = parts[1]
				}
			}

			if token == "" {
				respondError(w, shared.ErrUnauthorized("Токен авторизации не предоставлен"))
				return
			}

			// Валидируем токен через Auth Service
			ctx := r.Context()
			validateResp, err := authClient.ValidateToken(ctx, &authv1.ValidateTokenRequest{
				Token: token,
			})
			if err != nil {
				log.Error().Err(err).Msg("ошибка валидации токена")
				respondError(w, shared.ErrUnauthorized("Неверный токен авторизации"))
				return
			}

			if !validateResp.Valid {
				respondError(w, shared.ErrUnauthorized("Токен невалиден или истек"))
				return
			}

			user := validateResp.User
			if user == nil {
				respondError(w, shared.ErrUnauthorized("Информация о пользователе не найдена"))
				return
			}

			// Проверяем, что пользователь активен
			if !user.Active {
				respondError(w, shared.ErrForbidden("Пользователь неактивен"))
				return
			}

			// Проверяем, что это админ (для admin gateway требуется роль admin)
			if user.Role == nil || user.Role.Name != "admin" {
				respondError(w, shared.ErrForbidden("Доступ запрещен: требуется роль администратора"))
				return
			}

			// Получаем права доступа
			permsResp, err := authClient.GetPermissions(ctx, &authv1.GetPermissionsRequest{
				UserId: user.Id,
			})
			if err != nil {
				log.Warn().Err(err).Msg("ошибка получения прав доступа")
				// Продолжаем без прав, но это не критично для базовой проверки
			}

			// Добавляем информацию о пользователе в контекст
			userID, err := uuid.Parse(user.Id)
			if err != nil {
				log.Error().Err(err).Str("user_id", user.Id).Msg("ошибка парсинга user_id")
				respondError(w, shared.ErrInternalServer("Ошибка обработки пользователя"))
				return
			}

			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, UserKey, user)
			ctx = context.WithValue(ctx, RoleKey, user.Role)
			if permsResp != nil {
				ctx = context.WithValue(ctx, PermissionsKey, permsResp.Permissions)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID извлекает ID пользователя из контекста
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

// GetUser извлекает информацию о пользователе из контекста
func GetUser(ctx context.Context) (*authv1.UserInfo, bool) {
	user, ok := ctx.Value(UserKey).(*authv1.UserInfo)
	return user, ok
}

// GetRole извлекает роль пользователя из контекста
func GetRole(ctx context.Context) (*authv1.Role, bool) {
	role, ok := ctx.Value(RoleKey).(*authv1.Role)
	return role, ok
}

// HasPermission проверяет, есть ли у пользователя указанное право
func HasPermission(ctx context.Context, resource, action string) bool {
	permissions, ok := ctx.Value(PermissionsKey).([]*authv1.Permission)
	if !ok || permissions == nil {
		return false
	}

	for _, perm := range permissions {
		if perm.Resource == resource && perm.Action == action {
			return true
		}
	}

	return false
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
