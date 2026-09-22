package application

import (
	"context"
	"fmt"
	"math/big"

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

	// Aggregator quota
	quotaService *QuotaService

	// Phase 3 unified pricing — optional; nil until SetUnifiedDependencies.
	unifiedEnabled bool
	rollout        *Rollout
	unifiedDeps    *unifiedDeps

	// commitRetryRepo — optional persistent буфер для retry CommitCharge при
	// transport/timeout ошибках от billing-service. Nil — enqueue пропускается
	// (лог ошибки остаётся, но автоматический retry не произойдёт).
	commitRetryRepo domain.CommitRetryRepository
}

// SetCommitRetryRepo регистрирует persistent retry очередь для CommitCharge.
// Вызывается из main при наличии DB + старте CommitRetryWorker.
func (s *TarificationService) SetCommitRetryRepo(repo domain.CommitRetryRepository) {
	s.commitRetryRepo = repo
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

// SetQuotaService подключает сервис квот агрегатора
func (s *TarificationService) SetQuotaService(qs *QuotaService) {
	s.quotaService = qs
}

// SetUnifiedDependencies wires Phase 3 unified pricing components. Called
// from tarification-service main.go under cfg.Tarification.UnifiedEnabled.
// If rollout is nil, the unified path stays inert.
func (s *TarificationService) SetUnifiedDependencies(
	enabled bool,
	rollout *Rollout,
	resolver *PriceResolver,
	calc *CostCalculator,
	ruleRepo domain.PriceRuleRepository,
	subUsageRepo domain.SubaccountUsageCounterRepository,
	operatorLookup OperatorMetaLookup,
) {
	s.unifiedEnabled = enabled
	s.rollout = rollout
	s.unifiedDeps = &unifiedDeps{
		resolver:       resolver,
		calc:           calc,
		ruleRepo:       ruleRepo,
		subUsageRepo:   subUsageRepo,
		marginLogRepo:  s.aggMarginLogRepo,
		saga:           s.saga,
		logRepo:        s.logRepo,
		senderRepo:     s.senderRepo,
		operatorLookup: operatorLookup,
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

	// Phase 2 dual-charge поля — заполняются через Calculate.
	AggregatorID    string
	OperatorID      string
	SubAccountPrice string
	AggregatorPrice string
	SubAccountTotal string
	AggregatorTotal string
	SegmentCount    int32
	PoolSegments    int32
	OverageSegments int32
	ChargeMode      string
}

// calcResultToResponse маппит read-only CalculateResult в TarifyMessageResponse.
// TotalAmount выбирается по типу клиента: PlatformAmount для direct, SubAccountTotal
// для субаккаунта (это то, что списывалось бы с клиентского баланса в legacy flow).
func calcResultToResponse(c *CalculateResult) *TarifyMessageResponse {
	resp := &TarifyMessageResponse{
		Approved:         c.Approved,
		Currency:         c.Currency,
		Strategy:         c.Strategy,
		TariffPlanID:     c.TariffPlanID,
		RejectionReason:  c.RejectionReason,
		ThresholdCrossed: c.ThresholdCrossed,
		RecalcAmount:     c.RecalcAmount,
		SubAccountPrice:  c.SubAccountPrice,
		AggregatorPrice:  c.AggregatorPrice,
		SubAccountTotal:  c.SubAccountTotal,
		AggregatorTotal:  c.AggregatorTotal,
		SegmentCount:     int32(c.SegmentCount),
		PoolSegments:     int32(c.PoolSegments),
		OverageSegments:  int32(c.OverageSegments),
		ChargeMode:       c.ChargeMode,
	}
	if c.AggregatorID != uuid.Nil {
		resp.AggregatorID = c.AggregatorID.String()
	}
	if c.OperatorID != uuid.Nil {
		resp.OperatorID = c.OperatorID.String()
	}
	if c.IsDirect {
		resp.TotalAmount = c.PlatformAmount
	} else {
		resp.TotalAmount = c.SubAccountTotal
	}
	return resp
}

// TarifyMessage тарифицирует сообщение при отправке
func (s *TarificationService) TarifyMessage(ctx context.Context, req *TarifyMessageRequest) (*TarifyMessageResponse, error) {
	// 1. Проверка идемпотентности
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
			// Unified replay: резолвим currency через тот же cached lookup,
			// что использовался при первом вызове; fallback на RUB если lookup
			// недоступен (response-only, billing перевалидирует).
			existingCurrency = "RUB"
			// Invariant: SetUnifiedDependencies always populates operatorLookup non-nil,
			// но гард на nil-deps защищает от вызова из legacy-only деплоя, где setter не вызывался.
			if s.unifiedDeps != nil && s.unifiedDeps.operatorLookup != nil {
				if meta, metaErr := s.unifiedDeps.operatorLookup.Meta(ctx, existing.OperatorID); metaErr == nil && meta.Currency != "" {
					existingCurrency = meta.Currency
				}
			}
			tariffPlanID = existing.SourceRuleID.String()
		}
		return &TarifyMessageResponse{
			Approved:     true,
			TotalAmount:  existing.TotalAmount,
			Currency:     existingCurrency,
			Strategy:     string(existing.Strategy),
			TariffPlanID: tariffPlanID,
		}, nil
	}

	// Phase 3 unified gate. Active only if unified_enabled=true AND rollout
	// selects this subaccount. On non-fatal failure (fallbackReason != "")
	// execution falls through to default Phase 2 path below.
	if s.unifiedEnabled && s.rollout != nil && s.rollout.Enabled(req.ClientID) && s.unifiedDeps != nil {
		resp, fallbackReason, err := tarifyUnified(ctx, req, s.unifiedDeps)
		if err != nil {
			return nil, err
		}
		if fallbackReason == "" {
			return resp, nil
		}
		// else: fall through to default Calculate.
	}

	// Default Phase 2 path: read-only Calculate. Фактическое списание
	// выполняется в CommitCharge после успешного SUBMIT в pipeline.
	calc, err := s.Calculate(ctx, req)
	if err != nil {
		return nil, err
	}
	return calcResultToResponse(calc), nil
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
