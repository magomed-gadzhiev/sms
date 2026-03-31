package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	pipelinepersist "github.com/smpp-server/smpp-server/internal/pipeline/persist"
	pipelinerouter "github.com/smpp-server/smpp-server/internal/pipeline/router"
	pipelinesender "github.com/smpp-server/smpp-server/internal/pipeline/sender"
	pipelinestatus "github.com/smpp-server/smpp-server/internal/pipeline/status"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	// Parse flags
	stage := flag.String("stage", "", "Pipeline stage: router, sender, status")
	workerID := flag.String("worker-id", "", "Unique worker ID")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	// Override stage from flag (flag > env > config)
	if *stage != "" {
		cfg.Pipeline.Stage = *stage
	}
	if cfg.Pipeline.Stage == "" {
		log.Fatal().Msg("не указана стадия pipeline (--stage=router|sender|status|persist)")
	}

	// Validate stage
	switch cfg.Pipeline.Stage {
	case "router", "sender", "status", "persist":
		// ok
	default:
		log.Fatal().Str("stage", cfg.Pipeline.Stage).Msg("неизвестная стадия pipeline")
	}

	// Init logger
	shared.InitLogger(cfg.Service.Env)

	// Determine worker ID
	wID := *workerID
	if wID == "" {
		wID = os.Getenv("SERVICE_NAME")
	}
	if wID == "" {
		hostname, _ := os.Hostname()
		wID = fmt.Sprintf("pipeline-%s-%s", cfg.Pipeline.Stage, hostname)
	}

	log.Info().
		Str("stage", cfg.Pipeline.Stage).
		Str("worker_id", wID).
		Str("version", cfg.Service.Version).
		Msg("запуск pipeline-worker")

	// Connect to database
	db, err := storage.NewDBWithConfig(
		cfg.Database.GetDSN(),
		cfg.Database.MaxOpenConns,
		cfg.Database.MaxIdleConns,
		cfg.Database.ConnMaxLifetime,
		cfg.Database.ConnMaxIdleTime,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("ошибка подключения к базе данных")
	}
	defer db.Close()

	// Create pgx pool for COPY-based persist stage
	pgxConnStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&default_query_exec_mode=simple_protocol",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.Database, cfg.Database.SSLMode)
	pgxPool, err := pgxpool.New(context.Background(), pgxConnStr)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create pgx pool")
	}
	defer pgxPool.Close()

	// Connect to Redis (for sender stage limits cache + pub/sub)
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.GetAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Wait for Kafka
	log.Info().Msg("ожидание готовности Kafka брокеров")
	if err := queue.WaitForKafka(&cfg.Kafka, 30, 2*time.Second); err != nil {
		log.Fatal().Err(err).Msg("Kafka брокеры недоступны")
	}

	// Auto-create pipeline Kafka topics
	ensureTopics(&cfg.Kafka)

	// Health checker
	healthChecker := monitoring.NewHealthChecker(wID, cfg.Service.Version)
	healthChecker.SetDatabase(db.DB)

	// Metrics HTTP server
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/health", healthChecker.Handler())
	metricsMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	metricsMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())
	if cfg.Monitoring.Prometheus.Enabled {
		metricsMux.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler())
	}

	metricsServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Monitoring.MetricsPort),
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		log.Info().Str("addr", metricsServer.Addr).Msg("HTTP сервер для metrics запущен")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("ошибка запуска HTTP сервера для metrics")
		}
	}()

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start consumer lag monitor for pipeline consumer groups
	monitoring.StartConsumerLagMonitor(ctx, cfg.Kafka.Brokers,
		[]string{"pipeline-router", "pipeline-sender", "pipeline-status", "pipeline-persist"},
		15*time.Second,
	)

	// Dispatch to selected stage
	go func() {
		var stageErr error
		switch cfg.Pipeline.Stage {
		case "router":
			stageErr = runRouterStage(ctx, cfg, db)
		case "sender":
			stageErr = runSenderStage(ctx, cfg, db, rdb)
		case "status":
			stageErr = runStatusStage(ctx, cfg, db, pgxPool)
		case "persist":
			stageErr = runPersistStage(ctx, cfg, pgxPool)
		}
		if stageErr != nil {
			log.Error().Err(stageErr).Str("stage", cfg.Pipeline.Stage).Msg("ошибка стадии pipeline")
			cancel()
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("получен сигнал остановки")
	case <-ctx.Done():
		log.Info().Msg("context cancelled, shutting down")
	}

	// Graceful shutdown
	cancel()

	// Drain timeout: allow in-flight messages to complete
	log.Info().Msg("ожидание завершения in-flight сообщений (drain timeout 10s)")
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer drainCancel()

	// Wait for drain timeout or until context expires
	// Stage goroutine will exit because ctx is cancelled;
	// deferred stage.Close() calls will wait for consumer goroutines to finish.
	<-drainCtx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("ошибка остановки HTTP сервера")
	}

	log.Info().Str("stage", cfg.Pipeline.Stage).Msg("pipeline-worker остановлен")
}

