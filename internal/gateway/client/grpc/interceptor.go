package grpc

import (
	"context"
	"os"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
)

type contextKey string

const (
	ClientIDKey contextKey = "client_id"
	UserIDKey   contextKey = "user_id"
	UserKey     contextKey = "user"
)

// loadTestDummyID — фиксированный UUID, инжектируемый в ctx при LOAD_TEST_MODE=true.
// Совпадает с dummyID HTTP middleware.
var loadTestDummyID = uuid.MustParse("c0000000-0000-0000-0000-000000000001")

func isLoadTestMode() bool {
	return strings.EqualFold(os.Getenv("LOAD_TEST_MODE"), "true")
}

// extractToken извлекает bearer token из gRPC metadata.
// Поддерживает два источника (как HTTP middleware):
//   - metadata "authorization" с prefix "Bearer "
//   - metadata "x-api-key" (API key напрямую)
func extractToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if vals := md.Get("authorization"); len(vals) > 0 {
		if strings.HasPrefix(vals[0], "Bearer ") {
			return strings.TrimPrefix(vals[0], "Bearer ")
		}
	}
	if vals := md.Get("x-api-key"); len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// AuthInterceptor создаёт gRPC unary interceptor, который валидирует токен через
// Auth Service и инжектирует authenticated client_id / user_id в ctx. Ранее
// был stub с захардкоженным dummy UUID, независимо от запроса — что давало
// полный tenant-bypass (finding A7.3 из Cycle 2 ревью v2).
//
// Логика соответствует HTTP ClientAuthMiddleware: LOAD_TEST_MODE → dummy;
// иначе extractToken → ValidateToken → resp.User.ClientId (fallback resp.User.Id).
// Неудача = codes.Unauthenticated.
func AuthInterceptor(authClient authv1.AuthServiceClient) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if isLoadTestMode() {
			ctx = context.WithValue(ctx, ClientIDKey, loadTestDummyID)
			ctx = context.WithValue(ctx, UserIDKey, loadTestDummyID)
			return handler(ctx, req)
		}

		token := extractToken(ctx)
		if token == "" {
			return nil, status.Error(codes.Unauthenticated, "authorization or x-api-key metadata is required")
		}

		resp, err := authClient.ValidateToken(ctx, &authv1.ValidateTokenRequest{Token: token})
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}
		if !resp.Valid || resp.User == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}
		if !resp.User.Active {
			return nil, status.Error(codes.PermissionDenied, "user account is inactive")
		}

		userID, parseErr := uuid.Parse(resp.User.Id)
		if parseErr != nil {
			return nil, status.Error(codes.Internal, "invalid user data from auth service")
		}

		clientID := userID
		if resp.User.ClientId != "" {
			if parsed, cidErr := uuid.Parse(resp.User.ClientId); cidErr == nil {
				clientID = parsed
			}
		}

		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, ClientIDKey, clientID)
		ctx = context.WithValue(ctx, UserKey, resp.User)
		return handler(ctx, req)
	}
}

// GetClientID извлекает authenticated client_id из gRPC request ctx.
func GetClientID(ctx context.Context) (uuid.UUID, bool) {
	clientID, ok := ctx.Value(ClientIDKey).(uuid.UUID)
	return clientID, ok
}
