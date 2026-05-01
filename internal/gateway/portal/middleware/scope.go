package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/api/http/response"
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

// ScopeLoader загружает scope'ы по сырому Bearer-токену.
// Возвращает (scopes, found, err):
//   - found=false: ключ не найден или revoke'нут.
//   - found=true, scopes=nil/[]: ключ существует, но scope'ов нет → доступ запрещён.
//   - err≠nil: проблема с инфраструктурой (БД), возвращается 500.
type ScopeLoader interface {
	Load(ctx context.Context, token string) (scopes []string, found bool, err error)
}

// DBScopeLoader загружает scope'ы напрямую из api_keys + api_key_scopes по sha256-хешу токена.
// Hash-схема совпадает с auth_service.hashAPIKey.
//
// TODO(BUG-83 followup): чисто архитектурно scope'ы должны приходить из auth-service
// через расширение ValidateTokenResponse.scopes. Этот gateway-side loader — тактический
// фикс на момент, когда protoc недоступен локально (Device Guard на Windows). Когда
// окружение позволит regen — расширить proto и удалить DBScopeLoader. См. план
// docs/superpowers/specs/2026-05-01-audit-followups-plan.md §2.
type DBScopeLoader struct {
	pool *pgxpool.Pool
}

// NewDBScopeLoader создаёт loader на основе pgxpool.
func NewDBScopeLoader(pool *pgxpool.Pool) *DBScopeLoader {
	return &DBScopeLoader{pool: pool}
}

// Load выполняет SELECT по api_keys.key_hash + LEFT JOIN api_key_scopes.
// Один запрос даёт и факт существования ключа, и список scope'ов.
func (l *DBScopeLoader) Load(ctx context.Context, token string) ([]string, bool, error) {
	if token == "" {
		return nil, false, nil
	}
	sum := sha256.Sum256([]byte(token))
	keyHash := hex.EncodeToString(sum[:])

	// expires_at-предикат — defense-in-depth: auth-service уже отбраковывает
	// expired-ключи в ValidateToken, но если когда-нибудь invariant сломается,
	// scope-проверка не должна выдавать данные истёкшего ключа.
	rows, err := l.pool.Query(ctx,
		`SELECT s.scope
		 FROM api_keys k
		 LEFT JOIN api_key_scopes s ON s.api_key_id = k.id
		 WHERE k.key_hash = $1
		   AND k.active = true
		   AND (k.expires_at IS NULL OR k.expires_at > NOW())`,
		keyHash)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var scopes []string
	found := false
	for rows.Next() {
		var s *string
		if err := rows.Scan(&s); err != nil {
			return nil, false, err
		}
		found = true
		if s != nil && *s != "" {
			scopes = append(scopes, *s)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return scopes, found, nil
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
// Контракт ошибок:
//   - токен пуст / ключ не найден / revoked → 401.
//   - найден, но требуемого scope'а нет → 403.
//   - ошибка БД при загрузке scope'ов → 500.
//
// scope'ы кладутся в контекст под APIKeyScopesKey для будущих handler'ов.
func RequireScopeByMethod(loader ScopeLoader, readScope, writeScope string) func(http.Handler) http.Handler {
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

			authHeader := r.Header.Get("Authorization")
			token := strings.TrimPrefix(authHeader, "Bearer ")

			scopes, found, err := loader.Load(r.Context(), token)
			if err != nil {
				log.Error().Err(err).Msg("scope loader failed")
				response.Error(w, shared.ErrInternalServer("Не удалось проверить права API-ключа"))
				return
			}
			if !found {
				response.Error(w, shared.ErrUnauthorized("Недействительный API-ключ"))
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

			ctx := context.WithValue(r.Context(), APIKeyScopesKey, scopes)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

