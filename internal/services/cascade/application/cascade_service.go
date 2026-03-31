package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	cascadekafka "github.com/smpp-server/smpp-server/internal/services/cascade/infrastructure/kafka"
	smschannel "github.com/smpp-server/smpp-server/internal/services/cascade/channels/sms"
)

// CascadeProducer — интерфейс Kafka producer для каскадного сервиса
type CascadeProducer interface {
	PublishStart(ctx context.Context, evt cascadekafka.CascadeStartEvent) error
	PublishAttemptSend(ctx context.Context, cmd cascadekafka.CascadeAttemptSendCommand) error
	PublishAttemptResult(ctx context.Context, evt cascadekafka.CascadeAttemptResultEvent) error
	PublishBilling(ctx context.Context, cmd cascadekafka.CascadeBillingCommand) error
}

// ReachabilityChecker — интерфейс проверки доступности канала
type ReachabilityChecker interface {
	CheckReachability(ctx context.Context, msisdn string, channelType domain.ChannelType) (bool, error)
}

// CascadeService управляет жизненным циклом каскадной доставки
type CascadeService struct {
	deliveries    domain.DeliveryRepository
	attempts      domain.AttemptRepository
	strategies    domain.StrategyRepository
	channels      domain.ChannelRepository
	producer      CascadeProducer
	channelAdapters map[domain.ChannelType]domain.Channel
	reachability  ReachabilityChecker
	metrics       *CascadeMetrics
	logger        zerolog.Logger
}

func NewCascadeService(
	deliveries domain.DeliveryRepository,
	attempts domain.AttemptRepository,
	strategies domain.StrategyRepository,
	channels domain.ChannelRepository,
	producer CascadeProducer,
	channelAdapters map[domain.ChannelType]domain.Channel,
	logger zerolog.Logger,
) *CascadeService {
	return &CascadeService{
		deliveries:      deliveries,
		attempts:        attempts,
		strategies:      strategies,
		channels:        channels,
		producer:        producer,
		channelAdapters: channelAdapters,
		metrics:         NewCascadeMetrics(),
		logger:          logger.With().Str("component", "cascade_service").Logger(),
	}
}

// SetReachabilityService инжектирует сервис проверки доступности (циклическая зависимость)
func (s *CascadeService) SetReachabilityService(r ReachabilityChecker) {
	s.reachability = r
}

// CreateDelivery создаёт новую доставку и публикует событие начала каскада
func (s *CascadeService) CreateDelivery(ctx context.Context, clientID, strategyID uuid.UUID, recipient, text, senderName, requestID string, messageID *uuid.UUID) (*domain.Delivery, error) {
	strategy, err := s.strategies.Get(ctx, strategyID)
	if err != nil {
		return nil, fmt.Errorf("get strategy: %w", err)
	}
	if !strategy.Active {
		return nil, fmt.Errorf("strategy is not active")
	}

	delivery := domain.NewDelivery(clientID, strategyID, recipient, text, senderName, "RUB", requestID, messageID)
	if err := s.deliveries.Create(ctx, delivery); err != nil {
		return nil, fmt.Errorf("create delivery: %w", err)
	}

	evt := cascadekafka.NewCascadeStartEvent(
		delivery.ID.String(),
		clientID.String(),
		strategyID.String(),
		recipient,
		text,
		senderName,
		requestID,
	)
	if err := s.producer.PublishStart(ctx, evt); err != nil {
		s.logger.Error().Err(err).Str("delivery_id", delivery.ID.String()).Msg("ошибка публикации CascadeStartEvent")
	}

	s.logger.Info().
		Str("delivery_id", delivery.ID.String()).
		Str("client_id", clientID.String()).
		Str("strategy_id", strategyID.String()).
		Str("recipient", recipient).
		Msg("доставка создана")

	return delivery, nil
}

// StartCascade обрабатывает событие начала каскада: переводит доставку в in_progress и запускает первый шаг
func (s *CascadeService) StartCascade(ctx context.Context, evt cascadekafka.CascadeStartEvent) error {
	deliveryID, err := uuid.Parse(evt.DeliveryID)
	if err != nil {
		return fmt.Errorf("parse delivery_id: %w", err)
	}

	delivery, err := s.deliveries.Get(ctx, deliveryID)
	if err != nil {
		return fmt.Errorf("get delivery: %w", err)
	}

	// Идемпотентность: уже обработано
	if delivery.Status != domain.DeliveryPending {
		s.logger.Debug().Str("delivery_id", deliveryID.String()).Str("status", string(delivery.Status)).Msg("доставка уже обрабатывается, пропускаем")
		return nil
	}

	strategy, err := s.strategies.Get(ctx, delivery.StrategyID)
	if err != nil {
		return fmt.Errorf("get strategy: %w", err)
	}

	if err := s.deliveries.UpdateStatus(ctx, deliveryID, domain.DeliveryInProgress, ""); err != nil {
		return fmt.Errorf("update delivery status to in_progress: %w", err)
	}
	delivery.Status = domain.DeliveryInProgress
	s.metrics.ActiveDeliveries.Inc()

	return s.executeNextStep(ctx, delivery, strategy, 0)
}

