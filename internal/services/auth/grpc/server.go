package grpc

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/internal/services/auth/application"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	authinfra "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
)

// Server реализует gRPC сервис для аутентификации
type Server struct {
	authv1.UnimplementedAuthServiceServer
	authService          *application.AuthService
	tokenService         *application.TokenService
	totpService          *application.TOTPService
	passwordResetService *application.PasswordResetService
	sessionManager       *authinfra.SessionManager
	userRepo             *authrepo.UserRepository
	roleRepo             *authrepo.RoleRepository
	passwordHasher       *authinfra.PasswordHasherImpl
	clientService        clientv1.ClientServiceClient
}

// NewServer создает новый gRPC сервер для Auth Service
func NewServer(
	authService *application.AuthService,
	tokenService *application.TokenService,
	totpService *application.TOTPService,
	passwordResetService *application.PasswordResetService,
	sessionManager *authinfra.SessionManager,
	userRepo *authrepo.UserRepository,
	roleRepo *authrepo.RoleRepository,
	passwordHasher *authinfra.PasswordHasherImpl,
	clientService clientv1.ClientServiceClient,
) *Server {
	return &Server{
		authService:          authService,
		tokenService:         tokenService,
		totpService:          totpService,
		passwordResetService: passwordResetService,
		sessionManager:       sessionManager,
		userRepo:             userRepo,
		roleRepo:             roleRepo,
		passwordHasher:       passwordHasher,
		clientService:        clientService,
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

	key, apiKey, err := s.authService.CreateAPIKey(ctx, userID, req.Name, expiresAt, req.Scopes, req.AllowedIps)
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
			Id:         key.ID.String(),
			Name:       key.Name,
			Prefix:     key.KeyPrefix,
			Active:     key.Active,
			CreatedAt:  timestamppb.New(key.CreatedAt),
			Scopes:     key.Scopes,
			AllowedIps: key.AllowedIPs,
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

// UpdateAPIKey обновляет API ключ (имя, scopes, IP, срок действия)
func (s *Server) UpdateAPIKey(ctx context.Context, req *authv1.UpdateAPIKeyRequest) (*authv1.UpdateAPIKeyResponse, error) {
	if req.KeyId == "" {
		return nil, status.Error(codes.InvalidArgument, "key_id is required")
	}
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	keyID, err := uuid.Parse(req.KeyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid key_id format")
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

	key, err := s.authService.UpdateAPIKey(ctx, keyID, userID, req.Name, req.Scopes, req.AllowedIps, expiresAt)
	if err != nil {
		if err == application.ErrAPIKeyNotFound {
			return nil, status.Error(codes.NotFound, "API key not found")
		}
		if err == application.ErrAPIKeyNotOwned {
			return nil, status.Error(codes.PermissionDenied, "API key does not belong to user")
		}
		if err == application.ErrAPIKeyRevoked {
			return nil, status.Error(codes.FailedPrecondition, "API key is revoked")
		}
		log.Error().Err(err).Msg("ошибка обновления API ключа")
		return nil, status.Error(codes.Internal, "failed to update API key")
	}

	info := &authv1.APIKeyInfo{
		Id:         key.ID.String(),
		Name:       key.Name,
		Prefix:     key.KeyPrefix,
		Active:     key.Active,
		CreatedAt:  timestamppb.New(key.CreatedAt),
		Scopes:     key.Scopes,
		AllowedIps: key.AllowedIPs,
	}
	if key.ExpiresAt != nil {
		info.ExpiresAt = timestamppb.New(*key.ExpiresAt)
	}
	if key.LastUsedAt != nil {
		info.LastUsedAt = timestamppb.New(*key.LastUsedAt)
	}

	return &authv1.UpdateAPIKeyResponse{
		Key: info,
	}, nil
}

// ChangePassword меняет пароль пользователя (требует текущий пароль)
func (s *Server) ChangePassword(ctx context.Context, req *authv1.ChangePasswordRequest) (*authv1.ChangePasswordResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.CurrentPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "current_password is required")
	}
	if req.NewPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "new_password is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	err = s.authService.ChangePassword(ctx, userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		if err == application.ErrCurrentPasswordWrong {
			return nil, status.Error(codes.Unauthenticated, "current password is incorrect")
		}
		if err == application.ErrPasswordTooShort {
			return nil, status.Error(codes.InvalidArgument, "password must be at least 8 characters")
		}
		if err == application.ErrPasswordSameAsOld {
			return nil, status.Error(codes.InvalidArgument, "new password must differ from current")
		}
		if err == authrepo.ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		log.Error().Err(err).Msg("ошибка смены пароля")
		return nil, status.Error(codes.Internal, "failed to change password")
	}

	return &authv1.ChangePasswordResponse{
		Success: true,
	}, nil
}

