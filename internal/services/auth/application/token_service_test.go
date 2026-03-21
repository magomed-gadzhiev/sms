package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/auth/application"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/services/auth/mocks"
)

func TestTokenService(t *testing.T) {
	ctx := context.Background()

	t.Run("GenerateTokenPair", func(t *testing.T) {

		t.Run("valid claims returns JWT and refresh token", func(t *testing.T) {
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			svc := application.NewTokenService(
				"test-secret-key",
				15*time.Minute,
				7*24*time.Hour,
				refreshRepo,
			)

			userID := uuid.New()
			refreshRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

			accessToken, refreshToken, err := svc.GenerateTokenPair(ctx, userID, "admin")

			require.NoError(t, err)
			assert.NotEmpty(t, accessToken)
			assert.NotEmpty(t, refreshToken)

			// Verify the access token can be parsed and contains correct claims
			claims, err := svc.ValidateToken(accessToken)
			require.NoError(t, err)
			assert.Equal(t, userID.String(), claims.UserID)
			assert.Equal(t, "admin", claims.Role)

			refreshRepo.AssertExpectations(t)
		})

		t.Run("refresh token repo error propagates", func(t *testing.T) {
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			svc := application.NewTokenService(
				"test-secret-key",
				15*time.Minute,
				7*24*time.Hour,
				refreshRepo,
			)

			userID := uuid.New()
			refreshRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).
				Return(assert.AnError)

			accessToken, refreshToken, err := svc.GenerateTokenPair(ctx, userID, "admin")

			assert.Error(t, err)
			assert.Empty(t, accessToken)
			assert.Empty(t, refreshToken)

			refreshRepo.AssertExpectations(t)
		})
	})

	t.Run("ValidateToken", func(t *testing.T) {

		t.Run("valid token returns claims", func(t *testing.T) {
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			svc := application.NewTokenService(
				"test-secret-key",
				15*time.Minute,
				7*24*time.Hour,
				refreshRepo,
			)

			userID := uuid.New()
			refreshRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

			accessToken, _, err := svc.GenerateTokenPair(ctx, userID, "client")
			require.NoError(t, err)

			claims, err := svc.ValidateToken(accessToken)

			require.NoError(t, err)
			assert.Equal(t, userID.String(), claims.UserID)
			assert.Equal(t, "client", claims.Role)

			refreshRepo.AssertExpectations(t)
		})

		t.Run("expired token returns ErrExpiredToken", func(t *testing.T) {
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			// Create service with very short expiry
			svc := application.NewTokenService(
				"test-secret-key",
				-1*time.Second, // already expired
				7*24*time.Hour,
				refreshRepo,
			)

			userID := uuid.New()
			refreshRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

			accessToken, _, err := svc.GenerateTokenPair(ctx, userID, "admin")
			require.NoError(t, err)

			_, err = svc.ValidateToken(accessToken)

			assert.ErrorIs(t, err, application.ErrExpiredToken)

			refreshRepo.AssertExpectations(t)
		})

		t.Run("invalid token string returns ErrInvalidToken", func(t *testing.T) {
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			svc := application.NewTokenService(
				"test-secret-key",
				15*time.Minute,
				7*24*time.Hour,
				refreshRepo,
			)

			_, err := svc.ValidateToken("not-a-valid-jwt")

			assert.ErrorIs(t, err, application.ErrInvalidToken)
		})

		t.Run("wrong secret returns ErrInvalidToken", func(t *testing.T) {
			refreshRepo := new(mocks.MockRefreshTokenRepository)

			svcSigner := application.NewTokenService(
				"secret-A",
				15*time.Minute,
				7*24*time.Hour,
				refreshRepo,
			)
			svcValidator := application.NewTokenService(
				"secret-B",
				15*time.Minute,
				7*24*time.Hour,
				refreshRepo,
			)

			userID := uuid.New()
			refreshRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

			accessToken, _, err := svcSigner.GenerateTokenPair(ctx, userID, "admin")
			require.NoError(t, err)

			_, err = svcValidator.ValidateToken(accessToken)

			assert.ErrorIs(t, err, application.ErrInvalidToken)

			refreshRepo.AssertExpectations(t)
		})

		t.Run("token with wrong signing method returns ErrInvalidToken", func(t *testing.T) {
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			svc := application.NewTokenService(
				"test-secret-key",
				15*time.Minute,
				7*24*time.Hour,
				refreshRepo,
			)

			// Create a token with "none" signing method
			claims := application.TokenClaims{
				UserID: uuid.New().String(),
				Role:   "admin",
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
			}
			token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
			tokenStr, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
			require.NoError(t, err)

			_, err = svc.ValidateToken(tokenStr)

			assert.ErrorIs(t, err, application.ErrInvalidToken)
		})
	})

	t.Run("RefreshToken", func(t *testing.T) {

		t.Run("valid refresh token returns new token pair", func(t *testing.T) {
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			svc := application.NewTokenService(
				"test-secret-key",
				15*time.Minute,
				7*24*time.Hour,
				refreshRepo,
			)

			userID := uuid.New()
			// First generate a token pair
			refreshRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

			_, refreshTokenStr, err := svc.GenerateTokenPair(ctx, userID, "admin")
			require.NoError(t, err)

			// Now set up for refresh: GetByTokenHash, Revoke, Create (for new pair)
			validRefreshToken := &domain.RefreshToken{
				ID:        uuid.New(),
				UserID:    userID,
				ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
				Revoked:   false,
				CreatedAt: time.Now(),
			}
			refreshRepo.On("GetByTokenHash", ctx, mock.AnythingOfType("string")).Return(validRefreshToken, nil)
			refreshRepo.On("Revoke", ctx, mock.AnythingOfType("string")).Return(nil)

			getUserRole := func(id uuid.UUID) (string, error) {
				return "admin", nil
			}

			newAccess, newRefresh, err := svc.RefreshToken(ctx, refreshTokenStr, getUserRole)

			require.NoError(t, err)
			assert.NotEmpty(t, newAccess)
			assert.NotEmpty(t, newRefresh)

			refreshRepo.AssertExpectations(t)
		})

		t.Run("refresh token not found returns ErrInvalidToken", func(t *testing.T) {
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			svc := application.NewTokenService(
				"test-secret-key",
				15*time.Minute,
				7*24*time.Hour,
				refreshRepo,
			)

			refreshRepo.On("GetByTokenHash", ctx, mock.AnythingOfType("string")).
				Return(nil, authrepo.ErrRefreshTokenNotFound)

			getUserRole := func(id uuid.UUID) (string, error) {
				return "admin", nil
			}

			_, _, err := svc.RefreshToken(ctx, "nonexistent-token", getUserRole)

			assert.ErrorIs(t, err, application.ErrInvalidToken)

			refreshRepo.AssertExpectations(t)
		})

		t.Run("revoked refresh token returns ErrInvalidToken", func(t *testing.T) {
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			svc := application.NewTokenService(
				"test-secret-key",
				15*time.Minute,
				7*24*time.Hour,
				refreshRepo,
			)

			revokedToken := &domain.RefreshToken{
				ID:        uuid.New(),
				UserID:    uuid.New(),
				ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
				Revoked:   true, // revoked
				CreatedAt: time.Now(),
			}
			refreshRepo.On("GetByTokenHash", ctx, mock.AnythingOfType("string")).Return(revokedToken, nil)

			getUserRole := func(id uuid.UUID) (string, error) {
				return "admin", nil
			}

			_, _, err := svc.RefreshToken(ctx, "some-revoked-token", getUserRole)

			assert.ErrorIs(t, err, application.ErrInvalidToken)

			refreshRepo.AssertExpectations(t)
		})
	})

	t.Run("RevokeRefreshToken", func(t *testing.T) {

		t.Run("success", func(t *testing.T) {
			refreshRepo := new(mocks.MockRefreshTokenRepository)
			svc := application.NewTokenService(
				"test-secret-key",
				15*time.Minute,
				7*24*time.Hour,
				refreshRepo,
			)

			refreshRepo.On("Revoke", ctx, mock.AnythingOfType("string")).Return(nil)

			err := svc.RevokeRefreshToken(ctx, "some-refresh-token")

			require.NoError(t, err)
			refreshRepo.AssertExpectations(t)
		})
	})
}
