package status

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/pipeline"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// statusRecord — унифицированная запись для batch upsert в БД.
// Используется как для SentMessage (sms.sent), так и для DLRMessage (sms.dlr).
type statusRecord struct {
	MessageID     uuid.UUID
	Status        string
	SMPPMessageID string
	ProviderID    *uuid.UUID
	SubmittedAt   *time.Time
	UpdatedAt     time.Time
	SegmentCount  int
}

// Stage — pipeline stage для записи статусов сообщений в БД.
// Потребляет SentMessage из sms.sent и DLRMessage из sms.dlr,
// выполняет batch upsert в таблицу messages через INSERT ... ON CONFLICT (id) DO UPDATE.
// После успешного upsert публикует StatusUpdate в sms.status для campaign-service.
type Stage struct {
	consumer      *queue.BatchConsumer
	asyncProducer *queue.AsyncProducer
	db            *storage.DB
	pgxPool       *pgxpool.Pool
	cfg           *config.Config
	logger        zerolog.Logger

	// T030: Write-ahead buffer для retry при ошибках БД.
	failedMu     sync.Mutex
	failedBuffer []*statusRecord
}

// NewStage создает новый Status Writer stage pipeline.
func NewStage(cfg *config.Config, db *storage.DB, pgxPool *pgxpool.Pool) (*Stage, error) {
	// T028: потребляем из обоих топиков — sms.sent и sms.dlr.
	consumer, err := queue.NewBatchConsumer(
		&cfg.Kafka,
		"pipeline-status",
		[]string{cfg.Kafka.TopicSent, cfg.Kafka.TopicDLR},
		cfg.Pipeline.BatchSize,
		cfg.Pipeline.BatchTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания batch consumer: %w", err)
	}

	asyncProducer, err := queue.NewAsyncProducer(&cfg.Kafka)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания async producer: %w", err)
	}

	logger := log.With().Str("component", "pipeline_status").Logger()

	return &Stage{
		consumer:      consumer,
		asyncProducer: asyncProducer,
		db:            db,
		pgxPool:       pgxPool,
		cfg:           cfg,
		logger:        logger,
	}, nil
}

// Run запускает цикл потребления и записи статусов. Блокирует до отмены ctx.
func (s *Stage) Run(ctx context.Context) error {
	s.logger.Info().Msg("запуск status writer stage")

	// T030: запуск фоновой горутины для retry failedBuffer.
	go s.retryLoop(ctx)

	return s.consumer.ConsumeBatches(ctx, s.handleBatch)
}

// handleBatch обрабатывает пакет сообщений из Kafka.
func (s *Stage) handleBatch(ctx context.Context, msgs []*sarama.ConsumerMessage, session sarama.ConsumerGroupSession) error {
	start := time.Now()
	monitoring.PipelineBatchSize.WithLabelValues("status").Observe(float64(len(msgs)))

	// 1. Десериализация всех сообщений с учётом топика.
	records := make([]*statusRecord, 0, len(msgs))
	for _, msg := range msgs {
		rec, err := s.deserializeMessage(msg)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("topic", msg.Topic).
				Int32("partition", msg.Partition).
				Int64("offset", msg.Offset).
				Msg("ошибка десериализации сообщения")
			monitoring.PipelineMessagesProcessed.WithLabelValues("status", "error").Inc()
			continue
		}
		records = append(records, rec)
	}

	if len(records) == 0 {
		return nil
	}

	// 2. Batch upsert в БД.
	if err := s.batchUpsert(ctx, records); err != nil {
		s.logger.Error().
			Err(err).
			Int("batch_size", len(records)).
			Msg("ошибка batch upsert статусов, буферизация для retry")
		monitoring.PipelineMessagesProcessed.WithLabelValues("status", "error").Add(float64(len(records)))

		// T030: буферизация неудачных записей для retry.
		s.failedMu.Lock()
		s.failedBuffer = append(s.failedBuffer, records...)
		s.failedMu.Unlock()

		// Не возвращаем ошибку — продолжаем потребление из Kafka без блокировки.
		return nil
	}

	// 3. Публикуем StatusUpdate в sms.status для campaign-service.
	s.publishStatusUpdates(records)

	// 4. Метрики.
	monitoring.PipelineMessagesProcessed.WithLabelValues("status", "success").Add(float64(len(records)))
	elapsed := time.Since(start).Seconds()
	monitoring.PipelineProcessingDuration.WithLabelValues("status").Observe(elapsed)

	s.logger.Debug().
		Int("batch_size", len(records)).
		Float64("duration_sec", elapsed).
		Msg("batch статусов записан")

	return nil
}

