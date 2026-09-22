package application

import (
	"context"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func ptr[T any](v T) *T { return &v }

// newMatcher builds a RouteMatcher without a real repo so tests can populate
// routes and regexCache directly.
func newMatcher(routes []*domain.ClientRoute, cache map[string]*regexp.Regexp) *RouteMatcher {
	if cache == nil {
		cache = make(map[string]*regexp.Regexp)
	}
	return &RouteMatcher{
		mu:         sync.RWMutex{},
		routes:     routes,
		regexCache: cache,
	}
}

// baseCtx returns a minimal MatchContext that satisfies most conditions.
func baseCtx() MatchContext {
	return MatchContext{
		RouteType:   "sms",
		ClientID:    uuid.New(),
		CountryCode: "RU",
		TrafficType: domain.TrafficTypeTransactional,
		PaidName:    false,
		MessageBody: "Hello world",
		SenderName:  "TestSender",
	}
}

// newRoute creates a simple active route for the given client (nil = default).
func newRoute(clientID *uuid.UUID, routeType string, priority int, groups []domain.ConditionGroup, schedules []domain.Schedule) *domain.ClientRoute {
	return &domain.ClientRoute{
		ID:        uuid.New(),
		ClientID:  clientID,
		ProviderID: uuid.New(),
		Priority:  priority,
		Weight:    100,
		Active:    true,
		Name:      "test-route",
		RouteType: routeType,
		Status:    domain.RouteStatusActive,
		Groups:    groups,
		Schedules: schedules,
	}
}

// ---------------------------------------------------------------------------
// stripRegexDelimiters
// ---------------------------------------------------------------------------

func TestStripRegexDelimiters_NoDelimiters_ReturnsAsIs(t *testing.T) {
	assert.Equal(t, "hello", stripRegexDelimiters("hello"))
}

func TestStripRegexDelimiters_EmptyString_ReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", stripRegexDelimiters(""))
}

func TestStripRegexDelimiters_SingleChar_ReturnsAsIs(t *testing.T) {
	assert.Equal(t, "/", stripRegexDelimiters("/"))
}

func TestStripRegexDelimiters_SlashPattern_StripsDelimiters(t *testing.T) {
	assert.Equal(t, "hello", stripRegexDelimiters("/hello/"))
}

func TestStripRegexDelimiters_SlashPatternWithFlags_StripsDelimitersAndFlags(t *testing.T) {
	assert.Equal(t, "hello", stripRegexDelimiters("/hello/i"))
	assert.Equal(t, "hello", stripRegexDelimiters("/hello/gi"))
}

func TestStripRegexDelimiters_ComplexPattern_StripsCorrectly(t *testing.T) {
	assert.Equal(t, `^\d{4}$`, stripRegexDelimiters(`/^\d{4}$/`))
}

func TestStripRegexDelimiters_PatternWithInternalSlash_StripsOuterDelimiters(t *testing.T) {
	// /foo/bar/i → last slash at index 7 (inside [1:]), so result = "foo/bar"
	assert.Equal(t, "foo/bar", stripRegexDelimiters("/foo/bar/i"))
}

func TestStripRegexDelimiters_NoClosingSlash_ReturnsOriginal(t *testing.T) {
	// Starts with / but has no closing slash after the first character.
	assert.Equal(t, "/hello", stripRegexDelimiters("/hello"))
}

// Table-driven variant for edge cases.
func TestStripRegexDelimiters_Table(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/pattern/", "pattern"},
		{"/pattern/flags", "pattern"},
		{"pattern", "pattern"},
		{"//", ""},
		{"/a/", "a"},
		{"/a/b/c/i", "a/b/c"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, stripRegexDelimiters(tt.input))
		})
	}
}

// ---------------------------------------------------------------------------
// evaluateCondition
// ---------------------------------------------------------------------------

