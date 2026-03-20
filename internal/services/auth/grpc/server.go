package grpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/services/auth/application"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
)

// Server реализует gRPC сервис для аутентификации
type Server struct {
	authv1.UnimplementedAuthServiceServer
	authService   *application.AuthService
	tokenService  *application.TokenService
	userRepo      *authrepo.UserRepository
	roleRepo      *authrepo.RoleRepository
}

// NewServer создает новый gRPC сервер для Auth Service
func NewServer(
	authService *application.AuthService,
	tokenService *application.TokenService,
	userRepo *authrepo.UserRepository,
	roleRepo *authrepo.RoleRepository,
) *Server {
	return &Server{
		authService:  authService,
		tokenService: tokenService,
		userRepo:     userRepo,
		roleRepo:     roleRepo,
	}
}

// Authenticate выполняет аутентификацию пользователя
func (s *Server) Authenticate(ctx context.Context, req *authv1.AuthenticateRequest) (*authv1.AuthenticateResponse, error) {
	var user *domain.User
	var accessToken, refreshToken string
	var err error

	// Аутентификация по API ключу
	if req.ApiKey != "" {
		user, err = s.authService.AuthenticateByAPIKey(ctx, req.ApiKey)
		if err != nil {
			if err == application.ErrAPIKeyInvalid {
				return nil, status.Error(codes.Unauthenticated, "invalid API key")
			}
			log.Error().Err(err).Msg("ошибка аутентификации по API ключу")
			return nil, status.Error(codes.Internal, "authentication failed")
		}

		// Для API ключей генерируем токены
		accessToken, refreshToken, err = s.tokenService.GenerateTokenPair(ctx, user.ID, user.Role.Name)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to generate tokens")
		}
	} else if req.Username != "" && req.Password != "" {
		// Аутентификация по username/password
		user, accessToken, refreshToken, err = s.authService.AuthenticateByCredentials(
			ctx, req.Username, req.Password,
		)
		if err != nil {
			if err == application.ErrInvalidCredentials {
				return nil, status.Error(codes.Unauthenticated, "invalid credentials")
			}
			if err == application.ErrUserInactive {
				return nil, status.Error(codes.PermissionDenied, "user is inactive")
			}
			log.Error().Err(err).Msg("ошибка аутентификации по credentials")
			return nil, status.Error(codes.Internal, "authentication failed")
		}
	} else {
		return nil, status.Error(codes.InvalidArgument, "username/password or api_key required")
	}

	// Вычисляем время истечения токена (24 часа по умолчанию)
	expiresAt := timestamppb.New(time.Now().Add(24 * time.Hour))

	return &authv1.AuthenticateResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         s.domainUserToProto(user),
	}, nil
}

// ValidateToken проверяет валидность токена
func (s *Server) ValidateToken(ctx context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	if req.Token == "" {
		return &authv1.ValidateTokenResponse{
			Valid: false,
			Error: "token is required",
		}, nil
	}

	// Пробуем валидировать как JWT токен
	user, err := s.authService.ValidateToken(ctx, req.Token)
	if err != nil {
		// Если не JWT, пробуем как API ключ
		user, err = s.authService.AuthenticateByAPIKey(ctx, req.Token)
		if err != nil {
			return &authv1.ValidateTokenResponse{
				Valid: false,
				Error: "invalid token",
			}, nil
		}
	}

	return &authv1.ValidateTokenResponse{
		Valid: true,
		User:  s.domainUserToProto(user),
	}, nil
}

// RefreshToken обновляет токен доступа
func (s *Server) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	// Получаем роль пользователя для RefreshToken
	getUserRole := func(userID uuid.UUID) (string, error) {
		user, err := s.userRepo.GetByIDWithRole(ctx, userID)
		if err != nil {
			return "", err
		}
		return user.Role.Name, nil
	}

	accessToken, refreshToken, err := s.tokenService.RefreshToken(ctx, req.RefreshToken, getUserRole)
	if err != nil {
		if err == application.ErrInvalidToken {
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		}
		log.Error().Err(err).Msg("ошибка обновления токена")
		return nil, status.Error(codes.Internal, "failed to refresh token")
	}

	expiresAt := timestamppb.New(time.Now().Add(24 * time.Hour))

	return &authv1.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// GetPermissions получает права доступа пользователя
