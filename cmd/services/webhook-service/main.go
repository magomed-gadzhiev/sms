// cmd/services/webhook-service/main.go
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

	webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/webhook/application"
	webhookgrpc "github.com/smpp-server/smpp-server/internal/services/webhook/grpc"
	webhookhttp "github.com/smpp-server/smpp-server/internal/services/webhook/infrastructure/http"
	webhookrepo "github.com/smpp-server/smpp-server/internal/services/webhook/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	shared.InitLogger("development")
	logger := shared.WithService("webhook-service")

	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Webhook Service")

	// Wait for database
	logger.Info().Msg("ожидание готовности базы данных")
	if err := storage.WaitForDatabase(cfg.Database.GetDSN(), 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("база данных недоступна")
	}

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

	dbx := sqlx.NewDb(dbConn.DB, "pgx")

	// Wait for Kafka
	logger.Info().Msg("ожидание готовности Kafka")
	if err := queue.WaitForKafka(&cfg.Kafka, 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("Kafka недоступен")
	}

	// Repositories
	subRepo := webhookrepo.NewSubscriptionRepository(dbx)
	msgRepo := webhookrepo.NewMessageRepository(dbx)

	// HTTP delivery client
	httpClient := webhookhttp.NewDeliveryClient(5 * time.Second)

	// Application services
	deliveryService := application.NewDeliveryService(subRepo, msgRepo, httpClient, 50)
	webhookService := application.NewWebhookService(subRepo, deliveryService)
	defer deliveryService.Close()

	// Kafka consumer
	kafkaCfg := cfg.Kafka
	kafkaCfg.ConsumerGroup = "webhook-service"

	// No-op handler for outgoing messages (webhook service doesn't process them)
	messageHandler := func(ctx context.Context, kafkaMsg *queue.KafkaMessage) error {
		return nil
	}

	kafkaConsumer, err := queue.NewConsumer(&kafkaCfg, messageHandler, deliveryService.HandleDLR, deliveryService.HandleFailed)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka consumer")
	}
	logger.Info().Msg("Kafka consumer инициализирован")

	// Start consuming DLR and failed topics in separate goroutines
	go func() {
		if err := kafkaConsumer.ConsumeDLR(); err != nil {
			logger.Error().Err(err).Msg("ошибка запуска consumer для DLR")
		}
	}()
	go func() {
		if err := kafkaConsumer.ConsumeFailed(); err != nil {
			logger.Error().Err(err).Msg("ошибка запуска consumer для failed")
		}
	}()

	// Health checker
	healthChecker := monitoring.NewHealthChecker("webhook-service", cfg.Service.Version)
	healthChecker.SetDatabase(dbConn.DB)

	// gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	webhookGrpcServer := webhookgrpc.NewServer(webhookService)
	webhookv1.RegisterWebhookServiceServer(grpcServer, webhookGrpcServer)

	reflection.Register(grpcServer)
	logger.Info().Msg("gRPC reflection включен")

	grpcListener, err := net.Listen("tcp", ":9098")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC listener")
	}

	go func() {
		logger.Info().Str("addr", grpcListener.Addr().String()).Msg("gRPC сервер запущен")
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatal().Err(err).Msg("ошибка запуска gRPC сервера")
		}
	}()

	// Metrics HTTP server
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/health", healthChecker.Handler())
	metricsMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	metricsMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())

	if cfg.Monitoring.Prometheus.Enabled {
		metricsMux.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler())
		logger.Info().Str("path", cfg.Monitoring.Prometheus.Path).Msg("Prometheus metrics endpoint включен")
	}

	metricsServer := &http.Server{
		Addr:         ":2119",
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		logger.Info().Str("addr", metricsServer.Addr).Msg("HTTP сервер для метрик запущен")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("ошибка запуска HTTP сервера")
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("получен сигнал завершения, остановка сервиса")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop Kafka consumer
	if err := kafkaConsumer.Close(); err != nil {
		logger.Error().Err(err).Msg("ошибка при остановке Kafka consumer")
	}
	logger.Info().Msg("Kafka consumer остановлен")

	grpcServer.GracefulStop()
	logger.Info().Msg("gRPC сервер остановлен")

	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("ошибка при остановке HTTP сервера")
	}
	logger.Info().Msg("HTTP сервер остановлен")

	logger.Info().Msg("Webhook Service остановлен")
}
