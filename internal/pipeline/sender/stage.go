package sender

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/pipeline"
	"github.com/smpp-server/smpp-server/internal/pipeline/backpressure"
	"github.com/smpp-server/smpp-server/internal/pipeline/limits"
	"github.com/smpp-server/smpp-server/internal/pipeline/trace"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smsc"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// Stage — pipeline stage для отправки сообщений через SMPP.
// Потребляет RoutedMessage из sms.routed, применяет двухуровневый backpressure,
// тарифицирует сообщение, отправляет через SenderFactory (SMPP/Stub), публикует SentMessage в sms.sent.
type Stage struct {
	consumer           *queue.BatchConsumer
	producer           *queue.AsyncProducer
	pool               *smsc.Pool
	senderFactory      *smsc.SenderFactory
	bpManager          *backpressure.Manager
	providerRepo       *storage.ProviderRepository
	limitResolver      *limits.CachedLimitResolver
	tarificationClient tarificationv1.TarificationServiceClient
	billingClient      billingv1.BillingServiceClient
	tarificationConn   *grpc.ClientConn
	billingConn        *grpc.ClientConn
	defaultOperatorID  string
	clientRepo         *storage.ClientRepository
	cfg                *config.Config
	logger             zerolog.Logger
	// commitOnSubmitEnabled — Phase 2 dual-charge flag. При true TarifyMessage
	// работает read-only, фактическое списание делается через CommitCharge RPC
	// после успешного SUBMIT на провайдер. При false — legacy flow (списание
	// в TarifyMessage, рефанд при окончательном провале).
	commitOnSubmitEnabled bool
}

