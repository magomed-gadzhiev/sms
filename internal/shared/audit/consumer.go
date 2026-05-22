package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"
)

const (
	defaultBatchSize     = 100
	defaultFlushInterval = 5 * time.Second
	defaultGroupID       = "audit-consumer-group"
)

// Consumer consumes audit events from Kafka and persists them to the audit_log table.
type Consumer struct {
	consumerGroup sarama.ConsumerGroup
	db            *sql.DB
	topic         string
	logger        zerolog.Logger
	batchSize     int
	flushInterval time.Duration

	mu     sync.Mutex
	buffer []*AuditEvent
}

// NewConsumer creates a new audit event consumer.
func NewConsumer(
	brokers []string,
	db *sql.DB,
	topic string,
	groupID string,
	logger zerolog.Logger,
) (*Consumer, error) {
	if topic == "" {
		topic = defaultTopic
	}
	if groupID == "" {
		groupID = defaultGroupID
	}

	cfg := sarama.NewConfig()
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	cg, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, fmt.Errorf("audit: create consumer group: %w", err)
	}

	return &Consumer{
		consumerGroup: cg,
		db:            db,
		topic:         topic,
		logger:        logger.With().Str("component", "audit_consumer").Logger(),
		batchSize:     defaultBatchSize,
		flushInterval: defaultFlushInterval,
		buffer:        make([]*AuditEvent, 0, defaultBatchSize),
	}, nil
}

// Start begins consuming audit events. It blocks until the context is cancelled.
func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info().Str("topic", c.topic).Msg("starting audit consumer")

	for {
		if err := c.consumerGroup.Consume(ctx, []string{c.topic}, c); err != nil {
			c.logger.Error().Err(err).Msg("consumer group error")
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// Close closes the consumer group.
func (c *Consumer) Close() error {
	return c.consumerGroup.Close()
}

// Setup is called at the beginning of a new session, before ConsumeClaim.
func (c *Consumer) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is called at the end of a session; flushes remaining buffer.
func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.buffer) > 0 {
		if err := c.flushBuffer(context.Background()); err != nil {
			c.logger.Error().Err(err).Msg("error flushing buffer during cleanup")
		}
	}
	return nil
}

// ConsumeClaim processes messages from a partition claim.
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			var event AuditEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				c.logger.Error().Err(err).
					Int64("offset", msg.Offset).
					Msg("failed to unmarshal audit event")
				session.MarkMessage(msg, "")
				continue
			}

			c.mu.Lock()
			c.buffer = append(c.buffer, &event)
			shouldFlush := len(c.buffer) >= c.batchSize
			c.mu.Unlock()

			if shouldFlush {
				c.mu.Lock()
				if err := c.flushBuffer(session.Context()); err != nil {
					c.logger.Error().Err(err).Msg("error flushing batch")
				}
				c.mu.Unlock()
			}

			session.MarkMessage(msg, "")

		case <-ticker.C:
			c.mu.Lock()
			if len(c.buffer) > 0 {
				if err := c.flushBuffer(session.Context()); err != nil {
					c.logger.Error().Err(err).Msg("error flushing batch on timer")
				}
			}
			c.mu.Unlock()

		case <-session.Context().Done():
			return nil
		}
	}
}

// flushBuffer batch-inserts buffered events into the audit_log table.
// Caller must hold c.mu.
func (c *Consumer) flushBuffer(ctx context.Context) error {
	if len(c.buffer) == 0 {
		return nil
	}

	events := c.buffer
	c.buffer = make([]*AuditEvent, 0, c.batchSize)

	// Build batch INSERT ... ON CONFLICT (id) DO NOTHING
	const cols = "(id, tenant_id, user_id, action, resource_type, resource_id, details, ip_address, created_at)"
	valueStrings := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*9)

	for i, e := range events {
		base := i * 9
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9,
		))

		detailsJSON, _ := json.Marshal(e.Details)

		args = append(args,
			e.EventID,
			e.TenantID,
			e.UserID,
			e.Action,
			e.ResourceType,
			e.ResourceID,
			string(detailsJSON),
			e.IPAddress,
			e.Timestamp,
		)
	}

	query := fmt.Sprintf(
		"INSERT INTO audit_log %s VALUES %s ON CONFLICT (id) DO NOTHING",
		cols,
		strings.Join(valueStrings, ", "),
	)

	_, err := c.db.ExecContext(ctx, query, args...)
	if err != nil {
		c.logger.Error().Err(err).Int("count", len(events)).Msg("batch insert failed")
		return fmt.Errorf("audit: batch insert: %w", err)
	}

	c.logger.Debug().Int("count", len(events)).Msg("batch insert successful")
	return nil
}