// SetupTOTP генерирует TOTP секрет, возвращает секрет + QR URI + коды восстановления
func (s *Server) SetupTOTP(ctx context.Context, req *authv1.SetupTOTPRequest) (*authv1.SetupTOTPResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	// Получаем пользователя для account name
	user, err := s.userRepo.GetByIDWithRole(ctx, userID)
	if err != nil {
		if err == authrepo.ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	secret, qrURL, recoveryCodes, err := s.totpService.SetupTOTP(ctx, userID, user.Email)
	if err != nil {
		if err == domain.ErrTOTPAlreadyEnabled {
			return nil, status.Error(codes.AlreadyExists, "TOTP is already enabled")
		}
		log.Error().Err(err).Msg("ошибка настройки TOTP")
		return nil, status.Error(codes.Internal, "failed to setup TOTP")
	}

	return &authv1.SetupTOTPResponse{
		Secret:        secret,
		QrCodeUrl:     qrURL,
		RecoveryCodes: recoveryCodes,
	}, nil
}

// VerifyTOTP проверяет TOTP код для включения 2FA
func (s *Server) VerifyTOTP(ctx context.Context, req *authv1.VerifyTOTPRequest) (*authv1.VerifyTOTPResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.TotpCode == "" {
		return nil, status.Error(codes.InvalidArgument, "totp_code is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	err = s.totpService.VerifyAndEnable(ctx, userID, req.TotpCode)
	if err != nil {
		if err == domain.ErrTOTPAlreadyEnabled {
			return nil, status.Error(codes.AlreadyExists, "TOTP is already enabled")
		}
		if err == domain.ErrTOTPNotEnabled {
			return nil, status.Error(codes.FailedPrecondition, "TOTP setup not initiated")
		}
		if err == domain.ErrInvalidTOTPCode {
			return &authv1.VerifyTOTPResponse{
				Success:     false,
				TotpEnabled: false,
			}, nil
		}
		log.Error().Err(err).Msg("ошибка верификации TOTP")
		return nil, status.Error(codes.Internal, "failed to verify TOTP")
	}

	return &authv1.VerifyTOTPResponse{
		Success:     true,
		TotpEnabled: true,
	}, nil
}

// DisableTOTP отключает 2FA (требует пароль)
func (s *Server) DisableTOTP(ctx context.Context, req *authv1.DisableTOTPRequest) (*authv1.DisableTOTPResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	err = s.totpService.Disable(ctx, userID, req.Password)
	if err != nil {
		if err == application.ErrPasswordMismatch {
			return nil, status.Error(codes.Unauthenticated, "invalid password")
		}
		if err == authrepo.ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		log.Error().Err(err).Msg("ошибка отключения TOTP")
		return nil, status.Error(codes.Internal, "failed to disable TOTP")
	}

	return &authv1.DisableTOTPResponse{
		Success: true,
	}, nil
}

// RequestPasswordReset генерирует токен сброса пароля
func (s *Server) RequestPasswordReset(ctx context.Context, req *authv1.RequestPasswordResetRequest) (*authv1.RequestPasswordResetResponse, error) {
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	token, _, err := s.passwordResetService.RequestReset(ctx, req.Email)
	if err != nil {
		// Всегда возвращаем success=true для предотвращения перечисления email
		if err == authrepo.ErrUserNotFound {
			return &authv1.RequestPasswordResetResponse{
				Success: true,
			}, nil
		}
		if err == application.ErrPasswordResetRateLimit {
			return nil, status.Error(codes.ResourceExhausted, "too many reset requests")
		}
		log.Error().Err(err).Msg("ошибка запроса сброса пароля")
		return nil, status.Error(codes.Internal, "failed to request password reset")
	}

	return &authv1.RequestPasswordResetResponse{
		Success:    true,
		ResetToken: token,
	}, nil
}

// ResetPassword сбрасывает пароль по токену
func (s *Server) ResetPassword(ctx context.Context, req *authv1.ResetPasswordRequest) (*authv1.ResetPasswordResponse, error) {
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}
	if req.NewPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "new_password is required")
	}

	err := s.passwordResetService.ResetPassword(ctx, req.Token, req.NewPassword)
	if err != nil {
		if err == application.ErrPasswordResetInvalid {
			return nil, status.Error(codes.InvalidArgument, "invalid or expired reset token")
		}
		if err == domain.ErrPasswordResetTokenUsed {
			return nil, status.Error(codes.InvalidArgument, "reset token already used")
		}
		if err == domain.ErrPasswordResetTokenExpired {
			return nil, status.Error(codes.InvalidArgument, "reset token expired")
		}
		log.Error().Err(err).Msg("ошибка сброса пароля")
		return nil, status.Error(codes.Internal, "failed to reset password")
	}

	return &authv1.ResetPasswordResponse{
		Success: true,
	}, nil
}

