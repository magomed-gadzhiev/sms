package smpp

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcapi "github.com/smpp-server/smpp-server/internal/api/grpc"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
)

// ServiceClients содержит gRPC клиенты для всех сервисов, используемых SMPP Gateway
type ServiceClients struct {
	AuthClient authv1.AuthServiceClient
	
	conns []*grpc.ClientConn
}

// ServiceAddresses содержит адреса всех микросервисов
type ServiceAddresses struct {
	Auth string
}

// NewServiceClients создает подключения ко всем сервисам
func NewServiceClients(addresses ServiceAddresses) (*ServiceClients, error) {
	clients := &ServiceClients{}
	
	// Настройки для gRPC подключений
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5 * time.Second),
		grpc.WithUnaryInterceptor(grpcapi.TraceUnaryClientInterceptor()),
	}
	
	// Подключение к Auth Service
	if addresses.Auth != "" {
		conn, err := grpc.Dial(addresses.Auth, opts...)
		if err != nil {
			return nil, fmt.Errorf("не удалось подключиться к Auth Service: %w", err)
		}
		clients.AuthClient = authv1.NewAuthServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}
	
	return clients, nil
}

// Close закрывает все подключения
func (c *ServiceClients) Close() error {
	var errs []error
	for _, conn := range c.conns {
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("ошибки при закрытии подключений: %v", errs)
	}
	return nil
}

// WithAuthToken добавляет токен авторизации в контекст
func WithAuthToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, "auth_token", token)
}

// GetAuthToken извлекает токен авторизации из контекста
func GetAuthToken(ctx context.Context) (string, bool) {
	token, ok := ctx.Value("auth_token").(string)
	return token, ok
}
