package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/config"
)

// EventPublisher реализует domain.EventPublisher
type EventPublisher struct {
	producer sarama.SyncProducer
	topicBalance    string
	topicTransaction string
	logger   zerolog.Logger
}

// NewEventPublisher создает новый publisher событий биллинга
func NewEventPublisher(cfg *config.KafkaConfig, balanceTopic, transactionTopic string) (*EventPublisher, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Retry.Max = cfg.MaxRetries
	saramaConfig.Producer.Retry.Backoff = cfg.RetryBackoff
	saramaConfig.Producer.Compression = sarama.CompressionSnappy
	saramaConfig.Producer.Idempotent = true
	saramaConfig.Net.MaxOpenRequests = 1
	
	syncProducer, err := sarama.NewSyncProducer(cfg.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}
	
	return &EventPublisher{
		producer:         syncProducer,
		topicBalance:     balanceTopic,
		topicTransaction: transactionTopic,
		logger:           log.With().Str("component", "billing-event-publisher").Logger(),
	}, nil
}

// PublishBalanceChanged публикует событие изменения баланса
func (p *EventPublisher) PublishBalanceChanged(ctx context.Context, clientID, balance, currency string) error {
	event := map[string]interface{}{
		"client_id": clientID,
		"balance":   balance,
		"currency":  currency,
		"timestamp": time.Now().Unix(),
		"event_type": "balance.changed",
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal balance changed event: %w", err)
	}

	message := &sarama.ProducerMessage{
		Topic: p.topicBalance,
		Key:   sarama.StringEncoder(clientID),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("event_type"),
				Value: []byte("balance.changed"),
			},
			{
				Key:   []byte("client_id"),
				Value: []byte(clientID),
			},
		},
		Timestamp: time.Now(),
	}

	_, _, err = p.producer.SendMessage(message)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("client_id", clientID).
			Msg("ошибка публикации события balance.changed")
		return fmt.Errorf("failed to publish balance changed event: %w", err)
	}

	p.logger.Debug().
		Str("client_id", clientID).
		Str("balance", balance).
		Msg("событие balance.changed опубликовано")

	return nil
}

// PublishTransactionCompleted публикует событие завершения транзакции
func (p *EventPublisher) PublishTransactionCompleted(ctx context.Context, transactionID, clientID, transactionType, amount, currency string) error {
	event := map[string]interface{}{
		"transaction_id": transactionID,
		"client_id":      clientID,
		"type":           transactionType,
		"amount":         amount,
		"currency":       currency,
		"timestamp":      time.Now().Unix(),
		"event_type":     "transaction.completed",
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction completed event: %w", err)
	}

	message := &sarama.ProducerMessage{
		Topic: p.topicTransaction,
		Key:   sarama.StringEncoder(transactionID),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("event_type"),
				Value: []byte("transaction.completed"),
			},
			{
				Key:   []byte("transaction_id"),
				Value: []byte(transactionID),
			},
			{
				Key:   []byte("client_id"),
				Value: []byte(clientID),
			},
		},
		Timestamp: time.Now(),
	}

	_, _, err = p.producer.SendMessage(message)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("transaction_id", transactionID).
			Msg("ошибка публикации события transaction.completed")
		return fmt.Errorf("failed to publish transaction completed event: %w", err)
	}

	p.logger.Debug().
		Str("transaction_id", transactionID).
		Str("client_id", clientID).
		Msg("событие transaction.completed опубликовано")

	return nil
}

// PublishBalanceLow публикует событие низкого баланса
func (p *EventPublisher) PublishBalanceLow(ctx context.Context, clientID, balance, threshold, currency string) error {
	event := map[string]interface{}{
		"client_id": clientID,
		"balance":   balance,
		"threshold": threshold,
		"currency":  currency,
		"timestamp": time.Now().Unix(),
		"event_type": "balance.low",
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal balance low event: %w", err)
	}

	message := &sarama.ProducerMessage{
		Topic: p.topicBalance,
		Key:   sarama.StringEncoder(clientID),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("event_type"),
				Value: []byte("balance.low"),
			},
			{
				Key:   []byte("client_id"),
				Value: []byte(clientID),
			},
		},
		Timestamp: time.Now(),
	}

	_, _, err = p.producer.SendMessage(message)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("client_id", clientID).
			Msg("ошибка публикации события balance.low")
		return fmt.Errorf("failed to publish balance low event: %w", err)
	}

	p.logger.Warn().
		Str("client_id", clientID).
		Str("balance", balance).
		Str("threshold", threshold).
		Msg("баланс клиента упал ниже порогового значения")

	return nil
}

// Close закрывает producer
func (p *EventPublisher) Close() error {
	if err := p.producer.Close(); err != nil {
		p.logger.Error().Err(err).Msg("ошибка закрытия producer")
		return err
	}
	return nil
}
