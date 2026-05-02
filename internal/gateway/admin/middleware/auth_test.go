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

func (m *mockAuthServiceClient) RotateAPIKey(ctx context.Context, in *authv1.RotateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.RotateAPIKeyResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.RotateAPIKeyResponse), args.Error(1)
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

func (m *mockAuthServiceClient) CreateUser(ctx context.Context, in *authv1.CreateUserRequest, opts ...grpc.CallOption) (*authv1.CreateUserResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) UpdateUser(ctx context.Context, in *authv1.UpdateUserRequest, opts ...grpc.CallOption) (*authv1.UpdateUserResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) DeactivateUser(ctx context.Context, in *authv1.DeactivateUserRequest, opts ...grpc.CallOption) (*authv1.DeactivateUserResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) ResetUser2FA(ctx context.Context, in *authv1.ResetUser2FARequest, opts ...grpc.CallOption) (*authv1.ResetUser2FAResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) ResetUserPassword(ctx context.Context, in *authv1.ResetUserPasswordRequest, opts ...grpc.CallOption) (*authv1.ResetUserPasswordResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) ListUsers(ctx context.Context, in *authv1.ListUsersRequest, opts ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) GetUser(ctx context.Context, in *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) CreateRole(ctx context.Context, in *authv1.CreateRoleRequest, opts ...grpc.CallOption) (*authv1.CreateRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) UpdateRole(ctx context.Context, in *authv1.UpdateRoleRequest, opts ...grpc.CallOption) (*authv1.UpdateRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) DeleteRole(ctx context.Context, in *authv1.DeleteRoleRequest, opts ...grpc.CallOption) (*authv1.DeleteRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) ListRoles(ctx context.Context, in *authv1.ListRolesRequest, opts ...grpc.CallOption) (*authv1.ListRolesResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) GetRole(ctx context.Context, in *authv1.GetRoleRequest, opts ...grpc.CallOption) (*authv1.GetRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) ListAllPermissions(ctx context.Context, in *authv1.ListAllPermissionsRequest, opts ...grpc.CallOption) (*authv1.ListAllPermissionsResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceClient) GetUserPermissions(ctx context.Context, in *authv1.GetUserPermissionsRequest, opts ...grpc.CallOption) (*authv1.GetUserPermissionsResponse, error) {
	return nil, nil
}

// nextHandlerRecorder записывает, был ли вызван следующий обработчик
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

