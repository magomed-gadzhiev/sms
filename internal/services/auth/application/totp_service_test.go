package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/auth/application"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/services/auth/mocks"
)

// 32-byte key for AES-256 tests
var testEncryptionKey = []byte("01234567890123456789012345678901")

func TestTOTPService(t *testing.T) {
	ctx := context.Background()

	t.Run("SetupTOTP", func(t *testing.T) {

		t.Run("successful setup returns secret and QR URL and recovery codes", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()

			// TOTP not yet configured
			totpRepo.On("GetTOTPConfig", ctx, userID).Return(nil, authrepo.ErrTOTPConfigNotFound)
			totpRepo.On("SaveTOTPSecret", ctx, userID, mock.AnythingOfType("[]uint8")).Return(nil)
			totpRepo.On("DeleteRecoveryCodes", ctx, userID).Return(nil)
			totpRepo.On("SaveRecoveryCodes", ctx, mock.AnythingOfType("[]*domain.TOTPRecoveryCode")).Return(nil)

			secret, qrURL, recoveryCodes, err := svc.SetupTOTP(ctx, userID, "testuser@example.com")

			require.NoError(t, err)
			assert.NotEmpty(t, secret)
			assert.NotEmpty(t, qrURL)
			assert.Contains(t, qrURL, "otpauth://totp/")
			assert.Len(t, recoveryCodes, 10) // recoveryCodeCount = 10
			for _, code := range recoveryCodes {
				assert.Len(t, code, 8) // recoveryCodeLength = 8
			}

			totpRepo.AssertExpectations(t)
		})

		t.Run("already enabled returns ErrTOTPAlreadyEnabled", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()
			now := time.Now()

			config := &domain.TOTPConfig{
				UserID:          userID,
				SecretEncrypted: []byte("encrypted-secret"),
				Enabled:         true,
				VerifiedAt:      &now,
			}
			totpRepo.On("GetTOTPConfig", ctx, userID).Return(config, nil)

			_, _, _, err := svc.SetupTOTP(ctx, userID, "testuser@example.com")

			assert.ErrorIs(t, err, domain.ErrTOTPAlreadyEnabled)

			totpRepo.AssertExpectations(t)
		})
	})

	t.Run("VerifyAndEnable", func(t *testing.T) {

		t.Run("valid code enables TOTP", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()

			// First do a setup to get a real secret
			totpRepo.On("GetTOTPConfig", ctx, userID).Return(nil, authrepo.ErrTOTPConfigNotFound).Once()

			var savedEncryptedSecret []byte
			totpRepo.On("SaveTOTPSecret", ctx, userID, mock.AnythingOfType("[]uint8")).
				Run(func(args mock.Arguments) {
					savedEncryptedSecret = args.Get(2).([]byte)
				}).
				Return(nil)
			totpRepo.On("DeleteRecoveryCodes", ctx, userID).Return(nil)
			totpRepo.On("SaveRecoveryCodes", ctx, mock.AnythingOfType("[]*domain.TOTPRecoveryCode")).Return(nil)

			secret, _, _, err := svc.SetupTOTP(ctx, userID, "testuser@example.com")
			require.NoError(t, err)

			// Now verify: return config with encrypted secret, not yet enabled
			config := &domain.TOTPConfig{
				UserID:          userID,
				SecretEncrypted: savedEncryptedSecret,
				Enabled:         false,
			}
			totpRepo.On("GetTOTPConfig", ctx, userID).Return(config, nil).Once()
			totpRepo.On("EnableTOTP", ctx, userID).Return(nil)

			// Generate a valid TOTP code using the secret
			code, err := totp.GenerateCode(secret, time.Now())
			require.NoError(t, err)

			err = svc.VerifyAndEnable(ctx, userID, code)

			require.NoError(t, err)

			totpRepo.AssertExpectations(t)
		})

		t.Run("invalid code returns ErrInvalidTOTPCode", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()

			// Setup first
			totpRepo.On("GetTOTPConfig", ctx, userID).Return(nil, authrepo.ErrTOTPConfigNotFound).Once()

			var savedEncryptedSecret []byte
			totpRepo.On("SaveTOTPSecret", ctx, userID, mock.AnythingOfType("[]uint8")).
				Run(func(args mock.Arguments) {
					savedEncryptedSecret = args.Get(2).([]byte)
				}).
				Return(nil)
			totpRepo.On("DeleteRecoveryCodes", ctx, userID).Return(nil)
			totpRepo.On("SaveRecoveryCodes", ctx, mock.AnythingOfType("[]*domain.TOTPRecoveryCode")).Return(nil)

			_, _, _, err := svc.SetupTOTP(ctx, userID, "testuser@example.com")
			require.NoError(t, err)

			// Return config for verification
			config := &domain.TOTPConfig{
				UserID:          userID,
				SecretEncrypted: savedEncryptedSecret,
				Enabled:         false,
			}
			totpRepo.On("GetTOTPConfig", ctx, userID).Return(config, nil).Once()

			err = svc.VerifyAndEnable(ctx, userID, "000000") // wrong code

			assert.ErrorIs(t, err, domain.ErrInvalidTOTPCode)

			totpRepo.AssertExpectations(t)
		})

		t.Run("already enabled returns ErrTOTPAlreadyEnabled", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()
			now := time.Now()

			config := &domain.TOTPConfig{
				UserID:          userID,
				SecretEncrypted: []byte("encrypted-secret"),
				Enabled:         true,
				VerifiedAt:      &now,
			}
			totpRepo.On("GetTOTPConfig", ctx, userID).Return(config, nil)

			err := svc.VerifyAndEnable(ctx, userID, "123456")

			assert.ErrorIs(t, err, domain.ErrTOTPAlreadyEnabled)

			totpRepo.AssertExpectations(t)
		})
	})

	t.Run("ValidateCode", func(t *testing.T) {

		t.Run("valid code returns true", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()

			// Setup first to get encrypted secret
			totpRepo.On("GetTOTPConfig", ctx, userID).Return(nil, authrepo.ErrTOTPConfigNotFound).Once()

			var savedEncryptedSecret []byte
			totpRepo.On("SaveTOTPSecret", ctx, userID, mock.AnythingOfType("[]uint8")).
				Run(func(args mock.Arguments) {
					savedEncryptedSecret = args.Get(2).([]byte)
				}).
				Return(nil)
			totpRepo.On("DeleteRecoveryCodes", ctx, userID).Return(nil)
			totpRepo.On("SaveRecoveryCodes", ctx, mock.AnythingOfType("[]*domain.TOTPRecoveryCode")).Return(nil)

			secret, _, _, err := svc.SetupTOTP(ctx, userID, "testuser@example.com")
			require.NoError(t, err)

			// Now validate code with enabled config
			now := time.Now()
			config := &domain.TOTPConfig{
				UserID:          userID,
				SecretEncrypted: savedEncryptedSecret,
				Enabled:         true,
				VerifiedAt:      &now,
			}
			totpRepo.On("GetTOTPConfig", ctx, userID).Return(config, nil).Once()

			code, err := totp.GenerateCode(secret, time.Now())
			require.NoError(t, err)

			valid, err := svc.ValidateCode(ctx, userID, code)

			require.NoError(t, err)
			assert.True(t, valid)

			totpRepo.AssertExpectations(t)
		})

		t.Run("TOTP not enabled returns ErrTOTPNotEnabled", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()
			config := &domain.TOTPConfig{
				UserID:  userID,
				Enabled: false,
			}
			totpRepo.On("GetTOTPConfig", ctx, userID).Return(config, nil)

			valid, err := svc.ValidateCode(ctx, userID, "123456")

			assert.ErrorIs(t, err, domain.ErrTOTPNotEnabled)
			assert.False(t, valid)

			totpRepo.AssertExpectations(t)
		})
	})

	t.Run("ValidateRecoveryCode", func(t *testing.T) {

		t.Run("valid code returns true", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()
			totpRepo.On("UseRecoveryCode", ctx, userID, mock.AnythingOfType("string")).Return(nil)

			valid, err := svc.ValidateRecoveryCode(ctx, userID, "abc12345")

			require.NoError(t, err)
			assert.True(t, valid)

			totpRepo.AssertExpectations(t)
		})

		t.Run("invalid code returns false", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()
			totpRepo.On("UseRecoveryCode", ctx, userID, mock.AnythingOfType("string")).
				Return(authrepo.ErrRecoveryCodeNotFound)

			valid, err := svc.ValidateRecoveryCode(ctx, userID, "wrongcode")

			require.NoError(t, err)
			assert.False(t, valid)

			totpRepo.AssertExpectations(t)
		})
	})

	t.Run("Disable", func(t *testing.T) {

		t.Run("with correct password disables TOTP", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()
			user := activeUserWithRole(userID)

			userRepo.On("GetByID", ctx, userID).Return(user, nil)
			passwordHasher.On("CheckPassword", "correct-password", user.PasswordHash).Return(true)
			totpRepo.On("DisableTOTP", ctx, userID).Return(nil)
			totpRepo.On("DeleteRecoveryCodes", ctx, userID).Return(nil)

			err := svc.Disable(ctx, userID, "correct-password")

			require.NoError(t, err)

			userRepo.AssertExpectations(t)
			passwordHasher.AssertExpectations(t)
			totpRepo.AssertExpectations(t)
		})

		t.Run("with wrong password returns ErrPasswordMismatch", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()
			user := activeUserWithRole(userID)

			userRepo.On("GetByID", ctx, userID).Return(user, nil)
			passwordHasher.On("CheckPassword", "wrong-password", user.PasswordHash).Return(false)

			err := svc.Disable(ctx, userID, "wrong-password")

			assert.ErrorIs(t, err, application.ErrPasswordMismatch)

			userRepo.AssertExpectations(t)
			passwordHasher.AssertExpectations(t)
		})

		t.Run("user not found returns error", func(t *testing.T) {
			totpRepo := new(mocks.MockTOTPRepository)
			userRepo := new(mocks.MockUserRepository)
			passwordHasher := new(mocks.MockPasswordHasher)

			svc := application.NewTOTPService(totpRepo, userRepo, passwordHasher, testEncryptionKey)

			userID := uuid.New()

			userRepo.On("GetByID", ctx, userID).Return(nil, authrepo.ErrUserNotFound)

			err := svc.Disable(ctx, userID, "password")

			assert.ErrorIs(t, err, authrepo.ErrUserNotFound)

			userRepo.AssertExpectations(t)
		})
	})
}
