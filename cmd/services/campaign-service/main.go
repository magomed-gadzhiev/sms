package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	campaignv1 "github.com/smpp-server/smpp-server/api/proto/campaignv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/services/campaign/application"
	campaigngrpc "github.com/smpp-server/smpp-server/internal/services/campaign/grpc"
	campaignrepo "github.com/smpp-server/smpp-server/internal/services/campaign/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	shared.InitLogger("development")
	logger := shared.WithService("campaign-service")

	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Campaign Service")

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

	// --- Kafka producer (best-effort) ---
	var kafkaProducer sarama.SyncProducer
	if len(cfg.Kafka.Brokers) > 0 {
		saramaConfig := sarama.NewConfig()
		saramaConfig.Producer.Return.Successes = true
		saramaConfig.Producer.Return.Errors = true
		saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
		saramaConfig.Producer.Retry.Max = cfg.Kafka.MaxRetries
		saramaConfig.Producer.Retry.Backoff = cfg.Kafka.RetryBackoff
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
		saramaConfig.Net.MaxOpenRequests = 1

		kafkaProducer, err = sarama.NewSyncProducer(cfg.Kafka.Brokers, saramaConfig)
		if err != nil {
			logger.Warn().Err(err).Msg("не удалось создать Kafka producer, продолжаем без Kafka")
		} else {
			logger.Info().Msg("Kafka producer создан")
			defer kafkaProducer.Close()
		}
	}

	// --- Kafka consumer (best-effort) ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if len(cfg.Kafka.Brokers) > 0 && kafkaProducer != nil {
		saramaConsumerConfig := sarama.NewConfig()
		saramaConsumerConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
		saramaConsumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
		saramaConsumerConfig.Consumer.Return.Errors = true
		saramaConsumerConfig.Version = sarama.V2_6_0_0

		consumerGroup, cgErr := sarama.NewConsumerGroup(cfg.Kafka.Brokers, "campaign-status-consumer", saramaConsumerConfig)
		if cgErr != nil {
			logger.Warn().Err(cgErr).Msg("не удалось создать Kafka consumer group, продолжаем без потребления статусов")
		} else {
			logger.Info().Msg("Kafka consumer group 'campaign-status-consumer' создан")
			defer consumerGroup.Close()

			// Start consuming sms.status in background
			go func() {
				statusTopic := cfg.Kafka.TopicStatus
				if statusTopic == "" {
					statusTopic = "sms.status"
				}
				for {
					select {
					case <-ctx.Done():
						return
					default:
						handler := &statusConsumerHandler{logger: logger}
						if err := consumerGroup.Consume(ctx, []string{statusTopic}, handler); err != nil {
							logger.Error().Err(err).Msg("ошибка потребления сообщений из Kafka")
						}
					}
				}
			}()
		}
	}

	// Repositories
	campaignRepo := campaignrepo.NewCampaignRepository(dbx)
	recipientRepo := campaignrepo.NewRecipientRepository(dbx)
	statsRepo := campaignrepo.NewStatsRepository(dbx)

	// Application service
	campaignService := application.NewCampaignService(campaignRepo, recipientRepo, statsRepo)

	// Materialization worker
	if kafkaProducer != nil {
		startMaterializationWorker(ctx, dbx, kafkaProducer, cfg.Kafka.TopicOutgoing, recipientRepo, logger)
		logger.Info().Msg("воркер материализации кампаний запущен")
	} else {
		logger.Warn().Msg("Kafka недоступна, воркер материализации отключён")
	}

	// Health checker
	healthChecker := monitoring.NewHealthChecker("campaign-service", cfg.Service.Version)
	healthChecker.SetDatabase(dbConn.DB)

	// gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	campaignGrpcServer := campaigngrpc.NewServer(campaignService)
	campaignv1.RegisterCampaignServiceServer(grpcServer, campaignGrpcServer)

	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
		logger.Info().Msg("gRPC reflection включен")
	}

	grpcListener, err := net.Listen("tcp", ":5013")
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
		Addr:         ":2131",
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

	cancel() // cancel background goroutines

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	grpcServer.GracefulStop()
	logger.Info().Msg("gRPC сервер остановлен")

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("ошибка при остановке HTTP сервера")
	}
	logger.Info().Msg("HTTP сервер остановлен")

	logger.Info().Msg("Campaign Service остановлен")
}

// statusConsumerHandler implements sarama.ConsumerGroupHandler for processing delivery status updates.
type statusConsumerHandler struct {
	logger zerolog.Logger
}

func (h *statusConsumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *statusConsumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *statusConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		// Placeholder: process delivery status updates from sms.status
		// In production, deserialize the message, update recipient status, and update campaign counters
		session.MarkMessage(msg, "")
	}
	return nil
}
