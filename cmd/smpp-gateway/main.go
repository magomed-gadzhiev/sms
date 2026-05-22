package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	smppv1 "github.com/smpp-server/smpp-server/api/proto/smppv1"
	"github.com/smpp-server/smpp-server/internal/config"
	smppgateway "github.com/smpp-server/smpp-server/internal/gateway/smpp"
	smppserver "github.com/smpp-server/smpp-server/internal/gateway/smpp/server"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/cache"
	"github.com/smpp-server/smpp-server/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// cacheRedisAdapter adapts cache.Cache to the smppserver.RedisClient interface.
// cache.Cache uses Delete(ctx, keys...) while RedisClient expects Del(ctx, keys...).
type cacheRedisAdapter struct {
	cache *cache.Cache
}

func (a *cacheRedisAdapter) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return a.cache.Set(ctx, key, value, expiration)
}

func (a *cacheRedisAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.cache.Get(ctx, key)
}

func (a *cacheRedisAdapter) Del(ctx context.Context, keys ...string) error {
	return a.cache.Delete(ctx, keys...)
}

func (a *cacheRedisAdapter) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return a.cache.Expire(ctx, key, expiration)
}

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
	logger := shared.WithService("smpp-gateway")
	
	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}
	
	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск SMPP Gateway")
	
	// Получение адреса Auth Service из переменных окружения или использование значения по умолчанию
	authServiceAddr := config.EnvOrDefault("AUTH_SERVICE_ADDR", "localhost:9090")
	
	// Инициализация gRPC клиентов
	serviceAddresses := smppgateway.ServiceAddresses{
		Auth: authServiceAddr,
	}
	
	serviceClients, err := smppgateway.NewServiceClients(serviceAddresses)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC клиентов")
	}
	defer serviceClients.Close()
	
	logger.Info().
		Str("auth_service", authServiceAddr).
		Msg("gRPC клиенты инициализированы")
	
	// Ожидание готовности Kafka перед инициализацией producer
	logger.Info().Msg("ожидание готовности Kafka брокеров")
	if err := queue.WaitForKafka(&cfg.Kafka, 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("Kafka брокеры недоступны")
	}
	
	// Инициализация Kafka producer
	producer, err := queue.NewProducer(&cfg.Kafka)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka producer")
	}
	defer producer.Close()
	
	logger.Info().Msg("Kafka producer инициализирован")

	// Инициализация базы данных для message repository
	db, err := storage.NewDBWithConfig(
		cfg.Database.GetDSN(),
		cfg.Database.MaxOpenConns,
		cfg.Database.MaxIdleConns,
		cfg.Database.ConnMaxLifetime,
		cfg.Database.ConnMaxIdleTime,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к базе данных")
	}
	defer db.Close()

	messageRepo := storage.NewMessageRepository(db)
	optOutRepo := storage.NewOptOutRepository(db)

	// Redis для DLR маппинга
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
		logger.Fatal().Err(err).Msg("ошибка подключения к Redis")
	}
	defer redisCache.Close()

	redisStore := smppserver.NewRedisStore(&cacheRedisAdapter{cache: redisCache}, 24*time.Hour, 5*time.Minute)

	// Создание SMPP Gateway сервера
	smppGateway := smppserver.NewServer(
		&cfg.SMSP,
		serviceClients.AuthClient,
		messageRepo,
		optOutRepo,
		producer,
		redisStore,
		logger,
	)
	
	// Запуск сервера
	if err := smppGateway.Start(); err != nil {
		logger.Fatal().Err(err).Msg("ошибка запуска SMPP Gateway")
	}
	
	logger.Info().
		Str("addr", cfg.SMSP.GetAddr()).
		Msg("SMPP Gateway запущен и готов принимать соединения")

	// Запуск internal gRPC сервера для DLR доставки
	grpcInternalPort := config.EnvOrDefault("GRPC_INTERNAL_PORT", "9095")
	grpcListener, err := net.Listen("tcp", ":"+grpcInternalPort)
	if err != nil {
		logger.Fatal().Err(err).Str("port", grpcInternalPort).Msg("ошибка запуска internal gRPC listener")
	}

	grpcServer := grpc.NewServer()
	dlrGRPCServer := smppserver.NewGRPCServerFromServer(smppGateway)
	smppv1.RegisterSMPPGatewayServer(grpcServer, dlrGRPCServer)
	reflection.Register(grpcServer)

	go func() {
		logger.Info().Str("port", grpcInternalPort).Msg("internal gRPC сервер запущен")
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Error().Err(err).Msg("ошибка internal gRPC сервера")
		}
	}()

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("smpp-gateway", cfg.Service.Version)

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
		Addr:         fmt.Sprintf(":%d", cfg.Monitoring.MetricsPort),
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Запуск HTTP сервера для metrics
	go func() {
		logger.Info().
			Str("addr", metricsServer.Addr).
			Msg("HTTP сервер для metrics запущен")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("ошибка запуска HTTP сервера для metrics")
		}
	}()

	// Ожидание сигнала для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	<-sigChan
	logger.Info().Msg("получен сигнал остановки")
	
	// Остановка internal gRPC сервера
	grpcServer.GracefulStop()

	// Остановка сервера
	if err := smppGateway.Stop(); err != nil {
		logger.Error().Err(err).Msg("ошибка остановки SMPP Gateway")
		os.Exit(1)
	}

	// Остановка HTTP сервера для metrics
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.DefaultGracefulShutdownTimeout)
	defer shutdownCancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("ошибка остановки HTTP сервера для metrics")
	}

	logger.Info().Msg("SMPP Gateway остановлен")
}