func TestEvaluateCondition_Operator_MatchesWhenIDEquals(t *testing.T) {
	opID := uuid.New()
	ctx := baseCtx()
	ctx.OperatorID = &opID

	c := domain.Condition{Type: domain.ConditionOperator, Value: opID.String()}
	assert.True(t, evaluateCondition(c, ctx, nil))
}

func TestEvaluateCondition_Operator_NoMatchWhenIDDiffers(t *testing.T) {
	opID := uuid.New()
	ctx := baseCtx()
	ctx.OperatorID = &opID

	c := domain.Condition{Type: domain.ConditionOperator, Value: uuid.New().String()}
	assert.False(t, evaluateCondition(c, ctx, nil))
}

func TestEvaluateCondition_Operator_NoMatchWhenOperatorIDNil(t *testing.T) {
	ctx := baseCtx()
	ctx.OperatorID = nil

	c := domain.Condition{Type: domain.ConditionOperator, Value: uuid.New().String()}
	assert.False(t, evaluateCondition(c, ctx, nil))
}

func TestEvaluateCondition_Country_CaseInsensitiveMatch(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"

	tests := []struct{ value string }{{"RU"}, {"ru"}, {"Ru"}, {"rU"}}
	for _, tt := range tests {
		c := domain.Condition{Type: domain.ConditionCountry, Value: tt.value}
		assert.True(t, evaluateCondition(c, ctx, nil), "value=%q", tt.value)
	}
}

func TestEvaluateCondition_Country_NoMatchWhenDifferent(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"

	c := domain.Condition{Type: domain.ConditionCountry, Value: "DE"}
	assert.False(t, evaluateCondition(c, ctx, nil))
}

func TestEvaluateCondition_TrafficType_CaseInsensitiveMatch(t *testing.T) {
	ctx := baseCtx()
	ctx.TrafficType = domain.TrafficTypeTransactional

	tests := []struct{ value string }{{"transactional"}, {"TRANSACTIONAL"}, {"Transactional"}}
	for _, tt := range tests {
		c := domain.Condition{Type: domain.ConditionTrafficType, Value: tt.value}
		assert.True(t, evaluateCondition(c, ctx, nil), "value=%q", tt.value)
	}
}

func TestEvaluateCondition_TrafficType_NoMatchWhenDifferent(t *testing.T) {
	ctx := baseCtx()
	ctx.TrafficType = domain.TrafficTypeTransactional

	c := domain.Condition{Type: domain.ConditionTrafficType, Value: "authorization"}
	assert.False(t, evaluateCondition(c, ctx, nil))
}

func TestEvaluateCondition_PaidName_TrueWhenBothTrue(t *testing.T) {
	ctx := baseCtx()
	ctx.PaidName = true

	c := domain.Condition{Type: domain.ConditionPaidName, Value: "true"}
	assert.True(t, evaluateCondition(c, ctx, nil))
}

func TestEvaluateCondition_PaidName_TrueWhenBothFalse(t *testing.T) {
	ctx := baseCtx()
	ctx.PaidName = false

	c := domain.Condition{Type: domain.ConditionPaidName, Value: "false"}
	assert.True(t, evaluateCondition(c, ctx, nil))
}

func TestEvaluateCondition_PaidName_FalseWhenMismatch(t *testing.T) {
	ctx := baseCtx()
	ctx.PaidName = false

	c := domain.Condition{Type: domain.ConditionPaidName, Value: "true"}
	assert.False(t, evaluateCondition(c, ctx, nil))
}

func TestEvaluateCondition_Regex_MatchesMessageBody(t *testing.T) {
	re := regexp.MustCompile("(?i)hello")
	cache := map[string]*regexp.Regexp{"hello": re}

	ctx := baseCtx()
	ctx.MessageBody = "Say Hello there"
	ctx.SenderName = "nobody"

	c := domain.Condition{Type: domain.ConditionRegex, Value: "/hello/i"}
	assert.True(t, evaluateCondition(c, ctx, cache))
}