// ProcessAttemptResult обрабатывает результат попытки доставки
func (s *CascadeService) ProcessAttemptResult(ctx context.Context, evt cascadekafka.CascadeAttemptResultEvent) error {
	attemptID, err := uuid.Parse(evt.AttemptID)
	if err != nil {
		return fmt.Errorf("parse attempt_id: %w", err)
	}
	deliveryID, err := uuid.Parse(evt.DeliveryID)
	if err != nil {
		return fmt.Errorf("parse delivery_id: %w", err)
	}

	attempt, err := s.attempts.Get(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("get attempt: %w", err)
	}

	// Идемпотентность: already final
	if attempt.IsFinal() {
		s.logger.Debug().Str("attempt_id", attemptID.String()).Str("status", string(attempt.Status)).Msg("попытка уже финализирована")
		return nil
	}

	delivery, err := s.deliveries.Get(ctx, deliveryID)
	if err != nil {
		return fmt.Errorf("get delivery: %w", err)
	}

	if delivery.Status.IsTerminal() {
		// Доставка уже завершена - поздний дубликат
		if evt.Status == "delivered" {
			return s.HandleLateDuplicate(ctx, attemptID)
		}
		return nil
	}

	now := time.Now()
	resultAt := &now

	switch evt.Status {
	case "delivered":
		if err := s.attempts.UpdateStatus(ctx, attemptID, domain.AttemptDelivered, evt.ProviderRef, "", resultAt); err != nil {
			return fmt.Errorf("update attempt delivered: %w", err)
		}
		// Финализируем доставку
		if err := s.deliveries.UpdateStatus(ctx, deliveryID, domain.DeliveryDelivered, attempt.ChannelType); err != nil {
			return fmt.Errorf("update delivery delivered: %w", err)
		}
		s.logger.Info().
			Str("delivery_id", deliveryID.String()).
			Str("channel_type", attempt.ChannelType).
			Msg("доставка успешна")
		s.metrics.DeliveriesTotal.WithLabelValues("delivered").Inc()
		s.metrics.AttemptsTotal.WithLabelValues(attempt.ChannelType, "delivered").Inc()
		s.metrics.ActiveDeliveries.Dec()
		s.metrics.DeliveryDuration.WithLabelValues("delivered").Observe(time.Since(delivery.CreatedAt).Seconds())
		s.publishBillingForDelivery(ctx, delivery, attempt)

	case "failed":
		if err := s.attempts.UpdateStatus(ctx, attemptID, domain.AttemptFailed, evt.ProviderRef, evt.ErrorMessage, resultAt); err != nil {
			return fmt.Errorf("update attempt failed: %w", err)
		}
		s.metrics.AttemptsTotal.WithLabelValues(attempt.ChannelType, "failed").Inc()
		return s.tryNextStep(ctx, delivery, attempt)

	case "timeout":
		if err := s.attempts.UpdateStatus(ctx, attemptID, domain.AttemptTimeout, "", "timeout", resultAt); err != nil {
			return fmt.Errorf("update attempt timeout: %w", err)
		}
		s.metrics.AttemptsTotal.WithLabelValues(attempt.ChannelType, "timeout").Inc()
		return s.tryNextStep(ctx, delivery, attempt)
	}

	return nil
}

// HandleLateDuplicate помечает попытку как поздний дубликат
func (s *CascadeService) HandleLateDuplicate(ctx context.Context, attemptID uuid.UUID) error {
	now := time.Now()
	if err := s.attempts.UpdateStatus(ctx, attemptID, domain.AttemptLateDuplicate, "", "late duplicate", &now); err != nil {
		return fmt.Errorf("mark late duplicate: %w", err)
	}
	s.logger.Info().Str("attempt_id", attemptID.String()).Msg("поздний дубликат зафиксирован")
	return nil
}

func (s *CascadeService) tryNextStep(ctx context.Context, delivery *domain.Delivery, failedAttempt *domain.DeliveryAttempt) error {
	strategy, err := s.strategies.Get(ctx, delivery.StrategyID)
	if err != nil {
		return fmt.Errorf("get strategy: %w", err)
	}

	return s.executeNextStep(ctx, delivery, strategy, failedAttempt.StepOrder)
}

