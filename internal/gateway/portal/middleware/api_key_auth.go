package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/api/http/response"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AuthMethodKey помечает контекст методом аутентификации ("session" | "api_key").
const AuthMethodKey contextKey = "auth_method"

// APIKeyIDKey содержит префикс/идентификатор API-ключа (для audit log).
const APIKeyIDKey contextKey = "api_key_id"

const (
	AuthMethodSession = "session"
	AuthMethodAPIKey  = "api_key"
)

// GetAuthMethod возвращает метод аутентификации текущего запроса.
// "session" для cookie-based auth, "api_key" — для Bearer token.
func GetAuthMethod(ctx context.Context) string {
	if m, ok := ctx.Value(AuthMethodKey).(string); ok {
		return m
	}
	return ""
}

// GetAPIKeyPrefix возвращает префикс API-ключа (для audit).
func GetAPIKeyPrefix(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(APIKeyIDKey).(string)
	return v, ok
}

// APIKeyAuthMiddleware — middleware аутентификации через Authorization: Bearer <api_key>.
// Делегирует проверку authv1.ValidateToken (он принимает и JWT, и sk_live_* ключи).
//
// Возвращает middleware; если authClient == nil — middleware возвращает 503 на любых запросах
// (это conservative-fallback: API-key auth отключён, но session auth всё ещё работает через fallback).
func APIKeyAuthMiddleware(authClient authv1.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				response.Error(w, shared.ErrUnauthorized("Требуется Authorization: Bearer <api-key>"))
				return
			}
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				response.Error(w, shared.ErrUnauthorized("Пустой API-ключ"))
				return
			}

			if authClient == nil {
				response.Error(w, shared.ErrUnauthorized("API-key auth недоступна"))
				return
			}

			resp, err := authClient.ValidateToken(r.Context(), &authv1.ValidateTokenRequest{Token: token})
			if err != nil || resp == nil || !resp.Valid || resp.User == nil {
				response.Error(w, shared.ErrUnauthorized("Недействительный API-ключ"))
				return
			}
			if !resp.User.Active {
				response.Error(w, &shared.AppError{
					HTTPStatus: http.StatusForbidden,
					Code:       "USER_INACTIVE",
					Message:    "Пользователь деактивирован",
				})
				return
			}

			userID, err := uuid.Parse(resp.User.Id)
			if err != nil {
				response.Error(w, shared.ErrUnauthorized("Некорректный user_id от auth service"))
				return
			}

			// client_id: предпочитаем company client_id, иначе user_id.
			clientID := userID
			if resp.User.ClientId != "" {
				if parsed, err := uuid.Parse(resp.User.ClientId); err == nil {
					clientID = parsed
				}
			}

			role := "client"
			if resp.User.Role != nil && resp.User.Role.Name != "" {
				role = resp.User.Role.Name
			}

			// Префикс ключа (первые 12 символов) — для audit log. Полный ключ не логируем.
			keyPrefix := token
			if len(keyPrefix) > 12 {
				keyPrefix = keyPrefix[:12]
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, ClientIDKey, clientID)
			ctx = context.WithValue(ctx, RoleKey, role)
			ctx = context.WithValue(ctx, AuthMethodKey, AuthMethodAPIKey)
			ctx = context.WithValue(ctx, APIKeyIDKey, keyPrefix)
			// Scopes приходят из auth-service (ValidateTokenResponse.scopes).
			// RequireScopeByMethod читает их отсюда — без второго round-trip'а.
			ctx = context.WithValue(ctx, APIKeyScopesKey, resp.Scopes)

			log.Debug().
				Str("user_id", userID.String()).
				Str("client_id", clientID.String()).
				Str("role", role).
				Str("api_key_prefix", keyPrefix).
				Msg("API key authenticated")

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionOrAPIKeyMiddleware — композитный middleware: если есть cookie portal_session,
// маршрутизирует в session auth; иначе — в API-key auth (Authorization: Bearer).
//
// Контракт: после успеха в контексте установлены UserIDKey, ClientIDKey, RoleKey.
// AuthMethodKey проставляется в "session" или "api_key" соответственно.
//
// Оба sub-middleware — обычные http.Handler, поэтому CSRF middleware, навешанный ПОСЛЕ
// этого, должен сам проверить GetAuthMethod и пропустить API-key запросы (cookie-less =>
// нет CSRF-вектора).
func SessionOrAPIKeyMiddleware(
	redisClient *redis.Client,
	authClient authv1.AuthServiceClient,
) func(http.Handler) http.Handler {
	sessionMw := SessionAuthMiddleware(redisClient)
	apiKeyMw := APIKeyAuthMiddleware(authClient)

	// Тонкая обёртка, которая добавляет session-flag к context после session auth.
	sessionWithFlag := func(next http.Handler) http.Handler {
		return sessionMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), AuthMethodKey, AuthMethodSession)
			next.ServeHTTP(w, r.WithContext(ctx))
		}))
	}

	return func(next http.Handler) http.Handler {
		sessionChain := sessionWithFlag(next)
		apiKeyChain := apiKeyMw(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Session cookie присутствует и не пустой => session path.
			if cookie, err := r.Cookie("portal_session"); err == nil && cookie.Value != "" {
				sessionChain.ServeHTTP(w, r)
				return
			}
			// Иначе — пробуем API key.
			apiKeyChain.ServeHTTP(w, r)
		})
	}
}
