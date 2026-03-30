package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/shared/audit"
)

// --- Mock AuthServiceClient ---

type mockAuthClient struct {
	mock.Mock
}

func (m *mockAuthClient) Authenticate(ctx context.Context, in *authv1.AuthenticateRequest, opts ...grpc.CallOption) (*authv1.AuthenticateResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.AuthenticateResponse), args.Error(1)
}

func (m *mockAuthClient) ValidateToken(ctx context.Context, in *authv1.ValidateTokenRequest, opts ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.ValidateTokenResponse), args.Error(1)
}

func (m *mockAuthClient) RefreshToken(ctx context.Context, in *authv1.RefreshTokenRequest, opts ...grpc.CallOption) (*authv1.RefreshTokenResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.RefreshTokenResponse), args.Error(1)
}

func (m *mockAuthClient) GetPermissions(ctx context.Context, in *authv1.GetPermissionsRequest, opts ...grpc.CallOption) (*authv1.GetPermissionsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.GetPermissionsResponse), args.Error(1)
}

func (m *mockAuthClient) CreateAPIKey(ctx context.Context, in *authv1.CreateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.CreateAPIKeyResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.CreateAPIKeyResponse), args.Error(1)
}

func (m *mockAuthClient) RevokeAPIKey(ctx context.Context, in *authv1.RevokeAPIKeyRequest, opts ...grpc.CallOption) (*authv1.RevokeAPIKeyResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.RevokeAPIKeyResponse), args.Error(1)
}

func (m *mockAuthClient) ListAPIKeys(ctx context.Context, in *authv1.ListAPIKeysRequest, opts ...grpc.CallOption) (*authv1.ListAPIKeysResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.ListAPIKeysResponse), args.Error(1)
}

func (m *mockAuthClient) SetupTOTP(ctx context.Context, in *authv1.SetupTOTPRequest, opts ...grpc.CallOption) (*authv1.SetupTOTPResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.SetupTOTPResponse), args.Error(1)
}

func (m *mockAuthClient) VerifyTOTP(ctx context.Context, in *authv1.VerifyTOTPRequest, opts ...grpc.CallOption) (*authv1.VerifyTOTPResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.VerifyTOTPResponse), args.Error(1)
}

func (m *mockAuthClient) DisableTOTP(ctx context.Context, in *authv1.DisableTOTPRequest, opts ...grpc.CallOption) (*authv1.DisableTOTPResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.DisableTOTPResponse), args.Error(1)
}

func (m *mockAuthClient) RequestPasswordReset(ctx context.Context, in *authv1.RequestPasswordResetRequest, opts ...grpc.CallOption) (*authv1.RequestPasswordResetResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.RequestPasswordResetResponse), args.Error(1)
}

func (m *mockAuthClient) ResetPassword(ctx context.Context, in *authv1.ResetPasswordRequest, opts ...grpc.CallOption) (*authv1.ResetPasswordResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.ResetPasswordResponse), args.Error(1)
}

func (m *mockAuthClient) LoginWithSession(ctx context.Context, in *authv1.LoginWithSessionRequest, opts ...grpc.CallOption) (*authv1.LoginWithSessionResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.LoginWithSessionResponse), args.Error(1)
}

func (m *mockAuthClient) ValidateSession(ctx context.Context, in *authv1.ValidateSessionRequest, opts ...grpc.CallOption) (*authv1.ValidateSessionResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.ValidateSessionResponse), args.Error(1)
}

func (m *mockAuthClient) Logout(ctx context.Context, in *authv1.LogoutRequest, opts ...grpc.CallOption) (*authv1.LogoutResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authv1.LogoutResponse), args.Error(1)
}

