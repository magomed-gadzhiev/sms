package sender

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/pipeline"
	"github.com/smpp-server/smpp-server/internal/pipeline/backpressure"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smsc"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// Stage — pipeline stage для отправки сообщений через SMPP.
// Потребляет RoutedMessage из sms.routed, применяет backpressure,
// отправляет через async SMPP pool, публикует SentMessage в sms.sent.
type Stage struct {
	consumer     *queue.BatchConsumer
	producer     *queue.AsyncProducer
	pool         *smsc.Pool
	sender       *smsc.Sender
	bpManager    *backpressure.Manager
	providerRepo *storage.ProviderRepository
	cfg          *config.Config
	logger       zerolog.Logger
}

// NewStage создает новый Sender stage pipeline.
func NewStage(cfg *config.Config, db *storage.DB) (*Stage, error) {
	consumer, err := queue.NewBatchConsumer(
		&cfg.Kafka,
		"pipeline-sender",
		[]string{cfg.Kafka.TopicRouted},
		cfg.Pipeline.BatchSize,
		cfg.Pipeline.BatchTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания batch consumer: %w", err)
	}

	producer, err := queue.NewAsyncProducer(&cfg.Kafka)
	if err != nil {
		consumer.Close()
		return nil, fmt.Errorf("ошибка создания async producer: %w", err)
	}

	pool := smsc.NewPool(&cfg.Worker)
	snd := smsc.NewSender(pool)
	bpManager := backpressure.NewManager()
	providerRepo := storage.NewProviderRepository(db)

	logger := log.With().Str("component", "pipeline_sender").Logger()

	// Инициализация: подключаем все активные провайдеры и регистрируем backpressure.
	providers, err := providerRepo.GetAllActive(context.Background())
	if err != nil {
		consumer.Close()
		producer.Close()
		return nil, fmt.Errorf("ошибка загрузки активных провайдеров: %w", err)
	}

	for _, p := range providers {
		connsCount := p.MaxConnections
		if connsCount == 0 {
			connsCount = 1
		}

		for i := 0; i < connsCount; i++ {
			if _, connErr := pool.ConnectAsync(context.Background(), p, cfg.Pipeline.SMPPWindowSize); connErr != nil {
				logger.Error().
					Err(connErr).
					Str("provider_id", p.ID.String()).
					Str("provider_name", p.Name).
					Int("connection", i+1).
					Msg("ошибка async подключения к провайдеру")
			}
		}

		bpManager.Register(p.ID, p.ThroughputPerSec, cfg.Pipeline.SMPPWindowSize)
		monitoring.PipelineConnectionsActive.WithLabelValues(p.ID.String()).Set(float64(connsCount))

		logger.Info().
			Str("provider_id", p.ID.String()).
			Str("provider_name", p.Name).
			Int("connections", connsCount).
			Int("throughput_per_sec", p.ThroughputPerSec).
			Msg("провайдер подключён и зарегистрирован в backpressure")
	}

	return &Stage{
		consumer:     consumer,
		producer:     producer,
		pool:         pool,
		sender:       snd,
		bpManager:    bpManager,
		providerRepo: providerRepo,
		cfg:          cfg,
		logger:       logger,
	}, nil
}

// Run запускает цикл потребления и отправки. Блокирует до отмены ctx.
func (s *Stage) Run(ctx context.Context) error {
	s.logger.Info().Msg("запуск sender stage")
	return s.consumer.ConsumeBatches(ctx, s.handleBatch)
}

// handleBatch обрабатывает пакет сообщений из Kafka.
// Каждое сообщение обрабатывается индивидуально: при throttle сообщение
// не маркируется и будет повторно доставлено Kafka.
func (s *Stage) handleBatch(ctx context.Context, msgs []*sarama.ConsumerMessage, session sarama.ConsumerGroupSession) error {
	start := time.Now()
	monitoring.PipelineBatchSize.WithLabelValues("sender").Observe(float64(len(msgs)))

	for _, msg := range msgs {
		if err := s.processMessage(ctx, msg, session); err != nil {
			s.logger.Error().
				Err(err).
				Str("topic", msg.Topic).
				Int32("partition", msg.Partition).
				Int64("offset", msg.Offset).
				Msg("ошибка обработки сообщения")
			monitoring.PipelineMessagesProcessed.WithLabelValues("sender", "error").Inc()
			// Не маркируем — сообщение будет повторно доставлено.
			continue
		}
		monitoring.PipelineMessagesProcessed.WithLabelValues("sender", "success").Inc()
	}

	elapsed := time.Since(start).Seconds()
	monitoring.PipelineProcessingDuration.WithLabelValues("sender").Observe(elapsed)

	// Возвращаем nil — ошибки отдельных сообщений не должны прерывать batch.
	return nil
}

