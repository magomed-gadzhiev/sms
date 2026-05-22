package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
)

// stubAuthClient реализует authv1.AuthServiceClient: из всех методов определён только ValidateToken.
// Остальные — no-op, возвращают nil; их вызовы в тестах middleware не ожидаются.
type stubAuthClient struct {
	validateFn func(ctx context.Context, token string) (*authv1.ValidateTokenResponse, error)
}

func (s *stubAuthClient) ValidateToken(ctx context.Context, in *authv1.ValidateTokenRequest, opts ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	if s.validateFn != nil {
		return s.validateFn(ctx, in.Token)
	}
	return &authv1.ValidateTokenResponse{Valid: false, Error: "no stub"}, nil
}

// Заглушки для остальных методов интерфейса authv1.AuthServiceClient.
func (s *stubAuthClient) Authenticate(ctx context.Context, in *authv1.AuthenticateRequest, opts ...grpc.CallOption) (*authv1.AuthenticateResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) RefreshToken(ctx context.Context, in *authv1.RefreshTokenRequest, opts ...grpc.CallOption) (*authv1.RefreshTokenResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) GetPermissions(ctx context.Context, in *authv1.GetPermissionsRequest, opts ...grpc.CallOption) (*authv1.GetPermissionsResponse, error) {
	return &authv1.GetPermissionsResponse{}, nil
}
func (s *stubAuthClient) CreateAPIKey(ctx context.Context, in *authv1.CreateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.CreateAPIKeyResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) RevokeAPIKey(ctx context.Context, in *authv1.RevokeAPIKeyRequest, opts ...grpc.CallOption) (*authv1.RevokeAPIKeyResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) RotateAPIKey(ctx context.Context, in *authv1.RotateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.RotateAPIKeyResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) ListAPIKeys(ctx context.Context, in *authv1.ListAPIKeysRequest, opts ...grpc.CallOption) (*authv1.ListAPIKeysResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) SetupTOTP(ctx context.Context, in *authv1.SetupTOTPRequest, opts ...grpc.CallOption) (*authv1.SetupTOTPResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) VerifyTOTP(ctx context.Context, in *authv1.VerifyTOTPRequest, opts ...grpc.CallOption) (*authv1.VerifyTOTPResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) DisableTOTP(ctx context.Context, in *authv1.DisableTOTPRequest, opts ...grpc.CallOption) (*authv1.DisableTOTPResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) RequestPasswordReset(ctx context.Context, in *authv1.RequestPasswordResetRequest, opts ...grpc.CallOption) (*authv1.RequestPasswordResetResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) ResetPassword(ctx context.Context, in *authv1.ResetPasswordRequest, opts ...grpc.CallOption) (*authv1.ResetPasswordResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) LoginWithSession(ctx context.Context, in *authv1.LoginWithSessionRequest, opts ...grpc.CallOption) (*authv1.LoginWithSessionResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) ValidateSession(ctx context.Context, in *authv1.ValidateSessionRequest, opts ...grpc.CallOption) (*authv1.ValidateSessionResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) Logout(ctx context.Context, in *authv1.LogoutRequest, opts ...grpc.CallOption) (*authv1.LogoutResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) UpdateAPIKey(ctx context.Context, in *authv1.UpdateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.UpdateAPIKeyResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) ChangePassword(ctx context.Context, in *authv1.ChangePasswordRequest, opts ...grpc.CallOption) (*authv1.ChangePasswordResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) RegisterClient(ctx context.Context, in *authv1.RegisterClientRequest, opts ...grpc.CallOption) (*authv1.RegisterClientResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) CreateUser(ctx context.Context, in *authv1.CreateUserRequest, opts ...grpc.CallOption) (*authv1.CreateUserResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) UpdateUser(ctx context.Context, in *authv1.UpdateUserRequest, opts ...grpc.CallOption) (*authv1.UpdateUserResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) DeactivateUser(ctx context.Context, in *authv1.DeactivateUserRequest, opts ...grpc.CallOption) (*authv1.DeactivateUserResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) ResetUser2FA(ctx context.Context, in *authv1.ResetUser2FARequest, opts ...grpc.CallOption) (*authv1.ResetUser2FAResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) ResetUserPassword(ctx context.Context, in *authv1.ResetUserPasswordRequest, opts ...grpc.CallOption) (*authv1.ResetUserPasswordResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) ListUsers(ctx context.Context, in *authv1.ListUsersRequest, opts ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) GetUser(ctx context.Context, in *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) CreateRole(ctx context.Context, in *authv1.CreateRoleRequest, opts ...grpc.CallOption) (*authv1.CreateRoleResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) UpdateRole(ctx context.Context, in *authv1.UpdateRoleRequest, opts ...grpc.CallOption) (*authv1.UpdateRoleResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) DeleteRole(ctx context.Context, in *authv1.DeleteRoleRequest, opts ...grpc.CallOption) (*authv1.DeleteRoleResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) ListRoles(ctx context.Context, in *authv1.ListRolesRequest, opts ...grpc.CallOption) (*authv1.ListRolesResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) GetRole(ctx context.Context, in *authv1.GetRoleRequest, opts ...grpc.CallOption) (*authv1.GetRoleResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) ListAllPermissions(ctx context.Context, in *authv1.ListAllPermissionsRequest, opts ...grpc.CallOption) (*authv1.ListAllPermissionsResponse, error) {
	return nil, nil
}
func (s *stubAuthClient) GetUserPermissions(ctx context.Context, in *authv1.GetUserPermissionsRequest, opts ...grpc.CallOption) (*authv1.GetUserPermissionsResponse, error) {
	return nil, nil
}