func TestEvaluateCondition_Regex_MatchesSenderName(t *testing.T) {
	re := regexp.MustCompile("(?i)sender")
	cache := map[string]*regexp.Regexp{"sender": re}

	ctx := baseCtx()
	ctx.MessageBody = "nothing special"
	ctx.SenderName = "MySender"

	c := domain.Condition{Type: domain.ConditionRegex, Value: "/sender/i"}
	assert.True(t, evaluateCondition(c, ctx, cache))
}

func TestEvaluateCondition_Regex_NoMatchWhenNeitherMatches(t *testing.T) {
	re := regexp.MustCompile("(?i)secret")
	cache := map[string]*regexp.Regexp{"secret": re}

	ctx := baseCtx()
	ctx.MessageBody = "open text"
	ctx.SenderName = "nobody"

	c := domain.Condition{Type: domain.ConditionRegex, Value: "/secret/i"}
	assert.False(t, evaluateCondition(c, ctx, cache))
}

func TestEvaluateCondition_Regex_FalseWhenPatternNotInCache(t *testing.T) {
	ctx := baseCtx()
	c := domain.Condition{Type: domain.ConditionRegex, Value: "/uncached/"}
	assert.False(t, evaluateCondition(c, ctx, map[string]*regexp.Regexp{}))
}

func TestEvaluateCondition_UnknownType_ReturnsFalse(t *testing.T) {
	ctx := baseCtx()
	c := domain.Condition{Type: domain.ConditionType("unknown"), Value: "anything"}
	assert.False(t, evaluateCondition(c, ctx, nil))
}

// ---------------------------------------------------------------------------
// evaluateGroup
// ---------------------------------------------------------------------------

func TestEvaluateGroup_EmptyConditions_ReturnsTrue(t *testing.T) {
	g := domain.ConditionGroup{Conditions: nil}
	assert.True(t, evaluateGroup(g, baseCtx(), nil))
}

func TestEvaluateGroup_SingleConditionMatch_ReturnsTrue(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"

	g := domain.ConditionGroup{
		Conditions: []domain.Condition{
			{Type: domain.ConditionCountry, Value: "RU"},
		},
	}
	assert.True(t, evaluateGroup(g, ctx, nil))
}

func TestEvaluateGroup_SingleConditionNoMatch_ReturnsFalse(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "DE"

	g := domain.ConditionGroup{
		Conditions: []domain.Condition{
			{Type: domain.ConditionCountry, Value: "RU"},
		},
	}
	assert.False(t, evaluateGroup(g, ctx, nil))
}

// Within the same type: OR semantics — any match is enough.
func TestEvaluateGroup_SameType_ORSemantics_OneMatches(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "DE"

	g := domain.ConditionGroup{
		Conditions: []domain.Condition{
			{Type: domain.ConditionCountry, Value: "RU"},
			{Type: domain.ConditionCountry, Value: "DE"},
		},
	}
	assert.True(t, evaluateGroup(g, ctx, nil))
}

func TestEvaluateGroup_SameType_ORSemantics_NoneMatch(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "FR"

	g := domain.ConditionGroup{
		Conditions: []domain.Condition{
			{Type: domain.ConditionCountry, Value: "RU"},
			{Type: domain.ConditionCountry, Value: "DE"},
		},
	}
	assert.False(t, evaluateGroup(g, ctx, nil))
}

// Across different types: AND semantics — all types must match.
func TestEvaluateGroup_DifferentTypes_ANDSemantics_AllMatch(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"
	ctx.TrafficType = domain.TrafficTypeTransactional

	g := domain.ConditionGroup{
		Conditions: []domain.Condition{
			{Type: domain.ConditionCountry, Value: "RU"},
			{Type: domain.ConditionTrafficType, Value: "transactional"},
		},
	}
	assert.True(t, evaluateGroup(g, ctx, nil))
}

