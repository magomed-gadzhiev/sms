package kafka

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"
)

// BillingIntegrationHandler — интерфейс биллинга для consumer
type BillingIntegrationHandler interface {
	BillAttempt(ctx context.Context, cmd CascadeBillingCommand) error
}

// BillingConsumer — Kafka consumer для обработки биллинговых команд каскада
type BillingConsumer struct {
	brokers   []string
	group     string
	topic     string
	billing   BillingIntegrationHandler
	logger    zerolog.Logger
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewBillingConsumer создаёт новый BillingConsumer
func NewBillingConsumer(
	brokers []string,
	group string,
	topic string,
	billing BillingIntegrationHandler,
	logger zerolog.Logger,
) (*BillingConsumer, error) {
	return &BillingConsumer{
		brokers: brokers,
		group:   group,
		topic:   topic,
		billing: billing,
		logger:  logger.With().Str("component", "cascade_billing_consumer").Logger(),
	}, nil
}

// Start запускает BillingConsumer в фоновой горутине
func (c *BillingConsumer) Start(ctx context.Context) {
	childCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.run(childCtx)
	}()
}

// Stop останавливает BillingConsumer
func (c *BillingConsumer) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *BillingConsumer) run(ctx context.Context) {
	cfg := sarama.NewConfig()
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest

	cg, err := sarama.NewConsumerGroup(c.brokers, c.group, cfg)
	if err != nil {
		c.logger.Error().Err(err).Msg("ошибка создания billing consumer group")
		return
	}
	defer cg.Close()

	handler := &billingConsumerHandler{
		billing: c.billing,
		logger:  c.logger,
	}

	for {
		if err := cg.Consume(ctx, []string{c.topic}, handler); err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Error().Err(err).Msg("ошибка billing consumer")
		}
		if ctx.Err() != nil {
			return
		}
	}
}

type billingConsumerHandler struct {
	billing BillingIntegrationHandler
	logger  zerolog.Logger
}

func (h *billingConsumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *billingConsumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *billingConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		var cmd CascadeBillingCommand
		if err := json.Unmarshal(msg.Value, &cmd); err != nil {
			h.logger.Error().Err(err).Msg("ошибка десериализации CascadeBillingCommand")
			session.MarkMessage(msg, "")
			continue
		}

		if err := h.billing.BillAttempt(session.Context(), cmd); err != nil {
			h.logger.Error().Err(err).
				Str("attempt_id", cmd.AttemptID).
				Str("delivery_id", cmd.DeliveryID).
				Msg("ошибка биллинга попытки")
		}
		session.MarkMessage(msg, "")
	}
	return nil
}