func (s *Server) GetPermissions(ctx context.Context, req *authv1.GetPermissionsRequest) (*authv1.GetPermissionsResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	user, err := s.userRepo.GetByIDWithRole(ctx, userID)
	if err != nil {
		if err == authrepo.ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	permissions := make([]*authv1.Permission, len(user.Permissions))
	for i, perm := range user.Permissions {
		permissions[i] = &authv1.Permission{
			Id:       perm.ID.String(),
			Resource: perm.Resource,
			Action:   perm.Action,
		}
	}

	var role *authv1.Role
	if user.Role != nil {
		role = &authv1.Role{
			Id:          user.Role.ID.String(),
			Name:        user.Role.Name,
			Description: user.Role.Description,
		}
	}

	return &authv1.GetPermissionsResponse{
		Permissions: permissions,
		Role:        role,
	}, nil
}

// CreateAPIKey создает новый API ключ
func (s *Server) CreateAPIKey(ctx context.Context, req *authv1.CreateAPIKeyRequest) (*authv1.CreateAPIKeyResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t := req.ExpiresAt.AsTime()
		expiresAt = &t
	}

	key, apiKey, err := s.authService.CreateAPIKey(ctx, userID, req.Name, expiresAt, req.Scopes)
	if err != nil {
		if err == authrepo.ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		log.Error().Err(err).Msg("ошибка создания API ключа")
		return nil, status.Error(codes.Internal, "failed to create API key")
	}

	response := &authv1.CreateAPIKeyResponse{
		ApiKey:    apiKey, // Возвращаем полный ключ только один раз
		ApiKeyId:  key.ID.String(),
		CreatedAt: timestamppb.New(key.CreatedAt),
	}

	if key.ExpiresAt != nil {
		response.ExpiresAt = timestamppb.New(*key.ExpiresAt)
	}

	return response, nil
}

// RevokeAPIKey отзывает API ключ
func (s *Server) RevokeAPIKey(ctx context.Context, req *authv1.RevokeAPIKeyRequest) (*authv1.RevokeAPIKeyResponse, error) {
	if req.ApiKeyId == "" {
		return nil, status.Error(codes.InvalidArgument, "api_key_id is required")
	}

	keyID, err := uuid.Parse(req.ApiKeyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid api_key_id format")
	}

	err = s.authService.RevokeAPIKey(ctx, keyID)
	if err != nil {
		if err == authrepo.ErrAPIKeyNotFound {
			return nil, status.Error(codes.NotFound, "API key not found")
		}
		log.Error().Err(err).Msg("ошибка отзыва API ключа")
		return nil, status.Error(codes.Internal, "failed to revoke API key")
	}

	return &authv1.RevokeAPIKeyResponse{
		Success: true,
	}, nil
}

// ListAPIKeys получает список API ключей пользователя
func (s *Server) ListAPIKeys(ctx context.Context, req *authv1.ListAPIKeysRequest) (*authv1.ListAPIKeysResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	keys, err := s.authService.ListAPIKeys(ctx, userID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка API ключей")
		return nil, status.Error(codes.Internal, "failed to list API keys")
	}

	apiKeys := make([]*authv1.APIKeyInfo, len(keys))
	for i, key := range keys {
		apiKeys[i] = &authv1.APIKeyInfo{
			Id:        key.ID.String(),
			Name:      key.Name,
			Prefix:    key.KeyPrefix,
			Active:    key.Active,
			CreatedAt: timestamppb.New(key.CreatedAt),
			Scopes:    key.Scopes,
		}

		if key.ExpiresAt != nil {
			apiKeys[i].ExpiresAt = timestamppb.New(*key.ExpiresAt)
		}
		if key.LastUsedAt != nil {
			apiKeys[i].LastUsedAt = timestamppb.New(*key.LastUsedAt)
		}
	}

	return &authv1.ListAPIKeysResponse{
		Keys: apiKeys,
	}, nil
}

// domainUserToProto преобразует domain.User в proto UserInfo
func (s *Server) domainUserToProto(user *domain.User) *authv1.UserInfo {
	if user == nil {
		return nil
	}

	var role *authv1.Role
	if user.Role != nil {
		role = &authv1.Role{
			Id:          user.Role.ID.String(),
			Name:        user.Role.Name,
			Description: user.Role.Description,
		}
	}

	return &authv1.UserInfo{
		Id:        user.ID.String(),
		Username:  user.Username,
		Email:     user.Email,
		Role:      role,
		Active:    user.Active,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}
}
