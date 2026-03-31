package kafka

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"
)

// CascadeServiceOrchestrator — интерфейс для обработки каскадных событий
type CascadeServiceOrchestrator interface {
	StartCascade(ctx context.Context, evt CascadeStartEvent) error
	ProcessAttemptResult(ctx context.Context, evt CascadeAttemptResultEvent) error
}

// Orchestrator — Kafka consumer для оркестрации каскадной доставки
type Orchestrator struct {
	brokers       []string
	group         string
	topics        CascadeTopics
	cascadeService CascadeServiceOrchestrator
	logger        zerolog.Logger
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewOrchestrator создаёт новый Orchestrator
func NewOrchestrator(
	brokers []string,
	group string,
	topics CascadeTopics,
	cascadeService CascadeServiceOrchestrator,
	logger zerolog.Logger,
) (*Orchestrator, error) {
	return &Orchestrator{
		brokers:        brokers,
		group:          group,
		topics:         topics,
		cascadeService: cascadeService,
		logger:         logger.With().Str("component", "cascade_orchestrator").Logger(),
	}, nil
}

// Start запускает Orchestrator consumer в фоновой горутине
func (o *Orchestrator) Start(ctx context.Context) {
	childCtx, cancel := context.WithCancel(ctx)
	o.cancel = cancel

	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		o.run(childCtx)
	}()
}

// Stop останавливает Orchestrator consumer
func (o *Orchestrator) Stop() {
	if o.cancel != nil {
		o.cancel()
	}
	o.wg.Wait()
}

func (o *Orchestrator) run(ctx context.Context) {
	cfg := sarama.NewConfig()
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	cfg.Consumer.Group.Session.Timeout = 30e9   // 30 seconds
	cfg.Consumer.Group.Heartbeat.Interval = 3e9 // 3 seconds
	cfg.Consumer.MaxProcessingTime = 500e6      // 500ms

	cg, err := sarama.NewConsumerGroup(o.brokers, o.group, cfg)
	if err != nil {
		o.logger.Error().Err(err).Msg("ошибка создания consumer group")
		return
	}
	defer cg.Close()

	topics := []string{o.topics.Start, o.topics.AttemptResult}
	handler := &orchestratorHandler{
		cascadeService: o.cascadeService,
		logger:         o.logger,
	}

	for {
		if err := cg.Consume(ctx, topics, handler); err != nil {
			if ctx.Err() != nil {
				return
			}
			o.logger.Error().Err(err).Msg("ошибка обработки сообщений")
		}
		if ctx.Err() != nil {
			return
		}
	}
}

type orchestratorHandler struct {
	cascadeService CascadeServiceOrchestrator
	logger         zerolog.Logger
}

func (h *orchestratorHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *orchestratorHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *orchestratorHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.processMessage(session.Context(), msg)
		session.MarkMessage(msg, "")
	}
	return nil
}

func (h *orchestratorHandler) processMessage(ctx context.Context, msg *sarama.ConsumerMessage) {
	switch msg.Topic {
	case "cascade.start":
		var evt CascadeStartEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			h.logger.Error().Err(err).Msg("ошибка десериализации CascadeStartEvent")
			return
		}
		if err := h.cascadeService.StartCascade(ctx, evt); err != nil {
			h.logger.Error().Err(err).
				Str("delivery_id", evt.DeliveryID).
				Msg("ошибка обработки CascadeStartEvent")
		}
	default:
		// cascade.attempt.result
		var evt CascadeAttemptResultEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			h.logger.Error().Err(err).Msg("ошибка десериализации CascadeAttemptResultEvent")
			return
		}
		if err := h.cascadeService.ProcessAttemptResult(ctx, evt); err != nil {
			h.logger.Error().Err(err).
				Str("attempt_id", evt.AttemptID).
				Str("delivery_id", evt.DeliveryID).
				Msg("ошибка обработки CascadeAttemptResultEvent")
		}
	}
}
