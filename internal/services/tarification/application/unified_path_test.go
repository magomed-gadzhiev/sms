package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// --- local stubs for unified_path tests ---

type stubSubUsage struct {
	get       *domain.SubaccountUsageCounter
	incCalled bool
}

func (s *stubSubUsage) Get(_ context.Context, _ uuid.UUID, _ string) (*domain.SubaccountUsageCounter, error) {
	return s.get, nil
}
func (s *stubSubUsage) Increment(_ context.Context, _ uuid.UUID, _ string, _ int64, _ string) (int64, error) {
	s.incCalled = true
	return 0, nil
}

type stubLogRepo struct {
	existing *domain.TarificationLog
	created  *domain.TarificationLog
}

func (s *stubLogRepo) GetByIdempotencyKey(_ context.Context, _ string) (*domain.TarificationLog, error) {
	return s.existing, nil
}
func (s *stubLogRepo) Create(_ context.Context, log *domain.TarificationLog) error {
	s.created = log
	return nil
}
func (s *stubLogRepo) GetByMessageID(_ context.Context, _ uuid.UUID) (*domain.TarificationLog, error) {
	return nil, nil
}

type stubMarginLog struct {
	created *domain.AggregatorMarginLog
}

func (s *stubMarginLog) Create(_ context.Context, entry *domain.AggregatorMarginLog) error {
	s.created = entry
	return nil
}

type stubSaga struct {
	result *ChargeResult
	err    error
}

func (s *stubSaga) Charge(_ context.Context, _, _, _, _, _ string, _ int32) (*ChargeResult, error) {
	return s.result, s.err
}

type stubOperatorLookup struct {
	meta  domain.OperatorMeta
	err   error
	calls int
}

func (s *stubOperatorLookup) Meta(_ context.Context, _ uuid.UUID) (domain.OperatorMeta, error) {
	s.calls++
	return s.meta, s.err
}

type stubSenderRepo struct{}

func (s *stubSenderRepo) Create(_ context.Context, _ *domain.SenderRegistration) error { return nil }
func (s *stubSenderRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.SenderRegistration, error) {
	return nil, nil
}
func (s *stubSenderRepo) GetByClientAndOperator(_ context.Context, _, _ uuid.UUID) ([]*domain.SenderRegistration, error) {
	return nil, nil
}
func (s *stubSenderRepo) GetActiveByClientOperatorName(_ context.Context, _, _ uuid.UUID, _ string) (*domain.SenderRegistration, error) {
	return nil, nil
}
func (s *stubSenderRepo) Update(_ context.Context, _ *domain.SenderRegistration) error { return nil }
func (s *stubSenderRepo) List(_ context.Context, _, _ *uuid.UUID, _, _ int) ([]*domain.SenderRegistration, int, error) {
	return nil, 0, nil
}
func (s *stubSenderRepo) ListActivePaid(_ context.Context) ([]*domain.SenderRegistration, error) {
	return nil, nil
}

// callCountingRuleRepo returns different rules per call to simulate subaccount (1st)
// vs margin (2nd) resolve. Embedded in local tests only.
type callCountingRuleRepo struct {
	calls    int
	rules    []*domain.PriceRule // rules[0] for 1st call, rules[1] for 2nd, etc.
	fallback *domain.PriceRule
	err      error
}

func (r *callCountingRuleRepo) FindApplicable(_ context.Context, _ domain.ResolveInput) (*domain.PriceRule, error) {
	if r.err != nil {
		r.calls++
		return nil, r.err
	}
	idx := r.calls
	r.calls++
	if idx < len(r.rules) {
		return r.rules[idx], nil
	}
	return r.fallback, nil
}
func (r *callCountingRuleRepo) Create(_ context.Context, _ *domain.PriceRule) error             { return nil }
func (r *callCountingRuleRepo) Update(_ context.Context, _ *domain.PriceRule) error             { return nil }
func (r *callCountingRuleRepo) Delete(_ context.Context, _ uuid.UUID) error                     { return nil }
func (r *callCountingRuleRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.PriceRule, error) {
	return nil, nil
}
func (r *callCountingRuleRepo) HasPlatformCatchAll(_ context.Context) (bool, error) { return true, nil }

