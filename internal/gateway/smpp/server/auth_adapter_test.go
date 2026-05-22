package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- NewAuthAdapter ---

func TestNewAuthAdapter(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()

	adapter := NewAuthAdapter(mockClient, logger)

	require.NotNil(t, adapter)
	assert.Equal(t, mockClient, adapter.authClient)
}

// --- ValidateToken ---

func TestValidateToken_Success(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	userID := uuid.New().String()

	mockClient.On("ValidateToken", mock.Anything, mock.MatchedBy(func(req *authv1.ValidateTokenRequest) bool {
		return req.Token == "valid-token"
	})).Return(&authv1.ValidateTokenResponse{
		Valid: true,
		User: &authv1.UserInfo{
			Id:       userID,
			Username: "testuser",
			Email:    "test@example.com",
			Active:   true,
		},
	}, nil)

	info, err := adapter.ValidateToken(context.Background(), "valid-token")
	require.NoError(t, err)
	assert.Equal(t, userID, info.UserID)
	assert.Equal(t, userID, info.ClientID)
	assert.Equal(t, 100, info.RateLimit)
	assert.True(t, info.Active)
	mockClient.AssertExpectations(t)
}

func TestValidateToken_InvalidToken(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	mockClient.On("ValidateToken", mock.Anything, mock.Anything).Return(&authv1.ValidateTokenResponse{
		Valid: false,
		Error: "token expired",
	}, nil)

	_, err := adapter.ValidateToken(context.Background(), "expired-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "невалиден")
	mockClient.AssertExpectations(t)
}

func TestValidateToken_NilUser(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	mockClient.On("ValidateToken", mock.Anything, mock.Anything).Return(&authv1.ValidateTokenResponse{
		Valid: true,
		User:  nil,
	}, nil)

	_, err := adapter.ValidateToken(context.Background(), "token-no-user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "информация о пользователе отсутствует")
	mockClient.AssertExpectations(t)
}

func TestValidateToken_GRPCUnauthenticated(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	mockClient.On("ValidateToken", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unauthenticated, "unauthorized"))

	_, err := adapter.ValidateToken(context.Background(), "bad-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "невалиден")
	mockClient.AssertExpectations(t)
}

func TestValidateToken_GRPCPermissionDenied(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	mockClient.On("ValidateToken", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.PermissionDenied, "forbidden"))

	_, err := adapter.ValidateToken(context.Background(), "forbidden-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "невалиден")
	mockClient.AssertExpectations(t)
}

func TestValidateToken_GRPCInternalError(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	mockClient.On("ValidateToken", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Internal, "internal server error"))

	_, err := adapter.ValidateToken(context.Background(), "some-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ошибка валидации токена")
	mockClient.AssertExpectations(t)
}

// --- AuthenticateBySystemID ---

func TestAuthenticateBySystemID_Success(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	userID := uuid.New().String()

	mockClient.On("Authenticate", mock.Anything, mock.MatchedBy(func(req *authv1.AuthenticateRequest) bool {
		return req.ApiKey == "my-api-key"
	})).Return(&authv1.AuthenticateResponse{
		AccessToken: "new-token",
		User: &authv1.UserInfo{
			Id:       userID,
			Username: "apiuser",
			Active:   true,
		},
	}, nil)

	info, err := adapter.AuthenticateBySystemID(context.Background(), "my-api-key", "my-password")
	require.NoError(t, err)
	assert.Equal(t, userID, info.UserID)
	assert.Equal(t, userID, info.ClientID)
	assert.Equal(t, 100, info.RateLimit)
	assert.True(t, info.Active)
	mockClient.AssertExpectations(t)
}

func TestAuthenticateBySystemID_InactiveUser(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	mockClient.On("Authenticate", mock.Anything, mock.Anything).Return(&authv1.AuthenticateResponse{
		User: &authv1.UserInfo{
			Id:       uuid.New().String(),
			Username: "inactive",
			Active:   false,
		},
	}, nil)

	_, err := adapter.AuthenticateBySystemID(context.Background(), "sys", "pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "неактивен")
	mockClient.AssertExpectations(t)
}

func TestAuthenticateBySystemID_NilUser(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	mockClient.On("Authenticate", mock.Anything, mock.Anything).Return(&authv1.AuthenticateResponse{
		User: nil,
	}, nil)

	_, err := adapter.AuthenticateBySystemID(context.Background(), "sys", "pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "информация о пользователе отсутствует")
	mockClient.AssertExpectations(t)
}

func TestAuthenticateBySystemID_GRPCUnauthenticated(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	mockClient.On("Authenticate", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unauthenticated, "bad credentials"))

	_, err := adapter.AuthenticateBySystemID(context.Background(), "bad-sys", "bad-pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "неверный system_id")
	mockClient.AssertExpectations(t)
}

func TestAuthenticateBySystemID_GRPCGenericError(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	mockClient.On("Authenticate", mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("connection refused"))

	_, err := adapter.AuthenticateBySystemID(context.Background(), "sys", "pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ошибка аутентификации")
	mockClient.AssertExpectations(t)
}

// --- GetClientID ---

func TestGetClientID_ValidUUID(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	testUUID := uuid.New()

	id, err := adapter.GetClientID(context.Background(), testUUID.String())
	require.NoError(t, err)
	require.NotNil(t, id)
	assert.Equal(t, testUUID, *id)
}

func TestGetClientID_InvalidUUID(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	id, err := adapter.GetClientID(context.Background(), "not-a-uuid")
	require.NoError(t, err)
	assert.Nil(t, id)
}

func TestGetClientID_EmptyString(t *testing.T) {
	mockClient := &mockAuthClient{}
	logger := zerolog.Nop()
	adapter := NewAuthAdapter(mockClient, logger)

	id, err := adapter.GetClientID(context.Background(), "")
	require.NoError(t, err)
	assert.Nil(t, id)
}
