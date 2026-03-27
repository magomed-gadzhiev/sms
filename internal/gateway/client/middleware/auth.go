package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
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

// isLoadTestMode возвращает true, если переменная окружения LOAD_TEST_MODE=true
func isLoadTestMode() bool {
	return strings.EqualFold(os.Getenv("LOAD_TEST_MODE"), "true")
}

// ClientAuthMiddleware создает middleware для аутентификации клиентов.
// Если LOAD_TEST_MODE=true — использует dummy IDs (режим нагрузочного тестирования).
// Иначе — извлекает API ключ из заголовка X-API-Key и вызывает authClient.Authenticate().
func ClientAuthMiddleware(authClient authv1.AuthServiceClient) func(http.Handler) http.Handler {
	dummyID := uuid.MustParse("c0000000-0000-0000-0000-000000000001")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isLoadTestMode() {
				ctx := r.Context()
				ctx = context.WithValue(ctx, UserIDKey, dummyID)
				ctx = context.WithValue(ctx, ClientIDKey, dummyID)
				ctx = context.WithValue(ctx, RoleKey, &authv1.Role{Name: "client"})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				respondError(w, &shared.AppError{
					HTTPStatus: http.StatusUnauthorized,
					Code:       "MISSING_API_KEY",
					Message:    "API key is required",
				})
				return
			}

			resp, err := authClient.Authenticate(r.Context(), &authv1.AuthenticateRequest{
				ApiKey: apiKey,
			})
			if err != nil {
				respondError(w, &shared.AppError{
					HTTPStatus: http.StatusUnauthorized,
					Code:       "INVALID_API_KEY",
					Message:    "Invalid or expired API key",
				})
				return
			}

			ctx := r.Context()
			if resp.User != nil {
				userID, parseErr := uuid.Parse(resp.User.Id)
				if parseErr == nil {
					ctx = context.WithValue(ctx, UserIDKey, userID)
					// GetClientID() falls back to UserIDKey if ClientIDKey is not set
				}
				ctx = context.WithValue(ctx, UserKey, resp.User)
				if resp.User.Role != nil {
					ctx = context.WithValue(ctx, RoleKey, resp.User.Role)
				}
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