func runRouterStage(ctx context.Context, cfg *config.Config, db *storage.DB) error {
	stage, err := pipelinerouter.NewStage(cfg, db)
	if err != nil {
		return fmt.Errorf("ошибка создания router stage: %w", err)
	}
	defer stage.Close()
	return stage.Run(ctx)
}

func runSenderStage(ctx context.Context, cfg *config.Config, db *storage.DB, rdb *redis.Client) error {
	stage, err := pipelinesender.NewStage(cfg, db, rdb)
	if err != nil {
		return fmt.Errorf("ошибка создания sender stage: %w", err)
	}
	defer stage.Close()
	return stage.Run(ctx)
}

func runStatusStage(ctx context.Context, cfg *config.Config, db *storage.DB, pgxPool *pgxpool.Pool) error {
	stage, err := pipelinestatus.NewStage(cfg, db, pgxPool)
	if err != nil {
		return fmt.Errorf("ошибка создания status stage: %w", err)
	}
	defer stage.Close()
	return stage.Run(ctx)
}

func runPersistStage(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) error {
	stage, err := pipelinepersist.NewStage(cfg, pool)
	if err != nil {
		return fmt.Errorf("ошибка создания persist stage: %w", err)
	}
	defer stage.Close()
	return stage.Run(ctx)
}

// ensureTopics creates pipeline Kafka topics on startup if they don't already exist.
func ensureTopics(cfg *config.KafkaConfig) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V2_6_0_0

	admin, err := sarama.NewClusterAdmin(cfg.Brokers, saramaConfig)
	if err != nil {
		log.Error().Err(err).Msg("не удалось создать Kafka ClusterAdmin для auto-creation топиков")
		return
	}
	defer admin.Close()

	topics := []struct {
		name       string
		partitions int32
	}{
		{name: cfg.TopicOutgoing, partitions: 32},
		{name: cfg.TopicRouted, partitions: 32},
		{name: cfg.TopicSent, partitions: 16},
		{name: cfg.TopicDLR, partitions: 16},
		{name: cfg.TopicStatus, partitions: 16},
		{name: cfg.TopicFailed, partitions: 8},
	}

	for _, t := range topics {
		log.Info().Str("topic", t.name).Int32("partitions", t.partitions).Msg("создание Kafka топика")

		err := admin.CreateTopic(t.name, &sarama.TopicDetail{
			NumPartitions:     t.partitions,
			ReplicationFactor: 1,
		}, false)
		if err != nil {
			if errors.Is(err, sarama.ErrTopicAlreadyExists) {
				log.Info().Str("topic", t.name).Msg("топик уже существует")
				continue
			}
			log.Error().Err(err).Str("topic", t.name).Msg("ошибка создания Kafka топика")
			continue
		}

		log.Info().Str("topic", t.name).Msg("Kafka топик создан успешно")
	}
}
