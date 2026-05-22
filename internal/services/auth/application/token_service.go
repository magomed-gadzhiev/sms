package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	authrepo "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure/repository"
)

// Ensure authrepo is used (re-export sentinel error for callers).
var ErrRefreshTokenNotFound = authrepo.ErrRefreshTokenNotFound

var (
	ErrInvalidToken      = errors.New("invalid token")
	ErrExpiredToken      = errors.New("token expired")
	ErrTokenGeneration   = errors.New("token generation failed")
)

// TokenClaims представляет claims JWT токена
type TokenClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// TokenService предоставляет методы для работы с JWT токенами
type TokenService struct {
	secret           []byte
	accessExpiry     time.Duration
	refreshExpiry    time.Duration
	refreshTokenRepo RefreshTokenRepository
}

// NewTokenService создает новый сервис токенов
func NewTokenService(
	secret string,
	accessExpiry time.Duration,
	refreshExpiry time.Duration,
	refreshTokenRepo RefreshTokenRepository,
) *TokenService {
	return &TokenService{
		secret:           []byte(secret),
		accessExpiry:     accessExpiry,
		refreshExpiry:    refreshExpiry,
		refreshTokenRepo: refreshTokenRepo,
	}
}

// GenerateTokenPair генерирует пару access и refresh токенов
func (s *TokenService) GenerateTokenPair(
	ctx context.Context,
	userID uuid.UUID,
	roleName string,
) (string, string, error) {
	// Генерируем access token
	accessToken, err := s.generateAccessToken(userID.String(), roleName)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrTokenGeneration, err)
	}

	// Генерируем refresh token
	refreshToken, err := s.generateRefreshToken(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrTokenGeneration, err)
	}

	return accessToken, refreshToken, nil
}

// generateAccessToken генерирует access token
func (s *TokenService) generateAccessToken(userID, roleName string) (string, error) {
	now := time.Now()
	claims := TokenClaims{
		UserID: userID,
		Role:   roleName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// generateRefreshToken генерирует refresh token и сохраняет его в БД
func (s *TokenService) generateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	// Генерируем случайный токен
	tokenString := uuid.New().String() + uuid.New().String()

	// Хешируем токен для хранения
	tokenHash := s.hashToken(tokenString)

	// Создаем запись refresh токена
	refreshToken := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(s.refreshExpiry),
		Revoked:   false,
		CreatedAt: time.Now(),
	}

	if err := s.refreshTokenRepo.Create(ctx, refreshToken); err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken валидирует access token и возвращает claims
func (s *TokenService) ValidateToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Проверяем метод подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		// В jwt/v5 используем errors.Is для проверки типа ошибки
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// RefreshToken обновляет access token используя refresh token
func (s *TokenService) RefreshToken(ctx context.Context, refreshTokenString string, getUserRole func(uuid.UUID) (string, error)) (string, string, error) {
	// Хешируем refresh token для поиска
	tokenHash := s.hashToken(refreshTokenString)

	// Получаем refresh token из БД
	refreshToken, err := s.refreshTokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, authrepo.ErrRefreshTokenNotFound) {
			return "", "", ErrInvalidToken
		}
		return "", "", err
	}

	// Проверяем валидность
	if !refreshToken.IsValid() {
		return "", "", ErrInvalidToken
	}

	// Отзываем старый refresh token
	if err := s.refreshTokenRepo.Revoke(ctx, tokenHash); err != nil {
		log.Warn().Err(err).Msg("не удалось отозвать старый refresh token")
	}

	// Получаем роль пользователя
	userID := refreshToken.UserID
	roleName, err := getUserRole(userID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get user role: %w", err)
	}

	// Генерируем новую пару токенов
	return s.GenerateTokenPair(ctx, userID, roleName)
}

// RevokeRefreshToken отзывает refresh token
func (s *TokenService) RevokeRefreshToken(ctx context.Context, refreshTokenString string) error {
	tokenHash := s.hashToken(refreshTokenString)
	return s.refreshTokenRepo.Revoke(ctx, tokenHash)
}

// hashToken хеширует токен используя SHA256 (для refresh токенов)
func (s *TokenService) hashToken(token string) string {
	// Используем SHA256 для хеширования refresh токенов
	// В production можно использовать bcrypt, но для токенов SHA256 достаточно
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
