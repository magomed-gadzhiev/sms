package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"
)

// ServiceClients содержит gRPC клиенты для всех сервисов
type ServiceClients struct {
	AuthClient      authv1.AuthServiceClient
	MessagingClient messagingv1.MessagingServiceClient
	AnalyticsClient analyticsv1.AnalyticsServiceClient
	BillingClient   billingv1.BillingServiceClient
	WebhookClient   webhookv1.WebhookServiceClient
	TemplateClient  templatev1.TemplateServiceClient

	conns []*grpc.ClientConn
}

// ServiceAddresses содержит адреса всех микросервисов
type ServiceAddresses struct {
	Auth      string
	Messaging string
	Analytics string
	Billing   string
	Webhook   string
	Template  string
}

// NewServiceClients создает подключения ко всем сервисам
func NewServiceClients(addresses ServiceAddresses) (*ServiceClients, error) {
	clients := &ServiceClients{}
	
	// Настройки для gRPC подключений
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5 * time.Second),
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
	
	// Подключение к Messaging Service
	if addresses.Messaging != "" {
		conn, err := grpc.Dial(addresses.Messaging, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Messaging Service: %w", err)
		}
		clients.MessagingClient = messagingv1.NewMessagingServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}
	
	// Подключение к Analytics Service
	if addresses.Analytics != "" {
		conn, err := grpc.Dial(addresses.Analytics, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Analytics Service: %w", err)
		}
		clients.AnalyticsClient = analyticsv1.NewAnalyticsServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}
	
	// Подключение к Billing Service
	if addresses.Billing != "" {
		conn, err := grpc.Dial(addresses.Billing, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Billing Service: %w", err)
		}
		clients.BillingClient = billingv1.NewBillingServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}

	// Подключение к Webhook Service
	if addresses.Webhook != "" {
		conn, err := grpc.Dial(addresses.Webhook, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Webhook Service: %w", err)
		}
		clients.WebhookClient = webhookv1.NewWebhookServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}

	// Подключение к Template Service
	if addresses.Template != "" {
		conn, err := grpc.Dial(addresses.Template, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Template Service: %w", err)
		}
		clients.TemplateClient = templatev1.NewTemplateServiceClient(conn)
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
