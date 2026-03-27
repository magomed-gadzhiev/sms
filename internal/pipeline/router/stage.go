package router

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/pipeline"
	cache "github.com/smpp-server/smpp-server/internal/pipeline/cache"
	"github.com/smpp-server/smpp-server/internal/queue"
	msgrouter "github.com/smpp-server/smpp-server/internal/router"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// Stage — pipeline stage для маршрутизации сообщений.
// Потребляет из sms.outgoing и sms.failed, вызывает router.RouteMessage()
// для определения primary + fallback провайдера, публикует RoutedMessage
// в sms.routed.
type Stage struct {
	consumer   *queue.BatchConsumer
	producer   *queue.AsyncProducer
	router     *msgrouter.CachedRouter
	routeCache *cache.RouteCache
	cfg        *config.Config
	logger     zerolog.Logger
}

// NewStage создает новый Router stage pipeline.
func NewStage(cfg *config.Config, db *storage.DB) (*Stage, error) {
	consumer, err := queue.NewBatchConsumer(
		&cfg.Kafka,
		"pipeline-router",
		[]string{cfg.Kafka.TopicOutgoing, cfg.Kafka.TopicFailed},
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

	providerRepo := storage.NewProviderRepository(db)
	routeRepo := storage.NewRouteRepository(db)
	routeCache := cache.NewRouteCache(routeRepo, providerRepo, 30*time.Second)
	msgRouter := msgrouter.NewCachedRouter(routeCache)

	logger := log.With().Str("component", "pipeline_router").Logger()

	return &Stage{
		consumer:   consumer,
		producer:   producer,
		router:     msgRouter,
		routeCache: routeCache,
		cfg:        cfg,
		logger:     logger,
	}, nil
}

// Run запускает цикл потребления и маршрутизации. Блокирует до отмены ctx.
func (s *Stage) Run(ctx context.Context) error {
	s.logger.Info().Msg("запуск router stage")
	if err := s.routeCache.Start(ctx); err != nil {
		return fmt.Errorf("ошибка запуска кеша маршрутов: %w", err)
	}
	return s.consumer.ConsumeBatches(ctx, s.handleBatch)
}

// handleBatch обрабатывает пакет сообщений из Kafka.
func (s *Stage) handleBatch(ctx context.Context, msgs []*sarama.ConsumerMessage, session sarama.ConsumerGroupSession) error {
	start := time.Now()
	monitoring.PipelineBatchSize.WithLabelValues("router").Observe(float64(len(msgs)))

	for _, msg := range msgs {
		if err := s.processMessage(ctx, msg); err != nil {
			s.logger.Error().
				Err(err).
				Str("topic", msg.Topic).
				Int32("partition", msg.Partition).
				Int64("offset", msg.Offset).
				Msg("ошибка обработки сообщения")
			monitoring.PipelineMessagesProcessed.WithLabelValues("router", "error").Inc()
			continue
		}
		monitoring.PipelineMessagesProcessed.WithLabelValues("router", "success").Inc()
	}

	elapsed := time.Since(start).Seconds()
	monitoring.PipelineProcessingDuration.WithLabelValues("router").Observe(elapsed)

	return nil
}

// processMessage маршрутизирует одно сообщение из batch.
func (s *Stage) processMessage(ctx context.Context, msg *sarama.ConsumerMessage) error {
	kafkaMsg, err := s.deserializeByTopic(msg)
	if err != nil {
		return fmt.Errorf("десериализация: %w", err)
	}

	sharedMsg := kafkaMsg.ToMessage()

	provider, err := s.router.RouteMessage(ctx, sharedMsg)
	if err != nil {
		return fmt.Errorf("маршрутизация message_id=%s: %w", kafkaMsg.MessageID, err)
	}

	// Определяем fallback провайдера, если есть route_id.
	var fallbackProviderID *uuid.UUID
	if kafkaMsg.RouteID != nil {
		fallback, fbErr := s.router.GetFailoverProvider(ctx, *kafkaMsg.RouteID)
		if fbErr != nil {
			// Отсутствие fallback — не критичная ошибка, просто логируем.
			s.logger.Debug().
				Err(fbErr).
				Str("message_id", kafkaMsg.MessageID.String()).
				Msg("fallback провайдер недоступен")
		} else if fallback.ID != provider.ID {
			fallbackProviderID = &fallback.ID
		}
	}

	routed := &pipeline.RoutedMessage{
		SchemaVersion:      1,
		MessageID:          kafkaMsg.MessageID,
		Source:             kafkaMsg.Source,
		Destination:        kafkaMsg.Destination,
		Text:               kafkaMsg.Text,
		ClientID:           kafkaMsg.ClientID,
		ProviderID:         provider.ID,
		FallbackProviderID: fallbackProviderID,
		RouteID:            kafkaMsg.RouteID,
		Priority:           kafkaMsg.Priority,
		RetryCount:         kafkaMsg.RetryCount,
		MaxRetries:         kafkaMsg.MaxRetries,
		RoutedAt:           time.Now(),
		CreatedAt:          kafkaMsg.CreatedAt,
		Metadata:           kafkaMsg.Metadata,
	}

	data, err := routed.Serialize()
	if err != nil {
		return fmt.Errorf("сериализация RoutedMessage: %w", err)
	}

	s.producer.PublishAsync(
		s.cfg.Kafka.TopicRouted,
		kafkaMsg.MessageID.String(),
		data,
		[]sarama.RecordHeader{
			{Key: []byte("message_id"), Value: []byte(kafkaMsg.MessageID.String())},
			{Key: []byte("provider_id"), Value: []byte(provider.ID.String())},
		},
	)

	s.logger.Debug().
		Str("message_id", kafkaMsg.MessageID.String()).
		Str("provider_id", provider.ID.String()).
		Str("topic", s.cfg.Kafka.TopicRouted).
		Msg("сообщение маршрутизировано")

	return nil
}

// deserializeByTopic десериализует сообщение в зависимости от исходного топика.
// Для sms.failed проверяет retry_count < max_retries.
func (s *Stage) deserializeByTopic(msg *sarama.ConsumerMessage) (*queue.KafkaMessage, error) {
	if msg.Topic == s.cfg.Kafka.TopicFailed {
		failed, err := queue.DeserializeFailed(msg.Value)
		if err != nil {
			return nil, fmt.Errorf("десериализация FailedMessage: %w", err)
		}

		if failed.KafkaMessage == nil {
			return nil, fmt.Errorf("FailedMessage message_id=%s не содержит KafkaMessage", failed.MessageID)
		}

		if failed.RetryCount >= failed.KafkaMessage.MaxRetries {
			return nil, fmt.Errorf(
				"message_id=%s исчерпал retry (%d/%d)",
				failed.MessageID, failed.RetryCount, failed.KafkaMessage.MaxRetries,
			)
		}

		// Обновляем retry count в KafkaMessage для дальнейшей обработки.
		failed.KafkaMessage.RetryCount = failed.RetryCount
		return failed.KafkaMessage, nil
	}

	// sms.outgoing (или любой другой топик — default path)
	kafkaMsg, err := queue.Deserialize(msg.Value)
	if err != nil {
		return nil, fmt.Errorf("десериализация KafkaMessage: %w", err)
	}
	return kafkaMsg, nil
}

// Close выполняет graceful shutdown stage: закрывает consumer и producer.
func (s *Stage) Close() error {
	s.logger.Info().Msg("закрытие router stage")

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

	return firstErr
}
