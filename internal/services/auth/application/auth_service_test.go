package application_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// ---------- helpers ----------

func newTestTokenService(t *testing.T, refreshRepo *mocks.MockRefreshTokenRepository) *application.TokenService {
	t.Helper()
	return application.NewTokenService(
		"test-secret-key-for-unit-tests",
		15*time.Minute,
		7*24*time.Hour,
		refreshRepo,
	)
}

func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func activeUserWithRole(id uuid.UUID) *domain.User {
	roleID := uuid.New()
	return &domain.User{
		ID:           id,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashed-password",
		RoleID:       roleID,
		Active:       true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Role: &domain.Role{
			ID:   roleID,
			Name: "admin",
		},
	}
}

func inactiveUser(id uuid.UUID) *domain.User {
	u := activeUserWithRole(id)
	u.Active = false
	return u
}

// ---------- TestAuthService ----------

func TestAuthService(t *testing.T) {
	ctx := context.Background()

	t.Run("AuthenticateByCredentials", func(t *testing.T) {

		t.Run("with valid credentials returns user and tokens", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			userID := uuid.New()
			user := activeUserWithRole(userID)

			userRepo.On("GetByUsername", ctx, "testuser").Return(user, nil)
			passwordHasher.On("CheckPassword", "correct-password", user.PasswordHash).Return(true)
			userRepo.On("GetByIDWithRole", ctx, userID).Return(user, nil)
			refreshRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

			resultUser, accessToken, refreshToken, err := svc.AuthenticateByCredentials(ctx, "testuser", "correct-password")

			require.NoError(t, err)
			assert.Equal(t, userID, resultUser.ID)
			assert.NotEmpty(t, accessToken)
			assert.NotEmpty(t, refreshToken)
			assert.Equal(t, "admin", resultUser.Role.Name)

			userRepo.AssertExpectations(t)
			passwordHasher.AssertExpectations(t)
			refreshRepo.AssertExpectations(t)
		})

		t.Run("with valid email credentials returns user and tokens", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			userID := uuid.New()
			user := activeUserWithRole(userID)

			// Username lookup fails -> falls through to email lookup
			userRepo.On("GetByUsername", ctx, "test@example.com").Return(nil, authrepo.ErrUserNotFound)
			userRepo.On("GetByEmail", ctx, "test@example.com").Return(user, nil)
			passwordHasher.On("CheckPassword", "correct-password", user.PasswordHash).Return(true)
			userRepo.On("GetByIDWithRole", ctx, userID).Return(user, nil)
			refreshRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

			resultUser, accessToken, refreshToken, err := svc.AuthenticateByCredentials(ctx, "test@example.com", "correct-password")

			require.NoError(t, err)
			assert.Equal(t, userID, resultUser.ID)
			assert.NotEmpty(t, accessToken)
			assert.NotEmpty(t, refreshToken)

			userRepo.AssertExpectations(t)
			passwordHasher.AssertExpectations(t)
			refreshRepo.AssertExpectations(t)
		})

		t.Run("with invalid password returns ErrInvalidCredentials", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			userID := uuid.New()
			user := activeUserWithRole(userID)

			userRepo.On("GetByUsername", ctx, "testuser").Return(user, nil)
			passwordHasher.On("CheckPassword", "wrong-password", user.PasswordHash).Return(false)

			resultUser, accessToken, refreshToken, err := svc.AuthenticateByCredentials(ctx, "testuser", "wrong-password")

			assert.ErrorIs(t, err, application.ErrInvalidCredentials)
			assert.Nil(t, resultUser)
			assert.Empty(t, accessToken)
			assert.Empty(t, refreshToken)

			userRepo.AssertExpectations(t)
			passwordHasher.AssertExpectations(t)
		})

		t.Run("user not found returns ErrInvalidCredentials", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			userRepo.On("GetByUsername", ctx, "nonexistent").Return(nil, authrepo.ErrUserNotFound)
			userRepo.On("GetByEmail", ctx, "nonexistent").Return(nil, authrepo.ErrUserNotFound)

			resultUser, accessToken, refreshToken, err := svc.AuthenticateByCredentials(ctx, "nonexistent", "password")

			assert.ErrorIs(t, err, application.ErrInvalidCredentials)
			assert.Nil(t, resultUser)
			assert.Empty(t, accessToken)
			assert.Empty(t, refreshToken)

			userRepo.AssertExpectations(t)
		})

		t.Run("inactive user returns ErrUserInactive", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			userID := uuid.New()
			user := inactiveUser(userID)

			userRepo.On("GetByUsername", ctx, "testuser").Return(user, nil)
			passwordHasher.On("CheckPassword", "correct-password", user.PasswordHash).Return(true)

			resultUser, _, _, err := svc.AuthenticateByCredentials(ctx, "testuser", "correct-password")

			assert.ErrorIs(t, err, application.ErrUserInactive)
			assert.Nil(t, resultUser)

			userRepo.AssertExpectations(t)
			passwordHasher.AssertExpectations(t)
		})
	})

	t.Run("AuthenticateByAPIKey", func(t *testing.T) {

		t.Run("valid key returns user", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			userID := uuid.New()
			keyID := uuid.New()
			rawKey := "sk_test_abc123"
			keyHash := hashAPIKey(rawKey)

			apiKey := &domain.APIKey{
				ID:        keyID,
				UserID:    userID,
				Name:      "Test Key",
				KeyHash:   keyHash,
				KeyPrefix: "sk_test_",
				Active:    true,
				ExpiresAt: nil, // no expiration
				Scopes:    []string{"messages:read", "messages:send"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			user := activeUserWithRole(userID)

			apiKeyRepo.On("GetByKeyHash", ctx, keyHash).Return(apiKey, nil)
			apiKeyRepo.On("UpdateLastUsed", ctx, keyID).Return(nil)
			userRepo.On("GetByIDWithRole", ctx, userID).Return(user, nil)

			resultUser, scopes, err := svc.AuthenticateByAPIKey(ctx, rawKey)

			require.NoError(t, err)
			assert.Equal(t, userID, resultUser.ID)
			assert.Equal(t, []string{"messages:read", "messages:send"}, scopes)

			apiKeyRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})

		t.Run("expired key returns ErrAPIKeyInvalid", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			rawKey := "sk_test_expired"
			keyHash := hashAPIKey(rawKey)
			pastTime := time.Now().Add(-24 * time.Hour)

			apiKey := &domain.APIKey{
				ID:        uuid.New(),
				UserID:    uuid.New(),
				Name:      "Expired Key",
				KeyHash:   keyHash,
				Active:    true,
				ExpiresAt: &pastTime,
				CreatedAt: time.Now().Add(-48 * time.Hour),
				UpdatedAt: time.Now().Add(-48 * time.Hour),
			}

			apiKeyRepo.On("GetByKeyHash", ctx, keyHash).Return(apiKey, nil)

			resultUser, _, err := svc.AuthenticateByAPIKey(ctx, rawKey)

			assert.ErrorIs(t, err, application.ErrAPIKeyInvalid)
			assert.Nil(t, resultUser)

			apiKeyRepo.AssertExpectations(t)
		})

		t.Run("revoked key returns ErrAPIKeyInvalid", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			rawKey := "sk_test_revoked"
			keyHash := hashAPIKey(rawKey)

			apiKey := &domain.APIKey{
				ID:      uuid.New(),
				UserID:  uuid.New(),
				Name:    "Revoked Key",
				KeyHash: keyHash,
				Active:  false, // revoked
			}

			apiKeyRepo.On("GetByKeyHash", ctx, keyHash).Return(apiKey, nil)

			resultUser, _, err := svc.AuthenticateByAPIKey(ctx, rawKey)

			assert.ErrorIs(t, err, application.ErrAPIKeyInvalid)
			assert.Nil(t, resultUser)

			apiKeyRepo.AssertExpectations(t)
		})

		t.Run("key not found returns ErrAPIKeyInvalid", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			rawKey := "sk_test_unknown"
			keyHash := hashAPIKey(rawKey)

			apiKeyRepo.On("GetByKeyHash", ctx, keyHash).Return(nil, authrepo.ErrAPIKeyNotFound)

			resultUser, _, err := svc.AuthenticateByAPIKey(ctx, rawKey)

			assert.ErrorIs(t, err, application.ErrAPIKeyInvalid)
			assert.Nil(t, resultUser)

			apiKeyRepo.AssertExpectations(t)
		})

		t.Run("IP not in whitelist returns ErrIPNotAllowed", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			rawKey := "sk_test_ip_restricted"
			keyHash := hashAPIKey(rawKey)

			apiKey := &domain.APIKey{
				ID:         uuid.New(),
				UserID:     uuid.New(),
				Name:       "IP Restricted Key",
				KeyHash:    keyHash,
				Active:     true,
				AllowedIPs: []string{"10.0.0.1", "10.0.0.2"},
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			apiKeyRepo.On("GetByKeyHash", ctx, keyHash).Return(apiKey, nil)

			resultUser, _, err := svc.AuthenticateByAPIKey(ctx, rawKey, "192.168.1.1")

			assert.ErrorIs(t, err, application.ErrIPNotAllowed)
			assert.Nil(t, resultUser)

			apiKeyRepo.AssertExpectations(t)
		})

		t.Run("inactive user with valid key returns ErrUserInactive", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			userID := uuid.New()
			keyID := uuid.New()
			rawKey := "sk_test_inactive_user"
			keyHash := hashAPIKey(rawKey)

			apiKey := &domain.APIKey{
				ID:        keyID,
				UserID:    userID,
				Name:      "Key for inactive user",
				KeyHash:   keyHash,
				Active:    true,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			user := inactiveUser(userID)

			apiKeyRepo.On("GetByKeyHash", ctx, keyHash).Return(apiKey, nil)
			apiKeyRepo.On("UpdateLastUsed", ctx, keyID).Return(nil)
			userRepo.On("GetByIDWithRole", ctx, userID).Return(user, nil)

			resultUser, _, err := svc.AuthenticateByAPIKey(ctx, rawKey)

			assert.ErrorIs(t, err, application.ErrUserInactive)
			assert.Nil(t, resultUser)

			apiKeyRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	})

	t.Run("ValidateToken", func(t *testing.T) {

		t.Run("valid token returns user", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			userID := uuid.New()
			user := activeUserWithRole(userID)

			// Generate a real token pair first
			refreshRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)
			accessToken, _, err := tokenService.GenerateTokenPair(ctx, userID, "admin")
			require.NoError(t, err)

			userRepo.On("GetByIDWithRole", ctx, userID).Return(user, nil)

			resultUser, err := svc.ValidateToken(ctx, accessToken)

			require.NoError(t, err)
			assert.Equal(t, userID, resultUser.ID)

			userRepo.AssertExpectations(t)
			refreshRepo.AssertExpectations(t)
		})

		t.Run("invalid token returns error", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			resultUser, err := svc.ValidateToken(ctx, "invalid-token-string")

			assert.Error(t, err)
			assert.Nil(t, resultUser)
		})
	})

	t.Run("CreateAPIKey", func(t *testing.T) {

		t.Run("returns new key", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			userID := uuid.New()
			user := activeUserWithRole(userID)

			userRepo.On("GetByID", ctx, userID).Return(user, nil)
			apiKeyGen.On("GenerateAPIKey").Return("sk_test_newkey123", nil)
			apiKeyGen.On("GetKeyPrefix", "sk_test_newkey123").Return("sk_test_")
			apiKeyRepo.On("Create", ctx, mock.AnythingOfType("*domain.APIKey")).Return(nil)

			expiresAt := time.Now().Add(30 * 24 * time.Hour)
			scopes := []string{"messages:send", "messages:read"}
			allowedIPs := []string{"10.0.0.0/8"}

			key, rawKey, err := svc.CreateAPIKey(ctx, userID, "My Key", &expiresAt, scopes, allowedIPs)

			require.NoError(t, err)
			assert.Equal(t, "sk_test_newkey123", rawKey)
			assert.Equal(t, "My Key", key.Name)
			assert.Equal(t, userID, key.UserID)
			assert.True(t, key.Active)
			assert.Equal(t, "sk_test_", key.KeyPrefix)
			assert.Equal(t, scopes, key.Scopes)
			assert.Equal(t, allowedIPs, key.AllowedIPs)

			userRepo.AssertExpectations(t)
			apiKeyGen.AssertExpectations(t)
			apiKeyRepo.AssertExpectations(t)
		})

		t.Run("user not found returns error", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			userID := uuid.New()
			userRepo.On("GetByID", ctx, userID).Return(nil, authrepo.ErrUserNotFound)

			key, rawKey, err := svc.CreateAPIKey(ctx, userID, "My Key", nil, nil, nil)

			assert.ErrorIs(t, err, authrepo.ErrUserNotFound)
			assert.Nil(t, key)
			assert.Empty(t, rawKey)

			userRepo.AssertExpectations(t)
		})
	})

	t.Run("RevokeAPIKey", func(t *testing.T) {

		t.Run("success", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			keyID := uuid.New()
			userID := uuid.New()
			apiKeyRepo.On("GetByID", ctx, keyID).Return(&domain.APIKey{ID: keyID, UserID: userID, Active: true}, nil)
			apiKeyRepo.On("Revoke", ctx, keyID).Return(nil)

			err := svc.RevokeAPIKey(ctx, keyID, userID)

			require.NoError(t, err)
			apiKeyRepo.AssertExpectations(t)
		})

		t.Run("key not found returns error", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			keyID := uuid.New()
			userID := uuid.New()
			apiKeyRepo.On("GetByID", ctx, keyID).Return(nil, authrepo.ErrAPIKeyNotFound)

			err := svc.RevokeAPIKey(ctx, keyID, userID)

			assert.ErrorIs(t, err, authrepo.ErrAPIKeyNotFound)
			apiKeyRepo.AssertExpectations(t)
		})

		t.Run("not owned returns ErrAPIKeyNotOwned", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			keyID := uuid.New()
			ownerID := uuid.New()
			attackerID := uuid.New()
			apiKeyRepo.On("GetByID", ctx, keyID).Return(&domain.APIKey{ID: keyID, UserID: ownerID, Active: true}, nil)

			err := svc.RevokeAPIKey(ctx, keyID, attackerID)

			assert.ErrorIs(t, err, application.ErrAPIKeyNotOwned)
			apiKeyRepo.AssertExpectations(t)
			apiKeyRepo.AssertNotCalled(t, "Revoke")
		})
	})

	t.Run("RotateAPIKey", func(t *testing.T) {

		t.Run("success creates new key and soft-revokes old", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			keyID := uuid.New()
			userID := uuid.New()
			expiresAt := time.Now().Add(30 * 24 * time.Hour)
			oldKey := &domain.APIKey{
				ID:         keyID,
				UserID:     userID,
				Name:       "Slot",
				Active:     true,
				ExpiresAt:  &expiresAt,
				Scopes:     []string{"messages:read"},
				AllowedIPs: []string{"10.0.0.1"},
			}

			apiKeyRepo.On("GetByID", ctx, keyID).Return(oldKey, nil)
			apiKeyGen.On("GenerateAPIKey").Return("sk_test_newrawkey", nil)
			apiKeyGen.On("GetKeyPrefix", "sk_test_newrawkey").Return("sk_test_")
			apiKeyRepo.On("Create", ctx, mock.MatchedBy(func(k *domain.APIKey) bool {
				return k.Name == "Slot" &&
					k.UserID == userID &&
					k.Active == true &&
					len(k.Scopes) == 1 && k.Scopes[0] == "messages:read" &&
					len(k.AllowedIPs) == 1 && k.AllowedIPs[0] == "10.0.0.1" &&
					k.ExpiresAt != nil && k.ExpiresAt.Equal(expiresAt)
			})).Return(nil)
			apiKeyRepo.On("SetRevokeAt", ctx, keyID, mock.AnythingOfType("time.Time")).Return(nil)

			newKey, rawKey, oldRevokeAt, err := svc.RotateAPIKey(ctx, keyID, userID)

			require.NoError(t, err)
			require.NotNil(t, newKey)
			assert.Equal(t, "sk_test_newrawkey", rawKey)
			assert.NotEqual(t, keyID, newKey.ID, "new key must have different ID")
			assert.Equal(t, "Slot", newKey.Name)
			assert.WithinDuration(t, time.Now().Add(application.APIKeyRotateGrace), oldRevokeAt, 5*time.Second, "oldRevokeAt must be ~now+grace")
			apiKeyRepo.AssertExpectations(t)
			apiKeyGen.AssertExpectations(t)
		})

		t.Run("not owned returns ErrAPIKeyNotOwned", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			keyID := uuid.New()
			ownerID := uuid.New()
			attackerID := uuid.New()
			apiKeyRepo.On("GetByID", ctx, keyID).Return(&domain.APIKey{ID: keyID, UserID: ownerID, Active: true}, nil)

			newKey, rawKey, _, err := svc.RotateAPIKey(ctx, keyID, attackerID)

			assert.ErrorIs(t, err, application.ErrAPIKeyNotOwned)
			assert.Nil(t, newKey)
			assert.Empty(t, rawKey)
			apiKeyRepo.AssertNotCalled(t, "Create")
			apiKeyRepo.AssertNotCalled(t, "SetRevokeAt")
		})

		t.Run("revoked key returns ErrAPIKeyRevoked", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			keyID := uuid.New()
			userID := uuid.New()
			apiKeyRepo.On("GetByID", ctx, keyID).Return(&domain.APIKey{ID: keyID, UserID: userID, Active: false}, nil)

			newKey, _, _, err := svc.RotateAPIKey(ctx, keyID, userID)

			assert.ErrorIs(t, err, application.ErrAPIKeyRevoked)
			assert.Nil(t, newKey)
			apiKeyRepo.AssertNotCalled(t, "Create")
			apiKeyRepo.AssertNotCalled(t, "SetRevokeAt")
		})

		t.Run("setrevokeat fails triggers compensating delete", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			keyID := uuid.New()
			userID := uuid.New()
			apiKeyRepo.On("GetByID", ctx, keyID).Return(&domain.APIKey{ID: keyID, UserID: userID, Active: true}, nil)
			apiKeyGen.On("GenerateAPIKey").Return("sk_test_newrawkey", nil)
			apiKeyGen.On("GetKeyPrefix", "sk_test_newrawkey").Return("sk_test_")
			apiKeyRepo.On("Create", ctx, mock.AnythingOfType("*domain.APIKey")).Return(nil)
			apiKeyRepo.On("SetRevokeAt", ctx, keyID, mock.AnythingOfType("time.Time")).Return(errors.New("db down"))
			// Compensating delete должна быть вызвана:
			apiKeyRepo.On("Delete", ctx, mock.AnythingOfType("uuid.UUID")).Return(nil)

			newKey, _, _, err := svc.RotateAPIKey(ctx, keyID, userID)

			require.Error(t, err)
			assert.Nil(t, newKey)
			apiKeyRepo.AssertExpectations(t)
			apiKeyRepo.AssertCalled(t, "Delete", ctx, mock.AnythingOfType("uuid.UUID"))
		})
	})

	t.Run("ListAPIKeys", func(t *testing.T) {

		t.Run("returns keys for user", func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			apiKeyRepo := new(mocks.MockAPIKeyRepository)
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			passwordHasher := new(mocks.MockPasswordHasher)
			apiKeyGen := new(mocks.MockAPIKeyGenerator)

			tokenService := newTestTokenService(t, refreshRepo)
			svc := application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)

			userID := uuid.New()
			expectedKeys := []*domain.APIKey{
				{ID: uuid.New(), UserID: userID, Name: "Key 1", Active: true},
				{ID: uuid.New(), UserID: userID, Name: "Key 2", Active: false},
			}

			apiKeyRepo.On("ListByUserID", ctx, userID).Return(expectedKeys, nil)

			keys, err := svc.ListAPIKeys(ctx, userID)

			require.NoError(t, err)
			assert.Len(t, keys, 2)
			assert.Equal(t, "Key 1", keys[0].Name)
			assert.Equal(t, "Key 2", keys[1].Name)

			apiKeyRepo.AssertExpectations(t)
		})
	})
}

