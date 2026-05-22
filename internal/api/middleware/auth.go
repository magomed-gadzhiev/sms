package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type contextKey string

const (
	ClientIDKey contextKey = "client_id"
	ClientKey   contextKey = "client"
)

// isLoadTestMode возвращает true, если переменная окружения LOAD_TEST_MODE=true
func isLoadTestMode() bool {
	return strings.EqualFold(os.Getenv("LOAD_TEST_MODE"), "true")
}

// skipPaths содержит пути, которые не требуют аутентификации
var skipPaths = map[string]bool{
	"/health":  true,
	"/metrics": true,
}

// AuthMiddleware создает middleware для аутентификации по API ключу.
// Если LOAD_TEST_MODE=true — использует dummy client ID (режим нагрузочного тестирования).
// Иначе выполняет полную аутентификацию по API ключу из заголовка X-API-Key или Bearer токена.
func AuthMiddleware(clientRepo ClientRepository, cfg *config.AuthConfig) func(http.Handler) http.Handler {
	dummyID := uuid.MustParse("c0000000-0000-0000-0000-000000000001")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пропускаем служебные эндпоинты
			if skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Режим нагрузочного тестирования
			if isLoadTestMode() {
				ctx := context.WithValue(r.Context(), ClientIDKey, dummyID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Извлекаем API ключ из заголовков
			apiKey := r.Header.Get(cfg.APIKeyHeader)
			if apiKey == "" {
				// Пробуем Bearer токен из Authorization заголовка
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					apiKey = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if apiKey == "" {
				respondError(w, shared.ErrUnauthorized("API key required"))
				return
			}

			// Ищем клиента по API ключу
			client, err := clientRepo.GetByAPIKey(r.Context(), apiKey)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					respondError(w, shared.ErrUnauthorized("invalid API key"))
					return
				}
				respondError(w, shared.ErrDatabase("failed to authenticate", err))
				return
			}

			// Проверяем активность клиента
			if !client.Active {
				respondError(w, shared.ErrForbidden("client is inactive"))
				return
			}

			// Устанавливаем клиента в контекст
			ctx := context.WithValue(r.Context(), ClientIDKey, client.ID)
			ctx = context.WithValue(ctx, ClientKey, client)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClientID извлекает ID клиента из контекста
func GetClientID(ctx context.Context) (uuid.UUID, bool) {
	clientID, ok := ctx.Value(ClientIDKey).(uuid.UUID)
	return clientID, ok
}

// GetClient извлекает клиента из контекста
func GetClient(ctx context.Context) (*shared.Client, bool) {
	client, ok := ctx.Value(ClientKey).(*shared.Client)
	return client, ok
}

// respondError отправляет ошибку в формате JSON
func respondError(w http.ResponseWriter, err *shared.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus)
	
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		},
	}
	
	if err.Details != "" {
		response["error"].(map[string]interface{})["details"] = err.Details
	}
	
	json.NewEncoder(w).Encode(response)
}
