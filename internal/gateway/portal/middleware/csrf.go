package middleware

import (
	"net/http"
)

// CSRFMiddleware создает middleware для защиты от CSRF-атак
func CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пропускаем безопасные методы
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Пропускаем публичные пути аутентификации
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Получаем CSRF-токен из cookie
			cookie, err := r.Cookie("csrf_token")
			if err != nil || cookie.Value == "" {
				respondCSRFError(w)
				return
			}

			// Получаем CSRF-токен из заголовка
			headerToken := r.Header.Get("X-CSRF-Token")
			if headerToken == "" {
				respondCSRFError(w)
				return
			}

			// Сравниваем токены
			if cookie.Value != headerToken {
				respondCSRFError(w)
				return
			}

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
