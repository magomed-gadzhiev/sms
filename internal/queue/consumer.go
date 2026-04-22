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

// ---------------------------------------------------------------------------
// maxProcessingTime — общий лимит на «тишину» между чтениями сообщений из
// sarama claim channel (см. комментарий в NewBatchConsumer). Вынесен в
// константу, чтобы slowBatchWarnThreshold вычислялся от него, а не разъезжался.
// TODO: если разным сервисам (persist быстрый vs sender с TarifyMessage+SMPP)
// понадобятся разные значения — вынести в KafkaConfig.
const maxProcessingTime = 30 * time.Second

// slowBatchWarnThreshold — ~50% от maxProcessingTime. Diagnostic-сигнал для
// поиска «тихого зависания»: если вы видите эти WARN часто, значит порог
// maxProcessingTime близок и его пора повышать или разгружать handler.
var slowBatchWarnThreshold = maxProcessingTime / 2

// BatchConsumer — batch consumption mode (R-002)
// ---------------------------------------------------------------------------

// BatchHandler обрабатывает пакет сообщений из Kafka.
type BatchHandler func(ctx context.Context, msgs []*sarama.ConsumerMessage, session sarama.ConsumerGroupSession) error

// BatchConsumer собирает сообщения в пакеты по размеру или таймеру
// (что сработает первым) и вызывает BatchHandler для каждого пакета.
type BatchConsumer struct {
	consumer     sarama.ConsumerGroup
	config       *config.KafkaConfig
	topics       []string
	batchSize    int
	batchTimeout time.Duration
	logger       zerolog.Logger
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewBatchConsumer создает BatchConsumer с CooperativeStickyAssignor (R-004).
func NewBatchConsumer(cfg *config.KafkaConfig, groupID string, topics []string, batchSize int, batchTimeout time.Duration) (*BatchConsumer, error) {
	saramaCfg := sarama.NewConfig()
	saramaCfg.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategySticky()
	saramaCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	saramaCfg.Consumer.Return.Errors = true
	saramaCfg.Version = sarama.V2_6_0_0
	saramaCfg.Consumer.Fetch.Default = 1048576 // 1 MiB
	// MaxProcessingTime — sarama-таймаут на неблокируемый write сообщения в
	// claim.Messages(). В batch-режиме handler задерживает чтение из канала на
	// время flush (по размеру батча или batchTimeout). Если «тишина» на канале
	// превышает MaxProcessingTime, sarama приостанавливает fetch partition;
	// под нагрузкой это сочетается с последующим heartbeat-голоданием и
	// выглядит как «тихое зависание» consumer-группы (stable, lag=0, без
	// обработки). Handler в sender-стадии стабильно выполняет TarifyMessage +
	// SMPP submit + CommitCharge за ~1.5–2 s на сообщение; при batchSize до
	// нескольких десятков flush может занимать единицы секунд. 30 s даёт
	// значимый запас и остаётся заметно меньше SessionTimeout (чтобы
	// sessionTimeout оставался реальным предохранителем настоящего deadlock'а
	// handler'а, а не маскировался). Требование: MaxProcessingTime <
	// cfg.SessionTimeout.
	saramaCfg.Consumer.MaxProcessingTime = maxProcessingTime
	saramaCfg.Consumer.Group.Session.Timeout = cfg.SessionTimeout
	saramaCfg.Consumer.Group.Heartbeat.Interval = cfg.HeartbeatInterval

	group, err := sarama.NewConsumerGroup(cfg.Brokers, groupID, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Kafka batch consumer group: %w", err)
	}

	logger := log.With().
		Str("component", "kafka_batch_consumer").
		Str("group", groupID).
		Logger()

	ctx, cancel := context.WithCancel(context.Background())

	return &BatchConsumer{
		consumer:     group,
		config:       cfg,
		topics:       topics,
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
	}, nil
}

// ConsumeBatches запускает цикл потребления. Блокирует до отмены ctx
// или внутренней ошибки. handler вызывается для каждого собранного пакета.
func (bc *BatchConsumer) ConsumeBatches(ctx context.Context, handler BatchHandler) error {
	h := &batchConsumerGroupHandler{
		batchSize:    bc.batchSize,
		batchTimeout: bc.batchTimeout,
		handler:      handler,
		logger:       bc.logger,
	}

	// Обработка ошибок consumer group в фоне.
	bc.wg.Add(1)
	go func() {
		defer bc.wg.Done()
		for err := range bc.consumer.Errors() {
			bc.logger.Error().Err(err).Msg("batch consumer error")
		}
	}()

	// Основной цикл consume. sarama перезапускает Consume при ребалансе,
	// поэтому крутим в цикле.
	for {
		select {
		case <-ctx.Done():
			bc.logger.Info().Msg("batch consumer остановлен (внешний ctx)")
			return ctx.Err()
		case <-bc.ctx.Done():
			bc.logger.Info().Msg("batch consumer остановлен (Close)")
			return bc.ctx.Err()
		default:
			if err := bc.consumer.Consume(ctx, bc.topics, h); err != nil {
				bc.logger.Error().Err(err).Msg("ошибка batch consume")
				// Небольшая задержка перед повторной попыткой.
				select {
				case <-time.After(5 * time.Second):
				case <-ctx.Done():
					return ctx.Err()
				case <-bc.ctx.Done():
					return bc.ctx.Err()
				}
			}
		}
	}
}

// Close выполняет graceful shutdown BatchConsumer.
func (bc *BatchConsumer) Close() error {
	bc.logger.Info().Msg("закрытие batch consumer")
	bc.cancel()
	bc.wg.Wait()

	if err := bc.consumer.Close(); err != nil {
		bc.logger.Error().Err(err).Msg("ошибка закрытия batch consumer")
		return err
	}

	bc.logger.Info().Msg("batch consumer закрыт")
	return nil
}

// batchConsumerGroupHandler реализует sarama.ConsumerGroupHandler
// с пакетной обработкой сообщений.
type batchConsumerGroupHandler struct {
	batchSize    int
	batchTimeout time.Duration
	handler      BatchHandler
	logger       zerolog.Logger
}

func (h *batchConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	h.logger.Info().Msg("batch consumer group session setup")
	return nil
}

func (h *batchConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.logger.Info().Msg("batch consumer group session cleanup")
	return nil
}

// ConsumeClaim собирает сообщения в пакеты и вызывает handler.
func (h *batchConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	batch := make([]*sarama.ConsumerMessage, 0, h.batchSize)
	timer := time.NewTimer(h.batchTimeout)
	defer timer.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		start := time.Now()
		err := h.handler(session.Context(), batch, session)
		duration := time.Since(start)

		if err != nil {
			h.logger.Error().
				Err(err).
				Int("batch_size", len(batch)).
				Dur("duration", duration).
				Msg("ошибка обработки пакета")
			// При ошибке не помечаем сообщения — они будут повторно доставлены.
		} else {
			for _, msg := range batch {
				session.MarkMessage(msg, "")
			}
			// Пороговое предупреждение: приближение к MaxProcessingTime =
			// диагностический сигнал для будущего репро «тихого зависания».
			if duration > slowBatchWarnThreshold {
				h.logger.Warn().
					Int("batch_size", len(batch)).
					Dur("duration", duration).
					Dur("threshold", slowBatchWarnThreshold).
					Msg("медленный batch — приближение к MaxProcessingTime")
			}
			h.logger.Debug().
				Int("batch_size", len(batch)).
				Dur("duration", duration).
				Msg("пакет обработан")
		}

		// Сброс пакета.
		batch = batch[:0]
	}

	for {
		select {
		case msg := <-claim.Messages():
			if msg == nil {
				// Канал закрыт — flush остатки и выход.
				flush()
				return nil
			}

			batch = append(batch, msg)
			if len(batch) >= h.batchSize {
				flush()
				// Сброс таймера после flush по размеру.
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(h.batchTimeout)
			}

		case <-timer.C:
			flush()
			timer.Reset(h.batchTimeout)

		case <-session.Context().Done():
			// Сессия завершается — flush остатки.
			flush()
			return nil
		}
	}
}
