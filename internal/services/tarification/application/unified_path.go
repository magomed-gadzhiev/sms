// unified_path.go
package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// chargeRunner is the minimal interface for billing charge used by tarifyUnified.
// *SagaOrchestrator satisfies this interface. Extracted to allow test stubs.
type chargeRunner interface {
	Charge(ctx context.Context, clientID, messageID, amount, currency, description string, segments int32) (*ChargeResult, error)
}

// unifiedDeps bundles everything tarifyUnified needs. Populated by
// TarificationService.SetUnifiedDependencies (Task 9).
type unifiedDeps struct {
	resolver      *PriceResolver
	calc          *CostCalculator
	ruleRepo      domain.PriceRuleRepository // margin-path direct lookup
	subUsageRepo  domain.SubaccountUsageCounterRepository
	marginLogRepo domain.AggregatorMarginLogRepository
	saga          chargeRunner
	logRepo       domain.TarificationLogRepository
	senderRepo    domain.SenderRegistrationRepository
	operatorLookup OperatorCodeLookup
}

// tarifyUnified is the Phase 3 unified hot path: resolve → calculate →
// UPSERT usage counter → charge → margin second-resolve → log. Returns
// (resp, fallbackReason, err):
//   - err != nil: fatal (typically post-charge failures); surface to caller.
//   - fallbackReason != "": non-fatal; caller should run legacy branch.
//   - resp != nil & err == nil & fallbackReason == "": success.
//
// See docs/superpowers/plans/2026-04-19-phase3-unified-pricing-hot-path.md.
func tarifyUnified(ctx context.Context, req *TarifyMessageRequest, d *unifiedDeps) (*TarifyMessageResponse, string, error) {
	// 1. Idempotency (also checked before calling, but keep for safety).
	if existing, err := d.logRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey); err != nil {
		return nil, "", fmt.Errorf("idempotency check: %w", err)
	} else if existing != nil {
		// Unified-лог имеет nil TariffPlanID и заполненный SourceRuleID;
		// legacy-лог — наоборот. Возвращаем идентификатор, которым строка
		// была затарифицирована.
		tariffPlanID := ""
		switch {
		case existing.TariffPlanID != nil:
			tariffPlanID = existing.TariffPlanID.String()
		case existing.SourceRuleID != nil:
			tariffPlanID = existing.SourceRuleID.String()
		}
		return &TarifyMessageResponse{
			Approved:     true,
			TotalAmount:  existing.TotalAmount,
			Currency:     "RUB",
			Strategy:     string(existing.Strategy),
			TariffPlanID: tariffPlanID,
		}, "", nil
	}

	// 2. Category (wildcard if no sender name).
	category, err := resolveCategory(ctx, d.senderRepo, req.ClientID, req.OperatorID, req.SenderName)
	if err != nil {
		return nil, "", fmt.Errorf("resolve category: %w", err)
	}

	// 3. operator_id → operators.code (Task 0).
	operatorCode, err := d.operatorLookup.Code(ctx, req.OperatorID)
	if err != nil {
		unifiedFallbackTotal.WithLabelValues("operator_lookup_error").Inc()
		log.Warn().Err(err).Str("operator_id", req.OperatorID.String()).
			Msg("unified: operator code lookup failed — fallback")
		return nil, "operator_lookup_error", nil
	}

	// 4. Subaccount-path resolve.
	in := domain.ResolveInput{
		SubaccountID:   req.ClientID,
		Country:        "",
		Operator:       operatorCode,
		SenderCategory: string(category),
		TrafficType:    "",
		Now:            time.Now().UTC(),
	}
	startResolve := time.Now()
	rr, err := d.resolver.Resolve(WithResolverBranch(ctx, "subaccount"), in)
	unifiedResolveLatency.WithLabelValues("subaccount").Observe(time.Since(startResolve).Seconds())
	if err != nil {
		if isNotFound(err) {
			unifiedPriceNotFoundTotal.Inc()
			unifiedFallbackTotal.WithLabelValues("not_found").Inc()
			log.Warn().Err(err).Str("subaccount_id", req.ClientID.String()).
				Msg("unified resolve: no applicable rule — fallback to legacy")
			return nil, "not_found", nil
		}
		unifiedFallbackTotal.WithLabelValues("resolve_error").Inc()
		log.Warn().Err(err).Msg("unified resolve error — fallback to legacy")
		return nil, "resolve_error", nil
	}

	// 5. Usage-before for the calculator.
	periodKey := computePeriodKeyFromRule(rr)
	counter, err := d.subUsageRepo.Get(ctx, req.ClientID, periodKey)
	if err != nil {
		unifiedFallbackTotal.WithLabelValues("counter_error").Inc()
		log.Warn().Err(err).Msg("unified counter get error — fallback")
		return nil, "counter_error", nil
	}
	var usageBefore int64
	if counter != nil {
		usageBefore = counter.SegmentsUsed
	}

	// 6. Cost calculation.
	cost, err := d.calc.Calculate(rr, usageBefore, int64(req.SegmentCount))
	if err != nil {
		unifiedFallbackTotal.WithLabelValues("calc_error").Inc()
		log.Warn().Err(err).Str("rule_id", rr.SourceRuleID.String()).
			Msg("unified calc error — fallback")
		return nil, "calc_error", nil
	}
	costStr := strconv.FormatFloat(cost, 'f', 6, 64)

	// 7. Charge — after this point fallback is unsafe.
	// NOTE: double-charge on retry is prevented by billing-service idempotency
	// on message_id (see billing_service.go:425, GetByMessageID short-circuit).
	// If step 10 (tarification_log Create) later fails, a client retry with
	// the same idempotency key will miss step 1's short-circuit, re-enter
	// tarifyUnified, and billing will return the existing transaction
	// instead of charging twice.
	currency := "RUB" // unified model does not denormalize currency per rule; platform default. Non-RUB accounts will receive billing currency-mismatch errors → fallback to legacy. Resolve via resolved_rule in a follow-up.
	chargeResult, err := d.saga.Charge(ctx,
		req.ClientID.String(), req.MessageID.String(),
		costStr, currency,
		fmt.Sprintf("SMS tarification (unified): %d segments", req.SegmentCount),
		int32(req.SegmentCount),
	)
	if err != nil {
		return nil, "", fmt.Errorf("unified charge: %w", err)
	}
	if !chargeResult.Success {
		unifiedTarifyTotal.WithLabelValues("rejected").Inc()
		return &TarifyMessageResponse{
			Approved:        false,
			RejectionReason: domain.ErrInsufficientBalance.Error(),
		}, "", nil
	}

	// 8. Increment usage counter (post-charge; mismatch is acceptable — ledger wins).
	if _, err := d.subUsageRepo.Increment(ctx, req.ClientID, periodKey, int64(req.SegmentCount), costStr); err != nil {
		log.Error().Err(err).Msg("unified: usage counter increment failed after charge")
	}

	// 9. Margin second-resolve — bypasses resolved_rules cache by going
	// straight to ruleRepo with ExcludeSubaccount=true.
	marginIn := in
	marginIn.ExcludeSubaccount = true
	marginIn.AggregatorID = rr.AggregatorID
	startMargin := time.Now()
	aggRule, marginErr := d.ruleRepo.FindApplicable(ctx, marginIn)
	unifiedResolveLatency.WithLabelValues("margin").Observe(time.Since(startMargin).Seconds())
	if marginErr == nil && aggRule != nil {
		aggTempRR := &domain.ResolvedRule{
			PriceModel:   aggRule.PriceModel,
			PriceValue:   aggRule.PriceValue,
			TiersJSON:    aggRule.TiersJSON,
			SourceRuleID: aggRule.ID,
			RulesVersion: rr.RulesVersion,
		}
		aggCost, aggErr := d.calc.Calculate(aggTempRR, usageBefore, int64(req.SegmentCount))
		if aggErr == nil {
			marginFloat := cost - aggCost
			marginStr := strconv.FormatFloat(marginFloat, 'f', 6, 64)
			if d.marginLogRepo != nil {
				entry := &domain.AggregatorMarginLog{
					ID:              uuid.New(),
					AggregatorID:    rr.AggregatorID,
					SubAccountID:    req.ClientID,
					MessageID:       req.MessageID,
					OperatorID:      req.OperatorID,
					SegmentCount:    req.SegmentCount,
					SubAccountTotal: costStr,
					AggregatorTotal: strconv.FormatFloat(aggCost, 'f', 6, 64),
					Margin:          marginStr,
					IdempotencyKey:  req.IdempotencyKey + "_margin",
					CreatedAt:       time.Now(),
				}
				if err := d.marginLogRepo.Create(ctx, entry); err != nil {
					log.Error().Err(err).Msg("unified: margin log create failed")
				}
			}
			if marginFloat > 0 {
				unifiedMarginPerHourRub.Add(marginFloat)
			}
		}
	}

	// 10. Tarification log (for future idempotency).
	tarLog := domain.NewUnifiedTarificationLog(
		req.ClientID, req.MessageID, req.OperatorID, rr.SourceRuleID,
		category,
		req.SegmentCount,
		strconv.FormatFloat(cost/float64(req.SegmentCount), 'f', 6, 64),
		costStr,
		req.IdempotencyKey,
	)
	if err := d.logRepo.Create(ctx, tarLog); err != nil {
		log.Error().Err(err).Msg("unified: tarification log create failed")
	}

	unifiedTarifyTotal.WithLabelValues("approved").Inc()
	return &TarifyMessageResponse{
		Approved:     true,
		TotalAmount:  costStr,
		Currency:     currency,
		Strategy:     string(domain.StrategyUnified),
		TariffPlanID: rr.SourceRuleID.String(),
	}, "", nil
}

// resolveCategory mirrors TarificationService.determineSenderCategory
// without depending on the service receiver (testable in isolation).
func resolveCategory(ctx context.Context, repo domain.SenderRegistrationRepository, clientID, operatorID uuid.UUID, senderName string) (domain.SenderCategory, error) {
	if senderName == "" {
		return domain.CategoryShared, nil
	}
	reg, err := repo.GetActiveByClientOperatorName(ctx, clientID, operatorID, senderName)
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

// computePeriodKeyFromRule derives the usage-counter period key. Phase 3
// defaults to calendar_month (no period inspection of tiers_json yet);
// revisit when periodic tiered rules land.
func computePeriodKeyFromRule(_ *domain.ResolvedRule) string {
	return domain.ComputePeriodKey(domain.PeriodCalendarMonth, time.Now())
}

func isNotFound(err error) bool {
	return errors.Is(err, domain.ErrNoApplicableRule)
}
