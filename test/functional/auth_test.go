//go:build functional

package functional_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/auth/application"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	authinfra "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

// adminRoleID is the well-known UUID seeded by migration 000003.
const adminRoleID = "00000000-0000-0000-0000-000000000001"

// setupAuthDB creates a *database.DB wrapper that auth repositories expect,
// and registers cleanup to close it.
func setupAuthDB(t *testing.T) *database.DB {
	t.Helper()

	db, err := database.NewDB(testDSN())
	if err != nil {
		t.Fatalf("failed to open auth test DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// cleanupAuthTables truncates auth tables so tests do not collide.
func cleanupAuthTables(t *testing.T, db *database.DB) {
	t.Helper()

	ctx := context.Background()
	tables := []string{
		"api_key_scopes",
		"api_keys",
		"refresh_tokens",
		"users",
	}
	for _, tbl := range tables {
		if _, err := db.ExecContext(ctx, "DELETE FROM "+tbl); err != nil {
			t.Logf("cleanup: failed to delete from %s: %v", tbl, err)
		}
	}
}

// createTestUser hashes the password and inserts a user in the DB,
// returning the domain.User with the ID set.
func createTestUser(t *testing.T, userRepo *authrepo.UserRepository, username, email, password string, roleID uuid.UUID) *domain.User {
	t.Helper()

	hash, err := authinfra.HashPassword(password)
	require.NoError(t, err, "hashing password")

	now := time.Now()
	user := &domain.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		RoleID:       roleID,
		Active:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = userRepo.Create(context.Background(), user)
	require.NoError(t, err, "creating test user")
	return user
}

func TestAuthChain(t *testing.T) {
	skipIfNoDB(t)

	db := setupAuthDB(t)

	// Always clean up tables after test completes.
	t.Cleanup(func() { cleanupAuthTables(t, db) })
	// Also clean at the start in case of leftover data from a previous failed run.
	cleanupAuthTables(t, db)

	roleID := uuid.MustParse(adminRoleID)

	// Create repositories using the real DB.
	userRepo := authrepo.NewUserRepository(db)
	apiKeyRepo := authrepo.NewAPIKeyRepository(db)
	refreshTokenRepo := authrepo.NewRefreshTokenRepository(db)

	// Create services.
	jwtSecret := "func-test-jwt-secret-32bytes!!"
	tokenService := application.NewTokenService(
		jwtSecret,
		15*time.Minute, // access token expiry
		24*time.Hour,   // refresh token expiry
		refreshTokenRepo,
	)

	authService := application.NewAuthService(userRepo, apiKeyRepo, tokenService)

	t.Run("LoginWithCredentials", func(t *testing.T) {
		password := "S3cureP@ss!"
		user := createTestUser(t, userRepo, "loginuser", "loginuser@test.io", password, roleID)

		ctx := context.Background()

		// Authenticate with valid credentials.
		authedUser, accessToken, refreshToken, err := authService.AuthenticateByCredentials(ctx, "loginuser", password)
		require.NoError(t, err, "authenticate with valid credentials")
		require.NotNil(t, authedUser)
		assert.Equal(t, user.ID, authedUser.ID)
		assert.NotEmpty(t, accessToken, "access token must not be empty")
		assert.NotEmpty(t, refreshToken, "refresh token must not be empty")

		// Role must be loaded.
		require.NotNil(t, authedUser.Role, "role must be loaded")
		assert.Equal(t, "admin", authedUser.Role.Name)

		// Validate the access token.
		claims, err := tokenService.ValidateToken(accessToken)
		require.NoError(t, err, "validate access token")
		assert.Equal(t, user.ID.String(), claims.UserID)
		assert.Equal(t, "admin", claims.Role)

		// Wrong password must fail.
		_, _, _, err = authService.AuthenticateByCredentials(ctx, "loginuser", "wrongpassword")
		assert.ErrorIs(t, err, application.ErrInvalidCredentials, "wrong password must return ErrInvalidCredentials")

		// Non-existent user must fail.
		_, _, _, err = authService.AuthenticateByCredentials(ctx, "nouser", "anything")
		assert.ErrorIs(t, err, application.ErrInvalidCredentials, "non-existent user must return ErrInvalidCredentials")
	})

	t.Run("RefreshToken", func(t *testing.T) {
		password := "R3fresh!Pass"
		user := createTestUser(t, userRepo, "refreshuser", "refreshuser@test.io", password, roleID)

		ctx := context.Background()

		// Authenticate to get initial tokens.
		_, accessToken1, refreshTokenStr, err := authService.AuthenticateByCredentials(ctx, "refreshuser", password)
		require.NoError(t, err, "initial authentication")
		require.NotEmpty(t, accessToken1)
		require.NotEmpty(t, refreshTokenStr)

		// Refresh the token pair.
		getUserRole := func(id uuid.UUID) (string, error) {
			u, err := userRepo.GetByIDWithRole(ctx, id)
			if err != nil {
				return "", err
			}
			return u.Role.Name, nil
		}

		accessToken2, refreshToken2, err := tokenService.RefreshToken(ctx, refreshTokenStr, getUserRole)
		require.NoError(t, err, "refresh token")
		require.NotEmpty(t, accessToken2, "new access token must not be empty")
		require.NotEmpty(t, refreshToken2, "new refresh token must not be empty")

		// New access token must be valid and carry the same user ID.
		claims, err := tokenService.ValidateToken(accessToken2)
		require.NoError(t, err, "validate refreshed access token")
		assert.Equal(t, user.ID.String(), claims.UserID)

		// Old refresh token must be revoked and not reusable.
		_, _, err = tokenService.RefreshToken(ctx, refreshTokenStr, getUserRole)
		assert.Error(t, err, "reusing old refresh token must fail")
	})

	t.Run("APIKeyAuth", func(t *testing.T) {
		password := "AP1K3y!P@ss"
		user := createTestUser(t, userRepo, "apikeyuser", "apikeyuser@test.io", password, roleID)

		ctx := context.Background()

		// Create an API key with specific scopes.
		scopes := []string{"messages:read", "messages:write"}
		apiKeyObj, rawKey, err := authService.CreateAPIKey(ctx, user.ID, "test-key", nil, scopes, nil)
		require.NoError(t, err, "creating API key")
		require.NotEmpty(t, rawKey, "raw API key must not be empty")
		require.NotNil(t, apiKeyObj)

		// Authenticate by API key.
		authedUser, authedScopes, err := authService.AuthenticateByAPIKey(ctx, rawKey)
		require.NoError(t, err, "authenticate by API key")
		require.NotNil(t, authedUser)
		assert.Equal(t, user.ID, authedUser.ID)
		assert.ElementsMatch(t, scopes, authedScopes, "scopes must be returned to caller")

		// Role and permissions must be loaded.
		require.NotNil(t, authedUser.Role, "role must be loaded on API key auth")
		assert.Equal(t, "admin", authedUser.Role.Name)
		assert.True(t, len(authedUser.Permissions) > 0, "permissions must be loaded")

		// Verify the API key has expected scopes stored.
		storedKey, err := apiKeyRepo.GetByID(ctx, apiKeyObj.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, scopes, storedKey.Scopes)

		// Revoke the key and verify it can no longer authenticate.
		err = authService.RevokeAPIKey(ctx, apiKeyObj.ID, user.ID)
		require.NoError(t, err, "revoking API key")

		_, _, err = authService.AuthenticateByAPIKey(ctx, rawKey)
		assert.ErrorIs(t, err, application.ErrAPIKeyInvalid, "revoked key must fail")

		// Authenticate with a made-up key must fail.
		_, _, err = authService.AuthenticateByAPIKey(ctx, "sk_live_boguskey0000000000000000000")
		assert.ErrorIs(t, err, application.ErrAPIKeyInvalid, "invalid key must return ErrAPIKeyInvalid")
	})
}
