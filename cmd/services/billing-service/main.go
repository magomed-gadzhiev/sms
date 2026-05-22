package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpcapi "github.com/smpp-server/smpp-server/internal/api/grpc"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/billing/application"
	billinggrpc "github.com/smpp-server/smpp-server/internal/services/billing/grpc"
	billingrepo "github.com/smpp-server/smpp-server/internal/services/billing/infrastructure/repository"
	billingqueue "github.com/smpp-server/smpp-server/internal/services/billing/infrastructure/queue"
	tarrepo "github.com/smpp-server/smpp-server/internal/services/tarification/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	// Инициализация логгера
	shared.InitLogger("development")
	logger := shared.WithService("billing-service")

	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Billing Service")

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

	// Инициализация репозиториев
	accountRepo := billingrepo.NewAccountRepository(dbx)
	transactionRepo := billingrepo.NewTransactionRepository(dbx)
	pricingRepo := billingrepo.NewPricingRuleRepository(dbx)

	// Репозитории тарификации, нужные для атомарного dual-charge flow (квота + margin log).
	quotaRepo := tarrepo.NewAggregatorQuotaRepository(dbx)
	marginLogRepo := tarrepo.NewAggregatorMarginLogRepository(dbx)
	// Idempotency guard для dual-charge — non-partitioned, solo-PK=message_id.
	commitGuardRepo := billingrepo.NewCommitIdempotencyGuardRepository(dbx)

	// Инициализация Kafka producer для публикации событий биллинга
	// Используем дефолтные топики или создаем новые для billing событий
	balanceTopic := "billing.balance"
	transactionTopic := "billing.transactions"
	
	billingEventPublisher, err := billingqueue.NewEventPublisher(&cfg.Kafka, balanceTopic, transactionTopic)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka event publisher")
	}
	defer billingEventPublisher.Close()

	// Инициализация сервисов
	billingService := application.NewBillingService(accountRepo, transactionRepo, billingEventPublisher)
	pricingService := application.NewPricingService(pricingRepo)

	// Зависимости для атомарного ChargeMessageDual (dual-списание + квота + margin log в одной tx).
	dualDeps := application.DualChargeDeps{
		DB:              dbx,
		Guard:           commitGuardRepo,
		QuotaRepo:       quotaRepo,
		MarginLogRepo:   marginLogRepo,
		AccountRepo:     accountRepo,
		TransactionRepo: transactionRepo,
		EventPublisher:  billingEventPublisher,
	}

	// Инициализация Kafka consumer
	// Используем уникальный consumer group для billing service
	kafkaCfg := cfg.Kafka
	kafkaCfg.ConsumerGroup = "billing-service"

	// Обработчик DLR — тарификация доставленных сообщений.
	dlrHandler := func(ctx context.Context, dlr *queue.DLRMessage) error {
		if dlr.Stat != "DELIVRD" || dlr.ClientID == nil {
			logger.Debug().
				Str("message_id", dlr.MessageID.String()).
				Str("stat", dlr.Stat).
				Msg("DLR пропущен (не DELIVRD или нет client_id)")
			return nil
		}

		// Определяем цену: сначала pricing rule, при отсутствии — фолбэк.
		// Получаем валюту аккаунта, чтобы гарантировать совпадение.
		price := "1.50"
		currency := "RUB"
		if acc, accErr := billingService.GetBalance(ctx, *dlr.ClientID); accErr == nil {
			currency = acc.Currency
		}
		if p, c, pErr := pricingService.GetPriceForDestination(ctx, dlr.ClientID, dlr.Destination); pErr == nil && c == currency {
			price = p
		}

		_, err = billingService.ChargeMessage(
			ctx,
			*dlr.ClientID,
			dlr.MessageID,
			price,
			currency,
			"SMS delivery charge",
		)
		if err != nil {
			logger.Warn().Err(err).
				Str("message_id", dlr.MessageID.String()).
				Str("client_id", dlr.ClientID.String()).
				Msg("ошибка тарификации сообщения")
			return nil // не блокируем consumer
		}

		logger.Debug().
			Str("message_id", dlr.MessageID.String()).
			Str("client_id", dlr.ClientID.String()).
			Str("amount", price).
			Msg("сообщение тарифицировано")
		return nil
	}

	// Обработчик для failed сообщений - можем делать refund или просто логировать
	failedHandler := func(ctx context.Context, failed *queue.FailedMessage) error {
		// Для failed сообщений можно сделать refund или просто логировать
		// В зависимости от бизнес-логики
		if failed.KafkaMessage != nil && failed.KafkaMessage.ClientID != nil {
			logger.Info().
				Str("message_id", failed.MessageID.String()).
				Str("client_id", failed.KafkaMessage.ClientID.String()).
				Str("error", failed.Error).
				Msg("сообщение не доставлено, refund не выполняется")
		}
		return nil
	}

	// Пустой handler для обычных сообщений (не используем для billing)
	messageHandler := func(ctx context.Context, kafkaMsg *queue.KafkaMessage) error {
		// Billing service не обрабатывает обычные сообщения
		return nil
	}

	kafkaConsumer, err := queue.NewConsumer(&kafkaCfg, messageHandler, dlrHandler, failedHandler)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka consumer")
	}

	logger.Info().Msg("Kafka consumer инициализирован")

	// Запуск Kafka consumers для обработки событий
	go func() {
		// Обрабатываем DLR (доставленные сообщения) для тарификации
		if err := kafkaConsumer.ConsumeDLR(); err != nil {
			logger.Error().Err(err).Msg("ошибка запуска consumer для DLR сообщений")
		}

		// Обрабатываем failed сообщения
		if err := kafkaConsumer.ConsumeFailed(); err != nil {
			logger.Error().Err(err).Msg("ошибка запуска consumer для failed сообщений")
		}
	}()

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker("billing-service", cfg.Service.Version)
	healthChecker.SetDatabase(dbConn.DB)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcapi.TraceUnaryServerInterceptor()),
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	// Регистрация gRPC сервиса
	billingGrpcServer := billinggrpc.NewServer(billingService, pricingService, dualDeps)
	billingv1.RegisterBillingServiceServer(grpcServer, billingGrpcServer)

	reflection.Register(grpcServer)
	logger.Info().Msg("gRPC reflection включен")

	// Запуск gRPC сервера
	grpcListener, err := net.Listen("tcp", ":9097") // Используем порт для billing service
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
		Addr:         ":2118", // Используем отдельный порт для метрик billing service
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

	logger.Info().Msg("Billing Service остановлен")
}
