package session

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// SessionState представляет состояние SMPP сессии
type SessionState int

const (
	SessionStateOpen SessionState = iota
	SessionStateBoundRX
	SessionStateBoundTX
	SessionStateBoundTRX
	SessionStateClosed
)

// String возвращает строковое представление состояния сессии
func (s SessionState) String() string {
	switch s {
	case SessionStateOpen:
		return "OPEN"
	case SessionStateBoundRX:
		return "BOUND_RX"
	case SessionStateBoundTX:
		return "BOUND_TX"
	case SessionStateBoundTRX:
		return "BOUND_TRX"
	case SessionStateClosed:
		return "CLOSED"
	default:
		return "UNKNOWN"
	}
}

// Session представляет SMPP сессию
type Session struct {
	ID              string
	Conn            net.Conn
	State           SessionState
	SystemID        string
	ClientID        *uuid.UUID
	UserID          string // ID пользователя из Auth Service
	BindType        string // receiver, transmitter, transceiver
	SequenceNumber  uint32
	LastActivity    time.Time
	EnquireLinkSent time.Time
	RateLimiter     *RateLimiter
	Logger          zerolog.Logger
	
	// Защита от конкурентного доступа
	mu sync.RWMutex
	
	// Контекст для отмены операций
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSession создает новую SMPP сессию
func NewSession(conn net.Conn, logger zerolog.Logger) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Генерируем уникальный ID сессии
	sessionID := generateSessionID()
	
	return &Session{
		ID:             sessionID,
		Conn:           conn,
		State:          SessionStateOpen,
		SequenceNumber: 1,
		LastActivity:   time.Now(),
		RateLimiter:    NewRateLimiter(100), // По умолчанию 100 сообщений в секунду
		Logger:         logger.With().Str("session_id", sessionID).Logger(),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// generateSessionID генерирует уникальный ID сессии
func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", binary.BigEndian.Uint64(b))
}

// Bind выполняет bind операцию
func (s *Session) Bind(bindType string, systemID string, clientID *uuid.UUID, userID string, rateLimit int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.State != SessionStateOpen {
		return fmt.Errorf("сессия уже привязана (состояние: %s)", s.State)
	}
	
	s.BindType = bindType
	s.SystemID = systemID
	s.ClientID = clientID
	s.UserID = userID
	s.LastActivity = time.Now()
	
	if rateLimit > 0 {
		s.RateLimiter = NewRateLimiter(rateLimit)
	}
	
	switch bindType {
	case "receiver":
		s.State = SessionStateBoundRX
	case "transmitter":
		s.State = SessionStateBoundTX
	case "transceiver":
		s.State = SessionStateBoundTRX
	default:
		return fmt.Errorf("неизвестный тип bind: %s", bindType)
	}
	
	s.Logger.Info().
		Str("bind_type", bindType).
		Str("system_id", systemID).
		Str("user_id", userID).
		Msg("сессия привязана")
	
	return nil
}

// Unbind выполняет unbind операцию
func (s *Session) Unbind() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.State == SessionStateOpen || s.State == SessionStateClosed {
		return fmt.Errorf("сессия не привязана (состояние: %s)", s.State)
	}
	
	s.State = SessionStateClosed
	s.Logger.Info().Msg("сессия отвязана")
	
	return nil
}

// NextSequenceNumber возвращает следующий sequence number
func (s *Session) NextSequenceNumber() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	seqNum := s.SequenceNumber
	s.SequenceNumber++
	if s.SequenceNumber == 0 {
		s.SequenceNumber = 1 // Избегаем 0
	}
	return seqNum
}

// UpdateActivity обновляет время последней активности
func (s *Session) UpdateActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActivity = time.Now()
}

// IsBound проверяет, привязана ли сессия
func (s *Session) IsBound() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State == SessionStateBoundRX || 
		   s.State == SessionStateBoundTX || 
		   s.State == SessionStateBoundTRX
}

// CanSend проверяет, может ли сессия отправлять сообщения
func (s *Session) CanSend() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State == SessionStateBoundTX || s.State == SessionStateBoundTRX
}

// CanReceive проверяет, может ли сессия получать сообщения
func (s *Session) CanReceive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State == SessionStateBoundRX || s.State == SessionStateBoundTRX
}

// Close закрывает сессию
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.State == SessionStateClosed {
		return nil
	}
	
	s.State = SessionStateClosed
	s.cancel()
	
	if s.Conn != nil {
		if err := s.Conn.Close(); err != nil {
			s.Logger.Error().Err(err).Msg("ошибка закрытия соединения")
			return err
		}
	}
	
	s.Logger.Info().Msg("сессия закрыта")
	return nil
}

// RemoteAddr возвращает удаленный адрес
func (s *Session) RemoteAddr() net.Addr {
	if s.Conn != nil {
		return s.Conn.RemoteAddr()
	}
	return nil
}

// CheckRateLimit проверяет rate limit
func (s *Session) CheckRateLimit() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.RateLimiter.Allow()
}

// GetContext возвращает контекст сессии
func (s *Session) GetContext() context.Context {
	return s.ctx
}

// GetConn возвращает соединение
func (s *Session) GetConn() net.Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Conn
}

// GetLastActivity возвращает время последней активности
func (s *Session) GetLastActivity() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastActivity
}

// GetEnquireLinkSent возвращает время последней отправки enquire_link
func (s *Session) GetEnquireLinkSent() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.EnquireLinkSent
}

// UpdateEnquireLinkSent обновляет время отправки enquire_link
func (s *Session) UpdateEnquireLinkSent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.EnquireLinkSent = time.Now()
}