// ---------- TestChangePassword ----------

func TestChangePassword(t *testing.T) {
	ctx := context.Background()

	newSvc := func(userRepo *mocks.MockUserRepository, passwordHasher *mocks.MockPasswordHasher) *application.AuthService {
		apiKeyRepo := new(mocks.MockAPIKeyRepository)
		refreshRepo := new(mocks.MockRefreshTokenRepository)
		apiKeyGen := new(mocks.MockAPIKeyGenerator)
		tokenService := newTestTokenService(t, refreshRepo)
		return application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)
	}

	t.Run("success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		passwordHasher := new(mocks.MockPasswordHasher)
		svc := newSvc(userRepo, passwordHasher)

		userID := uuid.New()
		user := activeUserWithRole(userID)
		user.PasswordHash = "current-hash"

		userRepo.On("GetByID", ctx, userID).Return(user, nil)
		passwordHasher.On("CheckPassword", "current-pass", "current-hash").Return(true)
		passwordHasher.On("HashPassword", "new-password-123").Return("new-hash", nil)
		userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

		err := svc.ChangePassword(ctx, userID, "current-pass", "new-password-123")

		require.NoError(t, err)
		userRepo.AssertExpectations(t)
		passwordHasher.AssertExpectations(t)
	})

	t.Run("current password wrong returns ErrCurrentPasswordWrong", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		passwordHasher := new(mocks.MockPasswordHasher)
		svc := newSvc(userRepo, passwordHasher)

		userID := uuid.New()
		user := activeUserWithRole(userID)
		user.PasswordHash = "current-hash"

		userRepo.On("GetByID", ctx, userID).Return(user, nil)
		passwordHasher.On("CheckPassword", "wrong-pass", "current-hash").Return(false)

		err := svc.ChangePassword(ctx, userID, "wrong-pass", "new-password-123")

		assert.ErrorIs(t, err, application.ErrCurrentPasswordWrong)
		userRepo.AssertExpectations(t)
		passwordHasher.AssertExpectations(t)
	})

	t.Run("new password too short returns ErrPasswordTooShort", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		passwordHasher := new(mocks.MockPasswordHasher)
		svc := newSvc(userRepo, passwordHasher)

		userID := uuid.New()
		user := activeUserWithRole(userID)
		user.PasswordHash = "current-hash"

		userRepo.On("GetByID", ctx, userID).Return(user, nil)
		passwordHasher.On("CheckPassword", "current-pass", "current-hash").Return(true)

		err := svc.ChangePassword(ctx, userID, "current-pass", "short")

		assert.ErrorIs(t, err, application.ErrPasswordTooShort)
		userRepo.AssertExpectations(t)
		passwordHasher.AssertExpectations(t)
	})

	t.Run("new password same as old returns ErrPasswordSameAsOld", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		passwordHasher := new(mocks.MockPasswordHasher)
		svc := newSvc(userRepo, passwordHasher)

		userID := uuid.New()
		user := activeUserWithRole(userID)
		user.PasswordHash = "current-hash"

		userRepo.On("GetByID", ctx, userID).Return(user, nil)
		passwordHasher.On("CheckPassword", "same-password", "current-hash").Return(true)

		err := svc.ChangePassword(ctx, userID, "same-password", "same-password")

		assert.ErrorIs(t, err, application.ErrPasswordSameAsOld)
		userRepo.AssertExpectations(t)
		passwordHasher.AssertExpectations(t)
	})

	t.Run("user not found returns error", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		passwordHasher := new(mocks.MockPasswordHasher)
		svc := newSvc(userRepo, passwordHasher)

		userID := uuid.New()
		userRepo.On("GetByID", ctx, userID).Return(nil, authrepo.ErrUserNotFound)

		err := svc.ChangePassword(ctx, userID, "current-pass", "new-password-123")

		assert.ErrorIs(t, err, authrepo.ErrUserNotFound)
		userRepo.AssertExpectations(t)
	})
}

