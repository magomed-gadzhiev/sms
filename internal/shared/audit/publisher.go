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
func (p *Publisher) Publish(ctx context.Context, event *AuditEvent) error {
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

// Close closes the underlying Kafka producer.
func (p *Publisher) Close() error {
	return p.producer.Close()
}
