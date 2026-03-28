package middleware

import (
	"crypto/subtle"
	"net/http"
)

// csrfPublicPaths содержит пути, для которых CSRF-валидация не требуется.
var csrfPublicPaths = map[string]struct{}{
	"/portal/v1/auth/login":                   {},
	"/portal/v1/auth/register":                {},
	"/portal/v1/auth/password-reset/request":  {},
	"/portal/v1/auth/password-reset/confirm":  {},
}

// CSRFMiddleware создает middleware для защиты от CSRF-атак.
func CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			if _, ok := csrfPublicPaths[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("csrf_token")
			if err != nil || cookie.Value == "" {
				respondCSRFError(w)
				return
			}

			headerToken := r.Header.Get("X-CSRF-Token")
			if headerToken == "" || subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookie.Value)) != 1 {
				respondCSRFError(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func respondCSRFError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":{"code":"CSRF_VALIDATION_FAILED","message":"Ошибка валидации CSRF-токена"}}`))
}
