package application

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// TarificationService основной сервис тарификации
type TarificationService struct {
	senderRepo         domain.SenderRegistrationRepository
	planRepo           domain.TariffPlanRepository
	periodRepo         domain.TariffPeriodRepository
	tierRepo           domain.TariffTierRepository
	usageRepo          domain.UsageCounterRepository
	logRepo            domain.TarificationLogRepository
	prepaidRepo        domain.PrepaidFeeRepository
	saga               *SagaOrchestrator
	eventPublisher     domain.EventPublisher
	strategies         map[domain.TarificationStrategy]BillingStrategy
	hierarchicalLookup domain.HierarchicalPeriodLookup // nil until migration completes

	// Агрегаторская тарификация
	clientInfoRepo   domain.ClientInfoRepository
	aggTariffRepo    domain.AggregatorTariffRepository
	aggMarginLogRepo domain.AggregatorMarginLogRepository
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

// SetHierarchicalLookup wires the new hierarchical period lookup.
// Call this after migration 000082 completes and the new table is populated.
func (s *TarificationService) SetHierarchicalLookup(lookup domain.HierarchicalPeriodLookup) {
	s.hierarchicalLookup = lookup
}

// SetAggregatorRepos подключает репозитории агрегаторской тарификации
func (s *TarificationService) SetAggregatorRepos(
	clientInfoRepo domain.ClientInfoRepository,
	aggTariffRepo domain.AggregatorTariffRepository,
	aggMarginLogRepo domain.AggregatorMarginLogRepository,
) {
	s.clientInfoRepo = clientInfoRepo
	s.aggTariffRepo = aggTariffRepo
	s.aggMarginLogRepo = aggMarginLogRepo
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

	// 3. Поиск активного тарифного плана (платформенный тариф)
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

	// 7. Вычисление стоимости по стратегии (платформенный тариф)
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

	// 8. Проверяем, является ли клиент субаккаунтом, и применяем тариф агрегатора
	isSubAccount, aggregatorID, aggChargeAmount, subChargeAmount := s.resolveAggregatorBilling(
		ctx, req.ClientID, req.OperatorID, category, result, req.SegmentCount,
	)

	var chargeResult *ChargeResult
	if isSubAccount && aggregatorID != uuid.Nil {
		// Двойное списание: субаккаунт по тарифу агрегатора + агрегатор по платформенному тарифу
		dualResult, dualErr := s.saga.ChargeDual(ctx,
			req.ClientID.String(),
			aggregatorID.String(),
			req.MessageID.String(),
			subChargeAmount,
			aggChargeAmount,
			plan.Currency,
		)
		if dualErr != nil {
			return nil, fmt.Errorf("dual billing charge failed: %w", dualErr)
		}
		if !dualResult.Success {
			return &TarifyMessageResponse{
				Approved:        false,
				RejectionReason: domain.ErrInsufficientBalance.Error(),
			}, nil
		}
		chargeResult = dualResult.SubAccountCharge

		// Записываем маржу агрегатора
		s.logAggregatorMargin(ctx, aggregatorID, req.ClientID, req.MessageID, req.OperatorID,
			req.SegmentCount, result.PricePerSegment, subChargeAmount, aggChargeAmount,
			req.IdempotencyKey)
	} else {
		// Обычное списание (не субаккаунт или нет тарифа агрегатора)
		chargeResult, err = s.saga.Charge(ctx,
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
	}
	_ = chargeResult

	// 9. Обновление счётчика
	_, err = s.usageRepo.IncrementAndGet(ctx, req.ClientID, plan.ID, period.ID, req.SegmentCount)
	if err != nil {
		log.Error().Err(err).Msg("failed to increment usage counter after charge")
	}

	// 10. Запись в лог тарификации
	// Для субаккаунта фиксируем реально списанную сумму (тариф агрегатора)
	logPricePerSegment := result.PricePerSegment
	logChargeAmount := result.ChargeAmount
	if isSubAccount && subChargeAmount != "" {
		logPricePerSegment = computePricePerSegment(subChargeAmount, req.SegmentCount)
		logChargeAmount = subChargeAmount
	}

	tarLog := domain.NewTarificationLog(
		req.ClientID, req.MessageID, req.OperatorID, plan.ID, period.ID,
		category, plan.Strategy,
		req.SegmentCount, logPricePerSegment, logChargeAmount,
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
		TotalAmount:      logChargeAmount,
		Currency:         plan.Currency,
		Strategy:         string(plan.Strategy),
		TariffPlanID:     plan.ID.String(),
		ThresholdCrossed: result.ThresholdCrossed,
		RecalcAmount:     result.RecalcAmount,
	}, nil
}

// resolveAggregatorBilling определяет, нужна ли агрегаторская тарификация.
// Возвращает: isSubAccount, aggregatorID, aggAmount (платформенный), subAmount (тариф агрегатора).
func (s *TarificationService) resolveAggregatorBilling(
	ctx context.Context,
	clientID, operatorID uuid.UUID,
	category domain.SenderCategory,
	result *CalculationResult,
	segmentCount int,
) (isSubAccount bool, aggregatorID uuid.UUID, aggAmount, subAmount string) {
	if s.clientInfoRepo == nil || s.aggTariffRepo == nil {
		return false, uuid.Nil, "", ""
	}

	clientInfo, err := s.clientInfoRepo.GetAccountInfo(ctx, clientID)
	if err != nil || clientInfo == nil {
		return false, uuid.Nil, "", ""
	}
	if clientInfo.AccountType != "sub_account" || clientInfo.ParentClientID == nil {
		return false, uuid.Nil, "", ""
	}

	parentID := *clientInfo.ParentClientID

	aggTariff, err := s.aggTariffRepo.GetForSubAccount(ctx, parentID, clientID, operatorID, category)
	if err != nil || aggTariff == nil {
		return false, uuid.Nil, "", ""
	}

	subTotal, err := multiplyPrice(aggTariff.PricePerSegment, segmentCount)
	if err != nil {
		log.Error().Err(err).Msg("failed to compute sub-account total from aggregator tariff")
		return false, uuid.Nil, "", ""
	}

	return true, parentID, result.ChargeAmount, subTotal
}

// logAggregatorMargin записывает маржу агрегатора в aggregator_margin_log
func (s *TarificationService) logAggregatorMargin(
	ctx context.Context,
	aggregatorID, subAccountID, messageID, operatorID uuid.UUID,
	segmentCount int,
	platformPricePerSeg, subTotal, aggTotal string,
	idempotencyKey string,
) {
	if s.aggMarginLogRepo == nil {
		return
	}

	margin, err := subtractAmounts(subTotal, aggTotal)
	if err != nil {
		log.Error().Err(err).Msg("failed to compute aggregator margin")
		return
	}

	entry := &domain.AggregatorMarginLog{
		ID:              uuid.New(),
		AggregatorID:    aggregatorID,
		SubAccountID:    subAccountID,
		MessageID:       messageID,
		OperatorID:      operatorID,
		SegmentCount:    segmentCount,
		SubAccountPrice: computePricePerSegment(subTotal, segmentCount),
		AggregatorPrice: platformPricePerSeg,
		SubAccountTotal: subTotal,
		AggregatorTotal: aggTotal,
		Margin:          margin,
		IdempotencyKey:  idempotencyKey + "_margin",
		CreatedAt:       time.Now(),
	}

	if err := s.aggMarginLogRepo.Create(ctx, entry); err != nil {
		log.Error().Err(err).
			Str("aggregator_id", aggregatorID.String()).
			Str("sub_account_id", subAccountID.String()).
			Msg("failed to write aggregator margin log")
	}
}

// multiplyPrice умножает цену на количество сегментов
func multiplyPrice(pricePerSeg string, segments int) (string, error) {
	price, _, err := big.ParseFloat(pricePerSeg, 10, 128, big.ToNearestEven)
	if err != nil {
		return "", fmt.Errorf("invalid price: %w", err)
	}
	n := new(big.Float).SetInt64(int64(segments))
	total := new(big.Float).Mul(price, n)
	return total.Text('f', 6), nil
}

// subtractAmounts вычисляет a - b
func subtractAmounts(a, b string) (string, error) {
	af, _, err := big.ParseFloat(a, 10, 128, big.ToNearestEven)
	if err != nil {
		return "", fmt.Errorf("invalid amount a: %w", err)
	}
	bf, _, err := big.ParseFloat(b, 10, 128, big.ToNearestEven)
	if err != nil {
		return "", fmt.Errorf("invalid amount b: %w", err)
	}
	result := new(big.Float).Sub(af, bf)
	return result.Text('f', 6), nil
}

// computePricePerSegment вычисляет цену за сегмент из общей суммы
func computePricePerSegment(total string, segments int) string {
	if segments == 0 {
		return total
	}
	t, _, err := big.ParseFloat(total, 10, 128, big.ToNearestEven)
	if err != nil {
		return total
	}
	n := new(big.Float).SetInt64(int64(segments))
	price := new(big.Float).Quo(t, n)
	return price.Text('f', 6)
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
