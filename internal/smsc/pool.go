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
	"github.com/smpp-server/smpp-server/internal/config"
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

// Throttler управляет ограничением скорости отправки
type Throttler struct {
	tokensPerSecond int
	tokens          int64
	lastUpdate      int64
	mu              sync.Mutex
}

// NewThrottler создает новый throttler
func NewThrottler(tokensPerSecond int) *Throttler {
	return &Throttler{
		tokensPerSecond: tokensPerSecond,
		tokens:          int64(tokensPerSecond),
		lastUpdate:      time.Now().UnixNano(),
	}
}

// Allow проверяет, можно ли отправить сообщение
func (t *Throttler) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().UnixNano()
	elapsed := now - t.lastUpdate
	tokensToAdd := int64(t.tokensPerSecond) * elapsed / int64(time.Second)

	if tokensToAdd > 0 {
		t.tokens = min(int64(t.tokensPerSecond), t.tokens+tokensToAdd)
		t.lastUpdate = now
	}

	if t.tokens > 0 {
		t.tokens--
		return true
	}
	return false
}

// min возвращает минимальное значение
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

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

// Pool представляет пул соединений к SMSC провайдерам
type Pool struct {
	connections      map[uuid.UUID][]*Connection
	asyncConnections map[uuid.UUID][]*AsyncConnection
	config           *config.WorkerConfig
	mu               sync.RWMutex
	logger           zerolog.Logger
	ctx              context.Context
	cancel           context.CancelFunc
	seqNum           uint32
}

// NewPool создает новый пул соединений
func NewPool(cfg *config.WorkerConfig) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	logger := log.With().Str("component", "smsc_pool").Logger()

	return &Pool{
		connections:      make(map[uuid.UUID][]*Connection),
		asyncConnections: make(map[uuid.UUID][]*AsyncConnection),
		config:           cfg,
		logger:           logger,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// IsSimulator проверяет, является ли провайдер симулятором
func IsSimulator(provider *shared.Provider) bool {
	return provider.SystemType == "SIMULATOR"
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
	conn.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := conn.Conn.Write(pduBytes); err != nil {
		return fmt.Errorf("ошибка отправки bind запроса: %w", err)
	}

	// Читаем ответ
	conn.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	header := make([]byte, smppprotocol.PDUHeaderLength)
	if _, err := conn.Conn.Read(header); err != nil {
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
		if _, err := conn.Conn.Read(bodyData); err != nil {
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

// enquireLinkLoop запускает цикл enquire link для поддержания соединения
func (p *Pool) enquireLinkLoop(conn *Connection, provider *shared.Provider) {
	ticker := time.NewTicker(60 * time.Second) // enquire link каждые 60 секунд
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			if err := p.enquireLink(conn); err != nil {
				conn.logger.Error().Err(err).Msg("ошибка enquire link")
				// Переподключаемся
				p.reconnect(conn, provider)
				return
			}
		}
	}
}

// enquireLink отправляет enquire_link для проверки соединения
func (p *Pool) enquireLink(conn *Connection) error {
	seqNum := atomic.AddUint32(&p.seqNum, 1)
	conn.SequenceNum = seqNum

	encoder := smppprotocol.NewEncoder()
	body, err := encoder.EncodeEnquireLink(&smppprotocol.EnquireLinkPDU{})
	if err != nil {
		return err
	}

	pdu := &smppprotocol.PDU{
		CommandLength:  uint32(smppprotocol.PDUHeaderLength + len(body)),
		CommandID:      smppprotocol.EnquireLink,
		CommandStatus:  0,
		SequenceNumber: seqNum,
		Body:           body,
	}

	pduBytes, err := encoder.EncodePDU(pdu)
	if err != nil {
		return err
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Conn.Write(pduBytes); err != nil {
		return err
	}

	// Читаем ответ
	conn.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	header := make([]byte, smppprotocol.PDUHeaderLength)
	if _, err := conn.Conn.Read(header); err != nil {
		return err
	}

	var respCommandStatus uint32
	respCommandStatus = uint32(header[8])<<24 | uint32(header[9])<<16 | uint32(header[10])<<8 | uint32(header[11])

	if respCommandStatus != smppprotocol.ESME_ROK {
		return fmt.Errorf("enquire_link failed: статус 0x%08X", respCommandStatus)
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

// Close закрывает все соединения
func (p *Pool) Close() error {
	p.cancel()

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, conns := range p.connections {
		for _, conn := range conns {
			if conn.Conn != nil {
				conn.Conn.Close()
			}
		}
	}

	for _, conns := range p.asyncConnections {
		for _, ac := range conns {
			ac.Close()
		}
	}

	p.connections = make(map[uuid.UUID][]*Connection)
	p.asyncConnections = make(map[uuid.UUID][]*AsyncConnection)
	return nil
}

// HealthCheck проверяет здоровье соединений
func (p *Pool) HealthCheck() map[uuid.UUID]int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	health := make(map[uuid.UUID]int)
	for providerID, conns := range p.connections {
		healthyCount := 0
		for _, conn := range conns {
			conn.mu.RLock()
			if conn.Bound {
				healthyCount++
			}
			conn.mu.RUnlock()
		}
		health[providerID] = healthyCount
	}

	return health
}