// ---------- TestCreateUser ----------

func TestCreateUser(t *testing.T) {
	ctx := context.Background()

	newSvc := func(userRepo *mocks.MockUserRepository, passwordHasher *mocks.MockPasswordHasher) *application.AuthService {
		apiKeyRepo := new(mocks.MockAPIKeyRepository)
		refreshRepo := new(mocks.MockRefreshTokenRepository)
		apiKeyGen := new(mocks.MockAPIKeyGenerator)
		tokenService := newTestTokenService(t, refreshRepo)
		return application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)
	}

	t.Run("success returns user with role", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		passwordHasher := new(mocks.MockPasswordHasher)
		svc := newSvc(userRepo, passwordHasher)

		roleID := uuid.New()
		passwordHasher.On("HashPassword", "secure-pass").Return("hashed-pass", nil)
		userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

		expectedUser := &domain.User{
			ID:       uuid.New(),
			Username: "newuser",
			Email:    "new@example.com",
			RoleID:   roleID,
			Active:   true,
			Role:     &domain.Role{ID: roleID, Name: "client"},
		}
		userRepo.On("GetByIDWithRole", ctx, mock.AnythingOfType("uuid.UUID")).Return(expectedUser, nil)

		user, err := svc.CreateUser(ctx, "newuser", "new@example.com", "secure-pass", roleID, true)

		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "newuser", user.Username)
		assert.NotNil(t, user.Role)

		userRepo.AssertExpectations(t)
		passwordHasher.AssertExpectations(t)
	})

	t.Run("hash error returns error", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		passwordHasher := new(mocks.MockPasswordHasher)
		svc := newSvc(userRepo, passwordHasher)

		roleID := uuid.New()
		hashErr := errors.New("hash failure")
		passwordHasher.On("HashPassword", "secure-pass").Return("", hashErr)

		user, err := svc.CreateUser(ctx, "newuser", "new@example.com", "secure-pass", roleID, true)

		assert.ErrorIs(t, err, hashErr)
		assert.Nil(t, user)

		userRepo.AssertExpectations(t)
		passwordHasher.AssertExpectations(t)
	})
}

