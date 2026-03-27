package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/shared"
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

// AuthMiddleware создает middleware для аутентификации по API ключу.
// Если LOAD_TEST_MODE=true — использует dummy client ID (режим нагрузочного тестирования).
// Если LOAD_TEST_MODE не установлен или false — возвращает 401 "auth not configured"
// (сигнализирует о том, что middleware требует полной настройки gRPC клиента).
func AuthMiddleware(clientRepo ClientRepository, cfg *config.AuthConfig) func(http.Handler) http.Handler {
	dummyID := uuid.MustParse("c0000000-0000-0000-0000-000000000001")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isLoadTestMode() {
				ctx := context.WithValue(r.Context(), ClientIDKey, dummyID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			respondError(w, &shared.AppError{
				HTTPStatus: http.StatusUnauthorized,
				Code:       "AUTH_NOT_CONFIGURED",
				Message:    "auth not configured",
			})
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