// LoginWithSession выполняет аутентификацию и создает сессию
func (s *Server) LoginWithSession(ctx context.Context, req *authv1.LoginWithSessionRequest) (*authv1.LoginWithSessionResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	// Аутентифицируем пользователя по credentials (без генерации JWT)
	user, _, _, err := s.authService.AuthenticateByCredentials(ctx, req.Email, req.Password)
	if err != nil {
		if err == application.ErrInvalidCredentials {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		if err == application.ErrUserInactive {
			return nil, status.Error(codes.PermissionDenied, "user is inactive")
		}
		log.Error().Err(err).Msg("ошибка аутентификации при LoginWithSession")
		return nil, status.Error(codes.Internal, "authentication failed")
	}

	// Проверяем, включен ли TOTP
	totpEnabled := s.isTOTPEnabled(ctx, user.ID)

	if totpEnabled {
		if req.TotpCode == "" {
			// TOTP включен, но код не предоставлен - возвращаем login_ticket
			ticket, err := s.sessionManager.StoreLoginTicket(ctx, user.ID)
			if err != nil {
				log.Error().Err(err).Msg("ошибка создания login ticket")
				return nil, status.Error(codes.Internal, "failed to create login ticket")
			}

			return &authv1.LoginWithSessionResponse{
				Requires_2Fa:  true,
				LoginTicket: ticket,
			}, nil
		}

		// TOTP включен и код предоставлен - валидируем
		valid, err := s.totpService.ValidateCode(ctx, user.ID, req.TotpCode)
		if err != nil {
			log.Error().Err(err).Msg("ошибка валидации TOTP кода")
			return nil, status.Error(codes.Internal, "failed to validate TOTP code")
		}
		if !valid {
			// Попробуем как код восстановления
			valid, err = s.totpService.ValidateRecoveryCode(ctx, user.ID, req.TotpCode)
			if err != nil {
				log.Error().Err(err).Msg("ошибка валидации кода восстановления")
				return nil, status.Error(codes.Internal, "failed to validate recovery code")
			}
			if !valid {
				return nil, status.Error(codes.Unauthenticated, "invalid TOTP code")
			}
		}
	}

	// Создаем сессию
	clientID := uuid.Nil
	if user.ClientID != nil {
		clientID = *user.ClientID
	}
	roleName := ""
	if user.Role != nil {
		roleName = user.Role.Name
	}

	sessionID, err := s.sessionManager.CreateSession(
		ctx, user.ID, clientID, roleName, req.IpAddress, req.UserAgent,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания сессии")
		return nil, status.Error(codes.Internal, "failed to create session")
	}

	return &authv1.LoginWithSessionResponse{
		SessionId:    sessionID,
		User:         s.domainUserToProto(user),
		Requires_2Fa: false,
	}, nil
}

// ValidateSession проверяет валидность сессии
func (s *Server) ValidateSession(ctx context.Context, req *authv1.ValidateSessionRequest) (*authv1.ValidateSessionResponse, error) {
	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	userID, clientID, _, err := s.sessionManager.ValidateSession(ctx, req.SessionId)
	if err != nil {
		if err == authinfra.ErrSessionNotFound || err == authinfra.ErrSessionExpired {
			return &authv1.ValidateSessionResponse{
				Valid: false,
			}, nil
		}
		log.Error().Err(err).Msg("ошибка валидации сессии")
		return nil, status.Error(codes.Internal, "failed to validate session")
	}

	// Загружаем информацию о пользователе
	user, err := s.userRepo.GetByIDWithRole(ctx, userID)
	if err != nil {
		if err == authrepo.ErrUserNotFound {
			return &authv1.ValidateSessionResponse{
				Valid: false,
			}, nil
		}
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	// Проверяем активность пользователя
	if !user.IsActive() {
		return &authv1.ValidateSessionResponse{
			Valid: false,
		}, nil
	}

	return &authv1.ValidateSessionResponse{
		Valid:    true,
		User:     s.domainUserToProto(user),
		ClientId: clientID.String(),
	}, nil
}

// Logout уничтожает сессию
func (s *Server) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	err := s.sessionManager.DestroySession(ctx, req.SessionId)
	if err != nil {
		log.Error().Err(err).Msg("ошибка уничтожения сессии")
		return nil, status.Error(codes.Internal, "failed to destroy session")
	}

	return &authv1.LogoutResponse{
		Success: true,
	}, nil
}

// RegisterClient регистрирует нового клиента (public self-service)
func (s *Server) RegisterClient(ctx context.Context, req *authv1.RegisterClientRequest) (*authv1.RegisterClientResponse, error) {
	// Валидация входных данных
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}
	if req.CompanyName == "" {
		return nil, status.Error(codes.InvalidArgument, "company_name is required")
	}

	// Проверяем, что email не занят
	_, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil {
		return nil, status.Error(codes.AlreadyExists, "email already registered")
	}
	if err != authrepo.ErrUserNotFound {
		log.Error().Err(err).Msg("ошибка проверки email при регистрации")
		return nil, status.Error(codes.Internal, "registration failed")
	}

	// Создаем клиента через client service
	createClientResp, err := s.clientService.CreateClient(ctx, &clientv1.CreateClientRequest{
		Name:          req.CompanyName,
		Email:         req.Email,
		ContactPerson: req.ContactPerson,
		Phone:         req.Phone,
		Active:        true,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания клиента при регистрации")
		return nil, status.Error(codes.Internal, "failed to create client")
	}
	clientID := createClientResp.ClientId

	// Компенсирующее действие: удалить клиента при ошибке на любом следующем шаге
	var registrationErr error
	defer func() {
		if registrationErr != nil && clientID != "" {
			if _, delErr := s.clientService.DeleteClient(ctx, &clientv1.DeleteClientRequest{ClientId: clientID}); delErr != nil {
				log.Error().Err(delErr).Str("client_id", clientID).Msg("ошибка компенсации: не удалось удалить клиента после падения регистрации")
			} else {
				log.Info().Str("client_id", clientID).Msg("компенсация: клиент удалён после падения регистрации")
			}
		}
	}()

	// Назначаем тарифный план, если указан
	if req.PlanName != "" {
		// Получаем список планов, чтобы найти ID по имени
		plansResp, err := s.clientService.ListPlans(ctx, &clientv1.ListPlansRequest{})
		if err != nil {
			log.Warn().Err(err).Msg("не удалось получить список планов при регистрации, пропускаем назначение плана")
		} else {
			planName := strings.ToLower(req.PlanName)
			for _, plan := range plansResp.Plans {
				if strings.ToLower(plan.Name) == planName {
					_, assignErr := s.clientService.AssignPlan(ctx, &clientv1.AssignPlanRequest{
						ClientId: clientID,
						PlanId:   plan.Id,
					})
					if assignErr != nil {
						log.Warn().Err(assignErr).Str("plan", req.PlanName).Msg("не удалось назначить план клиенту")
					}
					break
				}
			}
		}
	}

	// Хешируем пароль
	passwordHash, err := s.passwordHasher.HashPassword(req.Password)
	if err != nil {
		log.Error().Err(err).Msg("ошибка хеширования пароля при регистрации")
		registrationErr = err
		return nil, status.Error(codes.Internal, "registration failed")
	}

	// Получаем роль "client"
	clientRole, err := s.roleRepo.GetByName(ctx, "client")
	if err != nil {
		log.Error().Err(err).Msg("роль 'client' не найдена при регистрации")
		registrationErr = err
		return nil, status.Error(codes.Internal, "registration failed")
	}

	// Создаем пользователя
	now := time.Now()
	// Используем email как username (уникально)
	username := req.Email
	clientUUIDForUser, _ := uuid.Parse(clientID)
	user := &domain.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		RoleID:       clientRole.ID,
		Active:       true,
		ClientID:     &clientUUIDForUser,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	user.Role = clientRole

	if err := s.userRepo.Create(ctx, user); err != nil {
		log.Error().Err(err).Msg("ошибка создания пользователя при регистрации")
		registrationErr = err
		return nil, status.Error(codes.Internal, "registration failed")
	}

	// Создаем сессию (авто-логин)
	sessionID, err := s.sessionManager.CreateSession(
		ctx, user.ID, clientUUIDForUser, clientRole.Name, "", "",
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания сессии при регистрации")
		return nil, status.Error(codes.Internal, "registration failed")
	}

	return &authv1.RegisterClientResponse{
		ClientId:  clientID,
		UserId:    user.ID.String(),
		SessionId: sessionID,
		User:      s.domainUserToProto(user),
	}, nil
}

// isTOTPEnabled проверяет, включен ли TOTP для пользователя
func (s *Server) isTOTPEnabled(ctx context.Context, userID uuid.UUID) bool {
	// Пробуем валидировать с пустым кодом - если TOTP не включен, получим ErrTOTPNotEnabled
	_, err := s.totpService.ValidateCode(ctx, userID, "000000")
	if err == domain.ErrTOTPNotEnabled {
		return false
	}
	// Если ошибка другая (включая "invalid code") - значит TOTP включен
	// Если нет ошибки (маловероятно) - тоже включен
	return true
}

// ============================================================
// User/Role Management gRPC methods
// ============================================================

// CreateUser создает нового пользователя (админ)
func (s *Server) CreateUser(ctx context.Context, req *authv1.CreateUserRequest) (*authv1.CreateUserResponse, error) {
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}
	if req.RoleId == "" {
		return nil, status.Error(codes.InvalidArgument, "role_id is required")
	}

	roleID, err := uuid.Parse(req.RoleId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid role_id format")
	}

	user, err := s.authService.CreateUser(ctx, req.Username, req.Email, req.Password, roleID, req.Active)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания пользователя")
		return nil, status.Error(codes.Internal, "failed to create user")
	}

	return &authv1.CreateUserResponse{
		User: s.domainUserToProto(user),
	}, nil
}

