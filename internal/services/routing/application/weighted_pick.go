package application

import (
	"math/rand"
	"sync"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// pickRNG is the default RNG used by PickWeightedRoute. It is seeded once at
// package init from a cryptographically-random source via rand.NewSource-less
// construction; we use the global rand here with a process-local mutex to stay
// safe under concurrent stage workers. Tests that need determinism must use
// PickWeightedRouteWithRand and pass their own *rand.Rand.
var (
	pickRNGMu sync.Mutex
	pickRNG   = rand.New(rand.NewSource(1)) // seed replaced in init()
)

func init() {
	// Re-seed on startup so distribution is not identical across process
	// restarts. Determinism for tests is provided by PickWeightedRouteWithRand.
	pickRNGMu.Lock()
	defer pickRNGMu.Unlock()
	pickRNG = rand.New(rand.NewSource(rand.Int63()))
}

// PickWeightedRoute chooses one route from a priority-ordered list using
// `share` as a weighting factor within the top (lowest) priority bucket.
//
// Contract:
//   - `routes` MUST be sorted ascending by Priority (matcher guarantees this).
//   - Only routes sharing the top priority compete in the weighted draw.
//     Lower-priority routes act as fallback and are ignored while a higher-
//     priority bucket has any route.
//   - When all routes in the top bucket have Share == 0 (the common legacy
//     case and the default after migration), the FIRST route of that bucket
//     is returned — deterministic, matches pre-weighted behaviour.
//   - When at least one route in the bucket has Share > 0, routes with
//     Share <= 0 in that bucket are EXCLUDED from the draw (operator opted
//     them out of the split). If every non-excluded route still has
//     Share == 0, fall back to the first by insertion order.
//   - nil / empty input → returns nil.
//
// Tie-breaker documentation: with N routes of identical Share in the top
// bucket, each is selected with probability 1/N (cumulative-distribution
// pick over Σshare). With a single route in the bucket the result is that
// route regardless of Share value.
func PickWeightedRoute(routes []*domain.ClientRoute) *domain.ClientRoute {
	// Delegate the Intn call to a closure that holds pickRNGMu for the full
	// duration of the RNG state mutation. math/rand.Rand is NOT goroutine-safe,
	// so we MUST NOT leak the *rand.Rand pointer outside the mutex — doing so
	// would let two pipeline-router workers race on the same internal state
	// and corrupt it / panic under `-race`.
	return pickWeightedRouteWithRoller(routes, func(n int) int {
		pickRNGMu.Lock()
		defer pickRNGMu.Unlock()
		return pickRNG.Intn(n)
	})
}

// PickWeightedRouteWithRand is the deterministic variant of PickWeightedRoute.
// Intended for tests and replay. The RNG is used only when the top priority
// bucket contains 2+ routes with Share > 0; otherwise the result is
// deterministic without any RNG calls.
//
// Caller owns synchronization on r. If r is nil, the package-level RNG is
// used (serialized by pickRNGMu).
func PickWeightedRouteWithRand(routes []*domain.ClientRoute, r *rand.Rand) *domain.ClientRoute {
	if r == nil {
		return PickWeightedRoute(routes)
	}
	return pickWeightedRouteWithRoller(routes, r.Intn)
}

// pickWeightedRouteWithRoller is the shared implementation. `roller(n)` MUST
// return a uniformly random integer in [0,n) and MUST be safe to call from the
// current goroutine (i.e. either wraps a mutex-protected RNG or is called from
// a goroutine that owns the RNG exclusively).
func pickWeightedRouteWithRoller(routes []*domain.ClientRoute, roller func(int) int) *domain.ClientRoute {
	if len(routes) == 0 {
		return nil
	}

	// Isolate the top priority bucket. Input is sorted ascending by Priority.
	topPriority := routes[0].Priority
	bucketEnd := 1
	for bucketEnd < len(routes) && routes[bucketEnd].Priority == topPriority {
		bucketEnd++
	}
	bucket := routes[:bucketEnd]

	if len(bucket) == 1 {
		return bucket[0]
	}

	// Compute total share. Only positive shares participate.
	total := 0
	for _, rt := range bucket {
		if rt.Share > 0 {
			total += rt.Share
		}
	}

	// Fallback: no route in the top bucket has a positive share. Pick first
	// by insertion order (stable, legacy-compatible).
	if total == 0 {
		return bucket[0]
	}

	roll := roller(total)

	cumulative := 0
	for _, rt := range bucket {
		if rt.Share <= 0 {
			continue
		}
		cumulative += rt.Share
		if roll < cumulative {
			return rt
		}
	}

	// Unreachable in theory (rounding/race), but return a safe fallback.
	return bucket[0]
}
