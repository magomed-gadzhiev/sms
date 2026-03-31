package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	grpcapi "github.com/smpp-server/smpp-server/internal/api/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	tarificationgrpc "github.com/smpp-server/smpp-server/internal/services/tarification/grpc"
	"github.com/smpp-server/smpp-server/internal/services/tarification/infrastructure"
	tarificationqueue "github.com/smpp-server/smpp-server/internal/services/tarification/infrastructure/queue"
	tarificationrepo "github.com/smpp-server/smpp-server/internal/services/tarification/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
	logger := shared.WithService("tarification-service")

	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Tarification Service")

	// Ожидание готовности базы данных
	logger.Info().Msg("ожидание готовности базы данных")
	if err := storage.WaitForDatabase(cfg.Database.GetDSN(), 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("база данных недоступна")
	}

	// Инициализация базы данных
	dbConn, err := database.NewDBWithConfig(database.Config{
		DSN:             cfg.Database.GetDSN(),
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к базе данных")
	}
	defer dbConn.Close()

	logger.Info().Msg("подключение к базе данных установлено")

	// Создаем sqlx.DB для использования в репозиториях
	dbx := sqlx.NewDb(dbConn.DB, "pgx")

	// Создаем pgxpool.Pool для провайдерских репозиториев
	pool, err := pgxpool.New(context.Background(), cfg.Database.GetDSN())
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания pgxpool")
	}
	defer pool.Close()

	// Ожидание готовности Kafka
	logger.Info().Msg("ожидание готовности Kafka")
	if err := queue.WaitForKafka(&cfg.Kafka, 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("Kafka недоступен")
	}

	// Инициализация репозиториев
	senderRegistrationRepo := tarificationrepo.NewSenderRegistrationRepository(dbx)
	tariffPlanRepo := tarificationrepo.NewTariffPlanRepository(dbx)
	tariffPeriodRepo := tarificationrepo.NewTariffPeriodRepository(dbx)
	tariffTierRepo := tarificationrepo.NewTariffTierRepository(dbx)
	pricingPeriodRepo := tarificationrepo.NewPricingPeriodRepository(dbx)
	prepaidFeeRepo := tarificationrepo.NewPrepaidFeeRepository(dbx)
	usageCounterRepo := tarificationrepo.NewUsageCounterRepository(dbx)
	tarificationLogRepo := tarificationrepo.NewTarificationLogRepository(dbx)

	// Provider tarification repositories
	providerPlanRepo := infrastructure.NewProviderTariffPlanRepo(pool)
	providerPeriodRepo := infrastructure.NewProviderTariffPeriodRepo(pool)
	providerTierRepo := infrastructure.NewProviderTariffTierRepo(pool)
	providerUsageRepo := infrastructure.NewProviderUsageCounterRepo(pool)
	providerLogRepo := infrastructure.NewProviderTarificationLogRepo(pool)
	marginRepo := infrastructure.NewMarginReportRepo(pool)

	providerTarificationService := application.NewProviderTarificationService(
		providerPlanRepo, providerPeriodRepo, providerTierRepo,
		providerUsageRepo, providerLogRepo,
	)

	// Инициализация Kafka event publisher
	eventPublisher, err := tarificationqueue.NewEventPublisher(
		&cfg.Kafka,
		"tarification.results",
		"tarification.recalc",
		"tarification.prepaid",
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka event publisher")
	}
	defer eventPublisher.Close()

	// Подключение к billing-service через gRPC
	billingAddr := os.Getenv("BILLING_GRPC_ADDR")
	if billingAddr == "" {
		billingAddr = "localhost:9097"
	}

	billingConn, err := grpc.NewClient(billingAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к billing-service")
	}
	defer billingConn.Close()

	billingClient := billingv1.NewBillingServiceClient(billingConn)
	logger.Info().Str("addr", billingAddr).Msg("подключение к billing-service установлено")

	// Инициализация сервисов
	senderBillingRepo := tarificationrepo.NewSenderBillingRepository(dbx)
	senderBillingService := application.NewSenderBillingService(senderBillingRepo)
	senderService := application.NewSenderService(senderRegistrationRepo)
	tariffPlanService := application.NewTariffPlanService(
		tariffPlanRepo, tariffPeriodRepo, tariffTierRepo, pricingPeriodRepo, prepaidFeeRepo,
	)
	tarificationService := application.NewTarificationService(
		senderRegistrationRepo,
		tariffPlanRepo,
		tariffPeriodRepo,
		tariffTierRepo,
		usageCounterRepo,
		tarificationLogRepo,
		prepaidFeeRepo,
		billingClient,
		eventPublisher,
	)

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("tarification-service", cfg.Service.Version)
	healthChecker.SetDatabase(dbConn.DB)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcapi.TraceUnaryServerInterceptor()),
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	// Регистрация gRPC сервиса
	tarificationGrpcServer := tarificationgrpc.NewServer(
		tarificationService, senderService, tariffPlanService, senderBillingService,
	)
	tarificationv1.RegisterTarificationServiceServer(grpcServer, tarificationGrpcServer)

	tarificationGrpcServer.SetProviderTarificationDeps(
		providerPlanRepo, providerPeriodRepo, providerTierRepo,
		marginRepo, providerTarificationService,
	)

	// Подключение к routing-service для планировщика биллинга
	routingAddr := os.Getenv("ROUTING_GRPC_ADDR")
	if routingAddr == "" {
		routingAddr = "routing-service:9090"
	}
	routingConn, err := grpc.NewClient(routingAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к routing-service")
	}
	defer routingConn.Close()
	routingClient := routingv1.NewRoutingServiceClient(routingConn)
	logger.Info().Str("addr", routingAddr).Msg("подключение к routing-service установлено")

	// Запуск планировщика ежемесячного биллинга
	billingScheduler := application.NewBillingScheduler(senderRegistrationRepo, senderBillingService, routingClient)
	billingScheduler.Start()

	// Включение reflection для разработки
	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
		logger.Info().Msg("gRPC reflection включен")
	}

	// Запуск gRPC сервера
	grpcListener, err := net.Listen("tcp", ":9100")
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

	// Создание HTTP сервера для health checks и metrics
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/health", healthChecker.Handler())
	metricsMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	metricsMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())

	if cfg.Monitoring.Prometheus.Enabled {
		metricsMux.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler())
		logger.Info().
			Str("path", cfg.Monitoring.Prometheus.Path).
			Msg("Prometheus metrics endpoint включен")
	}

	metricsServer := &http.Server{
		Addr:         ":2121",
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Запуск HTTP сервера для метрик
	go func() {
		logger.Info().
			Str("addr", metricsServer.Addr).
			Msg("HTTP сервер для метрик запущен")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("ошибка запуска HTTP сервера")
		}
	}()

	// Ожидание сигнала завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("получен сигнал завершения, остановка сервиса")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Остановка планировщика биллинга
	billingScheduler.Stop()

	// Остановка gRPC сервера
	grpcServer.GracefulStop()
	logger.Info().Msg("gRPC сервер остановлен")

	// Остановка HTTP сервера
	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("ошибка при остановке HTTP сервера")
	}
	logger.Info().Msg("HTTP сервер остановлен")

	logger.Info().Msg("Tarification Service остановлен")
}
