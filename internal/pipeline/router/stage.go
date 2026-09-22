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
	"github.com/smpp-server/smpp-server/internal/shared/cache"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// routerSenderCache caches resolveSenderName results keyed by
// (clientID|senderName|operatorID) → resolved sender. Hot-path: every routed
// message runs two SQL queries without this cache.
var routerSenderCache = cache.NewHardCache(60 * time.Second)

// routerFallbackCache caches the system_defaults fallback sender. Single-key
// cache, refreshed once per TTL.
var routerFallbackCache = cache.NewHardCache(300 * time.Second)

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
	pool              *pgxpool.Pool
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
		pool:              pool,
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

	// Определяем оператора и страну по номеру получателя.
	// country_id используется persist-stage для enrichment (bug #15).
	var operatorID uuid.UUID
	var countryID *uuid.UUID
	if kafkaMsg.ClientID != nil {
		operatorID, countryID = s.operatorResolver.ResolveWithCountry(ctx, kafkaMsg.Destination)
	} else {
		operatorID = s.defaultOperatorID
	}

	trace.Debug(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "operator_resolved").
		Str("operator_id", operatorID.String()).
		Str("destination", kafkaMsg.Destination).
		Msg("operator resolved")

	var providerID uuid.UUID
	var routeID *uuid.UUID
	// channel по умолчанию "sms"; если маршрут другой route_type (hlr/max) —
	// подставляем его, чтобы persist-stage писал корректный channel при INSERT.
	channel := "sms"

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

		// Routing Resolution (CONTEXT.md): client routes → reseller shared →
		// platform defaults, gated by the client's Routing Mode. The stage
		// knows nothing about buckets or mode strings — Resolve owns them.
		decision, err := s.matcher.Resolve(ctx, matchCtx)
		if err != nil {
			trace.Warn(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "no_route").
				Err(err).
				Str("operator_id", operatorID.String()).
				Str("traffic_type", string(trafficType)).
				Str("sender_name", kafkaMsg.Source).
				Int64("kafka_wait_ms", time.Since(kafkaMsg.CreatedAt).Milliseconds()).
				Msg("no matching route found")
			return fmt.Errorf("маршрут не найден для message_id=%s client=%s operator=%s traffic=%s: %w",
				kafkaMsg.MessageID, kafkaMsg.ClientID, operatorID, trafficType, err)
		}
		route := decision.Route
		providerID = route.ProviderID
		routeID = &route.ID
		if route.RouteType != "" {
			channel = route.RouteType
		}

		trace.Log(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "route_matched").
			Str("route_id", route.ID.String()).
			Str("route_name", route.Name).
			Str("provider_id", providerID.String()).
			Int("priority", route.Priority).
			Str("resolution_level", string(decision.Level)).
			Int64("kafka_wait_ms", time.Since(kafkaMsg.CreatedAt).Milliseconds()).
			Msg("route selected")
	} else {
		// Нет ClientID — используем provider_id если указан.
		if kafkaMsg.ProviderID == nil {
			return fmt.Errorf("message_id=%s: нет client_id и provider_id", kafkaMsg.MessageID)
		}
		providerID = *kafkaMsg.ProviderID
	}

	// Resolve sender name: substitute with system fallback if not approved.
	clientIDStr := ""
	if kafkaMsg.ClientID != nil {
		clientIDStr = kafkaMsg.ClientID.String()
	}
	resolvedSource := s.resolveSenderName(ctx, clientIDStr, kafkaMsg.Source, operatorID.String())

	resolvedOperatorID := operatorID
	routed := &pipeline.RoutedMessage{
		SchemaVersion: 2,
		MessageID:     kafkaMsg.MessageID,
		TraceID:       kafkaMsg.TraceID,
		Source:        resolvedSource,
		Destination:   kafkaMsg.Destination,
		Text:          kafkaMsg.Text,
		ClientID:      kafkaMsg.ClientID,
		OperatorID:    &resolvedOperatorID,
		CountryID:     countryID,
		Channel:       channel,
		TemplateID:    kafkaMsg.TemplateID,
		SenderNameID:  kafkaMsg.SenderNameID,
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

// resolveSenderName checks if sender name is approved for the given operator.
// Returns fallback system sender if not approved. Fail-open: on DB error returns original name.
func (s *Stage) resolveSenderName(ctx context.Context, clientID, senderName, operatorID string) string {
	if senderName == "" || s.pool == nil || clientID == "" {
		return senderName
	}

	// Numeric sender names (phone numbers) don't need registration checks.
	isNumeric := true
	for _, c := range senderName {
		if c < '0' || c > '9' {
			isNumeric = false
			break
		}
	}
	if isNumeric {
		return senderName
	}

	// Hard-cache hit: result by (client, sender, operator) — stable across TTL.
	cacheKey := clientID + "|" + senderName + "|" + operatorID
	if routerSenderCache.Enabled() {
		if v, ok := routerSenderCache.Get(cacheKey); ok {
			return v.(string)
		}
	}
	result := s.resolveSenderNameUncached(ctx, clientID, senderName, operatorID)
	if routerSenderCache.Enabled() {
		routerSenderCache.Set(cacheKey, result)
	}
	return result
}

// resolveSenderNameUncached is the DB-hitting path extracted so the cache
// wrapper can call it. Keeps the two SQL queries verbatim.
func (s *Stage) resolveSenderNameUncached(ctx context.Context, clientID, senderName, operatorID string) string {

	// Check if sub-account or direct client, and get sender_name status.
	var parentClientID *string
	var snStatus string
	err := s.pool.QueryRow(ctx,
		`SELECT c.parent_client_id, COALESCE(sn.status, '')
		 FROM clients c
		 LEFT JOIN sender_names sn ON sn.client_id = c.id AND sn.name = $2
		 WHERE c.id = $1
		 LIMIT 1`,
		clientID, senderName,
	).Scan(&parentClientID, &snStatus)
	if err != nil {
		return senderName // fail-open
	}

	// Sub-account: only check sender_name approval.
	if parentClientID != nil {
		if snStatus == "approved" {
			return senderName
		}
		fallback := s.getFallbackSender(ctx)
		log.Warn().
			Str("component", "pipeline_router").
			Str("client_id", clientID).
			Str("original_sender", senderName).
			Str("fallback", fallback).
			Str("sender_status", snStatus).
			Str("reason", "sender_name not approved for sub-account").
			Msg("sender substituted — регистрация неактивна")
		return fallback
	}

	// Direct client: check operator_registrations.approved_type.
	var approvedType *string
	err = s.pool.QueryRow(ctx,
		`SELECT or2.approved_type
		 FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 WHERE sn.client_id = $1 AND sn.name = $2 AND or2.operator_id = $3
		 LIMIT 1`,
		clientID, senderName, operatorID,
	).Scan(&approvedType)
	if err != nil || approvedType == nil {
		fallback := s.getFallbackSender(ctx)
		log.Warn().
			Str("component", "pipeline_router").
			Str("client_id", clientID).
			Str("operator_id", operatorID).
			Str("original_sender", senderName).
			Str("fallback", fallback).
			Str("reason", "no operator_registrations entry for direct client").
			Msg("sender substituted — отсутствует регистрация на оператора")
		return fallback
	}
	return senderName
}

// getFallbackSender returns the system-configured default sender name.
func (s *Stage) getFallbackSender(ctx context.Context) string {
	if s.pool == nil {
		return "SMS"
	}
	if routerFallbackCache.Enabled() {
		if v, ok := routerFallbackCache.Get("default"); ok {
			return v.(string)
		}
	}
	var val string
	err := s.pool.QueryRow(ctx,
		`SELECT value FROM system_defaults WHERE key = 'default_sender_name' LIMIT 1`,
	).Scan(&val)
	if err != nil || val == "" {
		val = "SMS"
	}
	if routerFallbackCache.Enabled() {
		routerFallbackCache.Set("default", val)
	}
	return val
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