// ---------- TestUpdateUser ----------

func TestUpdateUser(t *testing.T) {
	ctx := context.Background()

	newSvc := func(userRepo *mocks.MockUserRepository) *application.AuthService {
		apiKeyRepo := new(mocks.MockAPIKeyRepository)
		refreshRepo := new(mocks.MockRefreshTokenRepository)
		passwordHasher := new(mocks.MockPasswordHasher)
		apiKeyGen := new(mocks.MockAPIKeyGenerator)
		tokenService := newTestTokenService(t, refreshRepo)
		return application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)
	}

	t.Run("success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		svc := newSvc(userRepo)

		userID := uuid.New()
		roleID := uuid.New()
		existing := activeUserWithRole(userID)

		updatedUser := &domain.User{
			ID:       userID,
			Username: existing.Username,
			Email:    "updated@example.com",
			RoleID:   roleID,
			Active:   true,
			Role:     &domain.Role{ID: roleID, Name: "operator"},
		}

		userRepo.On("GetByID", ctx, userID).Return(existing, nil)
		userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)
		userRepo.On("GetByIDWithRole", ctx, userID).Return(updatedUser, nil)

		user, err := svc.UpdateUser(ctx, userID, "updated@example.com", roleID, true, nil)

		require.NoError(t, err)
		assert.Equal(t, "updated@example.com", user.Email)

		userRepo.AssertExpectations(t)
	})

	t.Run("user not found returns error", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		svc := newSvc(userRepo)

		userID := uuid.New()
		roleID := uuid.New()
		userRepo.On("GetByID", ctx, userID).Return(nil, authrepo.ErrUserNotFound)

		user, err := svc.UpdateUser(ctx, userID, "updated@example.com", roleID, true, nil)

		assert.ErrorIs(t, err, authrepo.ErrUserNotFound)
		assert.Nil(t, user)

		userRepo.AssertExpectations(t)
	})
}

