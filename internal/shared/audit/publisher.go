package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"
)

const defaultTopic = "audit.events"

// Publisher publishes audit events to Kafka.
type Publisher struct {
	producer sarama.SyncProducer
	topic    string
	logger   zerolog.Logger
}

// NewPublisher creates a new audit event publisher.
func NewPublisher(producer sarama.SyncProducer, topic string, logger zerolog.Logger) *Publisher {
	if topic == "" {
		topic = defaultTopic
	}
	return &Publisher{
		producer: producer,
		topic:    topic,
		logger:   logger.With().Str("component", "audit_publisher").Logger(),
	}
}

// Publish serializes the audit event to JSON and sends it to the Kafka topic.
//
// If the publisher was constructed with a nil producer (e.g. Kafka was
// unavailable at startup and the caller chose to proceed without an audit
// trail), Publish becomes a no-op rather than panicking. Callers can keep
// calling Publish unconditionally; the system gracefully degrades to "no
// audit trail" instead of failing the user-facing request.
func (p *Publisher) Publish(ctx context.Context, event *AuditEvent) error {
	if p == nil || p.producer == nil {
		return nil
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("audit: marshal event: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(event.EventID),
		Value: sarama.ByteEncoder(data),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.logger.Error().Err(err).
			Str("event_id", event.EventID).
			Str("action", event.Action).
			Msg("Failed to publish audit event")
		return fmt.Errorf("audit: publish event: %w", err)
	}

	p.logger.Debug().
		Str("event_id", event.EventID).
		Str("action", event.Action).
		Int32("partition", partition).
		Int64("offset", offset).
		Msg("Audit event published")

	return nil
}

// Close closes the underlying Kafka producer. If the publisher was constructed
// with a nil producer (graceful-degradation path, see Publish), Close is a
// no-op. This keeps `defer publisher.Close()` safe at every call site,
// regardless of whether Kafka was reachable at startup.
func (p *Publisher) Close() error {
	if p == nil || p.producer == nil {
		return nil
	}
	return p.producer.Close()
}