// processMessage обрабатывает одно сообщение: десериализация, backpressure,
// отправка через SMPP, публикация результата в sms.sent.
func (s *Stage) processMessage(ctx context.Context, msg *sarama.ConsumerMessage, session sarama.ConsumerGroupSession) error {
	// 1. Десериализация RoutedMessage.
	routedMsg, err := pipeline.DeserializeRoutedMessage(msg.Value)
	if err != nil {
		return fmt.Errorf("десериализация RoutedMessage: %w", err)
	}

	// 2. Backpressure check: если провайдер throttled, не обрабатываем —
	// сообщение не маркируется и будет повторно доставлено Kafka.
	if !s.bpManager.TryAcquire(routedMsg.ProviderID) {
		s.logger.Warn().
			Str("message_id", routedMsg.MessageID.String()).
			Str("provider_id", routedMsg.ProviderID.String()).
			Msg("backpressure: провайдер throttled, сообщение будет повторно доставлено")
		return fmt.Errorf("backpressure: провайдер %s throttled", routedMsg.ProviderID)
	}

	// 3. Получаем провайдера из БД.
	provider, err := s.providerRepo.GetByID(ctx, routedMsg.ProviderID)
	if err != nil {
		return fmt.Errorf("получение провайдера %s: %w", routedMsg.ProviderID, err)
	}

	// 4. Получаем async соединение из пула.
	conn, err := s.pool.GetAsyncConnection(routedMsg.ProviderID)
	if err != nil {
		return fmt.Errorf("получение async соединения для провайдера %s: %w", routedMsg.ProviderID, err)
	}

	// 5. Конвертируем RoutedMessage в shared.Message для SendMessageAsync.
	sharedMsg := routedToSharedMessage(routedMsg)

	// 6. Отправляем через async SMPP.
	smppMsgID, sendErr := s.sender.SendMessageAsync(ctx, sharedMsg, provider, conn)

	// 6a. Failover: при ошибке primary пробуем fallback_provider_id (R-007).
	usedProviderID := routedMsg.ProviderID
	usedConnID := conn.ID
	if sendErr != nil && routedMsg.FallbackProviderID != nil {
		s.logger.Warn().
			Err(sendErr).
			Str("message_id", routedMsg.MessageID.String()).
			Str("primary_provider", routedMsg.ProviderID.String()).
			Str("fallback_provider", routedMsg.FallbackProviderID.String()).
			Msg("primary send failed, trying fallback provider")

		fallbackProvider, fbErr := s.providerRepo.GetByID(ctx, *routedMsg.FallbackProviderID)
		if fbErr == nil {
			fbConn, fbConnErr := s.pool.GetAsyncConnection(*routedMsg.FallbackProviderID)
			if fbConnErr == nil {
				sharedMsg.ProviderID = routedMsg.FallbackProviderID
				smppMsgID, sendErr = s.sender.SendMessageAsync(ctx, sharedMsg, fallbackProvider, fbConn)
				if sendErr == nil {
					usedProviderID = *routedMsg.FallbackProviderID
					usedConnID = fbConn.ID
				}
			}
		}
	}

	// 6b. Если оба провайдера failed и retry_count < max_retries — публикуем в sms.failed (R-007).
	if sendErr != nil && routedMsg.RetryCount < routedMsg.MaxRetries {
		failedMsg := &queue.FailedMessage{
			MessageID:  routedMsg.MessageID,
			Error:      sendErr.Error(),
			ErrorCode:  "send_failed",
			RetryCount: routedMsg.RetryCount + 1,
			FailedAt:   time.Now(),
			KafkaMessage: &queue.KafkaMessage{
				ID:          routedMsg.MessageID.String(),
				MessageID:   routedMsg.MessageID,
				Source:      routedMsg.Source,
				Destination: routedMsg.Destination,
				Text:        routedMsg.Text,
				ClientID:    routedMsg.ClientID,
				Priority:    routedMsg.Priority,
				RetryCount:  routedMsg.RetryCount + 1,
				MaxRetries:  routedMsg.MaxRetries,
				CreatedAt:   routedMsg.CreatedAt,
				Metadata:    routedMsg.Metadata,
			},
		}
		failedData, fErr := failedMsg.Serialize()
		if fErr == nil {
			s.producer.PublishAsync(
				s.cfg.Kafka.TopicFailed,
				routedMsg.MessageID.String(),
				failedData,
				[]sarama.RecordHeader{
					{Key: []byte("message_id"), Value: []byte(routedMsg.MessageID.String())},
					{Key: []byte("retry_count"), Value: []byte(fmt.Sprintf("%d", failedMsg.RetryCount))},
				},
			)
			s.logger.Info().
				Str("message_id", routedMsg.MessageID.String()).
				Int("retry_count", failedMsg.RetryCount).
				Msg("сообщение опубликовано в sms.failed для повторной маршрутизации")
		}
	}

	segments := shared.SplitMessage(routedMsg.Text)

	// 7. Формируем SentMessage.
	sentMsg := &pipeline.SentMessage{
		SchemaVersion: 1,
		MessageID:     routedMsg.MessageID,
		ProviderID:    usedProviderID,
		SentAt:        time.Now(),
		ConnectionID:  usedConnID,
		SegmentsCount: len(segments),
	}

	if sendErr != nil {
		sentMsg.Status = "failed"
		errMsg := sendErr.Error()
		sentMsg.ErrorMessage = &errMsg
	} else {
		sentMsg.Status = "sent"
		sentMsg.SMPPMessageID = smppMsgID
	}

	// 8. Сериализуем и публикуем в sms.sent.
	data, err := sentMsg.Serialize()
	if err != nil {
		return fmt.Errorf("сериализация SentMessage: %w", err)
	}

	s.producer.PublishAsync(
		s.cfg.Kafka.TopicSent,
		routedMsg.ProviderID.String(),
		data,
		[]sarama.RecordHeader{
			{Key: []byte("message_id"), Value: []byte(routedMsg.MessageID.String())},
			{Key: []byte("provider_id"), Value: []byte(routedMsg.ProviderID.String())},
			{Key: []byte("status"), Value: []byte(sentMsg.Status)},
		},
	)

	// Для SIMULATOR-провайдеров генерируем DLR (DELIVRD) — полный lifecycle без реального SMSC.
	if sendErr == nil && smsc.IsSimulator(provider) {
		now := time.Now()
		dlrMsg := &queue.DLRMessage{
			MessageID:     routedMsg.MessageID,
			SMPPMessageID: smppMsgID,
			ProviderID:    &usedProviderID,
			ClientID:      routedMsg.ClientID,
			Stat:          "DELIVRD",
			SubmitDate:    &sentMsg.SentAt,
			DoneDate:      &now,
			Source:        routedMsg.Source,
			Destination:   routedMsg.Destination,
			CreatedAt:     now,
		}
		dlrData, dlrErr := dlrMsg.Serialize()
		if dlrErr == nil {
			s.producer.PublishAsync(
				s.cfg.Kafka.TopicDLR,
				routedMsg.MessageID.String(),
				dlrData,
				[]sarama.RecordHeader{
					{Key: []byte("message_id"), Value: []byte(routedMsg.MessageID.String())},
					{Key: []byte("stat"), Value: []byte("DELIVRD")},
				},
			)
		}
	}

	s.logger.Debug().
		Str("message_id", routedMsg.MessageID.String()).
		Str("provider_id", routedMsg.ProviderID.String()).
		Str("smpp_message_id", sentMsg.SMPPMessageID).
		Str("status", sentMsg.Status).
		Str("connection_id", conn.ID).
		Int("segments", sentMsg.SegmentsCount).
		Msg("сообщение обработано sender stage")

	// 9. Маркируем сообщение как обработанное.
	session.MarkMessage(msg, "")

	return nil
}

