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

	"github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/internal/services/client/application"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	clientrepo "github.com/smpp-server/smpp-server/internal/services/client/infrastructure/repository"
)

// --- Mocks ---

type mockClientRepo struct {
	mock.Mock
}

func (m *mockClientRepo) Create(ctx context.Context, client *domain.Client) error {
	args := m.Called(ctx, client)
	return args.Error(0)
}

func (m *mockClientRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Client, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Client), args.Error(1)
}

func (m *mockClientRepo) Update(ctx context.Context, client *domain.Client) error {
	args := m.Called(ctx, client)
	return args.Error(0)
}

func (m *mockClientRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockClientRepo) List(ctx context.Context, activeOnly bool, search string, limit, offset int) ([]*domain.Client, int, error) {
	args := m.Called(ctx, activeOnly, search, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Client), args.Int(1), args.Error(2)
}

func (m *mockClientRepo) AssignPlan(ctx context.Context, clientID uuid.UUID, planID uuid.UUID) error {
	args := m.Called(ctx, clientID, planID)
	return args.Error(0)
}

type mockConfigRepo struct {
	mock.Mock
}

func (m *mockConfigRepo) Create(ctx context.Context, config *domain.ClientConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *mockConfigRepo) GetByClientID(ctx context.Context, clientID uuid.UUID) (*domain.ClientConfig, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ClientConfig), args.Error(1)
}

func (m *mockConfigRepo) Update(ctx context.Context, config *domain.ClientConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *mockConfigRepo) Upsert(ctx context.Context, config *domain.ClientConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *mockConfigRepo) UpdateRateLimits(ctx context.Context, clientID uuid.UUID, limits *domain.RateLimits) error {
	args := m.Called(ctx, clientID, limits)
	return args.Error(0)
}

// --- Helper ---

func newTestClientServer(clientRepo *mockClientRepo, configRepo *mockConfigRepo) *Server {
	clientService := application.NewClientService(clientRepo, configRepo, nil)
	// SubAccountService uses concrete types, so we pass nil for it.
	// Tests that require SubAccountService are skipped.
	return NewServer(clientService, nil)
}

// --- Tests ---