// buildHappyDeps wires all deps for a happy-path test.
// Returns (deps, subUsage stub, logRepo stub, marginLog stub, operatorLookup stub).
func buildHappyDeps(
	sagaResult *ChargeResult,
) (*unifiedDeps, *stubSubUsage, *stubLogRepo, *stubMarginLog, *stubOperatorLookup, *callCountingRuleRepo) {
	aggID := uuid.New()
	ruleID := uuid.New()
	subRule := &domain.PriceRule{
		ID:         ruleID,
		OwnerType:  domain.OwnerSubaccount,
		PriceModel: domain.ModelFixed,
		PriceValue: strPtr("1.5"),
		ValidFrom:  time.Now().Add(-time.Hour),
	}
	aggRule := &domain.PriceRule{
		ID:         uuid.New(),
		OwnerType:  domain.OwnerAggregator,
		PriceModel: domain.ModelFixed,
		PriceValue: strPtr("1.0"),
		ValidFrom:  time.Now().Add(-time.Hour),
	}
	ruleRepo := &callCountingRuleRepo{rules: []*domain.PriceRule{subRule, aggRule}}

	resolvedRepo := &fakeResolvedRepo{cached: nil}
	versionRepo := &fakeVersionRepo{v: 1}
	aggResolver := &fakeAggResolver{aggID: aggID}

	resolver := NewPriceResolver(ruleRepo, resolvedRepo, versionRepo, aggResolver)
	calc := NewCostCalculator(NewTiersCache(256))

	subUsage := &stubSubUsage{get: nil}
	logRepo := &stubLogRepo{existing: nil}
	marginLog := &stubMarginLog{}
	opLookup := &stubOperatorLookup{meta: domain.OperatorMeta{Code: "mts-ru", Currency: "RUB"}}
	saga := &stubSaga{result: sagaResult}

	deps := &unifiedDeps{
		resolver:      resolver,
		calc:          calc,
		ruleRepo:      ruleRepo,
		subUsageRepo:  subUsage,
		marginLogRepo: marginLog,
		saga:          saga,
		logRepo:       logRepo,
		senderRepo:    &stubSenderRepo{},
		operatorLookup: opLookup,
	}
	return deps, subUsage, logRepo, marginLog, opLookup, ruleRepo
}

// --- tests ---

func TestTarifyUnified_HappyPathApproves(t *testing.T) {
	sagaOK := &ChargeResult{Success: true, TransactionID: "tx-1"}
	deps, subUsage, logRepo, marginLog, _, _ := buildHappyDeps(sagaOK)

	req := &TarifyMessageRequest{
		ClientID:       uuid.New(),
		MessageID:      uuid.New(),
		OperatorID:     uuid.New(),
		SenderName:     "",
		SegmentCount:   2,
		IdempotencyKey: "idem-happy-1",
	}

	beforeApproved := testutil.ToFloat64(unifiedTarifyTotal.WithLabelValues("approved"))

	resp, fallback, err := tarifyUnified(context.Background(), req, deps)

	require.NoError(t, err)
	require.Equal(t, "", fallback)
	require.NotNil(t, resp)
	require.True(t, resp.Approved)
	require.Equal(t, "3.000000", resp.TotalAmount)
	require.Equal(t, "RUB", resp.Currency)
	require.Equal(t, string(domain.StrategyUnified), resp.Strategy)

	require.True(t, subUsage.incCalled, "usage counter increment should be called")

	require.NotNil(t, marginLog.created)
	require.Equal(t, "1.000000", marginLog.created.Margin) // 3.0 - 2.0

	require.NotNil(t, logRepo.created)
	require.Equal(t, domain.StrategyUnified, logRepo.created.Strategy)
	require.Equal(t, "3.000000", logRepo.created.TotalAmount)
	require.Equal(t, 2, logRepo.created.SegmentCount)

	afterApproved := testutil.ToFloat64(unifiedTarifyTotal.WithLabelValues("approved"))
	require.Equal(t, float64(1), afterApproved-beforeApproved, "approved counter should increase by 1")
}

