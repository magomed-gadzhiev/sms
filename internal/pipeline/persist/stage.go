package persist

import (
	"context"
	"fmt"
	"os"
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
	"github.com/smpp-server/smpp-server/internal/pipeline/trace"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/messagestatus"
)

// persistTempTableReuse — load-test-only флаг. Под `true` использует
// `CREATE TEMP TABLE IF NOT EXISTS ... ON COMMIT DELETE ROWS` (зануляет
// DDL-overhead на каждом батче). Опасность: если миграция изменит схему
// `messages` без рестарта persist-pod'а — закешированная schema temp-table
// в pgbouncer-сессии разойдётся с реальной, COPY упадёт. Default false:
// делает DROP+CREATE на каждый батч (медленнее, но безопасно при in-place
// миграциях).
var persistTempTableReuse = os.Getenv("PERSIST_TEMP_TABLE_REUSE") == "true"

// messageRow holds the data for a single row to be COPYed into the messages table.
type messageRow struct {
	id           uuid.UUID
	traceID      string // not a DB column — used only for trace logging
	messageID    string
	source       string
	destination  string
	text         string
	encoding     string
	segmentCount int
	status       string
	priorityFlag int
	providerID   *uuid.UUID
	routeID      *uuid.UUID
	clientID     *uuid.UUID
	templateID   *uuid.UUID
	senderNameID *uuid.UUID
	operatorID   *uuid.UUID
	countryID    *uuid.UUID
	channel      *string
	retryCount   int
	maxRetries   int
	createdAt    time.Time
	updatedAt    time.Time
}

// copyColumns lists the columns sent via COPY protocol. Order must match Values() below.
// Bug #15: operator_id/country_id/channel added so enrichment data lands at INSERT
// time (previously only status-stage could write them, and only when a DLR arrived —
// most `pending` messages never got enriched).
var copyColumns = []string{
	"id",
	"message_id",
	"source",
	"destination",
	"text",
	"encoding",
	"segment_count",
	"status",
	"priority_flag",
	"provider_id",
	"route_id",
	"client_id",
	"template_id",
	"sender_name_id",
	"operator_id",
	"country_id",
	"channel",
	"retry_count",
	"max_retries",
	"created_at",
	"updated_at",
}

// Stage is the Persist pipeline stage. It consumes messages from sms.routed
// (separate consumer group from Sender) and batch-inserts them into the messages
// PostgreSQL table using the pgx CopyFrom protocol.
//
// Bug #15 / Option B (2026-04-22): Persist previously consumed sms.outgoing in
// parallel with Router, which meant rows were INSERTed before routing decided
// operator_id/provider_id/channel, and country_id was never resolved. Persist now
// consumes sms.routed so that all four enrichment columns are populated at
// INSERT time. If router is down, persist stops — which is intentional: persisting
// unrouted messages just fills the DB with eventually-useless `pending` rows.
type Stage struct {
	consumer *queue.BatchConsumer
	pool     *pgxpool.Pool
	cfg      *config.Config
	logger   zerolog.Logger
}

// NewStage creates a new Persist stage.
func NewStage(cfg *config.Config, pool *pgxpool.Pool) (*Stage, error) {
	batchSize := cfg.Pipeline.PersistBatchSize
	if batchSize <= 0 {
		batchSize = 2000
	}
	batchTimeout := cfg.Pipeline.PersistBatchTimeout
	if batchTimeout <= 0 {
		batchTimeout = 100 * time.Millisecond
	}

	consumer, err := queue.NewBatchConsumer(
		&cfg.Kafka,
		"pipeline-persist",
		[]string{cfg.Kafka.TopicRouted},
		batchSize,
		batchTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("creating batch consumer: %w", err)
	}

	logger := log.With().Str("component", "pipeline_persist").Logger()

	return &Stage{
		consumer: consumer,
		pool:     pool,
		cfg:      cfg,
		logger:   logger,
	}, nil
}

// Run starts the consumption loop. Blocks until ctx is cancelled.
func (s *Stage) Run(ctx context.Context) error {
	s.logger.Info().Msg("persist stage started")
	return s.consumer.ConsumeBatches(ctx, s.handleBatch)
}

