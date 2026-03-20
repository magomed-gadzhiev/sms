package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	routingapp "github.com/smpp-server/smpp-server/internal/services/routing/application"
	routinggrpc "github.com/smpp-server/smpp-server/internal/services/routing/grpc"
	routingqueue "github.com/smpp-server/smpp-server/internal/services/routing/infrastructure/queue"
	routingrepo "github.com/smpp-server/smpp-server/internal/services/routing/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/jmoiron/sqlx"
)

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
	logger := shared.WithService("routing-service")

	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Routing Service")

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

	// Создаем sqlx.DB для использования в репозиториях
	dbx := sqlx.NewDb(dbConn.DB, "pgx")

	// Ожидание готовности Kafka
	logger.Info().Msg("ожидание готовности Kafka")
	if err := queue.WaitForKafka(&cfg.Kafka, 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("Kafka недоступен")
	}

	// Инициализация Kafka producer
	kafkaProducer, err := queue.NewProducer(&cfg.Kafka)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka producer")
	}
	defer kafkaProducer.Close()

	logger.Info().Msg("Kafka producer инициализирован")

	// Инициализация репозиториев
	routeRepo := routingrepo.NewRouteRepository(dbx)
	providerRepo := routingrepo.NewProviderRepository(dbx)
	countryRepo := routingrepo.NewCountryRepository(dbx)
	operatorRepo := routingrepo.NewOperatorRepository(dbx)
	operatorPrefixRepo := routingrepo.NewOperatorPrefixRepository(dbx)

	// Инициализация event publisher
	eventPublisher := routingqueue.NewEventPublisher(kafkaProducer)

	// Инициализация сервисов
	routingService := routingapp.NewRoutingService(routeRepo, providerRepo, eventPublisher)
	operatorResolver := routingapp.NewOperatorResolver(operatorPrefixRepo, operatorRepo, countryRepo)

	// Создаем handler для обработки сообщений из очереди после создания routing service
	messageHandler := func(ctx context.Context, kafkaMsg *queue.KafkaMessage) error {
		var clientID *uuid.UUID
		if kafkaMsg.ClientID != nil {
			clientID = kafkaMsg.ClientID
		}

		var routeID *uuid.UUID
		if kafkaMsg.RouteID != nil {
			routeID = kafkaMsg.RouteID
		}

		var providerID *uuid.UUID
		if kafkaMsg.ProviderID != nil {
			providerID = kafkaMsg.ProviderID
		}

		// Проверяем, не обработано ли уже сообщение
		if providerID != nil && routeID != nil {
			logger.Debug().
				Str("message_id", kafkaMsg.MessageID.String()).
				Msg("сообщение уже маршрутизировано, пропускаем")
			return nil
		}

		// Маршрутизируем сообщение
		selectedRouteID, selectedProviderID, err := routingService.RouteMessage(
			ctx,
			kafkaMsg.MessageID,
			kafkaMsg.Destination,
			clientID,
			routeID,
			providerID,
		)
		if err != nil {
			logger.Error().
				Err(err).
				Str("message_id", kafkaMsg.MessageID.String()).
				Str("destination", kafkaMsg.Destination).
				Msg("ошибка маршрутизации сообщения")
			return err
		}

		logger.Info().
			Str("message_id", kafkaMsg.MessageID.String()).
			Str("route_id", selectedRouteID.String()).
			Str("provider_id", selectedProviderID.String()).
			Msg("сообщение успешно маршрутизировано")

		return nil
	}

	// Инициализация Kafka consumer с handler
	kafkaConsumer, err := queue.NewConsumer(&cfg.Kafka, messageHandler, nil, nil)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka consumer")
	}
	defer kafkaConsumer.Close()

	logger.Info().Msg("Kafka consumer инициализирован")

	// Запуск Kafka consumer для обработки сообщений из очереди
	go func() {
		logger.Info().Msg("запуск Kafka consumer для обработки message.queued")
		if err := kafkaConsumer.ConsumeOutgoing(); err != nil {
			logger.Fatal().Err(err).Msg("ошибка запуска Kafka consumer")
		}
	}()

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("routing-service", cfg.Service.Version)
	healthChecker.SetDatabase(dbConn.DB)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	// Регистрация gRPC сервиса
	routingGrpcServer := routinggrpc.NewServer(routingService, countryRepo, operatorRepo, operatorPrefixRepo, operatorResolver)
	routingv1.RegisterRoutingServiceServer(grpcServer, routingGrpcServer)

	// Включение reflection для разработки
	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
		logger.Info().Msg("gRPC reflection включен")
	}

	// Запуск gRPC сервера
	grpcListener, err := net.Listen("tcp", ":9093") // Используем порт для routing service
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
		Addr:         ":2114", // Используем отдельный порт для метрик routing service
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

	logger.Info().Msg("Routing Service остановлен")
}