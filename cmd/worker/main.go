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
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/router"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smsc"
	"github.com/smpp-server/smpp-server/internal/storage"

	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Загрузка конфигурации
	cfg, err := config.Load("")
	if err != nil {
		log.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	// Инициализация логгера
	shared.InitLogger(cfg.Service.Env)

	log.Info().
		Str("service", cfg.Service.Name).
		Str("version", cfg.Service.Version).
		Msg("запуск Worker сервиса")

	// Подключение к базе данных с настройками пула
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

	// Создание репозиториев
	messageRepo := storage.NewMessageRepository(db)
	providerRepo := storage.NewProviderRepository(db)
	routeRepo := storage.NewRouteRepository(db)

	// Создание SMSC connection pool
	smscPool := smsc.NewPool(&cfg.Worker)

	// Инициализация соединений к активным провайдерам
	ctx := context.Background()
	providers, err := providerRepo.GetAllActive(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения провайдеров")
	} else {
		log.Info().Int("count", len(providers)).Msg("инициализация соединений к провайдерам")
		for _, provider := range providers {
			if provider.MaxConnections > 0 {
				for i := 0; i < provider.MaxConnections; i++ {
					connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					if _, err := smscPool.Connect(connCtx, provider); err != nil {
						log.Error().
							Err(err).
							Str("provider_id", provider.ID.String()).
							Str("provider_name", provider.Name).
							Int("connection", i+1).
							Msg("ошибка подключения к провайдеру")
					} else {
						log.Debug().
							Str("provider_id", provider.ID.String()).
							Str("provider_name", provider.Name).
							Int("connection", i+1).
							Msg("соединение к провайдеру установлено")
					}
					cancel()
				}
			}
		}
	}

	// Подключение к tarification-service gRPC
	tarificationAddr := os.Getenv("TARIFICATION_SERVICE_ADDR")
	if tarificationAddr == "" {
		tarificationAddr = "tarification-service:9100"
	}
	tarificationConn, err := grpc.NewClient(tarificationAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Warn().Err(err).Msg("не удалось подключиться к tarification-service, тарификация отключена")
	}
	var tarificationClient tarificationv1.TarificationServiceClient
	if tarificationConn != nil {
		tarificationClient = tarificationv1.NewTarificationServiceClient(tarificationConn)
		defer tarificationConn.Close()
		log.Info().Str("addr", tarificationAddr).Msg("подключение к tarification-service")
	}

	// Создание sender
	sender := smsc.NewSender(smscPool)

	// Создание router
	msgRouter := router.NewRouter(routeRepo, providerRepo)

	// Создание retry manager
	retryManager := router.NewRetryManager(&cfg.Worker, messageRepo)

	// Получаем worker ID из переменной окружения или используем имя сервиса
	workerID := os.Getenv("SERVICE_NAME")
	if workerID == "" {
		workerID = cfg.Service.Name
	}

	// Ожидание готовности Kafka перед инициализацией consumer
	log.Info().Msg("ожидание готовности Kafka брокеров")
	if err := queue.WaitForKafka(&cfg.Kafka, 30, 2*time.Second); err != nil {
		log.Fatal().Err(err).Msg("Kafka брокеры недоступны")
	}

	// Создание Kafka consumer
	consumer, err := queue.NewConsumer(
		&cfg.Kafka,
		createOutgoingHandler(messageRepo, msgRouter, sender, retryManager, workerID, tarificationClient),
		createDLRHandler(messageRepo),
		createFailedHandler(messageRepo),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("ошибка создания Kafka consumer")
	}

	// Создание health checker
	healthChecker := monitoring.NewHealthChecker(cfg.Service.Name, cfg.Service.Version)
	healthChecker.SetDatabase(db.DB)
	if cfg.Redis.Host != "" {
		// Redis может быть не настроен для worker
		redisClient := redis.NewClient(&redis.Options{
			Addr:         cfg.Redis.GetAddr(),
			Password:     cfg.Redis.Password,
			DB:           cfg.Redis.DB,
		})
		if err := redisClient.Ping(ctx).Err(); err == nil {
			healthChecker.SetRedis(redisClient)
		}
	}

	// Создание HTTP сервера для health checks и metrics
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/health", healthChecker.Handler())
	metricsMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	metricsMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())
	if cfg.Monitoring.Prometheus.Enabled {
		metricsMux.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler())
		log.Info().
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
		log.Info().
			Str("addr", metricsServer.Addr).
			Msg("HTTP сервер для metrics запущен")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("ошибка запуска HTTP сервера для metrics")
		}
	}()

	// Запуск consumer'ов
	if err := consumer.ConsumeOutgoing(); err != nil {
		log.Fatal().Err(err).Msg("ошибка запуска consumer для outgoing сообщений")
	}

	if err := consumer.ConsumeDLR(); err != nil {
		log.Fatal().Err(err).Msg("ошибка запуска consumer для DLR сообщений")
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Info().Msg("получен сигнал остановки, выполняется graceful shutdown")

	// Закрытие consumer
	if err := consumer.Close(); err != nil {
		log.Error().Err(err).Msg("ошибка закрытия consumer")
	}

	// Закрытие SMSC pool
	if err := smscPool.Close(); err != nil {
		log.Error().Err(err).Msg("ошибка закрытия SMSC pool")
	}

	// Остановка HTTP сервера для metrics
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("ошибка остановки HTTP сервера для metrics")
	}

	log.Info().Msg("Worker сервис остановлен")
}

