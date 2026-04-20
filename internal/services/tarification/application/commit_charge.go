package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// CommitChargeRequest — запрос на фактическое списание средств после успешной
// отправки сообщения. Зеркалит proto tarification.v1.CommitChargeRequest.
type CommitChargeRequest struct {
	MessageID      uuid.UUID
	ClientID       uuid.UUID
	OperatorID     uuid.UUID
	SenderName     string
	SegmentCount   int
	IdempotencyKey string
}

// CommitChargeResult — результат CommitCharge. Набор boolean-флагов соответствует
// enum CommitChargeError в proto: транспортный слой маппит флаги на enum.
type CommitChargeResult struct {
	Committed        bool
	AlreadyCommitted bool
	QuotaMissing     bool
	SubInsufficient  bool
	AggInsufficient  bool
	NoTariff         bool

	SubAccountTxID string
	AggregatorTxID string
	MarginLogID    string
}

// CommitCharge выполняет фактическое списание средств за сообщение.
// Вызывается после подтверждения доставки в commit-on-submit flow.
//
// Алгоритм:
//  1. Idempotency-guard: если в tarification_log уже есть запись с тем же
//     ключом — возвращаем AlreadyCommitted=true без повторного списания.
//  2. Calculate() повторяет read-only расчёт (current tiers/quota).
//  3. Direct-клиент: saga.Charge() как в legacy TarifyMessage.
//  4. Субаккаунт: saga.ChargeDualAtomic() → billing.ChargeMessageDual (атомарно).
//  5. После успешного charge — в обеих ветках: tarification_log.Create,
//     usage counter increment, publish event, recalc goroutine (если стратегия
//     threshold_recalc вернула ThresholdCrossed).
//
// Drift: между TarifyMessage и CommitCharge тариф мог измениться. Calculate
// вернёт актуальные цены — это by design.
func (s *TarificationService) CommitCharge(ctx context.Context, req *CommitChargeRequest) (*CommitChargeResult, error) {
	// 1. Idempotency short-circuit. Если запись в tarification_log уже есть,
	// значит CommitCharge (или legacy TarifyMessage) уже отработал —
	// возвращаем AlreadyCommitted без обращения к billing.
	// Это позволяет не расширять replay-ветку в Calculate (которая не хранит
	// AggregatorID/SegmentCount для ChargeMessageDual).
	if req.IdempotencyKey != "" {
		existing, err := s.logRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("idempotency check failed: %w", err)
		}
		if existing != nil {
			return &CommitChargeResult{AlreadyCommitted: true}, nil
		}
	}

	// 2. Read-only расчёт.
	calc, err := s.Calculate(ctx, &TarifyMessageRequest{
		ClientID:       req.ClientID,
		MessageID:      req.MessageID,
		OperatorID:     req.OperatorID,
		SenderName:     req.SenderName,
		SegmentCount:   req.SegmentCount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, fmt.Errorf("calculate failed: %w", err)
	}

	if !calc.Approved {
		result := &CommitChargeResult{}
		switch calc.RejectionCode {
		case RejectionCodeQuotaNotConfigured, RejectionCodeQuotaServiceMissing:
			result.QuotaMissing = true
		case RejectionCodeInsufficientBalance:
			result.SubInsufficient = true
		case RejectionCodeNoTariffPlan, RejectionCodeNoPeriod, RejectionCodeNoTiers:
			result.NoTariff = true
		default:
			// Unknown / empty — консервативно маркируем как NoTariff
			// (поведение, идентичное legacy fallback).
			result.NoTariff = true
		}
		return result, nil
	}

	// 3. Direct-клиент — одиночное списание через legacy-сагу.
	// billing.ChargeMessage идемпотентен по message_id (см. billing_service.go:424-432:
	// transactionRepo.GetByMessageID проверяет существование до начала транзакции).
	if calc.IsDirect {
		chargeResult, err := s.saga.Charge(ctx,
			req.ClientID.String(), req.MessageID.String(),
			calc.PlatformAmount, calc.Currency,
			fmt.Sprintf("SMS commit: %s, %d segments", calc.Strategy, req.SegmentCount),
			int32(req.SegmentCount),
		)
		if err != nil {
			return nil, fmt.Errorf("billing charge failed: %w", err)
		}
		if !chargeResult.Success {
			return &CommitChargeResult{SubInsufficient: true}, nil
		}

		// Post-charge side-effects (direct branch): log + publish + counter + recalc.
		s.commitChargePostCharge(ctx, req, calc, calc.PlatformAmount)

		return &CommitChargeResult{
			Committed:      true,
			SubAccountTxID: chargeResult.TransactionID,
		}, nil
	}

	// 4. Субаккаунт — атомарная dual-транзакция через billing.ChargeMessageDual.
	dualReq := &billingv1.ChargeMessageDualRequest{
		MessageId:       req.MessageID.String(),
		SubAccountId:    req.ClientID.String(),
		AggregatorId:    calc.AggregatorID.String(),
		SubAccountPrice: calc.SubAccountPrice,
		AggregatorPrice: calc.AggregatorPrice,
		SubAccountTotal: calc.SubAccountTotal,
		AggregatorTotal: calc.AggregatorTotal,
		Currency:        calc.Currency,
		OperatorId:      calc.OperatorID.String(),
		SegmentCount:    int32(calc.SegmentCount),
		PoolSegments:    int32(calc.PoolSegments),
		OverageSegments: int32(calc.OverageSegments),
		ChargeMode:      calc.ChargeMode,
	}

	dualResp, err := s.saga.ChargeDualAtomic(ctx, dualReq)
	if err != nil {
		return nil, fmt.Errorf("dual charge failed: %w", err)
	}

	result := &CommitChargeResult{
		SubAccountTxID: dualResp.SubAccountTxID,
		AggregatorTxID: dualResp.AggregatorTxID,
		MarginLogID:    dualResp.MarginLogID,
	}

	switch dualResp.Error {
	case billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_UNSPECIFIED:
		result.Committed = dualResp.Committed
	case billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_ALREADY_COMMITTED:
		result.AlreadyCommitted = true
	case billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_QUOTA_NOT_CONFIGURED:
		result.QuotaMissing = true
	case billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_INSUFFICIENT_BALANCE_SUBACCOUNT:
		result.SubInsufficient = true
	case billingv1.ChargeMessageDualError_CHARGE_DUAL_ERROR_INSUFFICIENT_BALANCE_AGGREGATOR:
		result.AggInsufficient = true
	default:
		return nil, fmt.Errorf("unknown ChargeMessageDualError: %v", dualResp.Error)
	}

	// Post-charge side-effects (subaccount branch) — только при фактическом
	// успешном списании. AlreadyCommitted сюда не попадает — это replay
	// billing-стороны, log уже записан предыдущим вызовом.
	if result.Committed {
		s.commitChargePostCharge(ctx, req, calc, calc.SubAccountTotal)
	}

	return result, nil
}

