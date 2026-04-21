package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
)

// stubAuthClient — минимальная заглушка AuthServiceClient для юнит-тестов.
// Используем structured fields вместо testify/mock чтобы не тянуть лишнего.
type stubAuthClient struct {
	authv1.AuthServiceClient // embed для satisfy interface по остальным методам
	validateResp             *authv1.ValidateTokenResponse
	validateErr              error
	lastToken                string
}

func (s *stubAuthClient) ValidateToken(_ context.Context, in *authv1.ValidateTokenRequest, _ ...grpc.CallOption) (*authv1.ValidateTokenResponse, error) {
	s.lastToken = in.Token
	return s.validateResp, s.validateErr
}

func ctxWithMD(pairs ...string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(pairs...))
}

func noopHandler(ctx context.Context, _ interface{}) (interface{}, error) {
	return ctx, nil
}

// ─── A7.3 regression: ValidateToken вызывается вместо stub-инжекции ───

func TestAuthInterceptor_NoToken_Unauthenticated(t *testing.T) {
	t.Parallel()

	auth := &stubAuthClient{}
	interceptor := AuthInterceptor(auth)

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, noopHandler)

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Empty(t, auth.lastToken, "ValidateToken не должен вызываться без token")
}

func TestAuthInterceptor_InvalidToken_Unauthenticated(t *testing.T) {
	t.Parallel()

	auth := &stubAuthClient{validateErr: errors.New("token rejected")}
	interceptor := AuthInterceptor(auth)

	ctx := ctxWithMD("authorization", "Bearer bad-token")
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, noopHandler)

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Equal(t, "bad-token", auth.lastToken)
}

func TestAuthInterceptor_ValidToken_InjectsRealClientID(t *testing.T) {
	t.Parallel()

	realUserID := uuid.New()
	realClientID := uuid.New()
	auth := &stubAuthClient{
		validateResp: &authv1.ValidateTokenResponse{
			Valid: true,
			User: &authv1.UserInfo{
				Id:       realUserID.String(),
				ClientId: realClientID.String(),
				Active:   true,
			},
		},
	}
	interceptor := AuthInterceptor(auth)

	captured := ctxCapture{}
	ctx := ctxWithMD("authorization", "Bearer real-token")
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, captured.handle)

	require.NoError(t, err)
	require.NotNil(t, captured.ctx)
	gotClientID, ok := GetClientID(captured.ctx)
	require.True(t, ok, "ClientID должен быть в ctx")
	assert.Equal(t, realClientID, gotClientID, "должен быть реальный client_id из ValidateToken, не dummy UUID")
	assert.NotEqual(t, loadTestDummyID, gotClientID, "регрессия A7.3: не должно быть захардкоженного dummy")
}

func TestAuthInterceptor_XAPIKey_Alternative(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	auth := &stubAuthClient{
		validateResp: &authv1.ValidateTokenResponse{
			Valid: true,
			User:  &authv1.UserInfo{Id: userID.String(), Active: true},
		},
	}
	interceptor := AuthInterceptor(auth)

	ctx := ctxWithMD("x-api-key", "api-key-value")
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, noopHandler)

	require.NoError(t, err)
	assert.Equal(t, "api-key-value", auth.lastToken)
}

func TestAuthInterceptor_InactiveUser_PermissionDenied(t *testing.T) {
	t.Parallel()

	auth := &stubAuthClient{
		validateResp: &authv1.ValidateTokenResponse{
			Valid: true,
			User:  &authv1.UserInfo{Id: uuid.New().String(), Active: false},
		},
	}
	interceptor := AuthInterceptor(auth)

	ctx := ctxWithMD("authorization", "Bearer ok-token")
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, noopHandler)

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestAuthInterceptor_ClientIDFallsBackToUserID(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	auth := &stubAuthClient{
		validateResp: &authv1.ValidateTokenResponse{
			Valid: true,
			// ClientId пустой — fallback на user_id (match HTTP middleware поведение).
			User: &authv1.UserInfo{Id: userID.String(), Active: true},
		},
	}
	interceptor := AuthInterceptor(auth)

	captured := ctxCapture{}
	ctx := ctxWithMD("authorization", "Bearer token")
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, captured.handle)

	require.NoError(t, err)
	got, _ := GetClientID(captured.ctx)
	assert.Equal(t, userID, got)
}

// ─── Helper для захвата ctx, переданного в handler ───

type ctxCapture struct {
	ctx context.Context
}

func (c *ctxCapture) handle(ctx context.Context, _ interface{}) (interface{}, error) {
	c.ctx = ctx
	return nil, nil
}
