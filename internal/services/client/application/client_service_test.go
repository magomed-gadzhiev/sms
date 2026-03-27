package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	"github.com/smpp-server/smpp-server/internal/services/client/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	clientrepo "github.com/smpp-server/smpp-server/internal/services/client/infrastructure/repository"
)

func TestClientService(t *testing.T) {
	t.Run("CreateClient", func(t *testing.T) {
		t.Run("creates_client_with_default_config", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()

			clientRepo.On("Create", ctx, mock.MatchedBy(func(c *domain.Client) bool {
				return c.Name == "Test Company" &&
					c.Email == "test@example.com" &&
					c.ContactPerson == "John Doe" &&
					c.Phone == "+79001234567" &&
					c.Active == true
			})).Return(nil)

			configRepo.On("Create", ctx, mock.MatchedBy(func(cfg *domain.ClientConfig) bool {
				return cfg.RateLimitPerSecond == 10 &&
					cfg.RateLimitPerMinute == 100 &&
					cfg.RateLimitPerHour == 1000 &&
					cfg.RateLimitPerDay == 10000
			})).Return(nil)

			client, err := svc.CreateClient(ctx, "Test Company", "test@example.com", "John Doe", "+79001234567", true, false, nil)

			require.NoError(t, err)
			assert.NotNil(t, client)
			assert.Equal(t, "Test Company", client.Name)
			assert.Equal(t, "test@example.com", client.Email)
			assert.Equal(t, "John Doe", client.ContactPerson)
			assert.Equal(t, "+79001234567", client.Phone)
			assert.True(t, client.Active)
			assert.NotNil(t, client.Config)
			assert.Equal(t, json.RawMessage("{}"), client.Metadata)
			clientRepo.AssertExpectations(t)
			configRepo.AssertExpectations(t)
		})

		t.Run("creates_client_with_metadata", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			metadata := map[string]string{"industry": "fintech", "tier": "premium"}

			clientRepo.On("Create", ctx, mock.MatchedBy(func(c *domain.Client) bool {
				m := c.GetMetadata()
				return m["industry"] == "fintech" && m["tier"] == "premium"
			})).Return(nil)

			configRepo.On("Create", ctx, mock.AnythingOfType("*domain.ClientConfig")).Return(nil)

			client, err := svc.CreateClient(ctx, "Fintech Corp", "info@fintech.com", "Jane", "+79009876543", true, false, metadata)

			require.NoError(t, err)
			assert.NotNil(t, client)
			assert.Equal(t, "Fintech Corp", client.Name)
			clientMetadata := client.GetMetadata()
			assert.Equal(t, "fintech", clientMetadata["industry"])
			assert.Equal(t, "premium", clientMetadata["tier"])
			clientRepo.AssertExpectations(t)
			configRepo.AssertExpectations(t)
		})

		t.Run("returns_error_on_empty_name", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()

			client, err := svc.CreateClient(ctx, "", "test@example.com", "John", "+79001234567", true, false, nil)

			require.Error(t, err)
			assert.Nil(t, client)
			assert.Equal(t, ErrInvalidClientData, err)
		})

		t.Run("returns_error_on_repo_failure", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()

			clientRepo.On("Create", ctx, mock.AnythingOfType("*domain.Client")).Return(assert.AnError)

			client, err := svc.CreateClient(ctx, "Test", "test@example.com", "John", "+79001234567", true, false, nil)

			require.Error(t, err)
			assert.Nil(t, client)
			clientRepo.AssertExpectations(t)
		})
	})

	t.Run("GetClient", func(t *testing.T) {
		t.Run("existing", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			expectedClient := &domain.Client{
				ID:            clientID,
				Name:          "Test Company",
				Email:         "test@example.com",
				ContactPerson: "John",
				Phone:         "+79001234567",
				Active:        true,
				Metadata:      json.RawMessage("{}"),
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}

			expectedConfig := &domain.ClientConfig{
				ID:                 uuid.New(),
				ClientID:           clientID,
				RateLimitPerSecond: 10,
				RateLimitPerMinute: 100,
				RateLimitPerHour:   1000,
				RateLimitPerDay:    10000,
				AllowedSources:     []string{},
				BlockedDestinations: []string{},
				Settings:           json.RawMessage("{}"),
			}

			clientRepo.On("GetByID", ctx, clientID).Return(expectedClient, nil)
			configRepo.On("GetByClientID", ctx, clientID).Return(expectedConfig, nil)

			client, err := svc.GetClient(ctx, clientID)

			require.NoError(t, err)
			assert.NotNil(t, client)
			assert.Equal(t, clientID, client.ID)
			assert.Equal(t, "Test Company", client.Name)
			assert.NotNil(t, client.Config)
			assert.Equal(t, 10, client.Config.RateLimitPerSecond)
			clientRepo.AssertExpectations(t)
			configRepo.AssertExpectations(t)
		})

		t.Run("not_found", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			clientRepo.On("GetByID", ctx, clientID).Return(nil, clientrepo.ErrClientNotFound)

			client, err := svc.GetClient(ctx, clientID)

			require.Error(t, err)
			assert.Nil(t, client)
			assert.Equal(t, ErrClientNotFound, err)
			clientRepo.AssertExpectations(t)
		})

		t.Run("existing_without_config", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			expectedClient := &domain.Client{
				ID:     clientID,
				Name:   "No Config Client",
				Active: true,
			}

			clientRepo.On("GetByID", ctx, clientID).Return(expectedClient, nil)
			configRepo.On("GetByClientID", ctx, clientID).Return(nil, clientrepo.ErrConfigNotFound)

			client, err := svc.GetClient(ctx, clientID)

			require.NoError(t, err)
			assert.NotNil(t, client)
			assert.Nil(t, client.Config)
			clientRepo.AssertExpectations(t)
			configRepo.AssertExpectations(t)
		})
	})

	t.Run("UpdateClient", func(t *testing.T) {
		t.Run("updates_fields", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			existingClient := &domain.Client{
				ID:            clientID,
				Name:          "Old Name",
				Email:         "old@example.com",
				ContactPerson: "Old Person",
				Phone:         "+79001111111",
				Active:        true,
				Metadata:      json.RawMessage("{}"),
				CreatedAt:     time.Now().Add(-time.Hour),
				UpdatedAt:     time.Now().Add(-time.Hour),
			}

			newName := "New Name"
			newEmail := "new@example.com"
			newActive := false

			clientRepo.On("GetByID", ctx, clientID).Return(existingClient, nil)
			clientRepo.On("Update", ctx, mock.MatchedBy(func(c *domain.Client) bool {
				return c.Name == "New Name" &&
					c.Email == "new@example.com" &&
					c.Active == false &&
					c.ContactPerson == "Old Person" &&
					c.Phone == "+79001111111"
			})).Return(nil)

			client, err := svc.UpdateClient(ctx, clientID, &newName, &newEmail, nil, nil, &newActive, nil)

			require.NoError(t, err)
			assert.NotNil(t, client)
			assert.Equal(t, "New Name", client.Name)
			assert.Equal(t, "new@example.com", client.Email)
			assert.False(t, client.Active)
			assert.Equal(t, "Old Person", client.ContactPerson)
			clientRepo.AssertExpectations(t)
		})

		t.Run("returns_not_found_error", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()
			name := "New Name"

			clientRepo.On("GetByID", ctx, clientID).Return(nil, clientrepo.ErrClientNotFound)

			client, err := svc.UpdateClient(ctx, clientID, &name, nil, nil, nil, nil, nil)

			require.Error(t, err)
			assert.Nil(t, client)
			assert.Equal(t, ErrClientNotFound, err)
			clientRepo.AssertExpectations(t)
		})

		t.Run("updates_metadata", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			existingClient := &domain.Client{
				ID:        clientID,
				Name:      "Test",
				Active:    true,
				Metadata:  json.RawMessage("{}"),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			newMetadata := map[string]string{"key": "value"}

			clientRepo.On("GetByID", ctx, clientID).Return(existingClient, nil)
			clientRepo.On("Update", ctx, mock.MatchedBy(func(c *domain.Client) bool {
				m := c.GetMetadata()
				return m["key"] == "value"
			})).Return(nil)

			client, err := svc.UpdateClient(ctx, clientID, nil, nil, nil, nil, nil, newMetadata)

			require.NoError(t, err)
			assert.NotNil(t, client)
			m := client.GetMetadata()
			assert.Equal(t, "value", m["key"])
			clientRepo.AssertExpectations(t)
		})

		t.Run("returns_error_on_repo_update_failure", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			existingClient := &domain.Client{
				ID:       clientID,
				Name:     "Test",
				Active:   true,
				Metadata: json.RawMessage("{}"),
			}

			newName := "Updated"

			clientRepo.On("GetByID", ctx, clientID).Return(existingClient, nil)
			clientRepo.On("Update", ctx, mock.AnythingOfType("*domain.Client")).Return(assert.AnError)

			client, err := svc.UpdateClient(ctx, clientID, &newName, nil, nil, nil, nil, nil)

			require.Error(t, err)
			assert.Nil(t, client)
			clientRepo.AssertExpectations(t)
		})

		t.Run("returns_error_on_generic_getbyid_failure", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()
			name := "X"

			clientRepo.On("GetByID", ctx, clientID).Return(nil, assert.AnError)

			client, err := svc.UpdateClient(ctx, clientID, &name, nil, nil, nil, nil, nil)

			require.Error(t, err)
			assert.Nil(t, client)
			assert.Equal(t, assert.AnError, err)
		})

		t.Run("updates_all_pointer_fields", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			existingClient := &domain.Client{
				ID:            clientID,
				Name:          "Old",
				Email:         "old@test.com",
				ContactPerson: "OldPerson",
				Phone:         "+70000000000",
				Active:        false,
				Metadata:      json.RawMessage("{}"),
			}

			newName := "New"
			newEmail := "new@test.com"
			newContact := "NewPerson"
			newPhone := "+71111111111"
			newActive := true

			clientRepo.On("GetByID", ctx, clientID).Return(existingClient, nil)
			clientRepo.On("Update", ctx, mock.MatchedBy(func(c *domain.Client) bool {
				return c.Name == "New" &&
					c.Email == "new@test.com" &&
					c.ContactPerson == "NewPerson" &&
					c.Phone == "+71111111111" &&
					c.Active == true
			})).Return(nil)

			client, err := svc.UpdateClient(ctx, clientID, &newName, &newEmail, &newContact, &newPhone, &newActive, nil)

			require.NoError(t, err)
			assert.Equal(t, "New", client.Name)
			assert.Equal(t, "new@test.com", client.Email)
			assert.Equal(t, "NewPerson", client.ContactPerson)
			assert.Equal(t, "+71111111111", client.Phone)
			assert.True(t, client.Active)
		})
	})

	t.Run("ListClients", func(t *testing.T) {
		t.Run("returns_clients_list", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()

			expectedClients := []*domain.Client{
				{ID: uuid.New(), Name: "Client A", Active: true},
				{ID: uuid.New(), Name: "Client B", Active: true},
			}

			clientRepo.On("List", ctx, true, "search", 10, 0).Return(expectedClients, 2, nil)

			clients, total, err := svc.ListClients(ctx, true, "search", 10, 0)

			require.NoError(t, err)
			assert.Len(t, clients, 2)
			assert.Equal(t, 2, total)
			assert.Equal(t, "Client A", clients[0].Name)
			assert.Equal(t, "Client B", clients[1].Name)
			clientRepo.AssertExpectations(t)
		})

		t.Run("returns_empty_list", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()

			clientRepo.On("List", ctx, false, "", 20, 0).Return([]*domain.Client{}, 0, nil)

			clients, total, err := svc.ListClients(ctx, false, "", 20, 0)

			require.NoError(t, err)
			assert.Empty(t, clients)
			assert.Equal(t, 0, total)
			clientRepo.AssertExpectations(t)
		})

		t.Run("returns_error_on_repo_failure", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()

			clientRepo.On("List", ctx, false, "", 10, 0).Return(nil, 0, assert.AnError)

			clients, total, err := svc.ListClients(ctx, false, "", 10, 0)

			require.Error(t, err)
			assert.Nil(t, clients)
			assert.Equal(t, 0, total)
			clientRepo.AssertExpectations(t)
		})
	})

	t.Run("DeleteClient", func(t *testing.T) {
		t.Run("deletes_successfully", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			clientRepo.On("Delete", ctx, clientID).Return(nil)

			err := svc.DeleteClient(ctx, clientID)

			require.NoError(t, err)
			clientRepo.AssertExpectations(t)
		})

		t.Run("returns_error_on_failure", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			clientRepo.On("Delete", ctx, clientID).Return(assert.AnError)

			err := svc.DeleteClient(ctx, clientID)

			require.Error(t, err)
			assert.Equal(t, assert.AnError, err)
			clientRepo.AssertExpectations(t)
		})
	})

	t.Run("GetClientConfig", func(t *testing.T) {
		t.Run("returns_config", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			expectedConfig := &domain.ClientConfig{
				ID:                 uuid.New(),
				ClientID:           clientID,
				RateLimitPerSecond: 10,
				RateLimitPerMinute: 100,
				RateLimitPerHour:   1000,
				RateLimitPerDay:    10000,
				Settings:           json.RawMessage("{}"),
			}

			configRepo.On("GetByClientID", ctx, clientID).Return(expectedConfig, nil)

			config, err := svc.GetClientConfig(ctx, clientID)

			require.NoError(t, err)
			assert.NotNil(t, config)
			assert.Equal(t, clientID, config.ClientID)
			assert.Equal(t, 10, config.RateLimitPerSecond)
			configRepo.AssertExpectations(t)
		})

		t.Run("returns_not_found", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			configRepo.On("GetByClientID", ctx, clientID).Return(nil, clientrepo.ErrConfigNotFound)

			config, err := svc.GetClientConfig(ctx, clientID)

			require.Error(t, err)
			assert.Nil(t, config)
			assert.Equal(t, ErrConfigNotFound, err)
			configRepo.AssertExpectations(t)
		})

		t.Run("returns_generic_error", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			configRepo.On("GetByClientID", ctx, clientID).Return(nil, assert.AnError)

			config, err := svc.GetClientConfig(ctx, clientID)

			require.Error(t, err)
			assert.Nil(t, config)
			assert.Equal(t, assert.AnError, err)
			configRepo.AssertExpectations(t)
		})
	})

	t.Run("UpdateClientConfig", func(t *testing.T) {
		t.Run("upserts_config_successfully", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			existingClient := &domain.Client{ID: clientID, Name: "Test", Active: true}
			newConfig := &domain.ClientConfig{
				RateLimitPerSecond: 50,
				RateLimitPerMinute: 500,
				Settings:           json.RawMessage("{}"),
			}

			clientRepo.On("GetByID", ctx, clientID).Return(existingClient, nil)
			configRepo.On("Upsert", ctx, mock.MatchedBy(func(cfg *domain.ClientConfig) bool {
				return cfg.ClientID == clientID && cfg.RateLimitPerSecond == 50
			})).Return(nil)

			err := svc.UpdateClientConfig(ctx, clientID, newConfig)

			require.NoError(t, err)
			assert.Equal(t, clientID, newConfig.ClientID)
			clientRepo.AssertExpectations(t)
			configRepo.AssertExpectations(t)
		})

		t.Run("returns_not_found_when_client_missing", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()
			newConfig := &domain.ClientConfig{RateLimitPerSecond: 50}

			clientRepo.On("GetByID", ctx, clientID).Return(nil, clientrepo.ErrClientNotFound)

			err := svc.UpdateClientConfig(ctx, clientID, newConfig)

			require.Error(t, err)
			assert.Equal(t, ErrClientNotFound, err)
			clientRepo.AssertExpectations(t)
		})

		t.Run("returns_generic_error_on_getbyid_failure", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()
			newConfig := &domain.ClientConfig{RateLimitPerSecond: 50}

			clientRepo.On("GetByID", ctx, clientID).Return(nil, assert.AnError)

			err := svc.UpdateClientConfig(ctx, clientID, newConfig)

			require.Error(t, err)
			assert.Equal(t, assert.AnError, err)
		})

		t.Run("returns_error_on_upsert_failure", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			existingClient := &domain.Client{ID: clientID, Name: "Test"}
			newConfig := &domain.ClientConfig{RateLimitPerSecond: 50}

			clientRepo.On("GetByID", ctx, clientID).Return(existingClient, nil)
			configRepo.On("Upsert", ctx, mock.AnythingOfType("*domain.ClientConfig")).Return(assert.AnError)

			err := svc.UpdateClientConfig(ctx, clientID, newConfig)

			require.Error(t, err)
			assert.Equal(t, assert.AnError, err)
		})
	})

	t.Run("UpdateClientRateLimits", func(t *testing.T) {
		t.Run("updates_existing_limits", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			existingClient := &domain.Client{ID: clientID, Name: "Test"}
			limits := &domain.RateLimits{
				PerSecond: 20,
				PerMinute: 200,
				PerHour:   2000,
				PerDay:    20000,
			}

			clientRepo.On("GetByID", ctx, clientID).Return(existingClient, nil)
			configRepo.On("UpdateRateLimits", ctx, clientID, limits).Return(nil)

			err := svc.UpdateClientRateLimits(ctx, clientID, limits)

			require.NoError(t, err)
			clientRepo.AssertExpectations(t)
			configRepo.AssertExpectations(t)
		})

		t.Run("creates_config_when_not_found", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			existingClient := &domain.Client{ID: clientID, Name: "Test"}
			limits := &domain.RateLimits{
				PerSecond: 20,
				PerMinute: 200,
				PerHour:   2000,
				PerDay:    20000,
			}

			clientRepo.On("GetByID", ctx, clientID).Return(existingClient, nil)
			configRepo.On("UpdateRateLimits", ctx, clientID, limits).Return(clientrepo.ErrConfigNotFound)
			configRepo.On("Create", ctx, mock.MatchedBy(func(cfg *domain.ClientConfig) bool {
				return cfg.ClientID == clientID &&
					cfg.RateLimitPerSecond == 20 &&
					cfg.RateLimitPerMinute == 200 &&
					cfg.RateLimitPerHour == 2000 &&
					cfg.RateLimitPerDay == 20000
			})).Return(nil)

			err := svc.UpdateClientRateLimits(ctx, clientID, limits)

			require.NoError(t, err)
			clientRepo.AssertExpectations(t)
			configRepo.AssertExpectations(t)
		})

		t.Run("returns_not_found_when_client_missing", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()
			limits := &domain.RateLimits{PerSecond: 10}

			clientRepo.On("GetByID", ctx, clientID).Return(nil, clientrepo.ErrClientNotFound)

			err := svc.UpdateClientRateLimits(ctx, clientID, limits)

			require.Error(t, err)
			assert.Equal(t, ErrClientNotFound, err)
		})

		t.Run("returns_generic_error_on_getbyid_failure", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()
			limits := &domain.RateLimits{PerSecond: 10}

			clientRepo.On("GetByID", ctx, clientID).Return(nil, assert.AnError)

			err := svc.UpdateClientRateLimits(ctx, clientID, limits)

			require.Error(t, err)
			assert.Equal(t, assert.AnError, err)
		})

		t.Run("returns_error_on_update_rate_limits_failure", func(t *testing.T) {
			clientRepo := new(mocks.MockClientRepository)
			configRepo := new(mocks.MockConfigRepository)
			svc := NewClientService(clientRepo, configRepo)

			ctx := context.Background()
			clientID := uuid.New()

			existingClient := &domain.Client{ID: clientID, Name: "Test"}
			limits := &domain.RateLimits{PerSecond: 10}

			clientRepo.On("GetByID", ctx, clientID).Return(existingClient, nil)
			configRepo.On("UpdateRateLimits", ctx, clientID, limits).Return(assert.AnError)

			err := svc.UpdateClientRateLimits(ctx, clientID, limits)

			require.Error(t, err)
			assert.Equal(t, assert.AnError, err)
		})
	})

	t.Run("GetClient_config_error", func(t *testing.T) {
		clientRepo := new(mocks.MockClientRepository)
		configRepo := new(mocks.MockConfigRepository)
		svc := NewClientService(clientRepo, configRepo)

		ctx := context.Background()
		clientID := uuid.New()

		existingClient := &domain.Client{ID: clientID, Name: "Test"}

		clientRepo.On("GetByID", ctx, clientID).Return(existingClient, nil)
		configRepo.On("GetByClientID", ctx, clientID).Return(nil, assert.AnError)

		client, err := svc.GetClient(ctx, clientID)

		require.Error(t, err)
		assert.Nil(t, client)
		assert.Equal(t, assert.AnError, err)
	})

	t.Run("GetClient_generic_getbyid_error", func(t *testing.T) {
		clientRepo := new(mocks.MockClientRepository)
		configRepo := new(mocks.MockConfigRepository)
		svc := NewClientService(clientRepo, configRepo)

		ctx := context.Background()
		clientID := uuid.New()

		clientRepo.On("GetByID", ctx, clientID).Return(nil, assert.AnError)

		client, err := svc.GetClient(ctx, clientID)

		require.Error(t, err)
		assert.Nil(t, client)
		assert.Equal(t, assert.AnError, err)
	})
}

// SubAccountService tests are in sub_account_service_test.go