// handleBatch processes a batch of Kafka messages: deserializes, builds rows,
// and inserts via CopyFrom.
func (s *Stage) handleBatch(ctx context.Context, msgs []*sarama.ConsumerMessage, session sarama.ConsumerGroupSession) error {
	start := time.Now()
	monitoring.PipelineBatchSize.WithLabelValues("persist").Observe(float64(len(msgs)))

	rows, deserErrors := buildCopyRows(msgs)

	// Count deserialization errors.
	for _, err := range deserErrors {
		s.logger.Error().Err(err).Msg("deserialization error in persist batch")
		monitoring.PipelineMessagesProcessed.WithLabelValues("persist", "error").Inc()
	}

	if len(rows) > 0 {
		if err := s.copyInsert(ctx, rows); err != nil {
			s.logger.Error().Err(err).Int("batch_size", len(rows)).Msg("COPY insert failed")
			monitoring.PipelineMessagesProcessed.WithLabelValues("persist", "error").Add(float64(len(rows)))
			// Return error so that offsets are NOT committed — messages will be redelivered.
			return err
		}
		monitoring.PipelineMessagesProcessed.WithLabelValues("persist", "success").Add(float64(len(rows)))
		for _, r := range rows {
			trace.Debug(s.logger, r.traceID, r.id.String(), "persist", "completed").
				Str("encoding", r.encoding).
				Int("segment_count", r.segmentCount).
				Msg("message persisted to DB")
		}
	}

	elapsed := time.Since(start).Seconds()
	monitoring.PipelineProcessingDuration.WithLabelValues("persist").Observe(elapsed)

	return nil
}

