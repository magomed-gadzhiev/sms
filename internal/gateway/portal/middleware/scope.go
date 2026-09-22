package middleware

import (
	"context"
	"net/http"
	"slices"

	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/shared/response"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// APIKeyScopesKey помечает контекст списком scope'ов API-ключа.
const APIKeyScopesKey contextKey = "api_key_scopes"

// GetAPIKeyScopes возвращает scope'ы API-ключа из контекста.
// Для session-auth контекст не содержит scope'ов — возвращается nil, false.
func GetAPIKeyScopes(ctx context.Context) ([]string, bool) {
	v, ok := ctx.Value(APIKeyScopesKey).([]string)
	return v, ok
}

// RequireScopeByMethod возвращает middleware, который для API-key запросов проверяет
// наличие scope'а: GET → readScope, любой другой метод → writeScope.
//
// Для session-auth middleware пропускает запрос (cookie-аутентификация не подчиняется
// scope-ограничениям; они есть только у API-ключей).
//
// Если auth-method ещё не выставлен (middleware применён до auth-цепочки) — пропуск,
// чтобы не ломать публичные роуты.
//
// Scope'ы читаются из контекста (APIKeyScopesKey), куда их кладёт APIKeyAuthMiddleware
// после успешного ValidateToken — никаких дополнительных round-trip'ов.
//
// Контракт ошибок:
//   - в контексте отметка api_key, но scope'ы отсутствуют → 401 (ключ инвалидирован
//     между auth и scope-проверкой; не должно случаться, но safe-default).
//   - scope'ы есть, требуемого нет → 403.
func RequireScopeByMethod(readScope, writeScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if GetAuthMethod(r.Context()) != AuthMethodAPIKey {
				next.ServeHTTP(w, r)
				return
			}

			// GET/HEAD/OPTIONS — read-side по HTTP-семантике (RFC 9110: safe + idempotent
			// без side-effects). Остальное — write-side. CORS-preflight (OPTIONS) обычно
			// не доходит до auth-цепочки, но дешевле обрабатывать корректно.
			required := writeScope
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				required = readScope
			}

			scopes, ok := GetAPIKeyScopes(r.Context())
			if !ok {
				response.Error(w, shared.ErrUnauthorized("API-ключ не аутентифицирован"))
				return
			}
			if !slices.Contains(scopes, required) {
				log.Debug().
					Str("required_scope", required).
					Strs("key_scopes", scopes).
					Str("method", r.Method).
					Msg("API key denied: missing required scope")
				response.Error(w, shared.ErrForbidden("Требуется scope '"+required+"'"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
