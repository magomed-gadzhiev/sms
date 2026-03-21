package application_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/auth/application"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/services/auth/mocks"
)

func TestPasswordResetService(t *testing.T) {
	ctx := context.Background()

	t.Run("RequestReset", func(t *testing.T) {

		t.Run("valid email returns token and user ID", func(t *testing.T) {
			resetRepo := new(mocks.MockPasswordResetRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewPasswordResetService(resetRepo, userRepo, passwordHasher)

			userID := uuid.New()
			user := activeUserWithRole(userID)

			userRepo.On("GetByEmail", ctx, "test@example.com").Return(user, nil)
			resetRepo.On("CountRecentByUserID", ctx, userID, mock.AnythingOfType("time.Time")).Return(0, nil)
			resetRepo.On("Create", ctx, mock.AnythingOfType("*domain.PasswordResetToken")).Return(nil)

			tokenStr, returnedUserID, err := svc.RequestReset(ctx, "test@example.com")

			require.NoError(t, err)
			assert.NotEmpty(t, tokenStr)
			assert.Equal(t, userID, returnedUserID)

			userRepo.AssertExpectations(t)
			resetRepo.AssertExpectations(t)
		})

		t.Run("user not found returns ErrUserNotFound", func(t *testing.T) {
			resetRepo := new(mocks.MockPasswordResetRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewPasswordResetService(resetRepo, userRepo, passwordHasher)

			userRepo.On("GetByEmail", ctx, "unknown@example.com").Return(nil, authrepo.ErrUserNotFound)

			tokenStr, returnedUserID, err := svc.RequestReset(ctx, "unknown@example.com")

			assert.ErrorIs(t, err, authrepo.ErrUserNotFound)
			assert.Empty(t, tokenStr)
			assert.Equal(t, uuid.Nil, returnedUserID)

			userRepo.AssertExpectations(t)
		})

		t.Run("rate limit exceeded returns ErrPasswordResetRateLimit", func(t *testing.T) {
			resetRepo := new(mocks.MockPasswordResetRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewPasswordResetService(resetRepo, userRepo, passwordHasher)

			userID := uuid.New()
			user := activeUserWithRole(userID)

			userRepo.On("GetByEmail", ctx, "test@example.com").Return(user, nil)
			resetRepo.On("CountRecentByUserID", ctx, userID, mock.AnythingOfType("time.Time")).Return(3, nil) // max is 3

			tokenStr, returnedUserID, err := svc.RequestReset(ctx, "test@example.com")

			assert.ErrorIs(t, err, application.ErrPasswordResetRateLimit)
			assert.Empty(t, tokenStr)
			assert.Equal(t, uuid.Nil, returnedUserID)

			userRepo.AssertExpectations(t)
			resetRepo.AssertExpectations(t)
		})
	})

	t.Run("ResetPassword", func(t *testing.T) {

		t.Run("valid token resets password", func(t *testing.T) {
			resetRepo := new(mocks.MockPasswordResetRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewPasswordResetService(resetRepo, userRepo, passwordHasher)

			userID := uuid.New()
			tokenID := uuid.New()
			tokenStr := "reset-token-string"
			tokenHash := sha256Hash(tokenStr)

			resetToken := &domain.PasswordResetToken{
				ID:        tokenID,
				UserID:    userID,
				TokenHash: tokenHash,
				ExpiresAt: time.Now().Add(1 * time.Hour), // not expired
				Used:      false,
				CreatedAt: time.Now(),
			}

			user := activeUserWithRole(userID)

			resetRepo.On("GetByTokenHash", ctx, tokenHash).Return(resetToken, nil)
			passwordHasher.On("HashPassword", "new-password").Return("new-hashed-password", nil)
			userRepo.On("GetByID", ctx, userID).Return(user, nil)
			userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)
			resetRepo.On("MarkUsed", ctx, tokenID).Return(nil)

			err := svc.ResetPassword(ctx, tokenStr, "new-password")

			require.NoError(t, err)

			resetRepo.AssertExpectations(t)
			passwordHasher.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})

		t.Run("token not found returns ErrPasswordResetInvalid", func(t *testing.T) {
			resetRepo := new(mocks.MockPasswordResetRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewPasswordResetService(resetRepo, userRepo, passwordHasher)

			tokenStr := "nonexistent-token"
			tokenHash := sha256Hash(tokenStr)

			resetRepo.On("GetByTokenHash", ctx, tokenHash).
				Return(nil, authrepo.ErrPasswordResetTokenNotFound)

			err := svc.ResetPassword(ctx, tokenStr, "new-password")

			assert.ErrorIs(t, err, application.ErrPasswordResetInvalid)

			resetRepo.AssertExpectations(t)
		})

		t.Run("used token returns ErrPasswordResetTokenUsed", func(t *testing.T) {
			resetRepo := new(mocks.MockPasswordResetRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewPasswordResetService(resetRepo, userRepo, passwordHasher)

			tokenStr := "used-token"
			tokenHash := sha256Hash(tokenStr)

			resetToken := &domain.PasswordResetToken{
				ID:        uuid.New(),
				UserID:    uuid.New(),
				TokenHash: tokenHash,
				ExpiresAt: time.Now().Add(1 * time.Hour),
				Used:      true, // already used
				CreatedAt: time.Now(),
			}

			resetRepo.On("GetByTokenHash", ctx, tokenHash).Return(resetToken, nil)

			err := svc.ResetPassword(ctx, tokenStr, "new-password")

			assert.ErrorIs(t, err, domain.ErrPasswordResetTokenUsed)

			resetRepo.AssertExpectations(t)
		})

		t.Run("expired token returns ErrPasswordResetTokenExpired", func(t *testing.T) {
			resetRepo := new(mocks.MockPasswordResetRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewPasswordResetService(resetRepo, userRepo, passwordHasher)

			tokenStr := "expired-token"
			tokenHash := sha256Hash(tokenStr)

			resetToken := &domain.PasswordResetToken{
				ID:        uuid.New(),
				UserID:    uuid.New(),
				TokenHash: tokenHash,
				ExpiresAt: time.Now().Add(-1 * time.Hour), // expired
				Used:      false,
				CreatedAt: time.Now().Add(-2 * time.Hour),
			}

			resetRepo.On("GetByTokenHash", ctx, tokenHash).Return(resetToken, nil)

			err := svc.ResetPassword(ctx, tokenStr, "new-password")

			assert.ErrorIs(t, err, domain.ErrPasswordResetTokenExpired)

			resetRepo.AssertExpectations(t)
		})
	})
}

// sha256Hash is a test helper matching the service's internal hashing logic.
func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