func (s *CascadeService) executeNextStep(ctx context.Context, delivery *domain.Delivery, strategy *domain.DeliveryStrategy, afterStep int) error {
	nextStep := strategy.NextStep(afterStep)
	if nextStep == nil {
		// Все шаги исчерпаны
		if err := s.deliveries.UpdateStatus(ctx, delivery.ID, domain.DeliveryFailed, ""); err != nil {
			return fmt.Errorf("mark delivery failed: %w", err)
		}
		s.logger.Info().Str("delivery_id", delivery.ID.String()).Msg("все шаги каскада исчерпаны, доставка не удалась")
		s.metrics.DeliveriesTotal.WithLabelValues("failed").Inc()
		s.metrics.ActiveDeliveries.Dec()
		s.metrics.DeliveryDuration.WithLabelValues("failed").Observe(time.Since(delivery.CreatedAt).Seconds())
		return nil
	}

	// Проверка доступности канала для оператора
	if s.reachability != nil {
		supported, err := s.reachability.CheckReachability(ctx, delivery.Recipient, nextStep.ChannelType)
		if err != nil {
			s.logger.Warn().Err(err).Str("channel_type", string(nextStep.ChannelType)).Msg("ошибка проверки доступности канала")
		} else if !supported {
			// Создаём skipped attempt и идём дальше
			channelCfg, _ := s.channels.Get(ctx, nextStep.ChannelID)
			attempt := domain.NewDeliveryAttempt(delivery.ID, nextStep.ChannelID, string(nextStep.ChannelType), nextStep.StepOrder, delivery.Currency)
			if err := s.attempts.Create(ctx, attempt); err != nil {
				return fmt.Errorf("create skipped attempt: %w", err)
			}
			now := time.Now()
			if err := s.attempts.UpdateStatus(ctx, attempt.ID, domain.AttemptSkipped, "", "channel not supported by operator", &now); err != nil {
				return fmt.Errorf("mark attempt skipped: %w", err)
			}
			s.logger.Info().
				Str("delivery_id", delivery.ID.String()).
				Str("channel_type", string(nextStep.ChannelType)).
				Str("attempt_id", attempt.ID.String()).
				Msg("канал недоступен, пропускаем шаг")
			_ = channelCfg
			return s.executeNextStep(ctx, delivery, strategy, nextStep.StepOrder)
		}
	}

	// Получаем конфигурацию канала
	channelCfg, err := s.channels.Get(ctx, nextStep.ChannelID)
	if err != nil {
		return fmt.Errorf("get channel config: %w", err)
	}

	// Создаём attempt
	attempt := domain.NewDeliveryAttempt(delivery.ID, nextStep.ChannelID, string(nextStep.ChannelType), nextStep.StepOrder, delivery.Currency)
	if err := s.attempts.Create(ctx, attempt); err != nil {
		return fmt.Errorf("create attempt: %w", err)
	}

	// Обновляем текущий шаг доставки
	if err := s.deliveries.UpdateStep(ctx, delivery.ID, nextStep.StepOrder); err != nil {
		s.logger.Warn().Err(err).Str("delivery_id", delivery.ID.String()).Msg("ошибка обновления шага доставки")
	}

	// Получаем адаптер канала
	adapter, ok := s.channelAdapters[nextStep.ChannelType]
	if !ok {
		return fmt.Errorf("no adapter for channel type: %s", nextStep.ChannelType)
	}

	// Добавляем delivery в контекст для адаптеров
	ctx = smschannel.WithDelivery(ctx, delivery)

	if err := adapter.Send(ctx, attempt, channelCfg); err != nil {
		s.logger.Error().Err(err).
			Str("delivery_id", delivery.ID.String()).
			Str("attempt_id", attempt.ID.String()).
			Str("channel_type", string(nextStep.ChannelType)).
			Msg("ошибка отправки через канал")
		// Не фатально - будет таймаут или следующая попытка
	}

	s.logger.Info().
		Str("delivery_id", delivery.ID.String()).
		Str("attempt_id", attempt.ID.String()).
		Str("channel_type", string(nextStep.ChannelType)).
		Int("step_order", nextStep.StepOrder).
		Msg("попытка доставки создана")

	s.metrics.AttemptsTotal.WithLabelValues(string(nextStep.ChannelType), "sent").Inc()

	return nil
}

func (s *CascadeService) publishBillingForDelivery(ctx context.Context, delivery *domain.Delivery, successAttempt *domain.DeliveryAttempt) {
	strategy, err := s.strategies.Get(ctx, delivery.StrategyID)
	if err != nil {
		s.logger.Error().Err(err).Msg("ошибка получения стратегии для биллинга")
		return
	}

	// Публикуем биллинг только для тарифицируемых попыток
	for _, step := range strategy.Steps {
		if !step.Billable {
			continue
		}
		// Ищем завершённые attempt для этого шага
		attempts, err := s.attempts.ListByDelivery(ctx, delivery.ID)
		if err != nil {
			continue
		}
		for _, a := range attempts {
			if a.StepOrder != step.StepOrder {
				continue
			}
			if a.Status != domain.AttemptDelivered && a.Status != domain.AttemptFailed && a.Status != domain.AttemptTimeout {
				continue
			}
			if a.Status == domain.AttemptLateDuplicate {
				continue
			}
			cmd := cascadekafka.NewCascadeBillingCommand(
				delivery.ID.String(),
				a.ID.String(),
				delivery.ClientID.String(),
				a.ChannelType,
				step.Billable,
				a.ID.String(), // idempotency_key = attempt_id
				delivery.RequestID,
			)
			if err := s.producer.PublishBilling(ctx, cmd); err != nil {
				s.logger.Error().Err(err).Str("attempt_id", a.ID.String()).Msg("ошибка публикации биллинга")
			}
		}
	}
}