// ---------- TestDeactivateUser ----------

func TestDeactivateUser(t *testing.T) {
	ctx := context.Background()

	newSvc := func(userRepo *mocks.MockUserRepository) *application.AuthService {
		apiKeyRepo := new(mocks.MockAPIKeyRepository)
		refreshRepo := new(mocks.MockRefreshTokenRepository)
		passwordHasher := new(mocks.MockPasswordHasher)
		apiKeyGen := new(mocks.MockAPIKeyGenerator)
		tokenService := newTestTokenService(t, refreshRepo)
		return application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)
	}

	t.Run("success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		svc := newSvc(userRepo)

		userID := uuid.New()
		userRepo.On("Deactivate", ctx, userID).Return(nil)

		err := svc.DeactivateUser(ctx, userID)

		require.NoError(t, err)
		userRepo.AssertExpectations(t)
	})

	t.Run("error propagated", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		svc := newSvc(userRepo)

		userID := uuid.New()
		deactivateErr := errors.New("db error")
		userRepo.On("Deactivate", ctx, userID).Return(deactivateErr)

		err := svc.DeactivateUser(ctx, userID)

		assert.ErrorIs(t, err, deactivateErr)
		userRepo.AssertExpectations(t)
	})
}

// ---------- TestUpdateAPIKey ----------

