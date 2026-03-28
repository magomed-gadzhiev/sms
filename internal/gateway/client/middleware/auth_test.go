package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

// mockAuthServiceClient реализует authv1.AuthServiceClient для тестов
type mockAuthServiceClient struct {
	mock.Mock
}

func (m *mockAuthServiceClient) Authenticate(ctx context.Context, in *authv1.AuthenticateRequest, opts ...grpc.CallOption) (*authv1.AuthenticateResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.AuthenticateResponse), args.Error(1)
}

func (m *mockAuthServiceClient) ValidateToken(ctx context.Context, in *authv1.ValidateTokenRequest, opts ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.ValidateTokenResponse), args.Error(1)
}

func (m *mockAuthServiceClient) RefreshToken(ctx context.Context, in *authv1.RefreshTokenRequest, opts ...grpc.CallOption) (*authv1.RefreshTokenResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.RefreshTokenResponse), args.Error(1)
}

func (m *mockAuthServiceClient) GetPermissions(ctx context.Context, in *authv1.GetPermissionsRequest, opts ...grpc.CallOption) (*authv1.GetPermissionsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.GetPermissionsResponse), args.Error(1)
}

func (m *mockAuthServiceClient) CreateAPIKey(ctx context.Context, in *authv1.CreateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.CreateAPIKeyResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.CreateAPIKeyResponse), args.Error(1)
}

func (m *mockAuthServiceClient) RevokeAPIKey(ctx context.Context, in *authv1.RevokeAPIKeyRequest, opts ...grpc.CallOption) (*authv1.RevokeAPIKeyResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.RevokeAPIKeyResponse), args.Error(1)
}

func (m *mockAuthServiceClient) ListAPIKeys(ctx context.Context, in *authv1.ListAPIKeysRequest, opts ...grpc.CallOption) (*authv1.ListAPIKeysResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.ListAPIKeysResponse), args.Error(1)
}

func (m *mockAuthServiceClient) SetupTOTP(ctx context.Context, in *authv1.SetupTOTPRequest, opts ...grpc.CallOption) (*authv1.SetupTOTPResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.SetupTOTPResponse), args.Error(1)
}

func (m *mockAuthServiceClient) VerifyTOTP(ctx context.Context, in *authv1.VerifyTOTPRequest, opts ...grpc.CallOption) (*authv1.VerifyTOTPResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.VerifyTOTPResponse), args.Error(1)
}

func (m *mockAuthServiceClient) DisableTOTP(ctx context.Context, in *authv1.DisableTOTPRequest, opts ...grpc.CallOption) (*authv1.DisableTOTPResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.DisableTOTPResponse), args.Error(1)
}

func (m *mockAuthServiceClient) RequestPasswordReset(ctx context.Context, in *authv1.RequestPasswordResetRequest, opts ...grpc.CallOption) (*authv1.RequestPasswordResetResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.RequestPasswordResetResponse), args.Error(1)
}

func (m *mockAuthServiceClient) ResetPassword(ctx context.Context, in *authv1.ResetPasswordRequest, opts ...grpc.CallOption) (*authv1.ResetPasswordResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.ResetPasswordResponse), args.Error(1)
}

func (m *mockAuthServiceClient) LoginWithSession(ctx context.Context, in *authv1.LoginWithSessionRequest, opts ...grpc.CallOption) (*authv1.LoginWithSessionResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.LoginWithSessionResponse), args.Error(1)
}

func (m *mockAuthServiceClient) ValidateSession(ctx context.Context, in *authv1.ValidateSessionRequest, opts ...grpc.CallOption) (*authv1.ValidateSessionResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.ValidateSessionResponse), args.Error(1)
}

func (m *mockAuthServiceClient) Logout(ctx context.Context, in *authv1.LogoutRequest, opts ...grpc.CallOption) (*authv1.LogoutResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.LogoutResponse), args.Error(1)
}

func (m *mockAuthServiceClient) UpdateAPIKey(ctx context.Context, in *authv1.UpdateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.UpdateAPIKeyResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) ChangePassword(ctx context.Context, in *authv1.ChangePasswordRequest, opts ...grpc.CallOption) (*authv1.ChangePasswordResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) RegisterClient(ctx context.Context, in *authv1.RegisterClientRequest, opts ...grpc.CallOption) (*authv1.RegisterClientResponse, error) {
	return nil, nil
}

