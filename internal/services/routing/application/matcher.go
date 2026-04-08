package application

import (
	"context"
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

// RouteMatcher loads active routes into memory and matches them against a MatchContext.
type RouteMatcher struct {
	mu         sync.RWMutex
	routes     []*domain.ClientRoute
	regexCache map[string]*regexp.Regexp
	repo       *infrastructure.RouteRepo
}

// NewRouteMatcher creates a new RouteMatcher backed by the given repository.
func NewRouteMatcher(repo *infrastructure.RouteRepo) *RouteMatcher {
	return &RouteMatcher{
		repo:       repo,
		regexCache: make(map[string]*regexp.Regexp),
	}
}

// Load fetches all active routes from the database, compiles regex patterns,
// and stores them in memory under a write lock.
func (m *RouteMatcher) Load(ctx context.Context) error {
	routes, err := m.repo.LoadAllActive(ctx)
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
	m.regexCache = cache
	m.mu.Unlock()

	log.Info().Int("routes", len(routes)).Int("regex_patterns", len(cache)).Msg("route matcher loaded")
	return nil
}

// Invalidate reloads all routes from the database, logging any error.
func (m *RouteMatcher) Invalidate(ctx context.Context) {
	if err := m.Load(ctx); err != nil {
		log.Error().Err(err).Msg("route matcher invalidation failed")
	}
}

// Match returns all routes that match the given context, sorted by priority (ascending).
// It tries client-specific routes first; if none match, it falls back to default routes.
func (m *RouteMatcher) Match(ctx MatchContext) []*domain.ClientRoute {
	m.mu.RLock()
	routes := m.routes
	regexCache := m.regexCache
	m.mu.RUnlock()

	// Split routes by route_type, then by client-specific vs default.
	var clientRoutes, defaultRoutes []*domain.ClientRoute
	for _, r := range routes {
		if r.RouteType != ctx.RouteType {
			continue
		}
		if r.ClientID != nil && *r.ClientID == ctx.ClientID {
			clientRoutes = append(clientRoutes, r)
		} else if r.ClientID == nil {
			defaultRoutes = append(defaultRoutes, r)
		}
	}

	// Try client-specific first.
	matched := m.filterMatching(clientRoutes, ctx, regexCache)
	if len(matched) == 0 {
		matched = m.filterMatching(defaultRoutes, ctx, regexCache)
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Priority < matched[j].Priority
	})

	return matched
}

// MatchResult holds the outcome of route matching with diagnostic details.
type MatchResult struct {
	Matched       []*domain.ClientRoute
	ClientRoutes  int  // number of client-specific routes evaluated
	DefaultRoutes int  // number of default routes evaluated
	UsedDefault   bool // true if fell back to default routes
}

// MatchWithDetails returns matched routes plus diagnostic info for trace logging.
func (m *RouteMatcher) MatchWithDetails(ctx MatchContext) MatchResult {
	m.mu.RLock()
	routes := m.routes
	regexCache := m.regexCache
	m.mu.RUnlock()

	var clientRoutes, defaultRoutes []*domain.ClientRoute
	for _, r := range routes {
		if r.RouteType != ctx.RouteType {
			continue
		}
		if r.ClientID != nil && *r.ClientID == ctx.ClientID {
			clientRoutes = append(clientRoutes, r)
		} else if r.ClientID == nil {
			defaultRoutes = append(defaultRoutes, r)
		}
	}

	matched := m.filterMatching(clientRoutes, ctx, regexCache)
	usedDefault := false
	if len(matched) == 0 {
		matched = m.filterMatching(defaultRoutes, ctx, regexCache)
		usedDefault = true
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Priority < matched[j].Priority
	})

	return MatchResult{
		Matched:       matched,
		ClientRoutes:  len(clientRoutes),
		DefaultRoutes: len(defaultRoutes),
		UsedDefault:   usedDefault,
	}
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
func evaluateConditions(groups []domain.ConditionGroup, ctx MatchContext, regexCache map[string]*regexp.Regexp) bool {
	if len(groups) == 0 {
		return true
	}

	var result bool
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
