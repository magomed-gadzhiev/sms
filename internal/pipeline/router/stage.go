package router

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/pipeline"
	"github.com/smpp-server/smpp-server/internal/pipeline/trace"
	"github.com/smpp-server/smpp-server/internal/queue"
	msgunifiedrouter "github.com/smpp-server/smpp-server/internal/router"
	routingapp "github.com/smpp-server/smpp-server/internal/services/routing/application"
	routingdomain "github.com/smpp-server/smpp-server/internal/services/routing/domain"
	routinginfra "github.com/smpp-server/smpp-server/internal/services/routing/infrastructure"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// Stage — pipeline stage для маршрутизации сообщений.
// Потребляет из sms.outgoing и sms.failed, вызывает RouteMatcher
// для определения провайдера по условиям (operator, traffic_type, etc.),
// публикует RoutedMessage в sms.routed.
type Stage struct {
	consumer          *queue.BatchConsumer
	producer          *queue.AsyncProducer
	matcher           *routingapp.RouteMatcher
	operatorResolver  *msgunifiedrouter.OperatorResolver
	defaultOperatorID uuid.UUID
	cfg               *config.Config
	logger            zerolog.Logger
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

	// Create pgxpool for RouteMatcher (uses pgx/v5 natively).
	pool, err := pgxpool.New(context.Background(), cfg.Database.GetDSN())
	if err != nil {
		consumer.Close()
		producer.Close()
		return nil, fmt.Errorf("ошибка создания pgxpool для RouteMatcher: %w", err)
	}

	routeRepo := routinginfra.NewRouteRepo(pool)
	matcher := routingapp.NewRouteMatcher(routeRepo)
	if err := matcher.Load(context.Background()); err != nil {
		log.Warn().Err(err).Msg("не удалось загрузить маршруты в RouteMatcher при старте")
	}

	// OperatorResolver: определение оператора по номеру телефона
	opPrefixRepo := storage.NewOperatorPrefixRepository(db)
	defaultOpIDStr := os.Getenv("TARIFICATION_DEFAULT_OPERATOR_ID")
	defaultOpID, _ := uuid.Parse(defaultOpIDStr)
	if defaultOpID == uuid.Nil {
		defaultOpID, _ = uuid.Parse("d0000000-0000-0000-0000-000000000001")
	}
	operatorResolver := msgunifiedrouter.NewOperatorResolver(opPrefixRepo, defaultOpID)

	logger := log.With().Str("component", "pipeline_router").Logger()

	stage := &Stage{
		consumer:          consumer,
		producer:          producer,
		matcher:           matcher,
		operatorResolver:  operatorResolver,
		defaultOperatorID: defaultOpID,
		cfg:               cfg,
		logger:            logger,
	}

	// Периодически перезагружаем маршруты для подхватывания изменений.
	go stage.reloadLoop(context.Background(), pool)

	return stage, nil
}

// reloadLoop перезагружает маршруты каждые 60 секунд.
func (s *Stage) reloadLoop(ctx context.Context, pool *pgxpool.Pool) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.matcher.Invalidate(ctx)
			s.logger.Debug().Msg("маршруты перезагружены")
		}
	}
}

// Run запускает цикл потребления и маршрутизации. Блокирует до отмены ctx.
func (s *Stage) Run(ctx context.Context) error {
	s.logger.Info().Msg("запуск router stage")
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

	// Определяем оператора по номеру получателя
	var operatorID uuid.UUID
	if kafkaMsg.ClientID != nil {
		operatorID = s.operatorResolver.Resolve(ctx, kafkaMsg.Destination)
	} else {
		operatorID = s.defaultOperatorID
	}

	trace.Debug(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "operator_resolved").
		Str("operator_id", operatorID.String()).
		Str("destination", kafkaMsg.Destination).
		Msg("operator resolved")

	var providerID uuid.UUID
	var routeID *uuid.UUID

	if kafkaMsg.ClientID != nil {
		// Build match context with all available fields.
		trafficType := routingdomain.TrafficType(kafkaMsg.TrafficType)
		if trafficType == "" {
			trafficType = routingdomain.TrafficTypeTransactional
		}

		matchCtx := routingapp.MatchContext{
			RouteType:   "sms",
			ClientID:    *kafkaMsg.ClientID,
			OperatorID:  &operatorID,
			TrafficType: trafficType,
			MessageBody: kafkaMsg.Text,
			SenderName:  kafkaMsg.Source,
		}

		result := s.matcher.MatchWithDetails(matchCtx)
		if len(result.Matched) == 0 {
			trace.Warn(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "no_route").
				Str("operator_id", operatorID.String()).
				Str("traffic_type", string(trafficType)).
				Str("sender_name", kafkaMsg.Source).
				Int("client_routes_checked", result.ClientRoutes).
				Int("default_routes_checked", result.DefaultRoutes).
				Int64("kafka_wait_ms", time.Since(kafkaMsg.CreatedAt).Milliseconds()).
				Msg("no matching route found")
			return fmt.Errorf("маршрут не найден для message_id=%s client=%s operator=%s traffic=%s",
				kafkaMsg.MessageID, kafkaMsg.ClientID, operatorID, trafficType)
		}

		// Take the highest-priority route (lowest Priority value).
		route := result.Matched[0]
		providerID = route.ProviderID
		routeID = &route.ID

		trace.Log(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "route_matched").
			Str("route_id", route.ID.String()).
			Str("route_name", route.Name).
			Str("provider_id", providerID.String()).
			Int("priority", route.Priority).
			Bool("used_default", result.UsedDefault).
			Int("total_matched", len(result.Matched)).
			Int64("kafka_wait_ms", time.Since(kafkaMsg.CreatedAt).Milliseconds()).
			Msg("route selected")
	} else {
		// Нет ClientID — используем provider_id если указан.
		if kafkaMsg.ProviderID == nil {
			return fmt.Errorf("message_id=%s: нет client_id и provider_id", kafkaMsg.MessageID)
		}
		providerID = *kafkaMsg.ProviderID
	}

	resolvedOperatorID := operatorID
	routed := &pipeline.RoutedMessage{
		SchemaVersion: 1,
		MessageID:     kafkaMsg.MessageID,
		TraceID:       kafkaMsg.TraceID,
		Source:        kafkaMsg.Source,
		Destination:   kafkaMsg.Destination,
		Text:          kafkaMsg.Text,
		ClientID:      kafkaMsg.ClientID,
		OperatorID:    &resolvedOperatorID,
		ProviderID:    providerID,
		RouteID:       routeID,
		Priority:      kafkaMsg.Priority,
		RetryCount:    kafkaMsg.RetryCount,
		MaxRetries:    kafkaMsg.MaxRetries,
		RoutedAt:      time.Now(),
		CreatedAt:     kafkaMsg.CreatedAt,
		Metadata:      kafkaMsg.Metadata,
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
			{Key: []byte("provider_id"), Value: []byte(providerID.String())},
		},
	)

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

		s.logger.Info().
			Str("trace_id", failed.KafkaMessage.TraceID).
			Str("message_id", failed.MessageID.String()).
			Int("retry_count", failed.RetryCount).
			Int("max_retries", failed.KafkaMessage.MaxRetries).
			Msg("processing retry message")

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
