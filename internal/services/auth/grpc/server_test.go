package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/services/auth/application"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
)

// --- Mocks ---

type mockUserRepo struct {
	mock.Mock
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *mockUserRepo) GetByIDWithRole(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *mockUserRepo) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

type mockAPIKeyRepo struct {
	mock.Mock
}

func (m *mockAPIKeyRepo) Create(ctx context.Context, apiKey *domain.APIKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}

func (m *mockAPIKeyRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *mockAPIKeyRepo) GetByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	args := m.Called(ctx, keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *mockAPIKeyRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

func (m *mockAPIKeyRepo) Update(ctx context.Context, apiKey *domain.APIKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}

func (m *mockAPIKeyRepo) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockAPIKeyRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockRefreshTokenRepo struct {
	mock.Mock
}

func (m *mockRefreshTokenRepo) Create(ctx context.Context, token *domain.RefreshToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *mockRefreshTokenRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	args := m.Called(ctx, tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}

func (m *mockRefreshTokenRepo) Revoke(ctx context.Context, tokenHash string) error {
	args := m.Called(ctx, tokenHash)
	return args.Error(0)
}

func (m *mockRefreshTokenRepo) RevokeByUserID(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

type mockPasswordHasher struct {
	mock.Mock
}

func (m *mockPasswordHasher) HashPassword(password string) (string, error) {
	args := m.Called(password)
	return args.String(0), args.Error(1)
}

func (m *mockPasswordHasher) CheckPassword(password, hash string) bool {
	args := m.Called(password, hash)
	return args.Bool(0)
}

type mockAPIKeyGenerator struct {
	mock.Mock
}

func (m *mockAPIKeyGenerator) GenerateAPIKey() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *mockAPIKeyGenerator) GetKeyPrefix(key string) string {
	args := m.Called(key)
	return args.String(0)
}

// --- Helpers ---

func newTestUser() *domain.User {
	return &domain.User{
		ID:           uuid.New(),
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		Active:       true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Role: &domain.Role{
			ID:          uuid.New(),
			Name:        "admin",
			Description: "Administrator",
		},
	}
}

func newAuthServer(
	userRepo *mockUserRepo,
	apiKeyRepo *mockAPIKeyRepo,
	refreshTokenRepo *mockRefreshTokenRepo,
	passwordHasher *mockPasswordHasher,
	apiKeyGenerator *mockAPIKeyGenerator,
) *Server {
	tokenService := application.NewTokenService(
		"test-secret-key-that-is-long-enough-32",
		time.Hour,
		24*time.Hour,
		refreshTokenRepo,
	)
	authService := application.NewAuthServiceWithDeps(
		userRepo,
		apiKeyRepo,
		tokenService,
		passwordHasher,
		apiKeyGenerator,
	)

	// We pass nil for fields that depend on concrete infra types (sessionManager, totpService, etc.)
	// We only test methods that don't use them.
	return &Server{
		authService:  authService,
		tokenService: tokenService,
	}
}

// --- Tests ---

func TestAuthServer_Authenticate(t *testing.T) {
	t.Run("valid credentials returns OK with tokens", func(t *testing.T) {
		userRepo := new(mockUserRepo)
		apiKeyRepo := new(mockAPIKeyRepo)
		refreshTokenRepo := new(mockRefreshTokenRepo)
		passwordHasher := new(mockPasswordHasher)
		apiKeyGenerator := new(mockAPIKeyGenerator)

		srv := newAuthServer(userRepo, apiKeyRepo, refreshTokenRepo, passwordHasher, apiKeyGenerator)

		user := newTestUser()

		// AuthenticateByCredentials first tries GetByUsername, then GetByIDWithRole
		userRepo.On("GetByUsername", mock.Anything, "testuser").Return(user, nil)
		passwordHasher.On("CheckPassword", "password123", "hashed_password").Return(true)
		userRepo.On("GetByIDWithRole", mock.Anything, user.ID).Return(user, nil)
		refreshTokenRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

		resp, err := srv.Authenticate(context.Background(), &authv1.AuthenticateRequest{
			Username: "testuser",
			Password: "password123",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEmpty(t, resp.RefreshToken)
		assert.NotNil(t, resp.User)
		assert.Equal(t, user.ID.String(), resp.User.Id)
		assert.Equal(t, "testuser", resp.User.Username)

		userRepo.AssertExpectations(t)
		passwordHasher.AssertExpectations(t)
	})

	t.Run("invalid password returns Unauthenticated", func(t *testing.T) {
		userRepo := new(mockUserRepo)
		apiKeyRepo := new(mockAPIKeyRepo)
		refreshTokenRepo := new(mockRefreshTokenRepo)
		passwordHasher := new(mockPasswordHasher)
		apiKeyGenerator := new(mockAPIKeyGenerator)

		srv := newAuthServer(userRepo, apiKeyRepo, refreshTokenRepo, passwordHasher, apiKeyGenerator)

		user := newTestUser()

		userRepo.On("GetByUsername", mock.Anything, "testuser").Return(user, nil)
		passwordHasher.On("CheckPassword", "wrong_password", "hashed_password").Return(false)

		resp, err := srv.Authenticate(context.Background(), &authv1.AuthenticateRequest{
			Username: "testuser",
			Password: "wrong_password",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
		assert.Contains(t, st.Message(), "invalid credentials")
	})

	t.Run("inactive user returns PermissionDenied", func(t *testing.T) {
		userRepo := new(mockUserRepo)
		apiKeyRepo := new(mockAPIKeyRepo)
		refreshTokenRepo := new(mockRefreshTokenRepo)
		passwordHasher := new(mockPasswordHasher)
		apiKeyGenerator := new(mockAPIKeyGenerator)

		srv := newAuthServer(userRepo, apiKeyRepo, refreshTokenRepo, passwordHasher, apiKeyGenerator)

		user := newTestUser()
		user.Active = false

		userRepo.On("GetByUsername", mock.Anything, "testuser").Return(user, nil)
		passwordHasher.On("CheckPassword", "password123", "hashed_password").Return(true)

		resp, err := srv.Authenticate(context.Background(), &authv1.AuthenticateRequest{
			Username: "testuser",
			Password: "password123",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.PermissionDenied, st.Code())
		assert.Contains(t, st.Message(), "inactive")
	})

	t.Run("no credentials returns InvalidArgument", func(t *testing.T) {
		userRepo := new(mockUserRepo)
		apiKeyRepo := new(mockAPIKeyRepo)
		refreshTokenRepo := new(mockRefreshTokenRepo)
		passwordHasher := new(mockPasswordHasher)
		apiKeyGenerator := new(mockAPIKeyGenerator)

		srv := newAuthServer(userRepo, apiKeyRepo, refreshTokenRepo, passwordHasher, apiKeyGenerator)

		resp, err := srv.Authenticate(context.Background(), &authv1.AuthenticateRequest{})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid API key returns Unauthenticated", func(t *testing.T) {
		userRepo := new(mockUserRepo)
		apiKeyRepo := new(mockAPIKeyRepo)
		refreshTokenRepo := new(mockRefreshTokenRepo)
		passwordHasher := new(mockPasswordHasher)
		apiKeyGenerator := new(mockAPIKeyGenerator)

		srv := newAuthServer(userRepo, apiKeyRepo, refreshTokenRepo, passwordHasher, apiKeyGenerator)

		// AuthService.AuthenticateByAPIKey hashes the key and looks it up.
		// When not found, it returns ErrAPIKeyInvalid (api key not found sentinel from repo).
		// We need to return the sentinel error from the repo package.
		apiKeyRepo.On("GetByKeyHash", mock.Anything, mock.AnythingOfType("string")).
			Return(nil, application.ErrAPIKeyNotFound)

		resp, err := srv.Authenticate(context.Background(), &authv1.AuthenticateRequest{
			ApiKey: "invalid-api-key",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})
}

func TestAuthServer_ValidateToken(t *testing.T) {
	t.Run("empty token returns valid=false without error", func(t *testing.T) {
		userRepo := new(mockUserRepo)
		apiKeyRepo := new(mockAPIKeyRepo)
		refreshTokenRepo := new(mockRefreshTokenRepo)
		passwordHasher := new(mockPasswordHasher)
		apiKeyGenerator := new(mockAPIKeyGenerator)

		srv := newAuthServer(userRepo, apiKeyRepo, refreshTokenRepo, passwordHasher, apiKeyGenerator)

		resp, err := srv.ValidateToken(context.Background(), &authv1.ValidateTokenRequest{
			Token: "",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.False(t, resp.Valid)
		assert.Equal(t, "token is required", resp.Error)
	})

	t.Run("invalid token returns valid=false", func(t *testing.T) {
		userRepo := new(mockUserRepo)
		apiKeyRepo := new(mockAPIKeyRepo)
		refreshTokenRepo := new(mockRefreshTokenRepo)
		passwordHasher := new(mockPasswordHasher)
		apiKeyGenerator := new(mockAPIKeyGenerator)

		srv := newAuthServer(userRepo, apiKeyRepo, refreshTokenRepo, passwordHasher, apiKeyGenerator)

		// ValidateToken first tries JWT validation (will fail), then tries API key auth (will fail too)
		apiKeyRepo.On("GetByKeyHash", mock.Anything, mock.AnythingOfType("string")).
			Return(nil, errors.New("not found"))

		resp, err := srv.ValidateToken(context.Background(), &authv1.ValidateTokenRequest{
			Token: "invalid-token-string",
		})

		require.NoError(t, err) // ValidateToken returns response with valid=false, no gRPC error
		require.NotNil(t, resp)
		assert.False(t, resp.Valid)
		assert.Equal(t, "invalid token", resp.Error)
	})

	t.Run("valid JWT token returns valid=true with user info", func(t *testing.T) {
		userRepo := new(mockUserRepo)
		apiKeyRepo := new(mockAPIKeyRepo)
		refreshTokenRepo := new(mockRefreshTokenRepo)
		passwordHasher := new(mockPasswordHasher)
		apiKeyGenerator := new(mockAPIKeyGenerator)

		srv := newAuthServer(userRepo, apiKeyRepo, refreshTokenRepo, passwordHasher, apiKeyGenerator)

		user := newTestUser()

		// First generate a valid token pair
		refreshTokenRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)
		accessToken, _, err := srv.tokenService.GenerateTokenPair(context.Background(), user.ID, "admin")
		require.NoError(t, err)

		// Now validate it
		userRepo.On("GetByIDWithRole", mock.Anything, user.ID).Return(user, nil)

		resp, err := srv.ValidateToken(context.Background(), &authv1.ValidateTokenRequest{
			Token: accessToken,
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Valid)
		assert.NotNil(t, resp.User)
		assert.Equal(t, user.ID.String(), resp.User.Id)
	})
}
