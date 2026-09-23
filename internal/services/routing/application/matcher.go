package application

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/infrastructure"
)

// MatchContext holds all fields used to match a request against routes.
type MatchContext struct {
	RouteType   string
	ClientID    uuid.UUID
	OperatorID  *uuid.UUID
	CountryCode string
	TrafficType domain.TrafficType
	PaidName    bool
	MessageBody string
	SenderName  string
}

// RouteStore is the persistence seam the matcher loads its world from.
// *infrastructure.RouteRepo satisfies it; tests substitute fakes.
type RouteStore interface {
	LoadAllActive(ctx context.Context) ([]*domain.ClientRoute, error)
	LoadClientRouting(ctx context.Context) (map[uuid.UUID]infrastructure.ClientRoutingInfo, error)
}

// ErrNoRouteFound is returned by Resolve when no route at any allowed
// resolution level matches the context.
var ErrNoRouteFound = errors.New("no route found")

// ResolutionLevel names which level of Routing Resolution produced the
// decision (see CONTEXT.md: client's own routes, then the Reseller's shared
// route sets, then the platform default — first match wins).
type ResolutionLevel string

const (
	LevelClient   ResolutionLevel = "client"
	LevelReseller ResolutionLevel = "reseller"
	LevelPlatform ResolutionLevel = "platform"
)

// RoutingDecision is the outcome of Routing Resolution: the winning route and
// the level it came from.
type RoutingDecision struct {
	Route *domain.ClientRoute
	Level ResolutionLevel
}

// RouteMatcher is the single Routing Resolution engine. It holds all active
// managed routes plus per-client routing info (parent, Routing Mode) in
// memory and resolves a MatchContext to one route.
type RouteMatcher struct {
	mu          sync.RWMutex
	routes      []*domain.ClientRoute
	clientInfo  map[uuid.UUID]infrastructure.ClientRoutingInfo
	regexCache  map[string]*regexp.Regexp
	store       RouteStore
}

// NewRouteMatcher creates a new RouteMatcher backed by the given store.
func NewRouteMatcher(store RouteStore) *RouteMatcher {
	return &RouteMatcher{
		store:      store,
		regexCache: make(map[string]*regexp.Regexp),
	}
}

// Load fetches all active routes and client routing info from the database,
// compiles regex patterns, and stores everything in memory under a write lock.
func (m *RouteMatcher) Load(ctx context.Context) error {
	routes, err := m.store.LoadAllActive(ctx)
	if err != nil {
		return err
	}
	clientInfo, err := m.store.LoadClientRouting(ctx)
	if err != nil {
		return err
	}

	cache := make(map[string]*regexp.Regexp)
	for _, route := range routes {
		for _, g := range route.Groups {
			for _, c := range g.Conditions {
				if c.Type == domain.ConditionRegex {
					pattern := stripRegexDelimiters(c.Value)
					if _, exists := cache[pattern]; !exists {
						compiled, compErr := regexp.Compile("(?i)" + pattern)
						if compErr != nil {
							log.Warn().Err(compErr).
								Str("pattern", c.Value).
								Msg("failed to compile regex condition, skipping")
							continue
						}
						cache[pattern] = compiled
					}
				}
			}
		}
	}

	m.mu.Lock()
	m.routes = routes
	m.clientInfo = clientInfo
	m.regexCache = cache
	m.mu.Unlock()

	log.Info().Int("routes", len(routes)).Int("clients", len(clientInfo)).Int("regex_patterns", len(cache)).Msg("route matcher loaded")
	return nil
}

// Invalidate reloads all routes from the database, logging any error.
func (m *RouteMatcher) Invalidate(ctx context.Context) {
	if err := m.Load(ctx); err != nil {
		log.Error().Err(err).Msg("route matcher invalidation failed")
	}
}

