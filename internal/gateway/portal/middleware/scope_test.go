package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubScopeLoader — in-memory loader для unit-тестов middleware.
type stubScopeLoader struct {
	scopes []string
	found  bool
	err    error
}

func (s *stubScopeLoader) Load(ctx context.Context, token string) ([]string, bool, error) {
	return s.scopes, s.found, s.err
}

func authedAPIKeyRequest(method, path, token string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	ctx := context.WithValue(r.Context(), AuthMethodKey, AuthMethodAPIKey)
	return r.WithContext(ctx)
}

func authedSessionRequest(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(r.Context(), AuthMethodKey, AuthMethodSession)
	return r.WithContext(ctx)
}

func TestRequireScopeByMethod_SessionBypass(t *testing.T) {
	loader := &stubScopeLoader{} // never called
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
	h := mw(rec.handler())

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec.called = false
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, authedSessionRequest(method, "/portal/v1/campaigns"))
			require.True(t, rec.called, "session auth must bypass scope check")
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestRequireScopeByMethod_NoAuthMethod_Bypass(t *testing.T) {
	// Если auth-method ещё не выставлен (middleware применён до auth-цепочки),
	// scope-проверка не должна срабатывать — иначе сломаются публичные роуты.
	loader := &stubScopeLoader{}
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
	h := mw(rec.handler())

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/anything", nil))
	require.True(t, rec.called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequireScopeByMethod_APIKey_GET_RequiresReadScope(t *testing.T) {
	t.Run("HasReadScope_OK", func(t *testing.T) {
		loader := &stubScopeLoader{scopes: []string{"messages:read"}, found: true}
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodGet, "/portal/v1/campaigns", "sk_live_x"))

		require.True(t, rec.called)
		assert.Equal(t, http.StatusOK, rr.Code)
		gotScopes, ok := GetAPIKeyScopes(rec.ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"messages:read"}, gotScopes)
	})

	t.Run("HasOnlySendScope_GET_403", func(t *testing.T) {
		loader := &stubScopeLoader{scopes: []string{"messages:send"}, found: true}
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodGet, "/portal/v1/campaigns", "sk_live_x"))

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

func TestRequireScopeByMethod_APIKey_POST_RequiresSendScope(t *testing.T) {
	t.Run("HasSendScope_OK", func(t *testing.T) {
		loader := &stubScopeLoader{scopes: []string{"messages:send"}, found: true}
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodPost, "/portal/v1/campaigns", "sk_live_x"))

		require.True(t, rec.called)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("HasOnlyReadScope_POST_403", func(t *testing.T) {
		// Это TC-SCOPE-1c — основная регрессия BUG-83.
		loader := &stubScopeLoader{scopes: []string{"messages:read"}, found: true}
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodPost, "/portal/v1/campaigns", "sk_live_x"))

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("EmptyScopes_POST_403", func(t *testing.T) {
		// Ключ существует, но scope'ов нет (api_key_scopes пуст) — должен быть deny,
		// а не "полный доступ" (legacy domain.APIKey.HasScope-семантика тут переопределяется).
		loader := &stubScopeLoader{scopes: nil, found: true}
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodPost, "/portal/v1/campaigns", "sk_live_x"))

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("EmptyScopes_GET_403", func(t *testing.T) {
		loader := &stubScopeLoader{scopes: nil, found: true}
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodGet, "/portal/v1/campaigns", "sk_live_x"))

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

func TestRequireScopeByMethod_KeyNotFound_401(t *testing.T) {
	loader := &stubScopeLoader{found: false}
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
	h := mw(rec.handler())

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodPost, "/portal/v1/campaigns", "sk_live_revoked"))

	assert.False(t, rec.called)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireScopeByMethod_LoaderError_500(t *testing.T) {
	loader := &stubScopeLoader{err: errors.New("db down")}
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
	h := mw(rec.handler())

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodPost, "/portal/v1/campaigns", "sk_live_x"))

	assert.False(t, rec.called)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestRequireScopeByMethod_AllWriteMethods(t *testing.T) {
	loader := &stubScopeLoader{scopes: []string{"messages:send"}, found: true}
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
	h := mw(rec.handler())

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			rec.called = false
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, authedAPIKeyRequest(method, "/portal/v1/campaigns/x", "sk_live_x"))
			require.True(t, rec.called, "non-GET should require write scope; have it → pass")
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

// Симметричный кейс к TC-SCOPE-1c: для всех write-методов read-only ключ должен
// получать 403, без надежды что какой-то метод случайно проскочит. Если кто-то
// в будущем зарефакторит method-mapping (например с typo "DELET" вместо DELETE) —
// этот тест поймает регрессию.
func TestRequireScopeByMethod_AllWriteMethodsDeniedWithReadOnly(t *testing.T) {
	loader := &stubScopeLoader{scopes: []string{"messages:read"}, found: true}
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
	h := mw(rec.handler())

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			rec.called = false
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, authedAPIKeyRequest(method, "/portal/v1/campaigns/x", "sk_live_x"))
			assert.False(t, rec.called, "read-only key must be denied on write method")
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

// HEAD и OPTIONS должны рассматриваться как read-side (RFC 9110): safe-методы
// без side-effects. Read-only ключ обязан проходить, write-only — отказываться.
func TestRequireScopeByMethod_HEADAndOPTIONSAreReadSide(t *testing.T) {
	t.Run("HEAD_HasReadScope_OK", func(t *testing.T) {
		loader := &stubScopeLoader{scopes: []string{"messages:read"}, found: true}
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodHead, "/portal/v1/campaigns", "sk_live_x"))
		require.True(t, rec.called)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("OPTIONS_HasReadScope_OK", func(t *testing.T) {
		loader := &stubScopeLoader{scopes: []string{"messages:read"}, found: true}
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodOptions, "/portal/v1/campaigns", "sk_live_x"))
		require.True(t, rec.called)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("HEAD_OnlySendScope_403", func(t *testing.T) {
		loader := &stubScopeLoader{scopes: []string{"messages:send"}, found: true}
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodHead, "/portal/v1/campaigns", "sk_live_x"))
		assert.False(t, rec.called)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

// Дозорный: wildcard / "messages:*" не должен подменять exact-match. Если кто-то
// добавит scope "messages:*" в seed надеясь покрыть все messages-операции —
// middleware всё равно требует точный required-scope.
func TestRequireScopeByMethod_NoWildcardMatch(t *testing.T) {
	loader := &stubScopeLoader{scopes: []string{"messages:*"}, found: true}
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod(loader, "messages:read", "messages:send")
	h := mw(rec.handler())

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodPost, "/portal/v1/campaigns", "sk_live_x"))
	assert.False(t, rec.called, "wildcard scope must not grant exact-match scope")
	assert.Equal(t, http.StatusForbidden, rr.Code)
}