// deserializeMessage десериализует сообщение Kafka в statusRecord
// в зависимости от топика (sms.sent или sms.dlr).
func (s *Stage) deserializeMessage(msg *sarama.ConsumerMessage) (*statusRecord, error) {
	switch msg.Topic {
	case s.cfg.Kafka.TopicSent:
		sent, err := pipeline.DeserializeSentMessage(msg.Value)
		if err != nil {
			return nil, fmt.Errorf("десериализация SentMessage: %w", err)
		}
		status := mapStatus(sent.Status)
		providerID := &sent.ProviderID
		return &statusRecord{
			MessageID:     sent.MessageID,
			Status:        status,
			SMPPMessageID: sent.SMPPMessageID,
			ProviderID:    providerID,
			SubmittedAt:   &sent.SentAt,
			UpdatedAt:     time.Now(),
			SegmentCount:  sent.SegmentsCount,
		}, nil

	case s.cfg.Kafka.TopicDLR:
		dlr, err := queue.DeserializeDLR(msg.Value)
		if err != nil {
			return nil, fmt.Errorf("десериализация DLRMessage: %w", err)
		}
		status := mapDLRStat(dlr.Stat)
		updatedAt := time.Now()
		if dlr.DoneDate != nil {
			updatedAt = *dlr.DoneDate
		}
		return &statusRecord{
			MessageID:     dlr.MessageID,
			Status:        status,
			SMPPMessageID: dlr.SMPPMessageID,
			ProviderID:    dlr.ProviderID,
			SubmittedAt:   dlr.SubmitDate,
			UpdatedAt:     updatedAt,
			SegmentCount:  0,
		}, nil

	default:
		return nil, fmt.Errorf("неизвестный топик: %s", msg.Topic)
	}
}

// batchUpsert обновляет статусы сообщений в БД через temp table + COPY.
// T029: идемпотентный update с WHERE messages.updated_at < s.updated_at.
// Использует COPY для эффективной загрузки данных, затем UPDATE из temp table.
func (s *Stage) batchUpsert(ctx context.Context, records []*statusRecord) error {
	conn, err := s.pgxPool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Create temp table (idempotent for PgBouncer transaction pooling)
	_, err = tx.Exec(ctx, `
		CREATE TEMP TABLE IF NOT EXISTS status_batch (
			id UUID,
			status TEXT,
			smpp_message_id TEXT,
			provider_id UUID,
			submitted_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			segment_count INT
		) ON COMMIT DELETE ROWS
	`)
	if err != nil {
		return fmt.Errorf("create temp table: %w", err)
	}

	// 2. COPY data into temp table
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"status_batch"},
		[]string{"id", "status", "smpp_message_id", "provider_id", "submitted_at", "updated_at", "segment_count"},
		pgx.CopyFromSlice(len(records), func(i int) ([]any, error) {
			r := records[i]
			return []any{
				r.MessageID,
				r.Status,
				r.SMPPMessageID,
				r.ProviderID,
				r.SubmittedAt,
				r.UpdatedAt,
				r.SegmentCount,
			}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("copy to temp table: %w", err)
	}

	// 3. UPDATE from temp table (idempotent — only newer timestamps)
	_, err = tx.Exec(ctx, `
		UPDATE messages SET
			status = s.status,
			smpp_message_id = COALESCE(s.smpp_message_id, messages.smpp_message_id),
			provider_id = COALESCE(s.provider_id, messages.provider_id),
			submitted_at = COALESCE(s.submitted_at, messages.submitted_at),
			updated_at = s.updated_at,
			segment_count = COALESCE(s.segment_count, messages.segment_count)
		FROM status_batch s
		WHERE messages.id = s.id AND messages.updated_at < s.updated_at
	`)
	if err != nil {
		return fmt.Errorf("batch update: %w", err)
	}

	return tx.Commit(ctx)
}

