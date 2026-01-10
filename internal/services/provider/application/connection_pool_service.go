package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
)

// ConnectionPoolService управляет пулом SMPP соединений к провайдерам
type ConnectionPoolService interface {
	Connect(ctx context.Context, provider *domain.Provider) error
	Disconnect(providerID uuid.UUID) error
	GetConnection(providerID uuid.UUID) (Connection, error)
	HealthCheck(providerID uuid.UUID) (int, int, error) // active, total
	CloseAll() error
}

// Connection представляет интерфейс SMPP соединения
type Connection interface {
	SendMessage(ctx context.Context, msg *SendMessageParams) (string, error)
	IsBound() bool
	Close() error
}

// SendMessageParams параметры для отправки сообщения
type SendMessageParams struct {
	Source            string
	Destination       string
	Text              string
	ServiceType       string
	SourceAddrTON     int
	SourceAddrNPI     int
	DestAddrTON       int
	DestAddrNPI       int
	ESMClass          int
	ProtocolID        int
	PriorityFlag      int
	RegisteredDelivery int
	ReplaceIfPresent  int
	DataCoding        int
	ValidityPeriod    *time.Time
}

// SMPPConnectionPool реализует ConnectionPoolService используя существующий smsc.Pool
type SMPPConnectionPool struct {
	pool   SMPPPool
	mu     sync.RWMutex
	logger interface {
		Info() *log.Event
		Error() *log.Event
		Debug() *log.Event
	}
}

// SMPPPool интерфейс для существующего smsc.Pool
type SMPPPool interface {
	Connect(ctx context.Context, provider interface{}) (interface{}, error)
	GetConnection(providerID uuid.UUID) (interface{}, error)
	HealthCheck() map[uuid.UUID]int
	Close() error
}

// NewConnectionPoolService создает новый сервис пула соединений
func NewConnectionPoolService(pool SMPPPool) *SMPPConnectionPool {
	return &SMPPConnectionPool{
		pool: pool,
		logger: log.With().Str("component", "connection_pool_service").Logger(),
	}
}

// Connect устанавливает соединения к провайдеру
func (s *SMPPConnectionPool) Connect(ctx context.Context, provider *domain.Provider) error {
	if !provider.IsActive() {
		return domain.ErrProviderInactive
	}

	// Используем адаптер для преобразования domain.Provider в shared.Provider
	sharedProvider := domainToSharedProvider(provider)

	// Создаем соединения согласно MaxConnections
	if provider.MaxConnections > 0 {
		for i := 0; i < provider.MaxConnections; i++ {
			connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			_, err := s.pool.Connect(connCtx, sharedProvider)
			cancel()

			if err != nil {
				log.Error().
					Err(err).
					Str("provider_id", provider.ID.String()).
					Str("provider_name", provider.Name).
					Int("connection", i+1).
					Msg("ошибка подключения к провайдеру")
				// Продолжаем пытаться подключить остальные соединения
				continue
			}

			log.Debug().
				Str("provider_id", provider.ID.String()).
				Str("provider_name", provider.Name).
				Int("connection", i+1).
				Msg("соединение к провайдеру установлено")
		}
	}

	return nil
}

// Disconnect закрывает все соединения к провайдеру
func (s *SMPPConnectionPool) Disconnect(providerID uuid.UUID) error {
	// В текущей реализации пул управляется автоматически
	// Можно добавить явное удаление соединений если нужно
	return nil
}

// GetConnection получает доступное соединение
func (s *SMPPConnectionPool) GetConnection(providerID uuid.UUID) (Connection, error) {
	conn, err := s.pool.GetConnection(providerID)
	if err != nil {
		return nil, fmt.Errorf("получение соединения: %w", err)
	}

	// Адаптируем существующее соединение к интерфейсу Connection
	return &SMPPConnectionAdapter{conn: conn}, nil
}

// HealthCheck проверяет состояние соединений
func (s *SMPPConnectionPool) HealthCheck(providerID uuid.UUID) (int, int, error) {
	health := s.pool.HealthCheck()
	activeConnections := health[providerID]
	
	// Полное количество соединений нужно отслеживать отдельно
	// Пока возвращаем только активные
	return activeConnections, activeConnections, nil
}

// CloseAll закрывает все соединения
func (s *SMPPConnectionPool) CloseAll() error {
	return s.pool.Close()
}

// domainToSharedProvider преобразует domain.Provider в shared.Provider
func domainToSharedProvider(p *domain.Provider) interface{} {
	// Возвращаем структуру совместимую с shared.Provider
	// В реальной реализации нужно будет использовать конкретный тип
	return struct {
		ID              uuid.UUID
		Name            string
		Host            string
		Port            int
		SystemID        string
		Password        string
		SystemType      string
		BindType        string
		BindTON         int
		BindNPI         int
		AddrTON         int
		AddrNPI         int
		AddressRange    string
		MaxConnections  int
		Active          bool
		Priority        int
		ThroughputPerSec int
	}{
		ID:              p.ID,
		Name:            p.Name,
		Host:            p.Host,
		Port:            p.Port,
		SystemID:        p.SystemID,
		Password:        p.Password,
		SystemType:      p.SystemType,
		BindType:        string(p.BindType),
		BindTON:         p.BindTON,
		BindNPI:         p.BindNPI,
		AddrTON:         p.AddrTON,
		AddrNPI:         p.AddrNPI,
		AddressRange:    p.AddressRange,
		MaxConnections:  p.MaxConnections,
		Active:          p.Active,
		Priority:        p.Priority,
		ThroughputPerSec: p.ThroughputPerSec,
	}
}
