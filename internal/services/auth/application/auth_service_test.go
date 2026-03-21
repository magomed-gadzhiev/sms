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
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			user := activeUserWithRole(userID)

			apiKeyRepo.On("GetByKeyHash", ctx, keyHash).Return(apiKey, nil)
			apiKeyRepo.On("UpdateLastUsed", ctx, keyID).Return(nil)
			userRepo.On("GetByIDWithRole", ctx, userID).Return(user, nil)

			resultUser, err := svc.AuthenticateByAPIKey(ctx, rawKey)

			require.NoError(t, err)
			assert.Equal(t, userID, resultUser.ID)

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

			resultUser, err := svc.AuthenticateByAPIKey(ctx, rawKey)

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

			resultUser, err := svc.AuthenticateByAPIKey(ctx, rawKey)

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

			resultUser, err := svc.AuthenticateByAPIKey(ctx, rawKey)

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

			resultUser, err := svc.AuthenticateByAPIKey(ctx, rawKey, "192.168.1.1")

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

			resultUser, err := svc.AuthenticateByAPIKey(ctx, rawKey)

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
			apiKeyRepo.On("Revoke", ctx, keyID).Return(nil)

			err := svc.RevokeAPIKey(ctx, keyID)

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
			apiKeyRepo.On("Revoke", ctx, keyID).Return(authrepo.ErrAPIKeyNotFound)

			err := svc.RevokeAPIKey(ctx, keyID)

			assert.ErrorIs(t, err, authrepo.ErrAPIKeyNotFound)
			apiKeyRepo.AssertExpectations(t)
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
