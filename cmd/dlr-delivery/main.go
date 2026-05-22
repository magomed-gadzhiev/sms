package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	smppv1 "github.com/smpp-server/smpp-server/api/proto/smppv1"
	"github.com/smpp-server/smpp-server/internal/config"
	smppserver "github.com/smpp-server/smpp-server/internal/gateway/smpp/server"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/pipeline"
	"github.com/smpp-server/smpp-server/internal/queue"
	dlr "github.com/smpp-server/smpp-server/internal/services/dlr"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/cache"
)

func main() {
	// Init logger
	shared.InitLogger("development")
	logger := shared.WithService("dlr-delivery")

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load configuration")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("starting dlr-delivery service")

	// Connect to Redis
	redisCache, err := cache.NewCache(&cache.Config{
		Host:         cfg.Redis.Host,
		Port:         cfg.Redis.Port,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer redisCache.Close()
	logger.Info().Msg("Redis connected")

	// Create RedisStore with cache adapter (cache.Cache uses Delete, RedisClient needs Del)
	redisAdapter := &cacheAdapter{cache: redisCache}
	redisStore := smppserver.NewRedisStore(redisAdapter, 24*time.Hour, 2*time.Hour)

	// Connect to smpp-gateway gRPC
	gatewayAddr := config.EnvOrDefault("SMPP_GATEWAY_GRPC_ADDR", "smpp-gateway:9095")
	grpcConn, err := grpc.Dial(gatewayAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal().Err(err).Str("addr", gatewayAddr).Msg("failed to connect to smpp-gateway gRPC")
	}
	defer grpcConn.Close()
	logger.Info().Str("addr", gatewayAddr).Msg("gRPC connection to smpp-gateway established")

	// Create gRPC client adapter
	smppClient := smppv1.NewSMPPGatewayClient(grpcConn)
	grpcAdapter := &grpcClientAdapter{client: smppClient}

	// Create DLR Dispatcher and Processor
	dispatcher := dlr.NewDispatcher(grpcAdapter, logger)
	processor := dlr.NewProcessor(redisStore, dispatcher, logger)

	// Wait for Kafka
	logger.Info().Msg("waiting for Kafka brokers")
	if err := queue.WaitForKafka(&cfg.Kafka, 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("Kafka brokers unavailable")
	}

	// Create Kafka consumer group
	saramaConfig := sarama.NewConfig()
	saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	saramaConfig.Consumer.Return.Errors = true
	saramaConfig.Version = sarama.V2_6_0_0

	consumerGroup, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, "dlr-delivery", saramaConfig)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create Kafka consumer group")
	}
	defer consumerGroup.Close()
	logger.Info().Msg("Kafka consumer group created")

	// Health checker
	healthChecker := monitoring.NewHealthChecker("dlr-delivery", cfg.Service.Version)

	// HTTP server for health checks + Prometheus metrics
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/health", healthChecker.Handler())
	metricsMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	metricsMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())
	if cfg.Monitoring.Prometheus.Enabled {
		metricsMux.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler())
		logger.Info().
			Str("path", cfg.Monitoring.Prometheus.Path).
			Msg("Prometheus metrics endpoint enabled")
	}

	metricsServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Monitoring.MetricsPort),
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		logger.Info().Str("addr", metricsServer.Addr).Msg("HTTP metrics server started")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("HTTP metrics server error")
		}
	}()

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Consume Kafka messages
	handler := &dlrConsumerHandler{processor: processor, logger: logger}

	go func() {
		for {
			if err := consumerGroup.Consume(ctx, []string{cfg.Kafka.TopicStatus}, handler); err != nil {
				logger.Error().Err(err).Msg("Kafka consumer error")
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	// Forward consumer errors to log
	go func() {
		for err := range consumerGroup.Errors() {
			logger.Error().Err(err).Msg("Kafka consumer group error")
		}
	}()

	logger.Info().
		Str("topic", cfg.Kafka.TopicStatus).
		Msg("dlr-delivery service started, consuming messages")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	case <-ctx.Done():
		logger.Info().Msg("context cancelled")
	}

	// Graceful shutdown
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("HTTP metrics server shutdown error")
	}

	logger.Info().Msg("dlr-delivery service stopped")
}

// cacheAdapter adapts cache.Cache to the RedisClient interface expected by RedisStore.
// cache.Cache uses Delete() while RedisClient expects Del().
type cacheAdapter struct {
	cache *cache.Cache
}

func (a *cacheAdapter) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return a.cache.Set(ctx, key, value, expiration)
}

func (a *cacheAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.cache.Get(ctx, key)
}

func (a *cacheAdapter) Del(ctx context.Context, keys ...string) error {
	return a.cache.Delete(ctx, keys...)
}

func (a *cacheAdapter) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return a.cache.Expire(ctx, key, expiration)
}

// grpcClientAdapter adapts smppv1.SMPPGatewayClient to the dlr.GRPCClient interface.
type grpcClientAdapter struct {
	client smppv1.SMPPGatewayClient
}

func (a *grpcClientAdapter) DeliverDLR(ctx context.Context, systemID, sourceAddr, destAddr, receipt string) (delivered bool, errMsg string, err error) {
	resp, err := a.client.DeliverDLR(ctx, &smppv1.DeliverDLRRequest{
		SystemId:        systemID,
		SourceAddr:      sourceAddr,
		DestinationAddr: destAddr,
		ReceiptText:     receipt,
	})
	if err != nil {
		return false, "", err
	}
	return resp.GetDelivered(), resp.GetError(), nil
}

// dlrConsumerHandler implements sarama.ConsumerGroupHandler for DLR processing.
type dlrConsumerHandler struct {
	processor *dlr.Processor
	logger    zerolog.Logger
}

func (h *dlrConsumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *dlrConsumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *dlrConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		var statusUpdate pipeline.StatusUpdate
		if err := json.Unmarshal(msg.Value, &statusUpdate); err != nil {
			h.logger.Error().Err(err).
				Int64("offset", msg.Offset).
				Int32("partition", msg.Partition).
				Msg("failed to deserialize StatusUpdate")
			session.MarkMessage(msg, "")
			continue
		}

		if err := h.processor.ProcessStatusUpdate(session.Context(), &statusUpdate); err != nil {
			h.logger.Error().Err(err).
				Str("message_id", statusUpdate.MessageID.String()).
				Msg("failed to process StatusUpdate")
			// Still mark message to avoid infinite reprocessing of poison messages
		}

		session.MarkMessage(msg, "")
	}
	return nil
}