func TestAPIKeyAuthMiddleware(t *testing.T) {
	t.Run("ValidAPIKey_SetsContext", func(t *testing.T) {
		userID := uuid.New()
		clientID := uuid.New()
		stub := &stubAuthClient{
			validateFn: func(ctx context.Context, token string) (*authv1.ValidateTokenResponse, error) {
				assert.Equal(t, "sk_live_testkey", token)
				return &authv1.ValidateTokenResponse{
					Valid: true,
					User: &authv1.UserInfo{
						Id:       userID.String(),
						ClientId: clientID.String(),
						Active:   true,
						Role:     &authv1.Role{Name: "client"},
					},
				}, nil
			},
		}
		rec := &nextHandlerRecorder{}
		mw := APIKeyAuthMiddleware(stub)
		h := mw(rec.handler())
		req := httptest.NewRequest(http.MethodPost, "/portal/v1/campaigns", nil)
		req.Header.Set("Authorization", "Bearer sk_live_testkey")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		require.True(t, rec.called)
		assert.Equal(t, http.StatusOK, rr.Code)
		gotUID, ok := GetUserID(rec.ctx)
		require.True(t, ok)
		assert.Equal(t, userID, gotUID)
		gotCID, ok := GetClientID(rec.ctx)
		require.True(t, ok)
		assert.Equal(t, clientID, gotCID)
		assert.Equal(t, AuthMethodAPIKey, GetAuthMethod(rec.ctx))
	})

	t.Run("InvalidAPIKey_Returns401", func(t *testing.T) {
		stub := &stubAuthClient{
			validateFn: func(ctx context.Context, token string) (*authv1.ValidateTokenResponse, error) {
				return &authv1.ValidateTokenResponse{Valid: false, Error: "invalid token"}, nil
			},
		}
		rec := &nextHandlerRecorder{}
		mw := APIKeyAuthMiddleware(stub)
		h := mw(rec.handler())
		req := httptest.NewRequest(http.MethodPost, "/portal/v1/campaigns", nil)
		req.Header.Set("Authorization", "Bearer sk_live_bogus")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("MissingAuthHeader_Returns401", func(t *testing.T) {
		stub := &stubAuthClient{}
		rec := &nextHandlerRecorder{}
		mw := APIKeyAuthMiddleware(stub)
		h := mw(rec.handler())
		req := httptest.NewRequest(http.MethodPost, "/portal/v1/campaigns", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("NonBearerScheme_Returns401", func(t *testing.T) {
		stub := &stubAuthClient{}
		rec := &nextHandlerRecorder{}
		mw := APIKeyAuthMiddleware(stub)
		h := mw(rec.handler())
		req := httptest.NewRequest(http.MethodPost, "/portal/v1/campaigns", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("InactiveUser_Returns403", func(t *testing.T) {
		userID := uuid.New()
		stub := &stubAuthClient{
			validateFn: func(ctx context.Context, token string) (*authv1.ValidateTokenResponse, error) {
				return &authv1.ValidateTokenResponse{
					Valid: true,
					User: &authv1.UserInfo{
						Id:     userID.String(),
						Active: false,
						Role:   &authv1.Role{Name: "client"},
					},
				}, nil
			},
		}
		rec := &nextHandlerRecorder{}
		mw := APIKeyAuthMiddleware(stub)
		h := mw(rec.handler())
		req := httptest.NewRequest(http.MethodPost, "/portal/v1/campaigns", nil)
		req.Header.Set("Authorization", "Bearer sk_live_inactive")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

func TestSessionOrAPIKeyMiddleware_Routing(t *testing.T) {
	t.Run("WithSessionCookie_UsesSessionPath", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		ctx := context.Background()
		userID := uuid.New().String()
		clientID := uuid.New().String()
		sessionID := "test-session-composite"

		redisClient.HSet(ctx, "session:"+sessionID, map[string]interface{}{
			"user_id":   userID,
			"client_id": clientID,
			"role":      "owner",
		})

		// stub auth client: если сюда дойдёт — assertion зафейлится
		stub := &stubAuthClient{
			validateFn: func(ctx context.Context, token string) (*authv1.ValidateTokenResponse, error) {
				t.Fatalf("API-key path must not be invoked when session cookie present")
				return nil, nil
			},
		}
		rec := &nextHandlerRecorder{}
		mw := SessionOrAPIKeyMiddleware(redisClient, stub)
		h := mw(rec.handler())

		req := httptest.NewRequest(http.MethodPost, "/portal/v1/campaigns", nil)
		req.AddCookie(&http.Cookie{Name: "portal_session", Value: sessionID})
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		require.True(t, rec.called)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, AuthMethodSession, GetAuthMethod(rec.ctx))
	})

	t.Run("WithBearerToken_UsesAPIKeyPath", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		userID := uuid.New()
		clientID := uuid.New()
		stub := &stubAuthClient{
			validateFn: func(ctx context.Context, token string) (*authv1.ValidateTokenResponse, error) {
				return &authv1.ValidateTokenResponse{
					Valid: true,
					User: &authv1.UserInfo{
						Id:       userID.String(),
						ClientId: clientID.String(),
						Active:   true,
						Role:     &authv1.Role{Name: "client"},
					},
				}, nil
			},
		}
		rec := &nextHandlerRecorder{}
		mw := SessionOrAPIKeyMiddleware(redisClient, stub)
		h := mw(rec.handler())

		req := httptest.NewRequest(http.MethodPost, "/portal/v1/campaigns", nil)
		req.Header.Set("Authorization", "Bearer sk_live_valid")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		require.True(t, rec.called)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, AuthMethodAPIKey, GetAuthMethod(rec.ctx))
		gotCID, ok := GetClientID(rec.ctx)
		require.True(t, ok)
		assert.Equal(t, clientID, gotCID)
	})

	t.Run("NoAuth_Returns401", func(t *testing.T) {
		redisClient := setupTestRedis(t)
		stub := &stubAuthClient{}
		rec := &nextHandlerRecorder{}
		mw := SessionOrAPIKeyMiddleware(redisClient, stub)
		h := mw(rec.handler())

		req := httptest.NewRequest(http.MethodPost, "/portal/v1/campaigns", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		assert.False(t, rec.called)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

// TestCampaignCreateEnforcesAuthClientID_IDOR verifies the handler-level IDOR defense:
// CreateCampaign overrides req.ClientId with the auth subject's client_id — a caller
// cannot forge the client_id in the body. Covered at the handler layer (see campaigns.go:53).
// This test documents the contract from the middleware perspective: context must carry
// the auth subject's client_id, and downstream handlers are responsible for using it.
func TestAPIKeyContext_CarriesClientID_ForIDORDefense(t *testing.T) {
	ownerClientID := uuid.New()
	userID := uuid.New()
	stub := &stubAuthClient{
		validateFn: func(ctx context.Context, token string) (*authv1.ValidateTokenResponse, error) {
			return &authv1.ValidateTokenResponse{
				Valid: true,
				User: &authv1.UserInfo{
					Id:       userID.String(),
					ClientId: ownerClientID.String(),
					Active:   true,
					Role:     &authv1.Role{Name: "client"},
				},
			}, nil
		},
	}
	rec := &nextHandlerRecorder{}
	mw := APIKeyAuthMiddleware(stub)
	h := mw(rec.handler())

	// Attacker attempts to pass a different client_id via body (simulated here as
	// a request parameter). Middleware must set context from the authenticated
	// subject, not from the request.
	req := httptest.NewRequest(http.MethodPost, "/portal/v1/campaigns?client_id="+uuid.New().String(), nil)
	req.Header.Set("Authorization", "Bearer sk_live_idor")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.True(t, rec.called)
	gotCID, ok := GetClientID(rec.ctx)
	require.True(t, ok)
	assert.Equal(t, ownerClientID, gotCID, "context must carry auth subject's client_id, not request-supplied")
}
