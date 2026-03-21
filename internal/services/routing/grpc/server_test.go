package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/services/routing/application"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// --- Mocks ---

type mockRouteRepo struct {
	mock.Mock
}

func (m *mockRouteRepo) Create(ctx context.Context, route *domain.Route) error {
	args := m.Called(ctx, route)
	return args.Error(0)
}

func (m *mockRouteRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Route, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Route), args.Error(1)
}

func (m *mockRouteRepo) Update(ctx context.Context, route *domain.Route) error {
	args := m.Called(ctx, route)
	return args.Error(0)
}

func (m *mockRouteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRouteRepo) List(ctx context.Context, activeOnly bool, limit, offset int) ([]*domain.Route, int, error) {
	args := m.Called(ctx, activeOnly, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Route), args.Int(1), args.Error(2)
}

func (m *mockRouteRepo) GetActiveByDestination(ctx context.Context, destination string) ([]*domain.Route, error) {
	args := m.Called(ctx, destination)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Route), args.Error(1)
}

type mockProviderRepo struct {
	mock.Mock
}

func (m *mockProviderRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderInfo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderInfo), args.Error(1)
}

func (m *mockProviderRepo) GetAllActive(ctx context.Context) ([]*domain.ProviderInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ProviderInfo), args.Error(1)
}

func (m *mockProviderRepo) GetHealth(ctx context.Context, id uuid.UUID) (*domain.ProviderHealth, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderHealth), args.Error(1)
}

type mockRoutingEventPublisher struct {
	mock.Mock
}

func (m *mockRoutingEventPublisher) PublishMessageRouted(ctx context.Context, messageID uuid.UUID, routeID uuid.UUID, providerID uuid.UUID) error {
	args := m.Called(ctx, messageID, routeID, providerID)
	return args.Error(0)
}

// mockCountryRepo is a minimal mock of domain.CountryRepository
type mockCountryRepo struct {
	mock.Mock
}

func (m *mockCountryRepo) Create(ctx context.Context, c *domain.Country) error {
	return m.Called(ctx, c).Error(0)
}
func (m *mockCountryRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Country, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.Country), a.Error(1)
}
func (m *mockCountryRepo) GetByISOCode(ctx context.Context, code string) (*domain.Country, error) {
	a := m.Called(ctx, code)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.Country), a.Error(1)
}
func (m *mockCountryRepo) Update(ctx context.Context, c *domain.Country) error {
	return m.Called(ctx, c).Error(0)
}
func (m *mockCountryRepo) List(ctx context.Context, limit, offset int) ([]*domain.Country, int, error) {
	a := m.Called(ctx, limit, offset)
	if a.Get(0) == nil {
		return nil, 0, a.Error(2)
	}
	return a.Get(0).([]*domain.Country), a.Int(1), a.Error(2)
}

type mockOperatorRepo struct {
	mock.Mock
}

func (m *mockOperatorRepo) Create(ctx context.Context, op *domain.Operator) error {
	return m.Called(ctx, op).Error(0)
}
func (m *mockOperatorRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Operator, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.Operator), a.Error(1)
}
func (m *mockOperatorRepo) GetByCode(ctx context.Context, code string) (*domain.Operator, error) {
	a := m.Called(ctx, code)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.Operator), a.Error(1)
}
func (m *mockOperatorRepo) Update(ctx context.Context, op *domain.Operator) error {
	return m.Called(ctx, op).Error(0)
}
func (m *mockOperatorRepo) List(ctx context.Context, countryID *uuid.UUID, activeOnly bool, limit, offset int) ([]*domain.Operator, int, error) {
	a := m.Called(ctx, countryID, activeOnly, limit, offset)
	if a.Get(0) == nil {
		return nil, 0, a.Error(2)
	}
	return a.Get(0).([]*domain.Operator), a.Int(1), a.Error(2)
}

type mockPrefixRepo struct {
	mock.Mock
}

func (m *mockPrefixRepo) Create(ctx context.Context, p *domain.OperatorPrefix) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockPrefixRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockPrefixRepo) ListByOperatorID(ctx context.Context, operatorID uuid.UUID) ([]*domain.OperatorPrefix, error) {
	a := m.Called(ctx, operatorID)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*domain.OperatorPrefix), a.Error(1)
}
func (m *mockPrefixRepo) FindByNumber(ctx context.Context, phoneNumber string) (*domain.OperatorPrefix, error) {
	a := m.Called(ctx, phoneNumber)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.OperatorPrefix), a.Error(1)
}

