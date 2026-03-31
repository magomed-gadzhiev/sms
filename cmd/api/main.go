package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/smpp-server/smpp-server/api/proto/smsv1"
	grpcapi "github.com/smpp-server/smpp-server/internal/api/grpc"
	httphandler "github.com/smpp-server/smpp-server/internal/api/http"
	"github.com/smpp-server/smpp-server/internal/api/middleware"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
	logger := shared.WithService("api-gateway")

	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск API Gateway")

	// Ожидание готовности базы данных перед подключением
	logger.Info().Msg("ожидание готовности базы данных")
	if err := storage.WaitForDatabase(cfg.Database.GetDSN(), 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("база данных недоступна")
	}

	// Инициализация базы данных с настройками пула
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

	logger.Info().Msg("подключение к базе данных установлено")

	// Инициализация Redis для rate limiting
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

	// Проверка соединения с Redis
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Warn().Err(err).Msg("ошибка подключения к Redis (rate limiting может не работать)")
	} else {
		logger.Info().Msg("подключение к Redis установлено")
	}
	defer redisClient.Close()

	// Инициализация репозиториев
	clientRepo := storage.NewClientRepository(db)
	messageRepo := storage.NewMessageRepository(db)

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

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("api-gateway", cfg.Service.Version)
	healthChecker.SetDatabase(db.DB)
	if redisClient != nil {
		healthChecker.SetRedis(redisClient)
	}

	// Создание HTTP handler
	httpHandler := httphandler.NewHandler(producer, messageRepo, clientRepo, healthChecker)

	// Создание middleware
	authMiddleware := middleware.AuthMiddleware(clientRepo, &cfg.API.Auth)
	rateLimitMiddleware := middleware.RateLimitMiddleware(redisClient)
	loggingMiddleware := middleware.LoggingMiddleware(logger)
	recoveryMiddleware := middleware.RecoveryMiddleware()
	corsMiddleware := middleware.CORSMiddlewareFromConfig(os.Getenv("CORS_ALLOWED_ORIGINS"))
	tenantLoggerMiddleware := middleware.TenantLoggerMiddleware(logger)

	// Настройка HTTP роутера
	router := httphandler.SetupRouter(
		httpHandler,
		authMiddleware,
		rateLimitMiddleware,
		loggingMiddleware,
		recoveryMiddleware,
		corsMiddleware,
		tenantLoggerMiddleware,
	)

	// Добавляем Prometheus metrics endpoint
	if cfg.Monitoring.Prometheus.Enabled {
		router.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler()).Methods("GET")
		logger.Info().
			Str("path", cfg.Monitoring.Prometheus.Path).
			Msg("Prometheus metrics endpoint включен")
	}

	// Добавляем дополнительные health check endpoints
	router.HandleFunc("/health/live", healthChecker.LivenessHandler()).Methods("GET")
	router.HandleFunc("/health/ready", healthChecker.ReadinessHandler()).Methods("GET")

	// Создание HTTP сервера
	httpServer := &http.Server{
		Addr:         cfg.API.HTTP.GetAddr(),
		Handler:      router,
		ReadTimeout:  cfg.API.HTTP.ReadTimeout,
		WriteTimeout: cfg.API.HTTP.WriteTimeout,
		IdleTimeout:  cfg.API.HTTP.IdleTimeout,
	}

	// Запуск HTTP сервера
	go func() {
		logger.Info().
			Str("addr", cfg.API.HTTP.GetAddr()).
			Msg("HTTP сервер запущен")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("ошибка запуска HTTP сервера")
		}
	}()

	// Создание gRPC сервера
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcapi.TraceUnaryServerInterceptor(),
			grpcapi.AuthInterceptor(clientRepo, &cfg.API.Auth),
		),
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	// Регистрация gRPC сервиса
	grpcHandler := grpcapi.NewServer(producer, messageRepo, clientRepo)
	smsv1.RegisterSMSServiceServer(grpcServer, grpcHandler)

	// Включение reflection для разработки
	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
		logger.Info().Msg("gRPC reflection включен")
	}

	// Запуск gRPC сервера
	grpcListener, err := net.Listen("tcp", cfg.API.GRPC.GetAddr())
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC listener")
	}

	go func() {
		logger.Info().
			Str("addr", cfg.API.GRPC.GetAddr()).
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

	logger.Info().Msg("API Gateway остановлен")
}