func (m *mockAuthClient) UpdateAPIKey(ctx context.Context, in *authv1.UpdateAPIKeyRequest, opts ...grpc.CallOption) (*authv1.UpdateAPIKeyResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ChangePassword(ctx context.Context, in *authv1.ChangePasswordRequest, opts ...grpc.CallOption) (*authv1.ChangePasswordResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) RegisterClient(ctx context.Context, in *authv1.RegisterClientRequest, opts ...grpc.CallOption) (*authv1.RegisterClientResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) CreateUser(ctx context.Context, in *authv1.CreateUserRequest, opts ...grpc.CallOption) (*authv1.CreateUserResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) UpdateUser(ctx context.Context, in *authv1.UpdateUserRequest, opts ...grpc.CallOption) (*authv1.UpdateUserResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) DeactivateUser(ctx context.Context, in *authv1.DeactivateUserRequest, opts ...grpc.CallOption) (*authv1.DeactivateUserResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ResetUser2FA(ctx context.Context, in *authv1.ResetUser2FARequest, opts ...grpc.CallOption) (*authv1.ResetUser2FAResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ResetUserPassword(ctx context.Context, in *authv1.ResetUserPasswordRequest, opts ...grpc.CallOption) (*authv1.ResetUserPasswordResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ListUsers(ctx context.Context, in *authv1.ListUsersRequest, opts ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) GetUser(ctx context.Context, in *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) CreateRole(ctx context.Context, in *authv1.CreateRoleRequest, opts ...grpc.CallOption) (*authv1.CreateRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) UpdateRole(ctx context.Context, in *authv1.UpdateRoleRequest, opts ...grpc.CallOption) (*authv1.UpdateRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) DeleteRole(ctx context.Context, in *authv1.DeleteRoleRequest, opts ...grpc.CallOption) (*authv1.DeleteRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ListRoles(ctx context.Context, in *authv1.ListRolesRequest, opts ...grpc.CallOption) (*authv1.ListRolesResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) GetRole(ctx context.Context, in *authv1.GetRoleRequest, opts ...grpc.CallOption) (*authv1.GetRoleResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) ListAllPermissions(ctx context.Context, in *authv1.ListAllPermissionsRequest, opts ...grpc.CallOption) (*authv1.ListAllPermissionsResponse, error) {
	return nil, nil
}

func (m *mockAuthClient) GetUserPermissions(ctx context.Context, in *authv1.GetUserPermissionsRequest, opts ...grpc.CallOption) (*authv1.GetUserPermissionsResponse, error) {
	return nil, nil
}

// --- Mock audit.Publisher ---
// Since audit.Publisher uses a Kafka SyncProducer that is hard to mock,
// we pass nil for the audit publisher in tests. The handler checks for
// publish errors but does not fail the request because of them.

// --- Tests ---

func TestAuthHandlers(t *testing.T) {
	t.Run("Login", func(t *testing.T) {
		t.Run("successful login without 2FA", func(t *testing.T) {
			authClient := new(mockAuthClient)
			// Pass nil audit publisher -- the handler will get a nil pointer
			// but publishAuditEvent is called after the response is built,
			// so we wrap it in a sub-handler approach. We'll use a real
			// Publisher that is nil-safe, or skip audit entirely.
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			authClient.On("LoginWithSession", mock.Anything, mock.MatchedBy(func(req *authv1.LoginWithSessionRequest) bool {
				return req.Email == "user@example.com" && req.Password == "secret123"
			})).Return(&authv1.LoginWithSessionResponse{
				SessionId:    "session-abc-123",
				Requires_2Fa: false,
				User: &authv1.UserInfo{
					Id:       "user-1",
					Username: "testuser",
					Email:    "user@example.com",
					Active:   true,
					Role:     &authv1.Role{Name: "client"},
				},
			}, nil)

			body, _ := json.Marshal(loginRequest{
				Email:    "user@example.com",
				Password: "secret123",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.Login(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.NotNil(t, resp["user"])

			authClient.AssertExpectations(t)
		})

		t.Run("login requires 2FA", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			authClient.On("LoginWithSession", mock.Anything, mock.Anything).
				Return(&authv1.LoginWithSessionResponse{
					Requires_2Fa: true,
					LoginTicket:  "ticket-xyz",
				}, nil)

			body, _ := json.Marshal(loginRequest{
				Email:    "user@example.com",
				Password: "secret123",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.Login(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, true, resp["requires_2fa"])
			assert.Equal(t, "ticket-xyz", resp["login_ticket"])

			authClient.AssertExpectations(t)
		})

		t.Run("returns 400 for invalid JSON body", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", bytes.NewReader([]byte("bad json")))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.Login(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 400 when email or password is empty", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			body, _ := json.Marshal(loginRequest{
				Email:    "",
				Password: "secret",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.Login(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns error when auth service fails", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			authClient.On("LoginWithSession", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Unauthenticated, "invalid credentials"))

			body, _ := json.Marshal(loginRequest{
				Email:    "user@example.com",
				Password: "wrongpassword",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.Login(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)

			authClient.AssertExpectations(t)
		})
	})

	t.Run("LoginWith2FA", func(t *testing.T) {
		t.Run("successful 2FA login", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			authClient.On("LoginWithSession", mock.Anything, mock.MatchedBy(func(req *authv1.LoginWithSessionRequest) bool {
				return req.Email == "ticket-xyz" && req.TotpCode == "123456"
			})).Return(&authv1.LoginWithSessionResponse{
				SessionId: "session-2fa-ok",
				User: &authv1.UserInfo{
					Id:       "user-1",
					Username: "testuser",
					Email:    "user@example.com",
					Active:   true,
				},
			}, nil)

			body, _ := json.Marshal(login2FARequest{
				LoginTicket: "ticket-xyz",
				TOTPCode:    "123456",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login/2fa", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.LoginWith2FA(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.NotNil(t, resp["user"])

			authClient.AssertExpectations(t)
		})

		t.Run("returns 400 when fields are empty", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			body, _ := json.Marshal(login2FARequest{
				LoginTicket: "",
				TOTPCode:    "",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/login/2fa", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.LoginWith2FA(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	})

	t.Run("RequestPasswordReset", func(t *testing.T) {
		t.Run("always returns 202", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			authClient.On("RequestPasswordReset", mock.Anything, mock.Anything).
				Return(&authv1.RequestPasswordResetResponse{}, nil)

			body, _ := json.Marshal(passwordResetRequestBody{
				Email: "user@example.com",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/password/reset-request", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.RequestPasswordReset(rr, req)

			assert.Equal(t, http.StatusAccepted, rr.Code)

			authClient.AssertExpectations(t)
		})

		t.Run("returns 400 when email is empty", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			body, _ := json.Marshal(passwordResetRequestBody{
				Email: "",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/password/reset-request", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.RequestPasswordReset(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("still returns 202 even when service errors", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			authClient.On("RequestPasswordReset", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.NotFound, "user not found"))

			body, _ := json.Marshal(passwordResetRequestBody{
				Email: "nobody@example.com",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/password/reset-request", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.RequestPasswordReset(rr, req)

			// The handler always returns 202 to prevent user enumeration
			assert.Equal(t, http.StatusAccepted, rr.Code)
		})
	})

	t.Run("ResetPassword", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			authClient.On("ResetPassword", mock.Anything, mock.MatchedBy(func(req *authv1.ResetPasswordRequest) bool {
				return req.Token == "reset-token-123" && req.NewPassword == "newpassword"
			})).Return(&authv1.ResetPasswordResponse{}, nil)

			body, _ := json.Marshal(resetPasswordBody{
				Token:       "reset-token-123",
				NewPassword: "newpassword",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/password/reset", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.ResetPassword(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			authClient.AssertExpectations(t)
		})

		t.Run("returns 400 when token or password empty", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			body, _ := json.Marshal(resetPasswordBody{
				Token:       "",
				NewPassword: "",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/password/reset", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.ResetPassword(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	})

	t.Run("Logout", func(t *testing.T) {
		t.Run("returns 401 when no session cookie", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/logout", nil)

			rr := httptest.NewRecorder()
			handler.Logout(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})

		t.Run("success with valid session", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &AuthHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			authClient.On("Logout", mock.Anything, mock.MatchedBy(func(req *authv1.LogoutRequest) bool {
				return req.SessionId == "session-to-kill"
			})).Return(&authv1.LogoutResponse{}, nil)

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/auth/logout", nil)
			req.AddCookie(&http.Cookie{Name: "portal_session", Value: "session-to-kill"})

			rr := httptest.NewRecorder()
			handler.Logout(rr, req)

			assert.Equal(t, http.StatusNoContent, rr.Code)

			authClient.AssertExpectations(t)
		})
	})
}

// Ensure the mock satisfies the interface at compile time.
var _ authv1.AuthServiceClient = (*mockAuthClient)(nil)

// Suppress unused import warning for audit package.
var _ = audit.ActionLogin