// --- Helper ---

func newTestRoutingServer(routeRepo *mockRouteRepo, providerRepo *mockProviderRepo, pub *mockRoutingEventPublisher) *Server {
	routingService := application.NewRoutingService(routeRepo, providerRepo, pub)

	countryRepo := new(mockCountryRepo)
	operatorRepo := new(mockOperatorRepo)
	prefixRepo := new(mockPrefixRepo)
	operatorResolver := application.NewOperatorResolver(prefixRepo, operatorRepo, countryRepo)

	return NewServer(routingService, countryRepo, operatorRepo, prefixRepo, operatorResolver)
}

// --- Tests ---

func TestRoutingServer_GetRoute(t *testing.T) {
	t.Run("matching route returns OK", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		providerID := uuid.New()
		route := &domain.Route{
			ID:                  uuid.New(),
			Name:                "Russia Route",
			Pattern:             "+7",
			PatternType:         domain.PatternTypePrefix,
			ProviderIDs:         []uuid.UUID{providerID},
			Priority:            1,
			Active:              true,
			LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}

		routeRepo.On("GetActiveByDestination", mock.Anything, "+79001234567").Return([]*domain.Route{route}, nil)

		resp, err := srv.GetRoute(context.Background(), &routingv1.GetRouteRequest{
			Destination: "+79001234567",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Route)
		assert.Equal(t, route.ID.String(), resp.Route.RouteId)
		assert.Equal(t, "Russia Route", resp.Route.Name)
	})

	t.Run("no matching route returns NotFound", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		routeRepo.On("GetActiveByDestination", mock.Anything, "+44123456789").Return([]*domain.Route{}, nil)

		resp, err := srv.GetRoute(context.Background(), &routingv1.GetRouteRequest{
			Destination: "+44123456789",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("empty destination returns InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		resp, err := srv.GetRoute(context.Background(), &routingv1.GetRouteRequest{
			Destination: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestRoutingServer_NumberLookup(t *testing.T) {
	t.Run("HLR service not configured returns Unavailable", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)
		// hlrService is nil by default

		resp, err := srv.NumberLookup(context.Background(), &routingv1.NumberLookupRequest{
			Msisdn:   "+79001234567",
			ClientId: uuid.New().String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unavailable, st.Code())
	})

	t.Run("empty msisdn returns InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)
		// Set a non-nil HLR service to pass the nil check
		srv.hlrService = &application.HLRService{}

		resp, err := srv.NumberLookup(context.Background(), &routingv1.NumberLookupRequest{
			Msisdn:   "",
			ClientId: uuid.New().String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)
		srv.hlrService = &application.HLRService{}

		resp, err := srv.NumberLookup(context.Background(), &routingv1.NumberLookupRequest{
			Msisdn:   "+79001234567",
			ClientId: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)
		srv.hlrService = &application.HLRService{}

		resp, err := srv.NumberLookup(context.Background(), &routingv1.NumberLookupRequest{
			Msisdn:   "+79001234567",
			ClientId: "not-valid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestRoutingServer_SelectProvider(t *testing.T) {
	t.Run("no route returns InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		resp, err := srv.SelectProvider(context.Background(), &routingv1.SelectProviderRequest{
			Route: nil,
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("route with valid provider returns OK", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		providerID := uuid.New()
		providerInfo := &domain.ProviderInfo{
			ID:     providerID,
			Name:   "Provider1",
			Active: true,
		}

		providerRepo.On("GetByID", mock.Anything, providerID).Return(providerInfo, nil)

		resp, err := srv.SelectProvider(context.Background(), &routingv1.SelectProviderRequest{
			Route: &routingv1.RouteInfo{
				RouteId:             uuid.New().String(),
				Name:                "Test Route",
				Pattern:             "+7",
				ProviderIds:         []string{providerID.String()},
				LoadBalanceStrategy: "round_robin",
			},
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, providerID.String(), resp.ProviderId)
	})
}