// commitChargePostCharge выполняет побочные эффекты, которые в legacy
// TarifyMessage шли после успешного billing charge: tarification_log.Create,
// usage counter increment, publish event, recalc goroutine (threshold_recalc).
// Ошибки всех операций логируются, но не возвращаются — списание средств уже
// состоялось, и откатывать его некорректно.
//
// chargeAmount — фактически списанная сумма (PlatformAmount для direct,
// SubAccountTotal для субаккаунта).
func (s *TarificationService) commitChargePostCharge(
	ctx context.Context,
	req *CommitChargeRequest,
	calc *CalculateResult,
	chargeAmount string,
) {
	// Парсим plan_id и period_id. Если Calculate вернул approved=true, оба
	// обязаны быть заполнены. На случай баги — защищаемся логом.
	planID, err := uuid.Parse(calc.TariffPlanID)
	if err != nil {
		log.Error().Err(err).
			Str("tariff_plan_id", calc.TariffPlanID).
			Msg("commit-charge: failed to parse tariff_plan_id; skipping post-charge side-effects")
		return
	}
	periodID := calc.PeriodID
	if periodID == uuid.Nil {
		log.Error().
			Str("message_id", req.MessageID.String()).
			Msg("commit-charge: period_id is nil in approved calc; skipping post-charge side-effects")
		return
	}

	// Для логирования price_per_segment: для субаккаунта — фактическая цена
	// списания (SubAccountPrice); для direct — платформенный PricePerSegment
	// из strategy.Calculate.
	pricePerSegment := calc.PricePerSegment
	if !calc.IsDirect && calc.SubAccountPrice != "" {
		pricePerSegment = calc.SubAccountPrice
	}

	tarLog := domain.NewTarificationLog(
		req.ClientID, req.MessageID, req.OperatorID, planID, periodID,
		calc.Category, domain.TarificationStrategy(calc.Strategy),
		req.SegmentCount, pricePerSegment, chargeAmount,
		req.IdempotencyKey,
	)
	if calc.RecalcAmount != "" {
		recalc := calc.RecalcAmount
		tarLog.RecalcAmount = &recalc
	}
	if err := s.logRepo.Create(ctx, tarLog); err != nil {
		log.Error().Err(err).Msg("commit-charge: failed to create tarification log")
	}

	// Увеличиваем usage counter — критично для counter-based стратегий
	// (threshold/tiered/volume), иначе следующее сообщение посчитается от
	// устаревшего значения.
	if _, err := s.usageRepo.IncrementAndGet(ctx, req.ClientID, planID, periodID, req.SegmentCount); err != nil {
		log.Error().Err(err).Msg("commit-charge: failed to increment usage counter after charge")
	}

	// Публикуем событие тарификации.
	if s.eventPublisher != nil {
		if err := s.eventPublisher.PublishTarificationResult(ctx, tarLog); err != nil {
			log.Error().Err(err).Msg("commit-charge: failed to publish tarification result")
		}
	}

	// Recalc goroutine — только для threshold_recalc стратегии при пересечении
	// порога. Поведение зеркалит legacy TarifyMessage (строки 427-451).
	if calc.ThresholdCrossed && calc.RecalcAmount != "" {
		clientID := req.ClientID
		currency := calc.Currency
		recalcAmount := calc.RecalcAmount
		pricePerSeg := calc.PricePerSegment
		// segment count на момент списания — для publish event это «affected segments».
		// В legacy это counter.SegmentCount (до increment). Для commit-on-submit
		// точный счётчик недоступен без отдельного SELECT; используем req.SegmentCount
		// как приближение (отражает текущее сообщение). Это semantic drift от legacy,
		// но recalc event используется для аудита — не для расчётов.
		affectedSegments := req.SegmentCount
		planIDStr := calc.TariffPlanID
		periodIDStr := periodID.String()
		publisher := s.eventPublisher

		go func() {
			recalcCtx := context.Background()
			recalcResult, recalcErr := s.saga.HandleRecalc(recalcCtx,
				clientID.String(), recalcAmount, currency)
			if recalcErr != nil {
				log.Error().Err(recalcErr).
					Str("client_id", clientID.String()).
					Str("recalc_amount", recalcAmount).
					Msg("commit-charge: recalculation saga failed")
				return
			}
			if recalcResult != nil && !recalcResult.Success {
				log.Warn().
					Str("client_id", clientID.String()).
					Msg("commit-charge: recalculation deduction failed — debt recorded")
			}
			if publisher != nil {
				if pubErr := publisher.PublishRecalcEvent(recalcCtx,
					clientID.String(), planIDStr, periodIDStr,
					"", pricePerSeg, affectedSegments, recalcAmount, "recalc",
				); pubErr != nil {
					log.Error().Err(pubErr).Msg("commit-charge: failed to publish recalc event")
				}
			}
		}()
	}
}

// Guard: domain errors referenced implicitly by RejectionReason string matching.
// Explicit reference keeps the import alive and fails compilation if the
// error variables are renamed without updating this file.
var _ = domain.ErrNoActiveTariffPlan