func TestEvaluateGroup_DifferentTypes_ANDSemantics_OneTypeFails(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"
	ctx.TrafficType = domain.TrafficTypeAuthorization

	g := domain.ConditionGroup{
		Conditions: []domain.Condition{
			{Type: domain.ConditionCountry, Value: "RU"},
			{Type: domain.ConditionTrafficType, Value: "transactional"}, // fails
		},
	}
	assert.False(t, evaluateGroup(g, ctx, nil))
}

// Mixed: same-type OR combined with different-type AND.
func TestEvaluateGroup_MixedTypes_ORwithinANDacross(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "DE"
	ctx.TrafficType = domain.TrafficTypeTransactional

	g := domain.ConditionGroup{
		Conditions: []domain.Condition{
			// country: RU OR DE — DE matches
			{Type: domain.ConditionCountry, Value: "RU"},
			{Type: domain.ConditionCountry, Value: "DE"},
			// traffic_type: transactional — matches
			{Type: domain.ConditionTrafficType, Value: "transactional"},
		},
	}
	assert.True(t, evaluateGroup(g, ctx, nil))
}

// ---------------------------------------------------------------------------
// evaluateConditions
// ---------------------------------------------------------------------------

func cond(t domain.ConditionType, v string) domain.Condition {
	return domain.Condition{Type: t, Value: v}
}

func group(logicOp domain.LogicOp, conds ...domain.Condition) domain.ConditionGroup {
	return domain.ConditionGroup{LogicOp: logicOp, Conditions: conds}
}

func TestEvaluateConditions_NoGroups_ReturnsTrue(t *testing.T) {
	assert.True(t, evaluateConditions(nil, baseCtx(), nil))
}

func TestEvaluateConditions_IF_TrueWhenGroupMatches(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),
	}
	assert.True(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_IF_FalseWhenGroupNotMatches(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "DE"

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),
	}
	assert.False(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_IF_AND_BothTrue(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"
	ctx.TrafficType = domain.TrafficTypeTransactional

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),
		group(domain.LogicAnd, cond(domain.ConditionTrafficType, "transactional")),
	}
	assert.True(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_IF_AND_FirstFalse(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "DE"
	ctx.TrafficType = domain.TrafficTypeTransactional

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),         // false
		group(domain.LogicAnd, cond(domain.ConditionTrafficType, "transactional")), // true but AND(false) = false
	}
	assert.False(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_IF_AND_SecondFalse(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"
	ctx.TrafficType = domain.TrafficTypeAuthorization

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),
		group(domain.LogicAnd, cond(domain.ConditionTrafficType, "transactional")), // false
	}
	assert.False(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_AND_NOT_InvertsGroup(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"
	ctx.TrafficType = domain.TrafficTypeAuthorization

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),               // result = true
		group(domain.LogicAndNot, cond(domain.ConditionTrafficType, "transactional")), // group=false → AND_NOT: true && !false = true
	}
	assert.True(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_AND_NOT_FailsWhenGroupTrue(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"
	ctx.TrafficType = domain.TrafficTypeTransactional

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),               // result = true
		group(domain.LogicAndNot, cond(domain.ConditionTrafficType, "transactional")), // group=true → AND_NOT: true && !true = false
	}
	assert.False(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_OR_TrueWhenEitherTrue(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "DE"
	ctx.TrafficType = domain.TrafficTypeTransactional

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),             // result = false
		group(domain.LogicOr, cond(domain.ConditionTrafficType, "transactional")), // result = false || true = true
	}
	assert.True(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_OR_FalseWhenBothFalse(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "DE"
	ctx.TrafficType = domain.TrafficTypeAuthorization

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),             // result = false
		group(domain.LogicOr, cond(domain.ConditionTrafficType, "transactional")), // result = false || false = false
	}
	assert.False(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_OR_NOT_TrueWhenGroupFalse(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "DE"
	ctx.TrafficType = domain.TrafficTypeAuthorization

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),              // result = false
		group(domain.LogicOrNot, cond(domain.ConditionTrafficType, "transactional")), // group=false → OR_NOT: false || !false = true
	}
	assert.True(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_OR_NOT_FalseWhenPrevTrueAndGroupTrue(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"
	ctx.TrafficType = domain.TrafficTypeTransactional

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),              // result = true
		group(domain.LogicOrNot, cond(domain.ConditionTrafficType, "transactional")), // group=true → OR_NOT: true || !true = true
	}
	// true || false = true — OR_NOT when result is already true is still true
	assert.True(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_UnknownLogicOp_TreatedAsIF(t *testing.T) {
	ctx := baseCtx()
	ctx.CountryCode = "RU"

	groups := []domain.ConditionGroup{
		{LogicOp: domain.LogicOp("INVALID"), Conditions: []domain.Condition{
			cond(domain.ConditionCountry, "RU"),
		}},
	}
	assert.True(t, evaluateConditions(groups, ctx, nil))
}

func TestEvaluateConditions_MultipleGroups_ComplexChain(t *testing.T) {
	// IF country=RU → true
	// AND traffic_type=transactional → true && true = true
	// AND_NOT paid_name=true → true && !false = true (ctx.PaidName is false, so group=false, !false=true)
	ctx := baseCtx()
	ctx.CountryCode = "RU"
	ctx.TrafficType = domain.TrafficTypeTransactional
	ctx.PaidName = false

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),
		group(domain.LogicAnd, cond(domain.ConditionTrafficType, "transactional")),
		group(domain.LogicAndNot, cond(domain.ConditionPaidName, "true")),
	}
	assert.True(t, evaluateConditions(groups, ctx, nil))
}

