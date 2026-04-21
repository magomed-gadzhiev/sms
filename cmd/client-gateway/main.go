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
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/internal/api/middleware"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/gateway/client"
	clientgrpc "github.com/smpp-server/smpp-server/internal/gateway/client/grpc"
	"github.com/smpp-server/smpp-server/internal/gateway/client/handlers"
	clientmiddleware "github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
	clientrouter "github.com/smpp-server/smpp-server/internal/gateway/client/router"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/shared"
)

func main() {
	// Инициализация логгера (production = JSON format для Promtail/Loki)
	serviceEnv := os.Getenv("SMPP_SERVICE_ENV")
	if serviceEnv == "" {
		serviceEnv = "development"
	}
	shared.InitLogger(serviceEnv)
	logger := shared.WithService("client-gateway")

	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	// Переопределение портов для Client Gateway через переменные окружения
	clientHTTPPort := cfg.API.HTTP.Port
	if portStr := os.Getenv("CLIENT_HTTP_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			clientHTTPPort = port
		}
	}
	if clientHTTPPort == cfg.API.HTTP.Port {
		// По умолчанию используем 8080 для Client Gateway (согласно плану)
		clientHTTPPort = 8080
	}

	clientGRPCPort := cfg.API.GRPC.Port
	if portStr := os.Getenv("CLIENT_GRPC_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			clientGRPCPort = port
		}
	}
	if clientGRPCPort == cfg.API.GRPC.Port {
		// По умолчанию используем 9090 для Client Gateway (согласно плану)
		clientGRPCPort = 9090
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Int("http_port", clientHTTPPort).
		Int("grpc_port", clientGRPCPort).
		Msg("запуск Client Gateway")

	// Получение адресов сервисов из переменных окружения или использование значений по умолчанию
	serviceAddresses := client.ServiceAddresses{
		Auth:      config.EnvOrDefault("AUTH_SERVICE_ADDR", "localhost:9090"),
		Messaging: config.EnvOrDefault("MESSAGING_SERVICE_ADDR", "localhost:9090"),
		Analytics: config.EnvOrDefault("ANALYTICS_SERVICE_ADDR", "localhost:9090"),
		Billing:   config.EnvOrDefault("BILLING_SERVICE_ADDR", "localhost:9090"),
		Webhook:   config.EnvOrDefault("WEBHOOK_SERVICE_ADDR", "localhost:9098"),
		Template:  config.EnvOrDefault("TEMPLATE_SERVICE_ADDR", "localhost:9099"),
		Routing:   config.EnvOrDefault("ROUTING_SERVICE_ADDR", "localhost:9090"),
		Client:     config.EnvOrDefault("CLIENT_SERVICE_ADDR", "localhost:9091"),
		Cascade:    config.EnvOrDefault("CASCADE_SERVICE_ADDR", "localhost:9105"),
		// Sender Name Service is hosted inside the template-service process
		// (template.proto file registers both services on the same port), so
		// we share the template addr unless explicitly overridden.
		SenderName: config.EnvOrDefault("SENDER_NAME_SERVICE_ADDR",
			config.EnvOrDefault("TEMPLATE_SERVICE_ADDR", "localhost:9099")),
	}

	// Инициализация gRPC клиентов
	serviceClients, err := client.NewServiceClients(serviceAddresses)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC клиентов")
	}
	defer serviceClients.Close()

	logger.Info().Msg("gRPC клиенты инициализированы")

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("client-gateway", cfg.Service.Version)

	// Создание handlers
	smsHandlers := handlers.NewSMSHandlers(serviceClients.MessagingClient, serviceClients.TemplateClient, serviceClients.ClientClient, serviceClients.SenderNameClient)
	accountHandlers := handlers.NewAccountHandlers(
		serviceClients.BillingClient,
		serviceClients.AnalyticsClient,
	)
	webhookHandlers := handlers.NewWebhookHandlers(serviceClients.WebhookClient)
	templateHandlers := handlers.NewTemplateHandlers(serviceClients.TemplateClient)
	lookupHandlers := handlers.NewLookupHandlers(serviceClients.RoutingClient)
	cascadeHandlers := handlers.NewCascadeHandlers(serviceClients.CascadeClient)

	// Инициализация Redis для rate limiting
	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.GetAddr(),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	{
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()
		if err := redisClient.Ping(pingCtx).Err(); err != nil {
			logger.Warn().Err(err).Msg("ошибка подключения к Redis (rate limiting может не работать)")
		} else {
			logger.Info().Msg("подключение к Redis установлено")
		}
	}
	defer redisClient.Close()

	// Создание middleware
	authMiddleware := clientmiddleware.ClientAuthMiddleware(serviceClients.AuthClient)
	loggingMiddleware := middleware.LoggingMiddleware(logger)
	recoveryMiddleware := middleware.RecoveryMiddleware()
	corsMiddleware := middleware.CORSMiddlewareFromConfig(os.Getenv("CORS_ALLOWED_ORIGINS"))
	rateLimitMiddleware := middleware.RateLimitMiddleware(redisClient)
	quotaMiddleware := middleware.QuotaMiddleware
	tenantLoggerMiddleware := clientmiddleware.TenantLoggerMiddleware(logger)

	// Настройка HTTP роутера
	router := clientrouter.SetupRouter(
		smsHandlers,
		accountHandlers,
		webhookHandlers,
		templateHandlers,
		lookupHandlers,
		healthChecker,
		authMiddleware,
		loggingMiddleware,
		recoveryMiddleware,
		corsMiddleware,
		rateLimitMiddleware,
		quotaMiddleware,
		tenantLoggerMiddleware,
	)

	// Добавляем маршруты каскадной доставки
	clientrouter.RegisterCascadeRoutes(router, authMiddleware, cascadeHandlers)

	// Добавляем Prometheus metrics endpoint
	if cfg.Monitoring.Prometheus.Enabled {
		router.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler()).Methods("GET")
		logger.Info().
			Str("path", cfg.Monitoring.Prometheus.Path).
			Msg("Prometheus metrics endpoint включен")
	}

	// Создание HTTP сервера
	httpServer := &http.Server{
		Addr:         net.JoinHostPort(cfg.API.HTTP.Host, strconv.Itoa(clientHTTPPort)),
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

	// Создание gRPC interceptor для аутентификации
	authInterceptor := clientgrpc.AuthInterceptor(serviceClients.AuthClient)

	// Создание gRPC сервера с interceptor
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authInterceptor),
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	// Создание и регистрация gRPC handlers для Messaging Service
	grpcHandlers := clientgrpc.NewServer(
		serviceClients.MessagingClient,
		serviceClients.BillingClient,
		serviceClients.AnalyticsClient,
	)

	// Регистрация Messaging Service
	messagingv1.RegisterMessagingServiceServer(grpcServer, grpcHandlers)

	logger.Info().Msg("gRPC handlers зарегистрированы")

	// Включение reflection для разработки
	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
		logger.Info().Msg("gRPC reflection включен")
	}

	// Запуск gRPC сервера
	grpcListener, err := net.Listen("tcp", net.JoinHostPort(cfg.API.GRPC.Host, strconv.Itoa(clientGRPCPort)))
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

	logger.Info().Msg("Client Gateway остановлен")
}

