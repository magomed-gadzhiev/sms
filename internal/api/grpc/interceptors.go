package grpc

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/internal/api/middleware"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// AuthInterceptor создает gRPC interceptor для аутентификации
func AuthInterceptor(clientRepo *storage.ClientRepository, cfg *config.AuthConfig) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Получаем метаданные из контекста
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "метаданные не найдены")
		}

		// Получаем API ключ из заголовков
		var apiKey string
		if values := md.Get(strings.ToLower(cfg.APIKeyHeader)); len(values) > 0 {
			apiKey = values[0]
		} else if values := md.Get("authorization"); len(values) > 0 {
			// Пробуем получить из Authorization header (Bearer token)
			authHeader := values[0]
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			return nil, status.Error(codes.Unauthenticated, "API ключ не предоставлен")
		}

		// Получаем клиента по API ключу
		client, err := clientRepo.GetByAPIKey(ctx, apiKey)
		if err != nil {
			if err == storage.ErrNotFound {
				return nil, status.Error(codes.Unauthenticated, "неверный API ключ")
			}
			log.Error().Err(err).Msg("ошибка получения клиента")
			return nil, status.Error(codes.Internal, "ошибка аутентификации")
		}

		// Проверяем активность клиента
		if !client.Active {
			return nil, status.Error(codes.PermissionDenied, "клиент неактивен")
		}

		// Добавляем клиента в контекст
		ctx = context.WithValue(ctx, middleware.ClientIDKey, client.ID)
		ctx = context.WithValue(ctx, middleware.ClientKey, client)

		return handler(ctx, req)
	}
}
