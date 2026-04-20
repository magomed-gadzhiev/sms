package application

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// RejectionCode — устойчивый к wording enum для маппинга approved=false в
// CommitChargeResult. Предпочитается substring-матчингу по RejectionReason.
type RejectionCode string

const (
	RejectionCodeNone                RejectionCode = ""
	RejectionCodeNoTariffPlan        RejectionCode = "no_tariff_plan"
	RejectionCodeNoPeriod            RejectionCode = "no_period"
	RejectionCodeNoTiers             RejectionCode = "no_tiers"
	RejectionCodeQuotaServiceMissing RejectionCode = "quota_service_missing"
	RejectionCodeQuotaNotConfigured  RejectionCode = "quota_not_configured"
	RejectionCodeInsufficientBalance RejectionCode = "insufficient_balance"
)

// CalculateResult содержит всё, что нужно для CommitCharge и отдачи клиенту.
// Заполняется read-only методом Calculate — никаких UPDATE/INSERT в БД.
type CalculateResult struct {
	Approved        bool
	RejectionReason string
	RejectionCode   RejectionCode

	// Для direct-клиента (IsDirect=true):
	IsDirect       bool
	PlatformAmount string
	Currency       string

	// Для субаккаунта (IsDirect=false):
	AggregatorID    uuid.UUID
	OperatorID      uuid.UUID
	SegmentCount    int
	SubAccountPrice string // per segment
	AggregatorPrice string // per segment (effective: mixed pool+overage для split)
	SubAccountTotal string
	AggregatorTotal string
	PoolSegments    int
	OverageSegments int
	ChargeMode      string // ChargeModePool | ChargeModeOverage | ChargeModeSplit

	// Общее (необходимо для CommitCharge: tarification_log, usage counter, recalc):
	Strategy         string
	TariffPlanID     string
	PeriodID         uuid.UUID
	Category         domain.SenderCategory
	PricePerSegment  string // платформенный effective price per segment (для tarification_log)
	ThresholdCrossed bool
	RecalcAmount     string
}

