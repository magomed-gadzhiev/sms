package server

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthAdapter адаптирует Auth Service gRPC клиент для использования в SMPP Gateway
type AuthAdapter struct {
	authClient authv1.AuthServiceClient
	logger     zerolog.Logger
}

// NewAuthAdapter создает новый адаптер для Auth Service
func NewAuthAdapter(authClient authv1.AuthServiceClient, logger zerolog.Logger) *AuthAdapter {
	return &AuthAdapter{
		authClient: authClient,
		logger:     logger,
	}
}

// ValidateToken валидирует токен через Auth Service
func (a *AuthAdapter) ValidateToken(ctx context.Context, token string) (UserInfo, error) {
	resp, err := a.authClient.ValidateToken(ctx, &authv1.ValidateTokenRequest{
		Token: token,
	})
	if err != nil {
		// Проверяем, является ли это ошибкой аутентификации
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.Unauthenticated || st.Code() == codes.PermissionDenied {
				return UserInfo{}, fmt.Errorf("токен невалиден: %w", err)
			}
		}
		return UserInfo{}, fmt.Errorf("ошибка валидации токена: %w", err)
	}

	if !resp.Valid {
		return UserInfo{}, fmt.Errorf("токен невалиден")
	}

	if resp.User == nil {
		return UserInfo{}, fmt.Errorf("информация о пользователе отсутствует")
	}

	// Преобразуем ClientID из строки в UUID, если возможно
	var clientID string
	if resp.User.Id != "" {
		// Попытка получить client_id из метаданных пользователя
		// В текущей реализации используем user_id как client_id
		clientID = resp.User.Id
	}

	// Определяем rate limit по умолчанию (можно будет получить из конфига клиента)
	rateLimit := 100 // По умолчанию

	return UserInfo{
		UserID:    resp.User.Id,
		ClientID:  clientID,
		RateLimit: rateLimit,
		Active:    resp.User.Active,
	}, nil
}

// AuthenticateBySystemID аутентифицирует по system_id (используется как API ключ)
func (a *AuthAdapter) AuthenticateBySystemID(ctx context.Context, systemID, password string) (UserInfo, error) {
	// Используем system_id как API ключ
	// В SMPP протоколе password может использоваться для дополнительной проверки
	// но основная аутентификация происходит через system_id (API ключ)
	
	resp, err := a.authClient.Authenticate(ctx, &authv1.AuthenticateRequest{
		ApiKey: systemID,
	})
	if err != nil {
		// Проверяем, является ли это ошибкой аутентификации
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.Unauthenticated {
				return UserInfo{}, fmt.Errorf("неверный system_id или пароль: %w", err)
			}
		}
		return UserInfo{}, fmt.Errorf("ошибка аутентификации: %w", err)
	}

	if resp.User == nil {
		return UserInfo{}, fmt.Errorf("информация о пользователе отсутствует")
	}

	// Проверяем активность
	if !resp.User.Active {
		return UserInfo{}, fmt.Errorf("пользователь неактивен")
	}

	// Преобразуем UserID в UUID для совместимости
	userID := resp.User.Id
	var clientID string
	if resp.User.Id != "" {
		// Используем user_id как client_id (можно будет получить из Client Service)
		clientID = resp.User.Id
	}

	// Определяем rate limit по умолчанию
	rateLimit := 100 // По умолчанию, можно получить из Client Service

	return UserInfo{
		UserID:    userID,
		ClientID:  clientID,
		RateLimit: rateLimit,
		Active:    resp.User.Active,
	}, nil
}

// GetClientID возвращает ClientID из UserID (временная реализация)
// В будущем можно получить через Client Service
func (a *AuthAdapter) GetClientID(ctx context.Context, userID string) (*uuid.UUID, error) {
	// Временная реализация: пытаемся преобразовать user_id в UUID
	id, err := uuid.Parse(userID)
	if err != nil {
		// Если не удалось, возвращаем nil (ClientID опционален)
		return nil, nil
	}
	return &id, nil
}
