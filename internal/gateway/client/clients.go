package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcapi "github.com/smpp-server/smpp-server/internal/api/grpc"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	clientv1 "github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/api/proto/routingv1"
	cascadev1 "github.com/smpp-server/smpp-server/api/proto/cascadev1"
	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
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
	RoutingClient   routingv1.RoutingServiceClient
	ClientClient     clientv1.ClientServiceClient
	CascadeClient    cascadev1.CascadeServiceClient
	SenderNameClient sendernamev1.SenderNameServiceClient

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
	Routing   string
	Client     string
	Cascade    string
	SenderName string
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

	// Подключение к Routing Service
	if addresses.Routing != "" {
		conn, err := grpc.Dial(addresses.Routing, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Routing Service: %w", err)
		}
		clients.RoutingClient = routingv1.NewRoutingServiceClient(conn)
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

	// Подключение к Client Service
	if addresses.Client != "" {
		conn, err := grpc.Dial(addresses.Client, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Client Service: %w", err)
		}
		clients.ClientClient = clientv1.NewClientServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}


	// Подключение к Cascade Service
	if addresses.Cascade != "" {
		conn, err := grpc.Dial(addresses.Cascade, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Cascade Service: %w", err)
		}
		clients.CascadeClient = cascadev1.NewCascadeServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}

	// Подключение к Sender Name Service (template-service hosts it).
	if addresses.SenderName != "" {
		conn, err := grpc.Dial(addresses.SenderName, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Sender Name Service: %w", err)
		}
		clients.SenderNameClient = sendernamev1.NewSenderNameServiceClient(conn)
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
