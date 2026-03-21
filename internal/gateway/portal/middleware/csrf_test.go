package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCSRFMiddleware(t *testing.T) {
	t.Run("ValidToken_POST", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}
		csrfToken := "csrf-token-abc123"

		middleware := CSRFMiddleware()
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodPost, "/portal/v1/messages", nil)
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		req.Header.Set("X-CSRF-Token", csrfToken)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called with valid CSRF token")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("ValidToken_PUT", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}
		csrfToken := "csrf-token-put"

		middleware := CSRFMiddleware()
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodPut, "/portal/v1/settings", nil)
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		req.Header.Set("X-CSRF-Token", csrfToken)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called with valid CSRF token on PUT")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("ValidToken_DELETE", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}
		csrfToken := "csrf-token-delete"

		middleware := CSRFMiddleware()
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodDelete, "/portal/v1/messages/123", nil)
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		req.Header.Set("X-CSRF-Token", csrfToken)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called with valid CSRF token on DELETE")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("MissingCookie", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CSRFMiddleware()
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodPost, "/portal/v1/messages", nil)
		req.Header.Set("X-CSRF-Token", "some-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called without CSRF cookie")
		assert.Equal(t, http.StatusForbidden, rr.Code)

		var body map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&body)
		assert.NoError(t, err)
		errObj := body["error"].(map[string]interface{})
		assert.Equal(t, "CSRF_VALIDATION_FAILED", errObj["code"])
	})

	t.Run("MissingHeader", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CSRFMiddleware()
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodPost, "/portal/v1/messages", nil)
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "some-token"})
		// X-CSRF-Token header not set
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called without CSRF header")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("TokenMismatch", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CSRFMiddleware()
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodPost, "/portal/v1/messages", nil)
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookie-token"})
		req.Header.Set("X-CSRF-Token", "different-header-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called when tokens mismatch")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("GET_NoCSRFRequired", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CSRFMiddleware()
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/messages", nil)
		// No CSRF token at all
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called for GET requests without CSRF")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("HEAD_NoCSRFRequired", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CSRFMiddleware()
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodHead, "/portal/v1/messages", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called for HEAD requests without CSRF")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("OPTIONS_NoCSRFRequired", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CSRFMiddleware()
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodOptions, "/portal/v1/messages", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called for OPTIONS requests without CSRF")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("PublicPath_POST_NoCSRFRequired", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CSRFMiddleware()
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", nil)
		// No CSRF token
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called for public auth paths even on POST")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("EmptyCookieValue", func(t *testing.T) {
		recorder := &nextHandlerRecorder{}

		middleware := CSRFMiddleware()
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodPost, "/portal/v1/messages", nil)
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: ""})
		req.Header.Set("X-CSRF-Token", "some-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called with empty CSRF cookie")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}
