package infrastructure

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

var (
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionExpired      = errors.New("session expired")
	ErrLoginTicketNotFound = errors.New("login ticket not found or expired")
)

const (
	sessionPrefix     = "session:"
	loginTicketPrefix = "login_ticket:"
	sessionTTL        = 24 * time.Hour
	loginTicketTTL    = 5 * time.Minute
	sessionIDBytes    = 32 // 32 bytes = 64 hex chars
)

// SessionManager управляет сессиями пользователей
type SessionManager struct {
	redisClient *redis.Client
	db          *sqlx.DB
	maxSessions int
}

// NewSessionManager создает новый менеджер сессий
func NewSessionManager(redisClient *redis.Client, db *database.DB, maxSessions int) *SessionManager {
	return &SessionManager{
		redisClient: redisClient,
		db:          sqlx.NewDb(db.DB, "pgx"),
		maxSessions: maxSessions,
	}
}

// CreateSession создает новую сессию для пользователя
func (m *SessionManager) CreateSession(
	ctx context.Context,
	userID uuid.UUID,
	clientID uuid.UUID,
	role string,
	ipAddress string,
	userAgent string,
) (string, error) {
	// Генерируем криптографически случайный ID сессии
	sessionID, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("failed to generate session ID: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(sessionTTL)

	// Сохраняем в Redis hash
	redisKey := sessionPrefix + sessionID
	fields := map[string]interface{}{
		"user_id":    userID.String(),
		"client_id":  clientID.String(),
		"role":       role,
		"ip_address": ipAddress,
		"user_agent": userAgent,
		"created_at": now.Format(time.RFC3339),
		"expires_at": expiresAt.Format(time.RFC3339),
	}

	pipe := m.redisClient.Pipeline()
	pipe.HSet(ctx, redisKey, fields)
	pipe.ExpireAt(ctx, redisKey, expiresAt)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("failed to store session in Redis: %w", err)
	}

	// Сохраняем в PostgreSQL (client_id = NULL если uuid.Nil)
	var clientIDParam interface{} = clientID
	if clientID == uuid.Nil {
		clientIDParam = nil
	}
	_, err = m.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, client_id, role, ip_address, user_agent, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		sessionID, userID, clientIDParam, role, ipAddress, userAgent, now, expiresAt,
	)
	if err != nil {
		// Откатываем Redis при ошибке PostgreSQL
		m.redisClient.Del(ctx, redisKey)
		return "", fmt.Errorf("failed to store session in PostgreSQL: %w", err)
	}

	// Ограничиваем количество сессий пользователя
	if err := m.enforceMaxSessions(ctx, userID); err != nil {
		log.Warn().Err(err).Str("user_id", userID.String()).Msg("не удалось ограничить количество сессий")
	}

	return sessionID, nil
}

// ValidateSession проверяет валидность сессии
func (m *SessionManager) ValidateSession(ctx context.Context, sessionID string) (uuid.UUID, uuid.UUID, string, error) {
	redisKey := sessionPrefix + sessionID

	result, err := m.redisClient.HGetAll(ctx, redisKey).Result()
	if err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("failed to get session from Redis: %w", err)
	}

	if len(result) == 0 {
		return uuid.Nil, uuid.Nil, "", ErrSessionNotFound
	}

	// Проверяем время истечения
	expiresAt, err := time.Parse(time.RFC3339, result["expires_at"])
	if err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("failed to parse expires_at: %w", err)
	}

	if time.Now().After(expiresAt) {
		// Удаляем просроченную сессию
		m.redisClient.Del(ctx, redisKey)
		return uuid.Nil, uuid.Nil, "", ErrSessionExpired
	}

	userID, err := uuid.Parse(result["user_id"])
	if err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("failed to parse user_id: %w", err)
	}

	clientID, err := uuid.Parse(result["client_id"])
	if err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("failed to parse client_id: %w", err)
	}

	role := result["role"]

	return userID, clientID, role, nil
}