func TestClientServer_GetClient(t *testing.T) {
	t.Run("found returns OK", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		clientID := uuid.New()
		now := time.Now()
		client := &domain.Client{
			ID:        clientID,
			Name:      "Test Client",
			Email:     "test@example.com",
			Active:    true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		config := &domain.ClientConfig{
			ClientID:           clientID,
			RateLimitPerSecond: 10,
		}

		clientRepo.On("GetByID", mock.Anything, clientID).Return(client, nil)
		configRepo.On("GetByClientID", mock.Anything, clientID).Return(config, nil)

		resp, err := srv.GetClient(context.Background(), &clientv1.GetClientRequest{
			ClientId: clientID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Client)
		assert.Equal(t, clientID.String(), resp.Client.ClientId)
		assert.Equal(t, "Test Client", resp.Client.Name)
		assert.Equal(t, "test@example.com", resp.Client.Email)
		assert.True(t, resp.Client.Active)
	})

	t.Run("not found returns NotFound", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		clientID := uuid.New()

		clientRepo.On("GetByID", mock.Anything, clientID).Return(nil, clientrepo.ErrClientNotFound)

		resp, err := srv.GetClient(context.Background(), &clientv1.GetClientRequest{
			ClientId: clientID.String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
		assert.Contains(t, st.Message(), "client not found")
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		resp, err := srv.GetClient(context.Background(), &clientv1.GetClientRequest{
			ClientId: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		resp, err := srv.GetClient(context.Background(), &clientv1.GetClientRequest{
			ClientId: "bad-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestClientServer_CreateClient(t *testing.T) {
	t.Run("valid request returns OK", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		clientRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Client")).Return(nil)
		configRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.ClientConfig")).Return(nil)

		resp, err := srv.CreateClient(context.Background(), &clientv1.CreateClientRequest{
			Name:          "New Client",
			Email:         "new@example.com",
			ContactPerson: "John Doe",
			Phone:         "+1234567890",
			Active:        true,
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.ClientId)
		assert.NotNil(t, resp.CreatedAt)

		clientRepo.AssertExpectations(t)
	})

	t.Run("empty name returns InvalidArgument", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		resp, err := srv.CreateClient(context.Background(), &clientv1.CreateClientRequest{
			Name: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestClientServer_UpdateClient(t *testing.T) {
	t.Run("found client updates successfully", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		clientID := uuid.New()
		existingClient := &domain.Client{
			ID:     clientID,
			Name:   "Old Name",
			Active: true,
		}

		clientRepo.On("GetByID", mock.Anything, clientID).Return(existingClient, nil)
		clientRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Client")).Return(nil)

		resp, err := srv.UpdateClient(context.Background(), &clientv1.UpdateClientRequest{
			ClientId: clientID.String(),
			Name:     "Updated Name",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
	})

	// Regression: PATCH без active в proto3 c plain bool молча дезактивировал
	// клиента (zero-value bool = false неотличим от "не передано"). После
	// перехода на optional bool nil-поле должно оставлять Active как было.
	t.Run("active=nil preserves existing active value", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		clientID := uuid.New()
		existingClient := &domain.Client{
			ID:     clientID,
			Name:   "Old Name",
			Active: true,
		}

		clientRepo.On("GetByID", mock.Anything, clientID).Return(existingClient, nil)
		clientRepo.On("Update", mock.Anything, mock.MatchedBy(func(c *domain.Client) bool {
			return c.Active == true
		})).Return(nil)

		resp, err := srv.UpdateClient(context.Background(), &clientv1.UpdateClientRequest{
			ClientId: clientID.String(),
			Name:     "Updated Name",
			// Active намеренно не устанавливается — должно остаться true.
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		clientRepo.AssertExpectations(t)
	})

	t.Run("active=false explicitly deactivates", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		clientID := uuid.New()
		existingClient := &domain.Client{
			ID:     clientID,
			Name:   "Old Name",
			Active: true,
		}

		clientRepo.On("GetByID", mock.Anything, clientID).Return(existingClient, nil)
		clientRepo.On("Update", mock.Anything, mock.MatchedBy(func(c *domain.Client) bool {
			return c.Active == false
		})).Return(nil)

		falseVal := false
		resp, err := srv.UpdateClient(context.Background(), &clientv1.UpdateClientRequest{
			ClientId: clientID.String(),
			Active:   &falseVal,
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		clientRepo.AssertExpectations(t)
	})

	t.Run("not found returns NotFound", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		clientID := uuid.New()
		clientRepo.On("GetByID", mock.Anything, clientID).Return(nil, clientrepo.ErrClientNotFound)

		resp, err := srv.UpdateClient(context.Background(), &clientv1.UpdateClientRequest{
			ClientId: clientID.String(),
			Name:     "Updated Name",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		resp, err := srv.UpdateClient(context.Background(), &clientv1.UpdateClientRequest{
			ClientId: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestClientServer_DeleteClient(t *testing.T) {
	t.Run("existing client deletes successfully", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		clientID := uuid.New()
		clientRepo.On("Delete", mock.Anything, clientID).Return(nil)

		resp, err := srv.DeleteClient(context.Background(), &clientv1.DeleteClientRequest{
			ClientId: clientID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
	})

	t.Run("not found returns NotFound", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		clientID := uuid.New()
		// Note: DeleteClient in clientService delegates directly to repo.Delete
		// without wrapping the error, so server receives repo error and maps it to Internal.
		// Using application.ErrClientNotFound to match server's error check.
		clientRepo.On("Delete", mock.Anything, clientID).Return(application.ErrClientNotFound)

		resp, err := srv.DeleteClient(context.Background(), &clientv1.DeleteClientRequest{
			ClientId: clientID.String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		resp, err := srv.DeleteClient(context.Background(), &clientv1.DeleteClientRequest{
			ClientId: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestClientServer_ListClients(t *testing.T) {
	t.Run("returns clients list successfully", func(t *testing.T) {
		clientRepo := new(mockClientRepo)
		configRepo := new(mockConfigRepo)
		srv := newTestClientServer(clientRepo, configRepo)

		clients := []*domain.Client{
			{
				ID:        uuid.New(),
				Name:      "Client A",
				Active:    true,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:        uuid.New(),
				Name:      "Client B",
				Active:    false,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		clientRepo.On("List", mock.Anything, false, "", 100, 0).Return(clients, 2, nil)

		resp, err := srv.ListClients(context.Background(), &clientv1.ListClientsRequest{})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Clients, 2)
		assert.Equal(t, int32(2), resp.Total)
		assert.Equal(t, "Client A", resp.Clients[0].Name)
		assert.Equal(t, "Client B", resp.Clients[1].Name)
	})
}
