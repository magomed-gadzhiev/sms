package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/smpp-server/smpp-server/internal/services/tarification/mocks"
)

func TestSenderService(t *testing.T) {
	t.Run("CreateRegistration", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			clientID := uuid.New()
			operatorID := uuid.New()

			regRepo.On("GetActiveByClientOperatorName", ctx, clientID, operatorID, "MySender").
				Return(nil, domain.ErrSenderRegistrationNotFound)
			regRepo.On("Create", ctx, mock.AnythingOfType("*domain.SenderRegistration")).
				Return(nil)

			reg, err := svc.CreateRegistration(ctx, clientID, operatorID, "MySender", domain.SenderTypePaid)

			require.NoError(t, err)
			require.NotNil(t, reg)
			assert.Equal(t, clientID, reg.ClientID)
			assert.Equal(t, operatorID, reg.OperatorID)
			assert.Equal(t, "MySender", reg.SenderName)
			assert.Equal(t, domain.SenderTypePaid, reg.Type)
			assert.Equal(t, domain.SenderStatusPending, reg.Status)
			regRepo.AssertExpectations(t)
		})

		t.Run("duplicate_registration", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			clientID := uuid.New()
			operatorID := uuid.New()

			existing := &domain.SenderRegistration{
				ID:         uuid.New(),
				ClientID:   clientID,
				OperatorID: operatorID,
				SenderName: "MySender",
			}
			regRepo.On("GetActiveByClientOperatorName", ctx, clientID, operatorID, "MySender").
				Return(existing, nil)

			reg, err := svc.CreateRegistration(ctx, clientID, operatorID, "MySender", domain.SenderTypePaid)

			assert.Nil(t, reg)
			assert.ErrorIs(t, err, domain.ErrSenderRegistrationDuplicate)
		})

		t.Run("invalid_registration", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()

			// Empty sender name causes Validate to fail
			reg, err := svc.CreateRegistration(ctx, uuid.New(), uuid.New(), "", domain.SenderTypePaid)

			assert.Nil(t, reg)
			assert.Error(t, err)
		})

		t.Run("repo_create_error", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			clientID := uuid.New()
			operatorID := uuid.New()

			regRepo.On("GetActiveByClientOperatorName", ctx, clientID, operatorID, "MySender").
				Return(nil, domain.ErrSenderRegistrationNotFound)
			regRepo.On("Create", ctx, mock.AnythingOfType("*domain.SenderRegistration")).
				Return(errors.New("db error"))

			reg, err := svc.CreateRegistration(ctx, clientID, operatorID, "MySender", domain.SenderTypePaid)

			assert.Nil(t, reg)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create sender registration")
		})
	})

	t.Run("UpdateRegistration", func(t *testing.T) {
		t.Run("update_status_success", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			regID := uuid.New()

			existing := &domain.SenderRegistration{
				ID:     regID,
				Status: domain.SenderStatusPending,
				Type:   domain.SenderTypePaid,
			}
			regRepo.On("GetByID", ctx, regID).Return(existing, nil)
			regRepo.On("Update", ctx, mock.AnythingOfType("*domain.SenderRegistration")).Return(nil)

			reg, err := svc.UpdateRegistration(ctx, regID, domain.SenderStatusActive, nil)

			require.NoError(t, err)
			assert.Equal(t, domain.SenderStatusActive, reg.Status)
			assert.Equal(t, domain.SenderTypePaid, reg.Type) // type unchanged
		})

		t.Run("update_status_and_type", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			regID := uuid.New()

			existing := &domain.SenderRegistration{
				ID:     regID,
				Status: domain.SenderStatusPending,
				Type:   domain.SenderTypePaid,
			}
			regRepo.On("GetByID", ctx, regID).Return(existing, nil)
			regRepo.On("Update", ctx, mock.AnythingOfType("*domain.SenderRegistration")).Return(nil)

			newType := domain.SenderTypeFree
			reg, err := svc.UpdateRegistration(ctx, regID, domain.SenderStatusActive, &newType)

			require.NoError(t, err)
			assert.Equal(t, domain.SenderStatusActive, reg.Status)
			assert.Equal(t, domain.SenderTypeFree, reg.Type)
		})

		t.Run("invalid_status", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			regID := uuid.New()

			existing := &domain.SenderRegistration{ID: regID, Status: domain.SenderStatusPending}
			regRepo.On("GetByID", ctx, regID).Return(existing, nil)

			reg, err := svc.UpdateRegistration(ctx, regID, "invalid_status", nil)

			assert.Nil(t, reg)
			assert.ErrorIs(t, err, domain.ErrSenderRegistrationInvalidStatus)
		})

		t.Run("not_found", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			regID := uuid.New()

			regRepo.On("GetByID", ctx, regID).Return(nil, domain.ErrSenderRegistrationNotFound)

			reg, err := svc.UpdateRegistration(ctx, regID, domain.SenderStatusActive, nil)

			assert.Nil(t, reg)
			assert.Error(t, err)
		})

		t.Run("repo_update_error", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			regID := uuid.New()

			existing := &domain.SenderRegistration{ID: regID, Status: domain.SenderStatusPending}
			regRepo.On("GetByID", ctx, regID).Return(existing, nil)
			regRepo.On("Update", ctx, mock.AnythingOfType("*domain.SenderRegistration")).
				Return(errors.New("db error"))

			reg, err := svc.UpdateRegistration(ctx, regID, domain.SenderStatusActive, nil)

			assert.Nil(t, reg)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to update sender registration")
		})

		t.Run("set_expired_status", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			regID := uuid.New()

			existing := &domain.SenderRegistration{ID: regID, Status: domain.SenderStatusActive}
			regRepo.On("GetByID", ctx, regID).Return(existing, nil)
			regRepo.On("Update", ctx, mock.AnythingOfType("*domain.SenderRegistration")).Return(nil)

			reg, err := svc.UpdateRegistration(ctx, regID, domain.SenderStatusExpired, nil)

			require.NoError(t, err)
			assert.Equal(t, domain.SenderStatusExpired, reg.Status)
		})
	})

	t.Run("DetermineSenderCategory", func(t *testing.T) {
		t.Run("empty_sender_name_returns_shared", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()

			cat, err := svc.DetermineSenderCategory(ctx, uuid.New(), uuid.New(), "")

			require.NoError(t, err)
			assert.Equal(t, domain.CategoryShared, cat)
		})

		t.Run("paid_registration_returns_paid_registered", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			clientID := uuid.New()
			operatorID := uuid.New()

			reg := &domain.SenderRegistration{
				ID:   uuid.New(),
				Type: domain.SenderTypePaid,
			}
			regRepo.On("GetActiveByClientOperatorName", ctx, clientID, operatorID, "MySender").
				Return(reg, nil)

			cat, err := svc.DetermineSenderCategory(ctx, clientID, operatorID, "MySender")

			require.NoError(t, err)
			assert.Equal(t, domain.CategoryPaidRegistered, cat)
		})

		t.Run("free_registration_returns_free_registered", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			clientID := uuid.New()
			operatorID := uuid.New()

			reg := &domain.SenderRegistration{
				ID:   uuid.New(),
				Type: domain.SenderTypeFree,
			}
			regRepo.On("GetActiveByClientOperatorName", ctx, clientID, operatorID, "MySender").
				Return(reg, nil)

			cat, err := svc.DetermineSenderCategory(ctx, clientID, operatorID, "MySender")

			require.NoError(t, err)
			assert.Equal(t, domain.CategoryFreeRegistered, cat)
		})

		t.Run("no_registration_returns_shared", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			clientID := uuid.New()
			operatorID := uuid.New()

			regRepo.On("GetActiveByClientOperatorName", ctx, clientID, operatorID, "Unknown").
				Return(nil, domain.ErrSenderRegistrationNotFound)

			cat, err := svc.DetermineSenderCategory(ctx, clientID, operatorID, "Unknown")

			require.NoError(t, err)
			assert.Equal(t, domain.CategoryShared, cat)
		})

		t.Run("repo_error_returns_error", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			clientID := uuid.New()
			operatorID := uuid.New()

			regRepo.On("GetActiveByClientOperatorName", ctx, clientID, operatorID, "MySender").
				Return(nil, errors.New("db error"))

			cat, err := svc.DetermineSenderCategory(ctx, clientID, operatorID, "MySender")

			assert.Empty(t, cat)
			assert.Error(t, err)
		})

		t.Run("unknown_type_returns_shared", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			clientID := uuid.New()
			operatorID := uuid.New()

			reg := &domain.SenderRegistration{
				ID:   uuid.New(),
				Type: "unknown_type",
			}
			regRepo.On("GetActiveByClientOperatorName", ctx, clientID, operatorID, "MySender").
				Return(reg, nil)

			cat, err := svc.DetermineSenderCategory(ctx, clientID, operatorID, "MySender")

			require.NoError(t, err)
			assert.Equal(t, domain.CategoryShared, cat)
		})
	})

	t.Run("ListRegistrations", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			clientID := uuid.New()

			regs := []*domain.SenderRegistration{
				{ID: uuid.New(), ClientID: clientID},
			}
			regRepo.On("List", ctx, &clientID, (*uuid.UUID)(nil), 10, 0).Return(regs, 1, nil)

			result, total, err := svc.ListRegistrations(ctx, &clientID, nil, 10, 0)

			require.NoError(t, err)
			assert.Equal(t, 1, total)
			assert.Len(t, result, 1)
		})

		t.Run("error", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()

			regRepo.On("List", ctx, (*uuid.UUID)(nil), (*uuid.UUID)(nil), 10, 0).
				Return(nil, 0, errors.New("db error"))

			result, total, err := svc.ListRegistrations(ctx, nil, nil, 10, 0)

			assert.Nil(t, result)
			assert.Equal(t, 0, total)
			assert.Error(t, err)
		})
	})

	t.Run("GetRegistration", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			regID := uuid.New()

			expected := &domain.SenderRegistration{ID: regID}
			regRepo.On("GetByID", ctx, regID).Return(expected, nil)

			reg, err := svc.GetRegistration(ctx, regID)

			require.NoError(t, err)
			assert.Equal(t, regID, reg.ID)
		})

		t.Run("not_found", func(t *testing.T) {
			regRepo := new(mocks.MockSenderRegistrationRepository)
			svc := application.NewSenderService(regRepo)
			ctx := context.Background()
			regID := uuid.New()

			regRepo.On("GetByID", ctx, regID).Return(nil, domain.ErrSenderRegistrationNotFound)

			reg, err := svc.GetRegistration(ctx, regID)

			assert.Nil(t, reg)
			assert.Error(t, err)
		})
	})
}
