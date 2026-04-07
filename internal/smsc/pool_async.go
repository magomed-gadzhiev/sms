package smsc

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"regexp"
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

// DeliverSMData содержит распарсированные данные из deliver_sm PDU (DLR)
type DeliverSMData struct {
	ProviderID    uuid.UUID
	SMPPMessageID string // receipted_message_id из тела DLR
	Source        string
	Destination   string
	Stat          string // DELIVRD, UNDELIV, EXPIRED, etc.
	Err           string
	Text          string
	SubmitDate    string
	DoneDate      string
}

// DLRCallbackFunc — тип функции обратного вызова для обработки DLR (deliver_sm)
type DLRCallbackFunc func(data *DeliverSMData)

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
	dlrCallback      DLRCallbackFunc
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

		case smppprotocol.EnquireLink: // 0x00000015 — сервер проверяет соединение, отвечаем enquire_link_resp
			ac.logger.Debug().
				Uint32("sequence_num", sequenceNum).
				Msg("получен enquire_link от сервера, отправляем ответ")
			resp := buildEnquireLinkResp(sequenceNum)
			select {
			case ac.WriterCh <- resp:
			default:
				ac.logger.Warn().Msg("WriterCh переполнен, enquire_link_resp отброшен")
			}

		case smppprotocol.DeliverSM: // 0x00000005 — DLR от провайдера
			ac.logger.Info().
				Uint32("sequence_num", sequenceNum).
				Msg("получен deliver_sm (DLR) от провайдера")

			// Отправляем deliver_sm_resp
			resp := buildDeliverSMResp(sequenceNum)
			select {
			case ac.WriterCh <- resp:
			default:
				ac.logger.Warn().Msg("WriterCh переполнен, deliver_sm_resp отброшен")
			}

			// Декодируем и обрабатываем DLR
			if ac.dlrCallback != nil {
				decoder := smppprotocol.NewDecoder()
				deliverPDU, decErr := decoder.DecodeDeliverSM(body)
				if decErr != nil {
					ac.logger.Error().Err(decErr).Msg("ошибка декодирования deliver_sm PDU")
				} else {
					dlrData := parseDLRFromDeliverSM(deliverPDU, ac.ProviderID)
					ac.logger.Info().
						Str("smpp_message_id", dlrData.SMPPMessageID).
						Str("stat", dlrData.Stat).
						Str("source", dlrData.Source).
						Str("destination", dlrData.Destination).
						Msg("DLR распарсен, вызываем callback")
					ac.dlrCallback(dlrData)
				}
			}

		case smppprotocol.Unbind: // 0x00000006 — сервер инициирует отключение
			ac.logger.Info().
				Uint32("sequence_num", sequenceNum).
				Msg("получен unbind от сервера, отправляем unbind_resp и закрываем соединение")
			resp := buildUnbindResp(sequenceNum)
			select {
			case ac.WriterCh <- resp:
			default:
			}
			ac.Close()
			return

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

// buildEnquireLinkResp строит PDU enquire_link_resp (command_id=0x80000015, пустое тело)
func buildEnquireLinkResp(sequenceNum uint32) []byte {
	pdu := make([]byte, 16)
	pdu[0], pdu[1], pdu[2], pdu[3] = 0, 0, 0, 16         // command_length = 16
	pdu[4], pdu[5], pdu[6], pdu[7] = 0x80, 0, 0, 0x15    // command_id = 0x80000015
	pdu[8], pdu[9], pdu[10], pdu[11] = 0, 0, 0, 0         // command_status = 0
	pdu[12] = byte(sequenceNum >> 24)
	pdu[13] = byte(sequenceNum >> 16)
	pdu[14] = byte(sequenceNum >> 8)
	pdu[15] = byte(sequenceNum)
	return pdu
}

// buildUnbindResp строит PDU unbind_resp (command_id=0x80000006, пустое тело)
func buildUnbindResp(sequenceNum uint32) []byte {
	pdu := make([]byte, 16)
	pdu[0], pdu[1], pdu[2], pdu[3] = 0, 0, 0, 16         // command_length = 16
	pdu[4], pdu[5], pdu[6], pdu[7] = 0x80, 0, 0, 0x06    // command_id = 0x80000006
	pdu[8], pdu[9], pdu[10], pdu[11] = 0, 0, 0, 0         // command_status = 0
	pdu[12] = byte(sequenceNum >> 24)
	pdu[13] = byte(sequenceNum >> 16)
	pdu[14] = byte(sequenceNum >> 8)
	pdu[15] = byte(sequenceNum)
	return pdu
}