// routedToSharedMessage конвертирует pipeline.RoutedMessage в shared.Message
// для использования в smsc.Sender.SendMessageAsync.
func routedToSharedMessage(rm *pipeline.RoutedMessage) *shared.Message {
	return &shared.Message{
		ID:           rm.MessageID,
		Source:       rm.Source,
		Destination:  rm.Destination,
		Text:         rm.Text,
		ClientID:     rm.ClientID,
		ProviderID:   &rm.ProviderID,
		RouteID:      rm.RouteID,
		PriorityFlag: rm.Priority,
		RetryCount:   rm.RetryCount,
		MaxRetries:   rm.MaxRetries,
		CreatedAt:    rm.CreatedAt,
	}
}

// Close выполняет graceful shutdown stage: закрывает consumer, producer и pool.
func (s *Stage) Close() error {
	s.logger.Info().Msg("закрытие sender stage")

	var firstErr error

	if err := s.consumer.Close(); err != nil {
		s.logger.Error().Err(err).Msg("ошибка закрытия consumer")
		firstErr = err
	}

	if err := s.producer.Close(); err != nil {
		s.logger.Error().Err(err).Msg("ошибка закрытия producer")
		if firstErr == nil {
			firstErr = err
		}
	}

	if err := s.pool.Close(); err != nil {
		s.logger.Error().Err(err).Msg("ошибка закрытия SMPP pool")
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