// mapStatus преобразует статус из SentMessage в статус для БД.
func mapStatus(status string) string {
	switch status {
	case "sent":
		return "sent"
	case "failed":
		return "failed"
	default:
		return status
	}
}

// mapDLRStat преобразует DLR stat в статус для БД.
func mapDLRStat(stat string) string {
	switch stat {
	case "DELIVRD":
		return "delivered"
	case "UNDELIV":
		return "failed"
	case "EXPIRED":
		return "expired"
	default:
		return "unknown"
	}
}

// publishStatusUpdates публикует StatusUpdate сообщения в sms.status
// чтобы campaign-service мог обновить счётчики кампании.
func (s *Stage) publishStatusUpdates(records []*statusRecord) {
	topic := s.cfg.Kafka.TopicStatus
	for _, r := range records {
		update := &pipeline.StatusUpdate{
			SchemaVersion: 1,
			MessageID:     r.MessageID,
			Status:        r.Status,
			SMPPMessageID: r.SMPPMessageID,
			ProviderID:    r.ProviderID,
			SubmitDate:    r.SubmittedAt,
			UpdatedAt:     r.UpdatedAt,
		}
		data, err := update.Serialize()
		if err != nil {
			s.logger.Error().Err(err).Str("message_id", r.MessageID.String()).Msg("ошибка сериализации StatusUpdate")
			continue
		}
		s.asyncProducer.PublishAsync(topic, r.MessageID.String(), data, []sarama.RecordHeader{
			{Key: []byte("message_id"), Value: []byte(r.MessageID.String())},
		})
	}
}

// retryLoop — T030: фоновая горутина для периодического retry failedBuffer
// с экспоненциальным backoff.
func (s *Stage) retryLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	backoff := 5 * time.Second
	const maxBackoff = 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.failedMu.Lock()
			if len(s.failedBuffer) == 0 {
				s.failedMu.Unlock()
				// Сброс backoff при пустом буфере.
				backoff = 5 * time.Second
				continue
			}

			// Забираем буфер под блокировкой и очищаем.
			batch := s.failedBuffer
			s.failedBuffer = nil
			s.failedMu.Unlock()

			s.logger.Info().
				Int("buffer_size", len(batch)).
				Dur("backoff", backoff).
				Msg("retry failedBuffer")

			retryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := s.batchUpsert(retryCtx, batch)
			cancel()

			if err != nil {
				s.logger.Error().
					Err(err).
					Int("buffer_size", len(batch)).
					Msg("retry failedBuffer не удался, возврат в буфер")

				// Возвращаем обратно в буфер.
				s.failedMu.Lock()
				s.failedBuffer = append(batch, s.failedBuffer...)
				s.failedMu.Unlock()

				// Экспоненциальный backoff.
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}

				// Обновляем интервал тикера.
				ticker.Reset(backoff)
			} else {
				monitoring.PipelineMessagesProcessed.WithLabelValues("status", "success").Add(float64(len(batch)))
				s.logger.Info().
					Int("buffer_size", len(batch)).
					Msg("retry failedBuffer успешен")

				// Сброс backoff при успехе.
				backoff = 5 * time.Second
				ticker.Reset(backoff)
			}
		}
	}
}

// Close выполняет graceful shutdown stage: закрывает consumer и producer.
func (s *Stage) Close() error {
	s.logger.Info().Msg("закрытие status writer stage")
	if err := s.asyncProducer.Close(); err != nil {
		s.logger.Error().Err(err).Msg("ошибка закрытия async producer")
	}
	return s.consumer.Close()
}