func TestTarifyUnified_FallbackOnNotFound(t *testing.T) {
	ruleRepo := &callCountingRuleRepo{err: fmt.Errorf("wrapped: %w", domain.ErrNoApplicableRule)}
	resolvedRepo := &fakeResolvedRepo{cached: nil}
	versionRepo := &fakeVersionRepo{v: 1}
	aggResolver := &fakeAggResolver{aggID: uuid.New()}

	resolver := NewPriceResolver(ruleRepo, resolvedRepo, versionRepo, aggResolver)
	calc := NewCostCalculator(NewTiersCache(256))

	deps := &unifiedDeps{
		resolver:      resolver,
		calc:          calc,
		ruleRepo:      ruleRepo,
		subUsageRepo:  &stubSubUsage{},
		marginLogRepo: &stubMarginLog{},
		saga:          &stubSaga{result: &ChargeResult{Success: true}},
		logRepo:       &stubLogRepo{},
		senderRepo:    &stubSenderRepo{},
		operatorLookup: &stubOperatorLookup{meta: domain.OperatorMeta{Code: "mts-ru", Currency: "RUB"}},
	}

	req := &TarifyMessageRequest{
		ClientID:       uuid.New(),
		MessageID:      uuid.New(),
		OperatorID:     uuid.New(),
		SegmentCount:   1,
		IdempotencyKey: "idem-notfound-1",
	}

	beforeFallback := testutil.ToFloat64(unifiedFallbackTotal.WithLabelValues("not_found"))
	beforeNotFound := testutil.ToFloat64(unifiedPriceNotFoundTotal)

	resp, fallback, err := tarifyUnified(context.Background(), req, deps)

	require.NoError(t, err)
	require.Equal(t, "not_found", fallback)
	require.Nil(t, resp)

	afterFallback := testutil.ToFloat64(unifiedFallbackTotal.WithLabelValues("not_found"))
	afterNotFound := testutil.ToFloat64(unifiedPriceNotFoundTotal)
	require.Equal(t, float64(1), afterFallback-beforeFallback)
	require.Equal(t, float64(1), afterNotFound-beforeNotFound)
}

func TestTarifyUnified_FallbackOnCalcError(t *testing.T) {
	ruleID := uuid.New()
	badRule := &domain.PriceRule{
		ID:         ruleID,
		OwnerType:  domain.OwnerPlatform,
		PriceModel: domain.ModelTiered,
		TiersJSON:  []byte("{not valid json"),
		ValidFrom:  time.Now().Add(-time.Hour),
	}
	ruleRepo := &callCountingRuleRepo{rules: []*domain.PriceRule{badRule}}
	resolvedRepo := &fakeResolvedRepo{cached: nil}
	versionRepo := &fakeVersionRepo{v: 1}
	aggResolver := &fakeAggResolver{aggID: uuid.New()}

	resolver := NewPriceResolver(ruleRepo, resolvedRepo, versionRepo, aggResolver)
	calc := NewCostCalculator(NewTiersCache(256))

	deps := &unifiedDeps{
		resolver:      resolver,
		calc:          calc,
		ruleRepo:      ruleRepo,
		subUsageRepo:  &stubSubUsage{},
		marginLogRepo: &stubMarginLog{},
		saga:          &stubSaga{result: &ChargeResult{Success: true}},
		logRepo:       &stubLogRepo{},
		senderRepo:    &stubSenderRepo{},
		operatorLookup: &stubOperatorLookup{meta: domain.OperatorMeta{Code: "mts-ru", Currency: "RUB"}},
	}

	req := &TarifyMessageRequest{
		ClientID:       uuid.New(),
		MessageID:      uuid.New(),
		OperatorID:     uuid.New(),
		SegmentCount:   1,
		IdempotencyKey: "idem-calcerr-1",
	}

	beforeFallback := testutil.ToFloat64(unifiedFallbackTotal.WithLabelValues("calc_error"))

	resp, fallback, err := tarifyUnified(context.Background(), req, deps)

	require.NoError(t, err)
	require.Equal(t, "calc_error", fallback)
	require.Nil(t, resp)

	afterFallback := testutil.ToFloat64(unifiedFallbackTotal.WithLabelValues("calc_error"))
	require.Equal(t, float64(1), afterFallback-beforeFallback)
}