// NewStage создает новый Sender stage pipeline.
func NewStage(cfg *config.Config, db *storage.DB, rdb *redis.Client) (*Stage, error) {
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

	// Устанавливаем DLR callback для обработки deliver_sm от провайдеров
	dlrTopic := cfg.Kafka.TopicDLR
	pool.SetDLRCallback(func(data *smsc.DeliverSMData) {
		dlrMsg := &queue.DLRMessage{
			SMPPMessageID:      data.SMPPMessageID,
			ProviderID:         &data.ProviderID,
			ReceiptedMessageID: data.SMPPMessageID,
			Stat:               data.Stat,
			Source:             data.Source,
			Destination:        data.Destination,
			Text:               data.Text,
			CreatedAt:          time.Now(),
		}
		now := time.Now()
		dlrMsg.DoneDate = &now
		if dlrData, serErr := dlrMsg.Serialize(); serErr == nil {
			producer.PublishAsync(dlrTopic, data.SMPPMessageID, dlrData, nil)
			log.Info().
				Str("smpp_message_id", data.SMPPMessageID).
				Str("stat", data.Stat).
				Str("provider_id", data.ProviderID.String()).
				Msg("DLR опубликован в Kafka")
		} else {
			log.Error().Err(serErr).Msg("ошибка сериализации DLR")
		}
	})

	bpManager := backpressure.NewManager()
	providerRepo := storage.NewProviderRepository(db)
	clientRepo := storage.NewClientRepository(db)

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

	// Инициализация LimitResolver
	var limitResolver *limits.CachedLimitResolver
	if rdb != nil {
		querier := storage.NewLimitQuerierDB(db)
		dbResolver := limits.NewDBLimitResolver(querier)
		limitResolver = limits.NewCachedLimitResolver(dbResolver, rdb)
	}

	// Загрузка per-client TPS из client_providers
	cpRepo := storage.NewClientProviderRepository(db)
	clientProviders, cpErr := cpRepo.GetAllActiveWithTPS(context.Background())
	if cpErr == nil {
		for _, cp := range clientProviders {
			if cp.TPSLimit != nil {
				bpManager.RegisterClient(cp.ClientID, cp.ProviderID, *cp.TPSLimit)
			}
		}
	}

	// SenderFactory: SMPP vs Stub dispatch
	stubConfigRepo := storage.NewStubConfigRepository(db)
	stubSender := smsc.NewStubSender(stubConfigRepo, producer, cfg.Kafka.TopicDLR)
	smppSender := smsc.NewSender(pool)
	senderFactory := smsc.NewSenderFactory(smppSender, stubSender)

	// Подключение к billing-service gRPC
	var billingClient billingv1.BillingServiceClient
	var billingGRPCConn *grpc.ClientConn
	billingAddr := os.Getenv("BILLING_SERVICE_ADDR")
	if billingAddr == "" {
		billingAddr = "billing-service:9097"
	}
	billingGRPCConn, err = grpc.NewClient(billingAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(false)),
	)
	if err != nil {
		logger.Warn().Err(err).Msg("не удалось подключиться к billing-service")
	} else {
		billingClient = billingv1.NewBillingServiceClient(billingGRPCConn)
		logger.Info().Str("addr", billingAddr).Msg("подключение к billing-service")
	}

	// Подключение к tarification-service gRPC
	var tarificationClient tarificationv1.TarificationServiceClient
	var tarificationGRPCConn *grpc.ClientConn
	tarificationAddr := os.Getenv("TARIFICATION_SERVICE_ADDR")
	if tarificationAddr == "" {
		tarificationAddr = "tarification-service:9100"
	}
	tarificationGRPCConn, err = grpc.NewClient(tarificationAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(false)),
	)
	if err != nil {
		logger.Warn().Err(err).Msg("не удалось подключиться к tarification-service")
	} else {
		tarificationClient = tarificationv1.NewTarificationServiceClient(tarificationGRPCConn)
		logger.Info().Str("addr", tarificationAddr).Msg("подключение к tarification-service")
	}

	defaultOperatorID := os.Getenv("TARIFICATION_DEFAULT_OPERATOR_ID")
	if defaultOperatorID == "" {
		defaultOperatorID = "d0000000-0000-0000-0000-000000000001"
	}

	// Pub/sub listener для инвалидации per-client TPS
	if limitResolver != nil && rdb != nil {
		go func() {
			redisSub := rdb.Subscribe(context.Background(), "limits:invalidate")
			ch := redisSub.Channel()
			for msg := range ch {
				parts := strings.SplitN(msg.Payload, ":", 2)
				if len(parts) != 2 {
					continue
				}
				cid, err1 := uuid.Parse(parts[0])
				pid, err2 := uuid.Parse(parts[1])
				if err1 != nil || err2 != nil {
					continue
				}
				tps, resolveErr := limitResolver.ResolveProviderTPS(context.Background(), cid, pid)
				if resolveErr == nil {
					bpManager.UpdateClient(cid, pid, tps)
					limitResolver.InvalidateProviderTPS(context.Background(), cid, pid)
				}
			}
		}()
	}

	return &Stage{
		consumer:              consumer,
		producer:              producer,
		pool:                  pool,
		senderFactory:         senderFactory,
		bpManager:             bpManager,
		providerRepo:          providerRepo,
		limitResolver:         limitResolver,
		tarificationClient:    tarificationClient,
		billingClient:         billingClient,
		tarificationConn:      tarificationGRPCConn,
		billingConn:           billingGRPCConn,
		defaultOperatorID:     defaultOperatorID,
		clientRepo:            clientRepo,
		cfg:                   cfg,
		logger:                logger,
		commitOnSubmitEnabled: cfg.Tarification.CommitOnSubmitEnabled,
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

	pickupAt := time.Now()
	traceID := routedMsg.TraceID

	// 2. Проверка заморозки и тарификация — ДО отправки.
	if s.billingClient != nil && routedMsg.ClientID != nil {
		grpcCtx, grpcCancel := context.WithTimeout(ctx, 5*time.Second)
		balanceResp, balanceErr := s.billingClient.GetBalance(grpcCtx, &billingv1.GetBalanceRequest{
			ClientId: routedMsg.ClientID.String(),
		})
		grpcCancel()
		if balanceErr != nil {
			trace.Log(s.logger, traceID, routedMsg.MessageID.String(), "sender.billing", "error").
				Err(balanceErr).
				Msg("billing service unavailable, message rejected")
			monitoring.PipelineMessagesProcessed.WithLabelValues("sender", "billing_unavailable").Inc()
			session.MarkMessage(msg, "")
			return nil
		} else if balanceResp != nil && balanceResp.Frozen {
			trace.Warn(s.logger, traceID, routedMsg.MessageID.String(), "sender.billing", "account_frozen").
				Str("client_id", routedMsg.ClientID.String()).
				Msg("account frozen, message rejected")
			monitoring.PipelineMessagesProcessed.WithLabelValues("sender", "account_frozen").Inc()
			session.MarkMessage(msg, "")
			return nil
		}

		trace.Debug(s.logger, traceID, routedMsg.MessageID.String(), "sender.billing", "ok").
			Str("client_id", routedMsg.ClientID.String()).
			Msg("billing check passed")
	}

	var chargedAmount, chargedCurrency string
	// tarifyReq сохраняется для последующего вызова CommitCharge (Phase 2 dual-charge).
	var tarifyReq *tarificationv1.TarifyMessageRequest
	var tarifyApproved bool
	if s.tarificationClient != nil && routedMsg.ClientID != nil {
		segments := shared.SplitMessage(routedMsg.Text)
		segCount := int32(len(segments))
		if segCount == 0 {
			segCount = 1
		}
		operatorID := s.defaultOperatorID
		if routedMsg.OperatorID != nil {
			operatorID = routedMsg.OperatorID.String()
		}
		tarifyReq = &tarificationv1.TarifyMessageRequest{
			ClientId:       routedMsg.ClientID.String(),
			MessageId:      routedMsg.MessageID.String(),
			OperatorId:     operatorID,
			SenderName:     routedMsg.Source,
			SegmentCount:   segCount,
			IdempotencyKey: routedMsg.MessageID.String(),
		}
		tarifyCtx, tarifyCancel := context.WithTimeout(ctx, 5*time.Second)
		tarifyResp, tarifyErr := s.tarificationClient.TarifyMessage(tarifyCtx, tarifyReq)
		tarifyCancel()
		if tarifyErr != nil {
			trace.Log(s.logger, traceID, routedMsg.MessageID.String(), "sender.tarify", "error").
				Err(tarifyErr).
				Msg("tarification failed, message rejected")
			monitoring.PipelineMessagesProcessed.WithLabelValues("sender", "tarification_error").Inc()
			session.MarkMessage(msg, "")
			return nil
		}
		if tarifyResp != nil && !tarifyResp.Approved {
			trace.Warn(s.logger, traceID, routedMsg.MessageID.String(), "sender.tarify", "rejected").
				Str("reason", tarifyResp.RejectionReason).
				Msg("tarification rejected")
			monitoring.PipelineMessagesProcessed.WithLabelValues("sender", "tarification_rejected").Inc()
			session.MarkMessage(msg, "")
			return nil
		}
		if tarifyResp != nil {
			tarifyApproved = true
			chargedAmount = tarifyResp.TotalAmount
			chargedCurrency = tarifyResp.Currency
			trace.Debug(s.logger, traceID, routedMsg.MessageID.String(), "sender.tarify", "approved").
				Str("amount", chargedAmount).
				Str("currency", chargedCurrency).
				Int32("segments", segCount).
				Bool("commit_on_submit", s.commitOnSubmitEnabled).
				Msg("tarification approved")
		}

		// Обновляем счётчик monthly_sms_count у клиента (quota subscription)
		if tarifyResp != nil && tarifyResp.Approved && s.clientRepo != nil && routedMsg.ClientID != nil {
			if incrErr := s.clientRepo.IncrementMonthlySMSCount(ctx, *routedMsg.ClientID, int(segCount)); incrErr != nil {
				s.logger.Warn().Err(incrErr).
					Str("client_id", routedMsg.ClientID.String()).
					Int32("seg_count", segCount).
					Msg("не удалось обновить monthly_sms_count")
			}
		}
	}

	// 3. Backpressure check: если провайдер throttled, не обрабатываем —
	// сообщение не маркируется и будет повторно доставлено Kafka.
	var bpClientID uuid.UUID
	if routedMsg.ClientID != nil {
		bpClientID = *routedMsg.ClientID
	}
	if !s.bpManager.TryAcquire(bpClientID, routedMsg.ProviderID) {
		trace.Warn(s.logger, traceID, routedMsg.MessageID.String(), "sender.backpressure", "throttled").
			Str("provider_id", routedMsg.ProviderID.String()).
			Msg("provider throttled, message will be redelivered")
		return fmt.Errorf("backpressure: провайдер %s throttled", routedMsg.ProviderID)
	}

	// 4. Получаем провайдера из БД.
	provider, err := s.providerRepo.GetByID(ctx, routedMsg.ProviderID)
	if err != nil {
		return fmt.Errorf("получение провайдера %s: %w", routedMsg.ProviderID, err)
	}

	// 5. Получаем соединение (nil для stub-провайдеров).
	var conn *smsc.AsyncConnection
	if !smsc.IsSimulator(provider) {
		conn, err = s.pool.GetAsyncConnection(routedMsg.ProviderID)
		if err != nil {
			return fmt.Errorf("получение async соединения для провайдера %s: %w", routedMsg.ProviderID, err)
		}
	}

	// 6. Конвертируем RoutedMessage в shared.Message и отправляем через SenderFactory.
	sharedMsg := routedToSharedMessage(routedMsg)
	sender := s.senderFactory.For(provider)
	smppMsgID, sendErr := sender.SendMessageAsync(ctx, sharedMsg, provider, conn)

	usedProviderID := routedMsg.ProviderID
	usedConnID := ""
	if conn != nil {
		usedConnID = conn.ID
	}

	// 6a. Phase 2 dual-charge: CommitCharge после успешной отправки.
	// При флаге ON TarifyMessage работал read-only — списания ещё не было.
	// CommitCharge идемпотентен (по MessageId): повторные вызовы при retry
	// безопасны. Ошибка CommitCharge не прерывает pipeline — сообщение уже
	// ушло, финансы синхронизируются retry-worker'ом (вне scope Task 14).
	if s.commitOnSubmitEnabled && sendErr == nil && tarifyApproved && tarifyReq != nil && s.tarificationClient != nil {
		commitCtx, commitCancel := context.WithTimeout(ctx, 10*time.Second)
		_, commitErr := s.tarificationClient.CommitCharge(commitCtx, &tarificationv1.CommitChargeRequest{
			MessageId:      tarifyReq.MessageId,
			ClientId:       tarifyReq.ClientId,
			OperatorId:     tarifyReq.OperatorId,
			SenderName:     tarifyReq.SenderName,
			SegmentCount:   tarifyReq.SegmentCount,
			IdempotencyKey: tarifyReq.IdempotencyKey,
		})
		commitCancel()
		if commitErr != nil {
			s.logger.Error().Err(commitErr).
				Str("message_id", routedMsg.MessageID.String()).
				Str("client_id", tarifyReq.ClientId).
				Str("operator_id", tarifyReq.OperatorId).
				Int32("segments", tarifyReq.SegmentCount).
				Msg("CommitCharge failed after successful send — message uncharged pending retry")
			monitoring.PipelineMessagesProcessed.WithLabelValues("sender", "commit_charge_error").Inc()
		} else {
			trace.Debug(s.logger, traceID, routedMsg.MessageID.String(), "sender.commit", "ok").
				Str("client_id", tarifyReq.ClientId).
				Str("amount", chargedAmount).
				Str("currency", chargedCurrency).
				Msg("CommitCharge applied")
		}
	}

	// 6b. Legacy refund при окончательном провале (все retry исчерпаны).
	// При commitOnSubmitEnabled=true рефанд НЕ нужен: TarifyMessage был
	// read-only, CommitCharge при sendErr != nil не вызывался, ничего не
	// списано.
	if sendErr != nil && routedMsg.RetryCount >= routedMsg.MaxRetries && !s.commitOnSubmitEnabled {
		if s.billingClient != nil && routedMsg.ClientID != nil && chargedAmount != "" {
			refundCtx, refundCancel := context.WithTimeout(ctx, 5*time.Second)
			_, refundErr := s.billingClient.AddCredits(refundCtx, &billingv1.AddCreditsRequest{
				ClientId:    routedMsg.ClientID.String(),
				Amount:      chargedAmount,
				Currency:    chargedCurrency,
				Description: fmt.Sprintf("refund: send failed after %d retries, message %s", routedMsg.RetryCount, routedMsg.MessageID.String()),
			})
			refundCancel()
			if refundErr != nil {
				s.logger.Error().Err(refundErr).
					Str("message_id", routedMsg.MessageID.String()).
					Str("amount", chargedAmount).
					Msg("ошибка рефанда после окончательного провала отправки")
			} else {
				s.logger.Info().
					Str("message_id", routedMsg.MessageID.String()).
					Str("amount", chargedAmount).
					Msg("рефанд выполнен после окончательного провала отправки")
			}
		}
	}

	// 6c. Если оба провайдера failed и retry_count < max_retries — публикуем в sms.failed (R-007).
	if sendErr != nil && routedMsg.RetryCount < routedMsg.MaxRetries {
		failedMsg := &queue.FailedMessage{
			MessageID:  routedMsg.MessageID,
			TraceID:    traceID,
			Error:      sendErr.Error(),
			ErrorCode:  "send_failed",
			RetryCount: routedMsg.RetryCount + 1,
			FailedAt:   time.Now(),
			KafkaMessage: &queue.KafkaMessage{
				ID:          routedMsg.MessageID.String(),
				MessageID:   routedMsg.MessageID,
				TraceID:     traceID,
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
		TraceID:       traceID,
		ProviderID:    usedProviderID,
		RouteID:       routedMsg.RouteID,
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

	// DLR для SIMULATOR-провайдеров теперь генерируется StubSender внутренне.

	senderType := "smpp"
	if smsc.IsSimulator(provider) {
		senderType = "stub"
	}

	trace.Log(s.logger, traceID, routedMsg.MessageID.String(), "sender.send", "completed").
		Str("provider_id", routedMsg.ProviderID.String()).
		Str("smpp_message_id", sentMsg.SMPPMessageID).
		Str("status", sentMsg.Status).
		Str("connection_id", usedConnID).
		Str("sender_type", senderType).
		Int("segments", sentMsg.SegmentsCount).
		Int64("kafka_wait_ms", pickupAt.Sub(routedMsg.RoutedAt).Milliseconds()).
		Int64("processing_ms", time.Since(pickupAt).Milliseconds()).
		Msg("message processed by sender")

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

	if s.billingConn != nil {
		s.billingConn.Close()
	}
	if s.tarificationConn != nil {
		s.tarificationConn.Close()
	}

	return firstErr
}