// copyInsert performs a COPY via temp table + INSERT ON CONFLICT DO NOTHING
// to handle duplicate messages gracefully (idempotent inserts).
func (s *Stage) copyInsert(ctx context.Context, rows []messageRow) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Create temp table.
	// Default (safe): DROP+CREATE на каждом батче — DDL overhead, но
	// гарантирует совпадение схемы temp-table с актуальным набором
	// колонок даже после in-place миграции `messages`.
	// Под `PERSIST_TEMP_TABLE_REUSE=true` (load-test): CREATE IF NOT
	// EXISTS + ON COMMIT DELETE ROWS — переиспользуем существующую
	// pg_temp таблицу для всей сессии. См. комментарий к
	// persistTempTableReuse.
	var ddl string
	if persistTempTableReuse {
		ddl = `
		CREATE TEMP TABLE IF NOT EXISTS persist_batch (
			id UUID, message_id VARCHAR(255), source VARCHAR(20), destination VARCHAR(20),
			text TEXT, encoding VARCHAR(20), segment_count INT, status VARCHAR(50),
			priority_flag INT, provider_id UUID, route_id UUID, client_id UUID,
			template_id UUID, sender_name_id UUID,
			operator_id UUID, country_id UUID, channel VARCHAR(20),
			retry_count INT, max_retries INT, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
		) ON COMMIT DELETE ROWS`
	} else {
		ddl = `
		DROP TABLE IF EXISTS persist_batch;
		CREATE TEMP TABLE persist_batch (
			id UUID, message_id VARCHAR(255), source VARCHAR(20), destination VARCHAR(20),
			text TEXT, encoding VARCHAR(20), segment_count INT, status VARCHAR(50),
			priority_flag INT, provider_id UUID, route_id UUID, client_id UUID,
			template_id UUID, sender_name_id UUID,
			operator_id UUID, country_id UUID, channel VARCHAR(20),
			retry_count INT, max_retries INT, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
		)`
	}
	if _, err = tx.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("create temp table: %w", err)
	}

	// 2. COPY rows into temp table.
	src := &copySource{rows: rows}
	_, err = tx.CopyFrom(ctx, pgx.Identifier{"persist_batch"}, copyColumns, src)
	if err != nil {
		return fmt.Errorf("CopyFrom temp: %w", err)
	}

	// 3. INSERT into messages with ON CONFLICT DO NOTHING (idempotent).
	tag, err := tx.Exec(ctx, `
		INSERT INTO messages (
			id, message_id, source, destination, text, encoding, segment_count,
			status, priority_flag, provider_id, route_id, client_id,
			template_id, sender_name_id,
			operator_id, country_id, channel,
			retry_count, max_retries, created_at, updated_at
		)
		SELECT id, message_id, source, destination, text, encoding, segment_count,
			status, priority_flag, provider_id, route_id, client_id,
			template_id, sender_name_id,
			operator_id, country_id, channel,
			retry_count, max_retries, created_at, updated_at
		FROM persist_batch
		ON CONFLICT (id, created_at) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("INSERT ON CONFLICT: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.logger.Debug().
		Int64("rows_inserted", tag.RowsAffected()).
		Int("rows_provided", len(rows)).
		Msg("batch COPY insert complete")

	return nil
}

// buildCopyRows deserializes Kafka messages (expected to be RoutedMessage on
// sms.routed) and builds messageRow slices. Returns successfully parsed rows
// and a slice of errors for failed ones.
func buildCopyRows(msgs []*sarama.ConsumerMessage) ([]messageRow, []error) {
	rows := make([]messageRow, 0, len(msgs))
	var errs []error

	for _, msg := range msgs {
		rm, err := pipeline.DeserializeRoutedMessage(msg.Value)
		if err != nil {
			errs = append(errs, fmt.Errorf("offset %d partition %d: %w", msg.Offset, msg.Partition, err))
			continue
		}

		// Determine encoding from text content.
		enc := shared.DetectEncoding(rm.Text)
		encodingStr := "GSM7"
		if enc == shared.EncodingUCS2 {
			encodingStr = "UCS2"
		}

		segmentCount := shared.CountSegments(rm.Text)

		// MessageID is the primary key (id). Router always sets it, but fall back
		// to a new UUID for safety.
		id := rm.MessageID
		if id == uuid.Nil {
			id = uuid.New()
		}

		now := time.Now()
		createdAt := rm.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}

		// provider_id on RoutedMessage is a value type (always set by router).
		// Copy to pointer so NULL remains possible for legacy/in-flight messages
		// where ProviderID == uuid.Nil.
		var providerID *uuid.UUID
		if rm.ProviderID != uuid.Nil {
			pid := rm.ProviderID
			providerID = &pid
		}

		var channel *string
		if rm.Channel != "" {
			c := rm.Channel
			channel = &c
		}

		rows = append(rows, messageRow{
			id:           id,
			traceID:      rm.TraceID,
			messageID:    rm.MessageID.String(),
			source:       rm.Source,
			destination:  rm.Destination,
			text:         rm.Text,
			encoding:     encodingStr,
			segmentCount: segmentCount,
			status:       string(messagestatus.Pending),
			priorityFlag: rm.Priority,
			providerID:   providerID,
			routeID:      rm.RouteID,
			clientID:     rm.ClientID,
			templateID:   rm.TemplateID,
			senderNameID: rm.SenderNameID,
			operatorID:   rm.OperatorID,
			countryID:    rm.CountryID,
			channel:      channel,
			retryCount:   rm.RetryCount,
			maxRetries:   rm.MaxRetries,
			createdAt:    createdAt,
			updatedAt:    now,
		})
	}

	return rows, errs
}

// Close performs graceful shutdown of the persist stage.
func (s *Stage) Close() error {
	s.logger.Info().Msg("closing persist stage")

	if err := s.consumer.Close(); err != nil {
		s.logger.Error().Err(err).Msg("error closing consumer")
		return err
	}

	return nil
}

// ---------------------------------------------------------------------------
// copySource implements pgx.CopyFromSource for batch COPY.
// ---------------------------------------------------------------------------

type copySource struct {
	rows []messageRow
	idx  int
}

func (cs *copySource) Next() bool {
	return cs.idx < len(cs.rows)
}

func (cs *copySource) Values() ([]interface{}, error) {
	r := cs.rows[cs.idx]
	cs.idx++

	// channel is stored as *string so nil maps to NULL in pgx COPY.
	var channelVal interface{}
	if r.channel != nil {
		channelVal = *r.channel
	}

	return []interface{}{
		r.id,           // id
		r.messageID,    // message_id
		r.source,       // source
		r.destination,  // destination
		r.text,         // text
		r.encoding,     // encoding
		r.segmentCount, // segment_count
		r.status,       // status
		r.priorityFlag, // priority_flag
		r.providerID,   // provider_id
		r.routeID,      // route_id
		r.clientID,     // client_id
		r.templateID,   // template_id
		r.senderNameID, // sender_name_id
		r.operatorID,   // operator_id
		r.countryID,    // country_id
		channelVal,     // channel
		r.retryCount,   // retry_count
		r.maxRetries,   // max_retries
		r.createdAt,    // created_at
		r.updatedAt,    // updated_at
	}, nil
}

func (cs *copySource) Err() error {
	return nil
}
