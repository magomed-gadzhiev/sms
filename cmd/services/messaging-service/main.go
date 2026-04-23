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

	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/internal/config"
	grpcapi "github.com/smpp-server/smpp-server/internal/api/grpc"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/messaging/application"
	messaginggrpc "github.com/smpp-server/smpp-server/internal/services/messaging/grpc"
	messagingqueue "github.com/smpp-server/smpp-server/internal/services/messaging/infrastructure/queue"
	messagingrepo "github.com/smpp-server/smpp-server/internal/services/messaging/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/jmoiron/sqlx"
)

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
	logger := shared.WithService("messaging-service")

	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Messaging Service")

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

	// Ожидание готовности Kafka
	logger.Info().Msg("ожидание готовности Kafka")
	if err := queue.WaitForKafka(&cfg.Kafka, 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("Kafka недоступен")
	}

	// Инициализация Kafka producer
	kafkaProducer, err := queue.NewProducer(&cfg.Kafka)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka producer")
	}
	defer kafkaProducer.Close()

	logger.Info().Msg("Kafka producer инициализирован")

	// Инициализация репозиториев
	messageRepo := messagingrepo.NewMessageRepository(dbx)
	dlrRepo := messagingrepo.NewDLRRepository(dbx)

	// Инициализация event publisher
	eventPublisher := messagingqueue.NewEventPublisher(kafkaProducer)

	// Инициализация сервисов
	messageService := application.NewMessageService(messageRepo, dlrRepo, eventPublisher)
	dlrService := application.NewDLRService(messageRepo, dlrRepo, eventPublisher)

	// Start scheduler for scheduled messages
	schedulerInterval := 10 * time.Second
	if intervalStr := os.Getenv("SCHEDULER_INTERVAL"); intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil {
			schedulerInterval = d
		}
	}
	schedulerBatchSize := 100
	if batchStr := os.Getenv("SCHEDULER_BATCH_SIZE"); batchStr != "" {
		if n, err := strconv.Atoi(batchStr); err == nil && n > 0 {
			schedulerBatchSize = n
		}
	}
	stuckThreshold := 5 * time.Minute
	if thresholdStr := os.Getenv("SCHEDULER_STUCK_THRESHOLD"); thresholdStr != "" {
		if d, err := time.ParseDuration(thresholdStr); err == nil {
			stuckThreshold = d
		}
	}

	scheduler := application.NewScheduler(messageRepo, eventPublisher, schedulerInterval, schedulerBatchSize, stuckThreshold)
	scheduler.Start()

	// Start data purger for automatic partition cleanup
	messagesRetention := 90
	if v := os.Getenv("DATA_RETENTION_MESSAGES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			messagesRetention = n
		}
	}
	auditRetention := 365
	if v := os.Getenv("DATA_RETENTION_AUDIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			auditRetention = n
		}
	}
	purgeInterval := 24 * time.Hour
	if v := os.Getenv("DATA_PURGE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			purgeInterval = d
		}
	}

	dataPurger := application.NewDataPurger(dbConn.DB, application.DataPurgerConfig{
		MessagesRetentionDays: messagesRetention,
		AuditRetentionDays:    auditRetention,
		PurgeInterval:         purgeInterval,
	})
	dataPurger.Start()

	// Start DLR expiry goroutine
	dlrExpiryTimeout := 24 * time.Hour
	if timeoutStr := os.Getenv("DLR_EXPIRY_TIMEOUT"); timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			dlrExpiryTimeout = d
		}
	}
	dlrExpiryCheckInterval := 5 * time.Minute
	if intervalStr := os.Getenv("DLR_EXPIRY_CHECK_INTERVAL"); intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil {
			dlrExpiryCheckInterval = d
		}
	}
	dlrExpiryBatchSize := 500
	if batchStr := os.Getenv("DLR_EXPIRY_BATCH_SIZE"); batchStr != "" {
		if n, err := strconv.Atoi(batchStr); err == nil && n > 0 {
			dlrExpiryBatchSize = n
		}
	}

	dlrExpiry := application.NewDLRExpiry(messageRepo, dlrExpiryTimeout, dlrExpiryCheckInterval, dlrExpiryBatchSize)
	dlrExpiry.Start()

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("messaging-service", cfg.Service.Version)
	healthChecker.SetDatabase(dbConn.DB)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
		grpc.UnaryInterceptor(grpcapi.TraceUnaryServerInterceptor()),
	)

	// Регистрация gRPC сервиса
	messagingGrpcServer := messaginggrpc.NewServer(messageService, dlrService)
	messagingv1.RegisterMessagingServiceServer(grpcServer, messagingGrpcServer)

	reflection.Register(grpcServer)
	logger.Info().Msg("gRPC reflection включен")

	// Запуск gRPC сервера
	grpcListener, err := net.Listen("tcp", ":9092") // Используем порт для messaging service
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
		Addr:         ":2113", // Используем отдельный порт для метрик messaging service
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

	// Остановка планировщика
	scheduler.Stop()
	logger.Info().Msg("scheduler остановлен")

	// Остановка data purger
	dataPurger.Stop()
	logger.Info().Msg("data purger остановлен")

	// Остановка DLR expiry
	dlrExpiry.Stop()
	logger.Info().Msg("DLR expiry остановлен")

	// Остановка gRPC сервера
	grpcServer.GracefulStop()
	logger.Info().Msg("gRPC сервер остановлен")

	// Остановка HTTP сервера
	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("ошибка при остановке HTTP сервера")
	}
	logger.Info().Msg("HTTP сервер остановлен")

	logger.Info().Msg("Messaging Service остановлен")
}