// buildDeliverSMResp строит PDU deliver_sm_resp (command_id=0x80000005, тело = null-terminated message_id)
func buildDeliverSMResp(sequenceNum uint32) []byte {
	// Тело: message_id (C-Octet String) — для DLR ответа можно отправить пустой
	bodyLen := 1 // just null terminator for empty message_id
	totalLen := 16 + bodyLen
	pdu := make([]byte, totalLen)
	binary.BigEndian.PutUint32(pdu[0:4], uint32(totalLen)) // command_length
	binary.BigEndian.PutUint32(pdu[4:8], 0x80000005)       // command_id = deliver_sm_resp
	binary.BigEndian.PutUint32(pdu[8:12], 0)               // command_status = ESME_ROK
	binary.BigEndian.PutUint32(pdu[12:16], sequenceNum)     // sequence_number
	pdu[16] = 0                                             // message_id = "" (null-terminated)
	return pdu
}

// dlrFieldRegex парсит стандартные поля DLR из short_message
var dlrFieldRegex = regexp.MustCompile(`(?i)id:(\S+)\s+sub:(\S+)\s+dlvrd:(\S+)\s+submit date:(\S+)\s+done date:(\S+)\s+stat:(\S+)\s+err:(\S+)(?:\s+[Tt]ext:(.*))?`)

// parseDLRFromDeliverSM извлекает DLR-данные из deliver_sm PDU
func parseDLRFromDeliverSM(pdu *smppprotocol.DeliverSMPDU, providerID uuid.UUID) *DeliverSMData {
	data := &DeliverSMData{
		ProviderID:  providerID,
		Source:      pdu.SourceAddr,
		Destination: pdu.DestinationAddr,
	}

	// Парсим short_message для стандартного формата DLR
	msg := string(pdu.ShortMessage)
	matches := dlrFieldRegex.FindStringSubmatch(msg)
	if len(matches) >= 7 {
		data.SMPPMessageID = matches[1]
		data.SubmitDate = matches[4]
		data.DoneDate = matches[5]
		data.Stat = matches[6]
		data.Err = matches[7]
		if len(matches) >= 9 {
			data.Text = matches[8]
		}
	} else {
		// Fallback: пробуем TLV receipted_message_id (tag 0x001E)
		if receiptedID, ok := pdu.TLV[0x001E]; ok {
			// Убираем null-terminator если есть
			id := string(receiptedID)
			if len(id) > 0 && id[len(id)-1] == 0 {
				id = id[:len(id)-1]
			}
			data.SMPPMessageID = id
		}
		// Пробуем TLV message_state (tag 0x0427)
		if stateBytes, ok := pdu.TLV[0x0427]; ok && len(stateBytes) > 0 {
			data.Stat = mapMessageState(stateBytes[0])
		}
		// Если stat всё ещё пуст, но есть short_message — ставим его как text
		if data.Stat == "" {
			data.Stat = "UNKNOWN"
			data.Text = msg
		}
	}

	return data
}

// mapMessageState маппит числовой message_state из TLV в текстовый статус
func mapMessageState(state byte) string {
	switch state {
	case 1:
		return "ENROUTE"
	case 2:
		return "DELIVRD"
	case 3:
		return "EXPIRED"
	case 4:
		return "DELETED"
	case 5:
		return "UNDELIV"
	case 6:
		return "ACCEPTD"
	case 7:
		return "UNKNOWN"
	case 8:
		return "REJECTD"
	default:
		return "UNKNOWN"
	}
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

	// Пробуем подключиться с retry для обработки временных отказов сервера
	const maxBindRetries = 3
	var conn net.Conn
	var bindErr error
	for attempt := 1; attempt <= maxBindRetries; attempt++ {
		if attempt > 1 {
			p.logger.Warn().
				Str("provider_name", provider.Name).
				Int("attempt", attempt).
				Msg("повторная попытка bind к провайдеру")
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		var dialErr error
		conn, dialErr = net.DialTimeout("tcp", addr, 30*time.Second)
		if dialErr != nil {
			bindErr = fmt.Errorf("ошибка подключения к %s: %w", addr, dialErr)
			continue
		}

		tmpConn := &Connection{
			ID:          connectionID,
			ProviderID:  provider.ID,
			Conn:        conn,
			Bound:       false,
			LastUsed:    time.Now(),
			SequenceNum: 0,
			CreatedAt:   time.Now(),
			logger:      log.With().Str("async_connection_id", connectionID).Str("provider", provider.Name).Logger(),
		}

		if bindErr = p.bind(ctx, tmpConn, provider); bindErr == nil {
			break
		}
		conn.Close()
		conn = nil
	}

	if bindErr != nil {
		acCancel()
		return nil, fmt.Errorf("ошибка bind к провайдеру %s после %d попыток: %w", provider.Name, maxBindRetries, bindErr)
	}

	ac := &AsyncConnection{
		ID:          connectionID,
		ProviderID:  provider.ID,
		Conn:        conn,
		Bound:       false,
		WindowSem:   make(chan struct{}, windowSize),
		WriterCh:    make(chan []byte, 1000),
		CreatedAt:   time.Now(),
		LastUsed:    time.Now().UnixNano(),
		throttler:   NewThrottler(provider.ThroughputPerSec),
		logger:      log.With().Str("async_connection_id", connectionID).Str("provider", provider.Name).Logger(),
		cancel:      acCancel,
		done:        make(chan struct{}),
		dlrCallback: p.dlrCallback,
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