func TestTarifyUnified_ChargeFailReturnsRejection(t *testing.T) {
	sagaFail := &ChargeResult{Success: false, Error: domain.ErrInsufficientBalance.Error()}
	deps, subUsage, _, _, _, _ := buildHappyDeps(sagaFail)

	req := &TarifyMessageRequest{
		ClientID:       uuid.New(),
		MessageID:      uuid.New(),
		OperatorID:     uuid.New(),
		SenderName:     "",
		SegmentCount:   2,
		IdempotencyKey: "idem-reject-1",
	}

	beforeRejected := testutil.ToFloat64(unifiedTarifyTotal.WithLabelValues("rejected"))

	resp, fallback, err := tarifyUnified(context.Background(), req, deps)

	require.NoError(t, err)
	require.Equal(t, "", fallback)
	require.NotNil(t, resp)
	require.False(t, resp.Approved)
	require.Equal(t, domain.ErrInsufficientBalance.Error(), resp.RejectionReason)
	require.False(t, subUsage.incCalled, "usage counter must not be incremented on rejected charge")

	afterRejected := testutil.ToFloat64(unifiedTarifyTotal.WithLabelValues("rejected"))
	require.Equal(t, float64(1), afterRejected-beforeRejected)
}

func TestTarifyUnified_IdempotencyShortCircuits(t *testing.T) {
	// Unified replay scenario: TariffPlanID is nil, SourceRuleID is set.
	existingRuleID := uuid.New()
	existing := &domain.TarificationLog{
		ID:          uuid.New(),
		TotalAmount: "7.000000",
		Strategy:    domain.StrategyUnified,
		SourceRuleID: &existingRuleID,
		// TariffPlanID intentionally nil — unified path does not set it.
	}

	logRepo := &stubLogRepo{existing: existing}
	subUsage := &stubSubUsage{}
	marginLog := &stubMarginLog{}
	opLookup := &stubOperatorLookup{meta: domain.OperatorMeta{Code: "mts-ru", Currency: "RUB"}}

	deps := &unifiedDeps{
		resolver:      nil, // should not be reached
		calc:          nil, // should not be reached
		ruleRepo:      nil, // should not be reached
		subUsageRepo:  subUsage,
		marginLogRepo: marginLog,
		saga:          &stubSaga{result: &ChargeResult{Success: true}},
		logRepo:       logRepo,
		senderRepo:    &stubSenderRepo{},
		operatorLookup: opLookup,
	}

	req := &TarifyMessageRequest{
		ClientID:       uuid.New(),
		MessageID:      uuid.New(),
		OperatorID:     uuid.New(),
		SenderName:     "",
		SegmentCount:   2,
		IdempotencyKey: "idem-idempotent-1",
	}

	resp, fallback, err := tarifyUnified(context.Background(), req, deps)

	require.NoError(t, err)
	require.Equal(t, "", fallback)
	require.NotNil(t, resp)
	require.True(t, resp.Approved)
	require.Equal(t, "7.000000", resp.TotalAmount)
	require.Equal(t, "RUB", resp.Currency)
	require.Equal(t, string(domain.StrategyUnified), resp.Strategy)
	// Idempotency short-circuit returns SourceRuleID for unified replays.
	require.Equal(t, existingRuleID.String(), resp.TariffPlanID)

	require.False(t, subUsage.incCalled, "usage counter must not be incremented on idempotency hit")
	require.Nil(t, marginLog.created, "margin log must not be created on idempotency hit")
	require.Equal(t, 0, opLookup.calls, "operator lookup must not be called on idempotency hit")
}

