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

	providerv1 "github.com/smpp-server/smpp-server/api/proto/providerv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/services/provider/application"
	providergrpc "github.com/smpp-server/smpp-server/internal/services/provider/grpc"
	providerrepo "github.com/smpp-server/smpp-server/internal/services/provider/infrastructure/repository"
	providersmpp "github.com/smpp-server/smpp-server/internal/services/provider/infrastructure/smpp"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
	logger := shared.WithService("provider-service")

	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Provider Service")

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

	// Создаем storage.DB для использования в репозиториях
	storageDB := &storage.DB{DB: dbConn.DB}

	// Инициализация репозиториев
	storageProviderRepo := storage.NewProviderRepository(storageDB)
	providerRepo := providerrepo.NewProviderRepositoryAdapter(storageProviderRepo)

	// Инициализация SMSC connection pool
	poolAdapter := providersmpp.NewPoolAdapter(&cfg.Worker)
	defer poolAdapter.CloseAll()

	// Инициализация сервисов
	providerService := application.NewProviderService(providerRepo)
	senderService := application.NewSenderService(poolAdapter.GetSender())

	// Инициализация соединений к активным провайдерам (мигрировано из worker)
	ctx := context.Background()
	providers, _, err := providerRepo.List(ctx, true, 1000, 0)
	if err == nil && len(providers) > 0 {
		logger.Info().Int("count", len(providers)).Msg("инициализация соединений к провайдерам")
		for _, provider := range providers {
			if provider.Active && provider.MaxConnections > 0 {
				// Connect создаст все необходимые соединения согласно MaxConnections
				if err := poolAdapter.Connect(ctx, provider); err != nil {
					logger.Error().
						Err(err).
						Str("provider_id", provider.ID.String()).
						Str("provider_name", provider.Name).
						Msg("ошибка подключения к провайдеру")
				} else {
					logger.Info().
						Str("provider_id", provider.ID.String()).
						Str("provider_name", provider.Name).
						Int("max_connections", provider.MaxConnections).
						Msg("соединения к провайдеру инициализированы")
				}
			}
		}
	} else if err != nil {
		logger.Warn().Err(err).Msg("ошибка получения списка провайдеров при инициализации")
	}

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("provider-service", cfg.Service.Version)
	healthChecker.SetDatabase(dbConn.DB)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	// Регистрация gRPC сервиса
	// Преобразуем PoolAdapter в ConnectionPoolService интерфейс
	var connectionPoolService application.ConnectionPoolService = poolAdapter
	providerGrpcServer := providergrpc.NewServer(providerService, connectionPoolService, senderService)
	providerv1.RegisterProviderServiceServer(grpcServer, providerGrpcServer)

	// Включение reflection для разработки
	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
		logger.Info().Msg("gRPC reflection включен")
	}

	// Запуск gRPC сервера
	grpcListener, err := net.Listen("tcp", ":9093") // Используем порт для provider service
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
		Addr:         ":2114", // Используем отдельный порт для метрик provider service
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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Закрытие connection pool
	if err := poolAdapter.CloseAll(); err != nil {
		logger.Error().Err(err).Msg("ошибка закрытия connection pool")
	}
	logger.Info().Msg("connection pool закрыт")

	// Остановка gRPC сервера
	grpcServer.GracefulStop()
	logger.Info().Msg("gRPC сервер остановлен")

	// Остановка HTTP сервера
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("ошибка при остановке HTTP сервера")
	}
	logger.Info().Msg("HTTP сервер остановлен")

	logger.Info().Msg("Provider Service остановлен")
}