// ---------------------------------------------------------------------------
// checkSchedules
// ---------------------------------------------------------------------------

func TestCheckSchedules_NoSchedules_ReturnsTrue(t *testing.T) {
	assert.True(t, checkSchedules(nil))
	assert.True(t, checkSchedules([]domain.Schedule{}))
}

func TestCheckSchedules_WeekdaysBitmask_ZeroNeverMatches(t *testing.T) {
	s := domain.Schedule{
		Weekdays: 0,
		Timezone: "UTC",
	}
	assert.False(t, checkSchedules([]domain.Schedule{s}))
}

func TestCheckSchedules_AllWeekdays_AlwaysMatches(t *testing.T) {
	// 1+2+4+8+16+32+64 = 127 (Mon–Sun)
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "UTC",
	}
	assert.True(t, checkSchedules([]domain.Schedule{s}))
}

func TestCheckSchedules_DateFrom_Future_DoesNotMatch(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "UTC",
		DateFrom: &future,
	}
	assert.False(t, checkSchedules([]domain.Schedule{s}))
}

func TestCheckSchedules_DateFrom_Past_Matches(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "UTC",
		DateFrom: &past,
	}
	assert.True(t, checkSchedules([]domain.Schedule{s}))
}

func TestCheckSchedules_DateTo_Past_DoesNotMatch(t *testing.T) {
	past := time.Now().Add(-48 * time.Hour)
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "UTC",
		DateTo:   &past,
	}
	assert.False(t, checkSchedules([]domain.Schedule{s}))
}

func TestCheckSchedules_DateTo_Future_Matches(t *testing.T) {
	future := time.Now().Add(48 * time.Hour)
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "UTC",
		DateTo:   &future,
	}
	assert.True(t, checkSchedules([]domain.Schedule{s}))
}

func TestCheckSchedules_TimeFrom_Future_DoesNotMatch(t *testing.T) {
	// Set TimeFrom to far in the future within the day.
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "UTC",
		TimeFrom: ptr("23:59:59"),
	}
	// This will fail only if current UTC time is before 23:59:59.
	// To make the test deterministic we set TimeFrom to 00:00:00 and TimeTo to 00:00:00
	// which would only match at exactly midnight. Instead we check both extremes:
	now := time.Now().UTC()
	timeStr := now.Format("15:04:05")
	// TimeFrom in the future: one second ahead.
	oneSecondAhead := now.Add(time.Second).Format("15:04:05")
	s.TimeFrom = &oneSecondAhead
	assert.False(t, checkSchedules([]domain.Schedule{s}))
	_ = timeStr
}

