package grpc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"github.com/smpp-server/smpp-server/internal/api/middleware"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// AuthInterceptor создает gRPC interceptor для аутентификации
// LOAD TEST MODE: авторизация отключена для нагрузочного тестирования
func AuthInterceptor(clientRepo *storage.ClientRepository, cfg *config.AuthConfig) grpc.UnaryServerInterceptor {
	dummyID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		ctx = context.WithValue(ctx, middleware.ClientIDKey, dummyID)
		return handler(ctx, req)
	}
}
