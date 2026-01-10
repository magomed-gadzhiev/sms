package grpc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
)

type contextKey string

const (
	ClientIDKey contextKey = "client_id"
	UserKey     contextKey = "user"
)

// AuthInterceptor создает gRPC interceptor для аутентификации клиентов
func AuthInterceptor(authClient authv1.AuthServiceClient) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Получаем метаданные из контекста
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "метаданные не найдены")
		}

		// Получаем токен из заголовков
		var token string
		if values := md.Get("authorization"); len(values) > 0 {
			authHeader := values[0]
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			} else {
				token = authHeader
			}
		}

		// Также проверяем x-api-key для обратной совместимости
		if token == "" {
			if values := md.Get("x-api-key"); len(values) > 0 {
				token = values[0]
			}
		}

		if token == "" {
			return nil, status.Error(codes.Unauthenticated, "токен авторизации или API ключ не предоставлен")
		}

		// Валидируем токен через Auth Service
		validateResp, err := authClient.ValidateToken(ctx, &authv1.ValidateTokenRequest{
			Token: token,
		})
		if err != nil {
			log.Error().Err(err).Msg("ошибка валидации токена")
			return nil, status.Error(codes.Unauthenticated, "неверный токен авторизации")
		}

		if !validateResp.Valid {
			return nil, status.Error(codes.Unauthenticated, "токен невалиден или истек")
		}

		user := validateResp.User
		if user == nil {
			return nil, status.Error(codes.Unauthenticated, "информация о пользователе не найдена")
		}

		// Проверяем, что пользователь активен
		if !user.Active {
			return nil, status.Error(codes.PermissionDenied, "пользователь неактивен")
		}

		// Проверяем, что это клиент (для client gateway требуется роль client)
		if user.Role == nil || user.Role.Name != "client" {
			return nil, status.Error(codes.PermissionDenied, "доступ запрещен: требуется роль клиента")
		}

		// Парсим user ID
		userID, err := uuid.Parse(user.Id)
		if err != nil {
			log.Error().Err(err).Str("user_id", user.Id).Msg("ошибка парсинга user_id")
			return nil, status.Error(codes.Internal, "ошибка обработки пользователя")
		}

		// Добавляем информацию о клиенте в контекст
		ctx = context.WithValue(ctx, ClientIDKey, userID)
		ctx = context.WithValue(ctx, UserKey, user)

		return handler(ctx, req)
	}
}

// GetClientID извлекает ID клиента из контекста
func GetClientID(ctx context.Context) (uuid.UUID, bool) {
	clientID, ok := ctx.Value(ClientIDKey).(uuid.UUID)
	return clientID, ok
}
