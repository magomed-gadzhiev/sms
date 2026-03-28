package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/api/proto/providerv1"
	"github.com/smpp-server/smpp-server/internal/services/provider/application"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ─── Mock: ProviderRepository ─────────────────────────────────────────────────

type mockProviderRepo struct {
	mock.Mock
}

func (m *mockProviderRepo) Create(ctx context.Context, p *domain.Provider) error {
	return m.Called(ctx, p).Error(0)
}

func (m *mockProviderRepo) Update(ctx context.Context, p *domain.Provider) error {
	return m.Called(ctx, p).Error(0)
}

func (m *mockProviderRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Provider), args.Error(1)
}

func (m *mockProviderRepo) GetByName(ctx context.Context, name string) (*domain.Provider, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Provider), args.Error(1)
}

func (m *mockProviderRepo) List(ctx context.Context, activeOnly bool, limit, offset int) ([]*domain.Provider, int, error) {
	args := m.Called(ctx, activeOnly, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Provider), args.Int(1), args.Error(2)
}

func (m *mockProviderRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockProviderRepo) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Provider, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Provider), args.Error(1)
}

func (m *mockProviderRepo) CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error) {
	args := m.Called(ctx, clientID)
	return args.Int(0), args.Error(1)
}

func (m *mockProviderRepo) GetByIDAndClientID(ctx context.Context, id, clientID uuid.UUID) (*domain.Provider, error) {
	args := m.Called(ctx, id, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Provider), args.Error(1)
}

// ─── Mock: ConnectionPoolService ──────────────────────────────────────────────

type mockConnectionPool struct {
	mock.Mock
}

func (m *mockConnectionPool) Connect(ctx context.Context, p *domain.Provider) error {
	return m.Called(ctx, p).Error(0)
}

func (m *mockConnectionPool) Disconnect(providerID uuid.UUID) error {
	return m.Called(providerID).Error(0)
}

