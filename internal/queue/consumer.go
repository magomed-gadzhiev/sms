package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/config"
)

// MessageHandler обрабатывает сообщения из Kafka
type MessageHandler func(ctx context.Context, msg *KafkaMessage) error

// DLRHandler обрабатывает DLR сообщения из Kafka
type DLRHandler func(ctx context.Context, dlr *DLRMessage) error

// FailedHandler обрабатывает failed сообщения из Kafka
type FailedHandler func(ctx context.Context, failed *FailedMessage) error

// Consumer представляет Kafka consumer для чтения сообщений
type Consumer struct {
	consumer      sarama.ConsumerGroup
	config        *config.KafkaConfig
	logger        zerolog.Logger
	handler       MessageHandler
	dlrHandler    DLRHandler
	failedHandler FailedHandler
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewConsumer создает новый Kafka consumer
func NewConsumer(cfg *config.KafkaConfig, handler MessageHandler, dlrHandler DLRHandler, failedHandler FailedHandler) (*Consumer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	saramaConfig.Consumer.Group.Session.Timeout = cfg.SessionTimeout
	saramaConfig.Consumer.Group.Heartbeat.Interval = cfg.HeartbeatInterval
	saramaConfig.Consumer.Return.Errors = true
	saramaConfig.Version = sarama.V2_6_0_0

	consumer, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.ConsumerGroup, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka consumer: %w", err)
	}

	logger := log.With().Str("component", "kafka_consumer").Logger()
	ctx, cancel := context.WithCancel(context.Background())

	return &Consumer{
		consumer:      consumer,
		config:        cfg,
		logger:        logger,
		handler:       handler,
		dlrHandler:    dlrHandler,
		failedHandler: failedHandler,
		ctx:           ctx,
		cancel:        cancel,
	}, nil
}

// ConsumeOutgoing начинает потребление сообщений из топика sms.outgoing
func (c *Consumer) ConsumeOutgoing() error {
	return c.consume(c.config.TopicOutgoing, c.handleOutgoingMessage)
}

// ConsumeDLR начинает потребление сообщений из топика sms.dlr
func (c *Consumer) ConsumeDLR() error {
	return c.consume(c.config.TopicDLR, c.handleDLRMessage)
}

// ConsumeFailed начинает потребление сообщений из топика sms.failed
func (c *Consumer) ConsumeFailed() error {
	return c.consume(c.config.TopicFailed, c.handleFailedMessage)
}

// consume начинает потребление сообщений из указанного топика
func (c *Consumer) consume(topic string, handler func(*sarama.ConsumerMessage) error) error {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		consumerGroupHandler := &consumerGroupHandler{
			topic:   topic,
			handler: handler,
			logger:  c.logger,
		}

		for {
			select {
			case <-c.ctx.Done():
				c.logger.Info().Str("topic", topic).Msg("остановка consumer")
				return
			default:
				err := c.consumer.Consume(c.ctx, []string{topic}, consumerGroupHandler)
				if err != nil {
					c.logger.Error().
						Err(err).
						Str("topic", topic).
						Msg("ошибка потребления сообщений")
					// Небольшая задержка перед повторной попыткой
					time.Sleep(5 * time.Second)
				}
			}
		}
	}()

	// Обработка ошибок consumer
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for err := range c.consumer.Errors() {
			c.logger.Error().
				Err(err).
				Str("topic", topic).
				Msg("ошибка consumer")
		}
	}()

	c.logger.Info().Str("topic", topic).Msg("consumer запущен")
	return nil
}

// handleOutgoingMessage обрабатывает исходящее сообщение
func (c *Consumer) handleOutgoingMessage(msg *sarama.ConsumerMessage) error {
	kafkaMsg, err := Deserialize(msg.Value)
	if err != nil {
		return fmt.Errorf("ошибка десериализации сообщения: %w", err)
	}

	if c.handler == nil {
		return fmt.Errorf("handler для outgoing сообщений не установлен")
	}

	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	return c.handler(ctx, kafkaMsg)
}

// handleDLRMessage обрабатывает DLR сообщение
func (c *Consumer) handleDLRMessage(msg *sarama.ConsumerMessage) error {
	dlr, err := DeserializeDLR(msg.Value)
	if err != nil {
		return fmt.Errorf("ошибка десериализации DLR сообщения: %w", err)
	}

	if c.dlrHandler == nil {
		return fmt.Errorf("handler для DLR сообщений не установлен")
	}

	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	return c.dlrHandler(ctx, dlr)
}

// handleFailedMessage обрабатывает failed сообщение
func (c *Consumer) handleFailedMessage(msg *sarama.ConsumerMessage) error {
	failed, err := DeserializeFailed(msg.Value)
	if err != nil {
		return fmt.Errorf("ошибка десериализации failed сообщения: %w", err)
	}

	if c.failedHandler == nil {
		return fmt.Errorf("handler для failed сообщений не установлен")
	}

	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	return c.failedHandler(ctx, failed)
}

// Close закрывает consumer с graceful shutdown
func (c *Consumer) Close() error {
	c.logger.Info().Msg("закрытие consumer")
	c.cancel()
	c.wg.Wait()

	if err := c.consumer.Close(); err != nil {
		c.logger.Error().Err(err).Msg("ошибка закрытия consumer")
		return err
	}

	c.logger.Info().Msg("Kafka consumer закрыт")
	return nil
}

// consumerGroupHandler реализует sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	topic   string
	handler func(*sarama.ConsumerMessage) error
	logger  zerolog.Logger
}

// Setup вызывается перед началом потребления
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	h.logger.Info().Str("topic", h.topic).Msg("consumer group session setup")
	return nil
}

// Cleanup вызывается после завершения потребления
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.logger.Info().Str("topic", h.topic).Msg("consumer group session cleanup")
	return nil
}

// ConsumeClaim обрабатывает сообщения из claim
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			start := time.Now()
			err := h.handler(message)
			duration := time.Since(start)

			if err != nil {
				h.logger.Error().
					Err(err).
					Str("topic", message.Topic).
					Int32("partition", message.Partition).
					Int64("offset", message.Offset).
					Dur("duration", duration).
					Msg("ошибка обработки сообщения")

				// В production здесь можно добавить логику retry или отправки в DLQ
				// Пока просто логируем ошибку
				// session.MarkMessage(message, "") - не помечаем как обработанное при ошибке
			} else {
				session.MarkMessage(message, "")
				h.logger.Debug().
					Str("topic", message.Topic).
					Int32("partition", message.Partition).
					Int64("offset", message.Offset).
					Dur("duration", duration).
					Msg("сообщение обработано")
			}

		case <-session.Context().Done():
			return nil
		}
	}
}
