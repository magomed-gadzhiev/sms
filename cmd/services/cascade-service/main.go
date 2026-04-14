package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	cascadeapp "github.com/smpp-server/smpp-server/internal/services/cascade/application"
	"github.com/smpp-server/smpp-server/internal/services/cascade/channels/flashcall"
	"github.com/smpp-server/smpp-server/internal/services/cascade/channels/maxmessenger"
	"github.com/smpp-server/smpp-server/internal/services/cascade/channels/sms"
	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	cascadegrpc "github.com/smpp-server/smpp-server/internal/services/cascade/grpc"
	cascadekafka "github.com/smpp-server/smpp-server/internal/services/cascade/infrastructure/kafka"
	cascadepg "github.com/smpp-server/smpp-server/internal/services/cascade/infrastructure/postgres"
	"github.com/smpp-server/smpp-server/internal/shared"
)

func main() {
	shared.InitLogger("development")
	logger := shared.WithService("cascade-service")

	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Cascade Service")

	// PostgreSQL (pgxpool)
	pgDSN := cfg.Database.GetDSN()
	poolCfg, err := pgxpool.ParseConfig(pgDSN)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка парсинга DSN")
	}
	poolCfg.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.Database.MaxIdleConns)

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к PostgreSQL")
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal().Err(err).Msg("PostgreSQL недоступен")
	}
	logger.Info().Msg("подключение к PostgreSQL установлено")

	// Redis
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
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal().Err(err).Msg("Redis недоступен")
	}
	logger.Info().Msg("подключение к Redis установлено")

	// Kafka producer
	saramaCfg := sarama.NewConfig()
	saramaCfg.Producer.Return.Successes = true
	saramaCfg.Producer.Return.Errors = true
	saramaCfg.Producer.RequiredAcks = sarama.WaitForAll
	saramaCfg.Producer.Retry.Max = cfg.Kafka.MaxRetries
	saramaCfg.Producer.Retry.Backoff = cfg.Kafka.RetryBackoff
	saramaCfg.Producer.Compression = sarama.CompressionSnappy
	saramaCfg.Producer.Idempotent = true
	saramaCfg.Net.MaxOpenRequests = 1

	syncProducer, err := sarama.NewSyncProducer(cfg.Kafka.Brokers, saramaCfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka producer")
	}
	defer syncProducer.Close()
	logger.Info().Msg("Kafka producer инициализирован")

	// Kafka topics
	topics := cascadekafka.CascadeTopics{
		Start:         getEnv("KAFKA_TOPIC_CASCADE_START", "cascade.start"),
		AttemptSend:   getEnv("KAFKA_TOPIC_CASCADE_ATTEMPT_SEND", "cascade.attempt.send"),
		AttemptResult: getEnv("KAFKA_TOPIC_CASCADE_ATTEMPT_RESULT", "cascade.attempt.result"),
		Billing:       getEnv("KAFKA_TOPIC_CASCADE_BILLING", "cascade.billing"),
	}

	cascadeProducer := cascadekafka.NewCascadeProducer(syncProducer, topics)

	// Repositories
	channelRepo := cascadepg.NewChannelRepository(pool)
	strategyRepo := cascadepg.NewStrategyRepository(pool)
	deliveryRepo := cascadepg.NewDeliveryRepository(pool)
	attemptRepo := cascadepg.NewAttemptRepository(pool)
	operatorSupportRepo := cascadepg.NewOperatorSupportRepository(pool)

	// Channel adapters
	smsAdapter := sms.NewAdapter(cascadeProducer)
	flashCallAdapter := flashcall.NewAdapter(logger)
	maxMessengerMetrics := maxmessenger.NewMaxMessengerMetrics()
	maxMessengerAdapter := maxmessenger.NewAdapter(logger, maxMessengerMetrics)

	channelAdapters := map[domain.ChannelType]domain.Channel{
		domain.ChannelSMS:            smsAdapter,
		domain.ChannelFlashCall:      flashCallAdapter,
		domain.ChannelMaxMessenger:   maxMessengerAdapter,
	}

	// Application services
	cascadeService := cascadeapp.NewCascadeService(
		deliveryRepo,
		attemptRepo,
		strategyRepo,
		channelRepo,
		cascadeProducer,
		channelAdapters,
		logger,
	)

	channelService := cascadeapp.NewChannelService(channelRepo)
	strategyService := cascadeapp.NewStrategyService(strategyRepo, deliveryRepo)
	deliveryService := cascadeapp.NewDeliveryService(deliveryRepo, attemptRepo)

	// Scheduler (timeout poller)
	schedulerInterval := 5 * time.Second
	if v := os.Getenv("CASCADE_SCHEDULER_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			schedulerInterval = d
		}
	}
	scheduler := cascadeapp.NewScheduler(attemptRepo, strategyRepo, cascadeProducer, schedulerInterval, logger)
	scheduler.Start()

	// Kafka orchestrator consumer
	consumerGroup := getEnv("KAFKA_CASCADE_CONSUMER_GROUP", "cascade-orchestrator")
	orchestrator, err := cascadekafka.NewOrchestrator(
		cfg.Kafka.Brokers,
		consumerGroup,
		topics,
		cascadeService,
		logger,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka orchestrator")
	}
	orchestrator.Start(ctx)

	// gRPC connections to external services
	routingAddr := getEnv("ROUTING_SERVICE_ADDR", "routing-service:9093")
	tarificationAddr := getEnv("TARIFICATION_SERVICE_ADDR", "tarification-service:9100")
	billingAddr := getEnv("BILLING_SERVICE_ADDR", "billing-service:9097")

	routingConn, err := grpc.NewClient(routingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal().Err(err).Str("addr", routingAddr).Msg("ошибка подключения к routing-service")
	}
	defer routingConn.Close()

	tarificationConn, err := grpc.NewClient(tarificationAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal().Err(err).Str("addr", tarificationAddr).Msg("ошибка подключения к tarification-service")
	}
	defer tarificationConn.Close()

	billingConn, err := grpc.NewClient(billingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal().Err(err).Str("addr", billingAddr).Msg("ошибка подключения к billing-service")
	}
	defer billingConn.Close()

	// Reachability service
	reachabilityService := cascadeapp.NewReachabilityService(
		operatorSupportRepo,
		redisClient,
		routingConn,
		logger,
	)

	// Billing integration
	billingIntegration := cascadeapp.NewBillingIntegration(
		tarificationConn,
		billingConn,
		attemptRepo,
		deliveryRepo,
		logger,
	)

	// Billing consumer
	billingConsumer, err := cascadekafka.NewBillingConsumer(
		cfg.Kafka.Brokers,
		getEnv("KAFKA_CASCADE_BILLING_GROUP", "cascade-billing"),
		topics.Billing,
		billingIntegration,
		logger,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания billing consumer")
	}
	billingConsumer.Start(ctx)

	// Inject Max Messenger reachability checker
	maxMessengerReachability := maxmessenger.NewReachabilityChecker(logger, maxMessengerMetrics)
	reachabilityService.SetMaxMessengerChecker(maxMessengerReachability, channelRepo)

	// Inject reachability into cascade service
	cascadeService.SetReachabilityService(reachabilityService)

	// Health checker
	healthChecker := monitoring.NewHealthChecker("cascade-service", cfg.Service.Version)
	healthChecker.SetRedis(redisClient)

	// gRPC server
	grpcPort := getEnvInt("GRPC_PORT", 9110)
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	cascadeGrpcServer := cascadegrpc.NewServer(cascadeService, channelService, strategyService, deliveryService, operatorSupportRepo)
	cascadeGrpcServer.Register(grpcServer)

	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
		logger.Info().Msg("gRPC reflection включен")
	}

	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC listener")
	}

	go func() {
		logger.Info().Str("addr", grpcListener.Addr().String()).Msg("gRPC сервер запущен")
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatal().Err(err).Msg("ошибка запуска gRPC сервера")
		}
	}()

	// HTTP server for health + metrics
	metricsPort := getEnvInt("METRICS_PORT", 2132)
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/health", healthChecker.Handler())
	metricsMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	metricsMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())

	// Max Messenger webhook handler
	maxMessengerWebhook := maxmessenger.NewWebhookHandler(channelRepo, attemptRepo, cascadeProducer, maxMessengerMetrics, logger)
	metricsMux.HandleFunc("/webhooks/cascade/max_messenger", maxMessengerWebhook.Handle)

	if cfg.Monitoring.Prometheus.Enabled {
		metricsMux.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler())
		logger.Info().Str("path", cfg.Monitoring.Prometheus.Path).Msg("Prometheus metrics endpoint включен")
	}

	metricsServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", metricsPort),
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scheduler.Stop()
	logger.Info().Msg("scheduler остановлен")

	orchestrator.Stop()
	logger.Info().Msg("orchestrator остановлен")

	billingConsumer.Stop()
	logger.Info().Msg("billing consumer остановлен")

	grpcServer.GracefulStop()
	logger.Info().Msg("gRPC сервер остановлен")

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("ошибка при остановке HTTP сервера")
	}
	logger.Info().Msg("HTTP сервер остановлен")

	logger.Info().Msg("Cascade Service остановлен")
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
