package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type contextKey string

const (
	UserIDKey      contextKey = "user_id"
	UserKey        contextKey = "user"
	RoleKey        contextKey = "role"
	PermissionsKey contextKey = "permissions"
)

// publicPaths — пути, которые не требуют аутентификации
var publicPaths = []string{
	"/health",
	"/metrics",
}

func isPublicPath(path string) bool {
	for _, p := range publicPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// AdminAuthMiddleware создает middleware для аутентификации администраторов.
// Проверяет session cookie (portal_session) через auth service.
func AdminAuthMiddleware(authClient authv1.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пропускаем публичные эндпоинты
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Извлекаем session cookie
			cookie, err := r.Cookie("portal_session")
			if err != nil || cookie.Value == "" {
				respondError(w, shared.ErrUnauthorized("Session cookie required"))
				return
			}

			// Валидируем сессию через auth service
			resp, err := authClient.ValidateSession(r.Context(), &authv1.ValidateSessionRequest{
				SessionId: cookie.Value,
			})
			if err != nil || !resp.Valid {
				respondError(w, shared.ErrUnauthorized("Invalid or expired session"))
				return
			}

			// Проверяем роль — только admin и superadmin
			role := ""
			if resp.User != nil && resp.User.Role != nil {
				role = resp.User.Role.Name
			}
			if role != "admin" && role != "superadmin" {
				respondError(w, shared.ErrForbidden("Admin access required"))
				return
			}

			// Парсим user ID
			userID, err := uuid.Parse(resp.User.Id)
			if err != nil {
				respondError(w, shared.ErrInternalServer("Invalid user ID"))
				return
			}

			// Устанавливаем контекст
			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, UserKey, resp.User)
			ctx = context.WithValue(ctx, RoleKey, &authv1.Role{Name: role})

			// Загружаем permissions если доступны
			permResp, permErr := authClient.GetPermissions(r.Context(), &authv1.GetPermissionsRequest{
				UserId: resp.User.Id,
			})
			if permErr == nil && permResp.Permissions != nil {
				ctx = context.WithValue(ctx, PermissionsKey, permResp.Permissions)
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
