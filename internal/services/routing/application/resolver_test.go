package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/infrastructure"
)

// fakeStore implements RouteStore with in-memory data.
type fakeStore struct {
	routes      []*domain.ClientRoute
	clientInfo  map[uuid.UUID]infrastructure.ClientRoutingInfo
}

func (f *fakeStore) LoadAllActive(ctx context.Context) ([]*domain.ClientRoute, error) {
	return f.routes, nil
}

func (f *fakeStore) LoadClientRouting(ctx context.Context) (map[uuid.UUID]infrastructure.ClientRoutingInfo, error) {
	return f.clientInfo, nil
}

// routeAt builds a managed sms route owned by owner (nil = platform default).
func routeAt(owner *uuid.UUID, shared bool, providerID uuid.UUID) *domain.ClientRoute {
	return &domain.ClientRoute{
		ID:         uuid.New(),
		ClientID:   owner,
		ProviderID: providerID,
		Priority:   1,
		Weight:     0,
		Active:     true,
		Status:     domain.RouteStatusActive,
		RouteType:  "sms",
		Shared:     shared,
	}
}

func newTestMatcher(t *testing.T, routes []*domain.ClientRoute, info map[uuid.UUID]infrastructure.ClientRoutingInfo) *RouteMatcher {
	t.Helper()
	m := NewRouteMatcher(&fakeStore{routes: routes, clientInfo: info})
	if err := m.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return m
}

func smsCtx(client uuid.UUID) MatchContext {
	return MatchContext{
		RouteType:   "sms",
		ClientID:    client,
		TrafficType: domain.TrafficTypeTransactional,
	}
}

func TestResolve_Hybrid_ClientWinsOverResellerAndPlatform(t *testing.T) {
	client, parent := uuid.New(), uuid.New()
	clientProv, resellerProv, platformProv := uuid.New(), uuid.New(), uuid.New()
	m := newTestMatcher(t,
		[]*domain.ClientRoute{
			routeAt(&client, false, clientProv),
			routeAt(&parent, true, resellerProv),
			routeAt(nil, false, platformProv),
		},
		map[uuid.UUID]infrastructure.ClientRoutingInfo{
			client: {ParentID: &parent, Mode: "hybrid"},
		})

	dec, err := m.Resolve(context.Background(), smsCtx(client))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if dec.Level != LevelClient {
		t.Errorf("level = %s, want %s", dec.Level, LevelClient)
	}
	if dec.Route.ProviderID != clientProv {
		t.Errorf("provider = %s, want client provider %s", dec.Route.ProviderID, clientProv)
	}
}

func TestResolve_Hybrid_FallsBackToResellerShared(t *testing.T) {
	client, parent := uuid.New(), uuid.New()
	resellerProv, platformProv := uuid.New(), uuid.New()
	m := newTestMatcher(t,
		[]*domain.ClientRoute{
			// parent-owned but NOT shared — must be ignored at level 2
			routeAt(&parent, false, uuid.New()),
			routeAt(&parent, true, resellerProv),
			routeAt(nil, false, platformProv),
		},
		map[uuid.UUID]infrastructure.ClientRoutingInfo{
			client: {ParentID: &parent, Mode: "hybrid"},
		})

	dec, err := m.Resolve(context.Background(), smsCtx(client))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if dec.Level != LevelReseller {
		t.Errorf("level = %s, want %s", dec.Level, LevelReseller)
	}
	if dec.Route.ProviderID != resellerProv {
		t.Errorf("provider = %s, want reseller provider %s", dec.Route.ProviderID, resellerProv)
	}
}

func TestResolve_Hybrid_FallsBackToPlatform(t *testing.T) {
	client, parent := uuid.New(), uuid.New()
	platformProv := uuid.New()
	m := newTestMatcher(t,
		[]*domain.ClientRoute{routeAt(nil, false, platformProv)},
		map[uuid.UUID]infrastructure.ClientRoutingInfo{
			client: {ParentID: &parent, Mode: "hybrid"},
		})

	dec, err := m.Resolve(context.Background(), smsCtx(client))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if dec.Level != LevelPlatform {
		t.Errorf("level = %s, want %s", dec.Level, LevelPlatform)
	}
	if dec.Route.ProviderID != platformProv {
		t.Errorf("provider = %s, want platform provider %s", dec.Route.ProviderID, platformProv)
	}
}

func TestResolve_Hybrid_NoParent_SkipsResellerLevel(t *testing.T) {
	client := uuid.New()
	resellerOfOther := uuid.New() // unrelated reseller owning a shared route
	platformProv := uuid.New()
	m := newTestMatcher(t,
		[]*domain.ClientRoute{
			routeAt(&resellerOfOther, true, uuid.New()),
			routeAt(nil, false, platformProv),
		},
		map[uuid.UUID]infrastructure.ClientRoutingInfo{
			client: {Mode: "hybrid"}, // no parent
		})

	dec, err := m.Resolve(context.Background(), smsCtx(client))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if dec.Level != LevelPlatform {
		t.Errorf("level = %s, want %s (unrelated shared routes must not match)", dec.Level, LevelPlatform)
	}
}

