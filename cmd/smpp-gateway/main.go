package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
	smppgateway "github.com/smpp-server/smpp-server/internal/gateway/smpp"
	smppserver "github.com/smpp-server/smpp-server/internal/gateway/smpp/server"
)

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
	authServiceAddr := getEnvOrDefault("AUTH_SERVICE_ADDR", "localhost:9090")
	
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

	// Создание SMPP Gateway сервера
	smppGateway := smppserver.NewServer(
		&cfg.SMSP,
		serviceClients.AuthClient,
		messageRepo,
		producer,
		logger,
	)
	
	// Запуск сервера
	if err := smppGateway.Start(); err != nil {
		logger.Fatal().Err(err).Msg("ошибка запуска SMPP Gateway")
	}
	
	logger.Info().
		Str("addr", cfg.SMSP.GetAddr()).
		Msg("SMPP Gateway запущен и готов принимать соединения")

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

// getEnvOrDefault возвращает значение переменной окружения или значение по умолчанию
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