// DestroySession уничтожает сессию
func (m *SessionManager) DestroySession(ctx context.Context, sessionID string) error {
	// Удаляем из Redis
	redisKey := sessionPrefix + sessionID
	if err := m.redisClient.Del(ctx, redisKey).Err(); err != nil {
		return fmt.Errorf("failed to delete session from Redis: %w", err)
	}

	// Удаляем из PostgreSQL
	_, err := m.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session from PostgreSQL: %w", err)
	}

	return nil
}

// DestroyAllSessions уничтожает все сессии пользователя
func (m *SessionManager) DestroyAllSessions(ctx context.Context, userID uuid.UUID) error {
	// Получаем все сессии пользователя из PostgreSQL
	var sessionIDs []string
	err := m.db.SelectContext(ctx, &sessionIDs,
		`SELECT id FROM sessions WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	// Удаляем все из Redis
	if len(sessionIDs) > 0 {
		redisKeys := make([]string, len(sessionIDs))
		for i, id := range sessionIDs {
			redisKeys[i] = sessionPrefix + id
		}
		if err := m.redisClient.Del(ctx, redisKeys...).Err(); err != nil {
			log.Warn().Err(err).Msg("не удалось удалить сессии из Redis")
		}
	}

	// Удаляем все из PostgreSQL
	_, err = m.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete sessions from PostgreSQL: %w", err)
	}

	return nil
}

// StoreLoginTicket сохраняет временный тикет для 2FA потока
func (m *SessionManager) StoreLoginTicket(ctx context.Context, userID uuid.UUID) (string, error) {
	ticket, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("failed to generate login ticket: %w", err)
	}

	redisKey := loginTicketPrefix + ticket
	err = m.redisClient.Set(ctx, redisKey, userID.String(), loginTicketTTL).Err()
	if err != nil {
		return "", fmt.Errorf("failed to store login ticket: %w", err)
	}

	return ticket, nil
}

// ValidateLoginTicket проверяет и удаляет тикет (одноразовое использование)
func (m *SessionManager) ValidateLoginTicket(ctx context.Context, ticket string) (uuid.UUID, error) {
	redisKey := loginTicketPrefix + ticket

	// GetDel - атомарная операция: получить и удалить
	userIDStr, err := m.redisClient.GetDel(ctx, redisKey).Result()
	if err != nil {
		if err == redis.Nil {
			return uuid.Nil, ErrLoginTicketNotFound
		}
		return uuid.Nil, fmt.Errorf("failed to validate login ticket: %w", err)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse user_id from ticket: %w", err)
	}

	return userID, nil
}

// enforceMaxSessions удаляет самые старые сессии, если превышен лимит
func (m *SessionManager) enforceMaxSessions(ctx context.Context, userID uuid.UUID) error {
	// Получаем ID самых старых сессий, которые нужно удалить
	var oldSessionIDs []string
	err := m.db.SelectContext(ctx, &oldSessionIDs,
		`SELECT id FROM sessions WHERE user_id = $1
		 ORDER BY created_at DESC
		 OFFSET $2`, userID, m.maxSessions)
	if err != nil {
		return fmt.Errorf("failed to get old sessions: %w", err)
	}

	if len(oldSessionIDs) == 0 {
		return nil
	}

	// Удаляем из Redis
	redisKeys := make([]string, len(oldSessionIDs))
	for i, id := range oldSessionIDs {
		redisKeys[i] = sessionPrefix + id
	}
	if err := m.redisClient.Del(ctx, redisKeys...).Err(); err != nil {
		log.Warn().Err(err).Msg("не удалось удалить старые сессии из Redis")
	}

	// Удаляем из PostgreSQL
	query, args, err := sqlx.In(
		`DELETE FROM sessions WHERE id IN (?)`, oldSessionIDs)
	if err != nil {
		return fmt.Errorf("failed to build delete query: %w", err)
	}
	query = m.db.Rebind(query)
	_, err = m.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete old sessions: %w", err)
	}

	return nil
}

// generateSessionID генерирует криптографически случайный ID сессии (64 hex символа)
func generateSessionID() (string, error) {
	bytes := make([]byte, sessionIDBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
