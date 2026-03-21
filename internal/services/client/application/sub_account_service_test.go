package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	clientrepo "github.com/smpp-server/smpp-server/internal/services/client/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/services/client/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type testSubAccountEnv struct {
	svc            *SubAccountService
	clientRepo     *mocks.MockClientRepository
	subAccountRepo *mocks.MockSubAccountRepository
	configRepo     *mocks.MockConfigRepository
}

func newTestSubAccountService() *testSubAccountEnv {
	clientRepo := new(mocks.MockClientRepository)
	subAccountRepo := new(mocks.MockSubAccountRepository)
	configRepo := new(mocks.MockConfigRepository)
	svc := NewSubAccountService(clientRepo, subAccountRepo, configRepo)
	return &testSubAccountEnv{
		svc:            svc,
		clientRepo:     clientRepo,
		subAccountRepo: subAccountRepo,
		configRepo:     configRepo,
	}
}

func TestSubAccountService(t *testing.T) {
	t.Run("CreateSubAccount", func(t *testing.T) {
		t.Run("creates_sub_account_with_limits", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()

			parent := &domain.Client{
				ID:             parentID,
				Name:           "Reseller Corp",
				IsReseller:     true,
				MaxSubAccounts: 10,
				Active:         true,
			}

			env.clientRepo.On("GetByID", ctx, parentID).Return(parent, nil)
			env.subAccountRepo.On("CountByParentID", ctx, parentID).Return(2, nil)
			env.clientRepo.On("Create", ctx, mock.MatchedBy(func(c *domain.Client) bool {
				return c.Name == "Sub Account" &&
					c.Email == "sub@example.com" &&
					c.ContactPerson == "Contact" &&
					c.Active == true &&
					c.ParentClientID != nil &&
					*c.ParentClientID == parentID &&
					!c.IsReseller &&
					c.MaxSubAccounts == 0
			})).Return(nil)
			env.configRepo.On("Create", ctx, mock.MatchedBy(func(cfg *domain.ClientConfig) bool {
				return cfg.RateLimitPerDay == 5000 &&
					cfg.RateLimitPerSecond == 10 &&
					cfg.RateLimitPerMinute == 100 &&
					cfg.RateLimitPerHour == 1000
			})).Return(nil)

			result, err := env.svc.CreateSubAccount(ctx, parentID, "Sub Account", "sub@example.com", "Contact", 5000, 0)

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "Sub Account", result.Name)
			assert.Equal(t, "sub@example.com", result.Email)
			assert.Equal(t, "Contact", result.ContactPerson)
			assert.True(t, result.Active)
			assert.Equal(t, &parentID, result.ParentClientID)
			assert.NotNil(t, result.Config)
			assert.Equal(t, 5000, result.Config.RateLimitPerDay)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("creates_sub_account_with_monthly_limit_in_settings", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()

			parent := &domain.Client{
				ID:             parentID,
				IsReseller:     true,
				MaxSubAccounts: 10,
			}

			env.clientRepo.On("GetByID", ctx, parentID).Return(parent, nil)
			env.subAccountRepo.On("CountByParentID", ctx, parentID).Return(0, nil)
			env.clientRepo.On("Create", ctx, mock.AnythingOfType("*domain.Client")).Return(nil)
			env.configRepo.On("Create", ctx, mock.MatchedBy(func(cfg *domain.ClientConfig) bool {
				settings := cfg.GetSettings()
				return settings["monthly_limit"] == "50000"
			})).Return(nil)

			result, err := env.svc.CreateSubAccount(ctx, parentID, "Sub Monthly", "sub@test.com", "Person", 1000, 50000)

			require.NoError(t, err)
			assert.NotNil(t, result)
			settings := result.Config.GetSettings()
			assert.Equal(t, "50000", settings["monthly_limit"])
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("parent_not_found_returns_ErrClientNotFound", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()

			env.clientRepo.On("GetByID", ctx, parentID).Return(nil, clientrepo.ErrClientNotFound)

			result, err := env.svc.CreateSubAccount(ctx, parentID, "Sub", "sub@test.com", "Person", 1000, 0)

			assert.Nil(t, result)
			assert.Equal(t, ErrClientNotFound, err)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("parent_not_reseller_returns_ErrNotReseller", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()

			parent := &domain.Client{
				ID:         parentID,
				IsReseller: false,
			}

			env.clientRepo.On("GetByID", ctx, parentID).Return(parent, nil)

			result, err := env.svc.CreateSubAccount(ctx, parentID, "Sub", "sub@test.com", "Person", 1000, 0)

			assert.Nil(t, result)
			assert.Equal(t, ErrNotReseller, err)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("max_sub_accounts_reached_returns_ErrMaxSubAccounts", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()

			parent := &domain.Client{
				ID:             parentID,
				IsReseller:     true,
				MaxSubAccounts: 5,
			}

			env.clientRepo.On("GetByID", ctx, parentID).Return(parent, nil)
			env.subAccountRepo.On("CountByParentID", ctx, parentID).Return(5, nil)

			result, err := env.svc.CreateSubAccount(ctx, parentID, "Sub", "sub@test.com", "Person", 1000, 0)

			assert.Nil(t, result)
			assert.Equal(t, ErrMaxSubAccounts, err)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("empty_name_returns_ErrInvalidClientData", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()

			parent := &domain.Client{
				ID:             parentID,
				IsReseller:     true,
				MaxSubAccounts: 10,
			}

			env.clientRepo.On("GetByID", ctx, parentID).Return(parent, nil)
			env.subAccountRepo.On("CountByParentID", ctx, parentID).Return(0, nil)

			result, err := env.svc.CreateSubAccount(ctx, parentID, "", "sub@test.com", "Person", 1000, 0)

			assert.Nil(t, result)
			assert.Equal(t, ErrInvalidClientData, err)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("config_create_failure_logs_warning_but_returns_success", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()

			parent := &domain.Client{
				ID:             parentID,
				IsReseller:     true,
				MaxSubAccounts: 10,
			}

			env.clientRepo.On("GetByID", ctx, parentID).Return(parent, nil)
			env.subAccountRepo.On("CountByParentID", ctx, parentID).Return(0, nil)
			env.clientRepo.On("Create", ctx, mock.AnythingOfType("*domain.Client")).Return(nil)
			env.configRepo.On("Create", ctx, mock.AnythingOfType("*domain.ClientConfig")).Return(errors.New("db connection error"))

			result, err := env.svc.CreateSubAccount(ctx, parentID, "Sub Account", "sub@test.com", "Person", 1000, 0)

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, "Sub Account", result.Name)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})
	})

	t.Run("ListSubAccounts", func(t *testing.T) {
		t.Run("lists_sub_accounts_for_reseller", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()

			parent := &domain.Client{
				ID:         parentID,
				IsReseller: true,
			}

			subAccounts := []*domain.Client{
				{ID: uuid.New(), Name: "Sub 1", ParentClientID: &parentID},
				{ID: uuid.New(), Name: "Sub 2", ParentClientID: &parentID},
			}

			env.clientRepo.On("GetByID", ctx, parentID).Return(parent, nil)
			env.subAccountRepo.On("ListByParentID", ctx, parentID).Return(subAccounts, nil)

			result, err := env.svc.ListSubAccounts(ctx, parentID)

			require.NoError(t, err)
			assert.Len(t, result, 2)
			assert.Equal(t, "Sub 1", result[0].Name)
			assert.Equal(t, "Sub 2", result[1].Name)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("parent_not_found_returns_error", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()

			env.clientRepo.On("GetByID", ctx, parentID).Return(nil, clientrepo.ErrClientNotFound)

			result, err := env.svc.ListSubAccounts(ctx, parentID)

			assert.Nil(t, result)
			assert.Equal(t, ErrClientNotFound, err)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("parent_not_reseller_returns_error", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()

			parent := &domain.Client{
				ID:         parentID,
				IsReseller: false,
			}

			env.clientRepo.On("GetByID", ctx, parentID).Return(parent, nil)

			result, err := env.svc.ListSubAccounts(ctx, parentID)

			assert.Nil(t, result)
			assert.Equal(t, ErrNotReseller, err)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})
	})

	t.Run("GetSubAccount", func(t *testing.T) {
		t.Run("returns_sub_account_by_id", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()
			subID := uuid.New()

			subAccount := &domain.Client{
				ID:             subID,
				Name:           "Sub Account",
				ParentClientID: &parentID,
				Active:         true,
			}

			env.subAccountRepo.On("GetSubAccount", ctx, subID, parentID).Return(subAccount, nil)

			result, err := env.svc.GetSubAccount(ctx, subID, parentID)

			require.NoError(t, err)
			assert.Equal(t, subID, result.ID)
			assert.Equal(t, "Sub Account", result.Name)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("not_found_returns_ErrSubAccountNotFound", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()
			subID := uuid.New()

			env.subAccountRepo.On("GetSubAccount", ctx, subID, parentID).Return(nil, clientrepo.ErrClientNotFound)

			result, err := env.svc.GetSubAccount(ctx, subID, parentID)

			assert.Nil(t, result)
			assert.Equal(t, ErrSubAccountNotFound, err)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})
	})

	t.Run("DeleteSubAccount", func(t *testing.T) {
		t.Run("deletes_existing_sub_account", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()
			subID := uuid.New()

			subAccount := &domain.Client{
				ID:             subID,
				Name:           "Sub Account",
				ParentClientID: &parentID,
			}

			env.subAccountRepo.On("GetSubAccount", ctx, subID, parentID).Return(subAccount, nil)
			env.subAccountRepo.On("DeleteSubAccount", ctx, subID).Return(nil)

			err := env.svc.DeleteSubAccount(ctx, subID, parentID)

			require.NoError(t, err)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("not_found_returns_ErrSubAccountNotFound", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()
			subID := uuid.New()

			env.subAccountRepo.On("GetSubAccount", ctx, subID, parentID).Return(nil, clientrepo.ErrClientNotFound)

			err := env.svc.DeleteSubAccount(ctx, subID, parentID)

			assert.Equal(t, ErrSubAccountNotFound, err)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})
	})

	t.Run("UpdateSubAccountLimits", func(t *testing.T) {
		t.Run("updates_limits_with_existing_config", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()
			subID := uuid.New()

			subAccount := &domain.Client{
				ID:             subID,
				Name:           "Sub Account",
				ParentClientID: &parentID,
			}

			existingConfig := &domain.ClientConfig{
				ID:                  uuid.New(),
				ClientID:            subID,
				RateLimitPerSecond:  10,
				RateLimitPerMinute:  100,
				RateLimitPerHour:    1000,
				RateLimitPerDay:     3000,
				AllowedSources:      []string{"sender1"},
				BlockedDestinations: []string{},
				Settings:            json.RawMessage("{}"),
				CreatedAt:           time.Now().Add(-24 * time.Hour),
			}

			env.subAccountRepo.On("GetSubAccount", ctx, subID, parentID).Return(subAccount, nil)
			env.configRepo.On("GetByClientID", ctx, subID).Return(existingConfig, nil)
			env.configRepo.On("Upsert", ctx, mock.MatchedBy(func(cfg *domain.ClientConfig) bool {
				settings := cfg.GetSettings()
				return cfg.RateLimitPerDay == 8000 && settings["monthly_limit"] == "200000"
			})).Return(nil)

			result, err := env.svc.UpdateSubAccountLimits(ctx, subID, parentID, 8000, 200000)

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, subID, result.ID)
			assert.NotNil(t, result.Config)
			assert.Equal(t, 8000, result.Config.RateLimitPerDay)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("creates_config_if_not_found", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()
			subID := uuid.New()

			subAccount := &domain.Client{
				ID:             subID,
				Name:           "Sub Account",
				ParentClientID: &parentID,
			}

			env.subAccountRepo.On("GetSubAccount", ctx, subID, parentID).Return(subAccount, nil)
			env.configRepo.On("GetByClientID", ctx, subID).Return(nil, clientrepo.ErrConfigNotFound)
			env.configRepo.On("Upsert", ctx, mock.MatchedBy(func(cfg *domain.ClientConfig) bool {
				return cfg.ClientID == subID &&
					cfg.RateLimitPerDay == 5000 &&
					cfg.RateLimitPerSecond == 10 &&
					cfg.RateLimitPerMinute == 100 &&
					cfg.RateLimitPerHour == 1000
			})).Return(nil)

			result, err := env.svc.UpdateSubAccountLimits(ctx, subID, parentID, 5000, 0)

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotNil(t, result.Config)
			assert.Equal(t, 5000, result.Config.RateLimitPerDay)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})

		t.Run("not_found_sub_account_returns_error", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()
			subID := uuid.New()

			env.subAccountRepo.On("GetSubAccount", ctx, subID, parentID).Return(nil, clientrepo.ErrClientNotFound)

			result, err := env.svc.UpdateSubAccountLimits(ctx, subID, parentID, 5000, 10000)

			assert.Nil(t, result)
			assert.Equal(t, ErrSubAccountNotFound, err)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})
	})

	t.Run("CountByParentID", func(t *testing.T) {
		t.Run("returns_count", func(t *testing.T) {
			env := newTestSubAccountService()
			ctx := context.Background()
			parentID := uuid.New()

			env.subAccountRepo.On("CountByParentID", ctx, parentID).Return(7, nil)

			count, err := env.svc.CountByParentID(ctx, parentID)

			require.NoError(t, err)
			assert.Equal(t, 7, count)
			env.clientRepo.AssertExpectations(t)
			env.subAccountRepo.AssertExpectations(t)
			env.configRepo.AssertExpectations(t)
		})
	})
}