// createOutgoingHandler создает обработчик для исходящих сообщений
func createOutgoingHandler(
	messageRepo *storage.MessageRepository,
	msgRouter *router.Router,
	sender *smsc.Sender,
	retryManager *router.RetryManager,
	workerID string,
	tarificationClient tarificationv1.TarificationServiceClient,
) queue.MessageHandler {
	return func(ctx context.Context, kafkaMsg *queue.KafkaMessage) error {
		startTime := time.Now()
		defer func() {
			monitoring.WorkerProcessingDuration.WithLabelValues(workerID, "process_message").Observe(time.Since(startTime).Seconds())
		}()

		// Преобразуем Kafka сообщение в shared.Message
		msg := kafkaMsg.ToMessage()

		// Получаем полную информацию о сообщении из БД
		dbMsg, err := messageRepo.GetByID(ctx, msg.ID)
		if err != nil {
			monitoring.WorkerMessagesProcessed.WithLabelValues(workerID, "error").Inc()
			return fmt.Errorf("ошибка получения сообщения из БД: %w", err)
		}

		// Определяем провайдера
		provider, err := msgRouter.RouteMessage(ctx, dbMsg)
		if err != nil {
			log.Error().
				Err(err).
				Str("message_id", dbMsg.ID.String()).
				Msg("ошибка маршрутизации сообщения")

			monitoring.WorkerMessagesProcessed.WithLabelValues(workerID, "routing_error").Inc()

			// Помечаем сообщение как failed
			if err := retryManager.MarkAsFailed(ctx, dbMsg.ID, err.Error()); err != nil {
				log.Error().Err(err).Str("message_id", dbMsg.ID.String()).Msg("ошибка пометки сообщения как failed")
			}
			return err
		}

		// Отправляем сообщение
		smppMessageID, err := sender.SendMessage(ctx, dbMsg, provider)
		if err != nil {
			log.Error().
				Err(err).
				Str("message_id", dbMsg.ID.String()).
				Str("provider", provider.Name).
				Msg("ошибка отправки сообщения")

			// Проверяем, стоит ли повторять попытку
			if retryManager.IsPermanentError(err) {
				// Постоянная ошибка - помечаем как failed
				if err := retryManager.MarkAsFailed(ctx, dbMsg.ID, err.Error()); err != nil {
					log.Error().Err(err).Str("message_id", dbMsg.ID.String()).Msg("ошибка пометки сообщения как failed")
				}
				return err
			}

			// Временная ошибка - планируем retry
			if retryManager.ShouldRetry(dbMsg) {
				if err := retryManager.ScheduleRetry(ctx, dbMsg.ID, dbMsg.RetryCount); err != nil {
					log.Error().Err(err).Str("message_id", dbMsg.ID.String()).Msg("ошибка планирования retry")
				}
			} else {
				// Достигнут максимум попыток
				if err := retryManager.MarkAsFailed(ctx, dbMsg.ID, fmt.Sprintf("достигнут максимум попыток: %s", err.Error())); err != nil {
					log.Error().Err(err).Str("message_id", dbMsg.ID.String()).Msg("ошибка пометки сообщения как failed")
				}
			}

			return err
		}

		// Вызываем тарификацию
		if tarificationClient != nil {
			segCount := dbMsg.SegmentCount
			if segCount == 0 {
				segCount = 1
			}
			tarifyResp, tarifyErr := tarificationClient.TarifyMessage(ctx, &tarificationv1.TarifyMessageRequest{
				ClientId:       dbMsg.ClientID.String(),
				MessageId:      dbMsg.ID.String(),
				OperatorId:     "d0000000-0000-0000-0000-000000000005",
				SenderName:     dbMsg.Source,
				SegmentCount:   int32(segCount),
				IdempotencyKey: dbMsg.ID.String(),
			})
			if tarifyErr != nil {
				log.Warn().Err(tarifyErr).Str("message_id", dbMsg.ID.String()).Msg("ошибка тарификации (продолжаем)")
			} else if tarifyResp != nil && !tarifyResp.Approved {
				log.Warn().Str("message_id", dbMsg.ID.String()).Str("reason", tarifyResp.RejectionReason).Msg("тарификация отклонена")
			}
		}

		// Обновляем статус сообщения в БД
		now := time.Now()
		dbMsg.SMPPMessageID = smppMessageID
		dbMsg.ProviderID = &provider.ID
		dbMsg.SubmittedAt = &now
		dbMsg.UpdatedAt = now

		// Симулятор — сразу помечаем как delivered
		if smsc.IsSimulator(provider) {
			dbMsg.Status = shared.MessageStatusDelivered
			dbMsg.DeliveredAt = &now
		} else {
			dbMsg.Status = shared.MessageStatusSent
		}

		if err := messageRepo.Update(ctx, dbMsg); err != nil {
			log.Error().
				Err(err).
				Str("message_id", dbMsg.ID.String()).
				Msg("ошибка обновления статуса сообщения")
			return err
		}

		monitoring.WorkerMessagesProcessed.WithLabelValues(workerID, "success").Inc()

		log.Info().
			Str("message_id", dbMsg.ID.String()).
			Str("smpp_message_id", smppMessageID).
			Str("provider", provider.Name).
			Msg("сообщение успешно отправлено")

		return nil
	}
}

