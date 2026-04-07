package smsc

import (
	"context"
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

// Connection представляет SMPP соединение к SMSC провайдеру
type Connection struct {
	ID           string
	ProviderID   uuid.UUID
	Conn         net.Conn
	Bound        bool
	LastUsed     time.Time
	SequenceNum  uint32
	CreatedAt    time.Time
	mu           sync.RWMutex
	throttler    *Throttler
	logger       zerolog.Logger
}

// IsBound проверяет, привязано ли соединение
func (c *Connection) IsBound() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Bound
}

// Connect создает новое соединение к провайдеру
func (p *Pool) Connect(ctx context.Context, provider *shared.Provider) (*Connection, error) {
	connectionID := uuid.New().String()

	// Симулятор — создаём фейковое соединение без TCP
	if IsSimulator(provider) {
		connection := &Connection{
			ID:          connectionID,
			ProviderID:  provider.ID,
			Conn:        nil,
			Bound:       true,
			LastUsed:    time.Now(),
			SequenceNum: 0,
			CreatedAt:   time.Now(),
			throttler:   NewThrottler(provider.ThroughputPerSec),
			logger:      log.With().Str("connection_id", connectionID).Str("provider", provider.Name).Logger(),
		}

		p.mu.Lock()
		p.connections[provider.ID] = append(p.connections[provider.ID], connection)
		p.mu.Unlock()

		p.logger.Info().
			Str("provider_id", provider.ID.String()).
			Str("provider_name", provider.Name).
			Msg("симулятор провайдера подключён (без TCP)")

		return connection, nil
	}

	addr := fmt.Sprintf("%s:%d", provider.Host, provider.Port)

	p.logger.Info().
		Str("provider_id", provider.ID.String()).
		Str("provider_name", provider.Name).
		Str("address", addr).
		Msg("подключение к SMSC провайдеру")

	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к %s: %w", addr, err)
	}

	connection := &Connection{
		ID:          connectionID,
		ProviderID:  provider.ID,
		Conn:        conn,
		Bound:       false,
		LastUsed:    time.Now(),
		SequenceNum: 0,
		CreatedAt:   time.Now(),
		throttler:   NewThrottler(provider.ThroughputPerSec),
		logger:      log.With().Str("connection_id", connectionID).Str("provider", provider.Name).Logger(),
	}

	// Выполняем bind
	if err := p.bind(ctx, connection, provider); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ошибка bind к провайдеру %s: %w", provider.Name, err)
	}

	connection.Bound = true

	// Запускаем enquire link для keep-alive
	go p.enquireLinkLoop(connection, provider)

	p.mu.Lock()
	p.connections[provider.ID] = append(p.connections[provider.ID], connection)
	p.mu.Unlock()

	p.logger.Info().
		Str("provider_id", provider.ID.String()).
		Str("connection_id", connection.ID).
		Msg("соединение установлено и привязано")

	return connection, nil
}

