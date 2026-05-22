package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpcapi "github.com/smpp-server/smpp-server/internal/api/grpc"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/analytics/application"
	analyticsgrpc "github.com/smpp-server/smpp-server/internal/services/analytics/grpc"
	analyticsrepo "github.com/smpp-server/smpp-server/internal/services/analytics/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
	logger := shared.WithService("analytics-service")

	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Analytics Service")

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

	// Инициализация репозиториев
	metricRepo := analyticsrepo.NewMetricRepository(dbx)
	reportRepo := analyticsrepo.NewReportRepository(dbx)

	// Инициализация сервисов
	analyticsService := application.NewAnalyticsService(metricRepo)
	reportService := application.NewReportService(reportRepo, metricRepo)
	realtimeService := application.NewRealtimeService()

	// Инициализация Kafka consumer
	// Используем уникальный consumer group для analytics service
	kafkaCfg := cfg.Kafka
	kafkaCfg.ConsumerGroup = "analytics-service"

	// Создаем обработчики событий
	messageHandler := func(ctx context.Context, kafkaMsg *queue.KafkaMessage) error {
		// Определяем тип события из metadata
		eventType := "message.created" // по умолчанию считаем созданным
		if kafkaMsg.Metadata != nil {
			if et, ok := kafkaMsg.Metadata["event_type"].(string); ok {
				eventType = et
			}
		}

		var clientIDStr *string
		if kafkaMsg.ClientID != nil {
			clientID := kafkaMsg.ClientID.String()
			clientIDStr = &clientID
		}

		var providerIDStr *string
		if kafkaMsg.ProviderID != nil {
			providerID := kafkaMsg.ProviderID.String()
			providerIDStr = &providerID
		}

		timestamp := kafkaMsg.CreatedAt.Unix()

		// Вызываем соответствующий handler в зависимости от типа события
		switch eventType {
		case "message.created":
			return analyticsService.RecordMessageCreated(ctx, kafkaMsg.MessageID.String(), clientIDStr, providerIDStr, timestamp)
		case "message.sent":
			return analyticsService.RecordMessageSent(ctx, kafkaMsg.MessageID.String(), clientIDStr, providerIDStr, timestamp)
		}

		// Если тип события не определен, все равно записываем как created
		return analyticsService.RecordMessageCreated(ctx, kafkaMsg.MessageID.String(), clientIDStr, providerIDStr, timestamp)
	}

	dlrHandler := func(ctx context.Context, dlr *queue.DLRMessage) error {
		// Обрабатываем только доставленные сообщения
		if dlr.Stat == "DELIVRD" {
			var clientIDStr *string
			var providerIDStr *string

			if dlr.ProviderID != nil {
				providerID := dlr.ProviderID.String()
				providerIDStr = &providerID
			}

			var timestamp int64
			if dlr.DoneDate != nil {
				timestamp = dlr.DoneDate.Unix()
			} else {
				timestamp = time.Now().Unix()
			}

			return analyticsService.RecordMessageDelivered(ctx, dlr.MessageID.String(), clientIDStr, providerIDStr, timestamp)
		}
		return nil
	}

	failedHandler := func(ctx context.Context, failed *queue.FailedMessage) error {
		var clientIDStr *string
		var providerIDStr *string

		if failed.KafkaMessage != nil {
			if failed.KafkaMessage.ClientID != nil {
				clientID := failed.KafkaMessage.ClientID.String()
				clientIDStr = &clientID
			}

			if failed.KafkaMessage.ProviderID != nil {
				providerID := failed.KafkaMessage.ProviderID.String()
				providerIDStr = &providerID
			}
		}

		timestamp := failed.FailedAt.Unix()
		return analyticsService.RecordMessageFailed(ctx, failed.MessageID.String(), clientIDStr, providerIDStr, timestamp, failed.Error)
	}

	kafkaConsumer, err := queue.NewConsumer(&kafkaCfg, messageHandler, dlrHandler, failedHandler)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka consumer")
	}

	logger.Info().Msg("Kafka consumer инициализирован")

	// Запуск Kafka consumers для обработки событий
	go func() {
		// Запускаем потребление из всех нужных топиков
		if err := kafkaConsumer.ConsumeOutgoing(); err != nil {
			logger.Error().Err(err).Msg("ошибка запуска consumer для outgoing сообщений")
		}

		if err := kafkaConsumer.ConsumeDLR(); err != nil {
			logger.Error().Err(err).Msg("ошибка запуска consumer для DLR сообщений")
		}

		if err := kafkaConsumer.ConsumeFailed(); err != nil {
			logger.Error().Err(err).Msg("ошибка запуска consumer для failed сообщений")
		}
	}()

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("analytics-service", cfg.Service.Version)
	healthChecker.SetDatabase(dbConn.DB)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcapi.TraceUnaryServerInterceptor()),
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	// Регистрация gRPC сервиса
	analyticsGrpcServer := analyticsgrpc.NewServer(analyticsService, reportService, realtimeService)
	analyticsv1.RegisterAnalyticsServiceServer(grpcServer, analyticsGrpcServer)

	reflection.Register(grpcServer)
	logger.Info().Msg("gRPC reflection включен")

	// Запуск gRPC сервера
	grpcListener, err := net.Listen("tcp", ":9096") // Используем порт для analytics service
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
		Addr:         ":2117", // Используем отдельный порт для метрик analytics service
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

	// Остановка gRPC сервера
	grpcServer.GracefulStop()
	logger.Info().Msg("gRPC сервер остановлен")

	// Остановка HTTP сервера
	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("ошибка при остановке HTTP сервера")
	}
	logger.Info().Msg("HTTP сервер остановлен")

	logger.Info().Msg("Analytics Service остановлен")
}
