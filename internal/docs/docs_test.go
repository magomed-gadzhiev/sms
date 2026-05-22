package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedocHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	w := httptest.NewRecorder()
	RedocHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html; charset=utf-8, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), "redoc") {
		t.Error("body should contain 'redoc'")
	}
}

func TestSwaggerUIHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs/swagger", nil)
	w := httptest.NewRecorder()
	SwaggerUIHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html; charset=utf-8, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), "swagger-ui") {
		t.Error("body should contain 'swagger-ui'")
	}
}

func TestGRPCDocsHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs/grpc", nil)
	w := httptest.NewRecorder()
	GRPCDocsHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html; charset=utf-8, got %q", ct)
	}
	if len(w.Body.Bytes()) == 0 {
		t.Error("body should not be empty")
	}
}

func TestOpenAPISpecHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	w := httptest.NewRecorder()
	OpenAPISpecHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Errorf("expected application/yaml, got %q", ct)
	}
	if len(w.Body.Bytes()) == 0 {
		t.Error("body should not be empty")
	}
	if !strings.Contains(w.Body.String(), "openapi:") {
		t.Error("body should contain openapi spec content")
	}
}
