package smpp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/services/provider/application"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smsc"
)

// PoolAdapter адаптирует smsc.Pool к application.ConnectionPoolService
// и предоставляет доступ к smsc.Sender
type PoolAdapter struct {
	pool              *smsc.Pool
	sender            *smsc.Sender
	providerConnCount map[uuid.UUID]int // отслеживаем количество соединений для каждого провайдера
	mu                sync.RWMutex
	logger            zerolog.Logger
}

// GetSender возвращает smsc.Sender для использования в SenderService
func (a *PoolAdapter) GetSender() *smsc.Sender {
	return a.sender
}

// NewPoolAdapter создает новый адаптер пула
func NewPoolAdapter(cfg *config.WorkerConfig) *PoolAdapter {
	pool := smsc.NewPool(cfg)
	return &PoolAdapter{
		pool:              pool,
		sender:            smsc.NewSender(pool),
		providerConnCount: make(map[uuid.UUID]int),
		logger:            log.With().Str("component", "pool_adapter").Logger(),
	}
}

// Connect устанавливает соединения к провайдеру
// Создает количество соединений согласно MaxConnections провайдера (мигрировано из worker)
func (a *PoolAdapter) Connect(ctx context.Context, provider *domain.Provider) error {
	if !provider.IsActive() {
		return domain.ErrProviderInactive
	}

	sharedProvider := domainToSharedProvider(provider)

	// Получаем текущее количество соединений для провайдера
	a.mu.Lock()
	currentConnCount := a.providerConnCount[provider.ID]
	desiredConnCount := provider.MaxConnections
	a.mu.Unlock()

	// Если уже достаточно соединений, ничего не делаем
	if currentConnCount >= desiredConnCount {
		a.logger.Debug().
			Str("provider_id", provider.ID.String()).
			Int("current", currentConnCount).
			Int("desired", desiredConnCount).
			Msg("достаточно соединений к провайдеру")
		return nil
	}

	// Создаем недостающие соединения
	connectionsToCreate := desiredConnCount - currentConnCount
	a.logger.Info().
		Str("provider_id", provider.ID.String()).
		Str("provider_name", provider.Name).
		Int("current", currentConnCount).
		Int("desired", desiredConnCount).
		Int("creating", connectionsToCreate).
		Msg("инициализация соединений к провайдеру")

	successCount := 0
	for i := 0; i < connectionsToCreate; i++ {
		connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, err := a.pool.Connect(connCtx, sharedProvider)
		cancel()

		if err != nil {
			a.logger.Error().
				Err(err).
				Str("provider_id", provider.ID.String()).
				Str("provider_name", provider.Name).
				Int("connection", i+1).
				Msg("ошибка подключения к провайдеру")
			// Продолжаем пытаться подключить остальные соединения
			continue
		}

		successCount++
		a.logger.Debug().
			Str("provider_id", provider.ID.String()).
			Str("provider_name", provider.Name).
			Int("connection", i+1).
			Msg("соединение к провайдеру установлено")
	}

	// Обновляем счетчик успешных соединений
	if successCount > 0 {
		a.mu.Lock()
		a.providerConnCount[provider.ID] = currentConnCount + successCount
		a.mu.Unlock()
	}

	if successCount == 0 && connectionsToCreate > 0 {
		return fmt.Errorf("не удалось установить ни одного соединения к провайдеру %s", provider.Name)
	}

	a.logger.Info().
		Str("provider_id", provider.ID.String()).
		Str("provider_name", provider.Name).
		Int("success", successCount).
		Int("failed", connectionsToCreate-successCount).
		Msg("инициализация соединений завершена")

	return nil
}

// Disconnect закрывает соединения к провайдеру
func (a *PoolAdapter) Disconnect(providerID uuid.UUID) error {
	a.mu.Lock()
	delete(a.providerConnCount, providerID)
	a.mu.Unlock()

	// Пул управляется автоматически, но можем добавить явное закрытие если нужно
	// В текущей реализации smsc.Pool не предоставляет метод для закрытия соединений конкретного провайдера
	a.logger.Debug().
		Str("provider_id", providerID.String()).
		Msg("соединения провайдера помечены для закрытия")
	return nil
}

// GetConnection получает доступное соединение
func (a *PoolAdapter) GetConnection(providerID uuid.UUID) (application.Connection, error) {
	conn, err := a.pool.GetConnection(providerID)
	if err != nil {
		return nil, err
	}

	// Возвращаем адаптер соединения
	return &ConnectionAdapter{
		conn: conn,
		pool: a.pool,
	}, nil
}

// HealthCheck проверяет состояние соединений
func (a *PoolAdapter) HealthCheck(providerID uuid.UUID) (int, int, error) {
	health := a.pool.HealthCheck()
	activeConnections := health[providerID]
	
	// Получаем общее количество соединений из нашего счетчика
	a.mu.RLock()
	totalConnections := a.providerConnCount[providerID]
	a.mu.RUnlock()
	
	// Если счетчик пуст, но есть активные соединения, используем активные как общее количество
	if totalConnections == 0 && activeConnections > 0 {
		totalConnections = activeConnections
	}
	
	return activeConnections, totalConnections, nil
}

// CloseAll закрывает все соединения
func (a *PoolAdapter) CloseAll() error {
	return a.pool.Close()
}

// ConnectionAdapter адаптирует smsc.Connection к application.Connection
type ConnectionAdapter struct {
	conn *smsc.Connection
	pool *smsc.Pool
}

// IsBound проверяет, привязано ли соединение
func (a *ConnectionAdapter) IsBound() bool {
	return a.conn.IsBound()
}

// Close закрывает соединение
func (a *ConnectionAdapter) Close() error {
	return a.pool.CloseConnection(a.conn)
}

// domainToSharedProvider преобразует domain.Provider в shared.Provider
func domainToSharedProvider(p *domain.Provider) *shared.Provider {
	return &shared.Provider{
		ID:               p.ID,
		Name:             p.Name,
		Host:             p.Host,
		Port:             p.Port,
		SystemID:         p.SystemID,
		Password:         p.Password,
		SystemType:       p.SystemType,
		BindType:         string(p.BindType),
		BindTON:          p.BindTON,
		BindNPI:          p.BindNPI,
		AddrTON:          p.AddrTON,
		AddrNPI:          p.AddrNPI,
		AddressRange:     p.AddressRange,
		MaxConnections:   p.MaxConnections,
		Active:           p.Active,
		Priority:         p.Priority,
		ThroughputPerSec: p.ThroughputPerSec,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}
