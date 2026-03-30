package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// TarificationService основной сервис тарификации
type TarificationService struct {
	senderRepo      domain.SenderRegistrationRepository
	planRepo        domain.TariffPlanRepository
	periodRepo      domain.TariffPeriodRepository
	tierRepo        domain.TariffTierRepository
	usageRepo       domain.UsageCounterRepository
	logRepo         domain.TarificationLogRepository
	prepaidRepo     domain.PrepaidFeeRepository
	saga            *SagaOrchestrator
	eventPublisher  domain.EventPublisher
	strategies      map[domain.TarificationStrategy]BillingStrategy
}

// NewTarificationService создает новый сервис тарификации
func NewTarificationService(
	senderRepo domain.SenderRegistrationRepository,
	planRepo domain.TariffPlanRepository,
	periodRepo domain.TariffPeriodRepository,
	tierRepo domain.TariffTierRepository,
	usageRepo domain.UsageCounterRepository,
	logRepo domain.TarificationLogRepository,
	prepaidRepo domain.PrepaidFeeRepository,
	billingClient billingv1.BillingServiceClient,
	eventPublisher domain.EventPublisher,
) *TarificationService {
	return &TarificationService{
		senderRepo:     senderRepo,
		planRepo:       planRepo,
		periodRepo:     periodRepo,
		tierRepo:       tierRepo,
		usageRepo:      usageRepo,
		logRepo:        logRepo,
		prepaidRepo:    prepaidRepo,
		saga:           NewSagaOrchestrator(billingClient),
		eventPublisher: eventPublisher,
		strategies: map[domain.TarificationStrategy]BillingStrategy{
			domain.StrategyFixed:            NewFixedStrategy(),
			domain.StrategyThreshold:        NewThresholdStrategy(),
			domain.StrategyThresholdRecalc:  NewThresholdRecalcStrategy(),
			domain.StrategyPrepaidThreshold: NewPrepaidThresholdStrategy(),
		},
	}
}

// TarifyMessageRequest запрос на тарификацию сообщения
type TarifyMessageRequest struct {
	ClientID       uuid.UUID
	MessageID      uuid.UUID
	OperatorID     uuid.UUID
	SenderName     string
	SegmentCount   int
	IdempotencyKey string
}

// TarifyMessageResponse ответ на тарификацию сообщения
type TarifyMessageResponse struct {
	Approved         bool
	TotalAmount      string
	Currency         string
	Strategy         string
	TariffPlanID     string
	RejectionReason  string
	ThresholdCrossed bool
	RecalcAmount     string
}

