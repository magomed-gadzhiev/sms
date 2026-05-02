package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authedAPIKeyRequest возвращает запрос с auth_method=api_key и предзаполненными scope'ами
// в context — имитация состояния после APIKeyAuthMiddleware.
func authedAPIKeyRequest(method, path string, scopes []string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(r.Context(), AuthMethodKey, AuthMethodAPIKey)
	ctx = context.WithValue(ctx, APIKeyScopesKey, scopes)
	return r.WithContext(ctx)
}

// authedAPIKeyRequestNoScopes — auth прошёл, но scopes не положены в context
// (нарушение инварианта: APIKeyAuthMiddleware всегда обязан их класть; safe-default 401).
func authedAPIKeyRequestNoScopes(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(r.Context(), AuthMethodKey, AuthMethodAPIKey)
	return r.WithContext(ctx)
}

func authedSessionRequest(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(r.Context(), AuthMethodKey, AuthMethodSession)
	return r.WithContext(ctx)
}

func TestRequireScopeByMethod_SessionBypass(t *testing.T) {
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod("messages:read", "messages:send")
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
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod("messages:read", "messages:send")
	h := mw(rec.handler())

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/anything", nil))
	require.True(t, rec.called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequireScopeByMethod_APIKey_GET_RequiresReadScope(t *testing.T) {
	t.Run("HasReadScope_OK", func(t *testing.T) {
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod("messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodGet, "/portal/v1/campaigns", []string{"messages:read"}))

		require.True(t, rec.called)
		assert.Equal(t, http.StatusOK, rr.Code)
		gotScopes, ok := GetAPIKeyScopes(rec.ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"messages:read"}, gotScopes)
	})

	t.Run("HasOnlySendScope_GET_403", func(t *testing.T) {
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod("messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodGet, "/portal/v1/campaigns", []string{"messages:send"}))

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

func TestRequireScopeByMethod_APIKey_POST_RequiresSendScope(t *testing.T) {
	t.Run("HasSendScope_OK", func(t *testing.T) {
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod("messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodPost, "/portal/v1/campaigns", []string{"messages:send"}))

		require.True(t, rec.called)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("HasOnlyReadScope_POST_403", func(t *testing.T) {
		// Это TC-SCOPE-1c — основная регрессия BUG-83.
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod("messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodPost, "/portal/v1/campaigns", []string{"messages:read"}))

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("EmptyScopes_POST_403", func(t *testing.T) {
		// scopes — пустой slice (api_key_scopes пуст). Должен быть deny.
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod("messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodPost, "/portal/v1/campaigns", []string{}))

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("EmptyScopes_GET_403", func(t *testing.T) {
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod("messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodGet, "/portal/v1/campaigns", []string{}))

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

// Инвариант контракта: APIKeyAuthMiddleware всегда кладёт scopes в context при
// auth_method=api_key. Если этого не произошло — что-то сломалось в auth-цепочке;
// safe-default — 401, не пропускать запрос.
func TestRequireScopeByMethod_NoScopesInContext_401(t *testing.T) {
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod("messages:read", "messages:send")
	h := mw(rec.handler())

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, authedAPIKeyRequestNoScopes(http.MethodPost, "/portal/v1/campaigns"))

	assert.False(t, rec.called)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireScopeByMethod_AllWriteMethods(t *testing.T) {
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod("messages:read", "messages:send")
	h := mw(rec.handler())

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			rec.called = false
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, authedAPIKeyRequest(method, "/portal/v1/campaigns/x", []string{"messages:send"}))
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
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod("messages:read", "messages:send")
	h := mw(rec.handler())

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			rec.called = false
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, authedAPIKeyRequest(method, "/portal/v1/campaigns/x", []string{"messages:read"}))
			assert.False(t, rec.called, "read-only key must be denied on write method")
			assert.Equal(t, http.StatusForbidden, rr.Code)
		})
	}
}

// HEAD и OPTIONS должны рассматриваться как read-side (RFC 9110): safe-методы
// без side-effects. Read-only ключ обязан проходить, write-only — отказываться.
func TestRequireScopeByMethod_HEADAndOPTIONSAreReadSide(t *testing.T) {
	t.Run("HEAD_HasReadScope_OK", func(t *testing.T) {
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod("messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodHead, "/portal/v1/campaigns", []string{"messages:read"}))
		require.True(t, rec.called)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("OPTIONS_HasReadScope_OK", func(t *testing.T) {
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod("messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodOptions, "/portal/v1/campaigns", []string{"messages:read"}))
		require.True(t, rec.called)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("HEAD_OnlySendScope_403", func(t *testing.T) {
		rec := &nextHandlerRecorder{}
		mw := RequireScopeByMethod("messages:read", "messages:send")
		h := mw(rec.handler())

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodHead, "/portal/v1/campaigns", []string{"messages:send"}))
		assert.False(t, rec.called)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

// Дозорный: wildcard / "messages:*" не должен подменять exact-match. Если кто-то
// добавит scope "messages:*" в seed надеясь покрыть все messages-операции —
// middleware всё равно требует точный required-scope.
func TestRequireScopeByMethod_NoWildcardMatch(t *testing.T) {
	rec := &nextHandlerRecorder{}
	mw := RequireScopeByMethod("messages:read", "messages:send")
	h := mw(rec.handler())

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, authedAPIKeyRequest(http.MethodPost, "/portal/v1/campaigns", []string{"messages:*"}))
	assert.False(t, rec.called, "wildcard scope must not grant exact-match scope")
	assert.Equal(t, http.StatusForbidden, rr.Code)
}