// Resolve applies the domain rule of Routing Resolution (CONTEXT.md): the
// Client's own routes first, then its Reseller's shared route sets, then the
// platform default; first match wins. Within the winning level, routes are
// ranked by Priority and split by Share via PickWeightedRoute.
//
// The per-Client Routing Mode (ADR-0002: a transition mechanism) gates which
// levels are consulted — semantics folded from the retired UnifiedRouter:
//
//	legacy — platform defaults only
//	new    — the client's own routes only, no fallback
//	hybrid (and any unknown value) — full three-level fallback
func (m *RouteMatcher) Resolve(ctx context.Context, mc MatchContext) (RoutingDecision, error) {
	m.mu.RLock()
	routes := m.routes
	clientInfo := m.clientInfo
	regexCache := m.regexCache
	m.mu.RUnlock()

	mode := "hybrid"
	if info, ok := clientInfo[mc.ClientID]; ok {
		mode = info.Mode
	}

	// Split routes by route_type into the three ownership buckets.
	var clientRoutes, resellerRoutes, platformRoutes []*domain.ClientRoute
	var parentID *uuid.UUID
	if info, ok := clientInfo[mc.ClientID]; ok {
		parentID = info.ParentID
	}
	for _, r := range routes {
		if r.RouteType != mc.RouteType {
			continue
		}
		switch {
		case r.ClientID != nil && *r.ClientID == mc.ClientID:
			clientRoutes = append(clientRoutes, r)
		case r.ClientID != nil && parentID != nil && *r.ClientID == *parentID && r.Shared:
			resellerRoutes = append(resellerRoutes, r)
		case r.ClientID == nil:
			platformRoutes = append(platformRoutes, r)
		}
	}

	var levels []ResolutionLevel
	var buckets map[ResolutionLevel][]*domain.ClientRoute
	switch mode {
	case "legacy":
		levels = []ResolutionLevel{LevelPlatform}
	case "new":
		levels = []ResolutionLevel{LevelClient}
	default: // hybrid and unknown
		levels = []ResolutionLevel{LevelClient, LevelReseller, LevelPlatform}
	}
	buckets = map[ResolutionLevel][]*domain.ClientRoute{
		LevelClient:   clientRoutes,
		LevelReseller: resellerRoutes,
		LevelPlatform: platformRoutes,
	}

	for _, level := range levels {
		matched := m.filterMatching(buckets[level], mc, regexCache)
		if len(matched) == 0 {
			continue
		}
		sort.Slice(matched, func(i, j int) bool {
			return matched[i].Priority < matched[j].Priority
		})
		route := PickWeightedRoute(matched)
		if route == nil {
			// Defensive: PickWeightedRoute returns nil only for empty input.
			continue
		}
		return RoutingDecision{Route: route, Level: level}, nil
	}

	return RoutingDecision{}, ErrNoRouteFound
}

// filterMatching returns routes whose conditions and schedules pass.
func (m *RouteMatcher) filterMatching(routes []*domain.ClientRoute, ctx MatchContext, regexCache map[string]*regexp.Regexp) []*domain.ClientRoute {
	var result []*domain.ClientRoute
	for _, r := range routes {
		if !evaluateConditions(r.Groups, ctx, regexCache) {
			continue
		}
		if !checkSchedules(r.Schedules) {
			continue
		}
		result = append(result, r)
	}
	return result
}

// --- condition evaluation ---

// evaluateConditions applies the chain of condition groups using their logic operators.
//
// The running `result` is seeded based on the FIRST group's logic_op so that a
// chain beginning with AND/AND_NOT has the correct identity element. Without
// this seed a single-group route with logic_op=AND would evaluate to
// (false && groupMatch) = false no matter what, silently never matching
// (see QA 2026-04-22 bug #3 / project bug #12).
//
// Seed semantics per op:
//   - IF         : result = groupMatch (op overwrites; seed irrelevant)
//   - AND, AND_NOT: identity is true — chain of ANDs requires true start
//   - OR, OR_NOT : identity is false — chain of ORs requires false start
//   - unknown    : treated as IF (seed irrelevant)
func evaluateConditions(groups []domain.ConditionGroup, ctx MatchContext, regexCache map[string]*regexp.Regexp) bool {
	if len(groups) == 0 {
		return true
	}

	// Seed the running result from the FIRST group's logic op. For subsequent
	// groups the seed is immaterial because `result` carries the previous state.
	var result bool
	switch groups[0].LogicOp {
	case domain.LogicAnd, domain.LogicAndNot:
		result = true
	default:
		// LogicIf overwrites; LogicOr/LogicOrNot identity is false; unknown
		// treated as IF (also overwrites). `false` is correct for all.
		result = false
	}

	for _, g := range groups {
		groupMatch := evaluateGroup(g, ctx, regexCache)

		switch g.LogicOp {
		case domain.LogicIf:
			result = groupMatch
		case domain.LogicAnd:
			result = result && groupMatch
		case domain.LogicAndNot:
			result = result && !groupMatch
		case domain.LogicOr:
			result = result || groupMatch
		case domain.LogicOrNot:
			result = result || !groupMatch
		default:
			// Unknown op treated as IF.
			result = groupMatch
		}
	}
	return result
}