func TestResolve_LegacyMode_PlatformDefaultsOnly(t *testing.T) {
	client := uuid.New()
	clientProv, platformProv := uuid.New(), uuid.New()
	m := newTestMatcher(t,
		[]*domain.ClientRoute{
			routeAt(&client, false, clientProv),
			routeAt(nil, false, platformProv),
		},
		map[uuid.UUID]infrastructure.ClientRoutingInfo{
			client: {Mode: "legacy"},
		})

	dec, err := m.Resolve(context.Background(), smsCtx(client))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if dec.Level != LevelPlatform {
		t.Errorf("level = %s, want %s (legacy gate skips client routes)", dec.Level, LevelPlatform)
	}
}

func TestResolve_LegacyMode_NoPlatformRoute_ErrNoRoute(t *testing.T) {
	client := uuid.New()
	m := newTestMatcher(t,
		[]*domain.ClientRoute{routeAt(&client, false, uuid.New())},
		map[uuid.UUID]infrastructure.ClientRoutingInfo{
			client: {Mode: "legacy"},
		})

	_, err := m.Resolve(context.Background(), smsCtx(client))
	if !errors.Is(err, ErrNoRouteFound) {
		t.Fatalf("err = %v, want ErrNoRouteFound", err)
	}
}

func TestResolve_NewMode_ClientOnlyNoFallback(t *testing.T) {
	client := uuid.New()
	m := newTestMatcher(t,
		[]*domain.ClientRoute{routeAt(nil, false, uuid.New())}, // platform only
		map[uuid.UUID]infrastructure.ClientRoutingInfo{
			client: {Mode: "new"},
		})

	_, err := m.Resolve(context.Background(), smsCtx(client))
	if !errors.Is(err, ErrNoRouteFound) {
		t.Fatalf("err = %v, want ErrNoRouteFound (new mode must not fall back)", err)
	}
}

func TestResolve_UnknownMode_BehavesAsHybrid(t *testing.T) {
	client := uuid.New()
	platformProv := uuid.New()
	m := newTestMatcher(t,
		[]*domain.ClientRoute{routeAt(nil, false, platformProv)},
		map[uuid.UUID]infrastructure.ClientRoutingInfo{
			client: {Mode: "bogus"},
		})

	dec, err := m.Resolve(context.Background(), smsCtx(client))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if dec.Level != LevelPlatform {
		t.Errorf("level = %s, want %s", dec.Level, LevelPlatform)
	}
}

func TestResolve_UnknownClient_BehavesAsHybrid(t *testing.T) {
	client := uuid.New()
	platformProv := uuid.New()
	m := newTestMatcher(t,
		[]*domain.ClientRoute{routeAt(nil, false, platformProv)},
		map[uuid.UUID]infrastructure.ClientRoutingInfo{})

	dec, err := m.Resolve(context.Background(), smsCtx(client))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if dec.Level != LevelPlatform {
		t.Errorf("level = %s, want %s", dec.Level, LevelPlatform)
	}
}

func TestResolve_ConditionMismatch_FallsThroughToNextLevel(t *testing.T) {
	client := uuid.New()
	clientProv := uuid.New()
	otherOperator := uuid.New()
	m := newTestMatcher(t,
		[]*domain.ClientRoute{
			{
				ClientID: &client, ProviderID: clientProv, Priority: 1,
				Active: true, Status: domain.RouteStatusActive, RouteType: "sms",
				Groups: []domain.ConditionGroup{{
					LogicOp: domain.LogicIf,
					Conditions: []domain.Condition{
						{Type: domain.ConditionOperator, Value: otherOperator.String()},
					},
				}},
			},
			routeAt(nil, false, uuid.New()),
		},
		map[uuid.UUID]infrastructure.ClientRoutingInfo{client: {Mode: "hybrid"}})

	ctx := smsCtx(client)
	op := uuid.New()
	ctx.OperatorID = &op

	dec, err := m.Resolve(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if dec.Level != LevelPlatform {
		t.Errorf("level = %s, want %s (client route condition must filter it out)", dec.Level, LevelPlatform)
	}
}

func TestResolve_RouteTypeFilter(t *testing.T) {
	client := uuid.New()
	m := newTestMatcher(t,
		[]*domain.ClientRoute{
			{ClientID: &client, ProviderID: uuid.New(), Active: true,
				Status: domain.RouteStatusActive, RouteType: "hlr"},
		},
		map[uuid.UUID]infrastructure.ClientRoutingInfo{client: {Mode: "hybrid"}})

	_, err := m.Resolve(context.Background(), smsCtx(client)) // RouteType: sms
	if !errors.Is(err, ErrNoRouteFound) {
		t.Fatalf("err = %v, want ErrNoRouteFound (hlr route must not match sms context)", err)
	}
}

func TestResolve_NoRoutesAtAll_ErrNoRoute(t *testing.T) {
	client := uuid.New()
	m := newTestMatcher(t, nil, map[uuid.UUID]infrastructure.ClientRoutingInfo{client: {Mode: "hybrid"}})

	_, err := m.Resolve(context.Background(), smsCtx(client))
	if !errors.Is(err, ErrNoRouteFound) {
		t.Fatalf("err = %v, want ErrNoRouteFound", err)
	}
}
