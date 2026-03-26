package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type contextKey string

const (
	ClientIDKey contextKey = "client_id"
	ClientKey   contextKey = "client"
)

// AuthMiddleware создает middleware для аутентификации по API ключу
// LOAD TEST MODE: авторизация отключена для нагрузочного тестирования
func AuthMiddleware(clientRepo ClientRepository, cfg *config.AuthConfig) func(http.Handler) http.Handler {
	dummyID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ClientIDKey, dummyID)
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