// evaluateGroup groups conditions by type (OR within same type, AND across types).
func evaluateGroup(g domain.ConditionGroup, ctx MatchContext, regexCache map[string]*regexp.Regexp) bool {
	if len(g.Conditions) == 0 {
		return true
	}

	// Group conditions by type.
	byType := make(map[domain.ConditionType][]domain.Condition)
	for _, c := range g.Conditions {
		byType[c.Type] = append(byType[c.Type], c)
	}

	// All types must match (AND). Within a type, any match suffices (OR).
	for _, conds := range byType {
		typeMatched := false
		for _, c := range conds {
			if evaluateCondition(c, ctx, regexCache) {
				typeMatched = true
				break
			}
		}
		if !typeMatched {
			return false
		}
	}
	return true
}

// evaluateCondition checks a single condition against the context.
func evaluateCondition(c domain.Condition, ctx MatchContext, regexCache map[string]*regexp.Regexp) bool {
	switch c.Type {
	case domain.ConditionOperator:
		if ctx.OperatorID == nil {
			return false
		}
		return c.Value == ctx.OperatorID.String()

	case domain.ConditionCountry:
		return strings.EqualFold(c.Value, ctx.CountryCode)

	case domain.ConditionTrafficType:
		return strings.EqualFold(c.Value, string(ctx.TrafficType))

	case domain.ConditionPaidName:
		return (c.Value == "true") == ctx.PaidName

	case domain.ConditionRegex:
		pattern := stripRegexDelimiters(c.Value)
		re, ok := regexCache[pattern]
		if !ok {
			return false
		}
		return re.MatchString(ctx.MessageBody) || re.MatchString(ctx.SenderName)

	default:
		return false
	}
}

// --- schedule checking ---

// checkSchedules returns true if there are no schedules or at least one schedule matches now.
func checkSchedules(schedules []domain.Schedule) bool {
	if len(schedules) == 0 {
		return true
	}
	for _, s := range schedules {
		if matchSchedule(s) {
			return true
		}
	}
	return false
}

// matchSchedule checks whether the current time falls within the schedule.
func matchSchedule(s domain.Schedule) bool {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		log.Warn().Err(err).Str("timezone", s.Timezone).Msg("invalid schedule timezone, skipping")
		return false
	}
	now := time.Now().In(loc)

	// Check date range.
	if s.DateFrom != nil && now.Before(*s.DateFrom) {
		return false
	}
	if s.DateTo != nil {
		// DateTo is a DATE column — midnight of that day. Include the entire last day.
		endOfDay := s.DateTo.Add(24*time.Hour - time.Nanosecond)
		if now.After(endOfDay) {
			return false
		}
	}

	// Check weekday bitmask: 1=Mon, 2=Tue, 4=Wed, 8=Thu, 16=Fri, 32=Sat, 64=Sun.
	// time.Weekday: 0=Sun, 1=Mon ... 6=Sat.
	// Weekdays == 0 means no days selected — route never active.
	if s.Weekdays == 0 {
		return false
	}
	wd := now.Weekday()
	var bit int
	if wd == time.Sunday {
		bit = 64
	} else {
		bit = 1 << (wd - 1) // Mon=1<<0=1, Tue=1<<1=2, etc.
	}
	if s.Weekdays&bit == 0 {
		return false
	}

	// Check time range.
	nowTime := now.Format("15:04:05")
	if s.TimeFrom != nil && nowTime < *s.TimeFrom {
		return false
	}
	if s.TimeTo != nil && nowTime > *s.TimeTo {
		return false
	}

	return true
}

// --- helpers ---

// stripRegexDelimiters removes surrounding /pattern/flags notation used in some regex stores.
func stripRegexDelimiters(pattern string) string {
	if len(pattern) < 2 || pattern[0] != '/' {
		return pattern
	}
	lastSlash := strings.LastIndex(pattern[1:], "/")
	if lastSlash >= 0 {
		return pattern[1 : lastSlash+1]
	}
	return pattern
}
