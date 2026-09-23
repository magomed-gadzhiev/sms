package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/shared/response"
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

// resolveUser аутентифицирует запрос через Bearer JWT или portal_session cookie.
// Возвращает UserInfo при успехе, иначе nil.
func resolveUser(r *http.Request, authClient authv1.AuthServiceClient) *authv1.UserInfo {
	// 1. Bearer JWT
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != "" {
			resp, err := authClient.ValidateToken(r.Context(), &authv1.ValidateTokenRequest{Token: token})
			if err == nil && resp.Valid && resp.User != nil {
				return resp.User
			}
		}
		return nil
	}

	// 2. Cookie portal_session
	cookie, err := r.Cookie("portal_session")
	if err != nil || cookie.Value == "" {
		return nil
	}
	resp, err := authClient.ValidateSession(r.Context(), &authv1.ValidateSessionRequest{SessionId: cookie.Value})
	if err != nil || !resp.Valid || resp.User == nil {
		return nil
	}
	return resp.User
}

// AdminAuthMiddleware создает middleware для аутентификации администраторов.
// Поддерживает Bearer JWT (Authorization header) и portal_session cookie.
// Допускает только пользователей с ролью admin или superadmin.
func AdminAuthMiddleware(authClient authv1.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пропускаем публичные эндпоинты
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			user := resolveUser(r, authClient)
			if user == nil {
				response.Error(w, shared.ErrUnauthorized("Authentication required"))
				return
			}

			// Проверяем, что пользователь активен
			if !user.Active {
				response.Error(w, shared.ErrForbidden("User account is inactive"))
				return
			}

			// Проверяем роль — только admin и superadmin
			role := ""
			if user.Role != nil {
				role = user.Role.Name
			}
			if role != "admin" && role != "superadmin" {
				response.Error(w, shared.ErrForbidden("Admin access required"))
				return
			}

			// Парсим user ID
			userID, err := uuid.Parse(user.Id)
			if err != nil {
				response.Error(w, shared.ErrInternalServer("Invalid user ID"))
				return
			}

			// Устанавливаем контекст
			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, UserKey, user)
			ctx = context.WithValue(ctx, RoleKey, &authv1.Role{Name: role})

			// Загружаем permissions если доступны
			permResp, permErr := authClient.GetPermissions(r.Context(), &authv1.GetPermissionsRequest{
				UserId: user.Id,
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