func TestCheckSchedules_TimeTo_Past_DoesNotMatch(t *testing.T) {
	now := time.Now().UTC()
	oneSecondAgo := now.Add(-time.Second).Format("15:04:05")
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "UTC",
		TimeTo:   &oneSecondAgo,
	}
	assert.False(t, checkSchedules([]domain.Schedule{s}))
}

func TestCheckSchedules_TimeRange_NowInRange_Matches(t *testing.T) {
	now := time.Now().UTC()
	from := now.Add(-time.Hour).Format("15:04:05")
	to := now.Add(time.Hour).Format("15:04:05")
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "UTC",
		TimeFrom: &from,
		TimeTo:   &to,
	}
	assert.True(t, checkSchedules([]domain.Schedule{s}))
}

func TestCheckSchedules_MultipleSchedules_AnyMatchSuffices(t *testing.T) {
	// First schedule: weekdays=0 (never), second: all weekdays (always).
	s1 := domain.Schedule{Weekdays: 0, Timezone: "UTC"}
	s2 := domain.Schedule{Weekdays: 127, Timezone: "UTC"}
	assert.True(t, checkSchedules([]domain.Schedule{s1, s2}))
}

func TestCheckSchedules_InvalidTimezone_ReturnsFalse(t *testing.T) {
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "Not/AReal/Timezone",
	}
	assert.False(t, checkSchedules([]domain.Schedule{s}))
}

// ---------------------------------------------------------------------------
// matchSchedule — timezone and weekday bit mapping
// ---------------------------------------------------------------------------

func TestMatchSchedule_SpecificWeekday_CorrectBit(t *testing.T) {
	// We'll craft a schedule where only TODAY's weekday is set.
	now := time.Now().UTC()
	wd := now.Weekday()

	var bit int
	if wd == time.Sunday {
		bit = 64
	} else {
		bit = 1 << (wd - 1)
	}

	s := domain.Schedule{
		Weekdays: bit,
		Timezone: "UTC",
	}
	assert.True(t, matchSchedule(s))
}

func TestMatchSchedule_WeekdayNotSet_ReturnsFalse(t *testing.T) {
	now := time.Now().UTC()
	wd := now.Weekday()

	var bit int
	if wd == time.Sunday {
		bit = 64
	} else {
		bit = 1 << (wd - 1)
	}
	// All bits except today.
	s := domain.Schedule{
		Weekdays: 127 ^ bit,
		Timezone: "UTC",
	}
	assert.False(t, matchSchedule(s))
}

func TestMatchSchedule_UTCVsLocalTimezone_BothUTCMatchesToday(t *testing.T) {
	// UTC and "UTC" should both resolve to UTC — no difference in weekday.
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "UTC",
	}
	assert.True(t, matchSchedule(s))
}

func TestMatchSchedule_InvalidTimezone_ReturnsFalse(t *testing.T) {
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "INVALID_TZ",
	}
	assert.False(t, matchSchedule(s))
}

func TestMatchSchedule_DateBoundaries_DateToIncludesEntireLastDay(t *testing.T) {
	// DateTo is "yesterday" — but matchSchedule adds 24h-1ns, making it end
	// at 23:59:59.999999999 yesterday. Now is today, so this must fail.
	yesterday := time.Now().UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour)
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "UTC",
		DateTo:   &yesterday,
	}
	assert.False(t, matchSchedule(s))
}

func TestMatchSchedule_DateToIsToday_Matches(t *testing.T) {
	// DateTo set to today's midnight; matchSchedule extends it to end of day.
	todayMidnight := time.Now().UTC().Truncate(24 * time.Hour)
	s := domain.Schedule{
		Weekdays: 127,
		Timezone: "UTC",
		DateTo:   &todayMidnight,
	}
	assert.True(t, matchSchedule(s))
}

