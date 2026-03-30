package smsc

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
	smppprotocol "github.com/smpp-server/smpp-server/internal/smpp/protocol"
)

// SubmitSMResponse представляет ответ на submit_sm для асинхронного пайплайна
type SubmitSMResponse struct {
	MessageID     string
	CommandStatus uint32
	SequenceNum   uint32
}

// AsyncConnection представляет асинхронное SMPP соединение с sliding window
type AsyncConnection struct {
	ID               string
	ProviderID       uuid.UUID
	Conn             net.Conn
	Bound            bool
	sequenceNum      uint32              // atomic
	PendingResponses sync.Map            // map[uint32]chan *SubmitSMResponse
	WindowSem        chan struct{}        // semaphore for sliding window
	WriterCh         chan []byte          // channel for writer goroutine
	CreatedAt        time.Time
	LastUsed         int64               // atomic unix nano
	throttler        *Throttler
	logger           zerolog.Logger
	cancel           context.CancelFunc
	done             chan struct{}
}

// NextSequence возвращает следующий номер последовательности (atomic, wraps at 0x7FFFFFFF)
func (ac *AsyncConnection) NextSequence() uint32 {
	for {
		current := atomic.LoadUint32(&ac.sequenceNum)
		next := current + 1
		if next > 0x7FFFFFFF {
			next = 1
		}
		if atomic.CompareAndSwapUint32(&ac.sequenceNum, current, next) {
			return next
		}
	}
}

// startWriter — горутина, читающая из WriterCh и отправляющая данные в Conn
func (ac *AsyncConnection) startWriter(ctx context.Context) {
	defer func() {
		ac.logger.Debug().Msg("writer goroutine stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-ac.WriterCh:
			if !ok {
				return
			}
			ac.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if _, err := ac.Conn.Write(data); err != nil {
				ac.logger.Error().Err(err).Msg("ошибка записи в соединение, закрываем")
				ac.Close()
				return
			}
			atomic.StoreInt64(&ac.LastUsed, time.Now().UnixNano())
		}
	}
}

// startReader — горутина, читающая PDU ответы из Conn и маршрутизирующая их по sequence number
func (ac *AsyncConnection) startReader(ctx context.Context) {
	defer func() {
		ac.logger.Debug().Msg("reader goroutine stopped")
		close(ac.done)
	}()

	header := make([]byte, smppprotocol.PDUHeaderLength)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Читаем заголовок PDU (16 байт)
		ac.Conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		if _, err := io.ReadFull(ac.Conn, header); err != nil {
			if ctx.Err() != nil {
				return // контекст отменён
			}
			ac.logger.Error().Err(err).Msg("ошибка чтения заголовка PDU")
			ac.Close()
			return
		}

		// Парсим заголовок
		commandLength := binary.BigEndian.Uint32(header[0:4])
		commandID := binary.BigEndian.Uint32(header[4:8])
		commandStatus := binary.BigEndian.Uint32(header[8:12])
		sequenceNum := binary.BigEndian.Uint32(header[12:16])

		// Читаем тело если есть
		bodyLength := int(commandLength) - smppprotocol.PDUHeaderLength
		var body []byte
		if bodyLength > 0 {
			body = make([]byte, bodyLength)
			ac.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			if _, err := io.ReadFull(ac.Conn, body); err != nil {
				if ctx.Err() != nil {
					return
				}
				ac.logger.Error().Err(err).Msg("ошибка чтения тела PDU")
				ac.Close()
				return
			}
		}

		atomic.StoreInt64(&ac.LastUsed, time.Now().UnixNano())

		switch commandID {
		case smppprotocol.SubmitSMResp: // 0x80000004
			resp := &SubmitSMResponse{
				CommandStatus: commandStatus,
				SequenceNum:   sequenceNum,
			}

			// Парсим message_id из тела если статус OK и есть тело
			if commandStatus == smppprotocol.ESME_ROK && len(body) > 0 {
				// message_id — C-Octet String (null-terminated)
				for i, b := range body {
					if b == 0 {
						resp.MessageID = string(body[:i])
						break
					}
				}
			}

			// Отправляем ответ в канал по sequence number
			if ch, ok := ac.PendingResponses.LoadAndDelete(sequenceNum); ok {
				respCh := ch.(chan *SubmitSMResponse)
				select {
				case respCh <- resp:
				default:
					ac.logger.Warn().
						Uint32("sequence_num", sequenceNum).
						Msg("канал ответа переполнен, ответ отброшен")
				}
			} else {
				ac.logger.Warn().
					Uint32("sequence_num", sequenceNum).
					Msg("нет ожидающего запроса для sequence number")
			}

		case smppprotocol.EnquireLinkResp: // 0x80000015
			ac.logger.Debug().
				Uint32("sequence_num", sequenceNum).
				Msg("получен enquire_link_resp")

		default:
			ac.logger.Debug().
				Str("command", smppprotocol.GetCommandName(commandID)).
				Uint32("sequence_num", sequenceNum).
				Msg("получен неожиданный PDU, игнорируем")
		}
	}
}

