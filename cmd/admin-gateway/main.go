package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/redis/go-redis/v9"
	sharedmw "github.com/smpp-server/smpp-server/internal/api/middleware"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/gateway/admin"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/handlers"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/middleware"
	adminrouter "github.com/smpp-server/smpp-server/internal/gateway/admin/router"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
	logger := shared.WithService("admin-gateway")

	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	// Переопределение портов для Admin Gateway через переменные окружения
	adminHTTPPort := cfg.API.HTTP.Port
	if portStr := os.Getenv("ADMIN_HTTP_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			adminHTTPPort = port
		}
	}
	if adminHTTPPort == cfg.API.HTTP.Port {
		// По умолчанию используем 8081 для Admin Gateway
		adminHTTPPort = 8081
	}

	adminGRPCPort := cfg.API.GRPC.Port
	if portStr := os.Getenv("ADMIN_GRPC_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			adminGRPCPort = port
		}
	}
	if adminGRPCPort == cfg.API.GRPC.Port {
		// По умолчанию используем 9091 для Admin Gateway
		adminGRPCPort = 9091
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Int("http_port", adminHTTPPort).
		Int("grpc_port", adminGRPCPort).
		Msg("запуск Admin Gateway")

	// Получение адресов сервисов из переменных окружения или использование значений по умолчанию
	serviceAddresses := admin.ServiceAddresses{
		Auth:      config.EnvOrDefault("AUTH_SERVICE_ADDR", "localhost:9090"),
		Client:    config.EnvOrDefault("CLIENT_SERVICE_ADDR", "localhost:9090"),
		Provider:  config.EnvOrDefault("PROVIDER_SERVICE_ADDR", "localhost:9090"),
		Routing:   config.EnvOrDefault("ROUTING_SERVICE_ADDR", "localhost:9090"),
		Analytics: config.EnvOrDefault("ANALYTICS_SERVICE_ADDR", "localhost:9090"),
		Billing:   config.EnvOrDefault("BILLING_SERVICE_ADDR", "localhost:9090"),
		Webhook:   config.EnvOrDefault("WEBHOOK_SERVICE_ADDR", "localhost:9098"),
		Template:     config.EnvOrDefault("TEMPLATE_SERVICE_ADDR", "localhost:9099"),
		Tarification: config.EnvOrDefault("TARIFICATION_SERVICE_ADDR", "localhost:9100"),
	}

	// Инициализация gRPC клиентов
	serviceClients, err := admin.NewServiceClients(serviceAddresses)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC клиентов")
	}
	defer serviceClients.Close()

	logger.Info().Msg("gRPC клиенты инициализированы")

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("admin-gateway", cfg.Service.Version)

	// DB connection for handlers that require direct DB access
	adminDB, dbErr := storage.NewDBWithConfig(
		cfg.Database.GetDSN(),
		cfg.Database.MaxOpenConns,
		cfg.Database.MaxIdleConns,
		cfg.Database.ConnMaxLifetime,
		cfg.Database.ConnMaxIdleTime,
	)
	if dbErr != nil {
		logger.Fatal().Err(dbErr).Msg("ошибка подключения к БД для admin handlers")
	}
	defer adminDB.Close()

	// Создание handlers
	clientHandlers := handlers.NewClientHandlers(serviceClients.ClientClient)
	providerHandlers := handlers.NewProviderHandlers(serviceClients.ProviderClient)
	routingHandlers := handlers.NewRoutingHandlers(serviceClients.RoutingClient)
	analyticsHandlers := handlers.NewAnalyticsHandlers(serviceClients.AnalyticsClient)
	billingHandlers := handlers.NewBillingHandlers(serviceClients.BillingClient)
	webhookHandlers := handlers.NewWebhookHandlers(serviceClients.WebhookClient)
	templateHandlers := handlers.NewTemplateHandlers(serviceClients.TemplateClient)
	countryHandlers := handlers.NewCountryHandler(serviceClients.RoutingClient)
	operatorHandlers := handlers.NewOperatorHandler(serviceClients.RoutingClient)
	tarificationHandlers := handlers.NewTarificationHandler(serviceClients.TarificationClient, adminDB)
	hlrHandlers := handlers.NewHLRHandlers(serviceClients.RoutingClient)
	clientRoutingHandlers := handlers.NewClientRoutingHandlers(serviceClients.RoutingClient, serviceClients.TarificationClient)

	systemDefaultsRepo := storage.NewSystemDefaultsRepository(adminDB)
	systemDefaultsHandlers := handlers.NewSystemDefaultsHandlers(systemDefaultsRepo)
	stubConfigRepo := storage.NewStubConfigRepository(adminDB)
	stubConfigHandlers := handlers.NewStubConfigHandlers(stubConfigRepo)

	userHandlers := handlers.NewUserHandlers(serviceClients.AuthClient)
	userHandlers.SetClientClient(serviceClients.ClientClient)
	roleHandlers := handlers.NewRoleHandlers(serviceClients.AuthClient)
	senderNameHandlers := handlers.NewAdminSenderNameHandlers(serviceClients.SenderNameClient)
	senderNameHandlers.SetClients(serviceClients.RoutingClient, serviceClients.TarificationClient)
	hierarchicalPeriodsHandler := handlers.NewHierarchicalPeriodsHandler(adminDB)
	detalizationHandlers := handlers.NewDetalizationHandlers(adminDB)
	legalEntityHandlers := handlers.NewLegalEntityHandlers(adminDB)
	contractHandlers := handlers.NewContractHandlers(adminDB)
	operatorTemplateHandlers := handlers.NewOperatorTemplateHandlers(adminDB)

	// Redis client для ConnectionsHandlers
	redisDSN := config.EnvOrDefault("REDIS_ADDR", "localhost:6379")
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisDSN,
	})
	defer redisClient.Close()

	platformRoutesHandlers := handlers.NewPlatformRoutesHandlers(adminDB)
	connectionsHandlers := handlers.NewConnectionsHandlers(adminDB, serviceClients.ProviderClient, redisClient)
	auditHandlers := handlers.NewAdminAuditHandlers(serviceClients.AuditClient)

	// Создание middleware
	authMiddleware := middleware.AdminAuthMiddleware(serviceClients.AuthClient)
	loggingMiddleware := sharedmw.LoggingMiddleware(logger)
	recoveryMiddleware := sharedmw.RecoveryMiddleware()
	corsMiddleware := sharedmw.CORSMiddlewareFromConfig(os.Getenv("CORS_ALLOWED_ORIGINS"))

	// Настройка HTTP роутера
	router := adminrouter.SetupRouter(
		clientHandlers,
		providerHandlers,
		routingHandlers,
		analyticsHandlers,
		billingHandlers,
		webhookHandlers,
		templateHandlers,
		countryHandlers,
		operatorHandlers,
		tarificationHandlers,
		hlrHandlers,
		clientRoutingHandlers,
		systemDefaultsHandlers,
		stubConfigHandlers,
		userHandlers,
		roleHandlers,
		senderNameHandlers,
		hierarchicalPeriodsHandler,
		detalizationHandlers,
		legalEntityHandlers,
		contractHandlers,
		operatorTemplateHandlers,
		platformRoutesHandlers,
		connectionsHandlers,
		auditHandlers,
		healthChecker,
		authMiddleware,
		loggingMiddleware,
		recoveryMiddleware,
		corsMiddleware,
	)

	// Добавляем Prometheus metrics endpoint
	if cfg.Monitoring.Prometheus.Enabled {
		router.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler()).Methods("GET")
		logger.Info().
			Str("path", cfg.Monitoring.Prometheus.Path).
			Msg("Prometheus metrics endpoint включен")
	}

	// Создание HTTP сервера
	httpServer := &http.Server{
		Addr:         net.JoinHostPort(cfg.API.HTTP.Host, strconv.Itoa(adminHTTPPort)),
		Handler:      router,
		ReadTimeout:  cfg.API.HTTP.ReadTimeout,
		WriteTimeout: cfg.API.HTTP.WriteTimeout,
		IdleTimeout:  cfg.API.HTTP.IdleTimeout,
	}

	// Запуск HTTP сервера
	go func() {
		logger.Info().
			Str("addr", httpServer.Addr).
			Msg("HTTP сервер запущен")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("ошибка запуска HTTP сервера")
		}
	}()

	// Создание gRPC сервера (для будущих расширений)
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	// Включение reflection для разработки
	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
		logger.Info().Msg("gRPC reflection включен")
	}

	// Запуск gRPC сервера
	grpcListener, err := net.Listen("tcp", net.JoinHostPort(cfg.API.GRPC.Host, strconv.Itoa(adminGRPCPort)))
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC listener")
	}

	go func() {
		logger.Info().
			Str("addr", grpcListener.Addr().String()).
			Msg("gRPC сервер запущен")
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatal().Err(err).Msg("ошибка запуска gRPC сервера")
		}
	}()

	// Ожидание сигнала для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	logger.Info().Msg("получен сигнал остановки")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Остановка HTTP сервера
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("ошибка остановки HTTP сервера")
	} else {
		logger.Info().Msg("HTTP сервер остановлен")
	}

	// Остановка gRPC сервера
	grpcServer.GracefulStop()
	logger.Info().Msg("gRPC сервер остановлен")

	logger.Info().Msg("Admin Gateway остановлен")
}

