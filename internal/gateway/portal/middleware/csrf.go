package middleware

import (
	"net/http"
)

// CSRFMiddleware создает middleware для защиты от CSRF-атак
// LOAD TEST MODE: CSRF-защита отключена для нагрузочного тестирования
func CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

// respondCSRFError отправляет ошибку CSRF-валидации
func respondCSRFError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":{"code":"CSRF_VALIDATION_FAILED","message":"Ошибка валидации CSRF-токена"}}`))
}
