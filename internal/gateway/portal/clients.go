package portal

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcapi "github.com/smpp-server/smpp-server/internal/api/grpc"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/auditv1"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	campaignv1 "github.com/smpp-server/smpp-server/api/proto/campaignv1"
	cpv1 "github.com/smpp-server/smpp-server/api/proto/clientproviderv1"
	"github.com/smpp-server/smpp-server/api/proto/clientv1"
	contactv1 "github.com/smpp-server/smpp-server/api/proto/contactv1"
	linkv1 "github.com/smpp-server/smpp-server/api/proto/linkv1"
	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/api/proto/routingv1"
	cascadev1 "github.com/smpp-server/smpp-server/api/proto/cascadev1"
	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"
)

// ServiceClients содержит gRPC клиенты для всех сервисов Portal Gateway
type ServiceClients struct {
	AuthClient           authv1.AuthServiceClient
	ClientClient         clientv1.ClientServiceClient
	BillingClient        billingv1.BillingServiceClient
	MessagingClient      messagingv1.MessagingServiceClient
	AnalyticsClient      analyticsv1.AnalyticsServiceClient
	WebhookClient        webhookv1.WebhookServiceClient
	AuditClient          auditv1.AuditServiceClient
	RoutingClient        routingv1.RoutingServiceClient
	ProviderClient       cpv1.ClientProviderServiceClient
	ContactClient        contactv1.ContactServiceClient
	CampaignClient       campaignv1.CampaignServiceClient
	TemplateClient       templatev1.TemplateServiceClient
	TarificationClient   tarificationv1.TarificationServiceClient
	LinkDomainClient     linkv1.DomainServiceClient
	SenderNameClient     sendernamev1.SenderNameServiceClient
	CascadeClient         cascadev1.CascadeServiceClient
	CascadeChannelAdmin   cascadev1.ChannelAdminServiceClient
	CascadeStrategyAdmin  cascadev1.StrategyAdminServiceClient

	conns []*grpc.ClientConn
}

// ServiceAddresses содержит адреса всех микросервисов для Portal Gateway
type ServiceAddresses struct {
	Auth      string
	Client    string
	Billing   string
	Messaging string
	Analytics string
	Webhook   string
	Audit     string
	Routing   string
	Provider  string
	Contact      string
	Campaign     string
	Template     string
	Tarification string
	Link         string
	Cascade      string
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

	// Подключение к Audit Service
	if addresses.Audit != "" {
		conn, err := grpc.Dial(addresses.Audit, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Audit Service: %w", err)
		}
		clients.AuditClient = auditv1.NewAuditServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}

	// Подключение к Provider Service
	if addresses.Provider != "" {
		conn, err := grpc.Dial(addresses.Provider, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Provider Service: %w", err)
		}
		clients.ProviderClient = cpv1.NewClientProviderServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}

	// Подключение к Contact Service
	if addresses.Contact != "" {
		conn, err := grpc.Dial(addresses.Contact, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Contact Service: %w", err)
		}
		clients.ContactClient = contactv1.NewContactServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}

	// Подключение к Campaign Service
	if addresses.Campaign != "" {
		conn, err := grpc.Dial(addresses.Campaign, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Campaign Service: %w", err)
		}
		clients.CampaignClient = campaignv1.NewCampaignServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}

	// Подключение к Template Service (также регистрирует SenderNameService)
	if addresses.Template != "" {
		conn, err := grpc.Dial(addresses.Template, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Template Service: %w", err)
		}
		clients.TemplateClient = templatev1.NewTemplateServiceClient(conn)
		clients.SenderNameClient = sendernamev1.NewSenderNameServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}

	// Подключение к Tarification Service
	if addresses.Tarification != "" {
		conn, err := grpc.Dial(addresses.Tarification, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Tarification Service: %w", err)
		}
		clients.TarificationClient = tarificationv1.NewTarificationServiceClient(conn)
		clients.conns = append(clients.conns, conn)
	}

	// Подключение к Link Service
	if addresses.Link != "" {
		conn, err := grpc.Dial(addresses.Link, opts...)
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("не удалось подключиться к Link Service: %w", err)
		}
		clients.LinkDomainClient = linkv1.NewDomainServiceClient(conn)
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
		clients.CascadeChannelAdmin = cascadev1.NewChannelAdminServiceClient(conn)
		clients.CascadeStrategyAdmin = cascadev1.NewStrategyAdminServiceClient(conn)
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
