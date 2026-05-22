package application

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

func mkRoute(priority, share int) *domain.ClientRoute {
	return &domain.ClientRoute{
		ID:         uuid.New(),
		ProviderID: uuid.New(),
		Priority:   priority,
		Share:      share,
		Active:     true,
		Status:     domain.RouteStatusActive,
	}
}

func TestPickWeightedRoute_NilOrEmpty_ReturnsNil(t *testing.T) {
	assert.Nil(t, PickWeightedRoute(nil))
	assert.Nil(t, PickWeightedRoute([]*domain.ClientRoute{}))
}

func TestPickWeightedRoute_SingleRoute_ReturnsIt(t *testing.T) {
	r := mkRoute(0, 50)
	got := PickWeightedRoute([]*domain.ClientRoute{r})
	assert.Equal(t, r, got)
}

func TestPickWeightedRoute_AllSharesZero_FallsBackToFirst(t *testing.T) {
	// Legacy-compatible path: when every route in the top bucket has share=0
	// the picker must return the FIRST route deterministically.
	r1 := mkRoute(0, 0)
	r2 := mkRoute(0, 0)
	r3 := mkRoute(0, 0)
	got := PickWeightedRoute([]*domain.ClientRoute{r1, r2, r3})
	assert.Equal(t, r1, got, "must return first route when all shares are zero")
}

func TestPickWeightedRoute_OnlyTopPriorityCompetes(t *testing.T) {
	// Priority 0 bucket has share=0 → falls back to first OF THAT BUCKET,
	// NOT to the higher-share priority-10 row. Lower priorities are fallback.
	top := mkRoute(0, 0)
	mid := mkRoute(10, 100)
	got := PickWeightedRoute([]*domain.ClientRoute{top, mid})
	assert.Equal(t, top, got)
}

func TestPickWeightedRoute_WeightedDistribution_Deterministic(t *testing.T) {
	// 70/30 split — run 10_000 draws, expect within 3% of target.
	r70 := mkRoute(0, 70)
	r30 := mkRoute(0, 30)
	routes := []*domain.ClientRoute{r70, r30}

	rng := rand.New(rand.NewSource(42))

	const N = 10_000
	hits := map[uuid.UUID]int{}
	for i := 0; i < N; i++ {
		pick := PickWeightedRouteWithRand(routes, rng)
		require.NotNil(t, pick)
		hits[pick.ID]++
	}

	// Expect 7000 / 3000 ±3% (300).
	got70 := hits[r70.ID]
	got30 := hits[r30.ID]
	assert.InDelta(t, 7000, got70, 300, "70-share bucket outside tolerance")
	assert.InDelta(t, 3000, got30, 300, "30-share bucket outside tolerance")
	assert.Equal(t, N, got70+got30, "every draw must hit exactly one route")
}

func TestPickWeightedRoute_NegativeAndZeroSharesExcluded(t *testing.T) {
	// Mix: one route has share>0, others have share=0 → the positive one
	// must be chosen 100% of draws (since the zero-share routes are excluded
	// from the weighted pool once at least one positive exists).
	rActive := mkRoute(0, 50)
	rZero1 := mkRoute(0, 0)
	rZero2 := mkRoute(0, 0)
	routes := []*domain.ClientRoute{rZero1, rActive, rZero2}

	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 500; i++ {
		pick := PickWeightedRouteWithRand(routes, rng)
		require.NotNil(t, pick)
		assert.Equal(t, rActive.ID, pick.ID,
			"when only one route has share>0 it must always win")
	}
}

func TestPickWeightedRoute_EqualShares_UniformDistribution(t *testing.T) {
	// Three routes with share=1 each — expect ~33/33/33 over many draws.
	r1 := mkRoute(0, 1)
	r2 := mkRoute(0, 1)
	r3 := mkRoute(0, 1)
	routes := []*domain.ClientRoute{r1, r2, r3}

	rng := rand.New(rand.NewSource(7))
	const N = 6000
	hits := map[uuid.UUID]int{}
	for i := 0; i < N; i++ {
		pick := PickWeightedRouteWithRand(routes, rng)
		hits[pick.ID]++
	}
	// Each should be near 2000, tolerate 5% (100).
	assert.InDelta(t, 2000, hits[r1.ID], 150)
	assert.InDelta(t, 2000, hits[r2.ID], 150)
	assert.InDelta(t, 2000, hits[r3.ID], 150)
}

// TestPickWeightedRoute_Concurrent_NoRace spawns many goroutines hammering the
// package-level RNG path. Must pass under `go test -race`. Regression guard
// for the pre-fix bug where *rand.Rand was captured under the mutex and then
// dereferenced outside it — math/rand.Rand is NOT goroutine-safe.
func TestPickWeightedRoute_Concurrent_NoRace(t *testing.T) {
	r1 := mkRoute(0, 70)
	r2 := mkRoute(0, 30)
	routes := []*domain.ClientRoute{r1, r2}

	const goroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				pick := PickWeightedRoute(routes)
				if pick == nil {
					t.Errorf("nil pick in concurrent run")
					return
				}
			}
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Bug #12 regression: single-group route with logic_op=AND must evaluate
// correctly. Previously `result` was seeded to false, so `false && groupMatch`
// was always false, making any AND-initiated single-group route unmatchable.
// ---------------------------------------------------------------------------

func TestEvaluateConditions_SingleGroup_AND_MatchesWhenConditionTrue(t *testing.T) {
	// Regression for QA 2026-04-22 bug #3 (project bug #12).
	// The QA setup used logic_op=AND on a single group containing operator=MTS.
	// Before the fix this silently returned false for every message, making
	// the route dead. After the fix the single-group AND must behave like IF.
	opID := uuid.New()
	ctx := baseCtx()
	ctx.OperatorID = &opID

	groups := []domain.ConditionGroup{
		group(domain.LogicAnd, cond(domain.ConditionOperator, opID.String())),
	}
	assert.True(t, evaluateConditions(groups, ctx, nil),
		"single-group AND must match when condition is satisfied")
}

func TestEvaluateConditions_SingleGroup_AND_NoMatchWhenConditionFalse(t *testing.T) {
	// Inverse of the regression: when the condition itself is false, the
	// group must evaluate false (not spuriously true due to a new bug).
	opID := uuid.New()
	ctx := baseCtx()
	otherID := uuid.New()
	ctx.OperatorID = &otherID

	groups := []domain.ConditionGroup{
		group(domain.LogicAnd, cond(domain.ConditionOperator, opID.String())),
	}
	assert.False(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_SingleGroup_ANDNOT_MatchesWhenConditionFalse(t *testing.T) {
	// Single-group AND_NOT is seeded true too, so it must behave symmetrically:
	// true && !false = true.
	ctx := baseCtx()
	ctx.CountryCode = "RU"

	groups := []domain.ConditionGroup{
		group(domain.LogicAndNot, cond(domain.ConditionCountry, "DE")),
	}
	assert.True(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_SingleGroup_OR_MatchesWhenConditionTrue(t *testing.T) {
	// Single-group OR is seeded false (OR identity): false || true = true.
	ctx := baseCtx()
	ctx.CountryCode = "RU"

	groups := []domain.ConditionGroup{
		group(domain.LogicOr, cond(domain.ConditionCountry, "RU")),
	}
	assert.True(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_SingleGroup_OR_NoMatchWhenConditionFalse(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"

	groups := []domain.ConditionGroup{
		group(domain.LogicOr, cond(domain.ConditionCountry, "DE")),
	}
	assert.False(t, evaluateConditions(groups, ctx, nil))
}