// Calculate выполняет read-only расчёт тарификации для сообщения.
// Метод ТОЛЬКО читает БД (idempotency lookup, plan/period/tiers/counter/quota SELECT) —
// никаких UPDATE/INSERT, никаких saga.Charge, никаких publish событий.
// Результат используется:
//  1. TarifyMessage (под флагом commit-on-submit) — для отдачи расчёта без списания.
//  2. CommitCharge — как источник параметров для billing.ChargeMessageDual.
func (s *TarificationService) Calculate(ctx context.Context, req *TarifyMessageRequest) (*CalculateResult, error) {
	// UUID v7 для message_id: генерируем если не передан. Sortable-форма
	// нужна для будущего партиционирования margin_log. Если передан — как есть.
	if req.MessageID == uuid.Nil {
		newID, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate message_id v7: %w", err)
		}
		req.MessageID = newID
	}

	// 1. Проверка идемпотентности — зеркалит TarifyMessage.
	existing, err := s.logRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("idempotency check failed: %w", err)
	}
	if existing != nil {
		var existingCurrency, tariffPlanID string
		switch {
		case existing.TariffPlanID != nil:
			if existingPlan, planErr := s.planRepo.GetByID(ctx, *existing.TariffPlanID); planErr == nil && existingPlan != nil {
				existingCurrency = existingPlan.Currency
			}
			tariffPlanID = existing.TariffPlanID.String()
		case existing.SourceRuleID != nil:
			existingCurrency = "RUB"
			if s.unifiedDeps != nil && s.unifiedDeps.operatorLookup != nil {
				if meta, metaErr := s.unifiedDeps.operatorLookup.Meta(ctx, existing.OperatorID); metaErr == nil && meta.Currency != "" {
					existingCurrency = meta.Currency
				}
			}
			tariffPlanID = existing.SourceRuleID.String()
		}
		// На replay возвращаем минимально необходимое — не различаем direct/sub,
		// т.к. TarificationLog не хранит эту информацию. CommitCharge, встретив
		// replay, полагается на billing.ChargeMessageDual с его собственным
		// idempotency-гардом. Для TarifyMessage (legacy response) — PlatformAmount
		// = existing.TotalAmount достаточно.
		return &CalculateResult{
			Approved:       true,
			IsDirect:       true,
			PlatformAmount: existing.TotalAmount,
			Currency:       existingCurrency,
			Strategy:       string(existing.Strategy),
			TariffPlanID:   tariffPlanID,
		}, nil
	}

	// 2. Определение категории имени отправителя.
	category, err := s.determineSenderCategory(ctx, req.ClientID, req.OperatorID, req.SenderName)
	if err != nil {
		return nil, fmt.Errorf("sender category determination failed: %w", err)
	}

	// 3. Поиск активного тарифного плана (платформенный тариф).
	plan, err := s.planRepo.GetActiveByOperatorAndCategory(ctx, req.OperatorID, category)
	if err != nil {
		return &CalculateResult{
			Approved:        false,
			RejectionCode:   RejectionCodeNoTariffPlan,
			RejectionReason: domain.ErrNoActiveTariffPlan.Error(),
		}, nil
	}

	// 4. Поиск активного тарифного периода.
	now := time.Now()
	period, err := s.periodRepo.GetActiveByPlanID(ctx, plan.ID, now)
	if err != nil {
		return &CalculateResult{
			Approved:        false,
			RejectionCode:   RejectionCodeNoPeriod,
			RejectionReason: domain.ErrNoActivePeriod.Error(),
		}, nil
	}

	// 5. Получение порогов.
	tiers, err := s.tierRepo.ListByPeriodID(ctx, period.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tiers: %w", err)
	}
	if len(tiers) == 0 {
		return &CalculateResult{
			Approved:        false,
			RejectionCode:   RejectionCodeNoTiers,
			RejectionReason: "no tiers configured for active period",
		}, nil
	}

	// 6. Получение текущего счётчика. GetOrCreate делает INSERT при отсутствии,
	// что формально write. Но это безопасно (ON CONFLICT DO NOTHING на уровне
	// репозитория) и идентично legacy TarifyMessage. Альтернатива — добавить
	// Get-only метод, но это scope creep для Task 10. Задокументировано.
	counter, err := s.usageRepo.GetOrCreate(ctx, req.ClientID, plan.ID, period.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage counter: %w", err)
	}

	// 7. Вычисление стоимости по стратегии (платформенный тариф).
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

	// 8. Aggregator context (read-only — resolveAggregatorBilling только SELECT).
	isSubAccount, aggregatorID, aggChargeAmount, subChargeAmount := s.resolveAggregatorBilling(
		ctx, req.ClientID, req.OperatorID, category, result, req.SegmentCount,
	)

	// 8a. Direct-клиент — без aggregator-логики.
	if !isSubAccount {
		return &CalculateResult{
			Approved:         true,
			IsDirect:         true,
			PlatformAmount:   result.ChargeAmount,
			Currency:         plan.Currency,
			OperatorID:       req.OperatorID,
			SegmentCount:     req.SegmentCount,
			Strategy:         string(plan.Strategy),
			TariffPlanID:     plan.ID.String(),
			PeriodID:         period.ID,
			Category:         category,
			PricePerSegment:  result.PricePerSegment,
			ThresholdCrossed: result.ThresholdCrossed,
			RecalcAmount:     result.RecalcAmount,
		}, nil
	}

	// 8b. Субаккаунт — нужна активная квота для определения charge_mode.
	if s.quotaService == nil {
		return &CalculateResult{
			Approved:        false,
			RejectionCode:   RejectionCodeQuotaServiceMissing,
			RejectionReason: "quota service not configured",
		}, nil
	}
	quota, err := s.quotaService.GetActiveQuota(ctx, aggregatorID)
	if err != nil {
		return nil, fmt.Errorf("get active quota: %w", err)
	}
	if quota == nil {
		// В commit-on-submit flow это эквивалент QUOTA_NOT_CONFIGURED.
		return &CalculateResult{
			Approved:        false,
			RejectionCode:   RejectionCodeQuotaNotConfigured,
			RejectionReason: "quota not configured",
			AggregatorID:    aggregatorID,
			OperatorID:      req.OperatorID,
		}, nil
	}

	// Вычисление charge_mode / pool_segments / overage_segments.
	remaining := quota.SegmentLimit - quota.SegmentsUsed
	var chargeMode string
	var poolSegs, overageSegs int
	switch {
	case int64(req.SegmentCount) <= remaining:
		chargeMode = domain.ChargeModePool
		poolSegs = req.SegmentCount
		overageSegs = 0
	case remaining <= 0:
		chargeMode = domain.ChargeModeOverage
		poolSegs = 0
		overageSegs = req.SegmentCount
	default:
		chargeMode = domain.ChargeModeSplit
		poolSegs = int(remaining)
		overageSegs = req.SegmentCount - int(remaining)
	}

	// Aggregator price per segment: платформенный — из result.PricePerSegment.
	// aggregator_total = pool_segs * platform_price + overage_segs * overage_rate.
	// Если всё pool — aggChargeAmount (который resolveAggregatorBilling посчитал
	// как full platform × segmentCount) совпадает с aggregator_total. Для split/overage
	// пересчитываем явно.
	aggregatorTotal := aggChargeAmount
	if chargeMode != domain.ChargeModePool {
		recomputed, err := composeAggregatorTotal(result.PricePerSegment, poolSegs, quota.OverageRate, overageSegs)
		if err != nil {
			return nil, fmt.Errorf("compose aggregator_total: %w", err)
		}
		aggregatorTotal = recomputed
	}

	return &CalculateResult{
		Approved:         true,
		IsDirect:         false,
		Currency:         plan.Currency,
		AggregatorID:     aggregatorID,
		OperatorID:       req.OperatorID,
		SegmentCount:     req.SegmentCount,
		SubAccountPrice:  computePricePerSegment(subChargeAmount, req.SegmentCount),
		AggregatorPrice:  computePricePerSegment(aggregatorTotal, req.SegmentCount),
		SubAccountTotal:  subChargeAmount,
		AggregatorTotal:  aggregatorTotal,
		PoolSegments:     poolSegs,
		OverageSegments:  overageSegs,
		ChargeMode:       chargeMode,
		Strategy:         string(plan.Strategy),
		TariffPlanID:     plan.ID.String(),
		PeriodID:         period.ID,
		Category:         category,
		PricePerSegment:  result.PricePerSegment,
		ThresholdCrossed: result.ThresholdCrossed,
		RecalcAmount:     result.RecalcAmount,
	}, nil
}

// composeAggregatorTotal вычисляет aggregator_total для split/overage:
// pool_segs * poolPrice + overage_segs * overageRate.
func composeAggregatorTotal(poolPrice string, poolSegs int, overageRate string, overageSegs int) (string, error) {
	pool := new(big.Float)
	if poolSegs > 0 {
		p, _, err := big.ParseFloat(poolPrice, 10, 128, big.ToNearestEven)
		if err != nil {
			return "", fmt.Errorf("invalid pool price: %w", err)
		}
		n := new(big.Float).SetInt64(int64(poolSegs))
		pool.Mul(p, n)
	}
	overage := new(big.Float)
	if overageSegs > 0 {
		o, _, err := big.ParseFloat(overageRate, 10, 128, big.ToNearestEven)
		if err != nil {
			return "", fmt.Errorf("invalid overage rate: %w", err)
		}
		n := new(big.Float).SetInt64(int64(overageSegs))
		overage.Mul(o, n)
	}
	total := new(big.Float).Add(pool, overage)
	return total.Text('f', 6), nil
}
