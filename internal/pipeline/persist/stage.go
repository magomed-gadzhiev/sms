package persist

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// messageRow holds the data for a single row to be COPYed into the messages table.
type messageRow struct {
	id           uuid.UUID
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
	retryCount   int
	maxRetries   int
	createdAt    time.Time
	updatedAt    time.Time
}

// copyColumns lists the columns sent via COPY protocol. Order must match Values() below.
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
	"retry_count",
	"max_retries",
	"created_at",
	"updated_at",
}

// Stage is the Persist pipeline stage. It consumes messages from sms.outgoing
// (separate consumer group from Router) and batch-inserts them into the messages
// PostgreSQL table using the pgx CopyFrom protocol.
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
		[]string{cfg.Kafka.TopicOutgoing},
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

	// 1. Create temp table (dropped on commit).
	_, err = tx.Exec(ctx, `
		CREATE TEMP TABLE IF NOT EXISTS persist_batch (
			id UUID, message_id VARCHAR(255), source VARCHAR(20), destination VARCHAR(20),
			text TEXT, encoding VARCHAR(20), segment_count INT, status VARCHAR(50),
			priority_flag INT, provider_id UUID, route_id UUID, client_id UUID,
			retry_count INT, max_retries INT, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
		) ON COMMIT DELETE ROWS`)
	if err != nil {
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
			retry_count, max_retries, created_at, updated_at
		)
		SELECT id, message_id, source, destination, text, encoding, segment_count,
			status, priority_flag, provider_id, route_id, client_id,
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

// buildCopyRows deserializes Kafka messages and builds messageRow slices.
// It returns successfully parsed rows and a slice of errors for failed ones.
func buildCopyRows(msgs []*sarama.ConsumerMessage) ([]messageRow, []error) {
	rows := make([]messageRow, 0, len(msgs))
	var errs []error

	for _, msg := range msgs {
		km, err := queue.Deserialize(msg.Value)
		if err != nil {
			errs = append(errs, fmt.Errorf("offset %d partition %d: %w", msg.Offset, msg.Partition, err))
			continue
		}

		// Determine encoding from text content.
		enc := shared.DetectEncoding(km.Text)
		encodingStr := "GSM7"
		if enc == shared.EncodingUCS2 {
			encodingStr = "UCS2"
		}

		segmentCount := shared.CountSegments(km.Text)

		// Use MessageID from KafkaMessage as the primary key (id).
		// If MessageID is zero, generate a new UUID.
		id := km.MessageID
		if id == uuid.Nil {
			id = uuid.New()
		}

		now := time.Now()
		createdAt := km.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}

		rows = append(rows, messageRow{
			id:           id,
			messageID:    km.ID,
			source:       km.Source,
			destination:  km.Destination,
			text:         km.Text,
			encoding:     encodingStr,
			segmentCount: segmentCount,
			status:       "pending",
			priorityFlag: km.Priority,
			providerID:   km.ProviderID,
			routeID:      km.RouteID,
			clientID:     km.ClientID,
			retryCount:   km.RetryCount,
			maxRetries:   km.MaxRetries,
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
		r.retryCount,   // retry_count
		r.maxRetries,   // max_retries
		r.createdAt,    // created_at
		r.updatedAt,    // updated_at
	}, nil
}

func (cs *copySource) Err() error {
	return nil
}
