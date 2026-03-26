package grpc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
)

type contextKey string

const (
	ClientIDKey contextKey = "client_id"
	UserKey     contextKey = "user"
)

// AuthInterceptor создает gRPC interceptor для аутентификации клиентов
// LOAD TEST MODE: авторизация отключена для нагрузочного тестирования
func AuthInterceptor(authClient authv1.AuthServiceClient) grpc.UnaryServerInterceptor {
	dummyID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		ctx = context.WithValue(ctx, ClientIDKey, dummyID)
		return handler(ctx, req)
	}
}

// GetClientID извлекает ID клиента из контекста
func GetClientID(ctx context.Context) (uuid.UUID, bool) {
	clientID, ok := ctx.Value(ClientIDKey).(uuid.UUID)
	return clientID, ok
}