func TestUpdateAPIKey(t *testing.T) {
	ctx := context.Background()

	newSvc := func(apiKeyRepo *mocks.MockAPIKeyRepository) *application.AuthService {
		userRepo := new(mocks.MockUserRepository)
		refreshRepo := new(mocks.MockRefreshTokenRepository)
		passwordHasher := new(mocks.MockPasswordHasher)
		apiKeyGen := new(mocks.MockAPIKeyGenerator)
		tokenService := newTestTokenService(t, refreshRepo)
		return application.NewAuthServiceWithDeps(userRepo, apiKeyRepo, tokenService, passwordHasher, apiKeyGen)
	}

	t.Run("success", func(t *testing.T) {
		apiKeyRepo := new(mocks.MockAPIKeyRepository)
		svc := newSvc(apiKeyRepo)

		userID := uuid.New()
		keyID := uuid.New()
		existing := &domain.APIKey{
			ID:     keyID,
			UserID: userID,
			Name:   "Old Name",
			Active: true,
		}

		apiKeyRepo.On("GetByID", ctx, keyID).Return(existing, nil)
		apiKeyRepo.On("Update", ctx, mock.AnythingOfType("*domain.APIKey")).Return(nil)

		scopes := []string{"messages:send"}
		allowedIPs := []string{"10.0.0.1"}
		expiresAt := time.Now().Add(30 * 24 * time.Hour)

		key, err := svc.UpdateAPIKey(ctx, keyID, userID, "New Name", scopes, allowedIPs, &expiresAt)

		require.NoError(t, err)
		assert.Equal(t, "New Name", key.Name)
		assert.Equal(t, scopes, key.Scopes)
		assert.Equal(t, allowedIPs, key.AllowedIPs)

		apiKeyRepo.AssertExpectations(t)
	})

	t.Run("key not owned returns ErrAPIKeyNotOwned", func(t *testing.T) {
		apiKeyRepo := new(mocks.MockAPIKeyRepository)
		svc := newSvc(apiKeyRepo)

		userID := uuid.New()
		otherUserID := uuid.New()
		keyID := uuid.New()
		existing := &domain.APIKey{
			ID:     keyID,
			UserID: otherUserID,
			Name:   "Key",
			Active: true,
		}

		apiKeyRepo.On("GetByID", ctx, keyID).Return(existing, nil)

		key, err := svc.UpdateAPIKey(ctx, keyID, userID, "New Name", nil, nil, nil)

		assert.ErrorIs(t, err, application.ErrAPIKeyNotOwned)
		assert.Nil(t, key)

		apiKeyRepo.AssertExpectations(t)
	})

	t.Run("key revoked returns ErrAPIKeyRevoked", func(t *testing.T) {
		apiKeyRepo := new(mocks.MockAPIKeyRepository)
		svc := newSvc(apiKeyRepo)

		userID := uuid.New()
		keyID := uuid.New()
		existing := &domain.APIKey{
			ID:     keyID,
			UserID: userID,
			Name:   "Key",
			Active: false, // revoked
		}

		apiKeyRepo.On("GetByID", ctx, keyID).Return(existing, nil)

		key, err := svc.UpdateAPIKey(ctx, keyID, userID, "New Name", nil, nil, nil)

		assert.ErrorIs(t, err, application.ErrAPIKeyRevoked)
		assert.Nil(t, key)

		apiKeyRepo.AssertExpectations(t)
	})
}

