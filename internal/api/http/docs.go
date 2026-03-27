package http

import (
	"net/http"

	"github.com/smpp-server/smpp-server/internal/docs"
)

// DocsHandler отдаёт Swagger UI страницу
func DocsHandler() http.HandlerFunc {
	return docs.SwaggerUIHandler()
}

// OpenAPISpecHandler отдаёт OpenAPI спецификацию
func OpenAPISpecHandler() http.HandlerFunc {
	return docs.OpenAPISpecHandler()
}
