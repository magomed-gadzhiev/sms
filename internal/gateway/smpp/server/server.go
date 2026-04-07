package server

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/api/proto/authv1"
	"github.com/smpp-server/smpp-server/internal/config"
	smppsession "github.com/smpp-server/smpp-server/internal/gateway/smpp/session"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/smpp/protocol"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// Server представляет SMPP Gateway сервер
type Server struct {
	config      *config.SMSPConfig
	listener    net.Listener
	sessions    map[string]*smppsession.Session
	sessionsMu  sync.RWMutex
	authClient  authv1.AuthServiceClient
	messageRepo *storage.MessageRepository
	optOutRepo  *storage.OptOutRepository
	producer    *queue.Producer
	logger      zerolog.Logger

	// Контекст для graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}


// UserInfo содержит информацию о пользователе из Auth Service
type UserInfo struct {
	UserID        string
	ClientID      string
	RateLimit     int
	Active        bool
}

// NewServer создает новый SMPP Gateway сервер
func NewServer(
	cfg *config.SMSPConfig,
	authClient authv1.AuthServiceClient,
	messageRepo *storage.MessageRepository,
	optOutRepo *storage.OptOutRepository,
	producer *queue.Producer,
	logger zerolog.Logger,
) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		config:      cfg,
		sessions:    make(map[string]*smppsession.Session),
		authClient:  authClient,
		messageRepo: messageRepo,
		optOutRepo:  optOutRepo,
		producer:    producer,
		logger:      logger.With().Str("component", "smpp_gateway").Logger(),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start запускает SMPP Gateway сервер
func (s *Server) Start() error {
	addr := s.config.GetAddr()
	
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("ошибка запуска TCP сервера на %s: %w", addr, err)
	}
	
	s.listener = listener
	s.logger.Info().Str("addr", addr).Msg("SMPP Gateway запущен")
	
	// Запускаем горутину для принятия соединений
	s.wg.Add(1)
	go s.acceptConnections()
	
	// Запускаем горутину для очистки неактивных сессий
	s.wg.Add(1)
	go s.cleanupInactiveSessions()
	
	// Запускаем горутину для отправки enquire_link
	s.wg.Add(1)
	go s.enquireLinkLoop()
	
	return nil
}

// Stop останавливает SMPP Gateway сервер
func (s *Server) Stop() error {
	s.logger.Info().Msg("остановка SMPP Gateway")
	
	// Отменяем контекст
	s.cancel()
	
	// Закрываем listener
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.logger.Error().Err(err).Msg("ошибка закрытия listener")
		}
	}
	
	// Закрываем все сессии
	s.sessionsMu.Lock()
	for _, session := range s.sessions {
		if err := session.Close(); err != nil {
			s.logger.Error().Err(err).Str("session_id", session.ID).Msg("ошибка закрытия сессии")
		}
	}
	s.sessions = make(map[string]*smppsession.Session)
	s.sessionsMu.Unlock()
	
	// Ждем завершения всех горутин
	s.wg.Wait()
	
	s.logger.Info().Msg("SMPP Gateway остановлен")
	return nil
}

// acceptConnections принимает входящие соединения
func (s *Server) acceptConnections() {
	defer s.wg.Done()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.logger.Error().Err(err).Msg("ошибка принятия соединения")
				continue
			}
		}
		
		// Обрабатываем соединение в отдельной горутине
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection обрабатывает одно соединение
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()
	
	// Создаем сессию
	session := smppsession.NewSession(conn, s.logger)
	
	// Добавляем сессию в map
	s.sessionsMu.Lock()
	s.sessions[session.ID] = session
	s.sessionsMu.Unlock()
	
	// Обновляем метрики соединений
	monitoring.SMPPConnectionsActive.WithLabelValues("total").Set(float64(len(s.sessions)))
	
	// Удаляем сессию при завершении
	defer func() {
		s.sessionsMu.Lock()
		delete(s.sessions, session.ID)
		s.sessionsMu.Unlock()
		monitoring.SMPPConnectionsActive.WithLabelValues("total").Set(float64(len(s.sessions)))
		session.Close()
	}()
	
	s.logger.Info().
		Str("session_id", session.ID).
		Str("remote_addr", conn.RemoteAddr().String()).
		Msg("новое соединение")
	
	// Создаем адаптер для Auth Service
	authAdapter := NewAuthAdapter(s.authClient, s.logger)
	
	// Создаем обработчик команд
	handler := NewHandler(session, authAdapter, s.messageRepo, s.optOutRepo, s.producer, s.logger)
	
	// Читаем и обрабатываем PDU
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-session.GetContext().Done():
			return
		default:
		}
		
		// Устанавливаем таймаут чтения
		if err := conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout)); err != nil {
			s.logger.Error().Err(err).Msg("ошибка установки read deadline")
			return
		}
		
		// Читаем PDU
		pdu, err := s.readPDU(conn)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Таймаут - проверяем, нужно ли закрывать соединение
				if !session.IsBound() {
					// Если сессия не привязана и нет активности, закрываем
					if time.Since(session.GetLastActivity()) > s.config.ReadTimeout*2 {
						s.logger.Warn().
							Str("session_id", session.ID).
							Msg("таймаут неактивной сессии")
						return
					}
				}
				continue
			}
			
			s.logger.Error().
				Err(err).
				Str("session_id", session.ID).
				Msg("ошибка чтения PDU")
			return
		}
		
		// Обрабатываем PDU
		if err := handler.HandlePDU(pdu); err != nil {
			s.logger.Error().
				Err(err).
				Str("session_id", session.ID).
				Uint32("command_id", pdu.CommandID).
				Msg("ошибка обработки PDU")
			// Продолжаем обработку других PDU
		}
	}
}

