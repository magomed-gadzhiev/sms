package infrastructure

import (
	"context"
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// LookupCompletedEvent represents a lookup completion event for analytics
type LookupCompletedEvent struct {
	MSISDN         string `json:"msisdn"`
	OperatorMCCMNC string `json:"operator_mccmnc"`
	NumberStatus   string `json:"number_status"`
	Cached         bool   `json:"cached"`
	LatencyMs      int    `json:"latency_ms"`
	Source         string `json:"source"`
	ClientID       string `json:"client_id"`
	RequestID      string `json:"request_id"`
	Timestamp      string `json:"timestamp"`
}

// HLREventPublisher publishes HLR-related events to Kafka
type HLREventPublisher struct {
	producer sarama.SyncProducer
	topic    string
	logger   zerolog.Logger
}

// NewHLREventPublisher creates a new HLR event publisher
func NewHLREventPublisher(producer sarama.SyncProducer) *HLREventPublisher {
	return &HLREventPublisher{
		producer: producer,
		topic:    "lookup.completed",
		logger:   log.With().Str("component", "hlr-event-publisher").Logger(),
	}
}

// PublishLookupCompleted publishes a lookup completion event
func (p *HLREventPublisher) PublishLookupCompleted(ctx context.Context, result *domain.LookupResult, source domain.LookupSource, clientID, requestID string, latencyMs int) {
	event := LookupCompletedEvent{
		MSISDN:         result.MSISDN,
		OperatorMCCMNC: result.OperatorMCCMNC,
		NumberStatus:   string(result.NumberStatus),
		Cached:         result.Cached,
		LatencyMs:      latencyMs,
		Source:         string(source),
		ClientID:       clientID,
		RequestID:      requestID,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(event)
	if err != nil {
		p.logger.Warn().Err(err).Msg("ошибка сериализации события lookup.completed")
		return
	}

	msg := &sarama.ProducerMessage{
		Topic:     p.topic,
		Key:       sarama.StringEncoder(result.MSISDN),
		Value:     sarama.ByteEncoder(data),
		Timestamp: time.Now(),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.logger.Warn().Err(err).
			Str("msisdn", result.MSISDN).
			Msg("ошибка публикации события lookup.completed")
		return
	}

	p.logger.Debug().
		Str("topic", p.topic).
		Str("msisdn", result.MSISDN).
		Int32("partition", partition).
		Int64("offset", offset).
		Msg("событие lookup.completed опубликовано")
}