// UpdateUser обновляет пользователя
func (s *Server) UpdateUser(ctx context.Context, req *authv1.UpdateUserRequest) (*authv1.UpdateUserResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	roleID := uuid.Nil
	if req.RoleId != "" {
		roleID, err = uuid.Parse(req.RoleId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid role_id format")
		}
	}

	var clientID *uuid.UUID
	if req.ClientId != "" {
		parsed, parseErr := uuid.Parse(req.ClientId)
		if parseErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &parsed
	}

	// Если передан только client_id (без email/role) — используем специализированный метод,
	// чтобы не затирать остальные поля пользователя нулевыми значениями.
	var user *domain.User
	if req.Email == "" && req.RoleId == "" && clientID != nil {
		user, err = s.authService.AssignClientToUser(ctx, userID, *clientID)
	} else {
		user, err = s.authService.UpdateUser(ctx, userID, req.Email, roleID, req.Active, clientID)
	}
	if err != nil {
		if err == authrepo.ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		log.Error().Err(err).Msg("ошибка обновления пользователя")
		return nil, status.Error(codes.Internal, "failed to update user")
	}

	return &authv1.UpdateUserResponse{
		User: s.domainUserToProto(user),
	}, nil
}

// DeactivateUser деактивирует пользователя
func (s *Server) DeactivateUser(ctx context.Context, req *authv1.DeactivateUserRequest) (*authv1.DeactivateUserResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	err = s.authService.DeactivateUser(ctx, userID)
	if err != nil {
		if err == authrepo.ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		log.Error().Err(err).Msg("ошибка деактивации пользователя")
		return nil, status.Error(codes.Internal, "failed to deactivate user")
	}

	return &authv1.DeactivateUserResponse{
		Success: true,
	}, nil
}

