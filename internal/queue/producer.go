package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/config"
)

// Producer представляет Kafka producer для публикации сообщений
type Producer struct {
	producer sarama.SyncProducer
	config   *config.KafkaConfig
	logger   zerolog.Logger
}

// WaitForKafka ожидает готовности Kafka брокеров
func WaitForKafka(cfg *config.KafkaConfig, maxAttempts int, backoff time.Duration) error {
	logger := log.With().Str("component", "kafka_wait").Logger()
	
	saramaConfig := sarama.NewConfig()
	saramaConfig.Net.DialTimeout = 5 * time.Second
	saramaConfig.Net.ReadTimeout = 5 * time.Second
	saramaConfig.Net.WriteTimeout = 5 * time.Second
	saramaConfig.Metadata.Retry.Max = 1
	saramaConfig.Metadata.Timeout = 5 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Info().
			Int("attempt", attempt).
			Int("max_attempts", maxAttempts).
			Strs("brokers", cfg.Brokers).
			Msg("проверка доступности Kafka брокеров")

		client, err := sarama.NewClient(cfg.Brokers, saramaConfig)
		if err == nil {
			// Проверяем, что можем получить метаданные
			brokers := client.Brokers()
			client.Close()
			if len(brokers) > 0 {
				logger.Info().Msg("Kafka брокеры доступны")
				return nil
			}
			err = fmt.Errorf("нет доступных брокеров")
		}

		if attempt < maxAttempts {
			logger.Warn().
				Err(err).
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Msg("Kafka брокеры недоступны, повторная попытка")
			time.Sleep(backoff)
		} else {
			return fmt.Errorf("Kafka брокеры недоступны после %d попыток: %w", maxAttempts, err)
		}
	}

	return fmt.Errorf("Kafka брокеры недоступны после %d попыток", maxAttempts)
}

// NewProducer создает новый Kafka producer
func NewProducer(cfg *config.KafkaConfig) (*Producer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Retry.Max = cfg.MaxRetries
	saramaConfig.Producer.Retry.Backoff = cfg.RetryBackoff
	saramaConfig.Producer.Compression = sarama.CompressionSnappy
	saramaConfig.Producer.Idempotent = true
	saramaConfig.Net.MaxOpenRequests = 1

	producer, err := sarama.NewSyncProducer(cfg.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka producer: %w", err)
	}

	logger := log.With().Str("component", "kafka_producer").Logger()

	return &Producer{
		producer: producer,
		config:   cfg,
		logger:   logger,
	}, nil
}

// PublishOutgoing публикует исходящее SMS сообщение в топик sms.outgoing
func (p *Producer) PublishOutgoing(ctx context.Context, msg *KafkaMessage) error {
	return p.publish(ctx, p.config.TopicOutgoing, msg)
}

// PublishDLR публикует delivery receipt в топик sms.dlr
func (p *Producer) PublishDLR(ctx context.Context, dlr *DLRMessage) error {
	data, err := dlr.Serialize()
	if err != nil {
		return fmt.Errorf("ошибка сериализации DLR сообщения: %w", err)
	}

	message := &sarama.ProducerMessage{
		Topic: p.config.TopicDLR,
		Key:   sarama.StringEncoder(dlr.MessageID.String()),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("message_id"),
				Value: []byte(dlr.MessageID.String()),
			},
			{
				Key:   []byte("smpp_message_id"),
				Value: []byte(dlr.SMPPMessageID),
			},
		},
		Timestamp: time.Now(),
	}

	partition, offset, err := p.producer.SendMessage(message)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("topic", p.config.TopicDLR).
			Str("message_id", dlr.MessageID.String()).
			Msg("ошибка публикации DLR сообщения")
		return fmt.Errorf("ошибка публикации DLR сообщения: %w", err)
	}

	p.logger.Debug().
		Str("topic", p.config.TopicDLR).
		Str("message_id", dlr.MessageID.String()).
		Int32("partition", partition).
		Int64("offset", offset).
		Msg("DLR сообщение опубликовано")

	return nil
}

// PublishFailed публикует сообщение об ошибке в топик sms.failed
func (p *Producer) PublishFailed(ctx context.Context, failed *FailedMessage) error {
	data, err := failed.Serialize()
	if err != nil {
		return fmt.Errorf("ошибка сериализации failed сообщения: %w", err)
	}

	message := &sarama.ProducerMessage{
		Topic: p.config.TopicFailed,
		Key:   sarama.StringEncoder(failed.MessageID.String()),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("message_id"),
				Value: []byte(failed.MessageID.String()),
			},
			{
				Key:   []byte("error_code"),
				Value: []byte(failed.ErrorCode),
			},
		},
		Timestamp: time.Now(),
	}

	partition, offset, err := p.producer.SendMessage(message)
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("topic", p.config.TopicFailed).
			Str("message_id", failed.MessageID.String()).
			Msg("ошибка публикации failed сообщения")
		return fmt.Errorf("ошибка публикации failed сообщения: %w", err)
	}

	p.logger.Debug().
		Str("topic", p.config.TopicFailed).
		Str("message_id", failed.MessageID.String()).
		Int32("partition", partition).
		Int64("offset", offset).
		Msg("failed сообщение опубликовано")

	return nil
}