func TestAdminAuthMiddleware(t *testing.T) {
	userID := uuid.New().String()

	t.Run("ValidJWT", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, &authv1.ValidateTokenRequest{
			Token: "valid-admin-jwt",
		}).Return(&authv1.ValidateTokenResponse{
			Valid: true,
			User: &authv1.UserInfo{
				Id:       userID,
				Username: "admin_user",
				Email:    "admin@example.com",
				Role:     &authv1.Role{Id: "1", Name: "admin"},
				Active:   true,
			},
		}, nil)

		authClient.On("GetPermissions", mock.Anything, &authv1.GetPermissionsRequest{
			UserId: userID,
		}).Return(&authv1.GetPermissionsResponse{
			Permissions: []*authv1.Permission{
				{Id: "1", Resource: "clients", Action: "read"},
				{Id: "2", Resource: "clients", Action: "write"},
				{Id: "3", Resource: "providers", Action: "read"},
			},
		}, nil)

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/clients", nil)
		req.Header.Set("Authorization", "Bearer valid-admin-jwt")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called")
		assert.Equal(t, http.StatusOK, rr.Code)

		// Проверяем контекст
		ctxUserID, ok := GetUserID(recorder.ctx)
		assert.True(t, ok)
		assert.Equal(t, userID, ctxUserID.String())

		user, ok := GetUser(recorder.ctx)
		assert.True(t, ok)
		assert.Equal(t, "admin_user", user.Username)

		role, ok := GetRole(recorder.ctx)
		assert.True(t, ok)
		assert.Equal(t, "admin", role.Name)

		assert.True(t, HasPermission(recorder.ctx, "clients", "read"))
		assert.True(t, HasPermission(recorder.ctx, "clients", "write"))
		assert.True(t, HasPermission(recorder.ctx, "providers", "read"))
		assert.False(t, HasPermission(recorder.ctx, "providers", "delete"))

		authClient.AssertExpectations(t)
	})

	t.Run("ExpiredJWT", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(&authv1.ValidateTokenResponse{
				Valid: false,
				Error: "token expired",
			}, nil)

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/clients", nil)
		req.Header.Set("Authorization", "Bearer expired-jwt")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called for expired JWT")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		var body map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Contains(t, body, "error")
	})

	t.Run("InvalidJWT_ValidationError", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("invalid token signature"))

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/clients", nil)
		req.Header.Set("Authorization", "Bearer invalid-jwt")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called for invalid JWT")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("MissingAuthorizationHeader", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/clients", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called without Authorization header")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("MalformedAuthorizationHeader", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/clients", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called with non-Bearer auth")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("WrongRole_NotAdmin", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(&authv1.ValidateTokenResponse{
				Valid: true,
				User: &authv1.UserInfo{
					Id:     userID,
					Role:   &authv1.Role{Id: "2", Name: "client"},
					Active: true,
				},
			}, nil)

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/clients", nil)
		req.Header.Set("Authorization", "Bearer client-jwt")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called for non-admin role")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("NilRole", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(&authv1.ValidateTokenResponse{
				Valid: true,
				User: &authv1.UserInfo{
					Id:     userID,
					Role:   nil,
					Active: true,
				},
			}, nil)

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/clients", nil)
		req.Header.Set("Authorization", "Bearer no-role-jwt")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called when user has no role")
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("InactiveUser", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(&authv1.ValidateTokenResponse{
				Valid: true,
				User: &authv1.UserInfo{
					Id:     userID,
					Role:   &authv1.Role{Id: "1", Name: "admin"},
					Active: false,
				},
			}, nil)

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/clients", nil)
		req.Header.Set("Authorization", "Bearer inactive-admin-jwt")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called for inactive user")
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

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/clients", nil)
		req.Header.Set("Authorization", "Bearer valid-but-no-user")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called when user info is nil")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("HealthEndpoint_SkipsAuth", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		middleware := AdminAuthMiddleware(authClient)
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

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called for metrics endpoint")
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("GetPermissions_Error_StillProceeds", func(t *testing.T) {
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

		authClient.On("GetPermissions", mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("permissions service unavailable"))

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/clients", nil)
		req.Header.Set("Authorization", "Bearer valid-admin-jwt")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.True(t, recorder.called, "next handler should be called even if permissions fail")
		assert.Equal(t, http.StatusOK, rr.Code)
		// Без прав доступа HasPermission должен возвращать false
		assert.False(t, HasPermission(recorder.ctx, "clients", "read"))
	})

	t.Run("InvalidUserIDFormat", func(t *testing.T) {
		authClient := new(mockAuthServiceClient)
		recorder := &nextHandlerRecorder{}

		authClient.On("ValidateToken", mock.Anything, mock.Anything).
			Return(&authv1.ValidateTokenResponse{
				Valid: true,
				User: &authv1.UserInfo{
					Id:     "not-a-valid-uuid",
					Role:   &authv1.Role{Id: "1", Name: "admin"},
					Active: true,
				},
			}, nil)

		authClient.On("GetPermissions", mock.Anything, mock.Anything).
			Return(&authv1.GetPermissionsResponse{}, nil)

		middleware := AdminAuthMiddleware(authClient)
		handler := middleware(recorder.handler())

		req := httptest.NewRequest(http.MethodGet, "/admin/v1/clients", nil)
		req.Header.Set("Authorization", "Bearer valid-jwt")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.False(t, recorder.called, "next handler should NOT be called for invalid user_id format")
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