// nextHandlerRecorder записывает, был ли вызван следующий обработчик, и сохраняет контекст
type nextHandlerRecorder struct {
	called bool
	ctx    context.Context
}

func (h *nextHandlerRecorder) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.called = true
		h.ctx = r.Context()
		w.WriteHeader(http.StatusOK)
	})
}

func TestClientAuthMiddleware(t *testing.T) {
	userID := uuid.New().String()

	t.Run("ValidBearerToken", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, &authv1.ValidateTokenRequest{
			Token: "valid-token",
		}).Return(&authv1.ValidateTokenResponse{
			Valid: true,
			User: &authv1.UserInfo{
				Id:       userID,
				Username: "testclient",
				Email:    "client@example.com",
				Role:     &authv1.Role{Id: "1", Name: "client"},
				Active:   true,
			},
		}, nil)

		authClient.On("GetPermissions", mock.Anything, &authv1.GetPermissionsRequest{
			UserId: userID,
		}).Return(&authv1.GetPermissionsResponse{
			Permissions: []*authv1.Permission{
				{Id: "1", Resource: "messages", Action: "write"},
			},
		}, nil)

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called")
		assert.Equal(t, http.StatusOK, rr.Code)

		// Проверяем контекст
		ctxUserID, ok := GetUserID(recorder.ctx)
		assert.True(t, ok)
		assert.Equal(t, userID, ctxUserID.String())

		ctxClientID, ok := GetClientID(recorder.ctx)
		assert.True(t, ok)
		assert.Equal(t, userID, ctxClientID.String())

		user, ok := GetUser(recorder.ctx)
		assert.True(t, ok)
		assert.Equal(t, "testclient", user.Username)

		assert.True(t, HasPermission(recorder.ctx, "messages", "write"))
		assert.False(t, HasPermission(recorder.ctx, "messages", "delete"))

		authClient.AssertExpectations(t)
	})

	t.Run("ValidXAPIKey", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, &authv1.ValidateTokenRequest{
			Token: "api-key-123",
		}).Return(&authv1.ValidateTokenResponse{
			Valid: true,
			User: &authv1.UserInfo{
				Id:     userID,
				Role:   &authv1.Role{Id: "1", Name: "client"},
				Active: true,
			},
		}, nil)

		authClient.On("GetPermissions", mock.Anything, mock.Anything).
			Return(&authv1.GetPermissionsResponse{}, nil)

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
		req.Header.Set("X-API-Key", "api-key-123")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called")
		assert.Equal(t, http.StatusOK, rr.Code)
		authClient.AssertExpectations(t)
	})

	t.Run("MissingAuthHeader", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		var body map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Contains(t, body, "error")
	})

	t.Run("InvalidToken_ValidationFails", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("token validation failed"))

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("InvalidToken_NotValid", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(&authv1.ValidateTokenResponse{
				Valid: false,
				Error: "token expired",
			}, nil)

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
		req.Header.Set("Authorization", "Bearer expired-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("InactiveUser", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(&authv1.ValidateTokenResponse{
				Valid: true,
				User: &authv1.UserInfo{
					Id:     userID,
					Role:   &authv1.Role{Id: "1", Name: "client"},
					Active: false,
				},
			}, nil)

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("WrongRole_NotClient", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(&authv1.ValidateTokenResponse{
				Valid: true,
				User: &authv1.UserInfo{
					Id:     userID,
					Role:   &authv1.Role{Id: "1", Name: "admin"},
					Active: true,
				},
			}, nil)

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("NilUser", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(&authv1.ValidateTokenResponse{
				Valid: true,
				User:  nil,
			}, nil)

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("HealthEndpoint_SkipsAuth", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called for health endpoint")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("MetricsEndpoint_SkipsAuth", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called for metrics endpoint")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("MalformedAuthorizationHeader", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("GetPermissions_Error_StillProceeds", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(&authv1.ValidateTokenResponse{
				Valid: true,
				User: &authv1.UserInfo{
					Id:     userID,
					Role:   &authv1.Role{Id: "1", Name: "client"},
					Active: true,
				},
			}, nil)

		authClient.On("GetPermissions", mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("permissions service unavailable"))

		middleware := ClientAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Middleware продолжает без прав, это не критично
		assert.True(t, recorder.called, "next handler should be called even if permissions fail")
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.False(t, HasPermission(recorder.ctx, "messages", "write"))
	})
}