// ResetUser2FA сбрасывает 2FA пользователя (админ)
func (s *Server) ResetUser2FA(ctx context.Context, req *authv1.ResetUser2FARequest) (*authv1.ResetUser2FAResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	err = s.authService.ResetUser2FA(ctx, userID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка сброса 2FA пользователя")
		return nil, status.Error(codes.Internal, "failed to reset user 2FA")
	}

	return &authv1.ResetUser2FAResponse{
		Success: true,
	}, nil
}

// ResetUserPassword сбрасывает пароль пользователя (админ)
func (s *Server) ResetUserPassword(ctx context.Context, req *authv1.ResetUserPasswordRequest) (*authv1.ResetUserPasswordResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	tempPassword, err := s.authService.ResetUserPassword(ctx, userID)
	if err != nil {
		if err == authrepo.ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		log.Error().Err(err).Msg("ошибка сброса пароля пользователя")
		return nil, status.Error(codes.Internal, "failed to reset user password")
	}

	return &authv1.ResetUserPasswordResponse{
		TemporaryPassword: tempPassword,
	}, nil
}

// ListUsers получает список пользователей с фильтрацией
func (s *Server) ListUsers(ctx context.Context, req *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
	users, total, err := s.authService.ListUsers(ctx, req.Search, req.RoleId, req.ActiveOnly, req.Limit, req.Offset)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка пользователей")
		return nil, status.Error(codes.Internal, "failed to list users")
	}

	protoUsers := make([]*authv1.UserDetailInfo, len(users))
	for i, user := range users {
		protoUsers[i] = s.domainUserToDetailProto(ctx, user)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	return &authv1.ListUsersResponse{
		Users:  protoUsers,
		Total:  total,
		Limit:  limit,
		Offset: req.Offset,
	}, nil
}

