package queue

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/config"
)

// Producer представляет Kafka producer для публикации сообщений.
//
// Под флагом KAFKA_ASYNC_PUBLISH=true параллельно инициализируется
// sarama.AsyncProducer и publish() использует его вместо блокирующего
// SendMessage. Дубляж ресурсов приемлем: AsyncProducer живёт столько же,
// сколько и SyncProducer, shared-состояния нет.
type Producer struct {
	producer      sarama.SyncProducer
	asyncProducer sarama.AsyncProducer // nil если async-режим отключен
	asyncEnabled  bool
	config        *config.KafkaConfig
	logger        zerolog.Logger
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

// NewProducer создает новый Kafka producer.
//
// KAFKA_FAST_PRODUCER=true enables high-throughput mode for load testing:
//   - RequiredAcks=WaitForLocal (no cross-replica wait; single-broker setup
//     anyway makes WaitForAll == WaitForLocal semantically)
//   - MaxOpenRequests=5 (up to 5 in-flight requests per broker, allowing
//     parallel sends from concurrent goroutines — default 1 serializes ALL
//     producers through one in-flight slot)
//   - Idempotent=false (idempotence requires MaxOpenRequests<=5 + acks=all;
//     incompatible with the above under Sarama)
//
// Default (non-fast) preserves durability guarantees but is ~10x slower under
// high concurrency because sync SendMessage with MaxOpenRequests=1 serializes.
func NewProducer(cfg *config.KafkaConfig) (*Producer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.Retry.Max = cfg.MaxRetries
	saramaConfig.Producer.Retry.Backoff = cfg.RetryBackoff
	saramaConfig.Producer.Compression = sarama.CompressionSnappy

	if os.Getenv("KAFKA_FAST_PRODUCER") == "true" {
		saramaConfig.Producer.RequiredAcks = sarama.WaitForLocal
		saramaConfig.Producer.Idempotent = false
		saramaConfig.Net.MaxOpenRequests = 5
		saramaConfig.Producer.Flush.Frequency = 5 * time.Millisecond
		saramaConfig.Producer.Flush.Messages = 500
	} else {
		saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
		saramaConfig.Producer.Idempotent = true
		saramaConfig.Net.MaxOpenRequests = 1
	}

	producer, err := sarama.NewSyncProducer(cfg.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka producer: %w", err)
	}

	logger := log.With().Str("component", "kafka_producer").Logger()

	p := &Producer{
		producer: producer,
		config:   cfg,
		logger:   logger,
	}

	// Опциональный AsyncProducer — для inter-stage hot-path publish.
	// Активируется ENV KAFKA_ASYNC_PUBLISH=true. При ошибке создания —
	// падаем в sync (не fatal).
	//
	// ВАЖНО (load-test only): async-режим — fire-and-forget, caller получает
	// nil сразу после Input() <- message. Это нарушает at-least-once для
	// топика sms.outgoing (caller уже ответил клиенту HTTP 200 / SMPP
	// SUBMIT_OK), поэтому publish() форсит sync для TopicOutgoing
	// независимо от asyncEnabled. Под этим флагом async допустим только
	// для inter-stage топиков (sms.routed, sms.dlr, sms.failed) — там
	// retry-цепочка уже встроена в pipeline.
	if os.Getenv("KAFKA_ASYNC_PUBLISH") == "true" {
		asyncCfg := sarama.NewConfig()
		asyncCfg.Producer.Return.Successes = false
		asyncCfg.Producer.Return.Errors = true
		asyncCfg.Producer.RequiredAcks = sarama.WaitForLocal
		asyncCfg.Producer.Compression = sarama.CompressionSnappy
		asyncCfg.Producer.Flush.Frequency = 5 * time.Millisecond
		asyncCfg.Producer.Flush.Messages = 500
		asyncCfg.Producer.Flush.Bytes = 1 << 20 // 1 MB
		asyncCfg.Net.MaxOpenRequests = 8
		asyncCfg.Producer.Retry.Max = cfg.MaxRetries
		asyncCfg.Producer.Retry.Backoff = cfg.RetryBackoff

		ap, err := sarama.NewAsyncProducer(cfg.Brokers, asyncCfg)
		if err != nil {
			logger.Warn().Err(err).Msg("не удалось создать AsyncProducer, падаем в sync")
		} else {
			p.asyncProducer = ap
			p.asyncEnabled = true
			go func() {
				for e := range ap.Errors() {
					logger.Error().Err(e.Err).Str("topic", e.Msg.Topic).Msg("async publish error")
				}
			}()
			logger.Warn().
				Str("topic_outgoing_forced_sync", cfg.TopicOutgoing).
				Msg("KAFKA_ASYNC_PUBLISH=true — fire-and-forget включён для inter-stage; LOAD-TEST ONLY, в prod-конфигах флаг не должен быть выставлен")
		}
	}

	return p, nil
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

	// Fast path: если AsyncProducer активен И топик НЕ sms.outgoing —
	// fire-and-forget. Для sms.outgoing форсим sync, потому что caller
	// уже отрапортовал клиенту HTTP 200 / SMPP SUBMIT_OK и потеря
	// сообщения = реальная потеря для клиента, а не «pipeline catch-up».
	// Inter-stage топики (sms.routed/dlr/failed) переживают потерю —
	// retry-цепочка встроена в consumer'ы.
	if p.asyncEnabled && topic != p.config.TopicOutgoing {
		p.asyncProducer.Input() <- message
		return nil
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
	if p.asyncProducer != nil {
		if err := p.asyncProducer.Close(); err != nil {
			p.logger.Error().Err(err).Msg("ошибка закрытия async producer")
		}
	}
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
