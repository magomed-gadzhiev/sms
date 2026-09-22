package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// --- fakes ---

type fakeRuleRepo struct {
	byLookup *domain.PriceRule
	err      error
	called   int
}

func (f *fakeRuleRepo) FindApplicable(_ context.Context, _ domain.ResolveInput) (*domain.PriceRule, error) {
	f.called++
	return f.byLookup, f.err
}
func (f *fakeRuleRepo) Create(context.Context, *domain.PriceRule) error { return nil }
func (f *fakeRuleRepo) Update(context.Context, *domain.PriceRule) error { return nil }
func (f *fakeRuleRepo) Delete(context.Context, uuid.UUID) error         { return nil }
func (f *fakeRuleRepo) GetByID(context.Context, uuid.UUID) (*domain.PriceRule, error) {
	return nil, nil
}
func (f *fakeRuleRepo) HasPlatformCatchAll(context.Context) (bool, error) { return true, nil }

type fakeResolvedRepo struct {
	cached   *domain.ResolvedRule
	upserted *domain.ResolvedRule
}

func (f *fakeResolvedRepo) Get(context.Context, uuid.UUID, string, string, string, string, time.Time) (*domain.ResolvedRule, error) {
	return f.cached, nil
}
func (f *fakeResolvedRepo) Upsert(_ context.Context, r *domain.ResolvedRule) error {
	f.upserted = r
	return nil
}
func (f *fakeResolvedRepo) DeleteAffected(context.Context, domain.PriceOwnerType, *uuid.UUID, *string, *string, *string, *string) (int64, error) {
	return 0, nil
}

type fakeVersionRepo struct{ v int64 }

func (f *fakeVersionRepo) GetVersion(context.Context) (int64, error) { return f.v, nil }

type fakeAggResolver struct{ aggID uuid.UUID }

func (f *fakeAggResolver) AggregatorFor(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return f.aggID, nil
}

// --- tests ---

func TestPriceResolver_CacheHit_SameVersion_NoLookup(t *testing.T) {
	subID := uuid.New()
	aggID := uuid.New()
	ruleID := uuid.New()

	cached := &domain.ResolvedRule{
		SubaccountID:   subID,
		AggregatorID:   aggID,
		Country:        "RU",
		Operator:       "MTS",
		SenderCategory: "paid",
		TrafficType:    "transactional",
		EffectiveDate:  time.Now().UTC().Truncate(24 * time.Hour),
		PriceModel:     domain.ModelFixed,
		PriceValue:     strPtr("1.5"),
		SourceRuleID:   ruleID,
		SourceLevel:    domain.OwnerPlatform,
		RulesVersion:   5,
	}
	ruleRepo := &fakeRuleRepo{}
	resolver := NewPriceResolver(
		ruleRepo,
		&fakeResolvedRepo{cached: cached},
		&fakeVersionRepo{v: 5},
		&fakeAggResolver{aggID: aggID},
	)

	rr, err := resolver.Resolve(context.Background(), domain.ResolveInput{
		SubaccountID: subID, Country: "RU", Operator: "MTS",
		SenderCategory: "paid", TrafficType: "transactional",
		Now: time.Now(),
	})
	require.NoError(t, err)
	require.Equal(t, ruleID, rr.SourceRuleID)
	require.Zero(t, ruleRepo.called, "cache hit should skip ruleRepo lookup")
}

func TestPriceResolver_CacheMiss_FallsBackToLookup(t *testing.T) {
	subID := uuid.New()
	aggID := uuid.New()
	ruleID := uuid.New()

	rule := &domain.PriceRule{
		ID:         ruleID,
		OwnerType:  domain.OwnerPlatform,
		PriceModel: domain.ModelFixed,
		PriceValue: strPtr("2.0"),
		ValidFrom:  time.Now().Add(-time.Hour),
	}
	resolvedRepo := &fakeResolvedRepo{cached: nil}
	ruleRepo := &fakeRuleRepo{byLookup: rule}
	resolver := NewPriceResolver(
		ruleRepo,
		resolvedRepo,
		&fakeVersionRepo{v: 10},
		&fakeAggResolver{aggID: aggID},
	)

	rr, err := resolver.Resolve(context.Background(), domain.ResolveInput{
		SubaccountID: subID, Country: "RU", Operator: "MTS",
		SenderCategory: "paid", TrafficType: "transactional",
		Now: time.Now(),
	})
	require.NoError(t, err)
	require.Equal(t, ruleID, rr.SourceRuleID)
	require.Equal(t, 1, ruleRepo.called)
	require.NotNil(t, resolvedRepo.upserted)
	require.Equal(t, int64(10), resolvedRepo.upserted.RulesVersion)
	require.Equal(t, aggID, resolvedRepo.upserted.AggregatorID)
}

func TestPriceResolver_StaleVersion_ReresolvesAndUpserts(t *testing.T) {
	subID := uuid.New()
	aggID := uuid.New()

	cached := &domain.ResolvedRule{
		SubaccountID: subID,
		AggregatorID: aggID,
		RulesVersion: 3, // stale
	}
	newRule := &domain.PriceRule{
		ID:         uuid.New(),
		OwnerType:  domain.OwnerPlatform,
		PriceModel: domain.ModelFixed,
		PriceValue: strPtr("3.0"),
		ValidFrom:  time.Now().Add(-time.Hour),
	}
	resolvedRepo := &fakeResolvedRepo{cached: cached}
	ruleRepo := &fakeRuleRepo{byLookup: newRule}
	resolver := NewPriceResolver(
		ruleRepo,
		resolvedRepo,
		&fakeVersionRepo{v: 10},
		&fakeAggResolver{aggID: aggID},
	)

	rr, err := resolver.Resolve(context.Background(), domain.ResolveInput{
		SubaccountID: subID, Country: "RU", Operator: "MTS",
		SenderCategory: "paid", TrafficType: "transactional",
		Now: time.Now(),
	})
	require.NoError(t, err)
	require.Equal(t, newRule.ID, rr.SourceRuleID)
	require.Equal(t, 1, ruleRepo.called)
	require.NotNil(t, resolvedRepo.upserted)
	require.Equal(t, int64(10), resolvedRepo.upserted.RulesVersion)
}

func TestPriceResolver_NoRule_Propagates(t *testing.T) {
	resolver := NewPriceResolver(
		&fakeRuleRepo{err: domain.ErrNoApplicableRule},
		&fakeResolvedRepo{cached: nil},
		&fakeVersionRepo{v: 1},
		&fakeAggResolver{aggID: uuid.New()},
	)
	_, err := resolver.Resolve(context.Background(), domain.ResolveInput{
		SubaccountID: uuid.New(), Country: "XX", Operator: "YY",
		SenderCategory: "none", TrafficType: "service", Now: time.Now(),
	})
	require.ErrorIs(t, err, domain.ErrNoApplicableRule)
}