// GetUser получает информацию о пользователе
func (s *Server) GetUser(ctx context.Context, req *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	user, err := s.authService.GetUser(ctx, userID)
	if err != nil {
		if err == authrepo.ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		log.Error().Err(err).Msg("ошибка получения пользователя")
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	return &authv1.GetUserResponse{
		User: s.domainUserToDetailProto(ctx, user),
	}, nil
}

// CreateRole создает новую роль
func (s *Server) CreateRole(ctx context.Context, req *authv1.CreateRoleRequest) (*authv1.CreateRoleResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	permissionIDs, err := parseUUIDs(req.PermissionIds)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid permission_id format")
	}

	role, err := s.authService.CreateRole(ctx, req.Name, req.Description, permissionIDs)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания роли")
		return nil, status.Error(codes.Internal, "failed to create role")
	}

	return &authv1.CreateRoleResponse{
		Role: s.domainRoleToDetailProto(role),
	}, nil
}

// UpdateRole обновляет роль
func (s *Server) UpdateRole(ctx context.Context, req *authv1.UpdateRoleRequest) (*authv1.UpdateRoleResponse, error) {
	if req.RoleId == "" {
		return nil, status.Error(codes.InvalidArgument, "role_id is required")
	}

	roleID, err := uuid.Parse(req.RoleId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid role_id format")
	}

	permissionIDs, err := parseUUIDs(req.PermissionIds)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid permission_id format")
	}

	role, err := s.authService.UpdateRole(ctx, roleID, req.Name, req.Description, permissionIDs)
	if err != nil {
		if err == authrepo.ErrRoleNotFound {
			return nil, status.Error(codes.NotFound, "role not found")
		}
		log.Error().Err(err).Msg("ошибка обновления роли")
		return nil, status.Error(codes.Internal, "failed to update role")
	}

	return &authv1.UpdateRoleResponse{
		Role: s.domainRoleToDetailProto(role),
	}, nil
}

