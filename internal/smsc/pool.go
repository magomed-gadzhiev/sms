package smsc

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/shared"
)

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
	dlrCallback      DLRCallbackFunc
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

// SetDLRCallback устанавливает функцию обратного вызова для обработки DLR (deliver_sm) от провайдеров
func (p *Pool) SetDLRCallback(cb DLRCallbackFunc) {
	p.dlrCallback = cb
}

// IsSimulator проверяет, является ли провайдер симулятором
func IsSimulator(provider *shared.Provider) bool {
	return provider.SystemType == "SIMULATOR"
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