// readPDU читает один PDU из соединения
func (s *Server) readPDU(conn net.Conn) (*protocol.PDU, error) {
	// Читаем заголовок (16 байт)
	header := make([]byte, protocol.PDUHeaderLength)
	if _, err := conn.Read(header); err != nil {
		return nil, fmt.Errorf("ошибка чтения заголовка: %w", err)
	}
	
	// Декодируем длину команды
	commandLength := binary.BigEndian.Uint32(header[0:4])
	
	if commandLength < protocol.PDUHeaderLength {
		return nil, fmt.Errorf("неверная длина команды: %d", commandLength)
	}
	
	if commandLength > 65536 { // Максимальный разумный размер
		return nil, fmt.Errorf("слишком большая длина команды: %d", commandLength)
	}
	
	// Читаем тело
	bodyLength := int(commandLength) - protocol.PDUHeaderLength
	body := make([]byte, bodyLength)
	if bodyLength > 0 {
		if _, err := conn.Read(body); err != nil {
			return nil, fmt.Errorf("ошибка чтения тела: %w", err)
		}
	}
	
	// Декодируем PDU
	decoder := protocol.NewDecoder(append(header, body...))
	pdu, err := decoder.DecodePDU()
	if err != nil {
		return nil, fmt.Errorf("ошибка декодирования PDU: %w", err)
	}
	
	return pdu, nil
}

// cleanupInactiveSessions очищает неактивные сессии
func (s *Server) cleanupInactiveSessions() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.sessionsMu.Lock()
			now := time.Now()
			boundCount := 0
			for id, session := range s.sessions {
				// Закрываем сессии, которые неактивны более 5 минут
				if now.Sub(session.GetLastActivity()) > 5*time.Minute {
					s.logger.Warn().
						Str("session_id", id).
						Dur("inactive_time", now.Sub(session.GetLastActivity())).
						Msg("закрытие неактивной сессии")
					
					session.Close()
					delete(s.sessions, id)
				} else if session.IsBound() {
					boundCount++
				}
			}
			totalCount := len(s.sessions)
			s.sessionsMu.Unlock()
			
			// Обновляем метрики соединений
			monitoring.SMPPConnectionsActive.WithLabelValues("total").Set(float64(totalCount))
			monitoring.SMPPConnectionsActive.WithLabelValues("bound").Set(float64(boundCount))
			monitoring.SMPPConnectionsActive.WithLabelValues("unbound").Set(float64(totalCount - boundCount))
		}
	}
}

// enquireLinkLoop отправляет enquire_link для поддержания соединения
func (s *Server) enquireLinkLoop() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(s.config.EnquireLinkPeriod)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.sessionsMu.RLock()
			for _, session := range s.sessions {
				if !session.IsBound() {
					continue
				}
				
				// Проверяем, когда последний раз отправляли enquire_link
				lastSent := session.GetEnquireLinkSent()
				
				// Если прошло больше половины периода, отправляем enquire_link
				if time.Since(lastSent) > s.config.EnquireLinkPeriod/2 {
					s.sendEnquireLink(session)
				}
			}
			s.sessionsMu.RUnlock()
		}
	}
}

// sendEnquireLink отправляет enquire_link для сессии
func (s *Server) sendEnquireLink(session *smppsession.Session) {
	enquire := &protocol.EnquireLinkPDU{}
	encoder := protocol.NewEncoder()
	body, err := encoder.EncodeEnquireLink(enquire)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("session_id", session.ID).
			Msg("ошибка кодирования enquire_link")
		return
	}
	
	seqNum := session.NextSequenceNumber()
	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(body)),
		CommandID:      protocol.EnquireLink,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: seqNum,
		Body:           body,
	}
	
	data, err := encoder.EncodePDU(pdu)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("session_id", session.ID).
			Msg("ошибка кодирования PDU")
		return
	}
	
	if conn := session.GetConn(); conn != nil {
		if err := conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout)); err != nil {
			s.logger.Error().Err(err).Msg("ошибка установки write deadline")
			return
		}
		
		if _, err := conn.Write(data); err != nil {
			s.logger.Error().
				Err(err).
				Str("session_id", session.ID).
				Msg("ошибка отправки enquire_link")
			return
		}
		
		session.UpdateEnquireLinkSent()
	}
}

// GetActiveSessionsCount возвращает количество активных сессий
func (s *Server) GetActiveSessionsCount() int {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	return len(s.sessions)
}

// GetBoundSessionsCount возвращает количество привязанных сессий
func (s *Server) GetBoundSessionsCount() int {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	
	count := 0
	for _, session := range s.sessions {
		if session.IsBound() {
			count++
		}
	}
	return count
}
