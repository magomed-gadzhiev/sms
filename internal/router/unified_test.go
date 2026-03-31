package router_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/router"
	"github.com/smpp-server/smpp-server/internal/shared"
)

var (
	testClientID   = uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	testParentID   = uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")
	testOperatorID = uuid.MustParse("cccccccc-0000-0000-0000-000000000001")
	testProviderID = uuid.MustParse("dddddddd-0000-0000-0000-000000000001")
)

func makeRoute(cid *uuid.UUID, pid uuid.UUID, prio int, isShared bool) *shared.ClientRoute {
	return &shared.ClientRoute{
		ID: uuid.New(), ClientID: cid, OperatorID: testOperatorID,
		ProviderID: pid, Priority: prio, Weight: 1, Active: true, Shared: isShared,
	}
}

func TestUnifiedRouter_UsesOwnRoute(t *testing.T) {
	rt := makeRoute(&testClientID, testProviderID, 100, false)
	r := router.NewUnifiedRouter(mockRouteRepo{
		byClient: map[uuid.UUID][]*shared.ClientRoute{testClientID: {rt}},
	}, mockClientRepo{})

	dec, err := r.Route(context.Background(), testClientID, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, testProviderID, dec.ProviderID)
}

func TestUnifiedRouter_FallsBackToSharedParentRoute(t *testing.T) {
	sharedRoute := makeRoute(&testParentID, testProviderID, 100, true)
	r := router.NewUnifiedRouter(mockRouteRepo{
		sharedByClient: map[uuid.UUID][]*shared.ClientRoute{testParentID: {sharedRoute}},
	}, mockClientRepo{
		parentID: &testParentID,
	})

	dec, err := r.Route(context.Background(), testClientID, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, testProviderID, dec.ProviderID)
}

func TestUnifiedRouter_FallsBackToPlatformDefault(t *testing.T) {
	defaultRoute := makeRoute(nil, testProviderID, 50, false)
	r := router.NewUnifiedRouter(mockRouteRepo{
		defaults: map[uuid.UUID][]*shared.ClientRoute{testOperatorID: {defaultRoute}},
	}, mockClientRepo{})

	dec, err := r.Route(context.Background(), testClientID, testOperatorID)
	require.NoError(t, err)
	assert.Equal(t, testProviderID, dec.ProviderID)
}

func TestUnifiedRouter_ReturnsErrWhenNoRoutes(t *testing.T) {
	r := router.NewUnifiedRouter(mockRouteRepo{}, mockClientRepo{})
	_, err := r.Route(context.Background(), testClientID, testOperatorID)
	assert.ErrorIs(t, err, router.ErrNoRouteFound)
}

// --- mocks ---

type mockRouteRepo struct {
	byClient       map[uuid.UUID][]*shared.ClientRoute
	sharedByClient map[uuid.UUID][]*shared.ClientRoute
	defaults       map[uuid.UUID][]*shared.ClientRoute
}

func (m mockRouteRepo) ListByClientAndOperator(_ context.Context, cid, oid uuid.UUID) ([]*shared.ClientRoute, error) {
	return m.byClient[cid], nil
}
func (m mockRouteRepo) ListSharedByClientAndOperator(_ context.Context, cid, oid uuid.UUID) ([]*shared.ClientRoute, error) {
	return m.sharedByClient[cid], nil
}
func (m mockRouteRepo) ListDefaultByOperator(_ context.Context, oid uuid.UUID) ([]*shared.ClientRoute, error) {
	return m.defaults[oid], nil
}

type mockClientRepo struct{ parentID *uuid.UUID }

func (m mockClientRepo) GetParentClientID(_ context.Context, _ uuid.UUID) (*uuid.UUID, error) {
	return m.parentID, nil
}