// createDLRHandler создает обработчик для DLR сообщений
func createDLRHandler(messageRepo *storage.MessageRepository) queue.DLRHandler {
	return func(ctx context.Context, dlr *queue.DLRMessage) error {
		log.Info().
			Str("message_id", dlr.MessageID.String()).
			Str("smpp_message_id", dlr.SMPPMessageID).
			Str("stat", dlr.Stat).
			Msg("обработка DLR")

		// Получаем сообщение по SMPP message_id
		msg, err := messageRepo.GetBySMPPMessageID(ctx, dlr.SMPPMessageID)
		if err != nil {
			log.Warn().
				Err(err).
				Str("smpp_message_id", dlr.SMPPMessageID).
				Msg("сообщение не найдено по SMPP message_id")
			return nil // Не критичная ошибка, продолжаем
		}

		now := time.Now()

		// Обновляем статус в зависимости от stat
		switch dlr.Stat {
		case "DELIVRD":
			msg.Status = shared.MessageStatusDelivered
			msg.DeliveredAt = &now
		case "EXPIRED":
			msg.Status = shared.MessageStatusExpired
			msg.FailedAt = &now
		case "REJECTD", "UNDELIV":
			msg.Status = shared.MessageStatusFailed
			msg.FailedAt = &now
		default:
			// Для других статусов оставляем текущий статус
			log.Debug().
				Str("message_id", msg.ID.String()).
				Str("stat", dlr.Stat).
				Msg("неизвестный статус DLR")
		}

		msg.StatusMessage = dlr.Text
		msg.UpdatedAt = now

		if err := messageRepo.Update(ctx, msg); err != nil {
			log.Error().
				Err(err).
				Str("message_id", msg.ID.String()).
				Msg("ошибка обновления статуса сообщения по DLR")
			return err
		}

		log.Info().
			Str("message_id", msg.ID.String()).
			Str("status", string(msg.Status)).
			Msg("статус сообщения обновлен по DLR")

		return nil
	}
}

// createFailedHandler создает обработчик для failed сообщений
func createFailedHandler(messageRepo *storage.MessageRepository) queue.FailedHandler {
	return func(ctx context.Context, failed *queue.FailedMessage) error {
		log.Info().
			Str("message_id", failed.MessageID.String()).
			Str("error", failed.Error).
			Int("retry_count", failed.RetryCount).
			Msg("обработка failed сообщения")

		// Получаем сообщение
		msg, err := messageRepo.GetByID(ctx, failed.MessageID)
		if err != nil {
			log.Warn().
				Err(err).
				Str("message_id", failed.MessageID.String()).
				Msg("сообщение не найдено")
			return nil
		}

		// Помечаем как failed
		now := time.Now()
		msg.Status = shared.MessageStatusFailed
		msg.StatusMessage = failed.Error
		msg.FailedAt = &now
		msg.UpdatedAt = now

		if err := messageRepo.Update(ctx, msg); err != nil {
			log.Error().
				Err(err).
				Str("message_id", msg.ID.String()).
				Msg("ошибка обновления failed сообщения")
			return err
		}

		return nil
	}
}
