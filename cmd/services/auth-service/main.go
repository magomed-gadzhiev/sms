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
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/redis/go-redis/v9"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/services/auth/application"
	authgrpc "github.com/smpp-server/smpp-server/internal/services/auth/grpc"
	authinfra "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
	logger := shared.WithService("auth-service")

	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Auth Service")

	// Ожидание готовности базы данных
	logger.Info().Msg("ожидание готовности базы данных")
	if err := storage.WaitForDatabase(cfg.Database.GetDSN(), 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("база данных недоступна")
	}

	// Инициализация базы данных
	db, err := database.NewDBWithConfig(database.Config{
		DSN:             cfg.Database.GetDSN(),
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к базе данных")
	}
	defer db.Close()

	logger.Info().Msg("подключение к базе данных установлено")

	// Инициализация репозиториев
	userRepo := authrepo.NewUserRepository(db)
	roleRepo := authrepo.NewRoleRepository(db)
	apiKeyRepo := authrepo.NewAPIKeyRepository(db)
	refreshTokenRepo := authrepo.NewRefreshTokenRepository(db)

	// Инициализация сервисов
	tokenExpiry := cfg.API.Auth.TokenExpiry
	if tokenExpiry == 0 {
		tokenExpiry = 24 * time.Hour
	}

	jwtSecret := cfg.API.Auth.JWTSecret
	if jwtSecret == "" {
		logger.Warn().Msg("JWT секрет не установлен, используем значение по умолчанию (НЕБЕЗОПАСНО для production)")
		jwtSecret = "default-secret-change-in-production"
	}

	tokenService := application.NewTokenService(
		jwtSecret,
		tokenExpiry,
		7*24*time.Hour, // Refresh токены на 7 дней
		refreshTokenRepo,
	)

	authService := application.NewAuthService(
		userRepo,
		apiKeyRepo,
		tokenService,
	)

	// Password hasher
	passwordHasher := &authinfra.PasswordHasherImpl{}

	// Инициализация TOTP репозитория и сервиса
	totpRepo := authrepo.NewTOTPRepository(db)
	encryptionKeyStr := os.Getenv("TOTP_ENCRYPTION_KEY")
	if encryptionKeyStr == "" {
		encryptionKeyStr = "default-encryption-key-32bytes!!"
	}
	totpService := application.NewTOTPService(totpRepo, userRepo, passwordHasher, []byte(encryptionKeyStr))

	// Инициализация Password Reset репозитория и сервиса
	passwordResetRepo := authrepo.NewPasswordResetRepository(db)
	passwordResetService := application.NewPasswordResetService(passwordResetRepo, userRepo, passwordHasher)

	// Инициализация Redis и Session Manager
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
	})
	defer redisClient.Close()

	sessionManager := authinfra.NewSessionManager(redisClient, db, 10)

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("auth-service", cfg.Service.Version)
	healthChecker.SetDatabase(db.DB)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	// Регистрация gRPC сервиса
	authGrpcServer := authgrpc.NewServer(
		authService,
		tokenService,
		totpService,
		passwordResetService,
		sessionManager,
		userRepo,
		roleRepo,
	)
	authv1.RegisterAuthServiceServer(grpcServer, authGrpcServer)

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
		Addr:         ":2112", // Используем стандартный порт для метрик
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

	logger.Info().Msg("Auth Service остановлен")
}