func TestTarifyUnified_OperatorLookupError_Fallback(t *testing.T) {
	ruleRepo := &callCountingRuleRepo{rules: []*domain.PriceRule{}}
	resolvedRepo := &fakeResolvedRepo{cached: nil}
	versionRepo := &fakeVersionRepo{v: 1}
	aggResolver := &fakeAggResolver{aggID: uuid.New()}

	resolver := NewPriceResolver(ruleRepo, resolvedRepo, versionRepo, aggResolver)
	calc := NewCostCalculator(NewTiersCache(256))

	opLookup := &stubOperatorLookup{err: domain.ErrOperatorNotFound}

	deps := &unifiedDeps{
		resolver:      resolver,
		calc:          calc,
		ruleRepo:      ruleRepo,
		subUsageRepo:  &stubSubUsage{},
		marginLogRepo: &stubMarginLog{},
		saga:          &stubSaga{result: &ChargeResult{Success: true}},
		logRepo:       &stubLogRepo{},
		senderRepo:    &stubSenderRepo{},
		operatorLookup: opLookup,
	}

	req := &TarifyMessageRequest{
		ClientID:       uuid.New(),
		MessageID:      uuid.New(),
		OperatorID:     uuid.New(),
		SegmentCount:   1,
		IdempotencyKey: "idem-oplookup-1",
	}

	beforeFallback := testutil.ToFloat64(unifiedFallbackTotal.WithLabelValues("operator_lookup_error"))

	resp, fallback, err := tarifyUnified(context.Background(), req, deps)

	require.NoError(t, err)
	require.Equal(t, "operator_lookup_error", fallback)
	require.Nil(t, resp)

	require.Equal(t, 0, ruleRepo.calls, "rule repo must not be called when operator lookup fails")

	afterFallback := testutil.ToFloat64(unifiedFallbackTotal.WithLabelValues("operator_lookup_error"))
	require.Equal(t, float64(1), afterFallback-beforeFallback)
}

func TestTarifyUnified_EmptyCurrency_Fallback(t *testing.T) {
	ctx := context.Background()
	before := testutil.ToFloat64(unifiedFallbackTotal.WithLabelValues("currency_resolve_error"))

	// Operator found but country.currency=NULL → empty Currency.
	opLookup := &stubOperatorLookup{meta: domain.OperatorMeta{Code: "orphan", Currency: ""}}
	ruleRepo := &callCountingRuleRepo{}

	resolvedRepo := &fakeResolvedRepo{cached: nil}
	versionRepo := &fakeVersionRepo{v: 1}
	aggResolver := &fakeAggResolver{aggID: uuid.New()}

	resolver := NewPriceResolver(ruleRepo, resolvedRepo, versionRepo, aggResolver)
	calc := NewCostCalculator(NewTiersCache(256))

	deps := &unifiedDeps{
		resolver:       resolver,
		calc:           calc,
		ruleRepo:       ruleRepo,
		subUsageRepo:   &stubSubUsage{},
		marginLogRepo:  &stubMarginLog{},
		saga:           &stubSaga{result: &ChargeResult{Success: true}},
		logRepo:        &stubLogRepo{},
		senderRepo:     &stubSenderRepo{},
		operatorLookup: opLookup,
	}

	req := &TarifyMessageRequest{
		ClientID:       uuid.New(),
		MessageID:      uuid.New(),
		OperatorID:     uuid.New(),
		SegmentCount:   1,
		IdempotencyKey: "idem-empty-currency",
	}

	resp, fallback, err := tarifyUnified(ctx, req, deps)

	require.NoError(t, err)
	require.Nil(t, resp)
	require.Equal(t, "currency_resolve_error", fallback)
	after := testutil.ToFloat64(unifiedFallbackTotal.WithLabelValues("currency_resolve_error"))
	require.InDelta(t, 1.0, after-before, 0.001)
	// Rule repo NOT called — currency fallback fires before resolve.
	require.Equal(t, 0, ruleRepo.calls)
}
