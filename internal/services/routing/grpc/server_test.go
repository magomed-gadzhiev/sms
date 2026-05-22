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

func (m *mockProviderRepo) GetHealthBatch(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*domain.ProviderHealth, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uuid.UUID]*domain.ProviderHealth), args.Error(1)
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

// newTestRoutingServerFull creates a server with explicit country/operator/prefix repos so tests
// that exercise those paths can set up expectations on the same instances.
func newTestRoutingServerFull(
	routeRepo *mockRouteRepo,
	providerRepo *mockProviderRepo,
	pub *mockRoutingEventPublisher,
	countryRepo *mockCountryRepo,
	operatorRepo *mockOperatorRepo,
	prefixRepo *mockPrefixRepo,
) *Server {
	routingService := application.NewRoutingService(routeRepo, providerRepo, pub)
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

// ==================== CreateRoute ====================

func TestRoutingServer_CreateRoute(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		providerID := uuid.New()
		routeRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Route")).Return(nil)

		resp, err := srv.CreateRoute(context.Background(), &routingv1.CreateRouteRequest{
			Name:        "Test Route",
			Pattern:     "+7",
			ProviderIds: []string{providerID.String()},
			Priority:    1,
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.RouteId)
	})

	t.Run("missing_name_returns_InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		resp, err := srv.CreateRoute(context.Background(), &routingv1.CreateRouteRequest{
			Pattern:     "+7",
			ProviderIds: []string{uuid.New().String()},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("missing_pattern_returns_InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		resp, err := srv.CreateRoute(context.Background(), &routingv1.CreateRouteRequest{
			Name:        "Test Route",
			ProviderIds: []string{uuid.New().String()},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("missing_provider_ids_returns_InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		resp, err := srv.CreateRoute(context.Background(), &routingv1.CreateRouteRequest{
			Name:        "Test Route",
			Pattern:     "+7",
			ProviderIds: []string{},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

// ==================== DeleteRoute ====================

func TestRoutingServer_DeleteRoute(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		routeID := uuid.New()
		routeRepo.On("Delete", mock.Anything, routeID).Return(nil)

		resp, err := srv.DeleteRoute(context.Background(), &routingv1.DeleteRouteRequest{
			RouteId: routeID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
	})

	t.Run("empty_route_id_returns_InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		resp, err := srv.DeleteRoute(context.Background(), &routingv1.DeleteRouteRequest{
			RouteId: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid_route_id_format_returns_InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		resp, err := srv.DeleteRoute(context.Background(), &routingv1.DeleteRouteRequest{
			RouteId: "not-a-valid-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

// ==================== ListRoutes ====================

func TestRoutingServer_ListRoutes(t *testing.T) {
	t.Run("success_with_defaults", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		providerID := uuid.New()
		routes := []*domain.Route{
			{
				ID:                  uuid.New(),
				Name:                "Route A",
				Pattern:             "+7",
				PatternType:         domain.PatternTypePrefix,
				ProviderIDs:         []uuid.UUID{providerID},
				Priority:            1,
				Active:              true,
				LoadBalanceStrategy: domain.LoadBalanceRoundRobin,
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			},
		}
		// Default limit is 100, offset is 0
		routeRepo.On("List", mock.Anything, false, 100, 0).Return(routes, 1, nil)

		resp, err := srv.ListRoutes(context.Background(), &routingv1.ListRoutesRequest{})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Routes, 1)
		assert.Equal(t, int32(1), resp.Total)
		assert.Equal(t, "Route A", resp.Routes[0].Name)
	})

	t.Run("success_with_custom_limit_and_offset", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		routeRepo.On("List", mock.Anything, true, 10, 5).Return([]*domain.Route{}, 0, nil)

		resp, err := srv.ListRoutes(context.Background(), &routingv1.ListRoutesRequest{
			Limit:      10,
			Offset:     5,
			ActiveOnly: true,
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Empty(t, resp.Routes)
		assert.Equal(t, int32(0), resp.Total)
	})
}

// ==================== UpdateRoute ====================

func TestRoutingServer_UpdateRoute(t *testing.T) {
	t.Run("empty_route_id_returns_InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		resp, err := srv.UpdateRoute(context.Background(), &routingv1.UpdateRouteRequest{
			RouteId: "",
			Name:    "Updated Name",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid_route_id_format_returns_InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		resp, err := srv.UpdateRoute(context.Background(), &routingv1.UpdateRouteRequest{
			RouteId: "bad-uuid-format",
			Name:    "Updated Name",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

// ==================== CreateCountry ====================

func TestRoutingServer_CreateCountry(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		countryRepo := new(mockCountryRepo)
		operatorRepo := new(mockOperatorRepo)
		prefixRepo := new(mockPrefixRepo)
		srv := newTestRoutingServerFull(routeRepo, providerRepo, pub, countryRepo, operatorRepo, prefixRepo)

		// GetByISOCode returns error meaning no existing country
		countryRepo.On("GetByISOCode", mock.Anything, "RU").Return(nil, domain.ErrCountryNotFound)
		countryRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Country")).Return(nil)

		resp, err := srv.CreateCountry(context.Background(), &routingv1.CreateCountryRequest{
			Name:      "Russia",
			IsoCode:   "RU",
			PhoneCode: "+7",
			Currency:  "RUB",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "Russia", resp.Name)
		assert.Equal(t, "RU", resp.IsoCode)
	})

	t.Run("missing_name_returns_InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		countryRepo := new(mockCountryRepo)
		operatorRepo := new(mockOperatorRepo)
		prefixRepo := new(mockPrefixRepo)
		srv := newTestRoutingServerFull(routeRepo, providerRepo, pub, countryRepo, operatorRepo, prefixRepo)

		resp, err := srv.CreateCountry(context.Background(), &routingv1.CreateCountryRequest{
			IsoCode:   "RU",
			PhoneCode: "+7",
			Currency:  "RUB",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("missing_iso_code_returns_InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		countryRepo := new(mockCountryRepo)
		operatorRepo := new(mockOperatorRepo)
		prefixRepo := new(mockPrefixRepo)
		srv := newTestRoutingServerFull(routeRepo, providerRepo, pub, countryRepo, operatorRepo, prefixRepo)

		resp, err := srv.CreateCountry(context.Background(), &routingv1.CreateCountryRequest{
			Name:      "Russia",
			PhoneCode: "+7",
			Currency:  "RUB",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

// ==================== ResolveOperator ====================

func TestRoutingServer_ResolveOperator(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		countryRepo := new(mockCountryRepo)
		operatorRepo := new(mockOperatorRepo)
		prefixRepo := new(mockPrefixRepo)
		srv := newTestRoutingServerFull(routeRepo, providerRepo, pub, countryRepo, operatorRepo, prefixRepo)

		operatorID := uuid.New()
		countryID := uuid.New()

		prefix := &domain.OperatorPrefix{
			ID:         uuid.New(),
			OperatorID: operatorID,
			Prefix:     "+7",
			Priority:   1,
		}
		operator := &domain.Operator{
			ID:        operatorID,
			CountryID: countryID,
			Name:      "MTS",
			Code:      "RU-MTS",
			Active:    true,
		}
		country := &domain.Country{
			ID:        countryID,
			Name:      "Russia",
			ISOCode:   "RU",
			PhoneCode: "+7",
			Currency:  "RUB",
		}

		prefixRepo.On("FindByNumber", mock.Anything, "+79001234567").Return(prefix, nil)
		operatorRepo.On("GetByID", mock.Anything, operatorID).Return(operator, nil)
		countryRepo.On("GetByID", mock.Anything, countryID).Return(country, nil)

		resp, err := srv.ResolveOperator(context.Background(), &routingv1.ResolveOperatorRequest{
			PhoneNumber: "+79001234567",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, operatorID.String(), resp.OperatorId)
		assert.Equal(t, countryID.String(), resp.CountryId)
		assert.Equal(t, "RU-MTS", resp.OperatorCode)
		assert.Equal(t, "RU", resp.CountryCode)
		assert.Equal(t, "prefix", resp.ResolvedBy)
	})

	t.Run("empty_phone_number_returns_InvalidArgument", func(t *testing.T) {
		routeRepo := new(mockRouteRepo)
		providerRepo := new(mockProviderRepo)
		pub := new(mockRoutingEventPublisher)
		srv := newTestRoutingServer(routeRepo, providerRepo, pub)

		resp, err := srv.ResolveOperator(context.Background(), &routingv1.ResolveOperatorRequest{
			PhoneNumber: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}
