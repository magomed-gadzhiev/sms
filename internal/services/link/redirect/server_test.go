package redirect

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedirectServer_404OnMissingCode(t *testing.T) {
	handler := NewHandler(nil) // resolver will return error
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRedirectServer_404OnSlashInPath(t *testing.T) {
	handler := NewHandler(nil)
	req := httptest.NewRequest("GET", "/some/path", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRedirectServer_404WhenServiceNil(t *testing.T) {
	handler := NewHandler(nil)
	req := httptest.NewRequest("GET", "/abc1234", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
