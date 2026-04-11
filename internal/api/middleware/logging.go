package middleware

import (
	"bufio"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// LoggingMiddleware создает middleware для логирования запросов
func LoggingMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Генерируем request ID
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}

			// Добавляем request ID в заголовок ответа
			w.Header().Set("X-Request-ID", requestID)

			// Создаем logger с request ID
			reqLogger := logger.With().
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Logger()

			// Добавляем request ID в контекст (и в zerolog, и в shared для propagation)
			ctx := shared.WithRequestID(r.Context(), requestID)
			ctx = reqLogger.WithContext(ctx)
			r = r.WithContext(ctx)

			// Обертка для ResponseWriter для отслеживания статуса
			lw := &loggingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Выполняем следующий handler
			next.ServeHTTP(lw, r)

			// Логируем запрос
			duration := time.Since(start)
			reqLogger.Info().
				Int("status", lw.statusCode).
				Int64("duration_ms", duration.Milliseconds()).
				Int("size", lw.size).
				Msg("HTTP request")
		})
	}
}

// loggingResponseWriter обертка для ResponseWriter
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.statusCode = code
	lw.ResponseWriter.WriteHeader(code)
}

func (lw *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := lw.ResponseWriter.Write(b)
	lw.size += size
	return size, err
}

// Hijack implements http.Hijacker to support WebSocket upgrades.
func (lw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return lw.ResponseWriter.(http.Hijacker).Hijack()
}