func (m *mockConnectionPool) GetConnection(providerID uuid.UUID) (application.Connection, error) {
	args := m.Called(providerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(application.Connection), args.Error(1)
}

func (m *mockConnectionPool) HealthCheck(providerID uuid.UUID) (int, int, error) {
	args := m.Called(providerID)
	return args.Int(0), args.Int(1), args.Error(2)
}

func (m *mockConnectionPool) CloseAll() error {
	return m.Called().Error(0)
}

// ─── Mock: SMSCSender (for SenderService) ────────────────────────────────────

type mockSMSCSender struct {
	mock.Mock
}

func (m *mockSMSCSender) SendMessage(ctx context.Context, msg *shared.Message, provider *shared.Provider) (string, error) {
	args := m.Called(ctx, msg, provider)
	return args.String(0), args.Error(1)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// validProviderData returns request fields for a valid provider.
var validCreateReq = &providerv1.CreateProviderRequest{
	Name:           "test-provider",
	Host:           "smpp.example.com",
	Port:           2775,
	SystemId:       "sysid",
	Password:       "secret",
	BindType:       3, // transceiver
	MaxConnections: 2,
	Active:         true,
}

func newTestServer(repo *mockProviderRepo, pool *mockConnectionPool, sender *mockSMSCSender) *Server {
	providerSvc := application.NewProviderService(repo)
	senderSvc := application.NewSenderService(sender)
	return NewServer(providerSvc, pool, senderSvc)
}

func newTestServerWithNilSender(repo *mockProviderRepo, pool *mockConnectionPool) *Server {
	providerSvc := application.NewProviderService(repo)
	return NewServer(providerSvc, pool, nil)
}

// makeStoredProvider builds a domain.Provider as if it were already in the DB.
func makeStoredProvider() *domain.Provider {
	return &domain.Provider{
		ID:             uuid.New(),
		Name:           "test-provider",
		Host:           "smpp.example.com",
		Port:           2775,
		SystemID:       "sysid",
		Password:       "secret",
		BindType:       domain.BindTypeTransceiver,
		MaxConnections: 2,
		Active:         true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// ─── CreateProvider ───────────────────────────────────────────────────────────

func TestServer_CreateProvider_Success(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	sender := new(mockSMSCSender)
	srv := newTestServer(repo, pool, sender)

	req := &providerv1.CreateProviderRequest{
		Name:           "new-provider",
		Host:           "smpp.example.com",
		Port:           2775,
		SystemId:       "sysid",
		Password:       "secret",
		BindType:       3,
		MaxConnections: 2,
		Active:         true,
	}

	repo.On("GetByName", mock.Anything, "new-provider").Return(nil, domain.ErrProviderNotFound)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Provider")).Return(nil)
	pool.On("Connect", mock.Anything, mock.AnythingOfType("*domain.Provider")).Return(nil)

	resp, err := srv.CreateProvider(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.ProviderId)
	assert.False(t, resp.CreatedAt.AsTime().IsZero())
	repo.AssertExpectations(t)
	pool.AssertExpectations(t)
}

func TestServer_CreateProvider_MissingName(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	req := &providerv1.CreateProviderRequest{
		Host:     "smpp.example.com",
		Port:     2775,
		SystemId: "sysid",
		Password: "secret",
	}

	resp, err := srv.CreateProvider(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_CreateProvider_MissingHost(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	req := &providerv1.CreateProviderRequest{
		Name:     "test",
		Port:     2775,
		SystemId: "sysid",
		Password: "secret",
	}

	resp, err := srv.CreateProvider(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_CreateProvider_InvalidPort_Zero(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	req := &providerv1.CreateProviderRequest{
		Name:     "test",
		Host:     "host",
		Port:     0,
		SystemId: "sysid",
		Password: "secret",
	}

	resp, err := srv.CreateProvider(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_CreateProvider_InvalidPort_TooHigh(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	req := &providerv1.CreateProviderRequest{
		Name:     "test",
		Host:     "host",
		Port:     70000,
		SystemId: "sysid",
		Password: "secret",
	}

	resp, err := srv.CreateProvider(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_CreateProvider_MissingSystemID(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	req := &providerv1.CreateProviderRequest{
		Name:     "test",
		Host:     "host",
		Port:     2775,
		Password: "secret",
	}

	resp, err := srv.CreateProvider(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_CreateProvider_MissingPassword(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	req := &providerv1.CreateProviderRequest{
		Name:     "test",
		Host:     "host",
		Port:     2775,
		SystemId: "sysid",
	}

	resp, err := srv.CreateProvider(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_CreateProvider_ServiceError(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	sender := new(mockSMSCSender)
	srv := newTestServer(repo, pool, sender)

	req := &providerv1.CreateProviderRequest{
		Name:     "fail-provider",
		Host:     "smpp.example.com",
		Port:     2775,
		SystemId: "sysid",
		Password: "secret",
		BindType: 3,
	}

	repo.On("GetByName", mock.Anything, "fail-provider").Return(nil, domain.ErrProviderNotFound)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Provider")).Return(errors.New("db error"))

	resp, err := srv.CreateProvider(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
	repo.AssertExpectations(t)
}

func TestServer_CreateProvider_InactiveNoConnect(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	sender := new(mockSMSCSender)
	srv := newTestServer(repo, pool, sender)

	req := &providerv1.CreateProviderRequest{
		Name:     "inactive-provider",
		Host:     "smpp.example.com",
		Port:     2775,
		SystemId: "sysid",
		Password: "secret",
		BindType: 3,
		Active:   false, // inactive: no Connect expected
	}

	repo.On("GetByName", mock.Anything, "inactive-provider").Return(nil, domain.ErrProviderNotFound)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Provider")).Return(nil)

	resp, err := srv.CreateProvider(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	// Connect must NOT have been called
	pool.AssertNotCalled(t, "Connect")
	repo.AssertExpectations(t)
}

// ─── GetProvider ──────────────────────────────────────────────────────────────

func TestServer_GetProvider_Found(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	stored := makeStoredProvider()
	repo.On("GetByID", mock.Anything, stored.ID).Return(stored, nil)

	resp, err := srv.GetProvider(context.Background(), &providerv1.GetProviderRequest{
		ProviderId: stored.ID.String(),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, stored.ID.String(), resp.Provider.ProviderId)
	assert.Equal(t, stored.Name, resp.Provider.Name)
	assert.Equal(t, stored.Host, resp.Provider.Host)
	repo.AssertExpectations(t)
}

func TestServer_GetProvider_NotFound(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, domain.ErrProviderNotFound)

	resp, err := srv.GetProvider(context.Background(), &providerv1.GetProviderRequest{
		ProviderId: id.String(),
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	repo.AssertExpectations(t)
}

func TestServer_GetProvider_InvalidID(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	resp, err := srv.GetProvider(context.Background(), &providerv1.GetProviderRequest{
		ProviderId: "not-a-uuid",
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_GetProvider_InternalError(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, errors.New("unexpected db error"))

	resp, err := srv.GetProvider(context.Background(), &providerv1.GetProviderRequest{
		ProviderId: id.String(),
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
	repo.AssertExpectations(t)
}

// ─── UpdateProvider ───────────────────────────────────────────────────────────

func TestServer_UpdateProvider_Success(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	sender := new(mockSMSCSender)
	srv := newTestServer(repo, pool, sender)

	stored := makeStoredProvider()

	repo.On("GetByID", mock.Anything, stored.ID).Return(stored, nil).Once()
	repo.On("Update", mock.Anything, stored).Return(nil)
	// After update (active=true), GetProvider is called to reconnect.
	repo.On("GetByID", mock.Anything, stored.ID).Return(stored, nil).Once()
	pool.On("Connect", mock.Anything, stored).Return(nil)

	resp, err := srv.UpdateProvider(context.Background(), &providerv1.UpdateProviderRequest{
		ProviderId:     stored.ID.String(),
		Name:           stored.Name,
		Host:           "new-host.example.com",
		Port:           2776,
		SystemId:       stored.SystemID,
		Password:       stored.Password,
		MaxConnections: 3,
		Active:         true,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	repo.AssertExpectations(t)
}

func TestServer_UpdateProvider_InvalidID(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	resp, err := srv.UpdateProvider(context.Background(), &providerv1.UpdateProviderRequest{
		ProviderId: "bad-uuid",
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_UpdateProvider_ServiceError(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	sender := new(mockSMSCSender)
	srv := newTestServer(repo, pool, sender)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, errors.New("db error"))

	resp, err := srv.UpdateProvider(context.Background(), &providerv1.UpdateProviderRequest{
		ProviderId: id.String(),
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
	repo.AssertExpectations(t)
}

// ─── ListProviders ────────────────────────────────────────────────────────────

func TestServer_ListProviders_Success(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	p1 := makeStoredProvider()
	p2 := makeStoredProvider()
	p2.ID = uuid.New()
	p2.Name = "second-provider"

	repo.On("List", mock.Anything, false, 10, 0).Return([]*domain.Provider{p1, p2}, 2, nil)

	resp, err := srv.ListProviders(context.Background(), &providerv1.ListProvidersRequest{
		Limit:      10,
		Offset:     0,
		ActiveOnly: false,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(2), resp.Total)
	assert.Len(t, resp.Providers, 2)
	repo.AssertExpectations(t)
}

func TestServer_ListProviders_DefaultLimit(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	// limit=0 should default to 100
	repo.On("List", mock.Anything, false, 100, 0).Return([]*domain.Provider{}, 0, nil)

	resp, err := srv.ListProviders(context.Background(), &providerv1.ListProvidersRequest{
		Limit:  0,
		Offset: 0,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(0), resp.Total)
	repo.AssertExpectations(t)
}

func TestServer_ListProviders_CapAt1000(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	repo.On("List", mock.Anything, false, 1000, 0).Return([]*domain.Provider{}, 0, nil)

	resp, err := srv.ListProviders(context.Background(), &providerv1.ListProvidersRequest{
		Limit:  9999,
		Offset: 0,
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	repo.AssertExpectations(t)
}

func TestServer_ListProviders_ActiveOnly(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	stored := makeStoredProvider()
	repo.On("List", mock.Anything, true, 50, 0).Return([]*domain.Provider{stored}, 1, nil)

	resp, err := srv.ListProviders(context.Background(), &providerv1.ListProvidersRequest{
		Limit:      50,
		Offset:     0,
		ActiveOnly: true,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Total)
	assert.Len(t, resp.Providers, 1)
	repo.AssertExpectations(t)
}

func TestServer_ListProviders_Error(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	repo.On("List", mock.Anything, false, 100, 0).Return(nil, 0, errors.New("db error"))

	resp, err := srv.ListProviders(context.Background(), &providerv1.ListProvidersRequest{})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
	repo.AssertExpectations(t)
}

// ─── DeleteProvider ───────────────────────────────────────────────────────────

func TestServer_DeleteProvider_Success(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	stored := makeStoredProvider()

	repo.On("GetByID", mock.Anything, stored.ID).Return(stored, nil)
	repo.On("Update", mock.Anything, stored).Return(nil)
	pool.On("Disconnect", stored.ID).Return(nil)

	resp, err := srv.DeleteProvider(context.Background(), &providerv1.DeleteProviderRequest{
		ProviderId: stored.ID.String(),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	repo.AssertExpectations(t)
	pool.AssertExpectations(t)
}

func TestServer_DeleteProvider_InvalidID(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	resp, err := srv.DeleteProvider(context.Background(), &providerv1.DeleteProviderRequest{
		ProviderId: "not-valid",
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_DeleteProvider_NotFound(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, domain.ErrProviderNotFound)

	resp, err := srv.DeleteProvider(context.Background(), &providerv1.DeleteProviderRequest{
		ProviderId: id.String(),
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
	repo.AssertExpectations(t)
}

func TestServer_DeleteProvider_ServiceError(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, errors.New("db error"))

	resp, err := srv.DeleteProvider(context.Background(), &providerv1.DeleteProviderRequest{
		ProviderId: id.String(),
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
	repo.AssertExpectations(t)
}

// ─── GetProviderHealth ────────────────────────────────────────────────────────

func TestServer_GetProviderHealth_Healthy(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	stored := makeStoredProvider()
	repo.On("GetByID", mock.Anything, stored.ID).Return(stored, nil)
	pool.On("HealthCheck", stored.ID).Return(2, 2, nil) // 2 active / 2 total → healthy

	resp, err := srv.GetProviderHealth(context.Background(), &providerv1.GetProviderHealthRequest{
		ProviderId: stored.ID.String(),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, stored.ID.String(), resp.ProviderId)
	assert.Equal(t, int32(2), resp.ActiveConnections)
	assert.Equal(t, int32(2), resp.TotalConnections)
	assert.Equal(t, int32(100), resp.SuccessRate)
	assert.Equal(t, string(domain.HealthStatusHealthy), resp.Status)
	repo.AssertExpectations(t)
	pool.AssertExpectations(t)
}

func TestServer_GetProviderHealth_Unhealthy(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	stored := makeStoredProvider()
	repo.On("GetByID", mock.Anything, stored.ID).Return(stored, nil)
	pool.On("HealthCheck", stored.ID).Return(0, 2, nil) // 0 active / 2 total → unhealthy

	resp, err := srv.GetProviderHealth(context.Background(), &providerv1.GetProviderHealthRequest{
		ProviderId: stored.ID.String(),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(0), resp.ActiveConnections)
	assert.Equal(t, string(domain.HealthStatusUnhealthy), resp.Status)
	repo.AssertExpectations(t)
	pool.AssertExpectations(t)
}

func TestServer_GetProviderHealth_InvalidID(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	resp, err := srv.GetProviderHealth(context.Background(), &providerv1.GetProviderHealthRequest{
		ProviderId: "bad",
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_GetProviderHealth_ProviderNotFound(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, domain.ErrProviderNotFound)

	resp, err := srv.GetProviderHealth(context.Background(), &providerv1.GetProviderHealthRequest{
		ProviderId: id.String(),
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
	repo.AssertExpectations(t)
}

func TestServer_GetProviderHealth_PoolError(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	srv := newTestServerWithNilSender(repo, pool)

	stored := makeStoredProvider()
	repo.On("GetByID", mock.Anything, stored.ID).Return(stored, nil)
	pool.On("HealthCheck", stored.ID).Return(0, 0, errors.New("pool error"))

	// Error from HealthCheck is swallowed (logged as warning), response still returns
	resp, err := srv.GetProviderHealth(context.Background(), &providerv1.GetProviderHealthRequest{
		ProviderId: stored.ID.String(),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(0), resp.ActiveConnections)
	assert.Equal(t, int32(0), resp.TotalConnections)
	repo.AssertExpectations(t)
	pool.AssertExpectations(t)
}

// ─── SendToProvider ───────────────────────────────────────────────────────────

func TestServer_SendToProvider_Success(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	sender := new(mockSMSCSender)
	srv := newTestServer(repo, pool, sender)

	stored := makeStoredProvider()
	repo.On("GetByID", mock.Anything, stored.ID).Return(stored, nil)
	sender.On("SendMessage", mock.Anything, mock.AnythingOfType("*shared.Message"), mock.AnythingOfType("*shared.Provider")).
		Return("smpp-msg-id-123", nil)

	resp, err := srv.SendToProvider(context.Background(), &providerv1.SendToProviderRequest{
		ProviderId:  stored.ID.String(),
		Source:      "sender",
		Destination: "+79001234567",
		Text:        "Hello",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	assert.Equal(t, "smpp-msg-id-123", resp.SmppMessageId)
	repo.AssertExpectations(t)
	sender.AssertExpectations(t)
}

func TestServer_SendToProvider_InvalidID(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	sender := new(mockSMSCSender)
	srv := newTestServer(repo, pool, sender)

	resp, err := srv.SendToProvider(context.Background(), &providerv1.SendToProviderRequest{
		ProviderId: "bad-uuid",
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_SendToProvider_ProviderNotFound(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	sender := new(mockSMSCSender)
	srv := newTestServer(repo, pool, sender)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, domain.ErrProviderNotFound)

	resp, err := srv.SendToProvider(context.Background(), &providerv1.SendToProviderRequest{
		ProviderId:  id.String(),
		Source:      "sender",
		Destination: "+79001234567",
		Text:        "Hello",
	})

	require.Error(t, err)
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
	repo.AssertExpectations(t)
}

func TestServer_SendToProvider_SenderError(t *testing.T) {
	repo := new(mockProviderRepo)
	pool := new(mockConnectionPool)
	sender := new(mockSMSCSender)
	srv := newTestServer(repo, pool, sender)

	stored := makeStoredProvider()
	repo.On("GetByID", mock.Anything, stored.ID).Return(stored, nil)
	sender.On("SendMessage", mock.Anything, mock.AnythingOfType("*shared.Message"), mock.AnythingOfType("*shared.Provider")).
		Return("", errors.New("smpp send failed"))

	resp, err := srv.SendToProvider(context.Background(), &providerv1.SendToProviderRequest{
		ProviderId:  stored.ID.String(),
		Source:      "sender",
		Destination: "+79001234567",
		Text:        "Hello",
	})

	// Sender errors do not become gRPC errors — the server returns Success=false.
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
	repo.AssertExpectations(t)
	sender.AssertExpectations(t)
}

// ─── domainToProto ────────────────────────────────────────────────────────────

func TestDomainToProto(t *testing.T) {
	now := time.Now()
	p := &domain.Provider{
		ID:             uuid.New(),
		Name:           "my-provider",
		Host:           "smpp.test.com",
		Port:           2775,
		SystemID:       "sys1",
		SystemType:     "CMT",
		BindType:       domain.BindTypeTransmitter,
		MaxConnections: 4,
		WindowSize:     10,
		Active:         true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	proto := domainToProto(p)

	assert.Equal(t, p.ID.String(), proto.ProviderId)
	assert.Equal(t, p.Name, proto.Name)
	assert.Equal(t, p.Host, proto.Host)
	assert.Equal(t, int32(p.Port), proto.Port)
	assert.Equal(t, p.SystemID, proto.SystemId)
	assert.Equal(t, p.SystemType, proto.SystemType)
	assert.Equal(t, int32(1), proto.BindType) // transmitter → 1
	assert.Equal(t, int32(p.MaxConnections), proto.MaxConnections)
	assert.Equal(t, int32(p.WindowSize), proto.WindowSize)
	assert.Equal(t, p.Active, proto.Active)
}
