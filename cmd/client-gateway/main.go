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

	"github.com/smpp-server/smpp-server/internal/config"
	clientgateway 	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/client"
	clientgrpc "github.com/smpp-server/smpp-server/internal/gateway/client/grpc"
	"github.com/smpp-server/smpp-server/internal/gateway/client/handlers"
	clientmiddleware "github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
	clientrouter "github.com/smpp-server/smpp-server/internal/gateway/client/router"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/shared"
)

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
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
	serviceAddresses := clientgateway.ServiceAddresses{
		Auth:      getEnvOrDefault("AUTH_SERVICE_ADDR", "localhost:9090"),
		Messaging: getEnvOrDefault("MESSAGING_SERVICE_ADDR", "localhost:9090"),
		Analytics: getEnvOrDefault("ANALYTICS_SERVICE_ADDR", "localhost:9090"),
		Billing:   getEnvOrDefault("BILLING_SERVICE_ADDR", "localhost:9090"),
	}

	// Инициализация gRPC клиентов
	serviceClients, err := clientgateway.NewServiceClients(serviceAddresses)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC клиентов")
	}
	defer serviceClients.Close()

	logger.Info().Msg("gRPC клиенты инициализированы")

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("client-gateway", cfg.Service.Version)

	// Создание handlers
	smsHandlers := handlers.NewSMSHandlers(serviceClients.MessagingClient)
	accountHandlers := handlers.NewAccountHandlers(
		serviceClients.BillingClient,
		serviceClients.AnalyticsClient,
	)

	// Создание middleware
	authMiddleware := clientmiddleware.ClientAuthMiddleware(serviceClients.AuthClient)
	loggingMiddleware := clientmiddleware.LoggingMiddleware(logger)
	recoveryMiddleware := clientmiddleware.RecoveryMiddleware()
	corsMiddleware := clientmiddleware.CORSMiddleware(os.Getenv("CORS_ALLOWED_ORIGINS"))

	// Настройка HTTP роутера
	router := clientrouter.SetupRouter(
		smsHandlers,
		accountHandlers,
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

// getEnvOrDefault возвращает значение переменной окружения или значение по умолчанию
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