// bind выполняет SMPP bind операцию
func (p *Pool) bind(ctx context.Context, conn *Connection, provider *shared.Provider) error {
	seqNum := atomic.AddUint32(&p.seqNum, 1)
	conn.SequenceNum = seqNum

	// Определяем тип bind команды
	var commandID uint32
	switch provider.BindType {
	case "transceiver":
		commandID = smppprotocol.BindTransceiver
	case "transmitter":
		commandID = smppprotocol.BindTransmitter
	case "receiver":
		commandID = smppprotocol.BindReceiver
	default:
		return fmt.Errorf("неподдерживаемый тип bind: %s", provider.BindType)
	}

	// Создаем bind PDU
	bindPDU := &smppprotocol.BindPDU{
		SystemID:          provider.SystemID,
		Password:          provider.Password,
		SystemType:        provider.SystemType,
		InterfaceVersion:  smppprotocol.Version,
		AddrTON:           byte(provider.BindTON),
		AddrNPI:           byte(provider.BindNPI),
		AddressRange:      provider.AddressRange,
	}

	// Кодируем bind PDU
	encoder := smppprotocol.NewEncoder()
	body, err := encoder.EncodeBind(bindPDU)
	if err != nil {
		return fmt.Errorf("ошибка кодирования bind PDU: %w", err)
	}

	// Создаем полный PDU
	pdu := &smppprotocol.PDU{
		CommandLength:  uint32(smppprotocol.PDUHeaderLength + len(body)),
		CommandID:      commandID,
		CommandStatus:  0,
		SequenceNumber: seqNum,
		Body:           body,
	}

	pduBytes, err := encoder.EncodePDU(pdu)
	if err != nil {
		return fmt.Errorf("ошибка кодирования PDU: %w", err)
	}

	// Отправляем bind запрос
	conn.logger.Debug().
		Str("pdu_hex", fmt.Sprintf("%x", pduBytes)).
		Int("pdu_len", len(pduBytes)).
		Msg("отправка bind PDU")
	conn.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := conn.Conn.Write(pduBytes); err != nil {
		return fmt.Errorf("ошибка отправки bind запроса: %w", err)
	}

	// Читаем ответ (используем io.ReadFull для надёжного чтения ровно 16 байт заголовка)
	conn.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	header := make([]byte, smppprotocol.PDUHeaderLength)
	if _, err := io.ReadFull(conn.Conn, header); err != nil {
		return fmt.Errorf("ошибка чтения ответа bind: %w", err)
	}

	// Декодируем заголовок
	var respCommandLength uint32
	respCommandLength = uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
	var respCommandID uint32
	respCommandID = uint32(header[4])<<24 | uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7])
	var respCommandStatus uint32
	respCommandStatus = uint32(header[8])<<24 | uint32(header[9])<<16 | uint32(header[10])<<8 | uint32(header[11])

	// Читаем тело ответа
	bodyLength := int(respCommandLength) - smppprotocol.PDUHeaderLength
	bodyData := make([]byte, bodyLength)
	if bodyLength > 0 {
		if _, err := io.ReadFull(conn.Conn, bodyData); err != nil {
			return fmt.Errorf("ошибка чтения тела ответа bind: %w", err)
		}
	}

	// Проверяем статус
	if respCommandStatus != smppprotocol.ESME_ROK {
		return fmt.Errorf("bind отклонен: статус %s (0x%08X)",
			smppprotocol.GetStatusName(respCommandStatus), respCommandStatus)
	}

	// Проверяем, что это правильный ответ
	expectedRespID := commandID | 0x80000000
	if respCommandID != expectedRespID {
		return fmt.Errorf("неожиданный ответ на bind: ожидался 0x%08X, получен 0x%08X",
			expectedRespID, respCommandID)
	}

	return nil
}

// reconnect переподключается к провайдеру
func (p *Pool) reconnect(oldConn *Connection, provider *shared.Provider) {
	oldConn.mu.Lock()
	oldConn.Bound = false
	oldConn.mu.Unlock()

	if oldConn.Conn != nil {
		oldConn.Conn.Close()
	}

	// Удаляем старое соединение
	p.mu.Lock()
	conns := p.connections[provider.ID]
	for i, c := range conns {
		if c.ID == oldConn.ID {
			p.connections[provider.ID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
	p.mu.Unlock()

	// Пытаемся переподключиться
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := p.Connect(ctx, provider); err != nil {
		p.logger.Error().
			Err(err).
			Str("provider_id", provider.ID.String()).
			Msg("ошибка переподключения к провайдеру")
	}
}

// GetConnection получает доступное соединение для провайдера
func (p *Pool) GetConnection(providerID uuid.UUID) (*Connection, error) {
	p.mu.RLock()
	conns, exists := p.connections[providerID]
	p.mu.RUnlock()

	if !exists || len(conns) == 0 {
		return nil, fmt.Errorf("нет доступных соединений для провайдера %s", providerID.String())
	}

	// Находим первое доступное соединение
	for _, conn := range conns {
		conn.mu.RLock()
		bound := conn.Bound
		conn.mu.RUnlock()

		if bound {
			conn.mu.Lock()
			conn.LastUsed = time.Now()
			conn.mu.Unlock()
			return conn, nil
		}
	}

	return nil, fmt.Errorf("нет связанных соединений для провайдера %s", providerID.String())
}

// CloseConnection закрывает соединение
func (p *Pool) CloseConnection(conn *Connection) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.Conn != nil {
		// Отправляем unbind
		if conn.Bound {
			// Здесь можно добавить отправку unbind, но для простоты просто закрываем
			conn.Bound = false
		}
		if err := conn.Conn.Close(); err != nil {
			return err
		}
	}

	// Удаляем из пула
	p.mu.Lock()
	conns := p.connections[conn.ProviderID]
	for i, c := range conns {
		if c.ID == conn.ID {
			p.connections[conn.ProviderID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
	p.mu.Unlock()

	return nil
}