// ---------- TestRoleManagement ----------

func TestRoleManagement(t *testing.T) {
	ctx := context.Background()

	newSvc := func(userRepo *mocks.MockUserRepository, roleRepo *mocks.MockRoleRepository) *application.AuthService {
		apiKeyRepo := new(mocks.MockAPIKeyRepository)
		refreshRepo := new(mocks.MockRefreshTokenRepository)
		tokenService := newTestTokenService(t, refreshRepo)
		return application.NewAuthService(userRepo, apiKeyRepo, tokenService, roleRepo)
	}

	t.Run("CreateRole success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		roleRepo := new(mocks.MockRoleRepository)
		svc := newSvc(userRepo, roleRepo)

		permID := uuid.New()
		expectedRole := &domain.Role{
			ID:          uuid.New(),
			Name:        "editor",
			Description: "Can edit content",
		}

		roleRepo.On("Create", ctx, "editor", "Can edit content", []uuid.UUID{permID}).Return(expectedRole, nil)

		role, err := svc.CreateRole(ctx, "editor", "Can edit content", []uuid.UUID{permID})

		require.NoError(t, err)
		assert.Equal(t, "editor", role.Name)
		assert.Equal(t, "Can edit content", role.Description)

		roleRepo.AssertExpectations(t)
	})

	t.Run("ListRoles success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		roleRepo := new(mocks.MockRoleRepository)
		svc := newSvc(userRepo, roleRepo)

		expectedRoles := []*domain.Role{
			{ID: uuid.New(), Name: "admin"},
			{ID: uuid.New(), Name: "client"},
		}

		roleRepo.On("List", ctx, int32(10), int32(0)).Return(expectedRoles, int32(2), nil)

		roles, total, err := svc.ListRoles(ctx, 10, 0)

		require.NoError(t, err)
		assert.Len(t, roles, 2)
		assert.Equal(t, int32(2), total)
		assert.Equal(t, "admin", roles[0].Name)

		roleRepo.AssertExpectations(t)
	})

	t.Run("GetRole success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		roleRepo := new(mocks.MockRoleRepository)
		svc := newSvc(userRepo, roleRepo)

		roleID := uuid.New()
		expectedRole := &domain.Role{
			ID:   roleID,
			Name: "admin",
			Permissions: []domain.Permission{
				{ID: uuid.New(), Resource: "users", Action: "read"},
			},
		}

		roleRepo.On("GetByIDWithPermissions", ctx, roleID).Return(expectedRole, nil)
		roleRepo.On("GetUserCount", ctx, roleID).Return(int32(5), nil)

		role, err := svc.GetRole(ctx, roleID)

		require.NoError(t, err)
		assert.Equal(t, roleID, role.ID)
		assert.Equal(t, "admin", role.Name)
		assert.Equal(t, int32(5), role.UserCount)
		assert.Len(t, role.Permissions, 1)

		roleRepo.AssertExpectations(t)
	})
}