// DeleteRole удаляет роль
func (s *Server) DeleteRole(ctx context.Context, req *authv1.DeleteRoleRequest) (*authv1.DeleteRoleResponse, error) {
	if req.RoleId == "" {
		return nil, status.Error(codes.InvalidArgument, "role_id is required")
	}

	roleID, err := uuid.Parse(req.RoleId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid role_id format")
	}

	err = s.authService.DeleteRole(ctx, roleID)
	if err != nil {
		if err == authrepo.ErrRoleNotFound {
			return nil, status.Error(codes.NotFound, "role not found")
		}
		log.Error().Err(err).Msg("ошибка удаления роли")
		return nil, status.Error(codes.Internal, "failed to delete role")
	}

	return &authv1.DeleteRoleResponse{
		Success: true,
	}, nil
}

// ListRoles получает список ролей
func (s *Server) ListRoles(ctx context.Context, req *authv1.ListRolesRequest) (*authv1.ListRolesResponse, error) {
	roles, total, err := s.authService.ListRoles(ctx, req.Limit, req.Offset)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка ролей")
		return nil, status.Error(codes.Internal, "failed to list roles")
	}

	protoRoles := make([]*authv1.RoleDetail, len(roles))
	for i, role := range roles {
		protoRoles[i] = s.domainRoleToDetailProto(role)
	}

	return &authv1.ListRolesResponse{
		Roles: protoRoles,
		Total: total,
	}, nil
}

// GetRole получает информацию о роли
func (s *Server) GetRole(ctx context.Context, req *authv1.GetRoleRequest) (*authv1.GetRoleResponse, error) {
	if req.RoleId == "" {
		return nil, status.Error(codes.InvalidArgument, "role_id is required")
	}

	roleID, err := uuid.Parse(req.RoleId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid role_id format")
	}

	role, err := s.authService.GetRole(ctx, roleID)
	if err != nil {
		if err == authrepo.ErrRoleNotFound {
			return nil, status.Error(codes.NotFound, "role not found")
		}
		log.Error().Err(err).Msg("ошибка получения роли")
		return nil, status.Error(codes.Internal, "failed to get role")
	}

	return &authv1.GetRoleResponse{
		Role: s.domainRoleToDetailProto(role),
	}, nil
}

// ListAllPermissions получает все доступные права
func (s *Server) ListAllPermissions(ctx context.Context, req *authv1.ListAllPermissionsRequest) (*authv1.ListAllPermissionsResponse, error) {
	perms, err := s.authService.ListAllPermissions(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка прав")
		return nil, status.Error(codes.Internal, "failed to list permissions")
	}

	protoPerms := make([]*authv1.Permission, len(perms))
	for i, perm := range perms {
		protoPerms[i] = &authv1.Permission{
			Id:       perm.ID.String(),
			Resource: perm.Resource,
			Action:   perm.Action,
		}
	}

	return &authv1.ListAllPermissionsResponse{
		Permissions: protoPerms,
	}, nil
}

