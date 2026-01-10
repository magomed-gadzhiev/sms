package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type contextKey string

const (
	ClientIDKey contextKey = "client_id"
	ClientKey   contextKey = "client"
)

// AuthMiddleware создает middleware для аутентификации по API ключу
func AuthMiddleware(clientRepo ClientRepository, cfg *config.AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пропускаем health check и metrics endpoints
			if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			// Получаем API ключ из заголовка
			apiKey := r.Header.Get(cfg.APIKeyHeader)
			if apiKey == "" {
				// Пробуем получить из Authorization header (Bearer token)
				authHeader := r.Header.Get("Authorization")
				if authHeader != "" {
					parts := strings.Split(authHeader, " ")
					if len(parts) == 2 && parts[0] == "Bearer" {
						apiKey = parts[1]
					}
				}
			}

			if apiKey == "" {
				respondError(w, shared.ErrUnauthorized("API ключ не предоставлен"))
				return
			}

			// Получаем клиента по API ключу
			client, err := clientRepo.GetByAPIKey(r.Context(), apiKey)
			if err != nil {
				if err == storage.ErrNotFound {
					respondError(w, shared.ErrUnauthorized("Неверный API ключ"))
					return
				}
				log.Error().Err(err).Msg("ошибка получения клиента")
				respondError(w, shared.ErrInternalServer("Ошибка аутентификации"))
				return
			}

			// Проверяем активность клиента
			if !client.Active {
				respondError(w, shared.ErrForbidden("Клиент неактивен"))
				return
			}

			// Добавляем клиента в контекст
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