// Close закрывает асинхронное соединение
func (ac *AsyncConnection) Close() error {
	ac.cancel()

	if ac.Conn != nil {
		ac.Conn.Close()
	}

	// Закрываем WriterCh (writer горутина завершится)
	select {
	case <-ac.WriterCh:
		// уже закрыт или пуст — пробуем закрыть безопасно
	default:
	}
	// Безопасное закрытие канала — recover на случай если уже закрыт
	func() {
		defer func() { recover() }()
		close(ac.WriterCh)
	}()

	return nil
}

// ConnectAsync создает асинхронное SMPP соединение с sliding window
func (p *Pool) ConnectAsync(ctx context.Context, provider *shared.Provider, windowSize int) (*AsyncConnection, error) {
	connectionID := uuid.New().String()
	acCtx, acCancel := context.WithCancel(p.ctx)

	// Симулятор — создаём фейковое соединение без TCP
	if IsSimulator(provider) {
		ac := &AsyncConnection{
			ID:         connectionID,
			ProviderID: provider.ID,
			Conn:       nil,
			Bound:      true,
			WindowSem:  make(chan struct{}, windowSize),
			WriterCh:   make(chan []byte, 1000),
			CreatedAt:  time.Now(),
			LastUsed:   time.Now().UnixNano(),
			throttler:  NewThrottler(provider.ThroughputPerSec),
			logger:     log.With().Str("async_connection_id", connectionID).Str("provider", provider.Name).Logger(),
			cancel:     acCancel,
			done:       make(chan struct{}),
		}

		p.mu.Lock()
		p.asyncConnections[provider.ID] = append(p.asyncConnections[provider.ID], ac)
		p.mu.Unlock()

		p.logger.Info().
			Str("provider_id", provider.ID.String()).
			Str("provider_name", provider.Name).
			Int("window_size", windowSize).
			Msg("async симулятор провайдера подключён (без TCP)")

		return ac, nil
	}

	addr := fmt.Sprintf("%s:%d", provider.Host, provider.Port)

	p.logger.Info().
		Str("provider_id", provider.ID.String()).
		Str("provider_name", provider.Name).
		Str("address", addr).
		Int("window_size", windowSize).
		Msg("async подключение к SMSC провайдеру")

	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		acCancel()
		return nil, fmt.Errorf("ошибка подключения к %s: %w", addr, err)
	}

	ac := &AsyncConnection{
		ID:         connectionID,
		ProviderID: provider.ID,
		Conn:       conn,
		Bound:      false,
		WindowSem:  make(chan struct{}, windowSize),
		WriterCh:   make(chan []byte, 1000),
		CreatedAt:  time.Now(),
		LastUsed:   time.Now().UnixNano(),
		throttler:  NewThrottler(provider.ThroughputPerSec),
		logger:     log.With().Str("async_connection_id", connectionID).Str("provider", provider.Name).Logger(),
		cancel:     acCancel,
		done:       make(chan struct{}),
	}

	// Выполняем bind (переиспользуем синхронную Connection для bind)
	tmpConn := &Connection{
		ID:          connectionID,
		ProviderID:  provider.ID,
		Conn:        conn,
		Bound:       false,
		LastUsed:    time.Now(),
		SequenceNum: 0,
		CreatedAt:   time.Now(),
		logger:      ac.logger,
	}

	if err := p.bind(ctx, tmpConn, provider); err != nil {
		conn.Close()
		acCancel()
		return nil, fmt.Errorf("ошибка bind к провайдеру %s: %w", provider.Name, err)
	}

	ac.Bound = true

	// Запускаем writer и reader горутины
	go ac.startWriter(acCtx)
	go ac.startReader(acCtx)

	p.mu.Lock()
	p.asyncConnections[provider.ID] = append(p.asyncConnections[provider.ID], ac)
	p.mu.Unlock()

	p.logger.Info().
		Str("provider_id", provider.ID.String()).
		Str("connection_id", ac.ID).
		Int("window_size", windowSize).
		Msg("async соединение установлено и привязано")

	return ac, nil
}

// GetAsyncConnection получает доступное асинхронное соединение для провайдера
func (p *Pool) GetAsyncConnection(providerID uuid.UUID) (*AsyncConnection, error) {
	p.mu.RLock()
	conns, exists := p.asyncConnections[providerID]
	p.mu.RUnlock()

	if !exists || len(conns) == 0 {
		return nil, fmt.Errorf("нет доступных async соединений для провайдера %s", providerID.String())
	}

	// Находим первое доступное bound соединение
	for _, ac := range conns {
		if ac.Bound {
			atomic.StoreInt64(&ac.LastUsed, time.Now().UnixNano())
			return ac, nil
		}
	}

	return nil, fmt.Errorf("нет связанных async соединений для провайдера %s", providerID.String())
}