// publish публикует сообщение в указанный топик
func (p *Producer) publish(ctx context.Context, topic string, msg *KafkaMessage) error {
	data, err := msg.Serialize()
	if err != nil {
		return fmt.Errorf("ошибка сериализации сообщения: %w", err)
	}

	// Используем message_id как ключ для партиционирования
	key := sarama.StringEncoder(msg.MessageID.String())

	message := &sarama.ProducerMessage{
		Topic: topic,
		Key:   key,
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("message_id"),
				Value: []byte(msg.MessageID.String()),
			},
			{
				Key:   []byte("source"),
				Value: []byte(msg.Source),
			},
			{
				Key:   []byte("destination"),
				Value: []byte(msg.Destination),
			},
		},
		Timestamp: time.Now(),
	}

	// Retry механизм
	var lastErr error
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * p.config.RetryBackoff
			p.logger.Debug().
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Msg("повторная попытка публикации сообщения")
			time.Sleep(backoff)
		}

		partition, offset, err := p.producer.SendMessage(message)
		if err == nil {
			p.logger.Debug().
				Str("topic", topic).
				Str("message_id", msg.MessageID.String()).
				Int32("partition", partition).
				Int64("offset", offset).
				Int("attempt", attempt+1).
				Msg("сообщение опубликовано")
			return nil
		}

		lastErr = err
		p.logger.Warn().
			Err(err).
			Str("topic", topic).
			Str("message_id", msg.MessageID.String()).
			Int("attempt", attempt+1).
			Msg("ошибка публикации сообщения")
	}

	return fmt.Errorf("не удалось опубликовать сообщение после %d попыток: %w", p.config.MaxRetries+1, lastErr)
}

// Close закрывает producer
func (p *Producer) Close() error {
	if err := p.producer.Close(); err != nil {
		p.logger.Error().Err(err).Msg("ошибка закрытия producer")
		return err
	}
	p.logger.Info().Msg("Kafka producer закрыт")
	return nil
}

// Health проверяет здоровье producer
func (p *Producer) Health() error {
	// Простая проверка - producer должен быть не nil
	// В production можно добавить более детальную проверку соединения с брокерами
	if p.producer == nil {
		return fmt.Errorf("producer не инициализирован")
	}
	return nil
}

// AsyncProducer представляет высокопроизводительный Kafka async producer
type AsyncProducer struct {
	producer sarama.AsyncProducer
	config   *config.KafkaConfig
	logger   zerolog.Logger
	done     chan struct{}
}

// NewAsyncProducer создает новый асинхронный Kafka producer с настройками
// для высокой пропускной способности (R-002):
//   - Flush.Messages=500, Flush.Frequency=10ms
//   - Compression=Snappy, RequiredAcks=WaitForLocal
func NewAsyncProducer(cfg *config.KafkaConfig) (*AsyncProducer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForLocal
	saramaConfig.Producer.Compression = sarama.CompressionSnappy
	saramaConfig.Producer.Flush.Messages = 500
	saramaConfig.Producer.Flush.Frequency = 10 * time.Millisecond
	saramaConfig.Producer.Retry.Max = cfg.MaxRetries
	saramaConfig.Producer.Retry.Backoff = cfg.RetryBackoff

	producer, err := sarama.NewAsyncProducer(cfg.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka async producer: %w", err)
	}

	logger := log.With().Str("component", "kafka_async_producer").Logger()

	ap := &AsyncProducer{
		producer: producer,
		config:   cfg,
		logger:   logger,
		done:     make(chan struct{}),
	}

	go ap.handleResponses()

	return ap, nil
}

// handleResponses обрабатывает успешные и ошибочные ответы от AsyncProducer
// в фоновой горутине. Горутина завершается при закрытии каналов producer.
func (ap *AsyncProducer) handleResponses() {
	defer close(ap.done)

	for {
		select {
		case msg, ok := <-ap.producer.Successes():
			if !ok {
				// Канал закрыт — producer завершает работу
				return
			}
			ap.logger.Debug().
				Str("topic", msg.Topic).
				Int32("partition", msg.Partition).
				Int64("offset", msg.Offset).
				Msg("async сообщение доставлено")

		case err, ok := <-ap.producer.Errors():
			if !ok {
				// Канал закрыт — producer завершает работу
				return
			}
			// Трассировка: без key (message_id) и partition ошибку привязать
			// к конкретному сообщению в pipeline практически невозможно.
			keyStr := ""
			if err.Msg != nil && err.Msg.Key != nil {
				if encoded, encErr := err.Msg.Key.Encode(); encErr == nil {
					keyStr = string(encoded)
				}
			}
			ap.logger.Error().
				Err(err.Err).
				Str("topic", err.Msg.Topic).
				Int32("partition", err.Msg.Partition).
				Str("key", keyStr).
				Msg("ошибка доставки async сообщения")
		}
	}
}

// PublishAsync отправляет сообщение в AsyncProducer.Input() канал.
// Метод неблокирующий — сообщение ставится в очередь и отправляется
// пакетом согласно настройкам Flush.
func (ap *AsyncProducer) PublishAsync(topic string, key string, value []byte, headers []sarama.RecordHeader) {
	msg := &sarama.ProducerMessage{
		Topic:     topic,
		Key:       sarama.StringEncoder(key),
		Value:     sarama.ByteEncoder(value),
		Headers:   headers,
		Timestamp: time.Now(),
	}

	ap.producer.Input() <- msg
}

// Close закрывает AsyncProducer и ожидает завершения фоновой горутины
func (ap *AsyncProducer) Close() error {
	// AsyncClose запускает graceful shutdown: сбрасывает буферизованные
	// сообщения и закрывает каналы Successes/Errors.
	ap.producer.AsyncClose()

	// Ожидаем завершения горутины handleResponses
	<-ap.done

	ap.logger.Info().Msg("Kafka async producer закрыт")
	return nil
}