// GetUserPermissions получает права конкретного пользователя
func (s *Server) GetUserPermissions(ctx context.Context, req *authv1.GetUserPermissionsRequest) (*authv1.GetUserPermissionsResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	perms, role, err := s.authService.GetUserPermissions(ctx, userID)
	if err != nil {
		if err == authrepo.ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		log.Error().Err(err).Msg("ошибка получения прав пользователя")
		return nil, status.Error(codes.Internal, "failed to get user permissions")
	}

	protoPerms := make([]*authv1.Permission, len(perms))
	for i, perm := range perms {
		protoPerms[i] = &authv1.Permission{
			Id:       perm.ID.String(),
			Resource: perm.Resource,
			Action:   perm.Action,
		}
	}

	var protoRole *authv1.Role
	if role != nil {
		protoRole = &authv1.Role{
			Id:          role.ID.String(),
			Name:        role.Name,
			Description: role.Description,
		}
	}

	return &authv1.GetUserPermissionsResponse{
		Permissions: protoPerms,
		Role:        protoRole,
	}, nil
}

// ============================================================
// Proto conversion helpers
// ============================================================

// domainUserToDetailProto преобразует domain.User в proto UserDetailInfo
func (s *Server) domainUserToDetailProto(ctx context.Context, user *domain.User) *authv1.UserDetailInfo {
	if user == nil {
		return nil
	}

	detail := &authv1.UserDetailInfo{
		Id:        user.ID.String(),
		Username:  user.Username,
		Email:     user.Email,
		Active:    user.Active,
		CreatedAt: timestamppb.New(user.CreatedAt),
		UpdatedAt: timestamppb.New(user.UpdatedAt),
	}

	if user.Role != nil {
		detail.Role = &authv1.Role{
			Id:          user.Role.ID.String(),
			Name:        user.Role.Name,
			Description: user.Role.Description,
		}
	}

	if user.LastLoginAt != nil {
		detail.LastLoginAt = timestamppb.New(*user.LastLoginAt)
	}

	// Проверяем, включен ли TOTP
	detail.TotpEnabled = s.isTOTPEnabled(ctx, user.ID)

	return detail
}

// domainRoleToDetailProto преобразует domain.Role в proto RoleDetail
func (s *Server) domainRoleToDetailProto(role *domain.Role) *authv1.RoleDetail {
	if role == nil {
		return nil
	}

	detail := &authv1.RoleDetail{
		Id:          role.ID.String(),
		Name:        role.Name,
		Description: role.Description,
		Builtin:     isBuiltinRole(role.Name),
		UserCount:   role.UserCount,
		CreatedAt:   timestamppb.New(role.CreatedAt),
		UpdatedAt:   timestamppb.New(role.UpdatedAt),
	}

	protoPerms := make([]*authv1.Permission, len(role.Permissions))
	for i, perm := range role.Permissions {
		protoPerms[i] = &authv1.Permission{
			Id:       perm.ID.String(),
			Resource: perm.Resource,
			Action:   perm.Action,
		}
	}
	detail.Permissions = protoPerms

	return detail
}

// isBuiltinRole проверяет, является ли роль встроенной
func isBuiltinRole(name string) bool {
	switch strings.ToLower(name) {
	case "admin", "client", "operator":
		return true
	}
	return false
}

// parseUUIDs парсит слайс строк в слайс UUID
func parseUUIDs(ids []string) ([]uuid.UUID, error) {
	result := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		result[i] = parsed
	}
	return result, nil
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

	clientId := ""
	if user.ClientID != nil {
		clientId = user.ClientID.String()
	}

	return &authv1.UserInfo{
		Id:        user.ID.String(),
		Username:  user.Username,
		Email:     user.Email,
		Role:      role,
		Active:    user.Active,
		CreatedAt: timestamppb.New(user.CreatedAt),
		ClientId:  clientId,
	}
}
