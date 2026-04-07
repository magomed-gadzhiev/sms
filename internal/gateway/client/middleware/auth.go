package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/api/http/response"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type contextKey string

const (
	UserIDKey      contextKey = "user_id"
	UserKey        contextKey = "user"
	ClientIDKey    contextKey = "client_id"
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

// isLoadTestMode возвращает true, если переменная окружения LOAD_TEST_MODE=true
func isLoadTestMode() bool {
	return strings.EqualFold(os.Getenv("LOAD_TEST_MODE"), "true")
}

// ClientAuthMiddleware создает middleware для аутентификации клиентов.
// Если LOAD_TEST_MODE=true — использует dummy IDs (режим нагрузочного тестирования).
// Иначе — извлекает токен из заголовка Authorization (Bearer) или X-API-Key,
// и вызывает authClient.ValidateToken().
func ClientAuthMiddleware(authClient authv1.AuthServiceClient) func(http.Handler) http.Handler {
	dummyID := uuid.MustParse("c0000000-0000-0000-0000-000000000001")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пропускаем публичные эндпоинты
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			if isLoadTestMode() {
				ctx := r.Context()
				ctx = context.WithValue(ctx, UserIDKey, dummyID)
				ctx = context.WithValue(ctx, ClientIDKey, dummyID)
				ctx = context.WithValue(ctx, RoleKey, &authv1.Role{Name: "client"})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Извлекаем токен: сначала из Authorization: Bearer, затем из X-API-Key
			var token string
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				if !strings.HasPrefix(authHeader, "Bearer ") {
					response.Error(w, &shared.AppError{
						HTTPStatus: http.StatusUnauthorized,
						Code:       "INVALID_AUTH_HEADER",
						Message:    "Authorization header must use Bearer scheme",
					})
					return
				}
				token = strings.TrimPrefix(authHeader, "Bearer ")
			} else {
				token = r.Header.Get("X-API-Key")
			}

			if token == "" {
				response.Error(w, &shared.AppError{
					HTTPStatus: http.StatusUnauthorized,
					Code:       "MISSING_CREDENTIALS",
					Message:    "Authorization header or X-API-Key is required",
				})
				return
			}

			// Валидируем токен через auth service
			resp, err := authClient.ValidateToken(r.Context(), &authv1.ValidateTokenRequest{
				Token: token,
			})
			if err != nil {
				response.Error(w, &shared.AppError{
					HTTPStatus: http.StatusUnauthorized,
					Code:       "INVALID_TOKEN",
					Message:    "Invalid or expired token",
				})
				return
			}

			if !resp.Valid {
				response.Error(w, &shared.AppError{
					HTTPStatus: http.StatusUnauthorized,
					Code:       "INVALID_TOKEN",
					Message:    "Invalid or expired token",
				})
				return
			}

			// Проверяем наличие пользователя
			if resp.User == nil {
				response.Error(w, &shared.AppError{
					HTTPStatus: http.StatusUnauthorized,
					Code:       "INVALID_TOKEN",
					Message:    "No user info in token",
				})
				return
			}

			// Проверяем, что пользователь активен
			if !resp.User.Active {
				response.Error(w, &shared.AppError{
					HTTPStatus: http.StatusForbidden,
					Code:       "USER_INACTIVE",
					Message:    "User account is inactive",
				})
				return
			}

			// Проверяем роль — только client
			role := ""
			if resp.User.Role != nil {
				role = resp.User.Role.Name
			}
			if role != "client" {
				response.Error(w, &shared.AppError{
					HTTPStatus: http.StatusForbidden,
					Code:       "WRONG_ROLE",
					Message:    "Client access required",
				})
				return
			}

			// Парсим user ID
			userID, parseErr := uuid.Parse(resp.User.Id)
			if parseErr != nil {
				response.Error(w, &shared.AppError{
					HTTPStatus: http.StatusInternalServerError,
					Code:       "AUTH_ERROR",
					Message:    "Invalid user data from auth service",
				})
				return
			}

			// Используем company client_id если доступен, иначе user_id
			clientIDValue := userID
			if resp.User.ClientId != "" {
				if parsed, err := uuid.Parse(resp.User.ClientId); err == nil {
					clientIDValue = parsed
				}
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, ClientIDKey, clientIDValue)
			ctx = context.WithValue(ctx, UserKey, resp.User)
			if resp.User.Role != nil {
				ctx = context.WithValue(ctx, RoleKey, resp.User.Role)
			}

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

// GetClientID извлекает ID клиента из контекста
func GetClientID(ctx context.Context) (uuid.UUID, bool) {
	clientID, ok := ctx.Value(ClientIDKey).(uuid.UUID)
	if !ok {
		// Попробуем получить из UserIDKey
		userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
		return userID, ok
	}
	return clientID, ok
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

