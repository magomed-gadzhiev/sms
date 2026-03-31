package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// CascadeTopics содержит имена Kafka-топиков для каскадной доставки
type CascadeTopics struct {
	Start         string // cascade.start
	AttemptSend   string // cascade.attempt.send
	AttemptResult string // cascade.attempt.result
	Billing       string // cascade.billing
}

// CascadeProducer — Kafka producer для каскадной доставки
type CascadeProducer struct {
	producer sarama.SyncProducer
	topics   CascadeTopics
	logger   zerolog.Logger
}

// NewCascadeProducer создаёт новый CascadeProducer
func NewCascadeProducer(producer sarama.SyncProducer, topics CascadeTopics) *CascadeProducer {
	return &CascadeProducer{
		producer: producer,
		topics:   topics,
		logger:   log.With().Str("component", "cascade_kafka_producer").Logger(),
	}
}

// PublishStart публикует событие начала каскадной доставки
func (p *CascadeProducer) PublishStart(ctx context.Context, evt CascadeStartEvent) error {
	data, err := evt.Serialize()
	if err != nil {
		return fmt.Errorf("ошибка сериализации CascadeStartEvent: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: p.topics.Start,
		Key:   sarama.StringEncoder(evt.DeliveryID),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("delivery_id"), Value: []byte(evt.DeliveryID)},
			{Key: []byte("request_id"), Value: []byte(evt.RequestID)},
		},
		Timestamp: time.Now(),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("topic", p.topics.Start).
			Str("delivery_id", evt.DeliveryID).
			Str("request_id", evt.RequestID).
			Msg("ошибка публикации CascadeStartEvent")
		return fmt.Errorf("ошибка публикации CascadeStartEvent: %w", err)
	}

	p.logger.Debug().
		Str("topic", p.topics.Start).
		Str("delivery_id", evt.DeliveryID).
		Str("request_id", evt.RequestID).
		Int32("partition", partition).
		Int64("offset", offset).
		Msg("CascadeStartEvent опубликован")

	return nil
}

// PublishAttemptSend публикует команду на отправку попытки доставки
func (p *CascadeProducer) PublishAttemptSend(ctx context.Context, cmd CascadeAttemptSendCommand) error {
	data, err := cmd.Serialize()
	if err != nil {
		return fmt.Errorf("ошибка сериализации CascadeAttemptSendCommand: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: p.topics.AttemptSend,
		Key:   sarama.StringEncoder(cmd.DeliveryID),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("attempt_id"), Value: []byte(cmd.AttemptID)},
			{Key: []byte("delivery_id"), Value: []byte(cmd.DeliveryID)},
			{Key: []byte("request_id"), Value: []byte(cmd.RequestID)},
		},
		Timestamp: time.Now(),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("topic", p.topics.AttemptSend).
			Str("attempt_id", cmd.AttemptID).
			Str("delivery_id", cmd.DeliveryID).
			Msg("ошибка публикации CascadeAttemptSendCommand")
		return fmt.Errorf("ошибка публикации CascadeAttemptSendCommand: %w", err)
	}

	p.logger.Debug().
		Str("topic", p.topics.AttemptSend).
		Str("attempt_id", cmd.AttemptID).
		Str("delivery_id", cmd.DeliveryID).
		Str("channel_type", cmd.ChannelType).
		Int32("partition", partition).
		Int64("offset", offset).
		Msg("CascadeAttemptSendCommand опубликован")

	return nil
}

// PublishAttemptResult публикует событие результата попытки доставки
func (p *CascadeProducer) PublishAttemptResult(ctx context.Context, evt CascadeAttemptResultEvent) error {
	data, err := evt.Serialize()
	if err != nil {
		return fmt.Errorf("ошибка сериализации CascadeAttemptResultEvent: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: p.topics.AttemptResult,
		Key:   sarama.StringEncoder(evt.DeliveryID),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("attempt_id"), Value: []byte(evt.AttemptID)},
			{Key: []byte("delivery_id"), Value: []byte(evt.DeliveryID)},
		},
		Timestamp: time.Now(),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("topic", p.topics.AttemptResult).
			Str("attempt_id", evt.AttemptID).
			Str("delivery_id", evt.DeliveryID).
			Msg("ошибка публикации CascadeAttemptResultEvent")
		return fmt.Errorf("ошибка публикации CascadeAttemptResultEvent: %w", err)
	}

	p.logger.Debug().
		Str("topic", p.topics.AttemptResult).
		Str("attempt_id", evt.AttemptID).
		Str("delivery_id", evt.DeliveryID).
		Str("status", evt.Status).
		Int32("partition", partition).
		Int64("offset", offset).
		Msg("CascadeAttemptResultEvent опубликован")

	return nil
}

// PublishBilling публикует команду на биллинг попытки доставки
func (p *CascadeProducer) PublishBilling(ctx context.Context, cmd CascadeBillingCommand) error {
	data, err := cmd.Serialize()
	if err != nil {
		return fmt.Errorf("ошибка сериализации CascadeBillingCommand: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: p.topics.Billing,
		Key:   sarama.StringEncoder(cmd.DeliveryID),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("attempt_id"), Value: []byte(cmd.AttemptID)},
			{Key: []byte("delivery_id"), Value: []byte(cmd.DeliveryID)},
			{Key: []byte("idempotency_key"), Value: []byte(cmd.IdempotencyKey)},
			{Key: []byte("request_id"), Value: []byte(cmd.RequestID)},
		},
		Timestamp: time.Now(),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("topic", p.topics.Billing).
			Str("attempt_id", cmd.AttemptID).
			Str("delivery_id", cmd.DeliveryID).
			Msg("ошибка публикации CascadeBillingCommand")
		return fmt.Errorf("ошибка публикации CascadeBillingCommand: %w", err)
	}

	p.logger.Debug().
		Str("topic", p.topics.Billing).
		Str("attempt_id", cmd.AttemptID).
		Str("delivery_id", cmd.DeliveryID).
		Str("channel_type", cmd.ChannelType).
		Bool("billable", cmd.Billable).
		Int32("partition", partition).
		Int64("offset", offset).
		Msg("CascadeBillingCommand опубликован")

	return nil
}