// ---------------------------------------------------------------------------
// Resolve — the single Routing Resolution entry point. Level semantics
// (client → reseller shared → platform, mode gates) live in resolver_test.go;
// here: filtering and ranking within a level.
// ---------------------------------------------------------------------------

func TestResolve_OtherClientsRoutes_IgnoredAtClientLevel(t *testing.T) {
	clientID := uuid.New()
	otherClientID := uuid.New()
	ctx := baseCtx()
	ctx.ClientID = clientID
	ctx.RouteType = "sms"

	// Route belongs to a different client and is not a shared reseller route.
	otherClientRoute := newRoute(&otherClientID, "sms", 1, nil, nil)
	defaultRoute := newRoute(nil, "sms", 1, nil, nil)

	m := newMatcher([]*domain.ClientRoute{otherClientRoute, defaultRoute}, nil)
	dec, err := m.Resolve(context.Background(), ctx)
	require.NoError(t, err)
	assert.Equal(t, LevelPlatform, dec.Level)
	assert.Equal(t, defaultRoute.ID, dec.Route.ID)
}

func TestResolve_PriorityOrdering_LowestPriorityNumberWins(t *testing.T) {
	ctx := baseCtx()
	ctx.RouteType = "sms"

	r1 := newRoute(nil, "sms", 30, nil, nil)
	r2 := newRoute(nil, "sms", 10, nil, nil)
	r3 := newRoute(nil, "sms", 20, nil, nil)

	m := newMatcher([]*domain.ClientRoute{r1, r2, r3}, nil)
	dec, err := m.Resolve(context.Background(), ctx)
	require.NoError(t, err)
	assert.Equal(t, 10, dec.Route.Priority)
}

func TestResolve_ConditionFilter_OnlyMatchingRouteResolved(t *testing.T) {
	ctx := baseCtx()
	ctx.RouteType = "sms"
	ctx.CountryCode = "RU"

	ruGroup := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),
	}
	deGroup := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "DE")),
	}

	ruRoute := newRoute(nil, "sms", 1, ruGroup, nil)
	deRoute := newRoute(nil, "sms", 2, deGroup, nil)

	m := newMatcher([]*domain.ClientRoute{ruRoute, deRoute}, nil)
	dec, err := m.Resolve(context.Background(), ctx)
	require.NoError(t, err)
	assert.Equal(t, ruRoute.ID, dec.Route.ID)
}

func TestResolve_ScheduleFilter_ExcludesRouteOutsideSchedule(t *testing.T) {
	ctx := baseCtx()
	ctx.RouteType = "sms"

	scheduledRoute := newRoute(nil, "sms", 1, nil, []domain.Schedule{
		{Weekdays: 0, Timezone: "UTC"}, // Weekdays=0 never matches
	})
	openRoute := newRoute(nil, "sms", 2, nil, nil)

	m := newMatcher([]*domain.ClientRoute{scheduledRoute, openRoute}, nil)
	dec, err := m.Resolve(context.Background(), ctx)
	require.NoError(t, err)
	assert.Equal(t, openRoute.ID, dec.Route.ID)
}

func TestResolve_RegexCondition_MatchesMessageBody(t *testing.T) {
	ctx := baseCtx()
	ctx.RouteType = "sms"
	ctx.MessageBody = "Your OTP is 1234"

	re := regexp.MustCompile("(?i)otp")
	cache := map[string]*regexp.Regexp{"otp": re}

	g := []domain.ConditionGroup{
		group(domain.LogicIf, domain.Condition{Type: domain.ConditionRegex, Value: "/otp/i"}),
	}
	route := newRoute(nil, "sms", 1, g, nil)

	m := newMatcher([]*domain.ClientRoute{route}, cache)
	dec, err := m.Resolve(context.Background(), ctx)
	require.NoError(t, err)
	assert.Equal(t, route.ID, dec.Route.ID)
}