// TarifyMessage тарифицирует сообщение при отправке
func (s *TarificationService) TarifyMessage(ctx context.Context, req *TarifyMessageRequest) (*TarifyMessageResponse, error) {
	// 1. Проверка идемпотентности
	existing, err := s.logRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("idempotency check failed: %w", err)
	}
	if existing != nil {
		// Resolve currency from the tariff plan's operator country
		existingPlan, planErr := s.planRepo.GetByID(ctx, existing.TariffPlanID)
		existingCurrency := ""
		if planErr == nil && existingPlan != nil {
			existingCurrency = existingPlan.Currency
		}
		return &TarifyMessageResponse{
			Approved:     true,
			TotalAmount:  existing.TotalAmount,
			Currency:     existingCurrency,
			Strategy:     string(existing.Strategy),
			TariffPlanID: existing.TariffPlanID.String(),
		}, nil
	}

	// 2. Определение категории имени отправителя
	category, err := s.determineSenderCategory(ctx, req.ClientID, req.OperatorID, req.SenderName)
	if err != nil {
		return nil, fmt.Errorf("sender category determination failed: %w", err)
	}

	// 3. Поиск активного тарифного плана
	plan, err := s.planRepo.GetActiveByOperatorAndCategory(ctx, req.OperatorID, category)
	if err != nil {
		return &TarifyMessageResponse{
			Approved:        false,
			RejectionReason: domain.ErrNoActiveTariffPlan.Error(),
		}, nil
	}

	// 4. Поиск активного тарифного периода
	now := time.Now()
	period, err := s.periodRepo.GetActiveByPlanID(ctx, plan.ID, now)
	if err != nil {
		return &TarifyMessageResponse{
			Approved:        false,
			RejectionReason: domain.ErrNoActivePeriod.Error(),
		}, nil
	}

	// 5. Получение порогов
	tiers, err := s.tierRepo.ListByPeriodID(ctx, period.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tiers: %w", err)
	}
	if len(tiers) == 0 {
		return &TarifyMessageResponse{
			Approved:        false,
			RejectionReason: "no tiers configured for active period",
		}, nil
	}

	// 6. Получение текущего счётчика
	counter, err := s.usageRepo.GetOrCreate(ctx, req.ClientID, plan.ID, period.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage counter: %w", err)
	}

	// 7. Вычисление стоимости по стратегии
	strategy, ok := s.strategies[plan.Strategy]
	if !ok {
		return nil, fmt.Errorf("unknown strategy: %s", plan.Strategy)
	}

	result, err := strategy.Calculate(ctx, CalculationParams{
		CurrentCount: counter.SegmentCount,
		SegmentCount: req.SegmentCount,
		Tiers:        tiers,
	})
	if err != nil {
		return nil, fmt.Errorf("strategy calculation failed: %w", err)
	}

	// 8. Списание через billing-service (Saga)
	chargeResult, err := s.saga.Charge(ctx,
		req.ClientID.String(),
		req.MessageID.String(),
		result.ChargeAmount,
		plan.Currency,
		fmt.Sprintf("SMS tarification: %s strategy, %d segments", plan.Strategy, req.SegmentCount),
		int32(req.SegmentCount),
	)
	if err != nil {
		return nil, fmt.Errorf("billing charge failed: %w", err)
	}
	if !chargeResult.Success {
		return &TarifyMessageResponse{
			Approved:        false,
			RejectionReason: domain.ErrInsufficientBalance.Error(),
		}, nil
	}

	// 9. Обновление счётчика
	_, err = s.usageRepo.IncrementAndGet(ctx, req.ClientID, plan.ID, period.ID, req.SegmentCount)
	if err != nil {
		log.Error().Err(err).Msg("failed to increment usage counter after charge")
	}

	// 10. Запись в лог тарификации
	tarLog := domain.NewTarificationLog(
		req.ClientID, req.MessageID, req.OperatorID, plan.ID, period.ID,
		category, plan.Strategy,
		req.SegmentCount, result.PricePerSegment, result.ChargeAmount,
		req.IdempotencyKey,
	)
	if result.RecalcAmount != "" {
		tarLog.RecalcAmount = &result.RecalcAmount
	}
	if err := s.logRepo.Create(ctx, tarLog); err != nil {
		log.Error().Err(err).Msg("failed to create tarification log")
	}

	// 11. Обработка пересчёта при переходе порога (стратегия threshold_recalc)
	if result.ThresholdCrossed && result.RecalcAmount != "" {
		go func() {
			recalcCtx := context.Background()
			recalcResult, recalcErr := s.saga.HandleRecalc(recalcCtx,
				req.ClientID.String(), result.RecalcAmount, plan.Currency)
			if recalcErr != nil {
				log.Error().Err(recalcErr).
					Str("client_id", req.ClientID.String()).
					Str("recalc_amount", result.RecalcAmount).
					Msg("recalculation saga failed")
				return
			}
			if recalcResult != nil && !recalcResult.Success {
				log.Warn().
					Str("client_id", req.ClientID.String()).
					Msg("recalculation deduction failed — debt recorded")
			}
			// Публикуем событие пересчёта
			if pubErr := s.eventPublisher.PublishRecalcEvent(recalcCtx,
				req.ClientID.String(), plan.ID.String(), period.ID.String(),
				"", result.PricePerSegment, counter.SegmentCount, result.RecalcAmount, "recalc",
			); pubErr != nil {
				log.Error().Err(pubErr).Msg("failed to publish recalc event")
			}
		}()
	}

	// 12. Публикация события тарификации
	if err := s.eventPublisher.PublishTarificationResult(ctx, tarLog); err != nil {
		log.Error().Err(err).Msg("failed to publish tarification result")
	}

	return &TarifyMessageResponse{
		Approved:         true,
		TotalAmount:      result.ChargeAmount,
		Currency:         plan.Currency,
		Strategy:         string(plan.Strategy),
		TariffPlanID:     plan.ID.String(),
		ThresholdCrossed: result.ThresholdCrossed,
		RecalcAmount:     result.RecalcAmount,
	}, nil
}

// GetUsageCounter получает счётчик использования
func (s *TarificationService) GetUsageCounter(ctx context.Context, clientID, tariffPlanID, tariffPeriodID uuid.UUID) (*domain.UsageCounter, error) {
	return s.usageRepo.GetOrCreate(ctx, clientID, tariffPlanID, tariffPeriodID)
}

// ListUsageCounters получает список счётчиков использования клиента
func (s *TarificationService) ListUsageCounters(ctx context.Context, clientID uuid.UUID, tariffPlanID *uuid.UUID, limit, offset int) ([]*domain.UsageCounter, int, error) {
	return s.usageRepo.GetByClient(ctx, clientID, tariffPlanID, limit, offset)
}

// determineSenderCategory определяет категорию имени отправителя
func (s *TarificationService) determineSenderCategory(ctx context.Context, clientID, operatorID uuid.UUID, senderName string) (domain.SenderCategory, error) {
	if senderName == "" {
		return domain.CategoryShared, nil
	}

	reg, err := s.senderRepo.GetActiveByClientOperatorName(ctx, clientID, operatorID, senderName)
	if err != nil || reg == nil {
		return domain.CategoryShared, nil
	}

	switch reg.Type {
	case domain.SenderTypePaid:
		return domain.CategoryPaidRegistered, nil
	case domain.SenderTypeFree:
		return domain.CategoryFreeRegistered, nil
	default:
		return domain.CategoryShared, nil
	}
}