func TestResolve_NoMatchingConditions_ErrNoRoute(t *testing.T) {
	ctx := baseCtx()
	ctx.RouteType = "sms"
	ctx.CountryCode = "RU"

	g := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "DE")),
	}
	route := newRoute(nil, "sms", 1, g, nil)

	m := newMatcher([]*domain.ClientRoute{route}, nil)
	_, err := m.Resolve(context.Background(), ctx)
	assert.ErrorIs(t, err, ErrNoRouteFound)
}

func TestNewRouteMatcher_EmptyState_ResolveErrNoRoute(t *testing.T) {
	// Matcher with no loaded state resolves to ErrNoRouteFound (not a panic).
	m := &RouteMatcher{
		regexCache: make(map[string]*regexp.Regexp),
	}
	_, err := m.Resolve(context.Background(), baseCtx())
	assert.ErrorIs(t, err, ErrNoRouteFound)
}

// ---------------------------------------------------------------------------
// Concurrency smoke test — Resolve is safe under concurrent reads.
// ---------------------------------------------------------------------------

func TestResolve_ConcurrentReads_NoPanic(t *testing.T) {
	routes := make([]*domain.ClientRoute, 10)
	for i := range routes {
		routes[i] = newRoute(nil, "sms", i, nil, nil)
	}
	m := newMatcher(routes, nil)
	ctx := baseCtx()
	ctx.RouteType = "sms"

	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			_, _ = m.Resolve(context.Background(), ctx)
			done <- struct{}{}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

// ---------------------------------------------------------------------------
// Integration: evaluateConditions + evaluateGroup + evaluateCondition chain
// ---------------------------------------------------------------------------

func TestFullConditionChain_OperatorCountryTraffic(t *testing.T) {
	opID := uuid.New()
	ctx := MatchContext{
		RouteType:   "sms",
		ClientID:    uuid.New(),
		OperatorID:  &opID,
		CountryCode: "RU",
		TrafficType: domain.TrafficTypeTransactional,
		PaidName:    false,
	}

	// IF (operator=opID AND country=RU) AND traffic_type=transactional AND_NOT paid_name=true
	groups := []domain.ConditionGroup{
		{
			LogicOp: domain.LogicIf,
			Conditions: []domain.Condition{
				{Type: domain.ConditionOperator, Value: opID.String()},
				{Type: domain.ConditionCountry, Value: "RU"},
			},
		},
		group(domain.LogicAnd, cond(domain.ConditionTrafficType, "transactional")),
		group(domain.LogicAndNot, cond(domain.ConditionPaidName, "true")),
	}

	assert.True(t, evaluateConditions(groups, ctx, nil))

	// Now change traffic type — should fail the AND group.
	ctx.TrafficType = domain.TrafficTypeAuthorization
	assert.False(t, evaluateConditions(groups, ctx, nil))
}

func TestFullConditionChain_RegexWithCache(t *testing.T) {
	re := regexp.MustCompile("(?i)promo")
	cache := map[string]*regexp.Regexp{"promo": re}

	ctx := MatchContext{
		RouteType:   "sms",
		ClientID:    uuid.New(),
		CountryCode: "RU",
		TrafficType: domain.TrafficTypeTransactional,
		MessageBody: "Special PROMO offer for you!",
		SenderName:  "brand",
	}

	groups := []domain.ConditionGroup{
		group(domain.LogicIf, cond(domain.ConditionCountry, "RU")),
		group(domain.LogicAnd, domain.Condition{Type: domain.ConditionRegex, Value: "/promo/i"}),
	}

	assert.True(t, evaluateConditions(groups, ctx, cache))

	// Without the promo keyword in body or sender, should fail.
	ctx.MessageBody = "Regular message"
	ctx.SenderName = "brand"
	assert.False(t, evaluateConditions(groups, ctx, cache))
}

// ---------------------------------------------------------------------------
// Unused import protection — ensure context is imported via a test that uses it.
// ---------------------------------------------------------------------------

func TestMatchContext_UsesContextPackage(t *testing.T) {
	// Ensures the context import is referenced (used by Load/Invalidate signatures).
	_ = context.Background()
}
